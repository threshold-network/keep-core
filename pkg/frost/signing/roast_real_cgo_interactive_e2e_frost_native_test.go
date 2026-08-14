//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	btcec2 "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// realCgoSessionSeq gives each invocation a unique session id so that in-process
// repeats (go test -count=N) over the shared, process-stable signer state path add
// a fresh DKG session instead of conflicting on a fixed one.
var realCgoSessionSeq atomic.Uint64

type realCgoSingleTransactionChain struct {
	bitcoin.Chain
	transaction *bitcoin.Transaction
}

func (rcstc *realCgoSingleTransactionChain) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	if rcstc.transaction == nil || rcstc.transaction.Hash() != transactionHash {
		return nil, fmt.Errorf("transaction [%s] not found", transactionHash)
	}

	return rcstc.transaction, nil
}

// This file is the REAL-cgo interactive signing test: a full FROST DKG that
// PERSISTS a key group, followed by one signer's interactive ROAST contribution
// driven through the engine's INTERACTIVE session API
// (DeriveInteractiveAttemptContext -> InteractiveSessionOpen -> InteractiveRound1)
// with real frost-secp256k1-tr cryptography. It complements:
//   - TestBuildTaggedTBTCSignerInteractiveFROSTBridge_WithLinkedSigner, which
//     proves the real crypto over the LOW-LEVEL Sign/Aggregate API by feeding
//     KeyPackage bytes directly; and
//   - the fake-engine runner suite (roast_runner_bus_net_e2e_frost_native_test.go),
//     which proves the multi-node Go-side orchestration - a t-subset across nodes,
//     observers, the real pkg/net transport - without crypto.
// This test covers the remaining gap: the interactive session API the runner uses,
// with the REAL engine and REAL crypto.
//
// Scope and why it is one member: the signer engine state is a PROCESS GLOBAL (a
// single Mutex<EngineState>) that holds ONE open interactive member per session -
// InteractiveSessionOpen is fingerprinted once per session and
// interactive_state_for_attempt_mut rejects any member_identifier other than the
// opener's. That is by design: each production node is its own process. The engine
// also requires threshold >= 2. So a SINGLE process can faithfully drive exactly
// one signer's real contribution (open + round 1), but NOT the full t-of-n
// finalize (NewSigningPackage over t commitments -> round 2 per member -> aggregate),
// which needs the other members' contributions from their own processes. A
// complete real-crypto signature therefore requires a multi-process harness; the
// multi-member orchestration itself is covered with the fake engine over the real
// transport in the runner net e2e.
//
// What this test does NOT prove: interactive round 2 or aggregate over the cgo
// session surface (those need >= 2 signers, i.e. multiple processes). It is a
// real-crypto round-1 / FFI-bridge / persisted-state integration test, not a full
// interactive end-to-end signature.
//
// The DKG -> interactive keyGroup glue is the distributed DKG + persist:
// InteractiveSessionOpen resolves the signing key by a keyGroup IDENTIFIER
// (engine-internal persisted material), not KeyPackage bytes. runRealCgoDKGKeyGroup
// runs a real distributed FROST DKG (Part1/2/3) for the participants and PERSISTS
// each seat's key package (PersistDistributedDKGKeyPackage), keyed by the SESSION
// ID, under a keyGroup the interactive path resolves - the same distributed flow
// production uses (the trusted-dealer RunDKG was removed with the coarse path).
// Open then requires a completed, persisted DKG under the same session_id, so the
// DKG setup and the interactive flow share one session id.
//
// To run it, link the signer library so the frost_tbtc_* symbols resolve, e.g.:
//
//	CGO_ENABLED=1 \
//	  CGO_LDFLAGS="-L<dir> -lfrost_tbtc -Wl,-rpath,<dir>" \
//	  go test -tags "frost_native frost_tbtc_signer" \
//	    -run TestRealCgoInteractiveSigning_MemberContribution ./pkg/frost/signing/
//
// Every engine call is guarded by skipFrostUnavailable: when the lib is absent, or
// present but stale (an older dylib missing a newer symbol such as
// frost_tbtc_derive_interactive_attempt_context), the test SKIPS naming the
// operation, rather than failing - so it is inert in CI builds that do not link
// the Rust signer and runs only against a current lib.

func TestRealCgoInteractiveSigning_MemberContribution(t *testing.T) {
	setupRealCgoSignerState(t)

	engine := &buildTaggedTBTCSignerEngine{}

	// The engine requires threshold >= 2; the attempt's included set is the t-subset
	// {1,2} over a 3-party DKG. This process drives the one local signer (member 1).
	const threshold = 2
	// One session id for the whole flow: the engine keys the interactive session by
	// the DKG session id, so RunDKG, derive, and open all use sessionID. It is unique
	// per invocation so in-process repeats over the stable state path add a fresh
	// session instead of conflicting on a fixed one.
	sessionID := fmt.Sprintf("real-cgo-session-%d", realCgoSessionSeq.Add(1))
	const localMember = uint16(1)
	participantIDs := []byte{1, 2, 3}
	includedMembers := []byte{1, 2}
	message := bytesOf(0x42, 32)

	// 1. Full DKG that persists a key group under sessionID, returning the keyGroup
	// the signing path resolves.
	keyGroup := runRealCgoDKGKeyGroup(t, engine, sessionID, participantIDs, threshold)

	// 2. Derive the attempt context for the t-subset, then drive the local signer's
	// interactive contribution - the same calls the interactiveSigningRunner makes,
	// here with real crypto against the DKG-persisted keyGroup.
	derived, err := engine.DeriveInteractiveAttemptContext(
		sessionID,
		message,
		keyGroup,
		threshold,
		0, // 0-based attempt number; the bridge converts to the 1-based wire value
		uint16sOf(includedMembers),
	)
	skipFrostUnavailable(t, "derive interactive attempt context", err)

	open, err := engine.InteractiveSessionOpen(
		sessionID,
		localMember,
		message,
		keyGroup,
		threshold,
		nil, // key-path spend
		nil, // generic signing has no typed intent
		derived.AttemptContext,
	)
	skipFrostUnavailable(t, "interactive session open", err)
	if open.AttemptID == "" {
		t.Fatal("interactive session open returned an empty attempt id")
	}

	// Round 1: the real engine generates this member's signing nonces and returns
	// its public commitments. A non-empty commitment is the proof the real
	// interactive crypto path (DKG-persisted key resolution -> commit) works.
	commitmentData, err := engine.InteractiveRound1(sessionID, open.AttemptID, localMember)
	skipFrostUnavailable(t, "interactive round 1", err)
	if len(commitmentData) == 0 {
		t.Fatal("interactive round 1 returned an empty commitment from the real engine")
	}
}

// TestRealCgoInteractiveSigning_MultiSeatHeartbeatAggregate drives TWO local seats
// through the FULL typed-heartbeat signing flow in ONE process against the real cgo
// engine, producing a real 2-of-3 BIP-340 signature while the policy firewall is on.
// This is also the payoff of the multi-seat engine fix
// (member-keyed interactive_signing): before it, the second seat's InteractiveSessionOpen
// failed with SessionConflict, so a single process could only drive one seat's
// contribution (see _MemberContribution). Now both seats Open, Round1, and Round2
// independently and their shares aggregate. Skip-guarded; runs only against a linked,
// CURRENT libfrost_tbtc that includes the multi-seat fix.
func TestRealCgoInteractiveSigning_MultiSeatHeartbeatAggregate(t *testing.T) {
	setupRealCgoSignerState(t)
	t.Setenv("TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL", "true")

	engine := &buildTaggedTBTCSignerEngine{}

	const threshold = 2
	sessionID := fmt.Sprintf("real-cgo-multiseat-session-%d", realCgoSessionSeq.Add(1))
	participantIDs := []byte{1, 2, 3}
	// Both seats are LOCAL members in this one process (the multi-seat case).
	signingMembers := []byte{1, 2}
	heartbeatMessage := [16]byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}
	heartbeatDigest := bitcoin.ComputeHash(heartbeatMessage[:])
	message := heartbeatDigest[:]
	signingIntent := NewHeartbeatSigningIntent(heartbeatMessage)

	keyGroup := runRealCgoDKGKeyGroup(t, engine, sessionID, participantIDs, threshold)

	derived, err := engine.DeriveInteractiveAttemptContext(
		sessionID,
		message,
		keyGroup,
		threshold,
		0,
		uint16sOf(signingMembers),
	)
	skipFrostUnavailable(t, "derive interactive attempt context", err)
	frostIDByMember := map[byte]string{}
	for _, id := range derived.FrostIdentifiers {
		frostIDByMember[byte(id.ParticipantIdentifier)] = id.FrostIdentifier
	}

	// Open + Round1 for BOTH seats in one process. The second Open succeeding (rather
	// than SessionConflict) is exactly what the multi-seat engine fix enables.
	attemptIDByMember := make(map[byte]string, len(signingMembers))
	commitments := make([]nativeFROSTCommitment, 0, len(signingMembers))
	for _, member := range signingMembers {
		open, err := engine.InteractiveSessionOpen(
			sessionID,
			uint16(member),
			message,
			keyGroup,
			threshold,
			nil, // a heartbeat must not carry a Taproot merkle root
			signingIntent,
			derived.AttemptContext,
		)
		if isPreMultiSeatConflict(err) {
			t.Skipf(
				"linked libfrost_tbtc predates the multi-seat fix (member %d's Open conflicts "+
					"with a sibling's; rebuild it from a source with member-keyed "+
					"interactive_signing): %v",
				member, err,
			)
		}
		skipFrostUnavailable(t, fmt.Sprintf("interactive session open (member %d)", member), err)
		attemptIDByMember[member] = open.AttemptID

		commitmentData, err := engine.InteractiveRound1(sessionID, open.AttemptID, uint16(member))
		skipFrostUnavailable(t, fmt.Sprintf("interactive round 1 (member %d)", member), err)
		commitments = append(commitments, nativeFROSTCommitment{
			Identifier: frostIDByMember[member],
			Data:       commitmentData,
		})
	}

	// Both seats derive the SAME attempt id (the engine derives it member-
	// independently); the aggregate below keys off one of them, so pin the invariant.
	if a, b := attemptIDByMember[signingMembers[0]], attemptIDByMember[signingMembers[1]]; a != b {
		t.Fatalf("local seats derived different attempt ids (%q vs %q)", a, b)
	}

	signingPackage, err := engine.NewSigningPackage(message, commitments)
	skipFrostUnavailable(t, "new signing package", err)

	// Round2 for BOTH seats: each releases its share independently; member 1's Round2
	// must not disturb member 2's live state (the per-member entry isolation).
	shares := make([]nativeFROSTSignatureShare, 0, len(signingMembers))
	for _, member := range signingMembers {
		shareData, err := engine.InteractiveRound2(
			sessionID,
			attemptIDByMember[member],
			uint16(member),
			signingPackage,
		)
		skipFrostUnavailable(t, fmt.Sprintf("interactive round 2 (member %d)", member), err)
		shares = append(shares, nativeFROSTSignatureShare{
			Identifier: frostIDByMember[member],
			Data:       shareData,
		})
	}

	// Aggregate the two interactive shares into a real 2-of-3 BIP-340 signature -
	// produced by two local seats in ONE process via the cgo bridge. The engine
	// validates the shares + aggregate internally, so a non-error 64-byte result is a
	// valid threshold signature (see _MemberContribution on the absent external
	// keyGroup->pubkey accessor).
	signature, err := engine.InteractiveAggregate(
		sessionID,
		attemptIDByMember[signingMembers[0]],
		signingPackage,
		shares,
		nil,
	)
	skipFrostUnavailable(t, "interactive aggregate", err)
	if len(signature) != 64 {
		t.Fatalf("unexpected multi-seat interactive signature length: %d", len(signature))
	}
}

func TestRealCgoBuildTaprootTxMatchesGoBIP341Sighash(t *testing.T) {
	setupRealCgoSignerState(t)
	t.Setenv("TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL", "true")
	t.Setenv("TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES", "p2wpkh")
	t.Setenv("TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT", "64")
	t.Setenv("TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS", "100000000")
	t.Setenv("TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS", "100000000")

	engine := &buildTaggedTBTCSignerEngine{}
	signingSessionID := fmt.Sprintf(
		"real-cgo-bip341-signing-%d",
		realCgoSessionSeq.Add(1),
	)

	privateKeyBytes := bytesOf(0x01, 32)
	_, publicKey := btcec2.PrivKeyFromBytes(privateKeyBytes)
	var taprootOutputKey [32]byte
	copy(taprootOutputKey[:], schnorr.SerializePubKey(publicKey))
	prevoutScript, err := bitcoin.PayToTaproot(taprootOutputKey)
	if err != nil {
		t.Fatalf("build P2TR prevout script: %v", err)
	}
	var outputPublicKeyHash [20]byte
	copy(outputPublicKeyHash[:], bytesOf(0x02, 20))
	outputScript, err := bitcoin.PayToWitnessPublicKeyHash(outputPublicKeyHash)
	if err != nil {
		t.Fatalf("build P2WPKH output script: %v", err)
	}

	previousTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{
						0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
						0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
						0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27,
						0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
					},
					OutputIndex: 0,
				},
				SignatureScript: []byte{0x51},
				Sequence:        0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{Value: 100_000, PublicKeyScript: prevoutScript},
		},
		Locktime: 0,
	}
	txIDHex := previousTransaction.Hash().Hex(bitcoin.ReversedByteOrder)
	const inputValue = int64(100_000)
	const outputValue = int64(90_000)

	result, err := engine.BuildTaprootTx(
		signingSessionID,
		[]NativeTBTCSignerTxInput{
			{
				TxIDHex:         txIDHex,
				Vout:            0,
				ValueSats:       uint64(inputValue),
				ScriptPubKeyHex: hex.EncodeToString(prevoutScript),
			},
		},
		[]NativeTBTCSignerTxOutput{
			{
				ScriptPubKeyHex: hex.EncodeToString(outputScript),
				ValueSats:       uint64(outputValue),
			},
		},
		nil,
	)
	skipFrostUnavailable(t, "BIP-341 BuildTaprootTx", err)

	if len(result.TaprootKeySpendSighashesHex) != 1 {
		t.Fatalf(
			"BuildTaprootTx returned %d sighashes, want 1",
			len(result.TaprootKeySpendSighashesHex),
		)
	}

	referenceBuilder := bitcoin.NewTransactionBuilder(
		&realCgoSingleTransactionChain{transaction: previousTransaction},
	)
	if err := referenceBuilder.AddTaprootKeyPathInput(
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: previousTransaction.Hash(),
				OutputIndex:     0,
			},
			Value: inputValue,
		},
	); err != nil {
		t.Fatalf("add reference P2TR input: %v", err)
	}
	referenceBuilder.AddOutput(
		&bitcoin.TransactionOutput{
			Value:           outputValue,
			PublicKeyScript: outputScript,
		},
	)
	expectedUnsignedTransaction := referenceBuilder.UnsignedTransaction()
	expectedTxHex := hex.EncodeToString(expectedUnsignedTransaction.Serialize())
	if result.TxHex != expectedTxHex {
		t.Fatalf(
			"BuildTaprootTx returned a different unsigned transaction: Rust=%s Go=%s txid=%s",
			result.TxHex,
			expectedTxHex,
			txIDHex,
		)
	}

	expectedSighashes, err := referenceBuilder.ComputeSignatureHashes()
	if err != nil {
		t.Fatalf("calculate Go BIP-341 sighash: %v", err)
	}
	if len(expectedSighashes) != 1 {
		t.Fatalf("Go builder returned %d sighashes, want 1", len(expectedSighashes))
	}
	expectedSighashHex := fmt.Sprintf("%064x", expectedSighashes[0])
	if result.TaprootKeySpendSighashesHex[0] != expectedSighashHex {
		t.Fatalf(
			"cross-language BIP-341 sighash mismatch: Rust=%s Go=%s txid=%s tx=%s",
			result.TaprootKeySpendSighashesHex[0],
			expectedSighashHex,
			txIDHex,
			result.TxHex,
		)
	}

	// Complete the production ownership path: DKG material remains on the
	// long-lived wallet session, while BuildTaprootTx and the interactive flow
	// share a distinct per-signing ROAST session. With the firewall enabled,
	// Open can succeed only if it reads the BIP-341 artifact from that signing
	// session and resolves only wallet lifecycle/key material through keyGroup.
	walletSessionID := fmt.Sprintf(
		"real-cgo-bip341-wallet-%d",
		realCgoSessionSeq.Add(1),
	)
	const threshold = 2
	const localMember = uint16(1)
	includedMembers := []byte{1, 2}
	keyGroup := runRealCgoDKGKeyGroup(
		t,
		engine,
		walletSessionID,
		[]byte{1, 2, 3},
		threshold,
	)
	message, err := hex.DecodeString(result.TaprootKeySpendSighashesHex[0])
	if err != nil {
		t.Fatalf("decode Rust BIP-341 message: %v", err)
	}
	derived, err := engine.DeriveInteractiveAttemptContext(
		signingSessionID,
		message,
		keyGroup,
		threshold,
		0,
		uint16sOf(includedMembers),
	)
	skipFrostUnavailable(t, "derive policy-bound interactive attempt", err)
	opened, err := engine.InteractiveSessionOpen(
		signingSessionID,
		localMember,
		message,
		keyGroup,
		threshold,
		nil,
		nil,
		derived.AttemptContext,
	)
	skipFrostUnavailable(t, "open policy-bound interactive session", err)
	commitment, err := engine.InteractiveRound1(
		signingSessionID,
		opened.AttemptID,
		localMember,
	)
	skipFrostUnavailable(t, "policy-bound interactive round 1", err)
	if len(commitment) == 0 {
		t.Fatal("policy-bound interactive round 1 returned an empty commitment")
	}
}

// setupRealCgoSignerState sets the linked-signer env the persisted-DKG interactive flow
// needs: the development profile, a fixed state encryption key, and a per-PROCESS state
// path - stable across -count=N (the signer binds its process-global state-file lock to
// the first path and refuses to switch) and unique across processes (so separate runs
// do not contend on one lock). Tests pair it with a unique session id per invocation so
// in-process repeats add a fresh DKG session rather than conflicting on a fixed one.
func setupRealCgoSignerState(t *testing.T) {
	t.Helper()
	t.Setenv("TBTC_SIGNER_PROFILE", "development")
	t.Setenv("TBTC_SIGNER_ENFORCE_PROVENANCE_GATE", "false")
	stateKey := make([]byte, 32)
	for i := range stateKey {
		stateKey[i] = byte(i + 1)
	}
	t.Setenv("TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX", hex.EncodeToString(stateKey))
	stateDir := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("keep-frost-realcgo-state-%d", os.Getpid()),
	)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("create signer state dir: %v", err)
	}
	t.Setenv("TBTC_SIGNER_STATE_PATH", filepath.Join(stateDir, "signer-state"))
}

// isPreMultiSeatConflict reports whether an InteractiveSessionOpen error is the
// SessionConflict a PRE-multi-seat-fix libfrost_tbtc returns when a second LOCAL seat
// opens a session a sibling already opened. The fix (member-keyed interactive_signing)
// makes that Open succeed; against an older but otherwise-linked lib the test skips (a
// stale-lib environment issue, like a missing symbol) rather than failing. Matched on
// the error text - this is test-only environment detection, not production control flow.
func isPreMultiSeatConflict(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "session conflict")
}

// FrostRequireCgoEnvVar, when truthy, turns the "linked lib unavailable" SKIP into a
// FATAL. It is set by the frost-cgo-integration CI gate, which builds a current
// libfrost_tbtc and links it: there, a would-be skip means the lib is absent, stale, or
// failed to load - which must fail the job loudly rather than silently dropping the
// real-crypto coverage the gate exists to provide. Unset (local/normal CI), the tests
// skip as before so they stay inert where the lib is not linked.
const FrostRequireCgoEnvVar = "KEEP_CORE_FROST_REQUIRE_CGO"

func frostCgoRequired() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(FrostRequireCgoEnvVar)), "true")
}

const frostSubprocessSkipPrefixEnv = "KEEP_CORE_FROST_SUBPROCESS_SKIP_PREFIX"

func frostUnavailableSkipMessage(op string, err error) string {
	return fmt.Sprintf(
		"linked tbtc-signer FFI symbol for %s unavailable (lib absent or stale; "+
			"rebuild libfrost_tbtc): %v",
		op, err,
	)
}

func reportFrostSubprocessSkip(op string, err error) bool {
	if err == nil || !errors.Is(err, ErrNativeCryptographyUnavailable) || frostCgoRequired() {
		return false
	}
	if prefix := os.Getenv(frostSubprocessSkipPrefixEnv); prefix != "" {
		fmt.Printf("%s%s\n", prefix, frostUnavailableSkipMessage(op, err))
	}
	return true
}

// skipFrostUnavailable turns an engine-call error into the right outcome: a missing
// FFI symbol (lib absent, or present but stale and missing a newer symbol) SKIPS
// the test naming the operation, while any other error is a real failure. nil is a
// no-op. Centralizing this makes every step robust to an incomplete lib rather than
// only the first (RunDKG) call. Under the require-cgo gate the would-be skip is fatal.
func skipFrostUnavailable(t *testing.T, op string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		if frostCgoRequired() {
			t.Fatalf(
				"%s: tbtc-signer FFI symbol unavailable but %s is set (lib absent, stale, "+
					"or failed to load - the linked libfrost_tbtc must satisfy the bridge): %v",
				op, FrostRequireCgoEnvVar, err,
			)
		}
		if prefix := os.Getenv(frostSubprocessSkipPrefixEnv); prefix != "" {
			fmt.Printf("%s%s\n", prefix, frostUnavailableSkipMessage(op, err))
		}
		t.Skip(frostUnavailableSkipMessage(op, err))
	}
	t.Fatalf("%s: %v", op, err)
}

// runRealCgoDKGKeyGroup runs a real distributed FROST DKG (Part1/2/3 exchanged over
// an in-process bus) for the participants and PERSISTS each seat's key package under
// sessionID via PersistDistributedDKGKeyPackage, returning the keyGroup the
// interactive/ROAST signing path resolves. It replaces the removed trusted-dealer
// RunDKG glue: the go-forward engine has no dealer DKG, so setup now uses the same
// distributed keygen the production node runs, keyed by the interactive session id.
// Participants carry canonical FROST identifiers so the persist op and the
// interactive member lookup agree. It skips the test if the linked tbtc-signer FFI
// symbols are absent.
func runRealCgoDKGKeyGroup(
	t *testing.T,
	engine *buildTaggedTBTCSignerEngine,
	sessionID string,
	participantIDs []byte,
	threshold uint16,
) string {
	t.Helper()

	members := make([]group.MemberIndex, 0, len(participantIDs))
	identifiers := make(map[group.MemberIndex]string, len(participantIDs))
	for _, id := range participantIDs {
		m := group.MemberIndex(id)
		members = append(members, m)
		identifiers[m] = CanonicalFROSTIdentifier(uint16(m))
	}

	priv, pub := dkgTestKeys(t, members)
	bus := NewInProcessDKGBus(len(members) * 8)

	// Construct every runner (and thereby subscribe it to the shared bus) before any
	// starts, so no seat's round-1 broadcast is missed once the bus loops it back.
	runners := make(map[group.MemberIndex]*distributedDKGRunner, len(members))
	for _, m := range members {
		runner, err := newDistributedDKGRunner(
			m, sessionID, members, identifiers, threshold, engine, bus, priv[m], pub[m],
		)
		if err != nil {
			t.Fatalf("new distributed DKG runner (member %d): %v", m, err)
		}
		runners[m] = runner
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	type seatOutcome struct {
		result *NativeFROSTDKGResult
		err    error
	}
	outcomes := make(map[group.MemberIndex]*seatOutcome, len(members))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, m := range members {
		m := m
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := runners[m].Run(ctx)
			mu.Lock()
			outcomes[m] = &seatOutcome{result: result, err: err}
			mu.Unlock()
		}()
	}
	wg.Wait()

	for _, m := range members {
		skipFrostUnavailable(t, fmt.Sprintf("distributed DKG (member %d)", m), outcomes[m].err)
		if outcomes[m].result == nil ||
			outcomes[m].result.KeyPackage == nil ||
			outcomes[m].result.PublicKeyPackage == nil {
			t.Fatalf("member %d produced an incomplete distributed DKG result", m)
		}
	}

	// Persist every seat's key package under sessionID so the interactive signing
	// path resolves each local member's key by session id. The removed dealer RunDKG
	// persisted all seats in one call; the distributed DKG persists per seat.
	var keyGroup string
	for _, m := range members {
		persisted, err := engine.PersistDistributedDKGKeyPackage(
			sessionID,
			uint16(m),
			threshold,
			uint16(len(members)),
			outcomes[m].result.KeyPackage,
			outcomes[m].result.PublicKeyPackage,
		)
		skipFrostUnavailable(
			t,
			fmt.Sprintf("persist distributed DKG key package (member %d)", m),
			err,
		)
		if persisted == nil || persisted.KeyGroup == "" {
			t.Fatalf("member %d persisted DKG result has an empty key group", m)
		}
		if keyGroup == "" {
			keyGroup = persisted.KeyGroup
		} else if persisted.KeyGroup != keyGroup {
			t.Fatalf(
				"member %d persisted a different key group (%s != %s)",
				m, persisted.KeyGroup, keyGroup,
			)
		}
	}
	return keyGroup
}

// uint16sOf widens member ids to the uint16 participant list the engine's
// DeriveInteractiveAttemptContext expects.
func uint16sOf(members []byte) []uint16 {
	out := make([]uint16, 0, len(members))
	for _, member := range members {
		out = append(out, uint16(member))
	}
	return out
}

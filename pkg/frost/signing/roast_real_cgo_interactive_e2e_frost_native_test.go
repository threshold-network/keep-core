//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/operator"
)

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
// The DKG -> interactive keyGroup glue is RunDKG: InteractiveSessionOpen resolves
// the signing key by a keyGroup IDENTIFIER (engine-internal persisted material),
// not KeyPackage bytes. RunDKG runs the full DKG and PERSISTS the result, keyed by
// the SESSION ID, under a returned keyGroup the interactive path resolves - the
// same flow production uses. Open then requires a completed DKG session of the
// same session_id, so RunDKG and the interactive flow share one session id.
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
	t.Setenv("TBTC_SIGNER_PROFILE", "development")
	t.Setenv("TBTC_SIGNER_ENFORCE_PROVENANCE_GATE", "false")
	// RunDKG persists the DKG result in the signer's ENCRYPTED state, so a linked
	// signer needs a state encryption key and an ISOLATED, fresh state path. Without
	// them a clean linked environment fails before signing (missing key), and a
	// shared/default state path makes reruns conflict on the fixed session id.
	// t.TempDir() yields a fresh path per run, so each run starts clean.
	stateKey := make([]byte, 32)
	for i := range stateKey {
		stateKey[i] = byte(i + 1)
	}
	t.Setenv("TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX", hex.EncodeToString(stateKey))
	t.Setenv("TBTC_SIGNER_STATE_PATH", filepath.Join(t.TempDir(), "signer-state"))

	engine := &buildTaggedTBTCSignerEngine{}

	// The engine requires threshold >= 2; the attempt's included set is the t-subset
	// {1,2} over a 3-party DKG. This process drives the one local signer (member 1).
	const threshold = 2
	// One session id for the whole flow: the engine keys the interactive session by
	// the DKG session id, so RunDKG, derive, and open all use sessionID.
	const sessionID = "real-cgo-session-1"
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

// skipFrostUnavailable turns an engine-call error into the right outcome: a missing
// FFI symbol (lib absent, or present but stale and missing a newer symbol) SKIPS
// the test naming the operation, while any other error is a real failure. nil is a
// no-op. Centralizing this makes every step robust to an incomplete lib rather than
// only the first (RunDKG) call.
func skipFrostUnavailable(t *testing.T, op string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Skipf(
			"linked tbtc-signer FFI symbol for %s unavailable (lib absent or stale; "+
				"rebuild libfrost_tbtc): %v",
			op, err,
		)
	}
	t.Fatalf("%s: %v", op, err)
}

// runRealCgoDKGKeyGroup runs a full real FROST DKG over the participants via
// RunDKG under sessionID, which persists the result (keyed by that session id) and
// returns the keyGroup the signing path resolves. It skips the test if the linked
// tbtc-signer FFI symbols are absent. Each participant carries a freshly generated
// operator public key (the DKG's per-participant identifying key), so the request
// is well-formed.
func runRealCgoDKGKeyGroup(
	t *testing.T,
	engine *buildTaggedTBTCSignerEngine,
	sessionID string,
	participantIDs []byte,
	threshold uint16,
) string {
	t.Helper()

	participants := make([]NativeTBTCSignerDKGParticipant, 0, len(participantIDs))
	for _, id := range participantIDs {
		_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (participant %d): %v", id, err)
		}
		participants = append(participants, NativeTBTCSignerDKGParticipant{
			Identifier:   uint16(id),
			PublicKeyHex: hex.EncodeToString(operator.MarshalUncompressed(publicKey)),
		})
	}

	result, err := engine.RunDKG(sessionID, participants, threshold)
	skipFrostUnavailable(t, "run DKG", err)
	if result.KeyGroup == "" {
		t.Fatal("RunDKG returned an empty key group")
	}
	return result.KeyGroup
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

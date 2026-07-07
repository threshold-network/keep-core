//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This is the end-to-end proof that a DISTRIBUTED FROST DKG produces usable
// signing material through the persist bridge. The distributed-DKG orchestrator
// test (distributed_dkg_runner_real_cgo_frost_native_test.go) signs via the
// STATELESS path (passing each seat's raw KeyPackage bytes straight into Sign),
// which never touches persisted engine state. Production signs via the
// INTERACTIVE path, which loads the key from the engine's persisted session
// state (dkg_key_packages by member + the public key package) - material the
// dealer RunDKG persists but distributed Part3 previously discarded.
//
// So this test drives the real chain the tBTC node needs:
//
//	distributed DKG (bus-orchestrated Part1/2/3)
//	  -> PersistDistributedDKGKeyPackage  (the bridge under test)
//	  -> InteractiveSessionOpen / Round1 / Round2 / Aggregate
//	  -> a BIP-340 signature that verifies under the DKG group key.
//
// Crucially, each seat runs on its OWN engine with its OWN persisted state -
// exactly how nodes deploy, and unlike the dealer path (and the multi-seat
// interactive test) where every key package lives in one engine. A distributed
// node holds only its own secret key package; that this still opens, releases a
// share, and aggregates into a valid threshold signature is the whole point.
func TestDistributedDKG_PersistThenInteractiveSign(t *testing.T) {
	setupRealCgoSignerState(t)

	const n = 3
	const threshold uint16 = 2
	members := []group.MemberIndex{1, 2, 3}
	sessionID := fmt.Sprintf("real-cgo-distributed-persist-%d", realCgoSessionSeq.Add(1))
	message := bytesOf(0x42, 32)

	// Per-seat engines: each node has its OWN engine + persisted state, as
	// production deploys them. (The dealer path and the multi-seat interactive
	// test put every key package in one engine; a real distributed node does not.)
	engines := make(map[group.MemberIndex]*buildTaggedTBTCSignerEngine, n)
	for _, m := range members {
		engines[m] = &buildTaggedTBTCSignerEngine{}
	}

	// CANONICAL FROST identifiers: participant_identifier_to_frost_identifier(m)
	// serializes the scalar m big-endian (member index in the LEAST-significant
	// byte). Unlike the byte-0 scheme the stateless DKG tests use, these must be
	// canonical here because the persist op re-derives each seat's identifier from
	// its participant index and rejects a mismatch, and the interactive signing
	// path looks members up by the same canonical identifier.
	canonicalFrostIdentifier := func(m byte) string {
		id := make([]byte, 32)
		id[31] = m
		return fmt.Sprintf("%q", hex.EncodeToString(id))
	}
	identifiers := make(map[group.MemberIndex]string, n)
	for _, m := range members {
		identifiers[m] = canonicalFrostIdentifier(byte(m))
	}

	bus := NewInProcessDKGBus(64)
	priv, pub := dkgTestKeys(t, members)

	// ---- Distributed DKG across the per-seat engines. ----
	runners := make(map[group.MemberIndex]*distributedDKGRunner, n)
	for _, m := range members {
		runner, err := newDistributedDKGRunner(
			m, sessionID, members, identifiers, threshold, engines[m], bus, priv[m], pub[m],
		)
		if err != nil {
			t.Fatalf("new runner (member %d): %v", m, err)
		}
		runners[m] = runner
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	type seatOutcome struct {
		result *NativeFROSTDKGResult
		err    error
	}
	outcomes := make(map[group.MemberIndex]*seatOutcome, n)
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
		if errors.Is(outcomes[m].err, ErrNativeCryptographyUnavailable) {
			t.Skip("linked tbtc-signer FFI symbols unavailable")
		}
	}
	for _, m := range members {
		if outcomes[m].err != nil {
			t.Fatalf("member %d distributed DKG failed: %v", m, outcomes[m].err)
		}
		if outcomes[m].result == nil || outcomes[m].result.KeyPackage == nil ||
			outcomes[m].result.PublicKeyPackage == nil {
			t.Fatalf("member %d produced an incomplete DKG result", m)
		}
	}
	groupVerifyingKey := outcomes[members[0]].result.PublicKeyPackage.VerifyingKey

	// ---- The bridge under test: persist each seat's OWN key package into its
	// OWN engine, making a distributed-DKG share loadable for interactive signing.
	// Every seat must derive the SAME key group, equal to the DKG group key. ----
	var keyGroup string
	for _, m := range members {
		persisted, err := engines[m].PersistDistributedDKGKeyPackage(
			sessionID,
			uint16(m),
			threshold,
			uint16(n),
			outcomes[m].result.KeyPackage,
			outcomes[m].result.PublicKeyPackage,
		)
		if err != nil {
			t.Fatalf("persist distributed DKG key package (member %d): %v", m, err)
		}
		if keyGroup == "" {
			keyGroup = persisted.KeyGroup
		} else if persisted.KeyGroup != keyGroup {
			t.Fatalf("member %d persisted a different key group: %s != %s", m, persisted.KeyGroup, keyGroup)
		}
	}
	// The persisted key group is the 33-byte COMPRESSED verifying key - the form
	// run_dkg stores and the signing path consumes (even-Y taproot key, so a "02"
	// prefix); the DKG result exposes the 32-byte x-only key. They must denote the
	// same key.
	if len(keyGroup) != 66 || keyGroup[2:] != groupVerifyingKey {
		t.Fatalf("persisted key group %s does not match DKG group verifying key %s", keyGroup, groupVerifyingKey)
	}

	// ---- Interactive threshold signing over the persisted material: any t of n. ----
	signingMembers := members[:threshold] // {1, 2}
	includedParticipants := make([]uint16, len(signingMembers))
	for i, m := range signingMembers {
		includedParticipants[i] = uint16(m)
	}

	// The attempt context is deterministic; derive it once (any engine agrees).
	derived, err := engines[signingMembers[0]].DeriveInteractiveAttemptContext(
		sessionID, message, keyGroup, threshold, 0, includedParticipants,
	)
	if err != nil {
		t.Fatalf("derive interactive attempt context: %v", err)
	}
	frostIDByMember := make(map[group.MemberIndex]string, len(derived.FrostIdentifiers))
	for _, id := range derived.FrostIdentifiers {
		frostIDByMember[group.MemberIndex(id.ParticipantIdentifier)] = id.FrostIdentifier
	}

	// Open + Round1 for each signing seat ON ITS OWN engine (its persisted state).
	var attemptID string
	commitments := make([]nativeFROSTCommitment, 0, len(signingMembers))
	for _, m := range signingMembers {
		open, err := engines[m].InteractiveSessionOpen(
			sessionID, uint16(m), message, keyGroup, threshold, nil, derived.AttemptContext,
		)
		if err != nil {
			t.Fatalf("interactive session open (member %d): %v", m, err)
		}
		if attemptID == "" {
			attemptID = open.AttemptID
		} else if open.AttemptID != attemptID {
			t.Fatalf("seats derived different attempt ids (%q vs %q)", attemptID, open.AttemptID)
		}
		commitmentData, err := engines[m].InteractiveRound1(sessionID, open.AttemptID, uint16(m))
		if err != nil {
			t.Fatalf("interactive round 1 (member %d): %v", m, err)
		}
		commitments = append(commitments, nativeFROSTCommitment{
			Identifier: frostIDByMember[m],
			Data:       commitmentData,
		})
	}

	// The coordinator assembles the signing package (stateless).
	signingPackage, err := engines[signingMembers[0]].NewSigningPackage(message, commitments)
	if err != nil {
		t.Fatalf("new signing package: %v", err)
	}

	// Round2 for each signing seat ON ITS OWN engine: each releases its share from
	// its own persisted key package.
	shares := make([]nativeFROSTSignatureShare, 0, len(signingMembers))
	for _, m := range signingMembers {
		shareData, err := engines[m].InteractiveRound2(sessionID, attemptID, uint16(m), signingPackage)
		if err != nil {
			t.Fatalf("interactive round 2 (member %d): %v", m, err)
		}
		shares = append(shares, nativeFROSTSignatureShare{
			Identifier: frostIDByMember[m],
			Data:       shareData,
		})
	}

	// The coordinator aggregates the shares into a BIP-340 signature.
	signatureBytes, err := engines[signingMembers[0]].InteractiveAggregate(
		sessionID, attemptID, signingPackage, shares, nil,
	)
	if err != nil {
		t.Fatalf("interactive aggregate: %v", err)
	}
	if len(signatureBytes) != 64 {
		t.Fatalf("aggregate signature length = %d, want 64", len(signatureBytes))
	}

	// ---- GOLD: the interactive threshold signature verifies under the DKG group
	// key - proving the persisted distributed-DKG shares sign correctly. ----
	// Verify against the 32-byte x-only group key (schnorr/BIP-340 form).
	groupKeyBytes, err := hex.DecodeString(groupVerifyingKey)
	if err != nil {
		t.Fatalf("decode group verifying key: %v", err)
	}
	publicKey, err := schnorr.ParsePubKey(groupKeyBytes)
	if err != nil {
		t.Fatalf("parse group key: %v", err)
	}
	signature, err := schnorr.ParseSignature(signatureBytes)
	if err != nil {
		t.Fatalf("parse signature: %v", err)
	}
	if !signature.Verify(message, publicKey) {
		t.Fatal("interactive threshold signature does not verify under the distributed-DKG group key")
	}
	t.Logf(
		"distributed DKG -> persist -> interactive %d-of-%d signature verifies under group key %s…",
		threshold, n, keyGroup[:16],
	)
}

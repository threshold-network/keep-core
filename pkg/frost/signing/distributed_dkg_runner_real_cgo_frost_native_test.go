//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This test proves the FIRST increment of the distributed-DKG wiring
// (distributed_dkg_runner_frost_native.go): a per-member, BUS-DRIVEN orchestrator
// drives the real three-round FROST DKG so that n nodes, each contributing their
// own secret entropy and exchanging only round packages over a message bus, end
// up agreeing on ONE group key while each holds a DISTINCT secret share that no
// node ever saw in full. This is the shape the tBTC node needs to replace the
// transitional trusted-dealer keygen (RunDKGWithSeed), which the Rust signer
// hard-disables in production ("production requires distributed DKG wiring").
//
// It is the message-passing analogue of the inline single-goroutine DKG in
// native_frost_engine_..._test.go: the crypto (Part1/Part2/Part3) is identical,
// but here each seat is an independent goroutine that only learns its peers'
// contributions through the bus - so a passing run exercises the ORCHESTRATION
// (round-1 broadcast, round-2 per-recipient routing, round collection), not just
// the engine. Part3 cryptographically verifies each round-2 package against its
// sender's round-1 commitment, so a misrouted or dropped package fails the run;
// a passing run therefore proves the routing is correct. The final threshold
// signature, verified against the group key, proves the shares are a real,
// functional t-of-n key rather than unrelated per-node material.

func TestDistributedDKGRunner_ThreeSeatsAgreeOnGroupKeyWithDistinctShares(t *testing.T) {
	setupRealCgoSignerState(t)

	const n = 3
	const threshold uint16 = 2
	members := []group.MemberIndex{1, 2, 3}

	engine := &buildTaggedTBTCSignerEngine{}

	// Deterministic per-member FROST identifiers (byte-0 = member index), matching
	// the scheme the engine's DKG tests use. Production supplies identifiers
	// aligned with the signing path; the orchestrator takes them as input.
	identifiers := make(map[group.MemberIndex]string, n)
	for _, m := range members {
		identifiers[m] = buildTaggedTBTCSignerTestIdentifier(byte(m))
	}

	bus := NewInProcessDKGBus(64)

	// Per-member keys standing in for operator keys: round-2 shares are SEALED to
	// the recipient's key and opened with the recipient's own, so this exercises
	// the encrypted round-2 path end to end.
	pub, priv := dkgTestKeys(t, members)

	// Build every runner (each subscribes to the bus) BEFORE starting any, so no
	// peer's round-1 broadcast is missed.
	runners := make(map[group.MemberIndex]*distributedDKGRunner, n)
	for _, m := range members {
		runner, err := newDistributedDKGRunner(m, members, identifiers, threshold, engine, bus, pub, priv[m])
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

	// Skip cleanly if the linked signer lib is unavailable (matches the sibling
	// real-cgo tests); any other failure is real.
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

	// ---- All seats agree on ONE group key. ----
	groupKey := outcomes[members[0]].result.PublicKeyPackage.VerifyingKey
	if len(groupKey) != 64 {
		t.Fatalf("group verifying key must be a 32-byte x-only key (64 hex chars); got %d chars", len(groupKey))
	}
	for _, m := range members {
		pkp := outcomes[m].result.PublicKeyPackage
		if pkp.VerifyingKey != groupKey {
			t.Fatalf("member %d disagreed on the group key: %s != %s", m, pkp.VerifyingKey, groupKey)
		}
		if len(pkp.VerifyingShares) != n {
			t.Fatalf("member %d has %d verifying shares, want %d", m, len(pkp.VerifyingShares), n)
		}
	}

	// ---- Each seat holds a DISTINCT secret share bound to its own identifier
	// (no dealer broadcasting one key). Because each seat only ran Part1/2/3 with
	// its OWN secret packages plus bus-received packages, no seat ever held the
	// full secret - the distributed property, by construction. ----
	for _, m := range members {
		kp := outcomes[m].result.KeyPackage
		if kp.Identifier != identifiers[m] {
			t.Fatalf("member %d key package identifier = %q, want %q", m, kp.Identifier, identifiers[m])
		}
		if len(kp.Data) == 0 {
			t.Fatalf("member %d has an empty key package", m)
		}
	}
	for i := 0; i < len(members); i++ {
		for j := i + 1; j < len(members); j++ {
			a, b := members[i], members[j]
			if bytes.Equal(outcomes[a].result.KeyPackage.Data, outcomes[b].result.KeyPackage.Data) {
				t.Fatalf("members %d and %d hold identical key packages; shares must be distinct", a, b)
			}
		}
	}
	t.Logf("3 seats agreed on group key %s… with 3 distinct shares (bus-orchestrated distributed DKG)", groupKey[:16])

	// ---- GOLD: a real t-of-n threshold signature over the DKG output verifies
	// against the group key, proving the bus-orchestrated shares are a functional
	// threshold key (not just distinct blobs). ----
	signers := members[:threshold] // any t of the n
	message := bytesOf(0x42, 32)

	commitments := make([]nativeFROSTCommitment, 0, len(signers))
	noncesBySigner := make(map[group.MemberIndex][]byte, len(signers))
	for _, m := range signers {
		kp := outcomes[m].result.KeyPackage
		nonces, commitmentIdentifier, commitmentData, err := engine.GenerateNoncesAndCommitments(kp.Identifier, kp.Data)
		if err != nil {
			t.Fatalf("member %d nonce generation: %v", m, err)
		}
		commitments = append(commitments, nativeFROSTCommitment{Identifier: commitmentIdentifier, Data: commitmentData})
		noncesBySigner[m] = nonces
	}

	signingPackage, err := engine.NewSigningPackage(message, commitments)
	if err != nil {
		t.Fatalf("new signing package: %v", err)
	}

	shares := make([]nativeFROSTSignatureShare, 0, len(signers))
	for _, m := range signers {
		kp := outcomes[m].result.KeyPackage
		shareIdentifier, shareData, err := engine.Sign(signingPackage, noncesBySigner[m], kp.Identifier, kp.Data)
		if err != nil {
			t.Fatalf("member %d sign: %v", m, err)
		}
		shares = append(shares, nativeFROSTSignatureShare{Identifier: shareIdentifier, Data: shareData})
	}

	signatureBytes, err := engine.Aggregate(signingPackage, shares, outcomes[members[0]].result.PublicKeyPackage)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(signatureBytes) != 64 {
		t.Fatalf("aggregate signature length = %d, want 64", len(signatureBytes))
	}

	groupKeyBytes, err := hex.DecodeString(groupKey)
	if err != nil {
		t.Fatalf("decode group key: %v", err)
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
		t.Fatal("threshold signature does not verify under the distributed-DKG group key")
	}
	t.Logf("threshold signature by %d-of-%d verifies under the distributed-DKG group key", threshold, n)
}

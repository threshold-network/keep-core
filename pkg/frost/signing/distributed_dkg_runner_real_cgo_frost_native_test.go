//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This test proves the FIRST increment of the distributed-DKG wiring
// (distributed_dkg_runner_frost_native.go): a per-member, BUS-DRIVEN orchestrator
// drives the real three-round FROST DKG so that n nodes, each contributing their
// own secret entropy and exchanging only round packages over a message bus, end
// up agreeing on ONE group key while each holds a DISTINCT secret share that no
// node ever saw in full. This is the shape the tBTC node uses now that the
// transitional trusted-dealer keygen has been removed.
//
// It is the message-passing analogue of the inline single-goroutine DKG in
// native_frost_engine_..._test.go: the crypto (Part1/Part2/Part3) is identical,
// but here each seat is an independent goroutine that only learns its peers'
// contributions through the bus - so a passing run exercises the ORCHESTRATION
// (round-1 broadcast, round-2 per-recipient routing, round collection), not just
// the engine. Part3 cryptographically verifies each round-2 package against its
// sender's round-1 commitment, so a misrouted or dropped package fails the run;
// a passing run therefore proves the routing is correct, and that the seats agree
// on one group key while each holds a distinct secret share.

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

	// Per-member keys standing in for operator keys: each seat stamps its public
	// key on round 1, peers LEARN it, and round-2 shares are sealed to it and
	// opened with the seat's own private key - the full encrypted round-2 path.
	priv, pub := dkgTestKeys(t, members)

	// Build every runner (each subscribes to the bus) BEFORE starting any, so no
	// peer's round-1 broadcast is missed.
	runners := make(map[group.MemberIndex]*distributedDKGRunner, n)
	for _, m := range members {
		runner, err := newDistributedDKGRunner(m, testDKGSession, members, identifiers, threshold, engine, bus, priv[m], pub[m])
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

	// NOTE: this test previously followed the DKG with a standalone t-of-n FROST
	// threshold signature (engine.GenerateNoncesAndCommitments/Sign/Aggregate) to
	// prove the shares form a functional key. Those standalone FROST-primitive
	// engine ops were removed together with the coarse-FROST signing path; signing
	// over a DKG'd key is now exercised only through the interactive ROAST path
	// (see the interactive-signing tests). The assertions above still prove the
	// distributed property: 3 seats agree on ONE group key while each holds a
	// DISTINCT secret share, and Part3 cryptographically verifies every round-2
	// package against its sender's round-1 commitment, so a passing run proves the
	// orchestration and share structure are correct.
}

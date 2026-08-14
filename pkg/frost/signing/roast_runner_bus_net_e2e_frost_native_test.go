//go:build frost_native

package signing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	netlocal "github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// netSigningHarness wires n interactive signing runners over n REAL pkg/net
// BroadcastChannel runner buses (one operator per seat, all on one in-memory
// network) so a full interactive signing round flows through the production
// transport adapter (broadcastChannelRunnerBus / NewBroadcastChannelRunnerBus)
// end-to-end - exercising wire serialization, the claimed-seat<->operator-key
// authentication, the per-type demux, and bounded delivery - rather than the
// in-process test bus the rest of the runner suite uses.
type netSigningHarness struct {
	runners []*interactiveSigningRunner
	coords  []roast.Coordinator
	handles []roast.AttemptHandle
}

// buildInteractiveSigningNetHarness builds an n-seat interactive signing round
// over the real pkg/net transport. Seat i (1-based) is held by operator i, and
// the MembershipValidator maps each seat to that operator's address so the
// adapter authenticates every broadcast's claimed seat against the authenticated
// sender key. The engine is the deterministic fake (no cgo / no real FROST); this
// validates the TRANSPORT, not the crypto, which the engine suite covers.
func buildInteractiveSigningNetHarness(
	t *testing.T,
	ctx context.Context,
	n int,
	threshold uint16,
) netSigningHarness {
	t.Helper()

	// The runner's aggregate path releases results only under an active outer
	// memo session owner (the production executor holds one across all local
	// seats); the harness owns it for the round.
	memoSession, err := BeginInteractiveAggregateMemoSession("session-net-1")
	if err != nil {
		t.Fatalf("begin aggregate memo session: %v", err)
	}
	t.Cleanup(memoSession.Release)

	included := make([]group.MemberIndex, 0, n)
	for i := 1; i <= n; i++ {
		included = append(included, group.MemberIndex(i))
	}

	// One operator per seat. The same chain signing maps an operator public key to
	// the address the MembershipValidator checks, so the validator and the
	// per-operator net providers agree on who holds each seat.
	chainSigning := local_v1.Connect(n, n).Signing()
	publicKeys := make([]*operator.PublicKey, n)
	addresses := make([]chain.Address, n)
	for i := 0; i < n; i++ {
		_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("operator key (seat %d): %v", i+1, err)
		}
		publicKeys[i] = publicKey
		addresses[i] = chainSigning.PublicKeyBytesToAddress(
			operator.MarshalUncompressed(publicKey),
		)
	}
	validator := group.NewMembershipValidator(&testutils.MockLogger{}, addresses, chainSigning)

	dkgKey := []byte{0x01, 0x02}
	attemptCtx, err := attempt.NewAttemptContext(
		"session-net-1", "key-group-1", dkgKey,
		[attempt.MessageDigestLength]byte{0x42}, 0, included, nil,
	)
	if err != nil {
		t.Fatalf("attempt context: %v", err)
	}

	signer := fixedTestSigner{}
	verifier := roast.NoOpSignatureVerifier()
	logger := &testutils.MockLogger{}

	h := netSigningHarness{}
	for i := 0; i < n; i++ {
		member := group.MemberIndex(i + 1)

		// Each seat broadcasts on its OWN provider+channel; same-named channels on
		// the in-memory network are interconnected, so a broadcast reaches every
		// seat. The provider stamps this operator's key as the authenticated
		// sender, which is what the adapter binds the claimed seat to.
		channel, err := netlocal.ConnectWithKey(publicKeys[i]).
			BroadcastChannelFor("frost-roast-interactive-signing")
		if err != nil {
			t.Fatalf("broadcast channel (seat %d): %v", member, err)
		}
		bus, err := NewBroadcastChannelRunnerBus(ctx, logger, channel, validator)
		if err != nil {
			t.Fatalf("runner bus (seat %d): %v", member, err)
		}

		coord := roast.NewInMemoryCoordinatorWithSigning(member, signer, verifier)
		handle, err := coord.BeginAttempt(attemptCtx)
		if err != nil {
			t.Fatalf("begin attempt (seat %d): %v", member, err)
		}
		ara, err := NewActiveRoastAttempt(coord, handle, attemptCtx, "session-net-1", nil, dkgKey)
		if err != nil {
			t.Fatalf("active attempt (seat %d): %v", member, err)
		}
		engine := newFakeInteractiveSigningEngine()
		// The fake engine's derivation must agree with the binding's RFC-21
		// election (a real engine derives the same coordinator).
		engine.coordinatorIdentifier = uint16(ara.ElectedCoordinator())
		collector := roast.NewRound2Collector(verifier)
		runner, err := newInteractiveSigningRunner(
			ara, member, threshold, nil, engine, collector, coord, signer, bus,
		)
		if err != nil {
			t.Fatalf("runner (seat %d): %v", member, err)
		}

		h.coords = append(h.coords, coord)
		h.handles = append(h.handles, handle)
		h.runners = append(h.runners, runner)
	}
	return h
}

// runAllAndAssertSucceed runs every seat's runner concurrently and asserts each
// returns the signature and transitions its attempt to Succeeded.
func (h netSigningHarness) runAllAndAssertSucceed(t *testing.T, ctx context.Context) {
	t.Helper()
	sigs := make([][]byte, len(h.runners))
	errs := make([]error, len(h.runners))
	var wg sync.WaitGroup
	for i := range h.runners {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sigs[idx], errs[idx] = h.runners[idx].Run(ctx)
		}(i)
	}
	wg.Wait()

	for i := range h.runners {
		member := i + 1
		if errs[i] != nil {
			t.Fatalf("seat %d run failed over the net transport: %v", member, errs[i])
		}
		if string(sigs[i]) != "fake-bip340-signature" {
			t.Fatalf("seat %d unexpected signature: %q", member, sigs[i])
		}
		state, err := h.coords[i].State(h.handles[i])
		if err != nil {
			t.Fatalf("seat %d state: %v", member, err)
		}
		if state != roast.AttemptStateSucceeded {
			t.Fatalf("seat %d: expected Succeeded, got %v", member, state)
		}
	}
}

// TestInteractiveSigningRunner_NetTransport_FullIncludedRound runs a complete
// interactive signing round over the REAL pkg/net transport with a full-included
// attempt (group size == threshold == 2, every seat signs). It proves the round
// completes when every RunnerMessage type (commitments, signing package, shares)
// is serialized, authenticated by seat, demuxed, and delivered by the production
// adapter rather than the in-process bus.
func TestInteractiveSigningRunner_NetTransport_FullIncludedRound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	buildInteractiveSigningNetHarness(t, ctx, 2, 2).runAllAndAssertSucceed(t, ctx)
}

// TestInteractiveSigningRunner_NetTransport_ThresholdSubsetRound runs the round
// over the real transport with an oversized included set (group size 3, threshold
// 2): the coordinator finalizes over a t-subset and the remaining committed seat
// is an observer (RFC-21 Phase 7.3 t-of-included). Every seat still obtains the
// signature and reaches Succeeded, proving the subset/observer flow works across
// the production transport, not only the in-process bus.
func TestInteractiveSigningRunner_NetTransport_ThresholdSubsetRound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	buildInteractiveSigningNetHarness(t, ctx, 3, 2).runAllAndAssertSucceed(t, ctx)
}

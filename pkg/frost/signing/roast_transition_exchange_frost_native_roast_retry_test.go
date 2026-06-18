//go:build frost_native && frost_roast_retry

package signing

import (
	"context"
	"crypto/sha256"
	"sync"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// captureBus is a RunnerBus that records broadcasts so a test can deliver them
// to peers by hand -- driving the exchange synchronously, with no listener
// goroutine or delivery timing. Subscribe returns an undrained subscriber; the
// tests cancel the exchange ctx so listen() exits immediately and the test calls
// onSnapshot/onBundle directly.
type captureBus struct {
	mu         sync.Mutex
	broadcasts []RunnerMessage
}

func (b *captureBus) Broadcast(msg RunnerMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Own the payload so a later mutation cannot change what we captured.
	cloned := msg
	cloned.Payload = append([]byte(nil), msg.Payload...)
	b.broadcasts = append(b.broadcasts, cloned)
}

func (b *captureBus) Subscribe() *RunnerBusSubscriber {
	return &RunnerBusSubscriber{
		commitments:       make(chan RunnerMessage, 1),
		signingPackages:   make(chan RunnerMessage, 1),
		shares:            make(chan RunnerMessage, 1),
		evidenceSnapshots: make(chan RunnerMessage, 1),
		transitionBundles: make(chan RunnerMessage, 1),
		seen:              map[[sha256.Size]byte]struct{}{},
	}
}

func (b *captureBus) only(t RunnerMessageType) []RunnerMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]RunnerMessage, 0, len(b.broadcasts))
	for _, m := range b.broadcasts {
		if m.Type == t {
			out = append(out, m)
		}
	}
	return out
}

// fixedSigner returns a fixed non-empty signature. The wire encoders reject an
// empty signature, so the NoOp signer (which returns nil) cannot be used here;
// the NoOp verifier still accepts this fixed signature, so the test exercises the
// exchange flow without a real signature pipeline.
type fixedSigner struct{}

func (fixedSigner) Sign(_ []byte) ([]byte, error) { return []byte{0x01}, nil }

type exchangeNode struct {
	member group.MemberIndex
	coord  roast.Coordinator
	bus    *captureBus
	ex     *RoastTransitionExchange
}

// newExchangeTestNodes builds one node per included member: each gets its own
// in-memory coordinator, begins the SAME attempt context (so all nodes elect the
// same coordinator deterministically), records its observe binding, and wires an
// exchange over its own capture bus with an already-cancelled ctx (no listener).
func newExchangeTestNodes(
	t *testing.T,
	roastSessionID string,
	ctx attempt.AttemptContext,
	dkgKey []byte,
) map[group.MemberIndex]*exchangeNode {
	t.Helper()
	hash := ctx.Hash()
	nodes := map[group.MemberIndex]*exchangeNode{}
	exchangeCtx, cancel := context.WithCancel(context.Background())
	cancel() // listen() exits immediately; the tests drive methods directly.
	for _, m := range ctx.IncludedSet {
		coord := roast.NewInMemoryCoordinatorWithSigning(
			m, fixedSigner{}, roast.NoOpSignatureVerifier(),
		)
		handle, err := coord.BeginAttempt(ctx)
		if err != nil {
			t.Fatalf("begin attempt for member %d: %v", m, err)
		}
		recordObservedAttempt(roastSessionID, m, hash, observedAttemptBinding{
			handle:            handle,
			context:           ctx,
			dkgGroupPublicKey: dkgKey,
		})
		bus := &captureBus{}
		ex := NewRoastTransitionExchange(
			exchangeCtx,
			log.Logger("exchange-test"),
			bus,
			RoastRetryDeps{
				Coordinator: coord,
				Signer:      fixedSigner{},
				Verifier:    roast.NoOpSignatureVerifier(),
				SelfMember:  uint32(m),
			},
			roastSessionID,
			m,
		)
		nodes[m] = &exchangeNode{member: m, coord: coord, bus: bus, ex: ex}
	}
	return nodes
}

func newExchangeTestContext(
	t *testing.T,
	roastSessionID string,
	included []group.MemberIndex,
	dkgKey []byte,
) attempt.AttemptContext {
	t.Helper()
	var digest [attempt.MessageDigestLength]byte
	digest[0] = 0xaa
	ctx, err := attempt.NewAttemptContextWithParking(
		roastSessionID, "exchange-key-group", dkgKey, digest, 0, included, nil, nil,
	)
	if err != nil {
		t.Fatalf("build attempt context: %v", err)
	}
	return ctx
}

// TestRoastTransitionExchange_ProducesRecordsAcrossNodes drives the full
// failed-attempt exchange by hand: every seat broadcasts a forced snapshot, the
// elected coordinator collects them and aggregates+broadcasts the bundle, and
// every seat ends with a verified next-attempt transition record.
func TestRoastTransitionExchange_ProducesRecordsAcrossNodes(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	roastSessionID := "exchange-records-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x01, 0x02, 0x03}

	ctx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := ctx.Hash()
	nodes := newExchangeTestNodes(t, roastSessionID, ctx, dkgKey)

	// Find the deterministically elected coordinator.
	var elected group.MemberIndex
	for _, n := range nodes {
		binding, ok := observedAttempt(roastSessionID, n.member, hash)
		if !ok {
			t.Fatalf("missing observe binding for member %d", n.member)
		}
		e, err := n.coord.SelectedCoordinator(binding.handle)
		if err != nil {
			t.Fatalf("selected coordinator: %v", err)
		}
		elected = e
		break
	}

	// 1. Every participating seat broadcasts a forced snapshot.
	for _, m := range included {
		nodes[m].ex.BroadcastForcedSnapshot(hash)
	}

	// 2. Deliver each seat's snapshot to the elected coordinator's onSnapshot.
	for _, m := range included {
		if m == elected {
			continue // elected already recorded its own in BroadcastForcedSnapshot
		}
		snaps := nodes[m].bus.only(RunnerMsgEvidenceSnapshot)
		if len(snaps) != 1 {
			t.Fatalf("member %d expected to broadcast 1 snapshot, got %d", m, len(snaps))
		}
		nodes[elected].ex.onSnapshot(snaps[0])
	}

	// 3. The elected coordinator aggregates + broadcasts the bundle (a no-op on
	// the others).
	for _, m := range included {
		nodes[m].ex.AggregateAndBroadcast(hash)
	}

	bundles := nodes[elected].bus.only(RunnerMsgTransitionBundle)
	if len(bundles) != 1 {
		t.Fatalf("elected coordinator must broadcast exactly one bundle, got %d", len(bundles))
	}
	for _, m := range included {
		if m == elected {
			continue
		}
		if got := nodes[m].bus.only(RunnerMsgTransitionBundle); len(got) != 0 {
			t.Fatalf("non-elected member %d must not broadcast a bundle, got %d", m, len(got))
		}
	}

	// 4. Deliver the bundle to every other seat's onBundle.
	for _, m := range included {
		if m == elected {
			continue
		}
		nodes[m].ex.onBundle(bundles[0])
	}

	// 5. Every seat must now hold a verified transition record, and its observe
	// binding must be cleared (consumed).
	for _, m := range included {
		if _, ok := RoastTransitionForSession(roastSessionID, m); !ok {
			t.Fatalf("member %d must hold a transition record after the exchange", m)
		}
		if _, ok := observedAttempt(roastSessionID, m, hash); ok {
			t.Fatalf("member %d observe binding must be cleared after storing the record", m)
		}
	}
}

// TestRoastTransitionExchange_NonElectedDoesNotAggregate asserts a seat that is
// not the elected coordinator produces no bundle and stores no record from
// AggregateAndBroadcast (only the elected seat aggregates).
func TestRoastTransitionExchange_NonElectedDoesNotAggregate(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	roastSessionID := "exchange-nonelected-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x04, 0x05, 0x06}

	ctx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := ctx.Hash()
	nodes := newExchangeTestNodes(t, roastSessionID, ctx, dkgKey)

	var elected group.MemberIndex
	binding, _ := observedAttempt(roastSessionID, 1, hash)
	elected, _ = nodes[1].coord.SelectedCoordinator(binding.handle)

	var nonElected group.MemberIndex
	for _, m := range included {
		if m != elected {
			nonElected = m
			break
		}
	}

	nodes[nonElected].ex.BroadcastForcedSnapshot(hash)
	nodes[nonElected].ex.AggregateAndBroadcast(hash)

	if got := nodes[nonElected].bus.only(RunnerMsgTransitionBundle); len(got) != 0 {
		t.Fatalf("non-elected seat must not broadcast a bundle, got %d", len(got))
	}
	if _, ok := RoastTransitionForSession(roastSessionID, nonElected); ok {
		t.Fatal("non-elected seat must not store a transition record from AggregateAndBroadcast")
	}
}

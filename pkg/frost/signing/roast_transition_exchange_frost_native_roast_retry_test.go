//go:build frost_native && frost_roast_retry

package signing

import (
	"context"
	"crypto/sha256"
	"sync"
	"testing"
	"time"

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
// exchange over its own capture bus (the listener blocks on the unfed bus until
// test end; the tests drive the exchange methods directly).
func newExchangeTestNodes(
	t *testing.T,
	roastSessionID string,
	ctx attempt.AttemptContext,
	dkgKey []byte,
) map[group.MemberIndex]*exchangeNode {
	t.Helper()
	hash := ctx.Hash()
	nodes := map[group.MemberIndex]*exchangeNode{}
	// A test-lifetime ctx: listen() blocks harmlessly on the capture bus's unfed
	// streams (the bus captures broadcasts rather than delivering them), and the
	// tests drive onSnapshot/onBundle directly. Cancelling only at test end keeps
	// the session-end defer-clear from wiping the bindings before the assertions.
	exchangeCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
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

// produceTransitionBundleForTest runs the forced-snapshot + aggregate flow across
// the included nodes by hand and returns the elected coordinator's broadcast
// transition bundle message plus the elected member.
func produceTransitionBundleForTest(
	t *testing.T,
	roastSessionID string,
	ctx attempt.AttemptContext,
	nodes map[group.MemberIndex]*exchangeNode,
) (RunnerMessage, group.MemberIndex) {
	t.Helper()
	hash := ctx.Hash()

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

	for _, m := range ctx.IncludedSet {
		nodes[m].ex.BroadcastForcedSnapshot(hash)
	}
	for _, m := range ctx.IncludedSet {
		if m == elected {
			continue
		}
		snaps := nodes[m].bus.only(RunnerMsgEvidenceSnapshot)
		if len(snaps) != 1 {
			t.Fatalf("member %d expected to broadcast 1 snapshot, got %d", m, len(snaps))
		}
		nodes[elected].ex.onSnapshot(snaps[0])
	}
	nodes[elected].ex.AggregateAndBroadcast(hash)

	bundles := nodes[elected].bus.only(RunnerMsgTransitionBundle)
	if len(bundles) != 1 {
		t.Fatalf("elected coordinator must broadcast exactly one bundle, got %d", len(bundles))
	}
	return bundles[0], elected
}

// TestRoastTransitionExchange_LostSyncOnUnobservedBundle asserts a seat whose
// listener receives a transition bundle for an attempt it NEVER observed (it
// skipped a window the others committed) trips lost sync, so the retry loop can
// fail closed before selecting from a stale position (Codex lost-sync correction).
func TestRoastTransitionExchange_LostSyncOnUnobservedBundle(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	roastSessionID := "exchange-lost-sync-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x04, 0x05}
	ctx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	nodes := newExchangeTestNodes(t, roastSessionID, ctx, dkgKey)

	bundle, _ := produceTransitionBundleForTest(t, roastSessionID, ctx, nodes)

	// A lagging seat (member 4) that never observed this attempt -- its listener
	// receives the bundle the committed seats produced.
	var lagging group.MemberIndex = 4
	laggingCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	laggingEx := NewRoastTransitionExchange(
		laggingCtx,
		log.Logger("exchange-test-lagging"),
		&captureBus{},
		RoastRetryDeps{
			Coordinator: roast.NewInMemoryCoordinatorWithSigning(
				lagging, fixedSigner{}, roast.NoOpSignatureVerifier(),
			),
			Signer:     fixedSigner{},
			Verifier:   roast.NoOpSignatureVerifier(),
			SelfMember: uint32(lagging),
		},
		roastSessionID,
		lagging,
	)

	if laggingEx.HasLostSync() {
		t.Fatal("a seat must not start in lost sync")
	}
	laggingEx.onBundle(bundle)
	if !laggingEx.HasLostSync() {
		t.Fatal("a bundle for an attempt this seat never observed must trip lost sync")
	}
}

// TestRoastTransitionExchange_ConsumedRetransmitDoesNotLoseSync asserts that a
// duplicate bundle for an attempt this seat already observed and consumed is a
// benign no-op -- the observed-history marker (retained past the binding's
// clear) keeps a retransmit from falsely tripping lost sync.
func TestRoastTransitionExchange_ConsumedRetransmitDoesNotLoseSync(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	roastSessionID := "exchange-retransmit-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x06}
	ctx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := ctx.Hash()
	nodes := newExchangeTestNodes(t, roastSessionID, ctx, dkgKey)

	bundle, elected := produceTransitionBundleForTest(t, roastSessionID, ctx, nodes)

	// A non-elected receiver consumes the bundle: it stores the record and clears
	// its observe binding, leaving only the observed-history marker.
	var receiver group.MemberIndex
	for _, m := range included {
		if m != elected {
			receiver = m
			break
		}
	}
	nodes[receiver].ex.onBundle(bundle)
	if _, ok := RoastTransitionForSession(roastSessionID, receiver); !ok {
		t.Fatalf("receiver %d must hold a transition record after consuming", receiver)
	}
	if _, ok := observedAttempt(roastSessionID, receiver, hash); ok {
		t.Fatalf("receiver %d binding must be cleared after consuming", receiver)
	}

	// The same bundle re-delivered (a retransmit) must not trip lost sync.
	nodes[receiver].ex.onBundle(bundle)
	if nodes[receiver].ex.HasLostSync() {
		t.Fatal("a retransmit of an already-consumed bundle must not trip lost sync")
	}
}

// TestRoastTransitionExchange_MultiSeatElectedSeatAggregates is the PR2b-1.5
// acceptance test: when an operator controls multiple local seats and the elected
// ROAST coordinator is one of them, that seat aggregates the transition bundle
// using ITS OWN per-member coordinator (from the per-member registry). Pre-fix a
// single process-wide coordinator bound to ONE SelfMember returned ErrNotAggregator
// whenever the elected seat differed from it -> no bundle -> the next retry
// fail-closed for the whole group.
//
// NOTE on local fanout (Codex's guardrail): this test delivers the sibling seats'
// snapshots to the elected seat's exchange directly. In production the per-seat
// exchanges share ONE wallet BroadcastChannel; a node's own broadcast reaches its
// OTHER local seats' subscribers via the channel's self-delivery (libp2p FloodSub
// delivers a node's publishes to its own subscriptions, and the membership
// validator passes own-author). That self-delivery is a transport-contract
// assumption to confirm at prod-wiring; if a transport does not self-deliver,
// explicit local fanout is the remedy. The aggregation fix proven here is
// independent of how the sibling snapshot arrives.
func TestRoastTransitionExchange_MultiSeatElectedSeatAggregates(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")

	roastSessionID := "multiseat-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x0d}
	ctx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := ctx.Hash()

	// Deterministic elected coordinator for this context.
	probe := roast.NewInMemoryCoordinatorWithSigning(0, fixedSigner{}, roast.NoOpSignatureVerifier())
	probeHandle, err := probe.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("probe begin: %v", err)
	}
	elected, err := probe.SelectedCoordinator(probeHandle)
	if err != nil {
		t.Fatalf("probe elected: %v", err)
	}

	// One operator controls ALL included seats (the extreme multi-seat case): each
	// seat gets its OWN coordinator bound to its member, registered per-member,
	// sharing the operator key (fixedSigner).
	for _, m := range included {
		RegisterRoastRetryCoordinatorForMember(m, RoastRetryDeps{
			Coordinator: roast.NewInMemoryCoordinatorWithSigning(m, fixedSigner{}, roast.NoOpSignatureVerifier()),
			Signer:      fixedSigner{},
			Verifier:    roast.NoOpSignatureVerifier(),
			SelfMember:  uint32(m),
		})
	}

	// Build an exchange + observe binding for EVERY local seat from ITS OWN
	// registered per-member deps (the path the controller takes per signer); keep
	// each seat's capture bus to inspect what it broadcasts.
	exCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	buses := map[group.MemberIndex]*captureBus{}
	exchanges := map[group.MemberIndex]*RoastTransitionExchange{}
	for _, m := range included {
		deps, ok := RegisteredRoastRetryCoordinatorForMember(m)
		if !ok {
			t.Fatalf("deps for member %d missing", m)
		}
		handle, err := deps.Coordinator.BeginAttempt(ctx)
		if err != nil {
			t.Fatalf("begin attempt for %d: %v", m, err)
		}
		recordObservedAttempt(roastSessionID, m, hash, observedAttemptBinding{
			handle:            handle,
			context:           ctx,
			dkgGroupPublicKey: dkgKey,
		})
		bus := &captureBus{}
		buses[m] = bus
		exchanges[m] = NewRoastTransitionExchange(exCtx, log.Logger("multiseat"), bus, deps, roastSessionID, m)
	}

	// Every seat broadcasts its forced snapshot; the elected seat collects all (its
	// own recorded in BroadcastForcedSnapshot, the siblings' delivered to it).
	for _, m := range included {
		exchanges[m].BroadcastForcedSnapshot(hash)
	}
	for _, m := range included {
		if m == elected {
			continue
		}
		snaps := buses[m].only(RunnerMsgEvidenceSnapshot)
		if len(snaps) != 1 {
			t.Fatalf("member %d expected to broadcast 1 forced snapshot, got %d", m, len(snaps))
		}
		exchanges[elected].onSnapshot(snaps[0])
	}

	// Each seat runs aggregation.
	for _, m := range included {
		exchanges[m].AggregateAndBroadcast(hash)
	}

	// The elected local seat aggregated with ITS OWN per-member coordinator: a
	// transition record + exactly one broadcast bundle.
	if _, ok := RoastTransitionForSession(roastSessionID, elected); !ok {
		t.Fatal("the elected local seat must aggregate + store a record with its own per-member coordinator")
	}
	if got := buses[elected].only(RunnerMsgTransitionBundle); len(got) != 1 {
		t.Fatalf("the elected seat must broadcast exactly one bundle, got %d", len(got))
	}

	// The NON-elected sibling seats must NOT aggregate. This is the non-vacuous half:
	// pre-fix all seats shared ONE coordinator bound to the elected member, so a
	// sibling's AggregateBundle would have run as the elected member and broadcast a
	// bundle; per-member coordinators (bound to members != elected) make it
	// ErrNotAggregator -> no bundle.
	var sibling group.MemberIndex
	for _, m := range included {
		if m == elected {
			continue
		}
		if got := buses[m].only(RunnerMsgTransitionBundle); len(got) != 0 {
			t.Fatalf("non-elected local seat %d must not aggregate a bundle, got %d", m, len(got))
		}
		sibling = m
	}

	// A sibling seat receives the elected seat's bundle and stores ITS OWN
	// next-attempt record with its own coordinator (the multi-seat sibling-unparking
	// path that lets a non-elected local seat advance).
	exchanges[sibling].onBundle(buses[elected].only(RunnerMsgTransitionBundle)[0])
	if _, ok := RoastTransitionForSession(roastSessionID, sibling); !ok {
		t.Fatal("a sibling local seat must store its own transition record from the elected seat's bundle")
	}
}

// TestRoastTransitionExchange_SucceededSeatDoesNotStorePeerFailureBundle asserts
// the core B3 outcome: after a seat clears its observe binding on local success, a
// peer's failure bundle for that attempt is neither stored as a transition record
// (the attempt was won, not failed) nor mistaken for lost sync (the observed
// marker keeps it a benign no-op). With no fresh record the seat's next selection
// then fails closed -- the honest fail-closed-after-success path.
func TestRoastTransitionExchange_SucceededSeatDoesNotStorePeerFailureBundle(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	roastSessionID := "exchange-success-nostore-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x07}
	ctx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := ctx.Hash()
	nodes := newExchangeTestNodes(t, roastSessionID, ctx, dkgKey)

	bundle, elected := produceTransitionBundleForTest(t, roastSessionID, ctx, nodes)

	// A non-elected seat completed the attempt locally: it clears its observe
	// binding (B3), keeping only the observed-history marker. It has not consumed a
	// record (the helper delivered the bundle only to the elected coordinator).
	var succeeded group.MemberIndex
	for _, m := range included {
		if m != elected {
			succeeded = m
			break
		}
	}
	ClearObservedAttemptOnLocalSuccess(roastSessionID, succeeded, hash)

	// The peer's failure bundle now arrives at the succeeded seat's listener.
	nodes[succeeded].ex.onBundle(bundle)

	if _, ok := RoastTransitionForSession(roastSessionID, succeeded); ok {
		t.Fatal("a succeeded seat must not store a peer's failure transition for its won attempt")
	}
	if nodes[succeeded].ex.HasLostSync() {
		t.Fatal("a bundle for a succeeded (observed) attempt must not trip lost sync")
	}
}

// TestRoastTransitionExchange_SessionEndClearsObserveBindings asserts that when
// the exchange's session context ends, its listener clears the observe bindings
// this seat did not consume per-attempt (e.g. a signing whose attempts all
// succeeded), rather than leaking them until the TTL sweep.
func TestRoastTransitionExchange_SessionEndClearsObserveBindings(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	roastSessionID := "exchange-session-end"
	var hash [attempt.MessageDigestLength]byte
	hash[0] = 0x7e
	recordObservedAttempt(roastSessionID, 1, hash, observedAttemptBinding{})

	ctx, cancel := context.WithCancel(context.Background())
	_ = NewRoastTransitionExchange(
		ctx,
		log.Logger("exchange-test"),
		&captureBus{},
		RoastRetryDeps{
			Coordinator: roast.NewInMemoryCoordinatorWithSigning(
				1, fixedSigner{}, roast.NoOpSignatureVerifier(),
			),
			Signer:     fixedSigner{},
			Verifier:   roast.NoOpSignatureVerifier(),
			SelfMember: 1,
		},
		roastSessionID,
		1,
	)
	if !ObservedAttemptStoredForTest(roastSessionID, 1) {
		t.Fatal("binding should exist before session end")
	}

	cancel() // session end -> listener exits -> defer clears bindings.

	deadline := time.Now().Add(2 * time.Second)
	for ObservedAttemptStoredForTest(roastSessionID, 1) {
		if time.Now().After(deadline) {
			t.Fatal("session end must clear the observe binding")
		}
		time.Sleep(5 * time.Millisecond)
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

// TestRoastTransitionExchange_StashedEvidenceDrivesExclusion is the end-to-end
// blame-bridge proof (RFC-21 Phase 7.3 PR2b-2 step 2): coarse-path evidence stashed
// by the receive loop is carried by BroadcastForcedSnapshot into the aggregated
// bundle, so NextAttempt's f+1 accuser tally excludes the blamed member -- the
// exclusion the pre-bridge forced-EMPTY snapshots could never trigger.
func TestRoastTransitionExchange_StashedEvidenceDrivesExclusion(t *testing.T) {
	ResetObservedAttemptRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetObservedAttemptRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	roastSessionID := "exchange-blame-bridge-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x09, 0x0a}
	// original group size 3; quorum = ExclusionAccuserQuorum(3, threshold) =
	// 3 - 2 + 1 = 2, so two accusers establish a permanent reject exclusion.
	const threshold uint = 2

	ctx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := ctx.Hash()
	nodes := newExchangeTestNodes(t, roastSessionID, ctx, dkgKey)

	// Members 1 and 2 each observed a validation reject against member 3 during the
	// coarse receive loop; their submit stashed it. Member 3 observed nothing and
	// broadcasts only an empty proof-of-attendance snapshot.
	rejectAgainst3 := attempt.Evidence{
		Rejects: map[group.MemberIndex][]attempt.RejectEntry{
			3: {{Reason: "attempt_context_hash_mismatch", Count: 1}},
		},
	}
	stashPendingEvidence(roastSessionID, 1, hash, rejectAgainst3)
	stashPendingEvidence(roastSessionID, 2, hash, rejectAgainst3)

	bundleMsg, elected := produceTransitionBundleForTest(t, roastSessionID, ctx, nodes)

	// The aggregated bundle must carry real reject evidence from >=2 accusers, not
	// the forced-empty snapshots of the pre-bridge design.
	bundle := &roast.TransitionMessage{}
	if err := bundle.Unmarshal(bundleMsg.Payload); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	accusers := 0
	for i := range bundle.Bundle {
		if len(bundle.Bundle[i].Rejects) > 0 {
			accusers++
		}
	}
	if accusers < 2 {
		t.Fatalf("bundle must carry reject evidence from >=2 accusers; got %d", accusers)
	}

	// BroadcastForcedSnapshot consumed the stash.
	if PendingEvidenceStashedForTest(roastSessionID, 1) ||
		PendingEvidenceStashedForTest(roastSessionID, 2) {
		t.Fatal("BroadcastForcedSnapshot must consume the stash")
	}

	// A non-elected accuser verifies the bundle against its own observe handle and
	// computes the next attempt: member 3 meets the f+1 reject quorum -> excluded.
	var verifier group.MemberIndex
	for _, m := range included {
		if m != elected {
			verifier = m
			break
		}
	}
	binding, ok := observedAttempt(roastSessionID, verifier, hash)
	if !ok {
		t.Fatalf("non-elected seat %d must still hold its observe binding", verifier)
	}
	if err := nodes[verifier].coord.VerifyBundle(binding.handle, bundle); err != nil {
		t.Fatalf("verify bundle: %v", err)
	}
	next, err := nodes[verifier].coord.NextAttempt(binding.handle, bundle, threshold, dkgKey)
	if err != nil {
		t.Fatalf("next attempt: %v", err)
	}

	excluded3 := false
	for _, m := range next.ExcludedSet {
		if m == 3 {
			excluded3 = true
		}
	}
	if !excluded3 {
		t.Fatalf("member 3 must be excluded by the f+1 reject quorum; excluded=%v", next.ExcludedSet)
	}
	for _, m := range next.IncludedSet {
		if m == 3 {
			t.Fatalf("excluded member 3 must not be in the next included set; included=%v", next.IncludedSet)
		}
	}
}

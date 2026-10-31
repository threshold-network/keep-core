package spv

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_TimedOutTermWindowEdge checks that the
// re-check's late-settlement window edge for a TimedOut generation is
// timeoutAt + termSeconds: with a non-zero term the generation whose
// edge sits just ahead of the head is kept and still counts as
// late-proof risk.
func TestReservationKeeperObserver_TimedOutTermWindowEdge(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	key := big.NewInt(51)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, headNow),
	)
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		ActionType:  tbtc.ReservationActionTypeAcceptance,
		State:       tbtc.ReservationActionStateTimedOut,
		TimeoutAt:   headNow,
		TermSeconds: 30,
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 1 {
		t.Errorf(
			"TimedOut generation with a non-zero term: at-risk count = %v, "+
				"want 1",
			got,
		)
	}
}

// TestReservationKeeperObserver_TimedOutWindowEdgeAtHead checks the
// headNow == windowEdge boundary of the re-check eviction: a TimedOut
// generation whose window edge sits exactly at the chain head is still
// unproven risk (the eviction is strict, not inclusive).
func TestReservationKeeperObserver_TimedOutWindowEdgeAtHead(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	key := big.NewInt(52)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, headNow),
	)
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		ActionType:  tbtc.ReservationActionTypeAcceptance,
		State:       tbtc.ReservationActionStateTimedOut,
		TimeoutAt:   headNow,
		TermSeconds: 0,
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 1 {
		t.Errorf(
			"TimedOut generation with its window edge at the head: at-risk "+
				"count = %v, want 1",
			got,
		)
	}
}

// TestReservationKeeperObserver_RecheckMultiEviction checks that the
// re-check loop deletes every generation that can no longer matter
// (settled and absent) while a still-unproven generation stays tracked
// in the same pass.
func TestReservationKeeperObserver_RecheckMultiEviction(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	const day = 24 * 3600
	settledKey := big.NewInt(61)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(settledKey, headNow+day),
	)
	spvChain.setReservationAction(
		settledKey,
		1,
		acceptanceActionFixture(
			headNow+day,
			tbtc.ReservationActionStateSettled,
		),
	)

	pendingKey := big.NewInt(62)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(pendingKey, headNow+day),
	)
	spvChain.setReservationAction(
		pendingKey,
		1,
		acceptanceActionFixture(
			headNow+day,
			tbtc.ReservationActionStatePending,
		),
	)

	// An absent generation (no action record) is evicted too: three
	// tracked generations, two of them can no longer matter.
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(big.NewInt(63), headNow+day),
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedAcceptance); got != 1 {
		t.Fatalf("tracked generations = %d, want 1 (the pending one)", got)
	}
	if _, ok := observer.trackedAcceptance[reservationEventKey(pendingKey, 1)]; !ok {
		t.Fatal("the pending generation was evicted; it must stay tracked")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 1 {
		t.Errorf("at-risk count = %v, want 1", got)
	}
}

// TestReservationKeeperObserver_TimedOutEvictTwo checks that the
// re-check evicts two TimedOut generations whose late-settlement window
// edge has already passed, both in the same pass: the eviction loop
// must not stop at the first evicted generation and leave the second
// tracked.
func TestReservationKeeperObserver_TimedOutEvictTwo(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	for _, seed := range []int{81, 82} {
		key := big.NewInt(int64(seed))
		spvChain.addReservationAcceptanceRequestedEvent(
			acceptanceEventFixture(key, headNow),
		)
		spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
			ActionType: tbtc.ReservationActionTypeAcceptance,
			State:      tbtc.ReservationActionStateTimedOut,
			// The window edge sits behind the head: the generation can
			// no longer settle late and must be evicted.
			TimeoutAt: headNow - 3600,
		})
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedAcceptance); got != 0 {
		t.Fatalf(
			"tracked generations after evicting two past-edge TimedOut "+
				"generations = %d, want 0",
			got,
		)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 0 {
		t.Errorf("late-proof count after double eviction = %v, want 0", got)
	}
}

// TestReservationKeeperObserver_NilRecorderPassSucceeds checks that an
// observer built without a recorder runs a successful pass and publishes
// nothing.
func TestReservationKeeperObserver_NilRecorderPassSucceeds(t *testing.T) {
	spvChain, _, _, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	observer := newObserverForTest(spvChain, 0, nil)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
}

// TestReservationKeeperObserver_TypedNilRecorderNoPublish checks that a
// typed-nil *clientinfo.PerformanceMetrics wrapped in the recorder
// interface is treated as disabled: the pass completes and never
// reaches the nil recorder (which would panic under the guard).
func TestReservationKeeperObserver_TypedNilRecorderNoPublish(t *testing.T) {
	spvChain, _, _, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	var pm *clientinfo.PerformanceMetrics
	observer := newObserverForTest(
		spvChain,
		0,
		MetricsRecorder(pm),
	)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
}

// TestReservationKeeperObserver_LateProofUnrefreshedGeneration checks
// that the late-proof count skips a tracked generation whose
// snapshotted action was not refreshed this pass (a nil action) rather
// than dereferencing it, and counts the refreshed one.
func TestReservationKeeperObserver_LateProofUnrefreshedGeneration(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)
	headNow := uint64(head.Unix())

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	observer.trackedAcceptance[buildReservationActionKey(big.NewInt(1), 1)] =
		&trackedAcceptanceGeneration{
			reservationKey: big.NewInt(1),
			requestNonce:   1,
			// action is nil: not yet re-read this pass.
		}
	observer.trackedAcceptance[buildReservationActionKey(big.NewInt(2), 1)] =
		&trackedAcceptanceGeneration{
			reservationKey: big.NewInt(2),
			requestNonce:   1,
			action: &tbtc.ReservationAction{
				ActionType:  tbtc.ReservationActionTypeAcceptance,
				State:       tbtc.ReservationActionStatePending,
				TimeoutAt:   uint32(headNow + 3600),
				TermSeconds: 0,
			},
		}

	if got := observer.countLateProofsAtRisk(headNow); got != 1 {
		t.Errorf("late-proof count = %d, want 1 (the refreshed one only)", got)
	}
}

// TestReservationKeeperObserver_LateProofWindowEdgeWithTerm checks the
// late-proof window edge for a generation carrying a non-zero term:
// the edge is timeoutAt + termSeconds, and a generation whose edge sits
// just past the 7-day risk window does not count.
func TestReservationKeeperObserver_LateProofWindowEdgeWithTerm(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	const week = 7 * 24 * 3600
	key := big.NewInt(71)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, headNow+week+30),
	)
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		ActionType:  tbtc.ReservationActionTypeAcceptance,
		State:       tbtc.ReservationActionStatePending,
		TimeoutAt:   headNow + week + 30,
		TermSeconds: 60,
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 0 {
		t.Errorf(
			"generation past the 7-day edge (edge includes the term): "+
				"at-risk count = %v, want 0",
			got,
		)
	}
}

// TestReservationKeeperObserver_NilDataWalletSkipped checks that a
// cached wallet whose on-chain data was never fetched (a nil data
// pointer) is skipped by both the target and the source loops rather
// than a dereference hazard.
func TestReservationKeeperObserver_NilDataWalletSkipped(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)
	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	// A never-fetched entry sits next to a retiring source and an
	// aged target: the nil entry must be skipped by both loops.
	observer.cachedWallets[[20]byte{1}] = &cachedWalletState{}
	source := [20]byte{2}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateMovingFunds},
	}
	observer.cachedWallets[[20]byte{3}] = &cachedWalletState{
		data: &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
		},
	}

	key := big.NewInt(1)
	spvChain.setWalletReservations(source, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	got, rerr := observer.countReanchorsWithoutTarget(
		params,
		tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
		head,
	)
	if rerr != nil {
		t.Fatalf("no-target count failed: [%v]", rerr)
	}
	if got != 0 {
		t.Errorf("no-target count = %d, want 0", got)
	}
}

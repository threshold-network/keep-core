package spv

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_MultiEvictionDeletesAll checks that the
// post-pass eviction loop deletes every evicted deposit, not just the
// first: two released deposits both leave the tracked set in one pass.
func TestReservationKeeperObserver_MultiEvictionDeletesAll(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	const day = 24 * 3600
	k1 := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(11), 0, headNow+36*3600, 900,
	)
	k2 := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(12), 0, headNow+36*3600, 900,
	)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedDeposits); got != 2 {
		t.Fatalf("tracked deposits after the first pass = %d, want 2", got)
	}

	spvChain.setReservedDeposit(k1, [20]byte{}, false)
	spvChain.setReservedDeposit(k2, [20]byte{}, false)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("second observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedDeposits); got != 0 {
		t.Errorf(
			"tracked deposits after the double eviction = %d, want 0",
			got,
		)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 0 {
		t.Errorf("at-risk count after the double eviction = %v, want 0", got)
	}
}

// TestReservationKeeperObserver_MultiRequestedEvictionChecksOrder
// checks the unrequested-deposit eviction loop over a mixed tracked
// set: a requested deposit and a released deposit are evicted in the
// same pass, and the two evictions are ordered so that both complete -
// a requested deposit processed before a released one leaves the
// released deposit tracked if the loop stops early.
func TestReservationKeeperObserver_MultiRequestedEvictionChecksOrder(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	const day = 24 * 3600
	releasedKey := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(41), 0, headNow+day+day, 900,
	)
	spvChain.setReservedDeposit(releasedKey, [20]byte{}, false)

	requestedKey := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(42), 0, headNow+day+day, 900,
	)
	spvChain.setReservation(requestedKey, &tbtc.Reservation{
		State:        tbtc.ReservationStateActive,
		RequestNonce: 1,
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedDeposits); got != 0 {
		t.Errorf(
			"tracked deposits after the requested/released eviction pass = "+
				"%d, want 0",
			got,
		)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 0 {
		t.Errorf("at-risk count after the eviction pass = %v, want 0", got)
	}
}

// TestReservationKeeperObserver_RevealVaultMismatchExcluded checks that
// deposit reveals routed to a non-reservation vault are not tracked,
// while a reveal routed to the reservation vault later in the scan is
// still tracked: two mismatched reveals before one matched one track
// exactly one, and the unrequested-deposit count sees only the matched
// one.
func TestReservationKeeperObserver_RevealVaultMismatchExcluded(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	otherVault := chain.Address("0x00000000000000000000000000000000000000f2")
	var locktime [4]byte
	spvChain.addPastDepositRevealedEvent(
		nil,
		&tbtc.DepositRevealedEvent{
			FundingTxHash:  observerFundingTxHash(1),
			RefundLocktime: locktime,
			Vault:          &otherVault,
			BlockNumber:    900,
		},
	)
	spvChain.addPastDepositRevealedEvent(
		nil,
		&tbtc.DepositRevealedEvent{
			FundingTxHash:  observerFundingTxHash(2),
			RefundLocktime: locktime,
			Vault:          &otherVault,
			BlockNumber:    901,
		},
	)
	const day = 24 * 3600
	matchKey := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(3), 0, headNow+day+day+day, 902,
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedDeposits); got != 1 {
		t.Errorf(
			"tracked deposits = %d, want 1 (the vault-routed reveal only)",
			got,
		)
	}
	if _, ok := observer.trackedDeposits[matchKey.String()]; !ok {
		t.Fatal("the vault-routed reveal was not tracked")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Errorf("at-risk count = %v, want 1", got)
	}
}

// TestReservationKeeperObserver_RevealNilVaultExcluded checks that a
// deposit reveal with a nil vault is skipped by the reveal scan without
// a nil dereference, while a vault-routed reveal later in the chunk is
// still tracked.
func TestReservationKeeperObserver_RevealNilVaultExcluded(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	var locktime [4]byte
	spvChain.addPastDepositRevealedEvent(
		nil,
		&tbtc.DepositRevealedEvent{
			FundingTxHash:  observerFundingTxHash(31),
			RefundLocktime: locktime,
			Vault:          nil,
			BlockNumber:    900,
		},
	)
	const day = 24 * 3600
	matchKey := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(32), 0, headNow+day+day+day, 901,
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedDeposits); got != 1 {
		t.Errorf(
			"tracked deposits = %d, want 1 (the nil-vault reveal is skipped)",
			got,
		)
	}
	if _, ok := observer.trackedDeposits[matchKey.String()]; !ok {
		t.Fatal("the vault-routed reveal was not tracked")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Errorf("at-risk count = %v, want 1", got)
	}
}

// TestReservationKeeperObserver_EvictionLeavesTrackedSet checks the
// full eviction pass over a mixed tracked set: two released deposits
// and two deposits with an acceptance request made for it all leave the
// tracked set in one pass, and none of them count.
func TestReservationKeeperObserver_EvictionLeavesTrackedSet(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	const day = 24 * 3600
	released1 := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(41), 0, headNow+day+day, 900,
	)
	released2 := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(42), 0, headNow+day+day, 900,
	)
	spvChain.setReservedDeposit(released1, [20]byte{}, false)
	spvChain.setReservedDeposit(released2, [20]byte{}, false)

	requested1 := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(43), 0, headNow+day+day, 900,
	)
	requested2 := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(44), 0, headNow+day+day, 900,
	)
	spvChain.setReservation(requested1, &tbtc.Reservation{
		State:        tbtc.ReservationStateActive,
		RequestNonce: 1,
	})
	spvChain.setReservation(requested2, &tbtc.Reservation{
		State:        tbtc.ReservationStateActive,
		RequestNonce: 1,
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedDeposits); got != 0 {
		t.Errorf(
			"tracked deposits after the mixed eviction pass = %d, want 0",
			got,
		)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 0 {
		t.Errorf("at-risk count after the mixed eviction pass = %v", got)
	}
}

// TestReservationKeeperObserver_VaultMismatchRevealNotTracked checks
// that deposit reveals routed to a non-reservation vault are excluded
// from the tracked-deposit set even when a reserved-deposit record was
// installed for them: only the vault-routed reveal is tracked, and the
// at-risk gauge sees only it.
func TestReservationKeeperObserver_VaultMismatchRevealNotTracked(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	otherVault := chain.Address("0x00000000000000000000000000000000000000f2")
	const day = 24 * 3600
	deadline := headNow + day + day

	// Two reveals routed to a different vault, each with a
	// reserved-deposit record installed, so an identity read would keep
	// them in the at-risk set if they were tracked.
	for _, seed := range []byte{70, 71} {
		event := &tbtc.DepositRevealedEvent{
			FundingTxHash: observerFundingTxHash(seed),
			RefundLocktime: [4]byte{
				byte(deadline),
				byte(deadline >> 8),
				byte(deadline >> 16),
				byte(deadline >> 24),
			},
			Vault:       &otherVault,
			BlockNumber: 900,
		}
		spvChain.addPastDepositRevealedEvent(nil, event)
		key := spvChain.BuildDepositKey(
			observerFundingTxHash(seed),
			0,
		)
		spvChain.setReservedDeposit(key, [20]byte{2}, true)
	}
	// One reveal routed to the fixture vault, reserved as well: the
	// only one that must be tracked and counted.
	matchKey := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(72), 0, deadline, 901,
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := len(observer.trackedDeposits); got != 1 {
		t.Errorf(
			"tracked deposits = %d, want 1 (the vault-routed reveal only)",
			got,
		)
	}
	if _, ok := observer.trackedDeposits[matchKey.String()]; !ok {
		t.Error("the vault-routed reveal was not tracked")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Errorf("at-risk count = %v, want 1", got)
	}
}

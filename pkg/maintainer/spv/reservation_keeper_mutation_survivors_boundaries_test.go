package spv

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_HeadTimeClampZero checks that a
// non-positive chain head time clamps to a zero head now: the
// observation timestamp gauge publishes 0, not an underflowed uint64.
func TestReservationKeeperObserver_HeadTimeClampZero(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	spvChain.setLatestBlockTimestamp(time.Unix(-1, 0))

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != 0 {
		t.Errorf("negative head time: timestamp gauge = %v, want 0", got)
	}
}

// TestReservationKeeperObserver_HeadTimeClampEpoch is the clamp-floor
// boundary: a head time at the Unix epoch clamps to 0, not 1.
func TestReservationKeeperObserver_HeadTimeClampEpoch(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	spvChain.setLatestBlockTimestamp(time.Unix(0, 0))

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != 0 {
		t.Errorf("epoch head time: timestamp gauge = %v, want 0", got)
	}
}

// TestReservationKeeperObserver_DeadlineUnderflowEdge checks the
// underflow boundary of the last-possible-request-time computation: a
// refund deadline inside the 24h margin + action timeout maps to a
// closing-now request window, which counts as at risk.
func TestReservationKeeperObserver_DeadlineUnderflowEdge(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	// Deadline 47h ahead: the margin (24h) plus the fixture's 24h
	// action timeout is 48h, so the last possible request time
	// underflows and the deposit's request window is closing now.
	key := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(61), 0, headNow+47*3600, 900,
	)
	_ = key

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Errorf("at-risk count at the underflow edge = %v, want 1", got)
	}
}

// TestReservationKeeperObserver_RegistrationBeforeActivationBlock
// checks that the wallet-registration scan discovers a registration at
// block zero: the scan's start is block zero, not one.
func TestReservationKeeperObserver_RegistrationBeforeActivationBlock(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	observer := newObserverForTest(spvChain, 0, recorder)

	source := [20]byte{7}
	spvChain.setWallet(source, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: source,
			BlockNumber:         0,
		},
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if _, ok := observer.cachedWallets[source]; !ok {
		t.Error("a registration at block 0 was not discovered")
	}
}

// TestReservationKeeperObserver_AcceptanceCursorMultiChunk checks the
// acceptance scan cursor after a multi-chunk scan: the cursor lands on
// the last chunk's end block, the confirmed tip.
func TestReservationKeeperObserver_AcceptanceCursorMultiChunk(t *testing.T) {
	spvChain, _, recorder, _, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	blockCounter.SetCurrentBlock(21000)

	observer := newObserverForTest(spvChain, 5000, recorder)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := observer.acceptanceScanCursor; got != confirmedTip(21000) {
		t.Errorf(
			"acceptance cursor after multi-chunk pass = %d, want %d",
			got,
			confirmedTip(21000),
		)
	}
}

// TestReservationKeeperObserver_RevealCursorMultiChunk is the reveal
// cursor analog of the acceptance multi-chunk cursor test.
func TestReservationKeeperObserver_RevealCursorMultiChunk(t *testing.T) {
	spvChain, _, recorder, _, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	blockCounter.SetCurrentBlock(21000)

	observer := newObserverForTest(spvChain, 5000, recorder)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := observer.revealScanCursor; got != confirmedTip(21000) {
		t.Errorf(
			"reveal cursor after multi-chunk pass = %d, want %d",
			got,
			confirmedTip(21000),
		)
	}
}

// TestReservationKeeperObserver_RegistrationCursorMultiChunk is the
// registration cursor analog: a multi-chunk registration scan lands the
// cursor on the confirmed tip.
func TestReservationKeeperObserver_RegistrationCursorMultiChunk(t *testing.T) {
	spvChain, _, recorder, _, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	blockCounter.SetCurrentBlock(21000)

	observer := newObserverForTest(spvChain, 0, recorder)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := observer.registrationScanCursor; got != confirmedTip(21000) {
		t.Errorf(
			"registration cursor after multi-chunk pass = %d, want %d",
			got,
			confirmedTip(21000),
		)
	}
}

// TestReservationKeeperObserver_UnrequestedDeadlineUnder48hCounted
// checks that a pending reserved deposit whose refund deadline is in the
// underflow band (less than the 24h margin plus the 24h action timeout)
// is counted at risk: its last possible request time has already passed
// relative to the head, so the request window is closing now.
func TestReservationKeeperObserver_UnrequestedDeadlineUnder48hCounted(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	// The fixture's action timeout is 24h and the refund safety margin
	// is 24h, so the underflow band is an absolute refund deadline
	// within 48h. 24h (86400 s) sits in that band: the original maps
	// it to a last-possible-request time of zero (the request window
	// is closing now), which is at risk; a guard that underflows
	// instead wraps to a far-future time and drops it.
	key := addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(80), 0, 86400, 900,
	)
	_ = key

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Errorf(
			"at-risk count for the in-band underflow deadline = %v, "+
				"want 1",
			got,
		)
	}
}

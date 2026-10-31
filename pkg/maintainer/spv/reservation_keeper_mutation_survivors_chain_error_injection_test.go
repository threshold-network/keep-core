package spv

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_HeadTimeErrorFailsPass checks that a
// failed chain-head-time read fails the observation pass at the first
// read, before any gauge is published.
func TestReservationKeeperObserver_HeadTimeErrorFailsPass(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	spvChain.latestBlockTimestampErr = errors.New("injected RPC failure")

	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the head-time read error")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != 0 {
		t.Errorf("gauge published after a failed pass: got %v", got)
	}
}

// TestReservationKeeperObserver_ResParametersErrorFailsPass checks that
// a failed reservation-parameter read fails the pass.
func TestReservationKeeperObserver_ResParametersErrorFailsPass(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	chain_ := &mutationObserverChain{
		localChain:               spvChain,
		reservationParametersErr: errors.New("injected RPC failure"),
	}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = chain_
	if err := observer.ObservationPass(); err == nil {
		t.Fatal(
			"expected the pass to fail on the reservation-parameter error",
		)
	}
}

// TestReservationKeeperObserver_WalletParametersErrorFailsPass checks
// that a failed wallet-parameter read fails the pass.
func TestReservationKeeperObserver_WalletParametersErrorFailsPass(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	chain_ := &mutationObserverChain{
		localChain:          spvChain,
		walletParametersErr: errors.New("injected RPC failure"),
	}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = chain_
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the wallet-parameter error")
	}
}

// TestReservationKeeperObserver_AcceptanceFetchFailureFailsPass checks
// that a failed acceptance-event fetch fails the pass.
func TestReservationKeeperObserver_AcceptanceFetchFailureFailsPass(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	chain_ := &mutationObserverChain{
		localChain:          spvChain,
		acceptanceEventsErr: errors.New("injected RPC failure"),
	}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = chain_
	if err := observer.ObservationPass(); err == nil {
		t.Fatal(
			"expected the pass to fail on the acceptance-event fetch error",
		)
	}
}

// TestReservationKeeperObserver_RevealFetchFailureFailsPass checks that
// a failed reveal-event fetch fails the pass.
func TestReservationKeeperObserver_RevealFetchFailureFailsPass(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	chain_ := &mutationObserverChain{
		localChain:      spvChain,
		revealEventsErr: errors.New("injected RPC failure"),
	}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = chain_
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the reveal-event fetch error")
	}
}

// TestReservationKeeperObserver_RegistrationScanFailureFailsPass checks
// that a failed wallet-registration scan fails the pass.
func TestReservationKeeperObserver_RegistrationScanFailureFailsPass(t *testing.T) {
	spvChain, observer, _, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	spvChain.setPastNewWalletRegisteredEventsErr(
		errors.New("injected RPC failure"),
	)

	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the registration-scan error")
	}
}

// TestReservationKeeperObserver_ScanRangeFailureFailsPass checks that a
// failed block-counter read (the scan-range computation) fails the pass,
// leaves every scan cursor at its last good position, and holds the
// gauges at their last values.
func TestReservationKeeperObserver_ScanRangeFailureFailsPass(t *testing.T) {
	spvChain, observer, recorder, _, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	spvChain.setWallet(
		[20]byte{1},
		&tbtc.WalletChainData{State: tbtc.StateLive},
	)
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: [20]byte{1},
			BlockNumber:         800,
		},
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	tip := confirmedTip(1000)
	baseline := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	)
	if observer.acceptanceScanCursor != tip ||
		observer.revealScanCursor != tip ||
		observer.registrationScanCursor != tip {
		t.Fatalf(
			"first pass cursors = (%d, %d, %d), want %d each",
			observer.acceptanceScanCursor,
			observer.revealScanCursor,
			observer.registrationScanCursor,
			tip,
		)
	}

	blockCounter.SetCurrentBlockErr(errors.New("injected RPC failure"))
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the block-counter error")
	}
	if observer.acceptanceScanCursor != tip ||
		observer.revealScanCursor != tip ||
		observer.registrationScanCursor != tip {
		t.Errorf(
			"cursors advanced after a failed scan range: (%d, %d, %d)",
			observer.acceptanceScanCursor,
			observer.revealScanCursor,
			observer.registrationScanCursor,
		)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != baseline {
		t.Errorf("timestamp gauge moved after a failed pass: got %v", got)
	}
}

// TestReservationKeeperObserver_ScanRangeFailurePropagates calls each
// of the three incremental scans directly with the block-counter read
// failing, and asserts each scan returns the scan-range error: the
// error is not swallowed at the range-read site of any scan.
func TestReservationKeeperObserver_ScanRangeFailurePropagates(t *testing.T) {
	spvChain, observer, _, _, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	blockCounter.SetCurrentBlockErr(errors.New("injected RPC failure"))

	if err := observer.scanNewAcceptanceRequests(); err == nil {
		t.Error("acceptance scan did not propagate the scan-range error")
	}
	if err := observer.scanNewDepositReveals(params); err == nil {
		t.Error("reveal scan did not propagate the scan-range error")
	}
	if err := observer.scanNewWalletRegistrations(); err == nil {
		t.Error("registration scan did not propagate the scan-range error")
	}
}

// TestReservationKeeperObserver_NoScanCursorAdvances checks the
// no-activation-entry path: the first acceptance and reveal scans are
// no-ops that move their cursors straight to the confirmed tip without
// fetching, while the registration scan still covers block zero to tip.
// The seeded events must stay untracked: a scan that is not scheduled
// must not fetch.
func TestReservationKeeperObserver_NoScanCursorAdvances(t *testing.T) {
	spvChain, _, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	spvChain.setWallet(
		[20]byte{1},
		&tbtc.WalletChainData{State: tbtc.StateLive},
	)
	_ = head

	observer := newObserverForTest(spvChain, math.MaxUint64, recorder)
	key := big.NewInt(1)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, 0),
	)
	addObserverPendingReservedDeposit(
		t, spvChain, observerFundingTxHash(1), 0, 0, 900,
	)
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: [20]byte{1},
			BlockNumber:         800,
		},
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	tip := confirmedTip(1000)
	if got := observer.acceptanceScanCursor; got != tip {
		t.Errorf("acceptance cursor after the no-scan pass = %d, want %d", got, tip)
	}
	if got := observer.revealScanCursor; got != tip {
		t.Errorf("reveal cursor after the no-scan pass = %d, want %d", got, tip)
	}
	if got := observer.registrationScanCursor; got != tip {
		t.Errorf("registration cursor after the pass = %d, want %d", got, tip)
	}
	if got := len(observer.trackedAcceptance); got != 0 {
		t.Errorf("no-scan pass tracked acceptance events: %d", got)
	}
	if got := len(observer.trackedDeposits); got != 0 {
		t.Errorf("no-scan pass tracked reveal events: %d", got)
	}
	if _, ok := observer.cachedWallets[[20]byte{1}]; !ok {
		t.Error("the registration scan did not discover the registered wallet")
	}
}

// TestReservationKeeperObserver_RecheckFailureFailsPass checks that a
// failed re-read of a tracked acceptance generation fails the pass and
// holds every gauge at its last value.
func TestReservationKeeperObserver_RecheckFailureFailsPass(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setReservationVaultFeeDebt(250, nil)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	headNow := uint32(head.Unix())

	key := big.NewInt(1)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, headNow+3600),
	)
	spvChain.setReservationAction(
		key,
		1,
		acceptanceActionFixture(
			headNow+3600,
			tbtc.ReservationActionStatePending,
		),
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	baselineTimestamp := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	)

	spvChain.setGetReservationActionErr(errors.New("injected RPC failure"))
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the re-read error")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != baselineTimestamp {
		t.Errorf("timestamp gauge moved after a failed re-check: got %v", got)
	}
	if got := recorder.Gauge(clientinfo.MetricReservationVaultFeeDebtSat); got != 250 {
		t.Errorf("fee debt gauge changed after a failed pass: got %v", got)
	}
}

// TestReservationKeeperObserver_FeeDebtFailureFailsPass checks that a
// failed fee-debt read fails the pass before any gauge is published.
func TestReservationKeeperObserver_FeeDebtFailureFailsPass(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}

	spvChain.setReservationVaultFeeDebt(
		0,
		errors.New("injected RPC failure"),
	)
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the fee-debt read error")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got == 0 {
		t.Fatal("gauge published after a failed pass")
	}
}

// TestReservationKeeperObserver_WalletReservationsFailureCountsZero
// checks that a failed source-wallet reservation read fails the
// no-target count and reports a zero count, not a stale positive one.
func TestReservationKeeperObserver_WalletReservationsFailureCountsZero(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	source := [20]byte{1}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateMovingFunds},
	}
	spvChain.setWalletReservations(
		source,
		[]*big.Int{big.NewInt(1)},
	)
	spvChain.setWalletReservationsErr(errors.New("injected RPC failure"))

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	got, rerr := observer.countReanchorsWithoutTarget(
		params,
		tbtc.WalletParameters{},
		head,
	)
	if rerr == nil {
		t.Fatal("expected the no-target count to fail on the read error")
	}
	if got != 0 {
		t.Errorf("failed no-target count = %d, want 0", got)
	}
}

// TestReservationKeeperObserver_ReservationReadFailureCountsZero is the
// per-reservation analog of the wallet-reservation read failure check.
func TestReservationKeeperObserver_ReservationReadFailureCountsZero(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	source := [20]byte{1}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateClosing},
	}
	spvChain.setWalletReservations(
		source,
		[]*big.Int{big.NewInt(1)},
	)
	spvChain.setGetReservationErr(errors.New("injected RPC failure"))

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	got, rerr := observer.countReanchorsWithoutTarget(
		params,
		tbtc.WalletParameters{},
		head,
	)
	if rerr == nil {
		t.Fatal("expected the no-target count to fail on the read error")
	}
	if got != 0 {
		t.Errorf("failed no-target count = %d, want 0", got)
	}
}

// TestReservationKeeperObserver_DepositReservationReadFailureCountsZero
// checks that a failed per-deposit reservation read in the
// unrequested-deposit count fails and reports a zero count: the tracked
// set is populated directly so the read is exercised without depending
// on a full pass.
func TestReservationKeeperObserver_DepositReservationReadFailureCountsZero(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	key := big.NewInt(71)
	spvChain.setReservedDeposit(key, [20]byte{1}, true)
	observer.trackedDeposits[key.String()] = &trackedUnrequestedDeposit{
		depositKey: key,
	}
	spvChain.setGetReservationErr(errors.New("injected RPC failure"))

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	got, _, rerr := observer.countUnrequestedDepositsAtRisk(
		uint64(head.Unix()),
		params,
	)
	if rerr == nil {
		t.Fatal(
			"expected the unrequested-deposit count to fail on the read error",
		)
	}
	if got != 0 {
		t.Errorf("failed unrequested-deposit count = %d, want 0", got)
	}
}

// TestReservationKeeperObserver_IsReservedDepositReadFailureCountsZero
// is the reserved-deposit record read-failure variant of the same check:
// the record read fails before the reservation read, so the injected
// error is returned with a zero count.
func TestReservationKeeperObserver_IsReservedDepositReadFailureCountsZero(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	key := big.NewInt(72)
	spvChain.setReservedDeposit(key, [20]byte{1}, true)
	observer.trackedDeposits[key.String()] = &trackedUnrequestedDeposit{
		depositKey: key,
	}
	spvChain.setIsReservedDepositErr(errors.New("injected RPC failure"))

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	got, _, rerr := observer.countUnrequestedDepositsAtRisk(
		uint64(head.Unix()),
		params,
	)
	if rerr == nil {
		t.Fatal(
			"expected the unrequested-deposit count to fail on the read error",
		)
	}
	if got != 0 {
		t.Errorf("failed unrequested-deposit count = %d, want 0", got)
	}
}

// TestReservationKeeperObserver_WalletReadFailureFailsPass checks that
// a failed on-chain wallet read during the refresh fails the pass: a
// wallet with unknown state can hide a retiring source.
func TestReservationKeeperObserver_WalletReadFailureFailsPass(t *testing.T) {
	spvChain, observer, _, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	// A registered wallet with no on-chain data installed: the
	// discovery scan adds it to the cache, and the refresh's read
	// fails on it.
	source := [20]byte{9}
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: source,
			BlockNumber:         800,
		},
	)

	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail on the wallet read error")
	}
}

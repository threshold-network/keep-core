package spv

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// Behavior tests written against the go-mutesting survivor list for the
// two keeper observer files. They reuse the fixtures and fakes from
// reservation_keeper_observer_test.go and chain_test.go, and are named so
// the existing TestReservationKeeperObserver_ filter runs them.

// mutationObserverChain wraps the observer's fixture chain and injects
// errors into the chain reads the localChain has no error-injection
// field for: the reservation parameters, the wallet parameters, and the
// two event-fetch methods the observer's incremental scans issue.
type mutationObserverChain struct {
	*localChain
	reservationParametersErr error
	walletParametersErr      error
	acceptanceEventsErr      error
	revealEventsErr          error
}

func (c *mutationObserverChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	if c.reservationParametersErr != nil {
		return nil, c.reservationParametersErr
	}
	return c.localChain.ReservationParameters()
}

func (c *mutationObserverChain) GetWalletParameters() (
	tbtc.WalletParameters,
	error,
) {
	if c.walletParametersErr != nil {
		return tbtc.WalletParameters{}, c.walletParametersErr
	}
	return c.localChain.GetWalletParameters()
}

func (c *mutationObserverChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	if c.acceptanceEventsErr != nil {
		return nil, c.acceptanceEventsErr
	}
	return c.localChain.PastReservationAcceptanceRequestedEvents(filter)
}

func (c *mutationObserverChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	if c.revealEventsErr != nil {
		return nil, c.revealEventsErr
	}
	return c.localChain.PastDepositRevealedEvents(filter)
}

// walletCountingChain wraps the fixture chain and counts the GetWallet
// calls the wallet-state refresh issues, so a test can assert which
// cache entries a pass re-read.
type walletCountingChain struct {
	*localChain
	walletFetches int
}

func (c *walletCountingChain) GetWallet(walletPublicKeyHash [20]byte) (
	*tbtc.WalletChainData,
	error,
) {
	c.walletFetches++
	return c.localChain.GetWallet(walletPublicKeyHash)
}

// setGetReservationErr installs a chain-wide error for GetReservation.
func (lc *localChain) setGetReservationErr(err error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.getReservationErr = err
}

// confirmedTip returns the endBlock a reservation scan stops at for a
// given current block (the current block minus the confirmation lag).
func confirmedTip(currentBlock uint64) uint64 {
	if currentBlock <= reservationEventScanConfirmationBlocks {
		return 0
	}

	return currentBlock - reservationEventScanConfirmationBlocks
}

// newObserverForTest builds a fresh observer over the fixture chain with
// the given activation block and recorder.
func newObserverForTest(
	spvChain *localChain,
	activationBlock uint64,
	recorder MetricsRecorder,
) *ReservationKeeperObserver {
	return NewReservationKeeperObserver(
		spvChain,
		activationBlock,
		recorder,
	)
}

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

// TestReservationKeeperObserver_RunStopsOnCancel runs the background
// loop with a short interval: the initial pass publishes the gauges
// before the ticker loop starts, and a cancelled context stops the loop
// with a nil return.
func TestReservationKeeperObserver_RunStopsOnCancel(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- observer.Run(ctx, 50*time.Millisecond)
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	) == 0 {
		select {
		case err := <-done:
			t.Fatalf("Run returned before publishing a pass: [%v]", err)
		case <-deadline.C:
			cancel()
			t.Fatal("Run did not publish a pass within the deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Past the initial pass and at least one ticker tick.
	time.Sleep(120 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error on cancel: [%v]", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop when the context was cancelled")
	}
}

// TestReservationKeeperObserver_RunRejectsZeroInterval checks that Run
// rejects a non-positive interval before starting the ticker loop: a
// zero interval would create a spinning ticker. The context auto-cancels
// after 200 ms so a mutant that accepts the interval still returns
// within a bounded time.
func TestReservationKeeperObserver_RunRejectsZeroInterval(t *testing.T) {
	spvChain, observer, _, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	if err := observer.Run(ctx, 0); err == nil {
		t.Error("Run accepted a zero interval")
	}
}

// TestReservationKeeperObserver_RunTinyInterval checks that a positive
// (if tiny) interval is accepted: the loop must not reject it, must
// publish a pass, and must stop on cancel.
func TestReservationKeeperObserver_RunTinyInterval(t *testing.T) {
	spvChain, observer, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- observer.Run(ctx, 1*time.Nanosecond)
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	) == 0 {
		select {
		case err := <-done:
			t.Fatalf("Run rejected a positive interval: [%v]", err)
		case <-deadline.C:
			cancel()
			t.Fatal("Run did not publish a pass within the deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}

	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error on cancel: [%v]", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop when the context was cancelled")
	}
}

// TestReservationKeeperObserver_SigningWindowBoundaries checks the
// re-anchor signing-window floor against the live action timeout: the
// window is open only when the timeout strictly exceeds the floor. Each
// case carries an eligible target, so the no-target count is driven
// purely by the signing-window decision.
func TestReservationKeeperObserver_SigningWindowBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		timeout uint32
		want    int
	}{
		{name: "below floor", timeout: 2 * 3600, want: 1},
		{name: "at floor", timeout: 2*3600 + 1, want: 1},
		{name: "above floor", timeout: 2*3600 + 2, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spvChain := newLocalChain()
			head := time.Unix(1_000_000, 0)
			spvChain.setLatestBlockTimestamp(head)

			observer := newObserverForTest(
				spvChain,
				0,
				newObserverRecorder(),
			)

			source := [20]byte{1}
			observer.cachedWallets[source] = &cachedWalletState{
				data: &tbtc.WalletChainData{State: tbtc.StateMovingFunds},
			}
			observer.cachedWallets[[20]byte{2}] = &cachedWalletState{
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

			params := &tbtc.ReservationParameters{
				ReservationVault:         observerVault,
				ReservationActionTimeout: tc.timeout,
				ReservationMinAmount:     100,
				ReservationTxMaxFee:      20,
			}

			got, err := observer.countReanchorsWithoutTarget(
				params,
				tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
				head,
			)
			if err != nil {
				t.Fatalf("no-target count failed: [%v]", err)
			}
			if got != tc.want {
				t.Errorf(
					"action timeout %d s: no-target count = %d, want %d",
					tc.timeout,
					got,
					tc.want,
				)
			}
		})
	}
}

// TestReservationKeeperObserver_AnchorFloorBoundary checks the anchor
// amount floor boundary: an anchor exactly at min amount + max fee does
// not satisfy the strict above-floor check and still counts as
// no-target; one satoshi above it clears the count.
func TestReservationKeeperObserver_AnchorFloorBoundary(t *testing.T) {
	spvChain := newLocalChain()
	head := time.Unix(1_000_000, 0)
	spvChain.setLatestBlockTimestamp(head)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	source := [20]byte{1}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateClosing},
	}
	observer.cachedWallets[[20]byte{2}] = &cachedWalletState{
		data: &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
		},
	}

	key := big.NewInt(1)
	spvChain.setWalletReservations(source, []*big.Int{key})
	params := &tbtc.ReservationParameters{
		ReservationVault:         observerVault,
		ReservationActionTimeout: 24 * 3600,
		ReservationMinAmount:     100,
		ReservationTxMaxFee:      20,
	}
	walletParams := tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600}

	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(120), // exactly at the floor
	})
	got, err := observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 1 {
		t.Errorf("anchor at the floor: no-target count = %d, want 1", got)
	}

	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(121), // one above the floor
	})
	got, err = observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 0 {
		t.Errorf("anchor one above the floor: no-target count = %d, want 0", got)
	}
}

// TestReservationKeeperObserver_NilAnchorUtxoCounts checks that an
// Active reservation without an anchor UTXO does not satisfy the anchor
// check and counts toward the no-target gauge (with every other check
// satisfied, so the count is driven purely by the missing anchor), and
// that the missing anchor is not a dereference hazard.
func TestReservationKeeperObserver_NilAnchorUtxoCounts(t *testing.T) {
	spvChain := newLocalChain()
	head := time.Unix(1_000_000, 0)
	spvChain.setLatestBlockTimestamp(head)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	source := [20]byte{1}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateMovingFunds},
	}
	observer.cachedWallets[[20]byte{2}] = &cachedWalletState{
		data: &tbtc.WalletChainData{
			State:     tbtc.StateLive,
			CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
		},
	}

	key := big.NewInt(1)
	spvChain.setWalletReservations(source, []*big.Int{key})
	params := &tbtc.ReservationParameters{
		ReservationVault:         observerVault,
		ReservationActionTimeout: 24 * 3600,
		ReservationMinAmount:     100,
		ReservationTxMaxFee:      20,
	}
	walletParams := tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600}

	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})
	got, err := observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 0 {
		t.Errorf("anchored reservation: no-target count = %d, want 0", got)
	}

	// The source's only reservation carries no anchor: it must count,
	// and the missing anchor must not panic the anchor check.
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	got, err = observer.countReanchorsWithoutTarget(
		params,
		walletParams,
		head,
	)
	if err != nil {
		t.Fatalf("no-target count failed: [%v]", err)
	}
	if got != 1 {
		t.Errorf("nil anchor: no-target count = %d, want 1", got)
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

// TestReservationKeeperObserver_NonActiveReservationSkipped checks that
// a non-Active reservation on a retiring source wallet is skipped by
// the no-target count: one Closed reservation plus one Active one counts
// exactly one.
func TestReservationKeeperObserver_NonActiveReservationSkipped(t *testing.T) {
	spvChain, _, _, head, _ := observerFixture(t)

	observer := newObserverForTest(spvChain, 0, newObserverRecorder())
	source := [20]byte{1}
	observer.cachedWallets[source] = &cachedWalletState{
		data: &tbtc.WalletChainData{State: tbtc.StateClosing},
	}
	closedKey := big.NewInt(1)
	activeKey := big.NewInt(2)
	spvChain.setWalletReservations(
		source,
		[]*big.Int{closedKey, activeKey},
	)
	spvChain.setReservation(closedKey, &tbtc.Reservation{
		State:      tbtc.ReservationStateClosed,
		AnchorUtxo: observerAnchorUtxo(1000),
	})
	spvChain.setReservation(activeKey, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	params, err := spvChain.ReservationParameters()
	if err != nil {
		t.Fatalf("reservation parameters: [%v]", err)
	}
	got, rerr := observer.countReanchorsWithoutTarget(
		params,
		tbtc.WalletParameters{},
		head,
	)
	if rerr != nil {
		t.Fatalf("no-target count failed: [%v]", rerr)
	}
	if got != 1 {
		t.Errorf("no-target count = %d, want 1 (the Active one only)", got)
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

// TestReservationKeeperObserver_RefreshReadsAndStamps checks the wallet
// refresh read/stamp semantics: a cache entry last read past the
// refresh interval and a never-read entry are both re-read on a pass, a
// recently read one is skipped, and a successful pass stamps the entry
// so the next pass within the interval re-reads nothing.
func TestReservationKeeperObserver_RefreshReadsAndStamps(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	counting := &walletCountingChain{localChain: spvChain}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = counting

	a := [20]byte{1}
	b := [20]byte{2}
	c := [20]byte{3}
	d := [20]byte{4}
	now := time.Now()
	spvChain.setWallet(a, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(b, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(c, &tbtc.WalletChainData{State: tbtc.StateClosing})
	spvChain.setWallet(d, &tbtc.WalletChainData{State: tbtc.StateClosing})
	// a sits just past the 10-minute interval; d sits between the 9- and
	// 10-minute interval variants; c is recently read; b is never read.
	observer.cachedWallets[a] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateLive},
		fetchedAt: now.Add(-10*time.Minute - 30*time.Second),
	}
	observer.cachedWallets[b] = &cachedWalletState{}
	observer.cachedWallets[c] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateClosing},
		fetchedAt: now.Add(-30 * time.Second),
	}
	observer.cachedWallets[d] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateClosing},
		fetchedAt: now.Add(-9*time.Minute - 30*time.Second),
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	if got := counting.walletFetches; got != 2 {
		t.Fatalf(
			"wallet reads on the first pass = %d, want 2 "+
				"(the over-interval and the never-read entries)",
			got,
		)
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("second observation pass failed: [%v]", err)
	}
	if got := counting.walletFetches; got != 2 {
		t.Errorf(
			"wallet reads after the second pass = %d, want 2 "+
				"(every entry was stamped by the first pass)",
			got,
		)
	}
}

// TestReservationKeeperObserver_RefreshSkipsThenFetches checks that the
// refresh loop keeps walking after a skipped entry: an over-interval
// entry that follows a recently read one must still be re-read, not
// skipped because the loop stopped at the skip.
func TestReservationKeeperObserver_RefreshSkipsThenFetches(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	counting := &walletCountingChain{localChain: spvChain}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = counting

	a := [20]byte{1}
	b := [20]byte{2}
	now := time.Now()
	spvChain.setWallet(a, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(b, &tbtc.WalletChainData{State: tbtc.StateClosing})
	// a is recently read: skipped. b is over the interval: must still
	// be read on the same pass, even though it follows the skipped
	// entry in the map walk.
	observer.cachedWallets[a] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateLive},
		fetchedAt: now.Add(-30 * time.Second),
	}
	observer.cachedWallets[b] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateClosing},
		fetchedAt: now.Add(-11 * time.Minute),
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := counting.walletFetches; got != 1 {
		t.Errorf(
			"wallet reads = %d, want 1 (the over-interval entry, after "+
				"the skipped one)",
			got,
		)
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

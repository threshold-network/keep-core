package spv

import (
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// The reservation keeper observer publishes the reservation keeper
// watchdog gauges through a MetricsRecorder, so these tests assert on
// the recorded gauge values rather than on logs: the exact 7-day
// late-proof risk-window edge, the TimedOut->Settled transition,
// released-deposit exclusion, the exact 1-day unrequested-deposit edge,
// and that a failed observation pass leaves the observation timestamp
// and every gauge value unchanged.

// observerRecorder is a MetricsRecorder that records the last SetGauge
// value per metric name.
type observerRecorder struct {
	mutex  sync.Mutex
	gauges map[string]float64
}

func newObserverRecorder() *observerRecorder {
	return &observerRecorder{gauges: make(map[string]float64)}
}

func (r *observerRecorder) IncrementCounter(_ string, _ float64) {}

func (r *observerRecorder) SetGauge(name string, value float64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.gauges[name] = value
}

// Gauge returns the last recorded value of the named gauge (0 when the
// gauge was never set).
func (r *observerRecorder) Gauge(name string) float64 {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.gauges[name]
}

// observerVault is the reservation vault address the observer fixture
// reveals are routed to, so the reveal scan tracks them.
var observerVault chain.Address = "0x00000000000000000000000000000000000000f1"

// observerFixture builds a fresh observer against a local chain: a block
// counter at block 1000, a chain head time, and a reservation parameter
// set with a 1d action timeout (satisfying the re-anchor signing
// window), a 100 satoshi min amount and a 20 satoshi max fee. It also
// returns the mock block counter so tests can advance the chain head:
// the observer's event scans are incremental (bounded, from the
// activation block), so a wallet registration that appears after the
// first pass's scan range must land at a block past the previous pass's
// cursor and be picked up only once the head has advanced.
func observerFixture(
	t *testing.T,
) (
	*localChain,
	*ReservationKeeperObserver,
	*observerRecorder,
	time.Time,
	*mockBlockCounter,
) {
	t.Helper()

	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	head := time.Unix(1_000_000, 0)
	spvChain.setLatestBlockTimestamp(head)

	spvChain.setReservationParameters(
		&tbtc.ReservationParameters{
			ReservationVault:         observerVault,
			ReservationActionTimeout: 24 * 3600,
			ReservationMinAmount:     100,
			ReservationTxMaxFee:      20,
		},
	)

	recorder := newObserverRecorder()
	observer := NewReservationKeeperObserver(
		spvChain,
		0, // activation block
		recorder,
	)

	return spvChain, observer, recorder, head, blockCounter
}

// addObserverRevealEvent installs a reserved deposit-revealed event
// routed to the fixture reservation vault, decodable by the observer's
// reveal scan into the given refund deadline. It returns the deposit key
// the Bridge derives for the reveal's funding output.
func addObserverRevealEvent(
	t *testing.T,
	spvChain *localChain,
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	refundDeadline uint32,
	blockNumber uint64,
) *big.Int {
	t.Helper()

	// RefundLocktime is a little-endian encoding of the uint32 deadline
	// (see reverseUint32's doc).
	var locktime [4]byte
	locktime[0] = byte(refundDeadline)
	locktime[1] = byte(refundDeadline >> 8)
	locktime[2] = byte(refundDeadline >> 16)
	locktime[3] = byte(refundDeadline >> 24)

	depositKey := spvChain.BuildDepositKey(fundingTxHash, fundingOutputIndex)

	spvChain.addPastDepositRevealedEvent(
		nil,
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
			RefundLocktime:     locktime,
			Vault:              &observerVault,
			BlockNumber:        blockNumber,
		},
	)

	return depositKey
}

// addObserverPendingReservedDeposit installs a pending reserved deposit:
// a reveal event routed to the fixture vault and the pending
// reserved-deposit record the observer reads for it.
func addObserverPendingReservedDeposit(
	t *testing.T,
	spvChain *localChain,
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	refundDeadline uint32,
	blockNumber uint64,
) *big.Int {
	t.Helper()

	depositKey := addObserverRevealEvent(
		t,
		spvChain,
		fundingTxHash,
		fundingOutputIndex,
		refundDeadline,
		blockNumber,
	)
	spvChain.setReservedDeposit(depositKey, [20]byte{1}, true)

	return depositKey
}

func observerFundingTxHash(seed byte) bitcoin.Hash {
	var hash bitcoin.Hash
	hash[0] = seed

	return hash
}

func observerAnchorUtxo(value int64) *bitcoin.UnspentTransactionOutput {
	return &bitcoin.UnspentTransactionOutput{Value: value}
}

func TestReservationKeeperObserver_PassGauges(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setReservationVaultFeeDebt(1000, nil)
	spvChain.setWalletParameters(
		tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
	)

	headNow := uint32(head.Unix())

	// A pending acceptance generation whose late-settlement window edge
	// (timeoutAt + termSeconds) is 2 days ahead of the head: at risk
	// under the 7-day gauge rule.
	key := big.NewInt(1)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, headNow+2*24*3600),
	)
	spvChain.setReservationAction(
		key,
		1,
		acceptanceActionFixture(
			headNow+2*24*3600,
			tbtc.ReservationActionStatePending,
		),
	)

	// An Active reservation on a MovingFunds source with an aged Live
	// target: it has an eligible re-anchor target, so it does not count
	// toward the no-target gauge.
	source := [20]byte{1}
	spvChain.setWallet(source, &tbtc.WalletChainData{
		State: tbtc.StateMovingFunds,
	})
	spvChain.setWalletReservations(source, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: source,
		State:               tbtc.ReservationStateActive,
		AnchorUtxo:          observerAnchorUtxo(1000),
	})
	agedTarget := [20]byte{2}
	spvChain.setWallet(agedTarget, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
	})
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: source,
			BlockNumber:         800,
		},
	)
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: agedTarget,
			BlockNumber:         850,
		},
	)

	// A pending reserved deposit with no acceptance request whose last
	// possible request time is 12 hours behind the chain head: at risk
	// under the 1-day gauge rule. Last request time is
	// refundDeadline - 24h - 24h(actionTimeout), so the deadline is
	// head - 12h + 48h.
	depositKey := addObserverPendingReservedDeposit(
		t,
		spvChain,
		observerFundingTxHash(7),
		0,
		headNow+36*3600,
		950,
	)
	_ = depositKey

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}

	check := func(name string, want float64) {
		t.Helper()
		if got := recorder.Gauge(name); got != want {
			t.Errorf("gauge %s = %v, want %v", name, got, want)
		}
	}
	check(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount, 1,
	)
	check(
		clientinfo.MetricReservationReanchorNoEligibleTargetCount, 0,
	)
	check(clientinfo.MetricReservationVaultFeeDebtSat, 1000)
	check(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount, 1,
	)
	check(
		clientinfo.MetricReservationKeeperObservationTimestamp,
		float64(head.Unix()),
	)

	// A second pass advances the observation timestamp and refreshes
	// the fee debt.
	head2 := head.Add(5 * time.Second)
	spvChain.setLatestBlockTimestamp(head2)
	spvChain.setReservationVaultFeeDebt(2000, nil)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("second observation pass failed: [%v]", err)
	}
	check(
		clientinfo.MetricReservationKeeperObservationTimestamp,
		float64(head2.Unix()),
	)
	check(clientinfo.MetricReservationVaultFeeDebtSat, 2000)
}

func TestReservationKeeperObserver_LateProofWindowEdges(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	headNow := uint32(head.Unix())
	riskWindow := uint32(7 * 24 * 3600)

	// Edge: the late-settlement window edge (timeoutAt +
	// termSeconds) exactly 7 days ahead of the head - still at risk
	// (the comparison is inclusive).
	atRiskKey := big.NewInt(21)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(atRiskKey, headNow+riskWindow),
	)
	spvChain.setReservationAction(
		atRiskKey,
		1,
		acceptanceActionFixture(
			headNow+riskWindow,
			tbtc.ReservationActionStatePending,
		),
	)

	// Just past the edge: window edge 7 days + 1s ahead - no longer at
	// risk.
	farKey := big.NewInt(22)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(farKey, headNow+riskWindow+1),
	)
	spvChain.setReservationAction(
		farKey,
		1,
		acceptanceActionFixture(
			headNow+riskWindow+1,
			tbtc.ReservationActionStatePending,
		),
	)

	// Already closed: a TimedOut generation whose window edge passed
	// under the head is not unproven risk, it is simply evicted.
	closedKey := big.NewInt(23)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(closedKey, headNow-24*3600),
	)
	spvChain.setReservationAction(
		closedKey,
		1,
		acceptanceActionFixture(
			headNow-24*3600,
			tbtc.ReservationActionStateTimedOut,
		),
	)
	spvChain.setReservation(closedKey, &tbtc.Reservation{
		State: tbtc.ReservationStateUnknown,
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 1 {
		t.Errorf("late proof at-risk count = %v, want 1", got)
	}
}

func TestReservationKeeperObserver_TimedOutSettledTransition(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	headNow := uint32(head.Unix())
	key := big.NewInt(31)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, headNow+24*3600),
	)
	spvChain.setReservationAction(
		key,
		1,
		acceptanceActionFixture(
			headNow+24*3600,
			tbtc.ReservationActionStateTimedOut,
		),
	)
	// The reservation is still Unknown: the TimedOut generation is
	// unproven and its window edge is inside the 7-day window - at
	// risk.
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 1 {
		t.Fatalf("expected the TimedOut generation to count, got %v", got)
	}

	// The late proof settles: the action flips TimedOut->Settled and
	// the reservation leaves Unknown. It no longer counts.
	spvChain.setReservationAction(
		key,
		1,
		acceptanceActionFixture(
			headNow+24*3600,
			tbtc.ReservationActionStateSettled,
		),
	)
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 0 {
		t.Errorf("settled generation still counted: got %v, want 0", got)
	}
}

func TestReservationKeeperObserver_UnrequestedDepositEdges(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	headNow := uint32(head.Unix())
	const (
		actionTimeout = 24 * 3600 // matches the fixture parameter set
		margin        = 24 * 3600 // DEPOSIT_REFUND_SAFETY_MARGIN
		riskWindow    = 24 * 3600 // the 1-day gauge window
	)

	// lastRequest = refundDeadline - margin - actionTimeout.
	//
	// Edge: last request time exactly 1 day ahead of the head - at
	// risk (the comparison is inclusive).
	addObserverPendingReservedDeposit(
		t,
		spvChain,
		observerFundingTxHash(1),
		0,
		headNow+riskWindow+margin+actionTimeout,
		900,
	)

	// Just past the edge: last request time 1 day + 1s ahead - not
	// at risk yet.
	addObserverPendingReservedDeposit(
		t,
		spvChain,
		observerFundingTxHash(2),
		0,
		headNow+riskWindow+1+margin+actionTimeout,
		900,
	)

	// An acceptance request was made for this deposit (RequestNonce
	// nonzero): it is excluded regardless of its deadline.
	requestedKey := addObserverPendingReservedDeposit(
		t,
		spvChain,
		observerFundingTxHash(3),
		0,
		headNow+riskWindow+margin+actionTimeout,
		900,
	)
	spvChain.setReservation(requestedKey, &tbtc.Reservation{
		RequestNonce: 1,
	})

	// Released to the pool: the pending reserved-deposit record
	// cleared, so the reveal is no longer a pending reserved deposit.
	releasedKey := addObserverRevealEvent(
		t,
		spvChain,
		observerFundingTxHash(4),
		0,
		headNow+riskWindow+margin+actionTimeout,
		900,
	)
	spvChain.setReservedDeposit(releasedKey, [20]byte{1}, false)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Errorf("unrequested deposit at-risk count = %v, want 1", got)
	}
}

func TestReservationKeeperObserver_NoEligibleTarget(t *testing.T) {
	spvChain, observer, recorder, head, blockCounter := observerFixture(t)
	spvChain.setWalletParameters(
		tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
	)

	mfSource := [20]byte{1}
	spvChain.setWallet(mfSource, &tbtc.WalletChainData{
		State: tbtc.StateMovingFunds,
	})
	closingSource := [20]byte{2}
	spvChain.setWallet(closingSource, &tbtc.WalletChainData{
		State: tbtc.StateClosing,
	})
	liveSource := [20]byte{3}
	spvChain.setWallet(liveSource, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: head.Add(-time.Duration(29*24) * time.Hour),
	})
	for _, pkh := range [][20]byte{mfSource, closingSource, liveSource} {
		spvChain.addNewWalletRegisteredEvent(
			&tbtc.NewWalletRegisteredEvent{
				WalletPublicKeyHash: pkh,
				BlockNumber:         800,
			},
		)
	}

	// The only other Live target is still inside its creation period.
	youngTarget := [20]byte{4}
	spvChain.setWallet(youngTarget, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: head.Add(-time.Duration(29*24) * time.Hour),
	})
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: youngTarget,
			BlockNumber:         850,
		},
	)

	mfKey := big.NewInt(41)
	closingKey := big.NewInt(42)
	liveKey := big.NewInt(43)
	spvChain.setWalletReservations(mfSource, []*big.Int{mfKey})
	spvChain.setWalletReservations(closingSource, []*big.Int{closingKey})
	spvChain.setWalletReservations(liveSource, []*big.Int{liveKey})
	for _, reservationKey := range []*big.Int{mfKey, closingKey, liveKey} {
		spvChain.setReservation(reservationKey, &tbtc.Reservation{
			State:      tbtc.ReservationStateActive,
			AnchorUtxo: observerAnchorUtxo(1000),
		})
	}

	// The only other Live target is still inside its creation period,
	// and the Live source is not a re-anchor source at all: both the
	// MovingFunds and Closing sources have no eligible target.
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationReanchorNoEligibleTargetCount,
	); got != 2 {
		t.Fatalf("no-target count = %v, want 2", got)
	}

	// An aged Live target appears on-chain. The observer's
	// registration scan is incremental (the first pass scanned up to
	// block 1000 - 12), so the new registration must land past that
	// cursor and the head must advance for it to be picked up.
	agedTarget := [20]byte{5}
	spvChain.setWallet(agedTarget, &tbtc.WalletChainData{
		State:     tbtc.StateLive,
		CreatedAt: head.Add(-time.Duration(31*24) * time.Hour),
	})
	spvChain.addNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEvent{
			WalletPublicKeyHash: agedTarget,
			BlockNumber:         990,
		},
	)
	blockCounter.SetCurrentBlock(1050)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationReanchorNoEligibleTargetCount,
	); got != 0 {
		t.Errorf("no-target count = %v, want 0 after the aged target", got)
	}

	// The anchor amount floor - one of the contract checks that still
	// apply to privileged re-anchors - closes the target set for the
	// MovingFunds reservation again: its anchor is no longer above
	// minAmount + txMaxFee, while the Closing reservation's anchor
	// still is.
	spvChain.setReservation(mfKey, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(100), // not > 100 + 20
	})
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationReanchorNoEligibleTargetCount,
	); got != 1 {
		t.Errorf(
			"no-target count = %v, want 1 (anchor floor closed for the "+
				"MovingFunds reservation)",
			got,
		)
	}
}

func TestReservationKeeperObserver_FailedPassKeepsState(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setReservationVaultFeeDebt(500, nil)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	headNow := uint32(head.Unix())
	key := big.NewInt(51)
	spvChain.addReservationAcceptanceRequestedEvent(
		acceptanceEventFixture(key, headNow+2*24*3600),
	)
	spvChain.setReservationAction(
		key,
		1,
		acceptanceActionFixture(
			headNow+2*24*3600,
			tbtc.ReservationActionStatePending,
		),
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	baselineTimestamp := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	)
	if baselineTimestamp == 0 {
		t.Fatal("observation timestamp was not set by a successful pass")
	}

	// A chain read failure makes the pass fail: the timestamp and the
	// gauge values hold their last state.
	spvChain.setGetReservationActionErr(
		errors.New("injected RPC failure"),
	)
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the observation pass to fail on the RPC error")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != baselineTimestamp {
		t.Errorf(
			"observation timestamp changed after a failed pass: got %v, "+
				"want %v",
			got,
			baselineTimestamp,
		)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationVaultFeeDebtSat,
	); got != 500 {
		t.Errorf("fee debt gauge changed after a failed pass: got %v", got)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
	); got != 1 {
		t.Errorf("late-proof gauge changed after a failed pass: got %v", got)
	}
}

// A read that a gauge depends on and that fails must fail the whole
// pass. Skipping the item instead would publish a lower count with a
// fresh timestamp, which the monitor would read as healthy.
func TestReservationKeeperObserver_SourceReadFailureFailsPass(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(
		tbtc.WalletParameters{CreationPeriod: 30 * 24 * 3600},
	)

	source := [20]byte{1}
	spvChain.setWallet(source, &tbtc.WalletChainData{
		State: tbtc.StateMovingFunds,
	})
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		WalletPublicKeyHash: source,
		BlockNumber:         800,
	})
	key := big.NewInt(61)
	spvChain.setWalletReservations(source, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State:      tbtc.ReservationStateActive,
		AnchorUtxo: observerAnchorUtxo(1000),
	})

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationReanchorNoEligibleTargetCount,
	); got != 1 {
		t.Fatalf("no-target count = %v, want 1", got)
	}
	baseline := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	)

	spvChain.setWalletReservationsErr(errors.New("injected RPC failure"))
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail when a source wallet cannot be read")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationReanchorNoEligibleTargetCount,
	); got != 1 {
		t.Errorf("no-target count changed after a failed read: got %v", got)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != baseline {
		t.Errorf("observation timestamp moved after a failed read: got %v", got)
	}
	_ = head
}

func TestReservationKeeperObserver_DepositReadFailureFailsPass(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	headNow := uint32(head.Unix())
	const day = 24 * 3600
	addObserverPendingReservedDeposit(
		t,
		spvChain,
		observerFundingTxHash(7),
		0,
		headNow+day+day+day, // last request time exactly 1 day ahead
		900,
	)

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Fatalf("at-risk count = %v, want 1", got)
	}
	baseline := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	)

	spvChain.setIsReservedDepositErr(errors.New("injected RPC failure"))
	if err := observer.ObservationPass(); err == nil {
		t.Fatal("expected the pass to fail when a deposit cannot be read")
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Errorf("at-risk count changed after a failed read: got %v", got)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
	); got != baseline {
		t.Errorf("observation timestamp moved after a failed read: got %v", got)
	}
}

// A deposit that was pending and at risk, then released to the pool,
// leaves the at-risk count on the next pass.
func TestReservationKeeperObserver_ReleasedDepositLeavesCount(t *testing.T) {
	spvChain, observer, recorder, head, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})

	headNow := uint32(head.Unix())
	const day = 24 * 3600
	key := addObserverPendingReservedDeposit(
		t,
		spvChain,
		observerFundingTxHash(8),
		0,
		headNow+day+day+day,
		900,
	)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 1 {
		t.Fatalf("at-risk count = %v, want 1", got)
	}

	spvChain.setReservedDeposit(key, [20]byte{}, false)
	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("second observation pass failed: [%v]", err)
	}
	if got := recorder.Gauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
	); got != 0 {
		t.Errorf("at-risk count = %v after release, want 0", got)
	}
}

func acceptanceEventFixture(
	reservationKey *big.Int,
	timeoutAt uint32,
) *tbtc.ReservationAcceptanceRequestedEvent {
	return &tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey: reservationKey,
		RequestNonce:   1,
		TimeoutAt:      timeoutAt,
		BlockNumber:    900,
	}
}

func acceptanceActionFixture(
	timeoutAt uint32,
	state tbtc.ReservationActionState,
) *tbtc.ReservationAction {
	return &tbtc.ReservationAction{
		ActionType:  tbtc.ReservationActionTypeAcceptance,
		State:       state,
		TimeoutAt:   timeoutAt,
		TermSeconds: 0,
	}
}

package spv

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/ipfs/go-log/v2"
)

var reservationKeeperObserverLogger = log.Logger("keep-maintainer-spv-reservations-keeper-observer")

// trackedAcceptanceGeneration is one still-unproven acceptance action
// generation (Pending or TimedOut, not Settled) the keeper observer
// keeps under the late-proof risk window.
type trackedAcceptanceGeneration struct {
	reservationKey *big.Int
	requestNonce   uint64
	// action is the last successfully re-read on-chain action record
	// for this generation; its snapshotted TimeoutAt + TermSeconds is
	// the late-settlement window edge the risk check uses.
	action *tbtc.ReservationAction
}

// trackedUnrequestedDeposit is one pending reserved deposit the keeper
// observer tracks for the deadline-risk gauge: a deposit revealed
// reserved that has still had no acceptance request made for it.
type trackedUnrequestedDeposit struct {
	depositKey     *big.Int
	refundDeadline uint32
}

// ReservationKeeperObserver scans reservation events from activation and
// discovers wallets independently of that boundary. It publishes the five
// gauges only after a fully successful observation pass.
//
// The observer is a read-only network observer: it submits nothing and
// drives no transactions; every number it publishes is a view over
// what other maintainers and coordinators have already put on-chain.
// Its bounded state (the tracked sets and the wallet cache) is owned
// by the single goroutine that runs its loop, so it carries no locks.
type ReservationKeeperObserver struct {
	spvChain        Chain
	activationBlock uint64
	recorder        MetricsRecorder

	// Bounded incremental event-scan cursors: each cursor is the last
	// block already scanned for that event stream, so every pass
	// covers only what has appeared since the previous one.
	acceptanceScanCursor   uint64
	revealScanCursor       uint64
	registrationScanCursor uint64

	// Still-unproven acceptance generations (Pending or TimedOut, not
	// Settled), keyed like the proof loop's pending-event maps.
	trackedAcceptance map[string]*trackedAcceptanceGeneration

	// Pending reserved deposits with no acceptance request yet, keyed
	// by the Bridge deposit key.
	trackedDeposits map[string]*trackedUnrequestedDeposit

	// Every registered wallet seen since activation, with its last
	// successfully fetched on-chain data. The cache grows with
	// registrations (bounded by the network's wallet count, not by
	// time) and is re-fetched on a wall-clock refresh interval.
	cachedWallets map[[20]byte]*cachedWalletState
}

// NewReservationKeeperObserver scans reservation events from activationBlock.
// Without an activation entry, it covers only events after startup. Wallet
// discovery includes registrations before activation. A nil recorder disables
// publication.
func NewReservationKeeperObserver(
	spvChain Chain,
	activationBlock uint64,
	recorder MetricsRecorder,
) *ReservationKeeperObserver {
	return &ReservationKeeperObserver{
		spvChain:          spvChain,
		activationBlock:   activationBlock,
		recorder:          recorder,
		trackedAcceptance: make(map[string]*trackedAcceptanceGeneration),
		trackedDeposits:   make(map[string]*trackedUnrequestedDeposit),
		cachedWallets:     make(map[[20]byte]*cachedWalletState),
	}
}

// ObservationPass runs one full observation pass and publishes the five
// keeper watchdog gauges through the recorder. It returns an error when
// the pass could not complete - a scan failure, a failed re-check of a
// tracked generation, or a failed fee-debt read - in which case no gauge
// is updated and the observation timestamp holds its last value.
func (oko *ReservationKeeperObserver) ObservationPass() error {
	headTime, err := oko.spvChain.GetLatestBlockTimestamp()
	if err != nil {
		return fmt.Errorf("failed to read the chain head time: [%w]", err)
	}
	headNow := uint64(max(headTime.Unix(), 0))

	reservationParameters, err := oko.spvChain.ReservationParameters()
	if err != nil {
		return fmt.Errorf(
			"failed to read the reservation parameters: [%w]",
			err,
		)
	}
	walletParameters, err := oko.spvChain.GetWalletParameters()
	if err != nil {
		return fmt.Errorf(
			"failed to read the wallet parameters: [%w]",
			err,
		)
	}

	// 1. Incremental scans of new events since the last pass: new
	// acceptance requests, new reserved deposit reveals, and new
	// wallet registrations. A scan failure leaves the cursor at the
	// last good chunk, so the next pass resumes there instead of
	// replaying activation-to-tip.
	if err := oko.scanNewAcceptanceRequests(); err != nil {
		return err
	}
	if err := oko.scanNewDepositReveals(reservationParameters); err != nil {
		return err
	}
	if err := oko.scanNewWalletRegistrations(); err != nil {
		return err
	}
	// 2. Re-check the tracked acceptance generations against the
	// chain: a failed re-check fails the pass so the timestamp and
	// every gauge hold their last values.
	// A scan failure leaves the cursor at the last good chunk, so the
	// next pass resumes there instead of replaying activation-to-tip.
	if err := oko.recheckTrackedAcceptance(headNow); err != nil {
		return err
	}

	// 3. Refresh the wallet-state cache on its wall-clock interval.
	if err := oko.refreshWalletStates(); err != nil {
		return err
	}

	// 4. Compute the four value gauges from the current chain state.
	// Every read a gauge depends on must succeed: skipping a failed
	// item would publish a lower count with a fresh timestamp, which
	// reads as healthy.
	lateProofCount := oko.countLateProofsAtRisk(headNow)
	noTargetCount, err := oko.countReanchorsWithoutTarget(
		reservationParameters,
		walletParameters,
		headTime,
	)
	if err != nil {
		return err
	}
	unrequestedCount, evictedDeposits, err :=
		oko.countUnrequestedDepositsAtRisk(headNow, reservationParameters)
	if err != nil {
		return err
	}
	for _, key := range evictedDeposits {
		delete(oko.trackedDeposits, key)
	}

	feeDebt, err := oko.spvChain.ReservationVaultFeeDebtSat(
		reservationParameters.ReservationVault,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to read the reservation vault fee debt: [%w]",
			err,
		)
	}

	// 5. Fully successful pass: publish everything, including the
	// observation timestamp.
	oko.publishGauges(
		headNow,
		lateProofCount,
		noTargetCount,
		unrequestedCount,
		feeDebt,
	)

	return nil
}

// scanNewAcceptanceRequests incrementally scans new
// ReservationAcceptanceRequested events and adds each generation not
// yet tracked to the tracked set. The scan is bounded by
// nextReservationScanRange: the first pass covers
// activationBlock-to-tip in chunks and later passes only new blocks.
func (oko *ReservationKeeperObserver) scanNewAcceptanceRequests() error {
	startBlock, endBlock, scan, err := nextReservationScanRange(
		oko.spvChain,
		oko.acceptanceScanCursor,
		oko.activationBlock,
	)
	if err != nil {
		return err
	}

	if !scan {
		oko.acceptanceScanCursor = endBlock
		return nil
	}

	return fetchPastEventsInChunks(
		func(startBlock, endBlock uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
			end := endBlock
			return oko.spvChain.PastReservationAcceptanceRequestedEvents(
				&tbtc.ReservationAcceptanceRequestedEventFilter{
					StartBlock: startBlock,
					EndBlock:   &end,
				},
			)
		},
		startBlock,
		endBlock,
		func(events []*tbtc.ReservationAcceptanceRequestedEvent, chunkEnd uint64) {
			for _, event := range events {
				key := reservationEventKey(event.ReservationKey, event.RequestNonce)
				if _, tracked := oko.trackedAcceptance[key]; !tracked {
					oko.trackedAcceptance[key] = &trackedAcceptanceGeneration{
						reservationKey: event.ReservationKey,
						requestNonce:   event.RequestNonce,
					}
				}
			}
			oko.acceptanceScanCursor = chunkEnd
		},
	)
}

// scanNewDepositReveals incrementally scans new DepositRevealed events
// routed to the reservation vault and tracks each one as a pending
// reserved deposit, until it is evicted by a state read (released,
// stale, accepted, or requested).
func (oko *ReservationKeeperObserver) scanNewDepositReveals(
	reservationParameters *tbtc.ReservationParameters,
) error {
	startBlock, endBlock, scan, err := nextReservationScanRange(
		oko.spvChain,
		oko.revealScanCursor,
		oko.activationBlock,
	)
	if err != nil {
		return err
	}

	if !scan {
		oko.revealScanCursor = endBlock
		return nil
	}

	vault := reservationParameters.ReservationVault
	return fetchPastEventsInChunks(
		func(startBlock, endBlock uint64) ([]*tbtc.DepositRevealedEvent, error) {
			end := endBlock
			return oko.spvChain.PastDepositRevealedEvents(
				&tbtc.DepositRevealedEventFilter{
					StartBlock: startBlock,
					EndBlock:   &end,
				},
			)
		},
		startBlock,
		endBlock,
		func(events []*tbtc.DepositRevealedEvent, chunkEnd uint64) {
			for _, event := range events {
				if event.Vault == nil ||
					!strings.EqualFold(string(*event.Vault), string(vault)) {
					continue
				}
				depositKey := oko.spvChain.BuildDepositKey(
					event.FundingTxHash,
					event.FundingOutputIndex,
				)
				key := depositKey.String()
				if _, tracked := oko.trackedDeposits[key]; !tracked {
					oko.trackedDeposits[key] = &trackedUnrequestedDeposit{
						depositKey:     depositKey,
						refundDeadline: reverseUint32(event.RefundLocktime),
					}
				}
			}
			oko.revealScanCursor = chunkEnd
		},
	)
}

// recheckTrackedAcceptance re-reads every tracked acceptance generation's
// on-chain action record so a generation that can no longer matter is
// evicted: settled generations are proven, vetoed and superseded ones are
// gone from the risk set, an absent (unknown) generation was never
// requested on chain, and a TimedOut generation whose late-settlement
// window edge (snapshotted timeoutAt + termSeconds) has passed under the
// chain head can no longer settle late. The snapshot fields are refreshed
// on every pass so the risk window is always the on-chain one. A failed
// re-check is returned: a failed pass must not publish gauges or advance
// the observation timestamp.
func (oko *ReservationKeeperObserver) recheckTrackedAcceptance(
	headNow uint64,
) error {
	for key, generation := range oko.trackedAcceptance {
		action, err := oko.spvChain.GetReservationAction(
			generation.reservationKey,
			generation.requestNonce,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to re-read tracked acceptance action [%v]/%d: [%w]",
				generation.reservationKey,
				generation.requestNonce,
				err,
			)
		}
		switch action.State {
		case tbtc.ReservationActionStateSettled,
			tbtc.ReservationActionStateVetoed,
			tbtc.ReservationActionStateSuperseded,
			tbtc.ReservationActionStateUnknown:
			delete(oko.trackedAcceptance, key)
			continue
		case tbtc.ReservationActionStateTimedOut:
			windowEdge := uint64(action.TimeoutAt) + uint64(action.TermSeconds)
			if headNow > windowEdge {
				delete(oko.trackedAcceptance, key)
				continue
			}
		}
		generation.action = action
	}
	return nil
}

// publishGauges sets all five keeper watchdog gauges on the recorder.
// It is called only after a fully successful pass, so a failed pass
// never touches any gauge: the values, and the observation timestamp
// in particular, hold their last state.
func (oko *ReservationKeeperObserver) publishGauges(
	headNow uint64,
	lateProofCount int,
	noTargetCount int,
	unrequestedCount int,
	feeDebt uint64,
) {
	oko.setGauge(
		clientinfo.MetricReservationAcceptanceLateProofAtRiskCount,
		float64(lateProofCount),
	)
	oko.setGauge(
		clientinfo.MetricReservationReanchorNoEligibleTargetCount,
		float64(noTargetCount),
	)
	oko.setGauge(
		clientinfo.MetricReservationUnrequestedDepositDeadlineAtRiskCount,
		float64(unrequestedCount),
	)
	oko.setGauge(
		clientinfo.MetricReservationVaultFeeDebtSat,
		float64(feeDebt),
	)
	oko.setGauge(
		clientinfo.MetricReservationKeeperObservationTimestamp,
		float64(headNow),
	)
}

// setGauge is a nil-safe guard around the recorder: a nil recorder
// (no metrics pipeline) disables publication, and a typed-nil
// *clientinfo.PerformanceMetrics wrapped in the interface is treated as
// disabled, mirroring incrementReservationCounter's guard.
func (oko *ReservationKeeperObserver) setGauge(name string, value float64) {
	if oko.recorder == nil {
		return
	}
	if pm, ok := oko.recorder.(*clientinfo.PerformanceMetrics); ok && pm == nil {
		return
	}
	oko.recorder.SetGauge(name, value)
}

// Run starts the observer's background loop: one observation pass at
// the given interval on every tick, running until ctx is done. Pass
// failures are logged and retried on the next tick - a failed pass
// leaves every gauge at its last value, and the staleness of the
// observation timestamp is itself the signal that the monitoring
// service alerts on. Run returns when ctx is done.
func (oko *ReservationKeeperObserver) Run(
	ctx context.Context,
	interval time.Duration,
) error {
	if interval <= 0 {
		return fmt.Errorf(
			"non-positive observation interval: [%v]",
			interval,
		)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// A first pass before the ticker loop so the gauges are live
	// without waiting a full interval, mirroring the sibling
	// watchers' initial-pass ordering.
	if err := oko.ObservationPass(); err != nil {
		reservationKeeperObserverLogger.Errorf(
			"reservation keeper observation pass failed: [%v]",
			err,
		)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := oko.ObservationPass(); err != nil {
				reservationKeeperObserverLogger.Errorf(
					"reservation keeper observation pass failed: [%v]",
					err,
				)
			}
		}
	}
}

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

// lateProofRiskWindowSeconds is how far ahead of the chain head a
// still-unproven acceptance generation's late-settlement window edge
// (snapshotted timeoutAt + termSeconds) may sit before the keeper
// observer counts it as a late-proof risk: within 7 days of the head,
// the proof may still land as a late settlement; past 7 days, the
// exposure is already governed by the other watchdog signals.
const lateProofRiskWindowSeconds = 7 * 24 * 3600

// unrequestedDepositRiskWindowSeconds is the 1-day window ahead of the
// chain head inside which a pending reserved deposit's last possible
// acceptance-request time makes it a deadline risk: the depositors'
// request window is closing within the day, and no request has been
// made yet.
const unrequestedDepositRiskWindowSeconds = 24 * 3600

// reanchorSigningWindowActionTimeoutFloor is the smallest live
// reservation action timeout under which a re-anchor request has no
// signing window at all: the contract requires now + 1 < timeoutAt -
// REQUEST_TIMEOUT_SAFETY_MARGIN (2h), and timeoutAt = now +
// reservationActionTimeout, so the window is open only when the
// action timeout exceeds 2h + 1s.
const reanchorSigningWindowActionTimeoutFloor = 2*3600 + 1

// walletCacheRefreshInterval is the wall-clock interval between re-reads
// of a cached registered wallet's on-chain state: newly-registered
// wallets are read on their discovery pass, and already-cached ones
// are re-read at most this often. The interval keeps the per-tick RPC
// volume independent of the total number of wallets ever registered.
const walletCacheRefreshInterval = 10 * time.Minute

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

// cachedWalletState is the observer's cached view of one registered
// wallet's on-chain data: the state and creation time needed to decide
// both re-anchor target eligibility and source-wallet retraction.
type cachedWalletState struct {
	data      *tbtc.WalletChainData
	fetchedAt time.Time
}

// ReservationKeeperObserver computes the reservation keeper watchdog
// gauges on a single bounded observation pass: it scans new
// acceptance-request, deposit-reveal, and wallet-registration events
// incrementally (from the cursor; never from genesis), re-checks the
// tracked sets, and publishes the five gauges only when the pass is
// fully successful.
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

// NewReservationKeeperObserver builds the keeper observer against spvChain,
// starting its bounded scans at activationBlock (math.MaxUint64 means
// reservations are inactive on this network: the scans stay no-ops and
// only new blocks are covered). recorder may be nil, in which case the
// gauges are computed but not published.
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

// scanNewWalletRegistrations incrementally scans new
// NewWalletRegistered events and adds every registered wallet to the
// cache; its on-chain data is fetched lazily by refreshWalletStates.
func (oko *ReservationKeeperObserver) scanNewWalletRegistrations() error {
	startBlock, endBlock, scan, err := nextReservationScanRange(
		oko.spvChain,
		oko.registrationScanCursor,
		oko.activationBlock,
	)
	if err != nil {
		return err
	}

	if !scan {
		oko.registrationScanCursor = endBlock
		return nil
	}

	return fetchPastEventsInChunks(
		func(startBlock, endBlock uint64) ([]*tbtc.NewWalletRegisteredEvent, error) {
			end := endBlock
			return oko.spvChain.PastNewWalletRegisteredEvents(
				&tbtc.NewWalletRegisteredEventFilter{
					StartBlock: startBlock,
					EndBlock:   &end,
				},
			)
		},
		startBlock,
		endBlock,
		func(events []*tbtc.NewWalletRegisteredEvent, chunkEnd uint64) {
			for _, event := range events {
				if _, known := oko.cachedWallets[event.WalletPublicKeyHash]; !known {
					oko.cachedWallets[event.WalletPublicKeyHash] = &cachedWalletState{}
				}
			}
			oko.registrationScanCursor = chunkEnd
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

// countLateProofsAtRisk counts the tracked acceptance generations whose
// late-settlement window edge (snapshotted timeoutAt + termSeconds)
// sits within 7 days of the chain head: unproven proofs whose deadline
// is coming up, which is what the page-allow-listed-maintainers
// escalation is for.
func (oko *ReservationKeeperObserver) countLateProofsAtRisk(
	headNow uint64,
) int {
	count := 0
	for _, generation := range oko.trackedAcceptance {
		if generation.action == nil {
			// Not yet re-read this pass; the re-check ran before
			// this count and fills it on success.
			continue
		}
		windowEdge := uint64(generation.action.TimeoutAt) +
			uint64(generation.action.TermSeconds)
		if windowEdge <= headNow+lateProofRiskWindowSeconds {
			count++
		}
	}
	return count
}

// countReanchorsWithoutTarget counts, per the contract, the Active
// reservations on MovingFunds or Closing wallets for which no target
// exists under the rules a source-wallet member (privileged) re-anchor
// faces after the hand-off capacity change: the target must be Live,
// different from the source, past the wallet creation period by the
// chain head (keep-core's IsReanchorTargetAged), and the contract
// checks that still apply to privileged callers must hold - the anchor
// amount floor (anchor above min amount + tx max fee, the only check
// not skipped by the privileged path) and a satisfiable signing
// window (action timeout above the floor; the per-wallet count/amount
// caps are skipped by privileged re-anchors and are NOT part of the
// gauge). The gauge counts the number of (wallet, Active-reservation)
// pairs lacking any target, so one target satisfying all rules for a
// wallet clears every reservation on it.
func (oko *ReservationKeeperObserver) countReanchorsWithoutTarget(
	reservationParameters *tbtc.ReservationParameters,
	walletParameters tbtc.WalletParameters,
	headTime time.Time,
) (int, error) {
	// A shared target set: any eligible target suffices for every
	// source, since the remaining per-reservation checks (anchor
	// floor, signing window) depend only on the reservation and the
	// live parameters, not on which target is chosen.
	targetExists := false
	for _, state := range oko.cachedWallets {
		if state.data == nil {
			continue
		}
		if state.data.State != tbtc.StateLive {
			continue
		}
		if !tbtc.IsReanchorTargetAged(
			state.data.CreatedAt,
			walletParameters.CreationPeriod,
			headTime,
			tbtc.ReanchorTargetAgeTolerance,
		) {
			continue
		}
		targetExists = true
		break
	}

	// The signing window is open only when the live action timeout
	// leaves at least one second past the 2h request-timeout safety
	// margin (now + 1 < now + actionTimeout - 2h).
	signingWindowSatisfiable :=
		uint64(reservationParameters.ReservationActionTimeout) >
			uint64(reanchorSigningWindowActionTimeoutFloor)
	// The anchor floor: the anchor must stay above the minimum
	// amount plus the max fee, the check privileged re-anchors do
	// NOT skip (the per-wallet caps are the skipped ones).
	anchorFloor := int64(
		reservationParameters.ReservationMinAmount +
			reservationParameters.ReservationTxMaxFee,
	)

	count := 0
	for sourcePkh, state := range oko.cachedWallets {
		if state.data == nil {
			continue
		}
		switch state.data.State {
		case tbtc.StateMovingFunds, tbtc.StateClosing:
		default:
			// A Live wallet's reservations are not retracted by
			// re-anchor; only governance rotates them.
			continue
		}

		keys, err := oko.spvChain.WalletReservations(sourcePkh)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to read reservations for source wallet [0x%x]: [%w]",
				sourcePkh,
				err,
			)
		}
		for _, key := range keys {
			reservation, err := oko.spvChain.GetReservation(key)
			if err != nil {
				return 0, fmt.Errorf(
					"failed to read reservation [%v] on source [0x%x]: [%w]",
					key,
					sourcePkh,
					err,
				)
			}
			if reservation.State != tbtc.ReservationStateActive {
				continue
			}
			anchorUtxo := reservation.AnchorUtxo
			anchorOk := anchorUtxo != nil && anchorUtxo.Value > anchorFloor
			if !targetExists || !signingWindowSatisfiable || !anchorOk {
				count++
			}
		}
	}

	return count, nil
}

// countUnrequestedDepositsAtRisk counts the tracked pending reserved
// deposits with no acceptance request made whose last possible request
// time - the contract's request check (requestReservationAcceptance in
// Reservation.sol): timeoutAt + 24h safety margin <= refundDeadline
// with timeoutAt = now + actionTimeout, so last possible request time
// is refundDeadline - 24h - actionTimeout - falls within 1 day of the
// chain head. Released, stale, and accepted deposits (the record
// cleared or a request made) are evicted and never counted. It
// returns the count plus the keys to evict.
func (oko *ReservationKeeperObserver) countUnrequestedDepositsAtRisk(
	headNow uint64,
	reservationParameters *tbtc.ReservationParameters,
) (int, []string, error) {
	riskWindowEnd := headNow + unrequestedDepositRiskWindowSeconds
	margin := uint64(reservationDepositRefundSafetyMarginSeconds)
	actionTimeout := uint64(reservationParameters.ReservationActionTimeout)

	count := 0
	var evicted []string
	for key, deposit := range oko.trackedDeposits {
		reserved, err := oko.spvChain.IsReservedDeposit(deposit.depositKey)
		if err != nil {
			return 0, nil, fmt.Errorf(
				"failed to read reserved-deposit record for [%v]: [%w]",
				deposit.depositKey,
				err,
			)
		}
		if !reserved {
			// Released to the pool, stale, or accepted: the
			// pending reserved-deposit record is cleared.
			evicted = append(evicted, key)
			continue
		}

		reservation, err := oko.spvChain.GetReservation(deposit.depositKey)
		if err != nil {
			return 0, nil, fmt.Errorf(
				"failed to read reservation for deposit [%v]: [%w]",
				deposit.depositKey,
				err,
			)
		}
		if reservation != nil && reservation.RequestNonce > 0 {
			// An acceptance request was made: no longer an
			// unrequested deposit.
			evicted = append(evicted, key)
			continue
		}

		// Last possible request time = refund deadline - 24h
		// safety margin - action timeout; underflow (the deadline
		// already inside the margin) means the request window is
		// closing right now and counts as at risk.
		var lastRequest uint64
		if deposit.refundDeadline > uint32(margin+actionTimeout) {
			lastRequest = uint64(deposit.refundDeadline) - margin - actionTimeout
		}
		if lastRequest <= riskWindowEnd {
			count++
		}
	}

	return count, evicted, nil
}

// refreshWalletStates re-reads the on-chain data of the cached wallets
// on a wall-clock refresh interval: newly-discovered wallets are read
// immediately, and every cached one at most once per
// walletCacheRefreshInterval. A failed read fails the pass, because a
// wallet with unknown or outdated state can hide a retiring source.
func (oko *ReservationKeeperObserver) refreshWalletStates() error {
	now := time.Now()
	for pkh, state := range oko.cachedWallets {
		if !state.fetchedAt.IsZero() &&
			now.Sub(state.fetchedAt) < walletCacheRefreshInterval {
			continue
		}
		wallet, err := oko.spvChain.GetWallet(pkh)
		if err != nil {
			return fmt.Errorf("failed to read wallet [0x%x]: [%w]", pkh, err)
		}
		state.data = wallet
		state.fetchedAt = now
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

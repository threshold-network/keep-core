package spv

import (
	"fmt"
	"math"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

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

// cachedWalletState is the observer's cached view of one registered
// wallet's on-chain data: the state and creation time needed to decide
// both re-anchor target eligibility and source-wallet retraction.
type cachedWalletState struct {
	data      *tbtc.WalletChainData
	fetchedAt time.Time
}

// scanNewWalletRegistrations incrementally scans new
// NewWalletRegistered events and adds every registered wallet to the
// cache; its on-chain data is fetched lazily by refreshWalletStates.
func (oko *ReservationKeeperObserver) scanNewWalletRegistrations() error {
	// Existing wallets remain reservation sources and targets after activation.
	startBlock, endBlock, _, err := nextReservationScanRange(
		oko.spvChain,
		oko.registrationScanCursor,
		0,
	)
	if err != nil {
		return err
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
	// NOT skip (the per-wallet caps are the skipped ones). The sum is
	// computed as a saturating addition: both operands are governance-set
	// uint64 values with no shared upper bound (the contract only
	// requires minAmount > txMaxFee > 0, so a large minAmount with a
	// small non-zero fee is accepted), and a wrapping sum would turn an
	// unattainable floor into a tiny one and make ordinary anchors look
	// re-anchorable. The sum is then compared against signed anchor
	// values, so clamp it into the int64 range: a floor above MaxInt64
	// is above any real anchor value.
	anchorFloorSats := reservationParameters.ReservationMinAmount +
		reservationParameters.ReservationTxMaxFee
	if anchorFloorSats < reservationParameters.ReservationMinAmount {
		// The addition wrapped: the true floor exceeds MaxUint64.
		anchorFloorSats = math.MaxUint64
	}
	anchorFloor := int64(math.MaxInt64)
	if anchorFloorSats <= math.MaxInt64 {
		anchorFloor = int64(anchorFloorSats)
	}

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
		marginAndTimeout := margin + actionTimeout
		if marginAndTimeout <= math.MaxUint32 &&
			deposit.refundDeadline > uint32(marginAndTimeout) {
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
	now := oko.nowFn()
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

package spv

import (
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// staleDepositRevealScanLookBackBlocks bounds the reveal-timestamp fallback
// scan. 30 days at 12s/block, mirroring the convention used across this
// package.
const staleDepositRevealScanLookBackBlocks = uint64(216000)

// StaleDepositResolution indicates the outcome of a stale deposit check
// to help callers (e.g. pollers) decide whether to keep or drop the deposit
// from tracking.
type StaleDepositResolution uint8

const (
	// StaleDepositResolutionUnknown is the zero value representing an unknown or errored resolution.
	StaleDepositResolutionUnknown StaleDepositResolution = iota
	// StaleDepositResolutionKeep indicates the deposit is still pending-stale and should be retained in the tracking set.
	StaleDepositResolutionKeep
	// StaleDepositResolutionDrop indicates the deposit is no longer a candidate for staleness (e.g. not reserved, settled action) and can be dropped from tracking.
	StaleDepositResolutionDrop
	// StaleDepositResolutionNotified indicates the deposit was confirmed stale and the notification was submitted.
	StaleDepositResolutionNotified
)

// ReservationStaleDepositWatcher observes deposit-revealed events and
// notifies the Bridge when a reserved deposit's acceptance window expired
// without the assigned wallet becoming live.
//
// A reserved deposit is a deposit that was revealed against a reservation
// vault address. The Bridge records the assigned wallet via
// `ReservedDepositWallet`. If that wallet fails to transition to StateLive
// within the reservation action timeout window, the deposit must be released
// back to the default deposit sweep path; otherwise it sits orphaned
// because the anchor can never be produced. The watcher is the backstop that
// flips the deposit's bookkeeping when the wallet never shows up.
type ReservationStaleDepositWatcher struct {
	spvChain        Chain
	notified        map[string]struct{}
	memoizedTimeout map[string]staleDepositTimeoutMemo
}

// staleDepositTimeoutMemo caches a reveal-derived staleness deadline
// alongside the ReservationActionTimeout governance parameter it was
// derived from. If governance later changes ReservationActionTimeout, the
// stored parameter no longer matches the live one and the memo is
// recomputed instead of silently reusing a stale deadline.
type staleDepositTimeoutMemo struct {
	timeoutAt                uint32
	reservationActionTimeout uint32
}

// NewReservationStaleDepositWatcher constructs a stale-deposit watcher
// bound to the given chain.
func NewReservationStaleDepositWatcher(
	spvChain Chain,
) *ReservationStaleDepositWatcher {
	return &ReservationStaleDepositWatcher{
		spvChain:        spvChain,
		notified:        make(map[string]struct{}),
		memoizedTimeout: make(map[string]staleDepositTimeoutMemo),
	}
}

// CheckStaleReservedDeposit is the synchronous core of the watcher. It is
// invoked by the integration's polling or deferred callback once the action
// timeout window may have elapsed.
//
// The function may submit a Bridge notification (NotifyStaleReservedDeposit)
// as a side effect, and it caches derived timeouts and notification state
// on the receiver across calls. There is no internal scheduling; the
// caller owns invocation lifecycle and synchronization.
//
// Conditions for notification:
//
//  1. `IsReservedDeposit(depositKey)` returns true. A non-reserved deposit
//     is the default sweep path's responsibility; the watcher must not
//     interfere with it.
//  2. The reservation's assigned wallet exists and is NOT in StateLive.
//     A live wallet is expected to anchor the deposit itself; the action
//     timeout window only applies when the wallet is missing or has not
//     progressed to live.
//  3. The action timeout has elapsed. The watcher derives the timeout
//     from the reservation action record at the current reservation
//     RequestNonce. If the action has already been advanced
//     (Settled/TimedOut/Superseded/Vetoed), the deposit is no longer in
//     the pending-stale window and the watcher skips it without notifying.
//
// Parameters:
//   - depositKey: the deposit identifier reported by the Bridge.
//   - now:        the UNIX timestamp against which the action timeout is
//     compared. Tests pass an explicit value; production passes
//     time.Now().Unix() cast to uint32.
func (rsdw *ReservationStaleDepositWatcher) CheckStaleReservedDeposit(
	depositKey *big.Int,
	now uint32,
) (StaleDepositResolution, error) {
	if depositKey == nil {
		return StaleDepositResolutionUnknown, fmt.Errorf("deposit key must not be nil")
	}
	if _, ok := rsdw.notified[depositKey.String()]; ok {
		return StaleDepositResolutionNotified, nil
	}

	isReserved, err := rsdw.spvChain.IsReservedDeposit(depositKey)
	if err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to determine if deposit [%v] is reserved: [%v]",
			depositKey,
			err,
		)
	}
	if !isReserved {
		logger.Debugf(
			"deposit [%v] is not a reserved deposit; skipping stale check",
			depositKey,
		)
		return StaleDepositResolutionDrop, nil
	}

	walletPublicKeyHash, err := rsdw.spvChain.ReservedDepositWallet(depositKey)
	if err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to fetch wallet for reserved deposit [%v]: [%v]",
			depositKey,
			err,
		)
	}

	// The Bridge only assigns a non-zero wallet to a reserved deposit.
	// Defensive: if the wallet is zero the deposit bookkeeping is broken;
	// rather than notify on partial information, we skip with a warning.
	if walletPublicKeyHash == ([20]byte{}) {
		logger.Warnf(
			"reserved deposit [%v] has no wallet assigned; "+
				"skipping stale notification",
			depositKey,
		)
		return StaleDepositResolutionDrop, nil
	}

	wallet, err := rsdw.spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to fetch wallet [0x%x] for reserved deposit [%v]: [%v]",
			walletPublicKeyHash,
			depositKey,
			err,
		)
	}

	// Wallet live: anchor is expected on its own. The watcher does not
	// interfere.
	if wallet.State == tbtc.StateLive {
		logger.Debugf(
			"reserved deposit [%v] assigned to live wallet [0x%x]; "+
				"anchor expected; skipping stale notification",
			depositKey,
			walletPublicKeyHash,
		)
		return StaleDepositResolutionKeep, nil
	}

	// In m1 the reservation key and deposit key share the same identifier
	// space exposed by the Bridge (ReservedDepositWallet and Reservation
	// are both keyed by the same value); future revisions of the Bridge
	// may introduce disjoint identifiers, in which case this direct use
	// of depositKey as reservationKey must be replaced with a real lookup.
	reservationKey := depositKey

	reservation, err := rsdw.spvChain.GetReservation(reservationKey)
	if err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to fetch reservation [%v]: [%v]",
			reservationKey,
			err,
		)
	}

	var timeoutAt uint32
	if reservation.RequestNonce == 0 {
		// No acceptance action generation has ever been requested on-chain
		// for this reservation. Derive the staleness deadline from the
		// deposit's own reveal timestamp instead of the (nonexistent) action's
		// TimeoutAt: find the DepositRevealed event for this deposit key among
		// the wallet's events, then load the deposit request's RevealedAt.
		derivedTimeout, err := rsdw.deriveTimeoutFromReveal(
			depositKey,
			walletPublicKeyHash,
		)
		if err != nil {
			return StaleDepositResolutionUnknown, err
		}
		timeoutAt = derivedTimeout
	} else {
		action, err := rsdw.spvChain.GetReservationAction(
			reservationKey,
			reservation.RequestNonce,
		)
		if err != nil {
			return StaleDepositResolutionUnknown, fmt.Errorf(
				"failed to fetch reservation action for [%v] nonce [%d]: [%v]",
				reservationKey,
				reservation.RequestNonce,
				err,
			)
		}

		if action.State == tbtc.ReservationActionStateUnknown {
			// Confirmed no action generation exists yet on-chain for this nonce.
			// Derive the staleness deadline from the deposit's own reveal timestamp.
			derivedTimeout, err := rsdw.deriveTimeoutFromReveal(
				depositKey,
				walletPublicKeyHash,
			)
			if err != nil {
				return StaleDepositResolutionUnknown, err
			}
			timeoutAt = derivedTimeout
		} else if action.State != tbtc.ReservationActionStatePending {
			logger.Debugf(
				"reservation [%v] acceptance action state=%s; "+
					"deposit [%v] is no longer pending-stale; skipping",
				reservationKey,
				action.State,
				depositKey,
			)
			return StaleDepositResolutionDrop, nil
		} else {
			timeoutAt = action.TimeoutAt
		}
	}

	if now <= timeoutAt {
		logger.Debugf(
			"reserved deposit [%v] action timeout at [%d] not yet reached "+
				"(now=%d); deferring stale notification",
			depositKey,
			timeoutAt,
			now,
		)
		return StaleDepositResolutionKeep, nil
	}

	if err := rsdw.spvChain.NotifyStaleReservedDeposit(depositKey); err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to notify stale reserved deposit [%v]: [%v]",
			depositKey,
			err,
		)
	}
	rsdw.notified[depositKey.String()] = struct{}{}

	logger.Infof(
		"notified stale reserved deposit [%v] "+
			"(wallet [0x%x] state=%s, action timeout %d)",
		depositKey,
		walletPublicKeyHash,
		wallet.State,
		timeoutAt,
	)

	return StaleDepositResolutionNotified, nil
}

// forgetDeposit clears any cached notification state and memoized
// staleness deadline held for the given deposit key. The poller invokes
// this once a deposit resolves to Drop or Notified, so a resolved
// deposit's per-call cache entries do not linger in these maps for the
// remaining life of the process.
func (rsdw *ReservationStaleDepositWatcher) forgetDeposit(depositKey *big.Int) {
	key := depositKey.String()
	delete(rsdw.notified, key)
	delete(rsdw.memoizedTimeout, key)
}

func (rsdw *ReservationStaleDepositWatcher) deriveTimeoutFromReveal(
	depositKey *big.Int,
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	params, paramsErr := rsdw.spvChain.ReservationParameters()
	if paramsErr != nil {
		return 0, fmt.Errorf(
			"failed to load reservation parameters for staleness "+
				"deadline derivation: [%v]",
			paramsErr,
		)
	}

	if memo, ok := rsdw.memoizedTimeout[depositKey.String()]; ok &&
		memo.reservationActionTimeout == params.ReservationActionTimeout {
		return memo.timeoutAt, nil
	}

	blockCounter, err := rsdw.spvChain.BlockCounter()
	if err != nil {
		return 0, fmt.Errorf(
			"failed to get block counter for staleness deadline derivation: [%v]",
			err,
		)
	}
	if blockCounter == nil {
		return 0, fmt.Errorf(
			"failed to get block counter for staleness deadline derivation: nil block counter",
		)
	}
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return 0, fmt.Errorf(
			"failed to get current block for staleness deadline derivation: [%v]",
			err,
		)
	}

	startBlock := uint64(0)
	if currentBlock > staleDepositRevealScanLookBackBlocks {
		startBlock = currentBlock - staleDepositRevealScanLookBackBlocks
	}

	events, eventsErr := rsdw.spvChain.PastDepositRevealedEvents(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          startBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if eventsErr != nil {
		return 0, fmt.Errorf(
			"failed to fetch deposit revealed events for staleness "+
				"deadline derivation: [%v]",
			eventsErr,
		)
	}

	var matchingEvent *tbtc.DepositRevealedEvent
	for _, event := range events {
		if rsdw.spvChain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		).Cmp(depositKey) == 0 {
			matchingEvent = event
			break
		}
	}
	if matchingEvent == nil {
		return 0, fmt.Errorf(
			"no matching DepositRevealed event for deposit [%v]",
			depositKey,
		)
	}

	depositRequest, found, requestErr := rsdw.spvChain.GetDepositRequest(
		matchingEvent.FundingTxHash,
		matchingEvent.FundingOutputIndex,
	)
	if requestErr != nil {
		return 0, fmt.Errorf(
			"failed to load deposit request for staleness deadline "+
				"derivation: [%v]",
			requestErr,
		)
	}
	if !found {
		return 0, fmt.Errorf(
			"deposit request not found for deposit [%v]",
			depositKey,
		)
	}

	result := uint32(depositRequest.RevealedAt.Unix()) + params.ReservationActionTimeout
	rsdw.memoizedTimeout[depositKey.String()] = staleDepositTimeoutMemo{
		timeoutAt:                result,
		reservationActionTimeout: params.ReservationActionTimeout,
	}
	return result, nil
}

package spv

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// staleDepositRevealScanLookBackBlocks is a thin alias for
// reservationDefaultLookBackBlocks (see reservation_wiring.go), the
// single canonical 30-day/12s-per-block lookback bound shared by every
// reservation watcher's startup/first-pass catch-up scan - including
// this watcher's own pollTick, which uses the same bound for its
// incremental DepositRevealed scan. It keeps this exact name because
// reservation_stale_deposit_watch_test.go references it directly.
const staleDepositRevealScanLookBackBlocks = reservationDefaultLookBackBlocks

// reservationStaleDepositParkedReconcileInterval bounds how often the
// stale-deposit poller re-checks deposits parked because their assigned
// wallet was observed Live (see parkedReconcile). A Live wallet may
// still transition away from Live before anchoring its deposit, so
// parked deposits are not abandoned - they are just re-checked far less
// often than the actively-polled set, since a Live wallet is
// overwhelmingly expected to anchor its own deposit without further
// intervention.
const reservationStaleDepositParkedReconcileInterval = 30 * time.Minute

// StaleDepositResolution indicates the outcome of a stale deposit check
// to help callers (e.g. pollers) decide whether to keep or drop the deposit
// from tracking.
type StaleDepositResolution uint8

const (
	// StaleDepositResolutionUnknown is the zero value, returned alongside
	// a non-nil error whenever the check could not be completed (a chain
	// call failed or the deposit key was invalid). No watcher state was
	// updated on this call; the caller must retain the deposit in its
	// tracking set and retry on the next tick.
	StaleDepositResolutionUnknown StaleDepositResolution = iota
	// StaleDepositResolutionKeep indicates the deposit is still
	// pending-stale (the wallet has not gone live and the action timeout
	// has not yet elapsed) and must be retained in the caller's tracking
	// set for the next tick; no watcher state changes as a result.
	StaleDepositResolutionKeep
	// StaleDepositResolutionDrop indicates the deposit is no longer a
	// staleness candidate (not reserved, no wallet assigned, or its
	// reservation action already advanced past pending) and must be
	// removed from the caller's tracking set. The caller should also call
	// forgetDeposit to release any per-deposit cache entries the watcher
	// holds for this key.
	StaleDepositResolutionDrop
	// StaleDepositResolutionNotified indicates the deposit was confirmed
	// stale and NotifyStaleReservedDeposit was submitted, on this call or
	// a prior one (see the `notified` field). The caller must remove the
	// deposit from its tracking set and call forgetDeposit so the watcher
	// does not retain state for a deposit it will never check again.
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

	// walletTickCache memoizes GetWallet results within a single poll
	// tick, keyed by wallet public key hash. `now` is an opaque
	// tick-generation token: the caller computes it once per poll
	// iteration and passes that identical value to every deposit checked
	// in that iteration (CheckStaleReservedDeposit forwards its own `now`
	// parameter here unchanged). A `now` that differs from
	// walletTickCacheNow signals a new tick and invalidates the cache; a
	// repeated `now` signals the same tick and reuses it. This lets
	// deposits assigned to the same wallet share one GetWallet call per
	// tick instead of paying for one per deposit.
	walletTickCacheValid bool
	walletTickCacheNow   uint32
	walletTickCache      map[[20]byte]walletFetchResult

	// reservationParamsTickCache applies the identical
	// tick-generation-token contract described above to the
	// ReservationParameters fetch: a `now` that differs from
	// reservationParamsTickCacheNow signals a new tick and triggers a
	// fresh fetch; a repeated `now` reuses it. deriveTimeoutFromReveal
	// relies on this so every deposit checked within one tick shares a
	// single governance-parameter fetch instead of paying for one per
	// deposit.
	reservationParamsTickCacheValid bool
	reservationParamsTickCacheNow   uint32
	reservationParamsTickCache      reservationParamsFetchResult

	// operatorAddress identifies this process for
	// reservationOperatorStaggerOffset (see reservation_wiring.go), used
	// to stagger a deposit's FIRST NotifyStaleReservedDeposit attempt.
	// Left unset (the zero address) until SetOperatorAddress is called;
	// production wiring (WireReservationWatchers) calls it once after
	// construction.
	operatorAddress common.Address

	// attempted records every deposit key for which
	// NotifyStaleReservedDeposit has already been attempted at least
	// once (success or failure), so CheckStaleReservedDeposit knows to
	// apply the stagger offset only to the FIRST attempt (see
	// reservationOperatorStaggerOffset).
	attempted map[string]struct{}

	// pending and parked, together with lastSeenBlock, are this
	// watcher's own poll-loop cross-tick state (see Run, pollTick, and
	// parkedReconcile): the incremental DepositRevealed scan cursor and
	// the two tracked-deposit sets. Folding this state into the watcher
	// itself - rather than a separate type the wiring layer had to
	// construct and thread through free functions - mirrors
	// ReservationActionTimeoutWatcher's self-contained Run loop.
	// pending holds deposits re-checked every tick: newly revealed
	// reserved deposits, and deposits whose assigned wallet is not (or
	// is no longer) Live. parked holds deposits assigned to a Live
	// wallet, excluded from the per-tick re-check and only revisited by
	// the slower parkedReconcile pass.
	pending       map[string]*big.Int
	parked        map[string]*big.Int
	lastSeenBlock uint64
}

// walletFetchResult caches the outcome of a single GetWallet call,
// including an error, so a failed fetch for a wallet is not retried for
// every deposit sharing that wallet within the same poll tick.
type walletFetchResult struct {
	wallet *tbtc.WalletChainData
	err    error
}

// reservationParamsFetchResult caches the outcome of a single
// ReservationParameters call, including an error, so a failed fetch is
// not retried for every deposit sharing the same poll tick.
type reservationParamsFetchResult struct {
	params *tbtc.ReservationParameters
	err    error
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
		walletTickCache: make(map[[20]byte]walletFetchResult),
		attempted:       make(map[string]struct{}),
		pending:         make(map[string]*big.Int),
		parked:          make(map[string]*big.Int),
	}
}

// SetOperatorAddress wires the operator's own chain address into the
// watcher so CheckStaleReservedDeposit can derive a deterministic
// per-operator stagger offset for a deposit's FIRST
// NotifyStaleReservedDeposit attempt (see
// reservationOperatorStaggerOffset). It is a post-construction setter,
// mirroring ReservationActionTimeoutWatcher.SetOperatorAddress, so
// existing NewReservationStaleDepositWatcher call sites keep compiling
// unchanged; production wiring (WireReservationWatchers) calls it once
// after construction.
func (rsdw *ReservationStaleDepositWatcher) SetOperatorAddress(
	operatorAddress common.Address,
) {
	rsdw.operatorAddress = operatorAddress
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

	wallet, err := rsdw.getWalletForTick(walletPublicKeyHash, now)
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
			now,
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
				now,
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

	depositKeyStr := depositKey.String()
	if _, alreadyAttempted := rsdw.attempted[depositKeyStr]; !alreadyAttempted {
		// FIRST notify attempt for this deposit: stagger it by a
		// deterministic per-operator offset (see
		// reservationOperatorStaggerOffset in reservation_wiring.go) so
		// that many reservation-enabled operators independently
		// discovering the same overdue deposit do not all submit their
		// first NotifyStaleReservedDeposit attempt in the same poll
		// tick and collide on-chain. Unlike the action-timeout watcher,
		// this watcher has no dedicated renotify-interval backoff for
		// retries - a failed attempt is simply retried on the very
		// next poll tick (see pollTick) - so only the FIRST attempt is
		// staggered; reusing actionTimeoutRenotifyInterval as the
		// stagger window avoids introducing yet another near-duplicate
		// interval constant.
		offset := reservationOperatorStaggerOffset(
			rsdw.operatorAddress,
			depositKeyStr,
			actionTimeoutRenotifyInterval,
		)
		if now < timeoutAt+offset {
			return StaleDepositResolutionKeep, nil
		}
		rsdw.attempted[depositKeyStr] = struct{}{}
	}

	if err := rsdw.spvChain.NotifyStaleReservedDeposit(depositKey); err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to notify stale reserved deposit [%v]: [%v]",
			depositKey,
			err,
		)
	}
	rsdw.notified[depositKeyStr] = struct{}{}

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

// getWalletForTick fetches the given wallet's on-chain state, memoizing
// the result for the duration of one poll tick so every deposit assigned
// to the same wallet reuses a single GetWallet call instead of issuing
// one per deposit. `now` is an opaque tick-generation token, not a
// literal timestamp used in comparisons: the caller computes it once per
// poll iteration and passes that identical value to every deposit
// checked in that iteration (CheckStaleReservedDeposit forwards its own
// `now` parameter here unchanged). A `now` value that differs from the
// cached one signals a new tick and the cache is dropped and rebuilt
// from scratch; a repeated `now` signals the same tick and reuses it.
func (rsdw *ReservationStaleDepositWatcher) getWalletForTick(
	walletPublicKeyHash [20]byte,
	now uint32,
) (*tbtc.WalletChainData, error) {
	if !rsdw.walletTickCacheValid || now != rsdw.walletTickCacheNow {
		rsdw.walletTickCacheValid = true
		rsdw.walletTickCacheNow = now
		rsdw.walletTickCache = make(map[[20]byte]walletFetchResult)
	}

	if cached, ok := rsdw.walletTickCache[walletPublicKeyHash]; ok {
		return cached.wallet, cached.err
	}

	wallet, err := rsdw.spvChain.GetWallet(walletPublicKeyHash)
	rsdw.walletTickCache[walletPublicKeyHash] = walletFetchResult{
		wallet: wallet,
		err:    err,
	}
	return wallet, err
}

// getReservationParametersForTick fetches the live ReservationParameters,
// applying the identical tick-generation-token contract as
// getWalletForTick above: a `now` that differs from the cached tick
// token signals a new tick and triggers a fresh fetch, while a repeated
// `now` reuses the cached result. deriveTimeoutFromReveal relies on this
// so its per-deposit staleness comparison against the memoized deadline
// (see staleDepositTimeoutMemo) pays for the governance-parameter fetch
// at most once per tick rather than once per deposit.
func (rsdw *ReservationStaleDepositWatcher) getReservationParametersForTick(
	now uint32,
) (*tbtc.ReservationParameters, error) {
	if !rsdw.reservationParamsTickCacheValid || now != rsdw.reservationParamsTickCacheNow {
		rsdw.reservationParamsTickCacheValid = true
		rsdw.reservationParamsTickCacheNow = now
		params, err := rsdw.spvChain.ReservationParameters()
		rsdw.reservationParamsTickCache = reservationParamsFetchResult{
			params: params,
			err:    err,
		}
	}
	return rsdw.reservationParamsTickCache.params, rsdw.reservationParamsTickCache.err
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
	delete(rsdw.attempted, key)
}

// deriveTimeoutFromReveal computes the staleness deadline for a reserved
// deposit that has no reservation action recorded yet, from the
// deposit's own DepositRevealed timestamp plus the live
// ReservationActionTimeout governance parameter. It checks its own memo
// cache (memoizedTimeout) first: an existing memo whose
// reservationActionTimeout still matches the live governance value
// short-circuits the block scan and event/deposit-request lookups
// entirely. `now` only identifies the poll tick so the
// governance-parameter fetch itself is amortized across every deposit
// checked in that tick (see getReservationParametersForTick); it plays
// no role in the derivation or the staleness comparison.
func (rsdw *ReservationStaleDepositWatcher) deriveTimeoutFromReveal(
	depositKey *big.Int,
	walletPublicKeyHash [20]byte,
	now uint32,
) (uint32, error) {
	memo, hasMemo := rsdw.memoizedTimeout[depositKey.String()]

	params, paramsErr := rsdw.getReservationParametersForTick(now)
	if paramsErr != nil {
		return 0, fmt.Errorf(
			"failed to load reservation parameters for staleness "+
				"deadline derivation: [%v]",
			paramsErr,
		)
	}

	if hasMemo && memo.reservationActionTimeout == params.ReservationActionTimeout {
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

// trackedCount returns the total number of deposits currently tracked by
// this watcher's poll loop, across both the actively-polled and parked
// sets.
func (rsdw *ReservationStaleDepositWatcher) trackedCount() int {
	return len(rsdw.pending) + len(rsdw.parked)
}

// isWalletLive reports whether depositKey's currently assigned wallet is
// in StateLive. It is only called for a deposit whose
// CheckStaleReservedDeposit resolution was Keep (i.e. still reserved),
// so ReservedDepositWallet is expected to resolve to a real assigned
// wallet.
func (rsdw *ReservationStaleDepositWatcher) isWalletLive(depositKey *big.Int) (bool, error) {
	walletPublicKeyHash, err := rsdw.spvChain.ReservedDepositWallet(depositKey)
	if err != nil {
		return false, fmt.Errorf(
			"failed to resolve assigned wallet: [%w]",
			err,
		)
	}
	wallet, err := rsdw.spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf(
			"failed to fetch assigned wallet: [%w]",
			err,
		)
	}
	return wallet.State == tbtc.StateLive, nil
}

// pollTick runs one stale-deposit poll pass: it fetches deposit-revealed
// events since rsdw.lastSeenBlock, adds every reserved deposit among
// them to the actively-polled (pending) set, then re-runs
// CheckStaleReservedDeposit for every deposit already in that set. A
// deposit is dropped entirely once it resolves Drop (no longer
// reserved, released to the default sweep path, or swept) or Notified
// (its acceptance action has advanced past pending) - both mean it can
// never go stale again, so re-checking it forever would be wasted
// RPCs. A deposit that instead resolves Keep because its assigned
// wallet has gone Live is moved to the parked set rather than
// re-checked every tick going forward: the wallet may still transition
// away from Live (e.g. MovingFunds/Closing/Terminated) before
// anchoring, so it cannot be abandoned, but re-reading it every minute
// for the rest of the process lifetime would make steady-state cost
// grow without bound as more deposits anchor successfully. Parked
// deposits are instead revisited by parkedReconcile, far less often.
//
// The poller is intentionally tolerant of chain errors: a transient RPC
// failure logs and continues rather than aborting the poll pass.
// Returns the total tracked count (pending + parked) after the tick,
// for callers that want a results signal (see
// WireReservationWatchers's misconfiguration self-check).
func (rsdw *ReservationStaleDepositWatcher) pollTick(now uint32) int {
	blockCounter, err := rsdw.spvChain.BlockCounter()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to get block counter: [%v]",
			err,
		)
		return rsdw.trackedCount()
	}
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to get current block: [%v]",
			err,
		)
		return rsdw.trackedCount()
	}

	startBlock := rsdw.lastSeenBlock
	if startBlock == 0 && currentBlock > staleDepositRevealScanLookBackBlocks {
		startBlock = currentBlock - staleDepositRevealScanLookBackBlocks
	}

	events, err := rsdw.spvChain.PastDepositRevealedEvents(
		&tbtc.DepositRevealedEventFilter{
			StartBlock: startBlock + 1,
			EndBlock:   &currentBlock,
		},
	)
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to fetch deposit revealed "+
				"events: [%v]",
			err,
		)
		return rsdw.trackedCount()
	}

	params, err := rsdw.spvChain.ReservationParameters()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to fetch reservation "+
				"parameters: [%v]",
			err,
		)
		return rsdw.trackedCount()
	}

	for _, event := range events {
		if event.Vault == nil || !strings.EqualFold(string(*event.Vault), string(params.ReservationVault)) {
			continue
		}

		depositKey := rsdw.spvChain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		isReserved, err := rsdw.spvChain.IsReservedDeposit(depositKey)
		if err != nil {
			reservationWiringLogger.Errorf(
				"stale-deposit poll failed to check if deposit "+
					"[%v] is reserved: [%v]",
				depositKey,
				err,
			)
			// Track it for retry instead of dropping it: this
			// window's event won't be re-fetched once
			// lastSeenBlock advances below, so silently skipping
			// here would permanently orphan the deposit on one
			// transient RPC flake. CheckStaleReservedDeposit
			// performs its own independent IsReservedDeposit
			// re-check on every tick and resolves to Drop if the
			// deposit genuinely isn't reserved, so tracking it
			// speculatively here is safe.
			rsdw.pending[depositKey.String()] = depositKey
			continue
		}
		if !isReserved {
			continue
		}

		rsdw.pending[depositKey.String()] = depositKey
	}

	rsdw.lastSeenBlock = currentBlock

	for key, depositKey := range rsdw.pending {
		resolution, err := rsdw.CheckStaleReservedDeposit(depositKey, now)
		if err != nil {
			reservationWiringLogger.Errorf(
				"stale-deposit poll failed to check deposit "+
					"[%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}

		switch resolution {
		case StaleDepositResolutionDrop, StaleDepositResolutionNotified:
			delete(rsdw.pending, key)
			rsdw.forgetDeposit(depositKey)
		case StaleDepositResolutionKeep:
			live, err := rsdw.isWalletLive(depositKey)
			if err != nil {
				reservationWiringLogger.Warnf(
					"stale-deposit poll failed to check whether "+
						"deposit [%v]'s assigned wallet is live; "+
						"keeping it in the actively-polled set: [%v]",
					depositKey,
					err,
				)
				continue
			}
			if live {
				delete(rsdw.pending, key)
				rsdw.parked[key] = depositKey
			}
		}
	}

	return rsdw.trackedCount()
}

// parkedReconcile re-checks every parked deposit (assigned to a Live
// wallet at last check). A deposit resolving Drop or Notified is
// evicted entirely; one resolving Keep whose assigned wallet is no
// longer Live is reactivated into the actively-polled (pending) set so
// it starts being re-checked every tick again.
func (rsdw *ReservationStaleDepositWatcher) parkedReconcile(now uint32) {
	for key, depositKey := range rsdw.parked {
		resolution, err := rsdw.CheckStaleReservedDeposit(depositKey, now)
		if err != nil {
			reservationWiringLogger.Errorf(
				"stale-deposit parked reconcile failed to check "+
					"deposit [%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}

		switch resolution {
		case StaleDepositResolutionDrop, StaleDepositResolutionNotified:
			delete(rsdw.parked, key)
			rsdw.forgetDeposit(depositKey)
		case StaleDepositResolutionKeep:
			live, err := rsdw.isWalletLive(depositKey)
			if err != nil {
				reservationWiringLogger.Warnf(
					"stale-deposit parked reconcile failed to check "+
						"whether deposit [%v]'s assigned wallet is "+
						"still live; leaving it parked until the next "+
						"reconcile pass: [%v]",
					depositKey,
					err,
				)
				continue
			}
			if !live {
				delete(rsdw.parked, key)
				rsdw.pending[key] = depositKey
			}
		}
	}
}

// Run starts the background poll loop, mirroring
// ReservationActionTimeoutWatcher.Run. It returns when ctx is done or
// when a non-positive interval is supplied. Every interval tick runs
// one poll pass over the tracked deposit set (pollTick) and, every
// reservationStaleDepositParkedReconcileInterval, additionally
// reconciles the parked set (parkedReconcile).
func (rsdw *ReservationStaleDepositWatcher) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf(
			"stale-deposit watcher requires a positive poll interval",
		)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastParkedReconcile := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		now := uint32(time.Now().Unix())
		rsdw.pollTick(now)

		if time.Since(lastParkedReconcile) >= reservationStaleDepositParkedReconcileInterval {
			lastParkedReconcile = time.Now()
			rsdw.parkedReconcile(now)
		}
	}
}

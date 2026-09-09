package spv

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/ipfs/go-log/v2"
)

var reservationWiringLogger = log.Logger("keep-maintainer-spv-reservations")

// DefaultReservationStaleDepositPollInterval is the default poll interval used
// by the stale-deposit watcher fallback loop. The Bridge does not expose a
// live subscription for DepositRevealed in m1, so the wiring layer falls back
// to PastDepositRevealedEvents on a coarse interval and dispatches each new
// reveal to the watcher. The interval mirrors the action-timeout poll
// cadence so a single tick covers both reservation timers.
const DefaultReservationStaleDepositPollInterval = 1 * time.Minute

// DefaultReservationActionTimeoutPollInterval is the default, fixed poll
// interval for the action-timeout watcher's Run loop - the background
// loop WireReservationWatchers starts and the only way the watcher is
// driven in production. It is intentionally conservative (1 minute) to
// limit the Bridge load from the per-tracked-action GetReservationAction
// reads Run issues on every tick.
const DefaultReservationActionTimeoutPollInterval = 1 * time.Minute

// reservationStrandingStartupScanLookBackBlocks bounds the stranding
// watcher's startup catch-up scan of past wallet registrations. 30 days at
// 12s/block, mirroring the convention used across this package.
//
// This is an accepted operational limitation, not a gap covered elsewhere:
// a wallet that closed or was terminated more than this bound before the
// process started has no path to stranding notification. The startup scan
// does not look back far enough to observe its registration, and the live
// OnWalletClosed subscription only observes transitions occurring from this
// point forward - it cannot replay an already-emitted close event. No other
// re-check mechanism in this package covers this case. WireReservationWatchers
// logs a warning whenever the bound actually truncates the scan window, so
// the limitation is visible in startup logs rather than silent.
const reservationStrandingStartupScanLookBackBlocks = uint64(216000)

// reservationStrandingStartupScanRetryDelay bounds how long the startup
// catch-up scan's second-chance retry pass (see
// retryReservationStrandingStartupScan) waits before re-attempting wallets
// whose GetWallet call failed on every one of the first 3 attempts. It runs
// once, after WireReservationWatchers has already returned, giving a
// transient RPC hiccup time to clear without blocking client/maintainer
// startup on it.
const reservationStrandingStartupScanRetryDelay = 30 * time.Second

// WalletClosedChain defines the chain interface required to subscribe to
// wallet close events.
type WalletClosedChain interface {
	OnWalletClosed(
		handler func(event *tbtc.WalletClosedEvent),
	) subscription.EventSubscription
}

// WireReservationWatchers is the reservation watcher integration entry
// point: it constructs the three reservation watchers (stranding,
// stale-deposit, action-timeout), wires their Bridge-facing notifiers to
// the chain, and subscribes/starts each watcher against its source. The
// three watchers are mandatory, permissionless, network-wide duties, not
// leader-election duties: every process capable of driving them - both
// cmd/start.go's client process and this package's own spv.Initialize -
// calls this once at startup, each gated on its own LeaderDutiesEnabled
// flag. Running it from both when both processes happen to share one
// deployment is redundant but harmless: a Notify* call against an
// already-notified or already-settled reservation is a no-op on the
// Bridge.
//
// `walletClosedChain` supplies the OnWalletClosed event subscription;
// `spvChain` supplies the reservation data reads, event queries, and
// Notify* writes. `ctx` controls the goroutine lifetimes started by the
// wiring function.
//
// `pairedFlagEnabled` reports whether the caller could reliably confirm
// that the counterpart process's own LeaderDutiesEnabled flag is also
// enabled. Pass true when the caller has no reliable visibility into that
// flag (e.g. spv.Initialize, which has no access to the client's
// Tbtc.Reservations config) to skip the misconfiguration self-check below;
// pass the caller's best-effort read otherwise (e.g. cmd/start.go, which
// can read Maintainer.Spv.Reservations.LeaderDutiesEnabled even though the
// start command doesn't require that config category - see its call site
// for why that reading, alone, is only ever a warning signal).
//
// Self-check: once wiring completes, if pairedFlagEnabled is false AND all
// three watchers' initial scans found zero reservation activity on-chain
// (no wallet registrations, no reserved deposits, no pending reservation
// actions), WireReservationWatchers returns a hard error instead of only
// logging a warning. Either signal alone is too weak to act on - a false
// paired-flag reading can be a legitimate split deployment (see
// cmd/start.go), and zero activity alone can be a genuinely quiet, freshly
// activated network - but together they are a strong indicator that this
// process is misconfigured (wrong flags, wrong network, or wrong contract
// address) rather than simply idle.
func WireReservationWatchers(
	ctx context.Context,
	walletClosedChain WalletClosedChain,
	spvChain Chain,
	pairedFlagEnabled bool,
) error {
	if walletClosedChain == nil {
		return fmt.Errorf("wallet closed chain must not be nil")
	}
	if spvChain == nil {
		return fmt.Errorf("spv chain must not be nil")
	}

	reservationWiringLogger.Infof(
		"wiring reservation watchers; ensure Maintainer.Spv.Reservations.LeaderDutiesEnabled " +
			"is also enabled in the SPV maintainer config for end-to-end operation",
	)

	strandingWatcher := newReservationStrandingWatcher(spvChain)

	// Startup catch-up scan: a wallet closed/terminated while this
	// maintainer was down would otherwise never notify, since the live
	// OnWalletClosed subscription only sees events from this point forward.
	// We scan past wallet registrations bounded by
	// reservationStrandingStartupScanLookBackBlocks and check the ones
	// already Closed/Terminated now; see that constant's doc comment for
	// the accepted limitation this bound carries. Transient per-wallet
	// errors log warnings rather than failing client startup.
	strandingStartupStartBlock := uint64(0)
	if blockCounter, bcErr := spvChain.BlockCounter(); bcErr != nil {
		reservationWiringLogger.Warnf(
			"stranding startup scan failed to get block counter; "+
				"scanning full history: [%v]",
			bcErr,
		)
	} else if currentBlock, cbErr := blockCounter.CurrentBlock(); cbErr != nil {
		reservationWiringLogger.Warnf(
			"stranding startup scan failed to get current block; "+
				"scanning full history: [%v]",
			cbErr,
		)
	} else if currentBlock > reservationStrandingStartupScanLookBackBlocks {
		strandingStartupStartBlock = currentBlock - reservationStrandingStartupScanLookBackBlocks
		reservationWiringLogger.Warnf(
			"stranding startup scan is bounded to wallets registered at "+
				"block [%d] or later; a wallet registered and already "+
				"closed/terminated before that block will not be caught "+
				"by this scan, nor by the live OnWalletClosed "+
				"subscription, which cannot replay an already-emitted "+
				"close event - this is an accepted operational "+
				"limitation, not a gap covered elsewhere",
			strandingStartupStartBlock,
		)
	}

	registeredEvents, err := spvChain.PastNewWalletRegisteredEvents(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: strandingStartupStartBlock},
	)
	if err != nil {
		reservationWiringLogger.Warnf(
			"stranding startup scan failed to fetch wallet registration events: [%v]",
			err,
		)
	}

	// unresolvedWallets collects wallets whose GetWallet call failed on
	// every one of the 3 startup-scan attempts below. They are retried
	// once more, after a short delay, by retryReservationStrandingStartupScan
	// once the rest of the wiring has completed, instead of being assumed
	// Live and silently skipped: a wallet that is actually
	// Closed/Terminated would otherwise have all of its reservations
	// permanently omitted from stranding notification, since the live
	// OnWalletClosed subscription cannot replay an already-emitted close
	// event.
	var unresolvedWallets [][20]byte

	if err == nil {
		for _, event := range registeredEvents {
			var wallet *tbtc.WalletChainData
			var walletErr error
			for attempt := range 3 {
				wallet, walletErr = spvChain.GetWallet(event.WalletPublicKeyHash)
				if walletErr == nil {
					break
				}
				reservationWiringLogger.Warnf(
					"stranding startup scan attempt %d/3 failed to fetch wallet [0x%x]: [%v]",
					attempt+1,
					event.WalletPublicKeyHash,
					walletErr,
				)
			}
			if walletErr != nil {
				reservationWiringLogger.Warnf(
					"stranding startup scan giving up on wallet [0x%x] after "+
						"3 fetch attempts; queuing it for a second-chance "+
						"retry pass once wiring completes instead of "+
						"assuming it is Live: [%v]",
					event.WalletPublicKeyHash,
					walletErr,
				)
				unresolvedWallets = append(unresolvedWallets, event.WalletPublicKeyHash)
				continue
			}
			if wallet.State != tbtc.StateClosed &&
				wallet.State != tbtc.StateTerminated {
				continue
			}
			var checkErr error
			for attempt := range 3 {
				checkErr = strandingWatcher.checkReservationStrandingForWallet(
					event.WalletPublicKeyHash,
				)
				if checkErr == nil {
					break
				}
				reservationWiringLogger.Warnf(
					"stranding startup scan attempt %d/3 failed to check "+
						"wallet [0x%x]: [%v]",
					attempt+1,
					event.WalletPublicKeyHash,
					checkErr,
				)
			}
			if checkErr != nil {
				reservationWiringLogger.Warnf(
					"stranding startup scan giving up on wallet [0x%x] "+
						"after 3 check attempts; its stranded reservations, "+
						"if any, will not be notified by this startup scan: [%v]",
					event.WalletPublicKeyHash,
					checkErr,
				)
				continue
			}
		}
	}

	staleDepositWatcher := NewReservationStaleDepositWatcher(spvChain)

	actionTimeoutWatcher := NewReservationActionTimeoutWatcher(
		spvChain,
		DefaultReservationActionTimeoutPollInterval,
	)

	// Lets checkReservationActionTimeout immediately re-examine a
	// Reanchor-type action's reservation for stranding right after a
	// successful NotifyReservationActionTimeout call restores it to
	// Active under a possibly-already-dead wallet; see
	// recheckStrandingAfterActionTimeout's doc comment.
	actionTimeoutWatcher.SetStrandingWatcher(strandingWatcher)

	subscription := subscribeReservationWalletClosed(ctx, walletClosedChain, spvChain, strandingWatcher)
	go func() {
		<-ctx.Done()
		subscription.Unsubscribe()
	}()

	if len(unresolvedWallets) > 0 {
		go retryReservationStrandingStartupScan(
			ctx,
			spvChain,
			strandingWatcher,
			unresolvedWallets,
			reservationStrandingStartupScanRetryDelay,
		)
	}

	staleDepositState := newStaleDepositPollState()
	staleDepositInitialCount := runStaleDepositPollTick(
		spvChain,
		staleDepositWatcher,
		staleDepositState,
		uint32(time.Now().Unix()),
	)
	startStaleDepositPoll(ctx, spvChain, staleDepositWatcher, staleDepositState)

	if err := actionTimeoutWatcher.pollPendingActions(); err != nil {
		reservationWiringLogger.Errorf(
			"action-timeout watcher initial poll failed: [%v]",
			err,
		)
	}
	go func() {
		if err := actionTimeoutWatcher.Run(ctx); err != nil {
			reservationWiringLogger.Errorf(
				"failed to run reservation action-timeout watcher: [%v]",
				err,
			)
		}
	}()

	if !pairedFlagEnabled &&
		len(registeredEvents) == 0 &&
		staleDepositInitialCount == 0 &&
		len(actionTimeoutWatcher.pendingActions) == 0 {
		return fmt.Errorf(
			"reservation watchers wired but found zero reservation " +
				"activity on-chain (no wallet registrations, no reserved " +
				"deposits, no pending reservation actions) while the " +
				"paired process's LeaderDutiesEnabled flag could not be " +
				"confirmed enabled; verify both " +
				"Tbtc.Reservations.LeaderDutiesEnabled and " +
				"Maintainer.Spv.Reservations.LeaderDutiesEnabled are " +
				"enabled together and that this process is connected to " +
				"the intended network and contract addresses",
		)
	}

	return nil
}

// retryReservationStrandingStartupScan gives the startup catch-up scan's
// unresolved wallets (see WireReservationWatchers) one more chance, after
// waiting out delay, once the rest of the wiring has already completed.
// delay is a parameter (rather than reading reservationStrandingStartupScanRetryDelay
// directly) so tests can exercise this function without waiting on a real
// timer.
func retryReservationStrandingStartupScan(
	ctx context.Context,
	spvChain Chain,
	strandingWatcher *reservationStrandingWatcher,
	walletPublicKeyHashes [][20]byte,
	delay time.Duration,
) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	for _, walletPublicKeyHash := range walletPublicKeyHashes {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wallet, err := spvChain.GetWallet(walletPublicKeyHash)
		if err != nil {
			reservationWiringLogger.Errorf(
				"stranding startup scan second-chance retry still failed "+
					"to fetch wallet [0x%x]; its stranded reservations, "+
					"if any, will not be notified by the startup scan: [%v]",
				walletPublicKeyHash,
				err,
			)
			continue
		}
		if wallet.State != tbtc.StateClosed && wallet.State != tbtc.StateTerminated {
			continue
		}
		if err := strandingWatcher.checkReservationStrandingForWallet(
			walletPublicKeyHash,
		); err != nil {
			reservationWiringLogger.Errorf(
				"stranding startup scan second-chance retry failed to "+
					"check wallet [0x%x]: [%v]",
				walletPublicKeyHash,
				err,
			)
		}
	}
}

// subscribeReservationWalletClosed registers the stranding watcher against
// the chain's wallet close / termination events. Each event dispatches a
// worker goroutine that resolves the closed wallet's ECDSA wallet ID (the
// only identifier WalletClosedEvent carries) to its public key hash and
// runs the watcher's stranding check for that wallet.
func subscribeReservationWalletClosed(
	ctx context.Context,
	walletClosedChain WalletClosedChain,
	spvChain Chain,
	watcher *reservationStrandingWatcher,
) subscription.EventSubscription {
	return walletClosedChain.OnWalletClosed(func(event *tbtc.WalletClosedEvent) {
		go func() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			walletPublicKeyHash, err := resolveWalletPublicKeyHash(
				spvChain,
				event.WalletID,
			)
			if err != nil {
				reservationWiringLogger.Errorf(
					"failed to resolve public key hash for closed "+
						"wallet ID [0x%x]: [%v]",
					event.WalletID,
					err,
				)
				return
			}

			if err := watcher.checkReservationStrandingForWallet(
				walletPublicKeyHash,
			); err != nil {
				reservationWiringLogger.Errorf(
					"failed to check reservation stranding for closed "+
						"wallet [0x%x] (ID [0x%x]): [%v]",
					walletPublicKeyHash,
					event.WalletID,
					err,
				)
			}
		}()
	})
}

// resolveWalletPublicKeyHash maps an ECDSA wallet ID to the wallet's public
// key hash via its NewWalletRegistered event. Every wallet is registered
// exactly once before it can be closed, and the filter is indexed on the
// wallet ID, so this is a targeted lookup rather than a history scan.
func resolveWalletPublicKeyHash(
	spvChain Chain,
	walletID [32]byte,
) ([20]byte, error) {
	events, err := spvChain.PastNewWalletRegisteredEvents(
		&tbtc.NewWalletRegisteredEventFilter{
			EcdsaWalletID: [][32]byte{walletID},
		},
	)
	if err != nil {
		return [20]byte{}, fmt.Errorf(
			"failed to fetch wallet registration event: [%w]",
			err,
		)
	}
	if len(events) == 0 {
		return [20]byte{}, fmt.Errorf(
			"no wallet registration event found for wallet ID [0x%x]",
			walletID,
		)
	}

	// A wallet ID is registered at most once; take the latest match
	// defensively in case of a duplicate log delivery.
	return events[len(events)-1].WalletPublicKeyHash, nil
}

// reservationStaleDepositLookBackBlocks bounds the first stale-deposit poll
// tick's DepositRevealed scan. 30 days at 12s/block, mirroring
// ReservationAcceptanceLookBackBlocks in pkg/tbtcpg. Subsequent ticks scan
// incrementally from the previous tick's block, so this bound only matters
// once, at startup.
const reservationStaleDepositLookBackBlocks = uint64(216000)

// reservationStaleDepositParkedReconcileInterval bounds how often the
// stale-deposit poller re-checks deposits parked because their assigned
// wallet was observed Live (see staleDepositPollState and
// runStaleDepositParkedReconcile). A Live wallet may still transition away
// from Live before anchoring its deposit, so parked deposits are not
// abandoned - they are just re-checked far less often than the
// actively-polled set, since a Live wallet is overwhelmingly expected to
// anchor its own deposit without further intervention.
const reservationStaleDepositParkedReconcileInterval = 30 * time.Minute

// staleDepositPollState is the stale-deposit poller's cross-tick state: the
// incremental block-scan cursor and the two tracked-deposit sets. It is a
// distinct type so a single tick's logic (runStaleDepositPollTick) can run
// synchronously - once at wiring time to seed the misconfiguration
// self-check in WireReservationWatchers, and in tests - without waiting on
// the poller's real ticker.
type staleDepositPollState struct {
	lastSeenBlock uint64
	// pending holds deposits re-checked every tick: newly revealed reserved
	// deposits, and deposits whose assigned wallet is not (or is no longer)
	// Live.
	pending map[string]*big.Int
	// parked holds deposits assigned to a Live wallet. A Live wallet is
	// expected to anchor its own deposit, so these are excluded from the
	// per-tick re-check and are only revisited by the slow reconciliation
	// pass in runStaleDepositParkedReconcile.
	parked map[string]*big.Int
}

// newStaleDepositPollState returns an empty poller state.
func newStaleDepositPollState() *staleDepositPollState {
	return &staleDepositPollState{
		pending: make(map[string]*big.Int),
		parked:  make(map[string]*big.Int),
	}
}

// trackedCount returns the total number of deposits currently tracked,
// across both the actively-polled and parked sets.
func (s *staleDepositPollState) trackedCount() int {
	return len(s.pending) + len(s.parked)
}

// isStaleDepositWalletLive reports whether depositKey's currently assigned
// wallet is in StateLive. It is only called for a deposit whose
// CheckStaleReservedDeposit resolution was Keep (i.e. still reserved), so
// ReservedDepositWallet is expected to resolve to a real assigned wallet.
func isStaleDepositWalletLive(spvChain Chain, depositKey *big.Int) (bool, error) {
	walletPublicKeyHash, err := spvChain.ReservedDepositWallet(depositKey)
	if err != nil {
		return false, fmt.Errorf(
			"failed to resolve assigned wallet: [%w]",
			err,
		)
	}
	wallet, err := spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf(
			"failed to fetch assigned wallet: [%w]",
			err,
		)
	}
	return wallet.State == tbtc.StateLive, nil
}

// startStaleDepositPoll runs the stale-deposit watcher's live source as a
// polling loop over PastDepositRevealedEvents: the Bridge does not expose a
// live subscription for DepositRevealed in m1. Every DefaultReservationStaleDepositPollInterval
// tick runs runStaleDepositPollTick; every reservationStaleDepositParkedReconcileInterval
// it additionally runs runStaleDepositParkedReconcile over the parked set.
// state must already reflect any synchronous tick the caller ran before
// starting this loop (see WireReservationWatchers).
func startStaleDepositPoll(
	ctx context.Context,
	spvChain Chain,
	watcher *ReservationStaleDepositWatcher,
	state *staleDepositPollState,
) {
	go func() {
		ticker := time.NewTicker(DefaultReservationStaleDepositPollInterval)
		defer ticker.Stop()

		lastParkedReconcile := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			now := uint32(time.Now().Unix())
			runStaleDepositPollTick(spvChain, watcher, state, now)

			if time.Since(lastParkedReconcile) >= reservationStaleDepositParkedReconcileInterval {
				lastParkedReconcile = time.Now()
				runStaleDepositParkedReconcile(spvChain, watcher, state, now)
			}
		}
	}()
}

// runStaleDepositPollTick runs one stale-deposit poll pass: it fetches
// deposit-revealed events since state.lastSeenBlock, adds every reserved
// deposit among them to the actively-polled (pending) set, then re-runs
// CheckStaleReservedDeposit for every deposit already in that set. A
// deposit is dropped entirely once it resolves Drop (no longer reserved,
// released to the default sweep path, or swept) or Notified (its
// acceptance action has advanced past pending) - both mean it can never go
// stale again, so re-checking it forever would be wasted RPCs. A deposit
// that instead resolves Keep because its assigned wallet has gone Live is
// moved to the parked set rather than re-checked every tick going forward:
// the wallet may still transition away from Live (e.g.
// MovingFunds/Closing/Terminated) before anchoring, so it cannot be
// abandoned, but re-reading it every minute for the rest of the process
// lifetime would make steady-state cost grow without bound as more
// deposits anchor successfully. Parked deposits are instead revisited by
// runStaleDepositParkedReconcile, far less often.
//
// The poller is intentionally tolerant of chain errors: a transient RPC
// failure logs and continues rather than aborting the wiring. Returns the
// total tracked count (pending + parked) after the tick, for callers that
// want a results signal (see WireReservationWatchers's misconfiguration
// self-check).
func runStaleDepositPollTick(
	spvChain Chain,
	watcher *ReservationStaleDepositWatcher,
	state *staleDepositPollState,
	now uint32,
) int {
	blockCounter, err := spvChain.BlockCounter()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to get block counter: [%v]",
			err,
		)
		return state.trackedCount()
	}
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to get current block: [%v]",
			err,
		)
		return state.trackedCount()
	}

	startBlock := state.lastSeenBlock
	if startBlock == 0 && currentBlock > reservationStaleDepositLookBackBlocks {
		startBlock = currentBlock - reservationStaleDepositLookBackBlocks
	}

	events, err := spvChain.PastDepositRevealedEvents(
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
		return state.trackedCount()
	}

	params, err := spvChain.ReservationParameters()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to fetch reservation "+
				"parameters: [%v]",
			err,
		)
		return state.trackedCount()
	}

	for _, event := range events {
		if event.Vault == nil || !strings.EqualFold(string(*event.Vault), string(params.ReservationVault)) {
			continue
		}

		depositKey := spvChain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		isReserved, err := spvChain.IsReservedDeposit(depositKey)
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
			// re-check on every tick (see
			// reservation_stale_deposit_watch.go) and resolves to
			// Drop if the deposit genuinely isn't reserved, so
			// tracking it speculatively here is safe.
			state.pending[depositKey.String()] = depositKey
			continue
		}
		if !isReserved {
			continue
		}

		state.pending[depositKey.String()] = depositKey
	}

	state.lastSeenBlock = currentBlock

	for key, depositKey := range state.pending {
		resolution, err := watcher.CheckStaleReservedDeposit(
			depositKey,
			now,
		)
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
			delete(state.pending, key)
			watcher.forgetDeposit(depositKey)
		case StaleDepositResolutionKeep:
			live, err := isStaleDepositWalletLive(spvChain, depositKey)
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
				delete(state.pending, key)
				state.parked[key] = depositKey
			}
		}
	}

	return state.trackedCount()
}

// runStaleDepositParkedReconcile re-checks every parked deposit (assigned
// to a Live wallet at last check). A deposit resolving Drop or Notified is
// evicted entirely; one resolving Keep whose assigned wallet is no longer
// Live is reactivated into the actively-polled (pending) set so it starts
// being re-checked every tick again.
func runStaleDepositParkedReconcile(
	spvChain Chain,
	watcher *ReservationStaleDepositWatcher,
	state *staleDepositPollState,
	now uint32,
) {
	for key, depositKey := range state.parked {
		resolution, err := watcher.CheckStaleReservedDeposit(depositKey, now)
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
			delete(state.parked, key)
			watcher.forgetDeposit(depositKey)
		case StaleDepositResolutionKeep:
			live, err := isStaleDepositWalletLive(spvChain, depositKey)
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
				delete(state.parked, key)
				state.pending[key] = depositKey
			}
		}
	}
}

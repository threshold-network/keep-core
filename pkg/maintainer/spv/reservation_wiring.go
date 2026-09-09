package spv

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/ipfs/go-log/v2"
)

var reservationWiringLogger = log.Logger("keep-maintainer-spv-reservations")

// DefaultReservationStaleDepositPollInterval is the default poll interval used
// by the stale-deposit watcher fallback loop. The Bridge does not expose a
// live subscription for DepositRevealed in m1, so the wiring layer falls back
// to PastDepositRevealedEvents on a coarse interval and dispatches each new
// reveal to the watcher.
//
// This value is chosen and tuned independently of
// DefaultReservationActionTimeoutPollInterval below: the two watchers have
// different load profiles (this one scans a user-driven, rate-limited event
// stream; the action-timeout watcher re-reads a state-driven, potentially
// bursty tracked-action set) and sharing one process-wide cadence is a
// coincidence of today's values, not an invariant either watcher depends on.
// Changing one does not require changing the other.
const DefaultReservationStaleDepositPollInterval = 1 * time.Minute

// DefaultReservationActionTimeoutPollInterval is the default, fixed poll
// interval for the action-timeout watcher's Run loop - the background
// loop WireReservationWatchers starts and the only way the watcher is
// driven in production. It is intentionally conservative (1 minute) to
// limit the Bridge load from the per-tracked-action GetReservationAction
// reads Run issues on every tick. Tuned independently of
// DefaultReservationStaleDepositPollInterval above; see that constant's
// doc comment for why the two are not coupled.
const DefaultReservationActionTimeoutPollInterval = 1 * time.Minute

// reservationDefaultLookBackBlocks bounds every reservation watcher's
// startup/first-pass catch-up scan window: 30 days at 12s/block. It is
// the single source of truth for this bound, replacing what were
// previously 4+ independently-defined constants (this file's own
// reservationStrandingStartupScanLookBackBlocks and
// reservationStaleDepositLookBackBlocks,
// reservation_action_timeout_watch.go's
// reservationActionTimeoutLookBackBlocks, and
// reservation_proof_loop.go's reservationProofLookBackBlocks) all
// independently set to the identical literal value with near-duplicate
// doc comments. reservationProofLookBackBlocks and
// reservation_stale_deposit_watch.go's staleDepositRevealScanLookBackBlocks
// remain as thin aliases to this constant: their exact names are
// referenced directly by test files outside this change's scope, so
// they could not simply be deleted; every other former duplicate now
// references this constant directly.
const reservationDefaultLookBackBlocks = uint64(216000)

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

	// Signing exposes the operator's own chain signing identity. It lets
	// WireReservationWatchers derive a deterministic per-operator delay
	// offset (see reservationOperatorStaggerOffset) so that many
	// reservation-enabled operators independently discovering the same
	// overdue reservation/action/deposit do not all submit their first
	// permissionless notify attempt in the same poll tick and collide
	// on-chain - only one transaction per round can succeed; every
	// other operator's simultaneous attempt reverts and burns gas for
	// no benefit.
	Signing() chain.Signing
}

// reservationOperatorStaggerOffset derives a deterministic, per-operator
// delay offset for uniqueKey, reduced modulo interval. It exists so that
// when many reservation-enabled operators independently discover the
// same overdue, not-yet-notified reservation/action/deposit at
// essentially the same time, their FIRST permissionless notify attempts
// do not all land in the same poll tick and collide on-chain - only one
// operator's transaction can succeed per round; every other operator's
// simultaneous attempt reverts and burns gas for no benefit. The offset
// is derived from keccak256(operatorAddress || uniqueKey), so it is
// stable across ticks and process restarts for the same operator/key
// pair and is spread deterministically across [0, interval) over the
// operator set. Callers apply it only to a generation's/deposit's FIRST
// notify attempt; retries continue to use whatever renotify-interval
// backoff the caller already has (see actionTimeoutRenotifyInterval and
// its reuse in reservation_stale_deposit_watch.go's
// CheckStaleReservedDeposit). A non-positive interval disables
// staggering (returns zero).
func reservationOperatorStaggerOffset(
	operatorAddress common.Address,
	uniqueKey string,
	interval time.Duration,
) uint32 {
	intervalSeconds := uint64(interval / time.Second)
	if intervalSeconds == 0 {
		return 0
	}

	hash := crypto.Keccak256(append(operatorAddress.Bytes(), []byte(uniqueKey)...))
	offset := new(big.Int).Mod(
		new(big.Int).SetBytes(hash),
		new(big.Int).SetUint64(intervalSeconds),
	)

	return uint32(offset.Uint64())
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
// that the counterpart process's own reservation-enabling flag is also
// enabled. Pass true when the caller has no reliable visibility into that
// flag (e.g. spv.Initialize, which has no access to the client's
// Tbtc.ReservationsEnabled flag) to skip the misconfiguration self-check below;
// pass the caller's best-effort read otherwise (e.g. cmd/start.go, which
// can read Maintainer.Spv.ReservationProofsEnabled even though the
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

	// operatorAddress identifies this process for
	// reservationOperatorStaggerOffset (see WalletClosedChain.Signing's
	// doc). walletClosedChain (not spvChain) supplies it because every
	// production Chain implementation that satisfies WalletClosedChain
	// is the same handle already used to submit transactions, so its
	// own Signing().Address() is the operator's own address.
	operatorAddress := common.HexToAddress(walletClosedChain.Signing().Address().String())

	reservationWiringLogger.Infof(
		"wiring reservation watchers; ensure Maintainer.Spv.ReservationProofsEnabled " +
			"is also enabled in the SPV maintainer config for end-to-end operation",
	)

	strandingWatcher := newReservationStrandingWatcher(spvChain)

	// Startup catch-up scan: a wallet closed/terminated while this
	// maintainer was down would otherwise never notify, since the live
	// OnWalletClosed subscription only sees events from this point forward.
	// We scan past wallet registrations bounded by
	// reservationDefaultLookBackBlocks and check the ones already
	// Closed/Terminated now. This is an accepted operational
	// limitation, not a gap covered elsewhere: a wallet that closed or
	// was terminated more than this bound before the process started
	// has no path to stranding notification (see the warning logged
	// below when the bound actually truncates the scan window).
	// Transient per-wallet errors log warnings rather than failing
	// client startup.
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
	} else if currentBlock > reservationDefaultLookBackBlocks {
		strandingStartupStartBlock = currentBlock - reservationDefaultLookBackBlocks
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
	staleDepositWatcher.SetOperatorAddress(operatorAddress)

	actionTimeoutWatcher := NewReservationActionTimeoutWatcher(
		spvChain,
		DefaultReservationActionTimeoutPollInterval,
	)
	actionTimeoutWatcher.SetOperatorAddress(operatorAddress)

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

	staleDepositInitialCount := staleDepositWatcher.pollTick(uint32(time.Now().Unix()))
	go func() {
		defer func() {
			if r := recover(); r != nil {
				reservationWiringLogger.Errorf(
					"reservation stale-deposit watcher crashed and is "+
						"no longer running (recovered panic: [%v]); "+
						"stale reserved deposit notifications are silently "+
						"unmonitored until process restart",
					r,
				)
			}
		}()
		if err := staleDepositWatcher.Run(ctx, DefaultReservationStaleDepositPollInterval); err != nil {
			// Run only returns non-nil on the interval misconfiguration
			// guard at loop start (per-tick errors are logged and the loop
			// continues), so reaching this branch means the watcher is
			// not running at all. Log at distinct severity rather than
			// Fatalf-ing: WireReservationWatchers is called from two
			// callers (cmd/start.go's client process AND spv.go's maintainer
			// process), and the caller decides process-level fatality, not
			// this shared helper.
			reservationWiringLogger.Errorf(
				"reservation stale-deposit watcher is not running: [%v]; "+
					"stale reserved deposit notifications are silently "+
					"unmonitored until process restart",
				err,
			)
		}
	}()

	if err := actionTimeoutWatcher.pollPendingActions(); err != nil {
		reservationWiringLogger.Errorf(
			"action-timeout watcher initial poll failed: [%v]",
			err,
		)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				reservationWiringLogger.Errorf(
					"reservation action-timeout watcher crashed and is "+
						"no longer running (recovered panic: [%v]); "+
						"reservation action timeouts are silently "+
						"unmonitored until process restart",
					r,
				)
			}
		}()
		if err := actionTimeoutWatcher.Run(ctx); err != nil {
			// See the identical rationale on the stale-deposit watcher
			// launch above.
			reservationWiringLogger.Errorf(
				"reservation action-timeout watcher is not running: [%v]; "+
					"reservation action timeouts are silently unmonitored "+
					"until process restart",
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
				"paired process's reservation-enabling flag could not be " +
				"confirmed enabled; verify both " +
				"Tbtc.ReservationsEnabled and " +
				"Maintainer.Spv.ReservationProofsEnabled are " +
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

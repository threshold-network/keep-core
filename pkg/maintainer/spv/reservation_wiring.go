package spv

import (
	"context"
	"fmt"
	"math/big"
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
// 12s/block, mirroring the convention used across this package. Wallets
// registered further back than this bound are not covered by the startup
// scan; the live OnWalletClosed subscription plus the existing per-wallet
// close re-check are relied on to eventually catch them.
const reservationStrandingStartupScanLookBackBlocks = uint64(216000)

// WalletClosedChain defines the chain interface required to subscribe to
// wallet close events.
type WalletClosedChain interface {
	OnWalletClosed(
		handler func(event *tbtc.WalletClosedEvent),
	) subscription.EventSubscription
}

// WireReservationWatchers is the integration entry point that cmd/start.go
// calls directly when config.Reservations.Enabled is true. It constructs
// the three reservation watchers (stranding, stale-deposit, action-timeout),
// wires their Bridge-facing notifiers to the chain, and subscribes/starts
// each watcher against its source.
//
// `walletClosedChain` supplies the OnWalletClosed event subscription;
// `spvChain` supplies the reservation data reads, event queries, and
// Notify* writes. `ctx` controls the goroutine lifetimes started by the
// wiring function.
func WireReservationWatchers(
	ctx context.Context,
	walletClosedChain WalletClosedChain,
	spvChain Chain,
	walletMembersResolver tbtc.WalletMembersResolver,
) error {
	if walletClosedChain == nil {
		return fmt.Errorf("wallet closed chain must not be nil")
	}
	if spvChain == nil {
		return fmt.Errorf("spv chain must not be nil")
	}
	if walletMembersResolver == nil {
		return fmt.Errorf("wallet members resolver must not be nil")
	}

	reservationWiringLogger.Infof(
		"wiring reservation watchers; ensure Maintainer.Spv.Reservations.Enabled " +
			"is also enabled in the SPV maintainer config for end-to-end operation",
	)

	strandingWatcher := newReservationStrandingWatcher(spvChain)

	// Startup catch-up scan: a wallet closed/terminated while this
	// maintainer was down would otherwise never notify, since the live
	// OnWalletClosed subscription only sees events from this point forward.
	// We scan past wallet registrations bounded by
	// reservationStrandingStartupScanLookBackBlocks and check the ones
	// already Closed/Terminated now; older wallets are left to the live
	// subscription plus the existing per-wallet close re-check. Transient
	// per-wallet errors log warnings rather than failing client startup.
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
	}

	registeredEvents, err := spvChain.PastNewWalletRegisteredEvents(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: strandingStartupStartBlock},
	)
	if err != nil {
		reservationWiringLogger.Warnf(
			"stranding startup scan failed to fetch wallet registration events: [%v]",
			err,
		)
	} else {
		for _, event := range registeredEvents {
			wallet, err := spvChain.GetWallet(event.WalletPublicKeyHash)
			if err != nil {
				reservationWiringLogger.Warnf(
					"stranding startup scan failed to fetch wallet [0x%x]: [%v]",
					event.WalletPublicKeyHash,
					err,
				)
				continue
			}
			if wallet.State != tbtc.StateClosed &&
				wallet.State != tbtc.StateTerminated {
				continue
			}
			if err := strandingWatcher.checkReservationStrandingForWallet(
				event.WalletPublicKeyHash,
			); err != nil {
				reservationWiringLogger.Warnf(
					"stranding startup scan failed to check wallet [0x%x]: [%v]",
					event.WalletPublicKeyHash,
					err,
				)
				continue
			}
		}
	}

	staleDepositWatcher := NewReservationStaleDepositWatcher(spvChain)

	actionTimeoutWatcher := NewReservationActionTimeoutWatcher(
		spvChain,
		walletMembersResolver,
		DefaultReservationActionTimeoutPollInterval,
	)

	subscription := subscribeReservationWalletClosed(ctx, walletClosedChain, spvChain, strandingWatcher)
	go func() {
		<-ctx.Done()
		subscription.Unsubscribe()
	}()
	startStaleDepositPoll(ctx, spvChain, staleDepositWatcher)
	go func() {
		if err := actionTimeoutWatcher.Run(ctx); err != nil {
			reservationWiringLogger.Errorf(
				"failed to run reservation action-timeout watcher: [%v]",
				err,
			)
		}
	}()

	return nil
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

// startStaleDepositPoll runs the stale-deposit watcher's live source as a
// polling loop over PastDepositRevealedEvents: the Bridge does not expose a
// live subscription for DepositRevealed in m1. Each tick fetches reveals
// since the previously scanned block, adds every reserved deposit among
// them to a tracked pending set, then re-runs CheckStaleReservedDeposit for
// every deposit already in the set. A deposit is dropped from the set once
// it is no longer reserved (released to the default sweep path, or swept)
// or its acceptance action has advanced past pending - both mean it can
// never go stale again, so re-checking it forever would be wasted RPCs. A
// deposit whose wallet has gone Live is kept in the set instead of
// dropped: the wallet may still transition away from Live (e.g.
// MovingFunds/Closing/Terminated) before anchoring, and the scan cursor
// only ever advances, so a dropped deposit could never re-enter tracking.
//
// The poller is intentionally tolerant of chain errors: a transient RPC
// failure logs and continues rather than aborting the wiring.
func startStaleDepositPoll(
	ctx context.Context,
	spvChain Chain,
	watcher *ReservationStaleDepositWatcher,
) {
	go func() {
		ticker := time.NewTicker(DefaultReservationStaleDepositPollInterval)
		defer ticker.Stop()

		var lastSeenBlock uint64
		pending := make(map[string]*big.Int)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			blockCounter, err := spvChain.BlockCounter()
			if err != nil {
				reservationWiringLogger.Errorf(
					"stale-deposit poll failed to get block counter: [%v]",
					err,
				)
				continue
			}
			currentBlock, err := blockCounter.CurrentBlock()
			if err != nil {
				reservationWiringLogger.Errorf(
					"stale-deposit poll failed to get current block: [%v]",
					err,
				)
				continue
			}

			startBlock := lastSeenBlock
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
				continue
			}

			params, err := spvChain.ReservationParameters()
			if err != nil {
				reservationWiringLogger.Errorf(
					"stale-deposit poll failed to fetch reservation "+
						"parameters: [%v]",
					err,
				)
				continue
			}

			for _, event := range events {
				if event.Vault == nil || *event.Vault != params.ReservationVault {
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
					pending[depositKey.String()] = depositKey
					continue
				}
				if !isReserved {
					continue
				}

				pending[depositKey.String()] = depositKey
			}

			lastSeenBlock = currentBlock
			now := uint32(time.Now().Unix())

			for key, depositKey := range pending {
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

				if resolution == StaleDepositResolutionDrop ||
					resolution == StaleDepositResolutionNotified {
					delete(pending, key)
					watcher.forgetDeposit(depositKey)
				}
			}
		}
	}()
}

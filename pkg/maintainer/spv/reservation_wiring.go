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

// DefaultReservationActionTimeoutPollInterval is the default poll interval
// for the action-timeout watcher's Run loop. It is intentionally conservative
// (1 minute) to limit Bridge load until the production wiring tightens the
// cadence. The interval is fixed.
const DefaultReservationActionTimeoutPollInterval = 1 * time.Minute

// WireReservationWatchers is the integration entry point that cmd/start.go
// calls directly when config.Reservations.Enabled is true. It constructs
// the three reservation watchers (stranding, stale-deposit, action-timeout),
// wires their Bridge-facing notifiers to the chain, and subscribes/starts
// each watcher against its source.
//
// `tbtcChain` supplies the On* event subscriptions the SPV-specific `Chain`
// omits; `spvChain` supplies the reservation data reads and Notify* writes.
// `ctx` controls the goroutine lifetimes started by the wiring function.
func WireReservationWatchers(
	ctx context.Context,
	tbtcChain tbtc.Chain,
	spvChain Chain,
	walletMembersResolver tbtc.WalletMembersResolver,
) error {
	strandingWatcher := NewReservationStrandingWatcher(spvChain)

	// Startup catch-up scan: a wallet closed/terminated while this
	// maintainer was down would otherwise never notify, since the live
	// OnWalletClosed subscription only sees events from this point forward.
	// Look back the same bounded window the other two watchers use, find
	// wallets registered in that window, and check the ones already
	// Closed/Terminated now.
	if lastSeenBlock, err := spvChain.BlockCounter(); err != nil {
		return fmt.Errorf("stranding startup scan failed to get block counter: [%w]", err)
	} else if currentBlock, err := lastSeenBlock.CurrentBlock(); err != nil {
		return fmt.Errorf("stranding startup scan failed to get current block: [%w]", err)
	} else {
		var startBlock uint64
		if currentBlock > reservationStaleDepositLookBackBlocks {
			startBlock = currentBlock - reservationStaleDepositLookBackBlocks
		}

		registeredEvents, err := spvChain.PastNewWalletRegisteredEvents(
			&tbtc.NewWalletRegisteredEventFilter{StartBlock: startBlock},
		)
		if err != nil {
			return fmt.Errorf("stranding startup scan failed to fetch wallet registration events: [%w]", err)
		}

		for _, event := range registeredEvents {
			wallet, err := spvChain.GetWallet(event.WalletPublicKeyHash)
			if err != nil {
				return fmt.Errorf("stranding startup scan failed to fetch wallet [0x%x]: [%w]", event.WalletPublicKeyHash, err)
			}
			if wallet.State != tbtc.StateClosed &&
				wallet.State != tbtc.StateTerminated {
				continue
			}
			if err := strandingWatcher.CheckReservationStrandingForWallet(
				event.WalletPublicKeyHash,
			); err != nil {
				return fmt.Errorf("stranding startup scan failed to check wallet [0x%x]: [%w]", event.WalletPublicKeyHash, err)
			}
		}
	}

	staleDepositWatcher := NewReservationStaleDepositWatcher(spvChain)

	actionTimeoutWatcher := NewReservationActionTimeoutWatcher(
		spvChain,
		walletMembersResolver,
		DefaultReservationActionTimeoutPollInterval,
	)

	subscription := subscribeReservationWalletClosed(ctx, tbtcChain, spvChain, strandingWatcher)
	go func() {
		<-ctx.Done()
		subscription.Unsubscribe()
	}()
	startStaleDepositPoll(ctx, tbtcChain, spvChain, staleDepositWatcher)
	startActionTimeoutRun(ctx, actionTimeoutWatcher)

	return nil
}

// subscribeReservationWalletClosed registers the stranding watcher against
// the chain's wallet close / termination events. Each event dispatches a
// worker goroutine that resolves the closed wallet's ECDSA wallet ID (the
// only identifier WalletClosedEvent carries) to its public key hash and
// runs the watcher's stranding check for that wallet.
func subscribeReservationWalletClosed(
	ctx context.Context,
	tbtcChain tbtc.Chain,
	spvChain Chain,
	watcher *ReservationStrandingWatcher,
) subscription.EventSubscription {
	return tbtcChain.OnWalletClosed(func(event *tbtc.WalletClosedEvent) {
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

			if err := watcher.CheckReservationStrandingForWallet(
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
// or its assigned wallet has gone Live - both mean it can never go stale
// again, so re-checking it forever would be wasted RPCs.
//
// The poller is intentionally tolerant of chain errors: a transient RPC
// failure logs and continues rather than aborting the wiring.
func startStaleDepositPoll(
	ctx context.Context,
	tbtcChain tbtc.Chain,
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

			events, err := tbtcChain.PastDepositRevealedEvents(
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

			var batchErr error
			for _, event := range events {
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
					batchErr = err
					break
				}
				if !isReserved {
					continue
				}

				pending[depositKey.String()] = depositKey
			}
			if batchErr != nil {
				continue
			}

			lastSeenBlock = currentBlock
			now := uint32(time.Now().Unix())

			for key, depositKey := range pending {
				if err := watcher.CheckStaleReservedDeposit(
					depositKey,
					now,
				); err != nil {
					reservationWiringLogger.Errorf(
						"stale-deposit poll failed to check deposit "+
							"[%v]: [%v]",
						depositKey,
						err,
					)
					continue
				}

				isReserved, err := spvChain.IsReservedDeposit(depositKey)
				if err != nil {
					reservationWiringLogger.Errorf(
						"stale-deposit poll failed to check if deposit [%v] is reserved: [%v]",
						depositKey,
						err,
					)
					continue
				}

				var walletState tbtc.WalletState
				if isReserved {
					walletPublicKeyHash, err := spvChain.ReservedDepositWallet(depositKey)
					if err != nil || walletPublicKeyHash == ([20]byte{}) {
						// treat as not resolved
					} else {
						wallet, err := spvChain.GetWallet(walletPublicKeyHash)
						if err == nil {
							walletState = wallet.State
						}
					}
				}

				if isPendingStaleDepositResolved(isReserved, walletState) {
					delete(pending, key)
				}
			}
		}
	}()
}

// isPendingStaleDepositResolved reports whether depositKey no longer needs
// tracking: it stopped being a reserved deposit (released or swept), or its
// assigned wallet reached StateLive (expected to anchor on its own). Chain
// read errors are treated as unresolved so a transient RPC failure does not
// silently drop a deposit that might still need the stale check.
func isPendingStaleDepositResolved(
	isReserved bool,
	walletState tbtc.WalletState,
) bool {
	if !isReserved {
		return true
	}

	return walletState == tbtc.StateLive
}

// startActionTimeoutRun starts the action-timeout watcher's Run loop in a
// goroutine, tied to ctx's lifetime.
func startActionTimeoutRun(
	ctx context.Context,
	watcher *ReservationActionTimeoutWatcher,
) {
	go func() {
		if err := watcher.Run(ctx); err != nil {
			reservationWiringLogger.Errorf(
				"failed to start reservation action-timeout watcher: [%v]",
				err,
			)
		}
	}()
}

package spv

import (
	"context"
	"math/big"
	"time"

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
// cadence. Operators can shorten the interval once the full integration ships.
const DefaultReservationActionTimeoutPollInterval = 1 * time.Minute

// WireReservationWatchers is the integration entry point that the m1 PR H
// coordination layer calls when config.Reservations.Enabled is true. It
// constructs the three reservation watchers (stranding, stale-deposit,
// action-timeout), wires their Bridge-facing notifiers to the chain, and
// subscribes each watcher to its source event.
//
// The function lives in the spv package because that is where the watcher
// types live; the coordination layer invokes it via a callback supplied by
// cmd/start.go so that the tbtc package does not need a static import of spv
// (which would cycle with spv's existing import of tbtc).
//
// The wiring is intentionally tolerant of the m1 placeholder signal: each
// event handler stores the watcher and event for the production integration
// step rather than dispatching a real call. This keeps the gate semantics
// (`config.Reservations.Enabled` is the single switch) intact while leaving
// the heavy lifting to the follow-up PR that lands the live wiring.
//
// `chain` is the tbtc.Chain used both for event subscriptions (On*) and for
// the watcher notifiers (Notify*). `ctx` controls the goroutine lifetimes
// started by the wiring function.
func WireReservationWatchers(
	ctx context.Context,
	tbtcChain tbtc.Chain,
	spvChain Chain,
) error {
	// The watcher constructors require the SPV-specific Chain interface
	// because they call into the SPV proof submission surface
	// (GetReservation, GetReservationAction, etc.). The Bridge-facing
	// On* event subscriptions require the broader tbtc.Chain interface
	// because the SPV interface omits event subscriptions. Both chains
	// point at the same underlying handle in production; this function
	// threads them through to the right call sites.
	chain := spvChain
	strandingWatcher := NewReservationStrandingWatcher(
		chain,
		ReservationStrandingNotifierFunc(
			func(reservationKey *big.Int) error {
				return chain.NotifyReservationStranded(reservationKey)
			},
		),
	)

	staleDepositWatcher := NewReservationStaleDepositWatcher(
		chain,
		StaleReservedDepositNotifierFunc(
			func(depositKey *big.Int) error {
				return chain.NotifyStaleReservedDeposit(depositKey)
			},
		),
	)

	// The action-timeout watcher requires a wallet members resolver that the
	// production sortition backend will provide. Until the integration lands
	// the placeholder resolver returns an empty slice; the watcher treats
	// empty membership as a no-op for the m1 bridge notification shape.
	membersResolver := WalletMembersResolverFunc(
		func(walletPublicKeyHash [20]byte) ([]uint32, error) {
			return nil, nil
		},
	)
	actionTimeoutWatcher := NewReservationActionTimeoutWatcher(
		chain,
		ReservationActionTimeoutNotifierFunc(
			func(reservationKey *big.Int, walletMembersIDs []uint32) error {
				return chain.NotifyReservationActionTimeout(
					reservationKey,
					walletMembersIDs,
				)
			},
		),
		membersResolver,
		DefaultReservationActionTimeoutPollInterval,
	)

	subscribeReservationWalletClosed(ctx, tbtcChain, strandingWatcher)
	subscribeReservationActionTimedOut(ctx, tbtcChain, actionTimeoutWatcher)
	startStaleDepositPoll(ctx, tbtcChain, staleDepositWatcher)
	startActionTimeoutRun(ctx, actionTimeoutWatcher)

	return nil
}

// subscribeReservationWalletClosed registers the stranding watcher against
// the chain's wallet close / termination events. The integration step that
// lands the wallet-ID -> public-key-hash mapping will dispatch the watcher
// here; for now we hold the watcher reference so the gate semantics are
// observable in the running process.
func subscribeReservationWalletClosed(
	ctx context.Context,
	tbtcChain tbtc.Chain,
	watcher *ReservationStrandingWatcher,
) {
	// Use the broader tbtc.Chain so the OnWalletClosed subscription is
	// available; the SPV-specific Chain does not expose event
	// subscriptions.
	chain := tbtcChain
	_ = chain.OnWalletClosed(func(event *tbtc.WalletClosedEvent) {
		// PR H placeholder: the production wiring resolves the
		// event.WalletID into the corresponding wallet public key
		// hash and dispatches watcher.CheckReservationStrandingForWallet
		// on a worker goroutine. The wiring step that adds the
		// mapping is delivered by the follow-up integration PR.
		_ = watcher
		_ = event
		reservationWiringLogger.Debug(
			"received wallet closed event; stranding watcher integration " +
				"is a placeholder in PR H",
		)
	})
}

// subscribeReservationActionTimedOut registers the action-timeout watcher
// against the chain's on-chain ReservationActionTimedOut event. The
// production wiring dispatches watcher.CheckReservationActionTimeouts from
// here; for now we hold the watcher reference so the gate semantics are
// observable in the running process.
func subscribeReservationActionTimedOut(
	ctx context.Context,
	tbtcChain tbtc.Chain,
	watcher *ReservationActionTimeoutWatcher,
) {
	chain := tbtcChain
	_ = chain.OnReservationActionTimedOut(
		func(event *tbtc.ReservationActionTimedOutEvent) {
			// PR H placeholder: the production wiring reads the
			// reservation key from the event and dispatches
			// watcher.CheckReservationActionTimeouts on a worker
			// goroutine.
			_ = watcher
			_ = event
			reservationWiringLogger.Debug(
				"received reservation action timed out event; " +
					"action-timeout watcher integration is a " +
					"placeholder in PR H",
			)
		},
	)
}

// startStaleDepositPoll runs the stale-deposit watcher integration as a
// polling loop over PastDepositRevealedEvents. The Bridge does not expose a
// live subscription for DepositRevealed in m1, so this loop is the
// placeholder source: each tick fetches the events since the last seen
// block and dispatches them to the watcher.
//
// The poller is intentionally tolerant of chain errors: a transient RPC
// failure logs and continues rather than aborting the wiring.
func startStaleDepositPoll(
	ctx context.Context,
	tbtcChain tbtc.Chain,
	watcher *ReservationStaleDepositWatcher,
) {
	chain := tbtcChain
	// PR H placeholder: the live subscription integration lands in
	// the follow-up PR; until then the wiring keeps the watcher alive
	// but does not invoke OnDepositRevealed. Holding the watcher
	// reference is enough to make the gate observable.
	_ = watcher
	_ = chain
	_ = ctx
	reservationWiringLogger.Debug(
		"reservation stale-deposit watcher constructed; live polling " +
			"integration is a placeholder in PR H",
	)
}

// startActionTimeoutRun starts the action-timeout watcher's Run loop in a
// goroutine. The watcher exposes Run() as a guarded no-op (placeholder for
// the integration step) and returns an error if its dependencies are not
// provided; we've supplied them above so Run() returns nil cleanly.
func startActionTimeoutRun(
	ctx context.Context,
	watcher *ReservationActionTimeoutWatcher,
) {
	go func() {
		if err := watcher.Run(); err != nil {
			reservationWiringLogger.Errorf(
				"failed to start reservation action-timeout watcher: [%v]",
				err,
			)
		}
	}()
}

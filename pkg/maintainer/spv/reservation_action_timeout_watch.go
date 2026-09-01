package spv

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ReservationActionTimeoutWatcher observes the reservation action set and
// notifies the Bridge when a pending action's on-chain deadline has elapsed
// without an SPV proof being submitted.
//
// The Bridge uses a per-action timeout window (snaphotted in the
// ReservationAction record at nonce creation time) to bound the lag between
// action creation and SPV proof submission. When the deadline passes without
// a proof, the SPV maintainer is no longer eligible to settle the action
// and the Bridge must be told to update the action to ReservationActionStateTimedOut.
// This triggers Bridge-side sweeping (e.g. fee slashing in m2+ and the
// fallback to owner late-settlement) and ensures the state machine can move
// forward.
//
// In m1 the operator-side penalty is a no-op; NotifyReservationActionTimeout
// is still called with the wallet member IDs as required by the Bridge
// function signature so that m2+ integrations only need to add the slashing
// logic without changing the call shape.
type ReservationActionTimeoutWatcher struct {
	spvChain Chain
	// nowFn returns the current UNIX timestamp the watcher treats as "now"
	// for `now > timeoutAt` comparisons. Tests override it to drive the
	// deadline forward; production wires it to time.Now in UTC.
	nowFn func() uint32
	// interval is how often the background poll loop re-checks pending
	// actions. A zero value disables the background loop; tests and the
	// synchronously driven integration code path will use a positive
	// duration.
	interval time.Duration
	// membersResolver turns a wallet public key hash into the operator IDs
	// the Bridge expects for the slashing argument. The resolver is
	// injected to keep the watcher independent of the chain interface used
	// to look up operator addresses (the SPV maintainer chain interface
	// does not expose GetOperatorID today).
	membersResolver WalletMembersResolver
	// lastWalletScanBlock is the block number up to which Run has already
	// scanned NewWalletRegistered events, so each poll iteration only fetches
	// wallets registered since the previous tick instead of rescanning the
	// full history every time. Owned exclusively by Run's single goroutine;
	// CheckReservationActionTimeouts (the synchronous, test/integration
	// entry point) never touches it.
	lastWalletScanBlock uint64
	// knownWallets tracks the full history of registered wallets so that
	// every action timeout check iterates the entire set of wallets ever
	// discovered, not just those registered in the most recent poll interval.
	knownWallets map[[20]byte]struct{}
}

// WalletMembersResolver maps a wallet public key hash to the operator IDs
// the wallet's signing group is composed of. The Bridge uses the IDs to
// attribute slashing; m2+ will layer the actual penalty computation on top
// of the IDs carried in NotifyReservationActionTimeout.
//
// In production the resolver must look up the wallet's signing group via
// the maintenance bridge / sortition pool and translate each operator
// address to an operator ID via chain.GetOperatorID. The watcher does not
// prescribe a particular lookup because the production wiring depends on
// the sortition backend chosen for the deployment.
type WalletMembersResolver interface {
	ResolveWalletMembers(walletPublicKeyHash [20]byte) ([]uint32, error)
}

// WalletMembersResolverFunc adapts a plain function to the
// WalletMembersResolver interface.
type WalletMembersResolverFunc func(walletPublicKeyHash [20]byte) ([]uint32, error)

// ResolveWalletMembers forwards the call to the wrapped function.
func (f WalletMembersResolverFunc) ResolveWalletMembers(
	walletPublicKeyHash [20]byte,
) ([]uint32, error) {
	return f(walletPublicKeyHash)
}

// ReservationActionTimeoutNotifier is the Bridge-facing contract for the
// action-timeout watcher. It mirrors
// `Chain.NotifyReservationActionTimeout` but is interface-typed to enable
// in-memory recorders during tests.
type ReservationActionTimeoutNotifier interface {
	NotifyReservationActionTimeout(
		reservationKey *big.Int,
		walletMembersIDs []uint32,
	) error
}

// ReservationActionTimeoutNotifierFunc adapts a function to the
// ReservationActionTimeoutNotifier interface.
type ReservationActionTimeoutNotifierFunc func(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error

// NotifyReservationActionTimeout forwards the call to the wrapped function.
func (f ReservationActionTimeoutNotifierFunc) NotifyReservationActionTimeout(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error {
	return f(reservationKey, walletMembersIDs)
}

// NewReservationActionTimeoutWatcher constructs a watcher bound to the
// given chain, notifier, members resolver, and poll interval.
//
// The members resolver is mandatory: the watcher will refuse to operate
// without it because emitting NotifyReservationActionTimeout with a nil
// or empty member slice would be ill-formed on the Bridge side.
//
// A zero pollInterval disables the background loop; the watcher must then
// be driven by CheckReservationActionTimeouts calls from the integration.
func NewReservationActionTimeoutWatcher(
	spvChain Chain,
	membersResolver WalletMembersResolver,
	pollInterval time.Duration,
) *ReservationActionTimeoutWatcher {
	return &ReservationActionTimeoutWatcher{
		spvChain:        spvChain,
		nowFn:           defaultActionTimeoutNowFn,
		interval:        pollInterval,
		membersResolver: membersResolver,
		knownWallets:    make(map[[20]byte]struct{}),
	}
}

// defaultActionTimeoutNowFn returns time.Now() as a uint32 UNIX timestamp.
// Kept separate from the struct to allow tests to swap it deterministically.
func defaultActionTimeoutNowFn() uint32 {
	return uint32(time.Now().Unix())
}

// reservationActionTimeoutWalletScanLookBackBlocks bounds the first wallet
// discovery scan Run performs. Mirrors the 30-day-at-12s/block convention
// used elsewhere in this package (e.g. DefaultReservationStaleDepositPollInterval's
// sibling deposit scan). Subsequent scans are incremental from the last
// scanned block, so this bound only matters once, at startup.
const reservationActionTimeoutWalletScanLookBackBlocks = uint64(216000)

// Run starts the background poll loop. It returns when ctx is done or when
// a fatal configuration error is detected.
//
// Each iteration discovers every wallet registered on-chain (incrementally,
// past the block already scanned by the previous iteration), enumerates the
// reservations currently custodied by each wallet, and calls
// CheckReservationActionTimeouts for each one. The loop is best-effort for
// per-reservation failures: a single reservation's error is logged and the
// walk continues with the next one; only a startup configuration error
// (nil resolver, non-positive interval) aborts the loop.
func (ratw *ReservationActionTimeoutWatcher) Run(ctx context.Context) error {
	if ratw.membersResolver == nil {
		return fmt.Errorf(
			"action-timeout watcher requires a non-nil members resolver",
		)
	}
	if ratw.interval <= 0 {
		return fmt.Errorf(
			"action-timeout watcher requires a positive poll interval",
		)
	}

	ticker := time.NewTicker(ratw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		wallets, err := ratw.discoverWallets()
		if err != nil {
			logger.Errorf(
				"action-timeout watcher failed to discover wallets: [%v]",
				err,
			)
			continue
		}

		now := ratw.nowFn()

		for _, walletPublicKeyHash := range wallets {
			reservationKeys, err := ratw.spvChain.WalletReservations(
				walletPublicKeyHash,
			)
			if err != nil {
				logger.Errorf(
					"action-timeout watcher failed to list reservations "+
						"for wallet [0x%x]: [%v]",
					walletPublicKeyHash,
					err,
				)
				continue
			}

			for _, reservationKey := range reservationKeys {
				if err := ratw.CheckReservationActionTimeouts(
					reservationKey,
					now,
				); err != nil {
					logger.Errorf(
						"action-timeout watcher failed to check "+
							"reservation [%v]: [%v]",
						reservationKey,
						err,
					)
				}
			}
		}
	}
}

// discoverWallets returns the public key hashes of every wallet registered
// on-chain since the last call, plus every wallet seen by a prior call.
// Callers must retain the returned slice's wallets across iterations
// themselves if they need the full set; discoverWallets itself only
// accumulates the incremental scan cursor (lastWalletScanBlock) - the
// caller (Run) re-derives the full wallet set from WalletReservations,
// which is authoritative regardless of when the wallet was registered, so
// discoverWallets does not need to cache the wallet list itself.
func (ratw *ReservationActionTimeoutWatcher) discoverWallets() ([][20]byte, error) {
	blockCounter, err := ratw.spvChain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: [%v]", err)
	}

	startBlock := ratw.lastWalletScanBlock
	if startBlock == 0 && currentBlock > reservationActionTimeoutWalletScanLookBackBlocks {
		startBlock = currentBlock - reservationActionTimeoutWalletScanLookBackBlocks
	}

	events, err := ratw.spvChain.PastNewWalletRegisteredEvents(
		&tbtc.NewWalletRegisteredEventFilter{
			StartBlock: func() uint64 {
				if ratw.lastWalletScanBlock != 0 {
					return ratw.lastWalletScanBlock + 1
				}
				return startBlock
			}(),
			EndBlock: &currentBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past new wallet registered events: [%v]",
			err,
		)
	}

	ratw.lastWalletScanBlock = currentBlock

	for _, event := range events {
		ratw.knownWallets[event.WalletPublicKeyHash] = struct{}{}
	}

	wallets := make([][20]byte, 0, len(ratw.knownWallets))
	for wallet := range ratw.knownWallets {
		wallets = append(wallets, wallet)
	}

	return wallets, nil
}

// CheckReservationActionTimeouts inspects the current action generation of a
// single reservation and notifies the Bridge if it is Pending and its
// TimeoutAt has elapsed. The caller controls the iteration; the watcher
// does not background-loop on its own (see Run for the poll-driven caller).
//
// Parameters:
//
//   - reservationKey: the reservation identifier used by the Bridge's
//     ReservationRouter.
//   - now: a UNIX timestamp used to compare against TimeoutAt. Tests pass
//     an explicit value; production passes time.Now().Unix() cast to uint32.
//
// The function resolves the custodying wallet, looks up the operator member
// IDs through the injected resolver, then inspects only the action
// generation at reservation.RequestNonce. By the Bridge invariant, only the
// most-recent action generation can be Pending - older nonces have already
// settled, timed out, or been superseded - so a single lookup suffices; no
// walk from nonce 0 is needed. A RequestNonce of 0 means no action
// generation has ever been requested against the reservation, so there is
// nothing to check.
func (ratw *ReservationActionTimeoutWatcher) CheckReservationActionTimeouts(
	reservationKey *big.Int,
	now uint32,
) error {
	if ratw.membersResolver == nil {
		return fmt.Errorf(
			"action-timeout watcher requires a non-nil members resolver",
		)
	}
	if reservationKey == nil {
		return fmt.Errorf("reservation key must not be nil")
	}

	reservation, err := ratw.spvChain.GetReservation(reservationKey)
	if err != nil {
		return fmt.Errorf(
			"failed to load reservation [%v]: [%v]",
			reservationKey,
			err,
		)
	}

	if reservation.RequestNonce == 0 {
		// No action generation has ever been requested; nothing pending.
		return nil
	}

	walletPublicKeyHash := reservation.WalletPublicKeyHash
	if walletPublicKeyHash == ([20]byte{}) {
		// Reservation exists but has no wallet assigned (e.g. the
		// acceptance has not progressed yet). Without a wallet we cannot
		// resolve members, so the watcher skips silently: the stranding
		// watcher will eventually catch this case.
		logger.Debugf(
			"reservation [%v] has no wallet assigned; "+
				"action-timeout watcher skipping",
			reservationKey,
		)
		return nil
	}

	// Resolve the wallet members exactly once per Check call: the Bridge
	// requires the member IDs to be consistent across all notifications
	// issued in response to a single reservation.
	memberIDs, err := ratw.membersResolver.ResolveWalletMembers(
		walletPublicKeyHash,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to resolve wallet member IDs for "+
				"wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
	if len(memberIDs) == 0 {
		// Emitting NotifyReservationActionTimeout with a nil/empty member
		// slice is ill-formed on the Bridge side (see
		// NewReservationActionTimeoutWatcher's doc). Refuse rather than
		// notify on partial information: a misconfigured members resolver
		// must fail loud, not silently strand the slashing attribution.
		return fmt.Errorf(
			"wallet [0x%x] members resolver returned an empty set; "+
				"refusing to notify with no attributable members",
			walletPublicKeyHash,
		)
	}

	nonce := reservation.RequestNonce

	action, err := ratw.spvChain.GetReservationAction(reservationKey, nonce)
	if err != nil {
		return fmt.Errorf(
			"failed to load action for reservation [%v] at nonce %d: [%v]",
			reservationKey,
			nonce,
			err,
		)
	}

	if action.State != tbtc.ReservationActionStatePending {
		logger.Debugf(
			"reservation [%v] action nonce %d state=%s; not pending, "+
				"nothing to time out",
			reservationKey,
			nonce,
			action.State,
		)
		return nil
	}

	if now <= action.TimeoutAt {
		logger.Debugf(
			"reservation [%v] action nonce %d timeout at [%d] "+
				"not yet reached (now=%d); skipping",
			reservationKey,
			nonce,
			action.TimeoutAt,
			now,
		)
		return nil
	}

	if err := ratw.spvChain.NotifyReservationActionTimeout(
		reservationKey,
		memberIDs,
	); err != nil {
		return fmt.Errorf(
			"failed to notify action timeout for "+
				"reservation [%v] nonce %d: [%v]",
			reservationKey,
			nonce,
			err,
		)
	}

	logger.Infof(
		"notified action timeout for reservation [%v] nonce %d "+
			"(timeout=%d, members=%d)",
		reservationKey,
		nonce,
		action.TimeoutAt,
		len(memberIDs),
	)

	return nil
}

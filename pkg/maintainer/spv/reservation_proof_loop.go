package spv

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// errReservationActionNoLongerProvable is returned when a discovered
// transaction's action generation is no longer provable at submission time.
// This is an expected, benign skip rather than a submission failure.
var errReservationActionNoLongerProvable = errors.New(
	"reservation action generation is no longer provable",
)

// reservationProofLookBackBlocks bounds the pending-action-request event
// scan performed on the very first pass, before an incremental cursor
// exists. Mirrors ReservationAcceptanceLookBackBlocks /
// ReservationReanchorLookBackBlocks in pkg/tbtcpg: 30 days at 12s/block.
const reservationProofLookBackBlocks = uint64(216000)

// verifyReservationActionStillProvable re-fetches the reservation action at
// (reservationKey, requestNonce) immediately before an SPV proof
// submission and confirms it is still the exact pending action generation
// the discovered transaction was found for.
//
// Its purpose is to distinguish an expected, benign "this action generation is
// no longer the exact pending one" outcome (Warn-logged, and skipped so it is
// treated as "never attempted" rather than counted as a failed submission
// attempt by metricsRecorder) from a genuine chain-read error (propagated to
// the caller) or a genuine logic error caught later inside
// submitReservationActionProof (which remains the authoritative
// pre-submission check — it re-fetches the action itself right before
// SubmitReservationProof and is what actually prevents an incorrect or
// misdirected submission).
//
// This function does not, by itself, close any submission-correctness race —
// it only produces cleaner logs and metrics for an expected outcome that
// submitReservationActionProof's own checks already handle safely either way.
func verifyReservationActionStillProvable(
	spvChain Chain,
	reservationKey *big.Int,
	requestNonce uint64,
	expectedActionType tbtc.ReservationActionType,
	expectedTargetWalletPublicKeyHash [20]byte,
) (bool, error) {
	action, err := spvChain.GetReservationAction(reservationKey, requestNonce)
	if err != nil {
		return false, fmt.Errorf(
			"failed to re-verify reservation action [%v]/%d: [%v]",
			reservationKey,
			requestNonce,
			err,
		)
	}

	if action.ActionType != expectedActionType ||
		action.State != tbtc.ReservationActionStatePending {
		logger.Warnf(
			"skipping reservation proof submission for reservation "+
				"[%v]'s action generation [%d]: action generation is now "+
				"%s/%s, no longer the expected pending %s action",
			reservationKey,
			requestNonce,
			action.ActionType.String(),
			action.State.String(),
			expectedActionType.String(),
		)
		return false, nil
	}

	if action.TargetWalletPublicKeyHash != expectedTargetWalletPublicKeyHash {
		logger.Warnf(
			"skipping reservation proof submission for reservation "+
				"[%v]'s action generation [%d]: target wallet changed "+
				"since discovery",
			reservationKey,
			requestNonce,
		)
		return false, nil
	}

	return true, nil
}

// submitReservationAcceptanceActionProof re-verifies that event's action
// generation is still the exact pending one the discovered transaction was
// found for, then submits its SPV proof. Extracted out of
// proveReservationAcceptanceActions' submit callback so the wallet
// argument passed to verifyReservationActionStillProvable
// (event.WalletPublicKeyHash) can be exercised directly in a unit test,
// without going through Bitcoin transaction discovery.
func submitReservationAcceptanceActionProof(
	spvChain Chain,
	btcChain bitcoin.Chain,
	event *tbtc.ReservationAcceptanceRequestedEvent,
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
) error {
	stillProvable, err := verifyReservationActionStillProvable(
		spvChain,
		event.ReservationKey,
		event.RequestNonce,
		tbtc.ReservationActionTypeAcceptance,
		event.WalletPublicKeyHash,
	)
	if err != nil {
		return err
	}
	if !stillProvable {
		return errReservationActionNoLongerProvable
	}

	return SubmitReservationAcceptanceProof(
		transactionHash,
		requiredConfirmations,
		event.ReservationKey,
		event.RequestNonce,
		btcChain,
		spvChain,
	)
}

// submitReservationReanchorActionProof re-verifies that event's action
// generation is still the exact pending one the discovered transaction was
// found for, then submits its SPV proof. Extracted out of
// proveReservationReanchorActions' submit callback so the
// target-vs-source wallet-hash field selection passed to
// verifyReservationActionStillProvable (event.TargetWalletPublicKeyHash,
// not event.SourceWalletPublicKeyHash — a re-anchor event carries both)
// can be exercised directly in a unit test, without going through Bitcoin
// transaction discovery: this package's local test double can only
// discover a transaction via the source wallet's outputs, which forces
// the two fields to coincide by construction in any end-to-end test and
// so cannot catch a swap between them.
func submitReservationReanchorActionProof(
	spvChain Chain,
	btcChain bitcoin.Chain,
	event *tbtc.ReservationReanchorRequestedEvent,
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
) error {
	stillProvable, err := verifyReservationActionStillProvable(
		spvChain,
		event.ReservationKey,
		event.RequestNonce,
		tbtc.ReservationActionTypeReanchor,
		event.TargetWalletPublicKeyHash,
	)
	if err != nil {
		return err
	}
	if !stillProvable {
		return errReservationActionNoLongerProvable
	}

	return SubmitReservationReanchorProof(
		transactionHash,
		requiredConfirmations,
		event.ReservationKey,
		event.RequestNonce,
		btcChain,
		spvChain,
	)
}

// reservationAcceptanceWalletEvent adapts
// *tbtc.ReservationAcceptanceRequestedEvent to the walletEvent interface
// (see spv.go) so uniqueWalletPublicKeyHashes can be reused here instead of
// a reservation-specific duplicate of the same dedup logic.
type reservationAcceptanceWalletEvent struct {
	*tbtc.ReservationAcceptanceRequestedEvent
}

// GetWalletPublicKeyHash implements walletEvent.
func (e reservationAcceptanceWalletEvent) GetWalletPublicKeyHash() [20]byte {
	return e.WalletPublicKeyHash
}

// reservationReanchorWalletEvent adapts
// *tbtc.ReservationReanchorRequestedEvent to the walletEvent interface (see
// spv.go) so uniqueWalletPublicKeyHashes can be reused here instead of a
// reservation-specific duplicate of the same dedup logic.
type reservationReanchorWalletEvent struct {
	*tbtc.ReservationReanchorRequestedEvent
}

// GetWalletPublicKeyHash implements walletEvent.
func (e reservationReanchorWalletEvent) GetWalletPublicKeyHash() [20]byte {
	return e.SourceWalletPublicKeyHash
}

func wrapReservationAcceptanceEvents(
	events []*tbtc.ReservationAcceptanceRequestedEvent,
) []reservationAcceptanceWalletEvent {
	wrapped := make([]reservationAcceptanceWalletEvent, len(events))
	for i, event := range events {
		wrapped[i] = reservationAcceptanceWalletEvent{event}
	}
	return wrapped
}

func wrapReservationReanchorEvents(
	events []*tbtc.ReservationReanchorRequestedEvent,
) []reservationReanchorWalletEvent {
	wrapped := make([]reservationReanchorWalletEvent, len(events))
	for i, event := range events {
		wrapped[i] = reservationReanchorWalletEvent{event}
	}
	return wrapped
}

// reservationProofScanState persists the incremental event-scan cursor and
// the set of still-pending action-request events across successive passes
// of runReservationProofLoop, so proveReservationAcceptanceActions and
// proveReservationReanchorActions scan only the event/Bitcoin history that
// has appeared since the previous pass instead of rescanning the full
// reservationProofLookBackBlocks window - and refetching Bitcoin history
// for every wallet in it - every config.IdleBackoffTime.
type reservationProofScanState struct {
	acceptanceLastScannedBlock uint64
	pendingAcceptanceEvents    map[string]*tbtc.ReservationAcceptanceRequestedEvent

	reanchorLastScannedBlock uint64
	pendingReanchorEvents    map[string]*tbtc.ReservationReanchorRequestedEvent
}

func newReservationProofScanState() *reservationProofScanState {
	return &reservationProofScanState{
		pendingAcceptanceEvents: make(map[string]*tbtc.ReservationAcceptanceRequestedEvent),
		pendingReanchorEvents:   make(map[string]*tbtc.ReservationReanchorRequestedEvent),
	}
}

// reservationEventKey identifies one reservation action generation, unique
// across both the acceptance and re-anchor pending-event maps.
func reservationEventKey(reservationKey *big.Int, requestNonce uint64) string {
	return fmt.Sprintf("%s:%d", reservationKey.String(), requestNonce)
}

// reservationProofNextScanRange returns the block range to scan for new
// pending-action-request events this pass: the bounded
// reservationProofLookBackBlocks catch-up window on the very first pass
// (lastScannedBlock == 0), or just the delta since the previous pass's
// cursor on every pass thereafter, so a steady-state loop no longer
// re-fetches the full ~30-day window on every config.IdleBackoffTime tick.
func reservationProofNextScanRange(
	spvChain Chain,
	lastScannedBlock uint64,
) (startBlock uint64, currentBlock uint64, err error) {
	blockCounter, err := spvChain.BlockCounter()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err = blockCounter.CurrentBlock()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get current block: [%v]", err)
	}

	if lastScannedBlock == 0 {
		if currentBlock > reservationProofLookBackBlocks {
			return currentBlock - reservationProofLookBackBlocks, currentBlock, nil
		}
		return 0, currentBlock, nil
	}

	return lastScannedBlock + 1, currentBlock, nil
}

// maintainReservationProofs runs the SPV proof submission loop for
// reservation acceptance and re-anchor action generations. It is a
// dedicated loop, separate from spvMaintainer's generic proofTypes-driven
// control loop (see spv.go's Initialize), because SubmitReservationProof
// requires the (reservationKey, requestNonce) pair of the action generation
// being proven - context the generic
// unprovenTransactionsGetter/transactionProofSubmitter signatures (shared
// by deposit sweep, redemption, moving funds, and moved funds sweep) cannot
// carry.
//
// The loop shape mirrors spvMaintainer.startControlLoop/maintainSpv: an
// outer restart-backoff loop wraps an inner idle-backoff loop, so a
// transient error restarts after config.RestartBackoffTime and a clean pass
// with nothing to prove waits config.IdleBackoffTime before trying again.
func maintainReservationProofs(
	ctx context.Context,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
) {
	logger.Info("starting reservation proof maintainer")

	defer func() {
		logger.Info("stopping reservation proof maintainer")
	}()

	for {
		err := runReservationProofLoop(ctx, config, spvChain, btcDiffChain, btcChain)
		if err != nil {
			logger.Errorf(
				"error while maintaining reservation proofs: [%v]; "+
					"restarting reservation proof maintainer",
				err,
			)
		}

		select {
		case <-time.After(config.RestartBackoffTime):
		case <-ctx.Done():
			return
		}
	}
}

// runReservationProofLoop repeatedly proves pending reservation acceptance
// and re-anchor action generations until ctx is done or an unrecoverable
// error occurs. Per-action errors (a single reservation's proof failing to
// assemble or submit) are logged and skipped rather than propagated, so one
// bad action generation does not block the rest; only a chain-wide failure
// (e.g. cannot read the current block) aborts the pass and triggers the
// outer restart backoff.
//
// A single reservationProofScanState is created once and threaded through
// every pass for the lifetime of the loop, carrying the incremental event
// cursor and pending-action set described on that type.
func runReservationProofLoop(
	ctx context.Context,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
) error {
	state := newReservationProofScanState()

	for {
		if err := proveReservationAcceptanceActions(
			state,
			config,
			spvChain,
			btcDiffChain,
			btcChain,
		); err != nil {
			return fmt.Errorf(
				"error while proving reservation acceptance actions: [%v]",
				err,
			)
		}

		if err := proveReservationReanchorActions(
			state,
			config,
			spvChain,
			btcDiffChain,
			btcChain,
		); err != nil {
			return fmt.Errorf(
				"error while proving reservation re-anchor actions: [%v]",
				err,
			)
		}

		select {
		case <-time.After(config.IdleBackoffTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// proveReservationAcceptanceActions finds pending ReservationAcceptance
// action generations, locates each one's already-broadcast anchor
// transaction on the Bitcoin chain (if any), and submits its SPV proof once
// it has accumulated enough confirmations.
func proveReservationAcceptanceActions(
	state *reservationProofScanState,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
) error {
	startBlock, currentBlock, err := reservationProofNextScanRange(
		spvChain,
		state.acceptanceLastScannedBlock,
	)
	if err != nil {
		return err
	}

	newEvents, err := spvChain.PastReservationAcceptanceRequestedEvents(
		&tbtc.ReservationAcceptanceRequestedEventFilter{StartBlock: startBlock},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation acceptance requested "+
				"events: [%v]",
			err,
		)
	}

	for _, event := range newEvents {
		key := reservationEventKey(event.ReservationKey, event.RequestNonce)
		state.pendingAcceptanceEvents[key] = event
	}

	// Re-check every tracked event's on-chain action state and drop the
	// ones that are no longer pending, so the pending set does not grow
	// without bound.
	var pending []*tbtc.ReservationAcceptanceRequestedEvent
	for key, event := range state.pendingAcceptanceEvents {
		action, err := spvChain.GetReservationAction(
			event.ReservationKey,
			event.RequestNonce,
		)
		if err != nil {
			logger.Errorf(
				"failed to load reservation acceptance action [%v]/%d: [%v]",
				event.ReservationKey,
				event.RequestNonce,
				err,
			)
			// Keep tracking; retry on the next pass.
			continue
		}
		if action.State != tbtc.ReservationActionStatePending {
			delete(state.pendingAcceptanceEvents, key)
			continue
		}
		pending = append(pending, event)
	}

	// There will often be multiple pending events for a single wallet.
	// Fetch that wallet's Bitcoin transaction history once, not once per
	// event.
	walletPublicKeyHashes := uniqueWalletPublicKeyHashes(
		wrapReservationAcceptanceEvents(pending),
	)

	for _, walletPublicKeyHash := range walletPublicKeyHashes {
		walletTransactions, err := btcChain.GetTransactionsForPublicKeyHash(
			walletPublicKeyHash,
			config.TransactionLimit,
		)
		if err != nil {
			logger.Errorf("failed to get transactions for wallet: [%v]", err)
			continue
		}

		for _, event := range pending {
			if event.WalletPublicKeyHash != walletPublicKeyHash {
				continue
			}

			transaction, err := findReservationAcceptanceTransaction(
				spvChain,
				event,
				walletTransactions,
			)
			if err != nil {
				logger.Errorf(
					"failed to search for reservation acceptance transaction "+
						"for reservation [%v]: [%v]",
					event.ReservationKey,
					err,
				)
				continue
			}
			if transaction == nil {
				continue
			}

			if err := proveReservationTransaction(
				transaction,
				btcChain,
				spvChain,
				btcDiffChain,
				func(transactionHash bitcoin.Hash, requiredConfirmations uint) error {
					return submitReservationAcceptanceActionProof(
						spvChain,
						btcChain,
						event,
						transactionHash,
						requiredConfirmations,
					)
				},
			); err != nil {
				logger.Errorf(
					"failed to prove reservation acceptance transaction [%s] "+
						"for reservation [%v]: [%v]",
					transaction.Hash().Hex(bitcoin.ReversedByteOrder),
					event.ReservationKey,
					err,
				)
				continue
			}
		}
	}

	state.acceptanceLastScannedBlock = currentBlock

	return nil
}

// findReservationAcceptanceTransaction scans the candidate wallet's Bitcoin
// transaction history for the 1-input-1-output acceptance (anchor)
// transaction whose sole input spends the deposit identified by
// event.ReservationKey (== the deposit key; see the m1 identity mapping
// documented in reservation_stale_deposit_watch.go). Returns nil, nil if no
// matching transaction has been broadcast yet.
func findReservationAcceptanceTransaction(
	spvChain Chain,
	event *tbtc.ReservationAcceptanceRequestedEvent,
	walletTransactions []*bitcoin.Transaction,
) (*bitcoin.Transaction, error) {
	for _, transaction := range walletTransactions {
		if len(transaction.Inputs) != 1 || len(transaction.Outputs) != 1 {
			continue
		}

		input := transaction.Inputs[0]
		depositKey := spvChain.BuildDepositKey(
			input.Outpoint.TransactionHash,
			input.Outpoint.OutputIndex,
		)

		if depositKey.Cmp(event.ReservationKey) == 0 {
			return transaction, nil
		}
	}

	return nil, nil
}

// proveReservationReanchorActions finds pending ReservationReanchor action
// generations, locates each one's already-broadcast re-anchor transaction
// on the Bitcoin chain (if any), and submits its SPV proof once it has
// accumulated enough confirmations.
func proveReservationReanchorActions(
	state *reservationProofScanState,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
) error {
	startBlock, currentBlock, err := reservationProofNextScanRange(
		spvChain,
		state.reanchorLastScannedBlock,
	)
	if err != nil {
		return err
	}

	newEvents, err := spvChain.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{StartBlock: startBlock},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation re-anchor requested events: [%v]",
			err,
		)
	}

	for _, event := range newEvents {
		key := reservationEventKey(event.ReservationKey, event.RequestNonce)
		state.pendingReanchorEvents[key] = event
	}

	// Re-check every tracked event's on-chain action state and drop the
	// ones that are no longer pending, so the pending set does not grow
	// without bound.
	var pending []*tbtc.ReservationReanchorRequestedEvent
	for key, event := range state.pendingReanchorEvents {
		action, err := spvChain.GetReservationAction(
			event.ReservationKey,
			event.RequestNonce,
		)
		if err != nil {
			logger.Errorf(
				"failed to load reservation re-anchor action [%v]/%d: [%v]",
				event.ReservationKey,
				event.RequestNonce,
				err,
			)
			// Keep tracking; retry on the next pass.
			continue
		}
		if action.State != tbtc.ReservationActionStatePending {
			delete(state.pendingReanchorEvents, key)
			continue
		}
		pending = append(pending, event)
	}

	// There will often be multiple pending events for a single source
	// wallet. Fetch that wallet's Bitcoin transaction history once, not
	// once per event.
	walletPublicKeyHashes := uniqueWalletPublicKeyHashes(
		wrapReservationReanchorEvents(pending),
	)

	for _, walletPublicKeyHash := range walletPublicKeyHashes {
		walletTransactions, err := btcChain.GetTransactionsForPublicKeyHash(
			walletPublicKeyHash,
			config.TransactionLimit,
		)
		if err != nil {
			logger.Errorf("failed to get transactions for wallet: [%v]", err)
			continue
		}

		for _, event := range pending {
			if event.SourceWalletPublicKeyHash != walletPublicKeyHash {
				continue
			}

			reservation, err := spvChain.GetReservation(event.ReservationKey)
			if err != nil {
				logger.Errorf(
					"failed to load reservation [%v]: [%v]",
					event.ReservationKey,
					err,
				)
				continue
			}
			if reservation.AnchorUtxo == nil ||
				reservation.AnchorUtxo.Value == 0 ||
				reservation.AnchorUtxo.Outpoint == nil ||
				reservation.AnchorUtxo.Outpoint.TransactionHash == (bitcoin.Hash{}) {
				logger.Errorf(
					"reservation [%v] has no anchor UTXO to re-anchor from",
					event.ReservationKey,
				)
				continue
			}

			transaction, err := findReservationReanchorTransaction(
				event,
				reservation.AnchorUtxo,
				walletTransactions,
			)
			if err != nil {
				logger.Errorf(
					"failed to search for reservation re-anchor transaction "+
						"for reservation [%v]: [%v]",
					event.ReservationKey,
					err,
				)
				continue
			}
			if transaction == nil {
				continue
			}

			if err := proveReservationTransaction(
				transaction,
				btcChain,
				spvChain,
				btcDiffChain,
				func(transactionHash bitcoin.Hash, requiredConfirmations uint) error {
					return submitReservationReanchorActionProof(
						spvChain,
						btcChain,
						event,
						transactionHash,
						requiredConfirmations,
					)
				},
			); err != nil {
				logger.Errorf(
					"failed to prove reservation re-anchor transaction [%s] "+
						"for reservation [%v]: [%v]",
					transaction.Hash().Hex(bitcoin.ReversedByteOrder),
					event.ReservationKey,
					err,
				)
				continue
			}
		}
	}

	state.reanchorLastScannedBlock = currentBlock

	return nil
}

// findReservationReanchorTransaction scans the source wallet's Bitcoin
// transaction history for the 1-input-1-output re-anchor transaction whose
// sole input spends the reservation's current anchor UTXO. Returns nil, nil
// if no matching transaction has been broadcast yet.
func findReservationReanchorTransaction(
	event *tbtc.ReservationReanchorRequestedEvent,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	walletTransactions []*bitcoin.Transaction,
) (*bitcoin.Transaction, error) {
	for _, transaction := range walletTransactions {
		if len(transaction.Inputs) != 1 || len(transaction.Outputs) != 1 {
			continue
		}

		input := transaction.Inputs[0]
		if input.Outpoint.TransactionHash == anchorUtxo.Outpoint.TransactionHash &&
			input.Outpoint.OutputIndex == anchorUtxo.Outpoint.OutputIndex {
			return transaction, nil
		}
	}

	return nil, nil
}

// proveReservationTransaction assembles and submits the SPV proof for a
// single reservation acceptance or re-anchor transaction, once it has
// accumulated enough confirmations and its proof falls within the relay's
// difficulty range.
func proveReservationTransaction(
	transaction *bitcoin.Transaction,
	btcChain bitcoin.Chain,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	submit func(transactionHash bitcoin.Hash, requiredConfirmations uint) error,
) error {
	transactionHashStr := transaction.Hash().Hex(bitcoin.ReversedByteOrder)

	isProofWithinRelayRange, accumulatedConfirmations, requiredConfirmations, err :=
		getProofInfo(transaction.Hash(), btcChain, spvChain, btcDiffChain)
	if err != nil {
		return fmt.Errorf("failed to get proof info: [%v]", err)
	}

	if !isProofWithinRelayRange {
		logger.Warnf(
			"skipped proving transaction [%s]; the range of the "+
				"required proof goes outside the previous and current "+
				"difficulty epochs as seen by the relay",
			transactionHashStr,
		)
		return nil
	}

	if accumulatedConfirmations < requiredConfirmations {
		logger.Infof(
			"skipped proving transaction [%s]; transaction has [%v/%v] "+
				"confirmations",
			transactionHashStr,
			accumulatedConfirmations,
			requiredConfirmations,
		)
		return nil
	}

	if err := submit(transaction.Hash(), requiredConfirmations); err != nil {
		if errors.Is(err, errReservationActionNoLongerProvable) {
			return nil
		}

		return err
	}

	logger.Infof(
		"successfully submitted proof for transaction [%s]",
		transactionHashStr,
	)

	return nil
}

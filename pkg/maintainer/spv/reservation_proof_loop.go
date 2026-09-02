package spv

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationProofLookBackBlocks bounds the pending-action-request event
// scan. Mirrors ReservationAcceptanceLookBackBlocks /
// ReservationReanchorLookBackBlocks in pkg/tbtcpg: 30 days at 12s/block.
const reservationProofLookBackBlocks = uint64(216000)

// verifyReservationActionStillProvable re-fetches the reservation action at
// (reservationKey, requestNonce) immediately before an SPV proof
// submission and confirms it is still the exact pending action generation
// the discovered transaction was found for.
//
// proveReservationAcceptanceActions/proveReservationReanchorActions check
// Pending once near the top of their loop, then run a Bitcoin
// transaction-history scan before reaching the submit call - a window in
// which the action generation could settle, time out, or be superseded.
// This closes that window with a second, submission-time check.
//
// Whether a stale generation's action record could still read Pending
// after a superseding generation exists is an unverified on-chain
// assumption (see reservation_reanchor_proof.go's doc comments on
// submitReservationReanchorProof's discovery counterpart in the prior
// design) - this check does not resolve that, it only shrinks the window
// in which it could matter and skips, rather than misdirects a
// submission, if it does.
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
				"[%v]'s action generation [%d]: no longer a pending %v "+
				"action at submission time",
			reservationKey,
			requestNonce,
			expectedActionType,
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
func runReservationProofLoop(
	ctx context.Context,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
) error {
	for {
		if err := proveReservationAcceptanceActions(
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
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
) error {
	startBlock, err := reservationProofScanStartBlock(spvChain)
	if err != nil {
		return err
	}

	events, err := spvChain.PastReservationAcceptanceRequestedEvents(
		&tbtc.ReservationAcceptanceRequestedEventFilter{StartBlock: startBlock},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation acceptance requested "+
				"events: [%v]",
			err,
		)
	}

	for _, event := range events {
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
			continue
		}
		if action.State != tbtc.ReservationActionStatePending {
			// Already proven (Settled), or no longer provable
			// (TimedOut/Superseded/Vetoed).
			continue
		}

		transaction, err := findReservationAcceptanceTransaction(
			spvChain,
			btcChain,
			event,
			config.TransactionLimit,
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
			// The wallet has not broadcast the anchor transaction yet.
			continue
		}

		if err := proveReservationTransaction(
			transaction,
			btcChain,
			spvChain,
			btcDiffChain,
			func(transactionHash bitcoin.Hash, requiredConfirmations uint) error {
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
					return nil
				}

				return SubmitReservationAcceptanceProof(
					transactionHash,
					requiredConfirmations,
					event.ReservationKey,
					event.RequestNonce,
					btcChain,
					spvChain,
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
	btcChain bitcoin.Chain,
	event *tbtc.ReservationAcceptanceRequestedEvent,
	transactionLimit int,
) (*bitcoin.Transaction, error) {
	walletTransactions, err := btcChain.GetTransactionsForPublicKeyHash(
		event.WalletPublicKeyHash,
		transactionLimit,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transactions for wallet: [%v]",
			err,
		)
	}

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
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
) error {
	startBlock, err := reservationProofScanStartBlock(spvChain)
	if err != nil {
		return err
	}

	events, err := spvChain.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{StartBlock: startBlock},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation re-anchor requested events: [%v]",
			err,
		)
	}

	for _, event := range events {
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
			continue
		}
		if action.State != tbtc.ReservationActionStatePending {
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
		if reservation.AnchorUtxo == nil || reservation.AnchorUtxo.Outpoint == nil {
			logger.Errorf(
				"reservation [%v] has no anchor UTXO to re-anchor from",
				event.ReservationKey,
			)
			continue
		}

		transaction, err := findReservationReanchorTransaction(
			btcChain,
			event,
			reservation.AnchorUtxo,
			config.TransactionLimit,
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
					return nil
				}

				return SubmitReservationReanchorProof(
					transactionHash,
					requiredConfirmations,
					event.ReservationKey,
					event.RequestNonce,
					btcChain,
					spvChain,
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

	return nil
}

// findReservationReanchorTransaction scans the source wallet's Bitcoin
// transaction history for the 1-input-1-output re-anchor transaction whose
// sole input spends the reservation's current anchor UTXO. Returns nil, nil
// if no matching transaction has been broadcast yet.
func findReservationReanchorTransaction(
	btcChain bitcoin.Chain,
	event *tbtc.ReservationReanchorRequestedEvent,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	transactionLimit int,
) (*bitcoin.Transaction, error) {
	walletTransactions, err := btcChain.GetTransactionsForPublicKeyHash(
		event.SourceWalletPublicKeyHash,
		transactionLimit,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transactions for wallet: [%v]",
			err,
		)
	}

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

// proveReservationTransaction checks the given transaction's confirmation
// and relay-range status via the shared getProofInfo helper (also used by
// the generic proof loop in spv.go) and, once ready, invokes submit with
// the transaction hash and required confirmations.
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
		return err
	}

	logger.Infof(
		"successfully submitted proof for transaction [%s]",
		transactionHashStr,
	)

	return nil
}

// reservationProofScanStartBlock returns the start block for a bounded,
// look-back-limited scan of pending-action-request events.
func reservationProofScanStartBlock(spvChain Chain) (uint64, error) {
	blockCounter, err := spvChain.BlockCounter()
	if err != nil {
		return 0, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return 0, fmt.Errorf("failed to get current block: [%v]", err)
	}

	if currentBlock > reservationProofLookBackBlocks {
		return currentBlock - reservationProofLookBackBlocks, nil
	}

	return 0, nil
}

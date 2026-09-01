package spv

import (
	"context"
	"fmt"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationProofLookBackBlocks bounds the pending-action-request event
// scan. Mirrors ReservationAcceptanceLookBackBlocks /
// ReservationReanchorLookBackBlocks in pkg/tbtcpg: 30 days at 12s/block.
const reservationProofLookBackBlocks = uint64(216000)

// uniqueReservationWalletPublicKeyHashes deduplicates a list of reservation
// proof-loop events by wallet public key hash, extracted via
// walletPublicKeyHashOf since the acceptance and re-anchor event types name
// their wallet field differently (WalletPublicKeyHash vs
// SourceWalletPublicKeyHash) and therefore cannot share the walletEvent
// interface used by uniqueWalletPublicKeyHashes in spv.go.
func uniqueReservationWalletPublicKeyHashes[T any](
	items []T,
	walletPublicKeyHashOf func(T) [20]byte,
) [][20]byte {
	seen := make(map[[20]byte]struct{})
	var result [][20]byte
	for _, item := range items {
		pkh := walletPublicKeyHashOf(item)
		if _, ok := seen[pkh]; !ok {
			seen[pkh] = struct{}{}
			result = append(result, pkh)
		}
	}
	return result
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

	// There will often be multiple events emitted for a single wallet. Prepare
	// a list of unique wallet public key hashes.
	walletPublicKeyHashes := uniqueReservationWalletPublicKeyHashes(
		events,
		func(e *tbtc.ReservationAcceptanceRequestedEvent) [20]byte {
			return e.WalletPublicKeyHash
		},
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

		for _, event := range events {
			if event.WalletPublicKeyHash != walletPublicKeyHash {
				continue
			}

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

	// There will often be multiple events emitted for a single source
	// wallet. Prepare a list of unique wallet public key hashes so the
	// transaction history is fetched once per wallet instead of once per
	// event, mirroring the acceptance loop above and the sibling
	// getUnprovenDepositSweepTransactions convention.
	walletPublicKeyHashes := uniqueReservationWalletPublicKeyHashes(
		events,
		func(e *tbtc.ReservationReanchorRequestedEvent) [20]byte {
			return e.SourceWalletPublicKeyHash
		},
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

		for _, event := range events {
			if event.SourceWalletPublicKeyHash != walletPublicKeyHash {
				continue
			}

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
	}

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

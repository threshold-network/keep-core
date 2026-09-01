package spv

import (
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ProofTypeReservationReanchor is the value passed to
// SubmitReservationProof as proofType for a reservation re-anchor SPV proof.
// The numeric value mirrors the on-chain ReservationProofType enum (3 =
// Reanchor).
const ProofTypeReservationReanchor uint8 = 3

// SubmitReservationReanchorProof drives the SPV proof submission for a
// reservation re-anchor action generation. The caller (typically the
// reservation re-anchor watcher) supplies the (reservationKey, requestNonce)
// pair of the on-chain action generation it is proving, plus the Bitcoin
// transaction hash of the re-anchor transaction already signed and
// broadcast by the wallet coordinator. The proof is fetched from btcChain,
// the re-anchor transaction is rebuilt locally to extract the anchor UTXO
// and target wallet, and the proof is submitted directly to the Bridge via
// the SPV maintainer's SubmitReservationProof entry point (not via
// MaintainerProxy: reservations are not reimbursed).
//
// requiredConfirmations must be > 0; the SPV maintainer relies on it to
// assemble the proof.
func SubmitReservationReanchorProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	reservationKey *big.Int,
	requestNonce uint64,
	btcChain bitcoin.Chain,
	spvChain Chain,
) error {
	return submitReservationReanchorProof(
		transactionHash,
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
	)
}

func submitReservationReanchorProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	reservationKey *big.Int,
	requestNonce uint64,
	btcChain bitcoin.Chain,
	spvChain Chain,
	spvProofAssembler spvProofAssembler,
) error {
	if requiredConfirmations == 0 {
		return fmt.Errorf(
			"provided required confirmations count must be greater than 0",
		)
	}
	if reservationKey == nil {
		return fmt.Errorf("reservation key is required")
	}
	if requestNonce == 0 {
		return fmt.Errorf("request nonce must be > 0")
	}

	transaction, proof, err := spvProofAssembler(
		transactionHash,
		requiredConfirmations,
		btcChain,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to assemble transaction spv proof: [%v]",
			err,
		)
	}

	anchorUtxo, _, err := parseReservationReanchorTransactionInput(
		btcChain,
		transaction,
	)
	if err != nil {
		return fmt.Errorf(
			"error while parsing reservation re-anchor transaction inputs: [%v]",
			err,
		)
	}

	action, err := spvChain.GetReservationAction(reservationKey, requestNonce)
	if err != nil {
		return fmt.Errorf(
			"cannot fetch reservation action generation: [%v]",
			err,
		)
	}

	if action.ActionType != tbtc.ReservationActionTypeReanchor {
		return fmt.Errorf(
			"reservation action generation is not a re-anchor (got %v)",
			action.ActionType,
		)
	}

	if action.State != tbtc.ReservationActionStatePending {
		return fmt.Errorf(
			"reservation re-anchor action generation is not pending (state=%v)",
			action.State,
		)
	}

	txInfo := buildReservationProofTxInfo(transaction)
	txProof := buildReservationProofTxProof(proof)
	mainUtxo := buildReservationProofMainUtxo(anchorUtxo)

	if err := spvChain.SubmitReservationProof(
		ProofTypeReservationReanchor,
		txInfo,
		txProof,
		mainUtxo,
		reservationKey,
		requestNonce,
	); err != nil {
		return fmt.Errorf(
			"failed to submit reservation re-anchor proof: [%v]",
			err,
		)
	}

	return nil
}

// parseReservationReanchorTransactionInput parses the single input and
// single output of a reservation re-anchor transaction and returns the
// anchor UTXO that was spent and the target wallet's public key hash from
// the new anchor output script.
func parseReservationReanchorTransactionInput(
	btcChain bitcoin.Chain,
	transaction *bitcoin.Transaction,
) (*bitcoin.UnspentTransactionOutput, [20]byte, error) {
	if len(transaction.Inputs) != 1 {
		return nil, [20]byte{}, fmt.Errorf(
			"reservation re-anchor transaction must have exactly one input",
		)
	}

	if len(transaction.Outputs) != 1 {
		return nil, [20]byte{}, fmt.Errorf(
			"reservation re-anchor transaction must have exactly one output",
		)
	}

	input := transaction.Inputs[0]

	inputTx, err := btcChain.GetTransaction(input.Outpoint.TransactionHash)
	if err != nil {
		return nil, [20]byte{}, fmt.Errorf(
			"cannot get input transaction data: [%v]",
			err,
		)
	}

	spentOutput := inputTx.Outputs[input.Outpoint.OutputIndex]

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: input.Outpoint,
		Value:    spentOutput.Value,
	}

	targetWalletPublicKeyHash, err := bitcoin.ExtractPublicKeyHash(
		transaction.Outputs[0].PublicKeyScript,
	)
	if err != nil {
		return nil, [20]byte{}, fmt.Errorf(
			"cannot extract target wallet public key hash: [%v]",
			err,
		)
	}

	return anchorUtxo, targetWalletPublicKeyHash, nil
}

// buildReservationProofTxInfo serializes the relevant parts of the
// transaction into the BitcoinTxInfo structure expected by
// SubmitReservationProof.
func buildReservationProofTxInfo(
	transaction *bitcoin.Transaction,
) *tbtc.BitcoinTxInfo {
	return &tbtc.BitcoinTxInfo{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
}

// buildReservationProofTxProof converts a bitcoin.SpvProof into the
// BitcoinTxProof structure expected by SubmitReservationProof.
func buildReservationProofTxProof(
	proof *bitcoin.SpvProof,
) *tbtc.BitcoinTxProof {
	txIndexInBlock := big.NewInt(int64(proof.TxIndexInBlock))

	return &tbtc.BitcoinTxProof{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   txIndexInBlock,
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
}

// buildReservationProofMainUtxo packages the spent anchor UTXO into the
// BitcoinTxUTXO structure expected by SubmitReservationProof.
func buildReservationProofMainUtxo(
	anchorUtxo *bitcoin.UnspentTransactionOutput,
) *tbtc.BitcoinTxUTXO {
	var (
		txHash     [32]byte
		txOutIndex uint32
		txOutValue uint64
	)

	if anchorUtxo.Outpoint != nil {
		txHash = anchorUtxo.Outpoint.TransactionHash
		txOutIndex = anchorUtxo.Outpoint.OutputIndex
	}
	if anchorUtxo.Value < 0 {
		txOutValue = 0
	} else {
		txOutValue = uint64(anchorUtxo.Value)
	}

	return &tbtc.BitcoinTxUTXO{
		TxHash:        txHash,
		TxOutputIndex: txOutIndex,
		TxOutputValue: txOutValue,
	}
}

// getUnprovenReservationReanchorTransactions discovers reservation
// re-anchor Bitcoin transactions that have not yet had their SPV proof
// accepted by the Bridge. It walks ReservationReanchorRequested events in
// the look-back window, skips any whose action generation has already
// settled or timed out, and for the remainder scans the target wallet's
// recent transactions for the one-input-one-output re-anchor transaction
// whose spent input is still registered on-chain as that reservation's
// anchor outpoint.
func getUnprovenReservationReanchorTransactions(
	historyDepth uint64,
	transactionLimit int,
	btcChain bitcoin.Chain,
	spvChain Chain,
) ([]*bitcoin.Transaction, error) {
	blockCounter, err := spvChain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: [%v]", err)
	}

	// Calculate the starting block of the range in which the events will be
	// searched for.
	startBlock := currentBlock - historyDepth

	events, err := spvChain.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{
			StartBlock: startBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past reservation re-anchor requested events: [%v]",
			err,
		)
	}

	unprovenReservationReanchorTransactions := []*bitcoin.Transaction{}

	for _, event := range events {
		action, err := spvChain.GetReservationAction(
			event.ReservationKey,
			event.RequestNonce,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get reservation action generation: [%v]",
				err,
			)
		}

		if action.State != tbtc.ReservationActionStatePending {
			// The action generation already settled (proof accepted) or
			// timed out; there is nothing left to prove for this event.
			continue
		}

		// The re-anchor transaction pays the target wallet, not the source
		// wallet: none of the transaction's outputs transfer funds back to
		// the source wallet, so searching the source wallet's transaction
		// history would never find it. Mirrors the same reasoning
		// getUnprovenMovingFundsTransactions applies for its target wallets.
		walletTransactions, err := btcChain.GetTransactionsForPublicKeyHash(
			event.TargetWalletPublicKeyHash,
			transactionLimit,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get transactions for target wallet: [%v]",
				err,
			)
		}

		for _, transaction := range walletTransactions {
			isUnproven, err := isUnprovenReservationReanchorTransaction(
				transaction,
				event.ReservationKey,
				btcChain,
				spvChain,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to check if transaction is an unproven "+
						"reservation re-anchor transaction: [%v]",
					err,
				)
			}

			if isUnproven {
				unprovenReservationReanchorTransactions = append(
					unprovenReservationReanchorTransactions,
					transaction,
				)
			}
		}
	}

	return unprovenReservationReanchorTransactions, nil
}

// isUnprovenReservationReanchorTransaction reports whether the given
// transaction is the still-unproven re-anchor transaction for the given
// reservation. A transaction qualifies when it has the
// one-input-one-output re-anchor shape and its spent input is still
// registered on-chain as reservationKey's anchor outpoint. The Bridge
// clears that registration only once the re-anchor proof is accepted, so a
// match here is conclusive evidence the proof has not landed yet.
//
// Transactions that do not have the re-anchor shape (e.g. unrelated
// payments to the same wallet) are reported as non-matches rather than
// errors so a single unrelated transaction does not abort the discovery
// round.
func isUnprovenReservationReanchorTransaction(
	transaction *bitcoin.Transaction,
	reservationKey *big.Int,
	btcChain bitcoin.Chain,
	spvChain Chain,
) (bool, error) {
	anchorUtxo, _, err := parseReservationReanchorTransactionInput(
		btcChain,
		transaction,
	)
	if err != nil {
		return false, nil
	}

	matchedReservationKey, err := spvChain.ReservationByAnchorUtxo(
		anchorUtxo.Outpoint.TransactionHash,
		anchorUtxo.Outpoint.OutputIndex,
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to look up reservation by anchor utxo: [%v]",
			err,
		)
	}

	return matchedReservationKey != nil &&
		matchedReservationKey.Sign() != 0 &&
		matchedReservationKey.Cmp(reservationKey) == 0, nil
}

// reservationReanchorTransactionProofSubmitter adapts the reservation
// re-anchor proof submission to the generic transactionProofSubmitter
// signature used by the SPV maintainer's proof loop. It is a thin wrapper
// around submitDiscoveredReservationReanchorProof that plugs in the real
// SPV proof assembler; kept separate so tests can inject a mock assembler
// without needing a real Bitcoin merkle proof chain.
func reservationReanchorTransactionProofSubmitter(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
) error {
	return submitDiscoveredReservationReanchorProof(
		transactionHash,
		requiredConfirmations,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
	)
}

// submitDiscoveredReservationReanchorProof re-derives the
// (reservationKey, requestNonce) pair a discovered re-anchor transaction
// belongs to and submits its SPV proof. The generic transactionProofSubmitter
// signature only carries the transaction hash, so this function looks up
// which reservation is still registered against the transaction's spent
// anchor outpoint (ReservationByAnchorUtxo - the Bridge only clears that
// registration once the proof is accepted, so a match on the outpoint is
// conclusive that this reservation's re-anchor has not yet landed).
//
// Deriving the request nonce is not as simple as reading the reservation's
// current RequestNonce: that field tracks the reservation's live action
// generation, which can have moved on since this transaction was discovered
// (e.g. the original re-anchor action timed out and a new action generation,
// possibly with a different target wallet, was requested while this
// function's caller was waiting out requiredConfirmations). Submitting the
// live nonce for a stale transaction would pair an old, unrelated re-anchor
// transaction with the wrong action generation. To guard against that, this
// function fetches the action generation at the reservation's current nonce
// and requires it to still be a Pending Reanchor action targeting the exact
// wallet this transaction actually pays before treating the current nonce as
// correct for this transaction; any mismatch is reported as an error so the
// proof loop treats the transaction as not-yet-submittable rather than
// silently misattributing the proof.
func submitDiscoveredReservationReanchorProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
	spvProofAssembler spvProofAssembler,
) error {
	transaction, err := btcChain.GetTransaction(transactionHash)
	if err != nil {
		return fmt.Errorf(
			"failed to get reservation re-anchor transaction: [%v]",
			err,
		)
	}

	anchorUtxo, targetWalletPublicKeyHash, err :=
		parseReservationReanchorTransactionInput(btcChain, transaction)
	if err != nil {
		return fmt.Errorf(
			"failed to parse reservation re-anchor transaction input: [%v]",
			err,
		)
	}

	reservationKey, err := spvChain.ReservationByAnchorUtxo(
		anchorUtxo.Outpoint.TransactionHash,
		anchorUtxo.Outpoint.OutputIndex,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to look up reservation by anchor utxo: [%v]",
			err,
		)
	}

	if reservationKey == nil || reservationKey.Sign() == 0 {
		return fmt.Errorf(
			"no reservation is anchored at the spent outpoint of "+
				"transaction [%s]",
			transactionHash.Hex(bitcoin.ReversedByteOrder),
		)
	}

	reservation, err := spvChain.GetReservation(reservationKey)
	if err != nil {
		return fmt.Errorf("failed to get reservation: [%v]", err)
	}

	action, err := spvChain.GetReservationAction(
		reservationKey,
		reservation.RequestNonce,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get reservation's current action generation: [%v]",
			err,
		)
	}

	if action.ActionType != tbtc.ReservationActionTypeReanchor ||
		action.State != tbtc.ReservationActionStatePending {
		return fmt.Errorf(
			"reservation [%v]'s current action generation [%d] is no "+
				"longer a pending re-anchor; the discovered transaction "+
				"[%s] belongs to a superseded generation",
			reservationKey,
			reservation.RequestNonce,
			transactionHash.Hex(bitcoin.ReversedByteOrder),
		)
	}

	if action.TargetWalletPublicKeyHash != targetWalletPublicKeyHash {
		return fmt.Errorf(
			"reservation [%v]'s current action generation [%d] targets a "+
				"different wallet than the discovered transaction [%s]; "+
				"the transaction belongs to a superseded generation",
			reservationKey,
			reservation.RequestNonce,
			transactionHash.Hex(bitcoin.ReversedByteOrder),
		)
	}

	return submitReservationReanchorProof(
		transactionHash,
		requiredConfirmations,
		reservationKey,
		reservation.RequestNonce,
		btcChain,
		spvChain,
		spvProofAssembler,
	)
}

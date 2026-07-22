package spv

import (
	"bytes"
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// SubmitMovingFundsProof prepares moving funds proof for the given
// transaction and submits it to the on-chain contract. If the number of
// required confirmations is `0`, an error is returned.
func SubmitMovingFundsProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
) error {
	return submitMovingFundsProof(
		transactionHash,
		requiredConfirmations,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
	)
}

func submitMovingFundsProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
	spvProofAssembler spvProofAssembler,
) error {
	if requiredConfirmations == 0 {
		return fmt.Errorf(
			"provided required confirmations count must be greater than 0",
		)
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

	mainUTXO, walletPublicKeyHash, err := parseMovingFundsTransactionInput(
		btcChain,
		transaction,
	)
	if err != nil {
		return fmt.Errorf(
			"error while parsing transaction inputs: [%v]",
			err,
		)
	}

	if err := spvChain.SubmitMovingFundsProofWithReimbursement(
		transaction,
		proof,
		mainUTXO,
		walletPublicKeyHash,
	); err != nil {
		return fmt.Errorf(
			"failed to submit moving funds proof with reimbursement: [%v]",
			err,
		)
	}

	return nil
}

// parseMovingFundsTransactionInput parses the transaction's input and
// returns the main UTXO and the wallet public key hash.
func parseMovingFundsTransactionInput(
	btcChain bitcoin.Chain,
	transaction *bitcoin.Transaction,
) (bitcoin.UnspentTransactionOutput, [20]byte, error) {
	// Perform a sanity check: a moving funds transaction must have exactly one
	// input.
	if len(transaction.Inputs) != 1 {
		return bitcoin.UnspentTransactionOutput{}, [20]byte{}, fmt.Errorf(
			"moving funds transaction has more than one input",
		)
	}

	input := transaction.Inputs[0]

	// Get data of the input transaction whose output is spent by the moving
	// funds transaction.
	inputTx, err := btcChain.GetTransaction(input.Outpoint.TransactionHash)
	if err != nil {
		return bitcoin.UnspentTransactionOutput{}, [20]byte{}, fmt.Errorf(
			"cannot get input transaction data: [%v]",
			err,
		)
	}

	// Get the specific output spent by the moving funds transaction.
	spentOutput := inputTx.Outputs[input.Outpoint.OutputIndex]

	// Build the main UTXO object based on available data.
	mainUtxo := bitcoin.UnspentTransactionOutput{
		Outpoint: input.Outpoint,
		Value:    spentOutput.Value,
	}

	// Extract the wallet public key hash from script
	walletPublicKeyHash, err := bitcoin.ExtractPublicKeyHash(spentOutput.PublicKeyScript)
	if err != nil {
		return bitcoin.UnspentTransactionOutput{}, [20]byte{}, fmt.Errorf(
			"cannot extract wallet public key hash: [%v]",
			err,
		)
	}

	return mainUtxo, walletPublicKeyHash, nil
}

func getUnprovenMovingFundsTransactions(
	historyDepth uint64,
	transactionLimit int,
	btcChain bitcoin.Chain,
	spvChain Chain,
) (
	[]*bitcoin.Transaction,
	error,
) {
	startBlock, err := unprovenSearchStartBlock(historyDepth, spvChain)
	if err != nil {
		return nil, err
	}

	// The `MovingFundsCommitmentSubmitted` event can only be emitted once for
	// a given wallet. Therefore there will always be only one event for a wallet.
	// We do not have to worry about duplicate events for the same wallet.
	events, err :=
		spvChain.PastMovingFundsCommitmentSubmittedEvents(
			&tbtc.MovingFundsCommitmentSubmittedEventFilter{
				StartBlock: startBlock,
			},
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past moving funds commitment submitted events: [%v]",
			err,
		)
	}

	unprovenMovingFundsTransactions := []*bitcoin.Transaction{}

	for _, event := range events {
		walletPublicKeyHash := event.WalletPublicKeyHash
		targetWallets := event.TargetWallets

		wallet, err := spvChain.GetWallet(walletPublicKeyHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get wallet: [%v]", err)
		}

		if wallet.State != tbtc.StateMovingFunds {
			// The wallet can only submit moving funds proofs if it's in
			//`MovingFunds` state. If the state is different skip it.
			logger.Infof(
				"skipped proving moving funds transactions for wallet [%x] "+
					"because of wallet state [%v]",
				walletPublicKeyHash,
				wallet.State,
			)
			continue
		}

		// The moving funds transaction should be among the recent transactions
		// that pay to any of the target wallets. When retrieving the recent
		// transactions paying for the public key hash, we can use just one
		// target wallet. Since the smart contract guarantees there is at least
		// one target wallet, we can choose the one at index `0`. Notice that
		// unlike in case of deposit sweep or redemptions, we cannot use the
		// source wallet public key hash when retrieving transactions. This is
		// because none of the transaction's outputs transfers funds to the
		// source wallet.
		targetWalletPublicKeyHash := targetWallets[0]

		unproven, err := collectUnprovenWalletTransactions(
			targetWalletPublicKeyHash,
			transactionLimit,
			btcChain,
			func(transaction *bitcoin.Transaction) (bool, error) {
				return isUnprovenMovingFundsTransaction(
					transaction,
					walletPublicKeyHash,
					targetWallets,
					btcChain,
					spvChain,
				)
			},
			false,
		)
		if err != nil {
			return nil, err
		}

		unprovenMovingFundsTransactions = append(
			unprovenMovingFundsTransactions,
			unproven...,
		)
	}

	return unprovenMovingFundsTransactions, nil
}

func isUnprovenMovingFundsTransaction(
	transaction *bitcoin.Transaction,
	walletPublicKeyHash [20]byte,
	targetWalletsPublicKeyHashes [][20]byte,
	btcChain bitcoin.Chain,
	spvChain Chain,
) (bool, error) {
	// If the transaction does not have exactly one input, it cannot be a
	// moving funds transaction.
	if len(transaction.Inputs) != 1 {
		return false, nil
	}

	fundingTransactionHash := transaction.Inputs[0].Outpoint.TransactionHash
	fundingOutpointIndex := transaction.Inputs[0].Outpoint.OutputIndex

	// The input must refer to the current wallet's main UTXO.
	isMainUtxo, err := isInputCurrentWalletsMainUTXO(
		fundingTransactionHash,
		fundingOutpointIndex,
		walletPublicKeyHash,
		btcChain,
		spvChain,
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to check if input is the main UTXO: [%v]",
			err,
		)
	}

	// The input is not the current main UTXO of the wallet.
	// The transaction cannot be an unproven moving funds transaction.
	if !isMainUtxo {
		return false, nil
	}

	// If the number of transaction's outputs does not match the number of
	// target wallets from the commitment, the transaction cannot be a
	// moving funds transaction.
	if len(transaction.Outputs) != len(targetWalletsPublicKeyHashes) {
		return false, nil
	}

	// Look at the transaction outputs. If the transaction is a moving funds
	// transaction, each output must be transferring funds to the corresponding
	// target wallets from the commitment.
	for outputIndex, output := range transaction.Outputs {
		// Assume that the outputs of moving funds transaction were added
		// in the same order as target wallets on the list.
		targetWalletPublicKeyHash := targetWalletsPublicKeyHashes[outputIndex]

		p2pkh, err := bitcoin.PayToPublicKeyHash(targetWalletPublicKeyHash)
		if err != nil {
			return false, fmt.Errorf(
				"failed to compute p2pkh script for transaction output: [%v]",
				err,
			)
		}
		p2wpkh, err := bitcoin.PayToWitnessPublicKeyHash(targetWalletPublicKeyHash)
		if err != nil {
			return false, fmt.Errorf(
				"failed to compute p2wpkh script for transaction output: [%v]",
				err,
			)
		}

		if !bytes.Equal(output.PublicKeyScript, p2pkh) &&
			!bytes.Equal(output.PublicKeyScript, p2wpkh) {
			return false, nil
		}
	}

	return true, nil
}

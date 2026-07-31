package spv

import (
	"fmt"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
)

// SubmitDepositSweepProof prepares deposit sweep proof for the given
// transaction and submits it to the on-chain contract. If the number of required
// confirmations is `0`, an error is returned.
func SubmitDepositSweepProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
) error {
	return submitDepositSweepProof(
		transactionHash,
		requiredConfirmations,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
		getGlobalMetricsRecorder(),
	)
}

func submitDepositSweepProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
	spvProofAssembler spvProofAssembler,
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
	},
) error {
	// Record proof submission attempt
	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter("deposit_sweep_proof_submissions_total", 1)
	}

	if requiredConfirmations == 0 {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter("deposit_sweep_proof_submissions_failed_total", 1)
		}
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
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter("deposit_sweep_proof_submissions_failed_total", 1)
		}
		return fmt.Errorf(
			"failed to assemble transaction spv proof: [%v]",
			err,
		)
	}

	mainUTXO, vault, err := parseDepositSweepTransactionInputs(
		btcChain,
		spvChain,
		transaction,
	)
	if err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter("deposit_sweep_proof_submissions_failed_total", 1)
		}
		return fmt.Errorf(
			"error while parsing transaction inputs: [%v]",
			err,
		)
	}

	if err := spvChain.SubmitDepositSweepProofWithReimbursement(
		transaction,
		proof,
		mainUTXO,
		vault,
	); err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter("deposit_sweep_proof_submissions_failed_total", 1)
		}
		return fmt.Errorf(
			"failed to submit deposit sweep proof with reimbursement: [%v]",
			err,
		)
	}

	// Record successful proof submission
	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter("deposit_sweep_proof_submissions_success_total", 1)
	}

	return nil
}

// parseDepositSweepTransactionInputs parses the transaction's inputs and
// returns the main UTXO and the vault.
func parseDepositSweepTransactionInputs(
	btcChain bitcoin.Chain,
	spvChain Chain,
	transaction *bitcoin.Transaction,
) (
	bitcoin.UnspentTransactionOutput,
	common.Address,
	error,
) {
	// Represents the main UTXO of the deposit sweep transaction. Nil if there
	// was no main UTXO.
	var mainUTXO *bitcoin.UnspentTransactionOutput = nil

	// Stores the vault address of the deposits. Each deposit should have the
	// same value of vault. The zero-filled value indicates there was no vault
	// value set for the deposits.
	var vault = common.Address{}

	// This flag checks if at least one deposit input has been found during
	// deposit processing.
	var depositAlreadyProcessed = false

	// Perform a sanity check: a deposit sweep transaction must have exactly one
	// output.
	if len(transaction.Outputs) != 1 {
		return bitcoin.UnspentTransactionOutput{}, common.Address{}, fmt.Errorf(
			"deposit sweep transaction has more than one output",
		)
	}

	for _, input := range transaction.Inputs {
		outpointTransactionHash := input.Outpoint.TransactionHash
		outpointIndex := input.Outpoint.OutputIndex

		previousTransaction, err := btcChain.GetTransaction(
			outpointTransactionHash,
		)
		if err != nil {
			return bitcoin.UnspentTransactionOutput{}, common.Address{}, fmt.Errorf(
				"failed to get previous transaction: [%v]",
				err,
			)
		}

		publicKeyScript := previousTransaction.Outputs[outpointIndex].PublicKeyScript
		value := previousTransaction.Outputs[outpointIndex].Value
		scriptClass := txscript.GetScriptClass(publicKeyScript)

		if scriptClass == txscript.PubKeyHashTy ||
			scriptClass == txscript.WitnessV0PubKeyHashTy {
			// The input is P2PKH or P2WPKH, so we found main UTXO. There should
			// be at most one main UTXO. If any input of this kind has already
			// been found, report an error.
			if mainUTXO == nil {
				mainUTXO = &bitcoin.UnspentTransactionOutput{
					Outpoint: &bitcoin.TransactionOutpoint{
						TransactionHash: outpointTransactionHash,
						OutputIndex:     outpointIndex,
					},
					Value: value,
				}
			} else {
				return bitcoin.UnspentTransactionOutput{}, common.Address{}, fmt.Errorf(
					"deposit sweep transaction has more than one non-deposit " +
						"inputs",
				)
			}
		} else if scriptClass == txscript.ScriptHashTy ||
			scriptClass == txscript.WitnessV0ScriptHashTy {
			// The input is P2SH or P2WSH, so we found a deposit input. All
			// the deposits should have the same vault set or no vault at all.
			// If the vault if different than the vault from any previous
			// deposit input, report an error.
			deposit, found, err := spvChain.GetDepositRequest(
				outpointTransactionHash,
				outpointIndex,
			)
			if err != nil {
				return bitcoin.UnspentTransactionOutput{}, common.Address{}, fmt.Errorf(
					"failed to get deposit request: [%v]",
					err,
				)
			}

			if !found {
				return bitcoin.UnspentTransactionOutput{}, common.Address{}, fmt.Errorf(
					"deposit: [%v/%v] not found",
					outpointTransactionHash,
					outpointIndex,
				)
			}

			if depositAlreadyProcessed {
				if vault != convertVaultAddress(deposit.Vault) {
					return bitcoin.UnspentTransactionOutput{}, common.Address{}, fmt.Errorf(
						"swept deposits have different vaults",
					)
				}
			} else {
				// The first deposit input has been encountered, save the vault
				// and require subsequent deposit inputs to have the same vault.
				vault = convertVaultAddress(deposit.Vault)
				depositAlreadyProcessed = true
			}
		} else {
			// The type of the input is neither P2PKH, P2WPKH, P2SH or P2WSH.
			// Report an error.
			return bitcoin.UnspentTransactionOutput{}, common.Address{}, fmt.Errorf(
				"deposit sweep transaction has incorrect input types",
			)
		}
	}

	// If none of the input was main UTXO, return zero-filled main UTXO.
	if mainUTXO == nil {
		mainUTXO = &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: bitcoin.Hash{},
				OutputIndex:     0,
			},
			Value: 0,
		}
	}

	return *mainUTXO, vault, nil
}

// convertVaultAddress converts the vault's address from the chain's address
// to the common address. If the chain's address is nil, the returned value
// is zero-filled.
func convertVaultAddress(vault *chain.Address) common.Address {
	if vault == nil {
		return common.Address{}
	}

	return common.HexToAddress(string(*vault))
}

func getUnprovenDepositSweepTransactions(
	historyDepth uint64,
	transactionLimit int,
	transactionScanLimit int,
	scanner *walletTransactionScanner,
	btcChain bitcoin.Chain,
	spvChain Chain,
) (
	[]*bitcoin.Transaction,
	error,
) {
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

	events, err :=
		spvChain.PastDepositRevealedEvents(
			&tbtc.DepositRevealedEventFilter{
				StartBlock: startBlock,
			},
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past deposit revealed events: [%v]",
			err,
		)
	}

	// There will often be multiple events emitted for a single wallet. Prepare
	// a list of unique wallet public key hashes.
	walletPublicKeyHashes := uniqueWalletPublicKeyHashes(
		events,
	)

	unprovenDepositSweepTransactions := []*bitcoin.Transaction{}

	for _, walletPublicKeyHash := range walletPublicKeyHashes {
		wallet, err := spvChain.GetWallet(walletPublicKeyHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get wallet: [%v]", err)
		}

		if wallet.State != tbtc.StateLive &&
			wallet.State != tbtc.StateMovingFunds {
			// The wallet can only submit deposit sweep proofs if it's `Live` or
			// `MovingFunds`. If the state is different skip it.
			logger.Infof(
				"skipped proving deposit sweep transactions for wallet [%x] "+
					"because of wallet state [%v]",
				walletPublicKeyHash,
				wallet.State,
			)
			continue
		}

		walletUnprovenTransactions, err := scanner.getUnprovenWalletTransactions(
			walletTransactionScanKey{
				actionType:      tbtc.ActionDepositSweep,
				actorWalletPKH:  walletPublicKeyHash,
				lookupWalletPKH: walletPublicKeyHash,
			},
			walletPublicKeyHash,
			transactionLimit,
			transactionScanLimit,
			"deposit sweep",
			btcChain,
			func(transaction *bitcoin.Transaction) (bool, error) {
				return isUnprovenDepositSweepTransaction(
					transaction,
					walletPublicKeyHash,
					btcChain,
					spvChain,
				)
			},
		)
		if err != nil {
			return nil, err
		}

		unprovenDepositSweepTransactions = append(
			unprovenDepositSweepTransactions,
			walletUnprovenTransactions...,
		)
	}

	return unprovenDepositSweepTransactions, nil
}

func isUnprovenDepositSweepTransaction(
	transaction *bitcoin.Transaction,
	walletPublicKeyHash [20]byte,
	btcChain bitcoin.Chain,
	spvChain Chain,
) (bool, error) {
	// If the transaction does not have exactly one output, it cannot be a
	// deposit sweep transaction.
	if len(transaction.Outputs) != 1 {
		return false, nil
	}

	hasDepositInputs := false

	// Look at the transaction's inputs. All the inputs must be deposit inputs,
	// except for one input which can be the main UTXO.
	for _, input := range transaction.Inputs {
		fundingTransactionHash := input.Outpoint.TransactionHash
		fundingOutpointIndex := input.Outpoint.OutputIndex

		// Check if the input is a deposit input.
		deposit, found, err := spvChain.GetDepositRequest(
			fundingTransactionHash,
			fundingOutpointIndex,
		)
		if err != nil {
			return false, fmt.Errorf("failed to get deposit request: [%v]", err)
		}

		if !found {
			// The input is not a deposit input. The transaction can still be
			// a deposit sweep transaction, since the input may be the main UTXO.

			// Check if the input represents the current main UTXO of the wallet.
			// Notice that we don't have to verify if there is only one main
			// UTXO among the transaction's inputs since only one input may have
			// such a structure that the calculated hash will match the wallet's
			// main UTXO hash stored on-chain.
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

			// The input is not the current main UTXO of the wallet. The
			// transaction is either a deposit sweep transaction that is already
			// proven or it's not a deposit sweep transaction at all.
			if !isMainUtxo {
				return false, nil
			}

			// The input is the current main UTXO of the wallet. Proceed with
			// checking other inputs.
		} else {
			// The input is a deposit input. Check if it swept or not.
			if deposit.SweptAt.Equal(time.Unix(0, 0)) {
				// The input is a deposit and it's unswept.
				hasDepositInputs = true
			} else {
				// The input is a deposit, but it's already swept.
				// The transaction must a deposit sweep transaction, but it's
				// already proven.
				return false, nil
			}
		}
	}

	// All the inputs represent either unswept deposits or the current main UTXO.
	// As the final check verify if at least one of them was a deposit input.
	// This will distinguish a deposit sweep transaction from a different
	// transaction type that may have the main UTXO as input, e.g. redemption.
	return hasDepositInputs, nil
}

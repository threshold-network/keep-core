package spv

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/maintainer/btcdiff"
)

var logger = log.Logger("keep-maintainer-spv")

// The length of the Bitcoin difficulty epoch in blocks.
const difficultyEpochLength = 2016

// MetricsRecorder records counter metrics for SPV proof submissions. It is
// satisfied by *clientinfo.PerformanceMetrics. A nil MetricsRecorder is a valid
// argument that disables metrics recording: callers must treat nil as "metrics
// off" and guard every invocation against it.
type MetricsRecorder interface {
	IncrementCounter(name string, value float64)
}

func Initialize(
	ctx context.Context,
	config Config,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
	btcChain bitcoin.Chain,
	metricsRecorder MetricsRecorder,
) {
	spvMaintainer := &spvMaintainer{
		config:          config,
		spvChain:        spvChain,
		btcDiffChain:    btcDiffChain,
		btcChain:        btcChain,
		metricsRecorder: metricsRecorder,
	}

	go spvMaintainer.startControlLoop(ctx)
}

// proofTypes holds the information about proof types supported by the
// SPV maintainer.
var proofTypes = map[tbtc.WalletActionType]struct {
	unprovenTransactionsGetter unprovenTransactionsGetter
	transactionProofSubmitter  transactionProofSubmitter
}{
	tbtc.ActionDepositSweep: {
		unprovenTransactionsGetter: getUnprovenDepositSweepTransactions,
		transactionProofSubmitter:  SubmitDepositSweepProof,
	},
	tbtc.ActionRedemption: {
		unprovenTransactionsGetter: getUnprovenRedemptionTransactions,
		transactionProofSubmitter:  SubmitRedemptionProof,
	},
	tbtc.ActionMovingFunds: {
		unprovenTransactionsGetter: getUnprovenMovingFundsTransactions,
		transactionProofSubmitter:  SubmitMovingFundsProof,
	},
	tbtc.ActionMovedFundsSweep: {
		unprovenTransactionsGetter: getUnprovenMovedFundsSweepTransactions,
		transactionProofSubmitter:  SubmitMovedFundsSweepProof,
	},
}

type spvMaintainer struct {
	config          Config
	spvChain        Chain
	btcDiffChain    btcdiff.Chain
	btcChain        bitcoin.Chain
	metricsRecorder MetricsRecorder
}

func (sm *spvMaintainer) startControlLoop(ctx context.Context) {
	logger.Info("starting SPV maintainer")

	defer func() {
		logger.Info("stopping SPV maintainer")
	}()

	for {
		err := sm.maintainSpv(ctx)
		if err != nil {
			logger.Errorf(
				"error while maintaining SPV: [%v]; restarting maintainer",
				err,
			)
		}

		select {
		case <-time.After(sm.config.RestartBackoffTime):
		case <-ctx.Done():
			return
		}
	}
}

func (sm *spvMaintainer) maintainSpv(ctx context.Context) error {
	for {
		for action, v := range proofTypes {
			logger.Infof("starting [%s] proof task execution...", action)

			if err := sm.proveTransactions(
				v.unprovenTransactionsGetter,
				v.transactionProofSubmitter,
			); err != nil {
				return fmt.Errorf(
					"error while proving [%s] transactions: [%v]",
					action,
					err,
				)
			}

			logger.Infof("[%s] proof task completed", action)
		}

		logger.Infof(
			"proof tasks completed; next run in [%s]",
			sm.config.IdleBackoffTime,
		)

		select {
		case <-time.After(sm.config.IdleBackoffTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// unprovenTransactionsGetter is a type representing a function that is
// used to get unproven Bitcoin transactions.
type unprovenTransactionsGetter func(
	historyDepth uint64,
	transactionLimit int,
	btcChain bitcoin.Chain,
	spvChain Chain,
) (
	[]*bitcoin.Transaction,
	error,
)

// transactionProofSubmitter is a type representing a function that is used
// to submit the constructed SPV proof to the host chain.
type transactionProofSubmitter func(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
	metricsRecorder MetricsRecorder,
) error

// proveTransactions gets unproven Bitcoin transactions using the provided
// unprovenTransactionsGetter, build the SPV proofs, and submits them using
// the provided transactionProofSubmitter.
func (sm *spvMaintainer) proveTransactions(
	unprovenTransactionsGetter unprovenTransactionsGetter,
	transactionProofSubmitter transactionProofSubmitter,
) error {
	transactions, err := unprovenTransactionsGetter(
		sm.config.HistoryDepth,
		sm.config.TransactionLimit,
		sm.btcChain,
		sm.spvChain,
	)
	if err != nil {
		return fmt.Errorf("failed to get unproven transactions: [%v]", err)
	}

	logger.Infof("found [%d] unproven transaction(s)", len(transactions))

	for _, transaction := range transactions {
		// Print the transaction in the same endianness as block explorers do.
		transactionHashStr := transaction.Hash().Hex(bitcoin.ReversedByteOrder)

		logger.Infof(
			"proceeding with proof for transaction [%s]",
			transactionHashStr,
		)

		isProofWithinRelayRange, accumulatedConfirmations, requiredConfirmations, err := getProofInfo(
			transaction.Hash(),
			sm.btcChain,
			sm.spvChain,
			sm.btcDiffChain,
		)
		if err != nil {
			return fmt.Errorf("failed to get proof info: [%v]", err)
		}

		if !isProofWithinRelayRange {
			// The required proof goes outside the previous and current
			// difficulty epochs as seen by the relay. Skip the transaction. It
			// will most likely be proven later.
			logger.Warnf(
				"skipped proving transaction [%s]; the range "+
					"of the required proof goes outside the previous and "+
					"current difficulty epochs as seen by the relay",
				transactionHashStr,
			)
			continue
		}

		if accumulatedConfirmations < requiredConfirmations {
			// Skip the transaction as it has not accumulated enough
			// confirmations. It will be proven later.
			logger.Infof(
				"skipped proving transaction [%s]; transaction "+
					"has [%v/%v] confirmations",
				transactionHashStr,
				accumulatedConfirmations,
				requiredConfirmations,
			)
			continue
		}

		err = transactionProofSubmitter(
			transaction.Hash(),
			requiredConfirmations,
			sm.btcChain,
			sm.spvChain,
			sm.metricsRecorder,
		)
		if err != nil {
			return err
		}

		logger.Infof(
			"successfully submitted proof for transaction [%s]",
			transactionHashStr,
		)
	}

	logger.Infof("finished round of proving transactions")

	return nil
}

func isInputCurrentWalletsMainUTXO(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	walletPublicKeyHash [20]byte,
	btcChain bitcoin.Chain,
	spvChain Chain,
) (bool, error) {
	// Get the transaction the input originated from to calculate the input value.
	previousTransaction, err := btcChain.GetTransaction(fundingTxHash)
	if err != nil {
		return false, fmt.Errorf("failed to get previous transaction: [%v]", err)
	}
	if fundingOutputIndex >= uint32(len(previousTransaction.Outputs)) {
		return false, fmt.Errorf(
			"funding output index [%d] out of range for transaction [%s] "+
				"with [%d] outputs",
			fundingOutputIndex,
			fundingTxHash.String(),
			len(previousTransaction.Outputs),
		)
	}
	fundingOutputValue := previousTransaction.Outputs[fundingOutputIndex].Value

	// Assume the input is the main UTXO and calculate hash.
	mainUtxoHash := spvChain.ComputeMainUtxoHash(&bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: fundingTxHash,
			OutputIndex:     fundingOutputIndex,
		},
		Value: fundingOutputValue,
	})

	// Get the wallet and check if its main UTXO matches the calculated hash.
	wallet, err := spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf("failed to get wallet: [%v]", err)
	}

	return bytes.Equal(mainUtxoHash[:], wallet.MainUtxoHash[:]), nil
}

// getProofInfo returns information about the SPV proof. It includes the
// information whether the transaction proof range is within the previous and
// current difficulty epochs as seen by the relay, the accumulated number of
// confirmations and the required number of confirmations.
func getProofInfo(
	transactionHash bitcoin.Hash,
	btcChain bitcoin.Chain,
	spvChain Chain,
	btcDiffChain btcdiff.Chain,
) (
	bool, uint, uint, error,
) {
	latestBlockHeight, err := btcChain.GetLatestBlockHeight()
	if err != nil {
		return false, 0, 0, fmt.Errorf(
			"failed to get latest block height: [%v]",
			err,
		)
	}

	accumulatedConfirmations, err := btcChain.GetTransactionConfirmations(
		transactionHash,
	)
	if err != nil {
		return false, 0, 0, fmt.Errorf(
			"failed to get transaction confirmations: [%v]",
			err,
		)
	}

	txProofDifficultyFactor, err := spvChain.TxProofDifficultyFactor()
	if err != nil {
		return false, 0, 0, fmt.Errorf(
			"failed to get transaction proof difficulty factor: [%v]",
			err,
		)
	}

	// Calculate the starting block of the proof and the difficulty epoch number
	// it belongs to.
	proofStartBlock := uint64(latestBlockHeight - accumulatedConfirmations + 1)
	proofStartEpoch := proofStartBlock / difficultyEpochLength

	// Calculate the ending block of the proof and the difficulty epoch number
	// it belongs to.
	proofEndBlock := proofStartBlock + txProofDifficultyFactor.Uint64() - 1
	proofEndEpoch := proofEndBlock / difficultyEpochLength

	// Get the current difficulty epoch number as seen by the relay. Subtract
	// one to get the previous epoch number.
	currentEpoch, err := btcDiffChain.CurrentEpoch()
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to get current epoch: [%v]", err)
	}
	previousEpoch := currentEpoch - 1

	// There are only three possible valid combinations of the proof's block
	// headers range: the proof must either be entirely in the previous epoch,
	// must be entirely in the current epoch or must span the previous and
	// current epochs.

	// If the proof is entirely within the current epoch, required confirmations
	// does not need to be adjusted.
	if proofStartEpoch == currentEpoch &&
		proofEndEpoch == currentEpoch {
		return true, accumulatedConfirmations, uint(txProofDifficultyFactor.Uint64()), nil
	}

	// If the proof is entirely within the previous epoch, required confirmations
	// does not need to be adjusted.
	if proofStartEpoch == previousEpoch &&
		proofEndEpoch == previousEpoch {
		return true, accumulatedConfirmations, uint(txProofDifficultyFactor.Uint64()), nil
	}

	// If the proof spans the previous and current difficulty epochs, the
	// required confirmations may have to be adjusted. The reason for this is
	// that there may be a drop in the value of difficulty between the current
	// and the previous epochs. Example:
	// Let's assume the transaction was done near the end of an epoch, so that
	// part of the proof (let's say two block headers) is in the previous epoch
	// and part of it is in the current epoch.
	// If the previous epoch difficulty is 50 and the current epoch difficulty
	// is 30, the total required difficulty of the proof will be transaction
	// difficulty factor times previous difficulty: 6 * 50 = 300.
	// However, if we simply use transaction difficulty factor to get the number
	// of blocks we will end up with the difficulty sum that is too low:
	// 50 + 50 + 30 + 30 + 30 + 30 = 220. To calculate the correct number of
	// block headers needed we need to find how much difficulty needs to come
	// from from the current epoch block headers: 300 - 2*50 = 200 and divide
	// it by the current difficulty: 200 / 30 = 6 and add 1, because there
	// was a remainder. So the number of block headers from the current epoch
	// would be 7. The total number of block headers would be 9 and the sum
	// of their difficulties would be: 50 + 50 + 30 + 30 + 30 + 30 + 30 + 30 +
	// 30 = 310 which is enough to prove the transaction.
	if proofStartEpoch == previousEpoch &&
		proofEndEpoch == currentEpoch {
		currentEpochDifficulty, previousEpochDifficulty, err :=
			btcDiffChain.GetCurrentAndPrevEpochDifficulty()
		if err != nil {
			return false, 0, 0, fmt.Errorf(
				"failed to get Bitcoin epoch difficulties: [%v]",
				err,
			)
		}

		// Calculate the total difficulty that is required for the proof. The
		// proof begins in the previous difficulty epoch, therefore the total
		// required difficulty will be the previous epoch difficulty times
		// transaction proof difficulty factor.
		totalDifficultyRequired := new(big.Int).Mul(
			previousEpochDifficulty,
			txProofDifficultyFactor,
		)

		// Calculate the number of block headers in the proof that will come
		// from the previous difficulty epoch.
		numberOfBlocksPreviousEpoch :=
			uint64(difficultyEpochLength - proofStartBlock%difficultyEpochLength)

		// Calculate how much difficulty the blocks from the previous epoch part
		// of the proof have in total.
		totalDifficultyPreviousEpoch := new(big.Int).Mul(
			big.NewInt(int64(numberOfBlocksPreviousEpoch)),
			previousEpochDifficulty,
		)

		// Calculate how much difficulty must come from the current epoch.
		totalDifficultyCurrentEpoch := new(big.Int).Sub(
			totalDifficultyRequired,
			totalDifficultyPreviousEpoch,
		)

		// Calculate how many blocks from the current epoch we need.
		remainder := new(big.Int)
		numberOfBlocksCurrentEpoch, remainder := new(big.Int).DivMod(
			totalDifficultyCurrentEpoch,
			currentEpochDifficulty,
			remainder,
		)
		// If there is a remainder, it means there is still some amount of
		// difficulty missing that is less than one block difficulty. We need to
		// account for that by adding one additional block.
		if remainder.Cmp(big.NewInt(0)) > 0 {
			numberOfBlocksCurrentEpoch.Add(
				numberOfBlocksCurrentEpoch,
				big.NewInt(1),
			)
		}

		// The total required number of confirmations is the sum of blocks from
		// the previous and current epochs.
		requiredConfirmations := numberOfBlocksPreviousEpoch +
			numberOfBlocksCurrentEpoch.Uint64()

		return true, accumulatedConfirmations, uint(requiredConfirmations), nil
	}

	// If we entered here, it means that the proof's block headers range goes
	// outside the previous or current difficulty epochs as seen by the relay.
	// The reason for this is most likely that transaction entered the Bitcoin
	// blockchain within the very new difficulty epoch that is not yet proven in
	// the relay. In that case the transaction will be proven in the future.
	// The other case could be that the transaction is older than the last two
	// Bitcoin difficulty epochs. In that case the transaction will soon leave
	// the sliding window of recent transactions.
	return false, 0, 0, nil
}

// walletEvent is a type constraint representing wallet-related chain events.
type walletEvent interface {
	GetWalletPublicKeyHash() [20]byte
}

// uniqueWalletPublicKeyHashes parses the list of wallet-related events and
// returns a list of unique wallet public key hashes.
func uniqueWalletPublicKeyHashes[T walletEvent](events []T) [][20]byte {
	cache := make(map[string]struct{})
	var publicKeyHashes [][20]byte

	for _, event := range events {
		key := event.GetWalletPublicKeyHash()
		strKey := hex.EncodeToString(key[:])

		// Check for uniqueness
		if _, exists := cache[strKey]; !exists {
			cache[strKey] = struct{}{}
			publicKeyHashes = append(publicKeyHashes, key)
		}
	}

	return publicKeyHashes
}

// unprovenSearchStartBlock returns the starting block of the range in which
// the events used to find unproven transactions are searched for. It is
// derived from the current chain tip and the configured history depth.
func unprovenSearchStartBlock(
	historyDepth uint64,
	spvChain Chain,
) (uint64, error) {
	blockCounter, err := spvChain.BlockCounter()
	if err != nil {
		return 0, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return 0, fmt.Errorf("failed to get current block: [%v]", err)
	}

	// Guard against unsigned underflow on short chains (e.g. early test
	// networks) where the configured history depth can exceed the current
	// tip; clamp the search start to the genesis block instead of wrapping
	// around to a near-maximum block number.
	if historyDepth > currentBlock {
		return 0, nil
	}

	return currentBlock - historyDepth, nil
}

// collectUnprovenWalletTransactions returns the recent transactions of the
// wallet identified by lookupPublicKeyHash that satisfy the isUnproven
// predicate. When stopAtFirstMatch is true it returns as soon as the first
// matching transaction is found, which is sufficient for wallet operations
// that can have at most one unproven transaction at a time.
func collectUnprovenWalletTransactions(
	lookupPublicKeyHash [20]byte,
	transactionLimit int,
	btcChain bitcoin.Chain,
	isUnproven func(transaction *bitcoin.Transaction) (bool, error),
	stopAtFirstMatch bool,
) ([]*bitcoin.Transaction, error) {
	walletTransactions, err := btcChain.GetTransactionsForPublicKeyHash(
		lookupPublicKeyHash,
		transactionLimit,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transactions for wallet: [%v]",
			err,
		)
	}

	var unprovenTransactions []*bitcoin.Transaction

	for _, transaction := range walletTransactions {
		matched, err := isUnproven(transaction)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to check if transaction is unproven: [%v]",
				err,
			)
		}

		if matched {
			unprovenTransactions = append(unprovenTransactions, transaction)
			if stopAtFirstMatch {
				break
			}
		}
	}

	return unprovenTransactions, nil
}

// spvProofAssembler is a type representing a function that is used
// to assemble an SPV proof for the given transaction hash and confirmations
// count.
type spvProofAssembler func(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
) (*bitcoin.Transaction, *bitcoin.SpvProof, error)

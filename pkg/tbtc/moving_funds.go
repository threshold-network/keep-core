package tbtc

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"go.uber.org/zap"
)

const (
	// movingFundsProposalValidityBlocks determines the moving funds proposal
	// validity time expressed in blocks. In other words, this is the worst-case
	// time for moving funds during which the wallet is busy and cannot take
	// another actions. The value of 650 blocks is roughly 2 hours and 10
	// minutes, assuming 12 seconds per block. It is a slightly longer validity
	// time than in case of redemptions as moving funds involves waiting for
	// target wallets commitment transaction to be confirmed.
	movingFundsProposalValidityBlocks = 650
	// movingFundsSigningTimeoutSafetyMarginBlocks determines the duration of
	// the safety margin that must be preserved between the signing timeout and
	// the timeout of the entire moving funds action. This safety margin
	// prevents against the case where signing completes late and there is not
	// enough time to broadcast the moving funds transaction properly.
	// In such a case, wallet signatures may leak and make the wallet subject
	// of fraud accusations. Usage of the safety margin ensures there is enough
	// time to perform post-signing steps of the moving funds action.
	// The value of 300 blocks is roughly 1 hour, assuming 12 seconds per block.
	movingFundsSigningTimeoutSafetyMarginBlocks = 300
	// movingFundsBroadcastTimeout determines the time window for moving funds
	// transaction broadcast. It is guaranteed that at least
	// movingFundsSigningTimeoutSafetyMarginBlocks is preserved for the broadcast
	// step. However, the happy path for the broadcast step is usually quick
	// and few retries are needed to recover from temporary problems. That
	// said, if the broadcast step does not succeed in a tight timeframe,
	// there is no point to retry for the entire possible time window.
	// Hence, the timeout for broadcast step is set as 25% of the entire
	// time widow determined by movingFundsSigningTimeoutSafetyMarginBlocks.
	movingFundsBroadcastTimeout = 15 * time.Minute
	// movingFundsBroadcastCheckDelay determines the delay that must
	// be preserved between transaction broadcast and the check that ensures
	// the transaction is known on the Bitcoin chain. This delay is needed
	// as spreading the transaction over the Bitcoin network takes time.
	movingFundsBroadcastCheckDelay = 1 * time.Minute
	// movingFundsCommitmentConfirmationBlocks determines the period used when
	// waiting for the moving funds commitment confirmations. This period
	// ensures the moving funds commitment has definitely entered the blockchain
	// and will not be removed by a chain reorganization.
	movingFundsCommitmentConfirmationBlocks = 32
)

// MovingFundsCommitmentLookBackBlocks is the look-back period in blocks used
// when searching for submitted moving funds commitment events. It's equal to
// 30 days assuming 12 seconds per block.
const MovingFundsCommitmentLookBackBlocks = uint64(216000)

// MovingFundsProposal represents a moving funds proposal issued by a wallet's
// coordination leader.
type MovingFundsProposal struct {
	TargetWallets    [][20]byte
	MovingFundsTxFee *big.Int
}

func (mfp *MovingFundsProposal) ActionType() WalletActionType {
	return ActionMovingFunds
}

func (mfp *MovingFundsProposal) ValidityBlocks() uint64 {
	return movingFundsProposalValidityBlocks
}

// movingFundsAction is a walletAction implementation handling moving funds
// requests from the wallet coordinator.
type movingFundsAction struct {
	logger   *zap.SugaredLogger
	chain    Chain
	btcChain bitcoin.Chain

	movingFundsWallet   wallet
	transactionExecutor *walletTransactionExecutor

	proposal                     *MovingFundsProposal
	proposalProcessingStartBlock uint64
	proposalExpiryBlock          uint64

	signingTimeoutSafetyMarginBlocks uint64
	broadcastTimeout                 time.Duration
	broadcastCheckDelay              time.Duration
}

func newMovingFundsAction(
	logger *zap.SugaredLogger,
	chain Chain,
	btcChain bitcoin.Chain,
	movingFundsWallet wallet,
	signingExecutor walletSigningExecutor,
	proposal *MovingFundsProposal,
	proposalProcessingStartBlock uint64,
	proposalExpiryBlock uint64,
	waitForBlockFn waitForBlockFn,
) *movingFundsAction {
	transactionExecutor := newWalletTransactionExecutor(
		btcChain,
		movingFundsWallet,
		signingExecutor,
		waitForBlockFn,
	)
	transactionExecutor.action = ActionMovingFunds

	return &movingFundsAction{
		logger:                           logger,
		chain:                            chain,
		btcChain:                         btcChain,
		movingFundsWallet:                movingFundsWallet,
		transactionExecutor:              transactionExecutor,
		proposal:                         proposal,
		proposalProcessingStartBlock:     proposalProcessingStartBlock,
		proposalExpiryBlock:              proposalExpiryBlock,
		signingTimeoutSafetyMarginBlocks: movingFundsSigningTimeoutSafetyMarginBlocks,
		broadcastTimeout:                 movingFundsBroadcastTimeout,
		broadcastCheckDelay:              movingFundsBroadcastCheckDelay,
	}
}

func (mfa *movingFundsAction) execute() error {
	validateProposalLogger := mfa.logger.With(
		zap.String("step", "validateProposal"),
	)

	walletPublicKeyHash := bitcoin.PublicKeyHash(mfa.wallet().publicKey)

	walletMainUtxo, err := DetermineWalletMainUtxoForPublicKey(
		mfa.wallet().publicKey,
		mfa.chain,
		mfa.btcChain,
	)
	if err != nil {
		return fmt.Errorf(
			"error while determining wallet's main UTXO: [%v]",
			err,
		)
	}

	// Proposal validation should detect this but let's make a check just
	// in case.
	if walletMainUtxo == nil {
		return fmt.Errorf("moving funds wallet has no main UTXO")
	}

	// Perform initial validation of the moving funds proposal.
	err = ValidateMovingFundsProposal(
		validateProposalLogger,
		walletPublicKeyHash,
		walletMainUtxo,
		mfa.proposal,
		mfa.chain,
	)
	if err != nil {
		return fmt.Errorf("validate proposal step failed: [%v]", err)
	}

	// Wait until the moving funds commitment transaction has gathered a
	// significant number of confirmations. This ensures that even a deep reorg
	// will not remove the commitment from the chain.
	err = mfa.waitForCommitmentConfirmation(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf(
			"failed to ensure moving funds transaction confirmed: [%v]",
			err,
		)
	}

	// Validate the moving funds proposal again after waiting for the
	// transaction confirmations. This repeated validation ensures the moving
	// funds commitment was not changed during waiting for the confirmations.
	err = ValidateMovingFundsProposal(
		validateProposalLogger,
		walletPublicKeyHash,
		walletMainUtxo,
		mfa.proposal,
		mfa.chain,
	)
	if err != nil {
		return fmt.Errorf("validate proposal step failed: [%v]", err)
	}

	err = EnsureWalletSyncedBetweenChainsForPublicKey(
		mfa.wallet().publicKey,
		walletMainUtxo,
		mfa.chain,
		mfa.btcChain,
	)
	if err != nil {
		return fmt.Errorf(
			"error while ensuring wallet state is synced between "+
				"BTC and host chain: [%v]",
			err,
		)
	}

	targetWalletOutputScripts, err := walletOutputScriptsForPublicKeyHashes(
		mfa.chain,
		mfa.proposal.TargetWallets,
	)
	if err != nil {
		return fmt.Errorf(
			"error while resolving moving funds target wallet outputs: [%v]",
			err,
		)
	}

	unsignedMovingFundsTx, err := assembleMovingFundsTransaction(
		mfa.btcChain,
		walletMainUtxo,
		targetWalletOutputScripts,
		mfa.proposal.MovingFundsTxFee.Int64(),
	)
	if err != nil {
		return fmt.Errorf(
			"error while assembling moving funds transaction: [%v]",
			err,
		)
	}

	signTxLogger := mfa.logger.With(
		zap.String("step", "signTransaction"),
	)

	// Just in case. This should never happen.
	if mfa.proposalExpiryBlock < mfa.signingTimeoutSafetyMarginBlocks {
		return fmt.Errorf("invalid proposal expiry block")
	}

	mfa.transactionExecutor.frostPreSignActionContext = &FrostPreSignActionContext{
		MovingFunds: &FrostPreSignMovingFundsActionContext{
			Proposal: mfa.proposal,
			MainUtxo: walletMainUtxo,
		},
	}
	movingFundsTx, err := mfa.transactionExecutor.signTransaction(
		signTxLogger,
		unsignedMovingFundsTx,
		mfa.proposalProcessingStartBlock+movingFundsCommitmentConfirmationBlocks,
		mfa.proposalExpiryBlock-mfa.signingTimeoutSafetyMarginBlocks,
	)
	if err != nil {
		return fmt.Errorf("sign transaction step failed: [%v]", err)
	}

	broadcastTxLogger := mfa.logger.With(
		zap.String("step", "broadcastTransaction"),
		zap.String(
			"movingFundsTxHash",
			movingFundsTx.Hash().Hex(bitcoin.ReversedByteOrder),
		),
	)

	err = mfa.transactionExecutor.broadcastTransaction(
		broadcastTxLogger,
		movingFundsTx,
		mfa.broadcastTimeout,
		mfa.broadcastCheckDelay,
	)
	if err != nil {
		return fmt.Errorf("broadcast transaction step failed: [%v]", err)
	}

	return nil
}

func (mfa *movingFundsAction) waitForCommitmentConfirmation(
	walletPublicKeyHash [20]byte,
) error {
	blockCounter, err := mfa.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("error getting block counter [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("error getting current block [%w]", err)
	}

	// To verify the commitment transaction is in the Ethereum blockchain check
	// that the commitment hash is not zero.
	stateCheck := func() (bool, error) {
		walletData, err := mfa.chain.GetWallet(walletPublicKeyHash)
		if err != nil {
			return false, err
		}

		return walletData.MovingFundsTargetWalletsCommitmentHash != [32]byte{}, nil
	}

	// Wait a significant number of blocks to make sure the transaction has not
	// been reverted for some reason, e.g. due to a chain reorganization.
	result, err := ethereum.WaitForBlockConfirmations(
		blockCounter,
		currentBlock,
		movingFundsCommitmentConfirmationBlocks,
		stateCheck,
	)
	if err != nil {
		return fmt.Errorf(
			"error while waiting for transaction confirmation [%w]",
			err,
		)
	}

	if !result {
		return fmt.Errorf("transaction not included in blockchain")
	}

	return nil
}

// ValidateMovingFundsProposal checks the moving funds proposal with on-chain
// validation rules.
func ValidateMovingFundsProposal(
	validateProposalLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	mainUTXO *bitcoin.UnspentTransactionOutput,
	proposal *MovingFundsProposal,
	chain interface {
		// ValidateMovingFundsProposal validates the given moving funds proposal
		// against the chain. Returns an error if the proposal is not valid or
		// nil otherwise.
		ValidateMovingFundsProposal(
			walletPublicKeyHash [20]byte,
			mainUTXO *bitcoin.UnspentTransactionOutput,
			proposal *MovingFundsProposal,
		) error

		BlockCounter() (chain.BlockCounter, error)

		GetWallet(walletPublicKeyHash [20]byte) (*WalletChainData, error)

		GetMovingFundsParameters() (
			txMaxTotalFee uint64,
			dustThreshold uint64,
			timeoutResetDelay uint32,
			timeout uint32,
			timeoutSlashingAmount *big.Int,
			timeoutNotifierRewardMultiplier uint32,
			commitmentGasOffset uint16,
			sweepTxMaxTotalFee uint64,
			sweepTimeout uint32,
			sweepTimeoutSlashingAmount *big.Int,
			sweepTimeoutNotifierRewardMultiplier uint32,
			err error,
		)

		PastMovingFundsCommitmentSubmittedEvents(
			filter *MovingFundsCommitmentSubmittedEventFilter,
		) ([]*MovingFundsCommitmentSubmittedEvent, error)
	},
) error {
	validateProposalLogger.Infof("calling chain for proposal validation")

	err := ValidateMovingFundsSafetyMargin(walletPublicKeyHash, chain)
	if err != nil {
		return fmt.Errorf("moving funds proposal is invalid: [%v]", err)
	}

	err = chain.ValidateMovingFundsProposal(
		walletPublicKeyHash,
		mainUTXO,
		proposal,
	)
	if err != nil {
		return fmt.Errorf("moving funds proposal is invalid: [%v]", err)
	}

	validateProposalLogger.Infof("moving funds proposal is valid")

	return nil
}

// ValidateMovingFundsSafetyMargin checks if the moving funds safety margin
// is in force.
//
// Wallets that just entered the MovingFunds state may have received some last
// minute deposits just before. Even though deposit sweep typically occurs
// before moving funds, such deposits may not be mature enough or have enough
// confirmations to be swept yet. MovingFunds wallets cannot receive new
// deposits so, it makes sense to preserve a safety margin before moving
// funds to give the last minute deposits a chance to become eligible for
// deposit sweep.
//
// Similarly, wallets that just entered the MovingFunds state may have become
// target wallets for another moving funds wallets. It makes sense to preserve
// a safety margin to allow the wallet to merge the moved funds from another
// wallets. In this case a longer safety margin should be used.
func ValidateMovingFundsSafetyMargin(
	walletPublicKeyHash [20]byte,
	chain interface {
		BlockCounter() (chain.BlockCounter, error)

		GetWallet(walletPublicKeyHash [20]byte) (*WalletChainData, error)

		GetMovingFundsParameters() (
			txMaxTotalFee uint64,
			dustThreshold uint64,
			timeoutResetDelay uint32,
			timeout uint32,
			timeoutSlashingAmount *big.Int,
			timeoutNotifierRewardMultiplier uint32,
			commitmentGasOffset uint16,
			sweepTxMaxTotalFee uint64,
			sweepTimeout uint32,
			sweepTimeoutSlashingAmount *big.Int,
			sweepTimeoutNotifierRewardMultiplier uint32,
			err error,
		)

		PastMovingFundsCommitmentSubmittedEvents(
			filter *MovingFundsCommitmentSubmittedEventFilter,
		) ([]*MovingFundsCommitmentSubmittedEvent, error)
	},
) error {
	// In most cases the safety margin of 24 hours should be enough. It will
	// allow the wallet to sweep the last deposits that were made before the
	// wallet entered the moving funds state.
	safetyMargin := time.Duration(24) * time.Hour

	// It is possible that our wallet is the target wallet in another pending
	// moving funds procedure. If this is the case we must apply a longer
	// 14-day safety margin. This will ensure the funds moved from another
	// wallet can be merged with our wallet's main UTXO before moving funds.
	isMovingFundsTarget, err := isWalletPendingMovingFundsTarget(
		walletPublicKeyHash,
		chain,
	)
	if err != nil {
		return fmt.Errorf(
			"cannot check if wallet is pending moving funds target: [%w]",
			err,
		)
	}

	if isMovingFundsTarget {
		safetyMargin = time.Duration(14*24) * time.Hour
	}

	// As the moving funds procedure is time constrained, we must ensure the
	// safety margin does not exceed half of the moving funds timeout parameter.
	// This should give the wallet enough time to complete moving funds.
	_, _, _, movingFundsTimeout, _, _, _, _, _, _, _, err :=
		chain.GetMovingFundsParameters()
	if err != nil {
		return fmt.Errorf("cannot get moving funds parameters: [%w]", err)
	}

	maxAllowedSafetyMargin := time.Duration(movingFundsTimeout/2) * time.Second

	if safetyMargin > maxAllowedSafetyMargin {
		safetyMargin = maxAllowedSafetyMargin
	}

	walletChainData, err := chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf(
			"cannot get wallet's chain data: [%w]",
			err,
		)
	}

	safetyMarginExpiresAt := walletChainData.MovingFundsRequestedAt.Add(safetyMargin)

	if time.Now().Before(safetyMarginExpiresAt) {
		return fmt.Errorf("safety margin in force")
	}

	return nil
}

func (mfa *movingFundsAction) wallet() wallet {
	return mfa.movingFundsWallet
}

func (mfa *movingFundsAction) actionType() WalletActionType {
	return ActionMovingFunds
}

func isWalletPendingMovingFundsTarget(
	walletPublicKeyHash [20]byte,
	chain interface {
		BlockCounter() (chain.BlockCounter, error)

		GetWallet(walletPublicKeyHash [20]byte) (*WalletChainData, error)

		GetMovingFundsParameters() (
			txMaxTotalFee uint64,
			dustThreshold uint64,
			timeoutResetDelay uint32,
			timeout uint32,
			timeoutSlashingAmount *big.Int,
			timeoutNotifierRewardMultiplier uint32,
			commitmentGasOffset uint16,
			sweepTxMaxTotalFee uint64,
			sweepTimeout uint32,
			sweepTimeoutSlashingAmount *big.Int,
			sweepTimeoutNotifierRewardMultiplier uint32,
			err error,
		)

		PastMovingFundsCommitmentSubmittedEvents(
			filter *MovingFundsCommitmentSubmittedEventFilter,
		) ([]*MovingFundsCommitmentSubmittedEvent, error)
	},
) (bool, error) {
	blockCounter, err := chain.BlockCounter()
	if err != nil {
		return false, fmt.Errorf("failed to get block counter: [%w]", err)
	}

	currentBlockNumber, err := blockCounter.CurrentBlock()
	if err != nil {
		return false, fmt.Errorf(
			"failed to get current block number: [%w]",
			err,
		)
	}

	filterStartBlock := uint64(0)
	if currentBlockNumber > MovingFundsCommitmentLookBackBlocks {
		filterStartBlock = currentBlockNumber - MovingFundsCommitmentLookBackBlocks
	}

	// Get all the recent moving funds commitment submitted events.
	filter := &MovingFundsCommitmentSubmittedEventFilter{
		StartBlock: filterStartBlock,
	}

	events, err := chain.PastMovingFundsCommitmentSubmittedEvents(filter)
	if err != nil {
		return false, fmt.Errorf(
			"failed to get past moving funds commitment submitted events: [%w]",
			err,
		)
	}

	isWalletTarget := func(event *MovingFundsCommitmentSubmittedEvent) bool {
		for _, targetWallet := range event.TargetWallets {
			if walletPublicKeyHash == targetWallet {
				return true
			}
		}
		return false
	}

	for _, event := range events {
		if !isWalletTarget(event) {
			continue
		}

		// Our wallet is on the list of target wallets. If the state of the
		// source wallet is still MovingFunds, the moving funds process
		// targeting our wallet is likely in progress.
		walletChainData, err := chain.GetWallet(event.WalletPublicKeyHash)
		if err != nil {
			return false, fmt.Errorf(
				"cannot get wallet's chain data: [%w]",
				err,
			)
		}

		if walletChainData.State == StateMovingFunds {
			return true, nil
		}
	}

	return false, nil
}

func assembleMovingFundsTransaction(
	bitcoinChain bitcoin.Chain,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	targetWalletOutputScripts []bitcoin.Script,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if len(targetWalletOutputScripts) < 1 {
		return nil, fmt.Errorf("at least one target wallet is required")
	}

	if walletMainUtxo == nil {
		return nil, fmt.Errorf("wallet main UTXO is required")
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)
	err := addWalletUtxoInput(builder, bitcoinChain, walletMainUtxo)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to wallet main UTXO: [%v]",
			err,
		)
	}

	// The number of target wallets. It determines the number of outputs.
	targetWalletsCount := int64(len(targetWalletOutputScripts))

	// The sum of all the outputs equal to the input value minus fee.
	totalOutputValue := walletMainUtxo.Value - fee

	// The remainder left from distributing the total output value evenly
	// among the outputs. It should be added to the value of the last output.
	// The value of this remainder is at most `len(targetWallets) - 1` satoshis.
	remainder := totalOutputValue % targetWalletsCount

	// The amount of Bitcoin allocated to each output. Additionally, the amount
	// for the last output should be increased by the remainder left from
	// distributing the total output amount across all the outputs.
	singleOutputValue := (totalOutputValue - remainder) / targetWalletsCount

	// Add one output for each target wallets. The order of target wallets
	// should be the same as during the commitment. We don't to check it,
	// as the order of target wallets has already been validated by the on-chain
	// contract.
	for i, targetWalletOutputScript := range targetWalletOutputScripts {
		outputValue := singleOutputValue
		if i == len(targetWalletOutputScripts)-1 {
			// If we are at the last output, increase its value by the remaining
			// satoshis. If the total output amount is divisible by the number
			// of target wallets, the increase will be `0`.
			outputValue += remainder
		}

		builder.AddOutput(&bitcoin.TransactionOutput{
			Value:           outputValue,
			PublicKeyScript: targetWalletOutputScript,
		})
	}

	return builder, nil
}

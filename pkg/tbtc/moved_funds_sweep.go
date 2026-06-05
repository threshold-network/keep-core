package tbtc

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"go.uber.org/zap"

	"github.com/ipfs/go-log/v2"
)

// MovedFundsSweepRequestState represents the state of a moved funds request.
type MovedFundsSweepRequestState uint8

const (
	MovedFundsStateUnknown MovedFundsSweepRequestState = iota
	MovedFundsStatePending
	MovedFundsStateProcessed
	MovedFundsStateTimedOut
)

// MovedFundsSweepRequest represents a moved funds sweep request.
type MovedFundsSweepRequest struct {
	WalletPublicKeyHash [20]byte
	Value               uint64
	CreatedAt           time.Time
	State               MovedFundsSweepRequestState
}

const (
	// movedFundsSweepProposalValidityBlocks determines the moved funds sweep
	// proposal validity time expressed in blocks. In other words, this is the
	// worst-case time for a moved funds sweep during which the wallet is busy
	// and cannot take another actions. The value of 600 blocks is roughly
	// 2 hours, assuming 12 seconds per block.
	movedFundsSweepProposalValidityBlocks = 600
	// movedFundsSweepSigningTimeoutSafetyMarginBlocks determines the duration of
	// the safety margin that must be preserved between the signing timeout and
	// the timeout of the entire moved funds sweep action. This safety margin
	// prevents against the case where signing completes late and there is not
	// enough time to broadcast the moved funds sweep transaction properly.
	// In such a case, wallet signatures may leak and make the wallet subject
	// of fraud accusations. Usage of the safety margin ensures there is enough
	// time to perform post-signing steps of the moved funds sweep action.
	// The value of 300 blocks is roughly 1 hour, assuming 12 seconds per block.
	movedFundsSweepSigningTimeoutSafetyMarginBlocks = 300
	// movedFundsSweepBroadcastTimeout determines the time window for moved
	// funds sweep transaction broadcast. It is guaranteed that at least
	// movedFundsSweepSigningTimeoutSafetyMarginBlocks is preserved for the
	// broadcast step. However, the happy path for the broadcast step is usually
	// quick and few retries are needed to recover from temporary problems. That
	// said, if the broadcast step does not succeed in a tight timeframe, there
	// is no point to retry for the entire possible time window. Hence, the
	// timeout for broadcast step is set as 25% of the entire time widow
	// determined by movedFundsSweepSigningTimeoutSafetyMarginBlocks.
	movedFundsSweepBroadcastTimeout = 15 * time.Minute
	// movedFundsSweepBroadcastCheckDelay determines the delay that must
	// be preserved between transaction broadcast and the check that ensures
	// the transaction is known on the Bitcoin chain. This delay is needed
	// as spreading the transaction over the Bitcoin network takes time.
	movedFundsSweepBroadcastCheckDelay = 1 * time.Minute
)

// MovedFundsSweepProposal represents a moved funds sweep proposal issued by a
// wallet's coordination leader.
type MovedFundsSweepProposal struct {
	MovingFundsTxHash        [32]byte
	MovingFundsTxOutputIndex uint32
	SweepTxFee               *big.Int
}

func (mfsp *MovedFundsSweepProposal) ActionType() WalletActionType {
	return ActionMovedFundsSweep
}

func (mfsp *MovedFundsSweepProposal) ValidityBlocks() uint64 {
	return movedFundsSweepProposalValidityBlocks
}

type movedFundsSweepAction struct {
	logger   *zap.SugaredLogger
	chain    Chain
	btcChain bitcoin.Chain

	movedFundsSweepWallet wallet
	transactionExecutor   *walletTransactionExecutor

	proposal                     *MovedFundsSweepProposal
	proposalProcessingStartBlock uint64
	proposalExpiryBlock          uint64

	signingTimeoutSafetyMarginBlocks uint64
	broadcastTimeout                 time.Duration
	broadcastCheckDelay              time.Duration
}

func newMovedFundsSweepAction(
	logger *zap.SugaredLogger,
	chain Chain,
	btcChain bitcoin.Chain,
	movedFundsSweepWallet wallet,
	signingExecutor walletSigningExecutor,
	proposal *MovedFundsSweepProposal,
	proposalProcessingStartBlock uint64,
	proposalExpiryBlock uint64,
	waitForBlockFn waitForBlockFn,
) *movedFundsSweepAction {
	transactionExecutor := newWalletTransactionExecutor(
		btcChain,
		movedFundsSweepWallet,
		signingExecutor,
		waitForBlockFn,
	)

	return &movedFundsSweepAction{
		logger:                           logger,
		chain:                            chain,
		btcChain:                         btcChain,
		movedFundsSweepWallet:            movedFundsSweepWallet,
		transactionExecutor:              transactionExecutor,
		proposal:                         proposal,
		proposalProcessingStartBlock:     proposalProcessingStartBlock,
		proposalExpiryBlock:              proposalExpiryBlock,
		signingTimeoutSafetyMarginBlocks: movedFundsSweepSigningTimeoutSafetyMarginBlocks,
		broadcastTimeout:                 movedFundsSweepBroadcastTimeout,
		broadcastCheckDelay:              movedFundsSweepBroadcastCheckDelay,
	}
}

func (mfsa *movedFundsSweepAction) execute() error {
	validateProposalLogger := mfsa.logger.With(
		zap.String("step", "validateProposal"),
	)

	walletPublicKeyHash := bitcoin.PublicKeyHash(mfsa.wallet().publicKey)

	err := ValidateMovedFundsSweepProposal(
		validateProposalLogger,
		walletPublicKeyHash,
		mfsa.proposal,
		mfsa.chain,
	)
	if err != nil {
		return fmt.Errorf("validate proposal step failed: [%v]", err)
	}

	// Prepare the moved funds UTXO.
	movedFundsUtxo, err := assembleMovedFundsSweepUtxo(
		mfsa.btcChain,
		mfsa.proposal.MovingFundsTxHash,
		mfsa.proposal.MovingFundsTxOutputIndex,
	)
	if err != nil {
		return fmt.Errorf(
			"error while assembling moved funds sweep UTXO: [%v]",
			err,
		)
	}

	// Prepare the wallet's main UTXO.
	walletMainUtxo, err := DetermineWalletMainUtxoForPublicKey(
		mfsa.wallet().publicKey,
		mfsa.chain,
		mfsa.btcChain,
	)
	if err != nil {
		return fmt.Errorf(
			"error while determining wallet's main UTXO: [%v]",
			err,
		)
	}

	err = EnsureWalletSyncedBetweenChainsForPublicKey(
		mfsa.wallet().publicKey,
		walletMainUtxo,
		mfsa.chain,
		mfsa.btcChain,
	)
	if err != nil {
		return fmt.Errorf(
			"error while ensuring wallet state is synced between "+
				"BTC and host chain: [%v]",
			err,
		)
	}

	unsignedMovedFundsSweepTx, err := assembleMovedFundsSweepTransaction(
		mfsa.btcChain,
		mfsa.wallet().publicKey,
		movedFundsUtxo,
		walletMainUtxo,
		mfsa.proposal.SweepTxFee.Int64(),
	)
	if err != nil {
		return fmt.Errorf(
			"error while assembling moved funds sweep transaction: [%v]",
			err,
		)
	}

	signTxLogger := mfsa.logger.With(
		zap.String("step", "signTransaction"),
	)

	// Just in case. This should never happen.
	if mfsa.proposalExpiryBlock < mfsa.signingTimeoutSafetyMarginBlocks {
		return fmt.Errorf("invalid proposal expiry block")
	}

	movedFundsSweepTx, err := mfsa.transactionExecutor.signTransaction(
		signTxLogger,
		unsignedMovedFundsSweepTx,
		mfsa.proposalProcessingStartBlock,
		mfsa.proposalExpiryBlock-mfsa.signingTimeoutSafetyMarginBlocks,
	)
	if err != nil {
		return fmt.Errorf("sign transaction step failed: [%v]", err)
	}

	broadcastTxLogger := mfsa.logger.With(
		zap.String("step", "broadcastTransaction"),
		zap.String(
			"movedFundsSweepTxHash",
			movedFundsSweepTx.Hash().Hex(bitcoin.ReversedByteOrder),
		),
	)

	err = mfsa.transactionExecutor.broadcastTransaction(
		broadcastTxLogger,
		movedFundsSweepTx,
		mfsa.broadcastTimeout,
		mfsa.broadcastCheckDelay,
	)
	if err != nil {
		return fmt.Errorf("broadcast transaction step failed: [%v]", err)
	}

	return nil
}

func assembleMovedFundsSweepUtxo(
	bitcoinChain bitcoin.Chain,
	movingFundsTxHash [32]byte,
	movingFundsTxOutputIdx uint32,
) (*bitcoin.UnspentTransactionOutput, error) {
	movingFundsTx, err := bitcoinChain.GetTransaction(
		movingFundsTxHash,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not get moving funds transaction: [%v]",
			err,
		)
	}

	movingFundsTxValue := movingFundsTx.Outputs[movingFundsTxOutputIdx].Value

	return &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: movingFundsTxHash,
			OutputIndex:     movingFundsTxOutputIdx,
		},
		Value: movingFundsTxValue,
	}, nil
}

func (mfsa *movedFundsSweepAction) wallet() wallet {
	return mfsa.movedFundsSweepWallet
}

func (mfsa *movedFundsSweepAction) actionType() WalletActionType {
	return ActionMovedFundsSweep
}

// ValidateMovedFundsSweepProposal checks the moved funds sweep proposal with
// on-chain validation rules.
func ValidateMovedFundsSweepProposal(
	validateProposalLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	proposal *MovedFundsSweepProposal,
	chain interface {
		// ValidateMovedFundsSweepProposal validates the given moved funds sweep
		// proposal against the chain. Returns an error if the proposal is not
		// valid or nil otherwise.
		ValidateMovedFundsSweepProposal(
			walletPublicKeyHash [20]byte,
			proposal *MovedFundsSweepProposal,
		) error
	},
) error {
	validateProposalLogger.Infof("calling chain for proposal validation")

	err := chain.ValidateMovedFundsSweepProposal(
		walletPublicKeyHash,
		proposal,
	)

	if err != nil {
		return fmt.Errorf("moved funds sweep proposal is invalid: [%v]", err)
	}

	validateProposalLogger.Infof("moved funds sweep proposal is valid")

	return nil
}

func assembleMovedFundsSweepTransaction(
	bitcoinChain bitcoin.Chain,
	walletPublicKey *ecdsa.PublicKey,
	movedFundsUtxo *bitcoin.UnspentTransactionOutput,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if movedFundsUtxo == nil {
		return nil, fmt.Errorf("moved funds UTXO is required")
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	// The moved funds UTXO is always the first input.
	err := builder.AddPublicKeyHashInput(movedFundsUtxo)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to moved funds UTXO: [%v]",
			err,
		)
	}

	// If the wallet has the main UTXO it becomes the second input.
	if walletMainUtxo != nil {
		err := builder.AddPublicKeyHashInput(walletMainUtxo)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot add input pointing to main wallet UTXO: [%v]",
				err,
			)
		}
	}

	// Add a single output transferring funds to the wallet itself.
	outputValue := builder.TotalInputsValue() - fee

	outputScript, err := bitcoin.PayToWitnessPublicKeyHash(
		bitcoin.PublicKeyHash(walletPublicKey),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot compute output script: [%v]", err)
	}

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           outputValue,
		PublicKeyScript: outputScript,
	})

	return builder, nil
}

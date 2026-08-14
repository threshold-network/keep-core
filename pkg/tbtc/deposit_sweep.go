package tbtc

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ipfs/go-log/v2"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

const (
	// depositSweepProposalValidityBlocks determines the deposit sweep proposal
	// validity time expressed in blocks. In other words, this is the worst-case
	// time for a deposit sweep during which the wallet is busy and cannot take
	// another actions. The value of 1200 blocks is roughly 4 hours, assuming
	// 12 seconds per block.
	depositSweepProposalValidityBlocks = 1200
	// DepositSweepRequiredFundingTxConfirmations determines the minimum
	// number of confirmations that are needed for a deposit funding Bitcoin
	// transaction in order to consider it a valid part of the deposit sweep
	// proposal.
	DepositSweepRequiredFundingTxConfirmations = 6
	// depositSweepSigningTimeoutSafetyMarginBlocks determines the duration of
	// the safety margin that must be preserved between the signing timeout
	// and the timeout of the entire deposit sweep action. This safety
	// margin prevents against the case where signing completes late and there
	// is not enough time to broadcast the sweep transaction properly.
	// In such a case, wallet signatures may leak and make the wallet subject
	// of fraud accusations. Usage of the safety margin ensures there is enough
	// time to perform post-signing steps of the deposit sweep action.
	// The value of 300 blocks is roughly 1 hour, assuming 12 seconds per block.
	depositSweepSigningTimeoutSafetyMarginBlocks = 300
	// depositSweepBroadcastTimeout determines the time window for deposit
	// sweep transaction broadcast. It is guaranteed that at least
	// depositSweepSigningTimeoutSafetyMarginBlocks is preserved for the broadcast
	// step. However, the happy path for the broadcast step is usually quick
	// and few retries are needed to recover from temporary problems. That
	// said, if the broadcast step does not succeed in a tight timeframe,
	// there is no point to retry for the entire possible time window.
	// Hence, the timeout for broadcast step is set as 25% of the entire
	// time widow determined by depositSweepSigningTimeoutSafetyMarginBlocks.
	depositSweepBroadcastTimeout = 15 * time.Minute
	// depositSweepBroadcastCheckDelay determines the delay that must
	// be preserved between transaction broadcast and the check that ensures
	// the transaction is known on the Bitcoin chain. This delay is needed
	// as spreading the transaction over the Bitcoin network takes time.
	depositSweepBroadcastCheckDelay = 1 * time.Minute
)

// DepositSweepProposal represents a deposit sweep proposal issued by a
// wallet's coordination leader.
type DepositSweepProposal struct {
	DepositsKeys []struct {
		FundingTxHash      bitcoin.Hash
		FundingOutputIndex uint32
	}
	SweepTxFee           *big.Int
	DepositsRevealBlocks []*big.Int
	// MainUtxoHash is the wallet main UTXO snapshot used to size a Taproot
	// sweep. A zero value means the proposal was sized without a main UTXO.
	// Taproot execution fails closed if the current hash no longer matches.
	MainUtxoHash [32]byte
}

func (dsp *DepositSweepProposal) ActionType() WalletActionType {
	return ActionDepositSweep
}

func (dsp *DepositSweepProposal) ValidityBlocks() uint64 {
	return depositSweepProposalValidityBlocks
}

// depositSweepAction is a deposit sweep walletAction.
type depositSweepAction struct {
	logger   *zap.SugaredLogger
	chain    Chain
	btcChain bitcoin.Chain

	sweepingWallet      wallet
	transactionExecutor *walletTransactionExecutor

	proposal                     *DepositSweepProposal
	proposalProcessingStartBlock uint64
	proposalExpiryBlock          uint64

	requiredFundingTxConfirmations   uint
	signingTimeoutSafetyMarginBlocks uint64
	broadcastTimeout                 time.Duration
	broadcastCheckDelay              time.Duration

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}
}

func newDepositSweepAction(
	logger *zap.SugaredLogger,
	chain Chain,
	btcChain bitcoin.Chain,
	sweepingWallet wallet,
	signingExecutor walletSigningExecutor,
	proposal *DepositSweepProposal,
	proposalProcessingStartBlock uint64,
	proposalExpiryBlock uint64,
	waitForBlockFn waitForBlockFn,
) *depositSweepAction {
	transactionExecutor := newWalletTransactionExecutor(
		btcChain,
		sweepingWallet,
		signingExecutor,
		waitForBlockFn,
	)

	return &depositSweepAction{
		logger:                           logger,
		chain:                            chain,
		btcChain:                         btcChain,
		sweepingWallet:                   sweepingWallet,
		transactionExecutor:              transactionExecutor,
		proposal:                         proposal,
		proposalProcessingStartBlock:     proposalProcessingStartBlock,
		proposalExpiryBlock:              proposalExpiryBlock,
		requiredFundingTxConfirmations:   DepositSweepRequiredFundingTxConfirmations,
		signingTimeoutSafetyMarginBlocks: depositSweepSigningTimeoutSafetyMarginBlocks,
		broadcastTimeout:                 depositSweepBroadcastTimeout,
		broadcastCheckDelay:              depositSweepBroadcastCheckDelay,
	}
}

func (dsa *depositSweepAction) execute() error {
	executionStartTime := time.Now()

	// Record deposit sweep execution attempt
	if dsa.metricsRecorder != nil {
		dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_total", 1)
	}

	validateProposalLogger := dsa.logger.With(
		zap.String("step", "validateProposal"),
	)

	walletPublicKeyHash := bitcoin.PublicKeyHash(dsa.wallet().publicKey)

	validatedDeposits, err := ValidateDepositSweepProposal(
		validateProposalLogger,
		walletPublicKeyHash,
		dsa.proposal,
		dsa.requiredFundingTxConfirmations,
		dsa.chain,
		dsa.btcChain,
	)
	if err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf("validate proposal step failed: [%v]", err)
	}

	walletMainUtxo, err := DetermineWalletMainUtxoForPublicKey(
		dsa.wallet().publicKey,
		dsa.chain,
		dsa.btcChain,
	)
	if err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf(
			"error while determining wallet's main UTXO: [%v]",
			err,
		)
	}

	if err := validateDepositSweepMainUtxoSnapshot(
		dsa.chain,
		dsa.proposal,
		validatedDeposits,
		walletMainUtxo,
	); err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf("wallet main UTXO snapshot validation failed: [%v]", err)
	}

	err = EnsureWalletSyncedBetweenChainsForPublicKey(
		dsa.wallet().publicKey,
		walletMainUtxo,
		dsa.chain,
		dsa.btcChain,
	)
	if err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf(
			"error while ensuring wallet state is synced between "+
				"BTC and host chain: [%v]",
			err,
		)
	}

	unsignedSweepTx, err := assembleDepositSweepTransaction(
		dsa.btcChain,
		dsa.wallet().publicKey,
		walletMainUtxo,
		validatedDeposits,
		dsa.proposal.SweepTxFee.Int64(),
	)
	if err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf(
			"error while assembling deposit sweep transaction: [%v]",
			err,
		)
	}

	signTxLogger := dsa.logger.With(
		zap.String("step", "signTransaction"),
	)

	// Just in case. This should never happen.
	if dsa.proposalExpiryBlock < dsa.signingTimeoutSafetyMarginBlocks {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf("invalid proposal expiry block")
	}

	signingStartTime := time.Now()
	sweepTx, err := dsa.transactionExecutor.signTransaction(
		signTxLogger,
		unsignedSweepTx,
		dsa.proposalProcessingStartBlock,
		dsa.proposalExpiryBlock-dsa.signingTimeoutSafetyMarginBlocks,
	)
	if err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf("sign transaction step failed: [%v]", err)
	}

	// Record deposit sweep transaction signing duration
	if dsa.metricsRecorder != nil {
		dsa.metricsRecorder.RecordDuration("deposit_sweep_tx_signing_duration_seconds", time.Since(signingStartTime))
	}

	broadcastTxLogger := dsa.logger.With(
		zap.String("step", "broadcastTransaction"),
		zap.String("sweepTxHash", sweepTx.Hash().Hex(bitcoin.ReversedByteOrder)),
	)

	err = dsa.transactionExecutor.broadcastTransaction(
		broadcastTxLogger,
		sweepTx,
		dsa.broadcastTimeout,
		dsa.broadcastCheckDelay,
	)
	if err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_failed_total", 1)
			dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
		}
		return fmt.Errorf("broadcast transaction step failed: [%v]", err)
	}

	// Record successful deposit sweep execution
	if dsa.metricsRecorder != nil {
		dsa.metricsRecorder.IncrementCounter("deposit_sweep_executions_success_total", 1)
		dsa.metricsRecorder.RecordDuration("deposit_sweep_execution_duration_seconds", time.Since(executionStartTime))
	}

	return nil
}

func validateDepositSweepMainUtxoSnapshot(
	chain interface {
		ComputeMainUtxoHash(
			mainUtxo *bitcoin.UnspentTransactionOutput,
		) [32]byte
	},
	proposal *DepositSweepProposal,
	deposits []*Deposit,
	mainUtxo *bitcoin.UnspentTransactionOutput,
) error {
	if proposal == nil {
		return fmt.Errorf("deposit sweep proposal is nil")
	}
	if len(deposits) == 0 || !deposits[0].IsTaproot() {
		return nil
	}

	currentMainUtxoHash := [32]byte{}
	if mainUtxo != nil {
		currentMainUtxoHash = chain.ComputeMainUtxoHash(mainUtxo)
	}

	if proposal.MainUtxoHash != currentMainUtxoHash {
		return fmt.Errorf(
			"wallet main UTXO changed after Taproot sweep fee sizing: "+
				"expected [0x%x], current [0x%x]",
			proposal.MainUtxoHash,
			currentMainUtxoHash,
		)
	}

	return nil
}

// ValidateDepositSweepProposal checks the deposit sweep proposal with on-chain
// validation rules and verifies transactions on the Bitcoin chain.
func ValidateDepositSweepProposal(
	validateProposalLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	proposal *DepositSweepProposal,
	requiredFundingTxConfirmations uint,
	chain interface {
		// PastDepositRevealedEvents fetches past deposit reveal events according
		// to the provided filter or unfiltered if the filter is nil. Returned
		// events are sorted by the block number in the ascending order, i.e. the
		// latest event is at the end of the slice.
		PastDepositRevealedEvents(
			filter *DepositRevealedEventFilter,
		) ([]*DepositRevealedEvent, error)

		// PastTaprootDepositRevealedEvents fetches past Taproot deposit reveal
		// events according to the provided filter or unfiltered if the filter
		// is nil. Returned events are sorted by the block number in the
		// ascending order, i.e. the latest event is at the end of the slice.
		PastTaprootDepositRevealedEvents(
			filter *DepositRevealedEventFilter,
		) ([]*TaprootDepositRevealedEvent, error)

		// ValidateDepositSweepProposal validates the given deposit sweep proposal
		// against the chain. It requires some additional data about the deposits
		// that must be fetched externally. Returns an error if the proposal is
		// not valid or nil otherwise.
		ValidateDepositSweepProposal(
			walletPublicKeyHash [20]byte,
			proposal *DepositSweepProposal,
			depositsExtraInfo []struct {
				*Deposit
				FundingTx *bitcoin.Transaction
			},
		) error

		// ValidateTaprootDepositSweepProposal validates the given Taproot
		// deposit sweep proposal against the chain. It requires some additional
		// data about the deposits that must be fetched externally. Returns an
		// error if the proposal is not valid or nil otherwise.
		ValidateTaprootDepositSweepProposal(
			walletPublicKeyHash [20]byte,
			proposal *DepositSweepProposal,
			depositsExtraInfo []struct {
				*Deposit
				FundingTx *bitcoin.Transaction
			},
		) error

		// GetDepositRequest gets the on-chain deposit request for the given
		// funding transaction hash and output index.The returned values represent:
		// - deposit request which is non-nil only when the deposit request was
		//   found,
		// - boolean value which is true if the deposit request was found, false
		//   otherwise,
		// - error which is non-nil only when the function execution failed. It will
		//   be nil if the deposit request was not found, but the function execution
		//   succeeded.
		GetDepositRequest(
			fundingTxHash bitcoin.Hash,
			fundingOutputIndex uint32,
		) (*DepositChainRequest, bool, error)
	},
	btcChain bitcoin.Chain,
) ([]*Deposit, error) {
	depositExtraInfo := make(
		[]struct {
			*Deposit
			FundingTx *bitcoin.Transaction
		},
		len(proposal.DepositsKeys),
	)

	validateProposalLogger.Infof("gathering prerequisites for proposal validation")

	if len(proposal.DepositsKeys) != len(proposal.DepositsRevealBlocks) {
		return nil, fmt.Errorf("proposal's reveal blocks list has a wrong length")
	}

	taprootDepositsCount := 0

	for i, depositKey := range proposal.DepositsKeys {
		depositDisplayIndex := fmt.Sprintf("%v/%v", i+1, len(proposal.DepositsKeys))

		validateProposalLogger.Infof(
			"deposit [%v] - checking confirmations count for funding tx",
			depositDisplayIndex,
		)

		confirmations, err := btcChain.GetTransactionConfirmations(
			depositKey.FundingTxHash,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get funding tx confirmations count "+
					"for deposit [%v]: [%v]",
				depositDisplayIndex,
				err,
			)
		}

		if confirmations < requiredFundingTxConfirmations {
			return nil, fmt.Errorf(
				"funding tx of deposit [%v] has only [%v/%v] of "+
					"required confirmations",
				depositDisplayIndex,
				confirmations,
				requiredFundingTxConfirmations,
			)
		}

		validateProposalLogger.Infof(
			"deposit [%v] - fetching deposit's extra data",
			depositDisplayIndex,
		)

		fundingTx, err := btcChain.GetTransaction(depositKey.FundingTxHash)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get funding tx data for deposit [%v]: [%v]",
				depositDisplayIndex,
				err,
			)
		}

		revealBlock := proposal.DepositsRevealBlocks[i].Uint64()

		filter := &DepositRevealedEventFilter{
			StartBlock:          revealBlock,
			EndBlock:            &revealBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		}

		// We need to fetch the past DepositRevealed event for the given deposit.
		// It may be tempting to fetch such events for all deposit keys
		// in the proposal using a single call, however, this solution has
		// serious downsides. Popular chain clients have limitations
		// for fetching past chain events regarding the requested block
		// range and/or returned data size. In this context, it is better to
		// do several well-tailored calls than a single general one.
		// We have the revealBlock passed by the coordinator within the proposal
		// so, we can use it to make a narrow call. Moreover, we use the
		// wallet PKH as additional filter to limit the size of returned data.
		events, err := chain.PastDepositRevealedEvents(filter)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get on-chain DepositRevealed events for deposit [%v]: [%v]",
				depositDisplayIndex,
				err,
			)
		}

		// There may be multiple events returned for the provided filter.
		// Find the one matching our depositKey.
		var matchingEvent *DepositRevealedEvent
		for _, event := range events {
			if event.FundingTxHash == depositKey.FundingTxHash &&
				event.FundingOutputIndex == depositKey.FundingOutputIndex {
				matchingEvent = event
				break
			}
		}

		taprootEvents, err := chain.PastTaprootDepositRevealedEvents(filter)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get on-chain TaprootDepositRevealed events for deposit [%v]: [%v]",
				depositDisplayIndex,
				err,
			)
		}

		var matchingTaprootEvent *TaprootDepositRevealedEvent
		for _, event := range taprootEvents {
			if event.FundingTxHash == depositKey.FundingTxHash &&
				event.FundingOutputIndex == depositKey.FundingOutputIndex {
				matchingTaprootEvent = event
				break
			}
		}

		if matchingEvent == nil && matchingTaprootEvent == nil {
			return nil, fmt.Errorf(
				"no matching DepositRevealed or TaprootDepositRevealed event for deposit [%v]: [%v]",
				depositDisplayIndex,
				err,
			)
		}

		depositRequest, found, err := chain.GetDepositRequest(
			depositKey.FundingTxHash,
			depositKey.FundingOutputIndex,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get request data for deposit [%v]: [%v]",
				depositDisplayIndex,
				err,
			)
		}
		if !found {
			return nil, fmt.Errorf(
				"request data not found for deposit [%v]",
				depositDisplayIndex,
			)
		}

		depositExtraInfo[i] = struct {
			*Deposit
			FundingTx *bitcoin.Transaction
		}{
			Deposit: func() *Deposit {
				if matchingTaprootEvent != nil {
					taprootDepositsCount++
					return matchingTaprootEvent.unpack(depositRequest.ExtraData)
				}

				return matchingEvent.unpack(depositRequest.ExtraData)
			}(),
			FundingTx: fundingTx,
		}
	}

	if taprootDepositsCount > 0 && taprootDepositsCount != len(proposal.DepositsKeys) {
		return nil, fmt.Errorf(
			"mixed legacy and Taproot deposits are not supported in one sweep proposal",
		)
	}

	validateProposalLogger.Infof("calling chain for proposal validation")

	var err error
	if taprootDepositsCount > 0 {
		err = chain.ValidateTaprootDepositSweepProposal(
			walletPublicKeyHash,
			proposal,
			depositExtraInfo,
		)
	} else {
		err = chain.ValidateDepositSweepProposal(
			walletPublicKeyHash,
			proposal,
			depositExtraInfo,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("deposit sweep proposal is invalid: [%v]", err)
	}

	validateProposalLogger.Infof(
		"deposit sweep proposal is valid",
	)

	deposits := make([]*Deposit, len(depositExtraInfo))
	for i, dei := range depositExtraInfo {
		deposits[i] = dei.Deposit
	}

	return deposits, nil
}

func (dsa *depositSweepAction) wallet() wallet {
	return dsa.sweepingWallet
}

func (dsa *depositSweepAction) actionType() WalletActionType {
	return ActionDepositSweep
}

// setMetricsRecorder sets the metrics recorder for the deposit sweep action.
func (dsa *depositSweepAction) setMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	dsa.metricsRecorder = recorder
}

// assembleDepositSweepTransaction constructs an unsigned deposit sweep Bitcoin
// transaction.
//
// Regarding input arguments, the walletMainUtxo parameter is optional and
// can be set as nil if the wallet does not have a main UTXO at the moment.
// The deposits slice must contain at least one element. The fee argument
// is not validated in any way so must be chosen with respect to the system
// limitations.
//
// The resulting bitcoin.TransactionBuilder instance holds all the data
// necessary to sign the transaction and obtain a bitcoin.Transaction instance
// ready to be spread across the Bitcoin network.
func assembleDepositSweepTransaction(
	bitcoinChain bitcoin.Chain,
	walletPublicKey *ecdsa.PublicKey,
	walletMainUtxo *bitcoin.UnspentTransactionOutput,
	deposits []*Deposit,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if len(deposits) < 1 {
		return nil, fmt.Errorf("at least one deposit is required")
	}

	taprootDepositsCount := 0
	for _, deposit := range deposits {
		if deposit.IsTaproot() {
			taprootDepositsCount++
		}
	}

	if taprootDepositsCount > 0 && taprootDepositsCount != len(deposits) {
		return nil, fmt.Errorf(
			"mixed legacy and Taproot deposits are not supported in one sweep transaction",
		)
	}

	taprootSweep := taprootDepositsCount > 0

	if !taprootSweep && walletMainUtxo != nil {
		scriptType, err := walletMainUtxoScriptType(
			bitcoinChain,
			walletMainUtxo,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot inspect wallet main UTXO script: [%v]",
				err,
			)
		}

		if scriptType == bitcoin.P2TRScript {
			return nil, fmt.Errorf(
				"legacy deposit sweeps are not supported for " +
					"Taproot wallet main UTXOs",
			)
		}
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	if walletMainUtxo != nil {
		err := builder.AddPublicKeyHashInput(walletMainUtxo)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot add input pointing to wallet main UTXO: [%v]",
				err,
			)
		}
	}

	for i, deposit := range deposits {
		if deposit.IsTaproot() {
			merkleRoot, err := deposit.TaprootMerkleRoot()
			if err != nil {
				return nil, fmt.Errorf(
					"cannot compute Taproot merkle root for deposit [%v]: [%v]",
					i,
					err,
				)
			}

			err = builder.AddTaprootKeyPathInputWithMerkleRoot(
				deposit.Utxo,
				*deposit.WalletXOnlyPublicKey,
				merkleRoot,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot add input pointing to Taproot deposit [%v] UTXO: [%v]",
					i,
					err,
				)
			}
		} else {
			depositScript, err := deposit.Script()
			if err != nil {
				return nil, fmt.Errorf(
					"cannot get script for deposit [%v]: [%v]",
					i,
					err,
				)
			}

			err = builder.AddScriptHashInput(deposit.Utxo, depositScript)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot add input pointing to deposit [%v] UTXO: [%v]",
					i,
					err,
				)
			}
		}
	}

	if taprootSweep && !builder.HasOnlyTaprootKeyPathInputs() {
		return nil, fmt.Errorf(
			"Taproot deposit sweep requires a Taproot wallet main UTXO",
		)
	}

	var outputScript bitcoin.Script
	var err error
	if taprootSweep {
		walletXOnlyPublicKey, err := walletXOnlyPublicKey(walletPublicKey)
		if err != nil {
			return nil, err
		}

		outputScript, err = bitcoin.PayToTaproot(walletXOnlyPublicKey)
		if err != nil {
			return nil, fmt.Errorf("cannot compute Taproot output script: [%v]", err)
		}
	} else {
		walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
		outputScript, err = bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
		if err != nil {
			return nil, fmt.Errorf("cannot compute output script: [%v]", err)
		}
	}

	outputValue := builder.TotalInputsValue() - fee

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           outputValue,
		PublicKeyScript: outputScript,
	})

	return builder, nil
}

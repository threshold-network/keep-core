package tbtc

import (
	"context"
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
	// MinSweepTxSatPerVByteFee mirrors tbtcpg.MinWalletTxSatPerVByteFee, the safe
	// minimum sweep fee rate. It is duplicated here because pkg/tbtcpg imports
	// pkg/tbtc, so this package cannot import the canonical constant without a
	// dependency cycle; keep the two in sync. It is exported so the external
	// tbtc_test package can compare it against the canonical tbtcpg value
	// (guarded by TestSweepFeeConstantsMirrorTbtcpg). It backs a follower-side
	// soft (log-only) check that the leader's proposed sweep fee is not below
	// the floor (see threshold-network/keep-core#4171).
	MinSweepTxSatPerVByteFee = 5
	// DepositScriptByteSize mirrors tbtcpg.DepositScriptByteSize, the worst-case
	// deposit script size used to estimate the sweep transaction virtual size.
	// Exported alongside MinSweepTxSatPerVByteFee for the same cross-package
	// drift guard.
	DepositScriptByteSize = 126
)

// DepositKey identifies a deposit by the outpoint of its funding transaction.
//
// Note: DepositKey is a named type; it replaced the anonymous struct
// previously used inline as the element type of
// DepositSweepProposal.DepositsKeys. Go does not allow assigning an
// anonymous-struct-typed slice literal to a named-struct-typed slice field,
// so code outside this module that builds a DepositSweepProposal from the
// old anonymous struct literal must switch to constructing []DepositKey
// values instead.
type DepositKey struct {
	FundingTxHash      bitcoin.Hash
	FundingOutputIndex uint32
}

// DepositSweepProposal represents a deposit sweep proposal issued by a
// wallet's coordination leader.
type DepositSweepProposal struct {
	DepositsKeys         []DepositKey
	SweepTxFee           *big.Int
	DepositsRevealBlocks []*big.Int
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
	transactionMonitor *transactionMonitor,
) *depositSweepAction {
	transactionExecutor := newWalletTransactionExecutor(
		btcChain,
		sweepingWallet,
		signingExecutor,
		waitForBlockFn,
	)

	transactionExecutor.setTransactionMonitor(transactionMonitor)

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

	walletMainUtxo, err := DetermineWalletMainUtxo(
		walletPublicKeyHash,
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

	err = EnsureWalletSyncedBetweenChains(
		walletPublicKeyHash,
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

	for i, depositKey := range proposal.DepositsKeys {
		depositDisplayIndex := fmt.Sprintf("%v/%v", i+1, len(proposal.DepositsKeys))

		validateProposalLogger.Infof(
			"deposit [%v] - checking confirmations count for funding tx",
			depositDisplayIndex,
		)

		confirmations, err := btcChain.GetTransactionConfirmations(
			context.Background(),
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
		events, err := chain.PastDepositRevealedEvents(&DepositRevealedEventFilter{
			StartBlock:          revealBlock,
			EndBlock:            &revealBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		})
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

		if matchingEvent == nil {
			return nil, fmt.Errorf(
				"no matching DepositRevealed event for deposit [%v]: [%v]",
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
			Deposit:   matchingEvent.unpack(depositRequest.ExtraData),
			FundingTx: fundingTx,
		}
	}

	validateProposalLogger.Infof("calling chain for proposal validation")

	err := chain.ValidateDepositSweepProposal(
		walletPublicKeyHash,
		proposal,
		depositExtraInfo,
	)
	if err != nil {
		return nil, fmt.Errorf("deposit sweep proposal is invalid: [%v]", err)
	}

	validateProposalLogger.Infof(
		"deposit sweep proposal is valid",
	)

	// Follower-side soft check on the proposed fee. The on-chain
	// WalletProposalValidator only bounds the sweep fee from above, not below,
	// so a misbehaving or unpatched leader can propose a fee at the ~1 sat/vByte
	// relay floor that this node would otherwise sign - the same underpricing
	// that jams the wallet (see threshold-network/keep-core#4171). We recompute
	// the safe minimum and warn if the proposal is below it.
	//
	// This is intentionally log-only, not a rejection: rejecting a below-floor
	// proposal here would, during a mixed-version rollout, split signers (patched
	// nodes reject, unpatched nodes sign) and could stall signing. Hard
	// enforcement belongs on-chain in the WalletProposalValidator, or behind a
	// coordinated all-nodes upgrade.
	if sweepTxSize, sizeErr := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddScriptHashInputs(len(proposal.DepositsKeys), DepositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize(); sizeErr != nil {
		validateProposalLogger.Warnf(
			"cannot estimate sweep tx size for the fee sanity check: [%v]",
			sizeErr,
		)
	} else {
		minSweepTxFee := big.NewInt(int64(MinSweepTxSatPerVByteFee) * sweepTxSize)

		switch {
		// This branch is defense-in-depth for test/mock chain implementations
		// and is not expected to be reachable on the real production path: by
		// the time control reaches this point, chain.ValidateDepositSweepProposal
		// above has already ABI-packed proposal.SweepTxFee to call the on-chain
		// WalletProposalValidator, which panics on a nil *big.Int before this
		// code ever runs. Likewise, a proposal decoded off the wire
		// (DepositSweepProposal.Unmarshal in marshaling.go) always constructs
		// SweepTxFee via new(big.Int).SetBytes(...), which never yields nil.
		case proposal.SweepTxFee == nil:
			validateProposalLogger.Warnf(
				"proposal has no sweep tx fee set; expected at least the safe "+
					"minimum [%d] ([%d] sat/vByte * [%d] vByte)",
				minSweepTxFee,
				MinSweepTxSatPerVByteFee,
				sweepTxSize,
			)
		case proposal.SweepTxFee.Cmp(minSweepTxFee) < 0:
			validateProposalLogger.Warnf(
				"proposed sweep tx fee [%v] is below the safe minimum [%d] "+
					"([%d] sat/vByte * [%d] vByte); the leader may be underpricing "+
					"the sweep, which risks it getting stuck in the mempool and "+
					"jamming the wallet",
				proposal.SweepTxFee,
				minSweepTxFee,
				MinSweepTxSatPerVByteFee,
				sweepTxSize,
			)
		}
	}

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

	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	outputScript, err := bitcoin.PayToWitnessPublicKeyHash(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf("cannot compute output script: [%v]", err)
	}

	outputValue := builder.TotalInputsValue() - fee

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           outputValue,
		PublicKeyScript: outputScript,
	})

	return builder, nil
}

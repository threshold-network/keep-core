package tbtc

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ipfs/go-log/v2"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
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
	// DepositScriptByteSize mirrors tbtcpg.DepositScriptByteSize, the worst-case
	// deposit script size used to estimate the sweep transaction virtual size.
	// Exported for the external tbtc_test package to compare it against the
	// canonical tbtcpg value (guarded by TestSweepFeeConstantsMirrorTbtcpg).
	DepositScriptByteSize = 126
	// reservationParametersFetchRetries bounds how many consecutive
	// ReservationParameters attempts ValidateDepositSweepProposal makes
	// before concluding reservations are not currently active. Mirrors
	// tbtcpg.reservationParametersFetchRetries's reasoning: a
	// reservation-related Bridge call that doesn't exist yet on the
	// deployed contract (pre-upgrade) fails deterministically on every
	// attempt, while a transient RPC hiccup against an otherwise-live
	// reservation system usually recovers within a few - and the
	// consequence of guessing wrong the other way (co-signing a sweep of
	// a genuinely reserved deposit) is irreversible, so a single failed
	// attempt is not enough evidence to draw that conclusion. Retried
	// identically on both the leader (tbtcpg) and follower (here) sides:
	// no retry count eliminates the chance of independent followers
	// reaching different conclusions from independent RPC calls, but
	// fewer attempts only makes a wrong conclusion more likely, never
	// less, so there is no safety argument for the follower retrying
	// less than the leader.
	reservationParametersFetchRetries = 3
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
//
// Migrating from the old anonymous-struct literal:
//
//	// Before:
//	DepositsKeys: []struct{
//	    FundingTxHash:      chain.Hash(...),
//	    FundingOutputIndex: 0,
//	}{...},
//
//	// After:
//	DepositsKeys: []DepositKey{
//	    {FundingTxHash: chain.Hash(...), FundingOutputIndex: 0},
//	    ...
//	},
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
		dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsTotal, 1)
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
			dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsFailedTotal, 1)
			dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
		}
		return fmt.Errorf("validate proposal step failed: [%v]", err)
	}

	// Follower-side observability for the below-floor sweep-fee soft check
	// (threshold-network/keep-core#4171). ValidateDepositSweepProposal only warns
	// in the logs when the leader's proposed fee is below the safe minimum;
	// surface it as a counter too so operators can alert on underpriced proposals
	// during a mixed-version rollout rather than grepping node logs. Log-only,
	// like the check itself: the node still signs the proposal.
	if dsa.metricsRecorder != nil {
		if check, checkErr := checkSweepFeeFloor(dsa.proposal); checkErr == nil &&
			check.belowFloor {
			dsa.metricsRecorder.IncrementCounter(
				clientinfo.MetricDepositSweepFeeBelowFloorTotal, 1,
			)
		}
	}

	walletMainUtxo, err := DetermineWalletMainUtxo(
		walletPublicKeyHash,
		dsa.chain,
		dsa.btcChain,
	)
	if err != nil {
		if dsa.metricsRecorder != nil {
			dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsFailedTotal, 1)
			dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
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
			dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsFailedTotal, 1)
			dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
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
			dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsFailedTotal, 1)
			dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
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
			dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsFailedTotal, 1)
			dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
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
			dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsFailedTotal, 1)
			dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
		}
		return fmt.Errorf("sign transaction step failed: [%v]", err)
	}

	// Record deposit sweep transaction signing duration
	if dsa.metricsRecorder != nil {
		dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepTxSigningDurationSeconds, time.Since(signingStartTime))
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
			dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsFailedTotal, 1)
			dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
		}
		return fmt.Errorf("broadcast transaction step failed: [%v]", err)
	}

	// Record successful deposit sweep execution
	if dsa.metricsRecorder != nil {
		dsa.metricsRecorder.IncrementCounter(clientinfo.MetricDepositSweepExecutionsSuccessTotal, 1)
		dsa.metricsRecorder.RecordDuration(clientinfo.MetricDepositSweepExecutionDurationSeconds, time.Since(executionStartTime))
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

		// BuildDepositKey calculates a deposit key for the given funding
		// transaction hash and output index.
		BuildDepositKey(
			fundingTxHash bitcoin.Hash,
			fundingOutputIndex uint32,
		) *big.Int

		// ReservationParameters gets the current on-chain values of the
		// Bridge reservation parameters, including the reservation vault
		// address used to cheaply pre-filter which deposits are worth an
		// IsReservedDeposit call at all. Called once per validation
		// (before this loop), not per deposit. Fetched unconditionally,
		// so it IS reached pre-upgrade; a failure is treated as
		// "reservations not active" and the whole reserved-deposit
		// filter below is skipped for this validation, mirroring the
		// leader-side degradation in the deposit-sweep coordination
		// task - it must not turn into every follower rejecting every
		// sweep proposal before the Bridge exposes the reservation
		// contracts.
		ReservationParameters() (*ReservationParameters, error)

		// IsReservedDeposit returns true if the given deposit was revealed
		// with the reservation vault address and is therefore a
		// reservation rather than a default deposit. This is the
		// follower-side counterpart to the leader-path reserved-deposit
		// filter in the deposit-sweep coordination task: a follower must
		// not co-sign a reserved-deposit sweep. Only called for deposits
		// whose revealed vault matches the current reservation vault AND
		// only once ReservationParameters above has been fetched
		// successfully - see the call site.
		IsReservedDeposit(depositKey *big.Int) (bool, error)
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

	// Determine the reservation vault once per validation, not per
	// deposit, and retry a bounded number of times before concluding
	// reservations are not currently active - mirroring
	// tbtcpg.findDeposits's identical leader-side degradation (see
	// reservationParametersFetchRetries's doc comment for the reasoning,
	// including why the follower retries exactly as many times as the
	// leader rather than fewer). A failure here must not turn into
	// every follower rejecting every deposit-sweep proposal pre-upgrade.
	// This does not weaken safety: the IsReservedDeposit call below,
	// for any deposit the cheap vault-match prefilter flags as a
	// candidate, still rejects the proposal outright on error rather
	// than skipping - see its call site for why the two calls take
	// opposite failure postures.
	reservationsActive := true
	var reservationParams *ReservationParameters
	var reservationParamsErr error
	for attempt := 1; attempt <= reservationParametersFetchRetries; attempt++ {
		reservationParams, reservationParamsErr = chain.ReservationParameters()
		if reservationParamsErr == nil {
			break
		}
		validateProposalLogger.Debugf(
			"failed to fetch reservation parameters (attempt %d/%d): [%v]",
			attempt,
			reservationParametersFetchRetries,
			reservationParamsErr,
		)
	}
	if reservationParamsErr != nil {
		validateProposalLogger.Infof(
			"reservation parameters unavailable after %d attempts, "+
				"skipping reserved deposit filter: [%v]",
			reservationParametersFetchRetries,
			reservationParamsErr,
		)
		reservationsActive = false
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

		// Reuse the vault already carried by matchingEvent - fetched above
		// for unrelated reasons - instead of an extra chain call, to cheaply
		// pre-filter which deposits are worth an IsReservedDeposit call at
		// all. Only consulted when reservationsActive (see the
		// once-per-validation fetch above this loop); skipped entirely
		// otherwise so this call, like ReservationParameters, degrades
		// gracefully pre-upgrade instead of rejecting every proposal.
		if reservationsActive && depositTargetsReservationVault(
			matchingEvent.Vault,
			reservationParams.ReservationVault,
		) {
			// Hard reject, unlike the fee soft-check below: that check
			// is log-only specifically to avoid splitting signers during
			// a mixed-version rollout (see its comment ahead of
			// warnIfProposedWalletTxFeeBelowBufferedFloor). Sweeping a
			// reservation is irreversible, so here the tradeoff flips -
			// a stalled sweep (some followers reject, signing doesn't
			// reach threshold) is the acceptable cost, not a wrongly
			// swept reservation.
			validateProposalLogger.Infof(
				"deposit [%v] - checking reservation status",
				depositDisplayIndex,
			)

			depositReservationKey := chain.BuildDepositKey(
				depositKey.FundingTxHash,
				depositKey.FundingOutputIndex,
			)
			isReserved, err := chain.IsReservedDeposit(depositReservationKey)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot check reservation status for deposit [%v]: [%v]",
					depositDisplayIndex,
					err,
				)
			}
			if isReserved {
				return nil, fmt.Errorf(
					"deposit [%v] is a reserved deposit and cannot be swept",
					depositDisplayIndex,
				)
			}
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
	// the safe minimum (applying the 25% safety buffer that
	// tbtcpg.applyWalletTxFeeFloor would also enforce on the leader side) and
	// warn if the proposal is below it.
	//
	// This is intentionally log-only, not a rejection: rejecting a below-floor
	// proposal here would, during a mixed-version rollout, split signers (patched
	// nodes reject, unpatched nodes sign) and could stall signing. Hard
	// enforcement belongs on-chain in the WalletProposalValidator, or behind a
	// coordinated all-nodes upgrade. The threshold is recomputed in
	// checkSweepFeeFloor using the same buffered floor
	// (MinWalletTxSatPerVByteFee + WalletTxFeeBufferPercent) as
	// warnIfProposedWalletTxFeeBelowBufferedFloor (proposal_fee_check.go);
	// keep the size estimator in checkSweepFeeFloor in sync with the
	// leader-side estimator in tbtcpg/deposit_sweep.go.
	check, checkErr := checkSweepFeeFloor(proposal)
	if checkErr != nil {
		validateProposalLogger.Warnf(
			"cannot estimate sweep tx size for the fee sanity check: [%v]",
			checkErr,
		)
	} else {
		warnIfProposedWalletTxFeeBelowBufferedFloor(
			validateProposalLogger,
			MinWalletTxSatPerVByteFee,
			check.sweepTxSize,
			proposal.SweepTxFee,
			"deposit sweep",
		)
	}

	deposits := make([]*Deposit, len(depositExtraInfo))
	for i, dei := range depositExtraInfo {
		deposits[i] = dei.Deposit
	}

	return deposits, nil
}

// sweepFeeCheck is the result of the follower-side soft check that recomputes
// the safe buffered-minimum sweep fee and compares it against the leader's
// proposed fee.
type sweepFeeCheck struct {
	// sweepTxSize is the estimated virtual size, in vBytes, of the sweep
	// transaction described by the proposal.
	sweepTxSize int64
	// minSweepTxFee is the safe buffered-minimum total fee - the same
	// MinWalletTxSatPerVByteFee/WalletTxFeeBufferPercent policy applied by
	// warnIfProposedWalletTxFeeBelowBufferedFloor - the proposal is expected
	// to meet or exceed.
	minSweepTxFee *big.Int
	// belowFloor is true when the proposal fails to meet the safe minimum,
	// either because the proposed fee is strictly below minSweepTxFee or because
	// no fee is set at all (proposal.SweepTxFee == nil, treated as below floor).
	belowFloor bool
}

// checkSweepFeeFloor recomputes the safe buffered-minimum sweep fee for the
// given proposal - via bufferedWalletTxFeeFloor (proposal_fee_check.go), the
// same helper warnIfProposedWalletTxFeeBelowBufferedFloor uses - and reports
// whether the proposed fee is below it. Extracting the decision from
// ValidateDepositSweepProposal keeps it directly testable (rather than
// asserting on logger output) and lets the follower emit a below-floor metric
// without recomputing the estimate inline. It returns an error only when the
// sweep transaction virtual size cannot be estimated, or when the buffered
// fee floor itself cannot be computed from the operator-configured policy
// (MinWalletTxSatPerVByteFee <= 0, non-positive tx size, or a negative
// WalletTxFeeBufferPercent) - see bufferedWalletTxFeeFloor.
func checkSweepFeeFloor(proposal *DepositSweepProposal) (sweepFeeCheck, error) {
	sweepTxSize, err := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddScriptHashInputs(len(proposal.DepositsKeys), DepositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		return sweepFeeCheck{}, err
	}

	_, minSweepTxFee := bufferedWalletTxFeeFloor(MinWalletTxSatPerVByteFee, sweepTxSize)
	if minSweepTxFee == nil {
		return sweepFeeCheck{}, fmt.Errorf(
			"cannot compute the safe buffered minimum sweep fee: degenerate "+
				"policy inputs (MinWalletTxSatPerVByteFee=[%d], "+
				"WalletTxFeeBufferPercent=[%d])",
			MinWalletTxSatPerVByteFee,
			WalletTxFeeBufferPercent,
		)
	}

	belowFloor := proposal.SweepTxFee == nil ||
		proposal.SweepTxFee.Cmp(minSweepTxFee) < 0

	return sweepFeeCheck{
		sweepTxSize:   sweepTxSize,
		minSweepTxFee: minSweepTxFee,
		belowFloor:    belowFloor,
	}, nil
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

// depositTargetsReservationVault reports whether depositVault (a deposit's
// revealed vault, nil for a vault-less deposit) matches reservationVault
// (the current on-chain reservation vault address). Mirrors
// tbtcpg.depositTargetsReservationVault - kept as a separate, unexported
// copy since pkg/tbtc and pkg/tbtcpg share no reservation-helpers package.
func depositTargetsReservationVault(
	depositVault *chain.Address,
	reservationVault chain.Address,
) bool {
	if depositVault == nil {
		return false
	}
	return strings.EqualFold(
		string(*depositVault),
		string(reservationVault),
	)
}

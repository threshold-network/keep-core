package tbtcpg

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ipfs/go-log/v2"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/internal/hexutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// RedemptionTask is a task that may produce a redemption proposal.
type RedemptionTask struct {
	chain    Chain
	btcChain bitcoin.Chain

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		SetGauge(name string, value float64)
	}
}

func NewRedemptionTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *RedemptionTask {
	return &RedemptionTask{
		chain:    chain,
		btcChain: btcChain,
	}
}

// setMetricsRecorder sets the metrics recorder for the redemption task.
func (rt *RedemptionTask) setMetricsRecorder(recorder interface {
	SetGauge(name string, value float64)
}) {
	rt.metricsRecorder = recorder
}

func (rt *RedemptionTask) Run(request *tbtc.CoordinationProposalRequest) (
	tbtc.CoordinationProposal,
	bool,
	error,
) {
	walletPublicKeyHash := request.WalletPublicKeyHash

	taskLogger := logger.With(
		zap.String("task", rt.ActionType().String()),
		zap.String("walletPKH", fmt.Sprintf("0x%x", walletPublicKeyHash)),
	)

	redemptionMaxSize, err := rt.chain.GetRedemptionMaxSize()
	if err != nil {
		return nil, false, fmt.Errorf(
			"failed to get redemption max size: [%w]",
			err,
		)
	}

	redeemersOutputScripts, err := rt.FindPendingRedemptions(
		taskLogger,
		walletPublicKeyHash,
		redemptionMaxSize,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot find pending redemption requests: [%w]",
			err,
		)
	}

	if len(redeemersOutputScripts) == 0 {
		taskLogger.Info("no pending redemption requests")
		return nil, false, nil
	}

	proposal, err := rt.ProposeRedemption(
		taskLogger,
		walletPublicKeyHash,
		redeemersOutputScripts,
		0,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot prepare redemption proposal: [%w]",
			err,
		)
	}

	return proposal, true, nil
}

func (rt *RedemptionTask) ActionType() tbtc.WalletActionType {
	return tbtc.ActionRedemption
}

// RedemptionRequest represents a redemption request.
type RedemptionRequest struct {
	WalletPublicKeyHash  [20]byte
	RedemptionKey        string
	RedeemerOutputScript bitcoin.Script
	RequestedAt          time.Time
	RequestedAmount      uint64
}

// FindPendingRedemptions finds pending redemptions requests for the
// provided wallet. The returned value is a list of redeemers output
// scripts that come from detected pending requests targeting this wallet.
// The maxNumberOfRequests parameter is used as a ceiling for the number of
// requests in the result. If number of discovered requests meets the
// maxNumberOfRequests the function will stop fetching more requests.
func (rt *RedemptionTask) FindPendingRedemptions(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	maxNumberOfRequests uint16,
) ([]bitcoin.Script, error) {
	if walletPublicKeyHash == [20]byte{} {
		return nil, fmt.Errorf("wallet public key hash is required")
	}

	taskLogger.Infof(
		"fetching max [%d] redemption requests",
		maxNumberOfRequests,
	)

	blockCounter, err := rt.chain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get block counter: [%w]",
			err,
		)
	}

	currentBlockNumber, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get current block number: [%w]",
			err,
		)
	}

	requestMinAge, err := rt.chain.GetRedemptionRequestMinAge()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get redemption request minimum age: [%w]",
			err,
		)
	}

	redemptionParameters, err := rt.chain.GetRedemptionParameters()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get redemption parameters: [%w]",
			err,
		)
	}
	requestTimeout := redemptionParameters.Timeout

	taskLogger.Infof("fetching pending redemption requests")

	pendingRedemptions, err := findPendingRedemptions(
		taskLogger,
		rt.chain,
		walletPublicKeyHash,
		currentBlockNumber,
		maxNumberOfRequests,
		requestTimeout,
		requestMinAge,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot get pending redemptions: [%w]", err)
	}

	taskLogger.Infof("found [%d] redemption requests", len(pendingRedemptions))

	// Record pending redemption requests count
	if rt.metricsRecorder != nil {
		rt.metricsRecorder.SetGauge("redemption_pending_requests_count", float64(len(pendingRedemptions)))
	}

	result := make([]bitcoin.Script, 0)

	for _, pendingRedemption := range pendingRedemptions {
		taskLogger.Infof(
			"redemption request [%s] - requested at: [%s]",
			pendingRedemption.RedemptionKey,
			pendingRedemption.RequestedAt,
		)

		result = append(result, pendingRedemption.RedeemerOutputScript)
	}

	return result, nil
}

// ProposeRedemption returns a redemption proposal.
func (rt *RedemptionTask) ProposeRedemption(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	redeemersOutputScripts []bitcoin.Script,
	fee int64,
) (*tbtc.RedemptionProposal, error) {
	if len(redeemersOutputScripts) == 0 {
		return nil, fmt.Errorf("redemptions list is empty")
	}

	taskLogger.Infof("preparing a redemption proposal")

	// Estimate fee if it's missing. Here we bound the estimate by the maximum
	// total fee only. The per-request maximum fee (TxMaxFee, snapshotted per
	// request at request creation) is enforced solely by the on-chain
	// validation of the proposal and is not checked here. Note that the
	// safe-minimum floor (see EstimateRedemptionFee) raises the total fee and
	// therefore each request's fee share, so it can push a share above that
	// request's per-request maximum even while the total stays within
	// txMaxTotalFee; such a proposal is rejected only by on-chain validation.
	if fee <= 0 {
		taskLogger.Infof("estimating redemption transaction fee")

		redemptionParameters, err := rt.chain.GetRedemptionParameters()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get redemption tx max total fee: [%w]",
				err,
			)
		}
		txMaxFee := redemptionParameters.TxMaxFee
		txMaxTotalFee := redemptionParameters.TxMaxTotalFee

		estimatedFee, err := EstimateRedemptionFee(
			rt.btcChain,
			redeemersOutputScripts,
			txMaxTotalFee,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot estimate redemption transaction fee: [%w]",
				err,
			)
		}

		fee = estimatedFee

		// The safe-minimum floor raises the total fee and therefore each
		// request's fee share. When a share exceeds the per-request maximum
		// fee, on-chain validation rejects the whole proposal and the batch is
		// aborted with no lower-fee retry. Emit a distinct warning so operators
		// can tell this apart from a generic validation failure. The largest
		// share is the worst case to check: the on-chain fee distribution
		// (see withRedemptionTotalFee) splits the total evenly and assigns the
		// division remainder to the last request, so that request pays
		// floor(total/count) + total%count. Checking only the even floor share
		// would miss a rejection caused solely by the remainder. txMaxFee is
		// the current governance parameter; the actually enforced cap is the
		// value snapshotted per request at creation, so this is a best-effort
		// diagnostic rather than an exact predictor.
		requestsCount := int64(len(redeemersOutputScripts))
		maxShare := fee/requestsCount + fee%requestsCount
		if uint64(maxShare) > txMaxFee {
			taskLogger.Warnf(
				"floored redemption fee share [%d] exceeds the per-request "+
					"maximum fee [%d]; the proposal will likely be rejected "+
					"by on-chain validation",
				maxShare,
				txMaxFee,
			)
		}
	}

	taskLogger.Infof("redemption transaction fee: [%d]", fee)

	proposal := &tbtc.RedemptionProposal{
		RedeemersOutputScripts: redeemersOutputScripts,
		RedemptionTxFee:        big.NewInt(fee),
	}

	taskLogger.Infof("validating the redemption proposal")

	if _, err := tbtc.ValidateRedemptionProposal(
		taskLogger,
		walletPublicKeyHash,
		proposal,
		rt.chain,
	); err != nil {
		return nil, fmt.Errorf("failed to verify redemption proposal: %v", err)
	}

	return proposal, nil
}

func findPendingRedemptions(
	fnLogger log.StandardLogger,
	chain Chain,
	walletPublicKeyHash [20]byte,
	currentBlockNumber uint64,
	requestsLimit uint16,
	requestTimeout uint32,
	requestMinAge uint32,
) ([]*RedemptionRequest, error) {
	// We are interested with `RedemptionRequested` events that are not
	// timed out yet. That means there is no sense to look for events that
	// occurred earlier than `now - requestTimeout`. However,
	// the event filter expects a block range while `requestTimeout`
	// is in seconds. To overcome that problem, we estimate the redemption
	// request timeout in blocks, using the average block time of the host chain.
	// Note that this estimation is not 100% accurate as the actual block time
	// may differ from the assumed one.
	requestTimeoutBlocks :=
		uint64(requestTimeout) / uint64(chain.AverageBlockTime().Seconds())
	// Then, we set the start block of the filter using the estimated redemption
	// request timeout in blocks. Note that if the actual average block time is
	// lesser than the assumed one, some events being on the edge of the block
	// range may be omitted. To avoid that, we make the block range a little
	// wider by using a constant factor of 1000 blocks.
	filterStartBlock := uint64(0)
	if filterLookbackBlocks := requestTimeoutBlocks + 1000; currentBlockNumber > filterLookbackBlocks {
		filterStartBlock = currentBlockNumber - filterLookbackBlocks
	}

	filter := &tbtc.RedemptionRequestedEventFilter{
		StartBlock: filterStartBlock,
	}
	if walletPublicKeyHash != [20]byte{} {
		filter.WalletPublicKeyHash = [][20]byte{walletPublicKeyHash}
	}

	events, err := chain.PastRedemptionRequestedEvents(filter)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past redemption requested events: [%w]",
			err,
		)
	}

	// Take the oldest first.
	sort.SliceStable(
		events, func(i, j int) bool {
			return events[i].BlockNumber < events[j].BlockNumber
		},
	)

	// There may be multiple events targeting the same redemption key
	// (i.e. the same wallet and output script pair). The Bridge contract
	// allows only for one pending request with the given redemption key
	// at the same time. That means we need to deduplicate the events list
	// and take only the latest event for the given redemption key.
	eventsSet := make(map[string]*tbtc.RedemptionRequestedEvent)
	for _, event := range events {
		redemptionKey, err := chain.BuildRedemptionKey(
			event.WalletPublicKeyHash,
			event.RedeemerOutputScript,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build redemption key: [%v]", err)
		}

		// Events are sorted from the oldest to the newest so there is no
		// need to check for existence. We can just overwrite.
		eventsSet[hexutils.Encode(redemptionKey.Bytes())] = event
	}

	fnLogger.Infof("found [%d] RedemptionRequested events", len(eventsSet))

	fnLogger.Infof("checking pending redemptions details")

	pendingRedemptions := make([]*RedemptionRequest, 0)

	eventIndex := 0
redemptionRequestedLoop:
	for redemptionKey, event := range eventsSet {
		eventIndex++

		fnLogger.Debugf(
			"getting pending redemption details [%s]",
			redemptionKey,
		)

		// Check if there is still a pending redemption for the given redemption
		// requested event.
		pendingRedemption, found, err := chain.GetPendingRedemptionRequest(
			event.WalletPublicKeyHash,
			event.RedeemerOutputScript,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get pending redemption request: [%w]",
				err,
			)
		}
		if !found {
			fnLogger.Infof(
				"redemption request [%s] is no longer pending",
				redemptionKey,
			)

			continue redemptionRequestedLoop
		}

		pendingRedemptions = append(
			pendingRedemptions, &RedemptionRequest{
				WalletPublicKeyHash:  event.WalletPublicKeyHash,
				RedemptionKey:        redemptionKey,
				RedeemerOutputScript: event.RedeemerOutputScript,
				RequestedAt:          pendingRedemption.RequestedAt,
				RequestedAmount:      pendingRedemption.RequestedAmount,
			},
		)
	}

	resultSliceCapacity := len(pendingRedemptions)
	if requestsLimit > 0 {
		resultSliceCapacity = int(requestsLimit)
	}

	// Sort the pending redemptions from oldest to newest.
	sort.SliceStable(
		pendingRedemptions, func(i, j int) bool {
			return pendingRedemptions[i].RequestedAt.Before(pendingRedemptions[j].RequestedAt)
		},
	)

	// Capture time now for computations.
	timeNow := time.Now()

	// Only redemption requests in range:
	// [now - requestTimeout, now - minAge]
	// should be taken into consideration.
	redemptionRequestsRangeStartTimestamp := timeNow.Add(
		-time.Duration(requestTimeout) * time.Second,
	)
	redemptionRequestsRangeEndTimestampFn := func(
		redemption *RedemptionRequest,
	) (time.Time, error) {
		delay, err := chain.GetRedemptionDelay(
			redemption.WalletPublicKeyHash,
			redemption.RedeemerOutputScript,
		)
		if err != nil {
			return time.Time{}, fmt.Errorf(
				"failed to get redemption delay: [%w]",
				err,
			)
		}

		minAge := time.Duration(requestMinAge) * time.Second
		if delay > minAge {
			minAge = delay
		}

		fnLogger.Infof(
			"minimum age for redemption request [%s] is [%v]",
			redemption.RedemptionKey,
			minAge,
		)

		return timeNow.Add(-minAge), nil
	}

	result := make([]*RedemptionRequest, 0, resultSliceCapacity)
	for _, pendingRedemption := range pendingRedemptions {
		if len(result) == cap(result) {
			break
		}

		// Check if timeout passed for the redemption request.
		if pendingRedemption.RequestedAt.Before(redemptionRequestsRangeStartTimestamp) {
			fnLogger.Infof(
				"redemption request [%s] has already timed out",
				pendingRedemption.RedemptionKey,
			)
			continue
		}

		rangeEndTimestamp, err := redemptionRequestsRangeEndTimestampFn(
			pendingRedemption,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get minimum age for redemption request [%s]: [%w]",
				pendingRedemption.RedemptionKey,
				err,
			)
		}

		// Check if enough time elapsed since the redemption request.
		if pendingRedemption.RequestedAt.After(rangeEndTimestamp) {
			fnLogger.Infof(
				"redemption request [%s] is not old enough",
				pendingRedemption.RedemptionKey,
			)
			continue
		}

		result = append(result, pendingRedemption)
	}

	return result, nil
}

// EstimateRedemptionFee estimates fee for the redemption transaction that pays
// the provided redeemers output scripts. The estimated fee is floored at a safe
// minimum rate and bounded above by txMaxTotalFee (the Bridge total-fee
// maximum), so a non-RBF redemption is not proposed below the floor where it
// could get stuck and jam the wallet. Only the total fee is bounded here; the
// per-request maximum fee is enforced separately by on-chain validation.
func EstimateRedemptionFee(
	btcChain bitcoin.Chain,
	redeemersOutputScripts []bitcoin.Script,
	txMaxTotalFee uint64,
) (int64, error) {
	sizeEstimator := bitcoin.NewTransactionSizeEstimator().
		// 1 P2WPKH main UTXO input.
		AddPublicKeyHashInputs(1, true).
		// 1 P2WPKH change output.
		AddPublicKeyHashOutputs(1, true)

	for _, script := range redeemersOutputScripts {
		switch bitcoin.GetScriptType(script) {
		case bitcoin.P2PKHScript:
			sizeEstimator.AddPublicKeyHashOutputs(1, false)
		case bitcoin.P2WPKHScript:
			sizeEstimator.AddPublicKeyHashOutputs(1, true)
		case bitcoin.P2SHScript:
			sizeEstimator.AddScriptHashOutputs(1, false)
		case bitcoin.P2WSHScript:
			sizeEstimator.AddScriptHashOutputs(1, true)
		case bitcoin.P2TRScript:
			// AddOutputScript rather than a dedicated Add*Outputs helper: an
			// output costs its script length plus a fixed header, and the real
			// script is already in hand, so no canonical placeholder is needed.
			sizeEstimator.AddOutputScript(script)
		default:
			return 0, fmt.Errorf(
				"non-standard redeemer output script type [%v]",
				bitcoin.GetScriptType(script),
			)
		}
	}

	transactionSize, err := sizeEstimator.VirtualSize()
	if err != nil {
		return 0, fmt.Errorf("cannot estimate transaction virtual size: [%v]", err)
	}

	feeEstimator := bitcoin.NewTransactionFeeEstimator(btcChain)

	totalFee, err := feeEstimator.EstimateFee(transactionSize)
	if err != nil {
		return 0, fmt.Errorf("cannot estimate transaction fee: [%v]", err)
	}

	// A raw estimate already above the Bridge maximum means the redemption is
	// uneconomical to perform at the required fee; return an error rather than
	// clamping to the maximum and broadcasting an underpriced transaction.
	if uint64(totalFee) > txMaxTotalFee {
		return 0, fmt.Errorf("estimated fee exceeds the maximum fee")
	}

	// Enforce the safe minimum fee rate and buffer, bounded by the Bridge
	// maximum.
	totalFee, err = applyWalletTxFeeFloor(totalFee, transactionSize, txMaxTotalFee)
	if err != nil {
		return 0, err
	}

	return totalFee, nil
}

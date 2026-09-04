package tbtcpg

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ipfs/go-log/v2"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ReservationReanchorLookBackBlocks is the look-back period in blocks used
// when searching for submitted reservation-related events. It is equal to
// 30 days assuming 12 seconds per block.
const ReservationReanchorLookBackBlocks = uint64(216000)

// ReservationReanchorTask is a task that may produce a reservation re-anchor
// proposal. The wallet enters this task when the source wallet has begun a
// move to a new wallet (state StateMovingFunds) or when the source wallet's
// main UTXO has dropped below the moving funds dust threshold (below-dust
// re-anchor). For every reservation currently custodied by the wallet, the
// task picks a destination wallet and assembles a 1-input-1-output re-anchor
// transaction moving the anchor outpoint into that destination wallet.
type ReservationReanchorTask struct {
	chain    Chain
	btcChain bitcoin.Chain

	// metricsRecorder is optional and used for recording performance
	// metrics: live_wallets_count, sourced from the GetLiveWalletsCount
	// chain call this task already makes.
	metricsRecorder interface {
		SetGauge(name string, value float64)
	}
}

// NewReservationReanchorTask returns a new ReservationReanchorTask bound to
// the given tbtc and Bitcoin chains.
func NewReservationReanchorTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *ReservationReanchorTask {
	return &ReservationReanchorTask{
		chain:    chain,
		btcChain: btcChain,
	}
}

// setMetricsRecorder sets the metrics recorder for the reservation
// re-anchor task.
func (rrt *ReservationReanchorTask) setMetricsRecorder(recorder interface {
	SetGauge(name string, value float64)
}) {
	rrt.metricsRecorder = recorder
}

// ActionType returns the type of wallet action this task produces.
func (rrt *ReservationReanchorTask) ActionType() tbtc.WalletActionType {
	return tbtc.ActionReservationReanchor
}

// Run evaluates whether the given wallet needs to re-anchor any of its
// reservations and returns a single ReservationReanchorProposal for the
// first reservation found to be re-anchorable. A wallet is a candidate for
// re-anchor only once it has entered the StateMovingFunds state (the
// wallet is migrating and reservations must be released to a live
// wallet); tbtc-v2's Reservation.requestReservationReanchor requires a
// privileged (governance) caller for StateLive sources
// (Reservation.sol:742-746, ReservationRouter.sol:269-277), which the
// client's ordinary operator key can never satisfy, so no below-dust
// re-anchor trigger is attempted for Live wallets.
//
// Once a MovingFunds wallet's reservations are fully drained, Run also
// checks whether its main UTXO has fallen below the moving funds dust
// threshold and, if so, notifies the Bridge so wallet closing can proceed
// (see notifyMovingFundsBelowDustIfEligible).
//
// Returns (nil, false, nil) when no reservation is eligible; callers should
// treat that as a benign no-op for the coordination window.
func (rrt *ReservationReanchorTask) Run(
	request *tbtc.CoordinationProposalRequest,
) (
	tbtc.CoordinationProposal,
	bool,
	error,
) {
	walletPublicKeyHash := request.WalletPublicKeyHash

	taskLogger := logger.With(
		zap.String("task", rrt.ActionType().String()),
		zap.String("walletPKH", fmt.Sprintf("0x%x", walletPublicKeyHash)),
	)

	walletChainData, err := rrt.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get wallet chain data: [%w]",
			err,
		)
	}

	// live_wallets_count is published unconditionally on every Run pass,
	// mirroring the sibling active_reservations_count/max_active_reservations
	// gauges that ReservationAcceptanceTask publishes every coordination
	// window: it must not depend on this task's StateMovingFunds/
	// non-empty-reservations guards below, which are false for most
	// wallets in steady state. Gating the publish on those guards would
	// leave the gauge stuck at its registered-zero value indefinitely and
	// make the occupancy-monitor ratio permanently undefined.
	liveWalletsCount, err := rrt.chain.GetLiveWalletsCount()
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get live wallets count: [%w]",
			err,
		)
	}
	if rrt.metricsRecorder != nil {
		rrt.metricsRecorder.SetGauge("live_wallets_count", float64(liveWalletsCount))
	}

	if walletChainData.State != tbtc.StateMovingFunds {
		taskLogger.Info("wallet is not eligible for reservation re-anchor")
		return nil, false, nil
	}

	reservationKeys, err := rrt.chain.WalletReservations(walletPublicKeyHash)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot list wallet reservations: [%w]",
			err,
		)
	}

	if len(reservationKeys) == 0 {
		taskLogger.Info("wallet has no reservations to re-anchor")
		rrt.notifyMovingFundsBelowDustIfEligible(taskLogger, walletPublicKeyHash)
		return nil, false, nil
	}

	if liveWalletsCount == 0 {
		taskLogger.Info("no live wallets available for re-anchor target")
		return nil, false, nil
	}

	targetWalletPublicKeyHash, err := rrt.findTargetWallet(
		taskLogger,
		walletPublicKeyHash,
	)
	if err != nil {
		taskLogger.Errorf(
			"cannot pick re-anchor target wallet: [%v]",
			err,
		)
		return nil, false, nil
	}

	for _, reservationKey := range reservationKeys {
		reservation, err := rrt.chain.GetReservation(reservationKey)
		if err != nil {
			taskLogger.Errorf(
				"cannot get reservation [0x%x]: [%v]",
				reservationKey,
				err,
			)
			continue
		}

		// Filter out reservations that are not in the Active state.
		// Note: Checking Active state already covers pending actions,
		// because a reservation with a pending action is in ActionPending state.
		if reservation.State != tbtc.ReservationStateActive {
			taskLogger.Infof(
				"reservation [0x%x] not in Active state (state=%v), skipping",
				reservationKey,
				reservation.State,
			)
			continue
		}

		proposal, err := rrt.ProposeReservationReanchor(
			taskLogger,
			walletPublicKeyHash,
			reservationKey,
			reservation.RequestNonce+1,
			targetWalletPublicKeyHash,
			0,
		)
		if err != nil {
			taskLogger.Errorf(
				"cannot prepare reservation re-anchor proposal: [%v]",
				err,
			)
			var postWriteErr *errReservationReanchorPostWriteFailure
			if errors.As(err, &postWriteErr) {
				// RequestReservationReanchor already authorized a
				// re-anchor action generation on-chain for this
				// reservation before the post-write post-condition check
				// (re-read + nonce verification) failed. Stop instead of
				// continuing to the next reservation so at most one
				// on-chain authorization is issued per Run pass.
				break
			}
			continue
		}

		return proposal, true, nil
	}

	taskLogger.Info("no reservations eligible for re-anchor")
	return nil, false, nil
}

// errReservationReanchorPostWriteFailure marks a ProposeReservationReanchor
// failure that occurred after RequestReservationReanchor already
// authorized a re-anchor action generation on-chain. Run distinguishes
// this from a pre-write failure (validation, fee estimation, transaction
// assembly) via errors.As: on a pre-write failure no authorization was
// issued, so Run may safely try the next reservation, but on a post-write
// failure an authorization is already in flight and Run must stop instead
// of risking a second one in the same pass.
type errReservationReanchorPostWriteFailure struct {
	err error
}

func (e *errReservationReanchorPostWriteFailure) Error() string {
	return e.err.Error()
}

func (e *errReservationReanchorPostWriteFailure) Unwrap() error {
	return e.err
}

// ProposeReservationReanchor assembles a single reservation re-anchor proposal
// for the given reservation, targeting the given wallet. The supplied fee may
// be 0 to trigger on-chain-driven fee estimation; the caller is responsible
// for providing a RequestNonce that is exactly current_request_nonce + 1 on
// the reservation's view (the action generation being authorized).
func (rrt *ReservationReanchorTask) ProposeReservationReanchor(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	reservationKey *big.Int,
	requestNonce uint64,
	targetWalletPublicKeyHash [20]byte,
	fee int64,
) (*tbtc.ReservationReanchorProposal, error) {
	if reservationKey == nil {
		return nil, fmt.Errorf("reservation key is required")
	}
	if requestNonce == 0 {
		return nil, fmt.Errorf("request nonce must be > 0")
	}
	if targetWalletPublicKeyHash == [20]byte{} {
		return nil, fmt.Errorf("target wallet public key hash is required")
	}

	taskLogger.Infof(
		"preparing a reservation re-anchor proposal for reservation [0x%x]",
		reservationKey,
	)

	reservation, err := rrt.chain.GetReservation(reservationKey)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation [0x%x]: [%w]",
			reservationKey,
			err,
		)
	}

	// convertReservationFromAbiType (the Go-side chain adapter) always
	// allocates a non-nil AnchorUtxo, populated with zero hash/value when
	// no anchor exists on-chain, so a bare nil check can never fire against
	// the production chain. Detect the unset case by value instead.
	if reservation.AnchorUtxo == nil ||
		reservation.AnchorUtxo.Value == 0 ||
		reservation.AnchorUtxo.Outpoint == nil ||
		reservation.AnchorUtxo.Outpoint.TransactionHash == (bitcoin.Hash{}) {
		return nil, fmt.Errorf(
			"reservation [0x%x] has no anchor UTXO",
			reservationKey,
		)
	}

	// The Bridge caps each reservation lifecycle transaction with its own
	// ReservationTxMaxFee, not the moving-funds TxMaxTotalFee, so we use
	// the reservation parameters directly. Fetched unconditionally (not
	// only when fee needs estimating) because the pre-check below also
	// needs ReservationTxMaxFee to bound-check a caller-supplied fee.
	params, err := rrt.chain.ReservationParameters()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation parameters: [%w]",
			err,
		)
	}

	if fee <= 0 {
		taskLogger.Infof("estimating reservation re-anchor transaction fee")

		fee, err = estimateReservationReanchorFee(
			rrt.btcChain,
			params.ReservationTxMaxFee,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot estimate reservation re-anchor transaction fee: [%w]",
				err,
			)
		}
	}

	taskLogger.Infof("reservation re-anchor transaction fee: [%d]", fee)

	feeBoundAction := &tbtc.ReservationAction{
		TxMaxFee: params.ReservationTxMaxFee,
	}

	if _, err := tbtc.AssembleReservationReanchorTransaction(
		rrt.btcChain,
		reservation.AnchorUtxo,
		targetWalletPublicKeyHash,
		feeBoundAction,
		fee,
	); err != nil {
		return nil, fmt.Errorf(
			"cannot assemble reservation re-anchor transaction: [%v]",
			err,
		)
	}

	proposal := &tbtc.ReservationReanchorProposal{
		ReservationKey:            new(big.Int).Set(reservationKey),
		RequestNonce:              requestNonce,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		ReanchorTxFee:             big.NewInt(fee),
	}

	if err := rrt.chain.ValidateReservationReanchorProposal(
		sourceWalletPublicKeyHash,
		proposal,
	); err != nil {
		return nil, fmt.Errorf(
			"failed to verify reservation re-anchor proposal: [%w]",
			err,
		)
	}
	// The re-anchor request generation must be authorized on-chain.
	// Note: Calling RequestReservationReanchor during proposal generation is an
	// accepted deviation from the read-only-during-generation pattern, with
	// precedent in MovingFundsTask's SubmitMovingFundsCommitment.
	if err := rrt.chain.RequestReservationReanchor(
		reservationKey,
		targetWalletPublicKeyHash,
	); err != nil {
		return nil, fmt.Errorf("cannot request reservation re-anchor: [%v]", err)
	}

	updatedReservation, err := rrt.chain.GetReservation(reservationKey)
	if err != nil {
		return nil, &errReservationReanchorPostWriteFailure{
			err: fmt.Errorf("cannot re-read reservation: [%v]", err),
		}
	}
	if updatedReservation.RequestNonce != requestNonce {
		return nil, &errReservationReanchorPostWriteFailure{
			err: fmt.Errorf(
				"reservation request nonce mismatch after request: predicted [%d], on-chain [%d]",
				requestNonce,
				updatedReservation.RequestNonce,
			),
		}
	}
	proposal.RequestNonce = updatedReservation.RequestNonce

	return proposal, nil
}

// findTargetWallet picks a live destination wallet from the on-chain wallet
// registry for the re-anchor transaction's output. The new wallet must be
// in StateLive and must not be the source wallet itself.
//
// Selection here is independent of the source wallet's own moving funds
// commitment (SubmitMovingFundsCommitment /
// PastMovingFundsCommitmentSubmittedEvents): this method does not attempt
// to route the reservation's anchor UTXO to one of the specific wallets
// the source wallet has committed to for its Bitcoin funds move. A
// reservation re-anchored here may therefore end up under different
// custody than the BTC the source wallet moves in the same window. This
// is a deliberate custody-scatter-not-theft tradeoff: every reservation
// stays fully accounted for on-chain regardless of which Live wallet
// holds its anchor, so scattering custody across an arbitrary Live wallet
// is a bookkeeping inconvenience, not a fund-safety issue. Selecting from
// the source wallet's actual commitment would require this task to parse
// and disambiguate among possibly several committed target wallets at
// proposal time; that is not worth building unless the chain interface
// already exposed the mapping trivially, which it does not today.
//
// The primary registration scan is bounded to
// ReservationReanchorLookBackBlocks (mirroring the other look-back scans
// in this package): an unbounded eth_getLogs scan on every re-anchor
// attempt is too expensive to run every window. GetLiveWalletsCount
// (checked by the caller before this method runs) can confirm live
// wallets exist even when none of them registered within the look-back
// window, so a bounded scan that finds no candidate falls back to an
// unbounded one instead of leaving Run stuck returning no proposal
// indefinitely.
func (rrt *ReservationReanchorTask) findTargetWallet(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
) ([20]byte, error) {
	blockCounter, err := rrt.chain.BlockCounter()
	if err != nil {
		return [20]byte{}, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return [20]byte{}, fmt.Errorf("failed to get current block: [%v]", err)
	}

	startBlock := uint64(0)
	if currentBlock > ReservationReanchorLookBackBlocks {
		startBlock = currentBlock - ReservationReanchorLookBackBlocks
	}

	targetWalletPublicKeyHash, err := rrt.findLiveWalletFromRegistrationEvents(
		taskLogger,
		sourceWalletPublicKeyHash,
		startBlock,
	)
	if err == nil {
		return targetWalletPublicKeyHash, nil
	}
	if startBlock == 0 {
		// The bounded scan above already covered full chain history.
		return [20]byte{}, err
	}

	taskLogger.Infof(
		"no live re-anchor target registered within the last [%d] blocks, "+
			"falling back to an unbounded registration event scan",
		ReservationReanchorLookBackBlocks,
	)

	return rrt.findLiveWalletFromRegistrationEvents(
		taskLogger,
		sourceWalletPublicKeyHash,
		0,
	)
}

// findLiveWalletFromRegistrationEvents scans new-wallet-registered events
// starting at startBlock and returns the most-recently-registered Live
// wallet other than sourceWalletPublicKeyHash.
func (rrt *ReservationReanchorTask) findLiveWalletFromRegistrationEvents(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	startBlock uint64,
) ([20]byte, error) {
	events, err := rrt.chain.PastNewWalletRegisteredEvents(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: startBlock},
	)
	if err != nil {
		return [20]byte{}, fmt.Errorf(
			"failed to get past new wallet registered events: [%v]",
			err,
		)
	}

	for i := len(events) - 1; i >= 0; i-- {
		walletPubKeyHash := events[i].WalletPublicKeyHash
		if walletPubKeyHash == sourceWalletPublicKeyHash {
			continue
		}

		wallet, err := rrt.chain.GetWallet(walletPubKeyHash)
		if err != nil {
			taskLogger.Errorf(
				"failed to get wallet data for wallet with PKH [0x%x]: [%v]",
				walletPubKeyHash,
				err,
			)
			continue
		}

		if wallet.State == tbtc.StateLive {
			return walletPubKeyHash, nil
		}
	}

	return [20]byte{}, fmt.Errorf("no live wallet available for re-anchor target")
}

// isBelowMovingFundsDustThreshold returns the wallet's resolved main UTXO
// (nil if it has none) and whether its value is below the moving funds
// dust threshold. The threshold is sourced from the on-chain
// MovingFundsParameters. A wallet without a main UTXO is considered to
// have fallen below the threshold.
func (rrt *ReservationReanchorTask) isBelowMovingFundsDustThreshold(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
) (*bitcoin.UnspentTransactionOutput, bool, error) {
	params, err := rrt.chain.GetMovingFundsParameters()
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get moving funds parameters: [%w]",
			err,
		)
	}

	walletChainData, err := rrt.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get wallet chain data: [%w]",
			err,
		)
	}

	if walletChainData.MainUtxoHash == [32]byte{} {
		// No main UTXO on-chain, the wallet has fully depleted its pool and
		// must release any reservation anchors.
		taskLogger.Info("wallet has no main UTXO; below dust threshold")
		return nil, true, nil
	}

	walletMainUtxo, err := tbtc.DetermineWalletMainUtxo(
		walletPublicKeyHash,
		rrt.chain,
		rrt.btcChain,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot determine wallet main UTXO: [%w]",
			err,
		)
	}

	if walletMainUtxo == nil {
		taskLogger.Info("wallet has no resolvable main UTXO; below dust threshold")
		return nil, true, nil
	}

	below := walletMainUtxo.Value < int64(params.DustThreshold)
	if below {
		taskLogger.Infof(
			"wallet main UTXO value [%d] below moving funds dust threshold [%d]",
			walletMainUtxo.Value,
			params.DustThreshold,
		)
	}
	return walletMainUtxo, below, nil
}

// notifyMovingFundsBelowDustIfEligible checks whether the given (just
// drained) MovingFunds wallet's main UTXO has fallen below the moving
// funds dust threshold and, if so, notifies the Bridge so wallet closing
// can proceed. m1-b-implementation.md §5 documents this as the only
// remaining route to close a wallet that proved its funds moved while it
// still held reservation anchors: the Bridge's own automatic closing
// attempt runs once, while the reservation count is still non-zero, and is
// never retried. Errors are logged rather than propagated: a failed
// notification here must not block the coordination window, and the wallet
// remains in StateMovingFunds so the next call to Run retries.
func (rrt *ReservationReanchorTask) notifyMovingFundsBelowDustIfEligible(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
) {
	mainUtxo, below, err := rrt.isBelowMovingFundsDustThreshold(
		taskLogger,
		walletPublicKeyHash,
	)
	if err != nil {
		taskLogger.Errorf(
			"cannot determine moving funds below-dust eligibility: [%v]",
			err,
		)
		return
	}
	if !below {
		return
	}

	if err := rrt.chain.NotifyMovingFundsBelowDust(
		walletPublicKeyHash,
		mainUtxo,
	); err != nil {
		taskLogger.Errorf(
			"cannot notify moving funds below dust: [%v]",
			err,
		)
		return
	}

	taskLogger.Info(
		"notified moving funds below dust; wallet has no remaining reservations",
	)
}

// estimateReservationReanchorFee estimates the fee for a reservation
// re-anchor transaction. The transaction has one P2WPKH input (the
// reservation anchor) and one P2WPKH output (the new anchor under the
// target wallet), so its virtual size is fixed for any single re-anchor.
func estimateReservationReanchorFee(
	btcChain bitcoin.Chain,
	txMaxFee uint64,
) (int64, error) {
	sizeEstimator := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddPublicKeyHashOutputs(1, true)

	return estimateReservationFixedSizeTxFee(
		btcChain,
		sizeEstimator,
		txMaxFee,
		"reservation re-anchor estimated fee exceeds the maximum fee",
	)
}

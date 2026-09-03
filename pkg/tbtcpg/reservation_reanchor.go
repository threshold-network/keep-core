package tbtcpg

import (
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

// ActionType returns the type of wallet action this task produces.
func (rrt *ReservationReanchorTask) ActionType() tbtc.WalletActionType {
	return tbtc.ActionReservationReanchor
}

// Run evaluates whether the given wallet needs to re-anchor any of its
// reservations and returns a single ReservationReanchorProposal for the first
// reservation found to be re-anchorable. A wallet is a candidate for re-anchor
// when either:
//   - it has entered the StateMovingFunds state (the wallet is migrating and
//     reservations must be released to a live wallet), or
//   - its main UTXO has dropped below the moving funds dust threshold (a
//     re-anchor frees value from the wallet's main UTXO pool into a fresh
//     reservation anchor held by another live wallet).
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

	migrating := walletChainData.State == tbtc.StateMovingFunds
	if !migrating {
		// Check the below-dust trigger. Re-anchor also unlocks wallet value
		// when the main UTXO has fallen below the moving funds dust
		// threshold, so wallets in StateLive may still need to re-anchor
		// before they enter the moving funds flow.
		needs, err := rrt.isBelowMovingFundsDustThreshold(
			taskLogger,
			walletPublicKeyHash,
		)
		if err != nil {
			return nil, false, err
		}

		if !needs {
			taskLogger.Info("wallet is not eligible for reservation re-anchor")
			return nil, false, nil
		}
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
		return nil, false, nil
	}

	liveWalletsCount, err := rrt.chain.GetLiveWalletsCount()
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get live wallets count: [%w]",
			err,
		)
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
			continue
		}

		return proposal, true, nil
	}

	taskLogger.Info("no reservations eligible for re-anchor")
	return nil, false, nil
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

	// Estimate fee if it's missing. The Bridge caps each reservation
	// lifecycle transaction with its own ReservationTxMaxFee, not the
	// moving-funds TxMaxTotalFee, so we use the reservation parameters
	// directly.
	if fee <= 0 {
		taskLogger.Infof("estimating reservation re-anchor transaction fee")

		params, err := rrt.chain.ReservationParameters()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get reservation parameters: [%w]",
				err,
			)
		}

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

	return proposal, nil
}

// findTargetWallet picks a live destination wallet from the on-chain wallet
// registry, mirroring the moving funds target selection. The new wallet must
// be in StateLive and must not be the source wallet itself. The registration
// scan is bounded to ReservationReanchorLookBackBlocks (mirroring the other
// look-back scans in this package) rather than the full chain history: a
// live wallet must have registered recently, and an unbounded eth_getLogs
// scan on every re-anchor attempt does not.
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

// isBelowMovingFundsDustThreshold returns true when the wallet's main UTXO
// value is below the moving funds dust threshold. The threshold is sourced
// from the on-chain MovingFundsParameters. A wallet without a main UTXO is
// considered to have fallen below the threshold.
func (rrt *ReservationReanchorTask) isBelowMovingFundsDustThreshold(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
) (bool, error) {
	params, err := rrt.chain.GetMovingFundsParameters()
	if err != nil {
		return false, fmt.Errorf(
			"cannot get moving funds parameters: [%w]",
			err,
		)
	}

	walletChainData, err := rrt.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf(
			"cannot get wallet chain data: [%w]",
			err,
		)
	}

	if walletChainData.MainUtxoHash == [32]byte{} {
		// No main UTXO on-chain, the wallet has fully depleted its pool and
		// must release any reservation anchors.
		taskLogger.Info("wallet has no main UTXO; below dust threshold")
		return true, nil
	}

	walletMainUtxo, err := tbtc.DetermineWalletMainUtxo(
		walletPublicKeyHash,
		rrt.chain,
		rrt.btcChain,
	)
	if err != nil {
		return false, fmt.Errorf(
			"cannot determine wallet main UTXO: [%w]",
			err,
		)
	}

	if walletMainUtxo == nil {
		taskLogger.Info("wallet has no resolvable main UTXO; below dust threshold")
		return true, nil
	}

	below := walletMainUtxo.Value < int64(params.DustThreshold)
	if below {
		taskLogger.Infof(
			"wallet main UTXO value [%d] below moving funds dust threshold [%d]",
			walletMainUtxo.Value,
			params.DustThreshold,
		)
	}
	return below, nil
}

// hasPendingAction reports whether the on-chain reservation action
// generation at the reservation's current request nonce (if any) is in a
// pending state. This guards against duplicate re-anchor requests: the
// Bridge rejects a new request while the previous generation is still in
// flight. The caller supplies the reservation record it already fetched
// (see Run) rather than this function re-reading it.
func hasPendingAction(
	reservationKey *big.Int,
	reservation *tbtc.Reservation,
	chain Chain,
	taskLogger log.StandardLogger,
) bool {
	if reservation.RequestNonce == 0 {
		return false
	}

	action, err := chain.GetReservationAction(
		reservationKey,
		reservation.RequestNonce,
	)
	if err != nil {
		// Fail safe: a lookup error is indistinguishable from "still
		// pending" here, and treating it as not-pending would let the
		// caller send a duplicate re-anchor request that the Bridge
		// rejects while a real pending generation is in flight. Skip this
		// reservation for the current coordination window instead; the
		// next window retries.
		taskLogger.Errorf(
			"cannot get reservation action for [0x%x] nonce [%d]: [%v]",
			reservationKey,
			reservation.RequestNonce,
			err,
		)
		return true
	}

	return action.State == tbtc.ReservationActionStatePending
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

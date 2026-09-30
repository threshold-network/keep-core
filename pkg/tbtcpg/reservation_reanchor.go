package tbtcpg

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-log/v2"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ReservationReanchorLookBackBlocks is the look-back period in blocks used
// when searching for submitted reservation-related events. It is equal to
// 30 days assuming 12 seconds per block.
const ReservationReanchorLookBackBlocks = uint64(216000)

// reservationReanchorRequestWaitBlocks bounds how many blocks
// waitForReservationReanchorRequestMined waits for a submitted
// RequestReservationReanchor transaction to be mined, mirroring
// MovingFundsTask.SubmitMovingFundsCommitment's bounded wait-then-re-read
// pattern.
const reservationReanchorRequestWaitBlocks = uint64(6)

// reservationRequestTimeoutSafetyMarginSeconds mirrors
// WalletProposalValidatorConstants.REQUEST_TIMEOUT_SAFETY_MARGIN from
// tbtc-v2's WalletProposalValidatorConstants.sol (2 hours). The on-chain
// proposal validators only sign a reservation action generation while
// block.timestamp < action.timeoutAt - margin, so both the acceptance and
// re-anchor tasks use the same margin to skip candidate generations whose
// signing window has closed instead of proposing them only to have the
// validator reject them.
const reservationRequestTimeoutSafetyMarginSeconds = 2 * 60 * 60

// ReservationReanchorTask is a task that may produce a reservation re-anchor
// proposal. The wallet enters this task when the source wallet has begun a
// move to a new wallet (state StateMovingFunds) or is closing
// (state StateClosing). For every reservation currently custodied by the
// wallet, the task picks a destination wallet and assembles a
// 1-input-1-output re-anchor transaction moving the anchor outpoint into
// that destination wallet.
type ReservationReanchorTask struct {
	chain    Chain
	btcChain bitcoin.Chain

	// metricsRecorder is optional and used for recording performance
	// metrics: live_wallets_count, sourced from the GetLiveWalletsCount
	// chain call this task already makes.
	metricsRecorder interface {
		SetGauge(name string, value float64)
	}

	// targetWalletCacheMutex guards cachedTargetWalletPublicKeyHash and
	// hasCachedTargetWallet: this task instance is shared across
	// concurrent Run calls for different source wallets (see
	// TestReservationReanchorTask_TargetWalletExclusion_SharedTask).
	targetWalletCacheMutex sync.Mutex
	// cachedTargetWalletPublicKeyHash is the most recently selected
	// re-anchor target wallet. reanchorTargetSearch tries it first in
	// every Run -- validating headroom and liveness only for this one
	// wallet -- instead of repeating its O(W) GetWallet-per-registration
	// scan on every re-anchor window; a registration scan only runs when
	// the cache is empty or the cached wallet is unusable. Meaningful only
	// when hasCachedTargetWallet is true.
	cachedTargetWalletPublicKeyHash [20]byte
	hasCachedTargetWallet           bool
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
// re-anchor once it has entered the StateMovingFunds or StateClosing state
// (the wallet is winding down and reservations must be released to a live
// wallet), matching the source states tbtc-v2's permissionless
// Reservation.requestReservationReanchor accepts. A wallet can reach
// Closing while it still holds reservations, and it cannot finish closing
// until they are gone, so a re-anchor missed during MovingFunds must still
// be possible from Closing. Live sources require a privileged
// (governance) caller on-chain, which the client's ordinary operator key
// can never satisfy, so Live wallets are skipped.
//
// Each reservation is either Active (needs a freshly requested generation)
// or already ActionPending with its current generation a Pending Reanchor
// action (a request this task issued in an earlier Run that only got as
// far as authorization, or one issued by another party): the latter is
// resumed at its real nonce and authorized target rather than skipped, so
// an in-flight authorization is not abandoned and re-requested.
//
// A pass sends at most one RequestReservationReanchor: once a request has
// been sent, any later failure ends the pass without a proposal.
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
	// window: it must not depend on this task's wallet-state/
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

	if walletChainData.State != tbtc.StateMovingFunds &&
		walletChainData.State != tbtc.StateClosing {
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
		// This duty stays embedded in Run() rather than becoming its own
		// dedicated watcher: a genuine dedicated watcher needs independent
		// scheduling wired up wherever coordination tasks are registered
		// (outside this package), which is a larger cross-package change
		// than justified here, whereas embedding it costs only piggybacking
		// on this task's own already-scheduled invocation cadence. A
		// Closing wallet is already past the moving funds process, so the
		// notification only applies to MovingFunds wallets.
		if walletChainData.State == tbtc.StateMovingFunds {
			rrt.notifyMovingFundsBelowDustIfEligible(taskLogger, walletPublicKeyHash)
		}
		return nil, false, nil
	}

	if liveWalletsCount == 0 {
		taskLogger.Info("no live wallets available for re-anchor target")
		return nil, false, nil
	}

	params, err := rrt.chain.ReservationParameters()
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get reservation parameters: [%w]",
			err,
		)
	}

	// maxReservationsAmountPerWallet is fetched once per Run: it is a
	// governance-set cap, stable across a single coordination window, and
	// every target-headroom check below (across every reservation and
	// every candidate wallet) uses the same value.
	maxReservationsAmountPerWallet, _, err := rrt.chain.ReservationCaps()
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get reservation caps: [%w]",
			err,
		)
	}

	// One target search serves every reservation of this pass, so
	// registrations and wallet states are read at most once per pass.
	targets := rrt.newReanchorTargetSearch(
		taskLogger,
		walletPublicKeyHash,
		params.MaxReservationsPerWallet,
		maxReservationsAmountPerWallet,
	)

reservationLoop:
	for _, reservationKey := range reservationKeys {
		reservation, err := rrt.chain.GetReservation(reservationKey)
		if err != nil {
			return nil, false, fmt.Errorf(
				"cannot get reservation [0x%x]: [%w]",
				reservationKey,
				err,
			)
		}

		switch reservation.State {
		case tbtc.ReservationStateActionPending:
			// Resume path: this generation was already authorized (by
			// this task in an earlier Run, or by another party) and must
			// not be re-requested -- Reservation.requestReservationReanchor
			// requires the Active state, so a second request against an
			// ActionPending reservation would revert anyway. Only a
			// pending Reanchor generation is resumable here; any other
			// pending action type (e.g. a pending Acceptance action) is
			// outside this task's remit.
			action, err := rrt.chain.GetReservationAction(
				reservationKey,
				reservation.RequestNonce,
			)
			if err != nil {
				taskLogger.Errorf(
					"cannot get reservation action for [0x%x] nonce [%d]: [%v]",
					reservationKey,
					reservation.RequestNonce,
					err,
				)
				continue reservationLoop
			}

			if action.ActionType != tbtc.ReservationActionTypeReanchor ||
				action.State != tbtc.ReservationActionStatePending {
				taskLogger.Infof(
					"reservation [0x%x] has a pending action that is not a "+
						"resumable re-anchor (type=%v, state=%v), skipping",
					reservationKey,
					action.ActionType,
					action.State,
				)
				continue reservationLoop
			}

			// Mirror WalletProposalValidator.validateReservationReanchorProposal's
			// timeout safety margin gate: the validator only signs while
			// block.timestamp < action.TimeoutAt -
			// REQUEST_TIMEOUT_SAFETY_MARGIN, so a generation at or past
			// that boundary is skipped rather than proposed and rejected;
			// a fresh request may be issued instead once the reservation
			// is back in the Active state.
			if uint64(time.Now().Unix())+
				uint64(reservationRequestTimeoutSafetyMarginSeconds) >=
				uint64(action.TimeoutAt) {
				taskLogger.Infof(
					"reservation [0x%x] pending re-anchor generation "+
						"[%d] timeout [%d] is at or past the timeout "+
						"safety margin; skipping this generation",
					reservationKey,
					reservation.RequestNonce,
					action.TimeoutAt,
				)
				continue reservationLoop
			}

			taskLogger.Infof(
				"resuming pending reservation re-anchor for [0x%x] at nonce [%d]",
				reservationKey,
				reservation.RequestNonce,
			)

			proposal, err := rrt.ProposeReservationReanchor(
				taskLogger,
				walletPublicKeyHash,
				reservationKey,
				action.TargetWalletPublicKeyHash,
				0,
			)
			if err != nil {
				// The resume path issues no chain write of its own (the
				// generation was already authorized by a prior request),
				// so any failure here is read-only and safe to retry on
				// the next round: skip this reservation for this pass.
				taskLogger.Errorf(
					"cannot resume reservation re-anchor proposal: [%v]",
					err,
				)
				continue reservationLoop
			}

			return proposal, true, nil

		case tbtc.ReservationStateActive:
			if reservation.AnchorUtxo == nil || reservation.AnchorUtxo.Value <= 0 {
				taskLogger.Errorf(
					"reservation [0x%x] has no positive anchor value, skipping",
					reservationKey,
				)
				continue reservationLoop
			}
			anchorValue := uint64(reservation.AnchorUtxo.Value)

			// Mirror Reservation.sol's request-time gates for a
			// permissionless caller, which would otherwise revert the
			// request after a target search: no request before
			// reanchorCooldownUntil, and the anchor must stay above
			// txMaxFee + minAmount.
			if uint64(time.Now().Unix()) < uint64(reservation.ReanchorCooldownUntil) {
				taskLogger.Infof(
					"reservation [0x%x] is in its re-anchor cooldown until "+
						"[%d], skipping",
					reservationKey,
					reservation.ReanchorCooldownUntil,
				)
				continue reservationLoop
			}
			if anchorValue <= params.ReservationTxMaxFee+params.ReservationMinAmount {
				taskLogger.Infof(
					"reservation [0x%x] anchor [%d] is not above the "+
						"re-anchor floor (tx max fee [%d] + min amount [%d]), "+
						"skipping",
					reservationKey,
					anchorValue,
					params.ReservationTxMaxFee,
					params.ReservationMinAmount,
				)
				continue reservationLoop
			}

			for {
				target, err := targets.find(anchorValue)
				if err != nil {
					if errors.Is(err, errNoLiveTargetWallet) {
						taskLogger.Infof(
							"no live re-anchor target wallet with headroom "+
								"available for reservation [0x%x]",
							reservationKey,
						)
						continue reservationLoop
					}
					return nil, false, fmt.Errorf(
						"cannot pick re-anchor target wallet: [%w]",
						err,
					)
				}

				proposal, err := rrt.ProposeReservationReanchor(
					taskLogger,
					walletPublicKeyHash,
					reservationKey,
					target,
					0,
				)
				if err == nil {
					return proposal, true, nil
				}

				taskLogger.Errorf(
					"cannot prepare reservation re-anchor proposal: [%v]",
					err,
				)

				var postSubmitErr *errReservationReanchorPostSubmitFailure
				if errors.As(err, &postSubmitErr) {
					// The request was sent. End the pass so it sends at
					// most one request; a later round picks the request
					// up from chain state (resume path once mined, a new
					// request if the reservation is still Active).
					return nil, false, nil
				}

				if isReservationCapRevertError(err) {
					// The target's capacity changed since our headroom
					// pre-check (a concurrent re-anchor or acceptance
					// consumed it); exclude it and try the next candidate
					// for this same reservation instead of giving up.
					targets.exclude(target)
					continue
				}

				// Nothing was sent (fee estimation, assembly, or another
				// request revert), so no chain state changed: skip this
				// reservation this pass.
				continue reservationLoop
			}

		default:
			taskLogger.Infof(
				"reservation [0x%x] not in a re-anchorable state (state=%v), skipping",
				reservationKey,
				reservation.State,
			)
		}
	}

	taskLogger.Info("no reservations eligible for re-anchor")
	return nil, false, nil
}

// isReservationCapRevertError reports whether err is a reservation
// wallet-capacity revert from either RequestReservationReanchor or
// ValidateReservationReanchorProposal, mirroring the two capacity
// require() reasons in Reservation.sol's requestReservationReanchor and
// WalletProposalValidator.sol's validateReservationReanchorProposal
// ("Wallet reservations cap exceeded", "Wallet reserved amount cap
// exceeded"). Both end in the same suffix, so a single substring check
// covers both without needing separate matches.
func isReservationCapRevertError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cap exceeded")
}

// errReservationReanchorPostSubmitFailure marks a ProposeReservationReanchor
// failure that happened after RequestReservationReanchor was sent. Run
// checks for it with errors.As and ends the pass instead of moving on to
// another target or reservation, so a pass sends at most one re-anchor
// request. A failure before the request was sent leaves no chain state
// behind and is not wrapped.
type errReservationReanchorPostSubmitFailure struct {
	err error
}

func (e *errReservationReanchorPostSubmitFailure) Error() string {
	return e.err.Error()
}

func (e *errReservationReanchorPostSubmitFailure) Unwrap() error {
	return e.err
}

// ProposeReservationReanchor assembles a single reservation re-anchor
// proposal for the given reservation, targeting the given wallet.
//
// If the reservation's current generation is not already a Pending
// Reanchor action authorizing this exact target, this call first requests
// one on-chain (RequestReservationReanchor) and waits for it to be mined
// before re-reading the reservation and action for the real request nonce;
// only then does it build and validate the proposal. This ordering is
// required by the on-chain validator, which requires the action at
// proposal.RequestNonce to already be a Pending Reanchor --
// Reservation.sol's requestReservationReanchor is what writes that record,
// so validating first would read the zero action and always revert.
//
// Before sending the request, the fee is estimated and the transaction is
// assembled against the live ReservationTxMaxFee, the value Solidity
// snapshots into the action the request creates. A fee that cannot fit
// therefore fails before anything is sent instead of leaving an unusable
// request on-chain. Any failure after the request was sent is returned as
// an *errReservationReanchorPostSubmitFailure.
//
// A caller resuming an already-Pending generation (Run does this for
// reservations in ActionPending state) passes that generation's own
// authorized target; the fee is then checked against the action's own
// snapshotted TxMaxFee and no chain write happens.
//
// The supplied fee may be 0 to trigger on-chain-driven fee estimation.
func (rrt *ReservationReanchorTask) ProposeReservationReanchor(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
	fee int64,
) (*tbtc.ReservationReanchorProposal, error) {
	if reservationKey == nil {
		return nil, fmt.Errorf("reservation key is required")
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

	// Determine whether the reservation's current generation is already
	// the exact Pending Reanchor action this proposal is meant to carry
	// forward. A resumed generation is used as-is with no further chain
	// write; anything else means a new generation must be requested and
	// confirmed before this method can read its snapshot.
	action, err := rrt.chain.GetReservationAction(reservationKey, reservation.RequestNonce)
	resuming := err == nil &&
		action != nil &&
		action.ActionType == tbtc.ReservationActionTypeReanchor &&
		action.State == tbtc.ReservationActionStatePending &&
		action.TargetWalletPublicKeyHash == targetWalletPublicKeyHash

	if resuming {
		// The fee bound is the action's own snapshotted txMaxFee:
		// WalletProposalValidator.validateReservationReanchorProposal
		// checks the proposed fee against action.txMaxFee, not the live
		// parameter.
		fee, err = rrt.prepareReservationReanchorTransaction(
			taskLogger,
			reservation.AnchorUtxo,
			targetWalletPublicKeyHash,
			action.TxMaxFee,
			fee,
		)
		if err != nil {
			return nil, err
		}

		return rrt.validatedReservationReanchorProposal(
			taskLogger,
			sourceWalletPublicKeyHash,
			reservationKey,
			reservation.RequestNonce,
			targetWalletPublicKeyHash,
			fee,
		)
	}

	if reservation.State != tbtc.ReservationStateActive {
		return nil, fmt.Errorf(
			"reservation [0x%x] is not active and has no matching "+
				"pending re-anchor action to resume",
			reservationKey,
		)
	}

	params, err := rrt.chain.ReservationParameters()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation parameters: [%w]",
			err,
		)
	}

	fee, err = rrt.prepareReservationReanchorTransaction(
		taskLogger,
		reservation.AnchorUtxo,
		targetWalletPublicKeyHash,
		params.ReservationTxMaxFee,
		fee,
	)
	if err != nil {
		return nil, err
	}

	taskLogger.Infof(
		"requesting reservation re-anchor for [0x%x] to wallet [0x%x]",
		reservationKey,
		targetWalletPublicKeyHash,
	)

	if err := rrt.chain.RequestReservationReanchor(
		reservationKey,
		targetWalletPublicKeyHash,
	); err != nil {
		return nil, fmt.Errorf("cannot request reservation re-anchor: [%w]", err)
	}

	minedReservation, err := rrt.waitForReservationReanchorRequestMined(
		taskLogger,
		reservationKey,
		targetWalletPublicKeyHash,
		reservation.RequestNonce,
	)
	if err != nil {
		return nil, &errReservationReanchorPostSubmitFailure{
			err: fmt.Errorf(
				"reservation re-anchor request not confirmed: [%w]",
				err,
			),
		}
	}

	proposal, err := rrt.validatedReservationReanchorProposal(
		taskLogger,
		sourceWalletPublicKeyHash,
		reservationKey,
		minedReservation.RequestNonce,
		targetWalletPublicKeyHash,
		fee,
	)
	if err != nil {
		return nil, &errReservationReanchorPostSubmitFailure{err: err}
	}

	return proposal, nil
}

// prepareReservationReanchorTransaction estimates the re-anchor fee when
// fee is not positive, and checks that the re-anchor transaction can be
// assembled with that fee under txMaxFee. It returns the fee to propose.
func (rrt *ReservationReanchorTask) prepareReservationReanchorTransaction(
	taskLogger log.StandardLogger,
	anchorUtxo *bitcoin.UnspentTransactionOutput,
	targetWalletPublicKeyHash [20]byte,
	txMaxFee uint64,
	fee int64,
) (int64, error) {
	if fee <= 0 {
		taskLogger.Infof("estimating reservation re-anchor transaction fee")

		var err error
		fee, err = estimateReservationReanchorFee(rrt.btcChain, txMaxFee)
		if err != nil {
			return 0, fmt.Errorf(
				"cannot estimate reservation re-anchor transaction fee: [%w]",
				err,
			)
		}
	}

	taskLogger.Infof("reservation re-anchor transaction fee: [%d]", fee)

	if _, err := tbtc.AssembleReservationReanchorTransaction(
		rrt.btcChain,
		anchorUtxo,
		targetWalletPublicKeyHash,
		&tbtc.ReservationAction{TxMaxFee: txMaxFee},
		fee,
	); err != nil {
		return 0, fmt.Errorf(
			"cannot assemble reservation re-anchor transaction: [%v]",
			err,
		)
	}

	return fee, nil
}

// validatedReservationReanchorProposal builds the proposal for the given
// generation and validates it on-chain.
func (rrt *ReservationReanchorTask) validatedReservationReanchorProposal(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	reservationKey *big.Int,
	requestNonce uint64,
	targetWalletPublicKeyHash [20]byte,
	fee int64,
) (*tbtc.ReservationReanchorProposal, error) {
	proposal := &tbtc.ReservationReanchorProposal{
		ReservationKey:            new(big.Int).Set(reservationKey),
		RequestNonce:              requestNonce,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		ReanchorTxFee:             big.NewInt(fee),
	}

	taskLogger.Infof("validating the reservation re-anchor proposal")

	if err := rrt.chain.ValidateReservationReanchorProposal(
		sourceWalletPublicKeyHash,
		proposal,
	); err != nil {
		return nil, fmt.Errorf(
			"failed to verify reservation re-anchor proposal: [%w]",
			err,
		)
	}

	return proposal, nil
}

// waitForReservationReanchorRequestMined polls chain state for up to
// reservationReanchorRequestWaitBlocks blocks, waiting for a just-submitted
// RequestReservationReanchor transaction to be mined: the reservation's
// RequestNonce must have advanced past preRequestNonce, and the action at
// the new nonce must be the Pending Reanchor generation authorizing
// targetWalletPublicKeyHash. If the nonce advanced to any other action,
// another request was mined first and this one can only revert, so the
// wait ends immediately with an error.
func (rrt *ReservationReanchorTask) waitForReservationReanchorRequestMined(
	taskLogger log.StandardLogger,
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
	preRequestNonce uint64,
) (*tbtc.Reservation, error) {
	blockCounter, err := rrt.chain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf("error getting block counter: [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, fmt.Errorf("error getting current block: [%w]", err)
	}

	for blockHeight := currentBlock + 1; blockHeight <= currentBlock+reservationReanchorRequestWaitBlocks; blockHeight++ {
		if err := blockCounter.WaitForBlockHeight(blockHeight); err != nil {
			return nil, fmt.Errorf("error while waiting for block height: [%w]", err)
		}

		reservation, err := rrt.chain.GetReservation(reservationKey)
		if err != nil {
			taskLogger.Errorf(
				"cannot re-read reservation [0x%x]: [%v]",
				reservationKey,
				err,
			)
			continue
		}

		if reservation.RequestNonce > preRequestNonce {
			action, err := rrt.chain.GetReservationAction(
				reservationKey,
				reservation.RequestNonce,
			)
			if err != nil {
				taskLogger.Errorf(
					"cannot get reservation action for [0x%x] nonce [%d]: [%v]",
					reservationKey,
					reservation.RequestNonce,
					err,
				)
				continue
			}

			if action.ActionType != tbtc.ReservationActionTypeReanchor ||
				action.State != tbtc.ReservationActionStatePending ||
				action.TargetWalletPublicKeyHash != targetWalletPublicKeyHash {
				return nil, fmt.Errorf(
					"reservation [0x%x] advanced to nonce [%d] with a "+
						"different action (type=%v, state=%v, target=0x%x); "+
						"another request was mined first",
					reservationKey,
					reservation.RequestNonce,
					action.ActionType,
					action.State,
					action.TargetWalletPublicKeyHash,
				)
			}

			taskLogger.Infof(
				"reservation re-anchor request for [0x%x] confirmed at "+
					"block [%d], nonce [%d]",
				reservationKey,
				blockHeight,
				reservation.RequestNonce,
			)
			return reservation, nil
		}

		taskLogger.Infof(
			"reservation re-anchor request for [0x%x] still not confirmed "+
				"at block [%d]",
			reservationKey,
			blockHeight,
		)
	}

	return nil, fmt.Errorf(
		"reservation re-anchor request for [0x%x] not confirmed within "+
			"[%d] blocks",
		reservationKey,
		reservationReanchorRequestWaitBlocks,
	)
}

// reanchorTargetSearch picks live destination wallets for the re-anchor
// requests of one Run pass. A target must be in StateLive, must not be the
// source wallet itself, and must have room under maxReservationsPerWallet /
// maxReservationsAmountPerWallet for one more reservation of the anchor
// value -- mirroring the capacity Reservation.sol's
// requestReservationReanchor reserves on the target at request time.
//
// The search keeps its state for the whole pass and is local to one Run
// call (the task instance is shared across concurrent Runs for different
// source wallets). Registration events and wallet states are read at most
// once per pass, lazily on the first reservation that needs a target, and
// capacity reads are not repeated for wallets or anchor values already
// ruled out. Nothing in a pass releases target capacity, and a pass ends
// after its first request, so what was ruled out stays ruled out.
//
// The previously selected target wallet is cached across passes and is
// validated first; the registration scan only runs when the cached wallet
// is missing or unusable. The scan is bounded to
// ReservationReanchorLookBackBlocks (mirroring the other look-back scans in
// this package) and falls back to an unbounded scan only when no recently
// registered wallet has headroom: GetLiveWalletsCount (checked by Run) can
// confirm live wallets exist even when none of them registered within the
// look-back window. The fallback skips wallets the bounded scan already
// read.
//
// Selection here is independent of the source wallet's own moving funds
// commitment (SubmitMovingFundsCommitment /
// PastMovingFundsCommitmentSubmittedEvents): the search does not attempt
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
type reanchorTargetSearch struct {
	rrt                            *ReservationReanchorTask
	logger                         log.StandardLogger
	sourceWalletPublicKeyHash      [20]byte
	maxReservationsPerWallet       uint32
	maxReservationsAmountPerWallet uint64

	// checked holds every wallet whose state was already read this pass.
	checked map[[20]byte]bool
	// excluded holds wallets ruled out for every anchor value this pass:
	// count cap reached, or rejected by a request-time capacity revert.
	excluded map[[20]byte]bool
	// candidates are the Live wallets found so far, in selection order:
	// the cached wallet first, then newest registration first.
	candidates [][20]byte

	cachedChecked bool
	recentScanned bool
	fullScanned   bool

	// noHeadroomFromAnchor is the smallest anchor value no candidate had
	// headroom for; zero means no such value is known yet. The amount
	// check only gets harder as the anchor grows, so every anchor at or
	// above it is ruled out too.
	noHeadroomFromAnchor uint64
}

// newReanchorTargetSearch returns a target search for one Run pass. It
// reads nothing until find is first called.
func (rrt *ReservationReanchorTask) newReanchorTargetSearch(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	maxReservationsPerWallet uint32,
	maxReservationsAmountPerWallet uint64,
) *reanchorTargetSearch {
	return &reanchorTargetSearch{
		rrt:                            rrt,
		logger:                         taskLogger,
		sourceWalletPublicKeyHash:      sourceWalletPublicKeyHash,
		maxReservationsPerWallet:       maxReservationsPerWallet,
		maxReservationsAmountPerWallet: maxReservationsAmountPerWallet,
		checked:                        make(map[[20]byte]bool),
		excluded:                       make(map[[20]byte]bool),
	}
}

// errNoLiveTargetWallet signals that no eligible re-anchor destination
// wallet was found -- either no wallet is Live at all, or every candidate
// lacks count/amount headroom -- a legitimate "nothing to do yet" outcome,
// not a chain-read failure. Run treats it as a benign no-op for the
// current reservation; any other error returned by the target search is a
// genuine RPC/chain-read failure and is propagated so the coordinator
// retries.
var errNoLiveTargetWallet = errors.New("no live wallet available for re-anchor target")

// find returns a target wallet with headroom for anchorValue, loading more
// candidates only when the ones already found have none.
func (s *reanchorTargetSearch) find(anchorValue uint64) ([20]byte, error) {
	if s.noHeadroomFromAnchor != 0 && anchorValue >= s.noHeadroomFromAnchor {
		return [20]byte{}, errNoLiveTargetWallet
	}

	// Candidates already checked for this anchor value are not checked
	// again after more candidates are loaded.
	from := 0
	for {
		target, found, err := s.pick(anchorValue, from)
		if err != nil {
			return [20]byte{}, err
		}
		if found {
			s.rrt.setCachedTargetWallet(target)
			return target, nil
		}

		from = len(s.candidates)
		loaded, err := s.loadMoreCandidates()
		if err != nil {
			return [20]byte{}, err
		}
		if !loaded {
			s.noHeadroomFromAnchor = anchorValue
			return [20]byte{}, errNoLiveTargetWallet
		}
	}
}

// exclude rules target out for the rest of the pass, after a request-time
// capacity revert showed its capacity changed since the headroom check.
func (s *reanchorTargetSearch) exclude(target [20]byte) {
	s.excluded[target] = true
	s.rrt.evictCachedTargetWallet(target)
}

// pick returns the first candidate, starting at index from, with headroom
// for anchorValue. A candidate whose count cap is reached is excluded for
// the rest of the pass.
func (s *reanchorTargetSearch) pick(
	anchorValue uint64,
	from int,
) ([20]byte, bool, error) {
	for _, candidate := range s.candidates[from:] {
		if s.excluded[candidate] {
			continue
		}

		count, err := s.rrt.chain.WalletReservationsCount(candidate)
		if err != nil {
			return [20]byte{}, false, fmt.Errorf(
				"cannot get wallet reservations count: [%w]",
				err,
			)
		}
		if count+1 > s.maxReservationsPerWallet {
			s.logger.Infof(
				"candidate re-anchor target wallet [0x%x] has no count "+
					"headroom, skipping",
				candidate,
			)
			s.exclude(candidate)
			continue
		}

		// Zero disables the amount cap, matching Solidity's
		// `maxReservationsAmountPerWallet == 0` convention.
		if s.maxReservationsAmountPerWallet > 0 {
			amount, err := s.rrt.chain.WalletReservationsAmount(candidate)
			if err != nil {
				return [20]byte{}, false, fmt.Errorf(
					"cannot get wallet reservations amount: [%w]",
					err,
				)
			}
			if amount+anchorValue > s.maxReservationsAmountPerWallet {
				s.logger.Infof(
					"candidate re-anchor target wallet [0x%x] has no amount "+
						"headroom for [%d] satoshi, skipping",
					candidate,
					anchorValue,
				)
				s.rrt.evictCachedTargetWallet(candidate)
				continue
			}
		}

		return candidate, true, nil
	}

	return [20]byte{}, false, nil
}

// loadMoreCandidates adds the next batch of Live wallets to the
// candidates: the cached wallet, then the wallets registered within the
// look-back window, then every other registered wallet. It reports false
// once there is nothing left to load.
func (s *reanchorTargetSearch) loadMoreCandidates() (bool, error) {
	if !s.cachedChecked {
		s.cachedChecked = true
		if cached, ok := s.rrt.cachedTargetWallet(s.sourceWalletPublicKeyHash); ok {
			if !s.addCandidateIfLive(cached) {
				s.logger.Infof(
					"cached re-anchor target wallet [0x%x] is no longer "+
						"valid; scanning for a new one",
					cached,
				)
				s.rrt.evictCachedTargetWallet(cached)
			}
			return true, nil
		}
	}

	if !s.recentScanned {
		s.recentScanned = true

		blockCounter, err := s.rrt.chain.BlockCounter()
		if err != nil {
			return false, fmt.Errorf("failed to get block counter: [%v]", err)
		}
		currentBlock, err := blockCounter.CurrentBlock()
		if err != nil {
			return false, fmt.Errorf("failed to get current block: [%v]", err)
		}

		startBlock := uint64(0)
		if currentBlock > ReservationReanchorLookBackBlocks {
			startBlock = currentBlock - ReservationReanchorLookBackBlocks
		}
		if startBlock == 0 {
			// The bounded scan already covers full chain history.
			s.fullScanned = true
		}

		return true, s.addRegisteredCandidates(startBlock)
	}

	if !s.fullScanned {
		s.fullScanned = true

		s.logger.Infof(
			"no live re-anchor target with headroom registered within the "+
				"last [%d] blocks, falling back to an unbounded registration "+
				"event scan",
			ReservationReanchorLookBackBlocks,
		)

		return true, s.addRegisteredCandidates(0)
	}

	return false, nil
}

// addRegisteredCandidates scans new-wallet-registered events from
// startBlock and adds, newest first, the Live wallets not read yet.
func (s *reanchorTargetSearch) addRegisteredCandidates(startBlock uint64) error {
	events, err := s.rrt.chain.PastNewWalletRegisteredEvents(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: startBlock},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past new wallet registered events: [%v]",
			err,
		)
	}

	for i := len(events) - 1; i >= 0; i-- {
		s.addCandidateIfLive(events[i].WalletPublicKeyHash)
	}

	return nil
}

// addCandidateIfLive reads the wallet's state, unless it was already read
// this pass, and adds it to the candidates if it is Live and not the
// source wallet. It reports whether the wallet was added.
func (s *reanchorTargetSearch) addCandidateIfLive(walletPublicKeyHash [20]byte) bool {
	if walletPublicKeyHash == s.sourceWalletPublicKeyHash ||
		s.checked[walletPublicKeyHash] {
		return false
	}
	s.checked[walletPublicKeyHash] = true

	wallet, err := s.rrt.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		s.logger.Errorf(
			"failed to get wallet data for wallet with PKH [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
		return false
	}
	if wallet.State != tbtc.StateLive {
		return false
	}

	s.candidates = append(s.candidates, walletPublicKeyHash)
	return true
}

// cachedTargetWallet returns the cached re-anchor target wallet, if any,
// still usable for sourceWalletPublicKeyHash. A cached wallet that now
// equals the requesting source wallet itself (e.g. a former target has
// since entered MovingFunds and become a source in its own right) is
// reported as absent so the caller falls back to a fresh scan.
func (rrt *ReservationReanchorTask) cachedTargetWallet(
	sourceWalletPublicKeyHash [20]byte,
) ([20]byte, bool) {
	rrt.targetWalletCacheMutex.Lock()
	defer rrt.targetWalletCacheMutex.Unlock()

	if !rrt.hasCachedTargetWallet {
		return [20]byte{}, false
	}
	if rrt.cachedTargetWalletPublicKeyHash == sourceWalletPublicKeyHash {
		return [20]byte{}, false
	}
	return rrt.cachedTargetWalletPublicKeyHash, true
}

// setCachedTargetWallet records the most recently selected re-anchor
// target wallet for reuse by future Run passes.
func (rrt *ReservationReanchorTask) setCachedTargetWallet(
	targetWalletPublicKeyHash [20]byte,
) {
	rrt.targetWalletCacheMutex.Lock()
	defer rrt.targetWalletCacheMutex.Unlock()

	rrt.cachedTargetWalletPublicKeyHash = targetWalletPublicKeyHash
	rrt.hasCachedTargetWallet = true
}

// evictCachedTargetWallet clears the cached re-anchor target wallet if it
// currently equals target, so the next Run pass performs a fresh scan
// instead of reusing a wallet just proven to have no headroom or to have
// reverted on capacity.
func (rrt *ReservationReanchorTask) evictCachedTargetWallet(target [20]byte) {
	rrt.targetWalletCacheMutex.Lock()
	defer rrt.targetWalletCacheMutex.Unlock()

	if rrt.hasCachedTargetWallet && rrt.cachedTargetWalletPublicKeyHash == target {
		rrt.hasCachedTargetWallet = false
	}
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
		// A zero main UTXO hash means the wallet balance is zero, which is
		// below the moving-funds dust threshold.
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
		// DetermineWalletMainUtxo never returns (nil, nil) when the wallet's
		// on-chain MainUtxoHash is non-zero (checked above): a nil UTXO here
		// alongside a non-zero hash can only mean the resolver failed to
		// find the actual UTXO on the Bitcoin chain, not that the wallet's
		// balance is genuinely below dust. Treating it as below-dust would
		// submit a guaranteed-revert NotifyMovingFundsBelowDust call with a
		// zero UTXO, so this is reported as an error instead.
		return nil, false, fmt.Errorf(
			"wallet main UTXO hash is set but could not be resolved on the Bitcoin chain",
		)
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
// can proceed. This is the remaining route to close a wallet that proved
// its funds moved while it still held reservation anchors: the Bridge's
// own automatic closing attempt runs once, while the reservation count is
// still non-zero, and is never retried. Errors are logged rather than
// propagated: a failed notification here must not block the coordination
// window, and the wallet remains in StateMovingFunds so the next call to
// Run retries.
//
// The notification only fires for a wallet the reservation subsystem has
// actually touched (see walletHasReservationHistory): without that gate,
// any MovingFunds wallet with zero current reservations - including one
// that never held a reservation at all - would trigger a below-dust
// notification, widening reservation-enabled operators into the
// network's general below-dust notifier for wallets unrelated to
// reservations.
func (rrt *ReservationReanchorTask) notifyMovingFundsBelowDustIfEligible(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
) {
	touched, err := rrt.walletHasReservationHistory(walletPublicKeyHash)
	if err != nil {
		taskLogger.Errorf(
			"cannot determine reservation history for wallet: [%v]",
			err,
		)
		return
	}
	if !touched {
		taskLogger.Info(
			"wallet has no reservation history; skipping below-dust notification",
		)
		return
	}

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

// walletHasReservationHistory reports whether walletPublicKeyHash has ever
// been touched by the reservation subsystem: it accepted a reservation, or
// it appears as either side of a reservation re-anchor. Queries are
// unbounded (StartBlock 0) rather than restricted to the package's usual
// look-back window, since a wallet's reservation history can predate that
// window while remaining a valid signal that the wallet is genuinely part
// of the reservation subsystem's remit.
func (rrt *ReservationReanchorTask) walletHasReservationHistory(
	walletPublicKeyHash [20]byte,
) (bool, error) {
	acceptanceEvents, err := rrt.chain.PastReservationAcceptanceRequestedEvents(
		&tbtc.ReservationAcceptanceRequestedEventFilter{
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if err != nil {
		return false, fmt.Errorf(
			"cannot get past reservation acceptance requested events: [%w]",
			err,
		)
	}
	if len(acceptanceEvents) > 0 {
		return true, nil
	}

	reanchorAsSourceEvents, err := rrt.chain.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{
			SourceWalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if err != nil {
		return false, fmt.Errorf(
			"cannot get past reservation re-anchor requested events "+
				"(as source): [%w]",
			err,
		)
	}
	if len(reanchorAsSourceEvents) > 0 {
		return true, nil
	}

	reanchorAsTargetEvents, err := rrt.chain.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{
			TargetWalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if err != nil {
		return false, fmt.Errorf(
			"cannot get past reservation re-anchor requested events "+
				"(as target): [%w]",
			err,
		)
	}
	return len(reanchorAsTargetEvents) > 0, nil
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

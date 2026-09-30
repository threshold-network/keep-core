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
// pattern. It doubles as the same-round fast path for a fresh request
// (below) and the "not observed for this many blocks means dropped"
// bound for the in-flight receipt check in Run.
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

// reservationDepositRefundSafetyMarginSeconds mirrors
// WalletProposalValidatorConstants.DEPOSIT_REFUND_SAFETY_MARGIN
// (24 hours): the on-chain acceptance validator refuses to sign an
// anchor whose refund becomes available less than a day from now, so a
// wallet signing later cannot race the depositor's refund.
const reservationDepositRefundSafetyMarginSeconds = 24 * 60 * 60

// reservationReanchorInFlightRequest tracks a RequestReservationReanchor
// submission that has not yet been resolved by its receipt, keyed by the
// reservation key in the task's inFlightReanchorRequests map.
type reservationReanchorInFlightRequest struct {
	// txHash is the RequestReservationReanchor transaction hash the
	// receipt check looks up.
	txHash [32]byte
	// submittedAtBlock is the current block observed when the submission
	// was recorded.
	submittedAtBlock uint64
	// sourceWalletPublicKeyHash is the wallet that hosted the reservation
	// when the submission was recorded. It identifies the entry to
	// forgetInFlightReanchorRequests if that wallet's reservation list no
	// longer contains the key: a custody move that happened before the
	// next Run round means nothing left to resolve there and the chain
	// state alone drives any resume.
	sourceWalletPublicKeyHash [20]byte
}

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

	// targetWalletCacheMutex guards cachedTargetWalletPublicKeyHash and
	// hasCachedTargetWallet: this task instance is shared across
	// concurrent Run calls for different source wallets (see
	// TestReservationReanchorTask_TargetWalletExclusion_SharedTask).
	targetWalletCacheMutex sync.Mutex
	// cachedTargetWalletPublicKeyHash is the most recently selected
	// re-anchor target wallet. findTargetWallet reuses it across Run
	// calls -- validating headroom and liveness only for this one wallet
	// -- instead of repeating its O(W) GetWallet-per-registration scan on
	// every re-anchor window; a fresh full scan only runs when the cache
	// is empty, the cached wallet fails validation, or the cached wallet
	// is excluded. Meaningful only when hasCachedTargetWallet is true.
	cachedTargetWalletPublicKeyHash [20]byte
	hasCachedTargetWallet           bool

	// inFlightReanchorRequestsMutex guards inFlightReanchorRequests.
	// This task instance is shared across concurrent Run calls for
	// different source wallets, as targetWalletCacheMutex is, so the
	// per-reservation tracking state needs its own mutex.
	inFlightReanchorRequestsMutex sync.Mutex
	// inFlightReanchorRequests tracks RequestReservationReanchor
	// submissions that have not yet been resolved by their receipt,
	// keyed by reservation key. The receipt check at the top of Run
	// settles each entry: mined resumes from chain state, reverted or
	// not-observed-past-the-bound allows a fresh request, still
	// pending skips this round. State loss on restart is tolerable: a
	// mined request shows up as ActionPending (resume path) and a
	// dropped request leaves Active (a new request is safe).
	inFlightReanchorRequests map[string]reservationReanchorInFlightRequest
}

// NewReservationReanchorTask returns a new ReservationReanchorTask bound to
// the given tbtc and Bitcoin chains.
func NewReservationReanchorTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *ReservationReanchorTask {
	return &ReservationReanchorTask{
		chain:                    chain,
		btcChain:                 btcChain,
		inFlightReanchorRequests: make(map[string]reservationReanchorInFlightRequest),
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
// privileged (governance) caller for StateLive sources (enforced on-chain
// in the tbtc-v2 contracts repo, outside keep-core), which the
// client's ordinary operator key can never satisfy, so no below-dust
// re-anchor trigger is attempted for Live wallets.
//
// Each reservation is either Active (needs a freshly requested generation)
// or already ActionPending with its current generation a Pending Reanchor
// action (a request this task issued in an earlier Run that only got as
// far as authorization, or one issued by another party): the latter is
// resumed at its real nonce and authorized target rather than skipped, so
// an in-flight authorization is not abandoned and re-requested.
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

	// A reservation that left the wallet since the entry was recorded
	// has nothing left to resolve under it: drop the in-flight entries
	// that no longer appear in the wallet's reservation list before any
	// path below (including the empty-list early return) can settle
	// them.
	rrt.forgetInFlightReanchorRequestsNotIn(walletPublicKeyHash, reservationKeys)

	if len(reservationKeys) == 0 {
		taskLogger.Info("wallet has no reservations to re-anchor")
		// This duty stays embedded in Run() rather than becoming its own
		// dedicated watcher: a genuine dedicated watcher needs independent
		// scheduling wired up wherever coordination tasks are registered
		// (outside this package), which is a larger cross-package change
		// than justified here, whereas embedding it costs only piggybacking
		// on this task's own already-scheduled invocation cadence.
		rrt.notifyMovingFundsBelowDustIfEligible(taskLogger, walletPublicKeyHash)
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

	// triedTargets accumulates target wallets a cap-related revert has
	// already ruled out during this Run pass. Capacity only grows during
	// a pass (nothing here releases it), so a wallet excluded for one
	// reservation is correctly excluded for every later reservation in
	// the same pass too.
	triedTargets := make(map[[20]byte]bool)

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

		// In-flight request tracking: if a RequestReservationReanchor
		// submitted in an earlier round for this reservation is still
		// unresolved, check its receipt before doing anything else. A
		// mined request means the chain now holds the new generation
		// (resume from chain state below); a reverted or dropped one
		// means the chain still shows the old state and a fresh request
		// is allowed; a still-pending one means this reservation is
		// skipped this round. The submission's block number bounds the
		// pending window: once the submission is not observed within
		// reservationReanchorRequestWaitBlocks it is treated as dropped
		// and a new request is allowed.
		if inFlight, ok := rrt.getReanchorRequestInFlight(reservationKey); ok {
			resolved, reReadReservation, err :=
				rrt.resolveReanchorRequestInFlight(
					taskLogger,
					reservationKey,
					inFlight,
				)
			if err != nil {
				taskLogger.Errorf(
					"cannot resolve in-flight re-anchor request for "+
						"[0x%x]: [%v]",
					reservationKey,
					err,
				)
				continue reservationLoop
			}
			if !resolved {
				taskLogger.Infof(
					"re-anchor request for [0x%x] still in flight; "+
						"skipping this reservation this round",
					reservationKey,
				)
				continue reservationLoop
			}
			if reReadReservation {
				// The request just resolved as mined: re-read the
				// reservation record so the switch below dispatches on
				// the state the mined generation actually advanced it
				// to (ActionPending with the new nonce).
				reservation, err = rrt.chain.GetReservation(reservationKey)
				if err != nil {
					taskLogger.Errorf(
						"cannot re-read reservation [0x%x] after an "+
							"in-flight request resolved as mined: [%v]",
						reservationKey,
						err,
					)
					continue reservationLoop
				}
			}
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

			for {
				target, err := rrt.findTargetWallet(
					taskLogger,
					walletPublicKeyHash,
					anchorValue,
					params.MaxReservationsPerWallet,
					maxReservationsAmountPerWallet,
					triedTargets,
				)
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

				if isReservationCapRevertError(err) {
					// The target's capacity changed since our headroom
					// pre-check (a concurrent re-anchor or acceptance
					// consumed it); evict it and try the next candidate
					// for this same reservation instead of giving up.
					triedTargets[target] = true
					rrt.evictCachedTargetWallet(target)
					continue
				}

				// Every other failure on the request path is safe to
				// retry on the next round without re-requesting: a
				// request that went on-chain is tracked in-flight by
				// its transaction hash (resolved by the receipt check
				// at the top of this pass), and a pre-request local
				// failure (fee estimation, assembly, validation)
				// changed no chain state. Skip this reservation this
				// pass.
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

// errReservationReanchorRequestNotMined signals that a just-submitted
// RequestReservationReanchor was not observed as mined within the
// same-round fast-path wait. The submission itself succeeded: an
// in-flight tracking entry (with the submitted transaction hash and
// block) was recorded before the wait, so subsequent Run rounds
// settle it via its receipt instead of issuing a duplicate request.
// Run treats this as "skip this reservation this round" rather than
// as a window failure.
var errReservationReanchorRequestNotMined = errors.New(
	"reservation re-anchor request not yet mined",
)

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

// ProposeReservationReanchor assembles a single reservation re-anchor
// proposal for the given reservation, targeting the given wallet.
//
// If the reservation's current generation is not already a Pending
// Reanchor action authorizing this exact target, this call first requests
// one on-chain (RequestReservationReanchor) and waits for it to be mined
// before re-reading the reservation and action for the real request nonce
// and snapshotted fee bound; only then does it build and validate the
// proposal. This ordering is required by the on-chain validator, which
// requires the action at proposal.RequestNonce to already be a Pending
// Reanchor -- Reservation.sol's requestReservationReanchor is what writes
// that record, so validating first would read the zero action and always
// revert.
//
// A caller resuming an already-Pending generation (Run does this for
// reservations in ActionPending state) passes that generation's own
// authorized target and reaches this method with no chain write of its
// own; a failure here is therefore a pre-write failure, safe to retry.
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
	requestNonce := reservation.RequestNonce

	if !resuming {
		if reservation.State != tbtc.ReservationStateActive {
			return nil, fmt.Errorf(
				"reservation [0x%x] is not active and has no matching "+
					"pending re-anchor action to resume",
				reservationKey,
			)
		}

		taskLogger.Infof(
			"requesting reservation re-anchor for [0x%x] to wallet [0x%x]",
			reservationKey,
			targetWalletPublicKeyHash,
		)

		txHash, err := rrt.chain.RequestReservationReanchor(
			reservationKey,
			targetWalletPublicKeyHash,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot request reservation re-anchor: [%w]", err)
		}

		// Record the in-flight submission for cross-round receipt
		// resolution (see Run's in-flight receipt check): a later round
		// sees this entry and resolves the request via its receipt
		// (mined -> resume, reverted/dropped -> allow a new request,
		// pending -> skip this round).
		rrt.recordReanchorRequestInFlight(
			taskLogger,
			sourceWalletPublicKeyHash,
			reservationKey,
			txHash,
		)

		// Same-round fast path: wait for the request to mine so this
		// round builds the proposal directly rather than waiting for
		// the next round's receipt resolution. A timeout is no longer a
		// window-aborting post-write failure: the in-flight entry above
		// carries the submission into the receipt check, which resolves
		// it safely in subsequent rounds.
		reservation, action, err = rrt.waitForReservationReanchorRequestMined(
			taskLogger,
			reservationKey,
			targetWalletPublicKeyHash,
			requestNonce,
		)
		if err != nil {
			return nil, errReservationReanchorRequestNotMined
		}
		requestNonce = reservation.RequestNonce
	}

	// The fee bound is the action's own snapshotted txMaxFee, not a live
	// parameter value: WalletProposalValidator.validateReservationReanchorProposal
	// checks the proposed fee against action.txMaxFee specifically (mirroring
	// how it checks a redemption's fee against the request's own stored
	// txMaxFee rather than a live Bridge parameter).
	feeBoundAction := &tbtc.ReservationAction{TxMaxFee: action.TxMaxFee}

	if fee <= 0 {
		taskLogger.Infof("estimating reservation re-anchor transaction fee")

		fee, err = estimateReservationReanchorFee(rrt.btcChain, action.TxMaxFee)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot estimate reservation re-anchor transaction fee: [%w]",
				err,
			)
		}
	}

	taskLogger.Infof("reservation re-anchor transaction fee: [%d]", fee)

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
// targetWalletPublicKeyHash.
func (rrt *ReservationReanchorTask) waitForReservationReanchorRequestMined(
	taskLogger log.StandardLogger,
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
	preRequestNonce uint64,
) (*tbtc.Reservation, *tbtc.ReservationAction, error) {
	blockCounter, err := rrt.chain.BlockCounter()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting block counter: [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting current block: [%w]", err)
	}

	for blockHeight := currentBlock + 1; blockHeight <= currentBlock+reservationReanchorRequestWaitBlocks; blockHeight++ {
		if err := blockCounter.WaitForBlockHeight(blockHeight); err != nil {
			return nil, nil, fmt.Errorf("error while waiting for block height: [%w]", err)
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

			if action.ActionType == tbtc.ReservationActionTypeReanchor &&
				action.State == tbtc.ReservationActionStatePending &&
				action.TargetWalletPublicKeyHash == targetWalletPublicKeyHash {
				taskLogger.Infof(
					"reservation re-anchor request for [0x%x] confirmed at "+
						"block [%d], nonce [%d]",
					reservationKey,
					blockHeight,
					reservation.RequestNonce,
				)
				return reservation, action, nil
			}
		}

		taskLogger.Infof(
			"reservation re-anchor request for [0x%x] still not confirmed "+
				"at block [%d]",
			reservationKey,
			blockHeight,
		)
	}

	return nil, nil, fmt.Errorf(
		"reservation re-anchor request for [0x%x] not confirmed within "+
			"[%d] blocks",
		reservationKey,
		reservationReanchorRequestWaitBlocks,
	)
}

// errNoLiveTargetWallet signals that no eligible re-anchor destination
// wallet was found -- either no wallet is Live at all, or every candidate
// lacks count/amount headroom -- a legitimate "nothing to do yet" outcome,
// not a chain-read failure. Run treats it as a benign no-op for the
// current reservation; any other error returned by findTargetWallet is a
// genuine RPC/chain-read failure and is propagated so the coordinator
// retries.
var errNoLiveTargetWallet = errors.New("no live wallet available for re-anchor target")

// findTargetWallet picks a live destination wallet, with count and amount
// headroom for anchorValue, from the on-chain wallet registry for the
// re-anchor transaction's output. The new wallet must be in StateLive,
// must not be the source wallet itself, must not be in excluded (wallets
// a cap-related revert already ruled out earlier in this Run pass), and
// must have room under maxReservationsPerWallet /
// maxReservationsAmountPerWallet for one more reservation of anchorValue
// satoshi -- mirroring the capacity Reservation.sol's
// requestReservationReanchor reserves on the target at request time.
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
// The previously selected target wallet is cached and, when present and
// not excluded, is the only wallet validated (GetWallet + StateLive +
// not-the-source + headroom) before reuse -- avoiding the O(W)
// GetWallet-per-registration scan below on every re-anchor window. A
// fresh scan runs only when the cache is empty, the cached wallet fails
// validation (it since went non-Live, lost its headroom, or the caller's
// own source wallet now matches it), or the cached wallet is excluded.
func (rrt *ReservationReanchorTask) findTargetWallet(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	anchorValue uint64,
	maxReservationsPerWallet uint32,
	maxReservationsAmountPerWallet uint64,
	excluded map[[20]byte]bool,
) ([20]byte, error) {
	if cached, ok := rrt.cachedTargetWallet(sourceWalletPublicKeyHash); ok && !excluded[cached] {
		walletChainData, err := rrt.chain.GetWallet(cached)
		if err == nil && walletChainData.State == tbtc.StateLive {
			hasHeadroom, err := rrt.walletHasReanchorHeadroom(
				cached, anchorValue, maxReservationsPerWallet, maxReservationsAmountPerWallet,
			)
			if err != nil {
				return [20]byte{}, err
			}
			if hasHeadroom {
				return cached, nil
			}
			taskLogger.Infof(
				"cached re-anchor target wallet [0x%x] has no headroom; "+
					"scanning for a new one",
				cached,
			)
		} else {
			taskLogger.Infof(
				"cached re-anchor target wallet [0x%x] is no longer valid; "+
					"scanning for a new one",
				cached,
			)
		}
		rrt.evictCachedTargetWallet(cached)
	}

	targetWalletPublicKeyHash, err := rrt.scanForTargetWallet(
		taskLogger,
		sourceWalletPublicKeyHash,
		anchorValue,
		maxReservationsPerWallet,
		maxReservationsAmountPerWallet,
		excluded,
	)
	if err != nil {
		return [20]byte{}, err
	}

	rrt.setCachedTargetWallet(targetWalletPublicKeyHash)
	return targetWalletPublicKeyHash, nil
}

// walletHasReanchorHeadroom mirrors the capacity check
// Reservation.sol's requestReservationReanchor performs on the target
// wallet at request time: the wallet's count of currently-custodied
// reservations plus one must not exceed maxReservationsPerWallet, and
// (only when the amount cap is configured -- zero disables it, matching
// Solidity's `maxReservationsAmountPerWallet == 0` disable convention)
// its current reserved amount plus anchorValue must not exceed
// maxReservationsAmountPerWallet.
func (rrt *ReservationReanchorTask) walletHasReanchorHeadroom(
	walletPublicKeyHash [20]byte,
	anchorValue uint64,
	maxReservationsPerWallet uint32,
	maxReservationsAmountPerWallet uint64,
) (bool, error) {
	count, err := rrt.chain.WalletReservationsCount(walletPublicKeyHash)
	if err != nil {
		return false, fmt.Errorf("cannot get wallet reservations count: [%w]", err)
	}
	if count+1 > maxReservationsPerWallet {
		return false, nil
	}

	if maxReservationsAmountPerWallet > 0 {
		amount, err := rrt.chain.WalletReservationsAmount(walletPublicKeyHash)
		if err != nil {
			return false, fmt.Errorf("cannot get wallet reservations amount: [%w]", err)
		}
		if amount+anchorValue > maxReservationsAmountPerWallet {
			return false, nil
		}
	}

	return true, nil
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
// target wallet for reuse by future findTargetWallet calls.
func (rrt *ReservationReanchorTask) setCachedTargetWallet(
	targetWalletPublicKeyHash [20]byte,
) {
	rrt.targetWalletCacheMutex.Lock()
	defer rrt.targetWalletCacheMutex.Unlock()

	rrt.cachedTargetWalletPublicKeyHash = targetWalletPublicKeyHash
	rrt.hasCachedTargetWallet = true
}

// evictCachedTargetWallet clears the cached re-anchor target wallet if it
// currently equals target, so the next findTargetWallet call performs a
// fresh scan instead of reusing a wallet just proven to have no headroom
// or to have reverted on capacity.
func (rrt *ReservationReanchorTask) evictCachedTargetWallet(target [20]byte) {
	rrt.targetWalletCacheMutex.Lock()
	defer rrt.targetWalletCacheMutex.Unlock()

	if rrt.hasCachedTargetWallet && rrt.cachedTargetWalletPublicKeyHash == target {
		rrt.hasCachedTargetWallet = false
	}
}

// recordReanchorRequestInFlight records the just-submitted
// RequestReservationReanchor transaction hash for the reservation under
// sourceWalletPublicKeyHash, the wallet that hosted the reservation when
// the submission was recorded. The current block (or 0 if the block
// counter is unavailable) bounds the receipt check's "not observed for
// this many blocks means dropped" logic.
func (rrt *ReservationReanchorTask) recordReanchorRequestInFlight(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	reservationKey *big.Int,
	txHash [32]byte,
) {
	var submittedBlock uint64
	if blockCounter, err := rrt.chain.BlockCounter(); err == nil {
		if currentBlock, err := blockCounter.CurrentBlock(); err == nil {
			submittedBlock = currentBlock
		} else {
			taskLogger.Warnf(
				"cannot read current block when recording in-flight re-anchor "+
					"request for [0x%x]: [%v]; the receipt check will treat "+
					"any pending status as past the window",
				reservationKey,
				err,
			)
		}
	} else {
		taskLogger.Warnf(
			"cannot get block counter when recording in-flight re-anchor "+
				"request for [0x%x]: [%v]; the receipt check will treat any "+
				"pending status as past the window",
			reservationKey,
			err,
		)
	}

	rrt.inFlightReanchorRequestsMutex.Lock()
	defer rrt.inFlightReanchorRequestsMutex.Unlock()

	rrt.inFlightReanchorRequests[reservationKey.Text(16)] = reservationReanchorInFlightRequest{
		txHash:                    txHash,
		submittedAtBlock:          submittedBlock,
		sourceWalletPublicKeyHash: sourceWalletPublicKeyHash,
	}
}

// forgetInFlightReanchorRequestsNotIn drops the in-flight entries for
// sourceWalletPublicKeyHash whose reservation key is not in currentKeys,
// called at the top of Run, just after the wallet's reservation list is
// read: a reservation that has since left the wallet (moved to another
// custody) has nothing left to resume under this wallet, so the chain
// state alone drives any follow-up and the tracking entry would otherwise
// leak for the task's lifetime. Entries hosted by any other wallet are
// left untouched.
func (rrt *ReservationReanchorTask) forgetInFlightReanchorRequestsNotIn(
	sourceWalletPublicKeyHash [20]byte,
	currentKeys []*big.Int,
) {
	present := make(map[string]bool, len(currentKeys))
	for _, key := range currentKeys {
		present[key.Text(16)] = true
	}

	rrt.inFlightReanchorRequestsMutex.Lock()
	defer rrt.inFlightReanchorRequestsMutex.Unlock()

	for key, entry := range rrt.inFlightReanchorRequests {
		if entry.sourceWalletPublicKeyHash == sourceWalletPublicKeyHash &&
			!present[key] {
			delete(rrt.inFlightReanchorRequests, key)
		}
	}
}

// getReanchorRequestInFlight returns the in-flight record for the
// reservation key, if any.
func (rrt *ReservationReanchorTask) getReanchorRequestInFlight(
	reservationKey *big.Int,
) (reservationReanchorInFlightRequest, bool) {
	rrt.inFlightReanchorRequestsMutex.Lock()
	defer rrt.inFlightReanchorRequestsMutex.Unlock()

	entry, ok := rrt.inFlightReanchorRequests[reservationKey.Text(16)]
	return entry, ok
}

// forgetInFlightReanchorRequest drops the in-flight record for the
// reservation key, called once the receipt outcome has been resolved.
func (rrt *ReservationReanchorTask) forgetInFlightReanchorRequest(
	reservationKey *big.Int,
) {
	rrt.inFlightReanchorRequestsMutex.Lock()
	defer rrt.inFlightReanchorRequestsMutex.Unlock()

	delete(rrt.inFlightReanchorRequests, reservationKey.Text(16))
}

// resolveReanchorRequestInFlight looks up the receipt for an in-flight
// RequestReservationReanchor submission and settles it.
//
//   - Mined: the chain now holds the new generation; the entry is
//     forgotten. Callers should re-read the reservation and take the
//     ActionPending resume path.
//   - Reverted: no generation was written; the entry is forgotten and a
//     fresh request is allowed this round.
//   - Pending / NotFound: the submission is still unobserved. Within
//     reservationReanchorRequestWaitBlocks of submission the caller
//     should skip this reservation for this round (the receipt may
//     surface on a later one); past that bound the submission is
//     treated as dropped and a new request is allowed.
//
// The returned values are:
//
//	resolved        -- the in-flight entry has been settled and the
//	                   caller may continue past it;
//	re-mustClear    -- the chain state may have advanced (only true for
//	                   the Mined case), so the caller should re-read the
//	                   reservation record before dispatching.
func (rrt *ReservationReanchorTask) resolveReanchorRequestInFlight(
	taskLogger log.StandardLogger,
	reservationKey *big.Int,
	inFlight reservationReanchorInFlightRequest,
) (resolved bool, reMustClear bool, err error) {
	status, err := rrt.chain.GetReservationReanchorRequestReceipt(inFlight.txHash)
	if err != nil {
		return false, false, fmt.Errorf(
			"receipt lookup: [%w]",
			err,
		)
	}

	switch status {
	case tbtc.ReservationReanchorRequestReceiptMined:
		taskLogger.Infof(
			"in-flight re-anchor request [%x] for reservation [0x%x] mined; "+
				"resuming from chain state",
			inFlight.txHash,
			reservationKey,
		)
		rrt.forgetInFlightReanchorRequest(reservationKey)
		return true, true, nil
	case tbtc.ReservationReanchorRequestReceiptReverted:
		taskLogger.Infof(
			"in-flight re-anchor request [%x] for reservation [0x%x] reverted; "+
				"allowing a new request",
			inFlight.txHash,
			reservationKey,
		)
		rrt.forgetInFlightReanchorRequest(reservationKey)
		return true, false, nil
	case tbtc.ReservationReanchorRequestReceiptPending,
		tbtc.ReservationReanchorRequestReceiptNotFound:
		blockCounter, err := rrt.chain.BlockCounter()
		if err != nil {
			return false, false, fmt.Errorf(
				"block counter: [%w]",
				err,
			)
		}
		currentBlock, err := blockCounter.CurrentBlock()
		if err != nil {
			return false, false, fmt.Errorf(
				"current block: [%w]",
				err,
			)
		}
		if currentBlock > inFlight.submittedAtBlock &&
			currentBlock-inFlight.submittedAtBlock >
				reservationReanchorRequestWaitBlocks {
			taskLogger.Infof(
				"in-flight re-anchor request [%x] for reservation [0x%x] "+
					"unobserved for [%d] blocks; treating as dropped and "+
					"allowing a new request",
				inFlight.txHash,
				reservationKey,
				currentBlock-inFlight.submittedAtBlock,
			)
			rrt.forgetInFlightReanchorRequest(reservationKey)
			return true, false, nil
		}
		return false, false, nil
	}

	return true, false, nil
}

// scanForTargetWallet performs the registration-event scan findTargetWallet
// falls back to when no cached target wallet is usable. The primary scan
// is bounded to ReservationReanchorLookBackBlocks (mirroring the other
// look-back scans in this package): an unbounded eth_getLogs scan on
// every re-anchor attempt is too expensive to run every window.
// GetLiveWalletsCount (checked by the caller before findTargetWallet
// runs) can confirm live wallets exist even when none of them registered
// within the look-back window, so a bounded scan that finds no candidate
// falls back to an unbounded one instead of leaving Run stuck returning
// no proposal indefinitely.
func (rrt *ReservationReanchorTask) scanForTargetWallet(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	anchorValue uint64,
	maxReservationsPerWallet uint32,
	maxReservationsAmountPerWallet uint64,
	excluded map[[20]byte]bool,
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
		anchorValue,
		maxReservationsPerWallet,
		maxReservationsAmountPerWallet,
		excluded,
	)
	if err == nil {
		return targetWalletPublicKeyHash, nil
	}
	if startBlock == 0 {
		// The bounded scan above already covered full chain history.
		return [20]byte{}, err
	}

	taskLogger.Infof(
		"no live re-anchor target with headroom registered within the "+
			"last [%d] blocks, falling back to an unbounded registration "+
			"event scan",
		ReservationReanchorLookBackBlocks,
	)

	return rrt.findLiveWalletFromRegistrationEvents(
		taskLogger,
		sourceWalletPublicKeyHash,
		0,
		anchorValue,
		maxReservationsPerWallet,
		maxReservationsAmountPerWallet,
		excluded,
	)
}

// findLiveWalletFromRegistrationEvents scans new-wallet-registered events
// starting at startBlock and returns the most-recently-registered Live
// wallet, other than sourceWalletPublicKeyHash or any wallet in excluded,
// that has count/amount headroom for anchorValue.
func (rrt *ReservationReanchorTask) findLiveWalletFromRegistrationEvents(
	taskLogger log.StandardLogger,
	sourceWalletPublicKeyHash [20]byte,
	startBlock uint64,
	anchorValue uint64,
	maxReservationsPerWallet uint32,
	maxReservationsAmountPerWallet uint64,
	excluded map[[20]byte]bool,
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
		if walletPubKeyHash == sourceWalletPublicKeyHash || excluded[walletPubKeyHash] {
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

		if wallet.State != tbtc.StateLive {
			continue
		}

		hasHeadroom, err := rrt.walletHasReanchorHeadroom(
			walletPubKeyHash, anchorValue, maxReservationsPerWallet, maxReservationsAmountPerWallet,
		)
		if err != nil {
			return [20]byte{}, err
		}
		if !hasHeadroom {
			taskLogger.Infof(
				"candidate re-anchor target wallet [0x%x] has no headroom, skipping",
				walletPubKeyHash,
			)
			continue
		}

		return walletPubKeyHash, nil
	}

	return [20]byte{}, errNoLiveTargetWallet
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

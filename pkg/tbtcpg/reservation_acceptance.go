package tbtcpg

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/ipfs/go-log/v2"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ReservationAcceptanceLookBackBlocks is the look-back period in blocks used
// when searching for reservation candidate deposits. It mirrors the deposit
// sweep look-back window: 30 days at 12 seconds per block.
const ReservationAcceptanceLookBackBlocks = uint64(216000)

// ReservationAcceptanceTask is a task that may produce a reservation
// acceptance (anchor) proposal. It scans the chain for reserved deposits
// revealed to the operator's wallet, validates the wallet's eligibility
// against the active reservation caps, and emits a proposal whose resulting
// transaction is a 1-input-1-output anchor that disables the deposit's
// refund path.
type ReservationAcceptanceTask struct {
	chain    Chain
	btcChain bitcoin.Chain
}

// NewReservationAcceptanceTask constructs a ReservationAcceptanceTask.
func NewReservationAcceptanceTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *ReservationAcceptanceTask {
	return &ReservationAcceptanceTask{
		chain:    chain,
		btcChain: btcChain,
	}
}

// Run inspects the chain for an acceptance candidate reserved deposit and,
// if one passes the eligibility gate, returns the resulting anchor proposal.
// The task is a no-op (proposal == nil, shouldExecute == false) when no
// candidate exists.
func (rat *ReservationAcceptanceTask) Run(request *tbtc.CoordinationProposalRequest) (
	tbtc.CoordinationProposal,
	bool,
	error,
) {
	walletPublicKeyHash := request.WalletPublicKeyHash

	taskLogger := logger.With(
		zap.String("task", rat.ActionType().String()),
		zap.String("walletPKH", fmt.Sprintf("0x%x", walletPublicKeyHash)),
	)

	candidate, err := rat.findReservationAcceptanceCandidate(
		taskLogger,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot find reservation acceptance candidate: [%w]",
			err,
		)
	}
	if candidate == nil {
		taskLogger.Info("no reservation acceptance candidate")
		return nil, false, nil
	}

	proposal, shouldExecute, err := rat.proposeReservationAcceptance(
		taskLogger,
		walletPublicKeyHash,
		candidate,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot prepare reservation acceptance proposal: [%w]",
			err,
		)
	}

	return proposal, shouldExecute, nil
}

// ActionType returns the wallet action type this task proposes.
func (rat *ReservationAcceptanceTask) ActionType() tbtc.WalletActionType {
	return tbtc.ActionReservationAnchor
}

// reservationAcceptanceCandidate is the bundle a candidate reserved deposit
// for acceptance carries through the proposal builder. It captures the
// deposit's reveal context, the derived request nonce, plus the on-chain cap
// snapshot taken at scan time.
type reservationAcceptanceCandidate struct {
	Deposit               *tbtc.Deposit
	FundingTx             *bitcoin.Transaction
	ReservationParameters *tbtc.ReservationParameters
	TxMaxFee              uint64
	RequestNonce          uint64
	AnchorFee             int64
}

// findReservationAcceptanceCandidate returns the first reserved deposit
// that the operator's wallet may accept, or nil when none qualifies.
func (rat *ReservationAcceptanceTask) findReservationAcceptanceCandidate(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
) (*reservationAcceptanceCandidate, error) {
	if walletPublicKeyHash == [20]byte{} {
		return nil, fmt.Errorf("wallet public key hash is required")
	}

	reservationParameters, err := rat.chain.ReservationParameters()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get reservation parameters: [%w]",
			err,
		)
	}
	reservationVault := reservationParameters.ReservationVault
	if reservationVault == "" {
		taskLogger.Info("reservation vault not configured")
		return nil, nil
	}

	wallet, err := rat.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		taskLogger.Errorf(
			"failed to load wallet chain data: [%v]",
			err,
		)
		return nil, nil
	}
	if wallet.State != tbtc.StateLive {
		taskLogger.Infof(
			"wallet is not live (state=%v); cannot accept reservation",
			wallet.State,
		)
		return nil, nil
	}

	blockCounter, err := rat.chain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to get block counter: [%w]", err)
	}
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get current block: [%w]",
			err,
		)
	}

	maxReservationsAmountPerWallet, reservationMaxSingleAmount, err :=
		rat.chain.ReservationCaps()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get reservation caps: [%w]",
			err,
		)
	}

	walletReservationsCount, err := rat.chain.WalletReservationsCount(
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get wallet reservations count: [%w]",
			err,
		)
	}

	walletReservationsAmount, err := rat.chain.WalletReservationsAmount(
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get wallet reservations amount: [%w]",
			err,
		)
	}

	activeReservationsCount, maxActiveReservations, err :=
		rat.chain.ActiveReservationsCount()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get active reservations count: [%w]",
			err,
		)
	}

	depositMinAgeSeconds, err := rat.chain.GetDepositMinAge()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get deposit minimum age: [%w]",
			err,
		)
	}
	depositMinAge := time.Duration(depositMinAgeSeconds) * time.Second

	filterStartBlock := uint64(0)
	if currentBlock > ReservationAcceptanceLookBackBlocks {
		filterStartBlock = currentBlock - ReservationAcceptanceLookBackBlocks
	}
	filter := &tbtc.DepositRevealedEventFilter{
		StartBlock:          filterStartBlock,
		EndBlock:            &currentBlock,
		WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
	}

	depositRevealedEvents, err := rat.chain.PastDepositRevealedEvents(filter)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past deposit revealed events: [%w]",
			err,
		)
	}

	// Take the oldest first.
	sort.SliceStable(depositRevealedEvents, func(i, j int) bool {
		return depositRevealedEvents[i].BlockNumber < depositRevealedEvents[j].BlockNumber
	})

	now := time.Now()

	for _, event := range depositRevealedEvents {
		if !depositTargetsReservationVault(event.Vault, reservationVault) {
			continue
		}

		depositKey := rat.chain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		depositRequest, foundRequest, err := rat.chain.GetDepositRequest(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)
		if err != nil {
			taskLogger.Errorf(
				"failed to get deposit request for [%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}
		if !foundRequest {
			taskLogger.Warnf(
				"no deposit request for reserved deposit [%v]",
				depositKey,
			)
			continue
		}

		if depositRequest.Amount < reservationParameters.ReservationMinAmount {
			taskLogger.Infof(
				"reserved deposit [%v] amount [%d] below minimum [%d]; skipping",
				depositKey,
				depositRequest.Amount,
				reservationParameters.ReservationMinAmount,
			)
			continue
		}

		matureAt := depositRequest.RevealedAt.Add(depositMinAge)
		if !now.After(matureAt) {
			taskLogger.Infof(
				"reserved deposit [%v] is not old enough: now=%v, matureAt=%v",
				depositKey,
				now, matureAt,
			)
			continue
		}

		if depositRequest.SweptAt.Unix() != 0 {
			taskLogger.Debugf(
				"reserved deposit [%v] is already swept",
				depositKey,
			)
			continue
		}

		if !checkReservationAcceptanceEligibility(
			taskLogger,
			depositRequest,
			walletReservationsCount,
			walletReservationsAmount,
			activeReservationsCount,
			maxActiveReservations,
			maxReservationsAmountPerWallet,
			reservationMaxSingleAmount,
			reservationParameters,
		) {
			taskLogger.Infof("not eligible: [%v]", depositKey)
			continue
		}

		fundingTx, err := rat.btcChain.GetTransaction(event.FundingTxHash)
		if err != nil {
			taskLogger.Errorf(
				"failed to get funding tx for reserved deposit [%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}

		confirmations, err := rat.btcChain.GetTransactionConfirmations(
			context.Background(),
			event.FundingTxHash,
		)
		if err != nil {
			taskLogger.Errorf(
				"failed to get funding tx confirmations for [%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}
		if confirmations < tbtc.DepositSweepRequiredFundingTxConfirmations {
			taskLogger.Debugf(
				"reserved deposit [%v] funding tx confirmations [%d/%d] below required",
				depositKey,
				confirmations,
				tbtc.DepositSweepRequiredFundingTxConfirmations,
			)
			continue
		}

		// Third fix (a): check for existing acceptance requested events
		// and fail closed on error.
		acceptanceEvents, err := rat.chain.PastReservationAcceptanceRequestedEvents(
			&tbtc.ReservationAcceptanceRequestedEventFilter{
				ReservationKey: []*big.Int{depositKey},
			},
		)
		if err != nil {
			taskLogger.Errorf(
				"failed to get past reservation acceptance requested events for [%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}
		if len(acceptanceEvents) > 0 {
			taskLogger.Infof(
				"reservation [%v] already has acceptance requested event(s), skipping",
				depositKey,
			)
			continue
		}

		// Second & Third fix: check reservation state and derive RequestNonce.
		//
		// GetReservation's documented contract (chain.go) is "returns an
		// error if the reservation was not found" -- unlike the sibling
		// lookups above (GetDepositRequest's foundRequest bool,
		// PastReservationAcceptanceRequestedEvents' empty-slice-on-none),
		// this call has no way to distinguish "not found" (the expected,
		// common case for a brand-new candidate) from a genuine query
		// failure. Failing closed here like the siblings would reject
		// every first-time candidate, since "not yet reserved" is itself
		// signaled as an error. This is a deliberate fail-open deviation:
		// any GetReservation error is treated as "not yet created" and
		// requestNonce defaults to 1; see
		// TestReservationAcceptanceTask_GetReservationError for the pinned
		// RequestNonce == 1 fallback behavior.
		var requestNonce uint64 = 1
		reservation, err := rat.chain.GetReservation(depositKey)
		if err != nil {
			taskLogger.Debugf(
				"cannot get reservation [%v] (assuming not yet created): [%v]",
				depositKey,
				err,
			)
		} else if reservation != nil {
			if reservation.State == tbtc.ReservationStateActive ||
				reservation.State == tbtc.ReservationStateActionPending ||
				reservation.State == tbtc.ReservationStateClosed ||
				reservation.State == tbtc.ReservationStateStranded {
				taskLogger.Infof(
					"reservation [%v] in non-eligible state [%v], skipping",
					depositKey,
					reservation.State,
				)
				continue
			}

			if hasPendingAction(depositKey, reservation, rat.chain, taskLogger) {
				taskLogger.Infof(
					"reservation [%v] has pending action, skipping",
					depositKey,
				)
				continue
			}

			requestNonce = reservation.RequestNonce + 1
		}

		// Estimate the anchor fee and check net-of-fee viability here, as
		// part of candidate selection, rather than after a single candidate
		// has already been chosen. A candidate that fails this check is
		// skipped in favor of the next one; nothing marks it retried, so
		// leaving this check in proposeReservationAcceptance (which is
		// called for exactly one already-selected candidate) would cause
		// the same doomed deposit to be re-selected and abort on every
		// subsequent Run() until it aged out of the look-back window.
		anchorFee, err := estimateReservationAcceptanceFee(
			rat.btcChain,
			reservationParameters.ReservationTxMaxFee,
		)
		if err != nil {
			taskLogger.Errorf(
				"failed to estimate reservation acceptance transaction fee for [%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}

		anchorValue := int64(depositRequest.Amount) - anchorFee
		if anchorValue <= 0 {
			taskLogger.Infof(
				"reserved deposit [%v] value [%d] does not cover anchor fee [%d]; skipping",
				depositKey,
				depositRequest.Amount,
				anchorFee,
			)
			continue
		}
		if uint64(anchorValue) < reservationParameters.ReservationMinAmount {
			taskLogger.Infof(
				"reserved deposit [%v] net-of-fee value [%d] below minimum [%d]; skipping",
				depositKey,
				anchorValue,
				reservationParameters.ReservationMinAmount,
			)
			continue
		}

		taskLogger.Infof(
			"selected reserved deposit [%v] for acceptance",
			depositKey,
		)

		return &reservationAcceptanceCandidate{
			Deposit: &tbtc.Deposit{
				Utxo: &bitcoin.UnspentTransactionOutput{
					Outpoint: &bitcoin.TransactionOutpoint{
						TransactionHash: event.FundingTxHash,
						OutputIndex:     event.FundingOutputIndex,
					},
					Value: int64(depositRequest.Amount),
				},
				Depositor:           depositRequest.Depositor,
				BlindingFactor:      event.BlindingFactor,
				WalletPublicKeyHash: event.WalletPublicKeyHash,
				RefundPublicKeyHash: event.RefundPublicKeyHash,
				RefundLocktime:      event.RefundLocktime,
				Vault:               depositRequest.Vault,
				ExtraData:           depositRequest.ExtraData,
			},
			FundingTx:             fundingTx,
			ReservationParameters: reservationParameters,
			TxMaxFee:              reservationParameters.ReservationTxMaxFee,
			RequestNonce:          requestNonce,
			AnchorFee:             anchorFee,
		}, nil
	}

	return nil, nil
}

func checkReservationAcceptanceEligibility(
	taskLogger log.StandardLogger,
	depositRequest *tbtc.DepositChainRequest,
	walletReservationsCount uint32,
	walletReservationsAmount uint64,
	activeReservationsCount uint32,
	maxActiveReservations uint32,
	maxReservationsAmountPerWallet uint64,
	reservationMaxSingleAmount uint64,
	reservationParameters *tbtc.ReservationParameters,
) bool {
	if reservationParameters.MaxReservationsPerWallet > 0 &&
		walletReservationsCount >= reservationParameters.MaxReservationsPerWallet {
		taskLogger.Infof(
			"wallet reservations count [%d] already at max [%d]",
			walletReservationsCount,
			reservationParameters.MaxReservationsPerWallet,
		)
		return false
	}

	if maxActiveReservations > 0 &&
		activeReservationsCount >= maxActiveReservations {
		taskLogger.Infof(
			"active reservations count [%d] already at max [%d]",
			activeReservationsCount,
			maxActiveReservations,
		)
		return false
	}

	if reservationMaxSingleAmount > 0 &&
		depositRequest.Amount > reservationMaxSingleAmount {
		taskLogger.Infof(
			"deposit amount [%d] exceeds reservation single cap [%d]",
			depositRequest.Amount,
			reservationMaxSingleAmount,
		)
		return false
	}

	newWalletTotal := walletReservationsAmount + depositRequest.Amount
	if maxReservationsAmountPerWallet > 0 &&
		newWalletTotal > maxReservationsAmountPerWallet {
		taskLogger.Infof(
			"accepting would push wallet past aggregate cap "+
				"[current=%d, deposit=%d, cap=%d]",
			walletReservationsAmount,
			depositRequest.Amount,
			maxReservationsAmountPerWallet,
		)
		return false
	}

	if reservationParameters.ReservationMaxTotalAmount > 0 {
		newTotal := reservationParameters.ReservationTotalAmount +
			depositRequest.Amount
		if newTotal > reservationParameters.ReservationMaxTotalAmount {
			taskLogger.Infof(
				"global reservation total would exceed cap "+
					"[current=%d, deposit=%d, cap=%d]",
				reservationParameters.ReservationTotalAmount,
				depositRequest.Amount,
				reservationParameters.ReservationMaxTotalAmount,
			)
			return false
		}
	}

	return true
}

func (rat *ReservationAcceptanceTask) proposeReservationAcceptance(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	candidate *reservationAcceptanceCandidate,
) (*tbtc.ReservationAnchorProposal, bool, error) {
	if candidate == nil || candidate.Deposit == nil {
		return nil, false, fmt.Errorf("candidate is required")
	}

	taskLogger.Infof("preparing a reservation acceptance proposal")

	// The anchor fee and its net-of-fee viability were already computed and
	// validated during candidate selection in findReservationAcceptanceCandidate;
	// re-checking here (after exactly one candidate has already been chosen)
	// would abort this Run() outright on failure instead of trying the next
	// candidate, causing the same doomed deposit to be re-selected on every
	// subsequent Run() until it aged out of the look-back window.
	anchorFee := candidate.AnchorFee

	taskLogger.Infof("anchor transaction fee: [%d]", anchorFee)

	reservationKey := rat.chain.BuildDepositKey(
		candidate.Deposit.Utxo.Outpoint.TransactionHash,
		candidate.Deposit.Utxo.Outpoint.OutputIndex,
	)

	feeBoundAction := &tbtc.ReservationAction{
		TxMaxFee: candidate.TxMaxFee,
	}

	if _, err := tbtc.AssembleReservationAnchorTransaction(
		rat.btcChain,
		candidate.Deposit,
		walletPublicKeyHash,
		feeBoundAction,
		anchorFee,
	); err != nil {
		return nil, false, fmt.Errorf(
			"cannot assemble reservation anchor transaction: [%v]",
			err,
		)
	}

	proposal := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      candidate.Deposit.Utxo.Outpoint.TransactionHash,
		DepositFundingOutputIndex: candidate.Deposit.Utxo.Outpoint.OutputIndex,
		RequestNonce:              candidate.RequestNonce,
		AnchorTxFee:               big.NewInt(anchorFee),
	}

	taskLogger.Infof("validating the reservation anchor proposal")

	if err := rat.chain.ValidateReservationAnchorProposal(
		walletPublicKeyHash,
		proposal,
		struct {
			*tbtc.Deposit
			FundingTx *bitcoin.Transaction
		}{
			Deposit:   candidate.Deposit,
			FundingTx: candidate.FundingTx,
		},
	); err != nil {
		return nil, false, fmt.Errorf(
			"failed to verify reservation anchor proposal: %v",
			err,
		)
	}

	// Fifth fix: Note that RequestReservationAcceptance is called as a
	// side effect of proposal generation itself (before coordination has
	// agreed to anything), which is a known, accepted deviation from the
	// read-only-during-generation pattern (also present in MovingFundsTask's
	// SubmitMovingFundsCommitment). The guards against re-requesting
	// acceptance for existing or pending reservations are the primary
	// mitigation for spurious repeat writes.
	if err := rat.chain.RequestReservationAcceptance(
		reservationKey,
		walletPublicKeyHash,
	); err != nil {
		return nil, false, fmt.Errorf("cannot request reservation acceptance: [%v]", err)
	}

	return proposal, true, nil
}

func estimateReservationAcceptanceFee(
	btcChain bitcoin.Chain,
	txMaxFee uint64,
) (int64, error) {
	sizeEstimator := bitcoin.NewTransactionSizeEstimator().
		AddScriptHashInputs(1, depositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true)

	return estimateReservationFixedSizeTxFee(
		btcChain,
		sizeEstimator,
		txMaxFee,
		"reservation acceptance estimated fee exceeds the maximum fee",
	)
}

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

package tbtcpg

import (
	"fmt"
	"math/big"
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

// reservationAnchorFeeSat is the deterministic satoshi fee the operator
// charges for the 1-input-1-output anchor transaction. The transaction
// shape is fixed (one P2SH deposit input, one P2WPKH anchor output) so a
// constant estimate is appropriate. Operators can override this through
// governance if the network fee environment drifts.
const reservationAnchorFeeSat int64 = 1500

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

	proposal, err := rat.proposeReservationAcceptance(
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

	return proposal, true, nil
}

// ActionType returns the wallet action type this task proposes.
func (rat *ReservationAcceptanceTask) ActionType() tbtc.WalletActionType {
	return tbtc.ActionReservationAnchor
}

// reservationAcceptanceCandidate is the bundle a candidate reserved deposit
// for acceptance carries through the proposal builder. It captures the
// deposit's reveal context plus the on-chain cap snapshot taken at scan time.
type reservationAcceptanceCandidate struct {
	Deposit               *tbtc.Deposit
	FundingTx             *bitcoin.Transaction
	RevealBlock           uint64
	ReservationParameters *tbtc.ReservationParameters
	WalletCap             uint64
	SingleCap             uint64
	ActiveCount           uint32
	MaxActive             uint32
	MaxPerWallet          uint32
	PendingReserved       uint64
	TxMaxFee              uint64
}

// findReservationAcceptanceCandidate returns the first reserved deposit
// that the operator's wallet may accept, or nil when none qualifies. The
// function performs the look-back bounded scan over past
// DepositRevealedEvents, fetches each candidate's chain request to determine
// whether it is reserved, and applies the eligibility gate.
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
		// Reservation subsystem not active; no acceptance candidates.
		taskLogger.Info("reservation vault not configured")
		return nil, nil
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

	pendingReservedDeposits, err := rat.chain.PendingReservedDeposits()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get pending reserved deposits count: [%w]",
			err,
		)
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

	filterStartBlock := uint64(0)
	if currentBlock > ReservationAcceptanceLookBackBlocks {
		filterStartBlock = currentBlock - ReservationAcceptanceLookBackBlocks
	}

	filter := &tbtc.DepositRevealedEventFilter{
		StartBlock:          filterStartBlock,
		WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
	}

	revealedEvents, err := rat.chain.PastDepositRevealedEvents(filter)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past deposit revealed events: [%w]",
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

	now := time.Now()

	for _, event := range revealedEvents {
		depositKey := rat.chain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		isReserved, err := rat.chain.IsReservedDeposit(depositKey)
		if err != nil {
			taskLogger.Errorf(
				"failed to check if deposit [%v] is reserved: [%v]",
				depositKey,
				err,
			)
			continue
		}
		if !isReserved {
			taskLogger.Infof("not reserved deposit [%v]", depositKey)
			continue
		}

		depositRequest, found, err := rat.chain.GetDepositRequest(
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
		if !found {
			taskLogger.Warnf(
				"no deposit request for reserved deposit [%v]",
				depositKey,
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

		if !depositTargetsReservationVault(
			depositRequest.Vault,
			reservationVault,
		) {
			taskLogger.Debugf(
				"reserved deposit [%v] vault does not match "+
					"the active reservation vault",
				depositKey,
			)
			continue
		}

		candidate := &reservationAcceptanceCandidate{
			RevealBlock:           event.BlockNumber,
			ReservationParameters: reservationParameters,
			WalletCap:             maxReservationsAmountPerWallet,
			SingleCap:             reservationMaxSingleAmount,
			ActiveCount:           activeReservationsCount,
			MaxActive:             maxActiveReservations,
			MaxPerWallet:          reservationParameters.MaxReservationsPerWallet,
			PendingReserved:       pendingReservedDeposits,
			TxMaxFee:              reservationParameters.ReservationTxMaxFee,
		}

		if !rat.checkReservationAcceptanceEligibility(
			taskLogger,
			walletPublicKeyHash,
			depositRequest,
			walletReservationsCount,
			walletReservationsAmount,
			activeReservationsCount,
			maxActiveReservations,
			pendingReservedDeposits,
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

		candidate.Deposit = &tbtc.Deposit{
			Utxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: event.FundingTxHash,
					OutputIndex:     event.FundingOutputIndex,
				},
				Value: int64(depositRequest.Amount),
			},
			Depositor:           depositRequest.Depositor,
			WalletPublicKeyHash: event.WalletPublicKeyHash,
			Vault:               depositRequest.Vault,
		}
		candidate.FundingTx = fundingTx

		taskLogger.Infof(
			"selected reserved deposit [%v] for acceptance",
			depositKey,
		)

		return candidate, nil
	}

	return nil, nil
}

// checkReservationAcceptanceEligibility returns true iff the wallet may
// accept a new reserved deposit given the current cap snapshot. The
// predicate is intentionally strict: a single failing rule rejects the
// candidate so the wallet never publishes a proposal that the Bridge would
// reject.
func (rat *ReservationAcceptanceTask) checkReservationAcceptanceEligibility(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	depositRequest *tbtc.DepositChainRequest,
	walletReservationsCount uint32,
	walletReservationsAmount uint64,
	activeReservationsCount uint32,
	maxActiveReservations uint32,
	pendingReservedDeposits uint64,
	maxReservationsAmountPerWallet uint64,
	reservationMaxSingleAmount uint64,
	reservationParameters *tbtc.ReservationParameters,
) bool {
	wallet, err := rat.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		taskLogger.Errorf(
			"failed to load wallet chain data: [%v]",
			err,
		)
		return false
	}
	if wallet.State != tbtc.StateLive {
		taskLogger.Infof(
			"wallet is not live (state=%v); cannot accept reservation",
			wallet.State,
		)
		return false
	}

	if walletReservationsCount >= reservationParameters.MaxReservationsPerWallet {
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

	if depositRequest.Amount < reservationParameters.ReservationMinAmount {
		taskLogger.Infof(
			"deposit amount [%d] below reservation min [%d]",
			depositRequest.Amount,
			reservationParameters.ReservationMinAmount,
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

	if pendingReservedDeposits > 0 &&
		reservationParameters.ReservationTotalAmount+
			depositRequest.Amount >
			reservationParameters.ReservationMaxTotalAmount {
		taskLogger.Infof(
			"pending reserved deposits queue [%d] would push global total "+
				"past cap; deferring",
			pendingReservedDeposits,
		)
		return false
	}

	return true
}

// proposeReservationAcceptance assembles the anchor transaction for the
// candidate reserved deposit and returns the on-chain proposal.
func (rat *ReservationAcceptanceTask) proposeReservationAcceptance(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	candidate *reservationAcceptanceCandidate,
) (*tbtc.ReservationAnchorProposal, error) {
	if candidate == nil || candidate.Deposit == nil {
		return nil, fmt.Errorf("candidate is required")
	}

	taskLogger.Infof("preparing a reservation acceptance proposal")

	anchorFee := reservationAnchorFeeSat

	anchorValue := candidate.Deposit.Utxo.Value - anchorFee
	if anchorValue <= 0 {
		return nil, fmt.Errorf(
			"deposit value [%d] does not cover anchor fee [%d]",
			candidate.Deposit.Utxo.Value,
			anchorFee,
		)
	}

	if uint64(anchorFee) > candidate.TxMaxFee {
		return nil, fmt.Errorf(
			"anchor fee [%d] exceeds the configured max [%d]",
			anchorFee,
			candidate.TxMaxFee,
		)
	}

	taskLogger.Infof("anchor transaction fee: [%d]", anchorFee)

	if _, err := buildReservationAnchorTransaction(
		rat.btcChain,
		candidate.Deposit,
		walletPublicKeyHash,
		anchorFee,
	); err != nil {
		return nil, fmt.Errorf(
			"cannot assemble reservation anchor transaction: [%v]",
			err,
		)
	}

	proposal := &tbtc.ReservationAnchorProposal{
		DepositFundingTxHash:      candidate.Deposit.Utxo.Outpoint.TransactionHash,
		DepositFundingOutputIndex: candidate.Deposit.Utxo.Outpoint.OutputIndex,
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
		return nil, fmt.Errorf(
			"failed to verify reservation anchor proposal: %v",
			err,
		)
	}

	return proposal, nil
}

// buildReservationAnchorTransaction constructs the unsigned reservation
// anchor transaction: a 1-input-1-output spend of the reserved deposit
// into a fresh output controlled by the given wallet. Mirrors the private
// helper in pkg/tbtc/reservation.go; the tbtcpg package cannot call the
// helper directly because it lives in a different package, so the assembly
// logic is duplicated here. Any change to the anchor transaction shape
// must be applied to both sites.
func buildReservationAnchorTransaction(
	bitcoinChain bitcoin.Chain,
	deposit *tbtc.Deposit,
	walletPublicKeyHash [20]byte,
	fee int64,
) (*bitcoin.TransactionBuilder, error) {
	if deposit == nil {
		return nil, fmt.Errorf("deposit is required")
	}

	builder := bitcoin.NewTransactionBuilder(bitcoinChain)

	depositScript, err := deposit.Script()
	if err != nil {
		return nil, fmt.Errorf("cannot get deposit script: [%v]", err)
	}

	err = builder.AddScriptHashInput(deposit.Utxo, depositScript)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot add input pointing to deposit UTXO: [%v]",
			err,
		)
	}

	anchorValue := deposit.Utxo.Value - fee
	if anchorValue <= 0 {
		return nil, fmt.Errorf(
			"transaction fee exceeds the deposit value",
		)
	}

	anchorScript, err := bitcoin.PayToWitnessPublicKeyHash(
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot compute anchor script: [%v]", err)
	}

	builder.AddOutput(&bitcoin.TransactionOutput{
		Value:           anchorValue,
		PublicKeyScript: anchorScript,
	})

	return builder, nil
}

// depositTargetsReservationVault returns true iff the deposit's vault field
// (nil when not set, or pointer to an address) matches the configured
// reservation vault. Address comparison is case-insensitive.
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

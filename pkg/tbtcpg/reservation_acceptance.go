package tbtcpg

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
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

// reservationAcceptanceRequestNonce is the acceptance action generation
// nonce. A reservation does not exist on-chain before its first acceptance
// settles, so the acceptance is always the first action generation
// authorized against a not-yet-created reservation. Every other reservation
// action generation is numbered `reservation.RequestNonce + 1` (see
// ReservationReanchorTask.Run); this constant is that same 1-based
// convention's base case. ReservationAnchorProposal.Unmarshal rejects
// RequestNonce == 0, which is the on-chain confirmation of this convention.
const reservationAcceptanceRequestNonce uint64 = 1

// pendingCandidate holds a candidate deposit event and its
// already-fetched request data.
type pendingCandidate struct {
	Event                 *tbtc.DepositRevealedEvent
	DepositRequest        *tbtc.DepositChainRequest
	ReservationParameters *tbtc.ReservationParameters
}

// ReservationAcceptanceTask is a task that may produce a reservation
// acceptance (anchor) proposal. It scans the chain for reserved deposits
// revealed to the operator's wallet, validates the wallet's eligibility
// against the active reservation caps, and emits a proposal whose resulting
// transaction is a 1-input-1-output anchor that disables the deposit's
// refund path.
type ReservationAcceptanceTask struct {
	chain    Chain
	btcChain bitcoin.Chain

	// scanState guards lastScannedBlock and pendingCandidates: Run may be
	// invoked concurrently for different wallets (one coordinationExecutor
	// goroutine per wallet, sharing this task instance via
	// ProposalGenerator).
	scanState sync.Mutex
	// lastScannedBlock is the block number up to which
	// findReservationAcceptanceCandidate has already scanned
	// DepositRevealed events for a given wallet, so each call only fetches
	// events since the previous call instead of rescanning the full
	// ReservationAcceptanceLookBackBlocks window every time.
	lastScannedBlock map[[20]byte]uint64
	// pendingCandidates holds, per wallet, the reservation-vault-targeting
	// deposit events discovered so far that have not yet resolved (become
	// eligible and accepted, or permanently disqualified). A deposit that
	// is reserved but not yet mature, or briefly blocked by a full cap,
	// must still be reconsidered on a later call even though its block
	// falls before the cursor above; keeping it here is what makes the
	// cursor safe to advance without losing it.
	pendingCandidates map[[20]byte][]*pendingCandidate
}

// NewReservationAcceptanceTask constructs a ReservationAcceptanceTask.
func NewReservationAcceptanceTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *ReservationAcceptanceTask {
	return &ReservationAcceptanceTask{
		chain:             chain,
		btcChain:          btcChain,
		lastScannedBlock:  make(map[[20]byte]uint64),
		pendingCandidates: make(map[[20]byte][]*pendingCandidate),
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
// deposit's reveal context plus the on-chain cap snapshot taken at scan time.
type reservationAcceptanceCandidate struct {
	Deposit               *tbtc.Deposit
	FundingTx             *bitcoin.Transaction
	ReservationParameters *tbtc.ReservationParameters
	TxMaxFee              uint64
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

	candidateEvents, err := rat.scanForCandidateEvents(
		walletPublicKeyHash,
		currentBlock,
		reservationVault,
		reservationParameters,
	)
	if err != nil {
		return nil, err
	}

	if len(candidateEvents) == 0 {
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

	depositMinAgeSeconds, err := rat.chain.GetDepositMinAge()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get deposit minimum age: [%w]",
			err,
		)
	}
	depositMinAge := time.Duration(depositMinAgeSeconds) * time.Second

	now := time.Now()

	var (
		found           *reservationAcceptanceCandidate
		stillUnresolved []*pendingCandidate
	)

	for _, p := range candidateEvents {
		if found != nil {
			stillUnresolved = append(stillUnresolved, p)
			continue
		}
		event := p.Event

		depositKey := rat.chain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		var depositRequest *tbtc.DepositChainRequest
		if p.DepositRequest != nil {
			depositRequest = p.DepositRequest
		} else {
			var foundRequest bool
			var err error
			depositRequest, foundRequest, err = rat.chain.GetDepositRequest(
				event.FundingTxHash,
				event.FundingOutputIndex,
			)
			if err != nil {
				taskLogger.Errorf(
					"failed to get deposit request for [%v]: [%v]",
					depositKey,
					err,
				)
				stillUnresolved = append(stillUnresolved, p)
				continue
			}
			if !foundRequest {
				taskLogger.Warnf(
					"no deposit request for reserved deposit [%v]",
					depositKey,
				)
				stillUnresolved = append(stillUnresolved, p)
				continue
			}
			p.DepositRequest = depositRequest
		}

		// #6: Permanently drop candidate if amount is below minimum.
		if depositRequest.Amount < p.ReservationParameters.ReservationMinAmount {
			taskLogger.Infof("deposit [%v] amount [%d] below minimum; dropping", depositKey, depositRequest.Amount)
			continue
		}

		matureAt := depositRequest.RevealedAt.Add(depositMinAge)
		if !now.After(matureAt) {
			taskLogger.Infof(
				"reserved deposit [%v] is not old enough: now=%v, matureAt=%v",
				depositKey,
				now, matureAt,
			)
			stillUnresolved = append(stillUnresolved, p)
			continue
		}

		if depositRequest.SweptAt.Unix() != 0 {
			taskLogger.Debugf(
				"reserved deposit [%v] is already swept",
				depositKey,
			)
			continue
		}

		candidate := &reservationAcceptanceCandidate{
			ReservationParameters: reservationParameters,
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
			maxReservationsAmountPerWallet,
			reservationMaxSingleAmount,
			reservationParameters,
		) {
			taskLogger.Infof("not eligible: [%v]", depositKey)
			stillUnresolved = append(stillUnresolved, p)
			continue
		}

		fundingTx, err := rat.btcChain.GetTransaction(event.FundingTxHash)
		if err != nil {
			taskLogger.Errorf(
				"failed to get funding tx for reserved deposit [%v]: [%v]",
				depositKey,
				err,
			)
			stillUnresolved = append(stillUnresolved, p)
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
			stillUnresolved = append(stillUnresolved, p)
			continue
		}
		if confirmations < tbtc.DepositSweepRequiredFundingTxConfirmations {
			taskLogger.Debugf(
				"reserved deposit [%v] funding tx confirmations [%d/%d] below required",
				depositKey,
				confirmations,
				tbtc.DepositSweepRequiredFundingTxConfirmations,
			)
			stillUnresolved = append(stillUnresolved, p)
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
			BlindingFactor:      event.BlindingFactor,
			WalletPublicKeyHash: event.WalletPublicKeyHash,
			RefundPublicKeyHash: event.RefundPublicKeyHash,
			RefundLocktime:      event.RefundLocktime,
			Vault:               depositRequest.Vault,
			ExtraData:           depositRequest.ExtraData,
		}
		candidate.FundingTx = fundingTx

		taskLogger.Infof(
			"selected reserved deposit [%v] for acceptance",
			depositKey,
		)

		found = candidate
		stillUnresolved = append(stillUnresolved, p)
	}

	rat.scanState.Lock()
	rat.pendingCandidates[walletPublicKeyHash] = stillUnresolved
	rat.lastScannedBlock[walletPublicKeyHash] = currentBlock
	rat.scanState.Unlock()

	return found, nil
}

func (rat *ReservationAcceptanceTask) scanForCandidateEvents(
	walletPublicKeyHash [20]byte,
	currentBlock uint64,
	reservationVault chain.Address,
	reservationParameters *tbtc.ReservationParameters,
) ([]*pendingCandidate, error) {
	rat.scanState.Lock()
	lastScanned := rat.lastScannedBlock[walletPublicKeyHash]
	pending := rat.pendingCandidates[walletPublicKeyHash]
	rat.scanState.Unlock()

	startBlock := lastScanned + 1
	if lastScanned == 0 {
		startBlock = 0
		if currentBlock > ReservationAcceptanceLookBackBlocks {
			startBlock = currentBlock - ReservationAcceptanceLookBackBlocks
		}
	}
	filter := &tbtc.DepositRevealedEventFilter{
		StartBlock:          startBlock,
		EndBlock:            &currentBlock,
		WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
	}

	revealedEvents, err := rat.chain.PastDepositRevealedEvents(filter)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past deposit revealed events: [%w]",
			err,
		)
	}
	candidates := make([]*pendingCandidate, 0, len(pending)+len(revealedEvents))
	for _, p := range pending {
		candidates = append(candidates, p)
	}
	for _, event := range revealedEvents {
		if !depositTargetsReservationVault(event.Vault, reservationVault) {
			continue
		}
		candidates = append(candidates, &pendingCandidate{
			Event:                 event,
			ReservationParameters: reservationParameters,
		})
	}

	return candidates, nil
}

func (rat *ReservationAcceptanceTask) checkReservationAcceptanceEligibility(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	depositRequest *tbtc.DepositChainRequest,
	walletReservationsCount uint32,
	walletReservationsAmount uint64,
	activeReservationsCount uint32,
	maxActiveReservations uint32,
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

	anchorFee, err := estimateReservationAcceptanceFee(
		rat.btcChain,
		candidate.TxMaxFee,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot estimate reservation acceptance transaction fee: [%v]",
			err,
		)
	}

	anchorValue := candidate.Deposit.Utxo.Value - anchorFee
	if anchorValue <= 0 {
		return nil, false, fmt.Errorf(
			"deposit value [%d] does not cover anchor fee [%d]",
			candidate.Deposit.Utxo.Value,
			anchorFee,
		)
	}

	if candidate.ReservationParameters != nil &&
		uint64(anchorValue) < candidate.ReservationParameters.ReservationMinAmount {
		return nil, false, nil
	}

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
		RequestNonce:              reservationAcceptanceRequestNonce,
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

	reservation, err := rat.chain.GetReservation(reservationKey)
	if err != nil {
		taskLogger.Errorf("cannot get reservation [0x%x]: [%v]", reservationKey, err)
	} else if hasPendingAction(reservationKey, reservation, rat.chain, taskLogger) {
		taskLogger.Infof("reservation [0x%x] has pending action, skipping", reservationKey)
		return nil, false, nil
	}

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

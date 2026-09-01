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
	pendingCandidates map[[20]byte][]*tbtc.DepositRevealedEvent
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
		pendingCandidates: make(map[[20]byte][]*tbtc.DepositRevealedEvent),
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
	ReservationParameters *tbtc.ReservationParameters
	TxMaxFee              uint64
}

// findReservationAcceptanceCandidate returns the first reserved deposit
// that the operator's wallet may accept, or nil when none qualifies.
//
// Discovery is bounded and incremental: PastDepositRevealedEvents is only
// queried for blocks since this wallet's last scan (falling back to
// ReservationAcceptanceLookBackBlocks on the first call), and every
// vault-targeting event found is cached in rat.pendingCandidates so a
// deposit that isn't mature yet, or is briefly blocked by a full cap, is
// still reconsidered on a later call without re-fetching its (already
// past) block range. The vault check runs against event.Vault - already
// present on the DepositRevealedEvent for free - before either
// IsReservedDeposit or GetDepositRequest, so the two RPCs are skipped
// entirely for the common case of a deposit that isn't a reservation.
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
	)
	if err != nil {
		return nil, err
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
		stillUnresolved []*tbtc.DepositRevealedEvent
	)

	for _, event := range candidateEvents {
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
			stillUnresolved = append(stillUnresolved, event)
			continue
		}
		if !isReserved {
			taskLogger.Infof("not reserved deposit [%v]", depositKey)
			// A vault-targeting reveal that the Bridge never marked
			// reserved will never become reserved later; drop it.
			continue
		}

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
			stillUnresolved = append(stillUnresolved, event)
			continue
		}
		if !foundRequest {
			taskLogger.Warnf(
				"no deposit request for reserved deposit [%v]",
				depositKey,
			)
			stillUnresolved = append(stillUnresolved, event)
			continue
		}

		matureAt := depositRequest.RevealedAt.Add(depositMinAge)
		if !now.After(matureAt) {
			taskLogger.Infof(
				"reserved deposit [%v] is not old enough: now=%v, matureAt=%v",
				depositKey,
				now, matureAt,
			)
			stillUnresolved = append(stillUnresolved, event)
			continue
		}

		if depositRequest.SweptAt.Unix() != 0 {
			taskLogger.Debugf(
				"reserved deposit [%v] is already swept",
				depositKey,
			)
			// Already settled one way or another; never a candidate again.
			continue
		}

		if found != nil {
			// Already have this call's candidate; keep the rest pending
			// for the next call rather than dropping or re-fetching them.
			stillUnresolved = append(stillUnresolved, event)
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
			// Caps/state can free up later; keep it pending.
			stillUnresolved = append(stillUnresolved, event)
			continue
		}

		fundingTx, err := rat.btcChain.GetTransaction(event.FundingTxHash)
		if err != nil {
			taskLogger.Errorf(
				"failed to get funding tx for reserved deposit [%v]: [%v]",
				depositKey,
				err,
			)
			stillUnresolved = append(stillUnresolved, event)
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
			stillUnresolved = append(stillUnresolved, event)
			continue
		}
		if confirmations < tbtc.DepositSweepRequiredFundingTxConfirmations {
			taskLogger.Debugf(
				"reserved deposit [%v] funding tx confirmations [%d/%d] below required",
				depositKey,
				confirmations,
				tbtc.DepositSweepRequiredFundingTxConfirmations,
			)
			stillUnresolved = append(stillUnresolved, event)
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
		// Keep the resolved-to-a-proposal event out of the pending set:
		// if this proposal round doesn't settle, the deposit is still
		// IsReservedDeposit and not yet SweptAt, so the next call's fresh
		// pending set naturally would not include it either - it can only
		// be rediscovered by staying in stillUnresolved. Re-add it so a
		// failed round doesn't lose the candidate.
		stillUnresolved = append(stillUnresolved, event)
	}

	rat.scanState.Lock()
	rat.pendingCandidates[walletPublicKeyHash] = stillUnresolved
	rat.scanState.Unlock()

	return found, nil
}

// scanForCandidateEvents returns every reservation-vault-targeting
// DepositRevealed event known for walletPublicKeyHash: the wallet's cached
// pending candidates from previous calls, plus any newly revealed events
// since the last scan. It also advances the wallet's scan cursor to
// currentBlock and merges the new matches into rat.pendingCandidates so a
// later call resumes from here instead of rescanning.
func (rat *ReservationAcceptanceTask) scanForCandidateEvents(
	walletPublicKeyHash [20]byte,
	currentBlock uint64,
	reservationVault chain.Address,
) ([]*tbtc.DepositRevealedEvent, error) {
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
		WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
	}

	revealedEvents, err := rat.chain.PastDepositRevealedEvents(filter)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get past deposit revealed events: [%w]",
			err,
		)
	}

	candidates := pending
	for _, event := range revealedEvents {
		if !depositTargetsReservationVault(event.Vault, reservationVault) {
			continue
		}
		candidates = append(candidates, event)
	}

	rat.scanState.Lock()
	rat.lastScannedBlock[walletPublicKeyHash] = currentBlock
	rat.scanState.Unlock()

	return candidates, nil
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

	anchorFee, err := estimateReservationAcceptanceFee(
		rat.btcChain,
		candidate.TxMaxFee,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot estimate reservation acceptance transaction fee: [%v]",
			err,
		)
	}

	anchorValue := candidate.Deposit.Utxo.Value - anchorFee
	if anchorValue <= 0 {
		return nil, fmt.Errorf(
			"deposit value [%d] does not cover anchor fee [%d]",
			candidate.Deposit.Utxo.Value,
			anchorFee,
		)
	}

	// ReservationParameters.ReservationMinAmount documents itself as "the
	// minimal anchor output amount", i.e. the net value after the anchor
	// fee - not the gross deposit value checked cheaply during eligibility
	// filtering. The fee is only known here, once estimated, so this is the
	// authoritative gate; the eligibility-time gross check is a valid but
	// non-authoritative early filter (gross < min implies net < min too).
	if candidate.ReservationParameters != nil &&
		uint64(anchorValue) < candidate.ReservationParameters.ReservationMinAmount {
		return nil, fmt.Errorf(
			"anchor value [%d] (deposit [%d] minus fee [%d]) is below "+
				"the reservation minimum [%d]",
			anchorValue,
			candidate.Deposit.Utxo.Value,
			anchorFee,
			candidate.ReservationParameters.ReservationMinAmount,
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

// estimateReservationAcceptanceFee estimates the fee for a reservation
// acceptance (anchor) transaction. The transaction has one P2WSH deposit
// input and one P2WPKH output, so its virtual size is fixed for any single
// acceptance. Mirrors ReservationReanchorTask's estimateReservationReanchorFee.
func estimateReservationAcceptanceFee(
	btcChain bitcoin.Chain,
	txMaxFee uint64,
) (int64, error) {
	sizeEstimator := bitcoin.NewTransactionSizeEstimator().
		AddScriptHashInputs(1, depositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true)

	transactionSize, err := sizeEstimator.VirtualSize()
	if err != nil {
		return 0, fmt.Errorf(
			"cannot estimate transaction virtual size: [%v]",
			err,
		)
	}

	feeEstimator := bitcoin.NewTransactionFeeEstimator(btcChain)
	totalFee, err := feeEstimator.EstimateFee(transactionSize)
	if err != nil {
		return 0, fmt.Errorf("cannot estimate transaction fee: [%v]", err)
	}

	if uint64(totalFee) > txMaxFee {
		return 0, fmt.Errorf(
			"estimated fee [%d] exceeds the configured max [%d]",
			totalFee,
			txMaxFee,
		)
	}

	// Enforce the safe minimum fee rate and buffer so a non-RBF reservation
	// acceptance transaction is never broadcast below the floor where it
	// could get stuck and jam the wallet.
	totalFee, err = applyWalletTxFeeFloor(totalFee, transactionSize, txMaxFee)
	if err != nil {
		return 0, err
	}

	return totalFee, nil
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

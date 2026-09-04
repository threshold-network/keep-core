package tbtcpg

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
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

// zeroAddressHex is the Ethereum zero address as returned by the chain
// adapter's address converter (chain.Address(common.Address{}.String())
// never produces an empty string, even for the zero address) -- used to
// detect an unconfigured reservation vault instead of comparing against "".
const zeroAddressHex = "0x0000000000000000000000000000000000000000"

// ReservationAcceptanceTask is a task that may produce a reservation
// acceptance (anchor) proposal. It scans the chain for reserved deposits
// revealed to the operator's wallet, validates the wallet's eligibility
// against the active reservation caps, and emits a proposal whose resulting
// transaction is a 1-input-1-output anchor that disables the deposit's
// refund path.
type ReservationAcceptanceTask struct {
	chain    Chain
	btcChain bitcoin.Chain

	// metricsRecorder is optional and used for recording performance
	// metrics: active_reservations_count, max_active_reservations, and
	// wallet_reservations_count, sourced from the chain calls this task
	// already makes in findReservationAcceptanceCandidate. These are
	// leading indicators of the reservation capacity saturation cliff.
	metricsRecorder interface {
		SetGauge(name string, value float64)
	}

	// scanStateMutex guards scanState. Run() may be invoked for different
	// wallets concurrently, and every call shares this one task instance
	// (see NewProposalGenerator), so the per-wallet scan-state map needs
	// its own lock rather than relying on a single caller goroutine.
	scanStateMutex sync.Mutex
	// scanState holds, per wallet, the incremental deposit-reveal scan
	// cursor and its cached candidate events (see
	// reservationAcceptanceScanState).
	scanState map[[20]byte]*reservationAcceptanceScanState
}

// NewReservationAcceptanceTask constructs a ReservationAcceptanceTask.
func NewReservationAcceptanceTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *ReservationAcceptanceTask {
	return &ReservationAcceptanceTask{
		chain:     chain,
		btcChain:  btcChain,
		scanState: make(map[[20]byte]*reservationAcceptanceScanState),
	}
}

// setMetricsRecorder sets the metrics recorder for the reservation
// acceptance task.
func (rat *ReservationAcceptanceTask) setMetricsRecorder(recorder interface {
	SetGauge(name string, value float64)
}) {
	rat.metricsRecorder = recorder
}

// maxReservationAcceptanceCandidatesPerRun bounds the number of reserved
// deposits examined by findReservationAcceptanceCandidate in a single
// Run() call. Reveals are gas-only (no SPV proof required to appear), so
// reveal volume is not bounded by anything else; this cap keeps per-window
// work bounded even if a wallet's reveal volume spikes.
const maxReservationAcceptanceCandidatesPerRun = 50

// reservationAcceptanceScanState is the per-wallet incremental deposit-
// reveal scan cursor and its in-memory candidate cache, mirroring the
// cursor/cache split used by pkg/maintainer/spv/reservation_proof_loop.go's
// reservationProofScanState: only the block-range delta since the previous
// Run() call is fetched from the chain, while the cached event set is
// still fully re-evaluated against live eligibility state on every call,
// since an already-cached event's temporal maturity, applicable caps, and
// reservation state can all change between calls.
type reservationAcceptanceScanState struct {
	// mutex guards the fields below across the entire read-fetch-merge-
	// prune sequence in depositRevealedEventsSince, not just the map
	// lookup in the caller: two concurrent Run() calls for the same
	// wallet must serialize on this wallet's cursor rather than racing
	// on lastScannedBlock/events.
	mutex            sync.Mutex
	lastScannedBlock uint64
	events           []*tbtc.DepositRevealedEvent
}

// depositRevealedEventsSince returns every DepositRevealedEvent within the
// ReservationAcceptanceLookBackBlocks window for walletPublicKeyHash, using
// this task's per-wallet incremental cursor (see reservationAcceptanceScanState):
// the first call for a wallet performs the full look-back scan; every call
// after fetches only the block-range delta since the previous call and
// merges it into the cached set. Events that have aged out of the
// look-back window are pruned from the cache on every call.
func (rat *ReservationAcceptanceTask) depositRevealedEventsSince(
	walletPublicKeyHash [20]byte,
	currentBlock uint64,
) ([]*tbtc.DepositRevealedEvent, error) {
	rat.scanStateMutex.Lock()
	state, ok := rat.scanState[walletPublicKeyHash]
	if !ok {
		state = &reservationAcceptanceScanState{}
		rat.scanState[walletPublicKeyHash] = state
	}
	rat.scanStateMutex.Unlock()

	state.mutex.Lock()
	defer state.mutex.Unlock()

	windowStartBlock := uint64(0)
	if currentBlock > ReservationAcceptanceLookBackBlocks {
		windowStartBlock = currentBlock - ReservationAcceptanceLookBackBlocks
	}

	fetchStartBlock := windowStartBlock
	if state.lastScannedBlock != 0 {
		fetchStartBlock = state.lastScannedBlock + 1
	}

	if fetchStartBlock <= currentBlock {
		newEvents, err := rat.chain.PastDepositRevealedEvents(
			&tbtc.DepositRevealedEventFilter{
				StartBlock:          fetchStartBlock,
				EndBlock:            &currentBlock,
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get past deposit revealed events: [%w]",
				err,
			)
		}

		state.events = append(state.events, newEvents...)
		state.lastScannedBlock = currentBlock
	}

	// Prune events that have aged out of the look-back window and build a
	// fresh slice, so the caller's in-place sort does not reorder the
	// cached backing array shared across Run() calls for this wallet.
	prunedEvents := make([]*tbtc.DepositRevealedEvent, 0, len(state.events))
	for _, event := range state.events {
		if event.BlockNumber >= windowStartBlock {
			prunedEvents = append(prunedEvents, event)
		}
	}
	state.events = prunedEvents

	events := make([]*tbtc.DepositRevealedEvent, len(prunedEvents))
	copy(events, prunedEvents)
	return events, nil
}

// Run inspects the chain for an acceptance candidate reserved deposit and,
// if one passes the eligibility gate, returns the resulting anchor proposal.
// The task is a no-op (proposal == nil, shouldExecute == false) when no
// candidate exists. A candidate whose proposal generation fails before any
// chain-state-mutating call (assemble/validate) is skipped in favor of the
// next candidate rather than aborting the window outright -- see
// reservationAcceptancePreWriteError. A failure after a write still aborts
// the window, since a partial on-chain effect may already exist.
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

	skipDepositKeys := make(map[string]bool)

	for {
		candidate, err := rat.findReservationAcceptanceCandidate(
			taskLogger,
			walletPublicKeyHash,
			skipDepositKeys,
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
			var preWriteErr *reservationAcceptancePreWriteError
			if errors.As(err, &preWriteErr) {
				taskLogger.Warnf(
					"reservation acceptance candidate [%v] failed before "+
						"any chain-state-mutating call, trying next "+
						"candidate: [%v]",
					candidate.DepositKey,
					err,
				)
				skipDepositKeys[candidate.DepositKey.Text(16)] = true
				continue
			}
			return nil, false, fmt.Errorf(
				"cannot prepare reservation acceptance proposal: [%w]",
				err,
			)
		}

		if proposal == nil {
			return nil, shouldExecute, nil
		}
		return proposal, shouldExecute, nil
	}
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
	DepositKey            *big.Int
	Deposit               *tbtc.Deposit
	FundingTx             *bitcoin.Transaction
	ReservationParameters *tbtc.ReservationParameters
	TxMaxFee              uint64
	RequestNonce          uint64
	AnchorFee             int64
}

// findReservationAcceptanceCandidate returns the first reserved deposit
// that the operator's wallet may accept, or nil when none qualifies.
// skipDepositKeys (keyed by depositKey.Text(16)) excludes deposits the
// caller already tried and rejected earlier in the same Run() call, so a
// deposit whose proposal generation fails pre-write does not block every
// other candidate on the wallet.
func (rat *ReservationAcceptanceTask) findReservationAcceptanceCandidate(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	skipDepositKeys map[string]bool,
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
	if reservationVault == "" || reservationVault == chain.Address(zeroAddressHex) {
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
	if rat.metricsRecorder != nil {
		rat.metricsRecorder.SetGauge(
			"wallet_reservations_count",
			float64(walletReservationsCount),
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
	if rat.metricsRecorder != nil {
		rat.metricsRecorder.SetGauge(
			"active_reservations_count",
			float64(activeReservationsCount),
		)
		rat.metricsRecorder.SetGauge(
			"max_active_reservations",
			float64(maxActiveReservations),
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

	if uint64(reservationParameters.ReservationActionTimeout) <= uint64(depositMinAgeSeconds) {
		taskLogger.Errorf(
			"misconfiguration: ReservationActionTimeout [%d] <= DEPOSIT_MIN_AGE [%d]; "+
				"every reserved deposit will be marked stale before it can become "+
				"acceptance-eligible",
			reservationParameters.ReservationActionTimeout,
			depositMinAgeSeconds,
		)
	}

	depositRevealedEvents, err := rat.depositRevealedEventsSince(
		walletPublicKeyHash,
		currentBlock,
	)
	if err != nil {
		return nil, err
	}

	// Take the oldest first.
	sort.SliceStable(depositRevealedEvents, func(i, j int) bool {
		return depositRevealedEvents[i].BlockNumber < depositRevealedEvents[j].BlockNumber
	})

	now := time.Now()

	candidatesExamined := 0
	for _, event := range depositRevealedEvents {
		if !depositTargetsReservationVault(event.Vault, reservationVault) {
			continue
		}

		if candidatesExamined >= maxReservationAcceptanceCandidatesPerRun {
			taskLogger.Warnf(
				"reached max reservation acceptance candidates per run "+
					"[%d]; remaining reserved deposits will be examined "+
					"on a subsequent run",
				maxReservationAcceptanceCandidatesPerRun,
			)
			break
		}
		candidatesExamined++

		depositKey := rat.chain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		if skipDepositKeys[depositKey.Text(16)] {
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

		// Determine RequestNonce and re-request eligibility from the
		// reservation's own on-chain state, which authoritatively reflects
		// whether a prior generation is still pending -- not from acceptance-
		// requested event history, which would still show a first generation
		// that has since timed out and become eligible for retry again.
		var requestNonce uint64 = 1
		reservation, err := rat.chain.GetReservation(depositKey)
		if err != nil {
			// Fail safe: the production chain adapter never errors for "not
			// found" (it returns a zero record with State == Unknown), so a
			// non-nil error here can only be an RPC/decode failure -- not a
			// signal that the reservation is not yet created. Treating it as
			// "assume not yet created" would skip both the eligible-state
			// gate and the hasPendingAction gate below. Skip this deposit for
			// the current coordination window instead; the next window
			// retries (mirrors the fail-safe policy in hasPendingAction).
			taskLogger.Errorf(
				"cannot get reservation [%v], skipping deposit for this window: [%v]",
				depositKey,
				err,
			)
			continue
		}

		// "Not yet created" is derived only from a successful read: a zero
		// record reports State == Unknown with RequestNonce == 0, in which
		// case the predicted requestNonce of 1 (set above) already applies
		// and the gates below do not apply.
		if reservation != nil &&
			!(reservation.State == tbtc.ReservationStateUnknown && reservation.RequestNonce == 0) {
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
			DepositKey: depositKey,
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

// hasPendingAction reports whether the on-chain reservation action
// generation at the reservation's current request nonce (if any) is in a
// pending state. This guards against duplicate acceptance requests within
// findReservationAcceptanceCandidate: the Bridge rejects a new request
// while the previous generation is still in flight. The caller supplies
// the reservation record it already fetched rather than this function
// re-reading it.
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
		// caller send a duplicate acceptance request that the Bridge
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

	if maxActiveReservations == 0 {
		taskLogger.Errorf(
			"active reservations cap (maxActiveReservations) not configured " +
				"(is 0); failing closed rather than treating as unlimited",
		)
		return false
	}
	if activeReservationsCount >= maxActiveReservations {
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

	if reservationParameters.ReservationMaxTotalAmount == 0 {
		taskLogger.Errorf(
			"global reservation total amount cap (ReservationMaxTotalAmount) " +
				"not configured (is 0); failing closed rather than treating as unlimited",
		)
		return false
	}
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

	return true
}

// reservationAcceptancePreWriteError wraps a proposeReservationAcceptance
// failure that occurred before any chain-state-mutating call (assemble or
// validate). The caller (Run) treats this as "this deposit is doomed" and
// skips it in favor of the next candidate instead of aborting the whole
// coordination window -- a failure after a write (RequestReservationAcceptance
// or the post-request GetReservation check) still aborts the window as
// before, since a partial on-chain effect may already exist.
type reservationAcceptancePreWriteError struct {
	err error
}

func (e *reservationAcceptancePreWriteError) Error() string {
	return e.err.Error()
}

func (e *reservationAcceptancePreWriteError) Unwrap() error {
	return e.err
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
		return nil, false, &reservationAcceptancePreWriteError{
			err: fmt.Errorf(
				"cannot assemble reservation anchor transaction: [%v]",
				err,
			),
		}
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
		return nil, false, &reservationAcceptancePreWriteError{
			err: fmt.Errorf(
				"failed to verify reservation anchor proposal: %v",
				err,
			),
		}
	}

	// RequestReservationAcceptance is called as a side effect of proposal
	// generation itself (before coordination has agreed to anything), which
	// is a known, accepted deviation from the read-only-during-generation
	// pattern (also present in MovingFundsTask's SubmitMovingFundsCommitment).
	// The guards against re-requesting acceptance for existing or pending
	// reservations (checked in findReservationAcceptanceCandidate) are the
	// primary mitigation for spurious repeat writes.
	if err := rat.chain.RequestReservationAcceptance(
		reservationKey,
		walletPublicKeyHash,
	); err != nil {
		return nil, false, fmt.Errorf("cannot request reservation acceptance: [%v]", err)
	}

	updatedReservation, err := rat.chain.GetReservation(reservationKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot re-read reservation: [%v]", err)
	}
	if updatedReservation.RequestNonce != candidate.RequestNonce {
		return nil, false, fmt.Errorf(
			"reservation request nonce mismatch after request: predicted [%d], on-chain [%d]",
			candidate.RequestNonce,
			updatedReservation.RequestNonce,
		)
	}
	proposal.RequestNonce = updatedReservation.RequestNonce

	return proposal, true, nil
}

func estimateReservationAcceptanceFee(
	btcChain bitcoin.Chain,
	txMaxFee uint64,
) (int64, error) {
	sizeEstimator := bitcoin.NewTransactionSizeEstimator().
		AddScriptHashInputs(1, DepositScriptByteSize, true).
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

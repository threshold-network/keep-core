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

	// fundingTxLookupTimeout bounds a single candidate's
	// GetTransactionConfirmations call in
	// fetchReservationAcceptanceFundingTxs. NewReservationAcceptanceTask
	// defaults it to reservationAcceptanceFundingTxLookupTimeout; it is an
	// instance field, rather than that constant being read directly,
	// purely so a unit test can substitute a millisecond-scale value and
	// observe a real per-candidate timeout firing without the test itself
	// having to block for the production 30s default.
	fundingTxLookupTimeout time.Duration
}

// NewReservationAcceptanceTask constructs a ReservationAcceptanceTask.
func NewReservationAcceptanceTask(
	chain Chain,
	btcChain bitcoin.Chain,
) *ReservationAcceptanceTask {
	return &ReservationAcceptanceTask{
		chain:                  chain,
		btcChain:               btcChain,
		scanState:              make(map[[20]byte]*reservationAcceptanceScanState),
		fundingTxLookupTimeout: reservationAcceptanceFundingTxLookupTimeout,
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

// reservationAcceptanceFundingTxLookupWorkers bounds the number of reserved
// deposits whose funding-transaction lookups (GetTransaction and
// GetTransactionConfirmations) run concurrently against the Bitcoin chain
// adapter in fetchReservationAcceptanceFundingTxs. Fetching serially for up
// to maxReservationAcceptanceCandidatesPerRun candidates, each bound only
// by the chain adapter's own multi-minute retry budget, could turn one
// unhealthy Bitcoin backend into hours of serial retries before a run
// gives up; running a small bounded number of lookups concurrently instead
// caps the number of sequential retry windows to roughly
// maxReservationAcceptanceCandidatesPerRun /
// reservationAcceptanceFundingTxLookupWorkers.
const reservationAcceptanceFundingTxLookupWorkers = 8

// reservationAcceptanceFundingTxLookupTimeout bounds a single candidate's
// GetTransactionConfirmations call in fetchReservationAcceptanceFundingTxs.
// It is set well below a Bitcoin chain adapter's own default per-request
// retry budget (for the Electrum adapter, bitcoin/electrum.
// DefaultRequestRetryTimeout is two minutes) so an unhealthy backend fails
// a candidate's lookup fast instead of silently consuming the adapter's
// full retry budget on every one of the bounded pipeline's concurrent
// slots. GetTransaction itself takes no context (see bitcoin.Chain) and
// remains bound only by the adapter's own retry policy. This is the
// default assigned to ReservationAcceptanceTask.fundingTxLookupTimeout by
// NewReservationAcceptanceTask; see that field for why it is not read
// directly.
const reservationAcceptanceFundingTxLookupTimeout = 30 * time.Second

// reservationAcceptanceFundingTxCandidate is a phase-one-eligible reserved
// deposit awaiting the concurrent funding-transaction lookup performed by
// fetchReservationAcceptanceFundingTxs.
type reservationAcceptanceFundingTxCandidate struct {
	event          *tbtc.DepositRevealedEvent
	depositKey     *big.Int
	depositRequest *tbtc.DepositChainRequest
}

// reservationAcceptanceFundingTxLookup is the outcome of one candidate's
// funding-transaction lookup. fundingTxErr and confirmationsErr are tracked
// separately, instead of a single combined error, so the caller can
// reproduce the exact log message a strictly serial GetTransaction ->
// GetTransactionConfirmations call chain would have emitted for whichever
// of the two calls failed.
type reservationAcceptanceFundingTxLookup struct {
	fundingTx        *bitcoin.Transaction
	fundingTxErr     error
	confirmations    uint
	confirmationsErr error
}

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
		return nil, fmt.Errorf(
			"failed to load wallet chain data: [%w]",
			err,
		)
	}

	// wallet_reservations_count / active_reservations_count /
	// max_active_reservations are published unconditionally on every
	// findReservationAcceptanceCandidate pass, mirroring the sibling
	// live_wallets_count gauge that ReservationReanchorTask publishes every
	// coordination window: they must not depend on the StateLive guard
	// below, which is false for a wallet mid-rotation (moving funds or
	// closing) -- exactly when gauge staleness matters most. Gating the
	// publish on that guard would leave these gauges stuck at their
	// registered-zero value for as long as the wallet is not Live.
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

	walletReservationsAmount, err := rat.chain.WalletReservationsAmount(
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get wallet reservations amount: [%w]",
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

	// The anchor fee is identical for every candidate examined below (same
	// fixed-size 1-input-1-output anchor transaction, same fee-rate oracle
	// and cap). It is estimated at most once per Run() call -- on the
	// first candidate that survives every earlier per-candidate gate --
	// and the result is cached in anchorFee/anchorFeeComputed for reuse by
	// every subsequent candidate, instead of being unconditionally
	// recomputed for each of the up to
	// maxReservationAcceptanceCandidatesPerRun candidates examined by the
	// loop. It is intentionally not computed further up in this function:
	// a wallet with no candidate that reaches this gate (e.g. no reserved
	// deposits at all) must remain a clean no-op, without paying for a fee
	// estimate it will never use.
	var anchorFee int64
	anchorFeeComputed := false
	now := time.Now()

	var pendingCandidates []*reservationAcceptanceFundingTxCandidate

	candidatesExamined := 0
	for _, event := range depositRevealedEvents {
		if !depositTargetsReservationVault(event.Vault, reservationVault) {
			continue
		}

		depositKey := rat.chain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		if skipDepositKeys[depositKey.Text(16)] {
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

		pendingCandidates = append(
			pendingCandidates,
			&reservationAcceptanceFundingTxCandidate{
				event:          event,
				depositKey:     depositKey,
				depositRequest: depositRequest,
			},
		)
	}

	// The two Electrum calls below (GetTransaction and
	// GetTransactionConfirmations) are looked up through a bounded,
	// lazily-scheduled pipeline (see fetchReservationAcceptanceFundingTxs)
	// instead of eagerly for every phase-one-eligible candidate collected
	// above, so an unhealthy Bitcoin backend cannot turn this bounded
	// candidate set into a fully serial multi-hour retry chain, AND a
	// caller that only needs the first fully eligible candidate -- the
	// common case, since candidates are examined oldest-first -- does not
	// pay for fetching every other pending candidate's funding
	// transaction. fundingTxLookups.next(i) still hands back each
	// candidate's result in the original oldest-first order, so the loop
	// below still returns the first fully eligible one deterministically,
	// exactly as the previous strictly serial implementation did;
	// deferring stop() here ensures every exit from this loop -- a
	// candidate found, or every candidate exhausted -- tells the pipeline
	// to abandon any lookup it has not yet dispatched.
	fundingTxLookups := rat.fetchReservationAcceptanceFundingTxs(pendingCandidates)
	defer fundingTxLookups.stop()

	for i, pendingCandidate := range pendingCandidates {
		event := pendingCandidate.event
		depositKey := pendingCandidate.depositKey
		depositRequest := pendingCandidate.depositRequest
		lookup := fundingTxLookups.next(i)

		if lookup.fundingTxErr != nil {
			taskLogger.Errorf(
				"failed to get funding tx for reserved deposit [%v]: [%v]",
				depositKey,
				lookup.fundingTxErr,
			)
			continue
		}
		fundingTx := lookup.fundingTx

		if lookup.confirmationsErr != nil {
			taskLogger.Errorf(
				"failed to get funding tx confirmations for [%v]: [%v]",
				depositKey,
				lookup.confirmationsErr,
			)
			continue
		}
		if lookup.confirmations < tbtc.DepositSweepRequiredFundingTxConfirmations {
			taskLogger.Debugf(
				"reserved deposit [%v] funding tx confirmations [%d/%d] below required",
				depositKey,
				lookup.confirmations,
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

		// Check net-of-fee viability here, as part of candidate selection,
		// rather than after a single candidate has already been chosen. A
		// candidate that fails this check is skipped in favor of the next
		// one; nothing marks it retried, so leaving this check in
		// proposeReservationAcceptance (which is called for exactly one
		// already-selected candidate) would cause the same doomed deposit
		// to be re-selected and abort on every subsequent Run() until it
		// aged out of the look-back window.
		if !anchorFeeComputed {
			var feeErr error
			anchorFee, feeErr = estimateReservationAcceptanceFee(
				rat.btcChain,
				reservationParameters.ReservationTxMaxFee,
			)
			if feeErr != nil {
				return nil, fmt.Errorf(
					"failed to estimate reservation acceptance transaction fee: [%w]",
					feeErr,
				)
			}
			anchorFeeComputed = true
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

// reservationAcceptanceFundingTxPipeline is a lazily-scheduled, bounded,
// oldest-first funding-transaction lookup pipeline returned by
// fetchReservationAcceptanceFundingTxs. A naive worker pool launches every
// candidate's GetTransaction/GetTransactionConfirmations call
// unconditionally, so a caller that only needs the first fully eligible
// candidate -- the common case, since candidates are examined oldest-first
// -- still pays for fetching every other pending candidate's funding
// transaction. This pipeline instead hands results back one index at a
// time via next(), and stop() tells it to abandon every lookup it has not
// yet dispatched, so a caller that stops asking for more results after an
// early match never causes those later fetches to happen at all.
type reservationAcceptanceFundingTxPipeline struct {
	// results holds one buffered (capacity 1) channel per candidate index.
	// Buffering lets a fetch that completes after stop() has already been
	// called still deliver its result and exit, instead of blocking
	// forever on a receiver that will never call next() for that index.
	results []chan reservationAcceptanceFundingTxLookup

	stopCh   chan struct{}
	stopOnce sync.Once
}

// next blocks until candidate index i's funding-transaction lookup
// completes and returns its result. Callers MUST consume indexes in
// increasing order (0, 1, 2, ...) -- the same oldest-first order
// candidates was given to fetchReservationAcceptanceFundingTxs in. Because
// each candidate's result is delivered on its own per-index channel, a
// later candidate's fetch racing ahead and completing first can never be
// mistaken for an earlier candidate's result.
func (p *reservationAcceptanceFundingTxPipeline) next(
	i int,
) reservationAcceptanceFundingTxLookup {
	return <-p.results[i]
}

// stop tells the pipeline to abandon every funding-transaction lookup not
// already dispatched. It is idempotent and safe to call even after every
// candidate has already been consumed via next(), or not at all. A lookup
// already in flight when stop is called is not interrupted -- it still
// runs to completion or hits its own
// reservationAcceptanceFundingTxLookupTimeout. The dispatcher checks
// stopCh before acquiring each new dispatch slot, giving stop() priority
// over launching another lookup, so calling stop typically bounds the
// number of "wasted" fetches past the caller's answer to roughly
// reservationAcceptanceFundingTxLookupWorkers - 1 -- but because Go's
// select can still occasionally choose an already-ready dispatch slot
// over an already-closed stopCh, this is a best-effort reduction, not a
// strict bound, no matter how many candidates remain unexamined.
func (p *reservationAcceptanceFundingTxPipeline) stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
}

// fetchReservationAcceptanceFundingTxs starts looking up the funding
// transaction and its confirmation count for every candidate in
// candidates, in oldest-first order, and returns a
// reservationAcceptanceFundingTxPipeline the caller pulls results from one
// index at a time via next(). At most
// reservationAcceptanceFundingTxLookupWorkers lookups ever run
// concurrently -- a strict cap, exactly as the previous eager
// implementation enforced with its shared worker goroutines. This is a
// bound on concurrency, not on the total number of Bitcoin RPCs issued
// over the pipeline's lifetime: unlike that implementation, which
// launched every candidate's fetch before the caller could inspect any
// result, fetches here are dispatched lazily and the dispatcher only
// stops handing out new candidates once the caller calls the returned
// pipeline's stop() (see findReservationAcceptanceCandidate, which does
// so via defer once it either finds a fully eligible candidate or
// exhausts every candidate). Because the dispatcher waits on stop(), not
// on the caller actually consuming each result via next(), a caller whose
// own per-candidate work (e.g. Ethereum round-trips) is slower than the
// Bitcoin lookups can still see up to every one of
// maxReservationAcceptanceCandidatesPerRun (50) pending candidates
// dispatched before stop() is observed; the early-stop savings this
// affords on top of the strict concurrency cap are best-effort, not an
// absolute bound on total RPCs issued (see
// reservationAcceptanceFundingTxLookupTimeout for the per-candidate
// timeout that still applies to each dispatched lookup).
func (rat *ReservationAcceptanceTask) fetchReservationAcceptanceFundingTxs(
	candidates []*reservationAcceptanceFundingTxCandidate,
) *reservationAcceptanceFundingTxPipeline {
	pipeline := &reservationAcceptanceFundingTxPipeline{
		results: make([]chan reservationAcceptanceFundingTxLookup, len(candidates)),
		stopCh:  make(chan struct{}),
	}
	for i := range pipeline.results {
		pipeline.results[i] = make(chan reservationAcceptanceFundingTxLookup, 1)
	}
	if len(candidates) == 0 {
		return pipeline
	}

	workers := reservationAcceptanceFundingTxLookupWorkers
	if workers > len(candidates) {
		workers = len(candidates)
	}
	dispatchSlots := make(chan struct{}, workers)

	// The dispatcher below walks candidates in their given (oldest-first)
	// order, acquiring a dispatchSlots slot before starting each one's
	// fetch, so at most `workers` fetches ever run concurrently -- a
	// strict bound, the same one the previous eager implementation
	// enforced with its shared worker goroutines. It gives stop()
	// priority over dispatching another candidate: it checks
	// pipeline.stopCh non-blockingly before attempting to acquire a slot,
	// and returns immediately without acquiring or launching if stop has
	// already been observed. This narrows, but -- because Go's select can
	// still occasionally choose an already-ready dispatchSlots send over
	// an already-closed stopCh in the slot-acquisition select below --
	// does not strictly eliminate, the window in which one extra
	// candidate's fetch can be launched after the caller calls stop().
	go func() {
		for i, candidate := range candidates {
			select {
			case <-pipeline.stopCh:
				return
			default:
			}

			select {
			case dispatchSlots <- struct{}{}:
			case <-pipeline.stopCh:
				return
			}

			go func(i int, candidate *reservationAcceptanceFundingTxCandidate) {
				defer func() { <-dispatchSlots }()

				var lookup reservationAcceptanceFundingTxLookup
				fundingTxHash := candidate.event.FundingTxHash

				fundingTx, err := rat.btcChain.GetTransaction(fundingTxHash)
				if err != nil {
					lookup.fundingTxErr = err
					pipeline.results[i] <- lookup
					return
				}
				lookup.fundingTx = fundingTx

				fetchCtx, cancelFetchCtx := context.WithTimeout(
					context.Background(),
					rat.fundingTxLookupTimeout,
				)
				confirmations, err := rat.btcChain.GetTransactionConfirmations(
					fetchCtx,
					fundingTxHash,
				)
				cancelFetchCtx()
				if err != nil {
					lookup.confirmationsErr = err
					pipeline.results[i] <- lookup
					return
				}
				lookup.confirmations = confirmations

				pipeline.results[i] <- lookup
			}(i, candidate)
		}
	}()

	return pipeline
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

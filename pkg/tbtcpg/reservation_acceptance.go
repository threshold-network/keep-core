package tbtcpg

import (
	"context"
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

// reservationAcceptanceRequestLookBackMarginBlocks is added to the
// on-chain reservation action timeout (converted to blocks) to form the
// look-back window of the ReservationAcceptanceRequested scan. Solidity's
// requestReservationAcceptance sets timeoutAt = request time + the action
// timeout, so a generation that can still be signed was requested within
// that timeout; the margin (about one day at 12 seconds per block) absorbs
// block-time drift and a moderate governance decrease of the timeout.
const reservationAcceptanceRequestLookBackMarginBlocks = uint64(7200)

// reservationAcceptanceRevealLookBackMargin is added to a pending
// acceptance generation's snapshotted term to bound how far before the
// request the deposit's DepositRevealed event is searched for.
// Deposit.sol caps a reserved deposit's refund deadline at reveal time +
// term + 24 hours, and requestReservationAcceptance requires timeoutAt +
// 24 hours <= refund deadline, so the reveal happened at most one term
// before the request. The cap uses the term in force at reveal time,
// which governance may have lowered before the request; the margin
// absorbs such a decrease of up to about a week, plus block-time drift.
const reservationAcceptanceRevealLookBackMargin = 7 * 24 * time.Hour

// zeroAddressHex is the Ethereum zero address as returned by the chain
// adapter's address converter (chain.Address(common.Address{}.String())
// never produces an empty string, even for the zero address) -- used to
// detect an unconfigured reservation vault instead of comparing against "".
const zeroAddressHex = "0x0000000000000000000000000000000000000000"

// ReservationAcceptanceTask is a task that may produce a reservation
// acceptance (anchor) proposal. It discovers the Pending Acceptance
// generations depositors requested for the operator's wallet from
// ReservationAcceptanceRequested events, and emits a proposal whose
// resulting transaction is a 1-input-1-output anchor that disables the
// deposit's refund path.
type ReservationAcceptanceTask struct {
	chain    Chain
	btcChain bitcoin.Chain

	// metricsRecorder is optional and used for recording performance
	// metrics: active_reservations_count, max_active_reservations,
	// wallet_reservations_count, reservation_vault_fee_debt_sat, and
	// reservation_vault_fee_reserve_tbtc_base_units, published once per
	// Run. The occupancy gauges reuse chain reads the task makes anyway;
	// the two vault fee gauges add two reads of the vault per Run. They
	// are leading indicators of reservation capacity saturation and of
	// reservation fee pressure.
	metricsRecorder interface {
		SetGauge(name string, value float64)
	}

	// revealCacheMutex guards revealCache. Run() may be invoked for
	// different wallets concurrently, and every call shares this one task
	// instance (see NewProposalGenerator).
	revealCacheMutex sync.Mutex
	// revealCache holds, per wallet, the DepositRevealed events found for
	// the previous Run's pending acceptance candidates, keyed by
	// depositKey.Text(16). Reveal events never change, and a depositor may
	// request acceptance long after the reveal, so a candidate re-examined
	// in a later window does not repeat its reveal search. Each Run
	// replaces its wallet's entry with the reveals of its own candidates,
	// so the cache never outgrows the current candidate set.
	revealCache map[[20]byte]map[string]*tbtc.DepositRevealedEvent

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
		revealCache:            make(map[[20]byte]map[string]*tbtc.DepositRevealedEvent),
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

// maxReservationAcceptanceCandidatesPerRun bounds the number of pending
// acceptance generations examined past the on-chain pending-acceptance
// check in a single Run() call. Only generations a depositor requested
// for this wallet count toward it; each such request reserved wallet
// capacity on-chain, so the per-wallet reservation caps also bound how
// many can exist. The cap keeps per-window reveal lookups, Ethereum reads
// and Bitcoin lookups bounded.
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

// reservationAcceptanceFundingTxCandidate is a reserved deposit whose
// current generation is a Pending Acceptance targeting the operator's
// wallet and whose reveal and deposit request passed every Ethereum-side
// check, awaiting the concurrent funding-transaction lookup performed by
// fetchReservationAcceptanceFundingTxs.
type reservationAcceptanceFundingTxCandidate struct {
	event          *tbtc.DepositRevealedEvent
	depositKey     *big.Int
	depositRequest *tbtc.DepositChainRequest
	requestNonce   uint64
	action         *tbtc.ReservationAction
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

// reservationAcceptanceFeeEstimate is a cached anchor fee estimate for one
// snapshotted fee cap.
type reservationAcceptanceFeeEstimate struct {
	fee int64
	err error
}

// Run inspects the chain for a reserved deposit whose depositor requested
// acceptance by the operator's wallet and, if one passes every signer-time
// rule, returns the resulting anchor proposal. The per-window setup
// (parameters, wallet, gauges, candidate discovery) runs once; candidates
// are then tried in order. A candidate that fails a later check, the fee
// estimate, or proposal generation is logged and skipped in favor of the
// next one: the task performs no chain-state-mutating call, so a failed
// candidate leaves nothing behind. The task is a no-op (proposal == nil,
// shouldExecute == false) when no candidate succeeds.
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

	pendingCandidates, reservationParameters, err :=
		rat.findReservationAcceptanceCandidates(
			taskLogger,
			walletPublicKeyHash,
		)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot find reservation acceptance candidate: [%w]",
			err,
		)
	}

	// The two Electrum calls (GetTransaction and
	// GetTransactionConfirmations) are looked up through a bounded,
	// lazily-scheduled pipeline (see fetchReservationAcceptanceFundingTxs),
	// so an unhealthy Bitcoin backend cannot turn the candidate set into a
	// fully serial multi-hour retry chain, and the common case -- the
	// first candidate succeeds -- does not pay for fetching every other
	// candidate's funding transaction. fundingTxLookups.next(i) hands back
	// results in candidate order; deferring stop() tells the pipeline to
	// abandon any lookup it has not yet dispatched once Run returns.
	fundingTxLookups := rat.fetchReservationAcceptanceFundingTxs(pendingCandidates)
	defer fundingTxLookups.stop()

	feeEstimates := make(map[uint64]reservationAcceptanceFeeEstimate)

	for i, pendingCandidate := range pendingCandidates {
		candidate := rat.completeReservationAcceptanceCandidate(
			taskLogger,
			pendingCandidate,
			fundingTxLookups.next(i),
			feeEstimates,
			reservationParameters,
		)
		if candidate == nil {
			continue
		}

		proposal, err := rat.proposeReservationAcceptance(
			taskLogger,
			walletPublicKeyHash,
			candidate,
		)
		if err != nil {
			taskLogger.Warnf(
				"reservation acceptance candidate [%v] failed, trying "+
					"next candidate: [%v]",
				candidate.DepositKey,
				err,
			)
			continue
		}

		return proposal, true, nil
	}

	taskLogger.Info("no reservation acceptance candidate")
	return nil, false, nil
}

// ActionType returns the wallet action type this task proposes.
func (rat *ReservationAcceptanceTask) ActionType() tbtc.WalletActionType {
	return tbtc.ActionReservationAnchor
}

// reservationAcceptanceCandidate is the bundle a candidate reserved deposit
// for acceptance carries through the proposal builder. It captures the
// deposit's reveal context, the pending acceptance generation's real request
// nonce and snapshotted fields, plus the reservation parameters read at
// scan time.
//
// On-chain, only the deposit's depositor may call requestReservationAcceptance,
// and that call is what writes the Pending Acceptance action record. This
// task consumes, rather than creates, that record: candidate selection
// accepts only deposits whose current generation is a Pending Acceptance
// action record targeting this wallet, and the proposal is built from that
// record's real request nonce and snapshotted fee bound, matching the
// on-chain validator, which validates against the same record.
type reservationAcceptanceCandidate struct {
	DepositKey            *big.Int
	Deposit               *tbtc.Deposit
	FundingTx             *bitcoin.Transaction
	ReservationParameters *tbtc.ReservationParameters
	TxMaxFee              uint64
	RequestNonce          uint64
	AnchorFee             int64
}

// findReservationAcceptanceCandidates performs the per-window setup (it
// reads the reservation parameters and wallet, publishes the gauges) and
// returns, in the order they should be tried, the reserved deposits whose
// current generation is a Pending Acceptance targeting the wallet and
// whose reveal and deposit request pass every Ethereum-side check.
//
// Candidates are discovered from the wallet's ReservationAcceptanceRequested
// events rather than from deposit reveals: only a depositor's request
// creates an anchorable generation, and it may come long after the
// reveal. The on-chain pending-acceptance check runs before a candidate
// counts toward maxReservationAcceptanceCandidatesPerRun and before any
// reveal search or Bitcoin lookup, so reveals that were never requested
// cannot use up the budget.
func (rat *ReservationAcceptanceTask) findReservationAcceptanceCandidates(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
) (
	[]*reservationAcceptanceFundingTxCandidate,
	*tbtc.ReservationParameters,
	error,
) {
	if walletPublicKeyHash == [20]byte{} {
		return nil, nil, fmt.Errorf("wallet public key hash is required")
	}

	reservationParameters, err := rat.chain.ReservationParameters()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to get reservation parameters: [%w]",
			err,
		)
	}
	reservationVault := reservationParameters.ReservationVault
	if reservationVault == "" || reservationVault == chain.Address(zeroAddressHex) {
		// An unconfigured vault has no fee ledger, so the vault fee
		// gauges must be published as zero rather than left at whatever
		// a configured vault published on an earlier pass: a gauge that
		// retains a stale nonzero reading is indistinguishable from a
		// real debt/reserve.
		if rat.metricsRecorder != nil {
			rat.metricsRecorder.SetGauge(
				"reservation_vault_fee_debt_sat",
				0,
			)
			rat.metricsRecorder.SetGauge(
				"reservation_vault_fee_reserve_tbtc_base_units",
				0,
			)
		}
		taskLogger.Info("reservation vault not configured")
		return nil, reservationParameters, nil
	}

	wallet, err := rat.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to load wallet chain data: [%w]",
			err,
		)
	}

	if err := rat.publishReservationGauges(
		taskLogger,
		walletPublicKeyHash,
		reservationVault,
	); err != nil {
		return nil, nil, err
	}

	// Mirror WalletProposalValidator.sol's
	// requireWalletLiveOrMovingFunds: the on-chain validator accepts an
	// anchor proposal from a wallet in either the Live or the MovingFunds
	// state, and both states legitimately may hold pending acceptance
	// generations created by a depositor. Only a wallet past that
	// transition (Closing, Closed, Terminated, or unknown) is ineligible.
	if wallet.State != tbtc.StateLive &&
		wallet.State != tbtc.StateMovingFunds {
		taskLogger.Infof(
			"wallet is not live or moving funds (state=%v); "+
				"cannot accept reservation",
			wallet.State,
		)
		return nil, reservationParameters, nil
	}

	blockCounter, err := rat.chain.BlockCounter()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get block counter: [%w]", err)
	}
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to get current block: [%w]",
			err,
		)
	}

	// Request-time capacity caps are deliberately not re-checked on this
	// path: Solidity's requestReservationAcceptance already reserved the
	// active count, wallet count and amount, and global total capacity
	// when the Pending Acceptance generation this task consumes was
	// created. Re-applying the caps at consumption time would
	// double-count that generation against them, so only the signer-time
	// rules the validator enforces are mirrored here and in
	// completeReservationAcceptanceCandidate. Request-time cap checks
	// remain in place where a fresh request is made: the re-anchor task's
	// target headroom pre-check and its request flow.

	depositMinAgeSeconds, err := rat.chain.GetDepositMinAge()
	if err != nil {
		return nil, nil, fmt.Errorf(
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

	averageBlockTime := rat.chain.AverageBlockTime()
	if averageBlockTime <= 0 {
		averageBlockTime = tbtc.DepositRevealLookupDefaultBlockTime
	}

	requestEvents, err := rat.reservationAcceptanceRequestedEvents(
		walletPublicKeyHash,
		currentBlock,
		reservationParameters.ReservationActionTimeout,
		averageBlockTime,
	)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()

	rat.revealCacheMutex.Lock()
	previousReveals := rat.revealCache[walletPublicKeyHash]
	rat.revealCacheMutex.Unlock()
	currentReveals := make(map[string]*tbtc.DepositRevealedEvent)
	defer func() {
		rat.revealCacheMutex.Lock()
		rat.revealCache[walletPublicKeyHash] = currentReveals
		rat.revealCacheMutex.Unlock()
	}()

	var pendingCandidates []*reservationAcceptanceFundingTxCandidate

	candidatesExamined := 0
	for _, requestEvent := range requestEvents {
		depositKey := requestEvent.ReservationKey

		// The event's own timeout lets a generation whose signing window
		// has closed be dropped without any RPC.
		if uint64(now.Unix())+
			uint64(reservationRequestTimeoutSafetyMarginSeconds) >=
			uint64(requestEvent.TimeoutAt) {
			continue
		}

		// Determine the acceptance authorization generation from the
		// reservation's own on-chain state, which authoritatively
		// reflects whether the requested generation is still the
		// current one and still pending.
		reservation, err := rat.chain.GetReservation(depositKey)
		if err != nil {
			// Fail safe: the production chain adapter never errors for
			// "not found" (it returns a zero record with State ==
			// Unknown), so a non-nil error here can only be an RPC/decode
			// failure. Skip this deposit for the current coordination
			// window instead; the next window retries.
			taskLogger.Errorf(
				"cannot get reservation [%v], skipping deposit for this window: [%v]",
				depositKey,
				err,
			)
			continue
		}

		// The pending-generation precondition: a successful read of a
		// generation whose record is not a Pending Acceptance action
		// (a timed-out or settled generation, or any non-acceptance
		// record) is not anchorable.
		action, err := rat.chain.GetReservationAction(
			depositKey,
			reservation.RequestNonce,
		)
		if err != nil {
			// Fail safe: a lookup error is indistinguishable from "still
			// pending" here. Skip this deposit for the current
			// coordination window instead; the next window retries.
			taskLogger.Errorf(
				"cannot get reservation action for [0x%x] nonce [%d]: [%v]",
				depositKey,
				reservation.RequestNonce,
				err,
			)
			continue
		}

		if action.ActionType != tbtc.ReservationActionTypeAcceptance ||
			action.State != tbtc.ReservationActionStatePending {
			taskLogger.Infof(
				"reservation [%v] generation [%d] is not a pending "+
					"acceptance action (type=%v, state=%v); skipping",
				depositKey,
				reservation.RequestNonce,
				action.ActionType,
				action.State,
			)
			continue
		}

		if action.TargetWalletPublicKeyHash != walletPublicKeyHash {
			taskLogger.Infof(
				"reservation [%v] pending acceptance targets another wallet "+
					"[%x]; skipping",
				depositKey,
				action.TargetWalletPublicKeyHash,
			)
			continue
		}

		// Mirror WalletProposalValidator.validateReservationAnchorProposal's
		// timeout safety margin gate: the validator only signs while
		// block.timestamp < action.TimeoutAt -
		// REQUEST_TIMEOUT_SAFETY_MARGIN, so a generation whose signing
		// window has closed (now + margin >= TimeoutAt, using the
		// addition form to avoid the underflow the contract guards
		// against) is skipped rather than proposed and rejected.
		if uint64(now.Unix())+
			uint64(reservationRequestTimeoutSafetyMarginSeconds) >=
			uint64(action.TimeoutAt) {
			taskLogger.Infof(
				"reservation [%v] pending acceptance generation [%d] "+
					"timeout [%d] is at or past the timeout safety "+
					"margin; skipping",
				depositKey,
				reservation.RequestNonce,
				action.TimeoutAt,
			)
			continue
		}

		if candidatesExamined >= maxReservationAcceptanceCandidatesPerRun {
			taskLogger.Warnf(
				"reached max reservation acceptance candidates per run "+
					"[%d]; remaining pending acceptances will be examined "+
					"on a subsequent run",
				maxReservationAcceptanceCandidatesPerRun,
			)
			break
		}
		candidatesExamined++

		revealEvent, err := rat.findReservationAcceptanceReveal(
			walletPublicKeyHash,
			depositKey,
			requestEvent.BlockNumber,
			action.TermSeconds,
			averageBlockTime,
			previousReveals,
		)
		if err != nil {
			taskLogger.Errorf(
				"cannot find deposit reveal for reservation [%v], skipping "+
					"deposit for this window: [%v]",
				depositKey,
				err,
			)
			continue
		}
		if revealEvent == nil {
			taskLogger.Warnf(
				"no deposit reveal found for reservation [%v] in the "+
					"blocks before its acceptance request; skipping",
				depositKey,
			)
			continue
		}
		currentReveals[depositKey.Text(16)] = revealEvent

		if !depositTargetsReservationVault(revealEvent.Vault, reservationVault) {
			taskLogger.Warnf(
				"reserved deposit [%v] was not revealed to the reservation "+
					"vault; skipping",
				depositKey,
			)
			continue
		}

		depositRequest, foundRequest, err := rat.chain.GetDepositRequest(
			revealEvent.FundingTxHash,
			revealEvent.FundingOutputIndex,
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

		pendingCandidates = append(
			pendingCandidates,
			&reservationAcceptanceFundingTxCandidate{
				event:          revealEvent,
				depositKey:     depositKey,
				depositRequest: depositRequest,
				requestNonce:   reservation.RequestNonce,
				action:         action,
			},
		)
	}

	return pendingCandidates, reservationParameters, nil
}

// publishReservationGauges publishes the occupancy and vault fee gauges.
// wallet_reservations_count / active_reservations_count /
// max_active_reservations are published on every Run regardless of the
// wallet state, mirroring the sibling live_wallets_count gauge that
// ReservationReanchorTask publishes every coordination window: a wallet
// mid-rotation (moving funds or closing) is exactly when gauge staleness
// matters most.
//
// reservation_vault_fee_debt_sat / reservation_vault_fee_reserve_tbtc_base_units
// publish the ReservationVault's outstanding in-kind fee debt (in
// satoshi) and fee-reserve balance (in TBTC base units, 1e18 per whole
// TBTC; a gauge value of N means N / 1e18 whole TBTC): a vault whose fee
// debt or reserve is growing is the leading indicator of reservation fee
// pressure. The vault address the caller already read is passed to the
// chain, so each gauge costs a single vault read.
//
// A failed occupancy read is returned as an error. The vault fee gauges
// are observability-only: a read error is logged, the gauge keeps its
// previously recorded value, and proposal generation goes on.
func (rat *ReservationAcceptanceTask) publishReservationGauges(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	reservationVault chain.Address,
) error {
	walletReservationsCount, err := rat.chain.WalletReservationsCount(
		walletPublicKeyHash,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get wallet reservations count: [%w]",
			err,
		)
	}

	activeReservationsCount, maxActiveReservations, err :=
		rat.chain.ActiveReservationsCount()
	if err != nil {
		return fmt.Errorf(
			"failed to get active reservations count: [%w]",
			err,
		)
	}

	if rat.metricsRecorder == nil {
		return nil
	}

	rat.metricsRecorder.SetGauge(
		"wallet_reservations_count",
		float64(walletReservationsCount),
	)
	rat.metricsRecorder.SetGauge(
		"active_reservations_count",
		float64(activeReservationsCount),
	)
	rat.metricsRecorder.SetGauge(
		"max_active_reservations",
		float64(maxActiveReservations),
	)

	feeDebtSat, err := rat.chain.ReservationVaultFeeDebtSat(reservationVault)
	if err != nil {
		taskLogger.Warnf(
			"failed to get reservation vault fee debt: [%v]",
			err,
		)
	} else {
		rat.metricsRecorder.SetGauge(
			"reservation_vault_fee_debt_sat",
			float64(feeDebtSat),
		)
	}

	feeReserve, err :=
		rat.chain.ReservationVaultFeeReserveTbtcBaseUnits(reservationVault)
	if err != nil {
		taskLogger.Warnf(
			"failed to get reservation vault fee reserve: [%v]",
			err,
		)
	} else if feeReserve != nil {
		reserveFloat, _ := feeReserve.Float64()
		rat.metricsRecorder.SetGauge(
			"reservation_vault_fee_reserve_tbtc_base_units",
			reserveFloat,
		)
	}

	return nil
}

// reservationAcceptanceRequestedEvents returns the wallet's
// ReservationAcceptanceRequested events that may still describe a
// signable generation, one per reservation (the highest request nonce),
// ordered by timeout, the generation closest to timing out first. The
// look-back window is the on-chain action timeout converted to blocks plus
// reservationAcceptanceRequestLookBackMarginBlocks.
func (rat *ReservationAcceptanceTask) reservationAcceptanceRequestedEvents(
	walletPublicKeyHash [20]byte,
	currentBlock uint64,
	actionTimeoutSeconds uint32,
	averageBlockTime time.Duration,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	lookBackBlocks := uint64(
		time.Duration(actionTimeoutSeconds)*time.Second/averageBlockTime,
	) + reservationAcceptanceRequestLookBackMarginBlocks

	startBlock := uint64(0)
	if currentBlock > lookBackBlocks {
		startBlock = currentBlock - lookBackBlocks
	}

	// The window is wider than many providers accept for one log query, so
	// it is scanned in chunks of the reveal lookup's size.
	var events []*tbtc.ReservationAcceptanceRequestedEvent
	for chunkStart := startBlock; chunkStart <= currentBlock; {
		chunkEnd := currentBlock
		if currentBlock-chunkStart >= tbtc.DepositRevealLookupChunkBlocks {
			chunkEnd = chunkStart + tbtc.DepositRevealLookupChunkBlocks - 1
		}

		chunkEvents, err := rat.chain.PastReservationAcceptanceRequestedEvents(
			&tbtc.ReservationAcceptanceRequestedEventFilter{
				StartBlock:          chunkStart,
				EndBlock:            &chunkEnd,
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get past reservation acceptance requested "+
					"events in blocks [%d, %d]: [%w]",
				chunkStart,
				chunkEnd,
				err,
			)
		}
		events = append(events, chunkEvents...)

		chunkStart = chunkEnd + 1
	}

	latest := make(map[string]*tbtc.ReservationAcceptanceRequestedEvent)
	for _, event := range events {
		if event.ReservationKey == nil ||
			event.WalletPublicKeyHash != walletPublicKeyHash {
			continue
		}
		key := event.ReservationKey.Text(16)
		if existing, ok := latest[key]; !ok ||
			event.RequestNonce > existing.RequestNonce {
			latest[key] = event
		}
	}

	result := make([]*tbtc.ReservationAcceptanceRequestedEvent, 0, len(latest))
	for _, event := range latest {
		result = append(result, event)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TimeoutAt != result[j].TimeoutAt {
			return result[i].TimeoutAt < result[j].TimeoutAt
		}
		if result[i].BlockNumber != result[j].BlockNumber {
			return result[i].BlockNumber < result[j].BlockNumber
		}
		return result[i].ReservationKey.Cmp(result[j].ReservationKey) < 0
	})

	return result, nil
}

// findReservationAcceptanceReveal returns the DepositRevealed event of the
// reserved deposit identified by depositKey, or nil when it is not found.
// A reveal found on the previous Run is reused. Otherwise the wallet's
// reveals are scanned backward from the acceptance request's block down
// to one snapshotted term plus reservationAcceptanceRevealLookBackMargin
// before it (see that constant for why the reveal cannot be older).
func (rat *ReservationAcceptanceTask) findReservationAcceptanceReveal(
	walletPublicKeyHash [20]byte,
	depositKey *big.Int,
	requestBlock uint64,
	termSeconds uint32,
	averageBlockTime time.Duration,
	previousReveals map[string]*tbtc.DepositRevealedEvent,
) (*tbtc.DepositRevealedEvent, error) {
	if cached, ok := previousReveals[depositKey.Text(16)]; ok {
		return cached, nil
	}

	lookBackBlocks := uint64(
		(time.Duration(termSeconds)*time.Second +
			reservationAcceptanceRevealLookBackMargin) / averageBlockTime,
	)
	fromBlock := uint64(0)
	if requestBlock > lookBackBlocks {
		fromBlock = requestBlock - lookBackBlocks
	}

	return tbtc.FindDepositRevealedEvent(
		rat.chain,
		walletPublicKeyHash,
		fromBlock,
		requestBlock,
		func(event *tbtc.DepositRevealedEvent) bool {
			return rat.chain.BuildDepositKey(
				event.FundingTxHash,
				event.FundingOutputIndex,
			).Cmp(depositKey) == 0
		},
	)
}

// completeReservationAcceptanceCandidate applies the Bitcoin-side and fee
// checks to a candidate from findReservationAcceptanceCandidates and
// returns the complete candidate, or nil (after logging why) when it does
// not qualify. feeEstimates caches the anchor fee estimate per snapshotted
// fee cap for the duration of one Run.
func (rat *ReservationAcceptanceTask) completeReservationAcceptanceCandidate(
	taskLogger log.StandardLogger,
	pendingCandidate *reservationAcceptanceFundingTxCandidate,
	lookup reservationAcceptanceFundingTxLookup,
	feeEstimates map[uint64]reservationAcceptanceFeeEstimate,
	reservationParameters *tbtc.ReservationParameters,
) *reservationAcceptanceCandidate {
	event := pendingCandidate.event
	depositKey := pendingCandidate.depositKey
	depositRequest := pendingCandidate.depositRequest
	action := pendingCandidate.action

	if lookup.fundingTxErr != nil {
		taskLogger.Errorf(
			"failed to get funding tx for reserved deposit [%v]: [%v]",
			depositKey,
			lookup.fundingTxErr,
		)
		return nil
	}

	if lookup.confirmationsErr != nil {
		taskLogger.Errorf(
			"failed to get funding tx confirmations for [%v]: [%v]",
			depositKey,
			lookup.confirmationsErr,
		)
		return nil
	}
	if lookup.confirmations < tbtc.DepositSweepRequiredFundingTxConfirmations {
		taskLogger.Debugf(
			"reserved deposit [%v] funding tx confirmations [%d/%d] below required",
			depositKey,
			lookup.confirmations,
			tbtc.DepositSweepRequiredFundingTxConfirmations,
		)
		return nil
	}

	// The anchor transaction has a fixed size, so its fee only depends on
	// the fee-rate oracle and the generation's snapshotted fee cap. The
	// estimate (or its error) is cached per cap, so candidates sharing a
	// cap do not repeat it. A fee error -- including an estimate above
	// this generation's cap -- skips the candidate: another generation
	// with a higher cap may still be viable. The estimator does not
	// separate oracle failures from cap overruns, so both are skipped.
	txMaxFee := action.TxMaxFee
	feeEstimate, ok := feeEstimates[txMaxFee]
	if !ok {
		feeEstimate.fee, feeEstimate.err = estimateReservationAcceptanceFee(
			rat.btcChain,
			txMaxFee,
		)
		feeEstimates[txMaxFee] = feeEstimate
	}
	if feeEstimate.err != nil {
		taskLogger.Warnf(
			"cannot estimate reservation acceptance transaction fee for "+
				"reserved deposit [%v] (max fee [%d]); skipping: [%v]",
			depositKey,
			txMaxFee,
			feeEstimate.err,
		)
		return nil
	}
	anchorFee := feeEstimate.fee

	// Mirror WalletProposalValidator.validateReservationAnchorProposal's
	// minimum check against this generation's own snapshotted values:
	// the deposit amount must satisfy
	// amount >= action.MinAmount + anchorFee before the on-chain
	// validation can pass. The addition-based formulation avoids the
	// underflow the contract guards against when the estimated fee
	// exceeds a small deposit's amount. The live ReservationMinAmount
	// is deliberately not used: the snapshot is what the validator
	// enforces for this generation.
	feeSats := uint64(anchorFee)
	minPlusFee := action.MinAmount + feeSats
	if minPlusFee < action.MinAmount || depositRequest.Amount < minPlusFee {
		taskLogger.Infof(
			"reserved deposit [%v] amount [%d] below the generation's "+
				"snapshotted minimum [%d] plus anchor fee [%d]; skipping",
			depositKey,
			depositRequest.Amount,
			action.MinAmount,
			anchorFee,
		)
		return nil
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
		FundingTx:             lookup.fundingTx,
		ReservationParameters: reservationParameters,
		TxMaxFee:              txMaxFee,
		RequestNonce:          pendingCandidate.requestNonce,
		AnchorFee:             anchorFee,
	}
}

// reservationAcceptanceFundingTxPipeline is a lazily-scheduled, bounded,
// in-order funding-transaction lookup pipeline returned by
// fetchReservationAcceptanceFundingTxs. A naive worker pool launches every
// candidate's GetTransaction/GetTransactionConfirmations call
// unconditionally, so a caller that only needs the first fully eligible
// candidate still pays for fetching every other pending candidate's
// funding transaction. This pipeline instead hands results back one index
// at a time via next(), and stop() tells it to abandon every lookup it has
// not yet dispatched, so a caller that stops asking for more results after
// an early match never causes those later fetches to happen at all.
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
// increasing order (0, 1, 2, ...) -- the same order candidates was given
// to fetchReservationAcceptanceFundingTxs in. Because each candidate's
// result is delivered on its own per-index channel, a later candidate's
// fetch racing ahead and completing first can never be mistaken for an
// earlier candidate's result.
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
// candidates, in order, and returns a
// reservationAcceptanceFundingTxPipeline the caller pulls results from one
// index at a time via next(). At most
// reservationAcceptanceFundingTxLookupWorkers lookups ever run
// concurrently. This is a bound on concurrency, not on the total number
// of Bitcoin RPCs issued over the pipeline's lifetime: fetches are
// dispatched lazily and the dispatcher only stops handing out new
// candidates once the caller calls the returned pipeline's stop() (Run
// does so via defer). Because the dispatcher waits on stop(), not on the
// caller actually consuming each result via next(), a caller whose own
// per-candidate work (e.g. proposal validation) is slower than the
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

	// The dispatcher below walks candidates in their given order,
	// acquiring a dispatchSlots slot before starting each one's fetch, so
	// at most `workers` fetches ever run concurrently. It gives stop()
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

// proposeReservationAcceptance assembles and validates the anchor proposal
// for an already selected candidate. The pending acceptance generation the
// proposal consumes was created on-chain by the deposit's own depositor
// (the only caller permitted to request acceptance), so this builder
// performs no chain-state-mutating call and any error leaves nothing
// behind; Run skips the candidate on error.
func (rat *ReservationAcceptanceTask) proposeReservationAcceptance(
	taskLogger log.StandardLogger,
	walletPublicKeyHash [20]byte,
	candidate *reservationAcceptanceCandidate,
) (*tbtc.ReservationAnchorProposal, error) {
	if candidate == nil || candidate.Deposit == nil {
		return nil, fmt.Errorf("candidate is required")
	}

	taskLogger.Infof("preparing a reservation acceptance proposal")

	// The anchor fee and its net-of-fee viability were already computed
	// and validated during candidate selection, so a doomed candidate is
	// skipped there in favor of the next one.
	anchorFee := candidate.AnchorFee

	taskLogger.Infof("anchor transaction fee: [%d]", anchorFee)

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
		return nil, fmt.Errorf(
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
		return nil, fmt.Errorf(
			"failed to verify reservation anchor proposal: %v",
			err,
		)
	}

	return proposal, nil
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

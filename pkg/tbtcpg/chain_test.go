package tbtcpg

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationDepositRefundSafetyMarginSeconds mirrors
// WalletProposalValidatorConstants.DEPOSIT_REFUND_SAFETY_MARGIN
// (24 hours): the on-chain acceptance validator refuses to sign an
// anchor whose refund becomes available less than a day from now, so a
// wallet signing later cannot race the depositor's refund.
const reservationDepositRefundSafetyMarginSeconds = 24 * 60 * 60

type movingFundsCommitmentSubmission struct {
	WalletPublicKeyHash [20]byte
	WalletMainUtxo      *bitcoin.UnspentTransactionOutput
	WalletMembersIDs    []uint32
	WalletMemberIndex   uint32
	TargetWallets       [][20]byte
}

// reservationReanchorRequestSubmission captures a submitted reservation
// re-anchor request that tests can inspect for assertion.
type reservationReanchorRequestSubmission struct {
	ReservationKey            *big.Int
	TargetWalletPublicKeyHash [20]byte
}

// reservationCapacity is a wallet's reservation count and amount, as in
// Reservation.sol's walletReservationInfo.
type reservationCapacity struct {
	count  uint32
	amount uint64
}

// belowDustNotification captures a submitted NotifyMovingFundsBelowDust
// call that tests can inspect for assertion.
type belowDustNotification struct {
	WalletPublicKeyHash [20]byte
	MainUtxo            *bitcoin.UnspentTransactionOutput
}

type LocalChain struct {
	mutex sync.Mutex

	depositRequests                          map[[32]byte]*tbtc.DepositChainRequest
	pastDepositRevealedEvents                map[[32]byte][]*tbtc.DepositRevealedEvent
	pastNewWalletRegisteredEvents            map[[32]byte][]*tbtc.NewWalletRegisteredEvent
	depositParameters                        tbtc.DepositParameters
	depositSweepProposalValidations          map[[32]byte]bool
	redemptionParameters                     tbtc.RedemptionParameters
	redemptionRequestMinAge                  uint32
	walletParameters                         tbtc.WalletParameters
	walletChainData                          map[[20]byte]*tbtc.WalletChainData
	blockCounter                             chain.BlockCounter
	pastRedemptionRequestedEvents            map[[32]byte][]*tbtc.RedemptionRequestedEvent
	averageBlockTime                         time.Duration
	pendingRedemptionRequests                map[[32]byte]*tbtc.RedemptionRequest
	redemptionProposalValidations            map[[32]byte]bool
	heartbeatProposalValidations             map[[16]byte]bool
	movingFundsParameters                    tbtc.MovingFundsParameters
	pastMovingFundsCommitmentSubmittedEvents map[[32]byte][]*tbtc.MovingFundsCommitmentSubmittedEvent
	movingFundsProposalValidations           map[[32]byte]bool
	movingFundsCommitmentSubmissions         []*movingFundsCommitmentSubmission
	pastMovingFundsCompletedEvents           map[[32]byte][]*tbtc.MovingFundsCompletedEvent
	movedFundsSweepRequests                  map[[32]byte]*tbtc.MovedFundsSweepRequest
	movedFundsSweepProposalValidations       map[[32]byte]bool
	operatorIDs                              map[chain.Address]uint32
	redemptionDelays                         map[[32]byte]time.Duration
	depositMinAge                            uint32
	depositSweepMaxSizeErr                   error

	reservations       map[string]*tbtc.Reservation
	reservationActions map[string]*tbtc.ReservationAction
	// reservation vault fee observability (m1 fee debt / fee reserve
	// gauges); zero values mean the vault reports no debt/reserve.
	reservationVaultFeeDebtSat            uint64
	reservationVaultFeeDebtSatErr         error
	reservationVaultFeeReserveBalance     *big.Int
	reservationVaultFeeReserveErr         error
	reservationParametersValue            tbtc.ReservationParameters
	reservationParametersSet              bool
	reservationProposalValidations        map[[32]byte]bool
	reservationReanchorRequestSubmissions []*reservationReanchorRequestSubmission
	// reservationReanchorRequestAttempts records every
	// RequestReservationReanchor call, including reverted ones.
	reservationReanchorRequestAttempts []*reservationReanchorRequestSubmission
	// reanchorReservedCapacity is the count and amount capacity pending
	// re-anchor requests reserved on their target wallets. Together with
	// the wallet's custodied reservations it forms the wallet's
	// walletReservationInfo ledger entry (see walletReservationInfoLocked).
	reanchorReservedCapacity map[[20]byte]reservationCapacity
	// pendingNextReanchorRequest makes the next RequestReservationReanchor
	// submission succeed without writing a generation, modeling a
	// submission still unconfirmed in the mempool (or dropped from it).
	pendingNextReanchorRequest bool
	// reservationReanchorValidationCalls counts calls to
	// ValidateReservationReanchorProposal, so tests can observe whether
	// production actually reached on-chain validation for a candidate
	// proposal (e.g. to prove the resume-path timeout safety margin
	// gate short-circuited before validation would have run).
	reservationReanchorValidationCalls int
	// reservationCaps, when set, overrides the zero cap-pair returned
	// by ReservationCaps (used to model a chain configured with
	// specific capacity limits).
	reservationCaps    [2]uint64
	reservationCapsSet bool

	belowDustNotifications []*belowDustNotification
	reservationWalletKeys  map[[20]byte][]*big.Int
	reservedDeposits       map[string]bool
	liveWalletsCountValue  uint32
	liveWalletsCountSet    bool
}

func NewLocalChain() *LocalChain {
	return &LocalChain{
		depositRequests:                          make(map[[32]byte]*tbtc.DepositChainRequest),
		pastDepositRevealedEvents:                make(map[[32]byte][]*tbtc.DepositRevealedEvent),
		pastNewWalletRegisteredEvents:            make(map[[32]byte][]*tbtc.NewWalletRegisteredEvent),
		depositSweepProposalValidations:          make(map[[32]byte]bool),
		pastRedemptionRequestedEvents:            make(map[[32]byte][]*tbtc.RedemptionRequestedEvent),
		walletChainData:                          make(map[[20]byte]*tbtc.WalletChainData),
		pendingRedemptionRequests:                make(map[[32]byte]*tbtc.RedemptionRequest),
		redemptionProposalValidations:            make(map[[32]byte]bool),
		heartbeatProposalValidations:             make(map[[16]byte]bool),
		pastMovingFundsCommitmentSubmittedEvents: make(map[[32]byte][]*tbtc.MovingFundsCommitmentSubmittedEvent),
		movingFundsProposalValidations:           make(map[[32]byte]bool),
		movingFundsCommitmentSubmissions:         make([]*movingFundsCommitmentSubmission, 0),
		pastMovingFundsCompletedEvents:           make(map[[32]byte][]*tbtc.MovingFundsCompletedEvent),
		movedFundsSweepRequests:                  make(map[[32]byte]*tbtc.MovedFundsSweepRequest),
		movedFundsSweepProposalValidations:       make(map[[32]byte]bool),
		operatorIDs:                              make(map[chain.Address]uint32),
		redemptionDelays:                         make(map[[32]byte]time.Duration),

		reservations:                          make(map[string]*tbtc.Reservation),
		reservationActions:                    make(map[string]*tbtc.ReservationAction),
		reservationProposalValidations:        make(map[[32]byte]bool),
		reservationReanchorRequestSubmissions: make([]*reservationReanchorRequestSubmission, 0),
		reanchorReservedCapacity:              make(map[[20]byte]reservationCapacity),
		belowDustNotifications:                make([]*belowDustNotification, 0),
		reservationWalletKeys:                 make(map[[20]byte][]*big.Int),
		reservedDeposits:                      make(map[string]bool),
	}
}

func (lc *LocalChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastDepositRevealedEventsKey(filter)
	if err != nil {
		return nil, err
	}

	events, ok := lc.pastDepositRevealedEvents[eventsKey]
	if !ok {
		return nil, fmt.Errorf("no events for given filter")
	}

	return events, nil
}

func (lc *LocalChain) AddPastDepositRevealedEvent(
	filter *tbtc.DepositRevealedEventFilter,
	event *tbtc.DepositRevealedEvent,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastDepositRevealedEventsKey(filter)
	if err != nil {
		return err
	}

	lc.pastDepositRevealedEvents[eventsKey] = append(
		lc.pastDepositRevealedEvents[eventsKey],
		event,
	)

	return nil
}

func buildPastDepositRevealedEventsKey(
	filter *tbtc.DepositRevealedEventFilter,
) ([32]byte, error) {
	if filter == nil {
		return [32]byte{}, nil
	}

	var buffer bytes.Buffer

	startBlock := make([]byte, 8)
	binary.BigEndian.PutUint64(startBlock, filter.StartBlock)
	buffer.Write(startBlock)

	if filter.EndBlock != nil {
		endBlock := make([]byte, 8)
		binary.BigEndian.PutUint64(endBlock, *filter.EndBlock)
		buffer.Write(endBlock)
	}

	for _, depositor := range filter.Depositor {
		depositorBytes, err := hex.DecodeString(depositor.String())
		if err != nil {
			return [32]byte{}, err
		}

		buffer.Write(depositorBytes)
	}

	for _, walletPublicKeyHash := range filter.WalletPublicKeyHash {
		buffer.Write(walletPublicKeyHash[:])
	}

	return sha256.Sum256(buffer.Bytes()), nil
}

func (lc *LocalChain) GetDepositRequest(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) (*tbtc.DepositChainRequest, bool, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	requestKey := buildDepositRequestKey(fundingTxHash, fundingOutputIndex)

	request, ok := lc.depositRequests[requestKey]
	if !ok {
		return nil, false, nil
	}

	return request, true, nil
}

func (lc *LocalChain) SetDepositRequest(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	request *tbtc.DepositChainRequest,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	requestKey := buildDepositRequestKey(fundingTxHash, fundingOutputIndex)

	lc.depositRequests[requestKey] = request
}

func (lc *LocalChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastNewWalletRegisteredEventsKey(filter)
	if err != nil {
		return nil, err
	}

	events, ok := lc.pastNewWalletRegisteredEvents[eventsKey]
	if !ok {
		return nil, fmt.Errorf("no events for given filter")
	}

	return events, nil
}

func (lc *LocalChain) AddPastNewWalletRegisteredEvent(
	filter *tbtc.NewWalletRegisteredEventFilter,
	event *tbtc.NewWalletRegisteredEvent,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastNewWalletRegisteredEventsKey(filter)
	if err != nil {
		return err
	}

	if _, ok := lc.pastNewWalletRegisteredEvents[eventsKey]; !ok {
		lc.pastNewWalletRegisteredEvents[eventsKey] = []*tbtc.NewWalletRegisteredEvent{}
	}

	lc.pastNewWalletRegisteredEvents[eventsKey] = append(
		lc.pastNewWalletRegisteredEvents[eventsKey],
		event,
	)

	return nil
}

func buildPastNewWalletRegisteredEventsKey(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([32]byte, error) {
	if filter == nil {
		return [32]byte{}, nil
	}

	var buffer bytes.Buffer

	startBlock := make([]byte, 8)
	binary.BigEndian.PutUint64(startBlock, filter.StartBlock)
	buffer.Write(startBlock)

	if filter.EndBlock != nil {
		endBlock := make([]byte, 8)
		binary.BigEndian.PutUint64(endBlock, *filter.EndBlock)
		buffer.Write(endBlock)
	}

	for _, ecdsaWalletID := range filter.EcdsaWalletID {
		buffer.Write(ecdsaWalletID[:])
	}

	for _, walletPublicKeyHash := range filter.WalletPublicKeyHash {
		buffer.Write(walletPublicKeyHash[:])
	}

	return sha256.Sum256(buffer.Bytes()), nil
}

func (lc *LocalChain) PastRedemptionRequestedEvents(
	filter *tbtc.RedemptionRequestedEventFilter,
) ([]*tbtc.RedemptionRequestedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastRedemptionRequestedEventsKey(filter)
	if err != nil {
		return nil, err
	}

	events, ok := lc.pastRedemptionRequestedEvents[eventsKey]
	if !ok {
		return nil, fmt.Errorf("no events for given filter")
	}

	return events, nil
}

func (lc *LocalChain) AddPastRedemptionRequestedEvent(
	filter *tbtc.RedemptionRequestedEventFilter,
	event *tbtc.RedemptionRequestedEvent,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastRedemptionRequestedEventsKey(filter)
	if err != nil {
		return err
	}

	lc.pastRedemptionRequestedEvents[eventsKey] = append(
		lc.pastRedemptionRequestedEvents[eventsKey],
		event,
	)

	return nil
}

func buildPastRedemptionRequestedEventsKey(
	filter *tbtc.RedemptionRequestedEventFilter,
) ([32]byte, error) {
	if filter == nil {
		return [32]byte{}, nil
	}

	var buffer bytes.Buffer

	startBlock := make([]byte, 8)
	binary.BigEndian.PutUint64(startBlock, filter.StartBlock)
	buffer.Write(startBlock)

	if filter.EndBlock != nil {
		endBlock := make([]byte, 8)
		binary.BigEndian.PutUint64(endBlock, *filter.EndBlock)
		buffer.Write(endBlock)
	}

	for _, walletPublicKeyHash := range filter.WalletPublicKeyHash {
		buffer.Write(walletPublicKeyHash[:])
	}

	for _, redeemer := range filter.Redeemer {
		redeemerHex, err := hex.DecodeString(redeemer.String())
		if err != nil {
			return [32]byte{}, err
		}

		buffer.Write(redeemerHex)
	}

	return sha256.Sum256(buffer.Bytes()), nil
}

func buildPastMovingFundsCommitmentSubmittedEventsKey(
	filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
) ([32]byte, error) {
	if filter == nil {
		return [32]byte{}, nil
	}

	var buffer bytes.Buffer

	startBlock := make([]byte, 8)
	binary.BigEndian.PutUint64(startBlock, filter.StartBlock)
	buffer.Write(startBlock)

	if filter.EndBlock != nil {
		endBlock := make([]byte, 8)
		binary.BigEndian.PutUint64(endBlock, *filter.EndBlock)
		buffer.Write(endBlock)
	}

	for _, walletPublicKeyHash := range filter.WalletPublicKeyHash {
		buffer.Write(walletPublicKeyHash[:])
	}

	return sha256.Sum256(buffer.Bytes()), nil
}

func buildPastMovingFundsCompletedEventsKey(
	filter *tbtc.MovingFundsCompletedEventFilter,
) ([32]byte, error) {
	if filter == nil {
		return [32]byte{}, nil
	}

	var buffer bytes.Buffer

	startBlock := make([]byte, 8)
	binary.BigEndian.PutUint64(startBlock, filter.StartBlock)
	buffer.Write(startBlock)

	if filter.EndBlock != nil {
		endBlock := make([]byte, 8)
		binary.BigEndian.PutUint64(endBlock, *filter.EndBlock)
		buffer.Write(endBlock)
	}

	// The wallet public key hashes are sometimes in the undefined order as
	// they are read from a map. Convert them to strings and sort them so that
	// their order is defined.
	walletPublicKeyHashesStr := make([]string, len(filter.WalletPublicKeyHash))
	for i, hash := range filter.WalletPublicKeyHash {
		walletPublicKeyHashesStr[i] = hex.EncodeToString(hash[:])
	}
	sort.Strings(walletPublicKeyHashesStr)
	for _, hashStr := range walletPublicKeyHashesStr {
		hashBytes, err := hex.DecodeString(hashStr)
		if err != nil {
			return [32]byte{}, err
		}
		buffer.Write(hashBytes)
	}

	return sha256.Sum256(buffer.Bytes()), nil
}

func (lc *LocalChain) BuildDepositKey(fundingTxHash bitcoin.Hash, fundingOutputIndex uint32) *big.Int {
	depositKeyBytes := buildDepositRequestKey(fundingTxHash, fundingOutputIndex)

	return new(big.Int).SetBytes(depositKeyBytes[:])
}

func buildDepositRequestKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) [32]byte {
	fundingOutputIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fundingOutputIndexBytes, fundingOutputIndex)

	return sha256.Sum256(append(fundingTxHash[:], fundingOutputIndexBytes...))
}

func (lc *LocalChain) BuildRedemptionKey(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*big.Int, error) {
	redemptionKeyBytes := buildRedemptionRequestKey(
		walletPublicKeyHash,
		redeemerOutputScript,
	)

	return new(big.Int).SetBytes(redemptionKeyBytes[:]), nil
}

func buildRedemptionRequestKey(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) [32]byte {
	return sha256.Sum256(append(walletPublicKeyHash[:], redeemerOutputScript...))
}

func (lc *LocalChain) GetDepositParameters() (tbtc.DepositParameters, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.depositParameters, nil
}

func (lc *LocalChain) GetPendingRedemptionRequest(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*tbtc.RedemptionRequest, bool, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	requestKey := buildRedemptionRequestKey(walletPublicKeyHash, redeemerOutputScript)

	request, ok := lc.pendingRedemptionRequests[requestKey]
	if !ok {
		return nil, false, nil
	}

	return request, true, nil
}

func (lc *LocalChain) SetPendingRedemptionRequest(
	walletPublicKeyHash [20]byte,
	request *tbtc.RedemptionRequest,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	requestKey := buildRedemptionRequestKey(
		walletPublicKeyHash,
		request.RedeemerOutputScript,
	)

	lc.pendingRedemptionRequests[requestKey] = request
}

func (lc *LocalChain) SetDepositParameters(
	dustThreshold uint64,
	treasuryFeeDivisor uint64,
	txMaxFee uint64,
	revealAheadPeriod uint32,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.depositParameters = tbtc.DepositParameters{
		DustThreshold:      dustThreshold,
		TreasuryFeeDivisor: treasuryFeeDivisor,
		TxMaxFee:           txMaxFee,
		RevealAheadPeriod:  revealAheadPeriod,
	}
}

func (lc *LocalChain) GetRedemptionParameters() (tbtc.RedemptionParameters, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.redemptionParameters, nil
}

func (lc *LocalChain) SetRedemptionParameters(
	dustThreshold uint64,
	treasuryFeeDivisor uint64,
	txMaxFee uint64,
	txMaxTotalFee uint64,
	timeout uint32,
	timeoutSlashingAmount *big.Int,
	timeoutNotifierRewardMultiplier uint32,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.redemptionParameters = tbtc.RedemptionParameters{
		DustThreshold:                   dustThreshold,
		TreasuryFeeDivisor:              treasuryFeeDivisor,
		TxMaxFee:                        txMaxFee,
		TxMaxTotalFee:                   txMaxTotalFee,
		Timeout:                         timeout,
		TimeoutSlashingAmount:           timeoutSlashingAmount,
		TimeoutNotifierRewardMultiplier: timeoutNotifierRewardMultiplier,
	}
}

func (lc *LocalChain) ValidateDepositSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
	depositsExtraInfo []struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildDepositSweepProposalValidationKey(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return err
	}

	result, ok := lc.depositSweepProposalValidations[key]
	if !ok {
		return fmt.Errorf("validation result unknown")
	}

	if !result {
		return fmt.Errorf("validation failed")
	}

	return nil
}

func (lc *LocalChain) SetDepositSweepProposalValidationResult(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
	depositsExtraInfo []struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
	result bool,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildDepositSweepProposalValidationKey(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return err
	}

	lc.depositSweepProposalValidations[key] = result

	return nil
}

func buildDepositSweepProposalValidationKey(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
) ([32]byte, error) {
	var buffer bytes.Buffer

	buffer.Write(walletPublicKeyHash[:])

	for _, deposit := range proposal.DepositsKeys {
		buffer.Write(deposit.FundingTxHash[:])

		fundingOutputIndex := make([]byte, 4)
		binary.BigEndian.PutUint32(fundingOutputIndex, deposit.FundingOutputIndex)
		buffer.Write(fundingOutputIndex)
	}

	buffer.Write(proposal.SweepTxFee.Bytes())

	return sha256.Sum256(buffer.Bytes()), nil
}

func (lc *LocalChain) ValidateRedemptionProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.RedemptionProposal,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildRedemptionProposalValidationKey(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return err
	}

	result, ok := lc.redemptionProposalValidations[key]
	if !ok {
		return fmt.Errorf("validation result unknown")
	}

	if !result {
		return fmt.Errorf("validation failed")
	}

	return nil
}

func (lc *LocalChain) SetRedemptionProposalValidationResult(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.RedemptionProposal,
	result bool,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildRedemptionProposalValidationKey(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return err
	}

	lc.redemptionProposalValidations[key] = result

	return nil
}

func (lc *LocalChain) ValidateHeartbeatProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.HeartbeatProposal,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	result, ok := lc.heartbeatProposalValidations[proposal.Message]
	if !ok {
		return fmt.Errorf("validation result unknown")
	}

	if !result {
		return fmt.Errorf("validation failed")
	}

	return nil
}

func (lc *LocalChain) SetHeartbeatProposalValidationResult(
	proposal *tbtc.HeartbeatProposal,
	result bool,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.heartbeatProposalValidations[proposal.Message] = result
}

func (lc *LocalChain) GetMovingFundsParameters() (tbtc.MovingFundsParameters, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.movingFundsParameters, nil
}

func (lc *LocalChain) GetMovedFundsSweepRequest(
	movingFundsTxHash bitcoin.Hash,
	movingFundsTxOutpointIndex uint32,
) (*tbtc.MovedFundsSweepRequest, bool, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	requestKey := buildMovedFundsSweepRequestKey(
		movingFundsTxHash,
		movingFundsTxOutpointIndex,
	)

	request, ok := lc.movedFundsSweepRequests[requestKey]
	if !ok {
		return nil, false, nil
	}

	return request, true, nil
}

func (lc *LocalChain) SetMovedFundsSweepRequest(
	movingFundsTxHash bitcoin.Hash,
	movingFundsTxOutpointIndex uint32,
	request *tbtc.MovedFundsSweepRequest,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	requestKey := buildMovedFundsSweepRequestKey(
		movingFundsTxHash,
		movingFundsTxOutpointIndex,
	)

	lc.movedFundsSweepRequests[requestKey] = request
}

func buildMovedFundsSweepRequestKey(
	movingFundsTxHash bitcoin.Hash,
	movingFundsTxOutpointIndex uint32,
) [32]byte {
	var buffer bytes.Buffer

	buffer.Write(movingFundsTxHash[:])

	outputIndex := make([]byte, 4)
	binary.BigEndian.PutUint32(outputIndex, movingFundsTxOutpointIndex)
	buffer.Write(outputIndex)

	return sha256.Sum256(buffer.Bytes())
}

func (lc *LocalChain) SetMovingFundsParameters(
	txMaxTotalFee uint64,
	dustThreshold uint64,
	timeoutResetDelay uint32,
	timeout uint32,
	timeoutSlashingAmount *big.Int,
	timeoutNotifierRewardMultiplier uint32,
	commitmentGasOffset uint16,
	sweepTxMaxTotalFee uint64,
	sweepTimeout uint32,
	sweepTimeoutSlashingAmount *big.Int,
	sweepTimeoutNotifierRewardMultiplier uint32,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.movingFundsParameters = tbtc.MovingFundsParameters{
		TxMaxTotalFee:                        txMaxTotalFee,
		DustThreshold:                        dustThreshold,
		TimeoutResetDelay:                    timeoutResetDelay,
		Timeout:                              timeout,
		TimeoutSlashingAmount:                timeoutSlashingAmount,
		TimeoutNotifierRewardMultiplier:      timeoutNotifierRewardMultiplier,
		CommitmentGasOffset:                  commitmentGasOffset,
		SweepTxMaxTotalFee:                   sweepTxMaxTotalFee,
		SweepTimeout:                         sweepTimeout,
		SweepTimeoutSlashingAmount:           sweepTimeoutSlashingAmount,
		SweepTimeoutNotifierRewardMultiplier: sweepTimeoutNotifierRewardMultiplier,
	}
}

func buildMovingFundsProposalValidationKey(
	walletPublicKeyHash [20]byte,
	mainUTXO *bitcoin.UnspentTransactionOutput,
	proposal *tbtc.MovingFundsProposal,
) ([32]byte, error) {
	var buffer bytes.Buffer

	buffer.Write(walletPublicKeyHash[:])

	buffer.Write(mainUTXO.Outpoint.TransactionHash[:])
	binary.Write(&buffer, binary.BigEndian, mainUTXO.Outpoint.OutputIndex)
	binary.Write(&buffer, binary.BigEndian, mainUTXO.Value)

	for _, wallet := range proposal.TargetWallets {
		buffer.Write(wallet[:])
	}

	buffer.Write(proposal.MovingFundsTxFee.Bytes())

	return sha256.Sum256(buffer.Bytes()), nil
}

func (lc *LocalChain) ValidateMovingFundsProposal(
	walletPublicKeyHash [20]byte,
	mainUTXO *bitcoin.UnspentTransactionOutput,
	proposal *tbtc.MovingFundsProposal,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildMovingFundsProposalValidationKey(
		walletPublicKeyHash,
		mainUTXO,
		proposal,
	)
	if err != nil {
		return err
	}

	result, ok := lc.movingFundsProposalValidations[key]
	if !ok {
		return fmt.Errorf("validation result unknown")
	}

	if !result {
		return fmt.Errorf("validation failed")
	}

	return nil
}

func (lc *LocalChain) SetMovingFundsProposalValidationResult(
	walletPublicKeyHash [20]byte,
	mainUTXO *bitcoin.UnspentTransactionOutput,
	proposal *tbtc.MovingFundsProposal,
	result bool,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildMovingFundsProposalValidationKey(
		walletPublicKeyHash,
		mainUTXO,
		proposal,
	)
	if err != nil {
		return err
	}

	lc.movingFundsProposalValidations[key] = result

	return nil
}

func buildRedemptionProposalValidationKey(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.RedemptionProposal,
) ([32]byte, error) {
	var buffer bytes.Buffer

	buffer.Write(walletPublicKeyHash[:])

	for _, script := range proposal.RedeemersOutputScripts {
		buffer.Write(script)
	}

	buffer.Write(proposal.RedemptionTxFee.Bytes())

	return sha256.Sum256(buffer.Bytes()), nil
}

func (lc *LocalChain) GetRedemptionMaxSize() (uint16, error) {
	panic("unsupported")
}

func (lc *LocalChain) GetRedemptionRequestMinAge() (uint32, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.redemptionRequestMinAge, nil
}

func (lc *LocalChain) SetRedemptionRequestMinAge(redemptionRequestMinAge uint32) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.redemptionRequestMinAge = redemptionRequestMinAge
}

func (lc *LocalChain) GetDepositSweepMaxSize() (uint16, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.depositSweepMaxSizeErr != nil {
		return 0, lc.depositSweepMaxSizeErr
	}

	panic("unsupported")
}

// SetDepositSweepMaxSizeError configures the error GetDepositSweepMaxSize
// returns, allowing tests to exercise the max-size-lookup failure path
// without a real chain implementation.
func (lc *LocalChain) SetDepositSweepMaxSizeError(err error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.depositSweepMaxSizeErr = err
}

func (lc *LocalChain) BlockCounter() (chain.BlockCounter, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.blockCounter, nil
}

func (lc *LocalChain) SetBlockCounter(blockCounter chain.BlockCounter) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.blockCounter = blockCounter
}

func (lc *LocalChain) AverageBlockTime() time.Duration {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.averageBlockTime
}

func (lc *LocalChain) SetOperatorID(
	operatorAddress chain.Address,
	operatorID chain.OperatorID,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	_, ok := lc.operatorIDs[operatorAddress]
	if !ok {
		lc.operatorIDs[operatorAddress] = operatorID
	}

	return nil
}

func (lc *LocalChain) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	operatorID, ok := lc.operatorIDs[operatorAddress]
	if !ok {
		return 0, fmt.Errorf("operator not found")
	}

	return operatorID, nil
}

func (lc *LocalChain) SetAverageBlockTime(averageBlockTime time.Duration) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.averageBlockTime = averageBlockTime
}

func (lc *LocalChain) IsWalletRegistered(EcdsaWalletID [32]byte) (bool, error) {
	panic("unsupported")
}

func (lc *LocalChain) CalculateWalletID(
	walletPublicKey *ecdsa.PublicKey,
) ([32]byte, error) {
	panic("unsupported")
}

func (lc *LocalChain) GetWallet(walletPublicKeyHash [20]byte) (
	*tbtc.WalletChainData,
	error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	data, ok := lc.walletChainData[walletPublicKeyHash]
	if !ok {
		fmt.Println("Not found")
		return nil, fmt.Errorf("wallet chain data not found")
	}

	return data, nil
}

func (lc *LocalChain) SetWallet(
	walletPublicKeyHash [20]byte,
	data *tbtc.WalletChainData,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.walletChainData[walletPublicKeyHash] = data
}

func (lc *LocalChain) OnWalletClosed(
	handler func(event *tbtc.WalletClosedEvent),
) subscription.EventSubscription {
	panic("unsupported")
}

func (lc *LocalChain) GetWalletParameters() (tbtc.WalletParameters, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.walletParameters, nil
}

func (lc *LocalChain) SetWalletParameters(
	creationPeriod uint32,
	creationMinBtcBalance uint64,
	creationMaxBtcBalance uint64,
	closureMinBtcBalance uint64,
	maxAge uint32,
	maxBtcTransfer uint64,
	closingPeriod uint32,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.walletParameters = tbtc.WalletParameters{
		CreationPeriod:        creationPeriod,
		CreationMinBtcBalance: creationMinBtcBalance,
		CreationMaxBtcBalance: creationMaxBtcBalance,
		ClosureMinBtcBalance:  closureMinBtcBalance,
		MaxAge:                maxAge,
		MaxBtcTransfer:        maxBtcTransfer,
		ClosingPeriod:         closingPeriod,
	}
}

func (lc *LocalChain) GetLiveWalletsCount() (uint32, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.liveWalletsCountSet {
		return lc.liveWalletsCountValue, nil
	}

	count := uint32(0)
	for _, wallet := range lc.walletChainData {
		if wallet != nil && wallet.State == tbtc.StateLive {
			count++
		}
	}
	return count, nil
}

// SetLiveWalletsCount stores an explicit live-wallets count for tests.
func (lc *LocalChain) SetLiveWalletsCount(count uint32) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.liveWalletsCountValue = count
	lc.liveWalletsCountSet = true
}

func (lc *LocalChain) ComputeMainUtxoHash(mainUtxo *bitcoin.UnspentTransactionOutput) [32]byte {
	outputIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(outputIndexBytes, mainUtxo.Outpoint.OutputIndex)

	valueBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(valueBytes, uint64(mainUtxo.Value))

	return crypto.Keccak256Hash(
		append(
			append(
				mainUtxo.Outpoint.TransactionHash[:],
				outputIndexBytes...,
			), valueBytes...,
		),
	)
}

func (lc *LocalChain) ComputeMovingFundsCommitmentHash(targetWallets [][20]byte) [32]byte {
	packedWallets := []byte{}

	for _, wallet := range targetWallets {
		packedWallets = append(packedWallets, wallet[:]...)
		// Each wallet hash must be padded with 12 zero bytes following the
		// actual hash.
		packedWallets = append(packedWallets, make([]byte, 12)...)
	}

	return crypto.Keccak256Hash(packedWallets)
}

func (lc *LocalChain) AddPastMovingFundsCommitmentSubmittedEvent(
	filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
	event *tbtc.MovingFundsCommitmentSubmittedEvent,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastMovingFundsCommitmentSubmittedEventsKey(filter)
	if err != nil {
		return err
	}

	if _, ok := lc.pastMovingFundsCommitmentSubmittedEvents[eventsKey]; !ok {
		lc.pastMovingFundsCommitmentSubmittedEvents[eventsKey] = []*tbtc.MovingFundsCommitmentSubmittedEvent{}
	}

	lc.pastMovingFundsCommitmentSubmittedEvents[eventsKey] = append(
		lc.pastMovingFundsCommitmentSubmittedEvents[eventsKey],
		event,
	)

	return nil
}

func (lc *LocalChain) PastMovingFundsCommitmentSubmittedEvents(
	filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
) ([]*tbtc.MovingFundsCommitmentSubmittedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastMovingFundsCommitmentSubmittedEventsKey(filter)
	if err != nil {
		return nil, err
	}

	events, ok := lc.pastMovingFundsCommitmentSubmittedEvents[eventsKey]
	if !ok {
		return nil, fmt.Errorf("no events for given filter")
	}

	return events, nil
}

func (lc *LocalChain) AddPastMovingFundsCompletedEvent(
	filter *tbtc.MovingFundsCompletedEventFilter,
	event *tbtc.MovingFundsCompletedEvent,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastMovingFundsCompletedEventsKey(filter)
	if err != nil {
		return err
	}

	if _, ok := lc.pastMovingFundsCompletedEvents[eventsKey]; !ok {
		lc.pastMovingFundsCompletedEvents[eventsKey] = []*tbtc.MovingFundsCompletedEvent{}
	}

	lc.pastMovingFundsCompletedEvents[eventsKey] = append(
		lc.pastMovingFundsCompletedEvents[eventsKey],
		event,
	)

	return nil
}

func (lc *LocalChain) PastMovingFundsCompletedEvents(
	filter *tbtc.MovingFundsCompletedEventFilter,
) ([]*tbtc.MovingFundsCompletedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastMovingFundsCompletedEventsKey(filter)
	if err != nil {
		return nil, err
	}

	events, ok := lc.pastMovingFundsCompletedEvents[eventsKey]
	if !ok {
		return nil, fmt.Errorf("no events for given filter")
	}

	return events, nil
}

func (lc *LocalChain) SubmitMovingFundsCommitment(
	walletPublicKeyHash [20]byte,
	walletMainUtxo bitcoin.UnspentTransactionOutput,
	walletMembersIDs []uint32,
	walletMemberIndex uint32,
	targetWallets [][20]byte,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.movingFundsCommitmentSubmissions = append(
		lc.movingFundsCommitmentSubmissions,
		&movingFundsCommitmentSubmission{
			WalletPublicKeyHash: walletPublicKeyHash,
			WalletMainUtxo:      &walletMainUtxo,
			WalletMembersIDs:    walletMembersIDs,
			WalletMemberIndex:   walletMemberIndex,
			TargetWallets:       targetWallets,
		},
	)

	return nil
}

func (lc *LocalChain) GetMovingFundsSubmissions() []*movingFundsCommitmentSubmission {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.movingFundsCommitmentSubmissions
}

func buildMovedFundsSweepProposalValidationKey(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.MovedFundsSweepProposal,
) ([32]byte, error) {
	var buffer bytes.Buffer

	buffer.Write(walletPublicKeyHash[:])

	buffer.Write(proposal.MovingFundsTxHash[:])
	binary.Write(&buffer, binary.BigEndian, proposal.MovingFundsTxOutputIndex)
	buffer.Write(proposal.SweepTxFee.Bytes())

	return sha256.Sum256(buffer.Bytes()), nil
}

func (lc *LocalChain) ValidateMovedFundsSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.MovedFundsSweepProposal,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildMovedFundsSweepProposalValidationKey(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return err
	}

	result, ok := lc.movedFundsSweepProposalValidations[key]
	if !ok {
		return fmt.Errorf("validation result unknown")
	}

	if !result {
		return fmt.Errorf("validation failed")
	}

	return nil
}

func (lc *LocalChain) SetMovedFundsSweepProposalValidationResult(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.MovedFundsSweepProposal,
	result bool,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildMovedFundsSweepProposalValidationKey(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return err
	}

	lc.movedFundsSweepProposalValidations[key] = result

	return nil
}

func (lc *LocalChain) GetRedemptionDelay(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (time.Duration, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key := buildRedemptionRequestKey(walletPublicKeyHash, redeemerOutputScript)

	delay, ok := lc.redemptionDelays[key]
	if !ok {
		return 0, fmt.Errorf("redemption delay not found")
	}

	return delay, nil
}

func (lc *LocalChain) SetRedemptionDelay(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
	delay time.Duration,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key := buildRedemptionRequestKey(walletPublicKeyHash, redeemerOutputScript)

	lc.redemptionDelays[key] = delay
}

func (lc *LocalChain) GetDepositMinAge() (uint32, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.depositMinAge, nil
}

func (lc *LocalChain) SetDepositMinAge(depositMinAge uint32) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.depositMinAge = depositMinAge
}

type MockBlockCounter struct {
	mutex        sync.Mutex
	currentBlock uint64
}

func NewMockBlockCounter() *MockBlockCounter {
	return &MockBlockCounter{}
}

func (mbc *MockBlockCounter) WaitForBlockHeight(blockNumber uint64) error {
	return nil
}

func (mbc *MockBlockCounter) BlockHeightWaiter(blockNumber uint64) (
	<-chan uint64,
	error,
) {
	panic("unsupported")
}

func (mbc *MockBlockCounter) CurrentBlock() (uint64, error) {
	mbc.mutex.Lock()
	defer mbc.mutex.Unlock()

	return mbc.currentBlock, nil
}

func (mbc *MockBlockCounter) SetCurrentBlock(block uint64) {
	mbc.mutex.Lock()
	defer mbc.mutex.Unlock()

	mbc.currentBlock = block
}

func (mbc *MockBlockCounter) WatchBlocks(ctx context.Context) <-chan uint64 {
	panic("unsupported")
}

// ValidateReservationAnchorProposal mirrors every precondition
// WalletProposalValidator.sol's validateReservationAnchorProposal rejects,
// so a caller that violates any of them cannot pass the fake the way it
// would pass a lenient one: the wallet state, the pending acceptance action
// record and its timeout margin, the deposit's reveal/age/sweep/reserved
// state and vault routing, the anchor fee bounds against the generation's
// snapshotted max fee, the snapshotted minimum, the deposit extra info
// (funding transaction hash and deposit locking script, as
// validateDepositExtraInfo checks them), the refund safety margin, and
// the deposit's controlling wallet.
func (lc *LocalChain) ValidateReservationAnchorProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationAnchorProposal,
	depositExtraInfo struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	if proposal == nil {
		return fmt.Errorf("proposal is required")
	}

	wallet, err := lc.GetWallet(walletPublicKeyHash)
	if err != nil ||
		(wallet.State != tbtc.StateLive &&
			wallet.State != tbtc.StateMovingFunds) {
		return fmt.Errorf("wallet is not in Live or MovingFunds state")
	}

	depositKey := lc.BuildDepositKey(
		proposal.DepositFundingTxHash,
		proposal.DepositFundingOutputIndex,
	)

	action, err := lc.GetReservationAction(depositKey, proposal.RequestNonce)
	if err != nil {
		return fmt.Errorf("not a pending acceptance action")
	}
	if action.ActionType != tbtc.ReservationActionTypeAcceptance {
		return fmt.Errorf("not a pending acceptance action")
	}
	if action.State != tbtc.ReservationActionStatePending {
		return fmt.Errorf("acceptance action is not pending")
	}
	if uint64(time.Now().Unix())+
		reservationRequestTimeoutSafetyMarginSeconds >=
		uint64(action.TimeoutAt) {
		return fmt.Errorf("acceptance action has timed out")
	}
	if action.TargetWalletPublicKeyHash != walletPublicKeyHash {
		return fmt.Errorf("wallet does not match the authorized action")
	}

	depositRequest, foundRequest, err := lc.GetDepositRequest(
		proposal.DepositFundingTxHash,
		proposal.DepositFundingOutputIndex,
	)
	if err != nil {
		return err
	}
	if !foundRequest || depositRequest.RevealedAt.IsZero() {
		return fmt.Errorf("deposit not revealed")
	}
	if minAgeSeconds, err := lc.GetDepositMinAge(); err == nil &&
		!time.Now().After(
			depositRequest.RevealedAt.Add(
				time.Duration(minAgeSeconds)*time.Second,
			),
		) {
		return fmt.Errorf("deposit min age not achieved yet")
	}
	// The chain adapter reports an unswept deposit as the UNIX epoch
	// (sweptAt == 0 on-chain); a zero time.Time is accepted as well.
	if !depositRequest.SweptAt.IsZero() && depositRequest.SweptAt.Unix() != 0 {
		return fmt.Errorf("deposit already swept")
	}

	// The deposit must be reserved and routed to the configured
	// reservation vault -- the two conditions that make it anchorable
	// through the reservation flow instead of the default sweep.
	if reserved, err := lc.IsReservedDeposit(depositKey); err != nil ||
		!reserved {
		return fmt.Errorf("deposit was not revealed as reserved")
	}
	if params, err := lc.ReservationParameters(); err != nil ||
		params.ReservationVault == "" ||
		depositRequest.Vault == nil ||
		*depositRequest.Vault != params.ReservationVault {
		return fmt.Errorf("deposit not routed to the reservation vault")
	}

	if proposal.AnchorTxFee == nil || proposal.AnchorTxFee.Sign() <= 0 {
		return fmt.Errorf("proposed transaction fee cannot be zero")
	}
	if proposal.AnchorTxFee.Cmp(
		new(big.Int).SetUint64(action.TxMaxFee),
	) > 0 {
		return fmt.Errorf("proposed transaction fee is too high")
	}
	// The deposit amount must stay at or above the generation's
	// snapshotted minimum plus the proposed fee. Mirrors the on-chain
	// check that rejects when the fee exceeds a small deposit's amount.
	minPlusFee := new(big.Int).Add(
		new(big.Int).SetUint64(action.MinAmount),
		proposal.AnchorTxFee,
	)
	if new(big.Int).SetUint64(depositRequest.Amount).
		Cmp(minPlusFee) < 0 {
		return fmt.Errorf("anchor amount below the reservation minimum")
	}

	deposit := depositExtraInfo.Deposit
	if deposit == nil {
		return fmt.Errorf("deposit extra info is required")
	}

	// Mirror validateDepositExtraInfo: the extra info's funding
	// transaction must hash to the proposal's funding transaction hash,
	// and its output at the proposal's index must lock funds with the
	// deposit script rebuilt from the on-chain depositor and extra data
	// plus the extra info's reveal fields, as P2SH or P2WSH.
	fundingTx := depositExtraInfo.FundingTx
	if fundingTx == nil ||
		fundingTx.Hash() != proposal.DepositFundingTxHash {
		return fmt.Errorf("extra info funding tx hash does not match")
	}
	if int(proposal.DepositFundingOutputIndex) >= len(fundingTx.Outputs) {
		return fmt.Errorf("extra info funding output script does not match")
	}
	depositScript, err := (&tbtc.Deposit{
		Depositor:           depositRequest.Depositor,
		ExtraData:           depositRequest.ExtraData,
		BlindingFactor:      deposit.BlindingFactor,
		WalletPublicKeyHash: deposit.WalletPublicKeyHash,
		RefundPublicKeyHash: deposit.RefundPublicKeyHash,
		RefundLocktime:      deposit.RefundLocktime,
	}).Script()
	if err != nil {
		return fmt.Errorf("cannot build deposit script: [%v]", err)
	}
	p2wsh, err := bitcoin.PayToWitnessScriptHash(
		bitcoin.WitnessScriptHash(depositScript),
	)
	if err != nil {
		return err
	}
	p2sh, err := bitcoin.PayToScriptHash(bitcoin.ScriptHash(depositScript))
	if err != nil {
		return err
	}
	fundingOutputScript :=
		fundingTx.Outputs[proposal.DepositFundingOutputIndex].PublicKeyScript
	if !bytes.Equal(fundingOutputScript, p2wsh) &&
		!bytes.Equal(fundingOutputScript, p2sh) {
		return fmt.Errorf("extra info funding output script does not match")
	}

	// The refund locktime is stored little-endian on-chain; reverse it
	// back to a UNIX timestamp and preserve the 24-hour refund safety
	// margin the on-chain validator enforces.
	refundableTimestamp := uint64(
		uint32(deposit.RefundLocktime[0]) |
			uint32(deposit.RefundLocktime[1])<<8 |
			uint32(deposit.RefundLocktime[2])<<16 |
			uint32(deposit.RefundLocktime[3])<<24,
	)
	if uint64(time.Now().Unix())+
		reservationDepositRefundSafetyMarginSeconds >=
		refundableTimestamp {
		return fmt.Errorf("deposit refund safety margin is not preserved")
	}
	if deposit.WalletPublicKeyHash != walletPublicKeyHash {
		return fmt.Errorf("deposit controlled by different wallet")
	}

	return nil
}

// ValidateReservationReanchorProposal mirrors every precondition
// WalletProposalValidator.sol's validateReservationReanchorProposal
// rejects: the pending re-anchor action record and its timeout margin,
// the custody identity of the source wallet, the target's identity,
// state, and count/amount headroom, and the fee bounds against the
// generation's snapshotted max fee. A test-registered
// SetReservationReanchorProposalValidationResult(..., false) still forces
// failure (used to drive a specific non-precondition failure mode), but a
// registered "true" no longer bypasses the precondition checks themselves.
func (lc *LocalChain) ValidateReservationReanchorProposal(
	sourceWalletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationReanchorProposal,
) error {
	lc.mutex.Lock()
	lc.reservationReanchorValidationCalls++
	lc.mutex.Unlock()

	if proposal == nil {
		return fmt.Errorf("proposal is required")
	}

	lc.mutex.Lock()
	key, err := buildReservationReanchorProposalValidationKey(
		sourceWalletPublicKeyHash,
		proposal,
	)
	if err != nil {
		lc.mutex.Unlock()
		return err
	}
	if result, ok := lc.reservationProposalValidations[key]; ok && !result {
		lc.mutex.Unlock()
		return fmt.Errorf("validation failed")
	}
	lc.mutex.Unlock()

	action, err := lc.GetReservationAction(proposal.ReservationKey, proposal.RequestNonce)
	if err != nil {
		return fmt.Errorf("not a pending re-anchor action")
	}
	if action.ActionType != tbtc.ReservationActionTypeReanchor {
		return fmt.Errorf("not a pending re-anchor action")
	}
	if action.State != tbtc.ReservationActionStatePending {
		return fmt.Errorf("re-anchor action is not pending")
	}
	if uint64(time.Now().Unix())+
		reservationRequestTimeoutSafetyMarginSeconds >=
		uint64(action.TimeoutAt) {
		return fmt.Errorf("re-anchor action has timed out")
	}
	if action.TargetWalletPublicKeyHash != proposal.TargetWalletPublicKeyHash {
		return fmt.Errorf("target wallet does not match the authorized action")
	}

	reservation, err := lc.GetReservation(proposal.ReservationKey)
	if err != nil ||
		reservation.WalletPublicKeyHash != sourceWalletPublicKeyHash {
		return fmt.Errorf("reservation custodied by different wallet")
	}
	if sourceWalletPublicKeyHash == proposal.TargetWalletPublicKeyHash {
		return fmt.Errorf(
			"target wallet must differ from the source wallet",
		)
	}

	targetWallet, err := lc.GetWallet(proposal.TargetWalletPublicKeyHash)
	if err != nil || targetWallet.State != tbtc.StateLive {
		return fmt.Errorf("target wallet must be in Live state")
	}

	params, err := lc.ReservationParameters()
	if err != nil {
		return fmt.Errorf("cannot get reservation parameters: [%v]", err)
	}
	count, err := lc.WalletReservationsCount(proposal.TargetWalletPublicKeyHash)
	if err != nil {
		return fmt.Errorf("cannot get wallet reservations count: [%v]", err)
	}
	if count > params.MaxReservationsPerWallet {
		return fmt.Errorf("wallet reservations cap exceeded")
	}

	maxAmount, _, err := lc.ReservationCaps()
	if err != nil {
		return fmt.Errorf("cannot get reservation caps: [%v]", err)
	}
	if maxAmount > 0 {
		amount, err := lc.WalletReservationsAmount(proposal.TargetWalletPublicKeyHash)
		if err != nil {
			return fmt.Errorf("cannot get wallet reservations amount: [%v]", err)
		}
		if amount > maxAmount {
			return fmt.Errorf("wallet reserved amount cap exceeded")
		}
	}
	if proposal.ReanchorTxFee == nil || proposal.ReanchorTxFee.Sign() <= 0 {
		return fmt.Errorf("proposed transaction fee cannot be zero")
	}
	if proposal.ReanchorTxFee.Cmp(
		new(big.Int).SetUint64(action.TxMaxFee),
	) > 0 {
		return fmt.Errorf("proposed transaction fee is too high")
	}

	return nil
}

// SetReservationReanchorProposalValidationResult stores the validation
// outcome for the given (sourceWalletPublicKeyHash, proposal) tuple.
func (lc *LocalChain) SetReservationReanchorProposalValidationResult(
	sourceWalletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationReanchorProposal,
	result bool,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key, err := buildReservationReanchorProposalValidationKey(
		sourceWalletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return err
	}

	lc.reservationProposalValidations[key] = result
	return nil
}

// GetReservationReanchorValidationCallCount returns the number of times
// ValidateReservationReanchorProposal has been called, so tests can
// confirm whether production actually reached on-chain validation for a
// candidate proposal.
func (lc *LocalChain) GetReservationReanchorValidationCallCount() int {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.reservationReanchorValidationCalls
}

func buildReservationReanchorProposalValidationKey(
	sourceWalletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationReanchorProposal,
) ([32]byte, error) {
	var buffer bytes.Buffer

	buffer.Write(sourceWalletPublicKeyHash[:])

	if proposal != nil {
		if proposal.ReservationKey != nil {
			buffer.Write(proposal.ReservationKey.Bytes())
		}
		for i := 0; i < 8; i++ {
			buffer.Write([]byte{byte(proposal.RequestNonce >> (8 * i))})
		}
		buffer.Write(proposal.TargetWalletPublicKeyHash[:])
		if proposal.ReanchorTxFee != nil {
			buffer.Write(proposal.ReanchorTxFee.Bytes())
		}
	}

	return sha256.Sum256(buffer.Bytes()), nil
}

// RequestReservationReanchor mirrors Reservation.sol's
// requestReservationReanchor for a permissionless caller. It reverts, in
// Solidity's order and with Solidity's reasons, unless the reservation is
// Active and out of its re-anchor cooldown, the source wallet is
// MovingFunds or Closing, the target differs from the source and is Live,
// the action timeout leaves a signing window before the validator's safety
// margin, the anchor stays above ReservationTxMaxFee + ReservationMinAmount,
// and the target has count headroom (a zero MaxReservationsPerWallet blocks
// every request) and amount headroom (a zero amount cap disables that
// check). A request that passes is recorded as a submission; it then
// reserves the target's count and amount capacity, bumps the nonce, moves
// the reservation to ActionPending and writes exactly the action fields
// Solidity writes. Every call, reverted or not, is recorded as an attempt.
//
// SetNextReservationReanchorRequestPending makes the next request that
// passes the checks succeed without writing anything, matching a
// transaction that has been sent but not mined.
func (lc *LocalChain) RequestReservationReanchor(
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationReanchorRequestAttempts = append(
		lc.reservationReanchorRequestAttempts,
		&reservationReanchorRequestSubmission{
			ReservationKey:            new(big.Int).Set(reservationKey),
			TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		},
	)

	if !lc.reservationParametersSet {
		return fmt.Errorf("reservation parameters not set")
	}
	params := lc.reservationParametersValue

	key := reservationKey.Text(16)
	existing, ok := lc.reservations[key]
	if !ok || existing == nil || existing.State != tbtc.ReservationStateActive {
		return fmt.Errorf("Reservation is not active")
	}

	now := uint32(time.Now().Unix())
	if now < existing.ReanchorCooldownUntil {
		return fmt.Errorf("Reanchor cooldown in effect")
	}

	// walletChainData is read directly: GetWallet takes lc.mutex, which
	// this method already holds.
	sourceState := tbtc.StateUnknown
	if source, ok := lc.walletChainData[existing.WalletPublicKeyHash]; ok {
		sourceState = source.State
	}
	if sourceState == tbtc.StateLive {
		return fmt.Errorf("Only governance can rotate a Live wallet's anchor")
	}
	if sourceState != tbtc.StateMovingFunds && sourceState != tbtc.StateClosing {
		return fmt.Errorf("Source wallet must be in MovingFunds or Closing state")
	}

	if targetWalletPublicKeyHash == existing.WalletPublicKeyHash {
		return fmt.Errorf("Target wallet must differ from the source wallet")
	}
	if target, ok := lc.walletChainData[targetWalletPublicKeyHash]; !ok ||
		target.State != tbtc.StateLive {
		return fmt.Errorf("Target wallet must be in Live state")
	}

	timeoutAt := now + params.ReservationActionTimeout
	if !(uint64(timeoutAt) > reservationRequestTimeoutSafetyMarginSeconds &&
		uint64(now)+1 < uint64(timeoutAt)-reservationRequestTimeoutSafetyMarginSeconds) {
		return fmt.Errorf("Reanchor authorization has no signing window")
	}

	anchorAmount := uint64(0)
	if existing.AnchorUtxo != nil {
		anchorAmount = uint64(existing.AnchorUtxo.Value)
	}
	if anchorAmount <= params.ReservationTxMaxFee+params.ReservationMinAmount {
		return fmt.Errorf(
			"Reanchor would fall below the minimum reservation amount",
		)
	}

	targetCount, targetAmount := lc.walletReservationInfoLocked(
		targetWalletPublicKeyHash,
	)
	if targetCount+1 > params.MaxReservationsPerWallet {
		return fmt.Errorf("Wallet reservations cap exceeded")
	}
	if maxAmount := lc.reservationCapsLocked()[0]; maxAmount != 0 &&
		targetAmount+anchorAmount > maxAmount {
		return fmt.Errorf("Wallet reserved amount cap exceeded")
	}

	lc.reservationReanchorRequestSubmissions = append(
		lc.reservationReanchorRequestSubmissions,
		&reservationReanchorRequestSubmission{
			ReservationKey:            new(big.Int).Set(reservationKey),
			TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		},
	)

	if lc.pendingNextReanchorRequest {
		lc.pendingNextReanchorRequest = false
		return nil
	}

	reserved := lc.reanchorReservedCapacity[targetWalletPublicKeyHash]
	reserved.count++
	reserved.amount += anchorAmount
	lc.reanchorReservedCapacity[targetWalletPublicKeyHash] = reserved

	updated := *existing
	updated.RequestNonce++
	updated.State = tbtc.ReservationStateActionPending
	lc.reservations[key] = &updated

	actionKey := buildReservationActionKey(reservationKey, updated.RequestNonce)
	lc.reservationActions[actionKey] = &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		RequestedAt:               now,
		TimeoutAt:                 timeoutAt,
		TxMaxFee:                  params.ReservationTxMaxFee,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		Amount:                    anchorAmount,
		SourceAnchorUtxoHash:      reservationAnchorUtxoHash(existing),
	}

	return nil
}

// TimeOutReservationReanchor mirrors Reservation.sol's
// notifyReservationActionTimeout for the reservation's current Pending
// Reanchor generation, as if called now: the action becomes TimedOut, the
// target's reserved capacity is released, the reservation returns to
// Active, and its re-anchor cooldown runs for the action's own duration.
// The timeoutAt check is skipped so tests need not wait for it.
func (lc *LocalChain) TimeOutReservationReanchor(reservationKey *big.Int) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	reservation, action, err := lc.pendingReanchorLocked(reservationKey)
	if err != nil {
		return err
	}

	timedOut := *action
	timedOut.State = tbtc.ReservationActionStateTimedOut
	lc.reservationActions[buildReservationActionKey(
		reservationKey,
		reservation.RequestNonce,
	)] = &timedOut

	lc.releaseReanchorReservedCapacityLocked(action)

	updated := *reservation
	updated.State = tbtc.ReservationStateActive
	updated.ReanchorCooldownUntil = uint32(time.Now().Unix()) +
		(action.TimeoutAt - action.RequestedAt)
	lc.reservations[reservationKey.Text(16)] = &updated

	return nil
}

// SettleReservationReanchor mirrors the accounting of a proven re-anchor
// (ReservationProofs.sol's settleReanchorAccounting) for the reservation's
// current Pending Reanchor generation: the action becomes Settled, custody
// and the reservation key move from the source wallet to the target, the
// anchor becomes newAnchorUtxo, and the reservation returns to Active. The
// capacity reserved on the target at request time becomes the target's
// custodied reservation, so the target keeps count + 1 and its amount
// drops by the miner fee, while the source's count and amount are
// released.
func (lc *LocalChain) SettleReservationReanchor(
	reservationKey *big.Int,
	newAnchorUtxo *bitcoin.UnspentTransactionOutput,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	reservation, action, err := lc.pendingReanchorLocked(reservationKey)
	if err != nil {
		return err
	}

	settled := *action
	settled.State = tbtc.ReservationActionStateSettled
	lc.reservationActions[buildReservationActionKey(
		reservationKey,
		reservation.RequestNonce,
	)] = &settled

	lc.releaseReanchorReservedCapacityLocked(action)

	source := reservation.WalletPublicKeyHash
	remaining := make([]*big.Int, 0, len(lc.reservationWalletKeys[source]))
	for _, k := range lc.reservationWalletKeys[source] {
		if k.Cmp(reservationKey) != 0 {
			remaining = append(remaining, k)
		}
	}
	lc.reservationWalletKeys[source] = remaining
	lc.reservationWalletKeys[action.TargetWalletPublicKeyHash] = append(
		lc.reservationWalletKeys[action.TargetWalletPublicKeyHash],
		new(big.Int).Set(reservationKey),
	)

	updated := *reservation
	updated.State = tbtc.ReservationStateActive
	updated.WalletPublicKeyHash = action.TargetWalletPublicKeyHash
	updated.AnchorUtxo = newAnchorUtxo
	lc.reservations[reservationKey.Text(16)] = &updated

	return nil
}

// pendingReanchorLocked returns the reservation and its current action,
// failing unless the reservation is ActionPending and the action is a
// Pending Reanchor. Callers hold lc.mutex.
func (lc *LocalChain) pendingReanchorLocked(
	reservationKey *big.Int,
) (*tbtc.Reservation, *tbtc.ReservationAction, error) {
	reservation, ok := lc.reservations[reservationKey.Text(16)]
	if !ok || reservation.State != tbtc.ReservationStateActionPending {
		return nil, nil, fmt.Errorf("Reservation is not in ActionPending state")
	}
	action, ok := lc.reservationActions[buildReservationActionKey(
		reservationKey,
		reservation.RequestNonce,
	)]
	if !ok ||
		action.ActionType != tbtc.ReservationActionTypeReanchor ||
		action.State != tbtc.ReservationActionStatePending {
		return nil, nil, fmt.Errorf("Action is not a pending re-anchor")
	}
	return reservation, action, nil
}

// releaseReanchorReservedCapacityLocked releases the target capacity a
// re-anchor request reserved. Callers hold lc.mutex.
func (lc *LocalChain) releaseReanchorReservedCapacityLocked(
	action *tbtc.ReservationAction,
) {
	reserved := lc.reanchorReservedCapacity[action.TargetWalletPublicKeyHash]
	reserved.count--
	reserved.amount -= action.Amount
	lc.reanchorReservedCapacity[action.TargetWalletPublicKeyHash] = reserved
}

// reservationAnchorUtxoHash mirrors Reservation.sol's anchorUtxoHash:
// keccak256(anchorTxHash || uint32 anchorTxOutputIndex).
func reservationAnchorUtxoHash(reservation *tbtc.Reservation) [32]byte {
	if reservation.AnchorUtxo == nil || reservation.AnchorUtxo.Outpoint == nil {
		return [32]byte{}
	}
	outpoint := reservation.AnchorUtxo.Outpoint
	packed := make([]byte, 36)
	copy(packed, outpoint.TransactionHash[:])
	binary.BigEndian.PutUint32(packed[32:], outpoint.OutputIndex)
	var hash [32]byte
	copy(hash[:], crypto.Keccak256(packed))
	return hash
}

// GetReservationReanchorRequestAttempts returns every
// RequestReservationReanchor call, including ones the fake reverted.
func (lc *LocalChain) GetReservationReanchorRequestAttempts() []*reservationReanchorRequestSubmission {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	attempts := make([]*reservationReanchorRequestSubmission, len(lc.reservationReanchorRequestAttempts))
	for i, a := range lc.reservationReanchorRequestAttempts {
		attempts[i] = &reservationReanchorRequestSubmission{
			ReservationKey:            new(big.Int).Set(a.ReservationKey),
			TargetWalletPublicKeyHash: a.TargetWalletPublicKeyHash,
		}
	}
	return attempts
}

// SetNextReservationReanchorRequestPending makes the next
// RequestReservationReanchor submission that passes the checks succeed
// while writing nothing, matching a request still unconfirmed in the
// mempool.
func (lc *LocalChain) SetNextReservationReanchorRequestPending() {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.pendingNextReanchorRequest = true
}

// NotifyMovingFundsBelowDust records a submitted below-dust notification
// for assertion in tests.
func (lc *LocalChain) NotifyMovingFundsBelowDust(
	walletPublicKeyHash [20]byte,
	mainUtxo *bitcoin.UnspentTransactionOutput,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.belowDustNotifications = append(
		lc.belowDustNotifications,
		&belowDustNotification{
			WalletPublicKeyHash: walletPublicKeyHash,
			MainUtxo:            mainUtxo,
		},
	)
	return nil
}

// GetBelowDustNotifications returns the recorded NotifyMovingFundsBelowDust
// submissions for assertion.
func (lc *LocalChain) GetBelowDustNotifications() []*belowDustNotification {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	copy := make([]*belowDustNotification, len(lc.belowDustNotifications))
	for i, n := range lc.belowDustNotifications {
		copy[i] = n
	}
	return copy
}

// GetReservation returns the configured reservation record for the given
// reservation key, or an error if not found.
func (lc *LocalChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if reservation, ok := lc.reservations[reservationKey.Text(16)]; ok {
		return reservation, nil
	}
	return nil, fmt.Errorf("reservation not found")
}

// SetReservation stores the given reservation record keyed by reservationKey.
func (lc *LocalChain) SetReservation(
	reservationKey *big.Int,
	reservation *tbtc.Reservation,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservations[reservationKey.Text(16)] = reservation
}

// GetReservationAction returns the configured reservation action record.
func (lc *LocalChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key := buildReservationActionKey(reservationKey, requestNonce)
	if action, ok := lc.reservationActions[key]; ok {
		return action, nil
	}
	// Fall back to comparing by value
	for k, a := range lc.reservationActions {
		expected := buildReservationActionKey(reservationKey, requestNonce)
		if k == expected {
			return a, nil
		}
	}
	return nil, fmt.Errorf("reservation action not found")
}

// SetReservationAction stores the given reservation action record.
func (lc *LocalChain) SetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
	action *tbtc.ReservationAction,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key := buildReservationActionKey(reservationKey, requestNonce)
	lc.reservationActions[key] = action
}

func buildReservationActionKey(
	reservationKey *big.Int,
	requestNonce uint64,
) string {
	if reservationKey == nil {
		return fmt.Sprintf("nil/%d", requestNonce)
	}
	return fmt.Sprintf("%s/%d", reservationKey.String(), requestNonce)
}

// ReservationParameters returns the configured reservation parameters.
func (lc *LocalChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if !lc.reservationParametersSet {
		return nil, fmt.Errorf("reservation parameters not set")
	}
	params := lc.reservationParametersValue
	return &params, nil
}

// SetReservationParameters stores the given reservation parameters.
func (lc *LocalChain) SetReservationParameters(params tbtc.ReservationParameters) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationParametersValue = params
	lc.reservationParametersSet = true
}

// ReservationCaps returns the chain's configured reservation cap-pair,
// or a zero cap-pair when no caps have been set via SetReservationCaps
// (a zero cap disables the amount-cap checks, matching the on-chain
// `maxReservationsAmountPerWallet == 0` convention).
func (lc *LocalChain) ReservationCaps() (
	maxReservationsAmountPerWallet uint64,
	reservationMaxSingleAmount uint64,
	err error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	caps := lc.reservationCapsLocked()
	return caps[0], caps[1], nil
}

// reservationCapsLocked returns the chain's configured reservation
// cap-pair, or a zero cap-pair when SetReservationCaps has not been
// called; a zero cap disables the amount-cap checks, matching the
// on-chain convention. Callers must hold lc.mutex.
func (lc *LocalChain) reservationCapsLocked() [2]uint64 {
	if lc.reservationCapsSet {
		return lc.reservationCaps
	}
	return [2]uint64{}
}

// SetReservationCaps configures the cap-pair returned by
// ReservationCaps, modeling a chain whose reservation capacity limits
// were set with values other than the zero default (which disables the
// amount-cap checks).
func (lc *LocalChain) SetReservationCaps(
	maxReservationsAmountPerWallet uint64,
	reservationMaxSingleAmount uint64,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationCaps = [2]uint64{
		maxReservationsAmountPerWallet,
		reservationMaxSingleAmount,
	}
	lc.reservationCapsSet = true
}

// WalletReservationsAmount returns the wallet's reserved amount: the
// anchor values of its custodied reservations plus the amount pending
// re-anchor requests reserved on it.
func (lc *LocalChain) WalletReservationsAmount(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	_, amount := lc.walletReservationInfoLocked(walletPublicKeyHash)
	return amount, nil
}

// WalletReservationsCount returns the wallet's reservation count: its
// custodied reservations plus the pending re-anchor requests targeting it.
func (lc *LocalChain) WalletReservationsCount(
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	count, _ := lc.walletReservationInfoLocked(walletPublicKeyHash)
	return count, nil
}

// walletReservationInfoLocked returns the wallet's walletReservationInfo
// ledger entry. Reservation.sol counts a reservation against the wallet
// custodying it and, while a re-anchor request is pending, also against
// the request's target; the fake derives the first part from the wallet's
// reservation keys and tracks the second in reanchorReservedCapacity.
// Callers hold lc.mutex.
func (lc *LocalChain) walletReservationInfoLocked(
	walletPublicKeyHash [20]byte,
) (uint32, uint64) {
	reserved := lc.reanchorReservedCapacity[walletPublicKeyHash]
	count := uint32(len(lc.reservationWalletKeys[walletPublicKeyHash])) + reserved.count
	amount := reserved.amount
	for _, reservationKey := range lc.reservationWalletKeys[walletPublicKeyHash] {
		if r, ok := lc.reservations[reservationKey.Text(16)]; ok && r != nil && r.AnchorUtxo != nil {
			amount += uint64(r.AnchorUtxo.Value)
		}
	}
	return count, amount
}

// WalletReservations returns the configured reservation keys for the wallet.
func (lc *LocalChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	keys := lc.reservationWalletKeys[walletPublicKeyHash]
	result := make([]*big.Int, len(keys))
	for i, k := range keys {
		result[i] = new(big.Int).Set(k)
	}
	return result, nil
}

// SetWalletReservations stores the reservation keys associated with the wallet.
func (lc *LocalChain) SetWalletReservations(
	walletPublicKeyHash [20]byte,
	reservationKeys []*big.Int,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	copy := make([]*big.Int, len(reservationKeys))
	for i, k := range reservationKeys {
		copy[i] = new(big.Int).Set(k)
	}
	lc.reservationWalletKeys[walletPublicKeyHash] = copy
}

// ActiveReservationsCount reports zero active reservations by default.
func (lc *LocalChain) ActiveReservationsCount() (
	count uint32,
	maxActive uint32,
	err error,
) {
	return 0, 0, nil
}

// IsReservedDeposit reports whether the given deposit key was marked
// reserved via SetReservedDeposit; false by default.
func (lc *LocalChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if depositKey == nil {
		return false, nil
	}
	return lc.reservedDeposits[depositKey.Text(16)], nil
}

// SetReservedDeposit marks the given deposit key as reserved (or not) for
// subsequent IsReservedDeposit calls.
func (lc *LocalChain) SetReservedDeposit(depositKey *big.Int, reserved bool) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservedDeposits[depositKey.Text(16)] = reserved
}

// SetReservationVaultFeeDebtSat configures the value and optional error
// returned by ReservationVaultFeeDebtSat.
func (lc *LocalChain) SetReservationVaultFeeDebtSat(
	debtSat uint64,
	err error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationVaultFeeDebtSat = debtSat
	lc.reservationVaultFeeDebtSatErr = err
}

// SetReservationVaultFeeReserveBalance configures the value and optional
// error returned by ReservationVaultFeeReserveTbtcBaseUnits. A nil
// balance value is stored as a zero big.Int.
func (lc *LocalChain) SetReservationVaultFeeReserveBalance(
	balance *big.Int,
	err error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if balance == nil {
		balance = new(big.Int)
	}
	lc.reservationVaultFeeReserveBalance = balance
	lc.reservationVaultFeeReserveErr = err
}

// ReservationVaultFeeDebtSat returns the value configured via
// SetReservationVaultFeeDebtSat; zero by default.
func (lc *LocalChain) ReservationVaultFeeDebtSat(
	reservationVault chain.Address,
) (uint64, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.reservationVaultFeeDebtSat, lc.reservationVaultFeeDebtSatErr
}

// ReservationVaultFeeReserveTbtcBaseUnits returns the value configured
// via SetReservationVaultFeeReserveBalance; zero by default.
func (lc *LocalChain) ReservationVaultFeeReserveTbtcBaseUnits(
	reservationVault chain.Address,
) (*big.Int, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.reservationVaultFeeReserveBalance == nil {
		return new(big.Int), lc.reservationVaultFeeReserveErr
	}
	return lc.reservationVaultFeeReserveBalance, lc.reservationVaultFeeReserveErr
}

// PastReservationAcceptanceRequestedEvents returns no events by default.
func (lc *LocalChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	return nil, nil
}

// PastReservationReanchorRequestedEvents returns the recorded re-anchor
// request submissions that match the filter.
func (lc *LocalChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	results := make([]*tbtc.ReservationReanchorRequestedEvent, 0)
	for _, submission := range lc.reservationReanchorRequestSubmissions {
		event := &tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            new(big.Int).Set(submission.ReservationKey),
			TargetWalletPublicKeyHash: submission.TargetWalletPublicKeyHash,
		}

		if filter != nil {
			if len(filter.TargetWalletPublicKeyHash) > 0 {
				matched := false
				for _, w := range filter.TargetWalletPublicKeyHash {
					if w == submission.TargetWalletPublicKeyHash {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
		}

		results = append(results, event)
	}
	return results, nil
}

// GetReservationReanchorRequestSubmissions returns the recorded
// reservation re-anchor request submissions for assertion.
func (lc *LocalChain) GetReservationReanchorRequestSubmissions() []*reservationReanchorRequestSubmission {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	copy := make([]*reservationReanchorRequestSubmission, len(lc.reservationReanchorRequestSubmissions))
	for i, s := range lc.reservationReanchorRequestSubmissions {
		copy[i] = &reservationReanchorRequestSubmission{
			ReservationKey:            new(big.Int).Set(s.ReservationKey),
			TargetWalletPublicKeyHash: s.TargetWalletPublicKeyHash,
		}
	}
	return copy
}

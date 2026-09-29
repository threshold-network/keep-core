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
	TxHash                    [32]byte
}

// reservationAcceptanceRequest captures a submitted reservation acceptance
// request that tests can inspect for assertion. Production no longer
// submits these: the operator-side RequestReservationAcceptance call was
// removed from the acceptance task, and the only on-chain caller of the
// underlying entrypoint is the deposit's depositor. The recording exists
// so tests can assert that the production task never reaches for that
// entrypoint on its own.
type reservationAcceptanceRequest struct {
	ReservationKey      *big.Int
	WalletPublicKeyHash [20]byte
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
	reservationReanchorRequestReceipts    map[[32]byte]tbtc.ReservationReanchorRequestReceiptStatus
	// revertNextReanchorRequest makes the next RequestReservationReanchor
	// submission record a Reverted receipt and write no generation.
	revertNextReanchorRequest bool
	// pendingNextReanchorRequest makes the next RequestReservationReanchor
	// submission record a Pending receipt and write no generation,
	// modeling a submission still unconfirmed in the mempool.
	pendingNextReanchorRequest bool
	// reservationReanchorValidationCalls counts calls to
	// ValidateReservationReanchorProposal, so tests can observe whether
	// production actually reached on-chain validation for a candidate
	// proposal (e.g. to prove the resume-path timeout safety margin
	// gate short-circuited before validation would have run).
	reservationReanchorValidationCalls int
	reservationAcceptanceRequests      []*reservationAcceptanceRequest
	belowDustNotifications             []*belowDustNotification
	reservationWalletKeys              map[[20]byte][]*big.Int
	reservedDeposits                   map[string]bool
	liveWalletsCountValue              uint32
	liveWalletsCountSet                bool
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
		reservationReanchorRequestReceipts:    make(map[[32]byte]tbtc.ReservationReanchorRequestReceiptStatus),
		reservationAcceptanceRequests:         make([]*reservationAcceptanceRequest, 0),
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

// ValidateReservationAnchorProposal enforces the on-chain precondition
// WalletProposalValidator.sol's validateReservationAnchorProposal checks
// before anything else: the action at proposal.RequestNonce, keyed by the
// deposit's own funding outpoint, must be a Pending Acceptance action
// targeting walletPublicKeyHash. Validation fails unless that precondition
// holds, so a fake that skips it (as the previous panic stub effectively
// did for every caller that overrode it) cannot mask a caller that
// validates before the depositor's requestReservationAcceptance action
// actually exists.
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

	return nil
}

// ValidateReservationReanchorProposal enforces the on-chain precondition
// WalletProposalValidator.sol's validateReservationReanchorProposal checks
// before anything else: the action at (proposal.ReservationKey,
// proposal.RequestNonce) must be a Pending Reanchor action authorizing
// proposal.TargetWalletPublicKeyHash, and the target wallet must be Live
// with count/amount headroom under the configured caps. A test-registered
// SetReservationReanchorProposalValidationResult(..., false) still forces
// failure (used to drive a specific non-precondition failure mode), but a
// registered "true" no longer bypasses the precondition check itself.
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

// RequestReservationAcceptance records a submitted reservation acceptance
// request for assertion in tests. Production never submits these anymore:
// only the deposit's own depositor may call the underlying entrypoint on
// chain, and the operator-side RequestReservationAcceptance call was
// removed from the acceptance task. The fake therefore does nothing but
// record -- no nonce bump, no action write -- so a test that observes any
// recorded submission can assert the operator-side path is not reachable
// through the fake.
func (lc *LocalChain) RequestReservationAcceptance(
	reservationKey *big.Int,
	walletPublicKeyHash [20]byte,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationAcceptanceRequests = append(
		lc.reservationAcceptanceRequests,
		&reservationAcceptanceRequest{
			ReservationKey:      new(big.Int).Set(reservationKey),
			WalletPublicKeyHash: walletPublicKeyHash,
		},
	)
	return nil
}

// GetReservationAcceptanceRequests returns the recorded
// RequestReservationAcceptance submissions for assertion.
func (lc *LocalChain) GetReservationAcceptanceRequests() []*reservationAcceptanceRequest {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	copy := make([]*reservationAcceptanceRequest, len(lc.reservationAcceptanceRequests))
	for i, r := range lc.reservationAcceptanceRequests {
		copy[i] = &reservationAcceptanceRequest{
			ReservationKey:      new(big.Int).Set(r.ReservationKey),
			WalletPublicKeyHash: r.WalletPublicKeyHash,
		}
	}
	return copy
}

// RequestReservationReanchor records a submitted reservation re-anchor
// request for assertion in tests, and mirrors the on-chain effects of
// Reservation.sol's requestReservationReanchor: it requires the
// reservation to be Active (fails closed otherwise, matching "Reservation
// is not active"), then bumps its RequestNonce, flips its state to
// ActionPending, and writes the new generation as a Pending Reanchor
// action authorizing targetWalletPublicKeyHash. This makes the new
// request-then-wait-then-read flow in ProposeReservationReanchor
// observable in tests: reading the action before this call succeeds
// returns "not found", exactly like an on-chain read of an
// as-yet-unwritten generation would.
//
// The fake models the submission as immediately mined, matching the
// production adapter's ForceMining fast path: it returns a deterministic
// transaction hash (derived from the submission contents plus the
// submission count) and registers the receipt as Mined, so the default
// GetReservationReanchorRequestReceipt answer for a recorded submission is
// Mined. Tests that need a different outcome (a reverted or still-pending
// receipt) override it via SetReservationReanchorRequestReceipt before
// driving the task. SetRevertNextReservationReanchorRequest makes the
// next submission succeed with a hash but record the receipt as Reverted
// instead of writing the on-chain generation, matching a request that
// reverts on-chain after submission.
func (lc *LocalChain) RequestReservationReanchor(
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
) ([32]byte, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key := reservationKey.Text(16)
	existing, ok := lc.reservations[key]
	if !ok || existing == nil || existing.State != tbtc.ReservationStateActive {
		return [32]byte{}, fmt.Errorf("reservation is not active")
	}

	submissionCount := len(lc.reservationReanchorRequestSubmissions)
	txHash := lc.nextReservationReanchorRequestHash(
		reservationKey,
		targetWalletPublicKeyHash,
		submissionCount,
	)

	lc.reservationReanchorRequestSubmissions = append(
		lc.reservationReanchorRequestSubmissions,
		&reservationReanchorRequestSubmission{
			ReservationKey:            new(big.Int).Set(reservationKey),
			TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
			TxHash:                    txHash,
		},
	)

	if lc.revertNextReanchorRequest {
		lc.revertNextReanchorRequest = false
		lc.reservationReanchorRequestReceipts[txHash] =
			tbtc.ReservationReanchorRequestReceiptReverted
		return txHash, nil
	}

	if lc.pendingNextReanchorRequest {
		lc.pendingNextReanchorRequest = false
		lc.reservationReanchorRequestReceipts[txHash] =
			tbtc.ReservationReanchorRequestReceiptPending
		return txHash, nil
	}

	// Mirror the request-time count-capacity check Reservation.sol's
	// requestReservationReanchor performs before reserving target
	// capacity. Computing against lc.reservationWalletKeys directly
	// rather than via WalletReservationsCount keeps everything under
	// the same mutex and avoids a re-entrant lock deadlock. The check
	// is only enforced when the live parameters have been set with a
	// nonzero MaxReservationsPerWallet (which the on-chain setter
	// itself requires > 0, so a zero value here only appears in tests
	// that have not yet configured parameters).
	if lc.reservationParametersSet &&
		lc.reservationParametersValue.MaxReservationsPerWallet > 0 {
		currentTargetCount := uint32(
			len(lc.reservationWalletKeys[targetWalletPublicKeyHash]),
		)
		if currentTargetCount+1 >
			lc.reservationParametersValue.MaxReservationsPerWallet {
			return [32]byte{}, fmt.Errorf("wallet reservations cap exceeded")
		}
	}

	updated := *existing
	updated.RequestNonce++
	updated.State = tbtc.ReservationStateActionPending
	lc.reservations[key] = &updated

	txMaxFee := lc.reservationParametersValue.ReservationTxMaxFee
	actionKey := buildReservationActionKey(reservationKey, updated.RequestNonce)
	lc.reservationActions[actionKey] = &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
		TxMaxFee:                  txMaxFee,
		MinAmount:                 lc.reservationParametersValue.ReservationMinAmount,
		// A fresh generation's timeout sits far enough ahead that the
		// validator's REQUEST_TIMEOUT_SAFETY_MARGIN gate (now plus
		// two hours) cannot reject it; tests that exercise the gate
		// seed their own actions with nearer TimeoutAt values.
		TimeoutAt:   uint32(time.Now().Add(24 * time.Hour).Unix()),
		TermSeconds: lc.reservationParametersValue.ReservationTermSeconds,
	}

	lc.reservationReanchorRequestReceipts[txHash] =
		tbtc.ReservationReanchorRequestReceiptMined

	return txHash, nil
}

// nextReservationReanchorRequestHash derives the fake's deterministic
// transaction hash for a re-anchor submission: the keccak-256 of the
// reservation key, the target wallet hash, and the submission count.
// Callers hold lc.mutex.
func (lc *LocalChain) nextReservationReanchorRequestHash(
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
	submissionCount int,
) [32]byte {
	var buffer bytes.Buffer
	buffer.Write(reservationKey.Bytes())
	buffer.Write(targetWalletPublicKeyHash[:])
	for i := 0; i < 4; i++ {
		buffer.Write([]byte{byte(uint32(submissionCount) >> (8 * i))})
	}
	var hash [32]byte
	copy(hash[:], crypto.Keccak256(buffer.Bytes()))
	return hash
}

// GetReservationReanchorRequestReceipt returns the recorded receipt status
// of a RequestReservationReanchor submission: a hash recorded via the
// submission itself (Mined, or Reverted when a revert was forced) or via
// SetReservationReanchorRequestPending reports its recorded status, and a
// hash never seen by the fake reports NotFound.
func (lc *LocalChain) GetReservationReanchorRequestReceipt(
	txHash [32]byte,
) (tbtc.ReservationReanchorRequestReceiptStatus, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if status, ok := lc.reservationReanchorRequestReceipts[txHash]; ok {
		return status, nil
	}
	return tbtc.ReservationReanchorRequestReceiptNotFound, nil
}

// SetReservationReanchorRequestReceipt overrides the receipt status the
// fake reports for the given submission hash (e.g. to model a request
// that is still pending in the mempool).
func (lc *LocalChain) SetReservationReanchorRequestReceipt(
	txHash [32]byte,
	status tbtc.ReservationReanchorRequestReceiptStatus,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationReanchorRequestReceipts[txHash] = status
}

// SetRevertNextReservationReanchorRequest makes the next
// RequestReservationReanchor submission succeed (returning its hash)
// while recording its receipt as Reverted and writing no on-chain
// generation.
func (lc *LocalChain) SetRevertNextReservationReanchorRequest() {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.revertNextReanchorRequest = true
}

// SetNextReservationReanchorRequestPending makes the next
// RequestReservationReanchor submission succeed (returning its hash)
// while recording its receipt as Pending and writing no on-chain
// generation, matching a request still unconfirmed in the mempool.
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

// ReservationCaps returns a static cap-pair useful for tests.
func (lc *LocalChain) ReservationCaps() (
	maxReservationsAmountPerWallet uint64,
	reservationMaxSingleAmount uint64,
	err error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return 100000000, 10000000, nil
}

// WalletReservationsAmount returns the sum of anchor values for the
// wallet's reservations.
func (lc *LocalChain) WalletReservationsAmount(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	var total uint64
	for _, reservationKey := range lc.reservationWalletKeys[walletPublicKeyHash] {
		if r, ok := lc.reservations[reservationKey.Text(16)]; ok && r != nil && r.AnchorUtxo != nil {
			total += uint64(r.AnchorUtxo.Value)
		}
	}
	return total, nil
}

// WalletReservationsCount returns the count of reservations for the wallet.
func (lc *LocalChain) WalletReservationsCount(
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return uint32(len(lc.reservationWalletKeys[walletPublicKeyHash])), nil
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
func (lc *LocalChain) ReservationVaultFeeDebtSat() (uint64, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.reservationVaultFeeDebtSat, lc.reservationVaultFeeDebtSatErr
}

// ReservationVaultFeeReserveTbtcBaseUnits returns the value configured
// via SetReservationVaultFeeReserveBalance; zero by default.
func (lc *LocalChain) ReservationVaultFeeReserveTbtcBaseUnits() (*big.Int, error) {
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
			TxHash:                    s.TxHash,
		}
	}
	return copy
}

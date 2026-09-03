package spv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

type submittedRedemptionProof struct {
	transaction         *bitcoin.Transaction
	proof               *bitcoin.SpvProof
	mainUTXO            bitcoin.UnspentTransactionOutput
	walletPublicKeyHash [20]byte
}

type submittedDepositSweepProof struct {
	transaction *bitcoin.Transaction
	proof       *bitcoin.SpvProof
	mainUTXO    bitcoin.UnspentTransactionOutput
	vault       common.Address
}

type submittedMovingFundsProof struct {
	transaction         *bitcoin.Transaction
	proof               *bitcoin.SpvProof
	mainUTXO            bitcoin.UnspentTransactionOutput
	walletPublicKeyHash [20]byte
}

type submittedMovedFundsSweepProof struct {
	transaction *bitcoin.Transaction
	proof       *bitcoin.SpvProof
	mainUTXO    bitcoin.UnspentTransactionOutput
}

// submittedReservationStranded records a NotifyReservationStranded call for
// assertion in tests.
type submittedReservationStranded struct {
	reservationKey *big.Int
}

// submittedStaleReservedDeposit records a NotifyStaleReservedDeposit call for
// assertion in tests.
type submittedStaleReservedDeposit struct {
	depositKey *big.Int
}

// submittedReservationActionTimeout records a NotifyReservationActionTimeout
// call for assertion in tests.
type submittedReservationActionTimeout struct {
	reservationKey   *big.Int
	walletMembersIDs []uint32
}

// reservedDepositRecord is the local-chain-side booking for a reserved
// deposit. WalletPublicKeyHash is the wallet currently assigned to the
// deposit; IsReserved drives the IsReservedDeposit return value.
type reservedDepositRecord struct {
	walletPublicKeyHash [20]byte
	isReserved          bool
}

type localChain struct {
	mutex sync.Mutex

	blockCounter                             chain.BlockCounter
	wallets                                  map[[20]byte]*tbtc.WalletChainData
	depositRequests                          map[[32]byte]*tbtc.DepositChainRequest
	pendingRedemptionRequests                map[[32]byte]*tbtc.RedemptionRequest
	movedFundsSweepRequests                  map[[32]byte]*tbtc.MovedFundsSweepRequest
	submittedRedemptionProofs                []*submittedRedemptionProof
	submittedDepositSweepProofs              []*submittedDepositSweepProof
	submittedMovingFundsProofs               []*submittedMovingFundsProof
	submittedMovedFundsSweepProofs           []*submittedMovedFundsSweepProof
	pastRedemptionRequestedEvents            map[[32]byte][]*tbtc.RedemptionRequestedEvent
	pastDepositRevealedEvents                map[[32]byte][]*tbtc.DepositRevealedEvent
	pastMovingFundsCommitmentSubmittedEvents map[[32]byte][]*tbtc.MovingFundsCommitmentSubmittedEvent

	// Reservation watcher state. Indexed by [16]byte / [24]byte map keys
	// derived from the relevant big.Int so they fit the map type without
	// per-test marshalling.
	walletReservations      map[[20]byte][]*big.Int
	reservations            map[string]*tbtc.Reservation
	reservationActions      map[string]*tbtc.ReservationAction
	reservedDeposits        map[string]*reservedDepositRecord
	submittedStrandedKeys   []*big.Int
	submittedStaleDeposits  []*big.Int
	submittedActionTimeouts []*submittedReservationActionTimeout
	reservationParameters   *tbtc.ReservationParameters

	// Error-injection fields for the reservation watcher chain-error
	// passthrough tests: nil (the default) means the corresponding method
	// falls through to its normal, table-driven behavior.
	getReservationActionErr           error
	walletReservationsErr             error
	isReservedDepositErr              error
	reservedDepositWalletErr          error
	notifyReservationActionTimeoutErr error
	notifyStaleReservedDepositErr     error
	pastNewWalletRegisteredEventsErr  error
	notifyReservationStrandedErrByKey map[string]error

	// Wallet registration and pending-action-request event state for the
	// watcher dispatch and reservation proof loop tests.
	newWalletRegisteredEvents            []*tbtc.NewWalletRegisteredEvent
	reservationAcceptanceRequestedEvents []*tbtc.ReservationAcceptanceRequestedEvent
	reservationReanchorRequestedEvents   []*tbtc.ReservationReanchorRequestedEvent

	txProofDifficultyFactor *big.Int
	currentEpoch            uint64
	currentEpochDifficulty  *big.Int
	previousEpochDifficulty *big.Int
	// submitReservationProofHook, when non-nil, overrides the default
	// success stub and gives the test full control over
	// SubmitReservationProof behavior (e.g. to assert arguments or return
	// an error).
	submitReservationProofHook func(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		reservationKey *big.Int,
		requestNonce uint64,
	) error
}

func newLocalChain() *localChain {
	return &localChain{
		wallets:                                  make(map[[20]byte]*tbtc.WalletChainData),
		depositRequests:                          make(map[[32]byte]*tbtc.DepositChainRequest),
		pendingRedemptionRequests:                make(map[[32]byte]*tbtc.RedemptionRequest),
		movedFundsSweepRequests:                  make(map[[32]byte]*tbtc.MovedFundsSweepRequest),
		submittedRedemptionProofs:                make([]*submittedRedemptionProof, 0),
		submittedDepositSweepProofs:              make([]*submittedDepositSweepProof, 0),
		submittedMovingFundsProofs:               make([]*submittedMovingFundsProof, 0),
		submittedMovedFundsSweepProofs:           make([]*submittedMovedFundsSweepProof, 0),
		pastRedemptionRequestedEvents:            make(map[[32]byte][]*tbtc.RedemptionRequestedEvent),
		pastDepositRevealedEvents:                make(map[[32]byte][]*tbtc.DepositRevealedEvent),
		pastMovingFundsCommitmentSubmittedEvents: make(map[[32]byte][]*tbtc.MovingFundsCommitmentSubmittedEvent),
		walletReservations:                       make(map[[20]byte][]*big.Int),
		reservations:                             make(map[string]*tbtc.Reservation),
		reservationActions:                       make(map[string]*tbtc.ReservationAction),
		reservedDeposits:                         make(map[string]*reservedDepositRecord),
		submittedStrandedKeys:                    make([]*big.Int, 0),
		submittedStaleDeposits:                   make([]*big.Int, 0),
		submittedActionTimeouts:                  make([]*submittedReservationActionTimeout, 0),
	}
}

func (lc *localChain) SubmitDepositSweepProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	vault common.Address,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.submittedDepositSweepProofs = append(
		lc.submittedDepositSweepProofs,
		&submittedDepositSweepProof{
			transaction: transaction,
			proof:       proof,
			mainUTXO:    mainUTXO,
			vault:       vault,
		},
	)

	return nil
}

func (lc *localChain) getSubmittedDepositSweepProofs() []*submittedDepositSweepProof {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.submittedDepositSweepProofs
}

func (lc *localChain) GetDepositRequest(
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

func (lc *localChain) setDepositRequest(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	depositRequest *tbtc.DepositChainRequest,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	requestKey := buildDepositRequestKey(fundingTxHash, fundingOutputIndex)
	lc.depositRequests[requestKey] = depositRequest
}

func buildDepositRequestKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) [32]byte {
	fundingOutputIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fundingOutputIndexBytes, fundingOutputIndex)

	return sha256.Sum256(append(fundingTxHash[:], fundingOutputIndexBytes...))
}

func (lc *localChain) GetWallet(walletPublicKeyHash [20]byte) (
	*tbtc.WalletChainData,
	error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	walletChainData, ok := lc.wallets[walletPublicKeyHash]
	if !ok {
		return nil, fmt.Errorf("no wallet for given PKH")
	}

	return walletChainData, nil
}

func (lc *localChain) setWallet(
	walletPublicKeyHash [20]byte,
	walletChainData *tbtc.WalletChainData,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.wallets[walletPublicKeyHash] = walletChainData
}

func (lc *localChain) ComputeMainUtxoHash(
	mainUtxo *bitcoin.UnspentTransactionOutput,
) [32]byte {
	var buffer bytes.Buffer

	buffer.Write(mainUtxo.Outpoint.TransactionHash[:])

	outputIndex := make([]byte, 4)
	binary.BigEndian.PutUint32(outputIndex, mainUtxo.Outpoint.OutputIndex)
	buffer.Write(outputIndex)

	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(mainUtxo.Value))
	buffer.Write(value)

	return sha256.Sum256(buffer.Bytes())
}

func (lc *localChain) TxProofDifficultyFactor() (*big.Int, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.txProofDifficultyFactor == nil {
		return nil, fmt.Errorf("transaction proof difficulty factor not set")
	}

	return lc.txProofDifficultyFactor, nil
}

func (lc *localChain) BlockCounter() (chain.BlockCounter, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.blockCounter, nil
}

func (lc *localChain) setBlockCounter(blockCounter chain.BlockCounter) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.blockCounter = blockCounter
}

func (lc *localChain) GetPendingRedemptionRequest(
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

func (lc *localChain) setPendingRedemptionRequest(
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

func buildRedemptionRequestKey(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) [32]byte {
	return sha256.Sum256(append(walletPublicKeyHash[:], redeemerOutputScript...))
}

func (lc *localChain) SubmitRedemptionProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	walletPublicKeyHash [20]byte,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.submittedRedemptionProofs = append(
		lc.submittedRedemptionProofs,
		&submittedRedemptionProof{
			transaction:         transaction,
			proof:               proof,
			mainUTXO:            mainUTXO,
			walletPublicKeyHash: walletPublicKeyHash,
		},
	)

	return nil
}

func (lc *localChain) getSubmittedRedemptionProofs() []*submittedRedemptionProof {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.submittedRedemptionProofs
}

func (lc *localChain) SubmitMovingFundsProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	walletPublicKeyHash [20]byte,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.submittedMovingFundsProofs = append(
		lc.submittedMovingFundsProofs,
		&submittedMovingFundsProof{
			transaction:         transaction,
			proof:               proof,
			mainUTXO:            mainUTXO,
			walletPublicKeyHash: walletPublicKeyHash,
		},
	)

	return nil
}

func (lc *localChain) getSubmittedMovingFundsProofs() []*submittedMovingFundsProof {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.submittedMovingFundsProofs
}

func (lc *localChain) SubmitMovedFundsSweepProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.submittedMovedFundsSweepProofs = append(
		lc.submittedMovedFundsSweepProofs,
		&submittedMovedFundsSweepProof{
			transaction: transaction,
			proof:       proof,
			mainUTXO:    mainUTXO,
		},
	)

	return nil
}

func (lc *localChain) getSubmittedMovedFundsSweepProofs() []*submittedMovedFundsSweepProof {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.submittedMovedFundsSweepProofs
}

func (lc *localChain) Ready() (bool, error) {
	panic("unsupported")
}

func (lc *localChain) IsAuthorized(address chain.Address) (bool, error) {
	panic("unsupported")
}

func (lc *localChain) IsAuthorizedForRefund(address chain.Address) (
	bool,
	error,
) {
	panic("unsupported")
}

func (lc *localChain) Signing() chain.Signing {
	panic("unsupported")
}

func (lc *localChain) Retarget(headers []*bitcoin.BlockHeader) error {
	panic("unsupported")
}

func (lc *localChain) RetargetWithRefund(headers []*bitcoin.BlockHeader) error {
	panic("unsupported")
}

func (lc *localChain) CurrentEpoch() (uint64, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	return lc.currentEpoch, nil
}

func (lc *localChain) ProofLength() (uint64, error) {
	panic("unsupported")
}

func (lc *localChain) GetCurrentAndPrevEpochDifficulty() (
	*big.Int,
	*big.Int,
	error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.currentEpochDifficulty == nil || lc.previousEpochDifficulty == nil {
		return nil, nil, fmt.Errorf("epoch difficulties not set")
	}

	return lc.currentEpochDifficulty, lc.previousEpochDifficulty, nil
}

func (lc *localChain) setTxProofDifficultyFactor(
	txProofDifficultyFactor *big.Int,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.txProofDifficultyFactor = txProofDifficultyFactor
}

func (lc *localChain) setCurrentEpoch(currentEpoch uint64) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.currentEpoch = currentEpoch
}

func (lc *localChain) setCurrentAndPrevEpochDifficulty(
	previousEpochDifficulty, currentEpochDifficulty *big.Int,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.currentEpochDifficulty = currentEpochDifficulty
	lc.previousEpochDifficulty = previousEpochDifficulty
}

func (lc *localChain) PastDepositRevealedEvents(
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

func (lc *localChain) addPastDepositRevealedEvent(
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
		binary.BigEndian.PutUint64(startBlock, *filter.EndBlock)
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

func (lc *localChain) PastRedemptionRequestedEvents(
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

func (lc *localChain) addPastRedemptionRequestedEvent(
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
		binary.BigEndian.PutUint64(startBlock, *filter.EndBlock)
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

func (lc *localChain) PastMovingFundsCommitmentSubmittedEvents(
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

func (lc *localChain) addPastMovingFundsCommitmentSubmittedEvent(
	filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
	event *tbtc.MovingFundsCommitmentSubmittedEvent,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	eventsKey, err := buildPastMovingFundsCommitmentSubmittedEventsKey(filter)
	if err != nil {
		return err
	}

	lc.pastMovingFundsCommitmentSubmittedEvents[eventsKey] = append(
		lc.pastMovingFundsCommitmentSubmittedEvents[eventsKey],
		event,
	)

	return nil
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
		binary.BigEndian.PutUint64(startBlock, *filter.EndBlock)
		buffer.Write(endBlock)
	}

	for _, walletPublicKeyHash := range filter.WalletPublicKeyHash {
		buffer.Write(walletPublicKeyHash[:])
	}

	return sha256.Sum256(buffer.Bytes()), nil
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

func (lc *localChain) setMovedFundsSweepRequest(
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

func (lc *localChain) GetMovedFundsSweepRequest(
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

type mockBlockCounter struct {
	mutex        sync.Mutex
	currentBlock uint64
}

func newMockBlockCounter() *mockBlockCounter {
	return &mockBlockCounter{}
}

func (mbc *mockBlockCounter) WaitForBlockHeight(blockNumber uint64) error {
	panic("unsupported")
}

func (mbc *mockBlockCounter) BlockHeightWaiter(blockNumber uint64) (
	<-chan uint64,
	error,
) {
	panic("unsupported")
}

func (mbc *mockBlockCounter) CurrentBlock() (uint64, error) {
	mbc.mutex.Lock()
	defer mbc.mutex.Unlock()

	return mbc.currentBlock, nil
}

func (mbc *mockBlockCounter) SetCurrentBlock(block uint64) {
	mbc.mutex.Lock()
	defer mbc.mutex.Unlock()

	mbc.currentBlock = block
}

func (mbc *mockBlockCounter) WatchBlocks(ctx context.Context) <-chan uint64 {
	panic("unsupported")
}

// SubmitReservationProof is a stub matching the reservation additions on
// the production Chain interface.
func (lc *localChain) SubmitReservationProof(
	proofType uint8,
	txInfo *tbtc.BitcoinTxInfo,
	proof *tbtc.BitcoinTxProof,
	mainUtxo *tbtc.BitcoinTxUTXO,
	reservationKey *big.Int,
	requestNonce uint64,
) error {
	if lc.submitReservationProofHook != nil {
		return lc.submitReservationProofHook(
			proofType,
			txInfo,
			proof,
			mainUtxo,
			reservationKey,
			requestNonce,
		)
	}
	panic("unsupported")
}

// NotifyReservationActionTimeout records the notification for assertion in
// tests. The action-timeout watcher builder invokes this through the
// Chain interface to drive the notification path.
func (lc *localChain) NotifyReservationActionTimeout(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.submittedActionTimeouts = append(
		lc.submittedActionTimeouts,
		&submittedReservationActionTimeout{
			reservationKey:   reservationKey,
			walletMembersIDs: walletMembersIDs,
		},
	)

	return lc.notifyReservationActionTimeoutErr
}

// getSubmittedReservationActionTimeouts returns the recorded action-timeout
// notifications in submission order.
func (lc *localChain) getSubmittedReservationActionTimeouts() []*submittedReservationActionTimeout {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	out := make([]*submittedReservationActionTimeout, len(lc.submittedActionTimeouts))
	copy(out, lc.submittedActionTimeouts)
	return out
}

// NotifyStaleReservedDeposit records the notification for assertion in
// tests. The stale-deposit watcher builder invokes this through the
// Chain interface to drive the notification path.
func (lc *localChain) NotifyStaleReservedDeposit(depositKey *big.Int) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.submittedStaleDeposits = append(
		lc.submittedStaleDeposits,
		depositKey,
	)

	return lc.notifyStaleReservedDepositErr
}

// getSubmittedStaleReservedDeposits returns the recorded stale-deposit
// notifications in submission order.
func (lc *localChain) getSubmittedStaleReservedDeposits() []*big.Int {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	out := make([]*big.Int, len(lc.submittedStaleDeposits))
	copy(out, lc.submittedStaleDeposits)
	return out
}

// NotifyReservationStranded records the notification for assertion in
// tests. The stranding watcher builder invokes this through the Chain
// interface to drive the notification path.
func (lc *localChain) NotifyReservationStranded(reservationKey *big.Int) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if err, ok := lc.notifyReservationStrandedErrByKey[reservationKey.String()]; ok {
		return err
	}

	lc.submittedStrandedKeys = append(
		lc.submittedStrandedKeys,
		reservationKey,
	)

	return nil
}

// getSubmittedReservationStrandedKeys returns the recorded stranding
// notifications in submission order.
func (lc *localChain) getSubmittedReservationStrandedKeys() []*big.Int {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	out := make([]*big.Int, len(lc.submittedStrandedKeys))
	copy(out, lc.submittedStrandedKeys)
	return out
}

// GetReservation returns the reservation previously installed via
// setReservation. Returns an error if the reservation is not set, matching
// the production contract behavior.
func (lc *localChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key := bigIntKey(reservationKey)
	reservation, ok := lc.reservations[key]
	if !ok {
		return nil, fmt.Errorf("no reservation for given key")
	}
	return reservation, nil
}

// setReservation installs a reservation for GetReservation to return.
func (lc *localChain) setReservation(
	reservationKey *big.Int,
	reservation *tbtc.Reservation,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservations[bigIntKey(reservationKey)] = reservation
}

// GetReservationAction returns the reservation action previously installed
// via setReservationAction. Returns an error if the action is not set.
func (lc *localChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.getReservationActionErr != nil {
		return nil, lc.getReservationActionErr
	}

	key := buildReservationActionKey(reservationKey, requestNonce)
	action, ok := lc.reservationActions[key]
	if !ok {
		return nil, fmt.Errorf("no action for given reservation/nonce")
	}
	return action, nil
}

// setReservationAction installs a reservation action for GetReservationAction
// to return.
func (lc *localChain) setReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
	action *tbtc.ReservationAction,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationActions[buildReservationActionKey(reservationKey, requestNonce)] = action
}

// buildReservationActionKey produces a map key encoding the reservation
// identifier and the nonce, using the big.Int's full base-16 text
// representation so distinct reservation keys can never collide.
func buildReservationActionKey(
	reservationKey *big.Int,
	requestNonce uint64,
) string {
	return fmt.Sprintf("%s/%d", bigIntKey(reservationKey), requestNonce)
}

// bigIntKey returns a map key string from a big.Int using its full base-16
// text representation, so distinct values can never collide. Returns the
// empty string for nil.
func bigIntKey(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.Text(16)
}

// ReservationParameters returns the reservation parameters previously
// installed via setReservationParameters, or a default set if none was set.
func (lc *localChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.reservationParameters != nil {
		return lc.reservationParameters, nil
	}
	return &tbtc.ReservationParameters{
		ReservationActionTimeout: 3600,
	}, nil
}

// setReservationParameters installs reservation parameters for
// ReservationParameters to return.
func (lc *localChain) setReservationParameters(
	params *tbtc.ReservationParameters,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationParameters = params
}

// WalletReservations returns the reservation keys previously installed via
// setWalletReservations. The slice is a copy so the caller can mutate it
// without affecting the local chain.
func (lc *localChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.walletReservationsErr != nil {
		return nil, lc.walletReservationsErr
	}

	keys := lc.walletReservations[walletPublicKeyHash]
	out := make([]*big.Int, len(keys))
	copy(out, keys)
	return out, nil
}

// setWalletReservations installs the list of reservation keys for a wallet.
func (lc *localChain) setWalletReservations(
	walletPublicKeyHash [20]byte,
	keys []*big.Int,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.walletReservations[walletPublicKeyHash] = append(
		[]*big.Int{},
		keys...,
	)
}

// IsReservedDeposit returns whether the deposit was previously booked via
// setReservedDeposit.
func (lc *localChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.isReservedDepositErr != nil {
		return false, lc.isReservedDepositErr
	}

	record, ok := lc.reservedDeposits[bigIntKey(depositKey)]
	if !ok {
		return false, nil
	}
	return record.isReserved, nil
}

// ReservedDepositWallet returns the wallet previously assigned to the
// deposit via setReservedDeposit.
func (lc *localChain) ReservedDepositWallet(
	depositKey *big.Int,
) ([20]byte, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.reservedDepositWalletErr != nil {
		return [20]byte{}, lc.reservedDepositWalletErr
	}

	record, ok := lc.reservedDeposits[bigIntKey(depositKey)]
	if !ok {
		return [20]byte{}, nil
	}
	return record.walletPublicKeyHash, nil
}

// setReservedDeposit installs the reserved-deposit booking for
// IsReservedDeposit and ReservedDepositWallet to return.
func (lc *localChain) setReservedDeposit(
	depositKey *big.Int,
	walletPublicKeyHash [20]byte,
	isReserved bool,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservedDeposits[bigIntKey(depositKey)] = &reservedDepositRecord{
		walletPublicKeyHash: walletPublicKeyHash,
		isReserved:          isReserved,
	}
}

func (lc *localChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	var result []*tbtc.ReservationAcceptanceRequestedEvent
	for _, event := range lc.reservationAcceptanceRequestedEvents {
		if filter != nil && event.BlockNumber < filter.StartBlock {
			continue
		}
		if filter != nil && len(filter.ReservationKey) > 0 {
			matched := false
			for _, key := range filter.ReservationKey {
				if key.Cmp(event.ReservationKey) == 0 {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, event)
	}

	return result, nil
}

func (lc *localChain) addReservationAcceptanceRequestedEvent(
	event *tbtc.ReservationAcceptanceRequestedEvent,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationAcceptanceRequestedEvents = append(
		lc.reservationAcceptanceRequestedEvents,
		event,
	)
}

func (lc *localChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	var result []*tbtc.ReservationReanchorRequestedEvent
	for _, event := range lc.reservationReanchorRequestedEvents {
		if filter != nil && event.BlockNumber < filter.StartBlock {
			continue
		}
		if filter != nil && len(filter.ReservationKey) > 0 {
			matched := false
			for _, key := range filter.ReservationKey {
				if key.Cmp(event.ReservationKey) == 0 {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, event)
	}

	return result, nil
}

func (lc *localChain) addReservationReanchorRequestedEvent(
	event *tbtc.ReservationReanchorRequestedEvent,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationReanchorRequestedEvents = append(
		lc.reservationReanchorRequestedEvents,
		event,
	)
}

func (lc *localChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	if lc.pastNewWalletRegisteredEventsErr != nil {
		return nil, lc.pastNewWalletRegisteredEventsErr
	}

	var result []*tbtc.NewWalletRegisteredEvent
	for _, event := range lc.newWalletRegisteredEvents {
		if filter != nil && event.BlockNumber < filter.StartBlock {
			continue
		}
		if filter != nil && len(filter.EcdsaWalletID) > 0 {
			matched := false
			for _, id := range filter.EcdsaWalletID {
				if id == event.EcdsaWalletID {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, event)
	}

	return result, nil
}

func (lc *localChain) addNewWalletRegisteredEvent(
	event *tbtc.NewWalletRegisteredEvent,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.newWalletRegisteredEvents = append(lc.newWalletRegisteredEvents, event)
}

func (lc *localChain) setPastNewWalletRegisteredEventsErr(err error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.pastNewWalletRegisteredEventsErr = err
}

// BuildDepositKey is a test-double implementation independent of the
// production keccak256-based algorithm (pkg/chain/ethereum's unexported
// buildDepositKey): only self-consistency within this fake chain matters
// for unit tests, since nothing here cross-checks against a real contract.
func (lc *localChain) BuildDepositKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) *big.Int {
	key := buildDepositRequestKey(fundingTxHash, fundingOutputIndex)
	return new(big.Int).SetBytes(key[:])
}
func TestIsReservedDeposit_PointerIdentity(t *testing.T) {
	spvChain := newLocalChain()

	// Set reserved with one pointer
	key1 := big.NewInt(123)
	wallet := [20]byte{1, 2, 3}
	spvChain.setReservedDeposit(key1, wallet, true)

	// Check reserved with another pointer with same value
	key2 := big.NewInt(123)
	isReserved, err := spvChain.IsReservedDeposit(key2)

	if err != nil {
		t.Fatal(err)
	}
	if !isReserved {
		t.Fatal("expected deposit to be reserved")
	}
}

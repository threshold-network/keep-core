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

// submittedReservationStranded records a NotifyReservationStranded call.
// The stranding watcher builder replaces this stub with the call path
// that records a stray notification for assertion in tests.
type submittedReservationStranded struct {
	reservationKey *big.Int
}

// submittedStaleReservedDeposit records a NotifyStaleReservedDeposit call.
// The stale-deposit watcher builder replaces this stub with the call path
// that records a stale-deposit notification for assertion in tests.
type submittedStaleReservedDeposit struct {
	depositKey *big.Int
}

// submittedReservationActionTimeout records a
// NotifyReservationActionTimeout call. The action-timeout watcher builder
// replaces this stub with the call path that records a timeout notification
// for assertion in tests.
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
	walletReservations               map[[20]byte][]*big.Int
	reservations                     map[[16]byte]*tbtc.Reservation
	reservationActions               map[[24]byte]*tbtc.ReservationAction
	reservedDeposits                 map[[16]byte]*reservedDepositRecord
	submittedStrandedKeys            []*big.Int
	submittedStaleDeposits           []*big.Int
	submittedActionTimeouts          []*submittedReservationActionTimeout
	reservationParameters            *tbtc.ReservationParameters
	reservationReanchorRequestEvents []*tbtc.ReservationReanchorRequestedEvent
	reservationAnchorUtxoIndex       map[[36]byte]*big.Int

	txProofDifficultyFactor    *big.Int
	currentEpoch               uint64
	currentEpochDifficulty     *big.Int
	previousEpochDifficulty    *big.Int
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
		reservations:                             make(map[[16]byte]*tbtc.Reservation),
		reservationActions:                       make(map[[24]byte]*tbtc.ReservationAction),
		reservedDeposits:                         make(map[[16]byte]*reservedDepositRecord),
		submittedStrandedKeys:                    make([]*big.Int, 0),
		submittedStaleDeposits:                   make([]*big.Int, 0),
		submittedActionTimeouts:                  make([]*submittedReservationActionTimeout, 0),
		reservationReanchorRequestEvents:         make([]*tbtc.ReservationReanchorRequestedEvent, 0),
		reservationAnchorUtxoIndex:               make(map[[36]byte]*big.Int),
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
// the production Chain interface. The reservation acceptance and re-anchor
// proposal builders replace this stub with the call path that records a
// submitted proof for assertion in tests.
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

	return nil
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

	return nil
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

	key := bigIntToKey16(reservationKey)
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

	lc.reservations[bigIntToKey16(reservationKey)] = reservation
}

// GetReservationAction returns the reservation action previously installed
// via setReservationAction. Returns an error if the action is not set.
func (lc *localChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

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

// buildReservationActionKey produces a 24-byte map key encoding the
// reservation identifier and the nonce. The reservation identifier is
// truncated to its leading 16 bytes; tests are responsible for choosing
// reservation keys that are unique in those leading bytes.
func buildReservationActionKey(
	reservationKey *big.Int,
	requestNonce uint64,
) [24]byte {
	var out [24]byte
	if reservationKey != nil {
		fillBigInt16(reservationKey, out[:16])
	}
	binary.BigEndian.PutUint64(out[16:24], requestNonce)
	return out
}

// fillBigInt16 writes the leading 16 bytes of the big-endian representation
// of v into dst. The function is allocation-free so tests can use it inside
// hot paths.
func fillBigInt16(v *big.Int, dst []byte) {
	if v == nil {
		return
	}
	bytes := v.Bytes()
	offset := len(dst) - len(bytes)
	if offset < 0 {
		// Truncate to dst size; keeps the trailing high bytes of v.
		bytes = bytes[len(bytes)-len(dst):]
		offset = 0
	}
	for i, b := range bytes {
		dst[offset+i] = b
	}
}

// bigIntToKey16 returns a 16-byte map key from a big.Int by truncating to
// the leading 16 bytes (right-aligned). Returns the zero key for nil.
func bigIntToKey16(v *big.Int) [16]byte {
	var out [16]byte
	if v == nil {
		return out
	}
	fillBigInt16(v, out[:])
	return out
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

// Reservations is a stub matching the reservation additions on the
// production Chain interface. The reservation-side builder replaces this
// stub with the production contract call; the watchers do not need it.
func (lc *localChain) Reservations(
	reservationKey *big.Int,
) (*tbtc.ReservationRequest, error) {
	panic("unsupported")
}

// ReservationActions is a stub matching the reservation additions on the
// production Chain interface. The watchers use GetReservationAction
// instead; this stub exists only to satisfy the interface.
func (lc *localChain) ReservationActions(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationActionRecord, error) {
	panic("unsupported")
}

// IsReservedDeposit returns whether the deposit was previously booked via
// setReservedDeposit.
func (lc *localChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	record, ok := lc.reservedDeposits[bigIntToKey16(depositKey)]
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

	record, ok := lc.reservedDeposits[bigIntToKey16(depositKey)]
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

	lc.reservedDeposits[bigIntToKey16(depositKey)] = &reservedDepositRecord{
		walletPublicKeyHash: walletPublicKeyHash,
		isReserved:          isReserved,
	}
}

func (lc *localChain) PastReservationAcceptedEvents(
	filter *tbtc.ReservationAcceptedEventFilter,
) ([]*tbtc.ReservationAcceptedEvent, error) {
	return nil, nil
}

// submitReservationProofHook, when non-nil, overrides the default panic
// stub and gives the test full control over SubmitReservationProof behavior.
var _ = func() bool {
	_ = bytes.Equal
	return true
}()

func (lc *localChain) PastReservationReanchoredEvents(
	filter *tbtc.ReservationReanchoredEventFilter,
) ([]*tbtc.ReservationReanchoredEvent, error) {
	return nil, nil
}

func (lc *localChain) PastReservationActionTimedOutEvents(
	filter *tbtc.ReservationActionTimedOutEventFilter,
) ([]*tbtc.ReservationActionTimedOutEvent, error) {
	return nil, nil
}

// PastReservationReanchorRequestedEvents returns the events previously
// installed via setReservationReanchorRequestedEvents, ignoring the filter
// (tests install exactly the events they want returned).
func (lc *localChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	out := make([]*tbtc.ReservationReanchorRequestedEvent, len(lc.reservationReanchorRequestEvents))
	copy(out, lc.reservationReanchorRequestEvents)
	return out, nil
}

// setReservationReanchorRequestedEvents installs the events
// PastReservationReanchorRequestedEvents returns.
func (lc *localChain) setReservationReanchorRequestedEvents(
	events []*tbtc.ReservationReanchorRequestedEvent,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationReanchorRequestEvents = events
}

// anchorUtxoIndexKey builds the map key ReservationByAnchorUtxo and
// setReservationByAnchorUtxo use to index a Bitcoin outpoint.
func anchorUtxoIndexKey(txHash [32]byte, outputIndex uint32) [36]byte {
	var key [36]byte
	copy(key[:32], txHash[:])
	binary.BigEndian.PutUint32(key[32:36], outputIndex)
	return key
}

// ReservationByAnchorUtxo returns the reservation key previously installed
// via setReservationByAnchorUtxo for the given outpoint, or zero if none was
// installed - mirroring the production contract's "empty value" semantics
// for an unanchored outpoint rather than returning an error.
func (lc *localChain) ReservationByAnchorUtxo(
	anchorTxHash [32]byte,
	anchorTxOutputIndex uint32,
) (*big.Int, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	key := anchorUtxoIndexKey(anchorTxHash, anchorTxOutputIndex)
	if reservationKey, ok := lc.reservationAnchorUtxoIndex[key]; ok {
		return reservationKey, nil
	}
	return big.NewInt(0), nil
}

// setReservationByAnchorUtxo installs the reservation key
// ReservationByAnchorUtxo returns for the given outpoint.
func (lc *localChain) setReservationByAnchorUtxo(
	anchorTxHash [32]byte,
	anchorTxOutputIndex uint32,
	reservationKey *big.Int,
) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	lc.reservationAnchorUtxoIndex[anchorUtxoIndexKey(anchorTxHash, anchorTxOutputIndex)] = reservationKey
}

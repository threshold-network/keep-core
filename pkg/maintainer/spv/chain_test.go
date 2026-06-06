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

	txProofDifficultyFactor *big.Int
	currentEpoch            uint64
	currentEpochDifficulty  *big.Int
	previousEpochDifficulty *big.Int
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
		pastRedemptionRequestedEvents:            make(map[[32]byte][]*tbtc.RedemptionRequestedEvent),
		pastDepositRevealedEvents:                make(map[[32]byte][]*tbtc.DepositRevealedEvent),
		pastMovingFundsCommitmentSubmittedEvents: make(map[[32]byte][]*tbtc.MovingFundsCommitmentSubmittedEvent),
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

func (lc *localChain) WalletPublicKeyHashForWalletID(
	walletID [32]byte,
) ([20]byte, error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()

	for walletPublicKeyHash, walletChainData := range lc.wallets {
		if walletChainData.WalletID == walletID ||
			walletChainData.EcdsaWalletID == walletID {
			return walletPublicKeyHash, nil
		}
	}

	legacyWalletPublicKeyHash, ok :=
		tbtc.WalletPublicKeyHashFromLegacyWalletID(walletID)
	if ok {
		return legacyWalletPublicKeyHash, nil
	}

	return [20]byte{}, fmt.Errorf("no wallet for given wallet ID")
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

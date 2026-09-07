package bitcoin

import (
	"context"
	"fmt"
	"sync"
)

type localChain struct {
	transactionsMutex sync.Mutex
	transactions      map[Hash]*Transaction

	transactionConfirmationsMutex sync.Mutex
	transactionConfirmations      map[Hash]uint

	merkleProofsMutex sync.Mutex
	merkleProofs      map[Hash]*TransactionMerkleProof

	blockHeadersMutex sync.Mutex
	blockHeaders      map[uint]*BlockHeader

	satPerVByteFeeMutex sync.Mutex
	satPerVByteFee      int64

	coinbaseTxHashesMutex sync.Mutex
	coinbaseTxHashes      map[uint]Hash
}

func newLocalChain() *localChain {
	return &localChain{
		transactions:             make(map[Hash]*Transaction),
		transactionConfirmations: make(map[Hash]uint),
		merkleProofs:             make(map[Hash]*TransactionMerkleProof),
		blockHeaders:             make(map[uint]*BlockHeader),
		coinbaseTxHashes:         make(map[uint]Hash),
	}
}

func (lc *localChain) GetTransaction(
	transactionHash Hash,
) (*Transaction, error) {
	lc.transactionsMutex.Lock()
	defer lc.transactionsMutex.Unlock()

	if transaction, exists := lc.transactions[transactionHash]; exists {
		return transaction, nil
	}

	return nil, fmt.Errorf("transaction not found")
}

func (lc *localChain) BroadcastTransaction(
	transaction *Transaction,
) error {
	panic("not implemented")
}

func (lc *localChain) GetLatestBlockHeight() (uint, error) {
	lc.blockHeadersMutex.Lock()
	defer lc.blockHeadersMutex.Unlock()

	// Return the highest block header's height.
	blockchainTip := uint(0)
	for blockHeaderHeight := range lc.blockHeaders {
		if blockHeaderHeight > blockchainTip {
			blockchainTip = blockHeaderHeight
		}
	}

	if blockchainTip == 0 {
		return 0, fmt.Errorf("block headers not found")
	}

	return blockchainTip, nil
}

func (lc *localChain) GetBlockHeader(
	blockNumber uint,
) (*BlockHeader, error) {
	lc.blockHeadersMutex.Lock()
	defer lc.blockHeadersMutex.Unlock()

	if blockHeader, exists := lc.blockHeaders[blockNumber]; exists {
		return blockHeader, nil
	}

	return nil, fmt.Errorf("block header not found")
}

func (lc *localChain) GetTransactionMerkleProof(
	transactionHash Hash,
	blockHeight uint,
) (*TransactionMerkleProof, error) {
	lc.merkleProofsMutex.Lock()
	defer lc.merkleProofsMutex.Unlock()

	if merkleProof, exists := lc.merkleProofs[transactionHash]; exists {
		return merkleProof, nil
	}

	return nil, fmt.Errorf("transaction not found")
}

func (lc *localChain) GetTransactionConfirmations(
	_ context.Context,
	transactionHash Hash,
) (uint, error) {
	lc.transactionConfirmationsMutex.Lock()
	defer lc.transactionConfirmationsMutex.Unlock()

	if transactionConfirmations, exists := lc.transactionConfirmations[transactionHash]; exists {
		return transactionConfirmations, nil
	}

	return 0, fmt.Errorf("transaction not found")
}

func (lc *localChain) GetTransactionsForPublicKeyHash(
	publicKeyHash [20]byte,
	limit int,
) ([]*Transaction, error) {
	panic("not implemented")
}

func (lc *localChain) GetTxHashesForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]Hash, error) {
	panic("unsupported")
}

func (lc *localChain) GetMempoolForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*Transaction, error) {
	panic("not implemented")
}

func (lc *localChain) GetUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*UnspentTransactionOutput, error) {
	panic("unsupported")
}

func (lc *localChain) GetMempoolUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*UnspentTransactionOutput, error) {
	panic("unsupported")
}

func (lc *localChain) EstimateSatPerVByteFee(
	blocks uint32,
) (int64, error) {
	lc.satPerVByteFeeMutex.Lock()
	defer lc.satPerVByteFeeMutex.Unlock()

	return lc.satPerVByteFee, nil
}

func (lc *localChain) GetCoinbaseTxHash(blockHeight uint) (
	Hash,
	error,
) {
	lc.coinbaseTxHashesMutex.Lock()
	defer lc.coinbaseTxHashesMutex.Unlock()

	coinbaseTxHash, ok := lc.coinbaseTxHashes[blockHeight]
	if !ok {
		return Hash{}, fmt.Errorf("coinbase tx hash not found")
	}

	return coinbaseTxHash, nil
}

func (lc *localChain) setCoinbaseTxHash(blockHeight uint, hash Hash) {
	lc.coinbaseTxHashesMutex.Lock()
	defer lc.coinbaseTxHashesMutex.Unlock()

	lc.coinbaseTxHashes[blockHeight] = hash
}

func (lc *localChain) setSatPerVByteFee(
	satPerVByteFee int64,
) {
	lc.satPerVByteFeeMutex.Lock()
	defer lc.satPerVByteFeeMutex.Unlock()

	lc.satPerVByteFee = satPerVByteFee
}

func (lc *localChain) addTransaction(
	transaction *Transaction,
) error {
	lc.transactionsMutex.Lock()
	defer lc.transactionsMutex.Unlock()

	transactionHash := transaction.Hash()

	if _, exists := lc.transactions[transactionHash]; exists {
		return fmt.Errorf("transaction already exists")
	}

	lc.transactions[transactionHash] = transaction

	return nil
}

func (lc *localChain) addTransactionConfirmations(
	transactionHash Hash,
	transactionConfirmations uint,
) error {
	lc.transactionConfirmationsMutex.Lock()
	defer lc.transactionConfirmationsMutex.Unlock()

	if _, exists := lc.transactionConfirmations[transactionHash]; exists {
		return fmt.Errorf("transaction confirmations already set")
	}

	lc.transactionConfirmations[transactionHash] = transactionConfirmations

	return nil
}

func (lc *localChain) addTransactionMerkleProof(
	transactionHash Hash,
	merkleProof *TransactionMerkleProof,
) error {
	lc.merkleProofsMutex.Lock()
	defer lc.merkleProofsMutex.Unlock()

	if _, exists := lc.merkleProofs[transactionHash]; exists {
		return fmt.Errorf("merkle proof already set")
	}

	lc.merkleProofs[transactionHash] = merkleProof

	return nil
}

func (lc *localChain) addBlockHeader(
	blockNumber uint,
	blockHeader *BlockHeader,
) error {
	lc.blockHeadersMutex.Lock()
	defer lc.blockHeadersMutex.Unlock()

	if _, exists := lc.blockHeaders[blockNumber]; exists {
		return fmt.Errorf("block header already exists")
	}

	lc.blockHeaders[blockNumber] = blockHeader

	return nil
}

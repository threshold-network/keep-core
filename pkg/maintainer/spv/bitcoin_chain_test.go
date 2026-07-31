package spv

import (
	"bytes"
	"fmt"
	"math"
	"sync"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

type localBitcoinChain struct {
	mutex sync.Mutex

	transactions             []*bitcoin.Transaction
	transactionConfirmations map[bitcoin.Hash]uint
	blockHeaders             map[uint]*bitcoin.BlockHeader
}

func newLocalBitcoinChain() *localBitcoinChain {
	return &localBitcoinChain{
		transactions:             make([]*bitcoin.Transaction, 0),
		transactionConfirmations: make(map[bitcoin.Hash]uint),
		blockHeaders:             make(map[uint]*bitcoin.BlockHeader),
	}
}

func (lbc *localBitcoinChain) GetTransaction(transactionHash bitcoin.Hash) (
	*bitcoin.Transaction,
	error,
) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	for _, transaction := range lbc.transactions {
		if transaction.Hash() == transactionHash {
			return transaction, nil
		}
	}

	return nil, fmt.Errorf("transaction not found")
}

func (lbc *localBitcoinChain) GetTransactionConfirmations(transactionHash bitcoin.Hash) (
	uint,
	error,
) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	if transactionConfirmations, exists :=
		lbc.transactionConfirmations[transactionHash]; exists {
		return transactionConfirmations, nil
	}

	return 0, fmt.Errorf("transaction not found")
}

func (lbc *localBitcoinChain) BroadcastTransaction(transaction *bitcoin.Transaction) error {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	transactionHash := transaction.Hash()

	for _, existingTransaction := range lbc.transactions {
		if transactionHash == existingTransaction.Hash() {
			return fmt.Errorf("transaction already exists")
		}
	}

	lbc.transactions = append(lbc.transactions, transaction)

	return nil
}

func (lbc *localBitcoinChain) GetLatestBlockHeight() (uint, error) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	// Return the highest block header's height.
	blockchainTip := uint(0)
	for blockHeaderHeight := range lbc.blockHeaders {
		if blockHeaderHeight > blockchainTip {
			blockchainTip = blockHeaderHeight
		}
	}

	if blockchainTip == 0 {
		return 0, fmt.Errorf("block headers not found")
	}

	return blockchainTip, nil
}

func (lbc *localBitcoinChain) GetBlockHeader(blockHeight uint) (
	*bitcoin.BlockHeader,
	error,
) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	if blockHeader, exists := lbc.blockHeaders[blockHeight]; exists {
		return blockHeader, nil
	}

	return nil, fmt.Errorf("block header does not exist")
}

func (lbc *localBitcoinChain) GetTransactionMerkleProof(
	transactionHash bitcoin.Hash,
	blockHeight uint,
) (*bitcoin.TransactionMerkleProof, error) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) GetTransactionsForPublicKeyHash(
	publicKeyHash [20]byte,
	limit int,
) ([]*bitcoin.Transaction, error) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	p2pkh, err := bitcoin.PayToPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, err
	}

	p2wpkh, err := bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, err
	}

	matchingTransactions := make([]*bitcoin.Transaction, 0)

	for _, transaction := range lbc.transactions {
		for _, output := range transaction.Outputs {
			script := output.PublicKeyScript
			if bytes.Equal(script, p2pkh) || bytes.Equal(script, p2wpkh) {
				matchingTransactions = append(matchingTransactions, transaction)
				break
			}
		}
	}

	if len(matchingTransactions) > limit {
		return matchingTransactions[len(matchingTransactions)-limit:], nil
	}

	return matchingTransactions, nil
}

func (lbc *localBitcoinChain) GetTxHashesForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]bitcoin.Hash, error) {
	// Delegate to GetTransactionsForPublicKeyHash with an effectively
	// unbounded limit instead of duplicating its P2PKH/P2WPKH matching loop.
	// Do not hold lbc.mutex here: the delegated call locks it itself, and
	// sync.Mutex is not reentrant.
	transactions, err := lbc.GetTransactionsForPublicKeyHash(
		publicKeyHash,
		math.MaxInt32,
	)
	if err != nil {
		return nil, err
	}

	// Preserves the ascending block-height order GetTransactionsForPublicKeyHash
	// returns, which the tests rely on.
	hashes := make([]bitcoin.Hash, len(transactions))
	for i, transaction := range transactions {
		hashes[i] = transaction.Hash()
	}

	return hashes, nil
}

func (lbc *localBitcoinChain) GetMempoolForPublicKeyHash(publicKeyHash [20]byte) (
	[]*bitcoin.Transaction,
	error,
) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) GetUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) GetMempoolUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) EstimateSatPerVByteFee(blocks uint32) (
	int64,
	error,
) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) GetCoinbaseTxHash(blockHeight uint) (
	bitcoin.Hash,
	error,
) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) addBlockHeader(
	blockNumber uint,
	blockHeader *bitcoin.BlockHeader,
) error {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	if _, exists := lbc.blockHeaders[blockNumber]; exists {
		return fmt.Errorf("block header already exists")
	}

	lbc.blockHeaders[blockNumber] = blockHeader

	return nil
}

func (lbc *localBitcoinChain) addTransactionConfirmations(
	transactionHash bitcoin.Hash,
	transactionConfirmations uint,
) error {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	if _, exists := lbc.transactionConfirmations[transactionHash]; exists {
		return fmt.Errorf("transaction confirmations already set")
	}

	lbc.transactionConfirmations[transactionHash] = transactionConfirmations

	return nil
}

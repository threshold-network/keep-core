package tbtcpg

import (
	"context"
	"fmt"
	"sync"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

type LocalBitcoinChain struct {
	mutex sync.Mutex

	transactions              map[bitcoin.Hash]*bitcoin.Transaction
	transactionsConfirmations map[bitcoin.Hash]uint
	satPerVByteFeeEstimation  map[uint32]int64
	txHashesByPublicKeyHash   map[[20]byte][]bitcoin.Hash
}

func NewLocalBitcoinChain() *LocalBitcoinChain {
	return &LocalBitcoinChain{
		transactions:              make(map[bitcoin.Hash]*bitcoin.Transaction),
		transactionsConfirmations: make(map[bitcoin.Hash]uint),
		satPerVByteFeeEstimation:  make(map[uint32]int64),
		txHashesByPublicKeyHash:   make(map[[20]byte][]bitcoin.Hash),
	}
}

func (lbc *LocalBitcoinChain) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	transaction, ok := lbc.transactions[transactionHash]
	if !ok {
		return nil, fmt.Errorf("transaction not found")
	}
	return transaction, nil
}

func (lbc *LocalBitcoinChain) SetTransaction(
	transactionHash bitcoin.Hash,
	transaction *bitcoin.Transaction,
) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	lbc.transactions[transactionHash] = transaction
}

func (lbc *LocalBitcoinChain) GetTransactionConfirmations(
	_ context.Context,
	transactionHash bitcoin.Hash,
) (uint, error) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	if confirmations, ok := lbc.transactionsConfirmations[transactionHash]; ok {
		return confirmations, nil
	}

	return 0, fmt.Errorf("transaction not found")
}

func (lbc *LocalBitcoinChain) SetTransactionConfirmations(
	transactionHash bitcoin.Hash,
	confirmations uint,
) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	lbc.transactionsConfirmations[transactionHash] = confirmations
}

func (lbc *LocalBitcoinChain) BroadcastTransaction(
	transaction *bitcoin.Transaction,
) error {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) GetLatestBlockHeight() (uint, error) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) GetBlockHeader(
	blockNumber uint,
) (*bitcoin.BlockHeader, error) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) GetTransactionMerkleProof(
	transactionHash bitcoin.Hash,
	blockHeight uint,
) (*bitcoin.TransactionMerkleProof, error) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) GetTransactionsForPublicKeyHash(
	publicKeyHash [20]byte,
	limit int,
) ([]*bitcoin.Transaction, error) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) GetTxHashesForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]bitcoin.Hash, error) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	return lbc.txHashesByPublicKeyHash[publicKeyHash], nil
}

// SetTxHashesForPublicKeyHash wires the wallet's transaction history for
// GetTxHashesForPublicKeyHash, used by tbtc.DetermineWalletMainUtxo to
// locate the wallet's main UTXO among its past transactions.
func (lbc *LocalBitcoinChain) SetTxHashesForPublicKeyHash(
	publicKeyHash [20]byte,
	hashes []bitcoin.Hash,
) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	lbc.txHashesByPublicKeyHash[publicKeyHash] = hashes
}

func (lbc *LocalBitcoinChain) GetMempoolForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.Transaction, error) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) GetUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) GetMempoolUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) EstimateSatPerVByteFee(
	blocks uint32,
) (int64, error) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	return lbc.satPerVByteFeeEstimation[blocks], nil
}

func (lbc *LocalBitcoinChain) GetCoinbaseTxHash(blockHeight uint) (
	bitcoin.Hash,
	error,
) {
	panic("unsupported")
}

func (lbc *LocalBitcoinChain) SetEstimateSatPerVByteFee(
	blocks uint32,
	fee int64,
) {
	lbc.mutex.Lock()
	defer lbc.mutex.Unlock()

	lbc.satPerVByteFeeEstimation[blocks] = fee
}

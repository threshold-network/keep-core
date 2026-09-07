package btcdiff

import (
	"context"
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

var errNoBlocksSet = fmt.Errorf("blockchain does not contain any blocks")

// localBitcoinChain represents a local Bitcoin chain.
type localBitcoinChain struct {
	blockHeaders map[uint]*bitcoin.BlockHeader
}

// GetTransaction gets the transaction with the given transaction hash.
// If the transaction with the given hash was not found on the chain,
// this function returns an error.
func (lbc *localBitcoinChain) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	panic("unsupported")
}

// GetTransactionConfirmations gets the number of confirmations for the
// transaction with the given transaction hash. If the transaction with the
// given hash was not found on the chain, this function returns an error.
func (lbc *localBitcoinChain) GetTransactionConfirmations(
	_ context.Context,
	transactionHash bitcoin.Hash,
) (uint, error) {
	panic("unsupported")
}

// BroadcastTransaction broadcasts the given transaction over the
// network of the Bitcoin chain nodes. If the broadcast action could not be
// done, this function returns an error. This function does not give any
// guarantees regarding transaction mining. The transaction may be mined or
// rejected eventually.
func (lbc *localBitcoinChain) BroadcastTransaction(
	transaction *bitcoin.Transaction,
) error {
	panic("unsupported")
}

// GetLatestBlockHeight gets the height of the latest block (tip). If the
// latest block was not determined, this function returns an error.
func (lbc *localBitcoinChain) GetLatestBlockHeight() (uint, error) {
	blockchainTip := uint(0)
	for blockHeaderHeight := range lbc.blockHeaders {
		if blockHeaderHeight > blockchainTip {
			blockchainTip = blockHeaderHeight
		}
	}

	if blockchainTip == 0 {
		return 0, errNoBlocksSet
	}

	return blockchainTip, nil
}

// GetBlockHeader gets the block header for the given block number. If the
// block with the given number was not found on the chain, this function
// returns an error.
func (lbc *localBitcoinChain) GetBlockHeader(
	blockNumber uint,
) (*bitcoin.BlockHeader, error) {
	blockHeader, found := lbc.blockHeaders[blockNumber]
	if !found {
		return nil, fmt.Errorf(
			"block header at height %v does not exist",
			blockNumber,
		)
	}

	return blockHeader, nil
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
	panic("unsupported")
}

func (lbc *localBitcoinChain) GetTxHashesForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]bitcoin.Hash, error) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) GetMempoolForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.Transaction, error) {
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

// SetBlockHeaders sets internal headers for testing purposes.
func (lbc *localBitcoinChain) SetBlockHeaders(
	blockHeaders map[uint]*bitcoin.BlockHeader,
) {
	lbc.blockHeaders = blockHeaders
}

func (lbc *localBitcoinChain) EstimateSatPerVByteFee(
	blocks uint32,
) (int64, error) {
	panic("unsupported")
}

func (lbc *localBitcoinChain) GetCoinbaseTxHash(blockHeight uint) (
	bitcoin.Hash,
	error,
) {
	panic("unsupported")
}

// connectLocalBitcoinChain connects to the local Bitcoin chain and returns
// a chain handle.
func connectLocalBitcoinChain() *localBitcoinChain {
	return &localBitcoinChain{}
}

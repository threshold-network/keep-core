package bitcoin

import "context"

// Chain defines an interface meant to be used for interaction with the
// Bitcoin chain.
type Chain interface {
	// GetTransaction gets the transaction with the given transaction hash.
	// If the transaction with the given hash was not found on the chain,
	// this function returns an error.
	GetTransaction(transactionHash Hash) (*Transaction, error)

	// GetTransactionConfirmations gets the number of confirmations for the
	// transaction with the given transaction hash. If the transaction with the
	// given hash was not found on the chain, this function returns an error.
	// The provided context bounds the lookup; implementations should abort the
	// underlying call when it is cancelled.
	GetTransactionConfirmations(
		ctx context.Context,
		transactionHash Hash,
	) (uint, error)

	// BroadcastTransaction broadcasts the given transaction over the
	// network of the Bitcoin chain nodes. If the broadcast action could not be
	// done, this function returns an error. This function does not give any
	// guarantees regarding transaction mining. The transaction may be mined or
	// rejected eventually.
	BroadcastTransaction(transaction *Transaction) error

	// GetLatestBlockHeight gets the height of the latest block (tip). If the
	// latest block was not determined, this function returns an error.
	GetLatestBlockHeight() (uint, error)

	// GetBlockHeader gets the block header for the given block height. If the
	// block with the given height was not found on the chain, this function
	// returns an error.
	GetBlockHeader(blockHeight uint) (*BlockHeader, error)

	// GetTransactionMerkleProof gets the Merkle proof for a given transaction.
	// The transaction's hash and the block the transaction was included in the
	// blockchain need to be provided.
	GetTransactionMerkleProof(
		transactionHash Hash,
		blockHeight uint,
	) (*TransactionMerkleProof, error)

	// GetTransactionsForPublicKeyHash gets the confirmed transactions that pays the
	// given public key hash using either a P2PKH or P2WPKH script. The returned
	// transactions are ordered by block height in the ascending order, i.e.
	// the latest transaction is at the end of the list. The returned list does
	// not contain unconfirmed transactions living in the mempool at the moment
	// of request. The returned transactions list can be limited using the
	// `limit` parameter. For example, if `limit` is set to `5`, only the
	// latest five transactions will be returned. Note that taking an unlimited
	// transaction history may be time-consuming as this function fetches
	// complete transactions with all necessary data.
	GetTransactionsForPublicKeyHash(
		publicKeyHash [20]byte,
		limit int,
	) ([]*Transaction, error)

	// GetTxHashesForPublicKeyHash gets hashes of confirmed transactions that pays
	// the given public key hash using either a P2PKH or P2WPKH script. The returned
	// transactions hashes are ordered by block height in the ascending order, i.e.
	// the latest transaction hash is at the end of the list. The returned list does
	// not contain unconfirmed transactions hashes living in the mempool at the
	// moment of request.
	GetTxHashesForPublicKeyHash(
		publicKeyHash [20]byte,
	) ([]Hash, error)

	// GetMempoolForPublicKeyHash gets the unconfirmed mempool transactions
	// that pays the given public key hash using either a P2PKH or P2WPKH script.
	// The returned transactions are in an indefinite order.
	GetMempoolForPublicKeyHash(publicKeyHash [20]byte) ([]*Transaction, error)

	// GetUtxosForPublicKeyHash gets unspent outputs of confirmed transactions that
	// are controlled by the given public key hash (either a P2PKH or P2WPKH script).
	// The returned UTXOs are ordered by block height in the ascending order, i.e.
	// the latest UTXO is at the end of the list. The returned list does not contain
	// unspent outputs of unconfirmed transactions living in the mempool at the
	// moment of request. Outputs used as inputs of confirmed or mempool
	// transactions are not returned as well because they are no longer UTXOs.
	GetUtxosForPublicKeyHash(
		publicKeyHash [20]byte,
	) ([]*UnspentTransactionOutput, error)

	// GetMempoolUtxosForPublicKeyHash gets unspent outputs of unconfirmed transactions
	// that are controlled by the given public key hash (either a P2PKH or P2WPKH script).
	// The returned UTXOs are in an indefinite order. The returned list does not
	// contain unspent outputs of confirmed transactions. Outputs used as inputs of
	// confirmed or mempool transactions are not returned as well because they are
	// no longer UTXOs.
	GetMempoolUtxosForPublicKeyHash(
		publicKeyHash [20]byte,
	) ([]*UnspentTransactionOutput, error)

	// EstimateSatPerVByteFee returns the estimated sat/vbyte fee for a
	// transaction to be confirmed within the given number of blocks.
	EstimateSatPerVByteFee(blocks uint32) (int64, error)

	// GetCoinbaseTxHash gets the hash of the coinbase transaction for the given
	// block height.
	GetCoinbaseTxHash(blockHeight uint) (Hash, error)
}

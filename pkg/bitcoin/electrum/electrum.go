package electrum

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/checksum0/go-electrum/electrum"
	"github.com/ipfs/go-log"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/keep-network/keep-common/pkg/wrappers"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/internal/byteutils"
)

var (
	supportedProtocolVersions = []string{"1.4"}
	logger                    = log.Logger("keep-electrum")
)

// Connection is a handle for interactions with Electrum server.
type Connection struct {
	parentCtx   context.Context
	client      *electrum.Client
	clientMutex *sync.RWMutex
	config      Config
}

// Connect initializes handle with provided Config.
func Connect(parentCtx context.Context, config Config) (bitcoin.Chain, error) {
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = DefaultConnectTimeout
	}
	if config.ConnectRetryTimeout == 0 {
		config.ConnectRetryTimeout = DefaultConnectRetryTimeout
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = DefaultRequestTimeout
	}
	if config.RequestRetryTimeout == 0 {
		config.RequestRetryTimeout = DefaultRequestRetryTimeout
	}
	if config.KeepAliveInterval == 0 {
		config.KeepAliveInterval = DefaultKeepAliveInterval
	}

	c := &Connection{
		parentCtx:   parentCtx,
		config:      config,
		clientMutex: &sync.RWMutex{},
	}

	if err := c.electrumConnect(); err != nil {
		return nil, fmt.Errorf("failed to initialize electrum client: [%w]", err)
	}

	if err := c.verifyServer(); err != nil {
		return nil, fmt.Errorf("failed to verify electrum server: [%w]", err)
	}

	// Keep the connection alive and check the connection health.
	go c.keepAlive()

	// TODO: Add reconnects on lost connection.

	return c, nil
}

// GetTransaction gets the transaction with the given transaction hash.
// If the transaction with the given hash was not found on the chain,
// this function returns an error.
func (c *Connection) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	txID := transactionHash.Hex(bitcoin.ReversedByteOrder)

	rawTransaction, err := requestWithRetry(
		c,
		func(ctx context.Context, client *electrum.Client) (string, error) {
			// We cannot use `GetTransaction` to get the the transaction details
			// as Esplora/Electrs doesn't support verbose transactions.
			// See: https://github.com/Blockstream/electrs/pull/36
			tx, err := client.GetRawTransaction(ctx, txID)
			if err != nil {
				if isTxNotFoundErr(err) {
					// The transaction was not found on the chain. There is
					// no point in retrying the request and losing time.
					return "", nil
				}

				return "", err
			}

			return tx, nil
		},
		"GetRawTransaction",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get raw transaction with ID [%s]: [%w]",
			txID,
			err,
		)
	}
	if len(rawTransaction) == 0 {
		return nil, fmt.Errorf(
			"failed to get raw transaction with ID [%s]: [%v]",
			txID,
			fmt.Errorf("not found"),
		)
	}

	result, err := convertRawTransaction(rawTransaction)
	if err != nil {
		return nil, fmt.Errorf("failed to convert transaction: [%w]", err)
	}

	return result, nil
}

// GetTransactionConfirmations gets the number of confirmations for the
// transaction with the given transaction hash. If the transaction with the
// given hash was not found on the chain, this function returns an error.
func (c *Connection) GetTransactionConfirmations(
	transactionHash bitcoin.Hash,
) (uint, error) {
	txID := transactionHash.Hex(bitcoin.ReversedByteOrder)

	txLogger := logger.With(
		zap.String("txID", txID),
	)

	rawTransaction, err := requestWithRetry(
		c,
		func(ctx context.Context, client *electrum.Client) (string, error) {
			// We cannot use `GetTransaction` to get the transaction details
			// as Esplora/Electrs doesn't support verbose transactions.
			// See: https://github.com/Blockstream/electrs/pull/36
			tx, err := client.GetRawTransaction(ctx, txID)
			if err != nil {
				if isTxNotFoundErr(err) {
					// The transaction was not found on the chain. There is
					// no point in retrying the request and losing time.
					return "", nil
				}

				return "", err
			}

			return tx, nil
		},
		"GetRawTransaction",
	)
	if err != nil {
		return 0,
			fmt.Errorf(
				"failed to get raw transaction with ID [%s]: [%w]",
				txID,
				err,
			)
	}
	if len(rawTransaction) == 0 {
		return 0, fmt.Errorf(
			"failed to get raw transaction with ID [%s]: [%v]",
			txID,
			fmt.Errorf("not found"),
		)
	}

	tx, err := decodeTransaction(rawTransaction)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to decode the transaction [%s]: [%w]",
			rawTransaction,
			err,
		)
	}

	// As a workaround for the problem described in https://github.com/Blockstream/electrs/pull/36
	// we need to calculate the number of confirmations based on the latest
	// block height and block height of the transaction.
	// Electrum protocol doesn't expose a function to get the transaction's block
	// height (other that the `GetTransaction` that is unsupported by Esplora/Electrs).
	// To get the block height of the transaction we query the history of transactions
	// for the output script hash, as the history contains the transaction's block
	// height.

	// Initialize txBlockHeigh with minimum int32 value to identify a problem when
	// a block height was not found in a history of any of the script hashes.
	//
	// The history is expected to return a block height for confirmed transaction.
	// If a transaction is unconfirmed (is still in the mempool) the height will
	// have a value of `0` or `-1`.
	txBlockHeight := int32(math.MinInt32)
txOutLoop:
	for _, txOut := range tx.TxOut {
		script := txOut.PkScript
		scriptHash := sha256.Sum256(script)
		reversedScriptHash := byteutils.Reverse(scriptHash[:])
		reversedScriptHashString := hex.EncodeToString(reversedScriptHash)

		scriptHashHistory, err := requestWithRetry(
			c,
			func(
				ctx context.Context,
				client *electrum.Client,
			) ([]*electrum.GetMempoolResult, error) {
				return client.GetHistory(ctx, reversedScriptHashString)
			},
			"GetHistory",
		)
		if err != nil {
			// Don't return an error, but continue to the next TxOut entry.
			txLogger.Errorf("failed to get history for script hash: [%v]", err)
			continue txOutLoop
		}

		for _, transaction := range scriptHashHistory {
			if transaction.Hash == txID {
				txBlockHeight = transaction.Height
				break txOutLoop
			}
		}
	}

	// History querying didn't come up with the transaction's block height. Return
	// an error.
	if txBlockHeight == math.MinInt32 {
		return 0, fmt.Errorf(
			"failed to find the transaction block height in script hashes' histories",
		)
	}

	// If the block height is greater than `0` the transaction is confirmed.
	if txBlockHeight > 0 {
		latestBlockHeight, err := c.GetLatestBlockHeight()
		if err != nil {
			return 0, fmt.Errorf(
				"failed to get the latest block height: [%w]",
				err,
			)
		}

		if latestBlockHeight >= uint(txBlockHeight) {
			// Add `1` to the calculated difference as if the transaction block
			// height equals the latest block height the transaction is already
			// confirmed, so it has one confirmation.
			return latestBlockHeight - uint(txBlockHeight) + 1, nil
		}
	}

	return 0, nil
}

func isTxNotFoundErr(err error) bool {
	txNotFoundErrs := []string{
		"no such mempool or blockchain transaction",
		"missing transaction",
		"transaction not found",
	}

	errStr := strings.ToLower(err.Error())

	for _, txNotFoundErr := range txNotFoundErrs {
		if strings.Contains(errStr, txNotFoundErr) {
			return true
		}
	}

	return false
}

// BroadcastTransaction broadcasts the given transaction over the
// network of the Bitcoin chain nodes. If the broadcast action could not be
// done, this function returns an error. This function does not give any
// guarantees regarding transaction mining. The transaction may be mined or
// rejected eventually.
func (c *Connection) BroadcastTransaction(
	transaction *bitcoin.Transaction,
) error {
	rawTx := hex.EncodeToString(transaction.Serialize())

	rawTxLogger := logger.With(
		zap.String("rawTx", rawTx),
	)
	rawTxLogger.Debugf("broadcasting transaction")

	var response string

	response, err := requestWithRetry(
		c,
		func(ctx context.Context, client *electrum.Client) (string, error) {
			return client.BroadcastTransaction(ctx, rawTx)
		},
		"BroadcastTransaction",
	)
	if err != nil {
		return fmt.Errorf("failed to broadcast the transaction: [%w]", err)
	}

	rawTxLogger.Infof("transaction broadcast successful: [%s]", response)

	return nil
}

// GetLatestBlockHeight gets the height of the latest block (tip). If the
// latest block was not determined, this function returns an error.
func (c *Connection) GetLatestBlockHeight() (uint, error) {
	blockHeight, err := requestWithRetry(
		c,
		func(ctx context.Context, client *electrum.Client) (int32, error) {
			tip, err := client.SubscribeHeadersSingle(ctx)
			if err != nil {
				return 0, fmt.Errorf("failed to get the blocks tip height: [%w]", err)
			}

			return tip.Height, nil
		},
		"SubscribeHeaders",
	)
	if err != nil {
		return 0, fmt.Errorf("failed to subscribe for headers: [%w]", err)
	}

	if blockHeight > 0 {
		return uint(blockHeight), nil
	}

	return 0, nil
}

// GetBlockHeader gets the block header for the given block height. If the
// block with the given height was not found on the chain, this function
// returns an error.
func (c *Connection) GetBlockHeader(
	blockHeight uint,
) (*bitcoin.BlockHeader, error) {
	getBlockHeaderResult, err := requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrum.Client,
		) (*electrum.GetBlockHeaderResult, error) {
			return client.GetBlockHeader(ctx, uint32(blockHeight), 0)
		},
		"GetBlockHeader",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header: [%w]", err)
	}

	blockHeader, err := convertBlockHeader(getBlockHeaderResult)
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header: %w", err)
	}

	return blockHeader, nil
}

// GetTransactionMerkleProof gets the Merkle proof for a given transaction.
// The transaction's hash and the block the transaction was included in the
// blockchain need to be provided.
func (c *Connection) GetTransactionMerkleProof(
	transactionHash bitcoin.Hash,
	blockHeight uint,
) (*bitcoin.TransactionMerkleProof, error) {
	txID := transactionHash.Hex(bitcoin.ReversedByteOrder)

	getMerkleProofResult, err := requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrum.Client,
		) (*electrum.GetMerkleProofResult, error) {
			return client.GetMerkleProof(
				ctx,
				txID,
				uint32(blockHeight),
			)
		},
		"GetMerkleProof",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get merkle proof: [%w]", err)
	}

	return convertMerkleProof(getMerkleProofResult), nil
}

// GetTransactionsForPublicKeyHash gets confirmed transactions that pays the
// given public key hash using either a P2PKH or P2WPKH script. The returned
// transactions are ordered by block height in the ascending order, i.e.
// the latest transaction is at the end of the list. The returned list does
// not contain unconfirmed transactions living in the mempool at the moment
// of request. The returned transactions list can be limited using the
// `limit` parameter. For example, if `limit` is set to `5`, only the
// latest five transactions will be returned. Note that taking an unlimited
// transaction history may be time-consuming as this function fetches
// complete transactions with all necessary data.
func (c *Connection) GetTransactionsForPublicKeyHash(
	publicKeyHash [20]byte,
	limit int,
) ([]*bitcoin.Transaction, error) {
	txHashes, err := c.GetTxHashesForPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, err
	}

	selectedTxHashes := selectLatestUniqueTxHashes(txHashes, limit)

	transactions := make([]*bitcoin.Transaction, len(selectedTxHashes))
	for i, txHash := range selectedTxHashes {
		transaction, err := c.GetTransaction(txHash)
		if err != nil {
			return nil, fmt.Errorf("cannot get transaction: [%v]", err)
		}

		transactions[i] = transaction
	}

	return transactions, nil
}

// GetTransactionsForPublicKeyScripts gets confirmed transactions that pay to
// any of the given public key scripts. The returned transactions are ordered
// by block height in ascending order, i.e. the latest transaction is at the
// end of the list.
func (c *Connection) GetTransactionsForPublicKeyScripts(
	publicKeyScripts []bitcoin.Script,
	limit int,
) ([]*bitcoin.Transaction, error) {
	txHashes, err := c.GetTxHashesForPublicKeyScripts(publicKeyScripts)
	if err != nil {
		return nil, err
	}

	selectedTxHashes := selectLatestUniqueTxHashes(txHashes, limit)

	transactions := make([]*bitcoin.Transaction, len(selectedTxHashes))
	for i, txHash := range selectedTxHashes {
		transaction, err := c.GetTransaction(txHash)
		if err != nil {
			return nil, fmt.Errorf("cannot get transaction: [%v]", err)
		}

		transactions[i] = transaction
	}

	return transactions, nil
}

// selectLatestUniqueTxHashes deduplicates the given transaction hashes,
// preserving their original (block-height ascending) order, and returns at
// most limit trailing entries, i.e. the latest ones. Deduplication happens
// before the limit is applied, so a transaction paying two of the queried
// scripts consumes a single slot.
//
// A limit lower than or equal to zero returns an empty slice. The SPV
// maintainer's transaction limit is an operator-provided flag, so the guard
// keeps a misconfigured negative value from panicking on the slice bound; it
// makes the maintainer observe no transactions rather than crash.
func selectLatestUniqueTxHashes(
	txHashes []bitcoin.Hash,
	limit int,
) []bitcoin.Hash {
	if limit <= 0 {
		return []bitcoin.Hash{}
	}
	uniqueTxHashes := make([]bitcoin.Hash, 0, len(txHashes))
	seen := make(map[bitcoin.Hash]bool)
	for _, txHash := range txHashes {
		if seen[txHash] {
			continue
		}

		seen[txHash] = true
		uniqueTxHashes = append(uniqueTxHashes, txHash)
	}

	if len(uniqueTxHashes) > limit {
		return uniqueTxHashes[len(uniqueTxHashes)-limit:]
	}

	return uniqueTxHashes
}

// GetTxHashesForPublicKeyHash gets hashes of confirmed transactions that pays
// the given public key hash using either a P2PKH or P2WPKH script. The returned
// transactions hashes are ordered by block height in the ascending order, i.e.
// the latest transaction hash is at the end of the list. The returned list does
// not contain unconfirmed transactions hashes living in the mempool at the
// moment of request.
func (c *Connection) GetTxHashesForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]bitcoin.Hash, error) {
	p2pkh, err := bitcoin.PayToPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2PKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkh, err := bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2WPKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	return c.GetTxHashesForPublicKeyScripts([]bitcoin.Script{p2pkh, p2wpkh})
}

// GetTxHashesForPublicKeyScripts gets hashes of confirmed transactions that
// pay to any of the given public key scripts. The returned transactions
// hashes are ordered by block height in the ascending order, i.e. the
// latest transaction hash is at the end of the list.
func (c *Connection) GetTxHashesForPublicKeyScripts(
	publicKeyScripts []bitcoin.Script,
) ([]bitcoin.Hash, error) {
	items, err := c.getConfirmedScriptHistories(publicKeyScripts)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(
		items,
		func(i, j int) bool {
			return items[i].blockHeight < items[j].blockHeight
		},
	)

	txHashes := make([]bitcoin.Hash, len(items))
	for i, item := range items {
		txHashes[i] = item.txHash
	}

	return txHashes, nil
}

func (c *Connection) getConfirmedScriptHistories(
	publicKeyScripts []bitcoin.Script,
) ([]*scriptHistoryItem, error) {
	items := make([]*scriptHistoryItem, 0)

	for _, publicKeyScript := range publicKeyScripts {
		scriptItems, err := c.getConfirmedScriptHistory(publicKeyScript)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get history for script [0x%x]: [%v]",
				publicKeyScript,
				err,
			)
		}

		items = append(items, scriptItems...)
	}

	return items, nil
}

type scriptHistoryItem struct {
	txHash      bitcoin.Hash
	blockHeight int32
}

// getConfirmedScriptHistory returns a history of confirmed transactions for
// the given script (P2PKH, P2WPKH, P2SH, P2WSH, etc.). The returned list
// is sorted by the block height in the ascending order, i.e. the latest
// transaction is at the end of the list. The resulting list does not contain
// unconfirmed transactions living in the mempool at the moment of request.
func (c *Connection) getConfirmedScriptHistory(
	script []byte,
) ([]*scriptHistoryItem, error) {
	scriptHash := sha256.Sum256(script)
	reversedScriptHash := byteutils.Reverse(scriptHash[:])
	reversedScriptHashString := hex.EncodeToString(reversedScriptHash)

	items, err := requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrum.Client,
		) ([]*electrum.GetMempoolResult, error) {
			return client.GetHistory(ctx, reversedScriptHashString)
		},
		"GetHistory",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get history for script [0x%x]: [%v]",
			script,
			err,
		)
	}

	// According to https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-scripthash-get-history
	// unconfirmed items living in the mempool are appended at the end of the
	// returned list and their height value is either -1 or 0. That means
	// we need to take all items with height >0 to obtain a confirmed txs
	// history.
	confirmedItems := make([]*scriptHistoryItem, 0)
	for _, item := range items {
		if item.Height > 0 {
			txHash, err := bitcoin.NewHashFromString(
				item.Hash,
				bitcoin.ReversedByteOrder,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot parse hash [%s]: [%v]",
					item.Hash,
					err,
				)
			}

			confirmedItems = append(
				confirmedItems, &scriptHistoryItem{
					txHash:      txHash,
					blockHeight: item.Height,
				},
			)
		}
	}

	// The list returned from client.GetHistory is sorted by the block height
	// in the ascending order though we are sorting it again just in case
	// (e.g. API contract changes).
	sort.SliceStable(
		confirmedItems,
		func(i, j int) bool {
			return confirmedItems[i].blockHeight < confirmedItems[j].blockHeight
		},
	)

	return confirmedItems, nil
}

// GetCoinbaseTxHash gets the hash of the coinbase transaction for the given
// block height.
func (c *Connection) GetCoinbaseTxHash(blockHeight uint) (bitcoin.Hash, error) {
	txHashString, err := requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrum.Client,
		) (string, error) {
			return client.GetHashFromPosition(ctx, uint32(blockHeight), 0)
		},
		"GetHashFromPosition",
	)
	if err != nil {
		return bitcoin.Hash{}, fmt.Errorf(
			"failed to get coinbase tx hash for block height [%v]: [%v]",
			blockHeight,
			err,
		)
	}

	txHash, err := bitcoin.NewHashFromString(
		txHashString,
		bitcoin.ReversedByteOrder,
	)
	if err != nil {
		return bitcoin.Hash{}, fmt.Errorf(
			"cannot parse hash [%s]: [%v]",
			txHashString,
			err,
		)
	}

	return txHash, nil
}

// GetMempoolForPublicKeyHash gets the unconfirmed mempool transactions
// that pays the given public key hash using either a P2PKH or P2WPKH script.
// The returned transactions are in an indefinite order.
func (c *Connection) GetMempoolForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.Transaction, error) {
	p2pkh, err := bitcoin.PayToPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2PKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkh, err := bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2WPKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2pkhItems, err := c.getScriptMempool(p2pkh)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2PKH mempool items for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkhItems, err := c.getScriptMempool(p2wpkh)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2WPKH mempool items for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	items := append(p2pkhItems, p2wpkhItems...)

	transactions := make([]*bitcoin.Transaction, len(items))
	for i, item := range items {
		transaction, err := c.GetTransaction(item.txHash)
		if err != nil {
			return nil, fmt.Errorf("cannot get transaction: [%v]", err)
		}

		transactions[i] = transaction
	}

	return transactions, nil
}

type scriptMempoolItem struct {
	txHash      bitcoin.Hash
	blockHeight int32
	fee         uint32
}

// getScriptMempool returns unconfirmed mempool transactions for
// the given script (P2PKH, P2WPKH, P2SH, P2WSH, etc.). The returned list
// is in an indefinite order.
func (c *Connection) getScriptMempool(
	script []byte,
) ([]*scriptMempoolItem, error) {
	scriptHash := sha256.Sum256(script)
	reversedScriptHash := byteutils.Reverse(scriptHash[:])
	reversedScriptHashString := hex.EncodeToString(reversedScriptHash)

	items, err := requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrum.Client,
		) ([]*electrum.GetMempoolResult, error) {
			return client.GetMempool(ctx, reversedScriptHashString)
		},
		"GetMempool",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get mempool for script [0x%x]: [%v]",
			script,
			err,
		)
	}

	convertedItems := make([]*scriptMempoolItem, len(items))
	for i, item := range items {
		txHash, err := bitcoin.NewHashFromString(
			item.Hash,
			bitcoin.ReversedByteOrder,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse hash [%s]: [%v]",
				item.Hash,
				err,
			)
		}

		convertedItems[i] = &scriptMempoolItem{
			txHash:      txHash,
			blockHeight: item.Height,
			fee:         item.Fee,
		}
	}

	return convertedItems, nil
}

// GetUtxosForPublicKeyHash gets unspent outputs of confirmed transactions that
// are controlled by the given public key hash (either a P2PKH or P2WPKH script).
// The returned UTXOs are ordered by block height in the ascending order, i.e.
// the latest UTXO is at the end of the list. The returned list does not contain
// unspent outputs of unconfirmed transactions living in the mempool at the
// moment of request. Outputs used as inputs of confirmed or mempool
// transactions are not returned as well because they are no longer UTXOs.
func (c *Connection) GetUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	p2pkh, err := bitcoin.PayToPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2PKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkh, err := bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2WPKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	return c.GetUtxosForPublicKeyScripts([]bitcoin.Script{p2pkh, p2wpkh})
}

// GetUtxosForPublicKeyScripts gets unspent outputs of confirmed transactions
// that are controlled by any of the given public key scripts.
func (c *Connection) GetUtxosForPublicKeyScripts(
	publicKeyScripts []bitcoin.Script,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	return c.getUtxosForPublicKeyScripts(publicKeyScripts, true)
}

// GetMempoolUtxosForPublicKeyHash gets unspent outputs of unconfirmed transactions
// that are controlled by the given public key hash (either a P2PKH or P2WPKH script).
// The returned UTXOs are in an indefinite order. The returned list does not
// contain unspent outputs of confirmed transactions. Outputs used as inputs of
// confirmed or mempool transactions are not returned as well because they are
// no longer UTXOs.
func (c *Connection) GetMempoolUtxosForPublicKeyHash(
	publicKeyHash [20]byte,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	p2pkh, err := bitcoin.PayToPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2PKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkh, err := bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot build P2WPKH for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	return c.GetMempoolUtxosForPublicKeyScripts([]bitcoin.Script{p2pkh, p2wpkh})
}

// GetMempoolUtxosForPublicKeyScripts gets unspent outputs of unconfirmed
// transactions that are controlled by any of the given public key scripts.
func (c *Connection) GetMempoolUtxosForPublicKeyScripts(
	publicKeyScripts []bitcoin.Script,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	return c.getUtxosForPublicKeyScripts(publicKeyScripts, false)
}

func (c *Connection) getUtxosForPublicKeyScripts(
	publicKeyScripts []bitcoin.Script,
	confirmed bool,
) ([]*bitcoin.UnspentTransactionOutput, error) {
	items, err := c.getScriptUtxosForScripts(publicKeyScripts, confirmed)
	if err != nil {
		return nil, err
	}

	if confirmed {
		sort.SliceStable(
			items,
			func(i, j int) bool {
				return items[i].blockHeight < items[j].blockHeight
			},
		)
	}

	utxos := make([]*bitcoin.UnspentTransactionOutput, len(items))
	for i, item := range items {
		utxos[i] = &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: item.txHash,
				OutputIndex:     item.outputIndex,
			},
			Value: int64(item.value),
		}
	}

	return utxos, nil
}

func (c *Connection) getScriptUtxosForScripts(
	publicKeyScripts []bitcoin.Script,
	confirmed bool,
) ([]*scriptUtxoItem, error) {
	items := make([]*scriptUtxoItem, 0)
	seen := make(map[bitcoin.TransactionOutpoint]bool)

	for _, publicKeyScript := range publicKeyScripts {
		scriptItems, err := c.getScriptUtxos(publicKeyScript, confirmed)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get UTXOs for script [0x%x]: [%v]",
				publicKeyScript,
				err,
			)
		}

		for _, item := range scriptItems {
			outpoint := bitcoin.TransactionOutpoint{
				TransactionHash: item.txHash,
				OutputIndex:     item.outputIndex,
			}
			if seen[outpoint] {
				continue
			}
			seen[outpoint] = true
			items = append(items, item)
		}
	}

	return items, nil
}

type scriptUtxoItem struct {
	txHash      bitcoin.Hash
	outputIndex uint32
	value       uint64
	blockHeight uint32
}

// getScriptUtxos returns unspent outputs of confirmed/unconfirmed transactions
// that are locked using the given script (P2PKH, P2WPKH, P2SH, P2WSH, etc.).
//
// If the `confirmed` flag is true, the returned list contains unspent outputs
// of confirmed transactions, sorted by the block height in the ascending order,
// i.e. the latest UTXO is at the end of the list. The resulting list does not
// contain unspent outputs of unconfirmed transactions living in the mempool
// at the moment of request.
//
// If the `confirmed` flag is false, the returned list contains unspent outputs
// of unconfirmed transactions, in an indefinite order. The resulting list
// does not contain unspent outputs of confirmed transactions.
//
// In both cases, the resulted list DOES NOT CONTAIN outputs already used as
// inputs of confirmed or mempool transactions because they are no longer UTXOs.
func (c *Connection) getScriptUtxos(
	script []byte,
	confirmed bool,
) ([]*scriptUtxoItem, error) {
	scriptHash := sha256.Sum256(script)
	reversedScriptHash := byteutils.Reverse(scriptHash[:])
	reversedScriptHashString := hex.EncodeToString(reversedScriptHash)

	items, err := requestWithRetry(
		c,
		func(
			ctx context.Context,
			client *electrum.Client,
		) ([]*electrum.ListUnspentResult, error) {
			return client.ListUnspent(ctx, reversedScriptHashString)
		},
		"ListUnspent",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get UTXOs for script [0x%x]: [%v]",
			script,
			err,
		)
	}

	// According to https://electrumx.readthedocs.io/en/stable/protocol-methods.html#blockchain-scripthash-listunspent
	// unconfirmed items living in the mempool are appended at the end of the
	// returned list and their height value is either -1 or 0. That means
	// we need to take all items with height >0 to obtain confirmed UTXO
	// items and <=0 if we want to take the unconfirmed ones.
	var filterFn func(item *electrum.ListUnspentResult) bool
	if confirmed {
		filterFn = func(item *electrum.ListUnspentResult) bool {
			return item.Height > 0
		}
	} else {
		filterFn = func(item *electrum.ListUnspentResult) bool {
			return item.Height <= 0
		}
	}

	filteredItems := make([]*scriptUtxoItem, 0)
	for _, item := range items {
		if filterFn(item) {
			txHash, err := bitcoin.NewHashFromString(
				item.Hash,
				bitcoin.ReversedByteOrder,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot parse hash [%s]: [%v]",
					item.Hash,
					err,
				)
			}

			filteredItems = append(
				filteredItems, &scriptUtxoItem{
					txHash:      txHash,
					outputIndex: item.Position,
					value:       item.Value,
					blockHeight: item.Height,
				},
			)
		}
	}

	if confirmed {
		// The list returned from client.ListUnspent is sorted by the block height
		// in the ascending order though we are sorting it again just in case
		// (e.g. API contract changes). Sorting makes sense only for confirmed
		// items as unconfirmed ones have a block height of -1 or 0.
		sort.SliceStable(
			filteredItems,
			func(i, j int) bool {
				return filteredItems[i].blockHeight < filteredItems[j].blockHeight
			},
		)
	}

	return filteredItems, nil
}

// feeEstimateWithFallbackTargets returns confirmation targets to try, in order.
// Callers default to 1 block (see bitcoin.TransactionFeeEstimator); on public
// testnets many targets can fail (empty mempool). We try a wide set, then
// optional static fallback in EstimateSatPerVByteFee.
// See electrum_integration_test.go (TestEstimateSatPerVByteFee_Integration).
func feeEstimateWithFallbackTargets(primary uint32) []uint32 {
	seen := make(map[uint32]struct{})
	var out []uint32
	add := func(u uint32) {
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	add(primary)
	for _, fb := range []uint32{6, 25, 50, 100, 144, 500, 1008} {
		add(fb)
	}
	return out
}

// defaultFallbackSatPerVByteWhenEstimateFails is used when Electrum cannot
// return a fee for any confirmation target (typical on testnet4 / quiet
// mempools: -32603 for all N). It is only applied on test networks (see
// lowFeeFallbackAllowed): on mainnet a fixed low feerate can leave a
// transaction unconfirmable or evicted under congestion, so the oracle failure
// is surfaced as an error (fail-safe) rather than broadcast at a guessed
// feerate.
const defaultFallbackSatPerVByteWhenEstimateFails int64 = 2

// isElectrumFeeOracleFailure reports whether the error is the usual
// "no fee data" / JSON-RPC -32603 from estimatefee, as opposed to transport
// or auth failures where we should not invent a feerate.
func isElectrumFeeOracleFailure(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "cannot estimate fee") ||
		strings.Contains(s, "-32603")
}

// getFeeBtcPerKbOnce issues a single blockchain.estimatefee call (no multi-minute
// retry loop). Persistent RPC errors for one confirmation target should not
// exhaust RequestRetryTimeout; EstimateSatPerVByteFee tries looser targets next.
func (c *Connection) getFeeBtcPerKbOnce(blocks uint32) (float32, error) {
	if err := c.reconnectIfShutdown(); err != nil {
		return 0, err
	}
	requestCtx, requestCancel := context.WithTimeout(
		c.parentCtx,
		c.config.RequestTimeout,
	)
	defer requestCancel()
	fee, err := c.currentClient().GetFee(requestCtx, blocks)
	if err != nil {
		return 0, fmt.Errorf("request failed: [%w]", err)
	}
	return fee, nil
}

// EstimateSatPerVByteFee returns the estimated sat/vbyte fee for a
// transaction to be confirmed within the given number of blocks.
func (c *Connection) EstimateSatPerVByteFee(blocks uint32) (int64, error) {
	targets := feeEstimateWithFallbackTargets(blocks)
	var lastErr error
	sawFeeOracleFailure := false
	sawNonOracleFailure := false

	for _, b := range targets {
		btcPerKbFee, err := c.getFeeBtcPerKbOnce(b)
		if err != nil {
			lastErr = err
			if isElectrumFeeOracleFailure(err) {
				sawFeeOracleFailure = true
			} else {
				sawNonOracleFailure = true
			}
			logger.Debugf("GetFee for [%d] confirmation blocks failed: [%v]", b, err)
			continue
		}
		// According to Electrum protocol docs, if the daemon does not have
		// enough information to make an estimate, the integer -1 is returned.
		if btcPerKbFee < 0 {
			lastErr = fmt.Errorf(
				"daemon does not have enough information to make an estimate",
			)
			sawFeeOracleFailure = true
			logger.Debugf("GetFee for [%d] blocks returned no estimate (fee < 0)", b)
			continue
		}

		if b != blocks {
			logger.Infof(
				"using Electrum fee estimate for [%d] confirmation blocks "+
					"(requested [%d] was unavailable)",
				b,
				blocks,
			)
		}

		return convertBtcKbToSatVByte(btcPerKbFee), nil
	}

	// Only fall back to a fixed low feerate when every failure was a benign
	// "no estimate" response from the fee oracle. If a transport/auth (or any
	// other non-oracle) failure was also observed, we cannot assume the oracle
	// simply lacked data, so we fail safe and surface the error rather than
	// broadcasting at a guessed feerate.
	fallbackEligible := sawFeeOracleFailure && !sawNonOracleFailure

	if sawFeeOracleFailure {
		if fallbackEligible && lowFeeFallbackAllowed(c.config.Network) {
			logger.Warnf(
				"Electrum returned no fee estimate for any target %v on [%v] "+
					"network; using fallback [%d] sat/vbyte (last error: [%v])",
				targets,
				c.config.Network,
				defaultFallbackSatPerVByteWhenEstimateFails,
				lastErr,
			)
		} else {
			logger.Warnf(
				"Electrum returned no usable fee estimate for any target %v on "+
					"[%v] network; low-fee fallback not applicable (mainnet, "+
					"unknown network, or a non-oracle failure occurred), failing "+
					"safe (last error: [%v])",
				targets,
				c.config.Network,
				lastErr,
			)
		}
	}

	return feeFallbackResult(c.config.Network, fallbackEligible, lastErr, targets)
}

// lowFeeFallbackAllowed reports whether the hardcoded low-fee estimate fallback
// is acceptable for the given Bitcoin network. It is permitted only on test
// networks, where an underpriced transaction has no real economic consequence.
// Mainnet and any unset/unrecognized network fail closed, so an oracle failure
// is surfaced as an error rather than broadcast at a guessed feerate.
func lowFeeFallbackAllowed(network bitcoin.Network) bool {
	switch network {
	case bitcoin.Testnet, bitcoin.Testnet4, bitcoin.Regtest:
		return true
	default:
		return false
	}
}

// feeFallbackResult resolves the result of EstimateSatPerVByteFee when no
// Electrum fee estimate could be obtained for any confirmation target. On a
// fee-oracle failure it returns the low-fee fallback only where
// lowFeeFallbackAllowed permits it; otherwise (mainnet, an unset network, or a
// transport-level failure) it returns an error so the caller does not broadcast
// a transaction at a guessed feerate.
func feeFallbackResult(
	network bitcoin.Network,
	sawFeeOracleFailure bool,
	lastErr error,
	targets []uint32,
) (int64, error) {
	if sawFeeOracleFailure && lowFeeFallbackAllowed(network) {
		return defaultFallbackSatPerVByteWhenEstimateFails, nil
	}
	if lastErr != nil {
		return 0, fmt.Errorf("failed to get fee: [%v]", lastErr)
	}
	return 0, fmt.Errorf(
		"failed to get fee from Electrum after trying confirmation targets %v",
		targets,
	)
}

func convertBtcKbToSatVByte(btcPerKbFee float32) int64 {
	// To convert from BTC/KB to sat/vbyte, we need to multiply by 1e8/1e3.
	satPerVByte := (1e8 / 1e3) * float64(btcPerKbFee)
	// Make sure the minimum returned sat/vbyte fee is always 1.
	satPerVByte = math.Max(satPerVByte, 1)
	// Round the returned fee to be an integer.
	return int64(math.Round(satPerVByte))
}

func (c *Connection) electrumConnect() error {
	var client *electrum.Client
	var err error

	logger.Debug("establishing connection to electrum server...")
	client, err = connectWithRetry(
		c,
		func(ctx context.Context) (*electrum.Client, error) {
			return electrum.NewClient(ctx, c.config.URL, nil)
		},
	)

	if err == nil {
		c.client = client
	}

	return err
}

func (c *Connection) verifyServer() error {
	type Server struct {
		version  string
		protocol string
	}

	server, err := requestWithRetry(
		c,
		func(ctx context.Context, client *electrum.Client) (*Server, error) {
			serverVersion, protocolVersion, err := client.ServerVersion(ctx)
			if err != nil {
				return nil, err
			}
			return &Server{serverVersion, protocolVersion}, nil
		},
		"ServerVersion",
	)
	if err != nil {
		return fmt.Errorf("failed to get server version: [%w]", err)
	}

	logger.Infof(
		"connected to electrum server [version: [%s], protocol: [%s]]",
		server.version,
		server.protocol,
	)

	// Log a warning if connected to a server running an unsupported protocol version.
	if !slices.Contains(supportedProtocolVersions, server.protocol) {
		logger.Warnf(
			"electrum server [%s] runs an unsupported protocol version: [%s]; expected one of: [%s]",
			c.config.URL,
			server.protocol,
			strings.Join(supportedProtocolVersions, ","),
		)
	}

	return nil
}

func (c *Connection) keepAlive() {
	ticker := time.NewTicker(c.config.KeepAliveInterval)

	for {
		select {
		case <-ticker.C:
			_, err := requestWithRetry(
				c,
				func(ctx context.Context, client *electrum.Client) (interface{}, error) {
					return nil, client.Ping(ctx)
				},
				"Ping",
			)
			if err != nil {
				logger.Errorf(
					"failed to ping the electrum server; "+
						"please verify health of the electrum server: [%v]",
					err,
				)
			} else {
				// Adjust ticker starting at the time of the latest successful ping.
				ticker = time.NewTicker(c.config.KeepAliveInterval)
			}
		case <-c.parentCtx.Done():
			ticker.Stop()
			c.currentClient().Shutdown()
			return
		}
	}
}

func connectWithRetry(
	c *Connection,
	newClientFn func(ctx context.Context) (*electrum.Client, error),
) (*electrum.Client, error) {
	var result *electrum.Client
	err := wrappers.DoWithDefaultRetry(
		c.parentCtx,
		c.config.ConnectRetryTimeout,
		func(ctx context.Context) error {
			connectCtx, connectCancel := context.WithTimeout(
				ctx,
				c.config.ConnectTimeout,
			)
			defer connectCancel()

			client, err := newClientFn(connectCtx)
			if err == nil {
				result = client
			}

			return err
		},
	)

	return result, err
}

func requestWithRetry[K interface{}](
	c *Connection,
	requestFn func(ctx context.Context, client *electrum.Client) (K, error),
	requestName string,
) (K, error) {
	startTime := time.Now()
	logger.Debugf("starting [%s] request to Electrum server", requestName)

	var result K

	err := wrappers.DoWithDefaultRetry(
		c.parentCtx,
		c.config.RequestRetryTimeout,
		func(ctx context.Context) error {
			if err := c.reconnectIfShutdown(); err != nil {
				return err
			}

			requestCtx, requestCancel := context.WithTimeout(ctx, c.config.RequestTimeout)
			defer requestCancel()

			r, err := requestFn(requestCtx, c.currentClient())

			if err != nil {
				return fmt.Errorf("request failed: [%w]", err)
			}

			result = r
			return nil
		})

	solveRequestOutcome := func(err error) string {
		if err != nil {
			return fmt.Sprintf("error: [%v]", err)
		}
		return "success"
	}

	logger.Debugf("[%s] request to Electrum server completed with [%s] after [%s]",
		requestName,
		solveRequestOutcome(err),
		time.Since(startTime),
	)

	return result, err
}

// currentClient returns the live Electrum client under a read lock.
//
// The lock protects the client POINTER against a concurrent reconnect swap and
// nothing else. It is released before the caller issues its request, so a
// reconnect may swap the client while that request is still in flight against
// the old one.
//
// How such a stale-client request fails depends on the caller. Requests issued
// through requestWithRetry fail and are reconnected and repeated by its retry
// loop -- the same fallback it applies to any other request failure.
// getFeeBtcPerKbOnce calls currentClient directly, outside any retry wrapper,
// and surfaces a plain error to EstimateSatPerVByteFee's fallback loop, which
// moves on to the next confirmation target instead of retrying the same one.
//
// Note that the underlying go-electrum client's Shutdown() is not thread-safe
// against in-flight requests: it clears the transport and handler maps without
// holding the handler lock. That is a known upstream issue tracked separately;
// this lock does not protect against it.
func (c *Connection) currentClient() *electrum.Client {
	c.clientMutex.RLock()
	defer c.clientMutex.RUnlock()

	return c.client
}

// reconnectIfShutdown replaces a shut-down client with a fresh connection. It is
// the only writer of c.client, so it takes the write lock: readers hold the read
// lock just long enough to copy the pointer (see currentClient).
func (c *Connection) reconnectIfShutdown() error {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	isClientShutdown := c.client.IsShutdown()
	if isClientShutdown {
		logger.Warn("connection to electrum server is down; reconnecting...")
		err := c.electrumConnect()
		if err != nil {
			return fmt.Errorf("failed to reconnect to electrum server: [%w]", err)
		}
		logger.Info("reconnected to electrum server")
	}

	return nil
}

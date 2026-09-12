package electrum

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/checksum0/go-electrum/electrum"
	"github.com/ipfs/go-log"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/internal/byteutils"
	"github.com/keep-network/keep-core/pkg/wrappers"
)

var (
	supportedProtocolVersions = []string{"1.4"}
	logger                    = log.Logger("keep-electrum")
)

// watchAbort arranges to call client.Abort if ctx becomes Done and the
// protected operation has not finished on its own within a short grace
// period afterward. The returned stop function must be called immediately
// after the protected operation returns; it signals completion and blocks
// until the internal watcher goroutine has fully resolved one way or the
// other, so no abort can fire after the caller has already moved on.
//
// This avoids a scheduling race inherent to context.AfterFunc(ctx, f): its
// stop() can only reliably prevent f from running if called before the
// runtime actually schedules f's goroutine to execute. Under contention, the
// watcher goroutine can start and complete client.Abort() before the caller
// even reaches its stop() call, even though the protected operation had
// already returned successfully.
func watchAbort(ctx context.Context, client electrumClient) (stop func()) {
	done := make(chan struct{})
	resolved := make(chan struct{})

	go func() {
		defer close(resolved)

		select {
		case <-done:
			return
		case <-ctx.Done():
		}

		select {
		case <-done:
		case <-time.After(5 * time.Millisecond):
			go client.Abort()
		}
	}()

	return func() {
		close(done)
		<-resolved
	}
}

// Connection is a handle for interactions with Electrum server.
type Connection struct {
	parentCtx   context.Context
	client      electrumClient
	closeClient func()
	clientMutex *sync.Mutex
	config      Config
	serverURLs  []string
	serverIndex int
	newClient   func(context.Context, string) (electrumClient, error)
	// verifiedGenesisHash is the first non-empty genesis_hash adopted from
	// a verified candidate. Every later candidate must report the same
	// hash; a candidate may also omit the field (see verifyServer).
	// Accessed only while clientMutex is held.
	verifiedGenesisHash string
	// lastTipHeight is the most recently observed chain tip height, used to
	// sanity-check the tip reported right after a (re)connect. Accessed only
	// while clientMutex is held.
	lastTipHeight uint
	// pendingTipSanityCheck is set whenever the live client changes (a
	// (re)connect, failover, or primary re-canonicalization) and cleared
	// after the first subsequent tip height read compares it against
	// lastTipHeight. Accessed only while clientMutex is held.
	pendingTipSanityCheck bool
}

// Connect initializes handle with provided Config.
func Connect(parentCtx context.Context, config Config) (bitcoin.Chain, error) {
	connection, err := connect(parentCtx, config, func(ctx context.Context, url string) (electrumClient, error) {
		return electrum.NewClient(ctx, url, nil)
	})
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func connect(
	parentCtx context.Context,
	config Config,
	newClient func(context.Context, string) (electrumClient, error),
) (*Connection, error) {
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
		clientMutex: &sync.Mutex{},
		newClient:   newClient,
	}

	if config.URL != "" {
		c.serverURLs = append(c.serverURLs, config.URL)
	}
	for _, url := range config.FallbackURLs {
		if url != "" && !slices.Contains(c.serverURLs, url) {
			c.serverURLs = append(c.serverURLs, url)
		}
	}
	if len(c.serverURLs) == 0 {
		return nil, fmt.Errorf("no electrum server URLs configured")
	}

	if len(c.serverURLs) > 1 {
		logger.Infof("electrum failover armed with [%d] servers", len(c.serverURLs))
	} else {
		logger.Infof("single server configured; failover unavailable")
	}

	if err := c.electrumConnect(parentCtx, parentCtx); err != nil {
		return nil, fmt.Errorf("failed to initialize electrum client: [%w]", err)
	}

	// Keep the connection alive and check the connection health.
	go c.keepAlive()

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
		c.parentCtx,
		c,
		func(ctx context.Context, client electrumClient) (string, error) {
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
	ctx context.Context,
	transactionHash bitcoin.Hash,
) (uint, error) {
	txID := transactionHash.Hex(bitcoin.ReversedByteOrder)

	txLogger := logger.With(
		zap.String("txID", txID),
	)

	// Bound the network calls below by both the caller's context and the
	// connection lifetime context, so a cancelled caller aborts the lookup while
	// connection shutdown still cancels it as before.
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopOnParentDone := context.AfterFunc(c.parentCtx, cancel)
	defer stopOnParentDone()

	rawTransaction, err := requestWithRetry(
		reqCtx,
		c,
		func(ctx context.Context, client electrumClient) (string, error) {
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
			reqCtx,
			c,
			func(
				ctx context.Context,
				client electrumClient,
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
		c.parentCtx,
		c,
		func(ctx context.Context, client electrumClient) (string, error) {
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
		c.parentCtx,
		c,
		func(ctx context.Context, client electrumClient) (int32, error) {
			tip, err := client.SubscribeHeadersSingle(ctx)
			if err != nil {
				return 0, fmt.Errorf("failed to get the blocks tip height: [%w]", err)
			}

			if tip.Height > 0 {
				newTip := uint(tip.Height)
				if c.pendingTipSanityCheck {
					c.pendingTipSanityCheck = false
					if c.lastTipHeight > 0 {
						diff := int64(newTip) - int64(c.lastTipHeight)
						if diff < 0 {
							diff = -diff
						}
						if diff > 500 {
							logger.Warnf(
								"electrum tip height changed by [%d] blocks "+
									"after reconnecting (was [%d], now [%d]); "+
									"this is expected during a legitimate reorg",
								diff,
								c.lastTipHeight,
								newTip,
							)
						}
					}
				}
				c.lastTipHeight = newTip
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
	if blockHeight > math.MaxUint32 {
		return nil, fmt.Errorf("block height exceeds Electrum range: [%v]", blockHeight)
	}
	height := uint32(blockHeight)

	getBlockHeaderResult, err := requestWithRetry(
		c.parentCtx,
		c,
		func(
			ctx context.Context,
			client electrumClient,
		) (*electrum.GetBlockHeaderResult, error) {
			return client.GetBlockHeader(ctx, height, 0)
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
	if blockHeight > math.MaxUint32 {
		return nil, fmt.Errorf("block height exceeds Electrum range: [%v]", blockHeight)
	}
	height := uint32(blockHeight)

	txID := transactionHash.Hex(bitcoin.ReversedByteOrder)

	getMerkleProofResult, err := requestWithRetry(
		c.parentCtx,
		c,
		func(
			ctx context.Context,
			client electrumClient,
		) (*electrum.GetMerkleProofResult, error) {
			return client.GetMerkleProof(
				ctx,
				txID,
				height,
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

	var selectedTxHashes []bitcoin.Hash
	if len(txHashes) > limit {
		selectedTxHashes = txHashes[len(txHashes)-limit:]
	} else {
		selectedTxHashes = txHashes
	}

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

	p2pkhItems, err := c.getConfirmedScriptHistory(p2pkh)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2PKH history for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkhItems, err := c.getConfirmedScriptHistory(p2wpkh)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2WPKH history for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	items := append(p2pkhItems, p2wpkhItems...)

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
		c.parentCtx,
		c,
		func(
			ctx context.Context,
			client electrumClient,
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
	if blockHeight > math.MaxUint32 {
		return bitcoin.Hash{}, fmt.Errorf("block height exceeds Electrum range: [%v]", blockHeight)
	}
	height := uint32(blockHeight)

	txHashString, err := requestWithRetry(
		c.parentCtx,
		c,
		func(
			ctx context.Context,
			client electrumClient,
		) (string, error) {
			return client.GetHashFromPosition(ctx, height, 0)
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
		c.parentCtx,
		c,
		func(
			ctx context.Context,
			client electrumClient,
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

// utxoValue converts a server-reported UTXO amount to the signed satoshi
// value bitcoin.UnspentTransactionOutput expects. A malicious or buggy
// server could report a value that overflows int64; reject it outright
// rather than silently wrapping it negative.
func utxoValue(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("unexpected UTXO value from Electrum server")
	}
	return int64(value), nil
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

	p2pkhItems, err := c.getScriptUtxos(p2pkh, true)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2PKH UTXOs for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkhItems, err := c.getScriptUtxos(p2wpkh, true)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2WPKH UTXOs for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	items := append(p2pkhItems, p2wpkhItems...)

	sort.SliceStable(
		items,
		func(i, j int) bool {
			return items[i].blockHeight < items[j].blockHeight
		},
	)

	utxos := make([]*bitcoin.UnspentTransactionOutput, len(items))
	for i, item := range items {
		value, err := utxoValue(item.value)
		if err != nil {
			return nil, err
		}
		utxos[i] = &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: item.txHash,
				OutputIndex:     item.outputIndex,
			},
			Value: value,
		}
	}

	return utxos, nil
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

	p2pkhItems, err := c.getScriptUtxos(p2pkh, false)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2PKH UTXOs for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	p2wpkhItems, err := c.getScriptUtxos(p2wpkh, false)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get P2WPKH UTXOs for public key hash [0x%x]: [%v]",
			publicKeyHash,
			err,
		)
	}

	items := append(p2pkhItems, p2wpkhItems...)

	return convertUtxoItems(items)
}

func convertUtxoItems(items []*scriptUtxoItem) ([]*bitcoin.UnspentTransactionOutput, error) {
	utxos := make([]*bitcoin.UnspentTransactionOutput, len(items))
	for i, item := range items {
		value, err := utxoValue(item.value)
		if err != nil {
			return nil, err
		}
		utxos[i] = &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: item.txHash,
				OutputIndex:     item.outputIndex,
			},
			Value: value,
		}
	}

	return utxos, nil
}

type scriptUtxoItem struct {
	txHash      bitcoin.Hash
	outputIndex uint32
	value       uint64
	blockHeight int32
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
		c.parentCtx,
		c,
		func(
			ctx context.Context,
			client electrumClient,
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

// isTransportFailure reports whether err is a transport/connection-class
// failure (a dead socket, a request timeout, or the client shutting down) as
// opposed to a JSON-RPC application-level error returned by a server that is
// alive and responding correctly (tx-not-found, tx-rejected,
// height-out-of-range, -32602/-32603, etc). Only a transport failure means
// the current server is unhealthy; an application error means it answered
// and failing away from it would not help.
func isTransportFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, electrum.ErrServerShutdown) ||
		errors.Is(err, electrum.ErrTimeout) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// getFeeBtcPerKbOnce issues a single blockchain.estimatefee call (no multi-minute
// retry loop). Persistent RPC errors for one confirmation target should not
// exhaust RequestRetryTimeout; EstimateSatPerVByteFee tries looser targets next.
// budgetCtx bounds both reconnection and the request itself; callers share
// one budget across every confirmation target they try.
func (c *Connection) getFeeBtcPerKbOnce(budgetCtx context.Context, blocks uint32) (float32, error) {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
	if err := c.reconnectIfShutdown(c.parentCtx, budgetCtx); err != nil {
		return 0, err
	}
	requestCtx, requestCancel := context.WithTimeout(
		budgetCtx,
		c.config.RequestTimeout,
	)
	defer requestCancel()
	stopAbort := watchAbort(requestCtx, c.client)
	fee, err := c.client.GetFee(requestCtx, blocks)
	stopAbort()
	if err != nil {
		if isTransportFailure(err) {
			c.failover(c.parentCtx)
		}
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

	// One retry budget covers every confirmation target tried below, so a
	// slow or unresponsive server cannot multiply RequestRetryTimeout by the
	// number of fallback targets.
	budgetCtx, budgetCancel := context.WithTimeout(c.parentCtx, c.config.RequestRetryTimeout)
	defer budgetCancel()

	for _, b := range targets {
		if budgetCtx.Err() != nil {
			lastErr = budgetCtx.Err()
			sawNonOracleFailure = true
			break
		}
		btcPerKbFee, err := c.getFeeBtcPerKbOnce(budgetCtx, b)
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

// electrumConnect cycles through configured servers after failed attempts,
// until the connection and enclosing retry budgets are exhausted - it is not
// guaranteed to reach every configured URL.
func (c *Connection) electrumConnect(callerCtx, budgetCtx context.Context) error {
	return wrappers.DoWithDefaultRetry(
		budgetCtx,
		c.config.ConnectRetryTimeout,
		func(ctx context.Context) error {
			url := c.serverURLs[c.serverIndex]
			connectCtx, connectCancel := context.WithTimeout(ctx, c.config.ConnectTimeout)
			client, err := c.newClient(connectCtx, url)
			connectCancel()

			var closeClient func()
			if err == nil {
				// Verification is an RPC with its own timeout, independent
				// of dialing but still bounded by the caller and retry
				// deadlines. When ctx (this attempt's connect retry budget)
				// cannot cover the full request timeout, requestCtx below
				// inherits ctx's own nearer deadline instead of getting its
				// own timer (context.WithTimeout reuses the tighter parent
				// deadline) - so a truncated verification failure and an
				// expired ctx are the same event, and the check below
				// correctly treats it as budget exhaustion rather than
				// candidate health.
				requestCtx, requestCancel := context.WithTimeout(ctx, c.config.RequestTimeout)
				// While the client is provisional, only the requestCtx
				// abort watcher is installed: a blocked write cannot
				// outlive verification. The long-lived parentCtx watcher is
				// installed only once the client is published.
				stopVerification := watchAbort(requestCtx, client)
				var genesisHash string
				err = requestCtx.Err()
				if err == nil {
					genesisHash, err = verifyServer(requestCtx, client, url, c.verifiedGenesisHash)
				}
				// Stop before cancelling or retaining the client. An
				// expired context must not leave an aborted client
				// published as healthy.
				stopVerification()
				if err == nil {
					err = requestCtx.Err()
				}
				if err == nil {
					err = c.parentCtx.Err()
				}
				requestCancel()
				if err != nil {
					go client.Abort()
				} else {
					c.verifiedGenesisHash = genesisHash
					closeClient = watchClientCancellation(c.parentCtx, client)
				}
			}
			if err != nil {
				// Preserve the candidate when its attempt was interrupted by
				// the caller, by parentCtx cancellation, or by the
				// enclosing connect retry budget expiring mid-attempt
				// (ctx.Err() != nil at this point covers both a genuinely
				// expired budget and a verification requestCtx that shared
				// that same expired deadline). A genuine candidate failure
				// - a dial or verification error while ctx was still live -
				// is the only case that indicates server health and should
				// advance to the next configured URL.
				if callerCtx.Err() == nil &&
					c.parentCtx.Err() == nil &&
					ctx.Err() == nil {
					c.nextServer()
				}
				return err
			}
			c.client = client
			c.closeClient = closeClient
			c.pendingTipSanityCheck = true
			return nil
		},
	)
}

// verifyServer queries the candidate's protocol version (logging a warning
// if unsupported) and, when the candidate reports a genesis hash, verifies
// it against the connection's chain reference. A candidate that omits the
// genesis hash is accepted with a warning (chain identity simply isn't
// verified for that candidate), and only a candidate reporting a hash that
// actively mismatches the established reference is rejected.
// expectedGenesisHash is the hash previously adopted for this connection,
// or "" if no server has reported one yet. It returns the hash this
// connection should use going forward (unchanged or newly adopted), or an
// error if the candidate reports a genesis hash that mismatches the
// established reference.
func verifyServer(
	ctx context.Context,
	client electrumClient,
	url string,
	expectedGenesisHash string,
) (string, error) {
	version, protocol, err := client.ServerVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get server version: [%w]", err)
	}
	logger.Infof(
		"connected to electrum server [version: [%s], protocol: [%s]]",
		version,
		protocol,
	)
	if !slices.Contains(supportedProtocolVersions, protocol) {
		logger.Warnf(
			"electrum server [%s] runs an unsupported protocol version: [%s]; expected one of: [%s]",
			url,
			protocol,
			strings.Join(supportedProtocolVersions, ","),
		)
	}

	features, err := client.ServerFeatures(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get server features: [%w]", err)
	}

	if features == nil || features.GenesisHash == "" {
		logger.Warnf(
			"electrum server [%s] did not report a genesis hash; its chain "+
				"identity could not be verified",
			url,
		)
		return expectedGenesisHash, nil
	}

	if expectedGenesisHash == "" {
		logger.Warnf(
			"adopting genesis hash [%s] reported by electrum server [%s] as "+
				"this connection's chain reference",
			features.GenesisHash,
			url,
		)
		return features.GenesisHash, nil
	}

	if !strings.EqualFold(expectedGenesisHash, features.GenesisHash) {
		return "", fmt.Errorf(
			"electrum server [%s] reported genesis hash [%s], which does not "+
				"match this connection's chain reference [%s]; the server "+
				"appears to be on a different chain",
			url,
			features.GenesisHash,
			expectedGenesisHash,
		)
	}

	return expectedGenesisHash, nil
}

// nextServer, retireClient, and failover are called with clientMutex held.
func (c *Connection) nextServer() {
	c.serverIndex = (c.serverIndex + 1) % len(c.serverURLs)
}

func (c *Connection) retireClient() {
	c.client = nil
	if c.closeClient != nil {
		c.closeClient()
	}
	c.closeClient = nil
}

func (c *Connection) failover(callerCtx context.Context) {
	// Cancellation belongs to the caller, not to the server's health.
	if callerCtx.Err() != nil || c.parentCtx.Err() != nil {
		return
	}
	c.retireClient()
	c.nextServer()
	// A single configured URL now retires and re-dials the same server on a
	// transport failure instead of wedging on a half-open socket; explicit or
	// pinned URLs keep the existing retry semantics otherwise.
	if len(c.serverURLs) > 1 {
		logger.Warn("electrum request failed; trying the next configured server")
	} else {
		logger.Warn("electrum request failed; reconnecting to the configured server")
	}
}

// healAfterConsecutivePings is the number of consecutive successful keepalive
// pings on a non-primary server before Connection attempts to re-canonicalize
// the primary (index 0) as the live client.
const healAfterConsecutivePings = 2

func (c *Connection) keepAlive() {
	ticker := time.NewTicker(c.config.KeepAliveInterval)
	defer ticker.Stop()

	consecutiveSuccesses := 0

	for {
		select {
		case <-ticker.C:
			c.clientMutex.Lock()
			if err := c.reconnectIfShutdown(c.parentCtx, c.parentCtx); err != nil {
				c.clientMutex.Unlock()
				logger.Errorf(
					"failed to reconnect to the electrum server during keepalive: [%v]",
					err,
				)
				consecutiveSuccesses = 0
				continue
			}

			pingCtx, pingCancel := context.WithTimeout(c.parentCtx, c.config.RequestTimeout)
			stopAbort := watchAbort(pingCtx, c.client)
			pingErr := c.client.Ping(pingCtx)
			stopAbort()
			pingCancel()

			if pingErr != nil {
				c.failover(c.parentCtx)
				c.clientMutex.Unlock()
				logger.Errorf(
					"failed to ping the electrum server; "+
						"please verify health of the electrum server: [%v]",
					pingErr,
				)
				consecutiveSuccesses = 0
				continue
			}

			// Adjust ticker starting at the time of the latest successful ping.
			ticker.Reset(c.config.KeepAliveInterval)
			consecutiveSuccesses++
			onFallback := c.serverIndex != 0
			c.clientMutex.Unlock()

			if onFallback && consecutiveSuccesses >= healAfterConsecutivePings {
				c.healPrimary()
				consecutiveSuccesses = 0
			}
		case <-c.parentCtx.Done():
			// Each client's cancellation callback closes its transport even
			// if an RPC (including Ping above) is blocked while holding the
			// request mutex.
			return
		}
	}
}

// healPrimary re-verifies the primary (index 0) server after the connection
// has been running on a fallback for healAfterConsecutivePings consecutive
// keepalive pings. It dials and verifies a fresh client without holding
// clientMutex, so concurrent requests keep flowing through the current
// (fallback) client while the primary is probed. On success, the current
// client is retired and the freshly verified primary is published in its
// place. On any failure the current client is left untouched; the caller
// tries again after another healAfterConsecutivePings successful pings.
func (c *Connection) healPrimary() {
	primaryURL := c.serverURLs[0]

	connectCtx, connectCancel := context.WithTimeout(c.parentCtx, c.config.ConnectTimeout)
	client, err := c.newClient(connectCtx, primaryURL)
	connectCancel()
	if err != nil {
		logger.Debugf("primary electrum server re-verification failed to dial: [%v]", err)
		return
	}

	c.clientMutex.Lock()
	expectedGenesisHash := c.verifiedGenesisHash
	c.clientMutex.Unlock()

	requestCtx, requestCancel := context.WithTimeout(c.parentCtx, c.config.RequestTimeout)
	stopVerification := watchAbort(requestCtx, client)
	genesisHash, err := verifyServer(requestCtx, client, primaryURL, expectedGenesisHash)
	stopVerification()
	if err == nil {
		err = requestCtx.Err()
	}
	if err == nil {
		err = c.parentCtx.Err()
	}
	requestCancel()
	if err != nil {
		go client.Abort()
		logger.Debugf("primary electrum server re-verification failed: [%v]", err)
		return
	}

	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	if c.serverIndex == 0 || c.parentCtx.Err() != nil ||
		c.verifiedGenesisHash != expectedGenesisHash {
		// Another path already restored the primary, the connection is
		// shutting down, or the chain reference changed since this
		// candidate was verified against it; discard this candidate.
		go client.Abort()
		return
	}

	c.retireClient()
	c.serverIndex = 0
	c.client = client
	c.closeClient = watchClientCancellation(c.parentCtx, client)
	c.verifiedGenesisHash = genesisHash
	c.pendingTipSanityCheck = true
	logger.Infof("re-canonicalized primary electrum server [%s]", primaryURL)
}

func requestWithRetry[K any](
	parentCtx context.Context,
	c *Connection,
	requestFn func(ctx context.Context, client electrumClient) (K, error),
	requestName string,
) (K, error) {
	startTime := time.Now()
	logger.Debugf("starting [%s] request to Electrum server", requestName)

	var result K

	err := wrappers.DoWithDefaultRetry(
		parentCtx,
		c.config.RequestRetryTimeout,
		func(ctx context.Context) error {
			c.clientMutex.Lock()
			defer c.clientMutex.Unlock()

			if err := c.reconnectIfShutdown(parentCtx, ctx); err != nil {
				return err
			}

			requestCtx, requestCancel := context.WithTimeout(ctx, c.config.RequestTimeout)
			defer requestCancel()

			// An established client's write is not bound by the RPC
			// context: the vendored request() writes synchronously before
			// selecting on ctx.Done(). Abort the transport if the
			// per-request deadline elapses so a blocked write cannot
			// outlive its own request.
			stopAbort := watchAbort(requestCtx, c.client)
			r, err := requestFn(requestCtx, c.client)
			stopAbort()

			if err != nil {
				if isTransportFailure(err) {
					c.failover(parentCtx)
				}
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

// reconnectIfShutdown is called with clientMutex held. Reconnection is bounded
// by the enclosing request budget as well as the connection retry timeout.
func (c *Connection) reconnectIfShutdown(callerCtx, budgetCtx context.Context) error {
	if err := budgetCtx.Err(); err != nil {
		return err
	}
	if err := c.parentCtx.Err(); err != nil {
		return err
	}
	if c.client != nil && c.client.IsShutdown() {
		c.retireClient()
		c.nextServer()
	}
	if c.client == nil {
		logger.Warn("connection to electrum server is down; reconnecting...")
		err := c.electrumConnect(callerCtx, budgetCtx)
		if err != nil {
			return fmt.Errorf("failed to reconnect to electrum server: [%w]", err)
		}
		logger.Info("reconnected to electrum server")
	}

	return nil
}

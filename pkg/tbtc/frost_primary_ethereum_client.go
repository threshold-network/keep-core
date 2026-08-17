package tbtc

import (
	"context"
	"math/big"
	"time"

	geth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
)

// FrostPrimaryEthereumClient is the guarded primary Ethereum client exposed
// to the chain package. All finite request methods apply the configured
// transport timeout even when their caller supplies context.Background.
type FrostPrimaryEthereumClient interface {
	ethutil.EthereumClient
	ChainID(context.Context) (*big.Int, error)
	Client() *rpc.Client
	FrostPrimaryEthereumRequestTimeout() time.Duration
}

type frostPrimaryEthereumTimeoutClient struct {
	client         *ethclient.Client
	requestTimeout time.Duration
}

var _ FrostPrimaryEthereumClient = (*frostPrimaryEthereumTimeoutClient)(nil)

func (client *frostPrimaryEthereumTimeoutClient) FrostPrimaryEthereumRequestTimeout() time.Duration {
	return client.requestTimeout
}

func (client *frostPrimaryEthereumTimeoutClient) Client() *rpc.Client {
	return client.client.Client()
}

func (client *frostPrimaryEthereumTimeoutClient) SubscribeNewHead(
	ctx context.Context,
	channel chan<- *types.Header,
) (geth.Subscription, error) {
	return client.client.SubscribeNewHead(ctx, channel)
}

func (client *frostPrimaryEthereumTimeoutClient) SubscribeFilterLogs(
	ctx context.Context,
	query geth.FilterQuery,
	channel chan<- types.Log,
) (geth.Subscription, error) {
	return client.client.SubscribeFilterLogs(ctx, query, channel)
}

func (client *frostPrimaryEthereumTimeoutClient) requestContext(
	ctx context.Context,
) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, client.requestTimeout)
}

func (client *frostPrimaryEthereumTimeoutClient) ChainID(
	ctx context.Context,
) (*big.Int, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.ChainID(ctx)
}

func (client *frostPrimaryEthereumTimeoutClient) BlockByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Block, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.BlockByHash(ctx, hash)
}

func (client *frostPrimaryEthereumTimeoutClient) BlockByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Block, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.BlockByNumber(ctx, number)
}

func (client *frostPrimaryEthereumTimeoutClient) HeaderByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Header, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.HeaderByHash(ctx, hash)
}

func (client *frostPrimaryEthereumTimeoutClient) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Header, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.HeaderByNumber(ctx, number)
}

func (client *frostPrimaryEthereumTimeoutClient) TransactionCount(
	ctx context.Context,
	blockHash common.Hash,
) (uint, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.TransactionCount(ctx, blockHash)
}

func (client *frostPrimaryEthereumTimeoutClient) TransactionInBlock(
	ctx context.Context,
	blockHash common.Hash,
	index uint,
) (*types.Transaction, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.TransactionInBlock(ctx, blockHash, index)
}

func (client *frostPrimaryEthereumTimeoutClient) TransactionByHash(
	ctx context.Context,
	transactionHash common.Hash,
) (*types.Transaction, bool, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.TransactionByHash(ctx, transactionHash)
}

func (client *frostPrimaryEthereumTimeoutClient) TransactionReceipt(
	ctx context.Context,
	transactionHash common.Hash,
) (*types.Receipt, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.TransactionReceipt(ctx, transactionHash)
}

func (client *frostPrimaryEthereumTimeoutClient) BalanceAt(
	ctx context.Context,
	account common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.BalanceAt(ctx, account, blockNumber)
}

func (client *frostPrimaryEthereumTimeoutClient) CodeAt(
	ctx context.Context,
	account common.Address,
	blockNumber *big.Int,
) ([]byte, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.CodeAt(ctx, account, blockNumber)
}

func (client *frostPrimaryEthereumTimeoutClient) CallContract(
	ctx context.Context,
	message geth.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.CallContract(ctx, message, blockNumber)
}

func (client *frostPrimaryEthereumTimeoutClient) PendingCodeAt(
	ctx context.Context,
	account common.Address,
) ([]byte, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.PendingCodeAt(ctx, account)
}

func (client *frostPrimaryEthereumTimeoutClient) PendingNonceAt(
	ctx context.Context,
	account common.Address,
) (uint64, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.PendingNonceAt(ctx, account)
}

func (client *frostPrimaryEthereumTimeoutClient) SuggestGasPrice(
	ctx context.Context,
) (*big.Int, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.SuggestGasPrice(ctx)
}

func (client *frostPrimaryEthereumTimeoutClient) SuggestGasTipCap(
	ctx context.Context,
) (*big.Int, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.SuggestGasTipCap(ctx)
}

func (client *frostPrimaryEthereumTimeoutClient) EstimateGas(
	ctx context.Context,
	message geth.CallMsg,
) (uint64, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.EstimateGas(ctx, message)
}

func (client *frostPrimaryEthereumTimeoutClient) SendTransaction(
	ctx context.Context,
	transaction *types.Transaction,
) error {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.SendTransaction(ctx, transaction)
}

func (client *frostPrimaryEthereumTimeoutClient) FilterLogs(
	ctx context.Context,
	query geth.FilterQuery,
) ([]types.Log, error) {
	ctx, cancel := client.requestContext(ctx)
	defer cancel()
	return client.client.FilterLogs(ctx, query)
}

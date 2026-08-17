package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"time"

	geth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

const ethereumRPCAcquirePermitTimeout = 5 * time.Minute

type contextAwareEthereumRPCLimiter struct {
	requestRate *rate.Limiter
	concurrency *semaphore.Weighted
}

func newEthereumRPCLimiter(
	requestsPerSecondLimit int,
	concurrencyLimit int,
) *contextAwareEthereumRPCLimiter {
	result := &contextAwareEthereumRPCLimiter{}
	if requestsPerSecondLimit > 0 {
		result.requestRate = rate.NewLimiter(
			rate.Limit(requestsPerSecondLimit),
			1,
		)
	}
	if concurrencyLimit > 0 {
		result.concurrency = semaphore.NewWeighted(
			int64(concurrencyLimit),
		)
	}
	return result
}

func (limiter *contextAwareEthereumRPCLimiter) AcquirePermit(
	ctx context.Context,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(
		ctx,
		ethereumRPCAcquirePermitTimeout,
	)
	defer cancel()

	if limiter.requestRate != nil {
		if err := limiter.requestRate.Wait(ctx); err != nil {
			return err
		}
	}
	if limiter.concurrency != nil {
		if err := limiter.concurrency.Acquire(ctx, 1); err != nil {
			return err
		}
	}
	return nil
}

func (limiter *contextAwareEthereumRPCLimiter) ReleasePermit() {
	if limiter.concurrency != nil {
		limiter.concurrency.Release(1)
	}
}

type ethereumRPCLimitingClient struct {
	ethutil.EthereumClient
	limiter *contextAwareEthereumRPCLimiter
}

func wrapEthereumClientWithRPCLimiter(
	client ethutil.EthereumClient,
	limiter *contextAwareEthereumRPCLimiter,
) ethutil.EthereumClient {
	return &ethereumRPCLimitingClient{
		EthereumClient: client,
		limiter:        limiter,
	}
}

func (client *ethereumRPCLimitingClient) AcquirePermit(
	ctx context.Context,
) error {
	return client.limiter.AcquirePermit(ctx)
}

func (client *ethereumRPCLimitingClient) ReleasePermit() {
	client.limiter.ReleasePermit()
}

func (client *ethereumRPCLimitingClient) acquirePermit(
	ctx context.Context,
) error {
	if err := client.limiter.AcquirePermit(ctx); err != nil {
		return fmt.Errorf("cannot acquire rate limiter permit: [%w]", err)
	}
	return nil
}

func (client *ethereumRPCLimitingClient) CodeAt(
	ctx context.Context,
	contract common.Address,
	blockNumber *big.Int,
) ([]byte, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.CodeAt(ctx, contract, blockNumber)
}

func (client *ethereumRPCLimitingClient) CallContract(
	ctx context.Context,
	call geth.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.CallContract(ctx, call, blockNumber)
}

func (client *ethereumRPCLimitingClient) PendingCodeAt(
	ctx context.Context,
	account common.Address,
) ([]byte, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.PendingCodeAt(ctx, account)
}

func (client *ethereumRPCLimitingClient) PendingNonceAt(
	ctx context.Context,
	account common.Address,
) (uint64, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return 0, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.PendingNonceAt(ctx, account)
}

func (client *ethereumRPCLimitingClient) SuggestGasPrice(
	ctx context.Context,
) (*big.Int, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.SuggestGasPrice(ctx)
}

func (client *ethereumRPCLimitingClient) SuggestGasTipCap(
	ctx context.Context,
) (*big.Int, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.SuggestGasTipCap(ctx)
}

func (client *ethereumRPCLimitingClient) EstimateGas(
	ctx context.Context,
	call geth.CallMsg,
) (uint64, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return 0, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.EstimateGas(ctx, call)
}

func (client *ethereumRPCLimitingClient) SendTransaction(
	ctx context.Context,
	transaction *types.Transaction,
) error {
	if err := client.acquirePermit(ctx); err != nil {
		return err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.SendTransaction(ctx, transaction)
}

func (client *ethereumRPCLimitingClient) FilterLogs(
	ctx context.Context,
	query geth.FilterQuery,
) ([]types.Log, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.FilterLogs(ctx, query)
}

func (client *ethereumRPCLimitingClient) SubscribeFilterLogs(
	ctx context.Context,
	query geth.FilterQuery,
	channel chan<- types.Log,
) (geth.Subscription, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.SubscribeFilterLogs(ctx, query, channel)
}

func (client *ethereumRPCLimitingClient) BlockByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Block, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.BlockByHash(ctx, hash)
}

func (client *ethereumRPCLimitingClient) BlockByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Block, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.BlockByNumber(ctx, number)
}

func (client *ethereumRPCLimitingClient) HeaderByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Header, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.HeaderByHash(ctx, hash)
}

func (client *ethereumRPCLimitingClient) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Header, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.HeaderByNumber(ctx, number)
}

func (client *ethereumRPCLimitingClient) TransactionCount(
	ctx context.Context,
	blockHash common.Hash,
) (uint, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return 0, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.TransactionCount(ctx, blockHash)
}

func (client *ethereumRPCLimitingClient) TransactionInBlock(
	ctx context.Context,
	blockHash common.Hash,
	index uint,
) (*types.Transaction, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.TransactionInBlock(ctx, blockHash, index)
}

func (client *ethereumRPCLimitingClient) SubscribeNewHead(
	ctx context.Context,
	channel chan<- *types.Header,
) (geth.Subscription, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.SubscribeNewHead(ctx, channel)
}

func (client *ethereumRPCLimitingClient) TransactionByHash(
	ctx context.Context,
	transactionHash common.Hash,
) (*types.Transaction, bool, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, false, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.TransactionByHash(ctx, transactionHash)
}

func (client *ethereumRPCLimitingClient) TransactionReceipt(
	ctx context.Context,
	transactionHash common.Hash,
) (*types.Receipt, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.TransactionReceipt(ctx, transactionHash)
}

func (client *ethereumRPCLimitingClient) BalanceAt(
	ctx context.Context,
	account common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	if err := client.acquirePermit(ctx); err != nil {
		return nil, err
	}
	defer client.limiter.ReleasePermit()
	return client.EthereumClient.BalanceAt(ctx, account, blockNumber)
}

package ethutil

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-common/pkg/rate"
)

type rateLimiter struct {
	EthereumClient

	*rate.Limiter
}

// WrapRateLimiting wraps the given contract backend with rate limiting
// capabilities with respect to the provided configuration.
// All types of requests to the contract are rate-limited,
// including view function calls.
func WrapRateLimiting(
	client EthereumClient,
	config *rate.LimiterConfig,
) EthereumClient {
	return &rateLimiter{
		EthereumClient: client,
		Limiter:        rate.NewLimiter(config),
	}
}

func (rl *rateLimiter) CodeAt(
	ctx context.Context,
	contract common.Address,
	blockNumber *big.Int,
) ([]byte, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.CodeAt(ctx, contract, blockNumber)
}

func (rl *rateLimiter) CallContract(
	ctx context.Context,
	call ethereum.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.CallContract(ctx, call, blockNumber)
}

func (rl *rateLimiter) PendingCodeAt(
	ctx context.Context,
	account common.Address,
) ([]byte, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.PendingCodeAt(ctx, account)
}

func (rl *rateLimiter) PendingNonceAt(
	ctx context.Context,
	account common.Address,
) (uint64, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return 0, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.PendingNonceAt(ctx, account)
}

func (rl *rateLimiter) SuggestGasPrice(
	ctx context.Context,
) (*big.Int, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.SuggestGasPrice(ctx)
}

func (rl *rateLimiter) SuggestGasTipCap(
	ctx context.Context,
) (*big.Int, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.SuggestGasTipCap(ctx)
}

func (rl *rateLimiter) EstimateGas(
	ctx context.Context,
	call ethereum.CallMsg,
) (uint64, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return 0, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.EstimateGas(ctx, call)
}

func (rl *rateLimiter) SendTransaction(
	ctx context.Context,
	tx *types.Transaction,
) error {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.SendTransaction(ctx, tx)
}

func (rl *rateLimiter) FilterLogs(
	ctx context.Context,
	query ethereum.FilterQuery,
) ([]types.Log, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.FilterLogs(ctx, query)
}

func (rl *rateLimiter) SubscribeFilterLogs(
	ctx context.Context,
	query ethereum.FilterQuery,
	ch chan<- types.Log,
) (ethereum.Subscription, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.SubscribeFilterLogs(ctx, query, ch)
}

func (rl *rateLimiter) BlockByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Block, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.BlockByHash(ctx, hash)
}

func (rl *rateLimiter) BlockByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Block, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.BlockByNumber(ctx, number)
}

func (rl *rateLimiter) HeaderByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Header, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.HeaderByHash(ctx, hash)
}

func (rl *rateLimiter) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Header, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.HeaderByNumber(ctx, number)
}

func (rl *rateLimiter) TransactionCount(
	ctx context.Context,
	blockHash common.Hash,
) (uint, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return 0, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.TransactionCount(ctx, blockHash)
}

func (rl *rateLimiter) TransactionInBlock(
	ctx context.Context,
	blockHash common.Hash,
	index uint,
) (*types.Transaction, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.TransactionInBlock(ctx, blockHash, index)
}

func (rl *rateLimiter) SubscribeNewHead(
	ctx context.Context,
	ch chan<- *types.Header,
) (ethereum.Subscription, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.SubscribeNewHead(ctx, ch)
}

func (rl *rateLimiter) TransactionByHash(
	ctx context.Context,
	txHash common.Hash,
) (*types.Transaction, bool, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, false, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.TransactionByHash(ctx, txHash)
}

func (rl *rateLimiter) TransactionReceipt(
	ctx context.Context,
	txHash common.Hash,
) (*types.Receipt, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.TransactionReceipt(ctx, txHash)
}

func (rl *rateLimiter) BalanceAt(
	ctx context.Context,
	account common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	err := rl.Limiter.AcquirePermit()
	if err != nil {
		return nil, fmt.Errorf("cannot acquire rate limiter permit: [%v]", err)
	}
	defer rl.Limiter.ReleasePermit()

	return rl.EthereumClient.BalanceAt(ctx, account, blockNumber)
}

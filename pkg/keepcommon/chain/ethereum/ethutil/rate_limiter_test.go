package ethutil

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-common/pkg/rate"
)

func TestRateLimiter(t *testing.T) {
	requestsPerSecondLimit := 500
	concurrencyLimit := 5
	acquirePermitTimeout := time.Minute
	requests := 500
	requestDuration := 10 * time.Millisecond

	client := &mockEthereumClient{
		requestDuration,
		make([]string, 0),
		sync.Mutex{},
	}

	rateLimitingClient := WrapRateLimiting(
		client,
		&rate.LimiterConfig{
			RequestsPerSecondLimit: requestsPerSecondLimit,
			ConcurrencyLimit:       concurrencyLimit,
			AcquirePermitTimeout:   acquirePermitTimeout,
		},
	)

	for testName, test := range getTests(rateLimitingClient) {
		t.Run(testName, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(requests)

			startSignal := make(chan struct{})

			for i := 0; i < requests; i++ {
				go func() {
					<-startSignal

					err := test.function()
					if err != nil {
						t.Errorf("unexpected error: [%v]", err)
					}

					wg.Done()
				}()
			}

			startTime := time.Now()
			close(startSignal)

			wg.Wait()

			duration := time.Now().Sub(startTime)
			averageRequestsPerSecond := float64(requests) / duration.Seconds()

			if averageRequestsPerSecond > float64(requestsPerSecondLimit) {
				t.Errorf(
					"average requests per second exceeded the limit\n"+
						"limit:  [%v]\n"+
						"actual: [%v]",
					requestsPerSecondLimit,
					averageRequestsPerSecond,
				)
			}

			maxConcurrency := 0
			temporaryConcurrency := 0
			for _, event := range client.events {
				if event == "start" {
					temporaryConcurrency++
				}

				if event == "end" {
					temporaryConcurrency--
				}

				if temporaryConcurrency > maxConcurrency {
					maxConcurrency = temporaryConcurrency
				}
			}

			if maxConcurrency > concurrencyLimit {
				t.Errorf(
					"max concurrency exceeded the limit\n"+
						"limit:  [%v]\n"+
						"actual: [%v]",
					concurrencyLimit,
					maxConcurrency,
				)
			}
		})
	}
}

func TestRateLimiter_RequestsPerSecondLimitOnly(t *testing.T) {
	requestsPerSecondLimit := 500
	concurrencyLimit := 0 // disable the concurrency limit
	acquirePermitTimeout := time.Minute
	requests := 500
	requestDuration := 10 * time.Millisecond

	client := &mockEthereumClient{
		requestDuration,
		make([]string, 0),
		sync.Mutex{},
	}

	rateLimitingClient := WrapRateLimiting(
		client,
		&rate.LimiterConfig{
			RequestsPerSecondLimit: requestsPerSecondLimit,
			ConcurrencyLimit:       concurrencyLimit,
			AcquirePermitTimeout:   acquirePermitTimeout,
		},
	)

	for testName, test := range getTests(rateLimitingClient) {
		t.Run(testName, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(requests)

			startSignal := make(chan struct{})

			for i := 0; i < requests; i++ {
				go func() {
					<-startSignal

					err := test.function()
					if err != nil {
						t.Errorf("unexpected error: [%v]", err)
					}

					wg.Done()
				}()
			}

			startTime := time.Now()
			close(startSignal)

			wg.Wait()

			duration := time.Now().Sub(startTime)
			averageRequestsPerSecond := float64(requests) / duration.Seconds()

			// The actual average can exceed the limit a little bit because of
			// scheduling jitter and coarse timer resolution. Allow a slightly
			// wider deviation to avoid flakes on busy runners.
			maxDeviation := 0.15

			if averageRequestsPerSecond > (1+maxDeviation)*float64(requestsPerSecondLimit) {
				t.Errorf(
					"average requests per second exceeded the limit\n"+
						"limit:  [%v]\n"+
						"actual: [%v]",
					requestsPerSecondLimit,
					averageRequestsPerSecond,
				)
			}
		})
	}
}

func TestRateLimiter_ConcurrencyLimitOnly(t *testing.T) {
	requestsPerSecondLimit := 0 // disable the requests per second limit
	concurrencyLimit := 50
	acquirePermitTimeout := time.Minute
	requests := 500
	requestDuration := 10 * time.Millisecond

	client := &mockEthereumClient{
		requestDuration,
		make([]string, 0),
		sync.Mutex{},
	}

	rateLimitingClient := WrapRateLimiting(
		client,
		&rate.LimiterConfig{
			RequestsPerSecondLimit: requestsPerSecondLimit,
			ConcurrencyLimit:       concurrencyLimit,
			AcquirePermitTimeout:   acquirePermitTimeout,
		},
	)

	for testName, test := range getTests(rateLimitingClient) {
		t.Run(testName, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(requests)

			startSignal := make(chan struct{})

			for i := 0; i < requests; i++ {
				go func() {
					<-startSignal

					err := test.function()
					if err != nil {
						t.Errorf("unexpected error: [%v]", err)
					}

					wg.Done()
				}()
			}

			close(startSignal)

			wg.Wait()

			maxConcurrency := 0
			temporaryConcurrency := 0
			for _, event := range client.events {
				if event == "start" {
					temporaryConcurrency++
				}

				if event == "end" {
					temporaryConcurrency--
				}

				if temporaryConcurrency > maxConcurrency {
					maxConcurrency = temporaryConcurrency
				}
			}

			if maxConcurrency > concurrencyLimit {
				t.Errorf(
					"max concurrency exceeded the limit\n"+
						"limit:  [%v]\n"+
						"actual: [%v]",
					concurrencyLimit,
					maxConcurrency,
				)
			}
		})
	}
}

func TestRateLimiter_AcquirePermitTimout(t *testing.T) {
	requestsPerSecondLimit := 1
	concurrencyLimit := 1
	acquirePermitTimeout := 10 * time.Millisecond
	requests := 3
	requestDuration := 250 * time.Millisecond

	client := &mockEthereumClient{
		requestDuration,
		make([]string, 0),
		sync.Mutex{},
	}

	rateLimitingClient := WrapRateLimiting(
		client,
		&rate.LimiterConfig{
			RequestsPerSecondLimit: requestsPerSecondLimit,
			ConcurrencyLimit:       concurrencyLimit,
			AcquirePermitTimeout:   acquirePermitTimeout,
		},
	)

	wg := sync.WaitGroup{}
	wg.Add(requests)

	startSignal := make(chan struct{})
	errors := make(chan error, requests)

	for i := 0; i < requests; i++ {
		go func() {
			<-startSignal

			err := rateLimitingClient.SendTransaction(context.Background(), nil)
			if err != nil {
				errors <- err
			}

			wg.Done()
		}()
	}

	close(startSignal)

	wg.Wait()

	close(errors)
	if len(errors) == 0 {
		t.Fatalf("at least one timeout error should be present")
	}

	for e := range errors {
		if !strings.Contains(e.Error(), "context deadline") {
			t.Errorf(
				"error should be related with the context deadline\n"+
					"actual error: [%v]",
				e,
			)
		}
	}
}

type mockEthereumClient struct {
	requestDuration time.Duration

	events []string
	mutex  sync.Mutex
}

func (mec *mockEthereumClient) mockRequest() {
	mec.mutex.Lock()
	mec.events = append(mec.events, "start")
	mec.mutex.Unlock()

	time.Sleep(mec.requestDuration)

	mec.mutex.Lock()
	mec.events = append(mec.events, "end")
	mec.mutex.Unlock()
}

func (mec *mockEthereumClient) CodeAt(
	ctx context.Context,
	contract common.Address,
	blockNumber *big.Int,
) ([]byte, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) CallContract(
	ctx context.Context,
	call ethereum.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) PendingCodeAt(
	ctx context.Context,
	account common.Address,
) ([]byte, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) PendingNonceAt(
	ctx context.Context,
	account common.Address,
) (uint64, error) {
	mec.mockRequest()
	return 0, nil
}

func (mec *mockEthereumClient) SuggestGasPrice(
	ctx context.Context,
) (*big.Int, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) SuggestGasTipCap(
	ctx context.Context,
) (*big.Int, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) EstimateGas(
	ctx context.Context,
	call ethereum.CallMsg,
) (uint64, error) {
	mec.mockRequest()
	return 0, nil
}

func (mec *mockEthereumClient) SendTransaction(
	ctx context.Context,
	tx *types.Transaction,
) error {
	mec.mockRequest()
	return nil
}

func (mec *mockEthereumClient) FilterLogs(
	ctx context.Context,
	query ethereum.FilterQuery,
) ([]types.Log, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) SubscribeFilterLogs(
	ctx context.Context,
	query ethereum.FilterQuery,
	ch chan<- types.Log,
) (ethereum.Subscription, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) BlockByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Block, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) BlockByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Block, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) HeaderByHash(
	ctx context.Context,
	hash common.Hash,
) (*types.Header, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*types.Header, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) TransactionCount(
	ctx context.Context,
	blockHash common.Hash,
) (uint, error) {
	mec.mockRequest()
	return 0, nil
}

func (mec *mockEthereumClient) TransactionInBlock(
	ctx context.Context,
	blockHash common.Hash,
	index uint,
) (*types.Transaction, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) SubscribeNewHead(
	ctx context.Context,
	ch chan<- *types.Header,
) (ethereum.Subscription, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) TransactionByHash(
	ctx context.Context,
	txHash common.Hash,
) (*types.Transaction, bool, error) {
	mec.mockRequest()
	return nil, false, nil
}

func (mec *mockEthereumClient) TransactionReceipt(
	ctx context.Context,
	txHash common.Hash,
) (*types.Receipt, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) SubscribeTransactionReceipts(
	ctx context.Context,
	q *ethereum.TransactionReceiptsQuery,
	ch chan<- []*types.Receipt,
) (ethereum.Subscription, error) {
	mec.mockRequest()
	return nil, nil
}

func (mec *mockEthereumClient) BalanceAt(
	ctx context.Context,
	account common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	mec.mockRequest()
	return nil, nil
}

func getTests(
	client EthereumClient,
) map[string]struct{ function func() error } {
	return map[string]struct{ function func() error }{
		"test CodeAt": {
			function: func() error {
				_, err := client.CodeAt(
					context.Background(),
					[20]byte{},
					nil,
				)
				return err
			},
		},
		"test CallContract": {
			function: func() error {
				_, err := client.CallContract(
					context.Background(),
					ethereum.CallMsg{},
					nil,
				)
				return err
			},
		},
		"test PendingCodeAt": {
			function: func() error {
				_, err := client.PendingCodeAt(
					context.Background(),
					[20]byte{},
				)
				return err
			},
		},
		"test PendingNonceAt": {
			function: func() error {
				_, err := client.PendingNonceAt(
					context.Background(),
					[20]byte{},
				)
				return err
			},
		},
		"test SuggestGasPrice": {
			function: func() error {
				_, err := client.SuggestGasPrice(
					context.Background(),
				)
				return err
			},
		},
		"test SuggestGasTipCap": {
			function: func() error {
				_, err := client.SuggestGasTipCap(
					context.Background(),
				)
				return err
			},
		},
		"test EstimateGas": {
			function: func() error {
				_, err := client.EstimateGas(
					context.Background(),
					ethereum.CallMsg{},
				)
				return err
			},
		},
		"test SendTransaction": {
			function: func() error {
				err := client.SendTransaction(
					context.Background(),
					nil,
				)
				return err
			},
		},
		"test FilterLogs": {
			function: func() error {
				_, err := client.FilterLogs(
					context.Background(),
					ethereum.FilterQuery{},
				)
				return err
			},
		},
		"test SubscribeFilterLogs": {
			function: func() error {
				_, err := client.SubscribeFilterLogs(
					context.Background(),
					ethereum.FilterQuery{},
					nil,
				)
				return err
			},
		},
		"test BlockByHash": {
			function: func() error {
				_, err := client.BlockByHash(
					context.Background(),
					common.Hash{},
				)
				return err
			},
		},
		"test BlockByNumber": {
			function: func() error {
				_, err := client.BlockByNumber(
					context.Background(),
					nil,
				)
				return err
			},
		},
		"test HeaderByHash": {
			function: func() error {
				_, err := client.HeaderByHash(
					context.Background(),
					common.Hash{},
				)
				return err
			},
		},
		"test HeaderByNumber": {
			function: func() error {
				_, err := client.HeaderByNumber(
					context.Background(),
					nil,
				)
				return err
			},
		},
		"test TransactionCount": {
			function: func() error {
				_, err := client.TransactionCount(
					context.Background(),
					common.Hash{},
				)
				return err
			},
		},
		"test TransactionInBlock": {
			function: func() error {
				_, err := client.TransactionInBlock(
					context.Background(),
					common.Hash{},
					0,
				)
				return err
			},
		},
		"test SubscribeNewHead": {
			function: func() error {
				_, err := client.SubscribeNewHead(
					context.Background(),
					nil,
				)
				return err
			},
		},
		"test TransactionByHash": {
			function: func() error {
				_, _, err := client.TransactionByHash(
					context.Background(),
					common.Hash{},
				)
				return err
			},
		},
		"test TransactionReceipt": {
			function: func() error {
				_, err := client.TransactionReceipt(
					context.Background(),
					common.Hash{},
				)
				return err
			},
		},
		"test BalanceAt": {
			function: func() error {
				_, err := client.BalanceAt(
					context.Background(),
					common.Address{},
					big.NewInt(0),
				)
				return err
			},
		},
	}
}

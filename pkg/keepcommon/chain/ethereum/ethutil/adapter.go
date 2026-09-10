package ethutil

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	chainEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
)

type ethereumAdapter struct {
	delegate EthereumClient
}

func (ea *ethereumAdapter) HeaderByNumber(
	ctx context.Context,
	number *big.Int,
) (*chainEthereum.Header, error) {
	header, err := ea.delegate.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, err
	}

	return &chainEthereum.Header{
		Number: header.Number,
	}, nil
}

func (ea *ethereumAdapter) SubscribeNewHead(
	ctx context.Context,
	headersChan chan<- *chainEthereum.Header,
) (chainEthereum.Subscription, error) {
	internalHeadersChan := make(chan *types.Header)

	subscription, err := ea.delegate.SubscribeNewHead(ctx, internalHeadersChan)
	if err != nil {
		return nil, err
	}

	stop := make(chan struct{})

	go func() {
		for {
			select {
			case header := <-internalHeadersChan:
				headersChan <- &chainEthereum.Header{
					Number: header.Number,
				}
			case <-stop:
				return
			}
		}
	}()

	return &subscriptionWrapper{
		unsubscribeFn: func() {
			close(stop)
			subscription.Unsubscribe()
		},
		errChan: subscription.Err(),
	}, nil
}

type subscriptionWrapper struct {
	unsubscribeFn func()
	errChan       <-chan error
}

func (sw *subscriptionWrapper) Unsubscribe() {
	sw.unsubscribeFn()
}

func (sw *subscriptionWrapper) Err() <-chan error {
	return sw.errChan
}

func (ea *ethereumAdapter) PendingNonceAt(
	ctx context.Context,
	account chainEthereum.Address,
) (uint64, error) {
	return ea.delegate.PendingNonceAt(ctx, common.Address(account))
}

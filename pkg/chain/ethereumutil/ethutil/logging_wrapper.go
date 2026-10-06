package ethutil

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ipfs/go-log"
	"math/big"
)

type loggingWrapper struct {
	EthereumClient

	logger log.EventLogger
}

func (lw *loggingWrapper) SuggestGasPrice(
	ctx context.Context,
) (*big.Int, error) {
	price, err := lw.EthereumClient.SuggestGasPrice(ctx)

	if err != nil {
		lw.logger.Debugf("error requesting gas price suggestion: [%v]", err)
		return nil, err
	}

	lw.logger.Debugf("received gas price suggestion: [%v]", price)
	return price, nil
}

func (lw *loggingWrapper) EstimateGas(
	ctx context.Context,
	msg ethereum.CallMsg,
) (uint64, error) {
	gas, err := lw.EthereumClient.EstimateGas(ctx, msg)

	if err != nil {
		return 0, err
	}

	lw.logger.Debugf("received gas estimate: [%v]", gas)
	return gas, nil
}

// WrapCallLogging wraps certain call-related methods on the given `client`
// with debug logging sent to the given `logger`. Actual functionality is
// delegated to the passed client.
func WrapCallLogging(
	logger log.EventLogger,
	client EthereumClient,
) EthereumClient {
	return &loggingWrapper{client, logger}
}

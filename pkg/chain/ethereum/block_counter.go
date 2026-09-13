package ethereum

import (
	"context"
	"fmt"

	"github.com/keep-network/keep-core/pkg/chain"
)

func (bc *baseChain) BlockCounter() (chain.BlockCounter, error) {
	return bc.blockCounter, nil
}

// LatestBlockNumber fetches the latest header through the Ethereum RPC client.
// Unlike BlockCounter().CurrentBlock(), it does not read subscription cache
// state, so an unavailable endpoint returns an error even after prior success.
func (bc *baseChain) LatestBlockNumber(ctx context.Context) (uint64, error) {
	header, err := bc.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch latest Ethereum header: [%w]", err)
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() {
		return 0, fmt.Errorf("latest Ethereum header has no valid block number")
	}
	return header.Number.Uint64(), nil
}

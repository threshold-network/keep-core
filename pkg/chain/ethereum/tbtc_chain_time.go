// tbtc_chain_time.go: chain head time read for the TbtcChain adapter.
package ethereum

import (
	"fmt"
	"math"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// GetLatestBlockTimestamp returns the timestamp of the latest Ethereum block
// header, read with a bounded timeout via baseChain.currentBlock. Callers use
// this value instead of time.Now() when comparing against on-chain deadlines
// such as wallet creation periods and reservation timeouts.
func (tc *TbtcChain) GetLatestBlockTimestamp() (time.Time, error) {
	header, err := tc.currentBlock()
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot get the latest block header: %w", err)
	}

	if header.Time > math.MaxInt64 {
		return time.Time{}, fmt.Errorf(
			"block timestamp [%v] does not fit in int64",
			header.Time,
		)
	}

	return time.Unix(int64(header.Time), 0), nil
}

var _ tbtc.BridgeChain = (*TbtcChain)(nil)

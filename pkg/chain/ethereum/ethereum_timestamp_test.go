package ethereum

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
)

// timestampMockClient is a minimal ethutil.EthereumClient used to exercise the
// timestamp-based block search. It embeds the interface so it satisfies the
// full contract while only the two methods used by GetBlockNumberByTimestamp
// are implemented; any other call would panic, which keeps the test honest
// about what the searched code actually touches.
type timestampMockClient struct {
	ethutil.EthereumClient
	// blockTimes maps a block number to its timestamp.
	blockTimes map[uint64]uint64
	// latest is the number of the current (highest) block.
	latest uint64
}

func newTimestampMockClient(baseTime, spacing, latest uint64) *timestampMockClient {
	blockTimes := make(map[uint64]uint64)
	for n := uint64(0); n <= latest; n++ {
		blockTimes[n] = baseTime + n*spacing
	}
	return &timestampMockClient{blockTimes: blockTimes, latest: latest}
}

func (m *timestampMockClient) header(number uint64) (*types.Header, error) {
	time, ok := m.blockTimes[number]
	if !ok {
		return nil, errBlockOutOfRange
	}
	return &types.Header{
		Number: new(big.Int).SetUint64(number),
		Time:   time,
	}, nil
}

// HeaderByNumber returns the latest header when number is nil, matching the
// behavior currentBlock relies on.
func (m *timestampMockClient) HeaderByNumber(
	_ context.Context,
	number *big.Int,
) (*types.Header, error) {
	if number == nil {
		return m.header(m.latest)
	}
	return m.header(number.Uint64())
}

func (m *timestampMockClient) BlockByNumber(
	_ context.Context,
	number *big.Int,
) (*types.Block, error) {
	header, err := m.header(number.Uint64())
	if err != nil {
		return nil, err
	}
	return types.NewBlockWithHeader(header), nil
}

var errBlockOutOfRange = errors.New("block out of range")

func TestGetBlockNumberByTimestamp(t *testing.T) {
	const (
		baseTime = uint64(1_600_000_000)
		spacing  = uint64(12)
		latest   = uint64(100)
	)
	latestTime := baseTime + latest*spacing

	tests := map[string]struct {
		timestamp      uint64
		expectedBlock  uint64
		expectingError bool
	}{
		"timestamp of the latest block": {
			timestamp:     latestTime,
			expectedBlock: latest,
		},
		"timestamp exactly matching a middle block": {
			timestamp:     baseTime + 50*spacing,
			expectedBlock: 50,
		},
		"timestamp closer to the lower block": {
			// Between block 50 (t=+600) and 51 (t=+612); +605 is 5s from 50
			// and 7s from 51, so the lower block wins.
			timestamp:     baseTime + 50*spacing + 5,
			expectedBlock: 50,
		},
		"timestamp closer to the higher block": {
			// +607 is 7s from 50 and 5s from 51, so the higher block wins.
			timestamp:     baseTime + 50*spacing + 7,
			expectedBlock: 51,
		},
		"timestamp equidistant between two blocks": {
			// +606 is 6s from both 50 and 51; closerBlock returns the greater
			// block number on a tie.
			timestamp:     baseTime + 50*spacing + 6,
			expectedBlock: 51,
		},
		"timestamp of the earliest block": {
			timestamp:     baseTime,
			expectedBlock: 0,
		},
		"timestamp in the future": {
			timestamp:      latestTime + 1,
			expectingError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bc := &baseChain{
				client: newTimestampMockClient(baseTime, spacing, latest),
			}

			block, err := bc.GetBlockNumberByTimestamp(test.timestamp)

			if test.expectingError {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			if block != test.expectedBlock {
				t.Errorf(
					"unexpected block number\nexpected: [%d]\nactual:   [%d]",
					test.expectedBlock,
					block,
				)
			}
		})
	}
}

// TestGetBlockNumberByTimestamp_ForwardCompensation exercises the forward
// compensation loop in GetBlockNumberByTimestamp. When the actual block spacing
// (15s) is greater than the assumed averageBlockTime (13s), the initial backward
// jump overshoots below the target timestamp, so the search must walk forward
// block by block to converge. The main table above uses a 12s spacing, where the
// backward jump always lands at or after the target and the forward loop never
// runs.
func TestGetBlockNumberByTimestamp_ForwardCompensation(t *testing.T) {
	const (
		baseTime = uint64(1_600_000_000)
		spacing  = uint64(15)
		latest   = uint64(100)
	)

	bc := &baseChain{
		client: newTimestampMockClient(baseTime, spacing, latest),
	}

	// Target a point 7s after block 50 (t=+757), between block 50 (t=+750) and
	// block 51 (t=+765). The backward jump from the tip lands on block 43
	// (t=+645, before the target), so the forward loop walks 43->51 and the
	// closer-block tie-break then selects block 50 (7s away vs. 8s).
	timestamp := baseTime + 50*spacing + 7
	expectedBlock := uint64(50)

	block, err := bc.GetBlockNumberByTimestamp(timestamp)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if block != expectedBlock {
		t.Errorf(
			"unexpected block number\nexpected: [%d]\nactual:   [%d]",
			expectedBlock,
			block,
		)
	}
}

func TestCloserBlock(t *testing.T) {
	block := func(number, time uint64) *types.Block {
		return types.NewBlockWithHeader(&types.Header{
			Number: new(big.Int).SetUint64(number),
			Time:   time,
		})
	}

	tests := map[string]struct {
		timestamp      uint64
		b1, b2         *types.Block
		expectedNumber uint64
	}{
		"first block closer": {
			timestamp:      100,
			b1:             block(5, 100),
			b2:             block(6, 110),
			expectedNumber: 5,
		},
		"second block closer": {
			timestamp:      110,
			b1:             block(5, 100),
			b2:             block(6, 110),
			expectedNumber: 6,
		},
		"equidistant returns the greater block number (b2)": {
			timestamp:      105,
			b1:             block(5, 100),
			b2:             block(6, 110),
			expectedNumber: 6,
		},
		"equidistant returns the greater block number (b1)": {
			timestamp:      105,
			b1:             block(6, 110),
			b2:             block(5, 100),
			expectedNumber: 6,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := closerBlock(test.timestamp, test.b1, test.b2)
			if result.NumberU64() != test.expectedNumber {
				t.Errorf(
					"unexpected block number\nexpected: [%d]\nactual:   [%d]",
					test.expectedNumber,
					result.NumberU64(),
				)
			}
		})
	}
}

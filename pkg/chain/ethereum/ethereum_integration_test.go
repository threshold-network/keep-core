//go:build integration
// +build integration

package ethereum

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/internal/testutils"
)

// TODO: Include integration test in the CI.
// To run the tests execute `go test -v -tags=integration ./...`

const ethereumURL = "https://mainnet.infura.io/v3/f41c6e3d505d44c182a5e5adefdaa43f"

func TestBaseChain_GetBlockNumberByTimestamp(t *testing.T) {
	client, err := ethclient.Dial(ethereumURL)
	if err != nil {
		t.Fatal(err)
	}

	blockCounter, err := ethutil.NewBlockCounter(client)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize the baseChain with fields required by this scenario.
	bc := &baseChain{
		client:       client,
		blockCounter: blockCounter,
	}

	var tests = map[string]struct {
		timestamp           uint64
		expectedBlockNumber uint64
		expectedError       error
	}{
		"there is a block at the requested timestamp": {
			timestamp:           1681982135, // 20 April 2023 09:15:35
			expectedBlockNumber: 17086765,
		},
		"there is a block just after the requested timestamp": {
			timestamp:           1681982133, // 20 April 2023 09:15:33
			expectedBlockNumber: 17086765,
		},
		"there is a block just before the requested timestamp": {
			timestamp:           1681982125, // 20 April 2023 09:15:25
			expectedBlockNumber: 17086764,
		},
		"there are two blocks with equal distance to the requested timestamp": {
			timestamp:           1681982129, // 20 April 2023 09:15:29
			expectedBlockNumber: 17086765,
		},
		"the requested timestamp is in the future": {
			timestamp:     uint64(time.Now().Add(1 * time.Hour).Unix()),
			expectedError: fmt.Errorf("requested timestamp is in the future"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			blockNumber, err := bc.GetBlockNumberByTimestamp(test.timestamp)
			if isProviderRateLimitError(err) {
				t.Skipf("skipping test due to Ethereum provider rate limit: [%v]", err)
			}

			if !reflect.DeepEqual(err, test.expectedError) {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]\n",
					test.expectedError,
					err,
				)
			}

			testutils.AssertIntsEqual(
				t,
				"block number",
				int(test.expectedBlockNumber),
				int(blockNumber),
			)
		})
	}
}

func isProviderRateLimitError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "429 Too Many Requests")
}

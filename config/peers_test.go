package config

import (
	"reflect"
	"testing"

	"golang.org/x/exp/slices"

	"github.com/keep-network/keep-core/config/network"
)

func TestResolvePeers(t *testing.T) {
	var tests = map[string]struct {
		network       network.Type
		expectedPeers []string
		expectedError error
	}{
		"mainnet network": {
			network: network.Mainnet,
			expectedPeers: []string{
				"/ip4/143.198.18.229/tcp/3919/ipfs/16Uiu2HAmDP4Z6LCogRMictJ6deGs4DRo99A5JTz5u3CLMg7URxC6",
			}},
		"sepolia network": {
			network: network.Testnet,
			expectedPeers: []string{
				"/dns4/keep-operator-1.test.keep-nodes.io/tcp/3920/ipfs/16Uiu2HAmDrk2Bh4VNPUJfKRHTE2CvH9xfKzN4KFnmRJbGLkJFDqL",
				"/dns4/keep-operator-2.test.keep-nodes.io/tcp/3920/ipfs/16Uiu2HAm3ex8rGzwFpWYbRreRUiX9JEYCKxp7KDMzB8RZ6fQWnMa",
			},
		},
		"developer network": {
			network: network.Developer,
		},
		"unknown network": {
			network: network.Unknown,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			cfg := &Config{}

			err := cfg.resolvePeers(test.network)
			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\nexpected: %+v\nactual:   %+v\n",
					test.expectedError,
					err,
				)
			}

			for _, expectedPeer := range test.expectedPeers {
				if !slices.Contains(cfg.LibP2P.Peers, expectedPeer) {
					t.Errorf(
						"expected peer %v is not included in the resolved peers list: %v",
						expectedPeer,
						cfg.LibP2P.Peers,
					)
				}
			}
		})
	}
}

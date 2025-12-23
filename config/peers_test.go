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
			network:       network.Mainnet,
			expectedPeers: []string{},
		},
		"sepolia network": {
			network:       network.Testnet,
			expectedPeers: []string{},
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

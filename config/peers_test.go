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
				"/dns4/bst-a01.tbtc.boar.network/tcp/5001/ipfs/16Uiu2HAmAmCrLuUmnBgpavU8y8JBUN6jWAQ93JwydZy3ABRyY6wU",
				"/dns4/bst-b01.tbtc.boar.network/tcp/5001/ipfs/16Uiu2HAm4w5HdJQxBnadGRepaiGfWVvtMzhdAGZVcrf9i71mv69V",
			}},
		"sepolia network": {
			network: network.Testnet,
			expectedPeers: []string{
				"/dns4/bst-a01.test.keep.boar.network/tcp/6001/ipfs/16Uiu2HAmSLDSahiKyTbCNNu8wJmZAsiKF7wuYJ8mogY8ZuAG1jhu",
				"/dns4/keep-validator-0.eks-ap-northeast-2-secure.staging.staked.cloud/tcp/3919/ipfs/16Uiu2HAm77eSvRq5ioD4J8VFPkq3bJHBEHkssCuiFkgAoABwjo2S",
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

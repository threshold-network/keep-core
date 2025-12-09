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
				"/dns4/keep-validator-0.prod-eks-eu-west-1.staked.cloud/tcp/3919/ipfs/16Uiu2HAm6Fs6Fn71n7PqRmpHMbfMkZUCGYhW5RL81MSMg57AANkZ",
				"/dns4/keep-validator-1.prod-eks-ap-northeast-2.staked.cloud/tcp/3919/ipfs/16Uiu2HAm5UzZb1TTYBjb2959h4z4VHzjt585SQqZnJPBrDnJuob7",
				"/dns4/keep-validator-2.prod-eks-eu-north-1.staked.cloud/tcp/3919/ipfs/16Uiu2HAmJvbYNhzY6a8kiG2zzrqXGnYWax7CQTbiMHoAvY4qLvg7",
			}},
		"sepolia network": {
			network: network.Testnet,
			expectedPeers: []string{
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

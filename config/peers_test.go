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
				"/dns4/keep-operator-1.keep-nodes.io/tcp/3919/ipfs/16Uiu2HAmVUxCz2YjBpGaGirVLx6RGtHbPg5rygEWMPoUFE4bHTkr",
				"/dns4/keep-operator-2.keep-nodes.io/tcp/3919/ipfs/16Uiu2HAm8bLqTcGMDFaNPGPC6gxStKCnJr2DaVsMbce1ZEyaKo9S",
				"/dns4/keep-operator-3.keep-nodes.io/tcp/3919/ipfs/16Uiu2HAmQLCwPnNmFMDQkc5hLfapGKtXPvFJQKB3rUFYa1wjVnfi",
				"/dns4/keep-operator-4.keep-nodes.io/tcp/3919/ipfs/16Uiu2HAmTv4atEFadTVPz7BWhE3gRFMeJ5Kk4LQfgN2V8ViWYFRx",
				"/dns4/keep-operator-5.keep-nodes.io/tcp/3919/ipfs/16Uiu2HAmPwQuywYq9qFRn8gLCtiKaDZwg2u3JQhWia7RYHRdfk1r",
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

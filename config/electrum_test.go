package config

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"
)

func TestResolveElectrum(t *testing.T) {
	var tests = map[bitcoin.Network]struct {
		expectedConfig []electrum.Config
		expectedError  error
	}{
		bitcoin.Mainnet: {
			expectedConfig: []electrum.Config{
				{
					URL: "wss://electrum.boar.network:2083",
				},
			},
		},
		bitcoin.Testnet: {
			expectedConfig: []electrum.Config{
				{
					URL: "wss://electrum.testnet.boar.network:443/QxbJgaSLUHqrgAa9BW7bDpnGPxrlhnCa",
				},
			},
		},
		bitcoin.Regtest: {
			expectedConfig: []electrum.Config{
				{
					URL:               "",
					KeepAliveInterval: 0,
				},
			},
		},
		bitcoin.Unknown: {
			expectedConfig: []electrum.Config{
				{
					URL:               "",
					KeepAliveInterval: 0,
				},
			},
		},
	}

	for bitcoinNetwork, test := range tests {
		t.Run(bitcoinNetwork.String(), func(t *testing.T) {
			for i, expectedConfig := range test.expectedConfig {
				rand := rand.New(&fakeRandSource{int64(i)})

				cfg := &Config{}
				cfg.Bitcoin.Network = bitcoinNetwork

				err := cfg.resolveElectrum(rand)
				if !reflect.DeepEqual(test.expectedError, err) {
					t.Errorf(
						"unexpected error\nexpected: %+v\nactual:   %+v\n",
						test.expectedError,
						err,
					)
				}

				resolvedConfig := cfg.Bitcoin.Electrum

				if diff := deep.Equal(resolvedConfig, expectedConfig); diff != nil {
					t.Errorf("compare failed: %v", diff)
				}
			}
		})
	}
}

type fakeRandSource struct {
	expectedValue int64
}

func (s *fakeRandSource) Int63() int64 {
	return s.expectedValue << 32
}
func (s *fakeRandSource) Seed(expectedValue int64) {
	s.expectedValue = expectedValue
}

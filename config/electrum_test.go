package config

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

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
					URL:     "wss://electrum.boar.network:2083",
					Network: bitcoin.Mainnet,
				},
			},
		},
		bitcoin.Testnet: {
			expectedConfig: []electrum.Config{
				{
					URL:     "wss://electrum.testnet.boar.network:443/QxbJgaSLUHqrgAa9BW7bDpnGPxrlhnCa",
					Network: bitcoin.Testnet,
				},
			},
		},
		bitcoin.Testnet4: {
			expectedConfig: []electrum.Config{
				{
					URL:     "ssl://mempool.space:40002",
					Network: bitcoin.Testnet4,
				},
			},
		},
		bitcoin.Regtest: {
			expectedConfig: []electrum.Config{
				{
					URL:               "",
					KeepAliveInterval: 0,
					Network:           bitcoin.Regtest,
				},
			},
		},
		bitcoin.Unknown: {
			expectedConfig: []electrum.Config{
				{
					URL:               "",
					KeepAliveInterval: 0,
					Network:           bitcoin.Unknown,
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

func TestSelectElectrumServerRetainsAlternatives(t *testing.T) {
	cfg := &Config{}
	cfg.Bitcoin.Electrum.RequestTimeout = time.Second
	err := cfg.selectElectrumServer([]string{"first", "second", "third"}, rand.New(&fakeRandSource{1}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bitcoin.Electrum.URL != "second" ||
		!reflect.DeepEqual(cfg.Bitcoin.Electrum.FallbackURLs, []string{"first", "third"}) {
		t.Fatalf("unexpected server pool: %+v", cfg.Bitcoin.Electrum)
	}
	if cfg.Bitcoin.Electrum.RequestTimeout != time.Second {
		t.Fatal("connection settings were overwritten")
	}
}

func TestResolveElectrumExplicitURL(t *testing.T) {
	cfg := &Config{}
	cfg.Bitcoin.Network = bitcoin.Mainnet
	cfg.Bitcoin.Electrum.URL = "ssl://private.example:50002"
	if err := cfg.resolveElectrum(rand.New(&fakeRandSource{})); err != nil {
		t.Fatal(err)
	}
	if cfg.Bitcoin.Electrum.URL != "ssl://private.example:50002" || len(cfg.Bitcoin.Electrum.FallbackURLs) != 0 {
		t.Fatalf("explicit URL acquired automatic fallbacks: %+v", cfg.Bitcoin.Electrum)
	}
}

func TestSelectElectrumServerEmptyList(t *testing.T) {
	if err := new(Config).selectElectrumServer(nil, rand.New(&fakeRandSource{})); err == nil {
		t.Fatal("expected error for empty server pool")
	}
}

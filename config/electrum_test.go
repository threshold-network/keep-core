package config

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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
	if cfg.Bitcoin.Electrum.URL != "second" {
		t.Fatalf("unexpected selected server: %+v", cfg.Bitcoin.Electrum)
	}
	gotFallbacks := append([]string{}, cfg.Bitcoin.Electrum.FallbackURLs...)
	sort.Strings(gotFallbacks)
	if !reflect.DeepEqual(gotFallbacks, []string{"first", "third"}) {
		t.Fatalf("unexpected server pool: %+v", cfg.Bitcoin.Electrum)
	}
	if cfg.Bitcoin.Electrum.RequestTimeout != time.Second {
		t.Fatal("connection settings were overwritten")
	}
}

// TestSelectElectrumServerShufflesFallbacks proves that the fallback order is
// derived from the injected rng instead of always retaining the embedded
// list order. With a fixed seed the resulting order is deterministic.
func TestSelectElectrumServerShufflesFallbacks(t *testing.T) {
	cfg := &Config{}
	err := cfg.selectElectrumServer([]string{"first", "second", "third"}, rand.New(&fakeRandSource{1}))
	if err != nil {
		t.Fatal(err)
	}
	// The un-shuffled list order (excluding the selected primary "second")
	// would be ["first", "third"]; the seeded rng shuffles it to
	// ["third", "first"].
	if !reflect.DeepEqual(cfg.Bitcoin.Electrum.FallbackURLs, []string{"third", "first"}) {
		t.Fatalf("expected shuffled fallback order, got: %v", cfg.Bitcoin.Electrum.FallbackURLs)
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

// TestResolveElectrumMultiURLFile exercises the same parsing pipeline
// readElectrumUrls uses (split-on-newline plus cleanStrings) against a
// temporary file modeled on config/_electrum_urls/testnet4's format (leading
// comments and a blank line), then resolves it through selectElectrumServer
// exactly as resolveElectrum would.
func TestResolveElectrumMultiURLFile(t *testing.T) {
	content := "# Default Electrum URLs for Bitcoin testnet4 (one is chosen at random at startup).\n" +
		"# Override with --bitcoin.electrum.url or bitcoin.electrum.url in the config file.\n" +
		"\n" +
		"ssl://first.example:50002\n" +
		"ssl://second.example:50002\n" +
		"ssl://third.example:50002\n"

	path := filepath.Join(t.TempDir(), "testnet4")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	urls := cleanStrings(strings.Split(string(file), "\n"))

	expectedURLs := []string{
		"ssl://first.example:50002",
		"ssl://second.example:50002",
		"ssl://third.example:50002",
	}
	if !reflect.DeepEqual(urls, expectedURLs) {
		t.Fatalf("comments/blank lines leaked into parsed URLs: %v", urls)
	}

	cfg := &Config{}
	cfg.Bitcoin.Network = bitcoin.Testnet4
	if err := cfg.selectElectrumServer(urls, rand.New(&fakeRandSource{1})); err != nil {
		t.Fatal(err)
	}

	if cfg.Bitcoin.Electrum.URL == "" {
		t.Fatal("no primary URL was selected")
	}
	pool := append([]string{cfg.Bitcoin.Electrum.URL}, cfg.Bitcoin.Electrum.FallbackURLs...)
	sort.Strings(pool)
	if !reflect.DeepEqual(pool, expectedURLs) {
		t.Fatalf(
			"selected server pool does not match parsed URLs: url=%s fallback=%v",
			cfg.Bitcoin.Electrum.URL, cfg.Bitcoin.Electrum.FallbackURLs,
		)
	}
	if len(cfg.Bitcoin.Electrum.FallbackURLs) != len(expectedURLs)-1 {
		t.Fatalf("unexpected fallback count: %v", cfg.Bitcoin.Electrum.FallbackURLs)
	}
}

// TestSelectElectrumServerHonorsOperatorFallbacks guards the operator arm path
// for failover: an operator-supplied fallback list must survive auto-selection
// instead of being overwritten by the embedded candidate remainder.
func TestSelectElectrumServerHonorsOperatorFallbacks(t *testing.T) {
	t.Run("operator list replaces embedded remainder", func(t *testing.T) {
		cfg := &Config{}
		cfg.Bitcoin.Electrum.FallbackURLs = []string{
			"wss://op1.example:443",
			"wss://op2.example:443",
		}

		embedded := []string{
			"wss://embedded1.example:443",
			"wss://embedded2.example:443",
		}
		if err := cfg.selectElectrumServer(embedded, rand.New(&fakeRandSource{1})); err != nil {
			t.Fatal(err)
		}

		expected := []string{"wss://op1.example:443", "wss://op2.example:443"}
		if !reflect.DeepEqual(cfg.Bitcoin.Electrum.FallbackURLs, expected) {
			t.Fatalf(
				"operator fallbacks were not retained\nexpected: %v\nactual:   %v",
				expected,
				cfg.Bitcoin.Electrum.FallbackURLs,
			)
		}
	})

	t.Run("selected primary is deduped from operator list", func(t *testing.T) {
		cfg := &Config{}
		cfg.Bitcoin.Electrum.FallbackURLs = []string{
			"wss://embedded1.example:443", // equals the only embedded candidate
			"wss://op1.example:443",
		}

		if err := cfg.selectElectrumServer(
			[]string{"wss://embedded1.example:443"},
			rand.New(&fakeRandSource{1}),
		); err != nil {
			t.Fatal(err)
		}

		expected := []string{"wss://op1.example:443"}
		if !reflect.DeepEqual(cfg.Bitcoin.Electrum.FallbackURLs, expected) {
			t.Fatalf(
				"selected primary not deduped from operator fallbacks\nexpected: %v\nactual:   %v",
				expected,
				cfg.Bitcoin.Electrum.FallbackURLs,
			)
		}
	})
}

// TestResolveElectrumKeepsOperatorFallbacks covers the full resolution path:
// operator fallbacks set with no explicit URL survive auto-selection, and an
// explicit URL keeps both itself (pinned) and the operator fallbacks.
func TestResolveElectrumKeepsOperatorFallbacks(t *testing.T) {
	t.Run("auto-selected primary keeps operator fallbacks", func(t *testing.T) {
		cfg := &Config{}
		cfg.Bitcoin.Network = bitcoin.Mainnet
		cfg.Bitcoin.Electrum.FallbackURLs = []string{"wss://op1.example:443"}

		if err := cfg.resolveElectrum(rand.New(&fakeRandSource{1})); err != nil {
			t.Fatal(err)
		}

		if cfg.Bitcoin.Electrum.URL != "wss://electrum.boar.network:2083" {
			t.Fatalf("unexpected primary: %v", cfg.Bitcoin.Electrum.URL)
		}

		expected := []string{"wss://op1.example:443"}
		if !reflect.DeepEqual(cfg.Bitcoin.Electrum.FallbackURLs, expected) {
			t.Fatalf(
				"operator fallbacks lost in auto-select path\nexpected: %v\nactual:   %v",
				expected,
				cfg.Bitcoin.Electrum.FallbackURLs,
			)
		}
	})

	t.Run("explicit URL stays pinned with operator fallbacks", func(t *testing.T) {
		cfg := &Config{}
		cfg.Bitcoin.Network = bitcoin.Mainnet
		cfg.Bitcoin.Electrum.URL = "ssl://explicit.example:50002"
		cfg.Bitcoin.Electrum.FallbackURLs = []string{"ssl://backup.example:50002"}

		if err := cfg.resolveElectrum(rand.New(&fakeRandSource{1})); err != nil {
			t.Fatal(err)
		}

		if cfg.Bitcoin.Electrum.URL != "ssl://explicit.example:50002" {
			t.Fatalf("explicit URL was unpinned: %v", cfg.Bitcoin.Electrum.URL)
		}

		expected := []string{"ssl://backup.example:50002"}
		if !reflect.DeepEqual(cfg.Bitcoin.Electrum.FallbackURLs, expected) {
			t.Fatalf(
				"operator fallbacks lost with explicit URL\nexpected: %v\nactual:   %v",
				expected,
				cfg.Bitcoin.Electrum.FallbackURLs,
			)
		}
	})
}

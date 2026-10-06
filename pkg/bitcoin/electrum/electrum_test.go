package electrum

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/checksum0/go-electrum/electrum"
	"github.com/ipfs/go-log"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestFeeEstimateWithFallbackTargets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		primary uint32
		want    []uint32
	}{
		{
			name:    "primary 1 tries common confirmation horizons",
			primary: 1,
			want: []uint32{
				1, 6, 25, 50, 100, 144, 500, 1008,
			},
		},
		{
			name:    "dedup when primary is 25",
			primary: 25,
			want: []uint32{
				25, 6, 50, 100, 144, 500, 1008,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := feeEstimateWithFallbackTargets(tc.primary)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestConvertBtcKbToSatVByte(t *testing.T) {
	var tests = map[string]struct {
		btcPerKbFee            float32
		expectedSatPerVByteFee int64
	}{
		"BTC/KB is negative": {
			btcPerKbFee:            -1,
			expectedSatPerVByteFee: 1,
		},
		"BTC/KB is 0": {
			btcPerKbFee:            0,
			expectedSatPerVByteFee: 1,
		},
		"BTC/KB is 0.000001": {
			btcPerKbFee:            0.000001,
			expectedSatPerVByteFee: 1,
		},
		"BTC/KB is 0.00001": {
			btcPerKbFee:            0.00001,
			expectedSatPerVByteFee: 1,
		},
		"BTC/KB is 0.00002": {
			btcPerKbFee:            0.00002,
			expectedSatPerVByteFee: 2,
		},
		"BTC/KB is 0.0001": {
			btcPerKbFee:            0.0001,
			expectedSatPerVByteFee: 10,
		},
		"BTC/KB is 0.001": {
			btcPerKbFee:            0.001,
			expectedSatPerVByteFee: 100,
		},
		"BTC/KB is 0.0012350": {
			btcPerKbFee:            0.0012350,
			expectedSatPerVByteFee: 123,
		},
		"BTC/KB is 0.0012351": {
			btcPerKbFee:            0.0012351,
			expectedSatPerVByteFee: 124,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			satPerVByteFee := convertBtcKbToSatVByte(test.btcPerKbFee)

			testutils.AssertIntsEqual(
				t,
				"sat/vbyte fee",
				int(test.expectedSatPerVByteFee),
				int(satPerVByteFee),
			)
		})
	}
}

func TestFeeFallbackResult(t *testing.T) {
	t.Parallel()

	oracleFailure := fmt.Errorf("cannot estimate fee")
	transportFailure := fmt.Errorf("request failed: [connection refused]")
	targets := []uint32{1, 6, 25}

	for _, tc := range []struct {
		name                string
		network             bitcoin.Network
		sawFeeOracleFailure bool
		lastErr             error
		wantFee             int64
		wantErr             bool
	}{
		{
			name:                "mainnet oracle failure fails safe",
			network:             bitcoin.Mainnet,
			sawFeeOracleFailure: true,
			lastErr:             oracleFailure,
			wantErr:             true,
		},
		{
			name:                "unknown network oracle failure fails safe",
			network:             bitcoin.Unknown,
			sawFeeOracleFailure: true,
			lastErr:             oracleFailure,
			wantErr:             true,
		},
		{
			name:                "testnet4 oracle failure uses fallback",
			network:             bitcoin.Testnet4,
			sawFeeOracleFailure: true,
			lastErr:             oracleFailure,
			wantFee:             defaultFallbackSatPerVByteWhenEstimateFails,
		},
		{
			name:                "testnet oracle failure uses fallback",
			network:             bitcoin.Testnet,
			sawFeeOracleFailure: true,
			lastErr:             oracleFailure,
			wantFee:             defaultFallbackSatPerVByteWhenEstimateFails,
		},
		{
			name:                "regtest oracle failure uses fallback",
			network:             bitcoin.Regtest,
			sawFeeOracleFailure: true,
			lastErr:             oracleFailure,
			wantFee:             defaultFallbackSatPerVByteWhenEstimateFails,
		},
		{
			name:                "testnet4 transport failure does not use fallback",
			network:             bitcoin.Testnet4,
			sawFeeOracleFailure: false,
			lastErr:             transportFailure,
			wantErr:             true,
		},
		{
			name:                "mainnet transport failure errors",
			network:             bitcoin.Mainnet,
			sawFeeOracleFailure: false,
			lastErr:             transportFailure,
			wantErr:             true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fee, err := feeFallbackResult(
				tc.network,
				tc.sawFeeOracleFailure,
				tc.lastErr,
				targets,
			)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got fee [%d]", fee)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}
			if fee != tc.wantFee {
				t.Fatalf("expected fee [%d], got [%d]", tc.wantFee, fee)
			}
		})
	}
}

func TestUtxoValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		value   uint64
		want    int64
		wantErr bool
	}{
		{name: "zero", value: 0, want: 0},
		{name: "typical amount", value: 123456789, want: 123456789},
		{name: "largest representable int64", value: math.MaxInt64, want: math.MaxInt64},
		{name: "overflows int64", value: math.MaxInt64 + 1, wantErr: true},
		{name: "max uint64", value: math.MaxUint64, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := utxoValue(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error for an out-of-range UTXO value")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected value [%d], got [%d]", tc.want, got)
			}
		})
	}
}

func TestVerifyServerGenesisHash(t *testing.T) {
	t.Parallel()
	const (
		mainnetGenesis = "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26"
		testnetGenesis = "000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943"
	)

	for _, tc := range []struct {
		name                string
		expectedGenesisHash string
		serverGenesisHash   string
		wantErr             bool
		wantGenesisHash     string
	}{
		{
			name:                "first server adopts its genesis hash",
			expectedGenesisHash: "",
			serverGenesisHash:   mainnetGenesis,
			wantGenesisHash:     mainnetGenesis,
		},
		{
			name:                "matching genesis hash passes, case-insensitively",
			expectedGenesisHash: mainnetGenesis,
			serverGenesisHash:   strings.ToUpper(mainnetGenesis),
			wantGenesisHash:     mainnetGenesis,
		},
		{
			name:                "mismatched genesis hash is rejected",
			expectedGenesisHash: mainnetGenesis,
			serverGenesisHash:   testnetGenesis,
			wantErr:             true,
		},
		{
			name:                "server omitting the field is accepted when a reference is set",
			expectedGenesisHash: mainnetGenesis,
			serverGenesisHash:   "",
			wantErr:             false,
			wantGenesisHash:     mainnetGenesis,
		},
		{
			name:                "first server omitting the field is accepted with no reference adopted",
			expectedGenesisHash: "",
			serverGenesisHash:   "",
			wantErr:             false,
			wantGenesisHash:     "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &failoverTestClient{
				serverFeatures: func(context.Context) (*electrum.ServerFeaturesResult, error) {
					return &electrum.ServerFeaturesResult{GenesisHash: tc.serverGenesisHash}, nil
				},
			}
			gotHash, err := verifyServer(context.Background(), client, "server", tc.expectedGenesisHash)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotHash != tc.wantGenesisHash {
				t.Fatalf("expected genesis hash [%s], got [%s]", tc.wantGenesisHash, gotHash)
			}
		})
	}
}

// TestTipSanityCheckWarnsButAcceptsLargeHeightChange verifies the tip sanity
// check is advisory only: a large height jump right after a reconnect (a
// legitimate reorg, or simply a different server's view) is logged, not
// rejected, and the new tip is still returned and tracked.
func TestTipSanityCheckWarnsButAcceptsLargeHeightChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := failoverTestConfig()
	config.KeepAliveInterval = time.Hour
	heights := []int32{100, 100_000}
	call := 0
	client := &failoverTestClient{header: func(context.Context) (*electrum.SubscribeHeadersResult, error) {
		h := heights[call]
		if call < len(heights)-1 {
			call++
		}
		return &electrum.SubscribeHeadersResult{Height: h}, nil
	}}
	connection, err := connect(ctx, config, func(context.Context, string) (electrumClient, error) {
		return client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if height, err := connection.GetLatestBlockHeight(); err != nil || height != 100 {
		t.Fatalf("first height read: %d, %v", height, err)
	}
	if connection.lastTipHeight != 100 {
		t.Fatalf("expected tracked tip height 100, got %d", connection.lastTipHeight)
	}
	// A reconnect (electrumConnect, failover, or healPrimary) sets this flag;
	// simulate one directly to exercise the comparison in isolation.
	connection.clientMutex.Lock()
	connection.pendingTipSanityCheck = true
	connection.clientMutex.Unlock()
	if height, err := connection.GetLatestBlockHeight(); err != nil || height != 100_000 {
		t.Fatalf("a large tip jump after reconnect must be accepted with a warning, not rejected: %d, %v", height, err)
	}
	if connection.lastTipHeight != 100_000 {
		t.Fatalf("expected tracked tip height 100000, got %d", connection.lastTipHeight)
	}
}

// TestGetScriptUtxosPreservesMempoolHeightSentinel verifies getScriptUtxos
// correctly classifies a mempool item reported with Electrum's -1 height
// sentinel under both the confirmed and unconfirmed views.
func TestGetScriptUtxosPreservesMempoolHeightSentinel(t *testing.T) {
	t.Parallel()

	const mempoolTxHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	for _, tc := range []struct {
		name      string
		confirmed bool
		wantLen   int
	}{
		{
			name:      "confirmed view excludes the mempool item",
			confirmed: true,
			wantLen:   0,
		},
		{
			name:      "unconfirmed view includes the mempool item",
			confirmed: false,
			wantLen:   1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &failoverTestClient{
				listUnspent: func(context.Context, string) ([]*electrum.ListUnspentResult, error) {
					return []*electrum.ListUnspentResult{
						{
							Hash:     mempoolTxHash,
							Position: 0,
							Value:    1000,
							Height:   -1,
						},
					}, nil
				},
			}
			connection, err := connect(
				context.Background(),
				failoverTestConfig(),
				func(context.Context, string) (electrumClient, error) {
					return client, nil
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			items, err := connection.getScriptUtxos([]byte{0x00}, tc.confirmed)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != tc.wantLen {
				t.Fatalf("expected [%d] items, got [%d]", tc.wantLen, len(items))
			}
			if tc.wantLen == 1 && items[0].blockHeight != -1 {
				t.Fatalf(
					"expected the mempool sentinel height -1 to be preserved, got [%d]",
					items[0].blockHeight,
				)
			}
		})
	}
}

func TestSanitizeServerURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    string
		expected string
	}{
		"clean URL unchanged": {
			input:    "tcp://electrum.example.com:50001",
			expected: "tcp://electrum.example.com:50001",
		},
		"userinfo stripped": {
			input:    "wss://alice:secret123@electrum.example.com:443/path",
			expected: "wss://electrum.example.com:443/path",
		},
		"preserve @ in credential-free path": {
			input:    "wss://node.example/path@tenant",
			expected: "wss://node.example/path@tenant",
		},
		"userinfo stripped and path @ preserved": {
			input:    "wss://alice:secret123@node.example/path@tenant",
			expected: "wss://node.example/path@tenant",
		},
		"unsupported scheme with userinfo": {
			input:    "unsupported://user:pass@host:123",
			expected: "unsupported://host:123",
		},
		"malformed URL with userinfo": {
			input:    "tcp://user:secret@invalid%xx:50001",
			expected: "tcp://invalid%xx:50001",
		},
		"opaque raw credentials without scheme": {
			input:    "user:secret@host",
			expected: "host",
		},
		"missing host with credentials": {
			input:    "tcp:///user:secret@host",
			expected: "tcp://host",
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			actual := sanitizeServerURL(tc.input)
			if actual != tc.expected {
				t.Fatalf("expected [%s], got [%s]", tc.expected, actual)
			}
		})
	}
}

func TestValidateServerURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input       string
		wantErr     bool
		errContains string
	}{
		"valid tcp": {
			input:   "tcp://electrum.example.com:50001",
			wantErr: false,
		},
		"valid ssl ipv6": {
			input:   "ssl://[::1]:50002",
			wantErr: false,
		},
		"valid ws default port": {
			input:   "ws://127.0.0.1",
			wantErr: false,
		},
		"valid wss with path and @": {
			input:   "wss://node.example/path@tenant",
			wantErr: false,
		},
		"websocket with userinfo rejected": {
			input:       "ws://user:secretpass@127.0.0.1:8546",
			wantErr:     true,
			errContains: "userinfo is not supported in websocket URL [ws://127.0.0.1:8546]",
		},
		"ws with explicit empty port rejected": {
			input:       "ws://127.0.0.1:",
			wantErr:     true,
			errContains: "invalid port",
		},
		"wss with explicit empty port rejected": {
			input:       "wss://node.example:",
			wantErr:     true,
			errContains: "invalid port",
		},
		"missing scheme": {
			input:       "localhost:50001",
			wantErr:     true,
			errContains: "missing protocol scheme",
		},
		"missing host": {
			input:       "tcp://:50001",
			wantErr:     true,
			errContains: "missing host",
		},
		"missing port for tcp": {
			input:       "tcp://electrum.example.com",
			wantErr:     true,
			errContains: "missing port in electrum server URL [tcp://electrum.example.com]",
		},
		"invalid port number": {
			input:       "ssl://electrum.example.com:99999",
			wantErr:     true,
			errContains: "invalid port [99999]",
		},
		"unsupported scheme with redaction": {
			input:       "http://user:secretpass@example.com:80",
			wantErr:     true,
			errContains: "unsupported electrum server protocol scheme [http] in URL [http://example.com:80]",
		},
		"malformed URL with secret redaction": {
			input:       "tcp://user:mysecretpassword@invalid%xx:50001",
			wantErr:     true,
			errContains: "invalid electrum server URL [tcp://invalid%xx:50001]",
		},
		"opaque raw credentials without scheme": {
			input:       "user:secret@host",
			wantErr:     true,
			errContains: "missing protocol scheme in electrum server URL [host]",
		},
		"missing host with credentials": {
			input:       "tcp:///user:secret@host",
			wantErr:     true,
			errContains: "missing host in electrum server URL [tcp://host]",
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateServerURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input [%s], got nil", tc.input)
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error containing [%s], got [%v]", tc.errContains, err)
				}
				if strings.Contains(tc.input, "secret") && strings.Contains(err.Error(), "secret") {
					t.Fatalf("credential leaked in validation error: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input [%s]: %v", tc.input, err)
				}
			}
		})
	}
}

func TestConnectURLValidation(t *testing.T) {
	t.Parallel()

	t.Run("empty configuration fails before dial", func(t *testing.T) {
		t.Parallel()
		chain, err := Connect(context.Background(), Config{})
		if err == nil || chain != nil {
			t.Fatalf("expected error and nil chain, got: [%v], [%v]", err, chain)
		}
		if !strings.Contains(err.Error(), "no electrum server URLs configured") {
			t.Fatalf("expected 'no electrum server URLs configured', got: [%v]", err)
		}
	})

	t.Run("malformed fallback fails before primary is dialed", func(t *testing.T) {
		t.Parallel()
		config := Config{
			URL:          "tcp://primary.example.com:50001",
			FallbackURLs: []string{"http://bad.fallback:80"},
		}
		chain, err := Connect(context.Background(), config)
		if err == nil || chain != nil {
			t.Fatalf("expected error and nil chain, got: [%v], [%v]", err, chain)
		}
		if !strings.Contains(err.Error(), "unsupported electrum server protocol scheme [http]") {
			t.Fatalf("expected unsupported scheme error for fallback, got: [%v]", err)
		}
	})
}

func TestVerifyServerURLSanitization(t *testing.T) {
	t.Parallel()

	client := &failoverTestClient{
		serverFeatures: func(context.Context) (*electrum.ServerFeaturesResult, error) {
			return &electrum.ServerFeaturesResult{
				GenesisHash: "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
			}, nil
		},
	}

	rawURL := "wss://alice:secretpassword@electrum.example.com:443"
	expectedReference := "1111111111111111111111111111111111111111111111111111111111111111"

	_, err := verifyServer(context.Background(), client, rawURL, expectedReference)
	if err == nil {
		t.Fatal("expected error on genesis hash mismatch, got nil")
	}

	if strings.Contains(err.Error(), "secretpassword") || strings.Contains(err.Error(), "alice") {
		t.Fatalf("credentials leaked in verifyServer error: [%v]", err)
	}
	if !strings.Contains(err.Error(), "wss://electrum.example.com:443") {
		t.Fatalf("expected sanitized URL in verifyServer error, got: [%v]", err)
	}
}

type unsupportedProtocolMockClient struct {
	failoverTestClient
}

func (c *unsupportedProtocolMockClient) ServerVersion(ctx context.Context) (string, string, error) {
	return "test", "9.9", nil
}

func TestURLRedactionInLogs(t *testing.T) {
	core, recorded := observer.New(zap.DebugLevel)
	origLogger := logger
	logger = &log.ZapEventLogger{
		SugaredLogger: *zap.New(core).Sugar(),
	}
	t.Cleanup(func() {
		logger = origLogger
	})

	rawURL := "wss://alice:secretpassword@electrum.example.com:443"

	assertNoCredentialsInLogs := func(t *testing.T, expectedLogSubstring string) {
		t.Helper()
		allEntries := recorded.All()
		found := false
		for _, entry := range allEntries {
			msg := entry.Message
			if strings.Contains(msg, "alice") || strings.Contains(msg, "secretpassword") {
				t.Fatalf("credential leaked in log message: [%s]", msg)
			}
			if strings.Contains(msg, expectedLogSubstring) {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected log containing [%s], found entries: %v", expectedLogSubstring, allEntries)
		}
	}

	t.Run("verifyServer unsupported protocol warning", func(t *testing.T) {
		recorded.TakeAll()
		client := &unsupportedProtocolMockClient{}
		_, _ = verifyServer(context.Background(), client, rawURL, "")
		assertNoCredentialsInLogs(t, "electrum server [wss://electrum.example.com:443] runs an unsupported protocol version")
	})

	t.Run("verifyServer missing genesis hash warning", func(t *testing.T) {
		recorded.TakeAll()
		client := &failoverTestClient{
			serverFeatures: func(context.Context) (*electrum.ServerFeaturesResult, error) {
				return &electrum.ServerFeaturesResult{GenesisHash: ""}, nil
			},
		}
		_, _ = verifyServer(context.Background(), client, rawURL, "")
		assertNoCredentialsInLogs(t, "electrum server [wss://electrum.example.com:443] did not report a genesis hash")
	})

	t.Run("verifyServer genesis adoption warning", func(t *testing.T) {
		recorded.TakeAll()
		client := &failoverTestClient{
			serverFeatures: func(context.Context) (*electrum.ServerFeaturesResult, error) {
				return &electrum.ServerFeaturesResult{
					GenesisHash: "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
				}, nil
			},
		}
		_, _ = verifyServer(context.Background(), client, rawURL, "")
		assertNoCredentialsInLogs(t, "adopting genesis hash [000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f] reported by electrum server [wss://electrum.example.com:443]")
	})

	t.Run("healPrimary re-canonicalization info", func(t *testing.T) {
		recorded.TakeAll()
		ctx, cancel := context.WithCancel(context.Background())
		primaryClient := &failoverTestClient{
			serverFeatures: func(context.Context) (*electrum.ServerFeaturesResult, error) {
				return &electrum.ServerFeaturesResult{
					GenesisHash: "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
				}, nil
			},
		}
		conn := &Connection{
			parentCtx:           ctx,
			serverURLs:          []string{"wss://alice:secretpassword@primary.example:443", "wss://fallback.example:443"},
			serverIndex:         1,
			verifiedGenesisHash: "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
			clientMutex:         &sync.Mutex{},
			config:              Config{ConnectTimeout: time.Second, RequestTimeout: time.Second},
			newClient: func(context.Context, string) (electrumClient, error) {
				return primaryClient, nil
			},
		}
		t.Cleanup(func() {
			cancel()
			if conn.closeClient != nil {
				conn.closeClient()
				conn.closeClient = nil
			}
		})
		conn.healPrimary()
		assertNoCredentialsInLogs(t, "re-canonicalized primary electrum server [wss://primary.example:443]")
	})
}

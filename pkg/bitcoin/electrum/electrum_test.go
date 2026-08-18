package electrum

import (
	"fmt"
	"reflect"
	"testing"

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

func TestSelectLatestUniqueTxHashes(t *testing.T) {
	t.Parallel()

	hash := func(marker byte) bitcoin.Hash {
		var txHash bitcoin.Hash
		txHash[0] = marker
		return txHash
	}

	first := hash(0x01)
	second := hash(0x02)
	third := hash(0x03)

	tests := map[string]struct {
		txHashes []bitcoin.Hash
		limit    int
		expected []bitcoin.Hash
	}{
		"negative limit": {
			txHashes: []bitcoin.Hash{first, second},
			limit:    -1,
			expected: []bitcoin.Hash{},
		},
		"zero limit": {
			txHashes: []bitcoin.Hash{first, second},
			limit:    0,
			expected: []bitcoin.Hash{},
		},
		"no hashes": {
			txHashes: []bitcoin.Hash{},
			limit:    5,
			expected: []bitcoin.Hash{},
		},
		"duplicates deduplicated within limit": {
			txHashes: []bitcoin.Hash{first, second, first, second},
			limit:    5,
			expected: []bitcoin.Hash{first, second},
		},
		"deduplication happens before the limit is applied": {
			// Without dedup-before-limit, the duplicated first hash would
			// consume one of the two available slots.
			txHashes: []bitcoin.Hash{first, first, second},
			limit:    2,
			expected: []bitcoin.Hash{first, second},
		},
		"more unique hashes than limit keeps the latest ones in order": {
			txHashes: []bitcoin.Hash{first, second, third},
			limit:    2,
			expected: []bitcoin.Hash{second, third},
		},
		"fewer unique hashes than limit": {
			txHashes: []bitcoin.Hash{first, second},
			limit:    5,
			expected: []bitcoin.Hash{first, second},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			actual := selectLatestUniqueTxHashes(test.txHashes, test.limit)
			if !reflect.DeepEqual(test.expected, actual) {
				t.Fatalf(
					"unexpected selection\nexpected: [%v]\nactual:   [%v]",
					test.expected,
					actual,
				)
			}
		})
	}
}

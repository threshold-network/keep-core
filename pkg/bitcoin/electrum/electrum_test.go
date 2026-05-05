package electrum

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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

func TestIsElectrumFeeOracleFailure(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner cause")
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"plain error", errors.New("plain"), false},
		{"feeOracleFailure direct", feeOracleFailure{inner}, true},
		{"feeOracleFailure wrapped", fmt.Errorf("outer: %w", feeOracleFailure{inner}), true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isElectrumFeeOracleFailure(tc.err)
			if got != tc.want {
				t.Fatalf("isElectrumFeeOracleFailure(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestFeeOracleFallback_MainnetRefuses(t *testing.T) {
	targets := []uint32{1, 6, 25}
	lastErr := errors.New("cannot estimate fee")
	fee, err := feeOracleFallback(bitcoin.Mainnet, targets, lastErr)
	if err == nil {
		t.Fatal("expected error on mainnet, got nil")
	}
	if fee != 0 {
		t.Fatalf("expected 0 fee on mainnet error, got %d", fee)
	}
	if !strings.Contains(err.Error(), "refusing static fallback on mainnet") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFeeOracleFallback_NonMainnetReturnsFallback(t *testing.T) {
	targets := []uint32{1, 6, 25}
	lastErr := errors.New("cannot estimate fee")
	for _, network := range []bitcoin.Network{bitcoin.Testnet4} {
		fee, err := feeOracleFallback(network, targets, lastErr)
		if err != nil {
			t.Fatalf("expected no error on %v, got: %v", network, err)
		}
		if fee != defaultFallbackSatPerVByteWhenEstimateFails {
			t.Errorf(
				"expected fallback %d on %v, got %d",
				defaultFallbackSatPerVByteWhenEstimateFails,
				network,
				fee,
			)
		}
	}
}

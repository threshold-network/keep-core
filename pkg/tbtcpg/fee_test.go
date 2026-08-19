package tbtcpg

import (
	"math"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// withWalletTxFeePolicy saves the current wallet-tx fee-floor policy
// (floor, buffer ratio) and registers a t.Cleanup that restores it.
// Tests overriding the canonical pkg/tbtc vars MUST use this helper so
// later tests in the same package see the production defaults.
func withWalletTxFeePolicy(t *testing.T) {
	t.Helper()
	originalFloor := tbtc.MinWalletTxSatPerVByteFee
	originalNum := tbtc.WalletTxFeeBufferNumerator
	originalDen := tbtc.WalletTxFeeBufferDenominator
	t.Cleanup(func() {
		tbtc.MinWalletTxSatPerVByteFee = originalFloor
		tbtc.WalletTxFeeBufferNumerator = originalNum
		tbtc.WalletTxFeeBufferDenominator = originalDen
	})
}

func TestApplyWalletTxFeeFloor(t *testing.T) {
	const vsize = 200

	tests := map[string]struct {
		estimatedFee        int64
		txVsize             int64
		maxTotalFee         uint64
		expectedFee         int64
		expectErrorContains string
	}{
		"estimate above the floor is buffered by 25%": {
			estimatedFee: 4000, // rate 20 sat/vByte
			txVsize:      vsize,
			maxTotalFee:  100000,
			expectedFee:  5000, // ceil(20*1.25)=25 sat/vByte * 200
		},
		"low estimate is raised to the minimum floor": {
			estimatedFee: vsize, // rate 1 sat/vByte
			txVsize:      vsize,
			maxTotalFee:  100000,
			expectedFee:  1000, // max(5, ceil(1*1.25)=2)=5 sat/vByte * 200
		},
		"buffered fee above the cap is bounded to the cap": {
			estimatedFee: 4000, // rate 20 -> buffered 25 sat/vByte * 200 = 5000
			txVsize:      vsize,
			maxTotalFee:  4500, // below the buffered 5000
			expectedFee:  4500,
		},
		"buffered fee exactly at the cap is not clamped": {
			estimatedFee: 4000, // rate 20 -> buffered 25 sat/vByte * 200 = 5000
			txVsize:      vsize,
			maxTotalFee:  5000, // exactly the buffered total
			expectedFee:  5000,
		},
		"minimum floor exactly at the cap is allowed": {
			estimatedFee: 100, // rate 0 -> floored to 5 sat/vByte
			txVsize:      vsize,
			maxTotalFee:  1000, // exactly the 5 sat/vByte floor total (5*200)
			expectedFee:  1000,
		},
		"minimum floor above the cap returns an error": {
			estimatedFee:        100,
			txVsize:             vsize,
			maxTotalFee:         800, // below the 5 sat/vByte floor (1000)
			expectErrorContains: "minimum safe transaction fee",
		},
		"non-positive virtual size returns an error": {
			estimatedFee:        1000,
			txVsize:             0,
			maxTotalFee:         100000,
			expectErrorContains: "invalid transaction virtual size",
		},
		"negative estimated fee returns an error": {
			estimatedFee:        -1,
			txVsize:             vsize,
			maxTotalFee:         100000,
			expectErrorContains: "invalid estimated fee",
		},
		"implausibly large virtual size returns an error": {
			estimatedFee:        1000,
			txVsize:             maxWalletTxVsize + 1,
			maxTotalFee:         100000,
			expectErrorContains: "implausible transaction virtual size",
		},
		"implausibly large estimated fee returns an error": {
			estimatedFee:        maxWalletTxEstimatedFee + 1,
			txVsize:             vsize,
			maxTotalFee:         100000,
			expectErrorContains: "implausible estimated fee",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fee, err := applyWalletTxFeeFloor(
				tc.estimatedFee,
				tc.txVsize,
				tc.maxTotalFee,
			)

			if tc.expectErrorContains != "" {
				if err == nil {
					t.Fatalf("expected an error, got fee [%d]", fee)
				}
				if !strings.Contains(err.Error(), tc.expectErrorContains) {
					t.Fatalf(
						"expected error containing [%s]; got [%v]",
						tc.expectErrorContains, err,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}
			if fee != tc.expectedFee {
				t.Errorf(
					"unexpected fee\nexpected: [%d]\nactual:   [%d]",
					tc.expectedFee, fee,
				)
			}
		})
	}
}

// TestApplyWalletTxFeeFloor_BufferOverride verifies that the safety buffer
// is driven by the canonical pkg/tbtc WalletTxFeeBufferNumerator /
// WalletTxFeeBufferDenominator vars, not hardcoded constants. A test that
// overrides the vars MUST restore them via t.Cleanup so other tests see
// the production defaults.
func TestApplyWalletTxFeeFloor_BufferOverride(t *testing.T) {
	const vsize = 200
	withWalletTxFeePolicy(t)

	// 50% buffer (Numerator=3, Denominator=2). At rate 20 sat/vByte the
	// buffered rate becomes ceil(20 * 3 / 2) = 30 sat/vByte, total 6000.
	tbtc.WalletTxFeeBufferNumerator = 3
	tbtc.WalletTxFeeBufferDenominator = 2

	fee, err := applyWalletTxFeeFloor(
		4000, // rate 20 sat/vByte
		vsize,
		100000, // well above the buffered 6000
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if fee != 6000 {
		t.Errorf(
			"unexpected fee with 50%% buffer\nexpected: [6000]\nactual:   [%d]",
			fee,
		)
	}

	// Disable the buffer (Numerator=1, Denominator=1). The buffered rate
	// equals the raw rate, so a 20 sat/vByte estimate stays at 20
	// sat/vByte (above the floor), total 4000.
	tbtc.WalletTxFeeBufferNumerator = 1
	tbtc.WalletTxFeeBufferDenominator = 1

	fee, err = applyWalletTxFeeFloor(
		4000,
		vsize,
		100000,
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if fee != 4000 {
		t.Errorf(
			"unexpected fee with buffer disabled\nexpected: [4000]\nactual:   [%d]",
			fee,
		)
	}

	// Non-positive buffer values are rejected so the helper cannot
	// divide by zero or apply a non-positive buffer.
	tbtc.WalletTxFeeBufferNumerator = 0
	tbtc.WalletTxFeeBufferDenominator = 4
	_, err = applyWalletTxFeeFloor(4000, vsize, 100000)
	if err == nil {
		t.Fatalf("expected an error for Numerator=0")
	}
	if !strings.Contains(err.Error(), "invalid wallet tx fee buffer ratio") {
		t.Fatalf(
			"expected error containing [invalid wallet tx fee buffer ratio]; got [%v]",
			err,
		)
	}
}

// TestApplyWalletTxFeeFloor_OverflowGuard verifies that the helper rejects
// configurations whose internal multiplications (rate * Numerator, rate *
// txVsize, floor * txVsize) would overflow int64. The overflow guards are
// checked-arithmetic and are the hard guarantee; the input-cap
// (maxWalletTxVsize / maxWalletTxEstimatedFee) is defense-in-depth that
// can never be reached for sane operator-tuned values, so this test
// exercises the checked-arithmetic path explicitly.
func TestApplyWalletTxFeeFloor_OverflowGuard(t *testing.T) {
	const vsize = 200
	withWalletTxFeePolicy(t)

	// Buffer Numerator close to MaxInt64: rate * Numerator overflows
	// for any non-trivial rate. The helper rejects this rather than
	// silently wrapping around into the buffer math.
	tbtc.WalletTxFeeBufferNumerator = math.MaxInt64
	tbtc.WalletTxFeeBufferDenominator = 1

	_, err := applyWalletTxFeeFloor(
		4000, // rate 20 sat/vByte
		vsize,
		100000,
	)
	if err == nil {
		t.Fatalf("expected overflow error for Numerator=MaxInt64")
	}
	if !strings.Contains(err.Error(), "would overflow when applied with buffer") {
		t.Fatalf(
			"expected error containing [would overflow when applied with buffer]; got [%v]",
			err,
		)
	}

	// Restore sane buffer.
	tbtc.WalletTxFeeBufferNumerator = 5
	tbtc.WalletTxFeeBufferDenominator = 4

	// Floor so high that floor * txVsize would overflow int64. With
	// estimatedFee=0 the raw rate is 0, but the floor forces rate to
	// tbtc.MinWalletTxSatPerVByteFee, which the checked-arithmetic
	// guard catches before any multiplication happens.
	tbtc.MinWalletTxSatPerVByteFee = math.MaxInt64 / 2

	_, err = applyWalletTxFeeFloor(
		0, // raw rate 0
		vsize,
		math.MaxUint64,
	)
	if err == nil {
		t.Fatalf("expected overflow error for huge MinWalletTxSatPerVByteFee")
	}
	if !strings.Contains(err.Error(), "would overflow") {
		t.Fatalf(
			"expected error containing [would overflow]; got [%v]",
			err,
		)
	}
}

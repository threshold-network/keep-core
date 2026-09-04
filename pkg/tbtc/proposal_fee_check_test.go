package tbtc

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ipfs/go-log/v2"
)

// capturingFeeCheckLogger is a test double for log.StandardLogger that
// records every Warnf call so the follower-side soft check can be
// asserted on directly.
type capturingFeeCheckLogger struct {
	warnings []string
}

func (cl *capturingFeeCheckLogger) Warnf(format string, args ...interface{}) {
	cl.warnings = append(cl.warnings, fmt.Sprintf(format, args...))
}

func (cl *capturingFeeCheckLogger) Errorf(format string, args ...interface{}) {}
func (cl *capturingFeeCheckLogger) Infof(format string, args ...interface{})  {}
func (cl *capturingFeeCheckLogger) Debugf(format string, args ...interface{}) {}
func (cl *capturingFeeCheckLogger) Warn(args ...interface{})                  {}
func (cl *capturingFeeCheckLogger) Error(args ...interface{})                 {}
func (cl *capturingFeeCheckLogger) Info(args ...interface{})                  {}
func (cl *capturingFeeCheckLogger) Debug(args ...interface{})                 {}
func (cl *capturingFeeCheckLogger) Fatal(args ...interface{})                 {}
func (cl *capturingFeeCheckLogger) Fatalf(format string, args ...interface{}) {}
func (cl *capturingFeeCheckLogger) Panic(args ...interface{})                 {}
func (cl *capturingFeeCheckLogger) Panicf(format string, args ...interface{}) {}

// TestWarnIfProposedWalletTxFeeBelowBufferedFloor exercises the
// follower-side soft check end-to-end. The buffered threshold is
// derived from the policy vars + txVsize the same way the helper does
// (with big.Int math) so the boundary between "warn" and "no warn"
// is computed, not hardcoded.
func TestWarnIfProposedWalletTxFeeBelowBufferedFloor(t *testing.T) {
	const vsize = 200

	expectedBufferedRate := new(big.Int).Mul(
		big.NewInt(MinWalletTxSatPerVByteFee),
		big.NewInt(100+WalletTxFeeBufferPercent),
	)
	expectedBufferedRate.Add(
		expectedBufferedRate,
		big.NewInt(99),
	)
	expectedBufferedRate.Quo(
		expectedBufferedRate,
		big.NewInt(100),
	)
	expectedMinBufferedFee := new(big.Int).Mul(
		expectedBufferedRate,
		big.NewInt(vsize),
	)

	scenarios := map[string]struct {
		fee        *big.Int
		expectWarn bool
	}{
		"fee below the safe buffered minimum": {
			fee:        new(big.Int).Sub(expectedMinBufferedFee, big.NewInt(1)),
			expectWarn: true,
		},
		"fee at the safe buffered minimum": {
			fee:        expectedMinBufferedFee,
			expectWarn: false,
		},
		"fee above the safe buffered minimum": {
			fee:        new(big.Int).Add(expectedMinBufferedFee, big.NewInt(1000)),
			expectWarn: false,
		},
		"nil fee from a test/mock caller": {
			fee:        nil,
			expectWarn: true,
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			logger := &capturingFeeCheckLogger{}

			warnIfProposedWalletTxFeeBelowBufferedFloor(
				logger,
				MinWalletTxSatPerVByteFee,
				vsize,
				scenario.fee,
				"test",
			)

			gotWarn := len(logger.warnings) > 0
			if gotWarn != scenario.expectWarn {
				t.Errorf(
					"unexpected warning presence for fee [%v]\n"+
						"expected warning: %v\nactual warning:   %v\n"+
						"captured warnings: %v",
					scenario.fee,
					scenario.expectWarn,
					gotWarn,
					logger.warnings,
				)
			}
		})
	}
}

// TestWarnIfProposedWalletTxFeeBelowBufferedFloor_OverflowBoundary locks
// in the big.Int arithmetic path: with a policy that would overflow
// int64 in naive (rate*Numerator or bufferedRate*txVsize) math, the
// helper still computes a correct threshold and warns only for fees
// that are actually below it. The leader-side tbtcpg.applyWalletTxFeeFloor
// rejects the same implausible input with a checked-arithmetic error;
// the follower-side helper has no MaxInt64 ceiling, so a leader that
func TestWarnIfProposedWalletTxFeeBelowBufferedFloor_OverflowBoundary(t *testing.T) {
	const vsize = 200

	originalFloor := MinWalletTxSatPerVByteFee
	originalPercent := WalletTxFeeBufferPercent
	t.Cleanup(func() {
		MinWalletTxSatPerVByteFee = originalFloor
		WalletTxFeeBufferPercent = originalPercent
	})

	// Percent set so numerator (100+Percent) is MaxInt64, so
	// rate * numerator would overflow int64 in naive math. The big.Int
	// path should compute a threshold of satPerVByteFloor * MaxInt64
	// sat/vByte / 100 * vsize vByte = 5 * MaxInt64 / 100 * 200 total,
	// which no int64 fee can ever reach, so every realistic proposal
	// trips the warning.
	MinWalletTxSatPerVByteFee = 5
	WalletTxFeeBufferPercent = math.MaxInt64 - 100

	logger := &capturingFeeCheckLogger{}
	warnIfProposedWalletTxFeeBelowBufferedFloor(
		logger,
		MinWalletTxSatPerVByteFee,
		vsize,
		big.NewInt(1_000_000_000), // 1e9 sat, normal fee
		"test",
	)

	if len(logger.warnings) == 0 {
		t.Errorf(
			"expected a warning for a normal fee under a MaxInt64-buffered " +
				"threshold (the buffered minimum exceeds any int64 fee, so " +
				"every realistic proposal trips the warning); got no warnings",
		)
	}

	// And a nil fee still warns ...
	logger = &capturingFeeCheckLogger{}
	warnIfProposedWalletTxFeeBelowBufferedFloor(
		logger,
		MinWalletTxSatPerVByteFee,
		vsize,
		nil,
		"test",
	)
	if len(logger.warnings) == 0 {
		t.Errorf(
			"expected a warning for nil proposed fee regardless of " +
				"buffered threshold",
		)
	}
}

// Compile-time check that capturingFeeCheckLogger satisfies the
// log.StandardLogger interface used by warnIfProposedWalletTxFeeBelowBufferedFloor.
var _ log.StandardLogger = (*capturingFeeCheckLogger)(nil)

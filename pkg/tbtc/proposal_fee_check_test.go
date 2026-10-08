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

// TestRejectIfProposedWalletTxFeeBelowProtocolMin exercises the follower-side
// hard-reject gate. The reject threshold is ProtocolMinWalletTxSatPerVByteFee *
// (txVsize - 1); the one-vByte allowance covers leader-follower size differences.
func TestRejectIfProposedWalletTxFeeBelowProtocolMin(t *testing.T) {
	scenarios := []struct {
		name        string
		proposedFee *big.Int
		txVsize     int64
		wantErr     bool
	}{
		{
			name:        "nil fee is rejected (below minimum)",
			proposedFee: nil,
			txVsize:     200,
			wantErr:     true,
		},
		{
			name:        "fee exactly at the reject threshold is NOT rejected",
			proposedFee: big.NewInt(ProtocolMinWalletTxSatPerVByteFee * (200 - 1)),
			txVsize:     200,
			wantErr:     false,
		},
		{
			name:        "fee one sat below the reject threshold is rejected",
			proposedFee: big.NewInt(ProtocolMinWalletTxSatPerVByteFee*(200-1) - 1),
			txVsize:     200,
			wantErr:     true,
		},
		{
			name:        "fee one sat above the reject threshold is NOT rejected",
			proposedFee: big.NewInt(ProtocolMinWalletTxSatPerVByteFee*(200-1) + 1),
			txVsize:     200,
			wantErr:     false,
		},
		{
			name:        "zero vsize skips the check (caller surfaces underlying error)",
			proposedFee: nil,
			txVsize:     0,
			wantErr:     false,
		},
		{
			name:        "negative vsize skips the check",
			proposedFee: nil,
			txVsize:     -1,
			wantErr:     false,
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			err := rejectIfProposedWalletTxFeeBelowProtocolMin(
				tc.proposedFee,
				tc.txVsize,
				"test",
			)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for fee=%v, vsize=%d; got nil",
					tc.proposedFee, tc.txVsize)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for fee=%v, vsize=%d: %v",
					tc.proposedFee, tc.txVsize, err)
			}
		})
	}
}

// applyWalletTxFeeFloor mirrors tbtcpg.applyWalletTxFeeFloor
// (pkg/tbtcpg/fee.go): it computes the minimum total fee a leader would
// emit for a wallet tx of the given vsize, given the mempool estimate and
// the operator's floor+buffer policy. This package (tbtc) cannot import
// tbtcpg, so the formula is replicated inline; it cites
// tbtcpg.applyWalletTxFeeFloor for reference.
//
// Formula: max(ceil(estimate * (100 + buffer) / 100), floor) * vsize.
func applyWalletTxFeeFloor(estimate, floor int64, buffer int64, vsize int64) *big.Int {
	// ceil(estimate * (100 + buffer) / 100)
	numerator := estimate * (100 + buffer)
	denom := int64(100)
	ceil := (numerator + denom - 1) / denom
	if ceil < floor {
		ceil = floor
	}
	return big.NewInt(ceil * vsize)
}

// TestRejectIfProposedWalletTxFeeBelowProtocolMin_HonestLeaderOutputNeverRejected
// is the cross-check table test that converts the throwaway evidence/g1
// fee_floor_leader_vs_follower_test.go into a permanent regression test.
// It feeds every estimate from 1 to 10 sat/vB under four valid operator flag
// combinations (floor 5/buffer 25 and 0 and 50, floor 10/buffer 25) into the
// leader's applyWalletTxFeeFloor formula, then asserts that none of the honest
// leader outputs fall below the follower's protocol-minimum reject threshold.
// This proves the reject gate never fires on honest traffic, even in
// quiet-mempool conditions where the leader emits exactly the floor.
//
// A floor below ProtocolMinWalletTxSatPerVByteFee is not an honest
// configuration: applyWalletTxFeePolicy refuses it at startup (see the
// "floor below protocol minimum is rejected" case of
// TestApplyWalletTxFeePolicy), so such a leader cannot exist and is
// deliberately absent from the table.
func TestRejectIfProposedWalletTxFeeBelowProtocolMin_HonestLeaderOutputNeverRejected(
	t *testing.T,
) {
	flagCombos := []struct {
		name   string
		floor  int64
		buffer int64
	}{
		{"floor=5, buffer=25 (default)", 5, 25},
		{"floor=5, buffer=0", 5, 0},
		{"floor=5, buffer=50", 5, 50},
		{"floor=10, buffer=25", 10, 25},
	}

	vSizes := []int64{110, 142, 144, 173, 210, 235, 310} // realistic wallet tx sizes

	for _, combo := range flagCombos {
		for _, estimate := range []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
			for _, vsize := range vSizes {
				leaderFee := applyWalletTxFeeFloor(estimate, combo.floor, combo.buffer, vsize)
				err := rejectIfProposedWalletTxFeeBelowProtocolMin(leaderFee, vsize, "honest leader")
				if err != nil {
					t.Errorf(
						"honest leader fee rejected! flags=%s estimate=%d vsize=%d: "+
							"leaderFee=%v, reject threshold=%d, error=%v",
						combo.name, estimate, vsize, leaderFee,
						ProtocolMinWalletTxSatPerVByteFee*(vsize-1), err,
					)
				}
			}
		}
	}
}

// TestWarnIfProposedWalletTxFeeBelowBareFloor exercises the follower-side
// bare-floor soft check. The warning fires when
// proposedFee < satPerVByteFloor * txVsize, catching degraded fees that
// pass the protocol minimum but fall short of the operator's own floor.
func TestWarnIfProposedWalletTxFeeBelowBareFloor(t *testing.T) {
	const vsize = 200
	bareFloorFee := big.NewInt(MinWalletTxSatPerVByteFee * vsize) // 5 * 200 = 1000

	scenarios := []struct {
		name        string
		proposedFee *big.Int
		wantWarn    bool
	}{
		{
			name:        "nil fee warns",
			proposedFee: nil,
			wantWarn:    true,
		},
		{
			name:        "fee one sat below bare floor warns",
			proposedFee: new(big.Int).Sub(bareFloorFee, big.NewInt(1)),
			wantWarn:    true,
		},
		{
			name:        "fee exactly at bare floor does not warn",
			proposedFee: bareFloorFee,
			wantWarn:    false,
		},
		{
			name:        "fee above bare floor does not warn",
			proposedFee: new(big.Int).Add(bareFloorFee, big.NewInt(1)),
			wantWarn:    false,
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			logger := &capturingFeeCheckLogger{}
			warnIfProposedWalletTxFeeBelowBareFloor(
				logger,
				MinWalletTxSatPerVByteFee,
				vsize,
				tc.proposedFee,
				"test",
			)
			gotWarn := len(logger.warnings) > 0
			if gotWarn != tc.wantWarn {
				t.Errorf("warning mismatch: got %v, want %v (fee=%v)",
					gotWarn, tc.wantWarn, tc.proposedFee)
			}
		})
	}
}

// TestWarnIfProposedWalletTxFeeBelowBareFloor_LargeFloorDoesNotWrap guards
// the bare-floor product: an operator floor whose product with the vsize
// exceeds int64 must still warn on a small fee. With int64 multiplication
// the product wraps to a negative value and every fee looks above it.
func TestWarnIfProposedWalletTxFeeBelowBareFloor_LargeFloorDoesNotWrap(t *testing.T) {
	const vsize = 200
	floor := int64(math.MaxInt64/vsize) + 1

	logger := &capturingFeeCheckLogger{}
	warnIfProposedWalletTxFeeBelowBareFloor(
		logger,
		floor,
		vsize,
		big.NewInt(1000),
		"test",
	)
	if len(logger.warnings) == 0 {
		t.Errorf(
			"expected a warning for fee 1000 below floor %d * %d vByte",
			floor,
			vsize,
		)
	}
}

// TestRejectIfProposedWalletTxFeeBelowProtocolMin_AnchorReanchorSizeBoundaries
// applies the per-path vsize values computed from the btcd weight model to
// lock in the reject thresholds for the reservation anchor and re-anchor paths.
// Anchor: 1 P2WSH input (126-byte redeem) + 1 P2WPKH output → vsize=142.
// Reanchor: 1 P2WPKH input + 1 P2WPKH output → vsize=110.
func TestRejectIfProposedWalletTxFeeBelowProtocolMin_ReservationAnchorReanchorBoundaries(
	t *testing.T,
) {
	scenarios := []struct {
		path         string
		vsize        int64
		rejectThresh int64
	}{
		{"anchor", 142, 5 * (142 - 1)},   // 705
		{"reanchor", 110, 5 * (110 - 1)}, // 545
	}

	for _, tc := range scenarios {
		t.Run(tc.path, func(t *testing.T) {
			// Fee just below reject threshold → rejected
			below := big.NewInt(tc.rejectThresh - 1)
			err := rejectIfProposedWalletTxFeeBelowProtocolMin(below, tc.vsize, tc.path)
			if err == nil {
				t.Errorf("fee %d (just below threshold %d) should be rejected",
					below, tc.rejectThresh)
			}

			// Fee exactly at reject threshold → NOT rejected
			at := big.NewInt(tc.rejectThresh)
			err = rejectIfProposedWalletTxFeeBelowProtocolMin(at, tc.vsize, tc.path)
			if err != nil {
				t.Errorf("fee %d (at threshold %d) should NOT be rejected: %v",
					at, tc.rejectThresh, err)
			}

			// Honest leader minimum for mempool estimate = 1 sat/vB with floor=5:
			// rate = max(ceil(1*125/100),5) = 5, fee = 5*vsize
			honestMin := big.NewInt(5 * tc.vsize)
			err = rejectIfProposedWalletTxFeeBelowProtocolMin(honestMin, tc.vsize, tc.path)
			if err != nil {
				t.Errorf("honest leader minimum %d should NOT be rejected: %v",
					honestMin, err)
			}
		})
	}
}

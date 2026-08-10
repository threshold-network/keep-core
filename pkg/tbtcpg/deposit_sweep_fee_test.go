package tbtcpg_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// sweepVirtualSize returns the estimated virtual size of a sweep transaction
// with the given number of deposit inputs, mirroring the sizing that
// EstimateDepositsSweepFee performs internally: 1 P2WPKH main-UTXO input,
// depositsCount P2WSH deposit inputs, and 1 P2WPKH output. 126 ==
// depositScriptByteSize.
func sweepVirtualSize(t *testing.T, depositsCount int) int64 {
	t.Helper()
	size, err := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddScriptHashInputs(depositsCount, 126, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		t.Fatal(err)
	}
	return size
}

// TestEstimateDepositsSweepFee_MinimumFloorAndBuffer verifies the sweep fee
// logic: a low estimate is raised to the minimum floor, an estimate above the
// floor is buffered by 25%, the buffered fee is bounded by the Bridge maximum,
// and a Bridge maximum below either the raw estimate or the minimum floor
// returns an error rather than silently broadcasting an underpriced sweep. Both
// the informational SatPerVByteFee and the TotalFee actually broadcast on-chain
// are asserted, and multi-deposit sweeps (where transactionSize grows
// sub-linearly while totalMaxFee grows linearly) are exercised on both the happy
// path and the floor-exceeds-cap error branch. A depositsCount of 0 is also
// covered, verifying the max-size-lookup failure is reported with its real
// underlying cause rather than the zero-value size.
func TestEstimateDepositsSweepFee_MinimumFloorAndBuffer(t *testing.T) {
	// Virtual sizes used to pin the cap and the expected total fee (the on-chain
	// value) relative to the fee rate. The cap and expected-total expectations
	// are given as explicit multiples of the size rather than derived from the
	// rounded rate, so a TotalFee bug that rounds back to the expected rate still
	// fails the test.
	size1 := sweepVirtualSize(t, 1)
	size3 := sweepVirtualSize(t, 3)

	tests := map[string]struct {
		depositsCount          int
		estimateSatPerVByte    int64
		perDepositMaxFee       uint64
		sweepMaxSizeErr        error
		expectedSatPerVByteFee int64
		expectedTotalFee       int64
		expectErrorContains    string
	}{
		"low estimate is raised to the minimum floor": {
			depositsCount:          1,
			estimateSatPerVByte:    1,
			perDepositMaxFee:       100000,
			expectedSatPerVByteFee: 5, // max(5, ceil(1*1.25)=2) = 5
			expectedTotalFee:       5 * size1,
		},
		"estimate above the floor is buffered by 25%": {
			depositsCount:          1,
			estimateSatPerVByte:    20,
			perDepositMaxFee:       100000,
			expectedSatPerVByteFee: 25, // ceil(20*1.25) = 25
			expectedTotalFee:       25 * size1,
		},
		"multi-deposit estimate is buffered by 25%": {
			depositsCount:          3,
			estimateSatPerVByte:    20,
			perDepositMaxFee:       100000,
			expectedSatPerVByteFee: 25, // ceil(20*1.25) = 25
			expectedTotalFee:       25 * size3,
		},
		"buffered estimate above the cap is bounded to the cap": {
			depositsCount:       1,
			estimateSatPerVByte: 20,
			// ceil(20*1.25)=25 sat/vByte buffered fee exceeds the 22*size cap,
			// so it is bounded down to the cap (rate 22), not the buffered 25.
			perDepositMaxFee:       uint64(22 * size1),
			expectedSatPerVByteFee: 22,
			expectedTotalFee:       22 * size1, // the cap itself, not 25*size1
		},
		"raw estimate above the cap returns an error": {
			depositsCount:       1,
			estimateSatPerVByte: 30,
			// The raw 30*size fee already exceeds the 10*size cap, so the sweep
			// is uneconomical and the raw-fee check must error before the
			// minimum-floor logic runs. The substring pins this to the
			// raw-exceeds-cap branch, distinguishing it from the floor branch.
			perDepositMaxFee:    uint64(10 * size1),
			expectErrorContains: "estimated fee exceeds the maximum fee",
		},
		"minimum floor above the cap returns an error": {
			depositsCount:       1,
			estimateSatPerVByte: 1,
			// Cap sits below 5*size (the floor) but above the raw fee (1*size),
			// so the minimum-fee check must error rather than lower the fee. The
			// substring pins this to the floor-exceeds-cap branch specifically,
			// distinguishing it from the raw-fee-exceeds-cap error.
			perDepositMaxFee:    uint64(3 * size1),
			expectErrorContains: "minimum safe transaction fee",
		},
		"multi-deposit minimum floor above the cap returns an error": {
			depositsCount:       3,
			estimateSatPerVByte: 1,
			// At N=3 the total cap is 3*size3 (linear in N) while the floor is
			// 5*size3 (scales with the sub-linear tx size), so the floor exceeds
			// the cap and the sweep must error. The raw 1*size3 fee stays under
			// the cap, so the raw-fee check passes and the floor branch is the
			// one exercised.
			perDepositMaxFee:    uint64(size3),
			expectErrorContains: "minimum safe transaction fee",
		},
		"depositsCount of 0 with a failing max size lookup returns the wrapped error": {
			depositsCount: 0,
			// A depositsCount of 0 takes the "estimate for every count" branch,
			// which looks up the max sweep size first. Inject a distinctive
			// underlying error so the assertion below fails if that cause is
			// ever dropped again (the bug this guards against formatted the
			// zero-value max size instead of the real error).
			sweepMaxSizeErr:     errors.New("boom"),
			expectErrorContains: "cannot get sweep max size: [boom]",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			tbtcChain.SetDepositParameters(0, 0, test.perDepositMaxFee, 0)
			if test.sweepMaxSizeErr != nil {
				tbtcChain.SetDepositSweepMaxSizeError(test.sweepMaxSizeErr)
			}

			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, test.estimateSatPerVByte)

			fees, err := tbtcpg.EstimateDepositsSweepFee(
				tbtcChain, btcChain, test.depositsCount,
			)

			if test.expectErrorContains != "" {
				if err == nil {
					t.Fatalf("expected an error, got fee result [%v]", fees)
				}
				if !strings.Contains(err.Error(), test.expectErrorContains) {
					t.Fatalf(
						"expected error containing [%s]; got [%v]",
						test.expectErrorContains, err,
					)
				}
				if test.sweepMaxSizeErr != nil &&
					!errors.Is(err, test.sweepMaxSizeErr) {
					t.Fatalf(
						"expected error to wrap [%v]; got [%v]",
						test.sweepMaxSizeErr, err,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			fee := fees[test.depositsCount]
			if fee.SatPerVByteFee != test.expectedSatPerVByteFee {
				t.Errorf(
					"unexpected sweep fee rate\nexpected: [%d] sat/vByte\nactual:   [%d] sat/vByte",
					test.expectedSatPerVByteFee, fee.SatPerVByteFee,
				)
			}
			// TotalFee is the value actually broadcast on-chain; assert it
			// directly. SatPerVByteFee alone is lossy: math.Round collapses a
			// range of TotalFee values onto the same rate (e.g. in the cap case
			// both 22*size and 22*size+1 round to 22), so a TotalFee bug would be
			// invisible if only the rate were checked.
			if fee.TotalFee != test.expectedTotalFee {
				t.Errorf(
					"unexpected sweep total fee\nexpected: [%d] sat\nactual:   [%d] sat",
					test.expectedTotalFee, fee.TotalFee,
				)
			}
		})
	}
}

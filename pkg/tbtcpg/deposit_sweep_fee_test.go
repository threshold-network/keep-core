package tbtcpg_test

import (
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestEstimateDepositsSweepFee_MinimumFloorAndBuffer verifies the sweep fee
// logic: a low estimate is raised to the minimum floor, an estimate above the
// floor is buffered by 25%, and a Bridge maximum below the minimum floor
// returns an error rather than silently broadcasting an underpriced sweep.
func TestEstimateDepositsSweepFee_MinimumFloorAndBuffer(t *testing.T) {
	// Virtual size of a one-deposit sweep, used to size the cap for the error
	// case relative to the minimum floor. 126 == depositScriptByteSize.
	size, err := bitcoin.NewTransactionSizeEstimator().
		AddPublicKeyHashInputs(1, true).
		AddScriptHashInputs(1, 126, true).
		AddPublicKeyHashOutputs(1, true).
		VirtualSize()
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		estimateSatPerVByte    int64
		perDepositMaxFee       uint64
		expectedSatPerVByteFee int64
		expectErrorContains    string
	}{
		"low estimate is raised to the minimum floor": {
			estimateSatPerVByte:    1,
			perDepositMaxFee:       100000,
			expectedSatPerVByteFee: 5, // max(5, ceil(1*1.25)=2) = 5
		},
		"estimate above the floor is buffered by 25%": {
			estimateSatPerVByte:    20,
			perDepositMaxFee:       100000,
			expectedSatPerVByteFee: 25, // ceil(20*1.25) = 25
		},
		"buffered estimate above the cap is bounded to the cap": {
			estimateSatPerVByte: 20,
			// ceil(20*1.25)=25 sat/vByte buffered fee exceeds the 22*size cap,
			// so it is bounded down to the cap (rate 22), not the buffered 25.
			perDepositMaxFee:       uint64(22 * size),
			expectedSatPerVByteFee: 22,
		},
		"minimum floor above the cap returns an error": {
			estimateSatPerVByte: 1,
			// Cap sits below 5*size (the floor) but above the raw fee (1*size),
			// so the minimum-fee check must error rather than lower the fee. The
			// substring pins this to the floor-exceeds-cap branch specifically,
			// distinguishing it from the raw-fee-exceeds-cap error.
			perDepositMaxFee:    uint64(3 * size),
			expectErrorContains: "minimum safe sweep fee",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			tbtcChain.SetDepositParameters(0, 0, test.perDepositMaxFee, 0)

			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, test.estimateSatPerVByte)

			fees, err := tbtcpg.EstimateDepositsSweepFee(tbtcChain, btcChain, 1)

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
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}
			if got := fees[1].SatPerVByteFee; got != test.expectedSatPerVByteFee {
				t.Errorf(
					"unexpected sweep fee rate\nexpected: [%d] sat/vByte\nactual:   [%d] sat/vByte",
					test.expectedSatPerVByteFee, got,
				)
			}
		})
	}
}

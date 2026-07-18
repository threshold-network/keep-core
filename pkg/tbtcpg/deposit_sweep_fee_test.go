package tbtcpg_test

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestEstimateDepositsSweepFee_MinimumFloor verifies that an unusably low fee
// estimate is raised to the minimum sweep fee rate (see minSweepTxSatPerVByteFee,
// currently 5 sat/vByte), while a healthy estimate is left unchanged.
func TestEstimateDepositsSweepFee_MinimumFloor(t *testing.T) {
	tests := map[string]struct {
		estimateSatPerVByte    int64
		expectedSatPerVByteFee int64
	}{
		"low estimate is raised to the minimum": {
			estimateSatPerVByte:    1,
			expectedSatPerVByteFee: 5,
		},
		"healthy estimate is left unchanged": {
			estimateSatPerVByte:    20,
			expectedSatPerVByteFee: 20,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tbtcChain := tbtcpg.NewLocalChain()
			// A per-deposit maximum fee high enough not to bind here.
			tbtcChain.SetDepositParameters(0, 0, 100000, 0)

			btcChain := tbtcpg.NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, test.estimateSatPerVByte)

			fees, err := tbtcpg.EstimateDepositsSweepFee(tbtcChain, btcChain, 1)
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			got := fees[1].SatPerVByteFee
			if got != test.expectedSatPerVByteFee {
				t.Errorf(
					"unexpected sweep fee rate\nexpected: [%d] sat/vByte\nactual:   [%d] sat/vByte",
					test.expectedSatPerVByteFee,
					got,
				)
			}
		})
	}
}

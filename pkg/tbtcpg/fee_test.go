package tbtcpg

import (
	"strings"
	"testing"
)

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

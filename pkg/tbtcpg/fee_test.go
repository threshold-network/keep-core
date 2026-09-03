package tbtcpg

import (
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
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

func TestEstimateReservationFixedSizeTxFee(t *testing.T) {
	sizeEstimator := bitcoin.NewTransactionSizeEstimator().
		AddScriptHashInputs(1, depositScriptByteSize, true).
		AddPublicKeyHashOutputs(1, true)

	size, err := sizeEstimator.VirtualSize()
	if err != nil {
		t.Fatal(err)
	}

	const (
		acceptanceErrMsg = "reservation acceptance estimated fee exceeds the maximum fee"
		reanchorErrMsg   = "reservation re-anchor estimated fee exceeds the maximum fee"
	)

	tests := map[string]struct {
		estimateSatPerVByte int64
		txMaxFee            uint64
		exceedsMaxErrMsg    string
		expectedFee         int64
		expectErrorContains string
	}{
		"low estimate is raised to the minimum floor": {
			estimateSatPerVByte: 1,
			txMaxFee:            100000,
			exceedsMaxErrMsg:    acceptanceErrMsg,
			expectedFee:         5 * size, // max(5, ceil(1*1.25)=2)=5 sat/vByte * size
		},
		"estimate above the floor is buffered by 25%": {
			estimateSatPerVByte: 20,
			txMaxFee:            100000,
			exceedsMaxErrMsg:    acceptanceErrMsg,
			expectedFee:         25 * size, // ceil(20*1.25) = 25 sat/vByte * size
		},
		"buffered estimate above the cap is bounded to the cap": {
			estimateSatPerVByte: 20,
			txMaxFee:            uint64(22 * size),
			exceedsMaxErrMsg:    acceptanceErrMsg,
			expectedFee:         22 * size,
		},
		"raw estimate exactly equal to the cap is allowed and bounded": {
			estimateSatPerVByte: 20,
			txMaxFee:            uint64(20 * size),
			exceedsMaxErrMsg:    acceptanceErrMsg,
			expectedFee:         20 * size,
		},
		"raw estimate 1 sat above the cap returns an error": {
			estimateSatPerVByte: 20,
			txMaxFee:            uint64(20*size - 1),
			exceedsMaxErrMsg:    acceptanceErrMsg,
			expectErrorContains: acceptanceErrMsg,
		},
		"raw estimate above the cap returns acceptance error": {
			estimateSatPerVByte: 30,
			// The raw 30*size fee already exceeds the 10*size cap, so the raw-fee
			// check must error before the minimum-floor logic runs. The returned
			// error must match the exact exceedsMaxErrMsg passed by the caller.
			txMaxFee:            uint64(10 * size),
			exceedsMaxErrMsg:    acceptanceErrMsg,
			expectErrorContains: acceptanceErrMsg,
		},
		"raw estimate above the cap returns re-anchor error": {
			estimateSatPerVByte: 30,
			txMaxFee:            uint64(10 * size),
			exceedsMaxErrMsg:    reanchorErrMsg,
			expectErrorContains: reanchorErrMsg,
		},
		"minimum floor above the cap returns an error": {
			estimateSatPerVByte: 1,
			// Cap sits below 5*size (the floor) but above the raw fee (1*size),
			// so the minimum-fee check must error rather than lower the fee.
			txMaxFee:            uint64(3 * size),
			exceedsMaxErrMsg:    acceptanceErrMsg,
			expectErrorContains: "minimum safe transaction fee",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			btcChain := NewLocalBitcoinChain()
			btcChain.SetEstimateSatPerVByteFee(1, tc.estimateSatPerVByte)

			fee, err := estimateReservationFixedSizeTxFee(
				btcChain,
				sizeEstimator,
				tc.txMaxFee,
				tc.exceedsMaxErrMsg,
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

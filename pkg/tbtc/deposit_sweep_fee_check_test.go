package tbtc

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// TestCheckSweepFeeFloor covers the follower-side soft check decision that backs
// the below-floor warning and metric (threshold-network/keep-core#4171). It
// exercises the boundary directly - proposals just below, exactly at, and above
// the safe minimum, plus a missing fee - so that a flipped comparison, a dropped
// nil branch, or an off-by-one at the boundary fails here rather than passing CI
// silently. The static drift guard in sweep_fee_sync_test.go does not cover this
// logic.
func TestCheckSweepFeeFloor(t *testing.T) {
	proposalWithFee := func(fee *big.Int) *DepositSweepProposal {
		return &DepositSweepProposal{
			DepositsKeys: make([]struct {
				FundingTxHash      bitcoin.Hash
				FundingOutputIndex uint32
			}, 1),
			SweepTxFee: fee,
		}
	}

	// Read the recomputed safe minimum once so the boundary cases can be built
	// relative to it without hard-coding the estimated virtual size. A missing
	// fee is treated as below floor, but minSweepTxFee is still computed.
	baseline, err := checkSweepFeeFloor(proposalWithFee(nil))
	if err != nil {
		t.Fatalf("unexpected error computing baseline: [%v]", err)
	}
	minFee := baseline.minSweepTxFee
	if minFee == nil || minFee.Sign() <= 0 {
		t.Fatalf("expected a positive safe-minimum fee, got [%v]", minFee)
	}

	oneBelow := new(big.Int).Sub(minFee, big.NewInt(1))
	oneAbove := new(big.Int).Add(minFee, big.NewInt(1))

	tests := map[string]struct {
		fee                *big.Int
		expectedBelowFloor bool
	}{
		"fee missing is treated as below floor": {
			fee:                nil,
			expectedBelowFloor: true,
		},
		"fee one satoshi below floor is below floor": {
			fee:                oneBelow,
			expectedBelowFloor: true,
		},
		"fee exactly at floor is not below floor": {
			fee:                new(big.Int).Set(minFee),
			expectedBelowFloor: false,
		},
		"fee one satoshi above floor is not below floor": {
			fee:                oneAbove,
			expectedBelowFloor: false,
		},
		"zero fee is below floor": {
			fee:                big.NewInt(0),
			expectedBelowFloor: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			check, err := checkSweepFeeFloor(proposalWithFee(tc.fee))
			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			if check.belowFloor != tc.expectedBelowFloor {
				t.Errorf(
					"unexpected belowFloor\nexpected: [%t]\nactual:   [%t]",
					tc.expectedBelowFloor, check.belowFloor,
				)
			}

			if check.minSweepTxFee.Cmp(minFee) != 0 {
				t.Errorf(
					"unexpected minSweepTxFee\nexpected: [%d]\nactual:   [%d]",
					minFee, check.minSweepTxFee,
				)
			}
		})
	}
}

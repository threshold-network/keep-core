package tbtc_test

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestSweepFeeConstantsMirrorTbtcpg guards the sweep-fee constants that pkg/tbtc
// duplicates from pkg/tbtcpg. The follower-side soft check
// (threshold-network/keep-core#4171) recomputes the safe minimum sweep fee, but
// pkg/tbtcpg imports pkg/tbtc, so pkg/tbtc cannot import the canonical constants
// without a dependency cycle and hand-copies them instead.
//
// This test lives in the external tbtc_test package precisely because that
// package can import both pkg/tbtc and pkg/tbtcpg without forming the cycle. It
// compares the two actual constants directly - not against hand-copied literals
// - so it fails whenever the pkg/tbtc mirror and the canonical tbtcpg value
// drift apart, regardless of which side was changed. A literal-based guard
// could be defeated by updating tbtcpg and the literal together while forgetting
// the pkg/tbtc mirror; comparing the live values closes that gap.
func TestSweepFeeConstantsMirrorTbtcpg(t *testing.T) {
	if tbtc.MinSweepTxSatPerVByteFee != tbtcpg.MinWalletTxSatPerVByteFee {
		t.Errorf(
			"tbtc.MinSweepTxSatPerVByteFee [%d] has drifted from the canonical "+
				"tbtcpg.MinWalletTxSatPerVByteFee [%d]; the follower soft check "+
				"would warn at the wrong threshold",
			tbtc.MinSweepTxSatPerVByteFee,
			tbtcpg.MinWalletTxSatPerVByteFee,
		)
	}

	if tbtc.DepositScriptByteSize != tbtcpg.DepositScriptByteSize {
		t.Errorf(
			"tbtc.DepositScriptByteSize [%d] has drifted from the canonical "+
				"tbtcpg.DepositScriptByteSize [%d]; the follower soft check would "+
				"estimate the sweep tx size incorrectly",
			tbtc.DepositScriptByteSize,
			tbtcpg.DepositScriptByteSize,
		)
	}
}

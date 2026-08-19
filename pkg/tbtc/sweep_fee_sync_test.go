package tbtc_test

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestSweepFeeConstantsMirrorTbtcpg guards the cross-package mirrors of
// constants that pkg/tbtc duplicates from pkg/tbtcpg. The follower-side
// soft check (threshold-network/keep-core#4171) uses the same
// DepositScriptByteSize as the leader-side estimator, so a drift would
// produce a sweep tx estimate that disagrees with what the leader built.
//
// The minimum-floor and buffer-ratio constants are NOT mirrored: they
// live as canonical exported vars in pkg/tbtc (MinWalletTxSatPerVByteFee,
// WalletTxFeeBufferNumerator, WalletTxFeeBufferDenominator), which
// pkg/tbtcpg reads directly via tbtc.X. The operator-tunable runtime
// policy has a single source of truth, so a drift here would be a build
// error rather than a silent inconsistency.
//
// This test lives in the external tbtc_test package precisely because
// that package can import both pkg/tbtc and pkg/tbtcpg without forming
// the cycle. It compares the two actual constants directly - not
// against hand-copied literals - so it fails whenever the pkg/tbtc
// mirror and the canonical tbtcpg value drift apart, regardless of
// which side was changed. A literal-based guard could be defeated by
// updating tbtcpg and the literal together while forgetting the
// pkg/tbtc mirror; comparing the live values closes that gap.
func TestSweepFeeConstantsMirrorTbtcpg(t *testing.T) {
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

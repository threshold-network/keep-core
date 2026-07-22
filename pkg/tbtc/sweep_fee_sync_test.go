package tbtc_test

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestSweepFeeConstantsMirrorTbtcpg guards the sweep-fee constants that
// pkg/tbtc/deposit_sweep.go duplicates from pkg/tbtcpg. The follower-side soft
// check (threshold-network/keep-core#4171) recomputes the safe minimum sweep
// fee, but pkg/tbtcpg imports pkg/tbtc, so pkg/tbtc cannot import the canonical
// constants without a dependency cycle and hand-copies them instead.
//
// This test lives in the external tbtc_test package precisely because that
// package can import pkg/tbtcpg without forming the cycle. It pins the canonical
// tbtcpg values to the literals mirrored in pkg/tbtc/deposit_sweep.go
// (minSweepTxSatPerVByteFee and depositScriptByteSize). If the canonical values
// drift, this test fails, forcing the pkg/tbtc mirrors - and these expected
// literals - to be updated together.
func TestSweepFeeConstantsMirrorTbtcpg(t *testing.T) {
	// Mirrored by pkg/tbtc/deposit_sweep.go:minSweepTxSatPerVByteFee.
	const expectedMinWalletTxSatPerVByteFee = 5
	// Mirrored by pkg/tbtc/deposit_sweep.go:depositScriptByteSize.
	const expectedDepositScriptByteSize = 126

	if tbtcpg.MinWalletTxSatPerVByteFee != expectedMinWalletTxSatPerVByteFee {
		t.Errorf(
			"tbtcpg.MinWalletTxSatPerVByteFee is [%d]; the pkg/tbtc mirror "+
				"minSweepTxSatPerVByteFee [%d] is now stale and must be updated "+
				"along with this test",
			tbtcpg.MinWalletTxSatPerVByteFee,
			expectedMinWalletTxSatPerVByteFee,
		)
	}

	if tbtcpg.DepositScriptByteSize != expectedDepositScriptByteSize {
		t.Errorf(
			"tbtcpg.DepositScriptByteSize is [%d]; the pkg/tbtc mirror "+
				"depositScriptByteSize [%d] is now stale and must be updated "+
				"along with this test",
			tbtcpg.DepositScriptByteSize,
			expectedDepositScriptByteSize,
		)
	}
}

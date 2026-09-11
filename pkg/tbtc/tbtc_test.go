package tbtc

import (
	"testing"
)

func TestApplyWalletTxFeePolicy(t *testing.T) {
	originalFloor := MinWalletTxSatPerVByteFee
	originalPercent := WalletTxFeeBufferPercent
	t.Cleanup(func() {
		MinWalletTxSatPerVByteFee = originalFloor
		WalletTxFeeBufferPercent = originalPercent
	})

	// A zero-valued Config (e.g. a test that constructs Config{}) must
	// keep the package defaults so the leader-side and follower-side
	// fee-floor logic keeps working without a CLI override.
	t.Run("zero-valued config keeps defaults", func(t *testing.T) {
		MinWalletTxSatPerVByteFee = DefaultWalletTxSatPerVByteFloor
		WalletTxFeeBufferPercent = DefaultWalletTxFeeBufferPercent

		applyWalletTxFeePolicy(Config{})

		if MinWalletTxSatPerVByteFee != DefaultWalletTxSatPerVByteFloor {
			t.Errorf(
				"expected default floor [%d], got [%d]",
				DefaultWalletTxSatPerVByteFloor,
				MinWalletTxSatPerVByteFee,
			)
		}
		if WalletTxFeeBufferPercent != DefaultWalletTxFeeBufferPercent {
			t.Errorf(
				"expected default buffer percent [%d], got [%d]",
				DefaultWalletTxFeeBufferPercent,
				WalletTxFeeBufferPercent,
			)
		}
	})

	// An operator-supplied config (e.g. via a Viper flag) overrides
	// every field that is non-zero. The leader-side floor application
	// in tbtcpg.applyWalletTxFeeFloor and the follower-side soft check
	// in warnIfProposedWalletTxFeeBelowBufferedFloor both read the
	// same package vars, so a single tuning here propagates to both.
	t.Run("non-zero config overrides defaults", func(t *testing.T) {
		applyWalletTxFeePolicy(Config{
			WalletTxSatPerVByteFloor: 7,
			WalletTxFeeBufferPercent: 30,
		})

		if MinWalletTxSatPerVByteFee != 7 {
			t.Errorf(
				"expected floor [7], got [%d]",
				MinWalletTxSatPerVByteFee,
			)
		}
		if WalletTxFeeBufferPercent != 30 {
			t.Errorf(
				"expected buffer percent [30], got [%d]",
				WalletTxFeeBufferPercent,
			)
		}
	})

	// Partial config: only the floor is tuned, the buffer percentage
	// keeps the default. This is the realistic operator path where one
	// knob is changed at a time during a rollout.
	t.Run("partial config keeps unset defaults", func(t *testing.T) {
		MinWalletTxSatPerVByteFee = DefaultWalletTxSatPerVByteFloor
		WalletTxFeeBufferPercent = DefaultWalletTxFeeBufferPercent

		applyWalletTxFeePolicy(Config{
			WalletTxSatPerVByteFloor: 9,
		})

		if MinWalletTxSatPerVByteFee != 9 {
			t.Errorf(
				"expected floor [9], got [%d]",
				MinWalletTxSatPerVByteFee,
			)
		}
		if WalletTxFeeBufferPercent != DefaultWalletTxFeeBufferPercent {
			t.Errorf(
				"expected default buffer percent [%d], got [%d]",
				DefaultWalletTxFeeBufferPercent,
				WalletTxFeeBufferPercent,
			)
		}
	})
}

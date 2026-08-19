package tbtc

import (
	"testing"
)

func TestApplyWalletTxFeePolicy(t *testing.T) {
	originalFloor := MinWalletTxSatPerVByteFee
	originalNum := WalletTxFeeBufferNumerator
	originalDen := WalletTxFeeBufferDenominator
	t.Cleanup(func() {
		MinWalletTxSatPerVByteFee = originalFloor
		WalletTxFeeBufferNumerator = originalNum
		WalletTxFeeBufferDenominator = originalDen
	})

	// A zero-valued Config (e.g. a test that constructs Config{}) must
	// keep the package defaults so the leader-side and follower-side
	// fee-floor logic keeps working without a CLI override.
	t.Run("zero-valued config keeps defaults", func(t *testing.T) {
		MinWalletTxSatPerVByteFee = DefaultWalletTxSatPerVByteFloor
		WalletTxFeeBufferNumerator = DefaultWalletTxFeeBufferNumerator
		WalletTxFeeBufferDenominator = DefaultWalletTxFeeBufferDenominator

		applyWalletTxFeePolicy(Config{})

		if MinWalletTxSatPerVByteFee != DefaultWalletTxSatPerVByteFloor {
			t.Errorf(
				"expected default floor [%d], got [%d]",
				DefaultWalletTxSatPerVByteFloor,
				MinWalletTxSatPerVByteFee,
			)
		}
		if WalletTxFeeBufferNumerator != DefaultWalletTxFeeBufferNumerator {
			t.Errorf(
				"expected default numerator [%d], got [%d]",
				DefaultWalletTxFeeBufferNumerator,
				WalletTxFeeBufferNumerator,
			)
		}
		if WalletTxFeeBufferDenominator != DefaultWalletTxFeeBufferDenominator {
			t.Errorf(
				"expected default denominator [%d], got [%d]",
				DefaultWalletTxFeeBufferDenominator,
				WalletTxFeeBufferDenominator,
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
			WalletTxSatPerVByteFloor:     7,
			WalletTxFeeBufferNumerator:   3,
			WalletTxFeeBufferDenominator: 2,
		})

		if MinWalletTxSatPerVByteFee != 7 {
			t.Errorf(
				"expected floor [7], got [%d]",
				MinWalletTxSatPerVByteFee,
			)
		}
		if WalletTxFeeBufferNumerator != 3 {
			t.Errorf(
				"expected numerator [3], got [%d]",
				WalletTxFeeBufferNumerator,
			)
		}
		if WalletTxFeeBufferDenominator != 2 {
			t.Errorf(
				"expected denominator [2], got [%d]",
				WalletTxFeeBufferDenominator,
			)
		}
	})

	// Partial config: only the floor is tuned, the buffer ratio keeps
	// the default. This is the realistic operator path where one knob
	// is changed at a time during a rollout.
	t.Run("partial config keeps unset defaults", func(t *testing.T) {
		MinWalletTxSatPerVByteFee = DefaultWalletTxSatPerVByteFloor
		WalletTxFeeBufferNumerator = DefaultWalletTxFeeBufferNumerator
		WalletTxFeeBufferDenominator = DefaultWalletTxFeeBufferDenominator

		applyWalletTxFeePolicy(Config{
			WalletTxSatPerVByteFloor: 9,
		})

		if MinWalletTxSatPerVByteFee != 9 {
			t.Errorf(
				"expected floor [9], got [%d]",
				MinWalletTxSatPerVByteFee,
			)
		}
		if WalletTxFeeBufferNumerator != DefaultWalletTxFeeBufferNumerator {
			t.Errorf(
				"expected default numerator [%d], got [%d]",
				DefaultWalletTxFeeBufferNumerator,
				WalletTxFeeBufferNumerator,
			)
		}
		if WalletTxFeeBufferDenominator != DefaultWalletTxFeeBufferDenominator {
			t.Errorf(
				"expected default denominator [%d], got [%d]",
				DefaultWalletTxFeeBufferDenominator,
				WalletTxFeeBufferDenominator,
			)
		}
	})
}

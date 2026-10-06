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

	// A floor below the protocol minimum (ProtocolMinWalletTxSatPerVByteFee = 5)
	// is rejected: an operator-tuned floor under the fixed protocol minimum would
	// let every follower co-sign underpriced wallet transactions on all five paths.
	t.Run("floor below protocol minimum is rejected", func(t *testing.T) {
		err := applyWalletTxFeePolicy(Config{
			WalletTxSatPerVByteFloor: 4, // below ProtocolMinWalletTxSatPerVByteFee=5
		})
		if err == nil {
			t.Error("expected error for floor=4 (below protocol minimum 5); got nil")
		}
	})

	t.Run("floor at protocol minimum is accepted", func(t *testing.T) {
		MinWalletTxSatPerVByteFee = DefaultWalletTxSatPerVByteFloor
		err := applyWalletTxFeePolicy(Config{
			WalletTxSatPerVByteFloor: 5, // exactly at ProtocolMinWalletTxSatPerVByteFee=5
		})
		if err != nil {
			t.Errorf("expected no error for floor=5 (at protocol minimum); got: %v", err)
		}
		if MinWalletTxSatPerVByteFee != 5 {
			t.Errorf("expected floor=5, got %d", MinWalletTxSatPerVByteFee)
		}
	})

	t.Run("zero floor (default) is accepted", func(t *testing.T) {
		MinWalletTxSatPerVByteFee = DefaultWalletTxSatPerVByteFloor
		err := applyWalletTxFeePolicy(Config{}) // zero = use default, no error
		if err != nil {
			t.Errorf("expected no error for zero floor (default); got: %v", err)
		}
		if MinWalletTxSatPerVByteFee != DefaultWalletTxSatPerVByteFloor {
			t.Errorf("expected default floor %d, got %d", DefaultWalletTxSatPerVByteFloor, MinWalletTxSatPerVByteFee)
		}
	})
}

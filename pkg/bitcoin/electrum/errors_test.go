package electrum

import (
	"errors"
	"fmt"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// TestFeeFallbackResultPreservesSentinel checks that the fee-estimate sentinel
// survives the wrapping feeFallbackResult applies on its way to the caller.
//
// EstimateSatPerVByteFee reports ErrFeeEstimateUnavailable when the daemon has
// no estimate, and callers classify that with errors.Is. feeFallbackResult is
// the only path that returns it, so wrapping with %v instead of %w there
// breaks the chain silently: the message still reads correctly and only the
// classification stops working.
func TestFeeFallbackResultPreservesSentinel(t *testing.T) {
	_, err := feeFallbackResult(
		bitcoin.Mainnet,
		false, // no fee-oracle failure, so the lastErr branch is taken
		ErrFeeEstimateUnavailable,
		[]uint32{1, 2, 3},
	)
	if err == nil {
		t.Fatal("expected an error")
	}

	if !errors.Is(err, ErrFeeEstimateUnavailable) {
		t.Errorf(
			"fee fallback error does not match ErrFeeEstimateUnavailable: [%v]",
			err,
		)
	}
}

// TestFeeFallbackResultPreservesWrappedSentinel covers the same chain when the
// sentinel reaches feeFallbackResult already wrapped by an inner call.
func TestFeeFallbackResultPreservesWrappedSentinel(t *testing.T) {
	_, err := feeFallbackResult(
		bitcoin.Mainnet,
		false,
		fmt.Errorf("get fee for 2 blocks: [%w]", ErrFeeEstimateUnavailable),
		[]uint32{2},
	)
	if err == nil {
		t.Fatal("expected an error")
	}

	if !errors.Is(err, ErrFeeEstimateUnavailable) {
		t.Errorf(
			"fee fallback error does not match ErrFeeEstimateUnavailable: [%v]",
			err,
		)
	}
}

// TestFeeFallbackResultWithoutSentinel guards the negative case: an unrelated
// error must not be classified as a missing fee estimate.
func TestFeeFallbackResultWithoutSentinel(t *testing.T) {
	_, err := feeFallbackResult(
		bitcoin.Mainnet,
		false,
		errors.New("connection reset by peer"),
		[]uint32{1},
	)
	if err == nil {
		t.Fatal("expected an error")
	}

	if errors.Is(err, ErrFeeEstimateUnavailable) {
		t.Errorf("unrelated error misclassified as ErrFeeEstimateUnavailable")
	}
}

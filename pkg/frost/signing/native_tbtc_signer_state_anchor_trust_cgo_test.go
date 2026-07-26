//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyNativeTBTCSignerStateAnchorTrustHeadError(t *testing.T) {
	absent := fmt.Errorf(
		"%w: %w",
		ErrNativeBridgeOperationFailed,
		&buildTaggedTBTCSignerStructuredError{
			Code:    nativeTBTCSignerStateAnchorTrustHeadAbsentCode,
			Message: "ordinary trust journal is absent",
		},
	)
	classified :=
		classifyNativeTBTCSignerStateAnchorTrustHeadError(absent)
	if !errors.Is(
		classified,
		ErrNativeTBTCSignerStateAnchorTrustHeadAbsent,
	) {
		t.Fatal("typed no-journal error was not classified")
	}

	for _, code := range []string{
		"",
		"state_anchor_trust_journal_corrupt",
		"permission_denied",
	} {
		failure := fmt.Errorf(
			"%w: %w",
			ErrNativeBridgeOperationFailed,
			&buildTaggedTBTCSignerStructuredError{
				Code:    code,
				Message: "terminal failure",
			},
		)
		classified =
			classifyNativeTBTCSignerStateAnchorTrustHeadError(failure)
		if errors.Is(
			classified,
			ErrNativeTBTCSignerStateAnchorTrustHeadAbsent,
		) {
			t.Fatalf("terminal error code [%s] was classified as absence", code)
		}
		if !errors.Is(classified, ErrNativeBridgeOperationFailed) {
			t.Fatalf("terminal error code [%s] lost its original cause", code)
		}
	}
}

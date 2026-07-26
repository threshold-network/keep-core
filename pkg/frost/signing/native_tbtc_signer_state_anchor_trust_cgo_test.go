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

func TestClassifyNativeTBTCSignerStateAnchorTrustRecoveryRequiredError(
	t *testing.T,
) {
	recovery, err := decodeNativeTBTCSignerStateAnchorTrustRecoveryRequired(
		testNativeTBTCSignerStateAnchorTrustRecoveryRequiredWirePointer(),
	)
	if err != nil {
		t.Fatal(err)
	}
	structured := &buildTaggedTBTCSignerStructuredError{
		Code:                     nativeTBTCSignerStateAnchorTrustRecoveryRequiredCode,
		Message:                  "recovery required",
		StateAnchorTrustRecovery: recovery,
	}
	failure := fmt.Errorf(
		"%w: %w",
		ErrNativeBridgeOperationFailed,
		structured,
	)

	classifiers := map[string]func(error) error{
		"trust head": classifyNativeTBTCSignerStateAnchorTrustHeadError,
		"transition": classifyNativeTBTCSignerStateAnchorTrustTransitionError,
	}
	for name, classify := range classifiers {
		t.Run(name, func(t *testing.T) {
			classified := classify(failure)
			var recoveryError *NativeTBTCSignerStateAnchorTrustRecoveryRequiredError
			if !errors.As(classified, &recoveryError) {
				t.Fatalf("typed recovery error was not classified: %v", classified)
			}
			if !errors.Is(classified, ErrNativeBridgeOperationFailed) {
				t.Fatal("typed recovery error lost the original bridge failure")
			}
			if recoveryError.Recovery.CertificateCount != 2 ||
				len(recoveryError.Recovery.OrderedCertificateDigests) != 2 {
				t.Fatalf(
					"unexpected typed recovery selector: %+v",
					recoveryError.Recovery,
				)
			}

			// The classified error owns its selector; later changes to the
			// decoded envelope cannot alter which authenticated suffix is used.
			originalDigest :=
				recoveryError.Recovery.OrderedCertificateDigests[0]
			structured.StateAnchorTrustRecovery.
				OrderedCertificateDigests[0] = [32]byte{0xff}
			if recoveryError.Recovery.OrderedCertificateDigests[0] !=
				originalDigest {
				t.Fatal("classified recovery selector aliases the FFI envelope")
			}
			structured.StateAnchorTrustRecovery.
				OrderedCertificateDigests[0] = originalDigest
		})
	}

	malformed := fmt.Errorf(
		"%w: %w",
		ErrNativeBridgeOperationFailed,
		&buildTaggedTBTCSignerStructuredError{
			Code:    nativeTBTCSignerStateAnchorTrustRecoveryRequiredCode,
			Message: "recovery context was malformed",
		},
	)
	for name, classify := range classifiers {
		t.Run(name+" malformed", func(t *testing.T) {
			classified := classify(malformed)
			var recoveryError *NativeTBTCSignerStateAnchorTrustRecoveryRequiredError
			if errors.As(classified, &recoveryError) {
				t.Fatal("malformed recovery context became retryable")
			}
			if !errors.Is(classified, ErrNativeBridgeOperationFailed) {
				t.Fatal("malformed recovery context lost its terminal cause")
			}
		})
	}
}

func testNativeTBTCSignerStateAnchorTrustRecoveryRequiredWirePointer(
) *nativeTBTCSignerStateAnchorTrustRecoveryRequiredWire {
	wire := testNativeTBTCSignerStateAnchorTrustRecoveryRequiredWire()
	return &wire
}

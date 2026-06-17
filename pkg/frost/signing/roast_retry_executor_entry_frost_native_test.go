//go:build frost_native

package signing

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newEntryTestRequest(t *testing.T) *NativeExecutionFFISigningRequest {
	t.Helper()
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup: "tbtc-signer-entry-group",
	})
	return &NativeExecutionFFISigningRequest{
		Message:     new(big.Int).SetBytes([]byte{0xab, 0xcd}),
		SessionID:   "executor-entry-test",
		MemberIndex: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: payload,
		},
		Attempt: &Attempt{
			Number:                 1,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2, 3, 4, 5},
		},
	}
}

func TestEntry_StaticFallback_NoCoordinatorRegistered_TaggedBuild(t *testing.T) {
	// Without the frost_roast_retry build tag this is exercised by
	// the default-build test (which always falls through). Under the
	// frost_native build alone, the helper still treats the absence
	// of a registered coordinator as a static fallback because
	// BeginOrchestrationForSession returns
	// ErrNoRoastRetryCoordinatorRegistered (in the default build it
	// is the stub no-op-return-true).
	//
	// The helper must return (nil, nil) regardless: the executor
	// adapter proceeds without orchestration, matching Phase 5
	// receive semantics.
	logger := log.Logger("entry-static-test")
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), newEntryTestRequest(t), logger,
	)
	if err != nil {
		t.Fatalf("static fallback must not surface an error: %v", err)
	}
	if cleanup != nil {
		t.Fatal("static fallback must not return a cleanup function")
	}
}

func TestEntry_LogsSignerMaterialFormatTelemetry(t *testing.T) {
	logger := &captureInfoLogger{}
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), newEntryTestRequest(t), logger,
	)
	if err != nil {
		t.Fatalf("static fallback must not surface an error: %v", err)
	}
	if cleanup != nil {
		t.Fatal("static fallback must not return a cleanup function")
	}

	joined := strings.Join(logger.infoMessages, "\n")
	if !strings.Contains(joined, "signer_material_format") ||
		!strings.Contains(joined, NativeSignerMaterialFormatFrostTBTCSignerV1) ||
		!strings.Contains(joined, "key_group_id") {
		t.Fatalf("missing signer-material telemetry in logs: [%s]", joined)
	}
}

func TestEntry_StaticFallback_UnsupportedSignerFormat(t *testing.T) {
	// FrostUniFFIV1 material -> ExtractDkgGroupPublicKeyFromMaterial
	// returns ErrUnsupportedSignerMaterialFormat. The helper must
	// treat this as STATIC (deterministic across deployments) and
	// fall back without surfacing an error.
	req := newEntryTestRequest(t)
	req.SignerMaterial = &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV1,
		Payload: []byte("{}"),
	}
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), req, log.Logger("entry-v1-test"),
	)
	if err != nil {
		t.Fatalf("V1 material must be a static fallback: %v", err)
	}
	if cleanup != nil {
		t.Fatal("static fallback must not return a cleanup function")
	}
}

func TestEntry_StaticFallback_OnNilSignerMaterial(t *testing.T) {
	// Nil signer material is a deterministic, per-input
	// construction-precondition failure: every honest node with
	// the same request would observe it identically. Treated as a
	// STATIC fallback so the executor adapter proceeds without
	// orchestration. The HARD-FAIL discipline is reserved for
	// non-deterministic Coordinator state-machine errors.
	req := newEntryTestRequest(t)
	req.SignerMaterial = nil
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), req, log.Logger("entry-nil-mat-test"),
	)
	if err != nil {
		t.Fatalf("nil signer material must be a STATIC fallback; got %v", err)
	}
	if cleanup != nil {
		t.Fatal("static fallback must not return cleanup")
	}
}

type captureInfoLogger struct {
	testutils.MockLogger
	infoMessages []string
}

func (cil *captureInfoLogger) Infof(format string, args ...interface{}) {
	cil.infoMessages = append(cil.infoMessages, fmt.Sprintf(format, args...))
}

func TestEntry_StaticFallback_OnZeroAttemptNumber(t *testing.T) {
	// Zero attempt number is also a deterministic precondition
	// failure; treated as STATIC fallback.
	req := newEntryTestRequest(t)
	req.Attempt.Number = 0
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), req, log.Logger("entry-zero-attempt-test"),
	)
	if err != nil {
		t.Fatalf("zero attempt number must be a STATIC fallback; got %v", err)
	}
	if cleanup != nil {
		t.Fatal("static fallback must not return cleanup")
	}
}

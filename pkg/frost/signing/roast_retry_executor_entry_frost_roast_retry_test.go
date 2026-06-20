//go:build frost_native && frost_roast_retry

package signing

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newEntryRetryTestRequest(t *testing.T) *NativeExecutionFFISigningRequest {
	t.Helper()
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup: "tbtc-signer-entry-retry-group",
	})
	return &NativeExecutionFFISigningRequest{
		Message:     new(big.Int).SetBytes([]byte{0xab, 0xcd}),
		SessionID:   "executor-entry-retry-test",
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

func TestEntry_InteractiveOnly_RefusesCoarseFallback(t *testing.T) {
	// Coarse-path retirement: with interactive-only mode ON but interactive signing
	// not running (its audit gate off), the executor must REFUSE the coarse fallback
	// and fail closed, rather than returning a nil signature for the caller to sign
	// over the retired coarse path.
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	t.Setenv(InteractiveSigningOptInEnvVar, "") // audit gate OFF -> the drive returns nil
	t.Setenv(InteractiveSigningOnlyEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	signature, _, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), newEntryRetryTestRequest(t), log.Logger("entry-interactive-only"),
	)
	if signature != nil {
		t.Fatal("interactive-only refusal must not return a signature")
	}
	if err == nil {
		t.Fatal("interactive-only mode must refuse the coarse fallback when interactive signing did not run")
	}
	if !strings.Contains(err.Error(), InteractiveSigningOnlyEnvVar) {
		t.Fatalf("unexpected error (want a refusal naming %s): %v", InteractiveSigningOnlyEnvVar, err)
	}
}

func TestEntry_InteractiveSigningOnlyEnabled_ParsesFlag(t *testing.T) {
	t.Setenv(InteractiveSigningOnlyEnvVar, "")
	if InteractiveSigningOnlyEnabled() {
		t.Fatal("unset must be off")
	}
	t.Setenv(InteractiveSigningOnlyEnvVar, "  TrUe ")
	if !InteractiveSigningOnlyEnabled() {
		t.Fatal("case-insensitive, trimmed true must be on")
	}
	t.Setenv(InteractiveSigningOnlyEnvVar, "false")
	if InteractiveSigningOnlyEnabled() {
		t.Fatal("false must be off")
	}
}

func TestEntry_StaticFallback_ReadinessOptInUnset(t *testing.T) {
	// Explicitly unset the env var.
	t.Setenv(RoastRetryReadinessOptInEnvVar, "")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Register a coordinator -- the env var alone keeps us in
	// fallback.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), newEntryRetryTestRequest(t), log.Logger("entry-no-optin"),
	)
	if err != nil {
		t.Fatalf("static fallback (env var unset) must not surface an error: %v", err)
	}
	if cleanup != nil {
		t.Fatal("static fallback must not return a cleanup function")
	}
}

func TestEntry_StaticFallback_RegistryEmpty(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Registry is empty (no Register call).
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), newEntryRetryTestRequest(t), log.Logger("entry-no-registry"),
	)
	if err != nil {
		t.Fatalf("static fallback (registry empty) must not surface an error: %v", err)
	}
	if cleanup != nil {
		t.Fatal("static fallback must not return a cleanup function")
	}
}

func TestEntry_HappyPath_ActivatesOrchestration(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	req := newEntryRetryTestRequest(t)
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), req, log.Logger("entry-happy"),
	)
	if err != nil {
		t.Fatalf("happy path must not error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("happy path must return a cleanup function")
	}

	// Binding must exist for the session under this seat's member.
	if _, _, ok := currentAttemptHandleForCollect(req.SessionID, req.MemberIndex); !ok {
		t.Fatal("binding must exist after orchestration entry")
	}
	cleanup()
	if _, _, ok := currentAttemptHandleForCollect(req.SessionID, req.MemberIndex); ok {
		t.Fatal("binding must be cleared after cleanup")
	}
}

func TestEntry_HardFail_RuntimeBeginAttemptFailure(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Register an erroring coordinator -- BeginAttempt fails for
	// runtime reasons. Per the RFC-21 taxonomy, this must HARD FAIL.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: &erroringEntryCoordinator{
			err: errors.New("synthetic begin-attempt runtime failure"),
		},
		Signer:     roast.NoOpSigner(),
		Verifier:   roast.NoOpSignatureVerifier(),
		SelfMember: 1,
	})

	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), newEntryRetryTestRequest(t), log.Logger("entry-hard-fail"),
	)
	if err == nil {
		t.Fatal("runtime BeginAttempt error must HARD FAIL (not static fallback)")
	}
	if cleanup != nil {
		t.Fatal("hard-fail must not return cleanup")
	}
	if !contains(err.Error(), "synthetic begin-attempt runtime failure") {
		t.Fatalf("error must propagate underlying cause; got %v", err)
	}
}

// erroringEntryCoordinator implements roast.Coordinator with a
// synthetic BeginAttempt failure. Used to verify the HARD-FAIL
// branch of the executor-adapter entry helper.
type erroringEntryCoordinator struct {
	err error
}

func (e *erroringEntryCoordinator) BeginAttempt(_ attempt.AttemptContext) (roast.AttemptHandle, error) {
	return roast.AttemptHandle{}, e.err
}
func (e *erroringEntryCoordinator) State(_ roast.AttemptHandle) (roast.AttemptState, error) {
	return roast.AttemptStatePending, nil
}
func (e *erroringEntryCoordinator) SelectedCoordinator(_ roast.AttemptHandle) (group.MemberIndex, error) {
	return 0, nil
}
func (e *erroringEntryCoordinator) RecordEvidence(_ roast.AttemptHandle, _ *roast.LocalEvidenceSnapshot) error {
	return nil
}
func (e *erroringEntryCoordinator) AggregateBundle(_ roast.AttemptHandle) (*roast.TransitionMessage, error) {
	return nil, nil
}
func (e *erroringEntryCoordinator) MarkSucceeded(_ roast.AttemptHandle) error {
	return nil
}
func (e *erroringEntryCoordinator) VerifyBundle(_ roast.AttemptHandle, _ *roast.TransitionMessage) error {
	return nil
}
func (e *erroringEntryCoordinator) NextAttempt(
	_ roast.AttemptHandle, _ *roast.TransitionMessage, _ uint, _ []byte,
) (attempt.AttemptContext, error) {
	return attempt.AttemptContext{}, nil
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

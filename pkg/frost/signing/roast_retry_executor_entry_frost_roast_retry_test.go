//go:build frost_native && frost_roast_retry

package signing

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newEntryRetryTestRequest(t *testing.T) *NativeExecutionFFISigningRequest {
	t.Helper()
	const hexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	payload, _ := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "id",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: hexKey,
		},
	})
	return &NativeExecutionFFISigningRequest{
		Message:     new(big.Int).SetBytes([]byte{0xab, 0xcd}),
		SessionID:   "executor-entry-retry-test",
		MemberIndex: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV2,
			Payload: payload,
		},
		Attempt: &Attempt{
			Number:                 1,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2, 3, 4, 5},
		},
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

	cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		newEntryRetryTestRequest(t), log.Logger("entry-no-optin"),
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
	cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		newEntryRetryTestRequest(t), log.Logger("entry-no-registry"),
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
	cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		req, log.Logger("entry-happy"),
	)
	if err != nil {
		t.Fatalf("happy path must not error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("happy path must return a cleanup function")
	}

	// Binding must exist for the session.
	if _, _, ok := currentAttemptHandleForCollect(req.SessionID); !ok {
		t.Fatal("binding must exist after orchestration entry")
	}
	cleanup()
	if _, _, ok := currentAttemptHandleForCollect(req.SessionID); ok {
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

	cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		newEntryRetryTestRequest(t), log.Logger("entry-hard-fail"),
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

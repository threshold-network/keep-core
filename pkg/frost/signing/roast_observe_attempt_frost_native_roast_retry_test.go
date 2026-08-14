//go:build frost_native && frost_roast_retry

package signing

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newObserveTestRequest() *Request {
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup: "tbtc-signer-observe-group",
	})
	return &Request{
		Message:        new(big.Int).SetBytes([]byte{0xab, 0xcd}),
		RoastSessionID: "observe-test-session",
		MemberIndex:    2,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: payload,
		},
		Attempt: &Attempt{
			Number:                 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2, 3, 4, 5},
			ExcludedMembersIndexes: []group.MemberIndex{},
		},
	}
}

func registerObserveTestCoordinator() {
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  2,
		// Must match the key group the observe request's signer material yields, so
		// the wallet-scoped lookup in ObserveAttemptForTransition resolves.
		KeyGroupID: "tbtc-signer-observe-group",
	})
}

func TestObserveAttemptForTransition_StoresBinding(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	registerObserveTestCoordinator()

	req := newObserveTestRequest()
	if _, err := ObserveAttemptForTransition(req); err != nil {
		t.Fatalf("observe must not error: %v", err)
	}
	if !ObservedAttemptStoredForTest(req.RoastSessionID, req.MemberIndex) {
		t.Fatal("observe must store a binding for (roastSessionID, member)")
	}
}

func TestObserveAttemptForTransition_StaticFallback_NoCoordinator(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	// No coordinator registered -> static fallback, no binding.
	req := newObserveTestRequest()
	if _, err := ObserveAttemptForTransition(req); err != nil {
		t.Fatalf("static fallback must not error: %v", err)
	}
	if ObservedAttemptStoredForTest(req.RoastSessionID, req.MemberIndex) {
		t.Fatal("no binding must be stored when no coordinator is registered")
	}
}

func TestObserveAttemptForTransition_StaticFallback_ReadinessOff(t *testing.T) {
	// Env var empty -> opted out, even with a coordinator registered.
	t.Setenv(RoastRetryReadinessOptInEnvVar, "")
	ResetRoastRetryRegistrationForTest()
	ResetObservedAttemptRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetObservedAttemptRegistryForTest)

	registerObserveTestCoordinator()

	req := newObserveTestRequest()
	if _, err := ObserveAttemptForTransition(req); err != nil {
		t.Fatalf("readiness-off fallback must not error: %v", err)
	}
	if ObservedAttemptStoredForTest(req.RoastSessionID, req.MemberIndex) {
		t.Fatal("no binding must be stored when readiness opt-in is off")
	}
}

func TestObserveAttemptForTransition_NilRequest(t *testing.T) {
	if _, err := ObserveAttemptForTransition(nil); err == nil {
		t.Fatal("nil request must error")
	}
}

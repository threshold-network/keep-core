//go:build frost_native

package signing

import (
	"encoding/json"
	"fmt"
	"testing"
)

// persistedTBTCSignerMaterial builds a FrostTBTCSignerV1 signer material for a
// persisted DKG key group. The extracted DKG group public key is []byte(keyGroup)
// (see extractDkgGroupPublicKeyFromTBTCSignerV1), so callers that also build an
// attempt context must pass the same keyGroup bytes as the context's DKG key for
// the NewActiveRoastAttempt seed check to pass. Shared by the frost_native and
// frost_roast_retry interactive-signing tests.
func persistedTBTCSignerMaterial(
	t *testing.T,
	keyGroup string,
	threshold uint16,
	participants int,
) *NativeSignerMaterial {
	t.Helper()
	parts := make([]NativeTBTCSignerDKGParticipant, participants)
	for i := range parts {
		parts[i] = NativeTBTCSignerDKGParticipant{
			Identifier:   uint16(i + 1),
			PublicKeyHex: fmt.Sprintf("%064x", i+1),
		}
	}
	raw, err := json.Marshal(NativeTBTCSignerMaterialPayload{
		KeyGroup:        keyGroup,
		KeyGroupSource:  NativeTBTCSignerKeyGroupSourceDKGPersisted,
		DKGThreshold:    threshold,
		DKGParticipants: parts,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: raw,
	}
}

func TestRegisterInteractiveSigningEngineProvider(t *testing.T) {
	// Establish a clean precondition rather than assume one: other tests in this
	// package install an interactive provider as a global side effect (e.g. the
	// default FFI-provider registration under the cgo build calls
	// registerBuildTaggedNativeFROSTSigningEngine), so a bare "no provider yet"
	// assertion is order dependent and fails under -shuffle / a focused -run.
	// Resetting up front makes this test order-independent regardless of which
	// tests ran before it.
	ResetInteractiveSigningEngineProviderForTest()
	defer ResetInteractiveSigningEngineProviderForTest()

	if got := registeredInteractiveSigningEngine(); got != nil {
		t.Fatalf("expected nil engine after reset, got %T", got)
	}

	want := newFakeInteractiveSigningEngine()
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return want })
	if got := registeredInteractiveSigningEngine(); got != want {
		t.Fatalf("expected the registered engine, got %T", got)
	}

	// A provider that returns nil yields a nil engine (coarse fallback).
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine { return nil })
	if got := registeredInteractiveSigningEngine(); got != nil {
		t.Fatalf("expected nil from a nil-returning provider, got %T", got)
	}

	ResetInteractiveSigningEngineProviderForTest()
	if got := registeredInteractiveSigningEngine(); got != nil {
		t.Fatalf("expected nil engine after reset, got %T", got)
	}
}

func TestInteractiveRoastSigningThreshold(t *testing.T) {
	// The persisted DKG threshold is returned verbatim - NOT derived from the
	// per-attempt dishonest threshold.
	material := persistedTBTCSignerMaterial(t, "key-group", 2, 3)
	got, err := interactiveRoastSigningThreshold(&NativeExecutionFFISigningRequest{
		SignerMaterial: material,
		// A dishonest threshold that, mis-used as DishonestThreshold+1, would
		// yield 6 - the helper must ignore it and return the DKG threshold (2).
		DishonestThreshold: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected DKG threshold 2, got %d", got)
	}
}

func TestInteractiveRoastSigningThreshold_Rejects(t *testing.T) {
	zeroThreshold := persistedTBTCSignerMaterial(t, "kg", 0, 3)
	overParticipants := persistedTBTCSignerMaterial(t, "kg", 4, 3)

	bootstrap := persistedTBTCSignerMaterial(t, "kg", 2, 3)
	var bootstrapPayload NativeTBTCSignerMaterialPayload
	if err := json.Unmarshal(bootstrap.Payload, &bootstrapPayload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	bootstrapPayload.KeyGroupSource = "bootstrap"
	if raw, err := json.Marshal(bootstrapPayload); err != nil {
		t.Fatalf("marshal: %v", err)
	} else {
		bootstrap.Payload = raw
	}

	cases := map[string]*NativeExecutionFFISigningRequest{
		"nil request":              nil,
		"zero threshold":           {SignerMaterial: zeroThreshold},
		"threshold > participants": {SignerMaterial: overParticipants},
		"non-persisted source":     {SignerMaterial: bootstrap},
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := interactiveRoastSigningThreshold(request); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

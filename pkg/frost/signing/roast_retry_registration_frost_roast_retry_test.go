//go:build frost_roast_retry

package signing

import (
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

func TestRoastRetryRegistration_TaggedBuildRoundTrip(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	if _, ok := RegisteredRoastRetryCoordinator(); ok {
		t.Fatal("registry must start empty")
	}

	coord := roast.NewInMemoryCoordinator()
	deps := RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  7,
	}
	RegisterRoastRetryCoordinator(deps)

	got, ok := RegisteredRoastRetryCoordinator()
	if !ok {
		t.Fatal("expected ok=true after register")
	}
	if got.SelfMember != 7 {
		t.Fatalf("self member mismatch: got %d want 7", got.SelfMember)
	}
	if got.Coordinator == nil {
		t.Fatal("coordinator must round-trip")
	}
}

func TestRoastRetryRegistration_LaterRegistrationOverwrites(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: 1})
	RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: 2})
	got, ok := RegisteredRoastRetryCoordinator()
	if !ok {
		t.Fatal("expected ok=true after register")
	}
	if got.SelfMember != 2 {
		t.Fatalf("later registration must win: got %d want 2", got.SelfMember)
	}
}

func TestRoastRetryRegistration_ResetClearsRegistry(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: 1})
	ResetRoastRetryRegistrationForTest()
	if _, ok := RegisteredRoastRetryCoordinator(); ok {
		t.Fatal("registry must be empty after reset")
	}
}

func TestRoastRetryRegistration_ConcurrentRegisterAndLookupIsRaceSafe(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	var wg sync.WaitGroup
	const registers = 32
	const lookups = 64
	for i := 0; i < registers; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: uint32(i + 1)})
		}()
	}
	for i := 0; i < lookups; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = RegisteredRoastRetryCoordinator()
		}()
	}
	wg.Wait()

	// We don't assert a specific SelfMember -- registers race against
	// each other and any of them can land last. We assert only that
	// SOME registration succeeded.
	if _, ok := RegisteredRoastRetryCoordinator(); !ok {
		t.Fatal("expected at least one register to take effect")
	}
}

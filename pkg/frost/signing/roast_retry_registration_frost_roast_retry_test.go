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

// TestRoastRetryActive_GatesOnReadinessAndRegistration asserts RoastRetryActive
// is true only when BOTH the readiness opt-in is set AND a coordinator is
// registered -- the deterministic group-wide gate the signing loop uses to decide
// whether to key the active attempt off the committed roast number.
func TestRoastRetryActive_GatesOnReadinessAndRegistration(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	// Readiness off -> inactive regardless of registration.
	t.Setenv(RoastRetryReadinessOptInEnvVar, "false")
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		SelfMember:  1,
	})
	if RoastRetryActive() {
		t.Fatal("readiness off must yield inactive even with a coordinator")
	}

	// Readiness on but no coordinator -> inactive.
	ResetRoastRetryRegistrationForTest()
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	if RoastRetryActive() {
		t.Fatal("readiness on without a coordinator must yield inactive")
	}

	// Readiness on AND a coordinator -> active IFF a transition producer is built in
	// (frost_native). A frost_roast_retry && !frost_native build has no producer, so
	// ROAST stays inactive (legacy) even with readiness + a coordinator -- the
	// build-config gate from Codex P2-1.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		SelfMember:  1,
	})
	if RoastRetryActive() != roastTransitionProducerAvailable() {
		t.Fatalf(
			"readiness + coordinator: RoastRetryActive must equal producer availability (%v); got %v",
			roastTransitionProducerAvailable(), RoastRetryActive(),
		)
	}
}

// TestRoastRetryActiveForMember_GatesPerMember asserts per-member activation: a
// seat with a registered coordinator is active (given readiness + a producer); a
// seat WITHOUT one is inactive even when a sibling seat is registered.
func TestRoastRetryActiveForMember_GatesPerMember(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	RegisterRoastRetryCoordinatorForMember(1, RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinatorWithSigning(1, roast.NoOpSigner(), roast.NoOpSignatureVerifier()),
		SelfMember:  1,
	})

	// Member 1 active iff a producer is built in; member 2 (unregistered) never.
	if RoastRetryActiveForMember(1) != roastTransitionProducerAvailable() {
		t.Fatalf("member 1: active must equal producer availability (%v); got %v",
			roastTransitionProducerAvailable(), RoastRetryActiveForMember(1))
	}
	if RoastRetryActiveForMember(2) {
		t.Fatal("member 2 (unregistered) must be inactive even with a sibling registered")
	}

	t.Setenv(RoastRetryReadinessOptInEnvVar, "false")
	if RoastRetryActiveForMember(1) {
		t.Fatal("readiness off must yield inactive even for a registered member")
	}
}

// TestRoastRetryRegistration_PerMemberOverwriteAndCoexist asserts the per-member
// registry semantics (PR2b-1.5): registering the SAME member twice overwrites that
// member's entry, while DIFFERENT members coexist (a multi-seat operator registers
// one coordinator per local seat).
func TestRoastRetryRegistration_PerMemberOverwriteAndCoexist(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	coord1a := roast.NewInMemoryCoordinatorWithSigning(1, roast.NoOpSigner(), roast.NoOpSignatureVerifier())
	coord1b := roast.NewInMemoryCoordinatorWithSigning(1, roast.NoOpSigner(), roast.NoOpSignatureVerifier())
	coord2 := roast.NewInMemoryCoordinatorWithSigning(2, roast.NoOpSigner(), roast.NoOpSignatureVerifier())

	RegisterRoastRetryCoordinatorForMember(1, RoastRetryDeps{Coordinator: coord1a, SelfMember: 1})
	RegisterRoastRetryCoordinatorForMember(2, RoastRetryDeps{Coordinator: coord2, SelfMember: 2})
	RegisterRoastRetryCoordinatorForMember(1, RoastRetryDeps{Coordinator: coord1b, SelfMember: 1}) // overwrite 1

	got1, ok := RegisteredRoastRetryCoordinatorForMember(1)
	if !ok || got1.Coordinator != coord1b {
		t.Fatalf("member 1 must hold the later (overwriting) coordinator; ok=%v", ok)
	}
	got2, ok := RegisteredRoastRetryCoordinatorForMember(2)
	if !ok || got2.Coordinator != coord2 {
		t.Fatalf("member 2 must coexist with member 1; ok=%v", ok)
	}
}

// TestRoastRetryRegistration_RejectsSelfMemberMismatch asserts a coordinator
// registered under a member that does not match deps.SelfMember is rejected -- the
// coordinator is bound to deps.SelfMember at construction, so registering it under
// a different member would let it aggregate as the wrong seat.
func TestRoastRetryRegistration_RejectsSelfMemberMismatch(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	RegisterRoastRetryCoordinatorForMember(5, RoastRetryDeps{SelfMember: 3})
	if _, ok := RegisteredRoastRetryCoordinatorForMember(5); ok {
		t.Fatal("a SelfMember/member mismatch must not register")
	}
}

// TestRoastRetryRegistration_LegacyWrapperRegistersUnderSelfMember asserts the
// legacy single-arg RegisterRoastRetryCoordinator registers under deps.SelfMember,
// so existing single-seat callers + RegisteredRoastRetryCoordinator round-trip.
func TestRoastRetryRegistration_LegacyWrapperRegistersUnderSelfMember(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: 4})
	if _, ok := RegisteredRoastRetryCoordinatorForMember(4); !ok {
		t.Fatal("legacy register must place the entry under deps.SelfMember (4)")
	}
	if got, ok := RegisteredRoastRetryCoordinator(); !ok || got.SelfMember != 4 {
		t.Fatalf("legacy lookup must round-trip the single entry; got %d ok=%v", got.SelfMember, ok)
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

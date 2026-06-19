//go:build frost_roast_retry

package signing

import (
	"sync"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// RoastRetryDeps bundles the per-process dependencies the FROST
// receive loops need under the frost_roast_retry build tag. See the
// default-build file for the doc contract; this declaration is the
// real one used when the build tag is active.
type RoastRetryDeps struct {
	Coordinator roast.Coordinator
	Signer      roast.Signer
	Verifier    roast.SignatureVerifier
	SelfMember  uint32
}

// roastRetryRegistrationByMember holds one set of ROAST-retry dependencies PER
// local seat (member). A multi-seat operator registers one entry per member, each
// with a Coordinator bound to THAT member (deps.SelfMember == member) so whichever
// local seat is the elected ROAST coordinator for an attempt can aggregate; the
// Signer and Verifier are the shared operator key. A single-seat node has one
// entry. A later registration for the same member replaces the earlier one
// (runtime reconfiguration is intentional). RFC-21 Phase 7.3 PR2b-1.5.
var (
	roastRetryRegistrationMu       sync.RWMutex
	roastRetryRegistrationByMember = map[group.MemberIndex]RoastRetryDeps{}
)

// RegisterRoastRetryCoordinatorForMember stores the ROAST-retry dependencies for
// one local seat. deps.SelfMember MUST equal member: the Coordinator is bound to
// deps.SelfMember at construction, so registering it under a different member
// would let AggregateBundle run as the wrong seat. A mismatch is rejected with no
// registration (the seat stays ROAST-inactive -> legacy) rather than silently
// mis-binding.
//
// As a side effect, the first registration starts the session-handle sweeper
// goroutine that evicts orphaned bindings (defence-in-depth backstop); subsequent
// registrations do not restart it.
func RegisterRoastRetryCoordinatorForMember(member group.MemberIndex, deps RoastRetryDeps) {
	if member == 0 || deps.SelfMember != uint32(member) {
		// Member indices are 1-based; a coordinator bound to selfMember 0 is the
		// "disabled" sentinel that NEVER aggregates (coordinator_state.go), so
		// registering under member 0 -- or under any member that disagrees with
		// deps.SelfMember -- would silently mis-bind. REMOVE any existing entry for
		// this member so a bad re-registration deactivates the seat (fail-safe to
		// legacy) rather than leaving STALE deps active (Codex P2-2); member 0 never
		// has an entry, so the delete is a no-op there.
		roastRetryRegistrationMu.Lock()
		delete(roastRetryRegistrationByMember, member)
		roastRetryRegistrationMu.Unlock()
		return
	}
	roastRetryRegistrationMu.Lock()
	roastRetryRegistrationByMember[member] = deps
	roastRetryRegistrationMu.Unlock()
	StartSessionHandleSweeper()
}

// RegisteredRoastRetryCoordinatorForMember returns the dependencies registered for
// the given local seat and true, or the zero value and false if that seat has
// none. Member-aware receive/selection paths use this so a multi-seat operator's
// elected seat aggregates with its OWN coordinator.
func RegisteredRoastRetryCoordinatorForMember(member group.MemberIndex) (RoastRetryDeps, bool) {
	roastRetryRegistrationMu.RLock()
	defer roastRetryRegistrationMu.RUnlock()
	deps, ok := roastRetryRegistrationByMember[member]
	return deps, ok
}

// RegisterRoastRetryCoordinator is the legacy single-seat registration: it
// registers deps under deps.SelfMember. Kept for single-seat wiring and the
// existing test callers; production multi-seat wiring calls
// RegisterRoastRetryCoordinatorForMember once per local seat.
func RegisterRoastRetryCoordinator(deps RoastRetryDeps) {
	RegisterRoastRetryCoordinatorForMember(group.MemberIndex(deps.SelfMember), deps)
}

// RegisteredRoastRetryCoordinator is the legacy single-seat lookup: it returns
// SOME registered entry and true, or the zero value and false if none. For a
// single-seat node it returns that node's only entry; under multi-seat it returns
// an ARBITRARY entry (map order) and must not be used by member-aware code --
// those paths use RegisteredRoastRetryCoordinatorForMember. Kept for the readiness
// gate's any-registered check and single-seat/test compatibility.
func RegisteredRoastRetryCoordinator() (RoastRetryDeps, bool) {
	roastRetryRegistrationMu.RLock()
	defer roastRetryRegistrationMu.RUnlock()
	for _, deps := range roastRetryRegistrationByMember {
		return deps, true
	}
	return RoastRetryDeps{}, false
}

// ResetRoastRetryRegistrationForTest clears the registry. Exposed so tests in this
// and downstream packages can reset between cases without leaking state. Not
// intended for production code paths.
func ResetRoastRetryRegistrationForTest() {
	roastRetryRegistrationMu.Lock()
	defer roastRetryRegistrationMu.Unlock()
	roastRetryRegistrationByMember = map[group.MemberIndex]RoastRetryDeps{}
}

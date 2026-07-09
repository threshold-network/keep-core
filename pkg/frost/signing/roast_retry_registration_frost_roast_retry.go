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
	// KeyGroupID is the FROST key-group handle (AttemptContext.KeyGroupID) of the
	// wallet these deps belong to. It scopes the registration so a node controlling
	// more than one FROST wallet does not collide on the 1..N seat indices that
	// every wallet's group reuses. Empty string is the unscoped default used by
	// single-seat/legacy callers and tests.
	KeyGroupID string
}

// roastRetryRegistrationKey identifies one registration by the (wallet key group,
// seat) pair. Keying by seat alone let a second wallet's registration for the same
// seat index overwrite the first, so an honest node with multiple FROST wallets
// would aggregate one wallet's attempt with another wallet's coordinator. Scoping
// by key group isolates wallets while still isolating seats within a wallet.
type roastRetryRegistrationKey struct {
	keyGroupID string
	member     group.MemberIndex
}

// roastRetryRegistrationByKeyGroupMember holds one set of ROAST-retry dependencies
// PER (wallet key group, local seat). A multi-seat operator registers one entry per
// member of a wallet, each with a Coordinator bound to THAT member
// (deps.SelfMember == member) so whichever local seat is the elected ROAST
// coordinator for an attempt can aggregate; the Signer and Verifier are the shared
// operator key. A single-seat node has one entry per wallet. A later registration
// for the same (key group, member) replaces the earlier one (runtime
// reconfiguration is intentional). RFC-21 Phase 7.3 PR2b-1.5.
var (
	roastRetryRegistrationMu               sync.RWMutex
	roastRetryRegistrationByKeyGroupMember = map[roastRetryRegistrationKey]RoastRetryDeps{}
)

// RegisterRoastRetryCoordinatorForMember stores the ROAST-retry dependencies for
// one local seat of the wallet identified by deps.KeyGroupID. deps.SelfMember MUST
// equal member: the Coordinator is bound to deps.SelfMember at construction, so
// registering it under a different member would let AggregateBundle run as the
// wrong seat. A mismatch is rejected with no registration (the seat stays
// ROAST-inactive -> legacy) rather than silently mis-binding.
//
// As a side effect, the first registration starts the session-handle sweeper
// goroutine that evicts orphaned bindings (defence-in-depth backstop); subsequent
// registrations do not restart it.
func RegisterRoastRetryCoordinatorForMember(member group.MemberIndex, deps RoastRetryDeps) {
	key := roastRetryRegistrationKey{keyGroupID: deps.KeyGroupID, member: member}
	if member == 0 || deps.SelfMember != uint32(member) {
		// Member indices are 1-based; a coordinator bound to selfMember 0 is the
		// "disabled" sentinel that NEVER aggregates (coordinator_state.go), so
		// registering under member 0 -- or under any member that disagrees with
		// deps.SelfMember -- would silently mis-bind. REMOVE any existing entry for
		// this (key group, member) so a bad re-registration deactivates the seat
		// (fail-safe to legacy) rather than leaving STALE deps active (Codex P2-2);
		// member 0 never has an entry, so the delete is a no-op there.
		roastRetryRegistrationMu.Lock()
		delete(roastRetryRegistrationByKeyGroupMember, key)
		roastRetryRegistrationMu.Unlock()
		return
	}
	roastRetryRegistrationMu.Lock()
	roastRetryRegistrationByKeyGroupMember[key] = deps
	roastRetryRegistrationMu.Unlock()
	StartSessionHandleSweeper()
}

// RegisteredRoastRetryCoordinatorForKeyGroupMember returns the dependencies
// registered for a specific (wallet key group, local seat) and true, or the zero
// value and false if that pair has none. This is the AUTHORITATIVE lookup used by
// the coordinator-driving paths (the interactive drive, orchestration, observe, and
// the transition exchange), each of which has the attempt's KeyGroupID in hand, so
// a multi-wallet node always aggregates with the coordinator bound to the RIGHT
// wallet's seat.
func RegisteredRoastRetryCoordinatorForKeyGroupMember(
	keyGroupID string,
	member group.MemberIndex,
) (RoastRetryDeps, bool) {
	roastRetryRegistrationMu.RLock()
	defer roastRetryRegistrationMu.RUnlock()
	deps, ok := roastRetryRegistrationByKeyGroupMember[roastRetryRegistrationKey{
		keyGroupID: keyGroupID,
		member:     member,
	}]
	return deps, ok
}

// RegisteredRoastRetryCoordinatorForMember returns SOME registered entry for the
// given seat across ANY wallet key group, and true, or the zero value and false if
// no wallet has that seat registered. It is the seat-only view used by the coarse
// ROAST-vs-legacy activation gate (RoastRetryActiveForMember) at call sites that do
// not have a KeyGroupID in scope, and by tests. It must NOT be used to obtain the
// deps to aggregate with on a multi-wallet node -- that is what
// RegisteredRoastRetryCoordinatorForKeyGroupMember is for; a seat present in more
// than one wallet returns an arbitrary one here.
func RegisteredRoastRetryCoordinatorForMember(member group.MemberIndex) (RoastRetryDeps, bool) {
	roastRetryRegistrationMu.RLock()
	defer roastRetryRegistrationMu.RUnlock()
	for key, deps := range roastRetryRegistrationByKeyGroupMember {
		if key.member == member {
			return deps, true
		}
	}
	return RoastRetryDeps{}, false
}

// RegisterRoastRetryCoordinator is the legacy single-seat registration: it
// registers deps under (deps.KeyGroupID, deps.SelfMember). Kept for single-seat
// wiring and the existing test callers; production multi-seat wiring calls
// RegisterRoastRetryCoordinatorForMember once per local seat.
func RegisterRoastRetryCoordinator(deps RoastRetryDeps) {
	RegisterRoastRetryCoordinatorForMember(group.MemberIndex(deps.SelfMember), deps)
}

// RegisteredRoastRetryCoordinator is the legacy any-entry lookup: it returns SOME
// registered entry and true, or the zero value and false if none. Used by the
// process-wide ROAST-active check (RoastRetryActive) and the evidence recorder gate,
// neither of which is wallet-specific. Under multi-seat/multi-wallet it returns an
// ARBITRARY entry (map order) and must not be used by wallet- or member-aware code.
func RegisteredRoastRetryCoordinator() (RoastRetryDeps, bool) {
	roastRetryRegistrationMu.RLock()
	defer roastRetryRegistrationMu.RUnlock()
	for _, deps := range roastRetryRegistrationByKeyGroupMember {
		return deps, true
	}
	return RoastRetryDeps{}, false
}

// registeredRoastRetryMemberCount returns how many local seats of the given wallet
// key group currently have a coordinator registered. BeginOrchestrationForSession
// uses it for the one distinction that depends on per-wallet ROAST activation: when
// THIS seat has no coordinator for the attempt's key group, count==0 means ROAST is
// inactive for THIS wallet (safe legacy fallback) while count>0 means a sibling seat
// of the SAME wallet IS ROAST-active, so this unregistered seat must fail closed
// rather than fracture the attempt. Scoping the count to the key group keeps a
// second, independently-configured wallet from perturbing this wallet's decision.
func registeredRoastRetryMemberCount(keyGroupID string) int {
	roastRetryRegistrationMu.RLock()
	defer roastRetryRegistrationMu.RUnlock()
	count := 0
	for key := range roastRetryRegistrationByKeyGroupMember {
		if key.keyGroupID == keyGroupID {
			count++
		}
	}
	return count
}

// ResetRoastRetryRegistrationForTest clears the registry. Exposed so tests in this
// and downstream packages can reset between cases without leaking state. Not
// intended for production code paths.
func ResetRoastRetryRegistrationForTest() {
	roastRetryRegistrationMu.Lock()
	defer roastRetryRegistrationMu.Unlock()
	roastRetryRegistrationByKeyGroupMember = map[roastRetryRegistrationKey]RoastRetryDeps{}
}

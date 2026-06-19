//go:build !frost_roast_retry

package signing

import (
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// RoastRetryDeps bundles the per-process dependencies the FROST
// receive loops need to participate in RFC-21 Phase-4 coordinator-
// driven evidence flow:
//
//   - Coordinator drives BeginAttempt / RecordEvidence / AggregateBundle
//     / VerifyBundle / NextAttempt.
//   - Signer produces operator-key signatures over canonical
//     snapshot and bundle bytes.
//   - Verifier validates signatures on inbound snapshots and bundles.
//
// The type is exported in every build so callers can construct it
// without conditional compilation. In the default build the registry
// is a permanent no-op stub: the receive loops cannot find a
// registered coordinator and therefore fall back to the Phase-2
// `attempt.NoOpRecorder()` behaviour, preserving exact pre-RFC-21
// receive semantics.
//
// The real registry behind the `frost_roast_retry` build tag is in
// roast_retry_registration_frost_roast_retry.go.
type RoastRetryDeps struct {
	Coordinator roast.Coordinator
	Signer      roast.Signer
	Verifier    roast.SignatureVerifier
	// SelfMember is the local node's member index. The Coordinator
	// is already bound to this value via NewInMemoryCoordinatorWithSigning,
	// but receivers need it independently so they can correlate
	// AttemptHandles with their own snapshots in later Phase-4 PRs.
	SelfMember uint32
}

// RegisterRoastRetryCoordinator is a no-op in the default build.
// Callers in production code may invoke it unconditionally; the
// registration only takes effect when the `frost_roast_retry` build
// tag is active.
func RegisterRoastRetryCoordinator(_ RoastRetryDeps) {}

// RegisterRoastRetryCoordinatorForMember is a no-op in the default
// build. Production multi-seat wiring may invoke it unconditionally;
// the per-member registration only takes effect under the
// `frost_roast_retry` build tag.
func RegisterRoastRetryCoordinatorForMember(_ group.MemberIndex, _ RoastRetryDeps) {}

// RegisteredRoastRetryCoordinatorForMember returns (zero, false) in
// the default build: no ROAST-retry plumbing is active for any seat,
// so member-aware receivers use the Phase-2 NoOp fallback.
func RegisteredRoastRetryCoordinatorForMember(_ group.MemberIndex) (RoastRetryDeps, bool) {
	return RoastRetryDeps{}, false
}

// RegisteredRoastRetryCoordinator returns (zero, false) in the
// default build, signalling to receivers that ROAST-retry plumbing
// is not active and they should continue to use the Phase-2
// NoOpRecorder fallback.
func RegisteredRoastRetryCoordinator() (RoastRetryDeps, bool) {
	return RoastRetryDeps{}, false
}

// ResetRoastRetryRegistrationForTest is a no-op in the default
// build. Exposed so tests can call it unconditionally regardless of
// which build is active.
func ResetRoastRetryRegistrationForTest() {}

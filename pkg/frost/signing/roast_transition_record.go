package signing

import (
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// RoastTransitionRecord is the cross-attempt ROAST state the next attempt's
// participant selector consumes to compute its IncludedSet. It is produced by
// the elected-coordinator seat's orchestration cleanup when an attempt fails to
// aggregate, and carries everything NextAttempt needs WITHOUT depending on the
// live attempt-handle registry (which cleanup clears) or recomputing the DKG
// key (which would be nil at the selector):
//
//   - Bundle: the coordinator-signed TransitionMessage for the failed attempt.
//   - PreviousHandle: the failed attempt's handle (the selector calls
//     NextAttempt against it; it survives cleanup here so the selector is not
//     racing ClearCurrentAttemptHandleForSession).
//   - PreviousContext: the failed attempt's context (the handle's bound context).
//   - DkgGroupPublicKey: the group key NextAttempt needs to derive the next
//     attempt's seed; passing nil makes NextAttempt produce a context that
//     NewActiveRoastAttempt later rejects (seed mismatch).
//
// The type is defined in an untagged file because both the frost_roast_retry
// transition registry and the default-build no-op stub reference it (the
// orchestration cleanup, which calls RecordRoastTransition, compiles in every
// build).
type RoastTransitionRecord struct {
	Bundle            *roast.TransitionMessage
	PreviousHandle    roast.AttemptHandle
	PreviousContext   attempt.AttemptContext
	DkgGroupPublicKey []byte
}

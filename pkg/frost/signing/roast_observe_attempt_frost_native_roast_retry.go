//go:build frost_native && frost_roast_retry

package signing

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// ObserveAttemptForTransition begins a LOCAL "observe" attempt for the given
// signing request against the registered ROAST-retry coordinator and stores the
// resulting handle/context/dkg-key binding keyed by (roastSessionID, member,
// attemptContextHash).
//
// RFC-21 Phase 7.3 PR2b-1b: every local seat -- including ones excluded from the
// attempt -- observes each attempt so it holds a coordinator-instance-local
// handle to later VerifyBundle the attempt's transition bundle and run
// NextAttempt for participant selection (an AttemptHandle is not portable across
// coordinator instances, so a receiver cannot use another node's handle). The
// observe binding is DISTINCT from the active signing path's drive handle: it is
// produced here for the transition machinery, not for the FROST signing rounds.
//
// It is a no-op (returns nil) under the deterministic static conditions every
// honest node observes identically -- no coordinator registered, material not
// extractable, or an attempt context that cannot be constructed -- so the caller
// proceeds without an observe binding. Only a BeginAttempt failure (genuine
// runtime fault) is returned, so the caller can log it; in PR2b-1b's
// observe-only wiring a missing binding surfaces later as a fail-closed
// selection, never a divergent one.
func ObserveAttemptForTransition(
	request *Request,
) ([attempt.MessageDigestLength]byte, error) {
	var zeroHash [attempt.MessageDigestLength]byte
	if request == nil {
		return zeroHash, fmt.Errorf("observe attempt: request is nil")
	}

	// Respect the readiness opt-in gate, exactly as BeginOrchestrationForSession
	// does: when ROAST retry is opted out, observing is pointless (nothing
	// consumes the binding) and must stay inert. Opt-out is a deterministic
	// static condition every honest node sees identically.
	if err := EnsureRoastRetryReadinessOptIn(); err != nil {
		return zeroHash, nil
	}

	deps, ok := RegisteredRoastRetryCoordinator()
	if !ok || deps.Coordinator == nil {
		// No orchestration registered -- static fallback, every honest node
		// observes the same empty registry.
		return zeroHash, nil
	}

	signerMaterial, err := request.NativeSignerMaterial()
	if err != nil {
		// Material not extractable (e.g. a UniFFI v1 deployment) -- deterministic
		// static fallback.
		return zeroHash, nil
	}

	ffiRequest := &NativeExecutionFFISigningRequest{
		Message:           request.Message,
		SessionID:         request.SessionID,
		RoastSessionID:    request.RoastSessionID,
		MemberIndex:       request.MemberIndex,
		SignerMaterial:    signerMaterial,
		TaprootMerkleRoot: request.TaprootMerkleRoot,
		Attempt:           request.Attempt,
	}

	attemptCtx, err := BuildAttemptContextFromRequest(ffiRequest)
	if err != nil {
		// Deterministic per-input construction failure -- static fallback (matches
		// attemptRoastRetryOrchestrationFromRequest's handling).
		return zeroHash, nil
	}

	dkgGroupPublicKey, err := ExtractDkgGroupPublicKeyFromMaterial(signerMaterial)
	if err != nil {
		return zeroHash, nil
	}

	// Begin a local attempt to mint a coordinator-instance-local handle bound to
	// this attempt's context. BeginAttempt elects the coordinator deterministically
	// from the included set, so every honest seat's observe binding agrees on the
	// elected coordinator without exchanging anything.
	handle, err := deps.Coordinator.BeginAttempt(attemptCtx)
	if err != nil {
		return zeroHash, fmt.Errorf("observe attempt: begin attempt: %w", err)
	}

	// Key by the STABLE attemptCtx.SessionID (== RoastSessionID), matching the
	// transition-record registry, plus the attempt context hash so the transition
	// listener can resolve the binding from the hash an incoming bundle carries.
	recordObservedAttempt(
		attemptCtx.SessionID,
		request.MemberIndex,
		attemptCtx.Hash(),
		observedAttemptBinding{
			handle:            handle,
			context:           attemptCtx,
			dkgGroupPublicKey: dkgGroupPublicKey,
		},
	)
	return attemptCtx.Hash(), nil
}

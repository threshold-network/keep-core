//go:build frost_native && frost_roast_retry

package signing

import (
	"context"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/roast"
)

// driveInteractiveRoastSigningIfEnabled drives ONE interactive ROAST signing
// attempt for the local node when the audit gate is on, an interactive engine
// is registered, and ROAST orchestration is active for this session.
//
// Retry-loop ownership (per the Phase 7.3 executor-wiring design consult): the
// existing tBTC signingRetryLoop owns retries -- it re-invokes the executor
// (and therefore this helper) once per attempt with a fresh attempt context.
// This helper drives exactly one attempt; it never loops. On a runner failure
// it returns the error so the outer loop advances to the next attempt, and the
// deferred orchestration cleanup stashes the transition bundle the (later PR)
// blame/retry selector consumes.
//
// Return contract -- (signature, handled, error):
//   - (sig, true, nil)   the interactive attempt completed; the executor
//     returns sig and skips the coarse primitive.
//   - (nil, false, nil)  interactive signing is not enabled for this session
//     (gate off, no engine, or orchestration inactive); the
//     executor falls through to the coarse primitive.
//   - (nil, _, err)      the node had COMMITTED to interactive signing and a
//     step failed. This HARD-FAILS rather than silently
//     falling back to coarse: an honest node dropping to the
//     legacy path while peers proceed interactively would
//     fracture the signing group. The runner-failure case is
//     included here -- the outer loop retries the next
//     attempt.
//
// The two env gates (orchestration readiness + this audit gate) must be set
// consistently across the signing group; an inconsistent deployment splits the
// group exactly as an inconsistent KEEP_CORE_FROST_ROAST_RETRY_ENABLED would,
// and is an operator responsibility for this gated rollout.
func driveInteractiveRoastSigningIfEnabled(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
) (*frost.Signature, bool, error) {
	if logger == nil {
		logger = log.Logger("keep-frost-interactive-signing")
	}

	// Front door 1: the audit gate. Off -> coarse path, no diagnostics noise.
	if !InteractiveSigningOptInEnabled() {
		return nil, false, nil
	}

	// Front door 2: an engine provider must be registered. Absent is a
	// deployment-in-progress state (e.g. production has not registered the cgo
	// engine yet, or the audit has not cleared), NOT a runtime fault -> coarse.
	engine := registeredInteractiveSigningEngine()
	if engine == nil {
		logger.Infof(
			"interactive ROAST signing gated on but no engine registered "+
				"for session %q; using coarse path",
			request.SessionID,
		)
		return nil, false, nil
	}

	// Front door 3: orchestration must be active for this session. The handle
	// was minted and stashed by attemptRoastRetryOrchestrationFromRequest's
	// BeginOrchestrationForSession; its absence means the readiness gate is off,
	// no coordinator is registered, or the material was a static fallback ->
	// coarse path.
	handle, attemptCtx, ok := currentAttemptHandleForCollect(request.SessionID)
	if !ok {
		return nil, false, nil
	}
	deps, ok := RegisteredRoastRetryCoordinator()
	if !ok || deps.Coordinator == nil {
		return nil, false, nil
	}
	// deps.Coordinator is re-read from the registry rather than carried from the
	// handle's minter. A runtime re-registration between orchestration setup and
	// here would make NewActiveRoastAttempt's SelectedCoordinator(handle) return
	// ErrUnknownAttempt and hard-fail below -- safe (no wrong signature), and
	// only reachable by reconfiguring the coordinator mid-session.

	// From here the node has COMMITTED to interactive signing: gate on, engine
	// present, orchestration active. Every failure below HARD-FAILS.
	dkgGroupPublicKey, err := ExtractDkgGroupPublicKeyFromMaterial(request.SignerMaterial)
	if err != nil {
		return nil, true, fmt.Errorf(
			"interactive ROAST signing: extract dkg group public key: %w", err,
		)
	}

	threshold, err := interactiveRoastSigningThreshold(request)
	if err != nil {
		return nil, true, fmt.Errorf("interactive ROAST signing: %w", err)
	}

	active, err := NewActiveRoastAttempt(
		deps.Coordinator,
		handle,
		attemptCtx,
		request.SessionID,
		request.TaprootMerkleRoot,
		dkgGroupPublicKey,
	)
	if err != nil {
		return nil, true, fmt.Errorf("interactive ROAST signing: bind attempt: %w", err)
	}

	bus, err := NewBroadcastChannelRunnerBus(
		ctx, logger, request.Channel, request.MembershipValidator,
	)
	if err != nil {
		return nil, true, fmt.Errorf("interactive ROAST signing: build transport bus: %w", err)
	}

	collector := roast.NewRound2Collector(deps.Verifier)

	runner, err := newInteractiveSigningRunner(
		active,
		request.MemberIndex,
		threshold,
		engine,
		collector,
		deps.Coordinator,
		deps.Signer,
		bus,
	)
	if err != nil {
		return nil, true, fmt.Errorf("interactive ROAST signing: build runner: %w", err)
	}

	signatureBytes, err := runner.Run(ctx)
	if err != nil {
		// The attempt was driven and failed. Propagate so the outer tBTC
		// signingRetryLoop advances; the deferred orchestration cleanup stashes
		// the transition bundle for the next attempt's selector.
		return nil, true, fmt.Errorf("interactive ROAST signing attempt: %w", err)
	}

	signature, err := decodeBuildTaggedTBTCSignerSignature(signatureBytes)
	if err != nil {
		return nil, true, fmt.Errorf("interactive ROAST signing: decode signature: %w", err)
	}

	return signature, true, nil
}

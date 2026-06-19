//go:build frost_native

package signing

import (
	"context"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// driveInteractiveRoastSigningIfEnabled drives ONE interactive ROAST signing
// attempt for the local node, using the attempt handle + context minted for
// THIS Execute call by attemptRoastRetryOrchestrationFromRequest. The handle is
// passed in, NEVER looked up by session id: a multi-seat operator runs one
// concurrent Execute per seat (signing.go launches a goroutine per signer) and
// every seat shares one SessionID (signingSessionID is member-independent), so
// the session-keyed handle registry would return another seat's overwritten
// handle (or none after that seat's cleanup) -- which could make two seats mark
// the same handle succeeded, or drop one seat to coarse while peers stay
// interactive. Threading the minted handle keeps each seat bound to its own.
//
// Retry-loop ownership (per the Phase 7.3 executor-wiring design consult): the
// existing tBTC signingRetryLoop owns retries -- it re-invokes the executor
// (and therefore this helper) once per attempt with a fresh attempt context.
// This helper drives exactly one attempt; it never loops. On a runner failure
// it returns the error so the outer loop advances, and the deferred
// orchestration cleanup stashes the transition bundle the (later PR) blame/retry
// selector consumes.
//
// Return contract -- (signature, error):
//   - (sig, nil)  the interactive attempt completed; the executor returns sig
//     and skips the coarse primitive.
//   - (nil, nil)  interactive signing is not enabled for this session (audit
//     gate off, or no engine registered); the executor falls through to the
//     coarse primitive.
//   - (nil, err)  the node had COMMITTED to interactive signing and a step
//     failed. This HARD-FAILS rather than silently falling back to coarse: an
//     honest node dropping to the legacy path while peers proceed interactively
//     would fracture the signing group. The runner-failure case is included
//     here -- the outer loop retries the next attempt.
//
// The two env gates (orchestration readiness + this audit gate) must be set
// consistently across the signing group; an inconsistent deployment splits the
// group exactly as an inconsistent KEEP_CORE_FROST_ROAST_RETRY_ENABLED would,
// and is an operator responsibility for this gated rollout.
func driveInteractiveRoastSigningIfEnabled(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
	handle roast.AttemptHandle,
	attemptCtx attempt.AttemptContext,
) (*frost.Signature, error) {
	if logger == nil {
		logger = log.Logger("keep-frost-interactive-signing")
	}

	// Front door 1: the audit gate. Off -> coarse path, no diagnostics noise.
	if !InteractiveSigningOptInEnabled() {
		return nil, nil
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
		return nil, nil
	}

	// The coordinator deps are re-read from the registry (they were present when
	// BeginOrchestrationForSession minted the handle above). A mid-session
	// re-registration to a DIFFERENT coordinator would make NewActiveRoastAttempt's
	// SelectedCoordinator(handle) return ErrUnknownAttempt and hard-fail below --
	// safe (no wrong signature), and only reachable by reconfiguring the
	// coordinator mid-session. An absent registration falls back to coarse.
	deps, ok := RegisteredRoastRetryCoordinatorForMember(request.MemberIndex)
	if !ok || deps.Coordinator == nil {
		return nil, nil
	}

	// From here the node has COMMITTED to interactive signing: gate on, engine
	// present, orchestration active. Every failure below HARD-FAILS.
	dkgGroupPublicKey, err := ExtractDkgGroupPublicKeyFromMaterial(request.SignerMaterial)
	if err != nil {
		return nil, fmt.Errorf(
			"interactive ROAST signing: extract dkg group public key: %w", err,
		)
	}

	threshold, err := interactiveRoastSigningThreshold(request)
	if err != nil {
		return nil, fmt.Errorf("interactive ROAST signing: %w", err)
	}

	// Bind the attempt (and therefore the interactive engine session) to the
	// STABLE attemptCtx.SessionID, NOT request.SessionID: NewActiveRoastAttempt
	// requires sessionID == ctx.SessionID, and ctx.SessionID is the RoastSessionID
	// (the engine session is unified on the stable id - it separates attempts by
	// the canonical attempt id). request.SessionID is the attempt-specific coarse
	// id and would be rejected here.
	active, err := NewActiveRoastAttempt(
		deps.Coordinator,
		handle,
		attemptCtx,
		attemptCtx.SessionID,
		request.TaprootMerkleRoot,
		dkgGroupPublicKey,
	)
	if err != nil {
		return nil, fmt.Errorf("interactive ROAST signing: bind attempt: %w", err)
	}

	bus, err := NewBroadcastChannelRunnerBus(
		ctx, logger, request.Channel, request.MembershipValidator,
	)
	if err != nil {
		return nil, fmt.Errorf("interactive ROAST signing: build transport bus: %w", err)
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
		return nil, fmt.Errorf("interactive ROAST signing: build runner: %w", err)
	}

	signatureBytes, err := runner.Run(ctx)
	if err != nil {
		// The attempt was driven and failed. Propagate so the outer tBTC
		// signingRetryLoop advances; the deferred orchestration cleanup stashes
		// the transition bundle for the next attempt's selector.
		return nil, fmt.Errorf("interactive ROAST signing attempt: %w", err)
	}

	signature, err := decodeBuildTaggedTBTCSignerSignature(signatureBytes)
	if err != nil {
		return nil, fmt.Errorf("interactive ROAST signing: decode signature: %w", err)
	}

	return signature, nil
}

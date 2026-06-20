//go:build frost_native

package signing

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
)

// attemptRoastRetryOrchestrationFromRequest is the executor-adapter
// entry point for RFC-21 Phase-6 ROAST orchestration. It:
//
//  1. Builds an attempt.AttemptContext from the FFI signing
//     request (BuildAttemptContextFromRequest, gated frost_native).
//
//  2. If construction fails with ErrUnsupportedSignerMaterialFormat
//     -- e.g. the deployment still uses FrostUniFFIV1 material --
//     the failure is a STATIC configuration condition: every
//     honest signer with the same deployment material observes the
//     same error deterministically. Log at INFO and return
//     (nil, nil) so the executor proceeds without orchestration.
//
//  3. Any other AttemptContext construction error is a RUNTIME
//     failure (nil fields, malformed material payload, etc.). Per
//     the RFC-21 Phase-6 orchestration error taxonomy, runtime
//     errors must HARD FAIL to prevent group fracture: node A
//     falling back to legacy while node B proceeds with ROAST
//     would split the participant set on NextAttempt.
//
//  4. Calls BeginOrchestrationForSession with the context.
//     ErrRoastRetryReadinessOptOut and
//     ErrNoRoastRetryCoordinatorRegistered are static-configuration
//     errors -- log at INFO and return (nil, nil). Any other error
//     is treated as RUNTIME and propagated unchanged.
//
//  5. With orchestration active, drives ONE gated interactive ROAST
//     signing attempt (driveInteractiveRoastSigningIfEnabled) using
//     the handle minted HERE for this Execute call -- never a
//     session-keyed lookup, so concurrent multi-seat signers stay
//     bound to their own attempt. Returns the signature when the
//     interactive path handled signing; nil signature means the
//     executor falls through to the coarse primitive.
//
// The function returns (signature, cleanup, error):
//   - signature non-nil -> interactive signing produced it; executor returns it.
//   - signature nil + cleanup non-nil + error nil -> orchestration active but
//     interactive not enabled; defer cleanup, fall through to the coarse path.
//   - signature nil + cleanup nil + error nil -> static fallback; coarse path.
//   - error non-nil -> runtime/committed failure; propagate. cleanup may be
//     non-nil (interactive runner failure) so the caller defers it to stash the
//     failed attempt's transition bundle before returning the error.
func attemptRoastRetryOrchestrationFromRequest(
	execCtx context.Context,
	request *NativeExecutionFFISigningRequest,
	logger log.StandardLogger,
) (*frost.Signature, func(), error) {
	if logger == nil {
		// Defensive: existing executor-adapter tests pass nil here.
		// The helper logs static-fallback diagnostics, so a nil
		// logger must not panic the executor.
		logger = log.Logger("keep-frost-roast-orchestration")
	}
	attemptCtx, err := BuildAttemptContextFromRequest(request)
	if err != nil {
		// All BuildAttemptContextFromRequest errors are treated as
		// STATIC fallbacks because they are deterministic per-input:
		// the same NativeExecutionFFISigningRequest produces the
		// same construction outcome on every honest node, so
		// every node would make the same fall-back decision. The
		// RFC-21 Phase-6 hard-fail discipline applies only to
		// non-deterministic RUNTIME errors that originate inside
		// the Coordinator state machine (next branch).
		logger.Infof(
			"ROAST orchestration unavailable for session %q: %v",
			request.SessionID,
			err,
		)
		return nil, nil, nil
	}
	logger.Infof(
		"ROAST signer-material telemetry: session=%q key_group_id=%q signer_material_format=%q",
		request.SessionID,
		attemptCtx.KeyGroupID,
		request.SignerMaterial.Format,
	)

	// The handle registry is keyed by (request.SessionID, request.MemberIndex):
	// the coarse receive-loop binding validation + snapshot submission look the
	// handle up by that pair (RFC-21 Phase 7.3 PR2b-2), so a multi-seat operator's
	// sibling seats stay isolated. The cross-attempt transition record is produced
	// + keyed (by the stable RoastSessionID) entirely in the transition exchange
	// now, not here.
	handle, cleanup, err := BeginOrchestrationForSession(
		request.SessionID, request.MemberIndex, attemptCtx,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrRoastRetryReadinessOptOut),
			errors.Is(err, ErrNoRoastRetryCoordinatorRegistered):
			// Static-configuration errors -> safe to fall back.
			logger.Infof(
				"ROAST retry disabled for session %q: %v",
				request.SessionID,
				err,
			)
			return nil, nil, nil
		default:
			// Runtime failure: HARD FAIL.
			return nil, nil, fmt.Errorf(
				"ROAST orchestration: begin session %q: %w",
				request.SessionID,
				err,
			)
		}
	}

	// Orchestration is active. Drive ONE gated interactive attempt with the
	// handle minted HERE for this Execute (never a session-keyed lookup, so
	// concurrent multi-seat signers do not collide). A nil signature with nil
	// error means interactive signing is not enabled -> the executor falls
	// through to the coarse primitive. A drive error is a committed-path
	// failure: return it with the cleanup so the caller defers cleanup (stashing
	// the failed attempt's transition bundle) before propagating.
	signature, err := driveInteractiveRoastSigningIfEnabled(
		execCtx, logger, request, handle, attemptCtx,
	)
	if err != nil {
		return nil, cleanup, err
	}
	return signature, cleanup, nil
}

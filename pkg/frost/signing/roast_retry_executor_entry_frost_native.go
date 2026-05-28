//go:build frost_native

package signing

import (
	"errors"
	"fmt"

	"github.com/ipfs/go-log/v2"
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
//  5. On success returns the cleanup function the executor adapter
//     must defer.
//
// The function returns (cleanup, error):
//   - cleanup non-nil + error nil -> orchestration active; defer cleanup.
//   - cleanup nil + error nil      -> static fallback; proceed legacy.
//   - cleanup nil + error non-nil  -> runtime failure; propagate.
func attemptRoastRetryOrchestrationFromRequest(
	request *NativeExecutionFFISigningRequest,
	logger log.StandardLogger,
) (func(), error) {
	if logger == nil {
		// Defensive: existing executor-adapter tests pass nil here.
		// The helper logs static-fallback diagnostics, so a nil
		// logger must not panic the executor.
		logger = log.Logger("keep-frost-roast-orchestration")
	}
	ctx, err := BuildAttemptContextFromRequest(request)
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
		return nil, nil
	}
	logger.Infof(
		"ROAST signer-material telemetry: session=%q key_group_id=%q signer_material_format=%q",
		request.SessionID,
		ctx.KeyGroupID,
		request.SignerMaterial.Format,
	)

	handle, cleanup, err := BeginOrchestrationForSession(request.SessionID, ctx)
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
			return nil, nil
		default:
			// Runtime failure: HARD FAIL.
			return nil, fmt.Errorf(
				"ROAST orchestration: begin session %q: %w",
				request.SessionID,
				err,
			)
		}
	}
	_ = handle // Phase 6.4+ uses this for retry adapter invocation.
	return cleanup, nil
}

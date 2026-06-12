package signing

import (
	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastRetryLogger is the logger the snapshot-submission path uses
// for non-fatal diagnostics (submission failures, signature errors).
// A submission failure does not propagate to the signing flow:
// Phase 4 ships the submission code path unused in production, and
// even when wired (Phase 5+) a transient submission failure is
// recoverable by the next attempt's evidence flow.
var roastRetryLogger = log.Logger("keep-frost-roast-retry")

// submitSnapshotIfActive is invoked at end-of-collect to push the
// receive loop's accumulated evidence into the ROAST coordinator's
// RecordEvidence pipeline. The function is a no-op when any of the
// following is true:
//
//   - the ROAST-retry registry is empty (default build, no caller
//     has invoked RegisterRoastRetryCoordinator);
//   - no session-handle binding exists for sessionID (the typical
//     Phase-4 state, where the orchestration layer that calls
//     SetCurrentAttemptHandleForSession is not yet implemented);
//   - the recorder is a NoOp (no events were captured).
//
// When all three preconditions hold, the function builds a
// LocalEvidenceSnapshot, signs it with the registered Signer, and
// submits it via Coordinator.RecordEvidence. Errors at any step are
// logged at WARN level and otherwise swallowed -- snapshot
// submission must not break the receive loop's primary signing
// behaviour.
func submitSnapshotIfActive(
	sessionID string,
	recorder attempt.EvidenceRecorder,
) {
	if recorder == nil {
		return
	}
	deps, ok := RegisteredRoastRetryCoordinator()
	if !ok {
		return
	}
	handle, ctx, ok := currentAttemptHandleForCollect(sessionID)
	if !ok {
		return
	}
	evidence := recorder.Snapshot()
	if len(evidence.Overflows) == 0 {
		// Nothing observed worth submitting; emitting an empty
		// snapshot is still meaningful in the ROAST protocol
		// (proof-of-attendance) but adds noise to the bundle.
		// Phase 4.3 chooses to skip empty submissions; Phase 5
		// orchestration may revisit this if attestations need to
		// be unconditional.
		return
	}
	snap := buildSignedSnapshot(deps, ctx, evidence)
	if snap == nil {
		return
	}
	if err := deps.Coordinator.RecordEvidence(handle, snap); err != nil {
		roastRetryLogger.Warnf(
			"roast-retry: RecordEvidence failed for session %q: %v",
			sessionID,
			err,
		)
	}
}

// buildSignedSnapshot constructs and signs a LocalEvidenceSnapshot
// from the captured evidence. Returns nil and logs on signature
// failure; callers treat nil as "skip submission" and continue.
func buildSignedSnapshot(
	deps RoastRetryDeps,
	ctx attempt.AttemptContext,
	evidence attempt.Evidence,
) *roast.LocalEvidenceSnapshot {
	snap := roast.NewLocalEvidenceSnapshot(
		group.MemberIndex(deps.SelfMember),
		ctx.Hash(),
		evidence,
	)
	payload, err := snap.SignableBytes()
	if err != nil {
		roastRetryLogger.Warnf(
			"roast-retry: canonicalising snapshot failed: %v",
			err,
		)
		return nil
	}
	sig, err := deps.Signer.Sign(payload)
	if err != nil {
		roastRetryLogger.Warnf(
			"roast-retry: signing snapshot failed: %v",
			err,
		)
		return nil
	}
	snap.OperatorSignature = sig
	return snap
}

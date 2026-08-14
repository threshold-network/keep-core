//go:build frost_native

package signing

import (
	"errors"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// stashInteractiveShareBlame is the third blame-layer fault source (RFC-21 Phase
// 7.3, after PR2b-2's 2a coarse evidence + 2b coordinator-equivocation proofs): it
// turns the engine's interactive aggregate share-verification culprits into f+1
// reject accusations carried in the transition bundle.
//
// When InteractiveAggregate fails because a member submitted a bad FROST signature
// share, the engine names CANDIDATE culprits (pure crypto, no blame). This re-runs
// each candidate's RETAINED operator-signed share through the engine-backed
// Round2ShareVerifier: collector.ClassifyCandidateCulprits applies the frozen Q1
// boundary -- only an ACCEPTED retained share that re-verifies INVALID is blamed;
// every not-the-member's-fault condition (mis-binding, cross-attempt, wrong root,
// no retained share, divergent share, indeterminate) fails closed -- and the
// resulting reject accusations are stashed so BroadcastForcedSnapshot carries them
// in this seat's snapshot. computeNextAttempt's f+1 reject gate then excludes a
// member that enough honest observers independently re-verified as a bad-share
// submitter.
//
// f+1 (not instant, unlike 2b coordinator equivocation): a member's share is
// self-incriminating only against THE PACKAGE THIS OBSERVER ACCEPTED. A byzantine
// coordinator's targeted split (different packages to different members) could
// otherwise make one honest observer instant-exclude an honest peer; requiring f+1
// independent observers -- who re-verify identical retained bytes deterministically
// -- to agree closes that hole.
//
// Best-effort and fail-safe: a runErr that is not a share-verification failure, an
// engine without share re-verification, malformed candidates, a verifier-build
// failure, or an empty classification all stash nothing. It layers ON TOP of the
// 2b coordinator-proof stash for the same attempt -- the union pending-evidence
// entry carries both -- so a single failed attempt can publish reject evidence AND
// equivocation proofs.
func stashInteractiveShareBlame(
	runErr error,
	attemptCtx attempt.AttemptContext,
	request *NativeExecutionFFISigningRequest,
	collector *roast.Round2Collector,
	engine interactiveSigningEngine,
) {
	var shareErr *InteractiveAggregateShareVerificationError
	if !errors.As(runErr, &shareErr) || len(shareErr.CandidateCulprits) == 0 {
		return
	}
	// The engine-backed share re-verifier is an OPTIONAL capability (interface
	// segregation): absent (e.g. a deployment whose engine cannot re-verify shares)
	// -> skip share-blame. The 2b coordinator proofs were stashed separately.
	verifyEngine, ok := engine.(Round2ShareVerifyingEngine)
	if !ok {
		return
	}
	// Convert the engine's wire uint16 candidates to MemberIndex (uint8), dropping 0
	// and any value above the max member index: a malformed candidate must never
	// truncate into -- and so falsely blame -- an honest seat.
	candidates := make([]group.MemberIndex, 0, len(shareErr.CandidateCulprits))
	for _, c := range shareErr.CandidateCulprits {
		if c == 0 || c > uint16(group.MaxMemberIndex) {
			continue
		}
		candidates = append(candidates, group.MemberIndex(c))
	}
	if len(candidates) == 0 {
		return
	}

	attemptHash := attemptCtx.Hash()
	// The binding's SessionID is the STABLE engine/ROAST session
	// (attemptCtx.SessionID == active.SessionID()), consistent with attemptHash by
	// construction (both derive from attemptCtx) -- the verifier's hard
	// construction-time contract that keeps it from turning an honest share invalid.
	verifier, err := NewEngineRound2ShareVerifier(verifyEngine, Round2ShareVerificationBinding{
		SessionID:          attemptCtx.SessionID,
		AttemptContextHash: attemptHash,
		TaprootMerkleRoot:  request.TaprootMerkleRoot,
	})
	if err != nil {
		return
	}

	rejects, err := collector.ClassifyCandidateCulprits(attemptHash[:], candidates, verifier)
	if err != nil || len(rejects) == 0 {
		return
	}

	evidence := attempt.Evidence{
		Rejects: make(map[group.MemberIndex][]attempt.RejectEntry, len(rejects)),
	}
	for _, re := range rejects {
		if re.Count == 0 {
			continue
		}
		evidence.Rejects[re.Sender] = append(evidence.Rejects[re.Sender], attempt.RejectEntry{
			Reason: re.Reason,
			Count:  re.Count,
		})
	}
	if len(evidence.Rejects) == 0 {
		return
	}
	stashPendingEvidence(attemptCtx.SessionID, request.MemberIndex, attemptHash, evidence)
}

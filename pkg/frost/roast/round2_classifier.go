package roast

import (
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// candidateCulpritRejectReason is the canonical Reason recorded on a RejectEntry
// minted from an engine candidate culprit whose retained share re-verifies
// invalid. It is the Go-side reason string; the engine never supplies it.
const candidateCulpritRejectReason = "invalid_signature_share"

// ShareVerificationResult is the verdict of a FROST share re-verification.
//
// It is deliberately a three-way enum rather than a (bool, error): the boundary
// between a MEMBER-attributable failure (blame) and a not-the-member's-fault
// failure (don't blame) is security-critical, and a (bool, error) shape silently
// routes a member's own undecodable/garbage share bytes into the error channel -
// where the classifier would read them as "indeterminate" and let the cheater
// dodge blame. The enum forces the (engine-backed) implementer to categorize the
// failure boundary deliberately.
type ShareVerificationResult int

const (
	// ShareValid: the retained share is a valid FROST signature share for the
	// authoritative package. Not blamable.
	ShareValid ShareVerificationResult = iota
	// ShareInvalid: the share is MEMBER-attributable garbage - mathematically
	// invalid against the package, OR undecodable/malformed share bytes the
	// member operator-signed. Self-incriminating, hence blamable.
	ShareInvalid
	// ShareIndeterminate: verification could not be completed for a reason that
	// is NOT the member's fault - missing verifying material, ambiguous
	// key/taproot context, an undecodable AUTHORITATIVE PACKAGE, or an engine /
	// FFI execution failure. Fail closed against blame.
	ShareIndeterminate
)

// Round2ShareVerifier re-verifies a retained round-2 signature share against an
// attempt's authoritative signing package using FROST share verification.
//
// The Go host has no native FROST share crypto - the collector's
// SignatureVerifier checks OPERATOR signatures over envelopes, not the FROST
// share equation - so a faithful re-verification is engine-backed. This
// interface is that seam: implementations perform PURE crypto verification only
// (no envelope or operator-signature inspection, no blame), staying within the
// engine's crypto-only boundary (frozen Q1).
//
// VerifyRetainedShare reports whether submitter's retained, operator-signed
// share envelope is a valid FROST signature share for the authoritative signing
// package envelope the collector accepted. Implementations MUST map a member's
// OWN invalid or undecodable share bytes to ShareInvalid (it is self-incriminating
// member fault), and reserve ShareIndeterminate for failures that are not the
// member's fault (see the constants). Misclassifying undecodable member bytes as
// ShareIndeterminate would let a cheater escape blame.
//
// Implementations must be safe for concurrent calls from multiple goroutines.
type Round2ShareVerifier interface {
	VerifyRetainedShare(
		signingPackageEnvelope []byte,
		shareEnvelope []byte,
		submitter group.MemberIndex,
	) ShareVerificationResult
}

// classifierCandidate pairs a candidate culprit with the retained bytes the
// classifier needs to adjudicate it, snapshotted under the collector lock so the
// (engine-backed) re-verification runs lock-free. shareEnvelope is nil when the
// member has no ACCEPTED retained share for the attempt (divergent-only or
// absent), which the caller classifies as non-blamable.
type classifierCandidate struct {
	member        group.MemberIndex
	shareEnvelope []byte
}

// ClassifyCandidateCulprits turns the engine's candidate culprits for an attempt
// into this observer's reject accusations, applying the frozen Q1 boundary: the
// crypto-only engine reports who failed FROST verification against the package
// the coordinator aggregated, but only the Go host - against its OWN retained,
// operator-signed bytes - decides attributable member blame.
//
// For each candidate, against the attempt's authoritative package:
//
//   - an ACCEPTED retained share that re-verifies INVALID -> a RejectEntry. The
//     observer holds the member's operator-signed share and shows it invalid
//     against the package it accepted: self-incriminating, independently
//     checkable evidence (RFC-21 Layer B's "no bare counters" rule).
//   - an ACCEPTED retained share that re-verifies VALID (yet the engine flagged
//     it) -> nothing. The candidate is not self-incriminating under THIS
//     observer's retained package; the cause (a substituted package, share, or
//     root, or other coordinator input) is not provable from these bytes, so the
//     member must not be blamed. Coordinator-directed faults are a SEPARATE
//     adjudication path (Phase 7.2b-4b: package / divergent-share f+1
//     comparison), never inferred here.
//   - a DIVERGENT share only (validly signed but binding a different package /
//     coordinator) -> nothing. Kept NEUTRAL: a divergent share can be targeted
//     coordinator equivocation, so it must not alone permanently exclude its
//     member.
//   - no retained share at this observer -> nothing (nothing to corroborate).
//   - an INDETERMINATE re-verification -> nothing (fail closed against blame).
//
// The emitted accusations feed this observer's LocalEvidenceSnapshot.Rejects and
// hence NextAttempt's f+1 establishment gate; classification here never excludes
// anyone by itself. The result is deterministic - deduplicated and ascending by
// member, Count 1 each - so honest observers over identical retained bytes agree
// byte-for-byte. Returns ErrRound2UnknownAttempt if the attempt was never begun,
// ErrRound2NoSigningPackage if no authoritative package was recorded, and an
// error for a nil verifier.
func (c *Round2Collector) ClassifyCandidateCulprits(
	attemptContextHash []byte,
	candidates []group.MemberIndex,
	verifier Round2ShareVerifier,
) ([]RejectEntry, error) {
	if verifier == nil {
		return nil, fmt.Errorf(
			"roast: ClassifyCandidateCulprits requires a non-nil Round2ShareVerifier",
		)
	}

	// Snapshot the retained bytes the candidates need under the lock, then
	// release it before the (engine-backed, potentially slow) re-verification -
	// mirroring the collector's authenticate-outside-the-lock discipline. The
	// copies are collector-owned, so a concurrent PruneAttempt or record
	// mutation cannot race the lock-free re-verification below.
	signingPackageEnvelope, snapshot, err := c.snapshotCandidatesForClassification(
		attemptContextHash,
		candidates,
	)
	if err != nil {
		return nil, err
	}

	rejects := make([]RejectEntry, 0, len(snapshot))
	for _, candidate := range snapshot {
		if candidate.shareEnvelope == nil {
			// No ACCEPTED share retained here: a divergent-only share (NEUTRAL), a
			// member that never submitted to this observer, or an absent
			// submission. Nothing self-incriminating to accuse with.
			continue
		}
		// Blame ONLY a member-attributable invalid share. ShareValid (not
		// self-incriminating under this observer's package - coordinator-directed
		// faults are Phase 7.2b-4b), ShareIndeterminate (not the member's fault),
		// and any future verdict fail closed against blame.
		if verifier.VerifyRetainedShare(
			signingPackageEnvelope,
			candidate.shareEnvelope,
			candidate.member,
		) != ShareInvalid {
			continue
		}
		rejects = append(rejects, RejectEntry{
			Sender: candidate.member,
			Reason: candidateCulpritRejectReason,
			Count:  1,
		})
	}
	return rejects, nil
}

// snapshotCandidatesForClassification copies, under the collector lock, the
// authoritative package envelope and each deduplicated, ascending candidate's
// ACCEPTED retained share envelope, so re-verification can run lock-free.
// Divergent-only and absent candidates are carried with a nil share envelope.
func (c *Round2Collector) snapshotCandidatesForClassification(
	attemptContextHash []byte,
	candidates []group.MemberIndex,
) ([]byte, []classifierCandidate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, ok := c.attempts[round2AttemptKey(attemptContextHash)]
	if !ok {
		return nil, nil, ErrRound2UnknownAttempt
	}
	if record.signingPackageEnvelope == nil {
		return nil, nil, ErrRound2NoSigningPackage
	}

	signingPackageEnvelope := append([]byte(nil), record.signingPackageEnvelope...)

	// Deduplicate + sort so each member is adjudicated once, in a deterministic
	// order, regardless of how the engine ordered or repeated the candidates.
	seen := make(map[group.MemberIndex]struct{}, len(candidates))
	unique := make([]group.MemberIndex, 0, len(candidates))
	for _, member := range candidates {
		if _, dup := seen[member]; dup {
			continue
		}
		seen[member] = struct{}{}
		unique = append(unique, member)
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i] < unique[j] })

	snapshot := make([]classifierCandidate, 0, len(unique))
	for _, member := range unique {
		entry := classifierCandidate{member: member}
		if share, ok := record.shares[member]; ok && share != nil {
			entry.shareEnvelope = append([]byte(nil), share.envelope...)
		}
		snapshot = append(snapshot, entry)
	}
	return signingPackageEnvelope, snapshot, nil
}

// CoordinatorConflicts surfaces this observer's locally-detected coordinator
// equivocation for an attempt as a ConflictEntry accusation against the elected
// coordinator, for the observer's LocalEvidenceSnapshot.Conflicts ->
// NextAttempt's f+1 establishment gate (symmetric with ClassifyCandidateCulprits;
// it never excludes by itself). An established coordinator conflict excludes the
// coordinator, which auto-rotates it out of the next attempt's included set.
//
// The collector flags equivocation when the coordinator distributes two
// body-different signing packages for one attempt; both were individually
// authenticated as coordinator-signed before the conflict was recorded, so the
// accusation rests on unforgeable, self-incriminating proof (no re-verification
// needed, unlike candidate culprits). At most one entry - a single coordinator
// per attempt - with Count 1; empty when no conflict was observed.
//
// This catches only equivocation THIS observer directly received (the rare
// broadcast case). Targeted/split equivocation - different packages to disjoint
// members, so no single observer sees two - needs the cross-observer comparison
// (Phase 7.2b-4b-ii). Returns ErrRound2UnknownAttempt if the attempt was never
// begun.
func (c *Round2Collector) CoordinatorConflicts(attemptContextHash []byte) ([]ConflictEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, ok := c.attempts[round2AttemptKey(attemptContextHash)]
	if !ok {
		return nil, ErrRound2UnknownAttempt
	}
	if record.conflictingSigningPackageEnvelope == nil {
		return nil, nil
	}
	return []ConflictEntry{{Sender: record.electedCoordinator, Count: 1}}, nil
}

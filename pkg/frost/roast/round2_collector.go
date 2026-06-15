package roast

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// EquivocationKindSigningPackageConflict: the elected coordinator distributed
// two DIFFERENT signed signing packages for the same attempt. Two
// coordinator-signed envelopes with the same attempt context but different
// bytes are self-incriminating proof of coordinator equivocation.
const EquivocationKindSigningPackageConflict = "signing_package_conflict"

// EquivocationKindShareConflict: a submitter returned two DIFFERENT signed shares
// for the SAME authenticated instruction. Authentication pins coordinator_id,
// attempt, and the signing-package hash, so the differing field is the FROST
// signature_share itself - self-incriminating member double-signing. (A
// submission referencing a DIFFERENT package or coordinator is rejected at
// authentication, not flagged here; detecting that as targeted coordinator
// equivocation needs cross-package retention plus the f+1 quorum compare and is
// deferred to Phase 7.2b-4.)
const EquivocationKindShareConflict = "share_conflict"

// EquivocationKindDivergentShare: an included member submitted a validly-signed,
// attempt-bound share that does NOT bind the attempt's authoritative package
// (different signing-package hash or coordinator). It is retained as evidence of
// possible TARGETED coordinator equivocation (a package distributed to only some
// members) - the collector preserves the bytes but does NOT attribute fault
// locally; coordinator-vs-member classification is the f+1 quorum's job
// (Phase 7.2b-4). The evidence carries the divergent share (ConflictingEnvelope)
// and the attempt's authoritative signing-package envelope (ExistingEnvelope).
const EquivocationKindDivergentShare = "divergent_share"

// ErrRound2UnknownAttempt is returned by Round2Collector when no binding exists
// for an attempt (BeginAttempt was not called, or the attempt was pruned).
var ErrRound2UnknownAttempt = errors.New(
	"roast: round2 collector has no binding for this attempt",
)

// ErrRound2AttemptBindingConflict is returned by BeginAttempt when called again
// for the same attempt with a different elected coordinator or included set.
var ErrRound2AttemptBindingConflict = errors.New(
	"roast: round2 attempt binding conflicts with the existing one",
)

// ErrSigningPackageConflict is returned by RecordSigningPackage when a second
// authenticated package with a DIFFERENT signed body is recorded for an attempt
// that already has one - coordinator equivocation. The first package is retained
// (first-write-wins) and EquivocationEvidence is emitted.
var ErrSigningPackageConflict = errors.New(
	"roast: a different signing package was already recorded for this attempt (coordinator equivocation)",
)

// ErrRound2NoSigningPackage is returned by RecordShareSubmission when no signing
// package has been recorded for the attempt yet - a share cannot be bound or
// authenticated without the package it answers.
var ErrRound2NoSigningPackage = errors.New(
	"roast: no signing package recorded for this attempt yet",
)

// ErrRound2SubmitterNotIncluded is returned by RecordShareSubmission when the
// submitter is not in the attempt's included set.
var ErrRound2SubmitterNotIncluded = errors.New(
	"roast: share submitter is not in the attempt's included set",
)

// ErrShareConflict is returned by RecordShareSubmission when a submitter records
// a second share with a DIFFERENT signed body for an attempt - member
// equivocation. The first share is retained (first-write-wins) and
// EquivocationEvidence is emitted.
var ErrShareConflict = errors.New(
	"roast: a different share was already recorded by this submitter for this attempt (member equivocation)",
)

// ErrShareRetainedNotAccepted is returned by RecordShareSubmission when a share
// is genuinely submitter-signed for the attempt but does NOT bind the
// authoritative package/coordinator. It is RETAINED as divergent evidence (for
// Phase 7.2b-4) but is NOT an accepted aggregation share - the caller must not
// count it toward the signing threshold.
var ErrShareRetainedNotAccepted = errors.New(
	"roast: share retained as divergent evidence but not accepted (does not bind the authoritative package)",
)

// Round2Collector is the Go-side blame-input layer (RFC-21 Phase 7.2b). It
// retains the exact round-2 bytes seen for an attempt - the elected
// coordinator's signed signing package and (a later increment) members' signed
// share submissions - and flags equivocation by emitting EquivocationEvidence.
// The retained bytes are the input to f+1-quorum blame adjudication
// (Phase 7.2b-4); the Rust signing engine stays crypto-only.
//
// It is deliberately separate from the evidence/transition Coordinator: round-2
// signing is a distinct sub-protocol, and the blame layer must own the complete
// retained-byte set (packages and shares) that adjudication compares. It is
// keyed by the attempt context hash and does NOT fork coordinator selection -
// the caller supplies the elected coordinator (resolved via the same
// SelectCoordinator the Coordinator uses).
//
// Authentication runs outside the lock; equivocation evidence is emitted after
// the lock is released, mirroring the hardened snapshot path. Safe for
// concurrent use. Callers MUST PruneAttempt once an attempt concludes; the
// collector does not self-expire (it has no view of attempt lifecycle).
type Round2Collector struct {
	mu       sync.Mutex
	verifier SignatureVerifier
	attempts map[string]*round2Record
}

type round2Record struct {
	attemptContextHash []byte
	electedCoordinator group.MemberIndex
	includedSet        map[group.MemberIndex]struct{}
	// signingPackageEnvelope is a collector-OWNED copy of the attempt's
	// authoritative package's on-wire envelope (the first authenticated one);
	// nil until one is recorded. signingPackageBodyHash is its BodyHash - the
	// value a share submission binds to, and the identity used to detect
	// coordinator equivocation. The identity is the BODY, not the envelope: the
	// coordinator signature does not cover the outer envelope, so an unsigned
	// re-encoding of the same (body, signature) is not equivocation.
	signingPackageEnvelope []byte
	signingPackageBodyHash [sha256.Size]byte
	// shares retains each submitter's ACCEPTED share (binds the authoritative
	// package + elected coordinator): its BodyHash (the member-equivocation
	// identity) plus an owned copy of its envelope (evidence).
	shares map[group.MemberIndex]*round2ShareRecord
	// divergentShares retains each submitter's first validly-signed but
	// NON-authoritative-package-bound share - blame evidence of possible targeted
	// coordinator equivocation, not an aggregation input.
	divergentShares map[group.MemberIndex]*round2ShareRecord
	// conflictingSigningPackageEnvelope retains the FIRST body-different signing
	// package the elected coordinator distributed for this attempt (the
	// authoritative one is signingPackageEnvelope). Non-nil = the coordinator
	// equivocated: the observer holds two distinct, individually authenticated
	// coordinator-signed packages - unforgeable, self-incriminating proof.
	// Surfaced (with the authoritative package) as raw proof material by
	// CoordinatorPackageProofs for NextAttempt's proof-carrying coordinator
	// exclusion - never as a bare f+1 ConflictEntry.
	conflictingSigningPackageEnvelope []byte
}

// round2ShareRecord is a collector-owned record of one submitter's retained
// share: the body hash (identity, stable across envelope re-encoding) and an
// owned copy of the on-wire envelope (for equivocation evidence and blame).
type round2ShareRecord struct {
	bodyHash [sha256.Size]byte
	envelope []byte
}

// NewRound2Collector returns a collector that authenticates retained bytes with
// the given verifier.
func NewRound2Collector(verifier SignatureVerifier) *Round2Collector {
	return &Round2Collector{
		verifier: verifier,
		attempts: map[string]*round2Record{},
	}
}

func round2AttemptKey(attemptContextHash []byte) string {
	return hex.EncodeToString(attemptContextHash)
}

// BeginAttempt establishes the round-2 binding for an attempt: its elected
// coordinator and included set, resolved by the caller from the attempt context
// (the collector does not re-run coordinator selection). Idempotent for an
// identical binding; returns ErrRound2AttemptBindingConflict if called again
// with a different elected coordinator or included set.
func (c *Round2Collector) BeginAttempt(
	attemptContextHash []byte,
	electedCoordinator group.MemberIndex,
	includedSet []group.MemberIndex,
) error {
	if len(attemptContextHash) != attempt.MessageDigestLength {
		return fmt.Errorf(
			"round2: attempt context hash length [%d], expected [%d]",
			len(attemptContextHash),
			attempt.MessageDigestLength,
		)
	}
	if electedCoordinator == 0 {
		return errors.New("round2: elected coordinator is zero")
	}
	included := make(map[group.MemberIndex]struct{}, len(includedSet))
	for _, m := range includedSet {
		included[m] = struct{}{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	key := round2AttemptKey(attemptContextHash)
	if existing, ok := c.attempts[key]; ok {
		if existing.electedCoordinator != electedCoordinator ||
			!sameMemberSet(existing.includedSet, included) {
			return ErrRound2AttemptBindingConflict
		}
		return nil // idempotent re-begin
	}
	c.attempts[key] = &round2Record{
		attemptContextHash: append([]byte(nil), attemptContextHash...),
		electedCoordinator: electedCoordinator,
		includedSet:        included,
		shares:             map[group.MemberIndex]*round2ShareRecord{},
		divergentShares:    map[group.MemberIndex]*round2ShareRecord{},
	}
	return nil
}

// RecordSigningPackage authenticates a signed signing package against the
// attempt's binding (elected coordinator + attempt context) and retains it. The
// first authenticated package becomes the attempt's authoritative package - its
// BodyHash binds share submissions. Re-recording the same signed body is
// idempotent, including an unsigned envelope re-encoding of it. A second package
// with a DIFFERENT signed body for the same attempt is coordinator equivocation:
// the first is retained (first-write-wins), both envelopes are emitted as
// EquivocationEvidence, and ErrSigningPackageConflict is returned. A package
// that fails authentication is rejected without retention. Returns
// ErrRound2UnknownAttempt if BeginAttempt was not called.
func (c *Round2Collector) RecordSigningPackage(pkg *SigningPackage) error {
	if pkg == nil {
		return errors.New("round2: nil signing package")
	}
	key := round2AttemptKey(pkg.AttemptContextHash)

	c.mu.Lock()
	record, ok := c.attempts[key]
	if !ok {
		c.mu.Unlock()
		return ErrRound2UnknownAttempt
	}
	elected := record.electedCoordinator
	attemptHash := append([]byte(nil), record.attemptContextHash...)
	c.mu.Unlock()

	// Authenticate outside the lock (verification is the expensive step). A
	// failed package is forgeable noise - reject without retaining.
	if err := AuthenticateSigningPackage(c.verifier, pkg, elected, attemptHash); err != nil {
		return err
	}
	// Identity is the BODY hash, not the envelope: the coordinator signature does
	// not cover the outer envelope, so an unsigned re-encoding of the same
	// (body, signature) must NOT count as a different package.
	bodyHash, err := pkg.BodyHash()
	if err != nil {
		return err
	}
	// Own a defensive copy of the envelope: the caller may reuse pkg (e.g. a
	// receive loop calling Unmarshal for the next message), which would mutate
	// the retained bytes out from under us.
	envelope, err := pkg.Marshal()
	if err != nil {
		return err
	}
	ownedEnvelope := append([]byte(nil), envelope...)

	var evidence *EquivocationEvidence
	c.mu.Lock()
	record, ok = c.attempts[key]
	if !ok {
		c.mu.Unlock()
		return ErrRound2UnknownAttempt // pruned concurrently
	}
	if record.electedCoordinator != elected {
		// Binding changed under us (prune + re-begin): the authentication we ran
		// no longer matches this record.
		c.mu.Unlock()
		return ErrRound2UnknownAttempt
	}
	switch {
	case record.signingPackageEnvelope == nil:
		record.signingPackageEnvelope = ownedEnvelope
		record.signingPackageBodyHash = bodyHash
	case record.signingPackageBodyHash == bodyHash:
		// Idempotent: the same signed body re-recorded (possibly re-encoded).
	default:
		// A different signed body for the same attempt: coordinator equivocation.
		// Keep the first authoritative package; retain the first conflicting one
		// (idempotent) as self-incriminating proof, and emit both.
		if record.conflictingSigningPackageEnvelope == nil {
			record.conflictingSigningPackageEnvelope = ownedEnvelope
		}
		evidence = &EquivocationEvidence{
			Kind:                EquivocationKindSigningPackageConflict,
			AttemptContextHash:  append([]byte(nil), record.attemptContextHash...),
			Sender:              elected,
			ExistingEnvelope:    append([]byte(nil), record.signingPackageEnvelope...),
			ConflictingEnvelope: signingPackageEnvelopeForEvidence(pkg),
		}
	}
	c.mu.Unlock()

	if evidence != nil {
		emitEquivocationEvidence(*evidence)
		return ErrSigningPackageConflict
	}
	return nil
}

// RecordShareSubmission verifies and retains a member's signed share submission.
// The attempt must already have a recorded signing package
// (ErrRound2NoSigningPackage) and the submitter must be in the included set
// (ErrRound2SubmitterNotIncluded, checked cheaply before signature verification).
// The submitter signature is verified over the attempt-bound body; a share not
// genuinely submitter-signed for this attempt is rejected without retention.
//
// A verified share is classified:
//   - ACCEPTED if it binds the elected coordinator AND the authoritative signing
//     package (its BodyHash): retained as an aggregation share, first-write-wins.
//     A second ACCEPTED share with a different body from the same submitter is
//     member double-signing -> EquivocationKindShareConflict, ErrShareConflict.
//   - DIVERGENT otherwise (validly signed for the attempt but bound to a
//     different package/coordinator): retained SEPARATELY as evidence of possible
//     targeted coordinator equivocation -> EquivocationKindDivergentShare,
//     ErrShareRetainedNotAccepted (retained, NOT counted toward the threshold).
//     Fault is not attributed here; the f+1 quorum decides in Phase 7.2b-4.
//
// Re-recording the same body (accepted or divergent) is idempotent, including an
// unsigned envelope re-encoding.
func (c *Round2Collector) RecordShareSubmission(sub *ShareSubmission) error {
	if sub == nil {
		return errors.New("round2: nil share submission")
	}
	// Validate up front so SubmitterID() is bounded (uint32 -> MemberIndex) for
	// the membership pre-gate below. Membership is a cheap gate before the
	// expensive signature verify: it rejects validly-signed-but-not-included
	// junk without burning CPU. It is safe to gate here because the attempt
	// context hash cryptographically commits to the included set, so the set for
	// a given attempt key cannot change under us.
	if err := sub.Validate(); err != nil {
		return fmt.Errorf("share submission failed structural validation: %w", err)
	}
	key := round2AttemptKey(sub.AttemptContextHash)
	submitter := sub.SubmitterID()

	c.mu.Lock()
	record, ok := c.attempts[key]
	if !ok {
		c.mu.Unlock()
		return ErrRound2UnknownAttempt
	}
	if record.signingPackageEnvelope == nil {
		c.mu.Unlock()
		return ErrRound2NoSigningPackage
	}
	if _, included := record.includedSet[submitter]; !included {
		c.mu.Unlock()
		return ErrRound2SubmitterNotIncluded
	}
	elected := record.electedCoordinator
	attemptHash := append([]byte(nil), record.attemptContextHash...)
	pkgBodyHash := record.signingPackageBodyHash
	c.mu.Unlock()

	// Verify the submitter signature OUTSIDE the lock (the expensive step). This
	// is the WEAKER check: it does not require the share to bind the authoritative
	// package or elected coordinator, because a submitter-signed share that
	// diverges from the authoritative package is retained as evidence (see
	// EquivocationKindDivergentShare), never dropped.
	if err := verifyShareSubmissionForAttempt(c.verifier, sub, attemptHash); err != nil {
		return err
	}
	// Identity is the share BODY hash, not the envelope - an unsigned re-encoding
	// of the same signed body must NOT count as a conflicting share.
	bodyHash, err := sub.BodyHash()
	if err != nil {
		return err
	}
	envelope, err := sub.Marshal()
	if err != nil {
		return err
	}
	ownedEnvelope := append([]byte(nil), envelope...)
	// Accepted = binds the elected coordinator AND the authoritative package
	// (eligible for aggregation). Otherwise the share is divergent: retained as
	// targeted-equivocation evidence, not an aggregation input.
	accepted := sub.CoordinatorID() == elected &&
		bytes.Equal(sub.SigningPackageHash, pkgBodyHash[:])

	var (
		evidence *EquivocationEvidence
		result   error
	)
	c.mu.Lock()
	record, ok = c.attempts[key]
	if !ok {
		c.mu.Unlock()
		return ErrRound2UnknownAttempt
	}
	if record.electedCoordinator != elected || record.signingPackageBodyHash != pkgBodyHash {
		// Binding or authoritative package changed under us (prune + re-begin).
		c.mu.Unlock()
		return ErrRound2UnknownAttempt
	}
	if accepted {
		switch existing, present := record.shares[submitter]; {
		case !present:
			record.shares[submitter] = &round2ShareRecord{
				bodyHash: bodyHash,
				envelope: ownedEnvelope,
			}
		case existing.bodyHash == bodyHash:
			// Idempotent: the same accepted share re-recorded (possibly re-encoded).
		default:
			// A different accepted share body from the same submitter: member
			// double-signing the same instruction. Keep the first; emit both.
			evidence = &EquivocationEvidence{
				Kind:                EquivocationKindShareConflict,
				AttemptContextHash:  append([]byte(nil), record.attemptContextHash...),
				Sender:              submitter,
				ExistingEnvelope:    append([]byte(nil), existing.envelope...),
				ConflictingEnvelope: ownedEnvelope,
			}
			result = ErrShareConflict
		}
	} else {
		// Divergent: retain as evidence (first per submitter) and flag it WITHOUT
		// attributing fault - the f+1 quorum (7.2b-4) decides coordinator vs
		// member. Always reported as retained-not-accepted so the caller does not
		// count it toward the threshold.
		result = ErrShareRetainedNotAccepted
		switch existing, present := record.divergentShares[submitter]; {
		case !present:
			record.divergentShares[submitter] = &round2ShareRecord{
				bodyHash: bodyHash,
				envelope: ownedEnvelope,
			}
			evidence = divergentShareEvidence(record, submitter, ownedEnvelope)
		case existing.bodyHash == bodyHash:
			// Idempotent: the same divergent share re-recorded.
		default:
			// A second, different divergent share from the same submitter: re-flag
			// (keep the first retained).
			evidence = divergentShareEvidence(record, submitter, ownedEnvelope)
		}
	}
	c.mu.Unlock()

	if evidence != nil {
		emitEquivocationEvidence(*evidence)
	}
	return result
}

// divergentShareEvidence builds EquivocationKindDivergentShare evidence: the
// divergent share envelope plus the attempt's authoritative signing-package
// envelope for context. The caller holds the lock.
func divergentShareEvidence(
	record *round2Record,
	submitter group.MemberIndex,
	divergentEnvelope []byte,
) *EquivocationEvidence {
	return &EquivocationEvidence{
		Kind:                EquivocationKindDivergentShare,
		AttemptContextHash:  append([]byte(nil), record.attemptContextHash...),
		Sender:              submitter,
		ExistingEnvelope:    append([]byte(nil), record.signingPackageEnvelope...),
		ConflictingEnvelope: append([]byte(nil), divergentEnvelope...),
	}
}

// PruneAttempt drops all retained round-2 state for an attempt. Callers invoke
// it when an attempt concludes (success or abandonment) to bound retention.
func (c *Round2Collector) PruneAttempt(attemptContextHash []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.attempts, round2AttemptKey(attemptContextHash))
}

func sameMemberSet(a, b map[group.MemberIndex]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for m := range a {
		if _, ok := b[m]; !ok {
			return false
		}
	}
	return true
}

// signingPackageEnvelopeForEvidence encodes a package's signed envelope for
// evidence retention, tolerating encode failures (nil result) so the detection
// path never degrades. Mirrors snapshotEnvelopeForEvidence.
func signingPackageEnvelopeForEvidence(pkg *SigningPackage) []byte {
	if pkg == nil {
		return nil
	}
	envelope, err := pkg.Marshal()
	if err != nil {
		equivocationLogger.Warnf(
			"could not encode signing package envelope for evidence retention: [%v]",
			err,
		)
		return nil
	}
	// Defensive copy: Marshal returns the package's internal cache, but evidence
	// bytes are handed to an external observer that may retain/mutate them.
	return append([]byte(nil), envelope...)
}

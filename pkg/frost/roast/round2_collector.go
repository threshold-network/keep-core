package roast

import (
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
	// shares retains each submitter's authenticated share (populated by the
	// next increment).
	shares map[group.MemberIndex]*ShareSubmission
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
		shares:             map[group.MemberIndex]*ShareSubmission{},
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
		// Keep the first; emit both.
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

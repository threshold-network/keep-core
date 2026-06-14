package roast

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ErrShareSubmissionWrongCoordinator is returned by AuthenticateShareSubmission
// when a share names a coordinator other than the attempt's elected coordinator
// (RFC-21 Annex A). A member that resolved a different coordinator (e.g. under a
// partition) must not have its share accepted into this attempt.
var ErrShareSubmissionWrongCoordinator = errors.New(
	"roast: share submission coordinator is not the attempt's elected coordinator",
)

// ErrShareSubmissionWrongAttempt is returned when a share's attempt_context_hash
// does not match the live attempt.
var ErrShareSubmissionWrongAttempt = errors.New(
	"roast: share submission attempt context hash does not match the live attempt",
)

// ErrShareSubmissionWrongPackage is returned when a share's signing_package_hash
// does not match the signing package the coordinator distributed for the attempt
// - the share answers a different or stale package.
var ErrShareSubmissionWrongPackage = errors.New(
	"roast: share submission signing package hash does not match the live package",
)

// SignShareSubmission signs sub with the submitting member's operator key,
// setting sub.SubmitterSignature over sub.SignableBytes() (the domain-tagged
// body). A member calls this after authenticating the signing package and
// accepting its taproot root, to return its round-2 share. sub must be
// structurally valid (call Validate first).
func SignShareSubmission(signer Signer, sub *ShareSubmission) error {
	payload, err := sub.SignableBytes()
	if err != nil {
		return err
	}
	signature, err := signer.Sign(payload)
	if err != nil {
		return fmt.Errorf("roast: sign share submission: %w", err)
	}
	sub.SubmitterSignature = signature
	return nil
}

// AuthenticateShareSubmission verifies that sub is a genuine round-2 share from
// its declared submitter, for this exact attempt and package: it names
// electedCoordinator, its attempt_context_hash matches the live attempt, its
// signing_package_hash matches the package the coordinator distributed
// (liveSigningPackageHash), and its signature verifies under the submitter's
// operator key over the domain-tagged body. (electedCoordinator and
// liveSigningPackageHash are resolved by the caller from the attempt and the
// distributed SignedSigningPackage - see SigningPackage.EnvelopeHash.)
//
// The signature check is over sub.SubmitterID(), so a forged submitter_id does
// not verify: the signature binds the declared submitter to the actual signer.
//
// A submission that passes is attributable to its submitter and bound to the
// package, so the caller MUST retain its exact received bytes for the
// cross-member equivocation comparison (Phase 7.2b-4). A submission that fails
// any check is forgeable or misdirected noise: the caller rejects it WITHOUT
// retaining it. Membership of the submitter in the included set and de-dup of
// repeated shares are the caller's responsibility, not this function's.
func AuthenticateShareSubmission(
	verifier SignatureVerifier,
	sub *ShareSubmission,
	electedCoordinator group.MemberIndex,
	liveAttemptContextHash []byte,
	liveSigningPackageHash []byte,
) error {
	// Structurally validate first: this is an authentication boundary for
	// untrusted input, and the checks below use the truncating ID accessor and
	// bytes.Equal. A manually-assembled (un-Unmarshaled) submission must be
	// rejected before any field is trusted - e.g. a submitter_id that truncates
	// to another member (uint32 -> uint8), or empty hashes that would make
	// bytes.Equal(nil, nil) pass.
	if err := sub.Validate(); err != nil {
		return fmt.Errorf("share submission failed structural validation: %w", err)
	}
	if len(sub.SubmitterSignature) == 0 {
		return fmt.Errorf(
			"%w: share submission has no submitter signature",
			ErrSignatureMissing,
		)
	}
	if sub.CoordinatorID() != electedCoordinator {
		return fmt.Errorf(
			"%w: share coordinator %d, elected %d",
			ErrShareSubmissionWrongCoordinator,
			sub.CoordinatorID(),
			electedCoordinator,
		)
	}
	if !bytes.Equal(sub.AttemptContextHash, liveAttemptContextHash) {
		return ErrShareSubmissionWrongAttempt
	}
	if !bytes.Equal(sub.SigningPackageHash, liveSigningPackageHash) {
		return ErrShareSubmissionWrongPackage
	}
	payload, err := sub.SignableBytes()
	if err != nil {
		return fmt.Errorf("share submission signable bytes: %w", err)
	}
	if err := verifier.Verify(
		payload,
		sub.SubmitterSignature,
		sub.SubmitterID(),
	); err != nil {
		return fmt.Errorf(
			"%w: submitter %d: %s",
			ErrSignatureInvalid,
			sub.SubmitterID(),
			err.Error(),
		)
	}
	return nil
}

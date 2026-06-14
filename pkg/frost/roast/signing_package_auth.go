package roast

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ErrSigningPackageWrongCoordinator is returned by AuthenticateSigningPackage
// when a signed signing package names a coordinator other than the attempt's
// elected coordinator (RFC-21 Annex A). attempt_context_hash is public, so any
// operator could sign a body carrying it; an envelope from a non-elected
// coordinator is not attributable to the coordinator and MUST NOT be retained.
var ErrSigningPackageWrongCoordinator = errors.New(
	"roast: signing package coordinator is not the attempt's elected coordinator",
)

// ErrSigningPackageWrongAttempt is returned when a signed signing package's
// attempt_context_hash does not match the live attempt.
var ErrSigningPackageWrongAttempt = errors.New(
	"roast: signing package attempt context hash does not match the live attempt",
)

// SignSigningPackage signs pkg with the elected coordinator's operator key,
// setting pkg.CoordinatorSignature over pkg.SignableBytes() (the domain-tagged
// body). The elected coordinator calls this before distributing the
// SignedSigningPackage to the chosen signing subset. pkg must be structurally
// valid (call Validate first).
func SignSigningPackage(signer Signer, pkg *SigningPackage) error {
	payload, err := pkg.SignableBytes()
	if err != nil {
		return err
	}
	signature, err := signer.Sign(payload)
	if err != nil {
		return fmt.Errorf("roast: sign signing package: %w", err)
	}
	pkg.CoordinatorSignature = signature
	return nil
}

// AuthenticateSigningPackage verifies that pkg is genuine evidence from the
// attempt's elected coordinator: it names electedCoordinator, its signature
// verifies under that coordinator's operator key over the domain-tagged body,
// and its attempt_context_hash matches the live attempt. (electedCoordinator
// is resolved by the caller from the attempt via SelectCoordinator, exactly as
// Coordinator.VerifyBundle resolves the bundle coordinator.)
//
// A package that passes is attributable to the coordinator, so the member MUST
// retain its exact received bytes - the section-3 cross-member equivocation
// comparison needs them - BEFORE deciding whether to sign over it. A package
// that fails any check is forgeable noise: the caller rejects it WITHOUT
// retaining it.
//
// This deliberately does NOT check the taproot root. Root binding is the
// separate sign/no-sign decision (see MatchesRoot): a root-divergent but
// genuine-coordinator envelope is still retained as equivocation evidence and
// then refused, so root verification must not gate retention here.
func AuthenticateSigningPackage(
	verifier SignatureVerifier,
	pkg *SigningPackage,
	electedCoordinator group.MemberIndex,
	liveAttemptContextHash []byte,
) error {
	// Structurally validate first (authentication boundary): reject a
	// manually-assembled package before the truncating ID accessor or bytes.Equal
	// checks below trust any field - e.g. a coordinator_id that truncates to the
	// elected member (uint32 -> uint8). Mirrors AuthenticateShareSubmission.
	if err := pkg.Validate(); err != nil {
		return fmt.Errorf("signing package failed structural validation: %w", err)
	}
	if len(pkg.CoordinatorSignature) == 0 {
		return fmt.Errorf(
			"%w: signing package has no coordinator signature",
			ErrSignatureMissing,
		)
	}
	if pkg.CoordinatorID() != electedCoordinator {
		return fmt.Errorf(
			"%w: package coordinator %d, elected %d",
			ErrSigningPackageWrongCoordinator,
			pkg.CoordinatorID(),
			electedCoordinator,
		)
	}
	if !bytes.Equal(pkg.AttemptContextHash, liveAttemptContextHash) {
		return ErrSigningPackageWrongAttempt
	}
	payload, err := pkg.SignableBytes()
	if err != nil {
		return fmt.Errorf("signing package signable bytes: %w", err)
	}
	if err := verifier.Verify(
		payload,
		pkg.CoordinatorSignature,
		pkg.CoordinatorID(),
	); err != nil {
		return fmt.Errorf(
			"%w: coordinator %d: %s",
			ErrSignatureInvalid,
			pkg.CoordinatorID(),
			err.Error(),
		)
	}
	return nil
}

// MatchesRoot reports whether the package's taproot_merkle_root equals the
// live session/signing root (both empty for a key-path spend). After
// authenticating and retaining a package, a member signs over its
// signing_package ONLY when this is true: a divergent root means the
// coordinator is committing the subset to a tweaked key other than the
// session's, so the member refuses to sign and the retained envelope stands as
// root-equivocation evidence for the section-3 comparison.
func (p *SigningPackage) MatchesRoot(liveRoot []byte) bool {
	return bytes.Equal(p.TaprootMerkleRoot, liveRoot)
}

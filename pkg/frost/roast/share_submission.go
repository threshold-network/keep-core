package roast

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// SignedShareSubmissionType is the net.TaggedUnmarshaler Type() string for a
// member's signed Round2 share submission.
const SignedShareSubmissionType = roastMessageTypePrefix + "signed_share_submission"

// shareSubmissionSignatureDomain is the fixed domain-separation tag prefixed to
// the bytes the submitting member signs (see SignableBytes). A member's operator
// key also signs evidence snapshots, and the elected coordinator's operator key
// signs transition messages and signing packages; each signed body must be
// non-confusable. Like the other ROAST signed bodies, the tag BEGINS with byte
// 0x00 - an illegal protobuf tag (field number 0) - so the signed payload is
// undecodable as any protobuf message: a share-submission signature cannot be
// accepted on another envelope (whose decoder rejects the 0x00-leading body),
// and another body's signature cannot verify against domain || body (a genuine
// protobuf body starts with a valid tag, >= 0x08). The tag is NOT carried on
// the wire - signer and verifier prepend the same constant.
var shareSubmissionSignatureDomain = []byte("\x00roast/signed-share-submission/v1\x00")

// SigningPackageHashLength is the byte length of the signing_package_hash that
// binds a share submission to the package it answers - a SHA-256 of the
// authenticated SignedSigningPackage envelope.
const SigningPackageHashLength = 32

// MaxSignatureShareBytes caps the embedded FROST signature share. A round-2
// share is a single scalar (~32 bytes); the cap leaves generous headroom for
// other schemes while rejecting pathological payloads at Unmarshal time.
const MaxSignatureShareBytes = 256

// MaxSignedShareSubmissionBytes bounds a whole SignedShareSubmission envelope so
// Unmarshal can reject a grossly oversized message before proto.Unmarshal
// materializes it. Sized as the share cap plus the operator-signature cap plus
// generous framing for the two 32-byte hashes and field overhead.
const MaxSignedShareSubmissionBytes = MaxSignatureShareBytes + MaxOperatorSignatureBytes + 512

// ShareSubmission is a member's Round2 FROST signature share bound to the
// attempt and to the exact SignedSigningPackage envelope the member
// authenticated, signed with the member's operator key. The binding is the
// hard prerequisite for blame adjudication (Phase 7.2b-4).
type ShareSubmission struct {
	// AttemptContextHash binds the submission to one attempt. Exactly 32 bytes.
	AttemptContextHash []byte
	// SubmitterIDValue is the submitting member's index. A wire uint32; it must
	// fit group.MemberIndex (uint8), enforced by Validate.
	SubmitterIDValue uint32
	// CoordinatorIDValue is the elected coordinator this share is authorized
	// for, as resolved when authenticating the signing package. A wire uint32
	// bounded to group.MemberIndex by Validate.
	CoordinatorIDValue uint32
	// SigningPackageHash is the 32-byte hash of the SignedSigningPackage
	// envelope (body plus coordinator signature) this share answers - the exact
	// bytes the member retained. Assumes canonical operator signatures (see the
	// proto for the malleability caveat).
	SigningPackageHash []byte
	// SignatureShare is the serialized FROST round-2 signature share.
	SignatureShare []byte
	// SubmitterSignature is the submitting member's operator-key signature over
	// SignableBytes(): the share-submission domain tag followed by the
	// serialized ShareSubmissionBody.
	SubmitterSignature []byte

	// bodyCache caches the exact serialized body bytes carried on the wire:
	// marshaled once at signing time for a self-authored submission, or the
	// received bytes verbatim for a parsed one. Fields must not be mutated once
	// set.
	bodyCache []byte
	// signaturePayloadCache caches the domain-tagged bytes the
	// SubmitterSignature covers (shareSubmissionSignatureDomain || bodyCache);
	// primed at decode and never carried on the wire.
	signaturePayloadCache []byte
	// wireEnvelope caches the exact on-wire envelope (body + signature): the
	// received bytes verbatim for parsed submissions, or built once after
	// signing for self-authored ones.
	wireEnvelope []byte
}

func shareSubmissionBodyMessage(p *ShareSubmission) *pb.ShareSubmissionBody {
	return &pb.ShareSubmissionBody{
		AttemptContextHash: p.AttemptContextHash,
		SubmitterId:        p.SubmitterIDValue,
		CoordinatorId:      p.CoordinatorIDValue,
		SigningPackageHash: p.SigningPackageHash,
		SignatureShare:     p.SignatureShare,
	}
}

func shareSubmissionFieldsFromBody(p *ShareSubmission, body *pb.ShareSubmissionBody) {
	p.AttemptContextHash = append([]byte(nil), body.AttemptContextHash...)
	p.SubmitterIDValue = body.SubmitterId
	p.CoordinatorIDValue = body.CoordinatorId
	p.SigningPackageHash = append([]byte(nil), body.SigningPackageHash...)
	p.SignatureShare = append([]byte(nil), body.SignatureShare...)
}

// SubmitterID returns the submitting member index as a group.MemberIndex.
// Validate (or Unmarshal) must have confirmed it fits.
func (p *ShareSubmission) SubmitterID() group.MemberIndex {
	return group.MemberIndex(p.SubmitterIDValue)
}

// CoordinatorID returns the authorized coordinator index as a
// group.MemberIndex. Validate (or Unmarshal) must have confirmed it fits.
func (p *ShareSubmission) CoordinatorID() group.MemberIndex {
	return group.MemberIndex(p.CoordinatorIDValue)
}

// bodyBytes returns the exact serialized ShareSubmissionBody - the body carried
// verbatim in the SignedShareSubmission envelope. Marshaled once and cached for
// a self-authored submission; the received bytes verbatim for a parsed one. The
// returned slice is the internal cache - callers must not mutate it.
func (p *ShareSubmission) bodyBytes() ([]byte, error) {
	if p == nil {
		return nil, errors.New("roast: cannot encode a nil share submission")
	}
	if p.bodyCache != nil {
		return p.bodyCache, nil
	}
	body, err := proto.Marshal(shareSubmissionBodyMessage(p))
	if err != nil {
		return nil, fmt.Errorf("roast: marshal share submission body: %w", err)
	}
	p.bodyCache = body
	return body, nil
}

// SignableBytes returns the exact byte stream the SubmitterSignature covers: the
// share-submission domain tag (shareSubmissionSignatureDomain) followed by the
// serialized ShareSubmissionBody. The domain tag is a fixed constant prepended
// by both signer and verifier and is NOT carried on the wire - it
// domain-separates this signature from the node's other signed bodies. The body
// half is the bytes that travel: marshaled once for a self-authored submission,
// or the received body verbatim for a parsed one (verify exactly what was
// received). Fields must not be mutated afterwards, and the returned slice is
// the internal cache - callers must not mutate it.
func (p *ShareSubmission) SignableBytes() ([]byte, error) {
	if p == nil {
		return nil, errors.New("roast: cannot encode a nil share submission")
	}
	if p.signaturePayloadCache != nil {
		return p.signaturePayloadCache, nil
	}
	body, err := p.bodyBytes()
	if err != nil {
		return nil, err
	}
	payload := make([]byte, 0, len(shareSubmissionSignatureDomain)+len(body))
	payload = append(payload, shareSubmissionSignatureDomain...)
	payload = append(payload, body...)
	p.signaturePayloadCache = payload
	return payload, nil
}

// Type implements net.TaggedUnmarshaler.
func (p *ShareSubmission) Type() string {
	return SignedShareSubmissionType
}

// Marshal serialises the submission as a SignedShareSubmission envelope: the
// serialized ShareSubmissionBody plus the submitter signature (which covers the
// domain-tagged body, see SignableBytes). For a submission parsed off the wire
// the received envelope is returned verbatim, so the bytes a verifier retains
// for the section-3 equivocation comparison are exactly the bytes it received.
// The submission must be signed first. The returned slice is the internal
// cache - callers must not mutate it.
func (p *ShareSubmission) Marshal() ([]byte, error) {
	if p.wireEnvelope != nil {
		return p.wireEnvelope, nil
	}
	if len(p.SubmitterSignature) == 0 {
		return nil, errors.New(
			"roast: share submission must be signed before wire encoding",
		)
	}
	body, err := p.bodyBytes()
	if err != nil {
		return nil, err
	}
	envelope, err := proto.Marshal(&pb.SignedShareSubmission{
		Body:               body,
		SubmitterSignature: p.SubmitterSignature,
	})
	if err != nil {
		return nil, fmt.Errorf("roast: marshal share submission envelope: %w", err)
	}
	p.wireEnvelope = envelope
	return envelope, nil
}

// Unmarshal parses a SignedShareSubmission envelope, retains the received body
// and envelope bytes verbatim (the submitter signature is verified over exactly
// these bytes), populates the fields from the body, and validates the structure.
func (p *ShareSubmission) Unmarshal(data []byte) error {
	// Bound the input before allocating: reject a grossly oversized envelope
	// before proto.Unmarshal materializes it (and before the copies below), so
	// the caps protect memory rather than only rejecting after the fact.
	if len(data) > MaxSignedShareSubmissionBytes {
		return fmt.Errorf(
			"signed share submission: envelope length [%d] exceeds cap [%d]",
			len(data),
			MaxSignedShareSubmissionBytes,
		)
	}
	var envelope pb.SignedShareSubmission
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("signed share submission: parse envelope: %w", err)
	}
	if len(envelope.Body) == 0 {
		return errors.New("signed share submission: empty body")
	}
	var body pb.ShareSubmissionBody
	if err := proto.Unmarshal(envelope.Body, &body); err != nil {
		return fmt.Errorf("signed share submission: parse body: %w", err)
	}
	// Enforce the share cap on the parsed field before copying it into the
	// struct (and before caching the body), so an over-cap field is rejected
	// without the extra allocations.
	if len(body.SignatureShare) > MaxSignatureShareBytes {
		return fmt.Errorf(
			"signed share submission: signatureShare length [%d] exceeds cap [%d]",
			len(body.SignatureShare),
			MaxSignatureShareBytes,
		)
	}
	shareSubmissionFieldsFromBody(p, &body)
	p.SubmitterSignature = append([]byte(nil), envelope.SubmitterSignature...)
	p.bodyCache = append([]byte(nil), envelope.Body...)
	p.wireEnvelope = append([]byte(nil), data...)
	// Prime the signable-bytes cache from the body just received, discarding any
	// cache a prior SignableBytes call left on a reused value. Priming here -
	// rather than lazily in SignableBytes - keeps concurrent signature
	// verification of a parsed submission race-free.
	p.signaturePayloadCache = nil
	if _, err := p.SignableBytes(); err != nil {
		return err
	}
	return p.Validate()
}

// Validate runs the structural checks Unmarshal applies after a decode. Exposed
// so callers that construct submissions in memory can validate without a
// marshal/unmarshal round-trip.
func (p *ShareSubmission) Validate() error {
	if len(p.AttemptContextHash) != attempt.MessageDigestLength {
		return fmt.Errorf(
			"share submission: attemptContextHash length [%d], expected [%d]",
			len(p.AttemptContextHash),
			attempt.MessageDigestLength,
		)
	}
	if p.SubmitterIDValue == 0 {
		return errors.New("share submission: submitterID is zero")
	}
	if p.SubmitterIDValue > group.MaxMemberIndex {
		return fmt.Errorf(
			"share submission: submitterID [%d] exceeds max member index [%d]",
			p.SubmitterIDValue,
			group.MaxMemberIndex,
		)
	}
	if p.CoordinatorIDValue == 0 {
		return errors.New("share submission: coordinatorID is zero")
	}
	if p.CoordinatorIDValue > group.MaxMemberIndex {
		return fmt.Errorf(
			"share submission: coordinatorID [%d] exceeds max member index [%d]",
			p.CoordinatorIDValue,
			group.MaxMemberIndex,
		)
	}
	if len(p.SigningPackageHash) != SigningPackageHashLength {
		return fmt.Errorf(
			"share submission: signingPackageHash length [%d], expected [%d]",
			len(p.SigningPackageHash),
			SigningPackageHashLength,
		)
	}
	if len(p.SignatureShare) == 0 {
		return errors.New("share submission: signatureShare is empty")
	}
	if len(p.SignatureShare) > MaxSignatureShareBytes {
		return fmt.Errorf(
			"share submission: signatureShare length [%d] exceeds cap [%d]",
			len(p.SignatureShare),
			MaxSignatureShareBytes,
		)
	}
	if len(p.SubmitterSignature) > MaxOperatorSignatureBytes {
		return fmt.Errorf(
			"share submission: submitterSignature length [%d] exceeds cap [%d]",
			len(p.SubmitterSignature),
			MaxOperatorSignatureBytes,
		)
	}
	return nil
}

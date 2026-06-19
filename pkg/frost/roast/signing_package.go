package roast

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// SignedSigningPackageType is the stable Type() string for the
// coordinator-distributed, operator-signed signing package (Phase 7.2b).
const SignedSigningPackageType = roastMessageTypePrefix + "signed_signing_package"

// signingPackageSignatureDomain is the fixed domain-separation tag prefixed
// to the bytes the coordinator signs (see SignableBytes). The elected
// coordinator's operator key also signs TransitionMessage and evidence-snapshot
// bodies, and a SigningPackageBody is wire-compatible with a
// TransitionMessageBody (matching tags/types for attempt_context_hash and
// coordinator_id, and a length-delimited field 3), so the signed byte streams
// must not be confusable.
//
// The tag BEGINS with byte 0x00 - an illegal protobuf tag (field number 0) -
// which separates the domains in BOTH directions without relying on field
// layout:
//
//   - A signing-package signature cannot be accepted on another envelope:
//     presenting these signed bytes as that envelope's body fails when the
//     receiver decodes the body, because every signed-body decoder
//     proto.Unmarshals it and an illegal leading tag is rejected. (A
//     valid-protobuf ASCII tag does NOT give this: a parser skips it as an
//     unknown length-delimited field and resumes into a transition body
//     crafted inside signing_package.)
//   - Another message's signature cannot be accepted on a signing package:
//     the signature is verified over signingPackageSignatureDomain || body,
//     which begins with 0x00, whereas a serialized protobuf body always begins
//     with a valid field tag (>= 0x08), so the two signed byte streams differ
//     in their first byte and the signature cannot verify.
//
// The tag is NOT carried on the wire - it is a fixed constant both signer and
// verifier prepend.
var signingPackageSignatureDomain = []byte("\x00roast/signed-signing-package/v1\x00")

// MaxSigningPackageBytes caps the embedded FROST SigningPackage length,
// rejecting pathological payloads at Unmarshal time so a misbehaving
// coordinator cannot exhaust receiver memory. Sized for a worst-case
// production signing subset's round-1 commitments plus generous headroom.
const MaxSigningPackageBytes = 1 << 20 // 1 MiB

// TaprootMerkleRootLength is the byte length of a taproot script-tree
// root. A SigningPackage carries either exactly this many bytes or none
// (the key-path case).
const TaprootMerkleRootLength = 32

// MaxSignedSigningPackageBytes bounds a whole SignedSigningPackage envelope so
// Unmarshal can reject a grossly oversized message before proto.Unmarshal
// materializes it (and before the body/field copies). Sized as the
// signing-package cap plus the coordinator-signature cap plus generous
// framing/field overhead, so a legitimate maximum-size package still fits.
const MaxSignedSigningPackageBytes = MaxSigningPackageBytes + MaxCoordinatorSignatureBytes + 512

// SigningPackage is the coordinator-distributed signing package for one
// attempt, carried as a signed-body envelope (Phase 7.2b, frozen spec
// section 6). The elected coordinator signs the exact serialized
// SigningPackageBody with its operator key and distributes the
// SignedSigningPackage to the chosen signing subset; each member verifies
// the coordinator signature over the exact bytes it received and only then
// parses them - the same sign-what-you-transmit / verify-what-you-received
// discipline as the evidence envelopes (wire.go).
//
// This file defines the wire type and its byte-preservation contract only.
// Coordinator-side signing/distribution and member-side authentication
// (elected-coordinator check, signature verification, root binding) and
// retention land in later Phase 7.2b increments; the engine never sees this
// envelope (frozen spec: blame adjudication is Go-side).
type SigningPackage struct {
	// AttemptContextHash binds the package to one attempt. Always exactly
	// 32 bytes (attempt.MessageDigestLength).
	AttemptContextHash []byte
	// CoordinatorIDValue is the elected coordinator's member index
	// (RFC-21 Annex A). A member authenticates the envelope by checking
	// this equals the attempt's elected coordinator and that the
	// signature verifies under that coordinator's operator key.
	CoordinatorIDValue uint32
	// SigningPackageBytes is the serialized FROST SigningPackage the
	// chosen subset signs over.
	SigningPackageBytes []byte
	// TaprootMerkleRoot is the taproot script-tree root the signature is
	// tweaked by: exactly 32 bytes, or empty for a key-path spend.
	TaprootMerkleRoot []byte
	// SignerIDsValue is the wire (uint32) form of the chosen signing subset's
	// member indices the FROST SigningPackageBytes was built over (RFC-21 Phase
	// 7.3 t-of-included finalize): ascending, distinct, each a valid member index.
	// It lets non-coordinators know which members to await round-2 shares from when
	// the package covers a t-subset of the included set. The SigningPackageBytes is
	// the cryptographic source of truth, so a coordinator that lies here causes
	// only a liveness failure (aggregate fails closed), never a wrong signature or
	// false blame. Empty for the full-included flow. Use SignerIDs() for the
	// validated member-index form.
	SignerIDsValue []uint32
	// CoordinatorSignature is the elected coordinator's operator-key
	// signature over SignableBytes().
	CoordinatorSignature []byte

	// bodyCache caches the exact serialized SigningPackageBody: marshaled
	// once at signing time for a self-authored package, or the received body
	// bytes verbatim for a parsed one. This is the body field carried on the
	// wire; fields must not be mutated once set.
	bodyCache []byte
	// signaturePayloadCache caches the exact bytes the CoordinatorSignature
	// covers - the domain tag followed by the body (see SignableBytes).
	signaturePayloadCache []byte
	// wireEnvelope caches the exact on-wire envelope (body + signature):
	// the received bytes verbatim for parsed packages, or built once
	// after signing for self-authored ones.
	wireEnvelope []byte
}

func signingPackageBodyMessage(p *SigningPackage) *pb.SigningPackageBody {
	return &pb.SigningPackageBody{
		AttemptContextHash: p.AttemptContextHash,
		CoordinatorId:      p.CoordinatorIDValue,
		SigningPackage:     p.SigningPackageBytes,
		TaprootMerkleRoot:  p.TaprootMerkleRoot,
		SignerIds:          p.SignerIDsValue,
	}
}

func signingPackageFieldsFromBody(p *SigningPackage, body *pb.SigningPackageBody) {
	p.AttemptContextHash = append([]byte(nil), body.AttemptContextHash...)
	p.CoordinatorIDValue = body.CoordinatorId
	p.SigningPackageBytes = append([]byte(nil), body.SigningPackage...)
	p.TaprootMerkleRoot = append([]byte(nil), body.TaprootMerkleRoot...)
	p.SignerIDsValue = append([]uint32(nil), body.SignerIds...)
}

// SignableBytes returns the exact byte stream the CoordinatorSignature
// covers: the signing-package domain tag (see signingPackageSignatureDomain)
// followed by the serialized SigningPackageBody. The domain tag is a fixed
// constant prepended by both signer and verifier and is NOT carried on the
// wire - it domain-separates this signature from the coordinator's
// transition-message signatures (whose body is otherwise wire-compatible).
// The body half is the bytes the package transmits: marshaled once for a
// self-authored package, or the received body verbatim for a parsed one
// (verify exactly what was received). Fields must not be mutated afterwards,
// and the returned slice is the internal cache - callers must not mutate it.
func (p *SigningPackage) SignableBytes() ([]byte, error) {
	if p == nil {
		return nil, errors.New("roast: cannot encode a nil signing package")
	}
	if p.signaturePayloadCache != nil {
		return p.signaturePayloadCache, nil
	}
	body, err := p.bodyBytes()
	if err != nil {
		return nil, err
	}
	payload := make([]byte, 0, len(signingPackageSignatureDomain)+len(body))
	payload = append(payload, signingPackageSignatureDomain...)
	payload = append(payload, body...)
	p.signaturePayloadCache = payload
	return payload, nil
}

// bodyBytes returns the exact serialized SigningPackageBody - the body field
// carried in the SignedSigningPackage envelope. Marshaled once and cached for
// a self-authored package; the received bytes verbatim for a parsed one. The
// returned slice is the internal cache - callers must not mutate it.
func (p *SigningPackage) bodyBytes() ([]byte, error) {
	if p == nil {
		return nil, errors.New("roast: cannot encode a nil signing package")
	}
	if p.bodyCache != nil {
		return p.bodyCache, nil
	}
	body, err := proto.Marshal(signingPackageBodyMessage(p))
	if err != nil {
		return nil, fmt.Errorf("roast: marshal signing package body: %w", err)
	}
	p.bodyCache = body
	return body, nil
}

// Type implements net.TaggedUnmarshaler.
func (p *SigningPackage) Type() string {
	return SignedSigningPackageType
}

// Marshal serialises the package as a SignedSigningPackage envelope: the
// serialized SigningPackageBody plus the coordinator signature (which covers
// the domain-tagged body, see SignableBytes). For a package parsed off the
// wire the received envelope is returned verbatim, so the bytes a member
// retains for the section-3 equivocation comparison are the exact bytes it
// received. The package must be signed first. The returned slice is the
// internal cache - callers must not mutate it.
func (p *SigningPackage) Marshal() ([]byte, error) {
	if p == nil {
		return nil, errors.New("roast: cannot marshal a nil signing package")
	}
	if p.wireEnvelope != nil {
		return p.wireEnvelope, nil
	}
	if len(p.CoordinatorSignature) == 0 {
		return nil, errors.New(
			"roast: signing package must be signed before wire encoding",
		)
	}
	body, err := p.bodyBytes()
	if err != nil {
		return nil, err
	}
	envelope, err := proto.Marshal(&pb.SignedSigningPackage{
		Body:                 body,
		CoordinatorSignature: p.CoordinatorSignature,
	})
	if err != nil {
		return nil, fmt.Errorf("roast: marshal signing package envelope: %w", err)
	}
	p.wireEnvelope = envelope
	return envelope, nil
}

// BodyHash returns the SHA-256 of the package's signed body bytes - the value a
// ShareSubmission commits to in signing_package_hash, and the identity used to
// detect coordinator equivocation. It hashes the BODY (the serialized
// SigningPackageBody the coordinator signs), NOT the on-wire envelope: the
// coordinator signature does not cover the outer envelope, so an unsigned
// re-encoding of the same (body, signature) would change an envelope hash
// without being equivocation, and would fragment share bindings across members.
// The body bytes are stable - any re-serialization of the body would fail
// signature verification - so honest envelope re-encodings map to the same
// BodyHash. For a package parsed off the wire this hashes the received body
// verbatim.
func (p *SigningPackage) BodyHash() ([sha256.Size]byte, error) {
	body, err := p.bodyBytes()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(body), nil
}

// Unmarshal parses a SignedSigningPackage envelope, retains the received
// body and envelope bytes verbatim (the coordinator signature is verified
// over exactly these bytes), populates the fields from the body, and
// validates the structure.
func (p *SigningPackage) Unmarshal(data []byte) error {
	if p == nil {
		return errors.New("roast: cannot unmarshal into a nil signing package")
	}
	// Bound the input before allocating: reject a grossly oversized envelope
	// before proto.Unmarshal materializes it (and before the copies below), so
	// the MaxSigningPackageBytes cap protects memory rather than only rejecting
	// after the fact.
	if len(data) > MaxSignedSigningPackageBytes {
		return fmt.Errorf(
			"signed signing package: envelope length [%d] exceeds cap [%d]",
			len(data),
			MaxSignedSigningPackageBytes,
		)
	}
	var envelope pb.SignedSigningPackage
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("signed signing package: parse envelope: %w", err)
	}
	if len(envelope.Body) == 0 {
		return errors.New("signed signing package: empty body")
	}
	var body pb.SigningPackageBody
	if err := proto.Unmarshal(envelope.Body, &body); err != nil {
		return fmt.Errorf("signed signing package: parse body: %w", err)
	}
	// Enforce the signing-package cap on the parsed field before copying it
	// into the struct (and before caching the body), so an over-cap field is
	// rejected without the extra allocations.
	if len(body.SigningPackage) > MaxSigningPackageBytes {
		return fmt.Errorf(
			"signed signing package: signingPackage length [%d] exceeds cap [%d]",
			len(body.SigningPackage),
			MaxSigningPackageBytes,
		)
	}
	signingPackageFieldsFromBody(p, &body)
	p.CoordinatorSignature = append([]byte(nil), envelope.CoordinatorSignature...)
	p.bodyCache = append([]byte(nil), envelope.Body...)
	p.wireEnvelope = append([]byte(nil), data...)
	// Prime the signable-bytes cache from the body just received, discarding any
	// cache a prior SignableBytes call left on a reused value. Priming here -
	// rather than lazily in SignableBytes - keeps concurrent signature
	// verification of a parsed package race-free: verifiers read a ready cache
	// instead of racing on lazy initialization (authentication must verify
	// against the received bytes, never stale ones).
	p.signaturePayloadCache = nil
	if _, err := p.SignableBytes(); err != nil {
		return err
	}
	return p.Validate()
}

// AttemptContextHashArray returns the attempt context hash as a fixed
// 32-byte array. Validate (or Unmarshal) must have confirmed the length
// first; it copies at most 32 bytes and zero-pads a short slice.
func (p *SigningPackage) AttemptContextHashArray() [attempt.MessageDigestLength]byte {
	var out [attempt.MessageDigestLength]byte
	copy(out[:], p.AttemptContextHash)
	return out
}

// CoordinatorID returns the elected coordinator's member index.
func (p *SigningPackage) CoordinatorID() group.MemberIndex {
	return group.MemberIndex(p.CoordinatorIDValue)
}

// Validate runs the structural checks Unmarshal applies after a decode.
// Exposed so callers that construct packages in memory (e.g. the
// coordinator) can validate without a marshal/unmarshal round-trip. It
// does not verify the coordinator signature - that is the member-side
// authentication step (a later Phase 7.2b increment), which checks the
// signature against the attempt's elected coordinator's operator key.
func (p *SigningPackage) Validate() error {
	if p == nil {
		return errors.New("signed signing package: nil")
	}
	if len(p.AttemptContextHash) != attempt.MessageDigestLength {
		return fmt.Errorf(
			"signed signing package: attemptContextHash length [%d], expected [%d]",
			len(p.AttemptContextHash),
			attempt.MessageDigestLength,
		)
	}
	if p.CoordinatorIDValue == 0 {
		return errors.New("signed signing package: coordinatorID is zero")
	}
	// coordinator_id is a wire uint32 but a member index is a uint8
	// (group.MemberIndex); reject an out-of-range value here so CoordinatorID()
	// never silently truncates and the member-side elected-coordinator check
	// compares a faithful value.
	if p.CoordinatorIDValue > group.MaxMemberIndex {
		return fmt.Errorf(
			"signed signing package: coordinatorID [%d] exceeds max member index [%d]",
			p.CoordinatorIDValue,
			group.MaxMemberIndex,
		)
	}
	if len(p.SigningPackageBytes) == 0 {
		return errors.New("signed signing package: empty signing package")
	}
	if len(p.SigningPackageBytes) > MaxSigningPackageBytes {
		return fmt.Errorf(
			"signed signing package: signingPackage length [%d] exceeds cap [%d]",
			len(p.SigningPackageBytes),
			MaxSigningPackageBytes,
		)
	}
	if n := len(p.TaprootMerkleRoot); n != 0 && n != TaprootMerkleRootLength {
		return fmt.Errorf(
			"signed signing package: taprootMerkleRoot length [%d], expected 0 (key-path) or %d",
			n,
			TaprootMerkleRootLength,
		)
	}
	if len(p.CoordinatorSignature) > MaxCoordinatorSignatureBytes {
		return fmt.Errorf(
			"signed signing package: coordinatorSignature length [%d] exceeds cap [%d]",
			len(p.CoordinatorSignature),
			MaxCoordinatorSignatureBytes,
		)
	}
	// signer_ids (when present) names the chosen signing subset: each a real member
	// index (so SignerIDs() never truncates) and STRICTLY ASCENDING (hence
	// distinct, and bounded to <= the member-index space). Empty is valid -- the
	// full-included flow carries no subset. This is a structural/liveness check:
	// the engine verifies shares against the SigningPackageBytes (the cryptographic
	// source of truth), so a lying list only fails an attempt, never produces a
	// wrong signature or false blame.
	for i, id := range p.SignerIDsValue {
		if id == 0 || id > group.MaxMemberIndex {
			return fmt.Errorf(
				"signed signing package: signerID [%d] is not a valid member index",
				id,
			)
		}
		if i > 0 && id <= p.SignerIDsValue[i-1] {
			return fmt.Errorf(
				"signed signing package: signerIDs must be strictly ascending (got [%d] after [%d])",
				id, p.SignerIDsValue[i-1],
			)
		}
	}
	return nil
}

// SignerIDs returns the chosen signing subset's member indices in their validated
// form. Callers MUST Validate the package first (AuthenticateSigningPackage does):
// Validate guarantees each value is a real member index, so the uint32 ->
// group.MemberIndex conversion here cannot truncate. Returns an empty slice when
// the package carries no subset (the full-included flow).
func (p *SigningPackage) SignerIDs() []group.MemberIndex {
	out := make([]group.MemberIndex, 0, len(p.SignerIDsValue))
	for _, id := range p.SignerIDsValue {
		out = append(out, group.MemberIndex(id))
	}
	return out
}

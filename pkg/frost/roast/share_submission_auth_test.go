package roast

import (
	"bytes"
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestSignShareSubmission_RoundTripAuthenticates(t *testing.T) {
	const submitter = group.MemberIndex(3)
	pkgHash := testSigningPackageHash()
	sub := &ShareSubmission{
		AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
		SubmitterIDValue:   uint32(submitter),
		CoordinatorIDValue: testShareCoordinatorID,
		SigningPackageHash: pkgHash,
		SignatureShare:     []byte("frost-round2-share"),
	}
	if err := SignShareSubmission(&fakeSigner{id: submitter}, sub); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(sub.SubmitterSignature) == 0 {
		t.Fatal("SignShareSubmission must set a submitter signature")
	}

	// The coordinator receives the submission off the wire and authenticates it.
	wire, err := sub.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var received ShareSubmission
	if err := received.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := AuthenticateShareSubmission(
		fakeVerifier{},
		&received,
		group.MemberIndex(testShareCoordinatorID),
		pinnedContextHash[:],
		pkgHash,
	); err != nil {
		t.Fatalf("authenticate a genuine submission: %v", err)
	}
}

func TestAuthenticateShareSubmission_Rejections(t *testing.T) {
	const submitter = group.MemberIndex(3)
	elected := group.MemberIndex(testShareCoordinatorID)
	pkgHash := testSigningPackageHash()
	signed := func() *ShareSubmission {
		sub := &ShareSubmission{
			AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
			SubmitterIDValue:   uint32(submitter),
			CoordinatorIDValue: testShareCoordinatorID,
			SigningPackageHash: pkgHash,
			SignatureShare:     []byte("share"),
		}
		if err := SignShareSubmission(&fakeSigner{id: submitter}, sub); err != nil {
			t.Fatalf("sign: %v", err)
		}
		return sub
	}
	otherAttempt := bytes.Repeat([]byte{0x09}, attempt.MessageDigestLength)

	t.Run("missing signature is rejected", func(t *testing.T) {
		sub := signed()
		sub.SubmitterSignature = nil
		err := AuthenticateShareSubmission(fakeVerifier{}, sub, elected, pinnedContextHash[:], pkgHash)
		if !errors.Is(err, ErrSignatureMissing) {
			t.Fatalf("want ErrSignatureMissing, got %v", err)
		}
	})

	t.Run("non-elected coordinator is rejected", func(t *testing.T) {
		err := AuthenticateShareSubmission(fakeVerifier{}, signed(), elected+1, pinnedContextHash[:], pkgHash)
		if !errors.Is(err, ErrShareSubmissionWrongCoordinator) {
			t.Fatalf("want ErrShareSubmissionWrongCoordinator, got %v", err)
		}
	})

	t.Run("wrong attempt is rejected", func(t *testing.T) {
		err := AuthenticateShareSubmission(fakeVerifier{}, signed(), elected, otherAttempt, pkgHash)
		if !errors.Is(err, ErrShareSubmissionWrongAttempt) {
			t.Fatalf("want ErrShareSubmissionWrongAttempt, got %v", err)
		}
	})

	t.Run("wrong package is rejected", func(t *testing.T) {
		otherPkg := bytes.Repeat([]byte{0x11}, SigningPackageHashLength)
		err := AuthenticateShareSubmission(fakeVerifier{}, signed(), elected, pinnedContextHash[:], otherPkg)
		if !errors.Is(err, ErrShareSubmissionWrongPackage) {
			t.Fatalf("want ErrShareSubmissionWrongPackage, got %v", err)
		}
	})

	t.Run("tampered signature fails verification", func(t *testing.T) {
		sub := signed()
		sub.SubmitterSignature[0] ^= 0xff
		err := AuthenticateShareSubmission(fakeVerifier{}, sub, elected, pinnedContextHash[:], pkgHash)
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("want ErrSignatureInvalid, got %v", err)
		}
	})

	t.Run("submission signed by a non-submitter is rejected", func(t *testing.T) {
		// A different operator signs a body carrying submitter_id=3; the
		// signature does not verify under member 3's key.
		sub := &ShareSubmission{
			AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
			SubmitterIDValue:   uint32(submitter),
			CoordinatorIDValue: testShareCoordinatorID,
			SigningPackageHash: pkgHash,
			SignatureShare:     []byte("share"),
		}
		if err := SignShareSubmission(&fakeSigner{id: submitter + 7}, sub); err != nil {
			t.Fatalf("sign: %v", err)
		}
		err := AuthenticateShareSubmission(fakeVerifier{}, sub, elected, pinnedContextHash[:], pkgHash)
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("want ErrSignatureInvalid, got %v", err)
		}
	})

	t.Run("structurally invalid submission is rejected before verification", func(t *testing.T) {
		// submitter_id 259 truncates to member 3 (uint32 -> uint8); signed by
		// member 3 it would otherwise verify AS member 3 despite the wire id. The
		// structural pre-check (submitter_id > MaxMemberIndex) rejects it before
		// any signature is trusted.
		sub := &ShareSubmission{
			AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
			SubmitterIDValue:   259,
			CoordinatorIDValue: testShareCoordinatorID,
			SigningPackageHash: pkgHash,
			SignatureShare:     []byte("share"),
		}
		if err := SignShareSubmission(&fakeSigner{id: 3}, sub); err != nil {
			t.Fatalf("sign: %v", err)
		}
		if err := AuthenticateShareSubmission(fakeVerifier{}, sub, elected, pinnedContextHash[:], pkgHash); err == nil {
			t.Fatal("an out-of-range submitter_id must be rejected before verification")
		}
	})
}

func TestShareSubmissionBindsToSigningPackageBody(t *testing.T) {
	// End-to-end: a share bound to a real signing package's BodyHash
	// authenticates against that hash, and a share checked against a different
	// package's hash is rejected.
	const (
		submitter   = group.MemberIndex(3)
		coordinator = group.MemberIndex(7)
	)
	pkg := signedTestSigningPackage(t, coordinator, nil)
	pkgHash, err := pkg.BodyHash()
	if err != nil {
		t.Fatalf("body hash: %v", err)
	}

	sub := &ShareSubmission{
		AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
		SubmitterIDValue:   uint32(submitter),
		CoordinatorIDValue: uint32(coordinator),
		SigningPackageHash: pkgHash[:],
		SignatureShare:     []byte("share"),
	}
	if err := SignShareSubmission(&fakeSigner{id: submitter}, sub); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := AuthenticateShareSubmission(
		fakeVerifier{}, sub, coordinator, pinnedContextHash[:], pkgHash[:],
	); err != nil {
		t.Fatalf("authenticate against the bound package: %v", err)
	}

	// A different package -> different envelope hash -> rejected.
	otherPkg := signedTestSigningPackage(t, coordinator, bytes.Repeat([]byte{0xcd}, TaprootMerkleRootLength))
	otherHash, err := otherPkg.BodyHash()
	if err != nil {
		t.Fatalf("other body hash: %v", err)
	}
	if bytes.Equal(pkgHash[:], otherHash[:]) {
		t.Fatal("sanity: distinct packages must have distinct envelope hashes")
	}
	if err := AuthenticateShareSubmission(
		fakeVerifier{}, sub, coordinator, pinnedContextHash[:], otherHash[:],
	); !errors.Is(err, ErrShareSubmissionWrongPackage) {
		t.Fatalf("want ErrShareSubmissionWrongPackage, got %v", err)
	}
}

func TestSigningPackageBodyHash_StableAcrossWireAndReEncoding(t *testing.T) {
	pkg := signedTestSigningPackage(t, 3, nil)
	wire, err := pkg.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	producerHash, err := pkg.BodyHash()
	if err != nil {
		t.Fatalf("producer hash: %v", err)
	}

	var received SigningPackage
	if err := received.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	receivedHash, err := received.BodyHash()
	if err != nil {
		t.Fatalf("received hash: %v", err)
	}
	if producerHash != receivedHash {
		t.Fatal("body hash must match for producer and receiver over the same bytes")
	}

	// It must also be stable across an unsigned ENVELOPE re-encoding (reversed
	// field order) of the same (body, signature) - the property that stops a MitM
	// re-wrap from looking like a different package or fragmenting share bindings.
	body, _ := pkg.bodyBytes()
	var reEncoded []byte
	reEncoded = append(reEncoded, 0x12, byte(len(pkg.CoordinatorSignature)))
	reEncoded = append(reEncoded, pkg.CoordinatorSignature...)
	reEncoded = append(reEncoded, 0x0a, byte(len(body)))
	reEncoded = append(reEncoded, body...)

	var reDecoded SigningPackage
	if err := reDecoded.Unmarshal(reEncoded); err != nil {
		t.Fatalf("unmarshal re-encoded: %v", err)
	}
	reHash, err := reDecoded.BodyHash()
	if err != nil {
		t.Fatalf("re-encoded hash: %v", err)
	}
	if reHash != producerHash {
		t.Fatal("body hash must be stable across an unsigned envelope re-encoding")
	}
	if reWire, _ := reDecoded.Marshal(); bytes.Equal(reWire, wire) {
		t.Fatal("sanity: the re-encoded envelope must differ from the canonical envelope")
	}
}

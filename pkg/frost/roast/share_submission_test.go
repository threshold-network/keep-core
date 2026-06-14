package roast

import (
	"bytes"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// These pin the signed-body envelope contract for a member's Round2 share
// submission: the member signs exactly the bytes that travel, those bytes
// survive re-broadcast verbatim, the signed preimage is domain-separated from
// every other ROAST signed body, and parsing never depends on a serializer's
// canonical form. Member-side authentication arrives with a later increment.

func testSigningPackageHash() []byte {
	return bytes.Repeat([]byte{0xab}, SigningPackageHashLength)
}

func signedTestShareSubmission(
	t *testing.T,
	submitter group.MemberIndex,
	pkgHash []byte,
) *ShareSubmission {
	t.Helper()
	p := &ShareSubmission{
		AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
		SubmitterIDValue:   uint32(submitter),
		SigningPackageHash: pkgHash,
		SignatureShare:     []byte("frost-round2-signature-share"),
	}
	payload, err := p.SignableBytes()
	if err != nil {
		t.Fatalf("signable bytes: %v", err)
	}
	sig, err := (&fakeSigner{id: submitter}).Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	p.SubmitterSignature = sig
	return p
}

func TestShareSubmissionWire_ReceivedBytesPreservedVerbatim(t *testing.T) {
	original := signedTestShareSubmission(t, 3, testSigningPackageHash())
	wire, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &ShareSubmission{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	rebroadcast, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(rebroadcast, wire) {
		t.Fatal("re-marshal of a received share submission must return the received bytes verbatim")
	}

	producerBody, _ := original.SignableBytes()
	receiverBody, _ := decoded.SignableBytes()
	if !bytes.Equal(producerBody, receiverBody) {
		t.Fatal("receiver must verify over exactly the bytes the submitter signed")
	}
	if decoded.SubmitterIDValue != original.SubmitterIDValue ||
		!bytes.Equal(decoded.AttemptContextHash, original.AttemptContextHash) ||
		!bytes.Equal(decoded.SigningPackageHash, original.SigningPackageHash) ||
		!bytes.Equal(decoded.SignatureShare, original.SignatureShare) ||
		!bytes.Equal(decoded.SubmitterSignature, original.SubmitterSignature) {
		t.Fatal("decoded fields must match the original")
	}
}

func TestShareSubmissionWire_NonCanonicalEnvelopeEncodingSurvives(t *testing.T) {
	original := signedTestShareSubmission(t, 3, testSigningPackageHash())
	body, err := original.bodyBytes()
	if err != nil {
		t.Fatalf("body bytes: %v", err)
	}

	// Handcraft an envelope with fields in REVERSE tag order
	// (submitter_signature before body) - wire-legal but non-canonical. Field 1
	// (body) tag 0x0a, field 2 (submitter_signature) tag 0x12; both
	// length-delimited.
	var crafted []byte
	crafted = append(crafted, 0x12, byte(len(original.SubmitterSignature)))
	crafted = append(crafted, original.SubmitterSignature...)
	crafted = append(crafted, 0x0a, byte(len(body)))
	crafted = append(crafted, body...)

	var check pb.SignedShareSubmission
	if err := proto.Unmarshal(crafted, &check); err != nil {
		t.Fatalf("crafted envelope must be wire-legal: %v", err)
	}

	decoded := &ShareSubmission{}
	if err := decoded.Unmarshal(crafted); err != nil {
		t.Fatalf("unmarshal crafted: %v", err)
	}
	if gotBody, _ := decoded.bodyBytes(); !bytes.Equal(gotBody, body) {
		t.Fatal("the received body must be preserved verbatim")
	}
	remarshaled, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(remarshaled, crafted) {
		t.Fatal("re-marshal must preserve even a non-canonical received encoding verbatim")
	}
}

func TestShareSubmissionWire_DomainSeparatedAndUndecodable(t *testing.T) {
	p := signedTestShareSubmission(t, 3, testSigningPackageHash())
	signable, err := p.SignableBytes()
	if err != nil {
		t.Fatalf("signable: %v", err)
	}
	body, _ := p.bodyBytes()

	// SignableBytes = share-submission domain tag || bare body.
	if !bytes.HasPrefix(signable, shareSubmissionSignatureDomain) {
		t.Fatal("signed bytes must carry the share-submission domain tag")
	}
	if !bytes.Equal(signable[len(shareSubmissionSignatureDomain):], body) {
		t.Fatal("signed bytes must be the domain tag followed by the bare body")
	}
	// The signed bytes begin with an illegal protobuf tag and are undecodable.
	if signable[0] != 0x00 {
		t.Fatal("signed bytes must begin with an illegal protobuf tag (0x00)")
	}
	if err := proto.Unmarshal(signable, &pb.ShareSubmissionBody{}); err == nil {
		t.Fatal("domain-tagged signed bytes must not decode as a protobuf message")
	}
	// The bare wire body carries no tag and is a valid protobuf body.
	if bytes.HasPrefix(body, shareSubmissionSignatureDomain) {
		t.Fatal("the wire body must not carry the domain tag")
	}
	if err := proto.Unmarshal(body, &pb.ShareSubmissionBody{}); err != nil {
		t.Fatalf("the bare wire body must be a valid protobuf body: %v", err)
	}
}

func TestShareSubmissionDomain_DistinctFromOtherSignedBodies(t *testing.T) {
	// The share-submission tag must be distinct and prefix-free from every other
	// signed-body domain in the package, so a share signature can never be
	// confused with a signing-package, snapshot, or transition signature.
	others := map[string][]byte{
		"signing-package":   signingPackageSignatureDomain,
		"evidence-snapshot": localEvidenceSnapshotSignatureDomain,
		"transition":        transitionMessageSignatureDomain,
	}
	share := shareSubmissionSignatureDomain
	if share[0] != 0x00 {
		t.Fatal("share-submission domain must begin with an illegal protobuf tag (0x00)")
	}
	for name, other := range others {
		if bytes.Equal(share, other) {
			t.Fatalf("share-submission domain must differ from the %s domain", name)
		}
		if bytes.HasPrefix(share, other) || bytes.HasPrefix(other, share) {
			t.Fatalf("share-submission domain must be prefix-free vs the %s domain", name)
		}
	}
}

func TestShareSubmission_ValidateRejectsMalformed(t *testing.T) {
	valid := func() *ShareSubmission {
		return &ShareSubmission{
			AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
			SubmitterIDValue:   3,
			SigningPackageHash: testSigningPackageHash(),
			SignatureShare:     []byte("share"),
		}
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("a well-formed submission must validate: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ShareSubmission)
	}{
		{"short attempt hash", func(p *ShareSubmission) { p.AttemptContextHash = []byte{1, 2, 3} }},
		{"zero submitter", func(p *ShareSubmission) { p.SubmitterIDValue = 0 }},
		{"submitter out of member-index range", func(p *ShareSubmission) {
			p.SubmitterIDValue = group.MaxMemberIndex + 1
		}},
		{"short signing package hash", func(p *ShareSubmission) { p.SigningPackageHash = []byte{0x01} }},
		{"empty signature share", func(p *ShareSubmission) { p.SignatureShare = nil }},
		{"oversize signature share", func(p *ShareSubmission) {
			p.SignatureShare = make([]byte, MaxSignatureShareBytes+1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.mutate(p)
			if err := p.Validate(); err == nil {
				t.Fatal("expected Validate to reject the malformed submission")
			}
		})
	}
}

func TestShareSubmission_MarshalRequiresSignature(t *testing.T) {
	p := &ShareSubmission{
		AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
		SubmitterIDValue:   3,
		SigningPackageHash: testSigningPackageHash(),
		SignatureShare:     []byte("share"),
	}
	if _, err := p.Marshal(); err == nil {
		t.Fatal("Marshal must refuse an unsigned share submission")
	}
}

func TestShareSubmissionWire_UnmarshalRejectsOversizeBeforeCopy(t *testing.T) {
	// A peer-supplied envelope whose signature_share exceeds the cap is rejected
	// on receive, so the cap protects memory rather than only failing after the
	// field is materialized and copied.
	oversized := &ShareSubmission{
		AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
		SubmitterIDValue:   3,
		SigningPackageHash: testSigningPackageHash(),
		SignatureShare:     make([]byte, MaxSignatureShareBytes+1),
	}
	payload, err := oversized.SignableBytes()
	if err != nil {
		t.Fatalf("signable: %v", err)
	}
	sig, err := (&fakeSigner{id: 3}).Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	oversized.SubmitterSignature = sig
	wire, err := oversized.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ShareSubmission
	if err := decoded.Unmarshal(wire); err == nil {
		t.Fatal("Unmarshal must reject an over-cap signature share")
	}
}

func TestShareSubmission_ConcurrentSignableBytesAfterUnmarshalIsRaceFree(t *testing.T) {
	// Regression guard (run under -race): a parsed submission must carry a primed
	// signable-bytes cache so concurrent signature verification reads a ready
	// cache instead of racing on lazy initialization.
	wire, err := signedTestShareSubmission(t, 3, testSigningPackageHash()).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ShareSubmission
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := decoded.SignableBytes(); err != nil {
				t.Errorf("signable: %v", err)
			}
		}()
	}
	wg.Wait()
}

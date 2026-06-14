package roast

import (
	"bytes"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// These pin the signed-body envelope contract for the coordinator's signing
// package: the coordinator signs exactly the bytes that travel, those bytes
// survive re-broadcast verbatim, and parsing never depends on a serializer's
// canonical form. Coordinator-signature verification and member-side
// authentication arrive with a later Phase 7.2b increment.

func signedTestSigningPackage(
	t *testing.T,
	coordinator group.MemberIndex,
	root []byte,
) *SigningPackage {
	t.Helper()
	pkg := &SigningPackage{
		AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:  uint32(coordinator),
		SigningPackageBytes: []byte("frost-signing-package-bytes"),
		TaprootMerkleRoot:   root,
	}
	payload, err := pkg.SignableBytes()
	if err != nil {
		t.Fatalf("signable bytes: %v", err)
	}
	sig, err := (&fakeSigner{id: coordinator}).Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	pkg.CoordinatorSignature = sig
	return pkg
}

func TestSigningPackageWire_ReceivedBytesPreservedVerbatim(t *testing.T) {
	original := signedTestSigningPackage(t, 3, nil)
	wire, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &SigningPackage{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	rebroadcast, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(rebroadcast, wire) {
		t.Fatal("re-marshal of a received signing package must return the received bytes verbatim")
	}

	producerBody, _ := original.SignableBytes()
	receiverBody, _ := decoded.SignableBytes()
	if !bytes.Equal(producerBody, receiverBody) {
		t.Fatal("receiver must be able to verify over exactly the bytes the coordinator signed")
	}
	if decoded.CoordinatorIDValue != original.CoordinatorIDValue ||
		!bytes.Equal(decoded.AttemptContextHash, original.AttemptContextHash) ||
		!bytes.Equal(decoded.SigningPackageBytes, original.SigningPackageBytes) ||
		!bytes.Equal(decoded.CoordinatorSignature, original.CoordinatorSignature) {
		t.Fatal("decoded fields must match the original")
	}
}

func TestSigningPackageWire_NonCanonicalEnvelopeEncodingSurvives(t *testing.T) {
	original := signedTestSigningPackage(t, 3, nil)
	body, err := original.bodyBytes()
	if err != nil {
		t.Fatalf("body bytes: %v", err)
	}

	// Handcraft an envelope with fields in REVERSE tag order
	// (coordinator_signature before body) - wire-legal but non-canonical, no
	// Go marshaler would emit it. Field 1 (body) tag 0x0a, field 2
	// (coordinator_signature) tag 0x12; both length-delimited.
	var crafted []byte
	crafted = append(crafted, 0x12, byte(len(original.CoordinatorSignature)))
	crafted = append(crafted, original.CoordinatorSignature...)
	crafted = append(crafted, 0x0a, byte(len(body)))
	crafted = append(crafted, body...)

	var check pb.SignedSigningPackage
	if err := proto.Unmarshal(crafted, &check); err != nil {
		t.Fatalf("crafted envelope must be wire-legal: %v", err)
	}

	decoded := &SigningPackage{}
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

func TestSigningPackageWire_SignedBytesDomainSeparatedFromTransitionMessage(t *testing.T) {
	pkg := signedTestSigningPackage(t, 3, nil)
	signable, err := pkg.SignableBytes()
	if err != nil {
		t.Fatalf("signable: %v", err)
	}
	body, _ := pkg.bodyBytes()

	// The signed bytes are the domain tag followed by the body.
	if !bytes.HasPrefix(signable, signingPackageSignatureDomain) {
		t.Fatal("signed bytes must carry the signing-package domain tag")
	}
	if !bytes.Equal(signable[len(signingPackageSignatureDomain):], body) {
		t.Fatal("signed bytes must be the domain tag followed by the body")
	}

	// The bare body IS wire-compatible with a TransitionMessageBody - the
	// collision the domain tag defends against: it presents the same 32-byte
	// attempt_context_hash and coordinator_id a transition body would.
	var asTransition pb.TransitionMessageBody
	if err := proto.Unmarshal(body, &asTransition); err != nil {
		t.Fatalf("the bare body is expected to parse as a TransitionMessageBody: %v", err)
	}
	if len(asTransition.AttemptContextHash) != attempt.MessageDigestLength ||
		asTransition.CoordinatorId != pkg.CoordinatorIDValue {
		t.Fatal("sanity: the bare body must collide with TransitionMessageBody")
	}

	// But the domain-tagged SIGNED bytes begin with an illegal protobuf tag
	// (field 0), so they are not decodable as ANY protobuf message - a
	// signing-package signature therefore cannot be replayed onto a
	// transition-message (or other coordinator-signed) envelope, whose decoder
	// proto.Unmarshals and rejects the body. A valid-protobuf tag would not give
	// this (see TestSigningPackageWire_SignedBytesResistEmbeddedTransitionBody).
	if signable[0] != 0x00 {
		t.Fatal("signed bytes must begin with an illegal protobuf tag (0x00)")
	}
	if err := proto.Unmarshal(signable, &pb.TransitionMessageBody{}); err == nil {
		t.Fatal("domain-tagged signed bytes must not decode as a protobuf message")
	}
}

func TestSigningPackageWire_SignedBytesResistEmbeddedTransitionBody(t *testing.T) {
	// A malicious coordinator controls signing_package, so it can embed a fully
	// valid serialized TransitionMessageBody there. Under a domain tag that is
	// itself valid protobuf wire data, a parser skips the tag as an unknown
	// field and could resume into this embedded transition body, re-enabling
	// cross-protocol signature confusion. The leading illegal tag must make the
	// whole signed payload undecodable regardless of the embedded content.
	embeddedTransition, err := proto.Marshal(&pb.TransitionMessageBody{
		AttemptContextHash: bytes.Repeat([]byte{0x07}, attempt.MessageDigestLength),
		CoordinatorId:      3,
	})
	if err != nil {
		t.Fatalf("marshal embedded transition: %v", err)
	}
	// Sanity: the embedded payload really is a valid transition body.
	var sanity pb.TransitionMessageBody
	if err := proto.Unmarshal(embeddedTransition, &sanity); err != nil ||
		len(sanity.AttemptContextHash) != attempt.MessageDigestLength {
		t.Fatal("sanity: embedded payload must be a valid transition body")
	}

	pkg := &SigningPackage{
		AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:  3,
		SigningPackageBytes: embeddedTransition,
	}
	signable, err := pkg.SignableBytes()
	if err != nil {
		t.Fatalf("signable: %v", err)
	}
	if err := proto.Unmarshal(signable, &pb.TransitionMessageBody{}); err == nil {
		t.Fatal("signed bytes embedding a transition body must still be undecodable as protobuf")
	}
}

func TestSigningPackageWire_UnmarshalResetsSignableCache(t *testing.T) {
	// A SigningPackage value reused across a SignableBytes call and then an
	// Unmarshal must authenticate the newly decoded package against the bytes it
	// just received, never the stale cached payload.
	reused := &SigningPackage{
		AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:  3,
		SigningPackageBytes: []byte("stale-package"),
	}
	if _, err := reused.SignableBytes(); err != nil { // prime the cache
		t.Fatalf("prime cache: %v", err)
	}

	// Decode a different, genuine package into the SAME value.
	genuine := signedTestSigningPackage(t, 5, bytes.Repeat([]byte{0xab}, TaprootMerkleRootLength))
	wire, err := genuine.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := reused.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal into reused value: %v", err)
	}

	got, _ := reused.SignableBytes()
	want, _ := genuine.SignableBytes()
	if !bytes.Equal(got, want) {
		t.Fatal("Unmarshal must reset the signable-bytes cache to the received body")
	}
	if err := AuthenticateSigningPackage(fakeVerifier{}, reused, 5, pinnedContextHash[:]); err != nil {
		t.Fatalf("authenticate reused-decoded package: %v", err)
	}
}

func TestSigningPackageWire_RootRoundTrips(t *testing.T) {
	for _, tc := range []struct {
		name string
		root []byte
	}{
		{"key-path (empty root)", nil},
		{"script-path (32-byte root)", bytes.Repeat([]byte{0xab}, TaprootMerkleRootLength)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := signedTestSigningPackage(t, 5, tc.root).Marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			decoded := &SigningPackage{}
			if err := decoded.Unmarshal(wire); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !bytes.Equal(decoded.TaprootMerkleRoot, tc.root) {
				t.Fatalf("root mismatch: got %x want %x", decoded.TaprootMerkleRoot, tc.root)
			}
		})
	}
}

func TestSigningPackage_ValidateRejectsMalformed(t *testing.T) {
	valid := func() *SigningPackage {
		return &SigningPackage{
			AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
			CoordinatorIDValue:  3,
			SigningPackageBytes: []byte("pkg"),
		}
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("a well-formed package must validate: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*SigningPackage)
	}{
		{"short attempt hash", func(p *SigningPackage) { p.AttemptContextHash = []byte{1, 2, 3} }},
		{"zero coordinator", func(p *SigningPackage) { p.CoordinatorIDValue = 0 }},
		{"coordinator out of member-index range", func(p *SigningPackage) {
			p.CoordinatorIDValue = group.MaxMemberIndex + 1
		}},
		{"empty signing package", func(p *SigningPackage) { p.SigningPackageBytes = nil }},
		{"bad root length", func(p *SigningPackage) { p.TaprootMerkleRoot = []byte{0x01} }},
		{"oversize signing package", func(p *SigningPackage) {
			p.SigningPackageBytes = make([]byte, MaxSigningPackageBytes+1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.mutate(p)
			if err := p.Validate(); err == nil {
				t.Fatal("expected Validate to reject the malformed package")
			}
		})
	}
}

func TestSigningPackageWire_UnmarshalRejectsOversizeBeforeCopy(t *testing.T) {
	// A peer-supplied envelope whose signing_package exceeds the cap is
	// rejected on receive, so the cap protects memory rather than only
	// failing after the field is materialized and copied.
	oversized := &SigningPackage{
		AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:  3,
		SigningPackageBytes: make([]byte, MaxSigningPackageBytes+1),
	}
	payload, err := oversized.SignableBytes()
	if err != nil {
		t.Fatalf("signable: %v", err)
	}
	sig, err := (&fakeSigner{id: 3}).Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	oversized.CoordinatorSignature = sig
	wire, err := oversized.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SigningPackage
	if err := decoded.Unmarshal(wire); err == nil {
		t.Fatal("Unmarshal must reject an over-cap signing package")
	}
}

func TestSigningPackage_MarshalRequiresSignature(t *testing.T) {
	pkg := &SigningPackage{
		AttemptContextHash:  append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue:  3,
		SigningPackageBytes: []byte("pkg"),
	}
	if _, err := pkg.Marshal(); err == nil {
		t.Fatal("Marshal must refuse an unsigned signing package")
	}
}

func TestSigningPackage_ConcurrentSignableBytesAfterUnmarshalIsRaceFree(t *testing.T) {
	// Regression guard (run under -race): a parsed signing package must carry a
	// primed signable-bytes cache so concurrent signature verification reads a
	// ready cache instead of racing on lazy initialization. Without priming in
	// Unmarshal, the concurrent first SignableBytes calls below race on the
	// cache write.
	wire, err := signedTestSigningPackage(t, 3, nil).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SigningPackage
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

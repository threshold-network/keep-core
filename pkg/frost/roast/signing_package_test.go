package roast

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

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
	body, _ := original.SignableBytes()

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
	if gotBody, _ := decoded.SignableBytes(); !bytes.Equal(gotBody, body) {
		t.Fatal("SignableBytes must return the embedded body bytes verbatim")
	}
	remarshaled, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(remarshaled, crafted) {
		t.Fatal("re-marshal must preserve even a non-canonical received encoding verbatim")
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

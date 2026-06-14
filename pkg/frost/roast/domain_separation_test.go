package roast

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
)

// These pin the cross-protocol signature-confusion defense for the operator-key
// signed bodies: the signed bytes are domain-tagged and undecodable as
// protobuf, the bytes that travel on the wire stay the bare body, and the two
// body types use distinct tags so a signature over one can never verify as a
// signature over the other.

func TestSnapshotSignableBytes_DomainSeparated(t *testing.T) {
	snap := signedTestSnapshot(t, 7)
	signable, err := snap.SignableBytes()
	if err != nil {
		t.Fatalf("signable: %v", err)
	}
	body, err := snap.bodyBytes()
	if err != nil {
		t.Fatalf("body: %v", err)
	}

	// SignableBytes = snapshot domain tag || bare body.
	if !bytes.HasPrefix(signable, localEvidenceSnapshotSignatureDomain) {
		t.Fatal("snapshot signed bytes must carry the snapshot domain tag")
	}
	if !bytes.Equal(signable[len(localEvidenceSnapshotSignatureDomain):], body) {
		t.Fatal("snapshot signed bytes must be the domain tag followed by the bare body")
	}

	// The signed bytes begin with an illegal protobuf tag (field 0) and so are
	// undecodable as any protobuf message - a snapshot signature can never be
	// accepted on a transition (or other) envelope whose decoder parses the
	// forged body.
	if signable[0] != 0x00 {
		t.Fatal("snapshot signed bytes must begin with an illegal protobuf tag (0x00)")
	}
	if err := proto.Unmarshal(signable, &pb.LocalEvidenceSnapshotBody{}); err == nil {
		t.Fatal("snapshot signed bytes must not decode as a protobuf message")
	}

	// The bare wire body, by contrast, carries no tag and IS a valid protobuf
	// body - the domain tag never travels on the wire.
	if bytes.HasPrefix(body, localEvidenceSnapshotSignatureDomain) {
		t.Fatal("the wire body must not carry the domain tag")
	}
	if err := proto.Unmarshal(body, &pb.LocalEvidenceSnapshotBody{}); err != nil {
		t.Fatalf("the bare wire body must be a valid protobuf body: %v", err)
	}
}

func TestTransitionSignableBytes_DomainSeparated(t *testing.T) {
	msg := buildValidTransitionMessage()
	signable, err := msg.SignableBytes()
	if err != nil {
		t.Fatalf("signable: %v", err)
	}
	body, err := msg.bodyBytes()
	if err != nil {
		t.Fatalf("body: %v", err)
	}

	if !bytes.HasPrefix(signable, transitionMessageSignatureDomain) {
		t.Fatal("transition signed bytes must carry the transition domain tag")
	}
	if !bytes.Equal(signable[len(transitionMessageSignatureDomain):], body) {
		t.Fatal("transition signed bytes must be the domain tag followed by the bare body")
	}
	if signable[0] != 0x00 {
		t.Fatal("transition signed bytes must begin with an illegal protobuf tag (0x00)")
	}
	if err := proto.Unmarshal(signable, &pb.TransitionMessageBody{}); err == nil {
		t.Fatal("transition signed bytes must not decode as a protobuf message")
	}
	if bytes.HasPrefix(body, transitionMessageSignatureDomain) {
		t.Fatal("the wire body must not carry the domain tag")
	}
	if err := proto.Unmarshal(body, &pb.TransitionMessageBody{}); err != nil {
		t.Fatalf("the bare wire body must be a valid protobuf body: %v", err)
	}
}

func TestSignedBodyDomains_AreDistinctAndPrefixFree(t *testing.T) {
	// Distinct, prefix-free tags make the signed-byte spaces of the two body
	// types disjoint: domain_a || body_a == domain_b || body_b is impossible
	// unless one tag is a prefix of the other, so a signature over one body can
	// never verify as a signature over the other.
	a := localEvidenceSnapshotSignatureDomain
	b := transitionMessageSignatureDomain
	if bytes.Equal(a, b) {
		t.Fatal("each signed body type must use a distinct domain tag")
	}
	if bytes.HasPrefix(a, b) || bytes.HasPrefix(b, a) {
		t.Fatal("no domain tag may be a prefix of another")
	}
	for _, tag := range [][]byte{a, b} {
		if len(tag) == 0 || tag[0] != 0x00 {
			t.Fatalf("domain tag %q must begin with an illegal protobuf tag (0x00)", tag)
		}
	}
}

func TestSnapshotUnmarshal_ResetsSignableCache(t *testing.T) {
	// A snapshot value reused across a SignableBytes call and then an Unmarshal
	// must authenticate the newly decoded snapshot against the bytes it just
	// received, never the stale cached payload.
	reused := signedTestSnapshot(t, 7)
	if _, err := reused.SignableBytes(); err != nil { // prime the cache
		t.Fatalf("prime cache: %v", err)
	}

	genuine := signedTestSnapshot(t, 9)
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
		t.Fatal("Unmarshal must reset the snapshot signable-bytes cache")
	}
	if err := verifySnapshotSignature(fakeVerifier{}, reused); err != nil {
		t.Fatalf("authenticate reused-decoded snapshot: %v", err)
	}
}

func TestTransitionUnmarshal_ResetsSignableCache(t *testing.T) {
	reused := buildValidTransitionMessage()
	if _, err := reused.SignableBytes(); err != nil { // prime the cache
		t.Fatalf("prime cache: %v", err)
	}

	// Decode a structurally different genuine bundle into the SAME value.
	other := buildValidTransitionMessage()
	other.CoordinatorIDValue = 2
	other.CoordinatorSignature = bytes.Repeat([]byte{0xcd}, 64)
	wire, err := other.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := reused.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal into reused value: %v", err)
	}

	got, _ := reused.SignableBytes()
	want, _ := other.SignableBytes()
	if !bytes.Equal(got, want) {
		t.Fatal("Unmarshal must reset the transition signable-bytes cache")
	}
}

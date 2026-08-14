package roast

import (
	"bytes"
	"errors"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
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

func TestCrossProtocol_TransitionSignatureRejectedAsSnapshot(t *testing.T) {
	// End-to-end: a coordinator signature over a transition message must not
	// verify as an operator signature over a snapshot, even when the snapshot
	// shares the signer id and attempt context. The distinct domain tags (and
	// bodies) make the signed preimages disjoint.
	const id group.MemberIndex = 7

	transition := buildValidTransitionMessage()
	transition.CoordinatorIDValue = uint32(id)
	transition.CoordinatorSignature = nil
	tPayload, err := transition.SignableBytes()
	if err != nil {
		t.Fatalf("transition signable: %v", err)
	}
	tSig, err := (&fakeSigner{id: id}).Sign(tPayload)
	if err != nil {
		t.Fatalf("sign transition: %v", err)
	}
	transition.CoordinatorSignature = tSig
	// Control: the signature really is a valid bundle signature.
	if err := verifyBundleSignature(fakeVerifier{}, transition, id); err != nil {
		t.Fatalf("control: genuine transition signature must verify: %v", err)
	}

	// Paste it onto a snapshot from the same signer + attempt.
	snap := NewLocalEvidenceSnapshot(id, pinnedContextHash, attempt.Evidence{})
	snap.OperatorSignature = tSig
	if err := verifySnapshotSignature(fakeVerifier{}, snap); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("transition signature must not verify as a snapshot signature; got %v", err)
	}
}

func TestCrossProtocol_SnapshotSignatureRejectedAsTransition(t *testing.T) {
	// The reverse direction: an operator signature over a snapshot must not
	// verify as a coordinator signature over a transition message.
	const id group.MemberIndex = 7
	snap := signedTestSnapshot(t, id)
	// Control: it verifies as a snapshot signature.
	if err := verifySnapshotSignature(fakeVerifier{}, snap); err != nil {
		t.Fatalf("control: genuine snapshot signature must verify: %v", err)
	}

	transition := buildValidTransitionMessage()
	transition.CoordinatorIDValue = uint32(id)
	transition.CoordinatorSignature = snap.OperatorSignature
	if err := verifyBundleSignature(fakeVerifier{}, transition, id); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("snapshot signature must not verify as a transition signature; got %v", err)
	}
}

func TestSnapshotSignableBytes_ConcurrentAfterUnmarshalIsRaceFree(t *testing.T) {
	// Regression guard (run under -race): a parsed snapshot must carry a primed
	// signable-bytes cache so concurrent signature verification reads a ready
	// cache instead of racing on lazy initialization. Without priming in
	// Unmarshal, the concurrent first SignableBytes calls below race on the
	// cache write.
	wire, err := signedTestSnapshot(t, 7).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded LocalEvidenceSnapshot
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

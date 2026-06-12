package roast

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// The properties pinned here are the point of the signed-body envelope
// format: signatures verify over exactly the bytes received, those bytes
// travel verbatim through re-broadcast and bundle aggregation, and no step
// ever depends on a serializer's canonical form.

func signedTestSnapshot(t *testing.T, sender group.MemberIndex) *LocalEvidenceSnapshot {
	t.Helper()
	signer := &fakeSigner{id: sender}
	snap := NewLocalEvidenceSnapshot(sender, pinnedContextHash, attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{1: 2, 3: 4},
	})
	payload, err := snap.SignableBytes()
	if err != nil {
		t.Fatalf("signable bytes: %v", err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	snap.OperatorSignature = sig
	return snap
}

func TestSnapshotWire_ReceivedBytesPreservedVerbatim(t *testing.T) {
	original := signedTestSnapshot(t, 7)
	wire, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &LocalEvidenceSnapshot{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := verifySnapshotSignature(fakeVerifier{}, decoded); err != nil {
		t.Fatalf("verify decoded: %v", err)
	}

	rebroadcast, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(rebroadcast, wire) {
		t.Fatal("re-marshal of a received snapshot must return the received bytes verbatim")
	}

	producerBody, _ := original.SignableBytes()
	receiverBody, _ := decoded.SignableBytes()
	if !bytes.Equal(producerBody, receiverBody) {
		t.Fatal("receiver must verify over exactly the bytes the producer signed")
	}
}

func TestSnapshotWire_NonCanonicalEnvelopeEncodingSurvives(t *testing.T) {
	original := signedTestSnapshot(t, 7)
	body, _ := original.SignableBytes()

	// Handcraft an envelope with the fields in REVERSE tag order
	// (operator_signature before body) - a wire-legal but non-canonical
	// encoding no Go marshaler would produce. Field 1 (body) and field 2
	// (operator_signature) are both length-delimited: tags 0x0a and 0x12.
	var crafted []byte
	crafted = append(crafted, 0x12, byte(len(original.OperatorSignature)))
	crafted = append(crafted, original.OperatorSignature...)
	crafted = append(crafted, 0x0a, byte(len(body)))
	crafted = append(crafted, body...)

	// Sanity: protobuf accepts field-order-free encodings.
	var check pb.SignedLocalEvidenceSnapshot
	if err := proto.Unmarshal(crafted, &check); err != nil {
		t.Fatalf("crafted envelope must be wire-legal: %v", err)
	}

	decoded := &LocalEvidenceSnapshot{}
	if err := decoded.Unmarshal(crafted); err != nil {
		t.Fatalf("unmarshal crafted: %v", err)
	}
	if err := verifySnapshotSignature(fakeVerifier{}, decoded); err != nil {
		t.Fatalf("signature must verify over the embedded body bytes: %v", err)
	}

	remarshaled, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(remarshaled, crafted) {
		t.Fatal("re-marshal must preserve even a non-canonical received encoding verbatim")
	}
}

func TestBundleWire_EmbedsReceivedSnapshotEnvelopesVerbatim(t *testing.T) {
	snapshotWire := make([][]byte, 0, 2)
	bundle := make([]LocalEvidenceSnapshot, 0, 2)
	for _, sender := range []group.MemberIndex{1, 2} {
		wire, err := signedTestSnapshot(t, sender).Marshal()
		if err != nil {
			t.Fatalf("marshal snapshot: %v", err)
		}
		// The coordinator receives the snapshot off the wire.
		var received LocalEvidenceSnapshot
		if err := received.Unmarshal(wire); err != nil {
			t.Fatalf("coordinator unmarshal: %v", err)
		}
		snapshotWire = append(snapshotWire, wire)
		bundle = append(bundle, received)
	}

	coordinator := &fakeSigner{id: 2}
	msg := &TransitionMessage{
		AttemptContextHash: append([]byte{}, pinnedContextHash[:]...),
		CoordinatorIDValue: 2,
		Bundle:             bundle,
	}
	payload, err := msg.SignableBytes()
	if err != nil {
		t.Fatalf("bundle signable bytes: %v", err)
	}
	for _, wire := range snapshotWire {
		if !bytes.Contains(payload, wire) {
			t.Fatal("bundle body must embed each received snapshot envelope verbatim")
		}
	}
	sig, err := coordinator.Sign(payload)
	if err != nil {
		t.Fatalf("sign bundle: %v", err)
	}
	msg.CoordinatorSignature = sig

	bundleWire, err := msg.Marshal()
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	decoded := &TransitionMessage{}
	if err := decoded.Unmarshal(bundleWire); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if err := verifyBundleSignature(fakeVerifier{}, decoded, 2); err != nil {
		t.Fatalf("verify bundle: %v", err)
	}
	for i := range decoded.Bundle {
		if err := verifySnapshotSignature(fakeVerifier{}, &decoded.Bundle[i]); err != nil {
			t.Fatalf("verify embedded snapshot %d: %v", i, err)
		}
	}

	rebroadcast, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal bundle: %v", err)
	}
	if !bytes.Equal(rebroadcast, bundleWire) {
		t.Fatal("re-marshal of a received bundle must return the received bytes verbatim")
	}
}

func TestSnapshotWire_TamperedBodyFailsVerification(t *testing.T) {
	original := signedTestSnapshot(t, 7)
	wire, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Flip one byte inside the embedded body (the attempt context hash
	// content sits well inside the envelope; the envelope structure stays
	// parseable because only a value byte changes).
	tampered := append([]byte(nil), wire...)
	idx := bytes.Index(tampered, pinnedContextHash[:])
	if idx < 0 {
		t.Fatal("context hash not found in wire bytes")
	}
	tampered[idx] ^= 0xff

	decoded := &LocalEvidenceSnapshot{}
	if err := decoded.Unmarshal(tampered); err != nil {
		t.Fatalf("tampered envelope still parses (only a value changed): %v", err)
	}
	if err := verifySnapshotSignature(fakeVerifier{}, decoded); err == nil {
		t.Fatal("signature over tampered body bytes must fail verification")
	}
}

package roast

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// fakeSigner produces deterministic signatures of the form
// SHA256(memberID || payload) so tests can exercise the sign / verify
// pipeline without real crypto. Two fakeSigners with the same member
// id produce identical signatures.
type fakeSigner struct {
	id group.MemberIndex
}

func (f *fakeSigner) Sign(payload []byte) ([]byte, error) {
	h := sha256.New()
	h.Write([]byte{byte(f.id)})
	h.Write(payload)
	return h.Sum(nil), nil
}

// fakeVerifier mirrors fakeSigner's deterministic signature scheme so
// every member's signatures verify against the same recomputation.
// A signature attributed to memberID is valid iff it equals
// SHA256(memberID || payload).
type fakeVerifier struct{}

func (fakeVerifier) Verify(payload, signature []byte, signer group.MemberIndex) error {
	h := sha256.New()
	h.Write([]byte{byte(signer)})
	h.Write(payload)
	expected := h.Sum(nil)
	if !bytes.Equal(expected, signature) {
		return errors.New("fakeVerifier: signature does not match recomputed value")
	}
	return nil
}

func TestNoOpSigner_ReturnsEmptySignature(t *testing.T) {
	sig, err := NoOpSigner().Sign([]byte("payload"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sig) != 0 {
		t.Fatalf("expected empty signature, got %x", sig)
	}
}

func TestNoOpSignatureVerifier_AcceptsEverything(t *testing.T) {
	v := NoOpSignatureVerifier()
	if err := v.Verify([]byte("a"), []byte("b"), 1); err != nil {
		t.Fatalf("NoOp must accept everything: %v", err)
	}
	if err := v.Verify(nil, nil, 1); err != nil {
		t.Fatalf("NoOp must accept nil payload + nil sig: %v", err)
	}
}

func TestNoOpSigner_IsConcurrencySafe(t *testing.T) {
	signer := NoOpSigner()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 32; j++ {
				if _, err := signer.Sign([]byte("payload")); err != nil {
					t.Errorf("Sign error under concurrency: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestCanonicalSnapshotBytes_ExcludesOperatorSignature(t *testing.T) {
	snap := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{1: 2, 3: 4},
	})
	withoutSig, err := snap.SignableBytes()
	if err != nil {
		t.Fatalf("canonical bytes (no sig): %v", err)
	}
	snap.OperatorSignature = []byte{0xff, 0xee}
	withSig, err := snap.SignableBytes()
	if err != nil {
		t.Fatalf("canonical bytes (with sig): %v", err)
	}
	if !bytes.Equal(withoutSig, withSig) {
		t.Fatalf(
			"adding OperatorSignature changed canonical bytes; got %s vs %s",
			string(withoutSig), string(withSig),
		)
	}
}

func TestCanonicalSnapshotBytes_RejectsNil(t *testing.T) {
	if _, err := (*LocalEvidenceSnapshot)(nil).SignableBytes(); err == nil {
		t.Fatal("expected error for nil snapshot")
	}
}

func TestBundleSignableBytes_ExcludeCoordinatorSignatureButIncludeSnapshotSignatures(t *testing.T) {
	// Two fresh messages identical except for the coordinator signature
	// must sign over the same bytes (the signature is over the body, not
	// part of it). Fresh messages are required because signable bytes are
	// computed once and cached.
	msgA := buildValidTransitionMessage()
	msgA.CoordinatorSignature = bytes.Repeat([]byte{0xaa}, 64)
	msgB := buildValidTransitionMessage()
	msgB.CoordinatorSignature = bytes.Repeat([]byte{0xbb}, 64)
	bytesA, err := msgA.SignableBytes()
	if err != nil {
		t.Fatalf("signable bundle A: %v", err)
	}
	bytesB, err := msgB.SignableBytes()
	if err != nil {
		t.Fatalf("signable bundle B: %v", err)
	}
	if !bytes.Equal(bytesA, bytesB) {
		t.Fatal("coordinator signature leaked into the signed bundle body")
	}

	// A distinctive per-snapshot operator signature must appear verbatim
	// inside the signed bundle body: the coordinator attests to the exact
	// signed snapshot envelopes.
	distinctive := bytes.Repeat([]byte{0xc7, 0x3d}, 16)
	msgC := buildValidTransitionMessage()
	msgC.Bundle[0].OperatorSignature = distinctive
	bytesC, err := msgC.SignableBytes()
	if err != nil {
		t.Fatalf("signable bundle C: %v", err)
	}
	if !bytes.Contains(bytesC, distinctive) {
		t.Fatal("per-snapshot operator signature missing from signed bundle body")
	}
	if bytes.Equal(bytesA, bytesC) {
		t.Fatal("changing a snapshot operator signature must change the bundle body")
	}
}

func TestCanonicalBundleBytes_RejectsNil(t *testing.T) {
	if _, err := (*TransitionMessage)(nil).SignableBytes(); err == nil {
		t.Fatal("expected error for nil message")
	}
}

func TestVerifySnapshotSignature_RoundTripsThroughFakeSignerVerifier(t *testing.T) {
	signer := &fakeSigner{id: 7}
	snap := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{})
	payload, err := snap.SignableBytes()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	snap.OperatorSignature = sig
	if err := verifySnapshotSignature(fakeVerifier{}, snap); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestVerifySnapshotSignature_RejectsMissingSignature(t *testing.T) {
	snap := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{})
	err := verifySnapshotSignature(fakeVerifier{}, snap)
	if !errors.Is(err, ErrSignatureMissing) {
		t.Fatalf("expected ErrSignatureMissing, got %v", err)
	}
}

func TestVerifySnapshotSignature_RejectsTamperedPayload(t *testing.T) {
	signer := &fakeSigner{id: 7}
	snap := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{})
	payload, _ := snap.SignableBytes()
	sig, _ := signer.Sign(payload)
	// Attach the signature to a *different* snapshot (fresh struct, so
	// no cached bytes): its signed body differs, so verification over
	// its bytes must fail.
	tampered := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{
		Overflows: map[group.MemberIndex]uint{99: 1},
	})
	tampered.OperatorSignature = sig
	if err := verifySnapshotSignature(fakeVerifier{}, tampered); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyBundleSignature_RoundTrip(t *testing.T) {
	signer := &fakeSigner{id: 11}
	msg := buildValidTransitionMessage()
	msg.CoordinatorIDValue = 11
	msg.CoordinatorSignature = nil
	payload, _ := msg.SignableBytes()
	sig, _ := signer.Sign(payload)
	msg.CoordinatorSignature = sig
	if err := verifyBundleSignature(fakeVerifier{}, msg, 11); err != nil {
		t.Fatalf("expected verified, got %v", err)
	}
}

func TestVerifyBundleSignature_RejectsCoordinatorMismatch(t *testing.T) {
	msg := buildValidTransitionMessage()
	msg.CoordinatorIDValue = 1
	msg.CoordinatorSignature = []byte{0x01}
	err := verifyBundleSignature(fakeVerifier{}, msg, 99)
	if err == nil {
		t.Fatal("expected coordinator mismatch error")
	}
}

func TestVerifyOwnObservationsPresent_RequiresIdenticalSignature(t *testing.T) {
	selfSubmission := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{})
	selfSubmission.OperatorSignature = []byte{0xab}
	bundle := &TransitionMessage{
		Bundle: []LocalEvidenceSnapshot{
			func() LocalEvidenceSnapshot {
				s := *selfSubmission
				s.OperatorSignature = []byte{0xff}
				return s
			}(),
		},
	}
	if err := verifyOwnObservationsPresent(bundle, 7, selfSubmission); !errors.Is(err, ErrCensorshipDetected) {
		t.Fatalf("expected ErrCensorshipDetected on mutated sig, got %v", err)
	}
}

func TestVerifyOwnObservationsPresent_DetectsMissingSnapshot(t *testing.T) {
	selfSubmission := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{})
	bundle := &TransitionMessage{
		Bundle: []LocalEvidenceSnapshot{
			*NewLocalEvidenceSnapshot(8, pinnedContextHash, attempt.Evidence{}),
		},
	}
	if err := verifyOwnObservationsPresent(bundle, 7, selfSubmission); !errors.Is(err, ErrCensorshipDetected) {
		t.Fatalf("expected ErrCensorshipDetected, got %v", err)
	}
}

func TestVerifyOwnObservationsPresent_SkipsWhenSelfZero(t *testing.T) {
	bundle := &TransitionMessage{Bundle: []LocalEvidenceSnapshot{}}
	if err := verifyOwnObservationsPresent(bundle, 0, nil); err != nil {
		t.Fatalf("expected skip, got %v", err)
	}
}

func TestVerifyOwnObservationsPresent_SkipsWhenNoSelfSubmission(t *testing.T) {
	bundle := &TransitionMessage{Bundle: []LocalEvidenceSnapshot{}}
	if err := verifyOwnObservationsPresent(bundle, 7, nil); err != nil {
		t.Fatalf("expected skip when no self submission, got %v", err)
	}
}

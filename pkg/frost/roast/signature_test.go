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
	withoutSig, err := CanonicalSnapshotBytes(snap)
	if err != nil {
		t.Fatalf("canonical bytes (no sig): %v", err)
	}
	snap.OperatorSignature = []byte{0xff, 0xee}
	withSig, err := CanonicalSnapshotBytes(snap)
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
	if _, err := CanonicalSnapshotBytes(nil); err == nil {
		t.Fatal("expected error for nil snapshot")
	}
}

func TestCanonicalBundleBytes_ExcludesCoordinatorSignatureButIncludesSnapshots(t *testing.T) {
	msg := buildValidTransitionMessage()
	// Make sure each snapshot's OperatorSignature is non-empty so we
	// can verify they appear in the canonical bytes.
	for i := range msg.Bundle {
		msg.Bundle[i].OperatorSignature = []byte{byte(i + 1)}
	}
	msg.CoordinatorSignature = []byte{0xaa, 0xbb}
	canonical, err := CanonicalBundleBytes(msg)
	if err != nil {
		t.Fatalf("canonical bundle: %v", err)
	}
	// CoordinatorSignature bytes should not appear in the canonical
	// payload (omitempty + nil in clone).
	if bytes.Contains(canonical, []byte{0xaa, 0xbb}) {
		t.Fatalf(
			"CoordinatorSignature 0xaabb leaked into canonical bytes: %s",
			string(canonical),
		)
	}
	// Each snapshot's OperatorSignature should appear via base64
	// "AQ==", "Ag==", "Aw==" (1, 2, 3 → 0x01, 0x02, 0x03).
	for _, want := range []string{`"AQ=="`, `"Ag=="`, `"Aw=="`} {
		if !bytes.Contains(canonical, []byte(want)) {
			t.Fatalf(
				"expected per-snapshot OperatorSignature %q in canonical bundle: %s",
				want, string(canonical),
			)
		}
	}
}

func TestCanonicalBundleBytes_RejectsNil(t *testing.T) {
	if _, err := CanonicalBundleBytes(nil); err == nil {
		t.Fatal("expected error for nil message")
	}
}

func TestVerifySnapshotSignature_RoundTripsThroughFakeSignerVerifier(t *testing.T) {
	signer := &fakeSigner{id: 7}
	snap := NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{})
	payload, err := CanonicalSnapshotBytes(snap)
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
	payload, _ := CanonicalSnapshotBytes(snap)
	sig, _ := signer.Sign(payload)
	snap.OperatorSignature = sig
	// Tamper: change the overflow set; the recomputed canonical
	// bytes will no longer match.
	snap.Overflows = []OverflowEntry{{Sender: 99, Count: 1}}
	if err := verifySnapshotSignature(fakeVerifier{}, snap); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyBundleSignature_RoundTrip(t *testing.T) {
	signer := &fakeSigner{id: 11}
	msg := buildValidTransitionMessage()
	msg.CoordinatorIDValue = 11
	msg.CoordinatorSignature = nil
	payload, _ := CanonicalBundleBytes(msg)
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

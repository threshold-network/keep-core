package roast

import (
	"bytes"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// Proofs are opaque bytes to the snapshot layer: PR2 carries + canonicalizes
// them and folds them into the signed body; PR3 (NextAttempt) verifies them.

func TestLocalEvidenceSnapshot_CoordinatorPackageProofs_RoundTrip(t *testing.T) {
	proofA := []byte("coordinator-signed-package-A")
	proofB := []byte("coordinator-signed-package-B")

	// Passed out of order; the constructor must canonicalize (bytewise ascending).
	snap := signSnapshotForTest(t, NewLocalEvidenceSnapshot(
		3, pinnedContextHash, attempt.Evidence{}, proofB, proofA,
	))
	if len(snap.CoordinatorPackageProofs) != 2 ||
		!bytes.Equal(snap.CoordinatorPackageProofs[0], proofA) ||
		!bytes.Equal(snap.CoordinatorPackageProofs[1], proofB) {
		t.Fatalf("constructor must store proofs in bytewise-ascending order, got %q",
			snap.CoordinatorPackageProofs)
	}

	wire, err := snap.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &LocalEvidenceSnapshot{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.CoordinatorPackageProofs) != 2 ||
		!bytes.Equal(decoded.CoordinatorPackageProofs[0], proofA) ||
		!bytes.Equal(decoded.CoordinatorPackageProofs[1], proofB) {
		t.Fatalf("proofs must survive the wire round-trip verbatim, got %q",
			decoded.CoordinatorPackageProofs)
	}

	// The proofs are part of the signed body: re-marshal returns the received
	// bytes verbatim, and the producer/receiver signed payloads match exactly.
	rebroadcast, err := decoded.Marshal()
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(rebroadcast, wire) {
		t.Fatal("re-marshal of a received snapshot must return the received bytes verbatim")
	}
	producer, _ := snap.SignableBytes()
	receiver, _ := decoded.SignableBytes()
	if !bytes.Equal(producer, receiver) {
		t.Fatal("the receiver must verify over exactly the bytes the signer signed")
	}
}

func TestLocalEvidenceSnapshot_NoProofsEncodesAsBefore(t *testing.T) {
	snap := signSnapshotForTest(t, NewLocalEvidenceSnapshot(3, pinnedContextHash, attempt.Evidence{}))
	if snap.CoordinatorPackageProofs != nil {
		t.Fatalf("no proofs passed -> field must be nil, got %q", snap.CoordinatorPackageProofs)
	}
	wire, err := snap.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &LocalEvidenceSnapshot{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.CoordinatorPackageProofs) != 0 {
		t.Fatalf("a proof-free snapshot must decode with no proofs, got %q",
			decoded.CoordinatorPackageProofs)
	}
}

func TestLocalEvidenceSnapshot_CoordinatorPackageProofs_ValidateRejections(t *testing.T) {
	base := func() *LocalEvidenceSnapshot {
		return &LocalEvidenceSnapshot{
			SenderIDValue:      3,
			AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
		}
	}

	t.Run("over the cap", func(t *testing.T) {
		s := base()
		s.CoordinatorPackageProofs = [][]byte{{0x01}, {0x02}, {0x03}}
		if err := s.Validate(); err == nil {
			t.Fatal("more than two proofs must be rejected")
		}
	})
	t.Run("unsorted", func(t *testing.T) {
		s := base()
		s.CoordinatorPackageProofs = [][]byte{{0x02}, {0x01}}
		if err := s.Validate(); err == nil {
			t.Fatal("unsorted proofs must be rejected")
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		s := base()
		s.CoordinatorPackageProofs = [][]byte{{0x01}, {0x01}}
		if err := s.Validate(); err == nil {
			t.Fatal("duplicate proofs must be rejected")
		}
	})
	t.Run("empty proof", func(t *testing.T) {
		s := base()
		s.CoordinatorPackageProofs = [][]byte{{}}
		if err := s.Validate(); err == nil {
			t.Fatal("an empty proof must be rejected")
		}
	})
	t.Run("oversized proof", func(t *testing.T) {
		s := base()
		s.CoordinatorPackageProofs = [][]byte{bytes.Repeat([]byte{0x01}, MaxSignedSigningPackageBytes+1)}
		if err := s.Validate(); err == nil {
			t.Fatal("an oversized proof must be rejected")
		}
	})
	t.Run("valid two", func(t *testing.T) {
		s := base()
		s.CoordinatorPackageProofs = [][]byte{{0x01}, {0x02}}
		if err := s.Validate(); err != nil {
			t.Fatalf("two sorted, distinct, bounded proofs must validate: %v", err)
		}
	})
}

func TestLocalEvidenceSnapshot_Unmarshal_RejectsOversizedEnvelope(t *testing.T) {
	// Carrying coordinator package proofs makes a legitimate snapshot MBs large,
	// so Unmarshal must reject a grossly oversized envelope before the protobuf
	// decoder allocates for it (memory-DoS guard, mirroring SigningPackage /
	// ShareSubmission). TransitionMessage.Unmarshal applies the analogous cap.
	oversized := make([]byte, MaxSignedLocalEvidenceSnapshotBytes+1)
	if err := (&LocalEvidenceSnapshot{}).Unmarshal(oversized); err == nil {
		t.Fatal("an oversized snapshot envelope must be rejected before decoding")
	}
}

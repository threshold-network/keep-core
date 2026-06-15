package roast

import (
	"bytes"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func mustMarshalProof(t *testing.T, pkg *SigningPackage) []byte {
	t.Helper()
	b, err := pkg.Marshal()
	if err != nil {
		t.Fatalf("marshal signing package proof: %v", err)
	}
	return b
}

// equivocationBundle builds a bundle over the full fixture signer set {1..5}
// (so no member is silence-parked and excluding the coordinator stays feasible),
// bound to pinnedContextHash, with the given per-sender coordinator package
// proofs.
func equivocationBundle(coordinator group.MemberIndex, proofsBySender map[uint32][][]byte) *TransitionMessage {
	bundle := make([]LocalEvidenceSnapshot, 0, 5)
	for _, s := range []uint32{1, 2, 3, 4, 5} {
		bundle = append(bundle, LocalEvidenceSnapshot{
			SenderIDValue:            s,
			AttemptContextHash:       append([]byte(nil), pinnedContextHash[:]...),
			CoordinatorPackageProofs: proofsBySender[s],
		})
	}
	return &TransitionMessage{
		AttemptContextHash: append([]byte(nil), pinnedContextHash[:]...),
		CoordinatorIDValue: uint32(coordinator),
		Bundle:             bundle,
	}
}

func TestComputeNextAttempt_ProofCarryingCoordinatorEquivocationExcludesCoordinator(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	const coordinator = group.MemberIndex(1) // the fixture's bundle coordinator

	// Coordinator 1 distributed two body-different signing packages for this
	// attempt; members 2 and 3 each surface ONE (the targeted/split case, where no
	// single observer sees both). Authenticated + distinct => unforgeable
	// equivocation => INSTANT exclusion (no f+1 gate).
	proofA := mustMarshalProof(t, signedTestSigningPackage(t, coordinator, nil))
	proofB := mustMarshalProof(t, signedTestSigningPackage(
		t, coordinator, bytes.Repeat([]byte{0xab}, TaprootMerkleRootLength),
	))

	bundle := equivocationBundle(coordinator, map[uint32][][]byte{
		2: {proofA},
		3: {proofB},
	})

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.ExcludedSet, coordinator) {
		t.Fatalf("equivocating coordinator %d must be excluded; excluded=%v", coordinator, next.ExcludedSet)
	}
	if memberSliceContains(next.IncludedSet, coordinator) {
		t.Fatalf("excluded coordinator must drop from the next included set; included=%v", next.IncludedSet)
	}
}

func TestComputeNextAttempt_SingleCoordinatorPackageProofDoesNotExclude(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	const coordinator = group.MemberIndex(1)

	// Every member publishes the ONE package it accepted: legitimate, not
	// equivocation. Two members carrying the SAME package is still one distinct
	// body.
	proof := mustMarshalProof(t, signedTestSigningPackage(t, coordinator, nil))
	bundle := equivocationBundle(coordinator, map[uint32][][]byte{
		2: {proof},
		3: {proof},
	})

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.ExcludedSet, coordinator) {
		t.Fatalf("a single (or repeated identical) proof must not exclude the coordinator; excluded=%v", next.ExcludedSet)
	}
}

func TestComputeNextAttempt_UnauthenticatedProofsAreIgnored(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	const coordinator = group.MemberIndex(1)

	// Only authentic, this-coordinator, this-attempt proofs count. A garbage
	// envelope and a package signed by a DIFFERENT member (wrong coordinator) are
	// ignored - so there are not two distinct VALID bodies, and the coordinator
	// is not blamed.
	authentic := mustMarshalProof(t, signedTestSigningPackage(t, coordinator, nil))
	wrongCoordinator := mustMarshalProof(t, signedTestSigningPackage(
		t, 4, bytes.Repeat([]byte{0xcd}, TaprootMerkleRootLength),
	))
	bundle := equivocationBundle(coordinator, map[uint32][][]byte{
		2: {authentic, []byte("not-a-signing-package")},
		3: {wrongCoordinator},
	})

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.ExcludedSet, coordinator) {
		t.Fatalf("garbage / wrong-coordinator proofs must be ignored; excluded=%v", next.ExcludedSet)
	}
}

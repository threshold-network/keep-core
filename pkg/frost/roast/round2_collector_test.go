package roast

import (
	"bytes"
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func testIncludedSet() []group.MemberIndex {
	return []group.MemberIndex{3, 5, 7}
}

func TestRound2Collector_BeginAttempt(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	const elected = group.MemberIndex(3)

	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("begin: %v", err)
	}
	// Idempotent for an identical binding.
	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("re-begin (identical) must be idempotent: %v", err)
	}
	// Conflicting elected coordinator.
	if err := c.BeginAttempt(pinnedContextHash[:], elected+1, testIncludedSet()); !errors.Is(err, ErrRound2AttemptBindingConflict) {
		t.Fatalf("want ErrRound2AttemptBindingConflict for a different coordinator, got %v", err)
	}
	// Conflicting included set.
	if err := c.BeginAttempt(pinnedContextHash[:], elected, []group.MemberIndex{3, 5}); !errors.Is(err, ErrRound2AttemptBindingConflict) {
		t.Fatalf("want ErrRound2AttemptBindingConflict for a different included set, got %v", err)
	}
	// Malformed attempt hash + zero coordinator are rejected.
	if err := c.BeginAttempt([]byte{1, 2, 3}, elected, testIncludedSet()); err == nil {
		t.Fatal("a short attempt context hash must be rejected")
	}
	if err := c.BeginAttempt(bytes.Repeat([]byte{0x01}, 32), 0, testIncludedSet()); err == nil {
		t.Fatal("a zero elected coordinator must be rejected")
	}
}

func TestRound2Collector_RecordSigningPackage_RetainsAuthenticated(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	const elected = group.MemberIndex(3)
	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("begin: %v", err)
	}

	if err := c.RecordSigningPackage(signedTestSigningPackage(t, elected, nil)); err != nil {
		t.Fatalf("record authenticated package: %v", err)
	}
	// An identical package (deterministic fakeSigner) re-records idempotently.
	if err := c.RecordSigningPackage(signedTestSigningPackage(t, elected, nil)); err != nil {
		t.Fatalf("re-record identical package must be idempotent: %v", err)
	}
}

func TestRound2Collector_RecordSigningPackage_Rejections(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	const elected = group.MemberIndex(3)

	// No binding yet.
	if err := c.RecordSigningPackage(signedTestSigningPackage(t, elected, nil)); !errors.Is(err, ErrRound2UnknownAttempt) {
		t.Fatalf("want ErrRound2UnknownAttempt before BeginAttempt, got %v", err)
	}

	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("begin: %v", err)
	}
	// A package from a non-elected coordinator fails authentication and is not
	// retained.
	if err := c.RecordSigningPackage(signedTestSigningPackage(t, elected+6, nil)); !errors.Is(err, ErrSigningPackageWrongCoordinator) {
		t.Fatalf("want ErrSigningPackageWrongCoordinator, got %v", err)
	}
}

func TestRound2Collector_RecordSigningPackage_DetectsCoordinatorEquivocation(t *testing.T) {
	captured := captureEquivocationEvidence(t)
	c := NewRound2Collector(fakeVerifier{})
	const elected = group.MemberIndex(3)
	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("begin: %v", err)
	}

	if err := c.RecordSigningPackage(signedTestSigningPackage(t, elected, nil)); err != nil {
		t.Fatalf("record first package: %v", err)
	}
	// A second, DIFFERENT authenticated package (script-path root) for the same
	// attempt is coordinator equivocation.
	scriptRoot := bytes.Repeat([]byte{0xab}, TaprootMerkleRootLength)
	err := c.RecordSigningPackage(signedTestSigningPackage(t, elected, scriptRoot))
	if !errors.Is(err, ErrSigningPackageConflict) {
		t.Fatalf("want ErrSigningPackageConflict, got %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 equivocation event, got %d", len(*captured))
	}
	ev := (*captured)[0]
	if ev.Kind != EquivocationKindSigningPackageConflict {
		t.Fatalf("want kind %q, got %q", EquivocationKindSigningPackageConflict, ev.Kind)
	}
	if ev.Sender != elected {
		t.Fatalf("want sender %d (elected coordinator), got %d", elected, ev.Sender)
	}
	if len(ev.ExistingEnvelope) == 0 || len(ev.ConflictingEnvelope) == 0 {
		t.Fatal("both the existing and conflicting envelopes must be retained as evidence")
	}
	if bytes.Equal(ev.ExistingEnvelope, ev.ConflictingEnvelope) {
		t.Fatal("the conflicting envelope must differ from the existing one")
	}
}

func TestRound2Collector_PruneAttempt(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	const elected = group.MemberIndex(3)
	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("begin: %v", err)
	}
	c.PruneAttempt(pinnedContextHash[:])
	// After pruning, the binding is gone: recording requires BeginAttempt again.
	if err := c.RecordSigningPackage(signedTestSigningPackage(t, elected, nil)); !errors.Is(err, ErrRound2UnknownAttempt) {
		t.Fatalf("want ErrRound2UnknownAttempt after prune, got %v", err)
	}
}

func TestRound2Collector_RecordSigningPackage_EnvelopeReEncodingIsNotEquivocation(t *testing.T) {
	captured := captureEquivocationEvidence(t)
	c := NewRound2Collector(fakeVerifier{})
	const elected = group.MemberIndex(3)
	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("begin: %v", err)
	}

	pkg := signedTestSigningPackage(t, elected, nil)
	if err := c.RecordSigningPackage(pkg); err != nil {
		t.Fatalf("record: %v", err)
	}

	// Re-wrap the SAME (body, signature) in a different valid envelope encoding
	// (reversed field order). The coordinator signature does not cover the outer
	// envelope, so this is the same signed body - not equivocation.
	body, _ := pkg.bodyBytes()
	var reEncoded []byte
	reEncoded = append(reEncoded, 0x12, byte(len(pkg.CoordinatorSignature)))
	reEncoded = append(reEncoded, pkg.CoordinatorSignature...)
	reEncoded = append(reEncoded, 0x0a, byte(len(body)))
	reEncoded = append(reEncoded, body...)
	var reDecoded SigningPackage
	if err := reDecoded.Unmarshal(reEncoded); err != nil {
		t.Fatalf("unmarshal re-encoded: %v", err)
	}

	if err := c.RecordSigningPackage(&reDecoded); err != nil {
		t.Fatalf("a re-encoded same-body package must record idempotently, got %v", err)
	}
	if len(*captured) != 0 {
		t.Fatalf("envelope re-encoding must NOT emit equivocation evidence, got %d events", len(*captured))
	}
}

func TestRound2Collector_RecordSigningPackage_RetainsOwnedCopy(t *testing.T) {
	captured := captureEquivocationEvidence(t)
	c := NewRound2Collector(fakeVerifier{})
	const elected = group.MemberIndex(3)
	if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Record pkgA, capturing its original envelope bytes.
	pkgA := signedTestSigningPackage(t, elected, nil)
	origWire, err := pkgA.Marshal()
	if err != nil {
		t.Fatalf("marshal pkgA: %v", err)
	}
	origWire = append([]byte(nil), origWire...)
	if err := c.RecordSigningPackage(pkgA); err != nil {
		t.Fatalf("record pkgA: %v", err)
	}

	// Mutate the caller's pkgA object (as a struct-reusing receive loop would,
	// via Unmarshal). The collector must have retained its own copy.
	pkgB := signedTestSigningPackage(t, elected, bytes.Repeat([]byte{0xab}, TaprootMerkleRootLength))
	bWire, _ := pkgB.Marshal()
	if err := pkgA.Unmarshal(append([]byte(nil), bWire...)); err != nil {
		t.Fatalf("mutate pkgA: %v", err)
	}

	// Recording pkgB (different body) is equivocation; the evidence must carry
	// pkgA's ORIGINAL envelope, not the mutated bytes.
	if err := c.RecordSigningPackage(pkgB); !errors.Is(err, ErrSigningPackageConflict) {
		t.Fatalf("want ErrSigningPackageConflict, got %v", err)
	}
	if len(*captured) != 1 {
		t.Fatalf("expected 1 equivocation event, got %d", len(*captured))
	}
	if !bytes.Equal((*captured)[0].ExistingEnvelope, origWire) {
		t.Fatal("retained existing envelope must be the collector's own copy, unaffected by caller mutation")
	}
}

func TestRound2_NilInputsAreRejectedNotPanicked(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	if err := c.RecordSigningPackage(nil); err == nil {
		t.Fatal("RecordSigningPackage(nil) must return an error, not panic")
	}
	// The auth entry points validate first, so a nil package/share is rejected
	// via the Validate nil-receiver guard rather than panicking.
	if err := AuthenticateSigningPackage(fakeVerifier{}, nil, 3, pinnedContextHash[:]); err == nil {
		t.Fatal("AuthenticateSigningPackage(nil) must return an error")
	}
	if err := AuthenticateShareSubmission(fakeVerifier{}, nil, 3, pinnedContextHash[:], testSigningPackageHash()); err == nil {
		t.Fatal("AuthenticateShareSubmission(nil) must return an error")
	}

	// Marshal/Unmarshal on a nil receiver must error, not panic - matching the
	// Validate/SignableBytes/BodyHash contract.
	if _, err := (*SigningPackage)(nil).Marshal(); err == nil {
		t.Fatal("SigningPackage.Marshal on a nil receiver must return an error")
	}
	if err := (*SigningPackage)(nil).Unmarshal([]byte{0x01}); err == nil {
		t.Fatal("SigningPackage.Unmarshal into a nil receiver must return an error")
	}
	if _, err := (*ShareSubmission)(nil).Marshal(); err == nil {
		t.Fatal("ShareSubmission.Marshal on a nil receiver must return an error")
	}
	if err := (*ShareSubmission)(nil).Unmarshal([]byte{0x01}); err == nil {
		t.Fatal("ShareSubmission.Unmarshal into a nil receiver must return an error")
	}
}

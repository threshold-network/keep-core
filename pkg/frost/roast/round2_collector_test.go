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

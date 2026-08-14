package roast

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestSelectCoordinator_EmptySet(t *testing.T) {
	_, err := SelectCoordinator([]group.MemberIndex{}, 100, 1)
	if err == nil {
		t.Fatal("expected coordinator selection error")
	}
}

func TestSelectCoordinator_Deterministic(t *testing.T) {
	members := []group.MemberIndex{4, 1, 3, 2}

	first, err := SelectCoordinator(members, 12345, 2)
	if err != nil {
		t.Fatalf("selection failed: [%v]", err)
	}

	for i := 0; i < 20; i++ {
		again, err := SelectCoordinator(members, 12345, 2)
		if err != nil {
			t.Fatalf("selection failed on run [%d]: [%v]", i, err)
		}

		if again != first {
			t.Fatalf(
				"non-deterministic coordinator\nexpected: [%v]\nactual:   [%v]",
				first,
				again,
			)
		}
	}
}

func TestSelectCoordinator_InputOrderIndependent(t *testing.T) {
	left := []group.MemberIndex{1, 2, 3, 4, 5, 6}
	right := []group.MemberIndex{6, 1, 5, 2, 4, 3}

	leftCoordinator, err := SelectCoordinator(left, 333, 4)
	if err != nil {
		t.Fatalf("left selection failed: [%v]", err)
	}

	rightCoordinator, err := SelectCoordinator(right, 333, 4)
	if err != nil {
		t.Fatalf("right selection failed: [%v]", err)
	}

	if leftCoordinator != rightCoordinator {
		t.Fatalf(
			"input order should not matter\nleft:  [%v]\nright: [%v]",
			leftCoordinator,
			rightCoordinator,
		)
	}
}

func TestSelectCoordinator_AffectedByAttemptNumber(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5, 6}
	first, err := SelectCoordinator(members, 777, 1)
	if err != nil {
		t.Fatalf("selection failed: [%v]", err)
	}

	differentObserved := false
	for attempt := uint(2); attempt <= 20; attempt++ {
		candidate, err := SelectCoordinator(members, 777, attempt)
		if err != nil {
			t.Fatalf("selection failed for attempt [%d]: [%v]", attempt, err)
		}

		if candidate != first {
			differentObserved = true
			break
		}
	}

	if !differentObserved {
		t.Fatal("coordinator did not change for any attempt number")
	}
}

func TestSelectCoordinator_AffectedBySeed(t *testing.T) {
	members := []group.MemberIndex{1, 2, 3, 4, 5, 6}
	first, err := SelectCoordinator(members, 1000, 2)
	if err != nil {
		t.Fatalf("selection failed: [%v]", err)
	}

	differentObserved := false
	for seed := int64(1001); seed <= 1030; seed++ {
		candidate, err := SelectCoordinator(members, seed, 2)
		if err != nil {
			t.Fatalf("selection failed for seed [%d]: [%v]", seed, err)
		}

		if candidate != first {
			differentObserved = true
			break
		}
	}

	if !differentObserved {
		t.Fatal("coordinator did not change for any seed")
	}
}

// TestSelectCoordinator_CrossLanguagePinnedVectors pins concrete
// SelectCoordinator outputs so cross-language agreement with the Rust
// engine is enforced by tests on both sides. The Rust port of Go's
// math/rand (pkg/tbtc/signer/src/go_math_rand.rs,
// select_coordinator_matches_known_keep_core_vectors) asserts these
// exact values; the FROST/ROAST liveness path depends on every honest
// node electing the same coordinator for the same attempt.
//
// If this test breaks, the Go selection semantics changed (for
// example a move to math/rand/v2, a different shuffle, or a different
// seed composition). That is a network-fracturing change: it must be
// coordinated with the Rust engine and rolled out as a new versioned
// selection rule, never shipped silently.
func TestSelectCoordinator_CrossLanguagePinnedVectors(t *testing.T) {
	const seed = int64(6879463052285329321)

	vectors := []struct {
		members     []group.MemberIndex
		seed        int64
		attempt     uint
		coordinator group.MemberIndex
	}{
		{[]group.MemberIndex{1, 2}, seed, 1, 2},
		{[]group.MemberIndex{1, 2}, seed, 2, 1},
		{[]group.MemberIndex{1, 2}, seed, 3, 2},
		{[]group.MemberIndex{1, 2, 3}, seed, 1, 3},
		{[]group.MemberIndex{1, 2, 3}, seed, 2, 2},
		{[]group.MemberIndex{1, 2, 3}, seed, 4, 1},
		{[]group.MemberIndex{1, 2, 3, 4, 5, 6}, 333, 4, 4},
	}

	for _, vector := range vectors {
		actual, err := SelectCoordinator(
			vector.members,
			vector.seed,
			vector.attempt,
		)
		if err != nil {
			t.Fatalf(
				"selection failed for members [%v] seed [%d] attempt [%d]: [%v]",
				vector.members,
				vector.seed,
				vector.attempt,
				err,
			)
		}

		if actual != vector.coordinator {
			t.Fatalf(
				"pinned vector drift for members [%v] seed [%d] attempt [%d]\n"+
					"expected: [%v]\nactual:   [%v]",
				vector.members,
				vector.seed,
				vector.attempt,
				vector.coordinator,
				actual,
			)
		}
	}
}

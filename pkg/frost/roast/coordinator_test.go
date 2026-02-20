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

package retry

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
)

type groupMemberRandomizer func(
	[]chain.Address,
	int64,
	uint,
	uint,
) ([]chain.Address, error)

func TestEvaluateRetryParticipantsForSigning_100DifferentOperators(t *testing.T) {
	groupMembers := make([]chain.Address, 100)
	for i := 0; i < 100; i++ {
		groupMembers[i] = chain.Address(fmt.Sprintf("Operator-%d", i))
	}
	assertInvariants(t, EvaluateRetryParticipantsForSigning, groupMembers, int64(123), 0, 51)
}

func TestEvaluateRetryParticipantsForSigning_FewOperators(t *testing.T) {
	groupMembers := make([]chain.Address, 100)
	for i := 0; i < 100; i++ {
		groupMembers[i] = chain.Address(fmt.Sprintf("Operator-%d", i%3))
	}
	assertInvariants(t, EvaluateRetryParticipantsForSigning, groupMembers, int64(456), 0, 51)
}

func TestEvaluateRetryParticipantsForSigning_NotEnoughOperators(t *testing.T) {
	groupMembers := make([]chain.Address, 50)
	for i := 0; i < 50; i++ {
		groupMembers[i] = chain.Address(fmt.Sprintf("Operator-%d", i))
	}
	_, err := EvaluateRetryParticipantsForSigning(groupMembers, int64(123), 0, 51)
	expectation := "asked for too many seats"
	if err == nil {
		t.Fatalf(
			"unexpected error\nexpected: [%s]\nactual:   [%v]",
			fmt.Sprintf("%s...", expectation),
			nil,
		)
	}
	if !strings.HasPrefix(err.Error(), expectation) {
		t.Fatalf(
			"unexpected error\nexpected: [%s]\nactual:   [%s]",
			fmt.Sprintf("%s...", expectation),
			err.Error(),
		)
	}
}

func TestEvaluateRetryParticipantsForKeyGeneration_100DifferentOperators(t *testing.T) {
	groupMembers := make([]chain.Address, 100)
	for i := 0; i < 100; i++ {
		groupMembers[i] = chain.Address(fmt.Sprintf("Operator-%d", i))
	}
	assertInvariants(t, EvaluateRetryParticipantsForKeyGeneration, groupMembers, int64(123), 0, 90)
}

func TestEvaluateRetryParticipantsForKeyGeneration_FewOperators(t *testing.T) {
	groupMembers := make([]chain.Address, 100)
	for i := 0; i < 100; i++ {
		groupMembers[i] = chain.Address(fmt.Sprintf("Operator-%d", i%20))
	}
	// There are 20 unique operators, and any 3 of them can be excluded while
	// still being above the lower bound of 80 since each operator controls 5
	// seats. Thus, there are 20 single exclusions, 20 choose 2 = 190 pairs, and
	// 20 choose 3 = 1140 triplets for a total of 20 + 190 + 1140 = 1350 total
	// exclusions.

	// Single exclusion
	assertInvariants(t, EvaluateRetryParticipantsForKeyGeneration, groupMembers, int64(456), 15, 80)

	// Pair Exclusion
	assertInvariants(t, EvaluateRetryParticipantsForKeyGeneration, groupMembers, int64(456), 170, 80)

	// Triplet Exclusion
	assertInvariants(t, EvaluateRetryParticipantsForKeyGeneration, groupMembers, int64(456), 1000, 80)

	// Too many!
	_, err := EvaluateRetryParticipantsForKeyGeneration(groupMembers, int64(456), 1350, 80)
	expectation := "the retry count [1350] was too large to handle; tried every single, pair, and triplet, but still needed [0] more retries"
	if err.Error() != expectation {
		t.Errorf(
			"unexpected error\nexpected: [%s]\nactual:   [%s]",
			expectation,
			err.Error(),
		)
	}
}

func TestEvaluateRetryParticipantsForKeyGeneration_NotEnoughOperators(t *testing.T) {
	groupMembers := make([]chain.Address, 50)
	for i := 0; i < 50; i++ {
		groupMembers[i] = chain.Address(fmt.Sprintf("Operator-%d", i))
	}
	_, err := EvaluateRetryParticipantsForKeyGeneration(groupMembers, int64(123), 0, 90)
	expectation := "asked for too many seats"
	if err == nil {
		t.Fatalf(
			"unexpected error\nexpected: [%s]\nactual:   [%v]",
			fmt.Sprintf("%s...", expectation),
			nil,
		)
	}
	if !strings.HasPrefix(err.Error(), expectation) {
		t.Fatalf(
			"unexpected error\nexpected: [%s]\nactual:   [%s]",
			fmt.Sprintf("%s...", expectation),
			err.Error(),
		)
	}
}

func isSubset(
	t *testing.T,
	groupMemberRandomizer groupMemberRandomizer,
	groupMembers []chain.Address,
	seed int64,
	retryCount uint,
	retryParticipantsCount uint,
) {
	subset, err := groupMemberRandomizer(groupMembers, seed, retryCount, retryParticipantsCount)
	if err != nil {
		t.Fatalf("unexpected error: [%s]", err)
	}
	memberMap := make(map[chain.Address]struct{})
	for _, operator := range groupMembers {
		memberMap[operator] = struct{}{}
	}
	for _, operator := range subset {
		if _, ok := memberMap[operator]; !ok {
			t.Errorf("Subset member [%s] is not in the operator group.", operator)
		}
	}
}

func isStable(
	t *testing.T,
	groupMemberRandomizer groupMemberRandomizer,
	groupMembers []chain.Address,
	seed int64,
	retryCount uint,
	retryParticipantsCount uint,
) {
	subset, err := groupMemberRandomizer(groupMembers, seed, retryCount, retryParticipantsCount)
	if err != nil {
		t.Fatalf("unexpected error: [%s]", err)
	}
	for i := 0; i < 30; i++ {
		newSubset, err := groupMemberRandomizer(groupMembers, seed, retryCount, retryParticipantsCount)
		if err != nil {
			t.Fatalf("unexpected error: [%s]", err)
		}
		if ok := reflect.DeepEqual(subset, newSubset); !ok {
			t.Errorf(
				"The subsets changed\nexpected: [%v]\nactual:   [%v]",
				subset,
				newSubset,
			)
		}
	}
}

func isLargeEnough(
	t *testing.T,
	groupMemberRandomizer groupMemberRandomizer,
	groupMembers []chain.Address,
	seed int64,
	retryCount uint,
	retryParticipantsCount uint,
) {
	subset, err := groupMemberRandomizer(groupMembers, seed, retryCount, retryParticipantsCount)
	if err != nil {
		t.Fatalf("unexpected error: [%s]", err)
	}
	if len(subset) < int(retryParticipantsCount) {
		t.Errorf(
			"Subset isn't large enough\nexpected: [%d+]\nactual:   [%d]",
			retryParticipantsCount,
			len(subset),
		)
	}
}

// They don't all have to be different, but they shouldn't all be the same!
func affectedBySeed(
	t *testing.T,
	groupMemberRandomizer groupMemberRandomizer,
	groupMembers []chain.Address,
	originalSeed int64,
	retryCount uint,
	retryParticipantsCount uint,
) {
	allTheSame := true
	subset, err := groupMemberRandomizer(groupMembers, originalSeed, retryCount, retryParticipantsCount)
	if err != nil {
		t.Fatalf("unexpected error: [%s]", err)
	}
	for seed := int64(0); seed < 30 && allTheSame; seed++ {
		newSubset, _ := groupMemberRandomizer(groupMembers, seed, retryCount, retryParticipantsCount)
		allTheSame = allTheSame && reflect.DeepEqual(subset, newSubset)
	}
	if allTheSame {
		t.Error("The seed did not affect the subset generation. All subsets were the same.")
	}
}

// They don't all have to be different, but they shouldn't all be the same!
func affectedByRetryCount(
	t *testing.T,
	groupMemberRandomizer groupMemberRandomizer,
	groupMembers []chain.Address,
	seed int64,
	originalRetryCount uint,
	retryParticipantsCount uint,
) {
	allTheSame := true
	subset, err := groupMemberRandomizer(groupMembers, seed, originalRetryCount, retryParticipantsCount)
	if err != nil {
		t.Fatalf("unexpected error: [%s]", err)
	}
	for retryCount := uint(1); retryCount < 30 && allTheSame; retryCount++ {
		newSubset, _ := groupMemberRandomizer(groupMembers, seed, retryCount, retryParticipantsCount)
		allTheSame = allTheSame && reflect.DeepEqual(subset, newSubset)
	}
	if allTheSame {
		t.Error("The seed did not affect the subset generation. All subsets were the same.")
	}
}

func assertInvariants(
	t *testing.T,
	groupMemberRandomizer groupMemberRandomizer,
	groupMembers []chain.Address,
	seed int64,
	retryCount uint,
	retryParticipantsCount uint,
) {
	isSubset(t, groupMemberRandomizer, groupMembers, seed, retryCount, retryParticipantsCount)
	isStable(t, groupMemberRandomizer, groupMembers, seed, retryCount, retryParticipantsCount)
	isLargeEnough(t, groupMemberRandomizer, groupMembers, seed, retryCount, retryParticipantsCount)
	affectedBySeed(t, groupMemberRandomizer, groupMembers, seed, retryCount, retryParticipantsCount)
	affectedByRetryCount(t, groupMemberRandomizer, groupMembers, seed, retryCount, retryParticipantsCount)
}

package signing

import (
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestCloneAttempt(t *testing.T) {
	original := &Attempt{
		Number:                 3,
		CoordinatorMemberIndex: 7,
		IncludedMembersIndexes: []group.MemberIndex{1, 2, 3, 7},
		ExcludedMembersIndexes: []group.MemberIndex{4, 5, 6, 8},
	}

	cloned := cloneAttempt(original)
	if !reflect.DeepEqual(original, cloned) {
		t.Fatalf("unexpected clone\nexpected: [%+v]\nactual:   [%+v]", original, cloned)
	}

	if &original.IncludedMembersIndexes[0] == &cloned.IncludedMembersIndexes[0] {
		t.Fatal("included members slice should be copied")
	}

	if &original.ExcludedMembersIndexes[0] == &cloned.ExcludedMembersIndexes[0] {
		t.Fatal("excluded members slice should be copied")
	}

	cloned.IncludedMembersIndexes[0] = 99
	if original.IncludedMembersIndexes[0] == cloned.IncludedMembersIndexes[0] {
		t.Fatal("mutating clone should not mutate original")
	}
}

func TestCloneAttempt_Nil(t *testing.T) {
	if cloneAttempt(nil) != nil {
		t.Fatal("expected nil clone")
	}
}

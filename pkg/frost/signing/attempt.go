package signing

import "github.com/keep-network/keep-core/pkg/protocol/group"

// Attempt describes runtime context for a signing attempt coordinated by ROAST.
type Attempt struct {
	// Number is the 1-based signing attempt counter for the same message.
	Number uint
	// CoordinatorMemberIndex is the member coordinating this attempt.
	CoordinatorMemberIndex group.MemberIndex
	// IncludedMembersIndexes are members participating in this attempt.
	IncludedMembersIndexes []group.MemberIndex
	// ExcludedMembersIndexes are members excluded from this attempt.
	ExcludedMembersIndexes []group.MemberIndex
}

func cloneAttempt(attempt *Attempt) *Attempt {
	if attempt == nil {
		return nil
	}

	return &Attempt{
		Number:                 attempt.Number,
		CoordinatorMemberIndex: attempt.CoordinatorMemberIndex,
		IncludedMembersIndexes: append(
			[]group.MemberIndex{},
			attempt.IncludedMembersIndexes...,
		),
		ExcludedMembersIndexes: append(
			[]group.MemberIndex{},
			attempt.ExcludedMembersIndexes...,
		),
	}
}

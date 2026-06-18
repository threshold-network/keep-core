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
	// ExcludedMembersIndexes are members NOT participating in this attempt --
	// the permanently-excluded members together with the transiently-parked
	// ones (TransientlyParkedMembersIndexes is a subset).
	ExcludedMembersIndexes []group.MemberIndex
	// TransientlyParkedMembersIndexes are the members the prior ROAST transition
	// parked for THIS attempt only (silence/overflow): they do not participate
	// now but the attempt after this one reinstates them. RFC-21 Phase 7.3
	// PR2b-1b carries this so a one-attempt park does not become permanent.
	// Empty for the legacy/attempt-zero shape.
	TransientlyParkedMembersIndexes []group.MemberIndex
}

func cloneAttempt(attempt *Attempt) *Attempt {
	if attempt == nil {
		return nil
	}

	// Preserve nil-ness for the optional parked set (nil when there is no
	// parking) so a clone compares equal to an attempt that never set it.
	var transientlyParked []group.MemberIndex
	if attempt.TransientlyParkedMembersIndexes != nil {
		transientlyParked = append(
			[]group.MemberIndex{},
			attempt.TransientlyParkedMembersIndexes...,
		)
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
		TransientlyParkedMembersIndexes: transientlyParked,
	}
}

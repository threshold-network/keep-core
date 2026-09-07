package gjkr

import (
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dishonestThreshold := 2
	groupSize := 5

	committingMembers, err := initializeCommittingMembersGroup(dishonestThreshold, groupSize)
	if err != nil {
		t.Fatalf("group initialization failed [%s]", err)
	}

	var sharesMessages []*PeerSharesMessage
	var commitmentsMessages []*MemberCommitmentsMessage
	for _, member := range committingMembers {
		sharesMessage, commitmentsMessage, err := member.CalculateMembersSharesAndCommitments()
		if err != nil {
			t.Fatalf("shares and commitments calculation failed [%s]", err)
		}
		sharesMessages = append(sharesMessages, sharesMessage)
		commitmentsMessages = append(commitmentsMessages, commitmentsMessage)

	}

	var commitmentVerifyingMembers []*CommitmentsVerifyingMember
	for _, cm := range committingMembers {
		commitmentVerifyingMembers = append(commitmentVerifyingMembers,
			cm.InitializeCommitmentsVerification())
	}

	for _, member := range commitmentVerifyingMembers {
		accusedSecretSharesMessage, err := member.VerifyReceivedSharesAndCommitmentsMessages(
			filterPeerSharesMessage(sharesMessages, member.ID),
			filterMemberCommitmentsMessages(commitmentsMessages, member.ID),
		)
		if err != nil {
			t.Fatalf("shares and commitments verification failed [%s]", err)
		}

		if len(accusedSecretSharesMessage.accusedMembersKeys) > 0 {
			t.Fatalf("\nexpected: 0 accusations\nactual:   %d\n",
				len(accusedSecretSharesMessage.accusedMembersKeys),
			)
		}
	}

	var qualifiedMembers []*QualifiedMember
	for _, cvm := range commitmentVerifyingMembers {
		qualifiedMembers = append(qualifiedMembers,
			cvm.InitializeSharesJustification().InitializeQualified())
	}

	for _, member := range qualifiedMembers {
		member.CombineMemberShares()
	}

	var sharingMembers []*SharingMember
	for _, qm := range qualifiedMembers {
		sharingMembers = append(sharingMembers, qm.InitializeSharing())
	}

	for _, member := range sharingMembers {
		if len(member.receivedQualifiedSharesS) != groupSize-1 {
			t.Fatalf("\nexpected: %d received shares S\nactual:   %d\n",
				groupSize-1,
				len(member.receivedQualifiedSharesS),
			)
		}
		member.CombineMemberShares()
	}

	publicKeySharePointsMessages := make([]*MemberPublicKeySharePointsMessage, groupSize)
	for i, member := range sharingMembers {
		publicKeySharePointsMessages[i] = member.CalculatePublicKeySharePoints()
	}

	for _, member := range sharingMembers {
		accusedPointsMessage, err := member.VerifyPublicKeySharePoints(
			filterMemberPublicKeySharePointsMessages(publicKeySharePointsMessages, member.ID),
		)
		if err != nil {
			t.Fatalf("public coefficients verification failed [%s]", err)
		}
		if len(accusedPointsMessage.accusedMembersKeys) > 0 {
			t.Fatalf("\nexpected: 0 accusations\nactual:   %d\n",
				len(accusedPointsMessage.accusedMembersKeys),
			)
		}
	}

	var combiningMembers []*CombiningMember
	for _, sm := range sharingMembers {
		combiningMembers = append(combiningMembers,
			sm.InitializePointsJustification().InitializeRevealing().
				InitializeReconstruction().InitializeCombining())
	}

	for _, member := range combiningMembers {
		member.CombineGroupPublicKey()
	}

}

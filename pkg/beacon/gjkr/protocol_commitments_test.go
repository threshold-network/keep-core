package gjkr

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestCalculateSharesAndCommitments(t *testing.T) {
	dishonestThreshold := 2
	groupSize := 5

	members, err := initializeCommittingMembersGroup(dishonestThreshold, groupSize)
	if err != nil {
		t.Fatalf("group initialization failed [%s]", err)
	}

	member := members[0]
	sharesMessage, commitmentsMessage, err := member.CalculateMembersSharesAndCommitments()
	if err != nil {
		t.Fatalf("shares and commitments calculation failed [%s]", err)
	}

	// polynomial is of degree dishonestThreshold, so we have
	// dishonestThreshold+1 coefficients, including the constant coefficient
	if len(member.secretCoefficients) != (dishonestThreshold + 1) {
		t.Fatalf("\nexpected: %v secret coefficients\nactual:   %v\n",
			dishonestThreshold+1,
			len(member.secretCoefficients),
		)
	}
	if len(sharesMessage.shares) != (groupSize - 1) {
		t.Fatalf("\nexpected: %v shares in message\nactual:   %v\n",
			groupSize-1,
			len(sharesMessage.shares),
		)
	}

	if len(commitmentsMessage.commitments) != (dishonestThreshold + 1) {
		t.Fatalf("\nexpected: %v calculated commitments\nactual:   %v\n",
			dishonestThreshold+1,
			len(commitmentsMessage.commitments),
		)
	}
}

func TestStoreSharesMessageForEvidence(t *testing.T) {
	groupSize := 2

	members, err := initializeCommittingMembersGroup(0, groupSize)
	if err != nil {
		t.Fatalf("group initialization failed [%s]", err)
	}

	member1 := members[0]
	member2 := members[1]

	sharesMsg1, commitmentsMsg1, err := member1.CalculateMembersSharesAndCommitments()
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := member2.CalculateMembersSharesAndCommitments(); err != nil {
		t.Fatal(err)
	}

	verifyingMember2 := member2.InitializeCommitmentsVerification()

	if _, err := verifyingMember2.VerifyReceivedSharesAndCommitmentsMessages(
		[]*PeerSharesMessage{sharesMsg1},
		[]*MemberCommitmentsMessage{commitmentsMsg1},
	); err != nil {
		t.Fatal(err)
	}

	evidenceMsg := verifyingMember2.evidenceLog.peerSharesMessage(member1.ID)

	if !reflect.DeepEqual(sharesMsg1, evidenceMsg) {
		t.Fatalf(
			"unexpected message in evidence log\nexpected: %v\n actual:   %v",
			sharesMsg1,
			evidenceMsg,
		)
	}
}

func TestSharesAndCommitmentsCalculationAndVerification(t *testing.T) {
	dishonestThreshold := 1
	groupSize := 3

	members, err := initializeCommittingMembersGroup(dishonestThreshold, groupSize)
	if err != nil {
		t.Fatalf("group initialization failed [%s]", err)
	}

	member1 := members[0]
	member2 := members[1]
	member3 := members[2]

	verifyingMemberID := member3.ID
	verifyingMemberKeys := member3.symmetricKeys

	var tests = map[string]struct {
		modifyPeerSharesMessage  func(messages map[group.MemberIndex]*PeerSharesMessage) error
		modifyCommitmentsMessage func(messages map[group.MemberIndex]*MemberCommitmentsMessage)
		expectedAccusedIDs       []group.MemberIndex
	}{
		"no accusations": {
			expectedAccusedIDs: []group.MemberIndex{},
		},
		"invalid S share": {
			modifyPeerSharesMessage: func(messages map[group.MemberIndex]*PeerSharesMessage) error {
				return alterPeerSharesMessage(
					messages[member2.ID],
					verifyingMemberID,
					verifyingMemberKeys[member2.ID],
					true,
					false,
				)
			},
			expectedAccusedIDs: []group.MemberIndex{member2.ID},
		},
		"invalid T share": {
			modifyPeerSharesMessage: func(messages map[group.MemberIndex]*PeerSharesMessage) error {
				return alterPeerSharesMessage(
					messages[member1.ID],
					verifyingMemberID,
					verifyingMemberKeys[member1.ID],
					false,
					true,
				)
			},
			expectedAccusedIDs: []group.MemberIndex{member1.ID},
		},
		"invalid commitment": {
			modifyCommitmentsMessage: func(messages map[group.MemberIndex]*MemberCommitmentsMessage) {
				message := messages[member2.ID]
				message.commitments[0] = new(bn256.G1).ScalarMult(
					message.commitments[0],
					big.NewInt(3),
				)
			},
			expectedAccusedIDs: []group.MemberIndex{member2.ID},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			shareMessages := make(map[group.MemberIndex]*PeerSharesMessage)
			commitmentMessages := make(map[group.MemberIndex]*MemberCommitmentsMessage)

			for _, member := range members {
				shares, commitments, err := member.CalculateMembersSharesAndCommitments()
				if err != nil {
					t.Fatal(err)
				}

				shareMessages[member.ID] = shares
				commitmentMessages[member.ID] = commitments
			}

			if test.modifyPeerSharesMessage != nil {
				if err = test.modifyPeerSharesMessage(shareMessages); err != nil {
					t.Fatal(err)
				}

			}

			if test.modifyCommitmentsMessage != nil {
				test.modifyCommitmentsMessage(commitmentMessages)
			}

			verifyingMember := member3.InitializeCommitmentsVerification()

			accusationMessage, err := verifyingMember.VerifyReceivedSharesAndCommitmentsMessages(
				[]*PeerSharesMessage{
					shareMessages[member1.ID],
					shareMessages[member2.ID],
				},
				[]*MemberCommitmentsMessage{
					commitmentMessages[member1.ID],
					commitmentMessages[member2.ID],
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			assertAccusedMembers(
				test.expectedAccusedIDs,
				verifyingMember,
				accusationMessage,
				t,
			)

			assertValidSharesAndCommitments(
				test.expectedAccusedIDs,
				verifyingMember,
				groupSize,
				t,
			)
		})
	}
}

func alterPeerSharesMessage(
	message *PeerSharesMessage,
	receiverID group.MemberIndex,
	symmetricKey ephemeral.SymmetricKey,
	alterS bool,
	alterT bool,
) error {
	oldShareS, err := message.decryptShareS(receiverID, symmetricKey)
	if err != nil {
		return err
	}

	oldShareT, err := message.decryptShareT(receiverID, symmetricKey)
	if err != nil {
		return err
	}

	var newShareS = oldShareS
	var newShareT = oldShareT

	if alterS {
		newShareS = testutils.NewRandInt(oldShareS, bn256.Order)
	}
	if alterT {
		newShareT = testutils.NewRandInt(oldShareT, bn256.Order)
	}

	err = message.addShares(receiverID, newShareS, newShareT, symmetricKey)
	if err != nil {
		return err
	}

	return nil
}

func assertAccusedMembers(
	expectedAccusedIDs []group.MemberIndex,
	verifyingMember *CommitmentsVerifyingMember,
	accusationMessage *SecretSharesAccusationsMessage,
	t *testing.T,
) {
	expectedAccusedMembersKeys := make(map[group.MemberIndex]*ephemeral.PrivateKey)
	for _, id := range expectedAccusedIDs {
		expectedAccusedMembersKeys[id] = verifyingMember.ephemeralKeyPairs[id].PrivateKey
	}

	if !reflect.DeepEqual(accusationMessage.accusedMembersKeys, expectedAccusedMembersKeys) {
		t.Errorf("incorrect accused members IDs\nexpected: %v\nactual:   %v\n",
			expectedAccusedMembersKeys,
			accusationMessage.accusedMembersKeys,
		)
	}
}

func assertValidSharesAndCommitments(
	expectedAccusedIDs []group.MemberIndex,
	verifyingMember *CommitmentsVerifyingMember,
	groupSize int,
	t *testing.T,
) {
	expectedReceivedSharesLength := groupSize - 1 - len(expectedAccusedIDs)
	if len(verifyingMember.receivedQualifiedSharesS) != expectedReceivedSharesLength {
		t.Errorf("\nexpected: %v received shares S\nactual:   %v\n",
			expectedReceivedSharesLength,
			len(verifyingMember.receivedQualifiedSharesS),
		)
	}
	if len(verifyingMember.receivedPeerCommitments) != groupSize-1 {
		t.Errorf("\nexpected: %v received commitments\nactual:   %v\n",
			expectedReceivedSharesLength,
			len(verifyingMember.receivedPeerCommitments),
		)
	}
}

func TestGeneratePolynomial(t *testing.T) {
	degree := 3

	coefficients, err := generatePolynomial(degree)
	if err != nil {
		t.Fatalf("unexpected error [%s]", err)
	}

	if len(coefficients) != degree+1 {
		t.Fatalf("\nexpected: %d coefficients\nactual:   %d\n",
			degree+1,
			len(coefficients),
		)
	}
	for _, c := range coefficients {
		if c.Sign() <= 0 || c.Cmp(bn256.Order) >= 0 {
			t.Fatalf("coefficient out of range\nexpected: 0 < value < %d\nactual:   %v\n",
				bn256.Order,
				c,
			)
		}
	}
}

func initializeCommittingMembersGroup(dishonestThreshold, groupSize int) (
	[]*CommittingMember,
	error,
) {
	symmetricKeyMembers, err := generateGroupWithEphemeralKeys(
		dishonestThreshold,
		groupSize,
	)
	if err != nil {
		return nil, fmt.Errorf("group initialization failed [%v]", err)
	}

	var members []*CommittingMember
	for _, member := range symmetricKeyMembers {
		committingMember := member.InitializeCommitting()
		members = append(members, committingMember)
	}

	return members, nil
}

func initializeCommitmentsVerifiyingMembersGroup(dishonestThreshold, groupSize int) (
	[]*CommitmentsVerifyingMember,
	error,
) {
	committingMembers, err := initializeCommittingMembersGroup(
		dishonestThreshold,
		groupSize,
	)
	if err != nil {
		return nil, fmt.Errorf("group initialization failed [%v]", err)
	}

	var members []*CommitmentsVerifyingMember
	for _, member := range committingMembers {
		members = append(members, member.InitializeCommitmentsVerification())
	}

	return members, nil
}

func filterPeerSharesMessage(
	messages []*PeerSharesMessage,
	receiverID group.MemberIndex,
) []*PeerSharesMessage {
	var result []*PeerSharesMessage
	for _, msg := range messages {
		if msg.senderID != receiverID {
			result = append(result, msg)
		}
	}
	return result
}

func filterMemberCommitmentsMessages(
	messages []*MemberCommitmentsMessage,
	receiverID group.MemberIndex,
) []*MemberCommitmentsMessage {
	var result []*MemberCommitmentsMessage
	for _, msg := range messages {
		if msg.senderID != receiverID {
			result = append(result, msg)
		}
	}
	return result
}

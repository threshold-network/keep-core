package announcer

import (
	"context"
	"math/big"
	"reflect"
	"sync"
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestAnnouncementMessage_MarshalingRoundtrip(t *testing.T) {
	msg := &announcementMessage{
		senderID:   group.MemberIndex(38),
		protocolID: "protocol",
		sessionID:  "session",
	}
	unmarshaled := &announcementMessage{}

	err := pbutils.RoundTrip(msg, unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, unmarshaled) {
		t.Fatalf("unexpected content of unmarshaled message")
	}
}

func TestFuzzAnnouncementMessage_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			senderID   group.MemberIndex
			protocolID string
			sessionID  string
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&senderID)
		f.Fuzz(&protocolID)
		f.Fuzz(&sessionID)

		msg := &announcementMessage{
			senderID:   senderID,
			protocolID: protocolID,
			sessionID:  sessionID,
		}

		if err := pbutils.RoundTrip(msg, &announcementMessage{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzAnnouncementMessage_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&announcementMessage{})
}

func TestAnnouncer(t *testing.T) {
	protocolID := "protocol-test"
	groupSize := 5
	honestThreshold := 3

	operatorPrivateKey, operatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	localChain := local_v1.ConnectWithKey(
		groupSize,
		honestThreshold,
		operatorPrivateKey,
	)

	operatorAddress, err := localChain.Signing().PublicKeyToAddress(
		operatorPublicKey,
	)
	if err != nil {
		t.Fatal(err)
	}

	var operators []chain.Address
	for i := 0; i < groupSize; i++ {
		operators = append(operators, operatorAddress)
	}

	localProvider := local.ConnectWithKey(operatorPublicKey)

	type memberResult struct {
		memberIndex         group.MemberIndex
		readyMembersIndexes []group.MemberIndex
	}

	type memberError struct {
		memberIndex group.MemberIndex
		err         error
	}

	var tests = map[string]struct {
		message                  *big.Int
		announcingMembersIndexes []group.MemberIndex
		expectedResults          map[group.MemberIndex][]group.MemberIndex
	}{
		"all members members announced readiness": {
			message:                  big.NewInt(100),
			announcingMembersIndexes: []group.MemberIndex{1, 2, 3, 4, 5},
			expectedResults: map[group.MemberIndex][]group.MemberIndex{
				1: {1, 2, 3, 4, 5},
				2: {1, 2, 3, 4, 5},
				3: {1, 2, 3, 4, 5},
				4: {1, 2, 3, 4, 5},
				5: {1, 2, 3, 4, 5},
			},
		},
		"part of members announced readiness": {
			message:                  big.NewInt(200),
			announcingMembersIndexes: []group.MemberIndex{1, 3, 5},
			expectedResults: map[group.MemberIndex][]group.MemberIndex{
				1: {1, 3, 5},
				3: {1, 3, 5},
				5: {1, 3, 5},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			broadcastChannel, err := localProvider.BroadcastChannelFor(
				test.message.Text(16),
			)
			if err != nil {
				t.Fatal(err)
			}

			membershipValidator := group.NewMembershipValidator(
				&testutils.MockLogger{},
				operators,
				localChain.Signing(),
			)

			RegisterUnmarshaller(broadcastChannel)

			announcer := New(
				protocolID,
				broadcastChannel,
				membershipValidator,
			)

			resultsChan := make(
				chan *memberResult,
				len(test.announcingMembersIndexes),
			)
			errorsChan := make(
				chan *memberError,
				len(test.announcingMembersIndexes),
			)

			wg := sync.WaitGroup{}
			wg.Add(len(test.announcingMembersIndexes))

			for _, announcingMemberIndex := range test.announcingMembersIndexes {
				go func(memberIndex group.MemberIndex) {
					defer wg.Done()

					ctx, cancelCtx := context.WithTimeout(
						context.Background(),
						3*local.RetransmissionTick,
					)
					defer cancelCtx()

					readyMembersIndexes, err := announcer.Announce(
						ctx,
						memberIndex,
						"session-test",
					)
					if err != nil {
						errorsChan <- &memberError{memberIndex, err}
						return
					}

					resultsChan <- &memberResult{memberIndex, readyMembersIndexes}
				}(announcingMemberIndex)
			}

			wg.Wait()

			close(resultsChan)
			results := make(map[group.MemberIndex][]group.MemberIndex)
			for r := range resultsChan {
				results[r.memberIndex] = r.readyMembersIndexes
			}

			close(errorsChan)
			errors := make(map[group.MemberIndex]error)
			for e := range errorsChan {
				errors[e.memberIndex] = e.err
			}

			testutils.AssertIntsEqual(
				t,
				"errors count",
				0,
				len(errors),
			)

			if !reflect.DeepEqual(test.expectedResults, results) {
				t.Errorf(
					"unexpected results\n"+
						"expected: [%v]\n"+
						"actual:   [%v]",
					test.expectedResults,
					results,
				)
			}
		})
	}
}

func TestUnreadyMembers(t *testing.T) {
	tests := map[string]struct {
		readyMembers []group.MemberIndex
		groupSize    int
		expected     []group.MemberIndex
	}{
		"all members are ready": {
			readyMembers: []group.MemberIndex{1, 2, 3, 4, 5},
			groupSize:    5,
			expected:     []group.MemberIndex{},
		},
		"some members are not ready": {
			readyMembers: []group.MemberIndex{1, 3, 5},
			groupSize:    5,
			expected:     []group.MemberIndex{2, 4},
		},
		"no members are ready": {
			readyMembers: []group.MemberIndex{},
			groupSize:    5,
			expected:     []group.MemberIndex{1, 2, 3, 4, 5},
		},
		"group size is zero": {
			readyMembers: []group.MemberIndex{},
			groupSize:    0,
			expected:     []group.MemberIndex{},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := UnreadyMembers(test.readyMembers, test.groupSize)

			if !reflect.DeepEqual(test.expected, result) {
				t.Errorf(
					"unexpected result\nexpected: %v\nactual:   %v",
					test.expected,
					result,
				)
			}
		})
	}
}

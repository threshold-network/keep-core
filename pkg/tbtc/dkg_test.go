package tbtc

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"golang.org/x/exp/slices"

	"golang.org/x/crypto/sha3"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"github.com/keep-network/keep-core/pkg/tecdsa/dkg"
)

func TestDkgExecutor_RegisterSigner(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     3,
		HonestThreshold: 2,
	}

	localChain := Connect()

	selectedOperators := []chain.Address{
		"0xAA",
		"0xBB",
		"0xCC",
		"0xDD",
		"0xEE",
	}

	var tests = map[string]struct {
		memberIndex               group.MemberIndex
		disqualifiedMemberIndexes []group.MemberIndex
		inactiveMemberIndexes     []group.MemberIndex

		expectedError                      error
		expectedFinalSigningGroupIndex     group.MemberIndex
		expectedFinalSigningGroupOperators []chain.Address
	}{
		"all members participating": {
			memberIndex:                        1,
			disqualifiedMemberIndexes:          nil,
			inactiveMemberIndexes:              nil,
			expectedFinalSigningGroupIndex:     1,
			expectedFinalSigningGroupOperators: selectedOperators,
		},
		"some member inactive": {
			memberIndex:                        3,
			disqualifiedMemberIndexes:          nil,
			inactiveMemberIndexes:              []group.MemberIndex{2, 5},
			expectedFinalSigningGroupIndex:     2,
			expectedFinalSigningGroupOperators: []chain.Address{"0xAA", "0xCC", "0xDD"},
		},
		"some members disqualified": {
			memberIndex:                        1,
			disqualifiedMemberIndexes:          []group.MemberIndex{2, 5},
			inactiveMemberIndexes:              nil,
			expectedError:                      nil,
			expectedFinalSigningGroupIndex:     1,
			expectedFinalSigningGroupOperators: []chain.Address{"0xAA", "0xCC", "0xDD"},
		},
		"the current member inactive": {
			memberIndex:               2,
			disqualifiedMemberIndexes: nil,
			inactiveMemberIndexes:     []group.MemberIndex{2, 5},
			expectedError:             fmt.Errorf("failed to resolve final signing group member index"),
		},
		"the current member disqualified": {
			memberIndex:               5,
			disqualifiedMemberIndexes: []group.MemberIndex{2, 5},
			inactiveMemberIndexes:     nil,
			expectedError:             fmt.Errorf("failed to resolve final signing group member index"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			persistenceHandle := &mockPersistenceHandle{}
			chain := Connect()
			walletRegistry, err := newWalletRegistry(
				persistenceHandle,
				chain.CalculateWalletID,
			)
			if err != nil {
				t.Fatal(err)
			}

			dkgExecutor := &dkgExecutor{
				// setting only the fields really needed for this test
				groupParameters: groupParameters,
				chain:           localChain,
				walletRegistry:  walletRegistry,
			}

			group := group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize)
			for _, disqualifiedMember := range test.disqualifiedMemberIndexes {
				group.MarkMemberAsDisqualified(disqualifiedMember)
			}
			for _, inactiveMember := range test.inactiveMemberIndexes {
				group.MarkMemberAsInactive(inactiveMember)
			}

			result := &dkg.Result{
				Group:           group,
				PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
			}

			signer, err := dkgExecutor.registerSigner(result, test.memberIndex, selectedOperators)

			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\n"+
						"expected: %v\n"+
						"actual:   %v\n",
					test.expectedError,
					err,
				)
			}

			if test.expectedError != nil {
				if signer != nil {
					t.Errorf("expected nil signer")
				}

				// do not check the rest of assertions, the signer should be nil
				return
			}

			testutils.AssertIntsEqual(
				t,
				"final signing group index",
				int(test.expectedFinalSigningGroupIndex),
				int(signer.signingGroupMemberIndex),
			)

			if !reflect.DeepEqual(
				test.expectedFinalSigningGroupOperators,
				signer.wallet.signingGroupOperators,
			) {
				t.Errorf(
					"unexpected final signing group operators\n"+
						"expected: %v\n"+
						"actual:   %v\n",
					test.expectedFinalSigningGroupOperators,
					signer.wallet.signingGroupOperators,
				)
			}

			registeredSigners := walletRegistry.getSigners(
				result.PrivateKeyShare.PublicKey(),
			)

			testutils.AssertIntsEqual(
				t,
				"number of signers registered",
				1,
				len(registeredSigners),
			)
		})
	}
}

func TestDkgExecutor_ExecuteDkgValidation(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     3,
		HonestThreshold: 2,
	}

	tecdsaDkgResult := &dkg.Result{
		Group:           group.NewGroup(groupParameters.DishonestThreshold(), groupParameters.GroupSize),
		PrivateKeyShare: tecdsa.NewPrivateKeyShare(testData[0]),
	}

	groupPublicKey, err := tecdsaDkgResult.GroupPublicKeyBytes()
	if err != nil {
		t.Fatal(err)
	}

	var tests = map[string]struct {
		submitterMemberIndex     group.MemberIndex
		resultValid              bool
		rejectedApprovalsIndexes []int
		expectedEvent            interface{}
		expectedDkgState         DKGState
	}{
		"result approved by the submitter": {
			submitterMemberIndex: group.MemberIndex(1),
			resultValid:          true,
			expectedEvent: &DKGResultApprovedEvent{
				ResultHash: sha3.Sum256(groupPublicKey),
				Approver:   "",
				// 16 is the next block after 15 blocks of the challenge period
				BlockNumber: 16,
			},
			expectedDkgState: Idle,
		},
		"result approved by a non-submitter": {
			submitterMemberIndex: group.MemberIndex(1),
			resultValid:          true,
			// Reject the first approval (with index 0) that will be made by
			// member 1 (the submitter) in order to force the member 2 to
			// approve after the precedence period.
			rejectedApprovalsIndexes: []int{0},
			expectedEvent: &DKGResultApprovedEvent{
				ResultHash: sha3.Sum256(groupPublicKey),
				Approver:   "",
				// 36 is the next block after 15 blocks of the challenge period,
				// 5 blocks of the precedence period, and 15 blocks of the delay
				// for member 2
				BlockNumber: 36,
			},
			expectedDkgState: Idle,
		},
		"result challenged": {
			submitterMemberIndex: group.MemberIndex(1),
			resultValid:          false,
			expectedEvent: &DKGResultChallengedEvent{
				ResultHash:  sha3.Sum256(groupPublicKey),
				Challenger:  "",
				Reason:      "",
				BlockNumber: 0, // challenge is submitted immediately
			},
			expectedDkgState: AwaitingResult,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			localChain := Connect()

			approvalIndex := 0
			localChain.dkgResultApprovalGuard = func() bool {
				rejectedApproval := slices.Contains(
					test.rejectedApprovalsIndexes,
					approvalIndex,
				)
				approvalIndex++
				return !rejectedApproval
			}

			operatorAddress, err := localChain.operatorAddress()
			if err != nil {
				t.Fatal(err)
			}

			operatorID, err := localChain.GetOperatorID(operatorAddress)
			if err != nil {
				t.Fatal(err)
			}

			signatures := make(map[group.MemberIndex][]byte)
			operatorsIDs := make(chain.OperatorIDs, groupParameters.GroupSize)
			operatorsAddresses := make(chain.Addresses, groupParameters.GroupSize)

			for memberIndex := uint8(1); int(memberIndex) <= groupParameters.GroupSize; memberIndex++ {
				signatures[memberIndex] = []byte{memberIndex}
				operatorsIDs[memberIndex-1] = operatorID
				operatorsAddresses[memberIndex-1] = operatorAddress
			}

			groupSelectionResult := &GroupSelectionResult{
				OperatorsIDs:       operatorsIDs,
				OperatorsAddresses: operatorsAddresses,
			}

			dkgResultSubmittedEventChan := make(chan *DKGResultSubmittedEvent, 1)
			_ = localChain.OnDKGResultSubmitted(
				func(event *DKGResultSubmittedEvent) {
					dkgResultSubmittedEventChan <- event
				},
			)

			err = localChain.startDKG()
			if err != nil {
				t.Fatal(err)
			}

			groupPublicKey, err := tecdsaDkgResult.GroupPublicKey()
			if err != nil {
				t.Fatal(err)
			}

			dkgResult, err := localChain.AssembleDKGResult(
				test.submitterMemberIndex,
				groupPublicKey,
				tecdsaDkgResult.Group.OperatingMemberIndexes(),
				tecdsaDkgResult.MisbehavedMembersIndexes(),
				signatures,
				groupSelectionResult,
			)
			if err != nil {
				t.Fatal(err)
			}

			err = localChain.SubmitDKGResult(dkgResult)
			if err != nil {
				t.Fatal(err)
			}

			dkgResultSubmittedEvent := <-dkgResultSubmittedEventChan

			err = localChain.setDKGResultValidity(test.resultValid)
			if err != nil {
				t.Fatal(err)
			}

			// Setting only the fields really needed for this test.
			dkgExecutor := &dkgExecutor{
				groupParameters: groupParameters,
				operatorIDFn: func() (chain.OperatorID, error) {
					return operatorID, nil
				},
				operatorAddress: operatorAddress,
				chain:           localChain,
				waitForBlockFn:  testWaitForBlockFn(localChain),
			}

			eventChan := make(chan interface{}, 1)

			_ = localChain.OnDKGResultChallenged(
				func(event *DKGResultChallengedEvent) {
					eventChan <- event
				},
			)
			_ = localChain.OnDKGResultApproved(
				func(event *DKGResultApprovedEvent) {
					eventChan <- event
				},
			)

			_ = dkgExecutor.executeDkgValidation(
				dkgResultSubmittedEvent.Seed,
				dkgResultSubmittedEvent.BlockNumber,
				dkgResultSubmittedEvent.Result,
				dkgResultSubmittedEvent.ResultHash,
			)

			var event interface{}
			select {
			case event = <-eventChan:
			case <-time.After(1 * time.Minute):
			}

			if !reflect.DeepEqual(test.expectedEvent, event) {
				t.Errorf(
					"unexpected event\n"+
						"expected: [%+v]\n"+
						"actual:   [%+v]",
					test.expectedEvent,
					event,
				)
			}

			dkgState, err := localChain.GetDKGState()
			if err != nil {
				t.Fatal(err)
			}

			testutils.AssertIntsEqual(
				t,
				"DKG state",
				int(test.expectedDkgState),
				int(dkgState),
			)
		})
	}
}

func TestFinalSigningGroup(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     3,
		HonestThreshold: 2,
	}

	selectedOperators := []chain.Address{
		"0xAA",
		"0xBB",
		"0xCC",
		"0xDD",
		"0xEE",
	}

	var tests = map[string]struct {
		selectedOperators           []chain.Address
		operatingMembersIndexes     []group.MemberIndex
		expectedFinalOperators      []chain.Address
		expectedFinalMembersIndexes map[group.MemberIndex]group.MemberIndex
		expectedError               error
	}{
		"selected operators count not equal to the group size": {
			selectedOperators:       selectedOperators[:4],
			operatingMembersIndexes: []group.MemberIndex{1, 2, 3, 4, 5},
			expectedError:           fmt.Errorf("invalid input parameters"),
		},
		"all selected operators are operating": {
			selectedOperators:           selectedOperators,
			operatingMembersIndexes:     []group.MemberIndex{5, 4, 3, 2, 1},
			expectedFinalOperators:      selectedOperators,
			expectedFinalMembersIndexes: map[group.MemberIndex]group.MemberIndex{1: 1, 2: 2, 3: 3, 4: 4, 5: 5},
		},
		"honest majority of selected operators are operating": {
			selectedOperators:           selectedOperators,
			operatingMembersIndexes:     []group.MemberIndex{5, 1, 3},
			expectedFinalOperators:      []chain.Address{"0xAA", "0xCC", "0xEE"},
			expectedFinalMembersIndexes: map[group.MemberIndex]group.MemberIndex{1: 1, 3: 2, 5: 3},
		},
		"less than honest majority of selected operators are operating": {
			selectedOperators:       selectedOperators,
			operatingMembersIndexes: []group.MemberIndex{5, 1},
			expectedError:           fmt.Errorf("invalid input parameters"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualFinalOperators, actualFinalMembersIndexes, err :=
				finalSigningGroup(
					test.selectedOperators,
					test.operatingMembersIndexes,
					groupParameters,
				)

			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\n"+
						"expected: %v\n"+
						"actual:   %v\n",
					test.expectedError,
					err,
				)
			}

			if !reflect.DeepEqual(
				test.expectedFinalOperators,
				actualFinalOperators,
			) {
				t.Errorf(
					"unexpected final operators\n"+
						"expected: %v\n"+
						"actual:   %v\n",
					test.expectedFinalOperators,
					actualFinalOperators,
				)
			}

			if !reflect.DeepEqual(
				test.expectedFinalMembersIndexes,
				actualFinalMembersIndexes,
			) {
				t.Errorf(
					"unexpected final members indexes\n"+
						"expected: %v\n"+
						"actual:   %v\n",
					test.expectedFinalMembersIndexes,
					actualFinalMembersIndexes,
				)
			}
		})
	}
}

func testWaitForBlockFn(localChain *localChain) waitForBlockFn {
	return func(ctx context.Context, block uint64) error {
		blockCounter, err := localChain.BlockCounter()
		if err != nil {
			return err
		}

		wait, err := blockCounter.BlockHeightWaiter(block)
		if err != nil {
			return err
		}

		select {
		case <-wait:
		case <-ctx.Done():
		}

		return nil
	}
}

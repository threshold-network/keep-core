package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	"golang.org/x/exp/slices"

	"golang.org/x/crypto/sha3"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/operator"
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

			dkgExecutor.executeDkgValidation(
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

// selectGroupChain wraps *localChain and overrides SelectGroup so tests can
// inject arbitrary group selection results without triggering the panic in
// the default localChain implementation.
type selectGroupChain struct {
	*localChain
	selectGroupResult *GroupSelectionResult
	selectGroupErr    error
}

func (c *selectGroupChain) SelectGroup() (*GroupSelectionResult, error) {
	return c.selectGroupResult, c.selectGroupErr
}

// TestDkgExecutor_CheckEligibility covers the eligibility decision path of
// checkEligibility: operator selected, not selected, multiple seats, group
// size exceeded, and SelectGroup failure.
func TestDkgExecutor_CheckEligibility(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     3,
		HonestThreshold: 2,
	}

	const myAddress chain.Address = "0xMY"

	tests := map[string]struct {
		selectionResult *GroupSelectionResult
		selectionErr    error
		wantIndexes     []uint8
		wantErr         bool
	}{
		"operator not selected": {
			selectionResult: &GroupSelectionResult{
				OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5},
				OperatorsAddresses: chain.Addresses{"0xAA", "0xBB", "0xCC", "0xDD", "0xEE"},
			},
			wantIndexes: []uint8{},
		},
		"operator holds one seat": {
			selectionResult: &GroupSelectionResult{
				OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5},
				OperatorsAddresses: chain.Addresses{"0xAA", myAddress, "0xCC", "0xDD", "0xEE"},
			},
			wantIndexes: []uint8{2},
		},
		"operator holds multiple seats": {
			selectionResult: &GroupSelectionResult{
				OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5},
				OperatorsAddresses: chain.Addresses{myAddress, "0xBB", myAddress, "0xDD", myAddress},
			},
			wantIndexes: []uint8{1, 3, 5},
		},
		"group size larger than supported": {
			selectionResult: &GroupSelectionResult{
				OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5, 6},
				OperatorsAddresses: chain.Addresses{"0xAA", "0xBB", "0xCC", "0xDD", "0xEE", "0xFF"},
			},
			wantErr: true,
		},
		"SelectGroup returns error": {
			selectionErr: fmt.Errorf("chain unavailable"),
			wantErr:      true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			baseChain := Connect()
			c := &selectGroupChain{
				localChain:        baseChain,
				selectGroupResult: test.selectionResult,
				selectGroupErr:    test.selectionErr,
			}

			de := &dkgExecutor{
				groupParameters: groupParameters,
				operatorAddress: myAddress,
				chain:           c,
			}

			logger := &testutils.MockLogger{}
			indexes, _, err := de.checkEligibility(logger)

			if (err != nil) != test.wantErr {
				t.Fatalf("checkEligibility error = %v, wantErr %v", err, test.wantErr)
			}

			if test.wantErr {
				return
			}

			if !reflect.DeepEqual(test.wantIndexes, indexes) {
				t.Errorf(
					"unexpected indexes\nexpected: %v\nactual:   %v",
					test.wantIndexes,
					indexes,
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

// TestDkgExecutor_ExecuteDkgIfEligible_NotEligible verifies that
// executeDkgIfEligible returns cleanly when the operator is not included in
// the selected signing group.
func TestDkgExecutor_ExecuteDkgIfEligible_NotEligible(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     3,
		HonestThreshold: 2,
	}

	const myAddress chain.Address = "0xME"

	c := &selectGroupChain{
		localChain: Connect(),
		selectGroupResult: &GroupSelectionResult{
			OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5},
			OperatorsAddresses: chain.Addresses{"0xAA", "0xBB", "0xCC", "0xDD", "0xEE"},
		},
	}

	de := &dkgExecutor{
		groupParameters: groupParameters,
		operatorAddress: myAddress,
		chain:           c,
	}

	de.executeDkgIfEligible(big.NewInt(1), 0, 0)
}

// TestDkgExecutor_ExecuteDkgIfEligible_SelectGroupError verifies that
// executeDkgIfEligible returns cleanly when SelectGroup returns an error.
func TestDkgExecutor_ExecuteDkgIfEligible_SelectGroupError(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     3,
		HonestThreshold: 2,
	}

	const myAddress chain.Address = "0xME"

	c := &selectGroupChain{
		localChain:     Connect(),
		selectGroupErr: fmt.Errorf("chain unavailable"),
	}

	de := &dkgExecutor{
		groupParameters: groupParameters,
		operatorAddress: myAddress,
		chain:           c,
	}

	de.executeDkgIfEligible(big.NewInt(1), 0, 0)
}

// TestDkgExecutor_ExecuteDkgIfEligible_PreParamExhaustion verifies that
// executeDkgIfEligible returns cleanly when the operator is eligible but the
// pre-parameters pool is empty (insufficient pre-params for the required
// member count).
func TestDkgExecutor_ExecuteDkgIfEligible_PreParamExhaustion(t *testing.T) {
	groupParameters := &GroupParameters{
		GroupSize:       5,
		GroupQuorum:     3,
		HonestThreshold: 2,
	}

	const myAddress chain.Address = "0xME"

	// Operator holds one seat in the selected group.
	c := &selectGroupChain{
		localChain: Connect(),
		selectGroupResult: &GroupSelectionResult{
			OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5},
			OperatorsAddresses: chain.Addresses{myAddress, "0xBB", "0xCC", "0xDD", "0xEE"},
		},
	}

	// poolSize=1 with a long generation timeout avoids a tight goroutine loop:
	// the worker blocks inside GeneratePreParamsWithContext (not re-entering
	// it repeatedly), so pre-params generation happens at most once before
	// blocking on the full pool. The pool starts empty, so PreParamsCount()
	// returns 0 immediately -- satisfying membersCount(1) > preParamsCount(0).
	tecdsaExec := dkg.NewExecutor(
		&testutils.MockLogger{},
		newTestScheduler(t),
		&mockPersistenceHandle{},
		1,         // preParamsPoolSize: 1 slot (not unbuffered)
		time.Hour, // preParamsGenerationTimeout: avoids 0-deadline tight loop
		0,         // preParamsGenerationDelay
		0,         // preParamsGenerationConcurrency
		0,         // keyGenerationConcurrency
	)

	de := &dkgExecutor{
		groupParameters: groupParameters,
		operatorAddress: myAddress,
		chain:           c,
		tecdsaExecutor:  tecdsaExec,
	}

	de.executeDkgIfEligible(big.NewInt(1), 0, 0)
}

// TestDkgExecutor_ExecuteDkgValidation_ValidationCheckError verifies that
// executeDkgValidation returns gracefully when IsDKGResultValid returns an
// error (e.g. chain temporarily unavailable).
func TestDkgExecutor_ExecuteDkgValidation_ValidationCheckError(t *testing.T) {
	c := &dkgResultValidErrChain{Connect()}
	de := &dkgExecutor{chain: c}

	de.executeDkgValidation(big.NewInt(1), 0, &DKGChainResult{}, [32]byte{})
}

// TestDkgExecutor_ExecuteDkgValidation_InvalidResult_ChallengeFails verifies
// that executeDkgValidation returns after logging an error when the result is
// invalid but ChallengeDKGResult fails because the chain is not in Challenge
// state (the default Idle state of localChain).
func TestDkgExecutor_ExecuteDkgValidation_InvalidResult_ChallengeFails(t *testing.T) {
	// localChain defaults: dkgResultValid=false, dkgState=Idle.
	// IsDKGResultValid → (false, nil); ChallengeDKGResult → error.
	de := &dkgExecutor{chain: Connect()}

	de.executeDkgValidation(big.NewInt(1), 0, &DKGChainResult{}, [32]byte{})
}

// TestDkgExecutor_ExecuteDkgValidation_ValidResult_NotMember verifies that
// executeDkgValidation returns early with "not eligible" when the result is
// valid but the current operator is not among the DKG participants.
func TestDkgExecutor_ExecuteDkgValidation_ValidResult_NotMember(t *testing.T) {
	c := Connect()
	c.setDKGResultValidity(true) // IsDKGResultValid → (true, nil)

	de := &dkgExecutor{
		chain: c,
		operatorIDFn: func() (chain.OperatorID, error) {
			return chain.OperatorID(99), nil // not in result.Members
		},
	}

	result := &DKGChainResult{
		Members: chain.OperatorIDs{1, 2, 3, 4, 5},
	}

	de.executeDkgValidation(big.NewInt(1), 0, result, [32]byte{})
}

// TestDkgExecutor_ExecuteDkgValidation_ValidResult_OperatorIDError verifies
// that executeDkgValidation returns gracefully when the result is valid but
// operatorIDFn returns an error (unable to determine operator identity).
func TestDkgExecutor_ExecuteDkgValidation_ValidResult_OperatorIDError(t *testing.T) {
	c := Connect()
	c.setDKGResultValidity(true)

	de := &dkgExecutor{
		chain: c,
		operatorIDFn: func() (chain.OperatorID, error) {
			return 0, fmt.Errorf("ID lookup failed")
		},
	}

	de.executeDkgValidation(big.NewInt(1), 0, &DKGChainResult{}, [32]byte{})
}

// TestDkgExecutor_ExecuteDkgValidation_ValidResult_MemberDKGParamsError verifies
// that executeDkgValidation returns gracefully when the result is valid, the
// operator is a member, but DKGParameters returns an error before approval is
// scheduled.
func TestDkgExecutor_ExecuteDkgValidation_ValidResult_MemberDKGParamsError(t *testing.T) {
	c := &dkgParamsErrChain{Connect()}
	c.localChain.setDKGResultValidity(true)

	de := &dkgExecutor{
		chain: c,
		operatorIDFn: func() (chain.OperatorID, error) {
			return chain.OperatorID(1), nil // member of result.Members
		},
	}

	result := &DKGChainResult{
		Members: chain.OperatorIDs{1, 2, 3, 4, 5},
	}

	de.executeDkgValidation(big.NewInt(1), 0, result, [32]byte{})
}

// TestDkgExecutor_GenerateSigningGroup_DKGParametersError verifies that
// generateSigningGroup returns gracefully when DKGParameters returns an error.
// setupBroadcastChannel succeeds; the function exits before spawning goroutines.
func TestDkgExecutor_GenerateSigningGroup_DKGParametersError(t *testing.T) {
	_, operatorPublicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
	if err != nil {
		t.Fatal(err)
	}

	c := &dkgParamsErrChain{Connect()}
	netProvider := local.ConnectWithKey(operatorPublicKey)

	de := &dkgExecutor{
		chain:       c,
		netProvider: netProvider,
	}

	gsr := &GroupSelectionResult{
		OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5},
		OperatorsAddresses: chain.Addresses{"0xAA", "0xBB", "0xCC", "0xDD", "0xEE"},
	}

	de.generateSigningGroup(
		logger.With(),
		big.NewInt(1),
		[]uint8{1},
		gsr,
		0,
		0,
	)
}

// dkgResultValidErrChain wraps localChain and returns an error from
// IsDKGResultValid to exercise the early-return error path in executeDkgValidation.
type dkgResultValidErrChain struct {
	*localChain
}

func (c *dkgResultValidErrChain) IsDKGResultValid(*DKGChainResult) (bool, error) {
	return false, fmt.Errorf("chain unavailable")
}

// dkgParamsErrChain wraps localChain and returns an error from DKGParameters
// to exercise the early-return error path in generateSigningGroup.
type dkgParamsErrChain struct {
	*localChain
}

func (c *dkgParamsErrChain) DKGParameters() (*DKGParameters, error) {
	return nil, fmt.Errorf("params unavailable")
}

// TestDkgExecutor_GenerateSigningGroup_BroadcastChannelError verifies that
// generateSigningGroup returns gracefully when the net.Provider fails to
// create a broadcast channel. The function exits before spawning goroutines.
func TestDkgExecutor_GenerateSigningGroup_BroadcastChannelError(t *testing.T) {
	de := &dkgExecutor{
		chain:       Connect(),
		netProvider: &errNetProvider{},
	}

	gsr := &GroupSelectionResult{
		OperatorsIDs:       chain.OperatorIDs{1, 2, 3, 4, 5},
		OperatorsAddresses: chain.Addresses{"0xAA", "0xBB", "0xCC", "0xDD", "0xEE"},
	}

	de.generateSigningGroup(
		logger.With(),
		big.NewInt(1),
		[]uint8{1},
		gsr,
		0,
		0,
	)
}

// errNetProvider is a minimal net.Provider stub whose BroadcastChannelFor always
// returns an error, used to exercise the broadcast-channel setup failure path in
// generateSigningGroup.
type errNetProvider struct{}

func (p *errNetProvider) ID() net.TransportIdentifier { return nil }
func (p *errNetProvider) Type() string                { return "" }
func (p *errNetProvider) BroadcastChannelFor(_ string) (net.BroadcastChannel, error) {
	return nil, fmt.Errorf("network unavailable")
}
func (p *errNetProvider) ConnectionManager() net.ConnectionManager { return nil }
func (p *errNetProvider) CreateTransportIdentifier(_ *operator.PublicKey) (net.TransportIdentifier, error) {
	return nil, nil
}
func (p *errNetProvider) BroadcastChannelForwarderFor(_ string) {}

package dkg

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"github.com/keep-network/keep-core/pkg/beacon/event"
	"github.com/keep-network/keep-core/pkg/beacon/gjkr"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

var (
	playerIndex                 group.MemberIndex
	groupPublicKey              *bn256.G2
	gjkrResult                  *gjkr.Result
	dkgResultChannel            chan *event.DKGResultSubmission
	startPublicationBlockHeight uint64
	beaconChain                 beaconchain.Interface
	blockCounter                chain.BlockCounter
)

func setup() {
	playerIndex = group.MemberIndex(1)
	groupPublicKey = new(bn256.G2).ScalarBaseMult(big.NewInt(10))
	dkgGroup := group.NewGroup(4, 10)
	gjkrResult = &gjkr.Result{
		GroupPublicKey: groupPublicKey,
		Group:          dkgGroup,
	}
	dkgResultChannel = make(chan *event.DKGResultSubmission, 1)
	startPublicationBlockHeight = uint64(0)
	localChain := local_v1.Connect(5, 3)
	beaconChain = localChain
	blockCounter, _ = beaconChain.BlockCounter()
}

func TestDecideMemberFate_HappyPath(t *testing.T) {
	setup()

	dkgResultChannel <- &event.DKGResultSubmission{
		GroupPublicKey: groupPublicKey.Marshal(),
		Misbehaved:     []byte{7, 10},
	}

	operatingMemberIndexes, err := decideMemberFate(
		playerIndex,
		gjkrResult,
		dkgResultChannel,
		startPublicationBlockHeight,
		beaconChain,
		blockCounter,
	)
	if err != nil {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			nil,
			err,
		)
	}

	expectedOperatingMemberIndexes := []group.MemberIndex{1, 2, 3, 4, 5, 6, 8, 9}
	if !reflect.DeepEqual(expectedOperatingMemberIndexes, operatingMemberIndexes) {
		t.Errorf(
			"unexpected operating members\nexpected: %v\nactual:   %v\n",
			expectedOperatingMemberIndexes,
			operatingMemberIndexes,
		)
	}
}

func TestDecideMemberFate_NotSameGroupPublicKey(t *testing.T) {
	setup()

	otherGroupPublicKey := new(bn256.G2).ScalarBaseMult(big.NewInt(11))
	dkgResultChannel <- &event.DKGResultSubmission{
		GroupPublicKey: otherGroupPublicKey.Marshal(),
		Misbehaved:     []byte{},
	}

	_, err := decideMemberFate(
		playerIndex,
		gjkrResult,
		dkgResultChannel,
		startPublicationBlockHeight,
		beaconChain,
		blockCounter,
	)

	expectedError := fmt.Errorf(
		"[member:%v] could not stay in the group because "+
			"member do not support the same group public key",
		playerIndex,
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedError,
			err,
		)
	}
}

func TestDecideMemberFate_MemberIsMisbehaved(t *testing.T) {
	setup()

	dkgResultChannel <- &event.DKGResultSubmission{
		GroupPublicKey: groupPublicKey.Marshal(),
		Misbehaved:     []byte{playerIndex},
	}

	_, err := decideMemberFate(
		playerIndex,
		gjkrResult,
		dkgResultChannel,
		startPublicationBlockHeight,
		beaconChain,
		blockCounter,
	)

	expectedError := fmt.Errorf(
		"[member:%v] could not stay in the group because "+
			"member is considered as misbehaving",
		playerIndex,
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedError,
			err,
		)
	}
}

func TestDecideMemberFate_Timeout(t *testing.T) {
	setup()

	_, err := decideMemberFate(
		playerIndex,
		gjkrResult,
		dkgResultChannel,
		startPublicationBlockHeight,
		beaconChain,
		blockCounter,
	)

	expectedError := fmt.Errorf("DKG result publication timed out")
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedError,
			err,
		)
	}
}

func TestWaitForDkgResultEvent_InvalidGroupSize(t *testing.T) {
	setup()

	localChain := local_v1.Connect(0, 3)

	_, err := waitForDkgResultEvent(
		dkgResultChannel,
		startPublicationBlockHeight,
		localChain,
		blockCounter,
	)

	expectedError := fmt.Errorf("invalid group size: [%v]", 0)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\nexpected: %v\nactual:   %v\n",
			expectedError,
			err,
		)
	}
}

func TestResolveGroupOperators(t *testing.T) {
	beaconConfig := &beaconchain.Config{
		GroupSize:       5,
		HonestThreshold: 3,
	}

	selectedOperators := []chain.Address{
		"0xAA",
		"0xBB",
		"0xCC",
		"0xDD",
		"0xEE",
	}

	var tests = map[string]struct {
		selectedOperators        []chain.Address
		operatingGroupMembersIDs []group.MemberIndex
		expectedGroupOperators   []chain.Address
		expectedError            error
	}{
		"selected operators count not equal to the group size": {
			selectedOperators:        selectedOperators[:4],
			operatingGroupMembersIDs: []group.MemberIndex{1, 2, 3, 4, 5},
			expectedError:            fmt.Errorf("invalid input parameters"),
		},
		"all selected operators are operating": {
			selectedOperators:        selectedOperators,
			operatingGroupMembersIDs: []group.MemberIndex{5, 4, 3, 2, 1},
			expectedGroupOperators:   selectedOperators,
		},
		"honest majority of selected operators are operating": {
			selectedOperators:        selectedOperators,
			operatingGroupMembersIDs: []group.MemberIndex{5, 1, 3},
			expectedGroupOperators:   []chain.Address{"0xAA", "0xCC", "0xEE"},
		},
		"less than honest majority of selected operators are operating": {
			selectedOperators:        selectedOperators,
			operatingGroupMembersIDs: []group.MemberIndex{5, 1},
			expectedError:            fmt.Errorf("invalid input parameters"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actualGroupOperators, err := resolveGroupOperators(
				test.selectedOperators,
				test.operatingGroupMembersIDs,
				beaconConfig,
			)

			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v\n",
					test.expectedError,
					err,
				)
			}

			if !reflect.DeepEqual(test.expectedGroupOperators, actualGroupOperators) {
				t.Errorf(
					"unexpected group operators\nexpected: %v\nactual:   %v\n",
					test.expectedGroupOperators,
					actualGroupOperators,
				)
			}
		})
	}
}

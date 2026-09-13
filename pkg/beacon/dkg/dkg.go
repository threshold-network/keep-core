package dkg

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/ipfs/go-log/v2"

	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	dkgResult "github.com/keep-network/keep-core/pkg/beacon/dkg/result"
	"github.com/keep-network/keep-core/pkg/beacon/event"
	"github.com/keep-network/keep-core/pkg/beacon/gjkr"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ExecuteDKG runs the full distributed key generation lifecycle.
func ExecuteDKG(
	logger log.StandardLogger,
	seed *big.Int,
	memberIndex group.MemberIndex,
	startBlockHeight uint64,
	beaconChain beaconchain.Interface,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	selectedOperators []chain.Address,
) (*ThresholdSigner, error) {
	beaconConfig := beaconChain.GetConfig()

	blockCounter, err := beaconChain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	gjkr.RegisterUnmarshallers(channel)
	dkgResult.RegisterUnmarshallers(channel)

	sessionID := seed.Text(16)

	gjkrResult, gjkrEndBlockHeight, err := gjkr.Execute(
		logger,
		seed,
		sessionID,
		memberIndex,
		beaconConfig.GroupSize,
		blockCounter,
		channel,
		beaconConfig.DishonestThreshold(),
		membershipValidator,
		startBlockHeight,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"[member:%v] GJKR execution failed [%v]",
			memberIndex,
			err,
		)
	}

	startPublicationBlockHeight := gjkrEndBlockHeight

	operatingMemberIndexes := gjkrResult.Group.OperatingMemberIndexes()

	dkgResultChannel := make(chan *event.DKGResultSubmission)
	dkgResultSubscription := beaconChain.OnDKGResultSubmitted(
		func(event *event.DKGResultSubmission) {
			dkgResultChannel <- event
		},
	)
	defer dkgResultSubscription.Unsubscribe()

	err = dkgResult.Publish(
		logger,
		sessionID,
		memberIndex,
		gjkrResult.Group,
		membershipValidator,
		gjkrResult,
		channel,
		beaconChain,
		blockCounter,
		startPublicationBlockHeight,
	)
	if err != nil {
		// Result publication failed. It means that either the result this
		// member proposed is not supported by the majority of group members or
		// that the chain interaction failed. In either case, we observe the
		// chain for the result published by any other group member and based
		// on that, we decide whether we should stay in the final group
		// or drop our membership.
		logger.Warnf(
			"[member:%v] DKG result publication process failed [%v]",
			memberIndex,
			err,
		)

		if operatingMemberIndexes, err = decideMemberFate(
			memberIndex,
			gjkrResult,
			dkgResultChannel,
			startPublicationBlockHeight,
			beaconChain,
			blockCounter,
		); err != nil {
			return nil, err
		}
	}

	groupOperators, err := resolveGroupOperators(
		selectedOperators,
		operatingMemberIndexes,
		beaconConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve group operators: [%v]", err)
	}

	return &ThresholdSigner{
		memberIndex:          memberIndex,
		groupPublicKey:       gjkrResult.GroupPublicKey,
		groupPrivateKeyShare: gjkrResult.GroupPrivateKeyShare,
		groupPublicKeyShares: gjkrResult.GroupPublicKeyShares(),
		groupOperators:       groupOperators,
	}, nil
}

// decideMemberFate decides what the member will do in case it failed
// publishing its DKG result. Member can stay in the group if it
// supports the same group public key as the one registered on-chain and
// the member is not considered as misbehaving by the group.
func decideMemberFate(
	playerIndex group.MemberIndex,
	gjkrResult *gjkr.Result,
	dkgResultChannel chan *event.DKGResultSubmission,
	startPublicationBlockHeight uint64,
	beaconChain beaconchain.Interface,
	blockCounter chain.BlockCounter,
) ([]group.MemberIndex, error) {
	dkgResultEvent, err := waitForDkgResultEvent(
		dkgResultChannel,
		startPublicationBlockHeight,
		beaconChain,
		blockCounter,
	)
	if err != nil {
		return nil, err
	}

	groupPublicKey, err := gjkrResult.GroupPublicKeyBytes()
	if err != nil {
		return nil, err
	}

	// If member don't support the same group public key, it could not stay
	// in the group.
	if !bytes.Equal(groupPublicKey, dkgResultEvent.GroupPublicKey) {
		return nil, fmt.Errorf(
			"[member:%v] could not stay in the group because "+
				"member do not support the same group public key",
			playerIndex,
		)
	}

	misbehavedSet := make(map[group.MemberIndex]struct{})
	for _, misbehavedID := range dkgResultEvent.Misbehaved {
		misbehavedSet[misbehavedID] = struct{}{}
	}

	// If member is considered as misbehaved, it could not stay in the group.
	if _, isMisbehaved := misbehavedSet[playerIndex]; isMisbehaved {
		return nil, fmt.Errorf(
			"[member:%v] could not stay in the group because "+
				"member is considered as misbehaving",
			playerIndex,
		)
	}

	// Construct a new view of the operating members according to the accepted
	// DKG result.
	operatingMemberIndexes := make([]group.MemberIndex, 0)
	for _, memberID := range gjkrResult.Group.MemberIndexes() {
		if _, isMisbehaved := misbehavedSet[memberID]; !isMisbehaved {
			operatingMemberIndexes = append(operatingMemberIndexes, memberID)
		}
	}

	return operatingMemberIndexes, nil
}

func waitForDkgResultEvent(
	dkgResultChannel chan *event.DKGResultSubmission,
	startPublicationBlockHeight uint64,
	beaconChain beaconchain.Interface,
	blockCounter chain.BlockCounter,
) (*event.DKGResultSubmission, error) {
	config := beaconChain.GetConfig()
	if config.GroupSize <= 0 {
		return nil, fmt.Errorf("invalid group size: [%v]", config.GroupSize)
	}

	timeoutBlock := startPublicationBlockHeight +
		dkgResult.PrePublicationBlocks() +
		(uint64(config.GroupSize) * config.ResultPublicationBlockStep)

	timeoutBlockChannel, err := blockCounter.BlockHeightWaiter(timeoutBlock)
	if err != nil {
		return nil, err
	}

	select {
	case dkgResultEvent := <-dkgResultChannel:
		return dkgResultEvent, nil
	case <-timeoutBlockChannel:
		return nil, fmt.Errorf("DKG result publication timed out")
	}
}

// resolveGroupOperators takes two parameters:
//   - selectedOperators: Contains addresses of all selected operators. Slice
//     length equals to the groupSize. Each element with index N corresponds
//     to the group member with ID N+1.
//   - operatingGroupMembersIDs: Contains group members IDs that were neither
//     disqualified nor marked as inactive. Slice length is lesser than or equal
//     to the groupSize.
//
// Using those parameters, this function transforms the selectedOperators
// slice into another slice that contains addresses of all operators
// that were neither disqualified nor marked as inactive. This way, the
// resulting slice has only addresses of properly operating operators
// who form the resulting group.
//
// Example:
// selectedOperators: [member1, member2, member3, member4, member5]
// operatingGroupMembersIDs: [5, 1, 3]
// groupOperators: [member1, member3, member5]
func resolveGroupOperators(
	selectedOperators []chain.Address,
	operatingGroupMembersIDs []group.MemberIndex,
	beaconConfig *beaconchain.Config,
) ([]chain.Address, error) {
	if len(selectedOperators) != beaconConfig.GroupSize ||
		len(operatingGroupMembersIDs) < beaconConfig.HonestThreshold {
		return nil, fmt.Errorf("invalid input parameters")
	}

	sort.Slice(operatingGroupMembersIDs, func(i, j int) bool {
		return operatingGroupMembersIDs[i] < operatingGroupMembersIDs[j]
	})

	groupOperators := make(
		[]chain.Address,
		len(operatingGroupMembersIDs),
	)

	for i, operatingMemberID := range operatingGroupMembersIDs {
		groupOperators[i] = selectedOperators[operatingMemberID-1]
	}

	return groupOperators, nil
}

package ethereum

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/beacon/event"
	"github.com/keep-network/keep-core/pkg/subscription"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen/contract"
	"github.com/keep-network/keep-core/pkg/operator"
)

// Definitions of contract names.
const (
	RandomBeaconContractName = "RandomBeacon"
)

var errNotImplemented = fmt.Errorf("not implemented")

// BeaconChain represents a beacon-specific chain handle.
type BeaconChain struct {
	*baseChain

	randomBeacon  *contract.RandomBeacon
	sortitionPool *contract.BeaconSortitionPool
}

// newBeaconChain construct a new instance of the beacon-specific Ethereum
// chain handle.
func newBeaconChain(
	config ethereum.Config,
	baseChain *baseChain,
) (*BeaconChain, error) {
	randomBeaconAddress, err := config.ContractAddress(RandomBeaconContractName)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			RandomBeaconContractName,
			err,
		)
	}

	randomBeacon, err :=
		contract.NewRandomBeacon(
			randomBeaconAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to RandomBeacon contract: [%v]",
			err,
		)
	}

	sortitionPoolAddress, err := randomBeacon.SortitionPool()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get sortition pool address: [%v]",
			err,
		)
	}

	sortitionPool, err :=
		contract.NewBeaconSortitionPool(
			sortitionPoolAddress,
			baseChain.chainID,
			baseChain.key,
			baseChain.client,
			baseChain.nonceManager,
			baseChain.miningWaiter,
			baseChain.blockCounter,
			baseChain.transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to BeaconSortitionPool contract: [%v]",
			err,
		)
	}

	return &BeaconChain{
		baseChain:     baseChain,
		randomBeacon:  randomBeacon,
		sortitionPool: sortitionPool,
	}, nil
}

// GetConfig returns the expected configuration of the random beacon.
// TODO: Adjust to the random beacon v2 requirements.
func (bc *BeaconChain) GetConfig() *beaconchain.Config {
	groupSize := 64
	honestThreshold := 33
	resultPublicationBlockStep := 1
	relayEntryTimeout := groupSize * resultPublicationBlockStep

	return &beaconchain.Config{
		GroupSize:                  groupSize,
		HonestThreshold:            honestThreshold,
		ResultPublicationBlockStep: uint64(resultPublicationBlockStep),
		RelayEntryTimeout:          uint64(relayEntryTimeout),
	}
}

// Staking returns address of the TokenStaking contract the RandomBeacon is
// connected to.
func (bc *BeaconChain) Staking() (chain.Address, error) {
	stakingContractAddress, err := bc.randomBeacon.Staking()
	if err != nil {
		return "", fmt.Errorf(
			"failed to get the token staking address: [%w]",
			err,
		)
	}

	return chain.Address(stakingContractAddress.String()), nil
}

// OperatorToStakingProvider returns the staking provider address for the
// operator. If the staking provider has not been registered for the
// operator, the returned address is empty and the boolean flag is set to false
// If the staking provider has been registered, the address is not empty and the
// boolean flag indicates true.
func (bc *BeaconChain) OperatorToStakingProvider() (chain.Address, bool, error) {
	stakingProvider, err := bc.randomBeacon.OperatorToStakingProvider(bc.key.Address)
	if err != nil {
		return "", false, fmt.Errorf(
			"failed to map operator [%v] to a staking provider: [%v]",
			bc.key.Address,
			err,
		)
	}

	if (stakingProvider == common.Address{}) {
		return "", false, nil
	}

	return chain.Address(stakingProvider.Hex()), true, nil
}

// EligibleStake returns the current value of the staking provider's eligible
// stake. Eligible stake is defined as the currently authorized stake minus the
// pending authorization decrease. Eligible stake is what is used for operator's
// weight in the sortition pool. If the authorized stake minus the pending
// authorization decrease is below the minimum authorization, eligible stake
// is 0.
func (bc *BeaconChain) EligibleStake(stakingProvider chain.Address) (*big.Int, error) {
	eligibleStake, err := bc.randomBeacon.EligibleStake(common.HexToAddress(stakingProvider.String()))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get eligible stake for staking provider %s: [%w]",
			stakingProvider,
			err,
		)
	}

	return eligibleStake, nil
}

// IsPoolLocked returns true if the sortition pool is locked and no state
// changes are allowed.
func (bc *BeaconChain) IsPoolLocked() (bool, error) {
	return bc.sortitionPool.IsLocked()
}

// IsOperatorInPool returns true if the operator is registered in the
// sortition pool.
func (bc *BeaconChain) IsOperatorInPool() (bool, error) {
	return bc.randomBeacon.IsOperatorInPool(bc.key.Address)
}

// IsOperatorUpToDate checks if the operator's authorized stake is in sync
// with operator's weight in the sortition pool.
// If the operator's authorized stake is not in sync with sortition pool
// weight, function returns false.
// If the operator is not in the sortition pool and their authorized stake
// is non-zero, function returns false.
func (bc *BeaconChain) IsOperatorUpToDate() (bool, error) {
	return bc.randomBeacon.IsOperatorUpToDate(bc.key.Address)
}

// JoinSortitionPool executes a transaction to have the operator join the
// sortition pool.
func (bc *BeaconChain) JoinSortitionPool() error {
	_, err := bc.randomBeacon.JoinSortitionPool()
	return err
}

// UpdateOperatorStatus executes a transaction to update the operator's state in
// the sortition pool.
func (bc *BeaconChain) UpdateOperatorStatus() error {
	_, err := bc.randomBeacon.UpdateOperatorStatus(bc.key.Address)
	return err
}

// IsEligibleForRewards checks whether the operator is eligible for rewards or
// not.
func (bc *BeaconChain) IsEligibleForRewards() (bool, error) {
	return bc.sortitionPool.IsEligibleForRewards(bc.key.Address)
}

// Checks whether the operator is able to restore their eligibility for rewards
// right away.
func (bc *BeaconChain) CanRestoreRewardEligibility() (bool, error) {
	return bc.sortitionPool.CanRestoreRewardEligibility(bc.key.Address)
}

// Restores reward eligibility for the operator.
func (bc *BeaconChain) RestoreRewardEligibility() error {
	_, err := bc.sortitionPool.RestoreRewardEligibility(bc.key.Address)
	return err
}

// Returns true if the chaosnet phase is active, false otherwise.
func (bc *BeaconChain) IsChaosnetActive() (bool, error) {
	return bc.sortitionPool.IsChaosnetActive()
}

// Returns true if operator is a beta operator, false otherwise.
// Chaosnet status does not matter.
func (bc *BeaconChain) IsBetaOperator() (bool, error) {
	return bc.sortitionPool.IsBetaOperator(bc.key.Address)
}

// GetOperatorID returns the ID number of the given operator address. An ID
// number of 0 means the operator has not been allocated an ID number yet.
func (bc *BeaconChain) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	return bc.sortitionPool.GetOperatorID(
		common.HexToAddress(operatorAddress.String()),
	)
}

// SelectGroup returns the group members for the group generated by
// the given seed. This function can return an error if the beacon chain's
// state does not allow for group selection at the moment.
func (bc *BeaconChain) SelectGroup(seed *big.Int) (chain.Addresses, error) {
	groupSize := big.NewInt(int64(bc.GetConfig().GroupSize))
	seedBytes := [32]byte{}
	seed.FillBytes(seedBytes[:])

	// TODO: Replace with a call to the RandomBeacon.selectGroup function.
	operatorsIDs, err := bc.sortitionPool.SelectGroup(groupSize, seedBytes)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot select group in the sortition pool: [%v]",
			err,
		)
	}

	operatorsAddresses, err := bc.sortitionPool.GetIDOperators(operatorsIDs)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert operators' IDs to addresses: [%v]",
			err,
		)
	}

	result := make([]chain.Address, len(operatorsAddresses))
	for i := range result {
		result[i] = chain.Address(operatorsAddresses[i].String())
	}

	return result, nil
}

// TODO: Implement a real OnGroupRegistered function.
func (bc *BeaconChain) OnGroupRegistered(
	handler func(groupRegistration *event.GroupRegistration),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

// TODO: Implement a real IsGroupRegistered function.
func (bc *BeaconChain) IsGroupRegistered(groupPublicKey []byte) (bool, error) {
	return false, errNotImplemented
}

// TODO: Implement a real IsStaleGroup function.
func (bc *BeaconChain) IsStaleGroup(groupPublicKey []byte) (bool, error) {
	return false, nil
}

// TODO: Implement a real OnDKGStarted event subscription. The current
// implementation generates a fake event every 500th block where the
// seed is the keccak256 of the block number.
func (bc *BeaconChain) OnDKGStarted(
	handler func(event *event.DKGStarted),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

// TODO: Implement a real SubmitDKGResult action. The current implementation
// just creates and pipes the DKG submission event to the handlers
// registered in the dkgResultSubmissionHandlers map.
func (bc *BeaconChain) SubmitDKGResult(
	participantIndex beaconchain.GroupMemberIndex,
	dkgResult *beaconchain.DKGResult,
	signatures map[beaconchain.GroupMemberIndex][]byte,
) error {
	return errNotImplemented
}

// TODO: Implement a real OnDKGResultSubmitted event subscription. The current
// implementation just pipes the DKG submission event generated within
// SubmitDKGResult to the handlers registered in the
// dkgResultSubmissionHandlers map.
func (bc *BeaconChain) OnDKGResultSubmitted(
	handler func(event *event.DKGResultSubmission),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

// CalculateDKGResultHash calculates Keccak-256 hash of the DKG result. Operation
// is performed off-chain.
//
// It first encodes the result using solidity ABI and then calculates Keccak-256
// hash over it. This corresponds to the DKG result hash calculation on-chain.
// Hashes calculated off-chain and on-chain must always match.
func (bc *BeaconChain) CalculateDKGResultHash(
	dkgResult *beaconchain.DKGResult,
) (beaconchain.DKGResultHash, error) {
	// Encode DKG result to the format matched with Solidity keccak256(abi.encodePacked(...))
	hash := crypto.Keccak256(dkgResult.GroupPublicKey, dkgResult.Misbehaved)
	return beaconchain.DKGResultHashFromBytes(hash)
}

// IsRecognized checks whether the given operator is recognized by the BeaconChain
// as eligible to join the network. If the operator has a stake delegation or
// had a stake delegation in the past, it will be recognized.
func (bc *BeaconChain) IsRecognized(operatorPublicKey *operator.PublicKey) (bool, error) {
	operatorAddress, err := operatorPublicKeyToChainAddress(operatorPublicKey)
	if err != nil {
		return false, fmt.Errorf(
			"cannot convert from operator key to chain address: [%v]",
			err,
		)
	}

	stakingProvider, err := bc.randomBeacon.OperatorToStakingProvider(
		operatorAddress,
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to map operator [%v] to a staking provider: [%v]",
			operatorAddress,
			err,
		)
	}

	if (stakingProvider == common.Address{}) {
		return false, nil
	}

	// Check if the staking provider has an owner. This check ensures that there
	// is/was a stake delegation for the given staking provider.
	//
	// This deliberately differs from TbtcChain.IsRecognized, which reads
	// eligible stake instead, and the asymmetry must not be harmonised away.
	// TokenStaking.authorizedStake short-circuits to zero for every application
	// but one hard-coded constant, and the random beacon is not that
	// application, so RandomBeacon.eligibleStake is zero for every staking
	// provider that has ever registered a beacon operator. An eligible-stake
	// predicate here would therefore recognize nobody. The watchtower re-runs
	// this check against every connected peer every ten minutes and disconnects
	// on failure, so the whole fleet would come apart within a single round.
	_, _, _, hasStakeDelegation, err := bc.baseChain.RolesOf(
		chain.Address(stakingProvider.Hex()),
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to check stake delegation for staking provider [%v]: [%v]",
			stakingProvider,
			err,
		)
	}

	if !hasStakeDelegation {
		return false, nil
	}

	return true, nil
}

// TODO: Implement a real SubmitRelayEntry function.
func (bc *BeaconChain) SubmitRelayEntry(
	entry []byte,
) error {
	return errNotImplemented
}

// TODO: Implement a real OnRelayEntrySubmitted function.
func (bc *BeaconChain) OnRelayEntrySubmitted(
	handler func(entry *event.RelayEntrySubmitted),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

// TODO: Implement a real OnRelayEntryRequested function.
func (bc *BeaconChain) OnRelayEntryRequested(
	handler func(request *event.RelayEntryRequested),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

// TODO: Implement a real ReportRelayEntryTimeout function.
func (bc *BeaconChain) ReportRelayEntryTimeout() error {
	return errNotImplemented
}

// TODO: Implement a real IsEntryInProgress function.
func (bc *BeaconChain) IsEntryInProgress() (bool, error) {
	return false, nil // no chain integration so not in progress
}

// TODO: Implement a real CurrentRequestStartBlock function.
func (bc *BeaconChain) CurrentRequestStartBlock() (*big.Int, error) {
	return nil, errNotImplemented
}

// TODO: Implement a real CurrentRequestPreviousEntry function.
func (bc *BeaconChain) CurrentRequestPreviousEntry() ([]byte, error) {
	return nil, errNotImplemented
}

// TODO: Implement a real CurrentRequestGroupPublicKey function.
func (bc *BeaconChain) CurrentRequestGroupPublicKey() ([]byte, error) {
	return nil, errNotImplemented
}

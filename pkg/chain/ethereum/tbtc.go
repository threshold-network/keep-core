package ethereum

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"time"

	"github.com/keep-network/keep-common/pkg/cache"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/chain"
	ecdsaabi "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/abi"
	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
	"github.com/keep-network/keep-core/pkg/crypto/secp256k1"
	"github.com/keep-network/keep-core/pkg/internal/byteutils"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tecdsa/dkg"
)

// Definitions of contract names.
const (
	// TODO: The WalletRegistry address is taken from the Bridge contract.
	//       Remove the possibility of passing it through the config.
	WalletRegistryContractName          = "WalletRegistry"
	BridgeContractName                  = "Bridge"
	MaintainerProxyContractName         = "MaintainerProxy"
	WalletProposalValidatorContractName = "WalletProposalValidator"
	// EcdsaDkgValidatorContractName is optional: when set under
	// ethereum contract addresses or developer.ecdsaDkgValidatorAddress
	// alias in config, TBTC ECDSA sizing is read via eth_call instead of only
	// network defaults from defaultGroupParameters.
	EcdsaDkgValidatorContractName = "EcdsaDkgValidator"
)

const (
	sweptDepositsCachePeriod = 7 * 24 * time.Hour
)

// TbtcChain represents a TBTC-specific chain handle.
type TbtcChain struct {
	*baseChain

	bridge                  *tbtccontract.Bridge
	maintainerProxy         *tbtccontract.MaintainerProxy
	walletRegistry          *ecdsacontract.WalletRegistry
	sortitionPool           *ecdsacontract.EcdsaSortitionPool
	walletProposalValidator *tbtccontract.WalletProposalValidator
	redemptionWatchtower    *tbtccontract.RedemptionWatchtower
	// reservationRouter is the abigen binding for ReservationRouter.sol's ABI
	// (functions, events, errors). It is NOT bound to the deployed router
	// address -- the router contract holds its own empty storage and only
	// ever executes via Bridge.fallback's delegatecall. The binding is
	// constructed against the Bridge address so every read, write, and log
	// filter goes through Bridge.fallback, which dispatches to the router
	// code with the Bridge's storage and emits events under the Bridge's
	// address. Calling the binding against the router's standalone address
	// would invoke its empty storage and either revert (writes) or return
	// zeros (views).
	reservationRouter *tbtccontract.ReservationRouter
	// ecdsaDkgValidatorAddress optional; when zero, TBTC uses defaultGroupParameters(network).
	ecdsaDkgValidatorAddress common.Address

	sweptDepositsCache *cache.GenericTimeCache[*tbtc.DepositChainRequest]
}

// NewTbtcChain construct a new instance of the TBTC-specific Ethereum
// chain handle.
func newTbtcChain(
	config ethereum.Config,
	baseChain *baseChain,
) (*TbtcChain, error) {
	bridgeAddress, err := config.ContractAddress(BridgeContractName)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			BridgeContractName,
			err,
		)
	}

	bridge, err :=
		tbtccontract.NewBridge(
			bridgeAddress,
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
			"failed to attach to Bridge contract: [%v]",
			err,
		)
	}

	maintainerProxyAddress, err := config.ContractAddress(MaintainerProxyContractName)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			MaintainerProxyContractName,
			err,
		)
	}

	maintainerProxy, err :=
		tbtccontract.NewMaintainerProxy(
			maintainerProxyAddress,
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
			"failed to attach to MaintainerProxy contract: [%v]",
			err,
		)
	}

	references, err := bridge.ContractReferences()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get contract references from Bridge: [%v]",
			err,
		)
	}

	walletRegistryAddress := references.EcdsaWalletRegistry

	walletRegistry, err :=
		ecdsacontract.NewWalletRegistry(
			walletRegistryAddress,
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
			"failed to attach to WalletRegistry contract: [%v]",
			err,
		)
	}

	sortitionPoolAddress, err := walletRegistry.SortitionPool()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get sortition pool address: [%v]",
			err,
		)
	}

	sortitionPool, err :=
		ecdsacontract.NewEcdsaSortitionPool(
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
			"failed to attach to EcdsaSortitionPool contract: [%v]",
			err,
		)
	}

	walletProposalValidatorAddress, err := config.ContractAddress(
		WalletProposalValidatorContractName,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			WalletProposalValidatorContractName,
			err,
		)
	}

	walletProposalValidator, err :=
		tbtccontract.NewWalletProposalValidator(
			walletProposalValidatorAddress,
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
			"failed to attach to WalletProposalValidator contract: [%v]",
			err,
		)
	}

	redemptionWatchtowerAddress, err := bridge.GetRedemptionWatchtower()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get RedemptionWatchtower address from Bridge: [%v]",
			err,
		)
	}

	// The RedemptionWatchtower contract is an additional component
	// that implements the redemption veto mechanism. It is optional
	// and may not be present in the system. This code must be able
	// to handle the case when the RedemptionWatchtower contract is
	// not set.
	var redemptionWatchtower *tbtccontract.RedemptionWatchtower
	if redemptionWatchtowerAddress != [20]byte{} {
		redemptionWatchtower, err =
			tbtccontract.NewRedemptionWatchtower(
				redemptionWatchtowerAddress,
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
				"failed to attach to RedemptionWatchtower contract: [%v]",
				err,
			)
		}
	}

	var ecdsaDkgValidatorAddress common.Address
	validatorAddr, err := config.ContractAddress(EcdsaDkgValidatorContractName)
	switch {
	case err == nil:
		ecdsaDkgValidatorAddress = validatorAddr
	case errors.Is(err, ethereum.ErrAddressNotConfigured):
		logger.Warnf(
			"%s contract address is not configured; TBTC group parameters "+
				"will fall back to network defaults instead of on-chain values",
			EcdsaDkgValidatorContractName,
		)
	default:
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%w]",
			EcdsaDkgValidatorContractName,
			err,
		)
	}

	reservationRouter, err := reservationRouterBinding(bridgeAddress, baseChain)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to ReservationRouter binding: [%v]",
			err,
		)
	}

	return &TbtcChain{
		baseChain:                baseChain,
		bridge:                   bridge,
		maintainerProxy:          maintainerProxy,
		walletRegistry:           walletRegistry,
		sortitionPool:            sortitionPool,
		walletProposalValidator:  walletProposalValidator,
		redemptionWatchtower:     redemptionWatchtower,
		reservationRouter:        reservationRouter,
		ecdsaDkgValidatorAddress: ecdsaDkgValidatorAddress,
		sweptDepositsCache:       cache.NewGenericTimeCache[*tbtc.DepositChainRequest](sweptDepositsCachePeriod),
	}, nil
}

// reservationRouterBinding constructs the ReservationRouter abigen binding
// pointed at the Bridge address. The router code only ever executes via
// Bridge.fallback's delegatecall, so the binding MUST be constructed against
// the Bridge address: the deployed router contract holds its own empty
// storage, so any call routed to its standalone address would either revert
// (writes) or return zeros (views); events emitted by router code carry the
// Bridge's address in their log because delegatecall preserves the caller's
// address context. The router's own deployment address is only needed for
// the one-time governance Bridge.setReservationRouter(routerAddress) call,
// which is out of scope here.
func reservationRouterBinding(
	bridgeAddress common.Address,
	baseChain *baseChain,
) (*tbtccontract.ReservationRouter, error) {
	return tbtccontract.NewReservationRouter(
		bridgeAddress,
		baseChain.chainID,
		baseChain.key,
		baseChain.client,
		baseChain.nonceManager,
		baseChain.miningWaiter,
		baseChain.blockCounter,
		baseChain.transactionMutex,
	)
}

// EcdsaWalletGroupParametersFromChain mirrors EcdsaDkgValidator sizing constants
// when EcdsaDkgValidator contract address was configured under [ethereum]
// contract addresses or developer.ecdsaDkgValidatorAddress alias. When absent,
// returns (nil, nil) and callers use defaultGroupParameters(network).
func (tc *TbtcChain) EcdsaWalletGroupParametersFromChain(
	ctx context.Context,
) (*tbtc.GroupParameters, error) {
	if tc.ecdsaDkgValidatorAddress == (common.Address{}) {
		return nil, nil
	}
	return ecdsaWalletGroupParametersFromValidator(
		ctx,
		tc.baseChain.client,
		tc.ecdsaDkgValidatorAddress,
	)
}

// Staking returns address of the TokenStaking contract the WalletRegistry is
// connected to.
func (tc *TbtcChain) Staking() (chain.Address, error) {
	stakingContractAddress, err := tc.walletRegistry.Staking()
	if err != nil {
		return "", fmt.Errorf(
			"failed to get the token staking address: [%w]",
			err,
		)
	}

	return chain.Address(stakingContractAddress.String()), nil
}

// IsRecognized checks whether the given operator is recognized by the TbtcChain
// as eligible to join the network. If the operator has a stake delegation or
// had a stake delegation in the past, it will be recognized.
func (tc *TbtcChain) IsRecognized(operatorPublicKey *operator.PublicKey) (bool, error) {
	operatorAddress, err := operatorPublicKeyToChainAddress(operatorPublicKey)
	if err != nil {
		return false, fmt.Errorf(
			"cannot convert from operator key to chain address: [%v]",
			err,
		)
	}

	stakingProvider, err := tc.walletRegistry.OperatorToStakingProvider(
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
	_, _, _, hasStakeDelegation, err := tc.baseChain.RolesOf(
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

// OperatorToStakingProvider returns the staking provider address for the
// operator. If the staking provider has not been registered for the
// operator, the returned address is empty and the boolean flag is set to
// false. If the staking provider has been registered, the address is not
// empty and the boolean flag indicates true.
func (tc *TbtcChain) OperatorToStakingProvider() (chain.Address, bool, error) {
	stakingProvider, err := tc.walletRegistry.OperatorToStakingProvider(tc.key.Address)
	if err != nil {
		return "", false, fmt.Errorf(
			"failed to map operator [%v] to a staking provider: [%v]",
			tc.key.Address,
			err,
		)
	}

	if (stakingProvider == common.Address{}) {
		return "", false, nil
	}

	return chain.Address(stakingProvider.Hex()), true, nil
}

// EligibleStake returns the current value of the staking provider's
// eligible stake. Eligible stake is defined as the currently authorized
// stake minus the pending authorization decrease. Eligible stake
// is what is used for operator's weight in the sortition pool.
// If the authorized stake minus the pending authorization decrease
// is below the minimum authorization, eligible stake is 0.
func (tc *TbtcChain) EligibleStake(stakingProvider chain.Address) (*big.Int, error) {
	eligibleStake, err := tc.walletRegistry.EligibleStake(
		common.HexToAddress(stakingProvider.String()),
	)
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
func (tc *TbtcChain) IsPoolLocked() (bool, error) {
	return tc.sortitionPool.IsLocked()
}

// IsOperatorInPool returns true if the operator is registered in
// the sortition pool.
func (tc *TbtcChain) IsOperatorInPool() (bool, error) {
	return tc.walletRegistry.IsOperatorInPool(tc.key.Address)
}

// IsOperatorUpToDate checks if the operator's authorized stake is in sync
// with operator's weight in the sortition pool.
// If the operator's authorized stake is not in sync with sortition pool
// weight, function returns false.
// If the operator is not in the sortition pool and their authorized stake
// is non-zero, function returns false.
func (tc *TbtcChain) IsOperatorUpToDate() (bool, error) {
	return tc.walletRegistry.IsOperatorUpToDate(tc.key.Address)
}

// JoinSortitionPool executes a transaction to have the operator join the
// sortition pool.
func (tc *TbtcChain) JoinSortitionPool() error {
	_, err := tc.walletRegistry.JoinSortitionPool()
	return err
}

// UpdateOperatorStatus executes a transaction to update the operator's
// state in the sortition pool.
func (tc *TbtcChain) UpdateOperatorStatus() error {
	_, err := tc.walletRegistry.UpdateOperatorStatus(tc.key.Address)
	return err
}

// IsEligibleForRewards checks whether the operator is eligible for rewards
// or not.
func (tc *TbtcChain) IsEligibleForRewards() (bool, error) {
	return tc.sortitionPool.IsEligibleForRewards(tc.key.Address)
}

// Checks whether the operator is able to restore their eligibility for
// rewards right away.
func (tc *TbtcChain) CanRestoreRewardEligibility() (bool, error) {
	return tc.sortitionPool.CanRestoreRewardEligibility(tc.key.Address)
}

// Restores reward eligibility for the operator.
func (tc *TbtcChain) RestoreRewardEligibility() error {
	_, err := tc.sortitionPool.RestoreRewardEligibility(tc.key.Address)
	return err
}

// Returns true if the chaosnet phase is active, false otherwise.
func (tc *TbtcChain) IsChaosnetActive() (bool, error) {
	return tc.sortitionPool.IsChaosnetActive()
}

// Returns true if operator is a beta operator, false otherwise.
// Chaosnet status does not matter.
func (tc *TbtcChain) IsBetaOperator() (bool, error) {
	return tc.sortitionPool.IsBetaOperator(tc.key.Address)
}

// GetOperatorID returns the ID number of the given operator address. An ID
// number of 0 means the operator has not been allocated an ID number yet.
func (tc *TbtcChain) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	return tc.sortitionPool.GetOperatorID(
		common.HexToAddress(operatorAddress.String()),
	)
}

// SelectGroup returns the group members selected for the current group
// selection. The function returns an error if the chain's state does not allow
// for group selection at the moment.
func (tc *TbtcChain) SelectGroup() (*tbtc.GroupSelectionResult, error) {
	operatorsIDs, err := tc.walletRegistry.SelectGroup()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot select group in the sortition pool: [%v]",
			err,
		)
	}

	operatorsAddresses, err := tc.sortitionPool.GetIDOperators(operatorsIDs)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert operators' IDs to addresses: [%v]",
			err,
		)
	}

	// Should not happen as this is guaranteed by the contract but, just in case.
	if len(operatorsIDs) != len(operatorsAddresses) {
		return nil, fmt.Errorf("operators IDs and addresses mismatch")
	}

	ids := make([]chain.OperatorID, len(operatorsIDs))
	addresses := make([]chain.Address, len(operatorsIDs))
	for i := range ids {
		ids[i] = operatorsIDs[i]
		addresses[i] = chain.Address(operatorsAddresses[i].String())
	}

	return &tbtc.GroupSelectionResult{
		OperatorsIDs:       ids,
		OperatorsAddresses: addresses,
	}, nil
}

func (tc *TbtcChain) OnDKGStarted(
	handler func(event *tbtc.DKGStartedEvent),
) subscription.EventSubscription {
	onEvent := func(
		seed *big.Int,
		blockNumber uint64,
	) {
		handler(&tbtc.DKGStartedEvent{
			Seed:        seed,
			BlockNumber: blockNumber,
		})
	}

	return tc.walletRegistry.DkgStartedEvent(nil, nil).OnEvent(onEvent)
}

func (tc *TbtcChain) PastDKGStartedEvents(
	filter *tbtc.DKGStartedEventFilter,
) ([]*tbtc.DKGStartedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var seed []*big.Int

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		seed = filter.Seed
	}

	events, err := tc.walletRegistry.PastDkgStartedEvents(
		startBlock,
		endBlock,
		seed,
	)
	if err != nil {
		return nil, err
	}

	dkgStartedEvents := make([]*tbtc.DKGStartedEvent, len(events))
	for i, event := range events {
		dkgStartedEvents[i] = &tbtc.DKGStartedEvent{
			Seed:        event.Seed,
			BlockNumber: event.Raw.BlockNumber,
		}
	}

	sort.SliceStable(dkgStartedEvents, func(i, j int) bool {
		return dkgStartedEvents[i].BlockNumber < dkgStartedEvents[j].BlockNumber
	})

	return dkgStartedEvents, err
}

func (tc *TbtcChain) OnDKGResultSubmitted(
	handler func(event *tbtc.DKGResultSubmittedEvent),
) subscription.EventSubscription {
	onEvent := func(
		resultHash [32]byte,
		seed *big.Int,
		result ecdsaabi.EcdsaDkgResult,
		blockNumber uint64,
	) {
		tbtcResult, err := convertDkgResultFromAbiType(result)
		if err != nil {
			logger.Errorf(
				"unexpected DKG result in DKGResultSubmitted event: [%v]",
				err,
			)
			return
		}

		handler(&tbtc.DKGResultSubmittedEvent{
			Seed:        seed,
			ResultHash:  resultHash,
			Result:      tbtcResult,
			BlockNumber: blockNumber,
		})
	}

	return tc.walletRegistry.
		DkgResultSubmittedEvent(nil, nil, nil).
		OnEvent(onEvent)
}

// convertDkgResultFromAbiType converts the WalletRegistry-specific DKG
// result to the format applicable for the TBTC application.
func convertDkgResultFromAbiType(
	result ecdsaabi.EcdsaDkgResult,
) (*tbtc.DKGChainResult, error) {
	if err := validateMemberIndex(result.SubmitterMemberIndex); err != nil {
		return nil, fmt.Errorf(
			"unexpected submitter member index: [%v]",
			err,
		)
	}

	signingMembersIndexes := make(
		[]group.MemberIndex,
		len(result.SigningMembersIndices),
	)
	for i, memberIndex := range result.SigningMembersIndices {
		if err := validateMemberIndex(memberIndex); err != nil {
			return nil, fmt.Errorf(
				"unexpected signing member index: [%v]",
				err,
			)
		}

		signingMembersIndexes[i] = group.MemberIndex(memberIndex.Uint64())
	}

	return &tbtc.DKGChainResult{
		SubmitterMemberIndex:     group.MemberIndex(result.SubmitterMemberIndex.Uint64()),
		GroupPublicKey:           result.GroupPubKey,
		MisbehavedMembersIndexes: result.MisbehavedMembersIndices,
		Signatures:               result.Signatures,
		SigningMembersIndexes:    signingMembersIndexes,
		Members:                  result.Members,
		MembersHash:              result.MembersHash,
	}, nil
}

// convertDkgResultToAbiType converts the TBTC-specific DKG result to
// the format applicable for the WalletRegistry ABI.
func convertDkgResultToAbiType(
	result *tbtc.DKGChainResult,
) ecdsaabi.EcdsaDkgResult {
	signingMembersIndices := make([]*big.Int, len(result.SigningMembersIndexes))
	for i, memberIndex := range result.SigningMembersIndexes {
		signingMembersIndices[i] = big.NewInt(int64(memberIndex))
	}

	return ecdsaabi.EcdsaDkgResult{
		SubmitterMemberIndex:     big.NewInt(int64(result.SubmitterMemberIndex)),
		GroupPubKey:              result.GroupPublicKey,
		MisbehavedMembersIndices: result.MisbehavedMembersIndexes,
		Signatures:               result.Signatures,
		SigningMembersIndices:    signingMembersIndices,
		Members:                  result.Members,
		MembersHash:              result.MembersHash,
	}
}

func validateMemberIndex(chainMemberIndex *big.Int) error {
	maxMemberIndex := big.NewInt(group.MaxMemberIndex)
	if chainMemberIndex.Cmp(maxMemberIndex) > 0 {
		return fmt.Errorf("invalid member index value: [%v]", chainMemberIndex)
	}

	return nil
}

func (tc *TbtcChain) OnDKGResultChallenged(
	handler func(event *tbtc.DKGResultChallengedEvent),
) subscription.EventSubscription {
	onEvent := func(
		resultHash [32]byte,
		challenger common.Address,
		reason string,
		blockNumber uint64,
	) {
		handler(&tbtc.DKGResultChallengedEvent{
			ResultHash:  resultHash,
			Challenger:  chain.Address(challenger.Hex()),
			Reason:      reason,
			BlockNumber: blockNumber,
		})
	}

	return tc.walletRegistry.
		DkgResultChallengedEvent(nil, nil, nil).
		OnEvent(onEvent)
}

func (tc *TbtcChain) OnDKGResultApproved(
	handler func(event *tbtc.DKGResultApprovedEvent),
) subscription.EventSubscription {
	onEvent := func(
		resultHash [32]byte,
		approver common.Address,
		blockNumber uint64,
	) {
		handler(&tbtc.DKGResultApprovedEvent{
			ResultHash:  resultHash,
			Approver:    chain.Address(approver.Hex()),
			BlockNumber: blockNumber,
		})
	}

	return tc.walletRegistry.
		DkgResultApprovedEvent(nil, nil, nil).
		OnEvent(onEvent)
}

// AssembleDKGResult assembles the DKG chain result according to the rules
// expected by the given chain.
func (tc *TbtcChain) AssembleDKGResult(
	submitterMemberIndex group.MemberIndex,
	groupPublicKey *ecdsa.PublicKey,
	operatingMembersIndexes []group.MemberIndex,
	misbehavedMembersIndexes []group.MemberIndex,
	signatures map[group.MemberIndex][]byte,
	groupSelectionResult *tbtc.GroupSelectionResult,
) (*tbtc.DKGChainResult, error) {
	serializedGroupPublicKey, err := convertPubKeyToChainFormat(groupPublicKey)
	if err != nil {
		return nil, fmt.Errorf(
			"could not convert group public key to chain format: [%v]",
			err,
		)
	}

	// Sort misbehavedMembersIndexes slice in ascending order as expected
	// by the on-chain contract.
	sort.Slice(misbehavedMembersIndexes[:], func(i, j int) bool {
		return misbehavedMembersIndexes[i] < misbehavedMembersIndexes[j]
	})

	signingMemberIndices, signatureBytes, err := convertSignaturesToChainFormat(
		signatures,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not convert signatures to chain format: [%v]",
			err,
		)
	}

	// Sort operatingOperatorsIDs slice in ascending order as the slice
	// holding the operators IDs used to compute the members hash is
	// expected to be sorted in the same way.
	sort.Slice(operatingMembersIndexes[:], func(i, j int) bool {
		return operatingMembersIndexes[i] < operatingMembersIndexes[j]
	})

	operatingOperatorsIDs := make([]chain.OperatorID, len(operatingMembersIndexes))
	for i, operatingMemberIndex := range operatingMembersIndexes {
		operatingOperatorsIDs[i] =
			groupSelectionResult.OperatorsIDs[operatingMemberIndex-1]
	}

	membersHash, err := computeOperatorsIDsHash(operatingOperatorsIDs)
	if err != nil {
		return nil, fmt.Errorf("could not compute members hash: [%v]", err)
	}

	return &tbtc.DKGChainResult{
		SubmitterMemberIndex:     submitterMemberIndex,
		GroupPublicKey:           serializedGroupPublicKey[:],
		MisbehavedMembersIndexes: misbehavedMembersIndexes,
		Signatures:               signatureBytes,
		SigningMembersIndexes:    signingMemberIndices,
		Members:                  groupSelectionResult.OperatorsIDs,
		MembersHash:              membersHash,
	}, nil
}

func (tc *TbtcChain) SubmitDKGResult(
	dkgResult *tbtc.DKGChainResult,
) error {
	_, err := tc.walletRegistry.SubmitDkgResult(
		convertDkgResultToAbiType(dkgResult),
	)

	return err
}

// computeOperatorsIDsHash computes the keccak256 hash for the given list
// of operators IDs.
func computeOperatorsIDsHash(operatorsIDs chain.OperatorIDs) ([32]byte, error) {
	uint32SliceType, err := abi.NewType("uint32[]", "uint32[]", nil)
	if err != nil {
		return [32]byte{}, err
	}

	bytes, err := abi.Arguments{{Type: uint32SliceType}}.Pack(operatorsIDs)
	if err != nil {
		return [32]byte{}, err
	}

	return crypto.Keccak256Hash(bytes), nil
}

// convertSignaturesToChainFormat converts signatures map to two slices. The
// first slice contains indices of members from the map, sorted in ascending order
// as required by the contract. The second slice is a slice of concatenated
// signatures. Signatures and member indices are returned in the matching order.
// It requires each signature to be exactly 65-byte long.
func convertSignaturesToChainFormat(
	signatures map[group.MemberIndex][]byte,
) ([]group.MemberIndex, []byte, error) {
	membersIndexes := make([]group.MemberIndex, 0)
	for memberIndex := range signatures {
		membersIndexes = append(membersIndexes, memberIndex)
	}

	sort.Slice(membersIndexes, func(i, j int) bool {
		return membersIndexes[i] < membersIndexes[j]
	})

	signatureSize := 65

	var signaturesSlice []byte

	for _, memberIndex := range membersIndexes {
		signature := signatures[memberIndex]

		if len(signature) != signatureSize {
			return nil, nil, fmt.Errorf(
				"invalid signature size for member [%v] got [%d] bytes but [%d] bytes required",
				memberIndex,
				len(signature),
				signatureSize,
			)
		}

		signaturesSlice = append(signaturesSlice, signature...)
	}

	return membersIndexes, signaturesSlice, nil
}

// convertPubKeyToChainFormat takes X and Y coordinates of a signer's public key
// and concatenates it to a 64-byte long array. If any of coordinates is shorter
// than 32-byte it is preceded with zeros.
func convertPubKeyToChainFormat(publicKey *ecdsa.PublicKey) ([64]byte, error) {
	var serialized [64]byte

	x, err := byteutils.LeftPadTo32Bytes(publicKey.X.Bytes())
	if err != nil {
		return serialized, err
	}

	y, err := byteutils.LeftPadTo32Bytes(publicKey.Y.Bytes())
	if err != nil {
		return serialized, err
	}

	serializedBytes := append(x, y...)

	copy(serialized[:], serializedBytes)

	return serialized, nil
}

func (tc *TbtcChain) GetDKGState() (tbtc.DKGState, error) {
	walletCreationState, err := tc.walletRegistry.GetWalletCreationState()
	if err != nil {
		return 0, err
	}

	var state tbtc.DKGState

	switch walletCreationState {
	case 0:
		state = tbtc.Idle
	case 1:
		state = tbtc.AwaitingSeed
	case 2:
		state = tbtc.AwaitingResult
	case 3:
		state = tbtc.Challenge
	default:
		err = fmt.Errorf(
			"unexpected wallet creation state: [%v]",
			walletCreationState,
		)
	}

	return state, err
}

// CalculateDKGResultSignatureHash calculates a 32-byte hash that is used
// to produce a signature supporting the given groupPublicKey computed
// as result of the given DKG process. The misbehavedMembersIndexes parameter
// should contain indexes of members that were considered as misbehaved
// during the DKG process. The startBlock argument is the block at which
// the given DKG process started.
func (tc *TbtcChain) CalculateDKGResultSignatureHash(
	groupPublicKey *ecdsa.PublicKey,
	misbehavedMembersIndexes []group.MemberIndex,
	startBlock uint64,
) (dkg.ResultSignatureHash, error) {
	groupPublicKeyBytes := secp256k1.Marshal(groupPublicKey)
	// Crop the 04 prefix as the calculateDKGResultSignatureHash function
	// expects an unprefixed 64-byte public key,
	unprefixedGroupPublicKeyBytes := groupPublicKeyBytes[1:]

	// Sort misbehavedMembersIndexes slice in ascending order as expected
	// by the calculateDKGResultSignatureHash function.
	sort.Slice(misbehavedMembersIndexes[:], func(i, j int) bool {
		return misbehavedMembersIndexes[i] < misbehavedMembersIndexes[j]
	})

	return calculateDKGResultSignatureHash(
		tc.chainID,
		unprefixedGroupPublicKeyBytes,
		misbehavedMembersIndexes,
		big.NewInt(int64(startBlock)),
	)
}

// calculateDKGResultSignatureHash computes the keccak256 hash for the given DKG
// result parameters. It expects that the groupPublicKey is a 64-byte uncompressed
// public key without the 04 prefix and misbehavedMembersIndexes slice is
// sorted in ascending order. Those expectations are forced by the contract.
func calculateDKGResultSignatureHash(
	chainID *big.Int,
	groupPublicKey []byte,
	misbehavedMembersIndexes []group.MemberIndex,
	startBlock *big.Int,
) (dkg.ResultSignatureHash, error) {
	publicKeySize := 64

	if len(groupPublicKey) != publicKeySize {
		return dkg.ResultSignatureHash{}, fmt.Errorf(
			"wrong group public key length",
		)
	}

	uint256Type, err := abi.NewType("uint256", "uint256", nil)
	if err != nil {
		return dkg.ResultSignatureHash{}, err
	}
	bytesType, err := abi.NewType("bytes", "bytes", nil)
	if err != nil {
		return dkg.ResultSignatureHash{}, err
	}
	uint8SliceType, err := abi.NewType("uint8[]", "uint8[]", nil)
	if err != nil {
		return dkg.ResultSignatureHash{}, err
	}

	bytes, err := abi.Arguments{
		{Type: uint256Type},
		{Type: bytesType},
		{Type: uint8SliceType},
		{Type: uint256Type},
	}.Pack(
		chainID,
		groupPublicKey,
		misbehavedMembersIndexes,
		startBlock,
	)
	if err != nil {
		return dkg.ResultSignatureHash{}, err
	}

	return dkg.ResultSignatureHash(crypto.Keccak256Hash(bytes)), nil
}

func (tc *TbtcChain) IsDKGResultValid(
	dkgResult *tbtc.DKGChainResult,
) (bool, error) {
	outcome, err := tc.walletRegistry.IsDkgResultValid(
		convertDkgResultToAbiType(dkgResult),
	)
	if err != nil {
		return false, fmt.Errorf("cannot check result validity: [%v]", err)
	}

	return parseDkgResultValidationOutcome(&outcome)
}

// parseDkgResultValidationOutcome parses the DKG validation outcome and returns
// a boolean indicating whether the result is valid or not. The outcome parameter
// must be a pointer to a struct containing a boolean flag as the first field.
//
// TODO: Find a better way to get the validity flag. This would require changes
// in the contracts binding generator.
func parseDkgResultValidationOutcome(
	outcome interface{},
) (bool, error) {
	value := reflect.ValueOf(outcome)
	switch value.Kind() {
	case reflect.Pointer:
	default:
		return false, fmt.Errorf("result validation outcome is not a pointer")
	}

	field := value.Elem().Field(0)
	switch field.Kind() {
	case reflect.Bool:
		return field.Bool(), nil
	default:
		return false, fmt.Errorf("cannot parse result validation outcome")
	}
}

func (tc *TbtcChain) ChallengeDKGResult(dkgResult *tbtc.DKGChainResult) error {
	_, err := tc.walletRegistry.ChallengeDkgResult(
		convertDkgResultToAbiType(dkgResult),
	)

	return err
}

func (tc *TbtcChain) ApproveDKGResult(dkgResult *tbtc.DKGChainResult) error {
	result := convertDkgResultToAbiType(dkgResult)

	gasEstimate, err := tc.walletRegistry.ApproveDkgResultGasEstimate(result)
	if err != nil {
		return err
	}

	// The original estimate for this contract call turned out to be too low.
	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.walletRegistry.ApproveDkgResult(
		result,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func (tc *TbtcChain) DKGParameters() (*tbtc.DKGParameters, error) {
	parameters, err := tc.walletRegistry.DkgParameters()
	if err != nil {
		return nil, err
	}

	return &tbtc.DKGParameters{
		SubmissionTimeoutBlocks:       parameters.ResultSubmissionTimeout.Uint64(),
		ChallengePeriodBlocks:         parameters.ResultChallengePeriodLength.Uint64(),
		ApprovePrecedencePeriodBlocks: parameters.SubmitterPrecedencePeriodLength.Uint64(),
	}, nil
}

func (tc *TbtcChain) OnInactivityClaimed(
	handler func(event *tbtc.InactivityClaimedEvent),
) subscription.EventSubscription {
	onEvent := func(
		walletID [32]byte,
		nonce *big.Int,
		notifier common.Address,
		blockNumber uint64,
	) {
		handler(&tbtc.InactivityClaimedEvent{
			WalletID:    walletID,
			Nonce:       nonce,
			Notifier:    chain.Address(notifier.Hex()),
			BlockNumber: blockNumber,
		})
	}

	return tc.walletRegistry.InactivityClaimedEvent(nil, nil).OnEvent(onEvent)
}

func (tc *TbtcChain) AssembleInactivityClaim(
	walletID [32]byte,
	inactiveMembersIndices []group.MemberIndex,
	signatures map[group.MemberIndex][]byte,
	heartbeatFailed bool,
) (
	*tbtc.InactivityClaim,
	error,
) {
	signingMemberIndices, signatureBytes, err := convertSignaturesToChainFormat(
		signatures,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not convert signatures to chain format: [%v]",
			err,
		)
	}

	return &tbtc.InactivityClaim{
		WalletID:               walletID,
		InactiveMembersIndices: inactiveMembersIndices,
		HeartbeatFailed:        heartbeatFailed,
		Signatures:             signatureBytes,
		SigningMembersIndices:  signingMemberIndices,
	}, nil
}

// convertInactivityClaimToAbiType converts the TBTC-specific inactivity claim
// to the format applicable for the WalletRegistry ABI.
func convertInactivityClaimToAbiType(
	claim *tbtc.InactivityClaim,
) ecdsaabi.EcdsaInactivityClaim {
	inactiveMembersIndices := make([]*big.Int, len(claim.InactiveMembersIndices))
	for i, memberIndex := range claim.InactiveMembersIndices {
		inactiveMembersIndices[i] = big.NewInt(int64(memberIndex))
	}

	signingMembersIndices := make([]*big.Int, len(claim.SigningMembersIndices))
	for i, memberIndex := range claim.SigningMembersIndices {
		signingMembersIndices[i] = big.NewInt(int64(memberIndex))
	}

	return ecdsaabi.EcdsaInactivityClaim{
		WalletID:               claim.WalletID,
		InactiveMembersIndices: inactiveMembersIndices,
		HeartbeatFailed:        claim.HeartbeatFailed,
		Signatures:             claim.Signatures,
		SigningMembersIndices:  signingMembersIndices,
	}
}

func (tc *TbtcChain) SubmitInactivityClaim(
	claim *tbtc.InactivityClaim,
	nonce *big.Int,
	groupMembers []uint32,
) error {
	_, err := tc.walletRegistry.NotifyOperatorInactivity(
		convertInactivityClaimToAbiType(claim),
		nonce,
		groupMembers,
	)

	return err
}

func (tc *TbtcChain) CalculateInactivityClaimHash(
	claim *inactivity.ClaimPreimage,
) (inactivity.ClaimHash, error) {
	walletPublicKeyBytes := secp256k1.Marshal(claim.WalletPublicKey)
	// Crop the 04 prefix as the calculateInactivityClaimHash function expects
	// an unprefixed 64-byte public key,
	unprefixedGroupPublicKeyBytes := walletPublicKeyBytes[1:]

	// The type representing inactive member index should be `big.Int` as the
	// smart contract reading the calculated hash uses `uint256` for inactive
	// member indexes.
	inactiveMembersIndexes := make([]*big.Int, len(claim.InactiveMembersIndexes))
	for i, index := range claim.InactiveMembersIndexes {
		inactiveMembersIndexes[i] = big.NewInt(int64(index))
	}

	return calculateInactivityClaimHash(
		tc.chainID,
		claim.Nonce,
		unprefixedGroupPublicKeyBytes,
		inactiveMembersIndexes,
		claim.HeartbeatFailed,
	)
}

func calculateInactivityClaimHash(
	chainID *big.Int,
	nonce *big.Int,
	walletPublicKey []byte,
	inactiveMembersIndexes []*big.Int,
	heartbeatFailed bool,
) (inactivity.ClaimHash, error) {
	publicKeySize := 64

	if len(walletPublicKey) != publicKeySize {
		return inactivity.ClaimHash{}, fmt.Errorf(
			"wrong wallet public key length",
		)
	}

	uint256Type, err := abi.NewType("uint256", "uint256", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	bytesType, err := abi.NewType("bytes", "bytes", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	uint256SliceType, err := abi.NewType("uint256[]", "uint256[]", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}
	boolType, err := abi.NewType("bool", "bool", nil)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}

	bytes, err := abi.Arguments{
		{Type: uint256Type},
		{Type: uint256Type},
		{Type: bytesType},
		{Type: uint256SliceType},
		{Type: boolType},
	}.Pack(
		chainID,
		nonce,
		walletPublicKey,
		inactiveMembersIndexes,
		heartbeatFailed,
	)
	if err != nil {
		return inactivity.ClaimHash{}, err
	}

	return inactivity.ClaimHash(crypto.Keccak256Hash(bytes)), nil
}

func (tc *TbtcChain) GetInactivityClaimNonce(
	walletID [32]byte,
) (*big.Int, error) {
	nonce, err := tc.walletRegistry.InactivityClaimNonce(walletID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get inactivity claim nonce: [%w]",
			err,
		)
	}

	return nonce, nil
}

func (tc *TbtcChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var depositor []common.Address
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock

		for _, d := range filter.Depositor {
			depositor = append(depositor, common.HexToAddress(d.String()))
		}

		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastDepositRevealedEvents(
		startBlock,
		endBlock,
		depositor,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.DepositRevealedEvent, 0)
	for _, event := range events {
		var vault *chain.Address
		if event.Vault != [20]byte{} {
			v := chain.Address(event.Vault.Hex())
			vault = &v
		}

		convertedEvent := &tbtc.DepositRevealedEvent{
			// We can map the event.FundingTxHash field directly to the
			// bitcoin.Hash type. This is because event.FundingTxHash is
			// a [32]byte type representing a hash in the bitcoin.InternalByteOrder,
			// just as bitcoin.Hash assumes.
			FundingTxHash:       event.FundingTxHash,
			FundingOutputIndex:  event.FundingOutputIndex,
			Depositor:           chain.Address(event.Depositor.Hex()),
			Amount:              event.Amount,
			BlindingFactor:      event.BlindingFactor,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			RefundPublicKeyHash: event.RefundPubKeyHash,
			RefundLocktime:      event.RefundLocktime,
			Vault:               vault,
			BlockNumber:         event.Raw.BlockNumber,
		}

		convertedEvents = append(convertedEvents, convertedEvent)
	}

	sort.SliceStable(
		convertedEvents,
		func(i, j int) bool {
			return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
		},
	)

	return convertedEvents, err
}

func (tc *TbtcChain) PastRedemptionRequestedEvents(
	filter *tbtc.RedemptionRequestedEventFilter,
) ([]*tbtc.RedemptionRequestedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var redeemers []common.Address
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock

		for _, r := range filter.Redeemer {
			redeemers = append(redeemers, common.HexToAddress(r.String()))
		}

		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastRedemptionRequestedEvents(
		startBlock,
		endBlock,
		walletPublicKeyHash,
		redeemers,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.RedemptionRequestedEvent, 0)
	for _, event := range events {
		redeemerOutputScript, err := bitcoin.NewScriptFromVarLenData(
			event.RedeemerOutputScript,
		)
		if err != nil {
			return nil, err
		}

		convertedEvent := &tbtc.RedemptionRequestedEvent{
			WalletPublicKeyHash:  event.WalletPubKeyHash,
			RedeemerOutputScript: redeemerOutputScript,
			Redeemer:             chain.Address(event.Redeemer.Hex()),
			RequestedAmount:      event.RequestedAmount,
			TreasuryFee:          event.TreasuryFee,
			TxMaxFee:             event.TreasuryFee,
			BlockNumber:          event.Raw.BlockNumber,
		}

		convertedEvents = append(convertedEvents, convertedEvent)
	}

	sort.SliceStable(
		convertedEvents,
		func(i, j int) bool {
			return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
		},
	)

	return convertedEvents, err
}

func (tc *TbtcChain) GetDepositRequest(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) (*tbtc.DepositChainRequest, bool, error) {
	depositKey := buildDepositKey(fundingTxHash, fundingOutputIndex)
	depositCacheKey := depositKey.Text(16)

	tc.sweptDepositsCache.Sweep()
	if cachedRequest, ok := tc.sweptDepositsCache.Get(depositCacheKey); ok {
		return cachedRequest, true, nil
	}

	chainRequest, err := tc.bridge.Deposits(depositKey)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get deposit request for key [0x%x]: [%v]",
			depositKey.Text(16),
			err,
		)
	}

	// Deposit not found.
	if chainRequest.RevealedAt == 0 {
		return nil, false, nil
	}

	var vault *chain.Address
	if chainRequest.Vault != [20]byte{} {
		v := chain.Address(chainRequest.Vault.Hex())
		vault = &v
	}

	var extraData *[32]byte
	if chainRequest.ExtraData != [32]byte{} {
		extraData = &chainRequest.ExtraData
	}

	request := &tbtc.DepositChainRequest{
		Depositor:   chain.Address(chainRequest.Depositor.Hex()),
		Amount:      chainRequest.Amount,
		RevealedAt:  time.Unix(int64(chainRequest.RevealedAt), 0),
		Vault:       vault,
		TreasuryFee: chainRequest.TreasuryFee,
		SweptAt:     time.Unix(int64(chainRequest.SweptAt), 0),
		ExtraData:   extraData,
	}

	// If the request was swept on-chain, there is a guarantee that no
	// further changes will occur regarding its parameters.
	// Such a request can be cached.
	if isSwept := request.SweptAt.Unix() != 0; isSwept {
		tc.sweptDepositsCache.Add(depositCacheKey, request)
	}

	return request, true, nil
}

func (tc *TbtcChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var ecdsaWalletID [][32]byte
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		ecdsaWalletID = filter.EcdsaWalletID
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastNewWalletRegisteredEvents(
		startBlock,
		endBlock,
		ecdsaWalletID,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.NewWalletRegisteredEvent, 0)
	for _, event := range events {
		convertedEvent := &tbtc.NewWalletRegisteredEvent{
			EcdsaWalletID:       event.EcdsaWalletID,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			BlockNumber:         event.Raw.BlockNumber,
		}

		convertedEvents = append(convertedEvents, convertedEvent)
	}

	sort.SliceStable(
		convertedEvents,
		func(i, j int) bool {
			return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
		},
	)

	return convertedEvents, err
}

func (tc *TbtcChain) CalculateWalletID(
	walletPublicKey *ecdsa.PublicKey,
) ([32]byte, error) {
	return calculateWalletID(walletPublicKey)
}

func calculateWalletID(walletPublicKey *ecdsa.PublicKey) ([32]byte, error) {
	walletPublicKeyBytes, err := convertPubKeyToChainFormat(walletPublicKey)
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"error while converting wallet public key to chain format: [%v]",
			err,
		)
	}

	return crypto.Keccak256Hash(walletPublicKeyBytes[:]), nil
}

func (tc *TbtcChain) IsWalletRegistered(EcdsaWalletID [32]byte) (bool, error) {
	isWalletRegistered, err := tc.walletRegistry.IsWalletRegistered(
		EcdsaWalletID,
	)
	if err != nil {
		return false, fmt.Errorf(
			"cannot check if wallet with ECDSA ID [0x%x] is registered: [%v]",
			EcdsaWalletID,
			err,
		)
	}

	return isWalletRegistered, nil
}

func (tc *TbtcChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*tbtc.WalletChainData, error) {
	wallet, err := tc.bridge.Wallets(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get wallet for public key hash [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	// Wallet not found.
	if wallet.CreatedAt == 0 {
		return nil, fmt.Errorf(
			"no wallet for public key hash [0x%x]",
			wallet,
		)
	}

	walletState, err := parseWalletState(wallet.State)
	if err != nil {
		return nil, fmt.Errorf("cannot parse wallet state: [%v]", err)
	}

	return &tbtc.WalletChainData{
		EcdsaWalletID:                          wallet.EcdsaWalletID,
		MainUtxoHash:                           wallet.MainUtxoHash,
		PendingRedemptionsValue:                wallet.PendingRedemptionsValue,
		CreatedAt:                              time.Unix(int64(wallet.CreatedAt), 0),
		MovingFundsRequestedAt:                 time.Unix(int64(wallet.MovingFundsRequestedAt), 0),
		ClosingStartedAt:                       time.Unix(int64(wallet.ClosingStartedAt), 0),
		PendingMovedFundsSweepRequestsCount:    wallet.PendingMovedFundsSweepRequestsCount,
		State:                                  walletState,
		MovingFundsTargetWalletsCommitmentHash: wallet.MovingFundsTargetWalletsCommitmentHash,
	}, nil
}

func (tc *TbtcChain) OnWalletClosed(
	handler func(event *tbtc.WalletClosedEvent),
) subscription.EventSubscription {
	onEvent := func(
		walletID [32]byte,
		blockNumber uint64,
	) {
		handler(&tbtc.WalletClosedEvent{
			WalletID:    walletID,
			BlockNumber: blockNumber,
		})
	}
	return tc.walletRegistry.WalletClosedEvent(nil, nil).OnEvent(onEvent)
}

func (tc *TbtcChain) ComputeMainUtxoHash(
	mainUtxo *bitcoin.UnspentTransactionOutput,
) [32]byte {
	return computeMainUtxoHash(mainUtxo)
}

func computeMainUtxoHash(mainUtxo *bitcoin.UnspentTransactionOutput) [32]byte {
	outputIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(outputIndexBytes, mainUtxo.Outpoint.OutputIndex)

	valueBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(valueBytes, uint64(mainUtxo.Value))

	mainUtxoHash := crypto.Keccak256Hash(
		append(
			append(
				mainUtxo.Outpoint.TransactionHash[:],
				outputIndexBytes...,
			), valueBytes...,
		),
	)

	return mainUtxoHash
}

func (tc *TbtcChain) ComputeMovingFundsCommitmentHash(
	targetWallets [][20]byte,
) [32]byte {
	return computeMovingFundsCommitmentHash(targetWallets)
}

func computeMovingFundsCommitmentHash(targetWallets [][20]byte) [32]byte {
	packedWallets := []byte{}

	for _, wallet := range targetWallets {
		packedWallets = append(packedWallets, wallet[:]...)
		// Each wallet hash must be padded with 12 zero bytes following the
		// actual hash.
		packedWallets = append(packedWallets, make([]byte, 12)...)
	}

	return crypto.Keccak256Hash(packedWallets)
}

func (tc *TbtcChain) BuildDepositKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) *big.Int {
	return buildDepositKey(fundingTxHash, fundingOutputIndex)
}

func (tc *TbtcChain) BuildRedemptionKey(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*big.Int, error) {
	return buildRedemptionKey(walletPublicKeyHash, redeemerOutputScript)
}

func (tc *TbtcChain) GetDepositParameters() (tbtc.DepositParameters, error) {
	parameters, err := tc.bridge.DepositParameters()
	if err != nil {
		return tbtc.DepositParameters{}, err
	}

	return tbtc.DepositParameters{
		DustThreshold:      parameters.DepositDustThreshold,
		TreasuryFeeDivisor: parameters.DepositTreasuryFeeDivisor,
		TxMaxFee:           parameters.DepositTxMaxFee,
		RevealAheadPeriod:  parameters.DepositRevealAheadPeriod,
	}, nil
}

func (tc *TbtcChain) GetPendingRedemptionRequest(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*tbtc.RedemptionRequest, bool, error) {
	redemptionKey, err := buildRedemptionKey(walletPublicKeyHash, redeemerOutputScript)
	if err != nil {
		return nil, false, fmt.Errorf("cannot build redemption key: [%v]", err)
	}

	redemptionRequest, err := tc.bridge.PendingRedemptions(redemptionKey)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get pending redemption request for key [0x%x]: [%v]",
			redemptionKey.Text(16),
			err,
		)
	}

	// Redemption not found.
	if redemptionRequest.RequestedAt == 0 {
		return nil, false, nil
	}

	return &tbtc.RedemptionRequest{
		Redeemer:             chain.Address(redemptionRequest.Redeemer.Hex()),
		RedeemerOutputScript: redeemerOutputScript,
		RequestedAmount:      redemptionRequest.RequestedAmount,
		TreasuryFee:          redemptionRequest.TreasuryFee,
		TxMaxFee:             redemptionRequest.TxMaxFee,
		RequestedAt:          time.Unix(int64(redemptionRequest.RequestedAt), 0),
	}, true, nil
}

func (tc *TbtcChain) SubmitRedemptionProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	walletPublicKeyHash [20]byte,
) error {
	bitcoinTxInfo := tbtcabi.BitcoinTxInfo3{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
	redemptionProof := tbtcabi.BitcoinTxProof2{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   big.NewInt(int64(proof.TxIndexInBlock)),
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	utxo := tbtcabi.BitcoinTxUTXO2{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	gasEstimate, err := tc.maintainerProxy.SubmitRedemptionProofGasEstimate(
		bitcoinTxInfo,
		redemptionProof,
		utxo,
		walletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.maintainerProxy.SubmitRedemptionProof(
		bitcoinTxInfo,
		redemptionProof,
		utxo,
		walletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func buildRedemptionKey(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (*big.Int, error) {
	// The Bridge contract builds the redemption key using the length-prefixed
	// redeemer output script.
	prefixedRedeemerOutputScript, err := redeemerOutputScript.ToVarLenData()
	if err != nil {
		return nil, fmt.Errorf("cannot build prefixed redeemer output script: [%v]", err)
	}

	redeemerOutputScriptHash := crypto.Keccak256Hash(prefixedRedeemerOutputScript)

	redemptionKey := crypto.Keccak256Hash(
		append(redeemerOutputScriptHash[:], walletPublicKeyHash[:]...),
	)

	return redemptionKey.Big(), nil
}

func (tc *TbtcChain) TxProofDifficultyFactor() (*big.Int, error) {
	return tc.bridge.TxProofDifficultyFactor()
}

func (tc *TbtcChain) SubmitDepositSweepProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	vault common.Address,
) error {
	bitcoinTxInfo := tbtcabi.BitcoinTxInfo3{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
	sweepProof := tbtcabi.BitcoinTxProof2{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   big.NewInt(int64(proof.TxIndexInBlock)),
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	utxo := tbtcabi.BitcoinTxUTXO2{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	gasEstimate, err := tc.maintainerProxy.SubmitDepositSweepProofGasEstimate(
		bitcoinTxInfo,
		sweepProof,
		utxo,
		vault,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.maintainerProxy.SubmitDepositSweepProof(
		bitcoinTxInfo,
		sweepProof,
		utxo,
		vault,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func (tc *TbtcChain) GetRedemptionParameters() (tbtc.RedemptionParameters, error) {
	parameters, err := tc.bridge.RedemptionParameters()
	if err != nil {
		return tbtc.RedemptionParameters{}, err
	}

	return tbtc.RedemptionParameters{
		DustThreshold:                   parameters.RedemptionDustThreshold,
		TreasuryFeeDivisor:              parameters.RedemptionTreasuryFeeDivisor,
		TxMaxFee:                        parameters.RedemptionTxMaxFee,
		TxMaxTotalFee:                   parameters.RedemptionTxMaxTotalFee,
		Timeout:                         parameters.RedemptionTimeout,
		TimeoutSlashingAmount:           parameters.RedemptionTimeoutSlashingAmount,
		TimeoutNotifierRewardMultiplier: parameters.RedemptionTimeoutNotifierRewardMultiplier,
	}, nil
}

func (tc *TbtcChain) GetWalletParameters() (tbtc.WalletParameters, error) {
	parameters, err := tc.bridge.WalletParameters()
	if err != nil {
		return tbtc.WalletParameters{}, err
	}

	return tbtc.WalletParameters{
		CreationPeriod:        parameters.WalletCreationPeriod,
		CreationMinBtcBalance: parameters.WalletCreationMinBtcBalance,
		CreationMaxBtcBalance: parameters.WalletCreationMaxBtcBalance,
		ClosureMinBtcBalance:  parameters.WalletClosureMinBtcBalance,
		MaxAge:                parameters.WalletMaxAge,
		MaxBtcTransfer:        parameters.WalletMaxBtcTransfer,
		ClosingPeriod:         parameters.WalletClosingPeriod,
	}, nil
}

func (tc *TbtcChain) GetLiveWalletsCount() (uint32, error) {
	return tc.bridge.LiveWalletsCount()
}

func (tc *TbtcChain) PastMovingFundsCommitmentSubmittedEvents(
	filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
) ([]*tbtc.MovingFundsCommitmentSubmittedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastMovingFundsCommitmentSubmittedEvents(
		startBlock,
		endBlock,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.MovingFundsCommitmentSubmittedEvent, 0)
	for _, event := range events {
		convertedEvent := &tbtc.MovingFundsCommitmentSubmittedEvent{
			WalletPublicKeyHash: event.WalletPubKeyHash,
			TargetWallets:       event.TargetWallets,
			Submitter:           chain.Address(event.Submitter.Hex()),
			BlockNumber:         event.Raw.BlockNumber,
		}

		convertedEvents = append(convertedEvents, convertedEvent)
	}

	sort.SliceStable(
		convertedEvents,
		func(i, j int) bool {
			return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
		},
	)

	return convertedEvents, err
}

func (tc *TbtcChain) PastMovingFundsCompletedEvents(
	filter *tbtc.MovingFundsCompletedEventFilter,
) ([]*tbtc.MovingFundsCompletedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.bridge.PastMovingFundsCompletedEvents(
		startBlock,
		endBlock,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.MovingFundsCompletedEvent, 0)
	for _, event := range events {
		convertedEvent := &tbtc.MovingFundsCompletedEvent{
			WalletPublicKeyHash: event.WalletPubKeyHash,
			MovingFundsTxHash:   event.MovingFundsTxHash,
			BlockNumber:         event.Raw.BlockNumber,
		}

		convertedEvents = append(convertedEvents, convertedEvent)
	}

	sort.SliceStable(
		convertedEvents,
		func(i, j int) bool {
			return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
		},
	)

	return convertedEvents, err
}

func buildDepositKey(
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
) *big.Int {
	fundingOutputIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fundingOutputIndexBytes, fundingOutputIndex)

	depositKey := crypto.Keccak256Hash(
		append(fundingTxHash[:], fundingOutputIndexBytes...),
	)

	return depositKey.Big()
}

func convertDepositSweepProposalToAbiType(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
) tbtcabi.WalletProposalValidatorDepositSweepProposal {
	depositsKeys := make(
		[]tbtcabi.WalletProposalValidatorDepositKey,
		len(proposal.DepositsKeys),
	)

	for i, depositKey := range proposal.DepositsKeys {
		// We can map the depositKey.FundingTxHash field directly to the
		// [32]byte type. This is because depositKey.FundingTxHash is
		// a bitcoin.Hash type representing a hash in the
		// bitcoin.InternalByteOrder, just as the on-chain contract assumes.
		depositsKeys[i] = tbtcabi.WalletProposalValidatorDepositKey{
			FundingTxHash:      depositKey.FundingTxHash,
			FundingOutputIndex: depositKey.FundingOutputIndex,
		}
	}

	return tbtcabi.WalletProposalValidatorDepositSweepProposal{
		WalletPubKeyHash:     walletPublicKeyHash,
		DepositsKeys:         depositsKeys,
		SweepTxFee:           proposal.SweepTxFee,
		DepositsRevealBlocks: proposal.DepositsRevealBlocks,
	}
}

func parseWalletState(value uint8) (tbtc.WalletState, error) {
	switch value {
	case 0:
		return tbtc.StateUnknown, nil
	case 1:
		return tbtc.StateLive, nil
	case 2:
		return tbtc.StateMovingFunds, nil
	case 3:
		return tbtc.StateClosing, nil
	case 4:
		return tbtc.StateClosed, nil
	case 5:
		return tbtc.StateTerminated, nil
	default:
		return 0, fmt.Errorf("unexpected wallet state value: [%v]", value)
	}
}

func (tc *TbtcChain) ValidateDepositSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
	depositsExtraInfo []struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	dei := make([]tbtcabi.WalletProposalValidatorDepositExtraInfo, len(depositsExtraInfo))
	for i, depositExtraInfo := range depositsExtraInfo {
		fundingTx := tbtcabi.BitcoinTxInfo2{
			Version:      depositExtraInfo.FundingTx.SerializeVersion(),
			InputVector:  depositExtraInfo.FundingTx.SerializeInputs(),
			OutputVector: depositExtraInfo.FundingTx.SerializeOutputs(),
			Locktime:     depositExtraInfo.FundingTx.SerializeLocktime(),
		}

		dei[i] = tbtcabi.WalletProposalValidatorDepositExtraInfo{
			FundingTx:        fundingTx,
			BlindingFactor:   depositExtraInfo.Deposit.BlindingFactor,
			WalletPubKeyHash: depositExtraInfo.Deposit.WalletPublicKeyHash,
			RefundPubKeyHash: depositExtraInfo.Deposit.RefundPublicKeyHash,
			RefundLocktime:   depositExtraInfo.Deposit.RefundLocktime,
		}
	}

	valid, err := tc.walletProposalValidator.ValidateDepositSweepProposal(
		convertDepositSweepProposalToAbiType(walletPublicKeyHash, proposal),
		dei,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateDepositSweepProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func (tc *TbtcChain) GetDepositSweepMaxSize() (uint16, error) {
	return tc.walletProposalValidator.DEPOSITSWEEPMAXSIZE()
}

func (tc *TbtcChain) SubmitMovingFundsCommitment(
	walletPublicKeyHash [20]byte,
	walletMainUTXO bitcoin.UnspentTransactionOutput,
	walletMembersIDs []uint32,
	walletMemberIndex uint32,
	targetWallets [][20]byte,
) error {
	mainUtxo := tbtcabi.BitcoinTxUTXO{
		TxHash:        walletMainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: walletMainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(walletMainUTXO.Value),
	}
	_, err := tc.bridge.SubmitMovingFundsCommitment(
		walletPublicKeyHash,
		mainUtxo,
		walletMembersIDs,
		big.NewInt(int64(walletMemberIndex)),
		targetWallets,
	)
	return err
}

func (tc *TbtcChain) SubmitMovingFundsProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
	walletPublicKeyHash [20]byte,
) error {
	bitcoinTxInfo := tbtcabi.BitcoinTxInfo3{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
	movingFundsProof := tbtcabi.BitcoinTxProof2{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   big.NewInt(int64(proof.TxIndexInBlock)),
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	utxo := tbtcabi.BitcoinTxUTXO2{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	gasEstimate, err := tc.maintainerProxy.SubmitMovingFundsProofGasEstimate(
		bitcoinTxInfo,
		movingFundsProof,
		utxo,
		walletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.maintainerProxy.SubmitMovingFundsProof(
		bitcoinTxInfo,
		movingFundsProof,
		utxo,
		walletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func (tc *TbtcChain) SubmitMovedFundsSweepProofWithReimbursement(
	transaction *bitcoin.Transaction,
	proof *bitcoin.SpvProof,
	mainUTXO bitcoin.UnspentTransactionOutput,
) error {
	bitcoinTxInfo := tbtcabi.BitcoinTxInfo3{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
	movedFundsSweepProof := tbtcabi.BitcoinTxProof2{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   big.NewInt(int64(proof.TxIndexInBlock)),
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	utxo := tbtcabi.BitcoinTxUTXO2{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	gasEstimate, err := tc.maintainerProxy.SubmitMovedFundsSweepProofGasEstimate(
		bitcoinTxInfo,
		movedFundsSweepProof,
		utxo,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low and the call
	// fails on reimbursing the submitter. Example:
	// 0xe27a92883e0e64da8a3a54a15a260ea2f4d3d48470129ac5c09bfe9637d7e114
	// Here we add a 20% margin to overcome the gas problems.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.maintainerProxy.SubmitMovedFundsSweepProof(
		bitcoinTxInfo,
		movedFundsSweepProof,
		utxo,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

func (tc *TbtcChain) ValidateMovedFundsSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.MovedFundsSweepProposal,
) error {
	abiProposal := tbtcabi.WalletProposalValidatorMovedFundsSweepProposal{
		WalletPubKeyHash:         walletPublicKeyHash,
		MovingFundsTxHash:        proposal.MovingFundsTxHash,
		MovingFundsTxOutputIndex: proposal.MovingFundsTxOutputIndex,
		MovedFundsSweepTxFee:     proposal.SweepTxFee,
	}

	valid, err := tc.walletProposalValidator.ValidateMovedFundsSweepProposal(
		abiProposal,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateMovedFundsSweepProposal` returns
	// true or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func (tc *TbtcChain) ValidateRedemptionProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.RedemptionProposal,
) error {
	abiProposal, err := convertRedemptionProposalToAbiType(
		walletPublicKeyHash,
		proposal,
	)
	if err != nil {
		return fmt.Errorf("cannot convert proposal to abi type: [%v]", err)
	}

	valid, err := tc.walletProposalValidator.ValidateRedemptionProposal(
		abiProposal,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateRedemptionProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func convertRedemptionProposalToAbiType(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.RedemptionProposal,
) (tbtcabi.WalletProposalValidatorRedemptionProposal, error) {
	redeemersOutputScripts := make(
		[][]byte,
		len(proposal.RedeemersOutputScripts),
	)

	for i, script := range proposal.RedeemersOutputScripts {
		// The on-chain script representation must be prepended with the script's
		// byte-length while bitcoin.Script is not. We need to add the
		// length prefix.
		prefixedScript, err := script.ToVarLenData()
		if err != nil {
			return tbtcabi.WalletProposalValidatorRedemptionProposal{}, fmt.Errorf(
				"cannot convert redeemer output script: [%v]",
				err,
			)
		}

		redeemersOutputScripts[i] = prefixedScript
	}

	return tbtcabi.WalletProposalValidatorRedemptionProposal{
		WalletPubKeyHash:       walletPublicKeyHash,
		RedeemersOutputScripts: redeemersOutputScripts,
		RedemptionTxFee:        proposal.RedemptionTxFee,
	}, nil
}

func (tc *TbtcChain) GetRedemptionMaxSize() (uint16, error) {
	return tc.walletProposalValidator.REDEMPTIONMAXSIZE()
}

func (tc *TbtcChain) GetRedemptionRequestMinAge() (uint32, error) {
	return tc.walletProposalValidator.REDEMPTIONREQUESTMINAGE()
}

func (tc *TbtcChain) ValidateHeartbeatProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.HeartbeatProposal,
) error {
	valid, err := tc.walletProposalValidator.ValidateHeartbeatProposal(
		tbtcabi.WalletProposalValidatorHeartbeatProposal{
			WalletPubKeyHash: walletPublicKeyHash,
			Message:          proposal.Message[:],
		},
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateHeartbeatProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func (tc *TbtcChain) GetMovingFundsParameters() (tbtc.MovingFundsParameters, error) {
	parameters, err := tc.bridge.MovingFundsParameters()
	if err != nil {
		return tbtc.MovingFundsParameters{}, err
	}

	return tbtc.MovingFundsParameters{
		TxMaxTotalFee:                        parameters.MovingFundsTxMaxTotalFee,
		DustThreshold:                        parameters.MovingFundsDustThreshold,
		TimeoutResetDelay:                    parameters.MovingFundsTimeoutResetDelay,
		Timeout:                              parameters.MovingFundsTimeout,
		TimeoutSlashingAmount:                parameters.MovingFundsTimeoutSlashingAmount,
		TimeoutNotifierRewardMultiplier:      parameters.MovingFundsTimeoutNotifierRewardMultiplier,
		CommitmentGasOffset:                  parameters.MovingFundsCommitmentGasOffset,
		SweepTxMaxTotalFee:                   parameters.MovedFundsSweepTxMaxTotalFee,
		SweepTimeout:                         parameters.MovedFundsSweepTimeout,
		SweepTimeoutSlashingAmount:           parameters.MovedFundsSweepTimeoutSlashingAmount,
		SweepTimeoutNotifierRewardMultiplier: parameters.MovedFundsSweepTimeoutNotifierRewardMultiplier,
	}, nil
}

func (tc *TbtcChain) GetMovedFundsSweepRequest(
	movingFundsTxHash bitcoin.Hash,
	movingFundsTxOutpointIndex uint32,
) (*tbtc.MovedFundsSweepRequest, bool, error) {
	movedFundsKey := buildMovedFundsKey(
		movingFundsTxHash,
		movingFundsTxOutpointIndex,
	)

	movedFundsSweepRequest, err := tc.bridge.MovedFundsSweepRequests(
		movedFundsKey,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot get moved funds sweep request for key [0x%x]: [%v]",
			movedFundsKey.Text(16),
			err,
		)
	}

	// Moved funds sweep request not found.
	if movedFundsSweepRequest.CreatedAt == 0 {
		return nil, false, nil
	}

	state, err := parseMovedFundsSweepRequestState(movedFundsSweepRequest.State)
	if err != nil {
		return nil, false, fmt.Errorf(
			"cannot parse state for moved funds sweep request [0x%x]: [%v]",
			movedFundsKey.Text(16),
			err,
		)
	}

	return &tbtc.MovedFundsSweepRequest{
		WalletPublicKeyHash: movedFundsSweepRequest.WalletPubKeyHash,
		Value:               movedFundsSweepRequest.Value,
		CreatedAt:           time.Unix(int64(movedFundsSweepRequest.CreatedAt), 0),
		State:               state,
	}, true, nil
}

func parseMovedFundsSweepRequestState(value uint8) (
	tbtc.MovedFundsSweepRequestState,
	error,
) {
	switch value {
	case 0:
		return tbtc.MovedFundsStateUnknown, nil
	case 1:
		return tbtc.MovedFundsStatePending, nil
	case 2:
		return tbtc.MovedFundsStateProcessed, nil
	case 3:
		return tbtc.MovedFundsStateTimedOut, nil
	default:
		return 0, fmt.Errorf(
			"unexpected moved funds sweep request state value: [%v]",
			value,
		)
	}
}

func buildMovedFundsKey(
	movingFundsTxHash bitcoin.Hash,
	movingFundsTxOutpointIndex uint32,
) *big.Int {
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, movingFundsTxOutpointIndex)

	movedFundsKey := crypto.Keccak256Hash(
		append(movingFundsTxHash[:], indexBytes...),
	)

	return movedFundsKey.Big()
}

func (tc *TbtcChain) ValidateMovingFundsProposal(
	walletPublicKeyHash [20]byte,
	mainUTXO *bitcoin.UnspentTransactionOutput,
	proposal *tbtc.MovingFundsProposal,
) error {
	abiProposal := tbtcabi.WalletProposalValidatorMovingFundsProposal{
		WalletPubKeyHash: walletPublicKeyHash,
		TargetWallets:    proposal.TargetWallets,
		MovingFundsTxFee: proposal.MovingFundsTxFee,
	}
	abiMainUTXO := tbtcabi.BitcoinTxUTXO3{
		TxHash:        mainUTXO.Outpoint.TransactionHash,
		TxOutputIndex: mainUTXO.Outpoint.OutputIndex,
		TxOutputValue: uint64(mainUTXO.Value),
	}

	valid, err := tc.walletProposalValidator.ValidateMovingFundsProposal(
		abiProposal,
		abiMainUTXO,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateMovingFundsProposal` returns true
	// or reverts (returns an error) but do the check just in case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

func (tc *TbtcChain) GetRedemptionDelay(
	walletPublicKeyHash [20]byte,
	redeemerOutputScript bitcoin.Script,
) (time.Duration, error) {
	if tc.redemptionWatchtower == nil {
		return 0, nil
	}

	redemptionKey, err := tc.BuildRedemptionKey(walletPublicKeyHash, redeemerOutputScript)
	if err != nil {
		return 0, fmt.Errorf("cannot build redemption key: [%v]", err)
	}

	delay, err := tc.redemptionWatchtower.GetRedemptionDelay(redemptionKey)
	if err != nil {
		return 0, fmt.Errorf("cannot get redemption delay: [%v]", err)
	}

	return time.Duration(delay) * time.Second, nil
}

func (tc *TbtcChain) GetDepositMinAge() (uint32, error) {
	return tc.walletProposalValidator.DEPOSITMINAGE()
}

// GetReservation returns the on-chain reservation record for the given
// reservation key. The reservation router code is reached via
// Bridge.fallback's delegatecall; the reservationRouter binding is bound to
// the Bridge address so this call routes through the fallback into the
// router code that reads the Bridge's reservation storage.
func (tc *TbtcChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	abiReservation, err := tc.reservationRouter.Reservations(reservationKey)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation [0x%x]: [%v]",
			reservationKey,
			err,
		)
	}

	reservation, err := convertReservationFromAbiType(abiReservation)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert reservation [0x%x] from abi type: [%v]",
			reservationKey,
			err,
		)
	}

	return reservation, nil
}

// GetReservationAction returns the on-chain action record for the given
// reservation key and request nonce.
func (tc *TbtcChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	abiAction, err := tc.reservationRouter.ReservationActions(
		reservationKey,
		requestNonce,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation action [0x%x:%d]: [%v]",
			reservationKey,
			requestNonce,
			err,
		)
	}

	action, err := convertReservationActionFromAbiType(abiAction)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert reservation action [0x%x:%d] from abi type: [%v]",
			reservationKey,
			requestNonce,
			err,
		)
	}

	return action, nil
}

// ReservationParameters returns the current on-chain Bridge reservation
// parameters (10-tuple). The reservationRouter binding routes this read
// through Bridge.fallback into the router's reservationParameters view.
func (tc *TbtcChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	abiParameters, err := tc.reservationRouter.ReservationParameters()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation parameters: [%v]",
			err,
		)
	}

	return convertReservationParametersFromAbiType(abiParameters), nil
}

// ValidateReservationAnchorProposal asks the WalletProposalValidator
// whether the given anchor proposal is valid for the given wallet and
// reserved deposit. The validator is a separate contract reached at its
// own deployed address.
func (tc *TbtcChain) ValidateReservationAnchorProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationAnchorProposal,
	depositExtraInfo struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	// WalletProposalValidator's DepositExtraInfo.FundingTx is typed as
	// BitcoinTxInfo2 because the BitcoinTxInfo struct is renamed via the
	// collision hook in gen/Makefile (Bridge keeps the un-suffixed name;
	// WalletProposalValidator gets the 2-suffix; MaintainerProxy the 3;
	// ReservationRouter the 4). Mirroring the existing
	// ValidateDepositSweepProposal pattern.
	fundingTx := tbtcabi.BitcoinTxInfo2{
		Version:      depositExtraInfo.FundingTx.SerializeVersion(),
		InputVector:  depositExtraInfo.FundingTx.SerializeInputs(),
		OutputVector: depositExtraInfo.FundingTx.SerializeOutputs(),
		Locktime:     depositExtraInfo.FundingTx.SerializeLocktime(),
	}

	depositKey := tbtcabi.WalletProposalValidatorDepositKey{
		FundingTxHash:      proposal.DepositFundingTxHash,
		FundingOutputIndex: proposal.DepositFundingOutputIndex,
	}

	abiExtraInfo := tbtcabi.WalletProposalValidatorDepositExtraInfo{
		FundingTx:        fundingTx,
		BlindingFactor:   depositExtraInfo.Deposit.BlindingFactor,
		WalletPubKeyHash: depositExtraInfo.Deposit.WalletPublicKeyHash,
		RefundPubKeyHash: depositExtraInfo.Deposit.RefundPublicKeyHash,
		RefundLocktime:   depositExtraInfo.Deposit.RefundLocktime,
	}

	abiProposal := tbtcabi.WalletProposalValidatorReservationAnchorProposal{
		WalletPubKeyHash: walletPublicKeyHash,
		DepositKey:       depositKey,
		AnchorTxFee:      proposal.AnchorTxFee,
	}

	valid, err := tc.walletProposalValidator.ValidateReservationAnchorProposal(
		abiProposal,
		abiExtraInfo,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateReservationAnchorProposal`
	// returns true or reverts (returns an error) but do the check just in
	// case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

// ValidateReservedRedemptionProposal asks the WalletProposalValidator
// whether the given reserved redemption proposal is valid for the given
// wallet. The m1 bridge-integration surface does not expose a
// `validateReservedRedemptionProposal` entry on the WalletProposalValidator
// (only anchor and re-anchor validators are present at this milestone), so
// the interface stub returns an explicit error rather than calling a
// non-existent binding. Downstream tasks replacing this body will receive
// the bridge-integration Solidity once that validator lands.
func (tc *TbtcChain) ValidateReservedRedemptionProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservedRedemptionProposal,
) error {
	return fmt.Errorf(
		"reserved redemption proposal validator is not exposed on " +
			"the m1 bridge-integration surface",
	)
}

// ValidateReservationReanchorProposal asks the WalletProposalValidator
// whether the given re-anchor proposal is valid for the given source
// wallet. The validator is a separate contract reached at its own deployed
// address.
func (tc *TbtcChain) ValidateReservationReanchorProposal(
	sourceWalletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationReanchorProposal,
) error {
	abiProposal := tbtcabi.WalletProposalValidatorReservationReanchorProposal{
		SourceWalletPubKeyHash: sourceWalletPublicKeyHash,
		ReservationKey:         proposal.ReservationKey,
		TargetWalletPubKeyHash: proposal.TargetWalletPublicKeyHash,
		ReanchorTxFee:          proposal.ReanchorTxFee,
	}

	valid, err := tc.walletProposalValidator.ValidateReservationReanchorProposal(
		abiProposal,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateReservationReanchorProposal`
	// returns true or reverts (returns an error) but do the check just in
	// case.
	if !valid {
		return fmt.Errorf("unexpected validation result")
	}

	return nil
}

// ValidateReservationDissolutionProposal asks the WalletProposalValidator
// whether the given dissolution proposal is valid for the given wallet.
// The m1 bridge-integration surface does not expose a
// `validateReservationDissolutionProposal` entry on the
// WalletProposalValidator (only anchor and re-anchor validators are present
// at this milestone), so the interface stub returns an explicit error
// rather than calling a non-existent binding.
func (tc *TbtcChain) ValidateReservationDissolutionProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.ReservationDissolutionProposal,
) error {
	return fmt.Errorf(
		"reservation dissolution proposal validator is not exposed on " +
			"the m1 bridge-integration surface",
	)
}

// convertReservationFromAbiType converts the ReservationRouter-specific
// Reservation.ReservationRequest ABI struct to the TBTC application
// `tbtc.Reservation` representation.
//
// Field omissions (intentional, mirroring the Solidity-to-Go struct shrink):
//
//   - `CumulativeReanchorFee`: written by every re-anchor hop but not
//     exposed through the Go-side reservation; m1 has no fee-ceiling
//     enforcement, so the field is dropped on the Go boundary. A later
//     milestone that adds a fee ceiling should re-export this field on
//     `tbtc.Reservation`.
//
// Anchor shape reassembly: the on-chain request splits the anchor UTXO into
// `anchorAmount`, `anchorTxHash`, and `anchorTxOutputIndex`; the Go-side
// representation folds those three back into a single
// `bitcoin.UnspentTransactionOutput` for consistency with the rest of the
// reservation API.
func convertReservationFromAbiType(
	abiReservation tbtcabi.ReservationReservationRequest,
) (*tbtc.Reservation, error) {
	state, err := parseReservationState(abiReservation.State)
	if err != nil {
		return nil, fmt.Errorf("cannot parse reservation state: [%v]", err)
	}

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: &bitcoin.TransactionOutpoint{
			TransactionHash: abiReservation.AnchorTxHash,
			OutputIndex:     abiReservation.AnchorTxOutputIndex,
		},
		Value: int64(abiReservation.AnchorAmount),
	}

	return &tbtc.Reservation{
		Owner:                 chain.Address(abiReservation.Owner.String()),
		MintedAmount:          abiReservation.MintedAmount,
		AcceptedAt:            abiReservation.AcceptedAt,
		WalletPublicKeyHash:   abiReservation.WalletPubKeyHash,
		AnchorUtxo:            anchorUtxo,
		ExpiresAt:             abiReservation.ExpiresAt,
		State:                 state,
		RequestNonce:          abiReservation.RequestNonce,
		RetryCredit:           abiReservation.RetryCredit,
		DissolutionEligibleAt: abiReservation.DissolutionEligibleAt,
	}, nil
}

// convertReservationActionFromAbiType converts the ReservationRouter-
// specific Reservation.ReservationAction ABI struct to the TBTC
// application `tbtc.ReservationAction` representation.
//
// Field omissions (intentional):
//
//   - `SourceAnchorUtxoHash`, `UsedRetryCredit`,
//     `Watchtower{Default,LevelOne,LevelTwo}Delay`,
//     `RetryCreditSourceNonce`: written for governance / late-settlement
//     reconciliation but not read by the operator client in m1.
//
// The on-chain `actionDataHash` field is polymorphic across action types:
// it carries the keccak256 of the redeemer output script for redemptions,
// the wallet main UTXO hash for dissolutions, and is zero otherwise. The
// Go-side struct splits that polymorphism into two named fields
// (`RedeemerOutputScriptHash` for redemptions, `ExpectedMainUtxoHash`
// for dissolutions); we route `actionDataHash` to the field that matches
// the action's type and zero the other.
func convertReservationActionFromAbiType(
	abiAction tbtcabi.ReservationReservationAction,
) (*tbtc.ReservationAction, error) {
	actionType, err := parseReservationActionType(abiAction.ActionType)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse reservation action type: [%v]",
			err,
		)
	}

	state, err := parseReservationActionState(abiAction.State)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse reservation action state: [%v]",
			err,
		)
	}

	var (
		redeemerOutputScriptHash [32]byte
		expectedMainUtxoHash     [32]byte
	)
	switch actionType {
	case tbtc.ReservationActionTypeRedemption:
		redeemerOutputScriptHash = abiAction.ActionDataHash
	case tbtc.ReservationActionTypeDissolution:
		expectedMainUtxoHash = abiAction.ActionDataHash
	}

	return &tbtc.ReservationAction{
		TargetWalletPublicKeyHash: abiAction.TargetWalletPubKeyHash,
		RequestedAt:               abiAction.RequestedAt,
		TimeoutAt:                 abiAction.TimeoutAt,
		TxMaxFee:                  abiAction.TxMaxFee,
		ActionType:                actionType,
		State:                     state,
		FeePaid:                   abiAction.FeePaid,
		Redeemer:                  chain.Address(abiAction.Redeemer.String()),
		Amount:                    abiAction.Amount,
		RedeemerOutputScriptHash:  redeemerOutputScriptHash,
		ExpectedMainUtxoHash:      expectedMainUtxoHash,
		IsPartial:                 abiAction.IsPartial,
	}, nil
}

// convertReservationParametersFromAbiType converts the ReservationRouter
// 10-tuple to the `tbtc.ReservationParameters` representation.
func convertReservationParametersFromAbiType(
	abiParameters struct {
		ReservationVault                common.Address
		ReservationMinAmount            uint64
		ReservationTxMaxFee             uint64
		ReservationTermSeconds          uint32
		ReservationDissolutionDelay     uint32
		ReservationMaxTotalAmount       uint64
		ReservationTotalAmount          uint64
		MaxReservationsPerWallet        uint32
		ReservationActionTimeout        uint32
		ReservationRenewalWindowSeconds uint32
	},
) *tbtc.ReservationParameters {
	return &tbtc.ReservationParameters{
		ReservationVault:                chain.Address(abiParameters.ReservationVault.String()),
		ReservationMinAmount:            abiParameters.ReservationMinAmount,
		ReservationTxMaxFee:             abiParameters.ReservationTxMaxFee,
		ReservationTermSeconds:          abiParameters.ReservationTermSeconds,
		ReservationDissolutionDelay:     abiParameters.ReservationDissolutionDelay,
		ReservationMaxTotalAmount:       abiParameters.ReservationMaxTotalAmount,
		ReservationTotalAmount:          abiParameters.ReservationTotalAmount,
		MaxReservationsPerWallet:        abiParameters.MaxReservationsPerWallet,
		ReservationActionTimeout:        abiParameters.ReservationActionTimeout,
		ReservationRenewalWindowSeconds: abiParameters.ReservationRenewalWindowSeconds,
	}
}

// parseReservationState converts the on-chain ReservationState enum
// (uint8) to the tbtc.ReservationState value. Values match the Solidity
// declaration one-for-one (Unknown=0, Active=1, ActionPending=2,
// Closed=3, Stranded=4).
func parseReservationState(value uint8) (tbtc.ReservationState, error) {
	switch value {
	case 0:
		return tbtc.ReservationStateUnknown, nil
	case 1:
		return tbtc.ReservationStateActive, nil
	case 2:
		return tbtc.ReservationStateActionPending, nil
	case 3:
		return tbtc.ReservationStateClosed, nil
	case 4:
		return tbtc.ReservationStateStranded, nil
	default:
		return 0, fmt.Errorf("unexpected reservation state value: [%d]", value)
	}
}

// parseReservationActionType converts the on-chain ActionType enum
// (uint8) to the tbtc.ReservationActionType value. Values match the
// Solidity declaration one-for-one (None=0, Acceptance=1, Redemption=2,
// Reanchor=3, Dissolution=4).
func parseReservationActionType(value uint8) (tbtc.ReservationActionType, error) {
	switch value {
	case 0:
		return tbtc.ReservationActionTypeNone, nil
	case 1:
		return tbtc.ReservationActionTypeAcceptance, nil
	case 2:
		return tbtc.ReservationActionTypeRedemption, nil
	case 3:
		return tbtc.ReservationActionTypeReanchor, nil
	case 4:
		return tbtc.ReservationActionTypeDissolution, nil
	default:
		return 0, fmt.Errorf("unexpected reservation action type value: [%d]", value)
	}
}

// parseReservationActionState converts the on-chain ActionState enum
// (uint8) to the tbtc.ReservationActionState value. Values match the
// Solidity declaration one-for-one (Unknown=0, Pending=1, Settled=2,
// TimedOut=3, Vetoed=4, Superseded=5).
func parseReservationActionState(value uint8) (tbtc.ReservationActionState, error) {
	switch value {
	case 0:
		return tbtc.ReservationActionStateUnknown, nil
	case 1:
		return tbtc.ReservationActionStatePending, nil
	case 2:
		return tbtc.ReservationActionStateSettled, nil
	case 3:
		return tbtc.ReservationActionStateTimedOut, nil
	case 4:
		return tbtc.ReservationActionStateVetoed, nil
	case 5:
		return tbtc.ReservationActionStateSuperseded, nil
	default:
		return 0, fmt.Errorf("unexpected reservation action state value: [%d]", value)
	}
}

// RequestReservationAcceptance asks the Bridge (via its ReservationRouter
// delegatecall target) to start a new reservation acceptance action generation
// for the given reservation. The Bridge binding holds the actual storage; the
// reservationRouter binding is bound to the Bridge address so this call routes
// through Bridge.fallback into the router code.
func (tc *TbtcChain) RequestReservationAcceptance(
	reservationKey *big.Int,
	walletPublicKeyHash [20]byte,
) error {
	gasEstimate, err := tc.reservationRouter.RequestReservationAcceptanceGasEstimate(
		reservationKey,
		walletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.RequestReservationAcceptance(
		reservationKey,
		walletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// RequestReservationReanchor asks the Bridge (via its ReservationRouter
// delegatecall target) to start a new reservation re-anchor action generation
// for the given reservation, targeting the given wallet.
func (tc *TbtcChain) RequestReservationReanchor(
	reservationKey *big.Int,
	targetWalletPublicKeyHash [20]byte,
) error {
	gasEstimate, err := tc.reservationRouter.RequestReservationReanchorGasEstimate(
		reservationKey,
		targetWalletPublicKeyHash,
	)
	if err != nil {
		return err
	}

	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.RequestReservationReanchor(
		reservationKey,
		targetWalletPublicKeyHash,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// SubmitReservationProof submits an SPV proof for the given reservation
// action generation to the Bridge. The proof path is onlySpvMaintainer on
// the router; the call goes through Bridge.fallback's delegatecall so the
// router code reads the Bridge's isSpvMaintainer mapping at the Bridge's
// address.
func (tc *TbtcChain) SubmitReservationProof(
	proofType uint8,
	txInfo *tbtc.BitcoinTxInfo,
	proof *tbtc.BitcoinTxProof,
	mainUtxo *tbtc.BitcoinTxUTXO,
	reservationKey *big.Int,
	requestNonce uint64,
) error {
	abiTxInfo := tbtcabi.BitcoinTxInfo4{
		Version:      txInfo.Version,
		InputVector:  txInfo.InputVector,
		OutputVector: txInfo.OutputVector,
		Locktime:     txInfo.Locktime,
	}
	abiProof := tbtcabi.BitcoinTxProof3{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   proof.TxIndexInBlock,
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
	abiUtxo := tbtcabi.BitcoinTxUTXO4{
		TxHash:        mainUtxo.TxHash,
		TxOutputIndex: mainUtxo.TxOutputIndex,
		TxOutputValue: mainUtxo.TxOutputValue,
	}

	gasEstimate, err := tc.reservationRouter.SubmitReservationProofGasEstimate(
		proofType,
		abiTxInfo,
		abiProof,
		abiUtxo,
		reservationKey,
		requestNonce,
	)
	if err != nil {
		return err
	}

	// The original estimate for this contract call is too low; the
	// reservation proof path dispatches into ReservationProofs.submit*Proof,
	// which performs a non-trivial amount of storage I/O. Apply a 20%
	// margin mirroring the existing SubmitRedemptionProofWithReimbursement
	// pattern in this file.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.SubmitReservationProof(
		proofType,
		abiTxInfo,
		abiProof,
		abiUtxo,
		reservationKey,
		requestNonce,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyReservationActionTimeout notifies the Bridge that the timeout for
// the given reservation action generation has elapsed.
func (tc *TbtcChain) NotifyReservationActionTimeout(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyReservationActionTimeoutGasEstimate(
		reservationKey,
		walletMembersIDs,
	)
	if err != nil {
		return err
	}

	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyReservationActionTimeout(
		reservationKey,
		walletMembersIDs,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyStaleReservedDeposit notifies the Bridge that the given reserved
// deposit's wallet did not anchor it within the reservation-action timeout.
func (tc *TbtcChain) NotifyStaleReservedDeposit(
	depositKey *big.Int,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyStaleReservedDepositGasEstimate(
		depositKey,
	)
	if err != nil {
		return err
	}

	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyStaleReservedDeposit(
		depositKey,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// NotifyReservationStranded notifies the Bridge that the wallet custodying
// the given reservation has been closed or terminated.
func (tc *TbtcChain) NotifyReservationStranded(
	reservationKey *big.Int,
) error {
	gasEstimate, err := tc.reservationRouter.NotifyReservationStrandedGasEstimate(
		reservationKey,
	)
	if err != nil {
		return err
	}

	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)

	_, err = tc.reservationRouter.NotifyReservationStranded(
		reservationKey,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)

	return err
}

// ReservationCaps returns the cap parameters that gate reservation
// acceptance. The reservationRouter binding is bound to the Bridge
// address; the call routes through Bridge.fallback into the router's
// reservationCaps view.
func (tc *TbtcChain) ReservationCaps() (
	uint64,
	uint64,
	error,
) {
	caps, err := tc.reservationRouter.ReservationCaps()
	if err != nil {
		return 0, 0, fmt.Errorf(
			"cannot get reservation caps: [%v]",
			err,
		)
	}

	return caps.MaxReservationsAmountPerWallet, caps.ReservationMaxSingleAmount, nil
}

// WalletReservationsAmount returns the aggregate satoshi amount currently
// anchored by the given wallet across all of its reservations.
func (tc *TbtcChain) WalletReservationsAmount(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	amount, err := tc.reservationRouter.WalletReservationsAmount(walletPublicKeyHash)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get wallet reservations amount for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return amount, nil
}

// WalletReservationsCount returns the number of reservations currently
// custodied by the given wallet.
func (tc *TbtcChain) WalletReservationsCount(
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	count, err := tc.reservationRouter.WalletReservationsCount(walletPublicKeyHash)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get wallet reservations count for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return count, nil
}

// WalletReservations returns the reservation keys for all reservations
// currently custodied by the given wallet.
func (tc *TbtcChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	keys, err := tc.reservationRouter.WalletReservations(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get wallet reservations for [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	return keys, nil
}

// ReservationByAnchorUtxo returns the reservation key whose anchor outpoint
// is the given Bitcoin transaction output, or an empty value if no
// reservation is anchored there.
func (tc *TbtcChain) ReservationByAnchorUtxo(
	anchorTxHash [32]byte,
	anchorTxOutputIndex uint32,
) (*big.Int, error) {
	key, err := tc.reservationRouter.ReservationByAnchorUtxo(
		anchorTxHash,
		anchorTxOutputIndex,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation by anchor utxo [0x%x:%d]: [%v]",
			anchorTxHash,
			anchorTxOutputIndex,
			err,
		)
	}

	return key, nil
}

// ReservedDepositWallet returns the wallet public key hash to which the
// given reserved deposit was revealed. Returns the zero hash if the
// deposit is not a reserved deposit.
func (tc *TbtcChain) ReservedDepositWallet(
	depositKey *big.Int,
) ([20]byte, error) {
	walletPublicKeyHash, err := tc.reservationRouter.ReservedDepositWallet(depositKey)
	if err != nil {
		return [20]byte{}, fmt.Errorf(
			"cannot get reserved deposit wallet for [0x%x]: [%v]",
			depositKey,
			err,
		)
	}

	return walletPublicKeyHash, nil
}

// PendingReservedDeposits returns the number of reserved deposits that
// have been revealed to the Bridge but not yet accepted by a wallet.
func (tc *TbtcChain) PendingReservedDeposits() (uint64, error) {
	count, err := tc.reservationRouter.PendingReservedDeposits()
	if err != nil {
		return 0, fmt.Errorf(
			"cannot get pending reserved deposits: [%v]",
			err,
		)
	}

	return count, nil
}

// convertReservationRequestFromAbiType converts the ReservationRouter-
// specific Reservation.ReservationRequest ABI struct to the TBTC
// application `tbtc.ReservationRequest` representation. This is the
// verbatim-on-chain conversion; callers that want a slightly-shrunk Go
// representation use GetReservation, which drops CumulativeReanchorFee
// because m1 has no fee-ceiling enforcement.
func convertReservationRequestFromAbiType(
	abiReservation tbtcabi.ReservationReservationRequest,
) (*tbtc.ReservationRequest, error) {
	state, err := parseReservationState(abiReservation.State)
	if err != nil {
		return nil, fmt.Errorf("cannot parse reservation state: [%v]", err)
	}

	return &tbtc.ReservationRequest{
		Owner:                 chain.Address(abiReservation.Owner.String()),
		MintedAmount:          abiReservation.MintedAmount,
		AcceptedAt:            abiReservation.AcceptedAt,
		WalletPublicKeyHash:   abiReservation.WalletPubKeyHash,
		AnchorAmount:          abiReservation.AnchorAmount,
		ExpiresAt:             abiReservation.ExpiresAt,
		AnchorTxHash:          abiReservation.AnchorTxHash,
		AnchorTxOutputIndex:   abiReservation.AnchorTxOutputIndex,
		State:                 state,
		RequestNonce:          abiReservation.RequestNonce,
		RetryCredit:           abiReservation.RetryCredit,
		DissolutionEligibleAt: abiReservation.DissolutionEligibleAt,
		CumulativeReanchorFee: abiReservation.CumulativeReanchorFee,
	}, nil
}

// Reservations returns the on-chain reservation request record for the
// given reservation key, including the cumulative re-anchor fee that the
// existing GetReservation representation drops.
func (tc *TbtcChain) Reservations(
	reservationKey *big.Int,
) (*tbtc.ReservationRequest, error) {
	abiReservation, err := tc.reservationRouter.Reservations(reservationKey)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation [0x%x]: [%v]",
			reservationKey,
			err,
		)
	}

	reservation, err := convertReservationRequestFromAbiType(abiReservation)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert reservation [0x%x] from abi type: [%v]",
			reservationKey,
			err,
		)
	}

	return reservation, nil
}

// convertReservationActionRecordFromAbiType converts the ReservationRouter-
// specific Reservation.ReservationAction ABI struct to the TBTC
// application `tbtc.ReservationActionRecord` representation. This is the
// verbatim-on-chain conversion; callers that want a slightly-shrunk Go
// representation use GetReservationAction, which drops the late-settlement
// and retry-credit fields because m1 does not consume them.
func convertReservationActionRecordFromAbiType(
	abiAction tbtcabi.ReservationReservationAction,
) (*tbtc.ReservationActionRecord, error) {
	actionType, err := parseReservationActionType(abiAction.ActionType)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse reservation action type: [%v]",
			err,
		)
	}

	state, err := parseReservationActionState(abiAction.State)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse reservation action state: [%v]",
			err,
		)
	}

	return &tbtc.ReservationActionRecord{
		TargetWalletPublicKeyHash: abiAction.TargetWalletPubKeyHash,
		RequestedAt:               abiAction.RequestedAt,
		TimeoutAt:                 abiAction.TimeoutAt,
		TxMaxFee:                  abiAction.TxMaxFee,
		ActionType:                actionType,
		State:                     state,
		FeePaid:                   abiAction.FeePaid,
		Redeemer:                  chain.Address(abiAction.Redeemer.String()),
		Amount:                    abiAction.Amount,
		ActionDataHash:            abiAction.ActionDataHash,
		SourceAnchorUtxoHash:      abiAction.SourceAnchorUtxoHash,
		UsedRetryCredit:           abiAction.UsedRetryCredit,
		WatchtowerDefaultDelay:    abiAction.WatchtowerDefaultDelay,
		WatchtowerLevelOneDelay:   abiAction.WatchtowerLevelOneDelay,
		WatchtowerLevelTwoDelay:   abiAction.WatchtowerLevelTwoDelay,
		IsPartial:                 abiAction.IsPartial,
		RetryCreditSourceNonce:    abiAction.RetryCreditSourceNonce,
	}, nil
}

// ReservationActions returns the on-chain reservation action record for the
// given reservation key and request nonce, including the late-settlement
// and retry-credit fields that the existing GetReservationAction
// representation drops.
func (tc *TbtcChain) ReservationActions(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationActionRecord, error) {
	abiAction, err := tc.reservationRouter.ReservationActions(
		reservationKey,
		requestNonce,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get reservation action [0x%x:%d]: [%v]",
			reservationKey,
			requestNonce,
			err,
		)
	}

	action, err := convertReservationActionRecordFromAbiType(abiAction)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot convert reservation action [0x%x:%d] from abi type: [%v]",
			reservationKey,
			requestNonce,
			err,
		)
	}

	return action, nil
}

// ActiveReservationsCount returns the current count of active reservations
// across all wallets and the cap on that count.
func (tc *TbtcChain) ActiveReservationsCount() (uint32, uint32, error) {
	activeReservationsCount, err := tc.reservationRouter.ActiveReservationsCount()
	if err != nil {
		return 0, 0, fmt.Errorf(
			"cannot get active reservations count: [%v]",
			err,
		)
	}

	return activeReservationsCount.Count, activeReservationsCount.MaxActive, nil
}

// ReservationRouter returns the address of the ReservationRouter contract
// as stored on the Bridge. The router contract holds its own empty storage
// and only ever executes via Bridge.fallback's delegatecall, so this is
// the one place where the chain handle reads a router address value
// rather than binding a call to it: any actual reservation call routes
// through the Bridge binding (tc.reservationRouter, which is bound to the
// Bridge address) and dispatches into the router code via the fallback.
func (tc *TbtcChain) ReservationRouter() (chain.Address, error) {
	address, err := tc.bridge.GetReservationRouter()
	if err != nil {
		return "", fmt.Errorf(
			"cannot get reservation router address: [%v]",
			err,
		)
	}

	return chain.Address(address.Hex()), nil
}

// IsReservedDeposit returns true if the given deposit was revealed with
// the reservation vault address and is therefore a reservation rather than
// a default deposit.
func (tc *TbtcChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	isReserved, err := tc.bridge.IsReservedDeposit(depositKey)
	if err != nil {
		return false, fmt.Errorf(
			"cannot check if deposit [0x%x] is reserved: [%v]",
			depositKey,
			err,
		)
	}

	return isReserved, nil
}

// OnReservationAcceptanceRequested registers a callback that is invoked
// when an on-chain ReservationAcceptanceRequested event is seen. The
// subscription filters against the Bridge's address (the binding is bound
// to the Bridge address; delegatecall preserves the caller's address
// context so events emitted by router code carry the Bridge's address).
func (tc *TbtcChain) OnReservationAcceptanceRequested(
	handler func(event *tbtc.ReservationAcceptanceRequestedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		walletPublicKeyHash [20]byte,
		depositAmount uint64,
		txMaxFee uint64,
		timeoutAt uint32,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      reservationKey,
			RequestNonce:        requestNonce,
			WalletPublicKeyHash: walletPublicKeyHash,
			DepositAmount:       depositAmount,
			TxMaxFee:            txMaxFee,
			TimeoutAt:           timeoutAt,
			BlockNumber:         blockNumber,
		})
	}

	return tc.reservationRouter.ReservationAcceptanceRequestedEvent(
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// PastReservationAcceptanceRequestedEvents fetches past
// ReservationAcceptanceRequested events according to the provided filter
// or unfiltered if the filter is nil.
func (tc *TbtcChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var reservationKey []*big.Int
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		reservationKey = filter.ReservationKey
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	events, err := tc.reservationRouter.PastReservationAcceptanceRequestedEvents(
		startBlock,
		endBlock,
		reservationKey,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.ReservationAcceptanceRequestedEvent, 0)
	for _, event := range events {
		convertedEvents = append(convertedEvents, &tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      event.ReservationKey,
			RequestNonce:        event.RequestNonce,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			DepositAmount:       event.DepositAmount,
			TxMaxFee:            event.TxMaxFee,
			TimeoutAt:           event.TimeoutAt,
			BlockNumber:         event.Raw.BlockNumber,
		})
	}

	sort.SliceStable(convertedEvents, func(i, j int) bool {
		return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
	})

	return convertedEvents, nil
}

// OnReservationAccepted registers a callback that is invoked when an
// on-chain ReservationAccepted event is seen.
func (tc *TbtcChain) OnReservationAccepted(
	handler func(event *tbtc.ReservationAcceptedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		walletPublicKeyHash [20]byte,
		owner common.Address,
		anchorTxHash [32]byte,
		anchorAmount uint64,
		expiresAt uint32,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationAcceptedEvent{
			ReservationKey:      reservationKey,
			RequestNonce:        requestNonce,
			WalletPublicKeyHash: walletPublicKeyHash,
			Owner:               chain.Address(owner.Hex()),
			AnchorTxHash:        anchorTxHash,
			AnchorAmount:        anchorAmount,
			ExpiresAt:           expiresAt,
			BlockNumber:         blockNumber,
		})
	}

	return tc.reservationRouter.ReservationAcceptedEvent(
		nil,
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// PastReservationAcceptedEvents fetches past ReservationAccepted events
// according to the provided filter or unfiltered if the filter is nil.
func (tc *TbtcChain) PastReservationAcceptedEvents(
	filter *tbtc.ReservationAcceptedEventFilter,
) ([]*tbtc.ReservationAcceptedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var reservationKey []*big.Int
	var walletPublicKeyHash [][20]byte
	var owner []common.Address

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		reservationKey = filter.ReservationKey
		walletPublicKeyHash = filter.WalletPublicKeyHash
		for _, o := range filter.Owner {
			owner = append(owner, common.HexToAddress(string(o)))
		}
	}

	events, err := tc.reservationRouter.PastReservationAcceptedEvents(
		startBlock,
		endBlock,
		reservationKey,
		walletPublicKeyHash,
		owner,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.ReservationAcceptedEvent, 0)
	for _, event := range events {
		convertedEvents = append(convertedEvents, &tbtc.ReservationAcceptedEvent{
			ReservationKey:      event.ReservationKey,
			RequestNonce:        event.RequestNonce,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			Owner:               chain.Address(event.Owner.Hex()),
			AnchorTxHash:        event.AnchorTxHash,
			AnchorAmount:        event.AnchorAmount,
			ExpiresAt:           event.ExpiresAt,
			BlockNumber:         event.Raw.BlockNumber,
		})
	}

	sort.SliceStable(convertedEvents, func(i, j int) bool {
		return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
	})

	return convertedEvents, nil
}

// OnReservationReanchorRequested registers a callback that is invoked
// when an on-chain ReservationReanchorRequested event is seen.
func (tc *TbtcChain) OnReservationReanchorRequested(
	handler func(event *tbtc.ReservationReanchorRequestedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		sourceWalletPublicKeyHash [20]byte,
		targetWalletPublicKeyHash [20]byte,
		txMaxFee uint64,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            reservationKey,
			RequestNonce:              requestNonce,
			SourceWalletPublicKeyHash: sourceWalletPublicKeyHash,
			TargetWalletPublicKeyHash: targetWalletPublicKeyHash,
			TxMaxFee:                  txMaxFee,
			BlockNumber:               blockNumber,
		})
	}

	return tc.reservationRouter.ReservationReanchorRequestedEvent(
		nil,
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// PastReservationReanchorRequestedEvents fetches past
// ReservationReanchorRequested events according to the provided filter or
// unfiltered if the filter is nil.
func (tc *TbtcChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var reservationKey []*big.Int
	var sourceWalletPublicKeyHash [][20]byte
	var targetWalletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		reservationKey = filter.ReservationKey
		sourceWalletPublicKeyHash = filter.SourceWalletPublicKeyHash
		targetWalletPublicKeyHash = filter.TargetWalletPublicKeyHash
	}

	events, err := tc.reservationRouter.PastReservationReanchorRequestedEvents(
		startBlock,
		endBlock,
		reservationKey,
		sourceWalletPublicKeyHash,
		targetWalletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.ReservationReanchorRequestedEvent, 0)
	for _, event := range events {
		convertedEvents = append(convertedEvents, &tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            event.ReservationKey,
			RequestNonce:              event.RequestNonce,
			SourceWalletPublicKeyHash: event.SourceWalletPubKeyHash,
			TargetWalletPublicKeyHash: event.TargetWalletPubKeyHash,
			TxMaxFee:                  event.TxMaxFee,
			BlockNumber:               event.Raw.BlockNumber,
		})
	}

	sort.SliceStable(convertedEvents, func(i, j int) bool {
		return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
	})

	return convertedEvents, nil
}

// OnReservationReanchored registers a callback that is invoked when an
// on-chain ReservationReanchored event is seen.
func (tc *TbtcChain) OnReservationReanchored(
	handler func(event *tbtc.ReservationReanchoredEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		newWalletPublicKeyHash [20]byte,
		newAnchorTxHash [32]byte,
		newAnchorAmount uint64,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationReanchoredEvent{
			ReservationKey:         reservationKey,
			RequestNonce:           requestNonce,
			NewWalletPublicKeyHash: newWalletPublicKeyHash,
			NewAnchorTxHash:        newAnchorTxHash,
			NewAnchorAmount:        newAnchorAmount,
			BlockNumber:            blockNumber,
		})
	}

	return tc.reservationRouter.ReservationReanchoredEvent(
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// PastReservationReanchoredEvents fetches past ReservationReanchored
// events according to the provided filter or unfiltered if the filter is
// nil.
func (tc *TbtcChain) PastReservationReanchoredEvents(
	filter *tbtc.ReservationReanchoredEventFilter,
) ([]*tbtc.ReservationReanchoredEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var reservationKey []*big.Int
	var newWalletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		reservationKey = filter.ReservationKey
		newWalletPublicKeyHash = filter.NewWalletPublicKeyHash
	}

	events, err := tc.reservationRouter.PastReservationReanchoredEvents(
		startBlock,
		endBlock,
		reservationKey,
		newWalletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.ReservationReanchoredEvent, 0)
	for _, event := range events {
		convertedEvents = append(convertedEvents, &tbtc.ReservationReanchoredEvent{
			ReservationKey:         event.ReservationKey,
			RequestNonce:           event.RequestNonce,
			NewWalletPublicKeyHash: event.NewWalletPubKeyHash,
			NewAnchorTxHash:        event.NewAnchorTxHash,
			NewAnchorAmount:        event.NewAnchorAmount,
			BlockNumber:            event.Raw.BlockNumber,
		})
	}

	sort.SliceStable(convertedEvents, func(i, j int) bool {
		return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
	})

	return convertedEvents, nil
}

// OnReservationActionTimedOut registers a callback that is invoked when an
// on-chain ReservationActionTimedOut event is seen.
func (tc *TbtcChain) OnReservationActionTimedOut(
	handler func(event *tbtc.ReservationActionTimedOutEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		actionType uint8,
		blockNumber uint64,
	) {
		parsedActionType, err := parseReservationActionType(actionType)
		if err != nil {
			logger.Errorf(
				"unexpected reservation action type on ReservationActionTimedOut event: [%v]",
				err,
			)
			return
		}

		handler(&tbtc.ReservationActionTimedOutEvent{
			ReservationKey: reservationKey,
			RequestNonce:   requestNonce,
			ActionType:     parsedActionType,
			BlockNumber:    blockNumber,
		})
	}

	return tc.reservationRouter.ReservationActionTimedOutEvent(
		nil,
		nil,
	).OnEvent(onEvent)
}

// PastReservationActionTimedOutEvents fetches past
// ReservationActionTimedOut events according to the provided filter or
// unfiltered if the filter is nil.
func (tc *TbtcChain) PastReservationActionTimedOutEvents(
	filter *tbtc.ReservationActionTimedOutEventFilter,
) ([]*tbtc.ReservationActionTimedOutEvent, error) {
	var startBlock uint64
	var endBlock *uint64
	var reservationKey []*big.Int

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		reservationKey = filter.ReservationKey
	}

	events, err := tc.reservationRouter.PastReservationActionTimedOutEvents(
		startBlock,
		endBlock,
		reservationKey,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.ReservationActionTimedOutEvent, 0)
	for _, event := range events {
		parsedActionType, err := parseReservationActionType(event.ActionType)
		if err != nil {
			return nil, fmt.Errorf(
				"unexpected reservation action type on past ReservationActionTimedOut event: [%v]",
				err,
			)
		}

		convertedEvents = append(convertedEvents, &tbtc.ReservationActionTimedOutEvent{
			ReservationKey: event.ReservationKey,
			RequestNonce:   event.RequestNonce,
			ActionType:     parsedActionType,
			BlockNumber:    event.Raw.BlockNumber,
		})
	}

	sort.SliceStable(convertedEvents, func(i, j int) bool {
		return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
	})

	return convertedEvents, nil
}

// OnReservationActionSuperseded registers a callback that is invoked when
// an on-chain ReservationActionSuperseded event is seen.
func (tc *TbtcChain) OnReservationActionSuperseded(
	handler func(event *tbtc.ReservationActionSupersededEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationActionSupersededEvent{
			ReservationKey: reservationKey,
			RequestNonce:   requestNonce,
			BlockNumber:    blockNumber,
		})
	}

	return tc.reservationRouter.ReservationActionSupersededEvent(
		nil,
		nil,
	).OnEvent(onEvent)
}

// OnReservationLateSettled registers a callback that is invoked when an
// on-chain ReservationLateSettled event is seen.
func (tc *TbtcChain) OnReservationLateSettled(
	handler func(event *tbtc.ReservationLateSettledEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		requestNonce uint64,
		actionType uint8,
		blockNumber uint64,
	) {
		parsedActionType, err := parseReservationActionType(actionType)
		if err != nil {
			logger.Errorf(
				"unexpected reservation action type on ReservationLateSettled event: [%v]",
				err,
			)
			return
		}

		handler(&tbtc.ReservationLateSettledEvent{
			ReservationKey: reservationKey,
			RequestNonce:   requestNonce,
			ActionType:     parsedActionType,
			BlockNumber:    blockNumber,
		})
	}

	return tc.reservationRouter.ReservationLateSettledEvent(
		nil,
		nil,
	).OnEvent(onEvent)
}

// OnReservationRetryCreditMinted registers a callback that is invoked when
// an on-chain ReservationRetryCreditMinted event is seen.
func (tc *TbtcChain) OnReservationRetryCreditMinted(
	handler func(event *tbtc.ReservationRetryCreditMintedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationRetryCreditMintedEvent{
			ReservationKey: reservationKey,
			BlockNumber:    blockNumber,
		})
	}

	return tc.reservationRouter.ReservationRetryCreditMintedEvent(
		nil,
		nil,
	).OnEvent(onEvent)
}

// OnReservedDepositMarkedStale registers a callback that is invoked when
// an on-chain ReservedDepositMarkedStale event is seen.
func (tc *TbtcChain) OnReservedDepositMarkedStale(
	handler func(event *tbtc.ReservedDepositMarkedStaleEvent),
) subscription.EventSubscription {
	onEvent := func(
		depositKey *big.Int,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservedDepositMarkedStaleEvent{
			DepositKey:  depositKey,
			BlockNumber: blockNumber,
		})
	}

	return tc.reservationRouter.ReservedDepositMarkedStaleEvent(
		nil,
		nil,
	).OnEvent(onEvent)
}

// OnReservationStranded registers a callback that is invoked when an
// on-chain ReservationStranded event is seen.
func (tc *TbtcChain) OnReservationStranded(
	handler func(event *tbtc.ReservationStrandedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationKey *big.Int,
		walletPublicKeyHash [20]byte,
		owner common.Address,
		anchorAmount uint64,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationStrandedEvent{
			ReservationKey:      reservationKey,
			WalletPublicKeyHash: walletPublicKeyHash,
			Owner:               chain.Address(owner.Hex()),
			AnchorAmount:        anchorAmount,
			BlockNumber:         blockNumber,
		})
	}

	return tc.reservationRouter.ReservationStrandedEvent(
		nil,
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// OnReservationParametersUpdated registers a callback that is invoked when
// an on-chain ReservationParametersUpdated event is seen.
func (tc *TbtcChain) OnReservationParametersUpdated(
	handler func(event *tbtc.ReservationParametersUpdatedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationMinAmount uint64,
		reservationTxMaxFee uint64,
		reservationTermSeconds uint32,
		reservationDissolutionDelay uint32,
		reservationMaxTotalAmount uint64,
		maxReservationsPerWallet uint32,
		reservationActionTimeout uint32,
		reservationRenewalWindowSeconds uint32,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationParametersUpdatedEvent{
			ReservationMinAmount:            reservationMinAmount,
			ReservationTxMaxFee:             reservationTxMaxFee,
			ReservationTermSeconds:          reservationTermSeconds,
			ReservationDissolutionDelay:     reservationDissolutionDelay,
			ReservationMaxTotalAmount:       reservationMaxTotalAmount,
			MaxReservationsPerWallet:        maxReservationsPerWallet,
			ReservationActionTimeout:        reservationActionTimeout,
			ReservationRenewalWindowSeconds: reservationRenewalWindowSeconds,
			BlockNumber:                     blockNumber,
		})
	}

	return tc.reservationRouter.ReservationParametersUpdatedEvent(
		nil,
	).OnEvent(onEvent)
}

// OnReservationVaultUpdated registers a callback that is invoked when an
// on-chain ReservationVaultUpdated event is seen.
func (tc *TbtcChain) OnReservationVaultUpdated(
	handler func(event *tbtc.ReservationVaultUpdatedEvent),
) subscription.EventSubscription {
	onEvent := func(
		reservationVault common.Address,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationVaultUpdatedEvent{
			ReservationVault: chain.Address(reservationVault.Hex()),
			BlockNumber:      blockNumber,
		})
	}

	return tc.reservationRouter.ReservationVaultUpdatedEvent(
		nil,
	).OnEvent(onEvent)
}

// OnReservationCapsUpdated registers a callback that is invoked when an
// on-chain ReservationCapsUpdated event is seen.
func (tc *TbtcChain) OnReservationCapsUpdated(
	handler func(event *tbtc.ReservationCapsUpdatedEvent),
) subscription.EventSubscription {
	onEvent := func(
		maxReservationsAmountPerWallet uint64,
		reservationMaxSingleAmount uint64,
		maxActiveReservations uint32,
		blockNumber uint64,
	) {
		handler(&tbtc.ReservationCapsUpdatedEvent{
			MaxReservationsAmountPerWallet: maxReservationsAmountPerWallet,
			ReservationMaxSingleAmount:     reservationMaxSingleAmount,
			MaxActiveReservations:          maxActiveReservations,
			BlockNumber:                    blockNumber,
		})
	}

	return tc.reservationRouter.ReservationCapsUpdatedEvent(
		nil,
	).OnEvent(onEvent)
}

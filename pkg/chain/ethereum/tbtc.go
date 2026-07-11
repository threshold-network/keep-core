package ethereum

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/keep-network/keep-common/pkg/cache"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/bitcoin"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/chain"
	ecdsaabi "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/abi"
	ecdsacontract "github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/contract"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
	frostvalidatorabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/validatorabi"
	tbtcabi "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	tbtccontract "github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/contract"
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
	FrostWalletRegistryContractName     = "FrostWalletRegistry"
	FrostDkgValidatorContractName       = "FrostDkgValidator"
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

const frostWalletRegistryAuthorizationViewsABI = `[
	{
		"inputs": [{"internalType": "address", "name": "operator", "type": "address"}],
		"name": "operatorToStakingProvider",
		"outputs": [{"internalType": "address", "name": "", "type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"internalType": "address", "name": "stakingProvider", "type": "address"}],
		"name": "eligibleStake",
		"outputs": [{"internalType": "uint96", "name": "", "type": "uint96"}],
		"stateMutability": "view",
		"type": "function"
	}
]`

var frostWalletRegistryAuthorizationABI = mustParseABI(
	frostWalletRegistryAuthorizationViewsABI,
)

// TbtcChain represents a TBTC-specific chain handle.
type TbtcChain struct {
	*baseChain

	bridge                  *tbtccontract.Bridge
	bridgeAddress           common.Address
	maintainerProxy         *tbtccontract.MaintainerProxy
	walletRegistry          *ecdsacontract.WalletRegistry
	sortitionPool           *ecdsacontract.EcdsaSortitionPool
	frostWalletRegistry     *frostabi.FrostWalletRegistry
	frostWalletRegistryAddr common.Address
	frostDkgValidator       *frostvalidatorabi.FrostDkgValidator
	frostSortitionPool      *ecdsacontract.EcdsaSortitionPool
	walletProposalValidator *tbtccontract.WalletProposalValidator
	redemptionWatchtower    *tbtccontract.RedemptionWatchtower
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

	frostWalletRegistry, frostWalletRegistryAddr, frostSortitionPool, err := connectFrostWalletRegistry(
		config,
		baseChain,
	)
	if err != nil {
		return nil, err
	}

	frostDkgValidator, err := connectFrostDkgValidator(config, baseChain)
	if err != nil {
		return nil, err
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

	return &TbtcChain{
		baseChain:                baseChain,
		bridge:                   bridge,
		bridgeAddress:            bridgeAddress,
		maintainerProxy:          maintainerProxy,
		walletRegistry:           walletRegistry,
		sortitionPool:            sortitionPool,
		frostWalletRegistry:      frostWalletRegistry,
		frostWalletRegistryAddr:  frostWalletRegistryAddr,
		frostDkgValidator:        frostDkgValidator,
		frostSortitionPool:       frostSortitionPool,
		walletProposalValidator:  walletProposalValidator,
		redemptionWatchtower:     redemptionWatchtower,
		ecdsaDkgValidatorAddress: ecdsaDkgValidatorAddress,
		sweptDepositsCache:       cache.NewGenericTimeCache[*tbtc.DepositChainRequest](sweptDepositsCachePeriod),
	}, nil
}

func connectFrostWalletRegistry(
	config ethereum.Config,
	baseChain *baseChain,
) (
	*frostabi.FrostWalletRegistry,
	common.Address,
	*ecdsacontract.EcdsaSortitionPool,
	error,
) {
	frostWalletRegistryAddress, err := config.ContractAddress(
		FrostWalletRegistryContractName,
	)
	if err != nil {
		return nil, common.Address{}, nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			FrostWalletRegistryContractName,
			err,
		)
	}

	if frostWalletRegistryAddress == (common.Address{}) {
		logger.Infof(
			"%s contract address not configured; FROST DKG coordinator disabled",
			FrostWalletRegistryContractName,
		)
		return nil, common.Address{}, nil, nil
	}

	frostWalletRegistry, err := frostabi.NewFrostWalletRegistry(
		frostWalletRegistryAddress,
		baseChain.client,
	)
	if err != nil {
		return nil, common.Address{}, nil, fmt.Errorf(
			"failed to attach to FrostWalletRegistry contract: [%v]",
			err,
		)
	}

	frostSortitionPoolAddress, err := frostWalletRegistry.SortitionPool(
		&bind.CallOpts{From: baseChain.key.Address},
	)
	if err != nil {
		return nil, common.Address{}, nil, fmt.Errorf(
			"failed to get FROST sortition pool address: [%v]",
			err,
		)
	}

	// The FROST deployment uses a dedicated sortition pool instance but the
	// SortitionPool ABI is the same shape as the ECDSA pool binding.
	frostSortitionPool, err := ecdsacontract.NewEcdsaSortitionPool(
		frostSortitionPoolAddress,
		baseChain.chainID,
		baseChain.key,
		baseChain.client,
		baseChain.nonceManager,
		baseChain.miningWaiter,
		baseChain.blockCounter,
		baseChain.transactionMutex,
	)
	if err != nil {
		return nil, common.Address{}, nil, fmt.Errorf(
			"failed to attach to FrostSortitionPool contract: [%v]",
			err,
		)
	}

	return frostWalletRegistry, frostWalletRegistryAddress, frostSortitionPool, nil
}

func (tc *TbtcChain) hasFrostAuthorization() bool {
	return tc.frostWalletRegistry != nil && tc.frostSortitionPool != nil
}

func connectFrostDkgValidator(
	config ethereum.Config,
	baseChain *baseChain,
) (*frostvalidatorabi.FrostDkgValidator, error) {
	frostDkgValidatorAddress, err := config.ContractAddress(
		FrostDkgValidatorContractName,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			FrostDkgValidatorContractName,
			err,
		)
	}

	if frostDkgValidatorAddress == (common.Address{}) {
		logger.Infof(
			"%s contract address not configured; pre-submit FROST digest "+
				"view checks disabled (the address is required when the FROST "+
				"wallet registry is enabled)",
			FrostDkgValidatorContractName,
		)
		return nil, nil
	}

	frostDkgValidator, err := frostvalidatorabi.NewFrostDkgValidator(
		frostDkgValidatorAddress,
		baseChain.client,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to FrostDkgValidator contract: [%v]",
			err,
		)
	}

	return frostDkgValidator, nil
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

// FrostWalletGroupParametersFromChain mirrors FrostDkgValidator sizing
// constants. FROST and ECDSA validators are independent and may deliberately
// use different group sizes and thresholds, so callers must not reuse the
// legacy ECDSA parameters for FROST protocols.
func (tc *TbtcChain) FrostWalletGroupParametersFromChain(
	ctx context.Context,
) (*tbtc.GroupParameters, error) {
	if tc.frostWalletRegistry == nil {
		return nil, nil
	}
	if tc.frostDkgValidator == nil {
		return nil, fmt.Errorf(
			"FrostDkgValidator is required when FrostWalletRegistry is configured",
		)
	}

	callOpts := &bind.CallOpts{Context: ctx, From: tc.key.Address}
	groupSize, err := tc.frostDkgValidator.GroupSize(callOpts)
	if err != nil {
		return nil, fmt.Errorf("read FrostDkgValidator groupSize: %w", err)
	}
	activeThreshold, err := tc.frostDkgValidator.ActiveThreshold(callOpts)
	if err != nil {
		return nil, fmt.Errorf("read FrostDkgValidator activeThreshold: %w", err)
	}
	groupThreshold, err := tc.frostDkgValidator.GroupThreshold(callOpts)
	if err != nil {
		return nil, fmt.Errorf("read FrostDkgValidator groupThreshold: %w", err)
	}

	return walletGroupParametersFromValidatorValues(
		groupSize,
		activeThreshold,
		groupThreshold,
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
// as eligible to join the network. Legacy ECDSA operators are recognized if
// they have or had a stake delegation. FROST operators are recognized if the
// FROST registry maps them to a provider with non-zero eligible weight.
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

	if (stakingProvider != common.Address{}) {
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

		if hasStakeDelegation {
			return true, nil
		}
	}

	isRecognizedByFrost, err := tc.isRecognizedByFrostRegistry(operatorAddress)
	if err != nil {
		return false, err
	}
	if !isRecognizedByFrost {
		return false, nil
	}

	return true, nil
}

func (tc *TbtcChain) isRecognizedByFrostRegistry(
	operatorAddress common.Address,
) (bool, error) {
	if !tc.hasFrostAuthorization() ||
		(tc.frostWalletRegistryAddr == common.Address{}) {
		return false, nil
	}

	out, err := tc.callFrostRegistryAuthorizationView(
		"operatorToStakingProvider",
		operatorAddress,
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to map FROST operator [%v] to a provider: [%v]",
			operatorAddress,
			err,
		)
	}
	if len(out) != 1 {
		return false, fmt.Errorf(
			"unexpected FROST operatorToStakingProvider result length [%v]",
			len(out),
		)
	}

	stakingProvider := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	if (stakingProvider == common.Address{}) {
		return false, nil
	}

	out, err = tc.callFrostRegistryAuthorizationView(
		"eligibleStake",
		stakingProvider,
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to get FROST eligible weight for provider [%v]: [%v]",
			stakingProvider,
			err,
		)
	}
	if len(out) != 1 {
		return false, fmt.Errorf(
			"unexpected FROST eligibleStake result length [%v]",
			len(out),
		)
	}

	eligibleWeight := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	return eligibleWeight.Sign() > 0, nil
}

func (tc *TbtcChain) callFrostRegistryAuthorizationView(
	method string,
	args ...interface{},
) ([]interface{}, error) {
	var out []interface{}

	contract := bind.NewBoundContract(
		tc.frostWalletRegistryAddr,
		frostWalletRegistryAuthorizationABI,
		tc.baseChain.client,
		nil,
		nil,
	)

	err := contract.Call(
		&bind.CallOpts{From: tc.key.Address},
		&out,
		method,
		args...,
	)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func mustParseABI(rawABI string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(rawABI))
	if err != nil {
		panic(err)
	}

	return parsed
}

// OperatorToStakingProvider returns the staking provider address for the
// operator. If the staking provider has not been registered for the
// operator, the returned address is empty and the boolean flag is set to
// false. If the staking provider has been registered, the address is not
// empty and the boolean flag indicates true.
func (tc *TbtcChain) OperatorToStakingProvider() (chain.Address, bool, error) {
	var stakingProvider common.Address
	var err error

	if tc.hasFrostAuthorization() {
		stakingProvider, err = tc.frostWalletRegistry.OperatorToStakingProvider(
			&bind.CallOpts{From: tc.key.Address},
			tc.key.Address,
		)
	} else {
		stakingProvider, err = tc.walletRegistry.OperatorToStakingProvider(tc.key.Address)
	}

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
	stakingProviderAddress := common.HexToAddress(stakingProvider.String())

	var eligibleStake *big.Int
	var err error

	if tc.hasFrostAuthorization() {
		eligibleStake, err = tc.frostWalletRegistry.EligibleStake(
			&bind.CallOpts{From: tc.key.Address},
			stakingProviderAddress,
		)
	} else {
		eligibleStake, err = tc.walletRegistry.EligibleStake(stakingProviderAddress)
	}

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
	if tc.hasFrostAuthorization() {
		return tc.frostSortitionPool.IsLocked()
	}

	return tc.sortitionPool.IsLocked()
}

// IsOperatorInPool returns true if the operator is registered in
// the sortition pool.
func (tc *TbtcChain) IsOperatorInPool() (bool, error) {
	if tc.hasFrostAuthorization() {
		return tc.frostWalletRegistry.IsOperatorInPool(
			&bind.CallOpts{From: tc.key.Address},
			tc.key.Address,
		)
	}

	return tc.walletRegistry.IsOperatorInPool(tc.key.Address)
}

// IsOperatorUpToDate checks if the operator's authorized stake is in sync
// with operator's weight in the sortition pool.
// If the operator's authorized stake is not in sync with sortition pool
// weight, function returns false.
// If the operator is not in the sortition pool and their authorized stake
// is non-zero, function returns false.
func (tc *TbtcChain) IsOperatorUpToDate() (bool, error) {
	if tc.hasFrostAuthorization() {
		return tc.frostWalletRegistry.IsOperatorUpToDate(
			&bind.CallOpts{From: tc.key.Address},
			tc.key.Address,
		)
	}

	return tc.walletRegistry.IsOperatorUpToDate(tc.key.Address)
}

// JoinSortitionPool executes a transaction to have the operator join the
// sortition pool.
func (tc *TbtcChain) JoinSortitionPool() error {
	if tc.hasFrostAuthorization() {
		return tc.submitFrostWalletRegistryTransaction(
			"joinSortitionPool",
			func(opts *bind.TransactOpts) (*types.Transaction, error) {
				return tc.frostWalletRegistry.JoinSortitionPool(opts)
			},
		)
	}

	_, err := tc.walletRegistry.JoinSortitionPool()
	return err
}

// UpdateOperatorStatus executes a transaction to update the operator's
// state in the sortition pool.
func (tc *TbtcChain) UpdateOperatorStatus() error {
	if tc.hasFrostAuthorization() {
		return tc.submitFrostWalletRegistryTransaction(
			"updateOperatorStatus",
			func(opts *bind.TransactOpts) (*types.Transaction, error) {
				return tc.frostWalletRegistry.UpdateOperatorStatus(
					opts,
					tc.key.Address,
				)
			},
		)
	}

	_, err := tc.walletRegistry.UpdateOperatorStatus(tc.key.Address)
	return err
}

// IsEligibleForRewards checks whether the operator is eligible for rewards
// or not.
func (tc *TbtcChain) IsEligibleForRewards() (bool, error) {
	if tc.hasFrostAuthorization() {
		return tc.frostSortitionPool.IsEligibleForRewards(tc.key.Address)
	}

	return tc.sortitionPool.IsEligibleForRewards(tc.key.Address)
}

// Checks whether the operator is able to restore their eligibility for
// rewards right away.
func (tc *TbtcChain) CanRestoreRewardEligibility() (bool, error) {
	if tc.hasFrostAuthorization() {
		return tc.frostSortitionPool.CanRestoreRewardEligibility(tc.key.Address)
	}

	return tc.sortitionPool.CanRestoreRewardEligibility(tc.key.Address)
}

// Restores reward eligibility for the operator.
func (tc *TbtcChain) RestoreRewardEligibility() error {
	if tc.hasFrostAuthorization() {
		_, err := tc.frostSortitionPool.RestoreRewardEligibility(tc.key.Address)
		return err
	}

	_, err := tc.sortitionPool.RestoreRewardEligibility(tc.key.Address)
	return err
}

// Returns true if the chaosnet phase is active, false otherwise.
func (tc *TbtcChain) IsChaosnetActive() (bool, error) {
	if tc.hasFrostAuthorization() {
		return tc.frostSortitionPool.IsChaosnetActive()
	}

	return tc.sortitionPool.IsChaosnetActive()
}

// Returns true if operator is a beta operator, false otherwise.
// Chaosnet status does not matter.
func (tc *TbtcChain) IsBetaOperator() (bool, error) {
	if tc.hasFrostAuthorization() {
		return tc.frostSortitionPool.IsBetaOperator(tc.key.Address)
	}

	return tc.sortitionPool.IsBetaOperator(tc.key.Address)
}

// GetOperatorID returns the legacy ECDSA sortition pool ID number of the given
// operator address. An ID number of 0 means the operator has not been allocated
// an ID number yet.
//
// This method intentionally remains bound to the legacy ECDSA sortition pool
// even when FROST authorization is configured. Existing ECDSA tBTC flows such
// as DKG approval, inactivity claims, and tbtcpg moving-funds claims compare
// against ECDSA WalletRegistry member IDs. FROST DKG paths use
// SelectFrostGroup and the FROST sortition pool directly.
func (tc *TbtcChain) GetOperatorID(
	operatorAddress chain.Address,
) (chain.OperatorID, error) {
	return getOperatorID(
		tc.sortitionPool,
		common.HexToAddress(operatorAddress.String()),
	)
}

type operatorIDResolver interface {
	GetOperatorID(operator common.Address) (chain.OperatorID, error)
}

func getOperatorID(
	sortitionPool operatorIDResolver,
	operatorAddress common.Address,
) (chain.OperatorID, error) {
	return sortitionPool.GetOperatorID(operatorAddress)
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
	groupPublicKeyBytes := elliptic.Marshal(
		groupPublicKey.Curve,
		groupPublicKey.X,
		groupPublicKey.Y,
	)
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
	walletPublicKeyBytes := elliptic.Marshal(
		claim.WalletPublicKey.Curve,
		claim.WalletPublicKey.X,
		claim.WalletPublicKey.Y,
	)
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

func (tc *TbtcChain) PastTaprootDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.TaprootDepositRevealedEvent, error) {
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

	events, err := tc.bridge.PastTaprootDepositRevealedEvents(
		startBlock,
		endBlock,
		depositor,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make([]*tbtc.TaprootDepositRevealedEvent, 0)
	for _, event := range events {
		var vault *chain.Address
		if event.Vault != [20]byte{} {
			v := chain.Address(event.Vault.Hex())
			vault = &v
		}

		convertedEvent := &tbtc.TaprootDepositRevealedEvent{
			// We can map the event.FundingTxHash field directly to the
			// bitcoin.Hash type. This is because event.FundingTxHash is
			// a [32]byte type representing a hash in the bitcoin.InternalByteOrder,
			// just as bitcoin.Hash assumes.
			FundingTxHash:        event.FundingTxHash,
			FundingOutputIndex:   event.FundingOutputIndex,
			Depositor:            chain.Address(event.Depositor.Hex()),
			Amount:               event.Amount,
			BlindingFactor:       event.BlindingFactor,
			WalletPublicKeyHash:  event.WalletPubKeyHash,
			WalletXOnlyPublicKey: event.WalletXOnlyPublicKey,
			RefundPublicKeyHash:  event.RefundPubKeyHash,
			RefundXOnlyPublicKey: event.RefundXOnlyPublicKey,
			RefundLocktime:       event.RefundLocktime,
			Vault:                vault,
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
	var walletID [][32]byte
	var ecdsaWalletID [][32]byte
	var walletPublicKeyHash [][20]byte

	if filter != nil {
		startBlock = filter.StartBlock
		endBlock = filter.EndBlock
		walletID = filter.WalletID
		ecdsaWalletID = filter.EcdsaWalletID
		walletPublicKeyHash = filter.WalletPublicKeyHash
	}

	return pastNewWalletRegisteredEvents(
		startBlock,
		endBlock,
		walletID,
		ecdsaWalletID,
		walletPublicKeyHash,
		tc.bridge,
		tc.bridge.PastNewWalletRegisteredEvents,
	)
}

type pastNewWalletRegisteredEventsFn func(
	startBlock uint64,
	endBlock *uint64,
	ecdsaWalletID [][32]byte,
	walletPubKeyHash [][20]byte,
) ([]*tbtcabi.BridgeNewWalletRegistered, error)

func pastNewWalletRegisteredEvents(
	startBlock uint64,
	endBlock *uint64,
	walletID [][32]byte,
	ecdsaWalletID [][32]byte,
	walletPublicKeyHash [][20]byte,
	bridge any,
	pastLegacyEvents pastNewWalletRegisteredEventsFn,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	v2Events, err := pastNewWalletRegisteredV2Events(
		startBlock,
		endBlock,
		walletID,
		ecdsaWalletID,
		walletPublicKeyHash,
		bridge,
	)
	if err != nil {
		return nil, err
	}

	legacyEvents, err := pastLegacyEvents(
		startBlock,
		endBlock,
		ecdsaWalletID,
		walletPublicKeyHash,
	)
	if err != nil {
		return nil, err
	}

	convertedEvents := make(
		[]*tbtc.NewWalletRegisteredEvent,
		0,
		len(v2Events)+len(legacyEvents),
	)
	seenRegistrations := make(map[[20]byte]struct{})

	appendUnique := func(event *tbtc.NewWalletRegisteredEvent) {
		// The Bridge keys wallet state by public key hash. Compatibility legacy
		// events cannot carry a FROST wallet's canonical x-only wallet ID, so
		// the public key hash is the common identity available in both event
		// versions.
		if _, exists := seenRegistrations[event.WalletPublicKeyHash]; exists {
			return
		}

		seenRegistrations[event.WalletPublicKeyHash] = struct{}{}
		convertedEvents = append(convertedEvents, event)
	}

	// V2 events are appended first so they win over compatibility legacy
	// events emitted for the same registration.
	for _, event := range v2Events {
		appendUnique(event)
	}

	for _, event := range legacyEvents {
		// A genuine legacy ECDSA registration always carries a non-zero ECDSA
		// wallet ID. A zero value can only be a compatibility event for a
		// scheme whose canonical wallet ID is not present in this legacy event;
		// synthesizing a padded-PKH ID would invent the wrong identity.
		if event.EcdsaWalletID == [32]byte{} {
			continue
		}

		convertedEvent := &tbtc.NewWalletRegisteredEvent{
			WalletID:            tbtc.DeriveLegacyWalletID(event.WalletPubKeyHash),
			EcdsaWalletID:       event.EcdsaWalletID,
			WalletPublicKeyHash: event.WalletPubKeyHash,
			BlockNumber:         event.Raw.BlockNumber,
		}

		if len(walletID) > 0 && !containsWalletID(walletID, convertedEvent.WalletID) {
			continue
		}

		appendUnique(convertedEvent)
	}

	sort.SliceStable(
		convertedEvents,
		func(i, j int) bool {
			return convertedEvents[i].BlockNumber < convertedEvents[j].BlockNumber
		},
	)

	return convertedEvents, nil
}

func containsWalletID(walletIDs [][32]byte, walletID [32]byte) bool {
	for _, candidate := range walletIDs {
		if candidate == walletID {
			return true
		}
	}

	return false
}

func pastNewWalletRegisteredV2Events(
	startBlock uint64,
	endBlock *uint64,
	walletID [][32]byte,
	ecdsaWalletID [][32]byte,
	walletPublicKeyHash [][20]byte,
	bridge any,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	if bridge == nil {
		return nil, nil
	}

	bridgeValue := reflect.ValueOf(bridge)
	pastV2Events := bridgeValue.MethodByName("PastNewWalletRegisteredV2Events")
	if !pastV2Events.IsValid() {
		return nil, nil
	}

	var (
		results []reflect.Value
		callErr error
	)

	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				callErr = fmt.Errorf(
					"panic calling PastNewWalletRegisteredV2Events: [%v]",
					recovered,
				)
			}
		}()

		results = pastV2Events.Call(
			[]reflect.Value{
				reflect.ValueOf(startBlock),
				reflect.ValueOf(endBlock),
				reflect.ValueOf(walletID),
				reflect.ValueOf(ecdsaWalletID),
				reflect.ValueOf(walletPublicKeyHash),
			},
		)
	}()

	if callErr != nil {
		return nil, callErr
	}

	if len(results) != 2 {
		return nil, fmt.Errorf(
			"unexpected PastNewWalletRegisteredV2Events result count: [%v]",
			len(results),
		)
	}

	if !results[1].IsNil() {
		err, ok := results[1].Interface().(error)
		if !ok {
			return nil, fmt.Errorf(
				"unexpected PastNewWalletRegisteredV2Events error type: [%T]",
				results[1].Interface(),
			)
		}

		return nil, err
	}

	eventsValue := results[0]
	if eventsValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf(
			"unexpected PastNewWalletRegisteredV2Events events type: [%v]",
			eventsValue.Kind(),
		)
	}

	convertedEvents := make([]*tbtc.NewWalletRegisteredEvent, 0, eventsValue.Len())
	for i := 0; i < eventsValue.Len(); i++ {
		eventValue := eventsValue.Index(i)
		if eventValue.Kind() == reflect.Pointer {
			if eventValue.IsNil() {
				continue
			}

			eventValue = eventValue.Elem()
		}

		if eventValue.Kind() != reflect.Struct {
			return nil, fmt.Errorf(
				"unexpected NewWalletRegisteredV2 event kind: [%v]",
				eventValue.Kind(),
			)
		}

		walletIDField := eventValue.FieldByName("WalletID")
		ecdsaWalletIDField := eventValue.FieldByName("EcdsaWalletID")
		walletPubKeyHashField := eventValue.FieldByName("WalletPubKeyHash")
		if !walletPubKeyHashField.IsValid() {
			walletPubKeyHashField = eventValue.FieldByName("WalletPublicKeyHash")
		}
		rawField := eventValue.FieldByName("Raw")
		if !rawField.IsValid() {
			return nil, fmt.Errorf(
				"unexpected NewWalletRegisteredV2 raw event payload at index [%v]",
				i,
			)
		}

		if rawField.Kind() == reflect.Pointer {
			if rawField.IsNil() {
				return nil, fmt.Errorf("unexpected nil raw event payload")
			}

			rawField = rawField.Elem()
		}

		if rawField.Kind() != reflect.Struct {
			return nil, fmt.Errorf(
				"unexpected NewWalletRegisteredV2 raw event payload kind at index [%v]: [%v]",
				i,
				rawField.Kind(),
			)
		}

		blockNumberField := rawField.FieldByName("BlockNumber")

		if !walletIDField.IsValid() ||
			walletIDField.Type() != reflect.TypeOf([32]byte{}) ||
			!ecdsaWalletIDField.IsValid() ||
			ecdsaWalletIDField.Type() != reflect.TypeOf([32]byte{}) ||
			!walletPubKeyHashField.IsValid() ||
			walletPubKeyHashField.Type() != reflect.TypeOf([20]byte{}) ||
			!blockNumberField.IsValid() ||
			blockNumberField.Kind() != reflect.Uint64 {
			return nil, fmt.Errorf(
				"unexpected NewWalletRegisteredV2 event shape at index [%v]",
				i,
			)
		}

		convertedEvents = append(
			convertedEvents,
			&tbtc.NewWalletRegisteredEvent{
				WalletID:            walletIDField.Interface().([32]byte),
				EcdsaWalletID:       ecdsaWalletIDField.Interface().([32]byte),
				WalletPublicKeyHash: walletPubKeyHashField.Interface().([20]byte),
				BlockNumber:         blockNumberField.Uint(),
			},
		)
	}

	return convertedEvents, nil
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

func (tc *TbtcChain) IsFrostWalletRegistered(walletID [32]byte) (bool, error) {
	if tc.frostWalletRegistry == nil {
		return false, fmt.Errorf("FROST wallet registry is not configured")
	}

	isWalletRegistered, err := tc.frostWalletRegistry.IsWalletRegistered(
		&bind.CallOpts{},
		walletID,
	)
	if err != nil {
		return false, fmt.Errorf(
			"cannot check if FROST wallet with ID [0x%x] is registered: [%v]",
			walletID,
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

	walletID, err := resolveWalletID(
		tc.bridge,
		walletPublicKeyHash,
		wallet.EcdsaWalletID,
	)
	if err != nil {
		return nil, err
	}

	return &tbtc.WalletChainData{
		WalletID:                               walletID,
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

func (tc *TbtcChain) WalletPublicKeyHashForWalletID(
	walletID [32]byte,
) ([20]byte, error) {
	return resolveWalletPublicKeyHashForWalletID(
		walletID,
		tc.bridge,
	)
}

type walletIDForWalletPublicKeyHashFn interface {
	WalletID(walletPublicKeyHash [20]byte) ([32]byte, error)
}

func walletIDForWalletPublicKeyHash(
	bridge any,
	walletPublicKeyHash [20]byte,
) ([32]byte, error) {
	resolver, ok := bridge.(walletIDForWalletPublicKeyHashFn)
	if !ok {
		return [32]byte{}, fmt.Errorf("wallet ID accessor unavailable")
	}

	return resolver.WalletID(walletPublicKeyHash)
}

// resolveWalletID returns the canonical wallet ID for the wallet identified by
// walletPublicKeyHash. ecdsaWalletID is that wallet's ECDSA wallet ID from the
// Bridge record -- zero for FROST wallets, non-zero for legacy ECDSA wallets.
//
// On an accessor error the fallback is routed by SCHEME, not by error type
// (which cannot reliably distinguish a legacy on-chain Bridge -- whose walletID
// eth_call returns a normal RPC/ABI error even when the node uses the current
// binding -- from a transient failure):
//
//   - A legacy ECDSA wallet's canonical wallet ID equals its legacy derivation,
//     so falling back is correct, and it is the only option on a legacy Bridge
//     whose contract lacks the walletID accessor.
//   - A FROST wallet requires its canonical wallet ID; the legacy derivation
//     would be a different value and would select the wrong (P2WPKH vs P2TR)
//     wallet script, so the error is surfaced instead of falling back. A FROST
//     wallet only exists on a canonical-ID Bridge, so such an error is genuinely
//     transient.
func resolveWalletID(
	bridge any,
	walletPublicKeyHash [20]byte,
	ecdsaWalletID [32]byte,
) ([32]byte, error) {
	walletID, err := walletIDForWalletPublicKeyHash(bridge, walletPublicKeyHash)
	if err == nil {
		return walletID, nil
	}

	if ecdsaWalletID == ([32]byte{}) {
		return [32]byte{}, fmt.Errorf(
			"cannot resolve canonical wallet ID for FROST wallet [0x%x]: [%w]",
			walletPublicKeyHash,
			err,
		)
	}

	return tbtc.DeriveLegacyWalletID(walletPublicKeyHash), nil
}

type walletPublicKeyHashForWalletIDFn interface {
	WalletPubKeyHashForWalletID(walletID [32]byte) ([20]byte, error)
}

func resolveWalletPublicKeyHashForWalletID(
	walletID [32]byte,
	bridge any,
) ([20]byte, error) {
	resolveCanonical, ok := bridge.(walletPublicKeyHashForWalletIDFn)

	var walletPublicKeyHash [20]byte
	var err error
	if ok {
		walletPublicKeyHash, err = resolveCanonical.WalletPubKeyHashForWalletID(walletID)
	} else {
		err = fmt.Errorf("wallet public key hash accessor unavailable")
	}

	if err == nil {
		if walletPublicKeyHash != [20]byte{} {
			return walletPublicKeyHash, nil
		}
	}

	legacyWalletPublicKeyHash, ok := tbtc.WalletPublicKeyHashFromLegacyWalletID(walletID)
	if ok {
		if err != nil {
			logger.Infof(
				"canonical wallet public key hash resolution failed for wallet ID [0x%x]; using legacy derivation: [%v]",
				walletID,
				err,
			)
		}

		return legacyWalletPublicKeyHash, nil
	}

	if err != nil {
		return [20]byte{}, fmt.Errorf(
			"cannot resolve wallet public key hash for wallet ID [0x%x]: [%v]",
			walletID,
			err,
		)
	}

	return [20]byte{}, fmt.Errorf(
		"wallet public key hash not found for wallet ID [0x%x]",
		walletID,
	)
}

func (tc *TbtcChain) OnWalletClosed(
	handler func(event *tbtc.WalletClosedEvent),
) subscription.EventSubscription {
	onEcdsaEvent := func(
		walletID [32]byte,
		blockNumber uint64,
	) {
		handler(&tbtc.WalletClosedEvent{
			WalletID:    walletID,
			Scheme:      tbtc.WalletSchemeECDSA,
			BlockNumber: blockNumber,
		})
	}

	ecdsaSubscription := tc.walletRegistry.WalletClosedEvent(nil, nil).OnEvent(
		onEcdsaEvent,
	)
	frostSubscription := tc.onFrostWalletClosed(handler)

	return subscription.NewEventSubscription(func() {
		ecdsaSubscription.Unsubscribe()
		frostSubscription.Unsubscribe()
	})
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

func (tc *TbtcChain) GetDepositParameters() (
	dustThreshold uint64,
	treasuryFeeDivisor uint64,
	txMaxFee uint64,
	revealAheadPeriod uint32,
	err error,
) {
	parameters, callErr := tc.bridge.DepositParameters()
	if callErr != nil {
		err = callErr
		return
	}

	dustThreshold = parameters.DepositDustThreshold
	treasuryFeeDivisor = parameters.DepositTreasuryFeeDivisor
	txMaxFee = parameters.DepositTxMaxFee
	revealAheadPeriod = parameters.DepositRevealAheadPeriod

	return
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

func (tc *TbtcChain) GetRedemptionParameters() (
	dustThreshold uint64,
	treasuryFeeDivisor uint64,
	txMaxFee uint64,
	txMaxTotalFee uint64,
	timeout uint32,
	timeoutSlashingAmount *big.Int,
	timeoutNotifierRewardMultiplier uint32,
	err error,
) {
	parameters, callErr := tc.bridge.RedemptionParameters()
	if callErr != nil {
		err = callErr
		return
	}

	dustThreshold = parameters.RedemptionDustThreshold
	treasuryFeeDivisor = parameters.RedemptionTreasuryFeeDivisor
	txMaxFee = parameters.RedemptionTxMaxFee
	txMaxTotalFee = parameters.RedemptionTxMaxTotalFee
	timeout = parameters.RedemptionTimeout
	timeoutSlashingAmount = parameters.RedemptionTimeoutSlashingAmount
	timeoutNotifierRewardMultiplier = parameters.RedemptionTimeoutNotifierRewardMultiplier

	return
}

func (tc *TbtcChain) GetWalletParameters() (
	creationPeriod uint32,
	creationMinBtcBalance uint64,
	creationMaxBtcBalance uint64,
	closureMinBtcBalance uint64,
	maxAge uint32,
	maxBtcTransfer uint64,
	closingPeriod uint32,
	err error,
) {
	parameters, callErr := tc.bridge.WalletParameters()
	if callErr != nil {
		err = callErr
		return
	}

	creationPeriod = parameters.WalletCreationPeriod
	creationMinBtcBalance = parameters.WalletCreationMinBtcBalance
	creationMaxBtcBalance = parameters.WalletCreationMaxBtcBalance
	closureMinBtcBalance = parameters.WalletClosureMinBtcBalance
	maxAge = parameters.WalletMaxAge
	maxBtcTransfer = parameters.WalletMaxBtcTransfer
	closingPeriod = parameters.WalletClosingPeriod

	return
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

func (tc *TbtcChain) ValidateTaprootDepositSweepProposal(
	walletPublicKeyHash [20]byte,
	proposal *tbtc.DepositSweepProposal,
	depositsExtraInfo []struct {
		*tbtc.Deposit
		FundingTx *bitcoin.Transaction
	},
) error {
	dei := make(
		[]tbtcabi.WalletProposalValidatorTaprootDepositExtraInfo,
		len(depositsExtraInfo),
	)
	for i, depositExtraInfo := range depositsExtraInfo {
		fundingTx := tbtcabi.BitcoinTxInfo2{
			Version:      depositExtraInfo.FundingTx.SerializeVersion(),
			InputVector:  depositExtraInfo.FundingTx.SerializeInputs(),
			OutputVector: depositExtraInfo.FundingTx.SerializeOutputs(),
			Locktime:     depositExtraInfo.FundingTx.SerializeLocktime(),
		}

		if !depositExtraInfo.Deposit.IsTaproot() {
			return fmt.Errorf("deposit extra info [%v] is not Taproot-native", i)
		}

		dei[i] = tbtcabi.WalletProposalValidatorTaprootDepositExtraInfo{
			FundingTx:            fundingTx,
			BlindingFactor:       depositExtraInfo.Deposit.BlindingFactor,
			WalletPubKeyHash:     depositExtraInfo.Deposit.WalletPublicKeyHash,
			WalletXOnlyPublicKey: *depositExtraInfo.Deposit.WalletXOnlyPublicKey,
			RefundPubKeyHash:     depositExtraInfo.Deposit.RefundPublicKeyHash,
			RefundXOnlyPublicKey: *depositExtraInfo.Deposit.RefundXOnlyPublicKey,
			RefundLocktime:       depositExtraInfo.Deposit.RefundLocktime,
		}
	}

	valid, err := tc.walletProposalValidator.ValidateTaprootDepositSweepProposal(
		convertDepositSweepProposalToAbiType(walletPublicKeyHash, proposal),
		dei,
	)
	if err != nil {
		return fmt.Errorf("validation failed: [%v]", err)
	}

	// Should never happen because `validateTaprootDepositSweepProposal`
	// returns true or reverts (returns an error) but do the check just in case.
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

func (tc *TbtcChain) GetMovingFundsParameters() (
	txMaxTotalFee uint64,
	dustThreshold uint64,
	timeoutResetDelay uint32,
	timeout uint32,
	timeoutSlashingAmount *big.Int,
	timeoutNotifierRewardMultiplier uint32,
	commitmentGasOffset uint16,
	sweepTxMaxTotalFee uint64,
	sweepTimeout uint32,
	sweepTimeoutSlashingAmount *big.Int,
	sweepTimeoutNotifierRewardMultiplier uint32,
	err error,
) {
	parameters, callErr := tc.bridge.MovingFundsParameters()
	if callErr != nil {
		err = callErr
		return
	}

	txMaxTotalFee = parameters.MovingFundsTxMaxTotalFee
	dustThreshold = parameters.MovingFundsDustThreshold
	timeoutResetDelay = parameters.MovingFundsTimeoutResetDelay
	timeout = parameters.MovingFundsTimeout
	timeoutSlashingAmount = parameters.MovingFundsTimeoutSlashingAmount
	timeoutNotifierRewardMultiplier = parameters.MovingFundsTimeoutNotifierRewardMultiplier
	commitmentGasOffset = parameters.MovingFundsCommitmentGasOffset
	sweepTxMaxTotalFee = parameters.MovedFundsSweepTxMaxTotalFee
	sweepTimeout = parameters.MovedFundsSweepTimeout
	sweepTimeoutSlashingAmount = parameters.MovedFundsSweepTimeoutSlashingAmount
	sweepTimeoutNotifierRewardMultiplier = parameters.MovedFundsSweepTimeoutNotifierRewardMultiplier

	return
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

func (tc *TbtcChain) CurrentBlockTimestamp() (time.Time, error) {
	currentBlock, err := tc.currentBlockHeader()
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot get current block: [%v]", err)
	}

	return time.Unix(int64(currentBlock.Time), 0), nil
}

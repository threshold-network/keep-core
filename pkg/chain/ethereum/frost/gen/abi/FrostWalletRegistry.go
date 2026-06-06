// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package abi

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// FrostDkgParameters is an auto generated low-level Go binding around an user-defined struct.
type FrostDkgParameters struct {
	SeedTimeout                     *big.Int
	ResultChallengePeriodLength     *big.Int
	ResultChallengeExtraGas         *big.Int
	ResultSubmissionTimeout         *big.Int
	SubmitterPrecedencePeriodLength *big.Int
}

// FrostDkgResult is an auto generated low-level Go binding around an user-defined struct.
type FrostDkgResult struct {
	SubmitterMemberIndex     *big.Int
	XOnlyOutputKey           [32]byte
	MisbehavedMembersIndices []uint8
	Signatures               []byte
	SigningMembersIndices    []*big.Int
	Members                  []uint32
	MembersHash              [32]byte
}

// FrostInactivityClaim is an auto generated low-level Go binding around an user-defined struct.
type FrostInactivityClaim struct {
	WalletID               [32]byte
	InactiveMembersIndices []*big.Int
	HeartbeatFailed        bool
	Signatures             []byte
	SigningMembersIndices  []*big.Int
}

// FrostRegistryWalletsWallet is an auto generated low-level Go binding around an user-defined struct.
type FrostRegistryWalletsWallet struct {
	MembersIdsHash [32]byte
	XOnlyOutputKey [32]byte
}

// FrostWalletRegistryMetaData contains all meta data concerning the FrostWalletRegistry contract.
var FrostWalletRegistryMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"_sortitionPool\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"LifecycleOwnerNotSet\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"WalletNotRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"XOnlyOutputKeyAlreadyRegistered\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"XOnlyOutputKeyIsLegacyAlias\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"XOnlyOutputKeyIsZero\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"AuthorizationDecreaseApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"decreasingAt\",\"type\":\"uint64\"}],\"name\":\"AuthorizationDecreaseRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"AuthorizationIncreased\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"minimumAuthorization\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"name\":\"AuthorizationParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"maliciousSubmitter\",\"type\":\"address\"}],\"name\":\"DkgMaliciousResultSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"maliciousSubmitter\",\"type\":\"address\"}],\"name\":\"DkgMaliciousResultSlashingFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"seedTimeout\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultChallengePeriodLength\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultChallengeExtraGas\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultSubmissionTimeout\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultSubmitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"name\":\"DkgParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"approver\",\"type\":\"address\"}],\"name\":\"DkgResultApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"challenger\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"reason\",\"type\":\"string\"}],\"name\":\"DkgResultChallenged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"indexed\":false,\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"}],\"name\":\"DkgResultSubmitted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgSeedTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"}],\"name\":\"DkgStarted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgStateLocked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifySeedTimeoutGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyDkgTimeoutNegativeGasOffset\",\"type\":\"uint256\"}],\"name\":\"GasParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldGovernance\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"GovernanceTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"notifier\",\"type\":\"address\"}],\"name\":\"InactivityClaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"InvoluntaryAuthorizationDecreaseFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"lifecycleOwner\",\"type\":\"address\"}],\"name\":\"LifecycleOwnerUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorJoinedSortitionPool\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorStatusUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"randomBeacon\",\"type\":\"address\"}],\"name\":\"RandomBeaconUpgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newReimbursementPool\",\"type\":\"address\"}],\"name\":\"ReimbursementPoolUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"}],\"name\":\"RewardParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"RewardsWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint256\"}],\"name\":\"SlashingParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"WalletClosed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"dkgResultHash\",\"type\":\"bytes32\"}],\"name\":\"WalletCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"walletOwner\",\"type\":\"address\"}],\"name\":\"WalletOwnerUpdated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"relayEntry\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"__beaconCallback\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"approveAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"approveDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"authorizationDecreaseRequested\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"authorizationIncreased\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"authorizationParameters\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"minimumAuthorization\",\"type\":\"uint96\"},{\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"authorizationSource\",\"outputs\":[{\"internalType\":\"contractIFrostAuthorizationSource\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"availableRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"challengeDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"closeWallet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"dkgParameters\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"seedTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"resultChallengePeriodLength\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"resultChallengeExtraGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"resultSubmissionTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"submitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"internalType\":\"structFrostDkg.Parameters\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"eligibleStake\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"gasParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifySeedTimeoutGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyDkgTimeoutNegativeGasOffset\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"getWallet\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"membersIdsHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostRegistryWallets.Wallet\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getWalletCreationState\",\"outputs\":[{\"internalType\":\"enumFrostDkg.State\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"getWalletXOnlyOutputKey\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"governance\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"hasDkgTimedOut\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"hasSeedTimedOut\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"inactivityClaimNonce\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractFrostDkgValidator\",\"name\":\"_ecdsaDkgValidator\",\"type\":\"address\"},{\"internalType\":\"contractIRandomBeacon\",\"name\":\"_randomBeacon\",\"type\":\"address\"},{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_bridge\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_authorizationSource\",\"type\":\"address\"}],\"name\":\"initializeV2\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"involuntaryAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"}],\"name\":\"isDkgResultValid\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorInPool\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorUpToDate\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"walletMemberIndex\",\"type\":\"uint256\"}],\"name\":\"isWalletMember\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"isWalletRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"joinSortitionPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lifecycleOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minimumAuthorization\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifyDkgTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"internalType\":\"uint256[]\",\"name\":\"inactiveMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"bool\",\"name\":\"heartbeatFailed\",\"type\":\"bool\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"}],\"internalType\":\"structFrostInactivity.Claim\",\"name\":\"claim\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"internalType\":\"uint32[]\",\"name\":\"groupMembers\",\"type\":\"uint32[]\"}],\"name\":\"notifyOperatorInactivity\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifySeedTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"operatorToStakingProvider\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"pendingAuthorizationDecrease\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"randomBeacon\",\"outputs\":[{\"internalType\":\"contractIRandomBeacon\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"registerOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"registered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reimbursementPool\",\"outputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"remainingAuthorizationDecreaseDelay\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"requestNewWallet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rewardParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"},{\"internalType\":\"uint256\",\"name\":\"rewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"notifier\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"}],\"name\":\"seize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"selectGroup\",\"outputs\":[{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"slashingParameters\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"sortitionPool\",\"outputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"stakingProviderToOperator\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"submitDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"transferGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"_minimumAuthorization\",\"type\":\"uint96\"},{\"internalType\":\"uint64\",\"name\":\"_authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"_authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"name\":\"updateAuthorizationParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_seedTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_resultChallengePeriodLength\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_resultChallengeExtraGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_resultSubmissionTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_submitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"name\":\"updateDkgParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifySeedTimeoutGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyDkgTimeoutNegativeGasOffset\",\"type\":\"uint256\"}],\"name\":\"updateGasParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_lifecycleOwner\",\"type\":\"address\"}],\"name\":\"updateLifecycleOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"updateOperatorStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"name\":\"updateReimbursementPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"}],\"name\":\"updateRewardParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint96\"}],\"name\":\"updateSlashingParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIFrostWalletOwner\",\"name\":\"_walletOwner\",\"type\":\"address\"}],\"name\":\"updateWalletOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIRandomBeacon\",\"name\":\"_randomBeacon\",\"type\":\"address\"}],\"name\":\"upgradeRandomBeacon\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"walletOwner\",\"outputs\":[{\"internalType\":\"contractIFrostWalletOwner\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"withdrawIneligibleRewards\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"withdrawRewards\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// FrostWalletRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use FrostWalletRegistryMetaData.ABI instead.
var FrostWalletRegistryABI = FrostWalletRegistryMetaData.ABI

// FrostWalletRegistry is an auto generated Go binding around an Ethereum contract.
type FrostWalletRegistry struct {
	FrostWalletRegistryCaller     // Read-only binding to the contract
	FrostWalletRegistryTransactor // Write-only binding to the contract
	FrostWalletRegistryFilterer   // Log filterer for contract events
}

// FrostWalletRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type FrostWalletRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FrostWalletRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type FrostWalletRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FrostWalletRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type FrostWalletRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FrostWalletRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type FrostWalletRegistrySession struct {
	Contract     *FrostWalletRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts        // Call options to use throughout this session
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// FrostWalletRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type FrostWalletRegistryCallerSession struct {
	Contract *FrostWalletRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts              // Call options to use throughout this session
}

// FrostWalletRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type FrostWalletRegistryTransactorSession struct {
	Contract     *FrostWalletRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts              // Transaction auth options to use throughout this session
}

// FrostWalletRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type FrostWalletRegistryRaw struct {
	Contract *FrostWalletRegistry // Generic contract binding to access the raw methods on
}

// FrostWalletRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type FrostWalletRegistryCallerRaw struct {
	Contract *FrostWalletRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// FrostWalletRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type FrostWalletRegistryTransactorRaw struct {
	Contract *FrostWalletRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewFrostWalletRegistry creates a new instance of FrostWalletRegistry, bound to a specific deployed contract.
func NewFrostWalletRegistry(address common.Address, backend bind.ContractBackend) (*FrostWalletRegistry, error) {
	contract, err := bindFrostWalletRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistry{FrostWalletRegistryCaller: FrostWalletRegistryCaller{contract: contract}, FrostWalletRegistryTransactor: FrostWalletRegistryTransactor{contract: contract}, FrostWalletRegistryFilterer: FrostWalletRegistryFilterer{contract: contract}}, nil
}

// NewFrostWalletRegistryCaller creates a new read-only instance of FrostWalletRegistry, bound to a specific deployed contract.
func NewFrostWalletRegistryCaller(address common.Address, caller bind.ContractCaller) (*FrostWalletRegistryCaller, error) {
	contract, err := bindFrostWalletRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryCaller{contract: contract}, nil
}

// NewFrostWalletRegistryTransactor creates a new write-only instance of FrostWalletRegistry, bound to a specific deployed contract.
func NewFrostWalletRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*FrostWalletRegistryTransactor, error) {
	contract, err := bindFrostWalletRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryTransactor{contract: contract}, nil
}

// NewFrostWalletRegistryFilterer creates a new log filterer instance of FrostWalletRegistry, bound to a specific deployed contract.
func NewFrostWalletRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*FrostWalletRegistryFilterer, error) {
	contract, err := bindFrostWalletRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryFilterer{contract: contract}, nil
}

// bindFrostWalletRegistry binds a generic wrapper to an already deployed contract.
func bindFrostWalletRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := FrostWalletRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FrostWalletRegistry *FrostWalletRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FrostWalletRegistry.Contract.FrostWalletRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FrostWalletRegistry *FrostWalletRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.FrostWalletRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FrostWalletRegistry *FrostWalletRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.FrostWalletRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FrostWalletRegistry *FrostWalletRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FrostWalletRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FrostWalletRegistry *FrostWalletRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FrostWalletRegistry *FrostWalletRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.contract.Transact(opts, method, params...)
}

// AuthorizationParameters is a free data retrieval call binding the contract method 0x7b14729e.
//
// Solidity: function authorizationParameters() view returns(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) AuthorizationParameters(opts *bind.CallOpts) (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "authorizationParameters")

	outstruct := new(struct {
		MinimumAuthorization              *big.Int
		AuthorizationDecreaseDelay        uint64
		AuthorizationDecreaseChangePeriod uint64
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.MinimumAuthorization = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.AuthorizationDecreaseDelay = *abi.ConvertType(out[1], new(uint64)).(*uint64)
	outstruct.AuthorizationDecreaseChangePeriod = *abi.ConvertType(out[2], new(uint64)).(*uint64)

	return *outstruct, err

}

// AuthorizationParameters is a free data retrieval call binding the contract method 0x7b14729e.
//
// Solidity: function authorizationParameters() view returns(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_FrostWalletRegistry *FrostWalletRegistrySession) AuthorizationParameters() (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	return _FrostWalletRegistry.Contract.AuthorizationParameters(&_FrostWalletRegistry.CallOpts)
}

// AuthorizationParameters is a free data retrieval call binding the contract method 0x7b14729e.
//
// Solidity: function authorizationParameters() view returns(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) AuthorizationParameters() (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	return _FrostWalletRegistry.Contract.AuthorizationParameters(&_FrostWalletRegistry.CallOpts)
}

// AuthorizationSource is a free data retrieval call binding the contract method 0x0a3abae9.
//
// Solidity: function authorizationSource() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) AuthorizationSource(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "authorizationSource")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// AuthorizationSource is a free data retrieval call binding the contract method 0x0a3abae9.
//
// Solidity: function authorizationSource() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) AuthorizationSource() (common.Address, error) {
	return _FrostWalletRegistry.Contract.AuthorizationSource(&_FrostWalletRegistry.CallOpts)
}

// AuthorizationSource is a free data retrieval call binding the contract method 0x0a3abae9.
//
// Solidity: function authorizationSource() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) AuthorizationSource() (common.Address, error) {
	return _FrostWalletRegistry.Contract.AuthorizationSource(&_FrostWalletRegistry.CallOpts)
}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) AvailableRewards(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "availableRewards", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistrySession) AvailableRewards(stakingProvider common.Address) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.AvailableRewards(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) AvailableRewards(stakingProvider common.Address) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.AvailableRewards(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_FrostWalletRegistry *FrostWalletRegistryCaller) DkgParameters(opts *bind.CallOpts) (FrostDkgParameters, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "dkgParameters")

	if err != nil {
		return *new(FrostDkgParameters), err
	}

	out0 := *abi.ConvertType(out[0], new(FrostDkgParameters)).(*FrostDkgParameters)

	return out0, err

}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_FrostWalletRegistry *FrostWalletRegistrySession) DkgParameters() (FrostDkgParameters, error) {
	return _FrostWalletRegistry.Contract.DkgParameters(&_FrostWalletRegistry.CallOpts)
}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) DkgParameters() (FrostDkgParameters, error) {
	return _FrostWalletRegistry.Contract.DkgParameters(&_FrostWalletRegistry.CallOpts)
}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) EligibleStake(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "eligibleStake", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistrySession) EligibleStake(stakingProvider common.Address) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.EligibleStake(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) EligibleStake(stakingProvider common.Address) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.EligibleStake(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) GasParameters(opts *bind.CallOpts) (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
}, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "gasParameters")

	outstruct := new(struct {
		DkgResultSubmissionGas            *big.Int
		DkgResultApprovalGasOffset        *big.Int
		NotifyOperatorInactivityGasOffset *big.Int
		NotifySeedTimeoutGasOffset        *big.Int
		NotifyDkgTimeoutNegativeGasOffset *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.DkgResultSubmissionGas = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.DkgResultApprovalGasOffset = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.NotifyOperatorInactivityGasOffset = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.NotifySeedTimeoutGasOffset = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.NotifyDkgTimeoutNegativeGasOffset = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_FrostWalletRegistry *FrostWalletRegistrySession) GasParameters() (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
}, error) {
	return _FrostWalletRegistry.Contract.GasParameters(&_FrostWalletRegistry.CallOpts)
}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) GasParameters() (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
}, error) {
	return _FrostWalletRegistry.Contract.GasParameters(&_FrostWalletRegistry.CallOpts)
}

// GetWallet is a free data retrieval call binding the contract method 0x789d392a.
//
// Solidity: function getWallet(bytes32 walletID) view returns((bytes32,bytes32))
func (_FrostWalletRegistry *FrostWalletRegistryCaller) GetWallet(opts *bind.CallOpts, walletID [32]byte) (FrostRegistryWalletsWallet, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "getWallet", walletID)

	if err != nil {
		return *new(FrostRegistryWalletsWallet), err
	}

	out0 := *abi.ConvertType(out[0], new(FrostRegistryWalletsWallet)).(*FrostRegistryWalletsWallet)

	return out0, err

}

// GetWallet is a free data retrieval call binding the contract method 0x789d392a.
//
// Solidity: function getWallet(bytes32 walletID) view returns((bytes32,bytes32))
func (_FrostWalletRegistry *FrostWalletRegistrySession) GetWallet(walletID [32]byte) (FrostRegistryWalletsWallet, error) {
	return _FrostWalletRegistry.Contract.GetWallet(&_FrostWalletRegistry.CallOpts, walletID)
}

// GetWallet is a free data retrieval call binding the contract method 0x789d392a.
//
// Solidity: function getWallet(bytes32 walletID) view returns((bytes32,bytes32))
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) GetWallet(walletID [32]byte) (FrostRegistryWalletsWallet, error) {
	return _FrostWalletRegistry.Contract.GetWallet(&_FrostWalletRegistry.CallOpts, walletID)
}

// GetWalletCreationState is a free data retrieval call binding the contract method 0xcc562388.
//
// Solidity: function getWalletCreationState() view returns(uint8)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) GetWalletCreationState(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "getWalletCreationState")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetWalletCreationState is a free data retrieval call binding the contract method 0xcc562388.
//
// Solidity: function getWalletCreationState() view returns(uint8)
func (_FrostWalletRegistry *FrostWalletRegistrySession) GetWalletCreationState() (uint8, error) {
	return _FrostWalletRegistry.Contract.GetWalletCreationState(&_FrostWalletRegistry.CallOpts)
}

// GetWalletCreationState is a free data retrieval call binding the contract method 0xcc562388.
//
// Solidity: function getWalletCreationState() view returns(uint8)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) GetWalletCreationState() (uint8, error) {
	return _FrostWalletRegistry.Contract.GetWalletCreationState(&_FrostWalletRegistry.CallOpts)
}

// GetWalletXOnlyOutputKey is a free data retrieval call binding the contract method 0x13bd580a.
//
// Solidity: function getWalletXOnlyOutputKey(bytes32 walletID) view returns(bytes32)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) GetWalletXOnlyOutputKey(opts *bind.CallOpts, walletID [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "getWalletXOnlyOutputKey", walletID)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetWalletXOnlyOutputKey is a free data retrieval call binding the contract method 0x13bd580a.
//
// Solidity: function getWalletXOnlyOutputKey(bytes32 walletID) view returns(bytes32)
func (_FrostWalletRegistry *FrostWalletRegistrySession) GetWalletXOnlyOutputKey(walletID [32]byte) ([32]byte, error) {
	return _FrostWalletRegistry.Contract.GetWalletXOnlyOutputKey(&_FrostWalletRegistry.CallOpts, walletID)
}

// GetWalletXOnlyOutputKey is a free data retrieval call binding the contract method 0x13bd580a.
//
// Solidity: function getWalletXOnlyOutputKey(bytes32 walletID) view returns(bytes32)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) GetWalletXOnlyOutputKey(walletID [32]byte) ([32]byte, error) {
	return _FrostWalletRegistry.Contract.GetWalletXOnlyOutputKey(&_FrostWalletRegistry.CallOpts, walletID)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) Governance(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "governance")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) Governance() (common.Address, error) {
	return _FrostWalletRegistry.Contract.Governance(&_FrostWalletRegistry.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) Governance() (common.Address, error) {
	return _FrostWalletRegistry.Contract.Governance(&_FrostWalletRegistry.CallOpts)
}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) HasDkgTimedOut(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "hasDkgTimedOut")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistrySession) HasDkgTimedOut() (bool, error) {
	return _FrostWalletRegistry.Contract.HasDkgTimedOut(&_FrostWalletRegistry.CallOpts)
}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) HasDkgTimedOut() (bool, error) {
	return _FrostWalletRegistry.Contract.HasDkgTimedOut(&_FrostWalletRegistry.CallOpts)
}

// HasSeedTimedOut is a free data retrieval call binding the contract method 0x770124d3.
//
// Solidity: function hasSeedTimedOut() view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) HasSeedTimedOut(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "hasSeedTimedOut")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasSeedTimedOut is a free data retrieval call binding the contract method 0x770124d3.
//
// Solidity: function hasSeedTimedOut() view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistrySession) HasSeedTimedOut() (bool, error) {
	return _FrostWalletRegistry.Contract.HasSeedTimedOut(&_FrostWalletRegistry.CallOpts)
}

// HasSeedTimedOut is a free data retrieval call binding the contract method 0x770124d3.
//
// Solidity: function hasSeedTimedOut() view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) HasSeedTimedOut() (bool, error) {
	return _FrostWalletRegistry.Contract.HasSeedTimedOut(&_FrostWalletRegistry.CallOpts)
}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0x830f9e02.
//
// Solidity: function inactivityClaimNonce(bytes32 ) view returns(uint256)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) InactivityClaimNonce(opts *bind.CallOpts, arg0 [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "inactivityClaimNonce", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0x830f9e02.
//
// Solidity: function inactivityClaimNonce(bytes32 ) view returns(uint256)
func (_FrostWalletRegistry *FrostWalletRegistrySession) InactivityClaimNonce(arg0 [32]byte) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.InactivityClaimNonce(&_FrostWalletRegistry.CallOpts, arg0)
}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0x830f9e02.
//
// Solidity: function inactivityClaimNonce(bytes32 ) view returns(uint256)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) InactivityClaimNonce(arg0 [32]byte) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.InactivityClaimNonce(&_FrostWalletRegistry.CallOpts, arg0)
}

// IsDkgResultValid is a free data retrieval call binding the contract method 0x3b74e062.
//
// Solidity: function isDkgResultValid((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) view returns(bool, string)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) IsDkgResultValid(opts *bind.CallOpts, result FrostDkgResult) (bool, string, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "isDkgResultValid", result)

	if err != nil {
		return *new(bool), *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	out1 := *abi.ConvertType(out[1], new(string)).(*string)

	return out0, out1, err

}

// IsDkgResultValid is a free data retrieval call binding the contract method 0x3b74e062.
//
// Solidity: function isDkgResultValid((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) view returns(bool, string)
func (_FrostWalletRegistry *FrostWalletRegistrySession) IsDkgResultValid(result FrostDkgResult) (bool, string, error) {
	return _FrostWalletRegistry.Contract.IsDkgResultValid(&_FrostWalletRegistry.CallOpts, result)
}

// IsDkgResultValid is a free data retrieval call binding the contract method 0x3b74e062.
//
// Solidity: function isDkgResultValid((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) view returns(bool, string)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) IsDkgResultValid(result FrostDkgResult) (bool, string, error) {
	return _FrostWalletRegistry.Contract.IsDkgResultValid(&_FrostWalletRegistry.CallOpts, result)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) IsOperatorInPool(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "isOperatorInPool", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistrySession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _FrostWalletRegistry.Contract.IsOperatorInPool(&_FrostWalletRegistry.CallOpts, operator)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _FrostWalletRegistry.Contract.IsOperatorInPool(&_FrostWalletRegistry.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) IsOperatorUpToDate(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "isOperatorUpToDate", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistrySession) IsOperatorUpToDate(operator common.Address) (bool, error) {
	return _FrostWalletRegistry.Contract.IsOperatorUpToDate(&_FrostWalletRegistry.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) IsOperatorUpToDate(operator common.Address) (bool, error) {
	return _FrostWalletRegistry.Contract.IsOperatorUpToDate(&_FrostWalletRegistry.CallOpts, operator)
}

// IsWalletMember is a free data retrieval call binding the contract method 0xdf07ce59.
//
// Solidity: function isWalletMember(bytes32 walletID, uint32[] walletMembersIDs, address operator, uint256 walletMemberIndex) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) IsWalletMember(opts *bind.CallOpts, walletID [32]byte, walletMembersIDs []uint32, operator common.Address, walletMemberIndex *big.Int) (bool, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "isWalletMember", walletID, walletMembersIDs, operator, walletMemberIndex)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsWalletMember is a free data retrieval call binding the contract method 0xdf07ce59.
//
// Solidity: function isWalletMember(bytes32 walletID, uint32[] walletMembersIDs, address operator, uint256 walletMemberIndex) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistrySession) IsWalletMember(walletID [32]byte, walletMembersIDs []uint32, operator common.Address, walletMemberIndex *big.Int) (bool, error) {
	return _FrostWalletRegistry.Contract.IsWalletMember(&_FrostWalletRegistry.CallOpts, walletID, walletMembersIDs, operator, walletMemberIndex)
}

// IsWalletMember is a free data retrieval call binding the contract method 0xdf07ce59.
//
// Solidity: function isWalletMember(bytes32 walletID, uint32[] walletMembersIDs, address operator, uint256 walletMemberIndex) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) IsWalletMember(walletID [32]byte, walletMembersIDs []uint32, operator common.Address, walletMemberIndex *big.Int) (bool, error) {
	return _FrostWalletRegistry.Contract.IsWalletMember(&_FrostWalletRegistry.CallOpts, walletID, walletMembersIDs, operator, walletMemberIndex)
}

// IsWalletRegistered is a free data retrieval call binding the contract method 0x4d99f473.
//
// Solidity: function isWalletRegistered(bytes32 walletID) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) IsWalletRegistered(opts *bind.CallOpts, walletID [32]byte) (bool, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "isWalletRegistered", walletID)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsWalletRegistered is a free data retrieval call binding the contract method 0x4d99f473.
//
// Solidity: function isWalletRegistered(bytes32 walletID) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistrySession) IsWalletRegistered(walletID [32]byte) (bool, error) {
	return _FrostWalletRegistry.Contract.IsWalletRegistered(&_FrostWalletRegistry.CallOpts, walletID)
}

// IsWalletRegistered is a free data retrieval call binding the contract method 0x4d99f473.
//
// Solidity: function isWalletRegistered(bytes32 walletID) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) IsWalletRegistered(walletID [32]byte) (bool, error) {
	return _FrostWalletRegistry.Contract.IsWalletRegistered(&_FrostWalletRegistry.CallOpts, walletID)
}

// LifecycleOwner is a free data retrieval call binding the contract method 0x7780dea1.
//
// Solidity: function lifecycleOwner() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) LifecycleOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "lifecycleOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// LifecycleOwner is a free data retrieval call binding the contract method 0x7780dea1.
//
// Solidity: function lifecycleOwner() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) LifecycleOwner() (common.Address, error) {
	return _FrostWalletRegistry.Contract.LifecycleOwner(&_FrostWalletRegistry.CallOpts)
}

// LifecycleOwner is a free data retrieval call binding the contract method 0x7780dea1.
//
// Solidity: function lifecycleOwner() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) LifecycleOwner() (common.Address, error) {
	return _FrostWalletRegistry.Contract.LifecycleOwner(&_FrostWalletRegistry.CallOpts)
}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) MinimumAuthorization(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "minimumAuthorization")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistrySession) MinimumAuthorization() (*big.Int, error) {
	return _FrostWalletRegistry.Contract.MinimumAuthorization(&_FrostWalletRegistry.CallOpts)
}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) MinimumAuthorization() (*big.Int, error) {
	return _FrostWalletRegistry.Contract.MinimumAuthorization(&_FrostWalletRegistry.CallOpts)
}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) OperatorToStakingProvider(opts *bind.CallOpts, operator common.Address) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "operatorToStakingProvider", operator)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) OperatorToStakingProvider(operator common.Address) (common.Address, error) {
	return _FrostWalletRegistry.Contract.OperatorToStakingProvider(&_FrostWalletRegistry.CallOpts, operator)
}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) OperatorToStakingProvider(operator common.Address) (common.Address, error) {
	return _FrostWalletRegistry.Contract.OperatorToStakingProvider(&_FrostWalletRegistry.CallOpts, operator)
}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) PendingAuthorizationDecrease(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "pendingAuthorizationDecrease", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistrySession) PendingAuthorizationDecrease(stakingProvider common.Address) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.PendingAuthorizationDecrease(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) PendingAuthorizationDecrease(stakingProvider common.Address) (*big.Int, error) {
	return _FrostWalletRegistry.Contract.PendingAuthorizationDecrease(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// RandomBeacon is a free data retrieval call binding the contract method 0x153622b3.
//
// Solidity: function randomBeacon() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) RandomBeacon(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "randomBeacon")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// RandomBeacon is a free data retrieval call binding the contract method 0x153622b3.
//
// Solidity: function randomBeacon() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) RandomBeacon() (common.Address, error) {
	return _FrostWalletRegistry.Contract.RandomBeacon(&_FrostWalletRegistry.CallOpts)
}

// RandomBeacon is a free data retrieval call binding the contract method 0x153622b3.
//
// Solidity: function randomBeacon() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) RandomBeacon() (common.Address, error) {
	return _FrostWalletRegistry.Contract.RandomBeacon(&_FrostWalletRegistry.CallOpts)
}

// Registered is a free data retrieval call binding the contract method 0x5524d548.
//
// Solidity: function registered(bytes32 ) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) Registered(opts *bind.CallOpts, arg0 [32]byte) (bool, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "registered", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Registered is a free data retrieval call binding the contract method 0x5524d548.
//
// Solidity: function registered(bytes32 ) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistrySession) Registered(arg0 [32]byte) (bool, error) {
	return _FrostWalletRegistry.Contract.Registered(&_FrostWalletRegistry.CallOpts, arg0)
}

// Registered is a free data retrieval call binding the contract method 0x5524d548.
//
// Solidity: function registered(bytes32 ) view returns(bool)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) Registered(arg0 [32]byte) (bool, error) {
	return _FrostWalletRegistry.Contract.Registered(&_FrostWalletRegistry.CallOpts, arg0)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) ReimbursementPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "reimbursementPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) ReimbursementPool() (common.Address, error) {
	return _FrostWalletRegistry.Contract.ReimbursementPool(&_FrostWalletRegistry.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) ReimbursementPool() (common.Address, error) {
	return _FrostWalletRegistry.Contract.ReimbursementPool(&_FrostWalletRegistry.CallOpts)
}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) RemainingAuthorizationDecreaseDelay(opts *bind.CallOpts, stakingProvider common.Address) (uint64, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "remainingAuthorizationDecreaseDelay", stakingProvider)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_FrostWalletRegistry *FrostWalletRegistrySession) RemainingAuthorizationDecreaseDelay(stakingProvider common.Address) (uint64, error) {
	return _FrostWalletRegistry.Contract.RemainingAuthorizationDecreaseDelay(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) RemainingAuthorizationDecreaseDelay(stakingProvider common.Address) (uint64, error) {
	return _FrostWalletRegistry.Contract.RemainingAuthorizationDecreaseDelay(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) RewardParameters(opts *bind.CallOpts) (struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
}, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "rewardParameters")

	outstruct := new(struct {
		MaliciousDkgResultNotificationRewardMultiplier *big.Int
		SortitionPoolRewardsBanDuration                *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.MaliciousDkgResultNotificationRewardMultiplier = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.SortitionPoolRewardsBanDuration = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_FrostWalletRegistry *FrostWalletRegistrySession) RewardParameters() (struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
}, error) {
	return _FrostWalletRegistry.Contract.RewardParameters(&_FrostWalletRegistry.CallOpts)
}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) RewardParameters() (struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
}, error) {
	return _FrostWalletRegistry.Contract.RewardParameters(&_FrostWalletRegistry.CallOpts)
}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_FrostWalletRegistry *FrostWalletRegistryCaller) SelectGroup(opts *bind.CallOpts) ([]uint32, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "selectGroup")

	if err != nil {
		return *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint32)).(*[]uint32)

	return out0, err

}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_FrostWalletRegistry *FrostWalletRegistrySession) SelectGroup() ([]uint32, error) {
	return _FrostWalletRegistry.Contract.SelectGroup(&_FrostWalletRegistry.CallOpts)
}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) SelectGroup() ([]uint32, error) {
	return _FrostWalletRegistry.Contract.SelectGroup(&_FrostWalletRegistry.CallOpts)
}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 maliciousDkgResultSlashingAmount)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) SlashingParameters(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "slashingParameters")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 maliciousDkgResultSlashingAmount)
func (_FrostWalletRegistry *FrostWalletRegistrySession) SlashingParameters() (*big.Int, error) {
	return _FrostWalletRegistry.Contract.SlashingParameters(&_FrostWalletRegistry.CallOpts)
}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 maliciousDkgResultSlashingAmount)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) SlashingParameters() (*big.Int, error) {
	return _FrostWalletRegistry.Contract.SlashingParameters(&_FrostWalletRegistry.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) SortitionPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "sortitionPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) SortitionPool() (common.Address, error) {
	return _FrostWalletRegistry.Contract.SortitionPool(&_FrostWalletRegistry.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) SortitionPool() (common.Address, error) {
	return _FrostWalletRegistry.Contract.SortitionPool(&_FrostWalletRegistry.CallOpts)
}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) StakingProviderToOperator(opts *bind.CallOpts, stakingProvider common.Address) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "stakingProviderToOperator", stakingProvider)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) StakingProviderToOperator(stakingProvider common.Address) (common.Address, error) {
	return _FrostWalletRegistry.Contract.StakingProviderToOperator(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) StakingProviderToOperator(stakingProvider common.Address) (common.Address, error) {
	return _FrostWalletRegistry.Contract.StakingProviderToOperator(&_FrostWalletRegistry.CallOpts, stakingProvider)
}

// WalletOwner is a free data retrieval call binding the contract method 0x1ae879e8.
//
// Solidity: function walletOwner() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) WalletOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "walletOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// WalletOwner is a free data retrieval call binding the contract method 0x1ae879e8.
//
// Solidity: function walletOwner() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistrySession) WalletOwner() (common.Address, error) {
	return _FrostWalletRegistry.Contract.WalletOwner(&_FrostWalletRegistry.CallOpts)
}

// WalletOwner is a free data retrieval call binding the contract method 0x1ae879e8.
//
// Solidity: function walletOwner() view returns(address)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) WalletOwner() (common.Address, error) {
	return _FrostWalletRegistry.Contract.WalletOwner(&_FrostWalletRegistry.CallOpts)
}

// BeaconCallback is a paid mutator transaction binding the contract method 0x6febd464.
//
// Solidity: function __beaconCallback(uint256 relayEntry, uint256 ) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) BeaconCallback(opts *bind.TransactOpts, relayEntry *big.Int, arg1 *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "__beaconCallback", relayEntry, arg1)
}

// BeaconCallback is a paid mutator transaction binding the contract method 0x6febd464.
//
// Solidity: function __beaconCallback(uint256 relayEntry, uint256 ) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) BeaconCallback(relayEntry *big.Int, arg1 *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.BeaconCallback(&_FrostWalletRegistry.TransactOpts, relayEntry, arg1)
}

// BeaconCallback is a paid mutator transaction binding the contract method 0x6febd464.
//
// Solidity: function __beaconCallback(uint256 relayEntry, uint256 ) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) BeaconCallback(relayEntry *big.Int, arg1 *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.BeaconCallback(&_FrostWalletRegistry.TransactOpts, relayEntry, arg1)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) ApproveAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "approveAuthorizationDecrease", stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ApproveAuthorizationDecrease(&_FrostWalletRegistry.TransactOpts, stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ApproveAuthorizationDecrease(&_FrostWalletRegistry.TransactOpts, stakingProvider)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0xcf2feddd.
//
// Solidity: function approveDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) ApproveDkgResult(opts *bind.TransactOpts, dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "approveDkgResult", dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0xcf2feddd.
//
// Solidity: function approveDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) ApproveDkgResult(dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ApproveDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0xcf2feddd.
//
// Solidity: function approveDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) ApproveDkgResult(dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ApproveDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) AuthorizationDecreaseRequested(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "authorizationDecreaseRequested", stakingProvider, fromAmount, toAmount)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) AuthorizationDecreaseRequested(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.AuthorizationDecreaseRequested(&_FrostWalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) AuthorizationDecreaseRequested(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.AuthorizationDecreaseRequested(&_FrostWalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) AuthorizationIncreased(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "authorizationIncreased", stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) AuthorizationIncreased(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.AuthorizationIncreased(&_FrostWalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) AuthorizationIncreased(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.AuthorizationIncreased(&_FrostWalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x24ac833e.
//
// Solidity: function challengeDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) ChallengeDkgResult(opts *bind.TransactOpts, dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "challengeDkgResult", dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x24ac833e.
//
// Solidity: function challengeDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) ChallengeDkgResult(dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ChallengeDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x24ac833e.
//
// Solidity: function challengeDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) ChallengeDkgResult(dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ChallengeDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// CloseWallet is a paid mutator transaction binding the contract method 0x343bb927.
//
// Solidity: function closeWallet(bytes32 walletID) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) CloseWallet(opts *bind.TransactOpts, walletID [32]byte) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "closeWallet", walletID)
}

// CloseWallet is a paid mutator transaction binding the contract method 0x343bb927.
//
// Solidity: function closeWallet(bytes32 walletID) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) CloseWallet(walletID [32]byte) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.CloseWallet(&_FrostWalletRegistry.TransactOpts, walletID)
}

// CloseWallet is a paid mutator transaction binding the contract method 0x343bb927.
//
// Solidity: function closeWallet(bytes32 walletID) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) CloseWallet(walletID [32]byte) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.CloseWallet(&_FrostWalletRegistry.TransactOpts, walletID)
}

// Initialize is a paid mutator transaction binding the contract method 0xf8c8765e.
//
// Solidity: function initialize(address _ecdsaDkgValidator, address _randomBeacon, address _reimbursementPool, address _bridge) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) Initialize(opts *bind.TransactOpts, _ecdsaDkgValidator common.Address, _randomBeacon common.Address, _reimbursementPool common.Address, _bridge common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "initialize", _ecdsaDkgValidator, _randomBeacon, _reimbursementPool, _bridge)
}

// Initialize is a paid mutator transaction binding the contract method 0xf8c8765e.
//
// Solidity: function initialize(address _ecdsaDkgValidator, address _randomBeacon, address _reimbursementPool, address _bridge) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) Initialize(_ecdsaDkgValidator common.Address, _randomBeacon common.Address, _reimbursementPool common.Address, _bridge common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.Initialize(&_FrostWalletRegistry.TransactOpts, _ecdsaDkgValidator, _randomBeacon, _reimbursementPool, _bridge)
}

// Initialize is a paid mutator transaction binding the contract method 0xf8c8765e.
//
// Solidity: function initialize(address _ecdsaDkgValidator, address _randomBeacon, address _reimbursementPool, address _bridge) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) Initialize(_ecdsaDkgValidator common.Address, _randomBeacon common.Address, _reimbursementPool common.Address, _bridge common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.Initialize(&_FrostWalletRegistry.TransactOpts, _ecdsaDkgValidator, _randomBeacon, _reimbursementPool, _bridge)
}

// InitializeV2 is a paid mutator transaction binding the contract method 0x29b6eca9.
//
// Solidity: function initializeV2(address _authorizationSource) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) InitializeV2(opts *bind.TransactOpts, _authorizationSource common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "initializeV2", _authorizationSource)
}

// InitializeV2 is a paid mutator transaction binding the contract method 0x29b6eca9.
//
// Solidity: function initializeV2(address _authorizationSource) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) InitializeV2(_authorizationSource common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.InitializeV2(&_FrostWalletRegistry.TransactOpts, _authorizationSource)
}

// InitializeV2 is a paid mutator transaction binding the contract method 0x29b6eca9.
//
// Solidity: function initializeV2(address _authorizationSource) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) InitializeV2(_authorizationSource common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.InitializeV2(&_FrostWalletRegistry.TransactOpts, _authorizationSource)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) InvoluntaryAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "involuntaryAuthorizationDecrease", stakingProvider, fromAmount, toAmount)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) InvoluntaryAuthorizationDecrease(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.InvoluntaryAuthorizationDecrease(&_FrostWalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) InvoluntaryAuthorizationDecrease(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.InvoluntaryAuthorizationDecrease(&_FrostWalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) JoinSortitionPool(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "joinSortitionPool")
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) JoinSortitionPool() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.JoinSortitionPool(&_FrostWalletRegistry.TransactOpts)
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) JoinSortitionPool() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.JoinSortitionPool(&_FrostWalletRegistry.TransactOpts)
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) NotifyDkgTimeout(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "notifyDkgTimeout")
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) NotifyDkgTimeout() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.NotifyDkgTimeout(&_FrostWalletRegistry.TransactOpts)
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) NotifyDkgTimeout() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.NotifyDkgTimeout(&_FrostWalletRegistry.TransactOpts)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0x9879d19b.
//
// Solidity: function notifyOperatorInactivity((bytes32,uint256[],bool,bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) NotifyOperatorInactivity(opts *bind.TransactOpts, claim FrostInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "notifyOperatorInactivity", claim, nonce, groupMembers)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0x9879d19b.
//
// Solidity: function notifyOperatorInactivity((bytes32,uint256[],bool,bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) NotifyOperatorInactivity(claim FrostInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.NotifyOperatorInactivity(&_FrostWalletRegistry.TransactOpts, claim, nonce, groupMembers)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0x9879d19b.
//
// Solidity: function notifyOperatorInactivity((bytes32,uint256[],bool,bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) NotifyOperatorInactivity(claim FrostInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.NotifyOperatorInactivity(&_FrostWalletRegistry.TransactOpts, claim, nonce, groupMembers)
}

// NotifySeedTimeout is a paid mutator transaction binding the contract method 0xb13b55b2.
//
// Solidity: function notifySeedTimeout() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) NotifySeedTimeout(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "notifySeedTimeout")
}

// NotifySeedTimeout is a paid mutator transaction binding the contract method 0xb13b55b2.
//
// Solidity: function notifySeedTimeout() returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) NotifySeedTimeout() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.NotifySeedTimeout(&_FrostWalletRegistry.TransactOpts)
}

// NotifySeedTimeout is a paid mutator transaction binding the contract method 0xb13b55b2.
//
// Solidity: function notifySeedTimeout() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) NotifySeedTimeout() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.NotifySeedTimeout(&_FrostWalletRegistry.TransactOpts)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) RegisterOperator(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "registerOperator", operator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) RegisterOperator(operator common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.RegisterOperator(&_FrostWalletRegistry.TransactOpts, operator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) RegisterOperator(operator common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.RegisterOperator(&_FrostWalletRegistry.TransactOpts, operator)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0x72cc8c6d.
//
// Solidity: function requestNewWallet() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) RequestNewWallet(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "requestNewWallet")
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0x72cc8c6d.
//
// Solidity: function requestNewWallet() returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) RequestNewWallet() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.RequestNewWallet(&_FrostWalletRegistry.TransactOpts)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0x72cc8c6d.
//
// Solidity: function requestNewWallet() returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) RequestNewWallet() (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.RequestNewWallet(&_FrostWalletRegistry.TransactOpts)
}

// Seize is a paid mutator transaction binding the contract method 0xd8dc404d.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, bytes32 walletID, uint32[] walletMembersIDs) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) Seize(opts *bind.TransactOpts, amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, walletID [32]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "seize", amount, rewardMultiplier, notifier, walletID, walletMembersIDs)
}

// Seize is a paid mutator transaction binding the contract method 0xd8dc404d.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, bytes32 walletID, uint32[] walletMembersIDs) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) Seize(amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, walletID [32]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.Seize(&_FrostWalletRegistry.TransactOpts, amount, rewardMultiplier, notifier, walletID, walletMembersIDs)
}

// Seize is a paid mutator transaction binding the contract method 0xd8dc404d.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, bytes32 walletID, uint32[] walletMembersIDs) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) Seize(amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, walletID [32]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.Seize(&_FrostWalletRegistry.TransactOpts, amount, rewardMultiplier, notifier, walletID, walletMembersIDs)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x55129e3a.
//
// Solidity: function submitDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) SubmitDkgResult(opts *bind.TransactOpts, dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "submitDkgResult", dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x55129e3a.
//
// Solidity: function submitDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) SubmitDkgResult(dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.SubmitDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x55129e3a.
//
// Solidity: function submitDkgResult((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) SubmitDkgResult(dkgResult FrostDkgResult) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.SubmitDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) TransferGovernance(opts *bind.TransactOpts, newGovernance common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "transferGovernance", newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.TransferGovernance(&_FrostWalletRegistry.TransactOpts, newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.TransferGovernance(&_FrostWalletRegistry.TransactOpts, newGovernance)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateAuthorizationParameters(opts *bind.TransactOpts, _minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateAuthorizationParameters", _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateAuthorizationParameters(_minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateAuthorizationParameters(&_FrostWalletRegistry.TransactOpts, _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateAuthorizationParameters(_minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateAuthorizationParameters(&_FrostWalletRegistry.TransactOpts, _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateDkgParameters is a paid mutator transaction binding the contract method 0x8dcbdf4a.
//
// Solidity: function updateDkgParameters(uint256 _seedTimeout, uint256 _resultChallengePeriodLength, uint256 _resultChallengeExtraGas, uint256 _resultSubmissionTimeout, uint256 _submitterPrecedencePeriodLength) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateDkgParameters(opts *bind.TransactOpts, _seedTimeout *big.Int, _resultChallengePeriodLength *big.Int, _resultChallengeExtraGas *big.Int, _resultSubmissionTimeout *big.Int, _submitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateDkgParameters", _seedTimeout, _resultChallengePeriodLength, _resultChallengeExtraGas, _resultSubmissionTimeout, _submitterPrecedencePeriodLength)
}

// UpdateDkgParameters is a paid mutator transaction binding the contract method 0x8dcbdf4a.
//
// Solidity: function updateDkgParameters(uint256 _seedTimeout, uint256 _resultChallengePeriodLength, uint256 _resultChallengeExtraGas, uint256 _resultSubmissionTimeout, uint256 _submitterPrecedencePeriodLength) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateDkgParameters(_seedTimeout *big.Int, _resultChallengePeriodLength *big.Int, _resultChallengeExtraGas *big.Int, _resultSubmissionTimeout *big.Int, _submitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateDkgParameters(&_FrostWalletRegistry.TransactOpts, _seedTimeout, _resultChallengePeriodLength, _resultChallengeExtraGas, _resultSubmissionTimeout, _submitterPrecedencePeriodLength)
}

// UpdateDkgParameters is a paid mutator transaction binding the contract method 0x8dcbdf4a.
//
// Solidity: function updateDkgParameters(uint256 _seedTimeout, uint256 _resultChallengePeriodLength, uint256 _resultChallengeExtraGas, uint256 _resultSubmissionTimeout, uint256 _submitterPrecedencePeriodLength) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateDkgParameters(_seedTimeout *big.Int, _resultChallengePeriodLength *big.Int, _resultChallengeExtraGas *big.Int, _resultSubmissionTimeout *big.Int, _submitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateDkgParameters(&_FrostWalletRegistry.TransactOpts, _seedTimeout, _resultChallengePeriodLength, _resultChallengeExtraGas, _resultSubmissionTimeout, _submitterPrecedencePeriodLength)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xc88e70f4.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateGasParameters(opts *bind.TransactOpts, dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, notifySeedTimeoutGasOffset *big.Int, notifyDkgTimeoutNegativeGasOffset *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateGasParameters", dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, notifySeedTimeoutGasOffset, notifyDkgTimeoutNegativeGasOffset)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xc88e70f4.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateGasParameters(dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, notifySeedTimeoutGasOffset *big.Int, notifyDkgTimeoutNegativeGasOffset *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateGasParameters(&_FrostWalletRegistry.TransactOpts, dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, notifySeedTimeoutGasOffset, notifyDkgTimeoutNegativeGasOffset)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xc88e70f4.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateGasParameters(dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, notifySeedTimeoutGasOffset *big.Int, notifyDkgTimeoutNegativeGasOffset *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateGasParameters(&_FrostWalletRegistry.TransactOpts, dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, notifySeedTimeoutGasOffset, notifyDkgTimeoutNegativeGasOffset)
}

// UpdateLifecycleOwner is a paid mutator transaction binding the contract method 0x5c776294.
//
// Solidity: function updateLifecycleOwner(address _lifecycleOwner) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateLifecycleOwner(opts *bind.TransactOpts, _lifecycleOwner common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateLifecycleOwner", _lifecycleOwner)
}

// UpdateLifecycleOwner is a paid mutator transaction binding the contract method 0x5c776294.
//
// Solidity: function updateLifecycleOwner(address _lifecycleOwner) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateLifecycleOwner(_lifecycleOwner common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateLifecycleOwner(&_FrostWalletRegistry.TransactOpts, _lifecycleOwner)
}

// UpdateLifecycleOwner is a paid mutator transaction binding the contract method 0x5c776294.
//
// Solidity: function updateLifecycleOwner(address _lifecycleOwner) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateLifecycleOwner(_lifecycleOwner common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateLifecycleOwner(&_FrostWalletRegistry.TransactOpts, _lifecycleOwner)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateOperatorStatus(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateOperatorStatus", operator)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateOperatorStatus(operator common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateOperatorStatus(&_FrostWalletRegistry.TransactOpts, operator)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateOperatorStatus(operator common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateOperatorStatus(&_FrostWalletRegistry.TransactOpts, operator)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateReimbursementPool(opts *bind.TransactOpts, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateReimbursementPool", _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateReimbursementPool(&_FrostWalletRegistry.TransactOpts, _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateReimbursementPool(&_FrostWalletRegistry.TransactOpts, _reimbursementPool)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x6c9ecd64.
//
// Solidity: function updateRewardParameters(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateRewardParameters(opts *bind.TransactOpts, maliciousDkgResultNotificationRewardMultiplier *big.Int, sortitionPoolRewardsBanDuration *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateRewardParameters", maliciousDkgResultNotificationRewardMultiplier, sortitionPoolRewardsBanDuration)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x6c9ecd64.
//
// Solidity: function updateRewardParameters(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateRewardParameters(maliciousDkgResultNotificationRewardMultiplier *big.Int, sortitionPoolRewardsBanDuration *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateRewardParameters(&_FrostWalletRegistry.TransactOpts, maliciousDkgResultNotificationRewardMultiplier, sortitionPoolRewardsBanDuration)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x6c9ecd64.
//
// Solidity: function updateRewardParameters(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateRewardParameters(maliciousDkgResultNotificationRewardMultiplier *big.Int, sortitionPoolRewardsBanDuration *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateRewardParameters(&_FrostWalletRegistry.TransactOpts, maliciousDkgResultNotificationRewardMultiplier, sortitionPoolRewardsBanDuration)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x227fd44f.
//
// Solidity: function updateSlashingParameters(uint96 maliciousDkgResultSlashingAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateSlashingParameters(opts *bind.TransactOpts, maliciousDkgResultSlashingAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateSlashingParameters", maliciousDkgResultSlashingAmount)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x227fd44f.
//
// Solidity: function updateSlashingParameters(uint96 maliciousDkgResultSlashingAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateSlashingParameters(maliciousDkgResultSlashingAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateSlashingParameters(&_FrostWalletRegistry.TransactOpts, maliciousDkgResultSlashingAmount)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x227fd44f.
//
// Solidity: function updateSlashingParameters(uint96 maliciousDkgResultSlashingAmount) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateSlashingParameters(maliciousDkgResultSlashingAmount *big.Int) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateSlashingParameters(&_FrostWalletRegistry.TransactOpts, maliciousDkgResultSlashingAmount)
}

// UpdateWalletOwner is a paid mutator transaction binding the contract method 0xd0bcc0e3.
//
// Solidity: function updateWalletOwner(address _walletOwner) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpdateWalletOwner(opts *bind.TransactOpts, _walletOwner common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "updateWalletOwner", _walletOwner)
}

// UpdateWalletOwner is a paid mutator transaction binding the contract method 0xd0bcc0e3.
//
// Solidity: function updateWalletOwner(address _walletOwner) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpdateWalletOwner(_walletOwner common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateWalletOwner(&_FrostWalletRegistry.TransactOpts, _walletOwner)
}

// UpdateWalletOwner is a paid mutator transaction binding the contract method 0xd0bcc0e3.
//
// Solidity: function updateWalletOwner(address _walletOwner) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpdateWalletOwner(_walletOwner common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpdateWalletOwner(&_FrostWalletRegistry.TransactOpts, _walletOwner)
}

// UpgradeRandomBeacon is a paid mutator transaction binding the contract method 0x6b5f2bff.
//
// Solidity: function upgradeRandomBeacon(address _randomBeacon) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) UpgradeRandomBeacon(opts *bind.TransactOpts, _randomBeacon common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "upgradeRandomBeacon", _randomBeacon)
}

// UpgradeRandomBeacon is a paid mutator transaction binding the contract method 0x6b5f2bff.
//
// Solidity: function upgradeRandomBeacon(address _randomBeacon) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) UpgradeRandomBeacon(_randomBeacon common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpgradeRandomBeacon(&_FrostWalletRegistry.TransactOpts, _randomBeacon)
}

// UpgradeRandomBeacon is a paid mutator transaction binding the contract method 0x6b5f2bff.
//
// Solidity: function upgradeRandomBeacon(address _randomBeacon) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) UpgradeRandomBeacon(_randomBeacon common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.UpgradeRandomBeacon(&_FrostWalletRegistry.TransactOpts, _randomBeacon)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) WithdrawIneligibleRewards(opts *bind.TransactOpts, recipient common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "withdrawIneligibleRewards", recipient)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) WithdrawIneligibleRewards(recipient common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.WithdrawIneligibleRewards(&_FrostWalletRegistry.TransactOpts, recipient)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) WithdrawIneligibleRewards(recipient common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.WithdrawIneligibleRewards(&_FrostWalletRegistry.TransactOpts, recipient)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) WithdrawRewards(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "withdrawRewards", stakingProvider)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) WithdrawRewards(stakingProvider common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.WithdrawRewards(&_FrostWalletRegistry.TransactOpts, stakingProvider)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) WithdrawRewards(stakingProvider common.Address) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.WithdrawRewards(&_FrostWalletRegistry.TransactOpts, stakingProvider)
}

// FrostWalletRegistryAuthorizationDecreaseApprovedIterator is returned from FilterAuthorizationDecreaseApproved and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseApproved events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationDecreaseApprovedIterator struct {
	Event *FrostWalletRegistryAuthorizationDecreaseApproved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryAuthorizationDecreaseApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryAuthorizationDecreaseApproved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryAuthorizationDecreaseApproved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryAuthorizationDecreaseApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryAuthorizationDecreaseApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryAuthorizationDecreaseApproved represents a AuthorizationDecreaseApproved event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationDecreaseApproved struct {
	StakingProvider common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationDecreaseApproved is a free log retrieval operation binding the contract event 0x50270a522c2fef97b6b7385c2aa4a4518adda681530e0a1fe9f5e840f6f2cd9d.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterAuthorizationDecreaseApproved(opts *bind.FilterOpts, stakingProvider []common.Address) (*FrostWalletRegistryAuthorizationDecreaseApprovedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryAuthorizationDecreaseApprovedIterator{contract: _FrostWalletRegistry.contract, event: "AuthorizationDecreaseApproved", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseApproved is a free log subscription operation binding the contract event 0x50270a522c2fef97b6b7385c2aa4a4518adda681530e0a1fe9f5e840f6f2cd9d.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchAuthorizationDecreaseApproved(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryAuthorizationDecreaseApproved, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryAuthorizationDecreaseApproved)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAuthorizationDecreaseApproved is a log parse operation binding the contract event 0x50270a522c2fef97b6b7385c2aa4a4518adda681530e0a1fe9f5e840f6f2cd9d.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseAuthorizationDecreaseApproved(log types.Log) (*FrostWalletRegistryAuthorizationDecreaseApproved, error) {
	event := new(FrostWalletRegistryAuthorizationDecreaseApproved)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryAuthorizationDecreaseRequestedIterator is returned from FilterAuthorizationDecreaseRequested and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseRequested events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationDecreaseRequestedIterator struct {
	Event *FrostWalletRegistryAuthorizationDecreaseRequested // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryAuthorizationDecreaseRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryAuthorizationDecreaseRequested)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryAuthorizationDecreaseRequested)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryAuthorizationDecreaseRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryAuthorizationDecreaseRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryAuthorizationDecreaseRequested represents a AuthorizationDecreaseRequested event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationDecreaseRequested struct {
	StakingProvider common.Address
	Operator        common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	DecreasingAt    uint64
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationDecreaseRequested is a free log retrieval operation binding the contract event 0x545cbf267cef6fe43f11f6219417ab43a0e8e345adbaae5f626d9bc325e8535a.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount, uint64 decreasingAt)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterAuthorizationDecreaseRequested(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*FrostWalletRegistryAuthorizationDecreaseRequestedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryAuthorizationDecreaseRequestedIterator{contract: _FrostWalletRegistry.contract, event: "AuthorizationDecreaseRequested", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseRequested is a free log subscription operation binding the contract event 0x545cbf267cef6fe43f11f6219417ab43a0e8e345adbaae5f626d9bc325e8535a.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount, uint64 decreasingAt)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchAuthorizationDecreaseRequested(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryAuthorizationDecreaseRequested, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryAuthorizationDecreaseRequested)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAuthorizationDecreaseRequested is a log parse operation binding the contract event 0x545cbf267cef6fe43f11f6219417ab43a0e8e345adbaae5f626d9bc325e8535a.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount, uint64 decreasingAt)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseAuthorizationDecreaseRequested(log types.Log) (*FrostWalletRegistryAuthorizationDecreaseRequested, error) {
	event := new(FrostWalletRegistryAuthorizationDecreaseRequested)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryAuthorizationIncreasedIterator is returned from FilterAuthorizationIncreased and is used to iterate over the raw logs and unpacked data for AuthorizationIncreased events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationIncreasedIterator struct {
	Event *FrostWalletRegistryAuthorizationIncreased // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryAuthorizationIncreasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryAuthorizationIncreased)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryAuthorizationIncreased)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryAuthorizationIncreasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryAuthorizationIncreasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryAuthorizationIncreased represents a AuthorizationIncreased event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationIncreased struct {
	StakingProvider common.Address
	Operator        common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationIncreased is a free log retrieval operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterAuthorizationIncreased(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*FrostWalletRegistryAuthorizationIncreasedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "AuthorizationIncreased", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryAuthorizationIncreasedIterator{contract: _FrostWalletRegistry.contract, event: "AuthorizationIncreased", logs: logs, sub: sub}, nil
}

// WatchAuthorizationIncreased is a free log subscription operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchAuthorizationIncreased(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryAuthorizationIncreased, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "AuthorizationIncreased", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryAuthorizationIncreased)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAuthorizationIncreased is a log parse operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseAuthorizationIncreased(log types.Log) (*FrostWalletRegistryAuthorizationIncreased, error) {
	event := new(FrostWalletRegistryAuthorizationIncreased)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryAuthorizationParametersUpdatedIterator is returned from FilterAuthorizationParametersUpdated and is used to iterate over the raw logs and unpacked data for AuthorizationParametersUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationParametersUpdatedIterator struct {
	Event *FrostWalletRegistryAuthorizationParametersUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryAuthorizationParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryAuthorizationParametersUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryAuthorizationParametersUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryAuthorizationParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryAuthorizationParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryAuthorizationParametersUpdated represents a AuthorizationParametersUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryAuthorizationParametersUpdated struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
	Raw                               types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationParametersUpdated is a free log retrieval operation binding the contract event 0x544b726e42801bb47073854eeedae851903f66fe32a5bd24e626e10b90027b51.
//
// Solidity: event AuthorizationParametersUpdated(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterAuthorizationParametersUpdated(opts *bind.FilterOpts) (*FrostWalletRegistryAuthorizationParametersUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "AuthorizationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryAuthorizationParametersUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "AuthorizationParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchAuthorizationParametersUpdated is a free log subscription operation binding the contract event 0x544b726e42801bb47073854eeedae851903f66fe32a5bd24e626e10b90027b51.
//
// Solidity: event AuthorizationParametersUpdated(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchAuthorizationParametersUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryAuthorizationParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "AuthorizationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryAuthorizationParametersUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationParametersUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAuthorizationParametersUpdated is a log parse operation binding the contract event 0x544b726e42801bb47073854eeedae851903f66fe32a5bd24e626e10b90027b51.
//
// Solidity: event AuthorizationParametersUpdated(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseAuthorizationParametersUpdated(log types.Log) (*FrostWalletRegistryAuthorizationParametersUpdated, error) {
	event := new(FrostWalletRegistryAuthorizationParametersUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "AuthorizationParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgMaliciousResultSlashedIterator is returned from FilterDkgMaliciousResultSlashed and is used to iterate over the raw logs and unpacked data for DkgMaliciousResultSlashed events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgMaliciousResultSlashedIterator struct {
	Event *FrostWalletRegistryDkgMaliciousResultSlashed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgMaliciousResultSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgMaliciousResultSlashed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgMaliciousResultSlashed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgMaliciousResultSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgMaliciousResultSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgMaliciousResultSlashed represents a DkgMaliciousResultSlashed event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgMaliciousResultSlashed struct {
	ResultHash         [32]byte
	SlashingAmount     *big.Int
	MaliciousSubmitter common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterDkgMaliciousResultSlashed is a free log retrieval operation binding the contract event 0x88f76c659db78142f88e94db3ca791869495394c6c1b3d412ced9022dc97c9e3.
//
// Solidity: event DkgMaliciousResultSlashed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgMaliciousResultSlashed(opts *bind.FilterOpts, resultHash [][32]byte) (*FrostWalletRegistryDkgMaliciousResultSlashedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgMaliciousResultSlashed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgMaliciousResultSlashedIterator{contract: _FrostWalletRegistry.contract, event: "DkgMaliciousResultSlashed", logs: logs, sub: sub}, nil
}

// WatchDkgMaliciousResultSlashed is a free log subscription operation binding the contract event 0x88f76c659db78142f88e94db3ca791869495394c6c1b3d412ced9022dc97c9e3.
//
// Solidity: event DkgMaliciousResultSlashed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgMaliciousResultSlashed(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgMaliciousResultSlashed, resultHash [][32]byte) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgMaliciousResultSlashed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgMaliciousResultSlashed)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgMaliciousResultSlashed is a log parse operation binding the contract event 0x88f76c659db78142f88e94db3ca791869495394c6c1b3d412ced9022dc97c9e3.
//
// Solidity: event DkgMaliciousResultSlashed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgMaliciousResultSlashed(log types.Log) (*FrostWalletRegistryDkgMaliciousResultSlashed, error) {
	event := new(FrostWalletRegistryDkgMaliciousResultSlashed)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgMaliciousResultSlashingFailedIterator is returned from FilterDkgMaliciousResultSlashingFailed and is used to iterate over the raw logs and unpacked data for DkgMaliciousResultSlashingFailed events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgMaliciousResultSlashingFailedIterator struct {
	Event *FrostWalletRegistryDkgMaliciousResultSlashingFailed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgMaliciousResultSlashingFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgMaliciousResultSlashingFailed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgMaliciousResultSlashingFailed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgMaliciousResultSlashingFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgMaliciousResultSlashingFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgMaliciousResultSlashingFailed represents a DkgMaliciousResultSlashingFailed event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgMaliciousResultSlashingFailed struct {
	ResultHash         [32]byte
	SlashingAmount     *big.Int
	MaliciousSubmitter common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterDkgMaliciousResultSlashingFailed is a free log retrieval operation binding the contract event 0x14621289a12ab59e0737decc388bba91d929c723defb4682d5d19b9a12ecfecb.
//
// Solidity: event DkgMaliciousResultSlashingFailed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgMaliciousResultSlashingFailed(opts *bind.FilterOpts, resultHash [][32]byte) (*FrostWalletRegistryDkgMaliciousResultSlashingFailedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgMaliciousResultSlashingFailed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgMaliciousResultSlashingFailedIterator{contract: _FrostWalletRegistry.contract, event: "DkgMaliciousResultSlashingFailed", logs: logs, sub: sub}, nil
}

// WatchDkgMaliciousResultSlashingFailed is a free log subscription operation binding the contract event 0x14621289a12ab59e0737decc388bba91d929c723defb4682d5d19b9a12ecfecb.
//
// Solidity: event DkgMaliciousResultSlashingFailed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgMaliciousResultSlashingFailed(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgMaliciousResultSlashingFailed, resultHash [][32]byte) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgMaliciousResultSlashingFailed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgMaliciousResultSlashingFailed)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashingFailed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgMaliciousResultSlashingFailed is a log parse operation binding the contract event 0x14621289a12ab59e0737decc388bba91d929c723defb4682d5d19b9a12ecfecb.
//
// Solidity: event DkgMaliciousResultSlashingFailed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgMaliciousResultSlashingFailed(log types.Log) (*FrostWalletRegistryDkgMaliciousResultSlashingFailed, error) {
	event := new(FrostWalletRegistryDkgMaliciousResultSlashingFailed)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashingFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgParametersUpdatedIterator is returned from FilterDkgParametersUpdated and is used to iterate over the raw logs and unpacked data for DkgParametersUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgParametersUpdatedIterator struct {
	Event *FrostWalletRegistryDkgParametersUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgParametersUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgParametersUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgParametersUpdated represents a DkgParametersUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgParametersUpdated struct {
	SeedTimeout                           *big.Int
	ResultChallengePeriodLength           *big.Int
	ResultChallengeExtraGas               *big.Int
	ResultSubmissionTimeout               *big.Int
	ResultSubmitterPrecedencePeriodLength *big.Int
	Raw                                   types.Log // Blockchain specific contextual infos
}

// FilterDkgParametersUpdated is a free log retrieval operation binding the contract event 0x59ae8ed7b3a7e5f6dde4cff478f0ac0aa652c5edc4f4757b09a778a430b02c56.
//
// Solidity: event DkgParametersUpdated(uint256 seedTimeout, uint256 resultChallengePeriodLength, uint256 resultChallengeExtraGas, uint256 resultSubmissionTimeout, uint256 resultSubmitterPrecedencePeriodLength)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgParametersUpdated(opts *bind.FilterOpts) (*FrostWalletRegistryDkgParametersUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgParametersUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "DkgParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchDkgParametersUpdated is a free log subscription operation binding the contract event 0x59ae8ed7b3a7e5f6dde4cff478f0ac0aa652c5edc4f4757b09a778a430b02c56.
//
// Solidity: event DkgParametersUpdated(uint256 seedTimeout, uint256 resultChallengePeriodLength, uint256 resultChallengeExtraGas, uint256 resultSubmissionTimeout, uint256 resultSubmitterPrecedencePeriodLength)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgParametersUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgParametersUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgParametersUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgParametersUpdated is a log parse operation binding the contract event 0x59ae8ed7b3a7e5f6dde4cff478f0ac0aa652c5edc4f4757b09a778a430b02c56.
//
// Solidity: event DkgParametersUpdated(uint256 seedTimeout, uint256 resultChallengePeriodLength, uint256 resultChallengeExtraGas, uint256 resultSubmissionTimeout, uint256 resultSubmitterPrecedencePeriodLength)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgParametersUpdated(log types.Log) (*FrostWalletRegistryDkgParametersUpdated, error) {
	event := new(FrostWalletRegistryDkgParametersUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgResultApprovedIterator is returned from FilterDkgResultApproved and is used to iterate over the raw logs and unpacked data for DkgResultApproved events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgResultApprovedIterator struct {
	Event *FrostWalletRegistryDkgResultApproved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgResultApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgResultApproved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgResultApproved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgResultApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgResultApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgResultApproved represents a DkgResultApproved event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgResultApproved struct {
	ResultHash [32]byte
	Approver   common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultApproved is a free log retrieval operation binding the contract event 0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f.
//
// Solidity: event DkgResultApproved(bytes32 indexed resultHash, address indexed approver)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgResultApproved(opts *bind.FilterOpts, resultHash [][32]byte, approver []common.Address) (*FrostWalletRegistryDkgResultApprovedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var approverRule []interface{}
	for _, approverItem := range approver {
		approverRule = append(approverRule, approverItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgResultApproved", resultHashRule, approverRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgResultApprovedIterator{contract: _FrostWalletRegistry.contract, event: "DkgResultApproved", logs: logs, sub: sub}, nil
}

// WatchDkgResultApproved is a free log subscription operation binding the contract event 0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f.
//
// Solidity: event DkgResultApproved(bytes32 indexed resultHash, address indexed approver)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgResultApproved(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgResultApproved, resultHash [][32]byte, approver []common.Address) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var approverRule []interface{}
	for _, approverItem := range approver {
		approverRule = append(approverRule, approverItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgResultApproved", resultHashRule, approverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgResultApproved)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgResultApproved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgResultApproved is a log parse operation binding the contract event 0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f.
//
// Solidity: event DkgResultApproved(bytes32 indexed resultHash, address indexed approver)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgResultApproved(log types.Log) (*FrostWalletRegistryDkgResultApproved, error) {
	event := new(FrostWalletRegistryDkgResultApproved)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgResultApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgResultChallengedIterator is returned from FilterDkgResultChallenged and is used to iterate over the raw logs and unpacked data for DkgResultChallenged events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgResultChallengedIterator struct {
	Event *FrostWalletRegistryDkgResultChallenged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgResultChallengedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgResultChallenged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgResultChallenged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgResultChallengedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgResultChallengedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgResultChallenged represents a DkgResultChallenged event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgResultChallenged struct {
	ResultHash [32]byte
	Challenger common.Address
	Reason     string
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultChallenged is a free log retrieval operation binding the contract event 0x703feb01415a2995816e8d082fd7aad0eacada1a2f63fdb3226e47f8a0285436.
//
// Solidity: event DkgResultChallenged(bytes32 indexed resultHash, address indexed challenger, string reason)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgResultChallenged(opts *bind.FilterOpts, resultHash [][32]byte, challenger []common.Address) (*FrostWalletRegistryDkgResultChallengedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var challengerRule []interface{}
	for _, challengerItem := range challenger {
		challengerRule = append(challengerRule, challengerItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgResultChallenged", resultHashRule, challengerRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgResultChallengedIterator{contract: _FrostWalletRegistry.contract, event: "DkgResultChallenged", logs: logs, sub: sub}, nil
}

// WatchDkgResultChallenged is a free log subscription operation binding the contract event 0x703feb01415a2995816e8d082fd7aad0eacada1a2f63fdb3226e47f8a0285436.
//
// Solidity: event DkgResultChallenged(bytes32 indexed resultHash, address indexed challenger, string reason)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgResultChallenged(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgResultChallenged, resultHash [][32]byte, challenger []common.Address) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var challengerRule []interface{}
	for _, challengerItem := range challenger {
		challengerRule = append(challengerRule, challengerItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgResultChallenged", resultHashRule, challengerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgResultChallenged)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgResultChallenged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgResultChallenged is a log parse operation binding the contract event 0x703feb01415a2995816e8d082fd7aad0eacada1a2f63fdb3226e47f8a0285436.
//
// Solidity: event DkgResultChallenged(bytes32 indexed resultHash, address indexed challenger, string reason)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgResultChallenged(log types.Log) (*FrostWalletRegistryDkgResultChallenged, error) {
	event := new(FrostWalletRegistryDkgResultChallenged)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgResultChallenged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgResultSubmittedIterator is returned from FilterDkgResultSubmitted and is used to iterate over the raw logs and unpacked data for DkgResultSubmitted events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgResultSubmittedIterator struct {
	Event *FrostWalletRegistryDkgResultSubmitted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgResultSubmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgResultSubmitted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgResultSubmitted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgResultSubmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgResultSubmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgResultSubmitted represents a DkgResultSubmitted event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgResultSubmitted struct {
	ResultHash [32]byte
	Seed       *big.Int
	Result     FrostDkgResult
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultSubmitted is a free log retrieval operation binding the contract event 0xbfc6cd6291b6741d3ac1631ba81a0288d08265bea4d59d452e8c953e11ec11c6.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgResultSubmitted(opts *bind.FilterOpts, resultHash [][32]byte, seed []*big.Int) (*FrostWalletRegistryDkgResultSubmittedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgResultSubmitted", resultHashRule, seedRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgResultSubmittedIterator{contract: _FrostWalletRegistry.contract, event: "DkgResultSubmitted", logs: logs, sub: sub}, nil
}

// WatchDkgResultSubmitted is a free log subscription operation binding the contract event 0xbfc6cd6291b6741d3ac1631ba81a0288d08265bea4d59d452e8c953e11ec11c6.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgResultSubmitted(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgResultSubmitted, resultHash [][32]byte, seed []*big.Int) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgResultSubmitted", resultHashRule, seedRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgResultSubmitted)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgResultSubmitted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgResultSubmitted is a log parse operation binding the contract event 0xbfc6cd6291b6741d3ac1631ba81a0288d08265bea4d59d452e8c953e11ec11c6.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgResultSubmitted(log types.Log) (*FrostWalletRegistryDkgResultSubmitted, error) {
	event := new(FrostWalletRegistryDkgResultSubmitted)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgResultSubmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgSeedTimedOutIterator is returned from FilterDkgSeedTimedOut and is used to iterate over the raw logs and unpacked data for DkgSeedTimedOut events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgSeedTimedOutIterator struct {
	Event *FrostWalletRegistryDkgSeedTimedOut // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgSeedTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgSeedTimedOut)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgSeedTimedOut)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgSeedTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgSeedTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgSeedTimedOut represents a DkgSeedTimedOut event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgSeedTimedOut struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgSeedTimedOut is a free log retrieval operation binding the contract event 0x68c52f05452e81639fa06f379aee3178cddee4725521fff886f244c99e868b50.
//
// Solidity: event DkgSeedTimedOut()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgSeedTimedOut(opts *bind.FilterOpts) (*FrostWalletRegistryDkgSeedTimedOutIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgSeedTimedOut")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgSeedTimedOutIterator{contract: _FrostWalletRegistry.contract, event: "DkgSeedTimedOut", logs: logs, sub: sub}, nil
}

// WatchDkgSeedTimedOut is a free log subscription operation binding the contract event 0x68c52f05452e81639fa06f379aee3178cddee4725521fff886f244c99e868b50.
//
// Solidity: event DkgSeedTimedOut()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgSeedTimedOut(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgSeedTimedOut) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgSeedTimedOut")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgSeedTimedOut)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgSeedTimedOut", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgSeedTimedOut is a log parse operation binding the contract event 0x68c52f05452e81639fa06f379aee3178cddee4725521fff886f244c99e868b50.
//
// Solidity: event DkgSeedTimedOut()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgSeedTimedOut(log types.Log) (*FrostWalletRegistryDkgSeedTimedOut, error) {
	event := new(FrostWalletRegistryDkgSeedTimedOut)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgSeedTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgStartedIterator is returned from FilterDkgStarted and is used to iterate over the raw logs and unpacked data for DkgStarted events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgStartedIterator struct {
	Event *FrostWalletRegistryDkgStarted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgStartedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgStarted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgStarted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgStartedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgStartedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgStarted represents a DkgStarted event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgStarted struct {
	Seed *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterDkgStarted is a free log retrieval operation binding the contract event 0xb2ad26c2940889d79df2ee9c758a8aefa00c5ca90eee119af0e5d795df3b98bb.
//
// Solidity: event DkgStarted(uint256 indexed seed)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgStarted(opts *bind.FilterOpts, seed []*big.Int) (*FrostWalletRegistryDkgStartedIterator, error) {

	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgStarted", seedRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgStartedIterator{contract: _FrostWalletRegistry.contract, event: "DkgStarted", logs: logs, sub: sub}, nil
}

// WatchDkgStarted is a free log subscription operation binding the contract event 0xb2ad26c2940889d79df2ee9c758a8aefa00c5ca90eee119af0e5d795df3b98bb.
//
// Solidity: event DkgStarted(uint256 indexed seed)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgStarted(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgStarted, seed []*big.Int) (event.Subscription, error) {

	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgStarted", seedRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgStarted)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgStarted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgStarted is a log parse operation binding the contract event 0xb2ad26c2940889d79df2ee9c758a8aefa00c5ca90eee119af0e5d795df3b98bb.
//
// Solidity: event DkgStarted(uint256 indexed seed)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgStarted(log types.Log) (*FrostWalletRegistryDkgStarted, error) {
	event := new(FrostWalletRegistryDkgStarted)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgStarted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgStateLockedIterator is returned from FilterDkgStateLocked and is used to iterate over the raw logs and unpacked data for DkgStateLocked events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgStateLockedIterator struct {
	Event *FrostWalletRegistryDkgStateLocked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgStateLockedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgStateLocked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgStateLocked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgStateLockedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgStateLockedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgStateLocked represents a DkgStateLocked event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgStateLocked struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgStateLocked is a free log retrieval operation binding the contract event 0x5c3ed2397d4d21298b2fb5027ac8e2d42e3c9c72bbb55ddb030e2a36a0cdff6b.
//
// Solidity: event DkgStateLocked()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgStateLocked(opts *bind.FilterOpts) (*FrostWalletRegistryDkgStateLockedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgStateLocked")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgStateLockedIterator{contract: _FrostWalletRegistry.contract, event: "DkgStateLocked", logs: logs, sub: sub}, nil
}

// WatchDkgStateLocked is a free log subscription operation binding the contract event 0x5c3ed2397d4d21298b2fb5027ac8e2d42e3c9c72bbb55ddb030e2a36a0cdff6b.
//
// Solidity: event DkgStateLocked()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgStateLocked(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgStateLocked) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgStateLocked")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgStateLocked)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgStateLocked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgStateLocked is a log parse operation binding the contract event 0x5c3ed2397d4d21298b2fb5027ac8e2d42e3c9c72bbb55ddb030e2a36a0cdff6b.
//
// Solidity: event DkgStateLocked()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgStateLocked(log types.Log) (*FrostWalletRegistryDkgStateLocked, error) {
	event := new(FrostWalletRegistryDkgStateLocked)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgStateLocked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryDkgTimedOutIterator is returned from FilterDkgTimedOut and is used to iterate over the raw logs and unpacked data for DkgTimedOut events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgTimedOutIterator struct {
	Event *FrostWalletRegistryDkgTimedOut // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryDkgTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryDkgTimedOut)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryDkgTimedOut)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryDkgTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryDkgTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryDkgTimedOut represents a DkgTimedOut event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryDkgTimedOut struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgTimedOut is a free log retrieval operation binding the contract event 0x2852b3e178dd281713b041c3d90b4815bb55b7ec812931d1e8e8d8bb2ed72d3e.
//
// Solidity: event DkgTimedOut()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterDkgTimedOut(opts *bind.FilterOpts) (*FrostWalletRegistryDkgTimedOutIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "DkgTimedOut")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryDkgTimedOutIterator{contract: _FrostWalletRegistry.contract, event: "DkgTimedOut", logs: logs, sub: sub}, nil
}

// WatchDkgTimedOut is a free log subscription operation binding the contract event 0x2852b3e178dd281713b041c3d90b4815bb55b7ec812931d1e8e8d8bb2ed72d3e.
//
// Solidity: event DkgTimedOut()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchDkgTimedOut(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryDkgTimedOut) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "DkgTimedOut")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryDkgTimedOut)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgTimedOut", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDkgTimedOut is a log parse operation binding the contract event 0x2852b3e178dd281713b041c3d90b4815bb55b7ec812931d1e8e8d8bb2ed72d3e.
//
// Solidity: event DkgTimedOut()
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseDkgTimedOut(log types.Log) (*FrostWalletRegistryDkgTimedOut, error) {
	event := new(FrostWalletRegistryDkgTimedOut)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "DkgTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryGasParametersUpdatedIterator is returned from FilterGasParametersUpdated and is used to iterate over the raw logs and unpacked data for GasParametersUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryGasParametersUpdatedIterator struct {
	Event *FrostWalletRegistryGasParametersUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryGasParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryGasParametersUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryGasParametersUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryGasParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryGasParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryGasParametersUpdated represents a GasParametersUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryGasParametersUpdated struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
	Raw                               types.Log // Blockchain specific contextual infos
}

// FilterGasParametersUpdated is a free log retrieval operation binding the contract event 0x8a3e64fa6013a36bccca7362e8826b11ba41e57fb60f55309c0ca48904dad082.
//
// Solidity: event GasParametersUpdated(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterGasParametersUpdated(opts *bind.FilterOpts) (*FrostWalletRegistryGasParametersUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "GasParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryGasParametersUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "GasParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchGasParametersUpdated is a free log subscription operation binding the contract event 0x8a3e64fa6013a36bccca7362e8826b11ba41e57fb60f55309c0ca48904dad082.
//
// Solidity: event GasParametersUpdated(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchGasParametersUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryGasParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "GasParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryGasParametersUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "GasParametersUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseGasParametersUpdated is a log parse operation binding the contract event 0x8a3e64fa6013a36bccca7362e8826b11ba41e57fb60f55309c0ca48904dad082.
//
// Solidity: event GasParametersUpdated(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseGasParametersUpdated(log types.Log) (*FrostWalletRegistryGasParametersUpdated, error) {
	event := new(FrostWalletRegistryGasParametersUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "GasParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryGovernanceTransferredIterator is returned from FilterGovernanceTransferred and is used to iterate over the raw logs and unpacked data for GovernanceTransferred events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryGovernanceTransferredIterator struct {
	Event *FrostWalletRegistryGovernanceTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryGovernanceTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryGovernanceTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryGovernanceTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryGovernanceTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryGovernanceTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryGovernanceTransferred represents a GovernanceTransferred event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryGovernanceTransferred struct {
	OldGovernance common.Address
	NewGovernance common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterGovernanceTransferred is a free log retrieval operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterGovernanceTransferred(opts *bind.FilterOpts) (*FrostWalletRegistryGovernanceTransferredIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryGovernanceTransferredIterator{contract: _FrostWalletRegistry.contract, event: "GovernanceTransferred", logs: logs, sub: sub}, nil
}

// WatchGovernanceTransferred is a free log subscription operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchGovernanceTransferred(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryGovernanceTransferred) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryGovernanceTransferred)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseGovernanceTransferred is a log parse operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseGovernanceTransferred(log types.Log) (*FrostWalletRegistryGovernanceTransferred, error) {
	event := new(FrostWalletRegistryGovernanceTransferred)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryInactivityClaimedIterator is returned from FilterInactivityClaimed and is used to iterate over the raw logs and unpacked data for InactivityClaimed events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryInactivityClaimedIterator struct {
	Event *FrostWalletRegistryInactivityClaimed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryInactivityClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryInactivityClaimed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryInactivityClaimed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryInactivityClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryInactivityClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryInactivityClaimed represents a InactivityClaimed event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryInactivityClaimed struct {
	WalletID [32]byte
	Nonce    *big.Int
	Notifier common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterInactivityClaimed is a free log retrieval operation binding the contract event 0x326e1ff7c130ed708307116f79cf7dbca649503e7082e5e35a19ceeee1523b39.
//
// Solidity: event InactivityClaimed(bytes32 indexed walletID, uint256 nonce, address notifier)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterInactivityClaimed(opts *bind.FilterOpts, walletID [][32]byte) (*FrostWalletRegistryInactivityClaimedIterator, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "InactivityClaimed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryInactivityClaimedIterator{contract: _FrostWalletRegistry.contract, event: "InactivityClaimed", logs: logs, sub: sub}, nil
}

// WatchInactivityClaimed is a free log subscription operation binding the contract event 0x326e1ff7c130ed708307116f79cf7dbca649503e7082e5e35a19ceeee1523b39.
//
// Solidity: event InactivityClaimed(bytes32 indexed walletID, uint256 nonce, address notifier)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchInactivityClaimed(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryInactivityClaimed, walletID [][32]byte) (event.Subscription, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "InactivityClaimed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryInactivityClaimed)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "InactivityClaimed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInactivityClaimed is a log parse operation binding the contract event 0x326e1ff7c130ed708307116f79cf7dbca649503e7082e5e35a19ceeee1523b39.
//
// Solidity: event InactivityClaimed(bytes32 indexed walletID, uint256 nonce, address notifier)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseInactivityClaimed(log types.Log) (*FrostWalletRegistryInactivityClaimed, error) {
	event := new(FrostWalletRegistryInactivityClaimed)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "InactivityClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryInitializedIterator struct {
	Event *FrostWalletRegistryInitialized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryInitialized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryInitialized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryInitialized represents a Initialized event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterInitialized(opts *bind.FilterOpts) (*FrostWalletRegistryInitializedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryInitializedIterator{contract: _FrostWalletRegistry.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryInitialized) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryInitialized)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "Initialized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInitialized is a log parse operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseInitialized(log types.Log) (*FrostWalletRegistryInitialized, error) {
	event := new(FrostWalletRegistryInitialized)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator is returned from FilterInvoluntaryAuthorizationDecreaseFailed and is used to iterate over the raw logs and unpacked data for InvoluntaryAuthorizationDecreaseFailed events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator struct {
	Event *FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed represents a InvoluntaryAuthorizationDecreaseFailed event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed struct {
	StakingProvider common.Address
	Operator        common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterInvoluntaryAuthorizationDecreaseFailed is a free log retrieval operation binding the contract event 0x1b09380d63e78fd72c1d79a805a7e2dfadf02b22418e24bebff51376b7df33b0.
//
// Solidity: event InvoluntaryAuthorizationDecreaseFailed(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterInvoluntaryAuthorizationDecreaseFailed(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "InvoluntaryAuthorizationDecreaseFailed", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator{contract: _FrostWalletRegistry.contract, event: "InvoluntaryAuthorizationDecreaseFailed", logs: logs, sub: sub}, nil
}

// WatchInvoluntaryAuthorizationDecreaseFailed is a free log subscription operation binding the contract event 0x1b09380d63e78fd72c1d79a805a7e2dfadf02b22418e24bebff51376b7df33b0.
//
// Solidity: event InvoluntaryAuthorizationDecreaseFailed(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchInvoluntaryAuthorizationDecreaseFailed(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "InvoluntaryAuthorizationDecreaseFailed", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "InvoluntaryAuthorizationDecreaseFailed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInvoluntaryAuthorizationDecreaseFailed is a log parse operation binding the contract event 0x1b09380d63e78fd72c1d79a805a7e2dfadf02b22418e24bebff51376b7df33b0.
//
// Solidity: event InvoluntaryAuthorizationDecreaseFailed(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseInvoluntaryAuthorizationDecreaseFailed(log types.Log) (*FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed, error) {
	event := new(FrostWalletRegistryInvoluntaryAuthorizationDecreaseFailed)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "InvoluntaryAuthorizationDecreaseFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryLifecycleOwnerUpdatedIterator is returned from FilterLifecycleOwnerUpdated and is used to iterate over the raw logs and unpacked data for LifecycleOwnerUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryLifecycleOwnerUpdatedIterator struct {
	Event *FrostWalletRegistryLifecycleOwnerUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryLifecycleOwnerUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryLifecycleOwnerUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryLifecycleOwnerUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryLifecycleOwnerUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryLifecycleOwnerUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryLifecycleOwnerUpdated represents a LifecycleOwnerUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryLifecycleOwnerUpdated struct {
	LifecycleOwner common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterLifecycleOwnerUpdated is a free log retrieval operation binding the contract event 0xc41594e25066d174fb0130f0ddd858b71b9a4f035b2f07d903a4385337c93382.
//
// Solidity: event LifecycleOwnerUpdated(address lifecycleOwner)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterLifecycleOwnerUpdated(opts *bind.FilterOpts) (*FrostWalletRegistryLifecycleOwnerUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "LifecycleOwnerUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryLifecycleOwnerUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "LifecycleOwnerUpdated", logs: logs, sub: sub}, nil
}

// WatchLifecycleOwnerUpdated is a free log subscription operation binding the contract event 0xc41594e25066d174fb0130f0ddd858b71b9a4f035b2f07d903a4385337c93382.
//
// Solidity: event LifecycleOwnerUpdated(address lifecycleOwner)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchLifecycleOwnerUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryLifecycleOwnerUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "LifecycleOwnerUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryLifecycleOwnerUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "LifecycleOwnerUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLifecycleOwnerUpdated is a log parse operation binding the contract event 0xc41594e25066d174fb0130f0ddd858b71b9a4f035b2f07d903a4385337c93382.
//
// Solidity: event LifecycleOwnerUpdated(address lifecycleOwner)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseLifecycleOwnerUpdated(log types.Log) (*FrostWalletRegistryLifecycleOwnerUpdated, error) {
	event := new(FrostWalletRegistryLifecycleOwnerUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "LifecycleOwnerUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryOperatorJoinedSortitionPoolIterator is returned from FilterOperatorJoinedSortitionPool and is used to iterate over the raw logs and unpacked data for OperatorJoinedSortitionPool events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryOperatorJoinedSortitionPoolIterator struct {
	Event *FrostWalletRegistryOperatorJoinedSortitionPool // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryOperatorJoinedSortitionPoolIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryOperatorJoinedSortitionPool)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryOperatorJoinedSortitionPool)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryOperatorJoinedSortitionPoolIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryOperatorJoinedSortitionPoolIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryOperatorJoinedSortitionPool represents a OperatorJoinedSortitionPool event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryOperatorJoinedSortitionPool struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorJoinedSortitionPool is a free log retrieval operation binding the contract event 0x5075aaa89894a888eb2cac81a27320c60855febb0cf1706b66bdc754e640d433.
//
// Solidity: event OperatorJoinedSortitionPool(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterOperatorJoinedSortitionPool(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*FrostWalletRegistryOperatorJoinedSortitionPoolIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "OperatorJoinedSortitionPool", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryOperatorJoinedSortitionPoolIterator{contract: _FrostWalletRegistry.contract, event: "OperatorJoinedSortitionPool", logs: logs, sub: sub}, nil
}

// WatchOperatorJoinedSortitionPool is a free log subscription operation binding the contract event 0x5075aaa89894a888eb2cac81a27320c60855febb0cf1706b66bdc754e640d433.
//
// Solidity: event OperatorJoinedSortitionPool(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchOperatorJoinedSortitionPool(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryOperatorJoinedSortitionPool, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "OperatorJoinedSortitionPool", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryOperatorJoinedSortitionPool)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "OperatorJoinedSortitionPool", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOperatorJoinedSortitionPool is a log parse operation binding the contract event 0x5075aaa89894a888eb2cac81a27320c60855febb0cf1706b66bdc754e640d433.
//
// Solidity: event OperatorJoinedSortitionPool(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseOperatorJoinedSortitionPool(log types.Log) (*FrostWalletRegistryOperatorJoinedSortitionPool, error) {
	event := new(FrostWalletRegistryOperatorJoinedSortitionPool)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "OperatorJoinedSortitionPool", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryOperatorRegisteredIterator is returned from FilterOperatorRegistered and is used to iterate over the raw logs and unpacked data for OperatorRegistered events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryOperatorRegisteredIterator struct {
	Event *FrostWalletRegistryOperatorRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryOperatorRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryOperatorRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryOperatorRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryOperatorRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryOperatorRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryOperatorRegistered represents a OperatorRegistered event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryOperatorRegistered struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorRegistered is a free log retrieval operation binding the contract event 0xa453db612af59e5521d6ab9284dc3e2d06af286eb1b1b7b771fce4716c19f2c1.
//
// Solidity: event OperatorRegistered(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterOperatorRegistered(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*FrostWalletRegistryOperatorRegisteredIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "OperatorRegistered", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryOperatorRegisteredIterator{contract: _FrostWalletRegistry.contract, event: "OperatorRegistered", logs: logs, sub: sub}, nil
}

// WatchOperatorRegistered is a free log subscription operation binding the contract event 0xa453db612af59e5521d6ab9284dc3e2d06af286eb1b1b7b771fce4716c19f2c1.
//
// Solidity: event OperatorRegistered(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchOperatorRegistered(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryOperatorRegistered, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "OperatorRegistered", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryOperatorRegistered)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "OperatorRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOperatorRegistered is a log parse operation binding the contract event 0xa453db612af59e5521d6ab9284dc3e2d06af286eb1b1b7b771fce4716c19f2c1.
//
// Solidity: event OperatorRegistered(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseOperatorRegistered(log types.Log) (*FrostWalletRegistryOperatorRegistered, error) {
	event := new(FrostWalletRegistryOperatorRegistered)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "OperatorRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryOperatorStatusUpdatedIterator is returned from FilterOperatorStatusUpdated and is used to iterate over the raw logs and unpacked data for OperatorStatusUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryOperatorStatusUpdatedIterator struct {
	Event *FrostWalletRegistryOperatorStatusUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryOperatorStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryOperatorStatusUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryOperatorStatusUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryOperatorStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryOperatorStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryOperatorStatusUpdated represents a OperatorStatusUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryOperatorStatusUpdated struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorStatusUpdated is a free log retrieval operation binding the contract event 0x1231fe5ee649a593b524a494cd53146a196380a872115a0d0fe16c0735afdf26.
//
// Solidity: event OperatorStatusUpdated(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterOperatorStatusUpdated(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*FrostWalletRegistryOperatorStatusUpdatedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "OperatorStatusUpdated", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryOperatorStatusUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "OperatorStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchOperatorStatusUpdated is a free log subscription operation binding the contract event 0x1231fe5ee649a593b524a494cd53146a196380a872115a0d0fe16c0735afdf26.
//
// Solidity: event OperatorStatusUpdated(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchOperatorStatusUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryOperatorStatusUpdated, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "OperatorStatusUpdated", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryOperatorStatusUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "OperatorStatusUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOperatorStatusUpdated is a log parse operation binding the contract event 0x1231fe5ee649a593b524a494cd53146a196380a872115a0d0fe16c0735afdf26.
//
// Solidity: event OperatorStatusUpdated(address indexed stakingProvider, address indexed operator)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseOperatorStatusUpdated(log types.Log) (*FrostWalletRegistryOperatorStatusUpdated, error) {
	event := new(FrostWalletRegistryOperatorStatusUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "OperatorStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryRandomBeaconUpgradedIterator is returned from FilterRandomBeaconUpgraded and is used to iterate over the raw logs and unpacked data for RandomBeaconUpgraded events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryRandomBeaconUpgradedIterator struct {
	Event *FrostWalletRegistryRandomBeaconUpgraded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryRandomBeaconUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryRandomBeaconUpgraded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryRandomBeaconUpgraded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryRandomBeaconUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryRandomBeaconUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryRandomBeaconUpgraded represents a RandomBeaconUpgraded event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryRandomBeaconUpgraded struct {
	RandomBeacon common.Address
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRandomBeaconUpgraded is a free log retrieval operation binding the contract event 0x2b34e21b6daa8fcf8cba1c3ed709cbed2b0231d5fb60e9ccd8c2e75a5674bcb3.
//
// Solidity: event RandomBeaconUpgraded(address randomBeacon)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterRandomBeaconUpgraded(opts *bind.FilterOpts) (*FrostWalletRegistryRandomBeaconUpgradedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "RandomBeaconUpgraded")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryRandomBeaconUpgradedIterator{contract: _FrostWalletRegistry.contract, event: "RandomBeaconUpgraded", logs: logs, sub: sub}, nil
}

// WatchRandomBeaconUpgraded is a free log subscription operation binding the contract event 0x2b34e21b6daa8fcf8cba1c3ed709cbed2b0231d5fb60e9ccd8c2e75a5674bcb3.
//
// Solidity: event RandomBeaconUpgraded(address randomBeacon)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchRandomBeaconUpgraded(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryRandomBeaconUpgraded) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "RandomBeaconUpgraded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryRandomBeaconUpgraded)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "RandomBeaconUpgraded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRandomBeaconUpgraded is a log parse operation binding the contract event 0x2b34e21b6daa8fcf8cba1c3ed709cbed2b0231d5fb60e9ccd8c2e75a5674bcb3.
//
// Solidity: event RandomBeaconUpgraded(address randomBeacon)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseRandomBeaconUpgraded(log types.Log) (*FrostWalletRegistryRandomBeaconUpgraded, error) {
	event := new(FrostWalletRegistryRandomBeaconUpgraded)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "RandomBeaconUpgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryReimbursementPoolUpdatedIterator is returned from FilterReimbursementPoolUpdated and is used to iterate over the raw logs and unpacked data for ReimbursementPoolUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryReimbursementPoolUpdatedIterator struct {
	Event *FrostWalletRegistryReimbursementPoolUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryReimbursementPoolUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryReimbursementPoolUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryReimbursementPoolUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryReimbursementPoolUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryReimbursementPoolUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryReimbursementPoolUpdated represents a ReimbursementPoolUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryReimbursementPoolUpdated struct {
	NewReimbursementPool common.Address
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterReimbursementPoolUpdated is a free log retrieval operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterReimbursementPoolUpdated(opts *bind.FilterOpts) (*FrostWalletRegistryReimbursementPoolUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryReimbursementPoolUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "ReimbursementPoolUpdated", logs: logs, sub: sub}, nil
}

// WatchReimbursementPoolUpdated is a free log subscription operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchReimbursementPoolUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryReimbursementPoolUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryReimbursementPoolUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseReimbursementPoolUpdated is a log parse operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseReimbursementPoolUpdated(log types.Log) (*FrostWalletRegistryReimbursementPoolUpdated, error) {
	event := new(FrostWalletRegistryReimbursementPoolUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryRewardParametersUpdatedIterator is returned from FilterRewardParametersUpdated and is used to iterate over the raw logs and unpacked data for RewardParametersUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryRewardParametersUpdatedIterator struct {
	Event *FrostWalletRegistryRewardParametersUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryRewardParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryRewardParametersUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryRewardParametersUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryRewardParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryRewardParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryRewardParametersUpdated represents a RewardParametersUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryRewardParametersUpdated struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
	Raw                                            types.Log // Blockchain specific contextual infos
}

// FilterRewardParametersUpdated is a free log retrieval operation binding the contract event 0xf3a6ee10a78fb7d212e87d9be970fb16bd7324e9dc9c38d21cd7ecde781a1d2a.
//
// Solidity: event RewardParametersUpdated(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterRewardParametersUpdated(opts *bind.FilterOpts) (*FrostWalletRegistryRewardParametersUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "RewardParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryRewardParametersUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "RewardParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchRewardParametersUpdated is a free log subscription operation binding the contract event 0xf3a6ee10a78fb7d212e87d9be970fb16bd7324e9dc9c38d21cd7ecde781a1d2a.
//
// Solidity: event RewardParametersUpdated(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchRewardParametersUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryRewardParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "RewardParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryRewardParametersUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "RewardParametersUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRewardParametersUpdated is a log parse operation binding the contract event 0xf3a6ee10a78fb7d212e87d9be970fb16bd7324e9dc9c38d21cd7ecde781a1d2a.
//
// Solidity: event RewardParametersUpdated(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseRewardParametersUpdated(log types.Log) (*FrostWalletRegistryRewardParametersUpdated, error) {
	event := new(FrostWalletRegistryRewardParametersUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "RewardParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryRewardsWithdrawnIterator is returned from FilterRewardsWithdrawn and is used to iterate over the raw logs and unpacked data for RewardsWithdrawn events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryRewardsWithdrawnIterator struct {
	Event *FrostWalletRegistryRewardsWithdrawn // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryRewardsWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryRewardsWithdrawn)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryRewardsWithdrawn)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryRewardsWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryRewardsWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryRewardsWithdrawn represents a RewardsWithdrawn event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryRewardsWithdrawn struct {
	StakingProvider common.Address
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterRewardsWithdrawn is a free log retrieval operation binding the contract event 0x38532b6dea69d7266fa923c7813d190be37625f2454ddfa3d93c45c79482e3fd.
//
// Solidity: event RewardsWithdrawn(address indexed stakingProvider, uint96 amount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterRewardsWithdrawn(opts *bind.FilterOpts, stakingProvider []common.Address) (*FrostWalletRegistryRewardsWithdrawnIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "RewardsWithdrawn", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryRewardsWithdrawnIterator{contract: _FrostWalletRegistry.contract, event: "RewardsWithdrawn", logs: logs, sub: sub}, nil
}

// WatchRewardsWithdrawn is a free log subscription operation binding the contract event 0x38532b6dea69d7266fa923c7813d190be37625f2454ddfa3d93c45c79482e3fd.
//
// Solidity: event RewardsWithdrawn(address indexed stakingProvider, uint96 amount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchRewardsWithdrawn(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryRewardsWithdrawn, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "RewardsWithdrawn", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryRewardsWithdrawn)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "RewardsWithdrawn", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRewardsWithdrawn is a log parse operation binding the contract event 0x38532b6dea69d7266fa923c7813d190be37625f2454ddfa3d93c45c79482e3fd.
//
// Solidity: event RewardsWithdrawn(address indexed stakingProvider, uint96 amount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseRewardsWithdrawn(log types.Log) (*FrostWalletRegistryRewardsWithdrawn, error) {
	event := new(FrostWalletRegistryRewardsWithdrawn)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "RewardsWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistrySlashingParametersUpdatedIterator is returned from FilterSlashingParametersUpdated and is used to iterate over the raw logs and unpacked data for SlashingParametersUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistrySlashingParametersUpdatedIterator struct {
	Event *FrostWalletRegistrySlashingParametersUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistrySlashingParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistrySlashingParametersUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistrySlashingParametersUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistrySlashingParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistrySlashingParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistrySlashingParametersUpdated represents a SlashingParametersUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistrySlashingParametersUpdated struct {
	MaliciousDkgResultSlashingAmount *big.Int
	Raw                              types.Log // Blockchain specific contextual infos
}

// FilterSlashingParametersUpdated is a free log retrieval operation binding the contract event 0xe132b87eb6644ee4d4c3c32744f7e1c3906335a2d4f99330767bf573909c7d84.
//
// Solidity: event SlashingParametersUpdated(uint256 maliciousDkgResultSlashingAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterSlashingParametersUpdated(opts *bind.FilterOpts) (*FrostWalletRegistrySlashingParametersUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "SlashingParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistrySlashingParametersUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "SlashingParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchSlashingParametersUpdated is a free log subscription operation binding the contract event 0xe132b87eb6644ee4d4c3c32744f7e1c3906335a2d4f99330767bf573909c7d84.
//
// Solidity: event SlashingParametersUpdated(uint256 maliciousDkgResultSlashingAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchSlashingParametersUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistrySlashingParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "SlashingParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistrySlashingParametersUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "SlashingParametersUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSlashingParametersUpdated is a log parse operation binding the contract event 0xe132b87eb6644ee4d4c3c32744f7e1c3906335a2d4f99330767bf573909c7d84.
//
// Solidity: event SlashingParametersUpdated(uint256 maliciousDkgResultSlashingAmount)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseSlashingParametersUpdated(log types.Log) (*FrostWalletRegistrySlashingParametersUpdated, error) {
	event := new(FrostWalletRegistrySlashingParametersUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "SlashingParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryWalletClosedIterator is returned from FilterWalletClosed and is used to iterate over the raw logs and unpacked data for WalletClosed events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryWalletClosedIterator struct {
	Event *FrostWalletRegistryWalletClosed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryWalletClosedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryWalletClosed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryWalletClosed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryWalletClosedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryWalletClosedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryWalletClosed represents a WalletClosed event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryWalletClosed struct {
	WalletID [32]byte
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterWalletClosed is a free log retrieval operation binding the contract event 0xa6ae4af610b8ada39d3675190ead27a5552631a8e33f53e4e37dbb082f11a73e.
//
// Solidity: event WalletClosed(bytes32 indexed walletID)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterWalletClosed(opts *bind.FilterOpts, walletID [][32]byte) (*FrostWalletRegistryWalletClosedIterator, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "WalletClosed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryWalletClosedIterator{contract: _FrostWalletRegistry.contract, event: "WalletClosed", logs: logs, sub: sub}, nil
}

// WatchWalletClosed is a free log subscription operation binding the contract event 0xa6ae4af610b8ada39d3675190ead27a5552631a8e33f53e4e37dbb082f11a73e.
//
// Solidity: event WalletClosed(bytes32 indexed walletID)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchWalletClosed(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryWalletClosed, walletID [][32]byte) (event.Subscription, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "WalletClosed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryWalletClosed)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "WalletClosed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseWalletClosed is a log parse operation binding the contract event 0xa6ae4af610b8ada39d3675190ead27a5552631a8e33f53e4e37dbb082f11a73e.
//
// Solidity: event WalletClosed(bytes32 indexed walletID)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseWalletClosed(log types.Log) (*FrostWalletRegistryWalletClosed, error) {
	event := new(FrostWalletRegistryWalletClosed)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "WalletClosed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryWalletCreatedIterator is returned from FilterWalletCreated and is used to iterate over the raw logs and unpacked data for WalletCreated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryWalletCreatedIterator struct {
	Event *FrostWalletRegistryWalletCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryWalletCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryWalletCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryWalletCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryWalletCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryWalletCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryWalletCreated represents a WalletCreated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryWalletCreated struct {
	WalletID      [32]byte
	DkgResultHash [32]byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterWalletCreated is a free log retrieval operation binding the contract event 0xbe8f27cef1f3d94120c9c547c3614f5b992fdb0c0a497cc920fde06546291ab4.
//
// Solidity: event WalletCreated(bytes32 indexed walletID, bytes32 indexed dkgResultHash)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterWalletCreated(opts *bind.FilterOpts, walletID [][32]byte, dkgResultHash [][32]byte) (*FrostWalletRegistryWalletCreatedIterator, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}
	var dkgResultHashRule []interface{}
	for _, dkgResultHashItem := range dkgResultHash {
		dkgResultHashRule = append(dkgResultHashRule, dkgResultHashItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "WalletCreated", walletIDRule, dkgResultHashRule)
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryWalletCreatedIterator{contract: _FrostWalletRegistry.contract, event: "WalletCreated", logs: logs, sub: sub}, nil
}

// WatchWalletCreated is a free log subscription operation binding the contract event 0xbe8f27cef1f3d94120c9c547c3614f5b992fdb0c0a497cc920fde06546291ab4.
//
// Solidity: event WalletCreated(bytes32 indexed walletID, bytes32 indexed dkgResultHash)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchWalletCreated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryWalletCreated, walletID [][32]byte, dkgResultHash [][32]byte) (event.Subscription, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}
	var dkgResultHashRule []interface{}
	for _, dkgResultHashItem := range dkgResultHash {
		dkgResultHashRule = append(dkgResultHashRule, dkgResultHashItem)
	}

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "WalletCreated", walletIDRule, dkgResultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryWalletCreated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "WalletCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseWalletCreated is a log parse operation binding the contract event 0xbe8f27cef1f3d94120c9c547c3614f5b992fdb0c0a497cc920fde06546291ab4.
//
// Solidity: event WalletCreated(bytes32 indexed walletID, bytes32 indexed dkgResultHash)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseWalletCreated(log types.Log) (*FrostWalletRegistryWalletCreated, error) {
	event := new(FrostWalletRegistryWalletCreated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "WalletCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FrostWalletRegistryWalletOwnerUpdatedIterator is returned from FilterWalletOwnerUpdated and is used to iterate over the raw logs and unpacked data for WalletOwnerUpdated events raised by the FrostWalletRegistry contract.
type FrostWalletRegistryWalletOwnerUpdatedIterator struct {
	Event *FrostWalletRegistryWalletOwnerUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FrostWalletRegistryWalletOwnerUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FrostWalletRegistryWalletOwnerUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FrostWalletRegistryWalletOwnerUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FrostWalletRegistryWalletOwnerUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FrostWalletRegistryWalletOwnerUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FrostWalletRegistryWalletOwnerUpdated represents a WalletOwnerUpdated event raised by the FrostWalletRegistry contract.
type FrostWalletRegistryWalletOwnerUpdated struct {
	WalletOwner common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterWalletOwnerUpdated is a free log retrieval operation binding the contract event 0xa1993af5a189ba5ad4155263c920cfee33ce0593a8eb231a13bb3ce6f39459e3.
//
// Solidity: event WalletOwnerUpdated(address walletOwner)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) FilterWalletOwnerUpdated(opts *bind.FilterOpts) (*FrostWalletRegistryWalletOwnerUpdatedIterator, error) {

	logs, sub, err := _FrostWalletRegistry.contract.FilterLogs(opts, "WalletOwnerUpdated")
	if err != nil {
		return nil, err
	}
	return &FrostWalletRegistryWalletOwnerUpdatedIterator{contract: _FrostWalletRegistry.contract, event: "WalletOwnerUpdated", logs: logs, sub: sub}, nil
}

// WatchWalletOwnerUpdated is a free log subscription operation binding the contract event 0xa1993af5a189ba5ad4155263c920cfee33ce0593a8eb231a13bb3ce6f39459e3.
//
// Solidity: event WalletOwnerUpdated(address walletOwner)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) WatchWalletOwnerUpdated(opts *bind.WatchOpts, sink chan<- *FrostWalletRegistryWalletOwnerUpdated) (event.Subscription, error) {

	logs, sub, err := _FrostWalletRegistry.contract.WatchLogs(opts, "WalletOwnerUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FrostWalletRegistryWalletOwnerUpdated)
				if err := _FrostWalletRegistry.contract.UnpackLog(event, "WalletOwnerUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseWalletOwnerUpdated is a log parse operation binding the contract event 0xa1993af5a189ba5ad4155263c920cfee33ce0593a8eb231a13bb3ce6f39459e3.
//
// Solidity: event WalletOwnerUpdated(address walletOwner)
func (_FrostWalletRegistry *FrostWalletRegistryFilterer) ParseWalletOwnerUpdated(log types.Log) (*FrostWalletRegistryWalletOwnerUpdated, error) {
	event := new(FrostWalletRegistryWalletOwnerUpdated)
	if err := _FrostWalletRegistry.contract.UnpackLog(event, "WalletOwnerUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

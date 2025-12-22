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

// EcdsaDkgParameters is an auto generated low-level Go binding around an user-defined struct.
type EcdsaDkgParameters struct {
	SeedTimeout                     *big.Int
	ResultChallengePeriodLength     *big.Int
	ResultChallengeExtraGas         *big.Int
	ResultSubmissionTimeout         *big.Int
	SubmitterPrecedencePeriodLength *big.Int
}

// EcdsaDkgResult is an auto generated low-level Go binding around an user-defined struct.
type EcdsaDkgResult struct {
	SubmitterMemberIndex     *big.Int
	GroupPubKey              []byte
	MisbehavedMembersIndices []uint8
	Signatures               []byte
	SigningMembersIndices    []*big.Int
	Members                  []uint32
	MembersHash              [32]byte
}

// EcdsaInactivityClaim is an auto generated low-level Go binding around an user-defined struct.
type EcdsaInactivityClaim struct {
	WalletID               [32]byte
	InactiveMembersIndices []*big.Int
	HeartbeatFailed        bool
	Signatures             []byte
	SigningMembersIndices  []*big.Int
}

// WalletsWallet is an auto generated low-level Go binding around an user-defined struct.
type WalletsWallet struct {
	MembersIdsHash [32]byte
	PublicKeyX     [32]byte
	PublicKeyY     [32]byte
}

// WalletRegistryMetaData contains all meta data concerning the WalletRegistry contract.
var WalletRegistryMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"_sortitionPool\",\"type\":\"address\"},{\"internalType\":\"contractIStaking\",\"name\":\"_staking\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"AuthorizationDecreaseApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"decreasingAt\",\"type\":\"uint64\"}],\"name\":\"AuthorizationDecreaseRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"AuthorizationIncreased\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"minimumAuthorization\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"name\":\"AuthorizationParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"maliciousSubmitter\",\"type\":\"address\"}],\"name\":\"DkgMaliciousResultSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"maliciousSubmitter\",\"type\":\"address\"}],\"name\":\"DkgMaliciousResultSlashingFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"seedTimeout\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultChallengePeriodLength\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultChallengeExtraGas\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultSubmissionTimeout\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resultSubmitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"name\":\"DkgParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"approver\",\"type\":\"address\"}],\"name\":\"DkgResultApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"challenger\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"reason\",\"type\":\"string\"}],\"name\":\"DkgResultChallenged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"indexed\":false,\"internalType\":\"structEcdsaDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"}],\"name\":\"DkgResultSubmitted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgSeedTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"}],\"name\":\"DkgStarted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgStateLocked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifySeedTimeoutGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyDkgTimeoutNegativeGasOffset\",\"type\":\"uint256\"}],\"name\":\"GasParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldGovernance\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"GovernanceTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"notifier\",\"type\":\"address\"}],\"name\":\"InactivityClaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"InvoluntaryAuthorizationDecreaseFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorJoinedSortitionPool\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorStatusUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"randomBeacon\",\"type\":\"address\"}],\"name\":\"RandomBeaconUpgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newReimbursementPool\",\"type\":\"address\"}],\"name\":\"ReimbursementPoolUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"}],\"name\":\"RewardParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"RewardsWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint256\"}],\"name\":\"SlashingParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"WalletClosed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"dkgResultHash\",\"type\":\"bytes32\"}],\"name\":\"WalletCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"walletOwner\",\"type\":\"address\"}],\"name\":\"WalletOwnerUpdated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"relayEntry\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"__beaconCallback\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"approveAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structEcdsaDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"approveDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"authorizationDecreaseRequested\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"authorizationIncreased\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"authorizationParameters\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"minimumAuthorization\",\"type\":\"uint96\"},{\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"availableRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structEcdsaDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"challengeDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"closeWallet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"dkgParameters\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"seedTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"resultChallengePeriodLength\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"resultChallengeExtraGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"resultSubmissionTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"submitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"internalType\":\"structEcdsaDkg.Parameters\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"eligibleStake\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"gasParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifySeedTimeoutGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyDkgTimeoutNegativeGasOffset\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"getWallet\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"membersIdsHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"publicKeyX\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"publicKeyY\",\"type\":\"bytes32\"}],\"internalType\":\"structWallets.Wallet\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getWalletCreationState\",\"outputs\":[{\"internalType\":\"enumEcdsaDkg.State\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"getWalletPublicKey\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"governance\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"hasDkgTimedOut\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"hasSeedTimedOut\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"inactivityClaimNonce\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractEcdsaDkgValidator\",\"name\":\"_ecdsaDkgValidator\",\"type\":\"address\"},{\"internalType\":\"contractIRandomBeacon\",\"name\":\"_randomBeacon\",\"type\":\"address\"},{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"involuntaryAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structEcdsaDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"}],\"name\":\"isDkgResultValid\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"},{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorInPool\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorUpToDate\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"walletMemberIndex\",\"type\":\"uint256\"}],\"name\":\"isWalletMember\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"}],\"name\":\"isWalletRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"joinSortitionPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minimumAuthorization\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifyDkgTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"internalType\":\"uint256[]\",\"name\":\"inactiveMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"bool\",\"name\":\"heartbeatFailed\",\"type\":\"bool\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"}],\"internalType\":\"structEcdsaInactivity.Claim\",\"name\":\"claim\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"internalType\":\"uint32[]\",\"name\":\"groupMembers\",\"type\":\"uint32[]\"}],\"name\":\"notifyOperatorInactivity\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifySeedTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"operatorToStakingProvider\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"pendingAuthorizationDecrease\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"randomBeacon\",\"outputs\":[{\"internalType\":\"contractIRandomBeacon\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"registerOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reimbursementPool\",\"outputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"remainingAuthorizationDecreaseDelay\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"requestNewWallet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rewardParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"},{\"internalType\":\"uint256\",\"name\":\"rewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"notifier\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"walletID\",\"type\":\"bytes32\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"}],\"name\":\"seize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"selectGroup\",\"outputs\":[{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"slashingParameters\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"sortitionPool\",\"outputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"staking\",\"outputs\":[{\"internalType\":\"contractIStaking\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"stakingProviderToOperator\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structEcdsaDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"submitDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"transferGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"_minimumAuthorization\",\"type\":\"uint96\"},{\"internalType\":\"uint64\",\"name\":\"_authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"_authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"name\":\"updateAuthorizationParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_seedTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_resultChallengePeriodLength\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_resultChallengeExtraGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_resultSubmissionTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_submitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"name\":\"updateDkgParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifySeedTimeoutGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyDkgTimeoutNegativeGasOffset\",\"type\":\"uint256\"}],\"name\":\"updateGasParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"updateOperatorStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"name\":\"updateReimbursementPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"}],\"name\":\"updateRewardParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint96\"}],\"name\":\"updateSlashingParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIWalletOwner\",\"name\":\"_walletOwner\",\"type\":\"address\"}],\"name\":\"updateWalletOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIRandomBeacon\",\"name\":\"_randomBeacon\",\"type\":\"address\"}],\"name\":\"upgradeRandomBeacon\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"walletOwner\",\"outputs\":[{\"internalType\":\"contractIWalletOwner\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"withdrawIneligibleRewards\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"withdrawRewards\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// WalletRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use WalletRegistryMetaData.ABI instead.
var WalletRegistryABI = WalletRegistryMetaData.ABI

// WalletRegistry is an auto generated Go binding around an Ethereum contract.
type WalletRegistry struct {
	WalletRegistryCaller     // Read-only binding to the contract
	WalletRegistryTransactor // Write-only binding to the contract
	WalletRegistryFilterer   // Log filterer for contract events
}

// WalletRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type WalletRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type WalletRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type WalletRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type WalletRegistrySession struct {
	Contract     *WalletRegistry   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// WalletRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type WalletRegistryCallerSession struct {
	Contract *WalletRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// WalletRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type WalletRegistryTransactorSession struct {
	Contract     *WalletRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// WalletRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type WalletRegistryRaw struct {
	Contract *WalletRegistry // Generic contract binding to access the raw methods on
}

// WalletRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type WalletRegistryCallerRaw struct {
	Contract *WalletRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// WalletRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type WalletRegistryTransactorRaw struct {
	Contract *WalletRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewWalletRegistry creates a new instance of WalletRegistry, bound to a specific deployed contract.
func NewWalletRegistry(address common.Address, backend bind.ContractBackend) (*WalletRegistry, error) {
	contract, err := bindWalletRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &WalletRegistry{WalletRegistryCaller: WalletRegistryCaller{contract: contract}, WalletRegistryTransactor: WalletRegistryTransactor{contract: contract}, WalletRegistryFilterer: WalletRegistryFilterer{contract: contract}}, nil
}

// NewWalletRegistryCaller creates a new read-only instance of WalletRegistry, bound to a specific deployed contract.
func NewWalletRegistryCaller(address common.Address, caller bind.ContractCaller) (*WalletRegistryCaller, error) {
	contract, err := bindWalletRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryCaller{contract: contract}, nil
}

// NewWalletRegistryTransactor creates a new write-only instance of WalletRegistry, bound to a specific deployed contract.
func NewWalletRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*WalletRegistryTransactor, error) {
	contract, err := bindWalletRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryTransactor{contract: contract}, nil
}

// NewWalletRegistryFilterer creates a new log filterer instance of WalletRegistry, bound to a specific deployed contract.
func NewWalletRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*WalletRegistryFilterer, error) {
	contract, err := bindWalletRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryFilterer{contract: contract}, nil
}

// bindWalletRegistry binds a generic wrapper to an already deployed contract.
func bindWalletRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := WalletRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_WalletRegistry *WalletRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _WalletRegistry.Contract.WalletRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_WalletRegistry *WalletRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletRegistry.Contract.WalletRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_WalletRegistry *WalletRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _WalletRegistry.Contract.WalletRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_WalletRegistry *WalletRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _WalletRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_WalletRegistry *WalletRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_WalletRegistry *WalletRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _WalletRegistry.Contract.contract.Transact(opts, method, params...)
}

// AuthorizationParameters is a free data retrieval call binding the contract method 0x7b14729e.
//
// Solidity: function authorizationParameters() view returns(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_WalletRegistry *WalletRegistryCaller) AuthorizationParameters(opts *bind.CallOpts) (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "authorizationParameters")

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
func (_WalletRegistry *WalletRegistrySession) AuthorizationParameters() (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	return _WalletRegistry.Contract.AuthorizationParameters(&_WalletRegistry.CallOpts)
}

// AuthorizationParameters is a free data retrieval call binding the contract method 0x7b14729e.
//
// Solidity: function authorizationParameters() view returns(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_WalletRegistry *WalletRegistryCallerSession) AuthorizationParameters() (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	return _WalletRegistry.Contract.AuthorizationParameters(&_WalletRegistry.CallOpts)
}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistryCaller) AvailableRewards(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "availableRewards", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistrySession) AvailableRewards(stakingProvider common.Address) (*big.Int, error) {
	return _WalletRegistry.Contract.AvailableRewards(&_WalletRegistry.CallOpts, stakingProvider)
}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistryCallerSession) AvailableRewards(stakingProvider common.Address) (*big.Int, error) {
	return _WalletRegistry.Contract.AvailableRewards(&_WalletRegistry.CallOpts, stakingProvider)
}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_WalletRegistry *WalletRegistryCaller) DkgParameters(opts *bind.CallOpts) (EcdsaDkgParameters, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "dkgParameters")

	if err != nil {
		return *new(EcdsaDkgParameters), err
	}

	out0 := *abi.ConvertType(out[0], new(EcdsaDkgParameters)).(*EcdsaDkgParameters)

	return out0, err

}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_WalletRegistry *WalletRegistrySession) DkgParameters() (EcdsaDkgParameters, error) {
	return _WalletRegistry.Contract.DkgParameters(&_WalletRegistry.CallOpts)
}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_WalletRegistry *WalletRegistryCallerSession) DkgParameters() (EcdsaDkgParameters, error) {
	return _WalletRegistry.Contract.DkgParameters(&_WalletRegistry.CallOpts)
}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistryCaller) EligibleStake(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "eligibleStake", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistrySession) EligibleStake(stakingProvider common.Address) (*big.Int, error) {
	return _WalletRegistry.Contract.EligibleStake(&_WalletRegistry.CallOpts, stakingProvider)
}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistryCallerSession) EligibleStake(stakingProvider common.Address) (*big.Int, error) {
	return _WalletRegistry.Contract.EligibleStake(&_WalletRegistry.CallOpts, stakingProvider)
}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_WalletRegistry *WalletRegistryCaller) GasParameters(opts *bind.CallOpts) (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
}, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "gasParameters")

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
func (_WalletRegistry *WalletRegistrySession) GasParameters() (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
}, error) {
	return _WalletRegistry.Contract.GasParameters(&_WalletRegistry.CallOpts)
}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_WalletRegistry *WalletRegistryCallerSession) GasParameters() (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
}, error) {
	return _WalletRegistry.Contract.GasParameters(&_WalletRegistry.CallOpts)
}

// GetWallet is a free data retrieval call binding the contract method 0x789d392a.
//
// Solidity: function getWallet(bytes32 walletID) view returns((bytes32,bytes32,bytes32))
func (_WalletRegistry *WalletRegistryCaller) GetWallet(opts *bind.CallOpts, walletID [32]byte) (WalletsWallet, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "getWallet", walletID)

	if err != nil {
		return *new(WalletsWallet), err
	}

	out0 := *abi.ConvertType(out[0], new(WalletsWallet)).(*WalletsWallet)

	return out0, err

}

// GetWallet is a free data retrieval call binding the contract method 0x789d392a.
//
// Solidity: function getWallet(bytes32 walletID) view returns((bytes32,bytes32,bytes32))
func (_WalletRegistry *WalletRegistrySession) GetWallet(walletID [32]byte) (WalletsWallet, error) {
	return _WalletRegistry.Contract.GetWallet(&_WalletRegistry.CallOpts, walletID)
}

// GetWallet is a free data retrieval call binding the contract method 0x789d392a.
//
// Solidity: function getWallet(bytes32 walletID) view returns((bytes32,bytes32,bytes32))
func (_WalletRegistry *WalletRegistryCallerSession) GetWallet(walletID [32]byte) (WalletsWallet, error) {
	return _WalletRegistry.Contract.GetWallet(&_WalletRegistry.CallOpts, walletID)
}

// GetWalletCreationState is a free data retrieval call binding the contract method 0xcc562388.
//
// Solidity: function getWalletCreationState() view returns(uint8)
func (_WalletRegistry *WalletRegistryCaller) GetWalletCreationState(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "getWalletCreationState")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetWalletCreationState is a free data retrieval call binding the contract method 0xcc562388.
//
// Solidity: function getWalletCreationState() view returns(uint8)
func (_WalletRegistry *WalletRegistrySession) GetWalletCreationState() (uint8, error) {
	return _WalletRegistry.Contract.GetWalletCreationState(&_WalletRegistry.CallOpts)
}

// GetWalletCreationState is a free data retrieval call binding the contract method 0xcc562388.
//
// Solidity: function getWalletCreationState() view returns(uint8)
func (_WalletRegistry *WalletRegistryCallerSession) GetWalletCreationState() (uint8, error) {
	return _WalletRegistry.Contract.GetWalletCreationState(&_WalletRegistry.CallOpts)
}

// GetWalletPublicKey is a free data retrieval call binding the contract method 0xb5e9ce8b.
//
// Solidity: function getWalletPublicKey(bytes32 walletID) view returns(bytes)
func (_WalletRegistry *WalletRegistryCaller) GetWalletPublicKey(opts *bind.CallOpts, walletID [32]byte) ([]byte, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "getWalletPublicKey", walletID)

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// GetWalletPublicKey is a free data retrieval call binding the contract method 0xb5e9ce8b.
//
// Solidity: function getWalletPublicKey(bytes32 walletID) view returns(bytes)
func (_WalletRegistry *WalletRegistrySession) GetWalletPublicKey(walletID [32]byte) ([]byte, error) {
	return _WalletRegistry.Contract.GetWalletPublicKey(&_WalletRegistry.CallOpts, walletID)
}

// GetWalletPublicKey is a free data retrieval call binding the contract method 0xb5e9ce8b.
//
// Solidity: function getWalletPublicKey(bytes32 walletID) view returns(bytes)
func (_WalletRegistry *WalletRegistryCallerSession) GetWalletPublicKey(walletID [32]byte) ([]byte, error) {
	return _WalletRegistry.Contract.GetWalletPublicKey(&_WalletRegistry.CallOpts, walletID)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_WalletRegistry *WalletRegistryCaller) Governance(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "governance")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_WalletRegistry *WalletRegistrySession) Governance() (common.Address, error) {
	return _WalletRegistry.Contract.Governance(&_WalletRegistry.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) Governance() (common.Address, error) {
	return _WalletRegistry.Contract.Governance(&_WalletRegistry.CallOpts)
}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_WalletRegistry *WalletRegistryCaller) HasDkgTimedOut(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "hasDkgTimedOut")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_WalletRegistry *WalletRegistrySession) HasDkgTimedOut() (bool, error) {
	return _WalletRegistry.Contract.HasDkgTimedOut(&_WalletRegistry.CallOpts)
}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_WalletRegistry *WalletRegistryCallerSession) HasDkgTimedOut() (bool, error) {
	return _WalletRegistry.Contract.HasDkgTimedOut(&_WalletRegistry.CallOpts)
}

// HasSeedTimedOut is a free data retrieval call binding the contract method 0x770124d3.
//
// Solidity: function hasSeedTimedOut() view returns(bool)
func (_WalletRegistry *WalletRegistryCaller) HasSeedTimedOut(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "hasSeedTimedOut")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasSeedTimedOut is a free data retrieval call binding the contract method 0x770124d3.
//
// Solidity: function hasSeedTimedOut() view returns(bool)
func (_WalletRegistry *WalletRegistrySession) HasSeedTimedOut() (bool, error) {
	return _WalletRegistry.Contract.HasSeedTimedOut(&_WalletRegistry.CallOpts)
}

// HasSeedTimedOut is a free data retrieval call binding the contract method 0x770124d3.
//
// Solidity: function hasSeedTimedOut() view returns(bool)
func (_WalletRegistry *WalletRegistryCallerSession) HasSeedTimedOut() (bool, error) {
	return _WalletRegistry.Contract.HasSeedTimedOut(&_WalletRegistry.CallOpts)
}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0x830f9e02.
//
// Solidity: function inactivityClaimNonce(bytes32 ) view returns(uint256)
func (_WalletRegistry *WalletRegistryCaller) InactivityClaimNonce(opts *bind.CallOpts, arg0 [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "inactivityClaimNonce", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0x830f9e02.
//
// Solidity: function inactivityClaimNonce(bytes32 ) view returns(uint256)
func (_WalletRegistry *WalletRegistrySession) InactivityClaimNonce(arg0 [32]byte) (*big.Int, error) {
	return _WalletRegistry.Contract.InactivityClaimNonce(&_WalletRegistry.CallOpts, arg0)
}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0x830f9e02.
//
// Solidity: function inactivityClaimNonce(bytes32 ) view returns(uint256)
func (_WalletRegistry *WalletRegistryCallerSession) InactivityClaimNonce(arg0 [32]byte) (*big.Int, error) {
	return _WalletRegistry.Contract.InactivityClaimNonce(&_WalletRegistry.CallOpts, arg0)
}

// IsDkgResultValid is a free data retrieval call binding the contract method 0xe83ab3a5.
//
// Solidity: function isDkgResultValid((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result) view returns(bool, string)
func (_WalletRegistry *WalletRegistryCaller) IsDkgResultValid(opts *bind.CallOpts, result EcdsaDkgResult) (bool, string, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "isDkgResultValid", result)

	if err != nil {
		return *new(bool), *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	out1 := *abi.ConvertType(out[1], new(string)).(*string)

	return out0, out1, err

}

// IsDkgResultValid is a free data retrieval call binding the contract method 0xe83ab3a5.
//
// Solidity: function isDkgResultValid((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result) view returns(bool, string)
func (_WalletRegistry *WalletRegistrySession) IsDkgResultValid(result EcdsaDkgResult) (bool, string, error) {
	return _WalletRegistry.Contract.IsDkgResultValid(&_WalletRegistry.CallOpts, result)
}

// IsDkgResultValid is a free data retrieval call binding the contract method 0xe83ab3a5.
//
// Solidity: function isDkgResultValid((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result) view returns(bool, string)
func (_WalletRegistry *WalletRegistryCallerSession) IsDkgResultValid(result EcdsaDkgResult) (bool, string, error) {
	return _WalletRegistry.Contract.IsDkgResultValid(&_WalletRegistry.CallOpts, result)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_WalletRegistry *WalletRegistryCaller) IsOperatorInPool(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "isOperatorInPool", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_WalletRegistry *WalletRegistrySession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _WalletRegistry.Contract.IsOperatorInPool(&_WalletRegistry.CallOpts, operator)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_WalletRegistry *WalletRegistryCallerSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _WalletRegistry.Contract.IsOperatorInPool(&_WalletRegistry.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_WalletRegistry *WalletRegistryCaller) IsOperatorUpToDate(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "isOperatorUpToDate", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_WalletRegistry *WalletRegistrySession) IsOperatorUpToDate(operator common.Address) (bool, error) {
	return _WalletRegistry.Contract.IsOperatorUpToDate(&_WalletRegistry.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_WalletRegistry *WalletRegistryCallerSession) IsOperatorUpToDate(operator common.Address) (bool, error) {
	return _WalletRegistry.Contract.IsOperatorUpToDate(&_WalletRegistry.CallOpts, operator)
}

// IsWalletMember is a free data retrieval call binding the contract method 0xdf07ce59.
//
// Solidity: function isWalletMember(bytes32 walletID, uint32[] walletMembersIDs, address operator, uint256 walletMemberIndex) view returns(bool)
func (_WalletRegistry *WalletRegistryCaller) IsWalletMember(opts *bind.CallOpts, walletID [32]byte, walletMembersIDs []uint32, operator common.Address, walletMemberIndex *big.Int) (bool, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "isWalletMember", walletID, walletMembersIDs, operator, walletMemberIndex)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsWalletMember is a free data retrieval call binding the contract method 0xdf07ce59.
//
// Solidity: function isWalletMember(bytes32 walletID, uint32[] walletMembersIDs, address operator, uint256 walletMemberIndex) view returns(bool)
func (_WalletRegistry *WalletRegistrySession) IsWalletMember(walletID [32]byte, walletMembersIDs []uint32, operator common.Address, walletMemberIndex *big.Int) (bool, error) {
	return _WalletRegistry.Contract.IsWalletMember(&_WalletRegistry.CallOpts, walletID, walletMembersIDs, operator, walletMemberIndex)
}

// IsWalletMember is a free data retrieval call binding the contract method 0xdf07ce59.
//
// Solidity: function isWalletMember(bytes32 walletID, uint32[] walletMembersIDs, address operator, uint256 walletMemberIndex) view returns(bool)
func (_WalletRegistry *WalletRegistryCallerSession) IsWalletMember(walletID [32]byte, walletMembersIDs []uint32, operator common.Address, walletMemberIndex *big.Int) (bool, error) {
	return _WalletRegistry.Contract.IsWalletMember(&_WalletRegistry.CallOpts, walletID, walletMembersIDs, operator, walletMemberIndex)
}

// IsWalletRegistered is a free data retrieval call binding the contract method 0x4d99f473.
//
// Solidity: function isWalletRegistered(bytes32 walletID) view returns(bool)
func (_WalletRegistry *WalletRegistryCaller) IsWalletRegistered(opts *bind.CallOpts, walletID [32]byte) (bool, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "isWalletRegistered", walletID)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsWalletRegistered is a free data retrieval call binding the contract method 0x4d99f473.
//
// Solidity: function isWalletRegistered(bytes32 walletID) view returns(bool)
func (_WalletRegistry *WalletRegistrySession) IsWalletRegistered(walletID [32]byte) (bool, error) {
	return _WalletRegistry.Contract.IsWalletRegistered(&_WalletRegistry.CallOpts, walletID)
}

// IsWalletRegistered is a free data retrieval call binding the contract method 0x4d99f473.
//
// Solidity: function isWalletRegistered(bytes32 walletID) view returns(bool)
func (_WalletRegistry *WalletRegistryCallerSession) IsWalletRegistered(walletID [32]byte) (bool, error) {
	return _WalletRegistry.Contract.IsWalletRegistered(&_WalletRegistry.CallOpts, walletID)
}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_WalletRegistry *WalletRegistryCaller) MinimumAuthorization(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "minimumAuthorization")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_WalletRegistry *WalletRegistrySession) MinimumAuthorization() (*big.Int, error) {
	return _WalletRegistry.Contract.MinimumAuthorization(&_WalletRegistry.CallOpts)
}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_WalletRegistry *WalletRegistryCallerSession) MinimumAuthorization() (*big.Int, error) {
	return _WalletRegistry.Contract.MinimumAuthorization(&_WalletRegistry.CallOpts)
}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_WalletRegistry *WalletRegistryCaller) OperatorToStakingProvider(opts *bind.CallOpts, operator common.Address) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "operatorToStakingProvider", operator)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_WalletRegistry *WalletRegistrySession) OperatorToStakingProvider(operator common.Address) (common.Address, error) {
	return _WalletRegistry.Contract.OperatorToStakingProvider(&_WalletRegistry.CallOpts, operator)
}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) OperatorToStakingProvider(operator common.Address) (common.Address, error) {
	return _WalletRegistry.Contract.OperatorToStakingProvider(&_WalletRegistry.CallOpts, operator)
}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistryCaller) PendingAuthorizationDecrease(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "pendingAuthorizationDecrease", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistrySession) PendingAuthorizationDecrease(stakingProvider common.Address) (*big.Int, error) {
	return _WalletRegistry.Contract.PendingAuthorizationDecrease(&_WalletRegistry.CallOpts, stakingProvider)
}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_WalletRegistry *WalletRegistryCallerSession) PendingAuthorizationDecrease(stakingProvider common.Address) (*big.Int, error) {
	return _WalletRegistry.Contract.PendingAuthorizationDecrease(&_WalletRegistry.CallOpts, stakingProvider)
}

// RandomBeacon is a free data retrieval call binding the contract method 0x153622b3.
//
// Solidity: function randomBeacon() view returns(address)
func (_WalletRegistry *WalletRegistryCaller) RandomBeacon(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "randomBeacon")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// RandomBeacon is a free data retrieval call binding the contract method 0x153622b3.
//
// Solidity: function randomBeacon() view returns(address)
func (_WalletRegistry *WalletRegistrySession) RandomBeacon() (common.Address, error) {
	return _WalletRegistry.Contract.RandomBeacon(&_WalletRegistry.CallOpts)
}

// RandomBeacon is a free data retrieval call binding the contract method 0x153622b3.
//
// Solidity: function randomBeacon() view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) RandomBeacon() (common.Address, error) {
	return _WalletRegistry.Contract.RandomBeacon(&_WalletRegistry.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_WalletRegistry *WalletRegistryCaller) ReimbursementPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "reimbursementPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_WalletRegistry *WalletRegistrySession) ReimbursementPool() (common.Address, error) {
	return _WalletRegistry.Contract.ReimbursementPool(&_WalletRegistry.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) ReimbursementPool() (common.Address, error) {
	return _WalletRegistry.Contract.ReimbursementPool(&_WalletRegistry.CallOpts)
}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_WalletRegistry *WalletRegistryCaller) RemainingAuthorizationDecreaseDelay(opts *bind.CallOpts, stakingProvider common.Address) (uint64, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "remainingAuthorizationDecreaseDelay", stakingProvider)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_WalletRegistry *WalletRegistrySession) RemainingAuthorizationDecreaseDelay(stakingProvider common.Address) (uint64, error) {
	return _WalletRegistry.Contract.RemainingAuthorizationDecreaseDelay(&_WalletRegistry.CallOpts, stakingProvider)
}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_WalletRegistry *WalletRegistryCallerSession) RemainingAuthorizationDecreaseDelay(stakingProvider common.Address) (uint64, error) {
	return _WalletRegistry.Contract.RemainingAuthorizationDecreaseDelay(&_WalletRegistry.CallOpts, stakingProvider)
}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_WalletRegistry *WalletRegistryCaller) RewardParameters(opts *bind.CallOpts) (struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
}, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "rewardParameters")

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
func (_WalletRegistry *WalletRegistrySession) RewardParameters() (struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
}, error) {
	return _WalletRegistry.Contract.RewardParameters(&_WalletRegistry.CallOpts)
}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_WalletRegistry *WalletRegistryCallerSession) RewardParameters() (struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
}, error) {
	return _WalletRegistry.Contract.RewardParameters(&_WalletRegistry.CallOpts)
}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_WalletRegistry *WalletRegistryCaller) SelectGroup(opts *bind.CallOpts) ([]uint32, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "selectGroup")

	if err != nil {
		return *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint32)).(*[]uint32)

	return out0, err

}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_WalletRegistry *WalletRegistrySession) SelectGroup() ([]uint32, error) {
	return _WalletRegistry.Contract.SelectGroup(&_WalletRegistry.CallOpts)
}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_WalletRegistry *WalletRegistryCallerSession) SelectGroup() ([]uint32, error) {
	return _WalletRegistry.Contract.SelectGroup(&_WalletRegistry.CallOpts)
}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 maliciousDkgResultSlashingAmount)
func (_WalletRegistry *WalletRegistryCaller) SlashingParameters(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "slashingParameters")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 maliciousDkgResultSlashingAmount)
func (_WalletRegistry *WalletRegistrySession) SlashingParameters() (*big.Int, error) {
	return _WalletRegistry.Contract.SlashingParameters(&_WalletRegistry.CallOpts)
}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 maliciousDkgResultSlashingAmount)
func (_WalletRegistry *WalletRegistryCallerSession) SlashingParameters() (*big.Int, error) {
	return _WalletRegistry.Contract.SlashingParameters(&_WalletRegistry.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_WalletRegistry *WalletRegistryCaller) SortitionPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "sortitionPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_WalletRegistry *WalletRegistrySession) SortitionPool() (common.Address, error) {
	return _WalletRegistry.Contract.SortitionPool(&_WalletRegistry.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) SortitionPool() (common.Address, error) {
	return _WalletRegistry.Contract.SortitionPool(&_WalletRegistry.CallOpts)
}

// Staking is a free data retrieval call binding the contract method 0x4cf088d9.
//
// Solidity: function staking() view returns(address)
func (_WalletRegistry *WalletRegistryCaller) Staking(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "staking")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Staking is a free data retrieval call binding the contract method 0x4cf088d9.
//
// Solidity: function staking() view returns(address)
func (_WalletRegistry *WalletRegistrySession) Staking() (common.Address, error) {
	return _WalletRegistry.Contract.Staking(&_WalletRegistry.CallOpts)
}

// Staking is a free data retrieval call binding the contract method 0x4cf088d9.
//
// Solidity: function staking() view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) Staking() (common.Address, error) {
	return _WalletRegistry.Contract.Staking(&_WalletRegistry.CallOpts)
}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_WalletRegistry *WalletRegistryCaller) StakingProviderToOperator(opts *bind.CallOpts, stakingProvider common.Address) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "stakingProviderToOperator", stakingProvider)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_WalletRegistry *WalletRegistrySession) StakingProviderToOperator(stakingProvider common.Address) (common.Address, error) {
	return _WalletRegistry.Contract.StakingProviderToOperator(&_WalletRegistry.CallOpts, stakingProvider)
}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) StakingProviderToOperator(stakingProvider common.Address) (common.Address, error) {
	return _WalletRegistry.Contract.StakingProviderToOperator(&_WalletRegistry.CallOpts, stakingProvider)
}

// WalletOwner is a free data retrieval call binding the contract method 0x1ae879e8.
//
// Solidity: function walletOwner() view returns(address)
func (_WalletRegistry *WalletRegistryCaller) WalletOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _WalletRegistry.contract.Call(opts, &out, "walletOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// WalletOwner is a free data retrieval call binding the contract method 0x1ae879e8.
//
// Solidity: function walletOwner() view returns(address)
func (_WalletRegistry *WalletRegistrySession) WalletOwner() (common.Address, error) {
	return _WalletRegistry.Contract.WalletOwner(&_WalletRegistry.CallOpts)
}

// WalletOwner is a free data retrieval call binding the contract method 0x1ae879e8.
//
// Solidity: function walletOwner() view returns(address)
func (_WalletRegistry *WalletRegistryCallerSession) WalletOwner() (common.Address, error) {
	return _WalletRegistry.Contract.WalletOwner(&_WalletRegistry.CallOpts)
}

// BeaconCallback is a paid mutator transaction binding the contract method 0x6febd464.
//
// Solidity: function __beaconCallback(uint256 relayEntry, uint256 ) returns()
func (_WalletRegistry *WalletRegistryTransactor) BeaconCallback(opts *bind.TransactOpts, relayEntry *big.Int, arg1 *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "__beaconCallback", relayEntry, arg1)
}

// BeaconCallback is a paid mutator transaction binding the contract method 0x6febd464.
//
// Solidity: function __beaconCallback(uint256 relayEntry, uint256 ) returns()
func (_WalletRegistry *WalletRegistrySession) BeaconCallback(relayEntry *big.Int, arg1 *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.BeaconCallback(&_WalletRegistry.TransactOpts, relayEntry, arg1)
}

// BeaconCallback is a paid mutator transaction binding the contract method 0x6febd464.
//
// Solidity: function __beaconCallback(uint256 relayEntry, uint256 ) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) BeaconCallback(relayEntry *big.Int, arg1 *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.BeaconCallback(&_WalletRegistry.TransactOpts, relayEntry, arg1)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_WalletRegistry *WalletRegistryTransactor) ApproveAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "approveAuthorizationDecrease", stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_WalletRegistry *WalletRegistrySession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.ApproveAuthorizationDecrease(&_WalletRegistry.TransactOpts, stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.ApproveAuthorizationDecrease(&_WalletRegistry.TransactOpts, stakingProvider)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x5c5b3870.
//
// Solidity: function approveDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistryTransactor) ApproveDkgResult(opts *bind.TransactOpts, dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "approveDkgResult", dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x5c5b3870.
//
// Solidity: function approveDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistrySession) ApproveDkgResult(dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.Contract.ApproveDkgResult(&_WalletRegistry.TransactOpts, dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x5c5b3870.
//
// Solidity: function approveDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) ApproveDkgResult(dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.Contract.ApproveDkgResult(&_WalletRegistry.TransactOpts, dkgResult)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistryTransactor) AuthorizationDecreaseRequested(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "authorizationDecreaseRequested", stakingProvider, fromAmount, toAmount)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistrySession) AuthorizationDecreaseRequested(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.AuthorizationDecreaseRequested(&_WalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) AuthorizationDecreaseRequested(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.AuthorizationDecreaseRequested(&_WalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistryTransactor) AuthorizationIncreased(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "authorizationIncreased", stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistrySession) AuthorizationIncreased(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.AuthorizationIncreased(&_WalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) AuthorizationIncreased(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.AuthorizationIncreased(&_WalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x31376766.
//
// Solidity: function challengeDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistryTransactor) ChallengeDkgResult(opts *bind.TransactOpts, dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "challengeDkgResult", dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x31376766.
//
// Solidity: function challengeDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistrySession) ChallengeDkgResult(dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.Contract.ChallengeDkgResult(&_WalletRegistry.TransactOpts, dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x31376766.
//
// Solidity: function challengeDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) ChallengeDkgResult(dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.Contract.ChallengeDkgResult(&_WalletRegistry.TransactOpts, dkgResult)
}

// CloseWallet is a paid mutator transaction binding the contract method 0x343bb927.
//
// Solidity: function closeWallet(bytes32 walletID) returns()
func (_WalletRegistry *WalletRegistryTransactor) CloseWallet(opts *bind.TransactOpts, walletID [32]byte) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "closeWallet", walletID)
}

// CloseWallet is a paid mutator transaction binding the contract method 0x343bb927.
//
// Solidity: function closeWallet(bytes32 walletID) returns()
func (_WalletRegistry *WalletRegistrySession) CloseWallet(walletID [32]byte) (*types.Transaction, error) {
	return _WalletRegistry.Contract.CloseWallet(&_WalletRegistry.TransactOpts, walletID)
}

// CloseWallet is a paid mutator transaction binding the contract method 0x343bb927.
//
// Solidity: function closeWallet(bytes32 walletID) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) CloseWallet(walletID [32]byte) (*types.Transaction, error) {
	return _WalletRegistry.Contract.CloseWallet(&_WalletRegistry.TransactOpts, walletID)
}

// Initialize is a paid mutator transaction binding the contract method 0xc0c53b8b.
//
// Solidity: function initialize(address _ecdsaDkgValidator, address _randomBeacon, address _reimbursementPool) returns()
func (_WalletRegistry *WalletRegistryTransactor) Initialize(opts *bind.TransactOpts, _ecdsaDkgValidator common.Address, _randomBeacon common.Address, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "initialize", _ecdsaDkgValidator, _randomBeacon, _reimbursementPool)
}

// Initialize is a paid mutator transaction binding the contract method 0xc0c53b8b.
//
// Solidity: function initialize(address _ecdsaDkgValidator, address _randomBeacon, address _reimbursementPool) returns()
func (_WalletRegistry *WalletRegistrySession) Initialize(_ecdsaDkgValidator common.Address, _randomBeacon common.Address, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.Initialize(&_WalletRegistry.TransactOpts, _ecdsaDkgValidator, _randomBeacon, _reimbursementPool)
}

// Initialize is a paid mutator transaction binding the contract method 0xc0c53b8b.
//
// Solidity: function initialize(address _ecdsaDkgValidator, address _randomBeacon, address _reimbursementPool) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) Initialize(_ecdsaDkgValidator common.Address, _randomBeacon common.Address, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.Initialize(&_WalletRegistry.TransactOpts, _ecdsaDkgValidator, _randomBeacon, _reimbursementPool)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistryTransactor) InvoluntaryAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "involuntaryAuthorizationDecrease", stakingProvider, fromAmount, toAmount)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistrySession) InvoluntaryAuthorizationDecrease(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.InvoluntaryAuthorizationDecrease(&_WalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) InvoluntaryAuthorizationDecrease(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.InvoluntaryAuthorizationDecrease(&_WalletRegistry.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_WalletRegistry *WalletRegistryTransactor) JoinSortitionPool(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "joinSortitionPool")
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_WalletRegistry *WalletRegistrySession) JoinSortitionPool() (*types.Transaction, error) {
	return _WalletRegistry.Contract.JoinSortitionPool(&_WalletRegistry.TransactOpts)
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_WalletRegistry *WalletRegistryTransactorSession) JoinSortitionPool() (*types.Transaction, error) {
	return _WalletRegistry.Contract.JoinSortitionPool(&_WalletRegistry.TransactOpts)
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_WalletRegistry *WalletRegistryTransactor) NotifyDkgTimeout(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "notifyDkgTimeout")
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_WalletRegistry *WalletRegistrySession) NotifyDkgTimeout() (*types.Transaction, error) {
	return _WalletRegistry.Contract.NotifyDkgTimeout(&_WalletRegistry.TransactOpts)
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_WalletRegistry *WalletRegistryTransactorSession) NotifyDkgTimeout() (*types.Transaction, error) {
	return _WalletRegistry.Contract.NotifyDkgTimeout(&_WalletRegistry.TransactOpts)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0x9879d19b.
//
// Solidity: function notifyOperatorInactivity((bytes32,uint256[],bool,bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_WalletRegistry *WalletRegistryTransactor) NotifyOperatorInactivity(opts *bind.TransactOpts, claim EcdsaInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "notifyOperatorInactivity", claim, nonce, groupMembers)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0x9879d19b.
//
// Solidity: function notifyOperatorInactivity((bytes32,uint256[],bool,bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_WalletRegistry *WalletRegistrySession) NotifyOperatorInactivity(claim EcdsaInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _WalletRegistry.Contract.NotifyOperatorInactivity(&_WalletRegistry.TransactOpts, claim, nonce, groupMembers)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0x9879d19b.
//
// Solidity: function notifyOperatorInactivity((bytes32,uint256[],bool,bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) NotifyOperatorInactivity(claim EcdsaInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _WalletRegistry.Contract.NotifyOperatorInactivity(&_WalletRegistry.TransactOpts, claim, nonce, groupMembers)
}

// NotifySeedTimeout is a paid mutator transaction binding the contract method 0xb13b55b2.
//
// Solidity: function notifySeedTimeout() returns()
func (_WalletRegistry *WalletRegistryTransactor) NotifySeedTimeout(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "notifySeedTimeout")
}

// NotifySeedTimeout is a paid mutator transaction binding the contract method 0xb13b55b2.
//
// Solidity: function notifySeedTimeout() returns()
func (_WalletRegistry *WalletRegistrySession) NotifySeedTimeout() (*types.Transaction, error) {
	return _WalletRegistry.Contract.NotifySeedTimeout(&_WalletRegistry.TransactOpts)
}

// NotifySeedTimeout is a paid mutator transaction binding the contract method 0xb13b55b2.
//
// Solidity: function notifySeedTimeout() returns()
func (_WalletRegistry *WalletRegistryTransactorSession) NotifySeedTimeout() (*types.Transaction, error) {
	return _WalletRegistry.Contract.NotifySeedTimeout(&_WalletRegistry.TransactOpts)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_WalletRegistry *WalletRegistryTransactor) RegisterOperator(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "registerOperator", operator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_WalletRegistry *WalletRegistrySession) RegisterOperator(operator common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.RegisterOperator(&_WalletRegistry.TransactOpts, operator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) RegisterOperator(operator common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.RegisterOperator(&_WalletRegistry.TransactOpts, operator)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0x72cc8c6d.
//
// Solidity: function requestNewWallet() returns()
func (_WalletRegistry *WalletRegistryTransactor) RequestNewWallet(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "requestNewWallet")
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0x72cc8c6d.
//
// Solidity: function requestNewWallet() returns()
func (_WalletRegistry *WalletRegistrySession) RequestNewWallet() (*types.Transaction, error) {
	return _WalletRegistry.Contract.RequestNewWallet(&_WalletRegistry.TransactOpts)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0x72cc8c6d.
//
// Solidity: function requestNewWallet() returns()
func (_WalletRegistry *WalletRegistryTransactorSession) RequestNewWallet() (*types.Transaction, error) {
	return _WalletRegistry.Contract.RequestNewWallet(&_WalletRegistry.TransactOpts)
}

// Seize is a paid mutator transaction binding the contract method 0xd8dc404d.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, bytes32 walletID, uint32[] walletMembersIDs) returns()
func (_WalletRegistry *WalletRegistryTransactor) Seize(opts *bind.TransactOpts, amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, walletID [32]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "seize", amount, rewardMultiplier, notifier, walletID, walletMembersIDs)
}

// Seize is a paid mutator transaction binding the contract method 0xd8dc404d.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, bytes32 walletID, uint32[] walletMembersIDs) returns()
func (_WalletRegistry *WalletRegistrySession) Seize(amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, walletID [32]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _WalletRegistry.Contract.Seize(&_WalletRegistry.TransactOpts, amount, rewardMultiplier, notifier, walletID, walletMembersIDs)
}

// Seize is a paid mutator transaction binding the contract method 0xd8dc404d.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, bytes32 walletID, uint32[] walletMembersIDs) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) Seize(amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, walletID [32]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _WalletRegistry.Contract.Seize(&_WalletRegistry.TransactOpts, amount, rewardMultiplier, notifier, walletID, walletMembersIDs)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x7e0049fd.
//
// Solidity: function submitDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistryTransactor) SubmitDkgResult(opts *bind.TransactOpts, dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "submitDkgResult", dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x7e0049fd.
//
// Solidity: function submitDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistrySession) SubmitDkgResult(dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.Contract.SubmitDkgResult(&_WalletRegistry.TransactOpts, dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x7e0049fd.
//
// Solidity: function submitDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) SubmitDkgResult(dkgResult EcdsaDkgResult) (*types.Transaction, error) {
	return _WalletRegistry.Contract.SubmitDkgResult(&_WalletRegistry.TransactOpts, dkgResult)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_WalletRegistry *WalletRegistryTransactor) TransferGovernance(opts *bind.TransactOpts, newGovernance common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "transferGovernance", newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_WalletRegistry *WalletRegistrySession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.TransferGovernance(&_WalletRegistry.TransactOpts, newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.TransferGovernance(&_WalletRegistry.TransactOpts, newGovernance)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateAuthorizationParameters(opts *bind.TransactOpts, _minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateAuthorizationParameters", _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateAuthorizationParameters(_minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateAuthorizationParameters(&_WalletRegistry.TransactOpts, _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateAuthorizationParameters(_minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateAuthorizationParameters(&_WalletRegistry.TransactOpts, _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateDkgParameters is a paid mutator transaction binding the contract method 0x8dcbdf4a.
//
// Solidity: function updateDkgParameters(uint256 _seedTimeout, uint256 _resultChallengePeriodLength, uint256 _resultChallengeExtraGas, uint256 _resultSubmissionTimeout, uint256 _submitterPrecedencePeriodLength) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateDkgParameters(opts *bind.TransactOpts, _seedTimeout *big.Int, _resultChallengePeriodLength *big.Int, _resultChallengeExtraGas *big.Int, _resultSubmissionTimeout *big.Int, _submitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateDkgParameters", _seedTimeout, _resultChallengePeriodLength, _resultChallengeExtraGas, _resultSubmissionTimeout, _submitterPrecedencePeriodLength)
}

// UpdateDkgParameters is a paid mutator transaction binding the contract method 0x8dcbdf4a.
//
// Solidity: function updateDkgParameters(uint256 _seedTimeout, uint256 _resultChallengePeriodLength, uint256 _resultChallengeExtraGas, uint256 _resultSubmissionTimeout, uint256 _submitterPrecedencePeriodLength) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateDkgParameters(_seedTimeout *big.Int, _resultChallengePeriodLength *big.Int, _resultChallengeExtraGas *big.Int, _resultSubmissionTimeout *big.Int, _submitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateDkgParameters(&_WalletRegistry.TransactOpts, _seedTimeout, _resultChallengePeriodLength, _resultChallengeExtraGas, _resultSubmissionTimeout, _submitterPrecedencePeriodLength)
}

// UpdateDkgParameters is a paid mutator transaction binding the contract method 0x8dcbdf4a.
//
// Solidity: function updateDkgParameters(uint256 _seedTimeout, uint256 _resultChallengePeriodLength, uint256 _resultChallengeExtraGas, uint256 _resultSubmissionTimeout, uint256 _submitterPrecedencePeriodLength) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateDkgParameters(_seedTimeout *big.Int, _resultChallengePeriodLength *big.Int, _resultChallengeExtraGas *big.Int, _resultSubmissionTimeout *big.Int, _submitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateDkgParameters(&_WalletRegistry.TransactOpts, _seedTimeout, _resultChallengePeriodLength, _resultChallengeExtraGas, _resultSubmissionTimeout, _submitterPrecedencePeriodLength)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xc88e70f4.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateGasParameters(opts *bind.TransactOpts, dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, notifySeedTimeoutGasOffset *big.Int, notifyDkgTimeoutNegativeGasOffset *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateGasParameters", dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, notifySeedTimeoutGasOffset, notifyDkgTimeoutNegativeGasOffset)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xc88e70f4.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateGasParameters(dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, notifySeedTimeoutGasOffset *big.Int, notifyDkgTimeoutNegativeGasOffset *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateGasParameters(&_WalletRegistry.TransactOpts, dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, notifySeedTimeoutGasOffset, notifyDkgTimeoutNegativeGasOffset)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xc88e70f4.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateGasParameters(dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, notifySeedTimeoutGasOffset *big.Int, notifyDkgTimeoutNegativeGasOffset *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateGasParameters(&_WalletRegistry.TransactOpts, dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, notifySeedTimeoutGasOffset, notifyDkgTimeoutNegativeGasOffset)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateOperatorStatus(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateOperatorStatus", operator)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateOperatorStatus(operator common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateOperatorStatus(&_WalletRegistry.TransactOpts, operator)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateOperatorStatus(operator common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateOperatorStatus(&_WalletRegistry.TransactOpts, operator)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateReimbursementPool(opts *bind.TransactOpts, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateReimbursementPool", _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateReimbursementPool(&_WalletRegistry.TransactOpts, _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateReimbursementPool(&_WalletRegistry.TransactOpts, _reimbursementPool)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x6c9ecd64.
//
// Solidity: function updateRewardParameters(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateRewardParameters(opts *bind.TransactOpts, maliciousDkgResultNotificationRewardMultiplier *big.Int, sortitionPoolRewardsBanDuration *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateRewardParameters", maliciousDkgResultNotificationRewardMultiplier, sortitionPoolRewardsBanDuration)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x6c9ecd64.
//
// Solidity: function updateRewardParameters(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateRewardParameters(maliciousDkgResultNotificationRewardMultiplier *big.Int, sortitionPoolRewardsBanDuration *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateRewardParameters(&_WalletRegistry.TransactOpts, maliciousDkgResultNotificationRewardMultiplier, sortitionPoolRewardsBanDuration)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x6c9ecd64.
//
// Solidity: function updateRewardParameters(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateRewardParameters(maliciousDkgResultNotificationRewardMultiplier *big.Int, sortitionPoolRewardsBanDuration *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateRewardParameters(&_WalletRegistry.TransactOpts, maliciousDkgResultNotificationRewardMultiplier, sortitionPoolRewardsBanDuration)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x227fd44f.
//
// Solidity: function updateSlashingParameters(uint96 maliciousDkgResultSlashingAmount) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateSlashingParameters(opts *bind.TransactOpts, maliciousDkgResultSlashingAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateSlashingParameters", maliciousDkgResultSlashingAmount)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x227fd44f.
//
// Solidity: function updateSlashingParameters(uint96 maliciousDkgResultSlashingAmount) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateSlashingParameters(maliciousDkgResultSlashingAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateSlashingParameters(&_WalletRegistry.TransactOpts, maliciousDkgResultSlashingAmount)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x227fd44f.
//
// Solidity: function updateSlashingParameters(uint96 maliciousDkgResultSlashingAmount) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateSlashingParameters(maliciousDkgResultSlashingAmount *big.Int) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateSlashingParameters(&_WalletRegistry.TransactOpts, maliciousDkgResultSlashingAmount)
}

// UpdateWalletOwner is a paid mutator transaction binding the contract method 0xd0bcc0e3.
//
// Solidity: function updateWalletOwner(address _walletOwner) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpdateWalletOwner(opts *bind.TransactOpts, _walletOwner common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "updateWalletOwner", _walletOwner)
}

// UpdateWalletOwner is a paid mutator transaction binding the contract method 0xd0bcc0e3.
//
// Solidity: function updateWalletOwner(address _walletOwner) returns()
func (_WalletRegistry *WalletRegistrySession) UpdateWalletOwner(_walletOwner common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateWalletOwner(&_WalletRegistry.TransactOpts, _walletOwner)
}

// UpdateWalletOwner is a paid mutator transaction binding the contract method 0xd0bcc0e3.
//
// Solidity: function updateWalletOwner(address _walletOwner) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpdateWalletOwner(_walletOwner common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpdateWalletOwner(&_WalletRegistry.TransactOpts, _walletOwner)
}

// UpgradeRandomBeacon is a paid mutator transaction binding the contract method 0x6b5f2bff.
//
// Solidity: function upgradeRandomBeacon(address _randomBeacon) returns()
func (_WalletRegistry *WalletRegistryTransactor) UpgradeRandomBeacon(opts *bind.TransactOpts, _randomBeacon common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "upgradeRandomBeacon", _randomBeacon)
}

// UpgradeRandomBeacon is a paid mutator transaction binding the contract method 0x6b5f2bff.
//
// Solidity: function upgradeRandomBeacon(address _randomBeacon) returns()
func (_WalletRegistry *WalletRegistrySession) UpgradeRandomBeacon(_randomBeacon common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpgradeRandomBeacon(&_WalletRegistry.TransactOpts, _randomBeacon)
}

// UpgradeRandomBeacon is a paid mutator transaction binding the contract method 0x6b5f2bff.
//
// Solidity: function upgradeRandomBeacon(address _randomBeacon) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) UpgradeRandomBeacon(_randomBeacon common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.UpgradeRandomBeacon(&_WalletRegistry.TransactOpts, _randomBeacon)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_WalletRegistry *WalletRegistryTransactor) WithdrawIneligibleRewards(opts *bind.TransactOpts, recipient common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "withdrawIneligibleRewards", recipient)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_WalletRegistry *WalletRegistrySession) WithdrawIneligibleRewards(recipient common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.WithdrawIneligibleRewards(&_WalletRegistry.TransactOpts, recipient)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) WithdrawIneligibleRewards(recipient common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.WithdrawIneligibleRewards(&_WalletRegistry.TransactOpts, recipient)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_WalletRegistry *WalletRegistryTransactor) WithdrawRewards(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _WalletRegistry.contract.Transact(opts, "withdrawRewards", stakingProvider)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_WalletRegistry *WalletRegistrySession) WithdrawRewards(stakingProvider common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.WithdrawRewards(&_WalletRegistry.TransactOpts, stakingProvider)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_WalletRegistry *WalletRegistryTransactorSession) WithdrawRewards(stakingProvider common.Address) (*types.Transaction, error) {
	return _WalletRegistry.Contract.WithdrawRewards(&_WalletRegistry.TransactOpts, stakingProvider)
}

// WalletRegistryAuthorizationDecreaseApprovedIterator is returned from FilterAuthorizationDecreaseApproved and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseApproved events raised by the WalletRegistry contract.
type WalletRegistryAuthorizationDecreaseApprovedIterator struct {
	Event *WalletRegistryAuthorizationDecreaseApproved // Event containing the contract specifics and raw log

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
func (it *WalletRegistryAuthorizationDecreaseApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryAuthorizationDecreaseApproved)
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
		it.Event = new(WalletRegistryAuthorizationDecreaseApproved)
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
func (it *WalletRegistryAuthorizationDecreaseApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryAuthorizationDecreaseApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryAuthorizationDecreaseApproved represents a AuthorizationDecreaseApproved event raised by the WalletRegistry contract.
type WalletRegistryAuthorizationDecreaseApproved struct {
	StakingProvider common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationDecreaseApproved is a free log retrieval operation binding the contract event 0x50270a522c2fef97b6b7385c2aa4a4518adda681530e0a1fe9f5e840f6f2cd9d.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider)
func (_WalletRegistry *WalletRegistryFilterer) FilterAuthorizationDecreaseApproved(opts *bind.FilterOpts, stakingProvider []common.Address) (*WalletRegistryAuthorizationDecreaseApprovedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryAuthorizationDecreaseApprovedIterator{contract: _WalletRegistry.contract, event: "AuthorizationDecreaseApproved", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseApproved is a free log subscription operation binding the contract event 0x50270a522c2fef97b6b7385c2aa4a4518adda681530e0a1fe9f5e840f6f2cd9d.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider)
func (_WalletRegistry *WalletRegistryFilterer) WatchAuthorizationDecreaseApproved(opts *bind.WatchOpts, sink chan<- *WalletRegistryAuthorizationDecreaseApproved, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryAuthorizationDecreaseApproved)
				if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseAuthorizationDecreaseApproved(log types.Log) (*WalletRegistryAuthorizationDecreaseApproved, error) {
	event := new(WalletRegistryAuthorizationDecreaseApproved)
	if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryAuthorizationDecreaseRequestedIterator is returned from FilterAuthorizationDecreaseRequested and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseRequested events raised by the WalletRegistry contract.
type WalletRegistryAuthorizationDecreaseRequestedIterator struct {
	Event *WalletRegistryAuthorizationDecreaseRequested // Event containing the contract specifics and raw log

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
func (it *WalletRegistryAuthorizationDecreaseRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryAuthorizationDecreaseRequested)
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
		it.Event = new(WalletRegistryAuthorizationDecreaseRequested)
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
func (it *WalletRegistryAuthorizationDecreaseRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryAuthorizationDecreaseRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryAuthorizationDecreaseRequested represents a AuthorizationDecreaseRequested event raised by the WalletRegistry contract.
type WalletRegistryAuthorizationDecreaseRequested struct {
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
func (_WalletRegistry *WalletRegistryFilterer) FilterAuthorizationDecreaseRequested(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*WalletRegistryAuthorizationDecreaseRequestedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryAuthorizationDecreaseRequestedIterator{contract: _WalletRegistry.contract, event: "AuthorizationDecreaseRequested", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseRequested is a free log subscription operation binding the contract event 0x545cbf267cef6fe43f11f6219417ab43a0e8e345adbaae5f626d9bc325e8535a.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount, uint64 decreasingAt)
func (_WalletRegistry *WalletRegistryFilterer) WatchAuthorizationDecreaseRequested(opts *bind.WatchOpts, sink chan<- *WalletRegistryAuthorizationDecreaseRequested, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryAuthorizationDecreaseRequested)
				if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseAuthorizationDecreaseRequested(log types.Log) (*WalletRegistryAuthorizationDecreaseRequested, error) {
	event := new(WalletRegistryAuthorizationDecreaseRequested)
	if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryAuthorizationIncreasedIterator is returned from FilterAuthorizationIncreased and is used to iterate over the raw logs and unpacked data for AuthorizationIncreased events raised by the WalletRegistry contract.
type WalletRegistryAuthorizationIncreasedIterator struct {
	Event *WalletRegistryAuthorizationIncreased // Event containing the contract specifics and raw log

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
func (it *WalletRegistryAuthorizationIncreasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryAuthorizationIncreased)
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
		it.Event = new(WalletRegistryAuthorizationIncreased)
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
func (it *WalletRegistryAuthorizationIncreasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryAuthorizationIncreasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryAuthorizationIncreased represents a AuthorizationIncreased event raised by the WalletRegistry contract.
type WalletRegistryAuthorizationIncreased struct {
	StakingProvider common.Address
	Operator        common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationIncreased is a free log retrieval operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_WalletRegistry *WalletRegistryFilterer) FilterAuthorizationIncreased(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*WalletRegistryAuthorizationIncreasedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "AuthorizationIncreased", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryAuthorizationIncreasedIterator{contract: _WalletRegistry.contract, event: "AuthorizationIncreased", logs: logs, sub: sub}, nil
}

// WatchAuthorizationIncreased is a free log subscription operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_WalletRegistry *WalletRegistryFilterer) WatchAuthorizationIncreased(opts *bind.WatchOpts, sink chan<- *WalletRegistryAuthorizationIncreased, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "AuthorizationIncreased", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryAuthorizationIncreased)
				if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseAuthorizationIncreased(log types.Log) (*WalletRegistryAuthorizationIncreased, error) {
	event := new(WalletRegistryAuthorizationIncreased)
	if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryAuthorizationParametersUpdatedIterator is returned from FilterAuthorizationParametersUpdated and is used to iterate over the raw logs and unpacked data for AuthorizationParametersUpdated events raised by the WalletRegistry contract.
type WalletRegistryAuthorizationParametersUpdatedIterator struct {
	Event *WalletRegistryAuthorizationParametersUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryAuthorizationParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryAuthorizationParametersUpdated)
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
		it.Event = new(WalletRegistryAuthorizationParametersUpdated)
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
func (it *WalletRegistryAuthorizationParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryAuthorizationParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryAuthorizationParametersUpdated represents a AuthorizationParametersUpdated event raised by the WalletRegistry contract.
type WalletRegistryAuthorizationParametersUpdated struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
	Raw                               types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationParametersUpdated is a free log retrieval operation binding the contract event 0x544b726e42801bb47073854eeedae851903f66fe32a5bd24e626e10b90027b51.
//
// Solidity: event AuthorizationParametersUpdated(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_WalletRegistry *WalletRegistryFilterer) FilterAuthorizationParametersUpdated(opts *bind.FilterOpts) (*WalletRegistryAuthorizationParametersUpdatedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "AuthorizationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryAuthorizationParametersUpdatedIterator{contract: _WalletRegistry.contract, event: "AuthorizationParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchAuthorizationParametersUpdated is a free log subscription operation binding the contract event 0x544b726e42801bb47073854eeedae851903f66fe32a5bd24e626e10b90027b51.
//
// Solidity: event AuthorizationParametersUpdated(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_WalletRegistry *WalletRegistryFilterer) WatchAuthorizationParametersUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistryAuthorizationParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "AuthorizationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryAuthorizationParametersUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationParametersUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseAuthorizationParametersUpdated(log types.Log) (*WalletRegistryAuthorizationParametersUpdated, error) {
	event := new(WalletRegistryAuthorizationParametersUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "AuthorizationParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgMaliciousResultSlashedIterator is returned from FilterDkgMaliciousResultSlashed and is used to iterate over the raw logs and unpacked data for DkgMaliciousResultSlashed events raised by the WalletRegistry contract.
type WalletRegistryDkgMaliciousResultSlashedIterator struct {
	Event *WalletRegistryDkgMaliciousResultSlashed // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgMaliciousResultSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgMaliciousResultSlashed)
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
		it.Event = new(WalletRegistryDkgMaliciousResultSlashed)
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
func (it *WalletRegistryDkgMaliciousResultSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgMaliciousResultSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgMaliciousResultSlashed represents a DkgMaliciousResultSlashed event raised by the WalletRegistry contract.
type WalletRegistryDkgMaliciousResultSlashed struct {
	ResultHash         [32]byte
	SlashingAmount     *big.Int
	MaliciousSubmitter common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterDkgMaliciousResultSlashed is a free log retrieval operation binding the contract event 0x88f76c659db78142f88e94db3ca791869495394c6c1b3d412ced9022dc97c9e3.
//
// Solidity: event DkgMaliciousResultSlashed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgMaliciousResultSlashed(opts *bind.FilterOpts, resultHash [][32]byte) (*WalletRegistryDkgMaliciousResultSlashedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgMaliciousResultSlashed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgMaliciousResultSlashedIterator{contract: _WalletRegistry.contract, event: "DkgMaliciousResultSlashed", logs: logs, sub: sub}, nil
}

// WatchDkgMaliciousResultSlashed is a free log subscription operation binding the contract event 0x88f76c659db78142f88e94db3ca791869495394c6c1b3d412ced9022dc97c9e3.
//
// Solidity: event DkgMaliciousResultSlashed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgMaliciousResultSlashed(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgMaliciousResultSlashed, resultHash [][32]byte) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgMaliciousResultSlashed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgMaliciousResultSlashed)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashed", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgMaliciousResultSlashed(log types.Log) (*WalletRegistryDkgMaliciousResultSlashed, error) {
	event := new(WalletRegistryDkgMaliciousResultSlashed)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgMaliciousResultSlashingFailedIterator is returned from FilterDkgMaliciousResultSlashingFailed and is used to iterate over the raw logs and unpacked data for DkgMaliciousResultSlashingFailed events raised by the WalletRegistry contract.
type WalletRegistryDkgMaliciousResultSlashingFailedIterator struct {
	Event *WalletRegistryDkgMaliciousResultSlashingFailed // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgMaliciousResultSlashingFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgMaliciousResultSlashingFailed)
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
		it.Event = new(WalletRegistryDkgMaliciousResultSlashingFailed)
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
func (it *WalletRegistryDkgMaliciousResultSlashingFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgMaliciousResultSlashingFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgMaliciousResultSlashingFailed represents a DkgMaliciousResultSlashingFailed event raised by the WalletRegistry contract.
type WalletRegistryDkgMaliciousResultSlashingFailed struct {
	ResultHash         [32]byte
	SlashingAmount     *big.Int
	MaliciousSubmitter common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterDkgMaliciousResultSlashingFailed is a free log retrieval operation binding the contract event 0x14621289a12ab59e0737decc388bba91d929c723defb4682d5d19b9a12ecfecb.
//
// Solidity: event DkgMaliciousResultSlashingFailed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgMaliciousResultSlashingFailed(opts *bind.FilterOpts, resultHash [][32]byte) (*WalletRegistryDkgMaliciousResultSlashingFailedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgMaliciousResultSlashingFailed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgMaliciousResultSlashingFailedIterator{contract: _WalletRegistry.contract, event: "DkgMaliciousResultSlashingFailed", logs: logs, sub: sub}, nil
}

// WatchDkgMaliciousResultSlashingFailed is a free log subscription operation binding the contract event 0x14621289a12ab59e0737decc388bba91d929c723defb4682d5d19b9a12ecfecb.
//
// Solidity: event DkgMaliciousResultSlashingFailed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgMaliciousResultSlashingFailed(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgMaliciousResultSlashingFailed, resultHash [][32]byte) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgMaliciousResultSlashingFailed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgMaliciousResultSlashingFailed)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashingFailed", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgMaliciousResultSlashingFailed(log types.Log) (*WalletRegistryDkgMaliciousResultSlashingFailed, error) {
	event := new(WalletRegistryDkgMaliciousResultSlashingFailed)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgMaliciousResultSlashingFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgParametersUpdatedIterator is returned from FilterDkgParametersUpdated and is used to iterate over the raw logs and unpacked data for DkgParametersUpdated events raised by the WalletRegistry contract.
type WalletRegistryDkgParametersUpdatedIterator struct {
	Event *WalletRegistryDkgParametersUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgParametersUpdated)
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
		it.Event = new(WalletRegistryDkgParametersUpdated)
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
func (it *WalletRegistryDkgParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgParametersUpdated represents a DkgParametersUpdated event raised by the WalletRegistry contract.
type WalletRegistryDkgParametersUpdated struct {
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
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgParametersUpdated(opts *bind.FilterOpts) (*WalletRegistryDkgParametersUpdatedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgParametersUpdatedIterator{contract: _WalletRegistry.contract, event: "DkgParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchDkgParametersUpdated is a free log subscription operation binding the contract event 0x59ae8ed7b3a7e5f6dde4cff478f0ac0aa652c5edc4f4757b09a778a430b02c56.
//
// Solidity: event DkgParametersUpdated(uint256 seedTimeout, uint256 resultChallengePeriodLength, uint256 resultChallengeExtraGas, uint256 resultSubmissionTimeout, uint256 resultSubmitterPrecedencePeriodLength)
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgParametersUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgParametersUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgParametersUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgParametersUpdated(log types.Log) (*WalletRegistryDkgParametersUpdated, error) {
	event := new(WalletRegistryDkgParametersUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgResultApprovedIterator is returned from FilterDkgResultApproved and is used to iterate over the raw logs and unpacked data for DkgResultApproved events raised by the WalletRegistry contract.
type WalletRegistryDkgResultApprovedIterator struct {
	Event *WalletRegistryDkgResultApproved // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgResultApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgResultApproved)
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
		it.Event = new(WalletRegistryDkgResultApproved)
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
func (it *WalletRegistryDkgResultApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgResultApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgResultApproved represents a DkgResultApproved event raised by the WalletRegistry contract.
type WalletRegistryDkgResultApproved struct {
	ResultHash [32]byte
	Approver   common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultApproved is a free log retrieval operation binding the contract event 0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f.
//
// Solidity: event DkgResultApproved(bytes32 indexed resultHash, address indexed approver)
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgResultApproved(opts *bind.FilterOpts, resultHash [][32]byte, approver []common.Address) (*WalletRegistryDkgResultApprovedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var approverRule []interface{}
	for _, approverItem := range approver {
		approverRule = append(approverRule, approverItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgResultApproved", resultHashRule, approverRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgResultApprovedIterator{contract: _WalletRegistry.contract, event: "DkgResultApproved", logs: logs, sub: sub}, nil
}

// WatchDkgResultApproved is a free log subscription operation binding the contract event 0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f.
//
// Solidity: event DkgResultApproved(bytes32 indexed resultHash, address indexed approver)
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgResultApproved(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgResultApproved, resultHash [][32]byte, approver []common.Address) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var approverRule []interface{}
	for _, approverItem := range approver {
		approverRule = append(approverRule, approverItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgResultApproved", resultHashRule, approverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgResultApproved)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgResultApproved", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgResultApproved(log types.Log) (*WalletRegistryDkgResultApproved, error) {
	event := new(WalletRegistryDkgResultApproved)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgResultApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgResultChallengedIterator is returned from FilterDkgResultChallenged and is used to iterate over the raw logs and unpacked data for DkgResultChallenged events raised by the WalletRegistry contract.
type WalletRegistryDkgResultChallengedIterator struct {
	Event *WalletRegistryDkgResultChallenged // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgResultChallengedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgResultChallenged)
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
		it.Event = new(WalletRegistryDkgResultChallenged)
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
func (it *WalletRegistryDkgResultChallengedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgResultChallengedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgResultChallenged represents a DkgResultChallenged event raised by the WalletRegistry contract.
type WalletRegistryDkgResultChallenged struct {
	ResultHash [32]byte
	Challenger common.Address
	Reason     string
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultChallenged is a free log retrieval operation binding the contract event 0x703feb01415a2995816e8d082fd7aad0eacada1a2f63fdb3226e47f8a0285436.
//
// Solidity: event DkgResultChallenged(bytes32 indexed resultHash, address indexed challenger, string reason)
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgResultChallenged(opts *bind.FilterOpts, resultHash [][32]byte, challenger []common.Address) (*WalletRegistryDkgResultChallengedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var challengerRule []interface{}
	for _, challengerItem := range challenger {
		challengerRule = append(challengerRule, challengerItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgResultChallenged", resultHashRule, challengerRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgResultChallengedIterator{contract: _WalletRegistry.contract, event: "DkgResultChallenged", logs: logs, sub: sub}, nil
}

// WatchDkgResultChallenged is a free log subscription operation binding the contract event 0x703feb01415a2995816e8d082fd7aad0eacada1a2f63fdb3226e47f8a0285436.
//
// Solidity: event DkgResultChallenged(bytes32 indexed resultHash, address indexed challenger, string reason)
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgResultChallenged(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgResultChallenged, resultHash [][32]byte, challenger []common.Address) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var challengerRule []interface{}
	for _, challengerItem := range challenger {
		challengerRule = append(challengerRule, challengerItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgResultChallenged", resultHashRule, challengerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgResultChallenged)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgResultChallenged", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgResultChallenged(log types.Log) (*WalletRegistryDkgResultChallenged, error) {
	event := new(WalletRegistryDkgResultChallenged)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgResultChallenged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgResultSubmittedIterator is returned from FilterDkgResultSubmitted and is used to iterate over the raw logs and unpacked data for DkgResultSubmitted events raised by the WalletRegistry contract.
type WalletRegistryDkgResultSubmittedIterator struct {
	Event *WalletRegistryDkgResultSubmitted // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgResultSubmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgResultSubmitted)
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
		it.Event = new(WalletRegistryDkgResultSubmitted)
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
func (it *WalletRegistryDkgResultSubmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgResultSubmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgResultSubmitted represents a DkgResultSubmitted event raised by the WalletRegistry contract.
type WalletRegistryDkgResultSubmitted struct {
	ResultHash [32]byte
	Seed       *big.Int
	Result     EcdsaDkgResult
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultSubmitted is a free log retrieval operation binding the contract event 0x8e7fd4293d7db11807147d8890c287fad3396fbb09a4e92273fc7856076c153a.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgResultSubmitted(opts *bind.FilterOpts, resultHash [][32]byte, seed []*big.Int) (*WalletRegistryDkgResultSubmittedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgResultSubmitted", resultHashRule, seedRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgResultSubmittedIterator{contract: _WalletRegistry.contract, event: "DkgResultSubmitted", logs: logs, sub: sub}, nil
}

// WatchDkgResultSubmitted is a free log subscription operation binding the contract event 0x8e7fd4293d7db11807147d8890c287fad3396fbb09a4e92273fc7856076c153a.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgResultSubmitted(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgResultSubmitted, resultHash [][32]byte, seed []*big.Int) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgResultSubmitted", resultHashRule, seedRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgResultSubmitted)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgResultSubmitted", log); err != nil {
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

// ParseDkgResultSubmitted is a log parse operation binding the contract event 0x8e7fd4293d7db11807147d8890c287fad3396fbb09a4e92273fc7856076c153a.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgResultSubmitted(log types.Log) (*WalletRegistryDkgResultSubmitted, error) {
	event := new(WalletRegistryDkgResultSubmitted)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgResultSubmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgSeedTimedOutIterator is returned from FilterDkgSeedTimedOut and is used to iterate over the raw logs and unpacked data for DkgSeedTimedOut events raised by the WalletRegistry contract.
type WalletRegistryDkgSeedTimedOutIterator struct {
	Event *WalletRegistryDkgSeedTimedOut // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgSeedTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgSeedTimedOut)
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
		it.Event = new(WalletRegistryDkgSeedTimedOut)
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
func (it *WalletRegistryDkgSeedTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgSeedTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgSeedTimedOut represents a DkgSeedTimedOut event raised by the WalletRegistry contract.
type WalletRegistryDkgSeedTimedOut struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgSeedTimedOut is a free log retrieval operation binding the contract event 0x68c52f05452e81639fa06f379aee3178cddee4725521fff886f244c99e868b50.
//
// Solidity: event DkgSeedTimedOut()
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgSeedTimedOut(opts *bind.FilterOpts) (*WalletRegistryDkgSeedTimedOutIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgSeedTimedOut")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgSeedTimedOutIterator{contract: _WalletRegistry.contract, event: "DkgSeedTimedOut", logs: logs, sub: sub}, nil
}

// WatchDkgSeedTimedOut is a free log subscription operation binding the contract event 0x68c52f05452e81639fa06f379aee3178cddee4725521fff886f244c99e868b50.
//
// Solidity: event DkgSeedTimedOut()
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgSeedTimedOut(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgSeedTimedOut) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgSeedTimedOut")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgSeedTimedOut)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgSeedTimedOut", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgSeedTimedOut(log types.Log) (*WalletRegistryDkgSeedTimedOut, error) {
	event := new(WalletRegistryDkgSeedTimedOut)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgSeedTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgStartedIterator is returned from FilterDkgStarted and is used to iterate over the raw logs and unpacked data for DkgStarted events raised by the WalletRegistry contract.
type WalletRegistryDkgStartedIterator struct {
	Event *WalletRegistryDkgStarted // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgStartedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgStarted)
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
		it.Event = new(WalletRegistryDkgStarted)
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
func (it *WalletRegistryDkgStartedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgStartedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgStarted represents a DkgStarted event raised by the WalletRegistry contract.
type WalletRegistryDkgStarted struct {
	Seed *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterDkgStarted is a free log retrieval operation binding the contract event 0xb2ad26c2940889d79df2ee9c758a8aefa00c5ca90eee119af0e5d795df3b98bb.
//
// Solidity: event DkgStarted(uint256 indexed seed)
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgStarted(opts *bind.FilterOpts, seed []*big.Int) (*WalletRegistryDkgStartedIterator, error) {

	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgStarted", seedRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgStartedIterator{contract: _WalletRegistry.contract, event: "DkgStarted", logs: logs, sub: sub}, nil
}

// WatchDkgStarted is a free log subscription operation binding the contract event 0xb2ad26c2940889d79df2ee9c758a8aefa00c5ca90eee119af0e5d795df3b98bb.
//
// Solidity: event DkgStarted(uint256 indexed seed)
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgStarted(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgStarted, seed []*big.Int) (event.Subscription, error) {

	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgStarted", seedRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgStarted)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgStarted", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgStarted(log types.Log) (*WalletRegistryDkgStarted, error) {
	event := new(WalletRegistryDkgStarted)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgStarted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgStateLockedIterator is returned from FilterDkgStateLocked and is used to iterate over the raw logs and unpacked data for DkgStateLocked events raised by the WalletRegistry contract.
type WalletRegistryDkgStateLockedIterator struct {
	Event *WalletRegistryDkgStateLocked // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgStateLockedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgStateLocked)
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
		it.Event = new(WalletRegistryDkgStateLocked)
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
func (it *WalletRegistryDkgStateLockedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgStateLockedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgStateLocked represents a DkgStateLocked event raised by the WalletRegistry contract.
type WalletRegistryDkgStateLocked struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgStateLocked is a free log retrieval operation binding the contract event 0x5c3ed2397d4d21298b2fb5027ac8e2d42e3c9c72bbb55ddb030e2a36a0cdff6b.
//
// Solidity: event DkgStateLocked()
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgStateLocked(opts *bind.FilterOpts) (*WalletRegistryDkgStateLockedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgStateLocked")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgStateLockedIterator{contract: _WalletRegistry.contract, event: "DkgStateLocked", logs: logs, sub: sub}, nil
}

// WatchDkgStateLocked is a free log subscription operation binding the contract event 0x5c3ed2397d4d21298b2fb5027ac8e2d42e3c9c72bbb55ddb030e2a36a0cdff6b.
//
// Solidity: event DkgStateLocked()
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgStateLocked(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgStateLocked) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgStateLocked")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgStateLocked)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgStateLocked", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgStateLocked(log types.Log) (*WalletRegistryDkgStateLocked, error) {
	event := new(WalletRegistryDkgStateLocked)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgStateLocked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryDkgTimedOutIterator is returned from FilterDkgTimedOut and is used to iterate over the raw logs and unpacked data for DkgTimedOut events raised by the WalletRegistry contract.
type WalletRegistryDkgTimedOutIterator struct {
	Event *WalletRegistryDkgTimedOut // Event containing the contract specifics and raw log

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
func (it *WalletRegistryDkgTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryDkgTimedOut)
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
		it.Event = new(WalletRegistryDkgTimedOut)
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
func (it *WalletRegistryDkgTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryDkgTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryDkgTimedOut represents a DkgTimedOut event raised by the WalletRegistry contract.
type WalletRegistryDkgTimedOut struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgTimedOut is a free log retrieval operation binding the contract event 0x2852b3e178dd281713b041c3d90b4815bb55b7ec812931d1e8e8d8bb2ed72d3e.
//
// Solidity: event DkgTimedOut()
func (_WalletRegistry *WalletRegistryFilterer) FilterDkgTimedOut(opts *bind.FilterOpts) (*WalletRegistryDkgTimedOutIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "DkgTimedOut")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryDkgTimedOutIterator{contract: _WalletRegistry.contract, event: "DkgTimedOut", logs: logs, sub: sub}, nil
}

// WatchDkgTimedOut is a free log subscription operation binding the contract event 0x2852b3e178dd281713b041c3d90b4815bb55b7ec812931d1e8e8d8bb2ed72d3e.
//
// Solidity: event DkgTimedOut()
func (_WalletRegistry *WalletRegistryFilterer) WatchDkgTimedOut(opts *bind.WatchOpts, sink chan<- *WalletRegistryDkgTimedOut) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "DkgTimedOut")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryDkgTimedOut)
				if err := _WalletRegistry.contract.UnpackLog(event, "DkgTimedOut", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseDkgTimedOut(log types.Log) (*WalletRegistryDkgTimedOut, error) {
	event := new(WalletRegistryDkgTimedOut)
	if err := _WalletRegistry.contract.UnpackLog(event, "DkgTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryGasParametersUpdatedIterator is returned from FilterGasParametersUpdated and is used to iterate over the raw logs and unpacked data for GasParametersUpdated events raised by the WalletRegistry contract.
type WalletRegistryGasParametersUpdatedIterator struct {
	Event *WalletRegistryGasParametersUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryGasParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryGasParametersUpdated)
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
		it.Event = new(WalletRegistryGasParametersUpdated)
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
func (it *WalletRegistryGasParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryGasParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryGasParametersUpdated represents a GasParametersUpdated event raised by the WalletRegistry contract.
type WalletRegistryGasParametersUpdated struct {
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
func (_WalletRegistry *WalletRegistryFilterer) FilterGasParametersUpdated(opts *bind.FilterOpts) (*WalletRegistryGasParametersUpdatedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "GasParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryGasParametersUpdatedIterator{contract: _WalletRegistry.contract, event: "GasParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchGasParametersUpdated is a free log subscription operation binding the contract event 0x8a3e64fa6013a36bccca7362e8826b11ba41e57fb60f55309c0ca48904dad082.
//
// Solidity: event GasParametersUpdated(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 notifySeedTimeoutGasOffset, uint256 notifyDkgTimeoutNegativeGasOffset)
func (_WalletRegistry *WalletRegistryFilterer) WatchGasParametersUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistryGasParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "GasParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryGasParametersUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "GasParametersUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseGasParametersUpdated(log types.Log) (*WalletRegistryGasParametersUpdated, error) {
	event := new(WalletRegistryGasParametersUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "GasParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryGovernanceTransferredIterator is returned from FilterGovernanceTransferred and is used to iterate over the raw logs and unpacked data for GovernanceTransferred events raised by the WalletRegistry contract.
type WalletRegistryGovernanceTransferredIterator struct {
	Event *WalletRegistryGovernanceTransferred // Event containing the contract specifics and raw log

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
func (it *WalletRegistryGovernanceTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryGovernanceTransferred)
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
		it.Event = new(WalletRegistryGovernanceTransferred)
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
func (it *WalletRegistryGovernanceTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryGovernanceTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryGovernanceTransferred represents a GovernanceTransferred event raised by the WalletRegistry contract.
type WalletRegistryGovernanceTransferred struct {
	OldGovernance common.Address
	NewGovernance common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterGovernanceTransferred is a free log retrieval operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_WalletRegistry *WalletRegistryFilterer) FilterGovernanceTransferred(opts *bind.FilterOpts) (*WalletRegistryGovernanceTransferredIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryGovernanceTransferredIterator{contract: _WalletRegistry.contract, event: "GovernanceTransferred", logs: logs, sub: sub}, nil
}

// WatchGovernanceTransferred is a free log subscription operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_WalletRegistry *WalletRegistryFilterer) WatchGovernanceTransferred(opts *bind.WatchOpts, sink chan<- *WalletRegistryGovernanceTransferred) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryGovernanceTransferred)
				if err := _WalletRegistry.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseGovernanceTransferred(log types.Log) (*WalletRegistryGovernanceTransferred, error) {
	event := new(WalletRegistryGovernanceTransferred)
	if err := _WalletRegistry.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryInactivityClaimedIterator is returned from FilterInactivityClaimed and is used to iterate over the raw logs and unpacked data for InactivityClaimed events raised by the WalletRegistry contract.
type WalletRegistryInactivityClaimedIterator struct {
	Event *WalletRegistryInactivityClaimed // Event containing the contract specifics and raw log

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
func (it *WalletRegistryInactivityClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryInactivityClaimed)
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
		it.Event = new(WalletRegistryInactivityClaimed)
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
func (it *WalletRegistryInactivityClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryInactivityClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryInactivityClaimed represents a InactivityClaimed event raised by the WalletRegistry contract.
type WalletRegistryInactivityClaimed struct {
	WalletID [32]byte
	Nonce    *big.Int
	Notifier common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterInactivityClaimed is a free log retrieval operation binding the contract event 0x326e1ff7c130ed708307116f79cf7dbca649503e7082e5e35a19ceeee1523b39.
//
// Solidity: event InactivityClaimed(bytes32 indexed walletID, uint256 nonce, address notifier)
func (_WalletRegistry *WalletRegistryFilterer) FilterInactivityClaimed(opts *bind.FilterOpts, walletID [][32]byte) (*WalletRegistryInactivityClaimedIterator, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "InactivityClaimed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryInactivityClaimedIterator{contract: _WalletRegistry.contract, event: "InactivityClaimed", logs: logs, sub: sub}, nil
}

// WatchInactivityClaimed is a free log subscription operation binding the contract event 0x326e1ff7c130ed708307116f79cf7dbca649503e7082e5e35a19ceeee1523b39.
//
// Solidity: event InactivityClaimed(bytes32 indexed walletID, uint256 nonce, address notifier)
func (_WalletRegistry *WalletRegistryFilterer) WatchInactivityClaimed(opts *bind.WatchOpts, sink chan<- *WalletRegistryInactivityClaimed, walletID [][32]byte) (event.Subscription, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "InactivityClaimed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryInactivityClaimed)
				if err := _WalletRegistry.contract.UnpackLog(event, "InactivityClaimed", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseInactivityClaimed(log types.Log) (*WalletRegistryInactivityClaimed, error) {
	event := new(WalletRegistryInactivityClaimed)
	if err := _WalletRegistry.contract.UnpackLog(event, "InactivityClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the WalletRegistry contract.
type WalletRegistryInitializedIterator struct {
	Event *WalletRegistryInitialized // Event containing the contract specifics and raw log

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
func (it *WalletRegistryInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryInitialized)
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
		it.Event = new(WalletRegistryInitialized)
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
func (it *WalletRegistryInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryInitialized represents a Initialized event raised by the WalletRegistry contract.
type WalletRegistryInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_WalletRegistry *WalletRegistryFilterer) FilterInitialized(opts *bind.FilterOpts) (*WalletRegistryInitializedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryInitializedIterator{contract: _WalletRegistry.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_WalletRegistry *WalletRegistryFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *WalletRegistryInitialized) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryInitialized)
				if err := _WalletRegistry.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseInitialized(log types.Log) (*WalletRegistryInitialized, error) {
	event := new(WalletRegistryInitialized)
	if err := _WalletRegistry.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator is returned from FilterInvoluntaryAuthorizationDecreaseFailed and is used to iterate over the raw logs and unpacked data for InvoluntaryAuthorizationDecreaseFailed events raised by the WalletRegistry contract.
type WalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator struct {
	Event *WalletRegistryInvoluntaryAuthorizationDecreaseFailed // Event containing the contract specifics and raw log

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
func (it *WalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryInvoluntaryAuthorizationDecreaseFailed)
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
		it.Event = new(WalletRegistryInvoluntaryAuthorizationDecreaseFailed)
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
func (it *WalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryInvoluntaryAuthorizationDecreaseFailed represents a InvoluntaryAuthorizationDecreaseFailed event raised by the WalletRegistry contract.
type WalletRegistryInvoluntaryAuthorizationDecreaseFailed struct {
	StakingProvider common.Address
	Operator        common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterInvoluntaryAuthorizationDecreaseFailed is a free log retrieval operation binding the contract event 0x1b09380d63e78fd72c1d79a805a7e2dfadf02b22418e24bebff51376b7df33b0.
//
// Solidity: event InvoluntaryAuthorizationDecreaseFailed(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_WalletRegistry *WalletRegistryFilterer) FilterInvoluntaryAuthorizationDecreaseFailed(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*WalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "InvoluntaryAuthorizationDecreaseFailed", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryInvoluntaryAuthorizationDecreaseFailedIterator{contract: _WalletRegistry.contract, event: "InvoluntaryAuthorizationDecreaseFailed", logs: logs, sub: sub}, nil
}

// WatchInvoluntaryAuthorizationDecreaseFailed is a free log subscription operation binding the contract event 0x1b09380d63e78fd72c1d79a805a7e2dfadf02b22418e24bebff51376b7df33b0.
//
// Solidity: event InvoluntaryAuthorizationDecreaseFailed(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_WalletRegistry *WalletRegistryFilterer) WatchInvoluntaryAuthorizationDecreaseFailed(opts *bind.WatchOpts, sink chan<- *WalletRegistryInvoluntaryAuthorizationDecreaseFailed, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "InvoluntaryAuthorizationDecreaseFailed", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryInvoluntaryAuthorizationDecreaseFailed)
				if err := _WalletRegistry.contract.UnpackLog(event, "InvoluntaryAuthorizationDecreaseFailed", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseInvoluntaryAuthorizationDecreaseFailed(log types.Log) (*WalletRegistryInvoluntaryAuthorizationDecreaseFailed, error) {
	event := new(WalletRegistryInvoluntaryAuthorizationDecreaseFailed)
	if err := _WalletRegistry.contract.UnpackLog(event, "InvoluntaryAuthorizationDecreaseFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryOperatorJoinedSortitionPoolIterator is returned from FilterOperatorJoinedSortitionPool and is used to iterate over the raw logs and unpacked data for OperatorJoinedSortitionPool events raised by the WalletRegistry contract.
type WalletRegistryOperatorJoinedSortitionPoolIterator struct {
	Event *WalletRegistryOperatorJoinedSortitionPool // Event containing the contract specifics and raw log

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
func (it *WalletRegistryOperatorJoinedSortitionPoolIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryOperatorJoinedSortitionPool)
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
		it.Event = new(WalletRegistryOperatorJoinedSortitionPool)
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
func (it *WalletRegistryOperatorJoinedSortitionPoolIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryOperatorJoinedSortitionPoolIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryOperatorJoinedSortitionPool represents a OperatorJoinedSortitionPool event raised by the WalletRegistry contract.
type WalletRegistryOperatorJoinedSortitionPool struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorJoinedSortitionPool is a free log retrieval operation binding the contract event 0x5075aaa89894a888eb2cac81a27320c60855febb0cf1706b66bdc754e640d433.
//
// Solidity: event OperatorJoinedSortitionPool(address indexed stakingProvider, address indexed operator)
func (_WalletRegistry *WalletRegistryFilterer) FilterOperatorJoinedSortitionPool(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*WalletRegistryOperatorJoinedSortitionPoolIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "OperatorJoinedSortitionPool", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryOperatorJoinedSortitionPoolIterator{contract: _WalletRegistry.contract, event: "OperatorJoinedSortitionPool", logs: logs, sub: sub}, nil
}

// WatchOperatorJoinedSortitionPool is a free log subscription operation binding the contract event 0x5075aaa89894a888eb2cac81a27320c60855febb0cf1706b66bdc754e640d433.
//
// Solidity: event OperatorJoinedSortitionPool(address indexed stakingProvider, address indexed operator)
func (_WalletRegistry *WalletRegistryFilterer) WatchOperatorJoinedSortitionPool(opts *bind.WatchOpts, sink chan<- *WalletRegistryOperatorJoinedSortitionPool, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "OperatorJoinedSortitionPool", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryOperatorJoinedSortitionPool)
				if err := _WalletRegistry.contract.UnpackLog(event, "OperatorJoinedSortitionPool", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseOperatorJoinedSortitionPool(log types.Log) (*WalletRegistryOperatorJoinedSortitionPool, error) {
	event := new(WalletRegistryOperatorJoinedSortitionPool)
	if err := _WalletRegistry.contract.UnpackLog(event, "OperatorJoinedSortitionPool", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryOperatorRegisteredIterator is returned from FilterOperatorRegistered and is used to iterate over the raw logs and unpacked data for OperatorRegistered events raised by the WalletRegistry contract.
type WalletRegistryOperatorRegisteredIterator struct {
	Event *WalletRegistryOperatorRegistered // Event containing the contract specifics and raw log

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
func (it *WalletRegistryOperatorRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryOperatorRegistered)
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
		it.Event = new(WalletRegistryOperatorRegistered)
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
func (it *WalletRegistryOperatorRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryOperatorRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryOperatorRegistered represents a OperatorRegistered event raised by the WalletRegistry contract.
type WalletRegistryOperatorRegistered struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorRegistered is a free log retrieval operation binding the contract event 0xa453db612af59e5521d6ab9284dc3e2d06af286eb1b1b7b771fce4716c19f2c1.
//
// Solidity: event OperatorRegistered(address indexed stakingProvider, address indexed operator)
func (_WalletRegistry *WalletRegistryFilterer) FilterOperatorRegistered(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*WalletRegistryOperatorRegisteredIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "OperatorRegistered", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryOperatorRegisteredIterator{contract: _WalletRegistry.contract, event: "OperatorRegistered", logs: logs, sub: sub}, nil
}

// WatchOperatorRegistered is a free log subscription operation binding the contract event 0xa453db612af59e5521d6ab9284dc3e2d06af286eb1b1b7b771fce4716c19f2c1.
//
// Solidity: event OperatorRegistered(address indexed stakingProvider, address indexed operator)
func (_WalletRegistry *WalletRegistryFilterer) WatchOperatorRegistered(opts *bind.WatchOpts, sink chan<- *WalletRegistryOperatorRegistered, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "OperatorRegistered", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryOperatorRegistered)
				if err := _WalletRegistry.contract.UnpackLog(event, "OperatorRegistered", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseOperatorRegistered(log types.Log) (*WalletRegistryOperatorRegistered, error) {
	event := new(WalletRegistryOperatorRegistered)
	if err := _WalletRegistry.contract.UnpackLog(event, "OperatorRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryOperatorStatusUpdatedIterator is returned from FilterOperatorStatusUpdated and is used to iterate over the raw logs and unpacked data for OperatorStatusUpdated events raised by the WalletRegistry contract.
type WalletRegistryOperatorStatusUpdatedIterator struct {
	Event *WalletRegistryOperatorStatusUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryOperatorStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryOperatorStatusUpdated)
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
		it.Event = new(WalletRegistryOperatorStatusUpdated)
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
func (it *WalletRegistryOperatorStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryOperatorStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryOperatorStatusUpdated represents a OperatorStatusUpdated event raised by the WalletRegistry contract.
type WalletRegistryOperatorStatusUpdated struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorStatusUpdated is a free log retrieval operation binding the contract event 0x1231fe5ee649a593b524a494cd53146a196380a872115a0d0fe16c0735afdf26.
//
// Solidity: event OperatorStatusUpdated(address indexed stakingProvider, address indexed operator)
func (_WalletRegistry *WalletRegistryFilterer) FilterOperatorStatusUpdated(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*WalletRegistryOperatorStatusUpdatedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "OperatorStatusUpdated", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryOperatorStatusUpdatedIterator{contract: _WalletRegistry.contract, event: "OperatorStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchOperatorStatusUpdated is a free log subscription operation binding the contract event 0x1231fe5ee649a593b524a494cd53146a196380a872115a0d0fe16c0735afdf26.
//
// Solidity: event OperatorStatusUpdated(address indexed stakingProvider, address indexed operator)
func (_WalletRegistry *WalletRegistryFilterer) WatchOperatorStatusUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistryOperatorStatusUpdated, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "OperatorStatusUpdated", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryOperatorStatusUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "OperatorStatusUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseOperatorStatusUpdated(log types.Log) (*WalletRegistryOperatorStatusUpdated, error) {
	event := new(WalletRegistryOperatorStatusUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "OperatorStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryRandomBeaconUpgradedIterator is returned from FilterRandomBeaconUpgraded and is used to iterate over the raw logs and unpacked data for RandomBeaconUpgraded events raised by the WalletRegistry contract.
type WalletRegistryRandomBeaconUpgradedIterator struct {
	Event *WalletRegistryRandomBeaconUpgraded // Event containing the contract specifics and raw log

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
func (it *WalletRegistryRandomBeaconUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryRandomBeaconUpgraded)
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
		it.Event = new(WalletRegistryRandomBeaconUpgraded)
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
func (it *WalletRegistryRandomBeaconUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryRandomBeaconUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryRandomBeaconUpgraded represents a RandomBeaconUpgraded event raised by the WalletRegistry contract.
type WalletRegistryRandomBeaconUpgraded struct {
	RandomBeacon common.Address
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRandomBeaconUpgraded is a free log retrieval operation binding the contract event 0x2b34e21b6daa8fcf8cba1c3ed709cbed2b0231d5fb60e9ccd8c2e75a5674bcb3.
//
// Solidity: event RandomBeaconUpgraded(address randomBeacon)
func (_WalletRegistry *WalletRegistryFilterer) FilterRandomBeaconUpgraded(opts *bind.FilterOpts) (*WalletRegistryRandomBeaconUpgradedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "RandomBeaconUpgraded")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryRandomBeaconUpgradedIterator{contract: _WalletRegistry.contract, event: "RandomBeaconUpgraded", logs: logs, sub: sub}, nil
}

// WatchRandomBeaconUpgraded is a free log subscription operation binding the contract event 0x2b34e21b6daa8fcf8cba1c3ed709cbed2b0231d5fb60e9ccd8c2e75a5674bcb3.
//
// Solidity: event RandomBeaconUpgraded(address randomBeacon)
func (_WalletRegistry *WalletRegistryFilterer) WatchRandomBeaconUpgraded(opts *bind.WatchOpts, sink chan<- *WalletRegistryRandomBeaconUpgraded) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "RandomBeaconUpgraded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryRandomBeaconUpgraded)
				if err := _WalletRegistry.contract.UnpackLog(event, "RandomBeaconUpgraded", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseRandomBeaconUpgraded(log types.Log) (*WalletRegistryRandomBeaconUpgraded, error) {
	event := new(WalletRegistryRandomBeaconUpgraded)
	if err := _WalletRegistry.contract.UnpackLog(event, "RandomBeaconUpgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryReimbursementPoolUpdatedIterator is returned from FilterReimbursementPoolUpdated and is used to iterate over the raw logs and unpacked data for ReimbursementPoolUpdated events raised by the WalletRegistry contract.
type WalletRegistryReimbursementPoolUpdatedIterator struct {
	Event *WalletRegistryReimbursementPoolUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryReimbursementPoolUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryReimbursementPoolUpdated)
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
		it.Event = new(WalletRegistryReimbursementPoolUpdated)
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
func (it *WalletRegistryReimbursementPoolUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryReimbursementPoolUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryReimbursementPoolUpdated represents a ReimbursementPoolUpdated event raised by the WalletRegistry contract.
type WalletRegistryReimbursementPoolUpdated struct {
	NewReimbursementPool common.Address
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterReimbursementPoolUpdated is a free log retrieval operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_WalletRegistry *WalletRegistryFilterer) FilterReimbursementPoolUpdated(opts *bind.FilterOpts) (*WalletRegistryReimbursementPoolUpdatedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryReimbursementPoolUpdatedIterator{contract: _WalletRegistry.contract, event: "ReimbursementPoolUpdated", logs: logs, sub: sub}, nil
}

// WatchReimbursementPoolUpdated is a free log subscription operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_WalletRegistry *WalletRegistryFilterer) WatchReimbursementPoolUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistryReimbursementPoolUpdated) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryReimbursementPoolUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseReimbursementPoolUpdated(log types.Log) (*WalletRegistryReimbursementPoolUpdated, error) {
	event := new(WalletRegistryReimbursementPoolUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryRewardParametersUpdatedIterator is returned from FilterRewardParametersUpdated and is used to iterate over the raw logs and unpacked data for RewardParametersUpdated events raised by the WalletRegistry contract.
type WalletRegistryRewardParametersUpdatedIterator struct {
	Event *WalletRegistryRewardParametersUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryRewardParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryRewardParametersUpdated)
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
		it.Event = new(WalletRegistryRewardParametersUpdated)
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
func (it *WalletRegistryRewardParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryRewardParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryRewardParametersUpdated represents a RewardParametersUpdated event raised by the WalletRegistry contract.
type WalletRegistryRewardParametersUpdated struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
	Raw                                            types.Log // Blockchain specific contextual infos
}

// FilterRewardParametersUpdated is a free log retrieval operation binding the contract event 0xf3a6ee10a78fb7d212e87d9be970fb16bd7324e9dc9c38d21cd7ecde781a1d2a.
//
// Solidity: event RewardParametersUpdated(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_WalletRegistry *WalletRegistryFilterer) FilterRewardParametersUpdated(opts *bind.FilterOpts) (*WalletRegistryRewardParametersUpdatedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "RewardParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryRewardParametersUpdatedIterator{contract: _WalletRegistry.contract, event: "RewardParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchRewardParametersUpdated is a free log subscription operation binding the contract event 0xf3a6ee10a78fb7d212e87d9be970fb16bd7324e9dc9c38d21cd7ecde781a1d2a.
//
// Solidity: event RewardParametersUpdated(uint256 maliciousDkgResultNotificationRewardMultiplier, uint256 sortitionPoolRewardsBanDuration)
func (_WalletRegistry *WalletRegistryFilterer) WatchRewardParametersUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistryRewardParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "RewardParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryRewardParametersUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "RewardParametersUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseRewardParametersUpdated(log types.Log) (*WalletRegistryRewardParametersUpdated, error) {
	event := new(WalletRegistryRewardParametersUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "RewardParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryRewardsWithdrawnIterator is returned from FilterRewardsWithdrawn and is used to iterate over the raw logs and unpacked data for RewardsWithdrawn events raised by the WalletRegistry contract.
type WalletRegistryRewardsWithdrawnIterator struct {
	Event *WalletRegistryRewardsWithdrawn // Event containing the contract specifics and raw log

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
func (it *WalletRegistryRewardsWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryRewardsWithdrawn)
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
		it.Event = new(WalletRegistryRewardsWithdrawn)
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
func (it *WalletRegistryRewardsWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryRewardsWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryRewardsWithdrawn represents a RewardsWithdrawn event raised by the WalletRegistry contract.
type WalletRegistryRewardsWithdrawn struct {
	StakingProvider common.Address
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterRewardsWithdrawn is a free log retrieval operation binding the contract event 0x38532b6dea69d7266fa923c7813d190be37625f2454ddfa3d93c45c79482e3fd.
//
// Solidity: event RewardsWithdrawn(address indexed stakingProvider, uint96 amount)
func (_WalletRegistry *WalletRegistryFilterer) FilterRewardsWithdrawn(opts *bind.FilterOpts, stakingProvider []common.Address) (*WalletRegistryRewardsWithdrawnIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "RewardsWithdrawn", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryRewardsWithdrawnIterator{contract: _WalletRegistry.contract, event: "RewardsWithdrawn", logs: logs, sub: sub}, nil
}

// WatchRewardsWithdrawn is a free log subscription operation binding the contract event 0x38532b6dea69d7266fa923c7813d190be37625f2454ddfa3d93c45c79482e3fd.
//
// Solidity: event RewardsWithdrawn(address indexed stakingProvider, uint96 amount)
func (_WalletRegistry *WalletRegistryFilterer) WatchRewardsWithdrawn(opts *bind.WatchOpts, sink chan<- *WalletRegistryRewardsWithdrawn, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "RewardsWithdrawn", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryRewardsWithdrawn)
				if err := _WalletRegistry.contract.UnpackLog(event, "RewardsWithdrawn", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseRewardsWithdrawn(log types.Log) (*WalletRegistryRewardsWithdrawn, error) {
	event := new(WalletRegistryRewardsWithdrawn)
	if err := _WalletRegistry.contract.UnpackLog(event, "RewardsWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistrySlashingParametersUpdatedIterator is returned from FilterSlashingParametersUpdated and is used to iterate over the raw logs and unpacked data for SlashingParametersUpdated events raised by the WalletRegistry contract.
type WalletRegistrySlashingParametersUpdatedIterator struct {
	Event *WalletRegistrySlashingParametersUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistrySlashingParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistrySlashingParametersUpdated)
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
		it.Event = new(WalletRegistrySlashingParametersUpdated)
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
func (it *WalletRegistrySlashingParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistrySlashingParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistrySlashingParametersUpdated represents a SlashingParametersUpdated event raised by the WalletRegistry contract.
type WalletRegistrySlashingParametersUpdated struct {
	MaliciousDkgResultSlashingAmount *big.Int
	Raw                              types.Log // Blockchain specific contextual infos
}

// FilterSlashingParametersUpdated is a free log retrieval operation binding the contract event 0xe132b87eb6644ee4d4c3c32744f7e1c3906335a2d4f99330767bf573909c7d84.
//
// Solidity: event SlashingParametersUpdated(uint256 maliciousDkgResultSlashingAmount)
func (_WalletRegistry *WalletRegistryFilterer) FilterSlashingParametersUpdated(opts *bind.FilterOpts) (*WalletRegistrySlashingParametersUpdatedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "SlashingParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &WalletRegistrySlashingParametersUpdatedIterator{contract: _WalletRegistry.contract, event: "SlashingParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchSlashingParametersUpdated is a free log subscription operation binding the contract event 0xe132b87eb6644ee4d4c3c32744f7e1c3906335a2d4f99330767bf573909c7d84.
//
// Solidity: event SlashingParametersUpdated(uint256 maliciousDkgResultSlashingAmount)
func (_WalletRegistry *WalletRegistryFilterer) WatchSlashingParametersUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistrySlashingParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "SlashingParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistrySlashingParametersUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "SlashingParametersUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseSlashingParametersUpdated(log types.Log) (*WalletRegistrySlashingParametersUpdated, error) {
	event := new(WalletRegistrySlashingParametersUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "SlashingParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryWalletClosedIterator is returned from FilterWalletClosed and is used to iterate over the raw logs and unpacked data for WalletClosed events raised by the WalletRegistry contract.
type WalletRegistryWalletClosedIterator struct {
	Event *WalletRegistryWalletClosed // Event containing the contract specifics and raw log

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
func (it *WalletRegistryWalletClosedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryWalletClosed)
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
		it.Event = new(WalletRegistryWalletClosed)
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
func (it *WalletRegistryWalletClosedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryWalletClosedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryWalletClosed represents a WalletClosed event raised by the WalletRegistry contract.
type WalletRegistryWalletClosed struct {
	WalletID [32]byte
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterWalletClosed is a free log retrieval operation binding the contract event 0xa6ae4af610b8ada39d3675190ead27a5552631a8e33f53e4e37dbb082f11a73e.
//
// Solidity: event WalletClosed(bytes32 indexed walletID)
func (_WalletRegistry *WalletRegistryFilterer) FilterWalletClosed(opts *bind.FilterOpts, walletID [][32]byte) (*WalletRegistryWalletClosedIterator, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "WalletClosed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryWalletClosedIterator{contract: _WalletRegistry.contract, event: "WalletClosed", logs: logs, sub: sub}, nil
}

// WatchWalletClosed is a free log subscription operation binding the contract event 0xa6ae4af610b8ada39d3675190ead27a5552631a8e33f53e4e37dbb082f11a73e.
//
// Solidity: event WalletClosed(bytes32 indexed walletID)
func (_WalletRegistry *WalletRegistryFilterer) WatchWalletClosed(opts *bind.WatchOpts, sink chan<- *WalletRegistryWalletClosed, walletID [][32]byte) (event.Subscription, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "WalletClosed", walletIDRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryWalletClosed)
				if err := _WalletRegistry.contract.UnpackLog(event, "WalletClosed", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseWalletClosed(log types.Log) (*WalletRegistryWalletClosed, error) {
	event := new(WalletRegistryWalletClosed)
	if err := _WalletRegistry.contract.UnpackLog(event, "WalletClosed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryWalletCreatedIterator is returned from FilterWalletCreated and is used to iterate over the raw logs and unpacked data for WalletCreated events raised by the WalletRegistry contract.
type WalletRegistryWalletCreatedIterator struct {
	Event *WalletRegistryWalletCreated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryWalletCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryWalletCreated)
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
		it.Event = new(WalletRegistryWalletCreated)
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
func (it *WalletRegistryWalletCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryWalletCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryWalletCreated represents a WalletCreated event raised by the WalletRegistry contract.
type WalletRegistryWalletCreated struct {
	WalletID      [32]byte
	DkgResultHash [32]byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterWalletCreated is a free log retrieval operation binding the contract event 0xbe8f27cef1f3d94120c9c547c3614f5b992fdb0c0a497cc920fde06546291ab4.
//
// Solidity: event WalletCreated(bytes32 indexed walletID, bytes32 indexed dkgResultHash)
func (_WalletRegistry *WalletRegistryFilterer) FilterWalletCreated(opts *bind.FilterOpts, walletID [][32]byte, dkgResultHash [][32]byte) (*WalletRegistryWalletCreatedIterator, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}
	var dkgResultHashRule []interface{}
	for _, dkgResultHashItem := range dkgResultHash {
		dkgResultHashRule = append(dkgResultHashRule, dkgResultHashItem)
	}

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "WalletCreated", walletIDRule, dkgResultHashRule)
	if err != nil {
		return nil, err
	}
	return &WalletRegistryWalletCreatedIterator{contract: _WalletRegistry.contract, event: "WalletCreated", logs: logs, sub: sub}, nil
}

// WatchWalletCreated is a free log subscription operation binding the contract event 0xbe8f27cef1f3d94120c9c547c3614f5b992fdb0c0a497cc920fde06546291ab4.
//
// Solidity: event WalletCreated(bytes32 indexed walletID, bytes32 indexed dkgResultHash)
func (_WalletRegistry *WalletRegistryFilterer) WatchWalletCreated(opts *bind.WatchOpts, sink chan<- *WalletRegistryWalletCreated, walletID [][32]byte, dkgResultHash [][32]byte) (event.Subscription, error) {

	var walletIDRule []interface{}
	for _, walletIDItem := range walletID {
		walletIDRule = append(walletIDRule, walletIDItem)
	}
	var dkgResultHashRule []interface{}
	for _, dkgResultHashItem := range dkgResultHash {
		dkgResultHashRule = append(dkgResultHashRule, dkgResultHashItem)
	}

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "WalletCreated", walletIDRule, dkgResultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryWalletCreated)
				if err := _WalletRegistry.contract.UnpackLog(event, "WalletCreated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseWalletCreated(log types.Log) (*WalletRegistryWalletCreated, error) {
	event := new(WalletRegistryWalletCreated)
	if err := _WalletRegistry.contract.UnpackLog(event, "WalletCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// WalletRegistryWalletOwnerUpdatedIterator is returned from FilterWalletOwnerUpdated and is used to iterate over the raw logs and unpacked data for WalletOwnerUpdated events raised by the WalletRegistry contract.
type WalletRegistryWalletOwnerUpdatedIterator struct {
	Event *WalletRegistryWalletOwnerUpdated // Event containing the contract specifics and raw log

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
func (it *WalletRegistryWalletOwnerUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(WalletRegistryWalletOwnerUpdated)
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
		it.Event = new(WalletRegistryWalletOwnerUpdated)
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
func (it *WalletRegistryWalletOwnerUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *WalletRegistryWalletOwnerUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// WalletRegistryWalletOwnerUpdated represents a WalletOwnerUpdated event raised by the WalletRegistry contract.
type WalletRegistryWalletOwnerUpdated struct {
	WalletOwner common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterWalletOwnerUpdated is a free log retrieval operation binding the contract event 0xa1993af5a189ba5ad4155263c920cfee33ce0593a8eb231a13bb3ce6f39459e3.
//
// Solidity: event WalletOwnerUpdated(address walletOwner)
func (_WalletRegistry *WalletRegistryFilterer) FilterWalletOwnerUpdated(opts *bind.FilterOpts) (*WalletRegistryWalletOwnerUpdatedIterator, error) {

	logs, sub, err := _WalletRegistry.contract.FilterLogs(opts, "WalletOwnerUpdated")
	if err != nil {
		return nil, err
	}
	return &WalletRegistryWalletOwnerUpdatedIterator{contract: _WalletRegistry.contract, event: "WalletOwnerUpdated", logs: logs, sub: sub}, nil
}

// WatchWalletOwnerUpdated is a free log subscription operation binding the contract event 0xa1993af5a189ba5ad4155263c920cfee33ce0593a8eb231a13bb3ce6f39459e3.
//
// Solidity: event WalletOwnerUpdated(address walletOwner)
func (_WalletRegistry *WalletRegistryFilterer) WatchWalletOwnerUpdated(opts *bind.WatchOpts, sink chan<- *WalletRegistryWalletOwnerUpdated) (event.Subscription, error) {

	logs, sub, err := _WalletRegistry.contract.WatchLogs(opts, "WalletOwnerUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(WalletRegistryWalletOwnerUpdated)
				if err := _WalletRegistry.contract.UnpackLog(event, "WalletOwnerUpdated", log); err != nil {
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
func (_WalletRegistry *WalletRegistryFilterer) ParseWalletOwnerUpdated(log types.Log) (*WalletRegistryWalletOwnerUpdated, error) {
	event := new(WalletRegistryWalletOwnerUpdated)
	if err := _WalletRegistry.contract.UnpackLog(event, "WalletOwnerUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

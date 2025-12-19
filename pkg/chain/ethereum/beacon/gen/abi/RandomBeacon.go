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

// BeaconDkgResult is an auto generated low-level Go binding around an user-defined struct.
type BeaconDkgResult struct {
	SubmitterMemberIndex     *big.Int
	GroupPubKey              []byte
	MisbehavedMembersIndices []uint8
	Signatures               []byte
	SigningMembersIndices    []*big.Int
	Members                  []uint32
	MembersHash              [32]byte
}

// BeaconInactivityClaim is an auto generated low-level Go binding around an user-defined struct.
type BeaconInactivityClaim struct {
	GroupId                uint64
	InactiveMembersIndices []*big.Int
	Signatures             []byte
	SigningMembersIndices  []*big.Int
}

// GroupsGroup is an auto generated low-level Go binding around an user-defined struct.
type GroupsGroup struct {
	GroupPubKey             []byte
	RegistrationBlockNumber *big.Int
	MembersHash             [32]byte
	Terminated              bool
}

// RandomBeaconMetaData contains all meta data concerning the RandomBeacon contract.
var RandomBeaconMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"_sortitionPool\",\"type\":\"address\"},{\"internalType\":\"contractIERC20\",\"name\":\"_tToken\",\"type\":\"address\"},{\"internalType\":\"contractIStaking\",\"name\":\"_staking\",\"type\":\"address\"},{\"internalType\":\"contractBeaconDkgValidator\",\"name\":\"_dkgValidator\",\"type\":\"address\"},{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"AuthorizationDecreaseApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"decreasingAt\",\"type\":\"uint64\"}],\"name\":\"AuthorizationDecreaseRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"AuthorizationIncreased\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"minimumAuthorization\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"name\":\"AuthorizationParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"entry\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"entrySubmittedBlock\",\"type\":\"uint256\"}],\"name\":\"CallbackFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"maliciousSubmitter\",\"type\":\"address\"}],\"name\":\"DkgMaliciousResultSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"maliciousSubmitter\",\"type\":\"address\"}],\"name\":\"DkgMaliciousResultSlashingFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"approver\",\"type\":\"address\"}],\"name\":\"DkgResultApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"challenger\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"reason\",\"type\":\"string\"}],\"name\":\"DkgResultChallenged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"indexed\":false,\"internalType\":\"structBeaconDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"}],\"name\":\"DkgResultSubmitted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgSeedTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"}],\"name\":\"DkgStarted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgStateLocked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"DkgTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"relayEntrySubmissionGasOffset\",\"type\":\"uint256\"}],\"name\":\"GasParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldGovernance\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"GovernanceTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"groupCreationFrequency\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"groupLifetime\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultChallengePeriodLength\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultChallengeExtraGas\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionTimeout\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgResultSubmitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"name\":\"GroupCreationParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"},{\"indexed\":true,\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"}],\"name\":\"GroupRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"notifier\",\"type\":\"address\"}],\"name\":\"InactivityClaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"InvoluntaryAuthorizationDecreaseFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorJoinedSortitionPool\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"OperatorStatusUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newReimbursementPool\",\"type\":\"address\"}],\"name\":\"ReimbursementPoolUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"groupMembers\",\"type\":\"address[]\"}],\"name\":\"RelayEntryDelaySlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"groupMembers\",\"type\":\"address[]\"}],\"name\":\"RelayEntryDelaySlashingFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"relayEntrySoftTimeout\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"relayEntryHardTimeout\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"callbackGasLimit\",\"type\":\"uint256\"}],\"name\":\"RelayEntryParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"previousEntry\",\"type\":\"bytes\"}],\"name\":\"RelayEntryRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"entry\",\"type\":\"bytes\"}],\"name\":\"RelayEntrySubmitted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"terminatedGroupId\",\"type\":\"uint64\"}],\"name\":\"RelayEntryTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"groupMembers\",\"type\":\"address[]\"}],\"name\":\"RelayEntryTimeoutSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"slashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"groupMembers\",\"type\":\"address[]\"}],\"name\":\"RelayEntryTimeoutSlashingFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"requester\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"isAuthorized\",\"type\":\"bool\"}],\"name\":\"RequesterAuthorizationUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"relayEntryTimeoutNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unauthorizedSigningNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"dkgMaliciousResultNotificationRewardMultiplier\",\"type\":\"uint256\"}],\"name\":\"RewardParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"RewardsWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"relayEntrySubmissionFailureSlashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unauthorizedSigningSlashingAmount\",\"type\":\"uint256\"}],\"name\":\"SlashingParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unauthorizedSigningSlashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"groupMembers\",\"type\":\"address[]\"}],\"name\":\"UnauthorizedSigningSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unauthorizedSigningSlashingAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"groupMembers\",\"type\":\"address[]\"}],\"name\":\"UnauthorizedSigningSlashingFailed\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"approveAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structBeaconDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"approveDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"authorizationDecreaseRequested\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"authorizationIncreased\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"authorizationParameters\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"minimumAuthorization\",\"type\":\"uint96\"},{\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"authorizedRequesters\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"availableRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structBeaconDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"challengeDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"eligibleStake\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"gasParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"relayEntrySubmissionGasOffset\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"genesis\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"}],\"name\":\"getGroup\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"registrationBlockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"},{\"internalType\":\"bool\",\"name\":\"terminated\",\"type\":\"bool\"}],\"internalType\":\"structGroups.Group\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"}],\"name\":\"getGroup\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"registrationBlockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"},{\"internalType\":\"bool\",\"name\":\"terminated\",\"type\":\"bool\"}],\"internalType\":\"structGroups.Group\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getGroupCreationState\",\"outputs\":[{\"internalType\":\"enumBeaconDkg.State\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getGroupsRegistry\",\"outputs\":[{\"internalType\":\"bytes32[]\",\"name\":\"\",\"type\":\"bytes32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"governance\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"groupCreationParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"groupCreationFrequency\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"groupLifetime\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultChallengePeriodLength\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultChallengeExtraGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgSubmitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"hasDkgTimedOut\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"name\":\"inactivityClaimNonce\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"involuntaryAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorInPool\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorUpToDate\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"isRelayRequestInProgress\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"joinSortitionPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minimumAuthorization\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifyDkgTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"},{\"internalType\":\"uint256[]\",\"name\":\"inactiveMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"}],\"internalType\":\"structBeaconInactivity.Claim\",\"name\":\"claim\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"},{\"internalType\":\"uint32[]\",\"name\":\"groupMembers\",\"type\":\"uint32[]\"}],\"name\":\"notifyOperatorInactivity\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"operatorToStakingProvider\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"pendingAuthorizationDecrease\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"registerOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reimbursementPool\",\"outputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"relayEntryParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"relayEntrySoftTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"relayEntryHardTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"callbackGasLimit\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"remainingAuthorizationDecreaseDelay\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32[]\",\"name\":\"groupMembers\",\"type\":\"uint32[]\"}],\"name\":\"reportRelayEntryTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"signedMsgSender\",\"type\":\"bytes\"},{\"internalType\":\"uint64\",\"name\":\"groupId\",\"type\":\"uint64\"},{\"internalType\":\"uint32[]\",\"name\":\"groupMembers\",\"type\":\"uint32[]\"}],\"name\":\"reportUnauthorizedSigning\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIRandomBeaconConsumer\",\"name\":\"callbackContract\",\"type\":\"address\"}],\"name\":\"requestRelayEntry\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rewardParameters\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"relayEntryTimeoutNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"unauthorizedSigningNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgMaliciousResultNotificationRewardMultiplier\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"selectGroup\",\"outputs\":[{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"requester\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"isAuthorized\",\"type\":\"bool\"}],\"name\":\"setRequesterAuthorization\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"slashingParameters\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"relayEntrySubmissionFailureSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"unauthorizedSigningSlashingAmount\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"sortitionPool\",\"outputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"staking\",\"outputs\":[{\"internalType\":\"contractIStaking\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"stakingProviderToOperator\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"groupPubKey\",\"type\":\"bytes\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structBeaconDkg.Result\",\"name\":\"dkgResult\",\"type\":\"tuple\"}],\"name\":\"submitDkgResult\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"entry\",\"type\":\"bytes\"},{\"internalType\":\"uint32[]\",\"name\":\"groupMembers\",\"type\":\"uint32[]\"}],\"name\":\"submitRelayEntry\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"entry\",\"type\":\"bytes\"}],\"name\":\"submitRelayEntry\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tToken\",\"outputs\":[{\"internalType\":\"contractIERC20\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"transferGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"_minimumAuthorization\",\"type\":\"uint96\"},{\"internalType\":\"uint64\",\"name\":\"_authorizationDecreaseDelay\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"_authorizationDecreaseChangePeriod\",\"type\":\"uint64\"}],\"name\":\"updateAuthorizationParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultApprovalGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"notifyOperatorInactivityGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"relayEntrySubmissionGasOffset\",\"type\":\"uint256\"}],\"name\":\"updateGasParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"groupCreationFrequency\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"groupLifetime\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultChallengePeriodLength\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultChallengeExtraGas\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgResultSubmissionTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgSubmitterPrecedencePeriodLength\",\"type\":\"uint256\"}],\"name\":\"updateGroupCreationParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"updateOperatorStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"name\":\"updateReimbursementPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"relayEntrySoftTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"relayEntryHardTimeout\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"callbackGasLimit\",\"type\":\"uint256\"}],\"name\":\"updateRelayEntryParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"sortitionPoolRewardsBanDuration\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"relayEntryTimeoutNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"unauthorizedSigningNotificationRewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"dkgMaliciousResultNotificationRewardMultiplier\",\"type\":\"uint256\"}],\"name\":\"updateRewardParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"relayEntrySubmissionFailureSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"maliciousDkgResultSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"unauthorizedSigningSlashingAmount\",\"type\":\"uint96\"}],\"name\":\"updateSlashingParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"withdrawIneligibleRewards\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"withdrawRewards\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// RandomBeaconABI is the input ABI used to generate the binding from.
// Deprecated: Use RandomBeaconMetaData.ABI instead.
var RandomBeaconABI = RandomBeaconMetaData.ABI

// RandomBeacon is an auto generated Go binding around an Ethereum contract.
type RandomBeacon struct {
	RandomBeaconCaller     // Read-only binding to the contract
	RandomBeaconTransactor // Write-only binding to the contract
	RandomBeaconFilterer   // Log filterer for contract events
}

// RandomBeaconCaller is an auto generated read-only Go binding around an Ethereum contract.
type RandomBeaconCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RandomBeaconTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RandomBeaconTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RandomBeaconFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RandomBeaconFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RandomBeaconSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RandomBeaconSession struct {
	Contract     *RandomBeacon     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RandomBeaconCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RandomBeaconCallerSession struct {
	Contract *RandomBeaconCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// RandomBeaconTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RandomBeaconTransactorSession struct {
	Contract     *RandomBeaconTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// RandomBeaconRaw is an auto generated low-level Go binding around an Ethereum contract.
type RandomBeaconRaw struct {
	Contract *RandomBeacon // Generic contract binding to access the raw methods on
}

// RandomBeaconCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RandomBeaconCallerRaw struct {
	Contract *RandomBeaconCaller // Generic read-only contract binding to access the raw methods on
}

// RandomBeaconTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RandomBeaconTransactorRaw struct {
	Contract *RandomBeaconTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRandomBeacon creates a new instance of RandomBeacon, bound to a specific deployed contract.
func NewRandomBeacon(address common.Address, backend bind.ContractBackend) (*RandomBeacon, error) {
	contract, err := bindRandomBeacon(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RandomBeacon{RandomBeaconCaller: RandomBeaconCaller{contract: contract}, RandomBeaconTransactor: RandomBeaconTransactor{contract: contract}, RandomBeaconFilterer: RandomBeaconFilterer{contract: contract}}, nil
}

// NewRandomBeaconCaller creates a new read-only instance of RandomBeacon, bound to a specific deployed contract.
func NewRandomBeaconCaller(address common.Address, caller bind.ContractCaller) (*RandomBeaconCaller, error) {
	contract, err := bindRandomBeacon(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconCaller{contract: contract}, nil
}

// NewRandomBeaconTransactor creates a new write-only instance of RandomBeacon, bound to a specific deployed contract.
func NewRandomBeaconTransactor(address common.Address, transactor bind.ContractTransactor) (*RandomBeaconTransactor, error) {
	contract, err := bindRandomBeacon(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconTransactor{contract: contract}, nil
}

// NewRandomBeaconFilterer creates a new log filterer instance of RandomBeacon, bound to a specific deployed contract.
func NewRandomBeaconFilterer(address common.Address, filterer bind.ContractFilterer) (*RandomBeaconFilterer, error) {
	contract, err := bindRandomBeacon(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconFilterer{contract: contract}, nil
}

// bindRandomBeacon binds a generic wrapper to an already deployed contract.
func bindRandomBeacon(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := RandomBeaconMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RandomBeacon *RandomBeaconRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RandomBeacon.Contract.RandomBeaconCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RandomBeacon *RandomBeaconRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RandomBeacon.Contract.RandomBeaconTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RandomBeacon *RandomBeaconRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RandomBeacon.Contract.RandomBeaconTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RandomBeacon *RandomBeaconCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RandomBeacon.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RandomBeacon *RandomBeaconTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RandomBeacon.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RandomBeacon *RandomBeaconTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RandomBeacon.Contract.contract.Transact(opts, method, params...)
}

// AuthorizationParameters is a free data retrieval call binding the contract method 0x7b14729e.
//
// Solidity: function authorizationParameters() view returns(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_RandomBeacon *RandomBeaconCaller) AuthorizationParameters(opts *bind.CallOpts) (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "authorizationParameters")

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
func (_RandomBeacon *RandomBeaconSession) AuthorizationParameters() (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	return _RandomBeacon.Contract.AuthorizationParameters(&_RandomBeacon.CallOpts)
}

// AuthorizationParameters is a free data retrieval call binding the contract method 0x7b14729e.
//
// Solidity: function authorizationParameters() view returns(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_RandomBeacon *RandomBeaconCallerSession) AuthorizationParameters() (struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}, error) {
	return _RandomBeacon.Contract.AuthorizationParameters(&_RandomBeacon.CallOpts)
}

// AuthorizedRequesters is a free data retrieval call binding the contract method 0x3ea478aa.
//
// Solidity: function authorizedRequesters(address ) view returns(bool)
func (_RandomBeacon *RandomBeaconCaller) AuthorizedRequesters(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "authorizedRequesters", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// AuthorizedRequesters is a free data retrieval call binding the contract method 0x3ea478aa.
//
// Solidity: function authorizedRequesters(address ) view returns(bool)
func (_RandomBeacon *RandomBeaconSession) AuthorizedRequesters(arg0 common.Address) (bool, error) {
	return _RandomBeacon.Contract.AuthorizedRequesters(&_RandomBeacon.CallOpts, arg0)
}

// AuthorizedRequesters is a free data retrieval call binding the contract method 0x3ea478aa.
//
// Solidity: function authorizedRequesters(address ) view returns(bool)
func (_RandomBeacon *RandomBeaconCallerSession) AuthorizedRequesters(arg0 common.Address) (bool, error) {
	return _RandomBeacon.Contract.AuthorizedRequesters(&_RandomBeacon.CallOpts, arg0)
}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconCaller) AvailableRewards(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "availableRewards", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconSession) AvailableRewards(stakingProvider common.Address) (*big.Int, error) {
	return _RandomBeacon.Contract.AvailableRewards(&_RandomBeacon.CallOpts, stakingProvider)
}

// AvailableRewards is a free data retrieval call binding the contract method 0xf854a27f.
//
// Solidity: function availableRewards(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconCallerSession) AvailableRewards(stakingProvider common.Address) (*big.Int, error) {
	return _RandomBeacon.Contract.AvailableRewards(&_RandomBeacon.CallOpts, stakingProvider)
}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconCaller) EligibleStake(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "eligibleStake", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconSession) EligibleStake(stakingProvider common.Address) (*big.Int, error) {
	return _RandomBeacon.Contract.EligibleStake(&_RandomBeacon.CallOpts, stakingProvider)
}

// EligibleStake is a free data retrieval call binding the contract method 0x7e33cba6.
//
// Solidity: function eligibleStake(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconCallerSession) EligibleStake(stakingProvider common.Address) (*big.Int, error) {
	return _RandomBeacon.Contract.EligibleStake(&_RandomBeacon.CallOpts, stakingProvider)
}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset)
func (_RandomBeacon *RandomBeaconCaller) GasParameters(opts *bind.CallOpts) (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	RelayEntrySubmissionGasOffset     *big.Int
}, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "gasParameters")

	outstruct := new(struct {
		DkgResultSubmissionGas            *big.Int
		DkgResultApprovalGasOffset        *big.Int
		NotifyOperatorInactivityGasOffset *big.Int
		RelayEntrySubmissionGasOffset     *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.DkgResultSubmissionGas = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.DkgResultApprovalGasOffset = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.NotifyOperatorInactivityGasOffset = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.RelayEntrySubmissionGasOffset = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset)
func (_RandomBeacon *RandomBeaconSession) GasParameters() (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	RelayEntrySubmissionGasOffset     *big.Int
}, error) {
	return _RandomBeacon.Contract.GasParameters(&_RandomBeacon.CallOpts)
}

// GasParameters is a free data retrieval call binding the contract method 0x88a59590.
//
// Solidity: function gasParameters() view returns(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset)
func (_RandomBeacon *RandomBeaconCallerSession) GasParameters() (struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	RelayEntrySubmissionGasOffset     *big.Int
}, error) {
	return _RandomBeacon.Contract.GasParameters(&_RandomBeacon.CallOpts)
}

// GetGroup is a free data retrieval call binding the contract method 0x319ac101.
//
// Solidity: function getGroup(uint64 groupId) view returns((bytes,uint256,bytes32,bool))
func (_RandomBeacon *RandomBeaconCaller) GetGroup(opts *bind.CallOpts, groupId uint64) (GroupsGroup, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "getGroup", groupId)

	if err != nil {
		return *new(GroupsGroup), err
	}

	out0 := *abi.ConvertType(out[0], new(GroupsGroup)).(*GroupsGroup)

	return out0, err

}

// GetGroup is a free data retrieval call binding the contract method 0x319ac101.
//
// Solidity: function getGroup(uint64 groupId) view returns((bytes,uint256,bytes32,bool))
func (_RandomBeacon *RandomBeaconSession) GetGroup(groupId uint64) (GroupsGroup, error) {
	return _RandomBeacon.Contract.GetGroup(&_RandomBeacon.CallOpts, groupId)
}

// GetGroup is a free data retrieval call binding the contract method 0x319ac101.
//
// Solidity: function getGroup(uint64 groupId) view returns((bytes,uint256,bytes32,bool))
func (_RandomBeacon *RandomBeaconCallerSession) GetGroup(groupId uint64) (GroupsGroup, error) {
	return _RandomBeacon.Contract.GetGroup(&_RandomBeacon.CallOpts, groupId)
}

// GetGroup0 is a free data retrieval call binding the contract method 0x4549cc4b.
//
// Solidity: function getGroup(bytes groupPubKey) view returns((bytes,uint256,bytes32,bool))
func (_RandomBeacon *RandomBeaconCaller) GetGroup0(opts *bind.CallOpts, groupPubKey []byte) (GroupsGroup, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "getGroup0", groupPubKey)

	if err != nil {
		return *new(GroupsGroup), err
	}

	out0 := *abi.ConvertType(out[0], new(GroupsGroup)).(*GroupsGroup)

	return out0, err

}

// GetGroup0 is a free data retrieval call binding the contract method 0x4549cc4b.
//
// Solidity: function getGroup(bytes groupPubKey) view returns((bytes,uint256,bytes32,bool))
func (_RandomBeacon *RandomBeaconSession) GetGroup0(groupPubKey []byte) (GroupsGroup, error) {
	return _RandomBeacon.Contract.GetGroup0(&_RandomBeacon.CallOpts, groupPubKey)
}

// GetGroup0 is a free data retrieval call binding the contract method 0x4549cc4b.
//
// Solidity: function getGroup(bytes groupPubKey) view returns((bytes,uint256,bytes32,bool))
func (_RandomBeacon *RandomBeaconCallerSession) GetGroup0(groupPubKey []byte) (GroupsGroup, error) {
	return _RandomBeacon.Contract.GetGroup0(&_RandomBeacon.CallOpts, groupPubKey)
}

// GetGroupCreationState is a free data retrieval call binding the contract method 0xcb8b3779.
//
// Solidity: function getGroupCreationState() view returns(uint8)
func (_RandomBeacon *RandomBeaconCaller) GetGroupCreationState(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "getGroupCreationState")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetGroupCreationState is a free data retrieval call binding the contract method 0xcb8b3779.
//
// Solidity: function getGroupCreationState() view returns(uint8)
func (_RandomBeacon *RandomBeaconSession) GetGroupCreationState() (uint8, error) {
	return _RandomBeacon.Contract.GetGroupCreationState(&_RandomBeacon.CallOpts)
}

// GetGroupCreationState is a free data retrieval call binding the contract method 0xcb8b3779.
//
// Solidity: function getGroupCreationState() view returns(uint8)
func (_RandomBeacon *RandomBeaconCallerSession) GetGroupCreationState() (uint8, error) {
	return _RandomBeacon.Contract.GetGroupCreationState(&_RandomBeacon.CallOpts)
}

// GetGroupsRegistry is a free data retrieval call binding the contract method 0x1872ea94.
//
// Solidity: function getGroupsRegistry() view returns(bytes32[])
func (_RandomBeacon *RandomBeaconCaller) GetGroupsRegistry(opts *bind.CallOpts) ([][32]byte, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "getGroupsRegistry")

	if err != nil {
		return *new([][32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([][32]byte)).(*[][32]byte)

	return out0, err

}

// GetGroupsRegistry is a free data retrieval call binding the contract method 0x1872ea94.
//
// Solidity: function getGroupsRegistry() view returns(bytes32[])
func (_RandomBeacon *RandomBeaconSession) GetGroupsRegistry() ([][32]byte, error) {
	return _RandomBeacon.Contract.GetGroupsRegistry(&_RandomBeacon.CallOpts)
}

// GetGroupsRegistry is a free data retrieval call binding the contract method 0x1872ea94.
//
// Solidity: function getGroupsRegistry() view returns(bytes32[])
func (_RandomBeacon *RandomBeaconCallerSession) GetGroupsRegistry() ([][32]byte, error) {
	return _RandomBeacon.Contract.GetGroupsRegistry(&_RandomBeacon.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_RandomBeacon *RandomBeaconCaller) Governance(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "governance")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_RandomBeacon *RandomBeaconSession) Governance() (common.Address, error) {
	return _RandomBeacon.Contract.Governance(&_RandomBeacon.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_RandomBeacon *RandomBeaconCallerSession) Governance() (common.Address, error) {
	return _RandomBeacon.Contract.Governance(&_RandomBeacon.CallOpts)
}

// GroupCreationParameters is a free data retrieval call binding the contract method 0xb142f85c.
//
// Solidity: function groupCreationParameters() view returns(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgSubmitterPrecedencePeriodLength)
func (_RandomBeacon *RandomBeaconCaller) GroupCreationParameters(opts *bind.CallOpts) (struct {
	GroupCreationFrequency             *big.Int
	GroupLifetime                      *big.Int
	DkgResultChallengePeriodLength     *big.Int
	DkgResultChallengeExtraGas         *big.Int
	DkgResultSubmissionTimeout         *big.Int
	DkgSubmitterPrecedencePeriodLength *big.Int
}, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "groupCreationParameters")

	outstruct := new(struct {
		GroupCreationFrequency             *big.Int
		GroupLifetime                      *big.Int
		DkgResultChallengePeriodLength     *big.Int
		DkgResultChallengeExtraGas         *big.Int
		DkgResultSubmissionTimeout         *big.Int
		DkgSubmitterPrecedencePeriodLength *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.GroupCreationFrequency = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.GroupLifetime = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.DkgResultChallengePeriodLength = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.DkgResultChallengeExtraGas = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.DkgResultSubmissionTimeout = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.DkgSubmitterPrecedencePeriodLength = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GroupCreationParameters is a free data retrieval call binding the contract method 0xb142f85c.
//
// Solidity: function groupCreationParameters() view returns(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgSubmitterPrecedencePeriodLength)
func (_RandomBeacon *RandomBeaconSession) GroupCreationParameters() (struct {
	GroupCreationFrequency             *big.Int
	GroupLifetime                      *big.Int
	DkgResultChallengePeriodLength     *big.Int
	DkgResultChallengeExtraGas         *big.Int
	DkgResultSubmissionTimeout         *big.Int
	DkgSubmitterPrecedencePeriodLength *big.Int
}, error) {
	return _RandomBeacon.Contract.GroupCreationParameters(&_RandomBeacon.CallOpts)
}

// GroupCreationParameters is a free data retrieval call binding the contract method 0xb142f85c.
//
// Solidity: function groupCreationParameters() view returns(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgSubmitterPrecedencePeriodLength)
func (_RandomBeacon *RandomBeaconCallerSession) GroupCreationParameters() (struct {
	GroupCreationFrequency             *big.Int
	GroupLifetime                      *big.Int
	DkgResultChallengePeriodLength     *big.Int
	DkgResultChallengeExtraGas         *big.Int
	DkgResultSubmissionTimeout         *big.Int
	DkgSubmitterPrecedencePeriodLength *big.Int
}, error) {
	return _RandomBeacon.Contract.GroupCreationParameters(&_RandomBeacon.CallOpts)
}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_RandomBeacon *RandomBeaconCaller) HasDkgTimedOut(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "hasDkgTimedOut")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_RandomBeacon *RandomBeaconSession) HasDkgTimedOut() (bool, error) {
	return _RandomBeacon.Contract.HasDkgTimedOut(&_RandomBeacon.CallOpts)
}

// HasDkgTimedOut is a free data retrieval call binding the contract method 0x68c34948.
//
// Solidity: function hasDkgTimedOut() view returns(bool)
func (_RandomBeacon *RandomBeaconCallerSession) HasDkgTimedOut() (bool, error) {
	return _RandomBeacon.Contract.HasDkgTimedOut(&_RandomBeacon.CallOpts)
}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0xf5ee563f.
//
// Solidity: function inactivityClaimNonce(uint64 ) view returns(uint256)
func (_RandomBeacon *RandomBeaconCaller) InactivityClaimNonce(opts *bind.CallOpts, arg0 uint64) (*big.Int, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "inactivityClaimNonce", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0xf5ee563f.
//
// Solidity: function inactivityClaimNonce(uint64 ) view returns(uint256)
func (_RandomBeacon *RandomBeaconSession) InactivityClaimNonce(arg0 uint64) (*big.Int, error) {
	return _RandomBeacon.Contract.InactivityClaimNonce(&_RandomBeacon.CallOpts, arg0)
}

// InactivityClaimNonce is a free data retrieval call binding the contract method 0xf5ee563f.
//
// Solidity: function inactivityClaimNonce(uint64 ) view returns(uint256)
func (_RandomBeacon *RandomBeaconCallerSession) InactivityClaimNonce(arg0 uint64) (*big.Int, error) {
	return _RandomBeacon.Contract.InactivityClaimNonce(&_RandomBeacon.CallOpts, arg0)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_RandomBeacon *RandomBeaconCaller) IsOperatorInPool(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "isOperatorInPool", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_RandomBeacon *RandomBeaconSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _RandomBeacon.Contract.IsOperatorInPool(&_RandomBeacon.CallOpts, operator)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_RandomBeacon *RandomBeaconCallerSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _RandomBeacon.Contract.IsOperatorInPool(&_RandomBeacon.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_RandomBeacon *RandomBeaconCaller) IsOperatorUpToDate(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "isOperatorUpToDate", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_RandomBeacon *RandomBeaconSession) IsOperatorUpToDate(operator common.Address) (bool, error) {
	return _RandomBeacon.Contract.IsOperatorUpToDate(&_RandomBeacon.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0xe686440f.
//
// Solidity: function isOperatorUpToDate(address operator) view returns(bool)
func (_RandomBeacon *RandomBeaconCallerSession) IsOperatorUpToDate(operator common.Address) (bool, error) {
	return _RandomBeacon.Contract.IsOperatorUpToDate(&_RandomBeacon.CallOpts, operator)
}

// IsRelayRequestInProgress is a free data retrieval call binding the contract method 0x8f105e37.
//
// Solidity: function isRelayRequestInProgress() view returns(bool)
func (_RandomBeacon *RandomBeaconCaller) IsRelayRequestInProgress(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "isRelayRequestInProgress")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsRelayRequestInProgress is a free data retrieval call binding the contract method 0x8f105e37.
//
// Solidity: function isRelayRequestInProgress() view returns(bool)
func (_RandomBeacon *RandomBeaconSession) IsRelayRequestInProgress() (bool, error) {
	return _RandomBeacon.Contract.IsRelayRequestInProgress(&_RandomBeacon.CallOpts)
}

// IsRelayRequestInProgress is a free data retrieval call binding the contract method 0x8f105e37.
//
// Solidity: function isRelayRequestInProgress() view returns(bool)
func (_RandomBeacon *RandomBeaconCallerSession) IsRelayRequestInProgress() (bool, error) {
	return _RandomBeacon.Contract.IsRelayRequestInProgress(&_RandomBeacon.CallOpts)
}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_RandomBeacon *RandomBeaconCaller) MinimumAuthorization(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "minimumAuthorization")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_RandomBeacon *RandomBeaconSession) MinimumAuthorization() (*big.Int, error) {
	return _RandomBeacon.Contract.MinimumAuthorization(&_RandomBeacon.CallOpts)
}

// MinimumAuthorization is a free data retrieval call binding the contract method 0xf0820c92.
//
// Solidity: function minimumAuthorization() view returns(uint96)
func (_RandomBeacon *RandomBeaconCallerSession) MinimumAuthorization() (*big.Int, error) {
	return _RandomBeacon.Contract.MinimumAuthorization(&_RandomBeacon.CallOpts)
}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_RandomBeacon *RandomBeaconCaller) OperatorToStakingProvider(opts *bind.CallOpts, operator common.Address) (common.Address, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "operatorToStakingProvider", operator)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_RandomBeacon *RandomBeaconSession) OperatorToStakingProvider(operator common.Address) (common.Address, error) {
	return _RandomBeacon.Contract.OperatorToStakingProvider(&_RandomBeacon.CallOpts, operator)
}

// OperatorToStakingProvider is a free data retrieval call binding the contract method 0xded56d45.
//
// Solidity: function operatorToStakingProvider(address operator) view returns(address)
func (_RandomBeacon *RandomBeaconCallerSession) OperatorToStakingProvider(operator common.Address) (common.Address, error) {
	return _RandomBeacon.Contract.OperatorToStakingProvider(&_RandomBeacon.CallOpts, operator)
}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconCaller) PendingAuthorizationDecrease(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "pendingAuthorizationDecrease", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconSession) PendingAuthorizationDecrease(stakingProvider common.Address) (*big.Int, error) {
	return _RandomBeacon.Contract.PendingAuthorizationDecrease(&_RandomBeacon.CallOpts, stakingProvider)
}

// PendingAuthorizationDecrease is a free data retrieval call binding the contract method 0xfd2a4788.
//
// Solidity: function pendingAuthorizationDecrease(address stakingProvider) view returns(uint96)
func (_RandomBeacon *RandomBeaconCallerSession) PendingAuthorizationDecrease(stakingProvider common.Address) (*big.Int, error) {
	return _RandomBeacon.Contract.PendingAuthorizationDecrease(&_RandomBeacon.CallOpts, stakingProvider)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_RandomBeacon *RandomBeaconCaller) ReimbursementPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "reimbursementPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_RandomBeacon *RandomBeaconSession) ReimbursementPool() (common.Address, error) {
	return _RandomBeacon.Contract.ReimbursementPool(&_RandomBeacon.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_RandomBeacon *RandomBeaconCallerSession) ReimbursementPool() (common.Address, error) {
	return _RandomBeacon.Contract.ReimbursementPool(&_RandomBeacon.CallOpts)
}

// RelayEntryParameters is a free data retrieval call binding the contract method 0x74024ab7.
//
// Solidity: function relayEntryParameters() view returns(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit)
func (_RandomBeacon *RandomBeaconCaller) RelayEntryParameters(opts *bind.CallOpts) (struct {
	RelayEntrySoftTimeout *big.Int
	RelayEntryHardTimeout *big.Int
	CallbackGasLimit      *big.Int
}, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "relayEntryParameters")

	outstruct := new(struct {
		RelayEntrySoftTimeout *big.Int
		RelayEntryHardTimeout *big.Int
		CallbackGasLimit      *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.RelayEntrySoftTimeout = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.RelayEntryHardTimeout = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.CallbackGasLimit = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// RelayEntryParameters is a free data retrieval call binding the contract method 0x74024ab7.
//
// Solidity: function relayEntryParameters() view returns(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit)
func (_RandomBeacon *RandomBeaconSession) RelayEntryParameters() (struct {
	RelayEntrySoftTimeout *big.Int
	RelayEntryHardTimeout *big.Int
	CallbackGasLimit      *big.Int
}, error) {
	return _RandomBeacon.Contract.RelayEntryParameters(&_RandomBeacon.CallOpts)
}

// RelayEntryParameters is a free data retrieval call binding the contract method 0x74024ab7.
//
// Solidity: function relayEntryParameters() view returns(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit)
func (_RandomBeacon *RandomBeaconCallerSession) RelayEntryParameters() (struct {
	RelayEntrySoftTimeout *big.Int
	RelayEntryHardTimeout *big.Int
	CallbackGasLimit      *big.Int
}, error) {
	return _RandomBeacon.Contract.RelayEntryParameters(&_RandomBeacon.CallOpts)
}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_RandomBeacon *RandomBeaconCaller) RemainingAuthorizationDecreaseDelay(opts *bind.CallOpts, stakingProvider common.Address) (uint64, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "remainingAuthorizationDecreaseDelay", stakingProvider)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_RandomBeacon *RandomBeaconSession) RemainingAuthorizationDecreaseDelay(stakingProvider common.Address) (uint64, error) {
	return _RandomBeacon.Contract.RemainingAuthorizationDecreaseDelay(&_RandomBeacon.CallOpts, stakingProvider)
}

// RemainingAuthorizationDecreaseDelay is a free data retrieval call binding the contract method 0x9c9de028.
//
// Solidity: function remainingAuthorizationDecreaseDelay(address stakingProvider) view returns(uint64)
func (_RandomBeacon *RandomBeaconCallerSession) RemainingAuthorizationDecreaseDelay(stakingProvider common.Address) (uint64, error) {
	return _RandomBeacon.Contract.RemainingAuthorizationDecreaseDelay(&_RandomBeacon.CallOpts, stakingProvider)
}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier)
func (_RandomBeacon *RandomBeaconCaller) RewardParameters(opts *bind.CallOpts) (struct {
	SortitionPoolRewardsBanDuration                 *big.Int
	RelayEntryTimeoutNotificationRewardMultiplier   *big.Int
	UnauthorizedSigningNotificationRewardMultiplier *big.Int
	DkgMaliciousResultNotificationRewardMultiplier  *big.Int
}, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "rewardParameters")

	outstruct := new(struct {
		SortitionPoolRewardsBanDuration                 *big.Int
		RelayEntryTimeoutNotificationRewardMultiplier   *big.Int
		UnauthorizedSigningNotificationRewardMultiplier *big.Int
		DkgMaliciousResultNotificationRewardMultiplier  *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.SortitionPoolRewardsBanDuration = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.RelayEntryTimeoutNotificationRewardMultiplier = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.UnauthorizedSigningNotificationRewardMultiplier = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.DkgMaliciousResultNotificationRewardMultiplier = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier)
func (_RandomBeacon *RandomBeaconSession) RewardParameters() (struct {
	SortitionPoolRewardsBanDuration                 *big.Int
	RelayEntryTimeoutNotificationRewardMultiplier   *big.Int
	UnauthorizedSigningNotificationRewardMultiplier *big.Int
	DkgMaliciousResultNotificationRewardMultiplier  *big.Int
}, error) {
	return _RandomBeacon.Contract.RewardParameters(&_RandomBeacon.CallOpts)
}

// RewardParameters is a free data retrieval call binding the contract method 0x52902301.
//
// Solidity: function rewardParameters() view returns(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier)
func (_RandomBeacon *RandomBeaconCallerSession) RewardParameters() (struct {
	SortitionPoolRewardsBanDuration                 *big.Int
	RelayEntryTimeoutNotificationRewardMultiplier   *big.Int
	UnauthorizedSigningNotificationRewardMultiplier *big.Int
	DkgMaliciousResultNotificationRewardMultiplier  *big.Int
}, error) {
	return _RandomBeacon.Contract.RewardParameters(&_RandomBeacon.CallOpts)
}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_RandomBeacon *RandomBeaconCaller) SelectGroup(opts *bind.CallOpts) ([]uint32, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "selectGroup")

	if err != nil {
		return *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint32)).(*[]uint32)

	return out0, err

}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_RandomBeacon *RandomBeaconSession) SelectGroup() ([]uint32, error) {
	return _RandomBeacon.Contract.SelectGroup(&_RandomBeacon.CallOpts)
}

// SelectGroup is a free data retrieval call binding the contract method 0xe03e4535.
//
// Solidity: function selectGroup() view returns(uint32[])
func (_RandomBeacon *RandomBeaconCallerSession) SelectGroup() ([]uint32, error) {
	return _RandomBeacon.Contract.SelectGroup(&_RandomBeacon.CallOpts)
}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 relayEntrySubmissionFailureSlashingAmount, uint96 maliciousDkgResultSlashingAmount, uint96 unauthorizedSigningSlashingAmount)
func (_RandomBeacon *RandomBeaconCaller) SlashingParameters(opts *bind.CallOpts) (struct {
	RelayEntrySubmissionFailureSlashingAmount *big.Int
	MaliciousDkgResultSlashingAmount          *big.Int
	UnauthorizedSigningSlashingAmount         *big.Int
}, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "slashingParameters")

	outstruct := new(struct {
		RelayEntrySubmissionFailureSlashingAmount *big.Int
		MaliciousDkgResultSlashingAmount          *big.Int
		UnauthorizedSigningSlashingAmount         *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.RelayEntrySubmissionFailureSlashingAmount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.MaliciousDkgResultSlashingAmount = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.UnauthorizedSigningSlashingAmount = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 relayEntrySubmissionFailureSlashingAmount, uint96 maliciousDkgResultSlashingAmount, uint96 unauthorizedSigningSlashingAmount)
func (_RandomBeacon *RandomBeaconSession) SlashingParameters() (struct {
	RelayEntrySubmissionFailureSlashingAmount *big.Int
	MaliciousDkgResultSlashingAmount          *big.Int
	UnauthorizedSigningSlashingAmount         *big.Int
}, error) {
	return _RandomBeacon.Contract.SlashingParameters(&_RandomBeacon.CallOpts)
}

// SlashingParameters is a free data retrieval call binding the contract method 0x1d35fa63.
//
// Solidity: function slashingParameters() view returns(uint96 relayEntrySubmissionFailureSlashingAmount, uint96 maliciousDkgResultSlashingAmount, uint96 unauthorizedSigningSlashingAmount)
func (_RandomBeacon *RandomBeaconCallerSession) SlashingParameters() (struct {
	RelayEntrySubmissionFailureSlashingAmount *big.Int
	MaliciousDkgResultSlashingAmount          *big.Int
	UnauthorizedSigningSlashingAmount         *big.Int
}, error) {
	return _RandomBeacon.Contract.SlashingParameters(&_RandomBeacon.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_RandomBeacon *RandomBeaconCaller) SortitionPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "sortitionPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_RandomBeacon *RandomBeaconSession) SortitionPool() (common.Address, error) {
	return _RandomBeacon.Contract.SortitionPool(&_RandomBeacon.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_RandomBeacon *RandomBeaconCallerSession) SortitionPool() (common.Address, error) {
	return _RandomBeacon.Contract.SortitionPool(&_RandomBeacon.CallOpts)
}

// Staking is a free data retrieval call binding the contract method 0x4cf088d9.
//
// Solidity: function staking() view returns(address)
func (_RandomBeacon *RandomBeaconCaller) Staking(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "staking")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Staking is a free data retrieval call binding the contract method 0x4cf088d9.
//
// Solidity: function staking() view returns(address)
func (_RandomBeacon *RandomBeaconSession) Staking() (common.Address, error) {
	return _RandomBeacon.Contract.Staking(&_RandomBeacon.CallOpts)
}

// Staking is a free data retrieval call binding the contract method 0x4cf088d9.
//
// Solidity: function staking() view returns(address)
func (_RandomBeacon *RandomBeaconCallerSession) Staking() (common.Address, error) {
	return _RandomBeacon.Contract.Staking(&_RandomBeacon.CallOpts)
}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_RandomBeacon *RandomBeaconCaller) StakingProviderToOperator(opts *bind.CallOpts, stakingProvider common.Address) (common.Address, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "stakingProviderToOperator", stakingProvider)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_RandomBeacon *RandomBeaconSession) StakingProviderToOperator(stakingProvider common.Address) (common.Address, error) {
	return _RandomBeacon.Contract.StakingProviderToOperator(&_RandomBeacon.CallOpts, stakingProvider)
}

// StakingProviderToOperator is a free data retrieval call binding the contract method 0xc7c49c98.
//
// Solidity: function stakingProviderToOperator(address stakingProvider) view returns(address)
func (_RandomBeacon *RandomBeaconCallerSession) StakingProviderToOperator(stakingProvider common.Address) (common.Address, error) {
	return _RandomBeacon.Contract.StakingProviderToOperator(&_RandomBeacon.CallOpts, stakingProvider)
}

// TToken is a free data retrieval call binding the contract method 0xc35d64ea.
//
// Solidity: function tToken() view returns(address)
func (_RandomBeacon *RandomBeaconCaller) TToken(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RandomBeacon.contract.Call(opts, &out, "tToken")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TToken is a free data retrieval call binding the contract method 0xc35d64ea.
//
// Solidity: function tToken() view returns(address)
func (_RandomBeacon *RandomBeaconSession) TToken() (common.Address, error) {
	return _RandomBeacon.Contract.TToken(&_RandomBeacon.CallOpts)
}

// TToken is a free data retrieval call binding the contract method 0xc35d64ea.
//
// Solidity: function tToken() view returns(address)
func (_RandomBeacon *RandomBeaconCallerSession) TToken() (common.Address, error) {
	return _RandomBeacon.Contract.TToken(&_RandomBeacon.CallOpts)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_RandomBeacon *RandomBeaconTransactor) ApproveAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "approveAuthorizationDecrease", stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_RandomBeacon *RandomBeaconSession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ApproveAuthorizationDecrease(&_RandomBeacon.TransactOpts, stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ApproveAuthorizationDecrease(&_RandomBeacon.TransactOpts, stakingProvider)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x5c5b3870.
//
// Solidity: function approveDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconTransactor) ApproveDkgResult(opts *bind.TransactOpts, dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "approveDkgResult", dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x5c5b3870.
//
// Solidity: function approveDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconSession) ApproveDkgResult(dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ApproveDkgResult(&_RandomBeacon.TransactOpts, dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x5c5b3870.
//
// Solidity: function approveDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) ApproveDkgResult(dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ApproveDkgResult(&_RandomBeacon.TransactOpts, dkgResult)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconTransactor) AuthorizationDecreaseRequested(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "authorizationDecreaseRequested", stakingProvider, fromAmount, toAmount)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconSession) AuthorizationDecreaseRequested(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.AuthorizationDecreaseRequested(&_RandomBeacon.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationDecreaseRequested is a paid mutator transaction binding the contract method 0x6a7f7a90.
//
// Solidity: function authorizationDecreaseRequested(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) AuthorizationDecreaseRequested(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.AuthorizationDecreaseRequested(&_RandomBeacon.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconTransactor) AuthorizationIncreased(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "authorizationIncreased", stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconSession) AuthorizationIncreased(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.AuthorizationIncreased(&_RandomBeacon.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// AuthorizationIncreased is a paid mutator transaction binding the contract method 0xc9bacaad.
//
// Solidity: function authorizationIncreased(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) AuthorizationIncreased(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.AuthorizationIncreased(&_RandomBeacon.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x31376766.
//
// Solidity: function challengeDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconTransactor) ChallengeDkgResult(opts *bind.TransactOpts, dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "challengeDkgResult", dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x31376766.
//
// Solidity: function challengeDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconSession) ChallengeDkgResult(dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ChallengeDkgResult(&_RandomBeacon.TransactOpts, dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0x31376766.
//
// Solidity: function challengeDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) ChallengeDkgResult(dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ChallengeDkgResult(&_RandomBeacon.TransactOpts, dkgResult)
}

// Genesis is a paid mutator transaction binding the contract method 0xa7f0b3de.
//
// Solidity: function genesis() returns()
func (_RandomBeacon *RandomBeaconTransactor) Genesis(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "genesis")
}

// Genesis is a paid mutator transaction binding the contract method 0xa7f0b3de.
//
// Solidity: function genesis() returns()
func (_RandomBeacon *RandomBeaconSession) Genesis() (*types.Transaction, error) {
	return _RandomBeacon.Contract.Genesis(&_RandomBeacon.TransactOpts)
}

// Genesis is a paid mutator transaction binding the contract method 0xa7f0b3de.
//
// Solidity: function genesis() returns()
func (_RandomBeacon *RandomBeaconTransactorSession) Genesis() (*types.Transaction, error) {
	return _RandomBeacon.Contract.Genesis(&_RandomBeacon.TransactOpts)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconTransactor) InvoluntaryAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "involuntaryAuthorizationDecrease", stakingProvider, fromAmount, toAmount)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconSession) InvoluntaryAuthorizationDecrease(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.InvoluntaryAuthorizationDecrease(&_RandomBeacon.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// InvoluntaryAuthorizationDecrease is a paid mutator transaction binding the contract method 0x14a85474.
//
// Solidity: function involuntaryAuthorizationDecrease(address stakingProvider, uint96 fromAmount, uint96 toAmount) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) InvoluntaryAuthorizationDecrease(stakingProvider common.Address, fromAmount *big.Int, toAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.InvoluntaryAuthorizationDecrease(&_RandomBeacon.TransactOpts, stakingProvider, fromAmount, toAmount)
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_RandomBeacon *RandomBeaconTransactor) JoinSortitionPool(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "joinSortitionPool")
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_RandomBeacon *RandomBeaconSession) JoinSortitionPool() (*types.Transaction, error) {
	return _RandomBeacon.Contract.JoinSortitionPool(&_RandomBeacon.TransactOpts)
}

// JoinSortitionPool is a paid mutator transaction binding the contract method 0x167f0517.
//
// Solidity: function joinSortitionPool() returns()
func (_RandomBeacon *RandomBeaconTransactorSession) JoinSortitionPool() (*types.Transaction, error) {
	return _RandomBeacon.Contract.JoinSortitionPool(&_RandomBeacon.TransactOpts)
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_RandomBeacon *RandomBeaconTransactor) NotifyDkgTimeout(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "notifyDkgTimeout")
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_RandomBeacon *RandomBeaconSession) NotifyDkgTimeout() (*types.Transaction, error) {
	return _RandomBeacon.Contract.NotifyDkgTimeout(&_RandomBeacon.TransactOpts)
}

// NotifyDkgTimeout is a paid mutator transaction binding the contract method 0xd855c631.
//
// Solidity: function notifyDkgTimeout() returns()
func (_RandomBeacon *RandomBeaconTransactorSession) NotifyDkgTimeout() (*types.Transaction, error) {
	return _RandomBeacon.Contract.NotifyDkgTimeout(&_RandomBeacon.TransactOpts)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0xccfd3bbe.
//
// Solidity: function notifyOperatorInactivity((uint64,uint256[],bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactor) NotifyOperatorInactivity(opts *bind.TransactOpts, claim BeaconInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "notifyOperatorInactivity", claim, nonce, groupMembers)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0xccfd3bbe.
//
// Solidity: function notifyOperatorInactivity((uint64,uint256[],bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconSession) NotifyOperatorInactivity(claim BeaconInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.NotifyOperatorInactivity(&_RandomBeacon.TransactOpts, claim, nonce, groupMembers)
}

// NotifyOperatorInactivity is a paid mutator transaction binding the contract method 0xccfd3bbe.
//
// Solidity: function notifyOperatorInactivity((uint64,uint256[],bytes,uint256[]) claim, uint256 nonce, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) NotifyOperatorInactivity(claim BeaconInactivityClaim, nonce *big.Int, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.NotifyOperatorInactivity(&_RandomBeacon.TransactOpts, claim, nonce, groupMembers)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_RandomBeacon *RandomBeaconTransactor) RegisterOperator(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "registerOperator", operator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_RandomBeacon *RandomBeaconSession) RegisterOperator(operator common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.RegisterOperator(&_RandomBeacon.TransactOpts, operator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address operator) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) RegisterOperator(operator common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.RegisterOperator(&_RandomBeacon.TransactOpts, operator)
}

// ReportRelayEntryTimeout is a paid mutator transaction binding the contract method 0x4a07068e.
//
// Solidity: function reportRelayEntryTimeout(uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactor) ReportRelayEntryTimeout(opts *bind.TransactOpts, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "reportRelayEntryTimeout", groupMembers)
}

// ReportRelayEntryTimeout is a paid mutator transaction binding the contract method 0x4a07068e.
//
// Solidity: function reportRelayEntryTimeout(uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconSession) ReportRelayEntryTimeout(groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ReportRelayEntryTimeout(&_RandomBeacon.TransactOpts, groupMembers)
}

// ReportRelayEntryTimeout is a paid mutator transaction binding the contract method 0x4a07068e.
//
// Solidity: function reportRelayEntryTimeout(uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) ReportRelayEntryTimeout(groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ReportRelayEntryTimeout(&_RandomBeacon.TransactOpts, groupMembers)
}

// ReportUnauthorizedSigning is a paid mutator transaction binding the contract method 0xb53dacdf.
//
// Solidity: function reportUnauthorizedSigning(bytes signedMsgSender, uint64 groupId, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactor) ReportUnauthorizedSigning(opts *bind.TransactOpts, signedMsgSender []byte, groupId uint64, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "reportUnauthorizedSigning", signedMsgSender, groupId, groupMembers)
}

// ReportUnauthorizedSigning is a paid mutator transaction binding the contract method 0xb53dacdf.
//
// Solidity: function reportUnauthorizedSigning(bytes signedMsgSender, uint64 groupId, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconSession) ReportUnauthorizedSigning(signedMsgSender []byte, groupId uint64, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ReportUnauthorizedSigning(&_RandomBeacon.TransactOpts, signedMsgSender, groupId, groupMembers)
}

// ReportUnauthorizedSigning is a paid mutator transaction binding the contract method 0xb53dacdf.
//
// Solidity: function reportUnauthorizedSigning(bytes signedMsgSender, uint64 groupId, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) ReportUnauthorizedSigning(signedMsgSender []byte, groupId uint64, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.ReportUnauthorizedSigning(&_RandomBeacon.TransactOpts, signedMsgSender, groupId, groupMembers)
}

// RequestRelayEntry is a paid mutator transaction binding the contract method 0x4368ec0c.
//
// Solidity: function requestRelayEntry(address callbackContract) returns()
func (_RandomBeacon *RandomBeaconTransactor) RequestRelayEntry(opts *bind.TransactOpts, callbackContract common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "requestRelayEntry", callbackContract)
}

// RequestRelayEntry is a paid mutator transaction binding the contract method 0x4368ec0c.
//
// Solidity: function requestRelayEntry(address callbackContract) returns()
func (_RandomBeacon *RandomBeaconSession) RequestRelayEntry(callbackContract common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.RequestRelayEntry(&_RandomBeacon.TransactOpts, callbackContract)
}

// RequestRelayEntry is a paid mutator transaction binding the contract method 0x4368ec0c.
//
// Solidity: function requestRelayEntry(address callbackContract) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) RequestRelayEntry(callbackContract common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.RequestRelayEntry(&_RandomBeacon.TransactOpts, callbackContract)
}

// SetRequesterAuthorization is a paid mutator transaction binding the contract method 0x985e6487.
//
// Solidity: function setRequesterAuthorization(address requester, bool isAuthorized) returns()
func (_RandomBeacon *RandomBeaconTransactor) SetRequesterAuthorization(opts *bind.TransactOpts, requester common.Address, isAuthorized bool) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "setRequesterAuthorization", requester, isAuthorized)
}

// SetRequesterAuthorization is a paid mutator transaction binding the contract method 0x985e6487.
//
// Solidity: function setRequesterAuthorization(address requester, bool isAuthorized) returns()
func (_RandomBeacon *RandomBeaconSession) SetRequesterAuthorization(requester common.Address, isAuthorized bool) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SetRequesterAuthorization(&_RandomBeacon.TransactOpts, requester, isAuthorized)
}

// SetRequesterAuthorization is a paid mutator transaction binding the contract method 0x985e6487.
//
// Solidity: function setRequesterAuthorization(address requester, bool isAuthorized) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) SetRequesterAuthorization(requester common.Address, isAuthorized bool) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SetRequesterAuthorization(&_RandomBeacon.TransactOpts, requester, isAuthorized)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x7e0049fd.
//
// Solidity: function submitDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconTransactor) SubmitDkgResult(opts *bind.TransactOpts, dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "submitDkgResult", dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x7e0049fd.
//
// Solidity: function submitDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconSession) SubmitDkgResult(dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SubmitDkgResult(&_RandomBeacon.TransactOpts, dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0x7e0049fd.
//
// Solidity: function submitDkgResult((uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) dkgResult) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) SubmitDkgResult(dkgResult BeaconDkgResult) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SubmitDkgResult(&_RandomBeacon.TransactOpts, dkgResult)
}

// SubmitRelayEntry is a paid mutator transaction binding the contract method 0x55b64bcc.
//
// Solidity: function submitRelayEntry(bytes entry, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactor) SubmitRelayEntry(opts *bind.TransactOpts, entry []byte, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "submitRelayEntry", entry, groupMembers)
}

// SubmitRelayEntry is a paid mutator transaction binding the contract method 0x55b64bcc.
//
// Solidity: function submitRelayEntry(bytes entry, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconSession) SubmitRelayEntry(entry []byte, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SubmitRelayEntry(&_RandomBeacon.TransactOpts, entry, groupMembers)
}

// SubmitRelayEntry is a paid mutator transaction binding the contract method 0x55b64bcc.
//
// Solidity: function submitRelayEntry(bytes entry, uint32[] groupMembers) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) SubmitRelayEntry(entry []byte, groupMembers []uint32) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SubmitRelayEntry(&_RandomBeacon.TransactOpts, entry, groupMembers)
}

// SubmitRelayEntry0 is a paid mutator transaction binding the contract method 0x57665912.
//
// Solidity: function submitRelayEntry(bytes entry) returns()
func (_RandomBeacon *RandomBeaconTransactor) SubmitRelayEntry0(opts *bind.TransactOpts, entry []byte) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "submitRelayEntry0", entry)
}

// SubmitRelayEntry0 is a paid mutator transaction binding the contract method 0x57665912.
//
// Solidity: function submitRelayEntry(bytes entry) returns()
func (_RandomBeacon *RandomBeaconSession) SubmitRelayEntry0(entry []byte) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SubmitRelayEntry0(&_RandomBeacon.TransactOpts, entry)
}

// SubmitRelayEntry0 is a paid mutator transaction binding the contract method 0x57665912.
//
// Solidity: function submitRelayEntry(bytes entry) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) SubmitRelayEntry0(entry []byte) (*types.Transaction, error) {
	return _RandomBeacon.Contract.SubmitRelayEntry0(&_RandomBeacon.TransactOpts, entry)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_RandomBeacon *RandomBeaconTransactor) TransferGovernance(opts *bind.TransactOpts, newGovernance common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "transferGovernance", newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_RandomBeacon *RandomBeaconSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.TransferGovernance(&_RandomBeacon.TransactOpts, newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.TransferGovernance(&_RandomBeacon.TransactOpts, newGovernance)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateAuthorizationParameters(opts *bind.TransactOpts, _minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateAuthorizationParameters", _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateAuthorizationParameters(_minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateAuthorizationParameters(&_RandomBeacon.TransactOpts, _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateAuthorizationParameters is a paid mutator transaction binding the contract method 0xa04e2980.
//
// Solidity: function updateAuthorizationParameters(uint96 _minimumAuthorization, uint64 _authorizationDecreaseDelay, uint64 _authorizationDecreaseChangePeriod) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateAuthorizationParameters(_minimumAuthorization *big.Int, _authorizationDecreaseDelay uint64, _authorizationDecreaseChangePeriod uint64) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateAuthorizationParameters(&_RandomBeacon.TransactOpts, _minimumAuthorization, _authorizationDecreaseDelay, _authorizationDecreaseChangePeriod)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xb0d010d6.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateGasParameters(opts *bind.TransactOpts, dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, relayEntrySubmissionGasOffset *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateGasParameters", dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, relayEntrySubmissionGasOffset)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xb0d010d6.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateGasParameters(dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, relayEntrySubmissionGasOffset *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateGasParameters(&_RandomBeacon.TransactOpts, dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, relayEntrySubmissionGasOffset)
}

// UpdateGasParameters is a paid mutator transaction binding the contract method 0xb0d010d6.
//
// Solidity: function updateGasParameters(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateGasParameters(dkgResultSubmissionGas *big.Int, dkgResultApprovalGasOffset *big.Int, notifyOperatorInactivityGasOffset *big.Int, relayEntrySubmissionGasOffset *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateGasParameters(&_RandomBeacon.TransactOpts, dkgResultSubmissionGas, dkgResultApprovalGasOffset, notifyOperatorInactivityGasOffset, relayEntrySubmissionGasOffset)
}

// UpdateGroupCreationParameters is a paid mutator transaction binding the contract method 0x77a5a7bd.
//
// Solidity: function updateGroupCreationParameters(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgSubmitterPrecedencePeriodLength) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateGroupCreationParameters(opts *bind.TransactOpts, groupCreationFrequency *big.Int, groupLifetime *big.Int, dkgResultChallengePeriodLength *big.Int, dkgResultChallengeExtraGas *big.Int, dkgResultSubmissionTimeout *big.Int, dkgSubmitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateGroupCreationParameters", groupCreationFrequency, groupLifetime, dkgResultChallengePeriodLength, dkgResultChallengeExtraGas, dkgResultSubmissionTimeout, dkgSubmitterPrecedencePeriodLength)
}

// UpdateGroupCreationParameters is a paid mutator transaction binding the contract method 0x77a5a7bd.
//
// Solidity: function updateGroupCreationParameters(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgSubmitterPrecedencePeriodLength) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateGroupCreationParameters(groupCreationFrequency *big.Int, groupLifetime *big.Int, dkgResultChallengePeriodLength *big.Int, dkgResultChallengeExtraGas *big.Int, dkgResultSubmissionTimeout *big.Int, dkgSubmitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateGroupCreationParameters(&_RandomBeacon.TransactOpts, groupCreationFrequency, groupLifetime, dkgResultChallengePeriodLength, dkgResultChallengeExtraGas, dkgResultSubmissionTimeout, dkgSubmitterPrecedencePeriodLength)
}

// UpdateGroupCreationParameters is a paid mutator transaction binding the contract method 0x77a5a7bd.
//
// Solidity: function updateGroupCreationParameters(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgSubmitterPrecedencePeriodLength) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateGroupCreationParameters(groupCreationFrequency *big.Int, groupLifetime *big.Int, dkgResultChallengePeriodLength *big.Int, dkgResultChallengeExtraGas *big.Int, dkgResultSubmissionTimeout *big.Int, dkgSubmitterPrecedencePeriodLength *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateGroupCreationParameters(&_RandomBeacon.TransactOpts, groupCreationFrequency, groupLifetime, dkgResultChallengePeriodLength, dkgResultChallengeExtraGas, dkgResultSubmissionTimeout, dkgSubmitterPrecedencePeriodLength)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateOperatorStatus(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateOperatorStatus", operator)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateOperatorStatus(operator common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateOperatorStatus(&_RandomBeacon.TransactOpts, operator)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0x1c5b0762.
//
// Solidity: function updateOperatorStatus(address operator) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateOperatorStatus(operator common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateOperatorStatus(&_RandomBeacon.TransactOpts, operator)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateReimbursementPool(opts *bind.TransactOpts, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateReimbursementPool", _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateReimbursementPool(&_RandomBeacon.TransactOpts, _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateReimbursementPool(&_RandomBeacon.TransactOpts, _reimbursementPool)
}

// UpdateRelayEntryParameters is a paid mutator transaction binding the contract method 0x9a7d0935.
//
// Solidity: function updateRelayEntryParameters(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateRelayEntryParameters(opts *bind.TransactOpts, relayEntrySoftTimeout *big.Int, relayEntryHardTimeout *big.Int, callbackGasLimit *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateRelayEntryParameters", relayEntrySoftTimeout, relayEntryHardTimeout, callbackGasLimit)
}

// UpdateRelayEntryParameters is a paid mutator transaction binding the contract method 0x9a7d0935.
//
// Solidity: function updateRelayEntryParameters(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateRelayEntryParameters(relayEntrySoftTimeout *big.Int, relayEntryHardTimeout *big.Int, callbackGasLimit *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateRelayEntryParameters(&_RandomBeacon.TransactOpts, relayEntrySoftTimeout, relayEntryHardTimeout, callbackGasLimit)
}

// UpdateRelayEntryParameters is a paid mutator transaction binding the contract method 0x9a7d0935.
//
// Solidity: function updateRelayEntryParameters(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateRelayEntryParameters(relayEntrySoftTimeout *big.Int, relayEntryHardTimeout *big.Int, callbackGasLimit *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateRelayEntryParameters(&_RandomBeacon.TransactOpts, relayEntrySoftTimeout, relayEntryHardTimeout, callbackGasLimit)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x339646ac.
//
// Solidity: function updateRewardParameters(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateRewardParameters(opts *bind.TransactOpts, sortitionPoolRewardsBanDuration *big.Int, relayEntryTimeoutNotificationRewardMultiplier *big.Int, unauthorizedSigningNotificationRewardMultiplier *big.Int, dkgMaliciousResultNotificationRewardMultiplier *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateRewardParameters", sortitionPoolRewardsBanDuration, relayEntryTimeoutNotificationRewardMultiplier, unauthorizedSigningNotificationRewardMultiplier, dkgMaliciousResultNotificationRewardMultiplier)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x339646ac.
//
// Solidity: function updateRewardParameters(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateRewardParameters(sortitionPoolRewardsBanDuration *big.Int, relayEntryTimeoutNotificationRewardMultiplier *big.Int, unauthorizedSigningNotificationRewardMultiplier *big.Int, dkgMaliciousResultNotificationRewardMultiplier *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateRewardParameters(&_RandomBeacon.TransactOpts, sortitionPoolRewardsBanDuration, relayEntryTimeoutNotificationRewardMultiplier, unauthorizedSigningNotificationRewardMultiplier, dkgMaliciousResultNotificationRewardMultiplier)
}

// UpdateRewardParameters is a paid mutator transaction binding the contract method 0x339646ac.
//
// Solidity: function updateRewardParameters(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateRewardParameters(sortitionPoolRewardsBanDuration *big.Int, relayEntryTimeoutNotificationRewardMultiplier *big.Int, unauthorizedSigningNotificationRewardMultiplier *big.Int, dkgMaliciousResultNotificationRewardMultiplier *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateRewardParameters(&_RandomBeacon.TransactOpts, sortitionPoolRewardsBanDuration, relayEntryTimeoutNotificationRewardMultiplier, unauthorizedSigningNotificationRewardMultiplier, dkgMaliciousResultNotificationRewardMultiplier)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x0519e991.
//
// Solidity: function updateSlashingParameters(uint96 relayEntrySubmissionFailureSlashingAmount, uint96 maliciousDkgResultSlashingAmount, uint96 unauthorizedSigningSlashingAmount) returns()
func (_RandomBeacon *RandomBeaconTransactor) UpdateSlashingParameters(opts *bind.TransactOpts, relayEntrySubmissionFailureSlashingAmount *big.Int, maliciousDkgResultSlashingAmount *big.Int, unauthorizedSigningSlashingAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "updateSlashingParameters", relayEntrySubmissionFailureSlashingAmount, maliciousDkgResultSlashingAmount, unauthorizedSigningSlashingAmount)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x0519e991.
//
// Solidity: function updateSlashingParameters(uint96 relayEntrySubmissionFailureSlashingAmount, uint96 maliciousDkgResultSlashingAmount, uint96 unauthorizedSigningSlashingAmount) returns()
func (_RandomBeacon *RandomBeaconSession) UpdateSlashingParameters(relayEntrySubmissionFailureSlashingAmount *big.Int, maliciousDkgResultSlashingAmount *big.Int, unauthorizedSigningSlashingAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateSlashingParameters(&_RandomBeacon.TransactOpts, relayEntrySubmissionFailureSlashingAmount, maliciousDkgResultSlashingAmount, unauthorizedSigningSlashingAmount)
}

// UpdateSlashingParameters is a paid mutator transaction binding the contract method 0x0519e991.
//
// Solidity: function updateSlashingParameters(uint96 relayEntrySubmissionFailureSlashingAmount, uint96 maliciousDkgResultSlashingAmount, uint96 unauthorizedSigningSlashingAmount) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) UpdateSlashingParameters(relayEntrySubmissionFailureSlashingAmount *big.Int, maliciousDkgResultSlashingAmount *big.Int, unauthorizedSigningSlashingAmount *big.Int) (*types.Transaction, error) {
	return _RandomBeacon.Contract.UpdateSlashingParameters(&_RandomBeacon.TransactOpts, relayEntrySubmissionFailureSlashingAmount, maliciousDkgResultSlashingAmount, unauthorizedSigningSlashingAmount)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_RandomBeacon *RandomBeaconTransactor) WithdrawIneligibleRewards(opts *bind.TransactOpts, recipient common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "withdrawIneligibleRewards", recipient)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_RandomBeacon *RandomBeaconSession) WithdrawIneligibleRewards(recipient common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.WithdrawIneligibleRewards(&_RandomBeacon.TransactOpts, recipient)
}

// WithdrawIneligibleRewards is a paid mutator transaction binding the contract method 0x663032cd.
//
// Solidity: function withdrawIneligibleRewards(address recipient) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) WithdrawIneligibleRewards(recipient common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.WithdrawIneligibleRewards(&_RandomBeacon.TransactOpts, recipient)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_RandomBeacon *RandomBeaconTransactor) WithdrawRewards(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _RandomBeacon.contract.Transact(opts, "withdrawRewards", stakingProvider)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_RandomBeacon *RandomBeaconSession) WithdrawRewards(stakingProvider common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.WithdrawRewards(&_RandomBeacon.TransactOpts, stakingProvider)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0x42d86693.
//
// Solidity: function withdrawRewards(address stakingProvider) returns()
func (_RandomBeacon *RandomBeaconTransactorSession) WithdrawRewards(stakingProvider common.Address) (*types.Transaction, error) {
	return _RandomBeacon.Contract.WithdrawRewards(&_RandomBeacon.TransactOpts, stakingProvider)
}

// RandomBeaconAuthorizationDecreaseApprovedIterator is returned from FilterAuthorizationDecreaseApproved and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseApproved events raised by the RandomBeacon contract.
type RandomBeaconAuthorizationDecreaseApprovedIterator struct {
	Event *RandomBeaconAuthorizationDecreaseApproved // Event containing the contract specifics and raw log

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
func (it *RandomBeaconAuthorizationDecreaseApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconAuthorizationDecreaseApproved)
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
		it.Event = new(RandomBeaconAuthorizationDecreaseApproved)
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
func (it *RandomBeaconAuthorizationDecreaseApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconAuthorizationDecreaseApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconAuthorizationDecreaseApproved represents a AuthorizationDecreaseApproved event raised by the RandomBeacon contract.
type RandomBeaconAuthorizationDecreaseApproved struct {
	StakingProvider common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationDecreaseApproved is a free log retrieval operation binding the contract event 0x50270a522c2fef97b6b7385c2aa4a4518adda681530e0a1fe9f5e840f6f2cd9d.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider)
func (_RandomBeacon *RandomBeaconFilterer) FilterAuthorizationDecreaseApproved(opts *bind.FilterOpts, stakingProvider []common.Address) (*RandomBeaconAuthorizationDecreaseApprovedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconAuthorizationDecreaseApprovedIterator{contract: _RandomBeacon.contract, event: "AuthorizationDecreaseApproved", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseApproved is a free log subscription operation binding the contract event 0x50270a522c2fef97b6b7385c2aa4a4518adda681530e0a1fe9f5e840f6f2cd9d.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider)
func (_RandomBeacon *RandomBeaconFilterer) WatchAuthorizationDecreaseApproved(opts *bind.WatchOpts, sink chan<- *RandomBeaconAuthorizationDecreaseApproved, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconAuthorizationDecreaseApproved)
				if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseAuthorizationDecreaseApproved(log types.Log) (*RandomBeaconAuthorizationDecreaseApproved, error) {
	event := new(RandomBeaconAuthorizationDecreaseApproved)
	if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconAuthorizationDecreaseRequestedIterator is returned from FilterAuthorizationDecreaseRequested and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseRequested events raised by the RandomBeacon contract.
type RandomBeaconAuthorizationDecreaseRequestedIterator struct {
	Event *RandomBeaconAuthorizationDecreaseRequested // Event containing the contract specifics and raw log

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
func (it *RandomBeaconAuthorizationDecreaseRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconAuthorizationDecreaseRequested)
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
		it.Event = new(RandomBeaconAuthorizationDecreaseRequested)
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
func (it *RandomBeaconAuthorizationDecreaseRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconAuthorizationDecreaseRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconAuthorizationDecreaseRequested represents a AuthorizationDecreaseRequested event raised by the RandomBeacon contract.
type RandomBeaconAuthorizationDecreaseRequested struct {
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
func (_RandomBeacon *RandomBeaconFilterer) FilterAuthorizationDecreaseRequested(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*RandomBeaconAuthorizationDecreaseRequestedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconAuthorizationDecreaseRequestedIterator{contract: _RandomBeacon.contract, event: "AuthorizationDecreaseRequested", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseRequested is a free log subscription operation binding the contract event 0x545cbf267cef6fe43f11f6219417ab43a0e8e345adbaae5f626d9bc325e8535a.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount, uint64 decreasingAt)
func (_RandomBeacon *RandomBeaconFilterer) WatchAuthorizationDecreaseRequested(opts *bind.WatchOpts, sink chan<- *RandomBeaconAuthorizationDecreaseRequested, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconAuthorizationDecreaseRequested)
				if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseAuthorizationDecreaseRequested(log types.Log) (*RandomBeaconAuthorizationDecreaseRequested, error) {
	event := new(RandomBeaconAuthorizationDecreaseRequested)
	if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconAuthorizationIncreasedIterator is returned from FilterAuthorizationIncreased and is used to iterate over the raw logs and unpacked data for AuthorizationIncreased events raised by the RandomBeacon contract.
type RandomBeaconAuthorizationIncreasedIterator struct {
	Event *RandomBeaconAuthorizationIncreased // Event containing the contract specifics and raw log

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
func (it *RandomBeaconAuthorizationIncreasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconAuthorizationIncreased)
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
		it.Event = new(RandomBeaconAuthorizationIncreased)
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
func (it *RandomBeaconAuthorizationIncreasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconAuthorizationIncreasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconAuthorizationIncreased represents a AuthorizationIncreased event raised by the RandomBeacon contract.
type RandomBeaconAuthorizationIncreased struct {
	StakingProvider common.Address
	Operator        common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationIncreased is a free log retrieval operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_RandomBeacon *RandomBeaconFilterer) FilterAuthorizationIncreased(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*RandomBeaconAuthorizationIncreasedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "AuthorizationIncreased", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconAuthorizationIncreasedIterator{contract: _RandomBeacon.contract, event: "AuthorizationIncreased", logs: logs, sub: sub}, nil
}

// WatchAuthorizationIncreased is a free log subscription operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_RandomBeacon *RandomBeaconFilterer) WatchAuthorizationIncreased(opts *bind.WatchOpts, sink chan<- *RandomBeaconAuthorizationIncreased, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "AuthorizationIncreased", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconAuthorizationIncreased)
				if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseAuthorizationIncreased(log types.Log) (*RandomBeaconAuthorizationIncreased, error) {
	event := new(RandomBeaconAuthorizationIncreased)
	if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconAuthorizationParametersUpdatedIterator is returned from FilterAuthorizationParametersUpdated and is used to iterate over the raw logs and unpacked data for AuthorizationParametersUpdated events raised by the RandomBeacon contract.
type RandomBeaconAuthorizationParametersUpdatedIterator struct {
	Event *RandomBeaconAuthorizationParametersUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconAuthorizationParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconAuthorizationParametersUpdated)
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
		it.Event = new(RandomBeaconAuthorizationParametersUpdated)
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
func (it *RandomBeaconAuthorizationParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconAuthorizationParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconAuthorizationParametersUpdated represents a AuthorizationParametersUpdated event raised by the RandomBeacon contract.
type RandomBeaconAuthorizationParametersUpdated struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
	Raw                               types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationParametersUpdated is a free log retrieval operation binding the contract event 0x544b726e42801bb47073854eeedae851903f66fe32a5bd24e626e10b90027b51.
//
// Solidity: event AuthorizationParametersUpdated(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_RandomBeacon *RandomBeaconFilterer) FilterAuthorizationParametersUpdated(opts *bind.FilterOpts) (*RandomBeaconAuthorizationParametersUpdatedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "AuthorizationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconAuthorizationParametersUpdatedIterator{contract: _RandomBeacon.contract, event: "AuthorizationParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchAuthorizationParametersUpdated is a free log subscription operation binding the contract event 0x544b726e42801bb47073854eeedae851903f66fe32a5bd24e626e10b90027b51.
//
// Solidity: event AuthorizationParametersUpdated(uint96 minimumAuthorization, uint64 authorizationDecreaseDelay, uint64 authorizationDecreaseChangePeriod)
func (_RandomBeacon *RandomBeaconFilterer) WatchAuthorizationParametersUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconAuthorizationParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "AuthorizationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconAuthorizationParametersUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationParametersUpdated", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseAuthorizationParametersUpdated(log types.Log) (*RandomBeaconAuthorizationParametersUpdated, error) {
	event := new(RandomBeaconAuthorizationParametersUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "AuthorizationParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconCallbackFailedIterator is returned from FilterCallbackFailed and is used to iterate over the raw logs and unpacked data for CallbackFailed events raised by the RandomBeacon contract.
type RandomBeaconCallbackFailedIterator struct {
	Event *RandomBeaconCallbackFailed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconCallbackFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconCallbackFailed)
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
		it.Event = new(RandomBeaconCallbackFailed)
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
func (it *RandomBeaconCallbackFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconCallbackFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconCallbackFailed represents a CallbackFailed event raised by the RandomBeacon contract.
type RandomBeaconCallbackFailed struct {
	Entry               *big.Int
	EntrySubmittedBlock *big.Int
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterCallbackFailed is a free log retrieval operation binding the contract event 0x5e2af61e77ec91022b1cafe282abce475bb18e8cdd47083464caa06583fe3909.
//
// Solidity: event CallbackFailed(uint256 entry, uint256 entrySubmittedBlock)
func (_RandomBeacon *RandomBeaconFilterer) FilterCallbackFailed(opts *bind.FilterOpts) (*RandomBeaconCallbackFailedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "CallbackFailed")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconCallbackFailedIterator{contract: _RandomBeacon.contract, event: "CallbackFailed", logs: logs, sub: sub}, nil
}

// WatchCallbackFailed is a free log subscription operation binding the contract event 0x5e2af61e77ec91022b1cafe282abce475bb18e8cdd47083464caa06583fe3909.
//
// Solidity: event CallbackFailed(uint256 entry, uint256 entrySubmittedBlock)
func (_RandomBeacon *RandomBeaconFilterer) WatchCallbackFailed(opts *bind.WatchOpts, sink chan<- *RandomBeaconCallbackFailed) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "CallbackFailed")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconCallbackFailed)
				if err := _RandomBeacon.contract.UnpackLog(event, "CallbackFailed", log); err != nil {
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

// ParseCallbackFailed is a log parse operation binding the contract event 0x5e2af61e77ec91022b1cafe282abce475bb18e8cdd47083464caa06583fe3909.
//
// Solidity: event CallbackFailed(uint256 entry, uint256 entrySubmittedBlock)
func (_RandomBeacon *RandomBeaconFilterer) ParseCallbackFailed(log types.Log) (*RandomBeaconCallbackFailed, error) {
	event := new(RandomBeaconCallbackFailed)
	if err := _RandomBeacon.contract.UnpackLog(event, "CallbackFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgMaliciousResultSlashedIterator is returned from FilterDkgMaliciousResultSlashed and is used to iterate over the raw logs and unpacked data for DkgMaliciousResultSlashed events raised by the RandomBeacon contract.
type RandomBeaconDkgMaliciousResultSlashedIterator struct {
	Event *RandomBeaconDkgMaliciousResultSlashed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgMaliciousResultSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgMaliciousResultSlashed)
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
		it.Event = new(RandomBeaconDkgMaliciousResultSlashed)
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
func (it *RandomBeaconDkgMaliciousResultSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgMaliciousResultSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgMaliciousResultSlashed represents a DkgMaliciousResultSlashed event raised by the RandomBeacon contract.
type RandomBeaconDkgMaliciousResultSlashed struct {
	ResultHash         [32]byte
	SlashingAmount     *big.Int
	MaliciousSubmitter common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterDkgMaliciousResultSlashed is a free log retrieval operation binding the contract event 0x88f76c659db78142f88e94db3ca791869495394c6c1b3d412ced9022dc97c9e3.
//
// Solidity: event DkgMaliciousResultSlashed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgMaliciousResultSlashed(opts *bind.FilterOpts, resultHash [][32]byte) (*RandomBeaconDkgMaliciousResultSlashedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgMaliciousResultSlashed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgMaliciousResultSlashedIterator{contract: _RandomBeacon.contract, event: "DkgMaliciousResultSlashed", logs: logs, sub: sub}, nil
}

// WatchDkgMaliciousResultSlashed is a free log subscription operation binding the contract event 0x88f76c659db78142f88e94db3ca791869495394c6c1b3d412ced9022dc97c9e3.
//
// Solidity: event DkgMaliciousResultSlashed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgMaliciousResultSlashed(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgMaliciousResultSlashed, resultHash [][32]byte) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgMaliciousResultSlashed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgMaliciousResultSlashed)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgMaliciousResultSlashed", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgMaliciousResultSlashed(log types.Log) (*RandomBeaconDkgMaliciousResultSlashed, error) {
	event := new(RandomBeaconDkgMaliciousResultSlashed)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgMaliciousResultSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgMaliciousResultSlashingFailedIterator is returned from FilterDkgMaliciousResultSlashingFailed and is used to iterate over the raw logs and unpacked data for DkgMaliciousResultSlashingFailed events raised by the RandomBeacon contract.
type RandomBeaconDkgMaliciousResultSlashingFailedIterator struct {
	Event *RandomBeaconDkgMaliciousResultSlashingFailed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgMaliciousResultSlashingFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgMaliciousResultSlashingFailed)
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
		it.Event = new(RandomBeaconDkgMaliciousResultSlashingFailed)
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
func (it *RandomBeaconDkgMaliciousResultSlashingFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgMaliciousResultSlashingFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgMaliciousResultSlashingFailed represents a DkgMaliciousResultSlashingFailed event raised by the RandomBeacon contract.
type RandomBeaconDkgMaliciousResultSlashingFailed struct {
	ResultHash         [32]byte
	SlashingAmount     *big.Int
	MaliciousSubmitter common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterDkgMaliciousResultSlashingFailed is a free log retrieval operation binding the contract event 0x14621289a12ab59e0737decc388bba91d929c723defb4682d5d19b9a12ecfecb.
//
// Solidity: event DkgMaliciousResultSlashingFailed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgMaliciousResultSlashingFailed(opts *bind.FilterOpts, resultHash [][32]byte) (*RandomBeaconDkgMaliciousResultSlashingFailedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgMaliciousResultSlashingFailed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgMaliciousResultSlashingFailedIterator{contract: _RandomBeacon.contract, event: "DkgMaliciousResultSlashingFailed", logs: logs, sub: sub}, nil
}

// WatchDkgMaliciousResultSlashingFailed is a free log subscription operation binding the contract event 0x14621289a12ab59e0737decc388bba91d929c723defb4682d5d19b9a12ecfecb.
//
// Solidity: event DkgMaliciousResultSlashingFailed(bytes32 indexed resultHash, uint256 slashingAmount, address maliciousSubmitter)
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgMaliciousResultSlashingFailed(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgMaliciousResultSlashingFailed, resultHash [][32]byte) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgMaliciousResultSlashingFailed", resultHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgMaliciousResultSlashingFailed)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgMaliciousResultSlashingFailed", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgMaliciousResultSlashingFailed(log types.Log) (*RandomBeaconDkgMaliciousResultSlashingFailed, error) {
	event := new(RandomBeaconDkgMaliciousResultSlashingFailed)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgMaliciousResultSlashingFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgResultApprovedIterator is returned from FilterDkgResultApproved and is used to iterate over the raw logs and unpacked data for DkgResultApproved events raised by the RandomBeacon contract.
type RandomBeaconDkgResultApprovedIterator struct {
	Event *RandomBeaconDkgResultApproved // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgResultApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgResultApproved)
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
		it.Event = new(RandomBeaconDkgResultApproved)
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
func (it *RandomBeaconDkgResultApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgResultApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgResultApproved represents a DkgResultApproved event raised by the RandomBeacon contract.
type RandomBeaconDkgResultApproved struct {
	ResultHash [32]byte
	Approver   common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultApproved is a free log retrieval operation binding the contract event 0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f.
//
// Solidity: event DkgResultApproved(bytes32 indexed resultHash, address indexed approver)
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgResultApproved(opts *bind.FilterOpts, resultHash [][32]byte, approver []common.Address) (*RandomBeaconDkgResultApprovedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var approverRule []interface{}
	for _, approverItem := range approver {
		approverRule = append(approverRule, approverItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgResultApproved", resultHashRule, approverRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgResultApprovedIterator{contract: _RandomBeacon.contract, event: "DkgResultApproved", logs: logs, sub: sub}, nil
}

// WatchDkgResultApproved is a free log subscription operation binding the contract event 0xe6e9d5eba171e82025efb3f3d44fd35905e7283d104284cb9f3bbc5bf1e4276f.
//
// Solidity: event DkgResultApproved(bytes32 indexed resultHash, address indexed approver)
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgResultApproved(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgResultApproved, resultHash [][32]byte, approver []common.Address) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var approverRule []interface{}
	for _, approverItem := range approver {
		approverRule = append(approverRule, approverItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgResultApproved", resultHashRule, approverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgResultApproved)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgResultApproved", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgResultApproved(log types.Log) (*RandomBeaconDkgResultApproved, error) {
	event := new(RandomBeaconDkgResultApproved)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgResultApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgResultChallengedIterator is returned from FilterDkgResultChallenged and is used to iterate over the raw logs and unpacked data for DkgResultChallenged events raised by the RandomBeacon contract.
type RandomBeaconDkgResultChallengedIterator struct {
	Event *RandomBeaconDkgResultChallenged // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgResultChallengedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgResultChallenged)
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
		it.Event = new(RandomBeaconDkgResultChallenged)
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
func (it *RandomBeaconDkgResultChallengedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgResultChallengedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgResultChallenged represents a DkgResultChallenged event raised by the RandomBeacon contract.
type RandomBeaconDkgResultChallenged struct {
	ResultHash [32]byte
	Challenger common.Address
	Reason     string
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultChallenged is a free log retrieval operation binding the contract event 0x703feb01415a2995816e8d082fd7aad0eacada1a2f63fdb3226e47f8a0285436.
//
// Solidity: event DkgResultChallenged(bytes32 indexed resultHash, address indexed challenger, string reason)
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgResultChallenged(opts *bind.FilterOpts, resultHash [][32]byte, challenger []common.Address) (*RandomBeaconDkgResultChallengedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var challengerRule []interface{}
	for _, challengerItem := range challenger {
		challengerRule = append(challengerRule, challengerItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgResultChallenged", resultHashRule, challengerRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgResultChallengedIterator{contract: _RandomBeacon.contract, event: "DkgResultChallenged", logs: logs, sub: sub}, nil
}

// WatchDkgResultChallenged is a free log subscription operation binding the contract event 0x703feb01415a2995816e8d082fd7aad0eacada1a2f63fdb3226e47f8a0285436.
//
// Solidity: event DkgResultChallenged(bytes32 indexed resultHash, address indexed challenger, string reason)
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgResultChallenged(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgResultChallenged, resultHash [][32]byte, challenger []common.Address) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var challengerRule []interface{}
	for _, challengerItem := range challenger {
		challengerRule = append(challengerRule, challengerItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgResultChallenged", resultHashRule, challengerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgResultChallenged)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgResultChallenged", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgResultChallenged(log types.Log) (*RandomBeaconDkgResultChallenged, error) {
	event := new(RandomBeaconDkgResultChallenged)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgResultChallenged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgResultSubmittedIterator is returned from FilterDkgResultSubmitted and is used to iterate over the raw logs and unpacked data for DkgResultSubmitted events raised by the RandomBeacon contract.
type RandomBeaconDkgResultSubmittedIterator struct {
	Event *RandomBeaconDkgResultSubmitted // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgResultSubmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgResultSubmitted)
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
		it.Event = new(RandomBeaconDkgResultSubmitted)
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
func (it *RandomBeaconDkgResultSubmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgResultSubmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgResultSubmitted represents a DkgResultSubmitted event raised by the RandomBeacon contract.
type RandomBeaconDkgResultSubmitted struct {
	ResultHash [32]byte
	Seed       *big.Int
	Result     BeaconDkgResult
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultSubmitted is a free log retrieval operation binding the contract event 0x8e7fd4293d7db11807147d8890c287fad3396fbb09a4e92273fc7856076c153a.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgResultSubmitted(opts *bind.FilterOpts, resultHash [][32]byte, seed []*big.Int) (*RandomBeaconDkgResultSubmittedIterator, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgResultSubmitted", resultHashRule, seedRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgResultSubmittedIterator{contract: _RandomBeacon.contract, event: "DkgResultSubmitted", logs: logs, sub: sub}, nil
}

// WatchDkgResultSubmitted is a free log subscription operation binding the contract event 0x8e7fd4293d7db11807147d8890c287fad3396fbb09a4e92273fc7856076c153a.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes,uint8[],bytes,uint256[],uint32[],bytes32) result)
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgResultSubmitted(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgResultSubmitted, resultHash [][32]byte, seed []*big.Int) (event.Subscription, error) {

	var resultHashRule []interface{}
	for _, resultHashItem := range resultHash {
		resultHashRule = append(resultHashRule, resultHashItem)
	}
	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgResultSubmitted", resultHashRule, seedRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgResultSubmitted)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgResultSubmitted", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgResultSubmitted(log types.Log) (*RandomBeaconDkgResultSubmitted, error) {
	event := new(RandomBeaconDkgResultSubmitted)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgResultSubmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgSeedTimedOutIterator is returned from FilterDkgSeedTimedOut and is used to iterate over the raw logs and unpacked data for DkgSeedTimedOut events raised by the RandomBeacon contract.
type RandomBeaconDkgSeedTimedOutIterator struct {
	Event *RandomBeaconDkgSeedTimedOut // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgSeedTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgSeedTimedOut)
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
		it.Event = new(RandomBeaconDkgSeedTimedOut)
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
func (it *RandomBeaconDkgSeedTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgSeedTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgSeedTimedOut represents a DkgSeedTimedOut event raised by the RandomBeacon contract.
type RandomBeaconDkgSeedTimedOut struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgSeedTimedOut is a free log retrieval operation binding the contract event 0x68c52f05452e81639fa06f379aee3178cddee4725521fff886f244c99e868b50.
//
// Solidity: event DkgSeedTimedOut()
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgSeedTimedOut(opts *bind.FilterOpts) (*RandomBeaconDkgSeedTimedOutIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgSeedTimedOut")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgSeedTimedOutIterator{contract: _RandomBeacon.contract, event: "DkgSeedTimedOut", logs: logs, sub: sub}, nil
}

// WatchDkgSeedTimedOut is a free log subscription operation binding the contract event 0x68c52f05452e81639fa06f379aee3178cddee4725521fff886f244c99e868b50.
//
// Solidity: event DkgSeedTimedOut()
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgSeedTimedOut(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgSeedTimedOut) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgSeedTimedOut")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgSeedTimedOut)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgSeedTimedOut", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgSeedTimedOut(log types.Log) (*RandomBeaconDkgSeedTimedOut, error) {
	event := new(RandomBeaconDkgSeedTimedOut)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgSeedTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgStartedIterator is returned from FilterDkgStarted and is used to iterate over the raw logs and unpacked data for DkgStarted events raised by the RandomBeacon contract.
type RandomBeaconDkgStartedIterator struct {
	Event *RandomBeaconDkgStarted // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgStartedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgStarted)
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
		it.Event = new(RandomBeaconDkgStarted)
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
func (it *RandomBeaconDkgStartedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgStartedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgStarted represents a DkgStarted event raised by the RandomBeacon contract.
type RandomBeaconDkgStarted struct {
	Seed *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterDkgStarted is a free log retrieval operation binding the contract event 0xb2ad26c2940889d79df2ee9c758a8aefa00c5ca90eee119af0e5d795df3b98bb.
//
// Solidity: event DkgStarted(uint256 indexed seed)
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgStarted(opts *bind.FilterOpts, seed []*big.Int) (*RandomBeaconDkgStartedIterator, error) {

	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgStarted", seedRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgStartedIterator{contract: _RandomBeacon.contract, event: "DkgStarted", logs: logs, sub: sub}, nil
}

// WatchDkgStarted is a free log subscription operation binding the contract event 0xb2ad26c2940889d79df2ee9c758a8aefa00c5ca90eee119af0e5d795df3b98bb.
//
// Solidity: event DkgStarted(uint256 indexed seed)
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgStarted(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgStarted, seed []*big.Int) (event.Subscription, error) {

	var seedRule []interface{}
	for _, seedItem := range seed {
		seedRule = append(seedRule, seedItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgStarted", seedRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgStarted)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgStarted", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgStarted(log types.Log) (*RandomBeaconDkgStarted, error) {
	event := new(RandomBeaconDkgStarted)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgStarted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgStateLockedIterator is returned from FilterDkgStateLocked and is used to iterate over the raw logs and unpacked data for DkgStateLocked events raised by the RandomBeacon contract.
type RandomBeaconDkgStateLockedIterator struct {
	Event *RandomBeaconDkgStateLocked // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgStateLockedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgStateLocked)
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
		it.Event = new(RandomBeaconDkgStateLocked)
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
func (it *RandomBeaconDkgStateLockedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgStateLockedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgStateLocked represents a DkgStateLocked event raised by the RandomBeacon contract.
type RandomBeaconDkgStateLocked struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgStateLocked is a free log retrieval operation binding the contract event 0x5c3ed2397d4d21298b2fb5027ac8e2d42e3c9c72bbb55ddb030e2a36a0cdff6b.
//
// Solidity: event DkgStateLocked()
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgStateLocked(opts *bind.FilterOpts) (*RandomBeaconDkgStateLockedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgStateLocked")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgStateLockedIterator{contract: _RandomBeacon.contract, event: "DkgStateLocked", logs: logs, sub: sub}, nil
}

// WatchDkgStateLocked is a free log subscription operation binding the contract event 0x5c3ed2397d4d21298b2fb5027ac8e2d42e3c9c72bbb55ddb030e2a36a0cdff6b.
//
// Solidity: event DkgStateLocked()
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgStateLocked(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgStateLocked) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgStateLocked")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgStateLocked)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgStateLocked", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgStateLocked(log types.Log) (*RandomBeaconDkgStateLocked, error) {
	event := new(RandomBeaconDkgStateLocked)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgStateLocked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconDkgTimedOutIterator is returned from FilterDkgTimedOut and is used to iterate over the raw logs and unpacked data for DkgTimedOut events raised by the RandomBeacon contract.
type RandomBeaconDkgTimedOutIterator struct {
	Event *RandomBeaconDkgTimedOut // Event containing the contract specifics and raw log

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
func (it *RandomBeaconDkgTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconDkgTimedOut)
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
		it.Event = new(RandomBeaconDkgTimedOut)
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
func (it *RandomBeaconDkgTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconDkgTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconDkgTimedOut represents a DkgTimedOut event raised by the RandomBeacon contract.
type RandomBeaconDkgTimedOut struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDkgTimedOut is a free log retrieval operation binding the contract event 0x2852b3e178dd281713b041c3d90b4815bb55b7ec812931d1e8e8d8bb2ed72d3e.
//
// Solidity: event DkgTimedOut()
func (_RandomBeacon *RandomBeaconFilterer) FilterDkgTimedOut(opts *bind.FilterOpts) (*RandomBeaconDkgTimedOutIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "DkgTimedOut")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconDkgTimedOutIterator{contract: _RandomBeacon.contract, event: "DkgTimedOut", logs: logs, sub: sub}, nil
}

// WatchDkgTimedOut is a free log subscription operation binding the contract event 0x2852b3e178dd281713b041c3d90b4815bb55b7ec812931d1e8e8d8bb2ed72d3e.
//
// Solidity: event DkgTimedOut()
func (_RandomBeacon *RandomBeaconFilterer) WatchDkgTimedOut(opts *bind.WatchOpts, sink chan<- *RandomBeaconDkgTimedOut) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "DkgTimedOut")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconDkgTimedOut)
				if err := _RandomBeacon.contract.UnpackLog(event, "DkgTimedOut", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseDkgTimedOut(log types.Log) (*RandomBeaconDkgTimedOut, error) {
	event := new(RandomBeaconDkgTimedOut)
	if err := _RandomBeacon.contract.UnpackLog(event, "DkgTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconGasParametersUpdatedIterator is returned from FilterGasParametersUpdated and is used to iterate over the raw logs and unpacked data for GasParametersUpdated events raised by the RandomBeacon contract.
type RandomBeaconGasParametersUpdatedIterator struct {
	Event *RandomBeaconGasParametersUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconGasParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconGasParametersUpdated)
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
		it.Event = new(RandomBeaconGasParametersUpdated)
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
func (it *RandomBeaconGasParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconGasParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconGasParametersUpdated represents a GasParametersUpdated event raised by the RandomBeacon contract.
type RandomBeaconGasParametersUpdated struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	RelayEntrySubmissionGasOffset     *big.Int
	Raw                               types.Log // Blockchain specific contextual infos
}

// FilterGasParametersUpdated is a free log retrieval operation binding the contract event 0xeffd28b20afc0bf9349e8d49d5346c7568689b661a6bcd71c26a939b57b0acd1.
//
// Solidity: event GasParametersUpdated(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset)
func (_RandomBeacon *RandomBeaconFilterer) FilterGasParametersUpdated(opts *bind.FilterOpts) (*RandomBeaconGasParametersUpdatedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "GasParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconGasParametersUpdatedIterator{contract: _RandomBeacon.contract, event: "GasParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchGasParametersUpdated is a free log subscription operation binding the contract event 0xeffd28b20afc0bf9349e8d49d5346c7568689b661a6bcd71c26a939b57b0acd1.
//
// Solidity: event GasParametersUpdated(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset)
func (_RandomBeacon *RandomBeaconFilterer) WatchGasParametersUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconGasParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "GasParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconGasParametersUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "GasParametersUpdated", log); err != nil {
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

// ParseGasParametersUpdated is a log parse operation binding the contract event 0xeffd28b20afc0bf9349e8d49d5346c7568689b661a6bcd71c26a939b57b0acd1.
//
// Solidity: event GasParametersUpdated(uint256 dkgResultSubmissionGas, uint256 dkgResultApprovalGasOffset, uint256 notifyOperatorInactivityGasOffset, uint256 relayEntrySubmissionGasOffset)
func (_RandomBeacon *RandomBeaconFilterer) ParseGasParametersUpdated(log types.Log) (*RandomBeaconGasParametersUpdated, error) {
	event := new(RandomBeaconGasParametersUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "GasParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconGovernanceTransferredIterator is returned from FilterGovernanceTransferred and is used to iterate over the raw logs and unpacked data for GovernanceTransferred events raised by the RandomBeacon contract.
type RandomBeaconGovernanceTransferredIterator struct {
	Event *RandomBeaconGovernanceTransferred // Event containing the contract specifics and raw log

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
func (it *RandomBeaconGovernanceTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconGovernanceTransferred)
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
		it.Event = new(RandomBeaconGovernanceTransferred)
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
func (it *RandomBeaconGovernanceTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconGovernanceTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconGovernanceTransferred represents a GovernanceTransferred event raised by the RandomBeacon contract.
type RandomBeaconGovernanceTransferred struct {
	OldGovernance common.Address
	NewGovernance common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterGovernanceTransferred is a free log retrieval operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_RandomBeacon *RandomBeaconFilterer) FilterGovernanceTransferred(opts *bind.FilterOpts) (*RandomBeaconGovernanceTransferredIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconGovernanceTransferredIterator{contract: _RandomBeacon.contract, event: "GovernanceTransferred", logs: logs, sub: sub}, nil
}

// WatchGovernanceTransferred is a free log subscription operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_RandomBeacon *RandomBeaconFilterer) WatchGovernanceTransferred(opts *bind.WatchOpts, sink chan<- *RandomBeaconGovernanceTransferred) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconGovernanceTransferred)
				if err := _RandomBeacon.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseGovernanceTransferred(log types.Log) (*RandomBeaconGovernanceTransferred, error) {
	event := new(RandomBeaconGovernanceTransferred)
	if err := _RandomBeacon.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconGroupCreationParametersUpdatedIterator is returned from FilterGroupCreationParametersUpdated and is used to iterate over the raw logs and unpacked data for GroupCreationParametersUpdated events raised by the RandomBeacon contract.
type RandomBeaconGroupCreationParametersUpdatedIterator struct {
	Event *RandomBeaconGroupCreationParametersUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconGroupCreationParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconGroupCreationParametersUpdated)
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
		it.Event = new(RandomBeaconGroupCreationParametersUpdated)
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
func (it *RandomBeaconGroupCreationParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconGroupCreationParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconGroupCreationParametersUpdated represents a GroupCreationParametersUpdated event raised by the RandomBeacon contract.
type RandomBeaconGroupCreationParametersUpdated struct {
	GroupCreationFrequency                   *big.Int
	GroupLifetime                            *big.Int
	DkgResultChallengePeriodLength           *big.Int
	DkgResultChallengeExtraGas               *big.Int
	DkgResultSubmissionTimeout               *big.Int
	DkgResultSubmitterPrecedencePeriodLength *big.Int
	Raw                                      types.Log // Blockchain specific contextual infos
}

// FilterGroupCreationParametersUpdated is a free log retrieval operation binding the contract event 0x88243406f452bee756e4fff4cb19e6855a5d3b7bfab8814ea6838685e08da093.
//
// Solidity: event GroupCreationParametersUpdated(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgResultSubmitterPrecedencePeriodLength)
func (_RandomBeacon *RandomBeaconFilterer) FilterGroupCreationParametersUpdated(opts *bind.FilterOpts) (*RandomBeaconGroupCreationParametersUpdatedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "GroupCreationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconGroupCreationParametersUpdatedIterator{contract: _RandomBeacon.contract, event: "GroupCreationParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchGroupCreationParametersUpdated is a free log subscription operation binding the contract event 0x88243406f452bee756e4fff4cb19e6855a5d3b7bfab8814ea6838685e08da093.
//
// Solidity: event GroupCreationParametersUpdated(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgResultSubmitterPrecedencePeriodLength)
func (_RandomBeacon *RandomBeaconFilterer) WatchGroupCreationParametersUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconGroupCreationParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "GroupCreationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconGroupCreationParametersUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "GroupCreationParametersUpdated", log); err != nil {
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

// ParseGroupCreationParametersUpdated is a log parse operation binding the contract event 0x88243406f452bee756e4fff4cb19e6855a5d3b7bfab8814ea6838685e08da093.
//
// Solidity: event GroupCreationParametersUpdated(uint256 groupCreationFrequency, uint256 groupLifetime, uint256 dkgResultChallengePeriodLength, uint256 dkgResultChallengeExtraGas, uint256 dkgResultSubmissionTimeout, uint256 dkgResultSubmitterPrecedencePeriodLength)
func (_RandomBeacon *RandomBeaconFilterer) ParseGroupCreationParametersUpdated(log types.Log) (*RandomBeaconGroupCreationParametersUpdated, error) {
	event := new(RandomBeaconGroupCreationParametersUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "GroupCreationParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconGroupRegisteredIterator is returned from FilterGroupRegistered and is used to iterate over the raw logs and unpacked data for GroupRegistered events raised by the RandomBeacon contract.
type RandomBeaconGroupRegisteredIterator struct {
	Event *RandomBeaconGroupRegistered // Event containing the contract specifics and raw log

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
func (it *RandomBeaconGroupRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconGroupRegistered)
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
		it.Event = new(RandomBeaconGroupRegistered)
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
func (it *RandomBeaconGroupRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconGroupRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconGroupRegistered represents a GroupRegistered event raised by the RandomBeacon contract.
type RandomBeaconGroupRegistered struct {
	GroupId     uint64
	GroupPubKey common.Hash
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterGroupRegistered is a free log retrieval operation binding the contract event 0x4c0666db2c51896711b100d1f4f5d20a347c59e6e16ad8f365ed81ff97358b4e.
//
// Solidity: event GroupRegistered(uint64 indexed groupId, bytes indexed groupPubKey)
func (_RandomBeacon *RandomBeaconFilterer) FilterGroupRegistered(opts *bind.FilterOpts, groupId []uint64, groupPubKey [][]byte) (*RandomBeaconGroupRegisteredIterator, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}
	var groupPubKeyRule []interface{}
	for _, groupPubKeyItem := range groupPubKey {
		groupPubKeyRule = append(groupPubKeyRule, groupPubKeyItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "GroupRegistered", groupIdRule, groupPubKeyRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconGroupRegisteredIterator{contract: _RandomBeacon.contract, event: "GroupRegistered", logs: logs, sub: sub}, nil
}

// WatchGroupRegistered is a free log subscription operation binding the contract event 0x4c0666db2c51896711b100d1f4f5d20a347c59e6e16ad8f365ed81ff97358b4e.
//
// Solidity: event GroupRegistered(uint64 indexed groupId, bytes indexed groupPubKey)
func (_RandomBeacon *RandomBeaconFilterer) WatchGroupRegistered(opts *bind.WatchOpts, sink chan<- *RandomBeaconGroupRegistered, groupId []uint64, groupPubKey [][]byte) (event.Subscription, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}
	var groupPubKeyRule []interface{}
	for _, groupPubKeyItem := range groupPubKey {
		groupPubKeyRule = append(groupPubKeyRule, groupPubKeyItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "GroupRegistered", groupIdRule, groupPubKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconGroupRegistered)
				if err := _RandomBeacon.contract.UnpackLog(event, "GroupRegistered", log); err != nil {
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

// ParseGroupRegistered is a log parse operation binding the contract event 0x4c0666db2c51896711b100d1f4f5d20a347c59e6e16ad8f365ed81ff97358b4e.
//
// Solidity: event GroupRegistered(uint64 indexed groupId, bytes indexed groupPubKey)
func (_RandomBeacon *RandomBeaconFilterer) ParseGroupRegistered(log types.Log) (*RandomBeaconGroupRegistered, error) {
	event := new(RandomBeaconGroupRegistered)
	if err := _RandomBeacon.contract.UnpackLog(event, "GroupRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconInactivityClaimedIterator is returned from FilterInactivityClaimed and is used to iterate over the raw logs and unpacked data for InactivityClaimed events raised by the RandomBeacon contract.
type RandomBeaconInactivityClaimedIterator struct {
	Event *RandomBeaconInactivityClaimed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconInactivityClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconInactivityClaimed)
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
		it.Event = new(RandomBeaconInactivityClaimed)
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
func (it *RandomBeaconInactivityClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconInactivityClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconInactivityClaimed represents a InactivityClaimed event raised by the RandomBeacon contract.
type RandomBeaconInactivityClaimed struct {
	GroupId  uint64
	Nonce    *big.Int
	Notifier common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterInactivityClaimed is a free log retrieval operation binding the contract event 0x3ca10b33fda0a4adbc75ac1939659a06e8c332b8f6a2792bedbf8741e4268417.
//
// Solidity: event InactivityClaimed(uint64 indexed groupId, uint256 nonce, address notifier)
func (_RandomBeacon *RandomBeaconFilterer) FilterInactivityClaimed(opts *bind.FilterOpts, groupId []uint64) (*RandomBeaconInactivityClaimedIterator, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "InactivityClaimed", groupIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconInactivityClaimedIterator{contract: _RandomBeacon.contract, event: "InactivityClaimed", logs: logs, sub: sub}, nil
}

// WatchInactivityClaimed is a free log subscription operation binding the contract event 0x3ca10b33fda0a4adbc75ac1939659a06e8c332b8f6a2792bedbf8741e4268417.
//
// Solidity: event InactivityClaimed(uint64 indexed groupId, uint256 nonce, address notifier)
func (_RandomBeacon *RandomBeaconFilterer) WatchInactivityClaimed(opts *bind.WatchOpts, sink chan<- *RandomBeaconInactivityClaimed, groupId []uint64) (event.Subscription, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "InactivityClaimed", groupIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconInactivityClaimed)
				if err := _RandomBeacon.contract.UnpackLog(event, "InactivityClaimed", log); err != nil {
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

// ParseInactivityClaimed is a log parse operation binding the contract event 0x3ca10b33fda0a4adbc75ac1939659a06e8c332b8f6a2792bedbf8741e4268417.
//
// Solidity: event InactivityClaimed(uint64 indexed groupId, uint256 nonce, address notifier)
func (_RandomBeacon *RandomBeaconFilterer) ParseInactivityClaimed(log types.Log) (*RandomBeaconInactivityClaimed, error) {
	event := new(RandomBeaconInactivityClaimed)
	if err := _RandomBeacon.contract.UnpackLog(event, "InactivityClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconInvoluntaryAuthorizationDecreaseFailedIterator is returned from FilterInvoluntaryAuthorizationDecreaseFailed and is used to iterate over the raw logs and unpacked data for InvoluntaryAuthorizationDecreaseFailed events raised by the RandomBeacon contract.
type RandomBeaconInvoluntaryAuthorizationDecreaseFailedIterator struct {
	Event *RandomBeaconInvoluntaryAuthorizationDecreaseFailed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconInvoluntaryAuthorizationDecreaseFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconInvoluntaryAuthorizationDecreaseFailed)
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
		it.Event = new(RandomBeaconInvoluntaryAuthorizationDecreaseFailed)
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
func (it *RandomBeaconInvoluntaryAuthorizationDecreaseFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconInvoluntaryAuthorizationDecreaseFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconInvoluntaryAuthorizationDecreaseFailed represents a InvoluntaryAuthorizationDecreaseFailed event raised by the RandomBeacon contract.
type RandomBeaconInvoluntaryAuthorizationDecreaseFailed struct {
	StakingProvider common.Address
	Operator        common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterInvoluntaryAuthorizationDecreaseFailed is a free log retrieval operation binding the contract event 0x1b09380d63e78fd72c1d79a805a7e2dfadf02b22418e24bebff51376b7df33b0.
//
// Solidity: event InvoluntaryAuthorizationDecreaseFailed(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_RandomBeacon *RandomBeaconFilterer) FilterInvoluntaryAuthorizationDecreaseFailed(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*RandomBeaconInvoluntaryAuthorizationDecreaseFailedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "InvoluntaryAuthorizationDecreaseFailed", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconInvoluntaryAuthorizationDecreaseFailedIterator{contract: _RandomBeacon.contract, event: "InvoluntaryAuthorizationDecreaseFailed", logs: logs, sub: sub}, nil
}

// WatchInvoluntaryAuthorizationDecreaseFailed is a free log subscription operation binding the contract event 0x1b09380d63e78fd72c1d79a805a7e2dfadf02b22418e24bebff51376b7df33b0.
//
// Solidity: event InvoluntaryAuthorizationDecreaseFailed(address indexed stakingProvider, address indexed operator, uint96 fromAmount, uint96 toAmount)
func (_RandomBeacon *RandomBeaconFilterer) WatchInvoluntaryAuthorizationDecreaseFailed(opts *bind.WatchOpts, sink chan<- *RandomBeaconInvoluntaryAuthorizationDecreaseFailed, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "InvoluntaryAuthorizationDecreaseFailed", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconInvoluntaryAuthorizationDecreaseFailed)
				if err := _RandomBeacon.contract.UnpackLog(event, "InvoluntaryAuthorizationDecreaseFailed", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseInvoluntaryAuthorizationDecreaseFailed(log types.Log) (*RandomBeaconInvoluntaryAuthorizationDecreaseFailed, error) {
	event := new(RandomBeaconInvoluntaryAuthorizationDecreaseFailed)
	if err := _RandomBeacon.contract.UnpackLog(event, "InvoluntaryAuthorizationDecreaseFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconOperatorJoinedSortitionPoolIterator is returned from FilterOperatorJoinedSortitionPool and is used to iterate over the raw logs and unpacked data for OperatorJoinedSortitionPool events raised by the RandomBeacon contract.
type RandomBeaconOperatorJoinedSortitionPoolIterator struct {
	Event *RandomBeaconOperatorJoinedSortitionPool // Event containing the contract specifics and raw log

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
func (it *RandomBeaconOperatorJoinedSortitionPoolIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconOperatorJoinedSortitionPool)
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
		it.Event = new(RandomBeaconOperatorJoinedSortitionPool)
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
func (it *RandomBeaconOperatorJoinedSortitionPoolIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconOperatorJoinedSortitionPoolIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconOperatorJoinedSortitionPool represents a OperatorJoinedSortitionPool event raised by the RandomBeacon contract.
type RandomBeaconOperatorJoinedSortitionPool struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorJoinedSortitionPool is a free log retrieval operation binding the contract event 0x5075aaa89894a888eb2cac81a27320c60855febb0cf1706b66bdc754e640d433.
//
// Solidity: event OperatorJoinedSortitionPool(address indexed stakingProvider, address indexed operator)
func (_RandomBeacon *RandomBeaconFilterer) FilterOperatorJoinedSortitionPool(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*RandomBeaconOperatorJoinedSortitionPoolIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "OperatorJoinedSortitionPool", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconOperatorJoinedSortitionPoolIterator{contract: _RandomBeacon.contract, event: "OperatorJoinedSortitionPool", logs: logs, sub: sub}, nil
}

// WatchOperatorJoinedSortitionPool is a free log subscription operation binding the contract event 0x5075aaa89894a888eb2cac81a27320c60855febb0cf1706b66bdc754e640d433.
//
// Solidity: event OperatorJoinedSortitionPool(address indexed stakingProvider, address indexed operator)
func (_RandomBeacon *RandomBeaconFilterer) WatchOperatorJoinedSortitionPool(opts *bind.WatchOpts, sink chan<- *RandomBeaconOperatorJoinedSortitionPool, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "OperatorJoinedSortitionPool", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconOperatorJoinedSortitionPool)
				if err := _RandomBeacon.contract.UnpackLog(event, "OperatorJoinedSortitionPool", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseOperatorJoinedSortitionPool(log types.Log) (*RandomBeaconOperatorJoinedSortitionPool, error) {
	event := new(RandomBeaconOperatorJoinedSortitionPool)
	if err := _RandomBeacon.contract.UnpackLog(event, "OperatorJoinedSortitionPool", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconOperatorRegisteredIterator is returned from FilterOperatorRegistered and is used to iterate over the raw logs and unpacked data for OperatorRegistered events raised by the RandomBeacon contract.
type RandomBeaconOperatorRegisteredIterator struct {
	Event *RandomBeaconOperatorRegistered // Event containing the contract specifics and raw log

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
func (it *RandomBeaconOperatorRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconOperatorRegistered)
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
		it.Event = new(RandomBeaconOperatorRegistered)
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
func (it *RandomBeaconOperatorRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconOperatorRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconOperatorRegistered represents a OperatorRegistered event raised by the RandomBeacon contract.
type RandomBeaconOperatorRegistered struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorRegistered is a free log retrieval operation binding the contract event 0xa453db612af59e5521d6ab9284dc3e2d06af286eb1b1b7b771fce4716c19f2c1.
//
// Solidity: event OperatorRegistered(address indexed stakingProvider, address indexed operator)
func (_RandomBeacon *RandomBeaconFilterer) FilterOperatorRegistered(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*RandomBeaconOperatorRegisteredIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "OperatorRegistered", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconOperatorRegisteredIterator{contract: _RandomBeacon.contract, event: "OperatorRegistered", logs: logs, sub: sub}, nil
}

// WatchOperatorRegistered is a free log subscription operation binding the contract event 0xa453db612af59e5521d6ab9284dc3e2d06af286eb1b1b7b771fce4716c19f2c1.
//
// Solidity: event OperatorRegistered(address indexed stakingProvider, address indexed operator)
func (_RandomBeacon *RandomBeaconFilterer) WatchOperatorRegistered(opts *bind.WatchOpts, sink chan<- *RandomBeaconOperatorRegistered, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "OperatorRegistered", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconOperatorRegistered)
				if err := _RandomBeacon.contract.UnpackLog(event, "OperatorRegistered", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseOperatorRegistered(log types.Log) (*RandomBeaconOperatorRegistered, error) {
	event := new(RandomBeaconOperatorRegistered)
	if err := _RandomBeacon.contract.UnpackLog(event, "OperatorRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconOperatorStatusUpdatedIterator is returned from FilterOperatorStatusUpdated and is used to iterate over the raw logs and unpacked data for OperatorStatusUpdated events raised by the RandomBeacon contract.
type RandomBeaconOperatorStatusUpdatedIterator struct {
	Event *RandomBeaconOperatorStatusUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconOperatorStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconOperatorStatusUpdated)
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
		it.Event = new(RandomBeaconOperatorStatusUpdated)
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
func (it *RandomBeaconOperatorStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconOperatorStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconOperatorStatusUpdated represents a OperatorStatusUpdated event raised by the RandomBeacon contract.
type RandomBeaconOperatorStatusUpdated struct {
	StakingProvider common.Address
	Operator        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOperatorStatusUpdated is a free log retrieval operation binding the contract event 0x1231fe5ee649a593b524a494cd53146a196380a872115a0d0fe16c0735afdf26.
//
// Solidity: event OperatorStatusUpdated(address indexed stakingProvider, address indexed operator)
func (_RandomBeacon *RandomBeaconFilterer) FilterOperatorStatusUpdated(opts *bind.FilterOpts, stakingProvider []common.Address, operator []common.Address) (*RandomBeaconOperatorStatusUpdatedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "OperatorStatusUpdated", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconOperatorStatusUpdatedIterator{contract: _RandomBeacon.contract, event: "OperatorStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchOperatorStatusUpdated is a free log subscription operation binding the contract event 0x1231fe5ee649a593b524a494cd53146a196380a872115a0d0fe16c0735afdf26.
//
// Solidity: event OperatorStatusUpdated(address indexed stakingProvider, address indexed operator)
func (_RandomBeacon *RandomBeaconFilterer) WatchOperatorStatusUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconOperatorStatusUpdated, stakingProvider []common.Address, operator []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "OperatorStatusUpdated", stakingProviderRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconOperatorStatusUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "OperatorStatusUpdated", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseOperatorStatusUpdated(log types.Log) (*RandomBeaconOperatorStatusUpdated, error) {
	event := new(RandomBeaconOperatorStatusUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "OperatorStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconReimbursementPoolUpdatedIterator is returned from FilterReimbursementPoolUpdated and is used to iterate over the raw logs and unpacked data for ReimbursementPoolUpdated events raised by the RandomBeacon contract.
type RandomBeaconReimbursementPoolUpdatedIterator struct {
	Event *RandomBeaconReimbursementPoolUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconReimbursementPoolUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconReimbursementPoolUpdated)
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
		it.Event = new(RandomBeaconReimbursementPoolUpdated)
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
func (it *RandomBeaconReimbursementPoolUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconReimbursementPoolUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconReimbursementPoolUpdated represents a ReimbursementPoolUpdated event raised by the RandomBeacon contract.
type RandomBeaconReimbursementPoolUpdated struct {
	NewReimbursementPool common.Address
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterReimbursementPoolUpdated is a free log retrieval operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_RandomBeacon *RandomBeaconFilterer) FilterReimbursementPoolUpdated(opts *bind.FilterOpts) (*RandomBeaconReimbursementPoolUpdatedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconReimbursementPoolUpdatedIterator{contract: _RandomBeacon.contract, event: "ReimbursementPoolUpdated", logs: logs, sub: sub}, nil
}

// WatchReimbursementPoolUpdated is a free log subscription operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_RandomBeacon *RandomBeaconFilterer) WatchReimbursementPoolUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconReimbursementPoolUpdated) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconReimbursementPoolUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseReimbursementPoolUpdated(log types.Log) (*RandomBeaconReimbursementPoolUpdated, error) {
	event := new(RandomBeaconReimbursementPoolUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntryDelaySlashedIterator is returned from FilterRelayEntryDelaySlashed and is used to iterate over the raw logs and unpacked data for RelayEntryDelaySlashed events raised by the RandomBeacon contract.
type RandomBeaconRelayEntryDelaySlashedIterator struct {
	Event *RandomBeaconRelayEntryDelaySlashed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntryDelaySlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntryDelaySlashed)
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
		it.Event = new(RandomBeaconRelayEntryDelaySlashed)
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
func (it *RandomBeaconRelayEntryDelaySlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntryDelaySlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntryDelaySlashed represents a RelayEntryDelaySlashed event raised by the RandomBeacon contract.
type RandomBeaconRelayEntryDelaySlashed struct {
	RequestId      *big.Int
	SlashingAmount *big.Int
	GroupMembers   []common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterRelayEntryDelaySlashed is a free log retrieval operation binding the contract event 0x94af8e9c35b4ede2a77f659b202b1efe096bf99f0e6f5dd5905c800978a9a647.
//
// Solidity: event RelayEntryDelaySlashed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntryDelaySlashed(opts *bind.FilterOpts, requestId []*big.Int) (*RandomBeaconRelayEntryDelaySlashedIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntryDelaySlashed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntryDelaySlashedIterator{contract: _RandomBeacon.contract, event: "RelayEntryDelaySlashed", logs: logs, sub: sub}, nil
}

// WatchRelayEntryDelaySlashed is a free log subscription operation binding the contract event 0x94af8e9c35b4ede2a77f659b202b1efe096bf99f0e6f5dd5905c800978a9a647.
//
// Solidity: event RelayEntryDelaySlashed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntryDelaySlashed(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntryDelaySlashed, requestId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntryDelaySlashed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntryDelaySlashed)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryDelaySlashed", log); err != nil {
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

// ParseRelayEntryDelaySlashed is a log parse operation binding the contract event 0x94af8e9c35b4ede2a77f659b202b1efe096bf99f0e6f5dd5905c800978a9a647.
//
// Solidity: event RelayEntryDelaySlashed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntryDelaySlashed(log types.Log) (*RandomBeaconRelayEntryDelaySlashed, error) {
	event := new(RandomBeaconRelayEntryDelaySlashed)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryDelaySlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntryDelaySlashingFailedIterator is returned from FilterRelayEntryDelaySlashingFailed and is used to iterate over the raw logs and unpacked data for RelayEntryDelaySlashingFailed events raised by the RandomBeacon contract.
type RandomBeaconRelayEntryDelaySlashingFailedIterator struct {
	Event *RandomBeaconRelayEntryDelaySlashingFailed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntryDelaySlashingFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntryDelaySlashingFailed)
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
		it.Event = new(RandomBeaconRelayEntryDelaySlashingFailed)
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
func (it *RandomBeaconRelayEntryDelaySlashingFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntryDelaySlashingFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntryDelaySlashingFailed represents a RelayEntryDelaySlashingFailed event raised by the RandomBeacon contract.
type RandomBeaconRelayEntryDelaySlashingFailed struct {
	RequestId      *big.Int
	SlashingAmount *big.Int
	GroupMembers   []common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterRelayEntryDelaySlashingFailed is a free log retrieval operation binding the contract event 0xd3f7d9c595537a55b26b224409f386868056764bc4d55bf8c3d86e20d047afc1.
//
// Solidity: event RelayEntryDelaySlashingFailed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntryDelaySlashingFailed(opts *bind.FilterOpts, requestId []*big.Int) (*RandomBeaconRelayEntryDelaySlashingFailedIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntryDelaySlashingFailed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntryDelaySlashingFailedIterator{contract: _RandomBeacon.contract, event: "RelayEntryDelaySlashingFailed", logs: logs, sub: sub}, nil
}

// WatchRelayEntryDelaySlashingFailed is a free log subscription operation binding the contract event 0xd3f7d9c595537a55b26b224409f386868056764bc4d55bf8c3d86e20d047afc1.
//
// Solidity: event RelayEntryDelaySlashingFailed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntryDelaySlashingFailed(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntryDelaySlashingFailed, requestId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntryDelaySlashingFailed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntryDelaySlashingFailed)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryDelaySlashingFailed", log); err != nil {
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

// ParseRelayEntryDelaySlashingFailed is a log parse operation binding the contract event 0xd3f7d9c595537a55b26b224409f386868056764bc4d55bf8c3d86e20d047afc1.
//
// Solidity: event RelayEntryDelaySlashingFailed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntryDelaySlashingFailed(log types.Log) (*RandomBeaconRelayEntryDelaySlashingFailed, error) {
	event := new(RandomBeaconRelayEntryDelaySlashingFailed)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryDelaySlashingFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntryParametersUpdatedIterator is returned from FilterRelayEntryParametersUpdated and is used to iterate over the raw logs and unpacked data for RelayEntryParametersUpdated events raised by the RandomBeacon contract.
type RandomBeaconRelayEntryParametersUpdatedIterator struct {
	Event *RandomBeaconRelayEntryParametersUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntryParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntryParametersUpdated)
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
		it.Event = new(RandomBeaconRelayEntryParametersUpdated)
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
func (it *RandomBeaconRelayEntryParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntryParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntryParametersUpdated represents a RelayEntryParametersUpdated event raised by the RandomBeacon contract.
type RandomBeaconRelayEntryParametersUpdated struct {
	RelayEntrySoftTimeout *big.Int
	RelayEntryHardTimeout *big.Int
	CallbackGasLimit      *big.Int
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterRelayEntryParametersUpdated is a free log retrieval operation binding the contract event 0xea9006ae23cd9b51dbfbc9d747fc3b0bc77acba4fefba609c76f8e9a9513602e.
//
// Solidity: event RelayEntryParametersUpdated(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntryParametersUpdated(opts *bind.FilterOpts) (*RandomBeaconRelayEntryParametersUpdatedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntryParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntryParametersUpdatedIterator{contract: _RandomBeacon.contract, event: "RelayEntryParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchRelayEntryParametersUpdated is a free log subscription operation binding the contract event 0xea9006ae23cd9b51dbfbc9d747fc3b0bc77acba4fefba609c76f8e9a9513602e.
//
// Solidity: event RelayEntryParametersUpdated(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntryParametersUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntryParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntryParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntryParametersUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryParametersUpdated", log); err != nil {
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

// ParseRelayEntryParametersUpdated is a log parse operation binding the contract event 0xea9006ae23cd9b51dbfbc9d747fc3b0bc77acba4fefba609c76f8e9a9513602e.
//
// Solidity: event RelayEntryParametersUpdated(uint256 relayEntrySoftTimeout, uint256 relayEntryHardTimeout, uint256 callbackGasLimit)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntryParametersUpdated(log types.Log) (*RandomBeaconRelayEntryParametersUpdated, error) {
	event := new(RandomBeaconRelayEntryParametersUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntryRequestedIterator is returned from FilterRelayEntryRequested and is used to iterate over the raw logs and unpacked data for RelayEntryRequested events raised by the RandomBeacon contract.
type RandomBeaconRelayEntryRequestedIterator struct {
	Event *RandomBeaconRelayEntryRequested // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntryRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntryRequested)
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
		it.Event = new(RandomBeaconRelayEntryRequested)
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
func (it *RandomBeaconRelayEntryRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntryRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntryRequested represents a RelayEntryRequested event raised by the RandomBeacon contract.
type RandomBeaconRelayEntryRequested struct {
	RequestId     *big.Int
	GroupId       uint64
	PreviousEntry []byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterRelayEntryRequested is a free log retrieval operation binding the contract event 0xdf86f752d80d21879cbd208ba1d036c03f836b1d7f8f887b0b9e0b63f8a49f5d.
//
// Solidity: event RelayEntryRequested(uint256 indexed requestId, uint64 groupId, bytes previousEntry)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntryRequested(opts *bind.FilterOpts, requestId []*big.Int) (*RandomBeaconRelayEntryRequestedIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntryRequested", requestIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntryRequestedIterator{contract: _RandomBeacon.contract, event: "RelayEntryRequested", logs: logs, sub: sub}, nil
}

// WatchRelayEntryRequested is a free log subscription operation binding the contract event 0xdf86f752d80d21879cbd208ba1d036c03f836b1d7f8f887b0b9e0b63f8a49f5d.
//
// Solidity: event RelayEntryRequested(uint256 indexed requestId, uint64 groupId, bytes previousEntry)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntryRequested(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntryRequested, requestId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntryRequested", requestIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntryRequested)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryRequested", log); err != nil {
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

// ParseRelayEntryRequested is a log parse operation binding the contract event 0xdf86f752d80d21879cbd208ba1d036c03f836b1d7f8f887b0b9e0b63f8a49f5d.
//
// Solidity: event RelayEntryRequested(uint256 indexed requestId, uint64 groupId, bytes previousEntry)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntryRequested(log types.Log) (*RandomBeaconRelayEntryRequested, error) {
	event := new(RandomBeaconRelayEntryRequested)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntrySubmittedIterator is returned from FilterRelayEntrySubmitted and is used to iterate over the raw logs and unpacked data for RelayEntrySubmitted events raised by the RandomBeacon contract.
type RandomBeaconRelayEntrySubmittedIterator struct {
	Event *RandomBeaconRelayEntrySubmitted // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntrySubmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntrySubmitted)
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
		it.Event = new(RandomBeaconRelayEntrySubmitted)
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
func (it *RandomBeaconRelayEntrySubmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntrySubmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntrySubmitted represents a RelayEntrySubmitted event raised by the RandomBeacon contract.
type RandomBeaconRelayEntrySubmitted struct {
	RequestId *big.Int
	Submitter common.Address
	Entry     []byte
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterRelayEntrySubmitted is a free log retrieval operation binding the contract event 0x087df9fd862e4448d02f8b58e33a18941769c795a8b83cfb95dfaf9c92ca897e.
//
// Solidity: event RelayEntrySubmitted(uint256 indexed requestId, address submitter, bytes entry)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntrySubmitted(opts *bind.FilterOpts, requestId []*big.Int) (*RandomBeaconRelayEntrySubmittedIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntrySubmitted", requestIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntrySubmittedIterator{contract: _RandomBeacon.contract, event: "RelayEntrySubmitted", logs: logs, sub: sub}, nil
}

// WatchRelayEntrySubmitted is a free log subscription operation binding the contract event 0x087df9fd862e4448d02f8b58e33a18941769c795a8b83cfb95dfaf9c92ca897e.
//
// Solidity: event RelayEntrySubmitted(uint256 indexed requestId, address submitter, bytes entry)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntrySubmitted(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntrySubmitted, requestId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntrySubmitted", requestIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntrySubmitted)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntrySubmitted", log); err != nil {
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

// ParseRelayEntrySubmitted is a log parse operation binding the contract event 0x087df9fd862e4448d02f8b58e33a18941769c795a8b83cfb95dfaf9c92ca897e.
//
// Solidity: event RelayEntrySubmitted(uint256 indexed requestId, address submitter, bytes entry)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntrySubmitted(log types.Log) (*RandomBeaconRelayEntrySubmitted, error) {
	event := new(RandomBeaconRelayEntrySubmitted)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntrySubmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntryTimedOutIterator is returned from FilterRelayEntryTimedOut and is used to iterate over the raw logs and unpacked data for RelayEntryTimedOut events raised by the RandomBeacon contract.
type RandomBeaconRelayEntryTimedOutIterator struct {
	Event *RandomBeaconRelayEntryTimedOut // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntryTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntryTimedOut)
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
		it.Event = new(RandomBeaconRelayEntryTimedOut)
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
func (it *RandomBeaconRelayEntryTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntryTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntryTimedOut represents a RelayEntryTimedOut event raised by the RandomBeacon contract.
type RandomBeaconRelayEntryTimedOut struct {
	RequestId         *big.Int
	TerminatedGroupId uint64
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRelayEntryTimedOut is a free log retrieval operation binding the contract event 0xc28be0fa97c9083fa79881ba8950c1333fb45ec5100d2b893ec0f41544e78bef.
//
// Solidity: event RelayEntryTimedOut(uint256 indexed requestId, uint64 terminatedGroupId)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntryTimedOut(opts *bind.FilterOpts, requestId []*big.Int) (*RandomBeaconRelayEntryTimedOutIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntryTimedOut", requestIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntryTimedOutIterator{contract: _RandomBeacon.contract, event: "RelayEntryTimedOut", logs: logs, sub: sub}, nil
}

// WatchRelayEntryTimedOut is a free log subscription operation binding the contract event 0xc28be0fa97c9083fa79881ba8950c1333fb45ec5100d2b893ec0f41544e78bef.
//
// Solidity: event RelayEntryTimedOut(uint256 indexed requestId, uint64 terminatedGroupId)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntryTimedOut(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntryTimedOut, requestId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntryTimedOut", requestIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntryTimedOut)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryTimedOut", log); err != nil {
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

// ParseRelayEntryTimedOut is a log parse operation binding the contract event 0xc28be0fa97c9083fa79881ba8950c1333fb45ec5100d2b893ec0f41544e78bef.
//
// Solidity: event RelayEntryTimedOut(uint256 indexed requestId, uint64 terminatedGroupId)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntryTimedOut(log types.Log) (*RandomBeaconRelayEntryTimedOut, error) {
	event := new(RandomBeaconRelayEntryTimedOut)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntryTimeoutSlashedIterator is returned from FilterRelayEntryTimeoutSlashed and is used to iterate over the raw logs and unpacked data for RelayEntryTimeoutSlashed events raised by the RandomBeacon contract.
type RandomBeaconRelayEntryTimeoutSlashedIterator struct {
	Event *RandomBeaconRelayEntryTimeoutSlashed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntryTimeoutSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntryTimeoutSlashed)
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
		it.Event = new(RandomBeaconRelayEntryTimeoutSlashed)
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
func (it *RandomBeaconRelayEntryTimeoutSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntryTimeoutSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntryTimeoutSlashed represents a RelayEntryTimeoutSlashed event raised by the RandomBeacon contract.
type RandomBeaconRelayEntryTimeoutSlashed struct {
	RequestId      *big.Int
	SlashingAmount *big.Int
	GroupMembers   []common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterRelayEntryTimeoutSlashed is a free log retrieval operation binding the contract event 0x6e1369aebaf86903bf1f0ab69c3edd0cadbf0534635f699f57a74a3ce54d3718.
//
// Solidity: event RelayEntryTimeoutSlashed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntryTimeoutSlashed(opts *bind.FilterOpts, requestId []*big.Int) (*RandomBeaconRelayEntryTimeoutSlashedIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntryTimeoutSlashed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntryTimeoutSlashedIterator{contract: _RandomBeacon.contract, event: "RelayEntryTimeoutSlashed", logs: logs, sub: sub}, nil
}

// WatchRelayEntryTimeoutSlashed is a free log subscription operation binding the contract event 0x6e1369aebaf86903bf1f0ab69c3edd0cadbf0534635f699f57a74a3ce54d3718.
//
// Solidity: event RelayEntryTimeoutSlashed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntryTimeoutSlashed(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntryTimeoutSlashed, requestId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntryTimeoutSlashed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntryTimeoutSlashed)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryTimeoutSlashed", log); err != nil {
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

// ParseRelayEntryTimeoutSlashed is a log parse operation binding the contract event 0x6e1369aebaf86903bf1f0ab69c3edd0cadbf0534635f699f57a74a3ce54d3718.
//
// Solidity: event RelayEntryTimeoutSlashed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntryTimeoutSlashed(log types.Log) (*RandomBeaconRelayEntryTimeoutSlashed, error) {
	event := new(RandomBeaconRelayEntryTimeoutSlashed)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryTimeoutSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRelayEntryTimeoutSlashingFailedIterator is returned from FilterRelayEntryTimeoutSlashingFailed and is used to iterate over the raw logs and unpacked data for RelayEntryTimeoutSlashingFailed events raised by the RandomBeacon contract.
type RandomBeaconRelayEntryTimeoutSlashingFailedIterator struct {
	Event *RandomBeaconRelayEntryTimeoutSlashingFailed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRelayEntryTimeoutSlashingFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRelayEntryTimeoutSlashingFailed)
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
		it.Event = new(RandomBeaconRelayEntryTimeoutSlashingFailed)
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
func (it *RandomBeaconRelayEntryTimeoutSlashingFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRelayEntryTimeoutSlashingFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRelayEntryTimeoutSlashingFailed represents a RelayEntryTimeoutSlashingFailed event raised by the RandomBeacon contract.
type RandomBeaconRelayEntryTimeoutSlashingFailed struct {
	RequestId      *big.Int
	SlashingAmount *big.Int
	GroupMembers   []common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterRelayEntryTimeoutSlashingFailed is a free log retrieval operation binding the contract event 0x678dcbc52328cedef940fe45ab75280e81c09ec03fe55df62e67642aa18bc278.
//
// Solidity: event RelayEntryTimeoutSlashingFailed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) FilterRelayEntryTimeoutSlashingFailed(opts *bind.FilterOpts, requestId []*big.Int) (*RandomBeaconRelayEntryTimeoutSlashingFailedIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RelayEntryTimeoutSlashingFailed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRelayEntryTimeoutSlashingFailedIterator{contract: _RandomBeacon.contract, event: "RelayEntryTimeoutSlashingFailed", logs: logs, sub: sub}, nil
}

// WatchRelayEntryTimeoutSlashingFailed is a free log subscription operation binding the contract event 0x678dcbc52328cedef940fe45ab75280e81c09ec03fe55df62e67642aa18bc278.
//
// Solidity: event RelayEntryTimeoutSlashingFailed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) WatchRelayEntryTimeoutSlashingFailed(opts *bind.WatchOpts, sink chan<- *RandomBeaconRelayEntryTimeoutSlashingFailed, requestId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RelayEntryTimeoutSlashingFailed", requestIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRelayEntryTimeoutSlashingFailed)
				if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryTimeoutSlashingFailed", log); err != nil {
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

// ParseRelayEntryTimeoutSlashingFailed is a log parse operation binding the contract event 0x678dcbc52328cedef940fe45ab75280e81c09ec03fe55df62e67642aa18bc278.
//
// Solidity: event RelayEntryTimeoutSlashingFailed(uint256 indexed requestId, uint256 slashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) ParseRelayEntryTimeoutSlashingFailed(log types.Log) (*RandomBeaconRelayEntryTimeoutSlashingFailed, error) {
	event := new(RandomBeaconRelayEntryTimeoutSlashingFailed)
	if err := _RandomBeacon.contract.UnpackLog(event, "RelayEntryTimeoutSlashingFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRequesterAuthorizationUpdatedIterator is returned from FilterRequesterAuthorizationUpdated and is used to iterate over the raw logs and unpacked data for RequesterAuthorizationUpdated events raised by the RandomBeacon contract.
type RandomBeaconRequesterAuthorizationUpdatedIterator struct {
	Event *RandomBeaconRequesterAuthorizationUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRequesterAuthorizationUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRequesterAuthorizationUpdated)
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
		it.Event = new(RandomBeaconRequesterAuthorizationUpdated)
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
func (it *RandomBeaconRequesterAuthorizationUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRequesterAuthorizationUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRequesterAuthorizationUpdated represents a RequesterAuthorizationUpdated event raised by the RandomBeacon contract.
type RandomBeaconRequesterAuthorizationUpdated struct {
	Requester    common.Address
	IsAuthorized bool
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterRequesterAuthorizationUpdated is a free log retrieval operation binding the contract event 0xb4d9db68405970190721d2b3726fc9f728c1413038755e60518dea67d5545556.
//
// Solidity: event RequesterAuthorizationUpdated(address indexed requester, bool isAuthorized)
func (_RandomBeacon *RandomBeaconFilterer) FilterRequesterAuthorizationUpdated(opts *bind.FilterOpts, requester []common.Address) (*RandomBeaconRequesterAuthorizationUpdatedIterator, error) {

	var requesterRule []interface{}
	for _, requesterItem := range requester {
		requesterRule = append(requesterRule, requesterItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RequesterAuthorizationUpdated", requesterRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRequesterAuthorizationUpdatedIterator{contract: _RandomBeacon.contract, event: "RequesterAuthorizationUpdated", logs: logs, sub: sub}, nil
}

// WatchRequesterAuthorizationUpdated is a free log subscription operation binding the contract event 0xb4d9db68405970190721d2b3726fc9f728c1413038755e60518dea67d5545556.
//
// Solidity: event RequesterAuthorizationUpdated(address indexed requester, bool isAuthorized)
func (_RandomBeacon *RandomBeaconFilterer) WatchRequesterAuthorizationUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconRequesterAuthorizationUpdated, requester []common.Address) (event.Subscription, error) {

	var requesterRule []interface{}
	for _, requesterItem := range requester {
		requesterRule = append(requesterRule, requesterItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RequesterAuthorizationUpdated", requesterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRequesterAuthorizationUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "RequesterAuthorizationUpdated", log); err != nil {
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

// ParseRequesterAuthorizationUpdated is a log parse operation binding the contract event 0xb4d9db68405970190721d2b3726fc9f728c1413038755e60518dea67d5545556.
//
// Solidity: event RequesterAuthorizationUpdated(address indexed requester, bool isAuthorized)
func (_RandomBeacon *RandomBeaconFilterer) ParseRequesterAuthorizationUpdated(log types.Log) (*RandomBeaconRequesterAuthorizationUpdated, error) {
	event := new(RandomBeaconRequesterAuthorizationUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "RequesterAuthorizationUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRewardParametersUpdatedIterator is returned from FilterRewardParametersUpdated and is used to iterate over the raw logs and unpacked data for RewardParametersUpdated events raised by the RandomBeacon contract.
type RandomBeaconRewardParametersUpdatedIterator struct {
	Event *RandomBeaconRewardParametersUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRewardParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRewardParametersUpdated)
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
		it.Event = new(RandomBeaconRewardParametersUpdated)
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
func (it *RandomBeaconRewardParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRewardParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRewardParametersUpdated represents a RewardParametersUpdated event raised by the RandomBeacon contract.
type RandomBeaconRewardParametersUpdated struct {
	SortitionPoolRewardsBanDuration                 *big.Int
	RelayEntryTimeoutNotificationRewardMultiplier   *big.Int
	UnauthorizedSigningNotificationRewardMultiplier *big.Int
	DkgMaliciousResultNotificationRewardMultiplier  *big.Int
	Raw                                             types.Log // Blockchain specific contextual infos
}

// FilterRewardParametersUpdated is a free log retrieval operation binding the contract event 0xbe6727aab9b80431985c18deef80d48599397c63884c62e96959f82abd16f16d.
//
// Solidity: event RewardParametersUpdated(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier)
func (_RandomBeacon *RandomBeaconFilterer) FilterRewardParametersUpdated(opts *bind.FilterOpts) (*RandomBeaconRewardParametersUpdatedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RewardParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRewardParametersUpdatedIterator{contract: _RandomBeacon.contract, event: "RewardParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchRewardParametersUpdated is a free log subscription operation binding the contract event 0xbe6727aab9b80431985c18deef80d48599397c63884c62e96959f82abd16f16d.
//
// Solidity: event RewardParametersUpdated(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier)
func (_RandomBeacon *RandomBeaconFilterer) WatchRewardParametersUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconRewardParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RewardParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRewardParametersUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "RewardParametersUpdated", log); err != nil {
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

// ParseRewardParametersUpdated is a log parse operation binding the contract event 0xbe6727aab9b80431985c18deef80d48599397c63884c62e96959f82abd16f16d.
//
// Solidity: event RewardParametersUpdated(uint256 sortitionPoolRewardsBanDuration, uint256 relayEntryTimeoutNotificationRewardMultiplier, uint256 unauthorizedSigningNotificationRewardMultiplier, uint256 dkgMaliciousResultNotificationRewardMultiplier)
func (_RandomBeacon *RandomBeaconFilterer) ParseRewardParametersUpdated(log types.Log) (*RandomBeaconRewardParametersUpdated, error) {
	event := new(RandomBeaconRewardParametersUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "RewardParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconRewardsWithdrawnIterator is returned from FilterRewardsWithdrawn and is used to iterate over the raw logs and unpacked data for RewardsWithdrawn events raised by the RandomBeacon contract.
type RandomBeaconRewardsWithdrawnIterator struct {
	Event *RandomBeaconRewardsWithdrawn // Event containing the contract specifics and raw log

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
func (it *RandomBeaconRewardsWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconRewardsWithdrawn)
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
		it.Event = new(RandomBeaconRewardsWithdrawn)
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
func (it *RandomBeaconRewardsWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconRewardsWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconRewardsWithdrawn represents a RewardsWithdrawn event raised by the RandomBeacon contract.
type RandomBeaconRewardsWithdrawn struct {
	StakingProvider common.Address
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterRewardsWithdrawn is a free log retrieval operation binding the contract event 0x38532b6dea69d7266fa923c7813d190be37625f2454ddfa3d93c45c79482e3fd.
//
// Solidity: event RewardsWithdrawn(address indexed stakingProvider, uint96 amount)
func (_RandomBeacon *RandomBeaconFilterer) FilterRewardsWithdrawn(opts *bind.FilterOpts, stakingProvider []common.Address) (*RandomBeaconRewardsWithdrawnIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "RewardsWithdrawn", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconRewardsWithdrawnIterator{contract: _RandomBeacon.contract, event: "RewardsWithdrawn", logs: logs, sub: sub}, nil
}

// WatchRewardsWithdrawn is a free log subscription operation binding the contract event 0x38532b6dea69d7266fa923c7813d190be37625f2454ddfa3d93c45c79482e3fd.
//
// Solidity: event RewardsWithdrawn(address indexed stakingProvider, uint96 amount)
func (_RandomBeacon *RandomBeaconFilterer) WatchRewardsWithdrawn(opts *bind.WatchOpts, sink chan<- *RandomBeaconRewardsWithdrawn, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "RewardsWithdrawn", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconRewardsWithdrawn)
				if err := _RandomBeacon.contract.UnpackLog(event, "RewardsWithdrawn", log); err != nil {
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
func (_RandomBeacon *RandomBeaconFilterer) ParseRewardsWithdrawn(log types.Log) (*RandomBeaconRewardsWithdrawn, error) {
	event := new(RandomBeaconRewardsWithdrawn)
	if err := _RandomBeacon.contract.UnpackLog(event, "RewardsWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconSlashingParametersUpdatedIterator is returned from FilterSlashingParametersUpdated and is used to iterate over the raw logs and unpacked data for SlashingParametersUpdated events raised by the RandomBeacon contract.
type RandomBeaconSlashingParametersUpdatedIterator struct {
	Event *RandomBeaconSlashingParametersUpdated // Event containing the contract specifics and raw log

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
func (it *RandomBeaconSlashingParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconSlashingParametersUpdated)
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
		it.Event = new(RandomBeaconSlashingParametersUpdated)
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
func (it *RandomBeaconSlashingParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconSlashingParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconSlashingParametersUpdated represents a SlashingParametersUpdated event raised by the RandomBeacon contract.
type RandomBeaconSlashingParametersUpdated struct {
	RelayEntrySubmissionFailureSlashingAmount *big.Int
	MaliciousDkgResultSlashingAmount          *big.Int
	UnauthorizedSigningSlashingAmount         *big.Int
	Raw                                       types.Log // Blockchain specific contextual infos
}

// FilterSlashingParametersUpdated is a free log retrieval operation binding the contract event 0x1eda09aaee2b21bbf5571b06eab42dd1b2c2b629a5d6336230ff8b0e1f538276.
//
// Solidity: event SlashingParametersUpdated(uint256 relayEntrySubmissionFailureSlashingAmount, uint256 maliciousDkgResultSlashingAmount, uint256 unauthorizedSigningSlashingAmount)
func (_RandomBeacon *RandomBeaconFilterer) FilterSlashingParametersUpdated(opts *bind.FilterOpts) (*RandomBeaconSlashingParametersUpdatedIterator, error) {

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "SlashingParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &RandomBeaconSlashingParametersUpdatedIterator{contract: _RandomBeacon.contract, event: "SlashingParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchSlashingParametersUpdated is a free log subscription operation binding the contract event 0x1eda09aaee2b21bbf5571b06eab42dd1b2c2b629a5d6336230ff8b0e1f538276.
//
// Solidity: event SlashingParametersUpdated(uint256 relayEntrySubmissionFailureSlashingAmount, uint256 maliciousDkgResultSlashingAmount, uint256 unauthorizedSigningSlashingAmount)
func (_RandomBeacon *RandomBeaconFilterer) WatchSlashingParametersUpdated(opts *bind.WatchOpts, sink chan<- *RandomBeaconSlashingParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "SlashingParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconSlashingParametersUpdated)
				if err := _RandomBeacon.contract.UnpackLog(event, "SlashingParametersUpdated", log); err != nil {
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

// ParseSlashingParametersUpdated is a log parse operation binding the contract event 0x1eda09aaee2b21bbf5571b06eab42dd1b2c2b629a5d6336230ff8b0e1f538276.
//
// Solidity: event SlashingParametersUpdated(uint256 relayEntrySubmissionFailureSlashingAmount, uint256 maliciousDkgResultSlashingAmount, uint256 unauthorizedSigningSlashingAmount)
func (_RandomBeacon *RandomBeaconFilterer) ParseSlashingParametersUpdated(log types.Log) (*RandomBeaconSlashingParametersUpdated, error) {
	event := new(RandomBeaconSlashingParametersUpdated)
	if err := _RandomBeacon.contract.UnpackLog(event, "SlashingParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconUnauthorizedSigningSlashedIterator is returned from FilterUnauthorizedSigningSlashed and is used to iterate over the raw logs and unpacked data for UnauthorizedSigningSlashed events raised by the RandomBeacon contract.
type RandomBeaconUnauthorizedSigningSlashedIterator struct {
	Event *RandomBeaconUnauthorizedSigningSlashed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconUnauthorizedSigningSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconUnauthorizedSigningSlashed)
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
		it.Event = new(RandomBeaconUnauthorizedSigningSlashed)
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
func (it *RandomBeaconUnauthorizedSigningSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconUnauthorizedSigningSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconUnauthorizedSigningSlashed represents a UnauthorizedSigningSlashed event raised by the RandomBeacon contract.
type RandomBeaconUnauthorizedSigningSlashed struct {
	GroupId                           uint64
	UnauthorizedSigningSlashingAmount *big.Int
	GroupMembers                      []common.Address
	Raw                               types.Log // Blockchain specific contextual infos
}

// FilterUnauthorizedSigningSlashed is a free log retrieval operation binding the contract event 0xa311dcb2a3eb32651a722488f4f281b3f8e4ab05abcd8a37a5c7d663bba5a885.
//
// Solidity: event UnauthorizedSigningSlashed(uint64 indexed groupId, uint256 unauthorizedSigningSlashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) FilterUnauthorizedSigningSlashed(opts *bind.FilterOpts, groupId []uint64) (*RandomBeaconUnauthorizedSigningSlashedIterator, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "UnauthorizedSigningSlashed", groupIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconUnauthorizedSigningSlashedIterator{contract: _RandomBeacon.contract, event: "UnauthorizedSigningSlashed", logs: logs, sub: sub}, nil
}

// WatchUnauthorizedSigningSlashed is a free log subscription operation binding the contract event 0xa311dcb2a3eb32651a722488f4f281b3f8e4ab05abcd8a37a5c7d663bba5a885.
//
// Solidity: event UnauthorizedSigningSlashed(uint64 indexed groupId, uint256 unauthorizedSigningSlashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) WatchUnauthorizedSigningSlashed(opts *bind.WatchOpts, sink chan<- *RandomBeaconUnauthorizedSigningSlashed, groupId []uint64) (event.Subscription, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "UnauthorizedSigningSlashed", groupIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconUnauthorizedSigningSlashed)
				if err := _RandomBeacon.contract.UnpackLog(event, "UnauthorizedSigningSlashed", log); err != nil {
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

// ParseUnauthorizedSigningSlashed is a log parse operation binding the contract event 0xa311dcb2a3eb32651a722488f4f281b3f8e4ab05abcd8a37a5c7d663bba5a885.
//
// Solidity: event UnauthorizedSigningSlashed(uint64 indexed groupId, uint256 unauthorizedSigningSlashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) ParseUnauthorizedSigningSlashed(log types.Log) (*RandomBeaconUnauthorizedSigningSlashed, error) {
	event := new(RandomBeaconUnauthorizedSigningSlashed)
	if err := _RandomBeacon.contract.UnpackLog(event, "UnauthorizedSigningSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RandomBeaconUnauthorizedSigningSlashingFailedIterator is returned from FilterUnauthorizedSigningSlashingFailed and is used to iterate over the raw logs and unpacked data for UnauthorizedSigningSlashingFailed events raised by the RandomBeacon contract.
type RandomBeaconUnauthorizedSigningSlashingFailedIterator struct {
	Event *RandomBeaconUnauthorizedSigningSlashingFailed // Event containing the contract specifics and raw log

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
func (it *RandomBeaconUnauthorizedSigningSlashingFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RandomBeaconUnauthorizedSigningSlashingFailed)
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
		it.Event = new(RandomBeaconUnauthorizedSigningSlashingFailed)
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
func (it *RandomBeaconUnauthorizedSigningSlashingFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RandomBeaconUnauthorizedSigningSlashingFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RandomBeaconUnauthorizedSigningSlashingFailed represents a UnauthorizedSigningSlashingFailed event raised by the RandomBeacon contract.
type RandomBeaconUnauthorizedSigningSlashingFailed struct {
	GroupId                           uint64
	UnauthorizedSigningSlashingAmount *big.Int
	GroupMembers                      []common.Address
	Raw                               types.Log // Blockchain specific contextual infos
}

// FilterUnauthorizedSigningSlashingFailed is a free log retrieval operation binding the contract event 0xfd4a5a45de3194b94e2a7954706ac9023b7c9935cf1a25242691da2a94d720d4.
//
// Solidity: event UnauthorizedSigningSlashingFailed(uint64 indexed groupId, uint256 unauthorizedSigningSlashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) FilterUnauthorizedSigningSlashingFailed(opts *bind.FilterOpts, groupId []uint64) (*RandomBeaconUnauthorizedSigningSlashingFailedIterator, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.FilterLogs(opts, "UnauthorizedSigningSlashingFailed", groupIdRule)
	if err != nil {
		return nil, err
	}
	return &RandomBeaconUnauthorizedSigningSlashingFailedIterator{contract: _RandomBeacon.contract, event: "UnauthorizedSigningSlashingFailed", logs: logs, sub: sub}, nil
}

// WatchUnauthorizedSigningSlashingFailed is a free log subscription operation binding the contract event 0xfd4a5a45de3194b94e2a7954706ac9023b7c9935cf1a25242691da2a94d720d4.
//
// Solidity: event UnauthorizedSigningSlashingFailed(uint64 indexed groupId, uint256 unauthorizedSigningSlashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) WatchUnauthorizedSigningSlashingFailed(opts *bind.WatchOpts, sink chan<- *RandomBeaconUnauthorizedSigningSlashingFailed, groupId []uint64) (event.Subscription, error) {

	var groupIdRule []interface{}
	for _, groupIdItem := range groupId {
		groupIdRule = append(groupIdRule, groupIdItem)
	}

	logs, sub, err := _RandomBeacon.contract.WatchLogs(opts, "UnauthorizedSigningSlashingFailed", groupIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RandomBeaconUnauthorizedSigningSlashingFailed)
				if err := _RandomBeacon.contract.UnpackLog(event, "UnauthorizedSigningSlashingFailed", log); err != nil {
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

// ParseUnauthorizedSigningSlashingFailed is a log parse operation binding the contract event 0xfd4a5a45de3194b94e2a7954706ac9023b7c9935cf1a25242691da2a94d720d4.
//
// Solidity: event UnauthorizedSigningSlashingFailed(uint64 indexed groupId, uint256 unauthorizedSigningSlashingAmount, address[] groupMembers)
func (_RandomBeacon *RandomBeaconFilterer) ParseUnauthorizedSigningSlashingFailed(log types.Log) (*RandomBeaconUnauthorizedSigningSlashingFailed, error) {
	event := new(RandomBeaconUnauthorizedSigningSlashingFailed)
	if err := _RandomBeacon.contract.UnpackLog(event, "UnauthorizedSigningSlashingFailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

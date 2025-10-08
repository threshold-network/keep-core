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
)

// CheckpointsCheckpoint is an auto generated low-level Go binding around an user-defined struct.
type CheckpointsCheckpoint struct {
	FromBlock uint32
	Votes     *big.Int
}

// TokenStakingMetaData contains all meta data concerning the TokenStaking contract.
var TokenStakingMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractT\",\"name\":\"_token\",\"type\":\"address\"},{\"internalType\":\"contractIKeepTokenStaking\",\"name\":\"_keepStakingContract\",\"type\":\"address\"},{\"internalType\":\"contractINuCypherStakingEscrow\",\"name\":\"_nucypherStakingContract\",\"type\":\"address\"},{\"internalType\":\"contractVendingMachine\",\"name\":\"_keepVendingMachine\",\"type\":\"address\"},{\"internalType\":\"contractVendingMachine\",\"name\":\"_nucypherVendingMachine\",\"type\":\"address\"},{\"internalType\":\"contractKeepStake\",\"name\":\"_keepStake\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"enumTokenStaking.ApplicationStatus\",\"name\":\"newStatus\",\"type\":\"uint8\"}],\"name\":\"ApplicationStatusChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"ceiling\",\"type\":\"uint256\"}],\"name\":\"AuthorizationCeilingSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"AuthorizationDecreaseApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"AuthorizationDecreaseRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"}],\"name\":\"AuthorizationIncreased\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fromAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"toAmount\",\"type\":\"uint96\"},{\"indexed\":true,\"internalType\":\"bool\",\"name\":\"successfulCall\",\"type\":\"bool\"}],\"name\":\"AuthorizationInvoluntaryDecreased\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegator\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"fromDelegate\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"toDelegate\",\"type\":\"address\"}],\"name\":\"DelegateChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegate\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"previousBalance\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newBalance\",\"type\":\"uint256\"}],\"name\":\"DelegateVotesChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldGovernance\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"GovernanceTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"MinimumStakeAmountSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"reward\",\"type\":\"uint96\"}],\"name\":\"NotificationRewardPushed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"reward\",\"type\":\"uint96\"}],\"name\":\"NotificationRewardSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"NotificationRewardWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"notifier\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"NotifierRewarded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnerRefreshed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"panicButton\",\"type\":\"address\"}],\"name\":\"PanicButtonSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"caller\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"tAmount\",\"type\":\"uint256\"}],\"name\":\"SlashingProcessed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"penalty\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"rewardMultiplier\",\"type\":\"uint256\"}],\"name\":\"StakeDiscrepancyPenaltySet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"enumIStaking.StakeType\",\"name\":\"stakeType\",\"type\":\"uint8\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"beneficiary\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"authorizer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"Staked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"},{\"indexed\":true,\"internalType\":\"bool\",\"name\":\"discrepancy\",\"type\":\"bool\"}],\"name\":\"TokensSeized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"ToppedUp\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"Unstaked\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"applicationInfo\",\"outputs\":[{\"internalType\":\"enumTokenStaking.ApplicationStatus\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"panicButton\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"applications\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"}],\"name\":\"approveApplication\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"approveAuthorizationDecrease\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"authorizationCeiling\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"}],\"name\":\"authorizedStake\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"uint32\",\"name\":\"pos\",\"type\":\"uint32\"}],\"name\":\"checkpoints\",\"outputs\":[{\"components\":[{\"internalType\":\"uint32\",\"name\":\"fromBlock\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"votes\",\"type\":\"uint96\"}],\"internalType\":\"structCheckpoints.Checkpoint\",\"name\":\"checkpoint\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"delegatee\",\"type\":\"address\"}],\"name\":\"delegateVoting\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"delegates\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"}],\"name\":\"disableApplication\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"}],\"name\":\"forceDecreaseAuthorization\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getApplicationsLength\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"}],\"name\":\"getAvailableToAuthorize\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"availableTValue\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"enumIStaking.StakeType\",\"name\":\"stakeTypes\",\"type\":\"uint8\"}],\"name\":\"getMinStaked\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"getPastTotalSupply\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"getPastVotes\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getSlashingQueueLength\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"getStartStakingTimestamp\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"getVotes\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"governance\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"increaseAuthorization\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"minTStakeAmount\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notificationReward\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifiersTreasury\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"notifyKeepStakeDiscrepancy\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"notifyNuStakeDiscrepancy\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"numCheckpoints\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"}],\"name\":\"pauseApplication\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"count\",\"type\":\"uint256\"}],\"name\":\"processSlashing\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"reward\",\"type\":\"uint96\"}],\"name\":\"pushNotificationReward\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"refreshKeepStakeOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"requestAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"requestAuthorizationDecrease\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"rolesOf\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"addresspayable\",\"name\":\"beneficiary\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"authorizer\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"},{\"internalType\":\"uint256\",\"name\":\"rewardMultiplier\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"notifier\",\"type\":\"address\"},{\"internalType\":\"address[]\",\"name\":\"_stakingProviders\",\"type\":\"address[]\"}],\"name\":\"seize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"ceiling\",\"type\":\"uint256\"}],\"name\":\"setAuthorizationCeiling\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"setMinimumStakeAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"reward\",\"type\":\"uint96\"}],\"name\":\"setNotificationReward\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"application\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"panicButton\",\"type\":\"address\"}],\"name\":\"setPanicButton\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"penalty\",\"type\":\"uint96\"},{\"internalType\":\"uint256\",\"name\":\"rewardMultiplier\",\"type\":\"uint256\"}],\"name\":\"setStakeDiscrepancyPenalty\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"},{\"internalType\":\"address[]\",\"name\":\"_stakingProviders\",\"type\":\"address[]\"}],\"name\":\"slash\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"slashingQueue\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"slashingQueueIndex\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"addresspayable\",\"name\":\"beneficiary\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"authorizer\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"stake\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"stakeDiscrepancyPenalty\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"stakeDiscrepancyRewardMultiplier\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"stakeKeep\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"addresspayable\",\"name\":\"beneficiary\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"authorizer\",\"type\":\"address\"}],\"name\":\"stakeNu\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"stakedNu\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"nuAmount\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"stakes\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"tStake\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"keepInTStake\",\"type\":\"uint96\"},{\"internalType\":\"uint96\",\"name\":\"nuInTStake\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"topUp\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"topUpKeep\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"topUpNu\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newGuvnor\",\"type\":\"address\"}],\"name\":\"transferGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"unstakeAll\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"}],\"name\":\"unstakeKeep\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"unstakeNu\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"stakingProvider\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"unstakeT\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"amount\",\"type\":\"uint96\"}],\"name\":\"withdrawNotificationReward\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// TokenStakingABI is the input ABI used to generate the binding from.
// Deprecated: Use TokenStakingMetaData.ABI instead.
var TokenStakingABI = TokenStakingMetaData.ABI

// TokenStaking is an auto generated Go binding around an Ethereum contract.
type TokenStaking struct {
	TokenStakingCaller     // Read-only binding to the contract
	TokenStakingTransactor // Write-only binding to the contract
	TokenStakingFilterer   // Log filterer for contract events
}

// TokenStakingCaller is an auto generated read-only Go binding around an Ethereum contract.
type TokenStakingCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenStakingTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TokenStakingTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenStakingFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TokenStakingFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenStakingSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TokenStakingSession struct {
	Contract     *TokenStaking     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TokenStakingCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TokenStakingCallerSession struct {
	Contract *TokenStakingCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// TokenStakingTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TokenStakingTransactorSession struct {
	Contract     *TokenStakingTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// TokenStakingRaw is an auto generated low-level Go binding around an Ethereum contract.
type TokenStakingRaw struct {
	Contract *TokenStaking // Generic contract binding to access the raw methods on
}

// TokenStakingCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TokenStakingCallerRaw struct {
	Contract *TokenStakingCaller // Generic read-only contract binding to access the raw methods on
}

// TokenStakingTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TokenStakingTransactorRaw struct {
	Contract *TokenStakingTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTokenStaking creates a new instance of TokenStaking, bound to a specific deployed contract.
func NewTokenStaking(address common.Address, backend bind.ContractBackend) (*TokenStaking, error) {
	contract, err := bindTokenStaking(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TokenStaking{TokenStakingCaller: TokenStakingCaller{contract: contract}, TokenStakingTransactor: TokenStakingTransactor{contract: contract}, TokenStakingFilterer: TokenStakingFilterer{contract: contract}}, nil
}

// NewTokenStakingCaller creates a new read-only instance of TokenStaking, bound to a specific deployed contract.
func NewTokenStakingCaller(address common.Address, caller bind.ContractCaller) (*TokenStakingCaller, error) {
	contract, err := bindTokenStaking(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TokenStakingCaller{contract: contract}, nil
}

// NewTokenStakingTransactor creates a new write-only instance of TokenStaking, bound to a specific deployed contract.
func NewTokenStakingTransactor(address common.Address, transactor bind.ContractTransactor) (*TokenStakingTransactor, error) {
	contract, err := bindTokenStaking(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TokenStakingTransactor{contract: contract}, nil
}

// NewTokenStakingFilterer creates a new log filterer instance of TokenStaking, bound to a specific deployed contract.
func NewTokenStakingFilterer(address common.Address, filterer bind.ContractFilterer) (*TokenStakingFilterer, error) {
	contract, err := bindTokenStaking(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TokenStakingFilterer{contract: contract}, nil
}

// bindTokenStaking binds a generic wrapper to an already deployed contract.
func bindTokenStaking(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(TokenStakingABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TokenStaking *TokenStakingRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TokenStaking.Contract.TokenStakingCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TokenStaking *TokenStakingRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TokenStaking.Contract.TokenStakingTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TokenStaking *TokenStakingRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TokenStaking.Contract.TokenStakingTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TokenStaking *TokenStakingCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TokenStaking.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TokenStaking *TokenStakingTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TokenStaking.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TokenStaking *TokenStakingTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TokenStaking.Contract.contract.Transact(opts, method, params...)
}

// ApplicationInfo is a free data retrieval call binding the contract method 0x067e6bb1.
//
// Solidity: function applicationInfo(address ) view returns(uint8 status, address panicButton)
func (_TokenStaking *TokenStakingCaller) ApplicationInfo(opts *bind.CallOpts, arg0 common.Address) (struct {
	Status      uint8
	PanicButton common.Address
}, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "applicationInfo", arg0)

	outstruct := new(struct {
		Status      uint8
		PanicButton common.Address
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Status = *abi.ConvertType(out[0], new(uint8)).(*uint8)
	outstruct.PanicButton = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)

	return *outstruct, err

}

// ApplicationInfo is a free data retrieval call binding the contract method 0x067e6bb1.
//
// Solidity: function applicationInfo(address ) view returns(uint8 status, address panicButton)
func (_TokenStaking *TokenStakingSession) ApplicationInfo(arg0 common.Address) (struct {
	Status      uint8
	PanicButton common.Address
}, error) {
	return _TokenStaking.Contract.ApplicationInfo(&_TokenStaking.CallOpts, arg0)
}

// ApplicationInfo is a free data retrieval call binding the contract method 0x067e6bb1.
//
// Solidity: function applicationInfo(address ) view returns(uint8 status, address panicButton)
func (_TokenStaking *TokenStakingCallerSession) ApplicationInfo(arg0 common.Address) (struct {
	Status      uint8
	PanicButton common.Address
}, error) {
	return _TokenStaking.Contract.ApplicationInfo(&_TokenStaking.CallOpts, arg0)
}

// Applications is a free data retrieval call binding the contract method 0xdfefadff.
//
// Solidity: function applications(uint256 ) view returns(address)
func (_TokenStaking *TokenStakingCaller) Applications(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "applications", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Applications is a free data retrieval call binding the contract method 0xdfefadff.
//
// Solidity: function applications(uint256 ) view returns(address)
func (_TokenStaking *TokenStakingSession) Applications(arg0 *big.Int) (common.Address, error) {
	return _TokenStaking.Contract.Applications(&_TokenStaking.CallOpts, arg0)
}

// Applications is a free data retrieval call binding the contract method 0xdfefadff.
//
// Solidity: function applications(uint256 ) view returns(address)
func (_TokenStaking *TokenStakingCallerSession) Applications(arg0 *big.Int) (common.Address, error) {
	return _TokenStaking.Contract.Applications(&_TokenStaking.CallOpts, arg0)
}

// AuthorizationCeiling is a free data retrieval call binding the contract method 0x2cd62915.
//
// Solidity: function authorizationCeiling() view returns(uint256)
func (_TokenStaking *TokenStakingCaller) AuthorizationCeiling(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "authorizationCeiling")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AuthorizationCeiling is a free data retrieval call binding the contract method 0x2cd62915.
//
// Solidity: function authorizationCeiling() view returns(uint256)
func (_TokenStaking *TokenStakingSession) AuthorizationCeiling() (*big.Int, error) {
	return _TokenStaking.Contract.AuthorizationCeiling(&_TokenStaking.CallOpts)
}

// AuthorizationCeiling is a free data retrieval call binding the contract method 0x2cd62915.
//
// Solidity: function authorizationCeiling() view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) AuthorizationCeiling() (*big.Int, error) {
	return _TokenStaking.Contract.AuthorizationCeiling(&_TokenStaking.CallOpts)
}

// AuthorizedStake is a free data retrieval call binding the contract method 0xe009245a.
//
// Solidity: function authorizedStake(address stakingProvider, address application) view returns(uint96)
func (_TokenStaking *TokenStakingCaller) AuthorizedStake(opts *bind.CallOpts, stakingProvider common.Address, application common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "authorizedStake", stakingProvider, application)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AuthorizedStake is a free data retrieval call binding the contract method 0xe009245a.
//
// Solidity: function authorizedStake(address stakingProvider, address application) view returns(uint96)
func (_TokenStaking *TokenStakingSession) AuthorizedStake(stakingProvider common.Address, application common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.AuthorizedStake(&_TokenStaking.CallOpts, stakingProvider, application)
}

// AuthorizedStake is a free data retrieval call binding the contract method 0xe009245a.
//
// Solidity: function authorizedStake(address stakingProvider, address application) view returns(uint96)
func (_TokenStaking *TokenStakingCallerSession) AuthorizedStake(stakingProvider common.Address, application common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.AuthorizedStake(&_TokenStaking.CallOpts, stakingProvider, application)
}

// Checkpoints is a free data retrieval call binding the contract method 0xf1127ed8.
//
// Solidity: function checkpoints(address account, uint32 pos) view returns((uint32,uint96) checkpoint)
func (_TokenStaking *TokenStakingCaller) Checkpoints(opts *bind.CallOpts, account common.Address, pos uint32) (CheckpointsCheckpoint, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "checkpoints", account, pos)

	if err != nil {
		return *new(CheckpointsCheckpoint), err
	}

	out0 := *abi.ConvertType(out[0], new(CheckpointsCheckpoint)).(*CheckpointsCheckpoint)

	return out0, err

}

// Checkpoints is a free data retrieval call binding the contract method 0xf1127ed8.
//
// Solidity: function checkpoints(address account, uint32 pos) view returns((uint32,uint96) checkpoint)
func (_TokenStaking *TokenStakingSession) Checkpoints(account common.Address, pos uint32) (CheckpointsCheckpoint, error) {
	return _TokenStaking.Contract.Checkpoints(&_TokenStaking.CallOpts, account, pos)
}

// Checkpoints is a free data retrieval call binding the contract method 0xf1127ed8.
//
// Solidity: function checkpoints(address account, uint32 pos) view returns((uint32,uint96) checkpoint)
func (_TokenStaking *TokenStakingCallerSession) Checkpoints(account common.Address, pos uint32) (CheckpointsCheckpoint, error) {
	return _TokenStaking.Contract.Checkpoints(&_TokenStaking.CallOpts, account, pos)
}

// Delegates is a free data retrieval call binding the contract method 0x587cde1e.
//
// Solidity: function delegates(address account) view returns(address)
func (_TokenStaking *TokenStakingCaller) Delegates(opts *bind.CallOpts, account common.Address) (common.Address, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "delegates", account)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Delegates is a free data retrieval call binding the contract method 0x587cde1e.
//
// Solidity: function delegates(address account) view returns(address)
func (_TokenStaking *TokenStakingSession) Delegates(account common.Address) (common.Address, error) {
	return _TokenStaking.Contract.Delegates(&_TokenStaking.CallOpts, account)
}

// Delegates is a free data retrieval call binding the contract method 0x587cde1e.
//
// Solidity: function delegates(address account) view returns(address)
func (_TokenStaking *TokenStakingCallerSession) Delegates(account common.Address) (common.Address, error) {
	return _TokenStaking.Contract.Delegates(&_TokenStaking.CallOpts, account)
}

// GetApplicationsLength is a free data retrieval call binding the contract method 0xad30e4cd.
//
// Solidity: function getApplicationsLength() view returns(uint256)
func (_TokenStaking *TokenStakingCaller) GetApplicationsLength(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getApplicationsLength")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetApplicationsLength is a free data retrieval call binding the contract method 0xad30e4cd.
//
// Solidity: function getApplicationsLength() view returns(uint256)
func (_TokenStaking *TokenStakingSession) GetApplicationsLength() (*big.Int, error) {
	return _TokenStaking.Contract.GetApplicationsLength(&_TokenStaking.CallOpts)
}

// GetApplicationsLength is a free data retrieval call binding the contract method 0xad30e4cd.
//
// Solidity: function getApplicationsLength() view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) GetApplicationsLength() (*big.Int, error) {
	return _TokenStaking.Contract.GetApplicationsLength(&_TokenStaking.CallOpts)
}

// GetAvailableToAuthorize is a free data retrieval call binding the contract method 0x8b7adc09.
//
// Solidity: function getAvailableToAuthorize(address stakingProvider, address application) view returns(uint96 availableTValue)
func (_TokenStaking *TokenStakingCaller) GetAvailableToAuthorize(opts *bind.CallOpts, stakingProvider common.Address, application common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getAvailableToAuthorize", stakingProvider, application)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetAvailableToAuthorize is a free data retrieval call binding the contract method 0x8b7adc09.
//
// Solidity: function getAvailableToAuthorize(address stakingProvider, address application) view returns(uint96 availableTValue)
func (_TokenStaking *TokenStakingSession) GetAvailableToAuthorize(stakingProvider common.Address, application common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.GetAvailableToAuthorize(&_TokenStaking.CallOpts, stakingProvider, application)
}

// GetAvailableToAuthorize is a free data retrieval call binding the contract method 0x8b7adc09.
//
// Solidity: function getAvailableToAuthorize(address stakingProvider, address application) view returns(uint96 availableTValue)
func (_TokenStaking *TokenStakingCallerSession) GetAvailableToAuthorize(stakingProvider common.Address, application common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.GetAvailableToAuthorize(&_TokenStaking.CallOpts, stakingProvider, application)
}

// GetMinStaked is a free data retrieval call binding the contract method 0x6da91d8b.
//
// Solidity: function getMinStaked(address stakingProvider, uint8 stakeTypes) view returns(uint96)
func (_TokenStaking *TokenStakingCaller) GetMinStaked(opts *bind.CallOpts, stakingProvider common.Address, stakeTypes uint8) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getMinStaked", stakingProvider, stakeTypes)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetMinStaked is a free data retrieval call binding the contract method 0x6da91d8b.
//
// Solidity: function getMinStaked(address stakingProvider, uint8 stakeTypes) view returns(uint96)
func (_TokenStaking *TokenStakingSession) GetMinStaked(stakingProvider common.Address, stakeTypes uint8) (*big.Int, error) {
	return _TokenStaking.Contract.GetMinStaked(&_TokenStaking.CallOpts, stakingProvider, stakeTypes)
}

// GetMinStaked is a free data retrieval call binding the contract method 0x6da91d8b.
//
// Solidity: function getMinStaked(address stakingProvider, uint8 stakeTypes) view returns(uint96)
func (_TokenStaking *TokenStakingCallerSession) GetMinStaked(stakingProvider common.Address, stakeTypes uint8) (*big.Int, error) {
	return _TokenStaking.Contract.GetMinStaked(&_TokenStaking.CallOpts, stakingProvider, stakeTypes)
}

// GetPastTotalSupply is a free data retrieval call binding the contract method 0x8e539e8c.
//
// Solidity: function getPastTotalSupply(uint256 blockNumber) view returns(uint96)
func (_TokenStaking *TokenStakingCaller) GetPastTotalSupply(opts *bind.CallOpts, blockNumber *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getPastTotalSupply", blockNumber)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetPastTotalSupply is a free data retrieval call binding the contract method 0x8e539e8c.
//
// Solidity: function getPastTotalSupply(uint256 blockNumber) view returns(uint96)
func (_TokenStaking *TokenStakingSession) GetPastTotalSupply(blockNumber *big.Int) (*big.Int, error) {
	return _TokenStaking.Contract.GetPastTotalSupply(&_TokenStaking.CallOpts, blockNumber)
}

// GetPastTotalSupply is a free data retrieval call binding the contract method 0x8e539e8c.
//
// Solidity: function getPastTotalSupply(uint256 blockNumber) view returns(uint96)
func (_TokenStaking *TokenStakingCallerSession) GetPastTotalSupply(blockNumber *big.Int) (*big.Int, error) {
	return _TokenStaking.Contract.GetPastTotalSupply(&_TokenStaking.CallOpts, blockNumber)
}

// GetPastVotes is a free data retrieval call binding the contract method 0x3a46b1a8.
//
// Solidity: function getPastVotes(address account, uint256 blockNumber) view returns(uint96)
func (_TokenStaking *TokenStakingCaller) GetPastVotes(opts *bind.CallOpts, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getPastVotes", account, blockNumber)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetPastVotes is a free data retrieval call binding the contract method 0x3a46b1a8.
//
// Solidity: function getPastVotes(address account, uint256 blockNumber) view returns(uint96)
func (_TokenStaking *TokenStakingSession) GetPastVotes(account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return _TokenStaking.Contract.GetPastVotes(&_TokenStaking.CallOpts, account, blockNumber)
}

// GetPastVotes is a free data retrieval call binding the contract method 0x3a46b1a8.
//
// Solidity: function getPastVotes(address account, uint256 blockNumber) view returns(uint96)
func (_TokenStaking *TokenStakingCallerSession) GetPastVotes(account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return _TokenStaking.Contract.GetPastVotes(&_TokenStaking.CallOpts, account, blockNumber)
}

// GetSlashingQueueLength is a free data retrieval call binding the contract method 0xffd2f984.
//
// Solidity: function getSlashingQueueLength() view returns(uint256)
func (_TokenStaking *TokenStakingCaller) GetSlashingQueueLength(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getSlashingQueueLength")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetSlashingQueueLength is a free data retrieval call binding the contract method 0xffd2f984.
//
// Solidity: function getSlashingQueueLength() view returns(uint256)
func (_TokenStaking *TokenStakingSession) GetSlashingQueueLength() (*big.Int, error) {
	return _TokenStaking.Contract.GetSlashingQueueLength(&_TokenStaking.CallOpts)
}

// GetSlashingQueueLength is a free data retrieval call binding the contract method 0xffd2f984.
//
// Solidity: function getSlashingQueueLength() view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) GetSlashingQueueLength() (*big.Int, error) {
	return _TokenStaking.Contract.GetSlashingQueueLength(&_TokenStaking.CallOpts)
}

// GetStartStakingTimestamp is a free data retrieval call binding the contract method 0xf6e0faeb.
//
// Solidity: function getStartStakingTimestamp(address stakingProvider) view returns(uint256)
func (_TokenStaking *TokenStakingCaller) GetStartStakingTimestamp(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getStartStakingTimestamp", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetStartStakingTimestamp is a free data retrieval call binding the contract method 0xf6e0faeb.
//
// Solidity: function getStartStakingTimestamp(address stakingProvider) view returns(uint256)
func (_TokenStaking *TokenStakingSession) GetStartStakingTimestamp(stakingProvider common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.GetStartStakingTimestamp(&_TokenStaking.CallOpts, stakingProvider)
}

// GetStartStakingTimestamp is a free data retrieval call binding the contract method 0xf6e0faeb.
//
// Solidity: function getStartStakingTimestamp(address stakingProvider) view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) GetStartStakingTimestamp(stakingProvider common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.GetStartStakingTimestamp(&_TokenStaking.CallOpts, stakingProvider)
}

// GetVotes is a free data retrieval call binding the contract method 0x9ab24eb0.
//
// Solidity: function getVotes(address account) view returns(uint96)
func (_TokenStaking *TokenStakingCaller) GetVotes(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "getVotes", account)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetVotes is a free data retrieval call binding the contract method 0x9ab24eb0.
//
// Solidity: function getVotes(address account) view returns(uint96)
func (_TokenStaking *TokenStakingSession) GetVotes(account common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.GetVotes(&_TokenStaking.CallOpts, account)
}

// GetVotes is a free data retrieval call binding the contract method 0x9ab24eb0.
//
// Solidity: function getVotes(address account) view returns(uint96)
func (_TokenStaking *TokenStakingCallerSession) GetVotes(account common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.GetVotes(&_TokenStaking.CallOpts, account)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_TokenStaking *TokenStakingCaller) Governance(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "governance")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_TokenStaking *TokenStakingSession) Governance() (common.Address, error) {
	return _TokenStaking.Contract.Governance(&_TokenStaking.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_TokenStaking *TokenStakingCallerSession) Governance() (common.Address, error) {
	return _TokenStaking.Contract.Governance(&_TokenStaking.CallOpts)
}

// MinTStakeAmount is a free data retrieval call binding the contract method 0x32719e06.
//
// Solidity: function minTStakeAmount() view returns(uint96)
func (_TokenStaking *TokenStakingCaller) MinTStakeAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "minTStakeAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MinTStakeAmount is a free data retrieval call binding the contract method 0x32719e06.
//
// Solidity: function minTStakeAmount() view returns(uint96)
func (_TokenStaking *TokenStakingSession) MinTStakeAmount() (*big.Int, error) {
	return _TokenStaking.Contract.MinTStakeAmount(&_TokenStaking.CallOpts)
}

// MinTStakeAmount is a free data retrieval call binding the contract method 0x32719e06.
//
// Solidity: function minTStakeAmount() view returns(uint96)
func (_TokenStaking *TokenStakingCallerSession) MinTStakeAmount() (*big.Int, error) {
	return _TokenStaking.Contract.MinTStakeAmount(&_TokenStaking.CallOpts)
}

// NotificationReward is a free data retrieval call binding the contract method 0x7368dba2.
//
// Solidity: function notificationReward() view returns(uint256)
func (_TokenStaking *TokenStakingCaller) NotificationReward(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "notificationReward")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NotificationReward is a free data retrieval call binding the contract method 0x7368dba2.
//
// Solidity: function notificationReward() view returns(uint256)
func (_TokenStaking *TokenStakingSession) NotificationReward() (*big.Int, error) {
	return _TokenStaking.Contract.NotificationReward(&_TokenStaking.CallOpts)
}

// NotificationReward is a free data retrieval call binding the contract method 0x7368dba2.
//
// Solidity: function notificationReward() view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) NotificationReward() (*big.Int, error) {
	return _TokenStaking.Contract.NotificationReward(&_TokenStaking.CallOpts)
}

// NotifiersTreasury is a free data retrieval call binding the contract method 0x793c1365.
//
// Solidity: function notifiersTreasury() view returns(uint256)
func (_TokenStaking *TokenStakingCaller) NotifiersTreasury(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "notifiersTreasury")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NotifiersTreasury is a free data retrieval call binding the contract method 0x793c1365.
//
// Solidity: function notifiersTreasury() view returns(uint256)
func (_TokenStaking *TokenStakingSession) NotifiersTreasury() (*big.Int, error) {
	return _TokenStaking.Contract.NotifiersTreasury(&_TokenStaking.CallOpts)
}

// NotifiersTreasury is a free data retrieval call binding the contract method 0x793c1365.
//
// Solidity: function notifiersTreasury() view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) NotifiersTreasury() (*big.Int, error) {
	return _TokenStaking.Contract.NotifiersTreasury(&_TokenStaking.CallOpts)
}

// NumCheckpoints is a free data retrieval call binding the contract method 0x6fcfff45.
//
// Solidity: function numCheckpoints(address account) view returns(uint32)
func (_TokenStaking *TokenStakingCaller) NumCheckpoints(opts *bind.CallOpts, account common.Address) (uint32, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "numCheckpoints", account)

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// NumCheckpoints is a free data retrieval call binding the contract method 0x6fcfff45.
//
// Solidity: function numCheckpoints(address account) view returns(uint32)
func (_TokenStaking *TokenStakingSession) NumCheckpoints(account common.Address) (uint32, error) {
	return _TokenStaking.Contract.NumCheckpoints(&_TokenStaking.CallOpts, account)
}

// NumCheckpoints is a free data retrieval call binding the contract method 0x6fcfff45.
//
// Solidity: function numCheckpoints(address account) view returns(uint32)
func (_TokenStaking *TokenStakingCallerSession) NumCheckpoints(account common.Address) (uint32, error) {
	return _TokenStaking.Contract.NumCheckpoints(&_TokenStaking.CallOpts, account)
}

// RolesOf is a free data retrieval call binding the contract method 0x2de94807.
//
// Solidity: function rolesOf(address stakingProvider) view returns(address owner, address beneficiary, address authorizer)
func (_TokenStaking *TokenStakingCaller) RolesOf(opts *bind.CallOpts, stakingProvider common.Address) (struct {
	Owner       common.Address
	Beneficiary common.Address
	Authorizer  common.Address
}, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "rolesOf", stakingProvider)

	outstruct := new(struct {
		Owner       common.Address
		Beneficiary common.Address
		Authorizer  common.Address
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Owner = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Beneficiary = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.Authorizer = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)

	return *outstruct, err

}

// RolesOf is a free data retrieval call binding the contract method 0x2de94807.
//
// Solidity: function rolesOf(address stakingProvider) view returns(address owner, address beneficiary, address authorizer)
func (_TokenStaking *TokenStakingSession) RolesOf(stakingProvider common.Address) (struct {
	Owner       common.Address
	Beneficiary common.Address
	Authorizer  common.Address
}, error) {
	return _TokenStaking.Contract.RolesOf(&_TokenStaking.CallOpts, stakingProvider)
}

// RolesOf is a free data retrieval call binding the contract method 0x2de94807.
//
// Solidity: function rolesOf(address stakingProvider) view returns(address owner, address beneficiary, address authorizer)
func (_TokenStaking *TokenStakingCallerSession) RolesOf(stakingProvider common.Address) (struct {
	Owner       common.Address
	Beneficiary common.Address
	Authorizer  common.Address
}, error) {
	return _TokenStaking.Contract.RolesOf(&_TokenStaking.CallOpts, stakingProvider)
}

// SlashingQueue is a free data retrieval call binding the contract method 0xf1f6c315.
//
// Solidity: function slashingQueue(uint256 ) view returns(address stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingCaller) SlashingQueue(opts *bind.CallOpts, arg0 *big.Int) (struct {
	StakingProvider common.Address
	Amount          *big.Int
}, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "slashingQueue", arg0)

	outstruct := new(struct {
		StakingProvider common.Address
		Amount          *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.StakingProvider = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Amount = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// SlashingQueue is a free data retrieval call binding the contract method 0xf1f6c315.
//
// Solidity: function slashingQueue(uint256 ) view returns(address stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingSession) SlashingQueue(arg0 *big.Int) (struct {
	StakingProvider common.Address
	Amount          *big.Int
}, error) {
	return _TokenStaking.Contract.SlashingQueue(&_TokenStaking.CallOpts, arg0)
}

// SlashingQueue is a free data retrieval call binding the contract method 0xf1f6c315.
//
// Solidity: function slashingQueue(uint256 ) view returns(address stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingCallerSession) SlashingQueue(arg0 *big.Int) (struct {
	StakingProvider common.Address
	Amount          *big.Int
}, error) {
	return _TokenStaking.Contract.SlashingQueue(&_TokenStaking.CallOpts, arg0)
}

// SlashingQueueIndex is a free data retrieval call binding the contract method 0xa7bb8ba8.
//
// Solidity: function slashingQueueIndex() view returns(uint256)
func (_TokenStaking *TokenStakingCaller) SlashingQueueIndex(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "slashingQueueIndex")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SlashingQueueIndex is a free data retrieval call binding the contract method 0xa7bb8ba8.
//
// Solidity: function slashingQueueIndex() view returns(uint256)
func (_TokenStaking *TokenStakingSession) SlashingQueueIndex() (*big.Int, error) {
	return _TokenStaking.Contract.SlashingQueueIndex(&_TokenStaking.CallOpts)
}

// SlashingQueueIndex is a free data retrieval call binding the contract method 0xa7bb8ba8.
//
// Solidity: function slashingQueueIndex() view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) SlashingQueueIndex() (*big.Int, error) {
	return _TokenStaking.Contract.SlashingQueueIndex(&_TokenStaking.CallOpts)
}

// StakeDiscrepancyPenalty is a free data retrieval call binding the contract method 0xeaa86492.
//
// Solidity: function stakeDiscrepancyPenalty() view returns(uint96)
func (_TokenStaking *TokenStakingCaller) StakeDiscrepancyPenalty(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "stakeDiscrepancyPenalty")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// StakeDiscrepancyPenalty is a free data retrieval call binding the contract method 0xeaa86492.
//
// Solidity: function stakeDiscrepancyPenalty() view returns(uint96)
func (_TokenStaking *TokenStakingSession) StakeDiscrepancyPenalty() (*big.Int, error) {
	return _TokenStaking.Contract.StakeDiscrepancyPenalty(&_TokenStaking.CallOpts)
}

// StakeDiscrepancyPenalty is a free data retrieval call binding the contract method 0xeaa86492.
//
// Solidity: function stakeDiscrepancyPenalty() view returns(uint96)
func (_TokenStaking *TokenStakingCallerSession) StakeDiscrepancyPenalty() (*big.Int, error) {
	return _TokenStaking.Contract.StakeDiscrepancyPenalty(&_TokenStaking.CallOpts)
}

// StakeDiscrepancyRewardMultiplier is a free data retrieval call binding the contract method 0x44e97423.
//
// Solidity: function stakeDiscrepancyRewardMultiplier() view returns(uint256)
func (_TokenStaking *TokenStakingCaller) StakeDiscrepancyRewardMultiplier(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "stakeDiscrepancyRewardMultiplier")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// StakeDiscrepancyRewardMultiplier is a free data retrieval call binding the contract method 0x44e97423.
//
// Solidity: function stakeDiscrepancyRewardMultiplier() view returns(uint256)
func (_TokenStaking *TokenStakingSession) StakeDiscrepancyRewardMultiplier() (*big.Int, error) {
	return _TokenStaking.Contract.StakeDiscrepancyRewardMultiplier(&_TokenStaking.CallOpts)
}

// StakeDiscrepancyRewardMultiplier is a free data retrieval call binding the contract method 0x44e97423.
//
// Solidity: function stakeDiscrepancyRewardMultiplier() view returns(uint256)
func (_TokenStaking *TokenStakingCallerSession) StakeDiscrepancyRewardMultiplier() (*big.Int, error) {
	return _TokenStaking.Contract.StakeDiscrepancyRewardMultiplier(&_TokenStaking.CallOpts)
}

// StakedNu is a free data retrieval call binding the contract method 0x4a11fae3.
//
// Solidity: function stakedNu(address stakingProvider) view returns(uint256 nuAmount)
func (_TokenStaking *TokenStakingCaller) StakedNu(opts *bind.CallOpts, stakingProvider common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "stakedNu", stakingProvider)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// StakedNu is a free data retrieval call binding the contract method 0x4a11fae3.
//
// Solidity: function stakedNu(address stakingProvider) view returns(uint256 nuAmount)
func (_TokenStaking *TokenStakingSession) StakedNu(stakingProvider common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.StakedNu(&_TokenStaking.CallOpts, stakingProvider)
}

// StakedNu is a free data retrieval call binding the contract method 0x4a11fae3.
//
// Solidity: function stakedNu(address stakingProvider) view returns(uint256 nuAmount)
func (_TokenStaking *TokenStakingCallerSession) StakedNu(stakingProvider common.Address) (*big.Int, error) {
	return _TokenStaking.Contract.StakedNu(&_TokenStaking.CallOpts, stakingProvider)
}

// Stakes is a free data retrieval call binding the contract method 0x16934fc4.
//
// Solidity: function stakes(address stakingProvider) view returns(uint96 tStake, uint96 keepInTStake, uint96 nuInTStake)
func (_TokenStaking *TokenStakingCaller) Stakes(opts *bind.CallOpts, stakingProvider common.Address) (struct {
	TStake       *big.Int
	KeepInTStake *big.Int
	NuInTStake   *big.Int
}, error) {
	var out []interface{}
	err := _TokenStaking.contract.Call(opts, &out, "stakes", stakingProvider)

	outstruct := new(struct {
		TStake       *big.Int
		KeepInTStake *big.Int
		NuInTStake   *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.TStake = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.KeepInTStake = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.NuInTStake = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Stakes is a free data retrieval call binding the contract method 0x16934fc4.
//
// Solidity: function stakes(address stakingProvider) view returns(uint96 tStake, uint96 keepInTStake, uint96 nuInTStake)
func (_TokenStaking *TokenStakingSession) Stakes(stakingProvider common.Address) (struct {
	TStake       *big.Int
	KeepInTStake *big.Int
	NuInTStake   *big.Int
}, error) {
	return _TokenStaking.Contract.Stakes(&_TokenStaking.CallOpts, stakingProvider)
}

// Stakes is a free data retrieval call binding the contract method 0x16934fc4.
//
// Solidity: function stakes(address stakingProvider) view returns(uint96 tStake, uint96 keepInTStake, uint96 nuInTStake)
func (_TokenStaking *TokenStakingCallerSession) Stakes(stakingProvider common.Address) (struct {
	TStake       *big.Int
	KeepInTStake *big.Int
	NuInTStake   *big.Int
}, error) {
	return _TokenStaking.Contract.Stakes(&_TokenStaking.CallOpts, stakingProvider)
}

// ApproveApplication is a paid mutator transaction binding the contract method 0xe3ae4d0a.
//
// Solidity: function approveApplication(address application) returns()
func (_TokenStaking *TokenStakingTransactor) ApproveApplication(opts *bind.TransactOpts, application common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "approveApplication", application)
}

// ApproveApplication is a paid mutator transaction binding the contract method 0xe3ae4d0a.
//
// Solidity: function approveApplication(address application) returns()
func (_TokenStaking *TokenStakingSession) ApproveApplication(application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.ApproveApplication(&_TokenStaking.TransactOpts, application)
}

// ApproveApplication is a paid mutator transaction binding the contract method 0xe3ae4d0a.
//
// Solidity: function approveApplication(address application) returns()
func (_TokenStaking *TokenStakingTransactorSession) ApproveApplication(application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.ApproveApplication(&_TokenStaking.TransactOpts, application)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns(uint96)
func (_TokenStaking *TokenStakingTransactor) ApproveAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "approveAuthorizationDecrease", stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns(uint96)
func (_TokenStaking *TokenStakingSession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.ApproveAuthorizationDecrease(&_TokenStaking.TransactOpts, stakingProvider)
}

// ApproveAuthorizationDecrease is a paid mutator transaction binding the contract method 0x75e0ae5a.
//
// Solidity: function approveAuthorizationDecrease(address stakingProvider) returns(uint96)
func (_TokenStaking *TokenStakingTransactorSession) ApproveAuthorizationDecrease(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.ApproveAuthorizationDecrease(&_TokenStaking.TransactOpts, stakingProvider)
}

// DelegateVoting is a paid mutator transaction binding the contract method 0x0fa78bf4.
//
// Solidity: function delegateVoting(address stakingProvider, address delegatee) returns()
func (_TokenStaking *TokenStakingTransactor) DelegateVoting(opts *bind.TransactOpts, stakingProvider common.Address, delegatee common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "delegateVoting", stakingProvider, delegatee)
}

// DelegateVoting is a paid mutator transaction binding the contract method 0x0fa78bf4.
//
// Solidity: function delegateVoting(address stakingProvider, address delegatee) returns()
func (_TokenStaking *TokenStakingSession) DelegateVoting(stakingProvider common.Address, delegatee common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.DelegateVoting(&_TokenStaking.TransactOpts, stakingProvider, delegatee)
}

// DelegateVoting is a paid mutator transaction binding the contract method 0x0fa78bf4.
//
// Solidity: function delegateVoting(address stakingProvider, address delegatee) returns()
func (_TokenStaking *TokenStakingTransactorSession) DelegateVoting(stakingProvider common.Address, delegatee common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.DelegateVoting(&_TokenStaking.TransactOpts, stakingProvider, delegatee)
}

// DisableApplication is a paid mutator transaction binding the contract method 0x43445748.
//
// Solidity: function disableApplication(address application) returns()
func (_TokenStaking *TokenStakingTransactor) DisableApplication(opts *bind.TransactOpts, application common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "disableApplication", application)
}

// DisableApplication is a paid mutator transaction binding the contract method 0x43445748.
//
// Solidity: function disableApplication(address application) returns()
func (_TokenStaking *TokenStakingSession) DisableApplication(application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.DisableApplication(&_TokenStaking.TransactOpts, application)
}

// DisableApplication is a paid mutator transaction binding the contract method 0x43445748.
//
// Solidity: function disableApplication(address application) returns()
func (_TokenStaking *TokenStakingTransactorSession) DisableApplication(application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.DisableApplication(&_TokenStaking.TransactOpts, application)
}

// ForceDecreaseAuthorization is a paid mutator transaction binding the contract method 0xb626ca3e.
//
// Solidity: function forceDecreaseAuthorization(address stakingProvider, address application) returns()
func (_TokenStaking *TokenStakingTransactor) ForceDecreaseAuthorization(opts *bind.TransactOpts, stakingProvider common.Address, application common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "forceDecreaseAuthorization", stakingProvider, application)
}

// ForceDecreaseAuthorization is a paid mutator transaction binding the contract method 0xb626ca3e.
//
// Solidity: function forceDecreaseAuthorization(address stakingProvider, address application) returns()
func (_TokenStaking *TokenStakingSession) ForceDecreaseAuthorization(stakingProvider common.Address, application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.ForceDecreaseAuthorization(&_TokenStaking.TransactOpts, stakingProvider, application)
}

// ForceDecreaseAuthorization is a paid mutator transaction binding the contract method 0xb626ca3e.
//
// Solidity: function forceDecreaseAuthorization(address stakingProvider, address application) returns()
func (_TokenStaking *TokenStakingTransactorSession) ForceDecreaseAuthorization(stakingProvider common.Address, application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.ForceDecreaseAuthorization(&_TokenStaking.TransactOpts, stakingProvider, application)
}

// IncreaseAuthorization is a paid mutator transaction binding the contract method 0xf848beff.
//
// Solidity: function increaseAuthorization(address stakingProvider, address application, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) IncreaseAuthorization(opts *bind.TransactOpts, stakingProvider common.Address, application common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "increaseAuthorization", stakingProvider, application, amount)
}

// IncreaseAuthorization is a paid mutator transaction binding the contract method 0xf848beff.
//
// Solidity: function increaseAuthorization(address stakingProvider, address application, uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) IncreaseAuthorization(stakingProvider common.Address, application common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.IncreaseAuthorization(&_TokenStaking.TransactOpts, stakingProvider, application, amount)
}

// IncreaseAuthorization is a paid mutator transaction binding the contract method 0xf848beff.
//
// Solidity: function increaseAuthorization(address stakingProvider, address application, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) IncreaseAuthorization(stakingProvider common.Address, application common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.IncreaseAuthorization(&_TokenStaking.TransactOpts, stakingProvider, application, amount)
}

// Initialize is a paid mutator transaction binding the contract method 0x8129fc1c.
//
// Solidity: function initialize() returns()
func (_TokenStaking *TokenStakingTransactor) Initialize(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "initialize")
}

// Initialize is a paid mutator transaction binding the contract method 0x8129fc1c.
//
// Solidity: function initialize() returns()
func (_TokenStaking *TokenStakingSession) Initialize() (*types.Transaction, error) {
	return _TokenStaking.Contract.Initialize(&_TokenStaking.TransactOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0x8129fc1c.
//
// Solidity: function initialize() returns()
func (_TokenStaking *TokenStakingTransactorSession) Initialize() (*types.Transaction, error) {
	return _TokenStaking.Contract.Initialize(&_TokenStaking.TransactOpts)
}

// NotifyKeepStakeDiscrepancy is a paid mutator transaction binding the contract method 0x402121af.
//
// Solidity: function notifyKeepStakeDiscrepancy(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) NotifyKeepStakeDiscrepancy(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "notifyKeepStakeDiscrepancy", stakingProvider)
}

// NotifyKeepStakeDiscrepancy is a paid mutator transaction binding the contract method 0x402121af.
//
// Solidity: function notifyKeepStakeDiscrepancy(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) NotifyKeepStakeDiscrepancy(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.NotifyKeepStakeDiscrepancy(&_TokenStaking.TransactOpts, stakingProvider)
}

// NotifyKeepStakeDiscrepancy is a paid mutator transaction binding the contract method 0x402121af.
//
// Solidity: function notifyKeepStakeDiscrepancy(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) NotifyKeepStakeDiscrepancy(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.NotifyKeepStakeDiscrepancy(&_TokenStaking.TransactOpts, stakingProvider)
}

// NotifyNuStakeDiscrepancy is a paid mutator transaction binding the contract method 0x8e46ecb6.
//
// Solidity: function notifyNuStakeDiscrepancy(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) NotifyNuStakeDiscrepancy(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "notifyNuStakeDiscrepancy", stakingProvider)
}

// NotifyNuStakeDiscrepancy is a paid mutator transaction binding the contract method 0x8e46ecb6.
//
// Solidity: function notifyNuStakeDiscrepancy(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) NotifyNuStakeDiscrepancy(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.NotifyNuStakeDiscrepancy(&_TokenStaking.TransactOpts, stakingProvider)
}

// NotifyNuStakeDiscrepancy is a paid mutator transaction binding the contract method 0x8e46ecb6.
//
// Solidity: function notifyNuStakeDiscrepancy(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) NotifyNuStakeDiscrepancy(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.NotifyNuStakeDiscrepancy(&_TokenStaking.TransactOpts, stakingProvider)
}

// PauseApplication is a paid mutator transaction binding the contract method 0x2c686ca0.
//
// Solidity: function pauseApplication(address application) returns()
func (_TokenStaking *TokenStakingTransactor) PauseApplication(opts *bind.TransactOpts, application common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "pauseApplication", application)
}

// PauseApplication is a paid mutator transaction binding the contract method 0x2c686ca0.
//
// Solidity: function pauseApplication(address application) returns()
func (_TokenStaking *TokenStakingSession) PauseApplication(application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.PauseApplication(&_TokenStaking.TransactOpts, application)
}

// PauseApplication is a paid mutator transaction binding the contract method 0x2c686ca0.
//
// Solidity: function pauseApplication(address application) returns()
func (_TokenStaking *TokenStakingTransactorSession) PauseApplication(application common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.PauseApplication(&_TokenStaking.TransactOpts, application)
}

// ProcessSlashing is a paid mutator transaction binding the contract method 0xbe2f3351.
//
// Solidity: function processSlashing(uint256 count) returns()
func (_TokenStaking *TokenStakingTransactor) ProcessSlashing(opts *bind.TransactOpts, count *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "processSlashing", count)
}

// ProcessSlashing is a paid mutator transaction binding the contract method 0xbe2f3351.
//
// Solidity: function processSlashing(uint256 count) returns()
func (_TokenStaking *TokenStakingSession) ProcessSlashing(count *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.ProcessSlashing(&_TokenStaking.TransactOpts, count)
}

// ProcessSlashing is a paid mutator transaction binding the contract method 0xbe2f3351.
//
// Solidity: function processSlashing(uint256 count) returns()
func (_TokenStaking *TokenStakingTransactorSession) ProcessSlashing(count *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.ProcessSlashing(&_TokenStaking.TransactOpts, count)
}

// PushNotificationReward is a paid mutator transaction binding the contract method 0x483046bb.
//
// Solidity: function pushNotificationReward(uint96 reward) returns()
func (_TokenStaking *TokenStakingTransactor) PushNotificationReward(opts *bind.TransactOpts, reward *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "pushNotificationReward", reward)
}

// PushNotificationReward is a paid mutator transaction binding the contract method 0x483046bb.
//
// Solidity: function pushNotificationReward(uint96 reward) returns()
func (_TokenStaking *TokenStakingSession) PushNotificationReward(reward *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.PushNotificationReward(&_TokenStaking.TransactOpts, reward)
}

// PushNotificationReward is a paid mutator transaction binding the contract method 0x483046bb.
//
// Solidity: function pushNotificationReward(uint96 reward) returns()
func (_TokenStaking *TokenStakingTransactorSession) PushNotificationReward(reward *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.PushNotificationReward(&_TokenStaking.TransactOpts, reward)
}

// RefreshKeepStakeOwner is a paid mutator transaction binding the contract method 0xaf5f24ad.
//
// Solidity: function refreshKeepStakeOwner(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) RefreshKeepStakeOwner(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "refreshKeepStakeOwner", stakingProvider)
}

// RefreshKeepStakeOwner is a paid mutator transaction binding the contract method 0xaf5f24ad.
//
// Solidity: function refreshKeepStakeOwner(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) RefreshKeepStakeOwner(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.RefreshKeepStakeOwner(&_TokenStaking.TransactOpts, stakingProvider)
}

// RefreshKeepStakeOwner is a paid mutator transaction binding the contract method 0xaf5f24ad.
//
// Solidity: function refreshKeepStakeOwner(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) RefreshKeepStakeOwner(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.RefreshKeepStakeOwner(&_TokenStaking.TransactOpts, stakingProvider)
}

// RequestAuthorizationDecrease is a paid mutator transaction binding the contract method 0x5f2d5030.
//
// Solidity: function requestAuthorizationDecrease(address stakingProvider, address application, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) RequestAuthorizationDecrease(opts *bind.TransactOpts, stakingProvider common.Address, application common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "requestAuthorizationDecrease", stakingProvider, application, amount)
}

// RequestAuthorizationDecrease is a paid mutator transaction binding the contract method 0x5f2d5030.
//
// Solidity: function requestAuthorizationDecrease(address stakingProvider, address application, uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) RequestAuthorizationDecrease(stakingProvider common.Address, application common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.RequestAuthorizationDecrease(&_TokenStaking.TransactOpts, stakingProvider, application, amount)
}

// RequestAuthorizationDecrease is a paid mutator transaction binding the contract method 0x5f2d5030.
//
// Solidity: function requestAuthorizationDecrease(address stakingProvider, address application, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) RequestAuthorizationDecrease(stakingProvider common.Address, application common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.RequestAuthorizationDecrease(&_TokenStaking.TransactOpts, stakingProvider, application, amount)
}

// RequestAuthorizationDecrease0 is a paid mutator transaction binding the contract method 0x86d18a25.
//
// Solidity: function requestAuthorizationDecrease(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) RequestAuthorizationDecrease0(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "requestAuthorizationDecrease0", stakingProvider)
}

// RequestAuthorizationDecrease0 is a paid mutator transaction binding the contract method 0x86d18a25.
//
// Solidity: function requestAuthorizationDecrease(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) RequestAuthorizationDecrease0(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.RequestAuthorizationDecrease0(&_TokenStaking.TransactOpts, stakingProvider)
}

// RequestAuthorizationDecrease0 is a paid mutator transaction binding the contract method 0x86d18a25.
//
// Solidity: function requestAuthorizationDecrease(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) RequestAuthorizationDecrease0(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.RequestAuthorizationDecrease0(&_TokenStaking.TransactOpts, stakingProvider)
}

// Seize is a paid mutator transaction binding the contract method 0x83ddba8f.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, address[] _stakingProviders) returns()
func (_TokenStaking *TokenStakingTransactor) Seize(opts *bind.TransactOpts, amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, _stakingProviders []common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "seize", amount, rewardMultiplier, notifier, _stakingProviders)
}

// Seize is a paid mutator transaction binding the contract method 0x83ddba8f.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, address[] _stakingProviders) returns()
func (_TokenStaking *TokenStakingSession) Seize(amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, _stakingProviders []common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.Seize(&_TokenStaking.TransactOpts, amount, rewardMultiplier, notifier, _stakingProviders)
}

// Seize is a paid mutator transaction binding the contract method 0x83ddba8f.
//
// Solidity: function seize(uint96 amount, uint256 rewardMultiplier, address notifier, address[] _stakingProviders) returns()
func (_TokenStaking *TokenStakingTransactorSession) Seize(amount *big.Int, rewardMultiplier *big.Int, notifier common.Address, _stakingProviders []common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.Seize(&_TokenStaking.TransactOpts, amount, rewardMultiplier, notifier, _stakingProviders)
}

// SetAuthorizationCeiling is a paid mutator transaction binding the contract method 0xb1958150.
//
// Solidity: function setAuthorizationCeiling(uint256 ceiling) returns()
func (_TokenStaking *TokenStakingTransactor) SetAuthorizationCeiling(opts *bind.TransactOpts, ceiling *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "setAuthorizationCeiling", ceiling)
}

// SetAuthorizationCeiling is a paid mutator transaction binding the contract method 0xb1958150.
//
// Solidity: function setAuthorizationCeiling(uint256 ceiling) returns()
func (_TokenStaking *TokenStakingSession) SetAuthorizationCeiling(ceiling *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetAuthorizationCeiling(&_TokenStaking.TransactOpts, ceiling)
}

// SetAuthorizationCeiling is a paid mutator transaction binding the contract method 0xb1958150.
//
// Solidity: function setAuthorizationCeiling(uint256 ceiling) returns()
func (_TokenStaking *TokenStakingTransactorSession) SetAuthorizationCeiling(ceiling *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetAuthorizationCeiling(&_TokenStaking.TransactOpts, ceiling)
}

// SetMinimumStakeAmount is a paid mutator transaction binding the contract method 0x6d08f5b0.
//
// Solidity: function setMinimumStakeAmount(uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) SetMinimumStakeAmount(opts *bind.TransactOpts, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "setMinimumStakeAmount", amount)
}

// SetMinimumStakeAmount is a paid mutator transaction binding the contract method 0x6d08f5b0.
//
// Solidity: function setMinimumStakeAmount(uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) SetMinimumStakeAmount(amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetMinimumStakeAmount(&_TokenStaking.TransactOpts, amount)
}

// SetMinimumStakeAmount is a paid mutator transaction binding the contract method 0x6d08f5b0.
//
// Solidity: function setMinimumStakeAmount(uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) SetMinimumStakeAmount(amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetMinimumStakeAmount(&_TokenStaking.TransactOpts, amount)
}

// SetNotificationReward is a paid mutator transaction binding the contract method 0xd3e25ef3.
//
// Solidity: function setNotificationReward(uint96 reward) returns()
func (_TokenStaking *TokenStakingTransactor) SetNotificationReward(opts *bind.TransactOpts, reward *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "setNotificationReward", reward)
}

// SetNotificationReward is a paid mutator transaction binding the contract method 0xd3e25ef3.
//
// Solidity: function setNotificationReward(uint96 reward) returns()
func (_TokenStaking *TokenStakingSession) SetNotificationReward(reward *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetNotificationReward(&_TokenStaking.TransactOpts, reward)
}

// SetNotificationReward is a paid mutator transaction binding the contract method 0xd3e25ef3.
//
// Solidity: function setNotificationReward(uint96 reward) returns()
func (_TokenStaking *TokenStakingTransactorSession) SetNotificationReward(reward *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetNotificationReward(&_TokenStaking.TransactOpts, reward)
}

// SetPanicButton is a paid mutator transaction binding the contract method 0x1d5270d3.
//
// Solidity: function setPanicButton(address application, address panicButton) returns()
func (_TokenStaking *TokenStakingTransactor) SetPanicButton(opts *bind.TransactOpts, application common.Address, panicButton common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "setPanicButton", application, panicButton)
}

// SetPanicButton is a paid mutator transaction binding the contract method 0x1d5270d3.
//
// Solidity: function setPanicButton(address application, address panicButton) returns()
func (_TokenStaking *TokenStakingSession) SetPanicButton(application common.Address, panicButton common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetPanicButton(&_TokenStaking.TransactOpts, application, panicButton)
}

// SetPanicButton is a paid mutator transaction binding the contract method 0x1d5270d3.
//
// Solidity: function setPanicButton(address application, address panicButton) returns()
func (_TokenStaking *TokenStakingTransactorSession) SetPanicButton(application common.Address, panicButton common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetPanicButton(&_TokenStaking.TransactOpts, application, panicButton)
}

// SetStakeDiscrepancyPenalty is a paid mutator transaction binding the contract method 0x7d0379f0.
//
// Solidity: function setStakeDiscrepancyPenalty(uint96 penalty, uint256 rewardMultiplier) returns()
func (_TokenStaking *TokenStakingTransactor) SetStakeDiscrepancyPenalty(opts *bind.TransactOpts, penalty *big.Int, rewardMultiplier *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "setStakeDiscrepancyPenalty", penalty, rewardMultiplier)
}

// SetStakeDiscrepancyPenalty is a paid mutator transaction binding the contract method 0x7d0379f0.
//
// Solidity: function setStakeDiscrepancyPenalty(uint96 penalty, uint256 rewardMultiplier) returns()
func (_TokenStaking *TokenStakingSession) SetStakeDiscrepancyPenalty(penalty *big.Int, rewardMultiplier *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetStakeDiscrepancyPenalty(&_TokenStaking.TransactOpts, penalty, rewardMultiplier)
}

// SetStakeDiscrepancyPenalty is a paid mutator transaction binding the contract method 0x7d0379f0.
//
// Solidity: function setStakeDiscrepancyPenalty(uint96 penalty, uint256 rewardMultiplier) returns()
func (_TokenStaking *TokenStakingTransactorSession) SetStakeDiscrepancyPenalty(penalty *big.Int, rewardMultiplier *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.SetStakeDiscrepancyPenalty(&_TokenStaking.TransactOpts, penalty, rewardMultiplier)
}

// Slash is a paid mutator transaction binding the contract method 0xf07f91c5.
//
// Solidity: function slash(uint96 amount, address[] _stakingProviders) returns()
func (_TokenStaking *TokenStakingTransactor) Slash(opts *bind.TransactOpts, amount *big.Int, _stakingProviders []common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "slash", amount, _stakingProviders)
}

// Slash is a paid mutator transaction binding the contract method 0xf07f91c5.
//
// Solidity: function slash(uint96 amount, address[] _stakingProviders) returns()
func (_TokenStaking *TokenStakingSession) Slash(amount *big.Int, _stakingProviders []common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.Slash(&_TokenStaking.TransactOpts, amount, _stakingProviders)
}

// Slash is a paid mutator transaction binding the contract method 0xf07f91c5.
//
// Solidity: function slash(uint96 amount, address[] _stakingProviders) returns()
func (_TokenStaking *TokenStakingTransactorSession) Slash(amount *big.Int, _stakingProviders []common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.Slash(&_TokenStaking.TransactOpts, amount, _stakingProviders)
}

// Stake is a paid mutator transaction binding the contract method 0x5961d5e9.
//
// Solidity: function stake(address stakingProvider, address beneficiary, address authorizer, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) Stake(opts *bind.TransactOpts, stakingProvider common.Address, beneficiary common.Address, authorizer common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "stake", stakingProvider, beneficiary, authorizer, amount)
}

// Stake is a paid mutator transaction binding the contract method 0x5961d5e9.
//
// Solidity: function stake(address stakingProvider, address beneficiary, address authorizer, uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) Stake(stakingProvider common.Address, beneficiary common.Address, authorizer common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.Stake(&_TokenStaking.TransactOpts, stakingProvider, beneficiary, authorizer, amount)
}

// Stake is a paid mutator transaction binding the contract method 0x5961d5e9.
//
// Solidity: function stake(address stakingProvider, address beneficiary, address authorizer, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) Stake(stakingProvider common.Address, beneficiary common.Address, authorizer common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.Stake(&_TokenStaking.TransactOpts, stakingProvider, beneficiary, authorizer, amount)
}

// StakeKeep is a paid mutator transaction binding the contract method 0x570ea461.
//
// Solidity: function stakeKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) StakeKeep(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "stakeKeep", stakingProvider)
}

// StakeKeep is a paid mutator transaction binding the contract method 0x570ea461.
//
// Solidity: function stakeKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) StakeKeep(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.StakeKeep(&_TokenStaking.TransactOpts, stakingProvider)
}

// StakeKeep is a paid mutator transaction binding the contract method 0x570ea461.
//
// Solidity: function stakeKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) StakeKeep(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.StakeKeep(&_TokenStaking.TransactOpts, stakingProvider)
}

// StakeNu is a paid mutator transaction binding the contract method 0x81b0a0ce.
//
// Solidity: function stakeNu(address stakingProvider, address beneficiary, address authorizer) returns()
func (_TokenStaking *TokenStakingTransactor) StakeNu(opts *bind.TransactOpts, stakingProvider common.Address, beneficiary common.Address, authorizer common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "stakeNu", stakingProvider, beneficiary, authorizer)
}

// StakeNu is a paid mutator transaction binding the contract method 0x81b0a0ce.
//
// Solidity: function stakeNu(address stakingProvider, address beneficiary, address authorizer) returns()
func (_TokenStaking *TokenStakingSession) StakeNu(stakingProvider common.Address, beneficiary common.Address, authorizer common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.StakeNu(&_TokenStaking.TransactOpts, stakingProvider, beneficiary, authorizer)
}

// StakeNu is a paid mutator transaction binding the contract method 0x81b0a0ce.
//
// Solidity: function stakeNu(address stakingProvider, address beneficiary, address authorizer) returns()
func (_TokenStaking *TokenStakingTransactorSession) StakeNu(stakingProvider common.Address, beneficiary common.Address, authorizer common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.StakeNu(&_TokenStaking.TransactOpts, stakingProvider, beneficiary, authorizer)
}

// TopUp is a paid mutator transaction binding the contract method 0x28c8c55f.
//
// Solidity: function topUp(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) TopUp(opts *bind.TransactOpts, stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "topUp", stakingProvider, amount)
}

// TopUp is a paid mutator transaction binding the contract method 0x28c8c55f.
//
// Solidity: function topUp(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) TopUp(stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.TopUp(&_TokenStaking.TransactOpts, stakingProvider, amount)
}

// TopUp is a paid mutator transaction binding the contract method 0x28c8c55f.
//
// Solidity: function topUp(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) TopUp(stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.TopUp(&_TokenStaking.TransactOpts, stakingProvider, amount)
}

// TopUpKeep is a paid mutator transaction binding the contract method 0xef47bf40.
//
// Solidity: function topUpKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) TopUpKeep(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "topUpKeep", stakingProvider)
}

// TopUpKeep is a paid mutator transaction binding the contract method 0xef47bf40.
//
// Solidity: function topUpKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) TopUpKeep(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.TopUpKeep(&_TokenStaking.TransactOpts, stakingProvider)
}

// TopUpKeep is a paid mutator transaction binding the contract method 0xef47bf40.
//
// Solidity: function topUpKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) TopUpKeep(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.TopUpKeep(&_TokenStaking.TransactOpts, stakingProvider)
}

// TopUpNu is a paid mutator transaction binding the contract method 0x56f958ee.
//
// Solidity: function topUpNu(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) TopUpNu(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "topUpNu", stakingProvider)
}

// TopUpNu is a paid mutator transaction binding the contract method 0x56f958ee.
//
// Solidity: function topUpNu(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) TopUpNu(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.TopUpNu(&_TokenStaking.TransactOpts, stakingProvider)
}

// TopUpNu is a paid mutator transaction binding the contract method 0x56f958ee.
//
// Solidity: function topUpNu(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) TopUpNu(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.TopUpNu(&_TokenStaking.TransactOpts, stakingProvider)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGuvnor) returns()
func (_TokenStaking *TokenStakingTransactor) TransferGovernance(opts *bind.TransactOpts, newGuvnor common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "transferGovernance", newGuvnor)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGuvnor) returns()
func (_TokenStaking *TokenStakingSession) TransferGovernance(newGuvnor common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.TransferGovernance(&_TokenStaking.TransactOpts, newGuvnor)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGuvnor) returns()
func (_TokenStaking *TokenStakingTransactorSession) TransferGovernance(newGuvnor common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.TransferGovernance(&_TokenStaking.TransactOpts, newGuvnor)
}

// UnstakeAll is a paid mutator transaction binding the contract method 0xa0d6ff9a.
//
// Solidity: function unstakeAll(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) UnstakeAll(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "unstakeAll", stakingProvider)
}

// UnstakeAll is a paid mutator transaction binding the contract method 0xa0d6ff9a.
//
// Solidity: function unstakeAll(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) UnstakeAll(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeAll(&_TokenStaking.TransactOpts, stakingProvider)
}

// UnstakeAll is a paid mutator transaction binding the contract method 0xa0d6ff9a.
//
// Solidity: function unstakeAll(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) UnstakeAll(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeAll(&_TokenStaking.TransactOpts, stakingProvider)
}

// UnstakeKeep is a paid mutator transaction binding the contract method 0x4ec0a9fe.
//
// Solidity: function unstakeKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactor) UnstakeKeep(opts *bind.TransactOpts, stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "unstakeKeep", stakingProvider)
}

// UnstakeKeep is a paid mutator transaction binding the contract method 0x4ec0a9fe.
//
// Solidity: function unstakeKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingSession) UnstakeKeep(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeKeep(&_TokenStaking.TransactOpts, stakingProvider)
}

// UnstakeKeep is a paid mutator transaction binding the contract method 0x4ec0a9fe.
//
// Solidity: function unstakeKeep(address stakingProvider) returns()
func (_TokenStaking *TokenStakingTransactorSession) UnstakeKeep(stakingProvider common.Address) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeKeep(&_TokenStaking.TransactOpts, stakingProvider)
}

// UnstakeNu is a paid mutator transaction binding the contract method 0x58ccdf38.
//
// Solidity: function unstakeNu(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) UnstakeNu(opts *bind.TransactOpts, stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "unstakeNu", stakingProvider, amount)
}

// UnstakeNu is a paid mutator transaction binding the contract method 0x58ccdf38.
//
// Solidity: function unstakeNu(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) UnstakeNu(stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeNu(&_TokenStaking.TransactOpts, stakingProvider, amount)
}

// UnstakeNu is a paid mutator transaction binding the contract method 0x58ccdf38.
//
// Solidity: function unstakeNu(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) UnstakeNu(stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeNu(&_TokenStaking.TransactOpts, stakingProvider, amount)
}

// UnstakeT is a paid mutator transaction binding the contract method 0xd3ecb6cd.
//
// Solidity: function unstakeT(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) UnstakeT(opts *bind.TransactOpts, stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "unstakeT", stakingProvider, amount)
}

// UnstakeT is a paid mutator transaction binding the contract method 0xd3ecb6cd.
//
// Solidity: function unstakeT(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) UnstakeT(stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeT(&_TokenStaking.TransactOpts, stakingProvider, amount)
}

// UnstakeT is a paid mutator transaction binding the contract method 0xd3ecb6cd.
//
// Solidity: function unstakeT(address stakingProvider, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) UnstakeT(stakingProvider common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.UnstakeT(&_TokenStaking.TransactOpts, stakingProvider, amount)
}

// WithdrawNotificationReward is a paid mutator transaction binding the contract method 0x6d9b9a34.
//
// Solidity: function withdrawNotificationReward(address recipient, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactor) WithdrawNotificationReward(opts *bind.TransactOpts, recipient common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.contract.Transact(opts, "withdrawNotificationReward", recipient, amount)
}

// WithdrawNotificationReward is a paid mutator transaction binding the contract method 0x6d9b9a34.
//
// Solidity: function withdrawNotificationReward(address recipient, uint96 amount) returns()
func (_TokenStaking *TokenStakingSession) WithdrawNotificationReward(recipient common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.WithdrawNotificationReward(&_TokenStaking.TransactOpts, recipient, amount)
}

// WithdrawNotificationReward is a paid mutator transaction binding the contract method 0x6d9b9a34.
//
// Solidity: function withdrawNotificationReward(address recipient, uint96 amount) returns()
func (_TokenStaking *TokenStakingTransactorSession) WithdrawNotificationReward(recipient common.Address, amount *big.Int) (*types.Transaction, error) {
	return _TokenStaking.Contract.WithdrawNotificationReward(&_TokenStaking.TransactOpts, recipient, amount)
}

// TokenStakingApplicationStatusChangedIterator is returned from FilterApplicationStatusChanged and is used to iterate over the raw logs and unpacked data for ApplicationStatusChanged events raised by the TokenStaking contract.
type TokenStakingApplicationStatusChangedIterator struct {
	Event *TokenStakingApplicationStatusChanged // Event containing the contract specifics and raw log

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
func (it *TokenStakingApplicationStatusChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingApplicationStatusChanged)
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
		it.Event = new(TokenStakingApplicationStatusChanged)
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
func (it *TokenStakingApplicationStatusChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingApplicationStatusChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingApplicationStatusChanged represents a ApplicationStatusChanged event raised by the TokenStaking contract.
type TokenStakingApplicationStatusChanged struct {
	Application common.Address
	NewStatus   uint8
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterApplicationStatusChanged is a free log retrieval operation binding the contract event 0x96a3c8e9780312d2e82e746b65a21aaca458dd91c375fea6066416ef241e87cb.
//
// Solidity: event ApplicationStatusChanged(address indexed application, uint8 indexed newStatus)
func (_TokenStaking *TokenStakingFilterer) FilterApplicationStatusChanged(opts *bind.FilterOpts, application []common.Address, newStatus []uint8) (*TokenStakingApplicationStatusChangedIterator, error) {

	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}
	var newStatusRule []interface{}
	for _, newStatusItem := range newStatus {
		newStatusRule = append(newStatusRule, newStatusItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "ApplicationStatusChanged", applicationRule, newStatusRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingApplicationStatusChangedIterator{contract: _TokenStaking.contract, event: "ApplicationStatusChanged", logs: logs, sub: sub}, nil
}

// WatchApplicationStatusChanged is a free log subscription operation binding the contract event 0x96a3c8e9780312d2e82e746b65a21aaca458dd91c375fea6066416ef241e87cb.
//
// Solidity: event ApplicationStatusChanged(address indexed application, uint8 indexed newStatus)
func (_TokenStaking *TokenStakingFilterer) WatchApplicationStatusChanged(opts *bind.WatchOpts, sink chan<- *TokenStakingApplicationStatusChanged, application []common.Address, newStatus []uint8) (event.Subscription, error) {

	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}
	var newStatusRule []interface{}
	for _, newStatusItem := range newStatus {
		newStatusRule = append(newStatusRule, newStatusItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "ApplicationStatusChanged", applicationRule, newStatusRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingApplicationStatusChanged)
				if err := _TokenStaking.contract.UnpackLog(event, "ApplicationStatusChanged", log); err != nil {
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

// ParseApplicationStatusChanged is a log parse operation binding the contract event 0x96a3c8e9780312d2e82e746b65a21aaca458dd91c375fea6066416ef241e87cb.
//
// Solidity: event ApplicationStatusChanged(address indexed application, uint8 indexed newStatus)
func (_TokenStaking *TokenStakingFilterer) ParseApplicationStatusChanged(log types.Log) (*TokenStakingApplicationStatusChanged, error) {
	event := new(TokenStakingApplicationStatusChanged)
	if err := _TokenStaking.contract.UnpackLog(event, "ApplicationStatusChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingAuthorizationCeilingSetIterator is returned from FilterAuthorizationCeilingSet and is used to iterate over the raw logs and unpacked data for AuthorizationCeilingSet events raised by the TokenStaking contract.
type TokenStakingAuthorizationCeilingSetIterator struct {
	Event *TokenStakingAuthorizationCeilingSet // Event containing the contract specifics and raw log

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
func (it *TokenStakingAuthorizationCeilingSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingAuthorizationCeilingSet)
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
		it.Event = new(TokenStakingAuthorizationCeilingSet)
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
func (it *TokenStakingAuthorizationCeilingSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingAuthorizationCeilingSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingAuthorizationCeilingSet represents a AuthorizationCeilingSet event raised by the TokenStaking contract.
type TokenStakingAuthorizationCeilingSet struct {
	Ceiling *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationCeilingSet is a free log retrieval operation binding the contract event 0xf82f602e8097a5c312216f60ca94cd1ff03aac29893f9adef7ed7e6ae33c76e2.
//
// Solidity: event AuthorizationCeilingSet(uint256 ceiling)
func (_TokenStaking *TokenStakingFilterer) FilterAuthorizationCeilingSet(opts *bind.FilterOpts) (*TokenStakingAuthorizationCeilingSetIterator, error) {

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "AuthorizationCeilingSet")
	if err != nil {
		return nil, err
	}
	return &TokenStakingAuthorizationCeilingSetIterator{contract: _TokenStaking.contract, event: "AuthorizationCeilingSet", logs: logs, sub: sub}, nil
}

// WatchAuthorizationCeilingSet is a free log subscription operation binding the contract event 0xf82f602e8097a5c312216f60ca94cd1ff03aac29893f9adef7ed7e6ae33c76e2.
//
// Solidity: event AuthorizationCeilingSet(uint256 ceiling)
func (_TokenStaking *TokenStakingFilterer) WatchAuthorizationCeilingSet(opts *bind.WatchOpts, sink chan<- *TokenStakingAuthorizationCeilingSet) (event.Subscription, error) {

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "AuthorizationCeilingSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingAuthorizationCeilingSet)
				if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationCeilingSet", log); err != nil {
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

// ParseAuthorizationCeilingSet is a log parse operation binding the contract event 0xf82f602e8097a5c312216f60ca94cd1ff03aac29893f9adef7ed7e6ae33c76e2.
//
// Solidity: event AuthorizationCeilingSet(uint256 ceiling)
func (_TokenStaking *TokenStakingFilterer) ParseAuthorizationCeilingSet(log types.Log) (*TokenStakingAuthorizationCeilingSet, error) {
	event := new(TokenStakingAuthorizationCeilingSet)
	if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationCeilingSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingAuthorizationDecreaseApprovedIterator is returned from FilterAuthorizationDecreaseApproved and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseApproved events raised by the TokenStaking contract.
type TokenStakingAuthorizationDecreaseApprovedIterator struct {
	Event *TokenStakingAuthorizationDecreaseApproved // Event containing the contract specifics and raw log

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
func (it *TokenStakingAuthorizationDecreaseApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingAuthorizationDecreaseApproved)
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
		it.Event = new(TokenStakingAuthorizationDecreaseApproved)
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
func (it *TokenStakingAuthorizationDecreaseApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingAuthorizationDecreaseApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingAuthorizationDecreaseApproved represents a AuthorizationDecreaseApproved event raised by the TokenStaking contract.
type TokenStakingAuthorizationDecreaseApproved struct {
	StakingProvider common.Address
	Application     common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationDecreaseApproved is a free log retrieval operation binding the contract event 0xdfabb38007f28b342b8f536c8c832f746c2a53627133be65453ac0b98968f40e.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) FilterAuthorizationDecreaseApproved(opts *bind.FilterOpts, stakingProvider []common.Address, application []common.Address) (*TokenStakingAuthorizationDecreaseApprovedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule, applicationRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingAuthorizationDecreaseApprovedIterator{contract: _TokenStaking.contract, event: "AuthorizationDecreaseApproved", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseApproved is a free log subscription operation binding the contract event 0xdfabb38007f28b342b8f536c8c832f746c2a53627133be65453ac0b98968f40e.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) WatchAuthorizationDecreaseApproved(opts *bind.WatchOpts, sink chan<- *TokenStakingAuthorizationDecreaseApproved, stakingProvider []common.Address, application []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "AuthorizationDecreaseApproved", stakingProviderRule, applicationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingAuthorizationDecreaseApproved)
				if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
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

// ParseAuthorizationDecreaseApproved is a log parse operation binding the contract event 0xdfabb38007f28b342b8f536c8c832f746c2a53627133be65453ac0b98968f40e.
//
// Solidity: event AuthorizationDecreaseApproved(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) ParseAuthorizationDecreaseApproved(log types.Log) (*TokenStakingAuthorizationDecreaseApproved, error) {
	event := new(TokenStakingAuthorizationDecreaseApproved)
	if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationDecreaseApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingAuthorizationDecreaseRequestedIterator is returned from FilterAuthorizationDecreaseRequested and is used to iterate over the raw logs and unpacked data for AuthorizationDecreaseRequested events raised by the TokenStaking contract.
type TokenStakingAuthorizationDecreaseRequestedIterator struct {
	Event *TokenStakingAuthorizationDecreaseRequested // Event containing the contract specifics and raw log

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
func (it *TokenStakingAuthorizationDecreaseRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingAuthorizationDecreaseRequested)
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
		it.Event = new(TokenStakingAuthorizationDecreaseRequested)
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
func (it *TokenStakingAuthorizationDecreaseRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingAuthorizationDecreaseRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingAuthorizationDecreaseRequested represents a AuthorizationDecreaseRequested event raised by the TokenStaking contract.
type TokenStakingAuthorizationDecreaseRequested struct {
	StakingProvider common.Address
	Application     common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationDecreaseRequested is a free log retrieval operation binding the contract event 0x132e76775c4e3b4b2c36fe4acc18d539b6c34b984ac6ba494a5996c48d8a0174.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) FilterAuthorizationDecreaseRequested(opts *bind.FilterOpts, stakingProvider []common.Address, application []common.Address) (*TokenStakingAuthorizationDecreaseRequestedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, applicationRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingAuthorizationDecreaseRequestedIterator{contract: _TokenStaking.contract, event: "AuthorizationDecreaseRequested", logs: logs, sub: sub}, nil
}

// WatchAuthorizationDecreaseRequested is a free log subscription operation binding the contract event 0x132e76775c4e3b4b2c36fe4acc18d539b6c34b984ac6ba494a5996c48d8a0174.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) WatchAuthorizationDecreaseRequested(opts *bind.WatchOpts, sink chan<- *TokenStakingAuthorizationDecreaseRequested, stakingProvider []common.Address, application []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "AuthorizationDecreaseRequested", stakingProviderRule, applicationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingAuthorizationDecreaseRequested)
				if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
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

// ParseAuthorizationDecreaseRequested is a log parse operation binding the contract event 0x132e76775c4e3b4b2c36fe4acc18d539b6c34b984ac6ba494a5996c48d8a0174.
//
// Solidity: event AuthorizationDecreaseRequested(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) ParseAuthorizationDecreaseRequested(log types.Log) (*TokenStakingAuthorizationDecreaseRequested, error) {
	event := new(TokenStakingAuthorizationDecreaseRequested)
	if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationDecreaseRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingAuthorizationIncreasedIterator is returned from FilterAuthorizationIncreased and is used to iterate over the raw logs and unpacked data for AuthorizationIncreased events raised by the TokenStaking contract.
type TokenStakingAuthorizationIncreasedIterator struct {
	Event *TokenStakingAuthorizationIncreased // Event containing the contract specifics and raw log

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
func (it *TokenStakingAuthorizationIncreasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingAuthorizationIncreased)
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
		it.Event = new(TokenStakingAuthorizationIncreased)
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
func (it *TokenStakingAuthorizationIncreasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingAuthorizationIncreasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingAuthorizationIncreased represents a AuthorizationIncreased event raised by the TokenStaking contract.
type TokenStakingAuthorizationIncreased struct {
	StakingProvider common.Address
	Application     common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationIncreased is a free log retrieval operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) FilterAuthorizationIncreased(opts *bind.FilterOpts, stakingProvider []common.Address, application []common.Address) (*TokenStakingAuthorizationIncreasedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "AuthorizationIncreased", stakingProviderRule, applicationRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingAuthorizationIncreasedIterator{contract: _TokenStaking.contract, event: "AuthorizationIncreased", logs: logs, sub: sub}, nil
}

// WatchAuthorizationIncreased is a free log subscription operation binding the contract event 0x87f9f9f59204f53d57a89a817c6083a17979cd0531791c91e18551a56e3cfdd7.
//
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) WatchAuthorizationIncreased(opts *bind.WatchOpts, sink chan<- *TokenStakingAuthorizationIncreased, stakingProvider []common.Address, application []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "AuthorizationIncreased", stakingProviderRule, applicationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingAuthorizationIncreased)
				if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
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
// Solidity: event AuthorizationIncreased(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount)
func (_TokenStaking *TokenStakingFilterer) ParseAuthorizationIncreased(log types.Log) (*TokenStakingAuthorizationIncreased, error) {
	event := new(TokenStakingAuthorizationIncreased)
	if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationIncreased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingAuthorizationInvoluntaryDecreasedIterator is returned from FilterAuthorizationInvoluntaryDecreased and is used to iterate over the raw logs and unpacked data for AuthorizationInvoluntaryDecreased events raised by the TokenStaking contract.
type TokenStakingAuthorizationInvoluntaryDecreasedIterator struct {
	Event *TokenStakingAuthorizationInvoluntaryDecreased // Event containing the contract specifics and raw log

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
func (it *TokenStakingAuthorizationInvoluntaryDecreasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingAuthorizationInvoluntaryDecreased)
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
		it.Event = new(TokenStakingAuthorizationInvoluntaryDecreased)
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
func (it *TokenStakingAuthorizationInvoluntaryDecreasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingAuthorizationInvoluntaryDecreasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingAuthorizationInvoluntaryDecreased represents a AuthorizationInvoluntaryDecreased event raised by the TokenStaking contract.
type TokenStakingAuthorizationInvoluntaryDecreased struct {
	StakingProvider common.Address
	Application     common.Address
	FromAmount      *big.Int
	ToAmount        *big.Int
	SuccessfulCall  bool
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationInvoluntaryDecreased is a free log retrieval operation binding the contract event 0x0f0171fffaa54732b1f79a3164b315658061a1a51bf8c1010fbed908a8e333f9.
//
// Solidity: event AuthorizationInvoluntaryDecreased(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount, bool indexed successfulCall)
func (_TokenStaking *TokenStakingFilterer) FilterAuthorizationInvoluntaryDecreased(opts *bind.FilterOpts, stakingProvider []common.Address, application []common.Address, successfulCall []bool) (*TokenStakingAuthorizationInvoluntaryDecreasedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	var successfulCallRule []interface{}
	for _, successfulCallItem := range successfulCall {
		successfulCallRule = append(successfulCallRule, successfulCallItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "AuthorizationInvoluntaryDecreased", stakingProviderRule, applicationRule, successfulCallRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingAuthorizationInvoluntaryDecreasedIterator{contract: _TokenStaking.contract, event: "AuthorizationInvoluntaryDecreased", logs: logs, sub: sub}, nil
}

// WatchAuthorizationInvoluntaryDecreased is a free log subscription operation binding the contract event 0x0f0171fffaa54732b1f79a3164b315658061a1a51bf8c1010fbed908a8e333f9.
//
// Solidity: event AuthorizationInvoluntaryDecreased(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount, bool indexed successfulCall)
func (_TokenStaking *TokenStakingFilterer) WatchAuthorizationInvoluntaryDecreased(opts *bind.WatchOpts, sink chan<- *TokenStakingAuthorizationInvoluntaryDecreased, stakingProvider []common.Address, application []common.Address, successfulCall []bool) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}

	var successfulCallRule []interface{}
	for _, successfulCallItem := range successfulCall {
		successfulCallRule = append(successfulCallRule, successfulCallItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "AuthorizationInvoluntaryDecreased", stakingProviderRule, applicationRule, successfulCallRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingAuthorizationInvoluntaryDecreased)
				if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationInvoluntaryDecreased", log); err != nil {
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

// ParseAuthorizationInvoluntaryDecreased is a log parse operation binding the contract event 0x0f0171fffaa54732b1f79a3164b315658061a1a51bf8c1010fbed908a8e333f9.
//
// Solidity: event AuthorizationInvoluntaryDecreased(address indexed stakingProvider, address indexed application, uint96 fromAmount, uint96 toAmount, bool indexed successfulCall)
func (_TokenStaking *TokenStakingFilterer) ParseAuthorizationInvoluntaryDecreased(log types.Log) (*TokenStakingAuthorizationInvoluntaryDecreased, error) {
	event := new(TokenStakingAuthorizationInvoluntaryDecreased)
	if err := _TokenStaking.contract.UnpackLog(event, "AuthorizationInvoluntaryDecreased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingDelegateChangedIterator is returned from FilterDelegateChanged and is used to iterate over the raw logs and unpacked data for DelegateChanged events raised by the TokenStaking contract.
type TokenStakingDelegateChangedIterator struct {
	Event *TokenStakingDelegateChanged // Event containing the contract specifics and raw log

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
func (it *TokenStakingDelegateChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingDelegateChanged)
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
		it.Event = new(TokenStakingDelegateChanged)
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
func (it *TokenStakingDelegateChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingDelegateChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingDelegateChanged represents a DelegateChanged event raised by the TokenStaking contract.
type TokenStakingDelegateChanged struct {
	Delegator    common.Address
	FromDelegate common.Address
	ToDelegate   common.Address
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterDelegateChanged is a free log retrieval operation binding the contract event 0x3134e8a2e6d97e929a7e54011ea5485d7d196dd5f0ba4d4ef95803e8e3fc257f.
//
// Solidity: event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate)
func (_TokenStaking *TokenStakingFilterer) FilterDelegateChanged(opts *bind.FilterOpts, delegator []common.Address, fromDelegate []common.Address, toDelegate []common.Address) (*TokenStakingDelegateChangedIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var fromDelegateRule []interface{}
	for _, fromDelegateItem := range fromDelegate {
		fromDelegateRule = append(fromDelegateRule, fromDelegateItem)
	}
	var toDelegateRule []interface{}
	for _, toDelegateItem := range toDelegate {
		toDelegateRule = append(toDelegateRule, toDelegateItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "DelegateChanged", delegatorRule, fromDelegateRule, toDelegateRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingDelegateChangedIterator{contract: _TokenStaking.contract, event: "DelegateChanged", logs: logs, sub: sub}, nil
}

// WatchDelegateChanged is a free log subscription operation binding the contract event 0x3134e8a2e6d97e929a7e54011ea5485d7d196dd5f0ba4d4ef95803e8e3fc257f.
//
// Solidity: event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate)
func (_TokenStaking *TokenStakingFilterer) WatchDelegateChanged(opts *bind.WatchOpts, sink chan<- *TokenStakingDelegateChanged, delegator []common.Address, fromDelegate []common.Address, toDelegate []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var fromDelegateRule []interface{}
	for _, fromDelegateItem := range fromDelegate {
		fromDelegateRule = append(fromDelegateRule, fromDelegateItem)
	}
	var toDelegateRule []interface{}
	for _, toDelegateItem := range toDelegate {
		toDelegateRule = append(toDelegateRule, toDelegateItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "DelegateChanged", delegatorRule, fromDelegateRule, toDelegateRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingDelegateChanged)
				if err := _TokenStaking.contract.UnpackLog(event, "DelegateChanged", log); err != nil {
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

// ParseDelegateChanged is a log parse operation binding the contract event 0x3134e8a2e6d97e929a7e54011ea5485d7d196dd5f0ba4d4ef95803e8e3fc257f.
//
// Solidity: event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate)
func (_TokenStaking *TokenStakingFilterer) ParseDelegateChanged(log types.Log) (*TokenStakingDelegateChanged, error) {
	event := new(TokenStakingDelegateChanged)
	if err := _TokenStaking.contract.UnpackLog(event, "DelegateChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingDelegateVotesChangedIterator is returned from FilterDelegateVotesChanged and is used to iterate over the raw logs and unpacked data for DelegateVotesChanged events raised by the TokenStaking contract.
type TokenStakingDelegateVotesChangedIterator struct {
	Event *TokenStakingDelegateVotesChanged // Event containing the contract specifics and raw log

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
func (it *TokenStakingDelegateVotesChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingDelegateVotesChanged)
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
		it.Event = new(TokenStakingDelegateVotesChanged)
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
func (it *TokenStakingDelegateVotesChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingDelegateVotesChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingDelegateVotesChanged represents a DelegateVotesChanged event raised by the TokenStaking contract.
type TokenStakingDelegateVotesChanged struct {
	Delegate        common.Address
	PreviousBalance *big.Int
	NewBalance      *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterDelegateVotesChanged is a free log retrieval operation binding the contract event 0xdec2bacdd2f05b59de34da9b523dff8be42e5e38e818c82fdb0bae774387a724.
//
// Solidity: event DelegateVotesChanged(address indexed delegate, uint256 previousBalance, uint256 newBalance)
func (_TokenStaking *TokenStakingFilterer) FilterDelegateVotesChanged(opts *bind.FilterOpts, delegate []common.Address) (*TokenStakingDelegateVotesChangedIterator, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "DelegateVotesChanged", delegateRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingDelegateVotesChangedIterator{contract: _TokenStaking.contract, event: "DelegateVotesChanged", logs: logs, sub: sub}, nil
}

// WatchDelegateVotesChanged is a free log subscription operation binding the contract event 0xdec2bacdd2f05b59de34da9b523dff8be42e5e38e818c82fdb0bae774387a724.
//
// Solidity: event DelegateVotesChanged(address indexed delegate, uint256 previousBalance, uint256 newBalance)
func (_TokenStaking *TokenStakingFilterer) WatchDelegateVotesChanged(opts *bind.WatchOpts, sink chan<- *TokenStakingDelegateVotesChanged, delegate []common.Address) (event.Subscription, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "DelegateVotesChanged", delegateRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingDelegateVotesChanged)
				if err := _TokenStaking.contract.UnpackLog(event, "DelegateVotesChanged", log); err != nil {
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

// ParseDelegateVotesChanged is a log parse operation binding the contract event 0xdec2bacdd2f05b59de34da9b523dff8be42e5e38e818c82fdb0bae774387a724.
//
// Solidity: event DelegateVotesChanged(address indexed delegate, uint256 previousBalance, uint256 newBalance)
func (_TokenStaking *TokenStakingFilterer) ParseDelegateVotesChanged(log types.Log) (*TokenStakingDelegateVotesChanged, error) {
	event := new(TokenStakingDelegateVotesChanged)
	if err := _TokenStaking.contract.UnpackLog(event, "DelegateVotesChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingGovernanceTransferredIterator is returned from FilterGovernanceTransferred and is used to iterate over the raw logs and unpacked data for GovernanceTransferred events raised by the TokenStaking contract.
type TokenStakingGovernanceTransferredIterator struct {
	Event *TokenStakingGovernanceTransferred // Event containing the contract specifics and raw log

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
func (it *TokenStakingGovernanceTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingGovernanceTransferred)
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
		it.Event = new(TokenStakingGovernanceTransferred)
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
func (it *TokenStakingGovernanceTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingGovernanceTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingGovernanceTransferred represents a GovernanceTransferred event raised by the TokenStaking contract.
type TokenStakingGovernanceTransferred struct {
	OldGovernance common.Address
	NewGovernance common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterGovernanceTransferred is a free log retrieval operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_TokenStaking *TokenStakingFilterer) FilterGovernanceTransferred(opts *bind.FilterOpts) (*TokenStakingGovernanceTransferredIterator, error) {

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return &TokenStakingGovernanceTransferredIterator{contract: _TokenStaking.contract, event: "GovernanceTransferred", logs: logs, sub: sub}, nil
}

// WatchGovernanceTransferred is a free log subscription operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_TokenStaking *TokenStakingFilterer) WatchGovernanceTransferred(opts *bind.WatchOpts, sink chan<- *TokenStakingGovernanceTransferred) (event.Subscription, error) {

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingGovernanceTransferred)
				if err := _TokenStaking.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
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
func (_TokenStaking *TokenStakingFilterer) ParseGovernanceTransferred(log types.Log) (*TokenStakingGovernanceTransferred, error) {
	event := new(TokenStakingGovernanceTransferred)
	if err := _TokenStaking.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingMinimumStakeAmountSetIterator is returned from FilterMinimumStakeAmountSet and is used to iterate over the raw logs and unpacked data for MinimumStakeAmountSet events raised by the TokenStaking contract.
type TokenStakingMinimumStakeAmountSetIterator struct {
	Event *TokenStakingMinimumStakeAmountSet // Event containing the contract specifics and raw log

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
func (it *TokenStakingMinimumStakeAmountSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingMinimumStakeAmountSet)
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
		it.Event = new(TokenStakingMinimumStakeAmountSet)
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
func (it *TokenStakingMinimumStakeAmountSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingMinimumStakeAmountSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingMinimumStakeAmountSet represents a MinimumStakeAmountSet event raised by the TokenStaking contract.
type TokenStakingMinimumStakeAmountSet struct {
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterMinimumStakeAmountSet is a free log retrieval operation binding the contract event 0x91d1e8918c0ec490b6eccd803db78273458f0a7d4b3915e062f1402e9521f518.
//
// Solidity: event MinimumStakeAmountSet(uint96 amount)
func (_TokenStaking *TokenStakingFilterer) FilterMinimumStakeAmountSet(opts *bind.FilterOpts) (*TokenStakingMinimumStakeAmountSetIterator, error) {

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "MinimumStakeAmountSet")
	if err != nil {
		return nil, err
	}
	return &TokenStakingMinimumStakeAmountSetIterator{contract: _TokenStaking.contract, event: "MinimumStakeAmountSet", logs: logs, sub: sub}, nil
}

// WatchMinimumStakeAmountSet is a free log subscription operation binding the contract event 0x91d1e8918c0ec490b6eccd803db78273458f0a7d4b3915e062f1402e9521f518.
//
// Solidity: event MinimumStakeAmountSet(uint96 amount)
func (_TokenStaking *TokenStakingFilterer) WatchMinimumStakeAmountSet(opts *bind.WatchOpts, sink chan<- *TokenStakingMinimumStakeAmountSet) (event.Subscription, error) {

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "MinimumStakeAmountSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingMinimumStakeAmountSet)
				if err := _TokenStaking.contract.UnpackLog(event, "MinimumStakeAmountSet", log); err != nil {
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

// ParseMinimumStakeAmountSet is a log parse operation binding the contract event 0x91d1e8918c0ec490b6eccd803db78273458f0a7d4b3915e062f1402e9521f518.
//
// Solidity: event MinimumStakeAmountSet(uint96 amount)
func (_TokenStaking *TokenStakingFilterer) ParseMinimumStakeAmountSet(log types.Log) (*TokenStakingMinimumStakeAmountSet, error) {
	event := new(TokenStakingMinimumStakeAmountSet)
	if err := _TokenStaking.contract.UnpackLog(event, "MinimumStakeAmountSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingNotificationRewardPushedIterator is returned from FilterNotificationRewardPushed and is used to iterate over the raw logs and unpacked data for NotificationRewardPushed events raised by the TokenStaking contract.
type TokenStakingNotificationRewardPushedIterator struct {
	Event *TokenStakingNotificationRewardPushed // Event containing the contract specifics and raw log

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
func (it *TokenStakingNotificationRewardPushedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingNotificationRewardPushed)
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
		it.Event = new(TokenStakingNotificationRewardPushed)
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
func (it *TokenStakingNotificationRewardPushedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingNotificationRewardPushedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingNotificationRewardPushed represents a NotificationRewardPushed event raised by the TokenStaking contract.
type TokenStakingNotificationRewardPushed struct {
	Reward *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterNotificationRewardPushed is a free log retrieval operation binding the contract event 0x9de5348508c6ad1f6ff7fcfb84e126bb094784e85bf83f7e3801bc44f9c6dc97.
//
// Solidity: event NotificationRewardPushed(uint96 reward)
func (_TokenStaking *TokenStakingFilterer) FilterNotificationRewardPushed(opts *bind.FilterOpts) (*TokenStakingNotificationRewardPushedIterator, error) {

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "NotificationRewardPushed")
	if err != nil {
		return nil, err
	}
	return &TokenStakingNotificationRewardPushedIterator{contract: _TokenStaking.contract, event: "NotificationRewardPushed", logs: logs, sub: sub}, nil
}

// WatchNotificationRewardPushed is a free log subscription operation binding the contract event 0x9de5348508c6ad1f6ff7fcfb84e126bb094784e85bf83f7e3801bc44f9c6dc97.
//
// Solidity: event NotificationRewardPushed(uint96 reward)
func (_TokenStaking *TokenStakingFilterer) WatchNotificationRewardPushed(opts *bind.WatchOpts, sink chan<- *TokenStakingNotificationRewardPushed) (event.Subscription, error) {

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "NotificationRewardPushed")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingNotificationRewardPushed)
				if err := _TokenStaking.contract.UnpackLog(event, "NotificationRewardPushed", log); err != nil {
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

// ParseNotificationRewardPushed is a log parse operation binding the contract event 0x9de5348508c6ad1f6ff7fcfb84e126bb094784e85bf83f7e3801bc44f9c6dc97.
//
// Solidity: event NotificationRewardPushed(uint96 reward)
func (_TokenStaking *TokenStakingFilterer) ParseNotificationRewardPushed(log types.Log) (*TokenStakingNotificationRewardPushed, error) {
	event := new(TokenStakingNotificationRewardPushed)
	if err := _TokenStaking.contract.UnpackLog(event, "NotificationRewardPushed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingNotificationRewardSetIterator is returned from FilterNotificationRewardSet and is used to iterate over the raw logs and unpacked data for NotificationRewardSet events raised by the TokenStaking contract.
type TokenStakingNotificationRewardSetIterator struct {
	Event *TokenStakingNotificationRewardSet // Event containing the contract specifics and raw log

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
func (it *TokenStakingNotificationRewardSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingNotificationRewardSet)
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
		it.Event = new(TokenStakingNotificationRewardSet)
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
func (it *TokenStakingNotificationRewardSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingNotificationRewardSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingNotificationRewardSet represents a NotificationRewardSet event raised by the TokenStaking contract.
type TokenStakingNotificationRewardSet struct {
	Reward *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterNotificationRewardSet is a free log retrieval operation binding the contract event 0xd579c7b509b9a61b7408309a980bcfcbbf0f336f1b2bb0a760d71f72f0cf3132.
//
// Solidity: event NotificationRewardSet(uint96 reward)
func (_TokenStaking *TokenStakingFilterer) FilterNotificationRewardSet(opts *bind.FilterOpts) (*TokenStakingNotificationRewardSetIterator, error) {

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "NotificationRewardSet")
	if err != nil {
		return nil, err
	}
	return &TokenStakingNotificationRewardSetIterator{contract: _TokenStaking.contract, event: "NotificationRewardSet", logs: logs, sub: sub}, nil
}

// WatchNotificationRewardSet is a free log subscription operation binding the contract event 0xd579c7b509b9a61b7408309a980bcfcbbf0f336f1b2bb0a760d71f72f0cf3132.
//
// Solidity: event NotificationRewardSet(uint96 reward)
func (_TokenStaking *TokenStakingFilterer) WatchNotificationRewardSet(opts *bind.WatchOpts, sink chan<- *TokenStakingNotificationRewardSet) (event.Subscription, error) {

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "NotificationRewardSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingNotificationRewardSet)
				if err := _TokenStaking.contract.UnpackLog(event, "NotificationRewardSet", log); err != nil {
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

// ParseNotificationRewardSet is a log parse operation binding the contract event 0xd579c7b509b9a61b7408309a980bcfcbbf0f336f1b2bb0a760d71f72f0cf3132.
//
// Solidity: event NotificationRewardSet(uint96 reward)
func (_TokenStaking *TokenStakingFilterer) ParseNotificationRewardSet(log types.Log) (*TokenStakingNotificationRewardSet, error) {
	event := new(TokenStakingNotificationRewardSet)
	if err := _TokenStaking.contract.UnpackLog(event, "NotificationRewardSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingNotificationRewardWithdrawnIterator is returned from FilterNotificationRewardWithdrawn and is used to iterate over the raw logs and unpacked data for NotificationRewardWithdrawn events raised by the TokenStaking contract.
type TokenStakingNotificationRewardWithdrawnIterator struct {
	Event *TokenStakingNotificationRewardWithdrawn // Event containing the contract specifics and raw log

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
func (it *TokenStakingNotificationRewardWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingNotificationRewardWithdrawn)
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
		it.Event = new(TokenStakingNotificationRewardWithdrawn)
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
func (it *TokenStakingNotificationRewardWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingNotificationRewardWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingNotificationRewardWithdrawn represents a NotificationRewardWithdrawn event raised by the TokenStaking contract.
type TokenStakingNotificationRewardWithdrawn struct {
	Recipient common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNotificationRewardWithdrawn is a free log retrieval operation binding the contract event 0x7083cb4f4c81bb6d7425a5bde6b6969cd8c446730ed572607ef79246bc44ee42.
//
// Solidity: event NotificationRewardWithdrawn(address recipient, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) FilterNotificationRewardWithdrawn(opts *bind.FilterOpts) (*TokenStakingNotificationRewardWithdrawnIterator, error) {

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "NotificationRewardWithdrawn")
	if err != nil {
		return nil, err
	}
	return &TokenStakingNotificationRewardWithdrawnIterator{contract: _TokenStaking.contract, event: "NotificationRewardWithdrawn", logs: logs, sub: sub}, nil
}

// WatchNotificationRewardWithdrawn is a free log subscription operation binding the contract event 0x7083cb4f4c81bb6d7425a5bde6b6969cd8c446730ed572607ef79246bc44ee42.
//
// Solidity: event NotificationRewardWithdrawn(address recipient, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) WatchNotificationRewardWithdrawn(opts *bind.WatchOpts, sink chan<- *TokenStakingNotificationRewardWithdrawn) (event.Subscription, error) {

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "NotificationRewardWithdrawn")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingNotificationRewardWithdrawn)
				if err := _TokenStaking.contract.UnpackLog(event, "NotificationRewardWithdrawn", log); err != nil {
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

// ParseNotificationRewardWithdrawn is a log parse operation binding the contract event 0x7083cb4f4c81bb6d7425a5bde6b6969cd8c446730ed572607ef79246bc44ee42.
//
// Solidity: event NotificationRewardWithdrawn(address recipient, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) ParseNotificationRewardWithdrawn(log types.Log) (*TokenStakingNotificationRewardWithdrawn, error) {
	event := new(TokenStakingNotificationRewardWithdrawn)
	if err := _TokenStaking.contract.UnpackLog(event, "NotificationRewardWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingNotifierRewardedIterator is returned from FilterNotifierRewarded and is used to iterate over the raw logs and unpacked data for NotifierRewarded events raised by the TokenStaking contract.
type TokenStakingNotifierRewardedIterator struct {
	Event *TokenStakingNotifierRewarded // Event containing the contract specifics and raw log

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
func (it *TokenStakingNotifierRewardedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingNotifierRewarded)
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
		it.Event = new(TokenStakingNotifierRewarded)
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
func (it *TokenStakingNotifierRewardedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingNotifierRewardedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingNotifierRewarded represents a NotifierRewarded event raised by the TokenStaking contract.
type TokenStakingNotifierRewarded struct {
	Notifier common.Address
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterNotifierRewarded is a free log retrieval operation binding the contract event 0x104879b09c38b3b66d79ce8f4cbdcfbe117b79797db65a37ef151d22b5e31471.
//
// Solidity: event NotifierRewarded(address indexed notifier, uint256 amount)
func (_TokenStaking *TokenStakingFilterer) FilterNotifierRewarded(opts *bind.FilterOpts, notifier []common.Address) (*TokenStakingNotifierRewardedIterator, error) {

	var notifierRule []interface{}
	for _, notifierItem := range notifier {
		notifierRule = append(notifierRule, notifierItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "NotifierRewarded", notifierRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingNotifierRewardedIterator{contract: _TokenStaking.contract, event: "NotifierRewarded", logs: logs, sub: sub}, nil
}

// WatchNotifierRewarded is a free log subscription operation binding the contract event 0x104879b09c38b3b66d79ce8f4cbdcfbe117b79797db65a37ef151d22b5e31471.
//
// Solidity: event NotifierRewarded(address indexed notifier, uint256 amount)
func (_TokenStaking *TokenStakingFilterer) WatchNotifierRewarded(opts *bind.WatchOpts, sink chan<- *TokenStakingNotifierRewarded, notifier []common.Address) (event.Subscription, error) {

	var notifierRule []interface{}
	for _, notifierItem := range notifier {
		notifierRule = append(notifierRule, notifierItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "NotifierRewarded", notifierRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingNotifierRewarded)
				if err := _TokenStaking.contract.UnpackLog(event, "NotifierRewarded", log); err != nil {
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

// ParseNotifierRewarded is a log parse operation binding the contract event 0x104879b09c38b3b66d79ce8f4cbdcfbe117b79797db65a37ef151d22b5e31471.
//
// Solidity: event NotifierRewarded(address indexed notifier, uint256 amount)
func (_TokenStaking *TokenStakingFilterer) ParseNotifierRewarded(log types.Log) (*TokenStakingNotifierRewarded, error) {
	event := new(TokenStakingNotifierRewarded)
	if err := _TokenStaking.contract.UnpackLog(event, "NotifierRewarded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingOwnerRefreshedIterator is returned from FilterOwnerRefreshed and is used to iterate over the raw logs and unpacked data for OwnerRefreshed events raised by the TokenStaking contract.
type TokenStakingOwnerRefreshedIterator struct {
	Event *TokenStakingOwnerRefreshed // Event containing the contract specifics and raw log

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
func (it *TokenStakingOwnerRefreshedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingOwnerRefreshed)
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
		it.Event = new(TokenStakingOwnerRefreshed)
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
func (it *TokenStakingOwnerRefreshedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingOwnerRefreshedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingOwnerRefreshed represents a OwnerRefreshed event raised by the TokenStaking contract.
type TokenStakingOwnerRefreshed struct {
	StakingProvider common.Address
	OldOwner        common.Address
	NewOwner        common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOwnerRefreshed is a free log retrieval operation binding the contract event 0xda8908ea0ecabda6b6dec96ed40b9c47ffbf9be10f30912965c984face902e14.
//
// Solidity: event OwnerRefreshed(address indexed stakingProvider, address indexed oldOwner, address indexed newOwner)
func (_TokenStaking *TokenStakingFilterer) FilterOwnerRefreshed(opts *bind.FilterOpts, stakingProvider []common.Address, oldOwner []common.Address, newOwner []common.Address) (*TokenStakingOwnerRefreshedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var oldOwnerRule []interface{}
	for _, oldOwnerItem := range oldOwner {
		oldOwnerRule = append(oldOwnerRule, oldOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "OwnerRefreshed", stakingProviderRule, oldOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingOwnerRefreshedIterator{contract: _TokenStaking.contract, event: "OwnerRefreshed", logs: logs, sub: sub}, nil
}

// WatchOwnerRefreshed is a free log subscription operation binding the contract event 0xda8908ea0ecabda6b6dec96ed40b9c47ffbf9be10f30912965c984face902e14.
//
// Solidity: event OwnerRefreshed(address indexed stakingProvider, address indexed oldOwner, address indexed newOwner)
func (_TokenStaking *TokenStakingFilterer) WatchOwnerRefreshed(opts *bind.WatchOpts, sink chan<- *TokenStakingOwnerRefreshed, stakingProvider []common.Address, oldOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}
	var oldOwnerRule []interface{}
	for _, oldOwnerItem := range oldOwner {
		oldOwnerRule = append(oldOwnerRule, oldOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "OwnerRefreshed", stakingProviderRule, oldOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingOwnerRefreshed)
				if err := _TokenStaking.contract.UnpackLog(event, "OwnerRefreshed", log); err != nil {
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

// ParseOwnerRefreshed is a log parse operation binding the contract event 0xda8908ea0ecabda6b6dec96ed40b9c47ffbf9be10f30912965c984face902e14.
//
// Solidity: event OwnerRefreshed(address indexed stakingProvider, address indexed oldOwner, address indexed newOwner)
func (_TokenStaking *TokenStakingFilterer) ParseOwnerRefreshed(log types.Log) (*TokenStakingOwnerRefreshed, error) {
	event := new(TokenStakingOwnerRefreshed)
	if err := _TokenStaking.contract.UnpackLog(event, "OwnerRefreshed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingPanicButtonSetIterator is returned from FilterPanicButtonSet and is used to iterate over the raw logs and unpacked data for PanicButtonSet events raised by the TokenStaking contract.
type TokenStakingPanicButtonSetIterator struct {
	Event *TokenStakingPanicButtonSet // Event containing the contract specifics and raw log

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
func (it *TokenStakingPanicButtonSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingPanicButtonSet)
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
		it.Event = new(TokenStakingPanicButtonSet)
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
func (it *TokenStakingPanicButtonSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingPanicButtonSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingPanicButtonSet represents a PanicButtonSet event raised by the TokenStaking contract.
type TokenStakingPanicButtonSet struct {
	Application common.Address
	PanicButton common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterPanicButtonSet is a free log retrieval operation binding the contract event 0x5a38ebc6ef9570e77e12b2162c48413d53357005add9be886ab130a58d44feb8.
//
// Solidity: event PanicButtonSet(address indexed application, address indexed panicButton)
func (_TokenStaking *TokenStakingFilterer) FilterPanicButtonSet(opts *bind.FilterOpts, application []common.Address, panicButton []common.Address) (*TokenStakingPanicButtonSetIterator, error) {

	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}
	var panicButtonRule []interface{}
	for _, panicButtonItem := range panicButton {
		panicButtonRule = append(panicButtonRule, panicButtonItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "PanicButtonSet", applicationRule, panicButtonRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingPanicButtonSetIterator{contract: _TokenStaking.contract, event: "PanicButtonSet", logs: logs, sub: sub}, nil
}

// WatchPanicButtonSet is a free log subscription operation binding the contract event 0x5a38ebc6ef9570e77e12b2162c48413d53357005add9be886ab130a58d44feb8.
//
// Solidity: event PanicButtonSet(address indexed application, address indexed panicButton)
func (_TokenStaking *TokenStakingFilterer) WatchPanicButtonSet(opts *bind.WatchOpts, sink chan<- *TokenStakingPanicButtonSet, application []common.Address, panicButton []common.Address) (event.Subscription, error) {

	var applicationRule []interface{}
	for _, applicationItem := range application {
		applicationRule = append(applicationRule, applicationItem)
	}
	var panicButtonRule []interface{}
	for _, panicButtonItem := range panicButton {
		panicButtonRule = append(panicButtonRule, panicButtonItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "PanicButtonSet", applicationRule, panicButtonRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingPanicButtonSet)
				if err := _TokenStaking.contract.UnpackLog(event, "PanicButtonSet", log); err != nil {
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

// ParsePanicButtonSet is a log parse operation binding the contract event 0x5a38ebc6ef9570e77e12b2162c48413d53357005add9be886ab130a58d44feb8.
//
// Solidity: event PanicButtonSet(address indexed application, address indexed panicButton)
func (_TokenStaking *TokenStakingFilterer) ParsePanicButtonSet(log types.Log) (*TokenStakingPanicButtonSet, error) {
	event := new(TokenStakingPanicButtonSet)
	if err := _TokenStaking.contract.UnpackLog(event, "PanicButtonSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingSlashingProcessedIterator is returned from FilterSlashingProcessed and is used to iterate over the raw logs and unpacked data for SlashingProcessed events raised by the TokenStaking contract.
type TokenStakingSlashingProcessedIterator struct {
	Event *TokenStakingSlashingProcessed // Event containing the contract specifics and raw log

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
func (it *TokenStakingSlashingProcessedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingSlashingProcessed)
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
		it.Event = new(TokenStakingSlashingProcessed)
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
func (it *TokenStakingSlashingProcessedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingSlashingProcessedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingSlashingProcessed represents a SlashingProcessed event raised by the TokenStaking contract.
type TokenStakingSlashingProcessed struct {
	Caller  common.Address
	Count   *big.Int
	TAmount *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterSlashingProcessed is a free log retrieval operation binding the contract event 0x8efe68d059265c97157a10a6aadc9afe80dfa0e96fc959c4de863300e244e156.
//
// Solidity: event SlashingProcessed(address indexed caller, uint256 count, uint256 tAmount)
func (_TokenStaking *TokenStakingFilterer) FilterSlashingProcessed(opts *bind.FilterOpts, caller []common.Address) (*TokenStakingSlashingProcessedIterator, error) {

	var callerRule []interface{}
	for _, callerItem := range caller {
		callerRule = append(callerRule, callerItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "SlashingProcessed", callerRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingSlashingProcessedIterator{contract: _TokenStaking.contract, event: "SlashingProcessed", logs: logs, sub: sub}, nil
}

// WatchSlashingProcessed is a free log subscription operation binding the contract event 0x8efe68d059265c97157a10a6aadc9afe80dfa0e96fc959c4de863300e244e156.
//
// Solidity: event SlashingProcessed(address indexed caller, uint256 count, uint256 tAmount)
func (_TokenStaking *TokenStakingFilterer) WatchSlashingProcessed(opts *bind.WatchOpts, sink chan<- *TokenStakingSlashingProcessed, caller []common.Address) (event.Subscription, error) {

	var callerRule []interface{}
	for _, callerItem := range caller {
		callerRule = append(callerRule, callerItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "SlashingProcessed", callerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingSlashingProcessed)
				if err := _TokenStaking.contract.UnpackLog(event, "SlashingProcessed", log); err != nil {
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

// ParseSlashingProcessed is a log parse operation binding the contract event 0x8efe68d059265c97157a10a6aadc9afe80dfa0e96fc959c4de863300e244e156.
//
// Solidity: event SlashingProcessed(address indexed caller, uint256 count, uint256 tAmount)
func (_TokenStaking *TokenStakingFilterer) ParseSlashingProcessed(log types.Log) (*TokenStakingSlashingProcessed, error) {
	event := new(TokenStakingSlashingProcessed)
	if err := _TokenStaking.contract.UnpackLog(event, "SlashingProcessed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingStakeDiscrepancyPenaltySetIterator is returned from FilterStakeDiscrepancyPenaltySet and is used to iterate over the raw logs and unpacked data for StakeDiscrepancyPenaltySet events raised by the TokenStaking contract.
type TokenStakingStakeDiscrepancyPenaltySetIterator struct {
	Event *TokenStakingStakeDiscrepancyPenaltySet // Event containing the contract specifics and raw log

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
func (it *TokenStakingStakeDiscrepancyPenaltySetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingStakeDiscrepancyPenaltySet)
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
		it.Event = new(TokenStakingStakeDiscrepancyPenaltySet)
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
func (it *TokenStakingStakeDiscrepancyPenaltySetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingStakeDiscrepancyPenaltySetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingStakeDiscrepancyPenaltySet represents a StakeDiscrepancyPenaltySet event raised by the TokenStaking contract.
type TokenStakingStakeDiscrepancyPenaltySet struct {
	Penalty          *big.Int
	RewardMultiplier *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterStakeDiscrepancyPenaltySet is a free log retrieval operation binding the contract event 0x3f84f36f8e044bbbc00d303ae27c8871614781ac684742a3d8db7eb2eb98785f.
//
// Solidity: event StakeDiscrepancyPenaltySet(uint96 penalty, uint256 rewardMultiplier)
func (_TokenStaking *TokenStakingFilterer) FilterStakeDiscrepancyPenaltySet(opts *bind.FilterOpts) (*TokenStakingStakeDiscrepancyPenaltySetIterator, error) {

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "StakeDiscrepancyPenaltySet")
	if err != nil {
		return nil, err
	}
	return &TokenStakingStakeDiscrepancyPenaltySetIterator{contract: _TokenStaking.contract, event: "StakeDiscrepancyPenaltySet", logs: logs, sub: sub}, nil
}

// WatchStakeDiscrepancyPenaltySet is a free log subscription operation binding the contract event 0x3f84f36f8e044bbbc00d303ae27c8871614781ac684742a3d8db7eb2eb98785f.
//
// Solidity: event StakeDiscrepancyPenaltySet(uint96 penalty, uint256 rewardMultiplier)
func (_TokenStaking *TokenStakingFilterer) WatchStakeDiscrepancyPenaltySet(opts *bind.WatchOpts, sink chan<- *TokenStakingStakeDiscrepancyPenaltySet) (event.Subscription, error) {

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "StakeDiscrepancyPenaltySet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingStakeDiscrepancyPenaltySet)
				if err := _TokenStaking.contract.UnpackLog(event, "StakeDiscrepancyPenaltySet", log); err != nil {
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

// ParseStakeDiscrepancyPenaltySet is a log parse operation binding the contract event 0x3f84f36f8e044bbbc00d303ae27c8871614781ac684742a3d8db7eb2eb98785f.
//
// Solidity: event StakeDiscrepancyPenaltySet(uint96 penalty, uint256 rewardMultiplier)
func (_TokenStaking *TokenStakingFilterer) ParseStakeDiscrepancyPenaltySet(log types.Log) (*TokenStakingStakeDiscrepancyPenaltySet, error) {
	event := new(TokenStakingStakeDiscrepancyPenaltySet)
	if err := _TokenStaking.contract.UnpackLog(event, "StakeDiscrepancyPenaltySet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingStakedIterator is returned from FilterStaked and is used to iterate over the raw logs and unpacked data for Staked events raised by the TokenStaking contract.
type TokenStakingStakedIterator struct {
	Event *TokenStakingStaked // Event containing the contract specifics and raw log

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
func (it *TokenStakingStakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingStaked)
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
		it.Event = new(TokenStakingStaked)
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
func (it *TokenStakingStakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingStakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingStaked represents a Staked event raised by the TokenStaking contract.
type TokenStakingStaked struct {
	StakeType       uint8
	Owner           common.Address
	StakingProvider common.Address
	Beneficiary     common.Address
	Authorizer      common.Address
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterStaked is a free log retrieval operation binding the contract event 0xe5beba097f34db5d25b3e8383f0c9ba0b9fe180a3a8d2e761c11207221386dfd.
//
// Solidity: event Staked(uint8 indexed stakeType, address indexed owner, address indexed stakingProvider, address beneficiary, address authorizer, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) FilterStaked(opts *bind.FilterOpts, stakeType []uint8, owner []common.Address, stakingProvider []common.Address) (*TokenStakingStakedIterator, error) {

	var stakeTypeRule []interface{}
	for _, stakeTypeItem := range stakeType {
		stakeTypeRule = append(stakeTypeRule, stakeTypeItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "Staked", stakeTypeRule, ownerRule, stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingStakedIterator{contract: _TokenStaking.contract, event: "Staked", logs: logs, sub: sub}, nil
}

// WatchStaked is a free log subscription operation binding the contract event 0xe5beba097f34db5d25b3e8383f0c9ba0b9fe180a3a8d2e761c11207221386dfd.
//
// Solidity: event Staked(uint8 indexed stakeType, address indexed owner, address indexed stakingProvider, address beneficiary, address authorizer, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) WatchStaked(opts *bind.WatchOpts, sink chan<- *TokenStakingStaked, stakeType []uint8, owner []common.Address, stakingProvider []common.Address) (event.Subscription, error) {

	var stakeTypeRule []interface{}
	for _, stakeTypeItem := range stakeType {
		stakeTypeRule = append(stakeTypeRule, stakeTypeItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "Staked", stakeTypeRule, ownerRule, stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingStaked)
				if err := _TokenStaking.contract.UnpackLog(event, "Staked", log); err != nil {
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

// ParseStaked is a log parse operation binding the contract event 0xe5beba097f34db5d25b3e8383f0c9ba0b9fe180a3a8d2e761c11207221386dfd.
//
// Solidity: event Staked(uint8 indexed stakeType, address indexed owner, address indexed stakingProvider, address beneficiary, address authorizer, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) ParseStaked(log types.Log) (*TokenStakingStaked, error) {
	event := new(TokenStakingStaked)
	if err := _TokenStaking.contract.UnpackLog(event, "Staked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingTokensSeizedIterator is returned from FilterTokensSeized and is used to iterate over the raw logs and unpacked data for TokensSeized events raised by the TokenStaking contract.
type TokenStakingTokensSeizedIterator struct {
	Event *TokenStakingTokensSeized // Event containing the contract specifics and raw log

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
func (it *TokenStakingTokensSeizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingTokensSeized)
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
		it.Event = new(TokenStakingTokensSeized)
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
func (it *TokenStakingTokensSeizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingTokensSeizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingTokensSeized represents a TokensSeized event raised by the TokenStaking contract.
type TokenStakingTokensSeized struct {
	StakingProvider common.Address
	Amount          *big.Int
	Discrepancy     bool
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterTokensSeized is a free log retrieval operation binding the contract event 0xfab4356687062505cc650292203fc214dc8cb4b8bd603e53699e3297186e8dd6.
//
// Solidity: event TokensSeized(address indexed stakingProvider, uint96 amount, bool indexed discrepancy)
func (_TokenStaking *TokenStakingFilterer) FilterTokensSeized(opts *bind.FilterOpts, stakingProvider []common.Address, discrepancy []bool) (*TokenStakingTokensSeizedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	var discrepancyRule []interface{}
	for _, discrepancyItem := range discrepancy {
		discrepancyRule = append(discrepancyRule, discrepancyItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "TokensSeized", stakingProviderRule, discrepancyRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingTokensSeizedIterator{contract: _TokenStaking.contract, event: "TokensSeized", logs: logs, sub: sub}, nil
}

// WatchTokensSeized is a free log subscription operation binding the contract event 0xfab4356687062505cc650292203fc214dc8cb4b8bd603e53699e3297186e8dd6.
//
// Solidity: event TokensSeized(address indexed stakingProvider, uint96 amount, bool indexed discrepancy)
func (_TokenStaking *TokenStakingFilterer) WatchTokensSeized(opts *bind.WatchOpts, sink chan<- *TokenStakingTokensSeized, stakingProvider []common.Address, discrepancy []bool) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	var discrepancyRule []interface{}
	for _, discrepancyItem := range discrepancy {
		discrepancyRule = append(discrepancyRule, discrepancyItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "TokensSeized", stakingProviderRule, discrepancyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingTokensSeized)
				if err := _TokenStaking.contract.UnpackLog(event, "TokensSeized", log); err != nil {
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

// ParseTokensSeized is a log parse operation binding the contract event 0xfab4356687062505cc650292203fc214dc8cb4b8bd603e53699e3297186e8dd6.
//
// Solidity: event TokensSeized(address indexed stakingProvider, uint96 amount, bool indexed discrepancy)
func (_TokenStaking *TokenStakingFilterer) ParseTokensSeized(log types.Log) (*TokenStakingTokensSeized, error) {
	event := new(TokenStakingTokensSeized)
	if err := _TokenStaking.contract.UnpackLog(event, "TokensSeized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingToppedUpIterator is returned from FilterToppedUp and is used to iterate over the raw logs and unpacked data for ToppedUp events raised by the TokenStaking contract.
type TokenStakingToppedUpIterator struct {
	Event *TokenStakingToppedUp // Event containing the contract specifics and raw log

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
func (it *TokenStakingToppedUpIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingToppedUp)
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
		it.Event = new(TokenStakingToppedUp)
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
func (it *TokenStakingToppedUpIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingToppedUpIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingToppedUp represents a ToppedUp event raised by the TokenStaking contract.
type TokenStakingToppedUp struct {
	StakingProvider common.Address
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterToppedUp is a free log retrieval operation binding the contract event 0xb8f8e488e98410126386f575c0e233d2effb198a4e68af68ab1de9c2e542ae82.
//
// Solidity: event ToppedUp(address indexed stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) FilterToppedUp(opts *bind.FilterOpts, stakingProvider []common.Address) (*TokenStakingToppedUpIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "ToppedUp", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingToppedUpIterator{contract: _TokenStaking.contract, event: "ToppedUp", logs: logs, sub: sub}, nil
}

// WatchToppedUp is a free log subscription operation binding the contract event 0xb8f8e488e98410126386f575c0e233d2effb198a4e68af68ab1de9c2e542ae82.
//
// Solidity: event ToppedUp(address indexed stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) WatchToppedUp(opts *bind.WatchOpts, sink chan<- *TokenStakingToppedUp, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "ToppedUp", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingToppedUp)
				if err := _TokenStaking.contract.UnpackLog(event, "ToppedUp", log); err != nil {
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

// ParseToppedUp is a log parse operation binding the contract event 0xb8f8e488e98410126386f575c0e233d2effb198a4e68af68ab1de9c2e542ae82.
//
// Solidity: event ToppedUp(address indexed stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) ParseToppedUp(log types.Log) (*TokenStakingToppedUp, error) {
	event := new(TokenStakingToppedUp)
	if err := _TokenStaking.contract.UnpackLog(event, "ToppedUp", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenStakingUnstakedIterator is returned from FilterUnstaked and is used to iterate over the raw logs and unpacked data for Unstaked events raised by the TokenStaking contract.
type TokenStakingUnstakedIterator struct {
	Event *TokenStakingUnstaked // Event containing the contract specifics and raw log

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
func (it *TokenStakingUnstakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStakingUnstaked)
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
		it.Event = new(TokenStakingUnstaked)
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
func (it *TokenStakingUnstakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStakingUnstakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStakingUnstaked represents a Unstaked event raised by the TokenStaking contract.
type TokenStakingUnstaked struct {
	StakingProvider common.Address
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterUnstaked is a free log retrieval operation binding the contract event 0x1221739f34decc066e1d68b15c5fc76b65e7ebe2f08c9f38b3ea3092f9912353.
//
// Solidity: event Unstaked(address indexed stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) FilterUnstaked(opts *bind.FilterOpts, stakingProvider []common.Address) (*TokenStakingUnstakedIterator, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _TokenStaking.contract.FilterLogs(opts, "Unstaked", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return &TokenStakingUnstakedIterator{contract: _TokenStaking.contract, event: "Unstaked", logs: logs, sub: sub}, nil
}

// WatchUnstaked is a free log subscription operation binding the contract event 0x1221739f34decc066e1d68b15c5fc76b65e7ebe2f08c9f38b3ea3092f9912353.
//
// Solidity: event Unstaked(address indexed stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) WatchUnstaked(opts *bind.WatchOpts, sink chan<- *TokenStakingUnstaked, stakingProvider []common.Address) (event.Subscription, error) {

	var stakingProviderRule []interface{}
	for _, stakingProviderItem := range stakingProvider {
		stakingProviderRule = append(stakingProviderRule, stakingProviderItem)
	}

	logs, sub, err := _TokenStaking.contract.WatchLogs(opts, "Unstaked", stakingProviderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStakingUnstaked)
				if err := _TokenStaking.contract.UnpackLog(event, "Unstaked", log); err != nil {
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

// ParseUnstaked is a log parse operation binding the contract event 0x1221739f34decc066e1d68b15c5fc76b65e7ebe2f08c9f38b3ea3092f9912353.
//
// Solidity: event Unstaked(address indexed stakingProvider, uint96 amount)
func (_TokenStaking *TokenStakingFilterer) ParseUnstaked(log types.Log) (*TokenStakingUnstaked, error) {
	event := new(TokenStakingUnstaked)
	if err := _TokenStaking.contract.UnpackLog(event, "Unstaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

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

// BeaconSortitionPoolMetaData contains all meta data concerning the BeaconSortitionPool contract.
var BeaconSortitionPoolMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractIERC20WithPermit\",\"name\":\"_rewardToken\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_poolWeightDivisor\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"operators\",\"type\":\"address[]\"}],\"name\":\"BetaOperatorsAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"ChaosnetDeactivated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldChaosnetOwner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newChaosnetOwner\",\"type\":\"address\"}],\"name\":\"ChaosnetOwnerRoleTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32[]\",\"name\":\"ids\",\"type\":\"uint32[]\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"until\",\"type\":\"uint256\"}],\"name\":\"IneligibleForRewards\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"}],\"name\":\"RewardEligibilityRestored\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"operators\",\"type\":\"address[]\"}],\"name\":\"addBetaOperators\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"canRestoreRewardEligibility\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"chaosnetOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"deactivateChaosnet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"getAvailableRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"}],\"name\":\"getIDOperator\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32[]\",\"name\":\"ids\",\"type\":\"uint32[]\"}],\"name\":\"getIDOperators\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"getOperatorID\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"getPoolWeight\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ineligibleEarnedRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"authorizedStake\",\"type\":\"uint256\"}],\"name\":\"insertOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isBetaOperator\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"isChaosnetActive\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isEligibleForRewards\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"isLocked\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorInPool\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"authorizedStake\",\"type\":\"uint256\"}],\"name\":\"isOperatorUpToDate\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lock\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"operatorsInPool\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"poolWeightDivisor\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"name\":\"receiveApproval\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"restoreRewardEligibility\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rewardToken\",\"outputs\":[{\"internalType\":\"contractIERC20WithPermit\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"rewardsEligibilityRestorableAt\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"groupSize\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"seed\",\"type\":\"bytes32\"}],\"name\":\"selectGroup\",\"outputs\":[{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32[]\",\"name\":\"operators\",\"type\":\"uint32[]\"},{\"internalType\":\"uint256\",\"name\":\"until\",\"type\":\"uint256\"}],\"name\":\"setRewardIneligibility\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalWeight\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newChaosnetOwner\",\"type\":\"address\"}],\"name\":\"transferChaosnetOwnerRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unlock\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"authorizedStake\",\"type\":\"uint256\"}],\"name\":\"updateOperatorStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"withdrawIneligible\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"beneficiary\",\"type\":\"address\"}],\"name\":\"withdrawRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// BeaconSortitionPoolABI is the input ABI used to generate the binding from.
// Deprecated: Use BeaconSortitionPoolMetaData.ABI instead.
var BeaconSortitionPoolABI = BeaconSortitionPoolMetaData.ABI

// BeaconSortitionPool is an auto generated Go binding around an Ethereum contract.
type BeaconSortitionPool struct {
	BeaconSortitionPoolCaller     // Read-only binding to the contract
	BeaconSortitionPoolTransactor // Write-only binding to the contract
	BeaconSortitionPoolFilterer   // Log filterer for contract events
}

// BeaconSortitionPoolCaller is an auto generated read-only Go binding around an Ethereum contract.
type BeaconSortitionPoolCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BeaconSortitionPoolTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BeaconSortitionPoolTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BeaconSortitionPoolFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BeaconSortitionPoolFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BeaconSortitionPoolSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BeaconSortitionPoolSession struct {
	Contract     *BeaconSortitionPool // Generic contract binding to set the session for
	CallOpts     bind.CallOpts        // Call options to use throughout this session
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// BeaconSortitionPoolCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BeaconSortitionPoolCallerSession struct {
	Contract *BeaconSortitionPoolCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts              // Call options to use throughout this session
}

// BeaconSortitionPoolTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BeaconSortitionPoolTransactorSession struct {
	Contract     *BeaconSortitionPoolTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts              // Transaction auth options to use throughout this session
}

// BeaconSortitionPoolRaw is an auto generated low-level Go binding around an Ethereum contract.
type BeaconSortitionPoolRaw struct {
	Contract *BeaconSortitionPool // Generic contract binding to access the raw methods on
}

// BeaconSortitionPoolCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BeaconSortitionPoolCallerRaw struct {
	Contract *BeaconSortitionPoolCaller // Generic read-only contract binding to access the raw methods on
}

// BeaconSortitionPoolTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BeaconSortitionPoolTransactorRaw struct {
	Contract *BeaconSortitionPoolTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBeaconSortitionPool creates a new instance of BeaconSortitionPool, bound to a specific deployed contract.
func NewBeaconSortitionPool(address common.Address, backend bind.ContractBackend) (*BeaconSortitionPool, error) {
	contract, err := bindBeaconSortitionPool(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPool{BeaconSortitionPoolCaller: BeaconSortitionPoolCaller{contract: contract}, BeaconSortitionPoolTransactor: BeaconSortitionPoolTransactor{contract: contract}, BeaconSortitionPoolFilterer: BeaconSortitionPoolFilterer{contract: contract}}, nil
}

// NewBeaconSortitionPoolCaller creates a new read-only instance of BeaconSortitionPool, bound to a specific deployed contract.
func NewBeaconSortitionPoolCaller(address common.Address, caller bind.ContractCaller) (*BeaconSortitionPoolCaller, error) {
	contract, err := bindBeaconSortitionPool(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolCaller{contract: contract}, nil
}

// NewBeaconSortitionPoolTransactor creates a new write-only instance of BeaconSortitionPool, bound to a specific deployed contract.
func NewBeaconSortitionPoolTransactor(address common.Address, transactor bind.ContractTransactor) (*BeaconSortitionPoolTransactor, error) {
	contract, err := bindBeaconSortitionPool(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolTransactor{contract: contract}, nil
}

// NewBeaconSortitionPoolFilterer creates a new log filterer instance of BeaconSortitionPool, bound to a specific deployed contract.
func NewBeaconSortitionPoolFilterer(address common.Address, filterer bind.ContractFilterer) (*BeaconSortitionPoolFilterer, error) {
	contract, err := bindBeaconSortitionPool(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolFilterer{contract: contract}, nil
}

// bindBeaconSortitionPool binds a generic wrapper to an already deployed contract.
func bindBeaconSortitionPool(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BeaconSortitionPoolMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BeaconSortitionPool *BeaconSortitionPoolRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BeaconSortitionPool.Contract.BeaconSortitionPoolCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BeaconSortitionPool *BeaconSortitionPoolRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.BeaconSortitionPoolTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BeaconSortitionPool *BeaconSortitionPoolRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.BeaconSortitionPoolTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BeaconSortitionPool *BeaconSortitionPoolCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BeaconSortitionPool.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.contract.Transact(opts, method, params...)
}

// CanRestoreRewardEligibility is a free data retrieval call binding the contract method 0xc0a3f9eb.
//
// Solidity: function canRestoreRewardEligibility(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) CanRestoreRewardEligibility(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "canRestoreRewardEligibility", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CanRestoreRewardEligibility is a free data retrieval call binding the contract method 0xc0a3f9eb.
//
// Solidity: function canRestoreRewardEligibility(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) CanRestoreRewardEligibility(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.CanRestoreRewardEligibility(&_BeaconSortitionPool.CallOpts, operator)
}

// CanRestoreRewardEligibility is a free data retrieval call binding the contract method 0xc0a3f9eb.
//
// Solidity: function canRestoreRewardEligibility(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) CanRestoreRewardEligibility(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.CanRestoreRewardEligibility(&_BeaconSortitionPool.CallOpts, operator)
}

// ChaosnetOwner is a free data retrieval call binding the contract method 0x7c2cf6cd.
//
// Solidity: function chaosnetOwner() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) ChaosnetOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "chaosnetOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ChaosnetOwner is a free data retrieval call binding the contract method 0x7c2cf6cd.
//
// Solidity: function chaosnetOwner() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) ChaosnetOwner() (common.Address, error) {
	return _BeaconSortitionPool.Contract.ChaosnetOwner(&_BeaconSortitionPool.CallOpts)
}

// ChaosnetOwner is a free data retrieval call binding the contract method 0x7c2cf6cd.
//
// Solidity: function chaosnetOwner() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) ChaosnetOwner() (common.Address, error) {
	return _BeaconSortitionPool.Contract.ChaosnetOwner(&_BeaconSortitionPool.CallOpts)
}

// GetAvailableRewards is a free data retrieval call binding the contract method 0x873e31fa.
//
// Solidity: function getAvailableRewards(address operator) view returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) GetAvailableRewards(opts *bind.CallOpts, operator common.Address) (*big.Int, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "getAvailableRewards", operator)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetAvailableRewards is a free data retrieval call binding the contract method 0x873e31fa.
//
// Solidity: function getAvailableRewards(address operator) view returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) GetAvailableRewards(operator common.Address) (*big.Int, error) {
	return _BeaconSortitionPool.Contract.GetAvailableRewards(&_BeaconSortitionPool.CallOpts, operator)
}

// GetAvailableRewards is a free data retrieval call binding the contract method 0x873e31fa.
//
// Solidity: function getAvailableRewards(address operator) view returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) GetAvailableRewards(operator common.Address) (*big.Int, error) {
	return _BeaconSortitionPool.Contract.GetAvailableRewards(&_BeaconSortitionPool.CallOpts, operator)
}

// GetIDOperator is a free data retrieval call binding the contract method 0x8871ca5d.
//
// Solidity: function getIDOperator(uint32 id) view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) GetIDOperator(opts *bind.CallOpts, id uint32) (common.Address, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "getIDOperator", id)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetIDOperator is a free data retrieval call binding the contract method 0x8871ca5d.
//
// Solidity: function getIDOperator(uint32 id) view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) GetIDOperator(id uint32) (common.Address, error) {
	return _BeaconSortitionPool.Contract.GetIDOperator(&_BeaconSortitionPool.CallOpts, id)
}

// GetIDOperator is a free data retrieval call binding the contract method 0x8871ca5d.
//
// Solidity: function getIDOperator(uint32 id) view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) GetIDOperator(id uint32) (common.Address, error) {
	return _BeaconSortitionPool.Contract.GetIDOperator(&_BeaconSortitionPool.CallOpts, id)
}

// GetIDOperators is a free data retrieval call binding the contract method 0xf7f9a8fa.
//
// Solidity: function getIDOperators(uint32[] ids) view returns(address[])
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) GetIDOperators(opts *bind.CallOpts, ids []uint32) ([]common.Address, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "getIDOperators", ids)

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetIDOperators is a free data retrieval call binding the contract method 0xf7f9a8fa.
//
// Solidity: function getIDOperators(uint32[] ids) view returns(address[])
func (_BeaconSortitionPool *BeaconSortitionPoolSession) GetIDOperators(ids []uint32) ([]common.Address, error) {
	return _BeaconSortitionPool.Contract.GetIDOperators(&_BeaconSortitionPool.CallOpts, ids)
}

// GetIDOperators is a free data retrieval call binding the contract method 0xf7f9a8fa.
//
// Solidity: function getIDOperators(uint32[] ids) view returns(address[])
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) GetIDOperators(ids []uint32) ([]common.Address, error) {
	return _BeaconSortitionPool.Contract.GetIDOperators(&_BeaconSortitionPool.CallOpts, ids)
}

// GetOperatorID is a free data retrieval call binding the contract method 0x5a48b46b.
//
// Solidity: function getOperatorID(address operator) view returns(uint32)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) GetOperatorID(opts *bind.CallOpts, operator common.Address) (uint32, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "getOperatorID", operator)

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// GetOperatorID is a free data retrieval call binding the contract method 0x5a48b46b.
//
// Solidity: function getOperatorID(address operator) view returns(uint32)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) GetOperatorID(operator common.Address) (uint32, error) {
	return _BeaconSortitionPool.Contract.GetOperatorID(&_BeaconSortitionPool.CallOpts, operator)
}

// GetOperatorID is a free data retrieval call binding the contract method 0x5a48b46b.
//
// Solidity: function getOperatorID(address operator) view returns(uint32)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) GetOperatorID(operator common.Address) (uint32, error) {
	return _BeaconSortitionPool.Contract.GetOperatorID(&_BeaconSortitionPool.CallOpts, operator)
}

// GetPoolWeight is a free data retrieval call binding the contract method 0x5757ed5b.
//
// Solidity: function getPoolWeight(address operator) view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) GetPoolWeight(opts *bind.CallOpts, operator common.Address) (*big.Int, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "getPoolWeight", operator)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetPoolWeight is a free data retrieval call binding the contract method 0x5757ed5b.
//
// Solidity: function getPoolWeight(address operator) view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) GetPoolWeight(operator common.Address) (*big.Int, error) {
	return _BeaconSortitionPool.Contract.GetPoolWeight(&_BeaconSortitionPool.CallOpts, operator)
}

// GetPoolWeight is a free data retrieval call binding the contract method 0x5757ed5b.
//
// Solidity: function getPoolWeight(address operator) view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) GetPoolWeight(operator common.Address) (*big.Int, error) {
	return _BeaconSortitionPool.Contract.GetPoolWeight(&_BeaconSortitionPool.CallOpts, operator)
}

// IneligibleEarnedRewards is a free data retrieval call binding the contract method 0xa7a7d391.
//
// Solidity: function ineligibleEarnedRewards() view returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IneligibleEarnedRewards(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "ineligibleEarnedRewards")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// IneligibleEarnedRewards is a free data retrieval call binding the contract method 0xa7a7d391.
//
// Solidity: function ineligibleEarnedRewards() view returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IneligibleEarnedRewards() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.IneligibleEarnedRewards(&_BeaconSortitionPool.CallOpts)
}

// IneligibleEarnedRewards is a free data retrieval call binding the contract method 0xa7a7d391.
//
// Solidity: function ineligibleEarnedRewards() view returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IneligibleEarnedRewards() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.IneligibleEarnedRewards(&_BeaconSortitionPool.CallOpts)
}

// IsBetaOperator is a free data retrieval call binding the contract method 0x398ece9c.
//
// Solidity: function isBetaOperator(address ) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IsBetaOperator(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "isBetaOperator", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsBetaOperator is a free data retrieval call binding the contract method 0x398ece9c.
//
// Solidity: function isBetaOperator(address ) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IsBetaOperator(arg0 common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsBetaOperator(&_BeaconSortitionPool.CallOpts, arg0)
}

// IsBetaOperator is a free data retrieval call binding the contract method 0x398ece9c.
//
// Solidity: function isBetaOperator(address ) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IsBetaOperator(arg0 common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsBetaOperator(&_BeaconSortitionPool.CallOpts, arg0)
}

// IsChaosnetActive is a free data retrieval call binding the contract method 0xb0f3828e.
//
// Solidity: function isChaosnetActive() view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IsChaosnetActive(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "isChaosnetActive")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsChaosnetActive is a free data retrieval call binding the contract method 0xb0f3828e.
//
// Solidity: function isChaosnetActive() view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IsChaosnetActive() (bool, error) {
	return _BeaconSortitionPool.Contract.IsChaosnetActive(&_BeaconSortitionPool.CallOpts)
}

// IsChaosnetActive is a free data retrieval call binding the contract method 0xb0f3828e.
//
// Solidity: function isChaosnetActive() view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IsChaosnetActive() (bool, error) {
	return _BeaconSortitionPool.Contract.IsChaosnetActive(&_BeaconSortitionPool.CallOpts)
}

// IsEligibleForRewards is a free data retrieval call binding the contract method 0x660186e6.
//
// Solidity: function isEligibleForRewards(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IsEligibleForRewards(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "isEligibleForRewards", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsEligibleForRewards is a free data retrieval call binding the contract method 0x660186e6.
//
// Solidity: function isEligibleForRewards(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IsEligibleForRewards(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsEligibleForRewards(&_BeaconSortitionPool.CallOpts, operator)
}

// IsEligibleForRewards is a free data retrieval call binding the contract method 0x660186e6.
//
// Solidity: function isEligibleForRewards(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IsEligibleForRewards(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsEligibleForRewards(&_BeaconSortitionPool.CallOpts, operator)
}

// IsLocked is a free data retrieval call binding the contract method 0xa4e2d634.
//
// Solidity: function isLocked() view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IsLocked(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "isLocked")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsLocked is a free data retrieval call binding the contract method 0xa4e2d634.
//
// Solidity: function isLocked() view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IsLocked() (bool, error) {
	return _BeaconSortitionPool.Contract.IsLocked(&_BeaconSortitionPool.CallOpts)
}

// IsLocked is a free data retrieval call binding the contract method 0xa4e2d634.
//
// Solidity: function isLocked() view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IsLocked() (bool, error) {
	return _BeaconSortitionPool.Contract.IsLocked(&_BeaconSortitionPool.CallOpts)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IsOperatorInPool(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "isOperatorInPool", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsOperatorInPool(&_BeaconSortitionPool.CallOpts, operator)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsOperatorInPool(&_BeaconSortitionPool.CallOpts, operator)
}

// IsOperatorRegistered is a free data retrieval call binding the contract method 0x6b1906f8.
//
// Solidity: function isOperatorRegistered(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IsOperatorRegistered(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "isOperatorRegistered", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorRegistered is a free data retrieval call binding the contract method 0x6b1906f8.
//
// Solidity: function isOperatorRegistered(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IsOperatorRegistered(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsOperatorRegistered(&_BeaconSortitionPool.CallOpts, operator)
}

// IsOperatorRegistered is a free data retrieval call binding the contract method 0x6b1906f8.
//
// Solidity: function isOperatorRegistered(address operator) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IsOperatorRegistered(operator common.Address) (bool, error) {
	return _BeaconSortitionPool.Contract.IsOperatorRegistered(&_BeaconSortitionPool.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0x4de824f0.
//
// Solidity: function isOperatorUpToDate(address operator, uint256 authorizedStake) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) IsOperatorUpToDate(opts *bind.CallOpts, operator common.Address, authorizedStake *big.Int) (bool, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "isOperatorUpToDate", operator, authorizedStake)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0x4de824f0.
//
// Solidity: function isOperatorUpToDate(address operator, uint256 authorizedStake) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) IsOperatorUpToDate(operator common.Address, authorizedStake *big.Int) (bool, error) {
	return _BeaconSortitionPool.Contract.IsOperatorUpToDate(&_BeaconSortitionPool.CallOpts, operator, authorizedStake)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0x4de824f0.
//
// Solidity: function isOperatorUpToDate(address operator, uint256 authorizedStake) view returns(bool)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) IsOperatorUpToDate(operator common.Address, authorizedStake *big.Int) (bool, error) {
	return _BeaconSortitionPool.Contract.IsOperatorUpToDate(&_BeaconSortitionPool.CallOpts, operator, authorizedStake)
}

// OperatorsInPool is a free data retrieval call binding the contract method 0xe7bfd899.
//
// Solidity: function operatorsInPool() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) OperatorsInPool(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "operatorsInPool")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// OperatorsInPool is a free data retrieval call binding the contract method 0xe7bfd899.
//
// Solidity: function operatorsInPool() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) OperatorsInPool() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.OperatorsInPool(&_BeaconSortitionPool.CallOpts)
}

// OperatorsInPool is a free data retrieval call binding the contract method 0xe7bfd899.
//
// Solidity: function operatorsInPool() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) OperatorsInPool() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.OperatorsInPool(&_BeaconSortitionPool.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) Owner() (common.Address, error) {
	return _BeaconSortitionPool.Contract.Owner(&_BeaconSortitionPool.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) Owner() (common.Address, error) {
	return _BeaconSortitionPool.Contract.Owner(&_BeaconSortitionPool.CallOpts)
}

// PoolWeightDivisor is a free data retrieval call binding the contract method 0x43a3db30.
//
// Solidity: function poolWeightDivisor() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) PoolWeightDivisor(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "poolWeightDivisor")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PoolWeightDivisor is a free data retrieval call binding the contract method 0x43a3db30.
//
// Solidity: function poolWeightDivisor() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) PoolWeightDivisor() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.PoolWeightDivisor(&_BeaconSortitionPool.CallOpts)
}

// PoolWeightDivisor is a free data retrieval call binding the contract method 0x43a3db30.
//
// Solidity: function poolWeightDivisor() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) PoolWeightDivisor() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.PoolWeightDivisor(&_BeaconSortitionPool.CallOpts)
}

// RewardToken is a free data retrieval call binding the contract method 0xf7c618c1.
//
// Solidity: function rewardToken() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) RewardToken(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "rewardToken")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// RewardToken is a free data retrieval call binding the contract method 0xf7c618c1.
//
// Solidity: function rewardToken() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) RewardToken() (common.Address, error) {
	return _BeaconSortitionPool.Contract.RewardToken(&_BeaconSortitionPool.CallOpts)
}

// RewardToken is a free data retrieval call binding the contract method 0xf7c618c1.
//
// Solidity: function rewardToken() view returns(address)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) RewardToken() (common.Address, error) {
	return _BeaconSortitionPool.Contract.RewardToken(&_BeaconSortitionPool.CallOpts)
}

// RewardsEligibilityRestorableAt is a free data retrieval call binding the contract method 0x00983b73.
//
// Solidity: function rewardsEligibilityRestorableAt(address operator) view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) RewardsEligibilityRestorableAt(opts *bind.CallOpts, operator common.Address) (*big.Int, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "rewardsEligibilityRestorableAt", operator)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RewardsEligibilityRestorableAt is a free data retrieval call binding the contract method 0x00983b73.
//
// Solidity: function rewardsEligibilityRestorableAt(address operator) view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) RewardsEligibilityRestorableAt(operator common.Address) (*big.Int, error) {
	return _BeaconSortitionPool.Contract.RewardsEligibilityRestorableAt(&_BeaconSortitionPool.CallOpts, operator)
}

// RewardsEligibilityRestorableAt is a free data retrieval call binding the contract method 0x00983b73.
//
// Solidity: function rewardsEligibilityRestorableAt(address operator) view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) RewardsEligibilityRestorableAt(operator common.Address) (*big.Int, error) {
	return _BeaconSortitionPool.Contract.RewardsEligibilityRestorableAt(&_BeaconSortitionPool.CallOpts, operator)
}

// SelectGroup is a free data retrieval call binding the contract method 0x6c2530b9.
//
// Solidity: function selectGroup(uint256 groupSize, bytes32 seed) view returns(uint32[])
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) SelectGroup(opts *bind.CallOpts, groupSize *big.Int, seed [32]byte) ([]uint32, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "selectGroup", groupSize, seed)

	if err != nil {
		return *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint32)).(*[]uint32)

	return out0, err

}

// SelectGroup is a free data retrieval call binding the contract method 0x6c2530b9.
//
// Solidity: function selectGroup(uint256 groupSize, bytes32 seed) view returns(uint32[])
func (_BeaconSortitionPool *BeaconSortitionPoolSession) SelectGroup(groupSize *big.Int, seed [32]byte) ([]uint32, error) {
	return _BeaconSortitionPool.Contract.SelectGroup(&_BeaconSortitionPool.CallOpts, groupSize, seed)
}

// SelectGroup is a free data retrieval call binding the contract method 0x6c2530b9.
//
// Solidity: function selectGroup(uint256 groupSize, bytes32 seed) view returns(uint32[])
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) SelectGroup(groupSize *big.Int, seed [32]byte) ([]uint32, error) {
	return _BeaconSortitionPool.Contract.SelectGroup(&_BeaconSortitionPool.CallOpts, groupSize, seed)
}

// TotalWeight is a free data retrieval call binding the contract method 0x96c82e57.
//
// Solidity: function totalWeight() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCaller) TotalWeight(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _BeaconSortitionPool.contract.Call(opts, &out, "totalWeight")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalWeight is a free data retrieval call binding the contract method 0x96c82e57.
//
// Solidity: function totalWeight() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) TotalWeight() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.TotalWeight(&_BeaconSortitionPool.CallOpts)
}

// TotalWeight is a free data retrieval call binding the contract method 0x96c82e57.
//
// Solidity: function totalWeight() view returns(uint256)
func (_BeaconSortitionPool *BeaconSortitionPoolCallerSession) TotalWeight() (*big.Int, error) {
	return _BeaconSortitionPool.Contract.TotalWeight(&_BeaconSortitionPool.CallOpts)
}

// AddBetaOperators is a paid mutator transaction binding the contract method 0x3e723fc9.
//
// Solidity: function addBetaOperators(address[] operators) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) AddBetaOperators(opts *bind.TransactOpts, operators []common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "addBetaOperators", operators)
}

// AddBetaOperators is a paid mutator transaction binding the contract method 0x3e723fc9.
//
// Solidity: function addBetaOperators(address[] operators) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) AddBetaOperators(operators []common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.AddBetaOperators(&_BeaconSortitionPool.TransactOpts, operators)
}

// AddBetaOperators is a paid mutator transaction binding the contract method 0x3e723fc9.
//
// Solidity: function addBetaOperators(address[] operators) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) AddBetaOperators(operators []common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.AddBetaOperators(&_BeaconSortitionPool.TransactOpts, operators)
}

// DeactivateChaosnet is a paid mutator transaction binding the contract method 0xf23baf4a.
//
// Solidity: function deactivateChaosnet() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) DeactivateChaosnet(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "deactivateChaosnet")
}

// DeactivateChaosnet is a paid mutator transaction binding the contract method 0xf23baf4a.
//
// Solidity: function deactivateChaosnet() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) DeactivateChaosnet() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.DeactivateChaosnet(&_BeaconSortitionPool.TransactOpts)
}

// DeactivateChaosnet is a paid mutator transaction binding the contract method 0xf23baf4a.
//
// Solidity: function deactivateChaosnet() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) DeactivateChaosnet() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.DeactivateChaosnet(&_BeaconSortitionPool.TransactOpts)
}

// InsertOperator is a paid mutator transaction binding the contract method 0x241a4188.
//
// Solidity: function insertOperator(address operator, uint256 authorizedStake) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) InsertOperator(opts *bind.TransactOpts, operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "insertOperator", operator, authorizedStake)
}

// InsertOperator is a paid mutator transaction binding the contract method 0x241a4188.
//
// Solidity: function insertOperator(address operator, uint256 authorizedStake) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) InsertOperator(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.InsertOperator(&_BeaconSortitionPool.TransactOpts, operator, authorizedStake)
}

// InsertOperator is a paid mutator transaction binding the contract method 0x241a4188.
//
// Solidity: function insertOperator(address operator, uint256 authorizedStake) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) InsertOperator(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.InsertOperator(&_BeaconSortitionPool.TransactOpts, operator, authorizedStake)
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) Lock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "lock")
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) Lock() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.Lock(&_BeaconSortitionPool.TransactOpts)
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) Lock() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.Lock(&_BeaconSortitionPool.TransactOpts)
}

// ReceiveApproval is a paid mutator transaction binding the contract method 0x8f4ffcb1.
//
// Solidity: function receiveApproval(address sender, uint256 amount, address token, bytes ) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) ReceiveApproval(opts *bind.TransactOpts, sender common.Address, amount *big.Int, token common.Address, arg3 []byte) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "receiveApproval", sender, amount, token, arg3)
}

// ReceiveApproval is a paid mutator transaction binding the contract method 0x8f4ffcb1.
//
// Solidity: function receiveApproval(address sender, uint256 amount, address token, bytes ) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) ReceiveApproval(sender common.Address, amount *big.Int, token common.Address, arg3 []byte) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.ReceiveApproval(&_BeaconSortitionPool.TransactOpts, sender, amount, token, arg3)
}

// ReceiveApproval is a paid mutator transaction binding the contract method 0x8f4ffcb1.
//
// Solidity: function receiveApproval(address sender, uint256 amount, address token, bytes ) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) ReceiveApproval(sender common.Address, amount *big.Int, token common.Address, arg3 []byte) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.ReceiveApproval(&_BeaconSortitionPool.TransactOpts, sender, amount, token, arg3)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) RenounceOwnership() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.RenounceOwnership(&_BeaconSortitionPool.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.RenounceOwnership(&_BeaconSortitionPool.TransactOpts)
}

// RestoreRewardEligibility is a paid mutator transaction binding the contract method 0xb2f3db4d.
//
// Solidity: function restoreRewardEligibility(address operator) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) RestoreRewardEligibility(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "restoreRewardEligibility", operator)
}

// RestoreRewardEligibility is a paid mutator transaction binding the contract method 0xb2f3db4d.
//
// Solidity: function restoreRewardEligibility(address operator) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) RestoreRewardEligibility(operator common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.RestoreRewardEligibility(&_BeaconSortitionPool.TransactOpts, operator)
}

// RestoreRewardEligibility is a paid mutator transaction binding the contract method 0xb2f3db4d.
//
// Solidity: function restoreRewardEligibility(address operator) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) RestoreRewardEligibility(operator common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.RestoreRewardEligibility(&_BeaconSortitionPool.TransactOpts, operator)
}

// SetRewardIneligibility is a paid mutator transaction binding the contract method 0x942f6892.
//
// Solidity: function setRewardIneligibility(uint32[] operators, uint256 until) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) SetRewardIneligibility(opts *bind.TransactOpts, operators []uint32, until *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "setRewardIneligibility", operators, until)
}

// SetRewardIneligibility is a paid mutator transaction binding the contract method 0x942f6892.
//
// Solidity: function setRewardIneligibility(uint32[] operators, uint256 until) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) SetRewardIneligibility(operators []uint32, until *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.SetRewardIneligibility(&_BeaconSortitionPool.TransactOpts, operators, until)
}

// SetRewardIneligibility is a paid mutator transaction binding the contract method 0x942f6892.
//
// Solidity: function setRewardIneligibility(uint32[] operators, uint256 until) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) SetRewardIneligibility(operators []uint32, until *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.SetRewardIneligibility(&_BeaconSortitionPool.TransactOpts, operators, until)
}

// TransferChaosnetOwnerRole is a paid mutator transaction binding the contract method 0xc545b3a9.
//
// Solidity: function transferChaosnetOwnerRole(address newChaosnetOwner) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) TransferChaosnetOwnerRole(opts *bind.TransactOpts, newChaosnetOwner common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "transferChaosnetOwnerRole", newChaosnetOwner)
}

// TransferChaosnetOwnerRole is a paid mutator transaction binding the contract method 0xc545b3a9.
//
// Solidity: function transferChaosnetOwnerRole(address newChaosnetOwner) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) TransferChaosnetOwnerRole(newChaosnetOwner common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.TransferChaosnetOwnerRole(&_BeaconSortitionPool.TransactOpts, newChaosnetOwner)
}

// TransferChaosnetOwnerRole is a paid mutator transaction binding the contract method 0xc545b3a9.
//
// Solidity: function transferChaosnetOwnerRole(address newChaosnetOwner) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) TransferChaosnetOwnerRole(newChaosnetOwner common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.TransferChaosnetOwnerRole(&_BeaconSortitionPool.TransactOpts, newChaosnetOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.TransferOwnership(&_BeaconSortitionPool.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.TransferOwnership(&_BeaconSortitionPool.TransactOpts, newOwner)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) Unlock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "unlock")
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) Unlock() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.Unlock(&_BeaconSortitionPool.TransactOpts)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) Unlock() (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.Unlock(&_BeaconSortitionPool.TransactOpts)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0xdc7520c5.
//
// Solidity: function updateOperatorStatus(address operator, uint256 authorizedStake) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) UpdateOperatorStatus(opts *bind.TransactOpts, operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "updateOperatorStatus", operator, authorizedStake)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0xdc7520c5.
//
// Solidity: function updateOperatorStatus(address operator, uint256 authorizedStake) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) UpdateOperatorStatus(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.UpdateOperatorStatus(&_BeaconSortitionPool.TransactOpts, operator, authorizedStake)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0xdc7520c5.
//
// Solidity: function updateOperatorStatus(address operator, uint256 authorizedStake) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) UpdateOperatorStatus(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.UpdateOperatorStatus(&_BeaconSortitionPool.TransactOpts, operator, authorizedStake)
}

// WithdrawIneligible is a paid mutator transaction binding the contract method 0xa9649414.
//
// Solidity: function withdrawIneligible(address recipient) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) WithdrawIneligible(opts *bind.TransactOpts, recipient common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "withdrawIneligible", recipient)
}

// WithdrawIneligible is a paid mutator transaction binding the contract method 0xa9649414.
//
// Solidity: function withdrawIneligible(address recipient) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolSession) WithdrawIneligible(recipient common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.WithdrawIneligible(&_BeaconSortitionPool.TransactOpts, recipient)
}

// WithdrawIneligible is a paid mutator transaction binding the contract method 0xa9649414.
//
// Solidity: function withdrawIneligible(address recipient) returns()
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) WithdrawIneligible(recipient common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.WithdrawIneligible(&_BeaconSortitionPool.TransactOpts, recipient)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0xe20981ca.
//
// Solidity: function withdrawRewards(address operator, address beneficiary) returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolTransactor) WithdrawRewards(opts *bind.TransactOpts, operator common.Address, beneficiary common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.contract.Transact(opts, "withdrawRewards", operator, beneficiary)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0xe20981ca.
//
// Solidity: function withdrawRewards(address operator, address beneficiary) returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolSession) WithdrawRewards(operator common.Address, beneficiary common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.WithdrawRewards(&_BeaconSortitionPool.TransactOpts, operator, beneficiary)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0xe20981ca.
//
// Solidity: function withdrawRewards(address operator, address beneficiary) returns(uint96)
func (_BeaconSortitionPool *BeaconSortitionPoolTransactorSession) WithdrawRewards(operator common.Address, beneficiary common.Address) (*types.Transaction, error) {
	return _BeaconSortitionPool.Contract.WithdrawRewards(&_BeaconSortitionPool.TransactOpts, operator, beneficiary)
}

// BeaconSortitionPoolBetaOperatorsAddedIterator is returned from FilterBetaOperatorsAdded and is used to iterate over the raw logs and unpacked data for BetaOperatorsAdded events raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolBetaOperatorsAddedIterator struct {
	Event *BeaconSortitionPoolBetaOperatorsAdded // Event containing the contract specifics and raw log

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
func (it *BeaconSortitionPoolBetaOperatorsAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BeaconSortitionPoolBetaOperatorsAdded)
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
		it.Event = new(BeaconSortitionPoolBetaOperatorsAdded)
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
func (it *BeaconSortitionPoolBetaOperatorsAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BeaconSortitionPoolBetaOperatorsAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BeaconSortitionPoolBetaOperatorsAdded represents a BetaOperatorsAdded event raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolBetaOperatorsAdded struct {
	Operators []common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterBetaOperatorsAdded is a free log retrieval operation binding the contract event 0x79b60dc9f29a0514f5ce9bf1e89b7add7a22440cde3b203c03a842e3b534071b.
//
// Solidity: event BetaOperatorsAdded(address[] operators)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) FilterBetaOperatorsAdded(opts *bind.FilterOpts) (*BeaconSortitionPoolBetaOperatorsAddedIterator, error) {

	logs, sub, err := _BeaconSortitionPool.contract.FilterLogs(opts, "BetaOperatorsAdded")
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolBetaOperatorsAddedIterator{contract: _BeaconSortitionPool.contract, event: "BetaOperatorsAdded", logs: logs, sub: sub}, nil
}

// WatchBetaOperatorsAdded is a free log subscription operation binding the contract event 0x79b60dc9f29a0514f5ce9bf1e89b7add7a22440cde3b203c03a842e3b534071b.
//
// Solidity: event BetaOperatorsAdded(address[] operators)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) WatchBetaOperatorsAdded(opts *bind.WatchOpts, sink chan<- *BeaconSortitionPoolBetaOperatorsAdded) (event.Subscription, error) {

	logs, sub, err := _BeaconSortitionPool.contract.WatchLogs(opts, "BetaOperatorsAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BeaconSortitionPoolBetaOperatorsAdded)
				if err := _BeaconSortitionPool.contract.UnpackLog(event, "BetaOperatorsAdded", log); err != nil {
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

// ParseBetaOperatorsAdded is a log parse operation binding the contract event 0x79b60dc9f29a0514f5ce9bf1e89b7add7a22440cde3b203c03a842e3b534071b.
//
// Solidity: event BetaOperatorsAdded(address[] operators)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) ParseBetaOperatorsAdded(log types.Log) (*BeaconSortitionPoolBetaOperatorsAdded, error) {
	event := new(BeaconSortitionPoolBetaOperatorsAdded)
	if err := _BeaconSortitionPool.contract.UnpackLog(event, "BetaOperatorsAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BeaconSortitionPoolChaosnetDeactivatedIterator is returned from FilterChaosnetDeactivated and is used to iterate over the raw logs and unpacked data for ChaosnetDeactivated events raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolChaosnetDeactivatedIterator struct {
	Event *BeaconSortitionPoolChaosnetDeactivated // Event containing the contract specifics and raw log

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
func (it *BeaconSortitionPoolChaosnetDeactivatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BeaconSortitionPoolChaosnetDeactivated)
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
		it.Event = new(BeaconSortitionPoolChaosnetDeactivated)
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
func (it *BeaconSortitionPoolChaosnetDeactivatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BeaconSortitionPoolChaosnetDeactivatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BeaconSortitionPoolChaosnetDeactivated represents a ChaosnetDeactivated event raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolChaosnetDeactivated struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterChaosnetDeactivated is a free log retrieval operation binding the contract event 0xbea11dc6cfde2788be7e8a6ceef5c8d181bb1c628ba6d71675fca0e754367c74.
//
// Solidity: event ChaosnetDeactivated()
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) FilterChaosnetDeactivated(opts *bind.FilterOpts) (*BeaconSortitionPoolChaosnetDeactivatedIterator, error) {

	logs, sub, err := _BeaconSortitionPool.contract.FilterLogs(opts, "ChaosnetDeactivated")
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolChaosnetDeactivatedIterator{contract: _BeaconSortitionPool.contract, event: "ChaosnetDeactivated", logs: logs, sub: sub}, nil
}

// WatchChaosnetDeactivated is a free log subscription operation binding the contract event 0xbea11dc6cfde2788be7e8a6ceef5c8d181bb1c628ba6d71675fca0e754367c74.
//
// Solidity: event ChaosnetDeactivated()
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) WatchChaosnetDeactivated(opts *bind.WatchOpts, sink chan<- *BeaconSortitionPoolChaosnetDeactivated) (event.Subscription, error) {

	logs, sub, err := _BeaconSortitionPool.contract.WatchLogs(opts, "ChaosnetDeactivated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BeaconSortitionPoolChaosnetDeactivated)
				if err := _BeaconSortitionPool.contract.UnpackLog(event, "ChaosnetDeactivated", log); err != nil {
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

// ParseChaosnetDeactivated is a log parse operation binding the contract event 0xbea11dc6cfde2788be7e8a6ceef5c8d181bb1c628ba6d71675fca0e754367c74.
//
// Solidity: event ChaosnetDeactivated()
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) ParseChaosnetDeactivated(log types.Log) (*BeaconSortitionPoolChaosnetDeactivated, error) {
	event := new(BeaconSortitionPoolChaosnetDeactivated)
	if err := _BeaconSortitionPool.contract.UnpackLog(event, "ChaosnetDeactivated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BeaconSortitionPoolChaosnetOwnerRoleTransferredIterator is returned from FilterChaosnetOwnerRoleTransferred and is used to iterate over the raw logs and unpacked data for ChaosnetOwnerRoleTransferred events raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolChaosnetOwnerRoleTransferredIterator struct {
	Event *BeaconSortitionPoolChaosnetOwnerRoleTransferred // Event containing the contract specifics and raw log

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
func (it *BeaconSortitionPoolChaosnetOwnerRoleTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BeaconSortitionPoolChaosnetOwnerRoleTransferred)
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
		it.Event = new(BeaconSortitionPoolChaosnetOwnerRoleTransferred)
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
func (it *BeaconSortitionPoolChaosnetOwnerRoleTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BeaconSortitionPoolChaosnetOwnerRoleTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BeaconSortitionPoolChaosnetOwnerRoleTransferred represents a ChaosnetOwnerRoleTransferred event raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolChaosnetOwnerRoleTransferred struct {
	OldChaosnetOwner common.Address
	NewChaosnetOwner common.Address
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterChaosnetOwnerRoleTransferred is a free log retrieval operation binding the contract event 0xf7d2871c195d5dcbeca7c9bfb4f7ae4149d0915a5d3d03c8c2286c9a24e932be.
//
// Solidity: event ChaosnetOwnerRoleTransferred(address oldChaosnetOwner, address newChaosnetOwner)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) FilterChaosnetOwnerRoleTransferred(opts *bind.FilterOpts) (*BeaconSortitionPoolChaosnetOwnerRoleTransferredIterator, error) {

	logs, sub, err := _BeaconSortitionPool.contract.FilterLogs(opts, "ChaosnetOwnerRoleTransferred")
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolChaosnetOwnerRoleTransferredIterator{contract: _BeaconSortitionPool.contract, event: "ChaosnetOwnerRoleTransferred", logs: logs, sub: sub}, nil
}

// WatchChaosnetOwnerRoleTransferred is a free log subscription operation binding the contract event 0xf7d2871c195d5dcbeca7c9bfb4f7ae4149d0915a5d3d03c8c2286c9a24e932be.
//
// Solidity: event ChaosnetOwnerRoleTransferred(address oldChaosnetOwner, address newChaosnetOwner)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) WatchChaosnetOwnerRoleTransferred(opts *bind.WatchOpts, sink chan<- *BeaconSortitionPoolChaosnetOwnerRoleTransferred) (event.Subscription, error) {

	logs, sub, err := _BeaconSortitionPool.contract.WatchLogs(opts, "ChaosnetOwnerRoleTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BeaconSortitionPoolChaosnetOwnerRoleTransferred)
				if err := _BeaconSortitionPool.contract.UnpackLog(event, "ChaosnetOwnerRoleTransferred", log); err != nil {
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

// ParseChaosnetOwnerRoleTransferred is a log parse operation binding the contract event 0xf7d2871c195d5dcbeca7c9bfb4f7ae4149d0915a5d3d03c8c2286c9a24e932be.
//
// Solidity: event ChaosnetOwnerRoleTransferred(address oldChaosnetOwner, address newChaosnetOwner)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) ParseChaosnetOwnerRoleTransferred(log types.Log) (*BeaconSortitionPoolChaosnetOwnerRoleTransferred, error) {
	event := new(BeaconSortitionPoolChaosnetOwnerRoleTransferred)
	if err := _BeaconSortitionPool.contract.UnpackLog(event, "ChaosnetOwnerRoleTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BeaconSortitionPoolIneligibleForRewardsIterator is returned from FilterIneligibleForRewards and is used to iterate over the raw logs and unpacked data for IneligibleForRewards events raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolIneligibleForRewardsIterator struct {
	Event *BeaconSortitionPoolIneligibleForRewards // Event containing the contract specifics and raw log

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
func (it *BeaconSortitionPoolIneligibleForRewardsIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BeaconSortitionPoolIneligibleForRewards)
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
		it.Event = new(BeaconSortitionPoolIneligibleForRewards)
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
func (it *BeaconSortitionPoolIneligibleForRewardsIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BeaconSortitionPoolIneligibleForRewardsIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BeaconSortitionPoolIneligibleForRewards represents a IneligibleForRewards event raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolIneligibleForRewards struct {
	Ids   []uint32
	Until *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterIneligibleForRewards is a free log retrieval operation binding the contract event 0x01f5838e3dde8cf4817b958fe95be92bdfeccb34317e1d9f58d1cfe5230de231.
//
// Solidity: event IneligibleForRewards(uint32[] ids, uint256 until)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) FilterIneligibleForRewards(opts *bind.FilterOpts) (*BeaconSortitionPoolIneligibleForRewardsIterator, error) {

	logs, sub, err := _BeaconSortitionPool.contract.FilterLogs(opts, "IneligibleForRewards")
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolIneligibleForRewardsIterator{contract: _BeaconSortitionPool.contract, event: "IneligibleForRewards", logs: logs, sub: sub}, nil
}

// WatchIneligibleForRewards is a free log subscription operation binding the contract event 0x01f5838e3dde8cf4817b958fe95be92bdfeccb34317e1d9f58d1cfe5230de231.
//
// Solidity: event IneligibleForRewards(uint32[] ids, uint256 until)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) WatchIneligibleForRewards(opts *bind.WatchOpts, sink chan<- *BeaconSortitionPoolIneligibleForRewards) (event.Subscription, error) {

	logs, sub, err := _BeaconSortitionPool.contract.WatchLogs(opts, "IneligibleForRewards")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BeaconSortitionPoolIneligibleForRewards)
				if err := _BeaconSortitionPool.contract.UnpackLog(event, "IneligibleForRewards", log); err != nil {
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

// ParseIneligibleForRewards is a log parse operation binding the contract event 0x01f5838e3dde8cf4817b958fe95be92bdfeccb34317e1d9f58d1cfe5230de231.
//
// Solidity: event IneligibleForRewards(uint32[] ids, uint256 until)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) ParseIneligibleForRewards(log types.Log) (*BeaconSortitionPoolIneligibleForRewards, error) {
	event := new(BeaconSortitionPoolIneligibleForRewards)
	if err := _BeaconSortitionPool.contract.UnpackLog(event, "IneligibleForRewards", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BeaconSortitionPoolOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolOwnershipTransferredIterator struct {
	Event *BeaconSortitionPoolOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *BeaconSortitionPoolOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BeaconSortitionPoolOwnershipTransferred)
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
		it.Event = new(BeaconSortitionPoolOwnershipTransferred)
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
func (it *BeaconSortitionPoolOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BeaconSortitionPoolOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BeaconSortitionPoolOwnershipTransferred represents a OwnershipTransferred event raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*BeaconSortitionPoolOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _BeaconSortitionPool.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolOwnershipTransferredIterator{contract: _BeaconSortitionPool.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *BeaconSortitionPoolOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _BeaconSortitionPool.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BeaconSortitionPoolOwnershipTransferred)
				if err := _BeaconSortitionPool.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) ParseOwnershipTransferred(log types.Log) (*BeaconSortitionPoolOwnershipTransferred, error) {
	event := new(BeaconSortitionPoolOwnershipTransferred)
	if err := _BeaconSortitionPool.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BeaconSortitionPoolRewardEligibilityRestoredIterator is returned from FilterRewardEligibilityRestored and is used to iterate over the raw logs and unpacked data for RewardEligibilityRestored events raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolRewardEligibilityRestoredIterator struct {
	Event *BeaconSortitionPoolRewardEligibilityRestored // Event containing the contract specifics and raw log

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
func (it *BeaconSortitionPoolRewardEligibilityRestoredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BeaconSortitionPoolRewardEligibilityRestored)
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
		it.Event = new(BeaconSortitionPoolRewardEligibilityRestored)
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
func (it *BeaconSortitionPoolRewardEligibilityRestoredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BeaconSortitionPoolRewardEligibilityRestoredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BeaconSortitionPoolRewardEligibilityRestored represents a RewardEligibilityRestored event raised by the BeaconSortitionPool contract.
type BeaconSortitionPoolRewardEligibilityRestored struct {
	Operator common.Address
	Id       uint32
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterRewardEligibilityRestored is a free log retrieval operation binding the contract event 0xe61e9f0f049b3bfae1ae903a5e3018c02a008aa0d238ffddf23a4fb4c0278536.
//
// Solidity: event RewardEligibilityRestored(address indexed operator, uint32 indexed id)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) FilterRewardEligibilityRestored(opts *bind.FilterOpts, operator []common.Address, id []uint32) (*BeaconSortitionPoolRewardEligibilityRestoredIterator, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _BeaconSortitionPool.contract.FilterLogs(opts, "RewardEligibilityRestored", operatorRule, idRule)
	if err != nil {
		return nil, err
	}
	return &BeaconSortitionPoolRewardEligibilityRestoredIterator{contract: _BeaconSortitionPool.contract, event: "RewardEligibilityRestored", logs: logs, sub: sub}, nil
}

// WatchRewardEligibilityRestored is a free log subscription operation binding the contract event 0xe61e9f0f049b3bfae1ae903a5e3018c02a008aa0d238ffddf23a4fb4c0278536.
//
// Solidity: event RewardEligibilityRestored(address indexed operator, uint32 indexed id)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) WatchRewardEligibilityRestored(opts *bind.WatchOpts, sink chan<- *BeaconSortitionPoolRewardEligibilityRestored, operator []common.Address, id []uint32) (event.Subscription, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _BeaconSortitionPool.contract.WatchLogs(opts, "RewardEligibilityRestored", operatorRule, idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BeaconSortitionPoolRewardEligibilityRestored)
				if err := _BeaconSortitionPool.contract.UnpackLog(event, "RewardEligibilityRestored", log); err != nil {
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

// ParseRewardEligibilityRestored is a log parse operation binding the contract event 0xe61e9f0f049b3bfae1ae903a5e3018c02a008aa0d238ffddf23a4fb4c0278536.
//
// Solidity: event RewardEligibilityRestored(address indexed operator, uint32 indexed id)
func (_BeaconSortitionPool *BeaconSortitionPoolFilterer) ParseRewardEligibilityRestored(log types.Log) (*BeaconSortitionPoolRewardEligibilityRestored, error) {
	event := new(BeaconSortitionPoolRewardEligibilityRestored)
	if err := _BeaconSortitionPool.contract.UnpackLog(event, "RewardEligibilityRestored", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

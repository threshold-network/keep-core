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

// EcdsaSortitionPoolMetaData contains all meta data concerning the EcdsaSortitionPool contract.
var EcdsaSortitionPoolMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractIERC20WithPermit\",\"name\":\"_rewardToken\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_poolWeightDivisor\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"operators\",\"type\":\"address[]\"}],\"name\":\"BetaOperatorsAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"ChaosnetDeactivated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldChaosnetOwner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newChaosnetOwner\",\"type\":\"address\"}],\"name\":\"ChaosnetOwnerRoleTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32[]\",\"name\":\"ids\",\"type\":\"uint32[]\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"until\",\"type\":\"uint256\"}],\"name\":\"IneligibleForRewards\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"}],\"name\":\"RewardEligibilityRestored\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"operators\",\"type\":\"address[]\"}],\"name\":\"addBetaOperators\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"canRestoreRewardEligibility\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"chaosnetOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"deactivateChaosnet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"getAvailableRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"}],\"name\":\"getIDOperator\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32[]\",\"name\":\"ids\",\"type\":\"uint32[]\"}],\"name\":\"getIDOperators\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"getOperatorID\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"getPoolWeight\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ineligibleEarnedRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"authorizedStake\",\"type\":\"uint256\"}],\"name\":\"insertOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isBetaOperator\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"isChaosnetActive\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isEligibleForRewards\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"isLocked\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorInPool\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isOperatorRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"authorizedStake\",\"type\":\"uint256\"}],\"name\":\"isOperatorUpToDate\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lock\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"operatorsInPool\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"poolWeightDivisor\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"name\":\"receiveApproval\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"restoreRewardEligibility\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"rewardToken\",\"outputs\":[{\"internalType\":\"contractIERC20WithPermit\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"rewardsEligibilityRestorableAt\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"groupSize\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"seed\",\"type\":\"bytes32\"}],\"name\":\"selectGroup\",\"outputs\":[{\"internalType\":\"uint32[]\",\"name\":\"\",\"type\":\"uint32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32[]\",\"name\":\"operators\",\"type\":\"uint32[]\"},{\"internalType\":\"uint256\",\"name\":\"until\",\"type\":\"uint256\"}],\"name\":\"setRewardIneligibility\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalWeight\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newChaosnetOwner\",\"type\":\"address\"}],\"name\":\"transferChaosnetOwnerRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unlock\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"authorizedStake\",\"type\":\"uint256\"}],\"name\":\"updateOperatorStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"}],\"name\":\"withdrawIneligible\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"beneficiary\",\"type\":\"address\"}],\"name\":\"withdrawRewards\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"\",\"type\":\"uint96\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// EcdsaSortitionPoolABI is the input ABI used to generate the binding from.
// Deprecated: Use EcdsaSortitionPoolMetaData.ABI instead.
var EcdsaSortitionPoolABI = EcdsaSortitionPoolMetaData.ABI

// EcdsaSortitionPool is an auto generated Go binding around an Ethereum contract.
type EcdsaSortitionPool struct {
	EcdsaSortitionPoolCaller     // Read-only binding to the contract
	EcdsaSortitionPoolTransactor // Write-only binding to the contract
	EcdsaSortitionPoolFilterer   // Log filterer for contract events
}

// EcdsaSortitionPoolCaller is an auto generated read-only Go binding around an Ethereum contract.
type EcdsaSortitionPoolCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EcdsaSortitionPoolTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EcdsaSortitionPoolTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EcdsaSortitionPoolFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EcdsaSortitionPoolFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EcdsaSortitionPoolSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EcdsaSortitionPoolSession struct {
	Contract     *EcdsaSortitionPool // Generic contract binding to set the session for
	CallOpts     bind.CallOpts       // Call options to use throughout this session
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// EcdsaSortitionPoolCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EcdsaSortitionPoolCallerSession struct {
	Contract *EcdsaSortitionPoolCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts             // Call options to use throughout this session
}

// EcdsaSortitionPoolTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EcdsaSortitionPoolTransactorSession struct {
	Contract     *EcdsaSortitionPoolTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts             // Transaction auth options to use throughout this session
}

// EcdsaSortitionPoolRaw is an auto generated low-level Go binding around an Ethereum contract.
type EcdsaSortitionPoolRaw struct {
	Contract *EcdsaSortitionPool // Generic contract binding to access the raw methods on
}

// EcdsaSortitionPoolCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EcdsaSortitionPoolCallerRaw struct {
	Contract *EcdsaSortitionPoolCaller // Generic read-only contract binding to access the raw methods on
}

// EcdsaSortitionPoolTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EcdsaSortitionPoolTransactorRaw struct {
	Contract *EcdsaSortitionPoolTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEcdsaSortitionPool creates a new instance of EcdsaSortitionPool, bound to a specific deployed contract.
func NewEcdsaSortitionPool(address common.Address, backend bind.ContractBackend) (*EcdsaSortitionPool, error) {
	contract, err := bindEcdsaSortitionPool(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPool{EcdsaSortitionPoolCaller: EcdsaSortitionPoolCaller{contract: contract}, EcdsaSortitionPoolTransactor: EcdsaSortitionPoolTransactor{contract: contract}, EcdsaSortitionPoolFilterer: EcdsaSortitionPoolFilterer{contract: contract}}, nil
}

// NewEcdsaSortitionPoolCaller creates a new read-only instance of EcdsaSortitionPool, bound to a specific deployed contract.
func NewEcdsaSortitionPoolCaller(address common.Address, caller bind.ContractCaller) (*EcdsaSortitionPoolCaller, error) {
	contract, err := bindEcdsaSortitionPool(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolCaller{contract: contract}, nil
}

// NewEcdsaSortitionPoolTransactor creates a new write-only instance of EcdsaSortitionPool, bound to a specific deployed contract.
func NewEcdsaSortitionPoolTransactor(address common.Address, transactor bind.ContractTransactor) (*EcdsaSortitionPoolTransactor, error) {
	contract, err := bindEcdsaSortitionPool(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolTransactor{contract: contract}, nil
}

// NewEcdsaSortitionPoolFilterer creates a new log filterer instance of EcdsaSortitionPool, bound to a specific deployed contract.
func NewEcdsaSortitionPoolFilterer(address common.Address, filterer bind.ContractFilterer) (*EcdsaSortitionPoolFilterer, error) {
	contract, err := bindEcdsaSortitionPool(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolFilterer{contract: contract}, nil
}

// bindEcdsaSortitionPool binds a generic wrapper to an already deployed contract.
func bindEcdsaSortitionPool(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := EcdsaSortitionPoolMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EcdsaSortitionPool *EcdsaSortitionPoolRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EcdsaSortitionPool.Contract.EcdsaSortitionPoolCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EcdsaSortitionPool *EcdsaSortitionPoolRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.EcdsaSortitionPoolTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EcdsaSortitionPool *EcdsaSortitionPoolRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.EcdsaSortitionPoolTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EcdsaSortitionPool.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.contract.Transact(opts, method, params...)
}

// CanRestoreRewardEligibility is a free data retrieval call binding the contract method 0xc0a3f9eb.
//
// Solidity: function canRestoreRewardEligibility(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) CanRestoreRewardEligibility(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "canRestoreRewardEligibility", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CanRestoreRewardEligibility is a free data retrieval call binding the contract method 0xc0a3f9eb.
//
// Solidity: function canRestoreRewardEligibility(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) CanRestoreRewardEligibility(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.CanRestoreRewardEligibility(&_EcdsaSortitionPool.CallOpts, operator)
}

// CanRestoreRewardEligibility is a free data retrieval call binding the contract method 0xc0a3f9eb.
//
// Solidity: function canRestoreRewardEligibility(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) CanRestoreRewardEligibility(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.CanRestoreRewardEligibility(&_EcdsaSortitionPool.CallOpts, operator)
}

// ChaosnetOwner is a free data retrieval call binding the contract method 0x7c2cf6cd.
//
// Solidity: function chaosnetOwner() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) ChaosnetOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "chaosnetOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ChaosnetOwner is a free data retrieval call binding the contract method 0x7c2cf6cd.
//
// Solidity: function chaosnetOwner() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) ChaosnetOwner() (common.Address, error) {
	return _EcdsaSortitionPool.Contract.ChaosnetOwner(&_EcdsaSortitionPool.CallOpts)
}

// ChaosnetOwner is a free data retrieval call binding the contract method 0x7c2cf6cd.
//
// Solidity: function chaosnetOwner() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) ChaosnetOwner() (common.Address, error) {
	return _EcdsaSortitionPool.Contract.ChaosnetOwner(&_EcdsaSortitionPool.CallOpts)
}

// GetAvailableRewards is a free data retrieval call binding the contract method 0x873e31fa.
//
// Solidity: function getAvailableRewards(address operator) view returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) GetAvailableRewards(opts *bind.CallOpts, operator common.Address) (*big.Int, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "getAvailableRewards", operator)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetAvailableRewards is a free data retrieval call binding the contract method 0x873e31fa.
//
// Solidity: function getAvailableRewards(address operator) view returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) GetAvailableRewards(operator common.Address) (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.GetAvailableRewards(&_EcdsaSortitionPool.CallOpts, operator)
}

// GetAvailableRewards is a free data retrieval call binding the contract method 0x873e31fa.
//
// Solidity: function getAvailableRewards(address operator) view returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) GetAvailableRewards(operator common.Address) (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.GetAvailableRewards(&_EcdsaSortitionPool.CallOpts, operator)
}

// GetIDOperator is a free data retrieval call binding the contract method 0x8871ca5d.
//
// Solidity: function getIDOperator(uint32 id) view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) GetIDOperator(opts *bind.CallOpts, id uint32) (common.Address, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "getIDOperator", id)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetIDOperator is a free data retrieval call binding the contract method 0x8871ca5d.
//
// Solidity: function getIDOperator(uint32 id) view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) GetIDOperator(id uint32) (common.Address, error) {
	return _EcdsaSortitionPool.Contract.GetIDOperator(&_EcdsaSortitionPool.CallOpts, id)
}

// GetIDOperator is a free data retrieval call binding the contract method 0x8871ca5d.
//
// Solidity: function getIDOperator(uint32 id) view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) GetIDOperator(id uint32) (common.Address, error) {
	return _EcdsaSortitionPool.Contract.GetIDOperator(&_EcdsaSortitionPool.CallOpts, id)
}

// GetIDOperators is a free data retrieval call binding the contract method 0xf7f9a8fa.
//
// Solidity: function getIDOperators(uint32[] ids) view returns(address[])
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) GetIDOperators(opts *bind.CallOpts, ids []uint32) ([]common.Address, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "getIDOperators", ids)

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetIDOperators is a free data retrieval call binding the contract method 0xf7f9a8fa.
//
// Solidity: function getIDOperators(uint32[] ids) view returns(address[])
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) GetIDOperators(ids []uint32) ([]common.Address, error) {
	return _EcdsaSortitionPool.Contract.GetIDOperators(&_EcdsaSortitionPool.CallOpts, ids)
}

// GetIDOperators is a free data retrieval call binding the contract method 0xf7f9a8fa.
//
// Solidity: function getIDOperators(uint32[] ids) view returns(address[])
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) GetIDOperators(ids []uint32) ([]common.Address, error) {
	return _EcdsaSortitionPool.Contract.GetIDOperators(&_EcdsaSortitionPool.CallOpts, ids)
}

// GetOperatorID is a free data retrieval call binding the contract method 0x5a48b46b.
//
// Solidity: function getOperatorID(address operator) view returns(uint32)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) GetOperatorID(opts *bind.CallOpts, operator common.Address) (uint32, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "getOperatorID", operator)

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// GetOperatorID is a free data retrieval call binding the contract method 0x5a48b46b.
//
// Solidity: function getOperatorID(address operator) view returns(uint32)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) GetOperatorID(operator common.Address) (uint32, error) {
	return _EcdsaSortitionPool.Contract.GetOperatorID(&_EcdsaSortitionPool.CallOpts, operator)
}

// GetOperatorID is a free data retrieval call binding the contract method 0x5a48b46b.
//
// Solidity: function getOperatorID(address operator) view returns(uint32)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) GetOperatorID(operator common.Address) (uint32, error) {
	return _EcdsaSortitionPool.Contract.GetOperatorID(&_EcdsaSortitionPool.CallOpts, operator)
}

// GetPoolWeight is a free data retrieval call binding the contract method 0x5757ed5b.
//
// Solidity: function getPoolWeight(address operator) view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) GetPoolWeight(opts *bind.CallOpts, operator common.Address) (*big.Int, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "getPoolWeight", operator)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetPoolWeight is a free data retrieval call binding the contract method 0x5757ed5b.
//
// Solidity: function getPoolWeight(address operator) view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) GetPoolWeight(operator common.Address) (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.GetPoolWeight(&_EcdsaSortitionPool.CallOpts, operator)
}

// GetPoolWeight is a free data retrieval call binding the contract method 0x5757ed5b.
//
// Solidity: function getPoolWeight(address operator) view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) GetPoolWeight(operator common.Address) (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.GetPoolWeight(&_EcdsaSortitionPool.CallOpts, operator)
}

// IneligibleEarnedRewards is a free data retrieval call binding the contract method 0xa7a7d391.
//
// Solidity: function ineligibleEarnedRewards() view returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IneligibleEarnedRewards(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "ineligibleEarnedRewards")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// IneligibleEarnedRewards is a free data retrieval call binding the contract method 0xa7a7d391.
//
// Solidity: function ineligibleEarnedRewards() view returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IneligibleEarnedRewards() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.IneligibleEarnedRewards(&_EcdsaSortitionPool.CallOpts)
}

// IneligibleEarnedRewards is a free data retrieval call binding the contract method 0xa7a7d391.
//
// Solidity: function ineligibleEarnedRewards() view returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IneligibleEarnedRewards() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.IneligibleEarnedRewards(&_EcdsaSortitionPool.CallOpts)
}

// IsBetaOperator is a free data retrieval call binding the contract method 0x398ece9c.
//
// Solidity: function isBetaOperator(address ) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IsBetaOperator(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "isBetaOperator", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsBetaOperator is a free data retrieval call binding the contract method 0x398ece9c.
//
// Solidity: function isBetaOperator(address ) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IsBetaOperator(arg0 common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsBetaOperator(&_EcdsaSortitionPool.CallOpts, arg0)
}

// IsBetaOperator is a free data retrieval call binding the contract method 0x398ece9c.
//
// Solidity: function isBetaOperator(address ) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IsBetaOperator(arg0 common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsBetaOperator(&_EcdsaSortitionPool.CallOpts, arg0)
}

// IsChaosnetActive is a free data retrieval call binding the contract method 0xb0f3828e.
//
// Solidity: function isChaosnetActive() view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IsChaosnetActive(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "isChaosnetActive")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsChaosnetActive is a free data retrieval call binding the contract method 0xb0f3828e.
//
// Solidity: function isChaosnetActive() view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IsChaosnetActive() (bool, error) {
	return _EcdsaSortitionPool.Contract.IsChaosnetActive(&_EcdsaSortitionPool.CallOpts)
}

// IsChaosnetActive is a free data retrieval call binding the contract method 0xb0f3828e.
//
// Solidity: function isChaosnetActive() view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IsChaosnetActive() (bool, error) {
	return _EcdsaSortitionPool.Contract.IsChaosnetActive(&_EcdsaSortitionPool.CallOpts)
}

// IsEligibleForRewards is a free data retrieval call binding the contract method 0x660186e6.
//
// Solidity: function isEligibleForRewards(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IsEligibleForRewards(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "isEligibleForRewards", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsEligibleForRewards is a free data retrieval call binding the contract method 0x660186e6.
//
// Solidity: function isEligibleForRewards(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IsEligibleForRewards(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsEligibleForRewards(&_EcdsaSortitionPool.CallOpts, operator)
}

// IsEligibleForRewards is a free data retrieval call binding the contract method 0x660186e6.
//
// Solidity: function isEligibleForRewards(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IsEligibleForRewards(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsEligibleForRewards(&_EcdsaSortitionPool.CallOpts, operator)
}

// IsLocked is a free data retrieval call binding the contract method 0xa4e2d634.
//
// Solidity: function isLocked() view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IsLocked(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "isLocked")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsLocked is a free data retrieval call binding the contract method 0xa4e2d634.
//
// Solidity: function isLocked() view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IsLocked() (bool, error) {
	return _EcdsaSortitionPool.Contract.IsLocked(&_EcdsaSortitionPool.CallOpts)
}

// IsLocked is a free data retrieval call binding the contract method 0xa4e2d634.
//
// Solidity: function isLocked() view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IsLocked() (bool, error) {
	return _EcdsaSortitionPool.Contract.IsLocked(&_EcdsaSortitionPool.CallOpts)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IsOperatorInPool(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "isOperatorInPool", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsOperatorInPool(&_EcdsaSortitionPool.CallOpts, operator)
}

// IsOperatorInPool is a free data retrieval call binding the contract method 0xf7186ce0.
//
// Solidity: function isOperatorInPool(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IsOperatorInPool(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsOperatorInPool(&_EcdsaSortitionPool.CallOpts, operator)
}

// IsOperatorRegistered is a free data retrieval call binding the contract method 0x6b1906f8.
//
// Solidity: function isOperatorRegistered(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IsOperatorRegistered(opts *bind.CallOpts, operator common.Address) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "isOperatorRegistered", operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorRegistered is a free data retrieval call binding the contract method 0x6b1906f8.
//
// Solidity: function isOperatorRegistered(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IsOperatorRegistered(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsOperatorRegistered(&_EcdsaSortitionPool.CallOpts, operator)
}

// IsOperatorRegistered is a free data retrieval call binding the contract method 0x6b1906f8.
//
// Solidity: function isOperatorRegistered(address operator) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IsOperatorRegistered(operator common.Address) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsOperatorRegistered(&_EcdsaSortitionPool.CallOpts, operator)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0x4de824f0.
//
// Solidity: function isOperatorUpToDate(address operator, uint256 authorizedStake) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) IsOperatorUpToDate(opts *bind.CallOpts, operator common.Address, authorizedStake *big.Int) (bool, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "isOperatorUpToDate", operator, authorizedStake)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0x4de824f0.
//
// Solidity: function isOperatorUpToDate(address operator, uint256 authorizedStake) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) IsOperatorUpToDate(operator common.Address, authorizedStake *big.Int) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsOperatorUpToDate(&_EcdsaSortitionPool.CallOpts, operator, authorizedStake)
}

// IsOperatorUpToDate is a free data retrieval call binding the contract method 0x4de824f0.
//
// Solidity: function isOperatorUpToDate(address operator, uint256 authorizedStake) view returns(bool)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) IsOperatorUpToDate(operator common.Address, authorizedStake *big.Int) (bool, error) {
	return _EcdsaSortitionPool.Contract.IsOperatorUpToDate(&_EcdsaSortitionPool.CallOpts, operator, authorizedStake)
}

// OperatorsInPool is a free data retrieval call binding the contract method 0xe7bfd899.
//
// Solidity: function operatorsInPool() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) OperatorsInPool(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "operatorsInPool")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// OperatorsInPool is a free data retrieval call binding the contract method 0xe7bfd899.
//
// Solidity: function operatorsInPool() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) OperatorsInPool() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.OperatorsInPool(&_EcdsaSortitionPool.CallOpts)
}

// OperatorsInPool is a free data retrieval call binding the contract method 0xe7bfd899.
//
// Solidity: function operatorsInPool() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) OperatorsInPool() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.OperatorsInPool(&_EcdsaSortitionPool.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) Owner() (common.Address, error) {
	return _EcdsaSortitionPool.Contract.Owner(&_EcdsaSortitionPool.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) Owner() (common.Address, error) {
	return _EcdsaSortitionPool.Contract.Owner(&_EcdsaSortitionPool.CallOpts)
}

// PoolWeightDivisor is a free data retrieval call binding the contract method 0x43a3db30.
//
// Solidity: function poolWeightDivisor() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) PoolWeightDivisor(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "poolWeightDivisor")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PoolWeightDivisor is a free data retrieval call binding the contract method 0x43a3db30.
//
// Solidity: function poolWeightDivisor() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) PoolWeightDivisor() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.PoolWeightDivisor(&_EcdsaSortitionPool.CallOpts)
}

// PoolWeightDivisor is a free data retrieval call binding the contract method 0x43a3db30.
//
// Solidity: function poolWeightDivisor() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) PoolWeightDivisor() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.PoolWeightDivisor(&_EcdsaSortitionPool.CallOpts)
}

// RewardToken is a free data retrieval call binding the contract method 0xf7c618c1.
//
// Solidity: function rewardToken() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) RewardToken(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "rewardToken")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// RewardToken is a free data retrieval call binding the contract method 0xf7c618c1.
//
// Solidity: function rewardToken() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) RewardToken() (common.Address, error) {
	return _EcdsaSortitionPool.Contract.RewardToken(&_EcdsaSortitionPool.CallOpts)
}

// RewardToken is a free data retrieval call binding the contract method 0xf7c618c1.
//
// Solidity: function rewardToken() view returns(address)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) RewardToken() (common.Address, error) {
	return _EcdsaSortitionPool.Contract.RewardToken(&_EcdsaSortitionPool.CallOpts)
}

// RewardsEligibilityRestorableAt is a free data retrieval call binding the contract method 0x00983b73.
//
// Solidity: function rewardsEligibilityRestorableAt(address operator) view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) RewardsEligibilityRestorableAt(opts *bind.CallOpts, operator common.Address) (*big.Int, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "rewardsEligibilityRestorableAt", operator)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RewardsEligibilityRestorableAt is a free data retrieval call binding the contract method 0x00983b73.
//
// Solidity: function rewardsEligibilityRestorableAt(address operator) view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) RewardsEligibilityRestorableAt(operator common.Address) (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.RewardsEligibilityRestorableAt(&_EcdsaSortitionPool.CallOpts, operator)
}

// RewardsEligibilityRestorableAt is a free data retrieval call binding the contract method 0x00983b73.
//
// Solidity: function rewardsEligibilityRestorableAt(address operator) view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) RewardsEligibilityRestorableAt(operator common.Address) (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.RewardsEligibilityRestorableAt(&_EcdsaSortitionPool.CallOpts, operator)
}

// SelectGroup is a free data retrieval call binding the contract method 0x6c2530b9.
//
// Solidity: function selectGroup(uint256 groupSize, bytes32 seed) view returns(uint32[])
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) SelectGroup(opts *bind.CallOpts, groupSize *big.Int, seed [32]byte) ([]uint32, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "selectGroup", groupSize, seed)

	if err != nil {
		return *new([]uint32), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint32)).(*[]uint32)

	return out0, err

}

// SelectGroup is a free data retrieval call binding the contract method 0x6c2530b9.
//
// Solidity: function selectGroup(uint256 groupSize, bytes32 seed) view returns(uint32[])
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) SelectGroup(groupSize *big.Int, seed [32]byte) ([]uint32, error) {
	return _EcdsaSortitionPool.Contract.SelectGroup(&_EcdsaSortitionPool.CallOpts, groupSize, seed)
}

// SelectGroup is a free data retrieval call binding the contract method 0x6c2530b9.
//
// Solidity: function selectGroup(uint256 groupSize, bytes32 seed) view returns(uint32[])
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) SelectGroup(groupSize *big.Int, seed [32]byte) ([]uint32, error) {
	return _EcdsaSortitionPool.Contract.SelectGroup(&_EcdsaSortitionPool.CallOpts, groupSize, seed)
}

// TotalWeight is a free data retrieval call binding the contract method 0x96c82e57.
//
// Solidity: function totalWeight() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCaller) TotalWeight(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _EcdsaSortitionPool.contract.Call(opts, &out, "totalWeight")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalWeight is a free data retrieval call binding the contract method 0x96c82e57.
//
// Solidity: function totalWeight() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) TotalWeight() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.TotalWeight(&_EcdsaSortitionPool.CallOpts)
}

// TotalWeight is a free data retrieval call binding the contract method 0x96c82e57.
//
// Solidity: function totalWeight() view returns(uint256)
func (_EcdsaSortitionPool *EcdsaSortitionPoolCallerSession) TotalWeight() (*big.Int, error) {
	return _EcdsaSortitionPool.Contract.TotalWeight(&_EcdsaSortitionPool.CallOpts)
}

// AddBetaOperators is a paid mutator transaction binding the contract method 0x3e723fc9.
//
// Solidity: function addBetaOperators(address[] operators) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) AddBetaOperators(opts *bind.TransactOpts, operators []common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "addBetaOperators", operators)
}

// AddBetaOperators is a paid mutator transaction binding the contract method 0x3e723fc9.
//
// Solidity: function addBetaOperators(address[] operators) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) AddBetaOperators(operators []common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.AddBetaOperators(&_EcdsaSortitionPool.TransactOpts, operators)
}

// AddBetaOperators is a paid mutator transaction binding the contract method 0x3e723fc9.
//
// Solidity: function addBetaOperators(address[] operators) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) AddBetaOperators(operators []common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.AddBetaOperators(&_EcdsaSortitionPool.TransactOpts, operators)
}

// DeactivateChaosnet is a paid mutator transaction binding the contract method 0xf23baf4a.
//
// Solidity: function deactivateChaosnet() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) DeactivateChaosnet(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "deactivateChaosnet")
}

// DeactivateChaosnet is a paid mutator transaction binding the contract method 0xf23baf4a.
//
// Solidity: function deactivateChaosnet() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) DeactivateChaosnet() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.DeactivateChaosnet(&_EcdsaSortitionPool.TransactOpts)
}

// DeactivateChaosnet is a paid mutator transaction binding the contract method 0xf23baf4a.
//
// Solidity: function deactivateChaosnet() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) DeactivateChaosnet() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.DeactivateChaosnet(&_EcdsaSortitionPool.TransactOpts)
}

// InsertOperator is a paid mutator transaction binding the contract method 0x241a4188.
//
// Solidity: function insertOperator(address operator, uint256 authorizedStake) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) InsertOperator(opts *bind.TransactOpts, operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "insertOperator", operator, authorizedStake)
}

// InsertOperator is a paid mutator transaction binding the contract method 0x241a4188.
//
// Solidity: function insertOperator(address operator, uint256 authorizedStake) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) InsertOperator(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.InsertOperator(&_EcdsaSortitionPool.TransactOpts, operator, authorizedStake)
}

// InsertOperator is a paid mutator transaction binding the contract method 0x241a4188.
//
// Solidity: function insertOperator(address operator, uint256 authorizedStake) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) InsertOperator(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.InsertOperator(&_EcdsaSortitionPool.TransactOpts, operator, authorizedStake)
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) Lock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "lock")
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) Lock() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.Lock(&_EcdsaSortitionPool.TransactOpts)
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) Lock() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.Lock(&_EcdsaSortitionPool.TransactOpts)
}

// ReceiveApproval is a paid mutator transaction binding the contract method 0x8f4ffcb1.
//
// Solidity: function receiveApproval(address sender, uint256 amount, address token, bytes ) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) ReceiveApproval(opts *bind.TransactOpts, sender common.Address, amount *big.Int, token common.Address, arg3 []byte) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "receiveApproval", sender, amount, token, arg3)
}

// ReceiveApproval is a paid mutator transaction binding the contract method 0x8f4ffcb1.
//
// Solidity: function receiveApproval(address sender, uint256 amount, address token, bytes ) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) ReceiveApproval(sender common.Address, amount *big.Int, token common.Address, arg3 []byte) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.ReceiveApproval(&_EcdsaSortitionPool.TransactOpts, sender, amount, token, arg3)
}

// ReceiveApproval is a paid mutator transaction binding the contract method 0x8f4ffcb1.
//
// Solidity: function receiveApproval(address sender, uint256 amount, address token, bytes ) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) ReceiveApproval(sender common.Address, amount *big.Int, token common.Address, arg3 []byte) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.ReceiveApproval(&_EcdsaSortitionPool.TransactOpts, sender, amount, token, arg3)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) RenounceOwnership() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.RenounceOwnership(&_EcdsaSortitionPool.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.RenounceOwnership(&_EcdsaSortitionPool.TransactOpts)
}

// RestoreRewardEligibility is a paid mutator transaction binding the contract method 0xb2f3db4d.
//
// Solidity: function restoreRewardEligibility(address operator) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) RestoreRewardEligibility(opts *bind.TransactOpts, operator common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "restoreRewardEligibility", operator)
}

// RestoreRewardEligibility is a paid mutator transaction binding the contract method 0xb2f3db4d.
//
// Solidity: function restoreRewardEligibility(address operator) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) RestoreRewardEligibility(operator common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.RestoreRewardEligibility(&_EcdsaSortitionPool.TransactOpts, operator)
}

// RestoreRewardEligibility is a paid mutator transaction binding the contract method 0xb2f3db4d.
//
// Solidity: function restoreRewardEligibility(address operator) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) RestoreRewardEligibility(operator common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.RestoreRewardEligibility(&_EcdsaSortitionPool.TransactOpts, operator)
}

// SetRewardIneligibility is a paid mutator transaction binding the contract method 0x942f6892.
//
// Solidity: function setRewardIneligibility(uint32[] operators, uint256 until) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) SetRewardIneligibility(opts *bind.TransactOpts, operators []uint32, until *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "setRewardIneligibility", operators, until)
}

// SetRewardIneligibility is a paid mutator transaction binding the contract method 0x942f6892.
//
// Solidity: function setRewardIneligibility(uint32[] operators, uint256 until) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) SetRewardIneligibility(operators []uint32, until *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.SetRewardIneligibility(&_EcdsaSortitionPool.TransactOpts, operators, until)
}

// SetRewardIneligibility is a paid mutator transaction binding the contract method 0x942f6892.
//
// Solidity: function setRewardIneligibility(uint32[] operators, uint256 until) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) SetRewardIneligibility(operators []uint32, until *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.SetRewardIneligibility(&_EcdsaSortitionPool.TransactOpts, operators, until)
}

// TransferChaosnetOwnerRole is a paid mutator transaction binding the contract method 0xc545b3a9.
//
// Solidity: function transferChaosnetOwnerRole(address newChaosnetOwner) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) TransferChaosnetOwnerRole(opts *bind.TransactOpts, newChaosnetOwner common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "transferChaosnetOwnerRole", newChaosnetOwner)
}

// TransferChaosnetOwnerRole is a paid mutator transaction binding the contract method 0xc545b3a9.
//
// Solidity: function transferChaosnetOwnerRole(address newChaosnetOwner) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) TransferChaosnetOwnerRole(newChaosnetOwner common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.TransferChaosnetOwnerRole(&_EcdsaSortitionPool.TransactOpts, newChaosnetOwner)
}

// TransferChaosnetOwnerRole is a paid mutator transaction binding the contract method 0xc545b3a9.
//
// Solidity: function transferChaosnetOwnerRole(address newChaosnetOwner) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) TransferChaosnetOwnerRole(newChaosnetOwner common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.TransferChaosnetOwnerRole(&_EcdsaSortitionPool.TransactOpts, newChaosnetOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.TransferOwnership(&_EcdsaSortitionPool.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.TransferOwnership(&_EcdsaSortitionPool.TransactOpts, newOwner)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) Unlock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "unlock")
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) Unlock() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.Unlock(&_EcdsaSortitionPool.TransactOpts)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) Unlock() (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.Unlock(&_EcdsaSortitionPool.TransactOpts)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0xdc7520c5.
//
// Solidity: function updateOperatorStatus(address operator, uint256 authorizedStake) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) UpdateOperatorStatus(opts *bind.TransactOpts, operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "updateOperatorStatus", operator, authorizedStake)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0xdc7520c5.
//
// Solidity: function updateOperatorStatus(address operator, uint256 authorizedStake) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) UpdateOperatorStatus(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.UpdateOperatorStatus(&_EcdsaSortitionPool.TransactOpts, operator, authorizedStake)
}

// UpdateOperatorStatus is a paid mutator transaction binding the contract method 0xdc7520c5.
//
// Solidity: function updateOperatorStatus(address operator, uint256 authorizedStake) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) UpdateOperatorStatus(operator common.Address, authorizedStake *big.Int) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.UpdateOperatorStatus(&_EcdsaSortitionPool.TransactOpts, operator, authorizedStake)
}

// WithdrawIneligible is a paid mutator transaction binding the contract method 0xa9649414.
//
// Solidity: function withdrawIneligible(address recipient) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) WithdrawIneligible(opts *bind.TransactOpts, recipient common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "withdrawIneligible", recipient)
}

// WithdrawIneligible is a paid mutator transaction binding the contract method 0xa9649414.
//
// Solidity: function withdrawIneligible(address recipient) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) WithdrawIneligible(recipient common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.WithdrawIneligible(&_EcdsaSortitionPool.TransactOpts, recipient)
}

// WithdrawIneligible is a paid mutator transaction binding the contract method 0xa9649414.
//
// Solidity: function withdrawIneligible(address recipient) returns()
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) WithdrawIneligible(recipient common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.WithdrawIneligible(&_EcdsaSortitionPool.TransactOpts, recipient)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0xe20981ca.
//
// Solidity: function withdrawRewards(address operator, address beneficiary) returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactor) WithdrawRewards(opts *bind.TransactOpts, operator common.Address, beneficiary common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.contract.Transact(opts, "withdrawRewards", operator, beneficiary)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0xe20981ca.
//
// Solidity: function withdrawRewards(address operator, address beneficiary) returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolSession) WithdrawRewards(operator common.Address, beneficiary common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.WithdrawRewards(&_EcdsaSortitionPool.TransactOpts, operator, beneficiary)
}

// WithdrawRewards is a paid mutator transaction binding the contract method 0xe20981ca.
//
// Solidity: function withdrawRewards(address operator, address beneficiary) returns(uint96)
func (_EcdsaSortitionPool *EcdsaSortitionPoolTransactorSession) WithdrawRewards(operator common.Address, beneficiary common.Address) (*types.Transaction, error) {
	return _EcdsaSortitionPool.Contract.WithdrawRewards(&_EcdsaSortitionPool.TransactOpts, operator, beneficiary)
}

// EcdsaSortitionPoolBetaOperatorsAddedIterator is returned from FilterBetaOperatorsAdded and is used to iterate over the raw logs and unpacked data for BetaOperatorsAdded events raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolBetaOperatorsAddedIterator struct {
	Event *EcdsaSortitionPoolBetaOperatorsAdded // Event containing the contract specifics and raw log

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
func (it *EcdsaSortitionPoolBetaOperatorsAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EcdsaSortitionPoolBetaOperatorsAdded)
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
		it.Event = new(EcdsaSortitionPoolBetaOperatorsAdded)
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
func (it *EcdsaSortitionPoolBetaOperatorsAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EcdsaSortitionPoolBetaOperatorsAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EcdsaSortitionPoolBetaOperatorsAdded represents a BetaOperatorsAdded event raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolBetaOperatorsAdded struct {
	Operators []common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterBetaOperatorsAdded is a free log retrieval operation binding the contract event 0x79b60dc9f29a0514f5ce9bf1e89b7add7a22440cde3b203c03a842e3b534071b.
//
// Solidity: event BetaOperatorsAdded(address[] operators)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) FilterBetaOperatorsAdded(opts *bind.FilterOpts) (*EcdsaSortitionPoolBetaOperatorsAddedIterator, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.FilterLogs(opts, "BetaOperatorsAdded")
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolBetaOperatorsAddedIterator{contract: _EcdsaSortitionPool.contract, event: "BetaOperatorsAdded", logs: logs, sub: sub}, nil
}

// WatchBetaOperatorsAdded is a free log subscription operation binding the contract event 0x79b60dc9f29a0514f5ce9bf1e89b7add7a22440cde3b203c03a842e3b534071b.
//
// Solidity: event BetaOperatorsAdded(address[] operators)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) WatchBetaOperatorsAdded(opts *bind.WatchOpts, sink chan<- *EcdsaSortitionPoolBetaOperatorsAdded) (event.Subscription, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.WatchLogs(opts, "BetaOperatorsAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EcdsaSortitionPoolBetaOperatorsAdded)
				if err := _EcdsaSortitionPool.contract.UnpackLog(event, "BetaOperatorsAdded", log); err != nil {
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
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) ParseBetaOperatorsAdded(log types.Log) (*EcdsaSortitionPoolBetaOperatorsAdded, error) {
	event := new(EcdsaSortitionPoolBetaOperatorsAdded)
	if err := _EcdsaSortitionPool.contract.UnpackLog(event, "BetaOperatorsAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EcdsaSortitionPoolChaosnetDeactivatedIterator is returned from FilterChaosnetDeactivated and is used to iterate over the raw logs and unpacked data for ChaosnetDeactivated events raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolChaosnetDeactivatedIterator struct {
	Event *EcdsaSortitionPoolChaosnetDeactivated // Event containing the contract specifics and raw log

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
func (it *EcdsaSortitionPoolChaosnetDeactivatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EcdsaSortitionPoolChaosnetDeactivated)
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
		it.Event = new(EcdsaSortitionPoolChaosnetDeactivated)
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
func (it *EcdsaSortitionPoolChaosnetDeactivatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EcdsaSortitionPoolChaosnetDeactivatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EcdsaSortitionPoolChaosnetDeactivated represents a ChaosnetDeactivated event raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolChaosnetDeactivated struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterChaosnetDeactivated is a free log retrieval operation binding the contract event 0xbea11dc6cfde2788be7e8a6ceef5c8d181bb1c628ba6d71675fca0e754367c74.
//
// Solidity: event ChaosnetDeactivated()
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) FilterChaosnetDeactivated(opts *bind.FilterOpts) (*EcdsaSortitionPoolChaosnetDeactivatedIterator, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.FilterLogs(opts, "ChaosnetDeactivated")
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolChaosnetDeactivatedIterator{contract: _EcdsaSortitionPool.contract, event: "ChaosnetDeactivated", logs: logs, sub: sub}, nil
}

// WatchChaosnetDeactivated is a free log subscription operation binding the contract event 0xbea11dc6cfde2788be7e8a6ceef5c8d181bb1c628ba6d71675fca0e754367c74.
//
// Solidity: event ChaosnetDeactivated()
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) WatchChaosnetDeactivated(opts *bind.WatchOpts, sink chan<- *EcdsaSortitionPoolChaosnetDeactivated) (event.Subscription, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.WatchLogs(opts, "ChaosnetDeactivated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EcdsaSortitionPoolChaosnetDeactivated)
				if err := _EcdsaSortitionPool.contract.UnpackLog(event, "ChaosnetDeactivated", log); err != nil {
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
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) ParseChaosnetDeactivated(log types.Log) (*EcdsaSortitionPoolChaosnetDeactivated, error) {
	event := new(EcdsaSortitionPoolChaosnetDeactivated)
	if err := _EcdsaSortitionPool.contract.UnpackLog(event, "ChaosnetDeactivated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EcdsaSortitionPoolChaosnetOwnerRoleTransferredIterator is returned from FilterChaosnetOwnerRoleTransferred and is used to iterate over the raw logs and unpacked data for ChaosnetOwnerRoleTransferred events raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolChaosnetOwnerRoleTransferredIterator struct {
	Event *EcdsaSortitionPoolChaosnetOwnerRoleTransferred // Event containing the contract specifics and raw log

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
func (it *EcdsaSortitionPoolChaosnetOwnerRoleTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EcdsaSortitionPoolChaosnetOwnerRoleTransferred)
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
		it.Event = new(EcdsaSortitionPoolChaosnetOwnerRoleTransferred)
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
func (it *EcdsaSortitionPoolChaosnetOwnerRoleTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EcdsaSortitionPoolChaosnetOwnerRoleTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EcdsaSortitionPoolChaosnetOwnerRoleTransferred represents a ChaosnetOwnerRoleTransferred event raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolChaosnetOwnerRoleTransferred struct {
	OldChaosnetOwner common.Address
	NewChaosnetOwner common.Address
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterChaosnetOwnerRoleTransferred is a free log retrieval operation binding the contract event 0xf7d2871c195d5dcbeca7c9bfb4f7ae4149d0915a5d3d03c8c2286c9a24e932be.
//
// Solidity: event ChaosnetOwnerRoleTransferred(address oldChaosnetOwner, address newChaosnetOwner)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) FilterChaosnetOwnerRoleTransferred(opts *bind.FilterOpts) (*EcdsaSortitionPoolChaosnetOwnerRoleTransferredIterator, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.FilterLogs(opts, "ChaosnetOwnerRoleTransferred")
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolChaosnetOwnerRoleTransferredIterator{contract: _EcdsaSortitionPool.contract, event: "ChaosnetOwnerRoleTransferred", logs: logs, sub: sub}, nil
}

// WatchChaosnetOwnerRoleTransferred is a free log subscription operation binding the contract event 0xf7d2871c195d5dcbeca7c9bfb4f7ae4149d0915a5d3d03c8c2286c9a24e932be.
//
// Solidity: event ChaosnetOwnerRoleTransferred(address oldChaosnetOwner, address newChaosnetOwner)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) WatchChaosnetOwnerRoleTransferred(opts *bind.WatchOpts, sink chan<- *EcdsaSortitionPoolChaosnetOwnerRoleTransferred) (event.Subscription, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.WatchLogs(opts, "ChaosnetOwnerRoleTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EcdsaSortitionPoolChaosnetOwnerRoleTransferred)
				if err := _EcdsaSortitionPool.contract.UnpackLog(event, "ChaosnetOwnerRoleTransferred", log); err != nil {
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
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) ParseChaosnetOwnerRoleTransferred(log types.Log) (*EcdsaSortitionPoolChaosnetOwnerRoleTransferred, error) {
	event := new(EcdsaSortitionPoolChaosnetOwnerRoleTransferred)
	if err := _EcdsaSortitionPool.contract.UnpackLog(event, "ChaosnetOwnerRoleTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EcdsaSortitionPoolIneligibleForRewardsIterator is returned from FilterIneligibleForRewards and is used to iterate over the raw logs and unpacked data for IneligibleForRewards events raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolIneligibleForRewardsIterator struct {
	Event *EcdsaSortitionPoolIneligibleForRewards // Event containing the contract specifics and raw log

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
func (it *EcdsaSortitionPoolIneligibleForRewardsIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EcdsaSortitionPoolIneligibleForRewards)
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
		it.Event = new(EcdsaSortitionPoolIneligibleForRewards)
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
func (it *EcdsaSortitionPoolIneligibleForRewardsIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EcdsaSortitionPoolIneligibleForRewardsIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EcdsaSortitionPoolIneligibleForRewards represents a IneligibleForRewards event raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolIneligibleForRewards struct {
	Ids   []uint32
	Until *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterIneligibleForRewards is a free log retrieval operation binding the contract event 0x01f5838e3dde8cf4817b958fe95be92bdfeccb34317e1d9f58d1cfe5230de231.
//
// Solidity: event IneligibleForRewards(uint32[] ids, uint256 until)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) FilterIneligibleForRewards(opts *bind.FilterOpts) (*EcdsaSortitionPoolIneligibleForRewardsIterator, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.FilterLogs(opts, "IneligibleForRewards")
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolIneligibleForRewardsIterator{contract: _EcdsaSortitionPool.contract, event: "IneligibleForRewards", logs: logs, sub: sub}, nil
}

// WatchIneligibleForRewards is a free log subscription operation binding the contract event 0x01f5838e3dde8cf4817b958fe95be92bdfeccb34317e1d9f58d1cfe5230de231.
//
// Solidity: event IneligibleForRewards(uint32[] ids, uint256 until)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) WatchIneligibleForRewards(opts *bind.WatchOpts, sink chan<- *EcdsaSortitionPoolIneligibleForRewards) (event.Subscription, error) {

	logs, sub, err := _EcdsaSortitionPool.contract.WatchLogs(opts, "IneligibleForRewards")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EcdsaSortitionPoolIneligibleForRewards)
				if err := _EcdsaSortitionPool.contract.UnpackLog(event, "IneligibleForRewards", log); err != nil {
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
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) ParseIneligibleForRewards(log types.Log) (*EcdsaSortitionPoolIneligibleForRewards, error) {
	event := new(EcdsaSortitionPoolIneligibleForRewards)
	if err := _EcdsaSortitionPool.contract.UnpackLog(event, "IneligibleForRewards", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EcdsaSortitionPoolOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolOwnershipTransferredIterator struct {
	Event *EcdsaSortitionPoolOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *EcdsaSortitionPoolOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EcdsaSortitionPoolOwnershipTransferred)
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
		it.Event = new(EcdsaSortitionPoolOwnershipTransferred)
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
func (it *EcdsaSortitionPoolOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EcdsaSortitionPoolOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EcdsaSortitionPoolOwnershipTransferred represents a OwnershipTransferred event raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*EcdsaSortitionPoolOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _EcdsaSortitionPool.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolOwnershipTransferredIterator{contract: _EcdsaSortitionPool.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *EcdsaSortitionPoolOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _EcdsaSortitionPool.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EcdsaSortitionPoolOwnershipTransferred)
				if err := _EcdsaSortitionPool.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) ParseOwnershipTransferred(log types.Log) (*EcdsaSortitionPoolOwnershipTransferred, error) {
	event := new(EcdsaSortitionPoolOwnershipTransferred)
	if err := _EcdsaSortitionPool.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EcdsaSortitionPoolRewardEligibilityRestoredIterator is returned from FilterRewardEligibilityRestored and is used to iterate over the raw logs and unpacked data for RewardEligibilityRestored events raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolRewardEligibilityRestoredIterator struct {
	Event *EcdsaSortitionPoolRewardEligibilityRestored // Event containing the contract specifics and raw log

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
func (it *EcdsaSortitionPoolRewardEligibilityRestoredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EcdsaSortitionPoolRewardEligibilityRestored)
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
		it.Event = new(EcdsaSortitionPoolRewardEligibilityRestored)
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
func (it *EcdsaSortitionPoolRewardEligibilityRestoredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EcdsaSortitionPoolRewardEligibilityRestoredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EcdsaSortitionPoolRewardEligibilityRestored represents a RewardEligibilityRestored event raised by the EcdsaSortitionPool contract.
type EcdsaSortitionPoolRewardEligibilityRestored struct {
	Operator common.Address
	Id       uint32
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterRewardEligibilityRestored is a free log retrieval operation binding the contract event 0xe61e9f0f049b3bfae1ae903a5e3018c02a008aa0d238ffddf23a4fb4c0278536.
//
// Solidity: event RewardEligibilityRestored(address indexed operator, uint32 indexed id)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) FilterRewardEligibilityRestored(opts *bind.FilterOpts, operator []common.Address, id []uint32) (*EcdsaSortitionPoolRewardEligibilityRestoredIterator, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _EcdsaSortitionPool.contract.FilterLogs(opts, "RewardEligibilityRestored", operatorRule, idRule)
	if err != nil {
		return nil, err
	}
	return &EcdsaSortitionPoolRewardEligibilityRestoredIterator{contract: _EcdsaSortitionPool.contract, event: "RewardEligibilityRestored", logs: logs, sub: sub}, nil
}

// WatchRewardEligibilityRestored is a free log subscription operation binding the contract event 0xe61e9f0f049b3bfae1ae903a5e3018c02a008aa0d238ffddf23a4fb4c0278536.
//
// Solidity: event RewardEligibilityRestored(address indexed operator, uint32 indexed id)
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) WatchRewardEligibilityRestored(opts *bind.WatchOpts, sink chan<- *EcdsaSortitionPoolRewardEligibilityRestored, operator []common.Address, id []uint32) (event.Subscription, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _EcdsaSortitionPool.contract.WatchLogs(opts, "RewardEligibilityRestored", operatorRule, idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EcdsaSortitionPoolRewardEligibilityRestored)
				if err := _EcdsaSortitionPool.contract.UnpackLog(event, "RewardEligibilityRestored", log); err != nil {
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
func (_EcdsaSortitionPool *EcdsaSortitionPoolFilterer) ParseRewardEligibilityRestored(log types.Log) (*EcdsaSortitionPoolRewardEligibilityRestored, error) {
	event := new(EcdsaSortitionPoolRewardEligibilityRestored)
	if err := _EcdsaSortitionPool.contract.UnpackLog(event, "RewardEligibilityRestored", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

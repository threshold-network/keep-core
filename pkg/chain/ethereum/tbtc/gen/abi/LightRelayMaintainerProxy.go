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

// LightRelayMaintainerProxyMetaData contains all meta data concerning the LightRelayMaintainerProxy contract.
var LightRelayMaintainerProxyMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractILightRelay\",\"name\":\"_lightRelay\",\"type\":\"address\"},{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newRelay\",\"type\":\"address\"}],\"name\":\"LightRelayUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"MaintainerAuthorized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"MaintainerDeauthorized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newReimbursementPool\",\"type\":\"address\"}],\"name\":\"ReimbursementPoolUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"retargetGasOffset\",\"type\":\"uint256\"}],\"name\":\"RetargetGasOffsetUpdated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"authorize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"deauthorize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isAuthorized\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lightRelay\",\"outputs\":[{\"internalType\":\"contractILightRelay\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reimbursementPool\",\"outputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"headers\",\"type\":\"bytes\"}],\"name\":\"retarget\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"retargetGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractILightRelay\",\"name\":\"_lightRelay\",\"type\":\"address\"}],\"name\":\"updateLightRelay\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"name\":\"updateReimbursementPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newRetargetGasOffset\",\"type\":\"uint256\"}],\"name\":\"updateRetargetGasOffset\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// LightRelayMaintainerProxyABI is the input ABI used to generate the binding from.
// Deprecated: Use LightRelayMaintainerProxyMetaData.ABI instead.
var LightRelayMaintainerProxyABI = LightRelayMaintainerProxyMetaData.ABI

// LightRelayMaintainerProxy is an auto generated Go binding around an Ethereum contract.
type LightRelayMaintainerProxy struct {
	LightRelayMaintainerProxyCaller     // Read-only binding to the contract
	LightRelayMaintainerProxyTransactor // Write-only binding to the contract
	LightRelayMaintainerProxyFilterer   // Log filterer for contract events
}

// LightRelayMaintainerProxyCaller is an auto generated read-only Go binding around an Ethereum contract.
type LightRelayMaintainerProxyCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LightRelayMaintainerProxyTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LightRelayMaintainerProxyTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LightRelayMaintainerProxyFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LightRelayMaintainerProxyFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LightRelayMaintainerProxySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LightRelayMaintainerProxySession struct {
	Contract     *LightRelayMaintainerProxy // Generic contract binding to set the session for
	CallOpts     bind.CallOpts              // Call options to use throughout this session
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// LightRelayMaintainerProxyCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LightRelayMaintainerProxyCallerSession struct {
	Contract *LightRelayMaintainerProxyCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                    // Call options to use throughout this session
}

// LightRelayMaintainerProxyTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LightRelayMaintainerProxyTransactorSession struct {
	Contract     *LightRelayMaintainerProxyTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                    // Transaction auth options to use throughout this session
}

// LightRelayMaintainerProxyRaw is an auto generated low-level Go binding around an Ethereum contract.
type LightRelayMaintainerProxyRaw struct {
	Contract *LightRelayMaintainerProxy // Generic contract binding to access the raw methods on
}

// LightRelayMaintainerProxyCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LightRelayMaintainerProxyCallerRaw struct {
	Contract *LightRelayMaintainerProxyCaller // Generic read-only contract binding to access the raw methods on
}

// LightRelayMaintainerProxyTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LightRelayMaintainerProxyTransactorRaw struct {
	Contract *LightRelayMaintainerProxyTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLightRelayMaintainerProxy creates a new instance of LightRelayMaintainerProxy, bound to a specific deployed contract.
func NewLightRelayMaintainerProxy(address common.Address, backend bind.ContractBackend) (*LightRelayMaintainerProxy, error) {
	contract, err := bindLightRelayMaintainerProxy(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxy{LightRelayMaintainerProxyCaller: LightRelayMaintainerProxyCaller{contract: contract}, LightRelayMaintainerProxyTransactor: LightRelayMaintainerProxyTransactor{contract: contract}, LightRelayMaintainerProxyFilterer: LightRelayMaintainerProxyFilterer{contract: contract}}, nil
}

// NewLightRelayMaintainerProxyCaller creates a new read-only instance of LightRelayMaintainerProxy, bound to a specific deployed contract.
func NewLightRelayMaintainerProxyCaller(address common.Address, caller bind.ContractCaller) (*LightRelayMaintainerProxyCaller, error) {
	contract, err := bindLightRelayMaintainerProxy(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyCaller{contract: contract}, nil
}

// NewLightRelayMaintainerProxyTransactor creates a new write-only instance of LightRelayMaintainerProxy, bound to a specific deployed contract.
func NewLightRelayMaintainerProxyTransactor(address common.Address, transactor bind.ContractTransactor) (*LightRelayMaintainerProxyTransactor, error) {
	contract, err := bindLightRelayMaintainerProxy(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyTransactor{contract: contract}, nil
}

// NewLightRelayMaintainerProxyFilterer creates a new log filterer instance of LightRelayMaintainerProxy, bound to a specific deployed contract.
func NewLightRelayMaintainerProxyFilterer(address common.Address, filterer bind.ContractFilterer) (*LightRelayMaintainerProxyFilterer, error) {
	contract, err := bindLightRelayMaintainerProxy(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyFilterer{contract: contract}, nil
}

// bindLightRelayMaintainerProxy binds a generic wrapper to an already deployed contract.
func bindLightRelayMaintainerProxy(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := LightRelayMaintainerProxyMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _LightRelayMaintainerProxy.Contract.LightRelayMaintainerProxyCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.LightRelayMaintainerProxyTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.LightRelayMaintainerProxyTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _LightRelayMaintainerProxy.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.contract.Transact(opts, method, params...)
}

// IsAuthorized is a free data retrieval call binding the contract method 0xfe9fbb80.
//
// Solidity: function isAuthorized(address ) view returns(bool)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCaller) IsAuthorized(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _LightRelayMaintainerProxy.contract.Call(opts, &out, "isAuthorized", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsAuthorized is a free data retrieval call binding the contract method 0xfe9fbb80.
//
// Solidity: function isAuthorized(address ) view returns(bool)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) IsAuthorized(arg0 common.Address) (bool, error) {
	return _LightRelayMaintainerProxy.Contract.IsAuthorized(&_LightRelayMaintainerProxy.CallOpts, arg0)
}

// IsAuthorized is a free data retrieval call binding the contract method 0xfe9fbb80.
//
// Solidity: function isAuthorized(address ) view returns(bool)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCallerSession) IsAuthorized(arg0 common.Address) (bool, error) {
	return _LightRelayMaintainerProxy.Contract.IsAuthorized(&_LightRelayMaintainerProxy.CallOpts, arg0)
}

// LightRelay is a free data retrieval call binding the contract method 0x84dc4409.
//
// Solidity: function lightRelay() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCaller) LightRelay(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _LightRelayMaintainerProxy.contract.Call(opts, &out, "lightRelay")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// LightRelay is a free data retrieval call binding the contract method 0x84dc4409.
//
// Solidity: function lightRelay() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) LightRelay() (common.Address, error) {
	return _LightRelayMaintainerProxy.Contract.LightRelay(&_LightRelayMaintainerProxy.CallOpts)
}

// LightRelay is a free data retrieval call binding the contract method 0x84dc4409.
//
// Solidity: function lightRelay() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCallerSession) LightRelay() (common.Address, error) {
	return _LightRelayMaintainerProxy.Contract.LightRelay(&_LightRelayMaintainerProxy.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _LightRelayMaintainerProxy.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) Owner() (common.Address, error) {
	return _LightRelayMaintainerProxy.Contract.Owner(&_LightRelayMaintainerProxy.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCallerSession) Owner() (common.Address, error) {
	return _LightRelayMaintainerProxy.Contract.Owner(&_LightRelayMaintainerProxy.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCaller) ReimbursementPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _LightRelayMaintainerProxy.contract.Call(opts, &out, "reimbursementPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) ReimbursementPool() (common.Address, error) {
	return _LightRelayMaintainerProxy.Contract.ReimbursementPool(&_LightRelayMaintainerProxy.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCallerSession) ReimbursementPool() (common.Address, error) {
	return _LightRelayMaintainerProxy.Contract.ReimbursementPool(&_LightRelayMaintainerProxy.CallOpts)
}

// RetargetGasOffset is a free data retrieval call binding the contract method 0x293aa4a2.
//
// Solidity: function retargetGasOffset() view returns(uint256)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCaller) RetargetGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _LightRelayMaintainerProxy.contract.Call(opts, &out, "retargetGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RetargetGasOffset is a free data retrieval call binding the contract method 0x293aa4a2.
//
// Solidity: function retargetGasOffset() view returns(uint256)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) RetargetGasOffset() (*big.Int, error) {
	return _LightRelayMaintainerProxy.Contract.RetargetGasOffset(&_LightRelayMaintainerProxy.CallOpts)
}

// RetargetGasOffset is a free data retrieval call binding the contract method 0x293aa4a2.
//
// Solidity: function retargetGasOffset() view returns(uint256)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyCallerSession) RetargetGasOffset() (*big.Int, error) {
	return _LightRelayMaintainerProxy.Contract.RetargetGasOffset(&_LightRelayMaintainerProxy.CallOpts)
}

// Authorize is a paid mutator transaction binding the contract method 0xb6a5d7de.
//
// Solidity: function authorize(address maintainer) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) Authorize(opts *bind.TransactOpts, maintainer common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "authorize", maintainer)
}

// Authorize is a paid mutator transaction binding the contract method 0xb6a5d7de.
//
// Solidity: function authorize(address maintainer) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) Authorize(maintainer common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.Authorize(&_LightRelayMaintainerProxy.TransactOpts, maintainer)
}

// Authorize is a paid mutator transaction binding the contract method 0xb6a5d7de.
//
// Solidity: function authorize(address maintainer) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) Authorize(maintainer common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.Authorize(&_LightRelayMaintainerProxy.TransactOpts, maintainer)
}

// Deauthorize is a paid mutator transaction binding the contract method 0x27c97fa5.
//
// Solidity: function deauthorize(address maintainer) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) Deauthorize(opts *bind.TransactOpts, maintainer common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "deauthorize", maintainer)
}

// Deauthorize is a paid mutator transaction binding the contract method 0x27c97fa5.
//
// Solidity: function deauthorize(address maintainer) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) Deauthorize(maintainer common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.Deauthorize(&_LightRelayMaintainerProxy.TransactOpts, maintainer)
}

// Deauthorize is a paid mutator transaction binding the contract method 0x27c97fa5.
//
// Solidity: function deauthorize(address maintainer) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) Deauthorize(maintainer common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.Deauthorize(&_LightRelayMaintainerProxy.TransactOpts, maintainer)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) RenounceOwnership() (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.RenounceOwnership(&_LightRelayMaintainerProxy.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.RenounceOwnership(&_LightRelayMaintainerProxy.TransactOpts)
}

// Retarget is a paid mutator transaction binding the contract method 0x7ca5b1dd.
//
// Solidity: function retarget(bytes headers) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) Retarget(opts *bind.TransactOpts, headers []byte) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "retarget", headers)
}

// Retarget is a paid mutator transaction binding the contract method 0x7ca5b1dd.
//
// Solidity: function retarget(bytes headers) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) Retarget(headers []byte) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.Retarget(&_LightRelayMaintainerProxy.TransactOpts, headers)
}

// Retarget is a paid mutator transaction binding the contract method 0x7ca5b1dd.
//
// Solidity: function retarget(bytes headers) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) Retarget(headers []byte) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.Retarget(&_LightRelayMaintainerProxy.TransactOpts, headers)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.TransferOwnership(&_LightRelayMaintainerProxy.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.TransferOwnership(&_LightRelayMaintainerProxy.TransactOpts, newOwner)
}

// UpdateLightRelay is a paid mutator transaction binding the contract method 0x9d5f29df.
//
// Solidity: function updateLightRelay(address _lightRelay) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) UpdateLightRelay(opts *bind.TransactOpts, _lightRelay common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "updateLightRelay", _lightRelay)
}

// UpdateLightRelay is a paid mutator transaction binding the contract method 0x9d5f29df.
//
// Solidity: function updateLightRelay(address _lightRelay) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) UpdateLightRelay(_lightRelay common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.UpdateLightRelay(&_LightRelayMaintainerProxy.TransactOpts, _lightRelay)
}

// UpdateLightRelay is a paid mutator transaction binding the contract method 0x9d5f29df.
//
// Solidity: function updateLightRelay(address _lightRelay) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) UpdateLightRelay(_lightRelay common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.UpdateLightRelay(&_LightRelayMaintainerProxy.TransactOpts, _lightRelay)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) UpdateReimbursementPool(opts *bind.TransactOpts, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "updateReimbursementPool", _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.UpdateReimbursementPool(&_LightRelayMaintainerProxy.TransactOpts, _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.UpdateReimbursementPool(&_LightRelayMaintainerProxy.TransactOpts, _reimbursementPool)
}

// UpdateRetargetGasOffset is a paid mutator transaction binding the contract method 0xb214b498.
//
// Solidity: function updateRetargetGasOffset(uint256 newRetargetGasOffset) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactor) UpdateRetargetGasOffset(opts *bind.TransactOpts, newRetargetGasOffset *big.Int) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.contract.Transact(opts, "updateRetargetGasOffset", newRetargetGasOffset)
}

// UpdateRetargetGasOffset is a paid mutator transaction binding the contract method 0xb214b498.
//
// Solidity: function updateRetargetGasOffset(uint256 newRetargetGasOffset) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxySession) UpdateRetargetGasOffset(newRetargetGasOffset *big.Int) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.UpdateRetargetGasOffset(&_LightRelayMaintainerProxy.TransactOpts, newRetargetGasOffset)
}

// UpdateRetargetGasOffset is a paid mutator transaction binding the contract method 0xb214b498.
//
// Solidity: function updateRetargetGasOffset(uint256 newRetargetGasOffset) returns()
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyTransactorSession) UpdateRetargetGasOffset(newRetargetGasOffset *big.Int) (*types.Transaction, error) {
	return _LightRelayMaintainerProxy.Contract.UpdateRetargetGasOffset(&_LightRelayMaintainerProxy.TransactOpts, newRetargetGasOffset)
}

// LightRelayMaintainerProxyLightRelayUpdatedIterator is returned from FilterLightRelayUpdated and is used to iterate over the raw logs and unpacked data for LightRelayUpdated events raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyLightRelayUpdatedIterator struct {
	Event *LightRelayMaintainerProxyLightRelayUpdated // Event containing the contract specifics and raw log

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
func (it *LightRelayMaintainerProxyLightRelayUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayMaintainerProxyLightRelayUpdated)
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
		it.Event = new(LightRelayMaintainerProxyLightRelayUpdated)
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
func (it *LightRelayMaintainerProxyLightRelayUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayMaintainerProxyLightRelayUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayMaintainerProxyLightRelayUpdated represents a LightRelayUpdated event raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyLightRelayUpdated struct {
	NewRelay common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterLightRelayUpdated is a free log retrieval operation binding the contract event 0xa46f2602ab8b56454b3c3f2ac8bbad3a47d901cc73f0d800b2d607eb68743428.
//
// Solidity: event LightRelayUpdated(address newRelay)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) FilterLightRelayUpdated(opts *bind.FilterOpts) (*LightRelayMaintainerProxyLightRelayUpdatedIterator, error) {

	logs, sub, err := _LightRelayMaintainerProxy.contract.FilterLogs(opts, "LightRelayUpdated")
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyLightRelayUpdatedIterator{contract: _LightRelayMaintainerProxy.contract, event: "LightRelayUpdated", logs: logs, sub: sub}, nil
}

// WatchLightRelayUpdated is a free log subscription operation binding the contract event 0xa46f2602ab8b56454b3c3f2ac8bbad3a47d901cc73f0d800b2d607eb68743428.
//
// Solidity: event LightRelayUpdated(address newRelay)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) WatchLightRelayUpdated(opts *bind.WatchOpts, sink chan<- *LightRelayMaintainerProxyLightRelayUpdated) (event.Subscription, error) {

	logs, sub, err := _LightRelayMaintainerProxy.contract.WatchLogs(opts, "LightRelayUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayMaintainerProxyLightRelayUpdated)
				if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "LightRelayUpdated", log); err != nil {
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

// ParseLightRelayUpdated is a log parse operation binding the contract event 0xa46f2602ab8b56454b3c3f2ac8bbad3a47d901cc73f0d800b2d607eb68743428.
//
// Solidity: event LightRelayUpdated(address newRelay)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) ParseLightRelayUpdated(log types.Log) (*LightRelayMaintainerProxyLightRelayUpdated, error) {
	event := new(LightRelayMaintainerProxyLightRelayUpdated)
	if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "LightRelayUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayMaintainerProxyMaintainerAuthorizedIterator is returned from FilterMaintainerAuthorized and is used to iterate over the raw logs and unpacked data for MaintainerAuthorized events raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyMaintainerAuthorizedIterator struct {
	Event *LightRelayMaintainerProxyMaintainerAuthorized // Event containing the contract specifics and raw log

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
func (it *LightRelayMaintainerProxyMaintainerAuthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayMaintainerProxyMaintainerAuthorized)
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
		it.Event = new(LightRelayMaintainerProxyMaintainerAuthorized)
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
func (it *LightRelayMaintainerProxyMaintainerAuthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayMaintainerProxyMaintainerAuthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayMaintainerProxyMaintainerAuthorized represents a MaintainerAuthorized event raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyMaintainerAuthorized struct {
	Maintainer common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterMaintainerAuthorized is a free log retrieval operation binding the contract event 0xd47aed730622d4816ba8c667c5644d30d0042568ce573955dfa5cafd60ecba4b.
//
// Solidity: event MaintainerAuthorized(address indexed maintainer)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) FilterMaintainerAuthorized(opts *bind.FilterOpts, maintainer []common.Address) (*LightRelayMaintainerProxyMaintainerAuthorizedIterator, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _LightRelayMaintainerProxy.contract.FilterLogs(opts, "MaintainerAuthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyMaintainerAuthorizedIterator{contract: _LightRelayMaintainerProxy.contract, event: "MaintainerAuthorized", logs: logs, sub: sub}, nil
}

// WatchMaintainerAuthorized is a free log subscription operation binding the contract event 0xd47aed730622d4816ba8c667c5644d30d0042568ce573955dfa5cafd60ecba4b.
//
// Solidity: event MaintainerAuthorized(address indexed maintainer)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) WatchMaintainerAuthorized(opts *bind.WatchOpts, sink chan<- *LightRelayMaintainerProxyMaintainerAuthorized, maintainer []common.Address) (event.Subscription, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _LightRelayMaintainerProxy.contract.WatchLogs(opts, "MaintainerAuthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayMaintainerProxyMaintainerAuthorized)
				if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "MaintainerAuthorized", log); err != nil {
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

// ParseMaintainerAuthorized is a log parse operation binding the contract event 0xd47aed730622d4816ba8c667c5644d30d0042568ce573955dfa5cafd60ecba4b.
//
// Solidity: event MaintainerAuthorized(address indexed maintainer)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) ParseMaintainerAuthorized(log types.Log) (*LightRelayMaintainerProxyMaintainerAuthorized, error) {
	event := new(LightRelayMaintainerProxyMaintainerAuthorized)
	if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "MaintainerAuthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayMaintainerProxyMaintainerDeauthorizedIterator is returned from FilterMaintainerDeauthorized and is used to iterate over the raw logs and unpacked data for MaintainerDeauthorized events raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyMaintainerDeauthorizedIterator struct {
	Event *LightRelayMaintainerProxyMaintainerDeauthorized // Event containing the contract specifics and raw log

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
func (it *LightRelayMaintainerProxyMaintainerDeauthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayMaintainerProxyMaintainerDeauthorized)
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
		it.Event = new(LightRelayMaintainerProxyMaintainerDeauthorized)
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
func (it *LightRelayMaintainerProxyMaintainerDeauthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayMaintainerProxyMaintainerDeauthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayMaintainerProxyMaintainerDeauthorized represents a MaintainerDeauthorized event raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyMaintainerDeauthorized struct {
	Maintainer common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterMaintainerDeauthorized is a free log retrieval operation binding the contract event 0xc74b7e7972a420ccf708a44401a97c1bcaacb21a25a1e0f83ee40841654b2ec8.
//
// Solidity: event MaintainerDeauthorized(address indexed maintainer)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) FilterMaintainerDeauthorized(opts *bind.FilterOpts, maintainer []common.Address) (*LightRelayMaintainerProxyMaintainerDeauthorizedIterator, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _LightRelayMaintainerProxy.contract.FilterLogs(opts, "MaintainerDeauthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyMaintainerDeauthorizedIterator{contract: _LightRelayMaintainerProxy.contract, event: "MaintainerDeauthorized", logs: logs, sub: sub}, nil
}

// WatchMaintainerDeauthorized is a free log subscription operation binding the contract event 0xc74b7e7972a420ccf708a44401a97c1bcaacb21a25a1e0f83ee40841654b2ec8.
//
// Solidity: event MaintainerDeauthorized(address indexed maintainer)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) WatchMaintainerDeauthorized(opts *bind.WatchOpts, sink chan<- *LightRelayMaintainerProxyMaintainerDeauthorized, maintainer []common.Address) (event.Subscription, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _LightRelayMaintainerProxy.contract.WatchLogs(opts, "MaintainerDeauthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayMaintainerProxyMaintainerDeauthorized)
				if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "MaintainerDeauthorized", log); err != nil {
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

// ParseMaintainerDeauthorized is a log parse operation binding the contract event 0xc74b7e7972a420ccf708a44401a97c1bcaacb21a25a1e0f83ee40841654b2ec8.
//
// Solidity: event MaintainerDeauthorized(address indexed maintainer)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) ParseMaintainerDeauthorized(log types.Log) (*LightRelayMaintainerProxyMaintainerDeauthorized, error) {
	event := new(LightRelayMaintainerProxyMaintainerDeauthorized)
	if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "MaintainerDeauthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayMaintainerProxyOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyOwnershipTransferredIterator struct {
	Event *LightRelayMaintainerProxyOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *LightRelayMaintainerProxyOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayMaintainerProxyOwnershipTransferred)
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
		it.Event = new(LightRelayMaintainerProxyOwnershipTransferred)
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
func (it *LightRelayMaintainerProxyOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayMaintainerProxyOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayMaintainerProxyOwnershipTransferred represents a OwnershipTransferred event raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*LightRelayMaintainerProxyOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _LightRelayMaintainerProxy.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyOwnershipTransferredIterator{contract: _LightRelayMaintainerProxy.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *LightRelayMaintainerProxyOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _LightRelayMaintainerProxy.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayMaintainerProxyOwnershipTransferred)
				if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) ParseOwnershipTransferred(log types.Log) (*LightRelayMaintainerProxyOwnershipTransferred, error) {
	event := new(LightRelayMaintainerProxyOwnershipTransferred)
	if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayMaintainerProxyReimbursementPoolUpdatedIterator is returned from FilterReimbursementPoolUpdated and is used to iterate over the raw logs and unpacked data for ReimbursementPoolUpdated events raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyReimbursementPoolUpdatedIterator struct {
	Event *LightRelayMaintainerProxyReimbursementPoolUpdated // Event containing the contract specifics and raw log

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
func (it *LightRelayMaintainerProxyReimbursementPoolUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayMaintainerProxyReimbursementPoolUpdated)
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
		it.Event = new(LightRelayMaintainerProxyReimbursementPoolUpdated)
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
func (it *LightRelayMaintainerProxyReimbursementPoolUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayMaintainerProxyReimbursementPoolUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayMaintainerProxyReimbursementPoolUpdated represents a ReimbursementPoolUpdated event raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyReimbursementPoolUpdated struct {
	NewReimbursementPool common.Address
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterReimbursementPoolUpdated is a free log retrieval operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) FilterReimbursementPoolUpdated(opts *bind.FilterOpts) (*LightRelayMaintainerProxyReimbursementPoolUpdatedIterator, error) {

	logs, sub, err := _LightRelayMaintainerProxy.contract.FilterLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyReimbursementPoolUpdatedIterator{contract: _LightRelayMaintainerProxy.contract, event: "ReimbursementPoolUpdated", logs: logs, sub: sub}, nil
}

// WatchReimbursementPoolUpdated is a free log subscription operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) WatchReimbursementPoolUpdated(opts *bind.WatchOpts, sink chan<- *LightRelayMaintainerProxyReimbursementPoolUpdated) (event.Subscription, error) {

	logs, sub, err := _LightRelayMaintainerProxy.contract.WatchLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayMaintainerProxyReimbursementPoolUpdated)
				if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
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
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) ParseReimbursementPoolUpdated(log types.Log) (*LightRelayMaintainerProxyReimbursementPoolUpdated, error) {
	event := new(LightRelayMaintainerProxyReimbursementPoolUpdated)
	if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayMaintainerProxyRetargetGasOffsetUpdatedIterator is returned from FilterRetargetGasOffsetUpdated and is used to iterate over the raw logs and unpacked data for RetargetGasOffsetUpdated events raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyRetargetGasOffsetUpdatedIterator struct {
	Event *LightRelayMaintainerProxyRetargetGasOffsetUpdated // Event containing the contract specifics and raw log

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
func (it *LightRelayMaintainerProxyRetargetGasOffsetUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayMaintainerProxyRetargetGasOffsetUpdated)
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
		it.Event = new(LightRelayMaintainerProxyRetargetGasOffsetUpdated)
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
func (it *LightRelayMaintainerProxyRetargetGasOffsetUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayMaintainerProxyRetargetGasOffsetUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayMaintainerProxyRetargetGasOffsetUpdated represents a RetargetGasOffsetUpdated event raised by the LightRelayMaintainerProxy contract.
type LightRelayMaintainerProxyRetargetGasOffsetUpdated struct {
	RetargetGasOffset *big.Int
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRetargetGasOffsetUpdated is a free log retrieval operation binding the contract event 0x47498a8064df4e0fae8efc492d7ce3222dc1dc415d1e1ff688a327ff101940fd.
//
// Solidity: event RetargetGasOffsetUpdated(uint256 retargetGasOffset)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) FilterRetargetGasOffsetUpdated(opts *bind.FilterOpts) (*LightRelayMaintainerProxyRetargetGasOffsetUpdatedIterator, error) {

	logs, sub, err := _LightRelayMaintainerProxy.contract.FilterLogs(opts, "RetargetGasOffsetUpdated")
	if err != nil {
		return nil, err
	}
	return &LightRelayMaintainerProxyRetargetGasOffsetUpdatedIterator{contract: _LightRelayMaintainerProxy.contract, event: "RetargetGasOffsetUpdated", logs: logs, sub: sub}, nil
}

// WatchRetargetGasOffsetUpdated is a free log subscription operation binding the contract event 0x47498a8064df4e0fae8efc492d7ce3222dc1dc415d1e1ff688a327ff101940fd.
//
// Solidity: event RetargetGasOffsetUpdated(uint256 retargetGasOffset)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) WatchRetargetGasOffsetUpdated(opts *bind.WatchOpts, sink chan<- *LightRelayMaintainerProxyRetargetGasOffsetUpdated) (event.Subscription, error) {

	logs, sub, err := _LightRelayMaintainerProxy.contract.WatchLogs(opts, "RetargetGasOffsetUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayMaintainerProxyRetargetGasOffsetUpdated)
				if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "RetargetGasOffsetUpdated", log); err != nil {
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

// ParseRetargetGasOffsetUpdated is a log parse operation binding the contract event 0x47498a8064df4e0fae8efc492d7ce3222dc1dc415d1e1ff688a327ff101940fd.
//
// Solidity: event RetargetGasOffsetUpdated(uint256 retargetGasOffset)
func (_LightRelayMaintainerProxy *LightRelayMaintainerProxyFilterer) ParseRetargetGasOffsetUpdated(log types.Log) (*LightRelayMaintainerProxyRetargetGasOffsetUpdated, error) {
	event := new(LightRelayMaintainerProxyRetargetGasOffsetUpdated)
	if err := _LightRelayMaintainerProxy.contract.UnpackLog(event, "RetargetGasOffsetUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

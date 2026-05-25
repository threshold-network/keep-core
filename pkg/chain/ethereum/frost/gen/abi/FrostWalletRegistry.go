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

// Struct0 is an auto generated low-level Go binding around an user-defined struct.
type Struct0 struct {
	SubmitterMemberIndex     *big.Int
	XOnlyOutputKey           [32]byte
	MembersHash              [32]byte
	MisbehavedMembersIndices []uint8
	Signatures               []byte
	SigningMembersIndices    []*big.Int
	Members                  []uint32
}

// Struct1 is an auto generated low-level Go binding around an user-defined struct.
type Struct1 struct {
	SeedTimeout                     *big.Int
	ResultChallengePeriodLength     *big.Int
	ResultChallengeExtraGas         *big.Int
	ResultSubmissionTimeout         *big.Int
	SubmitterPrecedencePeriodLength *big.Int
}

// FrostWalletRegistryMetaData contains all meta data concerning the FrostWalletRegistry contract.
var FrostWalletRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"event\",\"name\":\"DkgStarted\",\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"seed\",\"type\":\"uint256\"}]},{\"type\":\"event\",\"name\":\"DkgResultSubmitted\",\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"name\":\"seed\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"result\",\"type\":\"tuple\",\"components\":[{\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"name\":\"membersHash\",\"type\":\"bytes32\"},{\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"name\":\"signatures\",\"type\":\"bytes\"},{\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"name\":\"members\",\"type\":\"uint32[]\"}]}]},{\"type\":\"event\",\"name\":\"DkgResultApproved\",\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"name\":\"approver\",\"type\":\"address\"}]},{\"type\":\"event\",\"name\":\"DkgResultChallenged\",\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"resultHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"name\":\"challenger\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"reason\",\"type\":\"string\"}]},{\"type\":\"event\",\"name\":\"DkgTimedOut\",\"anonymous\":false,\"inputs\":[]},{\"type\":\"event\",\"name\":\"DkgSeedTimedOut\",\"anonymous\":false,\"inputs\":[]},{\"type\":\"function\",\"name\":\"submitDkgResult\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"dkgResult\",\"type\":\"tuple\",\"components\":[{\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"name\":\"membersHash\",\"type\":\"bytes32\"},{\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"name\":\"signatures\",\"type\":\"bytes\"},{\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"name\":\"members\",\"type\":\"uint32[]\"}]}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"approveDkgResult\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"dkgResult\",\"type\":\"tuple\",\"components\":[{\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"name\":\"membersHash\",\"type\":\"bytes32\"},{\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"name\":\"signatures\",\"type\":\"bytes\"},{\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"name\":\"members\",\"type\":\"uint32[]\"}]}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"challengeDkgResult\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"dkgResult\",\"type\":\"tuple\",\"components\":[{\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"name\":\"membersHash\",\"type\":\"bytes32\"},{\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"name\":\"signatures\",\"type\":\"bytes\"},{\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"name\":\"members\",\"type\":\"uint32[]\"}]}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"notifyDkgTimeout\",\"stateMutability\":\"nonpayable\",\"inputs\":[],\"outputs\":[]},{\"type\":\"function\",\"name\":\"notifySeedTimeout\",\"stateMutability\":\"nonpayable\",\"inputs\":[],\"outputs\":[]},{\"type\":\"function\",\"name\":\"isDkgResultValid\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"result\",\"type\":\"tuple\",\"components\":[{\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"name\":\"membersHash\",\"type\":\"bytes32\"},{\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"name\":\"signatures\",\"type\":\"bytes\"},{\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"name\":\"members\",\"type\":\"uint32[]\"}]}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\"},{\"name\":\"\",\"type\":\"string\"}]},{\"type\":\"function\",\"name\":\"getWalletCreationState\",\"stateMutability\":\"view\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}]},{\"type\":\"function\",\"name\":\"selectGroup\",\"stateMutability\":\"view\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint32[]\"}]},{\"type\":\"function\",\"name\":\"sortitionPool\",\"stateMutability\":\"view\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\"}]},{\"type\":\"function\",\"name\":\"dkgParameters\",\"stateMutability\":\"view\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"tuple\",\"components\":[{\"name\":\"seedTimeout\",\"type\":\"uint256\"},{\"name\":\"resultChallengePeriodLength\",\"type\":\"uint256\"},{\"name\":\"resultChallengeExtraGas\",\"type\":\"uint256\"},{\"name\":\"resultSubmissionTimeout\",\"type\":\"uint256\"},{\"name\":\"submitterPrecedencePeriodLength\",\"type\":\"uint256\"}]}]}]",
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

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_FrostWalletRegistry *FrostWalletRegistryCaller) DkgParameters(opts *bind.CallOpts) (Struct1, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "dkgParameters")

	if err != nil {
		return *new(Struct1), err
	}

	out0 := *abi.ConvertType(out[0], new(Struct1)).(*Struct1)

	return out0, err

}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_FrostWalletRegistry *FrostWalletRegistrySession) DkgParameters() (Struct1, error) {
	return _FrostWalletRegistry.Contract.DkgParameters(&_FrostWalletRegistry.CallOpts)
}

// DkgParameters is a free data retrieval call binding the contract method 0x08aa090b.
//
// Solidity: function dkgParameters() view returns((uint256,uint256,uint256,uint256,uint256))
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) DkgParameters() (Struct1, error) {
	return _FrostWalletRegistry.Contract.DkgParameters(&_FrostWalletRegistry.CallOpts)
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

// IsDkgResultValid is a free data retrieval call binding the contract method 0x2fd29068.
//
// Solidity: function isDkgResultValid((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) result) view returns(bool, string)
func (_FrostWalletRegistry *FrostWalletRegistryCaller) IsDkgResultValid(opts *bind.CallOpts, result Struct0) (bool, string, error) {
	var out []interface{}
	err := _FrostWalletRegistry.contract.Call(opts, &out, "isDkgResultValid", result)

	if err != nil {
		return *new(bool), *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	out1 := *abi.ConvertType(out[1], new(string)).(*string)

	return out0, out1, err

}

// IsDkgResultValid is a free data retrieval call binding the contract method 0x2fd29068.
//
// Solidity: function isDkgResultValid((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) result) view returns(bool, string)
func (_FrostWalletRegistry *FrostWalletRegistrySession) IsDkgResultValid(result Struct0) (bool, string, error) {
	return _FrostWalletRegistry.Contract.IsDkgResultValid(&_FrostWalletRegistry.CallOpts, result)
}

// IsDkgResultValid is a free data retrieval call binding the contract method 0x2fd29068.
//
// Solidity: function isDkgResultValid((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) result) view returns(bool, string)
func (_FrostWalletRegistry *FrostWalletRegistryCallerSession) IsDkgResultValid(result Struct0) (bool, string, error) {
	return _FrostWalletRegistry.Contract.IsDkgResultValid(&_FrostWalletRegistry.CallOpts, result)
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

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x65b514e2.
//
// Solidity: function approveDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) ApproveDkgResult(opts *bind.TransactOpts, dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "approveDkgResult", dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x65b514e2.
//
// Solidity: function approveDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) ApproveDkgResult(dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ApproveDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// ApproveDkgResult is a paid mutator transaction binding the contract method 0x65b514e2.
//
// Solidity: function approveDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) ApproveDkgResult(dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ApproveDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0xf10f610b.
//
// Solidity: function challengeDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) ChallengeDkgResult(opts *bind.TransactOpts, dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "challengeDkgResult", dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0xf10f610b.
//
// Solidity: function challengeDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) ChallengeDkgResult(dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ChallengeDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// ChallengeDkgResult is a paid mutator transaction binding the contract method 0xf10f610b.
//
// Solidity: function challengeDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) ChallengeDkgResult(dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.ChallengeDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
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

// SubmitDkgResult is a paid mutator transaction binding the contract method 0xd776003c.
//
// Solidity: function submitDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactor) SubmitDkgResult(opts *bind.TransactOpts, dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.contract.Transact(opts, "submitDkgResult", dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0xd776003c.
//
// Solidity: function submitDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistrySession) SubmitDkgResult(dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.SubmitDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
}

// SubmitDkgResult is a paid mutator transaction binding the contract method 0xd776003c.
//
// Solidity: function submitDkgResult((uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) dkgResult) returns()
func (_FrostWalletRegistry *FrostWalletRegistryTransactorSession) SubmitDkgResult(dkgResult Struct0) (*types.Transaction, error) {
	return _FrostWalletRegistry.Contract.SubmitDkgResult(&_FrostWalletRegistry.TransactOpts, dkgResult)
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
	Result     Struct0
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDkgResultSubmitted is a free log retrieval operation binding the contract event 0x4384430e6f3647db226a1f2644148e4c22a002f0e84329434dab4a0f5d5b59aa.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) result)
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

// WatchDkgResultSubmitted is a free log subscription operation binding the contract event 0x4384430e6f3647db226a1f2644148e4c22a002f0e84329434dab4a0f5d5b59aa.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) result)
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

// ParseDkgResultSubmitted is a log parse operation binding the contract event 0x4384430e6f3647db226a1f2644148e4c22a002f0e84329434dab4a0f5d5b59aa.
//
// Solidity: event DkgResultSubmitted(bytes32 indexed resultHash, uint256 indexed seed, (uint256,bytes32,bytes32,uint8[],bytes,uint256[],uint32[]) result)
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

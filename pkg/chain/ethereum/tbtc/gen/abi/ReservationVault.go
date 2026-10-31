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

// ReservationVaultMetaData contains all meta data concerning the ReservationVault contract.
var ReservationVaultMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"inKindFeeDebtSat\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tbtcToken\",\"outputs\":[{\"internalType\":\"contractTBTC\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// ReservationVaultABI is the input ABI used to generate the binding from.
// Deprecated: Use ReservationVaultMetaData.ABI instead.
var ReservationVaultABI = ReservationVaultMetaData.ABI

// ReservationVault is an auto generated Go binding around an Ethereum contract.
type ReservationVault struct {
	ReservationVaultCaller     // Read-only binding to the contract
	ReservationVaultTransactor // Write-only binding to the contract
	ReservationVaultFilterer   // Log filterer for contract events
}

// ReservationVaultCaller is an auto generated read-only Go binding around an Ethereum contract.
type ReservationVaultCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReservationVaultTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ReservationVaultTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReservationVaultFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ReservationVaultFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReservationVaultSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ReservationVaultSession struct {
	Contract     *ReservationVault // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ReservationVaultCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ReservationVaultCallerSession struct {
	Contract *ReservationVaultCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// ReservationVaultTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ReservationVaultTransactorSession struct {
	Contract     *ReservationVaultTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// ReservationVaultRaw is an auto generated low-level Go binding around an Ethereum contract.
type ReservationVaultRaw struct {
	Contract *ReservationVault // Generic contract binding to access the raw methods on
}

// ReservationVaultCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ReservationVaultCallerRaw struct {
	Contract *ReservationVaultCaller // Generic read-only contract binding to access the raw methods on
}

// ReservationVaultTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ReservationVaultTransactorRaw struct {
	Contract *ReservationVaultTransactor // Generic write-only contract binding to access the raw methods on
}

// NewReservationVault creates a new instance of ReservationVault, bound to a specific deployed contract.
func NewReservationVault(address common.Address, backend bind.ContractBackend) (*ReservationVault, error) {
	contract, err := bindReservationVault(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ReservationVault{ReservationVaultCaller: ReservationVaultCaller{contract: contract}, ReservationVaultTransactor: ReservationVaultTransactor{contract: contract}, ReservationVaultFilterer: ReservationVaultFilterer{contract: contract}}, nil
}

// NewReservationVaultCaller creates a new read-only instance of ReservationVault, bound to a specific deployed contract.
func NewReservationVaultCaller(address common.Address, caller bind.ContractCaller) (*ReservationVaultCaller, error) {
	contract, err := bindReservationVault(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ReservationVaultCaller{contract: contract}, nil
}

// NewReservationVaultTransactor creates a new write-only instance of ReservationVault, bound to a specific deployed contract.
func NewReservationVaultTransactor(address common.Address, transactor bind.ContractTransactor) (*ReservationVaultTransactor, error) {
	contract, err := bindReservationVault(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ReservationVaultTransactor{contract: contract}, nil
}

// NewReservationVaultFilterer creates a new log filterer instance of ReservationVault, bound to a specific deployed contract.
func NewReservationVaultFilterer(address common.Address, filterer bind.ContractFilterer) (*ReservationVaultFilterer, error) {
	contract, err := bindReservationVault(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ReservationVaultFilterer{contract: contract}, nil
}

// bindReservationVault binds a generic wrapper to an already deployed contract.
func bindReservationVault(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ReservationVaultMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReservationVault *ReservationVaultRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ReservationVault.Contract.ReservationVaultCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReservationVault *ReservationVaultRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReservationVault.Contract.ReservationVaultTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReservationVault *ReservationVaultRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReservationVault.Contract.ReservationVaultTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReservationVault *ReservationVaultCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ReservationVault.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReservationVault *ReservationVaultTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReservationVault.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReservationVault *ReservationVaultTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReservationVault.Contract.contract.Transact(opts, method, params...)
}

// InKindFeeDebtSat is a free data retrieval call binding the contract method 0xde8b12df.
//
// Solidity: function inKindFeeDebtSat() view returns(uint64)
func (_ReservationVault *ReservationVaultCaller) InKindFeeDebtSat(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _ReservationVault.contract.Call(opts, &out, "inKindFeeDebtSat")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// InKindFeeDebtSat is a free data retrieval call binding the contract method 0xde8b12df.
//
// Solidity: function inKindFeeDebtSat() view returns(uint64)
func (_ReservationVault *ReservationVaultSession) InKindFeeDebtSat() (uint64, error) {
	return _ReservationVault.Contract.InKindFeeDebtSat(&_ReservationVault.CallOpts)
}

// InKindFeeDebtSat is a free data retrieval call binding the contract method 0xde8b12df.
//
// Solidity: function inKindFeeDebtSat() view returns(uint64)
func (_ReservationVault *ReservationVaultCallerSession) InKindFeeDebtSat() (uint64, error) {
	return _ReservationVault.Contract.InKindFeeDebtSat(&_ReservationVault.CallOpts)
}

// TbtcToken is a free data retrieval call binding the contract method 0xe5d3d714.
//
// Solidity: function tbtcToken() view returns(address)
func (_ReservationVault *ReservationVaultCaller) TbtcToken(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ReservationVault.contract.Call(opts, &out, "tbtcToken")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TbtcToken is a free data retrieval call binding the contract method 0xe5d3d714.
//
// Solidity: function tbtcToken() view returns(address)
func (_ReservationVault *ReservationVaultSession) TbtcToken() (common.Address, error) {
	return _ReservationVault.Contract.TbtcToken(&_ReservationVault.CallOpts)
}

// TbtcToken is a free data retrieval call binding the contract method 0xe5d3d714.
//
// Solidity: function tbtcToken() view returns(address)
func (_ReservationVault *ReservationVaultCallerSession) TbtcToken() (common.Address, error) {
	return _ReservationVault.Contract.TbtcToken(&_ReservationVault.CallOpts)
}

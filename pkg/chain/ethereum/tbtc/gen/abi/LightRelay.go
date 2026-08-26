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

// LightRelayMetaData contains all meta data concerning the LightRelay contract.
var LightRelayMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"newStatus\",\"type\":\"bool\"}],\"name\":\"AuthorizationRequirementChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"blockHeight\",\"type\":\"uint256\"}],\"name\":\"Genesis\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newLength\",\"type\":\"uint256\"}],\"name\":\"ProofLengthChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldDifficulty\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newDifficulty\",\"type\":\"uint256\"}],\"name\":\"Retarget\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"}],\"name\":\"SubmitterAuthorized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"}],\"name\":\"SubmitterDeauthorized\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"authorizationRequired\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"}],\"name\":\"authorize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"currentEpoch\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"}],\"name\":\"deauthorize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"genesisHeader\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"genesisHeight\",\"type\":\"uint256\"},{\"internalType\":\"uint64\",\"name\":\"genesisProofLength\",\"type\":\"uint64\"}],\"name\":\"genesis\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"genesisEpoch\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"}],\"name\":\"getBlockDifficulty\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getCurrentAndPrevEpochDifficulty\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"current\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"previous\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getCurrentEpochDifficulty\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"epochNumber\",\"type\":\"uint256\"}],\"name\":\"getEpochDifficulty\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getPrevEpochDifficulty\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getRelayRange\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"relayGenesis\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"currentEpochEnd\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isAuthorized\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"proofLength\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"ready\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"headers\",\"type\":\"bytes\"}],\"name\":\"retarget\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bool\",\"name\":\"status\",\"type\":\"bool\"}],\"name\":\"setAuthorizationStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"newLength\",\"type\":\"uint64\"}],\"name\":\"setProofLength\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"headers\",\"type\":\"bytes\"}],\"name\":\"validateChain\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"startingHeaderTimestamp\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"headerCount\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// LightRelayABI is the input ABI used to generate the binding from.
// Deprecated: Use LightRelayMetaData.ABI instead.
var LightRelayABI = LightRelayMetaData.ABI

// LightRelay is an auto generated Go binding around an Ethereum contract.
type LightRelay struct {
	LightRelayCaller     // Read-only binding to the contract
	LightRelayTransactor // Write-only binding to the contract
	LightRelayFilterer   // Log filterer for contract events
}

// LightRelayCaller is an auto generated read-only Go binding around an Ethereum contract.
type LightRelayCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LightRelayTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LightRelayTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LightRelayFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LightRelayFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LightRelaySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LightRelaySession struct {
	Contract     *LightRelay       // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LightRelayCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LightRelayCallerSession struct {
	Contract *LightRelayCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts     // Call options to use throughout this session
}

// LightRelayTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LightRelayTransactorSession struct {
	Contract     *LightRelayTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// LightRelayRaw is an auto generated low-level Go binding around an Ethereum contract.
type LightRelayRaw struct {
	Contract *LightRelay // Generic contract binding to access the raw methods on
}

// LightRelayCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LightRelayCallerRaw struct {
	Contract *LightRelayCaller // Generic read-only contract binding to access the raw methods on
}

// LightRelayTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LightRelayTransactorRaw struct {
	Contract *LightRelayTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLightRelay creates a new instance of LightRelay, bound to a specific deployed contract.
func NewLightRelay(address common.Address, backend bind.ContractBackend) (*LightRelay, error) {
	contract, err := bindLightRelay(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LightRelay{LightRelayCaller: LightRelayCaller{contract: contract}, LightRelayTransactor: LightRelayTransactor{contract: contract}, LightRelayFilterer: LightRelayFilterer{contract: contract}}, nil
}

// NewLightRelayCaller creates a new read-only instance of LightRelay, bound to a specific deployed contract.
func NewLightRelayCaller(address common.Address, caller bind.ContractCaller) (*LightRelayCaller, error) {
	contract, err := bindLightRelay(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LightRelayCaller{contract: contract}, nil
}

// NewLightRelayTransactor creates a new write-only instance of LightRelay, bound to a specific deployed contract.
func NewLightRelayTransactor(address common.Address, transactor bind.ContractTransactor) (*LightRelayTransactor, error) {
	contract, err := bindLightRelay(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LightRelayTransactor{contract: contract}, nil
}

// NewLightRelayFilterer creates a new log filterer instance of LightRelay, bound to a specific deployed contract.
func NewLightRelayFilterer(address common.Address, filterer bind.ContractFilterer) (*LightRelayFilterer, error) {
	contract, err := bindLightRelay(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LightRelayFilterer{contract: contract}, nil
}

// bindLightRelay binds a generic wrapper to an already deployed contract.
func bindLightRelay(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := LightRelayMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LightRelay *LightRelayRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _LightRelay.Contract.LightRelayCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LightRelay *LightRelayRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LightRelay.Contract.LightRelayTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LightRelay *LightRelayRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LightRelay.Contract.LightRelayTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LightRelay *LightRelayCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _LightRelay.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LightRelay *LightRelayTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LightRelay.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LightRelay *LightRelayTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LightRelay.Contract.contract.Transact(opts, method, params...)
}

// AuthorizationRequired is a free data retrieval call binding the contract method 0x95410d2b.
//
// Solidity: function authorizationRequired() view returns(bool)
func (_LightRelay *LightRelayCaller) AuthorizationRequired(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "authorizationRequired")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// AuthorizationRequired is a free data retrieval call binding the contract method 0x95410d2b.
//
// Solidity: function authorizationRequired() view returns(bool)
func (_LightRelay *LightRelaySession) AuthorizationRequired() (bool, error) {
	return _LightRelay.Contract.AuthorizationRequired(&_LightRelay.CallOpts)
}

// AuthorizationRequired is a free data retrieval call binding the contract method 0x95410d2b.
//
// Solidity: function authorizationRequired() view returns(bool)
func (_LightRelay *LightRelayCallerSession) AuthorizationRequired() (bool, error) {
	return _LightRelay.Contract.AuthorizationRequired(&_LightRelay.CallOpts)
}

// CurrentEpoch is a free data retrieval call binding the contract method 0x76671808.
//
// Solidity: function currentEpoch() view returns(uint64)
func (_LightRelay *LightRelayCaller) CurrentEpoch(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "currentEpoch")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// CurrentEpoch is a free data retrieval call binding the contract method 0x76671808.
//
// Solidity: function currentEpoch() view returns(uint64)
func (_LightRelay *LightRelaySession) CurrentEpoch() (uint64, error) {
	return _LightRelay.Contract.CurrentEpoch(&_LightRelay.CallOpts)
}

// CurrentEpoch is a free data retrieval call binding the contract method 0x76671808.
//
// Solidity: function currentEpoch() view returns(uint64)
func (_LightRelay *LightRelayCallerSession) CurrentEpoch() (uint64, error) {
	return _LightRelay.Contract.CurrentEpoch(&_LightRelay.CallOpts)
}

// GenesisEpoch is a free data retrieval call binding the contract method 0xb70e6be6.
//
// Solidity: function genesisEpoch() view returns(uint64)
func (_LightRelay *LightRelayCaller) GenesisEpoch(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "genesisEpoch")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// GenesisEpoch is a free data retrieval call binding the contract method 0xb70e6be6.
//
// Solidity: function genesisEpoch() view returns(uint64)
func (_LightRelay *LightRelaySession) GenesisEpoch() (uint64, error) {
	return _LightRelay.Contract.GenesisEpoch(&_LightRelay.CallOpts)
}

// GenesisEpoch is a free data retrieval call binding the contract method 0xb70e6be6.
//
// Solidity: function genesisEpoch() view returns(uint64)
func (_LightRelay *LightRelayCallerSession) GenesisEpoch() (uint64, error) {
	return _LightRelay.Contract.GenesisEpoch(&_LightRelay.CallOpts)
}

// GetBlockDifficulty is a free data retrieval call binding the contract method 0x06a27422.
//
// Solidity: function getBlockDifficulty(uint256 blockNumber) view returns(uint256)
func (_LightRelay *LightRelayCaller) GetBlockDifficulty(opts *bind.CallOpts, blockNumber *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "getBlockDifficulty", blockNumber)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetBlockDifficulty is a free data retrieval call binding the contract method 0x06a27422.
//
// Solidity: function getBlockDifficulty(uint256 blockNumber) view returns(uint256)
func (_LightRelay *LightRelaySession) GetBlockDifficulty(blockNumber *big.Int) (*big.Int, error) {
	return _LightRelay.Contract.GetBlockDifficulty(&_LightRelay.CallOpts, blockNumber)
}

// GetBlockDifficulty is a free data retrieval call binding the contract method 0x06a27422.
//
// Solidity: function getBlockDifficulty(uint256 blockNumber) view returns(uint256)
func (_LightRelay *LightRelayCallerSession) GetBlockDifficulty(blockNumber *big.Int) (*big.Int, error) {
	return _LightRelay.Contract.GetBlockDifficulty(&_LightRelay.CallOpts, blockNumber)
}

// GetCurrentAndPrevEpochDifficulty is a free data retrieval call binding the contract method 0x3a1b77b0.
//
// Solidity: function getCurrentAndPrevEpochDifficulty() view returns(uint256 current, uint256 previous)
func (_LightRelay *LightRelayCaller) GetCurrentAndPrevEpochDifficulty(opts *bind.CallOpts) (struct {
	Current  *big.Int
	Previous *big.Int
}, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "getCurrentAndPrevEpochDifficulty")

	outstruct := new(struct {
		Current  *big.Int
		Previous *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Current = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Previous = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetCurrentAndPrevEpochDifficulty is a free data retrieval call binding the contract method 0x3a1b77b0.
//
// Solidity: function getCurrentAndPrevEpochDifficulty() view returns(uint256 current, uint256 previous)
func (_LightRelay *LightRelaySession) GetCurrentAndPrevEpochDifficulty() (struct {
	Current  *big.Int
	Previous *big.Int
}, error) {
	return _LightRelay.Contract.GetCurrentAndPrevEpochDifficulty(&_LightRelay.CallOpts)
}

// GetCurrentAndPrevEpochDifficulty is a free data retrieval call binding the contract method 0x3a1b77b0.
//
// Solidity: function getCurrentAndPrevEpochDifficulty() view returns(uint256 current, uint256 previous)
func (_LightRelay *LightRelayCallerSession) GetCurrentAndPrevEpochDifficulty() (struct {
	Current  *big.Int
	Previous *big.Int
}, error) {
	return _LightRelay.Contract.GetCurrentAndPrevEpochDifficulty(&_LightRelay.CallOpts)
}

// GetCurrentEpochDifficulty is a free data retrieval call binding the contract method 0x113764be.
//
// Solidity: function getCurrentEpochDifficulty() view returns(uint256)
func (_LightRelay *LightRelayCaller) GetCurrentEpochDifficulty(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "getCurrentEpochDifficulty")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetCurrentEpochDifficulty is a free data retrieval call binding the contract method 0x113764be.
//
// Solidity: function getCurrentEpochDifficulty() view returns(uint256)
func (_LightRelay *LightRelaySession) GetCurrentEpochDifficulty() (*big.Int, error) {
	return _LightRelay.Contract.GetCurrentEpochDifficulty(&_LightRelay.CallOpts)
}

// GetCurrentEpochDifficulty is a free data retrieval call binding the contract method 0x113764be.
//
// Solidity: function getCurrentEpochDifficulty() view returns(uint256)
func (_LightRelay *LightRelayCallerSession) GetCurrentEpochDifficulty() (*big.Int, error) {
	return _LightRelay.Contract.GetCurrentEpochDifficulty(&_LightRelay.CallOpts)
}

// GetEpochDifficulty is a free data retrieval call binding the contract method 0x620414e6.
//
// Solidity: function getEpochDifficulty(uint256 epochNumber) view returns(uint256)
func (_LightRelay *LightRelayCaller) GetEpochDifficulty(opts *bind.CallOpts, epochNumber *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "getEpochDifficulty", epochNumber)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetEpochDifficulty is a free data retrieval call binding the contract method 0x620414e6.
//
// Solidity: function getEpochDifficulty(uint256 epochNumber) view returns(uint256)
func (_LightRelay *LightRelaySession) GetEpochDifficulty(epochNumber *big.Int) (*big.Int, error) {
	return _LightRelay.Contract.GetEpochDifficulty(&_LightRelay.CallOpts, epochNumber)
}

// GetEpochDifficulty is a free data retrieval call binding the contract method 0x620414e6.
//
// Solidity: function getEpochDifficulty(uint256 epochNumber) view returns(uint256)
func (_LightRelay *LightRelayCallerSession) GetEpochDifficulty(epochNumber *big.Int) (*big.Int, error) {
	return _LightRelay.Contract.GetEpochDifficulty(&_LightRelay.CallOpts, epochNumber)
}

// GetPrevEpochDifficulty is a free data retrieval call binding the contract method 0x2b97be24.
//
// Solidity: function getPrevEpochDifficulty() view returns(uint256)
func (_LightRelay *LightRelayCaller) GetPrevEpochDifficulty(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "getPrevEpochDifficulty")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetPrevEpochDifficulty is a free data retrieval call binding the contract method 0x2b97be24.
//
// Solidity: function getPrevEpochDifficulty() view returns(uint256)
func (_LightRelay *LightRelaySession) GetPrevEpochDifficulty() (*big.Int, error) {
	return _LightRelay.Contract.GetPrevEpochDifficulty(&_LightRelay.CallOpts)
}

// GetPrevEpochDifficulty is a free data retrieval call binding the contract method 0x2b97be24.
//
// Solidity: function getPrevEpochDifficulty() view returns(uint256)
func (_LightRelay *LightRelayCallerSession) GetPrevEpochDifficulty() (*big.Int, error) {
	return _LightRelay.Contract.GetPrevEpochDifficulty(&_LightRelay.CallOpts)
}

// GetRelayRange is a free data retrieval call binding the contract method 0x10b76ed8.
//
// Solidity: function getRelayRange() view returns(uint256 relayGenesis, uint256 currentEpochEnd)
func (_LightRelay *LightRelayCaller) GetRelayRange(opts *bind.CallOpts) (struct {
	RelayGenesis    *big.Int
	CurrentEpochEnd *big.Int
}, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "getRelayRange")

	outstruct := new(struct {
		RelayGenesis    *big.Int
		CurrentEpochEnd *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.RelayGenesis = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.CurrentEpochEnd = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetRelayRange is a free data retrieval call binding the contract method 0x10b76ed8.
//
// Solidity: function getRelayRange() view returns(uint256 relayGenesis, uint256 currentEpochEnd)
func (_LightRelay *LightRelaySession) GetRelayRange() (struct {
	RelayGenesis    *big.Int
	CurrentEpochEnd *big.Int
}, error) {
	return _LightRelay.Contract.GetRelayRange(&_LightRelay.CallOpts)
}

// GetRelayRange is a free data retrieval call binding the contract method 0x10b76ed8.
//
// Solidity: function getRelayRange() view returns(uint256 relayGenesis, uint256 currentEpochEnd)
func (_LightRelay *LightRelayCallerSession) GetRelayRange() (struct {
	RelayGenesis    *big.Int
	CurrentEpochEnd *big.Int
}, error) {
	return _LightRelay.Contract.GetRelayRange(&_LightRelay.CallOpts)
}

// IsAuthorized is a free data retrieval call binding the contract method 0xfe9fbb80.
//
// Solidity: function isAuthorized(address ) view returns(bool)
func (_LightRelay *LightRelayCaller) IsAuthorized(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "isAuthorized", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsAuthorized is a free data retrieval call binding the contract method 0xfe9fbb80.
//
// Solidity: function isAuthorized(address ) view returns(bool)
func (_LightRelay *LightRelaySession) IsAuthorized(arg0 common.Address) (bool, error) {
	return _LightRelay.Contract.IsAuthorized(&_LightRelay.CallOpts, arg0)
}

// IsAuthorized is a free data retrieval call binding the contract method 0xfe9fbb80.
//
// Solidity: function isAuthorized(address ) view returns(bool)
func (_LightRelay *LightRelayCallerSession) IsAuthorized(arg0 common.Address) (bool, error) {
	return _LightRelay.Contract.IsAuthorized(&_LightRelay.CallOpts, arg0)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LightRelay *LightRelayCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LightRelay *LightRelaySession) Owner() (common.Address, error) {
	return _LightRelay.Contract.Owner(&_LightRelay.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LightRelay *LightRelayCallerSession) Owner() (common.Address, error) {
	return _LightRelay.Contract.Owner(&_LightRelay.CallOpts)
}

// ProofLength is a free data retrieval call binding the contract method 0xf5619fda.
//
// Solidity: function proofLength() view returns(uint64)
func (_LightRelay *LightRelayCaller) ProofLength(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "proofLength")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// ProofLength is a free data retrieval call binding the contract method 0xf5619fda.
//
// Solidity: function proofLength() view returns(uint64)
func (_LightRelay *LightRelaySession) ProofLength() (uint64, error) {
	return _LightRelay.Contract.ProofLength(&_LightRelay.CallOpts)
}

// ProofLength is a free data retrieval call binding the contract method 0xf5619fda.
//
// Solidity: function proofLength() view returns(uint64)
func (_LightRelay *LightRelayCallerSession) ProofLength() (uint64, error) {
	return _LightRelay.Contract.ProofLength(&_LightRelay.CallOpts)
}

// Ready is a free data retrieval call binding the contract method 0x6defbf80.
//
// Solidity: function ready() view returns(bool)
func (_LightRelay *LightRelayCaller) Ready(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "ready")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Ready is a free data retrieval call binding the contract method 0x6defbf80.
//
// Solidity: function ready() view returns(bool)
func (_LightRelay *LightRelaySession) Ready() (bool, error) {
	return _LightRelay.Contract.Ready(&_LightRelay.CallOpts)
}

// Ready is a free data retrieval call binding the contract method 0x6defbf80.
//
// Solidity: function ready() view returns(bool)
func (_LightRelay *LightRelayCallerSession) Ready() (bool, error) {
	return _LightRelay.Contract.Ready(&_LightRelay.CallOpts)
}

// ValidateChain is a free data retrieval call binding the contract method 0x189179a3.
//
// Solidity: function validateChain(bytes headers) view returns(uint256 startingHeaderTimestamp, uint256 headerCount)
func (_LightRelay *LightRelayCaller) ValidateChain(opts *bind.CallOpts, headers []byte) (struct {
	StartingHeaderTimestamp *big.Int
	HeaderCount             *big.Int
}, error) {
	var out []interface{}
	err := _LightRelay.contract.Call(opts, &out, "validateChain", headers)

	outstruct := new(struct {
		StartingHeaderTimestamp *big.Int
		HeaderCount             *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.StartingHeaderTimestamp = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.HeaderCount = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// ValidateChain is a free data retrieval call binding the contract method 0x189179a3.
//
// Solidity: function validateChain(bytes headers) view returns(uint256 startingHeaderTimestamp, uint256 headerCount)
func (_LightRelay *LightRelaySession) ValidateChain(headers []byte) (struct {
	StartingHeaderTimestamp *big.Int
	HeaderCount             *big.Int
}, error) {
	return _LightRelay.Contract.ValidateChain(&_LightRelay.CallOpts, headers)
}

// ValidateChain is a free data retrieval call binding the contract method 0x189179a3.
//
// Solidity: function validateChain(bytes headers) view returns(uint256 startingHeaderTimestamp, uint256 headerCount)
func (_LightRelay *LightRelayCallerSession) ValidateChain(headers []byte) (struct {
	StartingHeaderTimestamp *big.Int
	HeaderCount             *big.Int
}, error) {
	return _LightRelay.Contract.ValidateChain(&_LightRelay.CallOpts, headers)
}

// Authorize is a paid mutator transaction binding the contract method 0xb6a5d7de.
//
// Solidity: function authorize(address submitter) returns()
func (_LightRelay *LightRelayTransactor) Authorize(opts *bind.TransactOpts, submitter common.Address) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "authorize", submitter)
}

// Authorize is a paid mutator transaction binding the contract method 0xb6a5d7de.
//
// Solidity: function authorize(address submitter) returns()
func (_LightRelay *LightRelaySession) Authorize(submitter common.Address) (*types.Transaction, error) {
	return _LightRelay.Contract.Authorize(&_LightRelay.TransactOpts, submitter)
}

// Authorize is a paid mutator transaction binding the contract method 0xb6a5d7de.
//
// Solidity: function authorize(address submitter) returns()
func (_LightRelay *LightRelayTransactorSession) Authorize(submitter common.Address) (*types.Transaction, error) {
	return _LightRelay.Contract.Authorize(&_LightRelay.TransactOpts, submitter)
}

// Deauthorize is a paid mutator transaction binding the contract method 0x27c97fa5.
//
// Solidity: function deauthorize(address submitter) returns()
func (_LightRelay *LightRelayTransactor) Deauthorize(opts *bind.TransactOpts, submitter common.Address) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "deauthorize", submitter)
}

// Deauthorize is a paid mutator transaction binding the contract method 0x27c97fa5.
//
// Solidity: function deauthorize(address submitter) returns()
func (_LightRelay *LightRelaySession) Deauthorize(submitter common.Address) (*types.Transaction, error) {
	return _LightRelay.Contract.Deauthorize(&_LightRelay.TransactOpts, submitter)
}

// Deauthorize is a paid mutator transaction binding the contract method 0x27c97fa5.
//
// Solidity: function deauthorize(address submitter) returns()
func (_LightRelay *LightRelayTransactorSession) Deauthorize(submitter common.Address) (*types.Transaction, error) {
	return _LightRelay.Contract.Deauthorize(&_LightRelay.TransactOpts, submitter)
}

// Genesis is a paid mutator transaction binding the contract method 0x4ca49f51.
//
// Solidity: function genesis(bytes genesisHeader, uint256 genesisHeight, uint64 genesisProofLength) returns()
func (_LightRelay *LightRelayTransactor) Genesis(opts *bind.TransactOpts, genesisHeader []byte, genesisHeight *big.Int, genesisProofLength uint64) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "genesis", genesisHeader, genesisHeight, genesisProofLength)
}

// Genesis is a paid mutator transaction binding the contract method 0x4ca49f51.
//
// Solidity: function genesis(bytes genesisHeader, uint256 genesisHeight, uint64 genesisProofLength) returns()
func (_LightRelay *LightRelaySession) Genesis(genesisHeader []byte, genesisHeight *big.Int, genesisProofLength uint64) (*types.Transaction, error) {
	return _LightRelay.Contract.Genesis(&_LightRelay.TransactOpts, genesisHeader, genesisHeight, genesisProofLength)
}

// Genesis is a paid mutator transaction binding the contract method 0x4ca49f51.
//
// Solidity: function genesis(bytes genesisHeader, uint256 genesisHeight, uint64 genesisProofLength) returns()
func (_LightRelay *LightRelayTransactorSession) Genesis(genesisHeader []byte, genesisHeight *big.Int, genesisProofLength uint64) (*types.Transaction, error) {
	return _LightRelay.Contract.Genesis(&_LightRelay.TransactOpts, genesisHeader, genesisHeight, genesisProofLength)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_LightRelay *LightRelayTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_LightRelay *LightRelaySession) RenounceOwnership() (*types.Transaction, error) {
	return _LightRelay.Contract.RenounceOwnership(&_LightRelay.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_LightRelay *LightRelayTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _LightRelay.Contract.RenounceOwnership(&_LightRelay.TransactOpts)
}

// Retarget is a paid mutator transaction binding the contract method 0x7ca5b1dd.
//
// Solidity: function retarget(bytes headers) returns()
func (_LightRelay *LightRelayTransactor) Retarget(opts *bind.TransactOpts, headers []byte) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "retarget", headers)
}

// Retarget is a paid mutator transaction binding the contract method 0x7ca5b1dd.
//
// Solidity: function retarget(bytes headers) returns()
func (_LightRelay *LightRelaySession) Retarget(headers []byte) (*types.Transaction, error) {
	return _LightRelay.Contract.Retarget(&_LightRelay.TransactOpts, headers)
}

// Retarget is a paid mutator transaction binding the contract method 0x7ca5b1dd.
//
// Solidity: function retarget(bytes headers) returns()
func (_LightRelay *LightRelayTransactorSession) Retarget(headers []byte) (*types.Transaction, error) {
	return _LightRelay.Contract.Retarget(&_LightRelay.TransactOpts, headers)
}

// SetAuthorizationStatus is a paid mutator transaction binding the contract method 0xeb8695ef.
//
// Solidity: function setAuthorizationStatus(bool status) returns()
func (_LightRelay *LightRelayTransactor) SetAuthorizationStatus(opts *bind.TransactOpts, status bool) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "setAuthorizationStatus", status)
}

// SetAuthorizationStatus is a paid mutator transaction binding the contract method 0xeb8695ef.
//
// Solidity: function setAuthorizationStatus(bool status) returns()
func (_LightRelay *LightRelaySession) SetAuthorizationStatus(status bool) (*types.Transaction, error) {
	return _LightRelay.Contract.SetAuthorizationStatus(&_LightRelay.TransactOpts, status)
}

// SetAuthorizationStatus is a paid mutator transaction binding the contract method 0xeb8695ef.
//
// Solidity: function setAuthorizationStatus(bool status) returns()
func (_LightRelay *LightRelayTransactorSession) SetAuthorizationStatus(status bool) (*types.Transaction, error) {
	return _LightRelay.Contract.SetAuthorizationStatus(&_LightRelay.TransactOpts, status)
}

// SetProofLength is a paid mutator transaction binding the contract method 0x19c9aa32.
//
// Solidity: function setProofLength(uint64 newLength) returns()
func (_LightRelay *LightRelayTransactor) SetProofLength(opts *bind.TransactOpts, newLength uint64) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "setProofLength", newLength)
}

// SetProofLength is a paid mutator transaction binding the contract method 0x19c9aa32.
//
// Solidity: function setProofLength(uint64 newLength) returns()
func (_LightRelay *LightRelaySession) SetProofLength(newLength uint64) (*types.Transaction, error) {
	return _LightRelay.Contract.SetProofLength(&_LightRelay.TransactOpts, newLength)
}

// SetProofLength is a paid mutator transaction binding the contract method 0x19c9aa32.
//
// Solidity: function setProofLength(uint64 newLength) returns()
func (_LightRelay *LightRelayTransactorSession) SetProofLength(newLength uint64) (*types.Transaction, error) {
	return _LightRelay.Contract.SetProofLength(&_LightRelay.TransactOpts, newLength)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LightRelay *LightRelayTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _LightRelay.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LightRelay *LightRelaySession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LightRelay.Contract.TransferOwnership(&_LightRelay.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LightRelay *LightRelayTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LightRelay.Contract.TransferOwnership(&_LightRelay.TransactOpts, newOwner)
}

// LightRelayAuthorizationRequirementChangedIterator is returned from FilterAuthorizationRequirementChanged and is used to iterate over the raw logs and unpacked data for AuthorizationRequirementChanged events raised by the LightRelay contract.
type LightRelayAuthorizationRequirementChangedIterator struct {
	Event *LightRelayAuthorizationRequirementChanged // Event containing the contract specifics and raw log

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
func (it *LightRelayAuthorizationRequirementChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayAuthorizationRequirementChanged)
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
		it.Event = new(LightRelayAuthorizationRequirementChanged)
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
func (it *LightRelayAuthorizationRequirementChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayAuthorizationRequirementChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayAuthorizationRequirementChanged represents a AuthorizationRequirementChanged event raised by the LightRelay contract.
type LightRelayAuthorizationRequirementChanged struct {
	NewStatus bool
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterAuthorizationRequirementChanged is a free log retrieval operation binding the contract event 0xd813b248d49c8bf08be2b6947126da6763df310beed7bea97756456c5727419a.
//
// Solidity: event AuthorizationRequirementChanged(bool newStatus)
func (_LightRelay *LightRelayFilterer) FilterAuthorizationRequirementChanged(opts *bind.FilterOpts) (*LightRelayAuthorizationRequirementChangedIterator, error) {

	logs, sub, err := _LightRelay.contract.FilterLogs(opts, "AuthorizationRequirementChanged")
	if err != nil {
		return nil, err
	}
	return &LightRelayAuthorizationRequirementChangedIterator{contract: _LightRelay.contract, event: "AuthorizationRequirementChanged", logs: logs, sub: sub}, nil
}

// WatchAuthorizationRequirementChanged is a free log subscription operation binding the contract event 0xd813b248d49c8bf08be2b6947126da6763df310beed7bea97756456c5727419a.
//
// Solidity: event AuthorizationRequirementChanged(bool newStatus)
func (_LightRelay *LightRelayFilterer) WatchAuthorizationRequirementChanged(opts *bind.WatchOpts, sink chan<- *LightRelayAuthorizationRequirementChanged) (event.Subscription, error) {

	logs, sub, err := _LightRelay.contract.WatchLogs(opts, "AuthorizationRequirementChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayAuthorizationRequirementChanged)
				if err := _LightRelay.contract.UnpackLog(event, "AuthorizationRequirementChanged", log); err != nil {
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

// ParseAuthorizationRequirementChanged is a log parse operation binding the contract event 0xd813b248d49c8bf08be2b6947126da6763df310beed7bea97756456c5727419a.
//
// Solidity: event AuthorizationRequirementChanged(bool newStatus)
func (_LightRelay *LightRelayFilterer) ParseAuthorizationRequirementChanged(log types.Log) (*LightRelayAuthorizationRequirementChanged, error) {
	event := new(LightRelayAuthorizationRequirementChanged)
	if err := _LightRelay.contract.UnpackLog(event, "AuthorizationRequirementChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayGenesisIterator is returned from FilterGenesis and is used to iterate over the raw logs and unpacked data for Genesis events raised by the LightRelay contract.
type LightRelayGenesisIterator struct {
	Event *LightRelayGenesis // Event containing the contract specifics and raw log

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
func (it *LightRelayGenesisIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayGenesis)
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
		it.Event = new(LightRelayGenesis)
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
func (it *LightRelayGenesisIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayGenesisIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayGenesis represents a Genesis event raised by the LightRelay contract.
type LightRelayGenesis struct {
	BlockHeight *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterGenesis is a free log retrieval operation binding the contract event 0x2381d16925551c2fb1a5edfcf4fce2f6d085e1f85f4b88340c09c9d191f9d4e9.
//
// Solidity: event Genesis(uint256 blockHeight)
func (_LightRelay *LightRelayFilterer) FilterGenesis(opts *bind.FilterOpts) (*LightRelayGenesisIterator, error) {

	logs, sub, err := _LightRelay.contract.FilterLogs(opts, "Genesis")
	if err != nil {
		return nil, err
	}
	return &LightRelayGenesisIterator{contract: _LightRelay.contract, event: "Genesis", logs: logs, sub: sub}, nil
}

// WatchGenesis is a free log subscription operation binding the contract event 0x2381d16925551c2fb1a5edfcf4fce2f6d085e1f85f4b88340c09c9d191f9d4e9.
//
// Solidity: event Genesis(uint256 blockHeight)
func (_LightRelay *LightRelayFilterer) WatchGenesis(opts *bind.WatchOpts, sink chan<- *LightRelayGenesis) (event.Subscription, error) {

	logs, sub, err := _LightRelay.contract.WatchLogs(opts, "Genesis")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayGenesis)
				if err := _LightRelay.contract.UnpackLog(event, "Genesis", log); err != nil {
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

// ParseGenesis is a log parse operation binding the contract event 0x2381d16925551c2fb1a5edfcf4fce2f6d085e1f85f4b88340c09c9d191f9d4e9.
//
// Solidity: event Genesis(uint256 blockHeight)
func (_LightRelay *LightRelayFilterer) ParseGenesis(log types.Log) (*LightRelayGenesis, error) {
	event := new(LightRelayGenesis)
	if err := _LightRelay.contract.UnpackLog(event, "Genesis", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the LightRelay contract.
type LightRelayOwnershipTransferredIterator struct {
	Event *LightRelayOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *LightRelayOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayOwnershipTransferred)
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
		it.Event = new(LightRelayOwnershipTransferred)
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
func (it *LightRelayOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayOwnershipTransferred represents a OwnershipTransferred event raised by the LightRelay contract.
type LightRelayOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_LightRelay *LightRelayFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*LightRelayOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _LightRelay.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &LightRelayOwnershipTransferredIterator{contract: _LightRelay.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_LightRelay *LightRelayFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *LightRelayOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _LightRelay.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayOwnershipTransferred)
				if err := _LightRelay.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_LightRelay *LightRelayFilterer) ParseOwnershipTransferred(log types.Log) (*LightRelayOwnershipTransferred, error) {
	event := new(LightRelayOwnershipTransferred)
	if err := _LightRelay.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayProofLengthChangedIterator is returned from FilterProofLengthChanged and is used to iterate over the raw logs and unpacked data for ProofLengthChanged events raised by the LightRelay contract.
type LightRelayProofLengthChangedIterator struct {
	Event *LightRelayProofLengthChanged // Event containing the contract specifics and raw log

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
func (it *LightRelayProofLengthChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayProofLengthChanged)
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
		it.Event = new(LightRelayProofLengthChanged)
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
func (it *LightRelayProofLengthChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayProofLengthChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayProofLengthChanged represents a ProofLengthChanged event raised by the LightRelay contract.
type LightRelayProofLengthChanged struct {
	NewLength *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterProofLengthChanged is a free log retrieval operation binding the contract event 0x3e9f904d8cf11753c79b67c8259c582056d4a7d8af120f81257a59eeb8824b96.
//
// Solidity: event ProofLengthChanged(uint256 newLength)
func (_LightRelay *LightRelayFilterer) FilterProofLengthChanged(opts *bind.FilterOpts) (*LightRelayProofLengthChangedIterator, error) {

	logs, sub, err := _LightRelay.contract.FilterLogs(opts, "ProofLengthChanged")
	if err != nil {
		return nil, err
	}
	return &LightRelayProofLengthChangedIterator{contract: _LightRelay.contract, event: "ProofLengthChanged", logs: logs, sub: sub}, nil
}

// WatchProofLengthChanged is a free log subscription operation binding the contract event 0x3e9f904d8cf11753c79b67c8259c582056d4a7d8af120f81257a59eeb8824b96.
//
// Solidity: event ProofLengthChanged(uint256 newLength)
func (_LightRelay *LightRelayFilterer) WatchProofLengthChanged(opts *bind.WatchOpts, sink chan<- *LightRelayProofLengthChanged) (event.Subscription, error) {

	logs, sub, err := _LightRelay.contract.WatchLogs(opts, "ProofLengthChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayProofLengthChanged)
				if err := _LightRelay.contract.UnpackLog(event, "ProofLengthChanged", log); err != nil {
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

// ParseProofLengthChanged is a log parse operation binding the contract event 0x3e9f904d8cf11753c79b67c8259c582056d4a7d8af120f81257a59eeb8824b96.
//
// Solidity: event ProofLengthChanged(uint256 newLength)
func (_LightRelay *LightRelayFilterer) ParseProofLengthChanged(log types.Log) (*LightRelayProofLengthChanged, error) {
	event := new(LightRelayProofLengthChanged)
	if err := _LightRelay.contract.UnpackLog(event, "ProofLengthChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelayRetargetIterator is returned from FilterRetarget and is used to iterate over the raw logs and unpacked data for Retarget events raised by the LightRelay contract.
type LightRelayRetargetIterator struct {
	Event *LightRelayRetarget // Event containing the contract specifics and raw log

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
func (it *LightRelayRetargetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelayRetarget)
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
		it.Event = new(LightRelayRetarget)
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
func (it *LightRelayRetargetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelayRetargetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelayRetarget represents a Retarget event raised by the LightRelay contract.
type LightRelayRetarget struct {
	OldDifficulty *big.Int
	NewDifficulty *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterRetarget is a free log retrieval operation binding the contract event 0xa282ee798b132f9dc11e06cd4d8e767e562be8709602ca14fea7ab3392acbdab.
//
// Solidity: event Retarget(uint256 oldDifficulty, uint256 newDifficulty)
func (_LightRelay *LightRelayFilterer) FilterRetarget(opts *bind.FilterOpts) (*LightRelayRetargetIterator, error) {

	logs, sub, err := _LightRelay.contract.FilterLogs(opts, "Retarget")
	if err != nil {
		return nil, err
	}
	return &LightRelayRetargetIterator{contract: _LightRelay.contract, event: "Retarget", logs: logs, sub: sub}, nil
}

// WatchRetarget is a free log subscription operation binding the contract event 0xa282ee798b132f9dc11e06cd4d8e767e562be8709602ca14fea7ab3392acbdab.
//
// Solidity: event Retarget(uint256 oldDifficulty, uint256 newDifficulty)
func (_LightRelay *LightRelayFilterer) WatchRetarget(opts *bind.WatchOpts, sink chan<- *LightRelayRetarget) (event.Subscription, error) {

	logs, sub, err := _LightRelay.contract.WatchLogs(opts, "Retarget")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelayRetarget)
				if err := _LightRelay.contract.UnpackLog(event, "Retarget", log); err != nil {
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

// ParseRetarget is a log parse operation binding the contract event 0xa282ee798b132f9dc11e06cd4d8e767e562be8709602ca14fea7ab3392acbdab.
//
// Solidity: event Retarget(uint256 oldDifficulty, uint256 newDifficulty)
func (_LightRelay *LightRelayFilterer) ParseRetarget(log types.Log) (*LightRelayRetarget, error) {
	event := new(LightRelayRetarget)
	if err := _LightRelay.contract.UnpackLog(event, "Retarget", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelaySubmitterAuthorizedIterator is returned from FilterSubmitterAuthorized and is used to iterate over the raw logs and unpacked data for SubmitterAuthorized events raised by the LightRelay contract.
type LightRelaySubmitterAuthorizedIterator struct {
	Event *LightRelaySubmitterAuthorized // Event containing the contract specifics and raw log

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
func (it *LightRelaySubmitterAuthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelaySubmitterAuthorized)
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
		it.Event = new(LightRelaySubmitterAuthorized)
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
func (it *LightRelaySubmitterAuthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelaySubmitterAuthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelaySubmitterAuthorized represents a SubmitterAuthorized event raised by the LightRelay contract.
type LightRelaySubmitterAuthorized struct {
	Submitter common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterSubmitterAuthorized is a free log retrieval operation binding the contract event 0xd53649b492f738bb59d6825099b5955073efda0bf9e3a7ad20da22e110122e29.
//
// Solidity: event SubmitterAuthorized(address submitter)
func (_LightRelay *LightRelayFilterer) FilterSubmitterAuthorized(opts *bind.FilterOpts) (*LightRelaySubmitterAuthorizedIterator, error) {

	logs, sub, err := _LightRelay.contract.FilterLogs(opts, "SubmitterAuthorized")
	if err != nil {
		return nil, err
	}
	return &LightRelaySubmitterAuthorizedIterator{contract: _LightRelay.contract, event: "SubmitterAuthorized", logs: logs, sub: sub}, nil
}

// WatchSubmitterAuthorized is a free log subscription operation binding the contract event 0xd53649b492f738bb59d6825099b5955073efda0bf9e3a7ad20da22e110122e29.
//
// Solidity: event SubmitterAuthorized(address submitter)
func (_LightRelay *LightRelayFilterer) WatchSubmitterAuthorized(opts *bind.WatchOpts, sink chan<- *LightRelaySubmitterAuthorized) (event.Subscription, error) {

	logs, sub, err := _LightRelay.contract.WatchLogs(opts, "SubmitterAuthorized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelaySubmitterAuthorized)
				if err := _LightRelay.contract.UnpackLog(event, "SubmitterAuthorized", log); err != nil {
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

// ParseSubmitterAuthorized is a log parse operation binding the contract event 0xd53649b492f738bb59d6825099b5955073efda0bf9e3a7ad20da22e110122e29.
//
// Solidity: event SubmitterAuthorized(address submitter)
func (_LightRelay *LightRelayFilterer) ParseSubmitterAuthorized(log types.Log) (*LightRelaySubmitterAuthorized, error) {
	event := new(LightRelaySubmitterAuthorized)
	if err := _LightRelay.contract.UnpackLog(event, "SubmitterAuthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LightRelaySubmitterDeauthorizedIterator is returned from FilterSubmitterDeauthorized and is used to iterate over the raw logs and unpacked data for SubmitterDeauthorized events raised by the LightRelay contract.
type LightRelaySubmitterDeauthorizedIterator struct {
	Event *LightRelaySubmitterDeauthorized // Event containing the contract specifics and raw log

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
func (it *LightRelaySubmitterDeauthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LightRelaySubmitterDeauthorized)
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
		it.Event = new(LightRelaySubmitterDeauthorized)
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
func (it *LightRelaySubmitterDeauthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LightRelaySubmitterDeauthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LightRelaySubmitterDeauthorized represents a SubmitterDeauthorized event raised by the LightRelay contract.
type LightRelaySubmitterDeauthorized struct {
	Submitter common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterSubmitterDeauthorized is a free log retrieval operation binding the contract event 0x7498b96beeabea5ad3139f1a2861a03e480034254e36b10aae2e6e42ad7b4b68.
//
// Solidity: event SubmitterDeauthorized(address submitter)
func (_LightRelay *LightRelayFilterer) FilterSubmitterDeauthorized(opts *bind.FilterOpts) (*LightRelaySubmitterDeauthorizedIterator, error) {

	logs, sub, err := _LightRelay.contract.FilterLogs(opts, "SubmitterDeauthorized")
	if err != nil {
		return nil, err
	}
	return &LightRelaySubmitterDeauthorizedIterator{contract: _LightRelay.contract, event: "SubmitterDeauthorized", logs: logs, sub: sub}, nil
}

// WatchSubmitterDeauthorized is a free log subscription operation binding the contract event 0x7498b96beeabea5ad3139f1a2861a03e480034254e36b10aae2e6e42ad7b4b68.
//
// Solidity: event SubmitterDeauthorized(address submitter)
func (_LightRelay *LightRelayFilterer) WatchSubmitterDeauthorized(opts *bind.WatchOpts, sink chan<- *LightRelaySubmitterDeauthorized) (event.Subscription, error) {

	logs, sub, err := _LightRelay.contract.WatchLogs(opts, "SubmitterDeauthorized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LightRelaySubmitterDeauthorized)
				if err := _LightRelay.contract.UnpackLog(event, "SubmitterDeauthorized", log); err != nil {
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

// ParseSubmitterDeauthorized is a log parse operation binding the contract event 0x7498b96beeabea5ad3139f1a2861a03e480034254e36b10aae2e6e42ad7b4b68.
//
// Solidity: event SubmitterDeauthorized(address submitter)
func (_LightRelay *LightRelayFilterer) ParseSubmitterDeauthorized(log types.Log) (*LightRelaySubmitterDeauthorized, error) {
	event := new(LightRelaySubmitterDeauthorized)
	if err := _LightRelay.contract.UnpackLog(event, "SubmitterDeauthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

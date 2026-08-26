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

// RedemptionWatchtowerMetaData contains all meta data concerning the RedemptionWatchtower contract.
var RedemptionWatchtowerMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"}],\"name\":\"Banned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"guardian\",\"type\":\"address\"}],\"name\":\"GuardianAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"guardian\",\"type\":\"address\"}],\"name\":\"GuardianRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"guardian\",\"type\":\"address\"}],\"name\":\"ObjectionRaised\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"}],\"name\":\"Unbanned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"}],\"name\":\"VetoFinalized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"}],\"name\":\"VetoPeriodCheckOmitted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"amount\",\"type\":\"uint64\"}],\"name\":\"VetoedFundsWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"disabledAt\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"executor\",\"type\":\"address\"}],\"name\":\"WatchtowerDisabled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"enabledAt\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"manager\",\"type\":\"address\"}],\"name\":\"WatchtowerEnabled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"watchtowerLifetime\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"vetoPenaltyFeeDivisor\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"vetoFreezePeriod\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"defaultDelay\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"levelOneDelay\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"levelTwoDelay\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"waivedAmountLimit\",\"type\":\"uint64\"}],\"name\":\"WatchtowerParametersUpdated\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"REQUIRED_OBJECTIONS_COUNT\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"guardian\",\"type\":\"address\"}],\"name\":\"addGuardian\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"bank\",\"outputs\":[{\"internalType\":\"contractBank\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"bridge\",\"outputs\":[{\"internalType\":\"contractBridge\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"defaultDelay\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"disableWatchtower\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_manager\",\"type\":\"address\"},{\"internalType\":\"address[]\",\"name\":\"_guardians\",\"type\":\"address[]\"}],\"name\":\"enableWatchtower\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"}],\"name\":\"getRedemptionDelay\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractBridge\",\"name\":\"_bridge\",\"type\":\"address\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isBanned\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isGuardian\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes\",\"name\":\"redeemerOutputScript\",\"type\":\"bytes\"},{\"internalType\":\"address\",\"name\":\"balanceOwner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"}],\"name\":\"isSafeRedemption\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"levelOneDelay\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"levelTwoDelay\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"objections\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes\",\"name\":\"redeemerOutputScript\",\"type\":\"bytes\"}],\"name\":\"raiseObjection\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"guardian\",\"type\":\"address\"}],\"name\":\"removeGuardian\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"}],\"name\":\"unban\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"_watchtowerLifetime\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"_vetoPenaltyFeeDivisor\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"_vetoFreezePeriod\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"_defaultDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"_levelOneDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"_levelTwoDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"_waivedAmountLimit\",\"type\":\"uint64\"}],\"name\":\"updateWatchtowerParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"vetoFreezePeriod\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"vetoPenaltyFeeDivisor\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"vetoProposals\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"withdrawableAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"finalizedAt\",\"type\":\"uint32\"},{\"internalType\":\"uint8\",\"name\":\"objectionsCount\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"waivedAmountLimit\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"watchtowerDisabledAt\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"watchtowerEnabledAt\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"watchtowerLifetime\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"}],\"name\":\"withdrawVetoedFunds\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// RedemptionWatchtowerABI is the input ABI used to generate the binding from.
// Deprecated: Use RedemptionWatchtowerMetaData.ABI instead.
var RedemptionWatchtowerABI = RedemptionWatchtowerMetaData.ABI

// RedemptionWatchtower is an auto generated Go binding around an Ethereum contract.
type RedemptionWatchtower struct {
	RedemptionWatchtowerCaller     // Read-only binding to the contract
	RedemptionWatchtowerTransactor // Write-only binding to the contract
	RedemptionWatchtowerFilterer   // Log filterer for contract events
}

// RedemptionWatchtowerCaller is an auto generated read-only Go binding around an Ethereum contract.
type RedemptionWatchtowerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RedemptionWatchtowerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RedemptionWatchtowerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RedemptionWatchtowerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RedemptionWatchtowerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RedemptionWatchtowerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RedemptionWatchtowerSession struct {
	Contract     *RedemptionWatchtower // Generic contract binding to set the session for
	CallOpts     bind.CallOpts         // Call options to use throughout this session
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// RedemptionWatchtowerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RedemptionWatchtowerCallerSession struct {
	Contract *RedemptionWatchtowerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts               // Call options to use throughout this session
}

// RedemptionWatchtowerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RedemptionWatchtowerTransactorSession struct {
	Contract     *RedemptionWatchtowerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts               // Transaction auth options to use throughout this session
}

// RedemptionWatchtowerRaw is an auto generated low-level Go binding around an Ethereum contract.
type RedemptionWatchtowerRaw struct {
	Contract *RedemptionWatchtower // Generic contract binding to access the raw methods on
}

// RedemptionWatchtowerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RedemptionWatchtowerCallerRaw struct {
	Contract *RedemptionWatchtowerCaller // Generic read-only contract binding to access the raw methods on
}

// RedemptionWatchtowerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RedemptionWatchtowerTransactorRaw struct {
	Contract *RedemptionWatchtowerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRedemptionWatchtower creates a new instance of RedemptionWatchtower, bound to a specific deployed contract.
func NewRedemptionWatchtower(address common.Address, backend bind.ContractBackend) (*RedemptionWatchtower, error) {
	contract, err := bindRedemptionWatchtower(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtower{RedemptionWatchtowerCaller: RedemptionWatchtowerCaller{contract: contract}, RedemptionWatchtowerTransactor: RedemptionWatchtowerTransactor{contract: contract}, RedemptionWatchtowerFilterer: RedemptionWatchtowerFilterer{contract: contract}}, nil
}

// NewRedemptionWatchtowerCaller creates a new read-only instance of RedemptionWatchtower, bound to a specific deployed contract.
func NewRedemptionWatchtowerCaller(address common.Address, caller bind.ContractCaller) (*RedemptionWatchtowerCaller, error) {
	contract, err := bindRedemptionWatchtower(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerCaller{contract: contract}, nil
}

// NewRedemptionWatchtowerTransactor creates a new write-only instance of RedemptionWatchtower, bound to a specific deployed contract.
func NewRedemptionWatchtowerTransactor(address common.Address, transactor bind.ContractTransactor) (*RedemptionWatchtowerTransactor, error) {
	contract, err := bindRedemptionWatchtower(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerTransactor{contract: contract}, nil
}

// NewRedemptionWatchtowerFilterer creates a new log filterer instance of RedemptionWatchtower, bound to a specific deployed contract.
func NewRedemptionWatchtowerFilterer(address common.Address, filterer bind.ContractFilterer) (*RedemptionWatchtowerFilterer, error) {
	contract, err := bindRedemptionWatchtower(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerFilterer{contract: contract}, nil
}

// bindRedemptionWatchtower binds a generic wrapper to an already deployed contract.
func bindRedemptionWatchtower(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := RedemptionWatchtowerMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RedemptionWatchtower *RedemptionWatchtowerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RedemptionWatchtower.Contract.RedemptionWatchtowerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RedemptionWatchtower *RedemptionWatchtowerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RedemptionWatchtowerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RedemptionWatchtower *RedemptionWatchtowerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RedemptionWatchtowerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RedemptionWatchtower *RedemptionWatchtowerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RedemptionWatchtower.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.contract.Transact(opts, method, params...)
}

// REQUIREDOBJECTIONSCOUNT is a free data retrieval call binding the contract method 0x7a497647.
//
// Solidity: function REQUIRED_OBJECTIONS_COUNT() view returns(uint8)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) REQUIREDOBJECTIONSCOUNT(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "REQUIRED_OBJECTIONS_COUNT")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// REQUIREDOBJECTIONSCOUNT is a free data retrieval call binding the contract method 0x7a497647.
//
// Solidity: function REQUIRED_OBJECTIONS_COUNT() view returns(uint8)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) REQUIREDOBJECTIONSCOUNT() (uint8, error) {
	return _RedemptionWatchtower.Contract.REQUIREDOBJECTIONSCOUNT(&_RedemptionWatchtower.CallOpts)
}

// REQUIREDOBJECTIONSCOUNT is a free data retrieval call binding the contract method 0x7a497647.
//
// Solidity: function REQUIRED_OBJECTIONS_COUNT() view returns(uint8)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) REQUIREDOBJECTIONSCOUNT() (uint8, error) {
	return _RedemptionWatchtower.Contract.REQUIREDOBJECTIONSCOUNT(&_RedemptionWatchtower.CallOpts)
}

// Bank is a free data retrieval call binding the contract method 0x76cdb03b.
//
// Solidity: function bank() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) Bank(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "bank")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Bank is a free data retrieval call binding the contract method 0x76cdb03b.
//
// Solidity: function bank() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) Bank() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Bank(&_RedemptionWatchtower.CallOpts)
}

// Bank is a free data retrieval call binding the contract method 0x76cdb03b.
//
// Solidity: function bank() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) Bank() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Bank(&_RedemptionWatchtower.CallOpts)
}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) Bridge(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "bridge")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) Bridge() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Bridge(&_RedemptionWatchtower.CallOpts)
}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) Bridge() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Bridge(&_RedemptionWatchtower.CallOpts)
}

// DefaultDelay is a free data retrieval call binding the contract method 0x27e72e41.
//
// Solidity: function defaultDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) DefaultDelay(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "defaultDelay")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// DefaultDelay is a free data retrieval call binding the contract method 0x27e72e41.
//
// Solidity: function defaultDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) DefaultDelay() (uint32, error) {
	return _RedemptionWatchtower.Contract.DefaultDelay(&_RedemptionWatchtower.CallOpts)
}

// DefaultDelay is a free data retrieval call binding the contract method 0x27e72e41.
//
// Solidity: function defaultDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) DefaultDelay() (uint32, error) {
	return _RedemptionWatchtower.Contract.DefaultDelay(&_RedemptionWatchtower.CallOpts)
}

// GetRedemptionDelay is a free data retrieval call binding the contract method 0x0df5db70.
//
// Solidity: function getRedemptionDelay(uint256 redemptionKey) view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) GetRedemptionDelay(opts *bind.CallOpts, redemptionKey *big.Int) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "getRedemptionDelay", redemptionKey)

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// GetRedemptionDelay is a free data retrieval call binding the contract method 0x0df5db70.
//
// Solidity: function getRedemptionDelay(uint256 redemptionKey) view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) GetRedemptionDelay(redemptionKey *big.Int) (uint32, error) {
	return _RedemptionWatchtower.Contract.GetRedemptionDelay(&_RedemptionWatchtower.CallOpts, redemptionKey)
}

// GetRedemptionDelay is a free data retrieval call binding the contract method 0x0df5db70.
//
// Solidity: function getRedemptionDelay(uint256 redemptionKey) view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) GetRedemptionDelay(redemptionKey *big.Int) (uint32, error) {
	return _RedemptionWatchtower.Contract.GetRedemptionDelay(&_RedemptionWatchtower.CallOpts, redemptionKey)
}

// IsBanned is a free data retrieval call binding the contract method 0x97f735d5.
//
// Solidity: function isBanned(address ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) IsBanned(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "isBanned", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsBanned is a free data retrieval call binding the contract method 0x97f735d5.
//
// Solidity: function isBanned(address ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) IsBanned(arg0 common.Address) (bool, error) {
	return _RedemptionWatchtower.Contract.IsBanned(&_RedemptionWatchtower.CallOpts, arg0)
}

// IsBanned is a free data retrieval call binding the contract method 0x97f735d5.
//
// Solidity: function isBanned(address ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) IsBanned(arg0 common.Address) (bool, error) {
	return _RedemptionWatchtower.Contract.IsBanned(&_RedemptionWatchtower.CallOpts, arg0)
}

// IsGuardian is a free data retrieval call binding the contract method 0x0c68ba21.
//
// Solidity: function isGuardian(address ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) IsGuardian(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "isGuardian", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsGuardian is a free data retrieval call binding the contract method 0x0c68ba21.
//
// Solidity: function isGuardian(address ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) IsGuardian(arg0 common.Address) (bool, error) {
	return _RedemptionWatchtower.Contract.IsGuardian(&_RedemptionWatchtower.CallOpts, arg0)
}

// IsGuardian is a free data retrieval call binding the contract method 0x0c68ba21.
//
// Solidity: function isGuardian(address ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) IsGuardian(arg0 common.Address) (bool, error) {
	return _RedemptionWatchtower.Contract.IsGuardian(&_RedemptionWatchtower.CallOpts, arg0)
}

// IsSafeRedemption is a free data retrieval call binding the contract method 0x4eadf73d.
//
// Solidity: function isSafeRedemption(bytes20 walletPubKeyHash, bytes redeemerOutputScript, address balanceOwner, address redeemer) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) IsSafeRedemption(opts *bind.CallOpts, walletPubKeyHash [20]byte, redeemerOutputScript []byte, balanceOwner common.Address, redeemer common.Address) (bool, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "isSafeRedemption", walletPubKeyHash, redeemerOutputScript, balanceOwner, redeemer)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsSafeRedemption is a free data retrieval call binding the contract method 0x4eadf73d.
//
// Solidity: function isSafeRedemption(bytes20 walletPubKeyHash, bytes redeemerOutputScript, address balanceOwner, address redeemer) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) IsSafeRedemption(walletPubKeyHash [20]byte, redeemerOutputScript []byte, balanceOwner common.Address, redeemer common.Address) (bool, error) {
	return _RedemptionWatchtower.Contract.IsSafeRedemption(&_RedemptionWatchtower.CallOpts, walletPubKeyHash, redeemerOutputScript, balanceOwner, redeemer)
}

// IsSafeRedemption is a free data retrieval call binding the contract method 0x4eadf73d.
//
// Solidity: function isSafeRedemption(bytes20 walletPubKeyHash, bytes redeemerOutputScript, address balanceOwner, address redeemer) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) IsSafeRedemption(walletPubKeyHash [20]byte, redeemerOutputScript []byte, balanceOwner common.Address, redeemer common.Address) (bool, error) {
	return _RedemptionWatchtower.Contract.IsSafeRedemption(&_RedemptionWatchtower.CallOpts, walletPubKeyHash, redeemerOutputScript, balanceOwner, redeemer)
}

// LevelOneDelay is a free data retrieval call binding the contract method 0x7825ef49.
//
// Solidity: function levelOneDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) LevelOneDelay(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "levelOneDelay")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// LevelOneDelay is a free data retrieval call binding the contract method 0x7825ef49.
//
// Solidity: function levelOneDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) LevelOneDelay() (uint32, error) {
	return _RedemptionWatchtower.Contract.LevelOneDelay(&_RedemptionWatchtower.CallOpts)
}

// LevelOneDelay is a free data retrieval call binding the contract method 0x7825ef49.
//
// Solidity: function levelOneDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) LevelOneDelay() (uint32, error) {
	return _RedemptionWatchtower.Contract.LevelOneDelay(&_RedemptionWatchtower.CallOpts)
}

// LevelTwoDelay is a free data retrieval call binding the contract method 0x988880d7.
//
// Solidity: function levelTwoDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) LevelTwoDelay(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "levelTwoDelay")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// LevelTwoDelay is a free data retrieval call binding the contract method 0x988880d7.
//
// Solidity: function levelTwoDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) LevelTwoDelay() (uint32, error) {
	return _RedemptionWatchtower.Contract.LevelTwoDelay(&_RedemptionWatchtower.CallOpts)
}

// LevelTwoDelay is a free data retrieval call binding the contract method 0x988880d7.
//
// Solidity: function levelTwoDelay() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) LevelTwoDelay() (uint32, error) {
	return _RedemptionWatchtower.Contract.LevelTwoDelay(&_RedemptionWatchtower.CallOpts)
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) Manager(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "manager")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) Manager() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Manager(&_RedemptionWatchtower.CallOpts)
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) Manager() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Manager(&_RedemptionWatchtower.CallOpts)
}

// Objections is a free data retrieval call binding the contract method 0xcc639fab.
//
// Solidity: function objections(uint256 ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) Objections(opts *bind.CallOpts, arg0 *big.Int) (bool, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "objections", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Objections is a free data retrieval call binding the contract method 0xcc639fab.
//
// Solidity: function objections(uint256 ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) Objections(arg0 *big.Int) (bool, error) {
	return _RedemptionWatchtower.Contract.Objections(&_RedemptionWatchtower.CallOpts, arg0)
}

// Objections is a free data retrieval call binding the contract method 0xcc639fab.
//
// Solidity: function objections(uint256 ) view returns(bool)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) Objections(arg0 *big.Int) (bool, error) {
	return _RedemptionWatchtower.Contract.Objections(&_RedemptionWatchtower.CallOpts, arg0)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) Owner() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Owner(&_RedemptionWatchtower.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) Owner() (common.Address, error) {
	return _RedemptionWatchtower.Contract.Owner(&_RedemptionWatchtower.CallOpts)
}

// VetoFreezePeriod is a free data retrieval call binding the contract method 0x33597014.
//
// Solidity: function vetoFreezePeriod() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) VetoFreezePeriod(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "vetoFreezePeriod")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// VetoFreezePeriod is a free data retrieval call binding the contract method 0x33597014.
//
// Solidity: function vetoFreezePeriod() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) VetoFreezePeriod() (uint32, error) {
	return _RedemptionWatchtower.Contract.VetoFreezePeriod(&_RedemptionWatchtower.CallOpts)
}

// VetoFreezePeriod is a free data retrieval call binding the contract method 0x33597014.
//
// Solidity: function vetoFreezePeriod() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) VetoFreezePeriod() (uint32, error) {
	return _RedemptionWatchtower.Contract.VetoFreezePeriod(&_RedemptionWatchtower.CallOpts)
}

// VetoPenaltyFeeDivisor is a free data retrieval call binding the contract method 0xbbaa5dbe.
//
// Solidity: function vetoPenaltyFeeDivisor() view returns(uint64)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) VetoPenaltyFeeDivisor(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "vetoPenaltyFeeDivisor")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// VetoPenaltyFeeDivisor is a free data retrieval call binding the contract method 0xbbaa5dbe.
//
// Solidity: function vetoPenaltyFeeDivisor() view returns(uint64)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) VetoPenaltyFeeDivisor() (uint64, error) {
	return _RedemptionWatchtower.Contract.VetoPenaltyFeeDivisor(&_RedemptionWatchtower.CallOpts)
}

// VetoPenaltyFeeDivisor is a free data retrieval call binding the contract method 0xbbaa5dbe.
//
// Solidity: function vetoPenaltyFeeDivisor() view returns(uint64)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) VetoPenaltyFeeDivisor() (uint64, error) {
	return _RedemptionWatchtower.Contract.VetoPenaltyFeeDivisor(&_RedemptionWatchtower.CallOpts)
}

// VetoProposals is a free data retrieval call binding the contract method 0xfac8da1b.
//
// Solidity: function vetoProposals(uint256 ) view returns(address redeemer, uint64 withdrawableAmount, uint32 finalizedAt, uint8 objectionsCount)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) VetoProposals(opts *bind.CallOpts, arg0 *big.Int) (struct {
	Redeemer           common.Address
	WithdrawableAmount uint64
	FinalizedAt        uint32
	ObjectionsCount    uint8
}, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "vetoProposals", arg0)

	outstruct := new(struct {
		Redeemer           common.Address
		WithdrawableAmount uint64
		FinalizedAt        uint32
		ObjectionsCount    uint8
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Redeemer = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.WithdrawableAmount = *abi.ConvertType(out[1], new(uint64)).(*uint64)
	outstruct.FinalizedAt = *abi.ConvertType(out[2], new(uint32)).(*uint32)
	outstruct.ObjectionsCount = *abi.ConvertType(out[3], new(uint8)).(*uint8)

	return *outstruct, err

}

// VetoProposals is a free data retrieval call binding the contract method 0xfac8da1b.
//
// Solidity: function vetoProposals(uint256 ) view returns(address redeemer, uint64 withdrawableAmount, uint32 finalizedAt, uint8 objectionsCount)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) VetoProposals(arg0 *big.Int) (struct {
	Redeemer           common.Address
	WithdrawableAmount uint64
	FinalizedAt        uint32
	ObjectionsCount    uint8
}, error) {
	return _RedemptionWatchtower.Contract.VetoProposals(&_RedemptionWatchtower.CallOpts, arg0)
}

// VetoProposals is a free data retrieval call binding the contract method 0xfac8da1b.
//
// Solidity: function vetoProposals(uint256 ) view returns(address redeemer, uint64 withdrawableAmount, uint32 finalizedAt, uint8 objectionsCount)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) VetoProposals(arg0 *big.Int) (struct {
	Redeemer           common.Address
	WithdrawableAmount uint64
	FinalizedAt        uint32
	ObjectionsCount    uint8
}, error) {
	return _RedemptionWatchtower.Contract.VetoProposals(&_RedemptionWatchtower.CallOpts, arg0)
}

// WaivedAmountLimit is a free data retrieval call binding the contract method 0x0a505427.
//
// Solidity: function waivedAmountLimit() view returns(uint64)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) WaivedAmountLimit(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "waivedAmountLimit")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// WaivedAmountLimit is a free data retrieval call binding the contract method 0x0a505427.
//
// Solidity: function waivedAmountLimit() view returns(uint64)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) WaivedAmountLimit() (uint64, error) {
	return _RedemptionWatchtower.Contract.WaivedAmountLimit(&_RedemptionWatchtower.CallOpts)
}

// WaivedAmountLimit is a free data retrieval call binding the contract method 0x0a505427.
//
// Solidity: function waivedAmountLimit() view returns(uint64)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) WaivedAmountLimit() (uint64, error) {
	return _RedemptionWatchtower.Contract.WaivedAmountLimit(&_RedemptionWatchtower.CallOpts)
}

// WatchtowerDisabledAt is a free data retrieval call binding the contract method 0x16cacdbe.
//
// Solidity: function watchtowerDisabledAt() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) WatchtowerDisabledAt(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "watchtowerDisabledAt")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// WatchtowerDisabledAt is a free data retrieval call binding the contract method 0x16cacdbe.
//
// Solidity: function watchtowerDisabledAt() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) WatchtowerDisabledAt() (uint32, error) {
	return _RedemptionWatchtower.Contract.WatchtowerDisabledAt(&_RedemptionWatchtower.CallOpts)
}

// WatchtowerDisabledAt is a free data retrieval call binding the contract method 0x16cacdbe.
//
// Solidity: function watchtowerDisabledAt() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) WatchtowerDisabledAt() (uint32, error) {
	return _RedemptionWatchtower.Contract.WatchtowerDisabledAt(&_RedemptionWatchtower.CallOpts)
}

// WatchtowerEnabledAt is a free data retrieval call binding the contract method 0x07a5af43.
//
// Solidity: function watchtowerEnabledAt() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) WatchtowerEnabledAt(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "watchtowerEnabledAt")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// WatchtowerEnabledAt is a free data retrieval call binding the contract method 0x07a5af43.
//
// Solidity: function watchtowerEnabledAt() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) WatchtowerEnabledAt() (uint32, error) {
	return _RedemptionWatchtower.Contract.WatchtowerEnabledAt(&_RedemptionWatchtower.CallOpts)
}

// WatchtowerEnabledAt is a free data retrieval call binding the contract method 0x07a5af43.
//
// Solidity: function watchtowerEnabledAt() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) WatchtowerEnabledAt() (uint32, error) {
	return _RedemptionWatchtower.Contract.WatchtowerEnabledAt(&_RedemptionWatchtower.CallOpts)
}

// WatchtowerLifetime is a free data retrieval call binding the contract method 0xedb0b4f9.
//
// Solidity: function watchtowerLifetime() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCaller) WatchtowerLifetime(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _RedemptionWatchtower.contract.Call(opts, &out, "watchtowerLifetime")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// WatchtowerLifetime is a free data retrieval call binding the contract method 0xedb0b4f9.
//
// Solidity: function watchtowerLifetime() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerSession) WatchtowerLifetime() (uint32, error) {
	return _RedemptionWatchtower.Contract.WatchtowerLifetime(&_RedemptionWatchtower.CallOpts)
}

// WatchtowerLifetime is a free data retrieval call binding the contract method 0xedb0b4f9.
//
// Solidity: function watchtowerLifetime() view returns(uint32)
func (_RedemptionWatchtower *RedemptionWatchtowerCallerSession) WatchtowerLifetime() (uint32, error) {
	return _RedemptionWatchtower.Contract.WatchtowerLifetime(&_RedemptionWatchtower.CallOpts)
}

// AddGuardian is a paid mutator transaction binding the contract method 0xa526d83b.
//
// Solidity: function addGuardian(address guardian) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) AddGuardian(opts *bind.TransactOpts, guardian common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "addGuardian", guardian)
}

// AddGuardian is a paid mutator transaction binding the contract method 0xa526d83b.
//
// Solidity: function addGuardian(address guardian) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) AddGuardian(guardian common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.AddGuardian(&_RedemptionWatchtower.TransactOpts, guardian)
}

// AddGuardian is a paid mutator transaction binding the contract method 0xa526d83b.
//
// Solidity: function addGuardian(address guardian) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) AddGuardian(guardian common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.AddGuardian(&_RedemptionWatchtower.TransactOpts, guardian)
}

// DisableWatchtower is a paid mutator transaction binding the contract method 0xbe773143.
//
// Solidity: function disableWatchtower() returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) DisableWatchtower(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "disableWatchtower")
}

// DisableWatchtower is a paid mutator transaction binding the contract method 0xbe773143.
//
// Solidity: function disableWatchtower() returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) DisableWatchtower() (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.DisableWatchtower(&_RedemptionWatchtower.TransactOpts)
}

// DisableWatchtower is a paid mutator transaction binding the contract method 0xbe773143.
//
// Solidity: function disableWatchtower() returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) DisableWatchtower() (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.DisableWatchtower(&_RedemptionWatchtower.TransactOpts)
}

// EnableWatchtower is a paid mutator transaction binding the contract method 0x344d294d.
//
// Solidity: function enableWatchtower(address _manager, address[] _guardians) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) EnableWatchtower(opts *bind.TransactOpts, _manager common.Address, _guardians []common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "enableWatchtower", _manager, _guardians)
}

// EnableWatchtower is a paid mutator transaction binding the contract method 0x344d294d.
//
// Solidity: function enableWatchtower(address _manager, address[] _guardians) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) EnableWatchtower(_manager common.Address, _guardians []common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.EnableWatchtower(&_RedemptionWatchtower.TransactOpts, _manager, _guardians)
}

// EnableWatchtower is a paid mutator transaction binding the contract method 0x344d294d.
//
// Solidity: function enableWatchtower(address _manager, address[] _guardians) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) EnableWatchtower(_manager common.Address, _guardians []common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.EnableWatchtower(&_RedemptionWatchtower.TransactOpts, _manager, _guardians)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _bridge) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) Initialize(opts *bind.TransactOpts, _bridge common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "initialize", _bridge)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _bridge) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) Initialize(_bridge common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.Initialize(&_RedemptionWatchtower.TransactOpts, _bridge)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _bridge) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) Initialize(_bridge common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.Initialize(&_RedemptionWatchtower.TransactOpts, _bridge)
}

// RaiseObjection is a paid mutator transaction binding the contract method 0xc4bb3785.
//
// Solidity: function raiseObjection(bytes20 walletPubKeyHash, bytes redeemerOutputScript) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) RaiseObjection(opts *bind.TransactOpts, walletPubKeyHash [20]byte, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "raiseObjection", walletPubKeyHash, redeemerOutputScript)
}

// RaiseObjection is a paid mutator transaction binding the contract method 0xc4bb3785.
//
// Solidity: function raiseObjection(bytes20 walletPubKeyHash, bytes redeemerOutputScript) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) RaiseObjection(walletPubKeyHash [20]byte, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RaiseObjection(&_RedemptionWatchtower.TransactOpts, walletPubKeyHash, redeemerOutputScript)
}

// RaiseObjection is a paid mutator transaction binding the contract method 0xc4bb3785.
//
// Solidity: function raiseObjection(bytes20 walletPubKeyHash, bytes redeemerOutputScript) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) RaiseObjection(walletPubKeyHash [20]byte, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RaiseObjection(&_RedemptionWatchtower.TransactOpts, walletPubKeyHash, redeemerOutputScript)
}

// RemoveGuardian is a paid mutator transaction binding the contract method 0x71404156.
//
// Solidity: function removeGuardian(address guardian) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) RemoveGuardian(opts *bind.TransactOpts, guardian common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "removeGuardian", guardian)
}

// RemoveGuardian is a paid mutator transaction binding the contract method 0x71404156.
//
// Solidity: function removeGuardian(address guardian) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) RemoveGuardian(guardian common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RemoveGuardian(&_RedemptionWatchtower.TransactOpts, guardian)
}

// RemoveGuardian is a paid mutator transaction binding the contract method 0x71404156.
//
// Solidity: function removeGuardian(address guardian) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) RemoveGuardian(guardian common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RemoveGuardian(&_RedemptionWatchtower.TransactOpts, guardian)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) RenounceOwnership() (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RenounceOwnership(&_RedemptionWatchtower.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.RenounceOwnership(&_RedemptionWatchtower.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.TransferOwnership(&_RedemptionWatchtower.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.TransferOwnership(&_RedemptionWatchtower.TransactOpts, newOwner)
}

// Unban is a paid mutator transaction binding the contract method 0xb9f14557.
//
// Solidity: function unban(address redeemer) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) Unban(opts *bind.TransactOpts, redeemer common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "unban", redeemer)
}

// Unban is a paid mutator transaction binding the contract method 0xb9f14557.
//
// Solidity: function unban(address redeemer) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) Unban(redeemer common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.Unban(&_RedemptionWatchtower.TransactOpts, redeemer)
}

// Unban is a paid mutator transaction binding the contract method 0xb9f14557.
//
// Solidity: function unban(address redeemer) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) Unban(redeemer common.Address) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.Unban(&_RedemptionWatchtower.TransactOpts, redeemer)
}

// UpdateWatchtowerParameters is a paid mutator transaction binding the contract method 0x2b98ab23.
//
// Solidity: function updateWatchtowerParameters(uint32 _watchtowerLifetime, uint64 _vetoPenaltyFeeDivisor, uint32 _vetoFreezePeriod, uint32 _defaultDelay, uint32 _levelOneDelay, uint32 _levelTwoDelay, uint64 _waivedAmountLimit) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) UpdateWatchtowerParameters(opts *bind.TransactOpts, _watchtowerLifetime uint32, _vetoPenaltyFeeDivisor uint64, _vetoFreezePeriod uint32, _defaultDelay uint32, _levelOneDelay uint32, _levelTwoDelay uint32, _waivedAmountLimit uint64) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "updateWatchtowerParameters", _watchtowerLifetime, _vetoPenaltyFeeDivisor, _vetoFreezePeriod, _defaultDelay, _levelOneDelay, _levelTwoDelay, _waivedAmountLimit)
}

// UpdateWatchtowerParameters is a paid mutator transaction binding the contract method 0x2b98ab23.
//
// Solidity: function updateWatchtowerParameters(uint32 _watchtowerLifetime, uint64 _vetoPenaltyFeeDivisor, uint32 _vetoFreezePeriod, uint32 _defaultDelay, uint32 _levelOneDelay, uint32 _levelTwoDelay, uint64 _waivedAmountLimit) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) UpdateWatchtowerParameters(_watchtowerLifetime uint32, _vetoPenaltyFeeDivisor uint64, _vetoFreezePeriod uint32, _defaultDelay uint32, _levelOneDelay uint32, _levelTwoDelay uint32, _waivedAmountLimit uint64) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.UpdateWatchtowerParameters(&_RedemptionWatchtower.TransactOpts, _watchtowerLifetime, _vetoPenaltyFeeDivisor, _vetoFreezePeriod, _defaultDelay, _levelOneDelay, _levelTwoDelay, _waivedAmountLimit)
}

// UpdateWatchtowerParameters is a paid mutator transaction binding the contract method 0x2b98ab23.
//
// Solidity: function updateWatchtowerParameters(uint32 _watchtowerLifetime, uint64 _vetoPenaltyFeeDivisor, uint32 _vetoFreezePeriod, uint32 _defaultDelay, uint32 _levelOneDelay, uint32 _levelTwoDelay, uint64 _waivedAmountLimit) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) UpdateWatchtowerParameters(_watchtowerLifetime uint32, _vetoPenaltyFeeDivisor uint64, _vetoFreezePeriod uint32, _defaultDelay uint32, _levelOneDelay uint32, _levelTwoDelay uint32, _waivedAmountLimit uint64) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.UpdateWatchtowerParameters(&_RedemptionWatchtower.TransactOpts, _watchtowerLifetime, _vetoPenaltyFeeDivisor, _vetoFreezePeriod, _defaultDelay, _levelOneDelay, _levelTwoDelay, _waivedAmountLimit)
}

// WithdrawVetoedFunds is a paid mutator transaction binding the contract method 0x126f6e9f.
//
// Solidity: function withdrawVetoedFunds(uint256 redemptionKey) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactor) WithdrawVetoedFunds(opts *bind.TransactOpts, redemptionKey *big.Int) (*types.Transaction, error) {
	return _RedemptionWatchtower.contract.Transact(opts, "withdrawVetoedFunds", redemptionKey)
}

// WithdrawVetoedFunds is a paid mutator transaction binding the contract method 0x126f6e9f.
//
// Solidity: function withdrawVetoedFunds(uint256 redemptionKey) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerSession) WithdrawVetoedFunds(redemptionKey *big.Int) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.WithdrawVetoedFunds(&_RedemptionWatchtower.TransactOpts, redemptionKey)
}

// WithdrawVetoedFunds is a paid mutator transaction binding the contract method 0x126f6e9f.
//
// Solidity: function withdrawVetoedFunds(uint256 redemptionKey) returns()
func (_RedemptionWatchtower *RedemptionWatchtowerTransactorSession) WithdrawVetoedFunds(redemptionKey *big.Int) (*types.Transaction, error) {
	return _RedemptionWatchtower.Contract.WithdrawVetoedFunds(&_RedemptionWatchtower.TransactOpts, redemptionKey)
}

// RedemptionWatchtowerBannedIterator is returned from FilterBanned and is used to iterate over the raw logs and unpacked data for Banned events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerBannedIterator struct {
	Event *RedemptionWatchtowerBanned // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerBannedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerBanned)
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
		it.Event = new(RedemptionWatchtowerBanned)
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
func (it *RedemptionWatchtowerBannedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerBannedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerBanned represents a Banned event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerBanned struct {
	Redeemer common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterBanned is a free log retrieval operation binding the contract event 0x30d1df1214d91553408ca5384ce29e10e5866af8423c628be22860e41fb81005.
//
// Solidity: event Banned(address indexed redeemer)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterBanned(opts *bind.FilterOpts, redeemer []common.Address) (*RedemptionWatchtowerBannedIterator, error) {

	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "Banned", redeemerRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerBannedIterator{contract: _RedemptionWatchtower.contract, event: "Banned", logs: logs, sub: sub}, nil
}

// WatchBanned is a free log subscription operation binding the contract event 0x30d1df1214d91553408ca5384ce29e10e5866af8423c628be22860e41fb81005.
//
// Solidity: event Banned(address indexed redeemer)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchBanned(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerBanned, redeemer []common.Address) (event.Subscription, error) {

	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "Banned", redeemerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerBanned)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "Banned", log); err != nil {
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

// ParseBanned is a log parse operation binding the contract event 0x30d1df1214d91553408ca5384ce29e10e5866af8423c628be22860e41fb81005.
//
// Solidity: event Banned(address indexed redeemer)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseBanned(log types.Log) (*RedemptionWatchtowerBanned, error) {
	event := new(RedemptionWatchtowerBanned)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "Banned", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerGuardianAddedIterator is returned from FilterGuardianAdded and is used to iterate over the raw logs and unpacked data for GuardianAdded events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerGuardianAddedIterator struct {
	Event *RedemptionWatchtowerGuardianAdded // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerGuardianAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerGuardianAdded)
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
		it.Event = new(RedemptionWatchtowerGuardianAdded)
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
func (it *RedemptionWatchtowerGuardianAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerGuardianAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerGuardianAdded represents a GuardianAdded event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerGuardianAdded struct {
	Guardian common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterGuardianAdded is a free log retrieval operation binding the contract event 0x038596bb31e2e7d3d9f184d4c98b310103f6d7f5830e5eec32bffe6f1728f969.
//
// Solidity: event GuardianAdded(address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterGuardianAdded(opts *bind.FilterOpts, guardian []common.Address) (*RedemptionWatchtowerGuardianAddedIterator, error) {

	var guardianRule []interface{}
	for _, guardianItem := range guardian {
		guardianRule = append(guardianRule, guardianItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "GuardianAdded", guardianRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerGuardianAddedIterator{contract: _RedemptionWatchtower.contract, event: "GuardianAdded", logs: logs, sub: sub}, nil
}

// WatchGuardianAdded is a free log subscription operation binding the contract event 0x038596bb31e2e7d3d9f184d4c98b310103f6d7f5830e5eec32bffe6f1728f969.
//
// Solidity: event GuardianAdded(address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchGuardianAdded(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerGuardianAdded, guardian []common.Address) (event.Subscription, error) {

	var guardianRule []interface{}
	for _, guardianItem := range guardian {
		guardianRule = append(guardianRule, guardianItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "GuardianAdded", guardianRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerGuardianAdded)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "GuardianAdded", log); err != nil {
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

// ParseGuardianAdded is a log parse operation binding the contract event 0x038596bb31e2e7d3d9f184d4c98b310103f6d7f5830e5eec32bffe6f1728f969.
//
// Solidity: event GuardianAdded(address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseGuardianAdded(log types.Log) (*RedemptionWatchtowerGuardianAdded, error) {
	event := new(RedemptionWatchtowerGuardianAdded)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "GuardianAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerGuardianRemovedIterator is returned from FilterGuardianRemoved and is used to iterate over the raw logs and unpacked data for GuardianRemoved events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerGuardianRemovedIterator struct {
	Event *RedemptionWatchtowerGuardianRemoved // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerGuardianRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerGuardianRemoved)
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
		it.Event = new(RedemptionWatchtowerGuardianRemoved)
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
func (it *RedemptionWatchtowerGuardianRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerGuardianRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerGuardianRemoved represents a GuardianRemoved event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerGuardianRemoved struct {
	Guardian common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterGuardianRemoved is a free log retrieval operation binding the contract event 0xb8107d0c6b40be480ce3172ee66ba6d64b71f6b1685a851340036e6e2e3e3c52.
//
// Solidity: event GuardianRemoved(address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterGuardianRemoved(opts *bind.FilterOpts, guardian []common.Address) (*RedemptionWatchtowerGuardianRemovedIterator, error) {

	var guardianRule []interface{}
	for _, guardianItem := range guardian {
		guardianRule = append(guardianRule, guardianItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "GuardianRemoved", guardianRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerGuardianRemovedIterator{contract: _RedemptionWatchtower.contract, event: "GuardianRemoved", logs: logs, sub: sub}, nil
}

// WatchGuardianRemoved is a free log subscription operation binding the contract event 0xb8107d0c6b40be480ce3172ee66ba6d64b71f6b1685a851340036e6e2e3e3c52.
//
// Solidity: event GuardianRemoved(address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchGuardianRemoved(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerGuardianRemoved, guardian []common.Address) (event.Subscription, error) {

	var guardianRule []interface{}
	for _, guardianItem := range guardian {
		guardianRule = append(guardianRule, guardianItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "GuardianRemoved", guardianRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerGuardianRemoved)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "GuardianRemoved", log); err != nil {
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

// ParseGuardianRemoved is a log parse operation binding the contract event 0xb8107d0c6b40be480ce3172ee66ba6d64b71f6b1685a851340036e6e2e3e3c52.
//
// Solidity: event GuardianRemoved(address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseGuardianRemoved(log types.Log) (*RedemptionWatchtowerGuardianRemoved, error) {
	event := new(RedemptionWatchtowerGuardianRemoved)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "GuardianRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerInitializedIterator struct {
	Event *RedemptionWatchtowerInitialized // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerInitialized)
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
		it.Event = new(RedemptionWatchtowerInitialized)
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
func (it *RedemptionWatchtowerInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerInitialized represents a Initialized event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterInitialized(opts *bind.FilterOpts) (*RedemptionWatchtowerInitializedIterator, error) {

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerInitializedIterator{contract: _RedemptionWatchtower.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerInitialized) (event.Subscription, error) {

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerInitialized)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseInitialized(log types.Log) (*RedemptionWatchtowerInitialized, error) {
	event := new(RedemptionWatchtowerInitialized)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerObjectionRaisedIterator is returned from FilterObjectionRaised and is used to iterate over the raw logs and unpacked data for ObjectionRaised events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerObjectionRaisedIterator struct {
	Event *RedemptionWatchtowerObjectionRaised // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerObjectionRaisedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerObjectionRaised)
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
		it.Event = new(RedemptionWatchtowerObjectionRaised)
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
func (it *RedemptionWatchtowerObjectionRaisedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerObjectionRaisedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerObjectionRaised represents a ObjectionRaised event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerObjectionRaised struct {
	RedemptionKey *big.Int
	Guardian      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterObjectionRaised is a free log retrieval operation binding the contract event 0x5d8525c7222757376eccb2f766e2a81b5afdb98843f7744a33488afe65b42066.
//
// Solidity: event ObjectionRaised(uint256 indexed redemptionKey, address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterObjectionRaised(opts *bind.FilterOpts, redemptionKey []*big.Int, guardian []common.Address) (*RedemptionWatchtowerObjectionRaisedIterator, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}
	var guardianRule []interface{}
	for _, guardianItem := range guardian {
		guardianRule = append(guardianRule, guardianItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "ObjectionRaised", redemptionKeyRule, guardianRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerObjectionRaisedIterator{contract: _RedemptionWatchtower.contract, event: "ObjectionRaised", logs: logs, sub: sub}, nil
}

// WatchObjectionRaised is a free log subscription operation binding the contract event 0x5d8525c7222757376eccb2f766e2a81b5afdb98843f7744a33488afe65b42066.
//
// Solidity: event ObjectionRaised(uint256 indexed redemptionKey, address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchObjectionRaised(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerObjectionRaised, redemptionKey []*big.Int, guardian []common.Address) (event.Subscription, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}
	var guardianRule []interface{}
	for _, guardianItem := range guardian {
		guardianRule = append(guardianRule, guardianItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "ObjectionRaised", redemptionKeyRule, guardianRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerObjectionRaised)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "ObjectionRaised", log); err != nil {
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

// ParseObjectionRaised is a log parse operation binding the contract event 0x5d8525c7222757376eccb2f766e2a81b5afdb98843f7744a33488afe65b42066.
//
// Solidity: event ObjectionRaised(uint256 indexed redemptionKey, address indexed guardian)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseObjectionRaised(log types.Log) (*RedemptionWatchtowerObjectionRaised, error) {
	event := new(RedemptionWatchtowerObjectionRaised)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "ObjectionRaised", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerOwnershipTransferredIterator struct {
	Event *RedemptionWatchtowerOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerOwnershipTransferred)
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
		it.Event = new(RedemptionWatchtowerOwnershipTransferred)
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
func (it *RedemptionWatchtowerOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerOwnershipTransferred represents a OwnershipTransferred event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*RedemptionWatchtowerOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerOwnershipTransferredIterator{contract: _RedemptionWatchtower.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerOwnershipTransferred)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseOwnershipTransferred(log types.Log) (*RedemptionWatchtowerOwnershipTransferred, error) {
	event := new(RedemptionWatchtowerOwnershipTransferred)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerUnbannedIterator is returned from FilterUnbanned and is used to iterate over the raw logs and unpacked data for Unbanned events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerUnbannedIterator struct {
	Event *RedemptionWatchtowerUnbanned // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerUnbannedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerUnbanned)
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
		it.Event = new(RedemptionWatchtowerUnbanned)
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
func (it *RedemptionWatchtowerUnbannedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerUnbannedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerUnbanned represents a Unbanned event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerUnbanned struct {
	Redeemer common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterUnbanned is a free log retrieval operation binding the contract event 0x2ab91b53354938415bb6962c4322231cd4cb2c84930f1a4b9abbedc2fe8abe72.
//
// Solidity: event Unbanned(address indexed redeemer)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterUnbanned(opts *bind.FilterOpts, redeemer []common.Address) (*RedemptionWatchtowerUnbannedIterator, error) {

	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "Unbanned", redeemerRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerUnbannedIterator{contract: _RedemptionWatchtower.contract, event: "Unbanned", logs: logs, sub: sub}, nil
}

// WatchUnbanned is a free log subscription operation binding the contract event 0x2ab91b53354938415bb6962c4322231cd4cb2c84930f1a4b9abbedc2fe8abe72.
//
// Solidity: event Unbanned(address indexed redeemer)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchUnbanned(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerUnbanned, redeemer []common.Address) (event.Subscription, error) {

	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "Unbanned", redeemerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerUnbanned)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "Unbanned", log); err != nil {
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

// ParseUnbanned is a log parse operation binding the contract event 0x2ab91b53354938415bb6962c4322231cd4cb2c84930f1a4b9abbedc2fe8abe72.
//
// Solidity: event Unbanned(address indexed redeemer)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseUnbanned(log types.Log) (*RedemptionWatchtowerUnbanned, error) {
	event := new(RedemptionWatchtowerUnbanned)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "Unbanned", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerVetoFinalizedIterator is returned from FilterVetoFinalized and is used to iterate over the raw logs and unpacked data for VetoFinalized events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerVetoFinalizedIterator struct {
	Event *RedemptionWatchtowerVetoFinalized // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerVetoFinalizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerVetoFinalized)
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
		it.Event = new(RedemptionWatchtowerVetoFinalized)
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
func (it *RedemptionWatchtowerVetoFinalizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerVetoFinalizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerVetoFinalized represents a VetoFinalized event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerVetoFinalized struct {
	RedemptionKey *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterVetoFinalized is a free log retrieval operation binding the contract event 0x8eb509395e8a959b0d0589ef622e986b3f49d1da636c2f27dc8a5e92a6625316.
//
// Solidity: event VetoFinalized(uint256 indexed redemptionKey)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterVetoFinalized(opts *bind.FilterOpts, redemptionKey []*big.Int) (*RedemptionWatchtowerVetoFinalizedIterator, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "VetoFinalized", redemptionKeyRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerVetoFinalizedIterator{contract: _RedemptionWatchtower.contract, event: "VetoFinalized", logs: logs, sub: sub}, nil
}

// WatchVetoFinalized is a free log subscription operation binding the contract event 0x8eb509395e8a959b0d0589ef622e986b3f49d1da636c2f27dc8a5e92a6625316.
//
// Solidity: event VetoFinalized(uint256 indexed redemptionKey)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchVetoFinalized(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerVetoFinalized, redemptionKey []*big.Int) (event.Subscription, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "VetoFinalized", redemptionKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerVetoFinalized)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "VetoFinalized", log); err != nil {
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

// ParseVetoFinalized is a log parse operation binding the contract event 0x8eb509395e8a959b0d0589ef622e986b3f49d1da636c2f27dc8a5e92a6625316.
//
// Solidity: event VetoFinalized(uint256 indexed redemptionKey)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseVetoFinalized(log types.Log) (*RedemptionWatchtowerVetoFinalized, error) {
	event := new(RedemptionWatchtowerVetoFinalized)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "VetoFinalized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerVetoPeriodCheckOmittedIterator is returned from FilterVetoPeriodCheckOmitted and is used to iterate over the raw logs and unpacked data for VetoPeriodCheckOmitted events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerVetoPeriodCheckOmittedIterator struct {
	Event *RedemptionWatchtowerVetoPeriodCheckOmitted // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerVetoPeriodCheckOmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerVetoPeriodCheckOmitted)
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
		it.Event = new(RedemptionWatchtowerVetoPeriodCheckOmitted)
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
func (it *RedemptionWatchtowerVetoPeriodCheckOmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerVetoPeriodCheckOmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerVetoPeriodCheckOmitted represents a VetoPeriodCheckOmitted event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerVetoPeriodCheckOmitted struct {
	RedemptionKey *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterVetoPeriodCheckOmitted is a free log retrieval operation binding the contract event 0x2d59b29d731ca4ec69587ad0b14c55ab6746d2b1ebf6ae0552d2dbd7dec886df.
//
// Solidity: event VetoPeriodCheckOmitted(uint256 indexed redemptionKey)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterVetoPeriodCheckOmitted(opts *bind.FilterOpts, redemptionKey []*big.Int) (*RedemptionWatchtowerVetoPeriodCheckOmittedIterator, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "VetoPeriodCheckOmitted", redemptionKeyRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerVetoPeriodCheckOmittedIterator{contract: _RedemptionWatchtower.contract, event: "VetoPeriodCheckOmitted", logs: logs, sub: sub}, nil
}

// WatchVetoPeriodCheckOmitted is a free log subscription operation binding the contract event 0x2d59b29d731ca4ec69587ad0b14c55ab6746d2b1ebf6ae0552d2dbd7dec886df.
//
// Solidity: event VetoPeriodCheckOmitted(uint256 indexed redemptionKey)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchVetoPeriodCheckOmitted(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerVetoPeriodCheckOmitted, redemptionKey []*big.Int) (event.Subscription, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "VetoPeriodCheckOmitted", redemptionKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerVetoPeriodCheckOmitted)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "VetoPeriodCheckOmitted", log); err != nil {
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

// ParseVetoPeriodCheckOmitted is a log parse operation binding the contract event 0x2d59b29d731ca4ec69587ad0b14c55ab6746d2b1ebf6ae0552d2dbd7dec886df.
//
// Solidity: event VetoPeriodCheckOmitted(uint256 indexed redemptionKey)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseVetoPeriodCheckOmitted(log types.Log) (*RedemptionWatchtowerVetoPeriodCheckOmitted, error) {
	event := new(RedemptionWatchtowerVetoPeriodCheckOmitted)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "VetoPeriodCheckOmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerVetoedFundsWithdrawnIterator is returned from FilterVetoedFundsWithdrawn and is used to iterate over the raw logs and unpacked data for VetoedFundsWithdrawn events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerVetoedFundsWithdrawnIterator struct {
	Event *RedemptionWatchtowerVetoedFundsWithdrawn // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerVetoedFundsWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerVetoedFundsWithdrawn)
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
		it.Event = new(RedemptionWatchtowerVetoedFundsWithdrawn)
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
func (it *RedemptionWatchtowerVetoedFundsWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerVetoedFundsWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerVetoedFundsWithdrawn represents a VetoedFundsWithdrawn event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerVetoedFundsWithdrawn struct {
	RedemptionKey *big.Int
	Redeemer      common.Address
	Amount        uint64
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterVetoedFundsWithdrawn is a free log retrieval operation binding the contract event 0xd826f2aef2bafd3d282c752ed164de2f4b2b44391c8b123c5c22c7846943f7c2.
//
// Solidity: event VetoedFundsWithdrawn(uint256 indexed redemptionKey, address indexed redeemer, uint64 amount)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterVetoedFundsWithdrawn(opts *bind.FilterOpts, redemptionKey []*big.Int, redeemer []common.Address) (*RedemptionWatchtowerVetoedFundsWithdrawnIterator, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}
	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "VetoedFundsWithdrawn", redemptionKeyRule, redeemerRule)
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerVetoedFundsWithdrawnIterator{contract: _RedemptionWatchtower.contract, event: "VetoedFundsWithdrawn", logs: logs, sub: sub}, nil
}

// WatchVetoedFundsWithdrawn is a free log subscription operation binding the contract event 0xd826f2aef2bafd3d282c752ed164de2f4b2b44391c8b123c5c22c7846943f7c2.
//
// Solidity: event VetoedFundsWithdrawn(uint256 indexed redemptionKey, address indexed redeemer, uint64 amount)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchVetoedFundsWithdrawn(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerVetoedFundsWithdrawn, redemptionKey []*big.Int, redeemer []common.Address) (event.Subscription, error) {

	var redemptionKeyRule []interface{}
	for _, redemptionKeyItem := range redemptionKey {
		redemptionKeyRule = append(redemptionKeyRule, redemptionKeyItem)
	}
	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "VetoedFundsWithdrawn", redemptionKeyRule, redeemerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerVetoedFundsWithdrawn)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "VetoedFundsWithdrawn", log); err != nil {
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

// ParseVetoedFundsWithdrawn is a log parse operation binding the contract event 0xd826f2aef2bafd3d282c752ed164de2f4b2b44391c8b123c5c22c7846943f7c2.
//
// Solidity: event VetoedFundsWithdrawn(uint256 indexed redemptionKey, address indexed redeemer, uint64 amount)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseVetoedFundsWithdrawn(log types.Log) (*RedemptionWatchtowerVetoedFundsWithdrawn, error) {
	event := new(RedemptionWatchtowerVetoedFundsWithdrawn)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "VetoedFundsWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerWatchtowerDisabledIterator is returned from FilterWatchtowerDisabled and is used to iterate over the raw logs and unpacked data for WatchtowerDisabled events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerWatchtowerDisabledIterator struct {
	Event *RedemptionWatchtowerWatchtowerDisabled // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerWatchtowerDisabledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerWatchtowerDisabled)
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
		it.Event = new(RedemptionWatchtowerWatchtowerDisabled)
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
func (it *RedemptionWatchtowerWatchtowerDisabledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerWatchtowerDisabledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerWatchtowerDisabled represents a WatchtowerDisabled event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerWatchtowerDisabled struct {
	DisabledAt uint32
	Executor   common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterWatchtowerDisabled is a free log retrieval operation binding the contract event 0x825315723dc33add785ed4b997cbc4559a6c2cfee60833751b22a1f7c83bbe29.
//
// Solidity: event WatchtowerDisabled(uint32 disabledAt, address executor)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterWatchtowerDisabled(opts *bind.FilterOpts) (*RedemptionWatchtowerWatchtowerDisabledIterator, error) {

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "WatchtowerDisabled")
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerWatchtowerDisabledIterator{contract: _RedemptionWatchtower.contract, event: "WatchtowerDisabled", logs: logs, sub: sub}, nil
}

// WatchWatchtowerDisabled is a free log subscription operation binding the contract event 0x825315723dc33add785ed4b997cbc4559a6c2cfee60833751b22a1f7c83bbe29.
//
// Solidity: event WatchtowerDisabled(uint32 disabledAt, address executor)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchWatchtowerDisabled(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerWatchtowerDisabled) (event.Subscription, error) {

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "WatchtowerDisabled")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerWatchtowerDisabled)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "WatchtowerDisabled", log); err != nil {
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

// ParseWatchtowerDisabled is a log parse operation binding the contract event 0x825315723dc33add785ed4b997cbc4559a6c2cfee60833751b22a1f7c83bbe29.
//
// Solidity: event WatchtowerDisabled(uint32 disabledAt, address executor)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseWatchtowerDisabled(log types.Log) (*RedemptionWatchtowerWatchtowerDisabled, error) {
	event := new(RedemptionWatchtowerWatchtowerDisabled)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "WatchtowerDisabled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerWatchtowerEnabledIterator is returned from FilterWatchtowerEnabled and is used to iterate over the raw logs and unpacked data for WatchtowerEnabled events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerWatchtowerEnabledIterator struct {
	Event *RedemptionWatchtowerWatchtowerEnabled // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerWatchtowerEnabledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerWatchtowerEnabled)
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
		it.Event = new(RedemptionWatchtowerWatchtowerEnabled)
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
func (it *RedemptionWatchtowerWatchtowerEnabledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerWatchtowerEnabledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerWatchtowerEnabled represents a WatchtowerEnabled event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerWatchtowerEnabled struct {
	EnabledAt uint32
	Manager   common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterWatchtowerEnabled is a free log retrieval operation binding the contract event 0xb4a5b13ab5dc30d41ccd0e7f76f660f6354b8ddb4c23008ec971f1ef41c7916b.
//
// Solidity: event WatchtowerEnabled(uint32 enabledAt, address manager)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterWatchtowerEnabled(opts *bind.FilterOpts) (*RedemptionWatchtowerWatchtowerEnabledIterator, error) {

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "WatchtowerEnabled")
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerWatchtowerEnabledIterator{contract: _RedemptionWatchtower.contract, event: "WatchtowerEnabled", logs: logs, sub: sub}, nil
}

// WatchWatchtowerEnabled is a free log subscription operation binding the contract event 0xb4a5b13ab5dc30d41ccd0e7f76f660f6354b8ddb4c23008ec971f1ef41c7916b.
//
// Solidity: event WatchtowerEnabled(uint32 enabledAt, address manager)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchWatchtowerEnabled(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerWatchtowerEnabled) (event.Subscription, error) {

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "WatchtowerEnabled")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerWatchtowerEnabled)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "WatchtowerEnabled", log); err != nil {
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

// ParseWatchtowerEnabled is a log parse operation binding the contract event 0xb4a5b13ab5dc30d41ccd0e7f76f660f6354b8ddb4c23008ec971f1ef41c7916b.
//
// Solidity: event WatchtowerEnabled(uint32 enabledAt, address manager)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseWatchtowerEnabled(log types.Log) (*RedemptionWatchtowerWatchtowerEnabled, error) {
	event := new(RedemptionWatchtowerWatchtowerEnabled)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "WatchtowerEnabled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RedemptionWatchtowerWatchtowerParametersUpdatedIterator is returned from FilterWatchtowerParametersUpdated and is used to iterate over the raw logs and unpacked data for WatchtowerParametersUpdated events raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerWatchtowerParametersUpdatedIterator struct {
	Event *RedemptionWatchtowerWatchtowerParametersUpdated // Event containing the contract specifics and raw log

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
func (it *RedemptionWatchtowerWatchtowerParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RedemptionWatchtowerWatchtowerParametersUpdated)
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
		it.Event = new(RedemptionWatchtowerWatchtowerParametersUpdated)
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
func (it *RedemptionWatchtowerWatchtowerParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RedemptionWatchtowerWatchtowerParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RedemptionWatchtowerWatchtowerParametersUpdated represents a WatchtowerParametersUpdated event raised by the RedemptionWatchtower contract.
type RedemptionWatchtowerWatchtowerParametersUpdated struct {
	WatchtowerLifetime    uint32
	VetoPenaltyFeeDivisor uint64
	VetoFreezePeriod      uint32
	DefaultDelay          uint32
	LevelOneDelay         uint32
	LevelTwoDelay         uint32
	WaivedAmountLimit     uint64
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterWatchtowerParametersUpdated is a free log retrieval operation binding the contract event 0x3926935ab567e74893b3e3c7ed329808cfe8db713a1456b29143a0d35b8ac40b.
//
// Solidity: event WatchtowerParametersUpdated(uint32 watchtowerLifetime, uint64 vetoPenaltyFeeDivisor, uint32 vetoFreezePeriod, uint32 defaultDelay, uint32 levelOneDelay, uint32 levelTwoDelay, uint64 waivedAmountLimit)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) FilterWatchtowerParametersUpdated(opts *bind.FilterOpts) (*RedemptionWatchtowerWatchtowerParametersUpdatedIterator, error) {

	logs, sub, err := _RedemptionWatchtower.contract.FilterLogs(opts, "WatchtowerParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &RedemptionWatchtowerWatchtowerParametersUpdatedIterator{contract: _RedemptionWatchtower.contract, event: "WatchtowerParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchWatchtowerParametersUpdated is a free log subscription operation binding the contract event 0x3926935ab567e74893b3e3c7ed329808cfe8db713a1456b29143a0d35b8ac40b.
//
// Solidity: event WatchtowerParametersUpdated(uint32 watchtowerLifetime, uint64 vetoPenaltyFeeDivisor, uint32 vetoFreezePeriod, uint32 defaultDelay, uint32 levelOneDelay, uint32 levelTwoDelay, uint64 waivedAmountLimit)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) WatchWatchtowerParametersUpdated(opts *bind.WatchOpts, sink chan<- *RedemptionWatchtowerWatchtowerParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _RedemptionWatchtower.contract.WatchLogs(opts, "WatchtowerParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RedemptionWatchtowerWatchtowerParametersUpdated)
				if err := _RedemptionWatchtower.contract.UnpackLog(event, "WatchtowerParametersUpdated", log); err != nil {
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

// ParseWatchtowerParametersUpdated is a log parse operation binding the contract event 0x3926935ab567e74893b3e3c7ed329808cfe8db713a1456b29143a0d35b8ac40b.
//
// Solidity: event WatchtowerParametersUpdated(uint32 watchtowerLifetime, uint64 vetoPenaltyFeeDivisor, uint32 vetoFreezePeriod, uint32 defaultDelay, uint32 levelOneDelay, uint32 levelTwoDelay, uint64 waivedAmountLimit)
func (_RedemptionWatchtower *RedemptionWatchtowerFilterer) ParseWatchtowerParametersUpdated(log types.Log) (*RedemptionWatchtowerWatchtowerParametersUpdated, error) {
	event := new(RedemptionWatchtowerWatchtowerParametersUpdated)
	if err := _RedemptionWatchtower.contract.UnpackLog(event, "WatchtowerParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

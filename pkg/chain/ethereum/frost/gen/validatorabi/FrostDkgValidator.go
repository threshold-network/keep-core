// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package validatorabi

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
	MisbehavedMembersIndices []uint8
	Signatures               []byte
	SigningMembersIndices    []*big.Int
	Members                  []uint32
	MembersHash              [32]byte
}

// FrostDkgValidatorMetaData contains all meta data concerning the FrostDkgValidator contract.
var FrostDkgValidatorMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"resultDigest\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"result\",\"type\":\"tuple\",\"components\":[{\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"name\":\"signatures\",\"type\":\"bytes\"},{\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"name\":\"members\",\"type\":\"uint32[]\"},{\"name\":\"membersHash\",\"type\":\"bytes32\"}]},{\"name\":\"seed\",\"type\":\"uint256\"},{\"name\":\"bridge\",\"type\":\"address\"},{\"name\":\"registry\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"digest\",\"type\":\"bytes32\"}]}]",
}

// FrostDkgValidatorABI is the input ABI used to generate the binding from.
// Deprecated: Use FrostDkgValidatorMetaData.ABI instead.
var FrostDkgValidatorABI = FrostDkgValidatorMetaData.ABI

// FrostDkgValidator is an auto generated Go binding around an Ethereum contract.
type FrostDkgValidator struct {
	FrostDkgValidatorCaller     // Read-only binding to the contract
	FrostDkgValidatorTransactor // Write-only binding to the contract
	FrostDkgValidatorFilterer   // Log filterer for contract events
}

// FrostDkgValidatorCaller is an auto generated read-only Go binding around an Ethereum contract.
type FrostDkgValidatorCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FrostDkgValidatorTransactor is an auto generated write-only Go binding around an Ethereum contract.
type FrostDkgValidatorTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FrostDkgValidatorFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type FrostDkgValidatorFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FrostDkgValidatorSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type FrostDkgValidatorSession struct {
	Contract     *FrostDkgValidator // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// FrostDkgValidatorCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type FrostDkgValidatorCallerSession struct {
	Contract *FrostDkgValidatorCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// FrostDkgValidatorTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type FrostDkgValidatorTransactorSession struct {
	Contract     *FrostDkgValidatorTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// FrostDkgValidatorRaw is an auto generated low-level Go binding around an Ethereum contract.
type FrostDkgValidatorRaw struct {
	Contract *FrostDkgValidator // Generic contract binding to access the raw methods on
}

// FrostDkgValidatorCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type FrostDkgValidatorCallerRaw struct {
	Contract *FrostDkgValidatorCaller // Generic read-only contract binding to access the raw methods on
}

// FrostDkgValidatorTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type FrostDkgValidatorTransactorRaw struct {
	Contract *FrostDkgValidatorTransactor // Generic write-only contract binding to access the raw methods on
}

// NewFrostDkgValidator creates a new instance of FrostDkgValidator, bound to a specific deployed contract.
func NewFrostDkgValidator(address common.Address, backend bind.ContractBackend) (*FrostDkgValidator, error) {
	contract, err := bindFrostDkgValidator(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &FrostDkgValidator{FrostDkgValidatorCaller: FrostDkgValidatorCaller{contract: contract}, FrostDkgValidatorTransactor: FrostDkgValidatorTransactor{contract: contract}, FrostDkgValidatorFilterer: FrostDkgValidatorFilterer{contract: contract}}, nil
}

// NewFrostDkgValidatorCaller creates a new read-only instance of FrostDkgValidator, bound to a specific deployed contract.
func NewFrostDkgValidatorCaller(address common.Address, caller bind.ContractCaller) (*FrostDkgValidatorCaller, error) {
	contract, err := bindFrostDkgValidator(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FrostDkgValidatorCaller{contract: contract}, nil
}

// NewFrostDkgValidatorTransactor creates a new write-only instance of FrostDkgValidator, bound to a specific deployed contract.
func NewFrostDkgValidatorTransactor(address common.Address, transactor bind.ContractTransactor) (*FrostDkgValidatorTransactor, error) {
	contract, err := bindFrostDkgValidator(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FrostDkgValidatorTransactor{contract: contract}, nil
}

// NewFrostDkgValidatorFilterer creates a new log filterer instance of FrostDkgValidator, bound to a specific deployed contract.
func NewFrostDkgValidatorFilterer(address common.Address, filterer bind.ContractFilterer) (*FrostDkgValidatorFilterer, error) {
	contract, err := bindFrostDkgValidator(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FrostDkgValidatorFilterer{contract: contract}, nil
}

// bindFrostDkgValidator binds a generic wrapper to an already deployed contract.
func bindFrostDkgValidator(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := FrostDkgValidatorMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FrostDkgValidator *FrostDkgValidatorRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FrostDkgValidator.Contract.FrostDkgValidatorCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FrostDkgValidator *FrostDkgValidatorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostDkgValidator.Contract.FrostDkgValidatorTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FrostDkgValidator *FrostDkgValidatorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FrostDkgValidator.Contract.FrostDkgValidatorTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FrostDkgValidator *FrostDkgValidatorCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FrostDkgValidator.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FrostDkgValidator *FrostDkgValidatorTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FrostDkgValidator.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FrostDkgValidator *FrostDkgValidatorTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FrostDkgValidator.Contract.contract.Transact(opts, method, params...)
}

// ResultDigest is a free data retrieval call binding the contract method 0xa63415cd.
//
// Solidity: function resultDigest((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, address bridge, address registry) view returns(bytes32 digest)
func (_FrostDkgValidator *FrostDkgValidatorCaller) ResultDigest(opts *bind.CallOpts, result Struct0, seed *big.Int, bridge common.Address, registry common.Address) ([32]byte, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "resultDigest", result, seed, bridge, registry)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ResultDigest is a free data retrieval call binding the contract method 0xa63415cd.
//
// Solidity: function resultDigest((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, address bridge, address registry) view returns(bytes32 digest)
func (_FrostDkgValidator *FrostDkgValidatorSession) ResultDigest(result Struct0, seed *big.Int, bridge common.Address, registry common.Address) ([32]byte, error) {
	return _FrostDkgValidator.Contract.ResultDigest(&_FrostDkgValidator.CallOpts, result, seed, bridge, registry)
}

// ResultDigest is a free data retrieval call binding the contract method 0xa63415cd.
//
// Solidity: function resultDigest((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, address bridge, address registry) view returns(bytes32 digest)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) ResultDigest(result Struct0, seed *big.Int, bridge common.Address, registry common.Address) ([32]byte, error) {
	return _FrostDkgValidator.Contract.ResultDigest(&_FrostDkgValidator.CallOpts, result, seed, bridge, registry)
}

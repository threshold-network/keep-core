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

// FrostDkgResult is an auto generated low-level Go binding around an user-defined struct.
type FrostDkgResult struct {
	SubmitterMemberIndex     *big.Int
	XOnlyOutputKey           [32]byte
	MisbehavedMembersIndices []uint8
	Signatures               []byte
	SigningMembersIndices    []*big.Int
	Members                  []uint32
	MembersHash              [32]byte
}

// FrostDkgValidatorDigestBinding is an auto generated low-level Go binding around an user-defined struct.
type FrostDkgValidatorDigestBinding struct {
	Bridge   common.Address
	Registry common.Address
}

// FrostDkgValidatorMetaData contains all meta data concerning the FrostDkgValidator contract.
var FrostDkgValidatorMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"_sortitionPool\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"activeThreshold\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"groupSize\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"groupThreshold\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"publicKeyByteSize\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"bridge\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"registry\",\"type\":\"address\"}],\"name\":\"resultDigest\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"digest\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"signatureByteSize\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"sortitionPool\",\"outputs\":[{\"internalType\":\"contractSortitionPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"startBlock\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"bridge\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"registry\",\"type\":\"address\"}],\"name\":\"validate\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"isValid\",\"type\":\"bool\"},{\"internalType\":\"string\",\"name\":\"errorMsg\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"}],\"name\":\"validateFields\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"isValid\",\"type\":\"bool\"},{\"internalType\":\"string\",\"name\":\"errorMsg\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"}],\"name\":\"validateGroupMembers\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"}],\"name\":\"validateMembersHash\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"submitterMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"xOnlyOutputKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint8[]\",\"name\":\"misbehavedMembersIndices\",\"type\":\"uint8[]\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"signingMembersIndices\",\"type\":\"uint256[]\"},{\"internalType\":\"uint32[]\",\"name\":\"members\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes32\",\"name\":\"membersHash\",\"type\":\"bytes32\"}],\"internalType\":\"structFrostDkg.Result\",\"name\":\"result\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"seed\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"bridge\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"registry\",\"type\":\"address\"}],\"internalType\":\"structFrostDkgValidator.DigestBinding\",\"name\":\"binding\",\"type\":\"tuple\"}],\"name\":\"validateSignatures\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
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

// ActiveThreshold is a free data retrieval call binding the contract method 0x281efe71.
//
// Solidity: function activeThreshold() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCaller) ActiveThreshold(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "activeThreshold")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ActiveThreshold is a free data retrieval call binding the contract method 0x281efe71.
//
// Solidity: function activeThreshold() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorSession) ActiveThreshold() (*big.Int, error) {
	return _FrostDkgValidator.Contract.ActiveThreshold(&_FrostDkgValidator.CallOpts)
}

// ActiveThreshold is a free data retrieval call binding the contract method 0x281efe71.
//
// Solidity: function activeThreshold() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) ActiveThreshold() (*big.Int, error) {
	return _FrostDkgValidator.Contract.ActiveThreshold(&_FrostDkgValidator.CallOpts)
}

// GroupSize is a free data retrieval call binding the contract method 0x63b635ea.
//
// Solidity: function groupSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCaller) GroupSize(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "groupSize")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GroupSize is a free data retrieval call binding the contract method 0x63b635ea.
//
// Solidity: function groupSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorSession) GroupSize() (*big.Int, error) {
	return _FrostDkgValidator.Contract.GroupSize(&_FrostDkgValidator.CallOpts)
}

// GroupSize is a free data retrieval call binding the contract method 0x63b635ea.
//
// Solidity: function groupSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) GroupSize() (*big.Int, error) {
	return _FrostDkgValidator.Contract.GroupSize(&_FrostDkgValidator.CallOpts)
}

// GroupThreshold is a free data retrieval call binding the contract method 0x6dcc64f8.
//
// Solidity: function groupThreshold() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCaller) GroupThreshold(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "groupThreshold")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GroupThreshold is a free data retrieval call binding the contract method 0x6dcc64f8.
//
// Solidity: function groupThreshold() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorSession) GroupThreshold() (*big.Int, error) {
	return _FrostDkgValidator.Contract.GroupThreshold(&_FrostDkgValidator.CallOpts)
}

// GroupThreshold is a free data retrieval call binding the contract method 0x6dcc64f8.
//
// Solidity: function groupThreshold() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) GroupThreshold() (*big.Int, error) {
	return _FrostDkgValidator.Contract.GroupThreshold(&_FrostDkgValidator.CallOpts)
}

// PublicKeyByteSize is a free data retrieval call binding the contract method 0x05f8ae15.
//
// Solidity: function publicKeyByteSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCaller) PublicKeyByteSize(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "publicKeyByteSize")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PublicKeyByteSize is a free data retrieval call binding the contract method 0x05f8ae15.
//
// Solidity: function publicKeyByteSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorSession) PublicKeyByteSize() (*big.Int, error) {
	return _FrostDkgValidator.Contract.PublicKeyByteSize(&_FrostDkgValidator.CallOpts)
}

// PublicKeyByteSize is a free data retrieval call binding the contract method 0x05f8ae15.
//
// Solidity: function publicKeyByteSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) PublicKeyByteSize() (*big.Int, error) {
	return _FrostDkgValidator.Contract.PublicKeyByteSize(&_FrostDkgValidator.CallOpts)
}

// ResultDigest is a free data retrieval call binding the contract method 0xa63415cd.
//
// Solidity: function resultDigest((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, address bridge, address registry) view returns(bytes32 digest)
func (_FrostDkgValidator *FrostDkgValidatorCaller) ResultDigest(opts *bind.CallOpts, result FrostDkgResult, seed *big.Int, bridge common.Address, registry common.Address) ([32]byte, error) {
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
func (_FrostDkgValidator *FrostDkgValidatorSession) ResultDigest(result FrostDkgResult, seed *big.Int, bridge common.Address, registry common.Address) ([32]byte, error) {
	return _FrostDkgValidator.Contract.ResultDigest(&_FrostDkgValidator.CallOpts, result, seed, bridge, registry)
}

// ResultDigest is a free data retrieval call binding the contract method 0xa63415cd.
//
// Solidity: function resultDigest((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, address bridge, address registry) view returns(bytes32 digest)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) ResultDigest(result FrostDkgResult, seed *big.Int, bridge common.Address, registry common.Address) ([32]byte, error) {
	return _FrostDkgValidator.Contract.ResultDigest(&_FrostDkgValidator.CallOpts, result, seed, bridge, registry)
}

// SignatureByteSize is a free data retrieval call binding the contract method 0x89ef44b0.
//
// Solidity: function signatureByteSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCaller) SignatureByteSize(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "signatureByteSize")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SignatureByteSize is a free data retrieval call binding the contract method 0x89ef44b0.
//
// Solidity: function signatureByteSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorSession) SignatureByteSize() (*big.Int, error) {
	return _FrostDkgValidator.Contract.SignatureByteSize(&_FrostDkgValidator.CallOpts)
}

// SignatureByteSize is a free data retrieval call binding the contract method 0x89ef44b0.
//
// Solidity: function signatureByteSize() view returns(uint256)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) SignatureByteSize() (*big.Int, error) {
	return _FrostDkgValidator.Contract.SignatureByteSize(&_FrostDkgValidator.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_FrostDkgValidator *FrostDkgValidatorCaller) SortitionPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "sortitionPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_FrostDkgValidator *FrostDkgValidatorSession) SortitionPool() (common.Address, error) {
	return _FrostDkgValidator.Contract.SortitionPool(&_FrostDkgValidator.CallOpts)
}

// SortitionPool is a free data retrieval call binding the contract method 0xb54a2374.
//
// Solidity: function sortitionPool() view returns(address)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) SortitionPool() (common.Address, error) {
	return _FrostDkgValidator.Contract.SortitionPool(&_FrostDkgValidator.CallOpts)
}

// Validate is a free data retrieval call binding the contract method 0x8a399fcf.
//
// Solidity: function validate((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, uint256 startBlock, address bridge, address registry) view returns(bool isValid, string errorMsg)
func (_FrostDkgValidator *FrostDkgValidatorCaller) Validate(opts *bind.CallOpts, result FrostDkgResult, seed *big.Int, startBlock *big.Int, bridge common.Address, registry common.Address) (struct {
	IsValid  bool
	ErrorMsg string
}, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "validate", result, seed, startBlock, bridge, registry)

	outstruct := new(struct {
		IsValid  bool
		ErrorMsg string
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.IsValid = *abi.ConvertType(out[0], new(bool)).(*bool)
	outstruct.ErrorMsg = *abi.ConvertType(out[1], new(string)).(*string)

	return *outstruct, err

}

// Validate is a free data retrieval call binding the contract method 0x8a399fcf.
//
// Solidity: function validate((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, uint256 startBlock, address bridge, address registry) view returns(bool isValid, string errorMsg)
func (_FrostDkgValidator *FrostDkgValidatorSession) Validate(result FrostDkgResult, seed *big.Int, startBlock *big.Int, bridge common.Address, registry common.Address) (struct {
	IsValid  bool
	ErrorMsg string
}, error) {
	return _FrostDkgValidator.Contract.Validate(&_FrostDkgValidator.CallOpts, result, seed, startBlock, bridge, registry)
}

// Validate is a free data retrieval call binding the contract method 0x8a399fcf.
//
// Solidity: function validate((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, uint256 startBlock, address bridge, address registry) view returns(bool isValid, string errorMsg)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) Validate(result FrostDkgResult, seed *big.Int, startBlock *big.Int, bridge common.Address, registry common.Address) (struct {
	IsValid  bool
	ErrorMsg string
}, error) {
	return _FrostDkgValidator.Contract.Validate(&_FrostDkgValidator.CallOpts, result, seed, startBlock, bridge, registry)
}

// ValidateFields is a free data retrieval call binding the contract method 0x0a51bd1f.
//
// Solidity: function validateFields((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) pure returns(bool isValid, string errorMsg)
func (_FrostDkgValidator *FrostDkgValidatorCaller) ValidateFields(opts *bind.CallOpts, result FrostDkgResult) (struct {
	IsValid  bool
	ErrorMsg string
}, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "validateFields", result)

	outstruct := new(struct {
		IsValid  bool
		ErrorMsg string
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.IsValid = *abi.ConvertType(out[0], new(bool)).(*bool)
	outstruct.ErrorMsg = *abi.ConvertType(out[1], new(string)).(*string)

	return *outstruct, err

}

// ValidateFields is a free data retrieval call binding the contract method 0x0a51bd1f.
//
// Solidity: function validateFields((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) pure returns(bool isValid, string errorMsg)
func (_FrostDkgValidator *FrostDkgValidatorSession) ValidateFields(result FrostDkgResult) (struct {
	IsValid  bool
	ErrorMsg string
}, error) {
	return _FrostDkgValidator.Contract.ValidateFields(&_FrostDkgValidator.CallOpts, result)
}

// ValidateFields is a free data retrieval call binding the contract method 0x0a51bd1f.
//
// Solidity: function validateFields((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) pure returns(bool isValid, string errorMsg)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) ValidateFields(result FrostDkgResult) (struct {
	IsValid  bool
	ErrorMsg string
}, error) {
	return _FrostDkgValidator.Contract.ValidateFields(&_FrostDkgValidator.CallOpts, result)
}

// ValidateGroupMembers is a free data retrieval call binding the contract method 0x11ee7310.
//
// Solidity: function validateGroupMembers((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed) view returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorCaller) ValidateGroupMembers(opts *bind.CallOpts, result FrostDkgResult, seed *big.Int) (bool, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "validateGroupMembers", result, seed)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateGroupMembers is a free data retrieval call binding the contract method 0x11ee7310.
//
// Solidity: function validateGroupMembers((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed) view returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorSession) ValidateGroupMembers(result FrostDkgResult, seed *big.Int) (bool, error) {
	return _FrostDkgValidator.Contract.ValidateGroupMembers(&_FrostDkgValidator.CallOpts, result, seed)
}

// ValidateGroupMembers is a free data retrieval call binding the contract method 0x11ee7310.
//
// Solidity: function validateGroupMembers((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed) view returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) ValidateGroupMembers(result FrostDkgResult, seed *big.Int) (bool, error) {
	return _FrostDkgValidator.Contract.ValidateGroupMembers(&_FrostDkgValidator.CallOpts, result, seed)
}

// ValidateMembersHash is a free data retrieval call binding the contract method 0xd01d1f3f.
//
// Solidity: function validateMembersHash((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) pure returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorCaller) ValidateMembersHash(opts *bind.CallOpts, result FrostDkgResult) (bool, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "validateMembersHash", result)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateMembersHash is a free data retrieval call binding the contract method 0xd01d1f3f.
//
// Solidity: function validateMembersHash((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) pure returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorSession) ValidateMembersHash(result FrostDkgResult) (bool, error) {
	return _FrostDkgValidator.Contract.ValidateMembersHash(&_FrostDkgValidator.CallOpts, result)
}

// ValidateMembersHash is a free data retrieval call binding the contract method 0xd01d1f3f.
//
// Solidity: function validateMembersHash((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result) pure returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) ValidateMembersHash(result FrostDkgResult) (bool, error) {
	return _FrostDkgValidator.Contract.ValidateMembersHash(&_FrostDkgValidator.CallOpts, result)
}

// ValidateSignatures is a free data retrieval call binding the contract method 0xb03a9444.
//
// Solidity: function validateSignatures((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, uint256 , (address,address) binding) view returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorCaller) ValidateSignatures(opts *bind.CallOpts, result FrostDkgResult, seed *big.Int, arg2 *big.Int, binding FrostDkgValidatorDigestBinding) (bool, error) {
	var out []interface{}
	err := _FrostDkgValidator.contract.Call(opts, &out, "validateSignatures", result, seed, arg2, binding)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateSignatures is a free data retrieval call binding the contract method 0xb03a9444.
//
// Solidity: function validateSignatures((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, uint256 , (address,address) binding) view returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorSession) ValidateSignatures(result FrostDkgResult, seed *big.Int, arg2 *big.Int, binding FrostDkgValidatorDigestBinding) (bool, error) {
	return _FrostDkgValidator.Contract.ValidateSignatures(&_FrostDkgValidator.CallOpts, result, seed, arg2, binding)
}

// ValidateSignatures is a free data retrieval call binding the contract method 0xb03a9444.
//
// Solidity: function validateSignatures((uint256,bytes32,uint8[],bytes,uint256[],uint32[],bytes32) result, uint256 seed, uint256 , (address,address) binding) view returns(bool)
func (_FrostDkgValidator *FrostDkgValidatorCallerSession) ValidateSignatures(result FrostDkgResult, seed *big.Int, arg2 *big.Int, binding FrostDkgValidatorDigestBinding) (bool, error) {
	return _FrostDkgValidator.Contract.ValidateSignatures(&_FrostDkgValidator.CallOpts, result, seed, arg2, binding)
}

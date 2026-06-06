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

// BitcoinTxInfo2 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxInfo2 struct {
	Version      [4]byte
	InputVector  []byte
	OutputVector []byte
	Locktime     [4]byte
}

// BitcoinTxUTXO3 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxUTXO3 struct {
	TxHash        [32]byte
	TxOutputIndex uint32
	TxOutputValue uint64
}

// WalletProposalValidatorDepositExtraInfo is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorDepositExtraInfo struct {
	FundingTx        BitcoinTxInfo2
	BlindingFactor   [8]byte
	WalletPubKeyHash [20]byte
	RefundPubKeyHash [20]byte
	RefundLocktime   [4]byte
}

// WalletProposalValidatorDepositKey is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorDepositKey struct {
	FundingTxHash      [32]byte
	FundingOutputIndex uint32
}

// WalletProposalValidatorDepositSweepProposal is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorDepositSweepProposal struct {
	WalletPubKeyHash     [20]byte
	DepositsKeys         []WalletProposalValidatorDepositKey
	SweepTxFee           *big.Int
	DepositsRevealBlocks []*big.Int
}

// WalletProposalValidatorHeartbeatProposal is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorHeartbeatProposal struct {
	WalletPubKeyHash [20]byte
	Message          []byte
}

// WalletProposalValidatorMovedFundsSweepProposal is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorMovedFundsSweepProposal struct {
	WalletPubKeyHash         [20]byte
	MovingFundsTxHash        [32]byte
	MovingFundsTxOutputIndex uint32
	MovedFundsSweepTxFee     *big.Int
}

// WalletProposalValidatorMovingFundsProposal is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorMovingFundsProposal struct {
	WalletPubKeyHash [20]byte
	TargetWallets    [][20]byte
	MovingFundsTxFee *big.Int
}

// WalletProposalValidatorRedemptionProposal is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorRedemptionProposal struct {
	WalletPubKeyHash       [20]byte
	RedeemersOutputScripts [][]byte
	RedemptionTxFee        *big.Int
}

// WalletProposalValidatorTaprootDepositExtraInfo is an auto generated low-level Go binding around an user-defined struct.
type WalletProposalValidatorTaprootDepositExtraInfo struct {
	FundingTx            BitcoinTxInfo2
	BlindingFactor       [8]byte
	WalletPubKeyHash     [20]byte
	WalletXOnlyPublicKey [32]byte
	RefundPubKeyHash     [20]byte
	RefundXOnlyPublicKey [32]byte
	RefundLocktime       [4]byte
}

// WalletProposalValidatorMetaData contains all meta data concerning the WalletProposalValidator contract.
var WalletProposalValidatorMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractBridge\",\"name\":\"_bridge\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"DEPOSIT_MIN_AGE\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"DEPOSIT_REFUND_SAFETY_MARGIN\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"DEPOSIT_SWEEP_MAX_SIZE\",\"outputs\":[{\"internalType\":\"uint16\",\"name\":\"\",\"type\":\"uint16\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"REDEMPTION_MAX_SIZE\",\"outputs\":[{\"internalType\":\"uint16\",\"name\":\"\",\"type\":\"uint16\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"REDEMPTION_REQUEST_MIN_AGE\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"bridge\",\"outputs\":[{\"internalType\":\"contractBridge\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"fundingTxHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"fundingOutputIndex\",\"type\":\"uint32\"}],\"internalType\":\"structWalletProposalValidator.DepositKey[]\",\"name\":\"depositsKeys\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256\",\"name\":\"sweepTxFee\",\"type\":\"uint256\"},{\"internalType\":\"uint256[]\",\"name\":\"depositsRevealBlocks\",\"type\":\"uint256[]\"}],\"internalType\":\"structWalletProposalValidator.DepositSweepProposal\",\"name\":\"proposal\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"fundingTx\",\"type\":\"tuple\"},{\"internalType\":\"bytes8\",\"name\":\"blindingFactor\",\"type\":\"bytes8\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes20\",\"name\":\"refundPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes4\",\"name\":\"refundLocktime\",\"type\":\"bytes4\"}],\"internalType\":\"structWalletProposalValidator.DepositExtraInfo[]\",\"name\":\"depositsExtraInfo\",\"type\":\"tuple[]\"}],\"name\":\"validateDepositSweepProposal\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes\",\"name\":\"message\",\"type\":\"bytes\"}],\"internalType\":\"structWalletProposalValidator.HeartbeatProposal\",\"name\":\"proposal\",\"type\":\"tuple\"}],\"name\":\"validateHeartbeatProposal\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes32\",\"name\":\"movingFundsTxHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTxOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"movedFundsSweepTxFee\",\"type\":\"uint256\"}],\"internalType\":\"structWalletProposalValidator.MovedFundsSweepProposal\",\"name\":\"proposal\",\"type\":\"tuple\"}],\"name\":\"validateMovedFundsSweepProposal\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes20[]\",\"name\":\"targetWallets\",\"type\":\"bytes20[]\"},{\"internalType\":\"uint256\",\"name\":\"movingFundsTxFee\",\"type\":\"uint256\"}],\"internalType\":\"structWalletProposalValidator.MovingFundsProposal\",\"name\":\"proposal\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"walletMainUtxo\",\"type\":\"tuple\"}],\"name\":\"validateMovingFundsProposal\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes[]\",\"name\":\"redeemersOutputScripts\",\"type\":\"bytes[]\"},{\"internalType\":\"uint256\",\"name\":\"redemptionTxFee\",\"type\":\"uint256\"}],\"internalType\":\"structWalletProposalValidator.RedemptionProposal\",\"name\":\"proposal\",\"type\":\"tuple\"}],\"name\":\"validateRedemptionProposal\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"fundingTxHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"fundingOutputIndex\",\"type\":\"uint32\"}],\"internalType\":\"structWalletProposalValidator.DepositKey[]\",\"name\":\"depositsKeys\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256\",\"name\":\"sweepTxFee\",\"type\":\"uint256\"},{\"internalType\":\"uint256[]\",\"name\":\"depositsRevealBlocks\",\"type\":\"uint256[]\"}],\"internalType\":\"structWalletProposalValidator.DepositSweepProposal\",\"name\":\"proposal\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"fundingTx\",\"type\":\"tuple\"},{\"internalType\":\"bytes8\",\"name\":\"blindingFactor\",\"type\":\"bytes8\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes32\",\"name\":\"walletXOnlyPublicKey\",\"type\":\"bytes32\"},{\"internalType\":\"bytes20\",\"name\":\"refundPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes32\",\"name\":\"refundXOnlyPublicKey\",\"type\":\"bytes32\"},{\"internalType\":\"bytes4\",\"name\":\"refundLocktime\",\"type\":\"bytes4\"}],\"internalType\":\"structWalletProposalValidator.TaprootDepositExtraInfo[]\",\"name\":\"depositsExtraInfo\",\"type\":\"tuple[]\"}],\"name\":\"validateTaprootDepositSweepProposal\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// WalletProposalValidatorABI is the input ABI used to generate the binding from.
// Deprecated: Use WalletProposalValidatorMetaData.ABI instead.
var WalletProposalValidatorABI = WalletProposalValidatorMetaData.ABI

// WalletProposalValidator is an auto generated Go binding around an Ethereum contract.
type WalletProposalValidator struct {
	WalletProposalValidatorCaller     // Read-only binding to the contract
	WalletProposalValidatorTransactor // Write-only binding to the contract
	WalletProposalValidatorFilterer   // Log filterer for contract events
}

// WalletProposalValidatorCaller is an auto generated read-only Go binding around an Ethereum contract.
type WalletProposalValidatorCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletProposalValidatorTransactor is an auto generated write-only Go binding around an Ethereum contract.
type WalletProposalValidatorTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletProposalValidatorFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type WalletProposalValidatorFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletProposalValidatorSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type WalletProposalValidatorSession struct {
	Contract     *WalletProposalValidator // Generic contract binding to set the session for
	CallOpts     bind.CallOpts            // Call options to use throughout this session
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// WalletProposalValidatorCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type WalletProposalValidatorCallerSession struct {
	Contract *WalletProposalValidatorCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                  // Call options to use throughout this session
}

// WalletProposalValidatorTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type WalletProposalValidatorTransactorSession struct {
	Contract     *WalletProposalValidatorTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                  // Transaction auth options to use throughout this session
}

// WalletProposalValidatorRaw is an auto generated low-level Go binding around an Ethereum contract.
type WalletProposalValidatorRaw struct {
	Contract *WalletProposalValidator // Generic contract binding to access the raw methods on
}

// WalletProposalValidatorCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type WalletProposalValidatorCallerRaw struct {
	Contract *WalletProposalValidatorCaller // Generic read-only contract binding to access the raw methods on
}

// WalletProposalValidatorTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type WalletProposalValidatorTransactorRaw struct {
	Contract *WalletProposalValidatorTransactor // Generic write-only contract binding to access the raw methods on
}

// NewWalletProposalValidator creates a new instance of WalletProposalValidator, bound to a specific deployed contract.
func NewWalletProposalValidator(address common.Address, backend bind.ContractBackend) (*WalletProposalValidator, error) {
	contract, err := bindWalletProposalValidator(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &WalletProposalValidator{WalletProposalValidatorCaller: WalletProposalValidatorCaller{contract: contract}, WalletProposalValidatorTransactor: WalletProposalValidatorTransactor{contract: contract}, WalletProposalValidatorFilterer: WalletProposalValidatorFilterer{contract: contract}}, nil
}

// NewWalletProposalValidatorCaller creates a new read-only instance of WalletProposalValidator, bound to a specific deployed contract.
func NewWalletProposalValidatorCaller(address common.Address, caller bind.ContractCaller) (*WalletProposalValidatorCaller, error) {
	contract, err := bindWalletProposalValidator(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &WalletProposalValidatorCaller{contract: contract}, nil
}

// NewWalletProposalValidatorTransactor creates a new write-only instance of WalletProposalValidator, bound to a specific deployed contract.
func NewWalletProposalValidatorTransactor(address common.Address, transactor bind.ContractTransactor) (*WalletProposalValidatorTransactor, error) {
	contract, err := bindWalletProposalValidator(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &WalletProposalValidatorTransactor{contract: contract}, nil
}

// NewWalletProposalValidatorFilterer creates a new log filterer instance of WalletProposalValidator, bound to a specific deployed contract.
func NewWalletProposalValidatorFilterer(address common.Address, filterer bind.ContractFilterer) (*WalletProposalValidatorFilterer, error) {
	contract, err := bindWalletProposalValidator(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &WalletProposalValidatorFilterer{contract: contract}, nil
}

// bindWalletProposalValidator binds a generic wrapper to an already deployed contract.
func bindWalletProposalValidator(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := WalletProposalValidatorMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_WalletProposalValidator *WalletProposalValidatorRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _WalletProposalValidator.Contract.WalletProposalValidatorCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_WalletProposalValidator *WalletProposalValidatorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletProposalValidator.Contract.WalletProposalValidatorTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_WalletProposalValidator *WalletProposalValidatorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _WalletProposalValidator.Contract.WalletProposalValidatorTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_WalletProposalValidator *WalletProposalValidatorCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _WalletProposalValidator.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_WalletProposalValidator *WalletProposalValidatorTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletProposalValidator.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_WalletProposalValidator *WalletProposalValidatorTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _WalletProposalValidator.Contract.contract.Transact(opts, method, params...)
}

// DEPOSITMINAGE is a free data retrieval call binding the contract method 0x6b562cec.
//
// Solidity: function DEPOSIT_MIN_AGE() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCaller) DEPOSITMINAGE(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "DEPOSIT_MIN_AGE")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// DEPOSITMINAGE is a free data retrieval call binding the contract method 0x6b562cec.
//
// Solidity: function DEPOSIT_MIN_AGE() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorSession) DEPOSITMINAGE() (uint32, error) {
	return _WalletProposalValidator.Contract.DEPOSITMINAGE(&_WalletProposalValidator.CallOpts)
}

// DEPOSITMINAGE is a free data retrieval call binding the contract method 0x6b562cec.
//
// Solidity: function DEPOSIT_MIN_AGE() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) DEPOSITMINAGE() (uint32, error) {
	return _WalletProposalValidator.Contract.DEPOSITMINAGE(&_WalletProposalValidator.CallOpts)
}

// DEPOSITREFUNDSAFETYMARGIN is a free data retrieval call binding the contract method 0x765f28f9.
//
// Solidity: function DEPOSIT_REFUND_SAFETY_MARGIN() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCaller) DEPOSITREFUNDSAFETYMARGIN(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "DEPOSIT_REFUND_SAFETY_MARGIN")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// DEPOSITREFUNDSAFETYMARGIN is a free data retrieval call binding the contract method 0x765f28f9.
//
// Solidity: function DEPOSIT_REFUND_SAFETY_MARGIN() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorSession) DEPOSITREFUNDSAFETYMARGIN() (uint32, error) {
	return _WalletProposalValidator.Contract.DEPOSITREFUNDSAFETYMARGIN(&_WalletProposalValidator.CallOpts)
}

// DEPOSITREFUNDSAFETYMARGIN is a free data retrieval call binding the contract method 0x765f28f9.
//
// Solidity: function DEPOSIT_REFUND_SAFETY_MARGIN() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) DEPOSITREFUNDSAFETYMARGIN() (uint32, error) {
	return _WalletProposalValidator.Contract.DEPOSITREFUNDSAFETYMARGIN(&_WalletProposalValidator.CallOpts)
}

// DEPOSITSWEEPMAXSIZE is a free data retrieval call binding the contract method 0x996a756b.
//
// Solidity: function DEPOSIT_SWEEP_MAX_SIZE() view returns(uint16)
func (_WalletProposalValidator *WalletProposalValidatorCaller) DEPOSITSWEEPMAXSIZE(opts *bind.CallOpts) (uint16, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "DEPOSIT_SWEEP_MAX_SIZE")

	if err != nil {
		return *new(uint16), err
	}

	out0 := *abi.ConvertType(out[0], new(uint16)).(*uint16)

	return out0, err

}

// DEPOSITSWEEPMAXSIZE is a free data retrieval call binding the contract method 0x996a756b.
//
// Solidity: function DEPOSIT_SWEEP_MAX_SIZE() view returns(uint16)
func (_WalletProposalValidator *WalletProposalValidatorSession) DEPOSITSWEEPMAXSIZE() (uint16, error) {
	return _WalletProposalValidator.Contract.DEPOSITSWEEPMAXSIZE(&_WalletProposalValidator.CallOpts)
}

// DEPOSITSWEEPMAXSIZE is a free data retrieval call binding the contract method 0x996a756b.
//
// Solidity: function DEPOSIT_SWEEP_MAX_SIZE() view returns(uint16)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) DEPOSITSWEEPMAXSIZE() (uint16, error) {
	return _WalletProposalValidator.Contract.DEPOSITSWEEPMAXSIZE(&_WalletProposalValidator.CallOpts)
}

// REDEMPTIONMAXSIZE is a free data retrieval call binding the contract method 0xe0276b26.
//
// Solidity: function REDEMPTION_MAX_SIZE() view returns(uint16)
func (_WalletProposalValidator *WalletProposalValidatorCaller) REDEMPTIONMAXSIZE(opts *bind.CallOpts) (uint16, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "REDEMPTION_MAX_SIZE")

	if err != nil {
		return *new(uint16), err
	}

	out0 := *abi.ConvertType(out[0], new(uint16)).(*uint16)

	return out0, err

}

// REDEMPTIONMAXSIZE is a free data retrieval call binding the contract method 0xe0276b26.
//
// Solidity: function REDEMPTION_MAX_SIZE() view returns(uint16)
func (_WalletProposalValidator *WalletProposalValidatorSession) REDEMPTIONMAXSIZE() (uint16, error) {
	return _WalletProposalValidator.Contract.REDEMPTIONMAXSIZE(&_WalletProposalValidator.CallOpts)
}

// REDEMPTIONMAXSIZE is a free data retrieval call binding the contract method 0xe0276b26.
//
// Solidity: function REDEMPTION_MAX_SIZE() view returns(uint16)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) REDEMPTIONMAXSIZE() (uint16, error) {
	return _WalletProposalValidator.Contract.REDEMPTIONMAXSIZE(&_WalletProposalValidator.CallOpts)
}

// REDEMPTIONREQUESTMINAGE is a free data retrieval call binding the contract method 0xd09520f7.
//
// Solidity: function REDEMPTION_REQUEST_MIN_AGE() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCaller) REDEMPTIONREQUESTMINAGE(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "REDEMPTION_REQUEST_MIN_AGE")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// REDEMPTIONREQUESTMINAGE is a free data retrieval call binding the contract method 0xd09520f7.
//
// Solidity: function REDEMPTION_REQUEST_MIN_AGE() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorSession) REDEMPTIONREQUESTMINAGE() (uint32, error) {
	return _WalletProposalValidator.Contract.REDEMPTIONREQUESTMINAGE(&_WalletProposalValidator.CallOpts)
}

// REDEMPTIONREQUESTMINAGE is a free data retrieval call binding the contract method 0xd09520f7.
//
// Solidity: function REDEMPTION_REQUEST_MIN_AGE() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) REDEMPTIONREQUESTMINAGE() (uint32, error) {
	return _WalletProposalValidator.Contract.REDEMPTIONREQUESTMINAGE(&_WalletProposalValidator.CallOpts)
}

// REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN is a free data retrieval call binding the contract method 0x0538ac1b.
//
// Solidity: function REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCaller) REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN is a free data retrieval call binding the contract method 0x0538ac1b.
//
// Solidity: function REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorSession) REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN() (uint32, error) {
	return _WalletProposalValidator.Contract.REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN(&_WalletProposalValidator.CallOpts)
}

// REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN is a free data retrieval call binding the contract method 0x0538ac1b.
//
// Solidity: function REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN() view returns(uint32)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN() (uint32, error) {
	return _WalletProposalValidator.Contract.REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN(&_WalletProposalValidator.CallOpts)
}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_WalletProposalValidator *WalletProposalValidatorCaller) Bridge(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "bridge")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_WalletProposalValidator *WalletProposalValidatorSession) Bridge() (common.Address, error) {
	return _WalletProposalValidator.Contract.Bridge(&_WalletProposalValidator.CallOpts)
}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) Bridge() (common.Address, error) {
	return _WalletProposalValidator.Contract.Bridge(&_WalletProposalValidator.CallOpts)
}

// ValidateDepositSweepProposal is a free data retrieval call binding the contract method 0x7405d7bf.
//
// Solidity: function validateDepositSweepProposal((bytes20,(bytes32,uint32)[],uint256,uint256[]) proposal, ((bytes4,bytes,bytes,bytes4),bytes8,bytes20,bytes20,bytes4)[] depositsExtraInfo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCaller) ValidateDepositSweepProposal(opts *bind.CallOpts, proposal WalletProposalValidatorDepositSweepProposal, depositsExtraInfo []WalletProposalValidatorDepositExtraInfo) (bool, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "validateDepositSweepProposal", proposal, depositsExtraInfo)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateDepositSweepProposal is a free data retrieval call binding the contract method 0x7405d7bf.
//
// Solidity: function validateDepositSweepProposal((bytes20,(bytes32,uint32)[],uint256,uint256[]) proposal, ((bytes4,bytes,bytes,bytes4),bytes8,bytes20,bytes20,bytes4)[] depositsExtraInfo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorSession) ValidateDepositSweepProposal(proposal WalletProposalValidatorDepositSweepProposal, depositsExtraInfo []WalletProposalValidatorDepositExtraInfo) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateDepositSweepProposal(&_WalletProposalValidator.CallOpts, proposal, depositsExtraInfo)
}

// ValidateDepositSweepProposal is a free data retrieval call binding the contract method 0x7405d7bf.
//
// Solidity: function validateDepositSweepProposal((bytes20,(bytes32,uint32)[],uint256,uint256[]) proposal, ((bytes4,bytes,bytes,bytes4),bytes8,bytes20,bytes20,bytes4)[] depositsExtraInfo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) ValidateDepositSweepProposal(proposal WalletProposalValidatorDepositSweepProposal, depositsExtraInfo []WalletProposalValidatorDepositExtraInfo) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateDepositSweepProposal(&_WalletProposalValidator.CallOpts, proposal, depositsExtraInfo)
}

// ValidateHeartbeatProposal is a free data retrieval call binding the contract method 0xf03a4bc5.
//
// Solidity: function validateHeartbeatProposal((bytes20,bytes) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCaller) ValidateHeartbeatProposal(opts *bind.CallOpts, proposal WalletProposalValidatorHeartbeatProposal) (bool, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "validateHeartbeatProposal", proposal)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateHeartbeatProposal is a free data retrieval call binding the contract method 0xf03a4bc5.
//
// Solidity: function validateHeartbeatProposal((bytes20,bytes) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorSession) ValidateHeartbeatProposal(proposal WalletProposalValidatorHeartbeatProposal) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateHeartbeatProposal(&_WalletProposalValidator.CallOpts, proposal)
}

// ValidateHeartbeatProposal is a free data retrieval call binding the contract method 0xf03a4bc5.
//
// Solidity: function validateHeartbeatProposal((bytes20,bytes) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) ValidateHeartbeatProposal(proposal WalletProposalValidatorHeartbeatProposal) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateHeartbeatProposal(&_WalletProposalValidator.CallOpts, proposal)
}

// ValidateMovedFundsSweepProposal is a free data retrieval call binding the contract method 0xd6c7fa78.
//
// Solidity: function validateMovedFundsSweepProposal((bytes20,bytes32,uint32,uint256) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCaller) ValidateMovedFundsSweepProposal(opts *bind.CallOpts, proposal WalletProposalValidatorMovedFundsSweepProposal) (bool, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "validateMovedFundsSweepProposal", proposal)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateMovedFundsSweepProposal is a free data retrieval call binding the contract method 0xd6c7fa78.
//
// Solidity: function validateMovedFundsSweepProposal((bytes20,bytes32,uint32,uint256) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorSession) ValidateMovedFundsSweepProposal(proposal WalletProposalValidatorMovedFundsSweepProposal) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateMovedFundsSweepProposal(&_WalletProposalValidator.CallOpts, proposal)
}

// ValidateMovedFundsSweepProposal is a free data retrieval call binding the contract method 0xd6c7fa78.
//
// Solidity: function validateMovedFundsSweepProposal((bytes20,bytes32,uint32,uint256) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) ValidateMovedFundsSweepProposal(proposal WalletProposalValidatorMovedFundsSweepProposal) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateMovedFundsSweepProposal(&_WalletProposalValidator.CallOpts, proposal)
}

// ValidateMovingFundsProposal is a free data retrieval call binding the contract method 0x9b337db2.
//
// Solidity: function validateMovingFundsProposal((bytes20,bytes20[],uint256) proposal, (bytes32,uint32,uint64) walletMainUtxo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCaller) ValidateMovingFundsProposal(opts *bind.CallOpts, proposal WalletProposalValidatorMovingFundsProposal, walletMainUtxo BitcoinTxUTXO3) (bool, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "validateMovingFundsProposal", proposal, walletMainUtxo)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateMovingFundsProposal is a free data retrieval call binding the contract method 0x9b337db2.
//
// Solidity: function validateMovingFundsProposal((bytes20,bytes20[],uint256) proposal, (bytes32,uint32,uint64) walletMainUtxo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorSession) ValidateMovingFundsProposal(proposal WalletProposalValidatorMovingFundsProposal, walletMainUtxo BitcoinTxUTXO3) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateMovingFundsProposal(&_WalletProposalValidator.CallOpts, proposal, walletMainUtxo)
}

// ValidateMovingFundsProposal is a free data retrieval call binding the contract method 0x9b337db2.
//
// Solidity: function validateMovingFundsProposal((bytes20,bytes20[],uint256) proposal, (bytes32,uint32,uint64) walletMainUtxo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) ValidateMovingFundsProposal(proposal WalletProposalValidatorMovingFundsProposal, walletMainUtxo BitcoinTxUTXO3) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateMovingFundsProposal(&_WalletProposalValidator.CallOpts, proposal, walletMainUtxo)
}

// ValidateRedemptionProposal is a free data retrieval call binding the contract method 0x0fde6c76.
//
// Solidity: function validateRedemptionProposal((bytes20,bytes[],uint256) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCaller) ValidateRedemptionProposal(opts *bind.CallOpts, proposal WalletProposalValidatorRedemptionProposal) (bool, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "validateRedemptionProposal", proposal)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateRedemptionProposal is a free data retrieval call binding the contract method 0x0fde6c76.
//
// Solidity: function validateRedemptionProposal((bytes20,bytes[],uint256) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorSession) ValidateRedemptionProposal(proposal WalletProposalValidatorRedemptionProposal) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateRedemptionProposal(&_WalletProposalValidator.CallOpts, proposal)
}

// ValidateRedemptionProposal is a free data retrieval call binding the contract method 0x0fde6c76.
//
// Solidity: function validateRedemptionProposal((bytes20,bytes[],uint256) proposal) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) ValidateRedemptionProposal(proposal WalletProposalValidatorRedemptionProposal) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateRedemptionProposal(&_WalletProposalValidator.CallOpts, proposal)
}

// ValidateTaprootDepositSweepProposal is a free data retrieval call binding the contract method 0xb1782302.
//
// Solidity: function validateTaprootDepositSweepProposal((bytes20,(bytes32,uint32)[],uint256,uint256[]) proposal, ((bytes4,bytes,bytes,bytes4),bytes8,bytes20,bytes32,bytes20,bytes32,bytes4)[] depositsExtraInfo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCaller) ValidateTaprootDepositSweepProposal(opts *bind.CallOpts, proposal WalletProposalValidatorDepositSweepProposal, depositsExtraInfo []WalletProposalValidatorTaprootDepositExtraInfo) (bool, error) {
	var out []interface{}
	err := _WalletProposalValidator.contract.Call(opts, &out, "validateTaprootDepositSweepProposal", proposal, depositsExtraInfo)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ValidateTaprootDepositSweepProposal is a free data retrieval call binding the contract method 0xb1782302.
//
// Solidity: function validateTaprootDepositSweepProposal((bytes20,(bytes32,uint32)[],uint256,uint256[]) proposal, ((bytes4,bytes,bytes,bytes4),bytes8,bytes20,bytes32,bytes20,bytes32,bytes4)[] depositsExtraInfo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorSession) ValidateTaprootDepositSweepProposal(proposal WalletProposalValidatorDepositSweepProposal, depositsExtraInfo []WalletProposalValidatorTaprootDepositExtraInfo) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateTaprootDepositSweepProposal(&_WalletProposalValidator.CallOpts, proposal, depositsExtraInfo)
}

// ValidateTaprootDepositSweepProposal is a free data retrieval call binding the contract method 0xb1782302.
//
// Solidity: function validateTaprootDepositSweepProposal((bytes20,(bytes32,uint32)[],uint256,uint256[]) proposal, ((bytes4,bytes,bytes,bytes4),bytes8,bytes20,bytes32,bytes20,bytes32,bytes4)[] depositsExtraInfo) view returns(bool)
func (_WalletProposalValidator *WalletProposalValidatorCallerSession) ValidateTaprootDepositSweepProposal(proposal WalletProposalValidatorDepositSweepProposal, depositsExtraInfo []WalletProposalValidatorTaprootDepositExtraInfo) (bool, error) {
	return _WalletProposalValidator.Contract.ValidateTaprootDepositSweepProposal(&_WalletProposalValidator.CallOpts, proposal, depositsExtraInfo)
}

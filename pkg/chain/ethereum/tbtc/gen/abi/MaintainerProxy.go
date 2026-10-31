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

// BitcoinTxInfo3 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxInfo3 struct {
	Version      [4]byte
	InputVector  []byte
	OutputVector []byte
	Locktime     [4]byte
}

// BitcoinTxProof2 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxProof2 struct {
	MerkleProof      []byte
	TxIndexInBlock   *big.Int
	BitcoinHeaders   []byte
	CoinbasePreimage [32]byte
	CoinbaseProof    []byte
}

// BitcoinTxUTXO2 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxUTXO2 struct {
	TxHash        [32]byte
	TxOutputIndex uint32
	TxOutputValue uint64
}

// MaintainerProxyMetaData contains all meta data concerning the MaintainerProxy contract.
var MaintainerProxyMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"contractBridge\",\"name\":\"_bridge\",\"type\":\"address\"},{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newBridge\",\"type\":\"address\"}],\"name\":\"BridgeUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"submitDepositSweepProofGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"submitRedemptionProofGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"resetMovingFundsTimeoutGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"submitMovingFundsProofGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyMovingFundsBelowDustGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"submitMovedFundsSweepProofGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"requestNewWalletGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyWalletCloseableGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"notifyWalletClosingPeriodElapsedGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"defeatFraudChallengeGasOffset\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"defeatFraudChallengeWithHeartbeatGasOffset\",\"type\":\"uint256\"}],\"name\":\"GasOffsetParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newReimbursementPool\",\"type\":\"address\"}],\"name\":\"ReimbursementPoolUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"SpvMaintainerAuthorized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"SpvMaintainerUnauthorized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"WalletMaintainerAuthorized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"WalletMaintainerUnauthorized\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"allSpvMaintainers\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"allWalletMaintainers\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"authorizeSpvMaintainer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"maintainer\",\"type\":\"address\"}],\"name\":\"authorizeWalletMaintainer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"bridge\",\"outputs\":[{\"internalType\":\"contractBridge\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"walletPublicKey\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"preimage\",\"type\":\"bytes\"},{\"internalType\":\"bool\",\"name\":\"witness\",\"type\":\"bool\"}],\"name\":\"defeatFraudChallenge\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"defeatFraudChallengeGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"walletPublicKey\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"heartbeatMessage\",\"type\":\"bytes\"}],\"name\":\"defeatFraudChallengeWithHeartbeat\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"defeatFraudChallengeWithHeartbeatGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isSpvMaintainer\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isWalletMaintainer\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"}],\"name\":\"notifyMovingFundsBelowDust\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifyMovingFundsBelowDustGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"walletMainUtxo\",\"type\":\"tuple\"}],\"name\":\"notifyWalletCloseable\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifyWalletCloseableGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"notifyWalletClosingPeriodElapsed\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"notifyWalletClosingPeriodElapsedGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reimbursementPool\",\"outputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"activeWalletMainUtxo\",\"type\":\"tuple\"}],\"name\":\"requestNewWallet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"requestNewWalletGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"resetMovingFundsTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"resetMovingFundsTimeoutGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"spvMaintainers\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"sweepTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"sweepProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"}],\"name\":\"submitDepositSweepProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"submitDepositSweepProofGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"sweepTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"sweepProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"}],\"name\":\"submitMovedFundsSweepProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"submitMovedFundsSweepProofGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"movingFundsTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"movingFundsProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"submitMovingFundsProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"submitMovingFundsProofGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"redemptionTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"redemptionProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"submitRedemptionProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"submitRedemptionProofGasOffset\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"maintainerToUnauthorize\",\"type\":\"address\"}],\"name\":\"unauthorizeSpvMaintainer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"maintainerToUnauthorize\",\"type\":\"address\"}],\"name\":\"unauthorizeWalletMaintainer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractBridge\",\"name\":\"_bridge\",\"type\":\"address\"}],\"name\":\"updateBridge\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newSubmitDepositSweepProofGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newSubmitRedemptionProofGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newResetMovingFundsTimeoutGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newSubmitMovingFundsProofGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newNotifyMovingFundsBelowDustGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newSubmitMovedFundsSweepProofGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newRequestNewWalletGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newNotifyWalletCloseableGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newNotifyWalletClosingPeriodElapsedGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newDefeatFraudChallengeGasOffset\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"newDefeatFraudChallengeWithHeartbeatGasOffset\",\"type\":\"uint256\"}],\"name\":\"updateGasOffsetParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractReimbursementPool\",\"name\":\"_reimbursementPool\",\"type\":\"address\"}],\"name\":\"updateReimbursementPool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"walletMaintainers\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// MaintainerProxyABI is the input ABI used to generate the binding from.
// Deprecated: Use MaintainerProxyMetaData.ABI instead.
var MaintainerProxyABI = MaintainerProxyMetaData.ABI

// MaintainerProxy is an auto generated Go binding around an Ethereum contract.
type MaintainerProxy struct {
	MaintainerProxyCaller     // Read-only binding to the contract
	MaintainerProxyTransactor // Write-only binding to the contract
	MaintainerProxyFilterer   // Log filterer for contract events
}

// MaintainerProxyCaller is an auto generated read-only Go binding around an Ethereum contract.
type MaintainerProxyCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MaintainerProxyTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MaintainerProxyTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MaintainerProxyFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type MaintainerProxyFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MaintainerProxySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type MaintainerProxySession struct {
	Contract     *MaintainerProxy  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// MaintainerProxyCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type MaintainerProxyCallerSession struct {
	Contract *MaintainerProxyCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// MaintainerProxyTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type MaintainerProxyTransactorSession struct {
	Contract     *MaintainerProxyTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// MaintainerProxyRaw is an auto generated low-level Go binding around an Ethereum contract.
type MaintainerProxyRaw struct {
	Contract *MaintainerProxy // Generic contract binding to access the raw methods on
}

// MaintainerProxyCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type MaintainerProxyCallerRaw struct {
	Contract *MaintainerProxyCaller // Generic read-only contract binding to access the raw methods on
}

// MaintainerProxyTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type MaintainerProxyTransactorRaw struct {
	Contract *MaintainerProxyTransactor // Generic write-only contract binding to access the raw methods on
}

// NewMaintainerProxy creates a new instance of MaintainerProxy, bound to a specific deployed contract.
func NewMaintainerProxy(address common.Address, backend bind.ContractBackend) (*MaintainerProxy, error) {
	contract, err := bindMaintainerProxy(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxy{MaintainerProxyCaller: MaintainerProxyCaller{contract: contract}, MaintainerProxyTransactor: MaintainerProxyTransactor{contract: contract}, MaintainerProxyFilterer: MaintainerProxyFilterer{contract: contract}}, nil
}

// NewMaintainerProxyCaller creates a new read-only instance of MaintainerProxy, bound to a specific deployed contract.
func NewMaintainerProxyCaller(address common.Address, caller bind.ContractCaller) (*MaintainerProxyCaller, error) {
	contract, err := bindMaintainerProxy(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyCaller{contract: contract}, nil
}

// NewMaintainerProxyTransactor creates a new write-only instance of MaintainerProxy, bound to a specific deployed contract.
func NewMaintainerProxyTransactor(address common.Address, transactor bind.ContractTransactor) (*MaintainerProxyTransactor, error) {
	contract, err := bindMaintainerProxy(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyTransactor{contract: contract}, nil
}

// NewMaintainerProxyFilterer creates a new log filterer instance of MaintainerProxy, bound to a specific deployed contract.
func NewMaintainerProxyFilterer(address common.Address, filterer bind.ContractFilterer) (*MaintainerProxyFilterer, error) {
	contract, err := bindMaintainerProxy(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyFilterer{contract: contract}, nil
}

// bindMaintainerProxy binds a generic wrapper to an already deployed contract.
func bindMaintainerProxy(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := MaintainerProxyMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MaintainerProxy *MaintainerProxyRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MaintainerProxy.Contract.MaintainerProxyCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MaintainerProxy *MaintainerProxyRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.MaintainerProxyTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MaintainerProxy *MaintainerProxyRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.MaintainerProxyTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MaintainerProxy *MaintainerProxyCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MaintainerProxy.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MaintainerProxy *MaintainerProxyTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MaintainerProxy *MaintainerProxyTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.contract.Transact(opts, method, params...)
}

// AllSpvMaintainers is a free data retrieval call binding the contract method 0x13f1a99f.
//
// Solidity: function allSpvMaintainers() view returns(address[])
func (_MaintainerProxy *MaintainerProxyCaller) AllSpvMaintainers(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "allSpvMaintainers")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// AllSpvMaintainers is a free data retrieval call binding the contract method 0x13f1a99f.
//
// Solidity: function allSpvMaintainers() view returns(address[])
func (_MaintainerProxy *MaintainerProxySession) AllSpvMaintainers() ([]common.Address, error) {
	return _MaintainerProxy.Contract.AllSpvMaintainers(&_MaintainerProxy.CallOpts)
}

// AllSpvMaintainers is a free data retrieval call binding the contract method 0x13f1a99f.
//
// Solidity: function allSpvMaintainers() view returns(address[])
func (_MaintainerProxy *MaintainerProxyCallerSession) AllSpvMaintainers() ([]common.Address, error) {
	return _MaintainerProxy.Contract.AllSpvMaintainers(&_MaintainerProxy.CallOpts)
}

// AllWalletMaintainers is a free data retrieval call binding the contract method 0x2f5e2af7.
//
// Solidity: function allWalletMaintainers() view returns(address[])
func (_MaintainerProxy *MaintainerProxyCaller) AllWalletMaintainers(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "allWalletMaintainers")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// AllWalletMaintainers is a free data retrieval call binding the contract method 0x2f5e2af7.
//
// Solidity: function allWalletMaintainers() view returns(address[])
func (_MaintainerProxy *MaintainerProxySession) AllWalletMaintainers() ([]common.Address, error) {
	return _MaintainerProxy.Contract.AllWalletMaintainers(&_MaintainerProxy.CallOpts)
}

// AllWalletMaintainers is a free data retrieval call binding the contract method 0x2f5e2af7.
//
// Solidity: function allWalletMaintainers() view returns(address[])
func (_MaintainerProxy *MaintainerProxyCallerSession) AllWalletMaintainers() ([]common.Address, error) {
	return _MaintainerProxy.Contract.AllWalletMaintainers(&_MaintainerProxy.CallOpts)
}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_MaintainerProxy *MaintainerProxyCaller) Bridge(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "bridge")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_MaintainerProxy *MaintainerProxySession) Bridge() (common.Address, error) {
	return _MaintainerProxy.Contract.Bridge(&_MaintainerProxy.CallOpts)
}

// Bridge is a free data retrieval call binding the contract method 0xe78cea92.
//
// Solidity: function bridge() view returns(address)
func (_MaintainerProxy *MaintainerProxyCallerSession) Bridge() (common.Address, error) {
	return _MaintainerProxy.Contract.Bridge(&_MaintainerProxy.CallOpts)
}

// DefeatFraudChallengeGasOffset is a free data retrieval call binding the contract method 0x89065e60.
//
// Solidity: function defeatFraudChallengeGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) DefeatFraudChallengeGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "defeatFraudChallengeGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// DefeatFraudChallengeGasOffset is a free data retrieval call binding the contract method 0x89065e60.
//
// Solidity: function defeatFraudChallengeGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) DefeatFraudChallengeGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallengeGasOffset(&_MaintainerProxy.CallOpts)
}

// DefeatFraudChallengeGasOffset is a free data retrieval call binding the contract method 0x89065e60.
//
// Solidity: function defeatFraudChallengeGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) DefeatFraudChallengeGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallengeGasOffset(&_MaintainerProxy.CallOpts)
}

// DefeatFraudChallengeWithHeartbeatGasOffset is a free data retrieval call binding the contract method 0x84862c19.
//
// Solidity: function defeatFraudChallengeWithHeartbeatGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) DefeatFraudChallengeWithHeartbeatGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "defeatFraudChallengeWithHeartbeatGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// DefeatFraudChallengeWithHeartbeatGasOffset is a free data retrieval call binding the contract method 0x84862c19.
//
// Solidity: function defeatFraudChallengeWithHeartbeatGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) DefeatFraudChallengeWithHeartbeatGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallengeWithHeartbeatGasOffset(&_MaintainerProxy.CallOpts)
}

// DefeatFraudChallengeWithHeartbeatGasOffset is a free data retrieval call binding the contract method 0x84862c19.
//
// Solidity: function defeatFraudChallengeWithHeartbeatGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) DefeatFraudChallengeWithHeartbeatGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallengeWithHeartbeatGasOffset(&_MaintainerProxy.CallOpts)
}

// IsSpvMaintainer is a free data retrieval call binding the contract method 0xf5c3c084.
//
// Solidity: function isSpvMaintainer(address ) view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) IsSpvMaintainer(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "isSpvMaintainer", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// IsSpvMaintainer is a free data retrieval call binding the contract method 0xf5c3c084.
//
// Solidity: function isSpvMaintainer(address ) view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) IsSpvMaintainer(arg0 common.Address) (*big.Int, error) {
	return _MaintainerProxy.Contract.IsSpvMaintainer(&_MaintainerProxy.CallOpts, arg0)
}

// IsSpvMaintainer is a free data retrieval call binding the contract method 0xf5c3c084.
//
// Solidity: function isSpvMaintainer(address ) view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) IsSpvMaintainer(arg0 common.Address) (*big.Int, error) {
	return _MaintainerProxy.Contract.IsSpvMaintainer(&_MaintainerProxy.CallOpts, arg0)
}

// IsWalletMaintainer is a free data retrieval call binding the contract method 0xca05b902.
//
// Solidity: function isWalletMaintainer(address ) view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) IsWalletMaintainer(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "isWalletMaintainer", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// IsWalletMaintainer is a free data retrieval call binding the contract method 0xca05b902.
//
// Solidity: function isWalletMaintainer(address ) view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) IsWalletMaintainer(arg0 common.Address) (*big.Int, error) {
	return _MaintainerProxy.Contract.IsWalletMaintainer(&_MaintainerProxy.CallOpts, arg0)
}

// IsWalletMaintainer is a free data retrieval call binding the contract method 0xca05b902.
//
// Solidity: function isWalletMaintainer(address ) view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) IsWalletMaintainer(arg0 common.Address) (*big.Int, error) {
	return _MaintainerProxy.Contract.IsWalletMaintainer(&_MaintainerProxy.CallOpts, arg0)
}

// NotifyMovingFundsBelowDustGasOffset is a free data retrieval call binding the contract method 0xe3cfecd1.
//
// Solidity: function notifyMovingFundsBelowDustGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) NotifyMovingFundsBelowDustGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "notifyMovingFundsBelowDustGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NotifyMovingFundsBelowDustGasOffset is a free data retrieval call binding the contract method 0xe3cfecd1.
//
// Solidity: function notifyMovingFundsBelowDustGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) NotifyMovingFundsBelowDustGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.NotifyMovingFundsBelowDustGasOffset(&_MaintainerProxy.CallOpts)
}

// NotifyMovingFundsBelowDustGasOffset is a free data retrieval call binding the contract method 0xe3cfecd1.
//
// Solidity: function notifyMovingFundsBelowDustGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) NotifyMovingFundsBelowDustGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.NotifyMovingFundsBelowDustGasOffset(&_MaintainerProxy.CallOpts)
}

// NotifyWalletCloseableGasOffset is a free data retrieval call binding the contract method 0xe46e6eda.
//
// Solidity: function notifyWalletCloseableGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) NotifyWalletCloseableGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "notifyWalletCloseableGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NotifyWalletCloseableGasOffset is a free data retrieval call binding the contract method 0xe46e6eda.
//
// Solidity: function notifyWalletCloseableGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) NotifyWalletCloseableGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.NotifyWalletCloseableGasOffset(&_MaintainerProxy.CallOpts)
}

// NotifyWalletCloseableGasOffset is a free data retrieval call binding the contract method 0xe46e6eda.
//
// Solidity: function notifyWalletCloseableGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) NotifyWalletCloseableGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.NotifyWalletCloseableGasOffset(&_MaintainerProxy.CallOpts)
}

// NotifyWalletClosingPeriodElapsedGasOffset is a free data retrieval call binding the contract method 0x93b743bd.
//
// Solidity: function notifyWalletClosingPeriodElapsedGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) NotifyWalletClosingPeriodElapsedGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "notifyWalletClosingPeriodElapsedGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NotifyWalletClosingPeriodElapsedGasOffset is a free data retrieval call binding the contract method 0x93b743bd.
//
// Solidity: function notifyWalletClosingPeriodElapsedGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) NotifyWalletClosingPeriodElapsedGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.NotifyWalletClosingPeriodElapsedGasOffset(&_MaintainerProxy.CallOpts)
}

// NotifyWalletClosingPeriodElapsedGasOffset is a free data retrieval call binding the contract method 0x93b743bd.
//
// Solidity: function notifyWalletClosingPeriodElapsedGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) NotifyWalletClosingPeriodElapsedGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.NotifyWalletClosingPeriodElapsedGasOffset(&_MaintainerProxy.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_MaintainerProxy *MaintainerProxyCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_MaintainerProxy *MaintainerProxySession) Owner() (common.Address, error) {
	return _MaintainerProxy.Contract.Owner(&_MaintainerProxy.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_MaintainerProxy *MaintainerProxyCallerSession) Owner() (common.Address, error) {
	return _MaintainerProxy.Contract.Owner(&_MaintainerProxy.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_MaintainerProxy *MaintainerProxyCaller) ReimbursementPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "reimbursementPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_MaintainerProxy *MaintainerProxySession) ReimbursementPool() (common.Address, error) {
	return _MaintainerProxy.Contract.ReimbursementPool(&_MaintainerProxy.CallOpts)
}

// ReimbursementPool is a free data retrieval call binding the contract method 0xc09975cd.
//
// Solidity: function reimbursementPool() view returns(address)
func (_MaintainerProxy *MaintainerProxyCallerSession) ReimbursementPool() (common.Address, error) {
	return _MaintainerProxy.Contract.ReimbursementPool(&_MaintainerProxy.CallOpts)
}

// RequestNewWalletGasOffset is a free data retrieval call binding the contract method 0x19fd5c45.
//
// Solidity: function requestNewWalletGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) RequestNewWalletGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "requestNewWalletGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RequestNewWalletGasOffset is a free data retrieval call binding the contract method 0x19fd5c45.
//
// Solidity: function requestNewWalletGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) RequestNewWalletGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.RequestNewWalletGasOffset(&_MaintainerProxy.CallOpts)
}

// RequestNewWalletGasOffset is a free data retrieval call binding the contract method 0x19fd5c45.
//
// Solidity: function requestNewWalletGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) RequestNewWalletGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.RequestNewWalletGasOffset(&_MaintainerProxy.CallOpts)
}

// ResetMovingFundsTimeoutGasOffset is a free data retrieval call binding the contract method 0x391f7170.
//
// Solidity: function resetMovingFundsTimeoutGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) ResetMovingFundsTimeoutGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "resetMovingFundsTimeoutGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ResetMovingFundsTimeoutGasOffset is a free data retrieval call binding the contract method 0x391f7170.
//
// Solidity: function resetMovingFundsTimeoutGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) ResetMovingFundsTimeoutGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.ResetMovingFundsTimeoutGasOffset(&_MaintainerProxy.CallOpts)
}

// ResetMovingFundsTimeoutGasOffset is a free data retrieval call binding the contract method 0x391f7170.
//
// Solidity: function resetMovingFundsTimeoutGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) ResetMovingFundsTimeoutGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.ResetMovingFundsTimeoutGasOffset(&_MaintainerProxy.CallOpts)
}

// SpvMaintainers is a free data retrieval call binding the contract method 0x4b0e4571.
//
// Solidity: function spvMaintainers(uint256 ) view returns(address)
func (_MaintainerProxy *MaintainerProxyCaller) SpvMaintainers(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "spvMaintainers", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SpvMaintainers is a free data retrieval call binding the contract method 0x4b0e4571.
//
// Solidity: function spvMaintainers(uint256 ) view returns(address)
func (_MaintainerProxy *MaintainerProxySession) SpvMaintainers(arg0 *big.Int) (common.Address, error) {
	return _MaintainerProxy.Contract.SpvMaintainers(&_MaintainerProxy.CallOpts, arg0)
}

// SpvMaintainers is a free data retrieval call binding the contract method 0x4b0e4571.
//
// Solidity: function spvMaintainers(uint256 ) view returns(address)
func (_MaintainerProxy *MaintainerProxyCallerSession) SpvMaintainers(arg0 *big.Int) (common.Address, error) {
	return _MaintainerProxy.Contract.SpvMaintainers(&_MaintainerProxy.CallOpts, arg0)
}

// SubmitDepositSweepProofGasOffset is a free data retrieval call binding the contract method 0x55e19036.
//
// Solidity: function submitDepositSweepProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) SubmitDepositSweepProofGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "submitDepositSweepProofGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SubmitDepositSweepProofGasOffset is a free data retrieval call binding the contract method 0x55e19036.
//
// Solidity: function submitDepositSweepProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) SubmitDepositSweepProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitDepositSweepProofGasOffset(&_MaintainerProxy.CallOpts)
}

// SubmitDepositSweepProofGasOffset is a free data retrieval call binding the contract method 0x55e19036.
//
// Solidity: function submitDepositSweepProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) SubmitDepositSweepProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitDepositSweepProofGasOffset(&_MaintainerProxy.CallOpts)
}

// SubmitMovedFundsSweepProofGasOffset is a free data retrieval call binding the contract method 0xe12bad86.
//
// Solidity: function submitMovedFundsSweepProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) SubmitMovedFundsSweepProofGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "submitMovedFundsSweepProofGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SubmitMovedFundsSweepProofGasOffset is a free data retrieval call binding the contract method 0xe12bad86.
//
// Solidity: function submitMovedFundsSweepProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) SubmitMovedFundsSweepProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitMovedFundsSweepProofGasOffset(&_MaintainerProxy.CallOpts)
}

// SubmitMovedFundsSweepProofGasOffset is a free data retrieval call binding the contract method 0xe12bad86.
//
// Solidity: function submitMovedFundsSweepProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) SubmitMovedFundsSweepProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitMovedFundsSweepProofGasOffset(&_MaintainerProxy.CallOpts)
}

// SubmitMovingFundsProofGasOffset is a free data retrieval call binding the contract method 0xddd23f5e.
//
// Solidity: function submitMovingFundsProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) SubmitMovingFundsProofGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "submitMovingFundsProofGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SubmitMovingFundsProofGasOffset is a free data retrieval call binding the contract method 0xddd23f5e.
//
// Solidity: function submitMovingFundsProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) SubmitMovingFundsProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitMovingFundsProofGasOffset(&_MaintainerProxy.CallOpts)
}

// SubmitMovingFundsProofGasOffset is a free data retrieval call binding the contract method 0xddd23f5e.
//
// Solidity: function submitMovingFundsProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) SubmitMovingFundsProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitMovingFundsProofGasOffset(&_MaintainerProxy.CallOpts)
}

// SubmitRedemptionProofGasOffset is a free data retrieval call binding the contract method 0x4147907c.
//
// Solidity: function submitRedemptionProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCaller) SubmitRedemptionProofGasOffset(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "submitRedemptionProofGasOffset")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SubmitRedemptionProofGasOffset is a free data retrieval call binding the contract method 0x4147907c.
//
// Solidity: function submitRedemptionProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxySession) SubmitRedemptionProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitRedemptionProofGasOffset(&_MaintainerProxy.CallOpts)
}

// SubmitRedemptionProofGasOffset is a free data retrieval call binding the contract method 0x4147907c.
//
// Solidity: function submitRedemptionProofGasOffset() view returns(uint256)
func (_MaintainerProxy *MaintainerProxyCallerSession) SubmitRedemptionProofGasOffset() (*big.Int, error) {
	return _MaintainerProxy.Contract.SubmitRedemptionProofGasOffset(&_MaintainerProxy.CallOpts)
}

// WalletMaintainers is a free data retrieval call binding the contract method 0xa95f7866.
//
// Solidity: function walletMaintainers(uint256 ) view returns(address)
func (_MaintainerProxy *MaintainerProxyCaller) WalletMaintainers(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _MaintainerProxy.contract.Call(opts, &out, "walletMaintainers", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// WalletMaintainers is a free data retrieval call binding the contract method 0xa95f7866.
//
// Solidity: function walletMaintainers(uint256 ) view returns(address)
func (_MaintainerProxy *MaintainerProxySession) WalletMaintainers(arg0 *big.Int) (common.Address, error) {
	return _MaintainerProxy.Contract.WalletMaintainers(&_MaintainerProxy.CallOpts, arg0)
}

// WalletMaintainers is a free data retrieval call binding the contract method 0xa95f7866.
//
// Solidity: function walletMaintainers(uint256 ) view returns(address)
func (_MaintainerProxy *MaintainerProxyCallerSession) WalletMaintainers(arg0 *big.Int) (common.Address, error) {
	return _MaintainerProxy.Contract.WalletMaintainers(&_MaintainerProxy.CallOpts, arg0)
}

// AuthorizeSpvMaintainer is a paid mutator transaction binding the contract method 0x9f4b337d.
//
// Solidity: function authorizeSpvMaintainer(address maintainer) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) AuthorizeSpvMaintainer(opts *bind.TransactOpts, maintainer common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "authorizeSpvMaintainer", maintainer)
}

// AuthorizeSpvMaintainer is a paid mutator transaction binding the contract method 0x9f4b337d.
//
// Solidity: function authorizeSpvMaintainer(address maintainer) returns()
func (_MaintainerProxy *MaintainerProxySession) AuthorizeSpvMaintainer(maintainer common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.AuthorizeSpvMaintainer(&_MaintainerProxy.TransactOpts, maintainer)
}

// AuthorizeSpvMaintainer is a paid mutator transaction binding the contract method 0x9f4b337d.
//
// Solidity: function authorizeSpvMaintainer(address maintainer) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) AuthorizeSpvMaintainer(maintainer common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.AuthorizeSpvMaintainer(&_MaintainerProxy.TransactOpts, maintainer)
}

// AuthorizeWalletMaintainer is a paid mutator transaction binding the contract method 0x0015b04e.
//
// Solidity: function authorizeWalletMaintainer(address maintainer) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) AuthorizeWalletMaintainer(opts *bind.TransactOpts, maintainer common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "authorizeWalletMaintainer", maintainer)
}

// AuthorizeWalletMaintainer is a paid mutator transaction binding the contract method 0x0015b04e.
//
// Solidity: function authorizeWalletMaintainer(address maintainer) returns()
func (_MaintainerProxy *MaintainerProxySession) AuthorizeWalletMaintainer(maintainer common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.AuthorizeWalletMaintainer(&_MaintainerProxy.TransactOpts, maintainer)
}

// AuthorizeWalletMaintainer is a paid mutator transaction binding the contract method 0x0015b04e.
//
// Solidity: function authorizeWalletMaintainer(address maintainer) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) AuthorizeWalletMaintainer(maintainer common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.AuthorizeWalletMaintainer(&_MaintainerProxy.TransactOpts, maintainer)
}

// DefeatFraudChallenge is a paid mutator transaction binding the contract method 0x77145f21.
//
// Solidity: function defeatFraudChallenge(bytes walletPublicKey, bytes preimage, bool witness) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) DefeatFraudChallenge(opts *bind.TransactOpts, walletPublicKey []byte, preimage []byte, witness bool) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "defeatFraudChallenge", walletPublicKey, preimage, witness)
}

// DefeatFraudChallenge is a paid mutator transaction binding the contract method 0x77145f21.
//
// Solidity: function defeatFraudChallenge(bytes walletPublicKey, bytes preimage, bool witness) returns()
func (_MaintainerProxy *MaintainerProxySession) DefeatFraudChallenge(walletPublicKey []byte, preimage []byte, witness bool) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallenge(&_MaintainerProxy.TransactOpts, walletPublicKey, preimage, witness)
}

// DefeatFraudChallenge is a paid mutator transaction binding the contract method 0x77145f21.
//
// Solidity: function defeatFraudChallenge(bytes walletPublicKey, bytes preimage, bool witness) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) DefeatFraudChallenge(walletPublicKey []byte, preimage []byte, witness bool) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallenge(&_MaintainerProxy.TransactOpts, walletPublicKey, preimage, witness)
}

// DefeatFraudChallengeWithHeartbeat is a paid mutator transaction binding the contract method 0x0674f266.
//
// Solidity: function defeatFraudChallengeWithHeartbeat(bytes walletPublicKey, bytes heartbeatMessage) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) DefeatFraudChallengeWithHeartbeat(opts *bind.TransactOpts, walletPublicKey []byte, heartbeatMessage []byte) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "defeatFraudChallengeWithHeartbeat", walletPublicKey, heartbeatMessage)
}

// DefeatFraudChallengeWithHeartbeat is a paid mutator transaction binding the contract method 0x0674f266.
//
// Solidity: function defeatFraudChallengeWithHeartbeat(bytes walletPublicKey, bytes heartbeatMessage) returns()
func (_MaintainerProxy *MaintainerProxySession) DefeatFraudChallengeWithHeartbeat(walletPublicKey []byte, heartbeatMessage []byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallengeWithHeartbeat(&_MaintainerProxy.TransactOpts, walletPublicKey, heartbeatMessage)
}

// DefeatFraudChallengeWithHeartbeat is a paid mutator transaction binding the contract method 0x0674f266.
//
// Solidity: function defeatFraudChallengeWithHeartbeat(bytes walletPublicKey, bytes heartbeatMessage) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) DefeatFraudChallengeWithHeartbeat(walletPublicKey []byte, heartbeatMessage []byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.DefeatFraudChallengeWithHeartbeat(&_MaintainerProxy.TransactOpts, walletPublicKey, heartbeatMessage)
}

// NotifyMovingFundsBelowDust is a paid mutator transaction binding the contract method 0x07f7d223.
//
// Solidity: function notifyMovingFundsBelowDust(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) NotifyMovingFundsBelowDust(opts *bind.TransactOpts, walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "notifyMovingFundsBelowDust", walletPubKeyHash, mainUtxo)
}

// NotifyMovingFundsBelowDust is a paid mutator transaction binding the contract method 0x07f7d223.
//
// Solidity: function notifyMovingFundsBelowDust(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo) returns()
func (_MaintainerProxy *MaintainerProxySession) NotifyMovingFundsBelowDust(walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.NotifyMovingFundsBelowDust(&_MaintainerProxy.TransactOpts, walletPubKeyHash, mainUtxo)
}

// NotifyMovingFundsBelowDust is a paid mutator transaction binding the contract method 0x07f7d223.
//
// Solidity: function notifyMovingFundsBelowDust(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) NotifyMovingFundsBelowDust(walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.NotifyMovingFundsBelowDust(&_MaintainerProxy.TransactOpts, walletPubKeyHash, mainUtxo)
}

// NotifyWalletCloseable is a paid mutator transaction binding the contract method 0xe44bdd31.
//
// Solidity: function notifyWalletCloseable(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) NotifyWalletCloseable(opts *bind.TransactOpts, walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "notifyWalletCloseable", walletPubKeyHash, walletMainUtxo)
}

// NotifyWalletCloseable is a paid mutator transaction binding the contract method 0xe44bdd31.
//
// Solidity: function notifyWalletCloseable(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo) returns()
func (_MaintainerProxy *MaintainerProxySession) NotifyWalletCloseable(walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.NotifyWalletCloseable(&_MaintainerProxy.TransactOpts, walletPubKeyHash, walletMainUtxo)
}

// NotifyWalletCloseable is a paid mutator transaction binding the contract method 0xe44bdd31.
//
// Solidity: function notifyWalletCloseable(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) NotifyWalletCloseable(walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.NotifyWalletCloseable(&_MaintainerProxy.TransactOpts, walletPubKeyHash, walletMainUtxo)
}

// NotifyWalletClosingPeriodElapsed is a paid mutator transaction binding the contract method 0xc8b5d2db.
//
// Solidity: function notifyWalletClosingPeriodElapsed(bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) NotifyWalletClosingPeriodElapsed(opts *bind.TransactOpts, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "notifyWalletClosingPeriodElapsed", walletPubKeyHash)
}

// NotifyWalletClosingPeriodElapsed is a paid mutator transaction binding the contract method 0xc8b5d2db.
//
// Solidity: function notifyWalletClosingPeriodElapsed(bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxySession) NotifyWalletClosingPeriodElapsed(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.NotifyWalletClosingPeriodElapsed(&_MaintainerProxy.TransactOpts, walletPubKeyHash)
}

// NotifyWalletClosingPeriodElapsed is a paid mutator transaction binding the contract method 0xc8b5d2db.
//
// Solidity: function notifyWalletClosingPeriodElapsed(bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) NotifyWalletClosingPeriodElapsed(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.NotifyWalletClosingPeriodElapsed(&_MaintainerProxy.TransactOpts, walletPubKeyHash)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_MaintainerProxy *MaintainerProxyTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_MaintainerProxy *MaintainerProxySession) RenounceOwnership() (*types.Transaction, error) {
	return _MaintainerProxy.Contract.RenounceOwnership(&_MaintainerProxy.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _MaintainerProxy.Contract.RenounceOwnership(&_MaintainerProxy.TransactOpts)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0xa145e2d5.
//
// Solidity: function requestNewWallet((bytes32,uint32,uint64) activeWalletMainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) RequestNewWallet(opts *bind.TransactOpts, activeWalletMainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "requestNewWallet", activeWalletMainUtxo)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0xa145e2d5.
//
// Solidity: function requestNewWallet((bytes32,uint32,uint64) activeWalletMainUtxo) returns()
func (_MaintainerProxy *MaintainerProxySession) RequestNewWallet(activeWalletMainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.RequestNewWallet(&_MaintainerProxy.TransactOpts, activeWalletMainUtxo)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0xa145e2d5.
//
// Solidity: function requestNewWallet((bytes32,uint32,uint64) activeWalletMainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) RequestNewWallet(activeWalletMainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.RequestNewWallet(&_MaintainerProxy.TransactOpts, activeWalletMainUtxo)
}

// ResetMovingFundsTimeout is a paid mutator transaction binding the contract method 0xee1dd3ea.
//
// Solidity: function resetMovingFundsTimeout(bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) ResetMovingFundsTimeout(opts *bind.TransactOpts, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "resetMovingFundsTimeout", walletPubKeyHash)
}

// ResetMovingFundsTimeout is a paid mutator transaction binding the contract method 0xee1dd3ea.
//
// Solidity: function resetMovingFundsTimeout(bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxySession) ResetMovingFundsTimeout(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.ResetMovingFundsTimeout(&_MaintainerProxy.TransactOpts, walletPubKeyHash)
}

// ResetMovingFundsTimeout is a paid mutator transaction binding the contract method 0xee1dd3ea.
//
// Solidity: function resetMovingFundsTimeout(bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) ResetMovingFundsTimeout(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.ResetMovingFundsTimeout(&_MaintainerProxy.TransactOpts, walletPubKeyHash)
}

// SubmitDepositSweepProof is a paid mutator transaction binding the contract method 0xbd150131.
//
// Solidity: function submitDepositSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo, address vault) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) SubmitDepositSweepProof(opts *bind.TransactOpts, sweepTx BitcoinTxInfo3, sweepProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, vault common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "submitDepositSweepProof", sweepTx, sweepProof, mainUtxo, vault)
}

// SubmitDepositSweepProof is a paid mutator transaction binding the contract method 0xbd150131.
//
// Solidity: function submitDepositSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo, address vault) returns()
func (_MaintainerProxy *MaintainerProxySession) SubmitDepositSweepProof(sweepTx BitcoinTxInfo3, sweepProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, vault common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitDepositSweepProof(&_MaintainerProxy.TransactOpts, sweepTx, sweepProof, mainUtxo, vault)
}

// SubmitDepositSweepProof is a paid mutator transaction binding the contract method 0xbd150131.
//
// Solidity: function submitDepositSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo, address vault) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) SubmitDepositSweepProof(sweepTx BitcoinTxInfo3, sweepProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, vault common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitDepositSweepProof(&_MaintainerProxy.TransactOpts, sweepTx, sweepProof, mainUtxo, vault)
}

// SubmitMovedFundsSweepProof is a paid mutator transaction binding the contract method 0x9821c38b.
//
// Solidity: function submitMovedFundsSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) SubmitMovedFundsSweepProof(opts *bind.TransactOpts, sweepTx BitcoinTxInfo3, sweepProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "submitMovedFundsSweepProof", sweepTx, sweepProof, mainUtxo)
}

// SubmitMovedFundsSweepProof is a paid mutator transaction binding the contract method 0x9821c38b.
//
// Solidity: function submitMovedFundsSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo) returns()
func (_MaintainerProxy *MaintainerProxySession) SubmitMovedFundsSweepProof(sweepTx BitcoinTxInfo3, sweepProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitMovedFundsSweepProof(&_MaintainerProxy.TransactOpts, sweepTx, sweepProof, mainUtxo)
}

// SubmitMovedFundsSweepProof is a paid mutator transaction binding the contract method 0x9821c38b.
//
// Solidity: function submitMovedFundsSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) SubmitMovedFundsSweepProof(sweepTx BitcoinTxInfo3, sweepProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitMovedFundsSweepProof(&_MaintainerProxy.TransactOpts, sweepTx, sweepProof, mainUtxo)
}

// SubmitMovingFundsProof is a paid mutator transaction binding the contract method 0xe509397d.
//
// Solidity: function submitMovingFundsProof((bytes4,bytes,bytes,bytes4) movingFundsTx, (bytes,uint256,bytes,bytes32,bytes) movingFundsProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) SubmitMovingFundsProof(opts *bind.TransactOpts, movingFundsTx BitcoinTxInfo3, movingFundsProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "submitMovingFundsProof", movingFundsTx, movingFundsProof, mainUtxo, walletPubKeyHash)
}

// SubmitMovingFundsProof is a paid mutator transaction binding the contract method 0xe509397d.
//
// Solidity: function submitMovingFundsProof((bytes4,bytes,bytes,bytes4) movingFundsTx, (bytes,uint256,bytes,bytes32,bytes) movingFundsProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxySession) SubmitMovingFundsProof(movingFundsTx BitcoinTxInfo3, movingFundsProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitMovingFundsProof(&_MaintainerProxy.TransactOpts, movingFundsTx, movingFundsProof, mainUtxo, walletPubKeyHash)
}

// SubmitMovingFundsProof is a paid mutator transaction binding the contract method 0xe509397d.
//
// Solidity: function submitMovingFundsProof((bytes4,bytes,bytes,bytes4) movingFundsTx, (bytes,uint256,bytes,bytes32,bytes) movingFundsProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) SubmitMovingFundsProof(movingFundsTx BitcoinTxInfo3, movingFundsProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitMovingFundsProof(&_MaintainerProxy.TransactOpts, movingFundsTx, movingFundsProof, mainUtxo, walletPubKeyHash)
}

// SubmitRedemptionProof is a paid mutator transaction binding the contract method 0x8ac89ce1.
//
// Solidity: function submitRedemptionProof((bytes4,bytes,bytes,bytes4) redemptionTx, (bytes,uint256,bytes,bytes32,bytes) redemptionProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) SubmitRedemptionProof(opts *bind.TransactOpts, redemptionTx BitcoinTxInfo3, redemptionProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "submitRedemptionProof", redemptionTx, redemptionProof, mainUtxo, walletPubKeyHash)
}

// SubmitRedemptionProof is a paid mutator transaction binding the contract method 0x8ac89ce1.
//
// Solidity: function submitRedemptionProof((bytes4,bytes,bytes,bytes4) redemptionTx, (bytes,uint256,bytes,bytes32,bytes) redemptionProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxySession) SubmitRedemptionProof(redemptionTx BitcoinTxInfo3, redemptionProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitRedemptionProof(&_MaintainerProxy.TransactOpts, redemptionTx, redemptionProof, mainUtxo, walletPubKeyHash)
}

// SubmitRedemptionProof is a paid mutator transaction binding the contract method 0x8ac89ce1.
//
// Solidity: function submitRedemptionProof((bytes4,bytes,bytes,bytes4) redemptionTx, (bytes,uint256,bytes,bytes32,bytes) redemptionProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) SubmitRedemptionProof(redemptionTx BitcoinTxInfo3, redemptionProof BitcoinTxProof2, mainUtxo BitcoinTxUTXO2, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.SubmitRedemptionProof(&_MaintainerProxy.TransactOpts, redemptionTx, redemptionProof, mainUtxo, walletPubKeyHash)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_MaintainerProxy *MaintainerProxySession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.TransferOwnership(&_MaintainerProxy.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.TransferOwnership(&_MaintainerProxy.TransactOpts, newOwner)
}

// UnauthorizeSpvMaintainer is a paid mutator transaction binding the contract method 0x77ee72c8.
//
// Solidity: function unauthorizeSpvMaintainer(address maintainerToUnauthorize) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) UnauthorizeSpvMaintainer(opts *bind.TransactOpts, maintainerToUnauthorize common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "unauthorizeSpvMaintainer", maintainerToUnauthorize)
}

// UnauthorizeSpvMaintainer is a paid mutator transaction binding the contract method 0x77ee72c8.
//
// Solidity: function unauthorizeSpvMaintainer(address maintainerToUnauthorize) returns()
func (_MaintainerProxy *MaintainerProxySession) UnauthorizeSpvMaintainer(maintainerToUnauthorize common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UnauthorizeSpvMaintainer(&_MaintainerProxy.TransactOpts, maintainerToUnauthorize)
}

// UnauthorizeSpvMaintainer is a paid mutator transaction binding the contract method 0x77ee72c8.
//
// Solidity: function unauthorizeSpvMaintainer(address maintainerToUnauthorize) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) UnauthorizeSpvMaintainer(maintainerToUnauthorize common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UnauthorizeSpvMaintainer(&_MaintainerProxy.TransactOpts, maintainerToUnauthorize)
}

// UnauthorizeWalletMaintainer is a paid mutator transaction binding the contract method 0xfc9a7c14.
//
// Solidity: function unauthorizeWalletMaintainer(address maintainerToUnauthorize) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) UnauthorizeWalletMaintainer(opts *bind.TransactOpts, maintainerToUnauthorize common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "unauthorizeWalletMaintainer", maintainerToUnauthorize)
}

// UnauthorizeWalletMaintainer is a paid mutator transaction binding the contract method 0xfc9a7c14.
//
// Solidity: function unauthorizeWalletMaintainer(address maintainerToUnauthorize) returns()
func (_MaintainerProxy *MaintainerProxySession) UnauthorizeWalletMaintainer(maintainerToUnauthorize common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UnauthorizeWalletMaintainer(&_MaintainerProxy.TransactOpts, maintainerToUnauthorize)
}

// UnauthorizeWalletMaintainer is a paid mutator transaction binding the contract method 0xfc9a7c14.
//
// Solidity: function unauthorizeWalletMaintainer(address maintainerToUnauthorize) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) UnauthorizeWalletMaintainer(maintainerToUnauthorize common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UnauthorizeWalletMaintainer(&_MaintainerProxy.TransactOpts, maintainerToUnauthorize)
}

// UpdateBridge is a paid mutator transaction binding the contract method 0x6eb38212.
//
// Solidity: function updateBridge(address _bridge) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) UpdateBridge(opts *bind.TransactOpts, _bridge common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "updateBridge", _bridge)
}

// UpdateBridge is a paid mutator transaction binding the contract method 0x6eb38212.
//
// Solidity: function updateBridge(address _bridge) returns()
func (_MaintainerProxy *MaintainerProxySession) UpdateBridge(_bridge common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UpdateBridge(&_MaintainerProxy.TransactOpts, _bridge)
}

// UpdateBridge is a paid mutator transaction binding the contract method 0x6eb38212.
//
// Solidity: function updateBridge(address _bridge) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) UpdateBridge(_bridge common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UpdateBridge(&_MaintainerProxy.TransactOpts, _bridge)
}

// UpdateGasOffsetParameters is a paid mutator transaction binding the contract method 0xbae516aa.
//
// Solidity: function updateGasOffsetParameters(uint256 newSubmitDepositSweepProofGasOffset, uint256 newSubmitRedemptionProofGasOffset, uint256 newResetMovingFundsTimeoutGasOffset, uint256 newSubmitMovingFundsProofGasOffset, uint256 newNotifyMovingFundsBelowDustGasOffset, uint256 newSubmitMovedFundsSweepProofGasOffset, uint256 newRequestNewWalletGasOffset, uint256 newNotifyWalletCloseableGasOffset, uint256 newNotifyWalletClosingPeriodElapsedGasOffset, uint256 newDefeatFraudChallengeGasOffset, uint256 newDefeatFraudChallengeWithHeartbeatGasOffset) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) UpdateGasOffsetParameters(opts *bind.TransactOpts, newSubmitDepositSweepProofGasOffset *big.Int, newSubmitRedemptionProofGasOffset *big.Int, newResetMovingFundsTimeoutGasOffset *big.Int, newSubmitMovingFundsProofGasOffset *big.Int, newNotifyMovingFundsBelowDustGasOffset *big.Int, newSubmitMovedFundsSweepProofGasOffset *big.Int, newRequestNewWalletGasOffset *big.Int, newNotifyWalletCloseableGasOffset *big.Int, newNotifyWalletClosingPeriodElapsedGasOffset *big.Int, newDefeatFraudChallengeGasOffset *big.Int, newDefeatFraudChallengeWithHeartbeatGasOffset *big.Int) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "updateGasOffsetParameters", newSubmitDepositSweepProofGasOffset, newSubmitRedemptionProofGasOffset, newResetMovingFundsTimeoutGasOffset, newSubmitMovingFundsProofGasOffset, newNotifyMovingFundsBelowDustGasOffset, newSubmitMovedFundsSweepProofGasOffset, newRequestNewWalletGasOffset, newNotifyWalletCloseableGasOffset, newNotifyWalletClosingPeriodElapsedGasOffset, newDefeatFraudChallengeGasOffset, newDefeatFraudChallengeWithHeartbeatGasOffset)
}

// UpdateGasOffsetParameters is a paid mutator transaction binding the contract method 0xbae516aa.
//
// Solidity: function updateGasOffsetParameters(uint256 newSubmitDepositSweepProofGasOffset, uint256 newSubmitRedemptionProofGasOffset, uint256 newResetMovingFundsTimeoutGasOffset, uint256 newSubmitMovingFundsProofGasOffset, uint256 newNotifyMovingFundsBelowDustGasOffset, uint256 newSubmitMovedFundsSweepProofGasOffset, uint256 newRequestNewWalletGasOffset, uint256 newNotifyWalletCloseableGasOffset, uint256 newNotifyWalletClosingPeriodElapsedGasOffset, uint256 newDefeatFraudChallengeGasOffset, uint256 newDefeatFraudChallengeWithHeartbeatGasOffset) returns()
func (_MaintainerProxy *MaintainerProxySession) UpdateGasOffsetParameters(newSubmitDepositSweepProofGasOffset *big.Int, newSubmitRedemptionProofGasOffset *big.Int, newResetMovingFundsTimeoutGasOffset *big.Int, newSubmitMovingFundsProofGasOffset *big.Int, newNotifyMovingFundsBelowDustGasOffset *big.Int, newSubmitMovedFundsSweepProofGasOffset *big.Int, newRequestNewWalletGasOffset *big.Int, newNotifyWalletCloseableGasOffset *big.Int, newNotifyWalletClosingPeriodElapsedGasOffset *big.Int, newDefeatFraudChallengeGasOffset *big.Int, newDefeatFraudChallengeWithHeartbeatGasOffset *big.Int) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UpdateGasOffsetParameters(&_MaintainerProxy.TransactOpts, newSubmitDepositSweepProofGasOffset, newSubmitRedemptionProofGasOffset, newResetMovingFundsTimeoutGasOffset, newSubmitMovingFundsProofGasOffset, newNotifyMovingFundsBelowDustGasOffset, newSubmitMovedFundsSweepProofGasOffset, newRequestNewWalletGasOffset, newNotifyWalletCloseableGasOffset, newNotifyWalletClosingPeriodElapsedGasOffset, newDefeatFraudChallengeGasOffset, newDefeatFraudChallengeWithHeartbeatGasOffset)
}

// UpdateGasOffsetParameters is a paid mutator transaction binding the contract method 0xbae516aa.
//
// Solidity: function updateGasOffsetParameters(uint256 newSubmitDepositSweepProofGasOffset, uint256 newSubmitRedemptionProofGasOffset, uint256 newResetMovingFundsTimeoutGasOffset, uint256 newSubmitMovingFundsProofGasOffset, uint256 newNotifyMovingFundsBelowDustGasOffset, uint256 newSubmitMovedFundsSweepProofGasOffset, uint256 newRequestNewWalletGasOffset, uint256 newNotifyWalletCloseableGasOffset, uint256 newNotifyWalletClosingPeriodElapsedGasOffset, uint256 newDefeatFraudChallengeGasOffset, uint256 newDefeatFraudChallengeWithHeartbeatGasOffset) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) UpdateGasOffsetParameters(newSubmitDepositSweepProofGasOffset *big.Int, newSubmitRedemptionProofGasOffset *big.Int, newResetMovingFundsTimeoutGasOffset *big.Int, newSubmitMovingFundsProofGasOffset *big.Int, newNotifyMovingFundsBelowDustGasOffset *big.Int, newSubmitMovedFundsSweepProofGasOffset *big.Int, newRequestNewWalletGasOffset *big.Int, newNotifyWalletCloseableGasOffset *big.Int, newNotifyWalletClosingPeriodElapsedGasOffset *big.Int, newDefeatFraudChallengeGasOffset *big.Int, newDefeatFraudChallengeWithHeartbeatGasOffset *big.Int) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UpdateGasOffsetParameters(&_MaintainerProxy.TransactOpts, newSubmitDepositSweepProofGasOffset, newSubmitRedemptionProofGasOffset, newResetMovingFundsTimeoutGasOffset, newSubmitMovingFundsProofGasOffset, newNotifyMovingFundsBelowDustGasOffset, newSubmitMovedFundsSweepProofGasOffset, newRequestNewWalletGasOffset, newNotifyWalletCloseableGasOffset, newNotifyWalletClosingPeriodElapsedGasOffset, newDefeatFraudChallengeGasOffset, newDefeatFraudChallengeWithHeartbeatGasOffset)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_MaintainerProxy *MaintainerProxyTransactor) UpdateReimbursementPool(opts *bind.TransactOpts, _reimbursementPool common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.contract.Transact(opts, "updateReimbursementPool", _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_MaintainerProxy *MaintainerProxySession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UpdateReimbursementPool(&_MaintainerProxy.TransactOpts, _reimbursementPool)
}

// UpdateReimbursementPool is a paid mutator transaction binding the contract method 0x7b35b4e6.
//
// Solidity: function updateReimbursementPool(address _reimbursementPool) returns()
func (_MaintainerProxy *MaintainerProxyTransactorSession) UpdateReimbursementPool(_reimbursementPool common.Address) (*types.Transaction, error) {
	return _MaintainerProxy.Contract.UpdateReimbursementPool(&_MaintainerProxy.TransactOpts, _reimbursementPool)
}

// MaintainerProxyBridgeUpdatedIterator is returned from FilterBridgeUpdated and is used to iterate over the raw logs and unpacked data for BridgeUpdated events raised by the MaintainerProxy contract.
type MaintainerProxyBridgeUpdatedIterator struct {
	Event *MaintainerProxyBridgeUpdated // Event containing the contract specifics and raw log

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
func (it *MaintainerProxyBridgeUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxyBridgeUpdated)
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
		it.Event = new(MaintainerProxyBridgeUpdated)
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
func (it *MaintainerProxyBridgeUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxyBridgeUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxyBridgeUpdated represents a BridgeUpdated event raised by the MaintainerProxy contract.
type MaintainerProxyBridgeUpdated struct {
	NewBridge common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterBridgeUpdated is a free log retrieval operation binding the contract event 0xe1694c0b21fdceff6411daed547c7463c2341b9695387bc82595b5b9b1851d4a.
//
// Solidity: event BridgeUpdated(address newBridge)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterBridgeUpdated(opts *bind.FilterOpts) (*MaintainerProxyBridgeUpdatedIterator, error) {

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "BridgeUpdated")
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyBridgeUpdatedIterator{contract: _MaintainerProxy.contract, event: "BridgeUpdated", logs: logs, sub: sub}, nil
}

// WatchBridgeUpdated is a free log subscription operation binding the contract event 0xe1694c0b21fdceff6411daed547c7463c2341b9695387bc82595b5b9b1851d4a.
//
// Solidity: event BridgeUpdated(address newBridge)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchBridgeUpdated(opts *bind.WatchOpts, sink chan<- *MaintainerProxyBridgeUpdated) (event.Subscription, error) {

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "BridgeUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxyBridgeUpdated)
				if err := _MaintainerProxy.contract.UnpackLog(event, "BridgeUpdated", log); err != nil {
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

// ParseBridgeUpdated is a log parse operation binding the contract event 0xe1694c0b21fdceff6411daed547c7463c2341b9695387bc82595b5b9b1851d4a.
//
// Solidity: event BridgeUpdated(address newBridge)
func (_MaintainerProxy *MaintainerProxyFilterer) ParseBridgeUpdated(log types.Log) (*MaintainerProxyBridgeUpdated, error) {
	event := new(MaintainerProxyBridgeUpdated)
	if err := _MaintainerProxy.contract.UnpackLog(event, "BridgeUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MaintainerProxyGasOffsetParametersUpdatedIterator is returned from FilterGasOffsetParametersUpdated and is used to iterate over the raw logs and unpacked data for GasOffsetParametersUpdated events raised by the MaintainerProxy contract.
type MaintainerProxyGasOffsetParametersUpdatedIterator struct {
	Event *MaintainerProxyGasOffsetParametersUpdated // Event containing the contract specifics and raw log

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
func (it *MaintainerProxyGasOffsetParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxyGasOffsetParametersUpdated)
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
		it.Event = new(MaintainerProxyGasOffsetParametersUpdated)
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
func (it *MaintainerProxyGasOffsetParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxyGasOffsetParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxyGasOffsetParametersUpdated represents a GasOffsetParametersUpdated event raised by the MaintainerProxy contract.
type MaintainerProxyGasOffsetParametersUpdated struct {
	SubmitDepositSweepProofGasOffset           *big.Int
	SubmitRedemptionProofGasOffset             *big.Int
	ResetMovingFundsTimeoutGasOffset           *big.Int
	SubmitMovingFundsProofGasOffset            *big.Int
	NotifyMovingFundsBelowDustGasOffset        *big.Int
	SubmitMovedFundsSweepProofGasOffset        *big.Int
	RequestNewWalletGasOffset                  *big.Int
	NotifyWalletCloseableGasOffset             *big.Int
	NotifyWalletClosingPeriodElapsedGasOffset  *big.Int
	DefeatFraudChallengeGasOffset              *big.Int
	DefeatFraudChallengeWithHeartbeatGasOffset *big.Int
	Raw                                        types.Log // Blockchain specific contextual infos
}

// FilterGasOffsetParametersUpdated is a free log retrieval operation binding the contract event 0x2696d8d4050334a4233b7a4a7d94d1ae1badb0a200e1b222f2a9837664b4cc5e.
//
// Solidity: event GasOffsetParametersUpdated(uint256 submitDepositSweepProofGasOffset, uint256 submitRedemptionProofGasOffset, uint256 resetMovingFundsTimeoutGasOffset, uint256 submitMovingFundsProofGasOffset, uint256 notifyMovingFundsBelowDustGasOffset, uint256 submitMovedFundsSweepProofGasOffset, uint256 requestNewWalletGasOffset, uint256 notifyWalletCloseableGasOffset, uint256 notifyWalletClosingPeriodElapsedGasOffset, uint256 defeatFraudChallengeGasOffset, uint256 defeatFraudChallengeWithHeartbeatGasOffset)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterGasOffsetParametersUpdated(opts *bind.FilterOpts) (*MaintainerProxyGasOffsetParametersUpdatedIterator, error) {

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "GasOffsetParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyGasOffsetParametersUpdatedIterator{contract: _MaintainerProxy.contract, event: "GasOffsetParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchGasOffsetParametersUpdated is a free log subscription operation binding the contract event 0x2696d8d4050334a4233b7a4a7d94d1ae1badb0a200e1b222f2a9837664b4cc5e.
//
// Solidity: event GasOffsetParametersUpdated(uint256 submitDepositSweepProofGasOffset, uint256 submitRedemptionProofGasOffset, uint256 resetMovingFundsTimeoutGasOffset, uint256 submitMovingFundsProofGasOffset, uint256 notifyMovingFundsBelowDustGasOffset, uint256 submitMovedFundsSweepProofGasOffset, uint256 requestNewWalletGasOffset, uint256 notifyWalletCloseableGasOffset, uint256 notifyWalletClosingPeriodElapsedGasOffset, uint256 defeatFraudChallengeGasOffset, uint256 defeatFraudChallengeWithHeartbeatGasOffset)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchGasOffsetParametersUpdated(opts *bind.WatchOpts, sink chan<- *MaintainerProxyGasOffsetParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "GasOffsetParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxyGasOffsetParametersUpdated)
				if err := _MaintainerProxy.contract.UnpackLog(event, "GasOffsetParametersUpdated", log); err != nil {
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

// ParseGasOffsetParametersUpdated is a log parse operation binding the contract event 0x2696d8d4050334a4233b7a4a7d94d1ae1badb0a200e1b222f2a9837664b4cc5e.
//
// Solidity: event GasOffsetParametersUpdated(uint256 submitDepositSweepProofGasOffset, uint256 submitRedemptionProofGasOffset, uint256 resetMovingFundsTimeoutGasOffset, uint256 submitMovingFundsProofGasOffset, uint256 notifyMovingFundsBelowDustGasOffset, uint256 submitMovedFundsSweepProofGasOffset, uint256 requestNewWalletGasOffset, uint256 notifyWalletCloseableGasOffset, uint256 notifyWalletClosingPeriodElapsedGasOffset, uint256 defeatFraudChallengeGasOffset, uint256 defeatFraudChallengeWithHeartbeatGasOffset)
func (_MaintainerProxy *MaintainerProxyFilterer) ParseGasOffsetParametersUpdated(log types.Log) (*MaintainerProxyGasOffsetParametersUpdated, error) {
	event := new(MaintainerProxyGasOffsetParametersUpdated)
	if err := _MaintainerProxy.contract.UnpackLog(event, "GasOffsetParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MaintainerProxyOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the MaintainerProxy contract.
type MaintainerProxyOwnershipTransferredIterator struct {
	Event *MaintainerProxyOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *MaintainerProxyOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxyOwnershipTransferred)
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
		it.Event = new(MaintainerProxyOwnershipTransferred)
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
func (it *MaintainerProxyOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxyOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxyOwnershipTransferred represents a OwnershipTransferred event raised by the MaintainerProxy contract.
type MaintainerProxyOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*MaintainerProxyOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyOwnershipTransferredIterator{contract: _MaintainerProxy.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *MaintainerProxyOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxyOwnershipTransferred)
				if err := _MaintainerProxy.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_MaintainerProxy *MaintainerProxyFilterer) ParseOwnershipTransferred(log types.Log) (*MaintainerProxyOwnershipTransferred, error) {
	event := new(MaintainerProxyOwnershipTransferred)
	if err := _MaintainerProxy.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MaintainerProxyReimbursementPoolUpdatedIterator is returned from FilterReimbursementPoolUpdated and is used to iterate over the raw logs and unpacked data for ReimbursementPoolUpdated events raised by the MaintainerProxy contract.
type MaintainerProxyReimbursementPoolUpdatedIterator struct {
	Event *MaintainerProxyReimbursementPoolUpdated // Event containing the contract specifics and raw log

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
func (it *MaintainerProxyReimbursementPoolUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxyReimbursementPoolUpdated)
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
		it.Event = new(MaintainerProxyReimbursementPoolUpdated)
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
func (it *MaintainerProxyReimbursementPoolUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxyReimbursementPoolUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxyReimbursementPoolUpdated represents a ReimbursementPoolUpdated event raised by the MaintainerProxy contract.
type MaintainerProxyReimbursementPoolUpdated struct {
	NewReimbursementPool common.Address
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterReimbursementPoolUpdated is a free log retrieval operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterReimbursementPoolUpdated(opts *bind.FilterOpts) (*MaintainerProxyReimbursementPoolUpdatedIterator, error) {

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyReimbursementPoolUpdatedIterator{contract: _MaintainerProxy.contract, event: "ReimbursementPoolUpdated", logs: logs, sub: sub}, nil
}

// WatchReimbursementPoolUpdated is a free log subscription operation binding the contract event 0x0e2d2343d31b085b7c4e56d1c8a6ec79f7ab07460386f1c9a1756239fe2533ac.
//
// Solidity: event ReimbursementPoolUpdated(address newReimbursementPool)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchReimbursementPoolUpdated(opts *bind.WatchOpts, sink chan<- *MaintainerProxyReimbursementPoolUpdated) (event.Subscription, error) {

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "ReimbursementPoolUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxyReimbursementPoolUpdated)
				if err := _MaintainerProxy.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
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
func (_MaintainerProxy *MaintainerProxyFilterer) ParseReimbursementPoolUpdated(log types.Log) (*MaintainerProxyReimbursementPoolUpdated, error) {
	event := new(MaintainerProxyReimbursementPoolUpdated)
	if err := _MaintainerProxy.contract.UnpackLog(event, "ReimbursementPoolUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MaintainerProxySpvMaintainerAuthorizedIterator is returned from FilterSpvMaintainerAuthorized and is used to iterate over the raw logs and unpacked data for SpvMaintainerAuthorized events raised by the MaintainerProxy contract.
type MaintainerProxySpvMaintainerAuthorizedIterator struct {
	Event *MaintainerProxySpvMaintainerAuthorized // Event containing the contract specifics and raw log

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
func (it *MaintainerProxySpvMaintainerAuthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxySpvMaintainerAuthorized)
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
		it.Event = new(MaintainerProxySpvMaintainerAuthorized)
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
func (it *MaintainerProxySpvMaintainerAuthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxySpvMaintainerAuthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxySpvMaintainerAuthorized represents a SpvMaintainerAuthorized event raised by the MaintainerProxy contract.
type MaintainerProxySpvMaintainerAuthorized struct {
	Maintainer common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSpvMaintainerAuthorized is a free log retrieval operation binding the contract event 0xeb6ed19fe6551d940b426deea5c55162c4b2e6b3256f3507173e8f15be01faf3.
//
// Solidity: event SpvMaintainerAuthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterSpvMaintainerAuthorized(opts *bind.FilterOpts, maintainer []common.Address) (*MaintainerProxySpvMaintainerAuthorizedIterator, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "SpvMaintainerAuthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxySpvMaintainerAuthorizedIterator{contract: _MaintainerProxy.contract, event: "SpvMaintainerAuthorized", logs: logs, sub: sub}, nil
}

// WatchSpvMaintainerAuthorized is a free log subscription operation binding the contract event 0xeb6ed19fe6551d940b426deea5c55162c4b2e6b3256f3507173e8f15be01faf3.
//
// Solidity: event SpvMaintainerAuthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchSpvMaintainerAuthorized(opts *bind.WatchOpts, sink chan<- *MaintainerProxySpvMaintainerAuthorized, maintainer []common.Address) (event.Subscription, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "SpvMaintainerAuthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxySpvMaintainerAuthorized)
				if err := _MaintainerProxy.contract.UnpackLog(event, "SpvMaintainerAuthorized", log); err != nil {
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

// ParseSpvMaintainerAuthorized is a log parse operation binding the contract event 0xeb6ed19fe6551d940b426deea5c55162c4b2e6b3256f3507173e8f15be01faf3.
//
// Solidity: event SpvMaintainerAuthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) ParseSpvMaintainerAuthorized(log types.Log) (*MaintainerProxySpvMaintainerAuthorized, error) {
	event := new(MaintainerProxySpvMaintainerAuthorized)
	if err := _MaintainerProxy.contract.UnpackLog(event, "SpvMaintainerAuthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MaintainerProxySpvMaintainerUnauthorizedIterator is returned from FilterSpvMaintainerUnauthorized and is used to iterate over the raw logs and unpacked data for SpvMaintainerUnauthorized events raised by the MaintainerProxy contract.
type MaintainerProxySpvMaintainerUnauthorizedIterator struct {
	Event *MaintainerProxySpvMaintainerUnauthorized // Event containing the contract specifics and raw log

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
func (it *MaintainerProxySpvMaintainerUnauthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxySpvMaintainerUnauthorized)
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
		it.Event = new(MaintainerProxySpvMaintainerUnauthorized)
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
func (it *MaintainerProxySpvMaintainerUnauthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxySpvMaintainerUnauthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxySpvMaintainerUnauthorized represents a SpvMaintainerUnauthorized event raised by the MaintainerProxy contract.
type MaintainerProxySpvMaintainerUnauthorized struct {
	Maintainer common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSpvMaintainerUnauthorized is a free log retrieval operation binding the contract event 0x8534c5565cb2e3175cf88e9616a3b20f6c5caf29d831c49d3f762921507780af.
//
// Solidity: event SpvMaintainerUnauthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterSpvMaintainerUnauthorized(opts *bind.FilterOpts, maintainer []common.Address) (*MaintainerProxySpvMaintainerUnauthorizedIterator, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "SpvMaintainerUnauthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxySpvMaintainerUnauthorizedIterator{contract: _MaintainerProxy.contract, event: "SpvMaintainerUnauthorized", logs: logs, sub: sub}, nil
}

// WatchSpvMaintainerUnauthorized is a free log subscription operation binding the contract event 0x8534c5565cb2e3175cf88e9616a3b20f6c5caf29d831c49d3f762921507780af.
//
// Solidity: event SpvMaintainerUnauthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchSpvMaintainerUnauthorized(opts *bind.WatchOpts, sink chan<- *MaintainerProxySpvMaintainerUnauthorized, maintainer []common.Address) (event.Subscription, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "SpvMaintainerUnauthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxySpvMaintainerUnauthorized)
				if err := _MaintainerProxy.contract.UnpackLog(event, "SpvMaintainerUnauthorized", log); err != nil {
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

// ParseSpvMaintainerUnauthorized is a log parse operation binding the contract event 0x8534c5565cb2e3175cf88e9616a3b20f6c5caf29d831c49d3f762921507780af.
//
// Solidity: event SpvMaintainerUnauthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) ParseSpvMaintainerUnauthorized(log types.Log) (*MaintainerProxySpvMaintainerUnauthorized, error) {
	event := new(MaintainerProxySpvMaintainerUnauthorized)
	if err := _MaintainerProxy.contract.UnpackLog(event, "SpvMaintainerUnauthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MaintainerProxyWalletMaintainerAuthorizedIterator is returned from FilterWalletMaintainerAuthorized and is used to iterate over the raw logs and unpacked data for WalletMaintainerAuthorized events raised by the MaintainerProxy contract.
type MaintainerProxyWalletMaintainerAuthorizedIterator struct {
	Event *MaintainerProxyWalletMaintainerAuthorized // Event containing the contract specifics and raw log

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
func (it *MaintainerProxyWalletMaintainerAuthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxyWalletMaintainerAuthorized)
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
		it.Event = new(MaintainerProxyWalletMaintainerAuthorized)
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
func (it *MaintainerProxyWalletMaintainerAuthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxyWalletMaintainerAuthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxyWalletMaintainerAuthorized represents a WalletMaintainerAuthorized event raised by the MaintainerProxy contract.
type MaintainerProxyWalletMaintainerAuthorized struct {
	Maintainer common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterWalletMaintainerAuthorized is a free log retrieval operation binding the contract event 0x7b22a9ca0e4062aa08f1b002d95a3d99342205c693cb088e382d8e20cfb70a3a.
//
// Solidity: event WalletMaintainerAuthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterWalletMaintainerAuthorized(opts *bind.FilterOpts, maintainer []common.Address) (*MaintainerProxyWalletMaintainerAuthorizedIterator, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "WalletMaintainerAuthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyWalletMaintainerAuthorizedIterator{contract: _MaintainerProxy.contract, event: "WalletMaintainerAuthorized", logs: logs, sub: sub}, nil
}

// WatchWalletMaintainerAuthorized is a free log subscription operation binding the contract event 0x7b22a9ca0e4062aa08f1b002d95a3d99342205c693cb088e382d8e20cfb70a3a.
//
// Solidity: event WalletMaintainerAuthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchWalletMaintainerAuthorized(opts *bind.WatchOpts, sink chan<- *MaintainerProxyWalletMaintainerAuthorized, maintainer []common.Address) (event.Subscription, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "WalletMaintainerAuthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxyWalletMaintainerAuthorized)
				if err := _MaintainerProxy.contract.UnpackLog(event, "WalletMaintainerAuthorized", log); err != nil {
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

// ParseWalletMaintainerAuthorized is a log parse operation binding the contract event 0x7b22a9ca0e4062aa08f1b002d95a3d99342205c693cb088e382d8e20cfb70a3a.
//
// Solidity: event WalletMaintainerAuthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) ParseWalletMaintainerAuthorized(log types.Log) (*MaintainerProxyWalletMaintainerAuthorized, error) {
	event := new(MaintainerProxyWalletMaintainerAuthorized)
	if err := _MaintainerProxy.contract.UnpackLog(event, "WalletMaintainerAuthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MaintainerProxyWalletMaintainerUnauthorizedIterator is returned from FilterWalletMaintainerUnauthorized and is used to iterate over the raw logs and unpacked data for WalletMaintainerUnauthorized events raised by the MaintainerProxy contract.
type MaintainerProxyWalletMaintainerUnauthorizedIterator struct {
	Event *MaintainerProxyWalletMaintainerUnauthorized // Event containing the contract specifics and raw log

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
func (it *MaintainerProxyWalletMaintainerUnauthorizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MaintainerProxyWalletMaintainerUnauthorized)
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
		it.Event = new(MaintainerProxyWalletMaintainerUnauthorized)
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
func (it *MaintainerProxyWalletMaintainerUnauthorizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MaintainerProxyWalletMaintainerUnauthorizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MaintainerProxyWalletMaintainerUnauthorized represents a WalletMaintainerUnauthorized event raised by the MaintainerProxy contract.
type MaintainerProxyWalletMaintainerUnauthorized struct {
	Maintainer common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterWalletMaintainerUnauthorized is a free log retrieval operation binding the contract event 0xb43b4403ab2d3b17d8f18b9366c2da409c988c0a99b8ca0b006f9115dd676cb6.
//
// Solidity: event WalletMaintainerUnauthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) FilterWalletMaintainerUnauthorized(opts *bind.FilterOpts, maintainer []common.Address) (*MaintainerProxyWalletMaintainerUnauthorizedIterator, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.FilterLogs(opts, "WalletMaintainerUnauthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return &MaintainerProxyWalletMaintainerUnauthorizedIterator{contract: _MaintainerProxy.contract, event: "WalletMaintainerUnauthorized", logs: logs, sub: sub}, nil
}

// WatchWalletMaintainerUnauthorized is a free log subscription operation binding the contract event 0xb43b4403ab2d3b17d8f18b9366c2da409c988c0a99b8ca0b006f9115dd676cb6.
//
// Solidity: event WalletMaintainerUnauthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) WatchWalletMaintainerUnauthorized(opts *bind.WatchOpts, sink chan<- *MaintainerProxyWalletMaintainerUnauthorized, maintainer []common.Address) (event.Subscription, error) {

	var maintainerRule []interface{}
	for _, maintainerItem := range maintainer {
		maintainerRule = append(maintainerRule, maintainerItem)
	}

	logs, sub, err := _MaintainerProxy.contract.WatchLogs(opts, "WalletMaintainerUnauthorized", maintainerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MaintainerProxyWalletMaintainerUnauthorized)
				if err := _MaintainerProxy.contract.UnpackLog(event, "WalletMaintainerUnauthorized", log); err != nil {
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

// ParseWalletMaintainerUnauthorized is a log parse operation binding the contract event 0xb43b4403ab2d3b17d8f18b9366c2da409c988c0a99b8ca0b006f9115dd676cb6.
//
// Solidity: event WalletMaintainerUnauthorized(address indexed maintainer)
func (_MaintainerProxy *MaintainerProxyFilterer) ParseWalletMaintainerUnauthorized(log types.Log) (*MaintainerProxyWalletMaintainerUnauthorized, error) {
	event := new(MaintainerProxyWalletMaintainerUnauthorized)
	if err := _MaintainerProxy.contract.UnpackLog(event, "WalletMaintainerUnauthorized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

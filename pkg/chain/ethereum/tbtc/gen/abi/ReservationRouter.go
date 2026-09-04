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

// BitcoinTxInfo4 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxInfo4 struct {
	Version      [4]byte
	InputVector  []byte
	OutputVector []byte
	Locktime     [4]byte
}

// BitcoinTxProof3 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxProof3 struct {
	MerkleProof      []byte
	TxIndexInBlock   *big.Int
	BitcoinHeaders   []byte
	CoinbasePreimage [32]byte
	CoinbaseProof    []byte
}

// BitcoinTxUTXO4 is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxUTXO4 struct {
	TxHash        [32]byte
	TxOutputIndex uint32
	TxOutputValue uint64
}

// ReservationReservationAction is an auto generated low-level Go binding around an user-defined struct.
type ReservationReservationAction struct {
	TargetWalletPubKeyHash  [20]byte
	RequestedAt             uint32
	TimeoutAt               uint32
	TxMaxFee                uint64
	ActionType              uint8
	State                   uint8
	FeePaid                 bool
	Redeemer                common.Address
	Amount                  uint64
	ActionDataHash          [32]byte
	SourceAnchorUtxoHash    [32]byte
	UsedRetryCredit         bool
	WatchtowerDefaultDelay  uint32
	WatchtowerLevelOneDelay uint32
	WatchtowerLevelTwoDelay uint32
	IsPartial               bool
	RetryCreditSourceNonce  uint64
}

// ReservationReservationRequest is an auto generated low-level Go binding around an user-defined struct.
type ReservationReservationRequest struct {
	Owner                 common.Address
	MintedAmount          uint64
	AcceptedAt            uint32
	WalletPubKeyHash      [20]byte
	AnchorAmount          uint64
	ExpiresAt             uint32
	AnchorTxHash          [32]byte
	AnchorTxOutputIndex   uint32
	State                 uint8
	RequestNonce          uint64
	RetryCredit           bool
	DissolutionEligibleAt uint32
	CumulativeReanchorFee uint64
}

// ReservationRouterMetaData contains all meta data concerning the ReservationRouter contract.
var ReservationRouterMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldGovernance\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"GovernanceTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"depositAmount\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"txMaxFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"timeoutAt\",\"type\":\"uint32\"}],\"name\":\"ReservationAcceptanceRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"anchorTxHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"anchorAmount\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"expiresAt\",\"type\":\"uint32\"}],\"name\":\"ReservationAccepted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"}],\"name\":\"ReservationActionSuperseded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"enumReservation.ActionType\",\"name\":\"actionType\",\"type\":\"uint8\"}],\"name\":\"ReservationActionTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"maxReservationsAmountPerWallet\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"reservationMaxSingleAmount\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"maxActiveReservations\",\"type\":\"uint32\"}],\"name\":\"ReservationCapsUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"enumReservation.ActionType\",\"name\":\"actionType\",\"type\":\"uint8\"}],\"name\":\"ReservationLateSettled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"reservationMinAmount\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"reservationTxMaxFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"reservationTermSeconds\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"reservationDissolutionDelay\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"reservationMaxTotalAmount\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"maxReservationsPerWallet\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"reservationActionTimeout\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"reservationRenewalWindowSeconds\",\"type\":\"uint32\"}],\"name\":\"ReservationParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"sourceWalletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"targetWalletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"txMaxFee\",\"type\":\"uint64\"}],\"name\":\"ReservationReanchorRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"newWalletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"newAnchorTxHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"newAnchorAmount\",\"type\":\"uint64\"}],\"name\":\"ReservationReanchored\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"}],\"name\":\"ReservationRetryCreditMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"reservationRouter\",\"type\":\"address\"}],\"name\":\"ReservationRouterSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"anchorAmount\",\"type\":\"uint64\"}],\"name\":\"ReservationStranded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"reservationVault\",\"type\":\"address\"}],\"name\":\"ReservationVaultUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"depositKey\",\"type\":\"uint256\"}],\"name\":\"ReservedDepositMarkedStale\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"activeReservationsCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"count\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"maxActive\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"governance\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"}],\"name\":\"notifyReservationActionTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"}],\"name\":\"notifyReservationStranded\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"depositKey\",\"type\":\"uint256\"}],\"name\":\"notifyStaleReservedDeposit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pendingReservedDeposits\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"requestReservationAcceptance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"internalType\":\"bytes20\",\"name\":\"targetWalletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"requestReservationReanchor\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"}],\"name\":\"reservationActions\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"targetWalletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"uint32\",\"name\":\"requestedAt\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"timeoutAt\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"enumReservation.ActionType\",\"name\":\"actionType\",\"type\":\"uint8\"},{\"internalType\":\"enumReservation.ActionState\",\"name\":\"state\",\"type\":\"uint8\"},{\"internalType\":\"bool\",\"name\":\"feePaid\",\"type\":\"bool\"},{\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"amount\",\"type\":\"uint64\"},{\"internalType\":\"bytes32\",\"name\":\"actionDataHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"sourceAnchorUtxoHash\",\"type\":\"bytes32\"},{\"internalType\":\"bool\",\"name\":\"usedRetryCredit\",\"type\":\"bool\"},{\"internalType\":\"uint32\",\"name\":\"watchtowerDefaultDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"watchtowerLevelOneDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"watchtowerLevelTwoDelay\",\"type\":\"uint32\"},{\"internalType\":\"bool\",\"name\":\"isPartial\",\"type\":\"bool\"},{\"internalType\":\"uint64\",\"name\":\"retryCreditSourceNonce\",\"type\":\"uint64\"}],\"internalType\":\"structReservation.ReservationAction\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"anchorTxHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"anchorTxOutputIndex\",\"type\":\"uint32\"}],\"name\":\"reservationByAnchorUtxo\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reservationCaps\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"maxReservationsAmountPerWallet\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"reservationMaxSingleAmount\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reservationParameters\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"reservationVault\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"reservationMinAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"reservationTxMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"reservationTermSeconds\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"reservationDissolutionDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"reservationMaxTotalAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"reservationTotalAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"maxReservationsPerWallet\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"reservationActionTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"reservationRenewalWindowSeconds\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"reservationRouter\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"}],\"name\":\"reservations\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"mintedAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"acceptedAt\",\"type\":\"uint32\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"uint64\",\"name\":\"anchorAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"expiresAt\",\"type\":\"uint32\"},{\"internalType\":\"bytes32\",\"name\":\"anchorTxHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"anchorTxOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"enumReservation.ReservationState\",\"name\":\"state\",\"type\":\"uint8\"},{\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"},{\"internalType\":\"bool\",\"name\":\"retryCredit\",\"type\":\"bool\"},{\"internalType\":\"uint32\",\"name\":\"dissolutionEligibleAt\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"cumulativeReanchorFee\",\"type\":\"uint64\"}],\"internalType\":\"structReservation.ReservationRequest\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"depositKey\",\"type\":\"uint256\"}],\"name\":\"reservedDepositWallet\",\"outputs\":[{\"internalType\":\"bytes20\",\"name\":\"\",\"type\":\"bytes20\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"proofType\",\"type\":\"uint8\"},{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"txInfo\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"proof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"uint256\",\"name\":\"reservationKey\",\"type\":\"uint256\"},{\"internalType\":\"uint64\",\"name\":\"requestNonce\",\"type\":\"uint64\"}],\"name\":\"submitReservationProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"transferGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"maxReservationsAmountPerWallet\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"reservationMaxSingleAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"maxActiveReservations\",\"type\":\"uint32\"}],\"name\":\"updateReservationCaps\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"reservationVault\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"reservationMinAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"reservationTxMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"reservationTermSeconds\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"reservationDissolutionDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"reservationMaxTotalAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"maxReservationsPerWallet\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"reservationActionTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"reservationRenewalWindowSeconds\",\"type\":\"uint32\"}],\"name\":\"updateReservationParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"walletReservations\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"walletReservationsAmount\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"walletReservationsCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// ReservationRouterABI is the input ABI used to generate the binding from.
// Deprecated: Use ReservationRouterMetaData.ABI instead.
var ReservationRouterABI = ReservationRouterMetaData.ABI

// ReservationRouter is an auto generated Go binding around an Ethereum contract.
type ReservationRouter struct {
	ReservationRouterCaller     // Read-only binding to the contract
	ReservationRouterTransactor // Write-only binding to the contract
	ReservationRouterFilterer   // Log filterer for contract events
}

// ReservationRouterCaller is an auto generated read-only Go binding around an Ethereum contract.
type ReservationRouterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReservationRouterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ReservationRouterTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReservationRouterFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ReservationRouterFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ReservationRouterSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ReservationRouterSession struct {
	Contract     *ReservationRouter // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// ReservationRouterCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ReservationRouterCallerSession struct {
	Contract *ReservationRouterCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// ReservationRouterTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ReservationRouterTransactorSession struct {
	Contract     *ReservationRouterTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// ReservationRouterRaw is an auto generated low-level Go binding around an Ethereum contract.
type ReservationRouterRaw struct {
	Contract *ReservationRouter // Generic contract binding to access the raw methods on
}

// ReservationRouterCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ReservationRouterCallerRaw struct {
	Contract *ReservationRouterCaller // Generic read-only contract binding to access the raw methods on
}

// ReservationRouterTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ReservationRouterTransactorRaw struct {
	Contract *ReservationRouterTransactor // Generic write-only contract binding to access the raw methods on
}

// NewReservationRouter creates a new instance of ReservationRouter, bound to a specific deployed contract.
func NewReservationRouter(address common.Address, backend bind.ContractBackend) (*ReservationRouter, error) {
	contract, err := bindReservationRouter(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ReservationRouter{ReservationRouterCaller: ReservationRouterCaller{contract: contract}, ReservationRouterTransactor: ReservationRouterTransactor{contract: contract}, ReservationRouterFilterer: ReservationRouterFilterer{contract: contract}}, nil
}

// NewReservationRouterCaller creates a new read-only instance of ReservationRouter, bound to a specific deployed contract.
func NewReservationRouterCaller(address common.Address, caller bind.ContractCaller) (*ReservationRouterCaller, error) {
	contract, err := bindReservationRouter(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterCaller{contract: contract}, nil
}

// NewReservationRouterTransactor creates a new write-only instance of ReservationRouter, bound to a specific deployed contract.
func NewReservationRouterTransactor(address common.Address, transactor bind.ContractTransactor) (*ReservationRouterTransactor, error) {
	contract, err := bindReservationRouter(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterTransactor{contract: contract}, nil
}

// NewReservationRouterFilterer creates a new log filterer instance of ReservationRouter, bound to a specific deployed contract.
func NewReservationRouterFilterer(address common.Address, filterer bind.ContractFilterer) (*ReservationRouterFilterer, error) {
	contract, err := bindReservationRouter(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterFilterer{contract: contract}, nil
}

// bindReservationRouter binds a generic wrapper to an already deployed contract.
func bindReservationRouter(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ReservationRouterMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReservationRouter *ReservationRouterRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ReservationRouter.Contract.ReservationRouterCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReservationRouter *ReservationRouterRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReservationRouter.Contract.ReservationRouterTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReservationRouter *ReservationRouterRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReservationRouter.Contract.ReservationRouterTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ReservationRouter *ReservationRouterCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ReservationRouter.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ReservationRouter *ReservationRouterTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ReservationRouter.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ReservationRouter *ReservationRouterTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ReservationRouter.Contract.contract.Transact(opts, method, params...)
}

// ActiveReservationsCount is a free data retrieval call binding the contract method 0x93fe5eab.
//
// Solidity: function activeReservationsCount() view returns(uint32 count, uint32 maxActive)
func (_ReservationRouter *ReservationRouterCaller) ActiveReservationsCount(opts *bind.CallOpts) (struct {
	Count     uint32
	MaxActive uint32
}, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "activeReservationsCount")

	outstruct := new(struct {
		Count     uint32
		MaxActive uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Count = *abi.ConvertType(out[0], new(uint32)).(*uint32)
	outstruct.MaxActive = *abi.ConvertType(out[1], new(uint32)).(*uint32)

	return *outstruct, err

}

// ActiveReservationsCount is a free data retrieval call binding the contract method 0x93fe5eab.
//
// Solidity: function activeReservationsCount() view returns(uint32 count, uint32 maxActive)
func (_ReservationRouter *ReservationRouterSession) ActiveReservationsCount() (struct {
	Count     uint32
	MaxActive uint32
}, error) {
	return _ReservationRouter.Contract.ActiveReservationsCount(&_ReservationRouter.CallOpts)
}

// ActiveReservationsCount is a free data retrieval call binding the contract method 0x93fe5eab.
//
// Solidity: function activeReservationsCount() view returns(uint32 count, uint32 maxActive)
func (_ReservationRouter *ReservationRouterCallerSession) ActiveReservationsCount() (struct {
	Count     uint32
	MaxActive uint32
}, error) {
	return _ReservationRouter.Contract.ActiveReservationsCount(&_ReservationRouter.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_ReservationRouter *ReservationRouterCaller) Governance(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "governance")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_ReservationRouter *ReservationRouterSession) Governance() (common.Address, error) {
	return _ReservationRouter.Contract.Governance(&_ReservationRouter.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_ReservationRouter *ReservationRouterCallerSession) Governance() (common.Address, error) {
	return _ReservationRouter.Contract.Governance(&_ReservationRouter.CallOpts)
}

// PendingReservedDeposits is a free data retrieval call binding the contract method 0x34830fc8.
//
// Solidity: function pendingReservedDeposits() view returns(uint64)
func (_ReservationRouter *ReservationRouterCaller) PendingReservedDeposits(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "pendingReservedDeposits")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// PendingReservedDeposits is a free data retrieval call binding the contract method 0x34830fc8.
//
// Solidity: function pendingReservedDeposits() view returns(uint64)
func (_ReservationRouter *ReservationRouterSession) PendingReservedDeposits() (uint64, error) {
	return _ReservationRouter.Contract.PendingReservedDeposits(&_ReservationRouter.CallOpts)
}

// PendingReservedDeposits is a free data retrieval call binding the contract method 0x34830fc8.
//
// Solidity: function pendingReservedDeposits() view returns(uint64)
func (_ReservationRouter *ReservationRouterCallerSession) PendingReservedDeposits() (uint64, error) {
	return _ReservationRouter.Contract.PendingReservedDeposits(&_ReservationRouter.CallOpts)
}

// ReservationActions is a free data retrieval call binding the contract method 0xcec8c6e9.
//
// Solidity: function reservationActions(uint256 reservationKey, uint64 requestNonce) view returns((bytes20,uint32,uint32,uint64,uint8,uint8,bool,address,uint64,bytes32,bytes32,bool,uint32,uint32,uint32,bool,uint64))
func (_ReservationRouter *ReservationRouterCaller) ReservationActions(opts *bind.CallOpts, reservationKey *big.Int, requestNonce uint64) (ReservationReservationAction, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "reservationActions", reservationKey, requestNonce)

	if err != nil {
		return *new(ReservationReservationAction), err
	}

	out0 := *abi.ConvertType(out[0], new(ReservationReservationAction)).(*ReservationReservationAction)

	return out0, err

}

// ReservationActions is a free data retrieval call binding the contract method 0xcec8c6e9.
//
// Solidity: function reservationActions(uint256 reservationKey, uint64 requestNonce) view returns((bytes20,uint32,uint32,uint64,uint8,uint8,bool,address,uint64,bytes32,bytes32,bool,uint32,uint32,uint32,bool,uint64))
func (_ReservationRouter *ReservationRouterSession) ReservationActions(reservationKey *big.Int, requestNonce uint64) (ReservationReservationAction, error) {
	return _ReservationRouter.Contract.ReservationActions(&_ReservationRouter.CallOpts, reservationKey, requestNonce)
}

// ReservationActions is a free data retrieval call binding the contract method 0xcec8c6e9.
//
// Solidity: function reservationActions(uint256 reservationKey, uint64 requestNonce) view returns((bytes20,uint32,uint32,uint64,uint8,uint8,bool,address,uint64,bytes32,bytes32,bool,uint32,uint32,uint32,bool,uint64))
func (_ReservationRouter *ReservationRouterCallerSession) ReservationActions(reservationKey *big.Int, requestNonce uint64) (ReservationReservationAction, error) {
	return _ReservationRouter.Contract.ReservationActions(&_ReservationRouter.CallOpts, reservationKey, requestNonce)
}

// ReservationByAnchorUtxo is a free data retrieval call binding the contract method 0x79731f67.
//
// Solidity: function reservationByAnchorUtxo(bytes32 anchorTxHash, uint32 anchorTxOutputIndex) view returns(uint256)
func (_ReservationRouter *ReservationRouterCaller) ReservationByAnchorUtxo(opts *bind.CallOpts, anchorTxHash [32]byte, anchorTxOutputIndex uint32) (*big.Int, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "reservationByAnchorUtxo", anchorTxHash, anchorTxOutputIndex)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ReservationByAnchorUtxo is a free data retrieval call binding the contract method 0x79731f67.
//
// Solidity: function reservationByAnchorUtxo(bytes32 anchorTxHash, uint32 anchorTxOutputIndex) view returns(uint256)
func (_ReservationRouter *ReservationRouterSession) ReservationByAnchorUtxo(anchorTxHash [32]byte, anchorTxOutputIndex uint32) (*big.Int, error) {
	return _ReservationRouter.Contract.ReservationByAnchorUtxo(&_ReservationRouter.CallOpts, anchorTxHash, anchorTxOutputIndex)
}

// ReservationByAnchorUtxo is a free data retrieval call binding the contract method 0x79731f67.
//
// Solidity: function reservationByAnchorUtxo(bytes32 anchorTxHash, uint32 anchorTxOutputIndex) view returns(uint256)
func (_ReservationRouter *ReservationRouterCallerSession) ReservationByAnchorUtxo(anchorTxHash [32]byte, anchorTxOutputIndex uint32) (*big.Int, error) {
	return _ReservationRouter.Contract.ReservationByAnchorUtxo(&_ReservationRouter.CallOpts, anchorTxHash, anchorTxOutputIndex)
}

// ReservationCaps is a free data retrieval call binding the contract method 0x63dfb29c.
//
// Solidity: function reservationCaps() view returns(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount)
func (_ReservationRouter *ReservationRouterCaller) ReservationCaps(opts *bind.CallOpts) (struct {
	MaxReservationsAmountPerWallet uint64
	ReservationMaxSingleAmount     uint64
}, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "reservationCaps")

	outstruct := new(struct {
		MaxReservationsAmountPerWallet uint64
		ReservationMaxSingleAmount     uint64
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.MaxReservationsAmountPerWallet = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.ReservationMaxSingleAmount = *abi.ConvertType(out[1], new(uint64)).(*uint64)

	return *outstruct, err

}

// ReservationCaps is a free data retrieval call binding the contract method 0x63dfb29c.
//
// Solidity: function reservationCaps() view returns(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount)
func (_ReservationRouter *ReservationRouterSession) ReservationCaps() (struct {
	MaxReservationsAmountPerWallet uint64
	ReservationMaxSingleAmount     uint64
}, error) {
	return _ReservationRouter.Contract.ReservationCaps(&_ReservationRouter.CallOpts)
}

// ReservationCaps is a free data retrieval call binding the contract method 0x63dfb29c.
//
// Solidity: function reservationCaps() view returns(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount)
func (_ReservationRouter *ReservationRouterCallerSession) ReservationCaps() (struct {
	MaxReservationsAmountPerWallet uint64
	ReservationMaxSingleAmount     uint64
}, error) {
	return _ReservationRouter.Contract.ReservationCaps(&_ReservationRouter.CallOpts)
}

// ReservationParameters is a free data retrieval call binding the contract method 0xf75b4b1c.
//
// Solidity: function reservationParameters() view returns(address reservationVault, uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint64 reservationTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds)
func (_ReservationRouter *ReservationRouterCaller) ReservationParameters(opts *bind.CallOpts) (struct {
	ReservationVault                common.Address
	ReservationMinAmount            uint64
	ReservationTxMaxFee             uint64
	ReservationTermSeconds          uint32
	ReservationDissolutionDelay     uint32
	ReservationMaxTotalAmount       uint64
	ReservationTotalAmount          uint64
	MaxReservationsPerWallet        uint32
	ReservationActionTimeout        uint32
	ReservationRenewalWindowSeconds uint32
}, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "reservationParameters")

	outstruct := new(struct {
		ReservationVault                common.Address
		ReservationMinAmount            uint64
		ReservationTxMaxFee             uint64
		ReservationTermSeconds          uint32
		ReservationDissolutionDelay     uint32
		ReservationMaxTotalAmount       uint64
		ReservationTotalAmount          uint64
		MaxReservationsPerWallet        uint32
		ReservationActionTimeout        uint32
		ReservationRenewalWindowSeconds uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.ReservationVault = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.ReservationMinAmount = *abi.ConvertType(out[1], new(uint64)).(*uint64)
	outstruct.ReservationTxMaxFee = *abi.ConvertType(out[2], new(uint64)).(*uint64)
	outstruct.ReservationTermSeconds = *abi.ConvertType(out[3], new(uint32)).(*uint32)
	outstruct.ReservationDissolutionDelay = *abi.ConvertType(out[4], new(uint32)).(*uint32)
	outstruct.ReservationMaxTotalAmount = *abi.ConvertType(out[5], new(uint64)).(*uint64)
	outstruct.ReservationTotalAmount = *abi.ConvertType(out[6], new(uint64)).(*uint64)
	outstruct.MaxReservationsPerWallet = *abi.ConvertType(out[7], new(uint32)).(*uint32)
	outstruct.ReservationActionTimeout = *abi.ConvertType(out[8], new(uint32)).(*uint32)
	outstruct.ReservationRenewalWindowSeconds = *abi.ConvertType(out[9], new(uint32)).(*uint32)

	return *outstruct, err

}

// ReservationParameters is a free data retrieval call binding the contract method 0xf75b4b1c.
//
// Solidity: function reservationParameters() view returns(address reservationVault, uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint64 reservationTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds)
func (_ReservationRouter *ReservationRouterSession) ReservationParameters() (struct {
	ReservationVault                common.Address
	ReservationMinAmount            uint64
	ReservationTxMaxFee             uint64
	ReservationTermSeconds          uint32
	ReservationDissolutionDelay     uint32
	ReservationMaxTotalAmount       uint64
	ReservationTotalAmount          uint64
	MaxReservationsPerWallet        uint32
	ReservationActionTimeout        uint32
	ReservationRenewalWindowSeconds uint32
}, error) {
	return _ReservationRouter.Contract.ReservationParameters(&_ReservationRouter.CallOpts)
}

// ReservationParameters is a free data retrieval call binding the contract method 0xf75b4b1c.
//
// Solidity: function reservationParameters() view returns(address reservationVault, uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint64 reservationTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds)
func (_ReservationRouter *ReservationRouterCallerSession) ReservationParameters() (struct {
	ReservationVault                common.Address
	ReservationMinAmount            uint64
	ReservationTxMaxFee             uint64
	ReservationTermSeconds          uint32
	ReservationDissolutionDelay     uint32
	ReservationMaxTotalAmount       uint64
	ReservationTotalAmount          uint64
	MaxReservationsPerWallet        uint32
	ReservationActionTimeout        uint32
	ReservationRenewalWindowSeconds uint32
}, error) {
	return _ReservationRouter.Contract.ReservationParameters(&_ReservationRouter.CallOpts)
}

// ReservationRouter is a free data retrieval call binding the contract method 0x06ca90d2.
//
// Solidity: function reservationRouter() view returns(address)
func (_ReservationRouter *ReservationRouterCaller) ReservationRouter(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "reservationRouter")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ReservationRouter is a free data retrieval call binding the contract method 0x06ca90d2.
//
// Solidity: function reservationRouter() view returns(address)
func (_ReservationRouter *ReservationRouterSession) ReservationRouter() (common.Address, error) {
	return _ReservationRouter.Contract.ReservationRouter(&_ReservationRouter.CallOpts)
}

// ReservationRouter is a free data retrieval call binding the contract method 0x06ca90d2.
//
// Solidity: function reservationRouter() view returns(address)
func (_ReservationRouter *ReservationRouterCallerSession) ReservationRouter() (common.Address, error) {
	return _ReservationRouter.Contract.ReservationRouter(&_ReservationRouter.CallOpts)
}

// Reservations is a free data retrieval call binding the contract method 0x067cf832.
//
// Solidity: function reservations(uint256 reservationKey) view returns((address,uint64,uint32,bytes20,uint64,uint32,bytes32,uint32,uint8,uint64,bool,uint32,uint64))
func (_ReservationRouter *ReservationRouterCaller) Reservations(opts *bind.CallOpts, reservationKey *big.Int) (ReservationReservationRequest, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "reservations", reservationKey)

	if err != nil {
		return *new(ReservationReservationRequest), err
	}

	out0 := *abi.ConvertType(out[0], new(ReservationReservationRequest)).(*ReservationReservationRequest)

	return out0, err

}

// Reservations is a free data retrieval call binding the contract method 0x067cf832.
//
// Solidity: function reservations(uint256 reservationKey) view returns((address,uint64,uint32,bytes20,uint64,uint32,bytes32,uint32,uint8,uint64,bool,uint32,uint64))
func (_ReservationRouter *ReservationRouterSession) Reservations(reservationKey *big.Int) (ReservationReservationRequest, error) {
	return _ReservationRouter.Contract.Reservations(&_ReservationRouter.CallOpts, reservationKey)
}

// Reservations is a free data retrieval call binding the contract method 0x067cf832.
//
// Solidity: function reservations(uint256 reservationKey) view returns((address,uint64,uint32,bytes20,uint64,uint32,bytes32,uint32,uint8,uint64,bool,uint32,uint64))
func (_ReservationRouter *ReservationRouterCallerSession) Reservations(reservationKey *big.Int) (ReservationReservationRequest, error) {
	return _ReservationRouter.Contract.Reservations(&_ReservationRouter.CallOpts, reservationKey)
}

// ReservedDepositWallet is a free data retrieval call binding the contract method 0x56803b55.
//
// Solidity: function reservedDepositWallet(uint256 depositKey) view returns(bytes20)
func (_ReservationRouter *ReservationRouterCaller) ReservedDepositWallet(opts *bind.CallOpts, depositKey *big.Int) ([20]byte, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "reservedDepositWallet", depositKey)

	if err != nil {
		return *new([20]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([20]byte)).(*[20]byte)

	return out0, err

}

// ReservedDepositWallet is a free data retrieval call binding the contract method 0x56803b55.
//
// Solidity: function reservedDepositWallet(uint256 depositKey) view returns(bytes20)
func (_ReservationRouter *ReservationRouterSession) ReservedDepositWallet(depositKey *big.Int) ([20]byte, error) {
	return _ReservationRouter.Contract.ReservedDepositWallet(&_ReservationRouter.CallOpts, depositKey)
}

// ReservedDepositWallet is a free data retrieval call binding the contract method 0x56803b55.
//
// Solidity: function reservedDepositWallet(uint256 depositKey) view returns(bytes20)
func (_ReservationRouter *ReservationRouterCallerSession) ReservedDepositWallet(depositKey *big.Int) ([20]byte, error) {
	return _ReservationRouter.Contract.ReservedDepositWallet(&_ReservationRouter.CallOpts, depositKey)
}

// WalletReservations is a free data retrieval call binding the contract method 0x78699d2f.
//
// Solidity: function walletReservations(bytes20 walletPubKeyHash) view returns(uint256[])
func (_ReservationRouter *ReservationRouterCaller) WalletReservations(opts *bind.CallOpts, walletPubKeyHash [20]byte) ([]*big.Int, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "walletReservations", walletPubKeyHash)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// WalletReservations is a free data retrieval call binding the contract method 0x78699d2f.
//
// Solidity: function walletReservations(bytes20 walletPubKeyHash) view returns(uint256[])
func (_ReservationRouter *ReservationRouterSession) WalletReservations(walletPubKeyHash [20]byte) ([]*big.Int, error) {
	return _ReservationRouter.Contract.WalletReservations(&_ReservationRouter.CallOpts, walletPubKeyHash)
}

// WalletReservations is a free data retrieval call binding the contract method 0x78699d2f.
//
// Solidity: function walletReservations(bytes20 walletPubKeyHash) view returns(uint256[])
func (_ReservationRouter *ReservationRouterCallerSession) WalletReservations(walletPubKeyHash [20]byte) ([]*big.Int, error) {
	return _ReservationRouter.Contract.WalletReservations(&_ReservationRouter.CallOpts, walletPubKeyHash)
}

// WalletReservationsAmount is a free data retrieval call binding the contract method 0x63481e98.
//
// Solidity: function walletReservationsAmount(bytes20 walletPubKeyHash) view returns(uint64)
func (_ReservationRouter *ReservationRouterCaller) WalletReservationsAmount(opts *bind.CallOpts, walletPubKeyHash [20]byte) (uint64, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "walletReservationsAmount", walletPubKeyHash)

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// WalletReservationsAmount is a free data retrieval call binding the contract method 0x63481e98.
//
// Solidity: function walletReservationsAmount(bytes20 walletPubKeyHash) view returns(uint64)
func (_ReservationRouter *ReservationRouterSession) WalletReservationsAmount(walletPubKeyHash [20]byte) (uint64, error) {
	return _ReservationRouter.Contract.WalletReservationsAmount(&_ReservationRouter.CallOpts, walletPubKeyHash)
}

// WalletReservationsAmount is a free data retrieval call binding the contract method 0x63481e98.
//
// Solidity: function walletReservationsAmount(bytes20 walletPubKeyHash) view returns(uint64)
func (_ReservationRouter *ReservationRouterCallerSession) WalletReservationsAmount(walletPubKeyHash [20]byte) (uint64, error) {
	return _ReservationRouter.Contract.WalletReservationsAmount(&_ReservationRouter.CallOpts, walletPubKeyHash)
}

// WalletReservationsCount is a free data retrieval call binding the contract method 0x1de0555a.
//
// Solidity: function walletReservationsCount(bytes20 walletPubKeyHash) view returns(uint32)
func (_ReservationRouter *ReservationRouterCaller) WalletReservationsCount(opts *bind.CallOpts, walletPubKeyHash [20]byte) (uint32, error) {
	var out []interface{}
	err := _ReservationRouter.contract.Call(opts, &out, "walletReservationsCount", walletPubKeyHash)

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// WalletReservationsCount is a free data retrieval call binding the contract method 0x1de0555a.
//
// Solidity: function walletReservationsCount(bytes20 walletPubKeyHash) view returns(uint32)
func (_ReservationRouter *ReservationRouterSession) WalletReservationsCount(walletPubKeyHash [20]byte) (uint32, error) {
	return _ReservationRouter.Contract.WalletReservationsCount(&_ReservationRouter.CallOpts, walletPubKeyHash)
}

// WalletReservationsCount is a free data retrieval call binding the contract method 0x1de0555a.
//
// Solidity: function walletReservationsCount(bytes20 walletPubKeyHash) view returns(uint32)
func (_ReservationRouter *ReservationRouterCallerSession) WalletReservationsCount(walletPubKeyHash [20]byte) (uint32, error) {
	return _ReservationRouter.Contract.WalletReservationsCount(&_ReservationRouter.CallOpts, walletPubKeyHash)
}

// NotifyReservationActionTimeout is a paid mutator transaction binding the contract method 0x88aa5729.
//
// Solidity: function notifyReservationActionTimeout(uint256 reservationKey, uint32[] walletMembersIDs) returns()
func (_ReservationRouter *ReservationRouterTransactor) NotifyReservationActionTimeout(opts *bind.TransactOpts, reservationKey *big.Int, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "notifyReservationActionTimeout", reservationKey, walletMembersIDs)
}

// NotifyReservationActionTimeout is a paid mutator transaction binding the contract method 0x88aa5729.
//
// Solidity: function notifyReservationActionTimeout(uint256 reservationKey, uint32[] walletMembersIDs) returns()
func (_ReservationRouter *ReservationRouterSession) NotifyReservationActionTimeout(reservationKey *big.Int, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _ReservationRouter.Contract.NotifyReservationActionTimeout(&_ReservationRouter.TransactOpts, reservationKey, walletMembersIDs)
}

// NotifyReservationActionTimeout is a paid mutator transaction binding the contract method 0x88aa5729.
//
// Solidity: function notifyReservationActionTimeout(uint256 reservationKey, uint32[] walletMembersIDs) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) NotifyReservationActionTimeout(reservationKey *big.Int, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _ReservationRouter.Contract.NotifyReservationActionTimeout(&_ReservationRouter.TransactOpts, reservationKey, walletMembersIDs)
}

// NotifyReservationStranded is a paid mutator transaction binding the contract method 0xf95ea36f.
//
// Solidity: function notifyReservationStranded(uint256 reservationKey) returns()
func (_ReservationRouter *ReservationRouterTransactor) NotifyReservationStranded(opts *bind.TransactOpts, reservationKey *big.Int) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "notifyReservationStranded", reservationKey)
}

// NotifyReservationStranded is a paid mutator transaction binding the contract method 0xf95ea36f.
//
// Solidity: function notifyReservationStranded(uint256 reservationKey) returns()
func (_ReservationRouter *ReservationRouterSession) NotifyReservationStranded(reservationKey *big.Int) (*types.Transaction, error) {
	return _ReservationRouter.Contract.NotifyReservationStranded(&_ReservationRouter.TransactOpts, reservationKey)
}

// NotifyReservationStranded is a paid mutator transaction binding the contract method 0xf95ea36f.
//
// Solidity: function notifyReservationStranded(uint256 reservationKey) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) NotifyReservationStranded(reservationKey *big.Int) (*types.Transaction, error) {
	return _ReservationRouter.Contract.NotifyReservationStranded(&_ReservationRouter.TransactOpts, reservationKey)
}

// NotifyStaleReservedDeposit is a paid mutator transaction binding the contract method 0x6ceb1b54.
//
// Solidity: function notifyStaleReservedDeposit(uint256 depositKey) returns()
func (_ReservationRouter *ReservationRouterTransactor) NotifyStaleReservedDeposit(opts *bind.TransactOpts, depositKey *big.Int) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "notifyStaleReservedDeposit", depositKey)
}

// NotifyStaleReservedDeposit is a paid mutator transaction binding the contract method 0x6ceb1b54.
//
// Solidity: function notifyStaleReservedDeposit(uint256 depositKey) returns()
func (_ReservationRouter *ReservationRouterSession) NotifyStaleReservedDeposit(depositKey *big.Int) (*types.Transaction, error) {
	return _ReservationRouter.Contract.NotifyStaleReservedDeposit(&_ReservationRouter.TransactOpts, depositKey)
}

// NotifyStaleReservedDeposit is a paid mutator transaction binding the contract method 0x6ceb1b54.
//
// Solidity: function notifyStaleReservedDeposit(uint256 depositKey) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) NotifyStaleReservedDeposit(depositKey *big.Int) (*types.Transaction, error) {
	return _ReservationRouter.Contract.NotifyStaleReservedDeposit(&_ReservationRouter.TransactOpts, depositKey)
}

// RequestReservationAcceptance is a paid mutator transaction binding the contract method 0xbc78a18e.
//
// Solidity: function requestReservationAcceptance(uint256 reservationKey, bytes20 walletPubKeyHash) returns()
func (_ReservationRouter *ReservationRouterTransactor) RequestReservationAcceptance(opts *bind.TransactOpts, reservationKey *big.Int, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "requestReservationAcceptance", reservationKey, walletPubKeyHash)
}

// RequestReservationAcceptance is a paid mutator transaction binding the contract method 0xbc78a18e.
//
// Solidity: function requestReservationAcceptance(uint256 reservationKey, bytes20 walletPubKeyHash) returns()
func (_ReservationRouter *ReservationRouterSession) RequestReservationAcceptance(reservationKey *big.Int, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _ReservationRouter.Contract.RequestReservationAcceptance(&_ReservationRouter.TransactOpts, reservationKey, walletPubKeyHash)
}

// RequestReservationAcceptance is a paid mutator transaction binding the contract method 0xbc78a18e.
//
// Solidity: function requestReservationAcceptance(uint256 reservationKey, bytes20 walletPubKeyHash) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) RequestReservationAcceptance(reservationKey *big.Int, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _ReservationRouter.Contract.RequestReservationAcceptance(&_ReservationRouter.TransactOpts, reservationKey, walletPubKeyHash)
}

// RequestReservationReanchor is a paid mutator transaction binding the contract method 0xf934beb5.
//
// Solidity: function requestReservationReanchor(uint256 reservationKey, bytes20 targetWalletPubKeyHash) returns()
func (_ReservationRouter *ReservationRouterTransactor) RequestReservationReanchor(opts *bind.TransactOpts, reservationKey *big.Int, targetWalletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "requestReservationReanchor", reservationKey, targetWalletPubKeyHash)
}

// RequestReservationReanchor is a paid mutator transaction binding the contract method 0xf934beb5.
//
// Solidity: function requestReservationReanchor(uint256 reservationKey, bytes20 targetWalletPubKeyHash) returns()
func (_ReservationRouter *ReservationRouterSession) RequestReservationReanchor(reservationKey *big.Int, targetWalletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _ReservationRouter.Contract.RequestReservationReanchor(&_ReservationRouter.TransactOpts, reservationKey, targetWalletPubKeyHash)
}

// RequestReservationReanchor is a paid mutator transaction binding the contract method 0xf934beb5.
//
// Solidity: function requestReservationReanchor(uint256 reservationKey, bytes20 targetWalletPubKeyHash) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) RequestReservationReanchor(reservationKey *big.Int, targetWalletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _ReservationRouter.Contract.RequestReservationReanchor(&_ReservationRouter.TransactOpts, reservationKey, targetWalletPubKeyHash)
}

// SubmitReservationProof is a paid mutator transaction binding the contract method 0x668a4980.
//
// Solidity: function submitReservationProof(uint8 proofType, (bytes4,bytes,bytes,bytes4) txInfo, (bytes,uint256,bytes,bytes32,bytes) proof, (bytes32,uint32,uint64) mainUtxo, uint256 reservationKey, uint64 requestNonce) returns()
func (_ReservationRouter *ReservationRouterTransactor) SubmitReservationProof(opts *bind.TransactOpts, proofType uint8, txInfo BitcoinTxInfo4, proof BitcoinTxProof3, mainUtxo BitcoinTxUTXO4, reservationKey *big.Int, requestNonce uint64) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "submitReservationProof", proofType, txInfo, proof, mainUtxo, reservationKey, requestNonce)
}

// SubmitReservationProof is a paid mutator transaction binding the contract method 0x668a4980.
//
// Solidity: function submitReservationProof(uint8 proofType, (bytes4,bytes,bytes,bytes4) txInfo, (bytes,uint256,bytes,bytes32,bytes) proof, (bytes32,uint32,uint64) mainUtxo, uint256 reservationKey, uint64 requestNonce) returns()
func (_ReservationRouter *ReservationRouterSession) SubmitReservationProof(proofType uint8, txInfo BitcoinTxInfo4, proof BitcoinTxProof3, mainUtxo BitcoinTxUTXO4, reservationKey *big.Int, requestNonce uint64) (*types.Transaction, error) {
	return _ReservationRouter.Contract.SubmitReservationProof(&_ReservationRouter.TransactOpts, proofType, txInfo, proof, mainUtxo, reservationKey, requestNonce)
}

// SubmitReservationProof is a paid mutator transaction binding the contract method 0x668a4980.
//
// Solidity: function submitReservationProof(uint8 proofType, (bytes4,bytes,bytes,bytes4) txInfo, (bytes,uint256,bytes,bytes32,bytes) proof, (bytes32,uint32,uint64) mainUtxo, uint256 reservationKey, uint64 requestNonce) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) SubmitReservationProof(proofType uint8, txInfo BitcoinTxInfo4, proof BitcoinTxProof3, mainUtxo BitcoinTxUTXO4, reservationKey *big.Int, requestNonce uint64) (*types.Transaction, error) {
	return _ReservationRouter.Contract.SubmitReservationProof(&_ReservationRouter.TransactOpts, proofType, txInfo, proof, mainUtxo, reservationKey, requestNonce)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_ReservationRouter *ReservationRouterTransactor) TransferGovernance(opts *bind.TransactOpts, newGovernance common.Address) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "transferGovernance", newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_ReservationRouter *ReservationRouterSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _ReservationRouter.Contract.TransferGovernance(&_ReservationRouter.TransactOpts, newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _ReservationRouter.Contract.TransferGovernance(&_ReservationRouter.TransactOpts, newGovernance)
}

// UpdateReservationCaps is a paid mutator transaction binding the contract method 0x8308c2ca.
//
// Solidity: function updateReservationCaps(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount, uint32 maxActiveReservations) returns()
func (_ReservationRouter *ReservationRouterTransactor) UpdateReservationCaps(opts *bind.TransactOpts, maxReservationsAmountPerWallet uint64, reservationMaxSingleAmount uint64, maxActiveReservations uint32) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "updateReservationCaps", maxReservationsAmountPerWallet, reservationMaxSingleAmount, maxActiveReservations)
}

// UpdateReservationCaps is a paid mutator transaction binding the contract method 0x8308c2ca.
//
// Solidity: function updateReservationCaps(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount, uint32 maxActiveReservations) returns()
func (_ReservationRouter *ReservationRouterSession) UpdateReservationCaps(maxReservationsAmountPerWallet uint64, reservationMaxSingleAmount uint64, maxActiveReservations uint32) (*types.Transaction, error) {
	return _ReservationRouter.Contract.UpdateReservationCaps(&_ReservationRouter.TransactOpts, maxReservationsAmountPerWallet, reservationMaxSingleAmount, maxActiveReservations)
}

// UpdateReservationCaps is a paid mutator transaction binding the contract method 0x8308c2ca.
//
// Solidity: function updateReservationCaps(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount, uint32 maxActiveReservations) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) UpdateReservationCaps(maxReservationsAmountPerWallet uint64, reservationMaxSingleAmount uint64, maxActiveReservations uint32) (*types.Transaction, error) {
	return _ReservationRouter.Contract.UpdateReservationCaps(&_ReservationRouter.TransactOpts, maxReservationsAmountPerWallet, reservationMaxSingleAmount, maxActiveReservations)
}

// UpdateReservationParameters is a paid mutator transaction binding the contract method 0x59f6408b.
//
// Solidity: function updateReservationParameters(address reservationVault, uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds) returns()
func (_ReservationRouter *ReservationRouterTransactor) UpdateReservationParameters(opts *bind.TransactOpts, reservationVault common.Address, reservationMinAmount uint64, reservationTxMaxFee uint64, reservationTermSeconds uint32, reservationDissolutionDelay uint32, reservationMaxTotalAmount uint64, maxReservationsPerWallet uint32, reservationActionTimeout uint32, reservationRenewalWindowSeconds uint32) (*types.Transaction, error) {
	return _ReservationRouter.contract.Transact(opts, "updateReservationParameters", reservationVault, reservationMinAmount, reservationTxMaxFee, reservationTermSeconds, reservationDissolutionDelay, reservationMaxTotalAmount, maxReservationsPerWallet, reservationActionTimeout, reservationRenewalWindowSeconds)
}

// UpdateReservationParameters is a paid mutator transaction binding the contract method 0x59f6408b.
//
// Solidity: function updateReservationParameters(address reservationVault, uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds) returns()
func (_ReservationRouter *ReservationRouterSession) UpdateReservationParameters(reservationVault common.Address, reservationMinAmount uint64, reservationTxMaxFee uint64, reservationTermSeconds uint32, reservationDissolutionDelay uint32, reservationMaxTotalAmount uint64, maxReservationsPerWallet uint32, reservationActionTimeout uint32, reservationRenewalWindowSeconds uint32) (*types.Transaction, error) {
	return _ReservationRouter.Contract.UpdateReservationParameters(&_ReservationRouter.TransactOpts, reservationVault, reservationMinAmount, reservationTxMaxFee, reservationTermSeconds, reservationDissolutionDelay, reservationMaxTotalAmount, maxReservationsPerWallet, reservationActionTimeout, reservationRenewalWindowSeconds)
}

// UpdateReservationParameters is a paid mutator transaction binding the contract method 0x59f6408b.
//
// Solidity: function updateReservationParameters(address reservationVault, uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds) returns()
func (_ReservationRouter *ReservationRouterTransactorSession) UpdateReservationParameters(reservationVault common.Address, reservationMinAmount uint64, reservationTxMaxFee uint64, reservationTermSeconds uint32, reservationDissolutionDelay uint32, reservationMaxTotalAmount uint64, maxReservationsPerWallet uint32, reservationActionTimeout uint32, reservationRenewalWindowSeconds uint32) (*types.Transaction, error) {
	return _ReservationRouter.Contract.UpdateReservationParameters(&_ReservationRouter.TransactOpts, reservationVault, reservationMinAmount, reservationTxMaxFee, reservationTermSeconds, reservationDissolutionDelay, reservationMaxTotalAmount, maxReservationsPerWallet, reservationActionTimeout, reservationRenewalWindowSeconds)
}

// ReservationRouterGovernanceTransferredIterator is returned from FilterGovernanceTransferred and is used to iterate over the raw logs and unpacked data for GovernanceTransferred events raised by the ReservationRouter contract.
type ReservationRouterGovernanceTransferredIterator struct {
	Event *ReservationRouterGovernanceTransferred // Event containing the contract specifics and raw log

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
func (it *ReservationRouterGovernanceTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterGovernanceTransferred)
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
		it.Event = new(ReservationRouterGovernanceTransferred)
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
func (it *ReservationRouterGovernanceTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterGovernanceTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterGovernanceTransferred represents a GovernanceTransferred event raised by the ReservationRouter contract.
type ReservationRouterGovernanceTransferred struct {
	OldGovernance common.Address
	NewGovernance common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterGovernanceTransferred is a free log retrieval operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_ReservationRouter *ReservationRouterFilterer) FilterGovernanceTransferred(opts *bind.FilterOpts) (*ReservationRouterGovernanceTransferredIterator, error) {

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return &ReservationRouterGovernanceTransferredIterator{contract: _ReservationRouter.contract, event: "GovernanceTransferred", logs: logs, sub: sub}, nil
}

// WatchGovernanceTransferred is a free log subscription operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_ReservationRouter *ReservationRouterFilterer) WatchGovernanceTransferred(opts *bind.WatchOpts, sink chan<- *ReservationRouterGovernanceTransferred) (event.Subscription, error) {

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterGovernanceTransferred)
				if err := _ReservationRouter.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
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

// ParseGovernanceTransferred is a log parse operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_ReservationRouter *ReservationRouterFilterer) ParseGovernanceTransferred(log types.Log) (*ReservationRouterGovernanceTransferred, error) {
	event := new(ReservationRouterGovernanceTransferred)
	if err := _ReservationRouter.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the ReservationRouter contract.
type ReservationRouterInitializedIterator struct {
	Event *ReservationRouterInitialized // Event containing the contract specifics and raw log

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
func (it *ReservationRouterInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterInitialized)
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
		it.Event = new(ReservationRouterInitialized)
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
func (it *ReservationRouterInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterInitialized represents a Initialized event raised by the ReservationRouter contract.
type ReservationRouterInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ReservationRouter *ReservationRouterFilterer) FilterInitialized(opts *bind.FilterOpts) (*ReservationRouterInitializedIterator, error) {

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &ReservationRouterInitializedIterator{contract: _ReservationRouter.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_ReservationRouter *ReservationRouterFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *ReservationRouterInitialized) (event.Subscription, error) {

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterInitialized)
				if err := _ReservationRouter.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_ReservationRouter *ReservationRouterFilterer) ParseInitialized(log types.Log) (*ReservationRouterInitialized, error) {
	event := new(ReservationRouterInitialized)
	if err := _ReservationRouter.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationAcceptanceRequestedIterator is returned from FilterReservationAcceptanceRequested and is used to iterate over the raw logs and unpacked data for ReservationAcceptanceRequested events raised by the ReservationRouter contract.
type ReservationRouterReservationAcceptanceRequestedIterator struct {
	Event *ReservationRouterReservationAcceptanceRequested // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationAcceptanceRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationAcceptanceRequested)
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
		it.Event = new(ReservationRouterReservationAcceptanceRequested)
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
func (it *ReservationRouterReservationAcceptanceRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationAcceptanceRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationAcceptanceRequested represents a ReservationAcceptanceRequested event raised by the ReservationRouter contract.
type ReservationRouterReservationAcceptanceRequested struct {
	ReservationKey   *big.Int
	RequestNonce     uint64
	WalletPubKeyHash [20]byte
	DepositAmount    uint64
	TxMaxFee         uint64
	TimeoutAt        uint32
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterReservationAcceptanceRequested is a free log retrieval operation binding the contract event 0x1444a0a2e553520e4766f36c68368e10105489e52dc701d7fc9859c651475059.
//
// Solidity: event ReservationAcceptanceRequested(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed walletPubKeyHash, uint64 depositAmount, uint64 txMaxFee, uint32 timeoutAt)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationAcceptanceRequested(opts *bind.FilterOpts, reservationKey []*big.Int, walletPubKeyHash [][20]byte) (*ReservationRouterReservationAcceptanceRequestedIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationAcceptanceRequested", reservationKeyRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationAcceptanceRequestedIterator{contract: _ReservationRouter.contract, event: "ReservationAcceptanceRequested", logs: logs, sub: sub}, nil
}

// WatchReservationAcceptanceRequested is a free log subscription operation binding the contract event 0x1444a0a2e553520e4766f36c68368e10105489e52dc701d7fc9859c651475059.
//
// Solidity: event ReservationAcceptanceRequested(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed walletPubKeyHash, uint64 depositAmount, uint64 txMaxFee, uint32 timeoutAt)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationAcceptanceRequested(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationAcceptanceRequested, reservationKey []*big.Int, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationAcceptanceRequested", reservationKeyRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationAcceptanceRequested)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationAcceptanceRequested", log); err != nil {
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

// ParseReservationAcceptanceRequested is a log parse operation binding the contract event 0x1444a0a2e553520e4766f36c68368e10105489e52dc701d7fc9859c651475059.
//
// Solidity: event ReservationAcceptanceRequested(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed walletPubKeyHash, uint64 depositAmount, uint64 txMaxFee, uint32 timeoutAt)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationAcceptanceRequested(log types.Log) (*ReservationRouterReservationAcceptanceRequested, error) {
	event := new(ReservationRouterReservationAcceptanceRequested)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationAcceptanceRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationAcceptedIterator is returned from FilterReservationAccepted and is used to iterate over the raw logs and unpacked data for ReservationAccepted events raised by the ReservationRouter contract.
type ReservationRouterReservationAcceptedIterator struct {
	Event *ReservationRouterReservationAccepted // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationAcceptedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationAccepted)
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
		it.Event = new(ReservationRouterReservationAccepted)
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
func (it *ReservationRouterReservationAcceptedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationAcceptedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationAccepted represents a ReservationAccepted event raised by the ReservationRouter contract.
type ReservationRouterReservationAccepted struct {
	ReservationKey   *big.Int
	RequestNonce     uint64
	WalletPubKeyHash [20]byte
	Owner            common.Address
	AnchorTxHash     [32]byte
	AnchorAmount     uint64
	ExpiresAt        uint32
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterReservationAccepted is a free log retrieval operation binding the contract event 0xcdba3d32072456500fc4b138dd3c63bb0a72d568e71af8a51744bab51238b770.
//
// Solidity: event ReservationAccepted(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed walletPubKeyHash, address indexed owner, bytes32 anchorTxHash, uint64 anchorAmount, uint32 expiresAt)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationAccepted(opts *bind.FilterOpts, reservationKey []*big.Int, walletPubKeyHash [][20]byte, owner []common.Address) (*ReservationRouterReservationAcceptedIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationAccepted", reservationKeyRule, walletPubKeyHashRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationAcceptedIterator{contract: _ReservationRouter.contract, event: "ReservationAccepted", logs: logs, sub: sub}, nil
}

// WatchReservationAccepted is a free log subscription operation binding the contract event 0xcdba3d32072456500fc4b138dd3c63bb0a72d568e71af8a51744bab51238b770.
//
// Solidity: event ReservationAccepted(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed walletPubKeyHash, address indexed owner, bytes32 anchorTxHash, uint64 anchorAmount, uint32 expiresAt)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationAccepted(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationAccepted, reservationKey []*big.Int, walletPubKeyHash [][20]byte, owner []common.Address) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationAccepted", reservationKeyRule, walletPubKeyHashRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationAccepted)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationAccepted", log); err != nil {
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

// ParseReservationAccepted is a log parse operation binding the contract event 0xcdba3d32072456500fc4b138dd3c63bb0a72d568e71af8a51744bab51238b770.
//
// Solidity: event ReservationAccepted(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed walletPubKeyHash, address indexed owner, bytes32 anchorTxHash, uint64 anchorAmount, uint32 expiresAt)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationAccepted(log types.Log) (*ReservationRouterReservationAccepted, error) {
	event := new(ReservationRouterReservationAccepted)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationAccepted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationActionSupersededIterator is returned from FilterReservationActionSuperseded and is used to iterate over the raw logs and unpacked data for ReservationActionSuperseded events raised by the ReservationRouter contract.
type ReservationRouterReservationActionSupersededIterator struct {
	Event *ReservationRouterReservationActionSuperseded // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationActionSupersededIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationActionSuperseded)
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
		it.Event = new(ReservationRouterReservationActionSuperseded)
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
func (it *ReservationRouterReservationActionSupersededIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationActionSupersededIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationActionSuperseded represents a ReservationActionSuperseded event raised by the ReservationRouter contract.
type ReservationRouterReservationActionSuperseded struct {
	ReservationKey *big.Int
	RequestNonce   uint64
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterReservationActionSuperseded is a free log retrieval operation binding the contract event 0x64979c37b08d25f639dac3b74caf99840af5995eba8a493b29dc8599312ea252.
//
// Solidity: event ReservationActionSuperseded(uint256 indexed reservationKey, uint64 requestNonce)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationActionSuperseded(opts *bind.FilterOpts, reservationKey []*big.Int) (*ReservationRouterReservationActionSupersededIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationActionSuperseded", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationActionSupersededIterator{contract: _ReservationRouter.contract, event: "ReservationActionSuperseded", logs: logs, sub: sub}, nil
}

// WatchReservationActionSuperseded is a free log subscription operation binding the contract event 0x64979c37b08d25f639dac3b74caf99840af5995eba8a493b29dc8599312ea252.
//
// Solidity: event ReservationActionSuperseded(uint256 indexed reservationKey, uint64 requestNonce)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationActionSuperseded(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationActionSuperseded, reservationKey []*big.Int) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationActionSuperseded", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationActionSuperseded)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationActionSuperseded", log); err != nil {
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

// ParseReservationActionSuperseded is a log parse operation binding the contract event 0x64979c37b08d25f639dac3b74caf99840af5995eba8a493b29dc8599312ea252.
//
// Solidity: event ReservationActionSuperseded(uint256 indexed reservationKey, uint64 requestNonce)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationActionSuperseded(log types.Log) (*ReservationRouterReservationActionSuperseded, error) {
	event := new(ReservationRouterReservationActionSuperseded)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationActionSuperseded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationActionTimedOutIterator is returned from FilterReservationActionTimedOut and is used to iterate over the raw logs and unpacked data for ReservationActionTimedOut events raised by the ReservationRouter contract.
type ReservationRouterReservationActionTimedOutIterator struct {
	Event *ReservationRouterReservationActionTimedOut // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationActionTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationActionTimedOut)
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
		it.Event = new(ReservationRouterReservationActionTimedOut)
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
func (it *ReservationRouterReservationActionTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationActionTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationActionTimedOut represents a ReservationActionTimedOut event raised by the ReservationRouter contract.
type ReservationRouterReservationActionTimedOut struct {
	ReservationKey *big.Int
	RequestNonce   uint64
	ActionType     uint8
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterReservationActionTimedOut is a free log retrieval operation binding the contract event 0xd3bb43b8c8b259f4da0efa2c7a34ce683c05d6e31864299fa4a867bb3ff218ba.
//
// Solidity: event ReservationActionTimedOut(uint256 indexed reservationKey, uint64 requestNonce, uint8 actionType)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationActionTimedOut(opts *bind.FilterOpts, reservationKey []*big.Int) (*ReservationRouterReservationActionTimedOutIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationActionTimedOut", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationActionTimedOutIterator{contract: _ReservationRouter.contract, event: "ReservationActionTimedOut", logs: logs, sub: sub}, nil
}

// WatchReservationActionTimedOut is a free log subscription operation binding the contract event 0xd3bb43b8c8b259f4da0efa2c7a34ce683c05d6e31864299fa4a867bb3ff218ba.
//
// Solidity: event ReservationActionTimedOut(uint256 indexed reservationKey, uint64 requestNonce, uint8 actionType)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationActionTimedOut(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationActionTimedOut, reservationKey []*big.Int) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationActionTimedOut", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationActionTimedOut)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationActionTimedOut", log); err != nil {
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

// ParseReservationActionTimedOut is a log parse operation binding the contract event 0xd3bb43b8c8b259f4da0efa2c7a34ce683c05d6e31864299fa4a867bb3ff218ba.
//
// Solidity: event ReservationActionTimedOut(uint256 indexed reservationKey, uint64 requestNonce, uint8 actionType)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationActionTimedOut(log types.Log) (*ReservationRouterReservationActionTimedOut, error) {
	event := new(ReservationRouterReservationActionTimedOut)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationActionTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationCapsUpdatedIterator is returned from FilterReservationCapsUpdated and is used to iterate over the raw logs and unpacked data for ReservationCapsUpdated events raised by the ReservationRouter contract.
type ReservationRouterReservationCapsUpdatedIterator struct {
	Event *ReservationRouterReservationCapsUpdated // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationCapsUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationCapsUpdated)
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
		it.Event = new(ReservationRouterReservationCapsUpdated)
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
func (it *ReservationRouterReservationCapsUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationCapsUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationCapsUpdated represents a ReservationCapsUpdated event raised by the ReservationRouter contract.
type ReservationRouterReservationCapsUpdated struct {
	MaxReservationsAmountPerWallet uint64
	ReservationMaxSingleAmount     uint64
	MaxActiveReservations          uint32
	Raw                            types.Log // Blockchain specific contextual infos
}

// FilterReservationCapsUpdated is a free log retrieval operation binding the contract event 0x846df5edb182147898ee0a522f03f78718544ce16b0f06e64fe2f593b1fb160d.
//
// Solidity: event ReservationCapsUpdated(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount, uint32 maxActiveReservations)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationCapsUpdated(opts *bind.FilterOpts) (*ReservationRouterReservationCapsUpdatedIterator, error) {

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationCapsUpdated")
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationCapsUpdatedIterator{contract: _ReservationRouter.contract, event: "ReservationCapsUpdated", logs: logs, sub: sub}, nil
}

// WatchReservationCapsUpdated is a free log subscription operation binding the contract event 0x846df5edb182147898ee0a522f03f78718544ce16b0f06e64fe2f593b1fb160d.
//
// Solidity: event ReservationCapsUpdated(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount, uint32 maxActiveReservations)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationCapsUpdated(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationCapsUpdated) (event.Subscription, error) {

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationCapsUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationCapsUpdated)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationCapsUpdated", log); err != nil {
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

// ParseReservationCapsUpdated is a log parse operation binding the contract event 0x846df5edb182147898ee0a522f03f78718544ce16b0f06e64fe2f593b1fb160d.
//
// Solidity: event ReservationCapsUpdated(uint64 maxReservationsAmountPerWallet, uint64 reservationMaxSingleAmount, uint32 maxActiveReservations)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationCapsUpdated(log types.Log) (*ReservationRouterReservationCapsUpdated, error) {
	event := new(ReservationRouterReservationCapsUpdated)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationCapsUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationLateSettledIterator is returned from FilterReservationLateSettled and is used to iterate over the raw logs and unpacked data for ReservationLateSettled events raised by the ReservationRouter contract.
type ReservationRouterReservationLateSettledIterator struct {
	Event *ReservationRouterReservationLateSettled // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationLateSettledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationLateSettled)
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
		it.Event = new(ReservationRouterReservationLateSettled)
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
func (it *ReservationRouterReservationLateSettledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationLateSettledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationLateSettled represents a ReservationLateSettled event raised by the ReservationRouter contract.
type ReservationRouterReservationLateSettled struct {
	ReservationKey *big.Int
	RequestNonce   uint64
	ActionType     uint8
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterReservationLateSettled is a free log retrieval operation binding the contract event 0x152c5b7b78a634931e032d1ab3d0c033e2e5d00e0e75f20252767746a1fa4f6d.
//
// Solidity: event ReservationLateSettled(uint256 indexed reservationKey, uint64 requestNonce, uint8 actionType)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationLateSettled(opts *bind.FilterOpts, reservationKey []*big.Int) (*ReservationRouterReservationLateSettledIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationLateSettled", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationLateSettledIterator{contract: _ReservationRouter.contract, event: "ReservationLateSettled", logs: logs, sub: sub}, nil
}

// WatchReservationLateSettled is a free log subscription operation binding the contract event 0x152c5b7b78a634931e032d1ab3d0c033e2e5d00e0e75f20252767746a1fa4f6d.
//
// Solidity: event ReservationLateSettled(uint256 indexed reservationKey, uint64 requestNonce, uint8 actionType)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationLateSettled(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationLateSettled, reservationKey []*big.Int) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationLateSettled", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationLateSettled)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationLateSettled", log); err != nil {
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

// ParseReservationLateSettled is a log parse operation binding the contract event 0x152c5b7b78a634931e032d1ab3d0c033e2e5d00e0e75f20252767746a1fa4f6d.
//
// Solidity: event ReservationLateSettled(uint256 indexed reservationKey, uint64 requestNonce, uint8 actionType)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationLateSettled(log types.Log) (*ReservationRouterReservationLateSettled, error) {
	event := new(ReservationRouterReservationLateSettled)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationLateSettled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationParametersUpdatedIterator is returned from FilterReservationParametersUpdated and is used to iterate over the raw logs and unpacked data for ReservationParametersUpdated events raised by the ReservationRouter contract.
type ReservationRouterReservationParametersUpdatedIterator struct {
	Event *ReservationRouterReservationParametersUpdated // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationParametersUpdated)
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
		it.Event = new(ReservationRouterReservationParametersUpdated)
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
func (it *ReservationRouterReservationParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationParametersUpdated represents a ReservationParametersUpdated event raised by the ReservationRouter contract.
type ReservationRouterReservationParametersUpdated struct {
	ReservationMinAmount            uint64
	ReservationTxMaxFee             uint64
	ReservationTermSeconds          uint32
	ReservationDissolutionDelay     uint32
	ReservationMaxTotalAmount       uint64
	MaxReservationsPerWallet        uint32
	ReservationActionTimeout        uint32
	ReservationRenewalWindowSeconds uint32
	Raw                             types.Log // Blockchain specific contextual infos
}

// FilterReservationParametersUpdated is a free log retrieval operation binding the contract event 0x7e6c56281c83edd8db45ddcb09afd1cc2cc009bcc94213c657592cb15a5b2901.
//
// Solidity: event ReservationParametersUpdated(uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationParametersUpdated(opts *bind.FilterOpts) (*ReservationRouterReservationParametersUpdatedIterator, error) {

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationParametersUpdatedIterator{contract: _ReservationRouter.contract, event: "ReservationParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchReservationParametersUpdated is a free log subscription operation binding the contract event 0x7e6c56281c83edd8db45ddcb09afd1cc2cc009bcc94213c657592cb15a5b2901.
//
// Solidity: event ReservationParametersUpdated(uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationParametersUpdated(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationParametersUpdated)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationParametersUpdated", log); err != nil {
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

// ParseReservationParametersUpdated is a log parse operation binding the contract event 0x7e6c56281c83edd8db45ddcb09afd1cc2cc009bcc94213c657592cb15a5b2901.
//
// Solidity: event ReservationParametersUpdated(uint64 reservationMinAmount, uint64 reservationTxMaxFee, uint32 reservationTermSeconds, uint32 reservationDissolutionDelay, uint64 reservationMaxTotalAmount, uint32 maxReservationsPerWallet, uint32 reservationActionTimeout, uint32 reservationRenewalWindowSeconds)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationParametersUpdated(log types.Log) (*ReservationRouterReservationParametersUpdated, error) {
	event := new(ReservationRouterReservationParametersUpdated)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationReanchorRequestedIterator is returned from FilterReservationReanchorRequested and is used to iterate over the raw logs and unpacked data for ReservationReanchorRequested events raised by the ReservationRouter contract.
type ReservationRouterReservationReanchorRequestedIterator struct {
	Event *ReservationRouterReservationReanchorRequested // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationReanchorRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationReanchorRequested)
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
		it.Event = new(ReservationRouterReservationReanchorRequested)
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
func (it *ReservationRouterReservationReanchorRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationReanchorRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationReanchorRequested represents a ReservationReanchorRequested event raised by the ReservationRouter contract.
type ReservationRouterReservationReanchorRequested struct {
	ReservationKey         *big.Int
	RequestNonce           uint64
	SourceWalletPubKeyHash [20]byte
	TargetWalletPubKeyHash [20]byte
	TxMaxFee               uint64
	Raw                    types.Log // Blockchain specific contextual infos
}

// FilterReservationReanchorRequested is a free log retrieval operation binding the contract event 0x90323e7ede1e7009754d91387f23522528e6356b2667952ff305a500ffaa9c6d.
//
// Solidity: event ReservationReanchorRequested(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed sourceWalletPubKeyHash, bytes20 indexed targetWalletPubKeyHash, uint64 txMaxFee)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationReanchorRequested(opts *bind.FilterOpts, reservationKey []*big.Int, sourceWalletPubKeyHash [][20]byte, targetWalletPubKeyHash [][20]byte) (*ReservationRouterReservationReanchorRequestedIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var sourceWalletPubKeyHashRule []interface{}
	for _, sourceWalletPubKeyHashItem := range sourceWalletPubKeyHash {
		sourceWalletPubKeyHashRule = append(sourceWalletPubKeyHashRule, sourceWalletPubKeyHashItem)
	}
	var targetWalletPubKeyHashRule []interface{}
	for _, targetWalletPubKeyHashItem := range targetWalletPubKeyHash {
		targetWalletPubKeyHashRule = append(targetWalletPubKeyHashRule, targetWalletPubKeyHashItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationReanchorRequested", reservationKeyRule, sourceWalletPubKeyHashRule, targetWalletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationReanchorRequestedIterator{contract: _ReservationRouter.contract, event: "ReservationReanchorRequested", logs: logs, sub: sub}, nil
}

// WatchReservationReanchorRequested is a free log subscription operation binding the contract event 0x90323e7ede1e7009754d91387f23522528e6356b2667952ff305a500ffaa9c6d.
//
// Solidity: event ReservationReanchorRequested(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed sourceWalletPubKeyHash, bytes20 indexed targetWalletPubKeyHash, uint64 txMaxFee)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationReanchorRequested(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationReanchorRequested, reservationKey []*big.Int, sourceWalletPubKeyHash [][20]byte, targetWalletPubKeyHash [][20]byte) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var sourceWalletPubKeyHashRule []interface{}
	for _, sourceWalletPubKeyHashItem := range sourceWalletPubKeyHash {
		sourceWalletPubKeyHashRule = append(sourceWalletPubKeyHashRule, sourceWalletPubKeyHashItem)
	}
	var targetWalletPubKeyHashRule []interface{}
	for _, targetWalletPubKeyHashItem := range targetWalletPubKeyHash {
		targetWalletPubKeyHashRule = append(targetWalletPubKeyHashRule, targetWalletPubKeyHashItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationReanchorRequested", reservationKeyRule, sourceWalletPubKeyHashRule, targetWalletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationReanchorRequested)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationReanchorRequested", log); err != nil {
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

// ParseReservationReanchorRequested is a log parse operation binding the contract event 0x90323e7ede1e7009754d91387f23522528e6356b2667952ff305a500ffaa9c6d.
//
// Solidity: event ReservationReanchorRequested(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed sourceWalletPubKeyHash, bytes20 indexed targetWalletPubKeyHash, uint64 txMaxFee)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationReanchorRequested(log types.Log) (*ReservationRouterReservationReanchorRequested, error) {
	event := new(ReservationRouterReservationReanchorRequested)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationReanchorRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationReanchoredIterator is returned from FilterReservationReanchored and is used to iterate over the raw logs and unpacked data for ReservationReanchored events raised by the ReservationRouter contract.
type ReservationRouterReservationReanchoredIterator struct {
	Event *ReservationRouterReservationReanchored // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationReanchoredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationReanchored)
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
		it.Event = new(ReservationRouterReservationReanchored)
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
func (it *ReservationRouterReservationReanchoredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationReanchoredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationReanchored represents a ReservationReanchored event raised by the ReservationRouter contract.
type ReservationRouterReservationReanchored struct {
	ReservationKey      *big.Int
	RequestNonce        uint64
	NewWalletPubKeyHash [20]byte
	NewAnchorTxHash     [32]byte
	NewAnchorAmount     uint64
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterReservationReanchored is a free log retrieval operation binding the contract event 0xe42922c665e9600f84def7070f9dfeeb5dd650fb3522fb6bb2326e676bd23319.
//
// Solidity: event ReservationReanchored(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed newWalletPubKeyHash, bytes32 newAnchorTxHash, uint64 newAnchorAmount)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationReanchored(opts *bind.FilterOpts, reservationKey []*big.Int, newWalletPubKeyHash [][20]byte) (*ReservationRouterReservationReanchoredIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var newWalletPubKeyHashRule []interface{}
	for _, newWalletPubKeyHashItem := range newWalletPubKeyHash {
		newWalletPubKeyHashRule = append(newWalletPubKeyHashRule, newWalletPubKeyHashItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationReanchored", reservationKeyRule, newWalletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationReanchoredIterator{contract: _ReservationRouter.contract, event: "ReservationReanchored", logs: logs, sub: sub}, nil
}

// WatchReservationReanchored is a free log subscription operation binding the contract event 0xe42922c665e9600f84def7070f9dfeeb5dd650fb3522fb6bb2326e676bd23319.
//
// Solidity: event ReservationReanchored(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed newWalletPubKeyHash, bytes32 newAnchorTxHash, uint64 newAnchorAmount)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationReanchored(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationReanchored, reservationKey []*big.Int, newWalletPubKeyHash [][20]byte) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	var newWalletPubKeyHashRule []interface{}
	for _, newWalletPubKeyHashItem := range newWalletPubKeyHash {
		newWalletPubKeyHashRule = append(newWalletPubKeyHashRule, newWalletPubKeyHashItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationReanchored", reservationKeyRule, newWalletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationReanchored)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationReanchored", log); err != nil {
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

// ParseReservationReanchored is a log parse operation binding the contract event 0xe42922c665e9600f84def7070f9dfeeb5dd650fb3522fb6bb2326e676bd23319.
//
// Solidity: event ReservationReanchored(uint256 indexed reservationKey, uint64 requestNonce, bytes20 indexed newWalletPubKeyHash, bytes32 newAnchorTxHash, uint64 newAnchorAmount)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationReanchored(log types.Log) (*ReservationRouterReservationReanchored, error) {
	event := new(ReservationRouterReservationReanchored)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationReanchored", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationRetryCreditMintedIterator is returned from FilterReservationRetryCreditMinted and is used to iterate over the raw logs and unpacked data for ReservationRetryCreditMinted events raised by the ReservationRouter contract.
type ReservationRouterReservationRetryCreditMintedIterator struct {
	Event *ReservationRouterReservationRetryCreditMinted // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationRetryCreditMintedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationRetryCreditMinted)
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
		it.Event = new(ReservationRouterReservationRetryCreditMinted)
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
func (it *ReservationRouterReservationRetryCreditMintedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationRetryCreditMintedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationRetryCreditMinted represents a ReservationRetryCreditMinted event raised by the ReservationRouter contract.
type ReservationRouterReservationRetryCreditMinted struct {
	ReservationKey *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterReservationRetryCreditMinted is a free log retrieval operation binding the contract event 0x919795353bc408e11a0d5a133a9c6969367014ad975910a8b31d08b12597c4c2.
//
// Solidity: event ReservationRetryCreditMinted(uint256 indexed reservationKey)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationRetryCreditMinted(opts *bind.FilterOpts, reservationKey []*big.Int) (*ReservationRouterReservationRetryCreditMintedIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationRetryCreditMinted", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationRetryCreditMintedIterator{contract: _ReservationRouter.contract, event: "ReservationRetryCreditMinted", logs: logs, sub: sub}, nil
}

// WatchReservationRetryCreditMinted is a free log subscription operation binding the contract event 0x919795353bc408e11a0d5a133a9c6969367014ad975910a8b31d08b12597c4c2.
//
// Solidity: event ReservationRetryCreditMinted(uint256 indexed reservationKey)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationRetryCreditMinted(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationRetryCreditMinted, reservationKey []*big.Int) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationRetryCreditMinted", reservationKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationRetryCreditMinted)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationRetryCreditMinted", log); err != nil {
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

// ParseReservationRetryCreditMinted is a log parse operation binding the contract event 0x919795353bc408e11a0d5a133a9c6969367014ad975910a8b31d08b12597c4c2.
//
// Solidity: event ReservationRetryCreditMinted(uint256 indexed reservationKey)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationRetryCreditMinted(log types.Log) (*ReservationRouterReservationRetryCreditMinted, error) {
	event := new(ReservationRouterReservationRetryCreditMinted)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationRetryCreditMinted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationRouterSetIterator is returned from FilterReservationRouterSet and is used to iterate over the raw logs and unpacked data for ReservationRouterSet events raised by the ReservationRouter contract.
type ReservationRouterReservationRouterSetIterator struct {
	Event *ReservationRouterReservationRouterSet // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationRouterSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationRouterSet)
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
		it.Event = new(ReservationRouterReservationRouterSet)
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
func (it *ReservationRouterReservationRouterSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationRouterSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationRouterSet represents a ReservationRouterSet event raised by the ReservationRouter contract.
type ReservationRouterReservationRouterSet struct {
	ReservationRouter common.Address
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterReservationRouterSet is a free log retrieval operation binding the contract event 0xd9eacf62803dd1f1bb5342d8eb5951546c371915e06223f589c0c95486c7c769.
//
// Solidity: event ReservationRouterSet(address reservationRouter)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationRouterSet(opts *bind.FilterOpts) (*ReservationRouterReservationRouterSetIterator, error) {

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationRouterSet")
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationRouterSetIterator{contract: _ReservationRouter.contract, event: "ReservationRouterSet", logs: logs, sub: sub}, nil
}

// WatchReservationRouterSet is a free log subscription operation binding the contract event 0xd9eacf62803dd1f1bb5342d8eb5951546c371915e06223f589c0c95486c7c769.
//
// Solidity: event ReservationRouterSet(address reservationRouter)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationRouterSet(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationRouterSet) (event.Subscription, error) {

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationRouterSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationRouterSet)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationRouterSet", log); err != nil {
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

// ParseReservationRouterSet is a log parse operation binding the contract event 0xd9eacf62803dd1f1bb5342d8eb5951546c371915e06223f589c0c95486c7c769.
//
// Solidity: event ReservationRouterSet(address reservationRouter)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationRouterSet(log types.Log) (*ReservationRouterReservationRouterSet, error) {
	event := new(ReservationRouterReservationRouterSet)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationRouterSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationStrandedIterator is returned from FilterReservationStranded and is used to iterate over the raw logs and unpacked data for ReservationStranded events raised by the ReservationRouter contract.
type ReservationRouterReservationStrandedIterator struct {
	Event *ReservationRouterReservationStranded // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationStrandedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationStranded)
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
		it.Event = new(ReservationRouterReservationStranded)
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
func (it *ReservationRouterReservationStrandedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationStrandedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationStranded represents a ReservationStranded event raised by the ReservationRouter contract.
type ReservationRouterReservationStranded struct {
	ReservationKey   *big.Int
	WalletPubKeyHash [20]byte
	Owner            common.Address
	AnchorAmount     uint64
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterReservationStranded is a free log retrieval operation binding the contract event 0x95a304fee209cd8169392534093ab2cb9ee3b8c7031cf873b2f0c40a03b44d4d.
//
// Solidity: event ReservationStranded(uint256 indexed reservationKey, bytes20 indexed walletPubKeyHash, address indexed owner, uint64 anchorAmount)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationStranded(opts *bind.FilterOpts, reservationKey []*big.Int, walletPubKeyHash [][20]byte, owner []common.Address) (*ReservationRouterReservationStrandedIterator, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationStranded", reservationKeyRule, walletPubKeyHashRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationStrandedIterator{contract: _ReservationRouter.contract, event: "ReservationStranded", logs: logs, sub: sub}, nil
}

// WatchReservationStranded is a free log subscription operation binding the contract event 0x95a304fee209cd8169392534093ab2cb9ee3b8c7031cf873b2f0c40a03b44d4d.
//
// Solidity: event ReservationStranded(uint256 indexed reservationKey, bytes20 indexed walletPubKeyHash, address indexed owner, uint64 anchorAmount)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationStranded(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationStranded, reservationKey []*big.Int, walletPubKeyHash [][20]byte, owner []common.Address) (event.Subscription, error) {

	var reservationKeyRule []interface{}
	for _, reservationKeyItem := range reservationKey {
		reservationKeyRule = append(reservationKeyRule, reservationKeyItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationStranded", reservationKeyRule, walletPubKeyHashRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationStranded)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationStranded", log); err != nil {
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

// ParseReservationStranded is a log parse operation binding the contract event 0x95a304fee209cd8169392534093ab2cb9ee3b8c7031cf873b2f0c40a03b44d4d.
//
// Solidity: event ReservationStranded(uint256 indexed reservationKey, bytes20 indexed walletPubKeyHash, address indexed owner, uint64 anchorAmount)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationStranded(log types.Log) (*ReservationRouterReservationStranded, error) {
	event := new(ReservationRouterReservationStranded)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationStranded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservationVaultUpdatedIterator is returned from FilterReservationVaultUpdated and is used to iterate over the raw logs and unpacked data for ReservationVaultUpdated events raised by the ReservationRouter contract.
type ReservationRouterReservationVaultUpdatedIterator struct {
	Event *ReservationRouterReservationVaultUpdated // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservationVaultUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservationVaultUpdated)
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
		it.Event = new(ReservationRouterReservationVaultUpdated)
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
func (it *ReservationRouterReservationVaultUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservationVaultUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservationVaultUpdated represents a ReservationVaultUpdated event raised by the ReservationRouter contract.
type ReservationRouterReservationVaultUpdated struct {
	ReservationVault common.Address
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterReservationVaultUpdated is a free log retrieval operation binding the contract event 0x81b37784221191020714846b5fdcdfcbde796cfad2627d47dc81c2b7765b1910.
//
// Solidity: event ReservationVaultUpdated(address reservationVault)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservationVaultUpdated(opts *bind.FilterOpts) (*ReservationRouterReservationVaultUpdatedIterator, error) {

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservationVaultUpdated")
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservationVaultUpdatedIterator{contract: _ReservationRouter.contract, event: "ReservationVaultUpdated", logs: logs, sub: sub}, nil
}

// WatchReservationVaultUpdated is a free log subscription operation binding the contract event 0x81b37784221191020714846b5fdcdfcbde796cfad2627d47dc81c2b7765b1910.
//
// Solidity: event ReservationVaultUpdated(address reservationVault)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservationVaultUpdated(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservationVaultUpdated) (event.Subscription, error) {

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservationVaultUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservationVaultUpdated)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservationVaultUpdated", log); err != nil {
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

// ParseReservationVaultUpdated is a log parse operation binding the contract event 0x81b37784221191020714846b5fdcdfcbde796cfad2627d47dc81c2b7765b1910.
//
// Solidity: event ReservationVaultUpdated(address reservationVault)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservationVaultUpdated(log types.Log) (*ReservationRouterReservationVaultUpdated, error) {
	event := new(ReservationRouterReservationVaultUpdated)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservationVaultUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ReservationRouterReservedDepositMarkedStaleIterator is returned from FilterReservedDepositMarkedStale and is used to iterate over the raw logs and unpacked data for ReservedDepositMarkedStale events raised by the ReservationRouter contract.
type ReservationRouterReservedDepositMarkedStaleIterator struct {
	Event *ReservationRouterReservedDepositMarkedStale // Event containing the contract specifics and raw log

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
func (it *ReservationRouterReservedDepositMarkedStaleIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ReservationRouterReservedDepositMarkedStale)
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
		it.Event = new(ReservationRouterReservedDepositMarkedStale)
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
func (it *ReservationRouterReservedDepositMarkedStaleIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ReservationRouterReservedDepositMarkedStaleIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ReservationRouterReservedDepositMarkedStale represents a ReservedDepositMarkedStale event raised by the ReservationRouter contract.
type ReservationRouterReservedDepositMarkedStale struct {
	DepositKey *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterReservedDepositMarkedStale is a free log retrieval operation binding the contract event 0xf4a10f7395c3a5e8165714c665ddb7f2e320f8bc2a21d9f8722f48f8d1d71eab.
//
// Solidity: event ReservedDepositMarkedStale(uint256 indexed depositKey)
func (_ReservationRouter *ReservationRouterFilterer) FilterReservedDepositMarkedStale(opts *bind.FilterOpts, depositKey []*big.Int) (*ReservationRouterReservedDepositMarkedStaleIterator, error) {

	var depositKeyRule []interface{}
	for _, depositKeyItem := range depositKey {
		depositKeyRule = append(depositKeyRule, depositKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.FilterLogs(opts, "ReservedDepositMarkedStale", depositKeyRule)
	if err != nil {
		return nil, err
	}
	return &ReservationRouterReservedDepositMarkedStaleIterator{contract: _ReservationRouter.contract, event: "ReservedDepositMarkedStale", logs: logs, sub: sub}, nil
}

// WatchReservedDepositMarkedStale is a free log subscription operation binding the contract event 0xf4a10f7395c3a5e8165714c665ddb7f2e320f8bc2a21d9f8722f48f8d1d71eab.
//
// Solidity: event ReservedDepositMarkedStale(uint256 indexed depositKey)
func (_ReservationRouter *ReservationRouterFilterer) WatchReservedDepositMarkedStale(opts *bind.WatchOpts, sink chan<- *ReservationRouterReservedDepositMarkedStale, depositKey []*big.Int) (event.Subscription, error) {

	var depositKeyRule []interface{}
	for _, depositKeyItem := range depositKey {
		depositKeyRule = append(depositKeyRule, depositKeyItem)
	}

	logs, sub, err := _ReservationRouter.contract.WatchLogs(opts, "ReservedDepositMarkedStale", depositKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ReservationRouterReservedDepositMarkedStale)
				if err := _ReservationRouter.contract.UnpackLog(event, "ReservedDepositMarkedStale", log); err != nil {
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

// ParseReservedDepositMarkedStale is a log parse operation binding the contract event 0xf4a10f7395c3a5e8165714c665ddb7f2e320f8bc2a21d9f8722f48f8d1d71eab.
//
// Solidity: event ReservedDepositMarkedStale(uint256 indexed depositKey)
func (_ReservationRouter *ReservationRouterFilterer) ParseReservedDepositMarkedStale(log types.Log) (*ReservationRouterReservedDepositMarkedStale, error) {
	event := new(ReservationRouterReservedDepositMarkedStale)
	if err := _ReservationRouter.contract.UnpackLog(event, "ReservedDepositMarkedStale", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

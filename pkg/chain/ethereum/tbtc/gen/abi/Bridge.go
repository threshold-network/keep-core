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

// BitcoinTxInfo is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxInfo struct {
	Version      [4]byte
	InputVector  []byte
	OutputVector []byte
	Locktime     [4]byte
}

// BitcoinTxProof is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxProof struct {
	MerkleProof      []byte
	TxIndexInBlock   *big.Int
	BitcoinHeaders   []byte
	CoinbasePreimage [32]byte
	CoinbaseProof    []byte
}

// BitcoinTxRSVSignature is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxRSVSignature struct {
	R [32]byte
	S [32]byte
	V uint8
}

// BitcoinTxUTXO is an auto generated low-level Go binding around an user-defined struct.
type BitcoinTxUTXO struct {
	TxHash        [32]byte
	TxOutputIndex uint32
	TxOutputValue uint64
}

// DepositDepositRequest is an auto generated low-level Go binding around an user-defined struct.
type DepositDepositRequest struct {
	Depositor   common.Address
	Amount      uint64
	RevealedAt  uint32
	Vault       common.Address
	TreasuryFee uint64
	SweptAt     uint32
	ExtraData   [32]byte
}

// DepositDepositRevealInfo is an auto generated low-level Go binding around an user-defined struct.
type DepositDepositRevealInfo struct {
	FundingOutputIndex uint32
	BlindingFactor     [8]byte
	WalletPubKeyHash   [20]byte
	RefundPubKeyHash   [20]byte
	RefundLocktime     [4]byte
	Vault              common.Address
}

// FraudFraudChallenge is an auto generated low-level Go binding around an user-defined struct.
type FraudFraudChallenge struct {
	Challenger    common.Address
	DepositAmount *big.Int
	ReportedAt    uint32
	Resolved      bool
}

// MovingFundsMovedFundsSweepRequest is an auto generated low-level Go binding around an user-defined struct.
type MovingFundsMovedFundsSweepRequest struct {
	WalletPubKeyHash [20]byte
	Value            uint64
	CreatedAt        uint32
	State            uint8
}

// RedemptionRedemptionRequest is an auto generated low-level Go binding around an user-defined struct.
type RedemptionRedemptionRequest struct {
	Redeemer        common.Address
	RequestedAmount uint64
	TreasuryFee     uint64
	TxMaxFee        uint64
	RequestedAt     uint32
}

// WalletsWallet is an auto generated low-level Go binding around an user-defined struct.
type WalletsWallet struct {
	EcdsaWalletID                          [32]byte
	MainUtxoHash                           [32]byte
	PendingRedemptionsValue                uint64
	CreatedAt                              uint32
	MovingFundsRequestedAt                 uint32
	ClosingStartedAt                       uint32
	PendingMovedFundsSweepRequestsCount    uint32
	State                                  uint8
	MovingFundsTargetWalletsCommitmentHash [32]byte
}

// BridgeMetaData contains all meta data concerning the Bridge contract.
var BridgeMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"depositDustThreshold\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"depositTreasuryFeeDivisor\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"depositTxMaxFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"depositRevealAheadPeriod\",\"type\":\"uint32\"}],\"name\":\"DepositParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fundingTxHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"fundingOutputIndex\",\"type\":\"uint32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"depositor\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"amount\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"bytes8\",\"name\":\"blindingFactor\",\"type\":\"bytes8\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes20\",\"name\":\"refundPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes4\",\"name\":\"refundLocktime\",\"type\":\"bytes4\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"}],\"name\":\"DepositRevealed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"depositKey\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newVault\",\"type\":\"address\"}],\"name\":\"DepositVaultFixed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"sweepTxHash\",\"type\":\"bytes32\"}],\"name\":\"DepositsSwept\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"sighash\",\"type\":\"bytes32\"}],\"name\":\"FraudChallengeDefeatTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"sighash\",\"type\":\"bytes32\"}],\"name\":\"FraudChallengeDefeated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"sighash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"v\",\"type\":\"uint8\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"r\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"s\",\"type\":\"bytes32\"}],\"name\":\"FraudChallengeSubmitted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fraudChallengeDepositAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"fraudChallengeDefeatTimeout\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"fraudSlashingAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"fraudNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"name\":\"FraudParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldGovernance\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"GovernanceTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"version\",\"type\":\"uint8\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"movingFundsTxHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"movingFundsTxOutputIndex\",\"type\":\"uint32\"}],\"name\":\"MovedFundsSweepTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"sweepTxHash\",\"type\":\"bytes32\"}],\"name\":\"MovedFundsSwept\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"MovingFundsBelowDustReported\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes20[]\",\"name\":\"targetWallets\",\"type\":\"bytes20[]\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"submitter\",\"type\":\"address\"}],\"name\":\"MovingFundsCommitmentSubmitted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"movingFundsTxHash\",\"type\":\"bytes32\"}],\"name\":\"MovingFundsCompleted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"movingFundsTxMaxTotalFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"movingFundsDustThreshold\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"movingFundsTimeoutResetDelay\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"movingFundsTimeout\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"movingFundsTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"movingFundsTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"movingFundsCommitmentGasOffset\",\"type\":\"uint16\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"movedFundsSweepTxMaxTotalFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"movedFundsSweepTimeout\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"movedFundsSweepTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"movedFundsSweepTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"name\":\"MovingFundsParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"MovingFundsTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"MovingFundsTimeoutReset\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"ecdsaWalletID\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"NewWalletRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"NewWalletRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldRebateStaking\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newRebateStaking\",\"type\":\"address\"}],\"name\":\"RebateStakingRepaired\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"rebateStaking\",\"type\":\"address\"}],\"name\":\"RebateStakingSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"redemptionDustThreshold\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"redemptionTreasuryFeeDivisor\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"redemptionTxMaxFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"redemptionTxMaxTotalFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"redemptionTimeout\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint96\",\"name\":\"redemptionTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"redemptionTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"name\":\"RedemptionParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"redeemerOutputScript\",\"type\":\"bytes\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"requestedAmount\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"treasuryFee\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"txMaxFee\",\"type\":\"uint64\"}],\"name\":\"RedemptionRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"redeemerOutputScript\",\"type\":\"bytes\"}],\"name\":\"RedemptionTimedOut\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"redemptionWatchtower\",\"type\":\"address\"}],\"name\":\"RedemptionWatchtowerSet\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"redemptionTxHash\",\"type\":\"bytes32\"}],\"name\":\"RedemptionsCompleted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"spvMaintainer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"isTrusted\",\"type\":\"bool\"}],\"name\":\"SpvMaintainerStatusUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"treasury\",\"type\":\"address\"}],\"name\":\"TreasuryUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"isTrusted\",\"type\":\"bool\"}],\"name\":\"VaultStatusUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"ecdsaWalletID\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"WalletClosed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"ecdsaWalletID\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"WalletClosing\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"ecdsaWalletID\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"WalletMovingFunds\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"walletCreationPeriod\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"walletCreationMinBtcBalance\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"walletCreationMaxBtcBalance\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"walletClosureMinBtcBalance\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"walletMaxAge\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"walletMaxBtcTransfer\",\"type\":\"uint64\"},{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"walletClosingPeriod\",\"type\":\"uint32\"}],\"name\":\"WalletParametersUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"ecdsaWalletID\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"WalletTerminated\",\"type\":\"event\"},{\"stateMutability\":\"payable\",\"type\":\"fallback\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"ecdsaWalletID\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"publicKeyX\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"publicKeyY\",\"type\":\"bytes32\"}],\"name\":\"__ecdsaWalletCreatedCallback\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"publicKeyX\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"publicKeyY\",\"type\":\"bytes32\"}],\"name\":\"__ecdsaWalletHeartbeatFailedCallback\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"activeWalletPubKeyHash\",\"outputs\":[{\"internalType\":\"bytes20\",\"name\":\"\",\"type\":\"bytes20\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"contractReferences\",\"outputs\":[{\"internalType\":\"contractBank\",\"name\":\"bank\",\"type\":\"address\"},{\"internalType\":\"contractIRelay\",\"name\":\"relay\",\"type\":\"address\"},{\"internalType\":\"contractIWalletRegistry\",\"name\":\"ecdsaWalletRegistry\",\"type\":\"address\"},{\"internalType\":\"contractReimbursementPool\",\"name\":\"reimbursementPool\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"walletPublicKey\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"preimage\",\"type\":\"bytes\"},{\"internalType\":\"bool\",\"name\":\"witness\",\"type\":\"bool\"}],\"name\":\"defeatFraudChallenge\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"walletPublicKey\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"heartbeatMessage\",\"type\":\"bytes\"}],\"name\":\"defeatFraudChallengeWithHeartbeat\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"depositParameters\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"depositDustThreshold\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"depositTreasuryFeeDivisor\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"depositTxMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"depositRevealAheadPeriod\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"depositKey\",\"type\":\"uint256\"}],\"name\":\"deposits\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"depositor\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"amount\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"revealedAt\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"treasuryFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"sweptAt\",\"type\":\"uint32\"},{\"internalType\":\"bytes32\",\"name\":\"extraData\",\"type\":\"bytes32\"}],\"internalType\":\"structDeposit.DepositRequest\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"challengeKey\",\"type\":\"uint256\"}],\"name\":\"fraudChallenges\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"challenger\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"depositAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint32\",\"name\":\"reportedAt\",\"type\":\"uint32\"},{\"internalType\":\"bool\",\"name\":\"resolved\",\"type\":\"bool\"}],\"internalType\":\"structFraud.FraudChallenge\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"fraudParameters\",\"outputs\":[{\"internalType\":\"uint96\",\"name\":\"fraudChallengeDepositAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"fraudChallengeDefeatTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"fraudSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"fraudNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getRebateStaking\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getRedemptionWatchtower\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getReservationRouter\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"governance\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_bank\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_relay\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_treasury\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_ecdsaWalletRegistry\",\"type\":\"address\"},{\"internalType\":\"addresspayable\",\"name\":\"_reimbursementPool\",\"type\":\"address\"},{\"internalType\":\"uint96\",\"name\":\"_txProofDifficultyFactor\",\"type\":\"uint96\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"initializeV2_FixVaultZeroDeposit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newRebateStaking\",\"type\":\"address\"}],\"name\":\"initializeV5_RepairRebateStaking\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"depositKey\",\"type\":\"uint256\"}],\"name\":\"isReservedDeposit\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"}],\"name\":\"isVaultTrusted\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"liveWalletsCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"requestKey\",\"type\":\"uint256\"}],\"name\":\"movedFundsSweepRequests\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"uint64\",\"name\":\"value\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"createdAt\",\"type\":\"uint32\"},{\"internalType\":\"enumMovingFunds.MovedFundsSweepRequestState\",\"name\":\"state\",\"type\":\"uint8\"}],\"internalType\":\"structMovingFunds.MovedFundsSweepRequest\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"movingFundsParameters\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"movingFundsTxMaxTotalFee\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"movingFundsDustThreshold\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTimeoutResetDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"movingFundsTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"},{\"internalType\":\"uint16\",\"name\":\"movingFundsCommitmentGasOffset\",\"type\":\"uint16\"},{\"internalType\":\"uint64\",\"name\":\"movedFundsSweepTxMaxTotalFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"movedFundsSweepTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"movedFundsSweepTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"movedFundsSweepTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"walletPublicKey\",\"type\":\"bytes\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes\",\"name\":\"preimageSha256\",\"type\":\"bytes\"}],\"name\":\"notifyFraudChallengeDefeatTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"movingFundsTxHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTxOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"}],\"name\":\"notifyMovedFundsSweepTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"}],\"name\":\"notifyMovingFundsBelowDust\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"}],\"name\":\"notifyMovingFundsTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"},{\"internalType\":\"bytes\",\"name\":\"redeemerOutputScript\",\"type\":\"bytes\"}],\"name\":\"notifyRedemptionTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes\",\"name\":\"redeemerOutputScript\",\"type\":\"bytes\"}],\"name\":\"notifyRedemptionVeto\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"walletMainUtxo\",\"type\":\"tuple\"}],\"name\":\"notifyWalletCloseable\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"notifyWalletClosingPeriodElapsed\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"}],\"name\":\"pendingRedemptions\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"requestedAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"treasuryFee\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"txMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"requestedAt\",\"type\":\"uint32\"}],\"internalType\":\"structRedemption.RedemptionRequest\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"balanceOwner\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"redemptionData\",\"type\":\"bytes\"}],\"name\":\"receiveBalanceApproval\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"redemptionParameters\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"redemptionDustThreshold\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"redemptionTreasuryFeeDivisor\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"redemptionTxMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"redemptionTxMaxTotalFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"redemptionTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"redemptionTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"redemptionTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"activeWalletMainUtxo\",\"type\":\"tuple\"}],\"name\":\"requestNewWallet\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"bytes\",\"name\":\"redeemerOutputScript\",\"type\":\"bytes\"},{\"internalType\":\"uint64\",\"name\":\"amount\",\"type\":\"uint64\"}],\"name\":\"requestRedemption\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"resetMovingFundsTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"fundingTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint32\",\"name\":\"fundingOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"bytes8\",\"name\":\"blindingFactor\",\"type\":\"bytes8\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes20\",\"name\":\"refundPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes4\",\"name\":\"refundLocktime\",\"type\":\"bytes4\"},{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"}],\"internalType\":\"structDeposit.DepositRevealInfo\",\"name\":\"reveal\",\"type\":\"tuple\"}],\"name\":\"revealDeposit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"fundingTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint32\",\"name\":\"fundingOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"bytes8\",\"name\":\"blindingFactor\",\"type\":\"bytes8\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes20\",\"name\":\"refundPubKeyHash\",\"type\":\"bytes20\"},{\"internalType\":\"bytes4\",\"name\":\"refundLocktime\",\"type\":\"bytes4\"},{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"}],\"internalType\":\"structDeposit.DepositRevealInfo\",\"name\":\"reveal\",\"type\":\"tuple\"},{\"internalType\":\"bytes32\",\"name\":\"extraData\",\"type\":\"bytes32\"}],\"name\":\"revealDepositWithExtraData\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"rebateStaking\",\"type\":\"address\"}],\"name\":\"setRebateStaking\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"redemptionWatchtower\",\"type\":\"address\"}],\"name\":\"setRedemptionWatchtower\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_reservationRouter\",\"type\":\"address\"}],\"name\":\"setReservationRouter\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spvMaintainer\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"isTrusted\",\"type\":\"bool\"}],\"name\":\"setSpvMaintainerStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"isTrusted\",\"type\":\"bool\"}],\"name\":\"setVaultStatus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"utxoKey\",\"type\":\"uint256\"}],\"name\":\"spentMainUTXOs\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"sweepTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"sweepProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"}],\"name\":\"submitDepositSweepProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"walletPublicKey\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"preimageSha256\",\"type\":\"bytes\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"r\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"s\",\"type\":\"bytes32\"},{\"internalType\":\"uint8\",\"name\":\"v\",\"type\":\"uint8\"}],\"internalType\":\"structBitcoinTx.RSVSignature\",\"name\":\"signature\",\"type\":\"tuple\"}],\"name\":\"submitFraudChallenge\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"sweepTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"sweepProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"}],\"name\":\"submitMovedFundsSweepProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"walletMainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"uint32[]\",\"name\":\"walletMembersIDs\",\"type\":\"uint32[]\"},{\"internalType\":\"uint256\",\"name\":\"walletMemberIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes20[]\",\"name\":\"targetWallets\",\"type\":\"bytes20[]\"}],\"name\":\"submitMovingFundsCommitment\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"movingFundsTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"movingFundsProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"submitMovingFundsProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes4\",\"name\":\"version\",\"type\":\"bytes4\"},{\"internalType\":\"bytes\",\"name\":\"inputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"outputVector\",\"type\":\"bytes\"},{\"internalType\":\"bytes4\",\"name\":\"locktime\",\"type\":\"bytes4\"}],\"internalType\":\"structBitcoinTx.Info\",\"name\":\"redemptionTx\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"merkleProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"txIndexInBlock\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"bitcoinHeaders\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"coinbasePreimage\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"coinbaseProof\",\"type\":\"bytes\"}],\"internalType\":\"structBitcoinTx.Proof\",\"name\":\"redemptionProof\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"txHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint32\",\"name\":\"txOutputIndex\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"txOutputValue\",\"type\":\"uint64\"}],\"internalType\":\"structBitcoinTx.UTXO\",\"name\":\"mainUtxo\",\"type\":\"tuple\"},{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"submitRedemptionProof\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"redemptionKey\",\"type\":\"uint256\"}],\"name\":\"timedOutRedemptions\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"redeemer\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"requestedAmount\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"treasuryFee\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"txMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"requestedAt\",\"type\":\"uint32\"}],\"internalType\":\"structRedemption.RedemptionRequest\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newGovernance\",\"type\":\"address\"}],\"name\":\"transferGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"treasury\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"txProofDifficultyFactor\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"depositDustThreshold\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"depositTreasuryFeeDivisor\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"depositTxMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"depositRevealAheadPeriod\",\"type\":\"uint32\"}],\"name\":\"updateDepositParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint96\",\"name\":\"fraudChallengeDepositAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"fraudChallengeDefeatTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"fraudSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"fraudNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"name\":\"updateFraudParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"movingFundsTxMaxTotalFee\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"movingFundsDustThreshold\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTimeoutResetDelay\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"movingFundsTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"},{\"internalType\":\"uint16\",\"name\":\"movingFundsCommitmentGasOffset\",\"type\":\"uint16\"},{\"internalType\":\"uint64\",\"name\":\"movedFundsSweepTxMaxTotalFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"movedFundsSweepTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"movedFundsSweepTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"movedFundsSweepTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"name\":\"updateMovingFundsParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"redemptionDustThreshold\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"redemptionTreasuryFeeDivisor\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"redemptionTxMaxFee\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"redemptionTxMaxTotalFee\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"redemptionTimeout\",\"type\":\"uint32\"},{\"internalType\":\"uint96\",\"name\":\"redemptionTimeoutSlashingAmount\",\"type\":\"uint96\"},{\"internalType\":\"uint32\",\"name\":\"redemptionTimeoutNotifierRewardMultiplier\",\"type\":\"uint32\"}],\"name\":\"updateRedemptionParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"treasury\",\"type\":\"address\"}],\"name\":\"updateTreasury\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"walletCreationPeriod\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"walletCreationMinBtcBalance\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"walletCreationMaxBtcBalance\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"walletClosureMinBtcBalance\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"walletMaxAge\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"walletMaxBtcTransfer\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"walletClosingPeriod\",\"type\":\"uint32\"}],\"name\":\"updateWalletParameters\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"walletParameters\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"walletCreationPeriod\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"walletCreationMinBtcBalance\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"walletCreationMaxBtcBalance\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"walletClosureMinBtcBalance\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"walletMaxAge\",\"type\":\"uint32\"},{\"internalType\":\"uint64\",\"name\":\"walletMaxBtcTransfer\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"walletClosingPeriod\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes20\",\"name\":\"walletPubKeyHash\",\"type\":\"bytes20\"}],\"name\":\"wallets\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"ecdsaWalletID\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"mainUtxoHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"pendingRedemptionsValue\",\"type\":\"uint64\"},{\"internalType\":\"uint32\",\"name\":\"createdAt\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"movingFundsRequestedAt\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"closingStartedAt\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"pendingMovedFundsSweepRequestsCount\",\"type\":\"uint32\"},{\"internalType\":\"enumWallets.WalletState\",\"name\":\"state\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"movingFundsTargetWalletsCommitmentHash\",\"type\":\"bytes32\"}],\"internalType\":\"structWallets.Wallet\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// BridgeABI is the input ABI used to generate the binding from.
// Deprecated: Use BridgeMetaData.ABI instead.
var BridgeABI = BridgeMetaData.ABI

// Bridge is an auto generated Go binding around an Ethereum contract.
type Bridge struct {
	BridgeCaller     // Read-only binding to the contract
	BridgeTransactor // Write-only binding to the contract
	BridgeFilterer   // Log filterer for contract events
}

// BridgeCaller is an auto generated read-only Go binding around an Ethereum contract.
type BridgeCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BridgeTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BridgeTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BridgeFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BridgeFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BridgeSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BridgeSession struct {
	Contract     *Bridge           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BridgeCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BridgeCallerSession struct {
	Contract *BridgeCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// BridgeTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BridgeTransactorSession struct {
	Contract     *BridgeTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BridgeRaw is an auto generated low-level Go binding around an Ethereum contract.
type BridgeRaw struct {
	Contract *Bridge // Generic contract binding to access the raw methods on
}

// BridgeCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BridgeCallerRaw struct {
	Contract *BridgeCaller // Generic read-only contract binding to access the raw methods on
}

// BridgeTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BridgeTransactorRaw struct {
	Contract *BridgeTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBridge creates a new instance of Bridge, bound to a specific deployed contract.
func NewBridge(address common.Address, backend bind.ContractBackend) (*Bridge, error) {
	contract, err := bindBridge(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Bridge{BridgeCaller: BridgeCaller{contract: contract}, BridgeTransactor: BridgeTransactor{contract: contract}, BridgeFilterer: BridgeFilterer{contract: contract}}, nil
}

// NewBridgeCaller creates a new read-only instance of Bridge, bound to a specific deployed contract.
func NewBridgeCaller(address common.Address, caller bind.ContractCaller) (*BridgeCaller, error) {
	contract, err := bindBridge(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BridgeCaller{contract: contract}, nil
}

// NewBridgeTransactor creates a new write-only instance of Bridge, bound to a specific deployed contract.
func NewBridgeTransactor(address common.Address, transactor bind.ContractTransactor) (*BridgeTransactor, error) {
	contract, err := bindBridge(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BridgeTransactor{contract: contract}, nil
}

// NewBridgeFilterer creates a new log filterer instance of Bridge, bound to a specific deployed contract.
func NewBridgeFilterer(address common.Address, filterer bind.ContractFilterer) (*BridgeFilterer, error) {
	contract, err := bindBridge(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BridgeFilterer{contract: contract}, nil
}

// bindBridge binds a generic wrapper to an already deployed contract.
func bindBridge(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BridgeMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Bridge *BridgeRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Bridge.Contract.BridgeCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Bridge *BridgeRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Bridge.Contract.BridgeTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Bridge *BridgeRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Bridge.Contract.BridgeTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Bridge *BridgeCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Bridge.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Bridge *BridgeTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Bridge.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Bridge *BridgeTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Bridge.Contract.contract.Transact(opts, method, params...)
}

// ActiveWalletPubKeyHash is a free data retrieval call binding the contract method 0xded1d24a.
//
// Solidity: function activeWalletPubKeyHash() view returns(bytes20)
func (_Bridge *BridgeCaller) ActiveWalletPubKeyHash(opts *bind.CallOpts) ([20]byte, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "activeWalletPubKeyHash")

	if err != nil {
		return *new([20]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([20]byte)).(*[20]byte)

	return out0, err

}

// ActiveWalletPubKeyHash is a free data retrieval call binding the contract method 0xded1d24a.
//
// Solidity: function activeWalletPubKeyHash() view returns(bytes20)
func (_Bridge *BridgeSession) ActiveWalletPubKeyHash() ([20]byte, error) {
	return _Bridge.Contract.ActiveWalletPubKeyHash(&_Bridge.CallOpts)
}

// ActiveWalletPubKeyHash is a free data retrieval call binding the contract method 0xded1d24a.
//
// Solidity: function activeWalletPubKeyHash() view returns(bytes20)
func (_Bridge *BridgeCallerSession) ActiveWalletPubKeyHash() ([20]byte, error) {
	return _Bridge.Contract.ActiveWalletPubKeyHash(&_Bridge.CallOpts)
}

// ContractReferences is a free data retrieval call binding the contract method 0xa9de2f3a.
//
// Solidity: function contractReferences() view returns(address bank, address relay, address ecdsaWalletRegistry, address reimbursementPool)
func (_Bridge *BridgeCaller) ContractReferences(opts *bind.CallOpts) (struct {
	Bank                common.Address
	Relay               common.Address
	EcdsaWalletRegistry common.Address
	ReimbursementPool   common.Address
}, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "contractReferences")

	outstruct := new(struct {
		Bank                common.Address
		Relay               common.Address
		EcdsaWalletRegistry common.Address
		ReimbursementPool   common.Address
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Bank = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Relay = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.EcdsaWalletRegistry = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)
	outstruct.ReimbursementPool = *abi.ConvertType(out[3], new(common.Address)).(*common.Address)

	return *outstruct, err

}

// ContractReferences is a free data retrieval call binding the contract method 0xa9de2f3a.
//
// Solidity: function contractReferences() view returns(address bank, address relay, address ecdsaWalletRegistry, address reimbursementPool)
func (_Bridge *BridgeSession) ContractReferences() (struct {
	Bank                common.Address
	Relay               common.Address
	EcdsaWalletRegistry common.Address
	ReimbursementPool   common.Address
}, error) {
	return _Bridge.Contract.ContractReferences(&_Bridge.CallOpts)
}

// ContractReferences is a free data retrieval call binding the contract method 0xa9de2f3a.
//
// Solidity: function contractReferences() view returns(address bank, address relay, address ecdsaWalletRegistry, address reimbursementPool)
func (_Bridge *BridgeCallerSession) ContractReferences() (struct {
	Bank                common.Address
	Relay               common.Address
	EcdsaWalletRegistry common.Address
	ReimbursementPool   common.Address
}, error) {
	return _Bridge.Contract.ContractReferences(&_Bridge.CallOpts)
}

// DepositParameters is a free data retrieval call binding the contract method 0xc42b64d0.
//
// Solidity: function depositParameters() view returns(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod)
func (_Bridge *BridgeCaller) DepositParameters(opts *bind.CallOpts) (struct {
	DepositDustThreshold      uint64
	DepositTreasuryFeeDivisor uint64
	DepositTxMaxFee           uint64
	DepositRevealAheadPeriod  uint32
}, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "depositParameters")

	outstruct := new(struct {
		DepositDustThreshold      uint64
		DepositTreasuryFeeDivisor uint64
		DepositTxMaxFee           uint64
		DepositRevealAheadPeriod  uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.DepositDustThreshold = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.DepositTreasuryFeeDivisor = *abi.ConvertType(out[1], new(uint64)).(*uint64)
	outstruct.DepositTxMaxFee = *abi.ConvertType(out[2], new(uint64)).(*uint64)
	outstruct.DepositRevealAheadPeriod = *abi.ConvertType(out[3], new(uint32)).(*uint32)

	return *outstruct, err

}

// DepositParameters is a free data retrieval call binding the contract method 0xc42b64d0.
//
// Solidity: function depositParameters() view returns(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod)
func (_Bridge *BridgeSession) DepositParameters() (struct {
	DepositDustThreshold      uint64
	DepositTreasuryFeeDivisor uint64
	DepositTxMaxFee           uint64
	DepositRevealAheadPeriod  uint32
}, error) {
	return _Bridge.Contract.DepositParameters(&_Bridge.CallOpts)
}

// DepositParameters is a free data retrieval call binding the contract method 0xc42b64d0.
//
// Solidity: function depositParameters() view returns(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod)
func (_Bridge *BridgeCallerSession) DepositParameters() (struct {
	DepositDustThreshold      uint64
	DepositTreasuryFeeDivisor uint64
	DepositTxMaxFee           uint64
	DepositRevealAheadPeriod  uint32
}, error) {
	return _Bridge.Contract.DepositParameters(&_Bridge.CallOpts)
}

// Deposits is a free data retrieval call binding the contract method 0xb02c43d0.
//
// Solidity: function deposits(uint256 depositKey) view returns((address,uint64,uint32,address,uint64,uint32,bytes32))
func (_Bridge *BridgeCaller) Deposits(opts *bind.CallOpts, depositKey *big.Int) (DepositDepositRequest, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "deposits", depositKey)

	if err != nil {
		return *new(DepositDepositRequest), err
	}

	out0 := *abi.ConvertType(out[0], new(DepositDepositRequest)).(*DepositDepositRequest)

	return out0, err

}

// Deposits is a free data retrieval call binding the contract method 0xb02c43d0.
//
// Solidity: function deposits(uint256 depositKey) view returns((address,uint64,uint32,address,uint64,uint32,bytes32))
func (_Bridge *BridgeSession) Deposits(depositKey *big.Int) (DepositDepositRequest, error) {
	return _Bridge.Contract.Deposits(&_Bridge.CallOpts, depositKey)
}

// Deposits is a free data retrieval call binding the contract method 0xb02c43d0.
//
// Solidity: function deposits(uint256 depositKey) view returns((address,uint64,uint32,address,uint64,uint32,bytes32))
func (_Bridge *BridgeCallerSession) Deposits(depositKey *big.Int) (DepositDepositRequest, error) {
	return _Bridge.Contract.Deposits(&_Bridge.CallOpts, depositKey)
}

// FraudChallenges is a free data retrieval call binding the contract method 0x33e957cb.
//
// Solidity: function fraudChallenges(uint256 challengeKey) view returns((address,uint256,uint32,bool))
func (_Bridge *BridgeCaller) FraudChallenges(opts *bind.CallOpts, challengeKey *big.Int) (FraudFraudChallenge, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "fraudChallenges", challengeKey)

	if err != nil {
		return *new(FraudFraudChallenge), err
	}

	out0 := *abi.ConvertType(out[0], new(FraudFraudChallenge)).(*FraudFraudChallenge)

	return out0, err

}

// FraudChallenges is a free data retrieval call binding the contract method 0x33e957cb.
//
// Solidity: function fraudChallenges(uint256 challengeKey) view returns((address,uint256,uint32,bool))
func (_Bridge *BridgeSession) FraudChallenges(challengeKey *big.Int) (FraudFraudChallenge, error) {
	return _Bridge.Contract.FraudChallenges(&_Bridge.CallOpts, challengeKey)
}

// FraudChallenges is a free data retrieval call binding the contract method 0x33e957cb.
//
// Solidity: function fraudChallenges(uint256 challengeKey) view returns((address,uint256,uint32,bool))
func (_Bridge *BridgeCallerSession) FraudChallenges(challengeKey *big.Int) (FraudFraudChallenge, error) {
	return _Bridge.Contract.FraudChallenges(&_Bridge.CallOpts, challengeKey)
}

// FraudParameters is a free data retrieval call binding the contract method 0x75b922d1.
//
// Solidity: function fraudParameters() view returns(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier)
func (_Bridge *BridgeCaller) FraudParameters(opts *bind.CallOpts) (struct {
	FraudChallengeDepositAmount   *big.Int
	FraudChallengeDefeatTimeout   uint32
	FraudSlashingAmount           *big.Int
	FraudNotifierRewardMultiplier uint32
}, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "fraudParameters")

	outstruct := new(struct {
		FraudChallengeDepositAmount   *big.Int
		FraudChallengeDefeatTimeout   uint32
		FraudSlashingAmount           *big.Int
		FraudNotifierRewardMultiplier uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.FraudChallengeDepositAmount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.FraudChallengeDefeatTimeout = *abi.ConvertType(out[1], new(uint32)).(*uint32)
	outstruct.FraudSlashingAmount = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.FraudNotifierRewardMultiplier = *abi.ConvertType(out[3], new(uint32)).(*uint32)

	return *outstruct, err

}

// FraudParameters is a free data retrieval call binding the contract method 0x75b922d1.
//
// Solidity: function fraudParameters() view returns(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier)
func (_Bridge *BridgeSession) FraudParameters() (struct {
	FraudChallengeDepositAmount   *big.Int
	FraudChallengeDefeatTimeout   uint32
	FraudSlashingAmount           *big.Int
	FraudNotifierRewardMultiplier uint32
}, error) {
	return _Bridge.Contract.FraudParameters(&_Bridge.CallOpts)
}

// FraudParameters is a free data retrieval call binding the contract method 0x75b922d1.
//
// Solidity: function fraudParameters() view returns(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier)
func (_Bridge *BridgeCallerSession) FraudParameters() (struct {
	FraudChallengeDepositAmount   *big.Int
	FraudChallengeDefeatTimeout   uint32
	FraudSlashingAmount           *big.Int
	FraudNotifierRewardMultiplier uint32
}, error) {
	return _Bridge.Contract.FraudParameters(&_Bridge.CallOpts)
}

// GetRebateStaking is a free data retrieval call binding the contract method 0x3edf8238.
//
// Solidity: function getRebateStaking() view returns(address)
func (_Bridge *BridgeCaller) GetRebateStaking(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "getRebateStaking")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetRebateStaking is a free data retrieval call binding the contract method 0x3edf8238.
//
// Solidity: function getRebateStaking() view returns(address)
func (_Bridge *BridgeSession) GetRebateStaking() (common.Address, error) {
	return _Bridge.Contract.GetRebateStaking(&_Bridge.CallOpts)
}

// GetRebateStaking is a free data retrieval call binding the contract method 0x3edf8238.
//
// Solidity: function getRebateStaking() view returns(address)
func (_Bridge *BridgeCallerSession) GetRebateStaking() (common.Address, error) {
	return _Bridge.Contract.GetRebateStaking(&_Bridge.CallOpts)
}

// GetRedemptionWatchtower is a free data retrieval call binding the contract method 0x5f3281ca.
//
// Solidity: function getRedemptionWatchtower() view returns(address)
func (_Bridge *BridgeCaller) GetRedemptionWatchtower(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "getRedemptionWatchtower")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetRedemptionWatchtower is a free data retrieval call binding the contract method 0x5f3281ca.
//
// Solidity: function getRedemptionWatchtower() view returns(address)
func (_Bridge *BridgeSession) GetRedemptionWatchtower() (common.Address, error) {
	return _Bridge.Contract.GetRedemptionWatchtower(&_Bridge.CallOpts)
}

// GetRedemptionWatchtower is a free data retrieval call binding the contract method 0x5f3281ca.
//
// Solidity: function getRedemptionWatchtower() view returns(address)
func (_Bridge *BridgeCallerSession) GetRedemptionWatchtower() (common.Address, error) {
	return _Bridge.Contract.GetRedemptionWatchtower(&_Bridge.CallOpts)
}

// GetReservationRouter is a free data retrieval call binding the contract method 0x5157ec0a.
//
// Solidity: function getReservationRouter() view returns(address)
func (_Bridge *BridgeCaller) GetReservationRouter(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "getReservationRouter")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetReservationRouter is a free data retrieval call binding the contract method 0x5157ec0a.
//
// Solidity: function getReservationRouter() view returns(address)
func (_Bridge *BridgeSession) GetReservationRouter() (common.Address, error) {
	return _Bridge.Contract.GetReservationRouter(&_Bridge.CallOpts)
}

// GetReservationRouter is a free data retrieval call binding the contract method 0x5157ec0a.
//
// Solidity: function getReservationRouter() view returns(address)
func (_Bridge *BridgeCallerSession) GetReservationRouter() (common.Address, error) {
	return _Bridge.Contract.GetReservationRouter(&_Bridge.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_Bridge *BridgeCaller) Governance(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "governance")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_Bridge *BridgeSession) Governance() (common.Address, error) {
	return _Bridge.Contract.Governance(&_Bridge.CallOpts)
}

// Governance is a free data retrieval call binding the contract method 0x5aa6e675.
//
// Solidity: function governance() view returns(address)
func (_Bridge *BridgeCallerSession) Governance() (common.Address, error) {
	return _Bridge.Contract.Governance(&_Bridge.CallOpts)
}

// IsReservedDeposit is a free data retrieval call binding the contract method 0x93df529b.
//
// Solidity: function isReservedDeposit(uint256 depositKey) view returns(bool)
func (_Bridge *BridgeCaller) IsReservedDeposit(opts *bind.CallOpts, depositKey *big.Int) (bool, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "isReservedDeposit", depositKey)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsReservedDeposit is a free data retrieval call binding the contract method 0x93df529b.
//
// Solidity: function isReservedDeposit(uint256 depositKey) view returns(bool)
func (_Bridge *BridgeSession) IsReservedDeposit(depositKey *big.Int) (bool, error) {
	return _Bridge.Contract.IsReservedDeposit(&_Bridge.CallOpts, depositKey)
}

// IsReservedDeposit is a free data retrieval call binding the contract method 0x93df529b.
//
// Solidity: function isReservedDeposit(uint256 depositKey) view returns(bool)
func (_Bridge *BridgeCallerSession) IsReservedDeposit(depositKey *big.Int) (bool, error) {
	return _Bridge.Contract.IsReservedDeposit(&_Bridge.CallOpts, depositKey)
}

// IsVaultTrusted is a free data retrieval call binding the contract method 0xe53c0b55.
//
// Solidity: function isVaultTrusted(address vault) view returns(bool)
func (_Bridge *BridgeCaller) IsVaultTrusted(opts *bind.CallOpts, vault common.Address) (bool, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "isVaultTrusted", vault)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsVaultTrusted is a free data retrieval call binding the contract method 0xe53c0b55.
//
// Solidity: function isVaultTrusted(address vault) view returns(bool)
func (_Bridge *BridgeSession) IsVaultTrusted(vault common.Address) (bool, error) {
	return _Bridge.Contract.IsVaultTrusted(&_Bridge.CallOpts, vault)
}

// IsVaultTrusted is a free data retrieval call binding the contract method 0xe53c0b55.
//
// Solidity: function isVaultTrusted(address vault) view returns(bool)
func (_Bridge *BridgeCallerSession) IsVaultTrusted(vault common.Address) (bool, error) {
	return _Bridge.Contract.IsVaultTrusted(&_Bridge.CallOpts, vault)
}

// LiveWalletsCount is a free data retrieval call binding the contract method 0x24028c11.
//
// Solidity: function liveWalletsCount() view returns(uint32)
func (_Bridge *BridgeCaller) LiveWalletsCount(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "liveWalletsCount")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// LiveWalletsCount is a free data retrieval call binding the contract method 0x24028c11.
//
// Solidity: function liveWalletsCount() view returns(uint32)
func (_Bridge *BridgeSession) LiveWalletsCount() (uint32, error) {
	return _Bridge.Contract.LiveWalletsCount(&_Bridge.CallOpts)
}

// LiveWalletsCount is a free data retrieval call binding the contract method 0x24028c11.
//
// Solidity: function liveWalletsCount() view returns(uint32)
func (_Bridge *BridgeCallerSession) LiveWalletsCount() (uint32, error) {
	return _Bridge.Contract.LiveWalletsCount(&_Bridge.CallOpts)
}

// MovedFundsSweepRequests is a free data retrieval call binding the contract method 0xf18cf1b1.
//
// Solidity: function movedFundsSweepRequests(uint256 requestKey) view returns((bytes20,uint64,uint32,uint8))
func (_Bridge *BridgeCaller) MovedFundsSweepRequests(opts *bind.CallOpts, requestKey *big.Int) (MovingFundsMovedFundsSweepRequest, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "movedFundsSweepRequests", requestKey)

	if err != nil {
		return *new(MovingFundsMovedFundsSweepRequest), err
	}

	out0 := *abi.ConvertType(out[0], new(MovingFundsMovedFundsSweepRequest)).(*MovingFundsMovedFundsSweepRequest)

	return out0, err

}

// MovedFundsSweepRequests is a free data retrieval call binding the contract method 0xf18cf1b1.
//
// Solidity: function movedFundsSweepRequests(uint256 requestKey) view returns((bytes20,uint64,uint32,uint8))
func (_Bridge *BridgeSession) MovedFundsSweepRequests(requestKey *big.Int) (MovingFundsMovedFundsSweepRequest, error) {
	return _Bridge.Contract.MovedFundsSweepRequests(&_Bridge.CallOpts, requestKey)
}

// MovedFundsSweepRequests is a free data retrieval call binding the contract method 0xf18cf1b1.
//
// Solidity: function movedFundsSweepRequests(uint256 requestKey) view returns((bytes20,uint64,uint32,uint8))
func (_Bridge *BridgeCallerSession) MovedFundsSweepRequests(requestKey *big.Int) (MovingFundsMovedFundsSweepRequest, error) {
	return _Bridge.Contract.MovedFundsSweepRequests(&_Bridge.CallOpts, requestKey)
}

// MovingFundsParameters is a free data retrieval call binding the contract method 0xbe05abe3.
//
// Solidity: function movingFundsParameters() view returns(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeCaller) MovingFundsParameters(opts *bind.CallOpts) (struct {
	MovingFundsTxMaxTotalFee                       uint64
	MovingFundsDustThreshold                       uint64
	MovingFundsTimeoutResetDelay                   uint32
	MovingFundsTimeout                             uint32
	MovingFundsTimeoutSlashingAmount               *big.Int
	MovingFundsTimeoutNotifierRewardMultiplier     uint32
	MovingFundsCommitmentGasOffset                 uint16
	MovedFundsSweepTxMaxTotalFee                   uint64
	MovedFundsSweepTimeout                         uint32
	MovedFundsSweepTimeoutSlashingAmount           *big.Int
	MovedFundsSweepTimeoutNotifierRewardMultiplier uint32
}, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "movingFundsParameters")

	outstruct := new(struct {
		MovingFundsTxMaxTotalFee                       uint64
		MovingFundsDustThreshold                       uint64
		MovingFundsTimeoutResetDelay                   uint32
		MovingFundsTimeout                             uint32
		MovingFundsTimeoutSlashingAmount               *big.Int
		MovingFundsTimeoutNotifierRewardMultiplier     uint32
		MovingFundsCommitmentGasOffset                 uint16
		MovedFundsSweepTxMaxTotalFee                   uint64
		MovedFundsSweepTimeout                         uint32
		MovedFundsSweepTimeoutSlashingAmount           *big.Int
		MovedFundsSweepTimeoutNotifierRewardMultiplier uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.MovingFundsTxMaxTotalFee = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.MovingFundsDustThreshold = *abi.ConvertType(out[1], new(uint64)).(*uint64)
	outstruct.MovingFundsTimeoutResetDelay = *abi.ConvertType(out[2], new(uint32)).(*uint32)
	outstruct.MovingFundsTimeout = *abi.ConvertType(out[3], new(uint32)).(*uint32)
	outstruct.MovingFundsTimeoutSlashingAmount = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.MovingFundsTimeoutNotifierRewardMultiplier = *abi.ConvertType(out[5], new(uint32)).(*uint32)
	outstruct.MovingFundsCommitmentGasOffset = *abi.ConvertType(out[6], new(uint16)).(*uint16)
	outstruct.MovedFundsSweepTxMaxTotalFee = *abi.ConvertType(out[7], new(uint64)).(*uint64)
	outstruct.MovedFundsSweepTimeout = *abi.ConvertType(out[8], new(uint32)).(*uint32)
	outstruct.MovedFundsSweepTimeoutSlashingAmount = *abi.ConvertType(out[9], new(*big.Int)).(**big.Int)
	outstruct.MovedFundsSweepTimeoutNotifierRewardMultiplier = *abi.ConvertType(out[10], new(uint32)).(*uint32)

	return *outstruct, err

}

// MovingFundsParameters is a free data retrieval call binding the contract method 0xbe05abe3.
//
// Solidity: function movingFundsParameters() view returns(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeSession) MovingFundsParameters() (struct {
	MovingFundsTxMaxTotalFee                       uint64
	MovingFundsDustThreshold                       uint64
	MovingFundsTimeoutResetDelay                   uint32
	MovingFundsTimeout                             uint32
	MovingFundsTimeoutSlashingAmount               *big.Int
	MovingFundsTimeoutNotifierRewardMultiplier     uint32
	MovingFundsCommitmentGasOffset                 uint16
	MovedFundsSweepTxMaxTotalFee                   uint64
	MovedFundsSweepTimeout                         uint32
	MovedFundsSweepTimeoutSlashingAmount           *big.Int
	MovedFundsSweepTimeoutNotifierRewardMultiplier uint32
}, error) {
	return _Bridge.Contract.MovingFundsParameters(&_Bridge.CallOpts)
}

// MovingFundsParameters is a free data retrieval call binding the contract method 0xbe05abe3.
//
// Solidity: function movingFundsParameters() view returns(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeCallerSession) MovingFundsParameters() (struct {
	MovingFundsTxMaxTotalFee                       uint64
	MovingFundsDustThreshold                       uint64
	MovingFundsTimeoutResetDelay                   uint32
	MovingFundsTimeout                             uint32
	MovingFundsTimeoutSlashingAmount               *big.Int
	MovingFundsTimeoutNotifierRewardMultiplier     uint32
	MovingFundsCommitmentGasOffset                 uint16
	MovedFundsSweepTxMaxTotalFee                   uint64
	MovedFundsSweepTimeout                         uint32
	MovedFundsSweepTimeoutSlashingAmount           *big.Int
	MovedFundsSweepTimeoutNotifierRewardMultiplier uint32
}, error) {
	return _Bridge.Contract.MovingFundsParameters(&_Bridge.CallOpts)
}

// PendingRedemptions is a free data retrieval call binding the contract method 0x03d952f7.
//
// Solidity: function pendingRedemptions(uint256 redemptionKey) view returns((address,uint64,uint64,uint64,uint32))
func (_Bridge *BridgeCaller) PendingRedemptions(opts *bind.CallOpts, redemptionKey *big.Int) (RedemptionRedemptionRequest, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "pendingRedemptions", redemptionKey)

	if err != nil {
		return *new(RedemptionRedemptionRequest), err
	}

	out0 := *abi.ConvertType(out[0], new(RedemptionRedemptionRequest)).(*RedemptionRedemptionRequest)

	return out0, err

}

// PendingRedemptions is a free data retrieval call binding the contract method 0x03d952f7.
//
// Solidity: function pendingRedemptions(uint256 redemptionKey) view returns((address,uint64,uint64,uint64,uint32))
func (_Bridge *BridgeSession) PendingRedemptions(redemptionKey *big.Int) (RedemptionRedemptionRequest, error) {
	return _Bridge.Contract.PendingRedemptions(&_Bridge.CallOpts, redemptionKey)
}

// PendingRedemptions is a free data retrieval call binding the contract method 0x03d952f7.
//
// Solidity: function pendingRedemptions(uint256 redemptionKey) view returns((address,uint64,uint64,uint64,uint32))
func (_Bridge *BridgeCallerSession) PendingRedemptions(redemptionKey *big.Int) (RedemptionRedemptionRequest, error) {
	return _Bridge.Contract.PendingRedemptions(&_Bridge.CallOpts, redemptionKey)
}

// RedemptionParameters is a free data retrieval call binding the contract method 0x6e70ce41.
//
// Solidity: function redemptionParameters() view returns(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeCaller) RedemptionParameters(opts *bind.CallOpts) (struct {
	RedemptionDustThreshold                   uint64
	RedemptionTreasuryFeeDivisor              uint64
	RedemptionTxMaxFee                        uint64
	RedemptionTxMaxTotalFee                   uint64
	RedemptionTimeout                         uint32
	RedemptionTimeoutSlashingAmount           *big.Int
	RedemptionTimeoutNotifierRewardMultiplier uint32
}, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "redemptionParameters")

	outstruct := new(struct {
		RedemptionDustThreshold                   uint64
		RedemptionTreasuryFeeDivisor              uint64
		RedemptionTxMaxFee                        uint64
		RedemptionTxMaxTotalFee                   uint64
		RedemptionTimeout                         uint32
		RedemptionTimeoutSlashingAmount           *big.Int
		RedemptionTimeoutNotifierRewardMultiplier uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.RedemptionDustThreshold = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.RedemptionTreasuryFeeDivisor = *abi.ConvertType(out[1], new(uint64)).(*uint64)
	outstruct.RedemptionTxMaxFee = *abi.ConvertType(out[2], new(uint64)).(*uint64)
	outstruct.RedemptionTxMaxTotalFee = *abi.ConvertType(out[3], new(uint64)).(*uint64)
	outstruct.RedemptionTimeout = *abi.ConvertType(out[4], new(uint32)).(*uint32)
	outstruct.RedemptionTimeoutSlashingAmount = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.RedemptionTimeoutNotifierRewardMultiplier = *abi.ConvertType(out[6], new(uint32)).(*uint32)

	return *outstruct, err

}

// RedemptionParameters is a free data retrieval call binding the contract method 0x6e70ce41.
//
// Solidity: function redemptionParameters() view returns(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeSession) RedemptionParameters() (struct {
	RedemptionDustThreshold                   uint64
	RedemptionTreasuryFeeDivisor              uint64
	RedemptionTxMaxFee                        uint64
	RedemptionTxMaxTotalFee                   uint64
	RedemptionTimeout                         uint32
	RedemptionTimeoutSlashingAmount           *big.Int
	RedemptionTimeoutNotifierRewardMultiplier uint32
}, error) {
	return _Bridge.Contract.RedemptionParameters(&_Bridge.CallOpts)
}

// RedemptionParameters is a free data retrieval call binding the contract method 0x6e70ce41.
//
// Solidity: function redemptionParameters() view returns(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeCallerSession) RedemptionParameters() (struct {
	RedemptionDustThreshold                   uint64
	RedemptionTreasuryFeeDivisor              uint64
	RedemptionTxMaxFee                        uint64
	RedemptionTxMaxTotalFee                   uint64
	RedemptionTimeout                         uint32
	RedemptionTimeoutSlashingAmount           *big.Int
	RedemptionTimeoutNotifierRewardMultiplier uint32
}, error) {
	return _Bridge.Contract.RedemptionParameters(&_Bridge.CallOpts)
}

// SpentMainUTXOs is a free data retrieval call binding the contract method 0xb2146cd6.
//
// Solidity: function spentMainUTXOs(uint256 utxoKey) view returns(bool)
func (_Bridge *BridgeCaller) SpentMainUTXOs(opts *bind.CallOpts, utxoKey *big.Int) (bool, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "spentMainUTXOs", utxoKey)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SpentMainUTXOs is a free data retrieval call binding the contract method 0xb2146cd6.
//
// Solidity: function spentMainUTXOs(uint256 utxoKey) view returns(bool)
func (_Bridge *BridgeSession) SpentMainUTXOs(utxoKey *big.Int) (bool, error) {
	return _Bridge.Contract.SpentMainUTXOs(&_Bridge.CallOpts, utxoKey)
}

// SpentMainUTXOs is a free data retrieval call binding the contract method 0xb2146cd6.
//
// Solidity: function spentMainUTXOs(uint256 utxoKey) view returns(bool)
func (_Bridge *BridgeCallerSession) SpentMainUTXOs(utxoKey *big.Int) (bool, error) {
	return _Bridge.Contract.SpentMainUTXOs(&_Bridge.CallOpts, utxoKey)
}

// TimedOutRedemptions is a free data retrieval call binding the contract method 0x0b6ba19d.
//
// Solidity: function timedOutRedemptions(uint256 redemptionKey) view returns((address,uint64,uint64,uint64,uint32))
func (_Bridge *BridgeCaller) TimedOutRedemptions(opts *bind.CallOpts, redemptionKey *big.Int) (RedemptionRedemptionRequest, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "timedOutRedemptions", redemptionKey)

	if err != nil {
		return *new(RedemptionRedemptionRequest), err
	}

	out0 := *abi.ConvertType(out[0], new(RedemptionRedemptionRequest)).(*RedemptionRedemptionRequest)

	return out0, err

}

// TimedOutRedemptions is a free data retrieval call binding the contract method 0x0b6ba19d.
//
// Solidity: function timedOutRedemptions(uint256 redemptionKey) view returns((address,uint64,uint64,uint64,uint32))
func (_Bridge *BridgeSession) TimedOutRedemptions(redemptionKey *big.Int) (RedemptionRedemptionRequest, error) {
	return _Bridge.Contract.TimedOutRedemptions(&_Bridge.CallOpts, redemptionKey)
}

// TimedOutRedemptions is a free data retrieval call binding the contract method 0x0b6ba19d.
//
// Solidity: function timedOutRedemptions(uint256 redemptionKey) view returns((address,uint64,uint64,uint64,uint32))
func (_Bridge *BridgeCallerSession) TimedOutRedemptions(redemptionKey *big.Int) (RedemptionRedemptionRequest, error) {
	return _Bridge.Contract.TimedOutRedemptions(&_Bridge.CallOpts, redemptionKey)
}

// Treasury is a free data retrieval call binding the contract method 0x61d027b3.
//
// Solidity: function treasury() view returns(address)
func (_Bridge *BridgeCaller) Treasury(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "treasury")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Treasury is a free data retrieval call binding the contract method 0x61d027b3.
//
// Solidity: function treasury() view returns(address)
func (_Bridge *BridgeSession) Treasury() (common.Address, error) {
	return _Bridge.Contract.Treasury(&_Bridge.CallOpts)
}

// Treasury is a free data retrieval call binding the contract method 0x61d027b3.
//
// Solidity: function treasury() view returns(address)
func (_Bridge *BridgeCallerSession) Treasury() (common.Address, error) {
	return _Bridge.Contract.Treasury(&_Bridge.CallOpts)
}

// TxProofDifficultyFactor is a free data retrieval call binding the contract method 0x2bb818c2.
//
// Solidity: function txProofDifficultyFactor() view returns(uint256)
func (_Bridge *BridgeCaller) TxProofDifficultyFactor(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "txProofDifficultyFactor")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TxProofDifficultyFactor is a free data retrieval call binding the contract method 0x2bb818c2.
//
// Solidity: function txProofDifficultyFactor() view returns(uint256)
func (_Bridge *BridgeSession) TxProofDifficultyFactor() (*big.Int, error) {
	return _Bridge.Contract.TxProofDifficultyFactor(&_Bridge.CallOpts)
}

// TxProofDifficultyFactor is a free data retrieval call binding the contract method 0x2bb818c2.
//
// Solidity: function txProofDifficultyFactor() view returns(uint256)
func (_Bridge *BridgeCallerSession) TxProofDifficultyFactor() (*big.Int, error) {
	return _Bridge.Contract.TxProofDifficultyFactor(&_Bridge.CallOpts)
}

// WalletParameters is a free data retrieval call binding the contract method 0x61ccf97a.
//
// Solidity: function walletParameters() view returns(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod)
func (_Bridge *BridgeCaller) WalletParameters(opts *bind.CallOpts) (struct {
	WalletCreationPeriod        uint32
	WalletCreationMinBtcBalance uint64
	WalletCreationMaxBtcBalance uint64
	WalletClosureMinBtcBalance  uint64
	WalletMaxAge                uint32
	WalletMaxBtcTransfer        uint64
	WalletClosingPeriod         uint32
}, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "walletParameters")

	outstruct := new(struct {
		WalletCreationPeriod        uint32
		WalletCreationMinBtcBalance uint64
		WalletCreationMaxBtcBalance uint64
		WalletClosureMinBtcBalance  uint64
		WalletMaxAge                uint32
		WalletMaxBtcTransfer        uint64
		WalletClosingPeriod         uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.WalletCreationPeriod = *abi.ConvertType(out[0], new(uint32)).(*uint32)
	outstruct.WalletCreationMinBtcBalance = *abi.ConvertType(out[1], new(uint64)).(*uint64)
	outstruct.WalletCreationMaxBtcBalance = *abi.ConvertType(out[2], new(uint64)).(*uint64)
	outstruct.WalletClosureMinBtcBalance = *abi.ConvertType(out[3], new(uint64)).(*uint64)
	outstruct.WalletMaxAge = *abi.ConvertType(out[4], new(uint32)).(*uint32)
	outstruct.WalletMaxBtcTransfer = *abi.ConvertType(out[5], new(uint64)).(*uint64)
	outstruct.WalletClosingPeriod = *abi.ConvertType(out[6], new(uint32)).(*uint32)

	return *outstruct, err

}

// WalletParameters is a free data retrieval call binding the contract method 0x61ccf97a.
//
// Solidity: function walletParameters() view returns(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod)
func (_Bridge *BridgeSession) WalletParameters() (struct {
	WalletCreationPeriod        uint32
	WalletCreationMinBtcBalance uint64
	WalletCreationMaxBtcBalance uint64
	WalletClosureMinBtcBalance  uint64
	WalletMaxAge                uint32
	WalletMaxBtcTransfer        uint64
	WalletClosingPeriod         uint32
}, error) {
	return _Bridge.Contract.WalletParameters(&_Bridge.CallOpts)
}

// WalletParameters is a free data retrieval call binding the contract method 0x61ccf97a.
//
// Solidity: function walletParameters() view returns(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod)
func (_Bridge *BridgeCallerSession) WalletParameters() (struct {
	WalletCreationPeriod        uint32
	WalletCreationMinBtcBalance uint64
	WalletCreationMaxBtcBalance uint64
	WalletClosureMinBtcBalance  uint64
	WalletMaxAge                uint32
	WalletMaxBtcTransfer        uint64
	WalletClosingPeriod         uint32
}, error) {
	return _Bridge.Contract.WalletParameters(&_Bridge.CallOpts)
}

// Wallets is a free data retrieval call binding the contract method 0xe65e19d5.
//
// Solidity: function wallets(bytes20 walletPubKeyHash) view returns((bytes32,bytes32,uint64,uint32,uint32,uint32,uint32,uint8,bytes32))
func (_Bridge *BridgeCaller) Wallets(opts *bind.CallOpts, walletPubKeyHash [20]byte) (WalletsWallet, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "wallets", walletPubKeyHash)

	if err != nil {
		return *new(WalletsWallet), err
	}

	out0 := *abi.ConvertType(out[0], new(WalletsWallet)).(*WalletsWallet)

	return out0, err

}

// Wallets is a free data retrieval call binding the contract method 0xe65e19d5.
//
// Solidity: function wallets(bytes20 walletPubKeyHash) view returns((bytes32,bytes32,uint64,uint32,uint32,uint32,uint32,uint8,bytes32))
func (_Bridge *BridgeSession) Wallets(walletPubKeyHash [20]byte) (WalletsWallet, error) {
	return _Bridge.Contract.Wallets(&_Bridge.CallOpts, walletPubKeyHash)
}

// Wallets is a free data retrieval call binding the contract method 0xe65e19d5.
//
// Solidity: function wallets(bytes20 walletPubKeyHash) view returns((bytes32,bytes32,uint64,uint32,uint32,uint32,uint32,uint8,bytes32))
func (_Bridge *BridgeCallerSession) Wallets(walletPubKeyHash [20]byte) (WalletsWallet, error) {
	return _Bridge.Contract.Wallets(&_Bridge.CallOpts, walletPubKeyHash)
}

// EcdsaWalletCreatedCallback is a paid mutator transaction binding the contract method 0xa8fa0f42.
//
// Solidity: function __ecdsaWalletCreatedCallback(bytes32 ecdsaWalletID, bytes32 publicKeyX, bytes32 publicKeyY) returns()
func (_Bridge *BridgeTransactor) EcdsaWalletCreatedCallback(opts *bind.TransactOpts, ecdsaWalletID [32]byte, publicKeyX [32]byte, publicKeyY [32]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "__ecdsaWalletCreatedCallback", ecdsaWalletID, publicKeyX, publicKeyY)
}

// EcdsaWalletCreatedCallback is a paid mutator transaction binding the contract method 0xa8fa0f42.
//
// Solidity: function __ecdsaWalletCreatedCallback(bytes32 ecdsaWalletID, bytes32 publicKeyX, bytes32 publicKeyY) returns()
func (_Bridge *BridgeSession) EcdsaWalletCreatedCallback(ecdsaWalletID [32]byte, publicKeyX [32]byte, publicKeyY [32]byte) (*types.Transaction, error) {
	return _Bridge.Contract.EcdsaWalletCreatedCallback(&_Bridge.TransactOpts, ecdsaWalletID, publicKeyX, publicKeyY)
}

// EcdsaWalletCreatedCallback is a paid mutator transaction binding the contract method 0xa8fa0f42.
//
// Solidity: function __ecdsaWalletCreatedCallback(bytes32 ecdsaWalletID, bytes32 publicKeyX, bytes32 publicKeyY) returns()
func (_Bridge *BridgeTransactorSession) EcdsaWalletCreatedCallback(ecdsaWalletID [32]byte, publicKeyX [32]byte, publicKeyY [32]byte) (*types.Transaction, error) {
	return _Bridge.Contract.EcdsaWalletCreatedCallback(&_Bridge.TransactOpts, ecdsaWalletID, publicKeyX, publicKeyY)
}

// EcdsaWalletHeartbeatFailedCallback is a paid mutator transaction binding the contract method 0x3dce9812.
//
// Solidity: function __ecdsaWalletHeartbeatFailedCallback(bytes32 , bytes32 publicKeyX, bytes32 publicKeyY) returns()
func (_Bridge *BridgeTransactor) EcdsaWalletHeartbeatFailedCallback(opts *bind.TransactOpts, arg0 [32]byte, publicKeyX [32]byte, publicKeyY [32]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "__ecdsaWalletHeartbeatFailedCallback", arg0, publicKeyX, publicKeyY)
}

// EcdsaWalletHeartbeatFailedCallback is a paid mutator transaction binding the contract method 0x3dce9812.
//
// Solidity: function __ecdsaWalletHeartbeatFailedCallback(bytes32 , bytes32 publicKeyX, bytes32 publicKeyY) returns()
func (_Bridge *BridgeSession) EcdsaWalletHeartbeatFailedCallback(arg0 [32]byte, publicKeyX [32]byte, publicKeyY [32]byte) (*types.Transaction, error) {
	return _Bridge.Contract.EcdsaWalletHeartbeatFailedCallback(&_Bridge.TransactOpts, arg0, publicKeyX, publicKeyY)
}

// EcdsaWalletHeartbeatFailedCallback is a paid mutator transaction binding the contract method 0x3dce9812.
//
// Solidity: function __ecdsaWalletHeartbeatFailedCallback(bytes32 , bytes32 publicKeyX, bytes32 publicKeyY) returns()
func (_Bridge *BridgeTransactorSession) EcdsaWalletHeartbeatFailedCallback(arg0 [32]byte, publicKeyX [32]byte, publicKeyY [32]byte) (*types.Transaction, error) {
	return _Bridge.Contract.EcdsaWalletHeartbeatFailedCallback(&_Bridge.TransactOpts, arg0, publicKeyX, publicKeyY)
}

// DefeatFraudChallenge is a paid mutator transaction binding the contract method 0x77145f21.
//
// Solidity: function defeatFraudChallenge(bytes walletPublicKey, bytes preimage, bool witness) returns()
func (_Bridge *BridgeTransactor) DefeatFraudChallenge(opts *bind.TransactOpts, walletPublicKey []byte, preimage []byte, witness bool) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "defeatFraudChallenge", walletPublicKey, preimage, witness)
}

// DefeatFraudChallenge is a paid mutator transaction binding the contract method 0x77145f21.
//
// Solidity: function defeatFraudChallenge(bytes walletPublicKey, bytes preimage, bool witness) returns()
func (_Bridge *BridgeSession) DefeatFraudChallenge(walletPublicKey []byte, preimage []byte, witness bool) (*types.Transaction, error) {
	return _Bridge.Contract.DefeatFraudChallenge(&_Bridge.TransactOpts, walletPublicKey, preimage, witness)
}

// DefeatFraudChallenge is a paid mutator transaction binding the contract method 0x77145f21.
//
// Solidity: function defeatFraudChallenge(bytes walletPublicKey, bytes preimage, bool witness) returns()
func (_Bridge *BridgeTransactorSession) DefeatFraudChallenge(walletPublicKey []byte, preimage []byte, witness bool) (*types.Transaction, error) {
	return _Bridge.Contract.DefeatFraudChallenge(&_Bridge.TransactOpts, walletPublicKey, preimage, witness)
}

// DefeatFraudChallengeWithHeartbeat is a paid mutator transaction binding the contract method 0x0674f266.
//
// Solidity: function defeatFraudChallengeWithHeartbeat(bytes walletPublicKey, bytes heartbeatMessage) returns()
func (_Bridge *BridgeTransactor) DefeatFraudChallengeWithHeartbeat(opts *bind.TransactOpts, walletPublicKey []byte, heartbeatMessage []byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "defeatFraudChallengeWithHeartbeat", walletPublicKey, heartbeatMessage)
}

// DefeatFraudChallengeWithHeartbeat is a paid mutator transaction binding the contract method 0x0674f266.
//
// Solidity: function defeatFraudChallengeWithHeartbeat(bytes walletPublicKey, bytes heartbeatMessage) returns()
func (_Bridge *BridgeSession) DefeatFraudChallengeWithHeartbeat(walletPublicKey []byte, heartbeatMessage []byte) (*types.Transaction, error) {
	return _Bridge.Contract.DefeatFraudChallengeWithHeartbeat(&_Bridge.TransactOpts, walletPublicKey, heartbeatMessage)
}

// DefeatFraudChallengeWithHeartbeat is a paid mutator transaction binding the contract method 0x0674f266.
//
// Solidity: function defeatFraudChallengeWithHeartbeat(bytes walletPublicKey, bytes heartbeatMessage) returns()
func (_Bridge *BridgeTransactorSession) DefeatFraudChallengeWithHeartbeat(walletPublicKey []byte, heartbeatMessage []byte) (*types.Transaction, error) {
	return _Bridge.Contract.DefeatFraudChallengeWithHeartbeat(&_Bridge.TransactOpts, walletPublicKey, heartbeatMessage)
}

// Initialize is a paid mutator transaction binding the contract method 0xd246ce16.
//
// Solidity: function initialize(address _bank, address _relay, address _treasury, address _ecdsaWalletRegistry, address _reimbursementPool, uint96 _txProofDifficultyFactor) returns()
func (_Bridge *BridgeTransactor) Initialize(opts *bind.TransactOpts, _bank common.Address, _relay common.Address, _treasury common.Address, _ecdsaWalletRegistry common.Address, _reimbursementPool common.Address, _txProofDifficultyFactor *big.Int) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "initialize", _bank, _relay, _treasury, _ecdsaWalletRegistry, _reimbursementPool, _txProofDifficultyFactor)
}

// Initialize is a paid mutator transaction binding the contract method 0xd246ce16.
//
// Solidity: function initialize(address _bank, address _relay, address _treasury, address _ecdsaWalletRegistry, address _reimbursementPool, uint96 _txProofDifficultyFactor) returns()
func (_Bridge *BridgeSession) Initialize(_bank common.Address, _relay common.Address, _treasury common.Address, _ecdsaWalletRegistry common.Address, _reimbursementPool common.Address, _txProofDifficultyFactor *big.Int) (*types.Transaction, error) {
	return _Bridge.Contract.Initialize(&_Bridge.TransactOpts, _bank, _relay, _treasury, _ecdsaWalletRegistry, _reimbursementPool, _txProofDifficultyFactor)
}

// Initialize is a paid mutator transaction binding the contract method 0xd246ce16.
//
// Solidity: function initialize(address _bank, address _relay, address _treasury, address _ecdsaWalletRegistry, address _reimbursementPool, uint96 _txProofDifficultyFactor) returns()
func (_Bridge *BridgeTransactorSession) Initialize(_bank common.Address, _relay common.Address, _treasury common.Address, _ecdsaWalletRegistry common.Address, _reimbursementPool common.Address, _txProofDifficultyFactor *big.Int) (*types.Transaction, error) {
	return _Bridge.Contract.Initialize(&_Bridge.TransactOpts, _bank, _relay, _treasury, _ecdsaWalletRegistry, _reimbursementPool, _txProofDifficultyFactor)
}

// InitializeV2FixVaultZeroDeposit is a paid mutator transaction binding the contract method 0x456ffee0.
//
// Solidity: function initializeV2_FixVaultZeroDeposit() returns()
func (_Bridge *BridgeTransactor) InitializeV2FixVaultZeroDeposit(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "initializeV2_FixVaultZeroDeposit")
}

// InitializeV2FixVaultZeroDeposit is a paid mutator transaction binding the contract method 0x456ffee0.
//
// Solidity: function initializeV2_FixVaultZeroDeposit() returns()
func (_Bridge *BridgeSession) InitializeV2FixVaultZeroDeposit() (*types.Transaction, error) {
	return _Bridge.Contract.InitializeV2FixVaultZeroDeposit(&_Bridge.TransactOpts)
}

// InitializeV2FixVaultZeroDeposit is a paid mutator transaction binding the contract method 0x456ffee0.
//
// Solidity: function initializeV2_FixVaultZeroDeposit() returns()
func (_Bridge *BridgeTransactorSession) InitializeV2FixVaultZeroDeposit() (*types.Transaction, error) {
	return _Bridge.Contract.InitializeV2FixVaultZeroDeposit(&_Bridge.TransactOpts)
}

// InitializeV5RepairRebateStaking is a paid mutator transaction binding the contract method 0x1ebf670d.
//
// Solidity: function initializeV5_RepairRebateStaking(address newRebateStaking) returns()
func (_Bridge *BridgeTransactor) InitializeV5RepairRebateStaking(opts *bind.TransactOpts, newRebateStaking common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "initializeV5_RepairRebateStaking", newRebateStaking)
}

// InitializeV5RepairRebateStaking is a paid mutator transaction binding the contract method 0x1ebf670d.
//
// Solidity: function initializeV5_RepairRebateStaking(address newRebateStaking) returns()
func (_Bridge *BridgeSession) InitializeV5RepairRebateStaking(newRebateStaking common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.InitializeV5RepairRebateStaking(&_Bridge.TransactOpts, newRebateStaking)
}

// InitializeV5RepairRebateStaking is a paid mutator transaction binding the contract method 0x1ebf670d.
//
// Solidity: function initializeV5_RepairRebateStaking(address newRebateStaking) returns()
func (_Bridge *BridgeTransactorSession) InitializeV5RepairRebateStaking(newRebateStaking common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.InitializeV5RepairRebateStaking(&_Bridge.TransactOpts, newRebateStaking)
}

// NotifyFraudChallengeDefeatTimeout is a paid mutator transaction binding the contract method 0x79fc4eb3.
//
// Solidity: function notifyFraudChallengeDefeatTimeout(bytes walletPublicKey, uint32[] walletMembersIDs, bytes preimageSha256) returns()
func (_Bridge *BridgeTransactor) NotifyFraudChallengeDefeatTimeout(opts *bind.TransactOpts, walletPublicKey []byte, walletMembersIDs []uint32, preimageSha256 []byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyFraudChallengeDefeatTimeout", walletPublicKey, walletMembersIDs, preimageSha256)
}

// NotifyFraudChallengeDefeatTimeout is a paid mutator transaction binding the contract method 0x79fc4eb3.
//
// Solidity: function notifyFraudChallengeDefeatTimeout(bytes walletPublicKey, uint32[] walletMembersIDs, bytes preimageSha256) returns()
func (_Bridge *BridgeSession) NotifyFraudChallengeDefeatTimeout(walletPublicKey []byte, walletMembersIDs []uint32, preimageSha256 []byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyFraudChallengeDefeatTimeout(&_Bridge.TransactOpts, walletPublicKey, walletMembersIDs, preimageSha256)
}

// NotifyFraudChallengeDefeatTimeout is a paid mutator transaction binding the contract method 0x79fc4eb3.
//
// Solidity: function notifyFraudChallengeDefeatTimeout(bytes walletPublicKey, uint32[] walletMembersIDs, bytes preimageSha256) returns()
func (_Bridge *BridgeTransactorSession) NotifyFraudChallengeDefeatTimeout(walletPublicKey []byte, walletMembersIDs []uint32, preimageSha256 []byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyFraudChallengeDefeatTimeout(&_Bridge.TransactOpts, walletPublicKey, walletMembersIDs, preimageSha256)
}

// NotifyMovedFundsSweepTimeout is a paid mutator transaction binding the contract method 0x50aea15a.
//
// Solidity: function notifyMovedFundsSweepTimeout(bytes32 movingFundsTxHash, uint32 movingFundsTxOutputIndex, uint32[] walletMembersIDs) returns()
func (_Bridge *BridgeTransactor) NotifyMovedFundsSweepTimeout(opts *bind.TransactOpts, movingFundsTxHash [32]byte, movingFundsTxOutputIndex uint32, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyMovedFundsSweepTimeout", movingFundsTxHash, movingFundsTxOutputIndex, walletMembersIDs)
}

// NotifyMovedFundsSweepTimeout is a paid mutator transaction binding the contract method 0x50aea15a.
//
// Solidity: function notifyMovedFundsSweepTimeout(bytes32 movingFundsTxHash, uint32 movingFundsTxOutputIndex, uint32[] walletMembersIDs) returns()
func (_Bridge *BridgeSession) NotifyMovedFundsSweepTimeout(movingFundsTxHash [32]byte, movingFundsTxOutputIndex uint32, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyMovedFundsSweepTimeout(&_Bridge.TransactOpts, movingFundsTxHash, movingFundsTxOutputIndex, walletMembersIDs)
}

// NotifyMovedFundsSweepTimeout is a paid mutator transaction binding the contract method 0x50aea15a.
//
// Solidity: function notifyMovedFundsSweepTimeout(bytes32 movingFundsTxHash, uint32 movingFundsTxOutputIndex, uint32[] walletMembersIDs) returns()
func (_Bridge *BridgeTransactorSession) NotifyMovedFundsSweepTimeout(movingFundsTxHash [32]byte, movingFundsTxOutputIndex uint32, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyMovedFundsSweepTimeout(&_Bridge.TransactOpts, movingFundsTxHash, movingFundsTxOutputIndex, walletMembersIDs)
}

// NotifyMovingFundsBelowDust is a paid mutator transaction binding the contract method 0x07f7d223.
//
// Solidity: function notifyMovingFundsBelowDust(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo) returns()
func (_Bridge *BridgeTransactor) NotifyMovingFundsBelowDust(opts *bind.TransactOpts, walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyMovingFundsBelowDust", walletPubKeyHash, mainUtxo)
}

// NotifyMovingFundsBelowDust is a paid mutator transaction binding the contract method 0x07f7d223.
//
// Solidity: function notifyMovingFundsBelowDust(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo) returns()
func (_Bridge *BridgeSession) NotifyMovingFundsBelowDust(walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyMovingFundsBelowDust(&_Bridge.TransactOpts, walletPubKeyHash, mainUtxo)
}

// NotifyMovingFundsBelowDust is a paid mutator transaction binding the contract method 0x07f7d223.
//
// Solidity: function notifyMovingFundsBelowDust(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo) returns()
func (_Bridge *BridgeTransactorSession) NotifyMovingFundsBelowDust(walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyMovingFundsBelowDust(&_Bridge.TransactOpts, walletPubKeyHash, mainUtxo)
}

// NotifyMovingFundsTimeout is a paid mutator transaction binding the contract method 0x92238f32.
//
// Solidity: function notifyMovingFundsTimeout(bytes20 walletPubKeyHash, uint32[] walletMembersIDs) returns()
func (_Bridge *BridgeTransactor) NotifyMovingFundsTimeout(opts *bind.TransactOpts, walletPubKeyHash [20]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyMovingFundsTimeout", walletPubKeyHash, walletMembersIDs)
}

// NotifyMovingFundsTimeout is a paid mutator transaction binding the contract method 0x92238f32.
//
// Solidity: function notifyMovingFundsTimeout(bytes20 walletPubKeyHash, uint32[] walletMembersIDs) returns()
func (_Bridge *BridgeSession) NotifyMovingFundsTimeout(walletPubKeyHash [20]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyMovingFundsTimeout(&_Bridge.TransactOpts, walletPubKeyHash, walletMembersIDs)
}

// NotifyMovingFundsTimeout is a paid mutator transaction binding the contract method 0x92238f32.
//
// Solidity: function notifyMovingFundsTimeout(bytes20 walletPubKeyHash, uint32[] walletMembersIDs) returns()
func (_Bridge *BridgeTransactorSession) NotifyMovingFundsTimeout(walletPubKeyHash [20]byte, walletMembersIDs []uint32) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyMovingFundsTimeout(&_Bridge.TransactOpts, walletPubKeyHash, walletMembersIDs)
}

// NotifyRedemptionTimeout is a paid mutator transaction binding the contract method 0x31a4889a.
//
// Solidity: function notifyRedemptionTimeout(bytes20 walletPubKeyHash, uint32[] walletMembersIDs, bytes redeemerOutputScript) returns()
func (_Bridge *BridgeTransactor) NotifyRedemptionTimeout(opts *bind.TransactOpts, walletPubKeyHash [20]byte, walletMembersIDs []uint32, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyRedemptionTimeout", walletPubKeyHash, walletMembersIDs, redeemerOutputScript)
}

// NotifyRedemptionTimeout is a paid mutator transaction binding the contract method 0x31a4889a.
//
// Solidity: function notifyRedemptionTimeout(bytes20 walletPubKeyHash, uint32[] walletMembersIDs, bytes redeemerOutputScript) returns()
func (_Bridge *BridgeSession) NotifyRedemptionTimeout(walletPubKeyHash [20]byte, walletMembersIDs []uint32, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyRedemptionTimeout(&_Bridge.TransactOpts, walletPubKeyHash, walletMembersIDs, redeemerOutputScript)
}

// NotifyRedemptionTimeout is a paid mutator transaction binding the contract method 0x31a4889a.
//
// Solidity: function notifyRedemptionTimeout(bytes20 walletPubKeyHash, uint32[] walletMembersIDs, bytes redeemerOutputScript) returns()
func (_Bridge *BridgeTransactorSession) NotifyRedemptionTimeout(walletPubKeyHash [20]byte, walletMembersIDs []uint32, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyRedemptionTimeout(&_Bridge.TransactOpts, walletPubKeyHash, walletMembersIDs, redeemerOutputScript)
}

// NotifyRedemptionVeto is a paid mutator transaction binding the contract method 0x11026ef8.
//
// Solidity: function notifyRedemptionVeto(bytes20 walletPubKeyHash, bytes redeemerOutputScript) returns()
func (_Bridge *BridgeTransactor) NotifyRedemptionVeto(opts *bind.TransactOpts, walletPubKeyHash [20]byte, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyRedemptionVeto", walletPubKeyHash, redeemerOutputScript)
}

// NotifyRedemptionVeto is a paid mutator transaction binding the contract method 0x11026ef8.
//
// Solidity: function notifyRedemptionVeto(bytes20 walletPubKeyHash, bytes redeemerOutputScript) returns()
func (_Bridge *BridgeSession) NotifyRedemptionVeto(walletPubKeyHash [20]byte, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyRedemptionVeto(&_Bridge.TransactOpts, walletPubKeyHash, redeemerOutputScript)
}

// NotifyRedemptionVeto is a paid mutator transaction binding the contract method 0x11026ef8.
//
// Solidity: function notifyRedemptionVeto(bytes20 walletPubKeyHash, bytes redeemerOutputScript) returns()
func (_Bridge *BridgeTransactorSession) NotifyRedemptionVeto(walletPubKeyHash [20]byte, redeemerOutputScript []byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyRedemptionVeto(&_Bridge.TransactOpts, walletPubKeyHash, redeemerOutputScript)
}

// NotifyWalletCloseable is a paid mutator transaction binding the contract method 0xe44bdd31.
//
// Solidity: function notifyWalletCloseable(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo) returns()
func (_Bridge *BridgeTransactor) NotifyWalletCloseable(opts *bind.TransactOpts, walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyWalletCloseable", walletPubKeyHash, walletMainUtxo)
}

// NotifyWalletCloseable is a paid mutator transaction binding the contract method 0xe44bdd31.
//
// Solidity: function notifyWalletCloseable(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo) returns()
func (_Bridge *BridgeSession) NotifyWalletCloseable(walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyWalletCloseable(&_Bridge.TransactOpts, walletPubKeyHash, walletMainUtxo)
}

// NotifyWalletCloseable is a paid mutator transaction binding the contract method 0xe44bdd31.
//
// Solidity: function notifyWalletCloseable(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo) returns()
func (_Bridge *BridgeTransactorSession) NotifyWalletCloseable(walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyWalletCloseable(&_Bridge.TransactOpts, walletPubKeyHash, walletMainUtxo)
}

// NotifyWalletClosingPeriodElapsed is a paid mutator transaction binding the contract method 0xc8b5d2db.
//
// Solidity: function notifyWalletClosingPeriodElapsed(bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactor) NotifyWalletClosingPeriodElapsed(opts *bind.TransactOpts, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "notifyWalletClosingPeriodElapsed", walletPubKeyHash)
}

// NotifyWalletClosingPeriodElapsed is a paid mutator transaction binding the contract method 0xc8b5d2db.
//
// Solidity: function notifyWalletClosingPeriodElapsed(bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeSession) NotifyWalletClosingPeriodElapsed(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyWalletClosingPeriodElapsed(&_Bridge.TransactOpts, walletPubKeyHash)
}

// NotifyWalletClosingPeriodElapsed is a paid mutator transaction binding the contract method 0xc8b5d2db.
//
// Solidity: function notifyWalletClosingPeriodElapsed(bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactorSession) NotifyWalletClosingPeriodElapsed(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.NotifyWalletClosingPeriodElapsed(&_Bridge.TransactOpts, walletPubKeyHash)
}

// ReceiveBalanceApproval is a paid mutator transaction binding the contract method 0x475d0570.
//
// Solidity: function receiveBalanceApproval(address balanceOwner, uint256 amount, bytes redemptionData) returns()
func (_Bridge *BridgeTransactor) ReceiveBalanceApproval(opts *bind.TransactOpts, balanceOwner common.Address, amount *big.Int, redemptionData []byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "receiveBalanceApproval", balanceOwner, amount, redemptionData)
}

// ReceiveBalanceApproval is a paid mutator transaction binding the contract method 0x475d0570.
//
// Solidity: function receiveBalanceApproval(address balanceOwner, uint256 amount, bytes redemptionData) returns()
func (_Bridge *BridgeSession) ReceiveBalanceApproval(balanceOwner common.Address, amount *big.Int, redemptionData []byte) (*types.Transaction, error) {
	return _Bridge.Contract.ReceiveBalanceApproval(&_Bridge.TransactOpts, balanceOwner, amount, redemptionData)
}

// ReceiveBalanceApproval is a paid mutator transaction binding the contract method 0x475d0570.
//
// Solidity: function receiveBalanceApproval(address balanceOwner, uint256 amount, bytes redemptionData) returns()
func (_Bridge *BridgeTransactorSession) ReceiveBalanceApproval(balanceOwner common.Address, amount *big.Int, redemptionData []byte) (*types.Transaction, error) {
	return _Bridge.Contract.ReceiveBalanceApproval(&_Bridge.TransactOpts, balanceOwner, amount, redemptionData)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0xa145e2d5.
//
// Solidity: function requestNewWallet((bytes32,uint32,uint64) activeWalletMainUtxo) returns()
func (_Bridge *BridgeTransactor) RequestNewWallet(opts *bind.TransactOpts, activeWalletMainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "requestNewWallet", activeWalletMainUtxo)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0xa145e2d5.
//
// Solidity: function requestNewWallet((bytes32,uint32,uint64) activeWalletMainUtxo) returns()
func (_Bridge *BridgeSession) RequestNewWallet(activeWalletMainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.RequestNewWallet(&_Bridge.TransactOpts, activeWalletMainUtxo)
}

// RequestNewWallet is a paid mutator transaction binding the contract method 0xa145e2d5.
//
// Solidity: function requestNewWallet((bytes32,uint32,uint64) activeWalletMainUtxo) returns()
func (_Bridge *BridgeTransactorSession) RequestNewWallet(activeWalletMainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.RequestNewWallet(&_Bridge.TransactOpts, activeWalletMainUtxo)
}

// RequestRedemption is a paid mutator transaction binding the contract method 0xd6eccdf0.
//
// Solidity: function requestRedemption(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo, bytes redeemerOutputScript, uint64 amount) returns()
func (_Bridge *BridgeTransactor) RequestRedemption(opts *bind.TransactOpts, walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO, redeemerOutputScript []byte, amount uint64) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "requestRedemption", walletPubKeyHash, mainUtxo, redeemerOutputScript, amount)
}

// RequestRedemption is a paid mutator transaction binding the contract method 0xd6eccdf0.
//
// Solidity: function requestRedemption(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo, bytes redeemerOutputScript, uint64 amount) returns()
func (_Bridge *BridgeSession) RequestRedemption(walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO, redeemerOutputScript []byte, amount uint64) (*types.Transaction, error) {
	return _Bridge.Contract.RequestRedemption(&_Bridge.TransactOpts, walletPubKeyHash, mainUtxo, redeemerOutputScript, amount)
}

// RequestRedemption is a paid mutator transaction binding the contract method 0xd6eccdf0.
//
// Solidity: function requestRedemption(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) mainUtxo, bytes redeemerOutputScript, uint64 amount) returns()
func (_Bridge *BridgeTransactorSession) RequestRedemption(walletPubKeyHash [20]byte, mainUtxo BitcoinTxUTXO, redeemerOutputScript []byte, amount uint64) (*types.Transaction, error) {
	return _Bridge.Contract.RequestRedemption(&_Bridge.TransactOpts, walletPubKeyHash, mainUtxo, redeemerOutputScript, amount)
}

// ResetMovingFundsTimeout is a paid mutator transaction binding the contract method 0xee1dd3ea.
//
// Solidity: function resetMovingFundsTimeout(bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactor) ResetMovingFundsTimeout(opts *bind.TransactOpts, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "resetMovingFundsTimeout", walletPubKeyHash)
}

// ResetMovingFundsTimeout is a paid mutator transaction binding the contract method 0xee1dd3ea.
//
// Solidity: function resetMovingFundsTimeout(bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeSession) ResetMovingFundsTimeout(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.ResetMovingFundsTimeout(&_Bridge.TransactOpts, walletPubKeyHash)
}

// ResetMovingFundsTimeout is a paid mutator transaction binding the contract method 0xee1dd3ea.
//
// Solidity: function resetMovingFundsTimeout(bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactorSession) ResetMovingFundsTimeout(walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.ResetMovingFundsTimeout(&_Bridge.TransactOpts, walletPubKeyHash)
}

// RevealDeposit is a paid mutator transaction binding the contract method 0xfca4ba4c.
//
// Solidity: function revealDeposit((bytes4,bytes,bytes,bytes4) fundingTx, (uint32,bytes8,bytes20,bytes20,bytes4,address) reveal) returns()
func (_Bridge *BridgeTransactor) RevealDeposit(opts *bind.TransactOpts, fundingTx BitcoinTxInfo, reveal DepositDepositRevealInfo) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "revealDeposit", fundingTx, reveal)
}

// RevealDeposit is a paid mutator transaction binding the contract method 0xfca4ba4c.
//
// Solidity: function revealDeposit((bytes4,bytes,bytes,bytes4) fundingTx, (uint32,bytes8,bytes20,bytes20,bytes4,address) reveal) returns()
func (_Bridge *BridgeSession) RevealDeposit(fundingTx BitcoinTxInfo, reveal DepositDepositRevealInfo) (*types.Transaction, error) {
	return _Bridge.Contract.RevealDeposit(&_Bridge.TransactOpts, fundingTx, reveal)
}

// RevealDeposit is a paid mutator transaction binding the contract method 0xfca4ba4c.
//
// Solidity: function revealDeposit((bytes4,bytes,bytes,bytes4) fundingTx, (uint32,bytes8,bytes20,bytes20,bytes4,address) reveal) returns()
func (_Bridge *BridgeTransactorSession) RevealDeposit(fundingTx BitcoinTxInfo, reveal DepositDepositRevealInfo) (*types.Transaction, error) {
	return _Bridge.Contract.RevealDeposit(&_Bridge.TransactOpts, fundingTx, reveal)
}

// RevealDepositWithExtraData is a paid mutator transaction binding the contract method 0x86f01439.
//
// Solidity: function revealDepositWithExtraData((bytes4,bytes,bytes,bytes4) fundingTx, (uint32,bytes8,bytes20,bytes20,bytes4,address) reveal, bytes32 extraData) returns()
func (_Bridge *BridgeTransactor) RevealDepositWithExtraData(opts *bind.TransactOpts, fundingTx BitcoinTxInfo, reveal DepositDepositRevealInfo, extraData [32]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "revealDepositWithExtraData", fundingTx, reveal, extraData)
}

// RevealDepositWithExtraData is a paid mutator transaction binding the contract method 0x86f01439.
//
// Solidity: function revealDepositWithExtraData((bytes4,bytes,bytes,bytes4) fundingTx, (uint32,bytes8,bytes20,bytes20,bytes4,address) reveal, bytes32 extraData) returns()
func (_Bridge *BridgeSession) RevealDepositWithExtraData(fundingTx BitcoinTxInfo, reveal DepositDepositRevealInfo, extraData [32]byte) (*types.Transaction, error) {
	return _Bridge.Contract.RevealDepositWithExtraData(&_Bridge.TransactOpts, fundingTx, reveal, extraData)
}

// RevealDepositWithExtraData is a paid mutator transaction binding the contract method 0x86f01439.
//
// Solidity: function revealDepositWithExtraData((bytes4,bytes,bytes,bytes4) fundingTx, (uint32,bytes8,bytes20,bytes20,bytes4,address) reveal, bytes32 extraData) returns()
func (_Bridge *BridgeTransactorSession) RevealDepositWithExtraData(fundingTx BitcoinTxInfo, reveal DepositDepositRevealInfo, extraData [32]byte) (*types.Transaction, error) {
	return _Bridge.Contract.RevealDepositWithExtraData(&_Bridge.TransactOpts, fundingTx, reveal, extraData)
}

// SetRebateStaking is a paid mutator transaction binding the contract method 0xca73c462.
//
// Solidity: function setRebateStaking(address rebateStaking) returns()
func (_Bridge *BridgeTransactor) SetRebateStaking(opts *bind.TransactOpts, rebateStaking common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "setRebateStaking", rebateStaking)
}

// SetRebateStaking is a paid mutator transaction binding the contract method 0xca73c462.
//
// Solidity: function setRebateStaking(address rebateStaking) returns()
func (_Bridge *BridgeSession) SetRebateStaking(rebateStaking common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SetRebateStaking(&_Bridge.TransactOpts, rebateStaking)
}

// SetRebateStaking is a paid mutator transaction binding the contract method 0xca73c462.
//
// Solidity: function setRebateStaking(address rebateStaking) returns()
func (_Bridge *BridgeTransactorSession) SetRebateStaking(rebateStaking common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SetRebateStaking(&_Bridge.TransactOpts, rebateStaking)
}

// SetRedemptionWatchtower is a paid mutator transaction binding the contract method 0xbe26ebad.
//
// Solidity: function setRedemptionWatchtower(address redemptionWatchtower) returns()
func (_Bridge *BridgeTransactor) SetRedemptionWatchtower(opts *bind.TransactOpts, redemptionWatchtower common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "setRedemptionWatchtower", redemptionWatchtower)
}

// SetRedemptionWatchtower is a paid mutator transaction binding the contract method 0xbe26ebad.
//
// Solidity: function setRedemptionWatchtower(address redemptionWatchtower) returns()
func (_Bridge *BridgeSession) SetRedemptionWatchtower(redemptionWatchtower common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SetRedemptionWatchtower(&_Bridge.TransactOpts, redemptionWatchtower)
}

// SetRedemptionWatchtower is a paid mutator transaction binding the contract method 0xbe26ebad.
//
// Solidity: function setRedemptionWatchtower(address redemptionWatchtower) returns()
func (_Bridge *BridgeTransactorSession) SetRedemptionWatchtower(redemptionWatchtower common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SetRedemptionWatchtower(&_Bridge.TransactOpts, redemptionWatchtower)
}

// SetReservationRouter is a paid mutator transaction binding the contract method 0xd394a4d3.
//
// Solidity: function setReservationRouter(address _reservationRouter) returns()
func (_Bridge *BridgeTransactor) SetReservationRouter(opts *bind.TransactOpts, _reservationRouter common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "setReservationRouter", _reservationRouter)
}

// SetReservationRouter is a paid mutator transaction binding the contract method 0xd394a4d3.
//
// Solidity: function setReservationRouter(address _reservationRouter) returns()
func (_Bridge *BridgeSession) SetReservationRouter(_reservationRouter common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SetReservationRouter(&_Bridge.TransactOpts, _reservationRouter)
}

// SetReservationRouter is a paid mutator transaction binding the contract method 0xd394a4d3.
//
// Solidity: function setReservationRouter(address _reservationRouter) returns()
func (_Bridge *BridgeTransactorSession) SetReservationRouter(_reservationRouter common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SetReservationRouter(&_Bridge.TransactOpts, _reservationRouter)
}

// SetSpvMaintainerStatus is a paid mutator transaction binding the contract method 0x5f2b2d0d.
//
// Solidity: function setSpvMaintainerStatus(address spvMaintainer, bool isTrusted) returns()
func (_Bridge *BridgeTransactor) SetSpvMaintainerStatus(opts *bind.TransactOpts, spvMaintainer common.Address, isTrusted bool) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "setSpvMaintainerStatus", spvMaintainer, isTrusted)
}

// SetSpvMaintainerStatus is a paid mutator transaction binding the contract method 0x5f2b2d0d.
//
// Solidity: function setSpvMaintainerStatus(address spvMaintainer, bool isTrusted) returns()
func (_Bridge *BridgeSession) SetSpvMaintainerStatus(spvMaintainer common.Address, isTrusted bool) (*types.Transaction, error) {
	return _Bridge.Contract.SetSpvMaintainerStatus(&_Bridge.TransactOpts, spvMaintainer, isTrusted)
}

// SetSpvMaintainerStatus is a paid mutator transaction binding the contract method 0x5f2b2d0d.
//
// Solidity: function setSpvMaintainerStatus(address spvMaintainer, bool isTrusted) returns()
func (_Bridge *BridgeTransactorSession) SetSpvMaintainerStatus(spvMaintainer common.Address, isTrusted bool) (*types.Transaction, error) {
	return _Bridge.Contract.SetSpvMaintainerStatus(&_Bridge.TransactOpts, spvMaintainer, isTrusted)
}

// SetVaultStatus is a paid mutator transaction binding the contract method 0x60d712fc.
//
// Solidity: function setVaultStatus(address vault, bool isTrusted) returns()
func (_Bridge *BridgeTransactor) SetVaultStatus(opts *bind.TransactOpts, vault common.Address, isTrusted bool) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "setVaultStatus", vault, isTrusted)
}

// SetVaultStatus is a paid mutator transaction binding the contract method 0x60d712fc.
//
// Solidity: function setVaultStatus(address vault, bool isTrusted) returns()
func (_Bridge *BridgeSession) SetVaultStatus(vault common.Address, isTrusted bool) (*types.Transaction, error) {
	return _Bridge.Contract.SetVaultStatus(&_Bridge.TransactOpts, vault, isTrusted)
}

// SetVaultStatus is a paid mutator transaction binding the contract method 0x60d712fc.
//
// Solidity: function setVaultStatus(address vault, bool isTrusted) returns()
func (_Bridge *BridgeTransactorSession) SetVaultStatus(vault common.Address, isTrusted bool) (*types.Transaction, error) {
	return _Bridge.Contract.SetVaultStatus(&_Bridge.TransactOpts, vault, isTrusted)
}

// SubmitDepositSweepProof is a paid mutator transaction binding the contract method 0xbd150131.
//
// Solidity: function submitDepositSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo, address vault) returns()
func (_Bridge *BridgeTransactor) SubmitDepositSweepProof(opts *bind.TransactOpts, sweepTx BitcoinTxInfo, sweepProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, vault common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "submitDepositSweepProof", sweepTx, sweepProof, mainUtxo, vault)
}

// SubmitDepositSweepProof is a paid mutator transaction binding the contract method 0xbd150131.
//
// Solidity: function submitDepositSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo, address vault) returns()
func (_Bridge *BridgeSession) SubmitDepositSweepProof(sweepTx BitcoinTxInfo, sweepProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, vault common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitDepositSweepProof(&_Bridge.TransactOpts, sweepTx, sweepProof, mainUtxo, vault)
}

// SubmitDepositSweepProof is a paid mutator transaction binding the contract method 0xbd150131.
//
// Solidity: function submitDepositSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo, address vault) returns()
func (_Bridge *BridgeTransactorSession) SubmitDepositSweepProof(sweepTx BitcoinTxInfo, sweepProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, vault common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitDepositSweepProof(&_Bridge.TransactOpts, sweepTx, sweepProof, mainUtxo, vault)
}

// SubmitFraudChallenge is a paid mutator transaction binding the contract method 0x685ce1b1.
//
// Solidity: function submitFraudChallenge(bytes walletPublicKey, bytes preimageSha256, (bytes32,bytes32,uint8) signature) payable returns()
func (_Bridge *BridgeTransactor) SubmitFraudChallenge(opts *bind.TransactOpts, walletPublicKey []byte, preimageSha256 []byte, signature BitcoinTxRSVSignature) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "submitFraudChallenge", walletPublicKey, preimageSha256, signature)
}

// SubmitFraudChallenge is a paid mutator transaction binding the contract method 0x685ce1b1.
//
// Solidity: function submitFraudChallenge(bytes walletPublicKey, bytes preimageSha256, (bytes32,bytes32,uint8) signature) payable returns()
func (_Bridge *BridgeSession) SubmitFraudChallenge(walletPublicKey []byte, preimageSha256 []byte, signature BitcoinTxRSVSignature) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitFraudChallenge(&_Bridge.TransactOpts, walletPublicKey, preimageSha256, signature)
}

// SubmitFraudChallenge is a paid mutator transaction binding the contract method 0x685ce1b1.
//
// Solidity: function submitFraudChallenge(bytes walletPublicKey, bytes preimageSha256, (bytes32,bytes32,uint8) signature) payable returns()
func (_Bridge *BridgeTransactorSession) SubmitFraudChallenge(walletPublicKey []byte, preimageSha256 []byte, signature BitcoinTxRSVSignature) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitFraudChallenge(&_Bridge.TransactOpts, walletPublicKey, preimageSha256, signature)
}

// SubmitMovedFundsSweepProof is a paid mutator transaction binding the contract method 0x9821c38b.
//
// Solidity: function submitMovedFundsSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo) returns()
func (_Bridge *BridgeTransactor) SubmitMovedFundsSweepProof(opts *bind.TransactOpts, sweepTx BitcoinTxInfo, sweepProof BitcoinTxProof, mainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "submitMovedFundsSweepProof", sweepTx, sweepProof, mainUtxo)
}

// SubmitMovedFundsSweepProof is a paid mutator transaction binding the contract method 0x9821c38b.
//
// Solidity: function submitMovedFundsSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo) returns()
func (_Bridge *BridgeSession) SubmitMovedFundsSweepProof(sweepTx BitcoinTxInfo, sweepProof BitcoinTxProof, mainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitMovedFundsSweepProof(&_Bridge.TransactOpts, sweepTx, sweepProof, mainUtxo)
}

// SubmitMovedFundsSweepProof is a paid mutator transaction binding the contract method 0x9821c38b.
//
// Solidity: function submitMovedFundsSweepProof((bytes4,bytes,bytes,bytes4) sweepTx, (bytes,uint256,bytes,bytes32,bytes) sweepProof, (bytes32,uint32,uint64) mainUtxo) returns()
func (_Bridge *BridgeTransactorSession) SubmitMovedFundsSweepProof(sweepTx BitcoinTxInfo, sweepProof BitcoinTxProof, mainUtxo BitcoinTxUTXO) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitMovedFundsSweepProof(&_Bridge.TransactOpts, sweepTx, sweepProof, mainUtxo)
}

// SubmitMovingFundsCommitment is a paid mutator transaction binding the contract method 0xabaeed8f.
//
// Solidity: function submitMovingFundsCommitment(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo, uint32[] walletMembersIDs, uint256 walletMemberIndex, bytes20[] targetWallets) returns()
func (_Bridge *BridgeTransactor) SubmitMovingFundsCommitment(opts *bind.TransactOpts, walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO, walletMembersIDs []uint32, walletMemberIndex *big.Int, targetWallets [][20]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "submitMovingFundsCommitment", walletPubKeyHash, walletMainUtxo, walletMembersIDs, walletMemberIndex, targetWallets)
}

// SubmitMovingFundsCommitment is a paid mutator transaction binding the contract method 0xabaeed8f.
//
// Solidity: function submitMovingFundsCommitment(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo, uint32[] walletMembersIDs, uint256 walletMemberIndex, bytes20[] targetWallets) returns()
func (_Bridge *BridgeSession) SubmitMovingFundsCommitment(walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO, walletMembersIDs []uint32, walletMemberIndex *big.Int, targetWallets [][20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitMovingFundsCommitment(&_Bridge.TransactOpts, walletPubKeyHash, walletMainUtxo, walletMembersIDs, walletMemberIndex, targetWallets)
}

// SubmitMovingFundsCommitment is a paid mutator transaction binding the contract method 0xabaeed8f.
//
// Solidity: function submitMovingFundsCommitment(bytes20 walletPubKeyHash, (bytes32,uint32,uint64) walletMainUtxo, uint32[] walletMembersIDs, uint256 walletMemberIndex, bytes20[] targetWallets) returns()
func (_Bridge *BridgeTransactorSession) SubmitMovingFundsCommitment(walletPubKeyHash [20]byte, walletMainUtxo BitcoinTxUTXO, walletMembersIDs []uint32, walletMemberIndex *big.Int, targetWallets [][20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitMovingFundsCommitment(&_Bridge.TransactOpts, walletPubKeyHash, walletMainUtxo, walletMembersIDs, walletMemberIndex, targetWallets)
}

// SubmitMovingFundsProof is a paid mutator transaction binding the contract method 0xe509397d.
//
// Solidity: function submitMovingFundsProof((bytes4,bytes,bytes,bytes4) movingFundsTx, (bytes,uint256,bytes,bytes32,bytes) movingFundsProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactor) SubmitMovingFundsProof(opts *bind.TransactOpts, movingFundsTx BitcoinTxInfo, movingFundsProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "submitMovingFundsProof", movingFundsTx, movingFundsProof, mainUtxo, walletPubKeyHash)
}

// SubmitMovingFundsProof is a paid mutator transaction binding the contract method 0xe509397d.
//
// Solidity: function submitMovingFundsProof((bytes4,bytes,bytes,bytes4) movingFundsTx, (bytes,uint256,bytes,bytes32,bytes) movingFundsProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeSession) SubmitMovingFundsProof(movingFundsTx BitcoinTxInfo, movingFundsProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitMovingFundsProof(&_Bridge.TransactOpts, movingFundsTx, movingFundsProof, mainUtxo, walletPubKeyHash)
}

// SubmitMovingFundsProof is a paid mutator transaction binding the contract method 0xe509397d.
//
// Solidity: function submitMovingFundsProof((bytes4,bytes,bytes,bytes4) movingFundsTx, (bytes,uint256,bytes,bytes32,bytes) movingFundsProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactorSession) SubmitMovingFundsProof(movingFundsTx BitcoinTxInfo, movingFundsProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitMovingFundsProof(&_Bridge.TransactOpts, movingFundsTx, movingFundsProof, mainUtxo, walletPubKeyHash)
}

// SubmitRedemptionProof is a paid mutator transaction binding the contract method 0x8ac89ce1.
//
// Solidity: function submitRedemptionProof((bytes4,bytes,bytes,bytes4) redemptionTx, (bytes,uint256,bytes,bytes32,bytes) redemptionProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactor) SubmitRedemptionProof(opts *bind.TransactOpts, redemptionTx BitcoinTxInfo, redemptionProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "submitRedemptionProof", redemptionTx, redemptionProof, mainUtxo, walletPubKeyHash)
}

// SubmitRedemptionProof is a paid mutator transaction binding the contract method 0x8ac89ce1.
//
// Solidity: function submitRedemptionProof((bytes4,bytes,bytes,bytes4) redemptionTx, (bytes,uint256,bytes,bytes32,bytes) redemptionProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeSession) SubmitRedemptionProof(redemptionTx BitcoinTxInfo, redemptionProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitRedemptionProof(&_Bridge.TransactOpts, redemptionTx, redemptionProof, mainUtxo, walletPubKeyHash)
}

// SubmitRedemptionProof is a paid mutator transaction binding the contract method 0x8ac89ce1.
//
// Solidity: function submitRedemptionProof((bytes4,bytes,bytes,bytes4) redemptionTx, (bytes,uint256,bytes,bytes32,bytes) redemptionProof, (bytes32,uint32,uint64) mainUtxo, bytes20 walletPubKeyHash) returns()
func (_Bridge *BridgeTransactorSession) SubmitRedemptionProof(redemptionTx BitcoinTxInfo, redemptionProof BitcoinTxProof, mainUtxo BitcoinTxUTXO, walletPubKeyHash [20]byte) (*types.Transaction, error) {
	return _Bridge.Contract.SubmitRedemptionProof(&_Bridge.TransactOpts, redemptionTx, redemptionProof, mainUtxo, walletPubKeyHash)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_Bridge *BridgeTransactor) TransferGovernance(opts *bind.TransactOpts, newGovernance common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "transferGovernance", newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_Bridge *BridgeSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.TransferGovernance(&_Bridge.TransactOpts, newGovernance)
}

// TransferGovernance is a paid mutator transaction binding the contract method 0xd38bfff4.
//
// Solidity: function transferGovernance(address newGovernance) returns()
func (_Bridge *BridgeTransactorSession) TransferGovernance(newGovernance common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.TransferGovernance(&_Bridge.TransactOpts, newGovernance)
}

// UpdateDepositParameters is a paid mutator transaction binding the contract method 0x38326507.
//
// Solidity: function updateDepositParameters(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod) returns()
func (_Bridge *BridgeTransactor) UpdateDepositParameters(opts *bind.TransactOpts, depositDustThreshold uint64, depositTreasuryFeeDivisor uint64, depositTxMaxFee uint64, depositRevealAheadPeriod uint32) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "updateDepositParameters", depositDustThreshold, depositTreasuryFeeDivisor, depositTxMaxFee, depositRevealAheadPeriod)
}

// UpdateDepositParameters is a paid mutator transaction binding the contract method 0x38326507.
//
// Solidity: function updateDepositParameters(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod) returns()
func (_Bridge *BridgeSession) UpdateDepositParameters(depositDustThreshold uint64, depositTreasuryFeeDivisor uint64, depositTxMaxFee uint64, depositRevealAheadPeriod uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateDepositParameters(&_Bridge.TransactOpts, depositDustThreshold, depositTreasuryFeeDivisor, depositTxMaxFee, depositRevealAheadPeriod)
}

// UpdateDepositParameters is a paid mutator transaction binding the contract method 0x38326507.
//
// Solidity: function updateDepositParameters(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod) returns()
func (_Bridge *BridgeTransactorSession) UpdateDepositParameters(depositDustThreshold uint64, depositTreasuryFeeDivisor uint64, depositTxMaxFee uint64, depositRevealAheadPeriod uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateDepositParameters(&_Bridge.TransactOpts, depositDustThreshold, depositTreasuryFeeDivisor, depositTxMaxFee, depositRevealAheadPeriod)
}

// UpdateFraudParameters is a paid mutator transaction binding the contract method 0xdc49117b.
//
// Solidity: function updateFraudParameters(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier) returns()
func (_Bridge *BridgeTransactor) UpdateFraudParameters(opts *bind.TransactOpts, fraudChallengeDepositAmount *big.Int, fraudChallengeDefeatTimeout uint32, fraudSlashingAmount *big.Int, fraudNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "updateFraudParameters", fraudChallengeDepositAmount, fraudChallengeDefeatTimeout, fraudSlashingAmount, fraudNotifierRewardMultiplier)
}

// UpdateFraudParameters is a paid mutator transaction binding the contract method 0xdc49117b.
//
// Solidity: function updateFraudParameters(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier) returns()
func (_Bridge *BridgeSession) UpdateFraudParameters(fraudChallengeDepositAmount *big.Int, fraudChallengeDefeatTimeout uint32, fraudSlashingAmount *big.Int, fraudNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateFraudParameters(&_Bridge.TransactOpts, fraudChallengeDepositAmount, fraudChallengeDefeatTimeout, fraudSlashingAmount, fraudNotifierRewardMultiplier)
}

// UpdateFraudParameters is a paid mutator transaction binding the contract method 0xdc49117b.
//
// Solidity: function updateFraudParameters(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier) returns()
func (_Bridge *BridgeTransactorSession) UpdateFraudParameters(fraudChallengeDepositAmount *big.Int, fraudChallengeDefeatTimeout uint32, fraudSlashingAmount *big.Int, fraudNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateFraudParameters(&_Bridge.TransactOpts, fraudChallengeDepositAmount, fraudChallengeDefeatTimeout, fraudSlashingAmount, fraudNotifierRewardMultiplier)
}

// UpdateMovingFundsParameters is a paid mutator transaction binding the contract method 0xc3b5ffc9.
//
// Solidity: function updateMovingFundsParameters(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier) returns()
func (_Bridge *BridgeTransactor) UpdateMovingFundsParameters(opts *bind.TransactOpts, movingFundsTxMaxTotalFee uint64, movingFundsDustThreshold uint64, movingFundsTimeoutResetDelay uint32, movingFundsTimeout uint32, movingFundsTimeoutSlashingAmount *big.Int, movingFundsTimeoutNotifierRewardMultiplier uint32, movingFundsCommitmentGasOffset uint16, movedFundsSweepTxMaxTotalFee uint64, movedFundsSweepTimeout uint32, movedFundsSweepTimeoutSlashingAmount *big.Int, movedFundsSweepTimeoutNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "updateMovingFundsParameters", movingFundsTxMaxTotalFee, movingFundsDustThreshold, movingFundsTimeoutResetDelay, movingFundsTimeout, movingFundsTimeoutSlashingAmount, movingFundsTimeoutNotifierRewardMultiplier, movingFundsCommitmentGasOffset, movedFundsSweepTxMaxTotalFee, movedFundsSweepTimeout, movedFundsSweepTimeoutSlashingAmount, movedFundsSweepTimeoutNotifierRewardMultiplier)
}

// UpdateMovingFundsParameters is a paid mutator transaction binding the contract method 0xc3b5ffc9.
//
// Solidity: function updateMovingFundsParameters(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier) returns()
func (_Bridge *BridgeSession) UpdateMovingFundsParameters(movingFundsTxMaxTotalFee uint64, movingFundsDustThreshold uint64, movingFundsTimeoutResetDelay uint32, movingFundsTimeout uint32, movingFundsTimeoutSlashingAmount *big.Int, movingFundsTimeoutNotifierRewardMultiplier uint32, movingFundsCommitmentGasOffset uint16, movedFundsSweepTxMaxTotalFee uint64, movedFundsSweepTimeout uint32, movedFundsSweepTimeoutSlashingAmount *big.Int, movedFundsSweepTimeoutNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateMovingFundsParameters(&_Bridge.TransactOpts, movingFundsTxMaxTotalFee, movingFundsDustThreshold, movingFundsTimeoutResetDelay, movingFundsTimeout, movingFundsTimeoutSlashingAmount, movingFundsTimeoutNotifierRewardMultiplier, movingFundsCommitmentGasOffset, movedFundsSweepTxMaxTotalFee, movedFundsSweepTimeout, movedFundsSweepTimeoutSlashingAmount, movedFundsSweepTimeoutNotifierRewardMultiplier)
}

// UpdateMovingFundsParameters is a paid mutator transaction binding the contract method 0xc3b5ffc9.
//
// Solidity: function updateMovingFundsParameters(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier) returns()
func (_Bridge *BridgeTransactorSession) UpdateMovingFundsParameters(movingFundsTxMaxTotalFee uint64, movingFundsDustThreshold uint64, movingFundsTimeoutResetDelay uint32, movingFundsTimeout uint32, movingFundsTimeoutSlashingAmount *big.Int, movingFundsTimeoutNotifierRewardMultiplier uint32, movingFundsCommitmentGasOffset uint16, movedFundsSweepTxMaxTotalFee uint64, movedFundsSweepTimeout uint32, movedFundsSweepTimeoutSlashingAmount *big.Int, movedFundsSweepTimeoutNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateMovingFundsParameters(&_Bridge.TransactOpts, movingFundsTxMaxTotalFee, movingFundsDustThreshold, movingFundsTimeoutResetDelay, movingFundsTimeout, movingFundsTimeoutSlashingAmount, movingFundsTimeoutNotifierRewardMultiplier, movingFundsCommitmentGasOffset, movedFundsSweepTxMaxTotalFee, movedFundsSweepTimeout, movedFundsSweepTimeoutSlashingAmount, movedFundsSweepTimeoutNotifierRewardMultiplier)
}

// UpdateRedemptionParameters is a paid mutator transaction binding the contract method 0xb5da2171.
//
// Solidity: function updateRedemptionParameters(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier) returns()
func (_Bridge *BridgeTransactor) UpdateRedemptionParameters(opts *bind.TransactOpts, redemptionDustThreshold uint64, redemptionTreasuryFeeDivisor uint64, redemptionTxMaxFee uint64, redemptionTxMaxTotalFee uint64, redemptionTimeout uint32, redemptionTimeoutSlashingAmount *big.Int, redemptionTimeoutNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "updateRedemptionParameters", redemptionDustThreshold, redemptionTreasuryFeeDivisor, redemptionTxMaxFee, redemptionTxMaxTotalFee, redemptionTimeout, redemptionTimeoutSlashingAmount, redemptionTimeoutNotifierRewardMultiplier)
}

// UpdateRedemptionParameters is a paid mutator transaction binding the contract method 0xb5da2171.
//
// Solidity: function updateRedemptionParameters(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier) returns()
func (_Bridge *BridgeSession) UpdateRedemptionParameters(redemptionDustThreshold uint64, redemptionTreasuryFeeDivisor uint64, redemptionTxMaxFee uint64, redemptionTxMaxTotalFee uint64, redemptionTimeout uint32, redemptionTimeoutSlashingAmount *big.Int, redemptionTimeoutNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateRedemptionParameters(&_Bridge.TransactOpts, redemptionDustThreshold, redemptionTreasuryFeeDivisor, redemptionTxMaxFee, redemptionTxMaxTotalFee, redemptionTimeout, redemptionTimeoutSlashingAmount, redemptionTimeoutNotifierRewardMultiplier)
}

// UpdateRedemptionParameters is a paid mutator transaction binding the contract method 0xb5da2171.
//
// Solidity: function updateRedemptionParameters(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier) returns()
func (_Bridge *BridgeTransactorSession) UpdateRedemptionParameters(redemptionDustThreshold uint64, redemptionTreasuryFeeDivisor uint64, redemptionTxMaxFee uint64, redemptionTxMaxTotalFee uint64, redemptionTimeout uint32, redemptionTimeoutSlashingAmount *big.Int, redemptionTimeoutNotifierRewardMultiplier uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateRedemptionParameters(&_Bridge.TransactOpts, redemptionDustThreshold, redemptionTreasuryFeeDivisor, redemptionTxMaxFee, redemptionTxMaxTotalFee, redemptionTimeout, redemptionTimeoutSlashingAmount, redemptionTimeoutNotifierRewardMultiplier)
}

// UpdateTreasury is a paid mutator transaction binding the contract method 0x7f51bb1f.
//
// Solidity: function updateTreasury(address treasury) returns()
func (_Bridge *BridgeTransactor) UpdateTreasury(opts *bind.TransactOpts, treasury common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "updateTreasury", treasury)
}

// UpdateTreasury is a paid mutator transaction binding the contract method 0x7f51bb1f.
//
// Solidity: function updateTreasury(address treasury) returns()
func (_Bridge *BridgeSession) UpdateTreasury(treasury common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateTreasury(&_Bridge.TransactOpts, treasury)
}

// UpdateTreasury is a paid mutator transaction binding the contract method 0x7f51bb1f.
//
// Solidity: function updateTreasury(address treasury) returns()
func (_Bridge *BridgeTransactorSession) UpdateTreasury(treasury common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateTreasury(&_Bridge.TransactOpts, treasury)
}

// UpdateWalletParameters is a paid mutator transaction binding the contract method 0x883d6a11.
//
// Solidity: function updateWalletParameters(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod) returns()
func (_Bridge *BridgeTransactor) UpdateWalletParameters(opts *bind.TransactOpts, walletCreationPeriod uint32, walletCreationMinBtcBalance uint64, walletCreationMaxBtcBalance uint64, walletClosureMinBtcBalance uint64, walletMaxAge uint32, walletMaxBtcTransfer uint64, walletClosingPeriod uint32) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "updateWalletParameters", walletCreationPeriod, walletCreationMinBtcBalance, walletCreationMaxBtcBalance, walletClosureMinBtcBalance, walletMaxAge, walletMaxBtcTransfer, walletClosingPeriod)
}

// UpdateWalletParameters is a paid mutator transaction binding the contract method 0x883d6a11.
//
// Solidity: function updateWalletParameters(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod) returns()
func (_Bridge *BridgeSession) UpdateWalletParameters(walletCreationPeriod uint32, walletCreationMinBtcBalance uint64, walletCreationMaxBtcBalance uint64, walletClosureMinBtcBalance uint64, walletMaxAge uint32, walletMaxBtcTransfer uint64, walletClosingPeriod uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateWalletParameters(&_Bridge.TransactOpts, walletCreationPeriod, walletCreationMinBtcBalance, walletCreationMaxBtcBalance, walletClosureMinBtcBalance, walletMaxAge, walletMaxBtcTransfer, walletClosingPeriod)
}

// UpdateWalletParameters is a paid mutator transaction binding the contract method 0x883d6a11.
//
// Solidity: function updateWalletParameters(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod) returns()
func (_Bridge *BridgeTransactorSession) UpdateWalletParameters(walletCreationPeriod uint32, walletCreationMinBtcBalance uint64, walletCreationMaxBtcBalance uint64, walletClosureMinBtcBalance uint64, walletMaxAge uint32, walletMaxBtcTransfer uint64, walletClosingPeriod uint32) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateWalletParameters(&_Bridge.TransactOpts, walletCreationPeriod, walletCreationMinBtcBalance, walletCreationMaxBtcBalance, walletClosureMinBtcBalance, walletMaxAge, walletMaxBtcTransfer, walletClosingPeriod)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() payable returns()
func (_Bridge *BridgeTransactor) Fallback(opts *bind.TransactOpts, calldata []byte) (*types.Transaction, error) {
	return _Bridge.contract.RawTransact(opts, calldata)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() payable returns()
func (_Bridge *BridgeSession) Fallback(calldata []byte) (*types.Transaction, error) {
	return _Bridge.Contract.Fallback(&_Bridge.TransactOpts, calldata)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() payable returns()
func (_Bridge *BridgeTransactorSession) Fallback(calldata []byte) (*types.Transaction, error) {
	return _Bridge.Contract.Fallback(&_Bridge.TransactOpts, calldata)
}

// BridgeDepositParametersUpdatedIterator is returned from FilterDepositParametersUpdated and is used to iterate over the raw logs and unpacked data for DepositParametersUpdated events raised by the Bridge contract.
type BridgeDepositParametersUpdatedIterator struct {
	Event *BridgeDepositParametersUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeDepositParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeDepositParametersUpdated)
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
		it.Event = new(BridgeDepositParametersUpdated)
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
func (it *BridgeDepositParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeDepositParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeDepositParametersUpdated represents a DepositParametersUpdated event raised by the Bridge contract.
type BridgeDepositParametersUpdated struct {
	DepositDustThreshold      uint64
	DepositTreasuryFeeDivisor uint64
	DepositTxMaxFee           uint64
	DepositRevealAheadPeriod  uint32
	Raw                       types.Log // Blockchain specific contextual infos
}

// FilterDepositParametersUpdated is a free log retrieval operation binding the contract event 0x20b0edfa765e9ea8de6866efa2435850051a5dc3737d1aec22b0fdff8a3cbddf.
//
// Solidity: event DepositParametersUpdated(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod)
func (_Bridge *BridgeFilterer) FilterDepositParametersUpdated(opts *bind.FilterOpts) (*BridgeDepositParametersUpdatedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "DepositParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &BridgeDepositParametersUpdatedIterator{contract: _Bridge.contract, event: "DepositParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchDepositParametersUpdated is a free log subscription operation binding the contract event 0x20b0edfa765e9ea8de6866efa2435850051a5dc3737d1aec22b0fdff8a3cbddf.
//
// Solidity: event DepositParametersUpdated(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod)
func (_Bridge *BridgeFilterer) WatchDepositParametersUpdated(opts *bind.WatchOpts, sink chan<- *BridgeDepositParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "DepositParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeDepositParametersUpdated)
				if err := _Bridge.contract.UnpackLog(event, "DepositParametersUpdated", log); err != nil {
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

// ParseDepositParametersUpdated is a log parse operation binding the contract event 0x20b0edfa765e9ea8de6866efa2435850051a5dc3737d1aec22b0fdff8a3cbddf.
//
// Solidity: event DepositParametersUpdated(uint64 depositDustThreshold, uint64 depositTreasuryFeeDivisor, uint64 depositTxMaxFee, uint32 depositRevealAheadPeriod)
func (_Bridge *BridgeFilterer) ParseDepositParametersUpdated(log types.Log) (*BridgeDepositParametersUpdated, error) {
	event := new(BridgeDepositParametersUpdated)
	if err := _Bridge.contract.UnpackLog(event, "DepositParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeDepositRevealedIterator is returned from FilterDepositRevealed and is used to iterate over the raw logs and unpacked data for DepositRevealed events raised by the Bridge contract.
type BridgeDepositRevealedIterator struct {
	Event *BridgeDepositRevealed // Event containing the contract specifics and raw log

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
func (it *BridgeDepositRevealedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeDepositRevealed)
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
		it.Event = new(BridgeDepositRevealed)
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
func (it *BridgeDepositRevealedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeDepositRevealedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeDepositRevealed represents a DepositRevealed event raised by the Bridge contract.
type BridgeDepositRevealed struct {
	FundingTxHash      [32]byte
	FundingOutputIndex uint32
	Depositor          common.Address
	Amount             uint64
	BlindingFactor     [8]byte
	WalletPubKeyHash   [20]byte
	RefundPubKeyHash   [20]byte
	RefundLocktime     [4]byte
	Vault              common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterDepositRevealed is a free log retrieval operation binding the contract event 0xa7382159a693ed317a024daf0fd1ba30805cdf9928ee09550af517c516e2ef05.
//
// Solidity: event DepositRevealed(bytes32 fundingTxHash, uint32 fundingOutputIndex, address indexed depositor, uint64 amount, bytes8 blindingFactor, bytes20 indexed walletPubKeyHash, bytes20 refundPubKeyHash, bytes4 refundLocktime, address vault)
func (_Bridge *BridgeFilterer) FilterDepositRevealed(opts *bind.FilterOpts, depositor []common.Address, walletPubKeyHash [][20]byte) (*BridgeDepositRevealedIterator, error) {

	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "DepositRevealed", depositorRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeDepositRevealedIterator{contract: _Bridge.contract, event: "DepositRevealed", logs: logs, sub: sub}, nil
}

// WatchDepositRevealed is a free log subscription operation binding the contract event 0xa7382159a693ed317a024daf0fd1ba30805cdf9928ee09550af517c516e2ef05.
//
// Solidity: event DepositRevealed(bytes32 fundingTxHash, uint32 fundingOutputIndex, address indexed depositor, uint64 amount, bytes8 blindingFactor, bytes20 indexed walletPubKeyHash, bytes20 refundPubKeyHash, bytes4 refundLocktime, address vault)
func (_Bridge *BridgeFilterer) WatchDepositRevealed(opts *bind.WatchOpts, sink chan<- *BridgeDepositRevealed, depositor []common.Address, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "DepositRevealed", depositorRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeDepositRevealed)
				if err := _Bridge.contract.UnpackLog(event, "DepositRevealed", log); err != nil {
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

// ParseDepositRevealed is a log parse operation binding the contract event 0xa7382159a693ed317a024daf0fd1ba30805cdf9928ee09550af517c516e2ef05.
//
// Solidity: event DepositRevealed(bytes32 fundingTxHash, uint32 fundingOutputIndex, address indexed depositor, uint64 amount, bytes8 blindingFactor, bytes20 indexed walletPubKeyHash, bytes20 refundPubKeyHash, bytes4 refundLocktime, address vault)
func (_Bridge *BridgeFilterer) ParseDepositRevealed(log types.Log) (*BridgeDepositRevealed, error) {
	event := new(BridgeDepositRevealed)
	if err := _Bridge.contract.UnpackLog(event, "DepositRevealed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeDepositVaultFixedIterator is returned from FilterDepositVaultFixed and is used to iterate over the raw logs and unpacked data for DepositVaultFixed events raised by the Bridge contract.
type BridgeDepositVaultFixedIterator struct {
	Event *BridgeDepositVaultFixed // Event containing the contract specifics and raw log

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
func (it *BridgeDepositVaultFixedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeDepositVaultFixed)
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
		it.Event = new(BridgeDepositVaultFixed)
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
func (it *BridgeDepositVaultFixedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeDepositVaultFixedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeDepositVaultFixed represents a DepositVaultFixed event raised by the Bridge contract.
type BridgeDepositVaultFixed struct {
	DepositKey *big.Int
	NewVault   common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDepositVaultFixed is a free log retrieval operation binding the contract event 0x6851c9da8832e374b52353e89727e1f35bd403bf45bc19c889e416393bd53973.
//
// Solidity: event DepositVaultFixed(uint256 indexed depositKey, address newVault)
func (_Bridge *BridgeFilterer) FilterDepositVaultFixed(opts *bind.FilterOpts, depositKey []*big.Int) (*BridgeDepositVaultFixedIterator, error) {

	var depositKeyRule []interface{}
	for _, depositKeyItem := range depositKey {
		depositKeyRule = append(depositKeyRule, depositKeyItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "DepositVaultFixed", depositKeyRule)
	if err != nil {
		return nil, err
	}
	return &BridgeDepositVaultFixedIterator{contract: _Bridge.contract, event: "DepositVaultFixed", logs: logs, sub: sub}, nil
}

// WatchDepositVaultFixed is a free log subscription operation binding the contract event 0x6851c9da8832e374b52353e89727e1f35bd403bf45bc19c889e416393bd53973.
//
// Solidity: event DepositVaultFixed(uint256 indexed depositKey, address newVault)
func (_Bridge *BridgeFilterer) WatchDepositVaultFixed(opts *bind.WatchOpts, sink chan<- *BridgeDepositVaultFixed, depositKey []*big.Int) (event.Subscription, error) {

	var depositKeyRule []interface{}
	for _, depositKeyItem := range depositKey {
		depositKeyRule = append(depositKeyRule, depositKeyItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "DepositVaultFixed", depositKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeDepositVaultFixed)
				if err := _Bridge.contract.UnpackLog(event, "DepositVaultFixed", log); err != nil {
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

// ParseDepositVaultFixed is a log parse operation binding the contract event 0x6851c9da8832e374b52353e89727e1f35bd403bf45bc19c889e416393bd53973.
//
// Solidity: event DepositVaultFixed(uint256 indexed depositKey, address newVault)
func (_Bridge *BridgeFilterer) ParseDepositVaultFixed(log types.Log) (*BridgeDepositVaultFixed, error) {
	event := new(BridgeDepositVaultFixed)
	if err := _Bridge.contract.UnpackLog(event, "DepositVaultFixed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeDepositsSweptIterator is returned from FilterDepositsSwept and is used to iterate over the raw logs and unpacked data for DepositsSwept events raised by the Bridge contract.
type BridgeDepositsSweptIterator struct {
	Event *BridgeDepositsSwept // Event containing the contract specifics and raw log

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
func (it *BridgeDepositsSweptIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeDepositsSwept)
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
		it.Event = new(BridgeDepositsSwept)
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
func (it *BridgeDepositsSweptIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeDepositsSweptIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeDepositsSwept represents a DepositsSwept event raised by the Bridge contract.
type BridgeDepositsSwept struct {
	WalletPubKeyHash [20]byte
	SweepTxHash      [32]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterDepositsSwept is a free log retrieval operation binding the contract event 0xe50ffdcc0a5f2c1ede5c122b9414ffd7b2c6bc870d2d775194049dc30da95e6a.
//
// Solidity: event DepositsSwept(bytes20 walletPubKeyHash, bytes32 sweepTxHash)
func (_Bridge *BridgeFilterer) FilterDepositsSwept(opts *bind.FilterOpts) (*BridgeDepositsSweptIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "DepositsSwept")
	if err != nil {
		return nil, err
	}
	return &BridgeDepositsSweptIterator{contract: _Bridge.contract, event: "DepositsSwept", logs: logs, sub: sub}, nil
}

// WatchDepositsSwept is a free log subscription operation binding the contract event 0xe50ffdcc0a5f2c1ede5c122b9414ffd7b2c6bc870d2d775194049dc30da95e6a.
//
// Solidity: event DepositsSwept(bytes20 walletPubKeyHash, bytes32 sweepTxHash)
func (_Bridge *BridgeFilterer) WatchDepositsSwept(opts *bind.WatchOpts, sink chan<- *BridgeDepositsSwept) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "DepositsSwept")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeDepositsSwept)
				if err := _Bridge.contract.UnpackLog(event, "DepositsSwept", log); err != nil {
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

// ParseDepositsSwept is a log parse operation binding the contract event 0xe50ffdcc0a5f2c1ede5c122b9414ffd7b2c6bc870d2d775194049dc30da95e6a.
//
// Solidity: event DepositsSwept(bytes20 walletPubKeyHash, bytes32 sweepTxHash)
func (_Bridge *BridgeFilterer) ParseDepositsSwept(log types.Log) (*BridgeDepositsSwept, error) {
	event := new(BridgeDepositsSwept)
	if err := _Bridge.contract.UnpackLog(event, "DepositsSwept", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeFraudChallengeDefeatTimedOutIterator is returned from FilterFraudChallengeDefeatTimedOut and is used to iterate over the raw logs and unpacked data for FraudChallengeDefeatTimedOut events raised by the Bridge contract.
type BridgeFraudChallengeDefeatTimedOutIterator struct {
	Event *BridgeFraudChallengeDefeatTimedOut // Event containing the contract specifics and raw log

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
func (it *BridgeFraudChallengeDefeatTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeFraudChallengeDefeatTimedOut)
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
		it.Event = new(BridgeFraudChallengeDefeatTimedOut)
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
func (it *BridgeFraudChallengeDefeatTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeFraudChallengeDefeatTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeFraudChallengeDefeatTimedOut represents a FraudChallengeDefeatTimedOut event raised by the Bridge contract.
type BridgeFraudChallengeDefeatTimedOut struct {
	WalletPubKeyHash [20]byte
	Sighash          [32]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterFraudChallengeDefeatTimedOut is a free log retrieval operation binding the contract event 0x635230b60143449f10a365568e2bd95e3e8aaed03855631722941c2bad634b77.
//
// Solidity: event FraudChallengeDefeatTimedOut(bytes20 indexed walletPubKeyHash, bytes32 sighash)
func (_Bridge *BridgeFilterer) FilterFraudChallengeDefeatTimedOut(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeFraudChallengeDefeatTimedOutIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "FraudChallengeDefeatTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeFraudChallengeDefeatTimedOutIterator{contract: _Bridge.contract, event: "FraudChallengeDefeatTimedOut", logs: logs, sub: sub}, nil
}

// WatchFraudChallengeDefeatTimedOut is a free log subscription operation binding the contract event 0x635230b60143449f10a365568e2bd95e3e8aaed03855631722941c2bad634b77.
//
// Solidity: event FraudChallengeDefeatTimedOut(bytes20 indexed walletPubKeyHash, bytes32 sighash)
func (_Bridge *BridgeFilterer) WatchFraudChallengeDefeatTimedOut(opts *bind.WatchOpts, sink chan<- *BridgeFraudChallengeDefeatTimedOut, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "FraudChallengeDefeatTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeFraudChallengeDefeatTimedOut)
				if err := _Bridge.contract.UnpackLog(event, "FraudChallengeDefeatTimedOut", log); err != nil {
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

// ParseFraudChallengeDefeatTimedOut is a log parse operation binding the contract event 0x635230b60143449f10a365568e2bd95e3e8aaed03855631722941c2bad634b77.
//
// Solidity: event FraudChallengeDefeatTimedOut(bytes20 indexed walletPubKeyHash, bytes32 sighash)
func (_Bridge *BridgeFilterer) ParseFraudChallengeDefeatTimedOut(log types.Log) (*BridgeFraudChallengeDefeatTimedOut, error) {
	event := new(BridgeFraudChallengeDefeatTimedOut)
	if err := _Bridge.contract.UnpackLog(event, "FraudChallengeDefeatTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeFraudChallengeDefeatedIterator is returned from FilterFraudChallengeDefeated and is used to iterate over the raw logs and unpacked data for FraudChallengeDefeated events raised by the Bridge contract.
type BridgeFraudChallengeDefeatedIterator struct {
	Event *BridgeFraudChallengeDefeated // Event containing the contract specifics and raw log

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
func (it *BridgeFraudChallengeDefeatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeFraudChallengeDefeated)
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
		it.Event = new(BridgeFraudChallengeDefeated)
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
func (it *BridgeFraudChallengeDefeatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeFraudChallengeDefeatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeFraudChallengeDefeated represents a FraudChallengeDefeated event raised by the Bridge contract.
type BridgeFraudChallengeDefeated struct {
	WalletPubKeyHash [20]byte
	Sighash          [32]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterFraudChallengeDefeated is a free log retrieval operation binding the contract event 0x6ff720470ffad78f316655e2c7fc77a76763c13de0e19ee52149916ba7e44d3b.
//
// Solidity: event FraudChallengeDefeated(bytes20 indexed walletPubKeyHash, bytes32 sighash)
func (_Bridge *BridgeFilterer) FilterFraudChallengeDefeated(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeFraudChallengeDefeatedIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "FraudChallengeDefeated", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeFraudChallengeDefeatedIterator{contract: _Bridge.contract, event: "FraudChallengeDefeated", logs: logs, sub: sub}, nil
}

// WatchFraudChallengeDefeated is a free log subscription operation binding the contract event 0x6ff720470ffad78f316655e2c7fc77a76763c13de0e19ee52149916ba7e44d3b.
//
// Solidity: event FraudChallengeDefeated(bytes20 indexed walletPubKeyHash, bytes32 sighash)
func (_Bridge *BridgeFilterer) WatchFraudChallengeDefeated(opts *bind.WatchOpts, sink chan<- *BridgeFraudChallengeDefeated, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "FraudChallengeDefeated", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeFraudChallengeDefeated)
				if err := _Bridge.contract.UnpackLog(event, "FraudChallengeDefeated", log); err != nil {
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

// ParseFraudChallengeDefeated is a log parse operation binding the contract event 0x6ff720470ffad78f316655e2c7fc77a76763c13de0e19ee52149916ba7e44d3b.
//
// Solidity: event FraudChallengeDefeated(bytes20 indexed walletPubKeyHash, bytes32 sighash)
func (_Bridge *BridgeFilterer) ParseFraudChallengeDefeated(log types.Log) (*BridgeFraudChallengeDefeated, error) {
	event := new(BridgeFraudChallengeDefeated)
	if err := _Bridge.contract.UnpackLog(event, "FraudChallengeDefeated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeFraudChallengeSubmittedIterator is returned from FilterFraudChallengeSubmitted and is used to iterate over the raw logs and unpacked data for FraudChallengeSubmitted events raised by the Bridge contract.
type BridgeFraudChallengeSubmittedIterator struct {
	Event *BridgeFraudChallengeSubmitted // Event containing the contract specifics and raw log

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
func (it *BridgeFraudChallengeSubmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeFraudChallengeSubmitted)
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
		it.Event = new(BridgeFraudChallengeSubmitted)
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
func (it *BridgeFraudChallengeSubmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeFraudChallengeSubmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeFraudChallengeSubmitted represents a FraudChallengeSubmitted event raised by the Bridge contract.
type BridgeFraudChallengeSubmitted struct {
	WalletPubKeyHash [20]byte
	Sighash          [32]byte
	V                uint8
	R                [32]byte
	S                [32]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterFraudChallengeSubmitted is a free log retrieval operation binding the contract event 0xf4aa58d09ba5de017eac597806dfcfc2cad287816cb1eb7729a032c82680c94d.
//
// Solidity: event FraudChallengeSubmitted(bytes20 indexed walletPubKeyHash, bytes32 sighash, uint8 v, bytes32 r, bytes32 s)
func (_Bridge *BridgeFilterer) FilterFraudChallengeSubmitted(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeFraudChallengeSubmittedIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "FraudChallengeSubmitted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeFraudChallengeSubmittedIterator{contract: _Bridge.contract, event: "FraudChallengeSubmitted", logs: logs, sub: sub}, nil
}

// WatchFraudChallengeSubmitted is a free log subscription operation binding the contract event 0xf4aa58d09ba5de017eac597806dfcfc2cad287816cb1eb7729a032c82680c94d.
//
// Solidity: event FraudChallengeSubmitted(bytes20 indexed walletPubKeyHash, bytes32 sighash, uint8 v, bytes32 r, bytes32 s)
func (_Bridge *BridgeFilterer) WatchFraudChallengeSubmitted(opts *bind.WatchOpts, sink chan<- *BridgeFraudChallengeSubmitted, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "FraudChallengeSubmitted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeFraudChallengeSubmitted)
				if err := _Bridge.contract.UnpackLog(event, "FraudChallengeSubmitted", log); err != nil {
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

// ParseFraudChallengeSubmitted is a log parse operation binding the contract event 0xf4aa58d09ba5de017eac597806dfcfc2cad287816cb1eb7729a032c82680c94d.
//
// Solidity: event FraudChallengeSubmitted(bytes20 indexed walletPubKeyHash, bytes32 sighash, uint8 v, bytes32 r, bytes32 s)
func (_Bridge *BridgeFilterer) ParseFraudChallengeSubmitted(log types.Log) (*BridgeFraudChallengeSubmitted, error) {
	event := new(BridgeFraudChallengeSubmitted)
	if err := _Bridge.contract.UnpackLog(event, "FraudChallengeSubmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeFraudParametersUpdatedIterator is returned from FilterFraudParametersUpdated and is used to iterate over the raw logs and unpacked data for FraudParametersUpdated events raised by the Bridge contract.
type BridgeFraudParametersUpdatedIterator struct {
	Event *BridgeFraudParametersUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeFraudParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeFraudParametersUpdated)
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
		it.Event = new(BridgeFraudParametersUpdated)
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
func (it *BridgeFraudParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeFraudParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeFraudParametersUpdated represents a FraudParametersUpdated event raised by the Bridge contract.
type BridgeFraudParametersUpdated struct {
	FraudChallengeDepositAmount   *big.Int
	FraudChallengeDefeatTimeout   uint32
	FraudSlashingAmount           *big.Int
	FraudNotifierRewardMultiplier uint32
	Raw                           types.Log // Blockchain specific contextual infos
}

// FilterFraudParametersUpdated is a free log retrieval operation binding the contract event 0xc6d044ae75b875a43eb23bedc79d2b694b00ed95b5b8bf2a657328af9dda090d.
//
// Solidity: event FraudParametersUpdated(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) FilterFraudParametersUpdated(opts *bind.FilterOpts) (*BridgeFraudParametersUpdatedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "FraudParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &BridgeFraudParametersUpdatedIterator{contract: _Bridge.contract, event: "FraudParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchFraudParametersUpdated is a free log subscription operation binding the contract event 0xc6d044ae75b875a43eb23bedc79d2b694b00ed95b5b8bf2a657328af9dda090d.
//
// Solidity: event FraudParametersUpdated(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) WatchFraudParametersUpdated(opts *bind.WatchOpts, sink chan<- *BridgeFraudParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "FraudParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeFraudParametersUpdated)
				if err := _Bridge.contract.UnpackLog(event, "FraudParametersUpdated", log); err != nil {
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

// ParseFraudParametersUpdated is a log parse operation binding the contract event 0xc6d044ae75b875a43eb23bedc79d2b694b00ed95b5b8bf2a657328af9dda090d.
//
// Solidity: event FraudParametersUpdated(uint96 fraudChallengeDepositAmount, uint32 fraudChallengeDefeatTimeout, uint96 fraudSlashingAmount, uint32 fraudNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) ParseFraudParametersUpdated(log types.Log) (*BridgeFraudParametersUpdated, error) {
	event := new(BridgeFraudParametersUpdated)
	if err := _Bridge.contract.UnpackLog(event, "FraudParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeGovernanceTransferredIterator is returned from FilterGovernanceTransferred and is used to iterate over the raw logs and unpacked data for GovernanceTransferred events raised by the Bridge contract.
type BridgeGovernanceTransferredIterator struct {
	Event *BridgeGovernanceTransferred // Event containing the contract specifics and raw log

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
func (it *BridgeGovernanceTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeGovernanceTransferred)
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
		it.Event = new(BridgeGovernanceTransferred)
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
func (it *BridgeGovernanceTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeGovernanceTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeGovernanceTransferred represents a GovernanceTransferred event raised by the Bridge contract.
type BridgeGovernanceTransferred struct {
	OldGovernance common.Address
	NewGovernance common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterGovernanceTransferred is a free log retrieval operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_Bridge *BridgeFilterer) FilterGovernanceTransferred(opts *bind.FilterOpts) (*BridgeGovernanceTransferredIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return &BridgeGovernanceTransferredIterator{contract: _Bridge.contract, event: "GovernanceTransferred", logs: logs, sub: sub}, nil
}

// WatchGovernanceTransferred is a free log subscription operation binding the contract event 0x5f56bee8cffbe9a78652a74a60705edede02af10b0bbb888ca44b79a0d42ce80.
//
// Solidity: event GovernanceTransferred(address oldGovernance, address newGovernance)
func (_Bridge *BridgeFilterer) WatchGovernanceTransferred(opts *bind.WatchOpts, sink chan<- *BridgeGovernanceTransferred) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "GovernanceTransferred")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeGovernanceTransferred)
				if err := _Bridge.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
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
func (_Bridge *BridgeFilterer) ParseGovernanceTransferred(log types.Log) (*BridgeGovernanceTransferred, error) {
	event := new(BridgeGovernanceTransferred)
	if err := _Bridge.contract.UnpackLog(event, "GovernanceTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the Bridge contract.
type BridgeInitializedIterator struct {
	Event *BridgeInitialized // Event containing the contract specifics and raw log

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
func (it *BridgeInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeInitialized)
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
		it.Event = new(BridgeInitialized)
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
func (it *BridgeInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeInitialized represents a Initialized event raised by the Bridge contract.
type BridgeInitialized struct {
	Version uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_Bridge *BridgeFilterer) FilterInitialized(opts *bind.FilterOpts) (*BridgeInitializedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &BridgeInitializedIterator{contract: _Bridge.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498.
//
// Solidity: event Initialized(uint8 version)
func (_Bridge *BridgeFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *BridgeInitialized) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeInitialized)
				if err := _Bridge.contract.UnpackLog(event, "Initialized", log); err != nil {
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
func (_Bridge *BridgeFilterer) ParseInitialized(log types.Log) (*BridgeInitialized, error) {
	event := new(BridgeInitialized)
	if err := _Bridge.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovedFundsSweepTimedOutIterator is returned from FilterMovedFundsSweepTimedOut and is used to iterate over the raw logs and unpacked data for MovedFundsSweepTimedOut events raised by the Bridge contract.
type BridgeMovedFundsSweepTimedOutIterator struct {
	Event *BridgeMovedFundsSweepTimedOut // Event containing the contract specifics and raw log

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
func (it *BridgeMovedFundsSweepTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovedFundsSweepTimedOut)
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
		it.Event = new(BridgeMovedFundsSweepTimedOut)
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
func (it *BridgeMovedFundsSweepTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovedFundsSweepTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovedFundsSweepTimedOut represents a MovedFundsSweepTimedOut event raised by the Bridge contract.
type BridgeMovedFundsSweepTimedOut struct {
	WalletPubKeyHash         [20]byte
	MovingFundsTxHash        [32]byte
	MovingFundsTxOutputIndex uint32
	Raw                      types.Log // Blockchain specific contextual infos
}

// FilterMovedFundsSweepTimedOut is a free log retrieval operation binding the contract event 0x4c25d874672bd8dcc921a53387892a0d5c26d5dae3d368ffe83f65cc99700612.
//
// Solidity: event MovedFundsSweepTimedOut(bytes20 indexed walletPubKeyHash, bytes32 movingFundsTxHash, uint32 movingFundsTxOutputIndex)
func (_Bridge *BridgeFilterer) FilterMovedFundsSweepTimedOut(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeMovedFundsSweepTimedOutIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovedFundsSweepTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeMovedFundsSweepTimedOutIterator{contract: _Bridge.contract, event: "MovedFundsSweepTimedOut", logs: logs, sub: sub}, nil
}

// WatchMovedFundsSweepTimedOut is a free log subscription operation binding the contract event 0x4c25d874672bd8dcc921a53387892a0d5c26d5dae3d368ffe83f65cc99700612.
//
// Solidity: event MovedFundsSweepTimedOut(bytes20 indexed walletPubKeyHash, bytes32 movingFundsTxHash, uint32 movingFundsTxOutputIndex)
func (_Bridge *BridgeFilterer) WatchMovedFundsSweepTimedOut(opts *bind.WatchOpts, sink chan<- *BridgeMovedFundsSweepTimedOut, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovedFundsSweepTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovedFundsSweepTimedOut)
				if err := _Bridge.contract.UnpackLog(event, "MovedFundsSweepTimedOut", log); err != nil {
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

// ParseMovedFundsSweepTimedOut is a log parse operation binding the contract event 0x4c25d874672bd8dcc921a53387892a0d5c26d5dae3d368ffe83f65cc99700612.
//
// Solidity: event MovedFundsSweepTimedOut(bytes20 indexed walletPubKeyHash, bytes32 movingFundsTxHash, uint32 movingFundsTxOutputIndex)
func (_Bridge *BridgeFilterer) ParseMovedFundsSweepTimedOut(log types.Log) (*BridgeMovedFundsSweepTimedOut, error) {
	event := new(BridgeMovedFundsSweepTimedOut)
	if err := _Bridge.contract.UnpackLog(event, "MovedFundsSweepTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovedFundsSweptIterator is returned from FilterMovedFundsSwept and is used to iterate over the raw logs and unpacked data for MovedFundsSwept events raised by the Bridge contract.
type BridgeMovedFundsSweptIterator struct {
	Event *BridgeMovedFundsSwept // Event containing the contract specifics and raw log

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
func (it *BridgeMovedFundsSweptIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovedFundsSwept)
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
		it.Event = new(BridgeMovedFundsSwept)
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
func (it *BridgeMovedFundsSweptIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovedFundsSweptIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovedFundsSwept represents a MovedFundsSwept event raised by the Bridge contract.
type BridgeMovedFundsSwept struct {
	WalletPubKeyHash [20]byte
	SweepTxHash      [32]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterMovedFundsSwept is a free log retrieval operation binding the contract event 0x1650566b0cdca3878702c6b169ab429b57de91a06c61998925d0adf6227b243e.
//
// Solidity: event MovedFundsSwept(bytes20 indexed walletPubKeyHash, bytes32 sweepTxHash)
func (_Bridge *BridgeFilterer) FilterMovedFundsSwept(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeMovedFundsSweptIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovedFundsSwept", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeMovedFundsSweptIterator{contract: _Bridge.contract, event: "MovedFundsSwept", logs: logs, sub: sub}, nil
}

// WatchMovedFundsSwept is a free log subscription operation binding the contract event 0x1650566b0cdca3878702c6b169ab429b57de91a06c61998925d0adf6227b243e.
//
// Solidity: event MovedFundsSwept(bytes20 indexed walletPubKeyHash, bytes32 sweepTxHash)
func (_Bridge *BridgeFilterer) WatchMovedFundsSwept(opts *bind.WatchOpts, sink chan<- *BridgeMovedFundsSwept, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovedFundsSwept", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovedFundsSwept)
				if err := _Bridge.contract.UnpackLog(event, "MovedFundsSwept", log); err != nil {
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

// ParseMovedFundsSwept is a log parse operation binding the contract event 0x1650566b0cdca3878702c6b169ab429b57de91a06c61998925d0adf6227b243e.
//
// Solidity: event MovedFundsSwept(bytes20 indexed walletPubKeyHash, bytes32 sweepTxHash)
func (_Bridge *BridgeFilterer) ParseMovedFundsSwept(log types.Log) (*BridgeMovedFundsSwept, error) {
	event := new(BridgeMovedFundsSwept)
	if err := _Bridge.contract.UnpackLog(event, "MovedFundsSwept", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovingFundsBelowDustReportedIterator is returned from FilterMovingFundsBelowDustReported and is used to iterate over the raw logs and unpacked data for MovingFundsBelowDustReported events raised by the Bridge contract.
type BridgeMovingFundsBelowDustReportedIterator struct {
	Event *BridgeMovingFundsBelowDustReported // Event containing the contract specifics and raw log

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
func (it *BridgeMovingFundsBelowDustReportedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovingFundsBelowDustReported)
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
		it.Event = new(BridgeMovingFundsBelowDustReported)
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
func (it *BridgeMovingFundsBelowDustReportedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovingFundsBelowDustReportedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovingFundsBelowDustReported represents a MovingFundsBelowDustReported event raised by the Bridge contract.
type BridgeMovingFundsBelowDustReported struct {
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterMovingFundsBelowDustReported is a free log retrieval operation binding the contract event 0xf54ae1d272063b0032d7dc7af48156e7a0c9711f404de48e494122d971b9dd16.
//
// Solidity: event MovingFundsBelowDustReported(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterMovingFundsBelowDustReported(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeMovingFundsBelowDustReportedIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovingFundsBelowDustReported", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeMovingFundsBelowDustReportedIterator{contract: _Bridge.contract, event: "MovingFundsBelowDustReported", logs: logs, sub: sub}, nil
}

// WatchMovingFundsBelowDustReported is a free log subscription operation binding the contract event 0xf54ae1d272063b0032d7dc7af48156e7a0c9711f404de48e494122d971b9dd16.
//
// Solidity: event MovingFundsBelowDustReported(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchMovingFundsBelowDustReported(opts *bind.WatchOpts, sink chan<- *BridgeMovingFundsBelowDustReported, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovingFundsBelowDustReported", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovingFundsBelowDustReported)
				if err := _Bridge.contract.UnpackLog(event, "MovingFundsBelowDustReported", log); err != nil {
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

// ParseMovingFundsBelowDustReported is a log parse operation binding the contract event 0xf54ae1d272063b0032d7dc7af48156e7a0c9711f404de48e494122d971b9dd16.
//
// Solidity: event MovingFundsBelowDustReported(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseMovingFundsBelowDustReported(log types.Log) (*BridgeMovingFundsBelowDustReported, error) {
	event := new(BridgeMovingFundsBelowDustReported)
	if err := _Bridge.contract.UnpackLog(event, "MovingFundsBelowDustReported", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovingFundsCommitmentSubmittedIterator is returned from FilterMovingFundsCommitmentSubmitted and is used to iterate over the raw logs and unpacked data for MovingFundsCommitmentSubmitted events raised by the Bridge contract.
type BridgeMovingFundsCommitmentSubmittedIterator struct {
	Event *BridgeMovingFundsCommitmentSubmitted // Event containing the contract specifics and raw log

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
func (it *BridgeMovingFundsCommitmentSubmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovingFundsCommitmentSubmitted)
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
		it.Event = new(BridgeMovingFundsCommitmentSubmitted)
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
func (it *BridgeMovingFundsCommitmentSubmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovingFundsCommitmentSubmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovingFundsCommitmentSubmitted represents a MovingFundsCommitmentSubmitted event raised by the Bridge contract.
type BridgeMovingFundsCommitmentSubmitted struct {
	WalletPubKeyHash [20]byte
	TargetWallets    [][20]byte
	Submitter        common.Address
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterMovingFundsCommitmentSubmitted is a free log retrieval operation binding the contract event 0xfcab32b1dc365ef82ae9df4be019512ca536c191bd68f523b9f33d0c070e8d4a.
//
// Solidity: event MovingFundsCommitmentSubmitted(bytes20 indexed walletPubKeyHash, bytes20[] targetWallets, address submitter)
func (_Bridge *BridgeFilterer) FilterMovingFundsCommitmentSubmitted(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeMovingFundsCommitmentSubmittedIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovingFundsCommitmentSubmitted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeMovingFundsCommitmentSubmittedIterator{contract: _Bridge.contract, event: "MovingFundsCommitmentSubmitted", logs: logs, sub: sub}, nil
}

// WatchMovingFundsCommitmentSubmitted is a free log subscription operation binding the contract event 0xfcab32b1dc365ef82ae9df4be019512ca536c191bd68f523b9f33d0c070e8d4a.
//
// Solidity: event MovingFundsCommitmentSubmitted(bytes20 indexed walletPubKeyHash, bytes20[] targetWallets, address submitter)
func (_Bridge *BridgeFilterer) WatchMovingFundsCommitmentSubmitted(opts *bind.WatchOpts, sink chan<- *BridgeMovingFundsCommitmentSubmitted, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovingFundsCommitmentSubmitted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovingFundsCommitmentSubmitted)
				if err := _Bridge.contract.UnpackLog(event, "MovingFundsCommitmentSubmitted", log); err != nil {
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

// ParseMovingFundsCommitmentSubmitted is a log parse operation binding the contract event 0xfcab32b1dc365ef82ae9df4be019512ca536c191bd68f523b9f33d0c070e8d4a.
//
// Solidity: event MovingFundsCommitmentSubmitted(bytes20 indexed walletPubKeyHash, bytes20[] targetWallets, address submitter)
func (_Bridge *BridgeFilterer) ParseMovingFundsCommitmentSubmitted(log types.Log) (*BridgeMovingFundsCommitmentSubmitted, error) {
	event := new(BridgeMovingFundsCommitmentSubmitted)
	if err := _Bridge.contract.UnpackLog(event, "MovingFundsCommitmentSubmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovingFundsCompletedIterator is returned from FilterMovingFundsCompleted and is used to iterate over the raw logs and unpacked data for MovingFundsCompleted events raised by the Bridge contract.
type BridgeMovingFundsCompletedIterator struct {
	Event *BridgeMovingFundsCompleted // Event containing the contract specifics and raw log

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
func (it *BridgeMovingFundsCompletedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovingFundsCompleted)
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
		it.Event = new(BridgeMovingFundsCompleted)
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
func (it *BridgeMovingFundsCompletedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovingFundsCompletedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovingFundsCompleted represents a MovingFundsCompleted event raised by the Bridge contract.
type BridgeMovingFundsCompleted struct {
	WalletPubKeyHash  [20]byte
	MovingFundsTxHash [32]byte
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterMovingFundsCompleted is a free log retrieval operation binding the contract event 0xc635af1892551655b9dbb3256a0eed3e35baf4fcc5392b80e6a907b6f44a2838.
//
// Solidity: event MovingFundsCompleted(bytes20 indexed walletPubKeyHash, bytes32 movingFundsTxHash)
func (_Bridge *BridgeFilterer) FilterMovingFundsCompleted(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeMovingFundsCompletedIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovingFundsCompleted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeMovingFundsCompletedIterator{contract: _Bridge.contract, event: "MovingFundsCompleted", logs: logs, sub: sub}, nil
}

// WatchMovingFundsCompleted is a free log subscription operation binding the contract event 0xc635af1892551655b9dbb3256a0eed3e35baf4fcc5392b80e6a907b6f44a2838.
//
// Solidity: event MovingFundsCompleted(bytes20 indexed walletPubKeyHash, bytes32 movingFundsTxHash)
func (_Bridge *BridgeFilterer) WatchMovingFundsCompleted(opts *bind.WatchOpts, sink chan<- *BridgeMovingFundsCompleted, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovingFundsCompleted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovingFundsCompleted)
				if err := _Bridge.contract.UnpackLog(event, "MovingFundsCompleted", log); err != nil {
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

// ParseMovingFundsCompleted is a log parse operation binding the contract event 0xc635af1892551655b9dbb3256a0eed3e35baf4fcc5392b80e6a907b6f44a2838.
//
// Solidity: event MovingFundsCompleted(bytes20 indexed walletPubKeyHash, bytes32 movingFundsTxHash)
func (_Bridge *BridgeFilterer) ParseMovingFundsCompleted(log types.Log) (*BridgeMovingFundsCompleted, error) {
	event := new(BridgeMovingFundsCompleted)
	if err := _Bridge.contract.UnpackLog(event, "MovingFundsCompleted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovingFundsParametersUpdatedIterator is returned from FilterMovingFundsParametersUpdated and is used to iterate over the raw logs and unpacked data for MovingFundsParametersUpdated events raised by the Bridge contract.
type BridgeMovingFundsParametersUpdatedIterator struct {
	Event *BridgeMovingFundsParametersUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeMovingFundsParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovingFundsParametersUpdated)
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
		it.Event = new(BridgeMovingFundsParametersUpdated)
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
func (it *BridgeMovingFundsParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovingFundsParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovingFundsParametersUpdated represents a MovingFundsParametersUpdated event raised by the Bridge contract.
type BridgeMovingFundsParametersUpdated struct {
	MovingFundsTxMaxTotalFee                       uint64
	MovingFundsDustThreshold                       uint64
	MovingFundsTimeoutResetDelay                   uint32
	MovingFundsTimeout                             uint32
	MovingFundsTimeoutSlashingAmount               *big.Int
	MovingFundsTimeoutNotifierRewardMultiplier     uint32
	MovingFundsCommitmentGasOffset                 uint16
	MovedFundsSweepTxMaxTotalFee                   uint64
	MovedFundsSweepTimeout                         uint32
	MovedFundsSweepTimeoutSlashingAmount           *big.Int
	MovedFundsSweepTimeoutNotifierRewardMultiplier uint32
	Raw                                            types.Log // Blockchain specific contextual infos
}

// FilterMovingFundsParametersUpdated is a free log retrieval operation binding the contract event 0xdd096c0b696aec16e055260a26ba0980984d19c11cd2ae6f6dbd0b67e0c3bf0d.
//
// Solidity: event MovingFundsParametersUpdated(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) FilterMovingFundsParametersUpdated(opts *bind.FilterOpts) (*BridgeMovingFundsParametersUpdatedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovingFundsParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &BridgeMovingFundsParametersUpdatedIterator{contract: _Bridge.contract, event: "MovingFundsParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchMovingFundsParametersUpdated is a free log subscription operation binding the contract event 0xdd096c0b696aec16e055260a26ba0980984d19c11cd2ae6f6dbd0b67e0c3bf0d.
//
// Solidity: event MovingFundsParametersUpdated(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) WatchMovingFundsParametersUpdated(opts *bind.WatchOpts, sink chan<- *BridgeMovingFundsParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovingFundsParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovingFundsParametersUpdated)
				if err := _Bridge.contract.UnpackLog(event, "MovingFundsParametersUpdated", log); err != nil {
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

// ParseMovingFundsParametersUpdated is a log parse operation binding the contract event 0xdd096c0b696aec16e055260a26ba0980984d19c11cd2ae6f6dbd0b67e0c3bf0d.
//
// Solidity: event MovingFundsParametersUpdated(uint64 movingFundsTxMaxTotalFee, uint64 movingFundsDustThreshold, uint32 movingFundsTimeoutResetDelay, uint32 movingFundsTimeout, uint96 movingFundsTimeoutSlashingAmount, uint32 movingFundsTimeoutNotifierRewardMultiplier, uint16 movingFundsCommitmentGasOffset, uint64 movedFundsSweepTxMaxTotalFee, uint32 movedFundsSweepTimeout, uint96 movedFundsSweepTimeoutSlashingAmount, uint32 movedFundsSweepTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) ParseMovingFundsParametersUpdated(log types.Log) (*BridgeMovingFundsParametersUpdated, error) {
	event := new(BridgeMovingFundsParametersUpdated)
	if err := _Bridge.contract.UnpackLog(event, "MovingFundsParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovingFundsTimedOutIterator is returned from FilterMovingFundsTimedOut and is used to iterate over the raw logs and unpacked data for MovingFundsTimedOut events raised by the Bridge contract.
type BridgeMovingFundsTimedOutIterator struct {
	Event *BridgeMovingFundsTimedOut // Event containing the contract specifics and raw log

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
func (it *BridgeMovingFundsTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovingFundsTimedOut)
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
		it.Event = new(BridgeMovingFundsTimedOut)
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
func (it *BridgeMovingFundsTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovingFundsTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovingFundsTimedOut represents a MovingFundsTimedOut event raised by the Bridge contract.
type BridgeMovingFundsTimedOut struct {
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterMovingFundsTimedOut is a free log retrieval operation binding the contract event 0x5862a5a7095622ec6e3a04e5bb6547f1e4034af0d7d4d7e9787678072fc66fb2.
//
// Solidity: event MovingFundsTimedOut(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterMovingFundsTimedOut(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeMovingFundsTimedOutIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovingFundsTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeMovingFundsTimedOutIterator{contract: _Bridge.contract, event: "MovingFundsTimedOut", logs: logs, sub: sub}, nil
}

// WatchMovingFundsTimedOut is a free log subscription operation binding the contract event 0x5862a5a7095622ec6e3a04e5bb6547f1e4034af0d7d4d7e9787678072fc66fb2.
//
// Solidity: event MovingFundsTimedOut(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchMovingFundsTimedOut(opts *bind.WatchOpts, sink chan<- *BridgeMovingFundsTimedOut, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovingFundsTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovingFundsTimedOut)
				if err := _Bridge.contract.UnpackLog(event, "MovingFundsTimedOut", log); err != nil {
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

// ParseMovingFundsTimedOut is a log parse operation binding the contract event 0x5862a5a7095622ec6e3a04e5bb6547f1e4034af0d7d4d7e9787678072fc66fb2.
//
// Solidity: event MovingFundsTimedOut(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseMovingFundsTimedOut(log types.Log) (*BridgeMovingFundsTimedOut, error) {
	event := new(BridgeMovingFundsTimedOut)
	if err := _Bridge.contract.UnpackLog(event, "MovingFundsTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeMovingFundsTimeoutResetIterator is returned from FilterMovingFundsTimeoutReset and is used to iterate over the raw logs and unpacked data for MovingFundsTimeoutReset events raised by the Bridge contract.
type BridgeMovingFundsTimeoutResetIterator struct {
	Event *BridgeMovingFundsTimeoutReset // Event containing the contract specifics and raw log

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
func (it *BridgeMovingFundsTimeoutResetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeMovingFundsTimeoutReset)
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
		it.Event = new(BridgeMovingFundsTimeoutReset)
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
func (it *BridgeMovingFundsTimeoutResetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeMovingFundsTimeoutResetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeMovingFundsTimeoutReset represents a MovingFundsTimeoutReset event raised by the Bridge contract.
type BridgeMovingFundsTimeoutReset struct {
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterMovingFundsTimeoutReset is a free log retrieval operation binding the contract event 0xa59c6e2153c28ecd6a3507cfa44c3ab392779ff484a1aee02e40345c63a59bc0.
//
// Solidity: event MovingFundsTimeoutReset(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterMovingFundsTimeoutReset(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeMovingFundsTimeoutResetIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "MovingFundsTimeoutReset", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeMovingFundsTimeoutResetIterator{contract: _Bridge.contract, event: "MovingFundsTimeoutReset", logs: logs, sub: sub}, nil
}

// WatchMovingFundsTimeoutReset is a free log subscription operation binding the contract event 0xa59c6e2153c28ecd6a3507cfa44c3ab392779ff484a1aee02e40345c63a59bc0.
//
// Solidity: event MovingFundsTimeoutReset(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchMovingFundsTimeoutReset(opts *bind.WatchOpts, sink chan<- *BridgeMovingFundsTimeoutReset, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "MovingFundsTimeoutReset", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeMovingFundsTimeoutReset)
				if err := _Bridge.contract.UnpackLog(event, "MovingFundsTimeoutReset", log); err != nil {
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

// ParseMovingFundsTimeoutReset is a log parse operation binding the contract event 0xa59c6e2153c28ecd6a3507cfa44c3ab392779ff484a1aee02e40345c63a59bc0.
//
// Solidity: event MovingFundsTimeoutReset(bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseMovingFundsTimeoutReset(log types.Log) (*BridgeMovingFundsTimeoutReset, error) {
	event := new(BridgeMovingFundsTimeoutReset)
	if err := _Bridge.contract.UnpackLog(event, "MovingFundsTimeoutReset", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeNewWalletRegisteredIterator is returned from FilterNewWalletRegistered and is used to iterate over the raw logs and unpacked data for NewWalletRegistered events raised by the Bridge contract.
type BridgeNewWalletRegisteredIterator struct {
	Event *BridgeNewWalletRegistered // Event containing the contract specifics and raw log

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
func (it *BridgeNewWalletRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeNewWalletRegistered)
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
		it.Event = new(BridgeNewWalletRegistered)
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
func (it *BridgeNewWalletRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeNewWalletRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeNewWalletRegistered represents a NewWalletRegistered event raised by the Bridge contract.
type BridgeNewWalletRegistered struct {
	EcdsaWalletID    [32]byte
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterNewWalletRegistered is a free log retrieval operation binding the contract event 0x2dbb47dce81d6b11cca1f1e3b10143d6f7e1e7e92d2dd9aacbb1f875379d308e.
//
// Solidity: event NewWalletRegistered(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterNewWalletRegistered(opts *bind.FilterOpts, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (*BridgeNewWalletRegisteredIterator, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "NewWalletRegistered", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeNewWalletRegisteredIterator{contract: _Bridge.contract, event: "NewWalletRegistered", logs: logs, sub: sub}, nil
}

// WatchNewWalletRegistered is a free log subscription operation binding the contract event 0x2dbb47dce81d6b11cca1f1e3b10143d6f7e1e7e92d2dd9aacbb1f875379d308e.
//
// Solidity: event NewWalletRegistered(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchNewWalletRegistered(opts *bind.WatchOpts, sink chan<- *BridgeNewWalletRegistered, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "NewWalletRegistered", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeNewWalletRegistered)
				if err := _Bridge.contract.UnpackLog(event, "NewWalletRegistered", log); err != nil {
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

// ParseNewWalletRegistered is a log parse operation binding the contract event 0x2dbb47dce81d6b11cca1f1e3b10143d6f7e1e7e92d2dd9aacbb1f875379d308e.
//
// Solidity: event NewWalletRegistered(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseNewWalletRegistered(log types.Log) (*BridgeNewWalletRegistered, error) {
	event := new(BridgeNewWalletRegistered)
	if err := _Bridge.contract.UnpackLog(event, "NewWalletRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeNewWalletRequestedIterator is returned from FilterNewWalletRequested and is used to iterate over the raw logs and unpacked data for NewWalletRequested events raised by the Bridge contract.
type BridgeNewWalletRequestedIterator struct {
	Event *BridgeNewWalletRequested // Event containing the contract specifics and raw log

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
func (it *BridgeNewWalletRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeNewWalletRequested)
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
		it.Event = new(BridgeNewWalletRequested)
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
func (it *BridgeNewWalletRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeNewWalletRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeNewWalletRequested represents a NewWalletRequested event raised by the Bridge contract.
type BridgeNewWalletRequested struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterNewWalletRequested is a free log retrieval operation binding the contract event 0x31fecb80caf1e1128496dd5a6f1083ba29fd5fe64c3fe04e2d1b6f9cfc27d5a3.
//
// Solidity: event NewWalletRequested()
func (_Bridge *BridgeFilterer) FilterNewWalletRequested(opts *bind.FilterOpts) (*BridgeNewWalletRequestedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "NewWalletRequested")
	if err != nil {
		return nil, err
	}
	return &BridgeNewWalletRequestedIterator{contract: _Bridge.contract, event: "NewWalletRequested", logs: logs, sub: sub}, nil
}

// WatchNewWalletRequested is a free log subscription operation binding the contract event 0x31fecb80caf1e1128496dd5a6f1083ba29fd5fe64c3fe04e2d1b6f9cfc27d5a3.
//
// Solidity: event NewWalletRequested()
func (_Bridge *BridgeFilterer) WatchNewWalletRequested(opts *bind.WatchOpts, sink chan<- *BridgeNewWalletRequested) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "NewWalletRequested")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeNewWalletRequested)
				if err := _Bridge.contract.UnpackLog(event, "NewWalletRequested", log); err != nil {
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

// ParseNewWalletRequested is a log parse operation binding the contract event 0x31fecb80caf1e1128496dd5a6f1083ba29fd5fe64c3fe04e2d1b6f9cfc27d5a3.
//
// Solidity: event NewWalletRequested()
func (_Bridge *BridgeFilterer) ParseNewWalletRequested(log types.Log) (*BridgeNewWalletRequested, error) {
	event := new(BridgeNewWalletRequested)
	if err := _Bridge.contract.UnpackLog(event, "NewWalletRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeRebateStakingRepairedIterator is returned from FilterRebateStakingRepaired and is used to iterate over the raw logs and unpacked data for RebateStakingRepaired events raised by the Bridge contract.
type BridgeRebateStakingRepairedIterator struct {
	Event *BridgeRebateStakingRepaired // Event containing the contract specifics and raw log

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
func (it *BridgeRebateStakingRepairedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeRebateStakingRepaired)
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
		it.Event = new(BridgeRebateStakingRepaired)
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
func (it *BridgeRebateStakingRepairedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeRebateStakingRepairedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeRebateStakingRepaired represents a RebateStakingRepaired event raised by the Bridge contract.
type BridgeRebateStakingRepaired struct {
	OldRebateStaking common.Address
	NewRebateStaking common.Address
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterRebateStakingRepaired is a free log retrieval operation binding the contract event 0x3e9cf92a8b1e7b429a80cef0378c64004ed9e723a896f58a2bb02005bb34d8c5.
//
// Solidity: event RebateStakingRepaired(address oldRebateStaking, address newRebateStaking)
func (_Bridge *BridgeFilterer) FilterRebateStakingRepaired(opts *bind.FilterOpts) (*BridgeRebateStakingRepairedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "RebateStakingRepaired")
	if err != nil {
		return nil, err
	}
	return &BridgeRebateStakingRepairedIterator{contract: _Bridge.contract, event: "RebateStakingRepaired", logs: logs, sub: sub}, nil
}

// WatchRebateStakingRepaired is a free log subscription operation binding the contract event 0x3e9cf92a8b1e7b429a80cef0378c64004ed9e723a896f58a2bb02005bb34d8c5.
//
// Solidity: event RebateStakingRepaired(address oldRebateStaking, address newRebateStaking)
func (_Bridge *BridgeFilterer) WatchRebateStakingRepaired(opts *bind.WatchOpts, sink chan<- *BridgeRebateStakingRepaired) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "RebateStakingRepaired")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeRebateStakingRepaired)
				if err := _Bridge.contract.UnpackLog(event, "RebateStakingRepaired", log); err != nil {
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

// ParseRebateStakingRepaired is a log parse operation binding the contract event 0x3e9cf92a8b1e7b429a80cef0378c64004ed9e723a896f58a2bb02005bb34d8c5.
//
// Solidity: event RebateStakingRepaired(address oldRebateStaking, address newRebateStaking)
func (_Bridge *BridgeFilterer) ParseRebateStakingRepaired(log types.Log) (*BridgeRebateStakingRepaired, error) {
	event := new(BridgeRebateStakingRepaired)
	if err := _Bridge.contract.UnpackLog(event, "RebateStakingRepaired", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeRebateStakingSetIterator is returned from FilterRebateStakingSet and is used to iterate over the raw logs and unpacked data for RebateStakingSet events raised by the Bridge contract.
type BridgeRebateStakingSetIterator struct {
	Event *BridgeRebateStakingSet // Event containing the contract specifics and raw log

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
func (it *BridgeRebateStakingSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeRebateStakingSet)
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
		it.Event = new(BridgeRebateStakingSet)
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
func (it *BridgeRebateStakingSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeRebateStakingSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeRebateStakingSet represents a RebateStakingSet event raised by the Bridge contract.
type BridgeRebateStakingSet struct {
	RebateStaking common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterRebateStakingSet is a free log retrieval operation binding the contract event 0xd1d9d4e9f516cb983e81d2a124ec97cb8d4ff00637f2a7f3229eadbed84e2df6.
//
// Solidity: event RebateStakingSet(address rebateStaking)
func (_Bridge *BridgeFilterer) FilterRebateStakingSet(opts *bind.FilterOpts) (*BridgeRebateStakingSetIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "RebateStakingSet")
	if err != nil {
		return nil, err
	}
	return &BridgeRebateStakingSetIterator{contract: _Bridge.contract, event: "RebateStakingSet", logs: logs, sub: sub}, nil
}

// WatchRebateStakingSet is a free log subscription operation binding the contract event 0xd1d9d4e9f516cb983e81d2a124ec97cb8d4ff00637f2a7f3229eadbed84e2df6.
//
// Solidity: event RebateStakingSet(address rebateStaking)
func (_Bridge *BridgeFilterer) WatchRebateStakingSet(opts *bind.WatchOpts, sink chan<- *BridgeRebateStakingSet) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "RebateStakingSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeRebateStakingSet)
				if err := _Bridge.contract.UnpackLog(event, "RebateStakingSet", log); err != nil {
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

// ParseRebateStakingSet is a log parse operation binding the contract event 0xd1d9d4e9f516cb983e81d2a124ec97cb8d4ff00637f2a7f3229eadbed84e2df6.
//
// Solidity: event RebateStakingSet(address rebateStaking)
func (_Bridge *BridgeFilterer) ParseRebateStakingSet(log types.Log) (*BridgeRebateStakingSet, error) {
	event := new(BridgeRebateStakingSet)
	if err := _Bridge.contract.UnpackLog(event, "RebateStakingSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeRedemptionParametersUpdatedIterator is returned from FilterRedemptionParametersUpdated and is used to iterate over the raw logs and unpacked data for RedemptionParametersUpdated events raised by the Bridge contract.
type BridgeRedemptionParametersUpdatedIterator struct {
	Event *BridgeRedemptionParametersUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeRedemptionParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeRedemptionParametersUpdated)
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
		it.Event = new(BridgeRedemptionParametersUpdated)
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
func (it *BridgeRedemptionParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeRedemptionParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeRedemptionParametersUpdated represents a RedemptionParametersUpdated event raised by the Bridge contract.
type BridgeRedemptionParametersUpdated struct {
	RedemptionDustThreshold                   uint64
	RedemptionTreasuryFeeDivisor              uint64
	RedemptionTxMaxFee                        uint64
	RedemptionTxMaxTotalFee                   uint64
	RedemptionTimeout                         uint32
	RedemptionTimeoutSlashingAmount           *big.Int
	RedemptionTimeoutNotifierRewardMultiplier uint32
	Raw                                       types.Log // Blockchain specific contextual infos
}

// FilterRedemptionParametersUpdated is a free log retrieval operation binding the contract event 0x2c682acb687f36d9e4d486fc0c681b0e802911b0fb56f3e0d54f373e7f89a7c4.
//
// Solidity: event RedemptionParametersUpdated(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) FilterRedemptionParametersUpdated(opts *bind.FilterOpts) (*BridgeRedemptionParametersUpdatedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "RedemptionParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &BridgeRedemptionParametersUpdatedIterator{contract: _Bridge.contract, event: "RedemptionParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchRedemptionParametersUpdated is a free log subscription operation binding the contract event 0x2c682acb687f36d9e4d486fc0c681b0e802911b0fb56f3e0d54f373e7f89a7c4.
//
// Solidity: event RedemptionParametersUpdated(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) WatchRedemptionParametersUpdated(opts *bind.WatchOpts, sink chan<- *BridgeRedemptionParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "RedemptionParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeRedemptionParametersUpdated)
				if err := _Bridge.contract.UnpackLog(event, "RedemptionParametersUpdated", log); err != nil {
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

// ParseRedemptionParametersUpdated is a log parse operation binding the contract event 0x2c682acb687f36d9e4d486fc0c681b0e802911b0fb56f3e0d54f373e7f89a7c4.
//
// Solidity: event RedemptionParametersUpdated(uint64 redemptionDustThreshold, uint64 redemptionTreasuryFeeDivisor, uint64 redemptionTxMaxFee, uint64 redemptionTxMaxTotalFee, uint32 redemptionTimeout, uint96 redemptionTimeoutSlashingAmount, uint32 redemptionTimeoutNotifierRewardMultiplier)
func (_Bridge *BridgeFilterer) ParseRedemptionParametersUpdated(log types.Log) (*BridgeRedemptionParametersUpdated, error) {
	event := new(BridgeRedemptionParametersUpdated)
	if err := _Bridge.contract.UnpackLog(event, "RedemptionParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeRedemptionRequestedIterator is returned from FilterRedemptionRequested and is used to iterate over the raw logs and unpacked data for RedemptionRequested events raised by the Bridge contract.
type BridgeRedemptionRequestedIterator struct {
	Event *BridgeRedemptionRequested // Event containing the contract specifics and raw log

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
func (it *BridgeRedemptionRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeRedemptionRequested)
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
		it.Event = new(BridgeRedemptionRequested)
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
func (it *BridgeRedemptionRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeRedemptionRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeRedemptionRequested represents a RedemptionRequested event raised by the Bridge contract.
type BridgeRedemptionRequested struct {
	WalletPubKeyHash     [20]byte
	RedeemerOutputScript []byte
	Redeemer             common.Address
	RequestedAmount      uint64
	TreasuryFee          uint64
	TxMaxFee             uint64
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterRedemptionRequested is a free log retrieval operation binding the contract event 0x97a0199072f487232635d50ab75860891afe0b91c976ed2fc76502c4d82d0d95.
//
// Solidity: event RedemptionRequested(bytes20 indexed walletPubKeyHash, bytes redeemerOutputScript, address indexed redeemer, uint64 requestedAmount, uint64 treasuryFee, uint64 txMaxFee)
func (_Bridge *BridgeFilterer) FilterRedemptionRequested(opts *bind.FilterOpts, walletPubKeyHash [][20]byte, redeemer []common.Address) (*BridgeRedemptionRequestedIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "RedemptionRequested", walletPubKeyHashRule, redeemerRule)
	if err != nil {
		return nil, err
	}
	return &BridgeRedemptionRequestedIterator{contract: _Bridge.contract, event: "RedemptionRequested", logs: logs, sub: sub}, nil
}

// WatchRedemptionRequested is a free log subscription operation binding the contract event 0x97a0199072f487232635d50ab75860891afe0b91c976ed2fc76502c4d82d0d95.
//
// Solidity: event RedemptionRequested(bytes20 indexed walletPubKeyHash, bytes redeemerOutputScript, address indexed redeemer, uint64 requestedAmount, uint64 treasuryFee, uint64 txMaxFee)
func (_Bridge *BridgeFilterer) WatchRedemptionRequested(opts *bind.WatchOpts, sink chan<- *BridgeRedemptionRequested, walletPubKeyHash [][20]byte, redeemer []common.Address) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	var redeemerRule []interface{}
	for _, redeemerItem := range redeemer {
		redeemerRule = append(redeemerRule, redeemerItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "RedemptionRequested", walletPubKeyHashRule, redeemerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeRedemptionRequested)
				if err := _Bridge.contract.UnpackLog(event, "RedemptionRequested", log); err != nil {
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

// ParseRedemptionRequested is a log parse operation binding the contract event 0x97a0199072f487232635d50ab75860891afe0b91c976ed2fc76502c4d82d0d95.
//
// Solidity: event RedemptionRequested(bytes20 indexed walletPubKeyHash, bytes redeemerOutputScript, address indexed redeemer, uint64 requestedAmount, uint64 treasuryFee, uint64 txMaxFee)
func (_Bridge *BridgeFilterer) ParseRedemptionRequested(log types.Log) (*BridgeRedemptionRequested, error) {
	event := new(BridgeRedemptionRequested)
	if err := _Bridge.contract.UnpackLog(event, "RedemptionRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeRedemptionTimedOutIterator is returned from FilterRedemptionTimedOut and is used to iterate over the raw logs and unpacked data for RedemptionTimedOut events raised by the Bridge contract.
type BridgeRedemptionTimedOutIterator struct {
	Event *BridgeRedemptionTimedOut // Event containing the contract specifics and raw log

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
func (it *BridgeRedemptionTimedOutIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeRedemptionTimedOut)
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
		it.Event = new(BridgeRedemptionTimedOut)
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
func (it *BridgeRedemptionTimedOutIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeRedemptionTimedOutIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeRedemptionTimedOut represents a RedemptionTimedOut event raised by the Bridge contract.
type BridgeRedemptionTimedOut struct {
	WalletPubKeyHash     [20]byte
	RedeemerOutputScript []byte
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterRedemptionTimedOut is a free log retrieval operation binding the contract event 0x013dddc8debc4730cd01232fdcea9ae6013689837732356c187fe5edb9a8a8d5.
//
// Solidity: event RedemptionTimedOut(bytes20 indexed walletPubKeyHash, bytes redeemerOutputScript)
func (_Bridge *BridgeFilterer) FilterRedemptionTimedOut(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeRedemptionTimedOutIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "RedemptionTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeRedemptionTimedOutIterator{contract: _Bridge.contract, event: "RedemptionTimedOut", logs: logs, sub: sub}, nil
}

// WatchRedemptionTimedOut is a free log subscription operation binding the contract event 0x013dddc8debc4730cd01232fdcea9ae6013689837732356c187fe5edb9a8a8d5.
//
// Solidity: event RedemptionTimedOut(bytes20 indexed walletPubKeyHash, bytes redeemerOutputScript)
func (_Bridge *BridgeFilterer) WatchRedemptionTimedOut(opts *bind.WatchOpts, sink chan<- *BridgeRedemptionTimedOut, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "RedemptionTimedOut", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeRedemptionTimedOut)
				if err := _Bridge.contract.UnpackLog(event, "RedemptionTimedOut", log); err != nil {
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

// ParseRedemptionTimedOut is a log parse operation binding the contract event 0x013dddc8debc4730cd01232fdcea9ae6013689837732356c187fe5edb9a8a8d5.
//
// Solidity: event RedemptionTimedOut(bytes20 indexed walletPubKeyHash, bytes redeemerOutputScript)
func (_Bridge *BridgeFilterer) ParseRedemptionTimedOut(log types.Log) (*BridgeRedemptionTimedOut, error) {
	event := new(BridgeRedemptionTimedOut)
	if err := _Bridge.contract.UnpackLog(event, "RedemptionTimedOut", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeRedemptionWatchtowerSetIterator is returned from FilterRedemptionWatchtowerSet and is used to iterate over the raw logs and unpacked data for RedemptionWatchtowerSet events raised by the Bridge contract.
type BridgeRedemptionWatchtowerSetIterator struct {
	Event *BridgeRedemptionWatchtowerSet // Event containing the contract specifics and raw log

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
func (it *BridgeRedemptionWatchtowerSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeRedemptionWatchtowerSet)
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
		it.Event = new(BridgeRedemptionWatchtowerSet)
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
func (it *BridgeRedemptionWatchtowerSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeRedemptionWatchtowerSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeRedemptionWatchtowerSet represents a RedemptionWatchtowerSet event raised by the Bridge contract.
type BridgeRedemptionWatchtowerSet struct {
	RedemptionWatchtower common.Address
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterRedemptionWatchtowerSet is a free log retrieval operation binding the contract event 0x13f696992cabdd1b00afcc752ff98fe39e3fe341c16ed5f55e55c64875de0b99.
//
// Solidity: event RedemptionWatchtowerSet(address redemptionWatchtower)
func (_Bridge *BridgeFilterer) FilterRedemptionWatchtowerSet(opts *bind.FilterOpts) (*BridgeRedemptionWatchtowerSetIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "RedemptionWatchtowerSet")
	if err != nil {
		return nil, err
	}
	return &BridgeRedemptionWatchtowerSetIterator{contract: _Bridge.contract, event: "RedemptionWatchtowerSet", logs: logs, sub: sub}, nil
}

// WatchRedemptionWatchtowerSet is a free log subscription operation binding the contract event 0x13f696992cabdd1b00afcc752ff98fe39e3fe341c16ed5f55e55c64875de0b99.
//
// Solidity: event RedemptionWatchtowerSet(address redemptionWatchtower)
func (_Bridge *BridgeFilterer) WatchRedemptionWatchtowerSet(opts *bind.WatchOpts, sink chan<- *BridgeRedemptionWatchtowerSet) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "RedemptionWatchtowerSet")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeRedemptionWatchtowerSet)
				if err := _Bridge.contract.UnpackLog(event, "RedemptionWatchtowerSet", log); err != nil {
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

// ParseRedemptionWatchtowerSet is a log parse operation binding the contract event 0x13f696992cabdd1b00afcc752ff98fe39e3fe341c16ed5f55e55c64875de0b99.
//
// Solidity: event RedemptionWatchtowerSet(address redemptionWatchtower)
func (_Bridge *BridgeFilterer) ParseRedemptionWatchtowerSet(log types.Log) (*BridgeRedemptionWatchtowerSet, error) {
	event := new(BridgeRedemptionWatchtowerSet)
	if err := _Bridge.contract.UnpackLog(event, "RedemptionWatchtowerSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeRedemptionsCompletedIterator is returned from FilterRedemptionsCompleted and is used to iterate over the raw logs and unpacked data for RedemptionsCompleted events raised by the Bridge contract.
type BridgeRedemptionsCompletedIterator struct {
	Event *BridgeRedemptionsCompleted // Event containing the contract specifics and raw log

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
func (it *BridgeRedemptionsCompletedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeRedemptionsCompleted)
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
		it.Event = new(BridgeRedemptionsCompleted)
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
func (it *BridgeRedemptionsCompletedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeRedemptionsCompletedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeRedemptionsCompleted represents a RedemptionsCompleted event raised by the Bridge contract.
type BridgeRedemptionsCompleted struct {
	WalletPubKeyHash [20]byte
	RedemptionTxHash [32]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterRedemptionsCompleted is a free log retrieval operation binding the contract event 0xa45596c10f758d32ec8cca64a0fbfe776052b08fdb3f026e0a87f52118bf8fbe.
//
// Solidity: event RedemptionsCompleted(bytes20 indexed walletPubKeyHash, bytes32 redemptionTxHash)
func (_Bridge *BridgeFilterer) FilterRedemptionsCompleted(opts *bind.FilterOpts, walletPubKeyHash [][20]byte) (*BridgeRedemptionsCompletedIterator, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "RedemptionsCompleted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeRedemptionsCompletedIterator{contract: _Bridge.contract, event: "RedemptionsCompleted", logs: logs, sub: sub}, nil
}

// WatchRedemptionsCompleted is a free log subscription operation binding the contract event 0xa45596c10f758d32ec8cca64a0fbfe776052b08fdb3f026e0a87f52118bf8fbe.
//
// Solidity: event RedemptionsCompleted(bytes20 indexed walletPubKeyHash, bytes32 redemptionTxHash)
func (_Bridge *BridgeFilterer) WatchRedemptionsCompleted(opts *bind.WatchOpts, sink chan<- *BridgeRedemptionsCompleted, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "RedemptionsCompleted", walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeRedemptionsCompleted)
				if err := _Bridge.contract.UnpackLog(event, "RedemptionsCompleted", log); err != nil {
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

// ParseRedemptionsCompleted is a log parse operation binding the contract event 0xa45596c10f758d32ec8cca64a0fbfe776052b08fdb3f026e0a87f52118bf8fbe.
//
// Solidity: event RedemptionsCompleted(bytes20 indexed walletPubKeyHash, bytes32 redemptionTxHash)
func (_Bridge *BridgeFilterer) ParseRedemptionsCompleted(log types.Log) (*BridgeRedemptionsCompleted, error) {
	event := new(BridgeRedemptionsCompleted)
	if err := _Bridge.contract.UnpackLog(event, "RedemptionsCompleted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeSpvMaintainerStatusUpdatedIterator is returned from FilterSpvMaintainerStatusUpdated and is used to iterate over the raw logs and unpacked data for SpvMaintainerStatusUpdated events raised by the Bridge contract.
type BridgeSpvMaintainerStatusUpdatedIterator struct {
	Event *BridgeSpvMaintainerStatusUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeSpvMaintainerStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeSpvMaintainerStatusUpdated)
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
		it.Event = new(BridgeSpvMaintainerStatusUpdated)
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
func (it *BridgeSpvMaintainerStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeSpvMaintainerStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeSpvMaintainerStatusUpdated represents a SpvMaintainerStatusUpdated event raised by the Bridge contract.
type BridgeSpvMaintainerStatusUpdated struct {
	SpvMaintainer common.Address
	IsTrusted     bool
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterSpvMaintainerStatusUpdated is a free log retrieval operation binding the contract event 0x8779ecd5a875d699b846953c9af5ef54f4a354fe4568c460d43c14c319119170.
//
// Solidity: event SpvMaintainerStatusUpdated(address indexed spvMaintainer, bool isTrusted)
func (_Bridge *BridgeFilterer) FilterSpvMaintainerStatusUpdated(opts *bind.FilterOpts, spvMaintainer []common.Address) (*BridgeSpvMaintainerStatusUpdatedIterator, error) {

	var spvMaintainerRule []interface{}
	for _, spvMaintainerItem := range spvMaintainer {
		spvMaintainerRule = append(spvMaintainerRule, spvMaintainerItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "SpvMaintainerStatusUpdated", spvMaintainerRule)
	if err != nil {
		return nil, err
	}
	return &BridgeSpvMaintainerStatusUpdatedIterator{contract: _Bridge.contract, event: "SpvMaintainerStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchSpvMaintainerStatusUpdated is a free log subscription operation binding the contract event 0x8779ecd5a875d699b846953c9af5ef54f4a354fe4568c460d43c14c319119170.
//
// Solidity: event SpvMaintainerStatusUpdated(address indexed spvMaintainer, bool isTrusted)
func (_Bridge *BridgeFilterer) WatchSpvMaintainerStatusUpdated(opts *bind.WatchOpts, sink chan<- *BridgeSpvMaintainerStatusUpdated, spvMaintainer []common.Address) (event.Subscription, error) {

	var spvMaintainerRule []interface{}
	for _, spvMaintainerItem := range spvMaintainer {
		spvMaintainerRule = append(spvMaintainerRule, spvMaintainerItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "SpvMaintainerStatusUpdated", spvMaintainerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeSpvMaintainerStatusUpdated)
				if err := _Bridge.contract.UnpackLog(event, "SpvMaintainerStatusUpdated", log); err != nil {
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

// ParseSpvMaintainerStatusUpdated is a log parse operation binding the contract event 0x8779ecd5a875d699b846953c9af5ef54f4a354fe4568c460d43c14c319119170.
//
// Solidity: event SpvMaintainerStatusUpdated(address indexed spvMaintainer, bool isTrusted)
func (_Bridge *BridgeFilterer) ParseSpvMaintainerStatusUpdated(log types.Log) (*BridgeSpvMaintainerStatusUpdated, error) {
	event := new(BridgeSpvMaintainerStatusUpdated)
	if err := _Bridge.contract.UnpackLog(event, "SpvMaintainerStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeTreasuryUpdatedIterator is returned from FilterTreasuryUpdated and is used to iterate over the raw logs and unpacked data for TreasuryUpdated events raised by the Bridge contract.
type BridgeTreasuryUpdatedIterator struct {
	Event *BridgeTreasuryUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeTreasuryUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeTreasuryUpdated)
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
		it.Event = new(BridgeTreasuryUpdated)
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
func (it *BridgeTreasuryUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeTreasuryUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeTreasuryUpdated represents a TreasuryUpdated event raised by the Bridge contract.
type BridgeTreasuryUpdated struct {
	Treasury common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterTreasuryUpdated is a free log retrieval operation binding the contract event 0x7dae230f18360d76a040c81f050aa14eb9d6dc7901b20fc5d855e2a20fe814d1.
//
// Solidity: event TreasuryUpdated(address treasury)
func (_Bridge *BridgeFilterer) FilterTreasuryUpdated(opts *bind.FilterOpts) (*BridgeTreasuryUpdatedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "TreasuryUpdated")
	if err != nil {
		return nil, err
	}
	return &BridgeTreasuryUpdatedIterator{contract: _Bridge.contract, event: "TreasuryUpdated", logs: logs, sub: sub}, nil
}

// WatchTreasuryUpdated is a free log subscription operation binding the contract event 0x7dae230f18360d76a040c81f050aa14eb9d6dc7901b20fc5d855e2a20fe814d1.
//
// Solidity: event TreasuryUpdated(address treasury)
func (_Bridge *BridgeFilterer) WatchTreasuryUpdated(opts *bind.WatchOpts, sink chan<- *BridgeTreasuryUpdated) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "TreasuryUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeTreasuryUpdated)
				if err := _Bridge.contract.UnpackLog(event, "TreasuryUpdated", log); err != nil {
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

// ParseTreasuryUpdated is a log parse operation binding the contract event 0x7dae230f18360d76a040c81f050aa14eb9d6dc7901b20fc5d855e2a20fe814d1.
//
// Solidity: event TreasuryUpdated(address treasury)
func (_Bridge *BridgeFilterer) ParseTreasuryUpdated(log types.Log) (*BridgeTreasuryUpdated, error) {
	event := new(BridgeTreasuryUpdated)
	if err := _Bridge.contract.UnpackLog(event, "TreasuryUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeVaultStatusUpdatedIterator is returned from FilterVaultStatusUpdated and is used to iterate over the raw logs and unpacked data for VaultStatusUpdated events raised by the Bridge contract.
type BridgeVaultStatusUpdatedIterator struct {
	Event *BridgeVaultStatusUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeVaultStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeVaultStatusUpdated)
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
		it.Event = new(BridgeVaultStatusUpdated)
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
func (it *BridgeVaultStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeVaultStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeVaultStatusUpdated represents a VaultStatusUpdated event raised by the Bridge contract.
type BridgeVaultStatusUpdated struct {
	Vault     common.Address
	IsTrusted bool
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterVaultStatusUpdated is a free log retrieval operation binding the contract event 0x9065599c12c4294d9e2201638226d0d0beb95c228f468c4e7c2bdb8322b6066f.
//
// Solidity: event VaultStatusUpdated(address indexed vault, bool isTrusted)
func (_Bridge *BridgeFilterer) FilterVaultStatusUpdated(opts *bind.FilterOpts, vault []common.Address) (*BridgeVaultStatusUpdatedIterator, error) {

	var vaultRule []interface{}
	for _, vaultItem := range vault {
		vaultRule = append(vaultRule, vaultItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "VaultStatusUpdated", vaultRule)
	if err != nil {
		return nil, err
	}
	return &BridgeVaultStatusUpdatedIterator{contract: _Bridge.contract, event: "VaultStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchVaultStatusUpdated is a free log subscription operation binding the contract event 0x9065599c12c4294d9e2201638226d0d0beb95c228f468c4e7c2bdb8322b6066f.
//
// Solidity: event VaultStatusUpdated(address indexed vault, bool isTrusted)
func (_Bridge *BridgeFilterer) WatchVaultStatusUpdated(opts *bind.WatchOpts, sink chan<- *BridgeVaultStatusUpdated, vault []common.Address) (event.Subscription, error) {

	var vaultRule []interface{}
	for _, vaultItem := range vault {
		vaultRule = append(vaultRule, vaultItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "VaultStatusUpdated", vaultRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeVaultStatusUpdated)
				if err := _Bridge.contract.UnpackLog(event, "VaultStatusUpdated", log); err != nil {
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

// ParseVaultStatusUpdated is a log parse operation binding the contract event 0x9065599c12c4294d9e2201638226d0d0beb95c228f468c4e7c2bdb8322b6066f.
//
// Solidity: event VaultStatusUpdated(address indexed vault, bool isTrusted)
func (_Bridge *BridgeFilterer) ParseVaultStatusUpdated(log types.Log) (*BridgeVaultStatusUpdated, error) {
	event := new(BridgeVaultStatusUpdated)
	if err := _Bridge.contract.UnpackLog(event, "VaultStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeWalletClosedIterator is returned from FilterWalletClosed and is used to iterate over the raw logs and unpacked data for WalletClosed events raised by the Bridge contract.
type BridgeWalletClosedIterator struct {
	Event *BridgeWalletClosed // Event containing the contract specifics and raw log

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
func (it *BridgeWalletClosedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeWalletClosed)
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
		it.Event = new(BridgeWalletClosed)
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
func (it *BridgeWalletClosedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeWalletClosedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeWalletClosed represents a WalletClosed event raised by the Bridge contract.
type BridgeWalletClosed struct {
	EcdsaWalletID    [32]byte
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterWalletClosed is a free log retrieval operation binding the contract event 0x47b159947c3066cb253f60e8f046cfd747411788a545cb189679e3fa1467b28d.
//
// Solidity: event WalletClosed(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterWalletClosed(opts *bind.FilterOpts, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (*BridgeWalletClosedIterator, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "WalletClosed", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeWalletClosedIterator{contract: _Bridge.contract, event: "WalletClosed", logs: logs, sub: sub}, nil
}

// WatchWalletClosed is a free log subscription operation binding the contract event 0x47b159947c3066cb253f60e8f046cfd747411788a545cb189679e3fa1467b28d.
//
// Solidity: event WalletClosed(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchWalletClosed(opts *bind.WatchOpts, sink chan<- *BridgeWalletClosed, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "WalletClosed", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeWalletClosed)
				if err := _Bridge.contract.UnpackLog(event, "WalletClosed", log); err != nil {
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

// ParseWalletClosed is a log parse operation binding the contract event 0x47b159947c3066cb253f60e8f046cfd747411788a545cb189679e3fa1467b28d.
//
// Solidity: event WalletClosed(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseWalletClosed(log types.Log) (*BridgeWalletClosed, error) {
	event := new(BridgeWalletClosed)
	if err := _Bridge.contract.UnpackLog(event, "WalletClosed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeWalletClosingIterator is returned from FilterWalletClosing and is used to iterate over the raw logs and unpacked data for WalletClosing events raised by the Bridge contract.
type BridgeWalletClosingIterator struct {
	Event *BridgeWalletClosing // Event containing the contract specifics and raw log

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
func (it *BridgeWalletClosingIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeWalletClosing)
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
		it.Event = new(BridgeWalletClosing)
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
func (it *BridgeWalletClosingIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeWalletClosingIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeWalletClosing represents a WalletClosing event raised by the Bridge contract.
type BridgeWalletClosing struct {
	EcdsaWalletID    [32]byte
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterWalletClosing is a free log retrieval operation binding the contract event 0x68cb496f5e64383745876664ef119840f154a729c03ba866b8aecb5c9f53d516.
//
// Solidity: event WalletClosing(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterWalletClosing(opts *bind.FilterOpts, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (*BridgeWalletClosingIterator, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "WalletClosing", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeWalletClosingIterator{contract: _Bridge.contract, event: "WalletClosing", logs: logs, sub: sub}, nil
}

// WatchWalletClosing is a free log subscription operation binding the contract event 0x68cb496f5e64383745876664ef119840f154a729c03ba866b8aecb5c9f53d516.
//
// Solidity: event WalletClosing(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchWalletClosing(opts *bind.WatchOpts, sink chan<- *BridgeWalletClosing, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "WalletClosing", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeWalletClosing)
				if err := _Bridge.contract.UnpackLog(event, "WalletClosing", log); err != nil {
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

// ParseWalletClosing is a log parse operation binding the contract event 0x68cb496f5e64383745876664ef119840f154a729c03ba866b8aecb5c9f53d516.
//
// Solidity: event WalletClosing(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseWalletClosing(log types.Log) (*BridgeWalletClosing, error) {
	event := new(BridgeWalletClosing)
	if err := _Bridge.contract.UnpackLog(event, "WalletClosing", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeWalletMovingFundsIterator is returned from FilterWalletMovingFunds and is used to iterate over the raw logs and unpacked data for WalletMovingFunds events raised by the Bridge contract.
type BridgeWalletMovingFundsIterator struct {
	Event *BridgeWalletMovingFunds // Event containing the contract specifics and raw log

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
func (it *BridgeWalletMovingFundsIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeWalletMovingFunds)
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
		it.Event = new(BridgeWalletMovingFunds)
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
func (it *BridgeWalletMovingFundsIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeWalletMovingFundsIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeWalletMovingFunds represents a WalletMovingFunds event raised by the Bridge contract.
type BridgeWalletMovingFunds struct {
	EcdsaWalletID    [32]byte
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterWalletMovingFunds is a free log retrieval operation binding the contract event 0xbdc9ce990a067e5fd3a5d8dfc68e27e9f221aaa3fe55265e0b7e93c460b3efe2.
//
// Solidity: event WalletMovingFunds(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterWalletMovingFunds(opts *bind.FilterOpts, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (*BridgeWalletMovingFundsIterator, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "WalletMovingFunds", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeWalletMovingFundsIterator{contract: _Bridge.contract, event: "WalletMovingFunds", logs: logs, sub: sub}, nil
}

// WatchWalletMovingFunds is a free log subscription operation binding the contract event 0xbdc9ce990a067e5fd3a5d8dfc68e27e9f221aaa3fe55265e0b7e93c460b3efe2.
//
// Solidity: event WalletMovingFunds(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchWalletMovingFunds(opts *bind.WatchOpts, sink chan<- *BridgeWalletMovingFunds, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "WalletMovingFunds", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeWalletMovingFunds)
				if err := _Bridge.contract.UnpackLog(event, "WalletMovingFunds", log); err != nil {
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

// ParseWalletMovingFunds is a log parse operation binding the contract event 0xbdc9ce990a067e5fd3a5d8dfc68e27e9f221aaa3fe55265e0b7e93c460b3efe2.
//
// Solidity: event WalletMovingFunds(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseWalletMovingFunds(log types.Log) (*BridgeWalletMovingFunds, error) {
	event := new(BridgeWalletMovingFunds)
	if err := _Bridge.contract.UnpackLog(event, "WalletMovingFunds", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeWalletParametersUpdatedIterator is returned from FilterWalletParametersUpdated and is used to iterate over the raw logs and unpacked data for WalletParametersUpdated events raised by the Bridge contract.
type BridgeWalletParametersUpdatedIterator struct {
	Event *BridgeWalletParametersUpdated // Event containing the contract specifics and raw log

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
func (it *BridgeWalletParametersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeWalletParametersUpdated)
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
		it.Event = new(BridgeWalletParametersUpdated)
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
func (it *BridgeWalletParametersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeWalletParametersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeWalletParametersUpdated represents a WalletParametersUpdated event raised by the Bridge contract.
type BridgeWalletParametersUpdated struct {
	WalletCreationPeriod        uint32
	WalletCreationMinBtcBalance uint64
	WalletCreationMaxBtcBalance uint64
	WalletClosureMinBtcBalance  uint64
	WalletMaxAge                uint32
	WalletMaxBtcTransfer        uint64
	WalletClosingPeriod         uint32
	Raw                         types.Log // Blockchain specific contextual infos
}

// FilterWalletParametersUpdated is a free log retrieval operation binding the contract event 0xc7d3a9af08692aeae771c329fddd95c7237a9f76fec996325f3959eeff07d4ac.
//
// Solidity: event WalletParametersUpdated(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod)
func (_Bridge *BridgeFilterer) FilterWalletParametersUpdated(opts *bind.FilterOpts) (*BridgeWalletParametersUpdatedIterator, error) {

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "WalletParametersUpdated")
	if err != nil {
		return nil, err
	}
	return &BridgeWalletParametersUpdatedIterator{contract: _Bridge.contract, event: "WalletParametersUpdated", logs: logs, sub: sub}, nil
}

// WatchWalletParametersUpdated is a free log subscription operation binding the contract event 0xc7d3a9af08692aeae771c329fddd95c7237a9f76fec996325f3959eeff07d4ac.
//
// Solidity: event WalletParametersUpdated(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod)
func (_Bridge *BridgeFilterer) WatchWalletParametersUpdated(opts *bind.WatchOpts, sink chan<- *BridgeWalletParametersUpdated) (event.Subscription, error) {

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "WalletParametersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeWalletParametersUpdated)
				if err := _Bridge.contract.UnpackLog(event, "WalletParametersUpdated", log); err != nil {
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

// ParseWalletParametersUpdated is a log parse operation binding the contract event 0xc7d3a9af08692aeae771c329fddd95c7237a9f76fec996325f3959eeff07d4ac.
//
// Solidity: event WalletParametersUpdated(uint32 walletCreationPeriod, uint64 walletCreationMinBtcBalance, uint64 walletCreationMaxBtcBalance, uint64 walletClosureMinBtcBalance, uint32 walletMaxAge, uint64 walletMaxBtcTransfer, uint32 walletClosingPeriod)
func (_Bridge *BridgeFilterer) ParseWalletParametersUpdated(log types.Log) (*BridgeWalletParametersUpdated, error) {
	event := new(BridgeWalletParametersUpdated)
	if err := _Bridge.contract.UnpackLog(event, "WalletParametersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeWalletTerminatedIterator is returned from FilterWalletTerminated and is used to iterate over the raw logs and unpacked data for WalletTerminated events raised by the Bridge contract.
type BridgeWalletTerminatedIterator struct {
	Event *BridgeWalletTerminated // Event containing the contract specifics and raw log

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
func (it *BridgeWalletTerminatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeWalletTerminated)
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
		it.Event = new(BridgeWalletTerminated)
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
func (it *BridgeWalletTerminatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeWalletTerminatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeWalletTerminated represents a WalletTerminated event raised by the Bridge contract.
type BridgeWalletTerminated struct {
	EcdsaWalletID    [32]byte
	WalletPubKeyHash [20]byte
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterWalletTerminated is a free log retrieval operation binding the contract event 0x9272a280b0f32f70b00ad0b546499c68e3ecc6f7bb7ef43491ec5d7b99bf69ef.
//
// Solidity: event WalletTerminated(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) FilterWalletTerminated(opts *bind.FilterOpts, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (*BridgeWalletTerminatedIterator, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "WalletTerminated", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return &BridgeWalletTerminatedIterator{contract: _Bridge.contract, event: "WalletTerminated", logs: logs, sub: sub}, nil
}

// WatchWalletTerminated is a free log subscription operation binding the contract event 0x9272a280b0f32f70b00ad0b546499c68e3ecc6f7bb7ef43491ec5d7b99bf69ef.
//
// Solidity: event WalletTerminated(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) WatchWalletTerminated(opts *bind.WatchOpts, sink chan<- *BridgeWalletTerminated, ecdsaWalletID [][32]byte, walletPubKeyHash [][20]byte) (event.Subscription, error) {

	var ecdsaWalletIDRule []interface{}
	for _, ecdsaWalletIDItem := range ecdsaWalletID {
		ecdsaWalletIDRule = append(ecdsaWalletIDRule, ecdsaWalletIDItem)
	}
	var walletPubKeyHashRule []interface{}
	for _, walletPubKeyHashItem := range walletPubKeyHash {
		walletPubKeyHashRule = append(walletPubKeyHashRule, walletPubKeyHashItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "WalletTerminated", ecdsaWalletIDRule, walletPubKeyHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeWalletTerminated)
				if err := _Bridge.contract.UnpackLog(event, "WalletTerminated", log); err != nil {
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

// ParseWalletTerminated is a log parse operation binding the contract event 0x9272a280b0f32f70b00ad0b546499c68e3ecc6f7bb7ef43491ec5d7b99bf69ef.
//
// Solidity: event WalletTerminated(bytes32 indexed ecdsaWalletID, bytes20 indexed walletPubKeyHash)
func (_Bridge *BridgeFilterer) ParseWalletTerminated(log types.Log) (*BridgeWalletTerminated, error) {
	event := new(BridgeWalletTerminated)
	if err := _Bridge.contract.UnpackLog(event, "WalletTerminated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

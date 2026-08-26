// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"fmt"
	"math/big"
	"strings"
	"sync"

	hostchainabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var wpvLogger = log.Logger("keep-contract-WalletProposalValidator")

type WalletProposalValidator struct {
	contract          *abi.WalletProposalValidator
	contractAddress   common.Address
	contractABI       *hostchainabi.ABI
	caller            bind.ContractCaller
	transactor        bind.ContractTransactor
	callerOptions     *bind.CallOpts
	transactorOptions *bind.TransactOpts
	errorResolver     *chainutil.ErrorResolver
	nonceManager      *ethereum.NonceManager
	miningWaiter      *chainutil.MiningWaiter
	blockCounter      *ethereum.BlockCounter

	transactionMutex *sync.Mutex
}

func NewWalletProposalValidator(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*WalletProposalValidator, error) {
	callerOptions := &bind.CallOpts{
		From: accountKey.Address,
	}

	transactorOptions, err := bind.NewKeyedTransactorWithChainID(
		accountKey.PrivateKey,
		chainId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate transactor: [%v]", err)
	}

	contract, err := abi.NewWalletProposalValidator(
		contractAddress,
		backend,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to instantiate contract at address: %s [%v]",
			contractAddress.String(),
			err,
		)
	}

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.WalletProposalValidatorABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &WalletProposalValidator{
		contract:          contract,
		contractAddress:   contractAddress,
		contractABI:       &contractABI,
		caller:            backend,
		transactor:        backend,
		callerOptions:     callerOptions,
		transactorOptions: transactorOptions,
		errorResolver:     chainutil.NewErrorResolver(backend, &contractABI, &contractAddress),
		nonceManager:      nonceManager,
		miningWaiter:      miningWaiter,
		blockCounter:      blockCounter,
		transactionMutex:  transactionMutex,
	}, nil
}

// ----- Non-const Methods ------

// ----- Const Methods ------

func (wpv *WalletProposalValidator) Bridge() (common.Address, error) {
	result, err := wpv.contract.Bridge(
		wpv.callerOptions,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"bridge",
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) BridgeAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"bridge",
		&result,
	)

	return result, err
}

func (wpv *WalletProposalValidator) DEPOSITMINAGE() (uint32, error) {
	result, err := wpv.contract.DEPOSITMINAGE(
		wpv.callerOptions,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"dEPOSITMINAGE",
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) DEPOSITMINAGEAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"dEPOSITMINAGE",
		&result,
	)

	return result, err
}

func (wpv *WalletProposalValidator) DEPOSITREFUNDSAFETYMARGIN() (uint32, error) {
	result, err := wpv.contract.DEPOSITREFUNDSAFETYMARGIN(
		wpv.callerOptions,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"dEPOSITREFUNDSAFETYMARGIN",
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) DEPOSITREFUNDSAFETYMARGINAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"dEPOSITREFUNDSAFETYMARGIN",
		&result,
	)

	return result, err
}

func (wpv *WalletProposalValidator) DEPOSITSWEEPMAXSIZE() (uint16, error) {
	result, err := wpv.contract.DEPOSITSWEEPMAXSIZE(
		wpv.callerOptions,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"dEPOSITSWEEPMAXSIZE",
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) DEPOSITSWEEPMAXSIZEAtBlock(
	blockNumber *big.Int,
) (uint16, error) {
	var result uint16

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"dEPOSITSWEEPMAXSIZE",
		&result,
	)

	return result, err
}

func (wpv *WalletProposalValidator) REDEMPTIONMAXSIZE() (uint16, error) {
	result, err := wpv.contract.REDEMPTIONMAXSIZE(
		wpv.callerOptions,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"rEDEMPTIONMAXSIZE",
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) REDEMPTIONMAXSIZEAtBlock(
	blockNumber *big.Int,
) (uint16, error) {
	var result uint16

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"rEDEMPTIONMAXSIZE",
		&result,
	)

	return result, err
}

func (wpv *WalletProposalValidator) REDEMPTIONREQUESTMINAGE() (uint32, error) {
	result, err := wpv.contract.REDEMPTIONREQUESTMINAGE(
		wpv.callerOptions,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"rEDEMPTIONREQUESTMINAGE",
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) REDEMPTIONREQUESTMINAGEAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"rEDEMPTIONREQUESTMINAGE",
		&result,
	)

	return result, err
}

func (wpv *WalletProposalValidator) REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN() (uint32, error) {
	result, err := wpv.contract.REDEMPTIONREQUESTTIMEOUTSAFETYMARGIN(
		wpv.callerOptions,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"rEDEMPTIONREQUESTTIMEOUTSAFETYMARGIN",
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) REDEMPTIONREQUESTTIMEOUTSAFETYMARGINAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"rEDEMPTIONREQUESTTIMEOUTSAFETYMARGIN",
		&result,
	)

	return result, err
}

func (wpv *WalletProposalValidator) ValidateDepositSweepProposal(
	arg_proposal abi.WalletProposalValidatorDepositSweepProposal,
	arg_depositsExtraInfo []abi.WalletProposalValidatorDepositExtraInfo,
) (bool, error) {
	result, err := wpv.contract.ValidateDepositSweepProposal(
		wpv.callerOptions,
		arg_proposal,
		arg_depositsExtraInfo,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"validateDepositSweepProposal",
			arg_proposal,
			arg_depositsExtraInfo,
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) ValidateDepositSweepProposalAtBlock(
	arg_proposal abi.WalletProposalValidatorDepositSweepProposal,
	arg_depositsExtraInfo []abi.WalletProposalValidatorDepositExtraInfo,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"validateDepositSweepProposal",
		&result,
		arg_proposal,
		arg_depositsExtraInfo,
	)

	return result, err
}

func (wpv *WalletProposalValidator) ValidateHeartbeatProposal(
	arg_proposal abi.WalletProposalValidatorHeartbeatProposal,
) (bool, error) {
	result, err := wpv.contract.ValidateHeartbeatProposal(
		wpv.callerOptions,
		arg_proposal,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"validateHeartbeatProposal",
			arg_proposal,
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) ValidateHeartbeatProposalAtBlock(
	arg_proposal abi.WalletProposalValidatorHeartbeatProposal,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"validateHeartbeatProposal",
		&result,
		arg_proposal,
	)

	return result, err
}

func (wpv *WalletProposalValidator) ValidateMovedFundsSweepProposal(
	arg_proposal abi.WalletProposalValidatorMovedFundsSweepProposal,
) (bool, error) {
	result, err := wpv.contract.ValidateMovedFundsSweepProposal(
		wpv.callerOptions,
		arg_proposal,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"validateMovedFundsSweepProposal",
			arg_proposal,
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) ValidateMovedFundsSweepProposalAtBlock(
	arg_proposal abi.WalletProposalValidatorMovedFundsSweepProposal,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"validateMovedFundsSweepProposal",
		&result,
		arg_proposal,
	)

	return result, err
}

func (wpv *WalletProposalValidator) ValidateMovingFundsProposal(
	arg_proposal abi.WalletProposalValidatorMovingFundsProposal,
	arg_walletMainUtxo abi.BitcoinTxUTXO3,
) (bool, error) {
	result, err := wpv.contract.ValidateMovingFundsProposal(
		wpv.callerOptions,
		arg_proposal,
		arg_walletMainUtxo,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"validateMovingFundsProposal",
			arg_proposal,
			arg_walletMainUtxo,
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) ValidateMovingFundsProposalAtBlock(
	arg_proposal abi.WalletProposalValidatorMovingFundsProposal,
	arg_walletMainUtxo abi.BitcoinTxUTXO3,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"validateMovingFundsProposal",
		&result,
		arg_proposal,
		arg_walletMainUtxo,
	)

	return result, err
}

func (wpv *WalletProposalValidator) ValidateRedemptionProposal(
	arg_proposal abi.WalletProposalValidatorRedemptionProposal,
) (bool, error) {
	result, err := wpv.contract.ValidateRedemptionProposal(
		wpv.callerOptions,
		arg_proposal,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"validateRedemptionProposal",
			arg_proposal,
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) ValidateRedemptionProposalAtBlock(
	arg_proposal abi.WalletProposalValidatorRedemptionProposal,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"validateRedemptionProposal",
		&result,
		arg_proposal,
	)

	return result, err
}

func (wpv *WalletProposalValidator) ValidateReservationAnchorProposal(
	arg_proposal abi.WalletProposalValidatorReservationAnchorProposal,
	arg_depositExtraInfo abi.WalletProposalValidatorDepositExtraInfo,
) (bool, error) {
	result, err := wpv.contract.ValidateReservationAnchorProposal(
		wpv.callerOptions,
		arg_proposal,
		arg_depositExtraInfo,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"validateReservationAnchorProposal",
			arg_proposal,
			arg_depositExtraInfo,
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) ValidateReservationAnchorProposalAtBlock(
	arg_proposal abi.WalletProposalValidatorReservationAnchorProposal,
	arg_depositExtraInfo abi.WalletProposalValidatorDepositExtraInfo,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"validateReservationAnchorProposal",
		&result,
		arg_proposal,
		arg_depositExtraInfo,
	)

	return result, err
}

func (wpv *WalletProposalValidator) ValidateReservationReanchorProposal(
	arg_proposal abi.WalletProposalValidatorReservationReanchorProposal,
) (bool, error) {
	result, err := wpv.contract.ValidateReservationReanchorProposal(
		wpv.callerOptions,
		arg_proposal,
	)

	if err != nil {
		return result, wpv.errorResolver.ResolveError(
			err,
			wpv.callerOptions.From,
			nil,
			"validateReservationReanchorProposal",
			arg_proposal,
		)
	}

	return result, err
}

func (wpv *WalletProposalValidator) ValidateReservationReanchorProposalAtBlock(
	arg_proposal abi.WalletProposalValidatorReservationReanchorProposal,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wpv.callerOptions.From,
		blockNumber,
		nil,
		wpv.contractABI,
		wpv.caller,
		wpv.errorResolver,
		wpv.contractAddress,
		"validateReservationReanchorProposal",
		&result,
		arg_proposal,
	)

	return result, err
}

// ------ Events -------

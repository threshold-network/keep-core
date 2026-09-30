// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	hostchainabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var tbtcLogger = log.Logger("keep-contract-TBTC")

type TBTC struct {
	contract          *abi.TBTC
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

func NewTBTC(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*TBTC, error) {
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

	contract, err := abi.NewTBTC(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.TBTCABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &TBTC{
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

// Transaction submission.
func (tbtc *TBTC) Approve(
	arg_spender common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction approve",
		" params: ",
		fmt.Sprint(
			arg_spender,
			arg_amount,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.Approve(
		transactorOptions,
		arg_spender,
		arg_amount,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"approve",
			arg_spender,
			arg_amount,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction approve with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.Approve(
				newTransactorOptions,
				arg_spender,
				arg_amount,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"approve",
					arg_spender,
					arg_amount,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction approve with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallApprove(
	arg_spender common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"approve",
		&result,
		arg_spender,
		arg_amount,
	)

	return result, err
}

func (tbtc *TBTC) ApproveGasEstimate(
	arg_spender common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"approve",
		tbtc.contractABI,
		tbtc.transactor,
		arg_spender,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) ApproveAndCall(
	arg_spender common.Address,
	arg_amount *big.Int,
	arg_extraData []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction approveAndCall",
		" params: ",
		fmt.Sprint(
			arg_spender,
			arg_amount,
			arg_extraData,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.ApproveAndCall(
		transactorOptions,
		arg_spender,
		arg_amount,
		arg_extraData,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"approveAndCall",
			arg_spender,
			arg_amount,
			arg_extraData,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction approveAndCall with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.ApproveAndCall(
				newTransactorOptions,
				arg_spender,
				arg_amount,
				arg_extraData,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"approveAndCall",
					arg_spender,
					arg_amount,
					arg_extraData,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction approveAndCall with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallApproveAndCall(
	arg_spender common.Address,
	arg_amount *big.Int,
	arg_extraData []byte,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"approveAndCall",
		&result,
		arg_spender,
		arg_amount,
		arg_extraData,
	)

	return result, err
}

func (tbtc *TBTC) ApproveAndCallGasEstimate(
	arg_spender common.Address,
	arg_amount *big.Int,
	arg_extraData []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"approveAndCall",
		tbtc.contractABI,
		tbtc.transactor,
		arg_spender,
		arg_amount,
		arg_extraData,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) Burn(
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction burn",
		" params: ",
		fmt.Sprint(
			arg_amount,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.Burn(
		transactorOptions,
		arg_amount,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"burn",
			arg_amount,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction burn with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.Burn(
				newTransactorOptions,
				arg_amount,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"burn",
					arg_amount,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction burn with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallBurn(
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"burn",
		&result,
		arg_amount,
	)

	return err
}

func (tbtc *TBTC) BurnGasEstimate(
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"burn",
		tbtc.contractABI,
		tbtc.transactor,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) BurnFrom(
	arg_account common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction burnFrom",
		" params: ",
		fmt.Sprint(
			arg_account,
			arg_amount,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.BurnFrom(
		transactorOptions,
		arg_account,
		arg_amount,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"burnFrom",
			arg_account,
			arg_amount,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction burnFrom with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.BurnFrom(
				newTransactorOptions,
				arg_account,
				arg_amount,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"burnFrom",
					arg_account,
					arg_amount,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction burnFrom with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallBurnFrom(
	arg_account common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"burnFrom",
		&result,
		arg_account,
		arg_amount,
	)

	return err
}

func (tbtc *TBTC) BurnFromGasEstimate(
	arg_account common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"burnFrom",
		tbtc.contractABI,
		tbtc.transactor,
		arg_account,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) Mint(
	arg_recipient common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction mint",
		" params: ",
		fmt.Sprint(
			arg_recipient,
			arg_amount,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.Mint(
		transactorOptions,
		arg_recipient,
		arg_amount,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"mint",
			arg_recipient,
			arg_amount,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction mint with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.Mint(
				newTransactorOptions,
				arg_recipient,
				arg_amount,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"mint",
					arg_recipient,
					arg_amount,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction mint with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallMint(
	arg_recipient common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"mint",
		&result,
		arg_recipient,
		arg_amount,
	)

	return err
}

func (tbtc *TBTC) MintGasEstimate(
	arg_recipient common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"mint",
		tbtc.contractABI,
		tbtc.transactor,
		arg_recipient,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) Permit(
	arg_owner common.Address,
	arg_spender common.Address,
	arg_amount *big.Int,
	arg_deadline *big.Int,
	arg_v uint8,
	arg_r [32]byte,
	arg_s [32]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction permit",
		" params: ",
		fmt.Sprint(
			arg_owner,
			arg_spender,
			arg_amount,
			arg_deadline,
			arg_v,
			arg_r,
			arg_s,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.Permit(
		transactorOptions,
		arg_owner,
		arg_spender,
		arg_amount,
		arg_deadline,
		arg_v,
		arg_r,
		arg_s,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"permit",
			arg_owner,
			arg_spender,
			arg_amount,
			arg_deadline,
			arg_v,
			arg_r,
			arg_s,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction permit with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.Permit(
				newTransactorOptions,
				arg_owner,
				arg_spender,
				arg_amount,
				arg_deadline,
				arg_v,
				arg_r,
				arg_s,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"permit",
					arg_owner,
					arg_spender,
					arg_amount,
					arg_deadline,
					arg_v,
					arg_r,
					arg_s,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction permit with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallPermit(
	arg_owner common.Address,
	arg_spender common.Address,
	arg_amount *big.Int,
	arg_deadline *big.Int,
	arg_v uint8,
	arg_r [32]byte,
	arg_s [32]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"permit",
		&result,
		arg_owner,
		arg_spender,
		arg_amount,
		arg_deadline,
		arg_v,
		arg_r,
		arg_s,
	)

	return err
}

func (tbtc *TBTC) PermitGasEstimate(
	arg_owner common.Address,
	arg_spender common.Address,
	arg_amount *big.Int,
	arg_deadline *big.Int,
	arg_v uint8,
	arg_r [32]byte,
	arg_s [32]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"permit",
		tbtc.contractABI,
		tbtc.transactor,
		arg_owner,
		arg_spender,
		arg_amount,
		arg_deadline,
		arg_v,
		arg_r,
		arg_s,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) RecoverERC20(
	arg_token common.Address,
	arg_recipient common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction recoverERC20",
		" params: ",
		fmt.Sprint(
			arg_token,
			arg_recipient,
			arg_amount,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.RecoverERC20(
		transactorOptions,
		arg_token,
		arg_recipient,
		arg_amount,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"recoverERC20",
			arg_token,
			arg_recipient,
			arg_amount,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction recoverERC20 with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.RecoverERC20(
				newTransactorOptions,
				arg_token,
				arg_recipient,
				arg_amount,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"recoverERC20",
					arg_token,
					arg_recipient,
					arg_amount,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction recoverERC20 with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallRecoverERC20(
	arg_token common.Address,
	arg_recipient common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"recoverERC20",
		&result,
		arg_token,
		arg_recipient,
		arg_amount,
	)

	return err
}

func (tbtc *TBTC) RecoverERC20GasEstimate(
	arg_token common.Address,
	arg_recipient common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"recoverERC20",
		tbtc.contractABI,
		tbtc.transactor,
		arg_token,
		arg_recipient,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) RecoverERC721(
	arg_token common.Address,
	arg_recipient common.Address,
	arg_tokenId *big.Int,
	arg_data []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction recoverERC721",
		" params: ",
		fmt.Sprint(
			arg_token,
			arg_recipient,
			arg_tokenId,
			arg_data,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.RecoverERC721(
		transactorOptions,
		arg_token,
		arg_recipient,
		arg_tokenId,
		arg_data,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"recoverERC721",
			arg_token,
			arg_recipient,
			arg_tokenId,
			arg_data,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction recoverERC721 with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.RecoverERC721(
				newTransactorOptions,
				arg_token,
				arg_recipient,
				arg_tokenId,
				arg_data,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"recoverERC721",
					arg_token,
					arg_recipient,
					arg_tokenId,
					arg_data,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction recoverERC721 with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallRecoverERC721(
	arg_token common.Address,
	arg_recipient common.Address,
	arg_tokenId *big.Int,
	arg_data []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"recoverERC721",
		&result,
		arg_token,
		arg_recipient,
		arg_tokenId,
		arg_data,
	)

	return err
}

func (tbtc *TBTC) RecoverERC721GasEstimate(
	arg_token common.Address,
	arg_recipient common.Address,
	arg_tokenId *big.Int,
	arg_data []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"recoverERC721",
		tbtc.contractABI,
		tbtc.transactor,
		arg_token,
		arg_recipient,
		arg_tokenId,
		arg_data,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) RenounceOwnership(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction renounceOwnership",
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.RenounceOwnership(
		transactorOptions,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"renounceOwnership",
		)
	}

	tbtcLogger.Infof(
		"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.RenounceOwnership(
				newTransactorOptions,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"renounceOwnership",
				)
			}

			tbtcLogger.Infof(
				"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallRenounceOwnership(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"renounceOwnership",
		&result,
	)

	return err
}

func (tbtc *TBTC) RenounceOwnershipGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"renounceOwnership",
		tbtc.contractABI,
		tbtc.transactor,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) Transfer(
	arg_recipient common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction transfer",
		" params: ",
		fmt.Sprint(
			arg_recipient,
			arg_amount,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.Transfer(
		transactorOptions,
		arg_recipient,
		arg_amount,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"transfer",
			arg_recipient,
			arg_amount,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction transfer with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.Transfer(
				newTransactorOptions,
				arg_recipient,
				arg_amount,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"transfer",
					arg_recipient,
					arg_amount,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction transfer with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallTransfer(
	arg_recipient common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"transfer",
		&result,
		arg_recipient,
		arg_amount,
	)

	return result, err
}

func (tbtc *TBTC) TransferGasEstimate(
	arg_recipient common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"transfer",
		tbtc.contractABI,
		tbtc.transactor,
		arg_recipient,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) TransferFrom(
	arg_spender common.Address,
	arg_recipient common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction transferFrom",
		" params: ",
		fmt.Sprint(
			arg_spender,
			arg_recipient,
			arg_amount,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.TransferFrom(
		transactorOptions,
		arg_spender,
		arg_recipient,
		arg_amount,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"transferFrom",
			arg_spender,
			arg_recipient,
			arg_amount,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction transferFrom with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.TransferFrom(
				newTransactorOptions,
				arg_spender,
				arg_recipient,
				arg_amount,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"transferFrom",
					arg_spender,
					arg_recipient,
					arg_amount,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction transferFrom with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallTransferFrom(
	arg_spender common.Address,
	arg_recipient common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"transferFrom",
		&result,
		arg_spender,
		arg_recipient,
		arg_amount,
	)

	return result, err
}

func (tbtc *TBTC) TransferFromGasEstimate(
	arg_spender common.Address,
	arg_recipient common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"transferFrom",
		tbtc.contractABI,
		tbtc.transactor,
		arg_spender,
		arg_recipient,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (tbtc *TBTC) TransferOwnership(
	arg_newOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tbtcLogger.Debug(
		"submitting transaction transferOwnership",
		" params: ",
		fmt.Sprint(
			arg_newOwner,
		),
	)

	tbtc.transactionMutex.Lock()
	defer tbtc.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *tbtc.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := tbtc.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := tbtc.contract.TransferOwnership(
		transactorOptions,
		arg_newOwner,
	)
	if err != nil {
		return transaction, tbtc.errorResolver.ResolveError(
			err,
			tbtc.transactorOptions.From,
			nil,
			"transferOwnership",
			arg_newOwner,
		)
	}

	tbtcLogger.Infof(
		"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go tbtc.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := tbtc.contract.TransferOwnership(
				newTransactorOptions,
				arg_newOwner,
			)
			if err != nil {
				return nil, tbtc.errorResolver.ResolveError(
					err,
					tbtc.transactorOptions.From,
					nil,
					"transferOwnership",
					arg_newOwner,
				)
			}

			tbtcLogger.Infof(
				"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	tbtc.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (tbtc *TBTC) CallTransferOwnership(
	arg_newOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		tbtc.transactorOptions.From,
		blockNumber, nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"transferOwnership",
		&result,
		arg_newOwner,
	)

	return err
}

func (tbtc *TBTC) TransferOwnershipGasEstimate(
	arg_newOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		tbtc.callerOptions.From,
		tbtc.contractAddress,
		"transferOwnership",
		tbtc.contractABI,
		tbtc.transactor,
		arg_newOwner,
	)

	return result, err
}

// ----- Const Methods ------

func (tbtc *TBTC) Allowance(
	arg0 common.Address,
	arg1 common.Address,
) (*big.Int, error) {
	result, err := tbtc.contract.Allowance(
		tbtc.callerOptions,
		arg0,
		arg1,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"allowance",
			arg0,
			arg1,
		)
	}

	return result, err
}

func (tbtc *TBTC) AllowanceAtBlock(
	arg0 common.Address,
	arg1 common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"allowance",
		&result,
		arg0,
		arg1,
	)

	return result, err
}

func (tbtc *TBTC) BalanceOf(
	arg0 common.Address,
) (*big.Int, error) {
	result, err := tbtc.contract.BalanceOf(
		tbtc.callerOptions,
		arg0,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"balanceOf",
			arg0,
		)
	}

	return result, err
}

func (tbtc *TBTC) BalanceOfAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"balanceOf",
		&result,
		arg0,
	)

	return result, err
}

func (tbtc *TBTC) CachedChainId() (*big.Int, error) {
	result, err := tbtc.contract.CachedChainId(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"cachedChainId",
		)
	}

	return result, err
}

func (tbtc *TBTC) CachedChainIdAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"cachedChainId",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) CachedDomainSeparator() ([32]byte, error) {
	result, err := tbtc.contract.CachedDomainSeparator(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"cachedDomainSeparator",
		)
	}

	return result, err
}

func (tbtc *TBTC) CachedDomainSeparatorAtBlock(
	blockNumber *big.Int,
) ([32]byte, error) {
	var result [32]byte

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"cachedDomainSeparator",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) DOMAINSEPARATOR() ([32]byte, error) {
	result, err := tbtc.contract.DOMAINSEPARATOR(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"dOMAINSEPARATOR",
		)
	}

	return result, err
}

func (tbtc *TBTC) DOMAINSEPARATORAtBlock(
	blockNumber *big.Int,
) ([32]byte, error) {
	var result [32]byte

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"dOMAINSEPARATOR",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) Decimals() (uint8, error) {
	result, err := tbtc.contract.Decimals(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"decimals",
		)
	}

	return result, err
}

func (tbtc *TBTC) DecimalsAtBlock(
	blockNumber *big.Int,
) (uint8, error) {
	var result uint8

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"decimals",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) Name() (string, error) {
	result, err := tbtc.contract.Name(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"name",
		)
	}

	return result, err
}

func (tbtc *TBTC) NameAtBlock(
	blockNumber *big.Int,
) (string, error) {
	var result string

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"name",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) Nonce(
	arg0 common.Address,
) (*big.Int, error) {
	result, err := tbtc.contract.Nonce(
		tbtc.callerOptions,
		arg0,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"nonce",
			arg0,
		)
	}

	return result, err
}

func (tbtc *TBTC) NonceAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"nonce",
		&result,
		arg0,
	)

	return result, err
}

func (tbtc *TBTC) Owner() (common.Address, error) {
	result, err := tbtc.contract.Owner(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"owner",
		)
	}

	return result, err
}

func (tbtc *TBTC) OwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"owner",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) PERMITTYPEHASH() ([32]byte, error) {
	result, err := tbtc.contract.PERMITTYPEHASH(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"pERMITTYPEHASH",
		)
	}

	return result, err
}

func (tbtc *TBTC) PERMITTYPEHASHAtBlock(
	blockNumber *big.Int,
) ([32]byte, error) {
	var result [32]byte

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"pERMITTYPEHASH",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) Symbol() (string, error) {
	result, err := tbtc.contract.Symbol(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"symbol",
		)
	}

	return result, err
}

func (tbtc *TBTC) SymbolAtBlock(
	blockNumber *big.Int,
) (string, error) {
	var result string

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"symbol",
		&result,
	)

	return result, err
}

func (tbtc *TBTC) TotalSupply() (*big.Int, error) {
	result, err := tbtc.contract.TotalSupply(
		tbtc.callerOptions,
	)

	if err != nil {
		return result, tbtc.errorResolver.ResolveError(
			err,
			tbtc.callerOptions.From,
			nil,
			"totalSupply",
		)
	}

	return result, err
}

func (tbtc *TBTC) TotalSupplyAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		tbtc.callerOptions.From,
		blockNumber,
		nil,
		tbtc.contractABI,
		tbtc.caller,
		tbtc.errorResolver,
		tbtc.contractAddress,
		"totalSupply",
		&result,
	)

	return result, err
}

// ------ Events -------

func (tbtc *TBTC) ApprovalEvent(
	opts *ethereum.SubscribeOpts,
	ownerFilter []common.Address,
	spenderFilter []common.Address,
) *TbtcApprovalSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TbtcApprovalSubscription{
		tbtc,
		opts,
		ownerFilter,
		spenderFilter,
	}
}

type TbtcApprovalSubscription struct {
	contract      *TBTC
	opts          *ethereum.SubscribeOpts
	ownerFilter   []common.Address
	spenderFilter []common.Address
}

type tBTCApprovalFunc func(
	Owner common.Address,
	Spender common.Address,
	Value *big.Int,
	blockNumber uint64,
)

func (as *TbtcApprovalSubscription) OnEvent(
	handler tBTCApprovalFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TBTCApproval)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Owner,
					event.Spender,
					event.Value,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := as.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (as *TbtcApprovalSubscription) Pipe(
	sink chan *abi.TBTCApproval,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(as.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := as.contract.blockCounter.CurrentBlock()
				if err != nil {
					tbtcLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - as.opts.PastBlocks

				tbtcLogger.Infof(
					"subscription monitoring fetching past Approval events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := as.contract.PastApprovalEvents(
					fromBlock,
					nil,
					as.ownerFilter,
					as.spenderFilter,
				)
				if err != nil {
					tbtcLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tbtcLogger.Infof(
					"subscription monitoring fetched [%v] past Approval events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := as.contract.watchApproval(
		sink,
		as.ownerFilter,
		as.spenderFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (tbtc *TBTC) watchApproval(
	sink chan *abi.TBTCApproval,
	ownerFilter []common.Address,
	spenderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return tbtc.contract.WatchApproval(
			&bind.WatchOpts{Context: ctx},
			sink,
			ownerFilter,
			spenderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tbtcLogger.Warnf(
			"subscription to event Approval had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tbtcLogger.Errorf(
			"subscription to event Approval failed "+
				"with error: [%v]; resubscription attempt will be "+
				"performed",
			err,
		)
	}

	return chainutil.WithResubscription(
		chainutil.SubscriptionBackoffMax,
		subscribeFn,
		chainutil.SubscriptionAlertThreshold,
		thresholdViolatedFn,
		subscriptionFailedFn,
	)
}

func (tbtc *TBTC) PastApprovalEvents(
	startBlock uint64,
	endBlock *uint64,
	ownerFilter []common.Address,
	spenderFilter []common.Address,
) ([]*abi.TBTCApproval, error) {
	iterator, err := tbtc.contract.FilterApproval(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		ownerFilter,
		spenderFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Approval events: [%v]",
			err,
		)
	}

	events := make([]*abi.TBTCApproval, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (tbtc *TBTC) OwnershipTransferredEvent(
	opts *ethereum.SubscribeOpts,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) *TbtcOwnershipTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TbtcOwnershipTransferredSubscription{
		tbtc,
		opts,
		previousOwnerFilter,
		newOwnerFilter,
	}
}

type TbtcOwnershipTransferredSubscription struct {
	contract            *TBTC
	opts                *ethereum.SubscribeOpts
	previousOwnerFilter []common.Address
	newOwnerFilter      []common.Address
}

type tBTCOwnershipTransferredFunc func(
	PreviousOwner common.Address,
	NewOwner common.Address,
	blockNumber uint64,
)

func (ots *TbtcOwnershipTransferredSubscription) OnEvent(
	handler tBTCOwnershipTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TBTCOwnershipTransferred)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.PreviousOwner,
					event.NewOwner,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ots.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ots *TbtcOwnershipTransferredSubscription) Pipe(
	sink chan *abi.TBTCOwnershipTransferred,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ots.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ots.contract.blockCounter.CurrentBlock()
				if err != nil {
					tbtcLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ots.opts.PastBlocks

				tbtcLogger.Infof(
					"subscription monitoring fetching past OwnershipTransferred events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ots.contract.PastOwnershipTransferredEvents(
					fromBlock,
					nil,
					ots.previousOwnerFilter,
					ots.newOwnerFilter,
				)
				if err != nil {
					tbtcLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tbtcLogger.Infof(
					"subscription monitoring fetched [%v] past OwnershipTransferred events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ots.contract.watchOwnershipTransferred(
		sink,
		ots.previousOwnerFilter,
		ots.newOwnerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (tbtc *TBTC) watchOwnershipTransferred(
	sink chan *abi.TBTCOwnershipTransferred,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return tbtc.contract.WatchOwnershipTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
			previousOwnerFilter,
			newOwnerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tbtcLogger.Warnf(
			"subscription to event OwnershipTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tbtcLogger.Errorf(
			"subscription to event OwnershipTransferred failed "+
				"with error: [%v]; resubscription attempt will be "+
				"performed",
			err,
		)
	}

	return chainutil.WithResubscription(
		chainutil.SubscriptionBackoffMax,
		subscribeFn,
		chainutil.SubscriptionAlertThreshold,
		thresholdViolatedFn,
		subscriptionFailedFn,
	)
}

func (tbtc *TBTC) PastOwnershipTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) ([]*abi.TBTCOwnershipTransferred, error) {
	iterator, err := tbtc.contract.FilterOwnershipTransferred(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		previousOwnerFilter,
		newOwnerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past OwnershipTransferred events: [%v]",
			err,
		)
	}

	events := make([]*abi.TBTCOwnershipTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (tbtc *TBTC) TransferEvent(
	opts *ethereum.SubscribeOpts,
	fromFilter []common.Address,
	toFilter []common.Address,
) *TbtcTransferSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TbtcTransferSubscription{
		tbtc,
		opts,
		fromFilter,
		toFilter,
	}
}

type TbtcTransferSubscription struct {
	contract   *TBTC
	opts       *ethereum.SubscribeOpts
	fromFilter []common.Address
	toFilter   []common.Address
}

type tBTCTransferFunc func(
	From common.Address,
	To common.Address,
	Value *big.Int,
	blockNumber uint64,
)

func (ts *TbtcTransferSubscription) OnEvent(
	handler tBTCTransferFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TBTCTransfer)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.From,
					event.To,
					event.Value,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ts.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TbtcTransferSubscription) Pipe(
	sink chan *abi.TBTCTransfer,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ts.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ts.contract.blockCounter.CurrentBlock()
				if err != nil {
					tbtcLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ts.opts.PastBlocks

				tbtcLogger.Infof(
					"subscription monitoring fetching past Transfer events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ts.contract.PastTransferEvents(
					fromBlock,
					nil,
					ts.fromFilter,
					ts.toFilter,
				)
				if err != nil {
					tbtcLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tbtcLogger.Infof(
					"subscription monitoring fetched [%v] past Transfer events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ts.contract.watchTransfer(
		sink,
		ts.fromFilter,
		ts.toFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (tbtc *TBTC) watchTransfer(
	sink chan *abi.TBTCTransfer,
	fromFilter []common.Address,
	toFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return tbtc.contract.WatchTransfer(
			&bind.WatchOpts{Context: ctx},
			sink,
			fromFilter,
			toFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tbtcLogger.Warnf(
			"subscription to event Transfer had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tbtcLogger.Errorf(
			"subscription to event Transfer failed "+
				"with error: [%v]; resubscription attempt will be "+
				"performed",
			err,
		)
	}

	return chainutil.WithResubscription(
		chainutil.SubscriptionBackoffMax,
		subscribeFn,
		chainutil.SubscriptionAlertThreshold,
		thresholdViolatedFn,
		subscriptionFailedFn,
	)
}

func (tbtc *TBTC) PastTransferEvents(
	startBlock uint64,
	endBlock *uint64,
	fromFilter []common.Address,
	toFilter []common.Address,
) ([]*abi.TBTCTransfer, error) {
	iterator, err := tbtc.contract.FilterTransfer(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		fromFilter,
		toFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Transfer events: [%v]",
			err,
		)
	}

	events := make([]*abi.TBTCTransfer, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

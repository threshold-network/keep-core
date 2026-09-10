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

	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	chainutil "github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/subscription"
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var lrmpLogger = log.Logger("keep-contract-LightRelayMaintainerProxy")

type LightRelayMaintainerProxy struct {
	contract          *abi.LightRelayMaintainerProxy
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

func NewLightRelayMaintainerProxy(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*LightRelayMaintainerProxy, error) {
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

	contract, err := abi.NewLightRelayMaintainerProxy(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.LightRelayMaintainerProxyABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &LightRelayMaintainerProxy{
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
func (lrmp *LightRelayMaintainerProxy) Authorize(
	arg_maintainer common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction authorize",
		" params: ",
		fmt.Sprint(
			arg_maintainer,
		),
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.Authorize(
		transactorOptions,
		arg_maintainer,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"authorize",
			arg_maintainer,
		)
	}

	lrmpLogger.Infof(
		"submitted transaction authorize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.Authorize(
				newTransactorOptions,
				arg_maintainer,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"authorize",
					arg_maintainer,
				)
			}

			lrmpLogger.Infof(
				"submitted transaction authorize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallAuthorize(
	arg_maintainer common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"authorize",
		&result,
		arg_maintainer,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) AuthorizeGasEstimate(
	arg_maintainer common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"authorize",
		lrmp.contractABI,
		lrmp.transactor,
		arg_maintainer,
	)

	return result, err
}

// Transaction submission.
func (lrmp *LightRelayMaintainerProxy) Deauthorize(
	arg_maintainer common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction deauthorize",
		" params: ",
		fmt.Sprint(
			arg_maintainer,
		),
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.Deauthorize(
		transactorOptions,
		arg_maintainer,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"deauthorize",
			arg_maintainer,
		)
	}

	lrmpLogger.Infof(
		"submitted transaction deauthorize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.Deauthorize(
				newTransactorOptions,
				arg_maintainer,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"deauthorize",
					arg_maintainer,
				)
			}

			lrmpLogger.Infof(
				"submitted transaction deauthorize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallDeauthorize(
	arg_maintainer common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"deauthorize",
		&result,
		arg_maintainer,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) DeauthorizeGasEstimate(
	arg_maintainer common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"deauthorize",
		lrmp.contractABI,
		lrmp.transactor,
		arg_maintainer,
	)

	return result, err
}

// Transaction submission.
func (lrmp *LightRelayMaintainerProxy) RenounceOwnership(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction renounceOwnership",
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.RenounceOwnership(
		transactorOptions,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"renounceOwnership",
		)
	}

	lrmpLogger.Infof(
		"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.RenounceOwnership(
				newTransactorOptions,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"renounceOwnership",
				)
			}

			lrmpLogger.Infof(
				"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallRenounceOwnership(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"renounceOwnership",
		&result,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) RenounceOwnershipGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"renounceOwnership",
		lrmp.contractABI,
		lrmp.transactor,
	)

	return result, err
}

// Transaction submission.
func (lrmp *LightRelayMaintainerProxy) Retarget(
	arg_headers []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction retarget",
		" params: ",
		fmt.Sprint(
			arg_headers,
		),
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.Retarget(
		transactorOptions,
		arg_headers,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"retarget",
			arg_headers,
		)
	}

	lrmpLogger.Infof(
		"submitted transaction retarget with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.Retarget(
				newTransactorOptions,
				arg_headers,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"retarget",
					arg_headers,
				)
			}

			lrmpLogger.Infof(
				"submitted transaction retarget with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallRetarget(
	arg_headers []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"retarget",
		&result,
		arg_headers,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) RetargetGasEstimate(
	arg_headers []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"retarget",
		lrmp.contractABI,
		lrmp.transactor,
		arg_headers,
	)

	return result, err
}

// Transaction submission.
func (lrmp *LightRelayMaintainerProxy) TransferOwnership(
	arg_newOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction transferOwnership",
		" params: ",
		fmt.Sprint(
			arg_newOwner,
		),
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.TransferOwnership(
		transactorOptions,
		arg_newOwner,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"transferOwnership",
			arg_newOwner,
		)
	}

	lrmpLogger.Infof(
		"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.TransferOwnership(
				newTransactorOptions,
				arg_newOwner,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"transferOwnership",
					arg_newOwner,
				)
			}

			lrmpLogger.Infof(
				"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallTransferOwnership(
	arg_newOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"transferOwnership",
		&result,
		arg_newOwner,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) TransferOwnershipGasEstimate(
	arg_newOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"transferOwnership",
		lrmp.contractABI,
		lrmp.transactor,
		arg_newOwner,
	)

	return result, err
}

// Transaction submission.
func (lrmp *LightRelayMaintainerProxy) UpdateLightRelay(
	arg__lightRelay common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction updateLightRelay",
		" params: ",
		fmt.Sprint(
			arg__lightRelay,
		),
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.UpdateLightRelay(
		transactorOptions,
		arg__lightRelay,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"updateLightRelay",
			arg__lightRelay,
		)
	}

	lrmpLogger.Infof(
		"submitted transaction updateLightRelay with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.UpdateLightRelay(
				newTransactorOptions,
				arg__lightRelay,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"updateLightRelay",
					arg__lightRelay,
				)
			}

			lrmpLogger.Infof(
				"submitted transaction updateLightRelay with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallUpdateLightRelay(
	arg__lightRelay common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"updateLightRelay",
		&result,
		arg__lightRelay,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) UpdateLightRelayGasEstimate(
	arg__lightRelay common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"updateLightRelay",
		lrmp.contractABI,
		lrmp.transactor,
		arg__lightRelay,
	)

	return result, err
}

// Transaction submission.
func (lrmp *LightRelayMaintainerProxy) UpdateReimbursementPool(
	arg__reimbursementPool common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction updateReimbursementPool",
		" params: ",
		fmt.Sprint(
			arg__reimbursementPool,
		),
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.UpdateReimbursementPool(
		transactorOptions,
		arg__reimbursementPool,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"updateReimbursementPool",
			arg__reimbursementPool,
		)
	}

	lrmpLogger.Infof(
		"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.UpdateReimbursementPool(
				newTransactorOptions,
				arg__reimbursementPool,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"updateReimbursementPool",
					arg__reimbursementPool,
				)
			}

			lrmpLogger.Infof(
				"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallUpdateReimbursementPool(
	arg__reimbursementPool common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"updateReimbursementPool",
		&result,
		arg__reimbursementPool,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) UpdateReimbursementPoolGasEstimate(
	arg__reimbursementPool common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"updateReimbursementPool",
		lrmp.contractABI,
		lrmp.transactor,
		arg__reimbursementPool,
	)

	return result, err
}

// Transaction submission.
func (lrmp *LightRelayMaintainerProxy) UpdateRetargetGasOffset(
	arg_newRetargetGasOffset *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrmpLogger.Debug(
		"submitting transaction updateRetargetGasOffset",
		" params: ",
		fmt.Sprint(
			arg_newRetargetGasOffset,
		),
	)

	lrmp.transactionMutex.Lock()
	defer lrmp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lrmp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lrmp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lrmp.contract.UpdateRetargetGasOffset(
		transactorOptions,
		arg_newRetargetGasOffset,
	)
	if err != nil {
		return transaction, lrmp.errorResolver.ResolveError(
			err,
			lrmp.transactorOptions.From,
			nil,
			"updateRetargetGasOffset",
			arg_newRetargetGasOffset,
		)
	}

	lrmpLogger.Infof(
		"submitted transaction updateRetargetGasOffset with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lrmp.miningWaiter.ForceMining(
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

			transaction, err := lrmp.contract.UpdateRetargetGasOffset(
				newTransactorOptions,
				arg_newRetargetGasOffset,
			)
			if err != nil {
				return nil, lrmp.errorResolver.ResolveError(
					err,
					lrmp.transactorOptions.From,
					nil,
					"updateRetargetGasOffset",
					arg_newRetargetGasOffset,
				)
			}

			lrmpLogger.Infof(
				"submitted transaction updateRetargetGasOffset with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lrmp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lrmp *LightRelayMaintainerProxy) CallUpdateRetargetGasOffset(
	arg_newRetargetGasOffset *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lrmp.transactorOptions.From,
		blockNumber, nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"updateRetargetGasOffset",
		&result,
		arg_newRetargetGasOffset,
	)

	return err
}

func (lrmp *LightRelayMaintainerProxy) UpdateRetargetGasOffsetGasEstimate(
	arg_newRetargetGasOffset *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lrmp.callerOptions.From,
		lrmp.contractAddress,
		"updateRetargetGasOffset",
		lrmp.contractABI,
		lrmp.transactor,
		arg_newRetargetGasOffset,
	)

	return result, err
}

// ----- Const Methods ------

func (lrmp *LightRelayMaintainerProxy) IsAuthorized(
	arg0 common.Address,
) (bool, error) {
	result, err := lrmp.contract.IsAuthorized(
		lrmp.callerOptions,
		arg0,
	)

	if err != nil {
		return result, lrmp.errorResolver.ResolveError(
			err,
			lrmp.callerOptions.From,
			nil,
			"isAuthorized",
			arg0,
		)
	}

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) IsAuthorizedAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		lrmp.callerOptions.From,
		blockNumber,
		nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"isAuthorized",
		&result,
		arg0,
	)

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) LightRelay() (common.Address, error) {
	result, err := lrmp.contract.LightRelay(
		lrmp.callerOptions,
	)

	if err != nil {
		return result, lrmp.errorResolver.ResolveError(
			err,
			lrmp.callerOptions.From,
			nil,
			"lightRelay",
		)
	}

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) LightRelayAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		lrmp.callerOptions.From,
		blockNumber,
		nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"lightRelay",
		&result,
	)

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) Owner() (common.Address, error) {
	result, err := lrmp.contract.Owner(
		lrmp.callerOptions,
	)

	if err != nil {
		return result, lrmp.errorResolver.ResolveError(
			err,
			lrmp.callerOptions.From,
			nil,
			"owner",
		)
	}

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) OwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		lrmp.callerOptions.From,
		blockNumber,
		nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"owner",
		&result,
	)

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) ReimbursementPool() (common.Address, error) {
	result, err := lrmp.contract.ReimbursementPool(
		lrmp.callerOptions,
	)

	if err != nil {
		return result, lrmp.errorResolver.ResolveError(
			err,
			lrmp.callerOptions.From,
			nil,
			"reimbursementPool",
		)
	}

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) ReimbursementPoolAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		lrmp.callerOptions.From,
		blockNumber,
		nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"reimbursementPool",
		&result,
	)

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) RetargetGasOffset() (*big.Int, error) {
	result, err := lrmp.contract.RetargetGasOffset(
		lrmp.callerOptions,
	)

	if err != nil {
		return result, lrmp.errorResolver.ResolveError(
			err,
			lrmp.callerOptions.From,
			nil,
			"retargetGasOffset",
		)
	}

	return result, err
}

func (lrmp *LightRelayMaintainerProxy) RetargetGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		lrmp.callerOptions.From,
		blockNumber,
		nil,
		lrmp.contractABI,
		lrmp.caller,
		lrmp.errorResolver,
		lrmp.contractAddress,
		"retargetGasOffset",
		&result,
	)

	return result, err
}

// ------ Events -------

func (lrmp *LightRelayMaintainerProxy) LightRelayUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *LrmpLightRelayUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrmpLightRelayUpdatedSubscription{
		lrmp,
		opts,
	}
}

type LrmpLightRelayUpdatedSubscription struct {
	contract *LightRelayMaintainerProxy
	opts     *ethereum.SubscribeOpts
}

type lightRelayMaintainerProxyLightRelayUpdatedFunc func(
	NewRelay common.Address,
	blockNumber uint64,
)

func (lrus *LrmpLightRelayUpdatedSubscription) OnEvent(
	handler lightRelayMaintainerProxyLightRelayUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayMaintainerProxyLightRelayUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.NewRelay,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := lrus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lrus *LrmpLightRelayUpdatedSubscription) Pipe(
	sink chan *abi.LightRelayMaintainerProxyLightRelayUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(lrus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := lrus.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - lrus.opts.PastBlocks

				lrmpLogger.Infof(
					"subscription monitoring fetching past LightRelayUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := lrus.contract.PastLightRelayUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrmpLogger.Infof(
					"subscription monitoring fetched [%v] past LightRelayUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := lrus.contract.watchLightRelayUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lrmp *LightRelayMaintainerProxy) watchLightRelayUpdated(
	sink chan *abi.LightRelayMaintainerProxyLightRelayUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lrmp.contract.WatchLightRelayUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrmpLogger.Warnf(
			"subscription to event LightRelayUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrmpLogger.Errorf(
			"subscription to event LightRelayUpdated failed "+
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

func (lrmp *LightRelayMaintainerProxy) PastLightRelayUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelayMaintainerProxyLightRelayUpdated, error) {
	iterator, err := lrmp.contract.FilterLightRelayUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past LightRelayUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayMaintainerProxyLightRelayUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lrmp *LightRelayMaintainerProxy) MaintainerAuthorizedEvent(
	opts *ethereum.SubscribeOpts,
	maintainerFilter []common.Address,
) *LrmpMaintainerAuthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrmpMaintainerAuthorizedSubscription{
		lrmp,
		opts,
		maintainerFilter,
	}
}

type LrmpMaintainerAuthorizedSubscription struct {
	contract         *LightRelayMaintainerProxy
	opts             *ethereum.SubscribeOpts
	maintainerFilter []common.Address
}

type lightRelayMaintainerProxyMaintainerAuthorizedFunc func(
	Maintainer common.Address,
	blockNumber uint64,
)

func (mas *LrmpMaintainerAuthorizedSubscription) OnEvent(
	handler lightRelayMaintainerProxyMaintainerAuthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayMaintainerProxyMaintainerAuthorized)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Maintainer,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mas.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mas *LrmpMaintainerAuthorizedSubscription) Pipe(
	sink chan *abi.LightRelayMaintainerProxyMaintainerAuthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mas.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mas.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mas.opts.PastBlocks

				lrmpLogger.Infof(
					"subscription monitoring fetching past MaintainerAuthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mas.contract.PastMaintainerAuthorizedEvents(
					fromBlock,
					nil,
					mas.maintainerFilter,
				)
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrmpLogger.Infof(
					"subscription monitoring fetched [%v] past MaintainerAuthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mas.contract.watchMaintainerAuthorized(
		sink,
		mas.maintainerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lrmp *LightRelayMaintainerProxy) watchMaintainerAuthorized(
	sink chan *abi.LightRelayMaintainerProxyMaintainerAuthorized,
	maintainerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lrmp.contract.WatchMaintainerAuthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
			maintainerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrmpLogger.Warnf(
			"subscription to event MaintainerAuthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrmpLogger.Errorf(
			"subscription to event MaintainerAuthorized failed "+
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

func (lrmp *LightRelayMaintainerProxy) PastMaintainerAuthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
	maintainerFilter []common.Address,
) ([]*abi.LightRelayMaintainerProxyMaintainerAuthorized, error) {
	iterator, err := lrmp.contract.FilterMaintainerAuthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		maintainerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MaintainerAuthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayMaintainerProxyMaintainerAuthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lrmp *LightRelayMaintainerProxy) MaintainerDeauthorizedEvent(
	opts *ethereum.SubscribeOpts,
	maintainerFilter []common.Address,
) *LrmpMaintainerDeauthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrmpMaintainerDeauthorizedSubscription{
		lrmp,
		opts,
		maintainerFilter,
	}
}

type LrmpMaintainerDeauthorizedSubscription struct {
	contract         *LightRelayMaintainerProxy
	opts             *ethereum.SubscribeOpts
	maintainerFilter []common.Address
}

type lightRelayMaintainerProxyMaintainerDeauthorizedFunc func(
	Maintainer common.Address,
	blockNumber uint64,
)

func (mds *LrmpMaintainerDeauthorizedSubscription) OnEvent(
	handler lightRelayMaintainerProxyMaintainerDeauthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayMaintainerProxyMaintainerDeauthorized)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Maintainer,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mds.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mds *LrmpMaintainerDeauthorizedSubscription) Pipe(
	sink chan *abi.LightRelayMaintainerProxyMaintainerDeauthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mds.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mds.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mds.opts.PastBlocks

				lrmpLogger.Infof(
					"subscription monitoring fetching past MaintainerDeauthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mds.contract.PastMaintainerDeauthorizedEvents(
					fromBlock,
					nil,
					mds.maintainerFilter,
				)
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrmpLogger.Infof(
					"subscription monitoring fetched [%v] past MaintainerDeauthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mds.contract.watchMaintainerDeauthorized(
		sink,
		mds.maintainerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lrmp *LightRelayMaintainerProxy) watchMaintainerDeauthorized(
	sink chan *abi.LightRelayMaintainerProxyMaintainerDeauthorized,
	maintainerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lrmp.contract.WatchMaintainerDeauthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
			maintainerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrmpLogger.Warnf(
			"subscription to event MaintainerDeauthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrmpLogger.Errorf(
			"subscription to event MaintainerDeauthorized failed "+
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

func (lrmp *LightRelayMaintainerProxy) PastMaintainerDeauthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
	maintainerFilter []common.Address,
) ([]*abi.LightRelayMaintainerProxyMaintainerDeauthorized, error) {
	iterator, err := lrmp.contract.FilterMaintainerDeauthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		maintainerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MaintainerDeauthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayMaintainerProxyMaintainerDeauthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lrmp *LightRelayMaintainerProxy) OwnershipTransferredEvent(
	opts *ethereum.SubscribeOpts,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) *LrmpOwnershipTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrmpOwnershipTransferredSubscription{
		lrmp,
		opts,
		previousOwnerFilter,
		newOwnerFilter,
	}
}

type LrmpOwnershipTransferredSubscription struct {
	contract            *LightRelayMaintainerProxy
	opts                *ethereum.SubscribeOpts
	previousOwnerFilter []common.Address
	newOwnerFilter      []common.Address
}

type lightRelayMaintainerProxyOwnershipTransferredFunc func(
	PreviousOwner common.Address,
	NewOwner common.Address,
	blockNumber uint64,
)

func (ots *LrmpOwnershipTransferredSubscription) OnEvent(
	handler lightRelayMaintainerProxyOwnershipTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayMaintainerProxyOwnershipTransferred)
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

func (ots *LrmpOwnershipTransferredSubscription) Pipe(
	sink chan *abi.LightRelayMaintainerProxyOwnershipTransferred,
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
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ots.opts.PastBlocks

				lrmpLogger.Infof(
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
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrmpLogger.Infof(
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

func (lrmp *LightRelayMaintainerProxy) watchOwnershipTransferred(
	sink chan *abi.LightRelayMaintainerProxyOwnershipTransferred,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lrmp.contract.WatchOwnershipTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
			previousOwnerFilter,
			newOwnerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrmpLogger.Warnf(
			"subscription to event OwnershipTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrmpLogger.Errorf(
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

func (lrmp *LightRelayMaintainerProxy) PastOwnershipTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) ([]*abi.LightRelayMaintainerProxyOwnershipTransferred, error) {
	iterator, err := lrmp.contract.FilterOwnershipTransferred(
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

	events := make([]*abi.LightRelayMaintainerProxyOwnershipTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lrmp *LightRelayMaintainerProxy) ReimbursementPoolUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *LrmpReimbursementPoolUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrmpReimbursementPoolUpdatedSubscription{
		lrmp,
		opts,
	}
}

type LrmpReimbursementPoolUpdatedSubscription struct {
	contract *LightRelayMaintainerProxy
	opts     *ethereum.SubscribeOpts
}

type lightRelayMaintainerProxyReimbursementPoolUpdatedFunc func(
	NewReimbursementPool common.Address,
	blockNumber uint64,
)

func (rpus *LrmpReimbursementPoolUpdatedSubscription) OnEvent(
	handler lightRelayMaintainerProxyReimbursementPoolUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayMaintainerProxyReimbursementPoolUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.NewReimbursementPool,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rpus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rpus *LrmpReimbursementPoolUpdatedSubscription) Pipe(
	sink chan *abi.LightRelayMaintainerProxyReimbursementPoolUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rpus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rpus.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				lrmpLogger.Infof(
					"subscription monitoring fetching past ReimbursementPoolUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastReimbursementPoolUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrmpLogger.Infof(
					"subscription monitoring fetched [%v] past ReimbursementPoolUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rpus.contract.watchReimbursementPoolUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lrmp *LightRelayMaintainerProxy) watchReimbursementPoolUpdated(
	sink chan *abi.LightRelayMaintainerProxyReimbursementPoolUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lrmp.contract.WatchReimbursementPoolUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrmpLogger.Warnf(
			"subscription to event ReimbursementPoolUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrmpLogger.Errorf(
			"subscription to event ReimbursementPoolUpdated failed "+
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

func (lrmp *LightRelayMaintainerProxy) PastReimbursementPoolUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelayMaintainerProxyReimbursementPoolUpdated, error) {
	iterator, err := lrmp.contract.FilterReimbursementPoolUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReimbursementPoolUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayMaintainerProxyReimbursementPoolUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lrmp *LightRelayMaintainerProxy) RetargetGasOffsetUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *LrmpRetargetGasOffsetUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrmpRetargetGasOffsetUpdatedSubscription{
		lrmp,
		opts,
	}
}

type LrmpRetargetGasOffsetUpdatedSubscription struct {
	contract *LightRelayMaintainerProxy
	opts     *ethereum.SubscribeOpts
}

type lightRelayMaintainerProxyRetargetGasOffsetUpdatedFunc func(
	RetargetGasOffset *big.Int,
	blockNumber uint64,
)

func (rgous *LrmpRetargetGasOffsetUpdatedSubscription) OnEvent(
	handler lightRelayMaintainerProxyRetargetGasOffsetUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayMaintainerProxyRetargetGasOffsetUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RetargetGasOffset,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rgous.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rgous *LrmpRetargetGasOffsetUpdatedSubscription) Pipe(
	sink chan *abi.LightRelayMaintainerProxyRetargetGasOffsetUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rgous.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rgous.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rgous.opts.PastBlocks

				lrmpLogger.Infof(
					"subscription monitoring fetching past RetargetGasOffsetUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rgous.contract.PastRetargetGasOffsetUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrmpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrmpLogger.Infof(
					"subscription monitoring fetched [%v] past RetargetGasOffsetUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rgous.contract.watchRetargetGasOffsetUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lrmp *LightRelayMaintainerProxy) watchRetargetGasOffsetUpdated(
	sink chan *abi.LightRelayMaintainerProxyRetargetGasOffsetUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lrmp.contract.WatchRetargetGasOffsetUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrmpLogger.Warnf(
			"subscription to event RetargetGasOffsetUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrmpLogger.Errorf(
			"subscription to event RetargetGasOffsetUpdated failed "+
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

func (lrmp *LightRelayMaintainerProxy) PastRetargetGasOffsetUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelayMaintainerProxyRetargetGasOffsetUpdated, error) {
	iterator, err := lrmp.contract.FilterRetargetGasOffsetUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RetargetGasOffsetUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayMaintainerProxyRetargetGasOffsetUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

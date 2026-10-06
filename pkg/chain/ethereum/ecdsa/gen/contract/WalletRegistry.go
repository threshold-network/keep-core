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

	"github.com/keep-network/keep-core/pkg/chain/ethereum/ecdsa/gen/abi"
	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	chainutil "github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil"
	"github.com/keep-network/keep-core/pkg/subscription"
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var wrLogger = log.Logger("keep-contract-WalletRegistry")

type WalletRegistry struct {
	contract          *abi.WalletRegistry
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

func NewWalletRegistry(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*WalletRegistry, error) {
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

	contract, err := abi.NewWalletRegistry(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.WalletRegistryABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &WalletRegistry{
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
func (wr *WalletRegistry) ApproveAuthorizationDecrease(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction approveAuthorizationDecrease",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.ApproveAuthorizationDecrease(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"approveAuthorizationDecrease",
			arg_stakingProvider,
		)
	}

	wrLogger.Infof(
		"submitted transaction approveAuthorizationDecrease with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.ApproveAuthorizationDecrease(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"approveAuthorizationDecrease",
					arg_stakingProvider,
				)
			}

			wrLogger.Infof(
				"submitted transaction approveAuthorizationDecrease with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallApproveAuthorizationDecrease(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"approveAuthorizationDecrease",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (wr *WalletRegistry) ApproveAuthorizationDecreaseGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"approveAuthorizationDecrease",
		wr.contractABI,
		wr.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) ApproveDkgResult(
	arg_dkgResult abi.EcdsaDkgResult,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction approveDkgResult",
		" params: ",
		fmt.Sprint(
			arg_dkgResult,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.ApproveDkgResult(
		transactorOptions,
		arg_dkgResult,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"approveDkgResult",
			arg_dkgResult,
		)
	}

	wrLogger.Infof(
		"submitted transaction approveDkgResult with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.ApproveDkgResult(
				newTransactorOptions,
				arg_dkgResult,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"approveDkgResult",
					arg_dkgResult,
				)
			}

			wrLogger.Infof(
				"submitted transaction approveDkgResult with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallApproveDkgResult(
	arg_dkgResult abi.EcdsaDkgResult,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"approveDkgResult",
		&result,
		arg_dkgResult,
	)

	return err
}

func (wr *WalletRegistry) ApproveDkgResultGasEstimate(
	arg_dkgResult abi.EcdsaDkgResult,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"approveDkgResult",
		wr.contractABI,
		wr.transactor,
		arg_dkgResult,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) AuthorizationDecreaseRequested(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction authorizationDecreaseRequested",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.AuthorizationDecreaseRequested(
		transactorOptions,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"authorizationDecreaseRequested",
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
	}

	wrLogger.Infof(
		"submitted transaction authorizationDecreaseRequested with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.AuthorizationDecreaseRequested(
				newTransactorOptions,
				arg_stakingProvider,
				arg_fromAmount,
				arg_toAmount,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"authorizationDecreaseRequested",
					arg_stakingProvider,
					arg_fromAmount,
					arg_toAmount,
				)
			}

			wrLogger.Infof(
				"submitted transaction authorizationDecreaseRequested with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallAuthorizationDecreaseRequested(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"authorizationDecreaseRequested",
		&result,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return err
}

func (wr *WalletRegistry) AuthorizationDecreaseRequestedGasEstimate(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"authorizationDecreaseRequested",
		wr.contractABI,
		wr.transactor,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) AuthorizationIncreased(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction authorizationIncreased",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.AuthorizationIncreased(
		transactorOptions,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"authorizationIncreased",
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
	}

	wrLogger.Infof(
		"submitted transaction authorizationIncreased with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.AuthorizationIncreased(
				newTransactorOptions,
				arg_stakingProvider,
				arg_fromAmount,
				arg_toAmount,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"authorizationIncreased",
					arg_stakingProvider,
					arg_fromAmount,
					arg_toAmount,
				)
			}

			wrLogger.Infof(
				"submitted transaction authorizationIncreased with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallAuthorizationIncreased(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"authorizationIncreased",
		&result,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return err
}

func (wr *WalletRegistry) AuthorizationIncreasedGasEstimate(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"authorizationIncreased",
		wr.contractABI,
		wr.transactor,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) BeaconCallback(
	arg_relayEntry *big.Int,
	arg1 *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction beaconCallback",
		" params: ",
		fmt.Sprint(
			arg_relayEntry,
			arg1,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.BeaconCallback(
		transactorOptions,
		arg_relayEntry,
		arg1,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"beaconCallback",
			arg_relayEntry,
			arg1,
		)
	}

	wrLogger.Infof(
		"submitted transaction beaconCallback with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.BeaconCallback(
				newTransactorOptions,
				arg_relayEntry,
				arg1,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"beaconCallback",
					arg_relayEntry,
					arg1,
				)
			}

			wrLogger.Infof(
				"submitted transaction beaconCallback with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallBeaconCallback(
	arg_relayEntry *big.Int,
	arg1 *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"beaconCallback",
		&result,
		arg_relayEntry,
		arg1,
	)

	return err
}

func (wr *WalletRegistry) BeaconCallbackGasEstimate(
	arg_relayEntry *big.Int,
	arg1 *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"beaconCallback",
		wr.contractABI,
		wr.transactor,
		arg_relayEntry,
		arg1,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) ChallengeDkgResult(
	arg_dkgResult abi.EcdsaDkgResult,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction challengeDkgResult",
		" params: ",
		fmt.Sprint(
			arg_dkgResult,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.ChallengeDkgResult(
		transactorOptions,
		arg_dkgResult,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"challengeDkgResult",
			arg_dkgResult,
		)
	}

	wrLogger.Infof(
		"submitted transaction challengeDkgResult with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.ChallengeDkgResult(
				newTransactorOptions,
				arg_dkgResult,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"challengeDkgResult",
					arg_dkgResult,
				)
			}

			wrLogger.Infof(
				"submitted transaction challengeDkgResult with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallChallengeDkgResult(
	arg_dkgResult abi.EcdsaDkgResult,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"challengeDkgResult",
		&result,
		arg_dkgResult,
	)

	return err
}

func (wr *WalletRegistry) ChallengeDkgResultGasEstimate(
	arg_dkgResult abi.EcdsaDkgResult,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"challengeDkgResult",
		wr.contractABI,
		wr.transactor,
		arg_dkgResult,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) CloseWallet(
	arg_walletID [32]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction closeWallet",
		" params: ",
		fmt.Sprint(
			arg_walletID,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.CloseWallet(
		transactorOptions,
		arg_walletID,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"closeWallet",
			arg_walletID,
		)
	}

	wrLogger.Infof(
		"submitted transaction closeWallet with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.CloseWallet(
				newTransactorOptions,
				arg_walletID,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"closeWallet",
					arg_walletID,
				)
			}

			wrLogger.Infof(
				"submitted transaction closeWallet with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallCloseWallet(
	arg_walletID [32]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"closeWallet",
		&result,
		arg_walletID,
	)

	return err
}

func (wr *WalletRegistry) CloseWalletGasEstimate(
	arg_walletID [32]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"closeWallet",
		wr.contractABI,
		wr.transactor,
		arg_walletID,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) Initialize(
	arg__ecdsaDkgValidator common.Address,
	arg__randomBeacon common.Address,
	arg__reimbursementPool common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction initialize",
		" params: ",
		fmt.Sprint(
			arg__ecdsaDkgValidator,
			arg__randomBeacon,
			arg__reimbursementPool,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.Initialize(
		transactorOptions,
		arg__ecdsaDkgValidator,
		arg__randomBeacon,
		arg__reimbursementPool,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"initialize",
			arg__ecdsaDkgValidator,
			arg__randomBeacon,
			arg__reimbursementPool,
		)
	}

	wrLogger.Infof(
		"submitted transaction initialize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.Initialize(
				newTransactorOptions,
				arg__ecdsaDkgValidator,
				arg__randomBeacon,
				arg__reimbursementPool,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"initialize",
					arg__ecdsaDkgValidator,
					arg__randomBeacon,
					arg__reimbursementPool,
				)
			}

			wrLogger.Infof(
				"submitted transaction initialize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallInitialize(
	arg__ecdsaDkgValidator common.Address,
	arg__randomBeacon common.Address,
	arg__reimbursementPool common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"initialize",
		&result,
		arg__ecdsaDkgValidator,
		arg__randomBeacon,
		arg__reimbursementPool,
	)

	return err
}

func (wr *WalletRegistry) InitializeGasEstimate(
	arg__ecdsaDkgValidator common.Address,
	arg__randomBeacon common.Address,
	arg__reimbursementPool common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"initialize",
		wr.contractABI,
		wr.transactor,
		arg__ecdsaDkgValidator,
		arg__randomBeacon,
		arg__reimbursementPool,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) InvoluntaryAuthorizationDecrease(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction involuntaryAuthorizationDecrease",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.InvoluntaryAuthorizationDecrease(
		transactorOptions,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"involuntaryAuthorizationDecrease",
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
	}

	wrLogger.Infof(
		"submitted transaction involuntaryAuthorizationDecrease with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.InvoluntaryAuthorizationDecrease(
				newTransactorOptions,
				arg_stakingProvider,
				arg_fromAmount,
				arg_toAmount,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"involuntaryAuthorizationDecrease",
					arg_stakingProvider,
					arg_fromAmount,
					arg_toAmount,
				)
			}

			wrLogger.Infof(
				"submitted transaction involuntaryAuthorizationDecrease with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallInvoluntaryAuthorizationDecrease(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"involuntaryAuthorizationDecrease",
		&result,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return err
}

func (wr *WalletRegistry) InvoluntaryAuthorizationDecreaseGasEstimate(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"involuntaryAuthorizationDecrease",
		wr.contractABI,
		wr.transactor,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) JoinSortitionPool(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction joinSortitionPool",
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.JoinSortitionPool(
		transactorOptions,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"joinSortitionPool",
		)
	}

	wrLogger.Infof(
		"submitted transaction joinSortitionPool with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.JoinSortitionPool(
				newTransactorOptions,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"joinSortitionPool",
				)
			}

			wrLogger.Infof(
				"submitted transaction joinSortitionPool with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallJoinSortitionPool(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"joinSortitionPool",
		&result,
	)

	return err
}

func (wr *WalletRegistry) JoinSortitionPoolGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"joinSortitionPool",
		wr.contractABI,
		wr.transactor,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) NotifyDkgTimeout(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction notifyDkgTimeout",
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.NotifyDkgTimeout(
		transactorOptions,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"notifyDkgTimeout",
		)
	}

	wrLogger.Infof(
		"submitted transaction notifyDkgTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.NotifyDkgTimeout(
				newTransactorOptions,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"notifyDkgTimeout",
				)
			}

			wrLogger.Infof(
				"submitted transaction notifyDkgTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallNotifyDkgTimeout(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"notifyDkgTimeout",
		&result,
	)

	return err
}

func (wr *WalletRegistry) NotifyDkgTimeoutGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"notifyDkgTimeout",
		wr.contractABI,
		wr.transactor,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) NotifyOperatorInactivity(
	arg_claim abi.EcdsaInactivityClaim,
	arg_nonce *big.Int,
	arg_groupMembers []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction notifyOperatorInactivity",
		" params: ",
		fmt.Sprint(
			arg_claim,
			arg_nonce,
			arg_groupMembers,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.NotifyOperatorInactivity(
		transactorOptions,
		arg_claim,
		arg_nonce,
		arg_groupMembers,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"notifyOperatorInactivity",
			arg_claim,
			arg_nonce,
			arg_groupMembers,
		)
	}

	wrLogger.Infof(
		"submitted transaction notifyOperatorInactivity with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.NotifyOperatorInactivity(
				newTransactorOptions,
				arg_claim,
				arg_nonce,
				arg_groupMembers,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"notifyOperatorInactivity",
					arg_claim,
					arg_nonce,
					arg_groupMembers,
				)
			}

			wrLogger.Infof(
				"submitted transaction notifyOperatorInactivity with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallNotifyOperatorInactivity(
	arg_claim abi.EcdsaInactivityClaim,
	arg_nonce *big.Int,
	arg_groupMembers []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"notifyOperatorInactivity",
		&result,
		arg_claim,
		arg_nonce,
		arg_groupMembers,
	)

	return err
}

func (wr *WalletRegistry) NotifyOperatorInactivityGasEstimate(
	arg_claim abi.EcdsaInactivityClaim,
	arg_nonce *big.Int,
	arg_groupMembers []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"notifyOperatorInactivity",
		wr.contractABI,
		wr.transactor,
		arg_claim,
		arg_nonce,
		arg_groupMembers,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) NotifySeedTimeout(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction notifySeedTimeout",
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.NotifySeedTimeout(
		transactorOptions,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"notifySeedTimeout",
		)
	}

	wrLogger.Infof(
		"submitted transaction notifySeedTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.NotifySeedTimeout(
				newTransactorOptions,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"notifySeedTimeout",
				)
			}

			wrLogger.Infof(
				"submitted transaction notifySeedTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallNotifySeedTimeout(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"notifySeedTimeout",
		&result,
	)

	return err
}

func (wr *WalletRegistry) NotifySeedTimeoutGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"notifySeedTimeout",
		wr.contractABI,
		wr.transactor,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) RegisterOperator(
	arg_operator common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction registerOperator",
		" params: ",
		fmt.Sprint(
			arg_operator,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.RegisterOperator(
		transactorOptions,
		arg_operator,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"registerOperator",
			arg_operator,
		)
	}

	wrLogger.Infof(
		"submitted transaction registerOperator with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.RegisterOperator(
				newTransactorOptions,
				arg_operator,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"registerOperator",
					arg_operator,
				)
			}

			wrLogger.Infof(
				"submitted transaction registerOperator with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallRegisterOperator(
	arg_operator common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"registerOperator",
		&result,
		arg_operator,
	)

	return err
}

func (wr *WalletRegistry) RegisterOperatorGasEstimate(
	arg_operator common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"registerOperator",
		wr.contractABI,
		wr.transactor,
		arg_operator,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) RequestNewWallet(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction requestNewWallet",
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.RequestNewWallet(
		transactorOptions,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"requestNewWallet",
		)
	}

	wrLogger.Infof(
		"submitted transaction requestNewWallet with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.RequestNewWallet(
				newTransactorOptions,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"requestNewWallet",
				)
			}

			wrLogger.Infof(
				"submitted transaction requestNewWallet with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallRequestNewWallet(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"requestNewWallet",
		&result,
	)

	return err
}

func (wr *WalletRegistry) RequestNewWalletGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"requestNewWallet",
		wr.contractABI,
		wr.transactor,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) Seize(
	arg_amount *big.Int,
	arg_rewardMultiplier *big.Int,
	arg_notifier common.Address,
	arg_walletID [32]byte,
	arg_walletMembersIDs []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction seize",
		" params: ",
		fmt.Sprint(
			arg_amount,
			arg_rewardMultiplier,
			arg_notifier,
			arg_walletID,
			arg_walletMembersIDs,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.Seize(
		transactorOptions,
		arg_amount,
		arg_rewardMultiplier,
		arg_notifier,
		arg_walletID,
		arg_walletMembersIDs,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"seize",
			arg_amount,
			arg_rewardMultiplier,
			arg_notifier,
			arg_walletID,
			arg_walletMembersIDs,
		)
	}

	wrLogger.Infof(
		"submitted transaction seize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.Seize(
				newTransactorOptions,
				arg_amount,
				arg_rewardMultiplier,
				arg_notifier,
				arg_walletID,
				arg_walletMembersIDs,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"seize",
					arg_amount,
					arg_rewardMultiplier,
					arg_notifier,
					arg_walletID,
					arg_walletMembersIDs,
				)
			}

			wrLogger.Infof(
				"submitted transaction seize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallSeize(
	arg_amount *big.Int,
	arg_rewardMultiplier *big.Int,
	arg_notifier common.Address,
	arg_walletID [32]byte,
	arg_walletMembersIDs []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"seize",
		&result,
		arg_amount,
		arg_rewardMultiplier,
		arg_notifier,
		arg_walletID,
		arg_walletMembersIDs,
	)

	return err
}

func (wr *WalletRegistry) SeizeGasEstimate(
	arg_amount *big.Int,
	arg_rewardMultiplier *big.Int,
	arg_notifier common.Address,
	arg_walletID [32]byte,
	arg_walletMembersIDs []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"seize",
		wr.contractABI,
		wr.transactor,
		arg_amount,
		arg_rewardMultiplier,
		arg_notifier,
		arg_walletID,
		arg_walletMembersIDs,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) SubmitDkgResult(
	arg_dkgResult abi.EcdsaDkgResult,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction submitDkgResult",
		" params: ",
		fmt.Sprint(
			arg_dkgResult,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.SubmitDkgResult(
		transactorOptions,
		arg_dkgResult,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"submitDkgResult",
			arg_dkgResult,
		)
	}

	wrLogger.Infof(
		"submitted transaction submitDkgResult with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.SubmitDkgResult(
				newTransactorOptions,
				arg_dkgResult,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"submitDkgResult",
					arg_dkgResult,
				)
			}

			wrLogger.Infof(
				"submitted transaction submitDkgResult with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallSubmitDkgResult(
	arg_dkgResult abi.EcdsaDkgResult,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"submitDkgResult",
		&result,
		arg_dkgResult,
	)

	return err
}

func (wr *WalletRegistry) SubmitDkgResultGasEstimate(
	arg_dkgResult abi.EcdsaDkgResult,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"submitDkgResult",
		wr.contractABI,
		wr.transactor,
		arg_dkgResult,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) TransferGovernance(
	arg_newGovernance common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction transferGovernance",
		" params: ",
		fmt.Sprint(
			arg_newGovernance,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.TransferGovernance(
		transactorOptions,
		arg_newGovernance,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"transferGovernance",
			arg_newGovernance,
		)
	}

	wrLogger.Infof(
		"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.TransferGovernance(
				newTransactorOptions,
				arg_newGovernance,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"transferGovernance",
					arg_newGovernance,
				)
			}

			wrLogger.Infof(
				"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallTransferGovernance(
	arg_newGovernance common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"transferGovernance",
		&result,
		arg_newGovernance,
	)

	return err
}

func (wr *WalletRegistry) TransferGovernanceGasEstimate(
	arg_newGovernance common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"transferGovernance",
		wr.contractABI,
		wr.transactor,
		arg_newGovernance,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateAuthorizationParameters(
	arg__minimumAuthorization *big.Int,
	arg__authorizationDecreaseDelay uint64,
	arg__authorizationDecreaseChangePeriod uint64,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateAuthorizationParameters",
		" params: ",
		fmt.Sprint(
			arg__minimumAuthorization,
			arg__authorizationDecreaseDelay,
			arg__authorizationDecreaseChangePeriod,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateAuthorizationParameters(
		transactorOptions,
		arg__minimumAuthorization,
		arg__authorizationDecreaseDelay,
		arg__authorizationDecreaseChangePeriod,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateAuthorizationParameters",
			arg__minimumAuthorization,
			arg__authorizationDecreaseDelay,
			arg__authorizationDecreaseChangePeriod,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateAuthorizationParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateAuthorizationParameters(
				newTransactorOptions,
				arg__minimumAuthorization,
				arg__authorizationDecreaseDelay,
				arg__authorizationDecreaseChangePeriod,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateAuthorizationParameters",
					arg__minimumAuthorization,
					arg__authorizationDecreaseDelay,
					arg__authorizationDecreaseChangePeriod,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateAuthorizationParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateAuthorizationParameters(
	arg__minimumAuthorization *big.Int,
	arg__authorizationDecreaseDelay uint64,
	arg__authorizationDecreaseChangePeriod uint64,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateAuthorizationParameters",
		&result,
		arg__minimumAuthorization,
		arg__authorizationDecreaseDelay,
		arg__authorizationDecreaseChangePeriod,
	)

	return err
}

func (wr *WalletRegistry) UpdateAuthorizationParametersGasEstimate(
	arg__minimumAuthorization *big.Int,
	arg__authorizationDecreaseDelay uint64,
	arg__authorizationDecreaseChangePeriod uint64,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateAuthorizationParameters",
		wr.contractABI,
		wr.transactor,
		arg__minimumAuthorization,
		arg__authorizationDecreaseDelay,
		arg__authorizationDecreaseChangePeriod,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateDkgParameters(
	arg__seedTimeout *big.Int,
	arg__resultChallengePeriodLength *big.Int,
	arg__resultChallengeExtraGas *big.Int,
	arg__resultSubmissionTimeout *big.Int,
	arg__submitterPrecedencePeriodLength *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateDkgParameters",
		" params: ",
		fmt.Sprint(
			arg__seedTimeout,
			arg__resultChallengePeriodLength,
			arg__resultChallengeExtraGas,
			arg__resultSubmissionTimeout,
			arg__submitterPrecedencePeriodLength,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateDkgParameters(
		transactorOptions,
		arg__seedTimeout,
		arg__resultChallengePeriodLength,
		arg__resultChallengeExtraGas,
		arg__resultSubmissionTimeout,
		arg__submitterPrecedencePeriodLength,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateDkgParameters",
			arg__seedTimeout,
			arg__resultChallengePeriodLength,
			arg__resultChallengeExtraGas,
			arg__resultSubmissionTimeout,
			arg__submitterPrecedencePeriodLength,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateDkgParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateDkgParameters(
				newTransactorOptions,
				arg__seedTimeout,
				arg__resultChallengePeriodLength,
				arg__resultChallengeExtraGas,
				arg__resultSubmissionTimeout,
				arg__submitterPrecedencePeriodLength,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateDkgParameters",
					arg__seedTimeout,
					arg__resultChallengePeriodLength,
					arg__resultChallengeExtraGas,
					arg__resultSubmissionTimeout,
					arg__submitterPrecedencePeriodLength,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateDkgParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateDkgParameters(
	arg__seedTimeout *big.Int,
	arg__resultChallengePeriodLength *big.Int,
	arg__resultChallengeExtraGas *big.Int,
	arg__resultSubmissionTimeout *big.Int,
	arg__submitterPrecedencePeriodLength *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateDkgParameters",
		&result,
		arg__seedTimeout,
		arg__resultChallengePeriodLength,
		arg__resultChallengeExtraGas,
		arg__resultSubmissionTimeout,
		arg__submitterPrecedencePeriodLength,
	)

	return err
}

func (wr *WalletRegistry) UpdateDkgParametersGasEstimate(
	arg__seedTimeout *big.Int,
	arg__resultChallengePeriodLength *big.Int,
	arg__resultChallengeExtraGas *big.Int,
	arg__resultSubmissionTimeout *big.Int,
	arg__submitterPrecedencePeriodLength *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateDkgParameters",
		wr.contractABI,
		wr.transactor,
		arg__seedTimeout,
		arg__resultChallengePeriodLength,
		arg__resultChallengeExtraGas,
		arg__resultSubmissionTimeout,
		arg__submitterPrecedencePeriodLength,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateGasParameters(
	arg_dkgResultSubmissionGas *big.Int,
	arg_dkgResultApprovalGasOffset *big.Int,
	arg_notifyOperatorInactivityGasOffset *big.Int,
	arg_notifySeedTimeoutGasOffset *big.Int,
	arg_notifyDkgTimeoutNegativeGasOffset *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateGasParameters",
		" params: ",
		fmt.Sprint(
			arg_dkgResultSubmissionGas,
			arg_dkgResultApprovalGasOffset,
			arg_notifyOperatorInactivityGasOffset,
			arg_notifySeedTimeoutGasOffset,
			arg_notifyDkgTimeoutNegativeGasOffset,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateGasParameters(
		transactorOptions,
		arg_dkgResultSubmissionGas,
		arg_dkgResultApprovalGasOffset,
		arg_notifyOperatorInactivityGasOffset,
		arg_notifySeedTimeoutGasOffset,
		arg_notifyDkgTimeoutNegativeGasOffset,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateGasParameters",
			arg_dkgResultSubmissionGas,
			arg_dkgResultApprovalGasOffset,
			arg_notifyOperatorInactivityGasOffset,
			arg_notifySeedTimeoutGasOffset,
			arg_notifyDkgTimeoutNegativeGasOffset,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateGasParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateGasParameters(
				newTransactorOptions,
				arg_dkgResultSubmissionGas,
				arg_dkgResultApprovalGasOffset,
				arg_notifyOperatorInactivityGasOffset,
				arg_notifySeedTimeoutGasOffset,
				arg_notifyDkgTimeoutNegativeGasOffset,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateGasParameters",
					arg_dkgResultSubmissionGas,
					arg_dkgResultApprovalGasOffset,
					arg_notifyOperatorInactivityGasOffset,
					arg_notifySeedTimeoutGasOffset,
					arg_notifyDkgTimeoutNegativeGasOffset,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateGasParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateGasParameters(
	arg_dkgResultSubmissionGas *big.Int,
	arg_dkgResultApprovalGasOffset *big.Int,
	arg_notifyOperatorInactivityGasOffset *big.Int,
	arg_notifySeedTimeoutGasOffset *big.Int,
	arg_notifyDkgTimeoutNegativeGasOffset *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateGasParameters",
		&result,
		arg_dkgResultSubmissionGas,
		arg_dkgResultApprovalGasOffset,
		arg_notifyOperatorInactivityGasOffset,
		arg_notifySeedTimeoutGasOffset,
		arg_notifyDkgTimeoutNegativeGasOffset,
	)

	return err
}

func (wr *WalletRegistry) UpdateGasParametersGasEstimate(
	arg_dkgResultSubmissionGas *big.Int,
	arg_dkgResultApprovalGasOffset *big.Int,
	arg_notifyOperatorInactivityGasOffset *big.Int,
	arg_notifySeedTimeoutGasOffset *big.Int,
	arg_notifyDkgTimeoutNegativeGasOffset *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateGasParameters",
		wr.contractABI,
		wr.transactor,
		arg_dkgResultSubmissionGas,
		arg_dkgResultApprovalGasOffset,
		arg_notifyOperatorInactivityGasOffset,
		arg_notifySeedTimeoutGasOffset,
		arg_notifyDkgTimeoutNegativeGasOffset,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateOperatorStatus(
	arg_operator common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateOperatorStatus",
		" params: ",
		fmt.Sprint(
			arg_operator,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateOperatorStatus(
		transactorOptions,
		arg_operator,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateOperatorStatus",
			arg_operator,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateOperatorStatus with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateOperatorStatus(
				newTransactorOptions,
				arg_operator,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateOperatorStatus",
					arg_operator,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateOperatorStatus with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateOperatorStatus(
	arg_operator common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateOperatorStatus",
		&result,
		arg_operator,
	)

	return err
}

func (wr *WalletRegistry) UpdateOperatorStatusGasEstimate(
	arg_operator common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateOperatorStatus",
		wr.contractABI,
		wr.transactor,
		arg_operator,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateReimbursementPool(
	arg__reimbursementPool common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateReimbursementPool",
		" params: ",
		fmt.Sprint(
			arg__reimbursementPool,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateReimbursementPool(
		transactorOptions,
		arg__reimbursementPool,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateReimbursementPool",
			arg__reimbursementPool,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateReimbursementPool(
				newTransactorOptions,
				arg__reimbursementPool,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateReimbursementPool",
					arg__reimbursementPool,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateReimbursementPool(
	arg__reimbursementPool common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateReimbursementPool",
		&result,
		arg__reimbursementPool,
	)

	return err
}

func (wr *WalletRegistry) UpdateReimbursementPoolGasEstimate(
	arg__reimbursementPool common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateReimbursementPool",
		wr.contractABI,
		wr.transactor,
		arg__reimbursementPool,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateRewardParameters(
	arg_maliciousDkgResultNotificationRewardMultiplier *big.Int,
	arg_sortitionPoolRewardsBanDuration *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateRewardParameters",
		" params: ",
		fmt.Sprint(
			arg_maliciousDkgResultNotificationRewardMultiplier,
			arg_sortitionPoolRewardsBanDuration,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateRewardParameters(
		transactorOptions,
		arg_maliciousDkgResultNotificationRewardMultiplier,
		arg_sortitionPoolRewardsBanDuration,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateRewardParameters",
			arg_maliciousDkgResultNotificationRewardMultiplier,
			arg_sortitionPoolRewardsBanDuration,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateRewardParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateRewardParameters(
				newTransactorOptions,
				arg_maliciousDkgResultNotificationRewardMultiplier,
				arg_sortitionPoolRewardsBanDuration,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateRewardParameters",
					arg_maliciousDkgResultNotificationRewardMultiplier,
					arg_sortitionPoolRewardsBanDuration,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateRewardParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateRewardParameters(
	arg_maliciousDkgResultNotificationRewardMultiplier *big.Int,
	arg_sortitionPoolRewardsBanDuration *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateRewardParameters",
		&result,
		arg_maliciousDkgResultNotificationRewardMultiplier,
		arg_sortitionPoolRewardsBanDuration,
	)

	return err
}

func (wr *WalletRegistry) UpdateRewardParametersGasEstimate(
	arg_maliciousDkgResultNotificationRewardMultiplier *big.Int,
	arg_sortitionPoolRewardsBanDuration *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateRewardParameters",
		wr.contractABI,
		wr.transactor,
		arg_maliciousDkgResultNotificationRewardMultiplier,
		arg_sortitionPoolRewardsBanDuration,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateSlashingParameters(
	arg_maliciousDkgResultSlashingAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateSlashingParameters",
		" params: ",
		fmt.Sprint(
			arg_maliciousDkgResultSlashingAmount,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateSlashingParameters(
		transactorOptions,
		arg_maliciousDkgResultSlashingAmount,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateSlashingParameters",
			arg_maliciousDkgResultSlashingAmount,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateSlashingParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateSlashingParameters(
				newTransactorOptions,
				arg_maliciousDkgResultSlashingAmount,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateSlashingParameters",
					arg_maliciousDkgResultSlashingAmount,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateSlashingParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateSlashingParameters(
	arg_maliciousDkgResultSlashingAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateSlashingParameters",
		&result,
		arg_maliciousDkgResultSlashingAmount,
	)

	return err
}

func (wr *WalletRegistry) UpdateSlashingParametersGasEstimate(
	arg_maliciousDkgResultSlashingAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateSlashingParameters",
		wr.contractABI,
		wr.transactor,
		arg_maliciousDkgResultSlashingAmount,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpdateWalletOwner(
	arg__walletOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction updateWalletOwner",
		" params: ",
		fmt.Sprint(
			arg__walletOwner,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpdateWalletOwner(
		transactorOptions,
		arg__walletOwner,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"updateWalletOwner",
			arg__walletOwner,
		)
	}

	wrLogger.Infof(
		"submitted transaction updateWalletOwner with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpdateWalletOwner(
				newTransactorOptions,
				arg__walletOwner,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"updateWalletOwner",
					arg__walletOwner,
				)
			}

			wrLogger.Infof(
				"submitted transaction updateWalletOwner with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpdateWalletOwner(
	arg__walletOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"updateWalletOwner",
		&result,
		arg__walletOwner,
	)

	return err
}

func (wr *WalletRegistry) UpdateWalletOwnerGasEstimate(
	arg__walletOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"updateWalletOwner",
		wr.contractABI,
		wr.transactor,
		arg__walletOwner,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) UpgradeRandomBeacon(
	arg__randomBeacon common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction upgradeRandomBeacon",
		" params: ",
		fmt.Sprint(
			arg__randomBeacon,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.UpgradeRandomBeacon(
		transactorOptions,
		arg__randomBeacon,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"upgradeRandomBeacon",
			arg__randomBeacon,
		)
	}

	wrLogger.Infof(
		"submitted transaction upgradeRandomBeacon with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.UpgradeRandomBeacon(
				newTransactorOptions,
				arg__randomBeacon,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"upgradeRandomBeacon",
					arg__randomBeacon,
				)
			}

			wrLogger.Infof(
				"submitted transaction upgradeRandomBeacon with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallUpgradeRandomBeacon(
	arg__randomBeacon common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"upgradeRandomBeacon",
		&result,
		arg__randomBeacon,
	)

	return err
}

func (wr *WalletRegistry) UpgradeRandomBeaconGasEstimate(
	arg__randomBeacon common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"upgradeRandomBeacon",
		wr.contractABI,
		wr.transactor,
		arg__randomBeacon,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) WithdrawIneligibleRewards(
	arg_recipient common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction withdrawIneligibleRewards",
		" params: ",
		fmt.Sprint(
			arg_recipient,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.WithdrawIneligibleRewards(
		transactorOptions,
		arg_recipient,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"withdrawIneligibleRewards",
			arg_recipient,
		)
	}

	wrLogger.Infof(
		"submitted transaction withdrawIneligibleRewards with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.WithdrawIneligibleRewards(
				newTransactorOptions,
				arg_recipient,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"withdrawIneligibleRewards",
					arg_recipient,
				)
			}

			wrLogger.Infof(
				"submitted transaction withdrawIneligibleRewards with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallWithdrawIneligibleRewards(
	arg_recipient common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"withdrawIneligibleRewards",
		&result,
		arg_recipient,
	)

	return err
}

func (wr *WalletRegistry) WithdrawIneligibleRewardsGasEstimate(
	arg_recipient common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"withdrawIneligibleRewards",
		wr.contractABI,
		wr.transactor,
		arg_recipient,
	)

	return result, err
}

// Transaction submission.
func (wr *WalletRegistry) WithdrawRewards(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	wrLogger.Debug(
		"submitting transaction withdrawRewards",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	wr.transactionMutex.Lock()
	defer wr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *wr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := wr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := wr.contract.WithdrawRewards(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, wr.errorResolver.ResolveError(
			err,
			wr.transactorOptions.From,
			nil,
			"withdrawRewards",
			arg_stakingProvider,
		)
	}

	wrLogger.Infof(
		"submitted transaction withdrawRewards with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go wr.miningWaiter.ForceMining(
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

			transaction, err := wr.contract.WithdrawRewards(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, wr.errorResolver.ResolveError(
					err,
					wr.transactorOptions.From,
					nil,
					"withdrawRewards",
					arg_stakingProvider,
				)
			}

			wrLogger.Infof(
				"submitted transaction withdrawRewards with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	wr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (wr *WalletRegistry) CallWithdrawRewards(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		wr.transactorOptions.From,
		blockNumber, nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"withdrawRewards",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (wr *WalletRegistry) WithdrawRewardsGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		wr.callerOptions.From,
		wr.contractAddress,
		"withdrawRewards",
		wr.contractABI,
		wr.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// ----- Const Methods ------

type authorizationParameters struct {
	MinimumAuthorization              *big.Int
	AuthorizationDecreaseDelay        uint64
	AuthorizationDecreaseChangePeriod uint64
}

func (wr *WalletRegistry) AuthorizationParameters() (authorizationParameters, error) {
	result, err := wr.contract.AuthorizationParameters(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"authorizationParameters",
		)
	}

	return result, err
}

func (wr *WalletRegistry) AuthorizationParametersAtBlock(
	blockNumber *big.Int,
) (authorizationParameters, error) {
	var result authorizationParameters

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"authorizationParameters",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) AvailableRewards(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := wr.contract.AvailableRewards(
		wr.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"availableRewards",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (wr *WalletRegistry) AvailableRewardsAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"availableRewards",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (wr *WalletRegistry) DkgParameters() (abi.EcdsaDkgParameters, error) {
	result, err := wr.contract.DkgParameters(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"dkgParameters",
		)
	}

	return result, err
}

func (wr *WalletRegistry) DkgParametersAtBlock(
	blockNumber *big.Int,
) (abi.EcdsaDkgParameters, error) {
	var result abi.EcdsaDkgParameters

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"dkgParameters",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) EligibleStake(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := wr.contract.EligibleStake(
		wr.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"eligibleStake",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (wr *WalletRegistry) EligibleStakeAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"eligibleStake",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

type gasParameters struct {
	DkgResultSubmissionGas            *big.Int
	DkgResultApprovalGasOffset        *big.Int
	NotifyOperatorInactivityGasOffset *big.Int
	NotifySeedTimeoutGasOffset        *big.Int
	NotifyDkgTimeoutNegativeGasOffset *big.Int
}

func (wr *WalletRegistry) GasParameters() (gasParameters, error) {
	result, err := wr.contract.GasParameters(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"gasParameters",
		)
	}

	return result, err
}

func (wr *WalletRegistry) GasParametersAtBlock(
	blockNumber *big.Int,
) (gasParameters, error) {
	var result gasParameters

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"gasParameters",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) GetWallet(
	arg_walletID [32]byte,
) (abi.WalletsWallet, error) {
	result, err := wr.contract.GetWallet(
		wr.callerOptions,
		arg_walletID,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"getWallet",
			arg_walletID,
		)
	}

	return result, err
}

func (wr *WalletRegistry) GetWalletAtBlock(
	arg_walletID [32]byte,
	blockNumber *big.Int,
) (abi.WalletsWallet, error) {
	var result abi.WalletsWallet

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"getWallet",
		&result,
		arg_walletID,
	)

	return result, err
}

func (wr *WalletRegistry) GetWalletCreationState() (uint8, error) {
	result, err := wr.contract.GetWalletCreationState(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"getWalletCreationState",
		)
	}

	return result, err
}

func (wr *WalletRegistry) GetWalletCreationStateAtBlock(
	blockNumber *big.Int,
) (uint8, error) {
	var result uint8

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"getWalletCreationState",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) GetWalletPublicKey(
	arg_walletID [32]byte,
) ([]byte, error) {
	result, err := wr.contract.GetWalletPublicKey(
		wr.callerOptions,
		arg_walletID,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"getWalletPublicKey",
			arg_walletID,
		)
	}

	return result, err
}

func (wr *WalletRegistry) GetWalletPublicKeyAtBlock(
	arg_walletID [32]byte,
	blockNumber *big.Int,
) ([]byte, error) {
	var result []byte

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"getWalletPublicKey",
		&result,
		arg_walletID,
	)

	return result, err
}

func (wr *WalletRegistry) Governance() (common.Address, error) {
	result, err := wr.contract.Governance(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"governance",
		)
	}

	return result, err
}

func (wr *WalletRegistry) GovernanceAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"governance",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) HasDkgTimedOut() (bool, error) {
	result, err := wr.contract.HasDkgTimedOut(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"hasDkgTimedOut",
		)
	}

	return result, err
}

func (wr *WalletRegistry) HasDkgTimedOutAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"hasDkgTimedOut",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) HasSeedTimedOut() (bool, error) {
	result, err := wr.contract.HasSeedTimedOut(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"hasSeedTimedOut",
		)
	}

	return result, err
}

func (wr *WalletRegistry) HasSeedTimedOutAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"hasSeedTimedOut",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) InactivityClaimNonce(
	arg0 [32]byte,
) (*big.Int, error) {
	result, err := wr.contract.InactivityClaimNonce(
		wr.callerOptions,
		arg0,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"inactivityClaimNonce",
			arg0,
		)
	}

	return result, err
}

func (wr *WalletRegistry) InactivityClaimNonceAtBlock(
	arg0 [32]byte,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"inactivityClaimNonce",
		&result,
		arg0,
	)

	return result, err
}

type isDkgResultValid struct {
	bool
	string
}

func (wr *WalletRegistry) IsDkgResultValid(
	arg_result abi.EcdsaDkgResult,
) (isDkgResultValid, error) {
	ret0, ret1, err := wr.contract.IsDkgResultValid(
		wr.callerOptions,
		arg_result,
	)

	result := isDkgResultValid{ret0, ret1}

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"isDkgResultValid",
			arg_result,
		)
	}

	return result, err
}

func (wr *WalletRegistry) IsDkgResultValidAtBlock(
	arg_result abi.EcdsaDkgResult,
	blockNumber *big.Int,
) (isDkgResultValid, error) {
	var result isDkgResultValid

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"isDkgResultValid",
		&result,
		arg_result,
	)

	return result, err
}

func (wr *WalletRegistry) IsOperatorInPool(
	arg_operator common.Address,
) (bool, error) {
	result, err := wr.contract.IsOperatorInPool(
		wr.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"isOperatorInPool",
			arg_operator,
		)
	}

	return result, err
}

func (wr *WalletRegistry) IsOperatorInPoolAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"isOperatorInPool",
		&result,
		arg_operator,
	)

	return result, err
}

func (wr *WalletRegistry) IsOperatorUpToDate(
	arg_operator common.Address,
) (bool, error) {
	result, err := wr.contract.IsOperatorUpToDate(
		wr.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"isOperatorUpToDate",
			arg_operator,
		)
	}

	return result, err
}

func (wr *WalletRegistry) IsOperatorUpToDateAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"isOperatorUpToDate",
		&result,
		arg_operator,
	)

	return result, err
}

func (wr *WalletRegistry) IsWalletMember(
	arg_walletID [32]byte,
	arg_walletMembersIDs []uint32,
	arg_operator common.Address,
	arg_walletMemberIndex *big.Int,
) (bool, error) {
	result, err := wr.contract.IsWalletMember(
		wr.callerOptions,
		arg_walletID,
		arg_walletMembersIDs,
		arg_operator,
		arg_walletMemberIndex,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"isWalletMember",
			arg_walletID,
			arg_walletMembersIDs,
			arg_operator,
			arg_walletMemberIndex,
		)
	}

	return result, err
}

func (wr *WalletRegistry) IsWalletMemberAtBlock(
	arg_walletID [32]byte,
	arg_walletMembersIDs []uint32,
	arg_operator common.Address,
	arg_walletMemberIndex *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"isWalletMember",
		&result,
		arg_walletID,
		arg_walletMembersIDs,
		arg_operator,
		arg_walletMemberIndex,
	)

	return result, err
}

func (wr *WalletRegistry) IsWalletRegistered(
	arg_walletID [32]byte,
) (bool, error) {
	result, err := wr.contract.IsWalletRegistered(
		wr.callerOptions,
		arg_walletID,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"isWalletRegistered",
			arg_walletID,
		)
	}

	return result, err
}

func (wr *WalletRegistry) IsWalletRegisteredAtBlock(
	arg_walletID [32]byte,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"isWalletRegistered",
		&result,
		arg_walletID,
	)

	return result, err
}

func (wr *WalletRegistry) MinimumAuthorization() (*big.Int, error) {
	result, err := wr.contract.MinimumAuthorization(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"minimumAuthorization",
		)
	}

	return result, err
}

func (wr *WalletRegistry) MinimumAuthorizationAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"minimumAuthorization",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) OperatorToStakingProvider(
	arg_operator common.Address,
) (common.Address, error) {
	result, err := wr.contract.OperatorToStakingProvider(
		wr.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"operatorToStakingProvider",
			arg_operator,
		)
	}

	return result, err
}

func (wr *WalletRegistry) OperatorToStakingProviderAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"operatorToStakingProvider",
		&result,
		arg_operator,
	)

	return result, err
}

func (wr *WalletRegistry) PendingAuthorizationDecrease(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := wr.contract.PendingAuthorizationDecrease(
		wr.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"pendingAuthorizationDecrease",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (wr *WalletRegistry) PendingAuthorizationDecreaseAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"pendingAuthorizationDecrease",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (wr *WalletRegistry) RandomBeacon() (common.Address, error) {
	result, err := wr.contract.RandomBeacon(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"randomBeacon",
		)
	}

	return result, err
}

func (wr *WalletRegistry) RandomBeaconAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"randomBeacon",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) ReimbursementPool() (common.Address, error) {
	result, err := wr.contract.ReimbursementPool(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"reimbursementPool",
		)
	}

	return result, err
}

func (wr *WalletRegistry) ReimbursementPoolAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"reimbursementPool",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) RemainingAuthorizationDecreaseDelay(
	arg_stakingProvider common.Address,
) (uint64, error) {
	result, err := wr.contract.RemainingAuthorizationDecreaseDelay(
		wr.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"remainingAuthorizationDecreaseDelay",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (wr *WalletRegistry) RemainingAuthorizationDecreaseDelayAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"remainingAuthorizationDecreaseDelay",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

type rewardParameters struct {
	MaliciousDkgResultNotificationRewardMultiplier *big.Int
	SortitionPoolRewardsBanDuration                *big.Int
}

func (wr *WalletRegistry) RewardParameters() (rewardParameters, error) {
	result, err := wr.contract.RewardParameters(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"rewardParameters",
		)
	}

	return result, err
}

func (wr *WalletRegistry) RewardParametersAtBlock(
	blockNumber *big.Int,
) (rewardParameters, error) {
	var result rewardParameters

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"rewardParameters",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) SelectGroup() ([]uint32, error) {
	result, err := wr.contract.SelectGroup(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"selectGroup",
		)
	}

	return result, err
}

func (wr *WalletRegistry) SelectGroupAtBlock(
	blockNumber *big.Int,
) ([]uint32, error) {
	var result []uint32

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"selectGroup",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) SlashingParameters() (*big.Int, error) {
	result, err := wr.contract.SlashingParameters(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"slashingParameters",
		)
	}

	return result, err
}

func (wr *WalletRegistry) SlashingParametersAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"slashingParameters",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) SortitionPool() (common.Address, error) {
	result, err := wr.contract.SortitionPool(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"sortitionPool",
		)
	}

	return result, err
}

func (wr *WalletRegistry) SortitionPoolAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"sortitionPool",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) Staking() (common.Address, error) {
	result, err := wr.contract.Staking(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"staking",
		)
	}

	return result, err
}

func (wr *WalletRegistry) StakingAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"staking",
		&result,
	)

	return result, err
}

func (wr *WalletRegistry) StakingProviderToOperator(
	arg_stakingProvider common.Address,
) (common.Address, error) {
	result, err := wr.contract.StakingProviderToOperator(
		wr.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"stakingProviderToOperator",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (wr *WalletRegistry) StakingProviderToOperatorAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"stakingProviderToOperator",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (wr *WalletRegistry) WalletOwner() (common.Address, error) {
	result, err := wr.contract.WalletOwner(
		wr.callerOptions,
	)

	if err != nil {
		return result, wr.errorResolver.ResolveError(
			err,
			wr.callerOptions.From,
			nil,
			"walletOwner",
		)
	}

	return result, err
}

func (wr *WalletRegistry) WalletOwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		wr.callerOptions.From,
		blockNumber,
		nil,
		wr.contractABI,
		wr.caller,
		wr.errorResolver,
		wr.contractAddress,
		"walletOwner",
		&result,
	)

	return result, err
}

// ------ Events -------

func (wr *WalletRegistry) AuthorizationDecreaseApprovedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
) *WrAuthorizationDecreaseApprovedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrAuthorizationDecreaseApprovedSubscription{
		wr,
		opts,
		stakingProviderFilter,
	}
}

type WrAuthorizationDecreaseApprovedSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
}

type walletRegistryAuthorizationDecreaseApprovedFunc func(
	StakingProvider common.Address,
	blockNumber uint64,
)

func (adas *WrAuthorizationDecreaseApprovedSubscription) OnEvent(
	handler walletRegistryAuthorizationDecreaseApprovedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryAuthorizationDecreaseApproved)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := adas.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (adas *WrAuthorizationDecreaseApprovedSubscription) Pipe(
	sink chan *abi.WalletRegistryAuthorizationDecreaseApproved,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(adas.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := adas.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - adas.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past AuthorizationDecreaseApproved events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := adas.contract.PastAuthorizationDecreaseApprovedEvents(
					fromBlock,
					nil,
					adas.stakingProviderFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past AuthorizationDecreaseApproved events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := adas.contract.watchAuthorizationDecreaseApproved(
		sink,
		adas.stakingProviderFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchAuthorizationDecreaseApproved(
	sink chan *abi.WalletRegistryAuthorizationDecreaseApproved,
	stakingProviderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchAuthorizationDecreaseApproved(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event AuthorizationDecreaseApproved had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event AuthorizationDecreaseApproved failed "+
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

func (wr *WalletRegistry) PastAuthorizationDecreaseApprovedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
) ([]*abi.WalletRegistryAuthorizationDecreaseApproved, error) {
	iterator, err := wr.contract.FilterAuthorizationDecreaseApproved(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationDecreaseApproved events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryAuthorizationDecreaseApproved, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) AuthorizationDecreaseRequestedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *WrAuthorizationDecreaseRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrAuthorizationDecreaseRequestedSubscription{
		wr,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type WrAuthorizationDecreaseRequestedSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type walletRegistryAuthorizationDecreaseRequestedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	DecreasingAt uint64,
	blockNumber uint64,
)

func (adrs *WrAuthorizationDecreaseRequestedSubscription) OnEvent(
	handler walletRegistryAuthorizationDecreaseRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryAuthorizationDecreaseRequested)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Operator,
					event.FromAmount,
					event.ToAmount,
					event.DecreasingAt,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := adrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (adrs *WrAuthorizationDecreaseRequestedSubscription) Pipe(
	sink chan *abi.WalletRegistryAuthorizationDecreaseRequested,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(adrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := adrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - adrs.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past AuthorizationDecreaseRequested events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := adrs.contract.PastAuthorizationDecreaseRequestedEvents(
					fromBlock,
					nil,
					adrs.stakingProviderFilter,
					adrs.operatorFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past AuthorizationDecreaseRequested events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := adrs.contract.watchAuthorizationDecreaseRequested(
		sink,
		adrs.stakingProviderFilter,
		adrs.operatorFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchAuthorizationDecreaseRequested(
	sink chan *abi.WalletRegistryAuthorizationDecreaseRequested,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchAuthorizationDecreaseRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event AuthorizationDecreaseRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event AuthorizationDecreaseRequested failed "+
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

func (wr *WalletRegistry) PastAuthorizationDecreaseRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.WalletRegistryAuthorizationDecreaseRequested, error) {
	iterator, err := wr.contract.FilterAuthorizationDecreaseRequested(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		operatorFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationDecreaseRequested events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryAuthorizationDecreaseRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) AuthorizationIncreasedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *WrAuthorizationIncreasedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrAuthorizationIncreasedSubscription{
		wr,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type WrAuthorizationIncreasedSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type walletRegistryAuthorizationIncreasedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	blockNumber uint64,
)

func (ais *WrAuthorizationIncreasedSubscription) OnEvent(
	handler walletRegistryAuthorizationIncreasedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryAuthorizationIncreased)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Operator,
					event.FromAmount,
					event.ToAmount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ais.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ais *WrAuthorizationIncreasedSubscription) Pipe(
	sink chan *abi.WalletRegistryAuthorizationIncreased,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ais.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ais.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ais.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past AuthorizationIncreased events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ais.contract.PastAuthorizationIncreasedEvents(
					fromBlock,
					nil,
					ais.stakingProviderFilter,
					ais.operatorFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past AuthorizationIncreased events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ais.contract.watchAuthorizationIncreased(
		sink,
		ais.stakingProviderFilter,
		ais.operatorFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchAuthorizationIncreased(
	sink chan *abi.WalletRegistryAuthorizationIncreased,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchAuthorizationIncreased(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event AuthorizationIncreased had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event AuthorizationIncreased failed "+
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

func (wr *WalletRegistry) PastAuthorizationIncreasedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.WalletRegistryAuthorizationIncreased, error) {
	iterator, err := wr.contract.FilterAuthorizationIncreased(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		operatorFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationIncreased events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryAuthorizationIncreased, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) AuthorizationParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *WrAuthorizationParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrAuthorizationParametersUpdatedSubscription{
		wr,
		opts,
	}
}

type WrAuthorizationParametersUpdatedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryAuthorizationParametersUpdatedFunc func(
	MinimumAuthorization *big.Int,
	AuthorizationDecreaseDelay uint64,
	AuthorizationDecreaseChangePeriod uint64,
	blockNumber uint64,
)

func (apus *WrAuthorizationParametersUpdatedSubscription) OnEvent(
	handler walletRegistryAuthorizationParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryAuthorizationParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.MinimumAuthorization,
					event.AuthorizationDecreaseDelay,
					event.AuthorizationDecreaseChangePeriod,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := apus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (apus *WrAuthorizationParametersUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistryAuthorizationParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(apus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := apus.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - apus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past AuthorizationParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := apus.contract.PastAuthorizationParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past AuthorizationParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := apus.contract.watchAuthorizationParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchAuthorizationParametersUpdated(
	sink chan *abi.WalletRegistryAuthorizationParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchAuthorizationParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event AuthorizationParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event AuthorizationParametersUpdated failed "+
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

func (wr *WalletRegistry) PastAuthorizationParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryAuthorizationParametersUpdated, error) {
	iterator, err := wr.contract.FilterAuthorizationParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryAuthorizationParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgMaliciousResultSlashedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
) *WrDkgMaliciousResultSlashedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgMaliciousResultSlashedSubscription{
		wr,
		opts,
		resultHashFilter,
	}
}

type WrDkgMaliciousResultSlashedSubscription struct {
	contract         *WalletRegistry
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
}

type walletRegistryDkgMaliciousResultSlashedFunc func(
	ResultHash [32]byte,
	SlashingAmount *big.Int,
	MaliciousSubmitter common.Address,
	blockNumber uint64,
)

func (dmrss *WrDkgMaliciousResultSlashedSubscription) OnEvent(
	handler walletRegistryDkgMaliciousResultSlashedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgMaliciousResultSlashed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ResultHash,
					event.SlashingAmount,
					event.MaliciousSubmitter,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dmrss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dmrss *WrDkgMaliciousResultSlashedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgMaliciousResultSlashed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dmrss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dmrss.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dmrss.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgMaliciousResultSlashed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dmrss.contract.PastDkgMaliciousResultSlashedEvents(
					fromBlock,
					nil,
					dmrss.resultHashFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgMaliciousResultSlashed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dmrss.contract.watchDkgMaliciousResultSlashed(
		sink,
		dmrss.resultHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgMaliciousResultSlashed(
	sink chan *abi.WalletRegistryDkgMaliciousResultSlashed,
	resultHashFilter [][32]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgMaliciousResultSlashed(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgMaliciousResultSlashed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgMaliciousResultSlashed failed "+
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

func (wr *WalletRegistry) PastDkgMaliciousResultSlashedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
) ([]*abi.WalletRegistryDkgMaliciousResultSlashed, error) {
	iterator, err := wr.contract.FilterDkgMaliciousResultSlashed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		resultHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgMaliciousResultSlashed events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgMaliciousResultSlashed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgMaliciousResultSlashingFailedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
) *WrDkgMaliciousResultSlashingFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgMaliciousResultSlashingFailedSubscription{
		wr,
		opts,
		resultHashFilter,
	}
}

type WrDkgMaliciousResultSlashingFailedSubscription struct {
	contract         *WalletRegistry
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
}

type walletRegistryDkgMaliciousResultSlashingFailedFunc func(
	ResultHash [32]byte,
	SlashingAmount *big.Int,
	MaliciousSubmitter common.Address,
	blockNumber uint64,
)

func (dmrsfs *WrDkgMaliciousResultSlashingFailedSubscription) OnEvent(
	handler walletRegistryDkgMaliciousResultSlashingFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgMaliciousResultSlashingFailed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ResultHash,
					event.SlashingAmount,
					event.MaliciousSubmitter,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dmrsfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dmrsfs *WrDkgMaliciousResultSlashingFailedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgMaliciousResultSlashingFailed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dmrsfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dmrsfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dmrsfs.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgMaliciousResultSlashingFailed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dmrsfs.contract.PastDkgMaliciousResultSlashingFailedEvents(
					fromBlock,
					nil,
					dmrsfs.resultHashFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgMaliciousResultSlashingFailed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dmrsfs.contract.watchDkgMaliciousResultSlashingFailed(
		sink,
		dmrsfs.resultHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgMaliciousResultSlashingFailed(
	sink chan *abi.WalletRegistryDkgMaliciousResultSlashingFailed,
	resultHashFilter [][32]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgMaliciousResultSlashingFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgMaliciousResultSlashingFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgMaliciousResultSlashingFailed failed "+
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

func (wr *WalletRegistry) PastDkgMaliciousResultSlashingFailedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
) ([]*abi.WalletRegistryDkgMaliciousResultSlashingFailed, error) {
	iterator, err := wr.contract.FilterDkgMaliciousResultSlashingFailed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		resultHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgMaliciousResultSlashingFailed events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgMaliciousResultSlashingFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *WrDkgParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgParametersUpdatedSubscription{
		wr,
		opts,
	}
}

type WrDkgParametersUpdatedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryDkgParametersUpdatedFunc func(
	SeedTimeout *big.Int,
	ResultChallengePeriodLength *big.Int,
	ResultChallengeExtraGas *big.Int,
	ResultSubmissionTimeout *big.Int,
	ResultSubmitterPrecedencePeriodLength *big.Int,
	blockNumber uint64,
)

func (dpus *WrDkgParametersUpdatedSubscription) OnEvent(
	handler walletRegistryDkgParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.SeedTimeout,
					event.ResultChallengePeriodLength,
					event.ResultChallengeExtraGas,
					event.ResultSubmissionTimeout,
					event.ResultSubmitterPrecedencePeriodLength,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dpus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dpus *WrDkgParametersUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dpus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dpus.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dpus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dpus.contract.PastDkgParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dpus.contract.watchDkgParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgParametersUpdated(
	sink chan *abi.WalletRegistryDkgParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgParametersUpdated failed "+
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

func (wr *WalletRegistry) PastDkgParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryDkgParametersUpdated, error) {
	iterator, err := wr.contract.FilterDkgParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgResultApprovedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
	approverFilter []common.Address,
) *WrDkgResultApprovedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgResultApprovedSubscription{
		wr,
		opts,
		resultHashFilter,
		approverFilter,
	}
}

type WrDkgResultApprovedSubscription struct {
	contract         *WalletRegistry
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
	approverFilter   []common.Address
}

type walletRegistryDkgResultApprovedFunc func(
	ResultHash [32]byte,
	Approver common.Address,
	blockNumber uint64,
)

func (dras *WrDkgResultApprovedSubscription) OnEvent(
	handler walletRegistryDkgResultApprovedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgResultApproved)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ResultHash,
					event.Approver,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dras.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dras *WrDkgResultApprovedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgResultApproved,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dras.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dras.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dras.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgResultApproved events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dras.contract.PastDkgResultApprovedEvents(
					fromBlock,
					nil,
					dras.resultHashFilter,
					dras.approverFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgResultApproved events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dras.contract.watchDkgResultApproved(
		sink,
		dras.resultHashFilter,
		dras.approverFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgResultApproved(
	sink chan *abi.WalletRegistryDkgResultApproved,
	resultHashFilter [][32]byte,
	approverFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgResultApproved(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
			approverFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgResultApproved had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgResultApproved failed "+
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

func (wr *WalletRegistry) PastDkgResultApprovedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
	approverFilter []common.Address,
) ([]*abi.WalletRegistryDkgResultApproved, error) {
	iterator, err := wr.contract.FilterDkgResultApproved(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		resultHashFilter,
		approverFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgResultApproved events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgResultApproved, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgResultChallengedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
	challengerFilter []common.Address,
) *WrDkgResultChallengedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgResultChallengedSubscription{
		wr,
		opts,
		resultHashFilter,
		challengerFilter,
	}
}

type WrDkgResultChallengedSubscription struct {
	contract         *WalletRegistry
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
	challengerFilter []common.Address
}

type walletRegistryDkgResultChallengedFunc func(
	ResultHash [32]byte,
	Challenger common.Address,
	Reason string,
	blockNumber uint64,
)

func (drcs *WrDkgResultChallengedSubscription) OnEvent(
	handler walletRegistryDkgResultChallengedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgResultChallenged)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ResultHash,
					event.Challenger,
					event.Reason,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := drcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (drcs *WrDkgResultChallengedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgResultChallenged,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(drcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := drcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - drcs.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgResultChallenged events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := drcs.contract.PastDkgResultChallengedEvents(
					fromBlock,
					nil,
					drcs.resultHashFilter,
					drcs.challengerFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgResultChallenged events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := drcs.contract.watchDkgResultChallenged(
		sink,
		drcs.resultHashFilter,
		drcs.challengerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgResultChallenged(
	sink chan *abi.WalletRegistryDkgResultChallenged,
	resultHashFilter [][32]byte,
	challengerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgResultChallenged(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
			challengerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgResultChallenged had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgResultChallenged failed "+
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

func (wr *WalletRegistry) PastDkgResultChallengedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
	challengerFilter []common.Address,
) ([]*abi.WalletRegistryDkgResultChallenged, error) {
	iterator, err := wr.contract.FilterDkgResultChallenged(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		resultHashFilter,
		challengerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgResultChallenged events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgResultChallenged, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgResultSubmittedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
	seedFilter []*big.Int,
) *WrDkgResultSubmittedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgResultSubmittedSubscription{
		wr,
		opts,
		resultHashFilter,
		seedFilter,
	}
}

type WrDkgResultSubmittedSubscription struct {
	contract         *WalletRegistry
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
	seedFilter       []*big.Int
}

type walletRegistryDkgResultSubmittedFunc func(
	ResultHash [32]byte,
	Seed *big.Int,
	Result abi.EcdsaDkgResult,
	blockNumber uint64,
)

func (drss *WrDkgResultSubmittedSubscription) OnEvent(
	handler walletRegistryDkgResultSubmittedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgResultSubmitted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ResultHash,
					event.Seed,
					event.Result,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := drss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (drss *WrDkgResultSubmittedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgResultSubmitted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(drss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := drss.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - drss.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgResultSubmitted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := drss.contract.PastDkgResultSubmittedEvents(
					fromBlock,
					nil,
					drss.resultHashFilter,
					drss.seedFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgResultSubmitted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := drss.contract.watchDkgResultSubmitted(
		sink,
		drss.resultHashFilter,
		drss.seedFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgResultSubmitted(
	sink chan *abi.WalletRegistryDkgResultSubmitted,
	resultHashFilter [][32]byte,
	seedFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgResultSubmitted(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
			seedFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgResultSubmitted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgResultSubmitted failed "+
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

func (wr *WalletRegistry) PastDkgResultSubmittedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
	seedFilter []*big.Int,
) ([]*abi.WalletRegistryDkgResultSubmitted, error) {
	iterator, err := wr.contract.FilterDkgResultSubmitted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		resultHashFilter,
		seedFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgResultSubmitted events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgResultSubmitted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgSeedTimedOutEvent(
	opts *ethereum.SubscribeOpts,
) *WrDkgSeedTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgSeedTimedOutSubscription{
		wr,
		opts,
	}
}

type WrDkgSeedTimedOutSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryDkgSeedTimedOutFunc func(
	blockNumber uint64,
)

func (dstos *WrDkgSeedTimedOutSubscription) OnEvent(
	handler walletRegistryDkgSeedTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgSeedTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dstos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dstos *WrDkgSeedTimedOutSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgSeedTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dstos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dstos.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dstos.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgSeedTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dstos.contract.PastDkgSeedTimedOutEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgSeedTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dstos.contract.watchDkgSeedTimedOut(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgSeedTimedOut(
	sink chan *abi.WalletRegistryDkgSeedTimedOut,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgSeedTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgSeedTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgSeedTimedOut failed "+
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

func (wr *WalletRegistry) PastDkgSeedTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryDkgSeedTimedOut, error) {
	iterator, err := wr.contract.FilterDkgSeedTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgSeedTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgSeedTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgStartedEvent(
	opts *ethereum.SubscribeOpts,
	seedFilter []*big.Int,
) *WrDkgStartedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgStartedSubscription{
		wr,
		opts,
		seedFilter,
	}
}

type WrDkgStartedSubscription struct {
	contract   *WalletRegistry
	opts       *ethereum.SubscribeOpts
	seedFilter []*big.Int
}

type walletRegistryDkgStartedFunc func(
	Seed *big.Int,
	blockNumber uint64,
)

func (dss *WrDkgStartedSubscription) OnEvent(
	handler walletRegistryDkgStartedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgStarted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Seed,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dss *WrDkgStartedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgStarted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dss.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dss.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgStarted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dss.contract.PastDkgStartedEvents(
					fromBlock,
					nil,
					dss.seedFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgStarted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dss.contract.watchDkgStarted(
		sink,
		dss.seedFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgStarted(
	sink chan *abi.WalletRegistryDkgStarted,
	seedFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgStarted(
			&bind.WatchOpts{Context: ctx},
			sink,
			seedFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgStarted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgStarted failed "+
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

func (wr *WalletRegistry) PastDkgStartedEvents(
	startBlock uint64,
	endBlock *uint64,
	seedFilter []*big.Int,
) ([]*abi.WalletRegistryDkgStarted, error) {
	iterator, err := wr.contract.FilterDkgStarted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		seedFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgStarted events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgStarted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgStateLockedEvent(
	opts *ethereum.SubscribeOpts,
) *WrDkgStateLockedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgStateLockedSubscription{
		wr,
		opts,
	}
}

type WrDkgStateLockedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryDkgStateLockedFunc func(
	blockNumber uint64,
)

func (dsls *WrDkgStateLockedSubscription) OnEvent(
	handler walletRegistryDkgStateLockedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgStateLocked)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dsls.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dsls *WrDkgStateLockedSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgStateLocked,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dsls.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dsls.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dsls.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgStateLocked events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dsls.contract.PastDkgStateLockedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgStateLocked events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dsls.contract.watchDkgStateLocked(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgStateLocked(
	sink chan *abi.WalletRegistryDkgStateLocked,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgStateLocked(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgStateLocked had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgStateLocked failed "+
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

func (wr *WalletRegistry) PastDkgStateLockedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryDkgStateLocked, error) {
	iterator, err := wr.contract.FilterDkgStateLocked(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgStateLocked events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgStateLocked, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) DkgTimedOutEvent(
	opts *ethereum.SubscribeOpts,
) *WrDkgTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrDkgTimedOutSubscription{
		wr,
		opts,
	}
}

type WrDkgTimedOutSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryDkgTimedOutFunc func(
	blockNumber uint64,
)

func (dtos *WrDkgTimedOutSubscription) OnEvent(
	handler walletRegistryDkgTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryDkgTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dtos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dtos *WrDkgTimedOutSubscription) Pipe(
	sink chan *abi.WalletRegistryDkgTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dtos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dtos.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dtos.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past DkgTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dtos.contract.PastDkgTimedOutEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past DkgTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dtos.contract.watchDkgTimedOut(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchDkgTimedOut(
	sink chan *abi.WalletRegistryDkgTimedOut,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchDkgTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event DkgTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event DkgTimedOut failed "+
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

func (wr *WalletRegistry) PastDkgTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryDkgTimedOut, error) {
	iterator, err := wr.contract.FilterDkgTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DkgTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryDkgTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) GasParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *WrGasParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrGasParametersUpdatedSubscription{
		wr,
		opts,
	}
}

type WrGasParametersUpdatedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryGasParametersUpdatedFunc func(
	DkgResultSubmissionGas *big.Int,
	DkgResultApprovalGasOffset *big.Int,
	NotifyOperatorInactivityGasOffset *big.Int,
	NotifySeedTimeoutGasOffset *big.Int,
	NotifyDkgTimeoutNegativeGasOffset *big.Int,
	blockNumber uint64,
)

func (gpus *WrGasParametersUpdatedSubscription) OnEvent(
	handler walletRegistryGasParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryGasParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.DkgResultSubmissionGas,
					event.DkgResultApprovalGasOffset,
					event.NotifyOperatorInactivityGasOffset,
					event.NotifySeedTimeoutGasOffset,
					event.NotifyDkgTimeoutNegativeGasOffset,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := gpus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (gpus *WrGasParametersUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistryGasParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(gpus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := gpus.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gpus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past GasParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gpus.contract.PastGasParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past GasParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := gpus.contract.watchGasParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchGasParametersUpdated(
	sink chan *abi.WalletRegistryGasParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchGasParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event GasParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event GasParametersUpdated failed "+
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

func (wr *WalletRegistry) PastGasParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryGasParametersUpdated, error) {
	iterator, err := wr.contract.FilterGasParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past GasParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryGasParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) GovernanceTransferredEvent(
	opts *ethereum.SubscribeOpts,
) *WrGovernanceTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrGovernanceTransferredSubscription{
		wr,
		opts,
	}
}

type WrGovernanceTransferredSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryGovernanceTransferredFunc func(
	OldGovernance common.Address,
	NewGovernance common.Address,
	blockNumber uint64,
)

func (gts *WrGovernanceTransferredSubscription) OnEvent(
	handler walletRegistryGovernanceTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryGovernanceTransferred)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.OldGovernance,
					event.NewGovernance,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := gts.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (gts *WrGovernanceTransferredSubscription) Pipe(
	sink chan *abi.WalletRegistryGovernanceTransferred,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(gts.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := gts.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gts.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past GovernanceTransferred events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gts.contract.PastGovernanceTransferredEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past GovernanceTransferred events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := gts.contract.watchGovernanceTransferred(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchGovernanceTransferred(
	sink chan *abi.WalletRegistryGovernanceTransferred,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchGovernanceTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event GovernanceTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event GovernanceTransferred failed "+
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

func (wr *WalletRegistry) PastGovernanceTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryGovernanceTransferred, error) {
	iterator, err := wr.contract.FilterGovernanceTransferred(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past GovernanceTransferred events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryGovernanceTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) InactivityClaimedEvent(
	opts *ethereum.SubscribeOpts,
	walletIDFilter [][32]byte,
) *WrInactivityClaimedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrInactivityClaimedSubscription{
		wr,
		opts,
		walletIDFilter,
	}
}

type WrInactivityClaimedSubscription struct {
	contract       *WalletRegistry
	opts           *ethereum.SubscribeOpts
	walletIDFilter [][32]byte
}

type walletRegistryInactivityClaimedFunc func(
	WalletID [32]byte,
	Nonce *big.Int,
	Notifier common.Address,
	blockNumber uint64,
)

func (ics *WrInactivityClaimedSubscription) OnEvent(
	handler walletRegistryInactivityClaimedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryInactivityClaimed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletID,
					event.Nonce,
					event.Notifier,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ics.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ics *WrInactivityClaimedSubscription) Pipe(
	sink chan *abi.WalletRegistryInactivityClaimed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ics.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ics.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ics.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past InactivityClaimed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ics.contract.PastInactivityClaimedEvents(
					fromBlock,
					nil,
					ics.walletIDFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past InactivityClaimed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ics.contract.watchInactivityClaimed(
		sink,
		ics.walletIDFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchInactivityClaimed(
	sink chan *abi.WalletRegistryInactivityClaimed,
	walletIDFilter [][32]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchInactivityClaimed(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletIDFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event InactivityClaimed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event InactivityClaimed failed "+
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

func (wr *WalletRegistry) PastInactivityClaimedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletIDFilter [][32]byte,
) ([]*abi.WalletRegistryInactivityClaimed, error) {
	iterator, err := wr.contract.FilterInactivityClaimed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletIDFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past InactivityClaimed events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryInactivityClaimed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) InitializedEvent(
	opts *ethereum.SubscribeOpts,
) *WrInitializedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrInitializedSubscription{
		wr,
		opts,
	}
}

type WrInitializedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryInitializedFunc func(
	Version uint8,
	blockNumber uint64,
)

func (is *WrInitializedSubscription) OnEvent(
	handler walletRegistryInitializedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryInitialized)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Version,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := is.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (is *WrInitializedSubscription) Pipe(
	sink chan *abi.WalletRegistryInitialized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(is.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := is.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - is.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past Initialized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := is.contract.PastInitializedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past Initialized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := is.contract.watchInitialized(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchInitialized(
	sink chan *abi.WalletRegistryInitialized,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchInitialized(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event Initialized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event Initialized failed "+
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

func (wr *WalletRegistry) PastInitializedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryInitialized, error) {
	iterator, err := wr.contract.FilterInitialized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Initialized events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryInitialized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) InvoluntaryAuthorizationDecreaseFailedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *WrInvoluntaryAuthorizationDecreaseFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrInvoluntaryAuthorizationDecreaseFailedSubscription{
		wr,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type WrInvoluntaryAuthorizationDecreaseFailedSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type walletRegistryInvoluntaryAuthorizationDecreaseFailedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	blockNumber uint64,
)

func (iadfs *WrInvoluntaryAuthorizationDecreaseFailedSubscription) OnEvent(
	handler walletRegistryInvoluntaryAuthorizationDecreaseFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryInvoluntaryAuthorizationDecreaseFailed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Operator,
					event.FromAmount,
					event.ToAmount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := iadfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (iadfs *WrInvoluntaryAuthorizationDecreaseFailedSubscription) Pipe(
	sink chan *abi.WalletRegistryInvoluntaryAuthorizationDecreaseFailed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(iadfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := iadfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - iadfs.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past InvoluntaryAuthorizationDecreaseFailed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := iadfs.contract.PastInvoluntaryAuthorizationDecreaseFailedEvents(
					fromBlock,
					nil,
					iadfs.stakingProviderFilter,
					iadfs.operatorFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past InvoluntaryAuthorizationDecreaseFailed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := iadfs.contract.watchInvoluntaryAuthorizationDecreaseFailed(
		sink,
		iadfs.stakingProviderFilter,
		iadfs.operatorFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchInvoluntaryAuthorizationDecreaseFailed(
	sink chan *abi.WalletRegistryInvoluntaryAuthorizationDecreaseFailed,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchInvoluntaryAuthorizationDecreaseFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event InvoluntaryAuthorizationDecreaseFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event InvoluntaryAuthorizationDecreaseFailed failed "+
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

func (wr *WalletRegistry) PastInvoluntaryAuthorizationDecreaseFailedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.WalletRegistryInvoluntaryAuthorizationDecreaseFailed, error) {
	iterator, err := wr.contract.FilterInvoluntaryAuthorizationDecreaseFailed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		operatorFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past InvoluntaryAuthorizationDecreaseFailed events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryInvoluntaryAuthorizationDecreaseFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) OperatorJoinedSortitionPoolEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *WrOperatorJoinedSortitionPoolSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrOperatorJoinedSortitionPoolSubscription{
		wr,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type WrOperatorJoinedSortitionPoolSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type walletRegistryOperatorJoinedSortitionPoolFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	blockNumber uint64,
)

func (ojsps *WrOperatorJoinedSortitionPoolSubscription) OnEvent(
	handler walletRegistryOperatorJoinedSortitionPoolFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryOperatorJoinedSortitionPool)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Operator,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ojsps.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ojsps *WrOperatorJoinedSortitionPoolSubscription) Pipe(
	sink chan *abi.WalletRegistryOperatorJoinedSortitionPool,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ojsps.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ojsps.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ojsps.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past OperatorJoinedSortitionPool events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ojsps.contract.PastOperatorJoinedSortitionPoolEvents(
					fromBlock,
					nil,
					ojsps.stakingProviderFilter,
					ojsps.operatorFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past OperatorJoinedSortitionPool events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ojsps.contract.watchOperatorJoinedSortitionPool(
		sink,
		ojsps.stakingProviderFilter,
		ojsps.operatorFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchOperatorJoinedSortitionPool(
	sink chan *abi.WalletRegistryOperatorJoinedSortitionPool,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchOperatorJoinedSortitionPool(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event OperatorJoinedSortitionPool had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event OperatorJoinedSortitionPool failed "+
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

func (wr *WalletRegistry) PastOperatorJoinedSortitionPoolEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.WalletRegistryOperatorJoinedSortitionPool, error) {
	iterator, err := wr.contract.FilterOperatorJoinedSortitionPool(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		operatorFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past OperatorJoinedSortitionPool events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryOperatorJoinedSortitionPool, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) OperatorRegisteredEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *WrOperatorRegisteredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrOperatorRegisteredSubscription{
		wr,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type WrOperatorRegisteredSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type walletRegistryOperatorRegisteredFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	blockNumber uint64,
)

func (ors *WrOperatorRegisteredSubscription) OnEvent(
	handler walletRegistryOperatorRegisteredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryOperatorRegistered)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Operator,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ors.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ors *WrOperatorRegisteredSubscription) Pipe(
	sink chan *abi.WalletRegistryOperatorRegistered,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ors.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ors.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ors.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past OperatorRegistered events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ors.contract.PastOperatorRegisteredEvents(
					fromBlock,
					nil,
					ors.stakingProviderFilter,
					ors.operatorFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past OperatorRegistered events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ors.contract.watchOperatorRegistered(
		sink,
		ors.stakingProviderFilter,
		ors.operatorFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchOperatorRegistered(
	sink chan *abi.WalletRegistryOperatorRegistered,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchOperatorRegistered(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event OperatorRegistered had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event OperatorRegistered failed "+
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

func (wr *WalletRegistry) PastOperatorRegisteredEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.WalletRegistryOperatorRegistered, error) {
	iterator, err := wr.contract.FilterOperatorRegistered(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		operatorFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past OperatorRegistered events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryOperatorRegistered, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) OperatorStatusUpdatedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *WrOperatorStatusUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrOperatorStatusUpdatedSubscription{
		wr,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type WrOperatorStatusUpdatedSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type walletRegistryOperatorStatusUpdatedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	blockNumber uint64,
)

func (osus *WrOperatorStatusUpdatedSubscription) OnEvent(
	handler walletRegistryOperatorStatusUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryOperatorStatusUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Operator,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := osus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (osus *WrOperatorStatusUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistryOperatorStatusUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(osus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := osus.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - osus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past OperatorStatusUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := osus.contract.PastOperatorStatusUpdatedEvents(
					fromBlock,
					nil,
					osus.stakingProviderFilter,
					osus.operatorFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past OperatorStatusUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := osus.contract.watchOperatorStatusUpdated(
		sink,
		osus.stakingProviderFilter,
		osus.operatorFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchOperatorStatusUpdated(
	sink chan *abi.WalletRegistryOperatorStatusUpdated,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchOperatorStatusUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event OperatorStatusUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event OperatorStatusUpdated failed "+
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

func (wr *WalletRegistry) PastOperatorStatusUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.WalletRegistryOperatorStatusUpdated, error) {
	iterator, err := wr.contract.FilterOperatorStatusUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		operatorFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past OperatorStatusUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryOperatorStatusUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) RandomBeaconUpgradedEvent(
	opts *ethereum.SubscribeOpts,
) *WrRandomBeaconUpgradedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrRandomBeaconUpgradedSubscription{
		wr,
		opts,
	}
}

type WrRandomBeaconUpgradedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryRandomBeaconUpgradedFunc func(
	RandomBeacon common.Address,
	blockNumber uint64,
)

func (rbus *WrRandomBeaconUpgradedSubscription) OnEvent(
	handler walletRegistryRandomBeaconUpgradedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryRandomBeaconUpgraded)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RandomBeacon,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rbus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rbus *WrRandomBeaconUpgradedSubscription) Pipe(
	sink chan *abi.WalletRegistryRandomBeaconUpgraded,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rbus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rbus.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rbus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past RandomBeaconUpgraded events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rbus.contract.PastRandomBeaconUpgradedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past RandomBeaconUpgraded events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rbus.contract.watchRandomBeaconUpgraded(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchRandomBeaconUpgraded(
	sink chan *abi.WalletRegistryRandomBeaconUpgraded,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchRandomBeaconUpgraded(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event RandomBeaconUpgraded had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event RandomBeaconUpgraded failed "+
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

func (wr *WalletRegistry) PastRandomBeaconUpgradedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryRandomBeaconUpgraded, error) {
	iterator, err := wr.contract.FilterRandomBeaconUpgraded(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RandomBeaconUpgraded events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryRandomBeaconUpgraded, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) ReimbursementPoolUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *WrReimbursementPoolUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrReimbursementPoolUpdatedSubscription{
		wr,
		opts,
	}
}

type WrReimbursementPoolUpdatedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryReimbursementPoolUpdatedFunc func(
	NewReimbursementPool common.Address,
	blockNumber uint64,
)

func (rpus *WrReimbursementPoolUpdatedSubscription) OnEvent(
	handler walletRegistryReimbursementPoolUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryReimbursementPoolUpdated)
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

func (rpus *WrReimbursementPoolUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistryReimbursementPoolUpdated,
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
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past ReimbursementPoolUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastReimbursementPoolUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
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

func (wr *WalletRegistry) watchReimbursementPoolUpdated(
	sink chan *abi.WalletRegistryReimbursementPoolUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchReimbursementPoolUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event ReimbursementPoolUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
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

func (wr *WalletRegistry) PastReimbursementPoolUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryReimbursementPoolUpdated, error) {
	iterator, err := wr.contract.FilterReimbursementPoolUpdated(
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

	events := make([]*abi.WalletRegistryReimbursementPoolUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) RewardParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *WrRewardParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrRewardParametersUpdatedSubscription{
		wr,
		opts,
	}
}

type WrRewardParametersUpdatedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryRewardParametersUpdatedFunc func(
	MaliciousDkgResultNotificationRewardMultiplier *big.Int,
	SortitionPoolRewardsBanDuration *big.Int,
	blockNumber uint64,
)

func (rpus *WrRewardParametersUpdatedSubscription) OnEvent(
	handler walletRegistryRewardParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryRewardParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.MaliciousDkgResultNotificationRewardMultiplier,
					event.SortitionPoolRewardsBanDuration,
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

func (rpus *WrRewardParametersUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistryRewardParametersUpdated,
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
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past RewardParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastRewardParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past RewardParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rpus.contract.watchRewardParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchRewardParametersUpdated(
	sink chan *abi.WalletRegistryRewardParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchRewardParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event RewardParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event RewardParametersUpdated failed "+
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

func (wr *WalletRegistry) PastRewardParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryRewardParametersUpdated, error) {
	iterator, err := wr.contract.FilterRewardParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RewardParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryRewardParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) RewardsWithdrawnEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
) *WrRewardsWithdrawnSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrRewardsWithdrawnSubscription{
		wr,
		opts,
		stakingProviderFilter,
	}
}

type WrRewardsWithdrawnSubscription struct {
	contract              *WalletRegistry
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
}

type walletRegistryRewardsWithdrawnFunc func(
	StakingProvider common.Address,
	Amount *big.Int,
	blockNumber uint64,
)

func (rws *WrRewardsWithdrawnSubscription) OnEvent(
	handler walletRegistryRewardsWithdrawnFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryRewardsWithdrawn)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Amount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rws.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rws *WrRewardsWithdrawnSubscription) Pipe(
	sink chan *abi.WalletRegistryRewardsWithdrawn,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rws.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rws.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rws.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past RewardsWithdrawn events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rws.contract.PastRewardsWithdrawnEvents(
					fromBlock,
					nil,
					rws.stakingProviderFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past RewardsWithdrawn events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rws.contract.watchRewardsWithdrawn(
		sink,
		rws.stakingProviderFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchRewardsWithdrawn(
	sink chan *abi.WalletRegistryRewardsWithdrawn,
	stakingProviderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchRewardsWithdrawn(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event RewardsWithdrawn had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event RewardsWithdrawn failed "+
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

func (wr *WalletRegistry) PastRewardsWithdrawnEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
) ([]*abi.WalletRegistryRewardsWithdrawn, error) {
	iterator, err := wr.contract.FilterRewardsWithdrawn(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RewardsWithdrawn events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryRewardsWithdrawn, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) SlashingParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *WrSlashingParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrSlashingParametersUpdatedSubscription{
		wr,
		opts,
	}
}

type WrSlashingParametersUpdatedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistrySlashingParametersUpdatedFunc func(
	MaliciousDkgResultSlashingAmount *big.Int,
	blockNumber uint64,
)

func (spus *WrSlashingParametersUpdatedSubscription) OnEvent(
	handler walletRegistrySlashingParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistrySlashingParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.MaliciousDkgResultSlashingAmount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := spus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (spus *WrSlashingParametersUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistrySlashingParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(spus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := spus.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - spus.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past SlashingParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := spus.contract.PastSlashingParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past SlashingParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := spus.contract.watchSlashingParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchSlashingParametersUpdated(
	sink chan *abi.WalletRegistrySlashingParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchSlashingParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event SlashingParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event SlashingParametersUpdated failed "+
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

func (wr *WalletRegistry) PastSlashingParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistrySlashingParametersUpdated, error) {
	iterator, err := wr.contract.FilterSlashingParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past SlashingParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistrySlashingParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) WalletClosedEvent(
	opts *ethereum.SubscribeOpts,
	walletIDFilter [][32]byte,
) *WrWalletClosedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrWalletClosedSubscription{
		wr,
		opts,
		walletIDFilter,
	}
}

type WrWalletClosedSubscription struct {
	contract       *WalletRegistry
	opts           *ethereum.SubscribeOpts
	walletIDFilter [][32]byte
}

type walletRegistryWalletClosedFunc func(
	WalletID [32]byte,
	blockNumber uint64,
)

func (wcs *WrWalletClosedSubscription) OnEvent(
	handler walletRegistryWalletClosedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryWalletClosed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletID,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wcs *WrWalletClosedSubscription) Pipe(
	sink chan *abi.WalletRegistryWalletClosed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wcs.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past WalletClosed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wcs.contract.PastWalletClosedEvents(
					fromBlock,
					nil,
					wcs.walletIDFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past WalletClosed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wcs.contract.watchWalletClosed(
		sink,
		wcs.walletIDFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchWalletClosed(
	sink chan *abi.WalletRegistryWalletClosed,
	walletIDFilter [][32]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchWalletClosed(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletIDFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event WalletClosed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event WalletClosed failed "+
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

func (wr *WalletRegistry) PastWalletClosedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletIDFilter [][32]byte,
) ([]*abi.WalletRegistryWalletClosed, error) {
	iterator, err := wr.contract.FilterWalletClosed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletIDFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletClosed events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryWalletClosed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) WalletCreatedEvent(
	opts *ethereum.SubscribeOpts,
	walletIDFilter [][32]byte,
	dkgResultHashFilter [][32]byte,
) *WrWalletCreatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrWalletCreatedSubscription{
		wr,
		opts,
		walletIDFilter,
		dkgResultHashFilter,
	}
}

type WrWalletCreatedSubscription struct {
	contract            *WalletRegistry
	opts                *ethereum.SubscribeOpts
	walletIDFilter      [][32]byte
	dkgResultHashFilter [][32]byte
}

type walletRegistryWalletCreatedFunc func(
	WalletID [32]byte,
	DkgResultHash [32]byte,
	blockNumber uint64,
)

func (wcs *WrWalletCreatedSubscription) OnEvent(
	handler walletRegistryWalletCreatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryWalletCreated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletID,
					event.DkgResultHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wcs *WrWalletCreatedSubscription) Pipe(
	sink chan *abi.WalletRegistryWalletCreated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wcs.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past WalletCreated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wcs.contract.PastWalletCreatedEvents(
					fromBlock,
					nil,
					wcs.walletIDFilter,
					wcs.dkgResultHashFilter,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past WalletCreated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wcs.contract.watchWalletCreated(
		sink,
		wcs.walletIDFilter,
		wcs.dkgResultHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchWalletCreated(
	sink chan *abi.WalletRegistryWalletCreated,
	walletIDFilter [][32]byte,
	dkgResultHashFilter [][32]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchWalletCreated(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletIDFilter,
			dkgResultHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event WalletCreated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event WalletCreated failed "+
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

func (wr *WalletRegistry) PastWalletCreatedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletIDFilter [][32]byte,
	dkgResultHashFilter [][32]byte,
) ([]*abi.WalletRegistryWalletCreated, error) {
	iterator, err := wr.contract.FilterWalletCreated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletIDFilter,
		dkgResultHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletCreated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryWalletCreated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (wr *WalletRegistry) WalletOwnerUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *WrWalletOwnerUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &WrWalletOwnerUpdatedSubscription{
		wr,
		opts,
	}
}

type WrWalletOwnerUpdatedSubscription struct {
	contract *WalletRegistry
	opts     *ethereum.SubscribeOpts
}

type walletRegistryWalletOwnerUpdatedFunc func(
	WalletOwner common.Address,
	blockNumber uint64,
)

func (wous *WrWalletOwnerUpdatedSubscription) OnEvent(
	handler walletRegistryWalletOwnerUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.WalletRegistryWalletOwnerUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletOwner,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wous.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wous *WrWalletOwnerUpdatedSubscription) Pipe(
	sink chan *abi.WalletRegistryWalletOwnerUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wous.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wous.contract.blockCounter.CurrentBlock()
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wous.opts.PastBlocks

				wrLogger.Infof(
					"subscription monitoring fetching past WalletOwnerUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wous.contract.PastWalletOwnerUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					wrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				wrLogger.Infof(
					"subscription monitoring fetched [%v] past WalletOwnerUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wous.contract.watchWalletOwnerUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wr *WalletRegistry) watchWalletOwnerUpdated(
	sink chan *abi.WalletRegistryWalletOwnerUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return wr.contract.WatchWalletOwnerUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		wrLogger.Warnf(
			"subscription to event WalletOwnerUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		wrLogger.Errorf(
			"subscription to event WalletOwnerUpdated failed "+
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

func (wr *WalletRegistry) PastWalletOwnerUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.WalletRegistryWalletOwnerUpdated, error) {
	iterator, err := wr.contract.FilterWalletOwnerUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletOwnerUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.WalletRegistryWalletOwnerUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

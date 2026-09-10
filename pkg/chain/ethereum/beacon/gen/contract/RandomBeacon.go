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

	"github.com/keep-network/keep-core/pkg/chain/ethereum/beacon/gen/abi"
	"github.com/keep-network/keep-core/pkg/keepcommon/chain/ethereum"
	chainutil "github.com/keep-network/keep-core/pkg/keepcommon/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/subscription"
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var rbLogger = log.Logger("keep-contract-RandomBeacon")

type RandomBeacon struct {
	contract          *abi.RandomBeacon
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

func NewRandomBeacon(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*RandomBeacon, error) {
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

	contract, err := abi.NewRandomBeacon(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.RandomBeaconABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &RandomBeacon{
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
func (rb *RandomBeacon) ApproveAuthorizationDecrease(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction approveAuthorizationDecrease",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.ApproveAuthorizationDecrease(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"approveAuthorizationDecrease",
			arg_stakingProvider,
		)
	}

	rbLogger.Infof(
		"submitted transaction approveAuthorizationDecrease with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.ApproveAuthorizationDecrease(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"approveAuthorizationDecrease",
					arg_stakingProvider,
				)
			}

			rbLogger.Infof(
				"submitted transaction approveAuthorizationDecrease with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallApproveAuthorizationDecrease(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"approveAuthorizationDecrease",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (rb *RandomBeacon) ApproveAuthorizationDecreaseGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"approveAuthorizationDecrease",
		rb.contractABI,
		rb.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) ApproveDkgResult(
	arg_dkgResult abi.BeaconDkgResult,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction approveDkgResult",
		" params: ",
		fmt.Sprint(
			arg_dkgResult,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.ApproveDkgResult(
		transactorOptions,
		arg_dkgResult,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"approveDkgResult",
			arg_dkgResult,
		)
	}

	rbLogger.Infof(
		"submitted transaction approveDkgResult with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.ApproveDkgResult(
				newTransactorOptions,
				arg_dkgResult,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"approveDkgResult",
					arg_dkgResult,
				)
			}

			rbLogger.Infof(
				"submitted transaction approveDkgResult with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallApproveDkgResult(
	arg_dkgResult abi.BeaconDkgResult,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"approveDkgResult",
		&result,
		arg_dkgResult,
	)

	return err
}

func (rb *RandomBeacon) ApproveDkgResultGasEstimate(
	arg_dkgResult abi.BeaconDkgResult,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"approveDkgResult",
		rb.contractABI,
		rb.transactor,
		arg_dkgResult,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) AuthorizationDecreaseRequested(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction authorizationDecreaseRequested",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.AuthorizationDecreaseRequested(
		transactorOptions,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"authorizationDecreaseRequested",
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
	}

	rbLogger.Infof(
		"submitted transaction authorizationDecreaseRequested with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.AuthorizationDecreaseRequested(
				newTransactorOptions,
				arg_stakingProvider,
				arg_fromAmount,
				arg_toAmount,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"authorizationDecreaseRequested",
					arg_stakingProvider,
					arg_fromAmount,
					arg_toAmount,
				)
			}

			rbLogger.Infof(
				"submitted transaction authorizationDecreaseRequested with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallAuthorizationDecreaseRequested(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"authorizationDecreaseRequested",
		&result,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return err
}

func (rb *RandomBeacon) AuthorizationDecreaseRequestedGasEstimate(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"authorizationDecreaseRequested",
		rb.contractABI,
		rb.transactor,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) AuthorizationIncreased(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction authorizationIncreased",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.AuthorizationIncreased(
		transactorOptions,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"authorizationIncreased",
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
	}

	rbLogger.Infof(
		"submitted transaction authorizationIncreased with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.AuthorizationIncreased(
				newTransactorOptions,
				arg_stakingProvider,
				arg_fromAmount,
				arg_toAmount,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"authorizationIncreased",
					arg_stakingProvider,
					arg_fromAmount,
					arg_toAmount,
				)
			}

			rbLogger.Infof(
				"submitted transaction authorizationIncreased with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallAuthorizationIncreased(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"authorizationIncreased",
		&result,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return err
}

func (rb *RandomBeacon) AuthorizationIncreasedGasEstimate(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"authorizationIncreased",
		rb.contractABI,
		rb.transactor,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) ChallengeDkgResult(
	arg_dkgResult abi.BeaconDkgResult,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction challengeDkgResult",
		" params: ",
		fmt.Sprint(
			arg_dkgResult,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.ChallengeDkgResult(
		transactorOptions,
		arg_dkgResult,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"challengeDkgResult",
			arg_dkgResult,
		)
	}

	rbLogger.Infof(
		"submitted transaction challengeDkgResult with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.ChallengeDkgResult(
				newTransactorOptions,
				arg_dkgResult,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"challengeDkgResult",
					arg_dkgResult,
				)
			}

			rbLogger.Infof(
				"submitted transaction challengeDkgResult with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallChallengeDkgResult(
	arg_dkgResult abi.BeaconDkgResult,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"challengeDkgResult",
		&result,
		arg_dkgResult,
	)

	return err
}

func (rb *RandomBeacon) ChallengeDkgResultGasEstimate(
	arg_dkgResult abi.BeaconDkgResult,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"challengeDkgResult",
		rb.contractABI,
		rb.transactor,
		arg_dkgResult,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) Genesis(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction genesis",
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.Genesis(
		transactorOptions,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"genesis",
		)
	}

	rbLogger.Infof(
		"submitted transaction genesis with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.Genesis(
				newTransactorOptions,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"genesis",
				)
			}

			rbLogger.Infof(
				"submitted transaction genesis with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallGenesis(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"genesis",
		&result,
	)

	return err
}

func (rb *RandomBeacon) GenesisGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"genesis",
		rb.contractABI,
		rb.transactor,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) InvoluntaryAuthorizationDecrease(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction involuntaryAuthorizationDecrease",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.InvoluntaryAuthorizationDecrease(
		transactorOptions,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"involuntaryAuthorizationDecrease",
			arg_stakingProvider,
			arg_fromAmount,
			arg_toAmount,
		)
	}

	rbLogger.Infof(
		"submitted transaction involuntaryAuthorizationDecrease with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.InvoluntaryAuthorizationDecrease(
				newTransactorOptions,
				arg_stakingProvider,
				arg_fromAmount,
				arg_toAmount,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"involuntaryAuthorizationDecrease",
					arg_stakingProvider,
					arg_fromAmount,
					arg_toAmount,
				)
			}

			rbLogger.Infof(
				"submitted transaction involuntaryAuthorizationDecrease with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallInvoluntaryAuthorizationDecrease(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"involuntaryAuthorizationDecrease",
		&result,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return err
}

func (rb *RandomBeacon) InvoluntaryAuthorizationDecreaseGasEstimate(
	arg_stakingProvider common.Address,
	arg_fromAmount *big.Int,
	arg_toAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"involuntaryAuthorizationDecrease",
		rb.contractABI,
		rb.transactor,
		arg_stakingProvider,
		arg_fromAmount,
		arg_toAmount,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) JoinSortitionPool(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction joinSortitionPool",
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.JoinSortitionPool(
		transactorOptions,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"joinSortitionPool",
		)
	}

	rbLogger.Infof(
		"submitted transaction joinSortitionPool with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.JoinSortitionPool(
				newTransactorOptions,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"joinSortitionPool",
				)
			}

			rbLogger.Infof(
				"submitted transaction joinSortitionPool with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallJoinSortitionPool(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"joinSortitionPool",
		&result,
	)

	return err
}

func (rb *RandomBeacon) JoinSortitionPoolGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"joinSortitionPool",
		rb.contractABI,
		rb.transactor,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) NotifyDkgTimeout(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction notifyDkgTimeout",
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.NotifyDkgTimeout(
		transactorOptions,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"notifyDkgTimeout",
		)
	}

	rbLogger.Infof(
		"submitted transaction notifyDkgTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.NotifyDkgTimeout(
				newTransactorOptions,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"notifyDkgTimeout",
				)
			}

			rbLogger.Infof(
				"submitted transaction notifyDkgTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallNotifyDkgTimeout(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"notifyDkgTimeout",
		&result,
	)

	return err
}

func (rb *RandomBeacon) NotifyDkgTimeoutGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"notifyDkgTimeout",
		rb.contractABI,
		rb.transactor,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) NotifyOperatorInactivity(
	arg_claim abi.BeaconInactivityClaim,
	arg_nonce *big.Int,
	arg_groupMembers []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction notifyOperatorInactivity",
		" params: ",
		fmt.Sprint(
			arg_claim,
			arg_nonce,
			arg_groupMembers,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.NotifyOperatorInactivity(
		transactorOptions,
		arg_claim,
		arg_nonce,
		arg_groupMembers,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"notifyOperatorInactivity",
			arg_claim,
			arg_nonce,
			arg_groupMembers,
		)
	}

	rbLogger.Infof(
		"submitted transaction notifyOperatorInactivity with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.NotifyOperatorInactivity(
				newTransactorOptions,
				arg_claim,
				arg_nonce,
				arg_groupMembers,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"notifyOperatorInactivity",
					arg_claim,
					arg_nonce,
					arg_groupMembers,
				)
			}

			rbLogger.Infof(
				"submitted transaction notifyOperatorInactivity with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallNotifyOperatorInactivity(
	arg_claim abi.BeaconInactivityClaim,
	arg_nonce *big.Int,
	arg_groupMembers []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"notifyOperatorInactivity",
		&result,
		arg_claim,
		arg_nonce,
		arg_groupMembers,
	)

	return err
}

func (rb *RandomBeacon) NotifyOperatorInactivityGasEstimate(
	arg_claim abi.BeaconInactivityClaim,
	arg_nonce *big.Int,
	arg_groupMembers []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"notifyOperatorInactivity",
		rb.contractABI,
		rb.transactor,
		arg_claim,
		arg_nonce,
		arg_groupMembers,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) RegisterOperator(
	arg_operator common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction registerOperator",
		" params: ",
		fmt.Sprint(
			arg_operator,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.RegisterOperator(
		transactorOptions,
		arg_operator,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"registerOperator",
			arg_operator,
		)
	}

	rbLogger.Infof(
		"submitted transaction registerOperator with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.RegisterOperator(
				newTransactorOptions,
				arg_operator,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"registerOperator",
					arg_operator,
				)
			}

			rbLogger.Infof(
				"submitted transaction registerOperator with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallRegisterOperator(
	arg_operator common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"registerOperator",
		&result,
		arg_operator,
	)

	return err
}

func (rb *RandomBeacon) RegisterOperatorGasEstimate(
	arg_operator common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"registerOperator",
		rb.contractABI,
		rb.transactor,
		arg_operator,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) ReportRelayEntryTimeout(
	arg_groupMembers []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction reportRelayEntryTimeout",
		" params: ",
		fmt.Sprint(
			arg_groupMembers,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.ReportRelayEntryTimeout(
		transactorOptions,
		arg_groupMembers,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"reportRelayEntryTimeout",
			arg_groupMembers,
		)
	}

	rbLogger.Infof(
		"submitted transaction reportRelayEntryTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.ReportRelayEntryTimeout(
				newTransactorOptions,
				arg_groupMembers,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"reportRelayEntryTimeout",
					arg_groupMembers,
				)
			}

			rbLogger.Infof(
				"submitted transaction reportRelayEntryTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallReportRelayEntryTimeout(
	arg_groupMembers []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"reportRelayEntryTimeout",
		&result,
		arg_groupMembers,
	)

	return err
}

func (rb *RandomBeacon) ReportRelayEntryTimeoutGasEstimate(
	arg_groupMembers []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"reportRelayEntryTimeout",
		rb.contractABI,
		rb.transactor,
		arg_groupMembers,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) ReportUnauthorizedSigning(
	arg_signedMsgSender []byte,
	arg_groupId uint64,
	arg_groupMembers []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction reportUnauthorizedSigning",
		" params: ",
		fmt.Sprint(
			arg_signedMsgSender,
			arg_groupId,
			arg_groupMembers,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.ReportUnauthorizedSigning(
		transactorOptions,
		arg_signedMsgSender,
		arg_groupId,
		arg_groupMembers,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"reportUnauthorizedSigning",
			arg_signedMsgSender,
			arg_groupId,
			arg_groupMembers,
		)
	}

	rbLogger.Infof(
		"submitted transaction reportUnauthorizedSigning with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.ReportUnauthorizedSigning(
				newTransactorOptions,
				arg_signedMsgSender,
				arg_groupId,
				arg_groupMembers,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"reportUnauthorizedSigning",
					arg_signedMsgSender,
					arg_groupId,
					arg_groupMembers,
				)
			}

			rbLogger.Infof(
				"submitted transaction reportUnauthorizedSigning with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallReportUnauthorizedSigning(
	arg_signedMsgSender []byte,
	arg_groupId uint64,
	arg_groupMembers []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"reportUnauthorizedSigning",
		&result,
		arg_signedMsgSender,
		arg_groupId,
		arg_groupMembers,
	)

	return err
}

func (rb *RandomBeacon) ReportUnauthorizedSigningGasEstimate(
	arg_signedMsgSender []byte,
	arg_groupId uint64,
	arg_groupMembers []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"reportUnauthorizedSigning",
		rb.contractABI,
		rb.transactor,
		arg_signedMsgSender,
		arg_groupId,
		arg_groupMembers,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) RequestRelayEntry(
	arg_callbackContract common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction requestRelayEntry",
		" params: ",
		fmt.Sprint(
			arg_callbackContract,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.RequestRelayEntry(
		transactorOptions,
		arg_callbackContract,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"requestRelayEntry",
			arg_callbackContract,
		)
	}

	rbLogger.Infof(
		"submitted transaction requestRelayEntry with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.RequestRelayEntry(
				newTransactorOptions,
				arg_callbackContract,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"requestRelayEntry",
					arg_callbackContract,
				)
			}

			rbLogger.Infof(
				"submitted transaction requestRelayEntry with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallRequestRelayEntry(
	arg_callbackContract common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"requestRelayEntry",
		&result,
		arg_callbackContract,
	)

	return err
}

func (rb *RandomBeacon) RequestRelayEntryGasEstimate(
	arg_callbackContract common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"requestRelayEntry",
		rb.contractABI,
		rb.transactor,
		arg_callbackContract,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) SetRequesterAuthorization(
	arg_requester common.Address,
	arg_isAuthorized bool,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction setRequesterAuthorization",
		" params: ",
		fmt.Sprint(
			arg_requester,
			arg_isAuthorized,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.SetRequesterAuthorization(
		transactorOptions,
		arg_requester,
		arg_isAuthorized,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"setRequesterAuthorization",
			arg_requester,
			arg_isAuthorized,
		)
	}

	rbLogger.Infof(
		"submitted transaction setRequesterAuthorization with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.SetRequesterAuthorization(
				newTransactorOptions,
				arg_requester,
				arg_isAuthorized,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"setRequesterAuthorization",
					arg_requester,
					arg_isAuthorized,
				)
			}

			rbLogger.Infof(
				"submitted transaction setRequesterAuthorization with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallSetRequesterAuthorization(
	arg_requester common.Address,
	arg_isAuthorized bool,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"setRequesterAuthorization",
		&result,
		arg_requester,
		arg_isAuthorized,
	)

	return err
}

func (rb *RandomBeacon) SetRequesterAuthorizationGasEstimate(
	arg_requester common.Address,
	arg_isAuthorized bool,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"setRequesterAuthorization",
		rb.contractABI,
		rb.transactor,
		arg_requester,
		arg_isAuthorized,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) SubmitDkgResult(
	arg_dkgResult abi.BeaconDkgResult,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction submitDkgResult",
		" params: ",
		fmt.Sprint(
			arg_dkgResult,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.SubmitDkgResult(
		transactorOptions,
		arg_dkgResult,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"submitDkgResult",
			arg_dkgResult,
		)
	}

	rbLogger.Infof(
		"submitted transaction submitDkgResult with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.SubmitDkgResult(
				newTransactorOptions,
				arg_dkgResult,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"submitDkgResult",
					arg_dkgResult,
				)
			}

			rbLogger.Infof(
				"submitted transaction submitDkgResult with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallSubmitDkgResult(
	arg_dkgResult abi.BeaconDkgResult,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"submitDkgResult",
		&result,
		arg_dkgResult,
	)

	return err
}

func (rb *RandomBeacon) SubmitDkgResultGasEstimate(
	arg_dkgResult abi.BeaconDkgResult,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"submitDkgResult",
		rb.contractABI,
		rb.transactor,
		arg_dkgResult,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) SubmitRelayEntry(
	arg_entry []byte,
	arg_groupMembers []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction submitRelayEntry",
		" params: ",
		fmt.Sprint(
			arg_entry,
			arg_groupMembers,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.SubmitRelayEntry(
		transactorOptions,
		arg_entry,
		arg_groupMembers,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"submitRelayEntry",
			arg_entry,
			arg_groupMembers,
		)
	}

	rbLogger.Infof(
		"submitted transaction submitRelayEntry with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.SubmitRelayEntry(
				newTransactorOptions,
				arg_entry,
				arg_groupMembers,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"submitRelayEntry",
					arg_entry,
					arg_groupMembers,
				)
			}

			rbLogger.Infof(
				"submitted transaction submitRelayEntry with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallSubmitRelayEntry(
	arg_entry []byte,
	arg_groupMembers []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"submitRelayEntry",
		&result,
		arg_entry,
		arg_groupMembers,
	)

	return err
}

func (rb *RandomBeacon) SubmitRelayEntryGasEstimate(
	arg_entry []byte,
	arg_groupMembers []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"submitRelayEntry",
		rb.contractABI,
		rb.transactor,
		arg_entry,
		arg_groupMembers,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) SubmitRelayEntry0(
	arg_entry []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction submitRelayEntry0",
		" params: ",
		fmt.Sprint(
			arg_entry,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.SubmitRelayEntry0(
		transactorOptions,
		arg_entry,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"submitRelayEntry0",
			arg_entry,
		)
	}

	rbLogger.Infof(
		"submitted transaction submitRelayEntry0 with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.SubmitRelayEntry0(
				newTransactorOptions,
				arg_entry,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"submitRelayEntry0",
					arg_entry,
				)
			}

			rbLogger.Infof(
				"submitted transaction submitRelayEntry0 with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallSubmitRelayEntry0(
	arg_entry []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"submitRelayEntry0",
		&result,
		arg_entry,
	)

	return err
}

func (rb *RandomBeacon) SubmitRelayEntry0GasEstimate(
	arg_entry []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"submitRelayEntry0",
		rb.contractABI,
		rb.transactor,
		arg_entry,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) TransferGovernance(
	arg_newGovernance common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction transferGovernance",
		" params: ",
		fmt.Sprint(
			arg_newGovernance,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.TransferGovernance(
		transactorOptions,
		arg_newGovernance,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"transferGovernance",
			arg_newGovernance,
		)
	}

	rbLogger.Infof(
		"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.TransferGovernance(
				newTransactorOptions,
				arg_newGovernance,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"transferGovernance",
					arg_newGovernance,
				)
			}

			rbLogger.Infof(
				"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallTransferGovernance(
	arg_newGovernance common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"transferGovernance",
		&result,
		arg_newGovernance,
	)

	return err
}

func (rb *RandomBeacon) TransferGovernanceGasEstimate(
	arg_newGovernance common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"transferGovernance",
		rb.contractABI,
		rb.transactor,
		arg_newGovernance,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateAuthorizationParameters(
	arg__minimumAuthorization *big.Int,
	arg__authorizationDecreaseDelay uint64,
	arg__authorizationDecreaseChangePeriod uint64,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateAuthorizationParameters",
		" params: ",
		fmt.Sprint(
			arg__minimumAuthorization,
			arg__authorizationDecreaseDelay,
			arg__authorizationDecreaseChangePeriod,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateAuthorizationParameters(
		transactorOptions,
		arg__minimumAuthorization,
		arg__authorizationDecreaseDelay,
		arg__authorizationDecreaseChangePeriod,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateAuthorizationParameters",
			arg__minimumAuthorization,
			arg__authorizationDecreaseDelay,
			arg__authorizationDecreaseChangePeriod,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateAuthorizationParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateAuthorizationParameters(
				newTransactorOptions,
				arg__minimumAuthorization,
				arg__authorizationDecreaseDelay,
				arg__authorizationDecreaseChangePeriod,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateAuthorizationParameters",
					arg__minimumAuthorization,
					arg__authorizationDecreaseDelay,
					arg__authorizationDecreaseChangePeriod,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateAuthorizationParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateAuthorizationParameters(
	arg__minimumAuthorization *big.Int,
	arg__authorizationDecreaseDelay uint64,
	arg__authorizationDecreaseChangePeriod uint64,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateAuthorizationParameters",
		&result,
		arg__minimumAuthorization,
		arg__authorizationDecreaseDelay,
		arg__authorizationDecreaseChangePeriod,
	)

	return err
}

func (rb *RandomBeacon) UpdateAuthorizationParametersGasEstimate(
	arg__minimumAuthorization *big.Int,
	arg__authorizationDecreaseDelay uint64,
	arg__authorizationDecreaseChangePeriod uint64,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateAuthorizationParameters",
		rb.contractABI,
		rb.transactor,
		arg__minimumAuthorization,
		arg__authorizationDecreaseDelay,
		arg__authorizationDecreaseChangePeriod,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateGasParameters(
	arg_dkgResultSubmissionGas *big.Int,
	arg_dkgResultApprovalGasOffset *big.Int,
	arg_notifyOperatorInactivityGasOffset *big.Int,
	arg_relayEntrySubmissionGasOffset *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateGasParameters",
		" params: ",
		fmt.Sprint(
			arg_dkgResultSubmissionGas,
			arg_dkgResultApprovalGasOffset,
			arg_notifyOperatorInactivityGasOffset,
			arg_relayEntrySubmissionGasOffset,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateGasParameters(
		transactorOptions,
		arg_dkgResultSubmissionGas,
		arg_dkgResultApprovalGasOffset,
		arg_notifyOperatorInactivityGasOffset,
		arg_relayEntrySubmissionGasOffset,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateGasParameters",
			arg_dkgResultSubmissionGas,
			arg_dkgResultApprovalGasOffset,
			arg_notifyOperatorInactivityGasOffset,
			arg_relayEntrySubmissionGasOffset,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateGasParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateGasParameters(
				newTransactorOptions,
				arg_dkgResultSubmissionGas,
				arg_dkgResultApprovalGasOffset,
				arg_notifyOperatorInactivityGasOffset,
				arg_relayEntrySubmissionGasOffset,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateGasParameters",
					arg_dkgResultSubmissionGas,
					arg_dkgResultApprovalGasOffset,
					arg_notifyOperatorInactivityGasOffset,
					arg_relayEntrySubmissionGasOffset,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateGasParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateGasParameters(
	arg_dkgResultSubmissionGas *big.Int,
	arg_dkgResultApprovalGasOffset *big.Int,
	arg_notifyOperatorInactivityGasOffset *big.Int,
	arg_relayEntrySubmissionGasOffset *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateGasParameters",
		&result,
		arg_dkgResultSubmissionGas,
		arg_dkgResultApprovalGasOffset,
		arg_notifyOperatorInactivityGasOffset,
		arg_relayEntrySubmissionGasOffset,
	)

	return err
}

func (rb *RandomBeacon) UpdateGasParametersGasEstimate(
	arg_dkgResultSubmissionGas *big.Int,
	arg_dkgResultApprovalGasOffset *big.Int,
	arg_notifyOperatorInactivityGasOffset *big.Int,
	arg_relayEntrySubmissionGasOffset *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateGasParameters",
		rb.contractABI,
		rb.transactor,
		arg_dkgResultSubmissionGas,
		arg_dkgResultApprovalGasOffset,
		arg_notifyOperatorInactivityGasOffset,
		arg_relayEntrySubmissionGasOffset,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateGroupCreationParameters(
	arg_groupCreationFrequency *big.Int,
	arg_groupLifetime *big.Int,
	arg_dkgResultChallengePeriodLength *big.Int,
	arg_dkgResultChallengeExtraGas *big.Int,
	arg_dkgResultSubmissionTimeout *big.Int,
	arg_dkgSubmitterPrecedencePeriodLength *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateGroupCreationParameters",
		" params: ",
		fmt.Sprint(
			arg_groupCreationFrequency,
			arg_groupLifetime,
			arg_dkgResultChallengePeriodLength,
			arg_dkgResultChallengeExtraGas,
			arg_dkgResultSubmissionTimeout,
			arg_dkgSubmitterPrecedencePeriodLength,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateGroupCreationParameters(
		transactorOptions,
		arg_groupCreationFrequency,
		arg_groupLifetime,
		arg_dkgResultChallengePeriodLength,
		arg_dkgResultChallengeExtraGas,
		arg_dkgResultSubmissionTimeout,
		arg_dkgSubmitterPrecedencePeriodLength,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateGroupCreationParameters",
			arg_groupCreationFrequency,
			arg_groupLifetime,
			arg_dkgResultChallengePeriodLength,
			arg_dkgResultChallengeExtraGas,
			arg_dkgResultSubmissionTimeout,
			arg_dkgSubmitterPrecedencePeriodLength,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateGroupCreationParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateGroupCreationParameters(
				newTransactorOptions,
				arg_groupCreationFrequency,
				arg_groupLifetime,
				arg_dkgResultChallengePeriodLength,
				arg_dkgResultChallengeExtraGas,
				arg_dkgResultSubmissionTimeout,
				arg_dkgSubmitterPrecedencePeriodLength,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateGroupCreationParameters",
					arg_groupCreationFrequency,
					arg_groupLifetime,
					arg_dkgResultChallengePeriodLength,
					arg_dkgResultChallengeExtraGas,
					arg_dkgResultSubmissionTimeout,
					arg_dkgSubmitterPrecedencePeriodLength,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateGroupCreationParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateGroupCreationParameters(
	arg_groupCreationFrequency *big.Int,
	arg_groupLifetime *big.Int,
	arg_dkgResultChallengePeriodLength *big.Int,
	arg_dkgResultChallengeExtraGas *big.Int,
	arg_dkgResultSubmissionTimeout *big.Int,
	arg_dkgSubmitterPrecedencePeriodLength *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateGroupCreationParameters",
		&result,
		arg_groupCreationFrequency,
		arg_groupLifetime,
		arg_dkgResultChallengePeriodLength,
		arg_dkgResultChallengeExtraGas,
		arg_dkgResultSubmissionTimeout,
		arg_dkgSubmitterPrecedencePeriodLength,
	)

	return err
}

func (rb *RandomBeacon) UpdateGroupCreationParametersGasEstimate(
	arg_groupCreationFrequency *big.Int,
	arg_groupLifetime *big.Int,
	arg_dkgResultChallengePeriodLength *big.Int,
	arg_dkgResultChallengeExtraGas *big.Int,
	arg_dkgResultSubmissionTimeout *big.Int,
	arg_dkgSubmitterPrecedencePeriodLength *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateGroupCreationParameters",
		rb.contractABI,
		rb.transactor,
		arg_groupCreationFrequency,
		arg_groupLifetime,
		arg_dkgResultChallengePeriodLength,
		arg_dkgResultChallengeExtraGas,
		arg_dkgResultSubmissionTimeout,
		arg_dkgSubmitterPrecedencePeriodLength,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateOperatorStatus(
	arg_operator common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateOperatorStatus",
		" params: ",
		fmt.Sprint(
			arg_operator,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateOperatorStatus(
		transactorOptions,
		arg_operator,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateOperatorStatus",
			arg_operator,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateOperatorStatus with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateOperatorStatus(
				newTransactorOptions,
				arg_operator,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateOperatorStatus",
					arg_operator,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateOperatorStatus with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateOperatorStatus(
	arg_operator common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateOperatorStatus",
		&result,
		arg_operator,
	)

	return err
}

func (rb *RandomBeacon) UpdateOperatorStatusGasEstimate(
	arg_operator common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateOperatorStatus",
		rb.contractABI,
		rb.transactor,
		arg_operator,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateReimbursementPool(
	arg__reimbursementPool common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateReimbursementPool",
		" params: ",
		fmt.Sprint(
			arg__reimbursementPool,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateReimbursementPool(
		transactorOptions,
		arg__reimbursementPool,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateReimbursementPool",
			arg__reimbursementPool,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateReimbursementPool(
				newTransactorOptions,
				arg__reimbursementPool,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateReimbursementPool",
					arg__reimbursementPool,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateReimbursementPool(
	arg__reimbursementPool common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateReimbursementPool",
		&result,
		arg__reimbursementPool,
	)

	return err
}

func (rb *RandomBeacon) UpdateReimbursementPoolGasEstimate(
	arg__reimbursementPool common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateReimbursementPool",
		rb.contractABI,
		rb.transactor,
		arg__reimbursementPool,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateRelayEntryParameters(
	arg_relayEntrySoftTimeout *big.Int,
	arg_relayEntryHardTimeout *big.Int,
	arg_callbackGasLimit *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateRelayEntryParameters",
		" params: ",
		fmt.Sprint(
			arg_relayEntrySoftTimeout,
			arg_relayEntryHardTimeout,
			arg_callbackGasLimit,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateRelayEntryParameters(
		transactorOptions,
		arg_relayEntrySoftTimeout,
		arg_relayEntryHardTimeout,
		arg_callbackGasLimit,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateRelayEntryParameters",
			arg_relayEntrySoftTimeout,
			arg_relayEntryHardTimeout,
			arg_callbackGasLimit,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateRelayEntryParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateRelayEntryParameters(
				newTransactorOptions,
				arg_relayEntrySoftTimeout,
				arg_relayEntryHardTimeout,
				arg_callbackGasLimit,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateRelayEntryParameters",
					arg_relayEntrySoftTimeout,
					arg_relayEntryHardTimeout,
					arg_callbackGasLimit,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateRelayEntryParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateRelayEntryParameters(
	arg_relayEntrySoftTimeout *big.Int,
	arg_relayEntryHardTimeout *big.Int,
	arg_callbackGasLimit *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateRelayEntryParameters",
		&result,
		arg_relayEntrySoftTimeout,
		arg_relayEntryHardTimeout,
		arg_callbackGasLimit,
	)

	return err
}

func (rb *RandomBeacon) UpdateRelayEntryParametersGasEstimate(
	arg_relayEntrySoftTimeout *big.Int,
	arg_relayEntryHardTimeout *big.Int,
	arg_callbackGasLimit *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateRelayEntryParameters",
		rb.contractABI,
		rb.transactor,
		arg_relayEntrySoftTimeout,
		arg_relayEntryHardTimeout,
		arg_callbackGasLimit,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateRewardParameters(
	arg_sortitionPoolRewardsBanDuration *big.Int,
	arg_relayEntryTimeoutNotificationRewardMultiplier *big.Int,
	arg_unauthorizedSigningNotificationRewardMultiplier *big.Int,
	arg_dkgMaliciousResultNotificationRewardMultiplier *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateRewardParameters",
		" params: ",
		fmt.Sprint(
			arg_sortitionPoolRewardsBanDuration,
			arg_relayEntryTimeoutNotificationRewardMultiplier,
			arg_unauthorizedSigningNotificationRewardMultiplier,
			arg_dkgMaliciousResultNotificationRewardMultiplier,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateRewardParameters(
		transactorOptions,
		arg_sortitionPoolRewardsBanDuration,
		arg_relayEntryTimeoutNotificationRewardMultiplier,
		arg_unauthorizedSigningNotificationRewardMultiplier,
		arg_dkgMaliciousResultNotificationRewardMultiplier,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateRewardParameters",
			arg_sortitionPoolRewardsBanDuration,
			arg_relayEntryTimeoutNotificationRewardMultiplier,
			arg_unauthorizedSigningNotificationRewardMultiplier,
			arg_dkgMaliciousResultNotificationRewardMultiplier,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateRewardParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateRewardParameters(
				newTransactorOptions,
				arg_sortitionPoolRewardsBanDuration,
				arg_relayEntryTimeoutNotificationRewardMultiplier,
				arg_unauthorizedSigningNotificationRewardMultiplier,
				arg_dkgMaliciousResultNotificationRewardMultiplier,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateRewardParameters",
					arg_sortitionPoolRewardsBanDuration,
					arg_relayEntryTimeoutNotificationRewardMultiplier,
					arg_unauthorizedSigningNotificationRewardMultiplier,
					arg_dkgMaliciousResultNotificationRewardMultiplier,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateRewardParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateRewardParameters(
	arg_sortitionPoolRewardsBanDuration *big.Int,
	arg_relayEntryTimeoutNotificationRewardMultiplier *big.Int,
	arg_unauthorizedSigningNotificationRewardMultiplier *big.Int,
	arg_dkgMaliciousResultNotificationRewardMultiplier *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateRewardParameters",
		&result,
		arg_sortitionPoolRewardsBanDuration,
		arg_relayEntryTimeoutNotificationRewardMultiplier,
		arg_unauthorizedSigningNotificationRewardMultiplier,
		arg_dkgMaliciousResultNotificationRewardMultiplier,
	)

	return err
}

func (rb *RandomBeacon) UpdateRewardParametersGasEstimate(
	arg_sortitionPoolRewardsBanDuration *big.Int,
	arg_relayEntryTimeoutNotificationRewardMultiplier *big.Int,
	arg_unauthorizedSigningNotificationRewardMultiplier *big.Int,
	arg_dkgMaliciousResultNotificationRewardMultiplier *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateRewardParameters",
		rb.contractABI,
		rb.transactor,
		arg_sortitionPoolRewardsBanDuration,
		arg_relayEntryTimeoutNotificationRewardMultiplier,
		arg_unauthorizedSigningNotificationRewardMultiplier,
		arg_dkgMaliciousResultNotificationRewardMultiplier,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) UpdateSlashingParameters(
	arg_relayEntrySubmissionFailureSlashingAmount *big.Int,
	arg_maliciousDkgResultSlashingAmount *big.Int,
	arg_unauthorizedSigningSlashingAmount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction updateSlashingParameters",
		" params: ",
		fmt.Sprint(
			arg_relayEntrySubmissionFailureSlashingAmount,
			arg_maliciousDkgResultSlashingAmount,
			arg_unauthorizedSigningSlashingAmount,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.UpdateSlashingParameters(
		transactorOptions,
		arg_relayEntrySubmissionFailureSlashingAmount,
		arg_maliciousDkgResultSlashingAmount,
		arg_unauthorizedSigningSlashingAmount,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"updateSlashingParameters",
			arg_relayEntrySubmissionFailureSlashingAmount,
			arg_maliciousDkgResultSlashingAmount,
			arg_unauthorizedSigningSlashingAmount,
		)
	}

	rbLogger.Infof(
		"submitted transaction updateSlashingParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.UpdateSlashingParameters(
				newTransactorOptions,
				arg_relayEntrySubmissionFailureSlashingAmount,
				arg_maliciousDkgResultSlashingAmount,
				arg_unauthorizedSigningSlashingAmount,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"updateSlashingParameters",
					arg_relayEntrySubmissionFailureSlashingAmount,
					arg_maliciousDkgResultSlashingAmount,
					arg_unauthorizedSigningSlashingAmount,
				)
			}

			rbLogger.Infof(
				"submitted transaction updateSlashingParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallUpdateSlashingParameters(
	arg_relayEntrySubmissionFailureSlashingAmount *big.Int,
	arg_maliciousDkgResultSlashingAmount *big.Int,
	arg_unauthorizedSigningSlashingAmount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"updateSlashingParameters",
		&result,
		arg_relayEntrySubmissionFailureSlashingAmount,
		arg_maliciousDkgResultSlashingAmount,
		arg_unauthorizedSigningSlashingAmount,
	)

	return err
}

func (rb *RandomBeacon) UpdateSlashingParametersGasEstimate(
	arg_relayEntrySubmissionFailureSlashingAmount *big.Int,
	arg_maliciousDkgResultSlashingAmount *big.Int,
	arg_unauthorizedSigningSlashingAmount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"updateSlashingParameters",
		rb.contractABI,
		rb.transactor,
		arg_relayEntrySubmissionFailureSlashingAmount,
		arg_maliciousDkgResultSlashingAmount,
		arg_unauthorizedSigningSlashingAmount,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) WithdrawIneligibleRewards(
	arg_recipient common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction withdrawIneligibleRewards",
		" params: ",
		fmt.Sprint(
			arg_recipient,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.WithdrawIneligibleRewards(
		transactorOptions,
		arg_recipient,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"withdrawIneligibleRewards",
			arg_recipient,
		)
	}

	rbLogger.Infof(
		"submitted transaction withdrawIneligibleRewards with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.WithdrawIneligibleRewards(
				newTransactorOptions,
				arg_recipient,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"withdrawIneligibleRewards",
					arg_recipient,
				)
			}

			rbLogger.Infof(
				"submitted transaction withdrawIneligibleRewards with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallWithdrawIneligibleRewards(
	arg_recipient common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"withdrawIneligibleRewards",
		&result,
		arg_recipient,
	)

	return err
}

func (rb *RandomBeacon) WithdrawIneligibleRewardsGasEstimate(
	arg_recipient common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"withdrawIneligibleRewards",
		rb.contractABI,
		rb.transactor,
		arg_recipient,
	)

	return result, err
}

// Transaction submission.
func (rb *RandomBeacon) WithdrawRewards(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rbLogger.Debug(
		"submitting transaction withdrawRewards",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	rb.transactionMutex.Lock()
	defer rb.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rb.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rb.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rb.contract.WithdrawRewards(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, rb.errorResolver.ResolveError(
			err,
			rb.transactorOptions.From,
			nil,
			"withdrawRewards",
			arg_stakingProvider,
		)
	}

	rbLogger.Infof(
		"submitted transaction withdrawRewards with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rb.miningWaiter.ForceMining(
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

			transaction, err := rb.contract.WithdrawRewards(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, rb.errorResolver.ResolveError(
					err,
					rb.transactorOptions.From,
					nil,
					"withdrawRewards",
					arg_stakingProvider,
				)
			}

			rbLogger.Infof(
				"submitted transaction withdrawRewards with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rb.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rb *RandomBeacon) CallWithdrawRewards(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rb.transactorOptions.From,
		blockNumber, nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"withdrawRewards",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (rb *RandomBeacon) WithdrawRewardsGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rb.callerOptions.From,
		rb.contractAddress,
		"withdrawRewards",
		rb.contractABI,
		rb.transactor,
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

func (rb *RandomBeacon) AuthorizationParameters() (authorizationParameters, error) {
	result, err := rb.contract.AuthorizationParameters(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"authorizationParameters",
		)
	}

	return result, err
}

func (rb *RandomBeacon) AuthorizationParametersAtBlock(
	blockNumber *big.Int,
) (authorizationParameters, error) {
	var result authorizationParameters

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"authorizationParameters",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) AuthorizedRequesters(
	arg0 common.Address,
) (bool, error) {
	result, err := rb.contract.AuthorizedRequesters(
		rb.callerOptions,
		arg0,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"authorizedRequesters",
			arg0,
		)
	}

	return result, err
}

func (rb *RandomBeacon) AuthorizedRequestersAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"authorizedRequesters",
		&result,
		arg0,
	)

	return result, err
}

func (rb *RandomBeacon) AvailableRewards(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := rb.contract.AvailableRewards(
		rb.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"availableRewards",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (rb *RandomBeacon) AvailableRewardsAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"availableRewards",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (rb *RandomBeacon) EligibleStake(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := rb.contract.EligibleStake(
		rb.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"eligibleStake",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (rb *RandomBeacon) EligibleStakeAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
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
	RelayEntrySubmissionGasOffset     *big.Int
}

func (rb *RandomBeacon) GasParameters() (gasParameters, error) {
	result, err := rb.contract.GasParameters(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"gasParameters",
		)
	}

	return result, err
}

func (rb *RandomBeacon) GasParametersAtBlock(
	blockNumber *big.Int,
) (gasParameters, error) {
	var result gasParameters

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"gasParameters",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) GetGroup(
	arg_groupId uint64,
) (abi.GroupsGroup, error) {
	result, err := rb.contract.GetGroup(
		rb.callerOptions,
		arg_groupId,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"getGroup",
			arg_groupId,
		)
	}

	return result, err
}

func (rb *RandomBeacon) GetGroupAtBlock(
	arg_groupId uint64,
	blockNumber *big.Int,
) (abi.GroupsGroup, error) {
	var result abi.GroupsGroup

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"getGroup",
		&result,
		arg_groupId,
	)

	return result, err
}

func (rb *RandomBeacon) GetGroup0(
	arg_groupPubKey []byte,
) (abi.GroupsGroup, error) {
	result, err := rb.contract.GetGroup0(
		rb.callerOptions,
		arg_groupPubKey,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"getGroup0",
			arg_groupPubKey,
		)
	}

	return result, err
}

func (rb *RandomBeacon) GetGroup0AtBlock(
	arg_groupPubKey []byte,
	blockNumber *big.Int,
) (abi.GroupsGroup, error) {
	var result abi.GroupsGroup

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"getGroup0",
		&result,
		arg_groupPubKey,
	)

	return result, err
}

func (rb *RandomBeacon) GetGroupCreationState() (uint8, error) {
	result, err := rb.contract.GetGroupCreationState(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"getGroupCreationState",
		)
	}

	return result, err
}

func (rb *RandomBeacon) GetGroupCreationStateAtBlock(
	blockNumber *big.Int,
) (uint8, error) {
	var result uint8

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"getGroupCreationState",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) GetGroupsRegistry() ([][32]byte, error) {
	result, err := rb.contract.GetGroupsRegistry(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"getGroupsRegistry",
		)
	}

	return result, err
}

func (rb *RandomBeacon) GetGroupsRegistryAtBlock(
	blockNumber *big.Int,
) ([][32]byte, error) {
	var result [][32]byte

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"getGroupsRegistry",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) Governance() (common.Address, error) {
	result, err := rb.contract.Governance(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"governance",
		)
	}

	return result, err
}

func (rb *RandomBeacon) GovernanceAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"governance",
		&result,
	)

	return result, err
}

type groupCreationParameters struct {
	GroupCreationFrequency             *big.Int
	GroupLifetime                      *big.Int
	DkgResultChallengePeriodLength     *big.Int
	DkgResultChallengeExtraGas         *big.Int
	DkgResultSubmissionTimeout         *big.Int
	DkgSubmitterPrecedencePeriodLength *big.Int
}

func (rb *RandomBeacon) GroupCreationParameters() (groupCreationParameters, error) {
	result, err := rb.contract.GroupCreationParameters(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"groupCreationParameters",
		)
	}

	return result, err
}

func (rb *RandomBeacon) GroupCreationParametersAtBlock(
	blockNumber *big.Int,
) (groupCreationParameters, error) {
	var result groupCreationParameters

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"groupCreationParameters",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) HasDkgTimedOut() (bool, error) {
	result, err := rb.contract.HasDkgTimedOut(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"hasDkgTimedOut",
		)
	}

	return result, err
}

func (rb *RandomBeacon) HasDkgTimedOutAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"hasDkgTimedOut",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) InactivityClaimNonce(
	arg0 uint64,
) (*big.Int, error) {
	result, err := rb.contract.InactivityClaimNonce(
		rb.callerOptions,
		arg0,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"inactivityClaimNonce",
			arg0,
		)
	}

	return result, err
}

func (rb *RandomBeacon) InactivityClaimNonceAtBlock(
	arg0 uint64,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"inactivityClaimNonce",
		&result,
		arg0,
	)

	return result, err
}

func (rb *RandomBeacon) IsOperatorInPool(
	arg_operator common.Address,
) (bool, error) {
	result, err := rb.contract.IsOperatorInPool(
		rb.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"isOperatorInPool",
			arg_operator,
		)
	}

	return result, err
}

func (rb *RandomBeacon) IsOperatorInPoolAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"isOperatorInPool",
		&result,
		arg_operator,
	)

	return result, err
}

func (rb *RandomBeacon) IsOperatorUpToDate(
	arg_operator common.Address,
) (bool, error) {
	result, err := rb.contract.IsOperatorUpToDate(
		rb.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"isOperatorUpToDate",
			arg_operator,
		)
	}

	return result, err
}

func (rb *RandomBeacon) IsOperatorUpToDateAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"isOperatorUpToDate",
		&result,
		arg_operator,
	)

	return result, err
}

func (rb *RandomBeacon) IsRelayRequestInProgress() (bool, error) {
	result, err := rb.contract.IsRelayRequestInProgress(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"isRelayRequestInProgress",
		)
	}

	return result, err
}

func (rb *RandomBeacon) IsRelayRequestInProgressAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"isRelayRequestInProgress",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) MinimumAuthorization() (*big.Int, error) {
	result, err := rb.contract.MinimumAuthorization(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"minimumAuthorization",
		)
	}

	return result, err
}

func (rb *RandomBeacon) MinimumAuthorizationAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"minimumAuthorization",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) OperatorToStakingProvider(
	arg_operator common.Address,
) (common.Address, error) {
	result, err := rb.contract.OperatorToStakingProvider(
		rb.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"operatorToStakingProvider",
			arg_operator,
		)
	}

	return result, err
}

func (rb *RandomBeacon) OperatorToStakingProviderAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"operatorToStakingProvider",
		&result,
		arg_operator,
	)

	return result, err
}

func (rb *RandomBeacon) PendingAuthorizationDecrease(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := rb.contract.PendingAuthorizationDecrease(
		rb.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"pendingAuthorizationDecrease",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (rb *RandomBeacon) PendingAuthorizationDecreaseAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"pendingAuthorizationDecrease",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (rb *RandomBeacon) ReimbursementPool() (common.Address, error) {
	result, err := rb.contract.ReimbursementPool(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"reimbursementPool",
		)
	}

	return result, err
}

func (rb *RandomBeacon) ReimbursementPoolAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"reimbursementPool",
		&result,
	)

	return result, err
}

type relayEntryParameters struct {
	RelayEntrySoftTimeout *big.Int
	RelayEntryHardTimeout *big.Int
	CallbackGasLimit      *big.Int
}

func (rb *RandomBeacon) RelayEntryParameters() (relayEntryParameters, error) {
	result, err := rb.contract.RelayEntryParameters(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"relayEntryParameters",
		)
	}

	return result, err
}

func (rb *RandomBeacon) RelayEntryParametersAtBlock(
	blockNumber *big.Int,
) (relayEntryParameters, error) {
	var result relayEntryParameters

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"relayEntryParameters",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) RemainingAuthorizationDecreaseDelay(
	arg_stakingProvider common.Address,
) (uint64, error) {
	result, err := rb.contract.RemainingAuthorizationDecreaseDelay(
		rb.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"remainingAuthorizationDecreaseDelay",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (rb *RandomBeacon) RemainingAuthorizationDecreaseDelayAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"remainingAuthorizationDecreaseDelay",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

type rewardParameters struct {
	SortitionPoolRewardsBanDuration                 *big.Int
	RelayEntryTimeoutNotificationRewardMultiplier   *big.Int
	UnauthorizedSigningNotificationRewardMultiplier *big.Int
	DkgMaliciousResultNotificationRewardMultiplier  *big.Int
}

func (rb *RandomBeacon) RewardParameters() (rewardParameters, error) {
	result, err := rb.contract.RewardParameters(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"rewardParameters",
		)
	}

	return result, err
}

func (rb *RandomBeacon) RewardParametersAtBlock(
	blockNumber *big.Int,
) (rewardParameters, error) {
	var result rewardParameters

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"rewardParameters",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) SelectGroup() ([]uint32, error) {
	result, err := rb.contract.SelectGroup(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"selectGroup",
		)
	}

	return result, err
}

func (rb *RandomBeacon) SelectGroupAtBlock(
	blockNumber *big.Int,
) ([]uint32, error) {
	var result []uint32

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"selectGroup",
		&result,
	)

	return result, err
}

type slashingParameters struct {
	RelayEntrySubmissionFailureSlashingAmount *big.Int
	MaliciousDkgResultSlashingAmount          *big.Int
	UnauthorizedSigningSlashingAmount         *big.Int
}

func (rb *RandomBeacon) SlashingParameters() (slashingParameters, error) {
	result, err := rb.contract.SlashingParameters(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"slashingParameters",
		)
	}

	return result, err
}

func (rb *RandomBeacon) SlashingParametersAtBlock(
	blockNumber *big.Int,
) (slashingParameters, error) {
	var result slashingParameters

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"slashingParameters",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) SortitionPool() (common.Address, error) {
	result, err := rb.contract.SortitionPool(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"sortitionPool",
		)
	}

	return result, err
}

func (rb *RandomBeacon) SortitionPoolAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"sortitionPool",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) Staking() (common.Address, error) {
	result, err := rb.contract.Staking(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"staking",
		)
	}

	return result, err
}

func (rb *RandomBeacon) StakingAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"staking",
		&result,
	)

	return result, err
}

func (rb *RandomBeacon) StakingProviderToOperator(
	arg_stakingProvider common.Address,
) (common.Address, error) {
	result, err := rb.contract.StakingProviderToOperator(
		rb.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"stakingProviderToOperator",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (rb *RandomBeacon) StakingProviderToOperatorAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"stakingProviderToOperator",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (rb *RandomBeacon) TToken() (common.Address, error) {
	result, err := rb.contract.TToken(
		rb.callerOptions,
	)

	if err != nil {
		return result, rb.errorResolver.ResolveError(
			err,
			rb.callerOptions.From,
			nil,
			"tToken",
		)
	}

	return result, err
}

func (rb *RandomBeacon) TTokenAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rb.callerOptions.From,
		blockNumber,
		nil,
		rb.contractABI,
		rb.caller,
		rb.errorResolver,
		rb.contractAddress,
		"tToken",
		&result,
	)

	return result, err
}

// ------ Events -------

func (rb *RandomBeacon) AuthorizationDecreaseApprovedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
) *RbAuthorizationDecreaseApprovedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbAuthorizationDecreaseApprovedSubscription{
		rb,
		opts,
		stakingProviderFilter,
	}
}

type RbAuthorizationDecreaseApprovedSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
}

type randomBeaconAuthorizationDecreaseApprovedFunc func(
	StakingProvider common.Address,
	blockNumber uint64,
)

func (adas *RbAuthorizationDecreaseApprovedSubscription) OnEvent(
	handler randomBeaconAuthorizationDecreaseApprovedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconAuthorizationDecreaseApproved)
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

func (adas *RbAuthorizationDecreaseApprovedSubscription) Pipe(
	sink chan *abi.RandomBeaconAuthorizationDecreaseApproved,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - adas.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchAuthorizationDecreaseApproved(
	sink chan *abi.RandomBeaconAuthorizationDecreaseApproved,
	stakingProviderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchAuthorizationDecreaseApproved(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event AuthorizationDecreaseApproved had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastAuthorizationDecreaseApprovedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
) ([]*abi.RandomBeaconAuthorizationDecreaseApproved, error) {
	iterator, err := rb.contract.FilterAuthorizationDecreaseApproved(
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

	events := make([]*abi.RandomBeaconAuthorizationDecreaseApproved, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) AuthorizationDecreaseRequestedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *RbAuthorizationDecreaseRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbAuthorizationDecreaseRequestedSubscription{
		rb,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type RbAuthorizationDecreaseRequestedSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type randomBeaconAuthorizationDecreaseRequestedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	DecreasingAt uint64,
	blockNumber uint64,
)

func (adrs *RbAuthorizationDecreaseRequestedSubscription) OnEvent(
	handler randomBeaconAuthorizationDecreaseRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconAuthorizationDecreaseRequested)
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

func (adrs *RbAuthorizationDecreaseRequestedSubscription) Pipe(
	sink chan *abi.RandomBeaconAuthorizationDecreaseRequested,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - adrs.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchAuthorizationDecreaseRequested(
	sink chan *abi.RandomBeaconAuthorizationDecreaseRequested,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchAuthorizationDecreaseRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event AuthorizationDecreaseRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastAuthorizationDecreaseRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.RandomBeaconAuthorizationDecreaseRequested, error) {
	iterator, err := rb.contract.FilterAuthorizationDecreaseRequested(
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

	events := make([]*abi.RandomBeaconAuthorizationDecreaseRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) AuthorizationIncreasedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *RbAuthorizationIncreasedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbAuthorizationIncreasedSubscription{
		rb,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type RbAuthorizationIncreasedSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type randomBeaconAuthorizationIncreasedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	blockNumber uint64,
)

func (ais *RbAuthorizationIncreasedSubscription) OnEvent(
	handler randomBeaconAuthorizationIncreasedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconAuthorizationIncreased)
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

func (ais *RbAuthorizationIncreasedSubscription) Pipe(
	sink chan *abi.RandomBeaconAuthorizationIncreased,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ais.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchAuthorizationIncreased(
	sink chan *abi.RandomBeaconAuthorizationIncreased,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchAuthorizationIncreased(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event AuthorizationIncreased had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastAuthorizationIncreasedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.RandomBeaconAuthorizationIncreased, error) {
	iterator, err := rb.contract.FilterAuthorizationIncreased(
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

	events := make([]*abi.RandomBeaconAuthorizationIncreased, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) AuthorizationParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RbAuthorizationParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbAuthorizationParametersUpdatedSubscription{
		rb,
		opts,
	}
}

type RbAuthorizationParametersUpdatedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconAuthorizationParametersUpdatedFunc func(
	MinimumAuthorization *big.Int,
	AuthorizationDecreaseDelay uint64,
	AuthorizationDecreaseChangePeriod uint64,
	blockNumber uint64,
)

func (apus *RbAuthorizationParametersUpdatedSubscription) OnEvent(
	handler randomBeaconAuthorizationParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconAuthorizationParametersUpdated)
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

func (apus *RbAuthorizationParametersUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconAuthorizationParametersUpdated,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - apus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past AuthorizationParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := apus.contract.PastAuthorizationParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchAuthorizationParametersUpdated(
	sink chan *abi.RandomBeaconAuthorizationParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchAuthorizationParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event AuthorizationParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastAuthorizationParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconAuthorizationParametersUpdated, error) {
	iterator, err := rb.contract.FilterAuthorizationParametersUpdated(
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

	events := make([]*abi.RandomBeaconAuthorizationParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) CallbackFailedEvent(
	opts *ethereum.SubscribeOpts,
) *RbCallbackFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbCallbackFailedSubscription{
		rb,
		opts,
	}
}

type RbCallbackFailedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconCallbackFailedFunc func(
	Entry *big.Int,
	EntrySubmittedBlock *big.Int,
	blockNumber uint64,
)

func (cfs *RbCallbackFailedSubscription) OnEvent(
	handler randomBeaconCallbackFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconCallbackFailed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Entry,
					event.EntrySubmittedBlock,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := cfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (cfs *RbCallbackFailedSubscription) Pipe(
	sink chan *abi.RandomBeaconCallbackFailed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(cfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := cfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - cfs.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past CallbackFailed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := cfs.contract.PastCallbackFailedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past CallbackFailed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := cfs.contract.watchCallbackFailed(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchCallbackFailed(
	sink chan *abi.RandomBeaconCallbackFailed,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchCallbackFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event CallbackFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event CallbackFailed failed "+
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

func (rb *RandomBeacon) PastCallbackFailedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconCallbackFailed, error) {
	iterator, err := rb.contract.FilterCallbackFailed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past CallbackFailed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconCallbackFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgMaliciousResultSlashedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
) *RbDkgMaliciousResultSlashedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgMaliciousResultSlashedSubscription{
		rb,
		opts,
		resultHashFilter,
	}
}

type RbDkgMaliciousResultSlashedSubscription struct {
	contract         *RandomBeacon
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
}

type randomBeaconDkgMaliciousResultSlashedFunc func(
	ResultHash [32]byte,
	SlashingAmount *big.Int,
	MaliciousSubmitter common.Address,
	blockNumber uint64,
)

func (dmrss *RbDkgMaliciousResultSlashedSubscription) OnEvent(
	handler randomBeaconDkgMaliciousResultSlashedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgMaliciousResultSlashed)
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

func (dmrss *RbDkgMaliciousResultSlashedSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgMaliciousResultSlashed,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dmrss.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgMaliciousResultSlashed(
	sink chan *abi.RandomBeaconDkgMaliciousResultSlashed,
	resultHashFilter [][32]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgMaliciousResultSlashed(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgMaliciousResultSlashed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgMaliciousResultSlashedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
) ([]*abi.RandomBeaconDkgMaliciousResultSlashed, error) {
	iterator, err := rb.contract.FilterDkgMaliciousResultSlashed(
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

	events := make([]*abi.RandomBeaconDkgMaliciousResultSlashed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgMaliciousResultSlashingFailedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
) *RbDkgMaliciousResultSlashingFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgMaliciousResultSlashingFailedSubscription{
		rb,
		opts,
		resultHashFilter,
	}
}

type RbDkgMaliciousResultSlashingFailedSubscription struct {
	contract         *RandomBeacon
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
}

type randomBeaconDkgMaliciousResultSlashingFailedFunc func(
	ResultHash [32]byte,
	SlashingAmount *big.Int,
	MaliciousSubmitter common.Address,
	blockNumber uint64,
)

func (dmrsfs *RbDkgMaliciousResultSlashingFailedSubscription) OnEvent(
	handler randomBeaconDkgMaliciousResultSlashingFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgMaliciousResultSlashingFailed)
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

func (dmrsfs *RbDkgMaliciousResultSlashingFailedSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgMaliciousResultSlashingFailed,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dmrsfs.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgMaliciousResultSlashingFailed(
	sink chan *abi.RandomBeaconDkgMaliciousResultSlashingFailed,
	resultHashFilter [][32]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgMaliciousResultSlashingFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgMaliciousResultSlashingFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgMaliciousResultSlashingFailedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
) ([]*abi.RandomBeaconDkgMaliciousResultSlashingFailed, error) {
	iterator, err := rb.contract.FilterDkgMaliciousResultSlashingFailed(
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

	events := make([]*abi.RandomBeaconDkgMaliciousResultSlashingFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgResultApprovedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
	approverFilter []common.Address,
) *RbDkgResultApprovedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgResultApprovedSubscription{
		rb,
		opts,
		resultHashFilter,
		approverFilter,
	}
}

type RbDkgResultApprovedSubscription struct {
	contract         *RandomBeacon
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
	approverFilter   []common.Address
}

type randomBeaconDkgResultApprovedFunc func(
	ResultHash [32]byte,
	Approver common.Address,
	blockNumber uint64,
)

func (dras *RbDkgResultApprovedSubscription) OnEvent(
	handler randomBeaconDkgResultApprovedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgResultApproved)
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

func (dras *RbDkgResultApprovedSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgResultApproved,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dras.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgResultApproved(
	sink chan *abi.RandomBeaconDkgResultApproved,
	resultHashFilter [][32]byte,
	approverFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgResultApproved(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
			approverFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgResultApproved had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgResultApprovedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
	approverFilter []common.Address,
) ([]*abi.RandomBeaconDkgResultApproved, error) {
	iterator, err := rb.contract.FilterDkgResultApproved(
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

	events := make([]*abi.RandomBeaconDkgResultApproved, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgResultChallengedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
	challengerFilter []common.Address,
) *RbDkgResultChallengedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgResultChallengedSubscription{
		rb,
		opts,
		resultHashFilter,
		challengerFilter,
	}
}

type RbDkgResultChallengedSubscription struct {
	contract         *RandomBeacon
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
	challengerFilter []common.Address
}

type randomBeaconDkgResultChallengedFunc func(
	ResultHash [32]byte,
	Challenger common.Address,
	Reason string,
	blockNumber uint64,
)

func (drcs *RbDkgResultChallengedSubscription) OnEvent(
	handler randomBeaconDkgResultChallengedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgResultChallenged)
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

func (drcs *RbDkgResultChallengedSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgResultChallenged,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - drcs.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgResultChallenged(
	sink chan *abi.RandomBeaconDkgResultChallenged,
	resultHashFilter [][32]byte,
	challengerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgResultChallenged(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
			challengerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgResultChallenged had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgResultChallengedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
	challengerFilter []common.Address,
) ([]*abi.RandomBeaconDkgResultChallenged, error) {
	iterator, err := rb.contract.FilterDkgResultChallenged(
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

	events := make([]*abi.RandomBeaconDkgResultChallenged, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgResultSubmittedEvent(
	opts *ethereum.SubscribeOpts,
	resultHashFilter [][32]byte,
	seedFilter []*big.Int,
) *RbDkgResultSubmittedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgResultSubmittedSubscription{
		rb,
		opts,
		resultHashFilter,
		seedFilter,
	}
}

type RbDkgResultSubmittedSubscription struct {
	contract         *RandomBeacon
	opts             *ethereum.SubscribeOpts
	resultHashFilter [][32]byte
	seedFilter       []*big.Int
}

type randomBeaconDkgResultSubmittedFunc func(
	ResultHash [32]byte,
	Seed *big.Int,
	Result abi.BeaconDkgResult,
	blockNumber uint64,
)

func (drss *RbDkgResultSubmittedSubscription) OnEvent(
	handler randomBeaconDkgResultSubmittedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgResultSubmitted)
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

func (drss *RbDkgResultSubmittedSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgResultSubmitted,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - drss.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgResultSubmitted(
	sink chan *abi.RandomBeaconDkgResultSubmitted,
	resultHashFilter [][32]byte,
	seedFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgResultSubmitted(
			&bind.WatchOpts{Context: ctx},
			sink,
			resultHashFilter,
			seedFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgResultSubmitted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgResultSubmittedEvents(
	startBlock uint64,
	endBlock *uint64,
	resultHashFilter [][32]byte,
	seedFilter []*big.Int,
) ([]*abi.RandomBeaconDkgResultSubmitted, error) {
	iterator, err := rb.contract.FilterDkgResultSubmitted(
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

	events := make([]*abi.RandomBeaconDkgResultSubmitted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgSeedTimedOutEvent(
	opts *ethereum.SubscribeOpts,
) *RbDkgSeedTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgSeedTimedOutSubscription{
		rb,
		opts,
	}
}

type RbDkgSeedTimedOutSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconDkgSeedTimedOutFunc func(
	blockNumber uint64,
)

func (dstos *RbDkgSeedTimedOutSubscription) OnEvent(
	handler randomBeaconDkgSeedTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgSeedTimedOut)
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

func (dstos *RbDkgSeedTimedOutSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgSeedTimedOut,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dstos.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past DkgSeedTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dstos.contract.PastDkgSeedTimedOutEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgSeedTimedOut(
	sink chan *abi.RandomBeaconDkgSeedTimedOut,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgSeedTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgSeedTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgSeedTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconDkgSeedTimedOut, error) {
	iterator, err := rb.contract.FilterDkgSeedTimedOut(
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

	events := make([]*abi.RandomBeaconDkgSeedTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgStartedEvent(
	opts *ethereum.SubscribeOpts,
	seedFilter []*big.Int,
) *RbDkgStartedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgStartedSubscription{
		rb,
		opts,
		seedFilter,
	}
}

type RbDkgStartedSubscription struct {
	contract   *RandomBeacon
	opts       *ethereum.SubscribeOpts
	seedFilter []*big.Int
}

type randomBeaconDkgStartedFunc func(
	Seed *big.Int,
	blockNumber uint64,
)

func (dss *RbDkgStartedSubscription) OnEvent(
	handler randomBeaconDkgStartedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgStarted)
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

func (dss *RbDkgStartedSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgStarted,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dss.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgStarted(
	sink chan *abi.RandomBeaconDkgStarted,
	seedFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgStarted(
			&bind.WatchOpts{Context: ctx},
			sink,
			seedFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgStarted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgStartedEvents(
	startBlock uint64,
	endBlock *uint64,
	seedFilter []*big.Int,
) ([]*abi.RandomBeaconDkgStarted, error) {
	iterator, err := rb.contract.FilterDkgStarted(
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

	events := make([]*abi.RandomBeaconDkgStarted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgStateLockedEvent(
	opts *ethereum.SubscribeOpts,
) *RbDkgStateLockedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgStateLockedSubscription{
		rb,
		opts,
	}
}

type RbDkgStateLockedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconDkgStateLockedFunc func(
	blockNumber uint64,
)

func (dsls *RbDkgStateLockedSubscription) OnEvent(
	handler randomBeaconDkgStateLockedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgStateLocked)
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

func (dsls *RbDkgStateLockedSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgStateLocked,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dsls.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past DkgStateLocked events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dsls.contract.PastDkgStateLockedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgStateLocked(
	sink chan *abi.RandomBeaconDkgStateLocked,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgStateLocked(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgStateLocked had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgStateLockedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconDkgStateLocked, error) {
	iterator, err := rb.contract.FilterDkgStateLocked(
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

	events := make([]*abi.RandomBeaconDkgStateLocked, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) DkgTimedOutEvent(
	opts *ethereum.SubscribeOpts,
) *RbDkgTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbDkgTimedOutSubscription{
		rb,
		opts,
	}
}

type RbDkgTimedOutSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconDkgTimedOutFunc func(
	blockNumber uint64,
)

func (dtos *RbDkgTimedOutSubscription) OnEvent(
	handler randomBeaconDkgTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconDkgTimedOut)
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

func (dtos *RbDkgTimedOutSubscription) Pipe(
	sink chan *abi.RandomBeaconDkgTimedOut,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dtos.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past DkgTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dtos.contract.PastDkgTimedOutEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchDkgTimedOut(
	sink chan *abi.RandomBeaconDkgTimedOut,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchDkgTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event DkgTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastDkgTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconDkgTimedOut, error) {
	iterator, err := rb.contract.FilterDkgTimedOut(
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

	events := make([]*abi.RandomBeaconDkgTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) GasParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RbGasParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbGasParametersUpdatedSubscription{
		rb,
		opts,
	}
}

type RbGasParametersUpdatedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconGasParametersUpdatedFunc func(
	DkgResultSubmissionGas *big.Int,
	DkgResultApprovalGasOffset *big.Int,
	NotifyOperatorInactivityGasOffset *big.Int,
	RelayEntrySubmissionGasOffset *big.Int,
	blockNumber uint64,
)

func (gpus *RbGasParametersUpdatedSubscription) OnEvent(
	handler randomBeaconGasParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconGasParametersUpdated)
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
					event.RelayEntrySubmissionGasOffset,
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

func (gpus *RbGasParametersUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconGasParametersUpdated,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gpus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past GasParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gpus.contract.PastGasParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchGasParametersUpdated(
	sink chan *abi.RandomBeaconGasParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchGasParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event GasParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastGasParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconGasParametersUpdated, error) {
	iterator, err := rb.contract.FilterGasParametersUpdated(
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

	events := make([]*abi.RandomBeaconGasParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) GovernanceTransferredEvent(
	opts *ethereum.SubscribeOpts,
) *RbGovernanceTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbGovernanceTransferredSubscription{
		rb,
		opts,
	}
}

type RbGovernanceTransferredSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconGovernanceTransferredFunc func(
	OldGovernance common.Address,
	NewGovernance common.Address,
	blockNumber uint64,
)

func (gts *RbGovernanceTransferredSubscription) OnEvent(
	handler randomBeaconGovernanceTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconGovernanceTransferred)
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

func (gts *RbGovernanceTransferredSubscription) Pipe(
	sink chan *abi.RandomBeaconGovernanceTransferred,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gts.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past GovernanceTransferred events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gts.contract.PastGovernanceTransferredEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchGovernanceTransferred(
	sink chan *abi.RandomBeaconGovernanceTransferred,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchGovernanceTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event GovernanceTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastGovernanceTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconGovernanceTransferred, error) {
	iterator, err := rb.contract.FilterGovernanceTransferred(
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

	events := make([]*abi.RandomBeaconGovernanceTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) GroupCreationParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RbGroupCreationParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbGroupCreationParametersUpdatedSubscription{
		rb,
		opts,
	}
}

type RbGroupCreationParametersUpdatedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconGroupCreationParametersUpdatedFunc func(
	GroupCreationFrequency *big.Int,
	GroupLifetime *big.Int,
	DkgResultChallengePeriodLength *big.Int,
	DkgResultChallengeExtraGas *big.Int,
	DkgResultSubmissionTimeout *big.Int,
	DkgResultSubmitterPrecedencePeriodLength *big.Int,
	blockNumber uint64,
)

func (gcpus *RbGroupCreationParametersUpdatedSubscription) OnEvent(
	handler randomBeaconGroupCreationParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconGroupCreationParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.GroupCreationFrequency,
					event.GroupLifetime,
					event.DkgResultChallengePeriodLength,
					event.DkgResultChallengeExtraGas,
					event.DkgResultSubmissionTimeout,
					event.DkgResultSubmitterPrecedencePeriodLength,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := gcpus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (gcpus *RbGroupCreationParametersUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconGroupCreationParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(gcpus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := gcpus.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gcpus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past GroupCreationParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gcpus.contract.PastGroupCreationParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past GroupCreationParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := gcpus.contract.watchGroupCreationParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchGroupCreationParametersUpdated(
	sink chan *abi.RandomBeaconGroupCreationParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchGroupCreationParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event GroupCreationParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event GroupCreationParametersUpdated failed "+
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

func (rb *RandomBeacon) PastGroupCreationParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconGroupCreationParametersUpdated, error) {
	iterator, err := rb.contract.FilterGroupCreationParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past GroupCreationParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconGroupCreationParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) GroupRegisteredEvent(
	opts *ethereum.SubscribeOpts,
	groupIdFilter []uint64,
	groupPubKeyFilter [][]byte,
) *RbGroupRegisteredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbGroupRegisteredSubscription{
		rb,
		opts,
		groupIdFilter,
		groupPubKeyFilter,
	}
}

type RbGroupRegisteredSubscription struct {
	contract          *RandomBeacon
	opts              *ethereum.SubscribeOpts
	groupIdFilter     []uint64
	groupPubKeyFilter [][]byte
}

type randomBeaconGroupRegisteredFunc func(
	GroupId uint64,
	GroupPubKey common.Hash,
	blockNumber uint64,
)

func (grs *RbGroupRegisteredSubscription) OnEvent(
	handler randomBeaconGroupRegisteredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconGroupRegistered)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.GroupId,
					event.GroupPubKey,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := grs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (grs *RbGroupRegisteredSubscription) Pipe(
	sink chan *abi.RandomBeaconGroupRegistered,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(grs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := grs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - grs.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past GroupRegistered events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := grs.contract.PastGroupRegisteredEvents(
					fromBlock,
					nil,
					grs.groupIdFilter,
					grs.groupPubKeyFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past GroupRegistered events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := grs.contract.watchGroupRegistered(
		sink,
		grs.groupIdFilter,
		grs.groupPubKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchGroupRegistered(
	sink chan *abi.RandomBeaconGroupRegistered,
	groupIdFilter []uint64,
	groupPubKeyFilter [][]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchGroupRegistered(
			&bind.WatchOpts{Context: ctx},
			sink,
			groupIdFilter,
			groupPubKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event GroupRegistered had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event GroupRegistered failed "+
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

func (rb *RandomBeacon) PastGroupRegisteredEvents(
	startBlock uint64,
	endBlock *uint64,
	groupIdFilter []uint64,
	groupPubKeyFilter [][]byte,
) ([]*abi.RandomBeaconGroupRegistered, error) {
	iterator, err := rb.contract.FilterGroupRegistered(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		groupIdFilter,
		groupPubKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past GroupRegistered events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconGroupRegistered, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) InactivityClaimedEvent(
	opts *ethereum.SubscribeOpts,
	groupIdFilter []uint64,
) *RbInactivityClaimedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbInactivityClaimedSubscription{
		rb,
		opts,
		groupIdFilter,
	}
}

type RbInactivityClaimedSubscription struct {
	contract      *RandomBeacon
	opts          *ethereum.SubscribeOpts
	groupIdFilter []uint64
}

type randomBeaconInactivityClaimedFunc func(
	GroupId uint64,
	Nonce *big.Int,
	Notifier common.Address,
	blockNumber uint64,
)

func (ics *RbInactivityClaimedSubscription) OnEvent(
	handler randomBeaconInactivityClaimedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconInactivityClaimed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.GroupId,
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

func (ics *RbInactivityClaimedSubscription) Pipe(
	sink chan *abi.RandomBeaconInactivityClaimed,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ics.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past InactivityClaimed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ics.contract.PastInactivityClaimedEvents(
					fromBlock,
					nil,
					ics.groupIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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
		ics.groupIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchInactivityClaimed(
	sink chan *abi.RandomBeaconInactivityClaimed,
	groupIdFilter []uint64,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchInactivityClaimed(
			&bind.WatchOpts{Context: ctx},
			sink,
			groupIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event InactivityClaimed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastInactivityClaimedEvents(
	startBlock uint64,
	endBlock *uint64,
	groupIdFilter []uint64,
) ([]*abi.RandomBeaconInactivityClaimed, error) {
	iterator, err := rb.contract.FilterInactivityClaimed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		groupIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past InactivityClaimed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconInactivityClaimed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) InvoluntaryAuthorizationDecreaseFailedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *RbInvoluntaryAuthorizationDecreaseFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbInvoluntaryAuthorizationDecreaseFailedSubscription{
		rb,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type RbInvoluntaryAuthorizationDecreaseFailedSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type randomBeaconInvoluntaryAuthorizationDecreaseFailedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	blockNumber uint64,
)

func (iadfs *RbInvoluntaryAuthorizationDecreaseFailedSubscription) OnEvent(
	handler randomBeaconInvoluntaryAuthorizationDecreaseFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconInvoluntaryAuthorizationDecreaseFailed)
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

func (iadfs *RbInvoluntaryAuthorizationDecreaseFailedSubscription) Pipe(
	sink chan *abi.RandomBeaconInvoluntaryAuthorizationDecreaseFailed,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - iadfs.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchInvoluntaryAuthorizationDecreaseFailed(
	sink chan *abi.RandomBeaconInvoluntaryAuthorizationDecreaseFailed,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchInvoluntaryAuthorizationDecreaseFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event InvoluntaryAuthorizationDecreaseFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastInvoluntaryAuthorizationDecreaseFailedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.RandomBeaconInvoluntaryAuthorizationDecreaseFailed, error) {
	iterator, err := rb.contract.FilterInvoluntaryAuthorizationDecreaseFailed(
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

	events := make([]*abi.RandomBeaconInvoluntaryAuthorizationDecreaseFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) OperatorJoinedSortitionPoolEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *RbOperatorJoinedSortitionPoolSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbOperatorJoinedSortitionPoolSubscription{
		rb,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type RbOperatorJoinedSortitionPoolSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type randomBeaconOperatorJoinedSortitionPoolFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	blockNumber uint64,
)

func (ojsps *RbOperatorJoinedSortitionPoolSubscription) OnEvent(
	handler randomBeaconOperatorJoinedSortitionPoolFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconOperatorJoinedSortitionPool)
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

func (ojsps *RbOperatorJoinedSortitionPoolSubscription) Pipe(
	sink chan *abi.RandomBeaconOperatorJoinedSortitionPool,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ojsps.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchOperatorJoinedSortitionPool(
	sink chan *abi.RandomBeaconOperatorJoinedSortitionPool,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchOperatorJoinedSortitionPool(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event OperatorJoinedSortitionPool had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastOperatorJoinedSortitionPoolEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.RandomBeaconOperatorJoinedSortitionPool, error) {
	iterator, err := rb.contract.FilterOperatorJoinedSortitionPool(
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

	events := make([]*abi.RandomBeaconOperatorJoinedSortitionPool, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) OperatorRegisteredEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *RbOperatorRegisteredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbOperatorRegisteredSubscription{
		rb,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type RbOperatorRegisteredSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type randomBeaconOperatorRegisteredFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	blockNumber uint64,
)

func (ors *RbOperatorRegisteredSubscription) OnEvent(
	handler randomBeaconOperatorRegisteredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconOperatorRegistered)
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

func (ors *RbOperatorRegisteredSubscription) Pipe(
	sink chan *abi.RandomBeaconOperatorRegistered,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ors.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchOperatorRegistered(
	sink chan *abi.RandomBeaconOperatorRegistered,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchOperatorRegistered(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event OperatorRegistered had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastOperatorRegisteredEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.RandomBeaconOperatorRegistered, error) {
	iterator, err := rb.contract.FilterOperatorRegistered(
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

	events := make([]*abi.RandomBeaconOperatorRegistered, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) OperatorStatusUpdatedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) *RbOperatorStatusUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbOperatorStatusUpdatedSubscription{
		rb,
		opts,
		stakingProviderFilter,
		operatorFilter,
	}
}

type RbOperatorStatusUpdatedSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	operatorFilter        []common.Address
}

type randomBeaconOperatorStatusUpdatedFunc func(
	StakingProvider common.Address,
	Operator common.Address,
	blockNumber uint64,
)

func (osus *RbOperatorStatusUpdatedSubscription) OnEvent(
	handler randomBeaconOperatorStatusUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconOperatorStatusUpdated)
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

func (osus *RbOperatorStatusUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconOperatorStatusUpdated,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - osus.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchOperatorStatusUpdated(
	sink chan *abi.RandomBeaconOperatorStatusUpdated,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchOperatorStatusUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			operatorFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event OperatorStatusUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastOperatorStatusUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	operatorFilter []common.Address,
) ([]*abi.RandomBeaconOperatorStatusUpdated, error) {
	iterator, err := rb.contract.FilterOperatorStatusUpdated(
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

	events := make([]*abi.RandomBeaconOperatorStatusUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) ReimbursementPoolUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RbReimbursementPoolUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbReimbursementPoolUpdatedSubscription{
		rb,
		opts,
	}
}

type RbReimbursementPoolUpdatedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconReimbursementPoolUpdatedFunc func(
	NewReimbursementPool common.Address,
	blockNumber uint64,
)

func (rpus *RbReimbursementPoolUpdatedSubscription) OnEvent(
	handler randomBeaconReimbursementPoolUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconReimbursementPoolUpdated)
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

func (rpus *RbReimbursementPoolUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconReimbursementPoolUpdated,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past ReimbursementPoolUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastReimbursementPoolUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchReimbursementPoolUpdated(
	sink chan *abi.RandomBeaconReimbursementPoolUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchReimbursementPoolUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event ReimbursementPoolUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastReimbursementPoolUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconReimbursementPoolUpdated, error) {
	iterator, err := rb.contract.FilterReimbursementPoolUpdated(
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

	events := make([]*abi.RandomBeaconReimbursementPoolUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntryDelaySlashedEvent(
	opts *ethereum.SubscribeOpts,
	requestIdFilter []*big.Int,
) *RbRelayEntryDelaySlashedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntryDelaySlashedSubscription{
		rb,
		opts,
		requestIdFilter,
	}
}

type RbRelayEntryDelaySlashedSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requestIdFilter []*big.Int
}

type randomBeaconRelayEntryDelaySlashedFunc func(
	RequestId *big.Int,
	SlashingAmount *big.Int,
	GroupMembers []common.Address,
	blockNumber uint64,
)

func (redss *RbRelayEntryDelaySlashedSubscription) OnEvent(
	handler randomBeaconRelayEntryDelaySlashedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntryDelaySlashed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RequestId,
					event.SlashingAmount,
					event.GroupMembers,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := redss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (redss *RbRelayEntryDelaySlashedSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntryDelaySlashed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(redss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := redss.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - redss.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntryDelaySlashed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := redss.contract.PastRelayEntryDelaySlashedEvents(
					fromBlock,
					nil,
					redss.requestIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntryDelaySlashed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := redss.contract.watchRelayEntryDelaySlashed(
		sink,
		redss.requestIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntryDelaySlashed(
	sink chan *abi.RandomBeaconRelayEntryDelaySlashed,
	requestIdFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntryDelaySlashed(
			&bind.WatchOpts{Context: ctx},
			sink,
			requestIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntryDelaySlashed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntryDelaySlashed failed "+
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

func (rb *RandomBeacon) PastRelayEntryDelaySlashedEvents(
	startBlock uint64,
	endBlock *uint64,
	requestIdFilter []*big.Int,
) ([]*abi.RandomBeaconRelayEntryDelaySlashed, error) {
	iterator, err := rb.contract.FilterRelayEntryDelaySlashed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requestIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntryDelaySlashed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntryDelaySlashed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntryDelaySlashingFailedEvent(
	opts *ethereum.SubscribeOpts,
	requestIdFilter []*big.Int,
) *RbRelayEntryDelaySlashingFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntryDelaySlashingFailedSubscription{
		rb,
		opts,
		requestIdFilter,
	}
}

type RbRelayEntryDelaySlashingFailedSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requestIdFilter []*big.Int
}

type randomBeaconRelayEntryDelaySlashingFailedFunc func(
	RequestId *big.Int,
	SlashingAmount *big.Int,
	GroupMembers []common.Address,
	blockNumber uint64,
)

func (redsfs *RbRelayEntryDelaySlashingFailedSubscription) OnEvent(
	handler randomBeaconRelayEntryDelaySlashingFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntryDelaySlashingFailed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RequestId,
					event.SlashingAmount,
					event.GroupMembers,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := redsfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (redsfs *RbRelayEntryDelaySlashingFailedSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntryDelaySlashingFailed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(redsfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := redsfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - redsfs.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntryDelaySlashingFailed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := redsfs.contract.PastRelayEntryDelaySlashingFailedEvents(
					fromBlock,
					nil,
					redsfs.requestIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntryDelaySlashingFailed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := redsfs.contract.watchRelayEntryDelaySlashingFailed(
		sink,
		redsfs.requestIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntryDelaySlashingFailed(
	sink chan *abi.RandomBeaconRelayEntryDelaySlashingFailed,
	requestIdFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntryDelaySlashingFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
			requestIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntryDelaySlashingFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntryDelaySlashingFailed failed "+
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

func (rb *RandomBeacon) PastRelayEntryDelaySlashingFailedEvents(
	startBlock uint64,
	endBlock *uint64,
	requestIdFilter []*big.Int,
) ([]*abi.RandomBeaconRelayEntryDelaySlashingFailed, error) {
	iterator, err := rb.contract.FilterRelayEntryDelaySlashingFailed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requestIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntryDelaySlashingFailed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntryDelaySlashingFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntryParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RbRelayEntryParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntryParametersUpdatedSubscription{
		rb,
		opts,
	}
}

type RbRelayEntryParametersUpdatedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconRelayEntryParametersUpdatedFunc func(
	RelayEntrySoftTimeout *big.Int,
	RelayEntryHardTimeout *big.Int,
	CallbackGasLimit *big.Int,
	blockNumber uint64,
)

func (repus *RbRelayEntryParametersUpdatedSubscription) OnEvent(
	handler randomBeaconRelayEntryParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntryParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RelayEntrySoftTimeout,
					event.RelayEntryHardTimeout,
					event.CallbackGasLimit,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := repus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (repus *RbRelayEntryParametersUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntryParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(repus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := repus.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - repus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntryParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := repus.contract.PastRelayEntryParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntryParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := repus.contract.watchRelayEntryParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntryParametersUpdated(
	sink chan *abi.RandomBeaconRelayEntryParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntryParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntryParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntryParametersUpdated failed "+
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

func (rb *RandomBeacon) PastRelayEntryParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconRelayEntryParametersUpdated, error) {
	iterator, err := rb.contract.FilterRelayEntryParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntryParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntryParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntryRequestedEvent(
	opts *ethereum.SubscribeOpts,
	requestIdFilter []*big.Int,
) *RbRelayEntryRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntryRequestedSubscription{
		rb,
		opts,
		requestIdFilter,
	}
}

type RbRelayEntryRequestedSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requestIdFilter []*big.Int
}

type randomBeaconRelayEntryRequestedFunc func(
	RequestId *big.Int,
	GroupId uint64,
	PreviousEntry []byte,
	blockNumber uint64,
)

func (rers *RbRelayEntryRequestedSubscription) OnEvent(
	handler randomBeaconRelayEntryRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntryRequested)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RequestId,
					event.GroupId,
					event.PreviousEntry,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rers.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rers *RbRelayEntryRequestedSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntryRequested,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rers.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rers.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rers.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntryRequested events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rers.contract.PastRelayEntryRequestedEvents(
					fromBlock,
					nil,
					rers.requestIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntryRequested events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rers.contract.watchRelayEntryRequested(
		sink,
		rers.requestIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntryRequested(
	sink chan *abi.RandomBeaconRelayEntryRequested,
	requestIdFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntryRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
			requestIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntryRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntryRequested failed "+
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

func (rb *RandomBeacon) PastRelayEntryRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
	requestIdFilter []*big.Int,
) ([]*abi.RandomBeaconRelayEntryRequested, error) {
	iterator, err := rb.contract.FilterRelayEntryRequested(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requestIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntryRequested events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntryRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntrySubmittedEvent(
	opts *ethereum.SubscribeOpts,
	requestIdFilter []*big.Int,
) *RbRelayEntrySubmittedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntrySubmittedSubscription{
		rb,
		opts,
		requestIdFilter,
	}
}

type RbRelayEntrySubmittedSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requestIdFilter []*big.Int
}

type randomBeaconRelayEntrySubmittedFunc func(
	RequestId *big.Int,
	Submitter common.Address,
	Entry []byte,
	blockNumber uint64,
)

func (ress *RbRelayEntrySubmittedSubscription) OnEvent(
	handler randomBeaconRelayEntrySubmittedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntrySubmitted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RequestId,
					event.Submitter,
					event.Entry,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ress.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ress *RbRelayEntrySubmittedSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntrySubmitted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ress.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ress.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ress.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntrySubmitted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ress.contract.PastRelayEntrySubmittedEvents(
					fromBlock,
					nil,
					ress.requestIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntrySubmitted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ress.contract.watchRelayEntrySubmitted(
		sink,
		ress.requestIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntrySubmitted(
	sink chan *abi.RandomBeaconRelayEntrySubmitted,
	requestIdFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntrySubmitted(
			&bind.WatchOpts{Context: ctx},
			sink,
			requestIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntrySubmitted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntrySubmitted failed "+
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

func (rb *RandomBeacon) PastRelayEntrySubmittedEvents(
	startBlock uint64,
	endBlock *uint64,
	requestIdFilter []*big.Int,
) ([]*abi.RandomBeaconRelayEntrySubmitted, error) {
	iterator, err := rb.contract.FilterRelayEntrySubmitted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requestIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntrySubmitted events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntrySubmitted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntryTimedOutEvent(
	opts *ethereum.SubscribeOpts,
	requestIdFilter []*big.Int,
) *RbRelayEntryTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntryTimedOutSubscription{
		rb,
		opts,
		requestIdFilter,
	}
}

type RbRelayEntryTimedOutSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requestIdFilter []*big.Int
}

type randomBeaconRelayEntryTimedOutFunc func(
	RequestId *big.Int,
	TerminatedGroupId uint64,
	blockNumber uint64,
)

func (retos *RbRelayEntryTimedOutSubscription) OnEvent(
	handler randomBeaconRelayEntryTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntryTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RequestId,
					event.TerminatedGroupId,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := retos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (retos *RbRelayEntryTimedOutSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntryTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(retos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := retos.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - retos.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntryTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := retos.contract.PastRelayEntryTimedOutEvents(
					fromBlock,
					nil,
					retos.requestIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntryTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := retos.contract.watchRelayEntryTimedOut(
		sink,
		retos.requestIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntryTimedOut(
	sink chan *abi.RandomBeaconRelayEntryTimedOut,
	requestIdFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntryTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
			requestIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntryTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntryTimedOut failed "+
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

func (rb *RandomBeacon) PastRelayEntryTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
	requestIdFilter []*big.Int,
) ([]*abi.RandomBeaconRelayEntryTimedOut, error) {
	iterator, err := rb.contract.FilterRelayEntryTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requestIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntryTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntryTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntryTimeoutSlashedEvent(
	opts *ethereum.SubscribeOpts,
	requestIdFilter []*big.Int,
) *RbRelayEntryTimeoutSlashedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntryTimeoutSlashedSubscription{
		rb,
		opts,
		requestIdFilter,
	}
}

type RbRelayEntryTimeoutSlashedSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requestIdFilter []*big.Int
}

type randomBeaconRelayEntryTimeoutSlashedFunc func(
	RequestId *big.Int,
	SlashingAmount *big.Int,
	GroupMembers []common.Address,
	blockNumber uint64,
)

func (retss *RbRelayEntryTimeoutSlashedSubscription) OnEvent(
	handler randomBeaconRelayEntryTimeoutSlashedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntryTimeoutSlashed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RequestId,
					event.SlashingAmount,
					event.GroupMembers,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := retss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (retss *RbRelayEntryTimeoutSlashedSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntryTimeoutSlashed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(retss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := retss.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - retss.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntryTimeoutSlashed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := retss.contract.PastRelayEntryTimeoutSlashedEvents(
					fromBlock,
					nil,
					retss.requestIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntryTimeoutSlashed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := retss.contract.watchRelayEntryTimeoutSlashed(
		sink,
		retss.requestIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntryTimeoutSlashed(
	sink chan *abi.RandomBeaconRelayEntryTimeoutSlashed,
	requestIdFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntryTimeoutSlashed(
			&bind.WatchOpts{Context: ctx},
			sink,
			requestIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntryTimeoutSlashed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntryTimeoutSlashed failed "+
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

func (rb *RandomBeacon) PastRelayEntryTimeoutSlashedEvents(
	startBlock uint64,
	endBlock *uint64,
	requestIdFilter []*big.Int,
) ([]*abi.RandomBeaconRelayEntryTimeoutSlashed, error) {
	iterator, err := rb.contract.FilterRelayEntryTimeoutSlashed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requestIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntryTimeoutSlashed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntryTimeoutSlashed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RelayEntryTimeoutSlashingFailedEvent(
	opts *ethereum.SubscribeOpts,
	requestIdFilter []*big.Int,
) *RbRelayEntryTimeoutSlashingFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRelayEntryTimeoutSlashingFailedSubscription{
		rb,
		opts,
		requestIdFilter,
	}
}

type RbRelayEntryTimeoutSlashingFailedSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requestIdFilter []*big.Int
}

type randomBeaconRelayEntryTimeoutSlashingFailedFunc func(
	RequestId *big.Int,
	SlashingAmount *big.Int,
	GroupMembers []common.Address,
	blockNumber uint64,
)

func (retsfs *RbRelayEntryTimeoutSlashingFailedSubscription) OnEvent(
	handler randomBeaconRelayEntryTimeoutSlashingFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRelayEntryTimeoutSlashingFailed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RequestId,
					event.SlashingAmount,
					event.GroupMembers,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := retsfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (retsfs *RbRelayEntryTimeoutSlashingFailedSubscription) Pipe(
	sink chan *abi.RandomBeaconRelayEntryTimeoutSlashingFailed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(retsfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := retsfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - retsfs.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RelayEntryTimeoutSlashingFailed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := retsfs.contract.PastRelayEntryTimeoutSlashingFailedEvents(
					fromBlock,
					nil,
					retsfs.requestIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RelayEntryTimeoutSlashingFailed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := retsfs.contract.watchRelayEntryTimeoutSlashingFailed(
		sink,
		retsfs.requestIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRelayEntryTimeoutSlashingFailed(
	sink chan *abi.RandomBeaconRelayEntryTimeoutSlashingFailed,
	requestIdFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRelayEntryTimeoutSlashingFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
			requestIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RelayEntryTimeoutSlashingFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RelayEntryTimeoutSlashingFailed failed "+
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

func (rb *RandomBeacon) PastRelayEntryTimeoutSlashingFailedEvents(
	startBlock uint64,
	endBlock *uint64,
	requestIdFilter []*big.Int,
) ([]*abi.RandomBeaconRelayEntryTimeoutSlashingFailed, error) {
	iterator, err := rb.contract.FilterRelayEntryTimeoutSlashingFailed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requestIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RelayEntryTimeoutSlashingFailed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRelayEntryTimeoutSlashingFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RequesterAuthorizationUpdatedEvent(
	opts *ethereum.SubscribeOpts,
	requesterFilter []common.Address,
) *RbRequesterAuthorizationUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRequesterAuthorizationUpdatedSubscription{
		rb,
		opts,
		requesterFilter,
	}
}

type RbRequesterAuthorizationUpdatedSubscription struct {
	contract        *RandomBeacon
	opts            *ethereum.SubscribeOpts
	requesterFilter []common.Address
}

type randomBeaconRequesterAuthorizationUpdatedFunc func(
	Requester common.Address,
	IsAuthorized bool,
	blockNumber uint64,
)

func (raus *RbRequesterAuthorizationUpdatedSubscription) OnEvent(
	handler randomBeaconRequesterAuthorizationUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRequesterAuthorizationUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Requester,
					event.IsAuthorized,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := raus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (raus *RbRequesterAuthorizationUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconRequesterAuthorizationUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(raus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := raus.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - raus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RequesterAuthorizationUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := raus.contract.PastRequesterAuthorizationUpdatedEvents(
					fromBlock,
					nil,
					raus.requesterFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past RequesterAuthorizationUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := raus.contract.watchRequesterAuthorizationUpdated(
		sink,
		raus.requesterFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchRequesterAuthorizationUpdated(
	sink chan *abi.RandomBeaconRequesterAuthorizationUpdated,
	requesterFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRequesterAuthorizationUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
			requesterFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RequesterAuthorizationUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event RequesterAuthorizationUpdated failed "+
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

func (rb *RandomBeacon) PastRequesterAuthorizationUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
	requesterFilter []common.Address,
) ([]*abi.RandomBeaconRequesterAuthorizationUpdated, error) {
	iterator, err := rb.contract.FilterRequesterAuthorizationUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		requesterFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RequesterAuthorizationUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconRequesterAuthorizationUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RewardParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RbRewardParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRewardParametersUpdatedSubscription{
		rb,
		opts,
	}
}

type RbRewardParametersUpdatedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconRewardParametersUpdatedFunc func(
	SortitionPoolRewardsBanDuration *big.Int,
	RelayEntryTimeoutNotificationRewardMultiplier *big.Int,
	UnauthorizedSigningNotificationRewardMultiplier *big.Int,
	DkgMaliciousResultNotificationRewardMultiplier *big.Int,
	blockNumber uint64,
)

func (rpus *RbRewardParametersUpdatedSubscription) OnEvent(
	handler randomBeaconRewardParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRewardParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.SortitionPoolRewardsBanDuration,
					event.RelayEntryTimeoutNotificationRewardMultiplier,
					event.UnauthorizedSigningNotificationRewardMultiplier,
					event.DkgMaliciousResultNotificationRewardMultiplier,
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

func (rpus *RbRewardParametersUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconRewardParametersUpdated,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past RewardParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastRewardParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchRewardParametersUpdated(
	sink chan *abi.RandomBeaconRewardParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRewardParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RewardParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastRewardParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconRewardParametersUpdated, error) {
	iterator, err := rb.contract.FilterRewardParametersUpdated(
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

	events := make([]*abi.RandomBeaconRewardParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) RewardsWithdrawnEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
) *RbRewardsWithdrawnSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbRewardsWithdrawnSubscription{
		rb,
		opts,
		stakingProviderFilter,
	}
}

type RbRewardsWithdrawnSubscription struct {
	contract              *RandomBeacon
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
}

type randomBeaconRewardsWithdrawnFunc func(
	StakingProvider common.Address,
	Amount *big.Int,
	blockNumber uint64,
)

func (rws *RbRewardsWithdrawnSubscription) OnEvent(
	handler randomBeaconRewardsWithdrawnFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconRewardsWithdrawn)
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

func (rws *RbRewardsWithdrawnSubscription) Pipe(
	sink chan *abi.RandomBeaconRewardsWithdrawn,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rws.opts.PastBlocks

				rbLogger.Infof(
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchRewardsWithdrawn(
	sink chan *abi.RandomBeaconRewardsWithdrawn,
	stakingProviderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchRewardsWithdrawn(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event RewardsWithdrawn had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastRewardsWithdrawnEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
) ([]*abi.RandomBeaconRewardsWithdrawn, error) {
	iterator, err := rb.contract.FilterRewardsWithdrawn(
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

	events := make([]*abi.RandomBeaconRewardsWithdrawn, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) SlashingParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RbSlashingParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbSlashingParametersUpdatedSubscription{
		rb,
		opts,
	}
}

type RbSlashingParametersUpdatedSubscription struct {
	contract *RandomBeacon
	opts     *ethereum.SubscribeOpts
}

type randomBeaconSlashingParametersUpdatedFunc func(
	RelayEntrySubmissionFailureSlashingAmount *big.Int,
	MaliciousDkgResultSlashingAmount *big.Int,
	UnauthorizedSigningSlashingAmount *big.Int,
	blockNumber uint64,
)

func (spus *RbSlashingParametersUpdatedSubscription) OnEvent(
	handler randomBeaconSlashingParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconSlashingParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RelayEntrySubmissionFailureSlashingAmount,
					event.MaliciousDkgResultSlashingAmount,
					event.UnauthorizedSigningSlashingAmount,
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

func (spus *RbSlashingParametersUpdatedSubscription) Pipe(
	sink chan *abi.RandomBeaconSlashingParametersUpdated,
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
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - spus.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past SlashingParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := spus.contract.PastSlashingParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
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

func (rb *RandomBeacon) watchSlashingParametersUpdated(
	sink chan *abi.RandomBeaconSlashingParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchSlashingParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event SlashingParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
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

func (rb *RandomBeacon) PastSlashingParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RandomBeaconSlashingParametersUpdated, error) {
	iterator, err := rb.contract.FilterSlashingParametersUpdated(
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

	events := make([]*abi.RandomBeaconSlashingParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) UnauthorizedSigningSlashedEvent(
	opts *ethereum.SubscribeOpts,
	groupIdFilter []uint64,
) *RbUnauthorizedSigningSlashedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbUnauthorizedSigningSlashedSubscription{
		rb,
		opts,
		groupIdFilter,
	}
}

type RbUnauthorizedSigningSlashedSubscription struct {
	contract      *RandomBeacon
	opts          *ethereum.SubscribeOpts
	groupIdFilter []uint64
}

type randomBeaconUnauthorizedSigningSlashedFunc func(
	GroupId uint64,
	UnauthorizedSigningSlashingAmount *big.Int,
	GroupMembers []common.Address,
	blockNumber uint64,
)

func (usss *RbUnauthorizedSigningSlashedSubscription) OnEvent(
	handler randomBeaconUnauthorizedSigningSlashedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconUnauthorizedSigningSlashed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.GroupId,
					event.UnauthorizedSigningSlashingAmount,
					event.GroupMembers,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := usss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (usss *RbUnauthorizedSigningSlashedSubscription) Pipe(
	sink chan *abi.RandomBeaconUnauthorizedSigningSlashed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(usss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := usss.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - usss.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past UnauthorizedSigningSlashed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := usss.contract.PastUnauthorizedSigningSlashedEvents(
					fromBlock,
					nil,
					usss.groupIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past UnauthorizedSigningSlashed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := usss.contract.watchUnauthorizedSigningSlashed(
		sink,
		usss.groupIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchUnauthorizedSigningSlashed(
	sink chan *abi.RandomBeaconUnauthorizedSigningSlashed,
	groupIdFilter []uint64,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchUnauthorizedSigningSlashed(
			&bind.WatchOpts{Context: ctx},
			sink,
			groupIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event UnauthorizedSigningSlashed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event UnauthorizedSigningSlashed failed "+
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

func (rb *RandomBeacon) PastUnauthorizedSigningSlashedEvents(
	startBlock uint64,
	endBlock *uint64,
	groupIdFilter []uint64,
) ([]*abi.RandomBeaconUnauthorizedSigningSlashed, error) {
	iterator, err := rb.contract.FilterUnauthorizedSigningSlashed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		groupIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past UnauthorizedSigningSlashed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconUnauthorizedSigningSlashed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rb *RandomBeacon) UnauthorizedSigningSlashingFailedEvent(
	opts *ethereum.SubscribeOpts,
	groupIdFilter []uint64,
) *RbUnauthorizedSigningSlashingFailedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RbUnauthorizedSigningSlashingFailedSubscription{
		rb,
		opts,
		groupIdFilter,
	}
}

type RbUnauthorizedSigningSlashingFailedSubscription struct {
	contract      *RandomBeacon
	opts          *ethereum.SubscribeOpts
	groupIdFilter []uint64
}

type randomBeaconUnauthorizedSigningSlashingFailedFunc func(
	GroupId uint64,
	UnauthorizedSigningSlashingAmount *big.Int,
	GroupMembers []common.Address,
	blockNumber uint64,
)

func (ussfs *RbUnauthorizedSigningSlashingFailedSubscription) OnEvent(
	handler randomBeaconUnauthorizedSigningSlashingFailedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RandomBeaconUnauthorizedSigningSlashingFailed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.GroupId,
					event.UnauthorizedSigningSlashingAmount,
					event.GroupMembers,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ussfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ussfs *RbUnauthorizedSigningSlashingFailedSubscription) Pipe(
	sink chan *abi.RandomBeaconUnauthorizedSigningSlashingFailed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ussfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ussfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ussfs.opts.PastBlocks

				rbLogger.Infof(
					"subscription monitoring fetching past UnauthorizedSigningSlashingFailed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ussfs.contract.PastUnauthorizedSigningSlashingFailedEvents(
					fromBlock,
					nil,
					ussfs.groupIdFilter,
				)
				if err != nil {
					rbLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rbLogger.Infof(
					"subscription monitoring fetched [%v] past UnauthorizedSigningSlashingFailed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ussfs.contract.watchUnauthorizedSigningSlashingFailed(
		sink,
		ussfs.groupIdFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rb *RandomBeacon) watchUnauthorizedSigningSlashingFailed(
	sink chan *abi.RandomBeaconUnauthorizedSigningSlashingFailed,
	groupIdFilter []uint64,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rb.contract.WatchUnauthorizedSigningSlashingFailed(
			&bind.WatchOpts{Context: ctx},
			sink,
			groupIdFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rbLogger.Warnf(
			"subscription to event UnauthorizedSigningSlashingFailed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rbLogger.Errorf(
			"subscription to event UnauthorizedSigningSlashingFailed failed "+
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

func (rb *RandomBeacon) PastUnauthorizedSigningSlashingFailedEvents(
	startBlock uint64,
	endBlock *uint64,
	groupIdFilter []uint64,
) ([]*abi.RandomBeaconUnauthorizedSigningSlashingFailed, error) {
	iterator, err := rb.contract.FilterUnauthorizedSigningSlashingFailed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		groupIdFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past UnauthorizedSigningSlashingFailed events: [%v]",
			err,
		)
	}

	events := make([]*abi.RandomBeaconUnauthorizedSigningSlashingFailed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

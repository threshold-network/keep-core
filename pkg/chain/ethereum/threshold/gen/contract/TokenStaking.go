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
	"github.com/keep-network/keep-core/pkg/chain/ethereum/threshold/gen/abi"
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var tsLogger = log.Logger("keep-contract-TokenStaking")

type TokenStaking struct {
	contract          *abi.TokenStaking
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

func NewTokenStaking(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*TokenStaking, error) {
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

	contract, err := abi.NewTokenStaking(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.TokenStakingABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &TokenStaking{
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
func (ts *TokenStaking) ApproveApplication(
	arg_application common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction approveApplication",
		" params: ",
		fmt.Sprint(
			arg_application,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.ApproveApplication(
		transactorOptions,
		arg_application,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"approveApplication",
			arg_application,
		)
	}

	tsLogger.Infof(
		"submitted transaction approveApplication with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.ApproveApplication(
				newTransactorOptions,
				arg_application,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"approveApplication",
					arg_application,
				)
			}

			tsLogger.Infof(
				"submitted transaction approveApplication with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallApproveApplication(
	arg_application common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"approveApplication",
		&result,
		arg_application,
	)

	return err
}

func (ts *TokenStaking) ApproveApplicationGasEstimate(
	arg_application common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"approveApplication",
		ts.contractABI,
		ts.transactor,
		arg_application,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) ApproveAuthorizationDecrease(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction approveAuthorizationDecrease",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.ApproveAuthorizationDecrease(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"approveAuthorizationDecrease",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction approveAuthorizationDecrease with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.ApproveAuthorizationDecrease(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"approveAuthorizationDecrease",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction approveAuthorizationDecrease with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallApproveAuthorizationDecrease(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"approveAuthorizationDecrease",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (ts *TokenStaking) ApproveAuthorizationDecreaseGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"approveAuthorizationDecrease",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) DelegateVoting(
	arg_stakingProvider common.Address,
	arg_delegatee common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction delegateVoting",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_delegatee,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.DelegateVoting(
		transactorOptions,
		arg_stakingProvider,
		arg_delegatee,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"delegateVoting",
			arg_stakingProvider,
			arg_delegatee,
		)
	}

	tsLogger.Infof(
		"submitted transaction delegateVoting with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.DelegateVoting(
				newTransactorOptions,
				arg_stakingProvider,
				arg_delegatee,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"delegateVoting",
					arg_stakingProvider,
					arg_delegatee,
				)
			}

			tsLogger.Infof(
				"submitted transaction delegateVoting with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallDelegateVoting(
	arg_stakingProvider common.Address,
	arg_delegatee common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"delegateVoting",
		&result,
		arg_stakingProvider,
		arg_delegatee,
	)

	return err
}

func (ts *TokenStaking) DelegateVotingGasEstimate(
	arg_stakingProvider common.Address,
	arg_delegatee common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"delegateVoting",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_delegatee,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) DisableApplication(
	arg_application common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction disableApplication",
		" params: ",
		fmt.Sprint(
			arg_application,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.DisableApplication(
		transactorOptions,
		arg_application,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"disableApplication",
			arg_application,
		)
	}

	tsLogger.Infof(
		"submitted transaction disableApplication with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.DisableApplication(
				newTransactorOptions,
				arg_application,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"disableApplication",
					arg_application,
				)
			}

			tsLogger.Infof(
				"submitted transaction disableApplication with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallDisableApplication(
	arg_application common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"disableApplication",
		&result,
		arg_application,
	)

	return err
}

func (ts *TokenStaking) DisableApplicationGasEstimate(
	arg_application common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"disableApplication",
		ts.contractABI,
		ts.transactor,
		arg_application,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) ForceDecreaseAuthorization(
	arg_stakingProvider common.Address,
	arg_application common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction forceDecreaseAuthorization",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_application,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.ForceDecreaseAuthorization(
		transactorOptions,
		arg_stakingProvider,
		arg_application,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"forceDecreaseAuthorization",
			arg_stakingProvider,
			arg_application,
		)
	}

	tsLogger.Infof(
		"submitted transaction forceDecreaseAuthorization with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.ForceDecreaseAuthorization(
				newTransactorOptions,
				arg_stakingProvider,
				arg_application,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"forceDecreaseAuthorization",
					arg_stakingProvider,
					arg_application,
				)
			}

			tsLogger.Infof(
				"submitted transaction forceDecreaseAuthorization with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallForceDecreaseAuthorization(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"forceDecreaseAuthorization",
		&result,
		arg_stakingProvider,
		arg_application,
	)

	return err
}

func (ts *TokenStaking) ForceDecreaseAuthorizationGasEstimate(
	arg_stakingProvider common.Address,
	arg_application common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"forceDecreaseAuthorization",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_application,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) IncreaseAuthorization(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction increaseAuthorization",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_application,
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.IncreaseAuthorization(
		transactorOptions,
		arg_stakingProvider,
		arg_application,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"increaseAuthorization",
			arg_stakingProvider,
			arg_application,
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction increaseAuthorization with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.IncreaseAuthorization(
				newTransactorOptions,
				arg_stakingProvider,
				arg_application,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"increaseAuthorization",
					arg_stakingProvider,
					arg_application,
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction increaseAuthorization with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallIncreaseAuthorization(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"increaseAuthorization",
		&result,
		arg_stakingProvider,
		arg_application,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) IncreaseAuthorizationGasEstimate(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"increaseAuthorization",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_application,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) Initialize(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction initialize",
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.Initialize(
		transactorOptions,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"initialize",
		)
	}

	tsLogger.Infof(
		"submitted transaction initialize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.Initialize(
				newTransactorOptions,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"initialize",
				)
			}

			tsLogger.Infof(
				"submitted transaction initialize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallInitialize(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"initialize",
		&result,
	)

	return err
}

func (ts *TokenStaking) InitializeGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"initialize",
		ts.contractABI,
		ts.transactor,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) NotifyKeepStakeDiscrepancy(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction notifyKeepStakeDiscrepancy",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.NotifyKeepStakeDiscrepancy(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"notifyKeepStakeDiscrepancy",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction notifyKeepStakeDiscrepancy with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.NotifyKeepStakeDiscrepancy(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"notifyKeepStakeDiscrepancy",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction notifyKeepStakeDiscrepancy with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallNotifyKeepStakeDiscrepancy(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"notifyKeepStakeDiscrepancy",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) NotifyKeepStakeDiscrepancyGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"notifyKeepStakeDiscrepancy",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) NotifyNuStakeDiscrepancy(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction notifyNuStakeDiscrepancy",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.NotifyNuStakeDiscrepancy(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"notifyNuStakeDiscrepancy",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction notifyNuStakeDiscrepancy with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.NotifyNuStakeDiscrepancy(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"notifyNuStakeDiscrepancy",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction notifyNuStakeDiscrepancy with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallNotifyNuStakeDiscrepancy(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"notifyNuStakeDiscrepancy",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) NotifyNuStakeDiscrepancyGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"notifyNuStakeDiscrepancy",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) PauseApplication(
	arg_application common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction pauseApplication",
		" params: ",
		fmt.Sprint(
			arg_application,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.PauseApplication(
		transactorOptions,
		arg_application,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"pauseApplication",
			arg_application,
		)
	}

	tsLogger.Infof(
		"submitted transaction pauseApplication with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.PauseApplication(
				newTransactorOptions,
				arg_application,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"pauseApplication",
					arg_application,
				)
			}

			tsLogger.Infof(
				"submitted transaction pauseApplication with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallPauseApplication(
	arg_application common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"pauseApplication",
		&result,
		arg_application,
	)

	return err
}

func (ts *TokenStaking) PauseApplicationGasEstimate(
	arg_application common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"pauseApplication",
		ts.contractABI,
		ts.transactor,
		arg_application,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) ProcessSlashing(
	arg_count *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction processSlashing",
		" params: ",
		fmt.Sprint(
			arg_count,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.ProcessSlashing(
		transactorOptions,
		arg_count,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"processSlashing",
			arg_count,
		)
	}

	tsLogger.Infof(
		"submitted transaction processSlashing with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.ProcessSlashing(
				newTransactorOptions,
				arg_count,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"processSlashing",
					arg_count,
				)
			}

			tsLogger.Infof(
				"submitted transaction processSlashing with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallProcessSlashing(
	arg_count *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"processSlashing",
		&result,
		arg_count,
	)

	return err
}

func (ts *TokenStaking) ProcessSlashingGasEstimate(
	arg_count *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"processSlashing",
		ts.contractABI,
		ts.transactor,
		arg_count,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) PushNotificationReward(
	arg_reward *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction pushNotificationReward",
		" params: ",
		fmt.Sprint(
			arg_reward,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.PushNotificationReward(
		transactorOptions,
		arg_reward,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"pushNotificationReward",
			arg_reward,
		)
	}

	tsLogger.Infof(
		"submitted transaction pushNotificationReward with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.PushNotificationReward(
				newTransactorOptions,
				arg_reward,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"pushNotificationReward",
					arg_reward,
				)
			}

			tsLogger.Infof(
				"submitted transaction pushNotificationReward with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallPushNotificationReward(
	arg_reward *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"pushNotificationReward",
		&result,
		arg_reward,
	)

	return err
}

func (ts *TokenStaking) PushNotificationRewardGasEstimate(
	arg_reward *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"pushNotificationReward",
		ts.contractABI,
		ts.transactor,
		arg_reward,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) RefreshKeepStakeOwner(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction refreshKeepStakeOwner",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.RefreshKeepStakeOwner(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"refreshKeepStakeOwner",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction refreshKeepStakeOwner with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.RefreshKeepStakeOwner(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"refreshKeepStakeOwner",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction refreshKeepStakeOwner with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallRefreshKeepStakeOwner(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"refreshKeepStakeOwner",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) RefreshKeepStakeOwnerGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"refreshKeepStakeOwner",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) RequestAuthorizationDecrease(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction requestAuthorizationDecrease",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_application,
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.RequestAuthorizationDecrease(
		transactorOptions,
		arg_stakingProvider,
		arg_application,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"requestAuthorizationDecrease",
			arg_stakingProvider,
			arg_application,
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction requestAuthorizationDecrease with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.RequestAuthorizationDecrease(
				newTransactorOptions,
				arg_stakingProvider,
				arg_application,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"requestAuthorizationDecrease",
					arg_stakingProvider,
					arg_application,
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction requestAuthorizationDecrease with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallRequestAuthorizationDecrease(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"requestAuthorizationDecrease",
		&result,
		arg_stakingProvider,
		arg_application,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) RequestAuthorizationDecreaseGasEstimate(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"requestAuthorizationDecrease",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_application,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) RequestAuthorizationDecrease0(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction requestAuthorizationDecrease0",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.RequestAuthorizationDecrease0(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"requestAuthorizationDecrease0",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction requestAuthorizationDecrease0 with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.RequestAuthorizationDecrease0(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"requestAuthorizationDecrease0",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction requestAuthorizationDecrease0 with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallRequestAuthorizationDecrease0(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"requestAuthorizationDecrease0",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) RequestAuthorizationDecrease0GasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"requestAuthorizationDecrease0",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) Seize(
	arg_amount *big.Int,
	arg_rewardMultiplier *big.Int,
	arg_notifier common.Address,
	arg__stakingProviders []common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction seize",
		" params: ",
		fmt.Sprint(
			arg_amount,
			arg_rewardMultiplier,
			arg_notifier,
			arg__stakingProviders,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.Seize(
		transactorOptions,
		arg_amount,
		arg_rewardMultiplier,
		arg_notifier,
		arg__stakingProviders,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"seize",
			arg_amount,
			arg_rewardMultiplier,
			arg_notifier,
			arg__stakingProviders,
		)
	}

	tsLogger.Infof(
		"submitted transaction seize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.Seize(
				newTransactorOptions,
				arg_amount,
				arg_rewardMultiplier,
				arg_notifier,
				arg__stakingProviders,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"seize",
					arg_amount,
					arg_rewardMultiplier,
					arg_notifier,
					arg__stakingProviders,
				)
			}

			tsLogger.Infof(
				"submitted transaction seize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallSeize(
	arg_amount *big.Int,
	arg_rewardMultiplier *big.Int,
	arg_notifier common.Address,
	arg__stakingProviders []common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"seize",
		&result,
		arg_amount,
		arg_rewardMultiplier,
		arg_notifier,
		arg__stakingProviders,
	)

	return err
}

func (ts *TokenStaking) SeizeGasEstimate(
	arg_amount *big.Int,
	arg_rewardMultiplier *big.Int,
	arg_notifier common.Address,
	arg__stakingProviders []common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"seize",
		ts.contractABI,
		ts.transactor,
		arg_amount,
		arg_rewardMultiplier,
		arg_notifier,
		arg__stakingProviders,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) SetAuthorizationCeiling(
	arg_ceiling *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction setAuthorizationCeiling",
		" params: ",
		fmt.Sprint(
			arg_ceiling,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.SetAuthorizationCeiling(
		transactorOptions,
		arg_ceiling,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"setAuthorizationCeiling",
			arg_ceiling,
		)
	}

	tsLogger.Infof(
		"submitted transaction setAuthorizationCeiling with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.SetAuthorizationCeiling(
				newTransactorOptions,
				arg_ceiling,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"setAuthorizationCeiling",
					arg_ceiling,
				)
			}

			tsLogger.Infof(
				"submitted transaction setAuthorizationCeiling with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallSetAuthorizationCeiling(
	arg_ceiling *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"setAuthorizationCeiling",
		&result,
		arg_ceiling,
	)

	return err
}

func (ts *TokenStaking) SetAuthorizationCeilingGasEstimate(
	arg_ceiling *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"setAuthorizationCeiling",
		ts.contractABI,
		ts.transactor,
		arg_ceiling,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) SetMinimumStakeAmount(
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction setMinimumStakeAmount",
		" params: ",
		fmt.Sprint(
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.SetMinimumStakeAmount(
		transactorOptions,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"setMinimumStakeAmount",
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction setMinimumStakeAmount with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.SetMinimumStakeAmount(
				newTransactorOptions,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"setMinimumStakeAmount",
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction setMinimumStakeAmount with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallSetMinimumStakeAmount(
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"setMinimumStakeAmount",
		&result,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) SetMinimumStakeAmountGasEstimate(
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"setMinimumStakeAmount",
		ts.contractABI,
		ts.transactor,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) SetNotificationReward(
	arg_reward *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction setNotificationReward",
		" params: ",
		fmt.Sprint(
			arg_reward,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.SetNotificationReward(
		transactorOptions,
		arg_reward,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"setNotificationReward",
			arg_reward,
		)
	}

	tsLogger.Infof(
		"submitted transaction setNotificationReward with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.SetNotificationReward(
				newTransactorOptions,
				arg_reward,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"setNotificationReward",
					arg_reward,
				)
			}

			tsLogger.Infof(
				"submitted transaction setNotificationReward with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallSetNotificationReward(
	arg_reward *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"setNotificationReward",
		&result,
		arg_reward,
	)

	return err
}

func (ts *TokenStaking) SetNotificationRewardGasEstimate(
	arg_reward *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"setNotificationReward",
		ts.contractABI,
		ts.transactor,
		arg_reward,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) SetPanicButton(
	arg_application common.Address,
	arg_panicButton common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction setPanicButton",
		" params: ",
		fmt.Sprint(
			arg_application,
			arg_panicButton,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.SetPanicButton(
		transactorOptions,
		arg_application,
		arg_panicButton,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"setPanicButton",
			arg_application,
			arg_panicButton,
		)
	}

	tsLogger.Infof(
		"submitted transaction setPanicButton with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.SetPanicButton(
				newTransactorOptions,
				arg_application,
				arg_panicButton,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"setPanicButton",
					arg_application,
					arg_panicButton,
				)
			}

			tsLogger.Infof(
				"submitted transaction setPanicButton with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallSetPanicButton(
	arg_application common.Address,
	arg_panicButton common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"setPanicButton",
		&result,
		arg_application,
		arg_panicButton,
	)

	return err
}

func (ts *TokenStaking) SetPanicButtonGasEstimate(
	arg_application common.Address,
	arg_panicButton common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"setPanicButton",
		ts.contractABI,
		ts.transactor,
		arg_application,
		arg_panicButton,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) SetStakeDiscrepancyPenalty(
	arg_penalty *big.Int,
	arg_rewardMultiplier *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction setStakeDiscrepancyPenalty",
		" params: ",
		fmt.Sprint(
			arg_penalty,
			arg_rewardMultiplier,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.SetStakeDiscrepancyPenalty(
		transactorOptions,
		arg_penalty,
		arg_rewardMultiplier,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"setStakeDiscrepancyPenalty",
			arg_penalty,
			arg_rewardMultiplier,
		)
	}

	tsLogger.Infof(
		"submitted transaction setStakeDiscrepancyPenalty with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.SetStakeDiscrepancyPenalty(
				newTransactorOptions,
				arg_penalty,
				arg_rewardMultiplier,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"setStakeDiscrepancyPenalty",
					arg_penalty,
					arg_rewardMultiplier,
				)
			}

			tsLogger.Infof(
				"submitted transaction setStakeDiscrepancyPenalty with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallSetStakeDiscrepancyPenalty(
	arg_penalty *big.Int,
	arg_rewardMultiplier *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"setStakeDiscrepancyPenalty",
		&result,
		arg_penalty,
		arg_rewardMultiplier,
	)

	return err
}

func (ts *TokenStaking) SetStakeDiscrepancyPenaltyGasEstimate(
	arg_penalty *big.Int,
	arg_rewardMultiplier *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"setStakeDiscrepancyPenalty",
		ts.contractABI,
		ts.transactor,
		arg_penalty,
		arg_rewardMultiplier,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) Slash(
	arg_amount *big.Int,
	arg__stakingProviders []common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction slash",
		" params: ",
		fmt.Sprint(
			arg_amount,
			arg__stakingProviders,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.Slash(
		transactorOptions,
		arg_amount,
		arg__stakingProviders,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"slash",
			arg_amount,
			arg__stakingProviders,
		)
	}

	tsLogger.Infof(
		"submitted transaction slash with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.Slash(
				newTransactorOptions,
				arg_amount,
				arg__stakingProviders,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"slash",
					arg_amount,
					arg__stakingProviders,
				)
			}

			tsLogger.Infof(
				"submitted transaction slash with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallSlash(
	arg_amount *big.Int,
	arg__stakingProviders []common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"slash",
		&result,
		arg_amount,
		arg__stakingProviders,
	)

	return err
}

func (ts *TokenStaking) SlashGasEstimate(
	arg_amount *big.Int,
	arg__stakingProviders []common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"slash",
		ts.contractABI,
		ts.transactor,
		arg_amount,
		arg__stakingProviders,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) Stake(
	arg_stakingProvider common.Address,
	arg_beneficiary common.Address,
	arg_authorizer common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction stake",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.Stake(
		transactorOptions,
		arg_stakingProvider,
		arg_beneficiary,
		arg_authorizer,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"stake",
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction stake with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.Stake(
				newTransactorOptions,
				arg_stakingProvider,
				arg_beneficiary,
				arg_authorizer,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"stake",
					arg_stakingProvider,
					arg_beneficiary,
					arg_authorizer,
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction stake with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallStake(
	arg_stakingProvider common.Address,
	arg_beneficiary common.Address,
	arg_authorizer common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"stake",
		&result,
		arg_stakingProvider,
		arg_beneficiary,
		arg_authorizer,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) StakeGasEstimate(
	arg_stakingProvider common.Address,
	arg_beneficiary common.Address,
	arg_authorizer common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"stake",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_beneficiary,
		arg_authorizer,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) StakeKeep(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction stakeKeep",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.StakeKeep(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"stakeKeep",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction stakeKeep with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.StakeKeep(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"stakeKeep",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction stakeKeep with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallStakeKeep(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"stakeKeep",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) StakeKeepGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"stakeKeep",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) StakeNu(
	arg_stakingProvider common.Address,
	arg_beneficiary common.Address,
	arg_authorizer common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction stakeNu",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.StakeNu(
		transactorOptions,
		arg_stakingProvider,
		arg_beneficiary,
		arg_authorizer,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"stakeNu",
			arg_stakingProvider,
			arg_beneficiary,
			arg_authorizer,
		)
	}

	tsLogger.Infof(
		"submitted transaction stakeNu with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.StakeNu(
				newTransactorOptions,
				arg_stakingProvider,
				arg_beneficiary,
				arg_authorizer,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"stakeNu",
					arg_stakingProvider,
					arg_beneficiary,
					arg_authorizer,
				)
			}

			tsLogger.Infof(
				"submitted transaction stakeNu with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallStakeNu(
	arg_stakingProvider common.Address,
	arg_beneficiary common.Address,
	arg_authorizer common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"stakeNu",
		&result,
		arg_stakingProvider,
		arg_beneficiary,
		arg_authorizer,
	)

	return err
}

func (ts *TokenStaking) StakeNuGasEstimate(
	arg_stakingProvider common.Address,
	arg_beneficiary common.Address,
	arg_authorizer common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"stakeNu",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_beneficiary,
		arg_authorizer,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) TopUp(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction topUp",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.TopUp(
		transactorOptions,
		arg_stakingProvider,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"topUp",
			arg_stakingProvider,
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction topUp with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.TopUp(
				newTransactorOptions,
				arg_stakingProvider,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"topUp",
					arg_stakingProvider,
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction topUp with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallTopUp(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"topUp",
		&result,
		arg_stakingProvider,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) TopUpGasEstimate(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"topUp",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) TopUpKeep(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction topUpKeep",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.TopUpKeep(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"topUpKeep",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction topUpKeep with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.TopUpKeep(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"topUpKeep",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction topUpKeep with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallTopUpKeep(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"topUpKeep",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) TopUpKeepGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"topUpKeep",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) TopUpNu(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction topUpNu",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.TopUpNu(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"topUpNu",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction topUpNu with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.TopUpNu(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"topUpNu",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction topUpNu with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallTopUpNu(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"topUpNu",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) TopUpNuGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"topUpNu",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) TransferGovernance(
	arg_newGuvnor common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction transferGovernance",
		" params: ",
		fmt.Sprint(
			arg_newGuvnor,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.TransferGovernance(
		transactorOptions,
		arg_newGuvnor,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"transferGovernance",
			arg_newGuvnor,
		)
	}

	tsLogger.Infof(
		"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.TransferGovernance(
				newTransactorOptions,
				arg_newGuvnor,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"transferGovernance",
					arg_newGuvnor,
				)
			}

			tsLogger.Infof(
				"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallTransferGovernance(
	arg_newGuvnor common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"transferGovernance",
		&result,
		arg_newGuvnor,
	)

	return err
}

func (ts *TokenStaking) TransferGovernanceGasEstimate(
	arg_newGuvnor common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"transferGovernance",
		ts.contractABI,
		ts.transactor,
		arg_newGuvnor,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) UnstakeAll(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction unstakeAll",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.UnstakeAll(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"unstakeAll",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction unstakeAll with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.UnstakeAll(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"unstakeAll",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction unstakeAll with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallUnstakeAll(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"unstakeAll",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) UnstakeAllGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"unstakeAll",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) UnstakeKeep(
	arg_stakingProvider common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction unstakeKeep",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.UnstakeKeep(
		transactorOptions,
		arg_stakingProvider,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"unstakeKeep",
			arg_stakingProvider,
		)
	}

	tsLogger.Infof(
		"submitted transaction unstakeKeep with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.UnstakeKeep(
				newTransactorOptions,
				arg_stakingProvider,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"unstakeKeep",
					arg_stakingProvider,
				)
			}

			tsLogger.Infof(
				"submitted transaction unstakeKeep with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallUnstakeKeep(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"unstakeKeep",
		&result,
		arg_stakingProvider,
	)

	return err
}

func (ts *TokenStaking) UnstakeKeepGasEstimate(
	arg_stakingProvider common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"unstakeKeep",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) UnstakeNu(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction unstakeNu",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.UnstakeNu(
		transactorOptions,
		arg_stakingProvider,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"unstakeNu",
			arg_stakingProvider,
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction unstakeNu with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.UnstakeNu(
				newTransactorOptions,
				arg_stakingProvider,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"unstakeNu",
					arg_stakingProvider,
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction unstakeNu with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallUnstakeNu(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"unstakeNu",
		&result,
		arg_stakingProvider,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) UnstakeNuGasEstimate(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"unstakeNu",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) UnstakeT(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction unstakeT",
		" params: ",
		fmt.Sprint(
			arg_stakingProvider,
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.UnstakeT(
		transactorOptions,
		arg_stakingProvider,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"unstakeT",
			arg_stakingProvider,
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction unstakeT with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.UnstakeT(
				newTransactorOptions,
				arg_stakingProvider,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"unstakeT",
					arg_stakingProvider,
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction unstakeT with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallUnstakeT(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"unstakeT",
		&result,
		arg_stakingProvider,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) UnstakeTGasEstimate(
	arg_stakingProvider common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"unstakeT",
		ts.contractABI,
		ts.transactor,
		arg_stakingProvider,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (ts *TokenStaking) WithdrawNotificationReward(
	arg_recipient common.Address,
	arg_amount *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	tsLogger.Debug(
		"submitting transaction withdrawNotificationReward",
		" params: ",
		fmt.Sprint(
			arg_recipient,
			arg_amount,
		),
	)

	ts.transactionMutex.Lock()
	defer ts.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *ts.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := ts.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := ts.contract.WithdrawNotificationReward(
		transactorOptions,
		arg_recipient,
		arg_amount,
	)
	if err != nil {
		return transaction, ts.errorResolver.ResolveError(
			err,
			ts.transactorOptions.From,
			nil,
			"withdrawNotificationReward",
			arg_recipient,
			arg_amount,
		)
	}

	tsLogger.Infof(
		"submitted transaction withdrawNotificationReward with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go ts.miningWaiter.ForceMining(
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

			transaction, err := ts.contract.WithdrawNotificationReward(
				newTransactorOptions,
				arg_recipient,
				arg_amount,
			)
			if err != nil {
				return nil, ts.errorResolver.ResolveError(
					err,
					ts.transactorOptions.From,
					nil,
					"withdrawNotificationReward",
					arg_recipient,
					arg_amount,
				)
			}

			tsLogger.Infof(
				"submitted transaction withdrawNotificationReward with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	ts.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (ts *TokenStaking) CallWithdrawNotificationReward(
	arg_recipient common.Address,
	arg_amount *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		ts.transactorOptions.From,
		blockNumber, nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"withdrawNotificationReward",
		&result,
		arg_recipient,
		arg_amount,
	)

	return err
}

func (ts *TokenStaking) WithdrawNotificationRewardGasEstimate(
	arg_recipient common.Address,
	arg_amount *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		ts.callerOptions.From,
		ts.contractAddress,
		"withdrawNotificationReward",
		ts.contractABI,
		ts.transactor,
		arg_recipient,
		arg_amount,
	)

	return result, err
}

// ----- Const Methods ------

type applicationInfo struct {
	Status      uint8
	PanicButton common.Address
}

func (ts *TokenStaking) ApplicationInfo(
	arg0 common.Address,
) (applicationInfo, error) {
	result, err := ts.contract.ApplicationInfo(
		ts.callerOptions,
		arg0,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"applicationInfo",
			arg0,
		)
	}

	return result, err
}

func (ts *TokenStaking) ApplicationInfoAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (applicationInfo, error) {
	var result applicationInfo

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"applicationInfo",
		&result,
		arg0,
	)

	return result, err
}

func (ts *TokenStaking) Applications(
	arg0 *big.Int,
) (common.Address, error) {
	result, err := ts.contract.Applications(
		ts.callerOptions,
		arg0,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"applications",
			arg0,
		)
	}

	return result, err
}

func (ts *TokenStaking) ApplicationsAtBlock(
	arg0 *big.Int,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"applications",
		&result,
		arg0,
	)

	return result, err
}

func (ts *TokenStaking) AuthorizationCeiling() (*big.Int, error) {
	result, err := ts.contract.AuthorizationCeiling(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"authorizationCeiling",
		)
	}

	return result, err
}

func (ts *TokenStaking) AuthorizationCeilingAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"authorizationCeiling",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) AuthorizedStake(
	arg_stakingProvider common.Address,
	arg_application common.Address,
) (*big.Int, error) {
	result, err := ts.contract.AuthorizedStake(
		ts.callerOptions,
		arg_stakingProvider,
		arg_application,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"authorizedStake",
			arg_stakingProvider,
			arg_application,
		)
	}

	return result, err
}

func (ts *TokenStaking) AuthorizedStakeAtBlock(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"authorizedStake",
		&result,
		arg_stakingProvider,
		arg_application,
	)

	return result, err
}

func (ts *TokenStaking) Checkpoints(
	arg_account common.Address,
	arg_pos uint32,
) (abi.CheckpointsCheckpoint, error) {
	result, err := ts.contract.Checkpoints(
		ts.callerOptions,
		arg_account,
		arg_pos,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"checkpoints",
			arg_account,
			arg_pos,
		)
	}

	return result, err
}

func (ts *TokenStaking) CheckpointsAtBlock(
	arg_account common.Address,
	arg_pos uint32,
	blockNumber *big.Int,
) (abi.CheckpointsCheckpoint, error) {
	var result abi.CheckpointsCheckpoint

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"checkpoints",
		&result,
		arg_account,
		arg_pos,
	)

	return result, err
}

func (ts *TokenStaking) Delegates(
	arg_account common.Address,
) (common.Address, error) {
	result, err := ts.contract.Delegates(
		ts.callerOptions,
		arg_account,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"delegates",
			arg_account,
		)
	}

	return result, err
}

func (ts *TokenStaking) DelegatesAtBlock(
	arg_account common.Address,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"delegates",
		&result,
		arg_account,
	)

	return result, err
}

func (ts *TokenStaking) GetApplicationsLength() (*big.Int, error) {
	result, err := ts.contract.GetApplicationsLength(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getApplicationsLength",
		)
	}

	return result, err
}

func (ts *TokenStaking) GetApplicationsLengthAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getApplicationsLength",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) GetAvailableToAuthorize(
	arg_stakingProvider common.Address,
	arg_application common.Address,
) (*big.Int, error) {
	result, err := ts.contract.GetAvailableToAuthorize(
		ts.callerOptions,
		arg_stakingProvider,
		arg_application,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getAvailableToAuthorize",
			arg_stakingProvider,
			arg_application,
		)
	}

	return result, err
}

func (ts *TokenStaking) GetAvailableToAuthorizeAtBlock(
	arg_stakingProvider common.Address,
	arg_application common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getAvailableToAuthorize",
		&result,
		arg_stakingProvider,
		arg_application,
	)

	return result, err
}

func (ts *TokenStaking) GetMinStaked(
	arg_stakingProvider common.Address,
	arg_stakeTypes uint8,
) (*big.Int, error) {
	result, err := ts.contract.GetMinStaked(
		ts.callerOptions,
		arg_stakingProvider,
		arg_stakeTypes,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getMinStaked",
			arg_stakingProvider,
			arg_stakeTypes,
		)
	}

	return result, err
}

func (ts *TokenStaking) GetMinStakedAtBlock(
	arg_stakingProvider common.Address,
	arg_stakeTypes uint8,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getMinStaked",
		&result,
		arg_stakingProvider,
		arg_stakeTypes,
	)

	return result, err
}

func (ts *TokenStaking) GetPastTotalSupply(
	arg_blockNumber *big.Int,
) (*big.Int, error) {
	result, err := ts.contract.GetPastTotalSupply(
		ts.callerOptions,
		arg_blockNumber,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getPastTotalSupply",
			arg_blockNumber,
		)
	}

	return result, err
}

func (ts *TokenStaking) GetPastTotalSupplyAtBlock(
	arg_blockNumber *big.Int,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getPastTotalSupply",
		&result,
		arg_blockNumber,
	)

	return result, err
}

func (ts *TokenStaking) GetPastVotes(
	arg_account common.Address,
	arg_blockNumber *big.Int,
) (*big.Int, error) {
	result, err := ts.contract.GetPastVotes(
		ts.callerOptions,
		arg_account,
		arg_blockNumber,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getPastVotes",
			arg_account,
			arg_blockNumber,
		)
	}

	return result, err
}

func (ts *TokenStaking) GetPastVotesAtBlock(
	arg_account common.Address,
	arg_blockNumber *big.Int,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getPastVotes",
		&result,
		arg_account,
		arg_blockNumber,
	)

	return result, err
}

func (ts *TokenStaking) GetSlashingQueueLength() (*big.Int, error) {
	result, err := ts.contract.GetSlashingQueueLength(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getSlashingQueueLength",
		)
	}

	return result, err
}

func (ts *TokenStaking) GetSlashingQueueLengthAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getSlashingQueueLength",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) GetStartStakingTimestamp(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := ts.contract.GetStartStakingTimestamp(
		ts.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getStartStakingTimestamp",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (ts *TokenStaking) GetStartStakingTimestampAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getStartStakingTimestamp",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

func (ts *TokenStaking) GetVotes(
	arg_account common.Address,
) (*big.Int, error) {
	result, err := ts.contract.GetVotes(
		ts.callerOptions,
		arg_account,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"getVotes",
			arg_account,
		)
	}

	return result, err
}

func (ts *TokenStaking) GetVotesAtBlock(
	arg_account common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"getVotes",
		&result,
		arg_account,
	)

	return result, err
}

func (ts *TokenStaking) Governance() (common.Address, error) {
	result, err := ts.contract.Governance(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"governance",
		)
	}

	return result, err
}

func (ts *TokenStaking) GovernanceAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"governance",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) MinTStakeAmount() (*big.Int, error) {
	result, err := ts.contract.MinTStakeAmount(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"minTStakeAmount",
		)
	}

	return result, err
}

func (ts *TokenStaking) MinTStakeAmountAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"minTStakeAmount",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) NotificationReward() (*big.Int, error) {
	result, err := ts.contract.NotificationReward(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"notificationReward",
		)
	}

	return result, err
}

func (ts *TokenStaking) NotificationRewardAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"notificationReward",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) NotifiersTreasury() (*big.Int, error) {
	result, err := ts.contract.NotifiersTreasury(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"notifiersTreasury",
		)
	}

	return result, err
}

func (ts *TokenStaking) NotifiersTreasuryAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"notifiersTreasury",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) NumCheckpoints(
	arg_account common.Address,
) (uint32, error) {
	result, err := ts.contract.NumCheckpoints(
		ts.callerOptions,
		arg_account,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"numCheckpoints",
			arg_account,
		)
	}

	return result, err
}

func (ts *TokenStaking) NumCheckpointsAtBlock(
	arg_account common.Address,
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"numCheckpoints",
		&result,
		arg_account,
	)

	return result, err
}

type rolesOf struct {
	Owner       common.Address
	Beneficiary common.Address
	Authorizer  common.Address
}

func (ts *TokenStaking) RolesOf(
	arg_stakingProvider common.Address,
) (rolesOf, error) {
	result, err := ts.contract.RolesOf(
		ts.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"rolesOf",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (ts *TokenStaking) RolesOfAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (rolesOf, error) {
	var result rolesOf

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"rolesOf",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

type slashingQueue struct {
	StakingProvider common.Address
	Amount          *big.Int
}

func (ts *TokenStaking) SlashingQueue(
	arg0 *big.Int,
) (slashingQueue, error) {
	result, err := ts.contract.SlashingQueue(
		ts.callerOptions,
		arg0,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"slashingQueue",
			arg0,
		)
	}

	return result, err
}

func (ts *TokenStaking) SlashingQueueAtBlock(
	arg0 *big.Int,
	blockNumber *big.Int,
) (slashingQueue, error) {
	var result slashingQueue

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"slashingQueue",
		&result,
		arg0,
	)

	return result, err
}

func (ts *TokenStaking) SlashingQueueIndex() (*big.Int, error) {
	result, err := ts.contract.SlashingQueueIndex(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"slashingQueueIndex",
		)
	}

	return result, err
}

func (ts *TokenStaking) SlashingQueueIndexAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"slashingQueueIndex",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) StakeDiscrepancyPenalty() (*big.Int, error) {
	result, err := ts.contract.StakeDiscrepancyPenalty(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"stakeDiscrepancyPenalty",
		)
	}

	return result, err
}

func (ts *TokenStaking) StakeDiscrepancyPenaltyAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"stakeDiscrepancyPenalty",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) StakeDiscrepancyRewardMultiplier() (*big.Int, error) {
	result, err := ts.contract.StakeDiscrepancyRewardMultiplier(
		ts.callerOptions,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"stakeDiscrepancyRewardMultiplier",
		)
	}

	return result, err
}

func (ts *TokenStaking) StakeDiscrepancyRewardMultiplierAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"stakeDiscrepancyRewardMultiplier",
		&result,
	)

	return result, err
}

func (ts *TokenStaking) StakedNu(
	arg_stakingProvider common.Address,
) (*big.Int, error) {
	result, err := ts.contract.StakedNu(
		ts.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"stakedNu",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (ts *TokenStaking) StakedNuAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"stakedNu",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

type stakes struct {
	TStake       *big.Int
	KeepInTStake *big.Int
	NuInTStake   *big.Int
}

func (ts *TokenStaking) Stakes(
	arg_stakingProvider common.Address,
) (stakes, error) {
	result, err := ts.contract.Stakes(
		ts.callerOptions,
		arg_stakingProvider,
	)

	if err != nil {
		return result, ts.errorResolver.ResolveError(
			err,
			ts.callerOptions.From,
			nil,
			"stakes",
			arg_stakingProvider,
		)
	}

	return result, err
}

func (ts *TokenStaking) StakesAtBlock(
	arg_stakingProvider common.Address,
	blockNumber *big.Int,
) (stakes, error) {
	var result stakes

	err := chainutil.CallAtBlock(
		ts.callerOptions.From,
		blockNumber,
		nil,
		ts.contractABI,
		ts.caller,
		ts.errorResolver,
		ts.contractAddress,
		"stakes",
		&result,
		arg_stakingProvider,
	)

	return result, err
}

// ------ Events -------

func (ts *TokenStaking) ApplicationStatusChangedEvent(
	opts *ethereum.SubscribeOpts,
	applicationFilter []common.Address,
	newStatusFilter []uint8,
) *TsApplicationStatusChangedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsApplicationStatusChangedSubscription{
		ts,
		opts,
		applicationFilter,
		newStatusFilter,
	}
}

type TsApplicationStatusChangedSubscription struct {
	contract          *TokenStaking
	opts              *ethereum.SubscribeOpts
	applicationFilter []common.Address
	newStatusFilter   []uint8
}

type tokenStakingApplicationStatusChangedFunc func(
	Application common.Address,
	NewStatus uint8,
	blockNumber uint64,
)

func (ascs *TsApplicationStatusChangedSubscription) OnEvent(
	handler tokenStakingApplicationStatusChangedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingApplicationStatusChanged)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Application,
					event.NewStatus,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ascs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ascs *TsApplicationStatusChangedSubscription) Pipe(
	sink chan *abi.TokenStakingApplicationStatusChanged,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ascs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ascs.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ascs.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past ApplicationStatusChanged events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ascs.contract.PastApplicationStatusChangedEvents(
					fromBlock,
					nil,
					ascs.applicationFilter,
					ascs.newStatusFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past ApplicationStatusChanged events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ascs.contract.watchApplicationStatusChanged(
		sink,
		ascs.applicationFilter,
		ascs.newStatusFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchApplicationStatusChanged(
	sink chan *abi.TokenStakingApplicationStatusChanged,
	applicationFilter []common.Address,
	newStatusFilter []uint8,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchApplicationStatusChanged(
			&bind.WatchOpts{Context: ctx},
			sink,
			applicationFilter,
			newStatusFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event ApplicationStatusChanged had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event ApplicationStatusChanged failed "+
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

func (ts *TokenStaking) PastApplicationStatusChangedEvents(
	startBlock uint64,
	endBlock *uint64,
	applicationFilter []common.Address,
	newStatusFilter []uint8,
) ([]*abi.TokenStakingApplicationStatusChanged, error) {
	iterator, err := ts.contract.FilterApplicationStatusChanged(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		applicationFilter,
		newStatusFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ApplicationStatusChanged events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingApplicationStatusChanged, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) AuthorizationCeilingSetEvent(
	opts *ethereum.SubscribeOpts,
) *TsAuthorizationCeilingSetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsAuthorizationCeilingSetSubscription{
		ts,
		opts,
	}
}

type TsAuthorizationCeilingSetSubscription struct {
	contract *TokenStaking
	opts     *ethereum.SubscribeOpts
}

type tokenStakingAuthorizationCeilingSetFunc func(
	Ceiling *big.Int,
	blockNumber uint64,
)

func (acss *TsAuthorizationCeilingSetSubscription) OnEvent(
	handler tokenStakingAuthorizationCeilingSetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingAuthorizationCeilingSet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Ceiling,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := acss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (acss *TsAuthorizationCeilingSetSubscription) Pipe(
	sink chan *abi.TokenStakingAuthorizationCeilingSet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(acss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := acss.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - acss.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past AuthorizationCeilingSet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := acss.contract.PastAuthorizationCeilingSetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past AuthorizationCeilingSet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := acss.contract.watchAuthorizationCeilingSet(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchAuthorizationCeilingSet(
	sink chan *abi.TokenStakingAuthorizationCeilingSet,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchAuthorizationCeilingSet(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event AuthorizationCeilingSet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event AuthorizationCeilingSet failed "+
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

func (ts *TokenStaking) PastAuthorizationCeilingSetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.TokenStakingAuthorizationCeilingSet, error) {
	iterator, err := ts.contract.FilterAuthorizationCeilingSet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationCeilingSet events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingAuthorizationCeilingSet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) AuthorizationDecreaseApprovedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) *TsAuthorizationDecreaseApprovedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsAuthorizationDecreaseApprovedSubscription{
		ts,
		opts,
		stakingProviderFilter,
		applicationFilter,
	}
}

type TsAuthorizationDecreaseApprovedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	applicationFilter     []common.Address
}

type tokenStakingAuthorizationDecreaseApprovedFunc func(
	StakingProvider common.Address,
	Application common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	blockNumber uint64,
)

func (adas *TsAuthorizationDecreaseApprovedSubscription) OnEvent(
	handler tokenStakingAuthorizationDecreaseApprovedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingAuthorizationDecreaseApproved)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Application,
					event.FromAmount,
					event.ToAmount,
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

func (adas *TsAuthorizationDecreaseApprovedSubscription) Pipe(
	sink chan *abi.TokenStakingAuthorizationDecreaseApproved,
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
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - adas.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past AuthorizationDecreaseApproved events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := adas.contract.PastAuthorizationDecreaseApprovedEvents(
					fromBlock,
					nil,
					adas.stakingProviderFilter,
					adas.applicationFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
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
		adas.applicationFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchAuthorizationDecreaseApproved(
	sink chan *abi.TokenStakingAuthorizationDecreaseApproved,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchAuthorizationDecreaseApproved(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			applicationFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event AuthorizationDecreaseApproved had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
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

func (ts *TokenStaking) PastAuthorizationDecreaseApprovedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) ([]*abi.TokenStakingAuthorizationDecreaseApproved, error) {
	iterator, err := ts.contract.FilterAuthorizationDecreaseApproved(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		applicationFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationDecreaseApproved events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingAuthorizationDecreaseApproved, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) AuthorizationDecreaseRequestedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) *TsAuthorizationDecreaseRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsAuthorizationDecreaseRequestedSubscription{
		ts,
		opts,
		stakingProviderFilter,
		applicationFilter,
	}
}

type TsAuthorizationDecreaseRequestedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	applicationFilter     []common.Address
}

type tokenStakingAuthorizationDecreaseRequestedFunc func(
	StakingProvider common.Address,
	Application common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	blockNumber uint64,
)

func (adrs *TsAuthorizationDecreaseRequestedSubscription) OnEvent(
	handler tokenStakingAuthorizationDecreaseRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingAuthorizationDecreaseRequested)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Application,
					event.FromAmount,
					event.ToAmount,
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

func (adrs *TsAuthorizationDecreaseRequestedSubscription) Pipe(
	sink chan *abi.TokenStakingAuthorizationDecreaseRequested,
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
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - adrs.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past AuthorizationDecreaseRequested events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := adrs.contract.PastAuthorizationDecreaseRequestedEvents(
					fromBlock,
					nil,
					adrs.stakingProviderFilter,
					adrs.applicationFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
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
		adrs.applicationFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchAuthorizationDecreaseRequested(
	sink chan *abi.TokenStakingAuthorizationDecreaseRequested,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchAuthorizationDecreaseRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			applicationFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event AuthorizationDecreaseRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
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

func (ts *TokenStaking) PastAuthorizationDecreaseRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) ([]*abi.TokenStakingAuthorizationDecreaseRequested, error) {
	iterator, err := ts.contract.FilterAuthorizationDecreaseRequested(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		applicationFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationDecreaseRequested events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingAuthorizationDecreaseRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) AuthorizationIncreasedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) *TsAuthorizationIncreasedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsAuthorizationIncreasedSubscription{
		ts,
		opts,
		stakingProviderFilter,
		applicationFilter,
	}
}

type TsAuthorizationIncreasedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	applicationFilter     []common.Address
}

type tokenStakingAuthorizationIncreasedFunc func(
	StakingProvider common.Address,
	Application common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	blockNumber uint64,
)

func (ais *TsAuthorizationIncreasedSubscription) OnEvent(
	handler tokenStakingAuthorizationIncreasedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingAuthorizationIncreased)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Application,
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

func (ais *TsAuthorizationIncreasedSubscription) Pipe(
	sink chan *abi.TokenStakingAuthorizationIncreased,
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
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ais.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past AuthorizationIncreased events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ais.contract.PastAuthorizationIncreasedEvents(
					fromBlock,
					nil,
					ais.stakingProviderFilter,
					ais.applicationFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
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
		ais.applicationFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchAuthorizationIncreased(
	sink chan *abi.TokenStakingAuthorizationIncreased,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchAuthorizationIncreased(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			applicationFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event AuthorizationIncreased had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
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

func (ts *TokenStaking) PastAuthorizationIncreasedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
) ([]*abi.TokenStakingAuthorizationIncreased, error) {
	iterator, err := ts.contract.FilterAuthorizationIncreased(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		applicationFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationIncreased events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingAuthorizationIncreased, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) AuthorizationInvoluntaryDecreasedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
	successfulCallFilter []bool,
) *TsAuthorizationInvoluntaryDecreasedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsAuthorizationInvoluntaryDecreasedSubscription{
		ts,
		opts,
		stakingProviderFilter,
		applicationFilter,
		successfulCallFilter,
	}
}

type TsAuthorizationInvoluntaryDecreasedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	applicationFilter     []common.Address
	successfulCallFilter  []bool
}

type tokenStakingAuthorizationInvoluntaryDecreasedFunc func(
	StakingProvider common.Address,
	Application common.Address,
	FromAmount *big.Int,
	ToAmount *big.Int,
	SuccessfulCall bool,
	blockNumber uint64,
)

func (aids *TsAuthorizationInvoluntaryDecreasedSubscription) OnEvent(
	handler tokenStakingAuthorizationInvoluntaryDecreasedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingAuthorizationInvoluntaryDecreased)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.Application,
					event.FromAmount,
					event.ToAmount,
					event.SuccessfulCall,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := aids.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (aids *TsAuthorizationInvoluntaryDecreasedSubscription) Pipe(
	sink chan *abi.TokenStakingAuthorizationInvoluntaryDecreased,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(aids.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := aids.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - aids.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past AuthorizationInvoluntaryDecreased events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := aids.contract.PastAuthorizationInvoluntaryDecreasedEvents(
					fromBlock,
					nil,
					aids.stakingProviderFilter,
					aids.applicationFilter,
					aids.successfulCallFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past AuthorizationInvoluntaryDecreased events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := aids.contract.watchAuthorizationInvoluntaryDecreased(
		sink,
		aids.stakingProviderFilter,
		aids.applicationFilter,
		aids.successfulCallFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchAuthorizationInvoluntaryDecreased(
	sink chan *abi.TokenStakingAuthorizationInvoluntaryDecreased,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
	successfulCallFilter []bool,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchAuthorizationInvoluntaryDecreased(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			applicationFilter,
			successfulCallFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event AuthorizationInvoluntaryDecreased had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event AuthorizationInvoluntaryDecreased failed "+
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

func (ts *TokenStaking) PastAuthorizationInvoluntaryDecreasedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	applicationFilter []common.Address,
	successfulCallFilter []bool,
) ([]*abi.TokenStakingAuthorizationInvoluntaryDecreased, error) {
	iterator, err := ts.contract.FilterAuthorizationInvoluntaryDecreased(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		applicationFilter,
		successfulCallFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationInvoluntaryDecreased events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingAuthorizationInvoluntaryDecreased, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) DelegateChangedEvent(
	opts *ethereum.SubscribeOpts,
	delegatorFilter []common.Address,
	fromDelegateFilter []common.Address,
	toDelegateFilter []common.Address,
) *TsDelegateChangedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsDelegateChangedSubscription{
		ts,
		opts,
		delegatorFilter,
		fromDelegateFilter,
		toDelegateFilter,
	}
}

type TsDelegateChangedSubscription struct {
	contract           *TokenStaking
	opts               *ethereum.SubscribeOpts
	delegatorFilter    []common.Address
	fromDelegateFilter []common.Address
	toDelegateFilter   []common.Address
}

type tokenStakingDelegateChangedFunc func(
	Delegator common.Address,
	FromDelegate common.Address,
	ToDelegate common.Address,
	blockNumber uint64,
)

func (dcs *TsDelegateChangedSubscription) OnEvent(
	handler tokenStakingDelegateChangedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingDelegateChanged)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Delegator,
					event.FromDelegate,
					event.ToDelegate,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dcs *TsDelegateChangedSubscription) Pipe(
	sink chan *abi.TokenStakingDelegateChanged,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dcs.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past DelegateChanged events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dcs.contract.PastDelegateChangedEvents(
					fromBlock,
					nil,
					dcs.delegatorFilter,
					dcs.fromDelegateFilter,
					dcs.toDelegateFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past DelegateChanged events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dcs.contract.watchDelegateChanged(
		sink,
		dcs.delegatorFilter,
		dcs.fromDelegateFilter,
		dcs.toDelegateFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchDelegateChanged(
	sink chan *abi.TokenStakingDelegateChanged,
	delegatorFilter []common.Address,
	fromDelegateFilter []common.Address,
	toDelegateFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchDelegateChanged(
			&bind.WatchOpts{Context: ctx},
			sink,
			delegatorFilter,
			fromDelegateFilter,
			toDelegateFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event DelegateChanged had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event DelegateChanged failed "+
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

func (ts *TokenStaking) PastDelegateChangedEvents(
	startBlock uint64,
	endBlock *uint64,
	delegatorFilter []common.Address,
	fromDelegateFilter []common.Address,
	toDelegateFilter []common.Address,
) ([]*abi.TokenStakingDelegateChanged, error) {
	iterator, err := ts.contract.FilterDelegateChanged(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		delegatorFilter,
		fromDelegateFilter,
		toDelegateFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DelegateChanged events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingDelegateChanged, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) DelegateVotesChangedEvent(
	opts *ethereum.SubscribeOpts,
	delegateFilter []common.Address,
) *TsDelegateVotesChangedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsDelegateVotesChangedSubscription{
		ts,
		opts,
		delegateFilter,
	}
}

type TsDelegateVotesChangedSubscription struct {
	contract       *TokenStaking
	opts           *ethereum.SubscribeOpts
	delegateFilter []common.Address
}

type tokenStakingDelegateVotesChangedFunc func(
	Delegate common.Address,
	PreviousBalance *big.Int,
	NewBalance *big.Int,
	blockNumber uint64,
)

func (dvcs *TsDelegateVotesChangedSubscription) OnEvent(
	handler tokenStakingDelegateVotesChangedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingDelegateVotesChanged)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Delegate,
					event.PreviousBalance,
					event.NewBalance,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dvcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dvcs *TsDelegateVotesChangedSubscription) Pipe(
	sink chan *abi.TokenStakingDelegateVotesChanged,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dvcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dvcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dvcs.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past DelegateVotesChanged events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dvcs.contract.PastDelegateVotesChangedEvents(
					fromBlock,
					nil,
					dvcs.delegateFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past DelegateVotesChanged events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dvcs.contract.watchDelegateVotesChanged(
		sink,
		dvcs.delegateFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchDelegateVotesChanged(
	sink chan *abi.TokenStakingDelegateVotesChanged,
	delegateFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchDelegateVotesChanged(
			&bind.WatchOpts{Context: ctx},
			sink,
			delegateFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event DelegateVotesChanged had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event DelegateVotesChanged failed "+
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

func (ts *TokenStaking) PastDelegateVotesChangedEvents(
	startBlock uint64,
	endBlock *uint64,
	delegateFilter []common.Address,
) ([]*abi.TokenStakingDelegateVotesChanged, error) {
	iterator, err := ts.contract.FilterDelegateVotesChanged(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		delegateFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DelegateVotesChanged events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingDelegateVotesChanged, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) GovernanceTransferredEvent(
	opts *ethereum.SubscribeOpts,
) *TsGovernanceTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsGovernanceTransferredSubscription{
		ts,
		opts,
	}
}

type TsGovernanceTransferredSubscription struct {
	contract *TokenStaking
	opts     *ethereum.SubscribeOpts
}

type tokenStakingGovernanceTransferredFunc func(
	OldGovernance common.Address,
	NewGovernance common.Address,
	blockNumber uint64,
)

func (gts *TsGovernanceTransferredSubscription) OnEvent(
	handler tokenStakingGovernanceTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingGovernanceTransferred)
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

func (gts *TsGovernanceTransferredSubscription) Pipe(
	sink chan *abi.TokenStakingGovernanceTransferred,
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
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gts.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past GovernanceTransferred events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gts.contract.PastGovernanceTransferredEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
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

func (ts *TokenStaking) watchGovernanceTransferred(
	sink chan *abi.TokenStakingGovernanceTransferred,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchGovernanceTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event GovernanceTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
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

func (ts *TokenStaking) PastGovernanceTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.TokenStakingGovernanceTransferred, error) {
	iterator, err := ts.contract.FilterGovernanceTransferred(
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

	events := make([]*abi.TokenStakingGovernanceTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) MinimumStakeAmountSetEvent(
	opts *ethereum.SubscribeOpts,
) *TsMinimumStakeAmountSetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsMinimumStakeAmountSetSubscription{
		ts,
		opts,
	}
}

type TsMinimumStakeAmountSetSubscription struct {
	contract *TokenStaking
	opts     *ethereum.SubscribeOpts
}

type tokenStakingMinimumStakeAmountSetFunc func(
	Amount *big.Int,
	blockNumber uint64,
)

func (msass *TsMinimumStakeAmountSetSubscription) OnEvent(
	handler tokenStakingMinimumStakeAmountSetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingMinimumStakeAmountSet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Amount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := msass.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (msass *TsMinimumStakeAmountSetSubscription) Pipe(
	sink chan *abi.TokenStakingMinimumStakeAmountSet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(msass.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := msass.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - msass.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past MinimumStakeAmountSet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := msass.contract.PastMinimumStakeAmountSetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past MinimumStakeAmountSet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := msass.contract.watchMinimumStakeAmountSet(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchMinimumStakeAmountSet(
	sink chan *abi.TokenStakingMinimumStakeAmountSet,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchMinimumStakeAmountSet(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event MinimumStakeAmountSet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event MinimumStakeAmountSet failed "+
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

func (ts *TokenStaking) PastMinimumStakeAmountSetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.TokenStakingMinimumStakeAmountSet, error) {
	iterator, err := ts.contract.FilterMinimumStakeAmountSet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MinimumStakeAmountSet events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingMinimumStakeAmountSet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) NotificationRewardPushedEvent(
	opts *ethereum.SubscribeOpts,
) *TsNotificationRewardPushedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsNotificationRewardPushedSubscription{
		ts,
		opts,
	}
}

type TsNotificationRewardPushedSubscription struct {
	contract *TokenStaking
	opts     *ethereum.SubscribeOpts
}

type tokenStakingNotificationRewardPushedFunc func(
	Reward *big.Int,
	blockNumber uint64,
)

func (nrps *TsNotificationRewardPushedSubscription) OnEvent(
	handler tokenStakingNotificationRewardPushedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingNotificationRewardPushed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Reward,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := nrps.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (nrps *TsNotificationRewardPushedSubscription) Pipe(
	sink chan *abi.TokenStakingNotificationRewardPushed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(nrps.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := nrps.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - nrps.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past NotificationRewardPushed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := nrps.contract.PastNotificationRewardPushedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past NotificationRewardPushed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := nrps.contract.watchNotificationRewardPushed(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchNotificationRewardPushed(
	sink chan *abi.TokenStakingNotificationRewardPushed,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchNotificationRewardPushed(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event NotificationRewardPushed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event NotificationRewardPushed failed "+
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

func (ts *TokenStaking) PastNotificationRewardPushedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.TokenStakingNotificationRewardPushed, error) {
	iterator, err := ts.contract.FilterNotificationRewardPushed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past NotificationRewardPushed events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingNotificationRewardPushed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) NotificationRewardSetEvent(
	opts *ethereum.SubscribeOpts,
) *TsNotificationRewardSetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsNotificationRewardSetSubscription{
		ts,
		opts,
	}
}

type TsNotificationRewardSetSubscription struct {
	contract *TokenStaking
	opts     *ethereum.SubscribeOpts
}

type tokenStakingNotificationRewardSetFunc func(
	Reward *big.Int,
	blockNumber uint64,
)

func (nrss *TsNotificationRewardSetSubscription) OnEvent(
	handler tokenStakingNotificationRewardSetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingNotificationRewardSet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Reward,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := nrss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (nrss *TsNotificationRewardSetSubscription) Pipe(
	sink chan *abi.TokenStakingNotificationRewardSet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(nrss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := nrss.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - nrss.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past NotificationRewardSet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := nrss.contract.PastNotificationRewardSetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past NotificationRewardSet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := nrss.contract.watchNotificationRewardSet(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchNotificationRewardSet(
	sink chan *abi.TokenStakingNotificationRewardSet,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchNotificationRewardSet(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event NotificationRewardSet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event NotificationRewardSet failed "+
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

func (ts *TokenStaking) PastNotificationRewardSetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.TokenStakingNotificationRewardSet, error) {
	iterator, err := ts.contract.FilterNotificationRewardSet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past NotificationRewardSet events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingNotificationRewardSet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) NotificationRewardWithdrawnEvent(
	opts *ethereum.SubscribeOpts,
) *TsNotificationRewardWithdrawnSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsNotificationRewardWithdrawnSubscription{
		ts,
		opts,
	}
}

type TsNotificationRewardWithdrawnSubscription struct {
	contract *TokenStaking
	opts     *ethereum.SubscribeOpts
}

type tokenStakingNotificationRewardWithdrawnFunc func(
	Recipient common.Address,
	Amount *big.Int,
	blockNumber uint64,
)

func (nrws *TsNotificationRewardWithdrawnSubscription) OnEvent(
	handler tokenStakingNotificationRewardWithdrawnFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingNotificationRewardWithdrawn)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Recipient,
					event.Amount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := nrws.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (nrws *TsNotificationRewardWithdrawnSubscription) Pipe(
	sink chan *abi.TokenStakingNotificationRewardWithdrawn,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(nrws.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := nrws.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - nrws.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past NotificationRewardWithdrawn events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := nrws.contract.PastNotificationRewardWithdrawnEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past NotificationRewardWithdrawn events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := nrws.contract.watchNotificationRewardWithdrawn(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchNotificationRewardWithdrawn(
	sink chan *abi.TokenStakingNotificationRewardWithdrawn,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchNotificationRewardWithdrawn(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event NotificationRewardWithdrawn had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event NotificationRewardWithdrawn failed "+
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

func (ts *TokenStaking) PastNotificationRewardWithdrawnEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.TokenStakingNotificationRewardWithdrawn, error) {
	iterator, err := ts.contract.FilterNotificationRewardWithdrawn(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past NotificationRewardWithdrawn events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingNotificationRewardWithdrawn, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) NotifierRewardedEvent(
	opts *ethereum.SubscribeOpts,
	notifierFilter []common.Address,
) *TsNotifierRewardedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsNotifierRewardedSubscription{
		ts,
		opts,
		notifierFilter,
	}
}

type TsNotifierRewardedSubscription struct {
	contract       *TokenStaking
	opts           *ethereum.SubscribeOpts
	notifierFilter []common.Address
}

type tokenStakingNotifierRewardedFunc func(
	Notifier common.Address,
	Amount *big.Int,
	blockNumber uint64,
)

func (nrs *TsNotifierRewardedSubscription) OnEvent(
	handler tokenStakingNotifierRewardedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingNotifierRewarded)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Notifier,
					event.Amount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := nrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (nrs *TsNotifierRewardedSubscription) Pipe(
	sink chan *abi.TokenStakingNotifierRewarded,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(nrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := nrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - nrs.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past NotifierRewarded events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := nrs.contract.PastNotifierRewardedEvents(
					fromBlock,
					nil,
					nrs.notifierFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past NotifierRewarded events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := nrs.contract.watchNotifierRewarded(
		sink,
		nrs.notifierFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchNotifierRewarded(
	sink chan *abi.TokenStakingNotifierRewarded,
	notifierFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchNotifierRewarded(
			&bind.WatchOpts{Context: ctx},
			sink,
			notifierFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event NotifierRewarded had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event NotifierRewarded failed "+
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

func (ts *TokenStaking) PastNotifierRewardedEvents(
	startBlock uint64,
	endBlock *uint64,
	notifierFilter []common.Address,
) ([]*abi.TokenStakingNotifierRewarded, error) {
	iterator, err := ts.contract.FilterNotifierRewarded(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		notifierFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past NotifierRewarded events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingNotifierRewarded, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) OwnerRefreshedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	oldOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) *TsOwnerRefreshedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsOwnerRefreshedSubscription{
		ts,
		opts,
		stakingProviderFilter,
		oldOwnerFilter,
		newOwnerFilter,
	}
}

type TsOwnerRefreshedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	oldOwnerFilter        []common.Address
	newOwnerFilter        []common.Address
}

type tokenStakingOwnerRefreshedFunc func(
	StakingProvider common.Address,
	OldOwner common.Address,
	NewOwner common.Address,
	blockNumber uint64,
)

func (ors *TsOwnerRefreshedSubscription) OnEvent(
	handler tokenStakingOwnerRefreshedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingOwnerRefreshed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakingProvider,
					event.OldOwner,
					event.NewOwner,
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

func (ors *TsOwnerRefreshedSubscription) Pipe(
	sink chan *abi.TokenStakingOwnerRefreshed,
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
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ors.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past OwnerRefreshed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ors.contract.PastOwnerRefreshedEvents(
					fromBlock,
					nil,
					ors.stakingProviderFilter,
					ors.oldOwnerFilter,
					ors.newOwnerFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past OwnerRefreshed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ors.contract.watchOwnerRefreshed(
		sink,
		ors.stakingProviderFilter,
		ors.oldOwnerFilter,
		ors.newOwnerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchOwnerRefreshed(
	sink chan *abi.TokenStakingOwnerRefreshed,
	stakingProviderFilter []common.Address,
	oldOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchOwnerRefreshed(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			oldOwnerFilter,
			newOwnerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event OwnerRefreshed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event OwnerRefreshed failed "+
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

func (ts *TokenStaking) PastOwnerRefreshedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	oldOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) ([]*abi.TokenStakingOwnerRefreshed, error) {
	iterator, err := ts.contract.FilterOwnerRefreshed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		oldOwnerFilter,
		newOwnerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past OwnerRefreshed events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingOwnerRefreshed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) PanicButtonSetEvent(
	opts *ethereum.SubscribeOpts,
	applicationFilter []common.Address,
	panicButtonFilter []common.Address,
) *TsPanicButtonSetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsPanicButtonSetSubscription{
		ts,
		opts,
		applicationFilter,
		panicButtonFilter,
	}
}

type TsPanicButtonSetSubscription struct {
	contract          *TokenStaking
	opts              *ethereum.SubscribeOpts
	applicationFilter []common.Address
	panicButtonFilter []common.Address
}

type tokenStakingPanicButtonSetFunc func(
	Application common.Address,
	PanicButton common.Address,
	blockNumber uint64,
)

func (pbss *TsPanicButtonSetSubscription) OnEvent(
	handler tokenStakingPanicButtonSetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingPanicButtonSet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Application,
					event.PanicButton,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := pbss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (pbss *TsPanicButtonSetSubscription) Pipe(
	sink chan *abi.TokenStakingPanicButtonSet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(pbss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := pbss.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - pbss.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past PanicButtonSet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := pbss.contract.PastPanicButtonSetEvents(
					fromBlock,
					nil,
					pbss.applicationFilter,
					pbss.panicButtonFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past PanicButtonSet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := pbss.contract.watchPanicButtonSet(
		sink,
		pbss.applicationFilter,
		pbss.panicButtonFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchPanicButtonSet(
	sink chan *abi.TokenStakingPanicButtonSet,
	applicationFilter []common.Address,
	panicButtonFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchPanicButtonSet(
			&bind.WatchOpts{Context: ctx},
			sink,
			applicationFilter,
			panicButtonFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event PanicButtonSet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event PanicButtonSet failed "+
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

func (ts *TokenStaking) PastPanicButtonSetEvents(
	startBlock uint64,
	endBlock *uint64,
	applicationFilter []common.Address,
	panicButtonFilter []common.Address,
) ([]*abi.TokenStakingPanicButtonSet, error) {
	iterator, err := ts.contract.FilterPanicButtonSet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		applicationFilter,
		panicButtonFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past PanicButtonSet events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingPanicButtonSet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) SlashingProcessedEvent(
	opts *ethereum.SubscribeOpts,
	callerFilter []common.Address,
) *TsSlashingProcessedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsSlashingProcessedSubscription{
		ts,
		opts,
		callerFilter,
	}
}

type TsSlashingProcessedSubscription struct {
	contract     *TokenStaking
	opts         *ethereum.SubscribeOpts
	callerFilter []common.Address
}

type tokenStakingSlashingProcessedFunc func(
	Caller common.Address,
	Count *big.Int,
	TAmount *big.Int,
	blockNumber uint64,
)

func (sps *TsSlashingProcessedSubscription) OnEvent(
	handler tokenStakingSlashingProcessedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingSlashingProcessed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Caller,
					event.Count,
					event.TAmount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := sps.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (sps *TsSlashingProcessedSubscription) Pipe(
	sink chan *abi.TokenStakingSlashingProcessed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(sps.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := sps.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - sps.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past SlashingProcessed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := sps.contract.PastSlashingProcessedEvents(
					fromBlock,
					nil,
					sps.callerFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past SlashingProcessed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := sps.contract.watchSlashingProcessed(
		sink,
		sps.callerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchSlashingProcessed(
	sink chan *abi.TokenStakingSlashingProcessed,
	callerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchSlashingProcessed(
			&bind.WatchOpts{Context: ctx},
			sink,
			callerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event SlashingProcessed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event SlashingProcessed failed "+
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

func (ts *TokenStaking) PastSlashingProcessedEvents(
	startBlock uint64,
	endBlock *uint64,
	callerFilter []common.Address,
) ([]*abi.TokenStakingSlashingProcessed, error) {
	iterator, err := ts.contract.FilterSlashingProcessed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		callerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past SlashingProcessed events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingSlashingProcessed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) StakeDiscrepancyPenaltySetEvent(
	opts *ethereum.SubscribeOpts,
) *TsStakeDiscrepancyPenaltySetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsStakeDiscrepancyPenaltySetSubscription{
		ts,
		opts,
	}
}

type TsStakeDiscrepancyPenaltySetSubscription struct {
	contract *TokenStaking
	opts     *ethereum.SubscribeOpts
}

type tokenStakingStakeDiscrepancyPenaltySetFunc func(
	Penalty *big.Int,
	RewardMultiplier *big.Int,
	blockNumber uint64,
)

func (sdpss *TsStakeDiscrepancyPenaltySetSubscription) OnEvent(
	handler tokenStakingStakeDiscrepancyPenaltySetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingStakeDiscrepancyPenaltySet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Penalty,
					event.RewardMultiplier,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := sdpss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (sdpss *TsStakeDiscrepancyPenaltySetSubscription) Pipe(
	sink chan *abi.TokenStakingStakeDiscrepancyPenaltySet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(sdpss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := sdpss.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - sdpss.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past StakeDiscrepancyPenaltySet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := sdpss.contract.PastStakeDiscrepancyPenaltySetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past StakeDiscrepancyPenaltySet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := sdpss.contract.watchStakeDiscrepancyPenaltySet(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchStakeDiscrepancyPenaltySet(
	sink chan *abi.TokenStakingStakeDiscrepancyPenaltySet,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchStakeDiscrepancyPenaltySet(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event StakeDiscrepancyPenaltySet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event StakeDiscrepancyPenaltySet failed "+
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

func (ts *TokenStaking) PastStakeDiscrepancyPenaltySetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.TokenStakingStakeDiscrepancyPenaltySet, error) {
	iterator, err := ts.contract.FilterStakeDiscrepancyPenaltySet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past StakeDiscrepancyPenaltySet events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingStakeDiscrepancyPenaltySet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) StakedEvent(
	opts *ethereum.SubscribeOpts,
	stakeTypeFilter []uint8,
	ownerFilter []common.Address,
	stakingProviderFilter []common.Address,
) *TsStakedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsStakedSubscription{
		ts,
		opts,
		stakeTypeFilter,
		ownerFilter,
		stakingProviderFilter,
	}
}

type TsStakedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakeTypeFilter       []uint8
	ownerFilter           []common.Address
	stakingProviderFilter []common.Address
}

type tokenStakingStakedFunc func(
	StakeType uint8,
	Owner common.Address,
	StakingProvider common.Address,
	Beneficiary common.Address,
	Authorizer common.Address,
	Amount *big.Int,
	blockNumber uint64,
)

func (ss *TsStakedSubscription) OnEvent(
	handler tokenStakingStakedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingStaked)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.StakeType,
					event.Owner,
					event.StakingProvider,
					event.Beneficiary,
					event.Authorizer,
					event.Amount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ss *TsStakedSubscription) Pipe(
	sink chan *abi.TokenStakingStaked,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ss.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ss.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past Staked events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ss.contract.PastStakedEvents(
					fromBlock,
					nil,
					ss.stakeTypeFilter,
					ss.ownerFilter,
					ss.stakingProviderFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past Staked events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ss.contract.watchStaked(
		sink,
		ss.stakeTypeFilter,
		ss.ownerFilter,
		ss.stakingProviderFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchStaked(
	sink chan *abi.TokenStakingStaked,
	stakeTypeFilter []uint8,
	ownerFilter []common.Address,
	stakingProviderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchStaked(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakeTypeFilter,
			ownerFilter,
			stakingProviderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event Staked had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event Staked failed "+
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

func (ts *TokenStaking) PastStakedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakeTypeFilter []uint8,
	ownerFilter []common.Address,
	stakingProviderFilter []common.Address,
) ([]*abi.TokenStakingStaked, error) {
	iterator, err := ts.contract.FilterStaked(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakeTypeFilter,
		ownerFilter,
		stakingProviderFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Staked events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingStaked, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) TokensSeizedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
	discrepancyFilter []bool,
) *TsTokensSeizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsTokensSeizedSubscription{
		ts,
		opts,
		stakingProviderFilter,
		discrepancyFilter,
	}
}

type TsTokensSeizedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
	discrepancyFilter     []bool
}

type tokenStakingTokensSeizedFunc func(
	StakingProvider common.Address,
	Amount *big.Int,
	Discrepancy bool,
	blockNumber uint64,
)

func (tss *TsTokensSeizedSubscription) OnEvent(
	handler tokenStakingTokensSeizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingTokensSeized)
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
					event.Discrepancy,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := tss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (tss *TsTokensSeizedSubscription) Pipe(
	sink chan *abi.TokenStakingTokensSeized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(tss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := tss.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - tss.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past TokensSeized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := tss.contract.PastTokensSeizedEvents(
					fromBlock,
					nil,
					tss.stakingProviderFilter,
					tss.discrepancyFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past TokensSeized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := tss.contract.watchTokensSeized(
		sink,
		tss.stakingProviderFilter,
		tss.discrepancyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchTokensSeized(
	sink chan *abi.TokenStakingTokensSeized,
	stakingProviderFilter []common.Address,
	discrepancyFilter []bool,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchTokensSeized(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
			discrepancyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event TokensSeized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event TokensSeized failed "+
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

func (ts *TokenStaking) PastTokensSeizedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
	discrepancyFilter []bool,
) ([]*abi.TokenStakingTokensSeized, error) {
	iterator, err := ts.contract.FilterTokensSeized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
		discrepancyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past TokensSeized events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingTokensSeized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) ToppedUpEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
) *TsToppedUpSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsToppedUpSubscription{
		ts,
		opts,
		stakingProviderFilter,
	}
}

type TsToppedUpSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
}

type tokenStakingToppedUpFunc func(
	StakingProvider common.Address,
	Amount *big.Int,
	blockNumber uint64,
)

func (tus *TsToppedUpSubscription) OnEvent(
	handler tokenStakingToppedUpFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingToppedUp)
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

	sub := tus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (tus *TsToppedUpSubscription) Pipe(
	sink chan *abi.TokenStakingToppedUp,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(tus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := tus.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - tus.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past ToppedUp events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := tus.contract.PastToppedUpEvents(
					fromBlock,
					nil,
					tus.stakingProviderFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past ToppedUp events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := tus.contract.watchToppedUp(
		sink,
		tus.stakingProviderFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchToppedUp(
	sink chan *abi.TokenStakingToppedUp,
	stakingProviderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchToppedUp(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event ToppedUp had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event ToppedUp failed "+
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

func (ts *TokenStaking) PastToppedUpEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
) ([]*abi.TokenStakingToppedUp, error) {
	iterator, err := ts.contract.FilterToppedUp(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ToppedUp events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingToppedUp, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (ts *TokenStaking) UnstakedEvent(
	opts *ethereum.SubscribeOpts,
	stakingProviderFilter []common.Address,
) *TsUnstakedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &TsUnstakedSubscription{
		ts,
		opts,
		stakingProviderFilter,
	}
}

type TsUnstakedSubscription struct {
	contract              *TokenStaking
	opts                  *ethereum.SubscribeOpts
	stakingProviderFilter []common.Address
}

type tokenStakingUnstakedFunc func(
	StakingProvider common.Address,
	Amount *big.Int,
	blockNumber uint64,
)

func (us *TsUnstakedSubscription) OnEvent(
	handler tokenStakingUnstakedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.TokenStakingUnstaked)
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

	sub := us.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (us *TsUnstakedSubscription) Pipe(
	sink chan *abi.TokenStakingUnstaked,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(us.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := us.contract.blockCounter.CurrentBlock()
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - us.opts.PastBlocks

				tsLogger.Infof(
					"subscription monitoring fetching past Unstaked events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := us.contract.PastUnstakedEvents(
					fromBlock,
					nil,
					us.stakingProviderFilter,
				)
				if err != nil {
					tsLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				tsLogger.Infof(
					"subscription monitoring fetched [%v] past Unstaked events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := us.contract.watchUnstaked(
		sink,
		us.stakingProviderFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ts *TokenStaking) watchUnstaked(
	sink chan *abi.TokenStakingUnstaked,
	stakingProviderFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return ts.contract.WatchUnstaked(
			&bind.WatchOpts{Context: ctx},
			sink,
			stakingProviderFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		tsLogger.Warnf(
			"subscription to event Unstaked had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		tsLogger.Errorf(
			"subscription to event Unstaked failed "+
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

func (ts *TokenStaking) PastUnstakedEvents(
	startBlock uint64,
	endBlock *uint64,
	stakingProviderFilter []common.Address,
) ([]*abi.TokenStakingUnstaked, error) {
	iterator, err := ts.contract.FilterUnstaked(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		stakingProviderFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Unstaked events: [%v]",
			err,
		)
	}

	events := make([]*abi.TokenStakingUnstaked, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

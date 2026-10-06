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
var espLogger = log.Logger("keep-contract-EcdsaSortitionPool")

type EcdsaSortitionPool struct {
	contract          *abi.EcdsaSortitionPool
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

func NewEcdsaSortitionPool(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*EcdsaSortitionPool, error) {
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

	contract, err := abi.NewEcdsaSortitionPool(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.EcdsaSortitionPoolABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &EcdsaSortitionPool{
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
func (esp *EcdsaSortitionPool) AddBetaOperators(
	arg_operators []common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction addBetaOperators",
		" params: ",
		fmt.Sprint(
			arg_operators,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.AddBetaOperators(
		transactorOptions,
		arg_operators,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"addBetaOperators",
			arg_operators,
		)
	}

	espLogger.Infof(
		"submitted transaction addBetaOperators with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.AddBetaOperators(
				newTransactorOptions,
				arg_operators,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"addBetaOperators",
					arg_operators,
				)
			}

			espLogger.Infof(
				"submitted transaction addBetaOperators with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallAddBetaOperators(
	arg_operators []common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"addBetaOperators",
		&result,
		arg_operators,
	)

	return err
}

func (esp *EcdsaSortitionPool) AddBetaOperatorsGasEstimate(
	arg_operators []common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"addBetaOperators",
		esp.contractABI,
		esp.transactor,
		arg_operators,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) DeactivateChaosnet(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction deactivateChaosnet",
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.DeactivateChaosnet(
		transactorOptions,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"deactivateChaosnet",
		)
	}

	espLogger.Infof(
		"submitted transaction deactivateChaosnet with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.DeactivateChaosnet(
				newTransactorOptions,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"deactivateChaosnet",
				)
			}

			espLogger.Infof(
				"submitted transaction deactivateChaosnet with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallDeactivateChaosnet(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"deactivateChaosnet",
		&result,
	)

	return err
}

func (esp *EcdsaSortitionPool) DeactivateChaosnetGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"deactivateChaosnet",
		esp.contractABI,
		esp.transactor,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) InsertOperator(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction insertOperator",
		" params: ",
		fmt.Sprint(
			arg_operator,
			arg_authorizedStake,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.InsertOperator(
		transactorOptions,
		arg_operator,
		arg_authorizedStake,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"insertOperator",
			arg_operator,
			arg_authorizedStake,
		)
	}

	espLogger.Infof(
		"submitted transaction insertOperator with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.InsertOperator(
				newTransactorOptions,
				arg_operator,
				arg_authorizedStake,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"insertOperator",
					arg_operator,
					arg_authorizedStake,
				)
			}

			espLogger.Infof(
				"submitted transaction insertOperator with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallInsertOperator(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"insertOperator",
		&result,
		arg_operator,
		arg_authorizedStake,
	)

	return err
}

func (esp *EcdsaSortitionPool) InsertOperatorGasEstimate(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"insertOperator",
		esp.contractABI,
		esp.transactor,
		arg_operator,
		arg_authorizedStake,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) Lock(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction lock",
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.Lock(
		transactorOptions,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"lock",
		)
	}

	espLogger.Infof(
		"submitted transaction lock with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.Lock(
				newTransactorOptions,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"lock",
				)
			}

			espLogger.Infof(
				"submitted transaction lock with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallLock(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"lock",
		&result,
	)

	return err
}

func (esp *EcdsaSortitionPool) LockGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"lock",
		esp.contractABI,
		esp.transactor,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) ReceiveApproval(
	arg_sender common.Address,
	arg_amount *big.Int,
	arg_token common.Address,
	arg3 []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction receiveApproval",
		" params: ",
		fmt.Sprint(
			arg_sender,
			arg_amount,
			arg_token,
			arg3,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.ReceiveApproval(
		transactorOptions,
		arg_sender,
		arg_amount,
		arg_token,
		arg3,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"receiveApproval",
			arg_sender,
			arg_amount,
			arg_token,
			arg3,
		)
	}

	espLogger.Infof(
		"submitted transaction receiveApproval with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.ReceiveApproval(
				newTransactorOptions,
				arg_sender,
				arg_amount,
				arg_token,
				arg3,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"receiveApproval",
					arg_sender,
					arg_amount,
					arg_token,
					arg3,
				)
			}

			espLogger.Infof(
				"submitted transaction receiveApproval with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallReceiveApproval(
	arg_sender common.Address,
	arg_amount *big.Int,
	arg_token common.Address,
	arg3 []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"receiveApproval",
		&result,
		arg_sender,
		arg_amount,
		arg_token,
		arg3,
	)

	return err
}

func (esp *EcdsaSortitionPool) ReceiveApprovalGasEstimate(
	arg_sender common.Address,
	arg_amount *big.Int,
	arg_token common.Address,
	arg3 []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"receiveApproval",
		esp.contractABI,
		esp.transactor,
		arg_sender,
		arg_amount,
		arg_token,
		arg3,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) RenounceOwnership(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction renounceOwnership",
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.RenounceOwnership(
		transactorOptions,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"renounceOwnership",
		)
	}

	espLogger.Infof(
		"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.RenounceOwnership(
				newTransactorOptions,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"renounceOwnership",
				)
			}

			espLogger.Infof(
				"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallRenounceOwnership(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"renounceOwnership",
		&result,
	)

	return err
}

func (esp *EcdsaSortitionPool) RenounceOwnershipGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"renounceOwnership",
		esp.contractABI,
		esp.transactor,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) RestoreRewardEligibility(
	arg_operator common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction restoreRewardEligibility",
		" params: ",
		fmt.Sprint(
			arg_operator,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.RestoreRewardEligibility(
		transactorOptions,
		arg_operator,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"restoreRewardEligibility",
			arg_operator,
		)
	}

	espLogger.Infof(
		"submitted transaction restoreRewardEligibility with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.RestoreRewardEligibility(
				newTransactorOptions,
				arg_operator,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"restoreRewardEligibility",
					arg_operator,
				)
			}

			espLogger.Infof(
				"submitted transaction restoreRewardEligibility with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallRestoreRewardEligibility(
	arg_operator common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"restoreRewardEligibility",
		&result,
		arg_operator,
	)

	return err
}

func (esp *EcdsaSortitionPool) RestoreRewardEligibilityGasEstimate(
	arg_operator common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"restoreRewardEligibility",
		esp.contractABI,
		esp.transactor,
		arg_operator,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) SetRewardIneligibility(
	arg_operators []uint32,
	arg_until *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction setRewardIneligibility",
		" params: ",
		fmt.Sprint(
			arg_operators,
			arg_until,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.SetRewardIneligibility(
		transactorOptions,
		arg_operators,
		arg_until,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"setRewardIneligibility",
			arg_operators,
			arg_until,
		)
	}

	espLogger.Infof(
		"submitted transaction setRewardIneligibility with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.SetRewardIneligibility(
				newTransactorOptions,
				arg_operators,
				arg_until,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"setRewardIneligibility",
					arg_operators,
					arg_until,
				)
			}

			espLogger.Infof(
				"submitted transaction setRewardIneligibility with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallSetRewardIneligibility(
	arg_operators []uint32,
	arg_until *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"setRewardIneligibility",
		&result,
		arg_operators,
		arg_until,
	)

	return err
}

func (esp *EcdsaSortitionPool) SetRewardIneligibilityGasEstimate(
	arg_operators []uint32,
	arg_until *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"setRewardIneligibility",
		esp.contractABI,
		esp.transactor,
		arg_operators,
		arg_until,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) TransferChaosnetOwnerRole(
	arg_newChaosnetOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction transferChaosnetOwnerRole",
		" params: ",
		fmt.Sprint(
			arg_newChaosnetOwner,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.TransferChaosnetOwnerRole(
		transactorOptions,
		arg_newChaosnetOwner,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"transferChaosnetOwnerRole",
			arg_newChaosnetOwner,
		)
	}

	espLogger.Infof(
		"submitted transaction transferChaosnetOwnerRole with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.TransferChaosnetOwnerRole(
				newTransactorOptions,
				arg_newChaosnetOwner,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"transferChaosnetOwnerRole",
					arg_newChaosnetOwner,
				)
			}

			espLogger.Infof(
				"submitted transaction transferChaosnetOwnerRole with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallTransferChaosnetOwnerRole(
	arg_newChaosnetOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"transferChaosnetOwnerRole",
		&result,
		arg_newChaosnetOwner,
	)

	return err
}

func (esp *EcdsaSortitionPool) TransferChaosnetOwnerRoleGasEstimate(
	arg_newChaosnetOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"transferChaosnetOwnerRole",
		esp.contractABI,
		esp.transactor,
		arg_newChaosnetOwner,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) TransferOwnership(
	arg_newOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction transferOwnership",
		" params: ",
		fmt.Sprint(
			arg_newOwner,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.TransferOwnership(
		transactorOptions,
		arg_newOwner,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"transferOwnership",
			arg_newOwner,
		)
	}

	espLogger.Infof(
		"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.TransferOwnership(
				newTransactorOptions,
				arg_newOwner,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"transferOwnership",
					arg_newOwner,
				)
			}

			espLogger.Infof(
				"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallTransferOwnership(
	arg_newOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"transferOwnership",
		&result,
		arg_newOwner,
	)

	return err
}

func (esp *EcdsaSortitionPool) TransferOwnershipGasEstimate(
	arg_newOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"transferOwnership",
		esp.contractABI,
		esp.transactor,
		arg_newOwner,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) Unlock(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction unlock",
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.Unlock(
		transactorOptions,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"unlock",
		)
	}

	espLogger.Infof(
		"submitted transaction unlock with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.Unlock(
				newTransactorOptions,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"unlock",
				)
			}

			espLogger.Infof(
				"submitted transaction unlock with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallUnlock(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"unlock",
		&result,
	)

	return err
}

func (esp *EcdsaSortitionPool) UnlockGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"unlock",
		esp.contractABI,
		esp.transactor,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) UpdateOperatorStatus(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction updateOperatorStatus",
		" params: ",
		fmt.Sprint(
			arg_operator,
			arg_authorizedStake,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.UpdateOperatorStatus(
		transactorOptions,
		arg_operator,
		arg_authorizedStake,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"updateOperatorStatus",
			arg_operator,
			arg_authorizedStake,
		)
	}

	espLogger.Infof(
		"submitted transaction updateOperatorStatus with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.UpdateOperatorStatus(
				newTransactorOptions,
				arg_operator,
				arg_authorizedStake,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"updateOperatorStatus",
					arg_operator,
					arg_authorizedStake,
				)
			}

			espLogger.Infof(
				"submitted transaction updateOperatorStatus with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallUpdateOperatorStatus(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"updateOperatorStatus",
		&result,
		arg_operator,
		arg_authorizedStake,
	)

	return err
}

func (esp *EcdsaSortitionPool) UpdateOperatorStatusGasEstimate(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"updateOperatorStatus",
		esp.contractABI,
		esp.transactor,
		arg_operator,
		arg_authorizedStake,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) WithdrawIneligible(
	arg_recipient common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction withdrawIneligible",
		" params: ",
		fmt.Sprint(
			arg_recipient,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.WithdrawIneligible(
		transactorOptions,
		arg_recipient,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"withdrawIneligible",
			arg_recipient,
		)
	}

	espLogger.Infof(
		"submitted transaction withdrawIneligible with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.WithdrawIneligible(
				newTransactorOptions,
				arg_recipient,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"withdrawIneligible",
					arg_recipient,
				)
			}

			espLogger.Infof(
				"submitted transaction withdrawIneligible with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallWithdrawIneligible(
	arg_recipient common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"withdrawIneligible",
		&result,
		arg_recipient,
	)

	return err
}

func (esp *EcdsaSortitionPool) WithdrawIneligibleGasEstimate(
	arg_recipient common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"withdrawIneligible",
		esp.contractABI,
		esp.transactor,
		arg_recipient,
	)

	return result, err
}

// Transaction submission.
func (esp *EcdsaSortitionPool) WithdrawRewards(
	arg_operator common.Address,
	arg_beneficiary common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	espLogger.Debug(
		"submitting transaction withdrawRewards",
		" params: ",
		fmt.Sprint(
			arg_operator,
			arg_beneficiary,
		),
	)

	esp.transactionMutex.Lock()
	defer esp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *esp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := esp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := esp.contract.WithdrawRewards(
		transactorOptions,
		arg_operator,
		arg_beneficiary,
	)
	if err != nil {
		return transaction, esp.errorResolver.ResolveError(
			err,
			esp.transactorOptions.From,
			nil,
			"withdrawRewards",
			arg_operator,
			arg_beneficiary,
		)
	}

	espLogger.Infof(
		"submitted transaction withdrawRewards with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go esp.miningWaiter.ForceMining(
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

			transaction, err := esp.contract.WithdrawRewards(
				newTransactorOptions,
				arg_operator,
				arg_beneficiary,
			)
			if err != nil {
				return nil, esp.errorResolver.ResolveError(
					err,
					esp.transactorOptions.From,
					nil,
					"withdrawRewards",
					arg_operator,
					arg_beneficiary,
				)
			}

			espLogger.Infof(
				"submitted transaction withdrawRewards with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	esp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (esp *EcdsaSortitionPool) CallWithdrawRewards(
	arg_operator common.Address,
	arg_beneficiary common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.transactorOptions.From,
		blockNumber, nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"withdrawRewards",
		&result,
		arg_operator,
		arg_beneficiary,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) WithdrawRewardsGasEstimate(
	arg_operator common.Address,
	arg_beneficiary common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		esp.callerOptions.From,
		esp.contractAddress,
		"withdrawRewards",
		esp.contractABI,
		esp.transactor,
		arg_operator,
		arg_beneficiary,
	)

	return result, err
}

// ----- Const Methods ------

func (esp *EcdsaSortitionPool) CanRestoreRewardEligibility(
	arg_operator common.Address,
) (bool, error) {
	result, err := esp.contract.CanRestoreRewardEligibility(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"canRestoreRewardEligibility",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) CanRestoreRewardEligibilityAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"canRestoreRewardEligibility",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) ChaosnetOwner() (common.Address, error) {
	result, err := esp.contract.ChaosnetOwner(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"chaosnetOwner",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) ChaosnetOwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"chaosnetOwner",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) GetAvailableRewards(
	arg_operator common.Address,
) (*big.Int, error) {
	result, err := esp.contract.GetAvailableRewards(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"getAvailableRewards",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) GetAvailableRewardsAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"getAvailableRewards",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) GetIDOperator(
	arg_id uint32,
) (common.Address, error) {
	result, err := esp.contract.GetIDOperator(
		esp.callerOptions,
		arg_id,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"getIDOperator",
			arg_id,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) GetIDOperatorAtBlock(
	arg_id uint32,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"getIDOperator",
		&result,
		arg_id,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) GetIDOperators(
	arg_ids []uint32,
) ([]common.Address, error) {
	result, err := esp.contract.GetIDOperators(
		esp.callerOptions,
		arg_ids,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"getIDOperators",
			arg_ids,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) GetIDOperatorsAtBlock(
	arg_ids []uint32,
	blockNumber *big.Int,
) ([]common.Address, error) {
	var result []common.Address

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"getIDOperators",
		&result,
		arg_ids,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) GetOperatorID(
	arg_operator common.Address,
) (uint32, error) {
	result, err := esp.contract.GetOperatorID(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"getOperatorID",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) GetOperatorIDAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"getOperatorID",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) GetPoolWeight(
	arg_operator common.Address,
) (*big.Int, error) {
	result, err := esp.contract.GetPoolWeight(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"getPoolWeight",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) GetPoolWeightAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"getPoolWeight",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IneligibleEarnedRewards() (*big.Int, error) {
	result, err := esp.contract.IneligibleEarnedRewards(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"ineligibleEarnedRewards",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IneligibleEarnedRewardsAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"ineligibleEarnedRewards",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IsBetaOperator(
	arg0 common.Address,
) (bool, error) {
	result, err := esp.contract.IsBetaOperator(
		esp.callerOptions,
		arg0,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"isBetaOperator",
			arg0,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IsBetaOperatorAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"isBetaOperator",
		&result,
		arg0,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IsChaosnetActive() (bool, error) {
	result, err := esp.contract.IsChaosnetActive(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"isChaosnetActive",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IsChaosnetActiveAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"isChaosnetActive",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IsEligibleForRewards(
	arg_operator common.Address,
) (bool, error) {
	result, err := esp.contract.IsEligibleForRewards(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"isEligibleForRewards",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IsEligibleForRewardsAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"isEligibleForRewards",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IsLocked() (bool, error) {
	result, err := esp.contract.IsLocked(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"isLocked",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IsLockedAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"isLocked",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IsOperatorInPool(
	arg_operator common.Address,
) (bool, error) {
	result, err := esp.contract.IsOperatorInPool(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"isOperatorInPool",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IsOperatorInPoolAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"isOperatorInPool",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IsOperatorRegistered(
	arg_operator common.Address,
) (bool, error) {
	result, err := esp.contract.IsOperatorRegistered(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"isOperatorRegistered",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IsOperatorRegisteredAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"isOperatorRegistered",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) IsOperatorUpToDate(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,
) (bool, error) {
	result, err := esp.contract.IsOperatorUpToDate(
		esp.callerOptions,
		arg_operator,
		arg_authorizedStake,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"isOperatorUpToDate",
			arg_operator,
			arg_authorizedStake,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) IsOperatorUpToDateAtBlock(
	arg_operator common.Address,
	arg_authorizedStake *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"isOperatorUpToDate",
		&result,
		arg_operator,
		arg_authorizedStake,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) OperatorsInPool() (*big.Int, error) {
	result, err := esp.contract.OperatorsInPool(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"operatorsInPool",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) OperatorsInPoolAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"operatorsInPool",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) Owner() (common.Address, error) {
	result, err := esp.contract.Owner(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"owner",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) OwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"owner",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) PoolWeightDivisor() (*big.Int, error) {
	result, err := esp.contract.PoolWeightDivisor(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"poolWeightDivisor",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) PoolWeightDivisorAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"poolWeightDivisor",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) RewardToken() (common.Address, error) {
	result, err := esp.contract.RewardToken(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"rewardToken",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) RewardTokenAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"rewardToken",
		&result,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) RewardsEligibilityRestorableAt(
	arg_operator common.Address,
) (*big.Int, error) {
	result, err := esp.contract.RewardsEligibilityRestorableAt(
		esp.callerOptions,
		arg_operator,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"rewardsEligibilityRestorableAt",
			arg_operator,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) RewardsEligibilityRestorableAtAtBlock(
	arg_operator common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"rewardsEligibilityRestorableAt",
		&result,
		arg_operator,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) SelectGroup(
	arg_groupSize *big.Int,
	arg_seed [32]byte,
) ([]uint32, error) {
	result, err := esp.contract.SelectGroup(
		esp.callerOptions,
		arg_groupSize,
		arg_seed,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"selectGroup",
			arg_groupSize,
			arg_seed,
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) SelectGroupAtBlock(
	arg_groupSize *big.Int,
	arg_seed [32]byte,
	blockNumber *big.Int,
) ([]uint32, error) {
	var result []uint32

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"selectGroup",
		&result,
		arg_groupSize,
		arg_seed,
	)

	return result, err
}

func (esp *EcdsaSortitionPool) TotalWeight() (*big.Int, error) {
	result, err := esp.contract.TotalWeight(
		esp.callerOptions,
	)

	if err != nil {
		return result, esp.errorResolver.ResolveError(
			err,
			esp.callerOptions.From,
			nil,
			"totalWeight",
		)
	}

	return result, err
}

func (esp *EcdsaSortitionPool) TotalWeightAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		esp.callerOptions.From,
		blockNumber,
		nil,
		esp.contractABI,
		esp.caller,
		esp.errorResolver,
		esp.contractAddress,
		"totalWeight",
		&result,
	)

	return result, err
}

// ------ Events -------

func (esp *EcdsaSortitionPool) BetaOperatorsAddedEvent(
	opts *ethereum.SubscribeOpts,
) *EspBetaOperatorsAddedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &EspBetaOperatorsAddedSubscription{
		esp,
		opts,
	}
}

type EspBetaOperatorsAddedSubscription struct {
	contract *EcdsaSortitionPool
	opts     *ethereum.SubscribeOpts
}

type ecdsaSortitionPoolBetaOperatorsAddedFunc func(
	Operators []common.Address,
	blockNumber uint64,
)

func (boas *EspBetaOperatorsAddedSubscription) OnEvent(
	handler ecdsaSortitionPoolBetaOperatorsAddedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.EcdsaSortitionPoolBetaOperatorsAdded)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Operators,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := boas.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (boas *EspBetaOperatorsAddedSubscription) Pipe(
	sink chan *abi.EcdsaSortitionPoolBetaOperatorsAdded,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(boas.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := boas.contract.blockCounter.CurrentBlock()
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - boas.opts.PastBlocks

				espLogger.Infof(
					"subscription monitoring fetching past BetaOperatorsAdded events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := boas.contract.PastBetaOperatorsAddedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				espLogger.Infof(
					"subscription monitoring fetched [%v] past BetaOperatorsAdded events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := boas.contract.watchBetaOperatorsAdded(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (esp *EcdsaSortitionPool) watchBetaOperatorsAdded(
	sink chan *abi.EcdsaSortitionPoolBetaOperatorsAdded,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return esp.contract.WatchBetaOperatorsAdded(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		espLogger.Warnf(
			"subscription to event BetaOperatorsAdded had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		espLogger.Errorf(
			"subscription to event BetaOperatorsAdded failed "+
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

func (esp *EcdsaSortitionPool) PastBetaOperatorsAddedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.EcdsaSortitionPoolBetaOperatorsAdded, error) {
	iterator, err := esp.contract.FilterBetaOperatorsAdded(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past BetaOperatorsAdded events: [%v]",
			err,
		)
	}

	events := make([]*abi.EcdsaSortitionPoolBetaOperatorsAdded, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (esp *EcdsaSortitionPool) ChaosnetDeactivatedEvent(
	opts *ethereum.SubscribeOpts,
) *EspChaosnetDeactivatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &EspChaosnetDeactivatedSubscription{
		esp,
		opts,
	}
}

type EspChaosnetDeactivatedSubscription struct {
	contract *EcdsaSortitionPool
	opts     *ethereum.SubscribeOpts
}

type ecdsaSortitionPoolChaosnetDeactivatedFunc func(
	blockNumber uint64,
)

func (cds *EspChaosnetDeactivatedSubscription) OnEvent(
	handler ecdsaSortitionPoolChaosnetDeactivatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.EcdsaSortitionPoolChaosnetDeactivated)
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

	sub := cds.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (cds *EspChaosnetDeactivatedSubscription) Pipe(
	sink chan *abi.EcdsaSortitionPoolChaosnetDeactivated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(cds.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := cds.contract.blockCounter.CurrentBlock()
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - cds.opts.PastBlocks

				espLogger.Infof(
					"subscription monitoring fetching past ChaosnetDeactivated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := cds.contract.PastChaosnetDeactivatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				espLogger.Infof(
					"subscription monitoring fetched [%v] past ChaosnetDeactivated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := cds.contract.watchChaosnetDeactivated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (esp *EcdsaSortitionPool) watchChaosnetDeactivated(
	sink chan *abi.EcdsaSortitionPoolChaosnetDeactivated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return esp.contract.WatchChaosnetDeactivated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		espLogger.Warnf(
			"subscription to event ChaosnetDeactivated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		espLogger.Errorf(
			"subscription to event ChaosnetDeactivated failed "+
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

func (esp *EcdsaSortitionPool) PastChaosnetDeactivatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.EcdsaSortitionPoolChaosnetDeactivated, error) {
	iterator, err := esp.contract.FilterChaosnetDeactivated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ChaosnetDeactivated events: [%v]",
			err,
		)
	}

	events := make([]*abi.EcdsaSortitionPoolChaosnetDeactivated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (esp *EcdsaSortitionPool) ChaosnetOwnerRoleTransferredEvent(
	opts *ethereum.SubscribeOpts,
) *EspChaosnetOwnerRoleTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &EspChaosnetOwnerRoleTransferredSubscription{
		esp,
		opts,
	}
}

type EspChaosnetOwnerRoleTransferredSubscription struct {
	contract *EcdsaSortitionPool
	opts     *ethereum.SubscribeOpts
}

type ecdsaSortitionPoolChaosnetOwnerRoleTransferredFunc func(
	OldChaosnetOwner common.Address,
	NewChaosnetOwner common.Address,
	blockNumber uint64,
)

func (corts *EspChaosnetOwnerRoleTransferredSubscription) OnEvent(
	handler ecdsaSortitionPoolChaosnetOwnerRoleTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.EcdsaSortitionPoolChaosnetOwnerRoleTransferred)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.OldChaosnetOwner,
					event.NewChaosnetOwner,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := corts.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (corts *EspChaosnetOwnerRoleTransferredSubscription) Pipe(
	sink chan *abi.EcdsaSortitionPoolChaosnetOwnerRoleTransferred,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(corts.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := corts.contract.blockCounter.CurrentBlock()
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - corts.opts.PastBlocks

				espLogger.Infof(
					"subscription monitoring fetching past ChaosnetOwnerRoleTransferred events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := corts.contract.PastChaosnetOwnerRoleTransferredEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				espLogger.Infof(
					"subscription monitoring fetched [%v] past ChaosnetOwnerRoleTransferred events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := corts.contract.watchChaosnetOwnerRoleTransferred(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (esp *EcdsaSortitionPool) watchChaosnetOwnerRoleTransferred(
	sink chan *abi.EcdsaSortitionPoolChaosnetOwnerRoleTransferred,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return esp.contract.WatchChaosnetOwnerRoleTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		espLogger.Warnf(
			"subscription to event ChaosnetOwnerRoleTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		espLogger.Errorf(
			"subscription to event ChaosnetOwnerRoleTransferred failed "+
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

func (esp *EcdsaSortitionPool) PastChaosnetOwnerRoleTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.EcdsaSortitionPoolChaosnetOwnerRoleTransferred, error) {
	iterator, err := esp.contract.FilterChaosnetOwnerRoleTransferred(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ChaosnetOwnerRoleTransferred events: [%v]",
			err,
		)
	}

	events := make([]*abi.EcdsaSortitionPoolChaosnetOwnerRoleTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (esp *EcdsaSortitionPool) IneligibleForRewardsEvent(
	opts *ethereum.SubscribeOpts,
) *EspIneligibleForRewardsSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &EspIneligibleForRewardsSubscription{
		esp,
		opts,
	}
}

type EspIneligibleForRewardsSubscription struct {
	contract *EcdsaSortitionPool
	opts     *ethereum.SubscribeOpts
}

type ecdsaSortitionPoolIneligibleForRewardsFunc func(
	Ids []uint32,
	Until *big.Int,
	blockNumber uint64,
)

func (ifrs *EspIneligibleForRewardsSubscription) OnEvent(
	handler ecdsaSortitionPoolIneligibleForRewardsFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.EcdsaSortitionPoolIneligibleForRewards)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Ids,
					event.Until,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ifrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ifrs *EspIneligibleForRewardsSubscription) Pipe(
	sink chan *abi.EcdsaSortitionPoolIneligibleForRewards,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ifrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ifrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ifrs.opts.PastBlocks

				espLogger.Infof(
					"subscription monitoring fetching past IneligibleForRewards events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ifrs.contract.PastIneligibleForRewardsEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				espLogger.Infof(
					"subscription monitoring fetched [%v] past IneligibleForRewards events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ifrs.contract.watchIneligibleForRewards(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (esp *EcdsaSortitionPool) watchIneligibleForRewards(
	sink chan *abi.EcdsaSortitionPoolIneligibleForRewards,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return esp.contract.WatchIneligibleForRewards(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		espLogger.Warnf(
			"subscription to event IneligibleForRewards had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		espLogger.Errorf(
			"subscription to event IneligibleForRewards failed "+
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

func (esp *EcdsaSortitionPool) PastIneligibleForRewardsEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.EcdsaSortitionPoolIneligibleForRewards, error) {
	iterator, err := esp.contract.FilterIneligibleForRewards(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past IneligibleForRewards events: [%v]",
			err,
		)
	}

	events := make([]*abi.EcdsaSortitionPoolIneligibleForRewards, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (esp *EcdsaSortitionPool) OwnershipTransferredEvent(
	opts *ethereum.SubscribeOpts,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) *EspOwnershipTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &EspOwnershipTransferredSubscription{
		esp,
		opts,
		previousOwnerFilter,
		newOwnerFilter,
	}
}

type EspOwnershipTransferredSubscription struct {
	contract            *EcdsaSortitionPool
	opts                *ethereum.SubscribeOpts
	previousOwnerFilter []common.Address
	newOwnerFilter      []common.Address
}

type ecdsaSortitionPoolOwnershipTransferredFunc func(
	PreviousOwner common.Address,
	NewOwner common.Address,
	blockNumber uint64,
)

func (ots *EspOwnershipTransferredSubscription) OnEvent(
	handler ecdsaSortitionPoolOwnershipTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.EcdsaSortitionPoolOwnershipTransferred)
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

func (ots *EspOwnershipTransferredSubscription) Pipe(
	sink chan *abi.EcdsaSortitionPoolOwnershipTransferred,
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
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ots.opts.PastBlocks

				espLogger.Infof(
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
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				espLogger.Infof(
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

func (esp *EcdsaSortitionPool) watchOwnershipTransferred(
	sink chan *abi.EcdsaSortitionPoolOwnershipTransferred,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return esp.contract.WatchOwnershipTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
			previousOwnerFilter,
			newOwnerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		espLogger.Warnf(
			"subscription to event OwnershipTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		espLogger.Errorf(
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

func (esp *EcdsaSortitionPool) PastOwnershipTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) ([]*abi.EcdsaSortitionPoolOwnershipTransferred, error) {
	iterator, err := esp.contract.FilterOwnershipTransferred(
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

	events := make([]*abi.EcdsaSortitionPoolOwnershipTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (esp *EcdsaSortitionPool) RewardEligibilityRestoredEvent(
	opts *ethereum.SubscribeOpts,
	operatorFilter []common.Address,
	idFilter []uint32,
) *EspRewardEligibilityRestoredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &EspRewardEligibilityRestoredSubscription{
		esp,
		opts,
		operatorFilter,
		idFilter,
	}
}

type EspRewardEligibilityRestoredSubscription struct {
	contract       *EcdsaSortitionPool
	opts           *ethereum.SubscribeOpts
	operatorFilter []common.Address
	idFilter       []uint32
}

type ecdsaSortitionPoolRewardEligibilityRestoredFunc func(
	Operator common.Address,
	Id uint32,
	blockNumber uint64,
)

func (rers *EspRewardEligibilityRestoredSubscription) OnEvent(
	handler ecdsaSortitionPoolRewardEligibilityRestoredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.EcdsaSortitionPoolRewardEligibilityRestored)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Operator,
					event.Id,
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

func (rers *EspRewardEligibilityRestoredSubscription) Pipe(
	sink chan *abi.EcdsaSortitionPoolRewardEligibilityRestored,
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
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rers.opts.PastBlocks

				espLogger.Infof(
					"subscription monitoring fetching past RewardEligibilityRestored events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rers.contract.PastRewardEligibilityRestoredEvents(
					fromBlock,
					nil,
					rers.operatorFilter,
					rers.idFilter,
				)
				if err != nil {
					espLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				espLogger.Infof(
					"subscription monitoring fetched [%v] past RewardEligibilityRestored events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rers.contract.watchRewardEligibilityRestored(
		sink,
		rers.operatorFilter,
		rers.idFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (esp *EcdsaSortitionPool) watchRewardEligibilityRestored(
	sink chan *abi.EcdsaSortitionPoolRewardEligibilityRestored,
	operatorFilter []common.Address,
	idFilter []uint32,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return esp.contract.WatchRewardEligibilityRestored(
			&bind.WatchOpts{Context: ctx},
			sink,
			operatorFilter,
			idFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		espLogger.Warnf(
			"subscription to event RewardEligibilityRestored had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		espLogger.Errorf(
			"subscription to event RewardEligibilityRestored failed "+
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

func (esp *EcdsaSortitionPool) PastRewardEligibilityRestoredEvents(
	startBlock uint64,
	endBlock *uint64,
	operatorFilter []common.Address,
	idFilter []uint32,
) ([]*abi.EcdsaSortitionPoolRewardEligibilityRestored, error) {
	iterator, err := esp.contract.FilterRewardEligibilityRestored(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		operatorFilter,
		idFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RewardEligibilityRestored events: [%v]",
			err,
		)
	}

	events := make([]*abi.EcdsaSortitionPoolRewardEligibilityRestored, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

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
var rwLogger = log.Logger("keep-contract-RedemptionWatchtower")

type RedemptionWatchtower struct {
	contract          *abi.RedemptionWatchtower
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

func NewRedemptionWatchtower(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*RedemptionWatchtower, error) {
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

	contract, err := abi.NewRedemptionWatchtower(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.RedemptionWatchtowerABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &RedemptionWatchtower{
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
func (rw *RedemptionWatchtower) AddGuardian(
	arg_guardian common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction addGuardian",
		" params: ",
		fmt.Sprint(
			arg_guardian,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.AddGuardian(
		transactorOptions,
		arg_guardian,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"addGuardian",
			arg_guardian,
		)
	}

	rwLogger.Infof(
		"submitted transaction addGuardian with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.AddGuardian(
				newTransactorOptions,
				arg_guardian,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"addGuardian",
					arg_guardian,
				)
			}

			rwLogger.Infof(
				"submitted transaction addGuardian with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallAddGuardian(
	arg_guardian common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"addGuardian",
		&result,
		arg_guardian,
	)

	return err
}

func (rw *RedemptionWatchtower) AddGuardianGasEstimate(
	arg_guardian common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"addGuardian",
		rw.contractABI,
		rw.transactor,
		arg_guardian,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) DisableWatchtower(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction disableWatchtower",
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.DisableWatchtower(
		transactorOptions,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"disableWatchtower",
		)
	}

	rwLogger.Infof(
		"submitted transaction disableWatchtower with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.DisableWatchtower(
				newTransactorOptions,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"disableWatchtower",
				)
			}

			rwLogger.Infof(
				"submitted transaction disableWatchtower with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallDisableWatchtower(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"disableWatchtower",
		&result,
	)

	return err
}

func (rw *RedemptionWatchtower) DisableWatchtowerGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"disableWatchtower",
		rw.contractABI,
		rw.transactor,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) EnableWatchtower(
	arg__manager common.Address,
	arg__guardians []common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction enableWatchtower",
		" params: ",
		fmt.Sprint(
			arg__manager,
			arg__guardians,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.EnableWatchtower(
		transactorOptions,
		arg__manager,
		arg__guardians,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"enableWatchtower",
			arg__manager,
			arg__guardians,
		)
	}

	rwLogger.Infof(
		"submitted transaction enableWatchtower with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.EnableWatchtower(
				newTransactorOptions,
				arg__manager,
				arg__guardians,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"enableWatchtower",
					arg__manager,
					arg__guardians,
				)
			}

			rwLogger.Infof(
				"submitted transaction enableWatchtower with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallEnableWatchtower(
	arg__manager common.Address,
	arg__guardians []common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"enableWatchtower",
		&result,
		arg__manager,
		arg__guardians,
	)

	return err
}

func (rw *RedemptionWatchtower) EnableWatchtowerGasEstimate(
	arg__manager common.Address,
	arg__guardians []common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"enableWatchtower",
		rw.contractABI,
		rw.transactor,
		arg__manager,
		arg__guardians,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) Initialize(
	arg__bridge common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction initialize",
		" params: ",
		fmt.Sprint(
			arg__bridge,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.Initialize(
		transactorOptions,
		arg__bridge,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"initialize",
			arg__bridge,
		)
	}

	rwLogger.Infof(
		"submitted transaction initialize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.Initialize(
				newTransactorOptions,
				arg__bridge,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"initialize",
					arg__bridge,
				)
			}

			rwLogger.Infof(
				"submitted transaction initialize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallInitialize(
	arg__bridge common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"initialize",
		&result,
		arg__bridge,
	)

	return err
}

func (rw *RedemptionWatchtower) InitializeGasEstimate(
	arg__bridge common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"initialize",
		rw.contractABI,
		rw.transactor,
		arg__bridge,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) RaiseObjection(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction raiseObjection",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.RaiseObjection(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"raiseObjection",
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
		)
	}

	rwLogger.Infof(
		"submitted transaction raiseObjection with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.RaiseObjection(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_redeemerOutputScript,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"raiseObjection",
					arg_walletPubKeyHash,
					arg_redeemerOutputScript,
				)
			}

			rwLogger.Infof(
				"submitted transaction raiseObjection with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallRaiseObjection(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"raiseObjection",
		&result,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
	)

	return err
}

func (rw *RedemptionWatchtower) RaiseObjectionGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"raiseObjection",
		rw.contractABI,
		rw.transactor,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) RemoveGuardian(
	arg_guardian common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction removeGuardian",
		" params: ",
		fmt.Sprint(
			arg_guardian,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.RemoveGuardian(
		transactorOptions,
		arg_guardian,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"removeGuardian",
			arg_guardian,
		)
	}

	rwLogger.Infof(
		"submitted transaction removeGuardian with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.RemoveGuardian(
				newTransactorOptions,
				arg_guardian,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"removeGuardian",
					arg_guardian,
				)
			}

			rwLogger.Infof(
				"submitted transaction removeGuardian with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallRemoveGuardian(
	arg_guardian common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"removeGuardian",
		&result,
		arg_guardian,
	)

	return err
}

func (rw *RedemptionWatchtower) RemoveGuardianGasEstimate(
	arg_guardian common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"removeGuardian",
		rw.contractABI,
		rw.transactor,
		arg_guardian,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) RenounceOwnership(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction renounceOwnership",
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.RenounceOwnership(
		transactorOptions,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"renounceOwnership",
		)
	}

	rwLogger.Infof(
		"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.RenounceOwnership(
				newTransactorOptions,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"renounceOwnership",
				)
			}

			rwLogger.Infof(
				"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallRenounceOwnership(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"renounceOwnership",
		&result,
	)

	return err
}

func (rw *RedemptionWatchtower) RenounceOwnershipGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"renounceOwnership",
		rw.contractABI,
		rw.transactor,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) TransferOwnership(
	arg_newOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction transferOwnership",
		" params: ",
		fmt.Sprint(
			arg_newOwner,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.TransferOwnership(
		transactorOptions,
		arg_newOwner,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"transferOwnership",
			arg_newOwner,
		)
	}

	rwLogger.Infof(
		"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.TransferOwnership(
				newTransactorOptions,
				arg_newOwner,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"transferOwnership",
					arg_newOwner,
				)
			}

			rwLogger.Infof(
				"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallTransferOwnership(
	arg_newOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"transferOwnership",
		&result,
		arg_newOwner,
	)

	return err
}

func (rw *RedemptionWatchtower) TransferOwnershipGasEstimate(
	arg_newOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"transferOwnership",
		rw.contractABI,
		rw.transactor,
		arg_newOwner,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) Unban(
	arg_redeemer common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction unban",
		" params: ",
		fmt.Sprint(
			arg_redeemer,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.Unban(
		transactorOptions,
		arg_redeemer,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"unban",
			arg_redeemer,
		)
	}

	rwLogger.Infof(
		"submitted transaction unban with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.Unban(
				newTransactorOptions,
				arg_redeemer,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"unban",
					arg_redeemer,
				)
			}

			rwLogger.Infof(
				"submitted transaction unban with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallUnban(
	arg_redeemer common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"unban",
		&result,
		arg_redeemer,
	)

	return err
}

func (rw *RedemptionWatchtower) UnbanGasEstimate(
	arg_redeemer common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"unban",
		rw.contractABI,
		rw.transactor,
		arg_redeemer,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) UpdateWatchtowerParameters(
	arg__watchtowerLifetime uint32,
	arg__vetoPenaltyFeeDivisor uint64,
	arg__vetoFreezePeriod uint32,
	arg__defaultDelay uint32,
	arg__levelOneDelay uint32,
	arg__levelTwoDelay uint32,
	arg__waivedAmountLimit uint64,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction updateWatchtowerParameters",
		" params: ",
		fmt.Sprint(
			arg__watchtowerLifetime,
			arg__vetoPenaltyFeeDivisor,
			arg__vetoFreezePeriod,
			arg__defaultDelay,
			arg__levelOneDelay,
			arg__levelTwoDelay,
			arg__waivedAmountLimit,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.UpdateWatchtowerParameters(
		transactorOptions,
		arg__watchtowerLifetime,
		arg__vetoPenaltyFeeDivisor,
		arg__vetoFreezePeriod,
		arg__defaultDelay,
		arg__levelOneDelay,
		arg__levelTwoDelay,
		arg__waivedAmountLimit,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"updateWatchtowerParameters",
			arg__watchtowerLifetime,
			arg__vetoPenaltyFeeDivisor,
			arg__vetoFreezePeriod,
			arg__defaultDelay,
			arg__levelOneDelay,
			arg__levelTwoDelay,
			arg__waivedAmountLimit,
		)
	}

	rwLogger.Infof(
		"submitted transaction updateWatchtowerParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.UpdateWatchtowerParameters(
				newTransactorOptions,
				arg__watchtowerLifetime,
				arg__vetoPenaltyFeeDivisor,
				arg__vetoFreezePeriod,
				arg__defaultDelay,
				arg__levelOneDelay,
				arg__levelTwoDelay,
				arg__waivedAmountLimit,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"updateWatchtowerParameters",
					arg__watchtowerLifetime,
					arg__vetoPenaltyFeeDivisor,
					arg__vetoFreezePeriod,
					arg__defaultDelay,
					arg__levelOneDelay,
					arg__levelTwoDelay,
					arg__waivedAmountLimit,
				)
			}

			rwLogger.Infof(
				"submitted transaction updateWatchtowerParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallUpdateWatchtowerParameters(
	arg__watchtowerLifetime uint32,
	arg__vetoPenaltyFeeDivisor uint64,
	arg__vetoFreezePeriod uint32,
	arg__defaultDelay uint32,
	arg__levelOneDelay uint32,
	arg__levelTwoDelay uint32,
	arg__waivedAmountLimit uint64,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"updateWatchtowerParameters",
		&result,
		arg__watchtowerLifetime,
		arg__vetoPenaltyFeeDivisor,
		arg__vetoFreezePeriod,
		arg__defaultDelay,
		arg__levelOneDelay,
		arg__levelTwoDelay,
		arg__waivedAmountLimit,
	)

	return err
}

func (rw *RedemptionWatchtower) UpdateWatchtowerParametersGasEstimate(
	arg__watchtowerLifetime uint32,
	arg__vetoPenaltyFeeDivisor uint64,
	arg__vetoFreezePeriod uint32,
	arg__defaultDelay uint32,
	arg__levelOneDelay uint32,
	arg__levelTwoDelay uint32,
	arg__waivedAmountLimit uint64,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"updateWatchtowerParameters",
		rw.contractABI,
		rw.transactor,
		arg__watchtowerLifetime,
		arg__vetoPenaltyFeeDivisor,
		arg__vetoFreezePeriod,
		arg__defaultDelay,
		arg__levelOneDelay,
		arg__levelTwoDelay,
		arg__waivedAmountLimit,
	)

	return result, err
}

// Transaction submission.
func (rw *RedemptionWatchtower) WithdrawVetoedFunds(
	arg_redemptionKey *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rwLogger.Debug(
		"submitting transaction withdrawVetoedFunds",
		" params: ",
		fmt.Sprint(
			arg_redemptionKey,
		),
	)

	rw.transactionMutex.Lock()
	defer rw.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rw.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rw.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rw.contract.WithdrawVetoedFunds(
		transactorOptions,
		arg_redemptionKey,
	)
	if err != nil {
		return transaction, rw.errorResolver.ResolveError(
			err,
			rw.transactorOptions.From,
			nil,
			"withdrawVetoedFunds",
			arg_redemptionKey,
		)
	}

	rwLogger.Infof(
		"submitted transaction withdrawVetoedFunds with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rw.miningWaiter.ForceMining(
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

			transaction, err := rw.contract.WithdrawVetoedFunds(
				newTransactorOptions,
				arg_redemptionKey,
			)
			if err != nil {
				return nil, rw.errorResolver.ResolveError(
					err,
					rw.transactorOptions.From,
					nil,
					"withdrawVetoedFunds",
					arg_redemptionKey,
				)
			}

			rwLogger.Infof(
				"submitted transaction withdrawVetoedFunds with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rw.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rw *RedemptionWatchtower) CallWithdrawVetoedFunds(
	arg_redemptionKey *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rw.transactorOptions.From,
		blockNumber, nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"withdrawVetoedFunds",
		&result,
		arg_redemptionKey,
	)

	return err
}

func (rw *RedemptionWatchtower) WithdrawVetoedFundsGasEstimate(
	arg_redemptionKey *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rw.callerOptions.From,
		rw.contractAddress,
		"withdrawVetoedFunds",
		rw.contractABI,
		rw.transactor,
		arg_redemptionKey,
	)

	return result, err
}

// ----- Const Methods ------

func (rw *RedemptionWatchtower) Bank() (common.Address, error) {
	result, err := rw.contract.Bank(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"bank",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) BankAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"bank",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) Bridge() (common.Address, error) {
	result, err := rw.contract.Bridge(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"bridge",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) BridgeAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"bridge",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) DefaultDelay() (uint32, error) {
	result, err := rw.contract.DefaultDelay(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"defaultDelay",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) DefaultDelayAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"defaultDelay",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) GetRedemptionDelay(
	arg_redemptionKey *big.Int,
) (uint32, error) {
	result, err := rw.contract.GetRedemptionDelay(
		rw.callerOptions,
		arg_redemptionKey,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"getRedemptionDelay",
			arg_redemptionKey,
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) GetRedemptionDelayAtBlock(
	arg_redemptionKey *big.Int,
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"getRedemptionDelay",
		&result,
		arg_redemptionKey,
	)

	return result, err
}

func (rw *RedemptionWatchtower) IsBanned(
	arg0 common.Address,
) (bool, error) {
	result, err := rw.contract.IsBanned(
		rw.callerOptions,
		arg0,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"isBanned",
			arg0,
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) IsBannedAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"isBanned",
		&result,
		arg0,
	)

	return result, err
}

func (rw *RedemptionWatchtower) IsGuardian(
	arg0 common.Address,
) (bool, error) {
	result, err := rw.contract.IsGuardian(
		rw.callerOptions,
		arg0,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"isGuardian",
			arg0,
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) IsGuardianAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"isGuardian",
		&result,
		arg0,
	)

	return result, err
}

func (rw *RedemptionWatchtower) IsSafeRedemption(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,
	arg_balanceOwner common.Address,
	arg_redeemer common.Address,
) (bool, error) {
	result, err := rw.contract.IsSafeRedemption(
		rw.callerOptions,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
		arg_balanceOwner,
		arg_redeemer,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"isSafeRedemption",
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
			arg_balanceOwner,
			arg_redeemer,
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) IsSafeRedemptionAtBlock(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,
	arg_balanceOwner common.Address,
	arg_redeemer common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"isSafeRedemption",
		&result,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
		arg_balanceOwner,
		arg_redeemer,
	)

	return result, err
}

func (rw *RedemptionWatchtower) LevelOneDelay() (uint32, error) {
	result, err := rw.contract.LevelOneDelay(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"levelOneDelay",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) LevelOneDelayAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"levelOneDelay",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) LevelTwoDelay() (uint32, error) {
	result, err := rw.contract.LevelTwoDelay(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"levelTwoDelay",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) LevelTwoDelayAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"levelTwoDelay",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) Manager() (common.Address, error) {
	result, err := rw.contract.Manager(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"manager",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) ManagerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"manager",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) Objections(
	arg0 *big.Int,
) (bool, error) {
	result, err := rw.contract.Objections(
		rw.callerOptions,
		arg0,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"objections",
			arg0,
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) ObjectionsAtBlock(
	arg0 *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"objections",
		&result,
		arg0,
	)

	return result, err
}

func (rw *RedemptionWatchtower) Owner() (common.Address, error) {
	result, err := rw.contract.Owner(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"owner",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) OwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"owner",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) REQUIREDOBJECTIONSCOUNT() (uint8, error) {
	result, err := rw.contract.REQUIREDOBJECTIONSCOUNT(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"rEQUIREDOBJECTIONSCOUNT",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) REQUIREDOBJECTIONSCOUNTAtBlock(
	blockNumber *big.Int,
) (uint8, error) {
	var result uint8

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"rEQUIREDOBJECTIONSCOUNT",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) VetoFreezePeriod() (uint32, error) {
	result, err := rw.contract.VetoFreezePeriod(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"vetoFreezePeriod",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) VetoFreezePeriodAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"vetoFreezePeriod",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) VetoPenaltyFeeDivisor() (uint64, error) {
	result, err := rw.contract.VetoPenaltyFeeDivisor(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"vetoPenaltyFeeDivisor",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) VetoPenaltyFeeDivisorAtBlock(
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"vetoPenaltyFeeDivisor",
		&result,
	)

	return result, err
}

type vetoProposals struct {
	Redeemer           common.Address
	WithdrawableAmount uint64
	FinalizedAt        uint32
	ObjectionsCount    uint8
}

func (rw *RedemptionWatchtower) VetoProposals(
	arg0 *big.Int,
) (vetoProposals, error) {
	result, err := rw.contract.VetoProposals(
		rw.callerOptions,
		arg0,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"vetoProposals",
			arg0,
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) VetoProposalsAtBlock(
	arg0 *big.Int,
	blockNumber *big.Int,
) (vetoProposals, error) {
	var result vetoProposals

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"vetoProposals",
		&result,
		arg0,
	)

	return result, err
}

func (rw *RedemptionWatchtower) WaivedAmountLimit() (uint64, error) {
	result, err := rw.contract.WaivedAmountLimit(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"waivedAmountLimit",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) WaivedAmountLimitAtBlock(
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"waivedAmountLimit",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) WatchtowerDisabledAt() (uint32, error) {
	result, err := rw.contract.WatchtowerDisabledAt(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"watchtowerDisabledAt",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) WatchtowerDisabledAtAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"watchtowerDisabledAt",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) WatchtowerEnabledAt() (uint32, error) {
	result, err := rw.contract.WatchtowerEnabledAt(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"watchtowerEnabledAt",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) WatchtowerEnabledAtAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"watchtowerEnabledAt",
		&result,
	)

	return result, err
}

func (rw *RedemptionWatchtower) WatchtowerLifetime() (uint32, error) {
	result, err := rw.contract.WatchtowerLifetime(
		rw.callerOptions,
	)

	if err != nil {
		return result, rw.errorResolver.ResolveError(
			err,
			rw.callerOptions.From,
			nil,
			"watchtowerLifetime",
		)
	}

	return result, err
}

func (rw *RedemptionWatchtower) WatchtowerLifetimeAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rw.callerOptions.From,
		blockNumber,
		nil,
		rw.contractABI,
		rw.caller,
		rw.errorResolver,
		rw.contractAddress,
		"watchtowerLifetime",
		&result,
	)

	return result, err
}

// ------ Events -------

func (rw *RedemptionWatchtower) BannedEvent(
	opts *ethereum.SubscribeOpts,
	redeemerFilter []common.Address,
) *RwBannedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwBannedSubscription{
		rw,
		opts,
		redeemerFilter,
	}
}

type RwBannedSubscription struct {
	contract       *RedemptionWatchtower
	opts           *ethereum.SubscribeOpts
	redeemerFilter []common.Address
}

type redemptionWatchtowerBannedFunc func(
	Redeemer common.Address,
	blockNumber uint64,
)

func (bs *RwBannedSubscription) OnEvent(
	handler redemptionWatchtowerBannedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerBanned)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Redeemer,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := bs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (bs *RwBannedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerBanned,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(bs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := bs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - bs.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past Banned events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := bs.contract.PastBannedEvents(
					fromBlock,
					nil,
					bs.redeemerFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past Banned events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := bs.contract.watchBanned(
		sink,
		bs.redeemerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchBanned(
	sink chan *abi.RedemptionWatchtowerBanned,
	redeemerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchBanned(
			&bind.WatchOpts{Context: ctx},
			sink,
			redeemerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event Banned had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event Banned failed "+
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

func (rw *RedemptionWatchtower) PastBannedEvents(
	startBlock uint64,
	endBlock *uint64,
	redeemerFilter []common.Address,
) ([]*abi.RedemptionWatchtowerBanned, error) {
	iterator, err := rw.contract.FilterBanned(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		redeemerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Banned events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerBanned, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) GuardianAddedEvent(
	opts *ethereum.SubscribeOpts,
	guardianFilter []common.Address,
) *RwGuardianAddedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwGuardianAddedSubscription{
		rw,
		opts,
		guardianFilter,
	}
}

type RwGuardianAddedSubscription struct {
	contract       *RedemptionWatchtower
	opts           *ethereum.SubscribeOpts
	guardianFilter []common.Address
}

type redemptionWatchtowerGuardianAddedFunc func(
	Guardian common.Address,
	blockNumber uint64,
)

func (gas *RwGuardianAddedSubscription) OnEvent(
	handler redemptionWatchtowerGuardianAddedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerGuardianAdded)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Guardian,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := gas.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (gas *RwGuardianAddedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerGuardianAdded,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(gas.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := gas.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gas.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past GuardianAdded events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gas.contract.PastGuardianAddedEvents(
					fromBlock,
					nil,
					gas.guardianFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past GuardianAdded events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := gas.contract.watchGuardianAdded(
		sink,
		gas.guardianFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchGuardianAdded(
	sink chan *abi.RedemptionWatchtowerGuardianAdded,
	guardianFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchGuardianAdded(
			&bind.WatchOpts{Context: ctx},
			sink,
			guardianFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event GuardianAdded had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event GuardianAdded failed "+
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

func (rw *RedemptionWatchtower) PastGuardianAddedEvents(
	startBlock uint64,
	endBlock *uint64,
	guardianFilter []common.Address,
) ([]*abi.RedemptionWatchtowerGuardianAdded, error) {
	iterator, err := rw.contract.FilterGuardianAdded(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		guardianFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past GuardianAdded events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerGuardianAdded, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) GuardianRemovedEvent(
	opts *ethereum.SubscribeOpts,
	guardianFilter []common.Address,
) *RwGuardianRemovedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwGuardianRemovedSubscription{
		rw,
		opts,
		guardianFilter,
	}
}

type RwGuardianRemovedSubscription struct {
	contract       *RedemptionWatchtower
	opts           *ethereum.SubscribeOpts
	guardianFilter []common.Address
}

type redemptionWatchtowerGuardianRemovedFunc func(
	Guardian common.Address,
	blockNumber uint64,
)

func (grs *RwGuardianRemovedSubscription) OnEvent(
	handler redemptionWatchtowerGuardianRemovedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerGuardianRemoved)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Guardian,
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

func (grs *RwGuardianRemovedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerGuardianRemoved,
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
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - grs.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past GuardianRemoved events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := grs.contract.PastGuardianRemovedEvents(
					fromBlock,
					nil,
					grs.guardianFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past GuardianRemoved events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := grs.contract.watchGuardianRemoved(
		sink,
		grs.guardianFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchGuardianRemoved(
	sink chan *abi.RedemptionWatchtowerGuardianRemoved,
	guardianFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchGuardianRemoved(
			&bind.WatchOpts{Context: ctx},
			sink,
			guardianFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event GuardianRemoved had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event GuardianRemoved failed "+
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

func (rw *RedemptionWatchtower) PastGuardianRemovedEvents(
	startBlock uint64,
	endBlock *uint64,
	guardianFilter []common.Address,
) ([]*abi.RedemptionWatchtowerGuardianRemoved, error) {
	iterator, err := rw.contract.FilterGuardianRemoved(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		guardianFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past GuardianRemoved events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerGuardianRemoved, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) InitializedEvent(
	opts *ethereum.SubscribeOpts,
) *RwInitializedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwInitializedSubscription{
		rw,
		opts,
	}
}

type RwInitializedSubscription struct {
	contract *RedemptionWatchtower
	opts     *ethereum.SubscribeOpts
}

type redemptionWatchtowerInitializedFunc func(
	Version uint8,
	blockNumber uint64,
)

func (is *RwInitializedSubscription) OnEvent(
	handler redemptionWatchtowerInitializedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerInitialized)
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

func (is *RwInitializedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerInitialized,
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
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - is.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past Initialized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := is.contract.PastInitializedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
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

func (rw *RedemptionWatchtower) watchInitialized(
	sink chan *abi.RedemptionWatchtowerInitialized,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchInitialized(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event Initialized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
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

func (rw *RedemptionWatchtower) PastInitializedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RedemptionWatchtowerInitialized, error) {
	iterator, err := rw.contract.FilterInitialized(
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

	events := make([]*abi.RedemptionWatchtowerInitialized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) ObjectionRaisedEvent(
	opts *ethereum.SubscribeOpts,
	redemptionKeyFilter []*big.Int,
	guardianFilter []common.Address,
) *RwObjectionRaisedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwObjectionRaisedSubscription{
		rw,
		opts,
		redemptionKeyFilter,
		guardianFilter,
	}
}

type RwObjectionRaisedSubscription struct {
	contract            *RedemptionWatchtower
	opts                *ethereum.SubscribeOpts
	redemptionKeyFilter []*big.Int
	guardianFilter      []common.Address
}

type redemptionWatchtowerObjectionRaisedFunc func(
	RedemptionKey *big.Int,
	Guardian common.Address,
	blockNumber uint64,
)

func (ors *RwObjectionRaisedSubscription) OnEvent(
	handler redemptionWatchtowerObjectionRaisedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerObjectionRaised)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RedemptionKey,
					event.Guardian,
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

func (ors *RwObjectionRaisedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerObjectionRaised,
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
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ors.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past ObjectionRaised events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ors.contract.PastObjectionRaisedEvents(
					fromBlock,
					nil,
					ors.redemptionKeyFilter,
					ors.guardianFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past ObjectionRaised events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ors.contract.watchObjectionRaised(
		sink,
		ors.redemptionKeyFilter,
		ors.guardianFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchObjectionRaised(
	sink chan *abi.RedemptionWatchtowerObjectionRaised,
	redemptionKeyFilter []*big.Int,
	guardianFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchObjectionRaised(
			&bind.WatchOpts{Context: ctx},
			sink,
			redemptionKeyFilter,
			guardianFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event ObjectionRaised had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event ObjectionRaised failed "+
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

func (rw *RedemptionWatchtower) PastObjectionRaisedEvents(
	startBlock uint64,
	endBlock *uint64,
	redemptionKeyFilter []*big.Int,
	guardianFilter []common.Address,
) ([]*abi.RedemptionWatchtowerObjectionRaised, error) {
	iterator, err := rw.contract.FilterObjectionRaised(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		redemptionKeyFilter,
		guardianFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ObjectionRaised events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerObjectionRaised, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) OwnershipTransferredEvent(
	opts *ethereum.SubscribeOpts,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) *RwOwnershipTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwOwnershipTransferredSubscription{
		rw,
		opts,
		previousOwnerFilter,
		newOwnerFilter,
	}
}

type RwOwnershipTransferredSubscription struct {
	contract            *RedemptionWatchtower
	opts                *ethereum.SubscribeOpts
	previousOwnerFilter []common.Address
	newOwnerFilter      []common.Address
}

type redemptionWatchtowerOwnershipTransferredFunc func(
	PreviousOwner common.Address,
	NewOwner common.Address,
	blockNumber uint64,
)

func (ots *RwOwnershipTransferredSubscription) OnEvent(
	handler redemptionWatchtowerOwnershipTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerOwnershipTransferred)
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

func (ots *RwOwnershipTransferredSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerOwnershipTransferred,
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
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ots.opts.PastBlocks

				rwLogger.Infof(
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
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
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

func (rw *RedemptionWatchtower) watchOwnershipTransferred(
	sink chan *abi.RedemptionWatchtowerOwnershipTransferred,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchOwnershipTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
			previousOwnerFilter,
			newOwnerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event OwnershipTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
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

func (rw *RedemptionWatchtower) PastOwnershipTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) ([]*abi.RedemptionWatchtowerOwnershipTransferred, error) {
	iterator, err := rw.contract.FilterOwnershipTransferred(
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

	events := make([]*abi.RedemptionWatchtowerOwnershipTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) UnbannedEvent(
	opts *ethereum.SubscribeOpts,
	redeemerFilter []common.Address,
) *RwUnbannedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwUnbannedSubscription{
		rw,
		opts,
		redeemerFilter,
	}
}

type RwUnbannedSubscription struct {
	contract       *RedemptionWatchtower
	opts           *ethereum.SubscribeOpts
	redeemerFilter []common.Address
}

type redemptionWatchtowerUnbannedFunc func(
	Redeemer common.Address,
	blockNumber uint64,
)

func (us *RwUnbannedSubscription) OnEvent(
	handler redemptionWatchtowerUnbannedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerUnbanned)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Redeemer,
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

func (us *RwUnbannedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerUnbanned,
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
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - us.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past Unbanned events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := us.contract.PastUnbannedEvents(
					fromBlock,
					nil,
					us.redeemerFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past Unbanned events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := us.contract.watchUnbanned(
		sink,
		us.redeemerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchUnbanned(
	sink chan *abi.RedemptionWatchtowerUnbanned,
	redeemerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchUnbanned(
			&bind.WatchOpts{Context: ctx},
			sink,
			redeemerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event Unbanned had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event Unbanned failed "+
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

func (rw *RedemptionWatchtower) PastUnbannedEvents(
	startBlock uint64,
	endBlock *uint64,
	redeemerFilter []common.Address,
) ([]*abi.RedemptionWatchtowerUnbanned, error) {
	iterator, err := rw.contract.FilterUnbanned(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		redeemerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Unbanned events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerUnbanned, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) VetoFinalizedEvent(
	opts *ethereum.SubscribeOpts,
	redemptionKeyFilter []*big.Int,
) *RwVetoFinalizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwVetoFinalizedSubscription{
		rw,
		opts,
		redemptionKeyFilter,
	}
}

type RwVetoFinalizedSubscription struct {
	contract            *RedemptionWatchtower
	opts                *ethereum.SubscribeOpts
	redemptionKeyFilter []*big.Int
}

type redemptionWatchtowerVetoFinalizedFunc func(
	RedemptionKey *big.Int,
	blockNumber uint64,
)

func (vfs *RwVetoFinalizedSubscription) OnEvent(
	handler redemptionWatchtowerVetoFinalizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerVetoFinalized)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RedemptionKey,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := vfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (vfs *RwVetoFinalizedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerVetoFinalized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(vfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := vfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - vfs.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past VetoFinalized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := vfs.contract.PastVetoFinalizedEvents(
					fromBlock,
					nil,
					vfs.redemptionKeyFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past VetoFinalized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := vfs.contract.watchVetoFinalized(
		sink,
		vfs.redemptionKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchVetoFinalized(
	sink chan *abi.RedemptionWatchtowerVetoFinalized,
	redemptionKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchVetoFinalized(
			&bind.WatchOpts{Context: ctx},
			sink,
			redemptionKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event VetoFinalized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event VetoFinalized failed "+
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

func (rw *RedemptionWatchtower) PastVetoFinalizedEvents(
	startBlock uint64,
	endBlock *uint64,
	redemptionKeyFilter []*big.Int,
) ([]*abi.RedemptionWatchtowerVetoFinalized, error) {
	iterator, err := rw.contract.FilterVetoFinalized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		redemptionKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past VetoFinalized events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerVetoFinalized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) VetoPeriodCheckOmittedEvent(
	opts *ethereum.SubscribeOpts,
	redemptionKeyFilter []*big.Int,
) *RwVetoPeriodCheckOmittedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwVetoPeriodCheckOmittedSubscription{
		rw,
		opts,
		redemptionKeyFilter,
	}
}

type RwVetoPeriodCheckOmittedSubscription struct {
	contract            *RedemptionWatchtower
	opts                *ethereum.SubscribeOpts
	redemptionKeyFilter []*big.Int
}

type redemptionWatchtowerVetoPeriodCheckOmittedFunc func(
	RedemptionKey *big.Int,
	blockNumber uint64,
)

func (vpcos *RwVetoPeriodCheckOmittedSubscription) OnEvent(
	handler redemptionWatchtowerVetoPeriodCheckOmittedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerVetoPeriodCheckOmitted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RedemptionKey,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := vpcos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (vpcos *RwVetoPeriodCheckOmittedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerVetoPeriodCheckOmitted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(vpcos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := vpcos.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - vpcos.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past VetoPeriodCheckOmitted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := vpcos.contract.PastVetoPeriodCheckOmittedEvents(
					fromBlock,
					nil,
					vpcos.redemptionKeyFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past VetoPeriodCheckOmitted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := vpcos.contract.watchVetoPeriodCheckOmitted(
		sink,
		vpcos.redemptionKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchVetoPeriodCheckOmitted(
	sink chan *abi.RedemptionWatchtowerVetoPeriodCheckOmitted,
	redemptionKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchVetoPeriodCheckOmitted(
			&bind.WatchOpts{Context: ctx},
			sink,
			redemptionKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event VetoPeriodCheckOmitted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event VetoPeriodCheckOmitted failed "+
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

func (rw *RedemptionWatchtower) PastVetoPeriodCheckOmittedEvents(
	startBlock uint64,
	endBlock *uint64,
	redemptionKeyFilter []*big.Int,
) ([]*abi.RedemptionWatchtowerVetoPeriodCheckOmitted, error) {
	iterator, err := rw.contract.FilterVetoPeriodCheckOmitted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		redemptionKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past VetoPeriodCheckOmitted events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerVetoPeriodCheckOmitted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) VetoedFundsWithdrawnEvent(
	opts *ethereum.SubscribeOpts,
	redemptionKeyFilter []*big.Int,
	redeemerFilter []common.Address,
) *RwVetoedFundsWithdrawnSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwVetoedFundsWithdrawnSubscription{
		rw,
		opts,
		redemptionKeyFilter,
		redeemerFilter,
	}
}

type RwVetoedFundsWithdrawnSubscription struct {
	contract            *RedemptionWatchtower
	opts                *ethereum.SubscribeOpts
	redemptionKeyFilter []*big.Int
	redeemerFilter      []common.Address
}

type redemptionWatchtowerVetoedFundsWithdrawnFunc func(
	RedemptionKey *big.Int,
	Redeemer common.Address,
	Amount uint64,
	blockNumber uint64,
)

func (vfws *RwVetoedFundsWithdrawnSubscription) OnEvent(
	handler redemptionWatchtowerVetoedFundsWithdrawnFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerVetoedFundsWithdrawn)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RedemptionKey,
					event.Redeemer,
					event.Amount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := vfws.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (vfws *RwVetoedFundsWithdrawnSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerVetoedFundsWithdrawn,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(vfws.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := vfws.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - vfws.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past VetoedFundsWithdrawn events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := vfws.contract.PastVetoedFundsWithdrawnEvents(
					fromBlock,
					nil,
					vfws.redemptionKeyFilter,
					vfws.redeemerFilter,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past VetoedFundsWithdrawn events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := vfws.contract.watchVetoedFundsWithdrawn(
		sink,
		vfws.redemptionKeyFilter,
		vfws.redeemerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchVetoedFundsWithdrawn(
	sink chan *abi.RedemptionWatchtowerVetoedFundsWithdrawn,
	redemptionKeyFilter []*big.Int,
	redeemerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchVetoedFundsWithdrawn(
			&bind.WatchOpts{Context: ctx},
			sink,
			redemptionKeyFilter,
			redeemerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event VetoedFundsWithdrawn had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event VetoedFundsWithdrawn failed "+
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

func (rw *RedemptionWatchtower) PastVetoedFundsWithdrawnEvents(
	startBlock uint64,
	endBlock *uint64,
	redemptionKeyFilter []*big.Int,
	redeemerFilter []common.Address,
) ([]*abi.RedemptionWatchtowerVetoedFundsWithdrawn, error) {
	iterator, err := rw.contract.FilterVetoedFundsWithdrawn(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		redemptionKeyFilter,
		redeemerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past VetoedFundsWithdrawn events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerVetoedFundsWithdrawn, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) WatchtowerDisabledEvent(
	opts *ethereum.SubscribeOpts,
) *RwWatchtowerDisabledSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwWatchtowerDisabledSubscription{
		rw,
		opts,
	}
}

type RwWatchtowerDisabledSubscription struct {
	contract *RedemptionWatchtower
	opts     *ethereum.SubscribeOpts
}

type redemptionWatchtowerWatchtowerDisabledFunc func(
	DisabledAt uint32,
	Executor common.Address,
	blockNumber uint64,
)

func (wds *RwWatchtowerDisabledSubscription) OnEvent(
	handler redemptionWatchtowerWatchtowerDisabledFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerWatchtowerDisabled)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.DisabledAt,
					event.Executor,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wds.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wds *RwWatchtowerDisabledSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerWatchtowerDisabled,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wds.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wds.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wds.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past WatchtowerDisabled events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wds.contract.PastWatchtowerDisabledEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past WatchtowerDisabled events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wds.contract.watchWatchtowerDisabled(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchWatchtowerDisabled(
	sink chan *abi.RedemptionWatchtowerWatchtowerDisabled,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchWatchtowerDisabled(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event WatchtowerDisabled had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event WatchtowerDisabled failed "+
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

func (rw *RedemptionWatchtower) PastWatchtowerDisabledEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RedemptionWatchtowerWatchtowerDisabled, error) {
	iterator, err := rw.contract.FilterWatchtowerDisabled(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WatchtowerDisabled events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerWatchtowerDisabled, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) WatchtowerEnabledEvent(
	opts *ethereum.SubscribeOpts,
) *RwWatchtowerEnabledSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwWatchtowerEnabledSubscription{
		rw,
		opts,
	}
}

type RwWatchtowerEnabledSubscription struct {
	contract *RedemptionWatchtower
	opts     *ethereum.SubscribeOpts
}

type redemptionWatchtowerWatchtowerEnabledFunc func(
	EnabledAt uint32,
	Manager common.Address,
	blockNumber uint64,
)

func (wes *RwWatchtowerEnabledSubscription) OnEvent(
	handler redemptionWatchtowerWatchtowerEnabledFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerWatchtowerEnabled)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.EnabledAt,
					event.Manager,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wes.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wes *RwWatchtowerEnabledSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerWatchtowerEnabled,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wes.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wes.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wes.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past WatchtowerEnabled events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wes.contract.PastWatchtowerEnabledEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past WatchtowerEnabled events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wes.contract.watchWatchtowerEnabled(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchWatchtowerEnabled(
	sink chan *abi.RedemptionWatchtowerWatchtowerEnabled,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchWatchtowerEnabled(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event WatchtowerEnabled had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event WatchtowerEnabled failed "+
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

func (rw *RedemptionWatchtower) PastWatchtowerEnabledEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RedemptionWatchtowerWatchtowerEnabled, error) {
	iterator, err := rw.contract.FilterWatchtowerEnabled(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WatchtowerEnabled events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerWatchtowerEnabled, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rw *RedemptionWatchtower) WatchtowerParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RwWatchtowerParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RwWatchtowerParametersUpdatedSubscription{
		rw,
		opts,
	}
}

type RwWatchtowerParametersUpdatedSubscription struct {
	contract *RedemptionWatchtower
	opts     *ethereum.SubscribeOpts
}

type redemptionWatchtowerWatchtowerParametersUpdatedFunc func(
	WatchtowerLifetime uint32,
	VetoPenaltyFeeDivisor uint64,
	VetoFreezePeriod uint32,
	DefaultDelay uint32,
	LevelOneDelay uint32,
	LevelTwoDelay uint32,
	WaivedAmountLimit uint64,
	blockNumber uint64,
)

func (wpus *RwWatchtowerParametersUpdatedSubscription) OnEvent(
	handler redemptionWatchtowerWatchtowerParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.RedemptionWatchtowerWatchtowerParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WatchtowerLifetime,
					event.VetoPenaltyFeeDivisor,
					event.VetoFreezePeriod,
					event.DefaultDelay,
					event.LevelOneDelay,
					event.LevelTwoDelay,
					event.WaivedAmountLimit,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wpus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wpus *RwWatchtowerParametersUpdatedSubscription) Pipe(
	sink chan *abi.RedemptionWatchtowerWatchtowerParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wpus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wpus.contract.blockCounter.CurrentBlock()
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wpus.opts.PastBlocks

				rwLogger.Infof(
					"subscription monitoring fetching past WatchtowerParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wpus.contract.PastWatchtowerParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rwLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rwLogger.Infof(
					"subscription monitoring fetched [%v] past WatchtowerParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wpus.contract.watchWatchtowerParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rw *RedemptionWatchtower) watchWatchtowerParametersUpdated(
	sink chan *abi.RedemptionWatchtowerWatchtowerParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rw.contract.WatchWatchtowerParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rwLogger.Warnf(
			"subscription to event WatchtowerParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rwLogger.Errorf(
			"subscription to event WatchtowerParametersUpdated failed "+
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

func (rw *RedemptionWatchtower) PastWatchtowerParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.RedemptionWatchtowerWatchtowerParametersUpdated, error) {
	iterator, err := rw.contract.FilterWatchtowerParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WatchtowerParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.RedemptionWatchtowerWatchtowerParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

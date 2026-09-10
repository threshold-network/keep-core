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
var lrLogger = log.Logger("keep-contract-LightRelay")

type LightRelay struct {
	contract          *abi.LightRelay
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

func NewLightRelay(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*LightRelay, error) {
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

	contract, err := abi.NewLightRelay(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.LightRelayABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &LightRelay{
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
func (lr *LightRelay) Authorize(
	arg_submitter common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction authorize",
		" params: ",
		fmt.Sprint(
			arg_submitter,
		),
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.Authorize(
		transactorOptions,
		arg_submitter,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"authorize",
			arg_submitter,
		)
	}

	lrLogger.Infof(
		"submitted transaction authorize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.Authorize(
				newTransactorOptions,
				arg_submitter,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"authorize",
					arg_submitter,
				)
			}

			lrLogger.Infof(
				"submitted transaction authorize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallAuthorize(
	arg_submitter common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"authorize",
		&result,
		arg_submitter,
	)

	return err
}

func (lr *LightRelay) AuthorizeGasEstimate(
	arg_submitter common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"authorize",
		lr.contractABI,
		lr.transactor,
		arg_submitter,
	)

	return result, err
}

// Transaction submission.
func (lr *LightRelay) Deauthorize(
	arg_submitter common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction deauthorize",
		" params: ",
		fmt.Sprint(
			arg_submitter,
		),
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.Deauthorize(
		transactorOptions,
		arg_submitter,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"deauthorize",
			arg_submitter,
		)
	}

	lrLogger.Infof(
		"submitted transaction deauthorize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.Deauthorize(
				newTransactorOptions,
				arg_submitter,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"deauthorize",
					arg_submitter,
				)
			}

			lrLogger.Infof(
				"submitted transaction deauthorize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallDeauthorize(
	arg_submitter common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"deauthorize",
		&result,
		arg_submitter,
	)

	return err
}

func (lr *LightRelay) DeauthorizeGasEstimate(
	arg_submitter common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"deauthorize",
		lr.contractABI,
		lr.transactor,
		arg_submitter,
	)

	return result, err
}

// Transaction submission.
func (lr *LightRelay) Genesis(
	arg_genesisHeader []byte,
	arg_genesisHeight *big.Int,
	arg_genesisProofLength uint64,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction genesis",
		" params: ",
		fmt.Sprint(
			arg_genesisHeader,
			arg_genesisHeight,
			arg_genesisProofLength,
		),
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.Genesis(
		transactorOptions,
		arg_genesisHeader,
		arg_genesisHeight,
		arg_genesisProofLength,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"genesis",
			arg_genesisHeader,
			arg_genesisHeight,
			arg_genesisProofLength,
		)
	}

	lrLogger.Infof(
		"submitted transaction genesis with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.Genesis(
				newTransactorOptions,
				arg_genesisHeader,
				arg_genesisHeight,
				arg_genesisProofLength,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"genesis",
					arg_genesisHeader,
					arg_genesisHeight,
					arg_genesisProofLength,
				)
			}

			lrLogger.Infof(
				"submitted transaction genesis with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallGenesis(
	arg_genesisHeader []byte,
	arg_genesisHeight *big.Int,
	arg_genesisProofLength uint64,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"genesis",
		&result,
		arg_genesisHeader,
		arg_genesisHeight,
		arg_genesisProofLength,
	)

	return err
}

func (lr *LightRelay) GenesisGasEstimate(
	arg_genesisHeader []byte,
	arg_genesisHeight *big.Int,
	arg_genesisProofLength uint64,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"genesis",
		lr.contractABI,
		lr.transactor,
		arg_genesisHeader,
		arg_genesisHeight,
		arg_genesisProofLength,
	)

	return result, err
}

// Transaction submission.
func (lr *LightRelay) RenounceOwnership(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction renounceOwnership",
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.RenounceOwnership(
		transactorOptions,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"renounceOwnership",
		)
	}

	lrLogger.Infof(
		"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.RenounceOwnership(
				newTransactorOptions,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"renounceOwnership",
				)
			}

			lrLogger.Infof(
				"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallRenounceOwnership(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"renounceOwnership",
		&result,
	)

	return err
}

func (lr *LightRelay) RenounceOwnershipGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"renounceOwnership",
		lr.contractABI,
		lr.transactor,
	)

	return result, err
}

// Transaction submission.
func (lr *LightRelay) Retarget(
	arg_headers []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction retarget",
		" params: ",
		fmt.Sprint(
			arg_headers,
		),
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.Retarget(
		transactorOptions,
		arg_headers,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"retarget",
			arg_headers,
		)
	}

	lrLogger.Infof(
		"submitted transaction retarget with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.Retarget(
				newTransactorOptions,
				arg_headers,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"retarget",
					arg_headers,
				)
			}

			lrLogger.Infof(
				"submitted transaction retarget with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallRetarget(
	arg_headers []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"retarget",
		&result,
		arg_headers,
	)

	return err
}

func (lr *LightRelay) RetargetGasEstimate(
	arg_headers []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"retarget",
		lr.contractABI,
		lr.transactor,
		arg_headers,
	)

	return result, err
}

// Transaction submission.
func (lr *LightRelay) SetAuthorizationStatus(
	arg_status bool,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction setAuthorizationStatus",
		" params: ",
		fmt.Sprint(
			arg_status,
		),
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.SetAuthorizationStatus(
		transactorOptions,
		arg_status,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"setAuthorizationStatus",
			arg_status,
		)
	}

	lrLogger.Infof(
		"submitted transaction setAuthorizationStatus with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.SetAuthorizationStatus(
				newTransactorOptions,
				arg_status,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"setAuthorizationStatus",
					arg_status,
				)
			}

			lrLogger.Infof(
				"submitted transaction setAuthorizationStatus with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallSetAuthorizationStatus(
	arg_status bool,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"setAuthorizationStatus",
		&result,
		arg_status,
	)

	return err
}

func (lr *LightRelay) SetAuthorizationStatusGasEstimate(
	arg_status bool,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"setAuthorizationStatus",
		lr.contractABI,
		lr.transactor,
		arg_status,
	)

	return result, err
}

// Transaction submission.
func (lr *LightRelay) SetProofLength(
	arg_newLength uint64,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction setProofLength",
		" params: ",
		fmt.Sprint(
			arg_newLength,
		),
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.SetProofLength(
		transactorOptions,
		arg_newLength,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"setProofLength",
			arg_newLength,
		)
	}

	lrLogger.Infof(
		"submitted transaction setProofLength with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.SetProofLength(
				newTransactorOptions,
				arg_newLength,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"setProofLength",
					arg_newLength,
				)
			}

			lrLogger.Infof(
				"submitted transaction setProofLength with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallSetProofLength(
	arg_newLength uint64,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"setProofLength",
		&result,
		arg_newLength,
	)

	return err
}

func (lr *LightRelay) SetProofLengthGasEstimate(
	arg_newLength uint64,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"setProofLength",
		lr.contractABI,
		lr.transactor,
		arg_newLength,
	)

	return result, err
}

// Transaction submission.
func (lr *LightRelay) TransferOwnership(
	arg_newOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	lrLogger.Debug(
		"submitting transaction transferOwnership",
		" params: ",
		fmt.Sprint(
			arg_newOwner,
		),
	)

	lr.transactionMutex.Lock()
	defer lr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *lr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := lr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := lr.contract.TransferOwnership(
		transactorOptions,
		arg_newOwner,
	)
	if err != nil {
		return transaction, lr.errorResolver.ResolveError(
			err,
			lr.transactorOptions.From,
			nil,
			"transferOwnership",
			arg_newOwner,
		)
	}

	lrLogger.Infof(
		"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go lr.miningWaiter.ForceMining(
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

			transaction, err := lr.contract.TransferOwnership(
				newTransactorOptions,
				arg_newOwner,
			)
			if err != nil {
				return nil, lr.errorResolver.ResolveError(
					err,
					lr.transactorOptions.From,
					nil,
					"transferOwnership",
					arg_newOwner,
				)
			}

			lrLogger.Infof(
				"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	lr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (lr *LightRelay) CallTransferOwnership(
	arg_newOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		lr.transactorOptions.From,
		blockNumber, nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"transferOwnership",
		&result,
		arg_newOwner,
	)

	return err
}

func (lr *LightRelay) TransferOwnershipGasEstimate(
	arg_newOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		lr.callerOptions.From,
		lr.contractAddress,
		"transferOwnership",
		lr.contractABI,
		lr.transactor,
		arg_newOwner,
	)

	return result, err
}

// ----- Const Methods ------

func (lr *LightRelay) AuthorizationRequired() (bool, error) {
	result, err := lr.contract.AuthorizationRequired(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"authorizationRequired",
		)
	}

	return result, err
}

func (lr *LightRelay) AuthorizationRequiredAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"authorizationRequired",
		&result,
	)

	return result, err
}

func (lr *LightRelay) CurrentEpoch() (uint64, error) {
	result, err := lr.contract.CurrentEpoch(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"currentEpoch",
		)
	}

	return result, err
}

func (lr *LightRelay) CurrentEpochAtBlock(
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"currentEpoch",
		&result,
	)

	return result, err
}

func (lr *LightRelay) GenesisEpoch() (uint64, error) {
	result, err := lr.contract.GenesisEpoch(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"genesisEpoch",
		)
	}

	return result, err
}

func (lr *LightRelay) GenesisEpochAtBlock(
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"genesisEpoch",
		&result,
	)

	return result, err
}

func (lr *LightRelay) GetBlockDifficulty(
	arg_blockNumber *big.Int,
) (*big.Int, error) {
	result, err := lr.contract.GetBlockDifficulty(
		lr.callerOptions,
		arg_blockNumber,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"getBlockDifficulty",
			arg_blockNumber,
		)
	}

	return result, err
}

func (lr *LightRelay) GetBlockDifficultyAtBlock(
	arg_blockNumber *big.Int,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"getBlockDifficulty",
		&result,
		arg_blockNumber,
	)

	return result, err
}

type CurrentAndPrevEpochDifficulty struct {
	Current  *big.Int
	Previous *big.Int
}

func (lr *LightRelay) GetCurrentAndPrevEpochDifficulty() (CurrentAndPrevEpochDifficulty, error) {
	result, err := lr.contract.GetCurrentAndPrevEpochDifficulty(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"getCurrentAndPrevEpochDifficulty",
		)
	}

	return result, err
}

func (lr *LightRelay) GetCurrentAndPrevEpochDifficultyAtBlock(
	blockNumber *big.Int,
) (CurrentAndPrevEpochDifficulty, error) {
	var result CurrentAndPrevEpochDifficulty

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"getCurrentAndPrevEpochDifficulty",
		&result,
	)

	return result, err
}

func (lr *LightRelay) GetCurrentEpochDifficulty() (*big.Int, error) {
	result, err := lr.contract.GetCurrentEpochDifficulty(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"getCurrentEpochDifficulty",
		)
	}

	return result, err
}

func (lr *LightRelay) GetCurrentEpochDifficultyAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"getCurrentEpochDifficulty",
		&result,
	)

	return result, err
}

func (lr *LightRelay) GetEpochDifficulty(
	arg_epochNumber *big.Int,
) (*big.Int, error) {
	result, err := lr.contract.GetEpochDifficulty(
		lr.callerOptions,
		arg_epochNumber,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"getEpochDifficulty",
			arg_epochNumber,
		)
	}

	return result, err
}

func (lr *LightRelay) GetEpochDifficultyAtBlock(
	arg_epochNumber *big.Int,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"getEpochDifficulty",
		&result,
		arg_epochNumber,
	)

	return result, err
}

func (lr *LightRelay) GetPrevEpochDifficulty() (*big.Int, error) {
	result, err := lr.contract.GetPrevEpochDifficulty(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"getPrevEpochDifficulty",
		)
	}

	return result, err
}

func (lr *LightRelay) GetPrevEpochDifficultyAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"getPrevEpochDifficulty",
		&result,
	)

	return result, err
}

type RelayRange struct {
	RelayGenesis    *big.Int
	CurrentEpochEnd *big.Int
}

func (lr *LightRelay) GetRelayRange() (RelayRange, error) {
	result, err := lr.contract.GetRelayRange(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"getRelayRange",
		)
	}

	return result, err
}

func (lr *LightRelay) GetRelayRangeAtBlock(
	blockNumber *big.Int,
) (RelayRange, error) {
	var result RelayRange

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"getRelayRange",
		&result,
	)

	return result, err
}

func (lr *LightRelay) IsAuthorized(
	arg0 common.Address,
) (bool, error) {
	result, err := lr.contract.IsAuthorized(
		lr.callerOptions,
		arg0,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"isAuthorized",
			arg0,
		)
	}

	return result, err
}

func (lr *LightRelay) IsAuthorizedAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"isAuthorized",
		&result,
		arg0,
	)

	return result, err
}

func (lr *LightRelay) Owner() (common.Address, error) {
	result, err := lr.contract.Owner(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"owner",
		)
	}

	return result, err
}

func (lr *LightRelay) OwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"owner",
		&result,
	)

	return result, err
}

func (lr *LightRelay) ProofLength() (uint64, error) {
	result, err := lr.contract.ProofLength(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"proofLength",
		)
	}

	return result, err
}

func (lr *LightRelay) ProofLengthAtBlock(
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"proofLength",
		&result,
	)

	return result, err
}

func (lr *LightRelay) Ready() (bool, error) {
	result, err := lr.contract.Ready(
		lr.callerOptions,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"ready",
		)
	}

	return result, err
}

func (lr *LightRelay) ReadyAtBlock(
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"ready",
		&result,
	)

	return result, err
}

type validateChain struct {
	StartingHeaderTimestamp *big.Int
	HeaderCount             *big.Int
}

func (lr *LightRelay) ValidateChain(
	arg_headers []byte,
) (validateChain, error) {
	result, err := lr.contract.ValidateChain(
		lr.callerOptions,
		arg_headers,
	)

	if err != nil {
		return result, lr.errorResolver.ResolveError(
			err,
			lr.callerOptions.From,
			nil,
			"validateChain",
			arg_headers,
		)
	}

	return result, err
}

func (lr *LightRelay) ValidateChainAtBlock(
	arg_headers []byte,
	blockNumber *big.Int,
) (validateChain, error) {
	var result validateChain

	err := chainutil.CallAtBlock(
		lr.callerOptions.From,
		blockNumber,
		nil,
		lr.contractABI,
		lr.caller,
		lr.errorResolver,
		lr.contractAddress,
		"validateChain",
		&result,
		arg_headers,
	)

	return result, err
}

// ------ Events -------

func (lr *LightRelay) AuthorizationRequirementChangedEvent(
	opts *ethereum.SubscribeOpts,
) *LrAuthorizationRequirementChangedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrAuthorizationRequirementChangedSubscription{
		lr,
		opts,
	}
}

type LrAuthorizationRequirementChangedSubscription struct {
	contract *LightRelay
	opts     *ethereum.SubscribeOpts
}

type lightRelayAuthorizationRequirementChangedFunc func(
	NewStatus bool,
	blockNumber uint64,
)

func (arcs *LrAuthorizationRequirementChangedSubscription) OnEvent(
	handler lightRelayAuthorizationRequirementChangedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayAuthorizationRequirementChanged)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.NewStatus,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := arcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (arcs *LrAuthorizationRequirementChangedSubscription) Pipe(
	sink chan *abi.LightRelayAuthorizationRequirementChanged,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(arcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := arcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - arcs.opts.PastBlocks

				lrLogger.Infof(
					"subscription monitoring fetching past AuthorizationRequirementChanged events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := arcs.contract.PastAuthorizationRequirementChangedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrLogger.Infof(
					"subscription monitoring fetched [%v] past AuthorizationRequirementChanged events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := arcs.contract.watchAuthorizationRequirementChanged(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lr *LightRelay) watchAuthorizationRequirementChanged(
	sink chan *abi.LightRelayAuthorizationRequirementChanged,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lr.contract.WatchAuthorizationRequirementChanged(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrLogger.Warnf(
			"subscription to event AuthorizationRequirementChanged had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrLogger.Errorf(
			"subscription to event AuthorizationRequirementChanged failed "+
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

func (lr *LightRelay) PastAuthorizationRequirementChangedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelayAuthorizationRequirementChanged, error) {
	iterator, err := lr.contract.FilterAuthorizationRequirementChanged(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past AuthorizationRequirementChanged events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayAuthorizationRequirementChanged, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lr *LightRelay) GenesisEvent(
	opts *ethereum.SubscribeOpts,
) *LrGenesisSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrGenesisSubscription{
		lr,
		opts,
	}
}

type LrGenesisSubscription struct {
	contract *LightRelay
	opts     *ethereum.SubscribeOpts
}

type lightRelayGenesisFunc func(
	BlockHeight *big.Int,
	blockNumber uint64,
)

func (gs *LrGenesisSubscription) OnEvent(
	handler lightRelayGenesisFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayGenesis)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.BlockHeight,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := gs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (gs *LrGenesisSubscription) Pipe(
	sink chan *abi.LightRelayGenesis,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(gs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := gs.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gs.opts.PastBlocks

				lrLogger.Infof(
					"subscription monitoring fetching past Genesis events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gs.contract.PastGenesisEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrLogger.Infof(
					"subscription monitoring fetched [%v] past Genesis events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := gs.contract.watchGenesis(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lr *LightRelay) watchGenesis(
	sink chan *abi.LightRelayGenesis,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lr.contract.WatchGenesis(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrLogger.Warnf(
			"subscription to event Genesis had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrLogger.Errorf(
			"subscription to event Genesis failed "+
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

func (lr *LightRelay) PastGenesisEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelayGenesis, error) {
	iterator, err := lr.contract.FilterGenesis(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Genesis events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayGenesis, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lr *LightRelay) OwnershipTransferredEvent(
	opts *ethereum.SubscribeOpts,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) *LrOwnershipTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrOwnershipTransferredSubscription{
		lr,
		opts,
		previousOwnerFilter,
		newOwnerFilter,
	}
}

type LrOwnershipTransferredSubscription struct {
	contract            *LightRelay
	opts                *ethereum.SubscribeOpts
	previousOwnerFilter []common.Address
	newOwnerFilter      []common.Address
}

type lightRelayOwnershipTransferredFunc func(
	PreviousOwner common.Address,
	NewOwner common.Address,
	blockNumber uint64,
)

func (ots *LrOwnershipTransferredSubscription) OnEvent(
	handler lightRelayOwnershipTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayOwnershipTransferred)
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

func (ots *LrOwnershipTransferredSubscription) Pipe(
	sink chan *abi.LightRelayOwnershipTransferred,
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
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ots.opts.PastBlocks

				lrLogger.Infof(
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
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrLogger.Infof(
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

func (lr *LightRelay) watchOwnershipTransferred(
	sink chan *abi.LightRelayOwnershipTransferred,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lr.contract.WatchOwnershipTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
			previousOwnerFilter,
			newOwnerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrLogger.Warnf(
			"subscription to event OwnershipTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrLogger.Errorf(
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

func (lr *LightRelay) PastOwnershipTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) ([]*abi.LightRelayOwnershipTransferred, error) {
	iterator, err := lr.contract.FilterOwnershipTransferred(
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

	events := make([]*abi.LightRelayOwnershipTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lr *LightRelay) ProofLengthChangedEvent(
	opts *ethereum.SubscribeOpts,
) *LrProofLengthChangedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrProofLengthChangedSubscription{
		lr,
		opts,
	}
}

type LrProofLengthChangedSubscription struct {
	contract *LightRelay
	opts     *ethereum.SubscribeOpts
}

type lightRelayProofLengthChangedFunc func(
	NewLength *big.Int,
	blockNumber uint64,
)

func (plcs *LrProofLengthChangedSubscription) OnEvent(
	handler lightRelayProofLengthChangedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayProofLengthChanged)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.NewLength,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := plcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (plcs *LrProofLengthChangedSubscription) Pipe(
	sink chan *abi.LightRelayProofLengthChanged,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(plcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := plcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - plcs.opts.PastBlocks

				lrLogger.Infof(
					"subscription monitoring fetching past ProofLengthChanged events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := plcs.contract.PastProofLengthChangedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrLogger.Infof(
					"subscription monitoring fetched [%v] past ProofLengthChanged events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := plcs.contract.watchProofLengthChanged(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lr *LightRelay) watchProofLengthChanged(
	sink chan *abi.LightRelayProofLengthChanged,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lr.contract.WatchProofLengthChanged(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrLogger.Warnf(
			"subscription to event ProofLengthChanged had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrLogger.Errorf(
			"subscription to event ProofLengthChanged failed "+
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

func (lr *LightRelay) PastProofLengthChangedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelayProofLengthChanged, error) {
	iterator, err := lr.contract.FilterProofLengthChanged(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ProofLengthChanged events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayProofLengthChanged, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lr *LightRelay) RetargetEvent(
	opts *ethereum.SubscribeOpts,
) *LrRetargetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrRetargetSubscription{
		lr,
		opts,
	}
}

type LrRetargetSubscription struct {
	contract *LightRelay
	opts     *ethereum.SubscribeOpts
}

type lightRelayRetargetFunc func(
	OldDifficulty *big.Int,
	NewDifficulty *big.Int,
	blockNumber uint64,
)

func (rs *LrRetargetSubscription) OnEvent(
	handler lightRelayRetargetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelayRetarget)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.OldDifficulty,
					event.NewDifficulty,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rs *LrRetargetSubscription) Pipe(
	sink chan *abi.LightRelayRetarget,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rs.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rs.opts.PastBlocks

				lrLogger.Infof(
					"subscription monitoring fetching past Retarget events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rs.contract.PastRetargetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrLogger.Infof(
					"subscription monitoring fetched [%v] past Retarget events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rs.contract.watchRetarget(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lr *LightRelay) watchRetarget(
	sink chan *abi.LightRelayRetarget,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lr.contract.WatchRetarget(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrLogger.Warnf(
			"subscription to event Retarget had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrLogger.Errorf(
			"subscription to event Retarget failed "+
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

func (lr *LightRelay) PastRetargetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelayRetarget, error) {
	iterator, err := lr.contract.FilterRetarget(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past Retarget events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelayRetarget, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lr *LightRelay) SubmitterAuthorizedEvent(
	opts *ethereum.SubscribeOpts,
) *LrSubmitterAuthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrSubmitterAuthorizedSubscription{
		lr,
		opts,
	}
}

type LrSubmitterAuthorizedSubscription struct {
	contract *LightRelay
	opts     *ethereum.SubscribeOpts
}

type lightRelaySubmitterAuthorizedFunc func(
	Submitter common.Address,
	blockNumber uint64,
)

func (sas *LrSubmitterAuthorizedSubscription) OnEvent(
	handler lightRelaySubmitterAuthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelaySubmitterAuthorized)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Submitter,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := sas.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (sas *LrSubmitterAuthorizedSubscription) Pipe(
	sink chan *abi.LightRelaySubmitterAuthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(sas.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := sas.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - sas.opts.PastBlocks

				lrLogger.Infof(
					"subscription monitoring fetching past SubmitterAuthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := sas.contract.PastSubmitterAuthorizedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrLogger.Infof(
					"subscription monitoring fetched [%v] past SubmitterAuthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := sas.contract.watchSubmitterAuthorized(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lr *LightRelay) watchSubmitterAuthorized(
	sink chan *abi.LightRelaySubmitterAuthorized,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lr.contract.WatchSubmitterAuthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrLogger.Warnf(
			"subscription to event SubmitterAuthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrLogger.Errorf(
			"subscription to event SubmitterAuthorized failed "+
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

func (lr *LightRelay) PastSubmitterAuthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelaySubmitterAuthorized, error) {
	iterator, err := lr.contract.FilterSubmitterAuthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past SubmitterAuthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelaySubmitterAuthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (lr *LightRelay) SubmitterDeauthorizedEvent(
	opts *ethereum.SubscribeOpts,
) *LrSubmitterDeauthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &LrSubmitterDeauthorizedSubscription{
		lr,
		opts,
	}
}

type LrSubmitterDeauthorizedSubscription struct {
	contract *LightRelay
	opts     *ethereum.SubscribeOpts
}

type lightRelaySubmitterDeauthorizedFunc func(
	Submitter common.Address,
	blockNumber uint64,
)

func (sds *LrSubmitterDeauthorizedSubscription) OnEvent(
	handler lightRelaySubmitterDeauthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.LightRelaySubmitterDeauthorized)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Submitter,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := sds.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (sds *LrSubmitterDeauthorizedSubscription) Pipe(
	sink chan *abi.LightRelaySubmitterDeauthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(sds.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := sds.contract.blockCounter.CurrentBlock()
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - sds.opts.PastBlocks

				lrLogger.Infof(
					"subscription monitoring fetching past SubmitterDeauthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := sds.contract.PastSubmitterDeauthorizedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					lrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				lrLogger.Infof(
					"subscription monitoring fetched [%v] past SubmitterDeauthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := sds.contract.watchSubmitterDeauthorized(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (lr *LightRelay) watchSubmitterDeauthorized(
	sink chan *abi.LightRelaySubmitterDeauthorized,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return lr.contract.WatchSubmitterDeauthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		lrLogger.Warnf(
			"subscription to event SubmitterDeauthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		lrLogger.Errorf(
			"subscription to event SubmitterDeauthorized failed "+
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

func (lr *LightRelay) PastSubmitterDeauthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.LightRelaySubmitterDeauthorized, error) {
	iterator, err := lr.contract.FilterSubmitterDeauthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past SubmitterDeauthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.LightRelaySubmitterDeauthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

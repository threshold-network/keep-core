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
var rrLogger = log.Logger("keep-contract-ReservationRouter")

type ReservationRouter struct {
	contract          *abi.ReservationRouter
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

func NewReservationRouter(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*ReservationRouter, error) {
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

	contract, err := abi.NewReservationRouter(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.ReservationRouterABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &ReservationRouter{
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
func (rr *ReservationRouter) NotifyReservationActionTimeout(
	arg_reservationKey *big.Int,
	arg_walletMembersIDs []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction notifyReservationActionTimeout",
		" params: ",
		fmt.Sprint(
			arg_reservationKey,
			arg_walletMembersIDs,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.NotifyReservationActionTimeout(
		transactorOptions,
		arg_reservationKey,
		arg_walletMembersIDs,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"notifyReservationActionTimeout",
			arg_reservationKey,
			arg_walletMembersIDs,
		)
	}

	rrLogger.Infof(
		"submitted transaction notifyReservationActionTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.NotifyReservationActionTimeout(
				newTransactorOptions,
				arg_reservationKey,
				arg_walletMembersIDs,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"notifyReservationActionTimeout",
					arg_reservationKey,
					arg_walletMembersIDs,
				)
			}

			rrLogger.Infof(
				"submitted transaction notifyReservationActionTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallNotifyReservationActionTimeout(
	arg_reservationKey *big.Int,
	arg_walletMembersIDs []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"notifyReservationActionTimeout",
		&result,
		arg_reservationKey,
		arg_walletMembersIDs,
	)

	return err
}

func (rr *ReservationRouter) NotifyReservationActionTimeoutGasEstimate(
	arg_reservationKey *big.Int,
	arg_walletMembersIDs []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"notifyReservationActionTimeout",
		rr.contractABI,
		rr.transactor,
		arg_reservationKey,
		arg_walletMembersIDs,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) NotifyReservationStranded(
	arg_reservationKey *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction notifyReservationStranded",
		" params: ",
		fmt.Sprint(
			arg_reservationKey,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.NotifyReservationStranded(
		transactorOptions,
		arg_reservationKey,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"notifyReservationStranded",
			arg_reservationKey,
		)
	}

	rrLogger.Infof(
		"submitted transaction notifyReservationStranded with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.NotifyReservationStranded(
				newTransactorOptions,
				arg_reservationKey,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"notifyReservationStranded",
					arg_reservationKey,
				)
			}

			rrLogger.Infof(
				"submitted transaction notifyReservationStranded with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallNotifyReservationStranded(
	arg_reservationKey *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"notifyReservationStranded",
		&result,
		arg_reservationKey,
	)

	return err
}

func (rr *ReservationRouter) NotifyReservationStrandedGasEstimate(
	arg_reservationKey *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"notifyReservationStranded",
		rr.contractABI,
		rr.transactor,
		arg_reservationKey,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) NotifyStaleReservedDeposit(
	arg_depositKey *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction notifyStaleReservedDeposit",
		" params: ",
		fmt.Sprint(
			arg_depositKey,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.NotifyStaleReservedDeposit(
		transactorOptions,
		arg_depositKey,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"notifyStaleReservedDeposit",
			arg_depositKey,
		)
	}

	rrLogger.Infof(
		"submitted transaction notifyStaleReservedDeposit with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.NotifyStaleReservedDeposit(
				newTransactorOptions,
				arg_depositKey,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"notifyStaleReservedDeposit",
					arg_depositKey,
				)
			}

			rrLogger.Infof(
				"submitted transaction notifyStaleReservedDeposit with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallNotifyStaleReservedDeposit(
	arg_depositKey *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"notifyStaleReservedDeposit",
		&result,
		arg_depositKey,
	)

	return err
}

func (rr *ReservationRouter) NotifyStaleReservedDepositGasEstimate(
	arg_depositKey *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"notifyStaleReservedDeposit",
		rr.contractABI,
		rr.transactor,
		arg_depositKey,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) RequestReservationAcceptance(
	arg_reservationKey *big.Int,
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction requestReservationAcceptance",
		" params: ",
		fmt.Sprint(
			arg_reservationKey,
			arg_walletPubKeyHash,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.RequestReservationAcceptance(
		transactorOptions,
		arg_reservationKey,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"requestReservationAcceptance",
			arg_reservationKey,
			arg_walletPubKeyHash,
		)
	}

	rrLogger.Infof(
		"submitted transaction requestReservationAcceptance with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.RequestReservationAcceptance(
				newTransactorOptions,
				arg_reservationKey,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"requestReservationAcceptance",
					arg_reservationKey,
					arg_walletPubKeyHash,
				)
			}

			rrLogger.Infof(
				"submitted transaction requestReservationAcceptance with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallRequestReservationAcceptance(
	arg_reservationKey *big.Int,
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"requestReservationAcceptance",
		&result,
		arg_reservationKey,
		arg_walletPubKeyHash,
	)

	return err
}

func (rr *ReservationRouter) RequestReservationAcceptanceGasEstimate(
	arg_reservationKey *big.Int,
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"requestReservationAcceptance",
		rr.contractABI,
		rr.transactor,
		arg_reservationKey,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) RequestReservationReanchor(
	arg_reservationKey *big.Int,
	arg_targetWalletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction requestReservationReanchor",
		" params: ",
		fmt.Sprint(
			arg_reservationKey,
			arg_targetWalletPubKeyHash,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.RequestReservationReanchor(
		transactorOptions,
		arg_reservationKey,
		arg_targetWalletPubKeyHash,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"requestReservationReanchor",
			arg_reservationKey,
			arg_targetWalletPubKeyHash,
		)
	}

	rrLogger.Infof(
		"submitted transaction requestReservationReanchor with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.RequestReservationReanchor(
				newTransactorOptions,
				arg_reservationKey,
				arg_targetWalletPubKeyHash,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"requestReservationReanchor",
					arg_reservationKey,
					arg_targetWalletPubKeyHash,
				)
			}

			rrLogger.Infof(
				"submitted transaction requestReservationReanchor with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallRequestReservationReanchor(
	arg_reservationKey *big.Int,
	arg_targetWalletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"requestReservationReanchor",
		&result,
		arg_reservationKey,
		arg_targetWalletPubKeyHash,
	)

	return err
}

func (rr *ReservationRouter) RequestReservationReanchorGasEstimate(
	arg_reservationKey *big.Int,
	arg_targetWalletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"requestReservationReanchor",
		rr.contractABI,
		rr.transactor,
		arg_reservationKey,
		arg_targetWalletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) SubmitReservationProof(
	arg_proofType uint8,
	arg_txInfo abi.BitcoinTxInfo4,
	arg_proof abi.BitcoinTxProof3,
	arg_mainUtxo abi.BitcoinTxUTXO4,
	arg_reservationKey *big.Int,
	arg_requestNonce uint64,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction submitReservationProof",
		" params: ",
		fmt.Sprint(
			arg_proofType,
			arg_txInfo,
			arg_proof,
			arg_mainUtxo,
			arg_reservationKey,
			arg_requestNonce,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.SubmitReservationProof(
		transactorOptions,
		arg_proofType,
		arg_txInfo,
		arg_proof,
		arg_mainUtxo,
		arg_reservationKey,
		arg_requestNonce,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"submitReservationProof",
			arg_proofType,
			arg_txInfo,
			arg_proof,
			arg_mainUtxo,
			arg_reservationKey,
			arg_requestNonce,
		)
	}

	rrLogger.Infof(
		"submitted transaction submitReservationProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.SubmitReservationProof(
				newTransactorOptions,
				arg_proofType,
				arg_txInfo,
				arg_proof,
				arg_mainUtxo,
				arg_reservationKey,
				arg_requestNonce,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"submitReservationProof",
					arg_proofType,
					arg_txInfo,
					arg_proof,
					arg_mainUtxo,
					arg_reservationKey,
					arg_requestNonce,
				)
			}

			rrLogger.Infof(
				"submitted transaction submitReservationProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallSubmitReservationProof(
	arg_proofType uint8,
	arg_txInfo abi.BitcoinTxInfo4,
	arg_proof abi.BitcoinTxProof3,
	arg_mainUtxo abi.BitcoinTxUTXO4,
	arg_reservationKey *big.Int,
	arg_requestNonce uint64,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"submitReservationProof",
		&result,
		arg_proofType,
		arg_txInfo,
		arg_proof,
		arg_mainUtxo,
		arg_reservationKey,
		arg_requestNonce,
	)

	return err
}

func (rr *ReservationRouter) SubmitReservationProofGasEstimate(
	arg_proofType uint8,
	arg_txInfo abi.BitcoinTxInfo4,
	arg_proof abi.BitcoinTxProof3,
	arg_mainUtxo abi.BitcoinTxUTXO4,
	arg_reservationKey *big.Int,
	arg_requestNonce uint64,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"submitReservationProof",
		rr.contractABI,
		rr.transactor,
		arg_proofType,
		arg_txInfo,
		arg_proof,
		arg_mainUtxo,
		arg_reservationKey,
		arg_requestNonce,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) TransferGovernance(
	arg_newGovernance common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction transferGovernance",
		" params: ",
		fmt.Sprint(
			arg_newGovernance,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.TransferGovernance(
		transactorOptions,
		arg_newGovernance,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"transferGovernance",
			arg_newGovernance,
		)
	}

	rrLogger.Infof(
		"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.TransferGovernance(
				newTransactorOptions,
				arg_newGovernance,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"transferGovernance",
					arg_newGovernance,
				)
			}

			rrLogger.Infof(
				"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallTransferGovernance(
	arg_newGovernance common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"transferGovernance",
		&result,
		arg_newGovernance,
	)

	return err
}

func (rr *ReservationRouter) TransferGovernanceGasEstimate(
	arg_newGovernance common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"transferGovernance",
		rr.contractABI,
		rr.transactor,
		arg_newGovernance,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) UpdateReservationCaps(
	arg_maxReservationsAmountPerWallet uint64,
	arg_reservationMaxSingleAmount uint64,
	arg_maxActiveReservations uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction updateReservationCaps",
		" params: ",
		fmt.Sprint(
			arg_maxReservationsAmountPerWallet,
			arg_reservationMaxSingleAmount,
			arg_maxActiveReservations,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.UpdateReservationCaps(
		transactorOptions,
		arg_maxReservationsAmountPerWallet,
		arg_reservationMaxSingleAmount,
		arg_maxActiveReservations,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"updateReservationCaps",
			arg_maxReservationsAmountPerWallet,
			arg_reservationMaxSingleAmount,
			arg_maxActiveReservations,
		)
	}

	rrLogger.Infof(
		"submitted transaction updateReservationCaps with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.UpdateReservationCaps(
				newTransactorOptions,
				arg_maxReservationsAmountPerWallet,
				arg_reservationMaxSingleAmount,
				arg_maxActiveReservations,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"updateReservationCaps",
					arg_maxReservationsAmountPerWallet,
					arg_reservationMaxSingleAmount,
					arg_maxActiveReservations,
				)
			}

			rrLogger.Infof(
				"submitted transaction updateReservationCaps with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallUpdateReservationCaps(
	arg_maxReservationsAmountPerWallet uint64,
	arg_reservationMaxSingleAmount uint64,
	arg_maxActiveReservations uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"updateReservationCaps",
		&result,
		arg_maxReservationsAmountPerWallet,
		arg_reservationMaxSingleAmount,
		arg_maxActiveReservations,
	)

	return err
}

func (rr *ReservationRouter) UpdateReservationCapsGasEstimate(
	arg_maxReservationsAmountPerWallet uint64,
	arg_reservationMaxSingleAmount uint64,
	arg_maxActiveReservations uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"updateReservationCaps",
		rr.contractABI,
		rr.transactor,
		arg_maxReservationsAmountPerWallet,
		arg_reservationMaxSingleAmount,
		arg_maxActiveReservations,
	)

	return result, err
}

// Transaction submission.
func (rr *ReservationRouter) UpdateReservationParameters(
	arg_reservationVault common.Address,
	arg_reservationMinAmount uint64,
	arg_reservationTxMaxFee uint64,
	arg_reservationTermSeconds uint32,
	arg_reservationDissolutionDelay uint32,
	arg_reservationMaxTotalAmount uint64,
	arg_maxReservationsPerWallet uint32,
	arg_reservationActionTimeout uint32,
	arg_reservationRenewalWindowSeconds uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	rrLogger.Debug(
		"submitting transaction updateReservationParameters",
		" params: ",
		fmt.Sprint(
			arg_reservationVault,
			arg_reservationMinAmount,
			arg_reservationTxMaxFee,
			arg_reservationTermSeconds,
			arg_reservationDissolutionDelay,
			arg_reservationMaxTotalAmount,
			arg_maxReservationsPerWallet,
			arg_reservationActionTimeout,
			arg_reservationRenewalWindowSeconds,
		),
	)

	rr.transactionMutex.Lock()
	defer rr.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *rr.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := rr.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := rr.contract.UpdateReservationParameters(
		transactorOptions,
		arg_reservationVault,
		arg_reservationMinAmount,
		arg_reservationTxMaxFee,
		arg_reservationTermSeconds,
		arg_reservationDissolutionDelay,
		arg_reservationMaxTotalAmount,
		arg_maxReservationsPerWallet,
		arg_reservationActionTimeout,
		arg_reservationRenewalWindowSeconds,
	)
	if err != nil {
		return transaction, rr.errorResolver.ResolveError(
			err,
			rr.transactorOptions.From,
			nil,
			"updateReservationParameters",
			arg_reservationVault,
			arg_reservationMinAmount,
			arg_reservationTxMaxFee,
			arg_reservationTermSeconds,
			arg_reservationDissolutionDelay,
			arg_reservationMaxTotalAmount,
			arg_maxReservationsPerWallet,
			arg_reservationActionTimeout,
			arg_reservationRenewalWindowSeconds,
		)
	}

	rrLogger.Infof(
		"submitted transaction updateReservationParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go rr.miningWaiter.ForceMining(
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

			transaction, err := rr.contract.UpdateReservationParameters(
				newTransactorOptions,
				arg_reservationVault,
				arg_reservationMinAmount,
				arg_reservationTxMaxFee,
				arg_reservationTermSeconds,
				arg_reservationDissolutionDelay,
				arg_reservationMaxTotalAmount,
				arg_maxReservationsPerWallet,
				arg_reservationActionTimeout,
				arg_reservationRenewalWindowSeconds,
			)
			if err != nil {
				return nil, rr.errorResolver.ResolveError(
					err,
					rr.transactorOptions.From,
					nil,
					"updateReservationParameters",
					arg_reservationVault,
					arg_reservationMinAmount,
					arg_reservationTxMaxFee,
					arg_reservationTermSeconds,
					arg_reservationDissolutionDelay,
					arg_reservationMaxTotalAmount,
					arg_maxReservationsPerWallet,
					arg_reservationActionTimeout,
					arg_reservationRenewalWindowSeconds,
				)
			}

			rrLogger.Infof(
				"submitted transaction updateReservationParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	rr.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (rr *ReservationRouter) CallUpdateReservationParameters(
	arg_reservationVault common.Address,
	arg_reservationMinAmount uint64,
	arg_reservationTxMaxFee uint64,
	arg_reservationTermSeconds uint32,
	arg_reservationDissolutionDelay uint32,
	arg_reservationMaxTotalAmount uint64,
	arg_maxReservationsPerWallet uint32,
	arg_reservationActionTimeout uint32,
	arg_reservationRenewalWindowSeconds uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		rr.transactorOptions.From,
		blockNumber, nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"updateReservationParameters",
		&result,
		arg_reservationVault,
		arg_reservationMinAmount,
		arg_reservationTxMaxFee,
		arg_reservationTermSeconds,
		arg_reservationDissolutionDelay,
		arg_reservationMaxTotalAmount,
		arg_maxReservationsPerWallet,
		arg_reservationActionTimeout,
		arg_reservationRenewalWindowSeconds,
	)

	return err
}

func (rr *ReservationRouter) UpdateReservationParametersGasEstimate(
	arg_reservationVault common.Address,
	arg_reservationMinAmount uint64,
	arg_reservationTxMaxFee uint64,
	arg_reservationTermSeconds uint32,
	arg_reservationDissolutionDelay uint32,
	arg_reservationMaxTotalAmount uint64,
	arg_maxReservationsPerWallet uint32,
	arg_reservationActionTimeout uint32,
	arg_reservationRenewalWindowSeconds uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		rr.callerOptions.From,
		rr.contractAddress,
		"updateReservationParameters",
		rr.contractABI,
		rr.transactor,
		arg_reservationVault,
		arg_reservationMinAmount,
		arg_reservationTxMaxFee,
		arg_reservationTermSeconds,
		arg_reservationDissolutionDelay,
		arg_reservationMaxTotalAmount,
		arg_maxReservationsPerWallet,
		arg_reservationActionTimeout,
		arg_reservationRenewalWindowSeconds,
	)

	return result, err
}

// ----- Const Methods ------

type activeReservationsCount struct {
	Count     uint32
	MaxActive uint32
}

func (rr *ReservationRouter) ActiveReservationsCount() (activeReservationsCount, error) {
	result, err := rr.contract.ActiveReservationsCount(
		rr.callerOptions,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"activeReservationsCount",
		)
	}

	return result, err
}

func (rr *ReservationRouter) ActiveReservationsCountAtBlock(
	blockNumber *big.Int,
) (activeReservationsCount, error) {
	var result activeReservationsCount

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"activeReservationsCount",
		&result,
	)

	return result, err
}

func (rr *ReservationRouter) Governance() (common.Address, error) {
	result, err := rr.contract.Governance(
		rr.callerOptions,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"governance",
		)
	}

	return result, err
}

func (rr *ReservationRouter) GovernanceAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"governance",
		&result,
	)

	return result, err
}

func (rr *ReservationRouter) PendingReservedDeposits() (uint64, error) {
	result, err := rr.contract.PendingReservedDeposits(
		rr.callerOptions,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"pendingReservedDeposits",
		)
	}

	return result, err
}

func (rr *ReservationRouter) PendingReservedDepositsAtBlock(
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"pendingReservedDeposits",
		&result,
	)

	return result, err
}

func (rr *ReservationRouter) ReservationActions(
	arg_reservationKey *big.Int,
	arg_requestNonce uint64,
) (abi.ReservationReservationAction, error) {
	result, err := rr.contract.ReservationActions(
		rr.callerOptions,
		arg_reservationKey,
		arg_requestNonce,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"reservationActions",
			arg_reservationKey,
			arg_requestNonce,
		)
	}

	return result, err
}

func (rr *ReservationRouter) ReservationActionsAtBlock(
	arg_reservationKey *big.Int,
	arg_requestNonce uint64,
	blockNumber *big.Int,
) (abi.ReservationReservationAction, error) {
	var result abi.ReservationReservationAction

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"reservationActions",
		&result,
		arg_reservationKey,
		arg_requestNonce,
	)

	return result, err
}

func (rr *ReservationRouter) ReservationByAnchorUtxo(
	arg_anchorTxHash [32]byte,
	arg_anchorTxOutputIndex uint32,
) (*big.Int, error) {
	result, err := rr.contract.ReservationByAnchorUtxo(
		rr.callerOptions,
		arg_anchorTxHash,
		arg_anchorTxOutputIndex,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"reservationByAnchorUtxo",
			arg_anchorTxHash,
			arg_anchorTxOutputIndex,
		)
	}

	return result, err
}

func (rr *ReservationRouter) ReservationByAnchorUtxoAtBlock(
	arg_anchorTxHash [32]byte,
	arg_anchorTxOutputIndex uint32,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"reservationByAnchorUtxo",
		&result,
		arg_anchorTxHash,
		arg_anchorTxOutputIndex,
	)

	return result, err
}

type reservationCaps struct {
	MaxReservationsAmountPerWallet uint64
	ReservationMaxSingleAmount     uint64
}

func (rr *ReservationRouter) ReservationCaps() (reservationCaps, error) {
	result, err := rr.contract.ReservationCaps(
		rr.callerOptions,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"reservationCaps",
		)
	}

	return result, err
}

func (rr *ReservationRouter) ReservationCapsAtBlock(
	blockNumber *big.Int,
) (reservationCaps, error) {
	var result reservationCaps

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"reservationCaps",
		&result,
	)

	return result, err
}

type reservationParameters struct {
	ReservationVault                common.Address
	ReservationMinAmount            uint64
	ReservationTxMaxFee             uint64
	ReservationTermSeconds          uint32
	ReservationDissolutionDelay     uint32
	ReservationMaxTotalAmount       uint64
	ReservationTotalAmount          uint64
	MaxReservationsPerWallet        uint32
	ReservationActionTimeout        uint32
	ReservationRenewalWindowSeconds uint32
}

func (rr *ReservationRouter) ReservationParameters() (reservationParameters, error) {
	result, err := rr.contract.ReservationParameters(
		rr.callerOptions,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"reservationParameters",
		)
	}

	return result, err
}

func (rr *ReservationRouter) ReservationParametersAtBlock(
	blockNumber *big.Int,
) (reservationParameters, error) {
	var result reservationParameters

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"reservationParameters",
		&result,
	)

	return result, err
}

func (rr *ReservationRouter) ReservationRouter() (common.Address, error) {
	result, err := rr.contract.ReservationRouter(
		rr.callerOptions,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"reservationRouter",
		)
	}

	return result, err
}

func (rr *ReservationRouter) ReservationRouterAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"reservationRouter",
		&result,
	)

	return result, err
}

func (rr *ReservationRouter) Reservations(
	arg_reservationKey *big.Int,
) (abi.ReservationReservationRequest, error) {
	result, err := rr.contract.Reservations(
		rr.callerOptions,
		arg_reservationKey,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"reservations",
			arg_reservationKey,
		)
	}

	return result, err
}

func (rr *ReservationRouter) ReservationsAtBlock(
	arg_reservationKey *big.Int,
	blockNumber *big.Int,
) (abi.ReservationReservationRequest, error) {
	var result abi.ReservationReservationRequest

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"reservations",
		&result,
		arg_reservationKey,
	)

	return result, err
}

func (rr *ReservationRouter) ReservedDepositWallet(
	arg_depositKey *big.Int,
) ([20]byte, error) {
	result, err := rr.contract.ReservedDepositWallet(
		rr.callerOptions,
		arg_depositKey,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"reservedDepositWallet",
			arg_depositKey,
		)
	}

	return result, err
}

func (rr *ReservationRouter) ReservedDepositWalletAtBlock(
	arg_depositKey *big.Int,
	blockNumber *big.Int,
) ([20]byte, error) {
	var result [20]byte

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"reservedDepositWallet",
		&result,
		arg_depositKey,
	)

	return result, err
}

func (rr *ReservationRouter) WalletReservations(
	arg_walletPubKeyHash [20]byte,
) ([]*big.Int, error) {
	result, err := rr.contract.WalletReservations(
		rr.callerOptions,
		arg_walletPubKeyHash,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"walletReservations",
			arg_walletPubKeyHash,
		)
	}

	return result, err
}

func (rr *ReservationRouter) WalletReservationsAtBlock(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) ([]*big.Int, error) {
	var result []*big.Int

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"walletReservations",
		&result,
		arg_walletPubKeyHash,
	)

	return result, err
}

func (rr *ReservationRouter) WalletReservationsAmount(
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	result, err := rr.contract.WalletReservationsAmount(
		rr.callerOptions,
		arg_walletPubKeyHash,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"walletReservationsAmount",
			arg_walletPubKeyHash,
		)
	}

	return result, err
}

func (rr *ReservationRouter) WalletReservationsAmountAtBlock(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"walletReservationsAmount",
		&result,
		arg_walletPubKeyHash,
	)

	return result, err
}

func (rr *ReservationRouter) WalletReservationsCount(
	arg_walletPubKeyHash [20]byte,
) (uint32, error) {
	result, err := rr.contract.WalletReservationsCount(
		rr.callerOptions,
		arg_walletPubKeyHash,
	)

	if err != nil {
		return result, rr.errorResolver.ResolveError(
			err,
			rr.callerOptions.From,
			nil,
			"walletReservationsCount",
			arg_walletPubKeyHash,
		)
	}

	return result, err
}

func (rr *ReservationRouter) WalletReservationsCountAtBlock(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		rr.callerOptions.From,
		blockNumber,
		nil,
		rr.contractABI,
		rr.caller,
		rr.errorResolver,
		rr.contractAddress,
		"walletReservationsCount",
		&result,
		arg_walletPubKeyHash,
	)

	return result, err
}

// ------ Events -------

func (rr *ReservationRouter) GovernanceTransferredEvent(
	opts *ethereum.SubscribeOpts,
) *RrGovernanceTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrGovernanceTransferredSubscription{
		rr,
		opts,
	}
}

type RrGovernanceTransferredSubscription struct {
	contract *ReservationRouter
	opts     *ethereum.SubscribeOpts
}

type reservationRouterGovernanceTransferredFunc func(
	OldGovernance common.Address,
	NewGovernance common.Address,
	blockNumber uint64,
)

func (gts *RrGovernanceTransferredSubscription) OnEvent(
	handler reservationRouterGovernanceTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterGovernanceTransferred)
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

func (gts *RrGovernanceTransferredSubscription) Pipe(
	sink chan *abi.ReservationRouterGovernanceTransferred,
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
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gts.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past GovernanceTransferred events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gts.contract.PastGovernanceTransferredEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
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

func (rr *ReservationRouter) watchGovernanceTransferred(
	sink chan *abi.ReservationRouterGovernanceTransferred,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchGovernanceTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event GovernanceTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
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

func (rr *ReservationRouter) PastGovernanceTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.ReservationRouterGovernanceTransferred, error) {
	iterator, err := rr.contract.FilterGovernanceTransferred(
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

	events := make([]*abi.ReservationRouterGovernanceTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) InitializedEvent(
	opts *ethereum.SubscribeOpts,
) *RrInitializedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrInitializedSubscription{
		rr,
		opts,
	}
}

type RrInitializedSubscription struct {
	contract *ReservationRouter
	opts     *ethereum.SubscribeOpts
}

type reservationRouterInitializedFunc func(
	Version uint8,
	blockNumber uint64,
)

func (is *RrInitializedSubscription) OnEvent(
	handler reservationRouterInitializedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterInitialized)
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

func (is *RrInitializedSubscription) Pipe(
	sink chan *abi.ReservationRouterInitialized,
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
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - is.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past Initialized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := is.contract.PastInitializedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
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

func (rr *ReservationRouter) watchInitialized(
	sink chan *abi.ReservationRouterInitialized,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchInitialized(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event Initialized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
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

func (rr *ReservationRouter) PastInitializedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.ReservationRouterInitialized, error) {
	iterator, err := rr.contract.FilterInitialized(
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

	events := make([]*abi.ReservationRouterInitialized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationAcceptanceRequestedEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
) *RrReservationAcceptanceRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationAcceptanceRequestedSubscription{
		rr,
		opts,
		reservationKeyFilter,
		walletPubKeyHashFilter,
	}
}

type RrReservationAcceptanceRequestedSubscription struct {
	contract               *ReservationRouter
	opts                   *ethereum.SubscribeOpts
	reservationKeyFilter   []*big.Int
	walletPubKeyHashFilter [][20]byte
}

type reservationRouterReservationAcceptanceRequestedFunc func(
	ReservationKey *big.Int,
	RequestNonce uint64,
	WalletPubKeyHash [20]byte,
	DepositAmount uint64,
	TxMaxFee uint64,
	TimeoutAt uint32,
	blockNumber uint64,
)

func (rars *RrReservationAcceptanceRequestedSubscription) OnEvent(
	handler reservationRouterReservationAcceptanceRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationAcceptanceRequested)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.RequestNonce,
					event.WalletPubKeyHash,
					event.DepositAmount,
					event.TxMaxFee,
					event.TimeoutAt,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rars.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rars *RrReservationAcceptanceRequestedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationAcceptanceRequested,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rars.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rars.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rars.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationAcceptanceRequested events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rars.contract.PastReservationAcceptanceRequestedEvents(
					fromBlock,
					nil,
					rars.reservationKeyFilter,
					rars.walletPubKeyHashFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationAcceptanceRequested events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rars.contract.watchReservationAcceptanceRequested(
		sink,
		rars.reservationKeyFilter,
		rars.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationAcceptanceRequested(
	sink chan *abi.ReservationRouterReservationAcceptanceRequested,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationAcceptanceRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationAcceptanceRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationAcceptanceRequested failed "+
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

func (rr *ReservationRouter) PastReservationAcceptanceRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.ReservationRouterReservationAcceptanceRequested, error) {
	iterator, err := rr.contract.FilterReservationAcceptanceRequested(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationAcceptanceRequested events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationAcceptanceRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationAcceptedEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
	ownerFilter []common.Address,
) *RrReservationAcceptedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationAcceptedSubscription{
		rr,
		opts,
		reservationKeyFilter,
		walletPubKeyHashFilter,
		ownerFilter,
	}
}

type RrReservationAcceptedSubscription struct {
	contract               *ReservationRouter
	opts                   *ethereum.SubscribeOpts
	reservationKeyFilter   []*big.Int
	walletPubKeyHashFilter [][20]byte
	ownerFilter            []common.Address
}

type reservationRouterReservationAcceptedFunc func(
	ReservationKey *big.Int,
	RequestNonce uint64,
	WalletPubKeyHash [20]byte,
	Owner common.Address,
	AnchorTxHash [32]byte,
	AnchorAmount uint64,
	ExpiresAt uint32,
	blockNumber uint64,
)

func (ras *RrReservationAcceptedSubscription) OnEvent(
	handler reservationRouterReservationAcceptedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationAccepted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.RequestNonce,
					event.WalletPubKeyHash,
					event.Owner,
					event.AnchorTxHash,
					event.AnchorAmount,
					event.ExpiresAt,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ras.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ras *RrReservationAcceptedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationAccepted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ras.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ras.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ras.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationAccepted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ras.contract.PastReservationAcceptedEvents(
					fromBlock,
					nil,
					ras.reservationKeyFilter,
					ras.walletPubKeyHashFilter,
					ras.ownerFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationAccepted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ras.contract.watchReservationAccepted(
		sink,
		ras.reservationKeyFilter,
		ras.walletPubKeyHashFilter,
		ras.ownerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationAccepted(
	sink chan *abi.ReservationRouterReservationAccepted,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
	ownerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationAccepted(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
			walletPubKeyHashFilter,
			ownerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationAccepted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationAccepted failed "+
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

func (rr *ReservationRouter) PastReservationAcceptedEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
	ownerFilter []common.Address,
) ([]*abi.ReservationRouterReservationAccepted, error) {
	iterator, err := rr.contract.FilterReservationAccepted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
		walletPubKeyHashFilter,
		ownerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationAccepted events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationAccepted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationActionSupersededEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
) *RrReservationActionSupersededSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationActionSupersededSubscription{
		rr,
		opts,
		reservationKeyFilter,
	}
}

type RrReservationActionSupersededSubscription struct {
	contract             *ReservationRouter
	opts                 *ethereum.SubscribeOpts
	reservationKeyFilter []*big.Int
}

type reservationRouterReservationActionSupersededFunc func(
	ReservationKey *big.Int,
	RequestNonce uint64,
	blockNumber uint64,
)

func (rass *RrReservationActionSupersededSubscription) OnEvent(
	handler reservationRouterReservationActionSupersededFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationActionSuperseded)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.RequestNonce,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rass.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rass *RrReservationActionSupersededSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationActionSuperseded,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rass.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rass.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rass.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationActionSuperseded events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rass.contract.PastReservationActionSupersededEvents(
					fromBlock,
					nil,
					rass.reservationKeyFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationActionSuperseded events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rass.contract.watchReservationActionSuperseded(
		sink,
		rass.reservationKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationActionSuperseded(
	sink chan *abi.ReservationRouterReservationActionSuperseded,
	reservationKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationActionSuperseded(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationActionSuperseded had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationActionSuperseded failed "+
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

func (rr *ReservationRouter) PastReservationActionSupersededEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
) ([]*abi.ReservationRouterReservationActionSuperseded, error) {
	iterator, err := rr.contract.FilterReservationActionSuperseded(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationActionSuperseded events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationActionSuperseded, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationActionTimedOutEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
) *RrReservationActionTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationActionTimedOutSubscription{
		rr,
		opts,
		reservationKeyFilter,
	}
}

type RrReservationActionTimedOutSubscription struct {
	contract             *ReservationRouter
	opts                 *ethereum.SubscribeOpts
	reservationKeyFilter []*big.Int
}

type reservationRouterReservationActionTimedOutFunc func(
	ReservationKey *big.Int,
	RequestNonce uint64,
	ActionType uint8,
	blockNumber uint64,
)

func (ratos *RrReservationActionTimedOutSubscription) OnEvent(
	handler reservationRouterReservationActionTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationActionTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.RequestNonce,
					event.ActionType,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := ratos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (ratos *RrReservationActionTimedOutSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationActionTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(ratos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := ratos.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ratos.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationActionTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := ratos.contract.PastReservationActionTimedOutEvents(
					fromBlock,
					nil,
					ratos.reservationKeyFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationActionTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := ratos.contract.watchReservationActionTimedOut(
		sink,
		ratos.reservationKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationActionTimedOut(
	sink chan *abi.ReservationRouterReservationActionTimedOut,
	reservationKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationActionTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationActionTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationActionTimedOut failed "+
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

func (rr *ReservationRouter) PastReservationActionTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
) ([]*abi.ReservationRouterReservationActionTimedOut, error) {
	iterator, err := rr.contract.FilterReservationActionTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationActionTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationActionTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationCapsUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RrReservationCapsUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationCapsUpdatedSubscription{
		rr,
		opts,
	}
}

type RrReservationCapsUpdatedSubscription struct {
	contract *ReservationRouter
	opts     *ethereum.SubscribeOpts
}

type reservationRouterReservationCapsUpdatedFunc func(
	MaxReservationsAmountPerWallet uint64,
	ReservationMaxSingleAmount uint64,
	MaxActiveReservations uint32,
	blockNumber uint64,
)

func (rcus *RrReservationCapsUpdatedSubscription) OnEvent(
	handler reservationRouterReservationCapsUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationCapsUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.MaxReservationsAmountPerWallet,
					event.ReservationMaxSingleAmount,
					event.MaxActiveReservations,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rcus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rcus *RrReservationCapsUpdatedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationCapsUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rcus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rcus.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rcus.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationCapsUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rcus.contract.PastReservationCapsUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationCapsUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rcus.contract.watchReservationCapsUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationCapsUpdated(
	sink chan *abi.ReservationRouterReservationCapsUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationCapsUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationCapsUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationCapsUpdated failed "+
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

func (rr *ReservationRouter) PastReservationCapsUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.ReservationRouterReservationCapsUpdated, error) {
	iterator, err := rr.contract.FilterReservationCapsUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationCapsUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationCapsUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationLateSettledEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
) *RrReservationLateSettledSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationLateSettledSubscription{
		rr,
		opts,
		reservationKeyFilter,
	}
}

type RrReservationLateSettledSubscription struct {
	contract             *ReservationRouter
	opts                 *ethereum.SubscribeOpts
	reservationKeyFilter []*big.Int
}

type reservationRouterReservationLateSettledFunc func(
	ReservationKey *big.Int,
	RequestNonce uint64,
	ActionType uint8,
	blockNumber uint64,
)

func (rlss *RrReservationLateSettledSubscription) OnEvent(
	handler reservationRouterReservationLateSettledFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationLateSettled)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.RequestNonce,
					event.ActionType,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rlss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rlss *RrReservationLateSettledSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationLateSettled,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rlss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rlss.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rlss.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationLateSettled events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rlss.contract.PastReservationLateSettledEvents(
					fromBlock,
					nil,
					rlss.reservationKeyFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationLateSettled events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rlss.contract.watchReservationLateSettled(
		sink,
		rlss.reservationKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationLateSettled(
	sink chan *abi.ReservationRouterReservationLateSettled,
	reservationKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationLateSettled(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationLateSettled had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationLateSettled failed "+
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

func (rr *ReservationRouter) PastReservationLateSettledEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
) ([]*abi.ReservationRouterReservationLateSettled, error) {
	iterator, err := rr.contract.FilterReservationLateSettled(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationLateSettled events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationLateSettled, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RrReservationParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationParametersUpdatedSubscription{
		rr,
		opts,
	}
}

type RrReservationParametersUpdatedSubscription struct {
	contract *ReservationRouter
	opts     *ethereum.SubscribeOpts
}

type reservationRouterReservationParametersUpdatedFunc func(
	ReservationMinAmount uint64,
	ReservationTxMaxFee uint64,
	ReservationTermSeconds uint32,
	ReservationDissolutionDelay uint32,
	ReservationMaxTotalAmount uint64,
	MaxReservationsPerWallet uint32,
	ReservationActionTimeout uint32,
	ReservationRenewalWindowSeconds uint32,
	blockNumber uint64,
)

func (rpus *RrReservationParametersUpdatedSubscription) OnEvent(
	handler reservationRouterReservationParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationMinAmount,
					event.ReservationTxMaxFee,
					event.ReservationTermSeconds,
					event.ReservationDissolutionDelay,
					event.ReservationMaxTotalAmount,
					event.MaxReservationsPerWallet,
					event.ReservationActionTimeout,
					event.ReservationRenewalWindowSeconds,
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

func (rpus *RrReservationParametersUpdatedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationParametersUpdated,
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
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastReservationParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rpus.contract.watchReservationParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationParametersUpdated(
	sink chan *abi.ReservationRouterReservationParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationParametersUpdated failed "+
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

func (rr *ReservationRouter) PastReservationParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.ReservationRouterReservationParametersUpdated, error) {
	iterator, err := rr.contract.FilterReservationParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationReanchorRequestedEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
	sourceWalletPubKeyHashFilter [][20]byte,
	targetWalletPubKeyHashFilter [][20]byte,
) *RrReservationReanchorRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationReanchorRequestedSubscription{
		rr,
		opts,
		reservationKeyFilter,
		sourceWalletPubKeyHashFilter,
		targetWalletPubKeyHashFilter,
	}
}

type RrReservationReanchorRequestedSubscription struct {
	contract                     *ReservationRouter
	opts                         *ethereum.SubscribeOpts
	reservationKeyFilter         []*big.Int
	sourceWalletPubKeyHashFilter [][20]byte
	targetWalletPubKeyHashFilter [][20]byte
}

type reservationRouterReservationReanchorRequestedFunc func(
	ReservationKey *big.Int,
	RequestNonce uint64,
	SourceWalletPubKeyHash [20]byte,
	TargetWalletPubKeyHash [20]byte,
	TxMaxFee uint64,
	blockNumber uint64,
)

func (rrrs *RrReservationReanchorRequestedSubscription) OnEvent(
	handler reservationRouterReservationReanchorRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationReanchorRequested)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.RequestNonce,
					event.SourceWalletPubKeyHash,
					event.TargetWalletPubKeyHash,
					event.TxMaxFee,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rrrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rrrs *RrReservationReanchorRequestedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationReanchorRequested,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rrrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rrrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rrrs.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationReanchorRequested events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rrrs.contract.PastReservationReanchorRequestedEvents(
					fromBlock,
					nil,
					rrrs.reservationKeyFilter,
					rrrs.sourceWalletPubKeyHashFilter,
					rrrs.targetWalletPubKeyHashFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationReanchorRequested events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rrrs.contract.watchReservationReanchorRequested(
		sink,
		rrrs.reservationKeyFilter,
		rrrs.sourceWalletPubKeyHashFilter,
		rrrs.targetWalletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationReanchorRequested(
	sink chan *abi.ReservationRouterReservationReanchorRequested,
	reservationKeyFilter []*big.Int,
	sourceWalletPubKeyHashFilter [][20]byte,
	targetWalletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationReanchorRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
			sourceWalletPubKeyHashFilter,
			targetWalletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationReanchorRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationReanchorRequested failed "+
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

func (rr *ReservationRouter) PastReservationReanchorRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
	sourceWalletPubKeyHashFilter [][20]byte,
	targetWalletPubKeyHashFilter [][20]byte,
) ([]*abi.ReservationRouterReservationReanchorRequested, error) {
	iterator, err := rr.contract.FilterReservationReanchorRequested(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
		sourceWalletPubKeyHashFilter,
		targetWalletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationReanchorRequested events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationReanchorRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationReanchoredEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
	newWalletPubKeyHashFilter [][20]byte,
) *RrReservationReanchoredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationReanchoredSubscription{
		rr,
		opts,
		reservationKeyFilter,
		newWalletPubKeyHashFilter,
	}
}

type RrReservationReanchoredSubscription struct {
	contract                  *ReservationRouter
	opts                      *ethereum.SubscribeOpts
	reservationKeyFilter      []*big.Int
	newWalletPubKeyHashFilter [][20]byte
}

type reservationRouterReservationReanchoredFunc func(
	ReservationKey *big.Int,
	RequestNonce uint64,
	NewWalletPubKeyHash [20]byte,
	NewAnchorTxHash [32]byte,
	NewAnchorAmount uint64,
	blockNumber uint64,
)

func (rrs *RrReservationReanchoredSubscription) OnEvent(
	handler reservationRouterReservationReanchoredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationReanchored)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.RequestNonce,
					event.NewWalletPubKeyHash,
					event.NewAnchorTxHash,
					event.NewAnchorAmount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rrs *RrReservationReanchoredSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationReanchored,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rrs.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationReanchored events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rrs.contract.PastReservationReanchoredEvents(
					fromBlock,
					nil,
					rrs.reservationKeyFilter,
					rrs.newWalletPubKeyHashFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationReanchored events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rrs.contract.watchReservationReanchored(
		sink,
		rrs.reservationKeyFilter,
		rrs.newWalletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationReanchored(
	sink chan *abi.ReservationRouterReservationReanchored,
	reservationKeyFilter []*big.Int,
	newWalletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationReanchored(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
			newWalletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationReanchored had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationReanchored failed "+
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

func (rr *ReservationRouter) PastReservationReanchoredEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
	newWalletPubKeyHashFilter [][20]byte,
) ([]*abi.ReservationRouterReservationReanchored, error) {
	iterator, err := rr.contract.FilterReservationReanchored(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
		newWalletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationReanchored events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationReanchored, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationRetryCreditMintedEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
) *RrReservationRetryCreditMintedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationRetryCreditMintedSubscription{
		rr,
		opts,
		reservationKeyFilter,
	}
}

type RrReservationRetryCreditMintedSubscription struct {
	contract             *ReservationRouter
	opts                 *ethereum.SubscribeOpts
	reservationKeyFilter []*big.Int
}

type reservationRouterReservationRetryCreditMintedFunc func(
	ReservationKey *big.Int,
	blockNumber uint64,
)

func (rrcms *RrReservationRetryCreditMintedSubscription) OnEvent(
	handler reservationRouterReservationRetryCreditMintedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationRetryCreditMinted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rrcms.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rrcms *RrReservationRetryCreditMintedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationRetryCreditMinted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rrcms.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rrcms.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rrcms.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationRetryCreditMinted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rrcms.contract.PastReservationRetryCreditMintedEvents(
					fromBlock,
					nil,
					rrcms.reservationKeyFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationRetryCreditMinted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rrcms.contract.watchReservationRetryCreditMinted(
		sink,
		rrcms.reservationKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationRetryCreditMinted(
	sink chan *abi.ReservationRouterReservationRetryCreditMinted,
	reservationKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationRetryCreditMinted(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationRetryCreditMinted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationRetryCreditMinted failed "+
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

func (rr *ReservationRouter) PastReservationRetryCreditMintedEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
) ([]*abi.ReservationRouterReservationRetryCreditMinted, error) {
	iterator, err := rr.contract.FilterReservationRetryCreditMinted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationRetryCreditMinted events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationRetryCreditMinted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationRouterSetEvent(
	opts *ethereum.SubscribeOpts,
) *RrReservationRouterSetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationRouterSetSubscription{
		rr,
		opts,
	}
}

type RrReservationRouterSetSubscription struct {
	contract *ReservationRouter
	opts     *ethereum.SubscribeOpts
}

type reservationRouterReservationRouterSetFunc func(
	ReservationRouter common.Address,
	blockNumber uint64,
)

func (rrss *RrReservationRouterSetSubscription) OnEvent(
	handler reservationRouterReservationRouterSetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationRouterSet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationRouter,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rrss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rrss *RrReservationRouterSetSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationRouterSet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rrss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rrss.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rrss.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationRouterSet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rrss.contract.PastReservationRouterSetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationRouterSet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rrss.contract.watchReservationRouterSet(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationRouterSet(
	sink chan *abi.ReservationRouterReservationRouterSet,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationRouterSet(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationRouterSet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationRouterSet failed "+
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

func (rr *ReservationRouter) PastReservationRouterSetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.ReservationRouterReservationRouterSet, error) {
	iterator, err := rr.contract.FilterReservationRouterSet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationRouterSet events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationRouterSet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationStrandedEvent(
	opts *ethereum.SubscribeOpts,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
	ownerFilter []common.Address,
) *RrReservationStrandedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationStrandedSubscription{
		rr,
		opts,
		reservationKeyFilter,
		walletPubKeyHashFilter,
		ownerFilter,
	}
}

type RrReservationStrandedSubscription struct {
	contract               *ReservationRouter
	opts                   *ethereum.SubscribeOpts
	reservationKeyFilter   []*big.Int
	walletPubKeyHashFilter [][20]byte
	ownerFilter            []common.Address
}

type reservationRouterReservationStrandedFunc func(
	ReservationKey *big.Int,
	WalletPubKeyHash [20]byte,
	Owner common.Address,
	AnchorAmount uint64,
	blockNumber uint64,
)

func (rss *RrReservationStrandedSubscription) OnEvent(
	handler reservationRouterReservationStrandedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationStranded)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationKey,
					event.WalletPubKeyHash,
					event.Owner,
					event.AnchorAmount,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rss *RrReservationStrandedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationStranded,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rss.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rss.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationStranded events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rss.contract.PastReservationStrandedEvents(
					fromBlock,
					nil,
					rss.reservationKeyFilter,
					rss.walletPubKeyHashFilter,
					rss.ownerFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationStranded events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rss.contract.watchReservationStranded(
		sink,
		rss.reservationKeyFilter,
		rss.walletPubKeyHashFilter,
		rss.ownerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationStranded(
	sink chan *abi.ReservationRouterReservationStranded,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
	ownerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationStranded(
			&bind.WatchOpts{Context: ctx},
			sink,
			reservationKeyFilter,
			walletPubKeyHashFilter,
			ownerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationStranded had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationStranded failed "+
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

func (rr *ReservationRouter) PastReservationStrandedEvents(
	startBlock uint64,
	endBlock *uint64,
	reservationKeyFilter []*big.Int,
	walletPubKeyHashFilter [][20]byte,
	ownerFilter []common.Address,
) ([]*abi.ReservationRouterReservationStranded, error) {
	iterator, err := rr.contract.FilterReservationStranded(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		reservationKeyFilter,
		walletPubKeyHashFilter,
		ownerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationStranded events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationStranded, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservationVaultUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *RrReservationVaultUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservationVaultUpdatedSubscription{
		rr,
		opts,
	}
}

type RrReservationVaultUpdatedSubscription struct {
	contract *ReservationRouter
	opts     *ethereum.SubscribeOpts
}

type reservationRouterReservationVaultUpdatedFunc func(
	ReservationVault common.Address,
	blockNumber uint64,
)

func (rvus *RrReservationVaultUpdatedSubscription) OnEvent(
	handler reservationRouterReservationVaultUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservationVaultUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.ReservationVault,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rvus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rvus *RrReservationVaultUpdatedSubscription) Pipe(
	sink chan *abi.ReservationRouterReservationVaultUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rvus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rvus.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rvus.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservationVaultUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rvus.contract.PastReservationVaultUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservationVaultUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rvus.contract.watchReservationVaultUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservationVaultUpdated(
	sink chan *abi.ReservationRouterReservationVaultUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservationVaultUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservationVaultUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservationVaultUpdated failed "+
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

func (rr *ReservationRouter) PastReservationVaultUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.ReservationRouterReservationVaultUpdated, error) {
	iterator, err := rr.contract.FilterReservationVaultUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservationVaultUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservationVaultUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (rr *ReservationRouter) ReservedDepositMarkedStaleEvent(
	opts *ethereum.SubscribeOpts,
	depositKeyFilter []*big.Int,
) *RrReservedDepositMarkedStaleSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &RrReservedDepositMarkedStaleSubscription{
		rr,
		opts,
		depositKeyFilter,
	}
}

type RrReservedDepositMarkedStaleSubscription struct {
	contract         *ReservationRouter
	opts             *ethereum.SubscribeOpts
	depositKeyFilter []*big.Int
}

type reservationRouterReservedDepositMarkedStaleFunc func(
	DepositKey *big.Int,
	blockNumber uint64,
)

func (rdmss *RrReservedDepositMarkedStaleSubscription) OnEvent(
	handler reservationRouterReservedDepositMarkedStaleFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.ReservationRouterReservedDepositMarkedStale)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.DepositKey,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rdmss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rdmss *RrReservedDepositMarkedStaleSubscription) Pipe(
	sink chan *abi.ReservationRouterReservedDepositMarkedStale,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rdmss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rdmss.contract.blockCounter.CurrentBlock()
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rdmss.opts.PastBlocks

				rrLogger.Infof(
					"subscription monitoring fetching past ReservedDepositMarkedStale events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rdmss.contract.PastReservedDepositMarkedStaleEvents(
					fromBlock,
					nil,
					rdmss.depositKeyFilter,
				)
				if err != nil {
					rrLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				rrLogger.Infof(
					"subscription monitoring fetched [%v] past ReservedDepositMarkedStale events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rdmss.contract.watchReservedDepositMarkedStale(
		sink,
		rdmss.depositKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rr *ReservationRouter) watchReservedDepositMarkedStale(
	sink chan *abi.ReservationRouterReservedDepositMarkedStale,
	depositKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return rr.contract.WatchReservedDepositMarkedStale(
			&bind.WatchOpts{Context: ctx},
			sink,
			depositKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		rrLogger.Warnf(
			"subscription to event ReservedDepositMarkedStale had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		rrLogger.Errorf(
			"subscription to event ReservedDepositMarkedStale failed "+
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

func (rr *ReservationRouter) PastReservedDepositMarkedStaleEvents(
	startBlock uint64,
	endBlock *uint64,
	depositKeyFilter []*big.Int,
) ([]*abi.ReservationRouterReservedDepositMarkedStale, error) {
	iterator, err := rr.contract.FilterReservedDepositMarkedStale(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		depositKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past ReservedDepositMarkedStale events: [%v]",
			err,
		)
	}

	events := make([]*abi.ReservationRouterReservedDepositMarkedStale, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

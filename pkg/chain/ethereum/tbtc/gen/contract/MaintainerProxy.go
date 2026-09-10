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
var mpLogger = log.Logger("keep-contract-MaintainerProxy")

type MaintainerProxy struct {
	contract          *abi.MaintainerProxy
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

func NewMaintainerProxy(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*MaintainerProxy, error) {
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

	contract, err := abi.NewMaintainerProxy(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.MaintainerProxyABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &MaintainerProxy{
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
func (mp *MaintainerProxy) AuthorizeSpvMaintainer(
	arg_maintainer common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction authorizeSpvMaintainer",
		" params: ",
		fmt.Sprint(
			arg_maintainer,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.AuthorizeSpvMaintainer(
		transactorOptions,
		arg_maintainer,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"authorizeSpvMaintainer",
			arg_maintainer,
		)
	}

	mpLogger.Infof(
		"submitted transaction authorizeSpvMaintainer with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.AuthorizeSpvMaintainer(
				newTransactorOptions,
				arg_maintainer,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"authorizeSpvMaintainer",
					arg_maintainer,
				)
			}

			mpLogger.Infof(
				"submitted transaction authorizeSpvMaintainer with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallAuthorizeSpvMaintainer(
	arg_maintainer common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"authorizeSpvMaintainer",
		&result,
		arg_maintainer,
	)

	return err
}

func (mp *MaintainerProxy) AuthorizeSpvMaintainerGasEstimate(
	arg_maintainer common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"authorizeSpvMaintainer",
		mp.contractABI,
		mp.transactor,
		arg_maintainer,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) AuthorizeWalletMaintainer(
	arg_maintainer common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction authorizeWalletMaintainer",
		" params: ",
		fmt.Sprint(
			arg_maintainer,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.AuthorizeWalletMaintainer(
		transactorOptions,
		arg_maintainer,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"authorizeWalletMaintainer",
			arg_maintainer,
		)
	}

	mpLogger.Infof(
		"submitted transaction authorizeWalletMaintainer with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.AuthorizeWalletMaintainer(
				newTransactorOptions,
				arg_maintainer,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"authorizeWalletMaintainer",
					arg_maintainer,
				)
			}

			mpLogger.Infof(
				"submitted transaction authorizeWalletMaintainer with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallAuthorizeWalletMaintainer(
	arg_maintainer common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"authorizeWalletMaintainer",
		&result,
		arg_maintainer,
	)

	return err
}

func (mp *MaintainerProxy) AuthorizeWalletMaintainerGasEstimate(
	arg_maintainer common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"authorizeWalletMaintainer",
		mp.contractABI,
		mp.transactor,
		arg_maintainer,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) DefeatFraudChallenge(
	arg_walletPublicKey []byte,
	arg_preimage []byte,
	arg_witness bool,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction defeatFraudChallenge",
		" params: ",
		fmt.Sprint(
			arg_walletPublicKey,
			arg_preimage,
			arg_witness,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.DefeatFraudChallenge(
		transactorOptions,
		arg_walletPublicKey,
		arg_preimage,
		arg_witness,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"defeatFraudChallenge",
			arg_walletPublicKey,
			arg_preimage,
			arg_witness,
		)
	}

	mpLogger.Infof(
		"submitted transaction defeatFraudChallenge with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.DefeatFraudChallenge(
				newTransactorOptions,
				arg_walletPublicKey,
				arg_preimage,
				arg_witness,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"defeatFraudChallenge",
					arg_walletPublicKey,
					arg_preimage,
					arg_witness,
				)
			}

			mpLogger.Infof(
				"submitted transaction defeatFraudChallenge with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallDefeatFraudChallenge(
	arg_walletPublicKey []byte,
	arg_preimage []byte,
	arg_witness bool,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"defeatFraudChallenge",
		&result,
		arg_walletPublicKey,
		arg_preimage,
		arg_witness,
	)

	return err
}

func (mp *MaintainerProxy) DefeatFraudChallengeGasEstimate(
	arg_walletPublicKey []byte,
	arg_preimage []byte,
	arg_witness bool,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"defeatFraudChallenge",
		mp.contractABI,
		mp.transactor,
		arg_walletPublicKey,
		arg_preimage,
		arg_witness,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) DefeatFraudChallengeWithHeartbeat(
	arg_walletPublicKey []byte,
	arg_heartbeatMessage []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction defeatFraudChallengeWithHeartbeat",
		" params: ",
		fmt.Sprint(
			arg_walletPublicKey,
			arg_heartbeatMessage,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.DefeatFraudChallengeWithHeartbeat(
		transactorOptions,
		arg_walletPublicKey,
		arg_heartbeatMessage,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"defeatFraudChallengeWithHeartbeat",
			arg_walletPublicKey,
			arg_heartbeatMessage,
		)
	}

	mpLogger.Infof(
		"submitted transaction defeatFraudChallengeWithHeartbeat with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.DefeatFraudChallengeWithHeartbeat(
				newTransactorOptions,
				arg_walletPublicKey,
				arg_heartbeatMessage,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"defeatFraudChallengeWithHeartbeat",
					arg_walletPublicKey,
					arg_heartbeatMessage,
				)
			}

			mpLogger.Infof(
				"submitted transaction defeatFraudChallengeWithHeartbeat with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallDefeatFraudChallengeWithHeartbeat(
	arg_walletPublicKey []byte,
	arg_heartbeatMessage []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"defeatFraudChallengeWithHeartbeat",
		&result,
		arg_walletPublicKey,
		arg_heartbeatMessage,
	)

	return err
}

func (mp *MaintainerProxy) DefeatFraudChallengeWithHeartbeatGasEstimate(
	arg_walletPublicKey []byte,
	arg_heartbeatMessage []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"defeatFraudChallengeWithHeartbeat",
		mp.contractABI,
		mp.transactor,
		arg_walletPublicKey,
		arg_heartbeatMessage,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) NotifyMovingFundsBelowDust(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO2,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction notifyMovingFundsBelowDust",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_mainUtxo,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.NotifyMovingFundsBelowDust(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_mainUtxo,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"notifyMovingFundsBelowDust",
			arg_walletPubKeyHash,
			arg_mainUtxo,
		)
	}

	mpLogger.Infof(
		"submitted transaction notifyMovingFundsBelowDust with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.NotifyMovingFundsBelowDust(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_mainUtxo,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"notifyMovingFundsBelowDust",
					arg_walletPubKeyHash,
					arg_mainUtxo,
				)
			}

			mpLogger.Infof(
				"submitted transaction notifyMovingFundsBelowDust with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallNotifyMovingFundsBelowDust(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"notifyMovingFundsBelowDust",
		&result,
		arg_walletPubKeyHash,
		arg_mainUtxo,
	)

	return err
}

func (mp *MaintainerProxy) NotifyMovingFundsBelowDustGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO2,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"notifyMovingFundsBelowDust",
		mp.contractABI,
		mp.transactor,
		arg_walletPubKeyHash,
		arg_mainUtxo,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) NotifyWalletCloseable(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO2,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction notifyWalletCloseable",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_walletMainUtxo,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.NotifyWalletCloseable(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"notifyWalletCloseable",
			arg_walletPubKeyHash,
			arg_walletMainUtxo,
		)
	}

	mpLogger.Infof(
		"submitted transaction notifyWalletCloseable with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.NotifyWalletCloseable(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_walletMainUtxo,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"notifyWalletCloseable",
					arg_walletPubKeyHash,
					arg_walletMainUtxo,
				)
			}

			mpLogger.Infof(
				"submitted transaction notifyWalletCloseable with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallNotifyWalletCloseable(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO2,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"notifyWalletCloseable",
		&result,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
	)

	return err
}

func (mp *MaintainerProxy) NotifyWalletCloseableGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO2,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"notifyWalletCloseable",
		mp.contractABI,
		mp.transactor,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) NotifyWalletClosingPeriodElapsed(
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction notifyWalletClosingPeriodElapsed",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.NotifyWalletClosingPeriodElapsed(
		transactorOptions,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"notifyWalletClosingPeriodElapsed",
			arg_walletPubKeyHash,
		)
	}

	mpLogger.Infof(
		"submitted transaction notifyWalletClosingPeriodElapsed with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.NotifyWalletClosingPeriodElapsed(
				newTransactorOptions,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"notifyWalletClosingPeriodElapsed",
					arg_walletPubKeyHash,
				)
			}

			mpLogger.Infof(
				"submitted transaction notifyWalletClosingPeriodElapsed with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallNotifyWalletClosingPeriodElapsed(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"notifyWalletClosingPeriodElapsed",
		&result,
		arg_walletPubKeyHash,
	)

	return err
}

func (mp *MaintainerProxy) NotifyWalletClosingPeriodElapsedGasEstimate(
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"notifyWalletClosingPeriodElapsed",
		mp.contractABI,
		mp.transactor,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) RenounceOwnership(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction renounceOwnership",
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.RenounceOwnership(
		transactorOptions,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"renounceOwnership",
		)
	}

	mpLogger.Infof(
		"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.RenounceOwnership(
				newTransactorOptions,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"renounceOwnership",
				)
			}

			mpLogger.Infof(
				"submitted transaction renounceOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallRenounceOwnership(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"renounceOwnership",
		&result,
	)

	return err
}

func (mp *MaintainerProxy) RenounceOwnershipGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"renounceOwnership",
		mp.contractABI,
		mp.transactor,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) RequestNewWallet(
	arg_activeWalletMainUtxo abi.BitcoinTxUTXO2,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction requestNewWallet",
		" params: ",
		fmt.Sprint(
			arg_activeWalletMainUtxo,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.RequestNewWallet(
		transactorOptions,
		arg_activeWalletMainUtxo,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"requestNewWallet",
			arg_activeWalletMainUtxo,
		)
	}

	mpLogger.Infof(
		"submitted transaction requestNewWallet with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.RequestNewWallet(
				newTransactorOptions,
				arg_activeWalletMainUtxo,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"requestNewWallet",
					arg_activeWalletMainUtxo,
				)
			}

			mpLogger.Infof(
				"submitted transaction requestNewWallet with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallRequestNewWallet(
	arg_activeWalletMainUtxo abi.BitcoinTxUTXO2,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"requestNewWallet",
		&result,
		arg_activeWalletMainUtxo,
	)

	return err
}

func (mp *MaintainerProxy) RequestNewWalletGasEstimate(
	arg_activeWalletMainUtxo abi.BitcoinTxUTXO2,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"requestNewWallet",
		mp.contractABI,
		mp.transactor,
		arg_activeWalletMainUtxo,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) ResetMovingFundsTimeout(
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction resetMovingFundsTimeout",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.ResetMovingFundsTimeout(
		transactorOptions,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"resetMovingFundsTimeout",
			arg_walletPubKeyHash,
		)
	}

	mpLogger.Infof(
		"submitted transaction resetMovingFundsTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.ResetMovingFundsTimeout(
				newTransactorOptions,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"resetMovingFundsTimeout",
					arg_walletPubKeyHash,
				)
			}

			mpLogger.Infof(
				"submitted transaction resetMovingFundsTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallResetMovingFundsTimeout(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"resetMovingFundsTimeout",
		&result,
		arg_walletPubKeyHash,
	)

	return err
}

func (mp *MaintainerProxy) ResetMovingFundsTimeoutGasEstimate(
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"resetMovingFundsTimeout",
		mp.contractABI,
		mp.transactor,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) SubmitDepositSweepProof(
	arg_sweepTx abi.BitcoinTxInfo3,
	arg_sweepProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_vault common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction submitDepositSweepProof",
		" params: ",
		fmt.Sprint(
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
			arg_vault,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.SubmitDepositSweepProof(
		transactorOptions,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
		arg_vault,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"submitDepositSweepProof",
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
			arg_vault,
		)
	}

	mpLogger.Infof(
		"submitted transaction submitDepositSweepProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.SubmitDepositSweepProof(
				newTransactorOptions,
				arg_sweepTx,
				arg_sweepProof,
				arg_mainUtxo,
				arg_vault,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"submitDepositSweepProof",
					arg_sweepTx,
					arg_sweepProof,
					arg_mainUtxo,
					arg_vault,
				)
			}

			mpLogger.Infof(
				"submitted transaction submitDepositSweepProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallSubmitDepositSweepProof(
	arg_sweepTx abi.BitcoinTxInfo3,
	arg_sweepProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_vault common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitDepositSweepProof",
		&result,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
		arg_vault,
	)

	return err
}

func (mp *MaintainerProxy) SubmitDepositSweepProofGasEstimate(
	arg_sweepTx abi.BitcoinTxInfo3,
	arg_sweepProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_vault common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"submitDepositSweepProof",
		mp.contractABI,
		mp.transactor,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
		arg_vault,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) SubmitMovedFundsSweepProof(
	arg_sweepTx abi.BitcoinTxInfo3,
	arg_sweepProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction submitMovedFundsSweepProof",
		" params: ",
		fmt.Sprint(
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.SubmitMovedFundsSweepProof(
		transactorOptions,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"submitMovedFundsSweepProof",
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
		)
	}

	mpLogger.Infof(
		"submitted transaction submitMovedFundsSweepProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.SubmitMovedFundsSweepProof(
				newTransactorOptions,
				arg_sweepTx,
				arg_sweepProof,
				arg_mainUtxo,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"submitMovedFundsSweepProof",
					arg_sweepTx,
					arg_sweepProof,
					arg_mainUtxo,
				)
			}

			mpLogger.Infof(
				"submitted transaction submitMovedFundsSweepProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallSubmitMovedFundsSweepProof(
	arg_sweepTx abi.BitcoinTxInfo3,
	arg_sweepProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitMovedFundsSweepProof",
		&result,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
	)

	return err
}

func (mp *MaintainerProxy) SubmitMovedFundsSweepProofGasEstimate(
	arg_sweepTx abi.BitcoinTxInfo3,
	arg_sweepProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"submitMovedFundsSweepProof",
		mp.contractABI,
		mp.transactor,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) SubmitMovingFundsProof(
	arg_movingFundsTx abi.BitcoinTxInfo3,
	arg_movingFundsProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction submitMovingFundsProof",
		" params: ",
		fmt.Sprint(
			arg_movingFundsTx,
			arg_movingFundsProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.SubmitMovingFundsProof(
		transactorOptions,
		arg_movingFundsTx,
		arg_movingFundsProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"submitMovingFundsProof",
			arg_movingFundsTx,
			arg_movingFundsProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		)
	}

	mpLogger.Infof(
		"submitted transaction submitMovingFundsProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.SubmitMovingFundsProof(
				newTransactorOptions,
				arg_movingFundsTx,
				arg_movingFundsProof,
				arg_mainUtxo,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"submitMovingFundsProof",
					arg_movingFundsTx,
					arg_movingFundsProof,
					arg_mainUtxo,
					arg_walletPubKeyHash,
				)
			}

			mpLogger.Infof(
				"submitted transaction submitMovingFundsProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallSubmitMovingFundsProof(
	arg_movingFundsTx abi.BitcoinTxInfo3,
	arg_movingFundsProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitMovingFundsProof",
		&result,
		arg_movingFundsTx,
		arg_movingFundsProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return err
}

func (mp *MaintainerProxy) SubmitMovingFundsProofGasEstimate(
	arg_movingFundsTx abi.BitcoinTxInfo3,
	arg_movingFundsProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"submitMovingFundsProof",
		mp.contractABI,
		mp.transactor,
		arg_movingFundsTx,
		arg_movingFundsProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) SubmitRedemptionProof(
	arg_redemptionTx abi.BitcoinTxInfo3,
	arg_redemptionProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction submitRedemptionProof",
		" params: ",
		fmt.Sprint(
			arg_redemptionTx,
			arg_redemptionProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.SubmitRedemptionProof(
		transactorOptions,
		arg_redemptionTx,
		arg_redemptionProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"submitRedemptionProof",
			arg_redemptionTx,
			arg_redemptionProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		)
	}

	mpLogger.Infof(
		"submitted transaction submitRedemptionProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.SubmitRedemptionProof(
				newTransactorOptions,
				arg_redemptionTx,
				arg_redemptionProof,
				arg_mainUtxo,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"submitRedemptionProof",
					arg_redemptionTx,
					arg_redemptionProof,
					arg_mainUtxo,
					arg_walletPubKeyHash,
				)
			}

			mpLogger.Infof(
				"submitted transaction submitRedemptionProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallSubmitRedemptionProof(
	arg_redemptionTx abi.BitcoinTxInfo3,
	arg_redemptionProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitRedemptionProof",
		&result,
		arg_redemptionTx,
		arg_redemptionProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return err
}

func (mp *MaintainerProxy) SubmitRedemptionProofGasEstimate(
	arg_redemptionTx abi.BitcoinTxInfo3,
	arg_redemptionProof abi.BitcoinTxProof2,
	arg_mainUtxo abi.BitcoinTxUTXO2,
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"submitRedemptionProof",
		mp.contractABI,
		mp.transactor,
		arg_redemptionTx,
		arg_redemptionProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) TransferOwnership(
	arg_newOwner common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction transferOwnership",
		" params: ",
		fmt.Sprint(
			arg_newOwner,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.TransferOwnership(
		transactorOptions,
		arg_newOwner,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"transferOwnership",
			arg_newOwner,
		)
	}

	mpLogger.Infof(
		"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.TransferOwnership(
				newTransactorOptions,
				arg_newOwner,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"transferOwnership",
					arg_newOwner,
				)
			}

			mpLogger.Infof(
				"submitted transaction transferOwnership with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallTransferOwnership(
	arg_newOwner common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"transferOwnership",
		&result,
		arg_newOwner,
	)

	return err
}

func (mp *MaintainerProxy) TransferOwnershipGasEstimate(
	arg_newOwner common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"transferOwnership",
		mp.contractABI,
		mp.transactor,
		arg_newOwner,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) UnauthorizeSpvMaintainer(
	arg_maintainerToUnauthorize common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction unauthorizeSpvMaintainer",
		" params: ",
		fmt.Sprint(
			arg_maintainerToUnauthorize,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.UnauthorizeSpvMaintainer(
		transactorOptions,
		arg_maintainerToUnauthorize,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"unauthorizeSpvMaintainer",
			arg_maintainerToUnauthorize,
		)
	}

	mpLogger.Infof(
		"submitted transaction unauthorizeSpvMaintainer with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.UnauthorizeSpvMaintainer(
				newTransactorOptions,
				arg_maintainerToUnauthorize,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"unauthorizeSpvMaintainer",
					arg_maintainerToUnauthorize,
				)
			}

			mpLogger.Infof(
				"submitted transaction unauthorizeSpvMaintainer with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallUnauthorizeSpvMaintainer(
	arg_maintainerToUnauthorize common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"unauthorizeSpvMaintainer",
		&result,
		arg_maintainerToUnauthorize,
	)

	return err
}

func (mp *MaintainerProxy) UnauthorizeSpvMaintainerGasEstimate(
	arg_maintainerToUnauthorize common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"unauthorizeSpvMaintainer",
		mp.contractABI,
		mp.transactor,
		arg_maintainerToUnauthorize,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) UnauthorizeWalletMaintainer(
	arg_maintainerToUnauthorize common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction unauthorizeWalletMaintainer",
		" params: ",
		fmt.Sprint(
			arg_maintainerToUnauthorize,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.UnauthorizeWalletMaintainer(
		transactorOptions,
		arg_maintainerToUnauthorize,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"unauthorizeWalletMaintainer",
			arg_maintainerToUnauthorize,
		)
	}

	mpLogger.Infof(
		"submitted transaction unauthorizeWalletMaintainer with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.UnauthorizeWalletMaintainer(
				newTransactorOptions,
				arg_maintainerToUnauthorize,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"unauthorizeWalletMaintainer",
					arg_maintainerToUnauthorize,
				)
			}

			mpLogger.Infof(
				"submitted transaction unauthorizeWalletMaintainer with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallUnauthorizeWalletMaintainer(
	arg_maintainerToUnauthorize common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"unauthorizeWalletMaintainer",
		&result,
		arg_maintainerToUnauthorize,
	)

	return err
}

func (mp *MaintainerProxy) UnauthorizeWalletMaintainerGasEstimate(
	arg_maintainerToUnauthorize common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"unauthorizeWalletMaintainer",
		mp.contractABI,
		mp.transactor,
		arg_maintainerToUnauthorize,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) UpdateBridge(
	arg__bridge common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction updateBridge",
		" params: ",
		fmt.Sprint(
			arg__bridge,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.UpdateBridge(
		transactorOptions,
		arg__bridge,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"updateBridge",
			arg__bridge,
		)
	}

	mpLogger.Infof(
		"submitted transaction updateBridge with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.UpdateBridge(
				newTransactorOptions,
				arg__bridge,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"updateBridge",
					arg__bridge,
				)
			}

			mpLogger.Infof(
				"submitted transaction updateBridge with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallUpdateBridge(
	arg__bridge common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"updateBridge",
		&result,
		arg__bridge,
	)

	return err
}

func (mp *MaintainerProxy) UpdateBridgeGasEstimate(
	arg__bridge common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"updateBridge",
		mp.contractABI,
		mp.transactor,
		arg__bridge,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) UpdateGasOffsetParameters(
	arg_newSubmitDepositSweepProofGasOffset *big.Int,
	arg_newSubmitRedemptionProofGasOffset *big.Int,
	arg_newResetMovingFundsTimeoutGasOffset *big.Int,
	arg_newSubmitMovingFundsProofGasOffset *big.Int,
	arg_newNotifyMovingFundsBelowDustGasOffset *big.Int,
	arg_newSubmitMovedFundsSweepProofGasOffset *big.Int,
	arg_newRequestNewWalletGasOffset *big.Int,
	arg_newNotifyWalletCloseableGasOffset *big.Int,
	arg_newNotifyWalletClosingPeriodElapsedGasOffset *big.Int,
	arg_newDefeatFraudChallengeGasOffset *big.Int,
	arg_newDefeatFraudChallengeWithHeartbeatGasOffset *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction updateGasOffsetParameters",
		" params: ",
		fmt.Sprint(
			arg_newSubmitDepositSweepProofGasOffset,
			arg_newSubmitRedemptionProofGasOffset,
			arg_newResetMovingFundsTimeoutGasOffset,
			arg_newSubmitMovingFundsProofGasOffset,
			arg_newNotifyMovingFundsBelowDustGasOffset,
			arg_newSubmitMovedFundsSweepProofGasOffset,
			arg_newRequestNewWalletGasOffset,
			arg_newNotifyWalletCloseableGasOffset,
			arg_newNotifyWalletClosingPeriodElapsedGasOffset,
			arg_newDefeatFraudChallengeGasOffset,
			arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.UpdateGasOffsetParameters(
		transactorOptions,
		arg_newSubmitDepositSweepProofGasOffset,
		arg_newSubmitRedemptionProofGasOffset,
		arg_newResetMovingFundsTimeoutGasOffset,
		arg_newSubmitMovingFundsProofGasOffset,
		arg_newNotifyMovingFundsBelowDustGasOffset,
		arg_newSubmitMovedFundsSweepProofGasOffset,
		arg_newRequestNewWalletGasOffset,
		arg_newNotifyWalletCloseableGasOffset,
		arg_newNotifyWalletClosingPeriodElapsedGasOffset,
		arg_newDefeatFraudChallengeGasOffset,
		arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"updateGasOffsetParameters",
			arg_newSubmitDepositSweepProofGasOffset,
			arg_newSubmitRedemptionProofGasOffset,
			arg_newResetMovingFundsTimeoutGasOffset,
			arg_newSubmitMovingFundsProofGasOffset,
			arg_newNotifyMovingFundsBelowDustGasOffset,
			arg_newSubmitMovedFundsSweepProofGasOffset,
			arg_newRequestNewWalletGasOffset,
			arg_newNotifyWalletCloseableGasOffset,
			arg_newNotifyWalletClosingPeriodElapsedGasOffset,
			arg_newDefeatFraudChallengeGasOffset,
			arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
		)
	}

	mpLogger.Infof(
		"submitted transaction updateGasOffsetParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.UpdateGasOffsetParameters(
				newTransactorOptions,
				arg_newSubmitDepositSweepProofGasOffset,
				arg_newSubmitRedemptionProofGasOffset,
				arg_newResetMovingFundsTimeoutGasOffset,
				arg_newSubmitMovingFundsProofGasOffset,
				arg_newNotifyMovingFundsBelowDustGasOffset,
				arg_newSubmitMovedFundsSweepProofGasOffset,
				arg_newRequestNewWalletGasOffset,
				arg_newNotifyWalletCloseableGasOffset,
				arg_newNotifyWalletClosingPeriodElapsedGasOffset,
				arg_newDefeatFraudChallengeGasOffset,
				arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"updateGasOffsetParameters",
					arg_newSubmitDepositSweepProofGasOffset,
					arg_newSubmitRedemptionProofGasOffset,
					arg_newResetMovingFundsTimeoutGasOffset,
					arg_newSubmitMovingFundsProofGasOffset,
					arg_newNotifyMovingFundsBelowDustGasOffset,
					arg_newSubmitMovedFundsSweepProofGasOffset,
					arg_newRequestNewWalletGasOffset,
					arg_newNotifyWalletCloseableGasOffset,
					arg_newNotifyWalletClosingPeriodElapsedGasOffset,
					arg_newDefeatFraudChallengeGasOffset,
					arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
				)
			}

			mpLogger.Infof(
				"submitted transaction updateGasOffsetParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallUpdateGasOffsetParameters(
	arg_newSubmitDepositSweepProofGasOffset *big.Int,
	arg_newSubmitRedemptionProofGasOffset *big.Int,
	arg_newResetMovingFundsTimeoutGasOffset *big.Int,
	arg_newSubmitMovingFundsProofGasOffset *big.Int,
	arg_newNotifyMovingFundsBelowDustGasOffset *big.Int,
	arg_newSubmitMovedFundsSweepProofGasOffset *big.Int,
	arg_newRequestNewWalletGasOffset *big.Int,
	arg_newNotifyWalletCloseableGasOffset *big.Int,
	arg_newNotifyWalletClosingPeriodElapsedGasOffset *big.Int,
	arg_newDefeatFraudChallengeGasOffset *big.Int,
	arg_newDefeatFraudChallengeWithHeartbeatGasOffset *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"updateGasOffsetParameters",
		&result,
		arg_newSubmitDepositSweepProofGasOffset,
		arg_newSubmitRedemptionProofGasOffset,
		arg_newResetMovingFundsTimeoutGasOffset,
		arg_newSubmitMovingFundsProofGasOffset,
		arg_newNotifyMovingFundsBelowDustGasOffset,
		arg_newSubmitMovedFundsSweepProofGasOffset,
		arg_newRequestNewWalletGasOffset,
		arg_newNotifyWalletCloseableGasOffset,
		arg_newNotifyWalletClosingPeriodElapsedGasOffset,
		arg_newDefeatFraudChallengeGasOffset,
		arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
	)

	return err
}

func (mp *MaintainerProxy) UpdateGasOffsetParametersGasEstimate(
	arg_newSubmitDepositSweepProofGasOffset *big.Int,
	arg_newSubmitRedemptionProofGasOffset *big.Int,
	arg_newResetMovingFundsTimeoutGasOffset *big.Int,
	arg_newSubmitMovingFundsProofGasOffset *big.Int,
	arg_newNotifyMovingFundsBelowDustGasOffset *big.Int,
	arg_newSubmitMovedFundsSweepProofGasOffset *big.Int,
	arg_newRequestNewWalletGasOffset *big.Int,
	arg_newNotifyWalletCloseableGasOffset *big.Int,
	arg_newNotifyWalletClosingPeriodElapsedGasOffset *big.Int,
	arg_newDefeatFraudChallengeGasOffset *big.Int,
	arg_newDefeatFraudChallengeWithHeartbeatGasOffset *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"updateGasOffsetParameters",
		mp.contractABI,
		mp.transactor,
		arg_newSubmitDepositSweepProofGasOffset,
		arg_newSubmitRedemptionProofGasOffset,
		arg_newResetMovingFundsTimeoutGasOffset,
		arg_newSubmitMovingFundsProofGasOffset,
		arg_newNotifyMovingFundsBelowDustGasOffset,
		arg_newSubmitMovedFundsSweepProofGasOffset,
		arg_newRequestNewWalletGasOffset,
		arg_newNotifyWalletCloseableGasOffset,
		arg_newNotifyWalletClosingPeriodElapsedGasOffset,
		arg_newDefeatFraudChallengeGasOffset,
		arg_newDefeatFraudChallengeWithHeartbeatGasOffset,
	)

	return result, err
}

// Transaction submission.
func (mp *MaintainerProxy) UpdateReimbursementPool(
	arg__reimbursementPool common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	mpLogger.Debug(
		"submitting transaction updateReimbursementPool",
		" params: ",
		fmt.Sprint(
			arg__reimbursementPool,
		),
	)

	mp.transactionMutex.Lock()
	defer mp.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *mp.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := mp.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := mp.contract.UpdateReimbursementPool(
		transactorOptions,
		arg__reimbursementPool,
	)
	if err != nil {
		return transaction, mp.errorResolver.ResolveError(
			err,
			mp.transactorOptions.From,
			nil,
			"updateReimbursementPool",
			arg__reimbursementPool,
		)
	}

	mpLogger.Infof(
		"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go mp.miningWaiter.ForceMining(
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

			transaction, err := mp.contract.UpdateReimbursementPool(
				newTransactorOptions,
				arg__reimbursementPool,
			)
			if err != nil {
				return nil, mp.errorResolver.ResolveError(
					err,
					mp.transactorOptions.From,
					nil,
					"updateReimbursementPool",
					arg__reimbursementPool,
				)
			}

			mpLogger.Infof(
				"submitted transaction updateReimbursementPool with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	mp.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (mp *MaintainerProxy) CallUpdateReimbursementPool(
	arg__reimbursementPool common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		mp.transactorOptions.From,
		blockNumber, nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"updateReimbursementPool",
		&result,
		arg__reimbursementPool,
	)

	return err
}

func (mp *MaintainerProxy) UpdateReimbursementPoolGasEstimate(
	arg__reimbursementPool common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		mp.callerOptions.From,
		mp.contractAddress,
		"updateReimbursementPool",
		mp.contractABI,
		mp.transactor,
		arg__reimbursementPool,
	)

	return result, err
}

// ----- Const Methods ------

func (mp *MaintainerProxy) AllSpvMaintainers() ([]common.Address, error) {
	result, err := mp.contract.AllSpvMaintainers(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"allSpvMaintainers",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) AllSpvMaintainersAtBlock(
	blockNumber *big.Int,
) ([]common.Address, error) {
	var result []common.Address

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"allSpvMaintainers",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) AllWalletMaintainers() ([]common.Address, error) {
	result, err := mp.contract.AllWalletMaintainers(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"allWalletMaintainers",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) AllWalletMaintainersAtBlock(
	blockNumber *big.Int,
) ([]common.Address, error) {
	var result []common.Address

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"allWalletMaintainers",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) Bridge() (common.Address, error) {
	result, err := mp.contract.Bridge(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"bridge",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) BridgeAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"bridge",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) DefeatFraudChallengeGasOffset() (*big.Int, error) {
	result, err := mp.contract.DefeatFraudChallengeGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"defeatFraudChallengeGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) DefeatFraudChallengeGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"defeatFraudChallengeGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) DefeatFraudChallengeWithHeartbeatGasOffset() (*big.Int, error) {
	result, err := mp.contract.DefeatFraudChallengeWithHeartbeatGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"defeatFraudChallengeWithHeartbeatGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) DefeatFraudChallengeWithHeartbeatGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"defeatFraudChallengeWithHeartbeatGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) IsSpvMaintainer(
	arg0 common.Address,
) (*big.Int, error) {
	result, err := mp.contract.IsSpvMaintainer(
		mp.callerOptions,
		arg0,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"isSpvMaintainer",
			arg0,
		)
	}

	return result, err
}

func (mp *MaintainerProxy) IsSpvMaintainerAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"isSpvMaintainer",
		&result,
		arg0,
	)

	return result, err
}

func (mp *MaintainerProxy) IsWalletMaintainer(
	arg0 common.Address,
) (*big.Int, error) {
	result, err := mp.contract.IsWalletMaintainer(
		mp.callerOptions,
		arg0,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"isWalletMaintainer",
			arg0,
		)
	}

	return result, err
}

func (mp *MaintainerProxy) IsWalletMaintainerAtBlock(
	arg0 common.Address,
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"isWalletMaintainer",
		&result,
		arg0,
	)

	return result, err
}

func (mp *MaintainerProxy) NotifyMovingFundsBelowDustGasOffset() (*big.Int, error) {
	result, err := mp.contract.NotifyMovingFundsBelowDustGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"notifyMovingFundsBelowDustGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) NotifyMovingFundsBelowDustGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"notifyMovingFundsBelowDustGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) NotifyWalletCloseableGasOffset() (*big.Int, error) {
	result, err := mp.contract.NotifyWalletCloseableGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"notifyWalletCloseableGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) NotifyWalletCloseableGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"notifyWalletCloseableGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) NotifyWalletClosingPeriodElapsedGasOffset() (*big.Int, error) {
	result, err := mp.contract.NotifyWalletClosingPeriodElapsedGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"notifyWalletClosingPeriodElapsedGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) NotifyWalletClosingPeriodElapsedGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"notifyWalletClosingPeriodElapsedGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) Owner() (common.Address, error) {
	result, err := mp.contract.Owner(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"owner",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) OwnerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"owner",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) ReimbursementPool() (common.Address, error) {
	result, err := mp.contract.ReimbursementPool(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"reimbursementPool",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) ReimbursementPoolAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"reimbursementPool",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) RequestNewWalletGasOffset() (*big.Int, error) {
	result, err := mp.contract.RequestNewWalletGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"requestNewWalletGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) RequestNewWalletGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"requestNewWalletGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) ResetMovingFundsTimeoutGasOffset() (*big.Int, error) {
	result, err := mp.contract.ResetMovingFundsTimeoutGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"resetMovingFundsTimeoutGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) ResetMovingFundsTimeoutGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"resetMovingFundsTimeoutGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) SpvMaintainers(
	arg0 *big.Int,
) (common.Address, error) {
	result, err := mp.contract.SpvMaintainers(
		mp.callerOptions,
		arg0,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"spvMaintainers",
			arg0,
		)
	}

	return result, err
}

func (mp *MaintainerProxy) SpvMaintainersAtBlock(
	arg0 *big.Int,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"spvMaintainers",
		&result,
		arg0,
	)

	return result, err
}

func (mp *MaintainerProxy) SubmitDepositSweepProofGasOffset() (*big.Int, error) {
	result, err := mp.contract.SubmitDepositSweepProofGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"submitDepositSweepProofGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) SubmitDepositSweepProofGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitDepositSweepProofGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) SubmitMovedFundsSweepProofGasOffset() (*big.Int, error) {
	result, err := mp.contract.SubmitMovedFundsSweepProofGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"submitMovedFundsSweepProofGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) SubmitMovedFundsSweepProofGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitMovedFundsSweepProofGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) SubmitMovingFundsProofGasOffset() (*big.Int, error) {
	result, err := mp.contract.SubmitMovingFundsProofGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"submitMovingFundsProofGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) SubmitMovingFundsProofGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitMovingFundsProofGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) SubmitRedemptionProofGasOffset() (*big.Int, error) {
	result, err := mp.contract.SubmitRedemptionProofGasOffset(
		mp.callerOptions,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"submitRedemptionProofGasOffset",
		)
	}

	return result, err
}

func (mp *MaintainerProxy) SubmitRedemptionProofGasOffsetAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"submitRedemptionProofGasOffset",
		&result,
	)

	return result, err
}

func (mp *MaintainerProxy) WalletMaintainers(
	arg0 *big.Int,
) (common.Address, error) {
	result, err := mp.contract.WalletMaintainers(
		mp.callerOptions,
		arg0,
	)

	if err != nil {
		return result, mp.errorResolver.ResolveError(
			err,
			mp.callerOptions.From,
			nil,
			"walletMaintainers",
			arg0,
		)
	}

	return result, err
}

func (mp *MaintainerProxy) WalletMaintainersAtBlock(
	arg0 *big.Int,
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		mp.callerOptions.From,
		blockNumber,
		nil,
		mp.contractABI,
		mp.caller,
		mp.errorResolver,
		mp.contractAddress,
		"walletMaintainers",
		&result,
		arg0,
	)

	return result, err
}

// ------ Events -------

func (mp *MaintainerProxy) BridgeUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *MpBridgeUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpBridgeUpdatedSubscription{
		mp,
		opts,
	}
}

type MpBridgeUpdatedSubscription struct {
	contract *MaintainerProxy
	opts     *ethereum.SubscribeOpts
}

type maintainerProxyBridgeUpdatedFunc func(
	NewBridge common.Address,
	blockNumber uint64,
)

func (bus *MpBridgeUpdatedSubscription) OnEvent(
	handler maintainerProxyBridgeUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxyBridgeUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.NewBridge,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := bus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (bus *MpBridgeUpdatedSubscription) Pipe(
	sink chan *abi.MaintainerProxyBridgeUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(bus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := bus.contract.blockCounter.CurrentBlock()
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - bus.opts.PastBlocks

				mpLogger.Infof(
					"subscription monitoring fetching past BridgeUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := bus.contract.PastBridgeUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
					"subscription monitoring fetched [%v] past BridgeUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := bus.contract.watchBridgeUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mp *MaintainerProxy) watchBridgeUpdated(
	sink chan *abi.MaintainerProxyBridgeUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchBridgeUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event BridgeUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
			"subscription to event BridgeUpdated failed "+
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

func (mp *MaintainerProxy) PastBridgeUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.MaintainerProxyBridgeUpdated, error) {
	iterator, err := mp.contract.FilterBridgeUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past BridgeUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.MaintainerProxyBridgeUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (mp *MaintainerProxy) GasOffsetParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *MpGasOffsetParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpGasOffsetParametersUpdatedSubscription{
		mp,
		opts,
	}
}

type MpGasOffsetParametersUpdatedSubscription struct {
	contract *MaintainerProxy
	opts     *ethereum.SubscribeOpts
}

type maintainerProxyGasOffsetParametersUpdatedFunc func(
	SubmitDepositSweepProofGasOffset *big.Int,
	SubmitRedemptionProofGasOffset *big.Int,
	ResetMovingFundsTimeoutGasOffset *big.Int,
	SubmitMovingFundsProofGasOffset *big.Int,
	NotifyMovingFundsBelowDustGasOffset *big.Int,
	SubmitMovedFundsSweepProofGasOffset *big.Int,
	RequestNewWalletGasOffset *big.Int,
	NotifyWalletCloseableGasOffset *big.Int,
	NotifyWalletClosingPeriodElapsedGasOffset *big.Int,
	DefeatFraudChallengeGasOffset *big.Int,
	DefeatFraudChallengeWithHeartbeatGasOffset *big.Int,
	blockNumber uint64,
)

func (gopus *MpGasOffsetParametersUpdatedSubscription) OnEvent(
	handler maintainerProxyGasOffsetParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxyGasOffsetParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.SubmitDepositSweepProofGasOffset,
					event.SubmitRedemptionProofGasOffset,
					event.ResetMovingFundsTimeoutGasOffset,
					event.SubmitMovingFundsProofGasOffset,
					event.NotifyMovingFundsBelowDustGasOffset,
					event.SubmitMovedFundsSweepProofGasOffset,
					event.RequestNewWalletGasOffset,
					event.NotifyWalletCloseableGasOffset,
					event.NotifyWalletClosingPeriodElapsedGasOffset,
					event.DefeatFraudChallengeGasOffset,
					event.DefeatFraudChallengeWithHeartbeatGasOffset,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := gopus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (gopus *MpGasOffsetParametersUpdatedSubscription) Pipe(
	sink chan *abi.MaintainerProxyGasOffsetParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(gopus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := gopus.contract.blockCounter.CurrentBlock()
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gopus.opts.PastBlocks

				mpLogger.Infof(
					"subscription monitoring fetching past GasOffsetParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gopus.contract.PastGasOffsetParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
					"subscription monitoring fetched [%v] past GasOffsetParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := gopus.contract.watchGasOffsetParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mp *MaintainerProxy) watchGasOffsetParametersUpdated(
	sink chan *abi.MaintainerProxyGasOffsetParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchGasOffsetParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event GasOffsetParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
			"subscription to event GasOffsetParametersUpdated failed "+
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

func (mp *MaintainerProxy) PastGasOffsetParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.MaintainerProxyGasOffsetParametersUpdated, error) {
	iterator, err := mp.contract.FilterGasOffsetParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past GasOffsetParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.MaintainerProxyGasOffsetParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (mp *MaintainerProxy) OwnershipTransferredEvent(
	opts *ethereum.SubscribeOpts,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) *MpOwnershipTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpOwnershipTransferredSubscription{
		mp,
		opts,
		previousOwnerFilter,
		newOwnerFilter,
	}
}

type MpOwnershipTransferredSubscription struct {
	contract            *MaintainerProxy
	opts                *ethereum.SubscribeOpts
	previousOwnerFilter []common.Address
	newOwnerFilter      []common.Address
}

type maintainerProxyOwnershipTransferredFunc func(
	PreviousOwner common.Address,
	NewOwner common.Address,
	blockNumber uint64,
)

func (ots *MpOwnershipTransferredSubscription) OnEvent(
	handler maintainerProxyOwnershipTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxyOwnershipTransferred)
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

func (ots *MpOwnershipTransferredSubscription) Pipe(
	sink chan *abi.MaintainerProxyOwnershipTransferred,
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
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - ots.opts.PastBlocks

				mpLogger.Infof(
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
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
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

func (mp *MaintainerProxy) watchOwnershipTransferred(
	sink chan *abi.MaintainerProxyOwnershipTransferred,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchOwnershipTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
			previousOwnerFilter,
			newOwnerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event OwnershipTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
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

func (mp *MaintainerProxy) PastOwnershipTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
	previousOwnerFilter []common.Address,
	newOwnerFilter []common.Address,
) ([]*abi.MaintainerProxyOwnershipTransferred, error) {
	iterator, err := mp.contract.FilterOwnershipTransferred(
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

	events := make([]*abi.MaintainerProxyOwnershipTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (mp *MaintainerProxy) ReimbursementPoolUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *MpReimbursementPoolUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpReimbursementPoolUpdatedSubscription{
		mp,
		opts,
	}
}

type MpReimbursementPoolUpdatedSubscription struct {
	contract *MaintainerProxy
	opts     *ethereum.SubscribeOpts
}

type maintainerProxyReimbursementPoolUpdatedFunc func(
	NewReimbursementPool common.Address,
	blockNumber uint64,
)

func (rpus *MpReimbursementPoolUpdatedSubscription) OnEvent(
	handler maintainerProxyReimbursementPoolUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxyReimbursementPoolUpdated)
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

func (rpus *MpReimbursementPoolUpdatedSubscription) Pipe(
	sink chan *abi.MaintainerProxyReimbursementPoolUpdated,
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
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				mpLogger.Infof(
					"subscription monitoring fetching past ReimbursementPoolUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastReimbursementPoolUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
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

func (mp *MaintainerProxy) watchReimbursementPoolUpdated(
	sink chan *abi.MaintainerProxyReimbursementPoolUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchReimbursementPoolUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event ReimbursementPoolUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
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

func (mp *MaintainerProxy) PastReimbursementPoolUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.MaintainerProxyReimbursementPoolUpdated, error) {
	iterator, err := mp.contract.FilterReimbursementPoolUpdated(
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

	events := make([]*abi.MaintainerProxyReimbursementPoolUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (mp *MaintainerProxy) SpvMaintainerAuthorizedEvent(
	opts *ethereum.SubscribeOpts,
	maintainerFilter []common.Address,
) *MpSpvMaintainerAuthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpSpvMaintainerAuthorizedSubscription{
		mp,
		opts,
		maintainerFilter,
	}
}

type MpSpvMaintainerAuthorizedSubscription struct {
	contract         *MaintainerProxy
	opts             *ethereum.SubscribeOpts
	maintainerFilter []common.Address
}

type maintainerProxySpvMaintainerAuthorizedFunc func(
	Maintainer common.Address,
	blockNumber uint64,
)

func (smas *MpSpvMaintainerAuthorizedSubscription) OnEvent(
	handler maintainerProxySpvMaintainerAuthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxySpvMaintainerAuthorized)
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

	sub := smas.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (smas *MpSpvMaintainerAuthorizedSubscription) Pipe(
	sink chan *abi.MaintainerProxySpvMaintainerAuthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(smas.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := smas.contract.blockCounter.CurrentBlock()
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - smas.opts.PastBlocks

				mpLogger.Infof(
					"subscription monitoring fetching past SpvMaintainerAuthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := smas.contract.PastSpvMaintainerAuthorizedEvents(
					fromBlock,
					nil,
					smas.maintainerFilter,
				)
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
					"subscription monitoring fetched [%v] past SpvMaintainerAuthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := smas.contract.watchSpvMaintainerAuthorized(
		sink,
		smas.maintainerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mp *MaintainerProxy) watchSpvMaintainerAuthorized(
	sink chan *abi.MaintainerProxySpvMaintainerAuthorized,
	maintainerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchSpvMaintainerAuthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
			maintainerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event SpvMaintainerAuthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
			"subscription to event SpvMaintainerAuthorized failed "+
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

func (mp *MaintainerProxy) PastSpvMaintainerAuthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
	maintainerFilter []common.Address,
) ([]*abi.MaintainerProxySpvMaintainerAuthorized, error) {
	iterator, err := mp.contract.FilterSpvMaintainerAuthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		maintainerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past SpvMaintainerAuthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.MaintainerProxySpvMaintainerAuthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (mp *MaintainerProxy) SpvMaintainerUnauthorizedEvent(
	opts *ethereum.SubscribeOpts,
	maintainerFilter []common.Address,
) *MpSpvMaintainerUnauthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpSpvMaintainerUnauthorizedSubscription{
		mp,
		opts,
		maintainerFilter,
	}
}

type MpSpvMaintainerUnauthorizedSubscription struct {
	contract         *MaintainerProxy
	opts             *ethereum.SubscribeOpts
	maintainerFilter []common.Address
}

type maintainerProxySpvMaintainerUnauthorizedFunc func(
	Maintainer common.Address,
	blockNumber uint64,
)

func (smus *MpSpvMaintainerUnauthorizedSubscription) OnEvent(
	handler maintainerProxySpvMaintainerUnauthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxySpvMaintainerUnauthorized)
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

	sub := smus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (smus *MpSpvMaintainerUnauthorizedSubscription) Pipe(
	sink chan *abi.MaintainerProxySpvMaintainerUnauthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(smus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := smus.contract.blockCounter.CurrentBlock()
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - smus.opts.PastBlocks

				mpLogger.Infof(
					"subscription monitoring fetching past SpvMaintainerUnauthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := smus.contract.PastSpvMaintainerUnauthorizedEvents(
					fromBlock,
					nil,
					smus.maintainerFilter,
				)
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
					"subscription monitoring fetched [%v] past SpvMaintainerUnauthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := smus.contract.watchSpvMaintainerUnauthorized(
		sink,
		smus.maintainerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mp *MaintainerProxy) watchSpvMaintainerUnauthorized(
	sink chan *abi.MaintainerProxySpvMaintainerUnauthorized,
	maintainerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchSpvMaintainerUnauthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
			maintainerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event SpvMaintainerUnauthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
			"subscription to event SpvMaintainerUnauthorized failed "+
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

func (mp *MaintainerProxy) PastSpvMaintainerUnauthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
	maintainerFilter []common.Address,
) ([]*abi.MaintainerProxySpvMaintainerUnauthorized, error) {
	iterator, err := mp.contract.FilterSpvMaintainerUnauthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		maintainerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past SpvMaintainerUnauthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.MaintainerProxySpvMaintainerUnauthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (mp *MaintainerProxy) WalletMaintainerAuthorizedEvent(
	opts *ethereum.SubscribeOpts,
	maintainerFilter []common.Address,
) *MpWalletMaintainerAuthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpWalletMaintainerAuthorizedSubscription{
		mp,
		opts,
		maintainerFilter,
	}
}

type MpWalletMaintainerAuthorizedSubscription struct {
	contract         *MaintainerProxy
	opts             *ethereum.SubscribeOpts
	maintainerFilter []common.Address
}

type maintainerProxyWalletMaintainerAuthorizedFunc func(
	Maintainer common.Address,
	blockNumber uint64,
)

func (wmas *MpWalletMaintainerAuthorizedSubscription) OnEvent(
	handler maintainerProxyWalletMaintainerAuthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxyWalletMaintainerAuthorized)
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

	sub := wmas.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wmas *MpWalletMaintainerAuthorizedSubscription) Pipe(
	sink chan *abi.MaintainerProxyWalletMaintainerAuthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wmas.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wmas.contract.blockCounter.CurrentBlock()
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wmas.opts.PastBlocks

				mpLogger.Infof(
					"subscription monitoring fetching past WalletMaintainerAuthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wmas.contract.PastWalletMaintainerAuthorizedEvents(
					fromBlock,
					nil,
					wmas.maintainerFilter,
				)
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
					"subscription monitoring fetched [%v] past WalletMaintainerAuthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wmas.contract.watchWalletMaintainerAuthorized(
		sink,
		wmas.maintainerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mp *MaintainerProxy) watchWalletMaintainerAuthorized(
	sink chan *abi.MaintainerProxyWalletMaintainerAuthorized,
	maintainerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchWalletMaintainerAuthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
			maintainerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event WalletMaintainerAuthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
			"subscription to event WalletMaintainerAuthorized failed "+
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

func (mp *MaintainerProxy) PastWalletMaintainerAuthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
	maintainerFilter []common.Address,
) ([]*abi.MaintainerProxyWalletMaintainerAuthorized, error) {
	iterator, err := mp.contract.FilterWalletMaintainerAuthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		maintainerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletMaintainerAuthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.MaintainerProxyWalletMaintainerAuthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (mp *MaintainerProxy) WalletMaintainerUnauthorizedEvent(
	opts *ethereum.SubscribeOpts,
	maintainerFilter []common.Address,
) *MpWalletMaintainerUnauthorizedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &MpWalletMaintainerUnauthorizedSubscription{
		mp,
		opts,
		maintainerFilter,
	}
}

type MpWalletMaintainerUnauthorizedSubscription struct {
	contract         *MaintainerProxy
	opts             *ethereum.SubscribeOpts
	maintainerFilter []common.Address
}

type maintainerProxyWalletMaintainerUnauthorizedFunc func(
	Maintainer common.Address,
	blockNumber uint64,
)

func (wmus *MpWalletMaintainerUnauthorizedSubscription) OnEvent(
	handler maintainerProxyWalletMaintainerUnauthorizedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.MaintainerProxyWalletMaintainerUnauthorized)
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

	sub := wmus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wmus *MpWalletMaintainerUnauthorizedSubscription) Pipe(
	sink chan *abi.MaintainerProxyWalletMaintainerUnauthorized,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wmus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wmus.contract.blockCounter.CurrentBlock()
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wmus.opts.PastBlocks

				mpLogger.Infof(
					"subscription monitoring fetching past WalletMaintainerUnauthorized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wmus.contract.PastWalletMaintainerUnauthorizedEvents(
					fromBlock,
					nil,
					wmus.maintainerFilter,
				)
				if err != nil {
					mpLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				mpLogger.Infof(
					"subscription monitoring fetched [%v] past WalletMaintainerUnauthorized events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wmus.contract.watchWalletMaintainerUnauthorized(
		sink,
		wmus.maintainerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mp *MaintainerProxy) watchWalletMaintainerUnauthorized(
	sink chan *abi.MaintainerProxyWalletMaintainerUnauthorized,
	maintainerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return mp.contract.WatchWalletMaintainerUnauthorized(
			&bind.WatchOpts{Context: ctx},
			sink,
			maintainerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		mpLogger.Warnf(
			"subscription to event WalletMaintainerUnauthorized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		mpLogger.Errorf(
			"subscription to event WalletMaintainerUnauthorized failed "+
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

func (mp *MaintainerProxy) PastWalletMaintainerUnauthorizedEvents(
	startBlock uint64,
	endBlock *uint64,
	maintainerFilter []common.Address,
) ([]*abi.MaintainerProxyWalletMaintainerUnauthorized, error) {
	iterator, err := mp.contract.FilterWalletMaintainerUnauthorized(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		maintainerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletMaintainerUnauthorized events: [%v]",
			err,
		)
	}

	events := make([]*abi.MaintainerProxyWalletMaintainerUnauthorized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

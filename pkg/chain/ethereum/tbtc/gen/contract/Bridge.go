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
var bLogger = log.Logger("keep-contract-Bridge")

type Bridge struct {
	contract          *abi.Bridge
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

func NewBridge(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*Bridge, error) {
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

	contract, err := abi.NewBridge(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.BridgeABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &Bridge{
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
func (b *Bridge) DefeatFraudChallenge(
	arg_walletPublicKey []byte,
	arg_preimage []byte,
	arg_witness bool,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction defeatFraudChallenge",
		" params: ",
		fmt.Sprint(
			arg_walletPublicKey,
			arg_preimage,
			arg_witness,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.DefeatFraudChallenge(
		transactorOptions,
		arg_walletPublicKey,
		arg_preimage,
		arg_witness,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"defeatFraudChallenge",
			arg_walletPublicKey,
			arg_preimage,
			arg_witness,
		)
	}

	bLogger.Infof(
		"submitted transaction defeatFraudChallenge with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.DefeatFraudChallenge(
				newTransactorOptions,
				arg_walletPublicKey,
				arg_preimage,
				arg_witness,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"defeatFraudChallenge",
					arg_walletPublicKey,
					arg_preimage,
					arg_witness,
				)
			}

			bLogger.Infof(
				"submitted transaction defeatFraudChallenge with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallDefeatFraudChallenge(
	arg_walletPublicKey []byte,
	arg_preimage []byte,
	arg_witness bool,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"defeatFraudChallenge",
		&result,
		arg_walletPublicKey,
		arg_preimage,
		arg_witness,
	)

	return err
}

func (b *Bridge) DefeatFraudChallengeGasEstimate(
	arg_walletPublicKey []byte,
	arg_preimage []byte,
	arg_witness bool,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"defeatFraudChallenge",
		b.contractABI,
		b.transactor,
		arg_walletPublicKey,
		arg_preimage,
		arg_witness,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) DefeatFraudChallengeWithHeartbeat(
	arg_walletPublicKey []byte,
	arg_heartbeatMessage []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction defeatFraudChallengeWithHeartbeat",
		" params: ",
		fmt.Sprint(
			arg_walletPublicKey,
			arg_heartbeatMessage,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.DefeatFraudChallengeWithHeartbeat(
		transactorOptions,
		arg_walletPublicKey,
		arg_heartbeatMessage,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"defeatFraudChallengeWithHeartbeat",
			arg_walletPublicKey,
			arg_heartbeatMessage,
		)
	}

	bLogger.Infof(
		"submitted transaction defeatFraudChallengeWithHeartbeat with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.DefeatFraudChallengeWithHeartbeat(
				newTransactorOptions,
				arg_walletPublicKey,
				arg_heartbeatMessage,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"defeatFraudChallengeWithHeartbeat",
					arg_walletPublicKey,
					arg_heartbeatMessage,
				)
			}

			bLogger.Infof(
				"submitted transaction defeatFraudChallengeWithHeartbeat with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallDefeatFraudChallengeWithHeartbeat(
	arg_walletPublicKey []byte,
	arg_heartbeatMessage []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"defeatFraudChallengeWithHeartbeat",
		&result,
		arg_walletPublicKey,
		arg_heartbeatMessage,
	)

	return err
}

func (b *Bridge) DefeatFraudChallengeWithHeartbeatGasEstimate(
	arg_walletPublicKey []byte,
	arg_heartbeatMessage []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"defeatFraudChallengeWithHeartbeat",
		b.contractABI,
		b.transactor,
		arg_walletPublicKey,
		arg_heartbeatMessage,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) EcdsaWalletCreatedCallback(
	arg_ecdsaWalletID [32]byte,
	arg_publicKeyX [32]byte,
	arg_publicKeyY [32]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction ecdsaWalletCreatedCallback",
		" params: ",
		fmt.Sprint(
			arg_ecdsaWalletID,
			arg_publicKeyX,
			arg_publicKeyY,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.EcdsaWalletCreatedCallback(
		transactorOptions,
		arg_ecdsaWalletID,
		arg_publicKeyX,
		arg_publicKeyY,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"ecdsaWalletCreatedCallback",
			arg_ecdsaWalletID,
			arg_publicKeyX,
			arg_publicKeyY,
		)
	}

	bLogger.Infof(
		"submitted transaction ecdsaWalletCreatedCallback with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.EcdsaWalletCreatedCallback(
				newTransactorOptions,
				arg_ecdsaWalletID,
				arg_publicKeyX,
				arg_publicKeyY,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"ecdsaWalletCreatedCallback",
					arg_ecdsaWalletID,
					arg_publicKeyX,
					arg_publicKeyY,
				)
			}

			bLogger.Infof(
				"submitted transaction ecdsaWalletCreatedCallback with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallEcdsaWalletCreatedCallback(
	arg_ecdsaWalletID [32]byte,
	arg_publicKeyX [32]byte,
	arg_publicKeyY [32]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"ecdsaWalletCreatedCallback",
		&result,
		arg_ecdsaWalletID,
		arg_publicKeyX,
		arg_publicKeyY,
	)

	return err
}

func (b *Bridge) EcdsaWalletCreatedCallbackGasEstimate(
	arg_ecdsaWalletID [32]byte,
	arg_publicKeyX [32]byte,
	arg_publicKeyY [32]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"ecdsaWalletCreatedCallback",
		b.contractABI,
		b.transactor,
		arg_ecdsaWalletID,
		arg_publicKeyX,
		arg_publicKeyY,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) EcdsaWalletHeartbeatFailedCallback(
	arg0 [32]byte,
	arg_publicKeyX [32]byte,
	arg_publicKeyY [32]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction ecdsaWalletHeartbeatFailedCallback",
		" params: ",
		fmt.Sprint(
			arg0,
			arg_publicKeyX,
			arg_publicKeyY,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.EcdsaWalletHeartbeatFailedCallback(
		transactorOptions,
		arg0,
		arg_publicKeyX,
		arg_publicKeyY,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"ecdsaWalletHeartbeatFailedCallback",
			arg0,
			arg_publicKeyX,
			arg_publicKeyY,
		)
	}

	bLogger.Infof(
		"submitted transaction ecdsaWalletHeartbeatFailedCallback with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.EcdsaWalletHeartbeatFailedCallback(
				newTransactorOptions,
				arg0,
				arg_publicKeyX,
				arg_publicKeyY,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"ecdsaWalletHeartbeatFailedCallback",
					arg0,
					arg_publicKeyX,
					arg_publicKeyY,
				)
			}

			bLogger.Infof(
				"submitted transaction ecdsaWalletHeartbeatFailedCallback with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallEcdsaWalletHeartbeatFailedCallback(
	arg0 [32]byte,
	arg_publicKeyX [32]byte,
	arg_publicKeyY [32]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"ecdsaWalletHeartbeatFailedCallback",
		&result,
		arg0,
		arg_publicKeyX,
		arg_publicKeyY,
	)

	return err
}

func (b *Bridge) EcdsaWalletHeartbeatFailedCallbackGasEstimate(
	arg0 [32]byte,
	arg_publicKeyX [32]byte,
	arg_publicKeyY [32]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"ecdsaWalletHeartbeatFailedCallback",
		b.contractABI,
		b.transactor,
		arg0,
		arg_publicKeyX,
		arg_publicKeyY,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) Initialize(
	arg__bank common.Address,
	arg__relay common.Address,
	arg__treasury common.Address,
	arg__ecdsaWalletRegistry common.Address,
	arg__reimbursementPool common.Address,
	arg__txProofDifficultyFactor *big.Int,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction initialize",
		" params: ",
		fmt.Sprint(
			arg__bank,
			arg__relay,
			arg__treasury,
			arg__ecdsaWalletRegistry,
			arg__reimbursementPool,
			arg__txProofDifficultyFactor,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.Initialize(
		transactorOptions,
		arg__bank,
		arg__relay,
		arg__treasury,
		arg__ecdsaWalletRegistry,
		arg__reimbursementPool,
		arg__txProofDifficultyFactor,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"initialize",
			arg__bank,
			arg__relay,
			arg__treasury,
			arg__ecdsaWalletRegistry,
			arg__reimbursementPool,
			arg__txProofDifficultyFactor,
		)
	}

	bLogger.Infof(
		"submitted transaction initialize with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.Initialize(
				newTransactorOptions,
				arg__bank,
				arg__relay,
				arg__treasury,
				arg__ecdsaWalletRegistry,
				arg__reimbursementPool,
				arg__txProofDifficultyFactor,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"initialize",
					arg__bank,
					arg__relay,
					arg__treasury,
					arg__ecdsaWalletRegistry,
					arg__reimbursementPool,
					arg__txProofDifficultyFactor,
				)
			}

			bLogger.Infof(
				"submitted transaction initialize with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallInitialize(
	arg__bank common.Address,
	arg__relay common.Address,
	arg__treasury common.Address,
	arg__ecdsaWalletRegistry common.Address,
	arg__reimbursementPool common.Address,
	arg__txProofDifficultyFactor *big.Int,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"initialize",
		&result,
		arg__bank,
		arg__relay,
		arg__treasury,
		arg__ecdsaWalletRegistry,
		arg__reimbursementPool,
		arg__txProofDifficultyFactor,
	)

	return err
}

func (b *Bridge) InitializeGasEstimate(
	arg__bank common.Address,
	arg__relay common.Address,
	arg__treasury common.Address,
	arg__ecdsaWalletRegistry common.Address,
	arg__reimbursementPool common.Address,
	arg__txProofDifficultyFactor *big.Int,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"initialize",
		b.contractABI,
		b.transactor,
		arg__bank,
		arg__relay,
		arg__treasury,
		arg__ecdsaWalletRegistry,
		arg__reimbursementPool,
		arg__txProofDifficultyFactor,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) InitializeV2FixVaultZeroDeposit(

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction initializeV2FixVaultZeroDeposit",
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.InitializeV2FixVaultZeroDeposit(
		transactorOptions,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"initializeV2FixVaultZeroDeposit",
		)
	}

	bLogger.Infof(
		"submitted transaction initializeV2FixVaultZeroDeposit with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.InitializeV2FixVaultZeroDeposit(
				newTransactorOptions,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"initializeV2FixVaultZeroDeposit",
				)
			}

			bLogger.Infof(
				"submitted transaction initializeV2FixVaultZeroDeposit with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallInitializeV2FixVaultZeroDeposit(
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"initializeV2FixVaultZeroDeposit",
		&result,
	)

	return err
}

func (b *Bridge) InitializeV2FixVaultZeroDepositGasEstimate() (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"initializeV2FixVaultZeroDeposit",
		b.contractABI,
		b.transactor,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) InitializeV5RepairRebateStaking(
	arg_newRebateStaking common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction initializeV5RepairRebateStaking",
		" params: ",
		fmt.Sprint(
			arg_newRebateStaking,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.InitializeV5RepairRebateStaking(
		transactorOptions,
		arg_newRebateStaking,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"initializeV5RepairRebateStaking",
			arg_newRebateStaking,
		)
	}

	bLogger.Infof(
		"submitted transaction initializeV5RepairRebateStaking with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.InitializeV5RepairRebateStaking(
				newTransactorOptions,
				arg_newRebateStaking,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"initializeV5RepairRebateStaking",
					arg_newRebateStaking,
				)
			}

			bLogger.Infof(
				"submitted transaction initializeV5RepairRebateStaking with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallInitializeV5RepairRebateStaking(
	arg_newRebateStaking common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"initializeV5RepairRebateStaking",
		&result,
		arg_newRebateStaking,
	)

	return err
}

func (b *Bridge) InitializeV5RepairRebateStakingGasEstimate(
	arg_newRebateStaking common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"initializeV5RepairRebateStaking",
		b.contractABI,
		b.transactor,
		arg_newRebateStaking,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyFraudChallengeDefeatTimeout(
	arg_walletPublicKey []byte,
	arg_walletMembersIDs []uint32,
	arg_preimageSha256 []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyFraudChallengeDefeatTimeout",
		" params: ",
		fmt.Sprint(
			arg_walletPublicKey,
			arg_walletMembersIDs,
			arg_preimageSha256,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyFraudChallengeDefeatTimeout(
		transactorOptions,
		arg_walletPublicKey,
		arg_walletMembersIDs,
		arg_preimageSha256,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyFraudChallengeDefeatTimeout",
			arg_walletPublicKey,
			arg_walletMembersIDs,
			arg_preimageSha256,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyFraudChallengeDefeatTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyFraudChallengeDefeatTimeout(
				newTransactorOptions,
				arg_walletPublicKey,
				arg_walletMembersIDs,
				arg_preimageSha256,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyFraudChallengeDefeatTimeout",
					arg_walletPublicKey,
					arg_walletMembersIDs,
					arg_preimageSha256,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyFraudChallengeDefeatTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyFraudChallengeDefeatTimeout(
	arg_walletPublicKey []byte,
	arg_walletMembersIDs []uint32,
	arg_preimageSha256 []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyFraudChallengeDefeatTimeout",
		&result,
		arg_walletPublicKey,
		arg_walletMembersIDs,
		arg_preimageSha256,
	)

	return err
}

func (b *Bridge) NotifyFraudChallengeDefeatTimeoutGasEstimate(
	arg_walletPublicKey []byte,
	arg_walletMembersIDs []uint32,
	arg_preimageSha256 []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyFraudChallengeDefeatTimeout",
		b.contractABI,
		b.transactor,
		arg_walletPublicKey,
		arg_walletMembersIDs,
		arg_preimageSha256,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyMovedFundsSweepTimeout(
	arg_movingFundsTxHash [32]byte,
	arg_movingFundsTxOutputIndex uint32,
	arg_walletMembersIDs []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyMovedFundsSweepTimeout",
		" params: ",
		fmt.Sprint(
			arg_movingFundsTxHash,
			arg_movingFundsTxOutputIndex,
			arg_walletMembersIDs,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyMovedFundsSweepTimeout(
		transactorOptions,
		arg_movingFundsTxHash,
		arg_movingFundsTxOutputIndex,
		arg_walletMembersIDs,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyMovedFundsSweepTimeout",
			arg_movingFundsTxHash,
			arg_movingFundsTxOutputIndex,
			arg_walletMembersIDs,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyMovedFundsSweepTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyMovedFundsSweepTimeout(
				newTransactorOptions,
				arg_movingFundsTxHash,
				arg_movingFundsTxOutputIndex,
				arg_walletMembersIDs,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyMovedFundsSweepTimeout",
					arg_movingFundsTxHash,
					arg_movingFundsTxOutputIndex,
					arg_walletMembersIDs,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyMovedFundsSweepTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyMovedFundsSweepTimeout(
	arg_movingFundsTxHash [32]byte,
	arg_movingFundsTxOutputIndex uint32,
	arg_walletMembersIDs []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyMovedFundsSweepTimeout",
		&result,
		arg_movingFundsTxHash,
		arg_movingFundsTxOutputIndex,
		arg_walletMembersIDs,
	)

	return err
}

func (b *Bridge) NotifyMovedFundsSweepTimeoutGasEstimate(
	arg_movingFundsTxHash [32]byte,
	arg_movingFundsTxOutputIndex uint32,
	arg_walletMembersIDs []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyMovedFundsSweepTimeout",
		b.contractABI,
		b.transactor,
		arg_movingFundsTxHash,
		arg_movingFundsTxOutputIndex,
		arg_walletMembersIDs,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyMovingFundsBelowDust(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyMovingFundsBelowDust",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_mainUtxo,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyMovingFundsBelowDust(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_mainUtxo,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyMovingFundsBelowDust",
			arg_walletPubKeyHash,
			arg_mainUtxo,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyMovingFundsBelowDust with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyMovingFundsBelowDust(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_mainUtxo,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyMovingFundsBelowDust",
					arg_walletPubKeyHash,
					arg_mainUtxo,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyMovingFundsBelowDust with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyMovingFundsBelowDust(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyMovingFundsBelowDust",
		&result,
		arg_walletPubKeyHash,
		arg_mainUtxo,
	)

	return err
}

func (b *Bridge) NotifyMovingFundsBelowDustGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyMovingFundsBelowDust",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
		arg_mainUtxo,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyMovingFundsTimeout(
	arg_walletPubKeyHash [20]byte,
	arg_walletMembersIDs []uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyMovingFundsTimeout",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_walletMembersIDs,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyMovingFundsTimeout(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_walletMembersIDs,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyMovingFundsTimeout",
			arg_walletPubKeyHash,
			arg_walletMembersIDs,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyMovingFundsTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyMovingFundsTimeout(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_walletMembersIDs,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyMovingFundsTimeout",
					arg_walletPubKeyHash,
					arg_walletMembersIDs,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyMovingFundsTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyMovingFundsTimeout(
	arg_walletPubKeyHash [20]byte,
	arg_walletMembersIDs []uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyMovingFundsTimeout",
		&result,
		arg_walletPubKeyHash,
		arg_walletMembersIDs,
	)

	return err
}

func (b *Bridge) NotifyMovingFundsTimeoutGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_walletMembersIDs []uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyMovingFundsTimeout",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
		arg_walletMembersIDs,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyRedemptionTimeout(
	arg_walletPubKeyHash [20]byte,
	arg_walletMembersIDs []uint32,
	arg_redeemerOutputScript []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyRedemptionTimeout",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_walletMembersIDs,
			arg_redeemerOutputScript,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyRedemptionTimeout(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_walletMembersIDs,
		arg_redeemerOutputScript,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyRedemptionTimeout",
			arg_walletPubKeyHash,
			arg_walletMembersIDs,
			arg_redeemerOutputScript,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyRedemptionTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyRedemptionTimeout(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_walletMembersIDs,
				arg_redeemerOutputScript,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyRedemptionTimeout",
					arg_walletPubKeyHash,
					arg_walletMembersIDs,
					arg_redeemerOutputScript,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyRedemptionTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyRedemptionTimeout(
	arg_walletPubKeyHash [20]byte,
	arg_walletMembersIDs []uint32,
	arg_redeemerOutputScript []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyRedemptionTimeout",
		&result,
		arg_walletPubKeyHash,
		arg_walletMembersIDs,
		arg_redeemerOutputScript,
	)

	return err
}

func (b *Bridge) NotifyRedemptionTimeoutGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_walletMembersIDs []uint32,
	arg_redeemerOutputScript []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyRedemptionTimeout",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
		arg_walletMembersIDs,
		arg_redeemerOutputScript,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyRedemptionVeto(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyRedemptionVeto",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyRedemptionVeto(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyRedemptionVeto",
			arg_walletPubKeyHash,
			arg_redeemerOutputScript,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyRedemptionVeto with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyRedemptionVeto(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_redeemerOutputScript,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyRedemptionVeto",
					arg_walletPubKeyHash,
					arg_redeemerOutputScript,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyRedemptionVeto with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyRedemptionVeto(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyRedemptionVeto",
		&result,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
	)

	return err
}

func (b *Bridge) NotifyRedemptionVetoGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_redeemerOutputScript []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyRedemptionVeto",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
		arg_redeemerOutputScript,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyWalletCloseable(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyWalletCloseable",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_walletMainUtxo,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyWalletCloseable(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyWalletCloseable",
			arg_walletPubKeyHash,
			arg_walletMainUtxo,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyWalletCloseable with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyWalletCloseable(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_walletMainUtxo,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyWalletCloseable",
					arg_walletPubKeyHash,
					arg_walletMainUtxo,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyWalletCloseable with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyWalletCloseable(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyWalletCloseable",
		&result,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
	)

	return err
}

func (b *Bridge) NotifyWalletCloseableGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyWalletCloseable",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) NotifyWalletClosingPeriodElapsed(
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction notifyWalletClosingPeriodElapsed",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.NotifyWalletClosingPeriodElapsed(
		transactorOptions,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"notifyWalletClosingPeriodElapsed",
			arg_walletPubKeyHash,
		)
	}

	bLogger.Infof(
		"submitted transaction notifyWalletClosingPeriodElapsed with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.NotifyWalletClosingPeriodElapsed(
				newTransactorOptions,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"notifyWalletClosingPeriodElapsed",
					arg_walletPubKeyHash,
				)
			}

			bLogger.Infof(
				"submitted transaction notifyWalletClosingPeriodElapsed with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallNotifyWalletClosingPeriodElapsed(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"notifyWalletClosingPeriodElapsed",
		&result,
		arg_walletPubKeyHash,
	)

	return err
}

func (b *Bridge) NotifyWalletClosingPeriodElapsedGasEstimate(
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"notifyWalletClosingPeriodElapsed",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) ReceiveBalanceApproval(
	arg_balanceOwner common.Address,
	arg_amount *big.Int,
	arg_redemptionData []byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction receiveBalanceApproval",
		" params: ",
		fmt.Sprint(
			arg_balanceOwner,
			arg_amount,
			arg_redemptionData,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.ReceiveBalanceApproval(
		transactorOptions,
		arg_balanceOwner,
		arg_amount,
		arg_redemptionData,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"receiveBalanceApproval",
			arg_balanceOwner,
			arg_amount,
			arg_redemptionData,
		)
	}

	bLogger.Infof(
		"submitted transaction receiveBalanceApproval with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.ReceiveBalanceApproval(
				newTransactorOptions,
				arg_balanceOwner,
				arg_amount,
				arg_redemptionData,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"receiveBalanceApproval",
					arg_balanceOwner,
					arg_amount,
					arg_redemptionData,
				)
			}

			bLogger.Infof(
				"submitted transaction receiveBalanceApproval with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallReceiveBalanceApproval(
	arg_balanceOwner common.Address,
	arg_amount *big.Int,
	arg_redemptionData []byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"receiveBalanceApproval",
		&result,
		arg_balanceOwner,
		arg_amount,
		arg_redemptionData,
	)

	return err
}

func (b *Bridge) ReceiveBalanceApprovalGasEstimate(
	arg_balanceOwner common.Address,
	arg_amount *big.Int,
	arg_redemptionData []byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"receiveBalanceApproval",
		b.contractABI,
		b.transactor,
		arg_balanceOwner,
		arg_amount,
		arg_redemptionData,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) RequestNewWallet(
	arg_activeWalletMainUtxo abi.BitcoinTxUTXO,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction requestNewWallet",
		" params: ",
		fmt.Sprint(
			arg_activeWalletMainUtxo,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.RequestNewWallet(
		transactorOptions,
		arg_activeWalletMainUtxo,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"requestNewWallet",
			arg_activeWalletMainUtxo,
		)
	}

	bLogger.Infof(
		"submitted transaction requestNewWallet with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.RequestNewWallet(
				newTransactorOptions,
				arg_activeWalletMainUtxo,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"requestNewWallet",
					arg_activeWalletMainUtxo,
				)
			}

			bLogger.Infof(
				"submitted transaction requestNewWallet with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallRequestNewWallet(
	arg_activeWalletMainUtxo abi.BitcoinTxUTXO,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"requestNewWallet",
		&result,
		arg_activeWalletMainUtxo,
	)

	return err
}

func (b *Bridge) RequestNewWalletGasEstimate(
	arg_activeWalletMainUtxo abi.BitcoinTxUTXO,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"requestNewWallet",
		b.contractABI,
		b.transactor,
		arg_activeWalletMainUtxo,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) RequestRedemption(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_redeemerOutputScript []byte,
	arg_amount uint64,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction requestRedemption",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_mainUtxo,
			arg_redeemerOutputScript,
			arg_amount,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.RequestRedemption(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_mainUtxo,
		arg_redeemerOutputScript,
		arg_amount,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"requestRedemption",
			arg_walletPubKeyHash,
			arg_mainUtxo,
			arg_redeemerOutputScript,
			arg_amount,
		)
	}

	bLogger.Infof(
		"submitted transaction requestRedemption with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.RequestRedemption(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_mainUtxo,
				arg_redeemerOutputScript,
				arg_amount,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"requestRedemption",
					arg_walletPubKeyHash,
					arg_mainUtxo,
					arg_redeemerOutputScript,
					arg_amount,
				)
			}

			bLogger.Infof(
				"submitted transaction requestRedemption with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallRequestRedemption(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_redeemerOutputScript []byte,
	arg_amount uint64,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"requestRedemption",
		&result,
		arg_walletPubKeyHash,
		arg_mainUtxo,
		arg_redeemerOutputScript,
		arg_amount,
	)

	return err
}

func (b *Bridge) RequestRedemptionGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_redeemerOutputScript []byte,
	arg_amount uint64,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"requestRedemption",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
		arg_mainUtxo,
		arg_redeemerOutputScript,
		arg_amount,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) ResetMovingFundsTimeout(
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction resetMovingFundsTimeout",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.ResetMovingFundsTimeout(
		transactorOptions,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"resetMovingFundsTimeout",
			arg_walletPubKeyHash,
		)
	}

	bLogger.Infof(
		"submitted transaction resetMovingFundsTimeout with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.ResetMovingFundsTimeout(
				newTransactorOptions,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"resetMovingFundsTimeout",
					arg_walletPubKeyHash,
				)
			}

			bLogger.Infof(
				"submitted transaction resetMovingFundsTimeout with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallResetMovingFundsTimeout(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"resetMovingFundsTimeout",
		&result,
		arg_walletPubKeyHash,
	)

	return err
}

func (b *Bridge) ResetMovingFundsTimeoutGasEstimate(
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"resetMovingFundsTimeout",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) RevealDeposit(
	arg_fundingTx abi.BitcoinTxInfo,
	arg_reveal abi.DepositDepositRevealInfo,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction revealDeposit",
		" params: ",
		fmt.Sprint(
			arg_fundingTx,
			arg_reveal,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.RevealDeposit(
		transactorOptions,
		arg_fundingTx,
		arg_reveal,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"revealDeposit",
			arg_fundingTx,
			arg_reveal,
		)
	}

	bLogger.Infof(
		"submitted transaction revealDeposit with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.RevealDeposit(
				newTransactorOptions,
				arg_fundingTx,
				arg_reveal,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"revealDeposit",
					arg_fundingTx,
					arg_reveal,
				)
			}

			bLogger.Infof(
				"submitted transaction revealDeposit with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallRevealDeposit(
	arg_fundingTx abi.BitcoinTxInfo,
	arg_reveal abi.DepositDepositRevealInfo,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"revealDeposit",
		&result,
		arg_fundingTx,
		arg_reveal,
	)

	return err
}

func (b *Bridge) RevealDepositGasEstimate(
	arg_fundingTx abi.BitcoinTxInfo,
	arg_reveal abi.DepositDepositRevealInfo,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"revealDeposit",
		b.contractABI,
		b.transactor,
		arg_fundingTx,
		arg_reveal,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) RevealDepositWithExtraData(
	arg_fundingTx abi.BitcoinTxInfo,
	arg_reveal abi.DepositDepositRevealInfo,
	arg_extraData [32]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction revealDepositWithExtraData",
		" params: ",
		fmt.Sprint(
			arg_fundingTx,
			arg_reveal,
			arg_extraData,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.RevealDepositWithExtraData(
		transactorOptions,
		arg_fundingTx,
		arg_reveal,
		arg_extraData,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"revealDepositWithExtraData",
			arg_fundingTx,
			arg_reveal,
			arg_extraData,
		)
	}

	bLogger.Infof(
		"submitted transaction revealDepositWithExtraData with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.RevealDepositWithExtraData(
				newTransactorOptions,
				arg_fundingTx,
				arg_reveal,
				arg_extraData,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"revealDepositWithExtraData",
					arg_fundingTx,
					arg_reveal,
					arg_extraData,
				)
			}

			bLogger.Infof(
				"submitted transaction revealDepositWithExtraData with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallRevealDepositWithExtraData(
	arg_fundingTx abi.BitcoinTxInfo,
	arg_reveal abi.DepositDepositRevealInfo,
	arg_extraData [32]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"revealDepositWithExtraData",
		&result,
		arg_fundingTx,
		arg_reveal,
		arg_extraData,
	)

	return err
}

func (b *Bridge) RevealDepositWithExtraDataGasEstimate(
	arg_fundingTx abi.BitcoinTxInfo,
	arg_reveal abi.DepositDepositRevealInfo,
	arg_extraData [32]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"revealDepositWithExtraData",
		b.contractABI,
		b.transactor,
		arg_fundingTx,
		arg_reveal,
		arg_extraData,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SetRebateStaking(
	arg_rebateStaking common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction setRebateStaking",
		" params: ",
		fmt.Sprint(
			arg_rebateStaking,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SetRebateStaking(
		transactorOptions,
		arg_rebateStaking,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"setRebateStaking",
			arg_rebateStaking,
		)
	}

	bLogger.Infof(
		"submitted transaction setRebateStaking with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SetRebateStaking(
				newTransactorOptions,
				arg_rebateStaking,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"setRebateStaking",
					arg_rebateStaking,
				)
			}

			bLogger.Infof(
				"submitted transaction setRebateStaking with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSetRebateStaking(
	arg_rebateStaking common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"setRebateStaking",
		&result,
		arg_rebateStaking,
	)

	return err
}

func (b *Bridge) SetRebateStakingGasEstimate(
	arg_rebateStaking common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"setRebateStaking",
		b.contractABI,
		b.transactor,
		arg_rebateStaking,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SetRedemptionWatchtower(
	arg_redemptionWatchtower common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction setRedemptionWatchtower",
		" params: ",
		fmt.Sprint(
			arg_redemptionWatchtower,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SetRedemptionWatchtower(
		transactorOptions,
		arg_redemptionWatchtower,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"setRedemptionWatchtower",
			arg_redemptionWatchtower,
		)
	}

	bLogger.Infof(
		"submitted transaction setRedemptionWatchtower with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SetRedemptionWatchtower(
				newTransactorOptions,
				arg_redemptionWatchtower,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"setRedemptionWatchtower",
					arg_redemptionWatchtower,
				)
			}

			bLogger.Infof(
				"submitted transaction setRedemptionWatchtower with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSetRedemptionWatchtower(
	arg_redemptionWatchtower common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"setRedemptionWatchtower",
		&result,
		arg_redemptionWatchtower,
	)

	return err
}

func (b *Bridge) SetRedemptionWatchtowerGasEstimate(
	arg_redemptionWatchtower common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"setRedemptionWatchtower",
		b.contractABI,
		b.transactor,
		arg_redemptionWatchtower,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SetReservationRouter(
	arg__reservationRouter common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction setReservationRouter",
		" params: ",
		fmt.Sprint(
			arg__reservationRouter,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SetReservationRouter(
		transactorOptions,
		arg__reservationRouter,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"setReservationRouter",
			arg__reservationRouter,
		)
	}

	bLogger.Infof(
		"submitted transaction setReservationRouter with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SetReservationRouter(
				newTransactorOptions,
				arg__reservationRouter,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"setReservationRouter",
					arg__reservationRouter,
				)
			}

			bLogger.Infof(
				"submitted transaction setReservationRouter with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSetReservationRouter(
	arg__reservationRouter common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"setReservationRouter",
		&result,
		arg__reservationRouter,
	)

	return err
}

func (b *Bridge) SetReservationRouterGasEstimate(
	arg__reservationRouter common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"setReservationRouter",
		b.contractABI,
		b.transactor,
		arg__reservationRouter,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SetSpvMaintainerStatus(
	arg_spvMaintainer common.Address,
	arg_isTrusted bool,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction setSpvMaintainerStatus",
		" params: ",
		fmt.Sprint(
			arg_spvMaintainer,
			arg_isTrusted,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SetSpvMaintainerStatus(
		transactorOptions,
		arg_spvMaintainer,
		arg_isTrusted,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"setSpvMaintainerStatus",
			arg_spvMaintainer,
			arg_isTrusted,
		)
	}

	bLogger.Infof(
		"submitted transaction setSpvMaintainerStatus with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SetSpvMaintainerStatus(
				newTransactorOptions,
				arg_spvMaintainer,
				arg_isTrusted,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"setSpvMaintainerStatus",
					arg_spvMaintainer,
					arg_isTrusted,
				)
			}

			bLogger.Infof(
				"submitted transaction setSpvMaintainerStatus with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSetSpvMaintainerStatus(
	arg_spvMaintainer common.Address,
	arg_isTrusted bool,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"setSpvMaintainerStatus",
		&result,
		arg_spvMaintainer,
		arg_isTrusted,
	)

	return err
}

func (b *Bridge) SetSpvMaintainerStatusGasEstimate(
	arg_spvMaintainer common.Address,
	arg_isTrusted bool,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"setSpvMaintainerStatus",
		b.contractABI,
		b.transactor,
		arg_spvMaintainer,
		arg_isTrusted,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SetVaultStatus(
	arg_vault common.Address,
	arg_isTrusted bool,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction setVaultStatus",
		" params: ",
		fmt.Sprint(
			arg_vault,
			arg_isTrusted,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SetVaultStatus(
		transactorOptions,
		arg_vault,
		arg_isTrusted,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"setVaultStatus",
			arg_vault,
			arg_isTrusted,
		)
	}

	bLogger.Infof(
		"submitted transaction setVaultStatus with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SetVaultStatus(
				newTransactorOptions,
				arg_vault,
				arg_isTrusted,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"setVaultStatus",
					arg_vault,
					arg_isTrusted,
				)
			}

			bLogger.Infof(
				"submitted transaction setVaultStatus with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSetVaultStatus(
	arg_vault common.Address,
	arg_isTrusted bool,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"setVaultStatus",
		&result,
		arg_vault,
		arg_isTrusted,
	)

	return err
}

func (b *Bridge) SetVaultStatusGasEstimate(
	arg_vault common.Address,
	arg_isTrusted bool,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"setVaultStatus",
		b.contractABI,
		b.transactor,
		arg_vault,
		arg_isTrusted,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SubmitDepositSweepProof(
	arg_sweepTx abi.BitcoinTxInfo,
	arg_sweepProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_vault common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction submitDepositSweepProof",
		" params: ",
		fmt.Sprint(
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
			arg_vault,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SubmitDepositSweepProof(
		transactorOptions,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
		arg_vault,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"submitDepositSweepProof",
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
			arg_vault,
		)
	}

	bLogger.Infof(
		"submitted transaction submitDepositSweepProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SubmitDepositSweepProof(
				newTransactorOptions,
				arg_sweepTx,
				arg_sweepProof,
				arg_mainUtxo,
				arg_vault,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"submitDepositSweepProof",
					arg_sweepTx,
					arg_sweepProof,
					arg_mainUtxo,
					arg_vault,
				)
			}

			bLogger.Infof(
				"submitted transaction submitDepositSweepProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSubmitDepositSweepProof(
	arg_sweepTx abi.BitcoinTxInfo,
	arg_sweepProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_vault common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"submitDepositSweepProof",
		&result,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
		arg_vault,
	)

	return err
}

func (b *Bridge) SubmitDepositSweepProofGasEstimate(
	arg_sweepTx abi.BitcoinTxInfo,
	arg_sweepProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_vault common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"submitDepositSweepProof",
		b.contractABI,
		b.transactor,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
		arg_vault,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SubmitFraudChallenge(
	arg_walletPublicKey []byte,
	arg_preimageSha256 []byte,
	arg_signature abi.BitcoinTxRSVSignature,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction submitFraudChallenge",
		" params: ",
		fmt.Sprint(
			arg_walletPublicKey,
			arg_preimageSha256,
			arg_signature,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SubmitFraudChallenge(
		transactorOptions,
		arg_walletPublicKey,
		arg_preimageSha256,
		arg_signature,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"submitFraudChallenge",
			arg_walletPublicKey,
			arg_preimageSha256,
			arg_signature,
		)
	}

	bLogger.Infof(
		"submitted transaction submitFraudChallenge with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SubmitFraudChallenge(
				newTransactorOptions,
				arg_walletPublicKey,
				arg_preimageSha256,
				arg_signature,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"submitFraudChallenge",
					arg_walletPublicKey,
					arg_preimageSha256,
					arg_signature,
				)
			}

			bLogger.Infof(
				"submitted transaction submitFraudChallenge with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSubmitFraudChallenge(
	arg_walletPublicKey []byte,
	arg_preimageSha256 []byte,
	arg_signature abi.BitcoinTxRSVSignature,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"submitFraudChallenge",
		&result,
		arg_walletPublicKey,
		arg_preimageSha256,
		arg_signature,
	)

	return err
}

func (b *Bridge) SubmitFraudChallengeGasEstimate(
	arg_walletPublicKey []byte,
	arg_preimageSha256 []byte,
	arg_signature abi.BitcoinTxRSVSignature,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"submitFraudChallenge",
		b.contractABI,
		b.transactor,
		arg_walletPublicKey,
		arg_preimageSha256,
		arg_signature,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SubmitMovedFundsSweepProof(
	arg_sweepTx abi.BitcoinTxInfo,
	arg_sweepProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction submitMovedFundsSweepProof",
		" params: ",
		fmt.Sprint(
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SubmitMovedFundsSweepProof(
		transactorOptions,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"submitMovedFundsSweepProof",
			arg_sweepTx,
			arg_sweepProof,
			arg_mainUtxo,
		)
	}

	bLogger.Infof(
		"submitted transaction submitMovedFundsSweepProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SubmitMovedFundsSweepProof(
				newTransactorOptions,
				arg_sweepTx,
				arg_sweepProof,
				arg_mainUtxo,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"submitMovedFundsSweepProof",
					arg_sweepTx,
					arg_sweepProof,
					arg_mainUtxo,
				)
			}

			bLogger.Infof(
				"submitted transaction submitMovedFundsSweepProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSubmitMovedFundsSweepProof(
	arg_sweepTx abi.BitcoinTxInfo,
	arg_sweepProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"submitMovedFundsSweepProof",
		&result,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
	)

	return err
}

func (b *Bridge) SubmitMovedFundsSweepProofGasEstimate(
	arg_sweepTx abi.BitcoinTxInfo,
	arg_sweepProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"submitMovedFundsSweepProof",
		b.contractABI,
		b.transactor,
		arg_sweepTx,
		arg_sweepProof,
		arg_mainUtxo,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SubmitMovingFundsCommitment(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO,
	arg_walletMembersIDs []uint32,
	arg_walletMemberIndex *big.Int,
	arg_targetWallets [][20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction submitMovingFundsCommitment",
		" params: ",
		fmt.Sprint(
			arg_walletPubKeyHash,
			arg_walletMainUtxo,
			arg_walletMembersIDs,
			arg_walletMemberIndex,
			arg_targetWallets,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SubmitMovingFundsCommitment(
		transactorOptions,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
		arg_walletMembersIDs,
		arg_walletMemberIndex,
		arg_targetWallets,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"submitMovingFundsCommitment",
			arg_walletPubKeyHash,
			arg_walletMainUtxo,
			arg_walletMembersIDs,
			arg_walletMemberIndex,
			arg_targetWallets,
		)
	}

	bLogger.Infof(
		"submitted transaction submitMovingFundsCommitment with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SubmitMovingFundsCommitment(
				newTransactorOptions,
				arg_walletPubKeyHash,
				arg_walletMainUtxo,
				arg_walletMembersIDs,
				arg_walletMemberIndex,
				arg_targetWallets,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"submitMovingFundsCommitment",
					arg_walletPubKeyHash,
					arg_walletMainUtxo,
					arg_walletMembersIDs,
					arg_walletMemberIndex,
					arg_targetWallets,
				)
			}

			bLogger.Infof(
				"submitted transaction submitMovingFundsCommitment with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSubmitMovingFundsCommitment(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO,
	arg_walletMembersIDs []uint32,
	arg_walletMemberIndex *big.Int,
	arg_targetWallets [][20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"submitMovingFundsCommitment",
		&result,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
		arg_walletMembersIDs,
		arg_walletMemberIndex,
		arg_targetWallets,
	)

	return err
}

func (b *Bridge) SubmitMovingFundsCommitmentGasEstimate(
	arg_walletPubKeyHash [20]byte,
	arg_walletMainUtxo abi.BitcoinTxUTXO,
	arg_walletMembersIDs []uint32,
	arg_walletMemberIndex *big.Int,
	arg_targetWallets [][20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"submitMovingFundsCommitment",
		b.contractABI,
		b.transactor,
		arg_walletPubKeyHash,
		arg_walletMainUtxo,
		arg_walletMembersIDs,
		arg_walletMemberIndex,
		arg_targetWallets,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SubmitMovingFundsProof(
	arg_movingFundsTx abi.BitcoinTxInfo,
	arg_movingFundsProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction submitMovingFundsProof",
		" params: ",
		fmt.Sprint(
			arg_movingFundsTx,
			arg_movingFundsProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SubmitMovingFundsProof(
		transactorOptions,
		arg_movingFundsTx,
		arg_movingFundsProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"submitMovingFundsProof",
			arg_movingFundsTx,
			arg_movingFundsProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		)
	}

	bLogger.Infof(
		"submitted transaction submitMovingFundsProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SubmitMovingFundsProof(
				newTransactorOptions,
				arg_movingFundsTx,
				arg_movingFundsProof,
				arg_mainUtxo,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"submitMovingFundsProof",
					arg_movingFundsTx,
					arg_movingFundsProof,
					arg_mainUtxo,
					arg_walletPubKeyHash,
				)
			}

			bLogger.Infof(
				"submitted transaction submitMovingFundsProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSubmitMovingFundsProof(
	arg_movingFundsTx abi.BitcoinTxInfo,
	arg_movingFundsProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"submitMovingFundsProof",
		&result,
		arg_movingFundsTx,
		arg_movingFundsProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return err
}

func (b *Bridge) SubmitMovingFundsProofGasEstimate(
	arg_movingFundsTx abi.BitcoinTxInfo,
	arg_movingFundsProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"submitMovingFundsProof",
		b.contractABI,
		b.transactor,
		arg_movingFundsTx,
		arg_movingFundsProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) SubmitRedemptionProof(
	arg_redemptionTx abi.BitcoinTxInfo,
	arg_redemptionProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_walletPubKeyHash [20]byte,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction submitRedemptionProof",
		" params: ",
		fmt.Sprint(
			arg_redemptionTx,
			arg_redemptionProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.SubmitRedemptionProof(
		transactorOptions,
		arg_redemptionTx,
		arg_redemptionProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"submitRedemptionProof",
			arg_redemptionTx,
			arg_redemptionProof,
			arg_mainUtxo,
			arg_walletPubKeyHash,
		)
	}

	bLogger.Infof(
		"submitted transaction submitRedemptionProof with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.SubmitRedemptionProof(
				newTransactorOptions,
				arg_redemptionTx,
				arg_redemptionProof,
				arg_mainUtxo,
				arg_walletPubKeyHash,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"submitRedemptionProof",
					arg_redemptionTx,
					arg_redemptionProof,
					arg_mainUtxo,
					arg_walletPubKeyHash,
				)
			}

			bLogger.Infof(
				"submitted transaction submitRedemptionProof with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallSubmitRedemptionProof(
	arg_redemptionTx abi.BitcoinTxInfo,
	arg_redemptionProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"submitRedemptionProof",
		&result,
		arg_redemptionTx,
		arg_redemptionProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return err
}

func (b *Bridge) SubmitRedemptionProofGasEstimate(
	arg_redemptionTx abi.BitcoinTxInfo,
	arg_redemptionProof abi.BitcoinTxProof,
	arg_mainUtxo abi.BitcoinTxUTXO,
	arg_walletPubKeyHash [20]byte,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"submitRedemptionProof",
		b.contractABI,
		b.transactor,
		arg_redemptionTx,
		arg_redemptionProof,
		arg_mainUtxo,
		arg_walletPubKeyHash,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) TransferGovernance(
	arg_newGovernance common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction transferGovernance",
		" params: ",
		fmt.Sprint(
			arg_newGovernance,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.TransferGovernance(
		transactorOptions,
		arg_newGovernance,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"transferGovernance",
			arg_newGovernance,
		)
	}

	bLogger.Infof(
		"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.TransferGovernance(
				newTransactorOptions,
				arg_newGovernance,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"transferGovernance",
					arg_newGovernance,
				)
			}

			bLogger.Infof(
				"submitted transaction transferGovernance with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallTransferGovernance(
	arg_newGovernance common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"transferGovernance",
		&result,
		arg_newGovernance,
	)

	return err
}

func (b *Bridge) TransferGovernanceGasEstimate(
	arg_newGovernance common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"transferGovernance",
		b.contractABI,
		b.transactor,
		arg_newGovernance,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) UpdateDepositParameters(
	arg_depositDustThreshold uint64,
	arg_depositTreasuryFeeDivisor uint64,
	arg_depositTxMaxFee uint64,
	arg_depositRevealAheadPeriod uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction updateDepositParameters",
		" params: ",
		fmt.Sprint(
			arg_depositDustThreshold,
			arg_depositTreasuryFeeDivisor,
			arg_depositTxMaxFee,
			arg_depositRevealAheadPeriod,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.UpdateDepositParameters(
		transactorOptions,
		arg_depositDustThreshold,
		arg_depositTreasuryFeeDivisor,
		arg_depositTxMaxFee,
		arg_depositRevealAheadPeriod,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"updateDepositParameters",
			arg_depositDustThreshold,
			arg_depositTreasuryFeeDivisor,
			arg_depositTxMaxFee,
			arg_depositRevealAheadPeriod,
		)
	}

	bLogger.Infof(
		"submitted transaction updateDepositParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.UpdateDepositParameters(
				newTransactorOptions,
				arg_depositDustThreshold,
				arg_depositTreasuryFeeDivisor,
				arg_depositTxMaxFee,
				arg_depositRevealAheadPeriod,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"updateDepositParameters",
					arg_depositDustThreshold,
					arg_depositTreasuryFeeDivisor,
					arg_depositTxMaxFee,
					arg_depositRevealAheadPeriod,
				)
			}

			bLogger.Infof(
				"submitted transaction updateDepositParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallUpdateDepositParameters(
	arg_depositDustThreshold uint64,
	arg_depositTreasuryFeeDivisor uint64,
	arg_depositTxMaxFee uint64,
	arg_depositRevealAheadPeriod uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"updateDepositParameters",
		&result,
		arg_depositDustThreshold,
		arg_depositTreasuryFeeDivisor,
		arg_depositTxMaxFee,
		arg_depositRevealAheadPeriod,
	)

	return err
}

func (b *Bridge) UpdateDepositParametersGasEstimate(
	arg_depositDustThreshold uint64,
	arg_depositTreasuryFeeDivisor uint64,
	arg_depositTxMaxFee uint64,
	arg_depositRevealAheadPeriod uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"updateDepositParameters",
		b.contractABI,
		b.transactor,
		arg_depositDustThreshold,
		arg_depositTreasuryFeeDivisor,
		arg_depositTxMaxFee,
		arg_depositRevealAheadPeriod,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) UpdateFraudParameters(
	arg_fraudChallengeDepositAmount *big.Int,
	arg_fraudChallengeDefeatTimeout uint32,
	arg_fraudSlashingAmount *big.Int,
	arg_fraudNotifierRewardMultiplier uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction updateFraudParameters",
		" params: ",
		fmt.Sprint(
			arg_fraudChallengeDepositAmount,
			arg_fraudChallengeDefeatTimeout,
			arg_fraudSlashingAmount,
			arg_fraudNotifierRewardMultiplier,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.UpdateFraudParameters(
		transactorOptions,
		arg_fraudChallengeDepositAmount,
		arg_fraudChallengeDefeatTimeout,
		arg_fraudSlashingAmount,
		arg_fraudNotifierRewardMultiplier,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"updateFraudParameters",
			arg_fraudChallengeDepositAmount,
			arg_fraudChallengeDefeatTimeout,
			arg_fraudSlashingAmount,
			arg_fraudNotifierRewardMultiplier,
		)
	}

	bLogger.Infof(
		"submitted transaction updateFraudParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.UpdateFraudParameters(
				newTransactorOptions,
				arg_fraudChallengeDepositAmount,
				arg_fraudChallengeDefeatTimeout,
				arg_fraudSlashingAmount,
				arg_fraudNotifierRewardMultiplier,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"updateFraudParameters",
					arg_fraudChallengeDepositAmount,
					arg_fraudChallengeDefeatTimeout,
					arg_fraudSlashingAmount,
					arg_fraudNotifierRewardMultiplier,
				)
			}

			bLogger.Infof(
				"submitted transaction updateFraudParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallUpdateFraudParameters(
	arg_fraudChallengeDepositAmount *big.Int,
	arg_fraudChallengeDefeatTimeout uint32,
	arg_fraudSlashingAmount *big.Int,
	arg_fraudNotifierRewardMultiplier uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"updateFraudParameters",
		&result,
		arg_fraudChallengeDepositAmount,
		arg_fraudChallengeDefeatTimeout,
		arg_fraudSlashingAmount,
		arg_fraudNotifierRewardMultiplier,
	)

	return err
}

func (b *Bridge) UpdateFraudParametersGasEstimate(
	arg_fraudChallengeDepositAmount *big.Int,
	arg_fraudChallengeDefeatTimeout uint32,
	arg_fraudSlashingAmount *big.Int,
	arg_fraudNotifierRewardMultiplier uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"updateFraudParameters",
		b.contractABI,
		b.transactor,
		arg_fraudChallengeDepositAmount,
		arg_fraudChallengeDefeatTimeout,
		arg_fraudSlashingAmount,
		arg_fraudNotifierRewardMultiplier,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) UpdateMovingFundsParameters(
	arg_movingFundsTxMaxTotalFee uint64,
	arg_movingFundsDustThreshold uint64,
	arg_movingFundsTimeoutResetDelay uint32,
	arg_movingFundsTimeout uint32,
	arg_movingFundsTimeoutSlashingAmount *big.Int,
	arg_movingFundsTimeoutNotifierRewardMultiplier uint32,
	arg_movingFundsCommitmentGasOffset uint16,
	arg_movedFundsSweepTxMaxTotalFee uint64,
	arg_movedFundsSweepTimeout uint32,
	arg_movedFundsSweepTimeoutSlashingAmount *big.Int,
	arg_movedFundsSweepTimeoutNotifierRewardMultiplier uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction updateMovingFundsParameters",
		" params: ",
		fmt.Sprint(
			arg_movingFundsTxMaxTotalFee,
			arg_movingFundsDustThreshold,
			arg_movingFundsTimeoutResetDelay,
			arg_movingFundsTimeout,
			arg_movingFundsTimeoutSlashingAmount,
			arg_movingFundsTimeoutNotifierRewardMultiplier,
			arg_movingFundsCommitmentGasOffset,
			arg_movedFundsSweepTxMaxTotalFee,
			arg_movedFundsSweepTimeout,
			arg_movedFundsSweepTimeoutSlashingAmount,
			arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.UpdateMovingFundsParameters(
		transactorOptions,
		arg_movingFundsTxMaxTotalFee,
		arg_movingFundsDustThreshold,
		arg_movingFundsTimeoutResetDelay,
		arg_movingFundsTimeout,
		arg_movingFundsTimeoutSlashingAmount,
		arg_movingFundsTimeoutNotifierRewardMultiplier,
		arg_movingFundsCommitmentGasOffset,
		arg_movedFundsSweepTxMaxTotalFee,
		arg_movedFundsSweepTimeout,
		arg_movedFundsSweepTimeoutSlashingAmount,
		arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"updateMovingFundsParameters",
			arg_movingFundsTxMaxTotalFee,
			arg_movingFundsDustThreshold,
			arg_movingFundsTimeoutResetDelay,
			arg_movingFundsTimeout,
			arg_movingFundsTimeoutSlashingAmount,
			arg_movingFundsTimeoutNotifierRewardMultiplier,
			arg_movingFundsCommitmentGasOffset,
			arg_movedFundsSweepTxMaxTotalFee,
			arg_movedFundsSweepTimeout,
			arg_movedFundsSweepTimeoutSlashingAmount,
			arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
		)
	}

	bLogger.Infof(
		"submitted transaction updateMovingFundsParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.UpdateMovingFundsParameters(
				newTransactorOptions,
				arg_movingFundsTxMaxTotalFee,
				arg_movingFundsDustThreshold,
				arg_movingFundsTimeoutResetDelay,
				arg_movingFundsTimeout,
				arg_movingFundsTimeoutSlashingAmount,
				arg_movingFundsTimeoutNotifierRewardMultiplier,
				arg_movingFundsCommitmentGasOffset,
				arg_movedFundsSweepTxMaxTotalFee,
				arg_movedFundsSweepTimeout,
				arg_movedFundsSweepTimeoutSlashingAmount,
				arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"updateMovingFundsParameters",
					arg_movingFundsTxMaxTotalFee,
					arg_movingFundsDustThreshold,
					arg_movingFundsTimeoutResetDelay,
					arg_movingFundsTimeout,
					arg_movingFundsTimeoutSlashingAmount,
					arg_movingFundsTimeoutNotifierRewardMultiplier,
					arg_movingFundsCommitmentGasOffset,
					arg_movedFundsSweepTxMaxTotalFee,
					arg_movedFundsSweepTimeout,
					arg_movedFundsSweepTimeoutSlashingAmount,
					arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
				)
			}

			bLogger.Infof(
				"submitted transaction updateMovingFundsParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallUpdateMovingFundsParameters(
	arg_movingFundsTxMaxTotalFee uint64,
	arg_movingFundsDustThreshold uint64,
	arg_movingFundsTimeoutResetDelay uint32,
	arg_movingFundsTimeout uint32,
	arg_movingFundsTimeoutSlashingAmount *big.Int,
	arg_movingFundsTimeoutNotifierRewardMultiplier uint32,
	arg_movingFundsCommitmentGasOffset uint16,
	arg_movedFundsSweepTxMaxTotalFee uint64,
	arg_movedFundsSweepTimeout uint32,
	arg_movedFundsSweepTimeoutSlashingAmount *big.Int,
	arg_movedFundsSweepTimeoutNotifierRewardMultiplier uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"updateMovingFundsParameters",
		&result,
		arg_movingFundsTxMaxTotalFee,
		arg_movingFundsDustThreshold,
		arg_movingFundsTimeoutResetDelay,
		arg_movingFundsTimeout,
		arg_movingFundsTimeoutSlashingAmount,
		arg_movingFundsTimeoutNotifierRewardMultiplier,
		arg_movingFundsCommitmentGasOffset,
		arg_movedFundsSweepTxMaxTotalFee,
		arg_movedFundsSweepTimeout,
		arg_movedFundsSweepTimeoutSlashingAmount,
		arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
	)

	return err
}

func (b *Bridge) UpdateMovingFundsParametersGasEstimate(
	arg_movingFundsTxMaxTotalFee uint64,
	arg_movingFundsDustThreshold uint64,
	arg_movingFundsTimeoutResetDelay uint32,
	arg_movingFundsTimeout uint32,
	arg_movingFundsTimeoutSlashingAmount *big.Int,
	arg_movingFundsTimeoutNotifierRewardMultiplier uint32,
	arg_movingFundsCommitmentGasOffset uint16,
	arg_movedFundsSweepTxMaxTotalFee uint64,
	arg_movedFundsSweepTimeout uint32,
	arg_movedFundsSweepTimeoutSlashingAmount *big.Int,
	arg_movedFundsSweepTimeoutNotifierRewardMultiplier uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"updateMovingFundsParameters",
		b.contractABI,
		b.transactor,
		arg_movingFundsTxMaxTotalFee,
		arg_movingFundsDustThreshold,
		arg_movingFundsTimeoutResetDelay,
		arg_movingFundsTimeout,
		arg_movingFundsTimeoutSlashingAmount,
		arg_movingFundsTimeoutNotifierRewardMultiplier,
		arg_movingFundsCommitmentGasOffset,
		arg_movedFundsSweepTxMaxTotalFee,
		arg_movedFundsSweepTimeout,
		arg_movedFundsSweepTimeoutSlashingAmount,
		arg_movedFundsSweepTimeoutNotifierRewardMultiplier,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) UpdateRedemptionParameters(
	arg_redemptionDustThreshold uint64,
	arg_redemptionTreasuryFeeDivisor uint64,
	arg_redemptionTxMaxFee uint64,
	arg_redemptionTxMaxTotalFee uint64,
	arg_redemptionTimeout uint32,
	arg_redemptionTimeoutSlashingAmount *big.Int,
	arg_redemptionTimeoutNotifierRewardMultiplier uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction updateRedemptionParameters",
		" params: ",
		fmt.Sprint(
			arg_redemptionDustThreshold,
			arg_redemptionTreasuryFeeDivisor,
			arg_redemptionTxMaxFee,
			arg_redemptionTxMaxTotalFee,
			arg_redemptionTimeout,
			arg_redemptionTimeoutSlashingAmount,
			arg_redemptionTimeoutNotifierRewardMultiplier,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.UpdateRedemptionParameters(
		transactorOptions,
		arg_redemptionDustThreshold,
		arg_redemptionTreasuryFeeDivisor,
		arg_redemptionTxMaxFee,
		arg_redemptionTxMaxTotalFee,
		arg_redemptionTimeout,
		arg_redemptionTimeoutSlashingAmount,
		arg_redemptionTimeoutNotifierRewardMultiplier,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"updateRedemptionParameters",
			arg_redemptionDustThreshold,
			arg_redemptionTreasuryFeeDivisor,
			arg_redemptionTxMaxFee,
			arg_redemptionTxMaxTotalFee,
			arg_redemptionTimeout,
			arg_redemptionTimeoutSlashingAmount,
			arg_redemptionTimeoutNotifierRewardMultiplier,
		)
	}

	bLogger.Infof(
		"submitted transaction updateRedemptionParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.UpdateRedemptionParameters(
				newTransactorOptions,
				arg_redemptionDustThreshold,
				arg_redemptionTreasuryFeeDivisor,
				arg_redemptionTxMaxFee,
				arg_redemptionTxMaxTotalFee,
				arg_redemptionTimeout,
				arg_redemptionTimeoutSlashingAmount,
				arg_redemptionTimeoutNotifierRewardMultiplier,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"updateRedemptionParameters",
					arg_redemptionDustThreshold,
					arg_redemptionTreasuryFeeDivisor,
					arg_redemptionTxMaxFee,
					arg_redemptionTxMaxTotalFee,
					arg_redemptionTimeout,
					arg_redemptionTimeoutSlashingAmount,
					arg_redemptionTimeoutNotifierRewardMultiplier,
				)
			}

			bLogger.Infof(
				"submitted transaction updateRedemptionParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallUpdateRedemptionParameters(
	arg_redemptionDustThreshold uint64,
	arg_redemptionTreasuryFeeDivisor uint64,
	arg_redemptionTxMaxFee uint64,
	arg_redemptionTxMaxTotalFee uint64,
	arg_redemptionTimeout uint32,
	arg_redemptionTimeoutSlashingAmount *big.Int,
	arg_redemptionTimeoutNotifierRewardMultiplier uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"updateRedemptionParameters",
		&result,
		arg_redemptionDustThreshold,
		arg_redemptionTreasuryFeeDivisor,
		arg_redemptionTxMaxFee,
		arg_redemptionTxMaxTotalFee,
		arg_redemptionTimeout,
		arg_redemptionTimeoutSlashingAmount,
		arg_redemptionTimeoutNotifierRewardMultiplier,
	)

	return err
}

func (b *Bridge) UpdateRedemptionParametersGasEstimate(
	arg_redemptionDustThreshold uint64,
	arg_redemptionTreasuryFeeDivisor uint64,
	arg_redemptionTxMaxFee uint64,
	arg_redemptionTxMaxTotalFee uint64,
	arg_redemptionTimeout uint32,
	arg_redemptionTimeoutSlashingAmount *big.Int,
	arg_redemptionTimeoutNotifierRewardMultiplier uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"updateRedemptionParameters",
		b.contractABI,
		b.transactor,
		arg_redemptionDustThreshold,
		arg_redemptionTreasuryFeeDivisor,
		arg_redemptionTxMaxFee,
		arg_redemptionTxMaxTotalFee,
		arg_redemptionTimeout,
		arg_redemptionTimeoutSlashingAmount,
		arg_redemptionTimeoutNotifierRewardMultiplier,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) UpdateTreasury(
	arg_treasury common.Address,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction updateTreasury",
		" params: ",
		fmt.Sprint(
			arg_treasury,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.UpdateTreasury(
		transactorOptions,
		arg_treasury,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"updateTreasury",
			arg_treasury,
		)
	}

	bLogger.Infof(
		"submitted transaction updateTreasury with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.UpdateTreasury(
				newTransactorOptions,
				arg_treasury,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"updateTreasury",
					arg_treasury,
				)
			}

			bLogger.Infof(
				"submitted transaction updateTreasury with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallUpdateTreasury(
	arg_treasury common.Address,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"updateTreasury",
		&result,
		arg_treasury,
	)

	return err
}

func (b *Bridge) UpdateTreasuryGasEstimate(
	arg_treasury common.Address,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"updateTreasury",
		b.contractABI,
		b.transactor,
		arg_treasury,
	)

	return result, err
}

// Transaction submission.
func (b *Bridge) UpdateWalletParameters(
	arg_walletCreationPeriod uint32,
	arg_walletCreationMinBtcBalance uint64,
	arg_walletCreationMaxBtcBalance uint64,
	arg_walletClosureMinBtcBalance uint64,
	arg_walletMaxAge uint32,
	arg_walletMaxBtcTransfer uint64,
	arg_walletClosingPeriod uint32,

	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	bLogger.Debug(
		"submitting transaction updateWalletParameters",
		" params: ",
		fmt.Sprint(
			arg_walletCreationPeriod,
			arg_walletCreationMinBtcBalance,
			arg_walletCreationMaxBtcBalance,
			arg_walletClosureMinBtcBalance,
			arg_walletMaxAge,
			arg_walletMaxBtcTransfer,
			arg_walletClosingPeriod,
		),
	)

	b.transactionMutex.Lock()
	defer b.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *b.transactorOptions

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := b.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := b.contract.UpdateWalletParameters(
		transactorOptions,
		arg_walletCreationPeriod,
		arg_walletCreationMinBtcBalance,
		arg_walletCreationMaxBtcBalance,
		arg_walletClosureMinBtcBalance,
		arg_walletMaxAge,
		arg_walletMaxBtcTransfer,
		arg_walletClosingPeriod,
	)
	if err != nil {
		return transaction, b.errorResolver.ResolveError(
			err,
			b.transactorOptions.From,
			nil,
			"updateWalletParameters",
			arg_walletCreationPeriod,
			arg_walletCreationMinBtcBalance,
			arg_walletCreationMaxBtcBalance,
			arg_walletClosureMinBtcBalance,
			arg_walletMaxAge,
			arg_walletMaxBtcTransfer,
			arg_walletClosingPeriod,
		)
	}

	bLogger.Infof(
		"submitted transaction updateWalletParameters with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go b.miningWaiter.ForceMining(
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

			transaction, err := b.contract.UpdateWalletParameters(
				newTransactorOptions,
				arg_walletCreationPeriod,
				arg_walletCreationMinBtcBalance,
				arg_walletCreationMaxBtcBalance,
				arg_walletClosureMinBtcBalance,
				arg_walletMaxAge,
				arg_walletMaxBtcTransfer,
				arg_walletClosingPeriod,
			)
			if err != nil {
				return nil, b.errorResolver.ResolveError(
					err,
					b.transactorOptions.From,
					nil,
					"updateWalletParameters",
					arg_walletCreationPeriod,
					arg_walletCreationMinBtcBalance,
					arg_walletCreationMaxBtcBalance,
					arg_walletClosureMinBtcBalance,
					arg_walletMaxAge,
					arg_walletMaxBtcTransfer,
					arg_walletClosingPeriod,
				)
			}

			bLogger.Infof(
				"submitted transaction updateWalletParameters with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	b.nonceManager.IncrementNonce()

	return transaction, err
}

// Non-mutating call, not a transaction submission.
func (b *Bridge) CallUpdateWalletParameters(
	arg_walletCreationPeriod uint32,
	arg_walletCreationMinBtcBalance uint64,
	arg_walletCreationMaxBtcBalance uint64,
	arg_walletClosureMinBtcBalance uint64,
	arg_walletMaxAge uint32,
	arg_walletMaxBtcTransfer uint64,
	arg_walletClosingPeriod uint32,
	blockNumber *big.Int,
) error {
	var result interface{} = nil

	err := chainutil.CallAtBlock(
		b.transactorOptions.From,
		blockNumber, nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"updateWalletParameters",
		&result,
		arg_walletCreationPeriod,
		arg_walletCreationMinBtcBalance,
		arg_walletCreationMaxBtcBalance,
		arg_walletClosureMinBtcBalance,
		arg_walletMaxAge,
		arg_walletMaxBtcTransfer,
		arg_walletClosingPeriod,
	)

	return err
}

func (b *Bridge) UpdateWalletParametersGasEstimate(
	arg_walletCreationPeriod uint32,
	arg_walletCreationMinBtcBalance uint64,
	arg_walletCreationMaxBtcBalance uint64,
	arg_walletClosureMinBtcBalance uint64,
	arg_walletMaxAge uint32,
	arg_walletMaxBtcTransfer uint64,
	arg_walletClosingPeriod uint32,
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		b.callerOptions.From,
		b.contractAddress,
		"updateWalletParameters",
		b.contractABI,
		b.transactor,
		arg_walletCreationPeriod,
		arg_walletCreationMinBtcBalance,
		arg_walletCreationMaxBtcBalance,
		arg_walletClosureMinBtcBalance,
		arg_walletMaxAge,
		arg_walletMaxBtcTransfer,
		arg_walletClosingPeriod,
	)

	return result, err
}

// ----- Const Methods ------

func (b *Bridge) ActiveWalletPubKeyHash() ([20]byte, error) {
	result, err := b.contract.ActiveWalletPubKeyHash(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"activeWalletPubKeyHash",
		)
	}

	return result, err
}

func (b *Bridge) ActiveWalletPubKeyHashAtBlock(
	blockNumber *big.Int,
) ([20]byte, error) {
	var result [20]byte

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"activeWalletPubKeyHash",
		&result,
	)

	return result, err
}

type contractReferences struct {
	Bank                common.Address
	Relay               common.Address
	EcdsaWalletRegistry common.Address
	ReimbursementPool   common.Address
}

func (b *Bridge) ContractReferences() (contractReferences, error) {
	result, err := b.contract.ContractReferences(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"contractReferences",
		)
	}

	return result, err
}

func (b *Bridge) ContractReferencesAtBlock(
	blockNumber *big.Int,
) (contractReferences, error) {
	var result contractReferences

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"contractReferences",
		&result,
	)

	return result, err
}

type depositParameters struct {
	DepositDustThreshold      uint64
	DepositTreasuryFeeDivisor uint64
	DepositTxMaxFee           uint64
	DepositRevealAheadPeriod  uint32
}

func (b *Bridge) DepositParameters() (depositParameters, error) {
	result, err := b.contract.DepositParameters(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"depositParameters",
		)
	}

	return result, err
}

func (b *Bridge) DepositParametersAtBlock(
	blockNumber *big.Int,
) (depositParameters, error) {
	var result depositParameters

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"depositParameters",
		&result,
	)

	return result, err
}

func (b *Bridge) Deposits(
	arg_depositKey *big.Int,
) (abi.DepositDepositRequest, error) {
	result, err := b.contract.Deposits(
		b.callerOptions,
		arg_depositKey,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"deposits",
			arg_depositKey,
		)
	}

	return result, err
}

func (b *Bridge) DepositsAtBlock(
	arg_depositKey *big.Int,
	blockNumber *big.Int,
) (abi.DepositDepositRequest, error) {
	var result abi.DepositDepositRequest

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"deposits",
		&result,
		arg_depositKey,
	)

	return result, err
}

func (b *Bridge) FraudChallenges(
	arg_challengeKey *big.Int,
) (abi.FraudFraudChallenge, error) {
	result, err := b.contract.FraudChallenges(
		b.callerOptions,
		arg_challengeKey,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"fraudChallenges",
			arg_challengeKey,
		)
	}

	return result, err
}

func (b *Bridge) FraudChallengesAtBlock(
	arg_challengeKey *big.Int,
	blockNumber *big.Int,
) (abi.FraudFraudChallenge, error) {
	var result abi.FraudFraudChallenge

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"fraudChallenges",
		&result,
		arg_challengeKey,
	)

	return result, err
}

type fraudParameters struct {
	FraudChallengeDepositAmount   *big.Int
	FraudChallengeDefeatTimeout   uint32
	FraudSlashingAmount           *big.Int
	FraudNotifierRewardMultiplier uint32
}

func (b *Bridge) FraudParameters() (fraudParameters, error) {
	result, err := b.contract.FraudParameters(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"fraudParameters",
		)
	}

	return result, err
}

func (b *Bridge) FraudParametersAtBlock(
	blockNumber *big.Int,
) (fraudParameters, error) {
	var result fraudParameters

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"fraudParameters",
		&result,
	)

	return result, err
}

func (b *Bridge) GetRebateStaking() (common.Address, error) {
	result, err := b.contract.GetRebateStaking(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"getRebateStaking",
		)
	}

	return result, err
}

func (b *Bridge) GetRebateStakingAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"getRebateStaking",
		&result,
	)

	return result, err
}

func (b *Bridge) GetRedemptionWatchtower() (common.Address, error) {
	result, err := b.contract.GetRedemptionWatchtower(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"getRedemptionWatchtower",
		)
	}

	return result, err
}

func (b *Bridge) GetRedemptionWatchtowerAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"getRedemptionWatchtower",
		&result,
	)

	return result, err
}

func (b *Bridge) GetReservationRouter() (common.Address, error) {
	result, err := b.contract.GetReservationRouter(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"getReservationRouter",
		)
	}

	return result, err
}

func (b *Bridge) GetReservationRouterAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"getReservationRouter",
		&result,
	)

	return result, err
}

func (b *Bridge) Governance() (common.Address, error) {
	result, err := b.contract.Governance(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"governance",
		)
	}

	return result, err
}

func (b *Bridge) GovernanceAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"governance",
		&result,
	)

	return result, err
}

func (b *Bridge) IsReservedDeposit(
	arg_depositKey *big.Int,
) (bool, error) {
	result, err := b.contract.IsReservedDeposit(
		b.callerOptions,
		arg_depositKey,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"isReservedDeposit",
			arg_depositKey,
		)
	}

	return result, err
}

func (b *Bridge) IsReservedDepositAtBlock(
	arg_depositKey *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"isReservedDeposit",
		&result,
		arg_depositKey,
	)

	return result, err
}

func (b *Bridge) IsVaultTrusted(
	arg_vault common.Address,
) (bool, error) {
	result, err := b.contract.IsVaultTrusted(
		b.callerOptions,
		arg_vault,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"isVaultTrusted",
			arg_vault,
		)
	}

	return result, err
}

func (b *Bridge) IsVaultTrustedAtBlock(
	arg_vault common.Address,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"isVaultTrusted",
		&result,
		arg_vault,
	)

	return result, err
}

func (b *Bridge) LiveWalletsCount() (uint32, error) {
	result, err := b.contract.LiveWalletsCount(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"liveWalletsCount",
		)
	}

	return result, err
}

func (b *Bridge) LiveWalletsCountAtBlock(
	blockNumber *big.Int,
) (uint32, error) {
	var result uint32

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"liveWalletsCount",
		&result,
	)

	return result, err
}

func (b *Bridge) MovedFundsSweepRequests(
	arg_requestKey *big.Int,
) (abi.MovingFundsMovedFundsSweepRequest, error) {
	result, err := b.contract.MovedFundsSweepRequests(
		b.callerOptions,
		arg_requestKey,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"movedFundsSweepRequests",
			arg_requestKey,
		)
	}

	return result, err
}

func (b *Bridge) MovedFundsSweepRequestsAtBlock(
	arg_requestKey *big.Int,
	blockNumber *big.Int,
) (abi.MovingFundsMovedFundsSweepRequest, error) {
	var result abi.MovingFundsMovedFundsSweepRequest

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"movedFundsSweepRequests",
		&result,
		arg_requestKey,
	)

	return result, err
}

type movingFundsParameters struct {
	MovingFundsTxMaxTotalFee                       uint64
	MovingFundsDustThreshold                       uint64
	MovingFundsTimeoutResetDelay                   uint32
	MovingFundsTimeout                             uint32
	MovingFundsTimeoutSlashingAmount               *big.Int
	MovingFundsTimeoutNotifierRewardMultiplier     uint32
	MovingFundsCommitmentGasOffset                 uint16
	MovedFundsSweepTxMaxTotalFee                   uint64
	MovedFundsSweepTimeout                         uint32
	MovedFundsSweepTimeoutSlashingAmount           *big.Int
	MovedFundsSweepTimeoutNotifierRewardMultiplier uint32
}

func (b *Bridge) MovingFundsParameters() (movingFundsParameters, error) {
	result, err := b.contract.MovingFundsParameters(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"movingFundsParameters",
		)
	}

	return result, err
}

func (b *Bridge) MovingFundsParametersAtBlock(
	blockNumber *big.Int,
) (movingFundsParameters, error) {
	var result movingFundsParameters

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"movingFundsParameters",
		&result,
	)

	return result, err
}

func (b *Bridge) PendingRedemptions(
	arg_redemptionKey *big.Int,
) (abi.RedemptionRedemptionRequest, error) {
	result, err := b.contract.PendingRedemptions(
		b.callerOptions,
		arg_redemptionKey,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"pendingRedemptions",
			arg_redemptionKey,
		)
	}

	return result, err
}

func (b *Bridge) PendingRedemptionsAtBlock(
	arg_redemptionKey *big.Int,
	blockNumber *big.Int,
) (abi.RedemptionRedemptionRequest, error) {
	var result abi.RedemptionRedemptionRequest

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"pendingRedemptions",
		&result,
		arg_redemptionKey,
	)

	return result, err
}

type redemptionParameters struct {
	RedemptionDustThreshold                   uint64
	RedemptionTreasuryFeeDivisor              uint64
	RedemptionTxMaxFee                        uint64
	RedemptionTxMaxTotalFee                   uint64
	RedemptionTimeout                         uint32
	RedemptionTimeoutSlashingAmount           *big.Int
	RedemptionTimeoutNotifierRewardMultiplier uint32
}

func (b *Bridge) RedemptionParameters() (redemptionParameters, error) {
	result, err := b.contract.RedemptionParameters(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"redemptionParameters",
		)
	}

	return result, err
}

func (b *Bridge) RedemptionParametersAtBlock(
	blockNumber *big.Int,
) (redemptionParameters, error) {
	var result redemptionParameters

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"redemptionParameters",
		&result,
	)

	return result, err
}

func (b *Bridge) SpentMainUTXOs(
	arg_utxoKey *big.Int,
) (bool, error) {
	result, err := b.contract.SpentMainUTXOs(
		b.callerOptions,
		arg_utxoKey,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"spentMainUTXOs",
			arg_utxoKey,
		)
	}

	return result, err
}

func (b *Bridge) SpentMainUTXOsAtBlock(
	arg_utxoKey *big.Int,
	blockNumber *big.Int,
) (bool, error) {
	var result bool

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"spentMainUTXOs",
		&result,
		arg_utxoKey,
	)

	return result, err
}

func (b *Bridge) TimedOutRedemptions(
	arg_redemptionKey *big.Int,
) (abi.RedemptionRedemptionRequest, error) {
	result, err := b.contract.TimedOutRedemptions(
		b.callerOptions,
		arg_redemptionKey,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"timedOutRedemptions",
			arg_redemptionKey,
		)
	}

	return result, err
}

func (b *Bridge) TimedOutRedemptionsAtBlock(
	arg_redemptionKey *big.Int,
	blockNumber *big.Int,
) (abi.RedemptionRedemptionRequest, error) {
	var result abi.RedemptionRedemptionRequest

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"timedOutRedemptions",
		&result,
		arg_redemptionKey,
	)

	return result, err
}

func (b *Bridge) Treasury() (common.Address, error) {
	result, err := b.contract.Treasury(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"treasury",
		)
	}

	return result, err
}

func (b *Bridge) TreasuryAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"treasury",
		&result,
	)

	return result, err
}

func (b *Bridge) TxProofDifficultyFactor() (*big.Int, error) {
	result, err := b.contract.TxProofDifficultyFactor(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"txProofDifficultyFactor",
		)
	}

	return result, err
}

func (b *Bridge) TxProofDifficultyFactorAtBlock(
	blockNumber *big.Int,
) (*big.Int, error) {
	var result *big.Int

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"txProofDifficultyFactor",
		&result,
	)

	return result, err
}

type walletParameters struct {
	WalletCreationPeriod        uint32
	WalletCreationMinBtcBalance uint64
	WalletCreationMaxBtcBalance uint64
	WalletClosureMinBtcBalance  uint64
	WalletMaxAge                uint32
	WalletMaxBtcTransfer        uint64
	WalletClosingPeriod         uint32
}

func (b *Bridge) WalletParameters() (walletParameters, error) {
	result, err := b.contract.WalletParameters(
		b.callerOptions,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"walletParameters",
		)
	}

	return result, err
}

func (b *Bridge) WalletParametersAtBlock(
	blockNumber *big.Int,
) (walletParameters, error) {
	var result walletParameters

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"walletParameters",
		&result,
	)

	return result, err
}

func (b *Bridge) Wallets(
	arg_walletPubKeyHash [20]byte,
) (abi.WalletsWallet, error) {
	result, err := b.contract.Wallets(
		b.callerOptions,
		arg_walletPubKeyHash,
	)

	if err != nil {
		return result, b.errorResolver.ResolveError(
			err,
			b.callerOptions.From,
			nil,
			"wallets",
			arg_walletPubKeyHash,
		)
	}

	return result, err
}

func (b *Bridge) WalletsAtBlock(
	arg_walletPubKeyHash [20]byte,
	blockNumber *big.Int,
) (abi.WalletsWallet, error) {
	var result abi.WalletsWallet

	err := chainutil.CallAtBlock(
		b.callerOptions.From,
		blockNumber,
		nil,
		b.contractABI,
		b.caller,
		b.errorResolver,
		b.contractAddress,
		"wallets",
		&result,
		arg_walletPubKeyHash,
	)

	return result, err
}

// ------ Events -------

func (b *Bridge) DepositParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *BDepositParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BDepositParametersUpdatedSubscription{
		b,
		opts,
	}
}

type BDepositParametersUpdatedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeDepositParametersUpdatedFunc func(
	DepositDustThreshold uint64,
	DepositTreasuryFeeDivisor uint64,
	DepositTxMaxFee uint64,
	DepositRevealAheadPeriod uint32,
	blockNumber uint64,
)

func (dpus *BDepositParametersUpdatedSubscription) OnEvent(
	handler bridgeDepositParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeDepositParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.DepositDustThreshold,
					event.DepositTreasuryFeeDivisor,
					event.DepositTxMaxFee,
					event.DepositRevealAheadPeriod,
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

func (dpus *BDepositParametersUpdatedSubscription) Pipe(
	sink chan *abi.BridgeDepositParametersUpdated,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dpus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past DepositParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dpus.contract.PastDepositParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past DepositParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dpus.contract.watchDepositParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchDepositParametersUpdated(
	sink chan *abi.BridgeDepositParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchDepositParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event DepositParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event DepositParametersUpdated failed "+
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

func (b *Bridge) PastDepositParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeDepositParametersUpdated, error) {
	iterator, err := b.contract.FilterDepositParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DepositParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeDepositParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) DepositRevealedEvent(
	opts *ethereum.SubscribeOpts,
	depositorFilter []common.Address,
	walletPubKeyHashFilter [][20]byte,
) *BDepositRevealedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BDepositRevealedSubscription{
		b,
		opts,
		depositorFilter,
		walletPubKeyHashFilter,
	}
}

type BDepositRevealedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	depositorFilter        []common.Address
	walletPubKeyHashFilter [][20]byte
}

type bridgeDepositRevealedFunc func(
	FundingTxHash [32]byte,
	FundingOutputIndex uint32,
	Depositor common.Address,
	Amount uint64,
	BlindingFactor [8]byte,
	WalletPubKeyHash [20]byte,
	RefundPubKeyHash [20]byte,
	RefundLocktime [4]byte,
	Vault common.Address,
	blockNumber uint64,
)

func (drs *BDepositRevealedSubscription) OnEvent(
	handler bridgeDepositRevealedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeDepositRevealed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.FundingTxHash,
					event.FundingOutputIndex,
					event.Depositor,
					event.Amount,
					event.BlindingFactor,
					event.WalletPubKeyHash,
					event.RefundPubKeyHash,
					event.RefundLocktime,
					event.Vault,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := drs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (drs *BDepositRevealedSubscription) Pipe(
	sink chan *abi.BridgeDepositRevealed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(drs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := drs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - drs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past DepositRevealed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := drs.contract.PastDepositRevealedEvents(
					fromBlock,
					nil,
					drs.depositorFilter,
					drs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past DepositRevealed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := drs.contract.watchDepositRevealed(
		sink,
		drs.depositorFilter,
		drs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchDepositRevealed(
	sink chan *abi.BridgeDepositRevealed,
	depositorFilter []common.Address,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchDepositRevealed(
			&bind.WatchOpts{Context: ctx},
			sink,
			depositorFilter,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event DepositRevealed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event DepositRevealed failed "+
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

func (b *Bridge) PastDepositRevealedEvents(
	startBlock uint64,
	endBlock *uint64,
	depositorFilter []common.Address,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeDepositRevealed, error) {
	iterator, err := b.contract.FilterDepositRevealed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		depositorFilter,
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DepositRevealed events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeDepositRevealed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) DepositVaultFixedEvent(
	opts *ethereum.SubscribeOpts,
	depositKeyFilter []*big.Int,
) *BDepositVaultFixedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BDepositVaultFixedSubscription{
		b,
		opts,
		depositKeyFilter,
	}
}

type BDepositVaultFixedSubscription struct {
	contract         *Bridge
	opts             *ethereum.SubscribeOpts
	depositKeyFilter []*big.Int
}

type bridgeDepositVaultFixedFunc func(
	DepositKey *big.Int,
	NewVault common.Address,
	blockNumber uint64,
)

func (dvfs *BDepositVaultFixedSubscription) OnEvent(
	handler bridgeDepositVaultFixedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeDepositVaultFixed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.DepositKey,
					event.NewVault,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := dvfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (dvfs *BDepositVaultFixedSubscription) Pipe(
	sink chan *abi.BridgeDepositVaultFixed,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(dvfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := dvfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dvfs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past DepositVaultFixed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dvfs.contract.PastDepositVaultFixedEvents(
					fromBlock,
					nil,
					dvfs.depositKeyFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past DepositVaultFixed events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dvfs.contract.watchDepositVaultFixed(
		sink,
		dvfs.depositKeyFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchDepositVaultFixed(
	sink chan *abi.BridgeDepositVaultFixed,
	depositKeyFilter []*big.Int,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchDepositVaultFixed(
			&bind.WatchOpts{Context: ctx},
			sink,
			depositKeyFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event DepositVaultFixed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event DepositVaultFixed failed "+
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

func (b *Bridge) PastDepositVaultFixedEvents(
	startBlock uint64,
	endBlock *uint64,
	depositKeyFilter []*big.Int,
) ([]*abi.BridgeDepositVaultFixed, error) {
	iterator, err := b.contract.FilterDepositVaultFixed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		depositKeyFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DepositVaultFixed events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeDepositVaultFixed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) DepositsSweptEvent(
	opts *ethereum.SubscribeOpts,
) *BDepositsSweptSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BDepositsSweptSubscription{
		b,
		opts,
	}
}

type BDepositsSweptSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeDepositsSweptFunc func(
	WalletPubKeyHash [20]byte,
	SweepTxHash [32]byte,
	blockNumber uint64,
)

func (dss *BDepositsSweptSubscription) OnEvent(
	handler bridgeDepositsSweptFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeDepositsSwept)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.SweepTxHash,
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

func (dss *BDepositsSweptSubscription) Pipe(
	sink chan *abi.BridgeDepositsSwept,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - dss.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past DepositsSwept events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := dss.contract.PastDepositsSweptEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past DepositsSwept events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := dss.contract.watchDepositsSwept(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchDepositsSwept(
	sink chan *abi.BridgeDepositsSwept,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchDepositsSwept(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event DepositsSwept had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event DepositsSwept failed "+
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

func (b *Bridge) PastDepositsSweptEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeDepositsSwept, error) {
	iterator, err := b.contract.FilterDepositsSwept(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past DepositsSwept events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeDepositsSwept, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) FraudChallengeDefeatTimedOutEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BFraudChallengeDefeatTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BFraudChallengeDefeatTimedOutSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BFraudChallengeDefeatTimedOutSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeFraudChallengeDefeatTimedOutFunc func(
	WalletPubKeyHash [20]byte,
	Sighash [32]byte,
	blockNumber uint64,
)

func (fcdtos *BFraudChallengeDefeatTimedOutSubscription) OnEvent(
	handler bridgeFraudChallengeDefeatTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeFraudChallengeDefeatTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.Sighash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := fcdtos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (fcdtos *BFraudChallengeDefeatTimedOutSubscription) Pipe(
	sink chan *abi.BridgeFraudChallengeDefeatTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(fcdtos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := fcdtos.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - fcdtos.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past FraudChallengeDefeatTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := fcdtos.contract.PastFraudChallengeDefeatTimedOutEvents(
					fromBlock,
					nil,
					fcdtos.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past FraudChallengeDefeatTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := fcdtos.contract.watchFraudChallengeDefeatTimedOut(
		sink,
		fcdtos.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchFraudChallengeDefeatTimedOut(
	sink chan *abi.BridgeFraudChallengeDefeatTimedOut,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchFraudChallengeDefeatTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event FraudChallengeDefeatTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event FraudChallengeDefeatTimedOut failed "+
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

func (b *Bridge) PastFraudChallengeDefeatTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeFraudChallengeDefeatTimedOut, error) {
	iterator, err := b.contract.FilterFraudChallengeDefeatTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past FraudChallengeDefeatTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeFraudChallengeDefeatTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) FraudChallengeDefeatedEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BFraudChallengeDefeatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BFraudChallengeDefeatedSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BFraudChallengeDefeatedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeFraudChallengeDefeatedFunc func(
	WalletPubKeyHash [20]byte,
	Sighash [32]byte,
	blockNumber uint64,
)

func (fcds *BFraudChallengeDefeatedSubscription) OnEvent(
	handler bridgeFraudChallengeDefeatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeFraudChallengeDefeated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.Sighash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := fcds.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (fcds *BFraudChallengeDefeatedSubscription) Pipe(
	sink chan *abi.BridgeFraudChallengeDefeated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(fcds.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := fcds.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - fcds.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past FraudChallengeDefeated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := fcds.contract.PastFraudChallengeDefeatedEvents(
					fromBlock,
					nil,
					fcds.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past FraudChallengeDefeated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := fcds.contract.watchFraudChallengeDefeated(
		sink,
		fcds.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchFraudChallengeDefeated(
	sink chan *abi.BridgeFraudChallengeDefeated,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchFraudChallengeDefeated(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event FraudChallengeDefeated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event FraudChallengeDefeated failed "+
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

func (b *Bridge) PastFraudChallengeDefeatedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeFraudChallengeDefeated, error) {
	iterator, err := b.contract.FilterFraudChallengeDefeated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past FraudChallengeDefeated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeFraudChallengeDefeated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) FraudChallengeSubmittedEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BFraudChallengeSubmittedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BFraudChallengeSubmittedSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BFraudChallengeSubmittedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeFraudChallengeSubmittedFunc func(
	WalletPubKeyHash [20]byte,
	Sighash [32]byte,
	V uint8,
	R [32]byte,
	S [32]byte,
	blockNumber uint64,
)

func (fcss *BFraudChallengeSubmittedSubscription) OnEvent(
	handler bridgeFraudChallengeSubmittedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeFraudChallengeSubmitted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.Sighash,
					event.V,
					event.R,
					event.S,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := fcss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (fcss *BFraudChallengeSubmittedSubscription) Pipe(
	sink chan *abi.BridgeFraudChallengeSubmitted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(fcss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := fcss.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - fcss.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past FraudChallengeSubmitted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := fcss.contract.PastFraudChallengeSubmittedEvents(
					fromBlock,
					nil,
					fcss.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past FraudChallengeSubmitted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := fcss.contract.watchFraudChallengeSubmitted(
		sink,
		fcss.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchFraudChallengeSubmitted(
	sink chan *abi.BridgeFraudChallengeSubmitted,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchFraudChallengeSubmitted(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event FraudChallengeSubmitted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event FraudChallengeSubmitted failed "+
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

func (b *Bridge) PastFraudChallengeSubmittedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeFraudChallengeSubmitted, error) {
	iterator, err := b.contract.FilterFraudChallengeSubmitted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past FraudChallengeSubmitted events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeFraudChallengeSubmitted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) FraudParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *BFraudParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BFraudParametersUpdatedSubscription{
		b,
		opts,
	}
}

type BFraudParametersUpdatedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeFraudParametersUpdatedFunc func(
	FraudChallengeDepositAmount *big.Int,
	FraudChallengeDefeatTimeout uint32,
	FraudSlashingAmount *big.Int,
	FraudNotifierRewardMultiplier uint32,
	blockNumber uint64,
)

func (fpus *BFraudParametersUpdatedSubscription) OnEvent(
	handler bridgeFraudParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeFraudParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.FraudChallengeDepositAmount,
					event.FraudChallengeDefeatTimeout,
					event.FraudSlashingAmount,
					event.FraudNotifierRewardMultiplier,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := fpus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (fpus *BFraudParametersUpdatedSubscription) Pipe(
	sink chan *abi.BridgeFraudParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(fpus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := fpus.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - fpus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past FraudParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := fpus.contract.PastFraudParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past FraudParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := fpus.contract.watchFraudParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchFraudParametersUpdated(
	sink chan *abi.BridgeFraudParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchFraudParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event FraudParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event FraudParametersUpdated failed "+
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

func (b *Bridge) PastFraudParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeFraudParametersUpdated, error) {
	iterator, err := b.contract.FilterFraudParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past FraudParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeFraudParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) GovernanceTransferredEvent(
	opts *ethereum.SubscribeOpts,
) *BGovernanceTransferredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BGovernanceTransferredSubscription{
		b,
		opts,
	}
}

type BGovernanceTransferredSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeGovernanceTransferredFunc func(
	OldGovernance common.Address,
	NewGovernance common.Address,
	blockNumber uint64,
)

func (gts *BGovernanceTransferredSubscription) OnEvent(
	handler bridgeGovernanceTransferredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeGovernanceTransferred)
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

func (gts *BGovernanceTransferredSubscription) Pipe(
	sink chan *abi.BridgeGovernanceTransferred,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - gts.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past GovernanceTransferred events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := gts.contract.PastGovernanceTransferredEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
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

func (b *Bridge) watchGovernanceTransferred(
	sink chan *abi.BridgeGovernanceTransferred,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchGovernanceTransferred(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event GovernanceTransferred had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
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

func (b *Bridge) PastGovernanceTransferredEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeGovernanceTransferred, error) {
	iterator, err := b.contract.FilterGovernanceTransferred(
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

	events := make([]*abi.BridgeGovernanceTransferred, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) InitializedEvent(
	opts *ethereum.SubscribeOpts,
) *BInitializedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BInitializedSubscription{
		b,
		opts,
	}
}

type BInitializedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeInitializedFunc func(
	Version uint8,
	blockNumber uint64,
)

func (is *BInitializedSubscription) OnEvent(
	handler bridgeInitializedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeInitialized)
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

func (is *BInitializedSubscription) Pipe(
	sink chan *abi.BridgeInitialized,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - is.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past Initialized events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := is.contract.PastInitializedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
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

func (b *Bridge) watchInitialized(
	sink chan *abi.BridgeInitialized,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchInitialized(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event Initialized had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
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

func (b *Bridge) PastInitializedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeInitialized, error) {
	iterator, err := b.contract.FilterInitialized(
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

	events := make([]*abi.BridgeInitialized, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovedFundsSweepTimedOutEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BMovedFundsSweepTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovedFundsSweepTimedOutSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BMovedFundsSweepTimedOutSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeMovedFundsSweepTimedOutFunc func(
	WalletPubKeyHash [20]byte,
	MovingFundsTxHash [32]byte,
	MovingFundsTxOutputIndex uint32,
	blockNumber uint64,
)

func (mfstos *BMovedFundsSweepTimedOutSubscription) OnEvent(
	handler bridgeMovedFundsSweepTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovedFundsSweepTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.MovingFundsTxHash,
					event.MovingFundsTxOutputIndex,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mfstos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mfstos *BMovedFundsSweepTimedOutSubscription) Pipe(
	sink chan *abi.BridgeMovedFundsSweepTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mfstos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mfstos.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mfstos.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovedFundsSweepTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mfstos.contract.PastMovedFundsSweepTimedOutEvents(
					fromBlock,
					nil,
					mfstos.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovedFundsSweepTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mfstos.contract.watchMovedFundsSweepTimedOut(
		sink,
		mfstos.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovedFundsSweepTimedOut(
	sink chan *abi.BridgeMovedFundsSweepTimedOut,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovedFundsSweepTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovedFundsSweepTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovedFundsSweepTimedOut failed "+
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

func (b *Bridge) PastMovedFundsSweepTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeMovedFundsSweepTimedOut, error) {
	iterator, err := b.contract.FilterMovedFundsSweepTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovedFundsSweepTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovedFundsSweepTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovedFundsSweptEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BMovedFundsSweptSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovedFundsSweptSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BMovedFundsSweptSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeMovedFundsSweptFunc func(
	WalletPubKeyHash [20]byte,
	SweepTxHash [32]byte,
	blockNumber uint64,
)

func (mfss *BMovedFundsSweptSubscription) OnEvent(
	handler bridgeMovedFundsSweptFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovedFundsSwept)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.SweepTxHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mfss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mfss *BMovedFundsSweptSubscription) Pipe(
	sink chan *abi.BridgeMovedFundsSwept,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mfss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mfss.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mfss.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovedFundsSwept events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mfss.contract.PastMovedFundsSweptEvents(
					fromBlock,
					nil,
					mfss.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovedFundsSwept events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mfss.contract.watchMovedFundsSwept(
		sink,
		mfss.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovedFundsSwept(
	sink chan *abi.BridgeMovedFundsSwept,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovedFundsSwept(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovedFundsSwept had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovedFundsSwept failed "+
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

func (b *Bridge) PastMovedFundsSweptEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeMovedFundsSwept, error) {
	iterator, err := b.contract.FilterMovedFundsSwept(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovedFundsSwept events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovedFundsSwept, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovingFundsBelowDustReportedEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BMovingFundsBelowDustReportedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovingFundsBelowDustReportedSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BMovingFundsBelowDustReportedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeMovingFundsBelowDustReportedFunc func(
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (mfbdrs *BMovingFundsBelowDustReportedSubscription) OnEvent(
	handler bridgeMovingFundsBelowDustReportedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovingFundsBelowDustReported)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mfbdrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mfbdrs *BMovingFundsBelowDustReportedSubscription) Pipe(
	sink chan *abi.BridgeMovingFundsBelowDustReported,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mfbdrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mfbdrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mfbdrs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovingFundsBelowDustReported events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mfbdrs.contract.PastMovingFundsBelowDustReportedEvents(
					fromBlock,
					nil,
					mfbdrs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovingFundsBelowDustReported events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mfbdrs.contract.watchMovingFundsBelowDustReported(
		sink,
		mfbdrs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovingFundsBelowDustReported(
	sink chan *abi.BridgeMovingFundsBelowDustReported,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovingFundsBelowDustReported(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovingFundsBelowDustReported had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovingFundsBelowDustReported failed "+
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

func (b *Bridge) PastMovingFundsBelowDustReportedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeMovingFundsBelowDustReported, error) {
	iterator, err := b.contract.FilterMovingFundsBelowDustReported(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovingFundsBelowDustReported events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovingFundsBelowDustReported, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovingFundsCommitmentSubmittedEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BMovingFundsCommitmentSubmittedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovingFundsCommitmentSubmittedSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BMovingFundsCommitmentSubmittedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeMovingFundsCommitmentSubmittedFunc func(
	WalletPubKeyHash [20]byte,
	TargetWallets [][20]byte,
	Submitter common.Address,
	blockNumber uint64,
)

func (mfcss *BMovingFundsCommitmentSubmittedSubscription) OnEvent(
	handler bridgeMovingFundsCommitmentSubmittedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovingFundsCommitmentSubmitted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.TargetWallets,
					event.Submitter,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mfcss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mfcss *BMovingFundsCommitmentSubmittedSubscription) Pipe(
	sink chan *abi.BridgeMovingFundsCommitmentSubmitted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mfcss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mfcss.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mfcss.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovingFundsCommitmentSubmitted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mfcss.contract.PastMovingFundsCommitmentSubmittedEvents(
					fromBlock,
					nil,
					mfcss.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovingFundsCommitmentSubmitted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mfcss.contract.watchMovingFundsCommitmentSubmitted(
		sink,
		mfcss.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovingFundsCommitmentSubmitted(
	sink chan *abi.BridgeMovingFundsCommitmentSubmitted,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovingFundsCommitmentSubmitted(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovingFundsCommitmentSubmitted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovingFundsCommitmentSubmitted failed "+
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

func (b *Bridge) PastMovingFundsCommitmentSubmittedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeMovingFundsCommitmentSubmitted, error) {
	iterator, err := b.contract.FilterMovingFundsCommitmentSubmitted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovingFundsCommitmentSubmitted events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovingFundsCommitmentSubmitted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovingFundsCompletedEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BMovingFundsCompletedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovingFundsCompletedSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BMovingFundsCompletedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeMovingFundsCompletedFunc func(
	WalletPubKeyHash [20]byte,
	MovingFundsTxHash [32]byte,
	blockNumber uint64,
)

func (mfcs *BMovingFundsCompletedSubscription) OnEvent(
	handler bridgeMovingFundsCompletedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovingFundsCompleted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.MovingFundsTxHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mfcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mfcs *BMovingFundsCompletedSubscription) Pipe(
	sink chan *abi.BridgeMovingFundsCompleted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mfcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mfcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mfcs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovingFundsCompleted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mfcs.contract.PastMovingFundsCompletedEvents(
					fromBlock,
					nil,
					mfcs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovingFundsCompleted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mfcs.contract.watchMovingFundsCompleted(
		sink,
		mfcs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovingFundsCompleted(
	sink chan *abi.BridgeMovingFundsCompleted,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovingFundsCompleted(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovingFundsCompleted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovingFundsCompleted failed "+
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

func (b *Bridge) PastMovingFundsCompletedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeMovingFundsCompleted, error) {
	iterator, err := b.contract.FilterMovingFundsCompleted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovingFundsCompleted events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovingFundsCompleted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovingFundsParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *BMovingFundsParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovingFundsParametersUpdatedSubscription{
		b,
		opts,
	}
}

type BMovingFundsParametersUpdatedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeMovingFundsParametersUpdatedFunc func(
	MovingFundsTxMaxTotalFee uint64,
	MovingFundsDustThreshold uint64,
	MovingFundsTimeoutResetDelay uint32,
	MovingFundsTimeout uint32,
	MovingFundsTimeoutSlashingAmount *big.Int,
	MovingFundsTimeoutNotifierRewardMultiplier uint32,
	MovingFundsCommitmentGasOffset uint16,
	MovedFundsSweepTxMaxTotalFee uint64,
	MovedFundsSweepTimeout uint32,
	MovedFundsSweepTimeoutSlashingAmount *big.Int,
	MovedFundsSweepTimeoutNotifierRewardMultiplier uint32,
	blockNumber uint64,
)

func (mfpus *BMovingFundsParametersUpdatedSubscription) OnEvent(
	handler bridgeMovingFundsParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovingFundsParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.MovingFundsTxMaxTotalFee,
					event.MovingFundsDustThreshold,
					event.MovingFundsTimeoutResetDelay,
					event.MovingFundsTimeout,
					event.MovingFundsTimeoutSlashingAmount,
					event.MovingFundsTimeoutNotifierRewardMultiplier,
					event.MovingFundsCommitmentGasOffset,
					event.MovedFundsSweepTxMaxTotalFee,
					event.MovedFundsSweepTimeout,
					event.MovedFundsSweepTimeoutSlashingAmount,
					event.MovedFundsSweepTimeoutNotifierRewardMultiplier,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mfpus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mfpus *BMovingFundsParametersUpdatedSubscription) Pipe(
	sink chan *abi.BridgeMovingFundsParametersUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mfpus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mfpus.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mfpus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovingFundsParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mfpus.contract.PastMovingFundsParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovingFundsParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mfpus.contract.watchMovingFundsParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovingFundsParametersUpdated(
	sink chan *abi.BridgeMovingFundsParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovingFundsParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovingFundsParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovingFundsParametersUpdated failed "+
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

func (b *Bridge) PastMovingFundsParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeMovingFundsParametersUpdated, error) {
	iterator, err := b.contract.FilterMovingFundsParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovingFundsParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovingFundsParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovingFundsTimedOutEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BMovingFundsTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovingFundsTimedOutSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BMovingFundsTimedOutSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeMovingFundsTimedOutFunc func(
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (mftos *BMovingFundsTimedOutSubscription) OnEvent(
	handler bridgeMovingFundsTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovingFundsTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mftos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mftos *BMovingFundsTimedOutSubscription) Pipe(
	sink chan *abi.BridgeMovingFundsTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mftos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mftos.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mftos.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovingFundsTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mftos.contract.PastMovingFundsTimedOutEvents(
					fromBlock,
					nil,
					mftos.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovingFundsTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mftos.contract.watchMovingFundsTimedOut(
		sink,
		mftos.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovingFundsTimedOut(
	sink chan *abi.BridgeMovingFundsTimedOut,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovingFundsTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovingFundsTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovingFundsTimedOut failed "+
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

func (b *Bridge) PastMovingFundsTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeMovingFundsTimedOut, error) {
	iterator, err := b.contract.FilterMovingFundsTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovingFundsTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovingFundsTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) MovingFundsTimeoutResetEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BMovingFundsTimeoutResetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BMovingFundsTimeoutResetSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BMovingFundsTimeoutResetSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeMovingFundsTimeoutResetFunc func(
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (mftrs *BMovingFundsTimeoutResetSubscription) OnEvent(
	handler bridgeMovingFundsTimeoutResetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeMovingFundsTimeoutReset)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := mftrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (mftrs *BMovingFundsTimeoutResetSubscription) Pipe(
	sink chan *abi.BridgeMovingFundsTimeoutReset,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(mftrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := mftrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - mftrs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past MovingFundsTimeoutReset events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := mftrs.contract.PastMovingFundsTimeoutResetEvents(
					fromBlock,
					nil,
					mftrs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past MovingFundsTimeoutReset events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := mftrs.contract.watchMovingFundsTimeoutReset(
		sink,
		mftrs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchMovingFundsTimeoutReset(
	sink chan *abi.BridgeMovingFundsTimeoutReset,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchMovingFundsTimeoutReset(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event MovingFundsTimeoutReset had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event MovingFundsTimeoutReset failed "+
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

func (b *Bridge) PastMovingFundsTimeoutResetEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeMovingFundsTimeoutReset, error) {
	iterator, err := b.contract.FilterMovingFundsTimeoutReset(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past MovingFundsTimeoutReset events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeMovingFundsTimeoutReset, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) NewWalletRegisteredEvent(
	opts *ethereum.SubscribeOpts,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) *BNewWalletRegisteredSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BNewWalletRegisteredSubscription{
		b,
		opts,
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	}
}

type BNewWalletRegisteredSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	ecdsaWalletIDFilter    [][32]byte
	walletPubKeyHashFilter [][20]byte
}

type bridgeNewWalletRegisteredFunc func(
	EcdsaWalletID [32]byte,
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (nwrs *BNewWalletRegisteredSubscription) OnEvent(
	handler bridgeNewWalletRegisteredFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeNewWalletRegistered)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.EcdsaWalletID,
					event.WalletPubKeyHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := nwrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (nwrs *BNewWalletRegisteredSubscription) Pipe(
	sink chan *abi.BridgeNewWalletRegistered,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(nwrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := nwrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - nwrs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past NewWalletRegistered events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := nwrs.contract.PastNewWalletRegisteredEvents(
					fromBlock,
					nil,
					nwrs.ecdsaWalletIDFilter,
					nwrs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past NewWalletRegistered events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := nwrs.contract.watchNewWalletRegistered(
		sink,
		nwrs.ecdsaWalletIDFilter,
		nwrs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchNewWalletRegistered(
	sink chan *abi.BridgeNewWalletRegistered,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchNewWalletRegistered(
			&bind.WatchOpts{Context: ctx},
			sink,
			ecdsaWalletIDFilter,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event NewWalletRegistered had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event NewWalletRegistered failed "+
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

func (b *Bridge) PastNewWalletRegisteredEvents(
	startBlock uint64,
	endBlock *uint64,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeNewWalletRegistered, error) {
	iterator, err := b.contract.FilterNewWalletRegistered(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past NewWalletRegistered events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeNewWalletRegistered, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) NewWalletRequestedEvent(
	opts *ethereum.SubscribeOpts,
) *BNewWalletRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BNewWalletRequestedSubscription{
		b,
		opts,
	}
}

type BNewWalletRequestedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeNewWalletRequestedFunc func(
	blockNumber uint64,
)

func (nwrs *BNewWalletRequestedSubscription) OnEvent(
	handler bridgeNewWalletRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeNewWalletRequested)
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

	sub := nwrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (nwrs *BNewWalletRequestedSubscription) Pipe(
	sink chan *abi.BridgeNewWalletRequested,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(nwrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := nwrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - nwrs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past NewWalletRequested events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := nwrs.contract.PastNewWalletRequestedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past NewWalletRequested events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := nwrs.contract.watchNewWalletRequested(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchNewWalletRequested(
	sink chan *abi.BridgeNewWalletRequested,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchNewWalletRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event NewWalletRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event NewWalletRequested failed "+
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

func (b *Bridge) PastNewWalletRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeNewWalletRequested, error) {
	iterator, err := b.contract.FilterNewWalletRequested(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past NewWalletRequested events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeNewWalletRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) RebateStakingRepairedEvent(
	opts *ethereum.SubscribeOpts,
) *BRebateStakingRepairedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BRebateStakingRepairedSubscription{
		b,
		opts,
	}
}

type BRebateStakingRepairedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeRebateStakingRepairedFunc func(
	OldRebateStaking common.Address,
	NewRebateStaking common.Address,
	blockNumber uint64,
)

func (rsrs *BRebateStakingRepairedSubscription) OnEvent(
	handler bridgeRebateStakingRepairedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeRebateStakingRepaired)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.OldRebateStaking,
					event.NewRebateStaking,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rsrs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rsrs *BRebateStakingRepairedSubscription) Pipe(
	sink chan *abi.BridgeRebateStakingRepaired,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rsrs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rsrs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rsrs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past RebateStakingRepaired events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rsrs.contract.PastRebateStakingRepairedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past RebateStakingRepaired events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rsrs.contract.watchRebateStakingRepaired(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchRebateStakingRepaired(
	sink chan *abi.BridgeRebateStakingRepaired,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchRebateStakingRepaired(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event RebateStakingRepaired had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event RebateStakingRepaired failed "+
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

func (b *Bridge) PastRebateStakingRepairedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeRebateStakingRepaired, error) {
	iterator, err := b.contract.FilterRebateStakingRepaired(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RebateStakingRepaired events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeRebateStakingRepaired, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) RebateStakingSetEvent(
	opts *ethereum.SubscribeOpts,
) *BRebateStakingSetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BRebateStakingSetSubscription{
		b,
		opts,
	}
}

type BRebateStakingSetSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeRebateStakingSetFunc func(
	RebateStaking common.Address,
	blockNumber uint64,
)

func (rsss *BRebateStakingSetSubscription) OnEvent(
	handler bridgeRebateStakingSetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeRebateStakingSet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RebateStaking,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rsss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rsss *BRebateStakingSetSubscription) Pipe(
	sink chan *abi.BridgeRebateStakingSet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rsss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rsss.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rsss.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past RebateStakingSet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rsss.contract.PastRebateStakingSetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past RebateStakingSet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rsss.contract.watchRebateStakingSet(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchRebateStakingSet(
	sink chan *abi.BridgeRebateStakingSet,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchRebateStakingSet(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event RebateStakingSet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event RebateStakingSet failed "+
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

func (b *Bridge) PastRebateStakingSetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeRebateStakingSet, error) {
	iterator, err := b.contract.FilterRebateStakingSet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RebateStakingSet events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeRebateStakingSet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) RedemptionParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *BRedemptionParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BRedemptionParametersUpdatedSubscription{
		b,
		opts,
	}
}

type BRedemptionParametersUpdatedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeRedemptionParametersUpdatedFunc func(
	RedemptionDustThreshold uint64,
	RedemptionTreasuryFeeDivisor uint64,
	RedemptionTxMaxFee uint64,
	RedemptionTxMaxTotalFee uint64,
	RedemptionTimeout uint32,
	RedemptionTimeoutSlashingAmount *big.Int,
	RedemptionTimeoutNotifierRewardMultiplier uint32,
	blockNumber uint64,
)

func (rpus *BRedemptionParametersUpdatedSubscription) OnEvent(
	handler bridgeRedemptionParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeRedemptionParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RedemptionDustThreshold,
					event.RedemptionTreasuryFeeDivisor,
					event.RedemptionTxMaxFee,
					event.RedemptionTxMaxTotalFee,
					event.RedemptionTimeout,
					event.RedemptionTimeoutSlashingAmount,
					event.RedemptionTimeoutNotifierRewardMultiplier,
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

func (rpus *BRedemptionParametersUpdatedSubscription) Pipe(
	sink chan *abi.BridgeRedemptionParametersUpdated,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rpus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past RedemptionParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rpus.contract.PastRedemptionParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past RedemptionParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rpus.contract.watchRedemptionParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchRedemptionParametersUpdated(
	sink chan *abi.BridgeRedemptionParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchRedemptionParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event RedemptionParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event RedemptionParametersUpdated failed "+
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

func (b *Bridge) PastRedemptionParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeRedemptionParametersUpdated, error) {
	iterator, err := b.contract.FilterRedemptionParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RedemptionParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeRedemptionParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) RedemptionRequestedEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
	redeemerFilter []common.Address,
) *BRedemptionRequestedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BRedemptionRequestedSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
		redeemerFilter,
	}
}

type BRedemptionRequestedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
	redeemerFilter         []common.Address
}

type bridgeRedemptionRequestedFunc func(
	WalletPubKeyHash [20]byte,
	RedeemerOutputScript []byte,
	Redeemer common.Address,
	RequestedAmount uint64,
	TreasuryFee uint64,
	TxMaxFee uint64,
	blockNumber uint64,
)

func (rrs *BRedemptionRequestedSubscription) OnEvent(
	handler bridgeRedemptionRequestedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeRedemptionRequested)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.RedeemerOutputScript,
					event.Redeemer,
					event.RequestedAmount,
					event.TreasuryFee,
					event.TxMaxFee,
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

func (rrs *BRedemptionRequestedSubscription) Pipe(
	sink chan *abi.BridgeRedemptionRequested,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rrs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past RedemptionRequested events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rrs.contract.PastRedemptionRequestedEvents(
					fromBlock,
					nil,
					rrs.walletPubKeyHashFilter,
					rrs.redeemerFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past RedemptionRequested events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rrs.contract.watchRedemptionRequested(
		sink,
		rrs.walletPubKeyHashFilter,
		rrs.redeemerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchRedemptionRequested(
	sink chan *abi.BridgeRedemptionRequested,
	walletPubKeyHashFilter [][20]byte,
	redeemerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchRedemptionRequested(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
			redeemerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event RedemptionRequested had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event RedemptionRequested failed "+
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

func (b *Bridge) PastRedemptionRequestedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
	redeemerFilter []common.Address,
) ([]*abi.BridgeRedemptionRequested, error) {
	iterator, err := b.contract.FilterRedemptionRequested(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
		redeemerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RedemptionRequested events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeRedemptionRequested, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) RedemptionTimedOutEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BRedemptionTimedOutSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BRedemptionTimedOutSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BRedemptionTimedOutSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeRedemptionTimedOutFunc func(
	WalletPubKeyHash [20]byte,
	RedeemerOutputScript []byte,
	blockNumber uint64,
)

func (rtos *BRedemptionTimedOutSubscription) OnEvent(
	handler bridgeRedemptionTimedOutFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeRedemptionTimedOut)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.RedeemerOutputScript,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rtos.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rtos *BRedemptionTimedOutSubscription) Pipe(
	sink chan *abi.BridgeRedemptionTimedOut,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rtos.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rtos.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rtos.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past RedemptionTimedOut events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rtos.contract.PastRedemptionTimedOutEvents(
					fromBlock,
					nil,
					rtos.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past RedemptionTimedOut events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rtos.contract.watchRedemptionTimedOut(
		sink,
		rtos.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchRedemptionTimedOut(
	sink chan *abi.BridgeRedemptionTimedOut,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchRedemptionTimedOut(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event RedemptionTimedOut had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event RedemptionTimedOut failed "+
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

func (b *Bridge) PastRedemptionTimedOutEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeRedemptionTimedOut, error) {
	iterator, err := b.contract.FilterRedemptionTimedOut(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RedemptionTimedOut events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeRedemptionTimedOut, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) RedemptionWatchtowerSetEvent(
	opts *ethereum.SubscribeOpts,
) *BRedemptionWatchtowerSetSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BRedemptionWatchtowerSetSubscription{
		b,
		opts,
	}
}

type BRedemptionWatchtowerSetSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeRedemptionWatchtowerSetFunc func(
	RedemptionWatchtower common.Address,
	blockNumber uint64,
)

func (rwss *BRedemptionWatchtowerSetSubscription) OnEvent(
	handler bridgeRedemptionWatchtowerSetFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeRedemptionWatchtowerSet)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.RedemptionWatchtower,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rwss.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rwss *BRedemptionWatchtowerSetSubscription) Pipe(
	sink chan *abi.BridgeRedemptionWatchtowerSet,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rwss.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rwss.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rwss.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past RedemptionWatchtowerSet events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rwss.contract.PastRedemptionWatchtowerSetEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past RedemptionWatchtowerSet events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rwss.contract.watchRedemptionWatchtowerSet(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchRedemptionWatchtowerSet(
	sink chan *abi.BridgeRedemptionWatchtowerSet,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchRedemptionWatchtowerSet(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event RedemptionWatchtowerSet had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event RedemptionWatchtowerSet failed "+
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

func (b *Bridge) PastRedemptionWatchtowerSetEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeRedemptionWatchtowerSet, error) {
	iterator, err := b.contract.FilterRedemptionWatchtowerSet(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RedemptionWatchtowerSet events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeRedemptionWatchtowerSet, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) RedemptionsCompletedEvent(
	opts *ethereum.SubscribeOpts,
	walletPubKeyHashFilter [][20]byte,
) *BRedemptionsCompletedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BRedemptionsCompletedSubscription{
		b,
		opts,
		walletPubKeyHashFilter,
	}
}

type BRedemptionsCompletedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	walletPubKeyHashFilter [][20]byte
}

type bridgeRedemptionsCompletedFunc func(
	WalletPubKeyHash [20]byte,
	RedemptionTxHash [32]byte,
	blockNumber uint64,
)

func (rcs *BRedemptionsCompletedSubscription) OnEvent(
	handler bridgeRedemptionsCompletedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeRedemptionsCompleted)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletPubKeyHash,
					event.RedemptionTxHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := rcs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (rcs *BRedemptionsCompletedSubscription) Pipe(
	sink chan *abi.BridgeRedemptionsCompleted,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(rcs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := rcs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - rcs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past RedemptionsCompleted events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := rcs.contract.PastRedemptionsCompletedEvents(
					fromBlock,
					nil,
					rcs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past RedemptionsCompleted events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := rcs.contract.watchRedemptionsCompleted(
		sink,
		rcs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchRedemptionsCompleted(
	sink chan *abi.BridgeRedemptionsCompleted,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchRedemptionsCompleted(
			&bind.WatchOpts{Context: ctx},
			sink,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event RedemptionsCompleted had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event RedemptionsCompleted failed "+
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

func (b *Bridge) PastRedemptionsCompletedEvents(
	startBlock uint64,
	endBlock *uint64,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeRedemptionsCompleted, error) {
	iterator, err := b.contract.FilterRedemptionsCompleted(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past RedemptionsCompleted events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeRedemptionsCompleted, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) SpvMaintainerStatusUpdatedEvent(
	opts *ethereum.SubscribeOpts,
	spvMaintainerFilter []common.Address,
) *BSpvMaintainerStatusUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BSpvMaintainerStatusUpdatedSubscription{
		b,
		opts,
		spvMaintainerFilter,
	}
}

type BSpvMaintainerStatusUpdatedSubscription struct {
	contract            *Bridge
	opts                *ethereum.SubscribeOpts
	spvMaintainerFilter []common.Address
}

type bridgeSpvMaintainerStatusUpdatedFunc func(
	SpvMaintainer common.Address,
	IsTrusted bool,
	blockNumber uint64,
)

func (smsus *BSpvMaintainerStatusUpdatedSubscription) OnEvent(
	handler bridgeSpvMaintainerStatusUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeSpvMaintainerStatusUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.SpvMaintainer,
					event.IsTrusted,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := smsus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (smsus *BSpvMaintainerStatusUpdatedSubscription) Pipe(
	sink chan *abi.BridgeSpvMaintainerStatusUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(smsus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := smsus.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - smsus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past SpvMaintainerStatusUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := smsus.contract.PastSpvMaintainerStatusUpdatedEvents(
					fromBlock,
					nil,
					smsus.spvMaintainerFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past SpvMaintainerStatusUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := smsus.contract.watchSpvMaintainerStatusUpdated(
		sink,
		smsus.spvMaintainerFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchSpvMaintainerStatusUpdated(
	sink chan *abi.BridgeSpvMaintainerStatusUpdated,
	spvMaintainerFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchSpvMaintainerStatusUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
			spvMaintainerFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event SpvMaintainerStatusUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event SpvMaintainerStatusUpdated failed "+
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

func (b *Bridge) PastSpvMaintainerStatusUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
	spvMaintainerFilter []common.Address,
) ([]*abi.BridgeSpvMaintainerStatusUpdated, error) {
	iterator, err := b.contract.FilterSpvMaintainerStatusUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		spvMaintainerFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past SpvMaintainerStatusUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeSpvMaintainerStatusUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) TreasuryUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *BTreasuryUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BTreasuryUpdatedSubscription{
		b,
		opts,
	}
}

type BTreasuryUpdatedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeTreasuryUpdatedFunc func(
	Treasury common.Address,
	blockNumber uint64,
)

func (tus *BTreasuryUpdatedSubscription) OnEvent(
	handler bridgeTreasuryUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeTreasuryUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Treasury,
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

func (tus *BTreasuryUpdatedSubscription) Pipe(
	sink chan *abi.BridgeTreasuryUpdated,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - tus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past TreasuryUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := tus.contract.PastTreasuryUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past TreasuryUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := tus.contract.watchTreasuryUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchTreasuryUpdated(
	sink chan *abi.BridgeTreasuryUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchTreasuryUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event TreasuryUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event TreasuryUpdated failed "+
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

func (b *Bridge) PastTreasuryUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeTreasuryUpdated, error) {
	iterator, err := b.contract.FilterTreasuryUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past TreasuryUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeTreasuryUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) VaultStatusUpdatedEvent(
	opts *ethereum.SubscribeOpts,
	vaultFilter []common.Address,
) *BVaultStatusUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BVaultStatusUpdatedSubscription{
		b,
		opts,
		vaultFilter,
	}
}

type BVaultStatusUpdatedSubscription struct {
	contract    *Bridge
	opts        *ethereum.SubscribeOpts
	vaultFilter []common.Address
}

type bridgeVaultStatusUpdatedFunc func(
	Vault common.Address,
	IsTrusted bool,
	blockNumber uint64,
)

func (vsus *BVaultStatusUpdatedSubscription) OnEvent(
	handler bridgeVaultStatusUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeVaultStatusUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.Vault,
					event.IsTrusted,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := vsus.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (vsus *BVaultStatusUpdatedSubscription) Pipe(
	sink chan *abi.BridgeVaultStatusUpdated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(vsus.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := vsus.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - vsus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past VaultStatusUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := vsus.contract.PastVaultStatusUpdatedEvents(
					fromBlock,
					nil,
					vsus.vaultFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past VaultStatusUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := vsus.contract.watchVaultStatusUpdated(
		sink,
		vsus.vaultFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchVaultStatusUpdated(
	sink chan *abi.BridgeVaultStatusUpdated,
	vaultFilter []common.Address,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchVaultStatusUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
			vaultFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event VaultStatusUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event VaultStatusUpdated failed "+
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

func (b *Bridge) PastVaultStatusUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
	vaultFilter []common.Address,
) ([]*abi.BridgeVaultStatusUpdated, error) {
	iterator, err := b.contract.FilterVaultStatusUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		vaultFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past VaultStatusUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeVaultStatusUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) WalletClosedEvent(
	opts *ethereum.SubscribeOpts,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) *BWalletClosedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BWalletClosedSubscription{
		b,
		opts,
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	}
}

type BWalletClosedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	ecdsaWalletIDFilter    [][32]byte
	walletPubKeyHashFilter [][20]byte
}

type bridgeWalletClosedFunc func(
	EcdsaWalletID [32]byte,
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (wcs *BWalletClosedSubscription) OnEvent(
	handler bridgeWalletClosedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeWalletClosed)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.EcdsaWalletID,
					event.WalletPubKeyHash,
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

func (wcs *BWalletClosedSubscription) Pipe(
	sink chan *abi.BridgeWalletClosed,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wcs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past WalletClosed events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wcs.contract.PastWalletClosedEvents(
					fromBlock,
					nil,
					wcs.ecdsaWalletIDFilter,
					wcs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
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
		wcs.ecdsaWalletIDFilter,
		wcs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchWalletClosed(
	sink chan *abi.BridgeWalletClosed,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchWalletClosed(
			&bind.WatchOpts{Context: ctx},
			sink,
			ecdsaWalletIDFilter,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event WalletClosed had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
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

func (b *Bridge) PastWalletClosedEvents(
	startBlock uint64,
	endBlock *uint64,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeWalletClosed, error) {
	iterator, err := b.contract.FilterWalletClosed(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletClosed events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeWalletClosed, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) WalletClosingEvent(
	opts *ethereum.SubscribeOpts,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) *BWalletClosingSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BWalletClosingSubscription{
		b,
		opts,
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	}
}

type BWalletClosingSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	ecdsaWalletIDFilter    [][32]byte
	walletPubKeyHashFilter [][20]byte
}

type bridgeWalletClosingFunc func(
	EcdsaWalletID [32]byte,
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (wcs *BWalletClosingSubscription) OnEvent(
	handler bridgeWalletClosingFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeWalletClosing)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.EcdsaWalletID,
					event.WalletPubKeyHash,
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

func (wcs *BWalletClosingSubscription) Pipe(
	sink chan *abi.BridgeWalletClosing,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wcs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past WalletClosing events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wcs.contract.PastWalletClosingEvents(
					fromBlock,
					nil,
					wcs.ecdsaWalletIDFilter,
					wcs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past WalletClosing events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wcs.contract.watchWalletClosing(
		sink,
		wcs.ecdsaWalletIDFilter,
		wcs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchWalletClosing(
	sink chan *abi.BridgeWalletClosing,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchWalletClosing(
			&bind.WatchOpts{Context: ctx},
			sink,
			ecdsaWalletIDFilter,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event WalletClosing had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event WalletClosing failed "+
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

func (b *Bridge) PastWalletClosingEvents(
	startBlock uint64,
	endBlock *uint64,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeWalletClosing, error) {
	iterator, err := b.contract.FilterWalletClosing(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletClosing events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeWalletClosing, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) WalletMovingFundsEvent(
	opts *ethereum.SubscribeOpts,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) *BWalletMovingFundsSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BWalletMovingFundsSubscription{
		b,
		opts,
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	}
}

type BWalletMovingFundsSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	ecdsaWalletIDFilter    [][32]byte
	walletPubKeyHashFilter [][20]byte
}

type bridgeWalletMovingFundsFunc func(
	EcdsaWalletID [32]byte,
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (wmfs *BWalletMovingFundsSubscription) OnEvent(
	handler bridgeWalletMovingFundsFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeWalletMovingFunds)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.EcdsaWalletID,
					event.WalletPubKeyHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wmfs.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wmfs *BWalletMovingFundsSubscription) Pipe(
	sink chan *abi.BridgeWalletMovingFunds,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wmfs.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wmfs.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wmfs.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past WalletMovingFunds events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wmfs.contract.PastWalletMovingFundsEvents(
					fromBlock,
					nil,
					wmfs.ecdsaWalletIDFilter,
					wmfs.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past WalletMovingFunds events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wmfs.contract.watchWalletMovingFunds(
		sink,
		wmfs.ecdsaWalletIDFilter,
		wmfs.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchWalletMovingFunds(
	sink chan *abi.BridgeWalletMovingFunds,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchWalletMovingFunds(
			&bind.WatchOpts{Context: ctx},
			sink,
			ecdsaWalletIDFilter,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event WalletMovingFunds had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event WalletMovingFunds failed "+
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

func (b *Bridge) PastWalletMovingFundsEvents(
	startBlock uint64,
	endBlock *uint64,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeWalletMovingFunds, error) {
	iterator, err := b.contract.FilterWalletMovingFunds(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletMovingFunds events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeWalletMovingFunds, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) WalletParametersUpdatedEvent(
	opts *ethereum.SubscribeOpts,
) *BWalletParametersUpdatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BWalletParametersUpdatedSubscription{
		b,
		opts,
	}
}

type BWalletParametersUpdatedSubscription struct {
	contract *Bridge
	opts     *ethereum.SubscribeOpts
}

type bridgeWalletParametersUpdatedFunc func(
	WalletCreationPeriod uint32,
	WalletCreationMinBtcBalance uint64,
	WalletCreationMaxBtcBalance uint64,
	WalletClosureMinBtcBalance uint64,
	WalletMaxAge uint32,
	WalletMaxBtcTransfer uint64,
	WalletClosingPeriod uint32,
	blockNumber uint64,
)

func (wpus *BWalletParametersUpdatedSubscription) OnEvent(
	handler bridgeWalletParametersUpdatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeWalletParametersUpdated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.WalletCreationPeriod,
					event.WalletCreationMinBtcBalance,
					event.WalletCreationMaxBtcBalance,
					event.WalletClosureMinBtcBalance,
					event.WalletMaxAge,
					event.WalletMaxBtcTransfer,
					event.WalletClosingPeriod,
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

func (wpus *BWalletParametersUpdatedSubscription) Pipe(
	sink chan *abi.BridgeWalletParametersUpdated,
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
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wpus.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past WalletParametersUpdated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wpus.contract.PastWalletParametersUpdatedEvents(
					fromBlock,
					nil,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past WalletParametersUpdated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wpus.contract.watchWalletParametersUpdated(
		sink,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchWalletParametersUpdated(
	sink chan *abi.BridgeWalletParametersUpdated,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchWalletParametersUpdated(
			&bind.WatchOpts{Context: ctx},
			sink,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event WalletParametersUpdated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event WalletParametersUpdated failed "+
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

func (b *Bridge) PastWalletParametersUpdatedEvents(
	startBlock uint64,
	endBlock *uint64,
) ([]*abi.BridgeWalletParametersUpdated, error) {
	iterator, err := b.contract.FilterWalletParametersUpdated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletParametersUpdated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeWalletParametersUpdated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

func (b *Bridge) WalletTerminatedEvent(
	opts *ethereum.SubscribeOpts,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) *BWalletTerminatedSubscription {
	if opts == nil {
		opts = new(ethereum.SubscribeOpts)
	}
	if opts.Tick == 0 {
		opts.Tick = chainutil.DefaultSubscribeOptsTick
	}
	if opts.PastBlocks == 0 {
		opts.PastBlocks = chainutil.DefaultSubscribeOptsPastBlocks
	}

	return &BWalletTerminatedSubscription{
		b,
		opts,
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	}
}

type BWalletTerminatedSubscription struct {
	contract               *Bridge
	opts                   *ethereum.SubscribeOpts
	ecdsaWalletIDFilter    [][32]byte
	walletPubKeyHashFilter [][20]byte
}

type bridgeWalletTerminatedFunc func(
	EcdsaWalletID [32]byte,
	WalletPubKeyHash [20]byte,
	blockNumber uint64,
)

func (wts *BWalletTerminatedSubscription) OnEvent(
	handler bridgeWalletTerminatedFunc,
) subscription.EventSubscription {
	eventChan := make(chan *abi.BridgeWalletTerminated)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventChan:
				handler(
					event.EcdsaWalletID,
					event.WalletPubKeyHash,
					event.Raw.BlockNumber,
				)
			}
		}
	}()

	sub := wts.Pipe(eventChan)
	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (wts *BWalletTerminatedSubscription) Pipe(
	sink chan *abi.BridgeWalletTerminated,
) subscription.EventSubscription {
	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(wts.opts.Tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lastBlock, err := wts.contract.blockCounter.CurrentBlock()
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
				}
				fromBlock := lastBlock - wts.opts.PastBlocks

				bLogger.Infof(
					"subscription monitoring fetching past WalletTerminated events "+
						"starting from block [%v]",
					fromBlock,
				)
				events, err := wts.contract.PastWalletTerminatedEvents(
					fromBlock,
					nil,
					wts.ecdsaWalletIDFilter,
					wts.walletPubKeyHashFilter,
				)
				if err != nil {
					bLogger.Errorf(
						"subscription failed to pull events: [%v]",
						err,
					)
					continue
				}
				bLogger.Infof(
					"subscription monitoring fetched [%v] past WalletTerminated events",
					len(events),
				)

				for _, event := range events {
					sink <- event
				}
			}
		}
	}()

	sub := wts.contract.watchWalletTerminated(
		sink,
		wts.ecdsaWalletIDFilter,
		wts.walletPubKeyHashFilter,
	)

	return subscription.NewEventSubscription(func() {
		sub.Unsubscribe()
		cancelCtx()
	})
}

func (b *Bridge) watchWalletTerminated(
	sink chan *abi.BridgeWalletTerminated,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) event.Subscription {
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		return b.contract.WatchWalletTerminated(
			&bind.WatchOpts{Context: ctx},
			sink,
			ecdsaWalletIDFilter,
			walletPubKeyHashFilter,
		)
	}

	thresholdViolatedFn := func(elapsed time.Duration) {
		bLogger.Warnf(
			"subscription to event WalletTerminated had to be "+
				"retried [%s] since the last attempt; please inspect "+
				"host chain connectivity",
			elapsed,
		)
	}

	subscriptionFailedFn := func(err error) {
		bLogger.Errorf(
			"subscription to event WalletTerminated failed "+
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

func (b *Bridge) PastWalletTerminatedEvents(
	startBlock uint64,
	endBlock *uint64,
	ecdsaWalletIDFilter [][32]byte,
	walletPubKeyHashFilter [][20]byte,
) ([]*abi.BridgeWalletTerminated, error) {
	iterator, err := b.contract.FilterWalletTerminated(
		&bind.FilterOpts{
			Start: startBlock,
			End:   endBlock,
		},
		ecdsaWalletIDFilter,
		walletPubKeyHashFilter,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error retrieving past WalletTerminated events: [%v]",
			err,
		)
	}

	events := make([]*abi.BridgeWalletTerminated, 0)

	for iterator.Next() {
		event := iterator.Event
		events = append(events, event)
	}

	return events, nil
}

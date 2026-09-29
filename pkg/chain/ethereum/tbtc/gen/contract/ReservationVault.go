// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"fmt"
	"math/big"
	"strings"
	"sync"

	hostchainabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	chainutil "github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/tbtc/gen/abi"
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var rvLogger = log.Logger("keep-contract-ReservationVault")

type ReservationVault struct {
	contract          *abi.ReservationVault
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

func NewReservationVault(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*ReservationVault, error) {
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

	contract, err := abi.NewReservationVault(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.ReservationVaultABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &ReservationVault{
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

// ----- Const Methods ------

func (rv *ReservationVault) InKindFeeDebtSat() (uint64, error) {
	result, err := rv.contract.InKindFeeDebtSat(
		rv.callerOptions,
	)

	if err != nil {
		return result, rv.errorResolver.ResolveError(
			err,
			rv.callerOptions.From,
			nil,
			"inKindFeeDebtSat",
		)
	}

	return result, err
}

func (rv *ReservationVault) InKindFeeDebtSatAtBlock(
	blockNumber *big.Int,
) (uint64, error) {
	var result uint64

	err := chainutil.CallAtBlock(
		rv.callerOptions.From,
		blockNumber,
		nil,
		rv.contractABI,
		rv.caller,
		rv.errorResolver,
		rv.contractAddress,
		"inKindFeeDebtSat",
		&result,
	)

	return result, err
}

func (rv *ReservationVault) TbtcToken() (common.Address, error) {
	result, err := rv.contract.TbtcToken(
		rv.callerOptions,
	)

	if err != nil {
		return result, rv.errorResolver.ResolveError(
			err,
			rv.callerOptions.From,
			nil,
			"tbtcToken",
		)
	}

	return result, err
}

func (rv *ReservationVault) TbtcTokenAtBlock(
	blockNumber *big.Int,
) (common.Address, error) {
	var result common.Address

	err := chainutil.CallAtBlock(
		rv.callerOptions.From,
		blockNumber,
		nil,
		rv.contractABI,
		rv.caller,
		rv.errorResolver,
		rv.contractAddress,
		"tbtcToken",
		&result,
	)

	return result, err
}

// ------ Events -------

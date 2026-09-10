package main

// contractTemplateContent contains the template string from contract.go.tmpl
var contractTemplateContent = `// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"strings"
	"sync"

	hostchainabi "{{.HostChainModule}}/accounts/abi"
	"{{.HostChainModule}}/accounts/abi/bind"
	"{{.HostChainModule}}/accounts/keystore"
	"{{.HostChainModule}}/common"
	"{{.HostChainModule}}/core/types"
	"{{.HostChainModule}}/crypto"
	"{{.HostChainModule}}/event"

	"github.com/ipfs/go-log"

	chainutil "{{.ChainUtilPackage}}"
	"github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/subscription"
{{if .GenPackage}}
	"{{.GenPackage}}/abi"
{{end}}
)

// Create a package-level logger for this contract. The logger exists at
// package level so that the logger is registered at startup and can be
// included or excluded from logging at startup by name.
var {{.ShortVar}}Logger = log.Logger("keep-contract-{{.Class}}")

type {{.Class}} struct {
	contract           *abi.{{.AbiClass}}
	contractAddress    common.Address
	contractABI        *hostchainabi.ABI
	caller             bind.ContractCaller
	transactor         bind.ContractTransactor
	callerOptions      *bind.CallOpts
	transactorOptions  *bind.TransactOpts
	errorResolver      *chainutil.ErrorResolver
	nonceManager       *ethereum.NonceManager
	miningWaiter       *chainutil.MiningWaiter
	blockCounter	   *ethereum.BlockCounter

	transactionMutex *sync.Mutex
}

func New{{.Class}}(
	contractAddress common.Address,
	chainId *big.Int,
	accountKey *keystore.Key,
	backend bind.ContractBackend,
	nonceManager *ethereum.NonceManager,
	miningWaiter *chainutil.MiningWaiter,
	blockCounter *ethereum.BlockCounter,
	transactionMutex *sync.Mutex,
) (*{{.Class}}, error) {
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

	contract, err := abi.New{{.AbiClass}}(
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

	contractABI, err := hostchainabi.JSON(strings.NewReader(abi.{{.AbiClass}}ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate ABI: [%v]", err)
	}

	return &{{.Class}}{
		contract:          contract,
		contractAddress:   contractAddress,
		contractABI: 	   &contractABI,
		caller:     	   backend,
		transactor:        backend,
		callerOptions:     callerOptions,
		transactorOptions: transactorOptions,
		errorResolver:     chainutil.NewErrorResolver(backend, &contractABI, &contractAddress),
		nonceManager:      nonceManager,
		miningWaiter:      miningWaiter,
		blockCounter: 	   blockCounter,
		transactionMutex:  transactionMutex,
	}, nil
}

// ----- Non-const Methods ------
{{template "contract_non_const_methods.go.tmpl" .}}

// ----- Const Methods ------
{{template "contract_const_methods.go.tmpl" .}}

// ------ Events -------
{{template "contract_events.go.tmpl" . -}}
`

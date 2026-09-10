// Package ethutil provides utilities used for dealing with Ethereum concerns in
// the context of implementing cross-chain interfaces defined in pkg/chain.
package ethutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"

	chainEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"

	"github.com/ipfs/go-log"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

var logger = log.Logger("keep-ethutil")

// EthereumClient wraps the core `bind.ContractBackend` interface with
// some other interfaces allowing to expose additional methods provided
// by client implementations.
type EthereumClient interface {
	bind.ContractBackend
	ethereum.ChainReader
	ethereum.TransactionReader

	BalanceAt(
		ctx context.Context,
		account common.Address,
		blockNumber *big.Int,
	) (*big.Int, error)
}

// AddressFromHex converts the passed string to a common.Address and returns it,
// unless it is not a valid address, in which case it returns an error. Compare
// to common.HexToAddress, which assumes the address is valid and does not
// provide for an error return.
func AddressFromHex(hex string) (common.Address, error) {
	if common.IsHexAddress(hex) {
		return common.HexToAddress(hex), nil
	}

	return common.Address{}, fmt.Errorf("[%v] is not a valid Ethereum address", hex)
}

// DecryptKeyFile reads in a key file and uses the password to decrypt it.
func DecryptKeyFile(keyFile, password string) (*keystore.Key, error) {
	// #nosec G304 (file path provided as taint input)
	// This line is used to read a local key file. There is no user input.
	data, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read KeyFile %s [%v]", keyFile, err)
	}
	key, err := keystore.DecryptKey(data, password)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt %s [%v]", keyFile, err)
	}
	return key, nil
}

// ConnectClients takes HTTP and RPC URLs and returns initialized versions of
// standard, WebSocket, and RPC clients for the Ethereum node at that address.
func ConnectClients(url string, urlRPC string) (*ethclient.Client, *rpc.Client, *rpc.Client, error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"error Connecting to Geth Server: %s [%v]",
			url,
			err,
		)
	}

	clientWS, err := rpc.Dial(url)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"error Connecting to Geth Server: %s [%v]",
			url,
			err,
		)
	}

	clientRPC, err := rpc.Dial(urlRPC)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"error Connecting to Geth Server: %s [%v]",
			url,
			err,
		)
	}

	return client, clientWS, clientRPC, nil
}

// CallAtBlock allows the invocation of a particular contract method at a
// particular block. It papers over the fact that abigen bindings don't directly
// support calling at a particular block, and is mostly meant for use from
// generated contract code.
func CallAtBlock(
	fromAddress common.Address,
	blockNumber *big.Int,
	value *big.Int,
	contractABI *abi.ABI,
	caller bind.ContractCaller,
	errorResolver *ErrorResolver,
	contractAddress common.Address,
	method string,
	result interface{},
	parameters ...interface{},
) error {
	input, err := contractABI.Pack(method, parameters...)
	if err != nil {
		return err
	}

	var (
		msg = ethereum.CallMsg{
			From:  fromAddress,
			To:    &contractAddress,
			Data:  input,
			Value: value,
		}
		code   []byte
		output []byte
	)

	output, err = caller.CallContract(context.TODO(), msg, blockNumber)
	if err == nil && len(output) == 0 {
		// Make sure we have a contract to operate on, and bail out otherwise.
		if code, err = caller.CodeAt(context.TODO(), contractAddress, nil); err != nil {
			return err
		} else if len(code) == 0 {
			return bind.ErrNoCode
		}
	}

	err = contractABI.UnpackIntoInterface(result, method, output)
	if err != nil {
		return errorResolver.ResolveError(
			err,
			fromAddress,
			value,
			method,
			parameters...,
		)
	}

	return nil
}

// EstimateGas tries to estimate the gas needed to execute a specific transaction based on
// the current pending state of the backend blockchain. There is no guarantee that this is
// the true gas limit requirement as other transactions may be added or removed by miners,
// but it should provide a basis for setting a reasonable default.
func EstimateGas(
	from common.Address,
	to common.Address,
	method string,
	contractABI *abi.ABI,
	transactor bind.ContractTransactor,
	parameters ...interface{},
) (uint64, error) {
	input, err := contractABI.Pack(method, parameters...)
	if err != nil {
		return 0, err
	}

	msg := ethereum.CallMsg{
		From: from,
		To:   &to,
		Data: input,
	}

	gas, err := transactor.EstimateGas(context.TODO(), msg)
	if err != nil {
		return 0, err
	}

	return gas, nil
}

// NewBlockCounter creates a new BlockCounter instance for the provided
// Ethereum client.
func NewBlockCounter(client EthereumClient) (*chainEthereum.BlockCounter, error) {
	return chainEthereum.CreateBlockCounter(&ethereumAdapter{client})
}

// NewNonceManager creates NonceManager instance for the provided account
// using the provided Ethereum client.
func NewNonceManager(
	client EthereumClient,
	account common.Address,
) *chainEthereum.NonceManager {
	return chainEthereum.NewNonceManager(
		&ethereumAdapter{client},
		chainEthereum.Address(account),
	)
}

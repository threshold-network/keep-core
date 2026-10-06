package ethutil

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// ABI for errors bubbled out from revert calls. Not used directly as errors are
// neither encoded strictly as method calls nor strictly as return values, nor
// strictly as events, but some various bits of it are used for unpacking the
// errors. See ResolveError below.
const errorABIString = "[{\"constant\":true,\"outputs\":[{\"type\":\"string\"}],\"inputs\":[{\"name\":\"message\", \"type\":\"string\"}],\"name\":\"Error\", \"type\": \"function\"}]"

var errorABI abi.ABI

func init() {
	var err error
	errorABI, err = abi.JSON(strings.NewReader(errorABIString))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse error ABI string: [%v] (ABI was: [%v])", err, errorABIString))
	}
}

// ErrorResolver bundles up the bits needed to turn errors like "failed to
// estimate gas needed" that are triggered by contract reverts but don't include
// revert causes into proper revert error messages from a contract by calling
// the contract method without trying to commit it.
//
// It has one method, ResolveError, that does the heavy lifting.
type ErrorResolver struct {
	contractCaller ethereum.ContractCaller
	abi            *abi.ABI
	address        *common.Address
}

// NewErrorResolver returns an ErroResolver for the given Ethereum client,
// contract ABI, and contract address combination.
func NewErrorResolver(
	contractCaller ethereum.ContractCaller,
	abi *abi.ABI,
	address *common.Address,
) *ErrorResolver {
	return &ErrorResolver{contractCaller, abi, address}
}

// ResolveError resolves the given transaction error to a standard error that,
// if available, contains the error message the transaction produced when
// reverting.
//
// ResolveError achieves this by re-calling the transaction (not submitting it
// for block inclusion, just calling it for its results). `value` is the value
// in gwei to send along with the simulated call.
func (er *ErrorResolver) ResolveError(
	originalErr error,
	from common.Address,
	value *big.Int,
	methodName string,
	parameters ...interface{},
) error {
	logger.Debugf(
		"packing parameters for method [%s] with ABI [%v]: [%+v]",
		methodName,
		er.abi.Methods[methodName],
		parameters,
	)

	packed, err := er.abi.Pack(methodName, parameters...)
	msg := ethereum.CallMsg{
		From:  from,
		To:    er.address,
		Data:  packed,
		Value: value,
	}

	logger.Debugf(
		"resolving error for contract call [%+v]",
		msg,
	)

	response, err := er.contractCaller.CallContract(context.TODO(), msg, nil)
	if err != nil {
		return fmt.Errorf("got error [%v] while resolving original error [%v]", err, originalErr)
	}

	// An error is returned as a 4-byte error id (same encoding as a method id)
	// followed by a set of ABI-encoded values as if the error were a method
	// that returned those values.
	//
	// Current spec-ish @ https://github.com/ethereum/EIPs/issues/838#issuecomment-458919375
	// Bless Ethereum's heart.
	if len(response) < 4 {
		return fmt.Errorf(
			"response [%v] was not long enough to interpret while resolving original error [%v]",
			response,
			originalErr,
		)
	}

	errorID, encodedReturns := response[0:4], response[4:]

	errorMethod, err := errorABI.MethodById(errorID)
	if err != nil {
		return fmt.Errorf("got [%v] while resolving original error [%v] on return [%v]", err, originalErr, response)
	}

	errorValues, err := errorMethod.Outputs.UnpackValues(encodedReturns)
	if err != nil {
		return fmt.Errorf("got [%v] while resolving original error [%v] on return [%v]", err, originalErr, response)
	}

	return fmt.Errorf(
		"contract failed with: [%v] (original error [%v])",
		errorValues,
		originalErr,
	)
}

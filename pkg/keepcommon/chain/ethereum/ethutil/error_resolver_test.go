package ethutil_test

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

var testABIMethods = map[string]abi.Method{
	"Test": abi.Method{
		Constant: false,
		Name:     "Test",
		Inputs:   []abi.Argument{},
		Outputs:  []abi.Argument{},
	},
}
var testABI = abi.ABI{
	Constructor: testABIMethods["Test"],
	Events:      map[string]abi.Event{},
	Methods:     testABIMethods,
}
var testAddress = common.Address([20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

var errOriginal = fmt.Errorf("OG")
var errCall = fmt.Errorf("call error")

func assertErrorContains(t *testing.T, err error, substrings ...string) {
	t.Helper()

	if err == nil {
		t.Errorf("\nexpected: {error}\nactual:   nil")
		return
	}

	for _, substring := range substrings {
		if !strings.Contains(err.Error(), substring) {
			t.Errorf(
				"\nexpected: {error containing [%#v]}\nactual:   %v",
				substring,
				err,
			)
		}
	}
}

// Helper caller type that always returns errCall.
type erroringCaller struct{}

func (*erroringCaller) CallContract(_ context.Context, _ ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	return nil, errCall
}

// Helper caller type that always returns a nil error and the provided
// returnedBytes.
type fixedReturnCaller struct {
	returnedBytes []byte
}

func (frc *fixedReturnCaller) CallContract(_ context.Context, _ ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	return frc.returnedBytes, nil
}

// Helper caller type that calls a referenced callback function for each
// CallContract call.
type callbackCaller struct {
	callbackCalled bool
	callback       contractCallFn
}

type contractCallFn func(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)

func callbackCallerWith(fn contractCallFn) *callbackCaller {
	return &callbackCaller{false, fn}
}

func (cc *callbackCaller) CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	cc.callbackCalled = true
	return cc.callback(ctx, call, blockNumber)
}

func TestErrorResolverHandlesErrorCall(t *testing.T) {
	caller := &erroringCaller{}
	resolver := ethutil.NewErrorResolver(caller, &testABI, &testAddress)

	err := resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
	if err == nil {
		t.Errorf("\nexpected: {error}\nactual:   nil")
		return
	}

	assertErrorContains(
		t,
		err,
		errOriginal.Error(),
		errCall.Error(),
	)
}

func TestErrorResolverHandlesShortResponses(t *testing.T) {
	caller := &fixedReturnCaller{}
	resolver := ethutil.NewErrorResolver(caller, &testABI, &testAddress)

	for returnLength := 0; returnLength < 4; returnLength++ {
		caller.returnedBytes = make([]byte, returnLength)
		err := resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
		assertErrorContains(
			t,
			err,
			errOriginal.Error(),
			fmt.Sprintf("%v", caller.returnedBytes),
			"was not long enough",
		)
	}
}

func TestErrorResolverHandlesUnknownMethodResponses(t *testing.T) {
	caller := &fixedReturnCaller{[]byte{0, 0, 0, 1}}
	resolver := ethutil.NewErrorResolver(caller, &testABI, &testAddress)

	err := resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
	assertErrorContains(
		t,
		err,
		errOriginal.Error(),
		fmt.Sprintf("%v", caller.returnedBytes),
		"no method with id",
	)
}

func TestErrorResolverHandlesBadParameterResponses(t *testing.T) {
	// method ABI signature for the basic error construction we get back in case
	// of a require failing.
	caller := &fixedReturnCaller{[]byte{8, 195, 121, 160}}
	resolver := ethutil.NewErrorResolver(caller, &testABI, &testAddress)

	// bad response length
	err := resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
	assertErrorContains(
		t,
		err,
		errOriginal.Error(),
		fmt.Sprintf("%v", caller.returnedBytes),
		"length insufficient",
	)

	// good response length, bad data offset in response
	buildingBlock := [32]byte{}
	caller.returnedBytes = append(caller.returnedBytes, buildingBlock[:]...)
	caller.returnedBytes[len(caller.returnedBytes)-1] = 1
	err = resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
	assertErrorContains(
		t,
		err,
		errOriginal.Error(),
		fmt.Sprintf("%v", caller.returnedBytes),
		"would go over slice boundary",
	)

	// good response length, good data offset, bad string length in response
	caller.returnedBytes = []byte{8, 195, 121, 160}
	caller.returnedBytes = append(caller.returnedBytes, buildingBlock[:]...)
	caller.returnedBytes[len(caller.returnedBytes)-1] = 32
	caller.returnedBytes = append(caller.returnedBytes, buildingBlock[:]...)
	caller.returnedBytes[len(caller.returnedBytes)-1] = 1
	err = resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
	assertErrorContains(
		t,
		err,
		errOriginal.Error(),
		fmt.Sprintf("%v", caller.returnedBytes),
		"length insufficient",
	)
}

func TestErrorResolverHandlesGoodErrorResponse(t *testing.T) {
	caller := &fixedReturnCaller{[]byte{8, 195, 121, 160}}

	// Build a blank error message.
	buildingBlock := [32]byte{}
	caller.returnedBytes = append(caller.returnedBytes, buildingBlock[:]...)
	caller.returnedBytes[len(caller.returnedBytes)-1] = 32 // data offset, fixed
	caller.returnedBytes = append(caller.returnedBytes, buildingBlock[:]...)
	caller.returnedBytes[len(caller.returnedBytes)-1] = 0

	resolver := ethutil.NewErrorResolver(caller, &testABI, &testAddress)
	err := resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
	assertErrorContains(
		t,
		err,
		errOriginal.Error(),
		"[]",
	)

	// Build an error message.
	errorMessage := "Something's gone awry."
	caller.returnedBytes = []byte{8, 195, 121, 160}
	caller.returnedBytes = append(caller.returnedBytes, buildingBlock[:]...)
	caller.returnedBytes[len(caller.returnedBytes)-1] = 32 // data offset, fixed
	caller.returnedBytes = append(caller.returnedBytes, buildingBlock[:]...)
	caller.returnedBytes[len(caller.returnedBytes)-1] = byte(len(errorMessage))
	caller.returnedBytes = append(caller.returnedBytes, errorMessage[:]...)

	err = resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")
	assertErrorContains(
		t,
		err,
		errOriginal.Error(),
		errorMessage,
	)
}

func TestErrorResolverPropagateFromAddress(t *testing.T) {
	fromAddress := common.HexToAddress("0xA86c468475EF9C2ce851Ea4125424672C3F7e0C8")

	assertingCaller := callbackCallerWith(func(
		ctx context.Context,
		msg ethereum.CallMsg,
		blockNumber *big.Int,
	) ([]byte, error) {
		if fromAddress.Hex() != msg.From.Hex() {
			t.Errorf(
				"Unexpected From address\nExpected: [%v]\nActual:   [%v]\n",
				fromAddress.Hex(),
				msg.From.Hex(),
			)
		}

		return nil, fmt.Errorf("I don't care")
	})

	resolver := ethutil.NewErrorResolver(assertingCaller, &testABI, &testAddress)
	resolver.ResolveError(errOriginal, fromAddress, nil, "Test")

	if !assertingCaller.callbackCalled {
		t.Error("CallContract not invoked")
	}
}

func TestErrorResolverPropagateToAddress(t *testing.T) {
	toAddress := common.HexToAddress("0x524f2E0176350d950fA630D9A5a59A0a190DAf48")

	assertingCaller := callbackCallerWith(func(
		ctx context.Context,
		msg ethereum.CallMsg,
		blockNumber *big.Int,
	) ([]byte, error) {
		if toAddress.Hex() != msg.To.Hex() {
			t.Errorf(
				"Unexpected To address\nExpected: [%v]\nActual:   [%v]\n",
				toAddress.Hex(),
				msg.To.Hex(),
			)
		}

		return (&erroringCaller{}).CallContract(ctx, msg, blockNumber)
	})

	resolver := ethutil.NewErrorResolver(assertingCaller, &testABI, &toAddress)
	resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")

	if !assertingCaller.callbackCalled {
		t.Error("CallContract not invoked")
	}
}

func TestErrorResolverPropagateValue(t *testing.T) {
	value := big.NewInt(123111)

	assertingCaller := callbackCallerWith(func(
		ctx context.Context,
		msg ethereum.CallMsg,
		blockNumber *big.Int,
	) ([]byte, error) {
		if value.Cmp(msg.Value) != 0 {
			t.Errorf(
				"Unexpected Value\nExpected: [%v]\nActual:   [%v]\n",
				value,
				msg.Value,
			)
		}

		return (&erroringCaller{}).CallContract(ctx, msg, blockNumber)
	})

	resolver := ethutil.NewErrorResolver(assertingCaller, &testABI, &testAddress)
	resolver.ResolveError(errOriginal, common.Address{}, value, "Test")

	if !assertingCaller.callbackCalled {
		t.Error("CallContract not invoked")
	}
}

func TestErrorResolverPropagateData(t *testing.T) {
	methodName := "Test"
	parameters := []interface{}{}

	assertingCaller := callbackCallerWith(func(
		ctx context.Context,
		msg ethereum.CallMsg,
		blockNumber *big.Int,
	) ([]byte, error) {
		expectedData, err := (&testABI).Pack(methodName, parameters...)
		if err != nil {
			panic(err)
		}

		if !reflect.DeepEqual(expectedData, msg.Data) {
			t.Errorf("Unexpected packed transaction Data")
		}

		return (&erroringCaller{}).CallContract(ctx, msg, blockNumber)
	})

	resolver := ethutil.NewErrorResolver(assertingCaller, &testABI, &testAddress)
	resolver.ResolveError(errOriginal, common.Address{}, nil, "Test")

	if !assertingCaller.callbackCalled {
		t.Error("CallContract not invoked")
	}
}

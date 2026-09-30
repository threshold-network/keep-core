package ethereum

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestContractAddress(t *testing.T) {
	contractName1 := "KeepECDSAContract"
	validContractAddressString := "0xbb2Ea17985f13D43e3AEC3963506A1B25ADDd57F"
	validContractAddress := common.HexToAddress(validContractAddressString)

	contractName2 := "InvalidContract"
	invalidHex := "0xZZZ"

	config := &Config{
		ContractAddresses: map[string]string{
			strings.ToLower(contractName1): validContractAddressString,
			strings.ToLower(contractName2): invalidHex,
		},
	}

	var tests = map[string]struct {
		contractName    string
		expectedAddress common.Address
		expectedError   error
	}{
		"contract name matching valid configuration": {
			contractName:    contractName1,
			expectedAddress: validContractAddress,
		},
		"invalid contract hex address": {
			contractName:    contractName2,
			expectedAddress: common.Address{},
			expectedError: fmt.Errorf(
				"configured address [%v] for contract [%v] "+
					"is not valid hex address",
				invalidHex,
				contractName2,
			),
		},
		"missing contract configuration": {
			contractName:    "Peekaboo",
			expectedAddress: common.Address{},
			expectedError:   ErrAddressNotConfigured,
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			actualAddress, err := config.ContractAddress(test.contractName)
			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v\n",
					test.expectedError,
					err,
				)
			}

			if !reflect.DeepEqual(test.expectedAddress, actualAddress) {
				t.Errorf(
					"unexpected address\nexpected: %v\nactual:   %v\n",
					test.expectedAddress,
					actualAddress,
				)
			}
		})
	}
}

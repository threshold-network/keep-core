package ethutil_test

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
)

func TestKeyFileDecryption(t *testing.T) {
	goodKeyFile := "./testdata/UTC--2018-02-15T19-57-35.216297214Z--6ffba2d0f4c8fd7961f516af43c55fe2d56f6044"
	badKeyFile := "./testdata/nonexistent-file.booyan"

	tests := map[string]struct {
		keyFile      string
		password     string
		errorMessage string
	}{
		"good password": {
			keyFile:      goodKeyFile,
			password:     "password",
			errorMessage: "",
		},
		"bad file": {
			keyFile:  badKeyFile,
			password: "",
			errorMessage: fmt.Sprintf(
				"unable to read KeyFile %v [open %v: no such file or directory]",
				badKeyFile,
				badKeyFile,
			),
		},
		"bad password": {
			keyFile:  goodKeyFile,
			password: "nanananana",
			errorMessage: fmt.Sprintf(
				"unable to decrypt %v [could not decrypt key with given password]",
				goodKeyFile,
			),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, err := ethutil.DecryptKeyFile(test.keyFile, test.password)
			message := ""
			if err != nil {
				message = err.Error()
			}

			if message != test.errorMessage {
				t.Errorf(
					"\nexpected: [%v]\nactual:   [%v]",
					test.errorMessage,
					err,
				)
			}
		})
	}
}

func TestAddressFromHex(t *testing.T) {
	tests := map[string]struct {
		hex          string
		errorMessage string
		address      common.Address
	}{
		"valid address": {
			hex:          "0x0000000000000000000000000000000000001234",
			errorMessage: "",
			address:      common.Address([20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 18, 52}),
		},
		"short address": {
			hex:          "0x1234",
			errorMessage: "[0x1234] is not a valid Ethereum address",
			address:      common.Address{},
		},
		"long address": {
			hex:          "0x000000000000000000000000000000000000001234",
			errorMessage: "[0x000000000000000000000000000000000000001234] is not a valid Ethereum address",
			address:      common.Address{},
		},
		"decimal number": {
			hex:          "000000000000000000000000000000000000001234",
			errorMessage: "[000000000000000000000000000000000000001234] is not a valid Ethereum address",
			address:      common.Address{},
		},
		"blank string": {
			hex:          "",
			errorMessage: "[] is not a valid Ethereum address",
			address:      common.Address{},
		},
		"arbitrary string": {
			hex:          "I am a booyan with booyans",
			errorMessage: "[I am a booyan with booyans] is not a valid Ethereum address",
			address:      common.Address{},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			address, err := ethutil.AddressFromHex(test.hex)

			message := ""
			if err != nil {
				message = err.Error()
			}

			if message != test.errorMessage {
				t.Errorf(
					"\nexpected: [%v]\nactual:   [%v]",
					test.errorMessage,
					err,
				)
			}

			if address != test.address {
				t.Errorf(
					"\nexpected: [%v]\nactual:   [%v]",
					test.address,
					address,
				)
			}
		})
	}
}

package decode

import (
	"fmt"
	"testing"
)

func TestParseBytes20(t *testing.T) {
	hexEncoded := "0x3805eed0bb0792eff8815addedb36add2c7257e5"
	bytes, err := ParseBytes20(hexEncoded)
	if err != nil {
		t.Fatal(err)
	}

	roundtrip := fmt.Sprintf("0x%x", bytes)

	if roundtrip != hexEncoded {
		t.Errorf(
			"unexpected parsed bytes\nexpected: %v\nactual:   %v\n",
			hexEncoded,
			roundtrip,
		)
	}
}

func TestParseBytes20_Fail(t *testing.T) {

	var tests = map[string]struct {
		input         string
		expectedError string
	}{
		"too short": {
			input:         "0xFFFF",
			expectedError: "expected 20 bytes array; has: [2]",
		},
		"empty": {
			input:         "",
			expectedError: "empty hex string",
		},
		"just 0x prefix": {
			input:         "0x",
			expectedError: "expected 20 bytes array; has: [0]",
		},
		"no prefix": {
			input:         "FF",
			expectedError: "hex string without 0x prefix",
		},
		"invalid hex": {
			input:         "0xLMA0",
			expectedError: "invalid hex string",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, actualError := ParseBytes20(test.input)
			if actualError.Error() != test.expectedError {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v\n",
					test.expectedError,
					actualError,
				)
			}
		})
	}
}

func TestParseBytes32(t *testing.T) {
	hexEncoded := "0xad63a8286ea7fa22d75e167216171417a96c4753946fd45e3a8dff4e4f29a830"
	bytes, err := ParseBytes32(hexEncoded)
	if err != nil {
		t.Fatal(err)
	}

	roundtrip := fmt.Sprintf("0x%x", bytes)

	if roundtrip != hexEncoded {
		t.Errorf(
			"unexpected parsed bytes\nexpected: %v\nactual:   %v\n",
			hexEncoded,
			roundtrip,
		)
	}
}

func TestParseBytes32_Fail(t *testing.T) {

	var tests = map[string]struct {
		input         string
		expectedError string
	}{
		"too short": {
			input:         "0xFFFF",
			expectedError: "expected 32 bytes array; has: [2]",
		},
		"empty": {
			input:         "",
			expectedError: "empty hex string",
		},
		"just 0x prefix": {
			input:         "0x",
			expectedError: "expected 32 bytes array; has: [0]",
		},
		"no prefix": {
			input:         "FF",
			expectedError: "hex string without 0x prefix",
		},
		"invalid hex": {
			input:         "0xLMA0",
			expectedError: "invalid hex string",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, actualError := ParseBytes32(test.input)
			if actualError.Error() != test.expectedError {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v\n",
					test.expectedError,
					actualError,
				)
			}
		})
	}
}

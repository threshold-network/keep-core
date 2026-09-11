package flag

import (
	"errors"
	"math/big"
	"testing"

	pflag "github.com/spf13/pflag"
)

const bigIntFlagName = "amount"

func TestBigIntVarFlag_Set(t *testing.T) {
	defaultValue := big.NewInt(11)

	tests := map[string]struct {
		value         string
		expectedError error
		expectedValue *big.Int
	}{
		"valid value": {
			value:         "8569412",
			expectedValue: big.NewInt(8569412),
		},
		"invalid value": {
			value: "100k",
			expectedError: errors.New(
				"invalid argument \"100k\" for \"--" + bigIntFlagName +
					"\" flag: failed to parse as big.Int: 100k",
			),
			expectedValue: defaultValue,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			flags := pflag.NewFlagSet("flag-set-"+testName, pflag.PanicOnError)

			var valueDest BigIntFlagValue

			BigIntVarFlag(flags, &valueDest, bigIntFlagName, defaultValue, "")

			err := flags.Set(bigIntFlagName, test.value)

			if errorMessage(err) != errorMessage(test.expectedError) {
				t.Errorf(
					"unexpected error\nexpected: %v\nactual:   %v\n",
					test.expectedError,
					err,
				)
			}

			if valueDest.Cmp(test.expectedValue) != 0 {
				t.Errorf(
					"\nexpected: %s\nactual:   %s",
					test.expectedValue,
					valueDest,
				)
			}
		})
	}
}

func TestBigIntVarFlag_DefaultValue(t *testing.T) {
	defaultValue := big.NewInt(2675)

	flags := pflag.NewFlagSet("flag-set", pflag.PanicOnError)

	var valueDest BigIntFlagValue

	BigIntVarFlag(flags, &valueDest, bigIntFlagName, defaultValue, "")

	if valueDest.Cmp(defaultValue) != 0 {
		t.Errorf(
			"\nexpected: %s\nactual:   %s",
			defaultValue,
			valueDest,
		)
	}

}

func TestBigIntVarFlag_DefaultValueNil(t *testing.T) {
	var defaultValue *big.Int = nil

	flags := pflag.NewFlagSet("flag-set", pflag.PanicOnError)

	var valueDest BigIntFlagValue

	BigIntVarFlag(flags, &valueDest, bigIntFlagName, defaultValue, "")

	if &valueDest == nil {
		t.Errorf(
			"invalid valueDest\nexpected: %+v\nactual:   %+v",
			&BigIntFlagValue{},
			valueDest,
		)
	}

	if valueDest.Int != nil {
		t.Errorf(
			"invalid valueDest.Int\nexpected: %+v\nactual:   %+v",
			nil,
			valueDest.Int,
		)
	}

}

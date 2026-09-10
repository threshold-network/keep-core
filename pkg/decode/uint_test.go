package decode

import (
	"fmt"
	"testing"
)

func TestParseInt(t *testing.T) {
	assertParseInt(t, "min int8", "-128", 8, int8(-128))
	assertParseInt(t, "max int8", "127", 8, int8(127))

	assertParseInt(t, "min int16", "-32768", 16, int16(-32768))
	assertParseInt(t, "max int16", "32767", 16, int16(32767))

	assertParseInt(t, "min int32", "-2147483648", 32, int32(-2147483648))
	assertParseInt(t, "max int32", "2147483647", 32, int32(2147483647))

	assertParseInt(t, "min int64", "-9223372036854775808", 64, int64(-9223372036854775808))
	assertParseInt(t, "max int64", "9223372036854775807", 64, int64(9223372036854775807))
}

func TestParseInt_Fail(t *testing.T) {
	assertParseIntFail[int8](t, "int8 out of range", "129", 8, fmt.Errorf(`strconv.ParseInt: parsing "129": value out of range`))
}

func TestParseUint(t *testing.T) {
	assertParseUint(t, "min uint8", "0", 8, uint8(0))
	assertParseUint(t, "max uint8", "255", 8, uint8(255))

	assertParseUint(t, "min uint16", "0", 16, uint16(0))
	assertParseUint(t, "max uint16", "65535", 16, uint16(65535))

	assertParseUint(t, "min uint32", "0", 32, uint32(0))
	assertParseUint(t, "max uint32", "4294967295", 32, uint32(4294967295))

	assertParseUint(t, "min uint64", "0", 64, uint64(0))
	assertParseUint(t, "max uint64", "18446744073709551615", 64, uint64(18446744073709551615))
}

func TestParseUint_Fail(t *testing.T) {
	assertParseUintFail[uint8](t, "int8 out of range", "256", 8, fmt.Errorf(`strconv.ParseUint: parsing "256": value out of range`))
}

func assertParseInt[K number](t *testing.T, testName string, str string, bitSize int, expectedResult K) {
	t.Run(testName, func(t *testing.T) {
		actualResult, err := ParseInt[K](str, bitSize)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if actualResult != expectedResult {
			t.Errorf(
				"unexpected result\nexpected: %d\nactual:   %d",
				expectedResult,
				actualResult,
			)
		}
	})
}

func assertParseIntFail[K number](t *testing.T, testName string, str string, bitSize int, expectedError error) {
	t.Run(testName, func(t *testing.T) {
		_, actualError := ParseInt[K](str, bitSize)
		if actualError.Error() != expectedError.Error() {
			t.Errorf(
				"unexpected error\nexpected: %s\nactual:   %s",
				expectedError,
				actualError,
			)
		}

	})
}

func assertParseUint[K unumber](t *testing.T, testName string, str string, bitSize int, expectedResult K) {
	t.Run(testName, func(t *testing.T) {
		actualResult, err := ParseUint[K](str, bitSize)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if actualResult != expectedResult {
			t.Errorf(
				"unexpected result\nexpected: %d\nactual:   %d",
				expectedResult,
				actualResult,
			)
		}
	})
}

func assertParseUintFail[K unumber](t *testing.T, testName string, str string, bitSize int, expectedError error) {
	t.Run(testName, func(t *testing.T) {
		_, actualError := ParseUint[K](str, bitSize)
		if actualError.Error() != expectedError.Error() {
			t.Errorf(
				"unexpected error\nexpected: %s\nactual:   %s",
				expectedError,
				actualError,
			)
		}

	})
}

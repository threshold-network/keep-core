package decode

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// ParseBytes20 parses `string` into `[20]byte` type. The input string must have
// 20 bytes hex-encoded.
func ParseBytes20(str string) ([20]byte, error) {
	bytesArray := [20]byte{}
	slice, err := hexutil.Decode(str)
	if err != nil {
		return bytesArray, err
	}
	if len(slice) != 20 {
		return bytesArray, fmt.Errorf("expected 20 bytes array; has: [%v]", len(slice))
	}

	copy(bytesArray[:], slice)
	return bytesArray, nil
}

// ParseBytes32 parses `string` into `[32]byte` type. The input string must have
// 32 bytes hex-encoded.
func ParseBytes32(str string) ([32]byte, error) {
	bytesArray := [32]byte{}
	slice, err := hexutil.Decode(str)
	if err != nil {
		return bytesArray, err
	}
	if len(slice) != 32 {
		return bytesArray, fmt.Errorf("expected 32 bytes array; has: [%v]", len(slice))
	}

	copy(bytesArray[:], slice)
	return bytesArray, nil
}

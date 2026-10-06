package decode

import (
	"strconv"
)

type number interface {
	int8 | int16 | int32 | int64
}

type unumber interface {
	uint8 | uint16 | uint32 | uint64
}

// ParseInt parses string to int of the given bit size.
func ParseInt[K number](str string, bitSize int) (K, error) {
	val64, err := strconv.ParseInt(str, 10, bitSize)
	if err != nil {
		return K(0), err
	}

	return K(val64), nil
}

// ParseUint parses string to uint of the given bit size.
func ParseUint[K unumber](str string, bitSize int) (K, error) {
	val64, err := strconv.ParseUint(str, 10, bitSize)
	if err != nil {
		return K(0), err
	}

	return K(val64), nil
}

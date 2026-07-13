// Package bitcoin defines types and interfaces required to work with the
// Bitcoin chain. This package is meant to reflect the part of the Bitcoin
// protocol to an extent required by the client applications. That is,
// this package can only hold the components specific to the Bitcoin domain
// and must remain free of application-specific items. Third-party helper
// libraries are allowed for use within this package though no external
// type should leak outside.
package bitcoin

import (
	"bytes"
	"github.com/btcsuite/btcd/wire"
)

// CompactSizeUint is a documentation type that is supposed to capture the
// details of the Bitcoin's CompactSize Unsigned Integer. It represents a
// number value encoded to bytes according to the following rules:
//
// ---------------- Value ---------------- | Bytes | --------------- Format --------------
//
// ---------------------------------------------------------------------------------------
// >= 0 && <= 252                          |   1   | uint8
// >= 253 && <= 0xffff                     |   3   | 0xfd followed by the number as uint16
// >= 0x10000 && <= 0xffffffff             |   5   | 0xfe followed by the number as uint32
// >= 0x100000000 && <= 0xffffffffffffffff |   9   | 0xff followed by the number as uint64
//
// Worth noting, the encoded number value is represented using the little-endian
// byte order. For example, to convert the compact size uint 0xfd0302, the
// 0xfd prefix must be skipped and the 0x0302 must be reversed to 0x0203 and
// then converted to a decimal number 515.
//
// For reference, see:
// https://developer.bitcoin.org/reference/transactions.html#compactsize-unsigned-integers
type CompactSizeUint uint64

// readCompactSizeUint reads the leading CompactSizeUint from the provided
// variable length data. Returns the value held by the CompactSizeUint as
// the first argument and the byte length of the CompactSizeUint as the
// second one.
func readCompactSizeUint(varLenData []byte) (CompactSizeUint, int, error) {
	csu, err := wire.ReadVarInt(bytes.NewReader(varLenData), 0)
	if err != nil {
		return 0, 0, err
	}

	return CompactSizeUint(csu), wire.VarIntSerializeSize(csu), nil
}

// writeCompactSizeUint writes the provided CompactSizeUint into a
// byte slice.
func writeCompactSizeUint(csu CompactSizeUint) ([]byte, error) {
	var buffer bytes.Buffer
	err := wire.WriteVarInt(&buffer, 0, uint64(csu))
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// ByteOrder represents the byte order used by the Bitcoin byte arrays. The
// Bitcoin ecosystem is not totally consistent in this regard and different
// byte orders are used depending on the purpose.
type ByteOrder int

const (
	// InternalByteOrder represents the internal byte order used by the Bitcoin
	// protocol. This is the primary byte order that is suitable for the
	// use cases related with the protocol logic and cryptography. Byte arrays
	// using this byte order should be converted to numbers according to
	// the little-endian sequence.
	InternalByteOrder ByteOrder = iota

	// ReversedByteOrder represents the "human" byte order. This is the
	// byte order that is typically used by the third party services like
	// block explorers or Bitcoin chain clients. Byte arrays using this byte
	// order should be converted to numbers according to the big-endian
	// sequence. This type is also known as the `RPC Byte Order` in the
	// Bitcoin specification.
	ReversedByteOrder
)

// Network is a type used for Bitcoin networks enumeration.
type Network int

// Bitcoin networks enumeration.
const (
	Unknown Network = iota
	Mainnet
	Testnet
	Testnet4
	Regtest
)

func (n Network) String() string {
	return []string{"unknown", "mainnet", "testnet", "testnet4", "regtest"}[n]
}

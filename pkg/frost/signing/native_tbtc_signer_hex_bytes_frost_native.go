//go:build frost_native

package signing

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// hexBytes carries a hex-encoded JSON string field as raw bytes, without ever
// materialising the decoded value as a Go string.
//
// This exists for secret material crossing the FFI boundary. The signer contract
// transports the DKG round-1 and round-2 secret packages and the long-term FROST
// key package as hex in JSON string fields. A Go string is immutable, so a
// secret that passes through one cannot be overwritten afterwards: it stays in
// heap memory until an unpredictable GC and is reachable through a core dump,
// swap file, or process read. The rest of this bridge is careful to scrub what it
// can (the marshalled request/response buffers via zeroBytes, and the C-heap copy
// via callBuildTaggedTBTCSignerOperation), and this type closes the remaining
// hole so that discipline covers the whole path.
//
// Both directions avoid the string: MarshalJSON hex-encodes straight into the
// output buffer, and UnmarshalJSON decodes from the raw JSON token bytes. Callers
// remain responsible for zeroing the resulting slice once done with it, exactly
// as they already do for every other secret-bearing buffer.
type hexBytes []byte

// MarshalJSON encodes the bytes as a hex JSON string, writing directly into the
// result buffer so no intermediate string is created.
func (h hexBytes) MarshalJSON() ([]byte, error) {
	out := make([]byte, 1+hex.EncodedLen(len(h))+1)
	out[0] = '"'
	hex.Encode(out[1:len(out)-1], h)
	out[len(out)-1] = '"'

	return out, nil
}

// UnmarshalJSON decodes a hex JSON string into the receiver, reading from the
// raw token bytes rather than a decoded string. A JSON null leaves the receiver
// nil; anything that is not a quoted string is an error.
func (h *hexBytes) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*h = nil

		return nil
	}

	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("hex field is not a JSON string")
	}
	encoded := data[1 : len(data)-1]

	decoded := make([]byte, hex.DecodedLen(len(encoded)))
	n, err := hex.Decode(decoded, encoded)
	if err != nil {
		// Do not include the offending value: for these fields it is secret
		// material, and hex.Decode's error already names the offset.
		return fmt.Errorf("hex field is invalid hex: %w", err)
	}
	*h = decoded[:n]

	return nil
}

// Ensure the JSON interfaces are satisfied with the intended receivers: value for
// marshal (so a non-pointer struct field still encodes) and pointer for
// unmarshal.
var (
	_ json.Marshaler   = hexBytes(nil)
	_ json.Unmarshaler = (*hexBytes)(nil)
)

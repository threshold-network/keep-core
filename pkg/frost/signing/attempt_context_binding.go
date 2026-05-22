package signing

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// AttemptContextHashFieldLength is the on-wire byte length of the
// optional AttemptContextHash field carried by the FROST/tbtc-signer
// protocol messages. The field is the canonical SHA-256 hash of the
// AttemptContext (see pkg/frost/roast/attempt), so 32 bytes.
const AttemptContextHashFieldLength = attempt.MessageDigestLength

// validateAttemptContextHashField checks the length invariant for the
// optional AttemptContextHash field on protocol messages. An absent
// field (nil or zero-length slice) is valid; a present field must
// match AttemptContextHashFieldLength exactly.
//
// This is the only validation Phase 1B performs on the field. Higher-
// level acceptance (the receiver-side check that the hash matches the
// locally-computed AttemptContext) lands in a later RFC-21 phase
// behind a build tag, since enabling it requires honest peers to have
// rolled out the new field first.
func validateAttemptContextHashField(field []byte) error {
	if len(field) == 0 {
		return nil
	}
	if len(field) != AttemptContextHashFieldLength {
		return fmt.Errorf(
			"attempt context hash field has wrong length [%d], expected [%d] or absent",
			len(field),
			AttemptContextHashFieldLength,
		)
	}
	return nil
}

// attemptContextHashFieldFromArray converts a fixed-size 32-byte hash
// into the slice form used on the wire. Returns a fresh slice so the
// caller's array cannot be mutated through the returned reference.
func attemptContextHashFieldFromArray(
	hash [AttemptContextHashFieldLength]byte,
) []byte {
	out := make([]byte, AttemptContextHashFieldLength)
	copy(out, hash[:])
	return out
}

// attemptContextHashFieldToArray converts a wire-form slice back to
// a fixed-size 32-byte hash plus a presence flag. Returns
// (zeroArray, false) when the field is absent. Caller has already
// validated length via validateAttemptContextHashField; this function
// trusts that invariant and panics on violation.
func attemptContextHashFieldToArray(
	field []byte,
) ([AttemptContextHashFieldLength]byte, bool) {
	var out [AttemptContextHashFieldLength]byte
	if len(field) == 0 {
		return out, false
	}
	if len(field) != AttemptContextHashFieldLength {
		panic(fmt.Sprintf(
			"attemptContextHashFieldToArray called with wrong-length field [%d]",
			len(field),
		))
	}
	copy(out[:], field)
	return out, true
}

//go:build frost_native

package signing

import (
	"encoding/json"
	"fmt"
)

type buildTaggedTBTCSignerErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// buildTaggedTBTCSignerStructuredError carries the FFI error envelope's
// structured fields so callers can match on Code via `errors.As` rather than
// substring-matching the rendered error string. Older signer builds may
// return errors without a Code field; this type still wraps them via the
// Message field, and consumers should treat an empty Code as a fall-back
// signal to apply legacy substring matching.
type buildTaggedTBTCSignerStructuredError struct {
	Code    string
	Message string
}

func (e *buildTaggedTBTCSignerStructuredError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

// buildTaggedTBTCSignerErrorPayload decodes the FFI error envelope into a
// structured form so callers can match on the `Code` field via `errors.As`
// rather than rely on substring matching against the rendered error string.
// Decode failures and missing-fields edge cases are surfaced via the
// `Message` field with `Code` left empty so consumers know to fall back to
// legacy matching.
func buildTaggedTBTCSignerErrorPayload(payload []byte) *buildTaggedTBTCSignerStructuredError {
	var errorResponse buildTaggedTBTCSignerErrorResponse
	if err := json.Unmarshal(payload, &errorResponse); err != nil {
		return &buildTaggedTBTCSignerStructuredError{
			Message: fmt.Sprintf(
				"cannot decode error payload [%x]: %v",
				payload,
				err,
			),
		}
	}

	if errorResponse.Code == "" && errorResponse.Message == "" {
		return &buildTaggedTBTCSignerStructuredError{
			Message: fmt.Sprintf("empty error payload: [%s]", string(payload)),
		}
	}

	return &buildTaggedTBTCSignerStructuredError{
		Code:    errorResponse.Code,
		Message: errorResponse.Message,
	}
}

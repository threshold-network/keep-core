//go:build frost_native

package signing

import (
	"encoding/json"
	"errors"
	"fmt"
)

// interactiveAggregateShareVerificationFailedCode is the FFI error `code` the
// engine returns when one or more collected shares failed FROST verification
// during interactive aggregation. The accompanying error payload carries the
// candidate culprits.
const interactiveAggregateShareVerificationFailedCode = "aggregate_share_verification_failed"

type buildTaggedTBTCSignerErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// CandidateCulprits is populated only for the
	// aggregate_share_verification_failed error: the u16 Go member identifiers
	// whose shares failed verification (omitted for every other error).
	CandidateCulprits        []uint16                                              `json:"candidate_culprits,omitempty"`
	StateAnchorTrustRecovery *nativeTBTCSignerStateAnchorTrustRecoveryRequiredWire `json:"state_anchor_trust_recovery,omitempty"`
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
	// CandidateCulprits carries the aggregate_share_verification_failed culprit
	// list when present; empty for every other error.
	CandidateCulprits        []uint16
	StateAnchorTrustRecovery *NativeTBTCSignerStateAnchorTrustRecoveryRequired
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

	structured := &buildTaggedTBTCSignerStructuredError{
		Code:              errorResponse.Code,
		Message:           errorResponse.Message,
		CandidateCulprits: errorResponse.CandidateCulprits,
	}
	if errorResponse.StateAnchorTrustRecovery != nil {
		recovery, err :=
			decodeNativeTBTCSignerStateAnchorTrustRecoveryRequired(
				errorResponse.StateAnchorTrustRecovery,
			)
		if err != nil {
			structured.Message = fmt.Sprintf(
				"%s (invalid state-anchor trust-recovery context: %v)",
				structured.Message,
				err,
			)
			return structured
		}
		structured.StateAnchorTrustRecovery = recovery
	}
	return structured
}

// InteractiveAggregateShareVerificationError is returned by InteractiveAggregate
// when aggregation failed because one or more collected shares did not verify.
//
// CandidateCulprits are the engine's PURE-CRYPTO candidates - the wire (u16) Go
// member identifiers whose FROST shares failed verification against the group's
// own verifying material. They are NOT adjudicated blame: a coordinator that
// aggregated honest shares against a substituted package/root would make those
// honest shares appear here. The Go host performs the envelope-bound blame
// adjudication at an f+1 accuser quorum over these candidates (frozen Phase 7.2b
// spec, section 6); this list is its input, never authoritative on its own.
type InteractiveAggregateShareVerificationError struct {
	SessionID         string
	AttemptID         string
	CandidateCulprits []uint16
	Message           string
}

func (e *InteractiveAggregateShareVerificationError) Error() string {
	return fmt.Sprintf(
		"interactive aggregate share verification failed for session %q attempt %q: "+
			"candidate culprits %v: %s",
		e.SessionID,
		e.AttemptID,
		e.CandidateCulprits,
		e.Message,
	)
}

// interpretInteractiveAggregateError maps a failed InteractiveAggregate call to
// a typed InteractiveAggregateShareVerificationError when the engine reported a
// share-verification failure (carrying the candidate culprits), so callers can
// errors.As it and feed the culprits to the envelope-bound blame adjudication.
// Any other error is returned unchanged. sessionID/attemptID are the caller's
// known request values - the error payload does not echo them.
func interpretInteractiveAggregateError(sessionID, attemptID string, err error) error {
	var structured *buildTaggedTBTCSignerStructuredError
	if errors.As(err, &structured) &&
		structured.Code == interactiveAggregateShareVerificationFailedCode {
		return &InteractiveAggregateShareVerificationError{
			SessionID:         sessionID,
			AttemptID:         attemptID,
			CandidateCulprits: structured.CandidateCulprits,
			Message:           structured.Message,
		}
	}
	return err
}

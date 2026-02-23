//go:build frost_native && frost_tbtc_signer && cgo && !frost_uniffi_sdk

package signing

/*
#cgo CFLAGS: -std=c11
#cgo linux LDFLAGS: -ldl
#cgo freebsd LDFLAGS: -ldl
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <dlfcn.h>

typedef struct {
  uint8_t* ptr;
  size_t len;
} TbtcBuffer;

typedef struct {
  int32_t status_code;
  TbtcBuffer buffer;
} TbtcSignerResult;

typedef TbtcSignerResult (*tbtc_version_fn)(void);
typedef TbtcSignerResult (*tbtc_run_dkg_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_start_sign_round_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_finalize_sign_round_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef void (*tbtc_free_buffer_fn)(uint8_t* ptr, size_t len);

static TbtcSignerResult unavailable_tbtc_signer_result(void) {
  TbtcSignerResult result;
  result.status_code = -1;
  result.buffer.ptr = NULL;
  result.buffer.len = 0;
  return result;
}

static TbtcSignerResult tbtc_signer_version(void) {
  tbtc_version_fn version = (tbtc_version_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_version"
  );
  if (version == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return version();
}

static TbtcSignerResult tbtc_signer_run_dkg(const uint8_t* request_ptr, size_t request_len) {
  tbtc_run_dkg_fn run_dkg = (tbtc_run_dkg_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_run_dkg"
  );
  if (run_dkg == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return run_dkg(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_start_sign_round(const uint8_t* request_ptr, size_t request_len) {
  tbtc_start_sign_round_fn start_sign_round = (tbtc_start_sign_round_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_start_sign_round"
  );
  if (start_sign_round == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return start_sign_round(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_finalize_sign_round(const uint8_t* request_ptr, size_t request_len) {
  tbtc_finalize_sign_round_fn finalize_sign_round = (tbtc_finalize_sign_round_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_finalize_sign_round"
  );
  if (finalize_sign_round == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return finalize_sign_round(request_ptr, request_len);
}

static void tbtc_signer_free_buffer(uint8_t* ptr, size_t len) {
  tbtc_free_buffer_fn free_buffer = (tbtc_free_buffer_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_free_buffer"
  );
  if (free_buffer != NULL) {
    free_buffer(ptr, len);
  }
}
*/
import "C"

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unsafe"
)

type buildTaggedTBTCSignerEngine struct{}
type buildTaggedTBTCSignerErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type buildTaggedTBTCSignerRunDKGRequest struct {
	SessionID    string                                `json:"session_id"`
	Participants []buildTaggedTBTCSignerDKGParticipant `json:"participants"`
	Threshold    uint16                                `json:"threshold"`
}

type buildTaggedTBTCSignerDKGParticipant struct {
	Identifier   uint16 `json:"identifier"`
	PublicKeyHex string `json:"public_key_hex"`
}

type buildTaggedTBTCSignerRunDKGResponse struct {
	SessionID        string `json:"session_id"`
	KeyGroup         string `json:"key_group"`
	ParticipantCount uint16 `json:"participant_count"`
	Threshold        uint16 `json:"threshold"`
	CreatedAtUnix    uint64 `json:"created_at_unix"`
}

type buildTaggedTBTCSignerStartSignRoundRequest struct {
	SessionID  string `json:"session_id"`
	MessageHex string `json:"message_hex"`
	KeyGroup   string `json:"key_group"`
}

type buildTaggedTBTCSignerStartSignRoundResponse struct {
	SessionID             string `json:"session_id"`
	RoundID               string `json:"round_id"`
	RequiredContributions uint16 `json:"required_contributions"`
	MessageDigestHex      string `json:"message_digest_hex"`
}

type buildTaggedTBTCSignerFinalizeSignRoundRequest struct {
	SessionID          string                                           `json:"session_id"`
	RoundContributions []buildTaggedTBTCSignerFinalizeRoundContribution `json:"round_contributions"`
}

type buildTaggedTBTCSignerFinalizeRoundContribution struct {
	Identifier        uint16 `json:"identifier"`
	SignatureShareHex string `json:"signature_share_hex"`
}

type buildTaggedTBTCSignerFinalizeSignRoundResponse struct {
	SessionID    string `json:"session_id"`
	RoundID      string `json:"round_id"`
	SignatureHex string `json:"signature_hex"`
}

const buildTaggedTBTCSignerUnavailableStatusCode = -1

func registerBuildTaggedNativeFROSTSigningEngine() error {
	return RegisterNativeTBTCSignerEngine(&buildTaggedTBTCSignerEngine{})
}

func (bttse *buildTaggedTBTCSignerEngine) Version() (string, error) {
	responsePayload, err := callBuildTaggedTBTCSignerVersion()
	if err != nil {
		return "", err
	}

	version := string(responsePayload)
	if version == "" {
		return "", buildTaggedTBTCSignerOperationError(
			"Version",
			"response version is empty",
		)
	}

	return version, nil
}

func (bttse *buildTaggedTBTCSignerEngine) RunDKG(
	sessionID string,
	participants []NativeTBTCSignerDKGParticipant,
	threshold uint16,
) (*NativeTBTCSignerDKGResult, error) {
	requestPayload, err := buildTaggedTBTCSignerRunDKGRequestPayload(
		sessionID,
		participants,
		threshold,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerRunDKG(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerRunDKGResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) StartSignRound(
	sessionID string,
	message []byte,
	keyGroup string,
) (*NativeTBTCSignerRoundState, error) {
	requestPayload, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		sessionID,
		message,
		keyGroup,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerStartSignRound(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerStartSignRoundResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) FinalizeSignRound(
	sessionID string,
	roundContributions []NativeTBTCSignerRoundContribution,
) ([]byte, error) {
	requestPayload, err := buildTaggedTBTCSignerFinalizeSignRoundRequestPayload(
		sessionID,
		roundContributions,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerFinalizeSignRound(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerFinalizeSignRoundResponse(responsePayload)
}

func buildTaggedTBTCSignerUnavailableError(operation string) error {
	return fmt.Errorf(
		"%w: tbtc-signer bridge operation [%v] is unavailable; link libfrost_tbtc",
		ErrNativeCryptographyUnavailable,
		operation,
	)
}

func buildTaggedTBTCSignerOperationError(
	operation string,
	message string,
) error {
	return fmt.Errorf(
		"%w: tbtc-signer bridge operation [%v] failed: [%s]",
		ErrNativeCryptographyUnavailable,
		operation,
		message,
	)
}

func buildTaggedTBTCSignerRunDKGRequestPayload(
	sessionID string,
	participants []NativeTBTCSignerDKGParticipant,
	threshold uint16,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			"session ID is empty",
		)
	}

	if len(participants) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			"participants are empty",
		)
	}

	if threshold == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			"threshold is zero",
		)
	}

	requestParticipants := make(
		[]buildTaggedTBTCSignerDKGParticipant,
		0,
		len(participants),
	)

	for i, participant := range participants {
		if participant.Identifier == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"RunDKG",
				fmt.Sprintf("participant [%d] identifier is zero", i),
			)
		}

		if participant.PublicKeyHex == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"RunDKG",
				fmt.Sprintf("participant [%d] public key hex is empty", i),
			)
		}

		requestParticipants = append(
			requestParticipants,
			buildTaggedTBTCSignerDKGParticipant{
				Identifier:   participant.Identifier,
				PublicKeyHex: participant.PublicKeyHex,
			},
		)
	}

	request := buildTaggedTBTCSignerRunDKGRequest{
		SessionID:    sessionID,
		Participants: requestParticipants,
		Threshold:    threshold,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			fmt.Sprintf("cannot marshal request: %v", err),
		)
	}

	return payload, nil
}

func decodeBuildTaggedTBTCSignerRunDKGResponse(
	responsePayload []byte,
) (*NativeTBTCSignerDKGResult, error) {
	var response buildTaggedTBTCSignerRunDKGResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	if response.SessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			"response session ID is empty",
		)
	}

	if response.KeyGroup == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			"response key group is empty",
		)
	}

	if response.ParticipantCount == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			"response participant count is zero",
		)
	}

	if response.Threshold == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"RunDKG",
			"response threshold is zero",
		)
	}

	return &NativeTBTCSignerDKGResult{
		SessionID:        response.SessionID,
		KeyGroup:         response.KeyGroup,
		ParticipantCount: response.ParticipantCount,
		Threshold:        response.Threshold,
		CreatedAtUnix:    response.CreatedAtUnix,
	}, nil
}

func buildTaggedTBTCSignerStartSignRoundRequestPayload(
	sessionID string,
	message []byte,
	keyGroup string,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			"session ID is empty",
		)
	}

	if keyGroup == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			"key group is empty",
		)
	}

	request := buildTaggedTBTCSignerStartSignRoundRequest{
		SessionID:  sessionID,
		MessageHex: hex.EncodeToString(message),
		KeyGroup:   keyGroup,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			fmt.Sprintf("cannot marshal request: %v", err),
		)
	}

	return payload, nil
}

func decodeBuildTaggedTBTCSignerStartSignRoundResponse(
	responsePayload []byte,
) (*NativeTBTCSignerRoundState, error) {
	var response buildTaggedTBTCSignerStartSignRoundResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	if response.SessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			"response session ID is empty",
		)
	}

	if response.RoundID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			"response round ID is empty",
		)
	}

	if response.MessageDigestHex == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			"response message digest is empty",
		)
	}

	return &NativeTBTCSignerRoundState{
		SessionID:             response.SessionID,
		RoundID:               response.RoundID,
		RequiredContributions: response.RequiredContributions,
		MessageDigestHex:      response.MessageDigestHex,
	}, nil
}

func buildTaggedTBTCSignerFinalizeSignRoundRequestPayload(
	sessionID string,
	roundContributions []NativeTBTCSignerRoundContribution,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"FinalizeSignRound",
			"session ID is empty",
		)
	}

	if len(roundContributions) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"FinalizeSignRound",
			"round contributions are empty",
		)
	}

	payloadContributions := make(
		[]buildTaggedTBTCSignerFinalizeRoundContribution,
		0,
		len(roundContributions),
	)

	for i, contribution := range roundContributions {
		if len(contribution.Data) == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"FinalizeSignRound",
				fmt.Sprintf("round contribution [%d] data is empty", i),
			)
		}

		payloadContributions = append(
			payloadContributions,
			buildTaggedTBTCSignerFinalizeRoundContribution{
				Identifier:        contribution.Identifier,
				SignatureShareHex: hex.EncodeToString(contribution.Data),
			},
		)
	}

	request := buildTaggedTBTCSignerFinalizeSignRoundRequest{
		SessionID:          sessionID,
		RoundContributions: payloadContributions,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"FinalizeSignRound",
			fmt.Sprintf("cannot marshal request: %v", err),
		)
	}

	return payload, nil
}

func decodeBuildTaggedTBTCSignerFinalizeSignRoundResponse(
	responsePayload []byte,
) ([]byte, error) {
	var response buildTaggedTBTCSignerFinalizeSignRoundResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"FinalizeSignRound",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	if response.SignatureHex == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"FinalizeSignRound",
			"response signature is empty",
		)
	}

	signature, err := hex.DecodeString(response.SignatureHex)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"FinalizeSignRound",
			fmt.Sprintf("response signature is invalid hex: %v", err),
		)
	}

	return signature, nil
}

func callBuildTaggedTBTCSignerVersion() ([]byte, error) {
	result := C.tbtc_signer_version()
	return parseBuildTaggedTBTCSignerResult("Version", result)
}

func callBuildTaggedTBTCSignerRunDKG(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"RunDKG",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_run_dkg(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerStartSignRound(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"StartSignRound",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_start_sign_round(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerFinalizeSignRound(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"FinalizeSignRound",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_finalize_sign_round(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerOperation(
	operation string,
	requestPayload []byte,
	call func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult,
) ([]byte, error) {
	if len(requestPayload) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			"request payload is empty",
		)
	}

	requestPtr := C.CBytes(requestPayload)
	defer C.free(requestPtr)

	result := call((*C.uint8_t)(requestPtr), C.size_t(len(requestPayload)))
	return parseBuildTaggedTBTCSignerResult(operation, result)
}

func parseBuildTaggedTBTCSignerResult(
	operation string,
	result C.TbtcSignerResult,
) ([]byte, error) {
	defer C.tbtc_signer_free_buffer(result.buffer.ptr, result.buffer.len)

	statusCode := int32(result.status_code)
	if statusCode == buildTaggedTBTCSignerUnavailableStatusCode {
		return nil, buildTaggedTBTCSignerUnavailableError(operation)
	}

	var payload []byte
	if result.buffer.ptr != nil && result.buffer.len > 0 {
		payload = C.GoBytes(unsafe.Pointer(result.buffer.ptr), C.int(result.buffer.len))
	}

	if statusCode != 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			buildTaggedTBTCSignerErrorMessage(payload),
		)
	}

	if len(payload) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			"response payload is empty",
		)
	}

	return payload, nil
}

func buildTaggedTBTCSignerErrorMessage(payload []byte) string {
	var errorResponse buildTaggedTBTCSignerErrorResponse
	if err := json.Unmarshal(payload, &errorResponse); err != nil {
		return fmt.Sprintf(
			"cannot decode error payload [%x]: %v",
			payload,
			err,
		)
	}

	if errorResponse.Code == "" && errorResponse.Message == "" {
		return fmt.Sprintf("empty error payload: [%s]", string(payload))
	}

	if errorResponse.Code != "" {
		return fmt.Sprintf("%s: %s", errorResponse.Code, errorResponse.Message)
	}

	return errorResponse.Message
}

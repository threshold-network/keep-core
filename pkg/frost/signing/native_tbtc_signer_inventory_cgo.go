//go:build frost_native && frost_tbtc_signer && cgo

package signing

/*
#cgo linux LDFLAGS: -ldl
#cgo freebsd LDFLAGS: -ldl
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <dlfcn.h>

typedef struct {
  uint8_t* ptr;
  size_t len;
} TbtcSignerInventoryBuffer;

typedef struct {
  int32_t status_code;
  TbtcSignerInventoryBuffer buffer;
} TbtcSignerInventoryResult;

typedef TbtcSignerInventoryResult (*tbtc_retained_key_package_inventory_fn)(void);
typedef TbtcSignerInventoryResult (*tbtc_state_witness_tip_fn)(void);
typedef TbtcSignerInventoryResult (*tbtc_acknowledge_state_witness_checkpoint_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerInventoryResult (*tbtc_recover_state_witness_checkpoint_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerInventoryResult (*tbtc_state_witness_proof_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef void (*tbtc_inventory_free_buffer_fn)(uint8_t* ptr, size_t len);

static TbtcSignerInventoryResult unavailable_tbtc_signer_inventory_result(void) {
  TbtcSignerInventoryResult result;
  result.status_code = -1;
  result.buffer.ptr = NULL;
  result.buffer.len = 0;
  return result;
}

static TbtcSignerInventoryResult tbtc_signer_retained_key_package_inventory(void) {
  tbtc_retained_key_package_inventory_fn operation =
    (tbtc_retained_key_package_inventory_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_retained_key_package_inventory"
    );
  if (operation == NULL) {
    return unavailable_tbtc_signer_inventory_result();
  }
  return operation();
}

static TbtcSignerInventoryResult tbtc_signer_state_witness_tip(void) {
  tbtc_state_witness_tip_fn operation =
    (tbtc_state_witness_tip_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_state_witness_tip"
    );
  if (operation == NULL) {
    return unavailable_tbtc_signer_inventory_result();
  }
  return operation();
}

static TbtcSignerInventoryResult tbtc_signer_acknowledge_state_witness_checkpoint(
  const uint8_t* request_ptr,
  size_t request_len
) {
  tbtc_acknowledge_state_witness_checkpoint_fn operation =
    (tbtc_acknowledge_state_witness_checkpoint_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_acknowledge_state_witness_checkpoint"
    );
  if (operation == NULL) {
    return unavailable_tbtc_signer_inventory_result();
  }
  return operation(request_ptr, request_len);
}

static TbtcSignerInventoryResult tbtc_signer_recover_state_witness_checkpoint(
  const uint8_t* request_ptr,
  size_t request_len
) {
  tbtc_recover_state_witness_checkpoint_fn operation =
    (tbtc_recover_state_witness_checkpoint_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_recover_state_witness_checkpoint"
    );
  if (operation == NULL) {
    return unavailable_tbtc_signer_inventory_result();
  }
  return operation(request_ptr, request_len);
}

static TbtcSignerInventoryResult tbtc_signer_state_witness_proof(
  const uint8_t* request_ptr,
  size_t request_len
) {
  tbtc_state_witness_proof_fn operation = (tbtc_state_witness_proof_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_state_witness_proof"
  );
  if (operation == NULL) {
    return unavailable_tbtc_signer_inventory_result();
  }
  return operation(request_ptr, request_len);
}

static void tbtc_signer_inventory_free_buffer(uint8_t* ptr, size_t len) {
  tbtc_inventory_free_buffer_fn free_buffer =
    (tbtc_inventory_free_buffer_fn)dlsym(RTLD_DEFAULT, "frost_tbtc_free_buffer");
  if (free_buffer != NULL) {
    free_buffer(ptr, len);
  }
}
*/
import "C"

import (
	"fmt"
	"math"
	"unsafe"
)

func ReadNativeTBTCSignerRetainedKeyPackageInventory() (
	*NativeTBTCSignerRetainedKeyPackageInventory,
	error,
) {
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}
	payload, err := parseNativeTBTCSignerInventoryResult(
		"RetainedKeyPackageInventory",
		C.tbtc_signer_retained_key_package_inventory(),
	)
	if err != nil {
		return nil, err
	}
	inventory, err := DecodeNativeTBTCSignerRetainedKeyPackageInventory(payload)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"RetainedKeyPackageInventory",
			err.Error(),
		)
	}
	return inventory, nil
}

// ReadNativeTBTCSignerStateWitnessTip reads the constant-size durable state
// checkpoint used by the request/output barrier. A stale native library
// without frost_tbtc_state_witness_tip fails closed.
func ReadNativeTBTCSignerStateWitnessTip() (
	*NativeTBTCSignerStateWitnessTip,
	error,
) {
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}
	payload, err := parseNativeTBTCSignerInventoryResult(
		"StateWitnessTip",
		C.tbtc_signer_state_witness_tip(),
	)
	if err != nil {
		return nil, err
	}
	tip, err := DecodeNativeTBTCSignerStateWitnessTip(payload)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"StateWitnessTip",
			err.Error(),
		)
	}
	return tip, nil
}

// AcknowledgeNativeTBTCSignerStateWitnessCheckpoint installs the exact signed
// remote CAS response into Rust's descriptor-bound anchor metadata. This is an
// internal half of the output barrier and deliberately bypasses the
// request-taking operation guard to avoid recursion.
func AcknowledgeNativeTBTCSignerStateWitnessCheckpoint(
	signedAcknowledgementJSON []byte,
) (*NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult, error) {
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}
	if len(signedAcknowledgementJSON) == 0 ||
		len(signedAcknowledgementJSON) > 64*1024 {
		return nil, buildTaggedTBTCSignerOperationError(
			"AcknowledgeStateWitnessCheckpoint",
			"signed acknowledgement size is invalid",
		)
	}
	requestPointer := C.CBytes(signedAcknowledgementJSON)
	defer func() {
		zeroBytes(unsafe.Slice((*byte)(requestPointer), len(signedAcknowledgementJSON)))
		C.free(requestPointer)
	}()
	payload, err := parseNativeTBTCSignerInventoryResult(
		"AcknowledgeStateWitnessCheckpoint",
		C.tbtc_signer_acknowledge_state_witness_checkpoint(
			(*C.uint8_t)(requestPointer),
			C.size_t(len(signedAcknowledgementJSON)),
		),
	)
	if err != nil {
		return nil, err
	}
	result, err :=
		DecodeNativeTBTCSignerStateWitnessCheckpointAcknowledgementResult(payload)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"AcknowledgeStateWitnessCheckpoint",
			err.Error(),
		)
	}
	return result, nil
}

func RecoverNativeTBTCSignerStateWitnessCheckpoint(
	exactReadResponseJSON []byte,
) (*NativeTBTCSignerStateWitnessCheckpointRecoveryResult, error) {
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}
	if len(exactReadResponseJSON) == 0 || len(exactReadResponseJSON) > 256*1024 {
		return nil, buildTaggedTBTCSignerOperationError(
			"RecoverStateWitnessCheckpoint",
			"signed read recovery response size is invalid",
		)
	}
	requestPointer := C.CBytes(exactReadResponseJSON)
	defer func() {
		zeroBytes(unsafe.Slice((*byte)(requestPointer), len(exactReadResponseJSON)))
		C.free(requestPointer)
	}()
	payload, err := parseNativeTBTCSignerInventoryResult(
		"RecoverStateWitnessCheckpoint",
		C.tbtc_signer_recover_state_witness_checkpoint(
			(*C.uint8_t)(requestPointer),
			C.size_t(len(exactReadResponseJSON)),
		),
	)
	if err != nil {
		return nil, err
	}
	result, err := DecodeNativeTBTCSignerStateWitnessCheckpointRecoveryResult(
		payload,
	)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"RecoverStateWitnessCheckpoint",
			err.Error(),
		)
	}
	return result, nil
}

func ReadNativeTBTCSignerStateWitnessProof(
	request *NativeTBTCSignerStateWitnessProofRequest,
) (*NativeTBTCSignerStateWitnessProof, error) {
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}
	requestPayload, err := request.MarshalJSON()
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError("StateWitnessProof", err.Error())
	}
	requestPointer := C.CBytes(requestPayload)
	defer func() {
		zeroBytes(unsafe.Slice((*byte)(requestPointer), len(requestPayload)))
		C.free(requestPointer)
	}()
	payload, err := parseNativeTBTCSignerInventoryResult(
		"StateWitnessProof",
		C.tbtc_signer_state_witness_proof(
			(*C.uint8_t)(requestPointer),
			C.size_t(len(requestPayload)),
		),
	)
	if err != nil {
		return nil, err
	}
	proof, err := DecodeNativeTBTCSignerStateWitnessProof(payload)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError("StateWitnessProof", err.Error())
	}
	return proof, nil
}

func parseNativeTBTCSignerInventoryResult(
	operation string,
	result C.TbtcSignerInventoryResult,
) ([]byte, error) {
	if result.buffer.ptr != nil {
		defer C.tbtc_signer_inventory_free_buffer(result.buffer.ptr, result.buffer.len)
	}
	if uint64(result.buffer.len) > uint64(math.MaxInt32) {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("response buffer length [%d] exceeds maximum", uint64(result.buffer.len)),
		)
	}
	var payload []byte
	if result.buffer.ptr != nil && result.buffer.len > 0 {
		payload = C.GoBytes(unsafe.Pointer(result.buffer.ptr), C.int(result.buffer.len))
	}
	if err := buildTaggedTBTCSignerResultStatusError(
		operation,
		int32(result.status_code),
		payload,
	); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(operation, "response payload is empty")
	}
	return payload, nil
}

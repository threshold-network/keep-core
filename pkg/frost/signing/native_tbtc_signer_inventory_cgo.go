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

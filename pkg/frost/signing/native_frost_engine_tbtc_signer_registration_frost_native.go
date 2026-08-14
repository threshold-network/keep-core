//go:build frost_native && frost_tbtc_signer && cgo

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
typedef TbtcSignerResult (*tbtc_abi_version_fn)(void);
typedef TbtcSignerResult (*tbtc_dkg_part1_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_dkg_part2_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_dkg_part3_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_persist_distributed_dkg_key_package_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_retire_distributed_dkg_key_packages_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_begin_share_repair_session_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_finish_share_repair_session_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_share_repair_part1_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_share_repair_part2_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_install_repaired_share_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_new_signing_package_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_verify_signature_share_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_derive_interactive_attempt_context_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_interactive_session_open_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_interactive_round1_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_interactive_round2_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_interactive_session_abort_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_interactive_aggregate_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_build_taproot_tx_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_init_signer_config_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_trigger_emergency_rekey_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_durable_store_identity_fn)(void);
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

static TbtcSignerResult tbtc_signer_abi_version(void) {
  tbtc_abi_version_fn abi_version = (tbtc_abi_version_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_abi_version"
  );
  if (abi_version == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return abi_version();
}

static TbtcSignerResult tbtc_signer_dkg_part1(const uint8_t* request_ptr, size_t request_len) {
  tbtc_dkg_part1_fn dkg_part1 = (tbtc_dkg_part1_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_dkg_part1"
  );
  if (dkg_part1 == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return dkg_part1(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_dkg_part2(const uint8_t* request_ptr, size_t request_len) {
  tbtc_dkg_part2_fn dkg_part2 = (tbtc_dkg_part2_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_dkg_part2"
  );
  if (dkg_part2 == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return dkg_part2(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_dkg_part3(const uint8_t* request_ptr, size_t request_len) {
  tbtc_dkg_part3_fn dkg_part3 = (tbtc_dkg_part3_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_dkg_part3"
  );
  if (dkg_part3 == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return dkg_part3(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_persist_distributed_dkg_key_package(const uint8_t* request_ptr, size_t request_len) {
  tbtc_persist_distributed_dkg_key_package_fn persist =
    (tbtc_persist_distributed_dkg_key_package_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_persist_distributed_dkg_key_package"
    );
  if (persist == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return persist(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_retire_distributed_dkg_key_packages(const uint8_t* request_ptr, size_t request_len) {
  tbtc_retire_distributed_dkg_key_packages_fn retire =
    (tbtc_retire_distributed_dkg_key_packages_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_retire_distributed_dkg_key_packages"
    );
  if (retire == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return retire(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_begin_share_repair_session(const uint8_t* request_ptr, size_t request_len) {
  tbtc_begin_share_repair_session_fn begin =
    (tbtc_begin_share_repair_session_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_begin_share_repair_session"
    );
  if (begin == NULL) {
    return unavailable_tbtc_signer_result();
  }
  return begin(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_finish_share_repair_session(const uint8_t* request_ptr, size_t request_len) {
  tbtc_finish_share_repair_session_fn finish =
    (tbtc_finish_share_repair_session_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_finish_share_repair_session"
    );
  if (finish == NULL) {
    return unavailable_tbtc_signer_result();
  }
  return finish(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_share_repair_part1(const uint8_t* request_ptr, size_t request_len) {
  tbtc_share_repair_part1_fn part1 =
    (tbtc_share_repair_part1_fn)dlsym(RTLD_DEFAULT, "frost_tbtc_share_repair_part1");
  if (part1 == NULL) {
    return unavailable_tbtc_signer_result();
  }
  return part1(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_share_repair_part2(const uint8_t* request_ptr, size_t request_len) {
  tbtc_share_repair_part2_fn part2 =
    (tbtc_share_repair_part2_fn)dlsym(RTLD_DEFAULT, "frost_tbtc_share_repair_part2");
  if (part2 == NULL) {
    return unavailable_tbtc_signer_result();
  }
  return part2(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_install_repaired_share(const uint8_t* request_ptr, size_t request_len) {
  tbtc_install_repaired_share_fn install =
    (tbtc_install_repaired_share_fn)dlsym(RTLD_DEFAULT, "frost_tbtc_install_repaired_share");
  if (install == NULL) {
    return unavailable_tbtc_signer_result();
  }
  return install(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_new_signing_package(const uint8_t* request_ptr, size_t request_len) {
  tbtc_new_signing_package_fn new_signing_package = (tbtc_new_signing_package_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_new_signing_package"
  );
  if (new_signing_package == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return new_signing_package(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_verify_signature_share(const uint8_t* request_ptr, size_t request_len) {
  tbtc_verify_signature_share_fn verify_signature_share = (tbtc_verify_signature_share_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_verify_signature_share"
  );
  if (verify_signature_share == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return verify_signature_share(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_derive_interactive_attempt_context(const uint8_t* request_ptr, size_t request_len) {
  tbtc_derive_interactive_attempt_context_fn derive_interactive_attempt_context = (tbtc_derive_interactive_attempt_context_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_derive_interactive_attempt_context"
  );
  if (derive_interactive_attempt_context == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return derive_interactive_attempt_context(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_interactive_session_open(const uint8_t* request_ptr, size_t request_len) {
  tbtc_interactive_session_open_fn interactive_session_open = (tbtc_interactive_session_open_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_interactive_session_open"
  );
  if (interactive_session_open == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return interactive_session_open(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_interactive_round1(const uint8_t* request_ptr, size_t request_len) {
  tbtc_interactive_round1_fn interactive_round1 = (tbtc_interactive_round1_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_interactive_round1"
  );
  if (interactive_round1 == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return interactive_round1(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_interactive_round2(const uint8_t* request_ptr, size_t request_len) {
  tbtc_interactive_round2_fn interactive_round2 = (tbtc_interactive_round2_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_interactive_round2"
  );
  if (interactive_round2 == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return interactive_round2(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_interactive_session_abort(const uint8_t* request_ptr, size_t request_len) {
  tbtc_interactive_session_abort_fn interactive_session_abort = (tbtc_interactive_session_abort_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_interactive_session_abort"
  );
  if (interactive_session_abort == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return interactive_session_abort(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_interactive_aggregate(const uint8_t* request_ptr, size_t request_len) {
  tbtc_interactive_aggregate_fn interactive_aggregate = (tbtc_interactive_aggregate_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_interactive_aggregate"
  );
  if (interactive_aggregate == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return interactive_aggregate(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_build_taproot_tx(const uint8_t* request_ptr, size_t request_len) {
  tbtc_build_taproot_tx_fn build_taproot_tx = (tbtc_build_taproot_tx_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_build_taproot_tx"
  );
  if (build_taproot_tx == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return build_taproot_tx(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_init_signer_config(const uint8_t* request_ptr, size_t request_len) {
  tbtc_init_signer_config_fn init_signer_config = (tbtc_init_signer_config_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_init_signer_config"
  );
  if (init_signer_config == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return init_signer_config(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_trigger_emergency_rekey(const uint8_t* request_ptr, size_t request_len) {
  tbtc_trigger_emergency_rekey_fn trigger_emergency_rekey = (tbtc_trigger_emergency_rekey_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_trigger_emergency_rekey"
  );
  if (trigger_emergency_rekey == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return trigger_emergency_rekey(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_durable_store_identity(void) {
  tbtc_durable_store_identity_fn durable_store_identity =
    (tbtc_durable_store_identity_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_durable_store_identity"
    );
  if (durable_store_identity == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return durable_store_identity();
}

static int tbtc_signer_free_buffer_available(void) {
  return dlsym(RTLD_DEFAULT, "frost_tbtc_free_buffer") != NULL;
}

static int tbtc_signer_share_repair_symbols_available(void) {
  return
    dlsym(RTLD_DEFAULT, "frost_tbtc_begin_share_repair_session") != NULL &&
    dlsym(RTLD_DEFAULT, "frost_tbtc_finish_share_repair_session") != NULL &&
    dlsym(RTLD_DEFAULT, "frost_tbtc_share_repair_part1") != NULL &&
    dlsym(RTLD_DEFAULT, "frost_tbtc_share_repair_part2") != NULL &&
    dlsym(RTLD_DEFAULT, "frost_tbtc_install_repaired_share") != NULL;
}

static void tbtc_signer_scrub_and_free_buffer(uint8_t* ptr, size_t len) {
  if (ptr != NULL) {
    volatile uint8_t* cursor = (volatile uint8_t*)ptr;
    for (size_t index = 0; index < len; index++) {
      cursor[index] = 0;
    }
  }
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
	"errors"
	"fmt"
	"math"
	"strings"
	"unsafe"
)

type buildTaggedTBTCSignerEngine struct{}

// The cgo-backed engine must satisfy the runner's interactiveSigningEngine
// boundary (defined under the frost_native tag, which this file also carries).
// This assertion lives in the cgo wiring layer so widening the interface there
// is compile-checked against the real engine, even before a production path
// constructs one.
var _ interactiveSigningEngine = (*buildTaggedTBTCSignerEngine)(nil)

// The cgo engine must also satisfy the share-blame re-verifier boundary
// (Round2ShareVerifyingEngine, RFC-21 Phase 7.3 share-blame): the drive type-asserts
// the registered engine to it to classify interactive aggregate share-verification
// culprits. Compile-check it here against the real engine.
var _ Round2ShareVerifyingEngine = (*buildTaggedTBTCSignerEngine)(nil)
var _ NativeTBTCSignerDistributedDKGRetirementEngine = (*buildTaggedTBTCSignerEngine)(nil)
var _ NativeTBTCSignerShareRepairEngine = (*buildTaggedTBTCSignerEngine)(nil)

type buildTaggedTBTCSignerRunDKGResponse struct {
	SessionID        string `json:"session_id"`
	KeyGroup         string `json:"key_group"`
	ParticipantCount uint16 `json:"participant_count"`
	Threshold        uint16 `json:"threshold"`
	CreatedAtUnix    uint64 `json:"created_at_unix"`
}

type buildTaggedTBTCSignerDKGPart1Request struct {
	ParticipantIdentifier string `json:"participant_identifier"`
	MaxSigners            uint16 `json:"max_signers"`
	MinSigners            uint16 `json:"min_signers"`
}

type buildTaggedTBTCSignerDKGRound1Package struct {
	Identifier string `json:"identifier"`
	PackageHex string `json:"package_hex"`
}

type buildTaggedTBTCSignerDKGRound2Package struct {
	Identifier       string  `json:"identifier"`
	SenderIdentifier *string `json:"sender_identifier,omitempty"`
	PackageHex       string  `json:"package_hex"`
}

type buildTaggedTBTCSignerDKGPart1Response struct {
	SecretPackageHex hexBytes                               `json:"secret_package_hex"`
	Package          *buildTaggedTBTCSignerDKGRound1Package `json:"package"`
}

type buildTaggedTBTCSignerDKGPart2Request struct {
	SecretPackageHex hexBytes                                `json:"secret_package_hex"`
	Round1Packages   []buildTaggedTBTCSignerDKGRound1Package `json:"round1_packages"`
}

type buildTaggedTBTCSignerDKGPart2Response struct {
	SecretPackageHex hexBytes                                `json:"secret_package_hex"`
	Packages         []buildTaggedTBTCSignerDKGRound2Package `json:"packages"`
}

type buildTaggedTBTCSignerDKGPart3Request struct {
	SecretPackageHex hexBytes                                `json:"secret_package_hex"`
	Round1Packages   []buildTaggedTBTCSignerDKGRound1Package `json:"round1_packages"`
	Round2Packages   []buildTaggedTBTCSignerDKGRound2Package `json:"round2_packages"`
}

type buildTaggedTBTCSignerNativeFROSTKeyPackage struct {
	Identifier string `json:"identifier"`
	// DataHex is the participant's long-term FROST key package: secret material.
	// hexBytes keeps it out of an unzeroable Go string.
	DataHex hexBytes `json:"data_hex"`
}

type buildTaggedTBTCSignerNativeFROSTPublicKeyPackage struct {
	VerifyingShares map[string]string `json:"verifying_shares"`
	VerifyingKey    string            `json:"verifying_key"`
}

type buildTaggedTBTCSignerDKGPart3Response struct {
	KeyPackage       *buildTaggedTBTCSignerNativeFROSTKeyPackage       `json:"key_package"`
	PublicKeyPackage *buildTaggedTBTCSignerNativeFROSTPublicKeyPackage `json:"public_key_package"`
}

type buildTaggedTBTCSignerPersistDistributedDKGKeyPackageRequest struct {
	SessionID             string                                            `json:"session_id"`
	ParticipantIdentifier uint16                                            `json:"participant_identifier"`
	Threshold             uint16                                            `json:"threshold"`
	ParticipantCount      uint16                                            `json:"participant_count"`
	KeyPackage            *buildTaggedTBTCSignerNativeFROSTKeyPackage       `json:"key_package"`
	PublicKeyPackage      *buildTaggedTBTCSignerNativeFROSTPublicKeyPackage `json:"public_key_package"`
}

type buildTaggedTBTCSignerRetireDistributedDKGKeyPackagesRequest struct {
	KeyGroup string `json:"key_group"`
}

type buildTaggedTBTCSignerRetireDistributedDKGKeyPackagesResponse struct {
	KeyGroup               string `json:"key_group"`
	Retired                bool   `json:"retired"`
	RetiredKeyPackageCount uint16 `json:"retired_key_package_count"`
}

const (
	buildTaggedTBTCSignerShareRepairPublicKeyLength = shareRepairEphemeralPublicKeyLength
	buildTaggedTBTCSignerShareRepairPayloadLength   = shareRepairEncryptedScalarPayloadLength
)

type buildTaggedTBTCSignerShareRepairSessionRequest struct {
	Authorization         *ShareRepairAuthorization `json:"authorization"`
	ParticipantIdentifier uint16                    `json:"participant_identifier"`
}

type buildTaggedTBTCSignerBeginShareRepairSessionResponse struct {
	ContextDigest         string `json:"context_digest"`
	ParticipantIdentifier uint16 `json:"participant_identifier"`
	StoreFingerprint      string `json:"store_fingerprint"`
	TransportPublicKeyHex string `json:"transport_public_key_hex"`
}

type buildTaggedTBTCSignerFinishShareRepairSessionResponse struct {
	ContextDigest         string `json:"context_digest"`
	ParticipantIdentifier uint16 `json:"participant_identifier"`
	Finished              bool   `json:"finished"`
}

type buildTaggedTBTCSignerShareRepairEncryptedDelta struct {
	ContextDigest       string `json:"context_digest"`
	SenderIdentifier    uint16 `json:"sender_identifier"`
	RecipientIdentifier uint16 `json:"recipient_identifier"`
	PayloadHex          string `json:"payload_hex"`
}

type buildTaggedTBTCSignerShareRepairPart1Request struct {
	Authorization    *ShareRepairAuthorization   `json:"authorization"`
	HelperIdentifier uint16                      `json:"helper_identifier"`
	TransportRoster  *ShareRepairTransportRoster `json:"transport_roster"`
}

type buildTaggedTBTCSignerShareRepairPart1Response struct {
	ContextDigest    string                                            `json:"context_digest"`
	HelperIdentifier uint16                                            `json:"helper_identifier"`
	PublicKeyPackage *buildTaggedTBTCSignerNativeFROSTPublicKeyPackage `json:"public_key_package"`
	Deltas           []buildTaggedTBTCSignerShareRepairEncryptedDelta  `json:"deltas"`
}

type buildTaggedTBTCSignerShareRepairPart2Request struct {
	Authorization    *ShareRepairAuthorization                        `json:"authorization"`
	HelperIdentifier uint16                                           `json:"helper_identifier"`
	Deltas           []buildTaggedTBTCSignerShareRepairEncryptedDelta `json:"deltas"`
	TransportRoster  *ShareRepairTransportRoster                      `json:"transport_roster"`
}

type buildTaggedTBTCSignerShareRepairEncryptedSigma struct {
	ContextDigest    string `json:"context_digest"`
	HelperIdentifier uint16 `json:"helper_identifier"`
	PayloadHex       string `json:"payload_hex"`
}

type buildTaggedTBTCSignerShareRepairPart2Response struct {
	ContextDigest string                                          `json:"context_digest"`
	Sigma         *buildTaggedTBTCSignerShareRepairEncryptedSigma `json:"sigma"`
}

type buildTaggedTBTCSignerInstallRepairedShareRequest struct {
	Authorization    *ShareRepairAuthorization                         `json:"authorization"`
	PublicKeyPackage *buildTaggedTBTCSignerNativeFROSTPublicKeyPackage `json:"public_key_package"`
	Sigmas           []buildTaggedTBTCSignerShareRepairEncryptedSigma  `json:"sigmas"`
	TransportRoster  *ShareRepairTransportRoster                       `json:"transport_roster"`
}

type buildTaggedTBTCSignerInstallRepairedShareResponse struct {
	Schema                 string `json:"schema"`
	SessionID              string `json:"session_id"`
	KeyGroup               string `json:"key_group"`
	TargetIdentifier       uint16 `json:"target_identifier"`
	RecoveryEpoch          uint64 `json:"recovery_epoch"`
	AuthorizationDigest    string `json:"authorization_digest"`
	ActiveStoreFingerprint string `json:"active_store_fingerprint"`
	Idempotent             bool   `json:"idempotent"`
}

type buildTaggedTBTCSignerTriggerEmergencyRekeyRequest struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}

type buildTaggedTBTCSignerTriggerEmergencyRekeyResponse struct {
	SessionID               string `json:"session_id"`
	EmergencyRekeyRequired  bool   `json:"emergency_rekey_required"`
	Reason                  string `json:"reason"`
	TriggeredAtUnix         uint64 `json:"triggered_at_unix"`
	RecommendedNewSessionID string `json:"recommended_new_session_id"`
}

type buildTaggedTBTCSignerNativeFROSTCommitment struct {
	Identifier string `json:"identifier"`
	DataHex    string `json:"data_hex"`
}

type buildTaggedTBTCSignerNativeFROSTSignatureShare struct {
	Identifier string `json:"identifier"`
	DataHex    string `json:"data_hex"`
}

type buildTaggedTBTCSignerNewSigningPackageRequest struct {
	MessageHex  string                                       `json:"message_hex"`
	Commitments []buildTaggedTBTCSignerNativeFROSTCommitment `json:"commitments"`
}

type buildTaggedTBTCSignerNewSigningPackageResponse struct {
	SigningPackageHex string `json:"signing_package_hex"`
}

type buildTaggedTBTCSignerVerifySignatureShareRequest struct {
	SessionID            string  `json:"session_id"`
	SigningPackageHex    string  `json:"signing_package_hex"`
	SignatureShareHex    string  `json:"signature_share_hex"`
	MemberIdentifier     uint16  `json:"member_identifier"`
	TaprootMerkleRootHex *string `json:"taproot_merkle_root_hex,omitempty"`
}

type buildTaggedTBTCSignerVerifySignatureShareResponse struct {
	Verdict string `json:"verdict"`
}

type buildTaggedTBTCSignerBuildTaprootTxRequest struct {
	SessionID     string                                      `json:"session_id"`
	Inputs        []buildTaggedTBTCSignerBuildTaprootTxInput  `json:"inputs"`
	Outputs       []buildTaggedTBTCSignerBuildTaprootTxOutput `json:"outputs"`
	ScriptTreeHex *string                                     `json:"script_tree_hex,omitempty"`
}

type buildTaggedTBTCSignerBuildTaprootTxInput struct {
	TxIDHex         string `json:"txid_hex"`
	Vout            uint32 `json:"vout"`
	ValueSats       uint64 `json:"value_sats"`
	ScriptPubKeyHex string `json:"script_pubkey_hex"`
}

type buildTaggedTBTCSignerBuildTaprootTxOutput struct {
	ScriptPubKeyHex string `json:"script_pubkey_hex"`
	ValueSats       uint64 `json:"value_sats"`
}

type buildTaggedTBTCSignerBuildTaprootTxResponse struct {
	SessionID                   string   `json:"session_id"`
	TxHex                       string   `json:"tx_hex"`
	TaprootKeySpendSighashesHex []string `json:"taproot_key_spend_sighashes_hex"`
}

const buildTaggedTBTCSignerUnavailableStatusCode = -1

func registerBuildTaggedNativeFROSTSigningEngine() error {
	engine := &buildTaggedTBTCSignerEngine{}

	// Do not register the tbtc-signer bridge as the generic UniFFI-shaped
	// FROST DKG/signing engine. That path persists `frost-uniffi-v2` wallet
	// material, which cannot produce Taproot-tweaked signatures. A wallet
	// using that material can accept Taproot deposits that are effectively
	// unsweepable, so this must fail before new FROST wallet material exists.
	// New FROST wallets in this build must use the coarse
	// `frost-tbtc-signer-v1` material path exclusively.
	//
	// RFC-21 Phase 7.3: this same engine satisfies interactiveSigningEngine, and
	// it IS registered as the interactive provider here. The prerequisites the
	// registration waited on have landed -- the f+1 blame/evidence bridge and the
	// stable ROAST session-key plumbing -- so the executor may drive the real cgo
	// engine through the interactive ROAST path. Registration on its own changes
	// nothing for an operator: the executor still requires the default-off
	// KEEP_CORE_FROST_INTERACTIVE_SIGNING_ENABLED opt-in (read per call, see
	// roast_interactive_signing_gate.go), so the interactive path stays dormant
	// until explicitly enabled on a cgo build, and the coarse path remains the
	// fallback. The frost-secp256k1-tr engine external audit gates the
	// threshold-ECDSA -> FROST CUTOVER in production (turning that opt-in on for
	// real wallets), NOT this registration. The provider is a factory: each call
	// returns a fresh stateless bridge handle (interactive sessions live
	// engine-side, keyed by session id).
	RegisterInteractiveSigningEngineProvider(func() interactiveSigningEngine {
		return &buildTaggedTBTCSignerEngine{}
	})

	return RegisterNativeTBTCSignerEngine(engine)
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

func (bttse *buildTaggedTBTCSignerEngine) Part1(
	participantIdentifier string,
	maxSigners uint16,
	minSigners uint16,
) (*NativeFROSTDKGPart1Result, error) {
	requestPayload, err := buildTaggedTBTCSignerDKGPart1RequestPayload(
		participantIdentifier,
		maxSigners,
		minSigners,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerDKGPart1(requestPayload)
	if err != nil {
		return nil, err
	}
	// The response carries the round-1 secret package (private polynomial
	// coefficients that must never be broadcast). Scrub the Go-side transport
	// buffer once decoded, mirroring the Sign path's zeroBytes hygiene; the
	// decoded secret returned to the caller is a fresh, independent copy.
	defer zeroBytes(responsePayload)

	return decodeBuildTaggedTBTCSignerDKGPart1Response(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) Part2(
	secretPackage *NativeFROSTDKGRound1SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
) (*NativeFROSTDKGPart2Result, error) {
	requestPayload, err := buildTaggedTBTCSignerDKGPart2RequestPayload(
		secretPackage,
		round1Packages,
	)
	if err != nil {
		return nil, err
	}
	// The request embeds the round-1 secret package; scrub the Go-side buffer
	// on every return path (including a failed FFI call), mirroring the Sign
	// path. The C copy is separately scrubbed in callBuildTaggedTBTCSignerOperation.
	defer zeroBytes(requestPayload)

	responsePayload, err := callBuildTaggedTBTCSignerDKGPart2(requestPayload)
	if err != nil {
		return nil, err
	}
	// The response carries the round-2 secret package and the per-recipient
	// round-2 packages (secret shares). Scrub the Go-side transport buffer once
	// decoded; the decoded values returned to the caller are fresh copies.
	defer zeroBytes(responsePayload)

	return decodeBuildTaggedTBTCSignerDKGPart2Response(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) Part3(
	secretPackage *NativeFROSTDKGRound2SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
	round2Packages []*NativeFROSTDKGRound2Package,
) (*NativeFROSTDKGResult, error) {
	requestPayload, err := buildTaggedTBTCSignerDKGPart3RequestPayload(
		secretPackage,
		round1Packages,
		round2Packages,
	)
	if err != nil {
		return nil, err
	}
	// The request embeds the round-2 secret package and the received round-2
	// packages (incoming secret shares); scrub the Go-side buffer on every
	// return path, mirroring the Sign path. The C copy is separately scrubbed
	// in callBuildTaggedTBTCSignerOperation.
	defer zeroBytes(requestPayload)

	responsePayload, err := callBuildTaggedTBTCSignerDKGPart3(requestPayload)
	if err != nil {
		return nil, err
	}
	// The response carries the final key package (the long-term signing share).
	// Scrub the Go-side transport buffer once decoded; the decoded key package
	// returned to the caller is a fresh copy.
	defer zeroBytes(responsePayload)

	return decodeBuildTaggedTBTCSignerDKGPart3Response(responsePayload)
}

// PersistDistributedDKGKeyPackage stores this node's Part3 key package plus the
// group public key package as signing material the interactive signing path can
// load (keyed by the returned key group). A distributed DKG - unlike the dealer
// RunDKG - leaves each node with only its OWN secret key package, which Part3
// returns; this persists it so the wallet can sign.
func (bttse *buildTaggedTBTCSignerEngine) PersistDistributedDKGKeyPackage(
	sessionID string,
	participantIdentifier uint16,
	threshold uint16,
	participantCount uint16,
	keyPackage *NativeFROSTKeyPackage,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) (*NativeTBTCSignerDKGResult, error) {
	requestPayload, err := buildTaggedTBTCSignerPersistDistributedDKGKeyPackageRequestPayload(
		sessionID,
		participantIdentifier,
		threshold,
		participantCount,
		keyPackage,
		publicKeyPackage,
	)
	if err != nil {
		return nil, err
	}
	// The request embeds this node's serialized key package (secret material);
	// scrub the Go-side transport buffer on every return path, mirroring Sign/Part3.
	defer zeroBytes(requestPayload)

	responsePayload, err := callBuildTaggedTBTCSignerPersistDistributedDKGKeyPackage(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerRunDKGResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) RetireDistributedDKGKeyPackages(
	keyGroup string,
) error {
	requestPayload, err :=
		buildTaggedTBTCSignerRetireDistributedDKGKeyPackagesRequestPayload(keyGroup)
	if err != nil {
		return err
	}
	responsePayload, err :=
		callBuildTaggedTBTCSignerRetireDistributedDKGKeyPackages(requestPayload)
	if err != nil {
		return err
	}
	return decodeBuildTaggedTBTCSignerRetireDistributedDKGKeyPackagesResponse(
		responsePayload,
		keyGroup,
	)
}

func (bttse *buildTaggedTBTCSignerEngine) BeginShareRepairSession(
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) (*NativeShareRepairSession, error) {
	requestPayload, err := buildTaggedTBTCSignerShareRepairSessionRequestPayload(
		"BeginShareRepairSession",
		authorization,
		participantIdentifier,
	)
	if err != nil {
		return nil, err
	}
	responsePayload, err := callBuildTaggedTBTCSignerBeginShareRepairSession(
		requestPayload,
	)
	if err != nil {
		return nil, err
	}
	return decodeBuildTaggedTBTCSignerBeginShareRepairSessionResponse(
		responsePayload,
		authorization,
		participantIdentifier,
	)
}

func (bttse *buildTaggedTBTCSignerEngine) FinishShareRepairSession(
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) error {
	requestPayload, err := buildTaggedTBTCSignerShareRepairSessionRequestPayload(
		"FinishShareRepairSession",
		authorization,
		participantIdentifier,
	)
	if err != nil {
		return err
	}
	responsePayload, err := callBuildTaggedTBTCSignerFinishShareRepairSession(
		requestPayload,
	)
	if err != nil {
		return err
	}
	return decodeBuildTaggedTBTCSignerFinishShareRepairSessionResponse(
		responsePayload,
		authorization,
		participantIdentifier,
	)
}

func (bttse *buildTaggedTBTCSignerEngine) ShareRepairPart1(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	transportRoster *ShareRepairTransportRoster,
) (*NativeShareRepairPart1Result, error) {
	requestPayload, err := buildTaggedTBTCSignerShareRepairPart1RequestPayload(
		authorization,
		helperIdentifier,
		transportRoster,
	)
	if err != nil {
		return nil, err
	}
	responsePayload, err := callBuildTaggedTBTCSignerShareRepairPart1(requestPayload)
	if err != nil {
		return nil, err
	}
	return decodeBuildTaggedTBTCSignerShareRepairPart1Response(
		responsePayload,
		authorization,
		helperIdentifier,
	)
}

func (bttse *buildTaggedTBTCSignerEngine) ShareRepairPart2(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	deltas []*NativeShareRepairEncryptedDelta,
	transportRoster *ShareRepairTransportRoster,
) (*NativeShareRepairPart2Result, error) {
	requestPayload, err := buildTaggedTBTCSignerShareRepairPart2RequestPayload(
		authorization,
		helperIdentifier,
		deltas,
		transportRoster,
	)
	if err != nil {
		return nil, err
	}
	responsePayload, err := callBuildTaggedTBTCSignerShareRepairPart2(requestPayload)
	if err != nil {
		return nil, err
	}
	return decodeBuildTaggedTBTCSignerShareRepairPart2Response(
		responsePayload,
		authorization,
		helperIdentifier,
	)
}

func (bttse *buildTaggedTBTCSignerEngine) InstallRepairedShare(
	authorization *ShareRepairAuthorization,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
	sigmas []*NativeShareRepairEncryptedSigma,
	transportRoster *ShareRepairTransportRoster,
) (*NativeShareRepairInstallResult, error) {
	requestPayload, err := buildTaggedTBTCSignerInstallRepairedShareRequestPayload(
		authorization,
		publicKeyPackage,
		sigmas,
		transportRoster,
	)
	if err != nil {
		return nil, err
	}
	responsePayload, err := callBuildTaggedTBTCSignerInstallRepairedShare(requestPayload)
	if err != nil {
		return nil, err
	}
	return decodeBuildTaggedTBTCSignerInstallRepairedShareResponse(
		responsePayload,
		authorization,
	)
}

func (bttse *buildTaggedTBTCSignerEngine) NewSigningPackage(
	message []byte,
	commitments []nativeFROSTCommitment,
) (signingPackageData []byte, err error) {
	requestPayload, err := buildTaggedTBTCSignerNewSigningPackageRequestPayload(
		message,
		commitments,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerNewSigningPackage(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerNewSigningPackageResponse(responsePayload)
}

// VerifySignatureShare re-verifies ONE retained round-2 signature share against
// an attempt's signing package, returning the engine's tri-state verdict. It
// backs the Go host's Round2ShareVerifier (member-blame classifier). On any
// FFI-transport error it returns (NativeShareVerdictIndeterminate, err); the
// caller fails closed to don't-blame.
func (bttse *buildTaggedTBTCSignerEngine) VerifySignatureShare(
	sessionID string,
	signingPackage []byte,
	signatureShare []byte,
	memberIdentifier uint16,
	taprootMerkleRoot *[32]byte,
) (NativeShareVerificationVerdict, error) {
	requestPayload, err := buildTaggedTBTCSignerVerifySignatureShareRequestPayload(
		sessionID,
		signingPackage,
		signatureShare,
		memberIdentifier,
		taprootMerkleRoot,
	)
	if err != nil {
		return NativeShareVerdictIndeterminate, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerVerifySignatureShare(requestPayload)
	if err != nil {
		return NativeShareVerdictIndeterminate, err
	}

	return decodeBuildTaggedTBTCSignerVerifySignatureShareResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) BuildTaprootTx(
	sessionID string,
	inputs []NativeTBTCSignerTxInput,
	outputs []NativeTBTCSignerTxOutput,
	scriptTreeHex *string,
) (*NativeTBTCSignerTxResult, error) {
	requestPayload, err := buildTaggedTBTCSignerBuildTaprootTxRequestPayload(
		sessionID,
		inputs,
		outputs,
		scriptTreeHex,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerBuildTaprootTx(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerBuildTaprootTxResponse(responsePayload)
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
		ErrNativeBridgeOperationFailed,
		operation,
		message,
	)
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

func buildTaggedTBTCSignerDKGPart1RequestPayload(
	participantIdentifier string,
	maxSigners uint16,
	minSigners uint16,
) ([]byte, error) {
	if participantIdentifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart1",
			"participant identifier is empty",
		)
	}
	if maxSigners == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart1",
			"max signers is zero",
		)
	}
	if minSigners == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart1",
			"min signers is zero",
		)
	}
	if minSigners > maxSigners {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart1",
			"min signers exceeds max signers",
		)
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"DKGPart1",
		buildTaggedTBTCSignerDKGPart1Request{
			ParticipantIdentifier: participantIdentifier,
			MaxSigners:            maxSigners,
			MinSigners:            minSigners,
		},
	)
}

func decodeBuildTaggedTBTCSignerDKGPart1Response(
	responsePayload []byte,
) (*NativeFROSTDKGPart1Result, error) {
	var response buildTaggedTBTCSignerDKGPart1Response
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart1",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	secretPackageData, err := buildTaggedTBTCSignerValidateHexBytesField(
		"DKGPart1",
		"response secret package",
		response.SecretPackageHex,
	)
	if err != nil {
		return nil, err
	}
	round1Package, err := decodeBuildTaggedTBTCSignerDKGRound1Package(
		"DKGPart1",
		"response package",
		response.Package,
	)
	if err != nil {
		return nil, err
	}

	return &NativeFROSTDKGPart1Result{
		SecretPackage: &NativeFROSTDKGRound1SecretPackage{
			Data: secretPackageData,
		},
		Package: round1Package,
	}, nil
}

func buildTaggedTBTCSignerDKGPart2RequestPayload(
	secretPackage *NativeFROSTDKGRound1SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
) ([]byte, error) {
	if secretPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart2",
			"secret package is nil",
		)
	}
	if len(secretPackage.Data) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart2",
			"secret package data is empty",
		)
	}

	requestPackages, err := buildTaggedTBTCSignerDKGRound1PackagePayloads(
		"DKGPart2",
		round1Packages,
	)
	if err != nil {
		return nil, err
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"DKGPart2",
		buildTaggedTBTCSignerDKGPart2Request{
			SecretPackageHex: hexBytes(secretPackage.Data),
			Round1Packages:   requestPackages,
		},
	)
}

func decodeBuildTaggedTBTCSignerDKGPart2Response(
	responsePayload []byte,
) (*NativeFROSTDKGPart2Result, error) {
	var response buildTaggedTBTCSignerDKGPart2Response
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart2",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	secretPackageData, err := buildTaggedTBTCSignerValidateHexBytesField(
		"DKGPart2",
		"response secret package",
		response.SecretPackageHex,
	)
	if err != nil {
		return nil, err
	}
	if len(response.Packages) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart2",
			"response packages are empty",
		)
	}

	packages := make([]*NativeFROSTDKGRound2Package, 0, len(response.Packages))
	for i := range response.Packages {
		pkg, err := decodeBuildTaggedTBTCSignerDKGRound2Package(
			"DKGPart2",
			fmt.Sprintf("response package [%d]", i),
			&response.Packages[i],
			false,
		)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}

	return &NativeFROSTDKGPart2Result{
		SecretPackage: &NativeFROSTDKGRound2SecretPackage{
			Data: secretPackageData,
		},
		Packages: packages,
	}, nil
}

func buildTaggedTBTCSignerDKGPart3RequestPayload(
	secretPackage *NativeFROSTDKGRound2SecretPackage,
	round1Packages []*NativeFROSTDKGRound1Package,
	round2Packages []*NativeFROSTDKGRound2Package,
) ([]byte, error) {
	if secretPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			"secret package is nil",
		)
	}
	if len(secretPackage.Data) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			"secret package data is empty",
		)
	}

	requestRound1Packages, err := buildTaggedTBTCSignerDKGRound1PackagePayloads(
		"DKGPart3",
		round1Packages,
	)
	if err != nil {
		return nil, err
	}
	requestRound2Packages, err := buildTaggedTBTCSignerDKGRound2PackagePayloads(
		"DKGPart3",
		round2Packages,
		true,
	)
	if err != nil {
		return nil, err
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"DKGPart3",
		buildTaggedTBTCSignerDKGPart3Request{
			SecretPackageHex: hexBytes(secretPackage.Data),
			Round1Packages:   requestRound1Packages,
			Round2Packages:   requestRound2Packages,
		},
	)
}

func buildTaggedTBTCSignerPersistDistributedDKGKeyPackageRequestPayload(
	sessionID string,
	participantIdentifier uint16,
	threshold uint16,
	participantCount uint16,
	keyPackage *NativeFROSTKeyPackage,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) ([]byte, error) {
	const op = "PersistDistributedDKGKeyPackage"
	if keyPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(op, "key package is nil")
	}
	if len(keyPackage.Data) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(op, "key package data is empty")
	}
	if publicKeyPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(op, "public key package is nil")
	}
	return buildTaggedTBTCSignerMarshalRequest(
		op,
		buildTaggedTBTCSignerPersistDistributedDKGKeyPackageRequest{
			SessionID:             sessionID,
			ParticipantIdentifier: participantIdentifier,
			Threshold:             threshold,
			ParticipantCount:      participantCount,
			KeyPackage: &buildTaggedTBTCSignerNativeFROSTKeyPackage{
				Identifier: keyPackage.Identifier,
				DataHex:    hexBytes(keyPackage.Data),
			},
			PublicKeyPackage: &buildTaggedTBTCSignerNativeFROSTPublicKeyPackage{
				VerifyingShares: publicKeyPackage.VerifyingShares,
				VerifyingKey:    publicKeyPackage.VerifyingKey,
			},
		},
	)
}

func buildTaggedTBTCSignerRetireDistributedDKGKeyPackagesRequestPayload(
	keyGroup string,
) ([]byte, error) {
	const op = "RetireDistributedDKGKeyPackages"
	if keyGroup == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"key group is empty",
		)
	}
	return buildTaggedTBTCSignerMarshalRequest(
		op,
		buildTaggedTBTCSignerRetireDistributedDKGKeyPackagesRequest{
			KeyGroup: keyGroup,
		},
	)
}

func buildTaggedTBTCSignerShareRepairSessionRequestPayload(
	op string,
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) ([]byte, error) {
	if _, err := ComputeShareRepairAuthorizationDigest(authorization); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	if participantIdentifier == 0 {
		return nil, buildTaggedTBTCSignerOperationError(op, "participant identifier is zero")
	}
	return buildTaggedTBTCSignerMarshalRequest(
		op,
		buildTaggedTBTCSignerShareRepairSessionRequest{
			Authorization:         authorization,
			ParticipantIdentifier: participantIdentifier,
		},
	)
}

func shareRepairContextWire(
	authorization *ShareRepairAuthorization,
) (string, error) {
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("0x%x", digest), nil
}

func decodeCanonicalShareRepairHex(
	op string,
	label string,
	value string,
	expectedLength int,
) ([]byte, error) {
	if len(value) != expectedLength*2 || strings.ToLower(value) != value {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			fmt.Sprintf("%s is not canonical lowercase %d-byte hex", label, expectedLength),
		)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			fmt.Sprintf("%s is invalid: %v", label, err),
		)
	}
	return decoded, nil
}

func validateBuildTaggedTBTCSignerShareRepairTransportRoster(
	op string,
	authorization *ShareRepairAuthorization,
	transportRoster *ShareRepairTransportRoster,
) error {
	if _, err := ComputeShareRepairTransportRosterDigest(
		transportRoster,
		authorization,
	); err != nil {
		return buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	if _, err := parseCanonicalShareRepairSignature(
		transportRoster.SignatureHex,
	); err != nil {
		return buildTaggedTBTCSignerOperationError(
			op,
			fmt.Sprintf("invalid transport roster signature encoding: %v", err),
		)
	}
	return nil
}

func buildTaggedTBTCSignerShareRepairPart1RequestPayload(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	transportRoster *ShareRepairTransportRoster,
) ([]byte, error) {
	const op = "ShareRepairPart1"
	if err := validateBuildTaggedTBTCSignerShareRepairTransportRoster(
		op,
		authorization,
		transportRoster,
	); err != nil {
		return nil, err
	}
	return buildTaggedTBTCSignerMarshalRequest(
		op,
		buildTaggedTBTCSignerShareRepairPart1Request{
			Authorization:    authorization,
			HelperIdentifier: helperIdentifier,
			TransportRoster:  transportRoster,
		},
	)
}

func buildTaggedTBTCSignerShareRepairDeltaPayloads(
	op string,
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	deltas []*NativeShareRepairEncryptedDelta,
) ([]buildTaggedTBTCSignerShareRepairEncryptedDelta, error) {
	if len(deltas) != len(authorization.HelperIdentifiers) {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"encrypted deltas do not contain the exact helper set",
		)
	}
	contextDigest, err := shareRepairContextWire(authorization)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	result := make([]buildTaggedTBTCSignerShareRepairEncryptedDelta, len(deltas))
	for index, delta := range deltas {
		if delta == nil || delta.ContextDigest != contextDigest ||
			delta.SenderIdentifier != authorization.HelperIdentifiers[index] ||
			delta.RecipientIdentifier != helperIdentifier ||
			len(delta.Payload) != buildTaggedTBTCSignerShareRepairPayloadLength {
			return nil, buildTaggedTBTCSignerOperationError(
				op,
				fmt.Sprintf("encrypted delta [%d] is invalid or out of order", index),
			)
		}
		result[index] = buildTaggedTBTCSignerShareRepairEncryptedDelta{
			ContextDigest:       delta.ContextDigest,
			SenderIdentifier:    delta.SenderIdentifier,
			RecipientIdentifier: delta.RecipientIdentifier,
			PayloadHex:          hex.EncodeToString(delta.Payload),
		}
	}
	return result, nil
}

func buildTaggedTBTCSignerShareRepairPart2RequestPayload(
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
	deltas []*NativeShareRepairEncryptedDelta,
	transportRoster *ShareRepairTransportRoster,
) ([]byte, error) {
	const op = "ShareRepairPart2"
	if err := validateBuildTaggedTBTCSignerShareRepairTransportRoster(
		op,
		authorization,
		transportRoster,
	); err != nil {
		return nil, err
	}
	wireDeltas, err := buildTaggedTBTCSignerShareRepairDeltaPayloads(
		op,
		authorization,
		helperIdentifier,
		deltas,
	)
	if err != nil {
		return nil, err
	}
	return buildTaggedTBTCSignerMarshalRequest(
		op,
		buildTaggedTBTCSignerShareRepairPart2Request{
			Authorization:    authorization,
			HelperIdentifier: helperIdentifier,
			Deltas:           wireDeltas,
			TransportRoster:  transportRoster,
		},
	)
}

func buildTaggedTBTCSignerShareRepairSigmaPayloads(
	op string,
	authorization *ShareRepairAuthorization,
	sigmas []*NativeShareRepairEncryptedSigma,
) ([]buildTaggedTBTCSignerShareRepairEncryptedSigma, error) {
	if len(sigmas) != len(authorization.HelperIdentifiers) {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"encrypted sigmas do not contain the exact helper set",
		)
	}
	contextDigest, err := shareRepairContextWire(authorization)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	result := make([]buildTaggedTBTCSignerShareRepairEncryptedSigma, len(sigmas))
	for index, sigma := range sigmas {
		if sigma == nil || sigma.ContextDigest != contextDigest ||
			sigma.HelperIdentifier != authorization.HelperIdentifiers[index] ||
			len(sigma.Payload) != buildTaggedTBTCSignerShareRepairPayloadLength {
			return nil, buildTaggedTBTCSignerOperationError(
				op,
				fmt.Sprintf("encrypted sigma [%d] is invalid or out of order", index),
			)
		}
		result[index] = buildTaggedTBTCSignerShareRepairEncryptedSigma{
			ContextDigest:    sigma.ContextDigest,
			HelperIdentifier: sigma.HelperIdentifier,
			PayloadHex:       hex.EncodeToString(sigma.Payload),
		}
	}
	return result, nil
}

func buildTaggedTBTCSignerInstallRepairedShareRequestPayload(
	authorization *ShareRepairAuthorization,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
	sigmas []*NativeShareRepairEncryptedSigma,
	transportRoster *ShareRepairTransportRoster,
) ([]byte, error) {
	const op = "InstallRepairedShare"
	if err := validateBuildTaggedTBTCSignerShareRepairTransportRoster(
		op,
		authorization,
		transportRoster,
	); err != nil {
		return nil, err
	}
	if publicKeyPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(op, "public key package is nil")
	}
	wireSigmas, err := buildTaggedTBTCSignerShareRepairSigmaPayloads(
		op,
		authorization,
		sigmas,
	)
	if err != nil {
		return nil, err
	}
	return buildTaggedTBTCSignerMarshalRequest(
		op,
		buildTaggedTBTCSignerInstallRepairedShareRequest{
			Authorization:   authorization,
			TransportRoster: transportRoster,
			PublicKeyPackage: &buildTaggedTBTCSignerNativeFROSTPublicKeyPackage{
				VerifyingShares: publicKeyPackage.VerifyingShares,
				VerifyingKey:    publicKeyPackage.VerifyingKey,
			},
			Sigmas: wireSigmas,
		},
	)
}

func decodeBuildTaggedTBTCSignerBeginShareRepairSessionResponse(
	responsePayload []byte,
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) (*NativeShareRepairSession, error) {
	const op = "BeginShareRepairSession"
	response := &buildTaggedTBTCSignerBeginShareRepairSessionResponse{}
	if err := json.Unmarshal(responsePayload, response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, fmt.Sprintf("cannot decode response: %v", err))
	}
	contextDigest, err := shareRepairContextWire(authorization)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	if response.ContextDigest != contextDigest ||
		response.ParticipantIdentifier != participantIdentifier {
		return nil, buildTaggedTBTCSignerOperationError(op, "response does not match the requested session")
	}
	if _, err := parseCanonicalShareRepairHex32(
		response.StoreFingerprint,
		"native share-repair session store_fingerprint",
	); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	publicKey, err := decodeCanonicalShareRepairHex(
		op,
		"transport_public_key_hex",
		response.TransportPublicKeyHex,
		buildTaggedTBTCSignerShareRepairPublicKeyLength,
	)
	if err != nil {
		return nil, err
	}
	return &NativeShareRepairSession{
		ContextDigest:         response.ContextDigest,
		ParticipantIdentifier: response.ParticipantIdentifier,
		StoreFingerprint:      response.StoreFingerprint,
		TransportPublicKey:    publicKey,
	}, nil
}

func decodeBuildTaggedTBTCSignerFinishShareRepairSessionResponse(
	responsePayload []byte,
	authorization *ShareRepairAuthorization,
	participantIdentifier uint16,
) error {
	const op = "FinishShareRepairSession"
	response := &buildTaggedTBTCSignerFinishShareRepairSessionResponse{}
	if err := json.Unmarshal(responsePayload, response); err != nil {
		return buildTaggedTBTCSignerOperationError(op, fmt.Sprintf("cannot decode response: %v", err))
	}
	contextDigest, err := shareRepairContextWire(authorization)
	if err != nil {
		return buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	if response.ContextDigest != contextDigest ||
		response.ParticipantIdentifier != participantIdentifier || !response.Finished {
		return buildTaggedTBTCSignerOperationError(op, "response does not confirm the requested session")
	}
	return nil
}

func decodeBuildTaggedTBTCSignerShareRepairPart1Response(
	responsePayload []byte,
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
) (*NativeShareRepairPart1Result, error) {
	const op = "ShareRepairPart1"
	response := &buildTaggedTBTCSignerShareRepairPart1Response{}
	if err := json.Unmarshal(responsePayload, response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, fmt.Sprintf("cannot decode response: %v", err))
	}
	contextDigest, err := shareRepairContextWire(authorization)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	if response.ContextDigest != contextDigest || response.HelperIdentifier != helperIdentifier ||
		response.PublicKeyPackage == nil ||
		len(response.PublicKeyPackage.VerifyingShares) == 0 ||
		response.PublicKeyPackage.VerifyingKey == "" ||
		len(response.Deltas) != len(authorization.HelperIdentifiers) {
		return nil, buildTaggedTBTCSignerOperationError(op, "response has the wrong context or shape")
	}
	deltas := make([]*NativeShareRepairEncryptedDelta, len(response.Deltas))
	for index, delta := range response.Deltas {
		if delta.ContextDigest != contextDigest || delta.SenderIdentifier != helperIdentifier ||
			delta.RecipientIdentifier != authorization.HelperIdentifiers[index] {
			return nil, buildTaggedTBTCSignerOperationError(op, fmt.Sprintf("invalid delta [%d] bindings", index))
		}
		payload, err := decodeCanonicalShareRepairHex(
			op,
			fmt.Sprintf("deltas[%d].payload_hex", index),
			delta.PayloadHex,
			buildTaggedTBTCSignerShareRepairPayloadLength,
		)
		if err != nil {
			return nil, err
		}
		deltas[index] = &NativeShareRepairEncryptedDelta{
			ContextDigest:       delta.ContextDigest,
			SenderIdentifier:    delta.SenderIdentifier,
			RecipientIdentifier: delta.RecipientIdentifier,
			Payload:             payload,
		}
	}
	return &NativeShareRepairPart1Result{
		ContextDigest:    response.ContextDigest,
		HelperIdentifier: response.HelperIdentifier,
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingShares: appendBuildTaggedTBTCSignerStringMap(
				response.PublicKeyPackage.VerifyingShares,
			),
			VerifyingKey: response.PublicKeyPackage.VerifyingKey,
		},
		Deltas: deltas,
	}, nil
}

func decodeBuildTaggedTBTCSignerShareRepairPart2Response(
	responsePayload []byte,
	authorization *ShareRepairAuthorization,
	helperIdentifier uint16,
) (*NativeShareRepairPart2Result, error) {
	const op = "ShareRepairPart2"
	response := &buildTaggedTBTCSignerShareRepairPart2Response{}
	if err := json.Unmarshal(responsePayload, response); err != nil || response.Sigma == nil {
		return nil, buildTaggedTBTCSignerOperationError(op, "cannot decode response sigma")
	}
	contextDigest, err := shareRepairContextWire(authorization)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, err.Error())
	}
	if response.ContextDigest != contextDigest || response.Sigma.ContextDigest != contextDigest ||
		response.Sigma.HelperIdentifier != helperIdentifier {
		return nil, buildTaggedTBTCSignerOperationError(op, "response sigma has invalid bindings")
	}
	payload, err := decodeCanonicalShareRepairHex(
		op,
		"sigma.payload_hex",
		response.Sigma.PayloadHex,
		buildTaggedTBTCSignerShareRepairPayloadLength,
	)
	if err != nil {
		return nil, err
	}
	return &NativeShareRepairPart2Result{
		ContextDigest: response.ContextDigest,
		Sigma: &NativeShareRepairEncryptedSigma{
			ContextDigest:    response.Sigma.ContextDigest,
			HelperIdentifier: response.Sigma.HelperIdentifier,
			Payload:          payload,
		},
	}, nil
}

func decodeBuildTaggedTBTCSignerInstallRepairedShareResponse(
	responsePayload []byte,
	authorization *ShareRepairAuthorization,
) (*NativeShareRepairInstallResult, error) {
	const op = "InstallRepairedShare"
	response := &buildTaggedTBTCSignerInstallRepairedShareResponse{}
	if err := json.Unmarshal(responsePayload, response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(op, fmt.Sprintf("cannot decode response: %v", err))
	}
	digest, err := ComputeShareRepairAuthorizationDigest(authorization)
	if err != nil {
		return nil, err
	}
	if response.Schema != ShareRepairInstallResultSchema ||
		response.SessionID != authorization.SessionID ||
		response.KeyGroup != authorization.KeyGroup ||
		response.TargetIdentifier != authorization.TargetIdentifier ||
		response.RecoveryEpoch != authorization.RecoveryEpoch ||
		response.AuthorizationDigest != fmt.Sprintf("0x%x", digest) ||
		response.ActiveStoreFingerprint != authorization.NewStoreFingerprint {
		return nil, buildTaggedTBTCSignerOperationError(op, "response does not match authorization")
	}
	return &NativeShareRepairInstallResult{
		Schema:                 response.Schema,
		SessionID:              response.SessionID,
		KeyGroup:               response.KeyGroup,
		TargetIdentifier:       response.TargetIdentifier,
		RecoveryEpoch:          response.RecoveryEpoch,
		AuthorizationDigest:    response.AuthorizationDigest,
		ActiveStoreFingerprint: response.ActiveStoreFingerprint,
		Idempotent:             response.Idempotent,
	}, nil
}

func decodeBuildTaggedTBTCSignerRetireDistributedDKGKeyPackagesResponse(
	responsePayload []byte,
	expectedKeyGroup string,
) error {
	const op = "RetireDistributedDKGKeyPackages"
	var response buildTaggedTBTCSignerRetireDistributedDKGKeyPackagesResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return buildTaggedTBTCSignerOperationError(
			op,
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}
	if response.KeyGroup != expectedKeyGroup {
		return buildTaggedTBTCSignerOperationError(
			op,
			"response key group does not match request",
		)
	}
	if response.Retired != (response.RetiredKeyPackageCount > 0) {
		return buildTaggedTBTCSignerOperationError(
			op,
			"response retirement status and key-package count disagree",
		)
	}
	return nil
}

func buildTaggedTBTCSignerTriggerEmergencyRekeyRequestPayload(
	sessionID string,
	reason string,
) ([]byte, string, error) {
	const op = "TriggerEmergencyRekey"
	if sessionID == "" {
		return nil, "", buildTaggedTBTCSignerOperationError(op, "session ID is empty")
	}
	// The engine validates the session ID too, but rejecting here keeps a
	// malformed operator input from costing a barrier lease and an anchor round
	// trip before it fails.
	if len(sessionID) > maximumTBTCSignerEmergencyRekeySessionIDLength {
		return nil, "", buildTaggedTBTCSignerOperationError(
			op,
			"session ID is too long",
		)
	}
	if strings.ContainsAny(sessionID, "\x00\n\r\t \"\\=") {
		return nil, "", buildTaggedTBTCSignerOperationError(
			op,
			"session ID contains disallowed characters",
		)
	}
	// Trim before the emptiness check so a whitespace-only reason cannot arm the
	// switch with no recorded justification; the engine trims identically, and
	// the response echo is compared against the trimmed value. Return the
	// trimmed reason so the caller can use the same normalization for the
	// byte-for-byte echo comparison instead of re-deriving it independently.
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return nil, "", buildTaggedTBTCSignerOperationError(op, "reason is empty")
	}
	payload, err := buildTaggedTBTCSignerMarshalRequest(
		op,
		buildTaggedTBTCSignerTriggerEmergencyRekeyRequest{
			SessionID: sessionID,
			Reason:    trimmedReason,
		},
	)
	if err != nil {
		return nil, "", err
	}
	return payload, trimmedReason, nil
}

func decodeBuildTaggedTBTCSignerTriggerEmergencyRekeyResponse(
	responsePayload []byte,
	expectedReason string,
) (*NativeTBTCSignerEmergencyRekey, error) {
	const op = "TriggerEmergencyRekey"
	var response buildTaggedTBTCSignerTriggerEmergencyRekeyResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}
	// The engine reports false only if it declined to arm the switch, which it
	// signals as an error; treat a false here as a contract violation rather
	// than as a quiet no-op, so a caller never reads "rekey done" from it.
	if !response.EmergencyRekeyRequired {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"response does not report the emergency rekey as required",
		)
	}
	if response.Reason != expectedReason {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"response reason does not match request",
		)
	}
	// The engine may retarget a per-signing session to the wallet session it
	// serves, so the echoed session ID legitimately differs from the request.
	// It must still name some session.
	if response.SessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"response session ID is empty",
		)
	}
	if response.TriggeredAtUnix == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"response trigger timestamp is zero",
		)
	}
	if response.RecommendedNewSessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			op,
			"response recommended new session ID is empty",
		)
	}
	return &NativeTBTCSignerEmergencyRekey{
		SessionID:               response.SessionID,
		Reason:                  response.Reason,
		TriggeredAtUnix:         response.TriggeredAtUnix,
		RecommendedNewSessionID: response.RecommendedNewSessionID,
	}, nil
}

func decodeBuildTaggedTBTCSignerDKGPart3Response(
	responsePayload []byte,
) (*NativeFROSTDKGResult, error) {
	var response buildTaggedTBTCSignerDKGPart3Response
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}
	if response.KeyPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			"response key package is nil",
		)
	}
	if response.KeyPackage.Identifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			"response key package identifier is empty",
		)
	}
	keyPackageData, err := buildTaggedTBTCSignerValidateHexBytesField(
		"DKGPart3",
		"response key package data",
		response.KeyPackage.DataHex,
	)
	if err != nil {
		return nil, err
	}
	if response.PublicKeyPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			"response public key package is nil",
		)
	}
	if response.PublicKeyPackage.VerifyingKey == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			"response public key package verifying key is empty",
		)
	}
	if len(response.PublicKeyPackage.VerifyingShares) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"DKGPart3",
			"response public key package verifying shares are empty",
		)
	}

	return &NativeFROSTDKGResult{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: response.KeyPackage.Identifier,
			Data:       keyPackageData,
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingShares: appendBuildTaggedTBTCSignerStringMap(
				response.PublicKeyPackage.VerifyingShares,
			),
			VerifyingKey: response.PublicKeyPackage.VerifyingKey,
		},
	}, nil
}

func buildTaggedTBTCSignerNewSigningPackageRequestPayload(
	message []byte,
	commitments []nativeFROSTCommitment,
) ([]byte, error) {
	if len(commitments) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"NewSigningPackage",
			"commitments are empty",
		)
	}

	requestCommitments := make(
		[]buildTaggedTBTCSignerNativeFROSTCommitment,
		0,
		len(commitments),
	)
	for i, commitment := range commitments {
		if commitment.Identifier == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"NewSigningPackage",
				fmt.Sprintf("commitment [%d] identifier is empty", i),
			)
		}
		if len(commitment.Data) == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"NewSigningPackage",
				fmt.Sprintf("commitment [%d] data is empty", i),
			)
		}
		requestCommitments = append(
			requestCommitments,
			buildTaggedTBTCSignerNativeFROSTCommitment{
				Identifier: commitment.Identifier,
				DataHex:    hex.EncodeToString(commitment.Data),
			},
		)
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"NewSigningPackage",
		buildTaggedTBTCSignerNewSigningPackageRequest{
			MessageHex:  hex.EncodeToString(message),
			Commitments: requestCommitments,
		},
	)
}

func decodeBuildTaggedTBTCSignerNewSigningPackageResponse(
	responsePayload []byte,
) ([]byte, error) {
	var response buildTaggedTBTCSignerNewSigningPackageResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"NewSigningPackage",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	return buildTaggedTBTCSignerDecodeHexField(
		"NewSigningPackage",
		"response signing package",
		response.SigningPackageHex,
	)
}

// buildTaggedTBTCSignerVerifySignatureShareRequestPayload builds the
// VerifySignatureShare request.
//
// Unlike every other bridge operation, it deliberately does NOT reject empty or
// short signing-package / signature-share bytes. For THIS operation those bytes
// are the SUBJECT of the engine's tri-state verdict: a member's retained share
// envelope can carry empty or malformed inner FROST bytes (the collector
// authenticates the operator signature over the envelope, not the FROST share
// equation), and the engine classifies such bytes as an `invalid` (blamable)
// verdict. If the bridge instead rejected them with an error here, the Go host
// would map that FFI error to ShareIndeterminate and a cheater who submitted
// garbage would dodge blame. So only the sessionID routing key is validated;
// the package, share, and member identifier are passed through for the engine
// to classify (a malformed package or out-of-range id yields `indeterminate`,
// never false blame).
func buildTaggedTBTCSignerVerifySignatureShareRequestPayload(
	sessionID string,
	signingPackage []byte,
	signatureShare []byte,
	memberIdentifier uint16,
	taprootMerkleRoot *[32]byte,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"VerifySignatureShare",
			"session ID is empty",
		)
	}

	var taprootMerkleRootHex *string
	if taprootMerkleRoot != nil {
		encoded := hex.EncodeToString(taprootMerkleRoot[:])
		taprootMerkleRootHex = &encoded
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"VerifySignatureShare",
		buildTaggedTBTCSignerVerifySignatureShareRequest{
			SessionID:            sessionID,
			SigningPackageHex:    hex.EncodeToString(signingPackage),
			SignatureShareHex:    hex.EncodeToString(signatureShare),
			MemberIdentifier:     memberIdentifier,
			TaprootMerkleRootHex: taprootMerkleRootHex,
		},
	)
}

// decodeBuildTaggedTBTCSignerVerifySignatureShareResponse maps the engine's
// snake_case verdict string to the typed tri-state. An unrecognized verdict is
// an error (never silently defaulted); the zero value of the verdict type is the
// safe Indeterminate, so an unchecked error never reads as blame.
func decodeBuildTaggedTBTCSignerVerifySignatureShareResponse(
	responsePayload []byte,
) (NativeShareVerificationVerdict, error) {
	var response buildTaggedTBTCSignerVerifySignatureShareResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return NativeShareVerdictIndeterminate, buildTaggedTBTCSignerOperationError(
			"VerifySignatureShare",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	switch response.Verdict {
	case "valid":
		return NativeShareVerdictValid, nil
	case "invalid":
		return NativeShareVerdictInvalid, nil
	case "indeterminate":
		return NativeShareVerdictIndeterminate, nil
	default:
		return NativeShareVerdictIndeterminate, buildTaggedTBTCSignerOperationError(
			"VerifySignatureShare",
			fmt.Sprintf("response verdict is unrecognized: %q", response.Verdict),
		)
	}
}

func buildTaggedTBTCSignerMarshalRequest(
	operation string,
	request interface{},
) ([]byte, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("cannot marshal request: %v", err),
		)
	}

	return payload, nil
}

func buildTaggedTBTCSignerDecodeHexField(
	operation string,
	fieldName string,
	value string,
) ([]byte, error) {
	if value == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s is empty", fieldName),
		)
	}

	data, err := hex.DecodeString(value)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s is invalid hex: %v", fieldName, err),
		)
	}
	if len(data) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s decoded to empty bytes", fieldName),
		)
	}

	return data, nil
}

// buildTaggedTBTCSignerValidateHexBytesField is the hexBytes counterpart of
// buildTaggedTBTCSignerDecodeHexField. Fields carrying secret material decode
// during json.Unmarshal (see hexBytes) so the plaintext never passes through a Go
// string, which leaves only the emptiness check to perform here. Malformed hex is
// already rejected by hexBytes.UnmarshalJSON.
func buildTaggedTBTCSignerValidateHexBytesField(
	operation string,
	fieldName string,
	value hexBytes,
) ([]byte, error) {
	if len(value) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s is empty", fieldName),
		)
	}

	return value, nil
}

func buildTaggedTBTCSignerDKGRound1PackagePayloads(
	operation string,
	packages []*NativeFROSTDKGRound1Package,
) ([]buildTaggedTBTCSignerDKGRound1Package, error) {
	if len(packages) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			"round-one packages are empty",
		)
	}

	payloads := make(
		[]buildTaggedTBTCSignerDKGRound1Package,
		0,
		len(packages),
	)
	for i, pkg := range packages {
		if pkg == nil {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf("round-one package [%d] is nil", i),
			)
		}
		if pkg.Identifier == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf("round-one package [%d] identifier is empty", i),
			)
		}
		if len(pkg.Data) == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf("round-one package [%d] data is empty", i),
			)
		}
		payloads = append(payloads, buildTaggedTBTCSignerDKGRound1Package{
			Identifier: pkg.Identifier,
			PackageHex: hex.EncodeToString(pkg.Data),
		})
	}

	return payloads, nil
}

func buildTaggedTBTCSignerDKGRound2PackagePayloads(
	operation string,
	packages []*NativeFROSTDKGRound2Package,
	requireSenderIdentifier bool,
) ([]buildTaggedTBTCSignerDKGRound2Package, error) {
	if len(packages) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			"round-two packages are empty",
		)
	}

	payloads := make(
		[]buildTaggedTBTCSignerDKGRound2Package,
		0,
		len(packages),
	)
	for i, pkg := range packages {
		if pkg == nil {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf("round-two package [%d] is nil", i),
			)
		}
		if pkg.Identifier == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf("round-two package [%d] identifier is empty", i),
			)
		}
		if requireSenderIdentifier && pkg.SenderIdentifier == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf("round-two package [%d] sender identifier is empty", i),
			)
		}
		if len(pkg.Data) == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf("round-two package [%d] data is empty", i),
			)
		}

		var senderIdentifier *string
		if pkg.SenderIdentifier != "" {
			copied := pkg.SenderIdentifier
			senderIdentifier = &copied
		}
		payloads = append(payloads, buildTaggedTBTCSignerDKGRound2Package{
			Identifier:       pkg.Identifier,
			SenderIdentifier: senderIdentifier,
			PackageHex:       hex.EncodeToString(pkg.Data),
		})
	}

	return payloads, nil
}

func decodeBuildTaggedTBTCSignerDKGRound1Package(
	operation string,
	fieldName string,
	pkg *buildTaggedTBTCSignerDKGRound1Package,
) (*NativeFROSTDKGRound1Package, error) {
	if pkg == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s is nil", fieldName),
		)
	}
	if pkg.Identifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s identifier is empty", fieldName),
		)
	}
	data, err := buildTaggedTBTCSignerDecodeHexField(
		operation,
		fmt.Sprintf("%s data", fieldName),
		pkg.PackageHex,
	)
	if err != nil {
		return nil, err
	}

	return &NativeFROSTDKGRound1Package{
		Identifier: pkg.Identifier,
		Data:       data,
	}, nil
}

func decodeBuildTaggedTBTCSignerDKGRound2Package(
	operation string,
	fieldName string,
	pkg *buildTaggedTBTCSignerDKGRound2Package,
	requireSenderIdentifier bool,
) (*NativeFROSTDKGRound2Package, error) {
	if pkg == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s is nil", fieldName),
		)
	}
	if pkg.Identifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s identifier is empty", fieldName),
		)
	}
	if requireSenderIdentifier && (pkg.SenderIdentifier == nil || *pkg.SenderIdentifier == "") {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s sender identifier is empty", fieldName),
		)
	}
	data, err := buildTaggedTBTCSignerDecodeHexField(
		operation,
		fmt.Sprintf("%s data", fieldName),
		pkg.PackageHex,
	)
	if err != nil {
		return nil, err
	}

	senderIdentifier := ""
	if pkg.SenderIdentifier != nil {
		senderIdentifier = *pkg.SenderIdentifier
	}

	return &NativeFROSTDKGRound2Package{
		Identifier:       pkg.Identifier,
		SenderIdentifier: senderIdentifier,
		Data:             data,
	}, nil
}

func appendBuildTaggedTBTCSignerStringMap(
	source map[string]string,
) map[string]string {
	if source == nil {
		return nil
	}

	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}

	return copy
}

func buildTaggedTBTCSignerBuildTaprootTxRequestPayload(
	sessionID string,
	inputs []NativeTBTCSignerTxInput,
	outputs []NativeTBTCSignerTxOutput,
	scriptTreeHex *string,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			"session ID is empty",
		)
	}

	if len(inputs) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			"inputs are empty",
		)
	}

	if len(outputs) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			"outputs are empty",
		)
	}

	requestInputs := make(
		[]buildTaggedTBTCSignerBuildTaprootTxInput,
		0,
		len(inputs),
	)
	for i, input := range inputs {
		if input.TxIDHex == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"BuildTaprootTx",
				fmt.Sprintf("input [%d] txid hex is empty", i),
			)
		}
		if input.ScriptPubKeyHex == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"BuildTaprootTx",
				fmt.Sprintf("input [%d] script pubkey hex is empty", i),
			)
		}

		requestInputs = append(
			requestInputs,
			buildTaggedTBTCSignerBuildTaprootTxInput{
				TxIDHex:         input.TxIDHex,
				Vout:            input.Vout,
				ValueSats:       input.ValueSats,
				ScriptPubKeyHex: input.ScriptPubKeyHex,
			},
		)
	}

	requestOutputs := make(
		[]buildTaggedTBTCSignerBuildTaprootTxOutput,
		0,
		len(outputs),
	)
	for i, output := range outputs {
		if output.ScriptPubKeyHex == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"BuildTaprootTx",
				fmt.Sprintf("output [%d] script pubkey hex is empty", i),
			)
		}

		requestOutputs = append(
			requestOutputs,
			buildTaggedTBTCSignerBuildTaprootTxOutput{
				ScriptPubKeyHex: output.ScriptPubKeyHex,
				ValueSats:       output.ValueSats,
			},
		)
	}

	var requestScriptTreeHex *string
	if scriptTreeHex != nil {
		if *scriptTreeHex == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"BuildTaprootTx",
				"script tree hex is empty",
			)
		}

		copied := *scriptTreeHex
		requestScriptTreeHex = &copied
	}

	request := buildTaggedTBTCSignerBuildTaprootTxRequest{
		SessionID:     sessionID,
		Inputs:        requestInputs,
		Outputs:       requestOutputs,
		ScriptTreeHex: requestScriptTreeHex,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			fmt.Sprintf("cannot marshal request: %v", err),
		)
	}

	return payload, nil
}

func decodeBuildTaggedTBTCSignerBuildTaprootTxResponse(
	responsePayload []byte,
) (*NativeTBTCSignerTxResult, error) {
	var response buildTaggedTBTCSignerBuildTaprootTxResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	if response.SessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			"response session ID is empty",
		)
	}

	if response.TxHex == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			"response tx hex is empty",
		)
	}

	if _, err := hex.DecodeString(response.TxHex); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			fmt.Sprintf("response tx hex is invalid: %v", err),
		)
	}
	if len(response.TaprootKeySpendSighashesHex) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"BuildTaprootTx",
			"response taproot key-spend sighashes are empty",
		)
	}
	for i, sighashHex := range response.TaprootKeySpendSighashesHex {
		sighash, err := hex.DecodeString(sighashHex)
		if err != nil || len(sighash) != 32 {
			return nil, buildTaggedTBTCSignerOperationError(
				"BuildTaprootTx",
				fmt.Sprintf("response taproot key-spend sighash [%d] is not 32-byte hex", i),
			)
		}
	}

	return &NativeTBTCSignerTxResult{
		SessionID:                   response.SessionID,
		TxHex:                       response.TxHex,
		TaprootKeySpendSighashesHex: append([]string(nil), response.TaprootKeySpendSighashesHex...),
	}, nil
}

func callBuildTaggedTBTCSignerVersion() ([]byte, error) {
	if err := ensureTBTCSignerFreeBufferAvailable(); err != nil {
		return nil, err
	}
	result := C.tbtc_signer_version()
	return parseBuildTaggedTBTCSignerResult("Version", result)
}

// callBuildTaggedTBTCSignerABIVersion fetches the structured FFI contract version. A
// missing frost_tbtc_abi_version symbol surfaces as ErrNativeCryptographyUnavailable
// (the lib predates ABI versioning), which the ABI preflight turns into an explicit
// incompatibility. It deliberately does NOT pass through callBuildTaggedTBTCSignerOperation
// (it takes no request and must not recurse into the ABI gate).
func callBuildTaggedTBTCSignerABIVersion() ([]byte, error) {
	if err := ensureTBTCSignerFreeBufferAvailable(); err != nil {
		return nil, err
	}
	result := C.tbtc_signer_abi_version()
	return parseBuildTaggedTBTCSignerResult("ABIVersion", result)
}

func callBuildTaggedTBTCSignerDKGPart1(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"DKGPart1",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_dkg_part1(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerDKGPart2(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"DKGPart2",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_dkg_part2(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerDKGPart3(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"DKGPart3",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_dkg_part3(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerPersistDistributedDKGKeyPackage(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"PersistDistributedDKGKeyPackage",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_persist_distributed_dkg_key_package(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerRetireDistributedDKGKeyPackages(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"RetireDistributedDKGKeyPackages",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_retire_distributed_dkg_key_packages(
				requestPtr,
				requestLen,
			)
		},
	)
}

func callBuildTaggedTBTCSignerBeginShareRepairSession(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"BeginShareRepairSession",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_begin_share_repair_session(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerFinishShareRepairSession(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"FinishShareRepairSession",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_finish_share_repair_session(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerShareRepairPart1(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"ShareRepairPart1",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_share_repair_part1(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerShareRepairPart2(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"ShareRepairPart2",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_share_repair_part2(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerInstallRepairedShare(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"InstallRepairedShare",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_install_repaired_share(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerNewSigningPackage(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"NewSigningPackage",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_new_signing_package(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerVerifySignatureShare(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"VerifySignatureShare",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_verify_signature_share(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerBuildTaprootTx(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"BuildTaprootTx",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_build_taproot_tx(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerTriggerEmergencyRekey(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"TriggerEmergencyRekey",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_trigger_emergency_rekey(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerOperation(
	operation string,
	requestPayload []byte,
	call func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult,
) ([]byte, error) {
	// ABI preflight (once per process): every request-taking engine operation funnels
	// through here, so a libfrost_tbtc whose FFI contract version is incompatible with
	// this bridge fails CLOSED before any contract-sensitive call rather than risking a
	// silently misinterpreted struct/JSON contract. The no-arg version/abi-version
	// calls bypass this helper, so the check does not recurse.
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}
	if len(requestPayload) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			"request payload is empty",
		)
	}

	var result C.TbtcSignerResult
	return executeNativeTBTCSignerStateAnchoredOutput(
		operation,
		func() {
			requestPtr := C.CBytes(requestPayload)
			requestLen := len(requestPayload)
			defer func() {
				// Scrub secret request bytes immediately after the native call.
				zeroBytes(unsafe.Slice((*byte)(requestPtr), requestLen))
				C.free(requestPtr)
			}()
			result = call(
				(*C.uint8_t)(requestPtr),
				C.size_t(requestLen),
			)
		},
		func() ([]byte, error) {
			return parseBuildTaggedTBTCSignerResult(operation, result)
		},
		func() {
			discardBuildTaggedTBTCSignerResult(result)
		},
	)
}

func discardBuildTaggedTBTCSignerResult(result C.TbtcSignerResult) {
	if result.buffer.ptr != nil {
		C.tbtc_signer_scrub_and_free_buffer(result.buffer.ptr, result.buffer.len)
	}
}

func ensureTBTCSignerFreeBufferAvailable() error {
	if C.tbtc_signer_free_buffer_available() == 0 {
		return fmt.Errorf(
			"%w: tbtc-signer buffer release symbol is unavailable",
			ErrNativeCryptographyUnavailable,
		)
	}
	return nil
}

func buildTaggedTBTCSignerShareRepairSymbolsAvailable() bool {
	return C.tbtc_signer_share_repair_symbols_available() != 0
}

func parseBuildTaggedTBTCSignerResult(
	operation string,
	result C.TbtcSignerResult,
) ([]byte, error) {
	// The C wrapper guards against a missing `frost_tbtc_free_buffer` symbol
	// but not against a NULL buffer pointer. Status code -1 paths (FFI lib
	// unavailable) and any future path that returns an empty buffer can leave
	// `result.buffer.ptr == nil`, so skip the deferred free in that case to
	// avoid handing a NULL pointer to Rust's `frost_tbtc_free_buffer`.
	if result.buffer.ptr != nil {
		defer C.tbtc_signer_scrub_and_free_buffer(
			result.buffer.ptr,
			result.buffer.len,
		)
	}

	statusCode := int32(result.status_code)

	var payload []byte
	if result.buffer.ptr != nil && result.buffer.len > 0 {
		// Guard the size_t -> C.int narrowing in C.GoBytes: a length that does
		// not fit in a C.int (>= 2^31) would overflow to a negative value and
		// panic ("length out of range") at the cgo boundary, or silently
		// truncate to a wrong length. A response that large is never valid, so
		// reject it; the buffer is still released by the deferred free above.
		if uint64(result.buffer.len) > uint64(math.MaxInt32) {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf(
					"response buffer length [%d] exceeds maximum [%d]",
					uint64(result.buffer.len), math.MaxInt32,
				),
			)
		}
		payload = C.GoBytes(unsafe.Pointer(result.buffer.ptr), C.int(result.buffer.len))
	}

	statusErr := buildTaggedTBTCSignerResultStatusError(operation, statusCode, payload)
	if statusErr != nil {
		return nil, statusErr
	}

	if len(payload) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			"response payload is empty",
		)
	}

	return payload, nil
}

func buildTaggedTBTCSignerResultStatusError(
	operation string,
	statusCode int32,
	payload []byte,
) error {
	if statusCode == buildTaggedTBTCSignerUnavailableStatusCode {
		return buildTaggedTBTCSignerUnavailableError(operation)
	}

	if statusCode != 0 {
		structured := buildTaggedTBTCSignerErrorPayload(payload)
		return fmt.Errorf(
			"%w: tbtc-signer bridge operation [%v] failed: [%w]",
			ErrNativeBridgeOperationFailed,
			operation,
			structured,
		)
	}

	return nil
}

func callBuildTaggedTBTCSignerInitSignerConfig(
	requestPayload []byte,
) ([]byte, error) {
	// This dedicated helper is the sole compile-time bootstrap exception to the
	// process-global state anchor. InitSignerConfig must open/lock the durable
	// store before its tip can be reconciled, and the Rust ABI guarantees an
	// initial or config-identical install neither mutates signer state nor emits
	// protocol material.
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}
	if len(requestPayload) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"InitSignerConfig",
			"request payload is empty",
		)
	}
	requestPtr := C.CBytes(requestPayload)
	requestLen := len(requestPayload)
	defer func() {
		zeroBytes(unsafe.Slice((*byte)(requestPtr), requestLen))
		C.free(requestPtr)
	}()
	result := C.tbtc_signer_init_signer_config(
		(*C.uint8_t)(requestPtr),
		C.size_t(requestLen),
	)
	return parseBuildTaggedTBTCSignerResult("InitSignerConfig", result)
}

// InstallNativeTBTCSignerConfig installs the tbtc-signer's init-time
// operational configuration via frost_tbtc_init_signer_config. configJSON is
// passed through verbatim: the Rust signer owns the schema and validation
// (unknown fields rejected, enforcement-gated policy combinations validated
// at install, environment ignored wholesale once installed) and the install
// is idempotent for an identical payload while a conflicting re-install is
// rejected. Must be called before the first state-touching signer operation.
// Returns an ErrNativeCryptographyUnavailable-classed error when the loaded
// signer library predates the symbol.
func InstallNativeTBTCSignerConfig(
	configJSON []byte,
) (*NativeTBTCSignerInitConfigResult, error) {
	responsePayload, err := callBuildTaggedTBTCSignerInitSignerConfig(configJSON)
	if err != nil {
		return nil, err
	}

	result := &NativeTBTCSignerInitConfigResult{}
	if err := json.Unmarshal(responsePayload, result); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"InitSignerConfig",
			fmt.Sprintf("response decode failed: [%v]", err),
		)
	}

	return result, nil
}

// ReadNativeTBTCSignerDurableStoreIdentity asks the linked signer for the
// identity of the store it has actually opened and locked. A stale library
// without frost_tbtc_durable_store_identity fails closed; config JSON is not a
// substitute for this runtime readback.
func ReadNativeTBTCSignerDurableStoreIdentity() (
	*NativeTBTCSignerDurableStoreIdentity,
	error,
) {
	if err := ensureTBTCSignerABICompatible(); err != nil {
		return nil, err
	}

	responsePayload, err := parseBuildTaggedTBTCSignerResult(
		"DurableStoreIdentity",
		C.tbtc_signer_durable_store_identity(),
	)
	if err != nil {
		return nil, err
	}

	identity, err := DecodeNativeTBTCSignerDurableStoreIdentity(responsePayload)
	if err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"DurableStoreIdentity",
			err.Error(),
		)
	}
	return identity, nil
}

// ErrNativeTBTCSignerEmergencyRekeyArmedButUnverified is returned by
// TriggerNativeTBTCSignerEmergencyRekey when the underlying engine call
// returned without error but the response could not be decoded into a
// verified rekey record. At that point the engine has already armed the
// emergency-rekey kill switch inside invoke() - the failure (a JSON decode
// error, a missing expected field, or any of the contract checks in
// decodeBuildTaggedTBTCSignerTriggerEmergencyRekeyResponse) reflects only
// that the Go host could not read back a fully verified acknowledgement,
// not that the switch itself is still unprimed.
//
// Callers that observe this error must treat the switch as ARMED. Retrying
// the trigger is useless: the engine treats an armed event as immutable and
// a second commit is a no-op that only wastes barrier time. Use
// ReadNativeTBTCSignerDurableStoreIdentity or a direct readback against the
// engine to confirm durable state before any operator decision.
var ErrNativeTBTCSignerEmergencyRekeyArmedButUnverified = errors.New(
	"native tbtc signer emergency rekey: switch armed but response unverifiable",
)

// TriggerNativeTBTCSignerEmergencyRekey arms the wallet-level emergency rekey
// kill switch through the state-anchored operation path, so a call that
// returns successfully has compare-and-swapped the durable write onto the
// anchor stream before returning.
//
// That pairing is not atomic, and the failure path is the interesting one. The
// engine arms the switch inside invoke(); the anchor only witnesses it in the
// lease.commit() that follows. A commit refusal - or a crash in that window -
// therefore returns an error from a call whose durable write already landed,
// leaving the switch armed locally but unwitnessed by the anchor. Nothing
// unwinds it: the discard path scrubs the Rust result buffer, not the engine's
// durable state. Recovery is not automatic either. It depends on the local
// store still being ahead of the anchor at the next boot, so reconciliation
// observes the armed event and re-anchors it - which is exactly the evidence an
// operator restoring a pre-rekey state file in that window destroys.
//
// Routing matters more than the call itself. The engine export has always
// existed, but with no Go caller the only way to arm the switch was to stop the
// node and mutate its store out of band - an uncertified local write that a
// restart cannot distinguish from an operator restoring the pre-rekey state
// file. Going through callBuildTaggedTBTCSignerOperation makes the write
// externally witnessed, so erasing it afterwards is detectable.
//
// Deliberately NOT admission-gated: callBuildTaggedTBTCSignerOperation performs
// no capacity reservation, and admission refuses all work once headroom reaches
// the rotation floor while the barrier keeps admitting until the certified
// window is genuinely exhausted. A kill switch that capacity accounting can
// veto is not a kill switch, so it runs in that band unreserved.
//
// The barrier, by contrast, stays mandatory. When it refuses - a poisoned
// anchor, or an exhausted certified window - this call fails. That is correct
// rather than a gap: every barrier refusal predicate is operation-independent,
// so a state in which this trigger is refused is a state in which the node is
// already refusing every signature-producing call. The kill switch is redundant
// there, not defeated, and its residual is availability, never authority.
//
// The call is serialized behind any in-flight anchored operation and may wait
// tens of seconds on the process-global barrier mutex; it is never refused for
// that reason. Callers must be single-flight - the engine treats an armed event
// as immutable, and no export clears it.

// When decodeBuildTaggedTBTCSignerTriggerEmergencyRekeyResponse fails on a
// payload the engine has already produced, this wraps the underlying error
// in ErrNativeTBTCSignerEmergencyRekeyArmedButUnverified so callers can
// distinguish "the switch was never armed" from "the switch was armed but
// the response was not verifiable". The wrapped error is terminal: do not
// retry the trigger, since the engine treats an armed event as immutable
// (see the single-flight paragraph above).
func TriggerNativeTBTCSignerEmergencyRekey(
	sessionID string,
	reason string,
) (*NativeTBTCSignerEmergencyRekey, error) {
	requestPayload, normalizedReason, err := buildTaggedTBTCSignerTriggerEmergencyRekeyRequestPayload(
		sessionID,
		reason,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerTriggerEmergencyRekey(
		requestPayload,
	)
	if err != nil {
		return nil, err
	}

	result, err := decodeBuildTaggedTBTCSignerTriggerEmergencyRekeyResponse(
		responsePayload,
		normalizedReason,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: [%w]",
			ErrNativeTBTCSignerEmergencyRekeyArmedButUnverified,
			err,
		)
	}
	return result, nil
}

// ----------------------------------------------------------------------------
// Phase 7.3 interactive signing session bridge: open / round1 / round2 / abort.
//
// The hardened interactive path - unlike the stateless nonce contract, secret
// nonces NEVER cross this boundary: the engine generates, holds, consumes, and
// zeroizes them keyed by (session_id, attempt_id). The caller exchanges only
// public commitments, the coordinator's signing package, and signature shares.
// Additive: no Go caller yet (the orchestrator adopts these in a later
// increment). interactive_aggregate, whose failure path surfaces candidate
// culprits, lands in a separate PR with its own structured error.
// ----------------------------------------------------------------------------

type buildTaggedTBTCSignerInteractiveAttemptContext struct {
	AttemptNumber                   uint32   `json:"attempt_number"`
	CoordinatorIdentifier           uint16   `json:"coordinator_identifier"`
	IncludedParticipants            []uint16 `json:"included_participants"`
	IncludedParticipantsFingerprint string   `json:"included_participants_fingerprint"`
	AttemptID                       string   `json:"attempt_id"`
}

type buildTaggedTBTCSignerInteractiveSessionOpenRequest struct {
	SessionID            string                                         `json:"session_id"`
	MemberIdentifier     uint16                                         `json:"member_identifier"`
	MessageHex           string                                         `json:"message_hex"`
	KeyGroup             string                                         `json:"key_group"`
	Threshold            uint16                                         `json:"threshold"`
	TaprootMerkleRootHex *string                                        `json:"taproot_merkle_root_hex,omitempty"`
	SigningIntent        *buildTaggedTBTCSignerSigningIntent            `json:"signing_intent,omitempty"`
	AttemptContext       buildTaggedTBTCSignerInteractiveAttemptContext `json:"attempt_context"`
}

type buildTaggedTBTCSignerSigningIntent struct {
	Type       string `json:"type"`
	MessageHex string `json:"message_hex"`
}

type buildTaggedTBTCSignerInteractiveSessionOpenResponse struct {
	SessionID  string `json:"session_id"`
	AttemptID  string `json:"attempt_id"`
	Idempotent bool   `json:"idempotent"`
}

type buildTaggedTBTCSignerInteractiveRound1Request struct {
	SessionID        string `json:"session_id"`
	AttemptID        string `json:"attempt_id"`
	MemberIdentifier uint16 `json:"member_identifier"`
}

type buildTaggedTBTCSignerInteractiveRound1Response struct {
	CommitmentsHex string `json:"commitments_hex"`
}

type buildTaggedTBTCSignerInteractiveRound2Request struct {
	SessionID         string `json:"session_id"`
	AttemptID         string `json:"attempt_id"`
	MemberIdentifier  uint16 `json:"member_identifier"`
	SigningPackageHex string `json:"signing_package_hex"`
}

type buildTaggedTBTCSignerInteractiveRound2Response struct {
	SessionID         string `json:"session_id"`
	AttemptID         string `json:"attempt_id"`
	SignatureShareHex string `json:"signature_share_hex"`
}

type buildTaggedTBTCSignerInteractiveSessionAbortRequest struct {
	SessionID string  `json:"session_id"`
	AttemptID *string `json:"attempt_id,omitempty"`
}

type buildTaggedTBTCSignerInteractiveSessionAbortResponse struct {
	SessionID string `json:"session_id"`
	Aborted   bool   `json:"aborted"`
}

func (bttse *buildTaggedTBTCSignerEngine) InteractiveSessionOpen(
	sessionID string,
	memberIdentifier uint16,
	message []byte,
	keyGroup string,
	threshold uint16,
	taprootMerkleRoot *[32]byte,
	signingIntent *SigningIntent,
	attemptContext NativeInteractiveAttemptContext,
) (*NativeInteractiveSessionOpenResult, error) {
	requestPayload, err := buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
		sessionID,
		memberIdentifier,
		message,
		keyGroup,
		threshold,
		taprootMerkleRoot,
		signingIntent,
		attemptContext,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerInteractiveSessionOpen(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerInteractiveSessionOpenResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) InteractiveRound1(
	sessionID string,
	attemptID string,
	memberIdentifier uint16,
) (commitments []byte, err error) {
	requestPayload, err := buildTaggedTBTCSignerInteractiveRound1RequestPayload(
		sessionID,
		attemptID,
		memberIdentifier,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerInteractiveRound1(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerInteractiveRound1Response(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) InteractiveRound2(
	sessionID string,
	attemptID string,
	memberIdentifier uint16,
	signingPackage []byte,
) (signatureShare []byte, err error) {
	requestPayload, err := buildTaggedTBTCSignerInteractiveRound2RequestPayload(
		sessionID,
		attemptID,
		memberIdentifier,
		signingPackage,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerInteractiveRound2(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerInteractiveRound2Response(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) InteractiveSessionAbort(
	sessionID string,
	attemptID *string,
) (*NativeInteractiveSessionAbortResult, error) {
	requestPayload, err := buildTaggedTBTCSignerInteractiveSessionAbortRequestPayload(
		sessionID,
		attemptID,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerInteractiveSessionAbort(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerInteractiveSessionAbortResponse(responsePayload)
}

func buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
	sessionID string,
	memberIdentifier uint16,
	message []byte,
	keyGroup string,
	threshold uint16,
	taprootMerkleRoot *[32]byte,
	signingIntent *SigningIntent,
	attemptContext NativeInteractiveAttemptContext,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "session ID is empty")
	}
	if memberIdentifier == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "member identifier is zero")
	}
	if len(message) == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "message is empty")
	}
	if keyGroup == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "key group is empty")
	}
	if threshold == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "threshold is zero")
	}
	if attemptContext.AttemptID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "attempt context attempt ID is empty")
	}
	if attemptContext.CoordinatorIdentifier == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "attempt context coordinator identifier is zero")
	}
	if len(attemptContext.IncludedParticipants) == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "attempt context included participants are empty")
	}

	// attempt.AttemptContext numbers attempts 0-based; the engine's wire
	// attempt_number is 1-based and rejects 0 ("must be at least 1"). Convert
	// here so the first attempt (RFC 0) is sent as wire 1 rather than rejected
	// before round 1. The engine subtracts 1 internally for its shuffle math.
	wireAttemptNumber := attemptContext.AttemptNumber + 1
	if wireAttemptNumber == 0 {
		// attemptContext.AttemptNumber was the max uint32; +1 wrapped to 0.
		return nil, buildTaggedTBTCSignerOperationError(
			"InteractiveSessionOpen",
			"attempt number overflows the 1-based wire encoding",
		)
	}

	var taprootMerkleRootHex *string
	if taprootMerkleRoot != nil {
		encoded := hex.EncodeToString(taprootMerkleRoot[:])
		taprootMerkleRootHex = &encoded
	}

	var wireSigningIntent *buildTaggedTBTCSignerSigningIntent
	if signingIntent != nil {
		heartbeatMessage, ok := signingIntent.HeartbeatMessage()
		if !ok {
			return nil, buildTaggedTBTCSignerOperationError(
				"InteractiveSessionOpen",
				"signing intent has an unsupported type",
			)
		}
		wireSigningIntent = &buildTaggedTBTCSignerSigningIntent{
			Type:       "heartbeat",
			MessageHex: hex.EncodeToString(heartbeatMessage[:]),
		}
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"InteractiveSessionOpen",
		buildTaggedTBTCSignerInteractiveSessionOpenRequest{
			SessionID:            sessionID,
			MemberIdentifier:     memberIdentifier,
			MessageHex:           hex.EncodeToString(message),
			KeyGroup:             keyGroup,
			Threshold:            threshold,
			TaprootMerkleRootHex: taprootMerkleRootHex,
			SigningIntent:        wireSigningIntent,
			AttemptContext: buildTaggedTBTCSignerInteractiveAttemptContext{
				AttemptNumber:                   wireAttemptNumber,
				CoordinatorIdentifier:           attemptContext.CoordinatorIdentifier,
				IncludedParticipants:            append([]uint16(nil), attemptContext.IncludedParticipants...),
				IncludedParticipantsFingerprint: attemptContext.IncludedParticipantsFingerprint,
				AttemptID:                       attemptContext.AttemptID,
			},
		},
	)
}

func decodeBuildTaggedTBTCSignerInteractiveSessionOpenResponse(
	responsePayload []byte,
) (*NativeInteractiveSessionOpenResult, error) {
	var response buildTaggedTBTCSignerInteractiveSessionOpenResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"InteractiveSessionOpen",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}
	if response.SessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "response session ID is empty")
	}
	if response.AttemptID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionOpen", "response attempt ID is empty")
	}

	return &NativeInteractiveSessionOpenResult{
		SessionID:  response.SessionID,
		AttemptID:  response.AttemptID,
		Idempotent: response.Idempotent,
	}, nil
}

func buildTaggedTBTCSignerInteractiveRound1RequestPayload(
	sessionID string,
	attemptID string,
	memberIdentifier uint16,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveRound1", "session ID is empty")
	}
	if attemptID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveRound1", "attempt ID is empty")
	}
	if memberIdentifier == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveRound1", "member identifier is zero")
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"InteractiveRound1",
		buildTaggedTBTCSignerInteractiveRound1Request{
			SessionID:        sessionID,
			AttemptID:        attemptID,
			MemberIdentifier: memberIdentifier,
		},
	)
}

func decodeBuildTaggedTBTCSignerInteractiveRound1Response(
	responsePayload []byte,
) ([]byte, error) {
	var response buildTaggedTBTCSignerInteractiveRound1Response
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"InteractiveRound1",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	return buildTaggedTBTCSignerDecodeHexField(
		"InteractiveRound1",
		"response commitments",
		response.CommitmentsHex,
	)
}

func buildTaggedTBTCSignerInteractiveRound2RequestPayload(
	sessionID string,
	attemptID string,
	memberIdentifier uint16,
	signingPackage []byte,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveRound2", "session ID is empty")
	}
	if attemptID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveRound2", "attempt ID is empty")
	}
	if memberIdentifier == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveRound2", "member identifier is zero")
	}
	if len(signingPackage) == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveRound2", "signing package is empty")
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"InteractiveRound2",
		buildTaggedTBTCSignerInteractiveRound2Request{
			SessionID:         sessionID,
			AttemptID:         attemptID,
			MemberIdentifier:  memberIdentifier,
			SigningPackageHex: hex.EncodeToString(signingPackage),
		},
	)
}

func decodeBuildTaggedTBTCSignerInteractiveRound2Response(
	responsePayload []byte,
) ([]byte, error) {
	var response buildTaggedTBTCSignerInteractiveRound2Response
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"InteractiveRound2",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	return buildTaggedTBTCSignerDecodeHexField(
		"InteractiveRound2",
		"response signature share",
		response.SignatureShareHex,
	)
}

func buildTaggedTBTCSignerInteractiveSessionAbortRequestPayload(
	sessionID string,
	attemptID *string,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionAbort", "session ID is empty")
	}

	var requestAttemptID *string
	if attemptID != nil {
		if *attemptID == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"InteractiveSessionAbort",
				"attempt ID is set but empty",
			)
		}
		copied := *attemptID
		requestAttemptID = &copied
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"InteractiveSessionAbort",
		buildTaggedTBTCSignerInteractiveSessionAbortRequest{
			SessionID: sessionID,
			AttemptID: requestAttemptID,
		},
	)
}

func decodeBuildTaggedTBTCSignerInteractiveSessionAbortResponse(
	responsePayload []byte,
) (*NativeInteractiveSessionAbortResult, error) {
	var response buildTaggedTBTCSignerInteractiveSessionAbortResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"InteractiveSessionAbort",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}
	if response.SessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveSessionAbort", "response session ID is empty")
	}

	return &NativeInteractiveSessionAbortResult{
		SessionID: response.SessionID,
		Aborted:   response.Aborted,
	}, nil
}

func callBuildTaggedTBTCSignerInteractiveSessionOpen(requestPayload []byte) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"InteractiveSessionOpen",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_interactive_session_open(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerInteractiveRound1(requestPayload []byte) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"InteractiveRound1",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_interactive_round1(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerInteractiveRound2(requestPayload []byte) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"InteractiveRound2",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_interactive_round2(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerInteractiveSessionAbort(requestPayload []byte) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"InteractiveSessionAbort",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_interactive_session_abort(requestPtr, requestLen)
		},
	)
}

// ----------------------------------------------------------------------------
// Phase 7.3 interactive aggregation bridge.
//
// Aggregates the responsive subset's signature shares for an interactive
// attempt into the BIP-340 signature. The engine resolves the verifying
// material from the session's own DKG state (no public key package crosses
// here). On a share-verification failure it returns the candidate culprits in
// the error payload; InteractiveAggregate surfaces them as a typed
// InteractiveAggregateShareVerificationError for the Go host's envelope-bound
// blame adjudication. Additive: no Go caller yet.
// ----------------------------------------------------------------------------

type buildTaggedTBTCSignerInteractiveAggregateRequest struct {
	SessionID            string                                           `json:"session_id"`
	AttemptID            string                                           `json:"attempt_id"`
	SigningPackageHex    string                                           `json:"signing_package_hex"`
	SignatureShares      []buildTaggedTBTCSignerNativeFROSTSignatureShare `json:"signature_shares"`
	TaprootMerkleRootHex *string                                          `json:"taproot_merkle_root_hex,omitempty"`
}

type buildTaggedTBTCSignerInteractiveAggregateResponse struct {
	SessionID    string `json:"session_id"`
	AttemptID    string `json:"attempt_id"`
	SignatureHex string `json:"signature_hex"`
}

func (bttse *buildTaggedTBTCSignerEngine) InteractiveAggregate(
	sessionID string,
	attemptID string,
	signingPackage []byte,
	signatureShares []nativeFROSTSignatureShare,
	taprootMerkleRoot *[32]byte,
) (signature []byte, err error) {
	requestPayload, err := buildTaggedTBTCSignerInteractiveAggregateRequestPayload(
		sessionID,
		attemptID,
		signingPackage,
		signatureShares,
		taprootMerkleRoot,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerInteractiveAggregate(requestPayload)
	if err != nil {
		// Surface a share-verification failure as the typed error carrying the
		// candidate culprits; any other error passes through unchanged.
		return nil, interpretInteractiveAggregateError(sessionID, attemptID, err)
	}

	return decodeBuildTaggedTBTCSignerInteractiveAggregateResponse(responsePayload)
}

func buildTaggedTBTCSignerInteractiveAggregateRequestPayload(
	sessionID string,
	attemptID string,
	signingPackage []byte,
	signatureShares []nativeFROSTSignatureShare,
	taprootMerkleRoot *[32]byte,
) ([]byte, error) {
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveAggregate", "session ID is empty")
	}
	if attemptID == "" {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveAggregate", "attempt ID is empty")
	}
	if len(signingPackage) == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveAggregate", "signing package is empty")
	}
	if len(signatureShares) == 0 {
		return nil, buildTaggedTBTCSignerOperationError("InteractiveAggregate", "signature shares are empty")
	}

	requestShares := make(
		[]buildTaggedTBTCSignerNativeFROSTSignatureShare,
		0,
		len(signatureShares),
	)
	for i, signatureShare := range signatureShares {
		if signatureShare.Identifier == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"InteractiveAggregate",
				fmt.Sprintf("signature share [%d] identifier is empty", i),
			)
		}
		if len(signatureShare.Data) == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"InteractiveAggregate",
				fmt.Sprintf("signature share [%d] data is empty", i),
			)
		}
		requestShares = append(
			requestShares,
			buildTaggedTBTCSignerNativeFROSTSignatureShare{
				Identifier: signatureShare.Identifier,
				DataHex:    hex.EncodeToString(signatureShare.Data),
			},
		)
	}

	var taprootMerkleRootHex *string
	if taprootMerkleRoot != nil {
		encoded := hex.EncodeToString(taprootMerkleRoot[:])
		taprootMerkleRootHex = &encoded
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"InteractiveAggregate",
		buildTaggedTBTCSignerInteractiveAggregateRequest{
			SessionID:            sessionID,
			AttemptID:            attemptID,
			SigningPackageHex:    hex.EncodeToString(signingPackage),
			SignatureShares:      requestShares,
			TaprootMerkleRootHex: taprootMerkleRootHex,
		},
	)
}

func decodeBuildTaggedTBTCSignerInteractiveAggregateResponse(
	responsePayload []byte,
) ([]byte, error) {
	var response buildTaggedTBTCSignerInteractiveAggregateResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"InteractiveAggregate",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	return buildTaggedTBTCSignerDecodeHexField(
		"InteractiveAggregate",
		"response signature",
		response.SignatureHex,
	)
}

func callBuildTaggedTBTCSignerInteractiveAggregate(requestPayload []byte) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"InteractiveAggregate",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_interactive_aggregate(requestPtr, requestLen)
		},
	)
}

// ----------------------------------------------------------------------------
// Phase 7.3 interactive attempt-context derivation bridge.
//
// Derives the canonical attempt context (coordinator, included-participants
// fingerprint, attempt id) + per-participant FROST identifiers from an attempt's
// public inputs, so the Go host never re-implements the engine's
// domain-separated derivations. Stateless and secret-free; the engine
// re-validates the derived context against the same strict check
// InteractiveSessionOpen runs, so the host can pass the result straight back in.
// Additive: no Go caller yet (the runner wiring is the next increment).
// ----------------------------------------------------------------------------

type buildTaggedTBTCSignerDeriveInteractiveAttemptContextRequest struct {
	SessionID            string   `json:"session_id"`
	MessageHex           string   `json:"message_hex"`
	KeyGroup             string   `json:"key_group"`
	Threshold            uint16   `json:"threshold"`
	AttemptNumber        uint32   `json:"attempt_number"`
	IncludedParticipants []uint16 `json:"included_participants"`
}

type buildTaggedTBTCSignerParticipantFrostIdentifier struct {
	ParticipantIdentifier uint16 `json:"participant_identifier"`
	FrostIdentifier       string `json:"frost_identifier"`
}

type buildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse struct {
	AttemptContext   buildTaggedTBTCSignerInteractiveAttemptContext    `json:"attempt_context"`
	FrostIdentifiers []buildTaggedTBTCSignerParticipantFrostIdentifier `json:"frost_identifiers"`
}

func (bttse *buildTaggedTBTCSignerEngine) DeriveInteractiveAttemptContext(
	sessionID string,
	message []byte,
	keyGroup string,
	threshold uint16,
	attemptNumber uint32,
	includedParticipants []uint16,
) (*NativeDeriveInteractiveAttemptContextResult, error) {
	requestPayload, err := buildTaggedTBTCSignerDeriveInteractiveAttemptContextRequestPayload(
		sessionID,
		message,
		keyGroup,
		threshold,
		attemptNumber,
		includedParticipants,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerDeriveInteractiveAttemptContext(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse(responsePayload)
}

func buildTaggedTBTCSignerDeriveInteractiveAttemptContextRequestPayload(
	sessionID string,
	message []byte,
	keyGroup string,
	threshold uint16,
	attemptNumber uint32,
	includedParticipants []uint16,
) ([]byte, error) {
	const operation = "DeriveInteractiveAttemptContext"
	if sessionID == "" {
		return nil, buildTaggedTBTCSignerOperationError(operation, "session ID is empty")
	}
	if len(message) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(operation, "message is empty")
	}
	if keyGroup == "" {
		return nil, buildTaggedTBTCSignerOperationError(operation, "key group is empty")
	}
	if threshold == 0 {
		return nil, buildTaggedTBTCSignerOperationError(operation, "threshold is zero")
	}
	if len(includedParticipants) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(operation, "included participants are empty")
	}

	// attempt.AttemptContext numbers attempts 0-based; the engine's wire
	// attempt_number is 1-based and rejects 0. Convert here so the first attempt
	// (RFC 0) is sent as wire 1, matching InteractiveSessionOpen.
	wireAttemptNumber := attemptNumber + 1
	if wireAttemptNumber == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			"attempt number overflows the 1-based wire encoding",
		)
	}

	return buildTaggedTBTCSignerMarshalRequest(
		operation,
		buildTaggedTBTCSignerDeriveInteractiveAttemptContextRequest{
			SessionID:            sessionID,
			MessageHex:           hex.EncodeToString(message),
			KeyGroup:             keyGroup,
			Threshold:            threshold,
			AttemptNumber:        wireAttemptNumber,
			IncludedParticipants: append([]uint16(nil), includedParticipants...),
		},
	)
}

func decodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse(
	responsePayload []byte,
) (*NativeDeriveInteractiveAttemptContextResult, error) {
	const operation = "DeriveInteractiveAttemptContext"
	var response buildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	attemptContext := response.AttemptContext
	if attemptContext.AttemptID == "" {
		return nil, buildTaggedTBTCSignerOperationError(operation, "response attempt context attempt ID is empty")
	}
	if attemptContext.CoordinatorIdentifier == 0 {
		return nil, buildTaggedTBTCSignerOperationError(operation, "response attempt context coordinator identifier is zero")
	}
	// The engine's wire attempt_number is 1-based; 0 is impossible and would
	// underflow the conversion below.
	if attemptContext.AttemptNumber == 0 {
		return nil, buildTaggedTBTCSignerOperationError(operation, "response attempt context attempt number is zero")
	}
	if len(attemptContext.IncludedParticipants) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(operation, "response attempt context included participants are empty")
	}
	// The engine returns exactly one FROST identifier per included participant
	// (canonical order); a mismatch is a malformed response the host must not
	// silently consume - downstream signing-package/aggregate keying depends on
	// the 1:1 correspondence.
	if len(response.FrostIdentifiers) != len(attemptContext.IncludedParticipants) {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf(
				"response has [%d] frost identifiers for [%d] included participants",
				len(response.FrostIdentifiers),
				len(attemptContext.IncludedParticipants),
			),
		)
	}

	frostIdentifiers := make([]NativeFROSTParticipantIdentifier, 0, len(response.FrostIdentifiers))
	for i, entry := range response.FrostIdentifiers {
		if entry.FrostIdentifier == "" {
			return nil, buildTaggedTBTCSignerOperationError(operation, "response frost identifier is empty")
		}
		// The engine returns identifiers in canonical participant order, one per
		// included participant. Bind each entry to the participant at its position:
		// a matching count alone still lets a duplicate, zero, reordered, or
		// foreign participant_identifier through, yielding a mapping that diverges
		// from included_participants - which the runner keys commitments and shares
		// by. (Index is in bounds: the count-match check above pins the lengths.)
		if entry.ParticipantIdentifier != attemptContext.IncludedParticipants[i] {
			return nil, buildTaggedTBTCSignerOperationError(
				operation,
				fmt.Sprintf(
					"response frost identifier [%d] is for participant [%d], expected [%d]",
					i,
					entry.ParticipantIdentifier,
					attemptContext.IncludedParticipants[i],
				),
			)
		}
		frostIdentifiers = append(frostIdentifiers, NativeFROSTParticipantIdentifier{
			ParticipantIdentifier: entry.ParticipantIdentifier,
			FrostIdentifier:       entry.FrostIdentifier,
		})
	}

	return &NativeDeriveInteractiveAttemptContextResult{
		AttemptContext: NativeInteractiveAttemptContext{
			// Wire 1-based -> RFC-21 0-based, the inverse of the request encoding,
			// so the host receives the natural attempt.AttemptContext value.
			AttemptNumber:                   attemptContext.AttemptNumber - 1,
			CoordinatorIdentifier:           attemptContext.CoordinatorIdentifier,
			IncludedParticipants:            append([]uint16(nil), attemptContext.IncludedParticipants...),
			IncludedParticipantsFingerprint: attemptContext.IncludedParticipantsFingerprint,
			AttemptID:                       attemptContext.AttemptID,
		},
		FrostIdentifiers: frostIdentifiers,
	}, nil
}

func callBuildTaggedTBTCSignerDeriveInteractiveAttemptContext(requestPayload []byte) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"DeriveInteractiveAttemptContext",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_derive_interactive_attempt_context(requestPtr, requestLen)
		},
	)
}

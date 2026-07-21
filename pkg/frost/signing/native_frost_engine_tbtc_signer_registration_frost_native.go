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
	"math"
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
	SecretPackageHex string                                 `json:"secret_package_hex"`
	Package          *buildTaggedTBTCSignerDKGRound1Package `json:"package"`
}

type buildTaggedTBTCSignerDKGPart2Request struct {
	SecretPackageHex string                                  `json:"secret_package_hex"`
	Round1Packages   []buildTaggedTBTCSignerDKGRound1Package `json:"round1_packages"`
}

type buildTaggedTBTCSignerDKGPart2Response struct {
	SecretPackageHex string                                  `json:"secret_package_hex"`
	Packages         []buildTaggedTBTCSignerDKGRound2Package `json:"packages"`
}

type buildTaggedTBTCSignerDKGPart3Request struct {
	SecretPackageHex string                                  `json:"secret_package_hex"`
	Round1Packages   []buildTaggedTBTCSignerDKGRound1Package `json:"round1_packages"`
	Round2Packages   []buildTaggedTBTCSignerDKGRound2Package `json:"round2_packages"`
}

type buildTaggedTBTCSignerNativeFROSTKeyPackage struct {
	Identifier string `json:"identifier"`
	DataHex    string `json:"data_hex"`
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

	secretPackageData, err := buildTaggedTBTCSignerDecodeHexField(
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
			SecretPackageHex: hex.EncodeToString(secretPackage.Data),
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

	secretPackageData, err := buildTaggedTBTCSignerDecodeHexField(
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
			SecretPackageHex: hex.EncodeToString(secretPackage.Data),
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
				DataHex:    hex.EncodeToString(keyPackage.Data),
			},
			PublicKeyPackage: &buildTaggedTBTCSignerNativeFROSTPublicKeyPackage{
				VerifyingShares: publicKeyPackage.VerifyingShares,
				VerifyingKey:    publicKeyPackage.VerifyingKey,
			},
		},
	)
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
	keyPackageData, err := buildTaggedTBTCSignerDecodeHexField(
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
	result := C.tbtc_signer_version()
	return parseBuildTaggedTBTCSignerResult("Version", result)
}

// callBuildTaggedTBTCSignerABIVersion fetches the structured FFI contract version. A
// missing frost_tbtc_abi_version symbol surfaces as ErrNativeCryptographyUnavailable
// (the lib predates ABI versioning), which the ABI preflight turns into an explicit
// incompatibility. It deliberately does NOT pass through callBuildTaggedTBTCSignerOperation
// (it takes no request and must not recurse into the ABI gate).
func callBuildTaggedTBTCSignerABIVersion() ([]byte, error) {
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

	requestPtr := C.CBytes(requestPayload)
	requestLen := len(requestPayload)
	defer func() {
		// Scrub the secret request bytes from the C heap before releasing them.
		// The request payload can carry signing-share / nonce material, and a
		// plain C.free does not overwrite; this mirrors the Go-side zeroBytes
		// hygiene applied to the caller's own copy.
		zeroBytes(unsafe.Slice((*byte)(requestPtr), requestLen))
		C.free(requestPtr)
	}()

	result := call((*C.uint8_t)(requestPtr), C.size_t(len(requestPayload)))
	return parseBuildTaggedTBTCSignerResult(operation, result)
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
		defer C.tbtc_signer_free_buffer(result.buffer.ptr, result.buffer.len)
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
	return callBuildTaggedTBTCSignerOperation(
		"InitSignerConfig",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_init_signer_config(requestPtr, requestLen)
		},
	)
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

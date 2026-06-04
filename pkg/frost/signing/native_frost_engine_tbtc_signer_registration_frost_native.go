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
typedef TbtcSignerResult (*tbtc_run_dkg_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
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
typedef TbtcSignerResult (*tbtc_generate_nonces_and_commitments_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_new_signing_package_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_sign_share_fn)(
  const uint8_t* request_ptr,
  size_t request_len
);
typedef TbtcSignerResult (*tbtc_aggregate_fn)(
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
typedef TbtcSignerResult (*tbtc_build_taproot_tx_fn)(
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

static TbtcSignerResult tbtc_signer_generate_nonces_and_commitments(const uint8_t* request_ptr, size_t request_len) {
  tbtc_generate_nonces_and_commitments_fn generate_nonces_and_commitments =
    (tbtc_generate_nonces_and_commitments_fn)dlsym(
      RTLD_DEFAULT,
      "frost_tbtc_generate_nonces_and_commitments"
    );
  if (generate_nonces_and_commitments == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return generate_nonces_and_commitments(request_ptr, request_len);
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

static TbtcSignerResult tbtc_signer_sign_share(const uint8_t* request_ptr, size_t request_len) {
  tbtc_sign_share_fn sign_share = (tbtc_sign_share_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_sign_share"
  );
  if (sign_share == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return sign_share(request_ptr, request_len);
}

static TbtcSignerResult tbtc_signer_aggregate(const uint8_t* request_ptr, size_t request_len) {
  tbtc_aggregate_fn aggregate = (tbtc_aggregate_fn)dlsym(
    RTLD_DEFAULT,
    "frost_tbtc_aggregate"
  );
  if (aggregate == NULL) {
    return unavailable_tbtc_signer_result();
  }

  return aggregate(request_ptr, request_len);
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

type buildTaggedTBTCSignerNativeFROSTCommitment struct {
	Identifier string `json:"identifier"`
	DataHex    string `json:"data_hex"`
}

type buildTaggedTBTCSignerNativeFROSTSignatureShare struct {
	Identifier string `json:"identifier"`
	DataHex    string `json:"data_hex"`
}

type buildTaggedTBTCSignerGenerateNoncesRequest struct {
	KeyPackageIdentifier string `json:"key_package_identifier"`
	KeyPackageHex        string `json:"key_package_hex"`
}

type buildTaggedTBTCSignerGenerateNoncesResponse struct {
	NoncesHex  string                                      `json:"nonces_hex"`
	Commitment *buildTaggedTBTCSignerNativeFROSTCommitment `json:"commitment"`
}

type buildTaggedTBTCSignerNewSigningPackageRequest struct {
	MessageHex  string                                       `json:"message_hex"`
	Commitments []buildTaggedTBTCSignerNativeFROSTCommitment `json:"commitments"`
}

type buildTaggedTBTCSignerNewSigningPackageResponse struct {
	SigningPackageHex string `json:"signing_package_hex"`
}

type buildTaggedTBTCSignerSignShareRequest struct {
	SigningPackageHex    string `json:"signing_package_hex"`
	NoncesHex            string `json:"nonces_hex"`
	KeyPackageIdentifier string `json:"key_package_identifier"`
	KeyPackageHex        string `json:"key_package_hex"`
}

type buildTaggedTBTCSignerSignShareResponse struct {
	SignatureShare *buildTaggedTBTCSignerNativeFROSTSignatureShare `json:"signature_share"`
}

type buildTaggedTBTCSignerAggregateRequest struct {
	SigningPackageHex string                                            `json:"signing_package_hex"`
	SignatureShares   []buildTaggedTBTCSignerNativeFROSTSignatureShare  `json:"signature_shares"`
	PublicKeyPackage  *buildTaggedTBTCSignerNativeFROSTPublicKeyPackage `json:"public_key_package"`
}

type buildTaggedTBTCSignerAggregateResponse struct {
	SignatureHex string `json:"signature_hex"`
}

type buildTaggedTBTCSignerStartSignRoundRequest struct {
	SessionID           string   `json:"session_id"`
	MemberIdentifier    uint16   `json:"member_identifier"`
	MessageHex          string   `json:"message_hex"`
	KeyGroup            string   `json:"key_group"`
	SigningParticipants []uint16 `json:"signing_participants,omitempty"`
}

type buildTaggedTBTCSignerStartSignRoundResponse struct {
	SessionID             string                                          `json:"session_id"`
	RoundID               string                                          `json:"round_id"`
	RequiredContributions uint16                                          `json:"required_contributions"`
	MessageDigestHex      string                                          `json:"message_digest_hex"`
	SigningParticipants   []uint16                                        `json:"signing_participants,omitempty"`
	OwnContribution       *buildTaggedTBTCSignerFinalizeRoundContribution `json:"own_contribution"`
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

type buildTaggedTBTCSignerBuildTaprootTxRequest struct {
	SessionID     string                                      `json:"session_id"`
	Inputs        []buildTaggedTBTCSignerBuildTaprootTxInput  `json:"inputs"`
	Outputs       []buildTaggedTBTCSignerBuildTaprootTxOutput `json:"outputs"`
	ScriptTreeHex *string                                     `json:"script_tree_hex,omitempty"`
}

type buildTaggedTBTCSignerBuildTaprootTxInput struct {
	TxIDHex   string `json:"txid_hex"`
	Vout      uint32 `json:"vout"`
	ValueSats uint64 `json:"value_sats"`
}

type buildTaggedTBTCSignerBuildTaprootTxOutput struct {
	ScriptPubKeyHex string `json:"script_pubkey_hex"`
	ValueSats       uint64 `json:"value_sats"`
}

type buildTaggedTBTCSignerBuildTaprootTxResponse struct {
	SessionID string `json:"session_id"`
	TxHex     string `json:"tx_hex"`
}

const buildTaggedTBTCSignerUnavailableStatusCode = -1

func registerBuildTaggedNativeFROSTSigningEngine() error {
	engine := &buildTaggedTBTCSignerEngine{}

	if err := RegisterNativeTBTCSignerEngine(engine); err != nil {
		return err
	}

	dkgEngine, err := newUniFFINativeFROSTDKGEngine(engine)
	if err != nil {
		return err
	}
	if err := RegisterNativeFROSTDKGEngine(dkgEngine); err != nil {
		return err
	}

	signingEngine, err := newUniFFINativeFROSTSigningEngine(engine)
	if err != nil {
		return err
	}
	return RegisterNativeFROSTSigningEngine(signingEngine)
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

	responsePayload, err := callBuildTaggedTBTCSignerDKGPart2(requestPayload)
	if err != nil {
		return nil, err
	}

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

	responsePayload, err := callBuildTaggedTBTCSignerDKGPart3(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerDKGPart3Response(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) GenerateNoncesAndCommitments(
	keyPackageIdentifier string,
	keyPackageData []byte,
) (noncesData []byte, commitmentIdentifier string, commitmentData []byte, err error) {
	requestPayload, err := buildTaggedTBTCSignerGenerateNoncesRequestPayload(
		keyPackageIdentifier,
		keyPackageData,
	)
	if err != nil {
		return nil, "", nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerGenerateNoncesAndCommitments(
		requestPayload,
	)
	if err != nil {
		return nil, "", nil, err
	}

	return decodeBuildTaggedTBTCSignerGenerateNoncesResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) NewSigningPackage(
	message []byte,
	commitments []uniFFINativeFROSTCommitment,
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

func (bttse *buildTaggedTBTCSignerEngine) Sign(
	signingPackageData []byte,
	noncesData []byte,
	keyPackageIdentifier string,
	keyPackageData []byte,
) (signatureShareIdentifier string, signatureShareData []byte, err error) {
	requestPayload, err := buildTaggedTBTCSignerSignShareRequestPayload(
		signingPackageData,
		noncesData,
		keyPackageIdentifier,
		keyPackageData,
	)
	if err != nil {
		return "", nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerSignShare(requestPayload)
	if err != nil {
		return "", nil, err
	}

	return decodeBuildTaggedTBTCSignerSignShareResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) Aggregate(
	signingPackageData []byte,
	signatureShares []uniFFINativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) (signature []byte, err error) {
	requestPayload, err := buildTaggedTBTCSignerAggregateRequestPayload(
		signingPackageData,
		signatureShares,
		publicKeyPackage,
	)
	if err != nil {
		return nil, err
	}

	responsePayload, err := callBuildTaggedTBTCSignerAggregate(requestPayload)
	if err != nil {
		return nil, err
	}

	return decodeBuildTaggedTBTCSignerAggregateResponse(responsePayload)
}

func (bttse *buildTaggedTBTCSignerEngine) StartSignRound(
	sessionID string,
	memberIdentifier uint16,
	message []byte,
	keyGroup string,
	signingParticipants []uint16,
) (*NativeTBTCSignerRoundState, error) {
	requestPayload, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		sessionID,
		memberIdentifier,
		message,
		keyGroup,
		signingParticipants,
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

func buildTaggedTBTCSignerGenerateNoncesRequestPayload(
	keyPackageIdentifier string,
	keyPackageData []byte,
) ([]byte, error) {
	if keyPackageIdentifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"GenerateNoncesAndCommitments",
			"key package identifier is empty",
		)
	}
	if len(keyPackageData) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"GenerateNoncesAndCommitments",
			"key package data is empty",
		)
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"GenerateNoncesAndCommitments",
		buildTaggedTBTCSignerGenerateNoncesRequest{
			KeyPackageIdentifier: keyPackageIdentifier,
			KeyPackageHex:        hex.EncodeToString(keyPackageData),
		},
	)
}

func decodeBuildTaggedTBTCSignerGenerateNoncesResponse(
	responsePayload []byte,
) (noncesData []byte, commitmentIdentifier string, commitmentData []byte, err error) {
	var response buildTaggedTBTCSignerGenerateNoncesResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, "", nil, buildTaggedTBTCSignerOperationError(
			"GenerateNoncesAndCommitments",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}
	noncesData, err = buildTaggedTBTCSignerDecodeHexField(
		"GenerateNoncesAndCommitments",
		"response nonces",
		response.NoncesHex,
	)
	if err != nil {
		return nil, "", nil, err
	}
	commitment, err := decodeBuildTaggedTBTCSignerCommitment(
		"GenerateNoncesAndCommitments",
		"response commitment",
		response.Commitment,
	)
	if err != nil {
		return nil, "", nil, err
	}

	return noncesData, commitment.Identifier, commitment.Data, nil
}

func buildTaggedTBTCSignerNewSigningPackageRequestPayload(
	message []byte,
	commitments []uniFFINativeFROSTCommitment,
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

func buildTaggedTBTCSignerSignShareRequestPayload(
	signingPackageData []byte,
	noncesData []byte,
	keyPackageIdentifier string,
	keyPackageData []byte,
) ([]byte, error) {
	if len(signingPackageData) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"SignShare",
			"signing package data is empty",
		)
	}
	if len(noncesData) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"SignShare",
			"nonces data is empty",
		)
	}
	if keyPackageIdentifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"SignShare",
			"key package identifier is empty",
		)
	}
	if len(keyPackageData) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"SignShare",
			"key package data is empty",
		)
	}

	return buildTaggedTBTCSignerMarshalRequest(
		"SignShare",
		buildTaggedTBTCSignerSignShareRequest{
			SigningPackageHex:    hex.EncodeToString(signingPackageData),
			NoncesHex:            hex.EncodeToString(noncesData),
			KeyPackageIdentifier: keyPackageIdentifier,
			KeyPackageHex:        hex.EncodeToString(keyPackageData),
		},
	)
}

func decodeBuildTaggedTBTCSignerSignShareResponse(
	responsePayload []byte,
) (string, []byte, error) {
	var response buildTaggedTBTCSignerSignShareResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return "", nil, buildTaggedTBTCSignerOperationError(
			"SignShare",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	signatureShare, err := decodeBuildTaggedTBTCSignerSignatureShare(
		"SignShare",
		"response signature share",
		response.SignatureShare,
	)
	if err != nil {
		return "", nil, err
	}

	return signatureShare.Identifier, signatureShare.Data, nil
}

func buildTaggedTBTCSignerAggregateRequestPayload(
	signingPackageData []byte,
	signatureShares []uniFFINativeFROSTSignatureShare,
	publicKeyPackage *NativeFROSTPublicKeyPackage,
) ([]byte, error) {
	if len(signingPackageData) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"Aggregate",
			"signing package data is empty",
		)
	}
	if len(signatureShares) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"Aggregate",
			"signature shares are empty",
		)
	}
	if publicKeyPackage == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"Aggregate",
			"public key package is nil",
		)
	}
	if publicKeyPackage.VerifyingKey == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			"Aggregate",
			"public key package verifying key is empty",
		)
	}
	if len(publicKeyPackage.VerifyingShares) == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"Aggregate",
			"public key package verifying shares are empty",
		)
	}

	requestShares := make(
		[]buildTaggedTBTCSignerNativeFROSTSignatureShare,
		0,
		len(signatureShares),
	)
	for i, signatureShare := range signatureShares {
		if signatureShare.Identifier == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"Aggregate",
				fmt.Sprintf("signature share [%d] identifier is empty", i),
			)
		}
		if len(signatureShare.Data) == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"Aggregate",
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

	return buildTaggedTBTCSignerMarshalRequest(
		"Aggregate",
		buildTaggedTBTCSignerAggregateRequest{
			SigningPackageHex: hex.EncodeToString(signingPackageData),
			SignatureShares:   requestShares,
			PublicKeyPackage: &buildTaggedTBTCSignerNativeFROSTPublicKeyPackage{
				VerifyingShares: appendBuildTaggedTBTCSignerStringMap(
					publicKeyPackage.VerifyingShares,
				),
				VerifyingKey: publicKeyPackage.VerifyingKey,
			},
		},
	)
}

func decodeBuildTaggedTBTCSignerAggregateResponse(
	responsePayload []byte,
) ([]byte, error) {
	var response buildTaggedTBTCSignerAggregateResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return nil, buildTaggedTBTCSignerOperationError(
			"Aggregate",
			fmt.Sprintf("cannot decode response payload: %v", err),
		)
	}

	return buildTaggedTBTCSignerDecodeHexField(
		"Aggregate",
		"response signature",
		response.SignatureHex,
	)
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

func decodeBuildTaggedTBTCSignerCommitment(
	operation string,
	fieldName string,
	commitment *buildTaggedTBTCSignerNativeFROSTCommitment,
) (*NativeFROSTCommitment, error) {
	if commitment == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s is nil", fieldName),
		)
	}
	if commitment.Identifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s identifier is empty", fieldName),
		)
	}
	data, err := buildTaggedTBTCSignerDecodeHexField(
		operation,
		fmt.Sprintf("%s data", fieldName),
		commitment.DataHex,
	)
	if err != nil {
		return nil, err
	}

	return &NativeFROSTCommitment{
		Identifier: commitment.Identifier,
		Data:       data,
	}, nil
}

func decodeBuildTaggedTBTCSignerSignatureShare(
	operation string,
	fieldName string,
	signatureShare *buildTaggedTBTCSignerNativeFROSTSignatureShare,
) (*NativeFROSTSignatureShare, error) {
	if signatureShare == nil {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s is nil", fieldName),
		)
	}
	if signatureShare.Identifier == "" {
		return nil, buildTaggedTBTCSignerOperationError(
			operation,
			fmt.Sprintf("%s identifier is empty", fieldName),
		)
	}
	data, err := buildTaggedTBTCSignerDecodeHexField(
		operation,
		fmt.Sprintf("%s data", fieldName),
		signatureShare.DataHex,
	)
	if err != nil {
		return nil, err
	}

	return &NativeFROSTSignatureShare{
		Identifier: signatureShare.Identifier,
		Data:       data,
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

func buildTaggedTBTCSignerStartSignRoundRequestPayload(
	sessionID string,
	memberIdentifier uint16,
	message []byte,
	keyGroup string,
	signingParticipants []uint16,
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

	if memberIdentifier == 0 {
		return nil, buildTaggedTBTCSignerOperationError(
			"StartSignRound",
			"member identifier is zero",
		)
	}

	seenParticipants := make(map[uint16]struct{}, len(signingParticipants))
	for i, participant := range signingParticipants {
		if participant == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"StartSignRound",
				fmt.Sprintf("signing participant [%d] is zero", i),
			)
		}
		if _, ok := seenParticipants[participant]; ok {
			return nil, buildTaggedTBTCSignerOperationError(
				"StartSignRound",
				fmt.Sprintf("signing participant [%d] is duplicated", participant),
			)
		}
		seenParticipants[participant] = struct{}{}
	}

	request := buildTaggedTBTCSignerStartSignRoundRequest{
		SessionID:           sessionID,
		MemberIdentifier:    memberIdentifier,
		MessageHex:          hex.EncodeToString(message),
		KeyGroup:            keyGroup,
		SigningParticipants: append([]uint16{}, signingParticipants...),
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

	seenSigningParticipants := make(map[uint16]struct{}, len(response.SigningParticipants))
	for _, participant := range response.SigningParticipants {
		if participant == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"StartSignRound",
				"response signing participant is zero",
			)
		}

		if _, ok := seenSigningParticipants[participant]; ok {
			return nil, buildTaggedTBTCSignerOperationError(
				"StartSignRound",
				fmt.Sprintf("response signing participant [%d] is duplicated", participant),
			)
		}

		seenSigningParticipants[participant] = struct{}{}
	}

	var ownContribution *NativeTBTCSignerRoundContribution
	if response.OwnContribution != nil {
		if response.OwnContribution.Identifier == 0 {
			return nil, buildTaggedTBTCSignerOperationError(
				"StartSignRound",
				"response own contribution identifier is zero",
			)
		}

		if response.OwnContribution.SignatureShareHex == "" {
			return nil, buildTaggedTBTCSignerOperationError(
				"StartSignRound",
				"response own contribution signature share is empty",
			)
		}

		ownContributionData, err := hex.DecodeString(
			response.OwnContribution.SignatureShareHex,
		)
		if err != nil {
			return nil, buildTaggedTBTCSignerOperationError(
				"StartSignRound",
				fmt.Sprintf(
					"response own contribution signature share is invalid hex: %v",
					err,
				),
			)
		}

		ownContribution = &NativeTBTCSignerRoundContribution{
			Identifier: response.OwnContribution.Identifier,
			Data:       ownContributionData,
		}
	}

	return &NativeTBTCSignerRoundState{
		SessionID:             response.SessionID,
		RoundID:               response.RoundID,
		RequiredContributions: response.RequiredContributions,
		MessageDigestHex:      response.MessageDigestHex,
		SigningParticipants:   append([]uint16{}, response.SigningParticipants...),
		OwnContribution:       ownContribution,
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

		requestInputs = append(
			requestInputs,
			buildTaggedTBTCSignerBuildTaprootTxInput{
				TxIDHex:   input.TxIDHex,
				Vout:      input.Vout,
				ValueSats: input.ValueSats,
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

	return &NativeTBTCSignerTxResult{
		SessionID: response.SessionID,
		TxHex:     response.TxHex,
	}, nil
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

func callBuildTaggedTBTCSignerGenerateNoncesAndCommitments(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"GenerateNoncesAndCommitments",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_generate_nonces_and_commitments(
				requestPtr,
				requestLen,
			)
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

func callBuildTaggedTBTCSignerSignShare(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"SignShare",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_sign_share(requestPtr, requestLen)
		},
	)
}

func callBuildTaggedTBTCSignerAggregate(
	requestPayload []byte,
) ([]byte, error) {
	return callBuildTaggedTBTCSignerOperation(
		"Aggregate",
		requestPayload,
		func(requestPtr *C.uint8_t, requestLen C.size_t) C.TbtcSignerResult {
			return C.tbtc_signer_aggregate(requestPtr, requestLen)
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

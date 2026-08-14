package tbtc

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

var frostNativeSignerAnchorTrustEd25519IdentityY = big.NewInt(1)

const (
	// FrostNativeSignerAnchorTrustCertificateSchema is the offline-authority
	// signed bootstrap/rotation certificate wire schema.
	FrostNativeSignerAnchorTrustCertificateSchema = "tbtc-frost-native-signer-state-anchor-trust-certificate/v1"

	// FrostNativeSignerAnchorTrustTransitionRequestSchema is the startup-only
	// native signer ABI 4.3 request carrying a bounded certificate chain and a
	// fresh target Read wrapper.
	FrostNativeSignerAnchorTrustTransitionRequestSchema = "tbtc-signer-state-anchor-trust-transition/v1"

	FrostNativeSignerAnchorTrustMaximumCertificateChainLength = 64

	// A certificate is persisted inside Rust's bounded 128 KiB trust-journal
	// record. Keep a conservative envelope allowance so every certificate Go
	// admits can be serialized with the record metadata before any mutation.
	frostNativeSignerAnchorTrustMaximumCertificateBytes            = 120 * 1024
	frostNativeSignerAnchorTrustMaximumAcknowledgementBytes        = 64 * 1024
	frostNativeSignerAnchorTrustMaximumReadResponseBytes           = 128 * 1024
	frostNativeSignerAnchorTrustMaximumTransitionRequestBytes      = frostsigning.NativeTBTCSignerStateAnchorTrustTransitionMaximumRequestBytes
	frostNativeSignerAnchorTrustMaximumJSONDepth                   = 32
	frostNativeSignerAnchorTrustCoreDomain                         = "tbtc-frost-native-signer-state-anchor-trust-transition-core/v1\x00"
	frostNativeSignerAnchorTrustOperationIDDomain                  = "tbtc-frost-native-signer-state-anchor-trust-transition-operation-id/v1\x00"
	frostNativeSignerAnchorTrustTransitionDigestDomain             = "tbtc-frost-native-signer-state-anchor-trust-transition-digest/v1\x00"
	frostNativeSignerAnchorTrustCertificateDomain                  = "tbtc-frost-native-signer-state-anchor-trust-certificate/v1\x00"
	frostNativeSignerAnchorTrustCertificateDigestDomain            = "tbtc-frost-native-signer-state-anchor-trust-certificate-digest/v1\x00"
	frostNativeSignerAnchorTrustBootstrapKindByte             byte = 1
	frostNativeSignerAnchorTrustRotationKindByte              byte = 2
)

var frostNativeSignerAnchorTrustEd25519SPKIPrefix = [...]byte{
	0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65,
	0x70, 0x03, 0x21, 0x00,
}

// FrostNativeSignerAnchorTrustCertificateKind selects the only two
// offline-authorized trust transitions.
type FrostNativeSignerAnchorTrustCertificateKind string

const (
	FrostNativeSignerAnchorTrustCertificateBootstrap FrostNativeSignerAnchorTrustCertificateKind = "bootstrap"
	FrostNativeSignerAnchorTrustCertificateRotation  FrostNativeSignerAnchorTrustCertificateKind = "rotation"
)

// FrostNativeSignerAnchorTrustReference is the certificate-specific full
// service reference. Unlike ordinary history references, it includes the
// previous event root so an epoch boundary cannot erase ancestry.
type FrostNativeSignerAnchorTrustReference struct {
	ServiceEpoch          uint64
	Revision              uint64
	PreviousEventRoot     [32]byte
	EventRoot             [32]byte
	AcknowledgementDigest [32]byte
	Checkpoint            FrostNativeSignerStateWitnessCheckpoint
}

// FrostNativeSignerAnchorTrustEndpoint is the exact trust/configuration state
// on one side of a certificate. Public keys are raw Ed25519 bytes; their SPKI
// hashes commit to the canonical DER prefix specified by RFC 8410.
type FrostNativeSignerAnchorTrustEndpoint struct {
	ActivationManifestHash          [32]byte
	ActivationManifestSequence      uint64
	BindingHash                     [32]byte
	ResponsePublicKey               [ed25519.PublicKeySize]byte
	ResponsePublicKeySPKISHA256     [32]byte
	OfflineAuthorityPublicKey       [ed25519.PublicKeySize]byte
	OfflineAuthoritySPKISHA256      [32]byte
	WitnessMaximumRecords           uint64
	WitnessRotationThresholdRecords uint64
	Reference                       FrostNativeSignerAnchorTrustReference
}

// FrostNativeSignerAnchorTrustCertificate is the parsed certificate. The
// target acknowledgement is retained byte-for-byte because its SHA-256,
// service signature, acknowledgement digest, and native recovery ABI all bind
// the exact JSON representation rather than a re-encoding.
type FrostNativeSignerAnchorTrustCertificate struct {
	Kind                        FrostNativeSignerAnchorTrustCertificateKind
	CertificateSequence         uint64
	PreviousCertificateDigest   [32]byte
	ProtocolID                  [32]byte
	StreamID                    [32]byte
	SignerStoreFingerprint      [32]byte
	From                        *FrostNativeSignerAnchorTrustEndpoint
	To                          FrostNativeSignerAnchorTrustEndpoint
	CoreDigest                  [32]byte
	CoreSignature               [ed25519.SignatureSize]byte
	OperationID                 [32]byte
	TransitionDigest            [32]byte
	TargetAcknowledgement       []byte
	TargetAcknowledgementSHA256 [32]byte
	FinalSignature              [ed25519.SignatureSize]byte
	CertificateDigest           [32]byte
}

// FrostNativeSignerAnchorTrustTransitionRequest is the parsed startup ABI
// request. TargetReadResponse remains exact signed JSON for the existing
// client/Rust semantic verifier.
type FrostNativeSignerAnchorTrustTransitionRequest struct {
	CertificateChain   []FrostNativeSignerAnchorTrustCertificate
	TargetReadResponse []byte
}

// FrostNativeSignerAnchorTrustCertificateHead is an authenticated external
// journal head. Production chain validation uses a prior head to authenticate
// a bounded missing suffix and always requires an exact expected final head.
type FrostNativeSignerAnchorTrustCertificateHead struct {
	CertificateSequence    uint64
	CertificateDigest      [32]byte
	ProtocolID             [32]byte
	StreamID               [32]byte
	SignerStoreFingerprint [32]byte
	Endpoint               FrostNativeSignerAnchorTrustEndpoint
}

// frostNativeSignerAnchorVerifiedTrustFloor is an unexported admission
// capability minted only after the complete certificate suffix, deployment
// pins, offline-authority signatures, and target acknowledgements have been
// authenticated. It is intentionally not caller-constructible through the
// public anchor-client configuration.
type frostNativeSignerAnchorVerifiedTrustFloor struct {
	certificate FrostNativeSignerAnchorTrustCertificate
}

// FrostNativeSignerAnchorTrustTargetAcknowledgementValidator is the deliberate
// semantic seam between this pure certificate protocol and the existing anchor
// acknowledgement verifier. It must verify the target response-key signature,
// binding, derived operation/transition IDs, full target reference, and the
// certificate-contextual revision-1 parent. The generic same-epoch verifier
// must not be weakened to accept a non-zero revision-1 parent.
type FrostNativeSignerAnchorTrustTargetAcknowledgementValidator func(
	certificate *FrostNativeSignerAnchorTrustCertificate,
	rawAcknowledgement []byte,
) error

// FrostNativeSignerAnchorTrustChainValidationOptions supplies deployment pins
// and the mandatory target-acknowledgement semantic verifier. Every expected
// pin and the expected final head are mandatory: accepting zero pins here
// would let an authority-signed but stale chain become its own rollback
// authority.
type FrostNativeSignerAnchorTrustChainValidationOptions struct {
	AllowLegacyAdoption bool

	ExpectedProtocolID                 [32]byte
	ExpectedStreamID                   [32]byte
	ExpectedSignerStoreFingerprint     [32]byte
	ExpectedOfflineAuthorityPublicKey  [ed25519.PublicKeySize]byte
	ExpectedOfflineAuthoritySPKISHA256 [32]byte
	PriorHead                          *FrostNativeSignerAnchorTrustCertificateHead
	ExpectedHead                       *FrostNativeSignerAnchorTrustCertificateHead

	ValidateTargetAcknowledgement FrostNativeSignerAnchorTrustTargetAcknowledgementValidator
}

type frostNativeSignerAnchorTrustCheckpointWire struct {
	StoreFingerprint        string `json:"storeFingerprint"`
	Generation              string `json:"generation"`
	PreviousStateCommitment string `json:"previousStateCommitment"`
	StateImageDigest        string `json:"stateImageDigest"`
	StateCommitment         string `json:"stateCommitment"`
}

type frostNativeSignerAnchorTrustReferenceWire struct {
	ServiceEpoch        string                                      `json:"serviceEpoch"`
	Revision            string                                      `json:"revision"`
	PreviousEventRoot   string                                      `json:"previousEventRoot"`
	EventRoot           string                                      `json:"eventRoot"`
	CheckpointAckDigest string                                      `json:"checkpointAckDigest"`
	Checkpoint          *frostNativeSignerAnchorTrustCheckpointWire `json:"checkpoint"`
}

type frostNativeSignerAnchorTrustEndpointWire struct {
	ActivationManifestHash          string                                     `json:"activationManifestHash"`
	ActivationManifestSequence      string                                     `json:"activationManifestSequence"`
	BindingHash                     string                                     `json:"bindingHash"`
	ResponsePublicKey               string                                     `json:"responsePublicKey"`
	ResponsePublicKeySPKISHA256     string                                     `json:"responsePublicKeySpkiSha256"`
	OfflineAuthorityPublicKey       string                                     `json:"offlineAuthorityPublicKey"`
	OfflineAuthoritySPKISHA256      string                                     `json:"offlineAuthoritySpkiSha256"`
	WitnessMaximumRecords           string                                     `json:"witnessMaximumRecords"`
	WitnessRotationThresholdRecords string                                     `json:"witnessRotationThresholdRecords"`
	Reference                       *frostNativeSignerAnchorTrustReferenceWire `json:"reference"`
}

type frostNativeSignerAnchorTrustCertificateWire struct {
	Schema                      string                                    `json:"schema"`
	Kind                        string                                    `json:"kind"`
	CertificateSequence         string                                    `json:"certificateSequence"`
	PreviousCertificateDigest   string                                    `json:"previousCertificateDigest"`
	ProtocolID                  string                                    `json:"protocolID"`
	StreamID                    string                                    `json:"streamID"`
	SignerStoreFingerprint      string                                    `json:"signerStoreFingerprint"`
	From                        *frostNativeSignerAnchorTrustEndpointWire `json:"from"`
	To                          *frostNativeSignerAnchorTrustEndpointWire `json:"to"`
	CoreDigest                  string                                    `json:"coreDigest"`
	CoreSignature               string                                    `json:"coreSignature"`
	OperationID                 string                                    `json:"operationID"`
	TransitionDigest            string                                    `json:"transitionDigest"`
	TargetAcknowledgementBase64 string                                    `json:"targetAcknowledgementBase64"`
	TargetAcknowledgementSHA256 string                                    `json:"targetAcknowledgementSHA256"`
	FinalSignature              string                                    `json:"finalSignature"`
	CertificateDigest           string                                    `json:"certificateDigest"`
}

type frostNativeSignerAnchorTrustTransitionRequestWire struct {
	Schema                   string             `json:"schema"`
	CertificateChain         *[]json.RawMessage `json:"certificateChain"`
	TargetReadResponseBase64 string             `json:"targetReadResponseBase64"`
}

// ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256 hashes the canonical
// RFC 8410 SubjectPublicKeyInfo DER for a raw Ed25519 key.
func ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
	publicKey [ed25519.PublicKeySize]byte,
) [32]byte {
	hasher := sha256.New()
	hasher.Write(frostNativeSignerAnchorTrustEd25519SPKIPrefix[:])
	hasher.Write(publicKey[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

// ComputeFrostNativeSignerAnchorTrustCoreDigest computes the fixed-width core
// authorization digest. Target event/ack fields are intentionally excluded:
// the service constructs them only after the offline authority signs the core.
func ComputeFrostNativeSignerAnchorTrustCoreDigest(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) ([32]byte, error) {
	if certificate == nil {
		return [32]byte{}, fmt.Errorf("native signer anchor trust certificate is nil")
	}
	kind, err := frostNativeSignerAnchorTrustKindByte(certificate.Kind)
	if err != nil {
		return [32]byte{}, err
	}
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(frostNativeSignerAnchorTrustCoreDomain)
	buffer.WriteByte(kind)
	frostNativeSignerAnchorTrustWriteUint64(buffer, certificate.CertificateSequence)
	buffer.Write(certificate.PreviousCertificateDigest[:])
	buffer.Write(certificate.ProtocolID[:])
	buffer.Write(certificate.StreamID[:])
	buffer.Write(certificate.SignerStoreFingerprint[:])
	if certificate.From == nil {
		frostNativeSignerAnchorTrustWriteCoreFrom(
			buffer,
			FrostNativeSignerAnchorTrustEndpoint{},
		)
	} else {
		frostNativeSignerAnchorTrustWriteCoreFrom(buffer, *certificate.From)
	}
	frostNativeSignerAnchorTrustWriteCoreTo(buffer, certificate.To)
	return sha256.Sum256(buffer.Bytes()), nil
}

// ComputeFrostNativeSignerAnchorTrustOperationID derives the unique operation
// identity solely from the signed core.
func ComputeFrostNativeSignerAnchorTrustOperationID(
	coreDigest [32]byte,
) [32]byte {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(frostNativeSignerAnchorTrustOperationIDDomain)
	buffer.Write(coreDigest[:])
	return sha256.Sum256(buffer.Bytes())
}

// ComputeFrostNativeSignerAnchorTrustTransitionDigest derives the successor
// acknowledgement transition digest without introducing a certificate-digest
// cycle.
func ComputeFrostNativeSignerAnchorTrustTransitionDigest(
	coreDigest [32]byte,
	operationID [32]byte,
) [32]byte {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(frostNativeSignerAnchorTrustTransitionDigestDomain)
	buffer.Write(coreDigest[:])
	buffer.Write(operationID[:])
	return sha256.Sum256(buffer.Bytes())
}

// ComputeFrostNativeSignerAnchorTrustFinalDigest computes the digest signed
// after the service has constructed the exact successor acknowledgement.
func ComputeFrostNativeSignerAnchorTrustFinalDigest(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) ([32]byte, error) {
	if certificate == nil {
		return [32]byte{}, fmt.Errorf("native signer anchor trust certificate is nil")
	}
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(frostNativeSignerAnchorTrustCertificateDomain)
	buffer.Write(certificate.CoreDigest[:])
	buffer.Write(certificate.CoreSignature[:])
	buffer.Write(certificate.OperationID[:])
	buffer.Write(certificate.TransitionDigest[:])
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		certificate.To.Reference.ServiceEpoch,
	)
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		certificate.To.Reference.Revision,
	)
	buffer.Write(certificate.To.Reference.PreviousEventRoot[:])
	buffer.Write(certificate.To.Reference.EventRoot[:])
	buffer.Write(certificate.To.Reference.AcknowledgementDigest[:])
	frostNativeSignerAnchorTrustWriteCheckpoint(
		buffer,
		certificate.To.Reference.Checkpoint,
	)
	buffer.Write(certificate.TargetAcknowledgementSHA256[:])
	return sha256.Sum256(buffer.Bytes()), nil
}

// ComputeFrostNativeSignerAnchorTrustCertificateDigest derives the append-only
// journal identity from the final authority signature and immutable authority
// SPKI pin.
func ComputeFrostNativeSignerAnchorTrustCertificateDigest(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) ([32]byte, error) {
	if certificate == nil {
		return [32]byte{}, fmt.Errorf("native signer anchor trust certificate is nil")
	}
	finalDigest, err := ComputeFrostNativeSignerAnchorTrustFinalDigest(certificate)
	if err != nil {
		return [32]byte{}, err
	}
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(frostNativeSignerAnchorTrustCertificateDigestDomain)
	buffer.Write(finalDigest[:])
	buffer.Write(certificate.FinalSignature[:])
	buffer.Write(certificate.To.OfflineAuthoritySPKISHA256[:])
	return sha256.Sum256(buffer.Bytes()), nil
}

// DecodeFrostNativeSignerAnchorTrustCertificate strictly decodes one bounded
// certificate without accepting case aliases, duplicate members, unknown
// members, non-canonical numbers/hex/base64, or parser trailing data.
func DecodeFrostNativeSignerAnchorTrustCertificate(
	data []byte,
) (*FrostNativeSignerAnchorTrustCertificate, error) {
	if err := frostNativeSignerAnchorTrustPreflightJSON(
		data,
		frostNativeSignerAnchorTrustMaximumCertificateBytes,
		frostNativeSignerAnchorTrustCertificateJSONMembers,
	); err != nil {
		return nil, err
	}
	members := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &members); err != nil {
		return nil, err
	}
	if _, present := members["from"]; !present {
		return nil, fmt.Errorf(
			"native signer anchor trust certificate from endpoint is missing",
		)
	}
	wire := frostNativeSignerAnchorTrustCertificateWire{}
	if err := frostNativeSignerAnchorTrustDecodeJSON(data, &wire); err != nil {
		return nil, err
	}
	return frostNativeSignerAnchorTrustCertificateFromWire(wire)
}

// EncodeFrostNativeSignerAnchorTrustCertificate emits the canonical JSON wire
// representation. Cryptographic validation remains an explicit separate step.
func EncodeFrostNativeSignerAnchorTrustCertificate(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) ([]byte, error) {
	if certificate == nil {
		return nil, fmt.Errorf("native signer anchor trust certificate is nil")
	}
	encoded, err := json.Marshal(
		frostNativeSignerAnchorTrustCertificateToWire(certificate),
	)
	if err != nil {
		return nil, err
	}
	if len(encoded) > frostNativeSignerAnchorTrustMaximumCertificateBytes {
		return nil, fmt.Errorf(
			"native signer anchor trust certificate exceeds the durable record bound",
		)
	}
	return encoded, nil
}

// DecodeFrostNativeSignerAnchorTrustCertificateChain decodes the secure config
// artifact: an exact top-level JSON array containing 1..64 independently
// bounded certificates. The fresh target Read is deliberately obtained later
// and is not part of this at-rest artifact.
func DecodeFrostNativeSignerAnchorTrustCertificateChain(
	data []byte,
) ([]FrostNativeSignerAnchorTrustCertificate, error) {
	if err := frostNativeSignerAnchorTrustPreflightJSONArray(
		data,
		frostNativeSignerAnchorTrustMaximumTransitionRequestBytes,
		frostNativeSignerAnchorTrustCertificateJSONMembers,
	); err != nil {
		return nil, err
	}
	var rawCertificates []json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&rawCertificates); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("JSON contains trailing data")
	}
	if len(rawCertificates) == 0 ||
		len(rawCertificates) >
			FrostNativeSignerAnchorTrustMaximumCertificateChainLength {
		return nil, fmt.Errorf(
			"native signer anchor trust certificate chain length is invalid",
		)
	}
	result := make(
		[]FrostNativeSignerAnchorTrustCertificate,
		len(rawCertificates),
	)
	for index, rawCertificate := range rawCertificates {
		certificate, err := DecodeFrostNativeSignerAnchorTrustCertificate(
			rawCertificate,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer anchor trust certificate [%d]: %w",
				index,
				err,
			)
		}
		result[index] = *certificate
	}
	return result, nil
}

// DecodeFrostNativeSignerAnchorTrustTransitionRequest strictly decodes the
// bounded ABI request and every certificate in its 1..64 chain.
func DecodeFrostNativeSignerAnchorTrustTransitionRequest(
	data []byte,
) (*FrostNativeSignerAnchorTrustTransitionRequest, error) {
	if err := frostNativeSignerAnchorTrustPreflightJSON(
		data,
		frostNativeSignerAnchorTrustMaximumTransitionRequestBytes,
		frostNativeSignerAnchorTrustRequestJSONMembers,
	); err != nil {
		return nil, err
	}
	wire := frostNativeSignerAnchorTrustTransitionRequestWire{}
	if err := frostNativeSignerAnchorTrustDecodeJSON(data, &wire); err != nil {
		return nil, err
	}
	if wire.Schema != FrostNativeSignerAnchorTrustTransitionRequestSchema ||
		wire.CertificateChain == nil ||
		len(*wire.CertificateChain) == 0 ||
		len(*wire.CertificateChain) >
			FrostNativeSignerAnchorTrustMaximumCertificateChainLength {
		return nil, fmt.Errorf("native signer anchor trust transition request is incomplete")
	}
	chain := make([]FrostNativeSignerAnchorTrustCertificate, len(*wire.CertificateChain))
	for index, certificateJSON := range *wire.CertificateChain {
		certificate, err := DecodeFrostNativeSignerAnchorTrustCertificate(
			certificateJSON,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer anchor trust certificate [%d]: %w",
				index,
				err,
			)
		}
		chain[index] = *certificate
	}
	readResponse, err := frostNativeSignerAnchorTrustDecodeBase64JSON(
		wire.TargetReadResponseBase64,
		frostNativeSignerAnchorTrustMaximumReadResponseBytes,
		"target Read response",
	)
	if err != nil {
		return nil, err
	}
	return &FrostNativeSignerAnchorTrustTransitionRequest{
		CertificateChain:   chain,
		TargetReadResponse: readResponse,
	}, nil
}

// EncodeFrostNativeSignerAnchorTrustTransitionRequest emits canonical request
// JSON while retaining exact certificate acknowledgement and Read bytes.
func EncodeFrostNativeSignerAnchorTrustTransitionRequest(
	request *FrostNativeSignerAnchorTrustTransitionRequest,
) ([]byte, error) {
	if request == nil ||
		len(request.CertificateChain) == 0 ||
		len(request.CertificateChain) >
			FrostNativeSignerAnchorTrustMaximumCertificateChainLength {
		return nil, fmt.Errorf("native signer anchor trust transition request is invalid")
	}
	if err := frostNativeSignerAnchorTrustValidateEmbeddedJSON(
		request.TargetReadResponse,
		frostNativeSignerAnchorTrustMaximumReadResponseBytes,
		"target Read response",
	); err != nil {
		return nil, err
	}
	wireChain := make([]json.RawMessage, len(request.CertificateChain))
	for index := range request.CertificateChain {
		encoded, err := EncodeFrostNativeSignerAnchorTrustCertificate(
			&request.CertificateChain[index],
		)
		if err != nil {
			return nil, err
		}
		wireChain[index] = encoded
	}
	wire := frostNativeSignerAnchorTrustTransitionRequestWire{
		Schema:                   FrostNativeSignerAnchorTrustTransitionRequestSchema,
		CertificateChain:         &wireChain,
		TargetReadResponseBase64: base64.StdEncoding.EncodeToString(request.TargetReadResponse),
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if len(encoded) > frostNativeSignerAnchorTrustMaximumTransitionRequestBytes {
		return nil, fmt.Errorf(
			"native signer anchor trust transition request exceeds the FFI bound",
		)
	}
	return encoded, nil
}

// ValidateFrostNativeSignerAnchorTrustCertificate validates all certificate
// structure, fixed transcripts, authority signatures, derived IDs, and exact
// target acknowledgement bytes before invoking the mandatory semantic seam.
func ValidateFrostNativeSignerAnchorTrustCertificate(
	certificate *FrostNativeSignerAnchorTrustCertificate,
	validateTargetAcknowledgement FrostNativeSignerAnchorTrustTargetAcknowledgementValidator,
) error {
	if certificate == nil {
		return fmt.Errorf("native signer anchor trust certificate is nil")
	}
	if validateTargetAcknowledgement == nil {
		return fmt.Errorf(
			"native signer anchor trust target acknowledgement validator is required",
		)
	}
	if certificate.ProtocolID == [32]byte{} ||
		certificate.StreamID == [32]byte{} ||
		certificate.SignerStoreFingerprint == [32]byte{} ||
		certificate.CertificateSequence == 0 {
		return fmt.Errorf("native signer anchor trust certificate identity is incomplete")
	}
	if err := frostNativeSignerAnchorTrustValidateEndpoint(
		&certificate.To,
		certificate.SignerStoreFingerprint,
		"to",
	); err != nil {
		return err
	}

	var authority [ed25519.PublicKeySize]byte
	switch certificate.Kind {
	case FrostNativeSignerAnchorTrustCertificateBootstrap:
		if certificate.From != nil ||
			certificate.CertificateSequence != 1 ||
			certificate.PreviousCertificateDigest != [32]byte{} ||
			certificate.To.Reference.ServiceEpoch != 1 ||
			certificate.To.Reference.Revision != 1 ||
			certificate.To.Reference.PreviousEventRoot != [32]byte{} {
			return fmt.Errorf("native signer anchor bootstrap certificate invariants are invalid")
		}
		authority = certificate.To.OfflineAuthorityPublicKey
	case FrostNativeSignerAnchorTrustCertificateRotation:
		if certificate.From == nil {
			return fmt.Errorf("native signer anchor rotation certificate has no from endpoint")
		}
		if err := frostNativeSignerAnchorTrustValidateEndpoint(
			certificate.From,
			certificate.SignerStoreFingerprint,
			"from",
		); err != nil {
			return err
		}
		if certificate.To.OfflineAuthorityPublicKey !=
			certificate.From.OfflineAuthorityPublicKey ||
			certificate.To.OfflineAuthoritySPKISHA256 !=
				certificate.From.OfflineAuthoritySPKISHA256 {
			return fmt.Errorf("native signer anchor offline authority rotation is unsupported")
		}
		if certificate.From.ActivationManifestSequence == ^uint64(0) ||
			certificate.To.ActivationManifestSequence !=
				certificate.From.ActivationManifestSequence+1 ||
			certificate.From.Reference.ServiceEpoch == ^uint64(0) ||
			certificate.To.Reference.ServiceEpoch !=
				certificate.From.Reference.ServiceEpoch+1 ||
			certificate.To.Reference.Revision != 1 ||
			certificate.To.Reference.PreviousEventRoot !=
				certificate.From.Reference.EventRoot ||
			certificate.To.Reference.Checkpoint !=
				certificate.From.Reference.Checkpoint ||
			certificate.To.WitnessMaximumRecords !=
				certificate.From.WitnessMaximumRecords ||
			certificate.To.WitnessRotationThresholdRecords !=
				certificate.From.WitnessRotationThresholdRecords {
			return fmt.Errorf("native signer anchor rotation transition invariants are invalid")
		}
		if certificate.To.ActivationManifestHash ==
			certificate.From.ActivationManifestHash ||
			certificate.To.BindingHash == certificate.From.BindingHash {
			return fmt.Errorf("native signer anchor rotation does not change its manifest binding")
		}
		switch {
		case certificate.CertificateSequence == 1:
			if certificate.PreviousCertificateDigest != [32]byte{} {
				return fmt.Errorf("legacy-adoption certificate has a previous digest")
			}
		case certificate.PreviousCertificateDigest == [32]byte{}:
			return fmt.Errorf("rotation certificate previous digest is absent")
		}
		authority = certificate.From.OfflineAuthorityPublicKey
	default:
		return fmt.Errorf("native signer anchor trust certificate kind is invalid")
	}

	coreDigest, err := ComputeFrostNativeSignerAnchorTrustCoreDigest(certificate)
	if err != nil || coreDigest != certificate.CoreDigest {
		return fmt.Errorf("native signer anchor trust certificate core digest mismatch")
	}
	if !ed25519.Verify(
		ed25519.PublicKey(authority[:]),
		certificate.CoreDigest[:],
		certificate.CoreSignature[:],
	) {
		return fmt.Errorf("native signer anchor trust certificate core signature is invalid")
	}
	operationID := ComputeFrostNativeSignerAnchorTrustOperationID(
		certificate.CoreDigest,
	)
	if operationID != certificate.OperationID {
		return fmt.Errorf("native signer anchor trust certificate operation ID mismatch")
	}
	transitionDigest := ComputeFrostNativeSignerAnchorTrustTransitionDigest(
		certificate.CoreDigest,
		certificate.OperationID,
	)
	if transitionDigest != certificate.TransitionDigest {
		return fmt.Errorf("native signer anchor trust certificate transition digest mismatch")
	}
	if err := frostNativeSignerAnchorTrustValidateEmbeddedJSON(
		certificate.TargetAcknowledgement,
		frostNativeSignerAnchorTrustMaximumAcknowledgementBytes,
		"target acknowledgement",
	); err != nil {
		return err
	}
	if sha256.Sum256(certificate.TargetAcknowledgement) !=
		certificate.TargetAcknowledgementSHA256 {
		return fmt.Errorf("native signer anchor target acknowledgement hash mismatch")
	}
	finalDigest, err := ComputeFrostNativeSignerAnchorTrustFinalDigest(certificate)
	if err != nil {
		return err
	}
	if !ed25519.Verify(
		ed25519.PublicKey(authority[:]),
		finalDigest[:],
		certificate.FinalSignature[:],
	) {
		return fmt.Errorf("native signer anchor trust certificate final signature is invalid")
	}
	certificateDigest, err :=
		ComputeFrostNativeSignerAnchorTrustCertificateDigest(certificate)
	if err != nil || certificateDigest != certificate.CertificateDigest {
		return fmt.Errorf("native signer anchor trust certificate digest mismatch")
	}
	validatedCertificate := *certificate
	validatedCertificate.TargetAcknowledgement = append(
		[]byte{},
		certificate.TargetAcknowledgement...,
	)
	if certificate.From != nil {
		from := *certificate.From
		validatedCertificate.From = &from
	}
	rawAcknowledgement := append([]byte{}, certificate.TargetAcknowledgement...)
	if err := validateTargetAcknowledgement(
		&validatedCertificate,
		rawAcknowledgement,
	); err != nil {
		return fmt.Errorf(
			"native signer anchor trust target acknowledgement is invalid: %w",
			err,
		)
	}
	return nil
}

// ValidateFrostNativeSignerAnchorTrustCertificateChain validates an exact
// 1..64 missing suffix. Without PriorHead, the suffix must start at sequence 1
// with a bootstrap or explicitly allowed legacy adoption. With PriorHead, its
// first certificate must extend that authenticated head exactly.
func ValidateFrostNativeSignerAnchorTrustCertificateChain(
	chain []FrostNativeSignerAnchorTrustCertificate,
	options FrostNativeSignerAnchorTrustChainValidationOptions,
) error {
	if len(chain) == 0 ||
		len(chain) > FrostNativeSignerAnchorTrustMaximumCertificateChainLength {
		return fmt.Errorf("native signer anchor trust certificate chain length is invalid")
	}
	if err := frostNativeSignerAnchorTrustValidateChainOptions(options); err != nil {
		return err
	}
	exactReplay := options.PriorHead != nil &&
		*options.PriorHead == *options.ExpectedHead
	if exactReplay && len(chain) != 1 {
		return fmt.Errorf(
			"native signer anchor trust exact replay contains additional certificates",
		)
	}
	head := &chain[len(chain)-1]
	if head.CertificateSequence != options.ExpectedHead.CertificateSequence ||
		head.CertificateDigest != options.ExpectedHead.CertificateDigest ||
		head.ProtocolID != options.ExpectedHead.ProtocolID ||
		head.StreamID != options.ExpectedHead.StreamID ||
		head.SignerStoreFingerprint !=
			options.ExpectedHead.SignerStoreFingerprint ||
		head.To != options.ExpectedHead.Endpoint {
		return fmt.Errorf("native signer anchor trust certificate head mismatch")
	}
	for index := range chain {
		certificate := &chain[index]
		if index == 0 {
			if exactReplay {
				// The frozen ABI forbids an empty chain. Revalidate the exact
				// already-installed head as the sole idempotent request item.
			} else if options.PriorHead == nil {
				if certificate.CertificateSequence != 1 ||
					certificate.PreviousCertificateDigest != [32]byte{} {
					return fmt.Errorf(
						"first native signer anchor trust certificate is not sequence one",
					)
				}
				if certificate.Kind ==
					FrostNativeSignerAnchorTrustCertificateRotation &&
					!options.AllowLegacyAdoption {
					return fmt.Errorf(
						"legacy native signer anchor adoption is not authorized",
					)
				}
			} else {
				prior := options.PriorHead
				if prior.CertificateSequence == ^uint64(0) ||
					certificate.Kind !=
						FrostNativeSignerAnchorTrustCertificateRotation ||
					certificate.CertificateSequence !=
						prior.CertificateSequence+1 ||
					certificate.PreviousCertificateDigest !=
						prior.CertificateDigest ||
					certificate.From == nil ||
					certificate.ProtocolID != prior.ProtocolID ||
					certificate.StreamID != prior.StreamID ||
					certificate.SignerStoreFingerprint !=
						prior.SignerStoreFingerprint {
					return fmt.Errorf(
						"first native signer anchor trust certificate does not extend the authenticated prior head",
					)
				}
				if !frostNativeSignerAnchorTrustStaticEndpointEqual(
					*certificate.From,
					prior.Endpoint,
				) {
					return fmt.Errorf(
						"first native signer anchor trust certificate changes the authenticated prior endpoint identity",
					)
				}
				if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
					prior.Endpoint.Reference,
					certificate.From.Reference,
				); err != nil {
					return fmt.Errorf(
						"first native signer anchor trust certificate from reference is not an authenticated prior-floor descendant: %w",
						err,
					)
				}
			}
		} else {
			previous := &chain[index-1]
			if previous.CertificateSequence == ^uint64(0) ||
				certificate.CertificateSequence !=
					previous.CertificateSequence+1 ||
				certificate.Kind !=
					FrostNativeSignerAnchorTrustCertificateRotation ||
				certificate.PreviousCertificateDigest !=
					previous.CertificateDigest ||
				certificate.From == nil ||
				certificate.ProtocolID != previous.ProtocolID ||
				certificate.StreamID != previous.StreamID ||
				certificate.SignerStoreFingerprint !=
					previous.SignerStoreFingerprint {
				return fmt.Errorf(
					"native signer anchor trust certificate [%d] does not extend its exact predecessor",
					index,
				)
			}
			if !frostNativeSignerAnchorTrustStaticEndpointEqual(
				*certificate.From,
				previous.To,
			) {
				return fmt.Errorf(
					"native signer anchor trust certificate [%d] changes its predecessor endpoint identity",
					index,
				)
			}
			if err := frostNativeSignerAnchorTrustValidateReferenceDescendant(
				previous.To.Reference,
				certificate.From.Reference,
			); err != nil {
				return fmt.Errorf(
					"native signer anchor trust certificate [%d] from reference is not a predecessor-floor descendant: %w",
					index,
					err,
				)
			}
		}
		if certificate.ProtocolID != options.ExpectedProtocolID ||
			certificate.StreamID != options.ExpectedStreamID ||
			certificate.SignerStoreFingerprint !=
				options.ExpectedSignerStoreFingerprint ||
			certificate.To.OfflineAuthorityPublicKey !=
				options.ExpectedOfflineAuthorityPublicKey ||
			certificate.To.OfflineAuthoritySPKISHA256 !=
				options.ExpectedOfflineAuthoritySPKISHA256 {
			return fmt.Errorf(
				"native signer anchor trust certificate [%d] pin mismatch",
				index,
			)
		}
		if err := ValidateFrostNativeSignerAnchorTrustCertificate(
			certificate,
			options.ValidateTargetAcknowledgement,
		); err != nil {
			return fmt.Errorf(
				"invalid native signer anchor trust certificate [%d]: %w",
				index,
				err,
			)
		}
	}
	return nil
}

func authenticateFrostNativeSignerAnchorTrustCertificateChain(
	chain []FrostNativeSignerAnchorTrustCertificate,
	options FrostNativeSignerAnchorTrustChainValidationOptions,
) (*frostNativeSignerAnchorVerifiedTrustFloor, error) {
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		chain,
		options,
	); err != nil {
		return nil, err
	}
	certificate := frostNativeSignerAnchorTrustCloneCertificate(
		&chain[len(chain)-1],
	)
	return &frostNativeSignerAnchorVerifiedTrustFloor{
		certificate: certificate,
	}, nil
}

func frostNativeSignerAnchorTrustCloneCertificate(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) FrostNativeSignerAnchorTrustCertificate {
	if certificate == nil {
		return FrostNativeSignerAnchorTrustCertificate{}
	}
	result := *certificate
	result.TargetAcknowledgement = append(
		[]byte{},
		certificate.TargetAcknowledgement...,
	)
	if certificate.From != nil {
		from := *certificate.From
		result.From = &from
	}
	return result
}

func frostNativeSignerAnchorTrustStaticEndpointEqual(
	left FrostNativeSignerAnchorTrustEndpoint,
	right FrostNativeSignerAnchorTrustEndpoint,
) bool {
	return left.ActivationManifestHash == right.ActivationManifestHash &&
		left.ActivationManifestSequence ==
			right.ActivationManifestSequence &&
		left.BindingHash == right.BindingHash &&
		left.ResponsePublicKey == right.ResponsePublicKey &&
		left.ResponsePublicKeySPKISHA256 ==
			right.ResponsePublicKeySPKISHA256 &&
		left.OfflineAuthorityPublicKey ==
			right.OfflineAuthorityPublicKey &&
		left.OfflineAuthoritySPKISHA256 ==
			right.OfflineAuthoritySPKISHA256 &&
		left.WitnessMaximumRecords == right.WitnessMaximumRecords &&
		left.WitnessRotationThresholdRecords ==
			right.WitnessRotationThresholdRecords
}

func frostNativeSignerAnchorTrustValidateReferenceDescendant(
	floor FrostNativeSignerAnchorTrustReference,
	candidate FrostNativeSignerAnchorTrustReference,
) error {
	if floor.Revision != 1 ||
		candidate.ServiceEpoch != floor.ServiceEpoch ||
		candidate.Revision < floor.Revision ||
		candidate.Revision-floor.Revision >
			FrostNativeSignerAnchorMaximumHistoryEvents {
		return fmt.Errorf(
			"reference is outside the restartable certified service-epoch floor",
		)
	}
	if candidate.Revision == floor.Revision {
		if candidate != floor {
			return fmt.Errorf(
				"equal reference revisions identify different events",
			)
		}
		return nil
	}
	// The store fingerprint is checked here to match the Rust engine's
	// descendant validator, which rejects a fingerprint change across
	// generations. Both trees must agree on which references are admissible:
	// where they disagree, one accepts a chain the other refuses on every
	// store open, which is a fail-closed brick with no truncation path back.
	if candidate.PreviousEventRoot == [32]byte{} ||
		candidate.Checkpoint.Generation < floor.Checkpoint.Generation ||
		candidate.Checkpoint.StoreFingerprint !=
			floor.Checkpoint.StoreFingerprint {
		return fmt.Errorf(
			"later reference is unlinked, rolls back its checkpoint " +
				"generation, or changes its store fingerprint",
		)
	}
	if candidate.Checkpoint.Generation == floor.Checkpoint.Generation &&
		candidate.Checkpoint != floor.Checkpoint {
		return fmt.Errorf(
			"later reference forks the certified checkpoint at an equal generation",
		)
	}
	return nil
}

// DecodeAndValidateFrostNativeSignerAnchorTrustTransitionRequest performs the
// complete pure-Go request/chain validation while leaving the fresh outer Read
// wrapper available for the existing contextual verifier.
func DecodeAndValidateFrostNativeSignerAnchorTrustTransitionRequest(
	data []byte,
	options FrostNativeSignerAnchorTrustChainValidationOptions,
) (*FrostNativeSignerAnchorTrustTransitionRequest, error) {
	request, err := DecodeFrostNativeSignerAnchorTrustTransitionRequest(data)
	if err != nil {
		return nil, err
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificateChain(
		request.CertificateChain,
		options,
	); err != nil {
		return nil, err
	}
	return request, nil
}

func frostNativeSignerAnchorTrustValidateChainOptions(
	options FrostNativeSignerAnchorTrustChainValidationOptions,
) error {
	if options.ExpectedProtocolID == [32]byte{} ||
		options.ExpectedStreamID == [32]byte{} ||
		options.ExpectedSignerStoreFingerprint == [32]byte{} ||
		options.ExpectedOfflineAuthorityPublicKey ==
			[ed25519.PublicKeySize]byte{} ||
		options.ExpectedOfflineAuthoritySPKISHA256 == [32]byte{} ||
		options.ExpectedHead == nil ||
		options.ValidateTargetAcknowledgement == nil {
		return fmt.Errorf("native signer anchor trust validation pins are incomplete")
	}
	if ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		options.ExpectedOfflineAuthorityPublicKey,
	) != options.ExpectedOfflineAuthoritySPKISHA256 {
		return fmt.Errorf("native signer anchor trust authority pins are inconsistent")
	}
	if err := frostNativeSignerAnchorTrustValidateHead(
		options.ExpectedHead,
		options,
		"expected",
	); err != nil {
		return err
	}
	if options.PriorHead != nil {
		if err := frostNativeSignerAnchorTrustValidateHead(
			options.PriorHead,
			options,
			"prior",
		); err != nil {
			return err
		}
		if options.PriorHead.CertificateSequence >=
			options.ExpectedHead.CertificateSequence &&
			*options.PriorHead != *options.ExpectedHead {
			return fmt.Errorf(
				"native signer anchor trust prior head does not precede the expected head",
			)
		}
	}
	return nil
}

func frostNativeSignerAnchorTrustValidateHead(
	head *FrostNativeSignerAnchorTrustCertificateHead,
	options FrostNativeSignerAnchorTrustChainValidationOptions,
	name string,
) error {
	if head == nil ||
		head.CertificateSequence == 0 ||
		head.CertificateDigest == [32]byte{} ||
		head.Endpoint.Reference.Revision != 1 ||
		head.ProtocolID != options.ExpectedProtocolID ||
		head.StreamID != options.ExpectedStreamID ||
		head.SignerStoreFingerprint !=
			options.ExpectedSignerStoreFingerprint ||
		head.Endpoint.OfflineAuthorityPublicKey !=
			options.ExpectedOfflineAuthorityPublicKey ||
		head.Endpoint.OfflineAuthoritySPKISHA256 !=
			options.ExpectedOfflineAuthoritySPKISHA256 {
		return fmt.Errorf("native signer anchor trust %s head is incomplete", name)
	}
	if err := frostNativeSignerAnchorTrustValidateEndpoint(
		&head.Endpoint,
		head.SignerStoreFingerprint,
		name+" head",
	); err != nil {
		return err
	}
	return nil
}

func frostNativeSignerAnchorTrustValidateEndpoint(
	endpoint *FrostNativeSignerAnchorTrustEndpoint,
	storeFingerprint [32]byte,
	name string,
) error {
	if endpoint == nil ||
		endpoint.ActivationManifestHash == [32]byte{} ||
		endpoint.ActivationManifestSequence == 0 ||
		endpoint.BindingHash == [32]byte{} ||
		endpoint.ResponsePublicKey == [ed25519.PublicKeySize]byte{} ||
		endpoint.OfflineAuthorityPublicKey ==
			[ed25519.PublicKeySize]byte{} ||
		endpoint.Reference.ServiceEpoch == 0 ||
		endpoint.Reference.Revision == 0 ||
		endpoint.Reference.EventRoot == [32]byte{} ||
		endpoint.Reference.AcknowledgementDigest == [32]byte{} {
		return fmt.Errorf("native signer anchor trust %s endpoint is incomplete", name)
	}
	if ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		endpoint.ResponsePublicKey,
	) != endpoint.ResponsePublicKeySPKISHA256 {
		return fmt.Errorf(
			"native signer anchor trust %s response-key SPKI hash mismatch",
			name,
		)
	}
	if err := frostNativeSignerAnchorTrustValidateEd25519Point(
		endpoint.ResponsePublicKey,
	); err != nil {
		return fmt.Errorf(
			"native signer anchor trust %s response key is invalid: %w",
			name,
			err,
		)
	}
	if endpoint.ResponsePublicKey == endpoint.OfflineAuthorityPublicKey {
		return fmt.Errorf(
			"native signer anchor trust %s response and authority keys are not separated",
			name,
		)
	}
	if ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		endpoint.OfflineAuthorityPublicKey,
	) != endpoint.OfflineAuthoritySPKISHA256 {
		return fmt.Errorf(
			"native signer anchor trust %s authority SPKI hash mismatch",
			name,
		)
	}
	if err := frostNativeSignerAnchorTrustValidateEd25519Point(
		endpoint.OfflineAuthorityPublicKey,
	); err != nil {
		return fmt.Errorf(
			"native signer anchor trust %s authority key is invalid: %w",
			name,
			err,
		)
	}
	if err := frostsigning.ValidateNativeTBTCSignerStateWitnessGeometry(
		endpoint.WitnessMaximumRecords,
		endpoint.WitnessRotationThresholdRecords,
	); err != nil {
		return fmt.Errorf(
			"native signer anchor trust %s witness geometry is invalid: %w",
			name,
			err,
		)
	}
	if err := validateFrostNativeSignerAnchorCheckpoint(
		endpoint.Reference.Checkpoint,
		storeFingerprint,
	); err != nil {
		return fmt.Errorf(
			"native signer anchor trust %s checkpoint is invalid: %w",
			name,
			err,
		)
	}
	return nil
}

func frostNativeSignerAnchorTrustValidateEd25519Point(
	publicKey [ed25519.PublicKeySize]byte,
) error {
	point, err := edwards.ParsePubKey(publicKey[:])
	if err != nil || point == nil ||
		!bytes.Equal(point.Serialize(), publicKey[:]) {
		return fmt.Errorf("non-canonical or off-curve Ed25519 point")
	}

	// Go's crypto/ed25519 verifier accepts the identity public key with the
	// trivial R=identity,S=0 signature, while Rust's ed25519-dalek
	// verify_strict rejects small-order keys. Enforce the stronger common trust
	// boundary explicitly: the key must be a non-identity member of the prime
	// order subgroup. [l]P == identity excludes every non-trivial torsion
	// component on the cofactor-8 Edwards25519 curve.
	curve := edwards.Edwards()
	if point.X.Sign() == 0 &&
		point.Y.Cmp(frostNativeSignerAnchorTrustEd25519IdentityY) == 0 {
		return fmt.Errorf("identity Ed25519 point")
	}
	subgroupX, subgroupY := curve.ScalarMult(
		point.X,
		point.Y,
		curve.Params().N.Bytes(),
	)
	if subgroupX == nil || subgroupY == nil ||
		subgroupX.Sign() != 0 ||
		subgroupY.Cmp(frostNativeSignerAnchorTrustEd25519IdentityY) != 0 {
		return fmt.Errorf("Ed25519 point is not in the prime-order subgroup")
	}
	return nil
}

// ValidateFrostNativeSignerAnchorTrustEd25519PublicKey exposes the exact
// cross-language key predicate used before every trust-boundary Ed25519
// verification. It rejects non-canonical, off-curve, identity, and torsion
// points even when the standard-library verifier would accept a degenerate
// signature for them.
func ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
	publicKey []byte,
) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf(
			"Ed25519 public key length [%d] differs from [%d]",
			len(publicKey),
			ed25519.PublicKeySize,
		)
	}
	var fixed [ed25519.PublicKeySize]byte
	copy(fixed[:], publicKey)
	return frostNativeSignerAnchorTrustValidateEd25519Point(fixed)
}

func frostNativeSignerAnchorTrustKindByte(
	kind FrostNativeSignerAnchorTrustCertificateKind,
) (byte, error) {
	switch kind {
	case FrostNativeSignerAnchorTrustCertificateBootstrap:
		return frostNativeSignerAnchorTrustBootstrapKindByte, nil
	case FrostNativeSignerAnchorTrustCertificateRotation:
		return frostNativeSignerAnchorTrustRotationKindByte, nil
	default:
		return 0, fmt.Errorf("native signer anchor trust certificate kind is invalid")
	}
}

func frostNativeSignerAnchorTrustWriteCoreFrom(
	buffer *bytes.Buffer,
	endpoint FrostNativeSignerAnchorTrustEndpoint,
) {
	frostNativeSignerAnchorTrustWriteEndpointStatic(buffer, endpoint)
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		endpoint.Reference.ServiceEpoch,
	)
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		endpoint.Reference.Revision,
	)
	buffer.Write(endpoint.Reference.PreviousEventRoot[:])
	buffer.Write(endpoint.Reference.EventRoot[:])
	buffer.Write(endpoint.Reference.AcknowledgementDigest[:])
	frostNativeSignerAnchorTrustWriteCheckpoint(
		buffer,
		endpoint.Reference.Checkpoint,
	)
}

func frostNativeSignerAnchorTrustWriteCoreTo(
	buffer *bytes.Buffer,
	endpoint FrostNativeSignerAnchorTrustEndpoint,
) {
	frostNativeSignerAnchorTrustWriteEndpointStatic(buffer, endpoint)
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		endpoint.Reference.ServiceEpoch,
	)
	frostNativeSignerAnchorTrustWriteCheckpoint(
		buffer,
		endpoint.Reference.Checkpoint,
	)
}

func frostNativeSignerAnchorTrustWriteEndpointStatic(
	buffer *bytes.Buffer,
	endpoint FrostNativeSignerAnchorTrustEndpoint,
) {
	buffer.Write(endpoint.ActivationManifestHash[:])
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		endpoint.ActivationManifestSequence,
	)
	buffer.Write(endpoint.BindingHash[:])
	buffer.Write(endpoint.ResponsePublicKey[:])
	buffer.Write(endpoint.ResponsePublicKeySPKISHA256[:])
	buffer.Write(endpoint.OfflineAuthorityPublicKey[:])
	buffer.Write(endpoint.OfflineAuthoritySPKISHA256[:])
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		endpoint.WitnessMaximumRecords,
	)
	frostNativeSignerAnchorTrustWriteUint64(
		buffer,
		endpoint.WitnessRotationThresholdRecords,
	)
}

func frostNativeSignerAnchorTrustWriteCheckpoint(
	buffer *bytes.Buffer,
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) {
	buffer.Write(checkpoint.StoreFingerprint[:])
	frostNativeSignerAnchorTrustWriteUint64(buffer, checkpoint.Generation)
	buffer.Write(checkpoint.PreviousStateCommitment[:])
	buffer.Write(checkpoint.StateImageDigest[:])
	buffer.Write(checkpoint.StateCommitment[:])
}

func frostNativeSignerAnchorTrustWriteUint64(
	buffer *bytes.Buffer,
	value uint64,
) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func frostNativeSignerAnchorTrustCertificateFromWire(
	wire frostNativeSignerAnchorTrustCertificateWire,
) (*FrostNativeSignerAnchorTrustCertificate, error) {
	if wire.Schema != FrostNativeSignerAnchorTrustCertificateSchema ||
		wire.To == nil {
		return nil, fmt.Errorf("native signer anchor trust certificate is incomplete")
	}
	certificate := &FrostNativeSignerAnchorTrustCertificate{
		Kind: FrostNativeSignerAnchorTrustCertificateKind(wire.Kind),
	}
	if _, err := frostNativeSignerAnchorTrustKindByte(certificate.Kind); err != nil {
		return nil, err
	}
	var err error
	if certificate.CertificateSequence, err =
		frostNativeSignerAnchorParseUint64(wire.CertificateSequence); err != nil {
		return nil, fmt.Errorf("invalid certificate sequence: %w", err)
	}
	bytes32Fields := []struct {
		name        string
		value       string
		destination *[32]byte
	}{
		{"previous certificate digest", wire.PreviousCertificateDigest, &certificate.PreviousCertificateDigest},
		{"protocol ID", wire.ProtocolID, &certificate.ProtocolID},
		{"stream ID", wire.StreamID, &certificate.StreamID},
		{"signer store fingerprint", wire.SignerStoreFingerprint, &certificate.SignerStoreFingerprint},
		{"core digest", wire.CoreDigest, &certificate.CoreDigest},
		{"operation ID", wire.OperationID, &certificate.OperationID},
		{"transition digest", wire.TransitionDigest, &certificate.TransitionDigest},
		{"target acknowledgement SHA-256", wire.TargetAcknowledgementSHA256, &certificate.TargetAcknowledgementSHA256},
		{"certificate digest", wire.CertificateDigest, &certificate.CertificateDigest},
	}
	for _, field := range bytes32Fields {
		value, err := frostNativeSignerAnchorParseHex32(field.value)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", field.name, err)
		}
		*field.destination = value
	}
	if certificate.CoreSignature, err =
		frostNativeSignerAnchorTrustDecodeBase64Signature(wire.CoreSignature); err != nil {
		return nil, fmt.Errorf("invalid core signature: %w", err)
	}
	if certificate.FinalSignature, err =
		frostNativeSignerAnchorTrustDecodeBase64Signature(wire.FinalSignature); err != nil {
		return nil, fmt.Errorf("invalid final signature: %w", err)
	}
	if wire.From != nil {
		from, err := frostNativeSignerAnchorTrustEndpointFromWire(*wire.From)
		if err != nil {
			return nil, fmt.Errorf("invalid from endpoint: %w", err)
		}
		certificate.From = &from
	}
	if (certificate.Kind == FrostNativeSignerAnchorTrustCertificateBootstrap &&
		certificate.From != nil) ||
		(certificate.Kind == FrostNativeSignerAnchorTrustCertificateRotation &&
			certificate.From == nil) {
		return nil, fmt.Errorf(
			"native signer anchor trust certificate from endpoint does not match its kind",
		)
	}
	certificate.To, err = frostNativeSignerAnchorTrustEndpointFromWire(*wire.To)
	if err != nil {
		return nil, fmt.Errorf("invalid to endpoint: %w", err)
	}
	certificate.TargetAcknowledgement, err =
		frostNativeSignerAnchorTrustDecodeBase64JSON(
			wire.TargetAcknowledgementBase64,
			frostNativeSignerAnchorTrustMaximumAcknowledgementBytes,
			"target acknowledgement",
		)
	if err != nil {
		return nil, err
	}
	return certificate, nil
}

func frostNativeSignerAnchorTrustEndpointFromWire(
	wire frostNativeSignerAnchorTrustEndpointWire,
) (FrostNativeSignerAnchorTrustEndpoint, error) {
	if wire.Reference == nil {
		return FrostNativeSignerAnchorTrustEndpoint{}, fmt.Errorf("endpoint reference is absent")
	}
	result := FrostNativeSignerAnchorTrustEndpoint{}
	var err error
	if result.ActivationManifestSequence, err =
		frostNativeSignerAnchorParseUint64(wire.ActivationManifestSequence); err != nil {
		return result, fmt.Errorf("invalid activation manifest sequence: %w", err)
	}
	if result.WitnessMaximumRecords, err =
		frostNativeSignerAnchorParseUint64(wire.WitnessMaximumRecords); err != nil {
		return result, fmt.Errorf("invalid witness maximum records: %w", err)
	}
	if result.WitnessRotationThresholdRecords, err =
		frostNativeSignerAnchorParseUint64(
			wire.WitnessRotationThresholdRecords,
		); err != nil {
		return result, fmt.Errorf("invalid witness rotation threshold: %w", err)
	}
	fields := []struct {
		name        string
		value       string
		destination *[32]byte
	}{
		{"activation manifest hash", wire.ActivationManifestHash, &result.ActivationManifestHash},
		{"binding hash", wire.BindingHash, &result.BindingHash},
		{"response public key", wire.ResponsePublicKey, &result.ResponsePublicKey},
		{"response public-key SPKI SHA-256", wire.ResponsePublicKeySPKISHA256, &result.ResponsePublicKeySPKISHA256},
		{"offline authority public key", wire.OfflineAuthorityPublicKey, &result.OfflineAuthorityPublicKey},
		{"offline authority SPKI SHA-256", wire.OfflineAuthoritySPKISHA256, &result.OfflineAuthoritySPKISHA256},
	}
	for _, field := range fields {
		value, err := frostNativeSignerAnchorParseHex32(field.value)
		if err != nil {
			return result, fmt.Errorf("invalid %s: %w", field.name, err)
		}
		*field.destination = value
	}
	result.Reference, err =
		frostNativeSignerAnchorTrustReferenceFromWire(*wire.Reference)
	if err != nil {
		return result, err
	}
	return result, nil
}

func frostNativeSignerAnchorTrustReferenceFromWire(
	wire frostNativeSignerAnchorTrustReferenceWire,
) (FrostNativeSignerAnchorTrustReference, error) {
	if wire.Checkpoint == nil {
		return FrostNativeSignerAnchorTrustReference{}, fmt.Errorf(
			"trust reference checkpoint is absent",
		)
	}
	result := FrostNativeSignerAnchorTrustReference{}
	var err error
	if result.ServiceEpoch, err =
		frostNativeSignerAnchorParseUint64(wire.ServiceEpoch); err != nil {
		return result, fmt.Errorf("invalid service epoch: %w", err)
	}
	if result.Revision, err =
		frostNativeSignerAnchorParseUint64(wire.Revision); err != nil {
		return result, fmt.Errorf("invalid revision: %w", err)
	}
	fields := []struct {
		name        string
		value       string
		destination *[32]byte
	}{
		{"previous event root", wire.PreviousEventRoot, &result.PreviousEventRoot},
		{"event root", wire.EventRoot, &result.EventRoot},
		{"checkpoint acknowledgement digest", wire.CheckpointAckDigest, &result.AcknowledgementDigest},
	}
	for _, field := range fields {
		value, err := frostNativeSignerAnchorParseHex32(field.value)
		if err != nil {
			return result, fmt.Errorf("invalid %s: %w", field.name, err)
		}
		*field.destination = value
	}
	result.Checkpoint, err =
		frostNativeSignerAnchorTrustCheckpointFromWire(*wire.Checkpoint)
	if err != nil {
		return result, err
	}
	return result, nil
}

func frostNativeSignerAnchorTrustCheckpointFromWire(
	wire frostNativeSignerAnchorTrustCheckpointWire,
) (FrostNativeSignerStateWitnessCheckpoint, error) {
	return frostNativeSignerAnchorCheckpointFromWire(
		frostNativeSignerAnchorCheckpointWire{
			StoreFingerprint:        wire.StoreFingerprint,
			Generation:              wire.Generation,
			PreviousStateCommitment: wire.PreviousStateCommitment,
			StateImageDigest:        wire.StateImageDigest,
			StateCommitment:         wire.StateCommitment,
		},
	)
}

func frostNativeSignerAnchorTrustCertificateToWire(
	certificate *FrostNativeSignerAnchorTrustCertificate,
) frostNativeSignerAnchorTrustCertificateWire {
	var from *frostNativeSignerAnchorTrustEndpointWire
	if certificate.From != nil {
		value := frostNativeSignerAnchorTrustEndpointToWire(*certificate.From)
		from = &value
	}
	to := frostNativeSignerAnchorTrustEndpointToWire(certificate.To)
	return frostNativeSignerAnchorTrustCertificateWire{
		Schema:                      FrostNativeSignerAnchorTrustCertificateSchema,
		Kind:                        string(certificate.Kind),
		CertificateSequence:         fmt.Sprint(certificate.CertificateSequence),
		PreviousCertificateDigest:   frostNativeSignerAnchorHex32(certificate.PreviousCertificateDigest),
		ProtocolID:                  frostNativeSignerAnchorHex32(certificate.ProtocolID),
		StreamID:                    frostNativeSignerAnchorHex32(certificate.StreamID),
		SignerStoreFingerprint:      frostNativeSignerAnchorHex32(certificate.SignerStoreFingerprint),
		From:                        from,
		To:                          &to,
		CoreDigest:                  frostNativeSignerAnchorHex32(certificate.CoreDigest),
		CoreSignature:               base64.StdEncoding.EncodeToString(certificate.CoreSignature[:]),
		OperationID:                 frostNativeSignerAnchorHex32(certificate.OperationID),
		TransitionDigest:            frostNativeSignerAnchorHex32(certificate.TransitionDigest),
		TargetAcknowledgementBase64: base64.StdEncoding.EncodeToString(certificate.TargetAcknowledgement),
		TargetAcknowledgementSHA256: frostNativeSignerAnchorHex32(certificate.TargetAcknowledgementSHA256),
		FinalSignature:              base64.StdEncoding.EncodeToString(certificate.FinalSignature[:]),
		CertificateDigest:           frostNativeSignerAnchorHex32(certificate.CertificateDigest),
	}
}

func frostNativeSignerAnchorTrustEndpointToWire(
	endpoint FrostNativeSignerAnchorTrustEndpoint,
) frostNativeSignerAnchorTrustEndpointWire {
	reference := frostNativeSignerAnchorTrustReferenceToWire(endpoint.Reference)
	return frostNativeSignerAnchorTrustEndpointWire{
		ActivationManifestHash:          frostNativeSignerAnchorHex32(endpoint.ActivationManifestHash),
		ActivationManifestSequence:      fmt.Sprint(endpoint.ActivationManifestSequence),
		BindingHash:                     frostNativeSignerAnchorHex32(endpoint.BindingHash),
		ResponsePublicKey:               frostNativeSignerAnchorHex32(endpoint.ResponsePublicKey),
		ResponsePublicKeySPKISHA256:     frostNativeSignerAnchorHex32(endpoint.ResponsePublicKeySPKISHA256),
		OfflineAuthorityPublicKey:       frostNativeSignerAnchorHex32(endpoint.OfflineAuthorityPublicKey),
		OfflineAuthoritySPKISHA256:      frostNativeSignerAnchorHex32(endpoint.OfflineAuthoritySPKISHA256),
		WitnessMaximumRecords:           fmt.Sprint(endpoint.WitnessMaximumRecords),
		WitnessRotationThresholdRecords: fmt.Sprint(endpoint.WitnessRotationThresholdRecords),
		Reference:                       &reference,
	}
}

func frostNativeSignerAnchorTrustReferenceToWire(
	reference FrostNativeSignerAnchorTrustReference,
) frostNativeSignerAnchorTrustReferenceWire {
	checkpoint := frostNativeSignerAnchorTrustCheckpointToWire(reference.Checkpoint)
	return frostNativeSignerAnchorTrustReferenceWire{
		ServiceEpoch:        fmt.Sprint(reference.ServiceEpoch),
		Revision:            fmt.Sprint(reference.Revision),
		PreviousEventRoot:   frostNativeSignerAnchorHex32(reference.PreviousEventRoot),
		EventRoot:           frostNativeSignerAnchorHex32(reference.EventRoot),
		CheckpointAckDigest: frostNativeSignerAnchorHex32(reference.AcknowledgementDigest),
		Checkpoint:          &checkpoint,
	}
}

func frostNativeSignerAnchorTrustCheckpointToWire(
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) frostNativeSignerAnchorTrustCheckpointWire {
	return frostNativeSignerAnchorTrustCheckpointWire{
		StoreFingerprint:        frostNativeSignerAnchorHex32(checkpoint.StoreFingerprint),
		Generation:              fmt.Sprint(checkpoint.Generation),
		PreviousStateCommitment: frostNativeSignerAnchorHex32(checkpoint.PreviousStateCommitment),
		StateImageDigest:        frostNativeSignerAnchorHex32(checkpoint.StateImageDigest),
		StateCommitment:         frostNativeSignerAnchorHex32(checkpoint.StateCommitment),
	}
}

func frostNativeSignerAnchorTrustDecodeBase64JSON(
	value string,
	maximum int,
	name string,
) ([]byte, error) {
	if value == "" {
		return nil, fmt.Errorf("native signer anchor %s is absent", name)
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, fmt.Errorf(
			"native signer anchor %s is not canonical padded base64",
			name,
		)
	}
	if err := frostNativeSignerAnchorTrustValidateEmbeddedJSON(
		decoded,
		maximum,
		name,
	); err != nil {
		return nil, err
	}
	return decoded, nil
}

func frostNativeSignerAnchorTrustDecodeBase64Signature(
	value string,
) ([ed25519.SignatureSize]byte, error) {
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil ||
		len(decoded) != ed25519.SignatureSize ||
		base64.StdEncoding.EncodeToString(decoded) != value {
		return [ed25519.SignatureSize]byte{}, fmt.Errorf(
			"signature is not canonical padded base64 bytes64",
		)
	}
	var result [ed25519.SignatureSize]byte
	copy(result[:], decoded)
	return result, nil
}

func frostNativeSignerAnchorTrustValidateEmbeddedJSON(
	data []byte,
	maximum int,
	name string,
) error {
	if len(data) == 0 || len(data) > maximum || !utf8.Valid(data) {
		return fmt.Errorf("native signer anchor %s size or UTF-8 is invalid", name)
	}
	if err := frostNativeSignerAnchorTrustPreflightJSON(
		data,
		maximum,
		nil,
	); err != nil {
		return fmt.Errorf("invalid native signer anchor %s JSON: %w", name, err)
	}
	return nil
}

func frostNativeSignerAnchorTrustDecodeJSON(
	data []byte,
	target interface{},
) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON contains trailing data")
	}
	return nil
}

func frostNativeSignerAnchorTrustPreflightJSON(
	data []byte,
	maximum int,
	allowedMembers map[string]struct{},
) error {
	if len(data) == 0 || len(data) > maximum || !utf8.Valid(data) {
		return fmt.Errorf("native signer anchor trust JSON size or UTF-8 is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("native signer anchor trust JSON must be one object")
	}
	if err := frostNativeSignerAnchorTrustScanJSONObject(
		decoder,
		0,
		allowedMembers,
	); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON contains trailing data")
		}
		return fmt.Errorf("invalid JSON trailing data: %w", err)
	}
	return nil
}

func frostNativeSignerAnchorTrustPreflightJSONArray(
	data []byte,
	maximum int,
	allowedMembers map[string]struct{},
) error {
	if len(data) == 0 || len(data) > maximum || !utf8.Valid(data) {
		return fmt.Errorf("native signer anchor trust JSON size or UTF-8 is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return fmt.Errorf("native signer anchor trust chain JSON must be one array")
	}
	for decoder.More() {
		if err := frostNativeSignerAnchorTrustScanJSONValue(
			decoder,
			1,
			allowedMembers,
		); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return fmt.Errorf("invalid JSON array termination")
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON contains trailing data")
		}
		return fmt.Errorf("invalid JSON trailing data: %w", err)
	}
	return nil
}

func frostNativeSignerAnchorTrustScanJSONObject(
	decoder *json.Decoder,
	depth int,
	allowedMembers map[string]struct{},
) error {
	if depth > frostNativeSignerAnchorTrustMaximumJSONDepth {
		return fmt.Errorf("JSON nesting exceeds the depth bound")
	}
	seen := make(map[string]struct{})
	seenFolded := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("invalid JSON object member: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok || !frostNativeSignerAnchorTrustASCIIJSONMember(key) {
			return fmt.Errorf("JSON object member name is not canonical ASCII")
		}
		if allowedMembers != nil {
			if _, allowed := allowedMembers[key]; !allowed {
				return fmt.Errorf("JSON object member name [%s] is not exact", key)
			}
		}
		folded := strings.ToLower(key)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("JSON object contains duplicate member [%s]", key)
		}
		if _, duplicate := seenFolded[folded]; duplicate {
			return fmt.Errorf(
				"JSON object contains case-folded duplicate member [%s]",
				key,
			)
		}
		seen[key] = struct{}{}
		seenFolded[folded] = struct{}{}
		if err := frostNativeSignerAnchorTrustScanJSONValue(
			decoder,
			depth+1,
			allowedMembers,
		); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return fmt.Errorf("invalid JSON object termination")
	}
	return nil
}

func frostNativeSignerAnchorTrustScanJSONValue(
	decoder *json.Decoder,
	depth int,
	allowedMembers map[string]struct{},
) error {
	if depth > frostNativeSignerAnchorTrustMaximumJSONDepth {
		return fmt.Errorf("JSON nesting exceeds the depth bound")
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		return frostNativeSignerAnchorTrustScanJSONObject(
			decoder,
			depth,
			allowedMembers,
		)
	case '[':
		for decoder.More() {
			if err := frostNativeSignerAnchorTrustScanJSONValue(
				decoder,
				depth+1,
				allowedMembers,
			); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("invalid JSON array termination")
		}
		return nil
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
}

func frostNativeSignerAnchorTrustASCIIJSONMember(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

var frostNativeSignerAnchorTrustCertificateJSONMembers = map[string]struct{}{
	"schema":                          {},
	"kind":                            {},
	"certificateSequence":             {},
	"previousCertificateDigest":       {},
	"protocolID":                      {},
	"streamID":                        {},
	"signerStoreFingerprint":          {},
	"from":                            {},
	"to":                              {},
	"coreDigest":                      {},
	"coreSignature":                   {},
	"operationID":                     {},
	"transitionDigest":                {},
	"targetAcknowledgementBase64":     {},
	"targetAcknowledgementSHA256":     {},
	"finalSignature":                  {},
	"certificateDigest":               {},
	"activationManifestHash":          {},
	"activationManifestSequence":      {},
	"bindingHash":                     {},
	"responsePublicKey":               {},
	"responsePublicKeySpkiSha256":     {},
	"offlineAuthorityPublicKey":       {},
	"offlineAuthoritySpkiSha256":      {},
	"witnessMaximumRecords":           {},
	"witnessRotationThresholdRecords": {},
	"reference":                       {},
	"serviceEpoch":                    {},
	"revision":                        {},
	"previousEventRoot":               {},
	"eventRoot":                       {},
	"checkpointAckDigest":             {},
	"checkpoint":                      {},
	"storeFingerprint":                {},
	"generation":                      {},
	"previousStateCommitment":         {},
	"stateImageDigest":                {},
	"stateCommitment":                 {},
}

var frostNativeSignerAnchorTrustRequestJSONMembers = func() map[string]struct{} {
	result := make(
		map[string]struct{},
		len(frostNativeSignerAnchorTrustCertificateJSONMembers)+3,
	)
	for name := range frostNativeSignerAnchorTrustCertificateJSONMembers {
		result[name] = struct{}{}
	}
	result["certificateChain"] = struct{}{}
	result["targetReadResponseBase64"] = struct{}{}
	return result
}()

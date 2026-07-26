package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const (
	FrostNativeSignerAnchorBootstrapPlanSchema              = "tbtc-frost-native-signer-state-anchor-bootstrap-plan/v1"
	FrostNativeSignerAnchorBootstrapCoreArtifactSchema      = "tbtc-frost-native-signer-state-anchor-bootstrap-core-signing-request/v1"
	FrostNativeSignerAnchorBootstrapDetachedSignatureSchema = "tbtc-frost-native-signer-state-anchor-bootstrap-detached-signature/v1"
	FrostNativeSignerAnchorBootstrapFinalArtifactSchema     = "tbtc-frost-native-signer-state-anchor-bootstrap-final-signing-request/v1"
	FrostNativeSignerAnchorBootstrapOutputBundleSchema      = "tbtc-frost-native-signer-state-anchor-bootstrap-output-bundle/v1"
)

type FrostNativeSignerAnchorBootstrapSignatureStage string

const (
	FrostNativeSignerAnchorBootstrapCoreSignatureStage  FrostNativeSignerAnchorBootstrapSignatureStage = "core"
	FrostNativeSignerAnchorBootstrapFinalSignatureStage FrostNativeSignerAnchorBootstrapSignatureStage = "final"
)

// FrostNativeSignerAnchorBootstrapPlan is the public activation projection
// consumed by the offline ceremony. Its fields are not trusted merely because
// they appear here: PrepareFrostNativeSignerAnchorBootstrapCore re-derives
// every stream, transport, binding, SPKI, store, and checkpoint relationship,
// and the offline authority subsequently signs the resulting fixed transcript.
type FrostNativeSignerAnchorBootstrapPlan struct {
	Schema                    string
	Endpoint                  string
	Identity                  FrostNativeSignerAnchorIdentity
	ResponsePublicKey         [ed25519.PublicKeySize]byte
	OfflineAuthorityPublicKey [ed25519.PublicKeySize]byte
}

// FrostNativeSignerAnchorBootstrapCoreArtifact is the immutable public input
// to the first offline signature. It contains no private key and no service
// result.
type FrostNativeSignerAnchorBootstrapCoreArtifact struct {
	Schema           string
	Plan             FrostNativeSignerAnchorBootstrapPlan
	Checkpoint       FrostNativeSignerStateWitnessCheckpoint
	FactsSHA256      [32]byte
	CoreDigest       [32]byte
	OperationID      [32]byte
	TransitionDigest [32]byte
}

// FrostNativeSignerAnchorBootstrapDetachedSignature carries a signature
// created outside the online ceremony process. Stage and Digest prevent a
// valid core signature from being accidentally accepted as the final
// signature or vice versa.
type FrostNativeSignerAnchorBootstrapDetachedSignature struct {
	Schema    string
	Stage     FrostNativeSignerAnchorBootstrapSignatureStage
	Digest    [32]byte
	Signature [ed25519.SignatureSize]byte
}

// FrostNativeSignerAnchorBootstrapFinalArtifact contains the exact service
// acknowledgement ratified by the second offline signature.
type FrostNativeSignerAnchorBootstrapFinalArtifact struct {
	Schema                      string
	Core                        FrostNativeSignerAnchorBootstrapCoreArtifact
	CoreSignature               [ed25519.SignatureSize]byte
	TargetReference             FrostNativeSignerAnchorTrustReference
	TargetAcknowledgement       []byte
	TargetAcknowledgementSHA256 [32]byte
	FinalDigest                 [32]byte
}

// FrostNativeSignerAnchorBootstrapOutputBundle is the parsed certified
// ceremony output. CertificateChainJSON and SignerConfigJSON retain the exact
// bundle bytes whose SHA-256 digests the bundle itself commits to.
type FrostNativeSignerAnchorBootstrapOutputBundle struct {
	Schema               string
	CertificateDigest    [32]byte
	CertificateChain     []FrostNativeSignerAnchorTrustCertificate
	CertificateChainJSON []byte
	SignerConfigJSON     []byte
}

// FrostNativeSignerAnchorBootstrapAuthorization is the only object passed to
// the online service client. It contains a detached offline signature, never
// the offline private key.
type FrostNativeSignerAnchorBootstrapAuthorization struct {
	Certificate FrostNativeSignerAnchorTrustCertificate
}

// FrostNativeSignerAnchorBootstrapClientResult must come from an authenticated
// create-if-absent operation followed by a fresh signed Read of the exact
// stored event. ReadRecoveryJSON makes that reconciliation explicit.
type FrostNativeSignerAnchorBootstrapClientResult struct {
	Record *FrostNativeSignerStateWitnessAnchorRecord
}

// FrostNativeSignerAnchorBootstrapClient is deliberately narrower than the
// runtime anchor store. A separate transport implementation supplies the
// create-if-absent endpoint without weakening ordinary Read/CAS semantics.
type FrostNativeSignerAnchorBootstrapClient interface {
	InitializeFrostNativeSignerAnchor(
		context.Context,
		FrostNativeSignerAnchorBootstrapAuthorization,
	) (*FrostNativeSignerAnchorBootstrapClientResult, error)
}

type frostNativeSignerAnchorBootstrapPlanWire struct {
	Schema                    string                              `json:"schema"`
	Endpoint                  string                              `json:"endpoint"`
	Identity                  frostNativeSignerAnchorIdentityWire `json:"identity"`
	ResponsePublicKey         string                              `json:"responsePublicKey"`
	OfflineAuthorityPublicKey string                              `json:"offlineAuthorityPublicKey"`
}

type frostNativeSignerAnchorBootstrapCoreArtifactWire struct {
	Schema           string                                   `json:"schema"`
	Plan             frostNativeSignerAnchorBootstrapPlanWire `json:"plan"`
	Checkpoint       frostNativeSignerAnchorCheckpointWire    `json:"checkpoint"`
	FactsSHA256      string                                   `json:"factsSHA256"`
	CoreDigest       string                                   `json:"coreDigest"`
	OperationID      string                                   `json:"operationID"`
	TransitionDigest string                                   `json:"transitionDigest"`
}

type frostNativeSignerAnchorBootstrapDetachedSignatureWire struct {
	Schema    string `json:"schema"`
	Stage     string `json:"stage"`
	Digest    string `json:"digest"`
	Signature string `json:"signature"`
}

type frostNativeSignerAnchorBootstrapFinalArtifactWire struct {
	Schema                      string                                           `json:"schema"`
	Core                        frostNativeSignerAnchorBootstrapCoreArtifactWire `json:"core"`
	CoreSignature               string                                           `json:"coreSignature"`
	TargetReference             frostNativeSignerAnchorTrustReferenceWire        `json:"targetReference"`
	TargetAcknowledgementBase64 string                                           `json:"targetAcknowledgementBase64"`
	TargetAcknowledgementSHA256 string                                           `json:"targetAcknowledgementSHA256"`
	FinalDigest                 string                                           `json:"finalDigest"`
}

type frostNativeSignerAnchorBootstrapOutputBundleWire struct {
	Schema                 string          `json:"schema"`
	CertificateDigest      string          `json:"certificateDigest"`
	CertificateChainSHA256 string          `json:"certificateChainSHA256"`
	SignerConfigSHA256     string          `json:"signerConfigSHA256"`
	CertificateChain       json.RawMessage `json:"certificateChain"`
	SignerConfig           json.RawMessage `json:"signerConfig"`
}

// PrepareFrostNativeSignerAnchorBootstrapCore constructs and validates the
// first fixed-width offline signing transcript.
func PrepareFrostNativeSignerAnchorBootstrapCore(
	facts *frostsigning.NativeTBTCSignerStateAnchorBootstrapFacts,
	plan *FrostNativeSignerAnchorBootstrapPlan,
) (*FrostNativeSignerAnchorBootstrapCoreArtifact, error) {
	if facts == nil || plan == nil {
		return nil, fmt.Errorf("native signer anchor bootstrap facts or plan are nil")
	}
	factsJSON, err :=
		frostsigning.EncodeNativeTBTCSignerStateAnchorBootstrapFacts(facts)
	if err != nil {
		return nil, err
	}
	if err := validateFrostNativeSignerAnchorBootstrapPlan(plan); err != nil {
		return nil, err
	}
	checkpoint := frostNativeSignerAnchorBootstrapCheckpoint(facts.CurrentCheckpoint)
	if checkpoint.StoreFingerprint != plan.Identity.SignerStoreFingerprint {
		return nil, fmt.Errorf(
			"native signer bootstrap facts differ from the activation plan store",
		)
	}
	certificate := frostNativeSignerAnchorBootstrapCoreCertificate(
		plan,
		checkpoint,
	)
	coreDigest, err := ComputeFrostNativeSignerAnchorTrustCoreDigest(&certificate)
	if err != nil {
		return nil, err
	}
	operationID := ComputeFrostNativeSignerAnchorTrustOperationID(coreDigest)
	transitionDigest := ComputeFrostNativeSignerAnchorTrustTransitionDigest(
		coreDigest,
		operationID,
	)
	result := &FrostNativeSignerAnchorBootstrapCoreArtifact{
		Schema:           FrostNativeSignerAnchorBootstrapCoreArtifactSchema,
		Plan:             *plan,
		Checkpoint:       checkpoint,
		FactsSHA256:      sha256.Sum256(factsJSON),
		CoreDigest:       coreDigest,
		OperationID:      operationID,
		TransitionDigest: transitionDigest,
	}
	if err := validateFrostNativeSignerAnchorBootstrapCore(result); err != nil {
		return nil, err
	}
	return result, nil
}

// InitializeFrostNativeSignerAnchorBootstrap verifies the detached core
// authorization, invokes the separate online bootstrap transport, requires its
// fresh-Read reconciliation record, and prepares the second offline digest.
func InitializeFrostNativeSignerAnchorBootstrap(
	ctx context.Context,
	core *FrostNativeSignerAnchorBootstrapCoreArtifact,
	signature *FrostNativeSignerAnchorBootstrapDetachedSignature,
	client FrostNativeSignerAnchorBootstrapClient,
) (*FrostNativeSignerAnchorBootstrapFinalArtifact, error) {
	if ctx == nil || client == nil {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap context or client is nil",
		)
	}
	if err := validateFrostNativeSignerAnchorBootstrapCore(core); err != nil {
		return nil, err
	}
	if err := validateFrostNativeSignerAnchorBootstrapSignature(
		signature,
		FrostNativeSignerAnchorBootstrapCoreSignatureStage,
		core.CoreDigest,
		core.Plan.OfflineAuthorityPublicKey,
	); err != nil {
		return nil, err
	}
	certificate := frostNativeSignerAnchorBootstrapCoreCertificate(
		&core.Plan,
		core.Checkpoint,
	)
	certificate.CoreDigest = core.CoreDigest
	certificate.CoreSignature = signature.Signature
	certificate.OperationID = core.OperationID
	certificate.TransitionDigest = core.TransitionDigest

	result, err := client.InitializeFrostNativeSignerAnchor(
		ctx,
		FrostNativeSignerAnchorBootstrapAuthorization{
			Certificate: certificate,
		},
	)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Record == nil ||
		len(result.Record.AcknowledgementJSON) == 0 ||
		len(result.Record.ReadRecoveryJSON) == 0 ||
		result.Record.ReadRecoveryExpires == 0 {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap client did not return a fresh reconciled record",
		)
	}
	record := result.Record
	reference := FrostNativeSignerAnchorTrustReference{
		ServiceEpoch:          record.ServiceEpoch,
		Revision:              record.Revision,
		PreviousEventRoot:     record.PreviousEventRoot,
		EventRoot:             record.EventRoot,
		AcknowledgementDigest: record.AcknowledgementDigest,
		Checkpoint:            record.Checkpoint,
	}
	certificate.To.Reference = reference
	certificate.TargetAcknowledgement = append(
		[]byte{},
		record.AcknowledgementJSON...,
	)
	certificate.TargetAcknowledgementSHA256 = sha256.Sum256(
		certificate.TargetAcknowledgement,
	)
	if record.BindingHash != certificate.To.BindingHash ||
		record.OperationID != certificate.OperationID ||
		record.TransitionDigest != certificate.TransitionDigest ||
		record.Checkpoint != core.Checkpoint ||
		record.ServiceEpoch != 1 ||
		record.Revision != 1 ||
		record.PreviousEventRoot != [32]byte{} {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap reconciled record differs from its offline core",
		)
	}
	if err := ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement(
		&certificate,
		certificate.TargetAcknowledgement,
	); err != nil {
		return nil, err
	}
	finalDigest, err := ComputeFrostNativeSignerAnchorTrustFinalDigest(
		&certificate,
	)
	if err != nil {
		return nil, err
	}
	return &FrostNativeSignerAnchorBootstrapFinalArtifact{
		Schema:                      FrostNativeSignerAnchorBootstrapFinalArtifactSchema,
		Core:                        *core,
		CoreSignature:               signature.Signature,
		TargetReference:             reference,
		TargetAcknowledgement:       certificate.TargetAcknowledgement,
		TargetAcknowledgementSHA256: certificate.TargetAcknowledgementSHA256,
		FinalDigest:                 finalDigest,
	}, nil
}

// FinalizeFrostNativeSignerAnchorBootstrap validates the second detached
// signature and emits one atomic bundle containing the canonical one-element
// certificate chain and complete normal-signer init config. The offline
// private key is never accepted by this API.
func FinalizeFrostNativeSignerAnchorBootstrap(
	final *FrostNativeSignerAnchorBootstrapFinalArtifact,
	signature *FrostNativeSignerAnchorBootstrapDetachedSignature,
	baseSignerConfig []byte,
) ([]byte, error) {
	certificate, err :=
		frostNativeSignerAnchorBootstrapCertificateFromFinal(final)
	if err != nil {
		return nil, err
	}
	if err := validateFrostNativeSignerAnchorBootstrapSignature(
		signature,
		FrostNativeSignerAnchorBootstrapFinalSignatureStage,
		final.FinalDigest,
		final.Core.Plan.OfflineAuthorityPublicKey,
	); err != nil {
		return nil, err
	}
	certificate.FinalSignature = signature.Signature
	certificate.CertificateDigest, err =
		ComputeFrostNativeSignerAnchorTrustCertificateDigest(certificate)
	if err != nil {
		return nil, err
	}
	if err := ValidateFrostNativeSignerAnchorTrustCertificate(
		certificate,
		ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement,
	); err != nil {
		return nil, err
	}
	certificateJSON, err :=
		EncodeFrostNativeSignerAnchorTrustCertificate(certificate)
	if err != nil {
		return nil, err
	}
	certificateChain, err := json.Marshal([]json.RawMessage{certificateJSON})
	if err != nil {
		return nil, err
	}
	if _, err := DecodeFrostNativeSignerAnchorTrustCertificateChain(
		certificateChain,
	); err != nil {
		return nil, err
	}
	signerConfig, err := frostNativeSignerAnchorBootstrapSignerConfig(
		baseSignerConfig,
		certificate,
	)
	if err != nil {
		return nil, err
	}
	wire := frostNativeSignerAnchorBootstrapOutputBundleWire{
		Schema:                 FrostNativeSignerAnchorBootstrapOutputBundleSchema,
		CertificateDigest:      frostNativeSignerAnchorHex32(certificate.CertificateDigest),
		CertificateChainSHA256: frostNativeSignerAnchorHex32(sha256.Sum256(certificateChain)),
		SignerConfigSHA256:     frostNativeSignerAnchorHex32(sha256.Sum256(signerConfig)),
		CertificateChain:       certificateChain,
		SignerConfig:           signerConfig,
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeFrostNativeSignerAnchorBootstrapOutputBundle(
		encoded,
	); err != nil {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap output bundle failed its decode round-trip: %w",
			err,
		)
	}
	return encoded, nil
}

// DecodeFrostNativeSignerAnchorBootstrapOutputBundle strictly decodes the
// certified ceremony output. It re-validates the embedded one-certificate
// bootstrap chain cryptographically and requires the embedded signer config to
// be the exact canonical derivation of that certificate.
func DecodeFrostNativeSignerAnchorBootstrapOutputBundle(
	data []byte,
) (*FrostNativeSignerAnchorBootstrapOutputBundle, error) {
	wire := &frostNativeSignerAnchorBootstrapOutputBundleWire{}
	if err := decodeStrictFrostNativeSignerAnchorProvisioningJSON(
		data,
		wire,
	); err != nil {
		return nil, err
	}
	if wire.Schema != FrostNativeSignerAnchorBootstrapOutputBundleSchema {
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap output-bundle schema",
		)
	}
	certificateDigest, err := frostNativeSignerAnchorParseHex32(
		wire.CertificateDigest,
	)
	if err != nil || certificateDigest == [32]byte{} {
		return nil, fmt.Errorf(
			"invalid bootstrap output-bundle certificate digest",
		)
	}
	chainSHA, err := frostNativeSignerAnchorParseHex32(
		wire.CertificateChainSHA256,
	)
	if err != nil || chainSHA != sha256.Sum256(wire.CertificateChain) {
		return nil, fmt.Errorf(
			"bootstrap output-bundle certificate chain SHA-256 mismatch",
		)
	}
	configSHA, err := frostNativeSignerAnchorParseHex32(wire.SignerConfigSHA256)
	if err != nil || configSHA != sha256.Sum256(wire.SignerConfig) {
		return nil, fmt.Errorf(
			"bootstrap output-bundle signer config SHA-256 mismatch",
		)
	}
	chain, err := DecodeFrostNativeSignerAnchorTrustCertificateChain(
		wire.CertificateChain,
	)
	if err != nil {
		return nil, err
	}
	if len(chain) != 1 ||
		chain[0].Kind != FrostNativeSignerAnchorTrustCertificateBootstrap ||
		chain[0].CertificateDigest != certificateDigest {
		return nil, fmt.Errorf(
			"bootstrap output-bundle chain is not the exact single bootstrap certificate",
		)
	}
	certificate := &chain[0]
	if err := ValidateFrostNativeSignerAnchorTrustCertificate(
		certificate,
		ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement,
	); err != nil {
		return nil, err
	}
	signerConfig, err := frostNativeSignerAnchorBootstrapSignerConfig(
		wire.SignerConfig,
		certificate,
	)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(signerConfig, wire.SignerConfig) {
		return nil, fmt.Errorf(
			"bootstrap output-bundle signer config is not the canonical certified derivation",
		)
	}
	return &FrostNativeSignerAnchorBootstrapOutputBundle{
		Schema:               wire.Schema,
		CertificateDigest:    certificateDigest,
		CertificateChain:     chain,
		CertificateChainJSON: append([]byte{}, wire.CertificateChain...),
		SignerConfigJSON:     append([]byte{}, wire.SignerConfig...),
	}, nil
}

func EncodeFrostNativeSignerAnchorBootstrapPlan(
	plan *FrostNativeSignerAnchorBootstrapPlan,
) ([]byte, error) {
	if err := validateFrostNativeSignerAnchorBootstrapPlan(plan); err != nil {
		return nil, err
	}
	return json.Marshal(frostNativeSignerAnchorBootstrapPlanToWire(plan))
}

func DecodeFrostNativeSignerAnchorBootstrapPlan(
	data []byte,
) (*FrostNativeSignerAnchorBootstrapPlan, error) {
	wire := &frostNativeSignerAnchorBootstrapPlanWire{}
	if err := decodeStrictFrostNativeSignerAnchorProvisioningJSON(
		data,
		wire,
	); err != nil {
		return nil, err
	}
	plan, err := frostNativeSignerAnchorBootstrapPlanFromWire(wire)
	if err != nil {
		return nil, err
	}
	if err := validateFrostNativeSignerAnchorBootstrapPlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func EncodeFrostNativeSignerAnchorBootstrapCoreArtifact(
	core *FrostNativeSignerAnchorBootstrapCoreArtifact,
) ([]byte, error) {
	if err := validateFrostNativeSignerAnchorBootstrapCore(core); err != nil {
		return nil, err
	}
	return json.Marshal(frostNativeSignerAnchorBootstrapCoreToWire(core))
}

func DecodeFrostNativeSignerAnchorBootstrapCoreArtifact(
	data []byte,
) (*FrostNativeSignerAnchorBootstrapCoreArtifact, error) {
	wire := &frostNativeSignerAnchorBootstrapCoreArtifactWire{}
	if err := decodeStrictFrostNativeSignerAnchorProvisioningJSON(
		data,
		wire,
	); err != nil {
		return nil, err
	}
	core, err := frostNativeSignerAnchorBootstrapCoreFromWire(wire)
	if err != nil {
		return nil, err
	}
	if err := validateFrostNativeSignerAnchorBootstrapCore(core); err != nil {
		return nil, err
	}
	return core, nil
}

func EncodeFrostNativeSignerAnchorBootstrapDetachedSignature(
	signature *FrostNativeSignerAnchorBootstrapDetachedSignature,
) ([]byte, error) {
	if signature == nil ||
		signature.Schema !=
			FrostNativeSignerAnchorBootstrapDetachedSignatureSchema ||
		(signature.Stage != FrostNativeSignerAnchorBootstrapCoreSignatureStage &&
			signature.Stage !=
				FrostNativeSignerAnchorBootstrapFinalSignatureStage) ||
		signature.Digest == [32]byte{} {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap detached signature is incomplete",
		)
	}
	return json.Marshal(frostNativeSignerAnchorBootstrapDetachedSignatureWire{
		Schema:    signature.Schema,
		Stage:     string(signature.Stage),
		Digest:    frostNativeSignerAnchorHex32(signature.Digest),
		Signature: base64.StdEncoding.EncodeToString(signature.Signature[:]),
	})
}

func DecodeFrostNativeSignerAnchorBootstrapDetachedSignature(
	data []byte,
) (*FrostNativeSignerAnchorBootstrapDetachedSignature, error) {
	wire := &frostNativeSignerAnchorBootstrapDetachedSignatureWire{}
	if err := decodeStrictFrostNativeSignerAnchorProvisioningJSON(
		data,
		wire,
	); err != nil {
		return nil, err
	}
	if wire.Schema !=
		FrostNativeSignerAnchorBootstrapDetachedSignatureSchema {
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap detached-signature schema",
		)
	}
	digest, err := frostNativeSignerAnchorParseHex32(wire.Digest)
	if err != nil || digest == [32]byte{} {
		return nil, fmt.Errorf("invalid detached signature digest")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(wire.Signature)
	if err != nil ||
		base64.StdEncoding.EncodeToString(decoded) != wire.Signature ||
		len(decoded) != ed25519.SignatureSize {
		return nil, fmt.Errorf("invalid detached Ed25519 signature")
	}
	result := &FrostNativeSignerAnchorBootstrapDetachedSignature{
		Schema: FrostNativeSignerAnchorBootstrapDetachedSignatureSchema,
		Stage:  FrostNativeSignerAnchorBootstrapSignatureStage(wire.Stage),
		Digest: digest,
	}
	copy(result.Signature[:], decoded)
	if result.Stage != FrostNativeSignerAnchorBootstrapCoreSignatureStage &&
		result.Stage != FrostNativeSignerAnchorBootstrapFinalSignatureStage {
		return nil, fmt.Errorf("invalid detached signature stage")
	}
	return result, nil
}

func EncodeFrostNativeSignerAnchorBootstrapFinalArtifact(
	final *FrostNativeSignerAnchorBootstrapFinalArtifact,
) ([]byte, error) {
	if _, err := frostNativeSignerAnchorBootstrapCertificateFromFinal(
		final,
	); err != nil {
		return nil, err
	}
	reference := frostNativeSignerAnchorTrustReferenceToWire(
		final.TargetReference,
	)
	return json.Marshal(frostNativeSignerAnchorBootstrapFinalArtifactWire{
		Schema:                      FrostNativeSignerAnchorBootstrapFinalArtifactSchema,
		Core:                        frostNativeSignerAnchorBootstrapCoreToWire(&final.Core),
		CoreSignature:               base64.StdEncoding.EncodeToString(final.CoreSignature[:]),
		TargetReference:             reference,
		TargetAcknowledgementBase64: base64.StdEncoding.EncodeToString(final.TargetAcknowledgement),
		TargetAcknowledgementSHA256: frostNativeSignerAnchorHex32(final.TargetAcknowledgementSHA256),
		FinalDigest:                 frostNativeSignerAnchorHex32(final.FinalDigest),
	})
}

func DecodeFrostNativeSignerAnchorBootstrapFinalArtifact(
	data []byte,
) (*FrostNativeSignerAnchorBootstrapFinalArtifact, error) {
	wire := &frostNativeSignerAnchorBootstrapFinalArtifactWire{}
	if err := decodeStrictFrostNativeSignerAnchorProvisioningJSON(
		data,
		wire,
	); err != nil {
		return nil, err
	}
	if wire.Schema != FrostNativeSignerAnchorBootstrapFinalArtifactSchema {
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap final-artifact schema",
		)
	}
	core, err := frostNativeSignerAnchorBootstrapCoreFromWire(&wire.Core)
	if err != nil {
		return nil, err
	}
	coreSignature, err := base64.StdEncoding.Strict().DecodeString(
		wire.CoreSignature,
	)
	if err != nil || len(coreSignature) != ed25519.SignatureSize ||
		base64.StdEncoding.EncodeToString(coreSignature) != wire.CoreSignature {
		return nil, fmt.Errorf("invalid bootstrap core signature")
	}
	reference, err := frostNativeSignerAnchorTrustReferenceFromWire(
		wire.TargetReference,
	)
	if err != nil {
		return nil, err
	}
	acknowledgement, err := base64.StdEncoding.Strict().DecodeString(
		wire.TargetAcknowledgementBase64,
	)
	if err != nil || len(acknowledgement) == 0 ||
		base64.StdEncoding.EncodeToString(acknowledgement) !=
			wire.TargetAcknowledgementBase64 {
		return nil, fmt.Errorf("invalid bootstrap target acknowledgement")
	}
	acknowledgementSHA, err := frostNativeSignerAnchorParseHex32(
		wire.TargetAcknowledgementSHA256,
	)
	if err != nil || acknowledgementSHA != sha256.Sum256(acknowledgement) {
		return nil, fmt.Errorf(
			"bootstrap target acknowledgement SHA-256 mismatch",
		)
	}
	finalDigest, err := frostNativeSignerAnchorParseHex32(wire.FinalDigest)
	if err != nil || finalDigest == [32]byte{} {
		return nil, fmt.Errorf("invalid bootstrap final digest")
	}
	result := &FrostNativeSignerAnchorBootstrapFinalArtifact{
		Schema:                      wire.Schema,
		Core:                        *core,
		TargetReference:             reference,
		TargetAcknowledgement:       acknowledgement,
		TargetAcknowledgementSHA256: acknowledgementSHA,
		FinalDigest:                 finalDigest,
	}
	copy(result.CoreSignature[:], coreSignature)
	if _, err := frostNativeSignerAnchorBootstrapCertificateFromFinal(
		result,
	); err != nil {
		return nil, err
	}
	return result, nil
}

func frostNativeSignerAnchorBootstrapPlanToWire(
	plan *FrostNativeSignerAnchorBootstrapPlan,
) frostNativeSignerAnchorBootstrapPlanWire {
	return frostNativeSignerAnchorBootstrapPlanWire{
		Schema:                    FrostNativeSignerAnchorBootstrapPlanSchema,
		Endpoint:                  plan.Endpoint,
		Identity:                  frostNativeSignerAnchorIdentityToWire(plan.Identity),
		ResponsePublicKey:         frostNativeSignerAnchorHex32(plan.ResponsePublicKey),
		OfflineAuthorityPublicKey: frostNativeSignerAnchorHex32(plan.OfflineAuthorityPublicKey),
	}
}

func frostNativeSignerAnchorBootstrapPlanFromWire(
	wire *frostNativeSignerAnchorBootstrapPlanWire,
) (*FrostNativeSignerAnchorBootstrapPlan, error) {
	if wire == nil || wire.Schema != FrostNativeSignerAnchorBootstrapPlanSchema {
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap plan schema",
		)
	}
	identity, err := frostNativeSignerAnchorIdentityFromWire(wire.Identity)
	if err != nil {
		return nil, err
	}
	responseKey, err := frostNativeSignerAnchorParseHex32(
		wire.ResponsePublicKey,
	)
	if err != nil {
		return nil, err
	}
	offlineKey, err := frostNativeSignerAnchorParseHex32(
		wire.OfflineAuthorityPublicKey,
	)
	if err != nil {
		return nil, err
	}
	return &FrostNativeSignerAnchorBootstrapPlan{
		Schema:                    wire.Schema,
		Endpoint:                  wire.Endpoint,
		Identity:                  identity,
		ResponsePublicKey:         responseKey,
		OfflineAuthorityPublicKey: offlineKey,
	}, nil
}

func frostNativeSignerAnchorBootstrapCoreToWire(
	core *FrostNativeSignerAnchorBootstrapCoreArtifact,
) frostNativeSignerAnchorBootstrapCoreArtifactWire {
	return frostNativeSignerAnchorBootstrapCoreArtifactWire{
		Schema:           FrostNativeSignerAnchorBootstrapCoreArtifactSchema,
		Plan:             frostNativeSignerAnchorBootstrapPlanToWire(&core.Plan),
		Checkpoint:       frostNativeSignerAnchorCheckpointToWire(core.Checkpoint),
		FactsSHA256:      frostNativeSignerAnchorHex32(core.FactsSHA256),
		CoreDigest:       frostNativeSignerAnchorHex32(core.CoreDigest),
		OperationID:      frostNativeSignerAnchorHex32(core.OperationID),
		TransitionDigest: frostNativeSignerAnchorHex32(core.TransitionDigest),
	}
}

func frostNativeSignerAnchorBootstrapCoreFromWire(
	wire *frostNativeSignerAnchorBootstrapCoreArtifactWire,
) (*FrostNativeSignerAnchorBootstrapCoreArtifact, error) {
	if wire == nil ||
		wire.Schema != FrostNativeSignerAnchorBootstrapCoreArtifactSchema {
		return nil, fmt.Errorf(
			"unsupported native signer anchor bootstrap core-artifact schema",
		)
	}
	plan, err := frostNativeSignerAnchorBootstrapPlanFromWire(&wire.Plan)
	if err != nil {
		return nil, err
	}
	checkpoint, err := frostNativeSignerAnchorCheckpointFromWire(
		wire.Checkpoint,
	)
	if err != nil {
		return nil, err
	}
	result := &FrostNativeSignerAnchorBootstrapCoreArtifact{
		Schema:     wire.Schema,
		Plan:       *plan,
		Checkpoint: checkpoint,
	}
	fields := []struct {
		encoded     string
		destination *[32]byte
	}{
		{wire.FactsSHA256, &result.FactsSHA256},
		{wire.CoreDigest, &result.CoreDigest},
		{wire.OperationID, &result.OperationID},
		{wire.TransitionDigest, &result.TransitionDigest},
	}
	for _, field := range fields {
		value, err := frostNativeSignerAnchorParseHex32(field.encoded)
		if err != nil || value == [32]byte{} {
			return nil, fmt.Errorf("bootstrap core artifact contains an invalid digest")
		}
		*field.destination = value
	}
	return result, nil
}

func validateFrostNativeSignerAnchorBootstrapPlan(
	plan *FrostNativeSignerAnchorBootstrapPlan,
) error {
	if plan == nil || plan.Schema != FrostNativeSignerAnchorBootstrapPlanSchema {
		return fmt.Errorf("native signer anchor bootstrap plan is incomplete")
	}
	_, https, err := validateFrostNativeSignerAnchorEndpoint(plan.Endpoint)
	if err != nil {
		return err
	}
	if err := validateFrostNativeSignerAnchorIdentity(
		plan.Identity,
		https,
	); err != nil {
		return err
	}
	if ComputeFrostNativeSignerAnchorTransportBinding(plan.Endpoint) !=
		plan.Identity.TransportBinding {
		return fmt.Errorf(
			"native signer anchor bootstrap endpoint differs from its transport binding",
		)
	}
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		plan.ResponsePublicKey[:],
	); err != nil {
		return fmt.Errorf("invalid bootstrap response key: %w", err)
	}
	if err := ValidateFrostNativeSignerAnchorTrustEd25519PublicKey(
		plan.OfflineAuthorityPublicKey[:],
	); err != nil {
		return fmt.Errorf("invalid bootstrap offline authority key: %w", err)
	}
	if ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
		plan.ResponsePublicKey,
	) != plan.Identity.OnlineKeyHash ||
		ComputeFrostNativeSignerAnchorTrustEd25519SPKISHA256(
			plan.OfflineAuthorityPublicKey,
		) != plan.Identity.OfflineAuthorityHash ||
		plan.ResponsePublicKey == plan.OfflineAuthorityPublicKey {
		return fmt.Errorf(
			"native signer anchor bootstrap public keys differ from their activation pins",
		)
	}
	return nil
}

func validateFrostNativeSignerAnchorBootstrapCore(
	core *FrostNativeSignerAnchorBootstrapCoreArtifact,
) error {
	if core == nil ||
		core.Schema != FrostNativeSignerAnchorBootstrapCoreArtifactSchema ||
		core.FactsSHA256 == [32]byte{} {
		return fmt.Errorf("native signer anchor bootstrap core artifact is incomplete")
	}
	if err := validateFrostNativeSignerAnchorBootstrapPlan(
		&core.Plan,
	); err != nil {
		return err
	}
	if err := validateFrostNativeSignerAnchorCheckpoint(
		core.Checkpoint,
		core.Plan.Identity.SignerStoreFingerprint,
	); err != nil {
		return err
	}
	if core.Checkpoint.Generation != 1 ||
		core.Checkpoint.PreviousStateCommitment !=
			frostsigning.ComputeNativeTBTCSignerStateWitnessGenesis(
				core.Checkpoint.StoreFingerprint,
			) {
		return fmt.Errorf(
			"native signer anchor bootstrap checkpoint is not the exact genesis",
		)
	}
	certificate := frostNativeSignerAnchorBootstrapCoreCertificate(
		&core.Plan,
		core.Checkpoint,
	)
	digest, err := ComputeFrostNativeSignerAnchorTrustCoreDigest(&certificate)
	if err != nil || digest != core.CoreDigest {
		return fmt.Errorf("native signer anchor bootstrap core digest mismatch")
	}
	operationID := ComputeFrostNativeSignerAnchorTrustOperationID(digest)
	if operationID != core.OperationID ||
		ComputeFrostNativeSignerAnchorTrustTransitionDigest(
			digest,
			operationID,
		) != core.TransitionDigest {
		return fmt.Errorf(
			"native signer anchor bootstrap operation or transition digest mismatch",
		)
	}
	return nil
}

func validateFrostNativeSignerAnchorBootstrapSignature(
	signature *FrostNativeSignerAnchorBootstrapDetachedSignature,
	stage FrostNativeSignerAnchorBootstrapSignatureStage,
	digest [32]byte,
	publicKey [ed25519.PublicKeySize]byte,
) error {
	if signature == nil ||
		signature.Schema !=
			FrostNativeSignerAnchorBootstrapDetachedSignatureSchema ||
		signature.Stage != stage ||
		signature.Digest != digest ||
		!ed25519.Verify(
			ed25519.PublicKey(publicKey[:]),
			digest[:],
			signature.Signature[:],
		) {
		return fmt.Errorf(
			"native signer anchor bootstrap %s signature is invalid",
			stage,
		)
	}
	return nil
}

func frostNativeSignerAnchorBootstrapCoreCertificate(
	plan *FrostNativeSignerAnchorBootstrapPlan,
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) FrostNativeSignerAnchorTrustCertificate {
	bindingHash := ComputeFrostNativeSignerAnchorBindingHash(plan.Identity)
	return FrostNativeSignerAnchorTrustCertificate{
		Kind:                   FrostNativeSignerAnchorTrustCertificateBootstrap,
		CertificateSequence:    1,
		ProtocolID:             plan.Identity.ProtocolID,
		StreamID:               plan.Identity.StreamID,
		SignerStoreFingerprint: plan.Identity.SignerStoreFingerprint,
		To: FrostNativeSignerAnchorTrustEndpoint{
			ActivationManifestHash:          plan.Identity.ActivationManifestHash,
			ActivationManifestSequence:      plan.Identity.ActivationManifestSequence,
			BindingHash:                     bindingHash,
			ResponsePublicKey:               plan.ResponsePublicKey,
			ResponsePublicKeySPKISHA256:     plan.Identity.OnlineKeyHash,
			OfflineAuthorityPublicKey:       plan.OfflineAuthorityPublicKey,
			OfflineAuthoritySPKISHA256:      plan.Identity.OfflineAuthorityHash,
			WitnessMaximumRecords:           plan.Identity.WitnessMaximumRecords,
			WitnessRotationThresholdRecords: plan.Identity.WitnessRotationThresholdRecords,
			Reference: FrostNativeSignerAnchorTrustReference{
				ServiceEpoch: 1,
				Checkpoint:   checkpoint,
			},
		},
	}
}

func frostNativeSignerAnchorBootstrapCertificateFromFinal(
	final *FrostNativeSignerAnchorBootstrapFinalArtifact,
) (*FrostNativeSignerAnchorTrustCertificate, error) {
	if final == nil ||
		final.Schema != FrostNativeSignerAnchorBootstrapFinalArtifactSchema ||
		len(final.TargetAcknowledgement) == 0 ||
		final.TargetAcknowledgementSHA256 !=
			sha256.Sum256(final.TargetAcknowledgement) {
		return nil, fmt.Errorf(
			"native signer anchor bootstrap final artifact is incomplete",
		)
	}
	if err := validateFrostNativeSignerAnchorBootstrapCore(&final.Core); err != nil {
		return nil, err
	}
	certificate := frostNativeSignerAnchorBootstrapCoreCertificate(
		&final.Core.Plan,
		final.Core.Checkpoint,
	)
	certificate.CoreDigest = final.Core.CoreDigest
	certificate.CoreSignature = final.CoreSignature
	certificate.OperationID = final.Core.OperationID
	certificate.TransitionDigest = final.Core.TransitionDigest
	certificate.To.Reference = final.TargetReference
	certificate.TargetAcknowledgement = append(
		[]byte{},
		final.TargetAcknowledgement...,
	)
	certificate.TargetAcknowledgementSHA256 =
		final.TargetAcknowledgementSHA256
	if !ed25519.Verify(
		ed25519.PublicKey(final.Core.Plan.OfflineAuthorityPublicKey[:]),
		certificate.CoreDigest[:],
		certificate.CoreSignature[:],
	) {
		return nil, fmt.Errorf("native signer anchor bootstrap core signature is invalid")
	}
	if err := ValidateFrostNativeSignerAnchorTrustTargetAcknowledgement(
		&certificate,
		certificate.TargetAcknowledgement,
	); err != nil {
		return nil, err
	}
	finalDigest, err := ComputeFrostNativeSignerAnchorTrustFinalDigest(
		&certificate,
	)
	if err != nil || finalDigest != final.FinalDigest {
		return nil, fmt.Errorf("native signer anchor bootstrap final digest mismatch")
	}
	return &certificate, nil
}

func frostNativeSignerAnchorBootstrapCheckpoint(
	checkpoint frostsigning.NativeTBTCSignerStateAnchorCheckpoint,
) FrostNativeSignerStateWitnessCheckpoint {
	return FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        checkpoint.StoreFingerprint,
		Generation:              checkpoint.Generation,
		PreviousStateCommitment: checkpoint.PreviousStateCommitment,
		StateImageDigest:        checkpoint.StateImageDigest,
		StateCommitment:         checkpoint.StateCommitment,
	}
}

func frostNativeSignerAnchorIdentityFromWire(
	wire frostNativeSignerAnchorIdentityWire,
) (FrostNativeSignerAnchorIdentity, error) {
	result := FrostNativeSignerAnchorIdentity{
		TrustDomainID:  wire.TrustDomainID,
		HistoryStoreID: wire.HistoryStoreID,
	}
	var err error
	if result.ActivationManifestSequence, err =
		frostNativeSignerAnchorParseUint64(
			wire.ActivationManifestSequence,
		); err != nil {
		return result, err
	}
	if result.WitnessMaximumRecords, err =
		frostNativeSignerAnchorParseUint64(
			wire.WitnessMaximumRecords,
		); err != nil {
		return result, err
	}
	if result.WitnessRotationThresholdRecords, err =
		frostNativeSignerAnchorParseUint64(
			wire.WitnessRotationThresholdRecords,
		); err != nil {
		return result, err
	}
	fields := []struct {
		encoded     string
		destination *[32]byte
		// allowZero is set only for the endpoint leaf SPKI pin: numeric
		// loopback HTTP plans legitimately carry a zero leaf hash, and
		// validateFrostNativeSignerAnchorIdentity enforces that exact pairing.
		allowZero bool
	}{
		{wire.ProtocolID, &result.ProtocolID, false},
		{wire.StreamID, &result.StreamID, false},
		{wire.ActivationManifestHash, &result.ActivationManifestHash, false},
		{wire.EndpointLeafSPKIHash, &result.EndpointLeafSPKIHash, true},
		{wire.OnlineKeyHash, &result.OnlineKeyHash, false},
		{wire.OperatorFingerprint, &result.OperatorFingerprint, false},
		{wire.HistoryStoreFingerprint, &result.HistoryStoreFingerprint, false},
		{wire.HistoryClusterFingerprint, &result.HistoryClusterFingerprint, false},
		{wire.OfflineAuthorityHash, &result.OfflineAuthorityHash, false},
		{wire.ClientSPKIHash, &result.ClientSPKIHash, false},
		{wire.SignerStoreFingerprint, &result.SignerStoreFingerprint, false},
		{wire.TransportBinding, &result.TransportBinding, false},
	}
	for _, field := range fields {
		value, err := frostNativeSignerAnchorParseHex32(field.encoded)
		if err != nil {
			return result, err
		}
		if !field.allowZero && value == [32]byte{} {
			return result, fmt.Errorf("bootstrap identity contains a zero pin")
		}
		*field.destination = value
	}
	return result, nil
}

func frostNativeSignerAnchorBootstrapSignerConfig(
	base []byte,
	certificate *FrostNativeSignerAnchorTrustCertificate,
) ([]byte, error) {
	value, err := decodeCanonicalFrostNativeSignerProvisioningObject(base)
	if err != nil {
		return nil, err
	}
	profile, ok := value["profile"].(string)
	if !ok || profile != "production" {
		return nil, fmt.Errorf(
			"base native signer config must explicitly select profile production",
		)
	}
	statePath, ok := value["state_path"].(string)
	if !ok || !filepath.IsAbs(statePath) ||
		filepath.Clean(statePath) != statePath {
		return nil, fmt.Errorf(
			"base native signer config must contain a canonical absolute state_path",
		)
	}
	if purpose, present := value["purpose"]; present &&
		purpose != "normal_signer" {
		return nil, fmt.Errorf(
			"base native signer config purpose is not normal_signer",
		)
	}
	derived := map[string]interface{}{
		"purpose": "normal_signer",
		"state_anchor_protocol_id": frostNativeSignerAnchorHex32(
			certificate.ProtocolID,
		),
		"state_anchor_stream_id": frostNativeSignerAnchorHex32(
			certificate.StreamID,
		),
		"state_anchor_activation_manifest_hash": frostNativeSignerAnchorHex32(
			certificate.To.ActivationManifestHash,
		),
		"state_anchor_activation_manifest_sequence": json.Number(
			strconv.FormatUint(
				certificate.To.ActivationManifestSequence,
				10,
			),
		),
		"state_anchor_binding_hash": frostNativeSignerAnchorHex32(
			certificate.To.BindingHash,
		),
		"state_anchor_response_public_key": frostNativeSignerAnchorHex32(
			certificate.To.ResponsePublicKey,
		),
		"state_anchor_response_public_key_spki_sha256": frostNativeSignerAnchorHex32(
			certificate.To.ResponsePublicKeySPKISHA256,
		),
		"state_anchor_offline_authority_public_key": frostNativeSignerAnchorHex32(
			certificate.To.OfflineAuthorityPublicKey,
		),
		"state_anchor_offline_authority_public_key_spki_sha256": frostNativeSignerAnchorHex32(
			certificate.To.OfflineAuthoritySPKISHA256,
		),
		"state_anchor_trust_certificate_sequence": json.Number(
			strconv.FormatUint(certificate.CertificateSequence, 10),
		),
		"state_anchor_trust_certificate_digest": frostNativeSignerAnchorHex32(
			certificate.CertificateDigest,
		),
		"state_witness_max_records": json.Number(
			strconv.FormatUint(certificate.To.WitnessMaximumRecords, 10),
		),
		"state_witness_rotation_threshold_records": json.Number(
			strconv.FormatUint(
				certificate.To.WitnessRotationThresholdRecords,
				10,
			),
		),
	}
	for key, expected := range derived {
		if existing, present := value[key]; present &&
			!reflect.DeepEqual(existing, expected) {
			return nil, fmt.Errorf(
				"base native signer config field [%s] conflicts with the certified value",
				key,
			)
		}
		value[key] = expected
	}
	return json.Marshal(value)
}

func decodeCanonicalFrostNativeSignerProvisioningObject(
	data []byte,
) (map[string]interface{}, error) {
	if err := preflightFrostNativeSignerProvisioningJSON(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value := make(map[string]interface{})
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("provisioning JSON contains trailing data")
	}
	if err := validateCanonicalFrostNativeSignerProvisioningValue(
		value,
	); err != nil {
		return nil, err
	}
	return value, nil
}

func validateCanonicalFrostNativeSignerProvisioningValue(
	value interface{},
) error {
	switch typed := value.(type) {
	case nil, bool, string:
		return nil
	case json.Number:
		encoded := string(typed)
		if encoded == "" ||
			(len(encoded) > 1 && encoded[0] == '0') {
			return fmt.Errorf("provisioning JSON number is not canonical uint64")
		}
		if _, err := strconv.ParseUint(encoded, 10, 64); err != nil {
			return fmt.Errorf("provisioning JSON number is not canonical uint64")
		}
		return nil
	case []interface{}:
		for _, item := range typed {
			if err := validateCanonicalFrostNativeSignerProvisioningValue(
				item,
			); err != nil {
				return err
			}
		}
		return nil
	case map[string]interface{}:
		for _, item := range typed {
			if err := validateCanonicalFrostNativeSignerProvisioningValue(
				item,
			); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("provisioning JSON contains an unsupported value")
	}
}

func decodeStrictFrostNativeSignerAnchorProvisioningJSON(
	data []byte,
	target interface{},
) error {
	if len(data) == 0 ||
		int64(len(data)) >
			FrostNativeSignerAnchorProvisioningArtifactMaximumBytes {
		return fmt.Errorf("native signer anchor provisioning artifact size is invalid")
	}
	if err := preflightFrostNativeSignerProvisioningJSON(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("native signer anchor provisioning JSON contains trailing data")
	}
	return nil
}

func preflightFrostNativeSignerProvisioningJSON(data []byte) error {
	const maximumDepth = 32
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var scan func(int) error
	scan = func(depth int) error {
		if depth > maximumDepth {
			return fmt.Errorf("provisioning JSON exceeds the depth bound")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			folded := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || key == "" {
					return fmt.Errorf(
						"provisioning JSON member name is invalid",
					)
				}
				for _, character := range key {
					if character < 0x21 || character > 0x7e {
						return fmt.Errorf(
							"provisioning JSON member name is not canonical ASCII",
						)
					}
				}
				lower := strings.ToLower(key)
				if _, duplicate := seen[key]; duplicate {
					return fmt.Errorf(
						"provisioning JSON contains duplicate member [%s]",
						key,
					)
				}
				if _, duplicate := folded[lower]; duplicate {
					return fmt.Errorf(
						"provisioning JSON contains case-folded duplicate member [%s]",
						key,
					)
				}
				seen[key] = struct{}{}
				folded[lower] = struct{}{}
				if err := scan(depth + 1); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return fmt.Errorf(
					"provisioning JSON object termination is invalid",
				)
			}
		case '[':
			for decoder.More() {
				if err := scan(depth + 1); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return fmt.Errorf(
					"provisioning JSON array termination is invalid",
				)
			}
		default:
			return fmt.Errorf("provisioning JSON delimiter is invalid")
		}
		return nil
	}
	if err := scan(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("provisioning JSON contains trailing data")
		}
		return err
	}
	return nil
}

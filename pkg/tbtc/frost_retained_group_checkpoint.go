package tbtc

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	"github.com/decred/dcrd/dcrec/edwards/v2"
)

const (
	frostRetainedGroupCheckpointBodySchema        = "tbtc-frost-retained-group-checkpoint-body/v1"
	frostRetainedGroupCheckpointCertificateSchema = "tbtc-frost-retained-group-checkpoint-certificate/v1"
	frostRetainedGroupCheckpointMetadataSchema    = "tbtc-frost-retained-group-checkpoint-metadata/v1"
	frostRetainedGroupCheckpointStateSchema       = "tbtc-frost-retained-group-checkpoint-state/v1"

	frostRetainedGroupCheckpointBodyDomain        = "tbtc-frost-retained-group-checkpoint-body-v1\x00"
	frostRetainedGroupCheckpointSignatureDomain   = "tbtc-frost-retained-group-checkpoint-signature-v1\x00"
	frostRetainedGroupCheckpointCertificateDomain = "tbtc-frost-retained-group-checkpoint-certificate-v1\x00"
	frostRetainedGroupCheckpointChainDomain       = "tbtc-frost-retained-group-checkpoint-chain-v1\x00"

	frostRetainedGroupCheckpointMetadataFile = "metadata.json"
	frostRetainedGroupCheckpointStateFile    = "state.json"
	frostRetainedGroupCheckpointFilePrefix   = "certificate-"
	// A recovery page is deliberately bounded, while a complete certificate
	// chain is not. The journal can durably advance through multiple pages and
	// resume from the last authenticated certificate after a crash or timeout.
	frostRetainedGroupMaximumCheckpointsPerPage = 256
	// Reconciliation durably publishes exactly one authenticated page before
	// yielding an explicit progress result. The controller then re-enters with
	// a fresh timeout from the new durable cursor, so total history is unbounded
	// without making one reconciliation attempt unbounded.
	frostRetainedGroupCheckpointPagesPerReconciliation = 1
	// Activation verifiers must obtain a fresh rollback-independent floor from
	// the transparency channel. An arbitrarily stale caller-supplied floor
	// cannot force the signer to allocate and serialize its entire lifetime
	// history. This remains above the old 256-certificate recovery limit.
	frostRetainedGroupMaximumHandshakeAncestry = 512
	// Canonical checkpoint proof bytes are accounted certificate-by-certificate
	// before the aggregate handshake payload is materialized. This protects the
	// signer from a small loopback request expanding into hundreds of megabytes
	// of authority credentials and canonical-JSON working buffers.
	frostRetainedGroupMaximumHandshakeProofBytes = 4 * 1024 * 1024
	frostRetainedGroupCheckpointDirectory        = "checkpoints"
)

// FrostRetainedGroupCheckpointCursor is the rollback-independent head after
// which a history source must return a cryptographically contiguous suffix.
// Sequence zero and a zero digest denote the manifest's sequence-one genesis
// predecessor.
type FrostRetainedGroupCheckpointCursor struct {
	Sequence        uint64
	CertificateHash [32]byte
}

// FrostRetainedGroupCheckpointBody commits a quorum to one deterministic
// semantic retained-group state. Per-node batch roots are deliberately
// excluded: nodes can reconcile at different intervals while deriving the
// same inventory and quarantine roots from the same complete history.
type FrostRetainedGroupCheckpointBody struct {
	Schema                  string
	ProtocolBindingHash     [32]byte
	ManifestHash            [32]byte
	ProfileHash             [32]byte
	ImplementationSetHash   [32]byte
	ChainID                 uint64
	DomainChainID           [32]byte
	GenesisBlockHash        [32]byte
	AuthoritySetHash        [32]byte
	Sequence                uint64
	PreviousCertificateHash [32]byte
	Point                   FrostPreSignFinality
	HistoryRoot             [32]byte
	CanonicalGeneration     uint64
	CanonicalInventoryRoot  [32]byte
	QuarantineGeneration    uint64
	QuarantineEventRoot     [32]byte
	QuarantineActiveRoot    [32]byte
	QuarantineTombstoneRoot [32]byte
}

type FrostRetainedGroupCheckpointSignature struct {
	AuthorityID         string
	SignerPublicKeySPKI string
	Signature           string
}

type FrostRetainedGroupCheckpointCertificate struct {
	Schema     string
	Body       FrostRetainedGroupCheckpointBody
	BodyHash   [32]byte
	Signatures []FrostRetainedGroupCheckpointSignature
}

// FrostRetainedGroupCheckpointCommitment is the exact durable semantic state
// that the tail of an externally verified checkpoint proof must certify.
type FrostRetainedGroupCheckpointCommitment struct {
	DurableHead             FrostRetainedGroupCheckpointCursor
	Point                   FrostPreSignFinality
	HistoryRoot             [32]byte
	CanonicalGeneration     uint64
	CanonicalInventoryRoot  [32]byte
	QuarantineGeneration    uint64
	QuarantineEventRoot     [32]byte
	QuarantineActiveRoot    [32]byte
	QuarantineTombstoneRoot [32]byte
}

type frostRetainedGroupWireCheckpointBody struct {
	Schema                  string                         `json:"schema"`
	ProtocolBindingHash     string                         `json:"protocolBindingHash"`
	ManifestHash            string                         `json:"manifestHash"`
	ProfileHash             string                         `json:"profileHash"`
	ImplementationSetHash   string                         `json:"implementationSetHash"`
	ChainID                 uint64                         `json:"chainID"`
	DomainChainID           string                         `json:"domainChainID"`
	GenesisBlockHash        string                         `json:"genesisBlockHash"`
	AuthoritySetHash        string                         `json:"authoritySetHash"`
	Sequence                uint64                         `json:"sequence"`
	PreviousCertificateHash string                         `json:"previousCertificateHash"`
	Point                   frostRetainedGroupWireFinality `json:"point"`
	HistoryRoot             string                         `json:"historyRoot"`
	CanonicalGeneration     uint64                         `json:"canonicalGeneration"`
	CanonicalInventoryRoot  string                         `json:"canonicalInventoryRoot"`
	QuarantineGeneration    uint64                         `json:"quarantineGeneration"`
	QuarantineEventRoot     string                         `json:"quarantineEventRoot"`
	QuarantineActiveRoot    string                         `json:"quarantineActiveRoot"`
	QuarantineTombstoneRoot string                         `json:"quarantineTombstoneRoot"`
}

type frostRetainedGroupWireCheckpointSignature struct {
	AuthorityID         string `json:"authorityID"`
	SignerPublicKeySPKI string `json:"signerPublicKeySpki"`
	Signature           string `json:"signature"`
}

type frostRetainedGroupWireCheckpointCertificate struct {
	Schema     string                                      `json:"schema"`
	Body       frostRetainedGroupWireCheckpointBody        `json:"body"`
	BodyHash   string                                      `json:"bodyHash"`
	Signatures []frostRetainedGroupWireCheckpointSignature `json:"signatures"`
}

type frostRetainedGroupCheckpointPolicy struct {
	ProtocolBindingHash   [32]byte
	ManifestHash          [32]byte
	ProfileHash           [32]byte
	ImplementationSetHash [32]byte
	ChainID               uint64
	DomainChainID         [32]byte
	GenesisBlockHash      [32]byte
	AuthoritySetHash      [32]byte
	AuthorityThreshold    uint64
	Authorities           []FrostRetainedGroupAuthority
	MinimumSequence       uint64
	PredecessorHash       [32]byte
	CanonicalMinimum      uint64
	QuarantineMinimum     uint64
	LiftPolicy            frostRetainedGroupQuarantineLiftPolicy
}

type frostRetainedGroupCheckpointMetadata struct {
	Schema             string                        `json:"schema"`
	ManifestHash       [32]byte                      `json:"manifestHash"`
	BindingHash        [32]byte                      `json:"bindingHash"`
	AuthoritySetHash   [32]byte                      `json:"authoritySetHash"`
	AuthorityThreshold uint64                        `json:"authorityThreshold"`
	Authorities        []FrostRetainedGroupAuthority `json:"authorities"`
	MinimumSequence    uint64                        `json:"minimumSequence"`
	PredecessorHash    [32]byte                      `json:"predecessorHash"`
}

type frostRetainedGroupCheckpointJournalState struct {
	Schema                  string               `json:"schema"`
	BindingHash             [32]byte             `json:"bindingHash"`
	Sequence                uint64               `json:"sequence"`
	CertificateHash         [32]byte             `json:"certificateHash"`
	Point                   FrostPreSignFinality `json:"point"`
	HistoryRoot             [32]byte             `json:"historyRoot"`
	CanonicalGeneration     uint64               `json:"canonicalGeneration"`
	CanonicalInventoryRoot  [32]byte             `json:"canonicalInventoryRoot"`
	QuarantineGeneration    uint64               `json:"quarantineGeneration"`
	QuarantineEventRoot     [32]byte             `json:"quarantineEventRoot"`
	QuarantineActiveRoot    [32]byte             `json:"quarantineActiveRoot"`
	QuarantineTombstoneRoot [32]byte             `json:"quarantineTombstoneRoot"`
}

func frostRetainedGroupCheckpointPolicyFromRuntimeManifest(
	bindingHash [32]byte,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
) (frostRetainedGroupCheckpointPolicy, error) {
	quarantine := runtimeManifest.QuarantineJournal
	authoritySetHash, err := frostRetainedGroupAuthoritySetHash(
		"tbtc-frost-retained-group-checkpoint-authority-set/v1",
		quarantine.CheckpointAuthorityThreshold,
		quarantine.CheckpointAuthorities,
	)
	if err != nil {
		return frostRetainedGroupCheckpointPolicy{}, err
	}
	if bindingHash == [32]byte{} ||
		runtimeManifest.ManifestHash == [32]byte{} ||
		runtimeManifest.ProfileHash == [32]byte{} ||
		runtimeManifest.ImplementationSetHash == [32]byte{} ||
		runtimeManifest.DomainChainID == [32]byte{} ||
		runtimeManifest.GenesisBlockHash == [32]byte{} ||
		quarantine.CheckpointMinimumSequence == 0 ||
		quarantine.CheckpointMinimumSequence >
			frostRetainedGroupMaximumCanonicalJSONInteger ||
		(quarantine.CheckpointMinimumSequence == 1 &&
			quarantine.CheckpointPredecessorHash != [32]byte{}) ||
		(quarantine.CheckpointMinimumSequence > 1 &&
			quarantine.CheckpointPredecessorHash == [32]byte{}) {
		return frostRetainedGroupCheckpointPolicy{}, fmt.Errorf(
			"FROST retained-group checkpoint policy is incomplete",
		)
	}
	for _, value := range runtimeManifest.DomainChainID[:24] {
		if value != 0 {
			return frostRetainedGroupCheckpointPolicy{}, fmt.Errorf(
				"FROST checkpoint chain ID exceeds uint64",
			)
		}
	}
	chainID := uint64(0)
	for _, value := range runtimeManifest.DomainChainID[24:] {
		chainID = (chainID << 8) | uint64(value)
	}
	if chainID == 0 {
		return frostRetainedGroupCheckpointPolicy{}, fmt.Errorf(
			"FROST checkpoint chain ID is zero",
		)
	}
	liftPolicy, err := frostRetainedGroupLiftPolicyFromRuntimeManifest(
		bindingHash,
		runtimeManifest,
	)
	if err != nil {
		return frostRetainedGroupCheckpointPolicy{}, err
	}
	return frostRetainedGroupCheckpointPolicy{
		ProtocolBindingHash:   bindingHash,
		ManifestHash:          runtimeManifest.ManifestHash,
		ProfileHash:           runtimeManifest.ProfileHash,
		ImplementationSetHash: runtimeManifest.ImplementationSetHash,
		ChainID:               chainID,
		DomainChainID:         runtimeManifest.DomainChainID,
		GenesisBlockHash:      runtimeManifest.GenesisBlockHash,
		AuthoritySetHash:      authoritySetHash,
		AuthorityThreshold:    quarantine.CheckpointAuthorityThreshold,
		Authorities: append(
			[]FrostRetainedGroupAuthority{},
			quarantine.CheckpointAuthorities...,
		),
		MinimumSequence:   quarantine.CheckpointMinimumSequence,
		PredecessorHash:   quarantine.CheckpointPredecessorHash,
		CanonicalMinimum:  runtimeManifest.CanonicalJournal.MinimumGeneration,
		QuarantineMinimum: quarantine.MinimumGeneration,
		LiftPolicy:        liftPolicy,
	}, nil
}

func frostRetainedGroupCheckpointCertificateToWire(
	certificate FrostRetainedGroupCheckpointCertificate,
) frostRetainedGroupWireCheckpointCertificate {
	body := certificate.Body
	signatures := make(
		[]frostRetainedGroupWireCheckpointSignature,
		len(certificate.Signatures),
	)
	for index, signature := range certificate.Signatures {
		signatures[index] = frostRetainedGroupWireCheckpointSignature{
			AuthorityID:         signature.AuthorityID,
			SignerPublicKeySPKI: signature.SignerPublicKeySPKI,
			Signature:           signature.Signature,
		}
	}
	return frostRetainedGroupWireCheckpointCertificate{
		Schema: certificate.Schema,
		Body: frostRetainedGroupWireCheckpointBody{
			Schema:                  body.Schema,
			ProtocolBindingHash:     frostActivationHex32(body.ProtocolBindingHash),
			ManifestHash:            frostActivationHex32(body.ManifestHash),
			ProfileHash:             frostActivationHex32(body.ProfileHash),
			ImplementationSetHash:   frostActivationHex32(body.ImplementationSetHash),
			ChainID:                 body.ChainID,
			DomainChainID:           frostActivationHex32(body.DomainChainID),
			GenesisBlockHash:        frostActivationHex32(body.GenesisBlockHash),
			AuthoritySetHash:        frostActivationHex32(body.AuthoritySetHash),
			Sequence:                body.Sequence,
			PreviousCertificateHash: frostActivationHex32(body.PreviousCertificateHash),
			Point:                   frostRetainedGroupFinalityToWire(body.Point),
			HistoryRoot:             frostActivationHex32(body.HistoryRoot),
			CanonicalGeneration:     body.CanonicalGeneration,
			CanonicalInventoryRoot:  frostActivationHex32(body.CanonicalInventoryRoot),
			QuarantineGeneration:    body.QuarantineGeneration,
			QuarantineEventRoot:     frostActivationHex32(body.QuarantineEventRoot),
			QuarantineActiveRoot:    frostActivationHex32(body.QuarantineActiveRoot),
			QuarantineTombstoneRoot: frostActivationHex32(body.QuarantineTombstoneRoot),
		},
		BodyHash:   frostActivationHex32(certificate.BodyHash),
		Signatures: signatures,
	}
}

func frostRetainedGroupCheckpointCertificateFromWire(
	wire frostRetainedGroupWireCheckpointCertificate,
) (FrostRetainedGroupCheckpointCertificate, error) {
	parse := func(name string, value string) ([32]byte, error) {
		result, err := parseFrostActivationHex32(value)
		if err != nil {
			return [32]byte{}, fmt.Errorf(
				"invalid FROST checkpoint %s: [%w]",
				name,
				err,
			)
		}
		return result, nil
	}
	protocolBindingHash, err := parse(
		"protocol binding hash",
		wire.Body.ProtocolBindingHash,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	manifestHash, err := parse("manifest hash", wire.Body.ManifestHash)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	profileHash, err := parse("profile hash", wire.Body.ProfileHash)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	implementationSetHash, err := parse(
		"implementation set hash",
		wire.Body.ImplementationSetHash,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	domainChainID, err := parse("domain chain ID", wire.Body.DomainChainID)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	genesisBlockHash, err := parse(
		"genesis block hash",
		wire.Body.GenesisBlockHash,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	authoritySetHash, err := parse(
		"authority set hash",
		wire.Body.AuthoritySetHash,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	previousCertificateHash, err := parse(
		"previous certificate hash",
		wire.Body.PreviousCertificateHash,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	point, err := frostRetainedGroupFinalityFromWire(wire.Body.Point)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, fmt.Errorf(
			"invalid FROST checkpoint point: [%w]",
			err,
		)
	}
	historyRoot, err := parse("history root", wire.Body.HistoryRoot)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	canonicalInventoryRoot, err := parse(
		"canonical inventory root",
		wire.Body.CanonicalInventoryRoot,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	quarantineEventRoot, err := parse(
		"quarantine event root",
		wire.Body.QuarantineEventRoot,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	quarantineActiveRoot, err := parse(
		"quarantine active root",
		wire.Body.QuarantineActiveRoot,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	quarantineTombstoneRoot, err := parse(
		"quarantine tombstone root",
		wire.Body.QuarantineTombstoneRoot,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	bodyHash, err := parse("body hash", wire.BodyHash)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, err
	}
	signatures := make(
		[]FrostRetainedGroupCheckpointSignature,
		len(wire.Signatures),
	)
	for index, signature := range wire.Signatures {
		signatures[index] = FrostRetainedGroupCheckpointSignature{
			AuthorityID:         signature.AuthorityID,
			SignerPublicKeySPKI: signature.SignerPublicKeySPKI,
			Signature:           signature.Signature,
		}
	}
	return FrostRetainedGroupCheckpointCertificate{
		Schema: wire.Schema,
		Body: FrostRetainedGroupCheckpointBody{
			Schema:                  wire.Body.Schema,
			ProtocolBindingHash:     protocolBindingHash,
			ManifestHash:            manifestHash,
			ProfileHash:             profileHash,
			ImplementationSetHash:   implementationSetHash,
			ChainID:                 wire.Body.ChainID,
			DomainChainID:           domainChainID,
			GenesisBlockHash:        genesisBlockHash,
			AuthoritySetHash:        authoritySetHash,
			Sequence:                wire.Body.Sequence,
			PreviousCertificateHash: previousCertificateHash,
			Point:                   point,
			HistoryRoot:             historyRoot,
			CanonicalGeneration:     wire.Body.CanonicalGeneration,
			CanonicalInventoryRoot:  canonicalInventoryRoot,
			QuarantineGeneration:    wire.Body.QuarantineGeneration,
			QuarantineEventRoot:     quarantineEventRoot,
			QuarantineActiveRoot:    quarantineActiveRoot,
			QuarantineTombstoneRoot: quarantineTombstoneRoot,
		},
		BodyHash:   bodyHash,
		Signatures: signatures,
	}, nil
}

func frostRetainedGroupCheckpointBodyHash(
	body FrostRetainedGroupCheckpointBody,
) ([32]byte, error) {
	if body.Schema != frostRetainedGroupCheckpointBodySchema {
		return [32]byte{}, fmt.Errorf(
			"unsupported FROST checkpoint body schema",
		)
	}
	wire := frostRetainedGroupCheckpointCertificateToWire(
		FrostRetainedGroupCheckpointCertificate{Body: body},
	)
	return frostRetainedGroupDomainHash(
		frostRetainedGroupCheckpointBodyDomain,
		wire.Body,
	)
}

func frostRetainedGroupCheckpointSignatureHash(
	bodyHash [32]byte,
) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupCheckpointSignatureDomain))
	hasher.Write(bodyHash[:])
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result
}

func frostRetainedGroupCheckpointCertificateHash(
	certificate FrostRetainedGroupCheckpointCertificate,
) ([32]byte, error) {
	if certificate.Schema !=
		frostRetainedGroupCheckpointCertificateSchema {
		return [32]byte{}, fmt.Errorf(
			"unsupported FROST checkpoint certificate schema",
		)
	}
	bodyHash, err := frostRetainedGroupCheckpointBodyHash(certificate.Body)
	if err != nil || bodyHash != certificate.BodyHash {
		return [32]byte{}, fmt.Errorf(
			"FROST checkpoint certificate body hash mismatch",
		)
	}
	// A checkpoint's durable identity must not depend on which valid quorum
	// subset an aggregator happened to include. Otherwise, the same signed
	// body can acquire multiple predecessor hashes and permanently fork
	// journals that received different 2-of-3 or 3-of-3 encodings.
	identity := struct {
		Schema   string `json:"schema"`
		BodyHash string `json:"bodyHash"`
	}{
		Schema:   certificate.Schema,
		BodyHash: frostActivationHex32(certificate.BodyHash),
	}
	return frostRetainedGroupDomainHash(
		frostRetainedGroupCheckpointCertificateDomain,
		identity,
	)
}

func validateFrostRetainedGroupCheckpointCertificateShape(
	policy frostRetainedGroupCheckpointPolicy,
	certificate FrostRetainedGroupCheckpointCertificate,
) ([32]byte, error) {
	if certificate.Schema != frostRetainedGroupCheckpointCertificateSchema ||
		certificate.BodyHash == [32]byte{} {
		return [32]byte{}, fmt.Errorf(
			"FROST checkpoint certificate has an unsupported schema",
		)
	}
	bodyHash, err := frostRetainedGroupCheckpointBodyHash(certificate.Body)
	if err != nil || bodyHash != certificate.BodyHash {
		return [32]byte{}, fmt.Errorf(
			"FROST checkpoint certificate body hash mismatch",
		)
	}
	body := certificate.Body
	if body.ProtocolBindingHash != policy.ProtocolBindingHash ||
		body.ManifestHash != policy.ManifestHash ||
		body.ProfileHash != policy.ProfileHash ||
		body.ImplementationSetHash != policy.ImplementationSetHash ||
		body.ChainID != policy.ChainID ||
		body.DomainChainID != policy.DomainChainID ||
		body.GenesisBlockHash != policy.GenesisBlockHash ||
		body.AuthoritySetHash != policy.AuthoritySetHash {
		return [32]byte{}, fmt.Errorf(
			"FROST checkpoint certificate differs from the signed production policy",
		)
	}
	if body.Sequence == 0 ||
		body.Sequence > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.Point.BlockNumber == 0 ||
		body.Point.BlockNumber > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.Point.BlockHash == [32]byte{} ||
		body.HistoryRoot == [32]byte{} ||
		body.CanonicalGeneration < policy.CanonicalMinimum ||
		body.CanonicalGeneration > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.CanonicalInventoryRoot == [32]byte{} ||
		body.QuarantineGeneration < policy.QuarantineMinimum ||
		body.QuarantineGeneration > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.QuarantineEventRoot == [32]byte{} ||
		body.QuarantineActiveRoot == [32]byte{} ||
		body.QuarantineTombstoneRoot == [32]byte{} {
		return [32]byte{}, fmt.Errorf(
			"FROST checkpoint body is incomplete or outside canonical bounds",
		)
	}
	if uint64(len(certificate.Signatures)) < policy.AuthorityThreshold ||
		len(certificate.Signatures) > len(policy.Authorities) {
		return [32]byte{}, fmt.Errorf(
			"FROST checkpoint certificate does not carry the required quorum",
		)
	}
	authorityByID := make(
		map[string]FrostRetainedGroupAuthority,
		len(policy.Authorities),
	)
	for _, authority := range policy.Authorities {
		authorityByID[authority.AuthorityID] = authority
	}
	signatureHash := frostRetainedGroupCheckpointSignatureHash(bodyHash)
	previousID := ""
	for index, signature := range certificate.Signatures {
		if !validFrostRetainedGroupAuthorityID(signature.AuthorityID) ||
			(index > 0 && signature.AuthorityID <= previousID) {
			return [32]byte{}, fmt.Errorf(
				"FROST checkpoint signatures are not strictly sorted and unique",
			)
		}
		previousID = signature.AuthorityID
		authority, known := authorityByID[signature.AuthorityID]
		if !known {
			return [32]byte{}, fmt.Errorf(
				"FROST checkpoint certificate contains unknown authority [%s]",
				signature.AuthorityID,
			)
		}
		if len(signature.SignerPublicKeySPKI) > 2048 ||
			len(signature.Signature) > 128 {
			return [32]byte{}, fmt.Errorf(
				"FROST checkpoint authority [%s] credential exceeds its bound",
				signature.AuthorityID,
			)
		}
		publicKeyDER, err := base64.StdEncoding.Strict().DecodeString(
			signature.SignerPublicKeySPKI,
		)
		if err != nil || len(publicKeyDER) == 0 ||
			len(publicKeyDER) > 1024 ||
			base64.StdEncoding.EncodeToString(publicKeyDER) !=
				signature.SignerPublicKeySPKI ||
			sha256.Sum256(publicKeyDER) != authority.PublicKeySPKIHash {
			return [32]byte{}, fmt.Errorf(
				"FROST checkpoint authority [%s] supplied an unpinned key",
				signature.AuthorityID,
			)
		}
		parsedPublicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
		if err != nil {
			return [32]byte{}, fmt.Errorf(
				"cannot parse FROST checkpoint authority [%s] key: [%w]",
				signature.AuthorityID,
				err,
			)
		}
		publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
		if !ok || len(publicKey) != ed25519.PublicKeySize {
			return [32]byte{}, fmt.Errorf(
				"FROST checkpoint authority [%s] key is not Ed25519",
				signature.AuthorityID,
			)
		}
		if err := validateFrostRetainedGroupPrimeOrderEd25519PublicKey(
			publicKey,
		); err != nil {
			return [32]byte{}, fmt.Errorf(
				"FROST checkpoint authority [%s] key is not a nonidentity prime-order Ed25519 point: [%w]",
				signature.AuthorityID,
				err,
			)
		}
		signatureBytes, err := base64.StdEncoding.Strict().DecodeString(
			signature.Signature,
		)
		if err != nil || len(signatureBytes) != ed25519.SignatureSize ||
			base64.StdEncoding.EncodeToString(signatureBytes) !=
				signature.Signature ||
			!ed25519.Verify(publicKey, signatureHash[:], signatureBytes) {
			return [32]byte{}, fmt.Errorf(
				"FROST checkpoint authority [%s] signature is invalid",
				signature.AuthorityID,
			)
		}
	}
	certificateHash, err := frostRetainedGroupCheckpointCertificateHash(
		certificate,
	)
	if err != nil || certificateHash == [32]byte{} {
		return [32]byte{}, fmt.Errorf(
			"cannot hash FROST checkpoint certificate: [%v]",
			err,
		)
	}
	return certificateHash, nil
}

func validateFrostRetainedGroupPrimeOrderEd25519PublicKey(
	publicKey ed25519.PublicKey,
) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid Ed25519 public-key length")
	}
	parsed, err := edwards.ParsePubKey(publicKey)
	if err != nil ||
		!bytes.Equal(parsed.Serialize(), publicKey) {
		return fmt.Errorf("invalid or noncanonical Ed25519 point encoding")
	}
	if parsed.GetX().Sign() == 0 &&
		parsed.GetY().Cmp(big.NewInt(1)) == 0 {
		return fmt.Errorf("Ed25519 identity point is forbidden")
	}
	curve := edwards.Edwards()
	x, y := curve.ScalarMult(
		parsed.GetX(),
		parsed.GetY(),
		curve.Params().N.Bytes(),
	)
	if x == nil || y == nil ||
		x.Sign() != 0 ||
		y.Cmp(big.NewInt(1)) != 0 {
		return fmt.Errorf("Ed25519 point is outside the prime-order subgroup")
	}
	return nil
}

// VerifyFrostRetainedGroupCheckpointProof validates an inclusive certificate
// proof from a rollback-independent floor through the exact durable head. The
// floor cursor is supplied by an external transparency channel; the proof must
// include the corresponding full floor certificate even when floor and head
// are equal.
func VerifyFrostRetainedGroupCheckpointProof(
	bindingHash [32]byte,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
	floor FrostRetainedGroupCheckpointCursor,
	commitment FrostRetainedGroupCheckpointCommitment,
	certificates []FrostRetainedGroupCheckpointCertificate,
) error {
	policy, err := frostRetainedGroupCheckpointPolicyFromRuntimeManifest(
		bindingHash,
		runtimeManifest,
	)
	if err != nil {
		return fmt.Errorf("invalid FROST checkpoint proof policy: [%w]", err)
	}
	if floor.Sequence < policy.MinimumSequence ||
		floor.Sequence > frostRetainedGroupMaximumCanonicalJSONInteger ||
		floor.CertificateHash == [32]byte{} ||
		commitment.DurableHead.Sequence < floor.Sequence ||
		commitment.DurableHead.Sequence >
			frostRetainedGroupMaximumCanonicalJSONInteger ||
		commitment.DurableHead.CertificateHash == [32]byte{} ||
		len(certificates) == 0 ||
		uint64(len(certificates)) !=
			commitment.DurableHead.Sequence-floor.Sequence+1 ||
		uint64(len(certificates)) >
			frostRetainedGroupMaximumHandshakeAncestry+1 {
		return fmt.Errorf(
			"FROST checkpoint proof bounds or cursors are invalid",
		)
	}
	var previousHash [32]byte
	var previousBody FrostRetainedGroupCheckpointBody
	for index, certificate := range certificates {
		certificateHash, err :=
			validateFrostRetainedGroupCheckpointCertificateShape(
				policy,
				certificate,
			)
		if err != nil {
			return fmt.Errorf(
				"invalid FROST checkpoint proof certificate [%d]: [%w]",
				index,
				err,
			)
		}
		body := certificate.Body
		if index == 0 {
			if body.Sequence != floor.Sequence ||
				certificateHash != floor.CertificateHash {
				return fmt.Errorf(
					"FROST checkpoint proof does not contain the exact external floor",
				)
			}
			if body.Sequence == policy.MinimumSequence &&
				body.PreviousCertificateHash != policy.PredecessorHash {
				return fmt.Errorf(
					"FROST checkpoint proof floor does not extend the manifest predecessor",
				)
			}
			if body.Sequence > policy.MinimumSequence &&
				body.PreviousCertificateHash == [32]byte{} {
				return fmt.Errorf(
					"FROST checkpoint proof floor has no predecessor",
				)
			}
		} else if body.Sequence != previousBody.Sequence+1 ||
			body.PreviousCertificateHash != previousHash ||
			body.Point.BlockNumber <= previousBody.Point.BlockNumber ||
			body.CanonicalGeneration <
				previousBody.CanonicalGeneration ||
			body.QuarantineGeneration <
				previousBody.QuarantineGeneration {
			return fmt.Errorf(
				"FROST checkpoint proof has a gap, fork, rollback, or nonmonotonic successor",
			)
		}
		previousHash = certificateHash
		previousBody = body
	}
	if _, err := frostRetainedGroupCheckpointProofCanonicalSize(
		certificates,
		frostRetainedGroupMaximumHandshakeProofBytes,
	); err != nil {
		return err
	}

	tail := certificates[len(certificates)-1].Body
	if previousBody.Sequence != commitment.DurableHead.Sequence ||
		previousHash != commitment.DurableHead.CertificateHash ||
		tail.Point != commitment.Point ||
		tail.HistoryRoot != commitment.HistoryRoot ||
		tail.CanonicalGeneration != commitment.CanonicalGeneration ||
		tail.CanonicalInventoryRoot !=
			commitment.CanonicalInventoryRoot ||
		tail.QuarantineGeneration != commitment.QuarantineGeneration ||
		tail.QuarantineEventRoot != commitment.QuarantineEventRoot ||
		tail.QuarantineActiveRoot != commitment.QuarantineActiveRoot ||
		tail.QuarantineTombstoneRoot !=
			commitment.QuarantineTombstoneRoot {
		return fmt.Errorf(
			"FROST checkpoint proof tail differs from the exact durable commitment",
		)
	}
	return nil
}

func frostRetainedGroupCheckpointProofCanonicalSize(
	certificates []FrostRetainedGroupCheckpointCertificate,
	maximum int,
) (int, error) {
	if maximum <= 0 {
		return 0, fmt.Errorf("FROST checkpoint proof byte limit is invalid")
	}
	size := 2 // JSON array brackets.
	for index, certificate := range certificates {
		encoded, err := frostRetainedGroupCanonicalValue(
			frostRetainedGroupCheckpointCertificateToWire(certificate),
		)
		if err != nil {
			return 0, fmt.Errorf(
				"cannot encode FROST checkpoint proof certificate [%d]: [%w]",
				index,
				err,
			)
		}
		delimiter := 0
		if index > 0 {
			delimiter = 1
		}
		if len(encoded) > maximum-size-delimiter {
			return 0, fmt.Errorf(
				"FROST checkpoint proof exceeds the canonical byte limit",
			)
		}
		size += delimiter + len(encoded)
	}
	return size, nil
}

func frostRetainedGroupCheckpointChainRoot(
	bindingHash [32]byte,
	after FrostRetainedGroupCheckpointCursor,
	certificateHashes [][32]byte,
) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupCheckpointChainDomain))
	hasher.Write(bindingHash[:])
	sequence := [8]byte{}
	for index := uint(0); index < 8; index++ {
		sequence[7-index] = byte(after.Sequence >> (8 * index))
	}
	hasher.Write(sequence[:])
	hasher.Write(after.CertificateHash[:])
	count := [8]byte{}
	for index := uint(0); index < 8; index++ {
		count[7-index] = byte(uint64(len(certificateHashes)) >> (8 * index))
	}
	hasher.Write(count[:])
	for _, certificateHash := range certificateHashes {
		hasher.Write(certificateHash[:])
	}
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result
}

func validateFrostRetainedGroupCheckpointSuffix(
	policy frostRetainedGroupCheckpointPolicy,
	after FrostRetainedGroupCheckpointCursor,
	certificates []FrostRetainedGroupCheckpointCertificate,
) ([][32]byte, error) {
	if after.Sequence+1 < after.Sequence ||
		after.Sequence+1 > frostRetainedGroupMaximumCanonicalJSONInteger {
		return nil, fmt.Errorf("FROST checkpoint cursor overflows")
	}
	if after.Sequence == policy.MinimumSequence-1 {
		if after.CertificateHash != policy.PredecessorHash {
			return nil, fmt.Errorf(
				"FROST checkpoint suffix does not start at the manifest transparency floor",
			)
		}
	} else if after.Sequence < policy.MinimumSequence ||
		after.CertificateHash == [32]byte{} {
		return nil, fmt.Errorf(
			"FROST checkpoint cursor is below the manifest transparency floor",
		)
	}
	if len(certificates) == 0 {
		if after.Sequence < policy.MinimumSequence ||
			after.CertificateHash == [32]byte{} {
			return nil, fmt.Errorf(
				"fresh FROST checkpoint state requires the manifest-minimum certificate",
			)
		}
		return [][32]byte{}, nil
	}
	hashes := make([][32]byte, len(certificates))
	previousSequence := after.Sequence
	previousHash := after.CertificateHash
	var previousPoint FrostPreSignFinality
	var previousCanonicalGeneration uint64
	var previousQuarantineGeneration uint64
	for index, certificate := range certificates {
		certificateHash, err :=
			validateFrostRetainedGroupCheckpointCertificateShape(
				policy,
				certificate,
			)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid FROST checkpoint certificate [%d]: [%w]",
				index,
				err,
			)
		}
		body := certificate.Body
		if body.Sequence != previousSequence+1 ||
			body.PreviousCertificateHash != previousHash {
			return nil, fmt.Errorf(
				"FROST checkpoint sequence has a gap, fork, or rollback at [%d]",
				body.Sequence,
			)
		}
		if index > 0 &&
			(body.Point.BlockNumber <= previousPoint.BlockNumber ||
				body.CanonicalGeneration < previousCanonicalGeneration ||
				body.QuarantineGeneration < previousQuarantineGeneration) {
			return nil, fmt.Errorf(
				"FROST checkpoint point or generation is not strictly monotonic",
			)
		}
		hashes[index] = certificateHash
		previousSequence = body.Sequence
		previousHash = certificateHash
		previousPoint = body.Point
		previousCanonicalGeneration = body.CanonicalGeneration
		previousQuarantineGeneration = body.QuarantineGeneration
	}
	return hashes, nil
}

func frostRetainedGroupCertifiedStateFromHistory(
	policy frostRetainedGroupCheckpointPolicy,
	from FrostPreSignFinality,
	point FrostPreSignFinality,
	mutations []FrostRetainedGroupMutation,
) (FrostRetainedGroupCheckpointBody, error) {
	if point.BlockNumber <= from.BlockNumber ||
		point.BlockHash == [32]byte{} {
		return FrostRetainedGroupCheckpointBody{}, fmt.Errorf(
			"FROST checkpoint point is not above the empty history baseline",
		)
	}
	prefix := make([]FrostRetainedGroupMutation, 0, len(mutations))
	for _, mutation := range mutations {
		if mutation.Point.BlockNumber > point.BlockNumber {
			break
		}
		if mutation.Point.BlockNumber == point.BlockNumber &&
			mutation.Point.BlockHash != point.BlockHash {
			return FrostRetainedGroupCheckpointBody{}, fmt.Errorf(
				"FROST checkpoint point conflicts with mutation block hash",
			)
		}
		prefix = append(prefix, mutation)
	}
	canonical := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		BindingHash:  policy.ProtocolBindingHash,
		CurrentPoint: from,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	if err := applyFrostRetainedGroupMutations(
		&canonical,
		frostRetainedGroupCanonicalMutations(prefix),
	); err != nil {
		return FrostRetainedGroupCheckpointBody{}, err
	}
	canonical.CurrentPoint = point
	inventoryRoot, _, _, _, err := frostRetainedGroupInventoryRoot(canonical)
	if err != nil {
		return FrostRetainedGroupCheckpointBody{}, err
	}
	emptyActiveRoot, err := frostRetainedGroupQuarantineActiveRoot(
		policy.ProtocolBindingHash,
		map[[32]byte]frostRetainedGroupQuarantineState{},
	)
	if err != nil {
		return FrostRetainedGroupCheckpointBody{}, err
	}
	emptyTombstoneRoot, err := frostRetainedGroupQuarantineTombstoneRoot(
		policy.ProtocolBindingHash,
		map[[32]byte]frostRetainedGroupQuarantineTombstone{},
	)
	if err != nil {
		return FrostRetainedGroupCheckpointBody{}, err
	}
	quarantine := frostRetainedGroupQuarantineJournalState{
		Schema:        frostRetainedGroupQuarantineStateSchema,
		BindingHash:   policy.ProtocolBindingHash,
		CurrentPoint:  from,
		Root:          sha256.Sum256([]byte(frostRetainedGroupQuarantineDomain)),
		ActiveRoot:    emptyActiveRoot,
		TombstoneRoot: emptyTombstoneRoot,
		Quarantines:   []frostRetainedGroupQuarantineState{},
		Tombstones:    []frostRetainedGroupQuarantineTombstone{},
	}
	if err := applyFrostRetainedGroupQuarantineMutations(
		&quarantine,
		frostRetainedGroupQuarantineMutations(prefix),
		policy.LiftPolicy,
	); err != nil {
		return FrostRetainedGroupCheckpointBody{}, err
	}
	quarantine.CurrentPoint = point
	wireMutations := make(
		[]frostRetainedGroupWireMutation,
		len(prefix),
	)
	for index, mutation := range prefix {
		wireMutations[index] = frostRetainedGroupMutationToWire(mutation)
	}
	query := frostRetainedGroupHistoryQuery{
		Schema:      frostRetainedGroupHistoryRequestSchema,
		BindingHash: frostActivationHex32(policy.ProtocolBindingHash),
		From:        frostRetainedGroupFinalityToWire(from),
		To:          frostRetainedGroupFinalityToWire(point),
	}
	queryHash, err := frostRetainedGroupDomainHash(
		frostRetainedGroupHistoryQueryDomain,
		query,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointBody{}, err
	}
	historyRoot, err := frostRetainedGroupHistoryRoot(
		policy.ProtocolBindingHash,
		queryHash,
		wireMutations,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointBody{}, err
	}
	return FrostRetainedGroupCheckpointBody{
		Point:                   point,
		HistoryRoot:             historyRoot,
		CanonicalGeneration:     canonical.SnapshotGeneration,
		CanonicalInventoryRoot:  inventoryRoot,
		QuarantineGeneration:    quarantine.Generation,
		QuarantineEventRoot:     quarantine.Root,
		QuarantineActiveRoot:    quarantine.ActiveRoot,
		QuarantineTombstoneRoot: quarantine.TombstoneRoot,
	}, nil
}

func validateFrostRetainedGroupCheckpointSemantics(
	policy frostRetainedGroupCheckpointPolicy,
	history *FrostRetainedGroupHistory,
	hashes [][32]byte,
) error {
	if history == nil || len(history.Checkpoints) != len(hashes) ||
		len(history.Checkpoints) == 0 {
		return fmt.Errorf(
			"FROST checkpoint semantic history is incomplete",
		)
	}
	if history.CheckpointChainRoot !=
		frostRetainedGroupCheckpointChainRoot(
			policy.ProtocolBindingHash,
			history.CheckpointAfter,
			hashes,
		) {
		return fmt.Errorf(
			"FROST history receipt does not bind the exact checkpoint suffix",
		)
	}
	for index, certificate := range history.Checkpoints {
		expected, err := frostRetainedGroupCertifiedStateFromHistory(
			policy,
			history.From,
			certificate.Body.Point,
			history.Mutations,
		)
		if err != nil {
			return fmt.Errorf(
				"cannot derive FROST checkpoint semantic state [%d]: [%w]",
				index,
				err,
			)
		}
		body := certificate.Body
		if body.HistoryRoot != expected.HistoryRoot ||
			body.CanonicalGeneration != expected.CanonicalGeneration ||
			body.CanonicalInventoryRoot != expected.CanonicalInventoryRoot ||
			body.QuarantineGeneration != expected.QuarantineGeneration ||
			body.QuarantineEventRoot != expected.QuarantineEventRoot ||
			body.QuarantineActiveRoot != expected.QuarantineActiveRoot ||
			body.QuarantineTombstoneRoot != expected.QuarantineTombstoneRoot {
			return fmt.Errorf(
				"FROST checkpoint certificate [%d] does not commit the independently derived semantic state",
				index,
			)
		}
	}
	tail := history.Checkpoints[len(history.Checkpoints)-1]
	if hashes[len(hashes)-1] != history.CheckpointTipHash {
		return fmt.Errorf(
			"FROST checkpoint tail digest differs from the receipt",
		)
	}
	if history.CheckpointComplete &&
		(tail.Body.Point != history.To ||
			tail.Body.HistoryRoot != history.HistoryRoot) {
		return fmt.Errorf(
			"FROST checkpoint tail does not bind the exact finalized target and receipt root",
		)
	}
	if !history.CheckpointComplete &&
		tail.Body.Point.BlockNumber >= history.To.BlockNumber {
		return fmt.Errorf(
			"nonfinal FROST checkpoint page does not precede the exact finalized target",
		)
	}
	return nil
}

func frostRetainedGroupCheckpointFileName(
	sequence uint64,
	certificateHash [32]byte,
) string {
	return fmt.Sprintf(
		"%s%020d-%s%s",
		frostRetainedGroupCheckpointFilePrefix,
		sequence,
		hex.EncodeToString(certificateHash[:]),
		frostRetainedGroupJournalFileSuffix,
	)
}

func frostRetainedGroupCheckpointStateFromCertificate(
	bindingHash [32]byte,
	certificate FrostRetainedGroupCheckpointCertificate,
	certificateHash [32]byte,
) frostRetainedGroupCheckpointJournalState {
	body := certificate.Body
	return frostRetainedGroupCheckpointJournalState{
		Schema:                  frostRetainedGroupCheckpointStateSchema,
		BindingHash:             bindingHash,
		Sequence:                body.Sequence,
		CertificateHash:         certificateHash,
		Point:                   body.Point,
		HistoryRoot:             body.HistoryRoot,
		CanonicalGeneration:     body.CanonicalGeneration,
		CanonicalInventoryRoot:  body.CanonicalInventoryRoot,
		QuarantineGeneration:    body.QuarantineGeneration,
		QuarantineEventRoot:     body.QuarantineEventRoot,
		QuarantineActiveRoot:    body.QuarantineActiveRoot,
		QuarantineTombstoneRoot: body.QuarantineTombstoneRoot,
	}
}

func equalFrostRetainedGroupCheckpointStates(
	left frostRetainedGroupCheckpointJournalState,
	right frostRetainedGroupCheckpointJournalState,
) bool {
	return left == right
}

func (frgj *frostRetainedGroupJournal) initializeCheckpointJournal() error {
	entries, err := os.ReadDir(frgj.checkpointDirectory)
	if err != nil {
		return fmt.Errorf("cannot read FROST checkpoint journal: [%w]", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	metadataExists := false
	stateExists := false
	certificateNames := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == frostRetainedGroupJournalLockFile {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf("unsafe entry in FROST checkpoint journal: [%s]", name)
		}
		switch {
		case name == frostRetainedGroupCheckpointMetadataFile:
			metadataExists = true
		case name == frostRetainedGroupCheckpointStateFile:
			stateExists = true
		case strings.HasPrefix(name, frostRetainedGroupCheckpointFilePrefix) &&
			strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix):
			certificateNames = append(certificateNames, name)
		case strings.HasSuffix(name, frostRetainedGroupJournalTempSuffix):
			return fmt.Errorf(
				"interrupted FROST checkpoint journal temp is present: [%s]",
				name,
			)
		default:
			return fmt.Errorf(
				"unexpected file in FROST checkpoint journal: [%s]",
				name,
			)
		}
	}
	expectedMetadata := frostRetainedGroupCheckpointMetadata{
		Schema:             frostRetainedGroupCheckpointMetadataSchema,
		ManifestHash:       frgj.checkpointPolicy.ManifestHash,
		BindingHash:        frgj.checkpointPolicy.ProtocolBindingHash,
		AuthoritySetHash:   frgj.checkpointPolicy.AuthoritySetHash,
		AuthorityThreshold: frgj.checkpointPolicy.AuthorityThreshold,
		Authorities: append(
			[]FrostRetainedGroupAuthority{},
			frgj.checkpointPolicy.Authorities...,
		),
		MinimumSequence: frgj.checkpointPolicy.MinimumSequence,
		PredecessorHash: frgj.checkpointPolicy.PredecessorHash,
	}
	if metadataExists {
		storedMetadata := frostRetainedGroupCheckpointMetadata{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.checkpointDirectory,
			frostRetainedGroupCheckpointMetadataFile,
			&storedMetadata,
		); err != nil {
			return fmt.Errorf(
				"cannot read FROST checkpoint metadata: [%w]",
				err,
			)
		}
		stored, storedErr := frostRetainedGroupCanonicalValue(storedMetadata)
		expected, expectedErr := frostRetainedGroupCanonicalValue(expectedMetadata)
		if storedErr != nil || expectedErr != nil ||
			!bytes.Equal(stored, expected) {
			return fmt.Errorf(
				"FROST checkpoint metadata differs from the signed manifest",
			)
		}
	} else {
		if stateExists || len(certificateNames) != 0 {
			return fmt.Errorf(
				"FROST checkpoint journal has state without immutable metadata",
			)
		}
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.checkpointDirectory,
			frostRetainedGroupCheckpointMetadataFile,
			expectedMetadata,
			false,
		); err != nil {
			return fmt.Errorf(
				"cannot persist FROST checkpoint metadata: [%w]",
				err,
			)
		}
	}

	initial := frostRetainedGroupCheckpointJournalState{
		Schema:          frostRetainedGroupCheckpointStateSchema,
		BindingHash:     frgj.checkpointPolicy.ProtocolBindingHash,
		Sequence:        frgj.checkpointPolicy.MinimumSequence - 1,
		CertificateHash: frgj.checkpointPolicy.PredecessorHash,
	}
	storedState := initial
	if stateExists {
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.checkpointDirectory,
			frostRetainedGroupCheckpointStateFile,
			&storedState,
		); err != nil {
			return fmt.Errorf(
				"cannot read FROST checkpoint journal state: [%w]",
				err,
			)
		}
		if storedState.Schema != frostRetainedGroupCheckpointStateSchema ||
			storedState.BindingHash !=
				frgj.checkpointPolicy.ProtocolBindingHash {
			return fmt.Errorf(
				"unsupported or differently bound FROST checkpoint state",
			)
		}
	}

	rebuilt := initial
	matchedStored := equalFrostRetainedGroupCheckpointStates(
		storedState,
		initial,
	)
	for index, name := range certificateNames {
		wire := frostRetainedGroupWireCheckpointCertificate{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.checkpointDirectory,
			name,
			&wire,
		); err != nil {
			return fmt.Errorf(
				"cannot read immutable FROST checkpoint certificate [%s]: [%w]",
				name,
				err,
			)
		}
		certificate, err :=
			frostRetainedGroupCheckpointCertificateFromWire(wire)
		if err != nil {
			return fmt.Errorf(
				"cannot decode immutable FROST checkpoint certificate [%s]: [%w]",
				name,
				err,
			)
		}
		if rebuilt.Sequence >= frgj.checkpointPolicy.MinimumSequence &&
			(certificate.Body.Point.BlockNumber <=
				rebuilt.Point.BlockNumber ||
				certificate.Body.CanonicalGeneration <
					rebuilt.CanonicalGeneration ||
				certificate.Body.QuarantineGeneration <
					rebuilt.QuarantineGeneration) {
			return fmt.Errorf(
				"immutable FROST checkpoint certificate [%s] is not monotonic",
				name,
			)
		}
		hashes, err := validateFrostRetainedGroupCheckpointSuffix(
			frgj.checkpointPolicy,
			FrostRetainedGroupCheckpointCursor{
				Sequence:        rebuilt.Sequence,
				CertificateHash: rebuilt.CertificateHash,
			},
			[]FrostRetainedGroupCheckpointCertificate{certificate},
		)
		if err != nil {
			return fmt.Errorf(
				"invalid immutable FROST checkpoint certificate [%s]: [%w]",
				name,
				err,
			)
		}
		certificateHash := hashes[0]
		expectedName := frostRetainedGroupCheckpointFileName(
			certificate.Body.Sequence,
			certificateHash,
		)
		if name != expectedName {
			return fmt.Errorf(
				"immutable FROST checkpoint filename [%s] does not match its sequence and digest",
				name,
			)
		}
		if index > 0 &&
			certificate.Body.Sequence !=
				frgj.checkpointPolicy.MinimumSequence+uint64(index) {
			return fmt.Errorf(
				"FROST checkpoint certificate sequence has a filesystem gap",
			)
		}
		frgj.checkpointCertificates[certificate.Body.Sequence] = certificate
		frgj.checkpointHashes[certificate.Body.Sequence] = certificateHash
		rebuilt = frostRetainedGroupCheckpointStateFromCertificate(
			frgj.checkpointPolicy.ProtocolBindingHash,
			certificate,
			certificateHash,
		)
		if rebuilt.Sequence == storedState.Sequence {
			if !equalFrostRetainedGroupCheckpointStates(
				rebuilt,
				storedState,
			) {
				return fmt.Errorf(
					"FROST checkpoint state differs from its exact certificate prefix",
				)
			}
			matchedStored = true
		}
	}
	if storedState.Sequence > rebuilt.Sequence || !matchedStored {
		return fmt.Errorf(
			"FROST checkpoint state has no exact immutable certificate prefix",
		)
	}
	if rebuilt.Sequence >= frgj.checkpointPolicy.MinimumSequence {
		if err := frgj.validateCheckpointAgainstDurablePrefix(rebuilt); err != nil {
			return fmt.Errorf(
				"durable FROST checkpoint is not an exact prefix of the canonical and quarantine journals: [%w]",
				err,
			)
		}
	}
	frgj.checkpointState = rebuilt
	if !stateExists || storedState.Sequence != rebuilt.Sequence {
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.checkpointDirectory,
			frostRetainedGroupCheckpointStateFile,
			rebuilt,
			true,
		); err != nil {
			return fmt.Errorf(
				"cannot integrate orphan FROST checkpoint certificate: [%w]",
				err,
			)
		}
	}
	return nil
}

func (frgj *frostRetainedGroupJournal) validateCheckpointAgainstDurableState(
	checkpoint frostRetainedGroupCheckpointJournalState,
) error {
	if checkpoint.Point != frgj.state.CurrentPoint ||
		checkpoint.Point != frgj.quarantineState.CurrentPoint ||
		checkpoint.CanonicalGeneration != frgj.state.SnapshotGeneration ||
		checkpoint.CanonicalInventoryRoot != frgj.state.InventoryRoot ||
		checkpoint.QuarantineGeneration != frgj.quarantineState.Generation ||
		checkpoint.QuarantineEventRoot != frgj.quarantineState.Root ||
		checkpoint.QuarantineActiveRoot != frgj.quarantineState.ActiveRoot ||
		checkpoint.QuarantineTombstoneRoot !=
			frgj.quarantineState.TombstoneRoot {
		return fmt.Errorf(
			"checkpoint roots or generations differ from durable semantic state",
		)
	}
	return nil
}

func (frgj *frostRetainedGroupJournal) validateCheckpointAgainstDurablePrefix(
	checkpoint frostRetainedGroupCheckpointJournalState,
) error {
	if checkpoint.Point.BlockNumber > frgj.state.CurrentPoint.BlockNumber ||
		checkpoint.Point.BlockNumber >
			frgj.quarantineState.CurrentPoint.BlockNumber {
		return fmt.Errorf(
			"checkpoint is ahead of a durable semantic journal",
		)
	}
	mutations := append(
		cloneFrostRetainedGroupMutations(frgj.mutations),
		cloneFrostRetainedGroupMutations(frgj.quarantineMutations)...,
	)
	sort.Slice(mutations, func(i, j int) bool {
		return compareFrostRetainedGroupEventPoints(
			mutations[i].Point,
			mutations[j].Point,
		) < 0
	})
	for index := 1; index < len(mutations); index++ {
		if compareFrostRetainedGroupEventPoints(
			mutations[index-1].Point,
			mutations[index].Point,
		) >= 0 {
			return fmt.Errorf(
				"durable semantic journals overlap or disagree in event order",
			)
		}
	}
	expected, err := frostRetainedGroupCertifiedStateFromHistory(
		frgj.checkpointPolicy,
		frgj.metadata.Checkpoint,
		checkpoint.Point,
		mutations,
	)
	if err != nil {
		return err
	}
	if checkpoint.HistoryRoot != expected.HistoryRoot ||
		checkpoint.CanonicalGeneration != expected.CanonicalGeneration ||
		checkpoint.CanonicalInventoryRoot != expected.CanonicalInventoryRoot ||
		checkpoint.QuarantineGeneration != expected.QuarantineGeneration ||
		checkpoint.QuarantineEventRoot != expected.QuarantineEventRoot ||
		checkpoint.QuarantineActiveRoot != expected.QuarantineActiveRoot ||
		checkpoint.QuarantineTombstoneRoot !=
			expected.QuarantineTombstoneRoot {
		return fmt.Errorf(
			"checkpoint roots or generations differ from the durable semantic prefix",
		)
	}
	return nil
}

func (frgj *frostRetainedGroupJournal) persistCheckpointSuffix(
	certificates []FrostRetainedGroupCheckpointCertificate,
	hashes [][32]byte,
) error {
	if len(certificates) == 0 || len(certificates) != len(hashes) {
		return fmt.Errorf("FROST checkpoint certificate/hash count mismatch")
	}
	tail := len(certificates) - 1
	next := frostRetainedGroupCheckpointStateFromCertificate(
		frgj.checkpointPolicy.ProtocolBindingHash,
		certificates[tail],
		hashes[tail],
	)
	if next.Point == frgj.state.CurrentPoint &&
		next.Point == frgj.quarantineState.CurrentPoint {
		if err := frgj.validateCheckpointAgainstDurableState(next); err != nil {
			return err
		}
	} else if err := frgj.validateCheckpointAgainstDurablePrefix(next); err != nil {
		return err
	}
	existingEntries, err := os.ReadDir(frgj.checkpointDirectory)
	if err != nil {
		return fmt.Errorf(
			"cannot inspect immutable FROST checkpoint certificates: [%w]",
			err,
		)
	}
	persistedCertificates := make(
		[]FrostRetainedGroupCheckpointCertificate,
		len(certificates),
	)
	for index, certificate := range certificates {
		hash := hashes[index]
		sequence := certificate.Body.Sequence
		if _, exists := frgj.checkpointCertificates[sequence]; exists {
			return fmt.Errorf(
				"FROST checkpoint suffix contains an already persisted sequence [%d]",
				sequence,
			)
		}
		persistedCertificate, err :=
			frgj.persistOrAdoptCheckpointCertificate(
				certificate,
				hash,
				existingEntries,
			)
		if err != nil {
			return fmt.Errorf(
				"cannot persist immutable FROST checkpoint certificate [%d]: [%w]",
				sequence,
				err,
			)
		}
		persistedCertificates[index] = persistedCertificate
		if frgj.checkpointPersistFailureHook != nil {
			if err := frgj.checkpointPersistFailureHook(
				"after-checkpoint-certificate-before-next",
			); err != nil {
				return err
			}
		}
	}
	if frgj.checkpointPersistFailureHook != nil {
		if err := frgj.checkpointPersistFailureHook(
			"after-checkpoint-certificates-before-state",
		); err != nil {
			return err
		}
	}
	if err := persistFrostRetainedGroupEnvelopeAt(
		frgj.checkpointDirectory,
		frostRetainedGroupCheckpointStateFile,
		next,
		true,
	); err != nil {
		return fmt.Errorf(
			"cannot advance durable FROST checkpoint head: [%w]",
			err,
		)
	}
	if frgj.checkpointPersistFailureHook != nil {
		if err := frgj.checkpointPersistFailureHook(
			"after-checkpoint-state-before-memory",
		); err != nil {
			return err
		}
	}
	for index, certificate := range persistedCertificates {
		sequence := certificate.Body.Sequence
		frgj.checkpointCertificates[sequence] = certificate
		frgj.checkpointHashes[sequence] = hashes[index]
	}
	frgj.checkpointState = next
	return nil
}

func (frgj *frostRetainedGroupJournal) persistOrAdoptCheckpointCertificate(
	certificate FrostRetainedGroupCheckpointCertificate,
	certificateHash [32]byte,
	existingEntries []os.DirEntry,
) (FrostRetainedGroupCheckpointCertificate, error) {
	sequence := certificate.Body.Sequence
	expectedName := frostRetainedGroupCheckpointFileName(
		sequence,
		certificateHash,
	)
	sequencePrefix := fmt.Sprintf(
		"%s%020d-",
		frostRetainedGroupCheckpointFilePrefix,
		sequence,
	)
	existingName := ""
	for _, entry := range existingEntries {
		name := entry.Name()
		if !strings.HasPrefix(name, sequencePrefix) ||
			!strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix) {
			continue
		}
		if existingName != "" || name != expectedName {
			return FrostRetainedGroupCheckpointCertificate{}, fmt.Errorf(
				"conflicting immutable checkpoint certificate exists for sequence [%d]",
				sequence,
			)
		}
		existingName = name
	}
	expectedWire :=
		frostRetainedGroupCheckpointCertificateToWire(certificate)
	if existingName == "" {
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.checkpointDirectory,
			expectedName,
			expectedWire,
			false,
		); err != nil {
			return FrostRetainedGroupCheckpointCertificate{}, err
		}
		return certificate, nil
	}
	storedWire := frostRetainedGroupWireCheckpointCertificate{}
	if err := readFrostRetainedGroupEnvelopeAt(
		frgj.checkpointDirectory,
		existingName,
		&storedWire,
	); err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, fmt.Errorf(
			"cannot read orphan checkpoint certificate: [%w]",
			err,
		)
	}
	storedCertificate, err :=
		frostRetainedGroupCheckpointCertificateFromWire(storedWire)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, fmt.Errorf(
			"cannot decode orphan checkpoint certificate: [%w]",
			err,
		)
	}
	storedHash, err := validateFrostRetainedGroupCheckpointCertificateShape(
		frgj.checkpointPolicy,
		storedCertificate,
	)
	if err != nil {
		return FrostRetainedGroupCheckpointCertificate{}, fmt.Errorf(
			"cannot validate orphan checkpoint certificate: [%w]",
			err,
		)
	}
	if storedHash != certificateHash ||
		storedCertificate.Schema != certificate.Schema ||
		storedCertificate.BodyHash != certificate.BodyHash ||
		storedCertificate.Body != certificate.Body {
		return FrostRetainedGroupCheckpointCertificate{}, fmt.Errorf(
			"orphan checkpoint certificate differs from the requested checkpoint body",
		)
	}
	// The durable encoding wins when the incoming certificate carries a
	// different, but equally valid, quorum subset for the same stable body.
	return storedCertificate, nil
}

func (frgj *frostRetainedGroupJournal) checkpointDescendsFrom(
	floor FrostRetainedGroupCheckpointCursor,
) bool {
	if floor.Sequence < frgj.checkpointPolicy.MinimumSequence ||
		floor.CertificateHash == [32]byte{} ||
		floor.Sequence > frgj.checkpointState.Sequence {
		return false
	}
	hash, exists := frgj.checkpointHashes[floor.Sequence]
	return exists && hash == floor.CertificateHash
}

func (frgj *frostRetainedGroupJournal) checkpointAncestryFrom(
	floor FrostRetainedGroupCheckpointCursor,
) ([]FrostRetainedGroupCheckpointCertificate, error) {
	if !frgj.checkpointDescendsFrom(floor) {
		return nil, fmt.Errorf(
			"FROST checkpoint head does not descend from the external transparency floor",
		)
	}
	distance := frgj.checkpointState.Sequence - floor.Sequence
	if distance > frostRetainedGroupMaximumHandshakeAncestry {
		return nil, fmt.Errorf(
			"external FROST checkpoint floor is too stale; obtain a fresh rollback-independent transparency floor",
		)
	}
	result := make(
		[]FrostRetainedGroupCheckpointCertificate,
		0,
		int(distance+1),
	)
	proofBytes := 2 // JSON array brackets.
	for sequence := floor.Sequence; sequence <= frgj.checkpointState.Sequence; sequence++ {
		certificate, exists := frgj.checkpointCertificates[sequence]
		if !exists {
			return nil, fmt.Errorf(
				"FROST checkpoint ancestry is missing certificate [%d]",
				sequence,
			)
		}
		encoded, err := frostRetainedGroupCanonicalValue(
			frostRetainedGroupCheckpointCertificateToWire(certificate),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot encode FROST checkpoint ancestry certificate [%d]: [%w]",
				sequence,
				err,
			)
		}
		delimiter := 0
		if len(result) > 0 {
			delimiter = 1
		}
		if len(encoded) >
			frostRetainedGroupMaximumHandshakeProofBytes-
				proofBytes-delimiter {
			return nil, fmt.Errorf(
				"FROST checkpoint ancestry exceeds the canonical byte limit",
			)
		}
		proofBytes += delimiter + len(encoded)
		result = append(result, certificate)
	}
	return result, nil
}

package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"golang.org/x/sys/unix"
)

const (
	frostRetainedGroupJournalMetadataSchema = "tbtc-frost-retained-group-journal-metadata/v4"
	frostRetainedGroupJournalBatchSchema    = "tbtc-frost-retained-group-journal-batch/v3"
	frostRetainedGroupJournalStateSchema    = "tbtc-frost-retained-group-journal-state/v3"
	frostRetainedGroupJournalSnapshotSchema = "tbtc-frost-retained-group-journal-snapshot/v4"
	frostRetainedGroupJournalLockFile       = ".lock"
	frostRetainedGroupJournalMetadataFile   = "metadata.json"
	frostRetainedGroupJournalStateFile      = "state.json"
	frostRetainedGroupJournalBatchPrefix    = "batch-"
	frostRetainedGroupLiftCertificatePrefix = "lift-certificate-"
	frostRetainedGroupJournalFileSuffix     = ".json"
	frostRetainedGroupJournalTempSuffix     = ".tmp"
	frostRetainedGroupJournalMaximumFile    = 8 * 1024 * 1024
	frostRetainedGroupCanonicalDirectory    = "canonical"
	frostRetainedGroupQuarantineDirectory   = "quarantine"

	frostRetainedGroupQuarantineMetadataSchema = "tbtc-frost-retained-group-quarantine-metadata/v3"
	frostRetainedGroupQuarantineBatchSchema    = "tbtc-frost-retained-group-quarantine-batch/v3"
	frostRetainedGroupQuarantineStateSchema    = "tbtc-frost-retained-group-quarantine-state/v3"

	frostRetainedGroupJournalMetadataSchemaV1 = "tbtc-frost-retained-group-journal-metadata/v1"
	frostRetainedGroupJournalBatchSchemaV1    = "tbtc-frost-retained-group-journal-batch/v1"
	frostRetainedGroupJournalStateSchemaV1    = "tbtc-frost-retained-group-journal-state/v1"
	frostRetainedGroupQuarantineMetadataV1    = "tbtc-frost-retained-group-quarantine-metadata/v1"
	frostRetainedGroupQuarantineBatchV1       = "tbtc-frost-retained-group-quarantine-batch/v1"
	frostRetainedGroupQuarantineStateV1       = "tbtc-frost-retained-group-quarantine-state/v1"
	frostRetainedGroupJournalMetadataSchemaV2 = "tbtc-frost-retained-group-journal-metadata/v2"
	frostRetainedGroupJournalMetadataSchemaV3 = "tbtc-frost-retained-group-journal-metadata/v3"
	frostRetainedGroupJournalBatchSchemaV2    = "tbtc-frost-retained-group-journal-batch/v2"
	frostRetainedGroupJournalStateSchemaV2    = "tbtc-frost-retained-group-journal-state/v2"
	frostRetainedGroupQuarantineMetadataV2    = "tbtc-frost-retained-group-quarantine-metadata/v2"
	frostRetainedGroupQuarantineBatchV2       = "tbtc-frost-retained-group-quarantine-batch/v2"
	frostRetainedGroupQuarantineStateV2       = "tbtc-frost-retained-group-quarantine-state/v2"

	frostRetainedGroupInventoryEntriesDomain = "tbtc-p2tr-frost-wallet-group-inventory-entries-v1\x00"
	frostRetainedGroupInventoryLeafDomain    = "tbtc-p2tr-frost-wallet-group-inventory-leaf-v1\x00"
	frostRetainedGroupInventoryNodeDomain    = "tbtc-p2tr-frost-wallet-group-inventory-node-v1\x00"
	frostRetainedGroupInventoryRootDomain    = "tbtc-p2tr-frost-wallet-group-inventory-root-v1\x00"
	frostRetainedGroupBatchDomain            = "tbtc-frost-retained-group-journal-batch-v3\x00"
	frostRetainedGroupQuarantineBatchDomain  = "tbtc-frost-retained-group-quarantine-batch-v3\x00"
	frostRetainedGroupQuarantineDomain       = "tbtc-frost-retained-group-quarantine-event-v3\x00"
	frostRetainedGroupQuarantineActiveDomain = "tbtc-frost-retained-group-quarantine-active-root-v1\x00"
	frostRetainedGroupTombstoneRootDomain    = "tbtc-frost-retained-group-quarantine-tombstone-root-v1\x00"
	frostRetainedGroupLiftAuthorityDomain    = "tbtc-frost-retained-group-quarantine-lift-authority-set-v1\x00"
	frostRetainedGroupLiftBodyDomain         = "tbtc-frost-retained-group-quarantine-lift-body-v1\x00"
	frostRetainedGroupLiftSignatureDomain    = "tbtc-frost-retained-group-quarantine-lift-signature-v1\x00"
	frostRetainedGroupLiftCertificateDomain  = "tbtc-frost-retained-group-quarantine-lift-certificate-v1\x00"

	frostRetainedGroupLiftAuthoritySetSchema             = "tbtc-frost-retained-group-quarantine-lift-authority-set/v1"
	frostRetainedGroupLiftBodySchema                     = "tbtc-frost-retained-group-quarantine-lift-body/v1"
	frostRetainedGroupLiftCertificateSchema              = "tbtc-frost-retained-group-quarantine-lift-certificate/v1"
	frostRetainedGroupMaximumCanonicalJSONInteger uint64 = 9007199254740991
)

var errFrostRetainedGroupCheckpointRecoveryProgress = errors.New(
	"FROST checkpoint recovery advanced the durable head",
)

func frostRetainedGroupCheckpointRecoveryProgressError(
	sequence uint64,
	cause error,
) error {
	if cause == nil {
		return fmt.Errorf(
			"%w after [%d] authenticated page; retry from durable sequence [%d]",
			errFrostRetainedGroupCheckpointRecoveryProgress,
			frostRetainedGroupCheckpointPagesPerReconciliation,
			sequence,
		)
	}
	return fmt.Errorf(
		"%w after [%d] authenticated page; retry from durable sequence [%d]; post-publication verification failed: %w",
		errFrostRetainedGroupCheckpointRecoveryProgress,
		frostRetainedGroupCheckpointPagesPerReconciliation,
		sequence,
		cause,
	)
}

// FrostRetainedGroupEventPoint identifies one canonical Ethereum log. The
// transaction identity and ordering indexes prove Registry closure follows
// the matching Bridge terminal transition in the same transaction.
type FrostRetainedGroupEventPoint struct {
	BlockNumber      uint64   `json:"blockNumber"`
	BlockHash        [32]byte `json:"blockHash"`
	TransactionHash  [32]byte `json:"transactionHash"`
	TransactionIndex uint32   `json:"transactionIndex"`
	LogIndex         uint32   `json:"logIndex"`
}

func (frgep FrostRetainedGroupEventPoint) valid() bool {
	return frgep.BlockNumber > 0 && frgep.BlockHash != [32]byte{} &&
		frgep.TransactionHash != [32]byte{}
}

func compareFrostRetainedGroupEventPoints(
	left FrostRetainedGroupEventPoint,
	right FrostRetainedGroupEventPoint,
) int {
	if left.BlockNumber < right.BlockNumber {
		return -1
	}
	if left.BlockNumber > right.BlockNumber {
		return 1
	}
	if left.TransactionIndex < right.TransactionIndex {
		return -1
	}
	if left.TransactionIndex > right.TransactionIndex {
		return 1
	}
	if left.LogIndex < right.LogIndex {
		return -1
	}
	if left.LogIndex > right.LogIndex {
		return 1
	}
	return bytes.Compare(left.TransactionHash[:], right.TransactionHash[:])
}

type FrostRetainedGroupMutationKind string

const (
	FrostRetainedGroupAdmissionMutation        FrostRetainedGroupMutationKind = "admission"
	FrostRetainedGroupMovingFundsMutation      FrostRetainedGroupMutationKind = "moving-funds"
	FrostRetainedGroupClosingMutation          FrostRetainedGroupMutationKind = "closing"
	FrostRetainedGroupClosedMutation           FrostRetainedGroupMutationKind = "closed"
	FrostRetainedGroupTerminatedMutation       FrostRetainedGroupMutationKind = "terminated"
	FrostRetainedGroupRegistryClosureMutation  FrostRetainedGroupMutationKind = "registry-closure"
	FrostRetainedGroupQuarantineMutation       FrostRetainedGroupMutationKind = "quarantine"
	FrostRetainedGroupQuarantineLiftMutation   FrostRetainedGroupMutationKind = "quarantine-lift"
	FrostRetainedGroupRecoveryRequiredMutation FrostRetainedGroupMutationKind = "recovery-required"
)

type FrostRetainedGroupLifecycle string

const (
	FrostRetainedGroupLive        FrostRetainedGroupLifecycle = "Live"
	FrostRetainedGroupMovingFunds FrostRetainedGroupLifecycle = "MovingFunds"
	FrostRetainedGroupClosing     FrostRetainedGroupLifecycle = "Closing"
	FrostRetainedGroupClosed      FrostRetainedGroupLifecycle = "Closed"
	FrostRetainedGroupTerminated  FrostRetainedGroupLifecycle = "Terminated"
)

const (
	frostRetainedGroupQuarantineActive = "active"
	frostRetainedGroupQuarantineLifted = "lifted"
)

func (frgl FrostRetainedGroupLifecycle) terminal() bool {
	return frgl == FrostRetainedGroupClosed || frgl == FrostRetainedGroupTerminated
}

// FrostRetainedGroupMutation is one complete source-authenticated semantic
// history item. Admission carries exact ordered DKG operator IDs. A lift is
// accepted only with a manifest-pinned quorum certificate for the exact
// durable quarantine state it resolves.
type FrostRetainedGroupMutation struct {
	Point                   FrostRetainedGroupEventPoint                 `json:"point"`
	Kind                    FrostRetainedGroupMutationKind               `json:"kind"`
	WalletID                [32]byte                                     `json:"walletID"`
	WalletPublicKeyHash     [20]byte                                     `json:"walletPublicKeyHash"`
	OperatorIDs             []uint32                                     `json:"operatorIDs,omitempty"`
	RetainedGroupHash       [32]byte                                     `json:"retainedGroupHash,omitempty"`
	DkgResultHash           [32]byte                                     `json:"dkgResultHash,omitempty"`
	DkgSubmissionPoint      FrostRetainedGroupEventPoint                 `json:"dkgSubmissionPoint,omitempty"`
	DkgApprovalPoint        FrostRetainedGroupEventPoint                 `json:"dkgApprovalPoint,omitempty"`
	CreationPoint           FrostRetainedGroupEventPoint                 `json:"creationPoint,omitempty"`
	BridgeRegistrationPoint FrostRetainedGroupEventPoint                 `json:"bridgeRegistrationPoint,omitempty"`
	QuarantineID            [32]byte                                     `json:"quarantineID,omitempty"`
	EvidenceHash            [32]byte                                     `json:"evidenceHash,omitempty"`
	LiftCertificateHash     [32]byte                                     `json:"liftCertificateHash,omitempty"`
	LiftCertificate         *FrostRetainedGroupQuarantineLiftCertificate `json:"liftCertificate,omitempty"`
	Reason                  string                                       `json:"reason,omitempty"`
}

// FrostRetainedGroupAuthority is one manifest-pinned Ed25519 authority.
// AuthorityID is the stable, human-auditable identity used to order both the
// manifest set and certificate signatures. PublicKeySPKIHash pins the exact
// DER SubjectPublicKeyInfo supplied by a lift certificate.
type FrostRetainedGroupAuthority struct {
	AuthorityID       string   `json:"authorityID"`
	PublicKeySPKIHash [32]byte `json:"publicKeySpkiHash"`
}

type FrostRetainedGroupQuarantineRaisedRecord struct {
	QuarantineID     [32]byte                     `json:"quarantineID"`
	WalletID         [32]byte                     `json:"walletID"`
	EvidenceHash     [32]byte                     `json:"evidenceHash"`
	Reason           string                       `json:"reason"`
	RecoveryRequired bool                         `json:"recoveryRequired"`
	RaisedAt         FrostRetainedGroupEventPoint `json:"raisedAt"`
}

// FrostRetainedGroupQuarantineLiftBody binds a lift to the signed production
// deployment and to the exact durable state immediately preceding the lift.
// NotBeforeBlock and ExpiresAtBlock are inclusive canonical Ethereum block
// bounds; wall-clock time is deliberately excluded.
type FrostRetainedGroupQuarantineLiftBody struct {
	Schema                 string                                   `json:"schema"`
	ProtocolBindingHash    [32]byte                                 `json:"protocolBindingHash"`
	ManifestHash           [32]byte                                 `json:"manifestHash"`
	ProfileHash            [32]byte                                 `json:"profileHash"`
	ImplementationSetHash  [32]byte                                 `json:"implementationSetHash"`
	ChainID                uint64                                   `json:"chainID"`
	DomainChainID          [32]byte                                 `json:"domainChainID"`
	GenesisBlockHash       [32]byte                                 `json:"genesisBlockHash"`
	QuarantineProtocolID   [32]byte                                 `json:"quarantineProtocolID"`
	LiftProtocolID         [32]byte                                 `json:"liftProtocolID"`
	TombstoneProtocolID    [32]byte                                 `json:"tombstoneProtocolID"`
	AuthoritySetHash       [32]byte                                 `json:"authoritySetHash"`
	QuarantineID           [32]byte                                 `json:"quarantineID"`
	WalletID               [32]byte                                 `json:"walletID"`
	OriginalRaisedRecord   FrostRetainedGroupQuarantineRaisedRecord `json:"originalRaisedRecord"`
	PriorGeneration        uint64                                   `json:"priorGeneration"`
	PriorEventRoot         [32]byte                                 `json:"priorEventRoot"`
	PriorActiveRoot        [32]byte                                 `json:"priorActiveRoot"`
	PriorTombstoneRoot     [32]byte                                 `json:"priorTombstoneRoot"`
	LiftPoint              FrostRetainedGroupEventPoint             `json:"liftPoint"`
	ResolutionEvidenceHash [32]byte                                 `json:"resolutionEvidenceHash"`
	ResolutionFinality     FrostPreSignFinality                     `json:"resolutionFinality"`
	NotBeforeBlock         uint64                                   `json:"notBeforeBlock"`
	ExpiresAtBlock         uint64                                   `json:"expiresAtBlock"`
}

type FrostRetainedGroupQuarantineLiftSignature struct {
	AuthorityID         string `json:"authorityID"`
	SignerPublicKeySPKI string `json:"signerPublicKeySpki"`
	Signature           string `json:"signature"`
}

type FrostRetainedGroupQuarantineLiftCertificate struct {
	Schema     string                                      `json:"schema"`
	Body       FrostRetainedGroupQuarantineLiftBody        `json:"body"`
	BodyHash   [32]byte                                    `json:"bodyHash"`
	Signatures []FrostRetainedGroupQuarantineLiftSignature `json:"signatures"`
}

type FrostRetainedGroupHistory struct {
	From                FrostPreSignFinality
	To                  FrostPreSignFinality
	Mutations           []FrostRetainedGroupMutation
	HistoryRoot         [32]byte
	CheckpointAfter     FrostRetainedGroupCheckpointCursor
	Checkpoints         []FrostRetainedGroupCheckpointCertificate
	CheckpointChainRoot [32]byte
	CheckpointTipHash   [32]byte
	CheckpointComplete  bool
	Complete            bool
	EmptyAtFrom         bool
	DescriptorSetHash   [32]byte
}

// FrostRetainedGroupHistorySource is independent of the primary deployment
// verifier. It authenticates canonicality, completeness, ordering, DKG
// admissions, and operator-ID resolution at the exact requested point.
type FrostRetainedGroupHistorySource interface {
	Identity(context.Context) (FrostRetainedGroupHistoryIdentity, error)
	FinalizedHead(context.Context) (FrostPreSignFinality, error)
	VerifyPoint(context.Context, FrostPreSignFinality) error
	ReadCompleteHistory(
		context.Context,
		FrostPreSignFinality,
		FrostPreSignFinality,
		FrostRetainedGroupCheckpointCursor,
	) (*FrostRetainedGroupHistory, error)
	ResolveOperatorID(
		context.Context,
		chain.Address,
		FrostPreSignFinality,
	) (chain.OperatorID, error)
}

type FrostRetainedGroupCanonicalJournalManifest struct {
	StoreID            string
	StoreFingerprint   [32]byte
	ClusterFingerprint [32]byte
	// Checkpoint is the exclusive, canonical empty-inventory baseline. It must
	// precede every FROST creation or lifecycle event returned by the source.
	Checkpoint                FrostPreSignFinality
	DescriptorSetHash         [32]byte
	SourceTrustDomainID       string
	SourceEndpointFingerprint [32]byte
	SourceOperatorFingerprint [32]byte
	SourceIdentity            FrostRetainedGroupHistoryIdentity
	MinimumGeneration         uint64
}

type FrostRetainedGroupQuarantineJournalManifest struct {
	ProtocolID                   [32]byte
	LiftProtocolID               [32]byte
	TombstoneProtocolID          [32]byte
	CheckpointAuthorityThreshold uint64
	CheckpointAuthorities        []FrostRetainedGroupAuthority
	CheckpointMinimumSequence    uint64
	CheckpointPredecessorHash    [32]byte
	LiftAuthorityThreshold       uint64
	LiftAuthorities              []FrostRetainedGroupAuthority
	StoreID                      string
	StoreFingerprint             [32]byte
	ClusterFingerprint           [32]byte
	MinimumGeneration            uint64
}

type frostRetainedGroupQuarantineLiftPolicy struct {
	ProtocolBindingHash   [32]byte
	ManifestHash          [32]byte
	ProfileHash           [32]byte
	ImplementationSetHash [32]byte
	ChainID               uint64
	DomainChainID         [32]byte
	GenesisBlockHash      [32]byte
	QuarantineProtocolID  [32]byte
	LiftProtocolID        [32]byte
	TombstoneProtocolID   [32]byte
	AuthoritySetHash      [32]byte
	AuthorityThreshold    uint64
	Authorities           []FrostRetainedGroupAuthority
}

func frostRetainedGroupLiftAuthoritySetHash(
	threshold uint64,
	authorities []FrostRetainedGroupAuthority,
) ([32]byte, error) {
	return frostRetainedGroupAuthoritySetHash(
		frostRetainedGroupLiftAuthoritySetSchema,
		threshold,
		authorities,
	)
}

func frostRetainedGroupAuthoritySetHash(
	schema string,
	threshold uint64,
	authorities []FrostRetainedGroupAuthority,
) ([32]byte, error) {
	if strings.TrimSpace(schema) == "" || threshold < 2 || len(authorities) < 3 ||
		threshold > uint64(len(authorities)) ||
		threshold <= uint64(len(authorities))/2 {
		return [32]byte{}, fmt.Errorf(
			"FROST retained-group authority set is not a production strict majority",
		)
	}
	seenHashes := make(map[[32]byte]bool, len(authorities))
	previousID := ""
	for index, authority := range authorities {
		if !validFrostRetainedGroupAuthorityID(authority.AuthorityID) ||
			(index > 0 && authority.AuthorityID <= previousID) ||
			authority.PublicKeySPKIHash == [32]byte{} ||
			seenHashes[authority.PublicKeySPKIHash] {
			return [32]byte{}, fmt.Errorf(
				"FROST retained-group authority set is not strictly sorted and unique",
			)
		}
		previousID = authority.AuthorityID
		seenHashes[authority.PublicKeySPKIHash] = true
	}
	type wireAuthority struct {
		AuthorityID       string `json:"authorityID"`
		PublicKeySPKIHash string `json:"publicKeySpkiHash"`
	}
	wireAuthorities := make([]wireAuthority, len(authorities))
	for index, authority := range authorities {
		wireAuthorities[index] = wireAuthority{
			AuthorityID:       authority.AuthorityID,
			PublicKeySPKIHash: frostActivationHex32(authority.PublicKeySPKIHash),
		}
	}
	commitment := struct {
		Schema      string          `json:"schema"`
		Threshold   uint64          `json:"threshold"`
		Authorities []wireAuthority `json:"authorities"`
	}{
		Schema:      schema,
		Threshold:   threshold,
		Authorities: wireAuthorities,
	}
	payload, err := frostRetainedGroupCanonicalValue(commitment)
	if err != nil {
		return [32]byte{}, err
	}
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupLiftAuthorityDomain))
	hasher.Write(payload)
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

func validFrostRetainedGroupAuthorityID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index := range value {
		character := value[index]
		if !((character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			(index > 0 && (character == '-' || character == '_'))) {
			return false
		}
	}
	return true
}

func frostRetainedGroupLiftPolicyFromRuntimeManifest(
	bindingHash [32]byte,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
) (frostRetainedGroupQuarantineLiftPolicy, error) {
	quarantine := runtimeManifest.QuarantineJournal
	liftAuthoritySetHash, err := frostRetainedGroupLiftAuthoritySetHash(
		quarantine.LiftAuthorityThreshold,
		quarantine.LiftAuthorities,
	)
	if err != nil {
		return frostRetainedGroupQuarantineLiftPolicy{}, err
	}
	if _, err := frostRetainedGroupAuthoritySetHash(
		"tbtc-frost-retained-group-checkpoint-authority-set/v1",
		quarantine.CheckpointAuthorityThreshold,
		quarantine.CheckpointAuthorities,
	); err != nil {
		return frostRetainedGroupQuarantineLiftPolicy{}, err
	}
	if bindingHash == [32]byte{} ||
		runtimeManifest.ManifestHash == [32]byte{} ||
		runtimeManifest.ProfileHash == [32]byte{} ||
		runtimeManifest.ImplementationSetHash == [32]byte{} ||
		runtimeManifest.DomainChainID == [32]byte{} ||
		runtimeManifest.GenesisBlockHash == [32]byte{} ||
		quarantine.ProtocolID == [32]byte{} ||
		quarantine.LiftProtocolID == [32]byte{} ||
		quarantine.TombstoneProtocolID == [32]byte{} ||
		quarantine.ProtocolID == quarantine.LiftProtocolID ||
		quarantine.ProtocolID == quarantine.TombstoneProtocolID ||
		quarantine.LiftProtocolID == quarantine.TombstoneProtocolID {
		return frostRetainedGroupQuarantineLiftPolicy{}, fmt.Errorf(
			"FROST retained-group lift policy is incomplete",
		)
	}
	for _, value := range runtimeManifest.DomainChainID[:24] {
		if value != 0 {
			return frostRetainedGroupQuarantineLiftPolicy{}, fmt.Errorf(
				"FROST retained-group lift chain ID exceeds uint64",
			)
		}
	}
	chainID := binary.BigEndian.Uint64(runtimeManifest.DomainChainID[24:])
	if chainID == 0 {
		return frostRetainedGroupQuarantineLiftPolicy{}, fmt.Errorf(
			"FROST retained-group lift chain ID is zero",
		)
	}
	forbidden := map[[32]byte]string{
		runtimeManifest.ActivationAuthorityKeyHash:                                         "activation",
		runtimeManifest.AttestationSignerKeyHash:                                           "runtime attestation",
		runtimeManifest.HandshakeOperatorFingerprint:                                       "runtime exporter",
		runtimeManifest.CanonicalJournal.SourceOperatorFingerprint:                         "retained history source",
		runtimeManifest.VerifierOperatorFingerprint:                                        "history verifier",
		runtimeManifest.CanonicalJournal.SourceIdentity.HistorySignerKeyHash:               "retained history signer",
		runtimeManifest.CanonicalJournal.SourceIdentity.Export.TLSLeafSPKIHash:             "retained export TLS leaf",
		runtimeManifest.CanonicalJournal.SourceIdentity.Verifier.TLSLeafSPKIHash:           "retained verifier TLS leaf",
		runtimeManifest.CanonicalJournal.SourceIdentity.Export.BackendServiceFingerprint:   "retained export backend",
		runtimeManifest.CanonicalJournal.SourceIdentity.Verifier.BackendServiceFingerprint: "retained verifier backend",
		runtimeManifest.CanonicalJournal.SourceIdentity.Export.AttestationKeyHash:          "retained export attestation",
		runtimeManifest.CanonicalJournal.SourceIdentity.Verifier.AttestationKeyHash:        "retained verifier attestation",
	}
	for hash, role := range forbidden {
		if hash == [32]byte{} {
			return frostRetainedGroupQuarantineLiftPolicy{}, fmt.Errorf(
				"FROST retained-group %s role identity is unavailable",
				role,
			)
		}
	}
	for _, authority := range quarantine.CheckpointAuthorities {
		if role, exists := forbidden[authority.PublicKeySPKIHash]; exists {
			return frostRetainedGroupQuarantineLiftPolicy{}, fmt.Errorf(
				"FROST checkpoint authority aliases the %s role",
				role,
			)
		}
		forbidden[authority.PublicKeySPKIHash] = "checkpoint authority"
	}
	for _, authority := range quarantine.LiftAuthorities {
		if role, exists := forbidden[authority.PublicKeySPKIHash]; exists {
			return frostRetainedGroupQuarantineLiftPolicy{}, fmt.Errorf(
				"FROST quarantine lift authority aliases the %s role",
				role,
			)
		}
		forbidden[authority.PublicKeySPKIHash] = "quarantine lift authority"
	}
	return frostRetainedGroupQuarantineLiftPolicy{
		ProtocolBindingHash:   bindingHash,
		ManifestHash:          runtimeManifest.ManifestHash,
		ProfileHash:           runtimeManifest.ProfileHash,
		ImplementationSetHash: runtimeManifest.ImplementationSetHash,
		ChainID:               chainID,
		DomainChainID:         runtimeManifest.DomainChainID,
		GenesisBlockHash:      runtimeManifest.GenesisBlockHash,
		QuarantineProtocolID:  quarantine.ProtocolID,
		LiftProtocolID:        quarantine.LiftProtocolID,
		TombstoneProtocolID:   quarantine.TombstoneProtocolID,
		AuthoritySetHash:      liftAuthoritySetHash,
		AuthorityThreshold:    quarantine.LiftAuthorityThreshold,
		Authorities: append(
			[]FrostRetainedGroupAuthority{},
			quarantine.LiftAuthorities...,
		),
	}, nil
}

func frostRetainedGroupLiftBodyHash(
	body FrostRetainedGroupQuarantineLiftBody,
) ([32]byte, error) {
	if body.Schema != frostRetainedGroupLiftBodySchema {
		return [32]byte{}, fmt.Errorf(
			"unsupported FROST quarantine lift body schema",
		)
	}
	wireCertificate := frostRetainedGroupLiftCertificateToWire(
		&FrostRetainedGroupQuarantineLiftCertificate{Body: body},
	)
	if wireCertificate == nil {
		return [32]byte{}, fmt.Errorf(
			"cannot project FROST quarantine lift body to its wire representation",
		)
	}
	payload, err := frostRetainedGroupCanonicalValue(wireCertificate.Body)
	if err != nil {
		return [32]byte{}, err
	}
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupLiftBodyDomain))
	hasher.Write(payload)
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

func frostRetainedGroupLiftSignatureHash(bodyHash [32]byte) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupLiftSignatureDomain))
	hasher.Write(bodyHash[:])
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result
}

func frostRetainedGroupLiftCertificateHash(
	certificate FrostRetainedGroupQuarantineLiftCertificate,
) ([32]byte, error) {
	wireCertificate := frostRetainedGroupLiftCertificateToWire(&certificate)
	if wireCertificate == nil {
		return [32]byte{}, fmt.Errorf(
			"cannot project FROST quarantine lift certificate to its wire representation",
		)
	}
	payload, err := frostRetainedGroupCanonicalValue(wireCertificate)
	if err != nil {
		return [32]byte{}, err
	}
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupLiftCertificateDomain))
	hasher.Write(payload)
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

func validateFrostRetainedGroupLiftCertificateShape(
	policy frostRetainedGroupQuarantineLiftPolicy,
	certificate *FrostRetainedGroupQuarantineLiftCertificate,
) ([32]byte, error) {
	if certificate == nil ||
		certificate.Schema != frostRetainedGroupLiftCertificateSchema ||
		certificate.BodyHash == [32]byte{} {
		return [32]byte{}, fmt.Errorf(
			"FROST quarantine lift certificate is absent or has an unsupported schema",
		)
	}
	body := certificate.Body
	bodyHash, err := frostRetainedGroupLiftBodyHash(body)
	if err != nil || bodyHash != certificate.BodyHash {
		return [32]byte{}, fmt.Errorf(
			"FROST quarantine lift certificate body hash mismatch",
		)
	}
	if body.ProtocolBindingHash != policy.ProtocolBindingHash ||
		body.ManifestHash != policy.ManifestHash ||
		body.ProfileHash != policy.ProfileHash ||
		body.ImplementationSetHash != policy.ImplementationSetHash ||
		body.ChainID != policy.ChainID ||
		body.DomainChainID != policy.DomainChainID ||
		body.GenesisBlockHash != policy.GenesisBlockHash ||
		body.QuarantineProtocolID != policy.QuarantineProtocolID ||
		body.LiftProtocolID != policy.LiftProtocolID ||
		body.TombstoneProtocolID != policy.TombstoneProtocolID ||
		body.AuthoritySetHash != policy.AuthoritySetHash {
		return [32]byte{}, fmt.Errorf(
			"FROST quarantine lift certificate differs from the signed production policy",
		)
	}
	raised := body.OriginalRaisedRecord
	if body.QuarantineID == [32]byte{} ||
		body.WalletID == [32]byte{} ||
		raised.QuarantineID != body.QuarantineID ||
		raised.WalletID != body.WalletID ||
		raised.EvidenceHash == [32]byte{} ||
		strings.TrimSpace(raised.Reason) == "" ||
		len(raised.Reason) > frostRetainedGroupMaximumReasonBytes ||
		!raised.RaisedAt.valid() ||
		body.PriorGeneration == 0 ||
		body.PriorEventRoot == [32]byte{} ||
		body.PriorActiveRoot == [32]byte{} ||
		body.PriorTombstoneRoot == [32]byte{} ||
		!body.LiftPoint.valid() ||
		body.ResolutionEvidenceHash == [32]byte{} ||
		body.ResolutionFinality.BlockNumber == 0 ||
		body.ResolutionFinality.BlockHash == [32]byte{} ||
		body.ResolutionFinality.BlockNumber < raised.RaisedAt.BlockNumber ||
		(body.ResolutionFinality.BlockNumber == raised.RaisedAt.BlockNumber &&
			body.ResolutionFinality.BlockHash != raised.RaisedAt.BlockHash) ||
		body.ResolutionFinality.BlockNumber > body.LiftPoint.BlockNumber ||
		(body.ResolutionFinality.BlockNumber == body.LiftPoint.BlockNumber &&
			body.ResolutionFinality.BlockHash != body.LiftPoint.BlockHash) ||
		body.NotBeforeBlock == 0 ||
		body.ExpiresAtBlock < body.NotBeforeBlock ||
		body.LiftPoint.BlockNumber < body.NotBeforeBlock ||
		body.LiftPoint.BlockNumber > body.ExpiresAtBlock ||
		body.ResolutionFinality.BlockNumber > body.ExpiresAtBlock ||
		body.ChainID > frostRetainedGroupMaximumCanonicalJSONInteger ||
		raised.RaisedAt.BlockNumber > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.PriorGeneration > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.LiftPoint.BlockNumber > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.ResolutionFinality.BlockNumber > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.NotBeforeBlock > frostRetainedGroupMaximumCanonicalJSONInteger ||
		body.ExpiresAtBlock > frostRetainedGroupMaximumCanonicalJSONInteger {
		return [32]byte{}, fmt.Errorf(
			"FROST quarantine lift certificate body is incomplete or outside its canonical block window",
		)
	}
	if uint64(len(certificate.Signatures)) < policy.AuthorityThreshold ||
		len(certificate.Signatures) > len(policy.Authorities) {
		return [32]byte{}, fmt.Errorf(
			"FROST quarantine lift certificate does not carry the required quorum",
		)
	}
	authorityByID := make(
		map[string]FrostRetainedGroupAuthority,
		len(policy.Authorities),
	)
	for _, authority := range policy.Authorities {
		authorityByID[authority.AuthorityID] = authority
	}
	signatureHash := frostRetainedGroupLiftSignatureHash(bodyHash)
	previousID := ""
	for index, signature := range certificate.Signatures {
		if !validFrostRetainedGroupAuthorityID(signature.AuthorityID) ||
			(index > 0 && signature.AuthorityID <= previousID) {
			return [32]byte{}, fmt.Errorf(
				"FROST quarantine lift signatures are not strictly sorted and unique",
			)
		}
		previousID = signature.AuthorityID
		authority, known := authorityByID[signature.AuthorityID]
		if !known {
			return [32]byte{}, fmt.Errorf(
				"FROST quarantine lift certificate contains an unknown authority [%s]",
				signature.AuthorityID,
			)
		}
		if len(signature.SignerPublicKeySPKI) > 2048 ||
			len(signature.Signature) > 128 {
			return [32]byte{}, fmt.Errorf(
				"FROST quarantine lift authority [%s] credential exceeds its bound",
				signature.AuthorityID,
			)
		}
		publicKeyDER, err := base64.StdEncoding.Strict().DecodeString(
			signature.SignerPublicKeySPKI,
		)
		if err != nil || len(publicKeyDER) == 0 || len(publicKeyDER) > 1024 ||
			base64.StdEncoding.EncodeToString(publicKeyDER) !=
				signature.SignerPublicKeySPKI ||
			sha256.Sum256(publicKeyDER) != authority.PublicKeySPKIHash {
			return [32]byte{}, fmt.Errorf(
				"FROST quarantine lift authority [%s] supplied an unpinned key",
				signature.AuthorityID,
			)
		}
		parsedPublicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
		if err != nil {
			return [32]byte{}, fmt.Errorf(
				"cannot parse FROST quarantine lift authority [%s] key: [%w]",
				signature.AuthorityID,
				err,
			)
		}
		publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
		if !ok || len(publicKey) != ed25519.PublicKeySize {
			return [32]byte{}, fmt.Errorf(
				"FROST quarantine lift authority [%s] key is not Ed25519",
				signature.AuthorityID,
			)
		}
		if err := validateFrostRetainedGroupPrimeOrderEd25519PublicKey(
			publicKey,
		); err != nil {
			return [32]byte{}, fmt.Errorf(
				"FROST quarantine lift authority [%s] key is not a nonidentity prime-order Ed25519 point: [%w]",
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
				"FROST quarantine lift authority [%s] signature is invalid",
				signature.AuthorityID,
			)
		}
	}
	certificateHash, err := frostRetainedGroupLiftCertificateHash(*certificate)
	if err != nil {
		return [32]byte{}, fmt.Errorf(
			"cannot hash FROST quarantine lift certificate: [%w]",
			err,
		)
	}
	if certificateHash == [32]byte{} {
		return [32]byte{}, fmt.Errorf(
			"FROST quarantine lift certificate hash is zero",
		)
	}
	return certificateHash, nil
}

func validateFrostRetainedGroupLiftCertificate(
	policy frostRetainedGroupQuarantineLiftPolicy,
	state frostRetainedGroupQuarantineJournalState,
	mutation FrostRetainedGroupMutation,
	quarantine frostRetainedGroupQuarantineState,
) ([32]byte, error) {
	certificateHash, err := validateFrostRetainedGroupLiftCertificateShape(
		policy,
		mutation.LiftCertificate,
	)
	if err != nil {
		return [32]byte{}, err
	}
	body := mutation.LiftCertificate.Body
	if mutation.Kind != FrostRetainedGroupQuarantineLiftMutation ||
		mutation.QuarantineID != body.QuarantineID ||
		mutation.WalletID != body.WalletID ||
		mutation.Point != body.LiftPoint ||
		mutation.LiftCertificateHash != certificateHash ||
		quarantine.Status != frostRetainedGroupQuarantineActive ||
		quarantine.RaisedRecord != body.OriginalRaisedRecord ||
		state.Generation != body.PriorGeneration ||
		state.Root != body.PriorEventRoot ||
		state.ActiveRoot != body.PriorActiveRoot ||
		state.TombstoneRoot != body.PriorTombstoneRoot {
		return [32]byte{}, fmt.Errorf(
			"FROST quarantine lift certificate does not bind the exact active durable state",
		)
	}
	return certificateHash, nil
}

type frostRetainedGroupJournalMetadata struct {
	Schema                    string                            `json:"schema"`
	ManifestHash              [32]byte                          `json:"manifestHash"`
	BindingHash               [32]byte                          `json:"bindingHash"`
	StoreID                   string                            `json:"storeID"`
	StoreFingerprint          [32]byte                          `json:"storeFingerprint"`
	ClusterFingerprint        [32]byte                          `json:"clusterFingerprint"`
	Checkpoint                FrostPreSignFinality              `json:"checkpoint"`
	DescriptorSetHash         [32]byte                          `json:"descriptorSetHash"`
	SourceTrustDomainID       string                            `json:"sourceTrustDomainID"`
	SourceEndpointFingerprint [32]byte                          `json:"sourceEndpointFingerprint"`
	SourceOperatorFingerprint [32]byte                          `json:"sourceOperatorFingerprint"`
	SourceIdentity            FrostRetainedGroupHistoryIdentity `json:"sourceIdentity"`
}

type frostRetainedGroupWalletState struct {
	WalletID                [32]byte                     `json:"walletID"`
	WalletPublicKeyHash     [20]byte                     `json:"walletPublicKeyHash"`
	OperatorIDs             []uint32                     `json:"operatorIDs"`
	RetainedGroupHash       [32]byte                     `json:"retainedGroupHash"`
	Lifecycle               FrostRetainedGroupLifecycle  `json:"lifecycle"`
	CreationPoint           FrostRetainedGroupEventPoint `json:"creationPoint"`
	BridgeRegistrationPoint FrostRetainedGroupEventPoint `json:"bridgeRegistrationPoint"`
	LifecyclePoint          FrostRetainedGroupEventPoint `json:"lifecyclePoint"`
	LastBridgePoint         FrostRetainedGroupEventPoint `json:"lastBridgePoint"`
	RegistryClosurePoint    FrostRetainedGroupEventPoint `json:"registryClosurePoint"`
	RegistryClosed          bool                         `json:"registryClosed"`
}

type frostRetainedGroupQuarantineState struct {
	RaisedRecord        FrostRetainedGroupQuarantineRaisedRecord `json:"raisedRecord"`
	Status              string                                   `json:"status"`
	LiftCertificateHash [32]byte                                 `json:"liftCertificateHash,omitempty"`
	LiftedAt            FrostRetainedGroupEventPoint             `json:"liftedAt,omitempty"`
}

type frostRetainedGroupQuarantineTombstone struct {
	QuarantineID           [32]byte                     `json:"quarantineID"`
	WalletID               [32]byte                     `json:"walletID"`
	LiftCertificateHash    [32]byte                     `json:"liftCertificateHash"`
	LiftedAt               FrostRetainedGroupEventPoint `json:"liftedAt"`
	ResolutionEvidenceHash [32]byte                     `json:"resolutionEvidenceHash"`
	ResolutionFinality     FrostPreSignFinality         `json:"resolutionFinality"`
}

type frostRetainedGroupJournalState struct {
	Schema             string                          `json:"schema"`
	BindingHash        [32]byte                        `json:"bindingHash"`
	BatchSequence      uint64                          `json:"batchSequence"`
	CurrentPoint       FrostPreSignFinality            `json:"currentPoint"`
	SnapshotGeneration uint64                          `json:"snapshotGeneration"`
	BatchRoot          [32]byte                        `json:"batchRoot"`
	InventoryRoot      [32]byte                        `json:"inventoryRoot"`
	Wallets            []frostRetainedGroupWalletState `json:"wallets"`
}

type frostRetainedGroupJournalBatch struct {
	Schema         string                       `json:"schema"`
	BindingHash    [32]byte                     `json:"bindingHash"`
	Sequence       uint64                       `json:"sequence"`
	From           FrostPreSignFinality         `json:"from"`
	To             FrostPreSignFinality         `json:"to"`
	PriorBatchRoot [32]byte                     `json:"priorBatchRoot"`
	Mutations      []FrostRetainedGroupMutation `json:"mutations"`
	Checksum       [32]byte                     `json:"checksum"`
}

type frostRetainedGroupQuarantineMetadata struct {
	Schema                 string                        `json:"schema"`
	ManifestHash           [32]byte                      `json:"manifestHash"`
	BindingHash            [32]byte                      `json:"bindingHash"`
	ProtocolID             [32]byte                      `json:"protocolID"`
	LiftProtocolID         [32]byte                      `json:"liftProtocolID"`
	TombstoneProtocolID    [32]byte                      `json:"tombstoneProtocolID"`
	LiftAuthoritySetHash   [32]byte                      `json:"liftAuthoritySetHash"`
	LiftAuthorityThreshold uint64                        `json:"liftAuthorityThreshold"`
	LiftAuthorities        []FrostRetainedGroupAuthority `json:"liftAuthorities"`
	StoreID                string                        `json:"storeID"`
	StoreFingerprint       [32]byte                      `json:"storeFingerprint"`
	ClusterFingerprint     [32]byte                      `json:"clusterFingerprint"`
	Checkpoint             FrostPreSignFinality          `json:"checkpoint"`
}

type frostRetainedGroupQuarantineJournalState struct {
	Schema        string                                  `json:"schema"`
	BindingHash   [32]byte                                `json:"bindingHash"`
	BatchSequence uint64                                  `json:"batchSequence"`
	CurrentPoint  FrostPreSignFinality                    `json:"currentPoint"`
	Generation    uint64                                  `json:"generation"`
	BatchRoot     [32]byte                                `json:"batchRoot"`
	Root          [32]byte                                `json:"root"`
	ActiveRoot    [32]byte                                `json:"activeRoot"`
	TombstoneRoot [32]byte                                `json:"tombstoneRoot"`
	Quarantines   []frostRetainedGroupQuarantineState     `json:"quarantines"`
	Tombstones    []frostRetainedGroupQuarantineTombstone `json:"tombstones"`
}

type frostRetainedGroupQuarantineJournalBatch struct {
	Schema         string                       `json:"schema"`
	BindingHash    [32]byte                     `json:"bindingHash"`
	Sequence       uint64                       `json:"sequence"`
	From           FrostPreSignFinality         `json:"from"`
	To             FrostPreSignFinality         `json:"to"`
	PriorBatchRoot [32]byte                     `json:"priorBatchRoot"`
	Mutations      []FrostRetainedGroupMutation `json:"mutations"`
	Checksum       [32]byte                     `json:"checksum"`
}

type frostRetainedGroupWireQuarantineJournalMutation struct {
	Point               frostRetainedGroupWireEventPoint `json:"point"`
	Kind                string                           `json:"kind"`
	WalletID            string                           `json:"walletID"`
	QuarantineID        string                           `json:"quarantineID"`
	EvidenceHash        string                           `json:"evidenceHash"`
	LiftCertificateHash string                           `json:"liftCertificateHash"`
	Reason              string                           `json:"reason"`
}

type frostRetainedGroupWireQuarantineJournalBatch struct {
	Schema         string                                            `json:"schema"`
	BindingHash    string                                            `json:"bindingHash"`
	Sequence       uint64                                            `json:"sequence"`
	From           frostRetainedGroupWireFinality                    `json:"from"`
	To             frostRetainedGroupWireFinality                    `json:"to"`
	PriorBatchRoot string                                            `json:"priorBatchRoot"`
	Mutations      []frostRetainedGroupWireQuarantineJournalMutation `json:"mutations"`
	Checksum       string                                            `json:"checksum"`
}

type frostRetainedGroupEnvelope struct {
	Payload  json.RawMessage `json:"payload"`
	Checksum [32]byte        `json:"checksum"`
}

// frostRetainedGroupActiveQuarantine is one active quarantine reduced to the
// identities a signing caller can present. A quarantine - including the
// recovery-required kind - is always raised against exactly one WalletID, and
// the authority-quorum certificate that lifts it is bound to that same
// WalletID, so quarantine scope is per wallet and never node-wide.
//
// WalletPublicKeyHash is the canonical journal's exact Bridge binding for the
// quarantined wallet. It is zero when no canonical retained group carries that
// wallet ID, and such a quarantine cannot hide a locally signable wallet:
// nativeSignerInventoryExpectations derives the expected retained key-group set
// from state.Wallets alone and the native inventory must match it entry for
// entry, so a wallet absent from the canonical journal can hold no local
// signing material.
type frostRetainedGroupActiveQuarantine struct {
	QuarantineID        [32]byte
	WalletID            [32]byte
	WalletPublicKeyHash [20]byte
	RecoveryRequired    bool
}

type frostRetainedGroupJournalSnapshot struct {
	Schema                       string
	BindingHash                  [32]byte
	StoreID                      string
	StoreFingerprint             [32]byte
	ClusterFingerprint           [32]byte
	CurrentPoint                 FrostPreSignFinality
	SnapshotGeneration           uint64
	BatchRoot                    [32]byte
	InventoryRoot                [32]byte
	WalletCount                  uint64
	MinimumActualGroupSize       uint64
	MaximumActualGroupSize       uint64
	QuarantineProtocolID         [32]byte
	QuarantineStoreID            string
	QuarantineStoreFingerprint   [32]byte
	QuarantineClusterFingerprint [32]byte
	QuarantineMinimumGeneration  uint64
	QuarantineGeneration         uint64
	QuarantineRoot               [32]byte
	QuarantineActiveRoot         [32]byte
	QuarantineTombstoneRoot      [32]byte
	QuarantineCount              uint64
	ActiveQuarantines            []frostRetainedGroupActiveQuarantine
	QuarantineTombstoneCount     uint64
	CheckpointMinimumSequence    uint64
	CheckpointPredecessorHash    [32]byte
	CheckpointSequence           uint64
	CheckpointCertificateHash    [32]byte
	CheckpointHistoryRoot        [32]byte
	LocalSessionCount            uint64
	Complete                     bool
}

type frostOrphanedDKGReconcilerFunc func(
	context.Context,
	FrostPreSignFinality,
	map[[32]byte]struct{},
) error

type frostRetainedGroupJournal struct {
	mutex                        sync.Mutex
	rootDirectory                string
	directory                    string
	quarantineDirectory          string
	checkpointDirectory          string
	metadata                     frostRetainedGroupJournalMetadata
	quarantineMetadata           frostRetainedGroupQuarantineMetadata
	minimumGeneration            uint64
	quarantineMinimumGeneration  uint64
	source                       FrostRetainedGroupHistorySource
	walletRegistry               *walletRegistry
	operatorAddress              chain.Address
	lockFile                     *os.File
	quarantineLockFile           *os.File
	checkpointLockFile           *os.File
	state                        frostRetainedGroupJournalState
	quarantineState              frostRetainedGroupQuarantineJournalState
	mutations                    []FrostRetainedGroupMutation
	quarantineMutations          []FrostRetainedGroupMutation
	liftPolicy                   frostRetainedGroupQuarantineLiftPolicy
	liftCertificates             map[[32]byte]FrostRetainedGroupQuarantineLiftCertificate
	checkpointPolicy             frostRetainedGroupCheckpointPolicy
	checkpointState              frostRetainedGroupCheckpointJournalState
	checkpointCertificates       map[uint64]FrostRetainedGroupCheckpointCertificate
	checkpointHashes             map[uint64][32]byte
	orphanedDKGReconciler        frostOrphanedDKGReconcilerFunc
	persistFailureHook           func(string) error
	checkpointPersistFailureHook func(string) error
	closed                       bool
}

func newFrostRetainedGroupJournal(
	directory string,
	bindingHash [32]byte,
	runtimeManifest FrostPreSignActivationRuntimeManifest,
	source FrostRetainedGroupHistorySource,
	walletRegistry *walletRegistry,
	operatorAddress chain.Address,
) (*frostRetainedGroupJournal, error) {
	manifest := runtimeManifest.CanonicalJournal
	quarantineManifest := runtimeManifest.QuarantineJournal
	liftPolicy, liftPolicyErr := frostRetainedGroupLiftPolicyFromRuntimeManifest(
		bindingHash,
		runtimeManifest,
	)
	if liftPolicyErr != nil {
		return nil, fmt.Errorf(
			"invalid FROST retained-group quarantine lift policy: [%w]",
			liftPolicyErr,
		)
	}
	checkpointPolicy, checkpointPolicyErr :=
		frostRetainedGroupCheckpointPolicyFromRuntimeManifest(
			bindingHash,
			runtimeManifest,
		)
	if checkpointPolicyErr != nil {
		return nil, fmt.Errorf(
			"invalid FROST retained-group checkpoint policy: [%w]",
			checkpointPolicyErr,
		)
	}
	if strings.TrimSpace(directory) == "" ||
		runtimeManifest.ManifestHash == [32]byte{} ||
		bindingHash == [32]byte{} ||
		strings.TrimSpace(manifest.StoreID) == "" ||
		manifest.StoreFingerprint == [32]byte{} ||
		manifest.ClusterFingerprint == [32]byte{} ||
		manifest.Checkpoint.BlockNumber == 0 || manifest.Checkpoint.BlockHash == [32]byte{} ||
		manifest.DescriptorSetHash == [32]byte{} ||
		strings.TrimSpace(manifest.SourceTrustDomainID) == "" ||
		manifest.SourceEndpointFingerprint == [32]byte{} ||
		manifest.SourceOperatorFingerprint == [32]byte{} ||
		validateFrostRetainedGroupHistoryIdentity(manifest.SourceIdentity) != nil ||
		manifest.SourceTrustDomainID != manifest.SourceIdentity.TrustDomainID ||
		manifest.SourceEndpointFingerprint != manifest.SourceIdentity.EndpointFingerprint ||
		manifest.SourceOperatorFingerprint != manifest.SourceIdentity.OperatorFingerprint ||
		quarantineManifest.ProtocolID == [32]byte{} ||
		quarantineManifest.LiftProtocolID == [32]byte{} ||
		quarantineManifest.TombstoneProtocolID == [32]byte{} ||
		strings.TrimSpace(quarantineManifest.StoreID) == "" ||
		quarantineManifest.StoreFingerprint == [32]byte{} ||
		quarantineManifest.ClusterFingerprint == [32]byte{} ||
		manifest.StoreID == quarantineManifest.StoreID ||
		manifest.StoreFingerprint == quarantineManifest.StoreFingerprint ||
		manifest.ClusterFingerprint == quarantineManifest.ClusterFingerprint ||
		source == nil || walletRegistry == nil ||
		strings.TrimSpace(string(operatorAddress)) == "" {
		return nil, fmt.Errorf("FROST retained-group journal dependencies are incomplete")
	}
	cleanRootDirectory, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return nil, fmt.Errorf("cannot resolve FROST retained-group journal directory: [%w]", err)
	}
	if err := os.MkdirAll(cleanRootDirectory, 0700); err != nil {
		return nil, fmt.Errorf("cannot create FROST retained-group journal: [%w]", err)
	}
	if err := validateSecureBitcoinBroadcastDirectory(cleanRootDirectory); err != nil {
		return nil, fmt.Errorf("invalid FROST retained-group journal directory: [%w]", err)
	}
	if err := validateFrostRetainedGroupJournalRoot(cleanRootDirectory); err != nil {
		return nil, err
	}
	canonicalDirectory := filepath.Join(cleanRootDirectory, frostRetainedGroupCanonicalDirectory)
	quarantineDirectory := filepath.Join(cleanRootDirectory, frostRetainedGroupQuarantineDirectory)
	checkpointDirectory := filepath.Join(cleanRootDirectory, frostRetainedGroupCheckpointDirectory)
	for _, child := range []string{
		canonicalDirectory,
		quarantineDirectory,
		checkpointDirectory,
	} {
		if err := os.MkdirAll(child, 0700); err != nil {
			return nil, fmt.Errorf("cannot create FROST retained-group journal store: [%w]", err)
		}
		if err := validateSecureBitcoinBroadcastDirectory(child); err != nil {
			return nil, fmt.Errorf("invalid FROST retained-group journal store: [%w]", err)
		}
	}
	if err := syncDirectory(cleanRootDirectory); err != nil {
		return nil, fmt.Errorf("cannot sync FROST retained-group journal directory: [%w]", err)
	}
	lockFile, err := acquireFrostRetainedGroupJournalLock(canonicalDirectory)
	if err != nil {
		return nil, err
	}
	quarantineLockFile, err := acquireFrostRetainedGroupJournalLock(quarantineDirectory)
	if err != nil {
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		_ = lockFile.Close()
		return nil, err
	}
	checkpointLockFile, err := acquireFrostRetainedGroupJournalLock(
		checkpointDirectory,
	)
	if err != nil {
		_ = unix.Flock(int(quarantineLockFile.Fd()), unix.LOCK_UN)
		_ = quarantineLockFile.Close()
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		_ = lockFile.Close()
		return nil, err
	}
	journal := &frostRetainedGroupJournal{
		rootDirectory:       cleanRootDirectory,
		directory:           canonicalDirectory,
		quarantineDirectory: quarantineDirectory,
		checkpointDirectory: checkpointDirectory,
		metadata: frostRetainedGroupJournalMetadata{
			Schema:                    frostRetainedGroupJournalMetadataSchema,
			ManifestHash:              runtimeManifest.ManifestHash,
			BindingHash:               bindingHash,
			StoreID:                   manifest.StoreID,
			StoreFingerprint:          manifest.StoreFingerprint,
			ClusterFingerprint:        manifest.ClusterFingerprint,
			Checkpoint:                manifest.Checkpoint,
			DescriptorSetHash:         manifest.DescriptorSetHash,
			SourceTrustDomainID:       manifest.SourceTrustDomainID,
			SourceEndpointFingerprint: manifest.SourceEndpointFingerprint,
			SourceOperatorFingerprint: manifest.SourceOperatorFingerprint,
			SourceIdentity:            manifest.SourceIdentity,
		},
		quarantineMetadata: frostRetainedGroupQuarantineMetadata{
			Schema:                 frostRetainedGroupQuarantineMetadataSchema,
			ManifestHash:           runtimeManifest.ManifestHash,
			BindingHash:            bindingHash,
			ProtocolID:             quarantineManifest.ProtocolID,
			LiftProtocolID:         quarantineManifest.LiftProtocolID,
			TombstoneProtocolID:    quarantineManifest.TombstoneProtocolID,
			LiftAuthoritySetHash:   liftPolicy.AuthoritySetHash,
			LiftAuthorityThreshold: liftPolicy.AuthorityThreshold,
			LiftAuthorities: append(
				[]FrostRetainedGroupAuthority{},
				liftPolicy.Authorities...,
			),
			StoreID:            quarantineManifest.StoreID,
			StoreFingerprint:   quarantineManifest.StoreFingerprint,
			ClusterFingerprint: quarantineManifest.ClusterFingerprint,
			Checkpoint:         manifest.Checkpoint,
		},
		minimumGeneration:           manifest.MinimumGeneration,
		quarantineMinimumGeneration: quarantineManifest.MinimumGeneration,
		source:                      source,
		walletRegistry:              walletRegistry,
		operatorAddress:             operatorAddress,
		lockFile:                    lockFile,
		quarantineLockFile:          quarantineLockFile,
		checkpointLockFile:          checkpointLockFile,
		liftPolicy:                  liftPolicy,
		liftCertificates:            make(map[[32]byte]FrostRetainedGroupQuarantineLiftCertificate),
		checkpointPolicy:            checkpointPolicy,
		checkpointCertificates:      make(map[uint64]FrostRetainedGroupCheckpointCertificate),
		checkpointHashes:            make(map[uint64][32]byte),
	}
	if err := journal.initialize(); err != nil {
		_ = journal.close()
		return nil, err
	}
	return journal, nil
}

func validateFrostRetainedGroupJournalRoot(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("cannot read FROST retained-group journal root: [%w]", err)
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 ||
			(entry.Name() != frostRetainedGroupCanonicalDirectory &&
				entry.Name() != frostRetainedGroupQuarantineDirectory &&
				entry.Name() != frostRetainedGroupCheckpointDirectory) ||
			!entry.IsDir() {
			return fmt.Errorf("unsafe entry in FROST retained-group journal root: [%s]", entry.Name())
		}
	}
	return nil
}

func acquireFrostRetainedGroupJournalLock(directory string) (*os.File, error) {
	lockPath := filepath.Join(directory, frostRetainedGroupJournalLockFile)
	file, err := openSecureBitcoinBroadcastFile(lockPath, unix.O_CREAT|unix.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot open FROST retained-group journal lock: [%w]", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("FROST retained-group journal is already owned by another process")
	}
	if err := file.Truncate(0); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	if _, err := file.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	if err := syncDirectory(directory); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (frgj *frostRetainedGroupJournal) close() error {
	frgj.mutex.Lock()
	defer frgj.mutex.Unlock()
	if frgj.closed {
		return nil
	}
	frgj.closed = true
	var result error
	for _, lock := range []*os.File{
		frgj.checkpointLockFile,
		frgj.quarantineLockFile,
		frgj.lockFile,
	} {
		if lock == nil {
			continue
		}
		if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil && result == nil {
			result = err
		}
		if err := lock.Close(); err != nil && result == nil {
			result = err
		}
	}
	frgj.lockFile = nil
	frgj.quarantineLockFile = nil
	frgj.checkpointLockFile = nil
	return result
}

func (frgj *frostRetainedGroupJournal) initialize() error {
	if err := frgj.verifyHistorySourceIdentity(context.Background()); err != nil {
		return err
	}
	if err := recoverFrostRetainedGroupJournalTemporaryFiles(
		frgj.directory,
	); err != nil {
		return fmt.Errorf(
			"cannot recover interrupted FROST retained-group journal persistence: [%w]",
			err,
		)
	}

	entries, err := os.ReadDir(frgj.directory)
	if err != nil {
		return fmt.Errorf("cannot read FROST retained-group journal: [%w]", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	metadataExists := false
	stateExists := false
	batchNames := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == frostRetainedGroupJournalLockFile {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf("unsafe entry in FROST retained-group journal: [%s]", name)
		}
		switch {
		case name == frostRetainedGroupJournalMetadataFile:
			metadataExists = true
		case name == frostRetainedGroupJournalStateFile:
			stateExists = true
		case strings.HasPrefix(name, frostRetainedGroupJournalBatchPrefix) &&
			strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix):
			batchNames = append(batchNames, name)
		default:
			return fmt.Errorf("unexpected file in FROST retained-group journal: [%s]", name)
		}
	}

	if metadataExists {
		stored := frostRetainedGroupJournalMetadata{}
		if err := frgj.readEnvelope(frostRetainedGroupJournalMetadataFile, &stored); err != nil {
			return fmt.Errorf("cannot read FROST retained-group journal metadata: [%w]", err)
		}
		if stored.Schema == frostRetainedGroupJournalMetadataSchemaV1 ||
			stored.Schema == frostRetainedGroupJournalMetadataSchemaV2 ||
			stored.Schema == frostRetainedGroupJournalMetadataSchemaV3 {
			return frostRetainedGroupLegacySchemaError("canonical metadata")
		}
		if stored != frgj.metadata {
			return fmt.Errorf("FROST retained-group journal metadata differs from signed manifest")
		}
	} else {
		if stateExists || len(batchNames) != 0 {
			return fmt.Errorf("FROST retained-group journal has state without immutable metadata")
		}
		if err := frgj.persistEnvelope(frostRetainedGroupJournalMetadataFile, &frgj.metadata, false); err != nil {
			return fmt.Errorf("cannot persist FROST retained-group journal metadata: [%w]", err)
		}
	}

	initial := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		BindingHash:  frgj.metadata.BindingHash,
		CurrentPoint: frgj.metadata.Checkpoint,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	initial.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(initial)
	if err != nil {
		return err
	}

	stored := initial
	if stateExists {
		if err := frgj.readEnvelope(frostRetainedGroupJournalStateFile, &stored); err != nil {
			return fmt.Errorf("cannot read FROST retained-group journal state: [%w]", err)
		}
		if stored.Schema == frostRetainedGroupJournalStateSchemaV1 ||
			stored.Schema == frostRetainedGroupJournalStateSchemaV2 {
			return frostRetainedGroupLegacySchemaError("canonical state")
		}
		if stored.Schema != frostRetainedGroupJournalStateSchema {
			return fmt.Errorf("unsupported FROST retained-group journal state schema")
		}
	} else if len(batchNames) != 0 {
		return fmt.Errorf("FROST retained-group journal has batches without state checkpoint")
	}

	rebuilt := initial
	rebuiltMutations := make([]FrostRetainedGroupMutation, 0)
	matchedStored := stored.BatchSequence == 0
	if matchedStored {
		if err := equalFrostRetainedGroupStates(stored, rebuilt); err != nil {
			return err
		}
	}
	for index, name := range batchNames {
		expectedSequence := uint64(index + 1)
		if name != frostRetainedGroupBatchFileName(expectedSequence) {
			return fmt.Errorf("FROST retained-group journal batch sequence has a gap at [%d]", expectedSequence)
		}
		batch := frostRetainedGroupJournalBatch{}
		if err := frgj.readEnvelope(name, &batch); err != nil {
			return fmt.Errorf("cannot read FROST retained-group journal batch [%d]: [%w]", expectedSequence, err)
		}
		if err := validateFrostRetainedGroupBatch(batch, rebuilt); err != nil {
			return fmt.Errorf("invalid FROST retained-group journal batch [%d]: [%w]", expectedSequence, err)
		}
		if err := applyFrostRetainedGroupMutations(&rebuilt, batch.Mutations); err != nil {
			return fmt.Errorf("cannot replay FROST retained-group batch [%d]: [%w]", expectedSequence, err)
		}
		rebuilt.BatchSequence = batch.Sequence
		rebuilt.CurrentPoint = batch.To
		rebuilt.BatchRoot = frostRetainedGroupBatchRoot(batch.PriorBatchRoot, batch.Checksum)
		rebuilt.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(rebuilt)
		if err != nil {
			return err
		}
		rebuiltMutations = append(rebuiltMutations, cloneFrostRetainedGroupMutations(batch.Mutations)...)
		if rebuilt.BatchSequence == stored.BatchSequence {
			if err := equalFrostRetainedGroupStates(stored, rebuilt); err != nil {
				return err
			}
			matchedStored = true
		}
	}
	if stored.BatchSequence > uint64(len(batchNames)) || !matchedStored {
		return fmt.Errorf("FROST retained-group state checkpoint has no exact batch prefix")
	}
	frgj.state = rebuilt
	frgj.mutations = rebuiltMutations
	if !stateExists || stored.BatchSequence != rebuilt.BatchSequence {
		if err := frgj.persistEnvelope(frostRetainedGroupJournalStateFile, &rebuilt, true); err != nil {
			return fmt.Errorf("cannot integrate orphan FROST retained-group batch: [%w]", err)
		}
	}
	if err := frgj.initializeQuarantine(); err != nil {
		return err
	}
	return frgj.initializeCheckpointJournal()
}

func (frgj *frostRetainedGroupJournal) verifyHistorySourceIdentity(
	ctx context.Context,
) error {
	identity, err := frgj.source.Identity(ctx)
	if err != nil {
		return fmt.Errorf("cannot authenticate FROST retained-group history source: [%w]", err)
	}
	if identity.TrustDomainID != frgj.metadata.SourceTrustDomainID ||
		identity.EndpointFingerprint != frgj.metadata.SourceEndpointFingerprint ||
		identity.OperatorFingerprint != frgj.metadata.SourceOperatorFingerprint ||
		identity != frgj.metadata.SourceIdentity {
		return fmt.Errorf("FROST retained-group history source identity differs from signed manifest")
	}
	return nil
}

func equalFrostRetainedGroupStates(
	stored frostRetainedGroupJournalState,
	rebuilt frostRetainedGroupJournalState,
) error {
	storedBytes, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	rebuiltBytes, err := json.Marshal(rebuilt)
	if err != nil {
		return err
	}
	if !bytes.Equal(storedBytes, rebuiltBytes) {
		return fmt.Errorf("FROST retained-group state differs from exact journal prefix")
	}
	return nil
}

func (frgj *frostRetainedGroupJournal) initializeQuarantine() error {
	if err := recoverFrostRetainedGroupJournalTemporaryFiles(
		frgj.quarantineDirectory,
	); err != nil {
		return fmt.Errorf(
			"cannot recover interrupted FROST retained-group quarantine persistence: [%w]",
			err,
		)
	}
	entries, err := os.ReadDir(frgj.quarantineDirectory)
	if err != nil {
		return fmt.Errorf("cannot read FROST retained-group quarantine journal: [%w]", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	metadataExists := false
	stateExists := false
	batchNames := make([]string, 0)
	certificateNames := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == frostRetainedGroupJournalLockFile {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf("unsafe entry in FROST retained-group quarantine journal: [%s]", name)
		}
		switch {
		case name == frostRetainedGroupJournalMetadataFile:
			metadataExists = true
		case name == frostRetainedGroupJournalStateFile:
			stateExists = true
		case strings.HasPrefix(name, frostRetainedGroupJournalBatchPrefix) &&
			strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix):
			batchNames = append(batchNames, name)
		case strings.HasPrefix(name, frostRetainedGroupLiftCertificatePrefix) &&
			strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix):
			certificateNames = append(certificateNames, name)
		default:
			return fmt.Errorf("unexpected file in FROST retained-group quarantine journal: [%s]", name)
		}
	}

	if metadataExists {
		stored := frostRetainedGroupQuarantineMetadata{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalMetadataFile,
			&stored,
		); err != nil {
			return fmt.Errorf("cannot read FROST retained-group quarantine metadata: [%w]", err)
		}
		if stored.Schema == frostRetainedGroupQuarantineMetadataV1 ||
			stored.Schema == frostRetainedGroupQuarantineMetadataV2 {
			return frostRetainedGroupLegacySchemaError("quarantine metadata")
		}
		storedBytes, storedErr := frostRetainedGroupCanonicalValue(stored)
		expectedBytes, expectedErr := frostRetainedGroupCanonicalValue(
			frgj.quarantineMetadata,
		)
		if storedErr != nil || expectedErr != nil ||
			!bytes.Equal(storedBytes, expectedBytes) {
			return fmt.Errorf("FROST retained-group quarantine metadata differs from signed manifest")
		}
	} else {
		if stateExists || len(batchNames) != 0 || len(certificateNames) != 0 {
			return fmt.Errorf("FROST retained-group quarantine journal has state without immutable metadata")
		}
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalMetadataFile,
			&frgj.quarantineMetadata,
			false,
		); err != nil {
			return fmt.Errorf("cannot persist FROST retained-group quarantine metadata: [%w]", err)
		}
	}
	for _, name := range certificateNames {
		wireCertificate := frostRetainedGroupWireQuarantineLiftCertificate{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			name,
			&wireCertificate,
		); err != nil {
			return fmt.Errorf(
				"cannot read immutable FROST quarantine lift certificate [%s]: [%w]",
				name,
				err,
			)
		}
		certificate, err := frostRetainedGroupLiftCertificateFromWire(
			&wireCertificate,
		)
		if err != nil || certificate == nil {
			return fmt.Errorf(
				"cannot decode immutable FROST quarantine lift certificate [%s]: [%v]",
				name,
				err,
			)
		}
		certificateHash, err := validateFrostRetainedGroupLiftCertificateShape(
			frgj.liftPolicy,
			certificate,
		)
		if err != nil {
			return fmt.Errorf(
				"invalid immutable FROST quarantine lift certificate [%s]: [%w]",
				name,
				err,
			)
		}
		if name != frostRetainedGroupLiftCertificateFileName(certificateHash) {
			return fmt.Errorf(
				"immutable FROST quarantine lift certificate filename [%s] does not match its digest",
				name,
			)
		}
		if _, exists := frgj.liftCertificates[certificateHash]; exists {
			return fmt.Errorf(
				"duplicate immutable FROST quarantine lift certificate [%s]",
				name,
			)
		}
		frgj.liftCertificates[certificateHash] = *certificate
	}

	emptyActiveRoot, err := frostRetainedGroupQuarantineActiveRoot(
		frgj.quarantineMetadata.BindingHash,
		map[[32]byte]frostRetainedGroupQuarantineState{},
	)
	if err != nil {
		return err
	}
	emptyTombstoneRoot, err := frostRetainedGroupQuarantineTombstoneRoot(
		frgj.quarantineMetadata.BindingHash,
		map[[32]byte]frostRetainedGroupQuarantineTombstone{},
	)
	if err != nil {
		return err
	}
	initial := frostRetainedGroupQuarantineJournalState{
		Schema:        frostRetainedGroupQuarantineStateSchema,
		BindingHash:   frgj.quarantineMetadata.BindingHash,
		CurrentPoint:  frgj.quarantineMetadata.Checkpoint,
		Root:          sha256.Sum256([]byte(frostRetainedGroupQuarantineDomain)),
		ActiveRoot:    emptyActiveRoot,
		TombstoneRoot: emptyTombstoneRoot,
		Quarantines:   []frostRetainedGroupQuarantineState{},
		Tombstones:    []frostRetainedGroupQuarantineTombstone{},
	}
	stored := initial
	if stateExists {
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalStateFile,
			&stored,
		); err != nil {
			return fmt.Errorf("cannot read FROST retained-group quarantine state: [%w]", err)
		}
		if stored.Schema == frostRetainedGroupQuarantineStateV1 ||
			stored.Schema == frostRetainedGroupQuarantineStateV2 {
			return frostRetainedGroupLegacySchemaError("quarantine state")
		}
		if stored.Schema != frostRetainedGroupQuarantineStateSchema {
			return fmt.Errorf("unsupported FROST retained-group quarantine state schema")
		}
	} else if len(batchNames) != 0 {
		return fmt.Errorf("FROST retained-group quarantine journal has batches without state checkpoint")
	}

	rebuilt := initial
	rebuiltMutations := make([]FrostRetainedGroupMutation, 0)
	matchedStored := stored.BatchSequence == 0
	if matchedStored {
		if err := equalFrostRetainedGroupQuarantineStates(stored, rebuilt); err != nil {
			return err
		}
	}
	for index, name := range batchNames {
		expectedSequence := uint64(index + 1)
		if name != frostRetainedGroupBatchFileName(expectedSequence) {
			return fmt.Errorf("FROST retained-group quarantine batch sequence has a gap at [%d]", expectedSequence)
		}
		wireBatch := frostRetainedGroupWireQuarantineJournalBatch{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			name,
			&wireBatch,
		); err != nil {
			return fmt.Errorf("cannot read FROST retained-group quarantine batch [%d]: [%w]", expectedSequence, err)
		}
		batch, err := frostRetainedGroupQuarantineBatchFromWire(
			wireBatch,
			frgj.liftCertificates,
		)
		if err != nil {
			return fmt.Errorf(
				"cannot decode FROST retained-group quarantine batch [%d]: [%w]",
				expectedSequence,
				err,
			)
		}
		if err := validateFrostRetainedGroupQuarantineBatch(batch, rebuilt); err != nil {
			return fmt.Errorf("invalid FROST retained-group quarantine batch [%d]: [%w]", expectedSequence, err)
		}
		if err := frgj.validatePersistedLiftCertificates(batch.Mutations); err != nil {
			return fmt.Errorf(
				"invalid persisted FROST quarantine lift certificate in batch [%d]: [%w]",
				expectedSequence,
				err,
			)
		}
		if err := applyFrostRetainedGroupQuarantineMutations(
			&rebuilt,
			batch.Mutations,
			frgj.liftPolicy,
		); err != nil {
			return fmt.Errorf("cannot replay FROST retained-group quarantine batch [%d]: [%w]", expectedSequence, err)
		}
		rebuilt.BatchSequence = batch.Sequence
		rebuilt.CurrentPoint = batch.To
		rebuilt.BatchRoot = frostRetainedGroupQuarantineBatchRoot(batch.PriorBatchRoot, batch.Checksum)
		rebuiltMutations = append(rebuiltMutations, cloneFrostRetainedGroupMutations(batch.Mutations)...)
		if rebuilt.BatchSequence == stored.BatchSequence {
			if err := equalFrostRetainedGroupQuarantineStates(stored, rebuilt); err != nil {
				return err
			}
			matchedStored = true
		}
	}
	if stored.BatchSequence > uint64(len(batchNames)) || !matchedStored {
		return fmt.Errorf("FROST retained-group quarantine state checkpoint has no exact batch prefix")
	}
	frgj.quarantineState = rebuilt
	frgj.quarantineMutations = rebuiltMutations
	if !stateExists || stored.BatchSequence != rebuilt.BatchSequence {
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalStateFile,
			&rebuilt,
			true,
		); err != nil {
			return fmt.Errorf("cannot integrate orphan FROST retained-group quarantine batch: [%w]", err)
		}
	}
	return nil
}

func equalFrostRetainedGroupQuarantineStates(
	stored frostRetainedGroupQuarantineJournalState,
	rebuilt frostRetainedGroupQuarantineJournalState,
) error {
	storedBytes, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	rebuiltBytes, err := json.Marshal(rebuilt)
	if err != nil {
		return err
	}
	if !bytes.Equal(storedBytes, rebuiltBytes) {
		return fmt.Errorf("FROST retained-group quarantine state differs from exact journal prefix")
	}
	return nil
}

func frostRetainedGroupBatchFileName(sequence uint64) string {
	return fmt.Sprintf("%s%020d%s", frostRetainedGroupJournalBatchPrefix, sequence, frostRetainedGroupJournalFileSuffix)
}

func frostRetainedGroupLiftCertificateFileName(
	certificateHash [32]byte,
) string {
	return fmt.Sprintf(
		"%s%s%s",
		frostRetainedGroupLiftCertificatePrefix,
		hex.EncodeToString(certificateHash[:]),
		frostRetainedGroupJournalFileSuffix,
	)
}

func (frgj *frostRetainedGroupJournal) validatePersistedLiftCertificates(
	mutations []FrostRetainedGroupMutation,
) error {
	for _, mutation := range mutations {
		if mutation.Kind != FrostRetainedGroupQuarantineLiftMutation {
			continue
		}
		certificateHash, err := validateFrostRetainedGroupLiftCertificateShape(
			frgj.liftPolicy,
			mutation.LiftCertificate,
		)
		if err != nil {
			return err
		}
		if mutation.LiftCertificateHash != certificateHash {
			return fmt.Errorf(
				"lift mutation certificate reference does not match its immutable certificate",
			)
		}
		stored, exists := frgj.liftCertificates[certificateHash]
		if !exists {
			return fmt.Errorf(
				"immutable lift certificate [%x] is absent",
				certificateHash,
			)
		}
		equal, err := equalFrostRetainedGroupLiftCertificates(
			stored,
			*mutation.LiftCertificate,
		)
		if err != nil {
			return fmt.Errorf(
				"immutable lift certificate [%x] is not byte-identical: [%w]",
				certificateHash,
				err,
			)
		}
		if !equal {
			return fmt.Errorf(
				"immutable lift certificate [%x] is not byte-identical",
				certificateHash,
			)
		}
	}
	return nil
}

func (frgj *frostRetainedGroupJournal) ensureLiftCertificatesPersisted(
	mutations []FrostRetainedGroupMutation,
) error {
	for _, mutation := range mutations {
		if mutation.Kind != FrostRetainedGroupQuarantineLiftMutation {
			continue
		}
		certificateHash, err := validateFrostRetainedGroupLiftCertificateShape(
			frgj.liftPolicy,
			mutation.LiftCertificate,
		)
		if err != nil {
			return err
		}
		if mutation.LiftCertificateHash != certificateHash {
			return fmt.Errorf(
				"lift mutation certificate reference does not match its immutable certificate",
			)
		}
		if stored, exists := frgj.liftCertificates[certificateHash]; exists {
			equal, err := equalFrostRetainedGroupLiftCertificates(
				stored,
				*mutation.LiftCertificate,
			)
			if err != nil {
				return fmt.Errorf(
					"immutable lift certificate [%x] conflicts with a persisted certificate: [%w]",
					certificateHash,
					err,
				)
			}
			if !equal {
				return fmt.Errorf(
					"immutable lift certificate [%x] conflicts with a persisted certificate",
					certificateHash,
				)
			}
			continue
		}
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupLiftCertificateFileName(certificateHash),
			frostRetainedGroupLiftCertificateToWire(
				mutation.LiftCertificate,
			),
			false,
		); err != nil {
			return fmt.Errorf(
				"cannot persist immutable FROST quarantine lift certificate [%x]: [%w]",
				certificateHash,
				err,
			)
		}
		certificate := *mutation.LiftCertificate
		certificate.Signatures = append(
			[]FrostRetainedGroupQuarantineLiftSignature{},
			mutation.LiftCertificate.Signatures...,
		)
		frgj.liftCertificates[certificateHash] = certificate
	}
	return nil
}

func equalFrostRetainedGroupLiftCertificates(
	left FrostRetainedGroupQuarantineLiftCertificate,
	right FrostRetainedGroupQuarantineLiftCertificate,
) (bool, error) {
	leftWire := frostRetainedGroupLiftCertificateToWire(&left)
	rightWire := frostRetainedGroupLiftCertificateToWire(&right)
	leftBytes, err := frostRetainedGroupCanonicalValue(leftWire)
	if err != nil {
		return false, err
	}
	rightBytes, err := frostRetainedGroupCanonicalValue(rightWire)
	if err != nil {
		return false, err
	}
	return bytes.Equal(leftBytes, rightBytes), nil
}

func frostRetainedGroupQuarantineBatchToWire(
	batch frostRetainedGroupQuarantineJournalBatch,
) (frostRetainedGroupWireQuarantineJournalBatch, error) {
	mutations := make(
		[]frostRetainedGroupWireQuarantineJournalMutation,
		len(batch.Mutations),
	)
	for index, mutation := range batch.Mutations {
		if !isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			return frostRetainedGroupWireQuarantineJournalBatch{}, fmt.Errorf(
				"canonical inventory mutation cannot enter a quarantine batch",
			)
		}
		if mutation.Kind == FrostRetainedGroupQuarantineLiftMutation {
			if mutation.LiftCertificateHash == [32]byte{} ||
				mutation.LiftCertificate == nil {
				return frostRetainedGroupWireQuarantineJournalBatch{}, fmt.Errorf(
					"quarantine lift batch mutation has no certificate reference",
				)
			}
		} else if mutation.LiftCertificateHash != [32]byte{} ||
			mutation.LiftCertificate != nil {
			return frostRetainedGroupWireQuarantineJournalBatch{}, fmt.Errorf(
				"non-lift quarantine batch mutation carries a certificate reference",
			)
		}
		mutations[index] = frostRetainedGroupWireQuarantineJournalMutation{
			Point:               frostRetainedGroupEventPointToWire(mutation.Point),
			Kind:                string(mutation.Kind),
			WalletID:            frostActivationHex32(mutation.WalletID),
			QuarantineID:        frostActivationHex32(mutation.QuarantineID),
			EvidenceHash:        frostActivationHex32(mutation.EvidenceHash),
			LiftCertificateHash: frostActivationHex32(mutation.LiftCertificateHash),
			Reason:              mutation.Reason,
		}
	}
	return frostRetainedGroupWireQuarantineJournalBatch{
		Schema:         batch.Schema,
		BindingHash:    frostActivationHex32(batch.BindingHash),
		Sequence:       batch.Sequence,
		From:           frostRetainedGroupFinalityToWire(batch.From),
		To:             frostRetainedGroupFinalityToWire(batch.To),
		PriorBatchRoot: frostActivationHex32(batch.PriorBatchRoot),
		Mutations:      mutations,
		Checksum:       frostActivationHex32(batch.Checksum),
	}, nil
}

func frostRetainedGroupQuarantineBatchFromWire(
	wire frostRetainedGroupWireQuarantineJournalBatch,
	certificates map[[32]byte]FrostRetainedGroupQuarantineLiftCertificate,
) (frostRetainedGroupQuarantineJournalBatch, error) {
	bindingHash, err := parseFrostActivationHex32(wire.BindingHash)
	if err != nil {
		return frostRetainedGroupQuarantineJournalBatch{}, err
	}
	from, err := frostRetainedGroupFinalityFromWire(wire.From)
	if err != nil {
		return frostRetainedGroupQuarantineJournalBatch{}, err
	}
	to, err := frostRetainedGroupFinalityFromWire(wire.To)
	if err != nil {
		return frostRetainedGroupQuarantineJournalBatch{}, err
	}
	priorBatchRoot, err := parseFrostActivationHex32(wire.PriorBatchRoot)
	if err != nil {
		return frostRetainedGroupQuarantineJournalBatch{}, err
	}
	checksum, err := parseFrostActivationHex32(wire.Checksum)
	if err != nil {
		return frostRetainedGroupQuarantineJournalBatch{}, err
	}
	mutations := make([]FrostRetainedGroupMutation, len(wire.Mutations))
	for index, wireMutation := range wire.Mutations {
		point, err := frostRetainedGroupEventPointFromWire(wireMutation.Point)
		if err != nil || !point.valid() {
			return frostRetainedGroupQuarantineJournalBatch{}, fmt.Errorf(
				"quarantine batch mutation [%d] point is invalid",
				index,
			)
		}
		walletID, err := parseFrostActivationHex32(wireMutation.WalletID)
		if err != nil {
			return frostRetainedGroupQuarantineJournalBatch{}, err
		}
		quarantineID, err := parseFrostActivationHex32(
			wireMutation.QuarantineID,
		)
		if err != nil {
			return frostRetainedGroupQuarantineJournalBatch{}, err
		}
		evidenceHash, err := parseFrostActivationHex32(wireMutation.EvidenceHash)
		if err != nil {
			return frostRetainedGroupQuarantineJournalBatch{}, err
		}
		certificateHash, err := parseFrostActivationHex32(
			wireMutation.LiftCertificateHash,
		)
		if err != nil {
			return frostRetainedGroupQuarantineJournalBatch{}, err
		}
		mutation := FrostRetainedGroupMutation{
			Point:               point,
			Kind:                FrostRetainedGroupMutationKind(wireMutation.Kind),
			WalletID:            walletID,
			QuarantineID:        quarantineID,
			EvidenceHash:        evidenceHash,
			LiftCertificateHash: certificateHash,
			Reason:              wireMutation.Reason,
		}
		if mutation.Kind == FrostRetainedGroupQuarantineLiftMutation {
			certificate, exists := certificates[certificateHash]
			if certificateHash == [32]byte{} || !exists {
				return frostRetainedGroupQuarantineJournalBatch{}, fmt.Errorf(
					"quarantine lift batch mutation [%d] references an absent certificate",
					index,
				)
			}
			certificate.Signatures = append(
				[]FrostRetainedGroupQuarantineLiftSignature{},
				certificate.Signatures...,
			)
			mutation.LiftCertificate = &certificate
		} else if certificateHash != [32]byte{} {
			return frostRetainedGroupQuarantineJournalBatch{}, fmt.Errorf(
				"non-lift quarantine batch mutation [%d] references a certificate",
				index,
			)
		}
		mutations[index] = mutation
	}
	return frostRetainedGroupQuarantineJournalBatch{
		Schema:         wire.Schema,
		BindingHash:    bindingHash,
		Sequence:       wire.Sequence,
		From:           from,
		To:             to,
		PriorBatchRoot: priorBatchRoot,
		Mutations:      mutations,
		Checksum:       checksum,
	}, nil
}

func frostRetainedGroupQuarantineBatchCanonicalValue(
	batch frostRetainedGroupQuarantineJournalBatch,
) ([]byte, error) {
	wire, err := frostRetainedGroupQuarantineBatchToWire(batch)
	if err != nil {
		return nil, err
	}
	return frostRetainedGroupCanonicalValue(wire)
}

func frostRetainedGroupLegacySchemaError(component string) error {
	return fmt.Errorf(
		"prior FROST retained-group %s store schema is not safely migratable; "+
			"the signed activation manifest must provision a new empty v3 store identity",
		component,
	)
}

func validateFrostRetainedGroupBatch(
	batch frostRetainedGroupJournalBatch,
	prior frostRetainedGroupJournalState,
) error {
	if batch.Schema == frostRetainedGroupJournalBatchSchemaV1 ||
		batch.Schema == frostRetainedGroupJournalBatchSchemaV2 {
		return frostRetainedGroupLegacySchemaError("canonical batch")
	}
	if batch.Schema != frostRetainedGroupJournalBatchSchema ||
		batch.BindingHash == [32]byte{} || batch.BindingHash != prior.BindingHash ||
		batch.Sequence != prior.BatchSequence+1 || batch.From != prior.CurrentPoint ||
		batch.PriorBatchRoot != prior.BatchRoot || batch.To.BlockNumber < batch.From.BlockNumber ||
		batch.To.BlockHash == [32]byte{} || batch.Checksum == [32]byte{} {
		return fmt.Errorf("batch header is invalid")
	}
	declared := batch.Checksum
	batch.Checksum = [32]byte{}
	payload, err := frostRetainedGroupCanonicalValue(batch)
	if err != nil {
		return err
	}
	if sha256.Sum256(payload) != declared {
		return fmt.Errorf("batch checksum mismatch")
	}
	if err := validateFrostRetainedGroupBatchMutationBounds(batch.From, batch.To, batch.Mutations); err != nil {
		return err
	}
	for _, mutation := range batch.Mutations {
		if isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			return fmt.Errorf("canonical batch contains a quarantine mutation")
		}
	}
	return nil
}

func validateFrostRetainedGroupQuarantineBatch(
	batch frostRetainedGroupQuarantineJournalBatch,
	prior frostRetainedGroupQuarantineJournalState,
) error {
	if batch.Schema == frostRetainedGroupQuarantineBatchV1 ||
		batch.Schema == frostRetainedGroupQuarantineBatchV2 {
		return frostRetainedGroupLegacySchemaError("quarantine batch")
	}
	if batch.Schema != frostRetainedGroupQuarantineBatchSchema ||
		batch.BindingHash == [32]byte{} || batch.BindingHash != prior.BindingHash ||
		batch.Sequence != prior.BatchSequence+1 || batch.From != prior.CurrentPoint ||
		batch.PriorBatchRoot != prior.BatchRoot || batch.To.BlockNumber < batch.From.BlockNumber ||
		batch.To.BlockHash == [32]byte{} || batch.Checksum == [32]byte{} {
		return fmt.Errorf("quarantine batch header is invalid")
	}
	declared := batch.Checksum
	batch.Checksum = [32]byte{}
	payload, err := frostRetainedGroupQuarantineBatchCanonicalValue(batch)
	if err != nil {
		return err
	}
	if sha256.Sum256(payload) != declared {
		return fmt.Errorf("quarantine batch checksum mismatch")
	}
	if err := validateFrostRetainedGroupBatchMutationBounds(batch.From, batch.To, batch.Mutations); err != nil {
		return err
	}
	for _, mutation := range batch.Mutations {
		if !isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			return fmt.Errorf("quarantine batch contains a canonical inventory mutation")
		}
	}
	return nil
}

func validateFrostRetainedGroupBatchMutationBounds(
	from FrostPreSignFinality,
	to FrostPreSignFinality,
	mutations []FrostRetainedGroupMutation,
) error {
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range mutations {
		if !mutation.Point.valid() || mutation.Point.BlockNumber <= from.BlockNumber ||
			mutation.Point.BlockNumber > to.BlockNumber ||
			(index > 0 && compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("batch mutations are outside cursor bounds or not strictly ordered")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("batch mutations disagree on canonical block hash")
		}
		previous = mutation.Point
	}
	return nil
}

func frostRetainedGroupBatchRoot(prior [32]byte, checksum [32]byte) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupBatchDomain))
	hasher.Write(prior[:])
	hasher.Write(checksum[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

func frostRetainedGroupQuarantineBatchRoot(
	prior [32]byte,
	checksum [32]byte,
) [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupQuarantineBatchDomain))
	hasher.Write(prior[:])
	hasher.Write(checksum[:])
	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

func (frgj *frostRetainedGroupJournal) readEnvelope(name string, target interface{}) error {
	return readFrostRetainedGroupEnvelopeAt(frgj.directory, name, target)
}

func readFrostRetainedGroupEnvelopeAt(
	directory string,
	name string,
	target interface{},
) error {
	if err := validateFrostRetainedGroupJournalFileName(name); err != nil {
		return err
	}
	file, err := openSecureBitcoinBroadcastFile(filepath.Join(directory, name), unix.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, frostRetainedGroupJournalMaximumFile+1))
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > frostRetainedGroupJournalMaximumFile {
		return fmt.Errorf("journal file size is invalid")
	}
	envelope := frostRetainedGroupEnvelope{}
	if err := decodeStrictFrostActivationJSON(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Payload) == 0 || sha256.Sum256(envelope.Payload) != envelope.Checksum {
		return fmt.Errorf("journal file checksum mismatch")
	}
	return decodeStrictFrostActivationJSON(envelope.Payload, target)
}

func (frgj *frostRetainedGroupJournal) persistEnvelope(
	name string,
	payload interface{},
	replace bool,
) error {
	return persistFrostRetainedGroupEnvelopeAt(frgj.directory, name, payload, replace)
}

func persistFrostRetainedGroupEnvelopeAt(
	directory string,
	name string,
	payload interface{},
	replace bool,
) error {
	if err := validateFrostRetainedGroupJournalFileName(name); err != nil {
		return err
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	envelopeBytes, err := json.Marshal(frostRetainedGroupEnvelope{
		Payload:  payloadBytes,
		Checksum: sha256.Sum256(payloadBytes),
	})
	if err != nil {
		return err
	}

	directoryFile, err := openFrostRetainedGroupJournalDirectory(directory)
	if err != nil {
		return err
	}
	defer directoryFile.Close()
	directoryDescriptor := int(directoryFile.Fd())

	if exists, err := validateFrostRetainedGroupJournalFileAt(
		directoryDescriptor,
		name,
	); err != nil {
		return err
	} else if exists {
		if !replace {
			return fmt.Errorf("immutable journal file already exists: [%s]", name)
		}
	}

	temporary, temporaryName, err := createFrostRetainedGroupJournalTemporaryFileAt(
		directoryDescriptor,
		directory,
		name,
	)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		if remove {
			_ = unix.Unlinkat(directoryDescriptor, temporaryName, 0)
		}
	}()
	if _, err := temporary.Write(envelopeBytes); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if !replace {
		// Link is an atomic no-replace publication: unlike Rename it cannot
		// overwrite an immutable batch or metadata file that appeared after
		// the destination check above.
		if err := unix.Linkat(
			directoryDescriptor,
			temporaryName,
			directoryDescriptor,
			name,
			0,
		); err != nil {
			return fmt.Errorf("cannot publish immutable journal file [%s]: [%w]", name, err)
		}
		if err := unix.Unlinkat(directoryDescriptor, temporaryName, 0); err != nil {
			return err
		}
		remove = false
		return directoryFile.Sync()
	}
	if err := unix.Renameat(
		directoryDescriptor,
		temporaryName,
		directoryDescriptor,
		name,
	); err != nil {
		return err
	}
	remove = false
	return directoryFile.Sync()
}

func validateFrostRetainedGroupJournalFileName(name string) error {
	if !filepath.IsLocal(name) || filepath.Base(name) != name {
		return fmt.Errorf("FROST retained-group journal file name is not local: [%s]", name)
	}
	if name == frostRetainedGroupJournalMetadataFile ||
		name == frostRetainedGroupJournalStateFile {
		return nil
	}
	if strings.HasPrefix(name, frostRetainedGroupCheckpointFilePrefix) {
		checkpointText := strings.TrimSuffix(
			strings.TrimPrefix(name, frostRetainedGroupCheckpointFilePrefix),
			frostRetainedGroupJournalFileSuffix,
		)
		sequenceText, digestText, separated := strings.Cut(checkpointText, "-")
		digestBytes, digestErr := hex.DecodeString(digestText)
		sequence, sequenceErr := strconv.ParseUint(sequenceText, 10, 64)
		if !separated || len(sequenceText) != 20 || sequenceErr != nil ||
			sequence == 0 || digestErr != nil || len(digestBytes) != 32 {
			return fmt.Errorf(
				"noncanonical FROST retained-group checkpoint certificate name: [%s]",
				name,
			)
		}
		digest := [32]byte{}
		copy(digest[:], digestBytes)
		if frostRetainedGroupCheckpointFileName(sequence, digest) != name {
			return fmt.Errorf(
				"noncanonical FROST retained-group checkpoint certificate name: [%s]",
				name,
			)
		}
		return nil
	}
	if strings.HasPrefix(name, frostRetainedGroupLiftCertificatePrefix) {
		digestText := strings.TrimSuffix(
			strings.TrimPrefix(name, frostRetainedGroupLiftCertificatePrefix),
			frostRetainedGroupJournalFileSuffix,
		)
		digestBytes, err := hex.DecodeString(digestText)
		if err != nil || len(digestBytes) != 32 {
			return fmt.Errorf(
				"noncanonical FROST retained-group lift certificate name: [%s]",
				name,
			)
		}
		digest := [32]byte{}
		copy(digest[:], digestBytes)
		if frostRetainedGroupLiftCertificateFileName(digest) != name {
			return fmt.Errorf(
				"noncanonical FROST retained-group lift certificate name: [%s]",
				name,
			)
		}
		return nil
	}
	if !strings.HasPrefix(name, frostRetainedGroupJournalBatchPrefix) ||
		!strings.HasSuffix(name, frostRetainedGroupJournalFileSuffix) {
		return fmt.Errorf("unsupported FROST retained-group journal file name: [%s]", name)
	}
	sequenceText := strings.TrimSuffix(
		strings.TrimPrefix(name, frostRetainedGroupJournalBatchPrefix),
		frostRetainedGroupJournalFileSuffix,
	)
	if len(sequenceText) != 20 {
		return fmt.Errorf("noncanonical FROST retained-group journal batch name: [%s]", name)
	}
	sequence, err := strconv.ParseUint(sequenceText, 10, 64)
	if err != nil || sequence == 0 ||
		frostRetainedGroupBatchFileName(sequence) != name {
		return fmt.Errorf("noncanonical FROST retained-group journal batch name: [%s]", name)
	}
	return nil
}

func openFrostRetainedGroupJournalDirectory(directory string) (*os.File, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, fmt.Errorf("FROST retained-group journal directory is not canonical")
	}
	fd, err := unix.Open(
		directory,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), directory)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("cannot wrap FROST retained-group journal directory descriptor")
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0700 {
		_ = file.Close()
		return nil, fmt.Errorf("FROST retained-group journal directory is unsafe")
	}
	if err := validateBitcoinBroadcastOwner(info); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func validateFrostRetainedGroupJournalFileAt(
	directoryDescriptor int,
	name string,
) (bool, error) {
	var info unix.Stat_t
	err := unix.Fstatat(
		directoryDescriptor,
		name,
		&info,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if uint32(info.Mode)&unix.S_IFMT != unix.S_IFREG ||
		uint32(info.Mode)&0777 != 0600 {
		return false, fmt.Errorf("journal destination is unsafe: [%s]", name)
	}
	if info.Uid != uint32(os.Geteuid()) {
		return false, fmt.Errorf(
			"Bitcoin broadcast storage is owned by uid [%d], expected [%d]",
			info.Uid,
			os.Geteuid(),
		)
	}
	return true, nil
}

// recoverFrostRetainedGroupJournalTemporaryFiles removes interrupted
// pre-publication files after validating that they have exactly the private,
// owner-bound shape and unpredictable name emitted by persistFrostRetainedGroupEnvelopeAt.
// A temporary name is never a commit point: immutable files commit at Linkat
// and replaceable state commits at Renameat. The final namespace is replayed
// after this cleanup, so an already-linked immutable file is retained and an
// unpublished state file is deterministically rebuilt from its immutable
// prefix.
func recoverFrostRetainedGroupJournalTemporaryFiles(
	directory string,
) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	directoryFile, err := openFrostRetainedGroupJournalDirectory(directory)
	if err != nil {
		return err
	}
	defer directoryFile.Close()
	directoryDescriptor := int(directoryFile.Fd())

	removed := false
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, frostRetainedGroupJournalTempSuffix) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf(
				"interrupted journal temporary file is unsafe: [%s]",
				name,
			)
		}
		if _, err := frostRetainedGroupJournalTemporaryFinalName(name); err != nil {
			return err
		}
		exists, err := validateFrostRetainedGroupJournalFileAt(
			directoryDescriptor,
			name,
		)
		if err != nil {
			return fmt.Errorf(
				"interrupted journal temporary file is unsafe [%s]: [%w]",
				name,
				err,
			)
		}
		if !exists {
			return fmt.Errorf(
				"interrupted journal temporary file disappeared: [%s]",
				name,
			)
		}
		if err := unix.Unlinkat(directoryDescriptor, name, 0); err != nil {
			return fmt.Errorf(
				"cannot remove interrupted journal temporary file [%s]: [%w]",
				name,
				err,
			)
		}
		removed = true
	}
	if removed {
		if err := directoryFile.Sync(); err != nil {
			return fmt.Errorf(
				"cannot sync interrupted journal temporary-file recovery: [%w]",
				err,
			)
		}
	}
	return nil
}

func frostRetainedGroupJournalTemporaryFinalName(
	name string,
) (string, error) {
	const entropyBytes = 16
	const entropyCharacters = entropyBytes * 2

	if !strings.HasSuffix(name, frostRetainedGroupJournalTempSuffix) {
		return "", fmt.Errorf(
			"interrupted journal temporary file name is invalid: [%s]",
			name,
		)
	}
	trimmed := strings.TrimSuffix(name, frostRetainedGroupJournalTempSuffix)
	delimiterIndex := len(trimmed) - entropyCharacters - 1
	if delimiterIndex <= 0 || trimmed[delimiterIndex] != '-' {
		return "", fmt.Errorf(
			"interrupted journal temporary file name is invalid: [%s]",
			name,
		)
	}
	entropyText := trimmed[delimiterIndex+1:]
	entropy, err := hex.DecodeString(entropyText)
	if err != nil || len(entropy) != entropyBytes ||
		hex.EncodeToString(entropy) != entropyText {
		return "", fmt.Errorf(
			"interrupted journal temporary file name is invalid: [%s]",
			name,
		)
	}
	finalName := trimmed[:delimiterIndex]
	if err := validateFrostRetainedGroupJournalFileName(finalName); err != nil {
		return "", fmt.Errorf(
			"interrupted journal temporary destination is invalid [%s]: [%w]",
			name,
			err,
		)
	}
	return finalName, nil
}

func createFrostRetainedGroupJournalTemporaryFileAt(
	directoryDescriptor int,
	directory string,
	finalName string,
) (*os.File, string, error) {
	const maximumAttempts = 128
	var entropy [16]byte
	for attempt := 0; attempt < maximumAttempts; attempt++ {
		if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
			return nil, "", fmt.Errorf("cannot generate journal temporary file name: [%w]", err)
		}
		name := finalName + "-" + hex.EncodeToString(entropy[:]) +
			frostRetainedGroupJournalTempSuffix
		fd, err := unix.Openat(
			directoryDescriptor,
			name,
			unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW,
			0600,
		)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		file := os.NewFile(uintptr(fd), filepath.Join(directory, name))
		if file == nil {
			_ = unix.Close(fd)
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", fmt.Errorf("cannot wrap journal temporary file descriptor")
		}
		if err := file.Chmod(0600); err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", err
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
			info.Mode().Perm() != 0600 {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", fmt.Errorf("journal temporary file is unsafe")
		}
		if err := validateBitcoinBroadcastOwner(info); err != nil {
			_ = file.Close()
			_ = unix.Unlinkat(directoryDescriptor, name, 0)
			return nil, "", err
		}
		return file, name, nil
	}
	return nil, "", fmt.Errorf("cannot allocate a unique journal temporary file")
}

func applyFrostRetainedGroupMutations(
	state *frostRetainedGroupJournalState,
	mutations []FrostRetainedGroupMutation,
) error {
	if state == nil {
		return fmt.Errorf("FROST retained-group state is nil")
	}
	if len(state.Wallets) > frostRetainedGroupMaximumWallets {
		return fmt.Errorf("FROST retained-group wallet state exceeds the wallet limit")
	}
	wallets := make(map[[32]byte]frostRetainedGroupWalletState, len(state.Wallets))
	publicKeyHashes := make(
		map[[20]byte][32]byte,
		len(state.Wallets),
	)
	for _, wallet := range state.Wallets {
		if wallet.WalletID == [32]byte{} ||
			wallet.WalletPublicKeyHash == [20]byte{} ||
			wallets[wallet.WalletID].WalletID != [32]byte{} {
			return fmt.Errorf("FROST retained-group wallet state is duplicate or invalid")
		}
		if _, exists := publicKeyHashes[wallet.WalletPublicKeyHash]; exists {
			return fmt.Errorf("FROST retained-group wallet state repeats a public-key hash")
		}
		wallet.OperatorIDs = append([]uint32{}, wallet.OperatorIDs...)
		wallets[wallet.WalletID] = wallet
		publicKeyHashes[wallet.WalletPublicKeyHash] = wallet.WalletID
	}
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range mutations {
		if !mutation.Point.valid() || (index > 0 &&
			compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("FROST retained-group mutations are not strictly ordered")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("FROST retained-group mutations disagree on canonical block hash")
		}
		previous = mutation.Point
		wallet := wallets[mutation.WalletID]
		switch mutation.Kind {
		case FrostRetainedGroupAdmissionMutation:
			if mutation.WalletID == [32]byte{} || mutation.WalletPublicKeyHash == [20]byte{} ||
				len(mutation.OperatorIDs) < 51 || len(mutation.OperatorIDs) > 100 ||
				mutation.RetainedGroupHash == [32]byte{} || mutation.DkgResultHash == [32]byte{} ||
				!mutation.DkgSubmissionPoint.valid() || !mutation.DkgApprovalPoint.valid() ||
				!mutation.CreationPoint.valid() || !mutation.BridgeRegistrationPoint.valid() ||
				mutation.Point != mutation.BridgeRegistrationPoint ||
				compareFrostRetainedGroupEventPoints(mutation.DkgSubmissionPoint, mutation.DkgApprovalPoint) >= 0 ||
				!sameFrostRetainedGroupTransaction(mutation.DkgApprovalPoint, mutation.CreationPoint) ||
				compareFrostRetainedGroupEventPoints(mutation.DkgApprovalPoint, mutation.CreationPoint) >= 0 ||
				!sameFrostRetainedGroupTransaction(mutation.CreationPoint, mutation.BridgeRegistrationPoint) ||
				compareFrostRetainedGroupEventPoints(mutation.CreationPoint, mutation.BridgeRegistrationPoint) >= 0 ||
				wallet.WalletID != [32]byte{} || mutation.QuarantineID != [32]byte{} {
				return fmt.Errorf("invalid or duplicate FROST retained-group admission")
			}
			if len(wallets) >= frostRetainedGroupMaximumWallets {
				return fmt.Errorf("FROST retained-group admission exceeds the wallet limit")
			}
			if _, exists := publicKeyHashes[mutation.WalletPublicKeyHash]; exists {
				return fmt.Errorf("FROST retained-group admission reuses a wallet public-key hash")
			}
			for _, operatorID := range mutation.OperatorIDs {
				if operatorID == 0 {
					return fmt.Errorf("FROST retained-group admission contains zero operator ID")
				}
			}
			wallets[mutation.WalletID] = frostRetainedGroupWalletState{
				WalletID:                mutation.WalletID,
				WalletPublicKeyHash:     mutation.WalletPublicKeyHash,
				OperatorIDs:             append([]uint32{}, mutation.OperatorIDs...),
				RetainedGroupHash:       mutation.RetainedGroupHash,
				Lifecycle:               FrostRetainedGroupLive,
				CreationPoint:           mutation.CreationPoint,
				BridgeRegistrationPoint: mutation.BridgeRegistrationPoint,
				LifecyclePoint:          mutation.BridgeRegistrationPoint,
				LastBridgePoint:         mutation.Point,
			}
			publicKeyHashes[mutation.WalletPublicKeyHash] = mutation.WalletID
			state.SnapshotGeneration++
		case FrostRetainedGroupMovingFundsMutation,
			FrostRetainedGroupClosingMutation,
			FrostRetainedGroupClosedMutation,
			FrostRetainedGroupTerminatedMutation:
			if wallet.WalletID == [32]byte{} ||
				mutation.WalletPublicKeyHash != wallet.WalletPublicKeyHash ||
				len(mutation.OperatorIDs) != 0 || wallet.Lifecycle.terminal() {
				return fmt.Errorf("invalid FROST retained-group lifecycle mutation")
			}
			next := FrostRetainedGroupLifecycle("")
			switch mutation.Kind {
			case FrostRetainedGroupMovingFundsMutation:
				next = FrostRetainedGroupMovingFunds
			case FrostRetainedGroupClosingMutation:
				next = FrostRetainedGroupClosing
			case FrostRetainedGroupClosedMutation:
				next = FrostRetainedGroupClosed
			case FrostRetainedGroupTerminatedMutation:
				next = FrostRetainedGroupTerminated
			}
			if !validFrostRetainedGroupTransition(wallet.Lifecycle, next) {
				return fmt.Errorf("invalid FROST retained-group transition [%s -> %s]", wallet.Lifecycle, next)
			}
			wallet.Lifecycle = next
			wallet.LifecyclePoint = mutation.Point
			wallet.LastBridgePoint = mutation.Point
			wallets[mutation.WalletID] = wallet
			state.SnapshotGeneration++
		case FrostRetainedGroupRegistryClosureMutation:
			if wallet.WalletID == [32]byte{} || !wallet.Lifecycle.terminal() || wallet.RegistryClosed ||
				mutation.WalletPublicKeyHash != wallet.WalletPublicKeyHash ||
				mutation.Point.BlockNumber != wallet.LastBridgePoint.BlockNumber ||
				mutation.Point.BlockHash != wallet.LastBridgePoint.BlockHash ||
				mutation.Point.TransactionHash != wallet.LastBridgePoint.TransactionHash ||
				mutation.Point.TransactionIndex != wallet.LastBridgePoint.TransactionIndex ||
				mutation.Point.LogIndex <= wallet.LastBridgePoint.LogIndex {
				return fmt.Errorf("FROST Registry closure does not follow its Bridge terminal event")
			}
			wallet.RegistryClosed = true
			wallet.RegistryClosurePoint = mutation.Point
			wallets[mutation.WalletID] = wallet
			state.SnapshotGeneration++
		case FrostRetainedGroupQuarantineMutation,
			FrostRetainedGroupRecoveryRequiredMutation,
			FrostRetainedGroupQuarantineLiftMutation:
			return fmt.Errorf("quarantine mutation cannot enter canonical retained-group journal")
		default:
			return fmt.Errorf("unknown FROST retained-group mutation kind [%s]", mutation.Kind)
		}
	}
	state.Wallets = state.Wallets[:0]
	for _, wallet := range wallets {
		state.Wallets = append(state.Wallets, wallet)
	}
	sort.Slice(state.Wallets, func(i, j int) bool {
		return bytes.Compare(state.Wallets[i].WalletID[:], state.Wallets[j].WalletID[:]) < 0
	})
	return nil
}

func isFrostRetainedGroupQuarantineMutation(
	kind FrostRetainedGroupMutationKind,
) bool {
	return kind == FrostRetainedGroupQuarantineMutation ||
		kind == FrostRetainedGroupRecoveryRequiredMutation ||
		kind == FrostRetainedGroupQuarantineLiftMutation
}

func frostRetainedGroupQuarantineMutations(
	mutations []FrostRetainedGroupMutation,
) []FrostRetainedGroupMutation {
	result := make([]FrostRetainedGroupMutation, 0)
	for _, mutation := range mutations {
		if isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			result = append(result, mutation)
		}
	}
	return cloneFrostRetainedGroupMutations(result)
}

func frostRetainedGroupCanonicalMutations(
	mutations []FrostRetainedGroupMutation,
) []FrostRetainedGroupMutation {
	result := make([]FrostRetainedGroupMutation, 0)
	for _, mutation := range mutations {
		if !isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			result = append(result, mutation)
		}
	}
	return cloneFrostRetainedGroupMutations(result)
}

func applyFrostRetainedGroupQuarantineMutations(
	state *frostRetainedGroupQuarantineJournalState,
	mutations []FrostRetainedGroupMutation,
	policy frostRetainedGroupQuarantineLiftPolicy,
) error {
	if state == nil {
		return fmt.Errorf("FROST retained-group quarantine state is nil")
	}
	quarantines := make(
		map[[32]byte]frostRetainedGroupQuarantineState,
		len(state.Quarantines),
	)
	for _, quarantine := range state.Quarantines {
		quarantineID := quarantine.RaisedRecord.QuarantineID
		if quarantineID == [32]byte{} ||
			quarantine.RaisedRecord.WalletID == [32]byte{} ||
			quarantine.RaisedRecord.EvidenceHash == [32]byte{} ||
			strings.TrimSpace(quarantine.RaisedRecord.Reason) == "" ||
			!quarantine.RaisedRecord.RaisedAt.valid() ||
			(quarantine.Status != frostRetainedGroupQuarantineActive &&
				quarantine.Status != frostRetainedGroupQuarantineLifted) {
			return fmt.Errorf("FROST retained-group quarantine state is duplicate or invalid")
		}
		if _, exists := quarantines[quarantineID]; exists {
			return fmt.Errorf("FROST retained-group quarantine state is duplicate or invalid")
		}
		quarantines[quarantineID] = quarantine
	}
	tombstones := make(
		map[[32]byte]frostRetainedGroupQuarantineTombstone,
		len(state.Tombstones),
	)
	for _, tombstone := range state.Tombstones {
		if tombstone.QuarantineID == [32]byte{} ||
			tombstone.WalletID == [32]byte{} ||
			tombstone.LiftCertificateHash == [32]byte{} ||
			!tombstone.LiftedAt.valid() ||
			tombstone.ResolutionEvidenceHash == [32]byte{} ||
			tombstone.ResolutionFinality.BlockNumber == 0 ||
			tombstone.ResolutionFinality.BlockHash == [32]byte{} {
			return fmt.Errorf("FROST retained-group tombstone state is invalid")
		}
		if _, exists := tombstones[tombstone.QuarantineID]; exists {
			return fmt.Errorf("FROST retained-group tombstone state is duplicate")
		}
		quarantine, exists := quarantines[tombstone.QuarantineID]
		if !exists || quarantine.Status != frostRetainedGroupQuarantineLifted ||
			quarantine.RaisedRecord.WalletID != tombstone.WalletID ||
			quarantine.LiftCertificateHash != tombstone.LiftCertificateHash ||
			quarantine.LiftedAt != tombstone.LiftedAt {
			return fmt.Errorf("FROST retained-group tombstone does not match its lifted record")
		}
		tombstones[tombstone.QuarantineID] = tombstone
	}
	for quarantineID, quarantine := range quarantines {
		_, hasTombstone := tombstones[quarantineID]
		if (quarantine.Status == frostRetainedGroupQuarantineLifted) != hasTombstone {
			return fmt.Errorf("FROST retained-group lifted state and tombstones disagree")
		}
	}
	activeRoot, err := frostRetainedGroupQuarantineActiveRoot(
		state.BindingHash,
		quarantines,
	)
	if err != nil || activeRoot != state.ActiveRoot {
		return fmt.Errorf("FROST retained-group active quarantine root mismatch")
	}
	tombstoneRoot, err := frostRetainedGroupQuarantineTombstoneRoot(
		state.BindingHash,
		tombstones,
	)
	if err != nil || tombstoneRoot != state.TombstoneRoot {
		return fmt.Errorf("FROST retained-group quarantine tombstone root mismatch")
	}
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range mutations {
		if !isFrostRetainedGroupQuarantineMutation(mutation.Kind) {
			return fmt.Errorf("canonical inventory mutation cannot enter quarantine journal")
		}
		if !mutation.Point.valid() || (index > 0 &&
			compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("FROST retained-group quarantine mutations are not strictly ordered")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("FROST retained-group quarantine mutations disagree on canonical block hash")
		}
		previous = mutation.Point
		switch mutation.Kind {
		case FrostRetainedGroupQuarantineMutation, FrostRetainedGroupRecoveryRequiredMutation:
			if mutation.QuarantineID == [32]byte{} ||
				mutation.WalletID == [32]byte{} ||
				mutation.EvidenceHash == [32]byte{} ||
				strings.TrimSpace(mutation.Reason) == "" ||
				mutation.LiftCertificateHash != [32]byte{} ||
				mutation.LiftCertificate != nil ||
				!frostRetainedGroupQuarantineMutationInventoryFieldsEmpty(mutation) {
				return fmt.Errorf("invalid or duplicate FROST retained-group quarantine")
			}
			if _, exists := quarantines[mutation.QuarantineID]; exists {
				return fmt.Errorf("invalid or duplicate FROST retained-group quarantine")
			}
			if _, exists := tombstones[mutation.QuarantineID]; exists {
				return fmt.Errorf("FROST retained-group quarantine ID has a permanent tombstone")
			}
			quarantines[mutation.QuarantineID] = frostRetainedGroupQuarantineState{
				RaisedRecord: FrostRetainedGroupQuarantineRaisedRecord{
					QuarantineID:     mutation.QuarantineID,
					WalletID:         mutation.WalletID,
					EvidenceHash:     mutation.EvidenceHash,
					Reason:           mutation.Reason,
					RecoveryRequired: mutation.Kind == FrostRetainedGroupRecoveryRequiredMutation,
					RaisedAt:         mutation.Point,
				},
				Status: frostRetainedGroupQuarantineActive,
			}
		case FrostRetainedGroupQuarantineLiftMutation:
			quarantine, exists := quarantines[mutation.QuarantineID]
			if !exists || mutation.EvidenceHash != [32]byte{} ||
				mutation.Reason != "" ||
				mutation.LiftCertificateHash == [32]byte{} ||
				!frostRetainedGroupQuarantineMutationInventoryFieldsEmpty(mutation) {
				return fmt.Errorf("unknown or malformed FROST retained-group quarantine lift")
			}
			if _, exists := tombstones[mutation.QuarantineID]; exists {
				return fmt.Errorf("FROST retained-group quarantine lift was already tombstoned")
			}
			certificateHash, err := validateFrostRetainedGroupLiftCertificate(
				policy,
				*state,
				mutation,
				quarantine,
			)
			if err != nil {
				return err
			}
			body := mutation.LiftCertificate.Body
			quarantine.Status = frostRetainedGroupQuarantineLifted
			quarantine.LiftCertificateHash = certificateHash
			quarantine.LiftedAt = mutation.Point
			quarantines[mutation.QuarantineID] = quarantine
			tombstones[mutation.QuarantineID] =
				frostRetainedGroupQuarantineTombstone{
					QuarantineID:           mutation.QuarantineID,
					WalletID:               mutation.WalletID,
					LiftCertificateHash:    certificateHash,
					LiftedAt:               mutation.Point,
					ResolutionEvidenceHash: body.ResolutionEvidenceHash,
					ResolutionFinality:     body.ResolutionFinality,
				}
		}
		if err := appendFrostRetainedGroupQuarantineRoot(state, mutation); err != nil {
			return err
		}
		if err := setFrostRetainedGroupQuarantineCollections(
			state,
			quarantines,
			tombstones,
		); err != nil {
			return err
		}
	}
	return setFrostRetainedGroupQuarantineCollections(
		state,
		quarantines,
		tombstones,
	)
}

func frostRetainedGroupQuarantineMutationInventoryFieldsEmpty(
	mutation FrostRetainedGroupMutation,
) bool {
	return mutation.WalletPublicKeyHash == [20]byte{} &&
		len(mutation.OperatorIDs) == 0 &&
		mutation.RetainedGroupHash == [32]byte{} &&
		mutation.DkgResultHash == [32]byte{} &&
		mutation.DkgSubmissionPoint == (FrostRetainedGroupEventPoint{}) &&
		mutation.DkgApprovalPoint == (FrostRetainedGroupEventPoint{}) &&
		mutation.CreationPoint == (FrostRetainedGroupEventPoint{}) &&
		mutation.BridgeRegistrationPoint == (FrostRetainedGroupEventPoint{})
}

func setFrostRetainedGroupQuarantineCollections(
	state *frostRetainedGroupQuarantineJournalState,
	quarantines map[[32]byte]frostRetainedGroupQuarantineState,
	tombstones map[[32]byte]frostRetainedGroupQuarantineTombstone,
) error {
	state.Quarantines = make(
		[]frostRetainedGroupQuarantineState,
		0,
		len(quarantines),
	)
	for _, quarantine := range quarantines {
		state.Quarantines = append(state.Quarantines, quarantine)
	}
	sort.Slice(state.Quarantines, func(i, j int) bool {
		return bytes.Compare(
			state.Quarantines[i].RaisedRecord.QuarantineID[:],
			state.Quarantines[j].RaisedRecord.QuarantineID[:],
		) < 0
	})
	state.Tombstones = make(
		[]frostRetainedGroupQuarantineTombstone,
		0,
		len(tombstones),
	)
	for _, tombstone := range tombstones {
		state.Tombstones = append(state.Tombstones, tombstone)
	}
	sort.Slice(state.Tombstones, func(i, j int) bool {
		return bytes.Compare(
			state.Tombstones[i].QuarantineID[:],
			state.Tombstones[j].QuarantineID[:],
		) < 0
	})
	activeRoot, err := frostRetainedGroupQuarantineActiveRoot(
		state.BindingHash,
		quarantines,
	)
	if err != nil {
		return err
	}
	tombstoneRoot, err := frostRetainedGroupQuarantineTombstoneRoot(
		state.BindingHash,
		tombstones,
	)
	if err != nil {
		return err
	}
	state.ActiveRoot = activeRoot
	state.TombstoneRoot = tombstoneRoot
	return nil
}

func frostRetainedGroupQuarantineActiveRoot(
	bindingHash [32]byte,
	quarantines map[[32]byte]frostRetainedGroupQuarantineState,
) ([32]byte, error) {
	active := make([]frostRetainedGroupWireQuarantineRaisedRecord, 0)
	for _, quarantine := range quarantines {
		if quarantine.Status == frostRetainedGroupQuarantineActive {
			record := quarantine.RaisedRecord
			active = append(
				active,
				frostRetainedGroupWireQuarantineRaisedRecord{
					QuarantineID:     frostActivationHex32(record.QuarantineID),
					WalletID:         frostActivationHex32(record.WalletID),
					EvidenceHash:     frostActivationHex32(record.EvidenceHash),
					Reason:           record.Reason,
					RecoveryRequired: record.RecoveryRequired,
					RaisedAt: frostRetainedGroupEventPointToWire(
						record.RaisedAt,
					),
				},
			)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].QuarantineID < active[j].QuarantineID
	})
	return frostRetainedGroupCollectionRoot(
		frostRetainedGroupQuarantineActiveDomain,
		bindingHash,
		active,
	)
}

func frostRetainedGroupQuarantineTombstoneRoot(
	bindingHash [32]byte,
	tombstones map[[32]byte]frostRetainedGroupQuarantineTombstone,
) ([32]byte, error) {
	type wireTombstone struct {
		QuarantineID           string                           `json:"quarantineID"`
		WalletID               string                           `json:"walletID"`
		LiftCertificateHash    string                           `json:"liftCertificateHash"`
		LiftedAt               frostRetainedGroupWireEventPoint `json:"liftedAt"`
		ResolutionEvidenceHash string                           `json:"resolutionEvidenceHash"`
		ResolutionFinality     frostRetainedGroupWireFinality   `json:"resolutionFinality"`
	}
	ordered := make(
		[]wireTombstone,
		0,
		len(tombstones),
	)
	for _, tombstone := range tombstones {
		ordered = append(ordered, wireTombstone{
			QuarantineID:           frostActivationHex32(tombstone.QuarantineID),
			WalletID:               frostActivationHex32(tombstone.WalletID),
			LiftCertificateHash:    frostActivationHex32(tombstone.LiftCertificateHash),
			LiftedAt:               frostRetainedGroupEventPointToWire(tombstone.LiftedAt),
			ResolutionEvidenceHash: frostActivationHex32(tombstone.ResolutionEvidenceHash),
			ResolutionFinality: frostRetainedGroupFinalityToWire(
				tombstone.ResolutionFinality,
			),
		})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].QuarantineID < ordered[j].QuarantineID
	})
	return frostRetainedGroupCollectionRoot(
		frostRetainedGroupTombstoneRootDomain,
		bindingHash,
		ordered,
	)
}

func frostRetainedGroupCollectionRoot(
	domain string,
	bindingHash [32]byte,
	collection interface{},
) ([32]byte, error) {
	if bindingHash == [32]byte{} {
		return [32]byte{}, fmt.Errorf(
			"FROST retained-group collection root has an empty protocol binding",
		)
	}
	payload, err := frostRetainedGroupCanonicalValue(collection)
	if err != nil {
		return [32]byte{}, err
	}
	hasher := sha256.New()
	hasher.Write([]byte(domain))
	hasher.Write(bindingHash[:])
	hasher.Write(payload)
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result, nil
}

func frostRetainedGroupActiveQuarantineCount(
	state frostRetainedGroupQuarantineJournalState,
) uint64 {
	count := uint64(0)
	for _, quarantine := range state.Quarantines {
		if quarantine.Status == frostRetainedGroupQuarantineActive {
			count++
		}
	}
	return count
}

// frostRetainedGroupActiveQuarantines joins the independent quarantine journal
// with the canonical retained-group inventory so a signing caller can decide,
// from one authenticated snapshot, whether the exact wallet it is about to sign
// for is quarantined. Ordering follows the quarantine ID, matching the active
// quarantine root's canonical order.
func frostRetainedGroupActiveQuarantines(
	state frostRetainedGroupJournalState,
	quarantineState frostRetainedGroupQuarantineJournalState,
) []frostRetainedGroupActiveQuarantine {
	publicKeyHashes := make(map[[32]byte][20]byte, len(state.Wallets))
	for _, wallet := range state.Wallets {
		publicKeyHashes[wallet.WalletID] = wallet.WalletPublicKeyHash
	}
	active := make([]frostRetainedGroupActiveQuarantine, 0)
	for _, quarantine := range quarantineState.Quarantines {
		if quarantine.Status != frostRetainedGroupQuarantineActive {
			continue
		}
		active = append(active, frostRetainedGroupActiveQuarantine{
			QuarantineID:        quarantine.RaisedRecord.QuarantineID,
			WalletID:            quarantine.RaisedRecord.WalletID,
			WalletPublicKeyHash: publicKeyHashes[quarantine.RaisedRecord.WalletID],
			RecoveryRequired:    quarantine.RaisedRecord.RecoveryRequired,
		})
	}
	sort.Slice(active, func(i, j int) bool {
		return bytes.Compare(
			active[i].QuarantineID[:],
			active[j].QuarantineID[:],
		) < 0
	})
	return active
}

// activeQuarantineFor returns the active quarantine raised against the wallet
// identified by the given Bridge public-key hash, or nil when that wallet is
// not quarantined at the reconciled point this snapshot pins.
func (frgjs *frostRetainedGroupJournalSnapshot) activeQuarantineFor(
	walletPublicKeyHash [20]byte,
) *frostRetainedGroupActiveQuarantine {
	if frgjs == nil {
		return nil
	}
	for index, quarantine := range frgjs.ActiveQuarantines {
		if quarantine.WalletPublicKeyHash == walletPublicKeyHash {
			return &frgjs.ActiveQuarantines[index]
		}
	}
	return nil
}

func validFrostRetainedGroupTransition(
	current FrostRetainedGroupLifecycle,
	next FrostRetainedGroupLifecycle,
) bool {
	switch current {
	case FrostRetainedGroupLive:
		return next == FrostRetainedGroupMovingFunds || next == FrostRetainedGroupClosing ||
			next == FrostRetainedGroupTerminated
	case FrostRetainedGroupMovingFunds:
		return next == FrostRetainedGroupClosing || next == FrostRetainedGroupTerminated
	case FrostRetainedGroupClosing:
		return next == FrostRetainedGroupClosed || next == FrostRetainedGroupTerminated
	default:
		return false
	}
}

func sameFrostRetainedGroupTransaction(
	left FrostRetainedGroupEventPoint,
	right FrostRetainedGroupEventPoint,
) bool {
	return left.BlockNumber == right.BlockNumber &&
		left.BlockHash == right.BlockHash &&
		left.TransactionHash == right.TransactionHash &&
		left.TransactionIndex == right.TransactionIndex
}

func appendFrostRetainedGroupQuarantineRoot(
	state *frostRetainedGroupQuarantineJournalState,
	mutation FrostRetainedGroupMutation,
) error {
	wireMutation := frostRetainedGroupWireQuarantineJournalMutation{
		Point:               frostRetainedGroupEventPointToWire(mutation.Point),
		Kind:                string(mutation.Kind),
		WalletID:            frostActivationHex32(mutation.WalletID),
		QuarantineID:        frostActivationHex32(mutation.QuarantineID),
		EvidenceHash:        frostActivationHex32(mutation.EvidenceHash),
		LiftCertificateHash: frostActivationHex32(mutation.LiftCertificateHash),
		Reason:              mutation.Reason,
	}
	payload, err := frostRetainedGroupCanonicalValue(wireMutation)
	if err != nil {
		return err
	}
	leaf := sha256.Sum256(payload)
	hasher := sha256.New()
	hasher.Write([]byte(frostRetainedGroupQuarantineDomain))
	hasher.Write(state.Root[:])
	hasher.Write(leaf[:])
	copy(state.Root[:], hasher.Sum(nil))
	state.Generation++
	return nil
}

func frostRetainedGroupInventoryRoot(
	state frostRetainedGroupJournalState,
) ([32]byte, uint64, uint64, uint64, error) {
	if len(state.Wallets) > frostRetainedGroupMaximumWallets {
		return [32]byte{}, 0, 0, 0, fmt.Errorf(
			"FROST retained-group inventory exceeds the wallet limit",
		)
	}
	type inventoryEventPoint struct {
		BlockNumber      uint64 `json:"blockNumber"`
		BlockHash        string `json:"blockHash"`
		TransactionHash  string `json:"transactionHash"`
		TransactionIndex uint32 `json:"transactionIndex"`
		LogIndex         uint32 `json:"logIndex"`
	}
	type inventoryEntry struct {
		WalletID                string               `json:"walletID"`
		RetainedGroupHash       string               `json:"retainedGroupHash"`
		ActualGroupSize         uint64               `json:"actualGroupSize"`
		Lifecycle               string               `json:"lifecycle"`
		CreationPoint           inventoryEventPoint  `json:"creationPoint"`
		BridgeRegistrationPoint inventoryEventPoint  `json:"bridgeRegistrationPoint"`
		LifecyclePoint          inventoryEventPoint  `json:"lifecyclePoint"`
		RegistryClosurePoint    *inventoryEventPoint `json:"registryClosurePoint,omitempty"`
	}
	eventPoint := func(point FrostRetainedGroupEventPoint) inventoryEventPoint {
		return inventoryEventPoint{
			BlockNumber:      point.BlockNumber,
			BlockHash:        "0x" + hex.EncodeToString(point.BlockHash[:]),
			TransactionHash:  "0x" + hex.EncodeToString(point.TransactionHash[:]),
			TransactionIndex: point.TransactionIndex,
			LogIndex:         point.LogIndex,
		}
	}
	entries := make([]inventoryEntry, 0)
	minimumSize := uint64(0)
	maximumSize := uint64(0)
	for _, wallet := range state.Wallets {
		size := uint64(len(wallet.OperatorIDs))
		if wallet.WalletID == [32]byte{} || size < 51 || size > 100 ||
			wallet.RetainedGroupHash == [32]byte{} ||
			!wallet.CreationPoint.valid() || !wallet.BridgeRegistrationPoint.valid() ||
			!wallet.LifecyclePoint.valid() ||
			!sameFrostRetainedGroupTransaction(wallet.CreationPoint, wallet.BridgeRegistrationPoint) ||
			compareFrostRetainedGroupEventPoints(wallet.CreationPoint, wallet.BridgeRegistrationPoint) >= 0 ||
			compareFrostRetainedGroupEventPoints(wallet.BridgeRegistrationPoint, wallet.LifecyclePoint) > 0 ||
			(wallet.BridgeRegistrationPoint.BlockNumber == wallet.LifecyclePoint.BlockNumber &&
				wallet.BridgeRegistrationPoint.BlockHash != wallet.LifecyclePoint.BlockHash) ||
			wallet.LifecyclePoint.BlockNumber > state.CurrentPoint.BlockNumber ||
			(wallet.LifecyclePoint.BlockNumber == state.CurrentPoint.BlockNumber &&
				wallet.LifecyclePoint.BlockHash != state.CurrentPoint.BlockHash) ||
			wallet.Lifecycle.terminal() != wallet.RegistryClosed ||
			(wallet.RegistryClosed &&
				(!sameFrostRetainedGroupTransaction(wallet.LifecyclePoint, wallet.RegistryClosurePoint) ||
					compareFrostRetainedGroupEventPoints(wallet.LifecyclePoint, wallet.RegistryClosurePoint) >= 0)) {
			return [32]byte{}, 0, 0, 0, fmt.Errorf("FROST retained-group inventory has invalid group size")
		}
		if minimumSize == 0 || size < minimumSize {
			minimumSize = size
		}
		if size > maximumSize {
			maximumSize = size
		}
		lifecycle := ""
		switch wallet.Lifecycle {
		case FrostRetainedGroupLive:
			lifecycle = "live"
		case FrostRetainedGroupMovingFunds:
			lifecycle = "moving-funds"
		case FrostRetainedGroupClosing:
			lifecycle = "closing"
		case FrostRetainedGroupClosed:
			lifecycle = "closed"
		case FrostRetainedGroupTerminated:
			lifecycle = "terminated"
		default:
			return [32]byte{}, 0, 0, 0, fmt.Errorf("FROST retained-group inventory has unknown lifecycle")
		}
		entry := inventoryEntry{
			WalletID:                "0x" + hex.EncodeToString(wallet.WalletID[:]),
			RetainedGroupHash:       "0x" + hex.EncodeToString(wallet.RetainedGroupHash[:]),
			ActualGroupSize:         size,
			Lifecycle:               lifecycle,
			CreationPoint:           eventPoint(wallet.CreationPoint),
			BridgeRegistrationPoint: eventPoint(wallet.BridgeRegistrationPoint),
			LifecyclePoint:          eventPoint(wallet.LifecyclePoint),
		}
		if wallet.RegistryClosed {
			registryPoint := eventPoint(wallet.RegistryClosurePoint)
			entry.RegistryClosurePoint = &registryPoint
		}
		entries = append(entries, entry)
	}
	// ASCII ordering of lowercase fixed-width wallet IDs is identical to byte
	// ordering and is stated explicitly here to make the wire commitment clear.
	sort.Slice(entries, func(i, j int) bool { return entries[i].WalletID < entries[j].WalletID })
	for index := 1; index < len(entries); index++ {
		if entries[index-1].WalletID == entries[index].WalletID {
			return [32]byte{}, 0, 0, 0, fmt.Errorf("FROST retained-group inventory has ambiguous wallet membership")
		}
	}
	accumulator := sha256.Sum256([]byte(frostRetainedGroupInventoryEntriesDomain))
	for _, entry := range entries {
		canonical, err := frostRetainedGroupCanonicalValue(entry)
		if err != nil {
			return [32]byte{}, 0, 0, 0, err
		}
		leafHasher := sha256.New()
		leafHasher.Write([]byte(frostRetainedGroupInventoryLeafDomain))
		leafHasher.Write(canonical)
		leaf := leafHasher.Sum(nil)
		nodeHasher := sha256.New()
		nodeHasher.Write([]byte(frostRetainedGroupInventoryNodeDomain))
		nodeHasher.Write(accumulator[:])
		nodeHasher.Write(leaf)
		copy(accumulator[:], nodeHasher.Sum(nil))
	}
	rootMetadata := struct {
		Point struct {
			BlockHash   string `json:"blockHash"`
			BlockNumber uint64 `json:"blockNumber"`
		} `json:"point"`
		SnapshotGeneration uint64 `json:"snapshotGeneration"`
		WalletCount        uint64 `json:"walletCount"`
	}{SnapshotGeneration: state.SnapshotGeneration, WalletCount: uint64(len(entries))}
	rootMetadata.Point.BlockNumber = state.CurrentPoint.BlockNumber
	rootMetadata.Point.BlockHash = "0x" + hex.EncodeToString(state.CurrentPoint.BlockHash[:])
	canonicalMetadata, err := frostRetainedGroupCanonicalValue(rootMetadata)
	if err != nil {
		return [32]byte{}, 0, 0, 0, err
	}
	rootHasher := sha256.New()
	rootHasher.Write([]byte(frostRetainedGroupInventoryRootDomain))
	rootHasher.Write(canonicalMetadata)
	rootHasher.Write(accumulator[:])
	var root [32]byte
	copy(root[:], rootHasher.Sum(nil))
	return root, uint64(len(entries)), minimumSize, maximumSize, nil
}

func frostRetainedGroupCanonicalValue(value interface{}) ([]byte, error) {
	return canonicalFrostActivationValue(value)
}

func cloneFrostRetainedGroupMutations(
	mutations []FrostRetainedGroupMutation,
) []FrostRetainedGroupMutation {
	result := make([]FrostRetainedGroupMutation, len(mutations))
	copy(result, mutations)
	for index := range result {
		result[index].OperatorIDs = append([]uint32{}, mutations[index].OperatorIDs...)
		if mutations[index].LiftCertificate != nil {
			certificate := *mutations[index].LiftCertificate
			certificate.Signatures = append(
				[]FrostRetainedGroupQuarantineLiftSignature{},
				mutations[index].LiftCertificate.Signatures...,
			)
			result[index].LiftCertificate = &certificate
		}
	}
	return result
}

func equalFrostRetainedGroupSemanticHistories(
	first *FrostRetainedGroupHistory,
	second *FrostRetainedGroupHistory,
) (bool, error) {
	if first == nil || second == nil ||
		first.From != second.From ||
		first.To != second.To ||
		first.HistoryRoot != second.HistoryRoot ||
		first.Complete != second.Complete ||
		first.EmptyAtFrom != second.EmptyAtFrom ||
		first.DescriptorSetHash != second.DescriptorSetHash ||
		len(first.Mutations) != len(second.Mutations) {
		return false, nil
	}
	for index := range first.Mutations {
		firstMutation, err := frostRetainedGroupCanonicalValue(
			first.Mutations[index],
		)
		if err != nil {
			return false, err
		}
		secondMutation, err := frostRetainedGroupCanonicalValue(
			second.Mutations[index],
		)
		if err != nil {
			return false, err
		}
		if !bytes.Equal(firstMutation, secondMutation) {
			return false, nil
		}
	}
	return true, nil
}

// adoptDurableBatchSuffixes integrates batches that are already durable but not
// yet reflected by their state checkpoint. Batch publication and the state
// checkpoint are two separate writes, so a failure between them - a full disk,
// an I/O error, or the persistence hooks used by the crash tests - leaves
// exactly that shape. Batch files are immutable no-replace publications, so
// without adoption every later reconciliation recomputes the same sequence,
// fails with "immutable journal file already exists", and the in-process
// journal stays wedged until a restart replays it. initialize() already
// performs this adoption on startup; doing it here keeps a running process
// consistent with its own restart semantics.
//
// Adoption is not a trust shortcut. The batch is revalidated against the exact
// durable predecessor state, and the reconciliation that follows re-derives the
// canonical history and rejects the whole durable prefix if the source rewrote,
// omitted, or reordered any adopted event.
func (frgj *frostRetainedGroupJournal) adoptDurableBatchSuffixes() error {
	if err := frgj.adoptDurableCanonicalBatches(); err != nil {
		return err
	}
	return frgj.adoptDurableQuarantineBatches()
}

func (frgj *frostRetainedGroupJournal) adoptDurableCanonicalBatches() error {
	for {
		name := frostRetainedGroupBatchFileName(frgj.state.BatchSequence + 1)
		batch := frostRetainedGroupJournalBatch{}
		if err := frgj.readEnvelope(name, &batch); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf(
				"cannot read durable FROST retained-group journal batch [%s]: [%w]",
				name,
				err,
			)
		}
		if err := validateFrostRetainedGroupBatch(batch, frgj.state); err != nil {
			return fmt.Errorf(
				"invalid durable FROST retained-group journal batch [%s]: [%w]",
				name,
				err,
			)
		}
		candidate := cloneFrostRetainedGroupState(frgj.state)
		if err := applyFrostRetainedGroupMutations(
			&candidate,
			batch.Mutations,
		); err != nil {
			return fmt.Errorf(
				"cannot replay durable FROST retained-group journal batch [%s]: [%w]",
				name,
				err,
			)
		}
		candidate.BatchSequence = batch.Sequence
		candidate.CurrentPoint = batch.To
		candidate.BatchRoot = frostRetainedGroupBatchRoot(
			batch.PriorBatchRoot,
			batch.Checksum,
		)
		inventoryRoot, _, _, _, err := frostRetainedGroupInventoryRoot(candidate)
		if err != nil {
			return err
		}
		candidate.InventoryRoot = inventoryRoot
		if err := frgj.persistEnvelope(
			frostRetainedGroupJournalStateFile,
			&candidate,
			true,
		); err != nil {
			return fmt.Errorf(
				"cannot integrate durable FROST retained-group journal batch [%s]: [%w]",
				name,
				err,
			)
		}
		frgj.state = candidate
		frgj.mutations = append(
			frgj.mutations,
			cloneFrostRetainedGroupMutations(batch.Mutations)...,
		)
	}
}

func (frgj *frostRetainedGroupJournal) adoptDurableQuarantineBatches() error {
	for {
		name := frostRetainedGroupBatchFileName(
			frgj.quarantineState.BatchSequence + 1,
		)
		wireBatch := frostRetainedGroupWireQuarantineJournalBatch{}
		if err := readFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			name,
			&wireBatch,
		); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf(
				"cannot read durable FROST retained-group quarantine batch [%s]: [%w]",
				name,
				err,
			)
		}
		batch, err := frostRetainedGroupQuarantineBatchFromWire(
			wireBatch,
			frgj.liftCertificates,
		)
		if err != nil {
			return fmt.Errorf(
				"cannot decode durable FROST retained-group quarantine batch [%s]: [%w]",
				name,
				err,
			)
		}
		if err := validateFrostRetainedGroupQuarantineBatch(
			batch,
			frgj.quarantineState,
		); err != nil {
			return fmt.Errorf(
				"invalid durable FROST retained-group quarantine batch [%s]: [%w]",
				name,
				err,
			)
		}
		if err := frgj.validatePersistedLiftCertificates(
			batch.Mutations,
		); err != nil {
			return fmt.Errorf(
				"invalid persisted FROST quarantine lift certificate in durable batch [%s]: [%w]",
				name,
				err,
			)
		}
		candidate := cloneFrostRetainedGroupQuarantineState(frgj.quarantineState)
		if err := applyFrostRetainedGroupQuarantineMutations(
			&candidate,
			batch.Mutations,
			frgj.liftPolicy,
		); err != nil {
			return fmt.Errorf(
				"cannot replay durable FROST retained-group quarantine batch [%s]: [%w]",
				name,
				err,
			)
		}
		candidate.BatchSequence = batch.Sequence
		candidate.CurrentPoint = batch.To
		candidate.BatchRoot = frostRetainedGroupQuarantineBatchRoot(
			batch.PriorBatchRoot,
			batch.Checksum,
		)
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalStateFile,
			&candidate,
			true,
		); err != nil {
			return fmt.Errorf(
				"cannot integrate durable FROST retained-group quarantine batch [%s]: [%w]",
				name,
				err,
			)
		}
		frgj.quarantineState = candidate
		frgj.quarantineMutations = append(
			frgj.quarantineMutations,
			cloneFrostRetainedGroupMutations(batch.Mutations)...,
		)
	}
}

func (frgj *frostRetainedGroupJournal) reconcile(
	ctx context.Context,
	target FrostPreSignFinality,
) (*frostRetainedGroupJournalSnapshot, error) {
	if ctx == nil {
		return nil, fmt.Errorf("FROST retained-group reconciliation context is nil")
	}
	reconciliationContext, cancel := context.WithTimeout(
		ctx,
		frostRetainedGroupMaximumReconciliationDuration,
	)
	defer cancel()
	ctx = reconciliationContext
	frgj.mutex.Lock()
	defer frgj.mutex.Unlock()
	if frgj.closed {
		return nil, fmt.Errorf("FROST retained-group journal is closed")
	}
	if err := frgj.adoptDurableBatchSuffixes(); err != nil {
		return nil, err
	}
	if target.BlockNumber == 0 || target.BlockHash == [32]byte{} ||
		target.BlockNumber < frgj.metadata.Checkpoint.BlockNumber ||
		target.BlockNumber < frgj.state.CurrentPoint.BlockNumber ||
		target.BlockNumber < frgj.quarantineState.CurrentPoint.BlockNumber ||
		(frgj.checkpointState.Sequence >=
			frgj.checkpointPolicy.MinimumSequence &&
			target.BlockNumber <
				frgj.checkpointState.Point.BlockNumber) {
		return nil, fmt.Errorf("FROST retained-group target is invalid or retrograde")
	}
	if err := frgj.verifyHistorySourceIdentity(ctx); err != nil {
		return nil, err
	}
	beforeHead, err := frgj.source.FinalizedHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read independent finalized head before journal replay: [%w]", err)
	}
	if beforeHead.BlockNumber < target.BlockNumber || beforeHead.BlockHash == [32]byte{} {
		return nil, fmt.Errorf("FROST retained-group target is above independent finalized head")
	}
	if beforeHead.BlockNumber == target.BlockNumber && beforeHead.BlockHash != target.BlockHash {
		return nil, fmt.Errorf("independent finalized head disagrees with challenged target")
	}
	if err := frgj.source.VerifyPoint(ctx, beforeHead); err != nil {
		return nil, fmt.Errorf("cannot verify independent finalized head before journal replay: [%w]", err)
	}
	for name, point := range map[string]FrostPreSignFinality{
		"signed checkpoint":         frgj.metadata.Checkpoint,
		"durable canonical cursor":  frgj.state.CurrentPoint,
		"durable quarantine cursor": frgj.quarantineState.CurrentPoint,
		"challenged target":         target,
	} {
		if err := frgj.source.VerifyPoint(ctx, point); err != nil {
			return nil, fmt.Errorf("cannot verify FROST retained-group %s: [%w]", name, err)
		}
	}
	checkpointAfter := FrostRetainedGroupCheckpointCursor{
		Sequence:        frgj.checkpointState.Sequence,
		CertificateHash: frgj.checkpointState.CertificateHash,
	}
	checkpointCursor := checkpointAfter
	var history *FrostRetainedGroupHistory
	checkpointHashes := make([][32]byte, 0)
	checkpointCertificates := make(
		[]FrostRetainedGroupCheckpointCertificate,
		0,
	)
	checkpointRecoveryComplete := false
	for checkpointPage := 0; checkpointPage <
		frostRetainedGroupCheckpointPagesPerReconciliation; checkpointPage++ {
		page, err := frgj.source.ReadCompleteHistory(
			ctx,
			frgj.metadata.Checkpoint,
			target,
			checkpointCursor,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot reconstruct complete FROST retained-group history: [%w]",
				err,
			)
		}
		if page == nil || !page.Complete || !page.EmptyAtFrom ||
			page.From != frgj.metadata.Checkpoint ||
			page.To != target ||
			page.DescriptorSetHash != frgj.metadata.DescriptorSetHash {
			return nil, fmt.Errorf(
				"FROST retained-group history receipt is incomplete or differently bound",
			)
		}
		if len(page.Mutations) > frostRetainedGroupMaximumMutations {
			return nil, fmt.Errorf(
				"FROST retained-group history exceeds the mutation limit",
			)
		}
		if len(page.Checkpoints) >
			frostRetainedGroupMaximumCheckpointsPerPage {
			return nil, fmt.Errorf(
				"FROST retained-group checkpoint page exceeds its bound",
			)
		}
		if err := validateCompleteFrostRetainedGroupHistory(
			page,
			frgj.liftPolicy,
		); err != nil {
			return nil, err
		}
		if page.CheckpointAfter != checkpointCursor {
			return nil, fmt.Errorf(
				"FROST retained-group history checkpoint cursor differs from the requested certified head",
			)
		}
		if history == nil {
			history = page
		} else {
			equal, err := equalFrostRetainedGroupSemanticHistories(
				history,
				page,
			)
			if err != nil {
				return nil, err
			}
			if !equal {
				return nil, fmt.Errorf(
					"FROST retained-group history changed between checkpoint pages",
				)
			}
		}
		pageCheckpointHashes, err :=
			validateFrostRetainedGroupCheckpointSuffix(
				frgj.checkpointPolicy,
				checkpointCursor,
				page.Checkpoints,
			)
		if err != nil {
			return nil, err
		}
		if len(page.Checkpoints) == 0 && !page.CheckpointComplete {
			return nil, fmt.Errorf(
				"nonfinal FROST checkpoint page made no progress",
			)
		}
		if len(page.Checkpoints) > 0 {
			if err := validateFrostRetainedGroupCheckpointSemantics(
				frgj.checkpointPolicy,
				page,
				pageCheckpointHashes,
			); err != nil {
				return nil, err
			}
			for index, certificate := range page.Checkpoints {
				if err := frgj.source.VerifyPoint(
					ctx,
					certificate.Body.Point,
				); err != nil {
					return nil, fmt.Errorf(
						"cannot verify FROST checkpoint certificate point [%d:%d]: [%w]",
						checkpointPage,
						index,
						err,
					)
				}
			}
			checkpointCertificates = append(
				checkpointCertificates,
				page.Checkpoints...,
			)
			checkpointHashes = append(
				checkpointHashes,
				pageCheckpointHashes...,
			)
			tail := len(page.Checkpoints) - 1
			checkpointCursor = FrostRetainedGroupCheckpointCursor{
				Sequence:        page.Checkpoints[tail].Body.Sequence,
				CertificateHash: pageCheckpointHashes[tail],
			}
		}
		if page.CheckpointComplete {
			checkpointRecoveryComplete = true
			break
		}
	}
	if history == nil {
		return nil, fmt.Errorf(
			"FROST retained-group history source returned no checkpoint page",
		)
	}
	history.CheckpointAfter = checkpointAfter
	history.Checkpoints = checkpointCertificates
	history.CheckpointChainRoot =
		frostRetainedGroupCheckpointChainRoot(
			frgj.checkpointPolicy.ProtocolBindingHash,
			checkpointAfter,
			checkpointHashes,
		)
	history.CheckpointTipHash = checkpointAfter.CertificateHash
	if len(checkpointHashes) > 0 {
		history.CheckpointTipHash =
			checkpointHashes[len(checkpointHashes)-1]
	}
	history.CheckpointComplete = checkpointRecoveryComplete
	if len(checkpointCertificates) > 0 {
		// Revalidate the cross-page predecessor chain and the exact aggregate
		// semantic binding. Per-page bounds remain enforced above.
		checkpointHashes, err =
			validateFrostRetainedGroupCheckpointSuffix(
				frgj.checkpointPolicy,
				checkpointAfter,
				checkpointCertificates,
			)
		if err != nil {
			return nil, err
		}
		if err := validateFrostRetainedGroupCheckpointSemantics(
			frgj.checkpointPolicy,
			history,
			checkpointHashes,
		); err != nil {
			return nil, err
		}
	}
	if len(history.Checkpoints) == 0 {
		if !history.CheckpointComplete ||
			frgj.checkpointState.Sequence <
				frgj.checkpointPolicy.MinimumSequence ||
			frgj.checkpointState.Point != target ||
			frgj.checkpointState.HistoryRoot != history.HistoryRoot ||
			history.CheckpointTipHash !=
				frgj.checkpointState.CertificateHash ||
			history.CheckpointChainRoot !=
				frostRetainedGroupCheckpointChainRoot(
					frgj.checkpointPolicy.ProtocolBindingHash,
					checkpointAfter,
					nil,
				) {
			return nil, fmt.Errorf(
				"canonical FROST retained-group history rewrote, omitted, or reordered the durable certified head",
			)
		}
	} else {
		if frgj.checkpointState.Sequence >=
			frgj.checkpointPolicy.MinimumSequence &&
			(history.Checkpoints[0].Body.Point.BlockNumber <=
				frgj.checkpointState.Point.BlockNumber ||
				history.Checkpoints[0].Body.CanonicalGeneration <
					frgj.checkpointState.CanonicalGeneration ||
				history.Checkpoints[0].Body.QuarantineGeneration <
					frgj.checkpointState.QuarantineGeneration) {
			return nil, fmt.Errorf(
				"FROST checkpoint suffix does not monotonically advance the durable head",
			)
		}
	}
	canonicalInventoryMutations := frostRetainedGroupCanonicalMutations(history.Mutations)
	if len(frgj.mutations) > len(canonicalInventoryMutations) {
		return nil, fmt.Errorf("durable FROST retained-group journal has events absent from canonical history")
	}
	for index := range frgj.mutations {
		durable, err := frostRetainedGroupCanonicalValue(frgj.mutations[index])
		if err != nil {
			return nil, err
		}
		canonical, err := frostRetainedGroupCanonicalValue(canonicalInventoryMutations[index])
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(durable, canonical) {
			return nil, fmt.Errorf("canonical FROST retained-group history rewrote, omitted, or reordered event [%d]", index)
		}
	}
	canonicalQuarantineMutations := frostRetainedGroupQuarantineMutations(history.Mutations)
	if len(frgj.quarantineMutations) > len(canonicalQuarantineMutations) {
		return nil, fmt.Errorf("durable FROST quarantine journal has events absent from canonical history")
	}
	for index := range frgj.quarantineMutations {
		durable, err := frostRetainedGroupCanonicalValue(frgj.quarantineMutations[index])
		if err != nil {
			return nil, err
		}
		canonical, err := frostRetainedGroupCanonicalValue(canonicalQuarantineMutations[index])
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(durable, canonical) {
			return nil, fmt.Errorf("canonical FROST retained-group history rewrote, omitted, or reordered quarantine event [%d]", index)
		}
	}
	suffix := cloneFrostRetainedGroupMutations(
		canonicalInventoryMutations[len(frgj.mutations):],
	)
	for _, mutation := range suffix {
		if mutation.Point.BlockNumber <= frgj.state.CurrentPoint.BlockNumber ||
			mutation.Point.BlockNumber > target.BlockNumber {
			return nil, fmt.Errorf("canonical FROST retained-group suffix is outside durable cursor bounds")
		}
	}
	if target != frgj.state.CurrentPoint || len(suffix) != 0 {
		candidate := cloneFrostRetainedGroupState(frgj.state)
		if err := applyFrostRetainedGroupMutations(&candidate, suffix); err != nil {
			return nil, err
		}
		batch := frostRetainedGroupJournalBatch{
			Schema:         frostRetainedGroupJournalBatchSchema,
			BindingHash:    frgj.metadata.BindingHash,
			Sequence:       frgj.state.BatchSequence + 1,
			From:           frgj.state.CurrentPoint,
			To:             target,
			PriorBatchRoot: frgj.state.BatchRoot,
			Mutations:      suffix,
		}
		checksumPayload, err := frostRetainedGroupCanonicalValue(batch)
		if err != nil {
			return nil, err
		}
		batch.Checksum = sha256.Sum256(checksumPayload)
		candidate.BatchSequence = batch.Sequence
		candidate.CurrentPoint = target
		candidate.BatchRoot = frostRetainedGroupBatchRoot(batch.PriorBatchRoot, batch.Checksum)
		candidate.InventoryRoot, _, _, _, err = frostRetainedGroupInventoryRoot(candidate)
		if err != nil {
			return nil, err
		}
		if err := frgj.persistEnvelope(frostRetainedGroupBatchFileName(batch.Sequence), &batch, false); err != nil {
			return nil, fmt.Errorf("cannot append FROST retained-group journal batch: [%w]", err)
		}
		if frgj.persistFailureHook != nil {
			if err := frgj.persistFailureHook("after-batch-before-state"); err != nil {
				return nil, err
			}
		}
		if err := frgj.persistEnvelope(frostRetainedGroupJournalStateFile, &candidate, true); err != nil {
			return nil, fmt.Errorf("cannot checkpoint FROST retained-group journal state: [%w]", err)
		}
		frgj.state = candidate
		frgj.mutations = append(frgj.mutations, suffix...)
	}
	if frgj.checkpointPersistFailureHook != nil {
		if err := frgj.checkpointPersistFailureHook(
			"after-canonical-before-quarantine",
		); err != nil {
			return nil, err
		}
	}
	quarantineSuffix := cloneFrostRetainedGroupMutations(
		canonicalQuarantineMutations[len(frgj.quarantineMutations):],
	)
	for _, mutation := range quarantineSuffix {
		if mutation.Point.BlockNumber <= frgj.quarantineState.CurrentPoint.BlockNumber ||
			mutation.Point.BlockNumber > target.BlockNumber {
			return nil, fmt.Errorf("canonical FROST quarantine suffix is outside durable cursor bounds")
		}
	}
	if target != frgj.quarantineState.CurrentPoint || len(quarantineSuffix) != 0 {
		candidate := cloneFrostRetainedGroupQuarantineState(frgj.quarantineState)
		if err := applyFrostRetainedGroupQuarantineMutations(
			&candidate,
			quarantineSuffix,
			frgj.liftPolicy,
		); err != nil {
			return nil, err
		}
		hasLift := false
		for _, mutation := range quarantineSuffix {
			if mutation.Kind == FrostRetainedGroupQuarantineLiftMutation {
				hasLift = true
				break
			}
		}
		if hasLift {
			if err := frgj.ensureLiftCertificatesPersisted(quarantineSuffix); err != nil {
				return nil, err
			}
			if frgj.persistFailureHook != nil {
				if err := frgj.persistFailureHook(
					"after-quarantine-lift-certificate-before-batch",
				); err != nil {
					return nil, err
				}
			}
		}
		batch := frostRetainedGroupQuarantineJournalBatch{
			Schema:         frostRetainedGroupQuarantineBatchSchema,
			BindingHash:    frgj.quarantineMetadata.BindingHash,
			Sequence:       frgj.quarantineState.BatchSequence + 1,
			From:           frgj.quarantineState.CurrentPoint,
			To:             target,
			PriorBatchRoot: frgj.quarantineState.BatchRoot,
			Mutations:      quarantineSuffix,
		}
		checksumPayload, err := frostRetainedGroupQuarantineBatchCanonicalValue(
			batch,
		)
		if err != nil {
			return nil, err
		}
		batch.Checksum = sha256.Sum256(checksumPayload)
		candidate.BatchSequence = batch.Sequence
		candidate.CurrentPoint = target
		candidate.BatchRoot = frostRetainedGroupQuarantineBatchRoot(
			batch.PriorBatchRoot,
			batch.Checksum,
		)
		wireBatch, err := frostRetainedGroupQuarantineBatchToWire(batch)
		if err != nil {
			return nil, err
		}
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupBatchFileName(batch.Sequence),
			&wireBatch,
			false,
		); err != nil {
			return nil, fmt.Errorf("cannot append FROST retained-group quarantine batch: [%w]", err)
		}
		if frgj.persistFailureHook != nil {
			if err := frgj.persistFailureHook("after-quarantine-batch-before-state"); err != nil {
				return nil, err
			}
		}
		if err := persistFrostRetainedGroupEnvelopeAt(
			frgj.quarantineDirectory,
			frostRetainedGroupJournalStateFile,
			&candidate,
			true,
		); err != nil {
			return nil, fmt.Errorf("cannot checkpoint FROST retained-group quarantine state: [%w]", err)
		}
		frgj.quarantineState = candidate
		frgj.quarantineMutations = append(frgj.quarantineMutations, quarantineSuffix...)
	}
	if frgj.checkpointPersistFailureHook != nil {
		if err := frgj.checkpointPersistFailureHook(
			"after-semantic-journals-before-checkpoints",
		); err != nil {
			return nil, err
		}
	}
	if len(history.Checkpoints) > 0 {
		if err := frgj.persistCheckpointSuffix(
			history.Checkpoints,
			checkpointHashes,
		); err != nil {
			return nil, err
		}
	} else if err := frgj.validateCheckpointAgainstDurableState(
		frgj.checkpointState,
	); err != nil {
		return nil, err
	}
	checkpointRecoveryAdvanced :=
		!history.CheckpointComplete &&
			frgj.checkpointState.Sequence > checkpointAfter.Sequence
	withCheckpointRecoveryProgress := func(cause error) error {
		if !checkpointRecoveryAdvanced {
			return cause
		}
		return frostRetainedGroupCheckpointRecoveryProgressError(
			frgj.checkpointState.Sequence,
			cause,
		)
	}
	afterHead, err := frgj.source.FinalizedHead(ctx)
	if err != nil {
		return nil, withCheckpointRecoveryProgress(fmt.Errorf(
			"cannot read independent finalized head after journal replay: [%w]",
			err,
		))
	}
	if afterHead.BlockNumber < beforeHead.BlockNumber ||
		afterHead.BlockNumber < target.BlockNumber || afterHead.BlockHash == [32]byte{} ||
		(beforeHead.BlockNumber == afterHead.BlockNumber && beforeHead.BlockHash != afterHead.BlockHash) {
		return nil, withCheckpointRecoveryProgress(fmt.Errorf(
			"independent finalized head changed inconsistently during journal replay",
		))
	}
	if afterHead.BlockNumber == target.BlockNumber && afterHead.BlockHash != target.BlockHash {
		return nil, withCheckpointRecoveryProgress(fmt.Errorf(
			"independent finalized head disagrees with challenged target after replay",
		))
	}
	if err := frgj.source.VerifyPoint(ctx, afterHead); err != nil {
		return nil, withCheckpointRecoveryProgress(fmt.Errorf(
			"cannot verify independent finalized head after journal replay: [%w]",
			err,
		))
	}
	if err := frgj.source.VerifyPoint(ctx, target); err != nil {
		return nil, withCheckpointRecoveryProgress(fmt.Errorf(
			"challenged FROST retained-group point changed during replay: [%w]",
			err,
		))
	}
	if !history.CheckpointComplete {
		if !checkpointRecoveryAdvanced {
			return nil, fmt.Errorf(
				"incomplete FROST checkpoint recovery did not advance the durable head",
			)
		}
		return nil, withCheckpointRecoveryProgress(nil)
	}
	if frgj.orphanedDKGReconciler != nil {
		canonicalWallets := make(map[[32]byte]struct{}, len(frgj.state.Wallets))
		for _, wallet := range frgj.state.Wallets {
			canonicalWallets[wallet.WalletID] = struct{}{}
		}
		if err := frgj.orphanedDKGReconciler(
			ctx,
			target,
			canonicalWallets,
		); err != nil {
			return nil, fmt.Errorf(
				"cannot retire orphaned native FROST DKG material: [%w]",
				err,
			)
		}
	}
	localSessionCount, err := frgj.reconcileLocalSessions(ctx, target)
	if err != nil {
		return nil, err
	}
	root, walletCount, minimumSize, maximumSize, err := frostRetainedGroupInventoryRoot(frgj.state)
	if err != nil {
		return nil, err
	}
	if root != frgj.state.InventoryRoot || frgj.state.SnapshotGeneration < frgj.minimumGeneration {
		return nil, fmt.Errorf("FROST retained-group journal generation or inventory root is not activation-ready")
	}
	// An active quarantine is deliberately not an activation-ready failure here.
	// The node-wide "no active quarantine" requirement belongs to the activation
	// handshake, which refuses to bootstrap or attest health while any
	// quarantine is unresolved. A quarantine is raised against exactly one
	// WalletID and lifted by an authority certificate bound to that same
	// WalletID, so a node that is already running fails closed per wallet
	// instead: ActiveQuarantines carries every unlifted record and the pre-sign
	// authorization gate refuses to authorize, or to keep authorizing, a
	// Bitcoin signing batch for a quarantined wallet.
	if frgj.quarantineState.CurrentPoint != target ||
		frgj.quarantineState.Generation < frgj.quarantineMinimumGeneration ||
		frgj.quarantineState.Root == [32]byte{} ||
		frgj.quarantineState.ActiveRoot == [32]byte{} ||
		frgj.quarantineState.TombstoneRoot == [32]byte{} {
		return nil, fmt.Errorf("independent FROST quarantine journal is not activation-ready")
	}
	if frgj.checkpointState.Point != target ||
		frgj.checkpointState.Sequence <
			frgj.checkpointPolicy.MinimumSequence ||
		frgj.checkpointState.CertificateHash == [32]byte{} ||
		frgj.checkpointState.HistoryRoot != history.HistoryRoot {
		return nil, fmt.Errorf(
			"quorum-certified FROST checkpoint journal is not activation-ready",
		)
	}
	return &frostRetainedGroupJournalSnapshot{
		Schema:                       frostRetainedGroupJournalSnapshotSchema,
		BindingHash:                  frgj.metadata.BindingHash,
		StoreID:                      frgj.metadata.StoreID,
		StoreFingerprint:             frgj.metadata.StoreFingerprint,
		ClusterFingerprint:           frgj.metadata.ClusterFingerprint,
		CurrentPoint:                 frgj.state.CurrentPoint,
		SnapshotGeneration:           frgj.state.SnapshotGeneration,
		BatchRoot:                    frgj.state.BatchRoot,
		InventoryRoot:                root,
		WalletCount:                  walletCount,
		MinimumActualGroupSize:       minimumSize,
		MaximumActualGroupSize:       maximumSize,
		QuarantineProtocolID:         frgj.quarantineMetadata.ProtocolID,
		QuarantineStoreID:            frgj.quarantineMetadata.StoreID,
		QuarantineStoreFingerprint:   frgj.quarantineMetadata.StoreFingerprint,
		QuarantineClusterFingerprint: frgj.quarantineMetadata.ClusterFingerprint,
		QuarantineMinimumGeneration:  frgj.quarantineMinimumGeneration,
		QuarantineGeneration:         frgj.quarantineState.Generation,
		QuarantineRoot:               frgj.quarantineState.Root,
		QuarantineActiveRoot:         frgj.quarantineState.ActiveRoot,
		QuarantineTombstoneRoot:      frgj.quarantineState.TombstoneRoot,
		QuarantineCount:              frostRetainedGroupActiveQuarantineCount(frgj.quarantineState),
		ActiveQuarantines: frostRetainedGroupActiveQuarantines(
			frgj.state,
			frgj.quarantineState,
		),
		QuarantineTombstoneCount:  uint64(len(frgj.quarantineState.Tombstones)),
		CheckpointMinimumSequence: frgj.checkpointPolicy.MinimumSequence,
		CheckpointPredecessorHash: frgj.checkpointPolicy.PredecessorHash,
		CheckpointSequence:        frgj.checkpointState.Sequence,
		CheckpointCertificateHash: frgj.checkpointState.CertificateHash,
		CheckpointHistoryRoot:     frgj.checkpointState.HistoryRoot,
		LocalSessionCount:         localSessionCount,
		Complete:                  true,
	}, nil
}

func validateCompleteFrostRetainedGroupHistory(
	history *FrostRetainedGroupHistory,
	liftPolicy frostRetainedGroupQuarantineLiftPolicy,
) error {
	if history == nil {
		return fmt.Errorf("complete FROST retained-group history is nil")
	}
	if len(history.Mutations) > frostRetainedGroupMaximumMutations {
		return fmt.Errorf("complete FROST retained-group history exceeds the mutation limit")
	}
	var previous FrostRetainedGroupEventPoint
	for index, mutation := range history.Mutations {
		if !mutation.Point.valid() || mutation.Point.BlockNumber <= history.From.BlockNumber ||
			mutation.Point.BlockNumber > history.To.BlockNumber ||
			(index > 0 && compareFrostRetainedGroupEventPoints(previous, mutation.Point) >= 0) {
			return fmt.Errorf("complete FROST retained-group history has invalid event bounds or ordering")
		}
		if index > 0 && mutation.Point.BlockNumber == previous.BlockNumber &&
			mutation.Point.BlockHash != previous.BlockHash {
			return fmt.Errorf("complete FROST retained-group history has conflicting block hashes")
		}
		previous = mutation.Point
	}
	probe := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: history.From,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	if err := applyFrostRetainedGroupMutations(
		&probe,
		frostRetainedGroupCanonicalMutations(history.Mutations),
	); err != nil {
		return fmt.Errorf("complete FROST retained-group history is semantically invalid: [%w]", err)
	}
	emptyActiveRoot, err := frostRetainedGroupQuarantineActiveRoot(
		liftPolicy.ProtocolBindingHash,
		map[[32]byte]frostRetainedGroupQuarantineState{},
	)
	if err != nil {
		return fmt.Errorf("cannot initialize complete FROST quarantine history: [%w]", err)
	}
	emptyTombstoneRoot, err := frostRetainedGroupQuarantineTombstoneRoot(
		liftPolicy.ProtocolBindingHash,
		map[[32]byte]frostRetainedGroupQuarantineTombstone{},
	)
	if err != nil {
		return fmt.Errorf("cannot initialize complete FROST quarantine history: [%w]", err)
	}
	quarantineProbe := frostRetainedGroupQuarantineJournalState{
		Schema:        frostRetainedGroupQuarantineStateSchema,
		BindingHash:   liftPolicy.ProtocolBindingHash,
		CurrentPoint:  history.From,
		Root:          sha256.Sum256([]byte(frostRetainedGroupQuarantineDomain)),
		ActiveRoot:    emptyActiveRoot,
		TombstoneRoot: emptyTombstoneRoot,
		Quarantines:   []frostRetainedGroupQuarantineState{},
		Tombstones:    []frostRetainedGroupQuarantineTombstone{},
	}
	if err := applyFrostRetainedGroupQuarantineMutations(
		&quarantineProbe,
		frostRetainedGroupQuarantineMutations(history.Mutations),
		liftPolicy,
	); err != nil {
		return fmt.Errorf("complete FROST quarantine history is semantically invalid: [%w]", err)
	}
	return nil
}

func cloneFrostRetainedGroupState(
	state frostRetainedGroupJournalState,
) frostRetainedGroupJournalState {
	result := state
	result.Wallets = append([]frostRetainedGroupWalletState{}, state.Wallets...)
	for index := range result.Wallets {
		result.Wallets[index].OperatorIDs = append([]uint32{}, state.Wallets[index].OperatorIDs...)
	}
	return result
}

func cloneFrostRetainedGroupQuarantineState(
	state frostRetainedGroupQuarantineJournalState,
) frostRetainedGroupQuarantineJournalState {
	result := state
	result.Quarantines = append(
		[]frostRetainedGroupQuarantineState{},
		state.Quarantines...,
	)
	result.Tombstones = append(
		[]frostRetainedGroupQuarantineTombstone{},
		state.Tombstones...,
	)
	return result
}

type frostLocalSessionSnapshot struct {
	WalletID            [32]byte
	WalletPublicKeyHash [20]byte
	KeyGroup            string
	OperatorAddresses   []chain.Address
	ControlledSeats     []group.MemberIndex
}

func (wr *walletRegistry) frostLocalSessionSnapshot() (
	[]frostLocalSessionSnapshot,
	error,
) {
	if wr == nil {
		return nil, fmt.Errorf("wallet registry is nil")
	}
	wr.mutex.Lock()
	defer wr.mutex.Unlock()
	return wr.frostLocalSessionSnapshotLocked()
}

func (wr *walletRegistry) frostLocalSessionSnapshotLocked() (
	[]frostLocalSessionSnapshot,
	error,
) {
	result := make([]frostLocalSessionSnapshot, 0)
	for _, value := range wr.walletCache {
		if value == nil || len(value.signers) == 0 {
			return nil, fmt.Errorf("wallet registry contains an empty session")
		}
		_, isFrostWallet, err := frostKeyGroupFromWalletCacheValue(value)
		if err != nil {
			return nil, fmt.Errorf(
				"wallet registry cannot classify local session material: [%w]",
				err,
			)
		}
		if !isFrostWallet {
			continue
		}
		nativeSigners := make([]*signer, 0)
		for _, signer := range value.signers {
			if signer == nil {
				return nil, fmt.Errorf("wallet registry contains a nil signer")
			}
			switch signer.signingMaterial().(type) {
			case *frostsigning.NativeSignerMaterial, frostsigning.NativeSignerMaterial:
				nativeSigners = append(nativeSigners, signer)
			}
		}
		if len(nativeSigners) == 0 {
			continue
		}
		if len(nativeSigners) != len(value.signers) {
			return nil, fmt.Errorf("wallet registry mixes FROST and legacy signer material")
		}
		wallet := nativeSigners[0].wallet
		session := frostLocalSessionSnapshot{
			WalletID:            value.walletID,
			WalletPublicKeyHash: value.walletPublicKeyHash,
			OperatorAddresses:   append([]chain.Address{}, wallet.signingGroupOperators...),
			ControlledSeats:     make([]group.MemberIndex, 0, len(nativeSigners)),
		}
		seenSeats := make(map[group.MemberIndex]struct{})
		for _, signer := range nativeSigners {
			if signer.wallet.publicKey == nil ||
				len(signer.wallet.signingGroupOperators) != len(session.OperatorAddresses) ||
				signer.signingGroupMemberIndex == 0 ||
				int(signer.signingGroupMemberIndex) > len(session.OperatorAddresses) {
				return nil, fmt.Errorf("FROST local session signer is malformed")
			}
			for index := range session.OperatorAddresses {
				if signer.wallet.signingGroupOperators[index] != session.OperatorAddresses[index] {
					return nil, fmt.Errorf("FROST local session operators disagree")
				}
			}
			var material *frostsigning.NativeSignerMaterial
			switch value := signer.signingMaterial().(type) {
			case *frostsigning.NativeSignerMaterial:
				material = value
			case frostsigning.NativeSignerMaterial:
				valueCopy := value
				material = &valueCopy
			default:
				return nil, fmt.Errorf("FROST local session signer material is unavailable")
			}
			keyGroup, err := frostKeyGroupFromSignerMaterial(
				material,
				session.WalletID,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"FROST local session key-group handle does not identify its wallet: [%w]",
					err,
				)
			}
			if session.KeyGroup == "" {
				session.KeyGroup = keyGroup
			} else if session.KeyGroup != keyGroup {
				return nil, fmt.Errorf("FROST local session key-group handles disagree")
			}
			if _, exists := seenSeats[signer.signingGroupMemberIndex]; exists {
				return nil, fmt.Errorf("FROST local session repeats a controlled seat")
			}
			seenSeats[signer.signingGroupMemberIndex] = struct{}{}
			session.ControlledSeats = append(session.ControlledSeats, signer.signingGroupMemberIndex)
		}
		sort.Slice(session.ControlledSeats, func(i, j int) bool {
			return session.ControlledSeats[i] < session.ControlledSeats[j]
		})
		result = append(result, session)
	}
	sort.Slice(result, func(i, j int) bool {
		return bytes.Compare(result[i].WalletID[:], result[j].WalletID[:]) < 0
	})
	return result, nil
}

func (frgj *frostRetainedGroupJournal) reconcileLocalSessions(
	ctx context.Context,
	point FrostPreSignFinality,
) (uint64, error) {
	localOperatorID, err := frgj.source.ResolveOperatorID(ctx, frgj.operatorAddress, point)
	if err != nil || localOperatorID == 0 {
		return 0, fmt.Errorf("cannot resolve local FROST operator ID at challenged point: [%w]", err)
	}
	sessions, err := frgj.walletRegistry.frostLocalSessionSnapshot()
	if err != nil {
		return 0, err
	}
	sessionsByWallet := make(map[[32]byte]frostLocalSessionSnapshot, len(sessions))
	for _, session := range sessions {
		if _, exists := sessionsByWallet[session.WalletID]; exists {
			return 0, fmt.Errorf("duplicate FROST local session wallet ID")
		}
		sessionsByWallet[session.WalletID] = session
	}
	operatorIDCache := make(map[chain.Address]chain.OperatorID)
	for _, wallet := range frgj.state.Wallets {
		session, hasSession := sessionsByWallet[wallet.WalletID]
		containsLocalOperator := false
		for _, operatorID := range wallet.OperatorIDs {
			if chain.OperatorID(operatorID) == localOperatorID {
				containsLocalOperator = true
				break
			}
		}
		if wallet.Lifecycle.terminal() {
			if hasSession {
				return 0, fmt.Errorf("terminal FROST retained group still has a local session")
			}
			continue
		}
		if containsLocalOperator != hasSession {
			return 0, fmt.Errorf("FROST local-session presence differs from retained-group membership")
		}
		if !hasSession {
			continue
		}
		if session.WalletPublicKeyHash != wallet.WalletPublicKeyHash ||
			len(session.OperatorAddresses) != len(wallet.OperatorIDs) {
			return 0, fmt.Errorf("FROST local session identity or group size differs from canonical DKG")
		}
		resolvedIDs := make([]chain.OperatorID, len(session.OperatorAddresses))
		for index, address := range session.OperatorAddresses {
			operatorID, exists := operatorIDCache[address]
			if !exists {
				operatorID, err = frgj.source.ResolveOperatorID(ctx, address, point)
				if err != nil || operatorID == 0 {
					return 0, fmt.Errorf("cannot resolve FROST DKG operator at challenged point: [%w]", err)
				}
				operatorIDCache[address] = operatorID
			}
			resolvedIDs[index] = operatorID
			if uint32(operatorID) != wallet.OperatorIDs[index] {
				return 0, fmt.Errorf("FROST local session operator ordering differs from canonical DKG")
			}
		}
		expectedSeats := make([]group.MemberIndex, 0)
		for index, operatorID := range resolvedIDs {
			if operatorID == localOperatorID {
				expectedSeats = append(expectedSeats, group.MemberIndex(index+1))
			}
		}
		if len(expectedSeats) != len(session.ControlledSeats) {
			return 0, fmt.Errorf("FROST local controlled-seat count differs from canonical DKG")
		}
		for index := range expectedSeats {
			if expectedSeats[index] != session.ControlledSeats[index] {
				return 0, fmt.Errorf("FROST local controlled seats differ from canonical DKG")
			}
		}
		delete(sessionsByWallet, wallet.WalletID)
	}
	if len(sessionsByWallet) != 0 {
		return 0, fmt.Errorf("FROST local session has no canonical retained group")
	}
	return uint64(len(sessions)), nil
}

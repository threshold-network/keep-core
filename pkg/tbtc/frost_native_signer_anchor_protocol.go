package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const (
	FrostNativeSignerAnchorReadRequestSchema     = "tbtc-frost-native-signer-state-anchor-read-request/v1"
	FrostNativeSignerAnchorReadResponseSchema    = "tbtc-frost-native-signer-state-anchor-read-response/v1"
	FrostNativeSignerAnchorCASRequestSchema      = "tbtc-frost-native-signer-state-anchor-advance-request/v1"
	FrostNativeSignerAnchorHistoryRequestSchema  = "tbtc-frost-native-signer-state-anchor-history-request/v1"
	FrostNativeSignerAnchorHistoryResponseSchema = "tbtc-frost-native-signer-state-anchor-history-response/v1"

	// FrostNativeSignerCheckpointAcknowledgementSchema is consumed verbatim by
	// the native signer after Go verifies the online authority signature and
	// exact checkpoint. Do not wrap or re-encode acknowledgement response bytes.
	FrostNativeSignerCheckpointAcknowledgementSchema = "tbtc-signer-state-witness-checkpoint-ack/v1"

	FrostNativeSignerAnchorMaximumProofEntries               = 4096
	FrostNativeSignerAnchorMaximumHistoryEventsPerPage       = 256
	FrostNativeSignerAnchorMaximumHistoryProofEntriesPerPage = 4096
	FrostNativeSignerAnchorMaximumHistoryEvents              = 4096
	FrostNativeSignerAnchorMaximumHistoryProofEntries        = 4096
	FrostNativeSignerAnchorMaximumHistoryPages               = 16
	frostNativeSignerAnchorMaximumJSONDepth                  = 32

	frostNativeSignerAnchorMaximumRequestBytes         = 2 * 1024 * 1024
	frostNativeSignerAnchorMaximumResponseBytes        = 256 * 1024
	frostNativeSignerAnchorMaximumHistoryResponseBytes = 4 * 1024 * 1024

	frostNativeSignerAnchorStreamDomain         = "tbtc-frost-native-signer-anchor-stream-v1\x00"
	frostNativeSignerAnchorBindingDomain        = "tbtc-frost-native-signer-anchor-binding-v1\x00"
	frostNativeSignerAnchorTransportDomain      = "tbtc-frost-native-signer-anchor-transport-v1\x00"
	frostNativeSignerAnchorReadRequestDomain    = "tbtc-frost-native-signer-anchor-read-request-v1\x00"
	frostNativeSignerAnchorCASRequestDomain     = "tbtc-frost-native-signer-anchor-cas-request-v1\x00"
	frostNativeSignerAnchorHistoryRequestDomain = "tbtc-frost-native-signer-anchor-history-request-v1\x00"
	frostNativeSignerAnchorTransitionDomain     = "tbtc-frost-native-signer-anchor-transition-v1\x00"
)

// FrostNativeSignerAnchorIdentity is the complete authenticated identity of
// one signer-to-history-service binding. StreamID is deliberately independent
// of activation-manifest epochs and all infrastructure/key rotations;
// BindingHash binds those mutable, offline-authorized pins for each request.
type FrostNativeSignerAnchorIdentity struct {
	ProtocolID                      [32]byte
	StreamID                        [32]byte
	ActivationManifestHash          [32]byte
	ActivationManifestSequence      uint64
	TrustDomainID                   string
	EndpointLeafSPKIHash            [32]byte
	OnlineKeyHash                   [32]byte
	OperatorFingerprint             [32]byte
	HistoryStoreID                  string
	HistoryStoreFingerprint         [32]byte
	HistoryClusterFingerprint       [32]byte
	OfflineAuthorityHash            [32]byte
	ClientSPKIHash                  [32]byte
	SignerStoreFingerprint          [32]byte
	TransportBinding                [32]byte
	WitnessMaximumRecords           uint64
	WitnessRotationThresholdRecords uint64
}

// FrostNativeSignerStateWitnessCheckpoint is the full externally anchored
// native state image. StateCommitment must authenticate all other witness
// fields according to the native signer commitment transcript.
type FrostNativeSignerStateWitnessCheckpoint struct {
	StoreFingerprint        [32]byte
	Generation              uint64
	PreviousStateCommitment [32]byte
	StateImageDigest        [32]byte
	StateCommitment         [32]byte
}

// FrostNativeSignerStateWitnessAnchorRecord is a signed history-service
// readback. A present stream always retains the exact acknowledgement JSON for
// its latest transition so an ambiguous CAS can be recovered without inventing
// acknowledgement bytes locally.
type FrostNativeSignerStateWitnessAnchorRecord struct {
	Checkpoint             FrostNativeSignerStateWitnessCheckpoint
	BindingHash            [32]byte
	AcknowledgementDigest  [32]byte
	OperationID            [32]byte
	TransitionDigest       [32]byte
	ServiceEpoch           uint64
	Revision               uint64
	PreviousEventRoot      [32]byte
	EventRoot              [32]byte
	AcknowledgementJSON    []byte
	AcknowledgementExpires uint64
	ReadRecoveryJSON       []byte
	ReadRecoveryExpires    uint64
}

// FrostNativeSignerCheckpointAcknowledgement is the parsed representation of
// the exact Rust checkpoint-acknowledgement schema.
type FrostNativeSignerCheckpointAcknowledgement struct {
	BindingHash           [32]byte
	RequestDigest         [32]byte
	Nonce                 [32]byte
	Status                string
	ServiceEpoch          uint64
	Revision              uint64
	PreviousEventRoot     [32]byte
	EventRoot             [32]byte
	Checkpoint            FrostNativeSignerStateWitnessCheckpoint
	OperationID           [32]byte
	TransitionDigest      [32]byte
	CommittedAtUnixMs     uint64
	ExpiresAtUnixMs       uint64
	Signature             [ed25519.SignatureSize]byte
	SigningDigest         [32]byte
	AcknowledgementDigest [32]byte
	ExactAcknowledgement  []byte
	ExactReadRecovery     []byte
	ReadRecoveryExpiresAt uint64
}

// FrostNativeSignerStateWitnessAnchorCASResult is returned only after an exact
// candidate acknowledgement, or an equivalent signed read recovery, passes
// every identity, request, transition, freshness, and Ed25519 check.
type FrostNativeSignerStateWitnessAnchorCASResult struct {
	Acknowledgement FrostNativeSignerCheckpointAcknowledgement
	Recovered       bool
}

type FrostNativeSignerStateWitnessAnchorReference struct {
	ServiceEpoch          uint64
	Revision              uint64
	EventRoot             [32]byte
	AcknowledgementDigest [32]byte
	Checkpoint            FrostNativeSignerStateWitnessCheckpoint
}

type FrostNativeSignerStateWitnessAnchorHistoryEvent struct {
	Acknowledgement FrostNativeSignerCheckpointAcknowledgement
	WitnessProof    []frostsigning.NativeTBTCSignerStateWitnessProofEntry
}

type FrostNativeSignerStateWitnessAnchorHistory struct {
	Floor     FrostNativeSignerStateWitnessAnchorReference
	Target    FrostNativeSignerStateWitnessAnchorReference
	Events    []FrostNativeSignerStateWitnessAnchorHistoryEvent
	FinalRead *FrostNativeSignerStateWitnessAnchorRecord
}

// FrostNativeSignerStateWitnessAnchorStore is the production-facing interface
// consumed by signer readiness. Implementations must serialize operations and
// fail closed after observing an authenticated state outside an in-flight
// CAS's exact expected/candidate set.
type FrostNativeSignerStateWitnessAnchorStore interface {
	ReadFrostNativeSignerStateWitnessAnchor(
		context.Context,
	) (*FrostNativeSignerStateWitnessAnchorRecord, error)
	CompareAndSwapFrostNativeSignerStateWitnessAnchor(
		context.Context,
		FrostNativeSignerStateWitnessCheckpoint,
		FrostNativeSignerStateWitnessCheckpoint,
		[]frostsigning.NativeTBTCSignerStateWitnessProofEntry,
	) (*FrostNativeSignerStateWitnessAnchorCASResult, error)
	ReadFrostNativeSignerStateWitnessAnchorHistory(
		context.Context,
		FrostNativeSignerStateWitnessAnchorReference,
	) (*FrostNativeSignerStateWitnessAnchorHistory, error)
}

type frostNativeSignerAnchorIdentityWire struct {
	ProtocolID                      string `json:"protocolID"`
	StreamID                        string `json:"streamID"`
	ActivationManifestHash          string `json:"activationManifestHash"`
	ActivationManifestSequence      string `json:"activationManifestSequence"`
	TrustDomainID                   string `json:"trustDomainID"`
	EndpointLeafSPKIHash            string `json:"endpointLeafSpkiHash"`
	OnlineKeyHash                   string `json:"onlineKeyHash"`
	OperatorFingerprint             string `json:"operatorFingerprint"`
	HistoryStoreID                  string `json:"historyStoreID"`
	HistoryStoreFingerprint         string `json:"historyStoreFingerprint"`
	HistoryClusterFingerprint       string `json:"historyClusterFingerprint"`
	OfflineAuthorityHash            string `json:"offlineAuthorityHash"`
	ClientSPKIHash                  string `json:"clientSpkiHash"`
	SignerStoreFingerprint          string `json:"signerStoreFingerprint"`
	TransportBinding                string `json:"transportBinding"`
	WitnessMaximumRecords           string `json:"witnessMaximumRecords"`
	WitnessRotationThresholdRecords string `json:"witnessRotationThresholdRecords"`
}

type frostNativeSignerAnchorCheckpointWire struct {
	StoreFingerprint        string `json:"storeFingerprint"`
	Generation              string `json:"generation"`
	PreviousStateCommitment string `json:"previousStateCommitment"`
	StateImageDigest        string `json:"stateImageDigest"`
	StateCommitment         string `json:"stateCommitment"`
}

type frostNativeSignerAnchorProofEntryWire struct {
	Generation              string `json:"generation"`
	PreviousStateCommitment string `json:"previousStateCommitment"`
	StateImageDigest        string `json:"stateImageDigest"`
	StateCommitment         string `json:"stateCommitment"`
}

type frostNativeSignerAnchorReadRequestPayload struct {
	Kind        string                              `json:"kind"`
	Nonce       string                              `json:"nonce"`
	BindingHash string                              `json:"bindingHash"`
	Identity    frostNativeSignerAnchorIdentityWire `json:"identity"`
}

type frostNativeSignerAnchorReadRequest struct {
	Schema              string                                    `json:"schema"`
	Payload             frostNativeSignerAnchorReadRequestPayload `json:"payload"`
	ClientPublicKeySPKI string                                    `json:"clientPublicKeySpki"`
	Signature           string                                    `json:"signature"`
}

type frostNativeSignerAnchorCASRequestPayload struct {
	Kind             string                                  `json:"kind"`
	Nonce            string                                  `json:"nonce"`
	BindingHash      string                                  `json:"bindingHash"`
	Identity         frostNativeSignerAnchorIdentityWire     `json:"identity"`
	OperationID      string                                  `json:"operationID"`
	TransitionDigest string                                  `json:"transitionDigest"`
	Expected         frostNativeSignerAnchorCheckpointWire   `json:"expected"`
	Candidate        frostNativeSignerAnchorCheckpointWire   `json:"candidate"`
	Proof            []frostNativeSignerAnchorProofEntryWire `json:"proof"`
}

type frostNativeSignerAnchorCASRequest struct {
	Schema              string                                   `json:"schema"`
	Payload             frostNativeSignerAnchorCASRequestPayload `json:"payload"`
	ClientPublicKeySPKI string                                   `json:"clientPublicKeySpki"`
	Signature           string                                   `json:"signature"`
}

type frostNativeSignerAnchorAcknowledgementWire struct {
	Schema            string                                `json:"schema"`
	BindingHash       string                                `json:"bindingHash"`
	RequestDigest     string                                `json:"requestDigest"`
	Nonce             string                                `json:"nonce"`
	Status            string                                `json:"status"`
	ServiceEpoch      string                                `json:"serviceEpoch"`
	Revision          string                                `json:"revision"`
	PreviousEventRoot string                                `json:"previousEventRoot"`
	EventRoot         string                                `json:"eventRoot"`
	Checkpoint        frostNativeSignerAnchorCheckpointWire `json:"checkpoint"`
	OperationID       string                                `json:"operationID"`
	TransitionDigest  string                                `json:"transitionDigest"`
	CommittedAtUnixMs string                                `json:"committedAtUnixMs"`
	ExpiresAtUnixMs   string                                `json:"expiresAtUnixMs"`
	Signature         string                                `json:"signature"`
}

type frostNativeSignerAnchorReadResponse struct {
	Schema              string                                 `json:"schema"`
	BindingHash         string                                 `json:"bindingHash"`
	RequestDigest       string                                 `json:"requestDigest"`
	Nonce               string                                 `json:"nonce"`
	Status              string                                 `json:"status"`
	ServiceEpoch        string                                 `json:"serviceEpoch"`
	Revision            string                                 `json:"revision"`
	EventRoot           string                                 `json:"eventRoot"`
	Checkpoint          *frostNativeSignerAnchorCheckpointWire `json:"checkpoint"`
	OperationID         string                                 `json:"operationID"`
	TransitionDigest    string                                 `json:"transitionDigest"`
	CommittedAtUnixMs   string                                 `json:"committedAtUnixMs"`
	ExpiresAtUnixMs     string                                 `json:"expiresAtUnixMs"`
	CheckpointAck       json.RawMessage                        `json:"checkpointAck"`
	CheckpointAckDigest string                                 `json:"checkpointAckDigest"`
	Signature           string                                 `json:"signature"`
}

type frostNativeSignerAnchorHistoryReferenceWire struct {
	ServiceEpoch        string                                `json:"serviceEpoch"`
	Revision            string                                `json:"revision"`
	EventRoot           string                                `json:"eventRoot"`
	CheckpointAckDigest string                                `json:"checkpointAckDigest"`
	Checkpoint          frostNativeSignerAnchorCheckpointWire `json:"checkpoint"`
}

type frostNativeSignerAnchorHistoryRequestPayload struct {
	Kind                string                                      `json:"kind"`
	Nonce               string                                      `json:"nonce"`
	BindingHash         string                                      `json:"bindingHash"`
	Identity            frostNativeSignerAnchorIdentityWire         `json:"identity"`
	FloorRef            frostNativeSignerAnchorHistoryReferenceWire `json:"floorRef"`
	TargetRef           frostNativeSignerAnchorHistoryReferenceWire `json:"targetRef"`
	StartRevision       string                                      `json:"startRevision"`
	MaximumEvents       string                                      `json:"maximumEvents"`
	MaximumProofEntries string                                      `json:"maximumProofEntries"`
}

type frostNativeSignerAnchorHistoryRequest struct {
	Schema              string                                       `json:"schema"`
	Payload             frostNativeSignerAnchorHistoryRequestPayload `json:"payload"`
	ClientPublicKeySPKI string                                       `json:"clientPublicKeySpki"`
	Signature           string                                       `json:"signature"`
}

type frostNativeSignerAnchorHistoryEventWire struct {
	CheckpointAck json.RawMessage                         `json:"checkpointAck"`
	WitnessProof  []frostNativeSignerAnchorProofEntryWire `json:"witnessProof"`
}

type frostNativeSignerAnchorHistoryResponse struct {
	Schema            string                                      `json:"schema"`
	BindingHash       string                                      `json:"bindingHash"`
	RequestDigest     string                                      `json:"requestDigest"`
	Nonce             string                                      `json:"nonce"`
	Status            string                                      `json:"status"`
	ServiceEpoch      string                                      `json:"serviceEpoch"`
	FloorRef          frostNativeSignerAnchorHistoryReferenceWire `json:"floorRef"`
	TargetRef         frostNativeSignerAnchorHistoryReferenceWire `json:"targetRef"`
	StartRevision     string                                      `json:"startRevision"`
	NextRevision      string                                      `json:"nextRevision"`
	EventCount        string                                      `json:"eventCount"`
	ProofEntryCount   string                                      `json:"proofEntryCount"`
	Events            *[]frostNativeSignerAnchorHistoryEventWire  `json:"events"`
	CommittedAtUnixMs string                                      `json:"committedAtUnixMs"`
	ExpiresAtUnixMs   string                                      `json:"expiresAtUnixMs"`
	Signature         string                                      `json:"signature"`
}

func frostNativeSignerAnchorIdentityToWire(
	identity FrostNativeSignerAnchorIdentity,
) frostNativeSignerAnchorIdentityWire {
	return frostNativeSignerAnchorIdentityWire{
		ProtocolID:                 frostNativeSignerAnchorHex32(identity.ProtocolID),
		StreamID:                   frostNativeSignerAnchorHex32(identity.StreamID),
		ActivationManifestHash:     frostNativeSignerAnchorHex32(identity.ActivationManifestHash),
		ActivationManifestSequence: strconv.FormatUint(identity.ActivationManifestSequence, 10),
		TrustDomainID:              identity.TrustDomainID,
		EndpointLeafSPKIHash:       frostNativeSignerAnchorHex32(identity.EndpointLeafSPKIHash),
		OnlineKeyHash:              frostNativeSignerAnchorHex32(identity.OnlineKeyHash),
		OperatorFingerprint:        frostNativeSignerAnchorHex32(identity.OperatorFingerprint),
		HistoryStoreID:             identity.HistoryStoreID,
		HistoryStoreFingerprint:    frostNativeSignerAnchorHex32(identity.HistoryStoreFingerprint),
		HistoryClusterFingerprint:  frostNativeSignerAnchorHex32(identity.HistoryClusterFingerprint),
		OfflineAuthorityHash:       frostNativeSignerAnchorHex32(identity.OfflineAuthorityHash),
		ClientSPKIHash:             frostNativeSignerAnchorHex32(identity.ClientSPKIHash),
		SignerStoreFingerprint:     frostNativeSignerAnchorHex32(identity.SignerStoreFingerprint),
		TransportBinding:           frostNativeSignerAnchorHex32(identity.TransportBinding),
		WitnessMaximumRecords:      strconv.FormatUint(identity.WitnessMaximumRecords, 10),
		WitnessRotationThresholdRecords: strconv.FormatUint(
			identity.WitnessRotationThresholdRecords,
			10,
		),
	}
}

func frostNativeSignerAnchorCheckpointToWire(
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) frostNativeSignerAnchorCheckpointWire {
	return frostNativeSignerAnchorCheckpointWire{
		StoreFingerprint:        frostNativeSignerAnchorHex32(checkpoint.StoreFingerprint),
		Generation:              strconv.FormatUint(checkpoint.Generation, 10),
		PreviousStateCommitment: frostNativeSignerAnchorHex32(checkpoint.PreviousStateCommitment),
		StateImageDigest:        frostNativeSignerAnchorHex32(checkpoint.StateImageDigest),
		StateCommitment:         frostNativeSignerAnchorHex32(checkpoint.StateCommitment),
	}
}

func frostNativeSignerAnchorProofToWire(
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
) []frostNativeSignerAnchorProofEntryWire {
	result := make([]frostNativeSignerAnchorProofEntryWire, len(proof))
	for index, entry := range proof {
		result[index] = frostNativeSignerAnchorProofEntryWire{
			Generation:              strconv.FormatUint(entry.Generation, 10),
			PreviousStateCommitment: frostNativeSignerAnchorHex32(entry.PreviousStateCommitment),
			StateImageDigest:        frostNativeSignerAnchorHex32(entry.StateImageDigest),
			StateCommitment:         frostNativeSignerAnchorHex32(entry.StateCommitment),
		}
	}
	return result
}

func frostNativeSignerAnchorHistoryReferenceToWire(
	reference FrostNativeSignerStateWitnessAnchorReference,
) frostNativeSignerAnchorHistoryReferenceWire {
	return frostNativeSignerAnchorHistoryReferenceWire{
		ServiceEpoch:        strconv.FormatUint(reference.ServiceEpoch, 10),
		Revision:            strconv.FormatUint(reference.Revision, 10),
		EventRoot:           frostNativeSignerAnchorHex32(reference.EventRoot),
		CheckpointAckDigest: frostNativeSignerAnchorHex32(reference.AcknowledgementDigest),
		Checkpoint:          frostNativeSignerAnchorCheckpointToWire(reference.Checkpoint),
	}
}

func frostNativeSignerAnchorHistoryReferenceFromWire(
	wire frostNativeSignerAnchorHistoryReferenceWire,
) (FrostNativeSignerStateWitnessAnchorReference, error) {
	serviceEpoch, err := frostNativeSignerAnchorParseUint64(wire.ServiceEpoch)
	if err != nil {
		return FrostNativeSignerStateWitnessAnchorReference{}, err
	}
	revision, err := frostNativeSignerAnchorParseUint64(wire.Revision)
	if err != nil {
		return FrostNativeSignerStateWitnessAnchorReference{}, err
	}
	eventRoot, err := frostNativeSignerAnchorParseHex32(wire.EventRoot)
	if err != nil {
		return FrostNativeSignerStateWitnessAnchorReference{}, err
	}
	acknowledgementDigest, err := frostNativeSignerAnchorParseHex32(
		wire.CheckpointAckDigest,
	)
	if err != nil {
		return FrostNativeSignerStateWitnessAnchorReference{}, err
	}
	checkpoint, err := frostNativeSignerAnchorCheckpointFromWire(wire.Checkpoint)
	if err != nil {
		return FrostNativeSignerStateWitnessAnchorReference{}, err
	}
	return FrostNativeSignerStateWitnessAnchorReference{
		ServiceEpoch:          serviceEpoch,
		Revision:              revision,
		EventRoot:             eventRoot,
		AcknowledgementDigest: acknowledgementDigest,
		Checkpoint:            checkpoint,
	}, nil
}

func frostNativeSignerAnchorProofFromWire(
	wire []frostNativeSignerAnchorProofEntryWire,
) ([]frostsigning.NativeTBTCSignerStateWitnessProofEntry, error) {
	result := make([]frostsigning.NativeTBTCSignerStateWitnessProofEntry, len(wire))
	for index, entry := range wire {
		generation, err := frostNativeSignerAnchorParseUint64(entry.Generation)
		if err != nil {
			return nil, err
		}
		previous, err := frostNativeSignerAnchorParseHex32(entry.PreviousStateCommitment)
		if err != nil {
			return nil, err
		}
		image, err := frostNativeSignerAnchorParseHex32(entry.StateImageDigest)
		if err != nil {
			return nil, err
		}
		commitment, err := frostNativeSignerAnchorParseHex32(entry.StateCommitment)
		if err != nil {
			return nil, err
		}
		result[index] = frostsigning.NativeTBTCSignerStateWitnessProofEntry{
			Generation:              generation,
			PreviousStateCommitment: previous,
			StateImageDigest:        image,
			StateCommitment:         commitment,
		}
	}
	return result, nil
}

// ComputeFrostNativeSignerAnchorStreamID derives the stable history stream.
// Activation manifest hash/sequence and online/TLS keys are intentionally
// omitted so an offline-authorized rotation cannot create an empty new stream.
func ComputeFrostNativeSignerAnchorStreamID(
	identity FrostNativeSignerAnchorIdentity,
) [32]byte {
	transcript := newFrostNativeSignerAnchorTranscript(frostNativeSignerAnchorStreamDomain)
	transcript.bytes32("protocolID", identity.ProtocolID)
	transcript.string("trustDomainID", identity.TrustDomainID)
	transcript.bytes32("signerStoreFingerprint", identity.SignerStoreFingerprint)
	return sha256.Sum256(transcript.bytes())
}

// ComputeFrostNativeSignerAnchorBindingHash binds the current manifest epoch
// and rotating online/TLS pins to the stable stream.
func ComputeFrostNativeSignerAnchorBindingHash(
	identity FrostNativeSignerAnchorIdentity,
) [32]byte {
	transcript := newFrostNativeSignerAnchorTranscript(frostNativeSignerAnchorBindingDomain)
	frostNativeSignerAnchorWriteIdentity(transcript, identity)
	return sha256.Sum256(transcript.bytes())
}

// ComputeFrostNativeSignerAnchorTransportBinding commits to the exact canonical
// base endpoint configured for the client.
func ComputeFrostNativeSignerAnchorTransportBinding(endpoint string) [32]byte {
	transcript := newFrostNativeSignerAnchorTranscript(frostNativeSignerAnchorTransportDomain)
	transcript.string("endpoint", endpoint)
	return sha256.Sum256(transcript.bytes())
}

func frostNativeSignerAnchorWriteIdentity(
	transcript *frostNativeSignerAnchorTranscript,
	identity FrostNativeSignerAnchorIdentity,
) {
	transcript.bytes32("protocolID", identity.ProtocolID)
	transcript.bytes32("streamID", identity.StreamID)
	transcript.bytes32("activationManifestHash", identity.ActivationManifestHash)
	transcript.uint64("activationManifestSequence", identity.ActivationManifestSequence)
	transcript.string("trustDomainID", identity.TrustDomainID)
	transcript.bytes32("endpointLeafSpkiHash", identity.EndpointLeafSPKIHash)
	transcript.bytes32("onlineKeyHash", identity.OnlineKeyHash)
	transcript.bytes32("operatorFingerprint", identity.OperatorFingerprint)
	transcript.string("historyStoreID", identity.HistoryStoreID)
	transcript.bytes32("historyStoreFingerprint", identity.HistoryStoreFingerprint)
	transcript.bytes32("historyClusterFingerprint", identity.HistoryClusterFingerprint)
	transcript.bytes32("offlineAuthorityHash", identity.OfflineAuthorityHash)
	transcript.bytes32("clientSpkiHash", identity.ClientSPKIHash)
	transcript.bytes32("signerStoreFingerprint", identity.SignerStoreFingerprint)
	transcript.bytes32("transportBinding", identity.TransportBinding)
	transcript.uint64("witnessMaximumRecords", identity.WitnessMaximumRecords)
	transcript.uint64(
		"witnessRotationThresholdRecords",
		identity.WitnessRotationThresholdRecords,
	)
}

func frostNativeSignerAnchorWriteCheckpoint(
	transcript *frostNativeSignerAnchorTranscript,
	prefix string,
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) {
	transcript.bytes32(prefix+".storeFingerprint", checkpoint.StoreFingerprint)
	transcript.uint64(prefix+".generation", checkpoint.Generation)
	transcript.bytes32(prefix+".previousStateCommitment", checkpoint.PreviousStateCommitment)
	transcript.bytes32(prefix+".stateImageDigest", checkpoint.StateImageDigest)
	transcript.bytes32(prefix+".stateCommitment", checkpoint.StateCommitment)
}

func frostNativeSignerAnchorReadRequestTranscript(
	identity FrostNativeSignerAnchorIdentity,
	nonce [32]byte,
	clientSPKIDER []byte,
) []byte {
	transcript := newFrostNativeSignerAnchorTranscript(frostNativeSignerAnchorReadRequestDomain)
	transcript.string("schema", FrostNativeSignerAnchorReadRequestSchema)
	transcript.string("kind", "read")
	frostNativeSignerAnchorWriteIdentity(transcript, identity)
	transcript.bytes32("bindingHash", ComputeFrostNativeSignerAnchorBindingHash(identity))
	transcript.bytes32("nonce", nonce)
	transcript.field("clientPublicKeySpki", clientSPKIDER)
	return transcript.bytes()
}

func frostNativeSignerAnchorCASRequestTranscript(
	identity FrostNativeSignerAnchorIdentity,
	nonce [32]byte,
	operationID [32]byte,
	transitionDigest [32]byte,
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
	clientSPKIDER []byte,
) []byte {
	transcript := newFrostNativeSignerAnchorTranscript(frostNativeSignerAnchorCASRequestDomain)
	transcript.string("schema", FrostNativeSignerAnchorCASRequestSchema)
	transcript.string("kind", "advance")
	frostNativeSignerAnchorWriteIdentity(transcript, identity)
	transcript.bytes32("bindingHash", ComputeFrostNativeSignerAnchorBindingHash(identity))
	transcript.bytes32("nonce", nonce)
	transcript.bytes32("operationID", operationID)
	transcript.bytes32("transitionDigest", transitionDigest)
	frostNativeSignerAnchorWriteCheckpoint(transcript, "expected", expected)
	frostNativeSignerAnchorWriteCheckpoint(transcript, "candidate", candidate)
	transcript.uint64("proof.count", uint64(len(proof)))
	for index, entry := range proof {
		prefix := "proof." + strconv.Itoa(index)
		transcript.uint64(prefix+".generation", entry.Generation)
		transcript.bytes32(prefix+".previousStateCommitment", entry.PreviousStateCommitment)
		transcript.bytes32(prefix+".stateImageDigest", entry.StateImageDigest)
		transcript.bytes32(prefix+".stateCommitment", entry.StateCommitment)
	}
	transcript.field("clientPublicKeySpki", clientSPKIDER)
	return transcript.bytes()
}

func frostNativeSignerAnchorHistoryRequestTranscript(
	identity FrostNativeSignerAnchorIdentity,
	nonce [32]byte,
	floor FrostNativeSignerStateWitnessAnchorReference,
	target FrostNativeSignerStateWitnessAnchorReference,
	startRevision uint64,
	maximumEvents uint64,
	maximumProofEntries uint64,
	clientSPKIDER []byte,
) []byte {
	transcript := newFrostNativeSignerAnchorTranscript(
		frostNativeSignerAnchorHistoryRequestDomain,
	)
	transcript.string("schema", FrostNativeSignerAnchorHistoryRequestSchema)
	transcript.string("kind", "history")
	frostNativeSignerAnchorWriteIdentity(transcript, identity)
	transcript.bytes32("bindingHash", ComputeFrostNativeSignerAnchorBindingHash(identity))
	transcript.bytes32("nonce", nonce)
	frostNativeSignerAnchorWriteHistoryReference(transcript, "floor", floor)
	frostNativeSignerAnchorWriteHistoryReference(transcript, "target", target)
	transcript.uint64("startRevision", startRevision)
	transcript.uint64("maximumEvents", maximumEvents)
	transcript.uint64("maximumProofEntries", maximumProofEntries)
	transcript.field("clientPublicKeySpki", clientSPKIDER)
	return transcript.bytes()
}

func frostNativeSignerAnchorWriteHistoryReference(
	transcript *frostNativeSignerAnchorTranscript,
	prefix string,
	reference FrostNativeSignerStateWitnessAnchorReference,
) {
	transcript.uint64(prefix+".serviceEpoch", reference.ServiceEpoch)
	transcript.uint64(prefix+".revision", reference.Revision)
	transcript.bytes32(prefix+".eventRoot", reference.EventRoot)
	transcript.bytes32(
		prefix+".checkpointAckDigest",
		reference.AcknowledgementDigest,
	)
	frostNativeSignerAnchorWriteCheckpoint(
		transcript,
		prefix+".checkpoint",
		reference.Checkpoint,
	)
}

func computeFrostNativeSignerAnchorTransitionDigest(
	identity FrostNativeSignerAnchorIdentity,
	operationID [32]byte,
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
) [32]byte {
	transcript := newFrostNativeSignerAnchorTranscript(frostNativeSignerAnchorTransitionDomain)
	transcript.bytes32("bindingHash", ComputeFrostNativeSignerAnchorBindingHash(identity))
	transcript.bytes32("operationID", operationID)
	frostNativeSignerAnchorWriteCheckpoint(transcript, "expected", expected)
	frostNativeSignerAnchorWriteCheckpoint(transcript, "candidate", candidate)
	transcript.uint64("proof.count", uint64(len(proof)))
	for index, entry := range proof {
		prefix := "proof." + strconv.Itoa(index)
		transcript.uint64(prefix+".generation", entry.Generation)
		transcript.bytes32(prefix+".previousStateCommitment", entry.PreviousStateCommitment)
		transcript.bytes32(prefix+".stateImageDigest", entry.StateImageDigest)
		transcript.bytes32(prefix+".stateCommitment", entry.StateCommitment)
	}
	return sha256.Sum256(transcript.bytes())
}

func frostNativeSignerAnchorAcknowledgementTranscript(
	wire frostNativeSignerAnchorAcknowledgementWire,
) ([]byte, error) {
	checkpoint, err := frostNativeSignerAnchorCheckpointFromWire(wire.Checkpoint)
	if err != nil {
		return nil, err
	}
	serviceEpoch, err := frostNativeSignerAnchorParseUint64(wire.ServiceEpoch)
	if err != nil {
		return nil, fmt.Errorf("invalid service epoch: %w", err)
	}
	revision, err := frostNativeSignerAnchorParseUint64(wire.Revision)
	if err != nil {
		return nil, fmt.Errorf("invalid revision: %w", err)
	}
	committedAt, err := frostNativeSignerAnchorParseUint64(wire.CommittedAtUnixMs)
	if err != nil {
		return nil, fmt.Errorf("invalid commit time: %w", err)
	}
	expiresAt, err := frostNativeSignerAnchorParseUint64(wire.ExpiresAtUnixMs)
	if err != nil {
		return nil, fmt.Errorf("invalid expiry time: %w", err)
	}
	bytes32Fields := []struct {
		name  string
		value string
	}{
		{"bindingHash", wire.BindingHash},
		{"requestDigest", wire.RequestDigest},
		{"nonce", wire.Nonce},
		{"previousEventRoot", wire.PreviousEventRoot},
		{"eventRoot", wire.EventRoot},
		{"operationID", wire.OperationID},
		{"transitionDigest", wire.TransitionDigest},
	}
	decoded := make(map[string][32]byte, len(bytes32Fields))
	for _, field := range bytes32Fields {
		value, err := frostNativeSignerAnchorParseHex32(field.value)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", field.name, err)
		}
		decoded[field.name] = value
	}
	status := byte(0)
	switch wire.Status {
	case "applied":
		status = 0x01
	case "already-applied":
		status = 0x02
	default:
		return nil, fmt.Errorf("invalid checkpoint acknowledgement status")
	}
	buffer := bytes.NewBuffer(nil)
	write32 := func(name string) {
		value := decoded[name]
		buffer.Write(value[:])
	}
	buffer.WriteString("tbtc-native-signer-state-anchor-service-response/v1\x00")
	write32("bindingHash")
	write32("requestDigest")
	write32("nonce")
	buffer.WriteByte(status)
	_ = binary.Write(buffer, binary.BigEndian, serviceEpoch)
	_ = binary.Write(buffer, binary.BigEndian, revision)
	write32("previousEventRoot")
	write32("eventRoot")
	buffer.Write(checkpoint.StoreFingerprint[:])
	_ = binary.Write(buffer, binary.BigEndian, checkpoint.Generation)
	buffer.Write(checkpoint.PreviousStateCommitment[:])
	buffer.Write(checkpoint.StateImageDigest[:])
	buffer.Write(checkpoint.StateCommitment[:])
	write32("operationID")
	write32("transitionDigest")
	_ = binary.Write(buffer, binary.BigEndian, committedAt)
	_ = binary.Write(buffer, binary.BigEndian, expiresAt)
	digest := sha256.Sum256(buffer.Bytes())
	return digest[:], nil
}

func frostNativeSignerAnchorReadResponseTranscript(
	response frostNativeSignerAnchorReadResponse,
) ([]byte, error) {
	bindingHash, err := frostNativeSignerAnchorParseHex32(response.BindingHash)
	if err != nil {
		return nil, err
	}
	requestDigest, err := frostNativeSignerAnchorParseHex32(response.RequestDigest)
	if err != nil {
		return nil, err
	}
	nonce, err := frostNativeSignerAnchorParseHex32(response.Nonce)
	if err != nil {
		return nil, err
	}
	serviceEpoch, err := frostNativeSignerAnchorParseUint64(response.ServiceEpoch)
	if err != nil {
		return nil, err
	}
	revision, err := frostNativeSignerAnchorParseUint64(response.Revision)
	if err != nil {
		return nil, err
	}
	eventRoot, err := frostNativeSignerAnchorParseHex32(response.EventRoot)
	if err != nil {
		return nil, err
	}
	operationID, err := frostNativeSignerAnchorParseHex32(response.OperationID)
	if err != nil {
		return nil, err
	}
	transitionDigest, err := frostNativeSignerAnchorParseHex32(response.TransitionDigest)
	if err != nil {
		return nil, err
	}
	ackDigest, err := frostNativeSignerAnchorParseHex32(response.CheckpointAckDigest)
	if err != nil {
		return nil, err
	}
	committedAt, err := frostNativeSignerAnchorParseUint64(response.CommittedAtUnixMs)
	if err != nil {
		return nil, err
	}
	expiresAt, err := frostNativeSignerAnchorParseUint64(response.ExpiresAtUnixMs)
	if err != nil {
		return nil, err
	}
	status := byte(0)
	checkpoint := FrostNativeSignerStateWitnessCheckpoint{}
	rawAcknowledgementDigest := [32]byte{}
	switch response.Status {
	case "present":
		status = 0x01
		if response.Checkpoint == nil || len(response.CheckpointAck) == 0 ||
			bytes.Equal(bytes.TrimSpace(response.CheckpointAck), []byte("null")) {
			return nil, fmt.Errorf("present checkpoint or acknowledgement is absent")
		}
		checkpoint, err = frostNativeSignerAnchorCheckpointFromWire(*response.Checkpoint)
		if err != nil {
			return nil, err
		}
		rawAcknowledgementDigest = sha256.Sum256(response.CheckpointAck)
	case "absent":
		if response.Checkpoint != nil ||
			(len(response.CheckpointAck) != 0 &&
				!bytes.Equal(bytes.TrimSpace(response.CheckpointAck), []byte("null"))) ||
			serviceEpoch != 0 || revision != 0 || eventRoot != [32]byte{} ||
			operationID != [32]byte{} || transitionDigest != [32]byte{} ||
			committedAt != 0 || expiresAt != 0 || ackDigest != [32]byte{} {
			return nil, fmt.Errorf("absent checkpoint response contains state")
		}
	default:
		return nil, fmt.Errorf("invalid checkpoint read status")
	}
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("tbtc-native-signer-state-anchor-read-response/v1\x00")
	buffer.Write(bindingHash[:])
	buffer.Write(requestDigest[:])
	buffer.Write(nonce[:])
	buffer.WriteByte(status)
	_ = binary.Write(buffer, binary.BigEndian, serviceEpoch)
	_ = binary.Write(buffer, binary.BigEndian, revision)
	buffer.Write(eventRoot[:])
	buffer.Write(checkpoint.StoreFingerprint[:])
	_ = binary.Write(buffer, binary.BigEndian, checkpoint.Generation)
	buffer.Write(checkpoint.PreviousStateCommitment[:])
	buffer.Write(checkpoint.StateImageDigest[:])
	buffer.Write(checkpoint.StateCommitment[:])
	buffer.Write(operationID[:])
	buffer.Write(transitionDigest[:])
	_ = binary.Write(buffer, binary.BigEndian, committedAt)
	_ = binary.Write(buffer, binary.BigEndian, expiresAt)
	buffer.Write(ackDigest[:])
	buffer.Write(rawAcknowledgementDigest[:])
	digest := sha256.Sum256(buffer.Bytes())
	return digest[:], nil
}

func computeFrostNativeSignerAnchorHistoryEventDigest(
	revision uint64,
	acknowledgementDigest [32]byte,
	rawAcknowledgement []byte,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
) [32]byte {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("tbtc-native-signer-state-anchor-history-event/v1\x00")
	_ = binary.Write(buffer, binary.BigEndian, revision)
	buffer.Write(acknowledgementDigest[:])
	rawDigest := sha256.Sum256(rawAcknowledgement)
	buffer.Write(rawDigest[:])
	_ = binary.Write(buffer, binary.BigEndian, uint32(len(proof)))
	for _, entry := range proof {
		_ = binary.Write(buffer, binary.BigEndian, entry.Generation)
		buffer.Write(entry.PreviousStateCommitment[:])
		buffer.Write(entry.StateImageDigest[:])
		buffer.Write(entry.StateCommitment[:])
	}
	return sha256.Sum256(buffer.Bytes())
}

func frostNativeSignerAnchorHistoryResponseTranscript(
	response frostNativeSignerAnchorHistoryResponse,
	eventDigests [][32]byte,
) ([]byte, error) {
	bindingHash, err := frostNativeSignerAnchorParseHex32(response.BindingHash)
	if err != nil {
		return nil, err
	}
	requestDigest, err := frostNativeSignerAnchorParseHex32(response.RequestDigest)
	if err != nil {
		return nil, err
	}
	nonce, err := frostNativeSignerAnchorParseHex32(response.Nonce)
	if err != nil {
		return nil, err
	}
	serviceEpoch, err := frostNativeSignerAnchorParseUint64(response.ServiceEpoch)
	if err != nil {
		return nil, err
	}
	floor, err := frostNativeSignerAnchorHistoryReferenceFromWire(response.FloorRef)
	if err != nil {
		return nil, err
	}
	target, err := frostNativeSignerAnchorHistoryReferenceFromWire(response.TargetRef)
	if err != nil {
		return nil, err
	}
	startRevision, err := frostNativeSignerAnchorParseUint64(response.StartRevision)
	if err != nil {
		return nil, err
	}
	nextRevision, err := frostNativeSignerAnchorParseUint64(response.NextRevision)
	if err != nil {
		return nil, err
	}
	eventCount, err := frostNativeSignerAnchorParseUint64(response.EventCount)
	if err != nil || eventCount > uint64(^uint32(0)) ||
		eventCount != uint64(len(eventDigests)) {
		return nil, fmt.Errorf("history event count is invalid")
	}
	proofEntryCount, err := frostNativeSignerAnchorParseUint64(response.ProofEntryCount)
	if err != nil || proofEntryCount > uint64(^uint32(0)) {
		return nil, fmt.Errorf("history proof-entry count is invalid")
	}
	committedAt, err := frostNativeSignerAnchorParseUint64(response.CommittedAtUnixMs)
	if err != nil {
		return nil, err
	}
	expiresAt, err := frostNativeSignerAnchorParseUint64(response.ExpiresAtUnixMs)
	if err != nil {
		return nil, err
	}
	status := byte(0)
	switch response.Status {
	case "partial":
		status = 0x01
	case "complete":
		status = 0x02
	default:
		return nil, fmt.Errorf("history response status is invalid")
	}
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("tbtc-native-signer-state-anchor-history-response/v1\x00")
	buffer.Write(bindingHash[:])
	buffer.Write(requestDigest[:])
	buffer.Write(nonce[:])
	buffer.WriteByte(status)
	_ = binary.Write(buffer, binary.BigEndian, serviceEpoch)
	frostNativeSignerAnchorWriteFixedHistoryReference(buffer, floor)
	frostNativeSignerAnchorWriteFixedHistoryReference(buffer, target)
	_ = binary.Write(buffer, binary.BigEndian, startRevision)
	_ = binary.Write(buffer, binary.BigEndian, nextRevision)
	_ = binary.Write(buffer, binary.BigEndian, uint32(eventCount))
	_ = binary.Write(buffer, binary.BigEndian, uint32(proofEntryCount))
	for _, digest := range eventDigests {
		buffer.Write(digest[:])
	}
	_ = binary.Write(buffer, binary.BigEndian, committedAt)
	_ = binary.Write(buffer, binary.BigEndian, expiresAt)
	digest := sha256.Sum256(buffer.Bytes())
	return digest[:], nil
}

func frostNativeSignerAnchorWriteFixedHistoryReference(
	buffer *bytes.Buffer,
	reference FrostNativeSignerStateWitnessAnchorReference,
) {
	_ = binary.Write(buffer, binary.BigEndian, reference.ServiceEpoch)
	_ = binary.Write(buffer, binary.BigEndian, reference.Revision)
	buffer.Write(reference.EventRoot[:])
	buffer.Write(reference.AcknowledgementDigest[:])
	buffer.Write(reference.Checkpoint.StoreFingerprint[:])
	_ = binary.Write(buffer, binary.BigEndian, reference.Checkpoint.Generation)
	buffer.Write(reference.Checkpoint.PreviousStateCommitment[:])
	buffer.Write(reference.Checkpoint.StateImageDigest[:])
	buffer.Write(reference.Checkpoint.StateCommitment[:])
}

func frostNativeSignerAnchorCheckpointFromWire(
	wire frostNativeSignerAnchorCheckpointWire,
) (FrostNativeSignerStateWitnessCheckpoint, error) {
	generation, err := frostNativeSignerAnchorParseUint64(wire.Generation)
	if err != nil {
		return FrostNativeSignerStateWitnessCheckpoint{}, fmt.Errorf("invalid checkpoint generation: %w", err)
	}
	result := FrostNativeSignerStateWitnessCheckpoint{Generation: generation}
	fields := []struct {
		name        string
		value       string
		destination *[32]byte
	}{
		{"store fingerprint", wire.StoreFingerprint, &result.StoreFingerprint},
		{"previous state commitment", wire.PreviousStateCommitment, &result.PreviousStateCommitment},
		{"state image digest", wire.StateImageDigest, &result.StateImageDigest},
		{"state commitment", wire.StateCommitment, &result.StateCommitment},
	}
	for _, field := range fields {
		value, err := frostNativeSignerAnchorParseHex32(field.value)
		if err != nil {
			return FrostNativeSignerStateWitnessCheckpoint{}, fmt.Errorf(
				"invalid checkpoint %s: %w",
				field.name,
				err,
			)
		}
		*field.destination = value
	}
	return result, nil
}

func validateFrostNativeSignerAnchorCheckpoint(
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
	storeFingerprint [32]byte,
) error {
	if checkpoint.StoreFingerprint != storeFingerprint ||
		checkpoint.Generation == 0 ||
		checkpoint.PreviousStateCommitment == [32]byte{} ||
		checkpoint.StateImageDigest == [32]byte{} ||
		checkpoint.StateCommitment == [32]byte{} {
		return fmt.Errorf("state-witness checkpoint is incomplete or belongs to another store")
	}
	computed := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
		checkpoint.StoreFingerprint,
		checkpoint.Generation,
		checkpoint.PreviousStateCommitment,
		checkpoint.StateImageDigest,
	)
	if computed != checkpoint.StateCommitment {
		return fmt.Errorf("state-witness checkpoint commitment mismatch")
	}
	return nil
}

func validateFrostNativeSignerAnchorHistoryBounds(
	floor FrostNativeSignerStateWitnessAnchorReference,
	target FrostNativeSignerStateWitnessAnchorReference,
	storeFingerprint [32]byte,
) error {
	for name, reference := range map[string]FrostNativeSignerStateWitnessAnchorReference{
		"floor":  floor,
		"target": target,
	} {
		if reference.ServiceEpoch == 0 || reference.Revision == 0 ||
			reference.EventRoot == [32]byte{} ||
			reference.AcknowledgementDigest == [32]byte{} {
			return fmt.Errorf("history %s reference is incomplete", name)
		}
		if err := validateFrostNativeSignerAnchorCheckpoint(
			reference.Checkpoint,
			storeFingerprint,
		); err != nil {
			return fmt.Errorf("invalid history %s checkpoint: %w", name, err)
		}
	}
	if floor.ServiceEpoch != target.ServiceEpoch ||
		target.Revision < floor.Revision ||
		target.Revision-floor.Revision > FrostNativeSignerAnchorMaximumHistoryEvents {
		return fmt.Errorf("history references are outside the bounded service epoch")
	}
	if target.Revision == floor.Revision && target != floor {
		return fmt.Errorf("equal history revisions identify different references")
	}
	if target.Checkpoint.Generation < floor.Checkpoint.Generation ||
		target.Checkpoint.Generation-floor.Checkpoint.Generation >
			FrostNativeSignerAnchorMaximumHistoryProofEntries {
		return fmt.Errorf("history checkpoint generations are outside the bounded proof window")
	}
	return nil
}

func validateFrostNativeSignerAnchorTransition(
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
	proof []frostsigning.NativeTBTCSignerStateWitnessProofEntry,
	storeFingerprint [32]byte,
) error {
	if err := validateFrostNativeSignerAnchorCheckpoint(expected, storeFingerprint); err != nil {
		return fmt.Errorf("invalid expected checkpoint: %w", err)
	}
	if err := validateFrostNativeSignerAnchorCheckpoint(candidate, storeFingerprint); err != nil {
		return fmt.Errorf("invalid candidate checkpoint: %w", err)
	}
	if candidate.Generation <= expected.Generation ||
		candidate.Generation-expected.Generation > FrostNativeSignerAnchorMaximumProofEntries ||
		uint64(len(proof)) != candidate.Generation-expected.Generation {
		return fmt.Errorf("checkpoint transition is not a bounded strict advance")
	}
	if len(proof) == 0 || len(proof) > FrostNativeSignerAnchorMaximumProofEntries {
		return fmt.Errorf("checkpoint proof length is invalid")
	}
	cursorGeneration := expected.Generation
	cursorCommitment := expected.StateCommitment
	for _, entry := range proof {
		if entry.Generation != cursorGeneration+1 ||
			entry.PreviousStateCommitment != cursorCommitment ||
			entry.StateImageDigest == [32]byte{} ||
			entry.StateCommitment == [32]byte{} {
			return fmt.Errorf("checkpoint proof is not contiguous")
		}
		computed := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			storeFingerprint,
			entry.Generation,
			entry.PreviousStateCommitment,
			entry.StateImageDigest,
		)
		if computed != entry.StateCommitment {
			return fmt.Errorf("checkpoint proof commitment mismatch")
		}
		cursorGeneration = entry.Generation
		cursorCommitment = entry.StateCommitment
	}
	last := proof[len(proof)-1]
	if last.Generation != candidate.Generation ||
		last.PreviousStateCommitment != candidate.PreviousStateCommitment ||
		last.StateImageDigest != candidate.StateImageDigest ||
		last.StateCommitment != candidate.StateCommitment {
		return fmt.Errorf("checkpoint proof does not terminate at the candidate")
	}
	return nil
}

func validateFrostNativeSignerAnchorIdentity(
	identity FrostNativeSignerAnchorIdentity,
	https bool,
) error {
	required := map[string][32]byte{
		"protocol ID":                 identity.ProtocolID,
		"stream ID":                   identity.StreamID,
		"activation manifest hash":    identity.ActivationManifestHash,
		"online key hash":             identity.OnlineKeyHash,
		"operator fingerprint":        identity.OperatorFingerprint,
		"history store fingerprint":   identity.HistoryStoreFingerprint,
		"history cluster fingerprint": identity.HistoryClusterFingerprint,
		"offline authority hash":      identity.OfflineAuthorityHash,
		"client SPKI hash":            identity.ClientSPKIHash,
		"signer store fingerprint":    identity.SignerStoreFingerprint,
		"transport binding":           identity.TransportBinding,
	}
	if https {
		required["endpoint leaf SPKI hash"] = identity.EndpointLeafSPKIHash
	} else if identity.EndpointLeafSPKIHash != [32]byte{} {
		return fmt.Errorf("loopback HTTP identity must use a zero endpoint leaf SPKI hash")
	}
	for name, value := range required {
		if value == [32]byte{} {
			return fmt.Errorf("%s is zero", name)
		}
	}
	if identity.ActivationManifestSequence == 0 ||
		identity.WitnessMaximumRecords < 2 ||
		identity.WitnessMaximumRecords > 1_000_000 ||
		identity.WitnessRotationThresholdRecords < 2 ||
		identity.WitnessRotationThresholdRecords >
			identity.WitnessMaximumRecords-2 ||
		!frostNativeSignerAnchorCanonicalIdentityString(identity.TrustDomainID, 256) ||
		!frostNativeSignerAnchorCanonicalIdentityString(identity.HistoryStoreID, 256) {
		return fmt.Errorf("anchor identity strings or manifest sequence are invalid")
	}
	if ComputeFrostNativeSignerAnchorStreamID(identity) != identity.StreamID {
		return fmt.Errorf("anchor stream ID does not match its stable identity")
	}
	return nil
}

func frostNativeSignerAnchorCanonicalIdentityString(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func frostNativeSignerAnchorParseUint64(value string) (uint64, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, fmt.Errorf("value is not canonical decimal uint64")
	}
	result, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(result, 10) != value {
		return 0, fmt.Errorf("value is not canonical decimal uint64")
	}
	return result, nil
}

func frostNativeSignerAnchorParseHex32(value string) ([32]byte, error) {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") ||
		value != strings.ToLower(value) {
		return [32]byte{}, fmt.Errorf("value is not canonical lowercase bytes32")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, fmt.Errorf("value is not canonical lowercase bytes32")
	}
	var result [32]byte
	copy(result[:], decoded)
	return result, nil
}

func frostNativeSignerAnchorHex32(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

func frostNativeSignerAnchorParseSignature(value string) ([ed25519.SignatureSize]byte, error) {
	if len(value) != 2+2*ed25519.SignatureSize ||
		!strings.HasPrefix(value, "0x") ||
		value != strings.ToLower(value) {
		return [ed25519.SignatureSize]byte{}, fmt.Errorf("signature is not canonical lowercase bytes64")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != ed25519.SignatureSize {
		return [ed25519.SignatureSize]byte{}, fmt.Errorf("signature is not canonical lowercase bytes64")
	}
	var result [ed25519.SignatureSize]byte
	copy(result[:], decoded)
	return result, nil
}

func frostNativeSignerAnchorSignatureHex(value []byte) string {
	return "0x" + hex.EncodeToString(value)
}

func frostNativeSignerAnchorCanonicalSPKI(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, fmt.Errorf("SPKI is not canonical base64")
	}
	return decoded, nil
}

func decodeStrictFrostNativeSignerAnchorJSON(data []byte, target interface{}) error {
	if err := preflightFrostNativeSignerAnchorJSON(data); err != nil {
		return err
	}
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

var frostNativeSignerAnchorJSONMembers = map[string]struct{}{
	"schema":                          {},
	"payload":                         {},
	"clientPublicKeySpki":             {},
	"signature":                       {},
	"kind":                            {},
	"nonce":                           {},
	"bindingHash":                     {},
	"identity":                        {},
	"protocolID":                      {},
	"streamID":                        {},
	"activationManifestHash":          {},
	"activationManifestSequence":      {},
	"trustDomainID":                   {},
	"endpointLeafSpkiHash":            {},
	"onlineKeyHash":                   {},
	"operatorFingerprint":             {},
	"historyStoreID":                  {},
	"historyStoreFingerprint":         {},
	"historyClusterFingerprint":       {},
	"offlineAuthorityHash":            {},
	"clientSpkiHash":                  {},
	"signerStoreFingerprint":          {},
	"transportBinding":                {},
	"witnessMaximumRecords":           {},
	"witnessRotationThresholdRecords": {},
	"operationID":                     {},
	"transitionDigest":                {},
	"expected":                        {},
	"candidate":                       {},
	"proof":                           {},
	"storeFingerprint":                {},
	"generation":                      {},
	"previousStateCommitment":         {},
	"stateImageDigest":                {},
	"stateCommitment":                 {},
	"requestDigest":                   {},
	"status":                          {},
	"serviceEpoch":                    {},
	"revision":                        {},
	"previousEventRoot":               {},
	"eventRoot":                       {},
	"checkpoint":                      {},
	"committedAtUnixMs":               {},
	"expiresAtUnixMs":                 {},
	"checkpointAck":                   {},
	"checkpointAckDigest":             {},
	"floorRef":                        {},
	"targetRef":                       {},
	"startRevision":                   {},
	"maximumEvents":                   {},
	"maximumProofEntries":             {},
	"nextRevision":                    {},
	"eventCount":                      {},
	"proofEntryCount":                 {},
	"events":                          {},
	"witnessProof":                    {},
}

func preflightFrostNativeSignerAnchorJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanFrostNativeSignerAnchorJSONValue(decoder, 0); err != nil {
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

func scanFrostNativeSignerAnchorJSONValue(
	decoder *json.Decoder,
	depth int,
) error {
	if depth > frostNativeSignerAnchorMaximumJSONDepth {
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
		seen := make(map[string]struct{})
		seenFolded := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("invalid JSON object member: %w", err)
			}
			key, ok := keyToken.(string)
			if !ok || !frostNativeSignerAnchorASCIIJSONMember(key) {
				return fmt.Errorf("JSON object member name is not canonical ASCII")
			}
			if _, allowed := frostNativeSignerAnchorJSONMembers[key]; !allowed {
				return fmt.Errorf("JSON object member name [%s] is not exact", key)
			}
			folded := strings.ToLower(key)
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("JSON object contains duplicate member [%s]", key)
			}
			if _, duplicate := seenFolded[folded]; duplicate {
				return fmt.Errorf("JSON object contains case-folded duplicate member [%s]", key)
			}
			seen[key] = struct{}{}
			seenFolded[folded] = struct{}{}
			if err := scanFrostNativeSignerAnchorJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("invalid JSON object termination")
		}
	case '[':
		for decoder.More() {
			if err := scanFrostNativeSignerAnchorJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("invalid JSON array termination")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	return nil
}

func frostNativeSignerAnchorASCIIJSONMember(value string) bool {
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

type frostNativeSignerAnchorTranscript struct {
	buffer bytes.Buffer
}

func newFrostNativeSignerAnchorTranscript(domain string) *frostNativeSignerAnchorTranscript {
	result := &frostNativeSignerAnchorTranscript{}
	result.field("domain", []byte(domain))
	return result
}

func (transcript *frostNativeSignerAnchorTranscript) field(name string, value []byte) {
	frostNativeSignerAnchorWriteLengthPrefixed(&transcript.buffer, []byte(name))
	frostNativeSignerAnchorWriteLengthPrefixed(&transcript.buffer, value)
}

func (transcript *frostNativeSignerAnchorTranscript) string(name string, value string) {
	transcript.field(name, []byte(value))
}

func (transcript *frostNativeSignerAnchorTranscript) bytes32(name string, value [32]byte) {
	transcript.field(name, value[:])
}

func (transcript *frostNativeSignerAnchorTranscript) uint64(name string, value uint64) {
	transcript.string(name, strconv.FormatUint(value, 10))
}

func (transcript *frostNativeSignerAnchorTranscript) bytes() []byte {
	return transcript.buffer.Bytes()
}

func frostNativeSignerAnchorWriteLengthPrefixed(writer io.Writer, value []byte) {
	// Transcript fields are bounded well below uint32 by construction.
	_ = binary.Write(writer, binary.BigEndian, uint32(len(value)))
	_, _ = writer.Write(value)
}

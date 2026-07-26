package signing

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"strings"
)

const (
	NativeTBTCSignerRetainedKeyPackageInventorySchema = "tbtc-signer-retained-key-package-inventory/v1"
	NativeTBTCSignerStateWitnessProofRequestSchema    = "tbtc-signer-state-witness-proof-request/v1"
	NativeTBTCSignerStateWitnessProofSchema           = "tbtc-signer-state-witness-proof/v1"

	NativeTBTCSignerStateWitnessProofMaximumEntries uint16 = 256

	nativeTBTCSignerRetainedKeyPackageInventoryCommitmentDomain = "tbtc-signer-retained-key-package-inventory-commitment-v1\x00"
	nativeTBTCSignerStateWitnessGenesisDomain                   = "tbtc-signer-state-witness-genesis-v2\x00"
	nativeTBTCSignerStateWitnessCommitmentDomain                = "tbtc-signer-state-witness-commitment-v2\x00"
)

// NativeTBTCSignerRetainedKeyPackage identifies one locally held key package.
// KeyPackageCommitment binds only canonical public package material. Secret
// signing-share bytes must never cross the native ABI.
type NativeTBTCSignerRetainedKeyPackage struct {
	ParticipantSeat      uint16
	KeyPackageCommitment [32]byte
}

// NativeTBTCSignerRetainedKeyGroup is the exact native key-package inventory
// for one wallet. ShareEpoch is independent of the legacy refresh telemetry:
// it advances only when the signer atomically replaces the real FROST key and
// public packages. The current protocol has no such replacement and therefore
// requires epoch zero.
type NativeTBTCSignerRetainedKeyGroup struct {
	WalletID                   [32]byte
	KeyGroup                   string
	Threshold                  uint16
	ParticipantCount           uint16
	ShareEpoch                 uint64
	PublicKeyPackageCommitment [32]byte
	KeyPackages                []NativeTBTCSignerRetainedKeyPackage
}

// NativeTBTCSignerRetainedKeyPackageInventory is a descriptor-locked snapshot
// of the native signer. The state witness covers every durable engine-state
// mutation, including replay markers; InventoryCommitment covers the sorted
// public key-package inventory alone.
type NativeTBTCSignerRetainedKeyPackageInventory struct {
	Schema                  string
	StoreFingerprint        [32]byte
	StateGeneration         uint64
	StateCommitment         [32]byte
	PreviousStateCommitment [32]byte
	StateImageDigest        [32]byte
	InventoryCommitment     [32]byte
	Entries                 []NativeTBTCSignerRetainedKeyGroup
}

type nativeTBTCSignerRetainedKeyPackageWire struct {
	ParticipantSeat      uint16 `json:"participantSeat"`
	KeyPackageCommitment string `json:"keyPackageCommitment"`
}

type nativeTBTCSignerRetainedKeyGroupWire struct {
	WalletID                   string                                   `json:"walletID"`
	KeyGroup                   string                                   `json:"keyGroup"`
	Threshold                  uint16                                   `json:"threshold"`
	ParticipantCount           uint16                                   `json:"participantCount"`
	ShareEpoch                 *uint64                                  `json:"shareEpoch"`
	PublicKeyPackageCommitment string                                   `json:"publicKeyPackageCommitment"`
	KeyPackages                []nativeTBTCSignerRetainedKeyPackageWire `json:"keyPackages"`
}

type nativeTBTCSignerRetainedKeyPackageInventoryWire struct {
	Schema                  string                                  `json:"schema"`
	StoreFingerprint        string                                  `json:"storeFingerprint"`
	StateGeneration         uint64                                  `json:"stateGeneration"`
	StateCommitment         string                                  `json:"stateCommitment"`
	PreviousStateCommitment string                                  `json:"previousStateCommitment"`
	StateImageDigest        string                                  `json:"stateImageDigest"`
	InventoryCommitment     string                                  `json:"inventoryCommitment"`
	Entries                 *[]nativeTBTCSignerRetainedKeyGroupWire `json:"entries"`
}

// DecodeNativeTBTCSignerRetainedKeyPackageInventory validates the exact wire
// contract returned by frost_tbtc_retained_key_package_inventory.
func DecodeNativeTBTCSignerRetainedKeyPackageInventory(
	payload []byte,
) (*NativeTBTCSignerRetainedKeyPackageInventory, error) {
	wire := &nativeTBTCSignerRetainedKeyPackageInventoryWire{}
	if err := decodeStrictNativeTBTCSignerJSON(payload, wire, "retained key-package inventory"); err != nil {
		return nil, err
	}
	if wire.Schema != NativeTBTCSignerRetainedKeyPackageInventorySchema {
		return nil, fmt.Errorf("unsupported retained key-package inventory schema")
	}
	if wire.StateGeneration == 0 {
		return nil, fmt.Errorf("retained key-package inventory state generation is zero")
	}

	result := &NativeTBTCSignerRetainedKeyPackageInventory{
		Schema:          wire.Schema,
		StateGeneration: wire.StateGeneration,
	}
	if wire.Entries == nil {
		return nil, fmt.Errorf("retained key-package inventory entries are missing")
	}
	result.Entries = make([]NativeTBTCSignerRetainedKeyGroup, len(*wire.Entries))
	bytes32Values := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"store fingerprint", wire.StoreFingerprint, &result.StoreFingerprint},
		{"state commitment", wire.StateCommitment, &result.StateCommitment},
		{"previous state commitment", wire.PreviousStateCommitment, &result.PreviousStateCommitment},
		{"state image digest", wire.StateImageDigest, &result.StateImageDigest},
		{"inventory commitment", wire.InventoryCommitment, &result.InventoryCommitment},
	}
	for _, value := range bytes32Values {
		decoded, err := decodeNativeTBTCSignerStoreBytes32(value.encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid retained key-package %s: %w", value.label, err)
		}
		*value.destination = decoded
	}
	computedStateCommitment := ComputeNativeTBTCSignerStateWitnessCommitment(
		result.StoreFingerprint,
		result.StateGeneration,
		result.PreviousStateCommitment,
		result.StateImageDigest,
	)
	if computedStateCommitment != result.StateCommitment {
		return nil, fmt.Errorf("retained key-package state commitment mismatch")
	}

	var previousWalletID [32]byte
	for entryIndex, entryWire := range *wire.Entries {
		entry := &result.Entries[entryIndex]
		walletID, err := decodeNativeTBTCSignerStoreBytes32(entryWire.WalletID)
		if err != nil {
			return nil, fmt.Errorf("invalid retained key-package wallet ID: %w", err)
		}
		if entryIndex > 0 && bytes.Compare(previousWalletID[:], walletID[:]) >= 0 {
			return nil, fmt.Errorf("retained key-package wallet entries are not strictly sorted")
		}
		previousWalletID = walletID
		entry.WalletID = walletID

		if entryWire.KeyGroup != strings.ToLower(entryWire.KeyGroup) ||
			(len(entryWire.KeyGroup) != 64 && len(entryWire.KeyGroup) != 66) ||
			strings.HasPrefix(entryWire.KeyGroup, "0x") {
			return nil, fmt.Errorf(
				"retained key group is not canonical lowercase x-only or compressed SEC1 hex",
			)
		}
		outputKey, err := TaprootOutputKeyFromTBTCSignerKey(entryWire.KeyGroup)
		if err != nil || len(outputKey) != len(walletID) {
			return nil, fmt.Errorf(
				"retained key group is not canonical lowercase x-only or compressed SEC1 hex",
			)
		}
		if !bytes.Equal(outputKey, walletID[:]) {
			return nil, fmt.Errorf("retained key group does not identify its wallet")
		}
		// Keep the exact Rust key-group handle. It is part of the inventory
		// commitment and is also the lookup key used by interactive signing; the
		// x-only projection above is only the canonical wallet identity.
		entry.KeyGroup = entryWire.KeyGroup
		entry.Threshold = entryWire.Threshold
		entry.ParticipantCount = entryWire.ParticipantCount
		if entryWire.ShareEpoch == nil {
			return nil, fmt.Errorf("retained key group share epoch is missing")
		}
		entry.ShareEpoch = *entryWire.ShareEpoch
		if entry.Threshold == 0 || entry.ParticipantCount == 0 ||
			entry.Threshold > entry.ParticipantCount || entry.ParticipantCount > 100 {
			return nil, fmt.Errorf("retained key group threshold or participant count is invalid")
		}
		publicCommitment, err := decodeNativeTBTCSignerStoreBytes32(
			entryWire.PublicKeyPackageCommitment,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid retained public key-package commitment: %w", err)
		}
		entry.PublicKeyPackageCommitment = publicCommitment
		if len(entryWire.KeyPackages) == 0 || len(entryWire.KeyPackages) > int(entry.ParticipantCount) {
			return nil, fmt.Errorf("retained key-package seat inventory is empty or oversized")
		}
		entry.KeyPackages = make(
			[]NativeTBTCSignerRetainedKeyPackage,
			len(entryWire.KeyPackages),
		)
		var previousSeat uint16
		for packageIndex, packageWire := range entryWire.KeyPackages {
			if packageWire.ParticipantSeat == 0 ||
				packageWire.ParticipantSeat > entry.ParticipantCount ||
				(packageIndex > 0 && packageWire.ParticipantSeat <= previousSeat) {
				return nil, fmt.Errorf("retained key-package seats are invalid or not strictly sorted")
			}
			previousSeat = packageWire.ParticipantSeat
			commitment, err := decodeNativeTBTCSignerStoreBytes32(
				packageWire.KeyPackageCommitment,
			)
			if err != nil {
				return nil, fmt.Errorf("invalid retained key-package commitment: %w", err)
			}
			entry.KeyPackages[packageIndex] = NativeTBTCSignerRetainedKeyPackage{
				ParticipantSeat:      packageWire.ParticipantSeat,
				KeyPackageCommitment: commitment,
			}
		}
	}

	computed := ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(result.Entries)
	if computed != result.InventoryCommitment {
		return nil, fmt.Errorf("retained key-package inventory commitment mismatch")
	}
	return result, nil
}

// ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment implements the
// language-independent commitment transcript used by the native signer.
func ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(
	entries []NativeTBTCSignerRetainedKeyGroup,
) [32]byte {
	digest := sha256.New()
	_, _ = digest.Write(
		[]byte(nativeTBTCSignerRetainedKeyPackageInventoryCommitmentDomain),
	)
	writeNativeTBTCSignerUint32(digest, uint32(len(entries)))
	for _, entry := range entries {
		_, _ = digest.Write(entry.WalletID[:])
		writeNativeTBTCSignerStoreFingerprintField(digest, []byte(entry.KeyGroup))
		writeNativeTBTCSignerUint16(digest, entry.Threshold)
		writeNativeTBTCSignerUint16(digest, entry.ParticipantCount)
		writeNativeTBTCSignerUint64(digest, entry.ShareEpoch)
		_, _ = digest.Write(entry.PublicKeyPackageCommitment[:])
		writeNativeTBTCSignerUint32(digest, uint32(len(entry.KeyPackages)))
		for _, keyPackage := range entry.KeyPackages {
			writeNativeTBTCSignerUint16(digest, keyPackage.ParticipantSeat)
			_, _ = digest.Write(keyPackage.KeyPackageCommitment[:])
		}
	}
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

// ComputeNativeTBTCSignerStateWitnessGenesis derives the root preceding a
// store's first state-witness record. It is exported so independent anchor
// implementations and cross-language tests can reproduce the Rust transcript.
func ComputeNativeTBTCSignerStateWitnessGenesis(
	storeFingerprint [32]byte,
) [32]byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(nativeTBTCSignerStateWitnessGenesisDomain))
	_, _ = digest.Write(storeFingerprint[:])
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

// ComputeNativeTBTCSignerStateWitnessCommitment binds one durable signer-state
// image to its store, generation, and direct hash-chain parent.
func ComputeNativeTBTCSignerStateWitnessCommitment(
	storeFingerprint [32]byte,
	generation uint64,
	previousStateCommitment [32]byte,
	stateImageDigest [32]byte,
) [32]byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(nativeTBTCSignerStateWitnessCommitmentDomain))
	_, _ = digest.Write(storeFingerprint[:])
	writeNativeTBTCSignerUint64(digest, generation)
	_, _ = digest.Write(previousStateCommitment[:])
	_, _ = digest.Write(stateImageDigest[:])
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

type NativeTBTCSignerStateWitnessProofRequest struct {
	Schema             string
	StoreFingerprint   [32]byte
	AncestorGeneration uint64
	AncestorCommitment [32]byte
	TargetGeneration   uint64
	TargetCommitment   [32]byte
	MaximumEntries     uint16
}

type NativeTBTCSignerStateWitnessProofEntry struct {
	Generation              uint64
	PreviousStateCommitment [32]byte
	StateCommitment         [32]byte
	StateImageDigest        [32]byte
}

type NativeTBTCSignerStateWitnessProof struct {
	Schema             string
	StoreFingerprint   [32]byte
	AncestorGeneration uint64
	AncestorCommitment [32]byte
	TargetGeneration   uint64
	TargetCommitment   [32]byte
	Complete           bool
	Entries            []NativeTBTCSignerStateWitnessProofEntry
}

type nativeTBTCSignerStateWitnessProofRequestWire struct {
	Schema             string `json:"schema"`
	StoreFingerprint   string `json:"storeFingerprint"`
	AncestorGeneration uint64 `json:"ancestorGeneration"`
	AncestorCommitment string `json:"ancestorCommitment"`
	TargetGeneration   uint64 `json:"targetGeneration"`
	TargetCommitment   string `json:"targetCommitment"`
	MaximumEntries     uint16 `json:"maximumEntries"`
}

type nativeTBTCSignerStateWitnessProofEntryWire struct {
	Generation              uint64 `json:"generation"`
	PreviousStateCommitment string `json:"previousStateCommitment"`
	StateCommitment         string `json:"stateCommitment"`
	StateImageDigest        string `json:"stateImageDigest"`
}

type nativeTBTCSignerStateWitnessProofWire struct {
	Schema             string                                        `json:"schema"`
	StoreFingerprint   string                                        `json:"storeFingerprint"`
	AncestorGeneration uint64                                        `json:"ancestorGeneration"`
	AncestorCommitment string                                        `json:"ancestorCommitment"`
	TargetGeneration   uint64                                        `json:"targetGeneration"`
	TargetCommitment   string                                        `json:"targetCommitment"`
	Complete           *bool                                         `json:"complete"`
	Entries            *[]nativeTBTCSignerStateWitnessProofEntryWire `json:"entries"`
}

func (request *NativeTBTCSignerStateWitnessProofRequest) MarshalJSON() ([]byte, error) {
	if err := request.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(nativeTBTCSignerStateWitnessProofRequestWire{
		Schema:             request.Schema,
		StoreFingerprint:   nativeTBTCSignerBytes32(request.StoreFingerprint),
		AncestorGeneration: request.AncestorGeneration,
		AncestorCommitment: nativeTBTCSignerBytes32(request.AncestorCommitment),
		TargetGeneration:   request.TargetGeneration,
		TargetCommitment:   nativeTBTCSignerBytes32(request.TargetCommitment),
		MaximumEntries:     request.MaximumEntries,
	})
}

func (request *NativeTBTCSignerStateWitnessProofRequest) validate() error {
	if request == nil || request.Schema != NativeTBTCSignerStateWitnessProofRequestSchema ||
		request.StoreFingerprint == [32]byte{} || request.AncestorGeneration == 0 ||
		request.AncestorCommitment == [32]byte{} || request.TargetGeneration == 0 ||
		request.TargetCommitment == [32]byte{} ||
		request.TargetGeneration < request.AncestorGeneration ||
		request.MaximumEntries == 0 ||
		request.MaximumEntries > NativeTBTCSignerStateWitnessProofMaximumEntries {
		return fmt.Errorf("native signer state-witness proof request is invalid")
	}
	if request.TargetGeneration == request.AncestorGeneration &&
		request.TargetCommitment != request.AncestorCommitment {
		return fmt.Errorf("native signer state-witness equal-generation commitments differ")
	}
	return nil
}

func DecodeNativeTBTCSignerStateWitnessProof(
	payload []byte,
) (*NativeTBTCSignerStateWitnessProof, error) {
	wire := &nativeTBTCSignerStateWitnessProofWire{}
	if err := decodeStrictNativeTBTCSignerJSON(payload, wire, "state-witness proof"); err != nil {
		return nil, err
	}
	if wire.Schema != NativeTBTCSignerStateWitnessProofSchema {
		return nil, fmt.Errorf("unsupported native signer state-witness proof schema")
	}
	result := &NativeTBTCSignerStateWitnessProof{
		Schema:             wire.Schema,
		AncestorGeneration: wire.AncestorGeneration,
		TargetGeneration:   wire.TargetGeneration,
	}
	if wire.Complete == nil || wire.Entries == nil {
		return nil, fmt.Errorf("native signer state-witness proof completeness or entries are missing")
	}
	result.Complete = *wire.Complete
	result.Entries = make(
		[]NativeTBTCSignerStateWitnessProofEntry,
		len(*wire.Entries),
	)
	values := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"store fingerprint", wire.StoreFingerprint, &result.StoreFingerprint},
		{"ancestor commitment", wire.AncestorCommitment, &result.AncestorCommitment},
		{"target commitment", wire.TargetCommitment, &result.TargetCommitment},
	}
	for _, value := range values {
		decoded, err := decodeNativeTBTCSignerStoreBytes32(value.encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid state-witness %s: %w", value.label, err)
		}
		*value.destination = decoded
	}
	if result.AncestorGeneration == 0 || result.TargetGeneration == 0 ||
		result.TargetGeneration < result.AncestorGeneration ||
		len(result.Entries) > int(NativeTBTCSignerStateWitnessProofMaximumEntries) {
		return nil, fmt.Errorf("native signer state-witness proof bounds are invalid")
	}

	previousGeneration := result.AncestorGeneration
	previousCommitment := result.AncestorCommitment
	for index, entryWire := range *wire.Entries {
		entry := &result.Entries[index]
		entry.Generation = entryWire.Generation
		entryValues := []struct {
			label       string
			encoded     string
			destination *[32]byte
		}{
			{"previous state commitment", entryWire.PreviousStateCommitment, &entry.PreviousStateCommitment},
			{"state commitment", entryWire.StateCommitment, &entry.StateCommitment},
			{"state image digest", entryWire.StateImageDigest, &entry.StateImageDigest},
		}
		for _, value := range entryValues {
			decoded, err := decodeNativeTBTCSignerStoreBytes32(value.encoded)
			if err != nil {
				return nil, fmt.Errorf("invalid state-witness proof entry %s: %w", value.label, err)
			}
			*value.destination = decoded
		}
		if entry.Generation != previousGeneration+1 ||
			entry.PreviousStateCommitment != previousCommitment ||
			entry.Generation > result.TargetGeneration {
			return nil, fmt.Errorf("native signer state-witness proof is not a contiguous chain")
		}
		computed := ComputeNativeTBTCSignerStateWitnessCommitment(
			result.StoreFingerprint,
			entry.Generation,
			entry.PreviousStateCommitment,
			entry.StateImageDigest,
		)
		if computed != entry.StateCommitment {
			return nil, fmt.Errorf("native signer state-witness proof commitment mismatch")
		}
		previousGeneration = entry.Generation
		previousCommitment = entry.StateCommitment
	}
	if result.Complete {
		if previousGeneration != result.TargetGeneration ||
			previousCommitment != result.TargetCommitment {
			return nil, fmt.Errorf("complete native signer state-witness proof does not reach its target")
		}
	} else if len(result.Entries) == 0 || previousGeneration >= result.TargetGeneration {
		return nil, fmt.Errorf("incomplete native signer state-witness proof made no bounded progress")
	}
	return result, nil
}

func decodeStrictNativeTBTCSignerJSON(payload []byte, target interface{}, subject string) error {
	if err := preflightStrictNativeTBTCSignerJSON(payload, 0); err != nil {
		return fmt.Errorf("cannot decode native signer %s: %w", subject, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("cannot decode native signer %s: %w", subject, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("native signer %s contains trailing JSON", subject)
		}
		return fmt.Errorf("cannot decode native signer %s trailing data: %w", subject, err)
	}
	return nil
}

func preflightStrictNativeTBTCSignerJSON(payload []byte, depth int) error {
	const maximumDepth = 32
	if depth != 0 {
		return fmt.Errorf("native signer JSON preflight must start at the root")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var scanValue func(int) error
	scanValue = func(currentDepth int) error {
		if currentDepth > maximumDepth {
			return fmt.Errorf("JSON nesting exceeds the depth bound")
		}
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
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
				if !ok || key == "" {
					return fmt.Errorf("JSON object member name is invalid")
				}
				for _, character := range key {
					if character < 0x21 || character > 0x7e {
						return fmt.Errorf(
							"JSON object member name [%s] is not canonical ASCII",
							key,
						)
					}
				}
				folded := strings.ToLower(key)
				if _, exists := seen[key]; exists {
					return fmt.Errorf("JSON object contains duplicate member [%s]", key)
				}
				if _, exists := seenFolded[folded]; exists {
					return fmt.Errorf(
						"JSON object contains case-folded duplicate member [%s]",
						key,
					)
				}
				seen[key] = struct{}{}
				seenFolded[folded] = struct{}{}
				if err := scanValue(currentDepth + 1); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return fmt.Errorf("invalid JSON object termination")
			}
		case '[':
			for decoder.More() {
				if err := scanValue(currentDepth + 1); err != nil {
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
	if err := scanValue(0); err != nil {
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

func nativeTBTCSignerBytes32(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

func writeNativeTBTCSignerUint16(destination hash.Hash, value uint16) {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], value)
	_, _ = destination.Write(encoded[:])
}

func writeNativeTBTCSignerUint32(destination hash.Hash, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	_, _ = destination.Write(encoded[:])
}

func writeNativeTBTCSignerUint64(destination hash.Hash, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = destination.Write(encoded[:])
}

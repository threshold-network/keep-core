package signing

import (
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const NativeTBTCSignerStateWitnessTipSchema = "tbtc-signer-state-witness-tip/v1"

const NativeTBTCSignerStateWitnessCheckpointAcknowledgementResultSchema = "tbtc-signer-state-witness-checkpoint-ack-result/v1"
const NativeTBTCSignerStateWitnessCheckpointRecoveryResultSchema = "tbtc-signer-state-witness-checkpoint-recovery-result/v1"

// NativeTBTCSignerStateWitnessTip is the constant-size durable state readback
// used around every request-taking native signer call. The five Anchor fields
// are either all zero (no remote acknowledgement has been installed in Rust
// yet) or all non-zero (the last acknowledgement Rust durably bound to the
// store). WitnessBase identifies the oldest proofable record after any
// authenticated history rotation.
type NativeTBTCSignerStateWitnessTip struct {
	Schema                      string
	StoreFingerprint            [32]byte
	Generation                  uint64
	PreviousStateCommitment     [32]byte
	StateImageDigest            [32]byte
	StateCommitment             [32]byte
	WitnessBaseGeneration       uint64
	WitnessBaseCommitment       [32]byte
	AnchorBindingHash           [32]byte
	AnchorServiceEpoch          uint64
	AnchorRevision              uint64
	AnchorEventRoot             [32]byte
	AnchorAcknowledgementDigest [32]byte
}

type nativeTBTCSignerStateWitnessTipWire struct {
	Schema                      string `json:"schema"`
	StoreFingerprint            string `json:"storeFingerprint"`
	Generation                  string `json:"generation"`
	PreviousStateCommitment     string `json:"previousStateCommitment"`
	StateImageDigest            string `json:"stateImageDigest"`
	StateCommitment             string `json:"stateCommitment"`
	WitnessBaseGeneration       string `json:"witnessBaseGeneration"`
	WitnessBaseCommitment       string `json:"witnessBaseCommitment"`
	AnchorBindingHash           string `json:"anchorBindingHash"`
	AnchorServiceEpoch          string `json:"anchorServiceEpoch"`
	AnchorRevision              string `json:"anchorRevision"`
	AnchorEventRoot             string `json:"anchorEventRoot"`
	AnchorAcknowledgementDigest string `json:"anchorAcknowledgementDigest"`
}

// NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult confirms the
// exact remote acknowledgement Rust durably installed in its descriptor-bound
// .state-anchor metadata. Installing it never changes the EngineState
// checkpoint five-tuple.
type NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult struct {
	Schema                      string
	Acknowledged                bool
	Idempotent                  bool
	Rotated                     bool
	StoreFingerprint            [32]byte
	Generation                  uint64
	StateCommitment             [32]byte
	WitnessBaseGeneration       uint64
	WitnessBaseCommitment       [32]byte
	AnchorServiceEpoch          uint64
	AnchorServiceRevision       uint64
	AnchorEventRoot             [32]byte
	AnchorAcknowledgementDigest [32]byte
}

type nativeTBTCSignerStateWitnessCheckpointAcknowledgementResultWire struct {
	Schema                      string `json:"schema"`
	Acknowledged                *bool  `json:"acknowledged"`
	Idempotent                  *bool  `json:"idempotent"`
	Rotated                     *bool  `json:"rotated"`
	StoreFingerprint            string `json:"storeFingerprint"`
	Generation                  string `json:"generation"`
	StateCommitment             string `json:"stateCommitment"`
	WitnessBaseGeneration       string `json:"witnessBaseGeneration"`
	WitnessBaseCommitment       string `json:"witnessBaseCommitment"`
	AnchorServiceEpoch          string `json:"anchorServiceEpoch"`
	AnchorServiceRevision       string `json:"anchorServiceRevision"`
	AnchorEventRoot             string `json:"anchorEventRoot"`
	AnchorAcknowledgementDigest string `json:"anchorAcknowledgementDigest"`
}

type NativeTBTCSignerStateWitnessCheckpointRecoveryResult struct {
	Schema                      string
	Recovered                   bool
	Idempotent                  bool
	Rotated                     bool
	StoreFingerprint            [32]byte
	Generation                  uint64
	StateCommitment             [32]byte
	WitnessBaseGeneration       uint64
	WitnessBaseCommitment       [32]byte
	AnchorServiceEpoch          uint64
	AnchorServiceRevision       uint64
	AnchorEventRoot             [32]byte
	AnchorAcknowledgementDigest [32]byte
}

type nativeTBTCSignerStateWitnessCheckpointRecoveryResultWire struct {
	Schema                      string `json:"schema"`
	Recovered                   *bool  `json:"recovered"`
	Idempotent                  *bool  `json:"idempotent"`
	Rotated                     *bool  `json:"rotated"`
	StoreFingerprint            string `json:"storeFingerprint"`
	Generation                  string `json:"generation"`
	StateCommitment             string `json:"stateCommitment"`
	WitnessBaseGeneration       string `json:"witnessBaseGeneration"`
	WitnessBaseCommitment       string `json:"witnessBaseCommitment"`
	AnchorServiceEpoch          string `json:"anchorServiceEpoch"`
	AnchorServiceRevision       string `json:"anchorServiceRevision"`
	AnchorEventRoot             string `json:"anchorEventRoot"`
	AnchorAcknowledgementDigest string `json:"anchorAcknowledgementDigest"`
}

// DecodeNativeTBTCSignerStateWitnessTip validates the exact wire contract
// returned by frost_tbtc_state_witness_tip. Decimal integers deliberately use
// JSON strings so every implementation must agree on the full uint64 range.
func DecodeNativeTBTCSignerStateWitnessTip(
	payload []byte,
) (*NativeTBTCSignerStateWitnessTip, error) {
	wire := &nativeTBTCSignerStateWitnessTipWire{}
	if err := decodeStrictNativeTBTCSignerJSON(
		payload,
		wire,
		"state-witness tip",
	); err != nil {
		return nil, err
	}
	if wire.Schema != NativeTBTCSignerStateWitnessTipSchema {
		return nil, fmt.Errorf("unsupported native signer state-witness tip schema")
	}

	generation, err := decodeNativeTBTCSignerCanonicalUint64(wire.Generation)
	if err != nil || generation == 0 {
		return nil, fmt.Errorf("invalid native signer state-witness tip generation")
	}
	witnessBaseGeneration, err := decodeNativeTBTCSignerCanonicalUint64(
		wire.WitnessBaseGeneration,
	)
	if err != nil || witnessBaseGeneration == 0 ||
		witnessBaseGeneration > generation {
		return nil, fmt.Errorf("invalid native signer state-witness base generation")
	}
	anchorServiceEpoch, err := decodeNativeTBTCSignerCanonicalUint64(
		wire.AnchorServiceEpoch,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid native signer anchor service epoch")
	}
	anchorRevision, err := decodeNativeTBTCSignerCanonicalUint64(
		wire.AnchorRevision,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid native signer anchor revision")
	}

	result := &NativeTBTCSignerStateWitnessTip{
		Schema:                wire.Schema,
		Generation:            generation,
		WitnessBaseGeneration: witnessBaseGeneration,
		AnchorServiceEpoch:    anchorServiceEpoch,
		AnchorRevision:        anchorRevision,
	}
	requiredBytes32 := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"store fingerprint", wire.StoreFingerprint, &result.StoreFingerprint},
		{"previous state commitment", wire.PreviousStateCommitment, &result.PreviousStateCommitment},
		{"state image digest", wire.StateImageDigest, &result.StateImageDigest},
		{"state commitment", wire.StateCommitment, &result.StateCommitment},
		{"witness base commitment", wire.WitnessBaseCommitment, &result.WitnessBaseCommitment},
	}
	for _, value := range requiredBytes32 {
		decoded, err := decodeNativeTBTCSignerCanonicalBytes32(value.encoded, false)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer state-witness tip %s: %w",
				value.label,
				err,
			)
		}
		*value.destination = decoded
	}
	optionalBytes32 := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"anchor binding hash", wire.AnchorBindingHash, &result.AnchorBindingHash},
		{"anchor event root", wire.AnchorEventRoot, &result.AnchorEventRoot},
		{"anchor acknowledgement digest", wire.AnchorAcknowledgementDigest, &result.AnchorAcknowledgementDigest},
	}
	for _, value := range optionalBytes32 {
		decoded, err := decodeNativeTBTCSignerCanonicalBytes32(value.encoded, true)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer state-witness tip %s: %w",
				value.label,
				err,
			)
		}
		*value.destination = decoded
	}

	computed := ComputeNativeTBTCSignerStateWitnessCommitment(
		result.StoreFingerprint,
		result.Generation,
		result.PreviousStateCommitment,
		result.StateImageDigest,
	)
	if computed != result.StateCommitment {
		return nil, fmt.Errorf("native signer state-witness tip commitment mismatch")
	}
	if result.WitnessBaseGeneration == result.Generation &&
		result.WitnessBaseCommitment != result.StateCommitment {
		return nil, fmt.Errorf(
			"native signer state-witness base at the tip has a different commitment",
		)
	}

	hasAnchorHash := result.AnchorBindingHash != [32]byte{}
	hasAnchorEpoch := result.AnchorServiceEpoch != 0
	hasAnchorRevision := result.AnchorRevision != 0
	hasAnchorRoot := result.AnchorEventRoot != [32]byte{}
	hasAnchorAcknowledgement := result.AnchorAcknowledgementDigest != [32]byte{}
	if hasAnchorHash != hasAnchorEpoch ||
		hasAnchorHash != hasAnchorRevision ||
		hasAnchorHash != hasAnchorRoot ||
		hasAnchorHash != hasAnchorAcknowledgement {
		return nil, fmt.Errorf(
			"native signer state-witness anchor metadata is not all-zero or all-nonzero",
		)
	}

	return result, nil
}

// DecodeNativeTBTCSignerStateWitnessCheckpointAcknowledgementResult validates
// the exact response from frost_tbtc_acknowledge_state_witness_checkpoint.
func DecodeNativeTBTCSignerStateWitnessCheckpointAcknowledgementResult(
	payload []byte,
) (*NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult, error) {
	wire := &nativeTBTCSignerStateWitnessCheckpointAcknowledgementResultWire{}
	if err := decodeStrictNativeTBTCSignerJSON(
		payload,
		wire,
		"state-witness checkpoint acknowledgement result",
	); err != nil {
		return nil, err
	}
	if wire.Schema !=
		NativeTBTCSignerStateWitnessCheckpointAcknowledgementResultSchema {
		return nil, fmt.Errorf(
			"unsupported native signer state-witness acknowledgement result schema",
		)
	}
	if wire.Acknowledged == nil || wire.Idempotent == nil || wire.Rotated == nil {
		return nil, fmt.Errorf(
			"native signer state-witness acknowledgement flags are missing",
		)
	}

	result := &NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult{
		Schema:       wire.Schema,
		Acknowledged: *wire.Acknowledged,
		Idempotent:   *wire.Idempotent,
		Rotated:      *wire.Rotated,
	}
	decimalFields := []struct {
		label       string
		encoded     string
		destination *uint64
	}{
		{"generation", wire.Generation, &result.Generation},
		{"witness base generation", wire.WitnessBaseGeneration, &result.WitnessBaseGeneration},
		{"anchor service epoch", wire.AnchorServiceEpoch, &result.AnchorServiceEpoch},
		{"anchor service revision", wire.AnchorServiceRevision, &result.AnchorServiceRevision},
	}
	for _, field := range decimalFields {
		decoded, err := decodeNativeTBTCSignerCanonicalUint64(field.encoded)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer acknowledgement %s: %w",
				field.label,
				err,
			)
		}
		*field.destination = decoded
	}
	bytes32Fields := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"store fingerprint", wire.StoreFingerprint, &result.StoreFingerprint},
		{"state commitment", wire.StateCommitment, &result.StateCommitment},
		{"witness base commitment", wire.WitnessBaseCommitment, &result.WitnessBaseCommitment},
		{"anchor event root", wire.AnchorEventRoot, &result.AnchorEventRoot},
		{"anchor acknowledgement digest", wire.AnchorAcknowledgementDigest, &result.AnchorAcknowledgementDigest},
	}
	for _, field := range bytes32Fields {
		decoded, err := decodeNativeTBTCSignerCanonicalBytes32(field.encoded, false)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer acknowledgement %s: %w",
				field.label,
				err,
			)
		}
		*field.destination = decoded
	}
	if !result.Acknowledged || result.Generation == 0 ||
		result.WitnessBaseGeneration == 0 ||
		result.WitnessBaseGeneration > result.Generation ||
		result.AnchorServiceEpoch == 0 || result.AnchorServiceRevision == 0 {
		return nil, fmt.Errorf(
			"native signer state-witness acknowledgement result is incomplete",
		)
	}
	if result.WitnessBaseGeneration == result.Generation &&
		result.WitnessBaseCommitment != result.StateCommitment {
		return nil, fmt.Errorf(
			"native signer acknowledgement witness base differs at the tip",
		)
	}
	return result, nil
}

func DecodeNativeTBTCSignerStateWitnessCheckpointRecoveryResult(
	payload []byte,
) (*NativeTBTCSignerStateWitnessCheckpointRecoveryResult, error) {
	wire := &nativeTBTCSignerStateWitnessCheckpointRecoveryResultWire{}
	if err := decodeStrictNativeTBTCSignerJSON(
		payload,
		wire,
		"state-witness checkpoint recovery result",
	); err != nil {
		return nil, err
	}
	if wire.Schema != NativeTBTCSignerStateWitnessCheckpointRecoveryResultSchema ||
		wire.Recovered == nil || wire.Idempotent == nil || wire.Rotated == nil ||
		!*wire.Recovered {
		return nil, fmt.Errorf(
			"native signer state-witness recovery result is incomplete",
		)
	}
	result := &NativeTBTCSignerStateWitnessCheckpointRecoveryResult{
		Schema:     wire.Schema,
		Recovered:  *wire.Recovered,
		Idempotent: *wire.Idempotent,
		Rotated:    *wire.Rotated,
	}
	decimalFields := []struct {
		label       string
		encoded     string
		destination *uint64
	}{
		{"generation", wire.Generation, &result.Generation},
		{"witness base generation", wire.WitnessBaseGeneration, &result.WitnessBaseGeneration},
		{"anchor service epoch", wire.AnchorServiceEpoch, &result.AnchorServiceEpoch},
		{"anchor service revision", wire.AnchorServiceRevision, &result.AnchorServiceRevision},
	}
	for _, field := range decimalFields {
		decoded, err := decodeNativeTBTCSignerCanonicalUint64(field.encoded)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer recovery %s: %w",
				field.label,
				err,
			)
		}
		*field.destination = decoded
	}
	bytes32Fields := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"store fingerprint", wire.StoreFingerprint, &result.StoreFingerprint},
		{"state commitment", wire.StateCommitment, &result.StateCommitment},
		{"witness base commitment", wire.WitnessBaseCommitment, &result.WitnessBaseCommitment},
		{"anchor event root", wire.AnchorEventRoot, &result.AnchorEventRoot},
		{"anchor acknowledgement digest", wire.AnchorAcknowledgementDigest, &result.AnchorAcknowledgementDigest},
	}
	for _, field := range bytes32Fields {
		decoded, err := decodeNativeTBTCSignerCanonicalBytes32(field.encoded, false)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid native signer recovery %s: %w",
				field.label,
				err,
			)
		}
		*field.destination = decoded
	}
	if result.Generation == 0 || result.WitnessBaseGeneration == 0 ||
		result.WitnessBaseGeneration > result.Generation ||
		result.AnchorServiceEpoch == 0 || result.AnchorServiceRevision == 0 ||
		(result.WitnessBaseGeneration == result.Generation &&
			result.WitnessBaseCommitment != result.StateCommitment) {
		return nil, fmt.Errorf(
			"native signer state-witness recovery result has invalid bounds",
		)
	}
	return result, nil
}

func decodeNativeTBTCSignerCanonicalUint64(value string) (uint64, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, fmt.Errorf("expected canonical unsigned decimal")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("expected canonical unsigned decimal")
		}
	}
	if len(value) > len(strconv.FormatUint(math.MaxUint64, 10)) {
		return 0, fmt.Errorf("unsigned decimal overflows uint64")
	}
	result, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unsigned decimal overflows uint64")
	}
	return result, nil
}

func decodeNativeTBTCSignerCanonicalBytes32(
	value string,
	allowZero bool,
) ([32]byte, error) {
	result := [32]byte{}
	if value != strings.ToLower(value) || !strings.HasPrefix(value, "0x") ||
		len(value) != 66 {
		return result, fmt.Errorf("expected canonical lowercase 0x-prefixed bytes32")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != len(result) {
		return result, fmt.Errorf("expected canonical lowercase 0x-prefixed bytes32")
	}
	copy(result[:], decoded)
	if !allowZero && result == [32]byte{} {
		return result, fmt.Errorf("value is zero")
	}
	return result, nil
}

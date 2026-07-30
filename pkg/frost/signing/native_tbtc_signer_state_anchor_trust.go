package signing

import (
	"errors"
	"fmt"
)

const (
	NativeTBTCSignerStateAnchorTrustHeadSchema             = "tbtc-signer-state-anchor-trust-head/v1"
	NativeTBTCSignerStateAnchorTrustTransitionResultSchema = "tbtc-signer-state-anchor-trust-transition-result/v1"
	NativeTBTCSignerStateAnchorTrustRecoveryRequiredSchema = "tbtc-signer-state-anchor-trust-recovery-required/v1"
	// NativeTBTCSignerStateAnchorTrustTransitionMaximumRequestBytes is the
	// shared Go-side admission bound for Rust's startup transition FFI.
	NativeTBTCSignerStateAnchorTrustTransitionMaximumRequestBytes            = 16 * 1024 * 1024
	NativeTBTCSignerStateAnchorTrustTransitionMaximumCertificateCount uint64 = 64

	// NativeTBTCSignerStateWitnessRotationTerminalRecordReservation mirrors the
	// native signer's TBTC_SIGNER_STATE_WITNESS_ROTATION_TERMINAL_RECORD_RESERVATION
	// (pkg/tbtc/signer/src/engine/store.rs). Reconciliation can commit an
	// interrupted write at the rotation threshold before a mutating interactive
	// retry persists two expiry-sweep repairs and its requested mutation; those
	// three snapshots need six PREPARE/COMMIT records to finish the in-flight
	// request before a checkpoint can be acknowledged.
	NativeTBTCSignerStateWitnessRotationTerminalRecordReservation uint64 = 6
	// NativeTBTCSignerStateWitnessQuarantineRecordReservation mirrors the
	// signer's TBTC_SIGNER_STATE_WITNESS_QUARANTINE_RECORD_RESERVATION, the
	// PREPARE/COMMIT pair kept above the terminal band so corruption recovery
	// stays reachable once an interrupted retry has parked the journal at the
	// terminal limit. Quarantine commits an absence, so it never extends usable
	// state, but it still needs two records that ordinary writes cannot consume.
	NativeTBTCSignerStateWitnessQuarantineRecordReservation uint64 = 2
	// NativeTBTCSignerStateWitnessMinimumRotationThresholdRecords mirrors the
	// signer's lower bound on the rotation threshold: the terminal reserve is
	// entered only after at least one complete PREPARE/COMMIT pair.
	NativeTBTCSignerStateWitnessMinimumRotationThresholdRecords uint64 = 2
	// NativeTBTCSignerStateWitnessHardMaximumRecords mirrors the signer's
	// TBTC_SIGNER_HARD_MAX_STATE_WITNESS_MAX_RECORDS.
	NativeTBTCSignerStateWitnessHardMaximumRecords uint64 = 1_000_000
)

// ValidateNativeTBTCSignerStateWitnessGeometry is the single Go copy of the
// witness-record geometry the native signer enforces at both of its own
// intakes: configured_state_anchor behind frost_tbtc_init_signer_config, and
// parse_endpoint behind frost_tbtc_transition_state_witness_anchor. Go mints
// and pre-verifies the offline-authority-signed trust certificates and
// validates the installed init configuration, so a geometry Go accepts must be
// exactly a geometry Rust accepts; anything looser lets an operator complete
// the whole offline ceremony against a plan the signer rejects at node startup,
// which can only be undone by re-running the ceremony.
//
// The bound is deliberately expressed as one helper over the exported
// reservation constants. Every previous copy of this arithmetic drifted
// independently when the reservation grew from two records to six, and the
// quarantine pair added on top of it would have drifted the same way.
func ValidateNativeTBTCSignerStateWitnessGeometry(
	maximumRecords uint64,
	rotationThresholdRecords uint64,
) error {
	if maximumRecords == 0 ||
		maximumRecords > NativeTBTCSignerStateWitnessHardMaximumRecords {
		return fmt.Errorf(
			"witnessMaximumRecords [%d] is outside [1,%d]",
			maximumRecords,
			NativeTBTCSignerStateWitnessHardMaximumRecords,
		)
	}
	reserved := rotationThresholdRecords +
		NativeTBTCSignerStateWitnessRotationTerminalRecordReservation +
		NativeTBTCSignerStateWitnessQuarantineRecordReservation
	if rotationThresholdRecords <
		NativeTBTCSignerStateWitnessMinimumRotationThresholdRecords ||
		reserved < rotationThresholdRecords ||
		reserved > maximumRecords {
		return fmt.Errorf(
			"witnessRotationThresholdRecords [%d] must be at least %d and "+
				"reserve %d records below witnessMaximumRecords [%d]",
			rotationThresholdRecords,
			NativeTBTCSignerStateWitnessMinimumRotationThresholdRecords,
			NativeTBTCSignerStateWitnessRotationTerminalRecordReservation+
				NativeTBTCSignerStateWitnessQuarantineRecordReservation,
			maximumRecords,
		)
	}
	return nil
}

var ErrNativeTBTCSignerStateAnchorTrustHeadAbsent = errors.New(
	"native tbtc signer state-anchor trust head is absent",
)

// NativeTBTCSignerStateAnchorCheckpoint is the complete state commitment
// carried by certified floors and transition readback.
type NativeTBTCSignerStateAnchorCheckpoint struct {
	StoreFingerprint        [32]byte
	Generation              uint64
	PreviousStateCommitment [32]byte
	StateImageDigest        [32]byte
	StateCommitment         [32]byte
}

// NativeTBTCSignerStateAnchorTrustReference identifies one exact signed
// service event. PreviousEventRoot is retained here because revision-one
// rotation certificates are the sole allowed cross-epoch predecessor-root
// exception.
type NativeTBTCSignerStateAnchorTrustReference struct {
	ServiceEpoch          uint64
	Revision              uint64
	PreviousEventRoot     [32]byte
	EventRoot             [32]byte
	AcknowledgementDigest [32]byte
	Checkpoint            NativeTBTCSignerStateAnchorCheckpoint
}

// NativeTBTCSignerStateAnchorTrustHead is the descriptor-bound offline trust
// journal head exposed by frost_tbtc_state_anchor_trust_head. The full
// certificate bytes remain private to the signer journal; this readback
// contains every value Go needs to compare with the verified manifest,
// certificate, service, and installed init configuration.
type NativeTBTCSignerStateAnchorTrustHead struct {
	Schema                          string
	CertificateSequence             uint64
	CertificateDigest               [32]byte
	ActivationManifestSequence      uint64
	ActivationManifestHash          [32]byte
	BindingHash                     [32]byte
	ResponsePublicKeySPKISHA256     [32]byte
	OfflineAuthoritySPKISHA256      [32]byte
	ServiceEpoch                    uint64
	CertifiedFloor                  NativeTBTCSignerStateAnchorTrustReference
	WitnessMaximumRecords           uint64
	WitnessRotationThresholdRecords uint64
}

type NativeTBTCSignerStateAnchorTrustTransitionResult struct {
	Schema                  string
	Installed               bool
	Idempotent              bool
	AppliedCertificateCount uint64
	TrustHead               NativeTBTCSignerStateAnchorTrustHead
	CurrentCheckpoint       NativeTBTCSignerStateAnchorCheckpoint
	WitnessBaseCheckpoint   NativeTBTCSignerStateAnchorCheckpoint
	CurrentAnchorReference  NativeTBTCSignerStateAnchorTrustReference
}

// NativeTBTCSignerStateAnchorTrustRecoveryRequired is an unauthoritative,
// bounded selector for the exact certificate suffix held in a crash-recovery
// intent. Callers must match it against an independently authenticated local
// artifact and obtain a new signed remote Read before retrying.
type NativeTBTCSignerStateAnchorTrustRecoveryRequired struct {
	Schema                    string
	StoreFingerprint          [32]byte
	CertificateCount          uint64
	FirstCertificateSequence  uint64
	OrderedCertificateDigests [][32]byte
	FinalCertificateSequence  uint64
	FinalCertificateDigest    [32]byte
	TargetBindingHash         [32]byte
	TargetServiceEpoch        uint64
	TargetRevision            uint64
	TargetCheckpoint          NativeTBTCSignerStateAnchorCheckpoint
}

// NativeTBTCSignerStateAnchorTrustRecoveryRequiredError preserves the original
// bridge failure while exposing its strictly decoded recovery selector through
// errors.As.
type NativeTBTCSignerStateAnchorTrustRecoveryRequiredError struct {
	Recovery NativeTBTCSignerStateAnchorTrustRecoveryRequired
	cause    error
}

func (e *NativeTBTCSignerStateAnchorTrustRecoveryRequiredError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf(
		"native tbtc signer state-anchor trust recovery is required for certificate suffix [%d..%d]",
		e.Recovery.FirstCertificateSequence,
		e.Recovery.FinalCertificateSequence,
	)
}

func (e *NativeTBTCSignerStateAnchorTrustRecoveryRequiredError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

type nativeTBTCSignerStateAnchorCheckpointWire struct {
	StoreFingerprint        string `json:"storeFingerprint"`
	Generation              string `json:"generation"`
	PreviousStateCommitment string `json:"previousStateCommitment"`
	StateImageDigest        string `json:"stateImageDigest"`
	StateCommitment         string `json:"stateCommitment"`
}

type nativeTBTCSignerStateAnchorTrustRecoveryRequiredWire struct {
	Schema                    string                                    `json:"schema"`
	StoreFingerprint          string                                    `json:"storeFingerprint"`
	CertificateCount          string                                    `json:"certificateCount"`
	FirstCertificateSequence  string                                    `json:"firstCertificateSequence"`
	OrderedCertificateDigests []string                                  `json:"orderedCertificateDigests"`
	FinalCertificateSequence  string                                    `json:"finalCertificateSequence"`
	FinalCertificateDigest    string                                    `json:"finalCertificateDigest"`
	TargetBindingHash         string                                    `json:"targetBindingHash"`
	TargetServiceEpoch        string                                    `json:"targetServiceEpoch"`
	TargetRevision            string                                    `json:"targetRevision"`
	TargetCheckpoint          nativeTBTCSignerStateAnchorCheckpointWire `json:"targetCheckpoint"`
}

type nativeTBTCSignerStateAnchorTrustReferenceWire struct {
	ServiceEpoch        string                                    `json:"serviceEpoch"`
	Revision            string                                    `json:"revision"`
	PreviousEventRoot   string                                    `json:"previousEventRoot"`
	EventRoot           string                                    `json:"eventRoot"`
	CheckpointAckDigest string                                    `json:"checkpointAckDigest"`
	Checkpoint          nativeTBTCSignerStateAnchorCheckpointWire `json:"checkpoint"`
}

type nativeTBTCSignerStateAnchorTrustHeadWire struct {
	Schema                          string                                        `json:"schema"`
	CertificateSequence             string                                        `json:"certificateSequence"`
	CertificateDigest               string                                        `json:"certificateDigest"`
	ActivationManifestSequence      string                                        `json:"activationManifestSequence"`
	ActivationManifestHash          string                                        `json:"activationManifestHash"`
	BindingHash                     string                                        `json:"bindingHash"`
	ResponsePublicKeySPKISHA256     string                                        `json:"responsePublicKeySpkiSha256"`
	OfflineAuthoritySPKISHA256      string                                        `json:"offlineAuthoritySpkiSha256"`
	ServiceEpoch                    string                                        `json:"serviceEpoch"`
	CertifiedFloor                  nativeTBTCSignerStateAnchorTrustReferenceWire `json:"certifiedFloor"`
	WitnessMaximumRecords           string                                        `json:"witnessMaximumRecords"`
	WitnessRotationThresholdRecords string                                        `json:"witnessRotationThresholdRecords"`
}

type nativeTBTCSignerStateAnchorTrustTransitionResultWire struct {
	Schema                  string                                        `json:"schema"`
	Installed               *bool                                         `json:"installed"`
	Idempotent              *bool                                         `json:"idempotent"`
	AppliedCertificateCount string                                        `json:"appliedCertificateCount"`
	TrustHead               nativeTBTCSignerStateAnchorTrustHeadWire      `json:"trustHead"`
	CurrentCheckpoint       nativeTBTCSignerStateAnchorCheckpointWire     `json:"currentCheckpoint"`
	WitnessBaseCheckpoint   nativeTBTCSignerStateAnchorCheckpointWire     `json:"witnessBaseCheckpoint"`
	CurrentAnchorReference  nativeTBTCSignerStateAnchorTrustReferenceWire `json:"currentAnchorReference"`
}

func DecodeNativeTBTCSignerStateAnchorTrustHead(
	payload []byte,
) (*NativeTBTCSignerStateAnchorTrustHead, error) {
	wire := &nativeTBTCSignerStateAnchorTrustHeadWire{}
	if err := decodeStrictNativeTBTCSignerJSON(
		payload,
		wire,
		"state-anchor trust head",
	); err != nil {
		return nil, err
	}
	return decodeNativeTBTCSignerStateAnchorTrustHeadWire(wire)
}

func decodeNativeTBTCSignerStateAnchorTrustHeadWire(
	wire *nativeTBTCSignerStateAnchorTrustHeadWire,
) (*NativeTBTCSignerStateAnchorTrustHead, error) {
	if wire == nil || wire.Schema != NativeTBTCSignerStateAnchorTrustHeadSchema {
		return nil, fmt.Errorf("unsupported native signer state-anchor trust-head schema")
	}
	result := &NativeTBTCSignerStateAnchorTrustHead{Schema: wire.Schema}
	decimalFields := []struct {
		label       string
		encoded     string
		destination *uint64
	}{
		{"certificate sequence", wire.CertificateSequence, &result.CertificateSequence},
		{"activation manifest sequence", wire.ActivationManifestSequence, &result.ActivationManifestSequence},
		{"service epoch", wire.ServiceEpoch, &result.ServiceEpoch},
		{"witness maximum records", wire.WitnessMaximumRecords, &result.WitnessMaximumRecords},
		{"witness rotation threshold", wire.WitnessRotationThresholdRecords, &result.WitnessRotationThresholdRecords},
	}
	for _, field := range decimalFields {
		decoded, err := decodeNativeTBTCSignerCanonicalUint64(field.encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid trust-head %s: %w", field.label, err)
		}
		*field.destination = decoded
	}
	bytes32Fields := []struct {
		label       string
		encoded     string
		destination *[32]byte
	}{
		{"certificate digest", wire.CertificateDigest, &result.CertificateDigest},
		{"activation manifest hash", wire.ActivationManifestHash, &result.ActivationManifestHash},
		{"binding hash", wire.BindingHash, &result.BindingHash},
		{"response public key SPKI hash", wire.ResponsePublicKeySPKISHA256, &result.ResponsePublicKeySPKISHA256},
		{"offline authority SPKI hash", wire.OfflineAuthoritySPKISHA256, &result.OfflineAuthoritySPKISHA256},
	}
	for _, field := range bytes32Fields {
		decoded, err := decodeNativeTBTCSignerCanonicalBytes32(field.encoded, false)
		if err != nil {
			return nil, fmt.Errorf("invalid trust-head %s: %w", field.label, err)
		}
		*field.destination = decoded
	}
	floor, err := decodeNativeTBTCSignerStateAnchorTrustReference(
		&wire.CertifiedFloor,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid trust-head certified floor: %w", err)
	}
	result.CertifiedFloor = floor
	if result.CertificateSequence == 0 ||
		result.ActivationManifestSequence == 0 ||
		result.ServiceEpoch == 0 ||
		result.ServiceEpoch != result.CertifiedFloor.ServiceEpoch ||
		result.CertifiedFloor.Revision != 1 {
		return nil, fmt.Errorf("native signer state-anchor trust head is incomplete")
	}
	// The head is emitted from an endpoint the signer itself parsed through the
	// same geometry rule, so decoding at exactly that rule cannot reject a head
	// the signer can produce. Decoding any looser would let a readback the
	// signer would refuse to install look installable to the Go startup path.
	if err := ValidateNativeTBTCSignerStateWitnessGeometry(
		result.WitnessMaximumRecords,
		result.WitnessRotationThresholdRecords,
	); err != nil {
		return nil, fmt.Errorf(
			"native signer state-anchor trust head witness geometry is invalid: %w",
			err,
		)
	}
	return result, nil
}

func DecodeNativeTBTCSignerStateAnchorTrustTransitionResult(
	payload []byte,
) (*NativeTBTCSignerStateAnchorTrustTransitionResult, error) {
	wire := &nativeTBTCSignerStateAnchorTrustTransitionResultWire{}
	if err := decodeStrictNativeTBTCSignerJSON(
		payload,
		wire,
		"state-anchor trust transition result",
	); err != nil {
		return nil, err
	}
	if wire.Schema != NativeTBTCSignerStateAnchorTrustTransitionResultSchema ||
		wire.Installed == nil || wire.Idempotent == nil {
		return nil, fmt.Errorf(
			"native signer state-anchor trust transition result is incomplete",
		)
	}
	applied, err := decodeNativeTBTCSignerCanonicalUint64(
		wire.AppliedCertificateCount,
	)
	if err != nil || applied > 64 {
		return nil, fmt.Errorf(
			"invalid native signer applied trust-certificate count",
		)
	}
	head, err := decodeNativeTBTCSignerStateAnchorTrustHeadWire(&wire.TrustHead)
	if err != nil {
		return nil, err
	}
	current, err := decodeNativeTBTCSignerStateAnchorCheckpoint(
		&wire.CurrentCheckpoint,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid transition current checkpoint: %w", err)
	}
	base, err := decodeNativeTBTCSignerStateAnchorCheckpoint(
		&wire.WitnessBaseCheckpoint,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid transition witness-base checkpoint: %w", err)
	}
	anchor, err := decodeNativeTBTCSignerStateAnchorTrustReference(
		&wire.CurrentAnchorReference,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid transition current anchor reference: %w", err)
	}
	result := &NativeTBTCSignerStateAnchorTrustTransitionResult{
		Schema:                  wire.Schema,
		Installed:               *wire.Installed,
		Idempotent:              *wire.Idempotent,
		AppliedCertificateCount: applied,
		TrustHead:               *head,
		CurrentCheckpoint:       current,
		WitnessBaseCheckpoint:   base,
		CurrentAnchorReference:  anchor,
	}
	if !result.Installed ||
		(result.Idempotent && result.AppliedCertificateCount != 0) ||
		result.WitnessBaseCheckpoint.StoreFingerprint !=
			result.CurrentCheckpoint.StoreFingerprint ||
		result.WitnessBaseCheckpoint.Generation >
			result.CurrentCheckpoint.Generation ||
		result.CurrentAnchorReference.Checkpoint != result.CurrentCheckpoint ||
		result.CurrentAnchorReference.ServiceEpoch != result.TrustHead.ServiceEpoch ||
		result.CurrentAnchorReference.Revision <
			result.TrustHead.CertifiedFloor.Revision {
		return nil, fmt.Errorf(
			"native signer state-anchor trust transition readback is inconsistent",
		)
	}
	return result, nil
}

func decodeNativeTBTCSignerStateAnchorTrustRecoveryRequired(
	wire *nativeTBTCSignerStateAnchorTrustRecoveryRequiredWire,
) (*NativeTBTCSignerStateAnchorTrustRecoveryRequired, error) {
	if wire == nil ||
		wire.Schema !=
			NativeTBTCSignerStateAnchorTrustRecoveryRequiredSchema {
		return nil, fmt.Errorf(
			"unsupported native signer state-anchor trust-recovery schema",
		)
	}
	result := &NativeTBTCSignerStateAnchorTrustRecoveryRequired{
		Schema: wire.Schema,
	}
	decimalFields := []struct {
		encoded     string
		destination *uint64
	}{
		{wire.CertificateCount, &result.CertificateCount},
		{wire.FirstCertificateSequence, &result.FirstCertificateSequence},
		{wire.FinalCertificateSequence, &result.FinalCertificateSequence},
		{wire.TargetServiceEpoch, &result.TargetServiceEpoch},
		{wire.TargetRevision, &result.TargetRevision},
	}
	for _, field := range decimalFields {
		value, err := decodeNativeTBTCSignerCanonicalUint64(field.encoded)
		if err != nil || value == 0 {
			return nil, fmt.Errorf(
				"native signer state-anchor trust-recovery decimal field is invalid",
			)
		}
		*field.destination = value
	}
	bytes32Fields := []struct {
		encoded     string
		destination *[32]byte
	}{
		{wire.StoreFingerprint, &result.StoreFingerprint},
		{wire.FinalCertificateDigest, &result.FinalCertificateDigest},
		{wire.TargetBindingHash, &result.TargetBindingHash},
	}
	for _, field := range bytes32Fields {
		value, err := decodeNativeTBTCSignerCanonicalBytes32(
			field.encoded,
			false,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"native signer state-anchor trust-recovery bytes32 field is invalid",
			)
		}
		*field.destination = value
	}
	if result.CertificateCount >
		NativeTBTCSignerStateAnchorTrustTransitionMaximumCertificateCount ||
		uint64(len(wire.OrderedCertificateDigests)) !=
			result.CertificateCount ||
		result.FirstCertificateSequence >
			^uint64(0)-(result.CertificateCount-1) ||
		result.FinalCertificateSequence !=
			result.FirstCertificateSequence+result.CertificateCount-1 {
		return nil, fmt.Errorf(
			"native signer state-anchor trust-recovery certificate range is invalid",
		)
	}
	result.OrderedCertificateDigests = make(
		[][32]byte,
		len(wire.OrderedCertificateDigests),
	)
	seenDigests := make(map[[32]byte]struct{}, len(wire.OrderedCertificateDigests))
	for index, encoded := range wire.OrderedCertificateDigests {
		digest, err := decodeNativeTBTCSignerCanonicalBytes32(encoded, false)
		if err != nil {
			return nil, fmt.Errorf(
				"native signer state-anchor trust-recovery certificate digest [%d] is invalid",
				index,
			)
		}
		if _, exists := seenDigests[digest]; exists {
			return nil, fmt.Errorf(
				"native signer state-anchor trust-recovery certificate digests are not unique",
			)
		}
		seenDigests[digest] = struct{}{}
		result.OrderedCertificateDigests[index] = digest
	}
	if result.OrderedCertificateDigests[len(result.OrderedCertificateDigests)-1] !=
		result.FinalCertificateDigest {
		return nil, fmt.Errorf(
			"native signer state-anchor trust-recovery final digest is inconsistent",
		)
	}
	checkpoint, err := decodeNativeTBTCSignerStateAnchorCheckpoint(
		&wire.TargetCheckpoint,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"native signer state-anchor trust-recovery checkpoint is invalid: %w",
			err,
		)
	}
	if checkpoint.StoreFingerprint != result.StoreFingerprint {
		return nil, fmt.Errorf(
			"native signer state-anchor trust-recovery checkpoint belongs to another store",
		)
	}
	result.TargetCheckpoint = checkpoint
	return result, nil
}

func decodeNativeTBTCSignerStateAnchorTrustReference(
	wire *nativeTBTCSignerStateAnchorTrustReferenceWire,
) (NativeTBTCSignerStateAnchorTrustReference, error) {
	result := NativeTBTCSignerStateAnchorTrustReference{}
	if wire == nil {
		return result, fmt.Errorf("reference is absent")
	}
	var err error
	if result.ServiceEpoch, err =
		decodeNativeTBTCSignerCanonicalUint64(wire.ServiceEpoch); err != nil {
		return result, err
	}
	if result.Revision, err =
		decodeNativeTBTCSignerCanonicalUint64(wire.Revision); err != nil {
		return result, err
	}
	bytes32Fields := []struct {
		encoded     string
		destination *[32]byte
		allowZero   bool
	}{
		{wire.PreviousEventRoot, &result.PreviousEventRoot, true},
		{wire.EventRoot, &result.EventRoot, false},
		{wire.CheckpointAckDigest, &result.AcknowledgementDigest, false},
	}
	for _, field := range bytes32Fields {
		value, err := decodeNativeTBTCSignerCanonicalBytes32(
			field.encoded,
			field.allowZero,
		)
		if err != nil {
			return result, err
		}
		*field.destination = value
	}
	checkpoint, err := decodeNativeTBTCSignerStateAnchorCheckpoint(
		&wire.Checkpoint,
	)
	if err != nil {
		return result, err
	}
	result.Checkpoint = checkpoint
	if result.ServiceEpoch == 0 || result.Revision == 0 ||
		(result.Revision > 1 && result.PreviousEventRoot == [32]byte{}) {
		return result, fmt.Errorf("reference audit identity is incomplete")
	}
	return result, nil
}

func decodeNativeTBTCSignerStateAnchorCheckpoint(
	wire *nativeTBTCSignerStateAnchorCheckpointWire,
) (NativeTBTCSignerStateAnchorCheckpoint, error) {
	result := NativeTBTCSignerStateAnchorCheckpoint{}
	if wire == nil {
		return result, fmt.Errorf("checkpoint is absent")
	}
	generation, err := decodeNativeTBTCSignerCanonicalUint64(wire.Generation)
	if err != nil || generation == 0 {
		return result, fmt.Errorf("checkpoint generation is invalid")
	}
	result.Generation = generation
	fields := []struct {
		encoded     string
		destination *[32]byte
	}{
		{wire.StoreFingerprint, &result.StoreFingerprint},
		{wire.PreviousStateCommitment, &result.PreviousStateCommitment},
		{wire.StateImageDigest, &result.StateImageDigest},
		{wire.StateCommitment, &result.StateCommitment},
	}
	for _, field := range fields {
		value, err := decodeNativeTBTCSignerCanonicalBytes32(field.encoded, false)
		if err != nil {
			return result, err
		}
		*field.destination = value
	}
	computed := ComputeNativeTBTCSignerStateWitnessCommitment(
		result.StoreFingerprint,
		result.Generation,
		result.PreviousStateCommitment,
		result.StateImageDigest,
	)
	if computed != result.StateCommitment {
		return result, fmt.Errorf("checkpoint commitment mismatch")
	}
	return result, nil
}

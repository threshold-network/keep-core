package roast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastMessageTypePrefix is the per-protocol prefix every ROAST-layer
// wire message uses for its net.TaggedUnmarshaler Type(). Distinct
// from frost_signing/native_frost/ and frost_signing/native_tbtc_signer/
// so the network router can dispatch unambiguously.
const roastMessageTypePrefix = "frost_signing/roast/"

// LocalEvidenceSnapshotType is the stable Type() string for a single
// signer's signed evidence snapshot.
const LocalEvidenceSnapshotType = roastMessageTypePrefix + "local_evidence_snapshot"

// TransitionMessageType is the stable Type() string for the
// coordinator-aggregated bundle.
const TransitionMessageType = roastMessageTypePrefix + "transition_message"

// MaxSnapshotsPerBundle caps the number of LocalEvidenceSnapshot
// entries a TransitionMessage may carry. Sized for the worst-case
// production signing group plus headroom; rejects pathological
// bundles at Unmarshal time so a misbehaving peer cannot exhaust
// memory on the receiver.
const MaxSnapshotsPerBundle = 256

// MaxOperatorSignatureBytes caps the per-snapshot OperatorSignature
// length. Sized to accept secp256k1 DER (~72 bytes), ed25519 (64
// bytes), and reasonable post-quantum candidates without committing
// to a specific scheme at this layer. Rejects oversize payloads.
const MaxOperatorSignatureBytes = 256

// MaxCoordinatorSignatureBytes caps the bundle-level
// CoordinatorSignature. Same justification as
// MaxOperatorSignatureBytes.
const MaxCoordinatorSignatureBytes = 256

// OverflowEntry is the JSON-friendly key/value pair representing one
// per-sender overflow count from an attempt.Evidence map. The slice
// representation is canonical (sorted by Sender ascending) so any
// two honest signers serialising the same evidence produce
// byte-identical JSON.
type OverflowEntry struct {
	Sender group.MemberIndex `json:"sender"`
	Count  uint              `json:"count"`
}

// LocalEvidenceSnapshot is the per-signer signed evidence produced
// during a single attempt. It is the input to the coordinator's
// aggregation and to the receiver-side bundle verification.
//
// Phase 3.2 (this file) defines the wire type only. Signature
// computation and verification land in Phase 3.3.
type LocalEvidenceSnapshot struct {
	SenderIDValue uint32 `json:"senderID"`
	// AttemptContextHash binds the snapshot to the attempt the
	// evidence describes. Always exactly 32 bytes.
	AttemptContextHash []byte `json:"attemptContextHash"`
	// Overflows is the canonical sorted form of the
	// attempt.Evidence.Overflows map; sorted ascending by Sender.
	// Omitted when no overflow events were observed.
	Overflows []OverflowEntry `json:"overflows,omitempty"`
	// OperatorSignature is the signer's operator-key signature over
	// the canonical encoding of (senderID, attemptContextHash,
	// overflows). Phase 3.3 defines the canonical-encoding
	// algorithm and the verification routine. Phase 3.2 treats this
	// field as opaque bytes with a length cap.
	OperatorSignature []byte `json:"operatorSignature,omitempty"`
}

// NewLocalEvidenceSnapshot converts an attempt.Evidence map into a
// LocalEvidenceSnapshot ready for signing and broadcast. The
// resulting snapshot's Overflows field is sorted ascending by
// Sender for deterministic JSON encoding. The OperatorSignature is
// left empty -- the caller must sign and populate it (Phase 3.3).
func NewLocalEvidenceSnapshot(
	sender group.MemberIndex,
	attemptContextHash [attempt.MessageDigestLength]byte,
	evidence attempt.Evidence,
) *LocalEvidenceSnapshot {
	overflows := make([]OverflowEntry, 0, len(evidence.Overflows))
	for s, c := range evidence.Overflows {
		overflows = append(overflows, OverflowEntry{Sender: s, Count: c})
	}
	sort.Slice(overflows, func(i, j int) bool {
		return overflows[i].Sender < overflows[j].Sender
	})
	return &LocalEvidenceSnapshot{
		SenderIDValue:      uint32(sender),
		AttemptContextHash: append([]byte{}, attemptContextHash[:]...),
		Overflows:          overflows,
	}
}

// SenderID returns the snapshot's sender as a group.MemberIndex.
func (s *LocalEvidenceSnapshot) SenderID() group.MemberIndex {
	return group.MemberIndex(s.SenderIDValue)
}

// AttemptContextHashArray returns the 32-byte attempt context hash
// as a fixed-size array. Returns the zero array if the field is
// malformed (caller should have validated via Unmarshal first).
func (s *LocalEvidenceSnapshot) AttemptContextHashArray() [attempt.MessageDigestLength]byte {
	var out [attempt.MessageDigestLength]byte
	if len(s.AttemptContextHash) == attempt.MessageDigestLength {
		copy(out[:], s.AttemptContextHash)
	}
	return out
}

// Evidence reconstructs the attempt.Evidence map form from the
// canonical sorted-slice representation. The returned Evidence
// shares no state with the snapshot.
func (s *LocalEvidenceSnapshot) Evidence() attempt.Evidence {
	out := attempt.Evidence{
		Overflows: make(map[group.MemberIndex]uint, len(s.Overflows)),
	}
	for _, e := range s.Overflows {
		out.Overflows[e.Sender] = e.Count
	}
	return out
}

// Type implements net.TaggedUnmarshaler.
func (s *LocalEvidenceSnapshot) Type() string {
	return LocalEvidenceSnapshotType
}

// Marshal serialises the snapshot to canonical JSON. The Overflows
// slice is sorted by Sender ascending in NewLocalEvidenceSnapshot
// so two honest signers with the same evidence produce
// byte-identical bytes.
func (s *LocalEvidenceSnapshot) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

// Unmarshal parses canonical JSON into the snapshot and validates
// the resulting structure.
func (s *LocalEvidenceSnapshot) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	return s.Validate()
}

// Validate runs the structural checks Unmarshal applies after a JSON
// decode. Exposed publicly so callers that construct snapshots in
// memory (e.g. the Coordinator state machine) can validate without
// a marshal/unmarshal round-trip.
func (s *LocalEvidenceSnapshot) Validate() error {
	if s.SenderIDValue == 0 {
		return errors.New("local evidence snapshot: senderID is zero")
	}
	if len(s.AttemptContextHash) != attempt.MessageDigestLength {
		return fmt.Errorf(
			"local evidence snapshot: attemptContextHash length [%d], expected [%d]",
			len(s.AttemptContextHash),
			attempt.MessageDigestLength,
		)
	}
	if len(s.OperatorSignature) > MaxOperatorSignatureBytes {
		return fmt.Errorf(
			"local evidence snapshot: operatorSignature length [%d] exceeds cap [%d]",
			len(s.OperatorSignature),
			MaxOperatorSignatureBytes,
		)
	}
	for i := 1; i < len(s.Overflows); i++ {
		if s.Overflows[i].Sender <= s.Overflows[i-1].Sender {
			return fmt.Errorf(
				"local evidence snapshot: overflows not sorted ascending or contain duplicate at index %d",
				i,
			)
		}
	}
	return nil
}

// TransitionMessage is the coordinator-aggregated bundle that drives
// the deterministic NextAttempt transition. It contains every
// participating signer's signed evidence snapshot for one attempt,
// plus the coordinator's own signature over the canonical bundle.
//
// Phase 3.2 (this file) defines the wire type. Aggregation,
// canonical encoding, and verification land in Phase 3.3.
type TransitionMessage struct {
	// AttemptContextHash identifies the attempt the bundle
	// describes. Must match every snapshot's AttemptContextHash.
	// Always exactly 32 bytes.
	AttemptContextHash []byte `json:"attemptContextHash"`
	// CoordinatorIDValue is the member index of the elected
	// coordinator that produced this bundle.
	CoordinatorIDValue uint32 `json:"coordinatorID"`
	// Bundle is the canonical sorted-by-SenderID list of signed
	// evidence snapshots aggregated by the coordinator.
	Bundle []LocalEvidenceSnapshot `json:"bundle"`
	// CoordinatorSignature is the coordinator's operator-key
	// signature over the canonical encoding of the bundle. Phase
	// 3.3 defines the canonical-encoding algorithm and the
	// verification routine. Phase 3.2 treats this field as opaque
	// bytes with a length cap.
	CoordinatorSignature []byte `json:"coordinatorSignature,omitempty"`
}

// CoordinatorID returns the coordinator member index as a
// group.MemberIndex.
func (m *TransitionMessage) CoordinatorID() group.MemberIndex {
	return group.MemberIndex(m.CoordinatorIDValue)
}

// AttemptContextHashArray returns the 32-byte attempt context hash
// as a fixed-size array. Returns the zero array if the field is
// malformed (caller should have validated via Unmarshal first).
func (m *TransitionMessage) AttemptContextHashArray() [attempt.MessageDigestLength]byte {
	var out [attempt.MessageDigestLength]byte
	if len(m.AttemptContextHash) == attempt.MessageDigestLength {
		copy(out[:], m.AttemptContextHash)
	}
	return out
}

// Type implements net.TaggedUnmarshaler.
func (m *TransitionMessage) Type() string {
	return TransitionMessageType
}

// Marshal serialises the message to canonical JSON.
func (m *TransitionMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal parses canonical JSON into the message and validates
// the structure: hash length, bundle size cap, signature size cap,
// snapshot validity, bundle ordering by SenderID ascending, and
// every snapshot binding to the same AttemptContextHash as the
// bundle.
func (m *TransitionMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, m); err != nil {
		return err
	}
	return m.Validate()
}

// Validate runs the structural checks Unmarshal applies after a JSON
// decode: bundle hash length, bundle size cap, coordinator id, every
// snapshot's validity, bundle ordering, and intra-bundle hash
// consistency. Exposed publicly so callers that construct messages
// in memory can validate without a marshal/unmarshal round-trip.
func (m *TransitionMessage) Validate() error {
	if len(m.AttemptContextHash) != attempt.MessageDigestLength {
		return fmt.Errorf(
			"transition message: attemptContextHash length [%d], expected [%d]",
			len(m.AttemptContextHash),
			attempt.MessageDigestLength,
		)
	}
	if m.CoordinatorIDValue == 0 {
		return errors.New("transition message: coordinatorID is zero")
	}
	if len(m.Bundle) == 0 {
		return errors.New("transition message: bundle must not be empty")
	}
	if len(m.Bundle) > MaxSnapshotsPerBundle {
		return fmt.Errorf(
			"transition message: bundle length [%d] exceeds cap [%d]",
			len(m.Bundle),
			MaxSnapshotsPerBundle,
		)
	}
	if len(m.CoordinatorSignature) > MaxCoordinatorSignatureBytes {
		return fmt.Errorf(
			"transition message: coordinatorSignature length [%d] exceeds cap [%d]",
			len(m.CoordinatorSignature),
			MaxCoordinatorSignatureBytes,
		)
	}
	for i := range m.Bundle {
		if err := m.Bundle[i].Validate(); err != nil {
			return fmt.Errorf(
				"transition message: bundle[%d] invalid: %w",
				i, err,
			)
		}
		if !bytes.Equal(m.Bundle[i].AttemptContextHash, m.AttemptContextHash) {
			return fmt.Errorf(
				"transition message: bundle[%d] attempt context hash does not match bundle hash",
				i,
			)
		}
		if i > 0 {
			if m.Bundle[i].SenderIDValue <= m.Bundle[i-1].SenderIDValue {
				return fmt.Errorf(
					"transition message: bundle not sorted ascending by senderID or contains duplicate at index %d",
					i,
				)
			}
		}
	}
	return nil
}

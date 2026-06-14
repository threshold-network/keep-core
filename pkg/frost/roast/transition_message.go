package roast

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/roast/gen/pb"
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
	Sender group.MemberIndex
	Count  uint
}

// RejectEntry carries one per-(sender, reason) reject count from an
// attempt.Evidence map. The bundle's Rejects field is sorted
// ascending first by Sender, then by Reason, so two honest signers
// produce byte-identical canonical encodings.
type RejectEntry struct {
	Sender group.MemberIndex
	Reason string
	Count  uint
}

// ConflictEntry carries one per-sender conflict count -- the number
// of first-write-wins disagreements detected during the attempt.
// Sorted ascending by Sender for canonical encoding.
type ConflictEntry struct {
	Sender group.MemberIndex
	Count  uint
}

// LocalEvidenceSnapshot is the per-signer signed evidence produced
// during a single attempt. It is the input to the coordinator's
// aggregation and to the receiver-side bundle verification.
//
// Phase 3.2 (this file) defines the wire type only. Signature
// computation and verification land in Phase 3.3.
type LocalEvidenceSnapshot struct {
	SenderIDValue uint32
	// AttemptContextHash binds the snapshot to the attempt the
	// evidence describes. Always exactly 32 bytes.
	AttemptContextHash []byte
	// Overflows is the canonical sorted form of the
	// attempt.Evidence.Overflows map; sorted ascending by Sender.
	// Omitted when no overflow events were observed.
	Overflows []OverflowEntry
	// Rejects is the canonical sorted form of the
	// attempt.Evidence.Rejects map; sorted ascending first by Sender,
	// then by Reason. Omitted when no validation-reject events were
	// observed. Each entry counts the number of rejects observed
	// for one (sender, reason) pair, saturated at the recorder's
	// reject quota.
	Rejects []RejectEntry
	// Conflicts is the canonical sorted form of the
	// attempt.Evidence.Conflicts map; sorted ascending by Sender.
	// Omitted when no first-write-wins-conflict events were
	// observed.
	Conflicts []ConflictEntry
	// OperatorSignature is the signer's operator-key signature over
	// SignableBytes(): the snapshot domain tag followed by the serialized
	// protobuf body of (senderID, attemptContextHash, overflows, rejects,
	// conflicts).
	OperatorSignature []byte

	// bodyCache caches the exact serialized body bytes carried on the wire:
	// marshaled once at signing time for self-authored snapshots, or the
	// received bytes verbatim for parsed ones. Evidence fields must not be
	// mutated once set.
	bodyCache []byte
	// signaturePayloadCache caches the domain-tagged bytes the
	// OperatorSignature covers (localEvidenceSnapshotSignatureDomain ||
	// bodyCache); rebuilt from bodyCache and never carried on the wire.
	signaturePayloadCache []byte
	// wireEnvelope caches the exact on-wire envelope (body +
	// signature): the received bytes verbatim for parsed snapshots,
	// or built once after signing for self-authored ones.
	wireEnvelope []byte
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

	rejects := make([]RejectEntry, 0)
	for s, entries := range evidence.Rejects {
		for _, e := range entries {
			rejects = append(rejects, RejectEntry{
				Sender: s,
				Reason: e.Reason,
				Count:  e.Count,
			})
		}
	}
	sort.Slice(rejects, func(i, j int) bool {
		if rejects[i].Sender != rejects[j].Sender {
			return rejects[i].Sender < rejects[j].Sender
		}
		return rejects[i].Reason < rejects[j].Reason
	})

	conflicts := make([]ConflictEntry, 0, len(evidence.Conflicts))
	for s, c := range evidence.Conflicts {
		conflicts = append(conflicts, ConflictEntry{Sender: s, Count: c})
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Sender < conflicts[j].Sender
	})

	snap := &LocalEvidenceSnapshot{
		SenderIDValue:      uint32(sender),
		AttemptContextHash: append([]byte{}, attemptContextHash[:]...),
		Overflows:          overflows,
	}
	if len(rejects) > 0 {
		snap.Rejects = rejects
	}
	if len(conflicts) > 0 {
		snap.Conflicts = conflicts
	}
	return snap
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
		Rejects:   make(map[group.MemberIndex][]attempt.RejectEntry, 0),
		Conflicts: make(map[group.MemberIndex]uint, len(s.Conflicts)),
	}
	for _, e := range s.Overflows {
		out.Overflows[e.Sender] = e.Count
	}
	for _, e := range s.Rejects {
		out.Rejects[e.Sender] = append(out.Rejects[e.Sender], attempt.RejectEntry{
			Reason: e.Reason,
			Count:  e.Count,
		})
	}
	for _, e := range s.Conflicts {
		out.Conflicts[e.Sender] = e.Count
	}
	return out
}

// Type implements net.TaggedUnmarshaler.
func (s *LocalEvidenceSnapshot) Type() string {
	return LocalEvidenceSnapshotType
}

// Marshal serialises the snapshot as a SignedLocalEvidenceSnapshot
// envelope: the exact signed body bytes plus the operator signature.
// For a snapshot parsed off the wire the received envelope is
// returned verbatim, so evidence bytes survive any re-broadcast
// unchanged. The snapshot must be signed first. The returned slice is
// the internal cache - callers must not mutate it.
func (s *LocalEvidenceSnapshot) Marshal() ([]byte, error) {
	return s.wireEnvelopeBytes()
}

// Unmarshal parses a SignedLocalEvidenceSnapshot envelope, retains
// the received body and envelope bytes verbatim (signature
// verification runs over exactly these bytes), populates the
// evidence fields from the body, and validates the structure.
func (s *LocalEvidenceSnapshot) Unmarshal(data []byte) error {
	var envelope pb.SignedLocalEvidenceSnapshot
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("local evidence snapshot: parse envelope: %w", err)
	}
	if len(envelope.Body) == 0 {
		return errors.New("local evidence snapshot: empty body")
	}
	var body pb.LocalEvidenceSnapshotBody
	if err := proto.Unmarshal(envelope.Body, &body); err != nil {
		return fmt.Errorf("local evidence snapshot: parse body: %w", err)
	}
	snapshotFieldsFromBody(s, &body)
	s.OperatorSignature = append([]byte(nil), envelope.OperatorSignature...)
	s.bodyCache = append([]byte(nil), envelope.Body...)
	s.wireEnvelope = append([]byte(nil), data...)
	// Clear any signable-bytes cache a prior SignableBytes call left on a
	// reused receiver, so the next call rebuilds it from the received body.
	s.signaturePayloadCache = nil
	return s.Validate()
}

// Validate runs the structural checks Unmarshal applies after a
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
	for i := 1; i < len(s.Rejects); i++ {
		prev := s.Rejects[i-1]
		cur := s.Rejects[i]
		if cur.Sender < prev.Sender {
			return fmt.Errorf(
				"local evidence snapshot: rejects not sorted ascending by sender at index %d",
				i,
			)
		}
		if cur.Sender == prev.Sender && cur.Reason <= prev.Reason {
			return fmt.Errorf(
				"local evidence snapshot: rejects not sorted ascending by reason or contain duplicate at index %d",
				i,
			)
		}
	}
	for i := 1; i < len(s.Conflicts); i++ {
		if s.Conflicts[i].Sender <= s.Conflicts[i-1].Sender {
			return fmt.Errorf(
				"local evidence snapshot: conflicts not sorted ascending or contain duplicate at index %d",
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
	AttemptContextHash []byte
	// CoordinatorIDValue is the member index of the elected
	// coordinator that produced this bundle.
	CoordinatorIDValue uint32
	// Bundle is the canonical sorted-by-SenderID list of signed
	// evidence snapshots aggregated by the coordinator.
	Bundle []LocalEvidenceSnapshot
	// CoordinatorSignature is the coordinator's operator-key signature over
	// SignableBytes(): the transition domain tag followed by the serialized
	// protobuf body embedding every snapshot's signed envelope verbatim.
	CoordinatorSignature []byte

	// bodyCache, signaturePayloadCache, and wireEnvelope cache exact bytes
	// with the same semantics as the LocalEvidenceSnapshot caches.
	bodyCache             []byte
	signaturePayloadCache []byte
	wireEnvelope          []byte
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

// Marshal serialises the message as a SignedTransitionMessage
// envelope: the exact signed body bytes plus the coordinator
// signature. For a message parsed off the wire the received envelope
// is returned verbatim. The message must be signed first. The
// returned slice is the internal cache - callers must not mutate it.
func (m *TransitionMessage) Marshal() ([]byte, error) {
	if m.wireEnvelope != nil {
		return m.wireEnvelope, nil
	}
	if len(m.CoordinatorSignature) == 0 {
		return nil, errors.New(
			"transition message: must be signed before wire encoding",
		)
	}
	body, err := m.bodyBytes()
	if err != nil {
		return nil, err
	}
	envelope, err := proto.Marshal(&pb.SignedTransitionMessage{
		Body:                 body,
		CoordinatorSignature: m.CoordinatorSignature,
	})
	if err != nil {
		return nil, fmt.Errorf("transition message: marshal envelope: %w", err)
	}
	m.wireEnvelope = envelope
	return envelope, nil
}

// Unmarshal parses a SignedTransitionMessage envelope, retains the
// received body and envelope bytes verbatim (the coordinator
// signature verifies over exactly these bytes), parses each embedded
// snapshot envelope (each retaining its own received bytes), and
// validates the structure: hash length, bundle size cap, signature
// size cap, snapshot validity, bundle ordering by SenderID
// ascending, and every snapshot binding to the same
// AttemptContextHash as the bundle.
func (m *TransitionMessage) Unmarshal(data []byte) error {
	var envelope pb.SignedTransitionMessage
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("transition message: parse envelope: %w", err)
	}
	if len(envelope.Body) == 0 {
		return errors.New("transition message: empty body")
	}
	var body pb.TransitionMessageBody
	if err := proto.Unmarshal(envelope.Body, &body); err != nil {
		return fmt.Errorf("transition message: parse body: %w", err)
	}
	if len(body.SignedSnapshots) > MaxSnapshotsPerBundle {
		return fmt.Errorf(
			"transition message: bundle length [%d] exceeds cap [%d]",
			len(body.SignedSnapshots),
			MaxSnapshotsPerBundle,
		)
	}
	m.AttemptContextHash = append([]byte(nil), body.AttemptContextHash...)
	m.CoordinatorIDValue = body.CoordinatorId
	m.Bundle = make([]LocalEvidenceSnapshot, 0, len(body.SignedSnapshots))
	for i, raw := range body.SignedSnapshots {
		var snapshot LocalEvidenceSnapshot
		if err := snapshot.Unmarshal(raw); err != nil {
			return fmt.Errorf("transition message: bundle[%d]: %w", i, err)
		}
		m.Bundle = append(m.Bundle, snapshot)
	}
	m.CoordinatorSignature = append([]byte(nil), envelope.CoordinatorSignature...)
	m.bodyCache = append([]byte(nil), envelope.Body...)
	m.wireEnvelope = append([]byte(nil), data...)
	// Clear any signable-bytes cache left on a reused receiver (see
	// LocalEvidenceSnapshot.Unmarshal).
	m.signaturePayloadCache = nil
	return m.Validate()
}

// Validate runs the structural checks Unmarshal applies after a
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

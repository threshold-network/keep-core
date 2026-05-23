package roast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// Signer produces operator-key signatures over canonical-encoded
// payloads. The ROAST coordinator state machine uses one Signer per
// node to sign its own LocalEvidenceSnapshot before broadcast, and
// the elected coordinator uses the same Signer to sign the assembled
// TransitionMessage bundle.
//
// Phase 3.3 (this file) defines the interface. Phase 4 wires it to
// pkg/net's operator-key signing surface so signatures are
// automatically attributable to the node's libp2p identity.
//
// Implementations must be safe for concurrent calls from multiple
// goroutines.
type Signer interface {
	// Sign returns a signature over the canonical payload produced
	// by CanonicalSnapshotBytes or CanonicalBundleBytes. The
	// returned signature is treated as opaque bytes by the
	// coordinator state machine; the SignatureVerifier is the only
	// component that interprets the byte sequence.
	Sign(payload []byte) ([]byte, error)
}

// SignatureVerifier verifies a signature attributed to a specific
// member. The verifier owns the member-to-public-key mapping; the
// coordinator state machine does not see public keys directly.
//
// Phase 3.3 (this file) defines the interface. Phase 4 wires it to
// pkg/net's member-keys table.
//
// Implementations must be safe for concurrent calls from multiple
// goroutines.
type SignatureVerifier interface {
	// Verify returns nil if signature is a valid signature over
	// payload produced by the operator key of signer. Returns a
	// descriptive error otherwise.
	Verify(payload []byte, signature []byte, signer group.MemberIndex) error
}

// ErrSignatureInvalid is the canonical sentinel a SignatureVerifier
// returns when a signature does not validate against the supplied
// payload and signer. Callers that want to distinguish
// signature-verification failure from other errors should use
// errors.Is(err, ErrSignatureInvalid).
var ErrSignatureInvalid = errors.New("roast: signature is invalid")

// ErrSignatureMissing is returned by VerifyBundle when a snapshot
// or bundle lacks the signature the protocol requires.
var ErrSignatureMissing = errors.New("roast: signature missing")

// ErrCensorshipDetected is returned by VerifyBundle when a receiver
// finds its own LocalEvidenceSnapshot absent from a bundle the
// receiver expected to be present in. The receiver's snapshot is
// missing either because the elected coordinator dropped it
// (malicious or otherwise) or because the bundle was constructed
// before the receiver's submission arrived. In either case, the
// receiver must not feed the bundle into NextAttempt.
var ErrCensorshipDetected = errors.New(
	"roast: own evidence snapshot missing from transition bundle (censorship or race)",
)

// NoOpSigner returns a Signer whose Sign returns an empty signature.
// Suitable as a default in tests that do not exercise the signature
// pipeline, and as the implicit default of NewInMemoryCoordinator
// (which is preserved for backward compatibility with Phase 3.1
// callers).
//
// A NoOpSigner-produced bundle is rejected by any non-NoOp verifier:
// the verifier sees a missing signature and fails closed. So the
// pair {NoOpSigner, NoOpSignatureVerifier} is only suitable when the
// caller wants to test the structural-aggregation pipeline in
// isolation from the crypto pipeline.
func NoOpSigner() Signer { return noOpSigner{} }

// NoOpSignatureVerifier returns a SignatureVerifier that accepts
// every signature, including empty ones. Use ONLY in tests that do
// not exercise the signature pipeline.
func NoOpSignatureVerifier() SignatureVerifier { return noOpSignatureVerifier{} }

type noOpSigner struct{}

func (noOpSigner) Sign(_ []byte) ([]byte, error) { return nil, nil }

type noOpSignatureVerifier struct{}

func (noOpSignatureVerifier) Verify(_, _ []byte, _ group.MemberIndex) error {
	return nil
}

// CanonicalSnapshotBytes returns the byte stream over which a signer
// signs a LocalEvidenceSnapshot. The encoding excludes the
// OperatorSignature field so a verifier can recompute the bytes from
// the snapshot it received over the wire.
//
// The encoding is canonical JSON: the Overflows slice must already
// be sorted ascending by Sender (NewLocalEvidenceSnapshot guarantees
// this; Unmarshal enforces it). Any two honest signers seeing the
// same snapshot fields produce byte-identical canonical bytes.
func CanonicalSnapshotBytes(s *LocalEvidenceSnapshot) ([]byte, error) {
	if s == nil {
		return nil, errors.New("roast: cannot canonicalise a nil snapshot")
	}
	clone := LocalEvidenceSnapshot{
		SenderIDValue:      s.SenderIDValue,
		AttemptContextHash: s.AttemptContextHash,
		Overflows:          s.Overflows,
		// OperatorSignature intentionally omitted -- it is the
		// signature *over* this canonical encoding, not part of it.
	}
	return json.Marshal(&clone)
}

// CanonicalBundleBytes returns the byte stream over which the elected
// coordinator signs a TransitionMessage. The encoding excludes the
// CoordinatorSignature field but *includes* every snapshot's
// OperatorSignature -- the coordinator's signature attests that
// these specific signed snapshots were assembled in this specific
// order.
//
// The Bundle slice must already be sorted ascending by SenderID; the
// canonical encoding assumes that invariant holds.
func CanonicalBundleBytes(m *TransitionMessage) ([]byte, error) {
	if m == nil {
		return nil, errors.New("roast: cannot canonicalise a nil transition message")
	}
	clone := TransitionMessage{
		AttemptContextHash: m.AttemptContextHash,
		CoordinatorIDValue: m.CoordinatorIDValue,
		Bundle:             m.Bundle,
		// CoordinatorSignature intentionally omitted.
	}
	return json.Marshal(&clone)
}

// verifySnapshotSignature checks the OperatorSignature on a single
// LocalEvidenceSnapshot against the verifier's record of the
// snapshot's sender's operator key.
func verifySnapshotSignature(
	verifier SignatureVerifier,
	snapshot *LocalEvidenceSnapshot,
) error {
	if len(snapshot.OperatorSignature) == 0 {
		return fmt.Errorf(
			"%w: snapshot from sender %d has no operator signature",
			ErrSignatureMissing,
			snapshot.SenderID(),
		)
	}
	payload, err := CanonicalSnapshotBytes(snapshot)
	if err != nil {
		return fmt.Errorf("canonical snapshot bytes: %w", err)
	}
	if err := verifier.Verify(
		payload,
		snapshot.OperatorSignature,
		snapshot.SenderID(),
	); err != nil {
		return fmt.Errorf(
			"%w: sender %d: %s",
			ErrSignatureInvalid,
			snapshot.SenderID(),
			err.Error(),
		)
	}
	return nil
}

// verifyBundleSignature checks the CoordinatorSignature on a
// TransitionMessage against the verifier's record of the bundle's
// declared coordinator's operator key. The coordinator member index
// passed in must match the elected coordinator for the attempt; the
// caller (Coordinator.VerifyBundle) resolves this from the
// AttemptHandle.
func verifyBundleSignature(
	verifier SignatureVerifier,
	msg *TransitionMessage,
	expectedCoordinator group.MemberIndex,
) error {
	if len(msg.CoordinatorSignature) == 0 {
		return fmt.Errorf(
			"%w: transition message has no coordinator signature",
			ErrSignatureMissing,
		)
	}
	if msg.CoordinatorID() != expectedCoordinator {
		return fmt.Errorf(
			"transition message coordinator id %d does not match expected %d for the attempt",
			msg.CoordinatorID(),
			expectedCoordinator,
		)
	}
	payload, err := CanonicalBundleBytes(msg)
	if err != nil {
		return fmt.Errorf("canonical bundle bytes: %w", err)
	}
	if err := verifier.Verify(
		payload,
		msg.CoordinatorSignature,
		msg.CoordinatorID(),
	); err != nil {
		return fmt.Errorf(
			"%w: coordinator %d: %s",
			ErrSignatureInvalid,
			msg.CoordinatorID(),
			err.Error(),
		)
	}
	return nil
}

// verifyOwnObservationsPresent is the receiver-side censorship-
// detection check: every receiver that has already submitted its
// own LocalEvidenceSnapshot to the elected coordinator must find
// that snapshot in the resulting bundle. A coordinator that drops a
// receiver's snapshot is detected here.
//
// When selfMember is zero, the check is skipped: that signals a
// caller that has not (yet) submitted its own snapshot and therefore
// has no censorship claim to verify.
func verifyOwnObservationsPresent(
	msg *TransitionMessage,
	selfMember group.MemberIndex,
	selfSubmission *LocalEvidenceSnapshot,
) error {
	if selfMember == 0 || selfSubmission == nil {
		return nil
	}
	for i := range msg.Bundle {
		if msg.Bundle[i].SenderID() != selfMember {
			continue
		}
		// Found the receiver's snapshot. The submitted-vs-bundled
		// signature must be byte-identical -- a coordinator that
		// re-signed or mutated the submission has tampered with
		// observed evidence.
		if !bytes.Equal(
			msg.Bundle[i].OperatorSignature,
			selfSubmission.OperatorSignature,
		) {
			return fmt.Errorf(
				"%w: own evidence snapshot signature mutated in bundle",
				ErrCensorshipDetected,
			)
		}
		return nil
	}
	return ErrCensorshipDetected
}

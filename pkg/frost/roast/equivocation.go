package roast

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

var equivocationLogger = log.Logger("keep-frost-roast-equivocation")

// Equivocation evidence kinds. Each event carries the exact signed
// envelope bytes behind a detection, so telemetry retains enough data to
// diagnose whether targeted equivocation is occurring before the full
// proof-carrying-blame wire format (follow-up item 7) exists. Two
// operator-signed bodies from the same sender for the same attempt are
// self-incriminating: both signatures verify, the bodies differ.
const (
	// EquivocationKindSnapshotConflict: a sender re-submitted a signed
	// snapshot for the same attempt that differs from its first
	// submission (first-write-wins conflict at the coordinator).
	EquivocationKindSnapshotConflict = "snapshot_conflict"
	// EquivocationKindOwnSnapshotMutatedInBundle: a coordinator bundle
	// carries this member's snapshot with a different signature than the
	// member actually submitted.
	EquivocationKindOwnSnapshotMutatedInBundle = "own_snapshot_mutated_in_bundle"
	// EquivocationKindOwnSnapshotMissingFromBundle: a coordinator bundle
	// omits this member's submitted snapshot entirely.
	EquivocationKindOwnSnapshotMissingFromBundle = "own_snapshot_missing_from_bundle"
)

// EquivocationEvidence carries the exact signed byte streams behind a
// detected conflict, censorship, or mutation event. Envelope fields hold
// SignedLocalEvidenceSnapshot wire bytes verbatim (body + operator
// signature) and may be nil when the corresponding side could not be
// encoded or does not exist (e.g. a snapshot missing from a bundle).
type EquivocationEvidence struct {
	Kind               string
	AttemptContextHash []byte
	Sender             group.MemberIndex
	// ExistingEnvelope is the first-accepted / self-submitted signed
	// snapshot envelope.
	ExistingEnvelope []byte
	// ConflictingEnvelope is the re-submitted / bundled signed snapshot
	// envelope that disagrees with ExistingEnvelope.
	ConflictingEnvelope []byte
}

// EquivocationEvidenceObserver consumes equivocation evidence events.
type EquivocationEvidenceObserver func(evidence EquivocationEvidence)

var (
	equivocationEvidenceObserverMutex sync.RWMutex
	equivocationEvidenceObserver      EquivocationEvidenceObserver
)

// RegisterEquivocationEvidenceObserver registers a process-wide observer
// used to retain equivocation evidence in the host's telemetry system.
// Only a single observer is supported.
func RegisterEquivocationEvidenceObserver(
	observer EquivocationEvidenceObserver,
) error {
	if observer == nil {
		return fmt.Errorf("equivocation evidence observer is nil")
	}

	equivocationEvidenceObserverMutex.Lock()
	defer equivocationEvidenceObserverMutex.Unlock()

	if equivocationEvidenceObserver != nil {
		return fmt.Errorf("equivocation evidence observer is already registered")
	}

	equivocationEvidenceObserver = observer

	return nil
}

// UnregisterEquivocationEvidenceObserver clears the observer registration.
func UnregisterEquivocationEvidenceObserver() {
	equivocationEvidenceObserverMutex.Lock()
	defer equivocationEvidenceObserverMutex.Unlock()

	equivocationEvidenceObserver = nil
}

// emitEquivocationEvidence logs the full evidence (these events are rare
// and the bytes are the diagnosis) and forwards it to the registered
// observer, if any. Never fails: evidence retention must not perturb the
// protocol path that detected the event.
func emitEquivocationEvidence(evidence EquivocationEvidence) {
	equivocationLogger.Warnf(
		"equivocation evidence [%s]: sender [%d], attempt context hash [%s], "+
			"existing envelope [%s], conflicting envelope [%s]",
		evidence.Kind,
		evidence.Sender,
		hex.EncodeToString(evidence.AttemptContextHash),
		hex.EncodeToString(evidence.ExistingEnvelope),
		hex.EncodeToString(evidence.ConflictingEnvelope),
	)

	equivocationEvidenceObserverMutex.RLock()
	observer := equivocationEvidenceObserver
	equivocationEvidenceObserverMutex.RUnlock()

	if observer != nil {
		observer(evidence)
	}
}

// snapshotEnvelopeForEvidence encodes a snapshot's signed envelope for
// evidence retention, tolerating encode failures (nil result) so the
// detection path never degrades.
func snapshotEnvelopeForEvidence(snapshot *LocalEvidenceSnapshot) []byte {
	if snapshot == nil {
		return nil
	}
	envelope, err := snapshot.Marshal()
	if err != nil {
		equivocationLogger.Warnf(
			"could not encode snapshot envelope for evidence retention: [%v]",
			err,
		)
		return nil
	}
	return envelope
}

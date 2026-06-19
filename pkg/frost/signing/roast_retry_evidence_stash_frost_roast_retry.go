//go:build frost_roast_retry

package signing

import (
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// PendingEvidenceRegistryTTL is how long a stashed evidence entry is retained
// before the background sweeper evicts it. Matches the observe-binding TTL: a
// stashed snapshot is useless once the session it tracks is archived.
const PendingEvidenceRegistryTTL = SessionHandleBindingTTL

// pendingEvidenceKey scopes one local seat's captured coarse-path evidence to a
// specific attempt of a ROAST session.
//
// RFC-21 Phase 7.3 PR2b-2 step 2 (the blame bridge): the coarse receive loop
// captures real rejects/overflows/conflicts into an EvidenceRecorder and, at
// end-of-collect, stashes the raw snapshot here (submitSnapshotIfActive). The
// transition exchange's BroadcastForcedSnapshot then reads it so the seat's
// broadcast snapshot -- and therefore the elected coordinator's aggregated bundle
// -- carries real evidence instead of a forced-empty proof-of-attendance one, so
// NextAttempt's f+1 accuser tally can finally fire.
//
// The key is (RoastSessionID, member, attemptHash): the same coordinate space the
// observe binding uses (attemptHash == ctx.Hash() is namespace-independent, and
// the stashing ctx.SessionID is the STABLE RoastSessionID == the exchange's
// e.roastSessionID). The member component isolates a multi-seat operator's sibling
// seats: they share a RoastSessionID and attemptHash but each stashes -- and
// broadcasts -- its OWN evidence, so neither overwrites the other.
type pendingEvidenceKey struct {
	sessionID   string
	member      group.MemberIndex
	attemptHash [attempt.MessageDigestLength]byte
}

type pendingEvidenceEntry struct {
	evidence attempt.Evidence
	// coordinatorProofs holds the coordinator-signed signing-package proof
	// envelope(s) the interactive path retained for the attempt (RFC-21 Phase 7.3
	// PR2b-2 step 2b). Empty for a coarse attempt. evidence and coordinatorProofs are
	// independent: a coarse attempt carries only evidence, while ONE interactive
	// failure can carry BOTH a coordinator proof (2b) and share-verification reject
	// evidence (7.3 share-blame) -- NextAttempt reads the categories independently.
	coordinatorProofs [][]byte
	createdAt         time.Time
}

var (
	pendingEvidenceMu       sync.Mutex
	pendingEvidenceRegistry = map[pendingEvidenceKey]pendingEvidenceEntry{}
)

// stashPendingEvidence stores a deep COPY of the captured evidence for
// (sessionID, member, attemptHash). A later call for the same key overwrites the
// earlier one (a re-driven attempt re-stashes). The deep copy means a later
// mutation of the caller's recorder snapshot cannot race the exchange's read.
func stashPendingEvidence(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
	evidence attempt.Evidence,
) {
	key := pendingEvidenceKey{sessionID, member, attemptHash}
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	// Upsert the evidence field, preserving any coordinator proofs already stashed
	// for this attempt. An interactive failure legitimately stashes BOTH coordinator
	// proofs (2b) and share-verification reject evidence (7.3 share-blame) for one
	// attempt, so preserving the sibling field is load-bearing -- never an XOR
	// assumption that would drop the other writer's data.
	entry := pendingEvidenceRegistry[key]
	entry.evidence = copyEvidence(evidence)
	entry.createdAt = time.Now()
	pendingEvidenceRegistry[key] = entry
}

// stashPendingCoordinatorProofs stores deep COPIES of the coordinator-signed
// signing-package proof envelope(s) the interactive runner's Round2Collector
// retained for (sessionID, member, attemptHash). BroadcastForcedSnapshot carries
// them in this seat's snapshot so the bundle's aggregated proofs let NextAttempt
// instant-exclude an equivocating coordinator (RFC-21 Phase 7.3 PR2b-2 step 2b).
// A no-op when proofs is empty. Like stashPendingEvidence it upserts only its own
// field, preserving any coarse evidence already stashed for the attempt.
func stashPendingCoordinatorProofs(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
	proofs [][]byte,
) {
	if len(proofs) == 0 {
		return
	}
	key := pendingEvidenceKey{sessionID, member, attemptHash}
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	entry := pendingEvidenceRegistry[key]
	entry.coordinatorProofs = copyProofs(proofs)
	entry.createdAt = time.Now()
	pendingEvidenceRegistry[key] = entry
}

// takePendingEvidence returns the stashed evidence AND coordinator proofs for
// (sessionID, member, attemptHash) plus a presence flag, REMOVING the entry
// (consume-on-read). The returned values are the sole references -- the entry is
// deleted from the registry -- so the caller owns them exclusively without a
// further copy. BroadcastForcedSnapshot calls it: a present entry means the coarse
// receive loop observed evidence (Evidence) and/or the interactive path retained
// coordinator-equivocation proofs ([][]byte) for the attempt; absent means neither
// was captured (the broadcast then carries an empty proof-of-attendance snapshot).
func takePendingEvidence(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
) (attempt.Evidence, [][]byte, bool) {
	key := pendingEvidenceKey{sessionID, member, attemptHash}
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	entry, ok := pendingEvidenceRegistry[key]
	if !ok {
		return attempt.Evidence{}, nil, false
	}
	delete(pendingEvidenceRegistry, key)
	return entry.evidence, entry.coordinatorProofs, true
}

// clearPendingEvidence removes any stashed evidence for (sessionID, member,
// attemptHash).
func clearPendingEvidence(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
) {
	key := pendingEvidenceKey{sessionID, member, attemptHash}
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	delete(pendingEvidenceRegistry, key)
}

// ClearPendingEvidenceOnLocalSuccess removes the stashed evidence for an attempt
// this seat completed successfully. A succeeded attempt produces no transition
// bundle, so BroadcastForcedSnapshot never consumes the stash; clearing here
// prevents a leak until the TTL backstop. Exported so the pkg/tbtc transition
// controller can call it alongside ClearObservedAttemptOnLocalSuccess (RFC-21
// Phase 7.3 PR2b-2).
func ClearPendingEvidenceOnLocalSuccess(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
) {
	clearPendingEvidence(sessionID, member, attemptHash)
}

// clearPendingEvidenceForSession removes every stashed evidence entry for
// (sessionID, member), regardless of attempt hash. The transition exchange calls
// it when the session ends (its listener context is done), mirroring
// clearObservedAttemptsForSession, so a signing whose attempts succeeded -- and
// therefore never consumed the stash via BroadcastForcedSnapshot -- does not leave
// entries behind.
func clearPendingEvidenceForSession(sessionID string, member group.MemberIndex) {
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	for key := range pendingEvidenceRegistry {
		if key.sessionID == sessionID && key.member == member {
			delete(pendingEvidenceRegistry, key)
		}
	}
}

// evictStalePendingEvidence sweeps the registry and removes entries older than
// maxAge. Exposed at the package level so tests can invoke it directly with small
// maxAge values and so the shared session-handle sweeper can fold it into one
// background goroutine: a session that ends abnormally must not orphan a stash
// entry past its backstop TTL.
func evictStalePendingEvidence(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	evicted := 0
	for key, entry := range pendingEvidenceRegistry {
		if entry.createdAt.Before(cutoff) {
			delete(pendingEvidenceRegistry, key)
			evicted++
		}
	}
	return evicted
}

// ResetPendingEvidenceRegistryForTest clears every stashed entry. Test-only seam;
// not for production code paths.
func ResetPendingEvidenceRegistryForTest() {
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	pendingEvidenceRegistry = map[pendingEvidenceKey]pendingEvidenceEntry{}
}

// PendingEvidenceStashedForTest reports whether any stash entry exists for
// (sessionID, member), regardless of attempt hash. Exported test seam so
// downstream-package tests can assert the stash without reaching into the
// unexported registry.
func PendingEvidenceStashedForTest(sessionID string, member group.MemberIndex) bool {
	pendingEvidenceMu.Lock()
	defer pendingEvidenceMu.Unlock()
	for key := range pendingEvidenceRegistry {
		if key.sessionID == sessionID && key.member == member {
			return true
		}
	}
	return false
}

// copyEvidence returns a deep copy of an attempt.Evidence: the three maps are
// re-allocated and the per-sender RejectEntry slices are cloned. RejectEntry is a
// flat value type (Reason string, Count uint), so a slice copy is a full deep
// copy. nil maps stay nil (a nil category means "did not fire", which
// NewLocalEvidenceSnapshot and NextAttempt both treat as empty).
func copyEvidence(e attempt.Evidence) attempt.Evidence {
	out := attempt.Evidence{}
	if e.Overflows != nil {
		out.Overflows = make(map[group.MemberIndex]uint, len(e.Overflows))
		for k, v := range e.Overflows {
			out.Overflows[k] = v
		}
	}
	if e.Rejects != nil {
		out.Rejects = make(map[group.MemberIndex][]attempt.RejectEntry, len(e.Rejects))
		for k, v := range e.Rejects {
			out.Rejects[k] = append([]attempt.RejectEntry(nil), v...)
		}
	}
	if e.Conflicts != nil {
		out.Conflicts = make(map[group.MemberIndex]uint, len(e.Conflicts))
		for k, v := range e.Conflicts {
			out.Conflicts[k] = v
		}
	}
	return out
}

// copyProofs returns a deep copy of a slice of proof envelopes: the outer slice
// and each inner []byte are re-allocated, so a later mutation of the caller's
// slice cannot race the exchange's read. nil/empty stays nil.
func copyProofs(proofs [][]byte) [][]byte {
	if len(proofs) == 0 {
		return nil
	}
	out := make([][]byte, len(proofs))
	for i, p := range proofs {
		out[i] = append([]byte(nil), p...)
	}
	return out
}

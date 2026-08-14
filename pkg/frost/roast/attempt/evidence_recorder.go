package attempt

import (
	"sync"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// OverflowQuotaDefault is the default per-sender overflow event quota
// enforced by NewBoundedRecorder. It matches the categoryQuota.Overflow
// value documented in RFC-21 Layer A.
//
// A peer that overflows the inbound message channel more than the
// quota allows in a single attempt is recorded only up to the quota:
// further overflows are silently dropped by the recorder. This bounds
// the per-attempt evidence size to O(|IncludedSet| * quota) regardless
// of how aggressively a peer (or its network link) misbehaves.
const OverflowQuotaDefault uint = 8

// RejectQuotaDefault is the default per-sender reject event quota.
// Matches categoryQuota.Reject in RFC-21 Layer A. A reject event is
// recorded each time a peer's payload fails the validation gate
// (shouldAcceptNativeFROSTMessage returning false), regardless of
// the specific reason.
const RejectQuotaDefault uint = 8

// ConflictQuotaDefault is the default per-sender conflict event
// quota. Matches categoryQuota.Conflict in RFC-21 Layer A. A
// conflict event is recorded when a peer retransmits a message for
// a sender slot that already holds a byte-different contribution
// (first-write-wins reject).
const ConflictQuotaDefault uint = 4

// EvidenceRecorder collects bounded, per-attempt evidence of receive-
// path anomalies that the ROAST coordinator's exclusion policy may
// later consume.
//
// The interface tracks three categories of evidence:
//   - Overflow: payload arrived but the inbound channel was full.
//   - Reject: payload arrived but failed validation
//     (shouldAcceptNativeFROSTMessage returning false).
//   - Conflict: a peer's later retransmission disagreed with its
//     earlier contribution for the same slot (equivocation
//     signal).
//
// Silence -- peers in the IncludedSet that produced no snapshot at
// all -- is derived implicitly by the NextAttempt policy from
// (ctx.IncludedSet - bundleSenders) and does not need a recorder
// method.
//
// Implementations must be safe for concurrent calls from multiple
// goroutines, since the receive-callback closure in pkg/frost/signing
// is driven by network goroutines.
type EvidenceRecorder interface {
	// RecordOverflow notes that the inbound message channel was full
	// when a payload from the named sender arrived, causing the
	// payload to be dropped at the receive callback. The recorder
	// applies its own quota; callers do not need to suppress at the
	// call site.
	RecordOverflow(sender group.MemberIndex)
	// RecordReject notes that a payload from the named sender failed
	// the validation gate (typically shouldAcceptNativeFROSTMessage
	// returning false). The reason string is preserved verbatim in
	// the snapshot so the coordinator's exclusion policy can later
	// route by reason if needed; the recorder applies its own
	// per-sender quota regardless of reason.
	RecordReject(sender group.MemberIndex, reason string)
	// RecordConflict notes that a peer retransmitted a message for
	// a sender slot that already holds a byte-different contribution
	// (equivocation signal under the first-write-wins assembly
	// policy).
	RecordConflict(sender group.MemberIndex)
	// Snapshot returns a copy of the recorded evidence so far. The
	// returned value does not alias internal state; the recorder may
	// continue receiving events after Snapshot is called.
	Snapshot() Evidence
}

// RejectEntry describes a single per-sender reject event recorded
// during an attempt. The reason captures *why* the validation gate
// rejected the payload; the coordinator's exclusion policy treats
// every distinct reason as equally blamable today, but the field
// is kept structured so future policy refinements can differentiate.
type RejectEntry struct {
	Reason string
	Count  uint
}

// Evidence is the per-attempt snapshot of receive-path anomalies
// captured by an EvidenceRecorder. It is the value the ROAST
// coordinator's NextAttempt policy consumes to derive the next
// attempt's ExcludedSet.
//
// Maps are nil-safe in callers: an absent key means the category
// did not fire for that sender, count zero.
type Evidence struct {
	// Overflows maps each sender to the number of overflow events
	// observed for that sender during the attempt, saturated at the
	// recorder's overflow quota.
	Overflows map[group.MemberIndex]uint
	// Rejects maps each sender to a per-reason set of reject entries.
	// The outer map's key is the sender; the inner slice carries one
	// entry per distinct reason, with Count saturated at the
	// recorder's reject quota.
	Rejects map[group.MemberIndex][]RejectEntry
	// Conflicts maps each sender to the number of first-write-wins
	// conflict events observed during the attempt, saturated at the
	// recorder's conflict quota.
	Conflicts map[group.MemberIndex]uint
}

// NewBoundedRecorder returns an EvidenceRecorder with default
// per-sender quotas across all three categories. The recorder is
// safe for concurrent use.
func NewBoundedRecorder() EvidenceRecorder {
	return NewBoundedRecorderWithQuotas(
		OverflowQuotaDefault,
		RejectQuotaDefault,
		ConflictQuotaDefault,
	)
}

// NewBoundedRecorderWithQuota returns a recorder with a custom
// overflow quota; reject and conflict quotas use their defaults.
// Preserved as the Phase-2 entry point so existing callers do not
// need to update.
func NewBoundedRecorderWithQuota(overflowQuota uint) EvidenceRecorder {
	return NewBoundedRecorderWithQuotas(
		overflowQuota,
		RejectQuotaDefault,
		ConflictQuotaDefault,
	)
}

// NewBoundedRecorderWithQuotas returns a recorder with custom
// per-category quotas. Intended for tests; production callers
// should use NewBoundedRecorder so the per-attempt evidence size
// is uniform across the network.
func NewBoundedRecorderWithQuotas(
	overflowQuota, rejectQuota, conflictQuota uint,
) EvidenceRecorder {
	return &boundedRecorder{
		overflowQuota: overflowQuota,
		rejectQuota:   rejectQuota,
		conflictQuota: conflictQuota,
		overflows:     map[group.MemberIndex]uint{},
		rejects:       map[group.MemberIndex]map[string]uint{},
		conflicts:     map[group.MemberIndex]uint{},
	}
}

// NoOpRecorder returns a recorder that discards every event and
// reports an empty Evidence on Snapshot. It is the default at
// every receive-loop call site when the ROAST-retry registry is
// not populated, so the receive loops' observable behaviour stays
// identical to pre-Phase-2 until a real recorder is wired.
func NoOpRecorder() EvidenceRecorder {
	return noOpRecorder{}
}

type boundedRecorder struct {
	mu            sync.Mutex
	overflowQuota uint
	rejectQuota   uint
	conflictQuota uint
	overflows     map[group.MemberIndex]uint
	// rejects[sender][reason] = count. The two-level map keeps each
	// reason bucket bounded by rejectQuota independently so a peer
	// cannot saturate one reason to mask another (RFC-21 Layer A:
	// "a peer cannot spam overflow events to drown out reject
	// evidence or vice-versa"; the same principle applies within
	// reject reasons).
	rejects   map[group.MemberIndex]map[string]uint
	conflicts map[group.MemberIndex]uint
}

func (r *boundedRecorder) RecordOverflow(sender group.MemberIndex) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.overflows[sender] < r.overflowQuota {
		r.overflows[sender]++
	}
}

func (r *boundedRecorder) RecordReject(
	sender group.MemberIndex,
	reason string,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bySender, ok := r.rejects[sender]
	if !ok {
		bySender = map[string]uint{}
		r.rejects[sender] = bySender
	}
	if bySender[reason] < r.rejectQuota {
		bySender[reason]++
	}
}

func (r *boundedRecorder) RecordConflict(sender group.MemberIndex) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conflicts[sender] < r.conflictQuota {
		r.conflicts[sender]++
	}
}

func (r *boundedRecorder) Snapshot() Evidence {
	r.mu.Lock()
	defer r.mu.Unlock()
	overflows := make(map[group.MemberIndex]uint, len(r.overflows))
	for sender, count := range r.overflows {
		overflows[sender] = count
	}
	rejects := make(
		map[group.MemberIndex][]RejectEntry,
		len(r.rejects),
	)
	for sender, reasons := range r.rejects {
		entries := make([]RejectEntry, 0, len(reasons))
		for reason, count := range reasons {
			entries = append(entries, RejectEntry{
				Reason: reason,
				Count:  count,
			})
		}
		rejects[sender] = entries
	}
	conflicts := make(map[group.MemberIndex]uint, len(r.conflicts))
	for sender, count := range r.conflicts {
		conflicts[sender] = count
	}
	return Evidence{
		Overflows: overflows,
		Rejects:   rejects,
		Conflicts: conflicts,
	}
}

type noOpRecorder struct{}

func (noOpRecorder) RecordOverflow(group.MemberIndex)       {}
func (noOpRecorder) RecordReject(group.MemberIndex, string) {}
func (noOpRecorder) RecordConflict(group.MemberIndex)       {}

func (noOpRecorder) Snapshot() Evidence {
	return Evidence{
		Overflows: map[group.MemberIndex]uint{},
		Rejects:   map[group.MemberIndex][]RejectEntry{},
		Conflicts: map[group.MemberIndex]uint{},
	}
}

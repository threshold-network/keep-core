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

// EvidenceRecorder collects bounded, per-attempt evidence of receive-
// path anomalies that the ROAST coordinator's exclusion policy may
// later consume.
//
// Phase 2 introduces only the overflow channel; future phases extend
// the interface with separate methods for reject events, first-write-
// wins conflicts, and silent peers.
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
	// Snapshot returns a copy of the recorded evidence so far. The
	// returned value does not alias internal state; the recorder may
	// continue receiving events after Snapshot is called.
	Snapshot() Evidence
}

// Evidence is the per-attempt snapshot of receive-path anomalies
// captured by an EvidenceRecorder. It is the value the ROAST
// coordinator's NextAttempt policy consumes (in a later RFC-21
// phase) to derive the next attempt's ExcludedSet.
type Evidence struct {
	// Overflows maps each sender to the number of overflow events
	// observed for that sender during the attempt, saturated at the
	// recorder's overflow quota. A missing key means the sender did
	// not overflow at all during the attempt.
	Overflows map[group.MemberIndex]uint
}

// NewBoundedRecorder returns an EvidenceRecorder with default
// per-sender quotas. The recorder is safe for concurrent use.
//
// Phase 2 wiring uses NoOpRecorder by default at every call site;
// real use of the bounded recorder lands in a later phase behind a
// build tag, when the coordinator state machine arrives.
func NewBoundedRecorder() EvidenceRecorder {
	return NewBoundedRecorderWithQuota(OverflowQuotaDefault)
}

// NewBoundedRecorderWithQuota returns a recorder with a custom
// overflow quota. Intended for tests; production callers should use
// NewBoundedRecorder so the per-attempt evidence size is uniform
// across the network.
func NewBoundedRecorderWithQuota(overflowQuota uint) EvidenceRecorder {
	return &boundedRecorder{
		overflowQuota: overflowQuota,
		overflows:     map[group.MemberIndex]uint{},
	}
}

// NoOpRecorder returns a recorder that discards every event and
// reports an empty Evidence on Snapshot. It is the default at every
// Phase 2 call site so the receive loops' observable behaviour stays
// identical to pre-Phase-2 until a later phase wires real recorders.
func NoOpRecorder() EvidenceRecorder {
	return noOpRecorder{}
}

type boundedRecorder struct {
	mu            sync.Mutex
	overflowQuota uint
	overflows     map[group.MemberIndex]uint
}

func (r *boundedRecorder) RecordOverflow(sender group.MemberIndex) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.overflows[sender] < r.overflowQuota {
		r.overflows[sender]++
	}
}

func (r *boundedRecorder) Snapshot() Evidence {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[group.MemberIndex]uint, len(r.overflows))
	for sender, count := range r.overflows {
		out[sender] = count
	}
	return Evidence{Overflows: out}
}

type noOpRecorder struct{}

func (noOpRecorder) RecordOverflow(group.MemberIndex) {}

func (noOpRecorder) Snapshot() Evidence {
	return Evidence{Overflows: map[group.MemberIndex]uint{}}
}

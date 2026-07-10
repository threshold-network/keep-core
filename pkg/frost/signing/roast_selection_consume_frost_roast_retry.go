//go:build frost_roast_retry

package signing

import (
	"errors"
	"fmt"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ErrRoastSelectionFallBackToLegacy signals that ROAST-driven participant
// selection does not apply to this attempt and the caller must use the legacy
// retry selection. It is a UNIFORM, deterministic decision every honest node
// makes identically -- the initial attempt, or ROAST retry not active -- so a
// legacy fallback on this sentinel never fractures the group. It is NOT a
// fail-closed condition.
var ErrRoastSelectionFallBackToLegacy = errors.New(
	"roast selection: not applicable; use legacy selection",
)

// ConsumeRoastTransitionForSelection computes the next attempt's included set
// from this seat's stored transition record (RFC-21 Phase 7.3 PR2b-1b C3 -- the
// activation of the cross-attempt ROAST retry path).
//
// Returns one of:
//   - (includedSet, parked, nil): a FRESH transition record drove the next
//     attempt; the caller uses includedSet verbatim (member-level) AND carries
//     parked so the attempt after reinstates the parked members.
//   - (nil, nil, ErrRoastSelectionFallBackToLegacy): the initial ROAST attempt,
//     or ROAST retry is not active -- a UNIFORM legacy fallback (deterministic
//     across honest nodes, so non-fracturing).
//   - (nil, nil, any other error): a committed ROAST attempt EXPECTED a
//     transition but no FRESH record exists, or NextAttempt failed. The caller
//     MUST FAIL CLOSED (terminate the retry loop), NEVER fall back to legacy: a
//     node that selected legacy while peers selected from the transition would
//     split the signing group into divergent included sets -- the fracture class.
//
// The "a transition is expected" predicate is deterministic group-wide: every
// honest node, with the same (uniformly deployed) gating, agrees that
// roastAttemptNumber > 0 under active ROAST retry expects a transition. VerifyBundle
// is NOT called here -- the transition listener already verified the bundle before
// storing the record, so the selector consumes only verified records.
//
// threshold is the FROST signing threshold t for the key group, used by
// NextAttempt's infeasibility check (the next included set must stay at or above
// t). For tBTC wallets the group's honest threshold IS that signing threshold,
// so the caller passes it; if the two ever diverge for a key group, the
// authoritative value is the persisted DKGThreshold.
// roastAttemptNumber is the 0-based COMMITTED ROAST attempt index (advanced only
// by observed attempts, decoupled from the block-paced loop attempt counter), so
// skipped loop iterations do not break the consecutive-transition chain. It
// returns the next attempt's included set AND its transiently-parked set; the
// caller MUST carry the parking, or a one-attempt park becomes permanent.
func ConsumeRoastTransitionForSelection(
	roastSessionID string,
	member group.MemberIndex,
	roastAttemptNumber uint,
	threshold uint,
) (included []group.MemberIndex, transientlyParked []group.MemberIndex, err error) {
	// The initial ROAST attempt has no prior transition: uniform legacy/initial
	// selection.
	if roastAttemptNumber == 0 {
		return nil, nil, ErrRoastSelectionFallBackToLegacy
	}

	// The legacy-fallback decision is PROCESS-level (group-uniform), NOT per-member:
	// readiness opted out, NO coordinator registered ANYWHERE in this process, or no
	// transition producer built in (frost_roast_retry && !frost_native) -> a uniform
	// legacy fallback every honest node makes identically. It must NOT be
	// RoastRetryActiveForMember here: a multi-seat operator with member A registered
	// and member B not would otherwise drive A via the transition and B via the
	// legacy shuffle for the SAME attempt -> divergent included sets (fracture). The
	// fallback sentinel is only safe when uniform (RFC-21 Phase 7.3 PR2b-1.5, Codex
	// P2-1).
	if !RoastRetryActive() {
		return nil, nil, ErrRoastSelectionFallBackToLegacy
	}
	// Coarse partial-registration gate (BOOLEAN only): THIS seat must be
	// ROAST-registered under SOME key group. A totally-unregistered seat under active
	// ROAST is partial registration (a wiring bug) and FAILS CLOSED -- never legacy:
	// falling back to legacy here while registered sibling seats select from the
	// transition would split the included set (the fracture the sentinel must not
	// enable). This scan must NOT supply the coordinator used below: on a multi-wallet
	// node it could return a DIFFERENT wallet's entry (seat indices reuse 1..N); the
	// coordinator that actually drives NextAttempt is resolved per key group.
	if _, ok := RegisteredRoastRetryCoordinatorForMember(member); !ok {
		return nil, nil, fmt.Errorf(
			"roast selection: seat %d has no registered coordinator under active ROAST retry; fail closed",
			member,
		)
	}

	// A retry expects a transition from the prior COMMITTED attempt of THIS wallet's
	// session; its absence under active ROAST is fail-closed, never a legacy fallback.
	// The record also carries the wallet's key group, which scopes the authoritative
	// coordinator lookup below.
	record, ok := RoastTransitionForSession(roastSessionID, member)
	if !ok {
		return nil, nil, fmt.Errorf(
			"roast selection: no transition record for roast attempt %d; fail closed",
			roastAttemptNumber,
		)
	}

	// Authoritative, wallet-scoped coordinator: record.PreviousHandle was minted by
	// THIS wallet's coordinator, so NextAttempt must run on the same instance. The
	// seat-only scan above could name a different wallet's coordinator on a
	// multi-wallet node, and NextAttempt would then reject the foreign handle
	// (ErrUnknownAttempt) -- fail-closed but wrong-for-the-reason. Look it up by the
	// record's key group so the right coordinator drives selection.
	deps, ok := RegisteredRoastRetryCoordinatorForKeyGroupMember(
		record.PreviousContext.KeyGroupID, member,
	)
	if !ok || deps.Coordinator == nil {
		return nil, nil, fmt.Errorf(
			"roast selection: seat %d has no coordinator for the transition's key group under active ROAST retry; fail closed",
			member,
		)
	}

	// Freshness: the record must describe the IMMEDIATELY prior committed attempt.
	// The previous attempt's 0-based AttemptNumber plus one must equal this
	// roast attempt number. A stale record (e.g. a missed intervening transition)
	// must not drive selection -- fail closed.
	if uint(record.PreviousContext.AttemptNumber)+1 != roastAttemptNumber {
		return nil, nil, fmt.Errorf(
			"roast selection: stale transition record (prev attempt %d, expected %d); fail closed",
			record.PreviousContext.AttemptNumber, roastAttemptNumber-1,
		)
	}

	nextContext, nextErr := deps.Coordinator.NextAttempt(
		record.PreviousHandle,
		record.Bundle,
		threshold,
		record.DkgGroupPublicKey,
	)
	if nextErr != nil {
		// Includes ErrAttemptInfeasible (the next included set would drop below
		// threshold): the session cannot make progress -- fail closed.
		return nil, nil, fmt.Errorf("roast selection: next attempt: %w", nextErr)
	}

	// Defensive: the derived attempt number must match the roast attempt we select
	// for (NextAttempt derives prev+1, which equals roastAttemptNumber given the
	// freshness check above; re-assert so any drift fails closed rather than
	// mis-selects).
	if uint(nextContext.AttemptNumber) != roastAttemptNumber {
		return nil, nil, fmt.Errorf(
			"roast selection: derived attempt %d does not match roast attempt %d; fail closed",
			nextContext.AttemptNumber, roastAttemptNumber,
		)
	}

	return nextContext.IncludedSet, nextContext.TransientlyParked, nil
}

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
	// ROAST retry is active for the process, so THIS seat must have its own
	// registered coordinator. A missing one is partial registration (a wiring bug)
	// and FAILS CLOSED -- never legacy: falling back to legacy here while the
	// registered sibling seats select from the transition would split the included
	// set (the fracture class the sentinel must not enable).
	deps, ok := RegisteredRoastRetryCoordinatorForMember(member)
	if !ok || deps.Coordinator == nil {
		return nil, nil, fmt.Errorf(
			"roast selection: seat %d has no registered coordinator under active ROAST retry; fail closed",
			member,
		)
	}

	// From here a transition from the prior COMMITTED attempt IS expected; its
	// absence is fail-closed, never a legacy fallback.
	record, ok := RoastTransitionForSession(roastSessionID, member)
	if !ok {
		return nil, nil, fmt.Errorf(
			"roast selection: no transition record for roast attempt %d; fail closed",
			roastAttemptNumber,
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

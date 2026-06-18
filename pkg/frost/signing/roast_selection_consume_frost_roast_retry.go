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
//   - (includedSet, nil): a FRESH transition record drove the next attempt; the
//     caller uses includedSet verbatim (member-level, no address round-trip).
//   - (nil, ErrRoastSelectionFallBackToLegacy): the initial attempt, or ROAST
//     retry is not active -- a UNIFORM legacy fallback (deterministic across
//     honest nodes, so non-fracturing).
//   - (nil, any other error): a committed ROAST attempt EXPECTED a transition
//     but no FRESH record exists, or NextAttempt failed. The caller MUST FAIL
//     CLOSED (terminate the retry loop), NEVER fall back to legacy: a node that
//     selected legacy while peers selected from the transition would split the
//     signing group into divergent included sets -- the fracture class.
//
// The "a transition is expected" predicate is deterministic group-wide: every
// honest node, with the same (uniformly deployed) gating, agrees that retry > 0
// under active ROAST retry expects a transition. VerifyBundle is NOT called here
// -- the transition listener already verified the bundle before storing the
// record, so the selector consumes only verified records.
//
// threshold is the FROST signing threshold t for the key group, used by
// NextAttempt's infeasibility check (the next included set must stay at or above
// t). For tBTC wallets the group's honest threshold IS that signing threshold,
// so the caller passes it; if the two ever diverge for a key group, the
// authoritative value is the persisted DKGThreshold.
func ConsumeRoastTransitionForSelection(
	roastSessionID string,
	member group.MemberIndex,
	retryCount uint,
	threshold uint,
) ([]group.MemberIndex, error) {
	// The initial attempt has no prior transition: uniform legacy/initial
	// selection.
	if retryCount == 0 {
		return nil, ErrRoastSelectionFallBackToLegacy
	}

	// ROAST retry inactive (readiness opted out or no coordinator registered):
	// a uniform legacy fallback. This MUST mirror the observe/exchange gating, so
	// a node that produced no records also does not expect to consume one.
	if err := EnsureRoastRetryReadinessOptIn(); err != nil {
		return nil, ErrRoastSelectionFallBackToLegacy
	}
	deps, ok := RegisteredRoastRetryCoordinator()
	if !ok || deps.Coordinator == nil {
		return nil, ErrRoastSelectionFallBackToLegacy
	}

	// From here a transition from the prior attempt IS expected; its absence is
	// fail-closed, never a legacy fallback.
	record, ok := RoastTransitionForSession(roastSessionID, member)
	if !ok {
		return nil, fmt.Errorf(
			"roast selection: no transition record for retry %d; fail closed",
			retryCount,
		)
	}

	// Freshness: the record must describe the IMMEDIATELY prior attempt. The
	// previous attempt's 0-based AttemptNumber plus one must equal this retry
	// count (also 0-based). A stale record (e.g. a missed intervening
	// transition) must not drive selection -- fail closed.
	if uint(record.PreviousContext.AttemptNumber)+1 != retryCount {
		return nil, fmt.Errorf(
			"roast selection: stale transition record (prev attempt %d, expected %d); fail closed",
			record.PreviousContext.AttemptNumber, retryCount-1,
		)
	}

	nextContext, err := deps.Coordinator.NextAttempt(
		record.PreviousHandle,
		record.Bundle,
		threshold,
		record.DkgGroupPublicKey,
	)
	if err != nil {
		// Includes ErrAttemptInfeasible (the next included set would drop below
		// threshold): the session cannot make progress -- fail closed.
		return nil, fmt.Errorf("roast selection: next attempt: %w", err)
	}

	// Defensive: the derived attempt number must match the retry we select for
	// (NextAttempt derives prev+1, which equals retryCount given the freshness
	// check above; re-assert so any drift fails closed rather than mis-selects).
	if uint(nextContext.AttemptNumber) != retryCount {
		return nil, fmt.Errorf(
			"roast selection: derived attempt %d does not match retry %d; fail closed",
			nextContext.AttemptNumber, retryCount,
		)
	}

	return nextContext.IncludedSet, nil
}

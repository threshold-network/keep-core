package signing

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// RoastRetryReadinessOptInEnvVar is the environment variable name
// operators must set to "true" to opt in to RFC-21 ROAST retry
// activation. The variable is read per call -- not cached -- so an
// operator can flip it during a debugging session without
// restarting the node.
//
// Pattern matches the existing
// KEEP_CORE_FROST_TBTC_SIGNER_ACCEPT_SCAFFOLD_KEY_GROUP env var
// from PR #3960: a build tag enables the code path, an env var
// enables the wiring, both must agree for the feature to be live.
const RoastRetryReadinessOptInEnvVar = "KEEP_CORE_FROST_ROAST_RETRY_ENABLED"

// ErrRoastRetryReadinessOptOut is the error
// EnsureRoastRetryReadinessOptIn returns when the env var is unset
// or set to anything other than "true". Use errors.Is to detect.
var ErrRoastRetryReadinessOptOut = errors.New(
	"roast retry readiness: operator opt-in env var is not set to true",
)

// EnsureRoastRetryReadinessOptIn reads the
// RoastRetryReadinessOptInEnvVar environment variable and returns
// nil if it is set to the string "true" (case-insensitive,
// whitespace-trimmed). Returns ErrRoastRetryReadinessOptOut
// otherwise.
//
// Callers in the orchestration layer invoke this before
// RegisterRoastRetryCoordinator so production builds with the
// frost_roast_retry build tag still refuse to wire orchestration
// without an explicit operator decision.
//
// The function is per-call (not cached) so operators can flip the
// env var dynamically during debugging.
func EnsureRoastRetryReadinessOptIn() error {
	if !RoastRetryReadinessOptInEnabled() {
		return fmt.Errorf(
			"%w: set %s=true to enable",
			ErrRoastRetryReadinessOptOut,
			RoastRetryReadinessOptInEnvVar,
		)
	}
	return nil
}

// RoastRetryReadinessOptInEnabled reports whether the readiness
// env var is currently set to "true". Cheap to call; use this when
// you need a boolean (e.g., to gate a log message) and
// EnsureRoastRetryReadinessOptIn when you need an error.
func RoastRetryReadinessOptInEnabled() bool {
	value := strings.TrimSpace(os.Getenv(RoastRetryReadinessOptInEnvVar))
	return strings.EqualFold(value, "true")
}

// RoastRetryActive reports whether ROAST retry orchestration is runtime-active:
// the readiness opt-in is set, a coordinator is registered, AND this build
// contains the transition producer (frost_native). It is the deterministic,
// process-level gate every honest node evaluates identically (env var +
// in-process registration + build), so the signing loop and the signing executor
// agree on whether to key the active attempt off the COMMITTED ROAST attempt
// index (roastAttemptNumber) rather than the block-paced attemptCounter -- RFC-21
// Phase 7.3 PR2b-1b. The participant selector gates on the same predicate, so
// selection, observe, and the active signing context stay consistent.
//
// The producer requirement matters in a frost_roast_retry && !frost_native build:
// there the selector and the registry exist but nothing PRODUCES transition
// records, so without this check a retry would fail-close against a record that
// can never be created instead of using the uniform legacy shuffle (Codex P2-1).
// Always false in builds without the frost_roast_retry tag (the registration and
// producer default stubs both report unavailable).
// readinessAndProducerReady is the build+env prefix shared by RoastRetryActive and
// RoastRetryActiveForMember: the readiness opt-in is set AND the transition producer
// is built in (frost_native). Both gates additionally require a registered
// coordinator (any entry / the specific member's).
func readinessAndProducerReady() bool {
	return RoastRetryReadinessOptInEnabled() && roastTransitionProducerAvailable()
}

func RoastRetryActive() bool {
	if !readinessAndProducerReady() {
		return false
	}
	_, ok := RegisteredRoastRetryCoordinator()
	return ok
}

// RoastRetryActiveForMember reports whether ROAST retry is runtime-active for a
// SPECIFIC local seat: readiness opt-in AND the producer is built in AND THIS
// member has a coordinator registered. Member-aware paths (the per-seat signing
// loop, the per-member selector, observe, and the exchange) use it so a multi-seat
// operator activates ROAST per seat -- a seat with no registered coordinator stays
// on the legacy path rather than fail-closing. Always false in builds without the
// frost_roast_retry tag (the per-member registration default stub reports
// not-registered). RFC-21 Phase 7.3 PR2b-1.5.
func RoastRetryActiveForMember(member group.MemberIndex) bool {
	if !readinessAndProducerReady() {
		return false
	}
	_, ok := RegisteredRoastRetryCoordinatorForMember(member)
	return ok
}

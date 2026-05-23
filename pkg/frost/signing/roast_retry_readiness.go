package signing

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

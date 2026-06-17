package signing

import (
	"os"
	"strings"
)

// InteractiveSigningOptInEnvVar is the operator audit gate for RFC-21 Phase 7.3
// interactive ROAST signing. It is DISTINCT from the orchestration readiness
// gate (KEEP_CORE_FROST_ROAST_RETRY_ENABLED, see roast_retry_readiness.go):
// enabling ROAST retry orchestration must NOT, on its own, activate the
// interactive signing runner driving the real cgo engine. Interactive signing
// requires BOTH gates on plus a registered engine, so an operator who opts in
// to retry orchestration does not silently start exercising the (separately
// audited) interactive FROST path.
//
// This gate stays OFF by default and is intended to remain off in production
// until the frost-secp256k1-tr engine external audit clears. While off, the
// executor uses the coarse signing path; the interactive code still compiles
// and is exercised under test with a fake engine.
//
// The variable is read per call -- not cached -- so an operator can flip it
// during a debugging session without restarting the node, matching the
// readiness gate's convention.
const InteractiveSigningOptInEnvVar = "KEEP_CORE_FROST_INTERACTIVE_SIGNING_ENABLED"

// InteractiveSigningOptInEnabled reports whether the interactive-signing audit
// gate is currently set to "true" (case-insensitive, whitespace-trimmed).
// Cheap to call.
func InteractiveSigningOptInEnabled() bool {
	value := strings.TrimSpace(os.Getenv(InteractiveSigningOptInEnvVar))
	return strings.EqualFold(value, "true")
}

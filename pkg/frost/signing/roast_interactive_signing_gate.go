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

// InteractiveSigningOnlyEnvVar is the no-coarse-fallback half of coarse-path
// retirement (RFC-21 Phase 7.3). When set to "true", the executor REFUSES to fall
// through to the coarse signing primitive: interactive signing is mandatory, and a
// session where it does not run fails CLOSED rather than silently signing via the
// retired coarse path. It is meant to be set ONLY together with the audit gate above
// (and a registered engine) - setting it on its own makes signing fail closed.
//
// It stays OFF by default and is intended to remain off in production until the
// frost-secp256k1-tr engine external audit clears and the tECDSA->FROST cutover is
// made: flipping it on IS that cutover for this node (the coarse path is no longer
// available as a fallback). Read per call, not cached, matching the audit gate.
//
// SCOPE: it presumes the node signs EXCLUSIVELY via the interactive tBTC-FROST path.
// The refusal is format-agnostic (it fails closed for every signer format the native
// executor handles, not only the tBTC-signer one); it closes BOTH the inner FFI coarse
// primitive and the outer legacy fallbacks (the latter via nativeExecutionFallbackAllowed);
// and in a build WITHOUT the interactive engine (no frost_native) it fails all native
// signing closed. Enable it only on a node running the frost_native interactive engine
// with the audit gate on.
const InteractiveSigningOnlyEnvVar = "KEEP_CORE_FROST_INTERACTIVE_SIGNING_ONLY"

// InteractiveSigningOnlyEnabled reports whether interactive-only (no coarse
// fallback) mode is currently set to "true" (case-insensitive, whitespace-trimmed).
func InteractiveSigningOnlyEnabled() bool {
	value := strings.TrimSpace(os.Getenv(InteractiveSigningOnlyEnvVar))
	return strings.EqualFold(value, "true")
}

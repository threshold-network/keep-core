//go:build !frost_native

package signing

import "github.com/ipfs/go-log/v2"

// attemptRoastRetryOrchestrationFromRequest is the executor-adapter
// entry point for RFC-21 Phase-6 ROAST orchestration. In the
// default build (no frost_native tag) it is a permanent no-op
// stub: orchestration cannot run without the frost_native code
// path, so the executor adapter behaves exactly as in Phase 5.
//
// The function returns (cleanup, error). cleanup is non-nil when
// orchestration started successfully; the executor adapter defers
// it. error is non-nil only for RUNTIME failures the executor
// must propagate to its caller (static-configuration errors are
// logged and the cleanup is returned nil to signal "no
// orchestration; fall back to legacy receive-loop semantics").
//
// The default-build stub returns (nil, nil) unconditionally.
func attemptRoastRetryOrchestrationFromRequest(
	_ *NativeExecutionFFISigningRequest,
	_ log.StandardLogger,
) (func(), error) {
	return nil, nil
}

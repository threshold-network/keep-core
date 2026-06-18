//go:build !frost_native

package signing

import (
	"context"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
)

// attemptRoastRetryOrchestrationFromRequest is the executor-adapter
// entry point for RFC-21 Phase-6 ROAST orchestration. In the
// default build (no frost_native tag) it is a permanent no-op
// stub: orchestration cannot run without the frost_native code
// path, so the executor adapter behaves exactly as in Phase 5.
//
// The function returns (signature, cleanup, error). In the
// frost_native build a non-nil signature means interactive signing
// produced it; cleanup is non-nil when orchestration started (the
// executor defers it); error is non-nil only for RUNTIME/committed
// failures the executor must propagate.
//
// The default-build stub returns (nil, nil, nil) unconditionally.
func attemptRoastRetryOrchestrationFromRequest(
	_ context.Context,
	_ *NativeExecutionFFISigningRequest,
	_ log.StandardLogger,
) (*frost.Signature, func(), error) {
	return nil, nil, nil
}

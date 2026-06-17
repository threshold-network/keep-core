package signing

import (
	"context"
	"testing"

	"github.com/ipfs/go-log/v2"
)

func TestAttemptRoastRetryOrchestrationFromRequest_DefaultBuildIsNoOp(t *testing.T) {
	// In the default build, the helper is a permanent stub returning
	// (nil, nil, nil) so the executor adapter behaves exactly as in
	// Phase 5: no orchestration, no signature, no error, no cleanup deferred.
	//
	// The tagged-build test surface
	// (roast_retry_executor_entry_frost_native_test.go) exercises
	// the real branching.
	_, cleanup, err := attemptRoastRetryOrchestrationFromRequest(
		context.Background(), &NativeExecutionFFISigningRequest{SessionID: "x"},
		log.Logger("test"),
	)
	if err != nil {
		t.Fatalf("default-build helper must not return an error; got %v", err)
	}
	if cleanup != nil {
		t.Fatal("default-build helper must not return a cleanup function")
	}
}

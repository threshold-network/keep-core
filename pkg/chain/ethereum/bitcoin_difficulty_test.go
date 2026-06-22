package ethereum

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubBlockCounter reports a fixed current block height, simulating a chain that
// has not yet reached the requested height (e.g. stalled, or an RPC stuck behind
// a stale load balancer).
type stubBlockCounter struct {
	current uint64
}

func (b *stubBlockCounter) CurrentBlock() (uint64, error)               { return b.current, nil }
func (b *stubBlockCounter) WatchBlocks(_ context.Context) <-chan uint64 { return nil }
func (b *stubBlockCounter) BlockHeightWaiter(_ uint64) (<-chan uint64, error) {
	return nil, nil
}
func (b *stubBlockCounter) WaitForBlockHeight(_ uint64) error { return nil }

// TestWaitForBlockHeightCtx_DeadlineExceeded asserts that the wait honors the
// parent context's deadline when the chain never reaches the target height.
// Regression: the original implementation called WaitForBlockHeight without any
// cancellation path, which could hang the maintainer indefinitely if the chain
// stalled mid-retarget. See waitDeployBackendTransactionMinedTimeout.
func TestWaitForBlockHeightCtx_DeadlineExceeded(t *testing.T) {
	bc := &stubBlockCounter{current: 0}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := waitForBlockHeightCtx(ctx, bc, 100)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("wait took too long: %v", elapsed)
	}
}

// TestWaitForBlockHeightCtx_ReturnsImmediatelyOnSuccess asserts the wait does
// not introduce extra latency when the chain has already reached the target
// height.
func TestWaitForBlockHeightCtx_ReturnsImmediatelyOnSuccess(t *testing.T) {
	bc := &stubBlockCounter{current: 1}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := waitForBlockHeightCtx(ctx, bc, 1); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

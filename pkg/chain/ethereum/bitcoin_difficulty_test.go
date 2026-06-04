package ethereum

import (
	"context"
	"errors"
	"testing"
	"time"
)

// blockingBlockCounter blocks WaitForBlockHeight until ch is closed, simulating
// a chain that has stopped producing blocks (or an RPC stuck behind a stale
// load balancer).
type blockingBlockCounter struct {
	ch chan struct{}
}

func (b *blockingBlockCounter) CurrentBlock() (uint64, error)               { return 0, nil }
func (b *blockingBlockCounter) WatchBlocks(_ context.Context) <-chan uint64 { return nil }
func (b *blockingBlockCounter) BlockHeightWaiter(_ uint64) (<-chan uint64, error) {
	return nil, nil
}
func (b *blockingBlockCounter) WaitForBlockHeight(_ uint64) error {
	<-b.ch
	return nil
}

// TestWaitForBlockHeightCtx_DeadlineExceeded asserts that the context shim
// honors the parent context's deadline when WaitForBlockHeight blocks forever.
// Regression: the original implementation called WaitForBlockHeight without any
// cancellation path, which could hang the maintainer indefinitely if the chain
// stalled mid-retarget. See waitDeployBackendTransactionMinedTimeout.
func TestWaitForBlockHeightCtx_DeadlineExceeded(t *testing.T) {
	bc := &blockingBlockCounter{ch: make(chan struct{})}
	defer close(bc.ch) // release the parked goroutine

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

// TestWaitForBlockHeightCtx_ReturnsImmediatelyOnSuccess asserts the shim does
// not introduce extra latency when the underlying counter returns promptly.
func TestWaitForBlockHeightCtx_ReturnsImmediatelyOnSuccess(t *testing.T) {
	bc := &blockingBlockCounter{ch: make(chan struct{})}
	close(bc.ch) // pre-close: WaitForBlockHeight returns nil immediately

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := waitForBlockHeightCtx(ctx, bc, 1); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

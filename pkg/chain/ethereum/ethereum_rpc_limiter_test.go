package ethereum

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEthereumRPCLimiterRateWaitHonorsCancellation(t *testing.T) {
	limiter := newEthereumRPCLimiter(1, 0)
	if err := limiter.AcquirePermit(context.Background()); err != nil {
		t.Fatal(err)
	}
	limiter.ReleasePermit()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- limiter.AcquirePermit(ctx)
	}()
	time.AfterFunc(25*time.Millisecond, cancel)

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected canceled rate wait error: [%v]", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request-rate wait ignored context cancellation")
	}
}

func TestEthereumRPCLimiterConcurrencyWaitHonorsCancellation(t *testing.T) {
	limiter := newEthereumRPCLimiter(0, 1)
	if err := limiter.AcquirePermit(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.AcquirePermit(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected canceled concurrency wait error: [%v]", err)
	}

	limiter.ReleasePermit()
	resumeCtx, resumeCancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer resumeCancel()
	if err := limiter.AcquirePermit(resumeCtx); err != nil {
		t.Fatalf("canceled waiter retained limiter capacity: [%v]", err)
	}
	limiter.ReleasePermit()
}

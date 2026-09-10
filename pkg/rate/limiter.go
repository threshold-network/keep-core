package rate

import (
	"context"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

var (
	// DefaultRequestsPerSecondLimit specifies the default maximum average number
	// of requests per second.
	DefaultRequestsPerSecondLimit = 150

	// DefaultConcurrencyLimit specifies the default number of concurrent requests
	// which can be executed against the target at the same time.
	DefaultConcurrencyLimit = 30
)

// Limiter is a helper tool which allows controlling the number and
// concurrency of requests made against a generic target.
type Limiter struct {
	limiter              *rate.Limiter
	semaphore            *semaphore.Weighted
	acquirePermitTimeout time.Duration
}

// LimiterConfig represents the configuration of the rate limiter.
type LimiterConfig struct {
	// RequestsPerSecondLimit sets the maximum average number of requests
	// per second. It's important to note that in short periods of time
	// the actual average may exceed this limit slightly.
	RequestsPerSecondLimit int

	// ConcurrencyLimit sets the maximum number of concurrent requests which
	// can be executed against the target at the same time.
	ConcurrencyLimit int

	// AcquirePermitTimeout determines how long a request can wait trying
	// to acquire a permit from the rate limiter.
	AcquirePermitTimeout time.Duration
}

// NewLimiter creates a new rate limiter instance basing on given config.
func NewLimiter(
	config *LimiterConfig,
) *Limiter {
	l := &Limiter{}

	if config.RequestsPerSecondLimit > 0 {
		l.limiter = rate.NewLimiter(
			rate.Limit(config.RequestsPerSecondLimit),
			1,
		)
	}

	if config.ConcurrencyLimit > 0 {
		l.semaphore = semaphore.NewWeighted(
			int64(config.ConcurrencyLimit),
		)
	}

	if config.AcquirePermitTimeout > 0 {
		l.acquirePermitTimeout = config.AcquirePermitTimeout
	} else {
		l.acquirePermitTimeout = 5 * time.Minute
	}

	return l
}

// AcquirePermit acquires the permit.
func (l *Limiter) AcquirePermit() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		l.acquirePermitTimeout,
	)
	defer cancel()

	if l.limiter != nil {
		err := l.limiter.Wait(ctx)
		if err != nil {
			return err
		}
	}

	if l.semaphore != nil {
		err := l.semaphore.Acquire(ctx, 1)
		if err != nil {
			return err
		}
	}

	return nil
}

// ReleasePermit releases the permit.
func (l *Limiter) ReleasePermit() {
	if l.semaphore != nil {
		l.semaphore.Release(1)
	}
}

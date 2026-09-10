package wrappers

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// DoWithRetry executes the provided doFn as long as it returns an error or until
// a timeout is hit. It applies exponential backoff wait of backoffTime * 2^n
// before nth retry of doFn. In case the calculated backoff is longer than
// backoffMax, the backoffMax wait is applied.
func DoWithRetry(
	parentCtx context.Context,
	backoffTime time.Duration,
	backoffMax time.Duration,
	timeout time.Duration,
	doFn func(ctx context.Context) error,
) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	var err error
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"retry timeout [%v] exceeded; most recent error: [%w]",
				timeout,
				err,
			)
		default:
			err = doFn(ctx)
			if err == nil {
				return nil
			}

			timedOut := backoffWait(ctx, backoffTime)
			if timedOut {
				return fmt.Errorf(
					"retry timeout [%v] exceeded; most recent error: [%w]",
					timeout,
					err,
				)
			}

			backoffTime = calculateBackoff(
				backoffTime,
				backoffMax,
			)
		}
	}
}

const (
	// DefaultDoBackoffTime is the default value of backoff time used by
	// DoWithDefaultRetry function.
	DefaultDoBackoffTime = 1 * time.Second

	// DefaultDoMaxBackoffTime is the default value of max backoff time used by
	// DoWithDefaultRetry function.
	DefaultDoMaxBackoffTime = 120 * time.Second
)

// DoWithDefaultRetry executes the provided doFn as long as it returns an error or
// until a timeout is hit. It applies exponential backoff wait of
// DefaultBackoffTime * 2^n before nth retry of doFn. In case the calculated
// backoff is longer than DefaultMaxBackoffTime, the DefaultMaxBackoffTime is
// applied.
func DoWithDefaultRetry(
	parentCtx context.Context,
	timeout time.Duration,
	doFn func(ctx context.Context) error,
) error {
	return DoWithRetry(
		parentCtx,
		DefaultDoBackoffTime,
		DefaultDoMaxBackoffTime,
		timeout,
		doFn,
	)
}

// ConfirmWithTimeout executes the provided confirmFn until it returns true or
// until it fails or until a timeout is hit. It applies exponential backoff wait
// of backoffTime * 2^n before nth execution of confirmFn. In case the
// calculated backoff is longer than backoffMax, the backoffMax is applied.
// In case confirmFn returns an error, ConfirmWithTimeout exits with the same
// error immediately. This is different from DoWithRetry behavior as the use
// case for this function is different. ConfirmWithTimeout is intended to be
// used to confirm a chain state and not to try to enforce a successful
// execution of some function.
func ConfirmWithTimeout(
	parentCtx context.Context,
	backoffTime time.Duration,
	backoffMax time.Duration,
	timeout time.Duration,
	confirmFn func(ctx context.Context) (bool, error),
) (bool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return false, nil
		default:
			ok, err := confirmFn(ctx)
			if err == nil && ok {
				return true, nil
			}
			if err != nil {
				return false, err
			}

			timedOut := backoffWait(ctx, backoffTime)
			if timedOut {
				return false, nil
			}

			backoffTime = calculateBackoff(
				backoffTime,
				backoffMax,
			)
		}
	}
}

const (
	// DefaultConfirmBackoffTime is the default value of backoff time used by
	// ConfirmWithDefaultTimeout function.
	DefaultConfirmBackoffTime = 5 * time.Second

	// DefaultConfirmMaxBackoffTime is the default value of max backoff time
	// used by ConfirmWithDefaultTimeout function.
	DefaultConfirmMaxBackoffTime = 10 * time.Second
)

// ConfirmWithTimeoutDefaultBackoff executed the provided confirmFn until it
// returns true or until it fails or until timeout is hit. It applies
// backoff wait of DefaultConfirmBackoffTime * 2^n before nth execution of
// confirmFn. In case the calculated backoff is longer than
// DefaultConfirmMaxBackoffTime, DefaultConfirmMaxBackoffTime is applied.
// In case confirmFn returns an error, ConfirmWithTimeoutDefaultBackoff exits
// with the same error immediately. This is different from DoWithDefaultRetry
// behavior as the use case for this function is different.
// ConfirmWithTimeoutDefaultBackoff is intended to be used to confirm a chain
// state and not to try to enforce a successful execution of some function.
func ConfirmWithTimeoutDefaultBackoff(
	parentCtx context.Context,
	timeout time.Duration,
	confirmFn func(ctx context.Context) (bool, error),
) (bool, error) {
	return ConfirmWithTimeout(
		parentCtx,
		DefaultConfirmBackoffTime,
		DefaultConfirmMaxBackoffTime,
		timeout,
		confirmFn,
	)
}

func calculateBackoff(
	backoffPrev time.Duration,
	backoffMax time.Duration,
) time.Duration {
	backoff := backoffPrev

	backoff *= 2

	// #nosec G404
	// we are fine with not using cryptographically secure random integer,
	// it is just exponential backoff jitter
	r := rand.Int63n(backoff.Nanoseconds()/10 + 1)
	jitter := time.Duration(r) * time.Nanosecond
	backoff += jitter

	if backoff > backoffMax {
		backoff = backoffMax
	}

	return backoff
}

func backoffWait(ctx context.Context, waitTime time.Duration) bool {
	timer := time.NewTimer(waitTime)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}

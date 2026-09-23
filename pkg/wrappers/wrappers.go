package wrappers

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// DoWithRetry executes the provided doFn as long as it returns an error or until
// a timeout is hit. It applies exponential backoff wait of backoffTime * 2^n
// before nth retry of doFn. In case the calculated backoff is longer than
// backoffMax, the backoffMax wait is applied.
//
// If the configured timeout elapses before doFn succeeds, DoWithRetry returns an
// error that matches ErrRetryTimeout via errors.Is. This signals that the retry
// budget was exhausted after repeated attempts, as opposed to a single attempt
// failing and returning immediately. Callers may use errors.Is(err, ErrRetryTimeout)
// to detect retry exhaustion, and errors.Unwrap to inspect the most recent error
// returned by doFn.
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
			return &retryTimeoutError{timeout: timeout, cause: err}
		default:
			err = doFn(ctx)
			if err == nil {
				return nil
			}

			timedOut := backoffWait(ctx, backoffTime)
			if timedOut {
				return &retryTimeoutError{timeout: timeout, cause: err}
			}

			backoffTime = calculateBackoff(
				backoffTime,
				backoffMax,
			)
		}
	}
}

// ErrRetryTimeout is the sentinel error matched via errors.Is when a retry loop
// (DoWithRetry or DoWithDefaultRetry) exceeds its configured timeout without
// doFn ever succeeding. It indicates that the entire retry budget was exhausted,
// as opposed to an individual execution of doFn failing once and returning.
var ErrRetryTimeout = errors.New("retry timeout")

// retryTimeoutError renders the historical "retry timeout [%v] exceeded;
// most recent error: [%v]" message verbatim while matching
// errors.Is(err, ErrRetryTimeout) and unwrapping to the most recent
// underlying error from doFn.
type retryTimeoutError struct {
	timeout time.Duration
	cause   error
}

func (e *retryTimeoutError) Error() string {
	return fmt.Sprintf(
		"retry timeout [%v] exceeded; most recent error: [%v]",
		e.timeout,
		e.cause,
	)
}

func (e *retryTimeoutError) Unwrap() error {
	return e.cause
}

func (e *retryTimeoutError) Is(target error) bool {
	return target == ErrRetryTimeout
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
//
// Like DoWithRetry, if the timeout is reached before doFn succeeds, it returns an
// error matching ErrRetryTimeout via errors.Is.
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

	// we are fine with not using cryptographically secure random integer,
	// it is just exponential backoff jitter
	r := rand.Int64N(backoff.Nanoseconds()/10 + 1)
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

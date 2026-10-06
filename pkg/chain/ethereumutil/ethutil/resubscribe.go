package ethutil

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/event"
)

// WithResubscription wraps the subscribe function to call it repeatedly
// to keep a subscription alive. When a subscription is established, it is
// monitored and in the case of a failure, resubscribe is attempted by
// calling the subscribe function again.
//
// The mechanism applies backoff between resubscription attempts.
// The time between calls is adapted based on the error rate, but will never
// exceed backoffMax.
//
// The mechanism monitors the time elapsed between resubscription attempts and
// if it is shorter than the specificed alertThreshold, it calls
// thresholdViolatedFn passing the time elapsed between resubscription attempts.
// This function alarms about potential problems with the stability of the
// subscription.
//
// In case of an error returned by the wrapped subscription function,
// subscriptionFailedFn is called with the underlying error.
//
// thresholdViolatedFn and subscriptionFailedFn calls are executed in a separate
// goroutine and thus are non-blocking.
func WithResubscription(
	backoffMax time.Duration,
	subscribeFn event.ResubscribeFunc,
	alertThreshold time.Duration,
	thresholdViolatedFn func(time.Duration),
	subscriptionFailedFn func(error),
) event.Subscription {
	lastAttempt := time.Time{}
	wrappedResubscribeFn := func(ctx context.Context) (event.Subscription, error) {
		now := time.Now()
		elapsed := now.Sub(lastAttempt)
		if elapsed < alertThreshold {
			go thresholdViolatedFn(elapsed)
		}

		lastAttempt = now

		sub, err := subscribeFn(ctx)
		if err != nil {
			go subscriptionFailedFn(err)
		}
		return sub, err
	}

	return event.Resubscribe(backoffMax, wrappedResubscribeFn)
}

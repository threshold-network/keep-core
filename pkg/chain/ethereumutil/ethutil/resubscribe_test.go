package ethutil

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/event"
)

func TestEmitOriginalError(t *testing.T) {
	backoffMax := 100 * time.Millisecond
	alertThreshold := 100 * time.Millisecond

	failedOnce := false
	expectedFailMessage := "wherever I go, he goes"
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		if !failedOnce {
			failedOnce = true
			return nil, errors.New(expectedFailMessage)
		}
		delegate := event.NewSubscription(func(unsubscribed <-chan struct{}) error {
			return nil
		})
		return delegate, nil
	}

	// Using buffered channels to do not block writes.
	// There should never be a need to write more to those channels if the code
	// under the test works as expected.
	thresholdViolated := make(chan time.Duration, 10)
	subscriptionFailed := make(chan error, 10)
	subscription := WithResubscription(
		backoffMax,
		subscribeFn,
		alertThreshold,
		func(elapsed time.Duration) { thresholdViolated <- elapsed },
		func(err error) { subscriptionFailed <- err },
	)
	<-subscription.Err()

	// Subscription failed one time so there should be one error in the channel.
	subscriptionFailCount := len(subscriptionFailed)
	if subscriptionFailCount != 1 {
		t.Fatalf(
			"subscription failure reported [%v] times, expected [1]",
			subscriptionFailCount,
		)
	}

	// That failure should refer the original error.
	err := <-subscriptionFailed
	if err.Error() != expectedFailMessage {
		t.Errorf(
			"unexpected subscription error message\nexpected: [%v]\nactual:   [%v]",
			expectedFailMessage,
			err.Error(),
		)
	}
}

func TestResubscribeAboveThreshold(t *testing.T) {
	backoffMax := 100 * time.Millisecond
	alertThreshold := 100 * time.Millisecond

	plannedSubscriptionFailures := 3
	elapsedBetweenFailures := 150 * time.Millisecond

	resubscribeFnCalls := 0
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		resubscribeFnCalls++
		time.Sleep(elapsedBetweenFailures) // 150ms > 100ms, above alert threshold
		if resubscribeFnCalls <= plannedSubscriptionFailures {
			return nil, fmt.Errorf("this is the way")
		}
		delegate := event.NewSubscription(func(unsubscribed <-chan struct{}) error {
			return nil
		})
		return delegate, nil
	}

	// Using buffered channels to do not block writes.
	// There should never be a need to write more to those channels if the code
	// under the test works as expected.
	thresholdViolated := make(chan time.Duration, 10)
	subscriptionFailed := make(chan error, 10)
	subscription := WithResubscription(
		backoffMax,
		subscribeFn,
		alertThreshold,
		func(elapsed time.Duration) { thresholdViolated <- elapsed },
		func(err error) { subscriptionFailed <- err },
	)
	<-subscription.Err()

	// Nothing expected in thresholdViolated channel.
	// Alert threshold is set to 100ms and there were no resubscription attempts
	// in a time shorter than 150ms one after another.
	violationCount := len(thresholdViolated)
	if violationCount != 0 {
		t.Errorf(
			"threshold violation reported [%v] times, expected none",
			violationCount,
		)
	}

	// Subscription failed plannedSubscriptionFailures times and resubscription
	// function should be called plannedSubscriptionFailures + 1 times. One time
	// for each failure and one time at the end - that subscription was
	// successful and had not to be retried.
	expectedResubscriptionCalls := plannedSubscriptionFailures + 1
	if resubscribeFnCalls != expectedResubscriptionCalls {
		t.Errorf(
			"resubscription called [%v] times, expected [%v]",
			resubscribeFnCalls,
			expectedResubscriptionCalls,
		)
	}

	// Expect all subscription failures to be reported.
	subscriptionFailCount := len(subscriptionFailed)
	if subscriptionFailCount != plannedSubscriptionFailures {
		t.Errorf(
			"subscription failure reported [%v] times, expected [%v]",
			subscriptionFailCount,
			plannedSubscriptionFailures,
		)
	}
}

func TestResubscribeBelowThreshold(t *testing.T) {
	backoffMax := 50 * time.Millisecond
	alertThreshold := 100 * time.Millisecond

	plannedSubscriptionFailures := 5
	elapsedBetweenFailures := 50 * time.Millisecond

	resubscribeFnCalls := 0
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		resubscribeFnCalls++
		time.Sleep(elapsedBetweenFailures) // 50ms < 100ms, below alert threshold
		if resubscribeFnCalls <= plannedSubscriptionFailures {
			return nil, fmt.Errorf("i have spoken")
		}
		delegate := event.NewSubscription(func(unsubscribed <-chan struct{}) error {
			return nil
		})
		return delegate, nil
	}

	// Using buffered channels to do not block writes.
	// There should never be a need to write more to those channels if the code
	// under the test works as expected.
	thresholdViolated := make(chan time.Duration, 10)
	subscriptionFailed := make(chan error, 10)
	subscription := WithResubscription(
		backoffMax,
		subscribeFn,
		alertThreshold,
		func(elapsed time.Duration) { thresholdViolated <- elapsed },
		func(err error) { subscriptionFailed <- err },
	)
	<-subscription.Err()

	// Threshold violaton should be reported for each subscription failure if
	// the time elapsed since the previous resubscription was shorter than the
	// threshold.
	// In this test, alert threshold is set to 100ms and delays between failures
	// are just 50ms. Thus, we expect the same number of threshold violations as
	// resubscription attempts.
	violationCount := len(thresholdViolated)
	if violationCount != plannedSubscriptionFailures {
		t.Errorf(
			"threshold violation reported [%v] times, expected [%v]",
			violationCount,
			plannedSubscriptionFailures,
		)
	}

	// All violations reported should have correct values - all of them should
	// be longer than the time elapsed between failures and shorter than the
	// alert threshold. It is not possible to assert on a precise value.
	for i := 0; i < violationCount; i++ {
		violation := <-thresholdViolated
		if violation < elapsedBetweenFailures {
			t.Errorf(
				"violation reported should be longer than the time elapsed "+
					"between failures; is: [%v] and should be longer than [%v]",
				violation,
				elapsedBetweenFailures,
			)
		}
		if violation > alertThreshold {
			t.Errorf(
				"violation reported should be shorter than the alert threshold; "+
					"; is: [%v] and should be shorter than [%v]",
				violation,
				alertThreshold,
			)
		}
	}

	// Subscription failed plannedSubscriptionFailures times and resubscription
	// function should be called plannedSubscriptionFailures + 1 times. One time
	// for each failure and one time at the end - that subscription was
	// successful and had not to be retried.
	expectedResubscriptionCalls := plannedSubscriptionFailures + 1
	if resubscribeFnCalls != expectedResubscriptionCalls {
		t.Errorf(
			"resubscription called [%v] times, expected [%v]",
			resubscribeFnCalls,
			expectedResubscriptionCalls,
		)
	}

	// Expect all subscription failures to be reported.
	subscriptionFailCount := len(subscriptionFailed)
	if subscriptionFailCount != plannedSubscriptionFailures {
		t.Errorf(
			"subscription failure reported [%v] times, expected [%v]",
			subscriptionFailCount,
			plannedSubscriptionFailures,
		)
	}
}

func TestDoNotBlockOnChannelWrites(t *testing.T) {
	backoffMax := 50 * time.Millisecond
	alertThreshold := 100 * time.Millisecond

	plannedSubscriptionFailures := 5
	elapsedBetweenFailures := 10 * time.Millisecond

	resubscribeFnCalls := 0
	subscribeFn := func(ctx context.Context) (event.Subscription, error) {
		resubscribeFnCalls++
		time.Sleep(elapsedBetweenFailures) // 10ms < 100ms, below alert threshold
		if resubscribeFnCalls <= plannedSubscriptionFailures {
			return nil, fmt.Errorf("Groku?")
		}
		delegate := event.NewSubscription(func(unsubscribed <-chan struct{}) error {
			return nil
		})
		return delegate, nil
	}

	// Non-buffered channels with no receivers, will block on write
	thresholdViolated := make(chan time.Duration)
	subscriptionFailed := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscription := WithResubscription(
		backoffMax,
		subscribeFn,
		alertThreshold,
		func(elapsed time.Duration) {
			select {
			case thresholdViolated <- elapsed:
			case <-ctx.Done():
				return
			}
		},
		func(err error) {
			select {
			case subscriptionFailed <- err:
			case <-ctx.Done():
				return
			}
		},
	)
	<-subscription.Err()

	// Subscription failed plannedSubscriptionFailures times and resubscription
	// function should be called plannedSubscriptionFailures + 1 times. One time
	// for each failure and one time at the end - that subscription was
	// successful and had not to be retried. No resubscription attempt should be
	// blocked by the lack of channel receivers on non-buffered channels.
	expectedResubscriptionCalls := plannedSubscriptionFailures + 1
	if resubscribeFnCalls != expectedResubscriptionCalls {
		t.Errorf(
			"resubscription called [%v] times, expected [%v]",
			resubscribeFnCalls,
			expectedResubscriptionCalls,
		)
	}
}

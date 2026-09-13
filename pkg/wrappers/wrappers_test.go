package wrappers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDoWithRetry(t *testing.T) {
	backoffTime := 10 * time.Millisecond
	backoffMax := 100 * time.Millisecond
	timeout := 2 * time.Second

	actualFailCount := 0
	expectedFailCount := 4
	doFn := func(ctx context.Context) error {
		if actualFailCount < expectedFailCount {
			actualFailCount++
			return fmt.Errorf("try again please")
		}

		return nil
	}

	err := DoWithRetry(context.Background(), backoffTime, backoffMax, timeout, doFn)
	if err != nil {
		t.Fatal(err)
	}

	if actualFailCount != expectedFailCount {
		t.Errorf(
			"unexpected fail count: actual [%v], expected [%v]",
			actualFailCount,
			expectedFailCount,
		)
	}
}

func TestDoWithRetryExceedTimeout(t *testing.T) {
	backoffTime := 100 * time.Millisecond
	backoffMax := 500 * time.Millisecond
	timeout := 1 * time.Second

	actualFailCount := 0

	// This function should be executed 4 times and timeout:
	// 100 ms
	// 200 ms
	// 400 ms
	// 500 ms <- here it should exceed 1s timeout
	doFn := func(ctx context.Context) error {
		if actualFailCount < 10 {
			actualFailCount++
			return fmt.Errorf("try again please %d", actualFailCount)
		}

		return nil
	}

	err := DoWithRetry(context.Background(), backoffTime, backoffMax, timeout, doFn)
	if err == nil {
		t.Fatal("expected a timeout error")
	}

	expectedLatestError := "try again please 4"
	expectedError := "retry timeout [1s] exceeded; most recent error: [try again please 4]"
	if err.Error() != expectedError {
		t.Errorf(
			"unexpected error message\nactual:   [%v]\nexpected: [%v]",
			err.Error(),
			expectedError,
		)
	}
	if errors.Unwrap(err).Error() != expectedLatestError {
		t.Errorf(
			"unexpected error message\nactual:   [%v]\nexpected: [%v]",
			err.Error(),
			expectedError,
		)
	}

	expectedFailCount := 4
	if actualFailCount != expectedFailCount {
		t.Errorf(
			"unexpected fail count: actual [%v], expected [%v]",
			actualFailCount,
			expectedFailCount,
		)
	}
}

func TestConfirmWithTimeout(t *testing.T) {
	backoffTime := 10 * time.Millisecond
	backoffMax := 100 * time.Millisecond
	timeout := 2 * time.Second

	actualCheckCount := 0
	expectedCheckCount := 3
	confirmFn := func(ctx context.Context) (bool, error) {
		if actualCheckCount < expectedCheckCount {
			actualCheckCount++
			return false, nil
		}

		return true, nil
	}

	ok, err := ConfirmWithTimeout(context.Background(), backoffTime, backoffMax, timeout, confirmFn)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected the check to eventually succeed")
	}

	if actualCheckCount != expectedCheckCount {
		t.Errorf(
			"unexpected check count: actual [%v], expected [%v]",
			actualCheckCount,
			expectedCheckCount,
		)
	}
}

func TestConfirmWithTimeoutExceedTimeout(t *testing.T) {
	backoffTime := 100 * time.Millisecond
	backoffMax := 300 * time.Millisecond
	timeout := 1 * time.Second

	actualCheckCount := 0

	// This function should be executed 5 times and timeout:
	// 100 ms
	// 200 ms
	// 300 ms
	// 300 ms
	// 300 ms <- here it should exceed 1s timeout
	confirmFn := func(ctx context.Context) (bool, error) {
		if actualCheckCount < 10 {
			actualCheckCount++
			return false, nil
		}

		return true, nil
	}

	ok, err := ConfirmWithTimeout(context.Background(), backoffTime, backoffMax, timeout, confirmFn)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("expected the check to eventually fail")
	}

	expectedCheckCount := 5
	if actualCheckCount != expectedCheckCount {
		t.Errorf(
			"unexpected check count: actual [%v], expected [%v]",
			actualCheckCount,
			expectedCheckCount,
		)
	}
}

func TestConfirmWithTimeoutFailure(t *testing.T) {
	backoffTime := 100 * time.Millisecond
	backoffMax := 300 * time.Millisecond
	timeout := 1 * time.Second

	confirmFn := func(ctx context.Context) (bool, error) {
		return false, fmt.Errorf("untada")
	}

	ok, err := ConfirmWithTimeout(context.Background(), backoffTime, backoffMax, timeout, confirmFn)
	if err == nil {
		t.Fatal("expected an error")
	}
	if ok {
		t.Errorf("should return false")
	}

	expectedError := "untada"
	if err.Error() != expectedError {
		t.Errorf(
			"unexpected error message\nactual:   [%v]\nexpected: [%v]",
			err.Error(),
			expectedError,
		)
	}
}

func TestCalculateBackoff(t *testing.T) {
	backoffInitial := 120 * time.Second
	backoffMax := 300 * time.Second

	expectedMin := 240 * time.Second // 2 * backoffInitial
	expectedMax := 265 * time.Second // 2 * backoffInitial * 110% + 1

	for i := 0; i < 100; i++ {
		backoff := calculateBackoff(backoffInitial, backoffMax)

		if backoff < expectedMin {
			t.Errorf(
				"backoff [%v] shorter than the expected minimum [%v]",
				backoff,
				expectedMin,
			)
		}

		if backoff > expectedMax {
			t.Errorf(
				"backoff [%v] longer than the expected maximum [%v]",
				backoff,
				expectedMax,
			)
		}
	}
}

func TestCalculateBackoffMax(t *testing.T) {
	backoffInitial := 220 * time.Second
	backoffMax := 300 * time.Second

	backoff := calculateBackoff(backoffInitial, backoffMax)
	if backoff != backoffMax {
		t.Errorf(
			"expected max backoff of [%v]; has [%v]",
			backoffMax,
			backoff,
		)
	}
}

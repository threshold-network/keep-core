package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"testing"

	"github.com/ipfs/go-log"
)

func TestBalanceMonitor_Retries(t *testing.T) {
	log.SetDebugLogging()

	attemptsCount := 0
	expectedAttempts := 3

	wg := &sync.WaitGroup{}
	wg.Add(expectedAttempts)

	balanceSource := func(address Address) (*Token, error) {
		attemptsCount++
		wg.Done()

		if attemptsCount < expectedAttempts {
			return nil, fmt.Errorf("not this time")
		}

		return &Token{big.NewInt(10)}, nil
	}

	balanceMonitor := NewBalanceMonitor(balanceSource)

	address := Address{1, 2}
	alertThreshold := &Token{big.NewInt(15)}
	tick := 1 * time.Minute
	retryTimeout := 5 * time.Second

	balanceMonitor.Observe(
		context.Background(),
		address,
		alertThreshold,
		tick,
		retryTimeout,
	)

	wg.Wait()

	if expectedAttempts != attemptsCount {
		t.Errorf(
			"unexpected retries count\nexpected: %d\nactual:   %d",
			expectedAttempts,
			attemptsCount,
		)
	}
}

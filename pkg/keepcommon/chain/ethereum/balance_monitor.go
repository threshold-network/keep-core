package ethereum

import (
	"context"
	"fmt"
	"time"

	"github.com/keep-network/keep-common/pkg/wrappers"
)

// BalanceSource provides a balance info for the given address.
type BalanceSource func(address Address) (*Token, error)

// BalanceMonitor provides the possibility to monitor balances for given
// accounts.
type BalanceMonitor struct {
	balanceSource BalanceSource
}

// NewBalanceMonitor creates a new instance of the balance monitor.
func NewBalanceMonitor(balanceSource BalanceSource) *BalanceMonitor {
	return &BalanceMonitor{balanceSource}
}

// Observe starts a process which checks the address balance with the given
// tick and triggers an alert in case the balance falls below the
// alert threshold value.
// The balance check will be retried in case of an error up to the retry timeout.
func (bm *BalanceMonitor) Observe(
	ctx context.Context,
	address Address,
	alertThreshold *Token,
	tick time.Duration,
	retryTimeout time.Duration,
) {
	check := func(ctx context.Context) error {
		balance, err := bm.balanceSource(address)
		if err != nil {
			wrappedErr := fmt.Errorf(
				"failed to get balance for account [%s]: [%w]",
				address.TerminalString(),
				err,
			)

			logger.Warning(wrappedErr)

			return wrappedErr
		}

		if balance.Cmp(alertThreshold.Int) == -1 {
			logger.Errorf(
				"balance for account [%v] is below [%v]; "+
					"account should be funded",
				address.TerminalString(),
				alertThreshold.Text(10),
			)
		}

		return nil
	}

	go func() {
		ticker := time.NewTicker(tick)
		defer ticker.Stop()

		checkBalance := func() {
			err := wrappers.DoWithDefaultRetry(ctx, retryTimeout, check)
			if err != nil {
				logger.Errorf("balance monitor error: [%v]", err)
			}
		}

		// Initial balance check at monitoring start.
		checkBalance()

		for {
			select {
			// Balance check at ticks.
			case <-ticker.C:
				checkBalance()
			case <-ctx.Done():
				return
			}
		}
	}()
}

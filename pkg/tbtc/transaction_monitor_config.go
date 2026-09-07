package tbtc

import (
	"fmt"
	"time"
)

const (
	// DefaultTransactionMonitorStuckThreshold is the age at which an
	// unconfirmed wallet transaction starts generating alerts.
	DefaultTransactionMonitorStuckThreshold = 6 * time.Hour
	// DefaultTransactionMonitorCheckInterval is the confirmation polling cadence.
	DefaultTransactionMonitorCheckInterval = 5 * time.Minute
	// DefaultTransactionMonitorMaxTracked bounds the in-memory tracking table.
	DefaultTransactionMonitorMaxTracked = 1000
	// DefaultTransactionMonitorMaxTrackingAge bounds how long an unconfirmed
	// transaction occupies the tracking table.
	DefaultTransactionMonitorMaxTrackingAge = 24 * time.Hour
	// DefaultTransactionMonitorCheckBudget bounds one confirmation-check pass.
	DefaultTransactionMonitorCheckBudget = 2 * time.Minute
)

// TransactionMonitorConfig configures monitoring of broadcast wallet
// transactions. Zero values select the defaults; negative values are invalid.
type TransactionMonitorConfig struct {
	StuckThreshold time.Duration
	CheckInterval  time.Duration
	MaxTracked     int
	MaxTrackingAge time.Duration
	CheckBudget    time.Duration
}

func (c TransactionMonitorConfig) withDefaults() TransactionMonitorConfig {
	if c.StuckThreshold == 0 {
		c.StuckThreshold = DefaultTransactionMonitorStuckThreshold
	}
	if c.CheckInterval == 0 {
		c.CheckInterval = DefaultTransactionMonitorCheckInterval
	}
	if c.MaxTracked == 0 {
		c.MaxTracked = DefaultTransactionMonitorMaxTracked
	}
	if c.MaxTrackingAge == 0 {
		c.MaxTrackingAge = DefaultTransactionMonitorMaxTrackingAge
	}
	if c.CheckBudget == 0 {
		c.CheckBudget = DefaultTransactionMonitorCheckBudget
	}
	return c
}

// Validate checks the effective configuration, including defaults for omitted
// fields. Transactions must remain tracked long enough to generate an alert.
func (c TransactionMonitorConfig) Validate() error {
	c = c.withDefaults()
	for _, setting := range []struct {
		name  string
		value time.Duration
	}{
		{"stuckThreshold", c.StuckThreshold},
		{"checkInterval", c.CheckInterval},
		{"maxTrackingAge", c.MaxTrackingAge},
		{"checkBudget", c.CheckBudget},
	} {
		if setting.value < 0 {
			return fmt.Errorf("tbtc.transactionMonitor.%s must not be negative", setting.name)
		}
	}
	if c.MaxTracked < 0 {
		return fmt.Errorf("tbtc.transactionMonitor.maxTracked must not be negative")
	}
	if c.MaxTrackingAge < c.StuckThreshold {
		return fmt.Errorf("tbtc.transactionMonitor.maxTrackingAge must be at least stuckThreshold")
	}
	return nil
}

package tbtc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
)

func TestTransactionMonitorConfig_Defaults(t *testing.T) {
	monitor, err := newTransactionMonitor(newLocalBitcoinChain(), TransactionMonitorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	expected := TransactionMonitorConfig{
		StuckThreshold: 6 * time.Hour,
		CheckInterval:  5 * time.Minute,
		MaxTracked:     1000,
		MaxTrackingAge: 24 * time.Hour,
		CheckBudget:    2 * time.Minute,
	}
	if monitor.config != expected {
		t.Fatalf("default configuration changed: got %+v, want %+v", monitor.config, expected)
	}
}

func TestTransactionMonitorConfig_Validation(t *testing.T) {
	tests := map[string]struct {
		config     TransactionMonitorConfig
		errorField string
	}{
		"negative threshold":          {TransactionMonitorConfig{StuckThreshold: -time.Second}, "stuckThreshold"},
		"negative interval":           {TransactionMonitorConfig{CheckInterval: -time.Second}, "checkInterval"},
		"negative capacity":           {TransactionMonitorConfig{MaxTracked: -1}, "maxTracked"},
		"negative age":                {TransactionMonitorConfig{MaxTrackingAge: -time.Second}, "maxTrackingAge"},
		"negative budget":             {TransactionMonitorConfig{CheckBudget: -time.Second}, "checkBudget"},
		"age below default threshold": {TransactionMonitorConfig{MaxTrackingAge: time.Hour}, "maxTrackingAge"},
		"threshold above default age": {TransactionMonitorConfig{StuckThreshold: 25 * time.Hour}, "maxTrackingAge"},
		"equal threshold and age":     {TransactionMonitorConfig{StuckThreshold: time.Hour, MaxTrackingAge: time.Hour}, ""},
		"partial override":            {TransactionMonitorConfig{CheckInterval: time.Minute}, ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newTransactionMonitor(newLocalBitcoinChain(), test.config)
			if test.errorField == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.errorField) {
				t.Fatalf("expected error naming %s, got %v", test.errorField, err)
			}
		})
	}
}

func TestTransactionMonitor_CustomThresholdCapacityAndAge(t *testing.T) {
	monitor, err := newTransactionMonitor(newLocalBitcoinChain(), TransactionMonitorConfig{
		StuckThreshold: time.Hour,
		MaxTracked:     2,
		MaxTrackingAge: 2 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := newCountingMetricsRecorder()
	monitor.setMetricsRecorder(recorder)
	first, second, overflow := bitcoin.Hash{1}, bitcoin.Hash{2}, bitcoin.Hash{3}
	monitor.track(first, [20]byte{1})
	monitor.track(second, [20]byte{1})
	monitor.track(overflow, [20]byte{1})
	if isTracked(monitor, overflow) || recorder.GetCounterValue(clientinfo.MetricUnmonitoredWalletTransactionsTotal) != 1 {
		t.Fatal("custom tracking capacity was not enforced")
	}
	ageTransaction(monitor, first, 45*time.Minute)
	ageTransaction(monitor, second, 90*time.Minute)
	monitor.check(context.Background())
	if got := recorder.GetCounterValue(clientinfo.MetricStuckWalletTransactionsTotal); got != 1 {
		t.Fatalf("expected one alert with the custom threshold, got %v", got)
	}
	if !isTracked(monitor, second) {
		t.Fatal("transaction evicted before custom maximum age")
	}
	ageTransaction(monitor, second, time.Hour)
	monitor.check(context.Background())
	if isTracked(monitor, second) {
		t.Fatal("transaction retained past custom maximum age")
	}
	if got := recorder.GetCounterValue(clientinfo.MetricStuckWalletTransactionsTotal); got != 1 {
		t.Fatalf("expected no duplicate alert before eviction, got %v", got)
	}
	monitor.track(overflow, [20]byte{1})
	if !isTracked(monitor, overflow) {
		t.Fatal("evicted transaction did not free tracking capacity")
	}
}

func TestTransactionMonitor_CustomCheckBudget(t *testing.T) {
	hash := bitcoin.Hash{1}
	chain := newBlockingTransactionConfirmationsChain(hash)
	monitor, err := newTransactionMonitor(chain, TransactionMonitorConfig{CheckBudget: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	monitor.track(hash, [20]byte{1})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	monitor.check(ctx)
	if ctx.Err() != nil {
		t.Fatal("custom check budget did not cancel the blocked lookup before the parent deadline")
	}
	if chain.getLookupCount() != 1 {
		t.Fatal("expected one confirmation lookup")
	}
}

func TestTransactionMonitor_CustomCheckInterval(t *testing.T) {
	hash := bitcoin.Hash{1}
	chain := newBlockingTransactionConfirmationsChain(hash)
	monitor, err := newTransactionMonitor(chain, TransactionMonitorConfig{CheckInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	monitor.track(hash, [20]byte{1})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); monitor.run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("monitor did not stop after cancellation")
		}
	})
	select {
	case <-chain.lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("custom polling interval did not trigger a confirmation check")
	}
}

func TestNewNode_RejectsInvalidTransactionMonitorConfig(t *testing.T) {
	// Validation must precede persistence, chain access, and scheduler setup.
	_, err := newNode(nil, nil, nil, nil, nil, nil, nil, nil, Config{
		TransactionMonitor: TransactionMonitorConfig{CheckInterval: -time.Second},
	})
	if err == nil || !strings.Contains(err.Error(), "checkInterval") {
		t.Fatalf("expected monitor configuration rejection before node setup, got %v", err)
	}
}

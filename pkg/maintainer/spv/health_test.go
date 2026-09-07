package spv

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

func TestProofTaskFailuresBeforeSubmission(t *testing.T) {
	for _, discoveryFails := range []bool{true, false} {
		for _, enabled := range []bool{true, false} {
			recorder := &recordingMetricsRecorder{counters: make(map[string]float64)}
			sm := &spvMaintainer{btcChain: newLocalBitcoinChain(), spvChain: newLocalChain(), btcDiffChain: newLocalChain()}
			if enabled {
				sm.metricsRecorder = recorder
			}
			getter := func(uint64, int, bitcoin.Chain, Chain) ([]*bitcoin.Transaction, error) {
				if discoveryFails {
					return nil, errors.New("cannot fetch redemption requests")
				}
				// Proof-info lookup fails because this transaction is unknown to
				// the Bitcoin backend. The submitter must never be called.
				return []*bitcoin.Transaction{{Version: 1}}, nil
			}
			submitter := func(bitcoin.Hash, uint, bitcoin.Chain, Chain, MetricsRecorder) error {
				t.Fatal("submission attempted after an earlier failure")
				return nil
			}
			if err := sm.runProofTask(tbtc.ActionRedemption, getter, submitter); err == nil {
				t.Fatal("expected a task error")
			}
			if enabled {
				if recorder.gauges[clientinfo.MetricSpvMaintainerLastFailureTimestamp] == 0 {
					t.Fatal("first task failure timestamp was not recorded")
				}
				for _, name := range []string{clientinfo.MetricSpvProofTaskFailuresTotal, clientinfo.MetricRedemptionProofTaskFailuresTotal} {
					if recorder.counters[name] != 1 {
						t.Errorf("expected one %s failure, got %v", name, recorder.counters[name])
					}
				}
				if recorder.gauges[clientinfo.MetricSpvMaintainerLastActivityTimestamp] == 0 {
					t.Fatal("task completion did not update activity")
				}
			}
		}
	}
}

func TestMaintainerHealthLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := &recordingMetricsRecorder{counters: make(map[string]float64)}
	sm := &spvMaintainer{
		config:          Config{RestartBackoffTime: time.Hour, IdleBackoffTime: 10 * time.Minute},
		metricsRecorder: recorder,
	}
	sm.startControlLoop(ctx)
	if recorder.gauges[clientinfo.MetricSpvMaintainerActive] != 0 {
		t.Fatal("stopped maintainer remains active")
	}
	if recorder.gauges[clientinfo.MetricSpvMaintainerMaxBackoffSeconds] != 3600 {
		t.Fatal("configured restart backoff was not exported")
	}
	if recorder.gauges[clientinfo.MetricSpvMaintainerLastSuccessTimestamp] != 0 {
		t.Fatal("canceled cycle was reported as successful")
	}
}

func TestMaintainerHealthLifecycle_MaxBackoffIdleLarger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := &recordingMetricsRecorder{counters: make(map[string]float64)}
	sm := &spvMaintainer{
		config:          Config{RestartBackoffTime: 10 * time.Minute, IdleBackoffTime: time.Hour},
		metricsRecorder: recorder,
	}
	sm.startControlLoop(ctx)
	if recorder.gauges[clientinfo.MetricSpvMaintainerMaxBackoffSeconds] != 3600 {
		t.Fatal("expected max backoff to use the larger configured idle backoff")
	}
}

// TestMaintainSpv_CancelsBetweenProofTasks verifies the per-action ctx.Err()
// check inside maintainSpv's loop actually stops the cycle: once the context
// is canceled while processing one proof type, no other proof type's getter
// is invoked afterward. proofTypes iteration order is randomized by Go's map
// semantics, so the fake getter shared by every action treats "first call"
// as whichever action the runtime visits first, and fails the test if a
// second call ever happens after cancellation.
func TestMaintainSpv_CancelsBetweenProofTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var callCount int32
	sharedGetter := func(uint64, int, bitcoin.Chain, Chain) ([]*bitcoin.Transaction, error) {
		if atomic.AddInt32(&callCount, 1) > 1 {
			t.Fatal("a proof task's getter ran after the SPV loop's context was canceled")
		}
		cancel()
		return nil, nil
	}
	sharedSubmitter := func(bitcoin.Hash, uint, bitcoin.Chain, Chain, MetricsRecorder) error {
		t.Fatal("submitter must not run when the getter reports no transactions")
		return nil
	}

	original := proofTypes
	defer func() { proofTypes = original }()
	proofTypes = map[tbtc.WalletActionType]struct {
		unprovenTransactionsGetter unprovenTransactionsGetter
		transactionProofSubmitter  transactionProofSubmitter
	}{
		tbtc.ActionDepositSweep: {unprovenTransactionsGetter: sharedGetter, transactionProofSubmitter: sharedSubmitter},
		tbtc.ActionRedemption:   {unprovenTransactionsGetter: sharedGetter, transactionProofSubmitter: sharedSubmitter},
	}

	sm := &spvMaintainer{btcChain: newLocalBitcoinChain(), spvChain: newLocalChain(), btcDiffChain: newLocalChain()}
	err := sm.maintainSpv(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from the canceled loop, got %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected exactly one proof task invocation before cancellation, got %d", got)
	}
}

// Proof submission test recorders do not observe lifecycle gauges.
func (*fakeMetricsRecorder) SetGauge(string, float64) {}

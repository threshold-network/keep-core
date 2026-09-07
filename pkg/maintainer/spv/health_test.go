package spv

import (
	"context"
	"errors"
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

// Proof submission test recorders do not observe lifecycle gauges.
func (*fakeMetricsRecorder) SetGauge(string, float64) {}

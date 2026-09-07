package clientinfo

import (
	"context"
	"errors"
	"testing"
	"time"

	keepclientinfo "github.com/keep-network/keep-common/pkg/clientinfo"
)

func TestRPCHealthMetricTransitions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eth := &fakeEthereumRPC{currentBlock: 100}
	btc := &fakeBitcoinChain{latestHeight: 100}
	checker := newTestChecker(eth, btc)
	checker.registry = &Registry{keepclientinfo.NewRegistry(), ctx}
	checker.registerMetrics()
	sources := checker.healthSources()
	for name := range sources {
		assertCounterExportedInRegistry(t, checker.registry, name)
	}
	for _, network := range []string{"eth", "btc"} {
		if sources["rpc_"+network+"_healthy"]() != 0 || sources["rpc_"+network+"_last_check_timestamp_seconds"]() != 0 {
			t.Fatal("unprobed RPC was reported healthy or recently checked")
		}
	}
	before := float64(time.Now().Unix())
	checker.checkEthereumHealth(ctx)
	checker.checkBitcoinHealth(ctx)
	for _, network := range []string{"eth", "btc"} {
		if sources["rpc_"+network+"_healthy"]() != 1 || sources["rpc_"+network+"_last_check_timestamp_seconds"]() < before {
			t.Fatalf("%s healthy check was not reflected in metrics", network)
		}
	}
	eth.err = errors.New("Ethereum unavailable")
	btc.latestErr = errors.New("Bitcoin unavailable")
	_, _, lastSuccess, _, _ := checker.GetEthereumHealthStatus()
	failedAt := time.Now()
	checker.checkEthereumHealth(ctx)
	checker.checkBitcoinHealth(ctx)
	if sources["rpc_eth_healthy"]() != 0 || sources["rpc_btc_healthy"]() != 0 {
		t.Fatal("RPC failures were not reflected in health metrics")
	}
	_, lastCheck, retainedSuccess, lastError, _ := checker.GetEthereumHealthStatus()
	if lastCheck.Before(failedAt) || !retainedSuccess.Equal(lastSuccess) || lastError == nil {
		t.Fatal("failed RPC must complete the check without advancing last success")
	}
}

type ethereumRPCFunc func(context.Context) (uint64, error)

func (probe ethereumRPCFunc) LatestBlockNumber(ctx context.Context) (uint64, error) {
	return probe(ctx)
}

func TestEthereumProbeDoesNotRefreshMetricsWhilePending(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	checker := NewRPCHealthChecker(nil, ethereumRPCFunc(func(probeCtx context.Context) (uint64, error) {
		close(started)
		<-probeCtx.Done()
		return 0, probeCtx.Err()
	}), nil, time.Minute)
	previousCheck := time.Now().Add(-time.Hour)
	checker.ethLastCheck = previousCheck
	checker.ethLastSuccess = previousCheck
	sources := checker.healthSources()
	done := make(chan struct{})
	go func() {
		defer close(done)
		checker.checkEthereumHealth(ctx)
	}()
	<-started
	if got := sources["rpc_eth_last_check_timestamp_seconds"](); got != float64(previousCheck.Unix()) {
		t.Errorf("pending RPC refreshed the completed-probe timestamp: %v", got)
	}
	cancel()
	<-done
	if sources["rpc_eth_healthy"]() != 0 {
		t.Fatal("cancelled RPC was reported healthy")
	}
	_, lastCheck, lastSuccess, lastError, _ := checker.GetEthereumHealthStatus()
	if !lastCheck.After(previousCheck) || !lastSuccess.Equal(previousCheck) || !errors.Is(lastError, context.Canceled) {
		t.Fatalf("unexpected cancellation state: %v, %v, %v", lastCheck, lastSuccess, lastError)
	}
}

func TestEthereumProbeDeadline(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "bounded RPC context", true: "expired caller context"}[expired], func(t *testing.T) {
			ctx := context.Background()
			if expired {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
			}
			checker := NewRPCHealthChecker(nil, ethereumRPCFunc(func(probeCtx context.Context) (uint64, error) {
				deadline, ok := probeCtx.Deadline()
				if !ok || time.Until(deadline) > 30*time.Second {
					t.Fatal("Ethereum RPC did not receive the probe deadline")
				}
				// Even a client that returns a cached/late success after cancellation
				// must not turn the health metric green.
				return 100, nil
			}), nil, time.Minute)
			checker.checkEthereumHealth(ctx)
			healthy, _, _, lastError, _ := checker.GetEthereumHealthStatus()
			if healthy == expired {
				t.Fatalf("unexpected health for expired=%v: %v", expired, healthy)
			}
			if expired && !errors.Is(lastError, context.DeadlineExceeded) {
				t.Fatalf("expected deadline failure, got %v", lastError)
			}
		})
	}
}

func TestMaintainerMetricsRegistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	metrics := NewPerformanceMetrics(ctx, registry)
	defer metrics.Stop()
	for _, name := range []string{
		MetricSpvMaintainerActive, MetricSpvMaintainerLastActivityTimestamp,
		MetricSpvMaintainerLastSuccessTimestamp, MetricSpvMaintainerMaxBackoffSeconds,
		MetricSpvMaintainerLastFailureTimestamp,
		MetricSpvProofTaskFailuresTotal, MetricRedemptionProofTaskFailuresTotal,
	} {
		assertCounterExportedInRegistry(t, registry, name)
	}
}

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
	eth := &fakeBlockCounter{currentBlock: 100}
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
	checker.checkEthereumHealth(ctx)
	checker.checkBitcoinHealth(ctx)
	if sources["rpc_eth_healthy"]() != 0 || sources["rpc_btc_healthy"]() != 0 {
		t.Fatal("RPC failures were not reflected in health metrics")
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

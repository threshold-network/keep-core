package clientinfo

import (
	"context"
	"testing"

	keepclientinfo "github.com/keep-network/keep-core/pkg/keepcommon/clientinfo"
)

// TestRedemptionProposalCountersRegistered tests that the redemption
// proposal counters are registered upfront so they appear in the metrics
// endpoint before any increment.
func TestRedemptionProposalCountersRegistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)
	defer pm.Stop()

	expectedCounters := []string{
		MetricRedemptionProposalGenerationTotal,
		MetricRedemptionProposalGenerationFailedTotal,
		MetricRedemptionProposalGenerationSuccessTotal,
		MetricRedemptionProposalBroadcastTotal,
		MetricRedemptionProposalBroadcastFailedTotal,
	}

	for _, counterName := range expectedCounters {
		pm.countersMutex.RLock()
		_, exists := pm.counters[counterName]
		pm.countersMutex.RUnlock()
		if !exists {
			t.Errorf("counter %s should be registered upfront", counterName)
			continue
		}

		assertCounterExportedInRegistry(t, registry, counterName)

		if value := pm.GetCounterValue(counterName); value != 0 {
			t.Errorf("counter %s should start at 0, got %v", counterName, value)
		}

		pm.IncrementCounter(counterName, 1)
		if value := pm.GetCounterValue(counterName); value != 1 {
			t.Errorf("counter %s should increment to 1, got %v", counterName, value)
		}
	}
}

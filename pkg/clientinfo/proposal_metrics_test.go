package clientinfo

import (
	"context"
	"testing"

	keepclientinfo "github.com/keep-network/keep-common/pkg/clientinfo"
)

func TestRedemptionProposalCountersRegistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	metrics := NewPerformanceMetrics(ctx, &Registry{keepclientinfo.NewRegistry(), ctx})
	defer metrics.Stop()
	for _, name := range []string{
		MetricRedemptionProposalGenerationAttemptsTotal,
		MetricRedemptionProposalGenerationFailuresTotal,
		MetricRedemptionProposalsGeneratedTotal,
		MetricRedemptionProposalsBroadcastTotal,
		MetricRedemptionProposalBroadcastFailuresTotal,
	} {
		if _, ok := metrics.counters[name]; !ok {
			t.Errorf("counter %s is absent before the first proposal", name)
		}
	}
}

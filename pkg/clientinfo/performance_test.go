package clientinfo

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	keepclientinfo "github.com/keep-network/keep-common/pkg/clientinfo"
)

// TestConcurrentCounterIncrement tests that concurrent counter increments
// are safe and produce correct results.
func TestConcurrentCounterIncrement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	const (
		numGoroutines = 100
		incrementsPer = 1000
		metricName    = MetricSigningOperationsTotal
	)

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPer; j++ {
				pm.IncrementCounter(metricName, 1)
			}
		}()
	}
	wg.Wait()

	expected := float64(numGoroutines * incrementsPer)
	actual := pm.GetCounterValue(metricName)
	if actual != expected {
		t.Errorf("Expected counter value %v, got %v", expected, actual)
	}
}

// TestConcurrentCounterDifferentMetrics tests concurrent increments on
// different counters.
func TestConcurrentCounterDifferentMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	const (
		numGoroutines = 50
		incrementsPer = 100
	)

	metrics := []string{
		MetricSigningOperationsTotal,
		MetricSigningSuccessTotal,
		MetricSigningFailedTotal,
	}

	// Add timeout to prevent test from hanging indefinitely
	testCtx, testCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer testCancel()

	done := make(chan struct{})
	var wg sync.WaitGroup
	for _, metricName := range metrics {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				for j := 0; j < incrementsPer; j++ {
					select {
					case <-testCtx.Done():
						return
					default:
						pm.IncrementCounter(name, 1)
					}
				}
			}(metricName)
		}
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-testCtx.Done():
		t.Fatal("Test timed out waiting for goroutines to complete")
	}

	expected := float64(numGoroutines * incrementsPer)
	for _, metricName := range metrics {
		actual := pm.GetCounterValue(metricName)
		if actual != expected {
			t.Errorf("Metric %s: expected %v, got %v", metricName, expected, actual)
		}
	}
}

// TestConcurrentDurationRecording tests that concurrent duration recordings
// are safe and produce correct results.
func TestConcurrentDurationRecording(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	const (
		numGoroutines = 50
		recordingsPer = 100
		metricName    = "signing_duration_seconds"
	)

	durations := []time.Duration{
		1 * time.Millisecond,
		10 * time.Millisecond,
		100 * time.Millisecond,
		1 * time.Second,
	}

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < recordingsPer; j++ {
				duration := durations[goroutineID%len(durations)]
				pm.RecordDuration(metricName, duration)
			}
		}(i)
	}
	wg.Wait()

	// Verify histogram was updated (we can't easily verify exact values
	// without exposing internal state, but we can verify the count matches)
	pm.histogramsMutex.RLock()
	h, exists := pm.histograms[metricName]
	pm.histogramsMutex.RUnlock()

	if !exists {
		t.Fatal("Histogram not found")
	}

	h.mutex.RLock()
	count := h.buckets[histogramCountKey]
	h.mutex.RUnlock()

	expectedCount := float64(numGoroutines * recordingsPer)
	if count != expectedCount {
		t.Errorf("Expected histogram count %v, got %v", expectedCount, count)
	}
}

// TestConcurrentGaugeSet tests that concurrent gauge updates are safe.
func TestConcurrentGaugeSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	const (
		numGoroutines = 100
		updatesPer    = 100
		metricName    = MetricIncomingMessageQueueSize
	)

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < updatesPer; j++ {
				value := float64(goroutineID*updatesPer + j)
				pm.SetGauge(metricName, value)
			}
		}(i)
	}
	wg.Wait()

	// We can't verify the exact value since goroutines race,
	// but we can verify the gauge exists and has been set
	value := pm.GetGaugeValue(metricName)
	if value < 0 {
		t.Errorf("Expected non-negative gauge value, got %v", value)
	}
}

// TestConcurrentDifferentOperations tests that different metric operations
// can run concurrently without issues.
func TestConcurrentDifferentOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	const (
		numGoroutines = 30
		operationsPer = 50
	)

	var wg sync.WaitGroup

	// Counter increments
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPer; j++ {
				pm.IncrementCounter(MetricSigningOperationsTotal, 1)
			}
		}()
	}

	// Duration recordings
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPer; j++ {
				pm.RecordDuration("signing_duration_seconds", time.Duration(j)*time.Millisecond)
			}
		}()
	}

	// Gauge sets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < operationsPer; j++ {
				pm.SetGauge(MetricIncomingMessageQueueSize, float64(goroutineID+j))
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations completed without race
	expectedCounter := float64(numGoroutines * operationsPer)
	actualCounter := pm.GetCounterValue(MetricSigningOperationsTotal)
	if actualCounter != expectedCounter {
		t.Errorf("Expected counter value %v, got %v", expectedCounter, actualCounter)
	}
}

// TestHistogramBucketPlacement tests that duration values are placed
// in the correct histogram buckets.
func TestHistogramBucketPlacement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	metricName := "test_duration_seconds"

	testCases := []struct {
		duration  time.Duration
		bucket    float64
		shouldRun bool
	}{
		{500 * time.Microsecond, 0.001, true}, // < 1ms
		{5 * time.Millisecond, 0.01, true},    // < 10ms
		{50 * time.Millisecond, 0.1, true},    // < 100ms
		{500 * time.Millisecond, 1, true},     // < 1s
		{5 * time.Second, 10, true},           // < 10s
		{30 * time.Second, 60, true},          // < 60s
		{200 * time.Second, 300, true},        // < 300s
		{500 * time.Second, 600, true},        // < 600s
		{1000 * time.Second, 0, false},        // > 600s (overflow)
	}

	for _, tc := range testCases {
		pm.RecordDuration(metricName, tc.duration)
	}

	// Verify histogram
	pm.histogramsMutex.RLock()
	h, exists := pm.histograms[metricName]
	pm.histogramsMutex.RUnlock()

	if !exists {
		t.Fatal("Histogram not found")
	}

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Verify count
	expectedCount := float64(len(testCases))
	actualCount := h.buckets[histogramCountKey]
	if actualCount != expectedCount {
		t.Errorf("Expected count %v, got %v", expectedCount, actualCount)
	}

	// Verify overflow bucket
	overflowCount := h.buckets[math.Inf(1)]
	if overflowCount != 1 {
		t.Errorf("Expected overflow bucket count 1, got %v", overflowCount)
	}
}

// TestMetricsInitialization tests that all metrics are initialized with zero values.
func TestMetricsInitialization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	// Test counters
	counters := []string{
		MetricDKGJoinedTotal,
		MetricSigningOperationsTotal,
		MetricSigningSuccessTotal,
	}

	for _, counterName := range counters {
		value := pm.GetCounterValue(counterName)
		if value != 0 {
			t.Errorf("Counter %s should start at 0, got %v", counterName, value)
		}
	}

	// Test gauges
	gauges := []string{
		MetricCPUUtilization,
		MetricMemoryUsageMB,
		MetricGoroutineCount,
		MetricCPULoadPercent,
		MetricRAMUtilizationPercent,
		MetricSwapUtilizationPercent,
	}

	for _, gaugeName := range gauges {
		value := pm.GetGaugeValue(gaugeName)
		if value != 0 {
			t.Errorf("Gauge %s should start at 0, got %v", gaugeName, value)
		}
	}
}

// TestContextCancelation tests that goroutines stop when context is cancelled.
func TestContextCancelation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	// Cancel context immediately
	cancel()

	// Give goroutines time to stop
	time.Sleep(100 * time.Millisecond)

	// This should not panic or cause issues
	pm.IncrementCounter(MetricSigningOperationsTotal, 1)
	pm.SetGauge(MetricIncomingMessageQueueSize, 5)
	pm.RecordDuration("signing_duration_seconds", 100*time.Millisecond)
}

// TestNetworkJoinFailureMetricName tests per-reason join failure counter
// name generation.
func TestNetworkJoinFailureMetricName(t *testing.T) {
	tests := map[string]string{
		JoinFailureReasonTimeout:              "network_join_requests_failed_timeout_total",
		JoinFailureReasonEOFReset:             "network_join_requests_failed_eof_reset_total",
		JoinFailureReasonProtocolCrypto:       "network_join_requests_failed_protocol_crypto_total",
		JoinFailureReasonFirewallUnrecognized: "network_join_requests_failed_firewall_unrecognized_total",
		JoinFailureReasonFirewallRPCError:     "network_join_requests_failed_firewall_rpc_error_total",
	}

	for reason, expectedName := range tests {
		if actualName := NetworkJoinFailureMetricName(reason); actualName != expectedName {
			t.Errorf(
				"unexpected metric name for reason %s\nexpected: %s\nactual: %s",
				reason,
				expectedName,
				actualName,
			)
		}
	}
}

// assertCounterExportedInRegistry verifies that counterName is actually
// exposed through the metrics registry (not just tracked in pm's internal
// counters map) by attempting to register the same gauge name again: a
// registration that was silently skipped (e.g. because
// ObserveApplicationSource was never called, or was called with the wrong
// metric name) would succeed here instead of failing with "already exists".
func assertCounterExportedInRegistry(
	t *testing.T,
	registry *Registry,
	counterName string,
) {
	t.Helper()

	metricName := fmt.Sprintf("performance_%s", counterName)
	if _, err := registry.NewMetricGauge(metricName); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Errorf(
			"counter %s should be exported in the metrics registry as %s",
			counterName, metricName,
		)
	}
}

// TestJoinFailureAndOnChainCountersRegistered tests that the per-reason join
// failure counters and the firewall on-chain checks counter are registered
// upfront so they appear in the metrics endpoint before any increment.
func TestJoinFailureAndOnChainCountersRegistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	expectedCounters := []string{MetricFirewallOnChainChecksTotal}
	for _, reason := range GetAllNetworkJoinFailureReasons() {
		expectedCounters = append(expectedCounters, NetworkJoinFailureMetricName(reason))
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

// TestDepositSweepProofSubmissionCountersRegistered tests that the
// deposit-sweep proof-submission counters are registered upfront so they
// appear in the metrics endpoint before any increment.
func TestDepositSweepProofSubmissionCountersRegistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := &Registry{keepclientinfo.NewRegistry(), ctx}
	pm := NewPerformanceMetrics(ctx, registry)

	expectedCounters := []string{
		MetricDepositSweepProofSubmissionsTotal,
		MetricDepositSweepProofSubmissionsSuccessTotal,
		MetricDepositSweepProofSubmissionsFailedTotal,
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

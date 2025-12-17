package clientinfo

import (
	"sync"
	"time"
)

// PerformanceMetricsRecorder provides a simple interface for recording
// performance metrics. It can be nil if metrics are not enabled.
type PerformanceMetricsRecorder interface {
	// IncrementCounter increments a counter metric
	IncrementCounter(name string, value float64)
	// RecordDuration records a duration in seconds
	RecordDuration(name string, duration time.Duration)
	// SetGauge sets a gauge metric value
	SetGauge(name string, value float64)
	// GetCounterValue returns current counter value
	GetCounterValue(name string) float64
	// GetGaugeValue returns current gauge value
	GetGaugeValue(name string) float64
}

// PerformanceMetrics provides a way to record performance-related metrics
// including operation counts, durations, and queue sizes.
// It implements PerformanceMetricsRecorder interface.
type PerformanceMetrics struct {
	registry *Registry

	// Counters track cumulative counts of events
	countersMutex sync.RWMutex
	counters      map[string]*counter

	// Histograms track distributions of values (like durations)
	histogramsMutex sync.RWMutex
	histograms      map[string]*histogram

	// Gauges track current values (like queue sizes)
	gaugesMutex sync.RWMutex
	gauges      map[string]*gauge
}

// Ensure PerformanceMetrics implements PerformanceMetricsRecorder
var _ PerformanceMetricsRecorder = (*PerformanceMetrics)(nil)

type counter struct {
	value float64
	mutex sync.RWMutex
}

type histogram struct {
	buckets map[float64]float64 // bucket upper bound -> count
	mutex   sync.RWMutex
}

type gauge struct {
	value float64
	mutex sync.RWMutex
}

// NewPerformanceMetrics creates a new performance metrics instance.
func NewPerformanceMetrics(registry *Registry) *PerformanceMetrics {
	pm := &PerformanceMetrics{
		registry:   registry,
		counters:   make(map[string]*counter),
		histograms: make(map[string]*histogram),
		gauges:     make(map[string]*gauge),
	}

	// Register gauge observers for all gauges
	go pm.observeGauges()

	return pm
}

// IncrementCounter increments a counter metric by the given value.
func (pm *PerformanceMetrics) IncrementCounter(name string, value float64) {
	pm.countersMutex.Lock()
	c, exists := pm.counters[name]
	if !exists {
		c = &counter{value: 0}
		pm.counters[name] = c
	}
	pm.countersMutex.Unlock()

	c.mutex.Lock()
	c.value += value
	c.mutex.Unlock()

	// Update the gauge observer for this counter
	pm.registry.ObserveApplicationSource(
		"performance",
		map[string]Source{
			name: func() float64 {
				c.mutex.RLock()
				defer c.mutex.RUnlock()
				return c.value
			},
		},
	)
}

// RecordDuration records a duration value in a histogram.
// The duration is recorded in seconds.
func (pm *PerformanceMetrics) RecordDuration(name string, duration time.Duration) {
	pm.histogramsMutex.Lock()
	h, exists := pm.histograms[name]
	if !exists {
		h = &histogram{
			buckets: make(map[float64]float64),
		}
		pm.histograms[name] = h
	}
	pm.histogramsMutex.Unlock()

	seconds := duration.Seconds()
	h.mutex.Lock()
	// Simple histogram: increment bucket counts
	// Buckets: 0.001, 0.01, 0.1, 1, 10, 60, 300, 600
	buckets := []float64{0.001, 0.01, 0.1, 1, 10, 60, 300, 600}
	for _, bucket := range buckets {
		if seconds <= bucket {
			h.buckets[bucket]++
			break
		}
	}
	// Also track total count and sum for average calculation
	h.buckets[-1]++          // -1 = count
	h.buckets[-2] += seconds // -2 = sum
	h.mutex.Unlock()

	// Expose as gauge for now (Prometheus-style histograms would be better)
	pm.registry.ObserveApplicationSource(
		"performance",
		map[string]Source{
			name + "_duration_seconds": func() float64 {
				h.mutex.RLock()
				defer h.mutex.RUnlock()
				count := h.buckets[-1]
				if count == 0 {
					return 0
				}
				return h.buckets[-2] / count // average
			},
			name + "_count": func() float64 {
				h.mutex.RLock()
				defer h.mutex.RUnlock()
				return h.buckets[-1]
			},
		},
	)
}

// SetGauge sets a gauge metric to the given value.
func (pm *PerformanceMetrics) SetGauge(name string, value float64) {
	pm.gaugesMutex.Lock()
	g, exists := pm.gauges[name]
	if !exists {
		g = &gauge{value: 0}
		pm.gauges[name] = g
	}
	pm.gaugesMutex.Unlock()

	g.mutex.Lock()
	g.value = value
	g.mutex.Unlock()

	// Register gauge observer if not already registered
	pm.registry.ObserveApplicationSource(
		"performance",
		map[string]Source{
			name: func() float64 {
				g.mutex.RLock()
				defer g.mutex.RUnlock()
				return g.value
			},
		},
	)
}

// observeGauges periodically updates gauge observers.
// This is handled automatically by ObserveApplicationSource.
func (pm *PerformanceMetrics) observeGauges() {
	// Gauges are observed automatically via ObserveApplicationSource
	// This function is kept for future use if needed
}

// NoOpPerformanceMetrics is a no-op implementation of PerformanceMetricsRecorder
// that can be used when metrics are disabled.
type NoOpPerformanceMetrics struct{}

// IncrementCounter is a no-op.
func (n *NoOpPerformanceMetrics) IncrementCounter(name string, value float64) {}

// RecordDuration is a no-op.
func (n *NoOpPerformanceMetrics) RecordDuration(name string, duration time.Duration) {}

// SetGauge is a no-op.
func (n *NoOpPerformanceMetrics) SetGauge(name string, value float64) {}

// GetCounterValue always returns 0.
func (n *NoOpPerformanceMetrics) GetCounterValue(name string) float64 { return 0 }

// GetGaugeValue always returns 0.
func (n *NoOpPerformanceMetrics) GetGaugeValue(name string) float64 { return 0 }

// GetCounterValue returns the current value of a counter.
func (pm *PerformanceMetrics) GetCounterValue(name string) float64 {
	pm.countersMutex.RLock()
	c, exists := pm.counters[name]
	pm.countersMutex.RUnlock()

	if !exists {
		return 0
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.value
}

// GetGaugeValue returns the current value of a gauge.
func (pm *PerformanceMetrics) GetGaugeValue(name string) float64 {
	pm.gaugesMutex.RLock()
	g, exists := pm.gauges[name]
	pm.gaugesMutex.RUnlock()

	if !exists {
		return 0
	}

	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.value
}

// Metric names for performance metrics
const (
	// DKG Metrics
	MetricDKGJoinedTotal              = "dkg_joined_total"
	MetricDKGFailedTotal              = "dkg_failed_total"
	MetricDKGDurationSeconds          = "dkg_duration_seconds"
	MetricDKGValidationTotal          = "dkg_validation_total"
	MetricDKGChallengesSubmittedTotal = "dkg_challenges_submitted_total"
	MetricDKGApprovalsSubmittedTotal  = "dkg_approvals_submitted_total"

	// Signing Metrics
	MetricSigningOperationsTotal      = "signing_operations_total"
	MetricSigningSuccessTotal         = "signing_success_total"
	MetricSigningFailedTotal          = "signing_failed_total"
	MetricSigningDurationSeconds      = "signing_duration_seconds"
	MetricSigningAttemptsPerOperation = "signing_attempts_per_operation"
	MetricSigningTimeoutsTotal        = "signing_timeouts_total"

	// Wallet Action Metrics
	MetricWalletActionsTotal           = "wallet_actions_total"
	MetricWalletActionSuccessTotal     = "wallet_action_success_total"
	MetricWalletActionFailedTotal      = "wallet_action_failed_total"
	MetricWalletActionDurationSeconds  = "wallet_action_duration_seconds"
	MetricWalletHeartbeatFailuresTotal = "wallet_heartbeat_failures_total"

	// Coordination Metrics
	MetricCoordinationWindowsDetectedTotal    = "coordination_windows_detected_total"
	MetricCoordinationProceduresExecutedTotal = "coordination_procedures_executed_total"
	MetricCoordinationFailedTotal             = "coordination_failed_total"
	MetricCoordinationDurationSeconds         = "coordination_duration_seconds"

	// Network Metrics
	MetricIncomingMessageQueueSize = "incoming_message_queue_size"
	MetricMessageHandlerQueueSize  = "message_handler_queue_size"
	MetricPeerConnectionsTotal     = "peer_connections_total"
	MetricPeerDisconnectionsTotal  = "peer_disconnections_total"
	MetricMessageBroadcastTotal    = "message_broadcast_total"
	MetricMessageReceivedTotal     = "message_received_total"
	MetricPingTestsTotal           = "ping_test_total"
	MetricPingTestSuccessTotal     = "ping_test_success_total"
	MetricPingTestFailedTotal      = "ping_test_failed_total"

	// Wallet Dispatcher Metrics
	MetricWalletDispatcherActiveActions = "wallet_dispatcher_active_actions"
	MetricWalletDispatcherRejectedTotal = "wallet_dispatcher_rejected_total"

	// Relay Entry Metrics (Beacon)
	MetricRelayEntryGenerationTotal      = "relay_entry_generation_total"
	MetricRelayEntrySuccessTotal         = "relay_entry_success_total"
	MetricRelayEntryFailedTotal          = "relay_entry_failed_total"
	MetricRelayEntryDurationSeconds      = "relay_entry_duration_seconds"
	MetricRelayEntryTimeoutReportedTotal = "relay_entry_timeout_reported_total"
)

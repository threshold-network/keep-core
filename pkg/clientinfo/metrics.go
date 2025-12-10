package clientinfo

import (
	"fmt"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/net"
)

type Source func() float64

// Names under which metrics are exposed.
const (
	ConnectedPeersCountMetricName     = "connected_peers_count"
	ConnectedBootstrapCountMetricName = "connected_bootstrap_count"
	EthConnectivityMetricName         = "eth_connectivity"
	BtcConnectivityMetricName         = "btc_connectivity"
	ClientInfoMetricName              = "client_info"

	// Protocol execution metrics
	// DKGExecutionDurationMetricName measures the time taken to complete a DKG
	// protocol execution, including all retry attempts. Recorded with status
	// label: "success", "failure", or "canceled".
	DKGExecutionDurationMetricName = "dkg_execution_duration_seconds"
	// DKGAttemptsTotalMetricName counts the total number of DKG attempts made.
	// Includes a status label indicating the outcome: "success" or "failure".
	DKGAttemptsTotalMetricName = "dkg_attempts_total"
	// DKGAttemptRetryCountMetricName tracks the number of retries performed
	// during a DKG execution. Higher values indicate more retries were needed.
	DKGAttemptRetryCountMetricName = "dkg_attempt_retry_count"
	// DKGGroupSizeMetricName records the actual group size used in DKG execution,
	// which may differ from the configured group size if members are excluded.
	DKGGroupSizeMetricName = "dkg_group_size"
	// DKGExcludedMembersCountMetricName tracks the number of members excluded
	// from a DKG attempt due to misbehavior or other issues.
	DKGExcludedMembersCountMetricName = "dkg_excluded_members_count"
	// SigningExecutionDurationMetricName measures the time taken to complete a
	// signing protocol execution for a single message. Recorded with status
	// label: "success", "failure", or "timeout".
	SigningExecutionDurationMetricName = "signing_execution_duration_seconds"
	// SigningAttemptsTotalMetricName counts the total number of signing attempts
	// made. Includes a status label indicating the outcome: "success", "failure",
	// or "timeout".
	SigningAttemptsTotalMetricName = "signing_attempts_total"
	// SigningBatchSizeMetricName records the number of messages in a signing batch.
	// Batches allow multiple messages to be signed sequentially.
	SigningBatchSizeMetricName = "signing_batch_size"
	// SigningBatchDurationMetricName measures the total time taken to sign all
	// messages in a batch, from start to completion of the last signature.
	SigningBatchDurationMetricName = "signing_batch_duration_seconds"

	// Relay entry metrics (Beacon)
	// RelayEntryGenerationDurationMetricName measures the time from when a relay
	// entry is requested until it is successfully submitted to the chain.
	RelayEntryGenerationDurationMetricName = "relay_entry_generation_duration_seconds"
	// RelayEntryTimeoutsTotalMetricName counts the number of times a relay entry
	// generation timed out before completion.
	RelayEntryTimeoutsTotalMetricName = "relay_entry_timeouts_total"
	// RelayEntrySubmissionDelayBlocksMetricName tracks the number of blocks between
	// when a relay entry is requested and when it is submitted. This helps monitor
	// submission timing and eligibility delays.
	RelayEntrySubmissionDelayBlocksMetricName = "relay_entry_submission_delay_blocks"

	// Chain operation metrics
	// ChainTransactionSubmissionDurationMetricName measures the time taken to
	// submit a transaction to the Ethereum network, from initiation to acceptance
	// by the mempool.
	ChainTransactionSubmissionDurationMetricName = "chain_transaction_submission_duration_seconds"
	// ChainTransactionConfirmationBlocksMetricName tracks the number of blocks
	// between transaction submission and confirmation on-chain.
	ChainTransactionConfirmationBlocksMetricName = "chain_transaction_confirmation_blocks"
	// ChainTransactionGasUsedMetricName records the amount of gas consumed by
	// transactions. Useful for monitoring gas costs and optimizing operations.
	ChainTransactionGasUsedMetricName = "chain_transaction_gas_used"
	// ChainTransactionFailuresTotalMetricName counts the number of failed
	// transaction submissions. Includes transactions that revert or fail validation.
	ChainTransactionFailuresTotalMetricName = "chain_transaction_failures_total"
	// ChainCallDurationMetricName measures the time taken to execute read-only
	// chain calls (view functions). Helps monitor chain connectivity and performance.
	ChainCallDurationMetricName = "chain_call_duration_seconds"
	// ChainCallFailuresTotalMetricName counts the number of failed read-only chain
	// calls. High values may indicate connectivity issues or contract problems.
	ChainCallFailuresTotalMetricName = "chain_call_failures_total"

	// Network metrics
	// MessageSendDurationMetricName measures the time taken to send a message
	// over the LibP2P network, from initiation to acknowledgment.
	MessageSendDurationMetricName = "message_send_duration_seconds"
	// MessageReceiveDurationMetricName measures the time taken to receive and
	// process an incoming message from the network.
	MessageReceiveDurationMetricName = "message_receive_duration_seconds"
	// MessageRetransmissionCountMetricName tracks the number of times a message
	// had to be retransmitted. Higher values indicate network reliability issues.
	MessageRetransmissionCountMetricName = "message_retransmission_count"
	// MessageDroppedTotalMetricName counts the number of messages dropped due to
	// slow handlers or queue overflow. Indicates potential performance bottlenecks.
	MessageDroppedTotalMetricName = "message_dropped_total"

	// Group metrics
	// ActiveGroupsCountMetricName tracks the current number of active groups
	// this node is a member of. Active groups are those currently participating
	// in protocol operations.
	ActiveGroupsCountMetricName = "active_groups_count"
	// GroupRegistrationsTotalMetricName counts the total number of group
	// registrations this node has participated in since startup.
	GroupRegistrationsTotalMetricName = "group_registrations_total"
	// GroupUnregistrationsTotalMetricName counts the total number of group
	// unregistrations (when groups become stale or expire).
	GroupUnregistrationsTotalMetricName = "group_unregistrations_total"
	// GroupMembershipCountMetricName records the distribution of group sizes
	// this node participates in. Helps monitor group composition and health.
	GroupMembershipCountMetricName = "group_membership_count"

	// Error metrics
	// ProtocolErrorsTotalMetricName counts protocol-level errors encountered
	// during execution. Includes labels for protocol type ("dkg", "signing", etc.)
	// and error_type for categorization.
	ProtocolErrorsTotalMetricName = "protocol_errors_total"
	// ContextTimeoutTotalMetricName counts the number of context timeouts
	// encountered. Includes an "operation" label indicating which operation timed out.
	ContextTimeoutTotalMetricName = "context_timeout_total"
	// RetryAttemptsTotalMetricName counts the total number of retry attempts
	// made across all operations. Includes labels for operation type and final_status.
	RetryAttemptsTotalMetricName = "retry_attempts_total"
	// OperationTimeoutsTotalMetricName counts operation-level timeouts (distinct
	// from context timeouts). Includes an "operation_type" label for categorization.
	OperationTimeoutsTotalMetricName = "operation_timeouts_total"
)

// MetricsRecorder provides methods to record various types of metrics.
// It wraps the Registry and provides convenient methods for recording counters,
// histograms, and duration metrics. All methods are safe to call with a nil
// receiver, making it easy to use when metrics are not configured.
type MetricsRecorder struct {
	registry *Registry
	counters map[string]*counterValue
	mu       sync.RWMutex
}

// counterValue represents a counter metric value with thread-safe access.
type counterValue struct {
	value float64
	mu    sync.RWMutex
}

// NewMetricsRecorder creates a new metrics recorder for the given registry.
// Returns nil if the registry is nil, allowing for graceful handling when
// metrics are not configured.
func NewMetricsRecorder(registry *Registry) *MetricsRecorder {
	if registry == nil {
		return nil
	}
	mr := &MetricsRecorder{
		registry: registry,
		counters: make(map[string]*counterValue),
	}
	return mr
}

// RecordCounter increments a counter metric by one. The counter is created
// lazily on first use and observed periodically. Labels can be provided for
// metric categorization, though they are currently stored but not exposed
// in the current implementation.
func (mr *MetricsRecorder) RecordCounter(name string, labels map[string]string) {
	if mr == nil {
		return
	}
	mr.mu.Lock()
	counter, exists := mr.counters[name]
	if !exists {
		counter = &counterValue{value: 0}
		mr.counters[name] = counter
		// Set up gauge observer for this counter
		mr.registry.observe(
			name,
			func() float64 {
				counter.mu.RLock()
				defer counter.mu.RUnlock()
				return counter.value
			},
			ApplicationMetricsTick,
		)
	}
	mr.mu.Unlock()

	counter.mu.Lock()
	counter.value++
	counter.mu.Unlock()
}

// RecordHistogram records a value in a histogram metric. In the current
// implementation, this tracks the last observed value. In a full Prometheus
// implementation, this would use proper histogram buckets for distribution
// analysis. Labels can be provided for metric categorization.
func (mr *MetricsRecorder) RecordHistogram(name string, value float64, labels map[string]string) {
	if mr == nil {
		return
	}
	// For histograms, we use a gauge observer that tracks the last value
	// In a full Prometheus implementation, this would use proper histogram buckets
	mr.registry.observe(
		fmt.Sprintf("%s_last", name),
		func() float64 { return value },
		ApplicationMetricsTick,
	)
}

// RecordDuration records a duration metric in seconds. This is a convenience
// wrapper around RecordHistogram that converts a time.Duration to seconds.
// Labels can be provided for metric categorization.
func (mr *MetricsRecorder) RecordDuration(name string, duration time.Duration, labels map[string]string) {
	if mr == nil {
		return
	}
	mr.RecordHistogram(name, duration.Seconds(), labels)
}

const (
	// DefaultNetworkMetricsTick is the default duration of the
	// observation tick for network metrics.
	DefaultNetworkMetricsTick = 1 * time.Minute
	// DefaultEthereumMetricsTick is the default duration of the
	// observation tick for Ethereum metrics.
	DefaultEthereumMetricsTick = 10 * time.Minute
	// DefaultBitcoinMetricsTick is the default duration of the
	// observation tick for Bitcoin metrics.
	DefaultBitcoinMetricsTick = 10 * time.Minute
	// The duration of the observation tick for all application-specific
	// metrics.
	ApplicationMetricsTick = 1 * time.Minute
)

// ObserveConnectedPeersCount triggers an observation process of the
// connected_peers_count metric.
func (r *Registry) ObserveConnectedPeersCount(
	netProvider net.Provider,
	tick time.Duration,
) {
	input := func() float64 {
		connectedPeers := netProvider.ConnectionManager().ConnectedPeers()
		return float64(len(connectedPeers))
	}

	r.observe(
		ConnectedPeersCountMetricName,
		input,
		validateTick(tick, DefaultNetworkMetricsTick),
	)
}

// ObserveConnectedBootstrapCount triggers an observation process of the
// connected_bootstrap_count metric.
func (r *Registry) ObserveConnectedBootstrapCount(
	netProvider net.Provider,
	bootstraps []string,
	tick time.Duration,
) {
	input := func() float64 {
		currentCount := 0

		for _, address := range bootstraps {
			if netProvider.ConnectionManager().IsConnected(address) {
				currentCount++
			}
		}

		return float64(currentCount)
	}

	r.observe(
		ConnectedBootstrapCountMetricName,
		input,
		validateTick(tick, DefaultNetworkMetricsTick),
	)
}

// ObserveEthConnectivity triggers an observation process of the
// eth_connectivity metric.
func (r *Registry) ObserveEthConnectivity(
	blockCounter chain.BlockCounter,
	tick time.Duration,
) {
	input := func() float64 {
		_, err := blockCounter.CurrentBlock()
		if err != nil {
			return 0
		}

		return 1
	}

	r.observe(
		EthConnectivityMetricName,
		input,
		validateTick(tick, DefaultEthereumMetricsTick),
	)
}

// ObserveBtcConnectivity triggers an observation process of the
// btc_connectivity metric.
func (r *Registry) ObserveBtcConnectivity(
	btcChain bitcoin.Chain,
	tick time.Duration,
) {
	input := func() float64 {
		_, err := btcChain.GetLatestBlockHeight()
		if err != nil {
			return 0
		}

		return 1
	}

	r.observe(
		BtcConnectivityMetricName,
		input,
		validateTick(tick, DefaultBitcoinMetricsTick),
	)
}

// ObserveApplicationSource triggers an observation process of
// application-specific metrics.
func (r *Registry) ObserveApplicationSource(
	application string,
	inputs map[string]Source,
) {
	for k, v := range inputs {
		r.observe(
			fmt.Sprintf("%s_%s", application, k),
			v,
			ApplicationMetricsTick,
		)
	}
}

// RegisterMetricClientInfo registers static client information labels for metrics.
func (r *Registry) RegisterMetricClientInfo(version string) {
	_, err := r.NewMetricInfo(
		ClientInfoMetricName,
		[]clientinfo.Label{
			clientinfo.NewLabel("version", version),
		},
	)
	if err != nil {
		logger.Warnf("could not register metric client info: [%v]", err)
	}
}

func (r *Registry) observe(
	name string,
	input Source,
	tick time.Duration,
) {
	observer, err := r.NewMetricGaugeObserver(name, clientinfo.MetricObserverInput(input))
	if err != nil {
		logger.Warnf("could not create gauge observer [%v]", name)
		return
	}

	observer.Observe(r.ctx, tick)

	logger.Infof("observing %s with [%s] tick", name, tick)
}

func validateTick(tick time.Duration, defaultTick time.Duration) time.Duration {
	if tick > 0 {
		return tick
	}

	return defaultTick
}

// RecordDKGExecutionDuration records the duration of a DKG execution and
// increments the attempt counter. The status parameter should be one of:
// "success", "failure", or "canceled" to indicate the outcome of the execution.
func (mr *MetricsRecorder) RecordDKGExecutionDuration(duration time.Duration, status string) {
	if mr == nil {
		return
	}
	mr.RecordDuration(DKGExecutionDurationMetricName, duration, map[string]string{"status": status})
	mr.RecordCounter(DKGAttemptsTotalMetricName, map[string]string{"status": status})
}

// RecordDKGAttemptRetry records metrics for a DKG attempt retry, including
// the retry count, actual group size used, and number of excluded members.
// This helps track retry patterns and group composition during DKG execution.
func (mr *MetricsRecorder) RecordDKGAttemptRetry(retryCount int, groupSize int, excludedMembers int) {
	if mr == nil {
		return
	}
	mr.RecordHistogram(DKGAttemptRetryCountMetricName, float64(retryCount), nil)
	mr.RecordHistogram(DKGGroupSizeMetricName, float64(groupSize), nil)
	mr.RecordHistogram(DKGExcludedMembersCountMetricName, float64(excludedMembers), nil)
}

// RecordSigningExecutionDuration records the duration of a signing execution
// for a single message and increments the attempt counter. The status parameter
// should be one of: "success", "failure", or "timeout" to indicate the outcome.
func (mr *MetricsRecorder) RecordSigningExecutionDuration(duration time.Duration, status string) {
	if mr == nil {
		return
	}
	mr.RecordDuration(SigningExecutionDurationMetricName, duration, map[string]string{"status": status})
	mr.RecordCounter(SigningAttemptsTotalMetricName, map[string]string{"status": status})
}

// RecordSigningBatch records metrics for a batch signing operation, including
// the number of messages in the batch and the total duration to sign all messages.
// Batches allow multiple messages to be signed sequentially for efficiency.
func (mr *MetricsRecorder) RecordSigningBatch(batchSize int, duration time.Duration) {
	if mr == nil {
		return
	}
	mr.RecordHistogram(SigningBatchSizeMetricName, float64(batchSize), nil)
	mr.RecordDuration(SigningBatchDurationMetricName, duration, nil)
}

// RecordRelayEntryGeneration records metrics for relay entry generation
// in the beacon application, including the total duration and the block delay
// between request and submission. This helps monitor beacon performance.
func (mr *MetricsRecorder) RecordRelayEntryGeneration(duration time.Duration, delayBlocks uint64) {
	if mr == nil {
		return
	}
	mr.RecordDuration(RelayEntryGenerationDurationMetricName, duration, nil)
	mr.RecordHistogram(RelayEntrySubmissionDelayBlocksMetricName, float64(delayBlocks), nil)
}

// RecordRelayEntryTimeout increments the counter for relay entry timeouts.
// Timeouts occur when a relay entry is not generated and submitted within
// the expected time window.
func (mr *MetricsRecorder) RecordRelayEntryTimeout() {
	if mr == nil {
		return
	}
	mr.RecordCounter(RelayEntryTimeoutsTotalMetricName, nil)
}

// RecordChainTransaction records metrics for Ethereum chain transactions,
// including submission duration, confirmation block count, gas usage, and
// success/failure status. This provides comprehensive transaction performance data.
func (mr *MetricsRecorder) RecordChainTransaction(
	submissionDuration time.Duration,
	confirmationBlocks uint64,
	gasUsed uint64,
	success bool,
) {
	if mr == nil {
		return
	}
	mr.RecordDuration(ChainTransactionSubmissionDurationMetricName, submissionDuration, nil)
	mr.RecordHistogram(ChainTransactionConfirmationBlocksMetricName, float64(confirmationBlocks), nil)
	mr.RecordHistogram(ChainTransactionGasUsedMetricName, float64(gasUsed), nil)
	if !success {
		mr.RecordCounter(ChainTransactionFailuresTotalMetricName, nil)
	}
}

// RecordChainCall records metrics for read-only chain calls (view functions),
// including call duration and success/failure status. Useful for monitoring
// chain connectivity and call performance.
func (mr *MetricsRecorder) RecordChainCall(duration time.Duration, success bool) {
	if mr == nil {
		return
	}
	mr.RecordDuration(ChainCallDurationMetricName, duration, nil)
	if !success {
		mr.RecordCounter(ChainCallFailuresTotalMetricName, nil)
	}
}

// RecordMessageSend records the duration of sending a message over the LibP2P
// network. This helps monitor network performance and message propagation times.
func (mr *MetricsRecorder) RecordMessageSend(duration time.Duration) {
	if mr == nil {
		return
	}
	mr.RecordDuration(MessageSendDurationMetricName, duration, nil)
}

// RecordMessageReceive records the duration of receiving and processing an
// incoming message from the network. High durations may indicate processing
// bottlenecks.
func (mr *MetricsRecorder) RecordMessageReceive(duration time.Duration) {
	if mr == nil {
		return
	}
	mr.RecordDuration(MessageReceiveDurationMetricName, duration, nil)
}

// RecordMessageRetransmission records the number of retransmissions required
// for a message. Higher values indicate network reliability issues or message
// delivery problems.
func (mr *MetricsRecorder) RecordMessageRetransmission(retryCount int) {
	if mr == nil {
		return
	}
	mr.RecordHistogram(MessageRetransmissionCountMetricName, float64(retryCount), nil)
}

// RecordMessageDropped increments the counter for dropped messages. Messages
// are dropped when handlers are too slow or queues overflow, indicating
// potential performance bottlenecks.
func (mr *MetricsRecorder) RecordMessageDropped() {
	if mr == nil {
		return
	}
	mr.RecordCounter(MessageDroppedTotalMetricName, nil)
}

// RecordProtocolError records a protocol-level error with protocol type and
// error type labels. Protocol should be one of: "dkg", "signing", "relay", etc.
// Error type provides additional categorization of the error.
func (mr *MetricsRecorder) RecordProtocolError(protocol string, errorType string) {
	if mr == nil {
		return
	}
	mr.RecordCounter(ProtocolErrorsTotalMetricName, map[string]string{
		"protocol":   protocol,
		"error_type": errorType,
	})
}

// RecordContextTimeout records a context timeout with an operation label
// indicating which operation timed out (e.g., "dkg_execution", "signing_execution").
// Context timeouts occur when operations exceed their allocated time window.
func (mr *MetricsRecorder) RecordContextTimeout(operation string) {
	if mr == nil {
		return
	}
	mr.RecordCounter(ContextTimeoutTotalMetricName, map[string]string{"operation": operation})
}

// RecordRetryAttempt records a retry attempt with operation and final status
// labels. Operation indicates what was retried, and finalStatus indicates
// whether the retry succeeded ("success") or failed ("failure").
func (mr *MetricsRecorder) RecordRetryAttempt(operation string, finalStatus string) {
	if mr == nil {
		return
	}
	mr.RecordCounter(RetryAttemptsTotalMetricName, map[string]string{
		"operation":    operation,
		"final_status": finalStatus,
	})
}

// RecordOperationTimeout records an operation-level timeout with an operation
// type label (e.g., "dkg", "signing"). These are distinct from context timeouts
// and represent operation-specific timeout conditions.
func (mr *MetricsRecorder) RecordOperationTimeout(operationType string) {
	if mr == nil {
		return
	}
	mr.RecordCounter(OperationTimeoutsTotalMetricName, map[string]string{"operation_type": operationType})
}

// ObserveActiveGroupsCount sets up periodic observation for the count of active
// groups this node is a member of. The getActiveGroupsCount function is called
// periodically to update the metric. Active groups are those currently
// participating in protocol operations.
func (r *Registry) ObserveActiveGroupsCount(
	getActiveGroupsCount func() int,
	tick time.Duration,
) {
	input := func() float64 {
		return float64(getActiveGroupsCount())
	}

	r.observe(
		ActiveGroupsCountMetricName,
		input,
		validateTick(tick, ApplicationMetricsTick),
	)
}

// RecordGroupRegistration increments the counter for group registrations.
// This should be called whenever this node successfully registers as a member
// of a new group.
func (mr *MetricsRecorder) RecordGroupRegistration() {
	if mr == nil {
		return
	}
	mr.RecordCounter(GroupRegistrationsTotalMetricName, nil)
}

// RecordGroupUnregistration increments the counter for group unregistrations.
// This should be called when a group becomes stale, expires, or is otherwise
// unregistered.
func (mr *MetricsRecorder) RecordGroupUnregistration() {
	if mr == nil {
		return
	}
	mr.RecordCounter(GroupUnregistrationsTotalMetricName, nil)
}

// RecordGroupMembershipCount records the size of a group this node participates in.
// This helps track the distribution of group sizes and monitor group composition
// over time.
func (mr *MetricsRecorder) RecordGroupMembershipCount(count int) {
	if mr == nil {
		return
	}
	mr.RecordHistogram(GroupMembershipCountMetricName, float64(count), nil)
}

# Implemented Performance Metrics

This document provides a comprehensive overview of all performance metrics that have been implemented in the Keep Core node.

## Overview

Performance metrics provide visibility into node operations, network activity, and system health. All metrics are exposed via the `/metrics` HTTP endpoint when the client info endpoint is configured (default port: 9601).

**Metric Naming Convention**: All performance metrics are prefixed with `performance_` and follow Prometheus naming conventions.

## Metric Categories

### 1. Wallet Dispatcher Metrics

**Location**: `pkg/tbtc/wallet.go`

These metrics track wallet action dispatching and execution.

| Metric Name | Type | Description |
|------------|------|-------------|
| `performance_wallet_dispatcher_active_actions` | Gauge | Current number of wallets with active actions being executed |
| `performance_wallet_dispatcher_rejected_total` | Counter | Total number of wallet actions rejected because the wallet was busy |
| `performance_wallet_actions_total` | Counter | Total number of wallet actions dispatched |
| `performance_wallet_action_success_total` | Counter | Total number of successfully completed wallet actions |
| `performance_wallet_action_failed_total` | Counter | Total number of failed wallet actions |
| `performance_wallet_action_duration_seconds` | Histogram | Duration of wallet actions (exposed as average and count) |

**Implementation Details**:
- Active actions gauge is updated when actions start and complete
- Rejected actions counter increments when a wallet is busy and cannot accept new actions
- Duration is recorded for all actions (success and failure)
- Success/failure counters track action outcomes

**Use Cases**:
- Monitor wallet utilization and busy states
- Track action throughput and success rates
- Identify bottlenecks when wallets are frequently busy

---

### 2. DKG (Distributed Key Generation) Metrics

**Location**: `pkg/tbtc/dkg.go`

These metrics track DKG operations, including joins, validations, and on-chain submissions.

| Metric Name | Type | Description |
|------------|------|-------------|
| `performance_dkg_joined_total` | Counter | Total number of DKG joins (counts members joined) |
| `performance_dkg_failed_total` | Counter | Total number of failed DKG executions |
| `performance_dkg_duration_seconds` | Histogram | Duration of DKG operations (exposed as average and count) |
| `performance_dkg_validation_total` | Counter | Total number of DKG result validations performed |
| `performance_dkg_challenges_submitted_total` | Counter | Total number of DKG challenges submitted on-chain |
| `performance_dkg_approvals_submitted_total` | Counter | Total number of DKG approvals submitted on-chain |

**Implementation Details**:
- `dkg_joined_total` increments with the number of members that joined
- Duration is recorded when DKG completes (success or failure)
- Validation counter increments when DKG results are validated
- Challenge/approval counters track on-chain interactions

**Use Cases**:
- Monitor DKG participation rates
- Track DKG success rates and durations
- Monitor on-chain DKG interactions (challenges/approvals)

---

### 3. Signing Operations Metrics

**Location**: `pkg/tbtc/signing.go`, `pkg/tbtc/node.go`

These metrics track message signing operations, including success, failures, and timeouts.

| Metric Name | Type | Description |
|------------|------|-------------|
| `performance_signing_operations_total` | Counter | Total number of signing operations attempted |
| `performance_signing_success_total` | Counter | Total number of successful signing operations |
| `performance_signing_failed_total` | Counter | Total number of failed signing operations |
| `performance_signing_duration_seconds` | Histogram | Duration of signing operations (exposed as average and count) |
| `performance_signing_timeouts_total` | Counter | Total number of signing operations that timed out (all signers failed) |

**Implementation Details**:
- Operations counter increments at the start of each signing operation
- Success counter increments when a signature is successfully generated
- Failed counter increments when all signers fail
- Timeout counter increments when signing times out (subset of failures)
- Duration is recorded for both successful and failed operations

**Use Cases**:
- Monitor signing throughput and success rates
- Track signing performance and identify slow operations
- Detect timeout issues that may indicate network or coordination problems

---

### 4. Coordination Operations Metrics

**Location**: `pkg/tbtc/coordination.go`, `pkg/tbtc/node.go`

These metrics track coordination window detection and procedure execution.

| Metric Name | Type | Description |
|------------|------|-------------|
| `performance_coordination_windows_detected_total` | Counter | Total number of coordination windows detected |
| `performance_coordination_procedures_executed_total` | Counter | Total number of coordination procedures executed successfully |
| `performance_coordination_failed_total` | Counter | Total number of failed coordination procedures |
| `performance_coordination_duration_seconds` | Histogram | Duration of coordination procedures (exposed as average and count) |

**Implementation Details**:
- Windows detected counter increments when a new coordination window is found
- Procedures executed counter increments on successful coordination
- Failed counter increments when coordination fails (leader or follower errors)
- Duration is recorded for both successful and failed coordination procedures

**Use Cases**:
- Monitor coordination window detection frequency
- Track coordination success rates
- Identify coordination performance issues

---

### 5. Network Operations Metrics

**Location**: `pkg/net/libp2p/libp2p.go`, `pkg/net/libp2p/channel.go`, `pkg/net/libp2p/channel_manager.go`

These metrics track LibP2P network activity, including peer connections, message handling, and queue sizes.

#### Peer Connection Metrics

| Metric Name | Type | Description |
|------------|------|-------------|
| `performance_peer_connections_total` | Counter | Total number of peer connections established |
| `performance_peer_disconnections_total` | Counter | Total number of peer disconnections |

#### Message Metrics

| Metric Name | Type | Description |
|------------|------|-------------|
| `performance_message_broadcast_total` | Counter | Total number of messages broadcast to the network |
| `performance_message_received_total` | Counter | Total number of messages received from the network |

#### Queue Size Metrics

| Metric Name | Type | Description | Labels |
|------------|------|-------------|--------|
| `performance_incoming_message_queue_size` | Gauge | Current size of the incoming message queue | `channel` (channel name) |
| `performance_message_handler_queue_size` | Gauge | Current size of message handler queues | `channel` (channel name), `handler` (handler ID) |

**Note**: Queue sizes are monitored every minute. Maximum queue sizes:
- Incoming message queue: 4096
- Message handler queue: 512 per handler

#### Ping Test Metrics

| Metric Name | Type | Description |
|------------|------|-------------|
| `performance_ping_test_total` | Counter | Total number of ping tests performed |
| `performance_ping_test_success_total` | Counter | Total number of successful ping tests |
| `performance_ping_test_failed_total` | Counter | Total number of failed ping tests |
| `performance_ping_test_duration_seconds` | Histogram | Duration of ping tests (exposed as average and count) |

**Implementation Details**:
- Peer connection/disconnection counters increment on network events
- Message counters track broadcast and receive operations
- Queue size gauges are updated periodically (every minute)
- Ping tests are executed on peer connections and results are tracked

**Use Cases**:
- Monitor network connectivity and peer health
- Track message throughput
- Identify message processing bottlenecks (queue sizes)
- Monitor network latency (ping tests)

---

## Metric Types Explained

### Counters
- **Behavior**: Cumulative values that only increase
- **Use**: Track total occurrences of events
- **Example**: `performance_signing_operations_total`
- **Prometheus Format**: Exposed as a gauge (for compatibility)

### Gauges
- **Behavior**: Current values that can increase or decrease
- **Use**: Track current state (queue sizes, active operations)
- **Example**: `performance_wallet_dispatcher_active_actions`
- **Prometheus Format**: Standard gauge

### Histograms (Durations)
- **Behavior**: Track distributions of values (typically durations)
- **Use**: Measure operation durations
- **Example**: `performance_signing_duration_seconds`
- **Prometheus Format**: Exposed as:
  - `performance_<operation>_duration_seconds` (average)
  - `performance_<operation>_duration_seconds_count` (total count)

---

## Accessing Metrics

### Endpoint
Metrics are available at:
```
http://localhost:9601/metrics
```

### Configuration
Enable metrics by configuring the ClientInfo section:
```toml
[ClientInfo]
Port = 9601
NetworkMetricsTick = "1m"
EthereumMetricsTick = "10m"
BitcoinMetricsTick = "10m"
```

### Example Query
```bash
# Get all performance metrics
curl http://localhost:9601/metrics | grep performance_

# Get specific metric
curl http://localhost:9601/metrics | grep performance_signing_operations_total
```

---

## Prometheus Integration

All metrics are compatible with Prometheus and can be scraped using a Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'keep-node'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:9601']
```

---

## Monitoring Recommendations

### Key Metrics to Monitor

1. **Operation Success Rates**
   - `performance_signing_success_total / performance_signing_operations_total`
   - `performance_wallet_action_success_total / performance_wallet_actions_total`
   - `performance_coordination_procedures_executed_total / performance_coordination_windows_detected_total`

2. **Operation Durations**
   - Alert if `performance_signing_duration_seconds` exceeds 60 seconds (average)
   - Alert if `performance_dkg_duration_seconds` exceeds 300 seconds (average)
   - Alert if `performance_coordination_duration_seconds` exceeds expected thresholds

3. **Queue Sizes**
   - Alert if `performance_incoming_message_queue_size` exceeds 3000 (75% of capacity)
   - Alert if `performance_message_handler_queue_size` exceeds 400 (75% of capacity)

4. **Wallet Dispatcher**
   - Alert if `performance_wallet_dispatcher_rejected_total` rate > 5% of dispatched actions
   - Monitor `performance_wallet_dispatcher_active_actions` to understand wallet utilization

5. **Network Health**
   - Monitor `performance_peer_connections_total` vs `performance_peer_disconnections_total`
   - Alert if `performance_ping_test_failed_total` rate > 10% of ping tests

### Alert Thresholds

**High Priority**:
- `performance_signing_failed_total` rate > 10% of total operations
- `performance_wallet_action_failed_total` rate > 5% of total actions
- `performance_incoming_message_queue_size` > 3000

**Medium Priority**:
- `performance_wallet_dispatcher_rejected_total` rate > 5% of dispatched actions
- `performance_ping_test_failed_total` rate > 10% of ping tests
- `performance_coordination_failed_total` rate > 5% of coordination windows

**Low Priority**:
- `performance_signing_duration_seconds` > 60 seconds (average)
- `performance_dkg_duration_seconds` > 300 seconds (average)

---

## Implementation Architecture

### Component Wiring

Metrics are wired through the component hierarchy:

1. **Initialization** (`cmd/start.go`):
   - Creates `PerformanceMetrics` instance from `clientInfoRegistry`
   - Wires metrics into network provider

2. **Node Level** (`pkg/tbtc/node.go`):
   - Node receives metrics via `setPerformanceMetrics()`
   - Wires metrics into:
     - Wallet dispatcher
     - DKG executor
     - Signing executor
     - Coordination executor

3. **Network Level** (`pkg/net/libp2p/`):
   - Provider receives metrics via `SetMetricsRecorder()`
   - Wires metrics into:
     - Channel manager
     - Individual channels
     - Peer connection handlers

### Thread Safety

All metrics operations are thread-safe:
- Counters use mutex-protected maps
- Gauges use mutex-protected values
- Histograms use mutex-protected bucket maps

### Optional Metrics

Metrics are optional - if the metrics recorder is `nil`, operations continue normally without recording. This allows the system to function even when metrics are not configured.

---

## Summary

### Implemented Metrics by Category

✅ **Wallet Dispatcher**: 6 metrics (active actions, rejected actions, actions total, success, failure, duration)

✅ **DKG Operations**: 6 metrics (joined, failed, duration, validation, challenges, approvals)

✅ **Signing Operations**: 5 metrics (operations total, success, failure, duration, timeouts)

✅ **Coordination Operations**: 4 metrics (windows detected, procedures executed, failed, duration)

✅ **Network Operations**: 10 metrics (peer connections/disconnections, message broadcast/received, queue sizes, ping tests)

**Total**: 31 performance metrics implemented

### Not Yet Implemented

- **Beacon Relay Entry Metrics**: Relay entry generation for beacon nodes (defined but not instrumented)

---

## Related Documentation

- `docs/performance-metrics.adoc` - Comprehensive metrics documentation with Prometheus integration
- `docs/performance-metrics-implementation.md` - Implementation status and technical details
- `pkg/clientinfo/performance.go` - Core metrics implementation

---

*Last Updated: Based on current codebase implementation*



# Performance Metrics Implementation Status

## Overview

Performance metrics tracking has been implemented to monitor key operations of the Keep Core node. This document tracks the implementation status of various metrics.

## Implementation Status

### ✅ Fully Implemented

#### Wallet Dispatcher Metrics
- **Location**: `pkg/tbtc/wallet.go`
- **Metrics Tracked**:
  - `wallet_dispatcher_active_actions` (gauge) - Current number of wallets with active actions
  - `wallet_dispatcher_rejected_total` (counter) - Total rejected actions due to busy wallet
  - `wallet_actions_total` (counter) - Total wallet actions dispatched
  - `wallet_action_success_total` (counter) - Successful wallet actions
  - `wallet_action_failed_total` (counter) - Failed wallet actions
  - `wallet_action_duration_seconds` (histogram) - Duration of wallet actions

#### DKG (Distributed Key Generation) Metrics
- **Location**: `pkg/tbtc/dkg.go`
- **Metrics Tracked**:
  - `dkg_joined_total` (counter) - Total DKG joins (counts members joined)
  - `dkg_failed_total` (counter) - Failed DKG executions
  - `dkg_duration_seconds` (histogram) - DKG operation duration
  - `dkg_validation_total` (counter) - DKG result validations performed
  - `dkg_challenges_submitted_total` (counter) - DKG challenges submitted on-chain
  - `dkg_approvals_submitted_total` (counter) - DKG approvals submitted on-chain

### ✅ Network Operations Metrics
- **Location**: `pkg/net/libp2p/libp2p.go`, `pkg/net/libp2p/channel.go`, `pkg/net/libp2p/channel_manager.go`
- **Metrics Tracked**:
  - `peer_connections_total` (counter) - Total peer connections established
  - `peer_disconnections_total` (counter) - Total peer disconnections
  - `message_broadcast_total` (counter) - Total messages broadcast
  - `message_received_total` (counter) - Total messages received
  - `ping_test_total` (counter) - Total ping tests performed
  - `ping_test_success_total` (counter) - Successful ping tests
  - `ping_test_failed_total` (counter) - Failed ping tests
  - `incoming_message_queue_size` (gauge) - Current incoming message queue size (monitored every minute)
  - `message_handler_queue_size` (gauge) - Current message handler queue sizes (monitored every minute)

### 🔄 Ready for Implementation

The following metrics are defined in `pkg/clientinfo/performance.go` but require instrumentation:

#### Signing Operations
- ✅ **COMPLETED** - All signing metrics have been implemented
- **Location**: `pkg/tbtc/signing.go`, `pkg/tbtc/node.go`
- **Metrics Tracked**:
  - `signing_operations_total` (counter) - Total signing operations attempted
  - `signing_success_total` (counter) - Successful signing operations
  - `signing_failed_total` (counter) - Failed signing operations
  - `signing_duration_seconds` (histogram) - Duration of signing operations
  - `signing_timeouts_total` (counter) - Signing operations that timed out (all signers failed)

#### Network Operations
- ✅ **COMPLETED** - All network metrics have been implemented

#### Coordination Operations
- ✅ **COMPLETED** - All coordination metrics have been implemented
- **Location**: `pkg/tbtc/coordination.go`, `pkg/tbtc/node.go`
- **Metrics Tracked**:
  - `coordination_windows_detected_total` (counter) - Total coordination windows detected
  - `coordination_procedures_executed_total` (counter) - Total coordination procedures executed successfully
  - `coordination_failed_total` (counter) - Failed coordination procedures
  - `coordination_duration_seconds` (histogram) - Duration of coordination procedures

#### Beacon Relay Entry (Beacon Node)
- **Files to modify**: `pkg/beacon/entry/entry.go`, `pkg/beacon/node.go`
- **Metrics to add**:
  - `relay_entry_generation_total`
  - `relay_entry_success_total`
  - `relay_entry_failed_total`
  - `relay_entry_duration_seconds`
  - `relay_entry_timeout_reported_total`

## How Metrics Are Recorded

### Counter Metrics
```go
if metricsRecorder != nil {
    metricsRecorder.IncrementCounter("metric_name", 1)
}
```

### Duration Metrics
```go
startTime := time.Now()
// ... operation ...
if metricsRecorder != nil {
    metricsRecorder.RecordDuration("operation_duration_seconds", time.Since(startTime))
}
```

### Gauge Metrics
```go
if metricsRecorder != nil {
    metricsRecorder.SetGauge("queue_size", float64(queueLen))
}
```

## Integration Points

### Node Initialization
Metrics are initialized in `pkg/tbtc/tbtc.go`:
```go
if clientInfo != nil {
    perfMetrics := clientinfo.NewPerformanceMetrics(clientInfo)
    node.setPerformanceMetrics(perfMetrics)
}
```

### Component Wiring
Components receive metrics recorder via setter methods:
- `node.setPerformanceMetrics()` - wires metrics into node and all components
- `walletDispatcher.setMetricsRecorder()` - wires metrics into wallet dispatcher
- `dkgExecutor.setMetricsRecorder()` - wires metrics into DKG executor
- `signingExecutor.setMetricsRecorder()` - wires metrics into signing executor
- `coordinationExecutor.setMetricsRecorder()` - wires metrics into coordination executor
- `provider.SetMetricsRecorder()` - wires metrics into network provider
- `channelManager.setMetricsRecorder()` - wires metrics into channel manager and channels
- `channel.setMetricsRecorder()` - wires metrics into individual channels and starts queue monitoring

## Testing Metrics

To verify metrics are being recorded:

1. Start a node with client info enabled (port > 0)
2. Perform operations (wallet actions, DKG)
3. Query metrics endpoint:
   ```bash
   curl http://localhost:9601/metrics | grep performance_
   ```

Expected output should include:
```
performance_wallet_actions_total
performance_wallet_action_duration_seconds
performance_dkg_joined_total
performance_dkg_duration_seconds
```

## Next Steps

1. ✅ **Signing Metrics**: COMPLETED - All signing operations are now instrumented
2. ✅ **Coordination Metrics**: COMPLETED - All coordination operations are now instrumented
3. **Add Beacon Metrics**: Track relay entry generation (for beacon nodes)
4. ✅ **Network Metrics**: COMPLETED - All network operations are now instrumented

## Notes

- All metrics are optional - operations continue normally if metrics are disabled
- Metrics use thread-safe implementations with mutexes
- Duration metrics track both average duration and total count
- Counter metrics are cumulative and never decrease
- Gauge metrics reflect current state

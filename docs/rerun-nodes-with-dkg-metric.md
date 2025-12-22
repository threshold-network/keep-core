# How to Rerun Nodes Locally with DKG Requested Metric

This guide explains how to rebuild and restart your local nodes to enable the new `dkg_requested_total` metric.

## Prerequisites

- ✅ Geth Ethereum node running locally
- ✅ Contracts deployed (TokenStaking, RandomBeacon, WalletRegistry, etc.)
- ✅ Client configuration file (`configs/config.toml`) with `[clientInfo]` section enabled

## Step-by-Step Guide

### 1. Stop Running Nodes (if any)

```bash
# Stop all running keep-client processes
pkill -f keep-client

# Or if using node-specific scripts:
./configs/stop-all-nodes.sh  # if you have this script
```

### 2. Rebuild the Keep Client Binary

Since we modified the code (`pkg/tbtc/tbtc.go`), you need to rebuild the binary:

```bash
cd /Users/levakhnazarov/threshold/fork2/keep-core

# Build the keep-client binary
go build -o keep-client ./cmd

# Verify the binary was created
ls -lh keep-client
```

**Alternative:** If you're using Docker-based builds:

```bash
./scripts/build.sh
# This will build binaries in out/bin/
```

### 3. Verify Metrics Configuration

Ensure your `configs/config.toml` has the `[clientInfo]` section enabled:

```toml
[clientInfo]
Port = 9601
NetworkMetricsTick = 60
EthereumMetricsTick = 600
```

The metric will be automatically enabled when:
- ✅ `Port` is set (default: 9601)
- ✅ Client info endpoint is initialized
- ✅ Performance metrics are wired into the node

### 4. Start Nodes

#### Option A: Single Node (Development)

```bash
export KEEP_ETHEREUM_PASSWORD=password
export GETH_DATA_DIR=~/ethereum/data

# Start using the start script
./scripts/start.sh
# Select: 1 (config.toml), 1 (info log level)

# Or start directly:
./keep-client --config configs/config.toml start --developer
```

#### Option B: Multiple Nodes

```bash
# If you have a script to start all nodes:
./configs/start-all-nodes.sh

# Or start each node manually:
export KEEP_ETHEREUM_PASSWORD=password

# Node 1 (port 9601)
./keep-client --config configs/node1.toml start --developer > logs/node1.log 2>&1 &

# Node 2 (port 9602)
./keep-client --config configs/node2.toml start --developer > logs/node2.log 2>&1 &

# Node 3 (port 9603)
./keep-client --config configs/node3.toml start --developer > logs/node3.log 2>&1 &
```

### 5. Verify Nodes Started Successfully

```bash
# Check if nodes are running
ps aux | grep keep-client

# Check node logs for any errors
tail -f logs/node1.log  # or your log file

# Verify metrics endpoint is accessible
curl -s http://localhost:9601/metrics | head -20
```

### 6. Verify the DKG Requested Metric is Available

```bash
# Check if the metric exists (it will be 0 until a DKG is requested)
curl -s http://localhost:9601/metrics | grep dkg_requested

# Expected output (initially):
# performance_dkg_requested_total 0

# Or check all DKG metrics:
curl -s http://localhost:9601/metrics | grep performance_dkg
```

### 7. Test the Metric by Triggering a DKG Request

```bash
# Request a new wallet (this triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml \
  --developer

# Wait a few seconds for the event to be processed
sleep 5

# Check the metric again
curl -s http://localhost:9601/metrics | grep dkg_requested

# Expected output (after DKG request):
# performance_dkg_requested_total 1
```

### 8. Monitor DKG Request Events

```bash
# Watch logs for DKG started events
tail -f logs/node1.log | grep -i "DKG started\|dkg_requested"

# Or watch all node logs:
tail -f logs/node*.log | grep -i "DKG started"
```

## Expected Metric Behavior

The `performance_dkg_requested_total` metric will increment when:

1. ✅ A DKG started event is observed on-chain
2. ✅ The event is unique (not already processed by deduplicator)
3. ✅ Confirmation blocks have elapsed (20 blocks)
4. ✅ DKG state is confirmed as `AwaitingResult`

**Important:** The metric increments **after** confirmation, not immediately when the event is emitted. This ensures we only count valid, confirmed DKG requests.

## Troubleshooting

### Metric Not Appearing

**Check:**
1. ✅ Client info port is configured: `grep -A 3 "\[clientInfo\]" configs/config.toml`
2. ✅ Metrics endpoint is accessible: `curl http://localhost:9601/metrics`
3. ✅ Node logs show no errors: `tail -20 logs/node1.log`
4. ✅ Binary was rebuilt after code changes: `ls -lh keep-client`

### Metric Stays at 0

**Possible reasons:**
- No DKG requests have been made yet
- DKG events haven't been confirmed yet (wait ~20 blocks)
- DKG state is not `AwaitingResult` (check logs)

**Debug:**
```bash
# Check DKG state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml \
  --developer

# Check for DKG started events in logs
grep -i "DKG started\|observed DKG started event" logs/node*.log
```

### Multiple Nodes - Metric Only on One Node

**Note:** Each node tracks its own metrics. The metric increments on the node that:
- Observes the DKG started event
- Confirms the event
- Processes it

If you have multiple nodes, each will independently track DKG requests they observe.

## Quick Reference

```bash
# Complete workflow:
pkill -f keep-client                    # Stop nodes
go build -o keep-client ./cmd           # Rebuild binary
./keep-client --config configs/config.toml start --developer  # Start node
curl http://localhost:9601/metrics | grep dkg_requested  # Check metric

# Trigger DKG and verify:
KEEP_ETHEREUM_PASSWORD=password \
  ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
sleep 5
curl http://localhost:9601/metrics | grep dkg_requested  # Should show 1
```

## Related Metrics

The DKG requested metric works alongside other DKG metrics:

- `performance_dkg_joined_total` - Number of times node joined DKG
- `performance_dkg_failed_total` - Number of failed DKG attempts
- `performance_dkg_duration_seconds` - Duration of DKG operations
- `performance_dkg_validation_total` - Number of DKG validations
- `performance_dkg_challenges_submitted_total` - Number of DKG challenges
- `performance_dkg_approvals_submitted_total` - Number of DKG approvals
- `performance_dkg_requested_total` - Number of DKG requests (NEW!)

View all DKG metrics:
```bash
curl -s http://localhost:9601/metrics | grep performance_dkg
```


# How to Process DKG with 3 Running Nodes

## Important Limitation

**DKG requires 100 operators**, but you only have **3 nodes**. The sortition pool can select the same operator multiple times to fill all 100 slots, but this may cause issues.

## Prerequisites

Before processing DKG, ensure:

1. ✅ **All 3 operators are registered** in both RandomBeacon and WalletRegistry
2. ✅ **All 3 operators are in the sortition pool**
3. ✅ **All 3 nodes are running and connected**
4. ✅ **All operators have sufficient authorization** (40k T minimum)

## Step-by-Step Process

### Step 1: Verify Prerequisites

```bash
# Check node status
./configs/check-nodes.sh

# Check connectivity
for i in {1..3}; do
  echo "Node $i: $(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length') peers"
done

# Check operator registration
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i ($OPERATOR):"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

**Expected:** All operators should return `true` (in pool)

### Step 2: Ensure Operators Are in Sortition Pool

**CRITICAL:** Operators must be in the sortition pool before DKG can work!

**Nodes automatically join the pool** when they start, but only if:
- Chaosnet is **not active**, OR
- Chaosnet is active AND the operator is a **beta operator**

Check if operators are in pool:
```bash
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i ($OPERATOR):"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

If operators are **not in the pool** (`false`), check:

1. **Is chaosnet active?** (If yes, operators must be beta operators)
2. **Is pool locked?** (If yes, wait for DKG to complete/timeout)
3. **Have nodes tried to join?** (They check every 6 hours by default)

**To manually join operators** (if pool is unlocked and policy allows):

```bash
# Join each operator using their node config
for i in {1..3}; do
  echo "Joining Node $i operator to pool..."
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry join-sortition-pool \
    --submit --config "configs/node$i.toml" --developer
  sleep 2
done
```

**Note:** 
- Pool must be unlocked (DKG state = IDLE)
- If chaosnet is active, operators must be beta operators (use Hardhat tasks to add them)
- Nodes check pool status every 6 hours, so they may join automatically

### Step 3: Trigger DKG

```bash
# Request new wallet (triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

This will:
- Lock the sortition pool
- Request relay entry from Random Beacon
- Start DKG process

### Step 4: Wait for Seed

```bash
# Monitor DKG state
./scripts/monitor-dkg.sh

# Or check manually
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**Expected:** State should change from `0` → `1` → `2`

### Step 5: Verify Group Selection

Once pool is locked (state 1 or 2), check which operators were selected:

```bash
# Select group (shows operator IDs)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer
```

**Note:** With only 3 operators, the same operators will be selected multiple times to fill 100 slots.

### Step 6: Monitor DKG Progress

```bash
# Watch logs for DKG activity
tail -f logs/node*.log | grep -iE "dkg|keygen|member|protocol"

# Or use monitoring script
./scripts/monitor-dkg.sh
```

**What to look for:**
- `keygen/prepare.go` messages (key generation)
- DKG protocol messages
- Member coordination messages

### Step 7: Wait for DKG Completion

DKG will complete when:
- Operators generate keys successfully
- Result is submitted to chain
- Result is approved (after challenge period)

**Monitor:**
```bash
# Check for result submission
tail -f logs/node*.log | grep -iE "DkgResultSubmitted|result.*submitted"

# Check for wallet creation
tail -f logs/node*.log | grep -iE "WalletCreated|wallet.*created"
```

### Step 8: Verify Completion

```bash
# Check DKG state (should return to 0 = IDLE)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Should return: 0 (IDLE)
```

## Complete Workflow Script

```bash
#!/bin/bash
# Complete DKG workflow with 3 nodes

set -eou pipefail

CONFIG="configs/config.toml"

echo "=========================================="
echo "DKG Process with 3 Nodes"
echo "=========================================="
echo ""

# Step 1: Verify prerequisites
echo "Step 1: Checking prerequisites..."
echo ""

# Check nodes are running
echo "Node status:"
./configs/check-nodes.sh | head -5

# Check connectivity
echo ""
echo "Connectivity:"
for i in {1..3}; do
  PEERS=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq '.connected_peers | length' 2>/dev/null || echo "0")
  echo "  Node $i: $PEERS peers"
done

# Check operators are in pool
echo ""
echo "Operator pool status:"
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null)
  if [ -n "$OPERATOR" ] && [ "$OPERATOR" != "null" ]; then
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
      "$OPERATOR" --config "$CONFIG" --developer 2>&1 | tail -1)
    echo "  Node $i ($OPERATOR): $IN_POOL"
  fi
done

echo ""
echo "Step 2: Triggering DKG..."
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config "$CONFIG" --developer

echo ""
echo "Step 3: Waiting for pool to lock..."
sleep 5

STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG" --developer 2>&1 | tail -1)

echo "DKG State: $STATE"

if [ "$STATE" != "0" ]; then
  echo ""
  echo "Step 4: Pool is locked. Selecting group..."
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
    --config "$CONFIG" --developer
  
  echo ""
  echo "Step 5: Monitoring DKG progress..."
  echo "Watch logs: tail -f logs/node*.log | grep -i dkg"
  echo ""
  echo "Monitor script: ./scripts/monitor-dkg.sh"
else
  echo "⚠ Pool is still unlocked. Wait a few seconds and check again."
fi

echo ""
echo "=========================================="
echo "DKG Process Started"
echo "=========================================="
```

## Potential Issues with 3 Nodes

### Issue 1: Same Operator Selected Multiple Times

With only 3 operators, each operator will be selected ~33 times to fill 100 slots. This means:
- Each node needs to handle multiple member indexes
- DKG protocol must coordinate between the same operator multiple times
- May cause confusion in member identification

### Issue 2: Insufficient Operators

If operators are not properly registered or not in the pool:
- Group selection may fail
- DKG cannot proceed

**Solution:** Ensure all operators are registered and in the pool.

### Issue 3: Connectivity Issues

If nodes can't communicate:
- DKG protocol cannot complete
- Key generation fails

**Solution:** Verify peer connectivity before triggering DKG.

## Troubleshooting

### DKG Stuck

```bash
# Check state
./scripts/check-dkg-state.sh

# Check timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# If timed out, notify timeout
./scripts/stop-dkg.sh
```

### Operators Not Selected

```bash
# Check if operators are in pool
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i:"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer
done
```

### Nodes Not Connected

```bash
# Restart nodes with proper peer configuration
./configs/stop-all-nodes.sh
sleep 3
./scripts/update-peer-ids.sh
./configs/start-all-nodes.sh
sleep 10
```

## Expected Timeline

- **Trigger DKG**: Immediate
- **Pool locks**: ~5 seconds
- **Seed arrives**: Depends on Random Beacon
- **DKG execution**: 10-30 minutes (with 3 nodes, may take longer)
- **Result submission**: After DKG completes
- **Challenge period**: ~48 hours (in production), shorter in dev
- **Wallet created**: After challenge period

## Quick Start

```bash
# Run complete workflow
./scripts/process-dkg-3-nodes.sh

# Or manually:
# 1. Verify prerequisites (nodes running, operators registered)
# 2. Check if operators are in pool
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done

# 3. If not in pool, wait for auto-join (nodes check every 6 hours) or manually join
# 4. Trigger DKG
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer

# 5. Monitor progress
./scripts/monitor-dkg.sh
```

## Summary

To process DKG with 3 nodes:
1. ✅ Ensure all 3 operators are registered and in pool
2. ✅ Ensure nodes are connected (2 peers each)
3. ✅ Trigger DKG with `request-new-wallet`
4. ✅ Monitor progress with `monitor-dkg.sh`
5. ✅ Wait for completion or timeout (~89 minutes)
6. ✅ Approve result when ready

**Note:** With only 3 operators, DKG may take longer or encounter issues since it's designed for 100 operators. Consider registering more operators for better results.

# How to Test Nodes in Sortition Pool

## Quick Test Script

Use the automated test script:

```bash
./scripts/test-nodes-in-pool.sh
```

This script checks:
- ✅ Nodes are running
- ✅ Operators are registered
- ✅ Operators are in sortition pool
- ✅ Pool state (locked/unlocked)
- ✅ Authorized stake amounts

## Manual Testing Commands

### 1. Check if Operator is in Pool

```bash
# Get operator address from node diagnostics
OPERATOR=$(curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address')

# Check if in pool
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
  "$OPERATOR" --config configs/config.toml --developer
```

**Expected output:** `true` or `false`

### 2. Check All Nodes at Once

```bash
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i ($OPERATOR):"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

### 3. Check Operator Authorization/Stake

```bash
OPERATOR="0x..."  # Operator address

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry authorized-stake \
  "$OPERATOR" --config configs/config.toml --developer
```

**Expected:** Returns authorized stake amount (should be >= 40k T minimum)

### 4. Check Pool State

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**Expected values:**
- `0` = IDLE (pool unlocked, operators can join)
- `1` = AWAITING_SEED (pool locked)
- `2` = AWAITING_RESULT (pool locked, DKG in progress)
- `3` = CHALLENGE (pool locked)

### 5. Check if Pool is Locked

```bash
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer 2>&1 | tail -1)

if [ "$STATE" = "0" ]; then
  echo "Pool is UNLOCKED (operators can join)"
else
  echo "Pool is LOCKED (DKG in progress)"
fi
```

### 6. Verify Node Connectivity

```bash
for i in {1..3}; do
  echo "Node $i:"
  curl -s http://localhost:960$i/diagnostics | jq '{
    operator: .client_info.chain_address,
    peers: (.connected_peers | length),
    network_id: .client_info.network_id
  }'
done
```

**Expected:**
- Each node should show its operator address
- Each node should have 2 connected peers (in 3-node setup)
- Network ID should be present

## Complete Test Workflow

```bash
#!/bin/bash
# Complete test workflow

echo "=== Testing Nodes in Pool ==="
echo ""

# 1. Check nodes are running
echo "1. Checking nodes..."
./configs/check-nodes.sh

# 2. Test pool status
echo ""
echo "2. Testing pool status..."
./scripts/test-nodes-in-pool.sh

# 3. If all in pool, test DKG trigger
echo ""
echo "3. Testing DKG trigger (dry run)..."
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --config configs/config.toml --developer 2>&1 | head -10
```

## Expected Test Results

### ✅ All Tests Pass

```
Node 1: ✓ In Pool: YES
Node 2: ✓ In Pool: YES
Node 3: ✓ In Pool: YES
Pool State: IDLE (unlocked)
✅ All operators are in the sortition pool!
```

### ⚠️ Some Tests Fail

```
Node 1: ✗ In Pool: NO
Node 2: ✓ In Pool: YES
Node 3: ✗ In Pool: NO
⚠ Some operators are NOT in the pool
```

**Next steps:**
1. Check if chaosnet is active (requires beta operators)
2. Check if pool is locked (wait for DKG to complete)
3. Check operator authorization (must be >= 40k T)
4. Manually join operators to pool

## Troubleshooting Failed Tests

### Issue: Operator Not in Pool

**Check:**
```bash
# 1. Is pool locked?
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# 2. Is operator registered?
OPERATOR="0x..."
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-registered \
  "$OPERATOR" --config configs/config.toml --developer

# 3. Is authorization sufficient?
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry authorized-stake \
  "$OPERATOR" --config configs/config.toml --developer
```

**Fix:**
- If pool locked: Wait for DKG or notify timeout
- If not registered: Register operator
- If insufficient authorization: Top up stake
- If chaosnet active: Add as beta operator

### Issue: Node Not Running

**Check:**
```bash
curl -s http://localhost:9601/diagnostics
```

**Fix:**
```bash
./configs/start-all-nodes.sh
```

### Issue: Cannot Get Operator Address

**Check logs:**
```bash
tail -50 logs/node1.log | grep -i "operator\|chain_address"
```

**Fix:**
- Ensure node is fully started
- Check node configuration
- Verify Ethereum connection

## Advanced Testing

### Test Group Selection (Requires Pool Locked)

```bash
# First trigger DKG to lock pool
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer

# Wait for pool to lock
sleep 5

# Then test group selection
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer
```

### Test Pool Size

```bash
# Check how many operators are in pool (via contract)
# Note: This may require direct contract interaction
```

### Test Operator Weight

```bash
OPERATOR="0x..."

# Get authorized stake
AUTH_STAKE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry authorized-stake \
  "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1)

echo "Authorized stake: $AUTH_STAKE"
```

## Continuous Monitoring

Monitor pool status continuously:

```bash
watch -n 5 './scripts/test-nodes-in-pool.sh'
```

Or create a monitoring script:

```bash
#!/bin/bash
# Monitor pool status every 10 seconds

while true; do
  clear
  echo "=== Pool Status Monitor ==="
  echo "Time: $(date)"
  echo ""
  ./scripts/test-nodes-in-pool.sh | tail -15
  sleep 10
done
```

## Summary

**Quick test:**
```bash
./scripts/test-nodes-in-pool.sh
```

**Manual checks:**
- `is-operator-in-pool` - Check if operator is in pool
- `authorized-stake` - Check operator authorization
- `get-wallet-creation-state` - Check pool state
- Node diagnostics - Check node connectivity

**Expected result:** All 3 operators should return `true` for `is-operator-in-pool` when pool is unlocked and prerequisites are met.

# Why DKG Gets Stuck in AWAITING_RESULT State

## Root Cause

DKG gets stuck in `AWAITING_RESULT` (state `2`) when operators **cannot communicate with each other** via LibP2P. The DKG protocol requires operators to coordinate off-chain, and without peer connectivity, they cannot complete the key generation process.

## Common Causes

### 1. **No Peer Connectivity** (Most Common)

**Symptom:**
```bash
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'
# Returns: 0
```

**Cause:**
- Nodes don't have peer IDs configured
- Nodes weren't restarted after updating peer IDs
- Peer IDs are incorrect or outdated

**Solution:**
1. Update peer IDs:
   ```bash
   ./scripts/update-peer-ids.sh
   ```

2. Restart nodes:
   ```bash
   ./configs/stop-all-nodes.sh
   sleep 3
   ./configs/start-all-nodes.sh
   sleep 10
   ```

3. Verify connectivity:
   ```bash
   for i in {1..3}; do
     echo "Node $i peers:"
     curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length'
   done
   ```

### 2. **Insufficient Operators Selected**

**Symptom:**
- DKG state is `2` but only 1-2 operators are running
- DKG requires minimum group size (typically 100+ operators)

**Solution:**
- Register more operators
- Ensure enough operators are in the sortition pool
- Check operator count:
  ```bash
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry \
    sortition-pool operators-count --config configs/config.toml --developer
  ```

### 3. **DKG Timeout**

**Symptom:**
```bash
./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
# Returns: true
```

**Solution:**
Notify timeout to unlock:
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer
```

### 4. **Network/Port Issues**

**Symptom:**
- Nodes can't bind to ports
- Connection refused errors in logs

**Solution:**
- Check ports aren't blocked:
  ```bash
  netstat -an | grep -E "3919|3920|3921"
  ```
- Ensure ports match in config files
- Check firewall settings

### 5. **Operator Not Selected for DKG**

**Symptom:**
- Your operator is running but wasn't selected
- Other operators are participating

**Solution:**
- Check if your operator is in the sortition pool:
  ```bash
  OPERATOR=$(curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address')
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer
  ```
- Ensure sufficient authorization
- Wait for next DKG round

## Diagnostic Steps

### Step 1: Check DKG State

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

### Step 2: Check Peer Connectivity

```bash
# Check each node
for i in {1..3}; do
  echo "=== Node $i ==="
  curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length'
done
```

**Expected:** Each node should have at least 1-2 connected peers

### Step 3: Check Peer Configuration

```bash
# Verify peer IDs are configured
grep -A 1 "^Peers" configs/node*.toml
```

**Expected:** Each node (except node 1) should have peer IDs of other nodes

### Step 4: Check Logs for Errors

```bash
tail -100 logs/node*.log | grep -iE "error|fatal|cannot.*connect|peer.*fail"
```

### Step 5: Check if DKG Timed Out

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
```

## Fixing the Issue

### Complete Fix Procedure

1. **Update Peer IDs:**
   ```bash
   ./scripts/update-peer-ids.sh
   ```

2. **Verify Peer Configuration:**
   ```bash
   # Node 1 should have no peers (first node)
   # Node 2 should have Node 1's peer ID
   # Node 3 should have Node 1 and Node 2's peer IDs
   grep Peers configs/node*.toml
   ```

3. **Restart All Nodes:**
   ```bash
   ./configs/stop-all-nodes.sh
   sleep 3
   ./configs/start-all-nodes.sh
   sleep 10
   ```

4. **Verify Connectivity:**
   ```bash
   ./configs/check-nodes.sh
   
   # Check connected peers
   for i in {1..3}; do
     echo "Node $i: $(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length') peers"
   done
   ```

5. **If Still Stuck, Notify Timeout:**
   ```bash
   # Check timeout
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
     --config configs/config.toml --developer
   
   # If true, notify timeout
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
     --submit --config configs/config.toml --developer
   
   # Wait for state to reset to IDLE
   sleep 5
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
     --config configs/config.toml --developer
   ```

6. **Trigger New DKG:**
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
     --submit --config configs/config.toml --developer
   ```

## Prevention

1. **Always restart nodes after updating peer IDs**
2. **Verify connectivity before triggering DKG**
3. **Ensure all operators are running and connected**
4. **Monitor logs for connection errors**

## Quick Reference

```bash
# Check why DKG is stuck
./scripts/monitor-dkg.sh

# Fix peer connectivity
./scripts/update-peer-ids.sh
./configs/stop-all-nodes.sh
./configs/start-all-nodes.sh

# Verify fix
for i in {1..3}; do
  echo "Node $i: $(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length') peers"
done

# If timed out, unlock
./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer
```


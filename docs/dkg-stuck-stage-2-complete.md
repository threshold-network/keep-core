# When DKG Gets Stuck in Stage 2 (AWAITING_RESULT)

## Overview

DKG gets stuck in **Stage 2 (AWAITING_RESULT)** when the off-chain protocol cannot complete successfully and no result is submitted to the blockchain. This is the most common DKG failure scenario.

## Understanding Stage 2

**Stage 2 (AWAITING_RESULT)** means:
- ✅ Sortition pool is locked
- ✅ Group of operators has been selected
- ✅ Random seed has been received
- ⏳ Operators are supposed to generate keys collaboratively off-chain
- ⏳ A result should be submitted to the blockchain
- ❌ **But the result never arrives**

## Root Causes: When DKG Gets Stuck

### 1. **No Peer Connectivity** (Most Common - ~80% of cases)

**What Happens:**
- Operators cannot communicate via LibP2P
- DKG protocol requires peer-to-peer messaging
- Without connectivity, operators cannot coordinate key generation

**Symptoms:**
```bash
# Check peer count (should be > 0)
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'
# Returns: 0
```

**Logs Show:**
```
ERROR ... failed to send message to peer ...
ERROR ... connection refused ...
WARN ... no peers connected ...
```

**Why It Happens:**
- Peer IDs not configured in config files
- Nodes restarted before peer IDs were updated
- Incorrect peer IDs (typos, wrong format)
- Network/firewall blocking LibP2P ports
- Nodes started in wrong order

**Fix:**
```bash
# 1. Update peer IDs
./scripts/update-peer-ids.sh

# 2. Restart all nodes
./configs/stop-all-nodes.sh
sleep 3
./configs/start-all-nodes.sh
sleep 10

# 3. Verify connectivity
for i in {1..3}; do
  echo "Node $i: $(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length') peers"
done
```

---

### 2. **Insufficient Signatures for Result Submission**

**What Happens:**
- DKG protocol completes successfully
- Operators generate keys
- But not enough operators sign the result
- Result cannot be submitted (requires quorum)

**Symptoms:**
```bash
# Check logs for signature collection
tail -f logs/node*.log | grep -iE "signature|quorum|group.*quorum"
```

**Logs Show:**
```
ERROR ... could not submit result with [X] signatures for group quorum [Y]
WARN ... insufficient signatures collected ...
```

**Why It Happens:**
- Not enough operators participated in DKG
- Some operators crashed/disconnected during result signing phase
- Operators marked as inactive/misbehaved
- Group quorum requirement not met

**Requirements:**
- For 100 operators: Need ~51+ signatures (honest threshold)
- For 3 operators: Need all 3 signatures (or 2 if threshold allows)

**Fix:**
- Ensure all selected operators are running
- Check operator logs for participation
- Verify operators weren't marked as misbehaved
- Wait for more operators to sign (if possible)

---

### 3. **DKG Protocol Execution Failure**

**What Happens:**
- Operators connect but protocol fails during execution
- Key generation fails at some phase
- No result is produced

**Symptoms:**
```bash
# Check logs for protocol errors
tail -f logs/node*.log | grep -iE "protocol.*fail|keygen.*fail|dkg.*error"
```

**Logs Show:**
```
ERROR ... failed to execute DKG: [protocol error]
ERROR ... key generation failed: [cryptographic error]
FATAL ... DKG attempt failed: [specific error]
```

**Why It Happens:**

**a) Cryptographic Failures:**
- Paillier key generation fails (CPU/memory issues)
- Secret sharing validation fails
- Invalid shares detected
- Proof verification fails

**b) Member Failures:**
- Operators marked as inactive (IA)
- Operators marked as disqualified (DQ)
- Not enough honest operators remain

**c) Timeout During Protocol:**
- Protocol phases take too long
- Network delays cause timeouts
- CPU overload slows computation

**Fix:**
```bash
# 1. Check specific error in logs
tail -100 logs/node*.log | grep -iE "error|fatal" | tail -20

# 2. Check if operators are still running
./configs/check-nodes.sh

# 3. Check CPU/memory usage
top -p $(pgrep -f keep-client)

# 4. If protocol failed, wait for timeout and retry
```

---

### 4. **Result Validation Failure**

**What Happens:**
- DKG completes successfully
- Result is prepared
- But result validation fails before submission
- Result is never submitted

**Symptoms:**
```bash
# Check logs for validation errors
tail -f logs/node*.log | grep -iE "invalid.*result|validation.*fail|result.*invalid"
```

**Logs Show:**
```
ERROR ... invalid DKG result
ERROR ... cannot validate DKG result: [error]
WARN ... result validation failed ...
```

**Why It Happens:**
- Result doesn't meet contract requirements
- Group public key is invalid
- Misbehaved members list is incorrect
- Signatures don't match expected format
- Result hash doesn't match

**Fix:**
- Check contract validation logic
- Verify result assembly code
- Check if result meets all requirements
- Review misbehaved members calculation

---

### 5. **State Changed Before Submission**

**What Happens:**
- DKG completes successfully
- Result is ready to submit
- But another operator submits first
- Or DKG state changes (timeout, challenge)
- Submission is aborted

**Symptoms:**
```bash
# Check logs for submission abort
tail -f logs/node*.log | grep -iE "aborting.*submission|no longer awaiting|state.*changed"
```

**Logs Show:**
```
INFO ... DKG is no longer awaiting the result; aborting DKG result on-chain submission
INFO ... someone else submitted the result; giving up
```

**Why It Happens:**
- Multiple operators try to submit simultaneously
- Another operator's submission succeeds first
- DKG timed out while preparing submission
- Result was challenged before submission

**Fix:**
- This is normal behavior (only one submission needed)
- Check if result was successfully submitted by another operator
- Verify DKG state changed to CHALLENGE (stage 3)

---

### 6. **Insufficient Operators Selected**

**What Happens:**
- DKG requires 100 operators
- Only 3 operators are in the pool
- Same operators selected multiple times
- Protocol may fail or take extremely long

**Symptoms:**
```bash
# Check operator count in pool (returns array of operator addresses)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool operators-in-pool \
  --config configs/config.toml --developer

# Count operators
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool operators-in-pool \
  --config configs/config.toml --developer 2>&1 | jq 'length'
# Returns: 3 (should be 100+)
```

**Why It Happens:**
- Not enough operators registered
- Operators not in sortition pool
- Pool size is too small

**Fix:**
- Register more operators (ideally 100+)
- Ensure operators join sortition pool
- For testing with 3 nodes, expect longer execution time

---

### 7. **DKG Timeout**

**What Happens:**
- DKG takes longer than allowed timeout
- Contract prevents result submission after timeout
- DKG remains stuck in AWAITING_RESULT

**Symptoms:**
```bash
# Check if timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
# Returns: true
```

**Why It Happens:**
- Key generation takes too long (CPU-bound operations)
- Network delays slow protocol
- Too few operators (each handles many member slots)
- Protocol phases timeout internally

**Timeout Duration:**
- Default: ~89 minutes (536 blocks × ~10s block time)
- For 3 nodes: May timeout before completion

**Fix:**
```bash
# 1. Check timeout status
./scripts/check-dkg-state.sh

# 2. If timed out, notify timeout
./scripts/stop-dkg.sh

# 3. Wait for state to reset to IDLE
sleep 5
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# 4. Fix underlying issue and retry
```

---

### 8. **Operator Crashes During DKG**

**What Happens:**
- DKG starts successfully
- Some operators crash/disconnect mid-protocol
- Remaining operators cannot complete
- Result never submitted

**Symptoms:**
```bash
# Check if all nodes are running
./configs/check-nodes.sh
# Some nodes may be down
```

**Logs Show:**
```
ERROR ... connection lost to peer ...
ERROR ... member [X] is inactive ...
WARN ... not enough active members ...
```

**Why It Happens:**
- Node crashes (OOM, panic, etc.)
- Network disconnection
- Process killed
- System restart

**Fix:**
```bash
# 1. Check node status
./configs/check-nodes.sh

# 2. Restart crashed nodes
# (restart specific node)

# 3. If DKG already failed, wait for timeout and retry
```

---

### 9. **Blockchain Connection Issues**

**What Happens:**
- DKG completes successfully
- Result is ready
- But cannot submit to blockchain
- Connection to Ethereum node fails

**Symptoms:**
```bash
# Check Ethereum connection
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
```

**Logs Show:**
```
ERROR ... failed to submit result: connection refused
ERROR ... cannot connect to Ethereum node
ERROR ... transaction failed: network error
```

**Why It Happens:**
- Geth node stopped/crashed
- Network partition
- RPC endpoint incorrect
- Gas issues (though less likely for result submission)

**Fix:**
```bash
# 1. Check Geth is running
ps aux | grep geth

# 2. Restart Geth if needed
# (restart Geth node)

# 3. Verify connection
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
```

---

### 10. **Incorrect Configuration**

**What Happens:**
- Nodes misconfigured
- Wrong contract addresses
- Incorrect network settings
- DKG cannot proceed correctly

**Symptoms:**
```bash
# Check config errors
tail -f logs/node*.log | grep -iE "config.*error|validation.*fail|missing.*config"
```

**Why It Happens:**
- Wrong `WalletRegistry` address
- Incorrect `RandomBeacon` address
- Wrong network ID
- Missing required config values

**Fix:**
- Verify all config files are correct
- Check contract addresses match deployed contracts
- Ensure network settings match Geth node
- Restart nodes after config changes

---

## Diagnostic Checklist

When DKG is stuck in Stage 2, check these in order:

### ✅ Step 1: Check DKG State
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
# Should return: 2 (AWAITING_RESULT)
```

### ✅ Step 2: Check Timeout Status
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
# If true → DKG timed out, notify timeout
```

### ✅ Step 3: Check Peer Connectivity
```bash
for i in {1..3}; do
  PEERS=$(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length')
  echo "Node $i: $PEERS peers"
done
# Each should have at least 1-2 peers
```

### ✅ Step 4: Check Node Status
```bash
./configs/check-nodes.sh
# All nodes should be running
```

### ✅ Step 5: Check Logs for Errors
```bash
tail -100 logs/node*.log | grep -iE "error|fatal|fail" | tail -20
# Look for specific error messages
```

### ✅ Step 6: Check DKG Activity
```bash
tail -f logs/node*.log | grep -iE "dkg|keygen|protocol|member"
# Should see active DKG messages
```

### ✅ Step 7: Check Ethereum Connection
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
# Should return block number
```

## Quick Fix Procedure

If DKG is stuck, try this sequence:

```bash
# 1. Check current state
./scripts/check-dkg-state.sh

# 2. Check peer connectivity
for i in {1..3}; do
  echo "Node $i: $(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length') peers"
done

# 3. If no peers, fix connectivity
if [ "$(curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length')" = "0" ]; then
  echo "Fixing peer connectivity..."
  ./scripts/update-peer-ids.sh
  ./configs/stop-all-nodes.sh
  sleep 3
  ./configs/start-all-nodes.sh
  sleep 10
fi

# 4. Check if timed out
TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer 2>&1 | tail -1)

if [ "$TIMED_OUT" = "true" ]; then
  echo "DKG timed out, notifying timeout..."
  ./scripts/stop-dkg.sh
  sleep 5
  echo "DKG reset. You can trigger a new DKG now."
else
  echo "DKG still in progress. Monitor with: ./scripts/monitor-dkg.sh"
fi
```

## Prevention

To prevent DKG from getting stuck:

1. **✅ Always verify peer connectivity before triggering DKG**
   ```bash
   for i in {1..3}; do
     PEERS=$(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length')
     if [ "$PEERS" = "0" ]; then
       echo "⚠ Node $i has no peers! Fix connectivity first."
       exit 1
     fi
   done
   ```

2. **✅ Ensure all nodes are running**
   ```bash
   ./configs/check-nodes.sh
   ```

3. **✅ Update peer IDs after node restarts**
   ```bash
   ./scripts/update-peer-ids.sh
   ```

4. **✅ Monitor DKG progress**
   ```bash
   ./scripts/monitor-dkg.sh
   ```

5. **✅ Check logs regularly**
   ```bash
   tail -f logs/node*.log | grep -iE "dkg|error"
   ```

## Summary

DKG gets stuck in Stage 2 (AWAITING_RESULT) when:

| Cause | Frequency | Fix Difficulty |
|-------|-----------|---------------|
| No peer connectivity | ~80% | Easy (update peer IDs, restart) |
| Insufficient signatures | ~5% | Medium (check operator participation) |
| Protocol execution failure | ~5% | Hard (check logs, fix underlying issue) |
| Result validation failure | ~2% | Hard (check result assembly) |
| State changed before submission | ~3% | Normal (another operator submitted) |
| Insufficient operators | ~2% | Medium (register more operators) |
| DKG timeout | ~2% | Easy (notify timeout, retry) |
| Operator crashes | ~1% | Easy (restart nodes) |
| Blockchain connection issues | ~1% | Easy (restart Geth) |
| Incorrect configuration | ~1% | Medium (fix config) |

**Most Common Fix:** Update peer IDs and restart nodes.

**Quick Diagnostic:** Run `./scripts/monitor-dkg.sh` to see current status.

**Quick Fix:** Run `./scripts/update-peer-ids.sh && ./configs/stop-all-nodes.sh && ./configs/start-all-nodes.sh`

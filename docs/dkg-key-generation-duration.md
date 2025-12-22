# DKG Key Generation Duration

## Expected Duration

### Standard Duration (100 Operators)

**Key generation phase:** ~36 minutes

**Calculation:**
- Off-chain DKG protocol: **216 blocks maximum** (per WalletRegistry.sol)
- Local Geth block time: **~10 seconds**
- Duration: `216 blocks × 10 seconds = 2160 seconds = 36 minutes`

### With 3 Nodes (Your Setup)

**Estimated duration:** **30-60 minutes**

**Why longer:**
- Each operator handles **~33 member slots** (instead of 1)
- More coordination overhead per operator
- Protocol designed for 100 distinct operators
- May encounter retries or delays

**Factors affecting duration:**
- ✅ **Node connectivity** - Good peer connections speed things up
- ✅ **Network latency** - Local network is fast
- ⚠️ **Operator load** - Each operator doing 33x the work
- ⚠️ **Coordination complexity** - More complex with repeated operators

## DKG State Timeline

The DKG process goes through these states:

1. **State 0 (IDLE)** → **State 1 (AWAITING_SEED)**
   - Duration: ~5 seconds
   - Pool locks, seed requested from Random Beacon

2. **State 1 (AWAITING_SEED)** → **State 2 (AWAITING_RESULT)**
   - Duration: Depends on Random Beacon
   - Seed arrives, DKG starts

3. **State 2 (AWAITING_RESULT)** ← **You are here: "Operators are generating keys"**
   - Duration: **30-60 minutes** (with 3 nodes)
   - Operators generate keys, coordinate via LibP2P
   - This is the longest phase

4. **State 2 → State 3 (CHALLENGE)**
   - Duration: Immediate after result submission
   - Result submitted to chain

5. **State 3 (CHALLENGE)** → **State 0 (IDLE)**
   - Duration: Challenge period (~48 hours in production, shorter in dev)
   - Result approved, wallet created

## Monitoring Key Generation

### Check DKG State

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**Expected:** `2` (AWAITING_RESULT) during key generation

### Watch Logs for Keygen Activity

```bash
# Watch for keygen messages
tail -f logs/node*.log | grep -iE "keygen|generating|member.*key|protocol"

# Or check recent activity
tail -100 logs/node*.log | grep -iE "keygen|member"
```

**What to look for:**
- `keygen/prepare.go` - Key generation preparation
- `member` - Member coordination messages
- `protocol` - DKG protocol messages
- `broadcast` - Message broadcasting

### Monitor Progress Script

```bash
./scripts/monitor-dkg.sh
```

This shows:
- Current DKG state
- Time elapsed
- Keygen activity detection

## What Happens During Key Generation

1. **Group Selection** - Operators selected (your 3 operators, each ~33 times)
2. **Seed Distribution** - DKG seed shared among members
3. **Key Generation** - Each member generates their share of the group key
4. **Coordination** - Members exchange messages via LibP2P
5. **Verification** - Members verify each other's contributions
6. **Result Assembly** - Group public key assembled from shares
7. **Result Submission** - First member submits result to chain

## Troubleshooting Slow Key Generation

### If Taking Longer Than Expected

**Check connectivity:**
```bash
# Check peer connections
for i in {1..3}; do
  echo "Node $i:"
  curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length'
done
```

**Expected:** Each node should have 2 connected peers

**Check for errors:**
```bash
tail -50 logs/node*.log | grep -iE "error|fail|timeout" | grep -v "notifications not supported"
```

**Check DKG timeout:**
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
```

**If timed out:**
- DKG timeout is **536 blocks** = **~89 minutes** (with 10s block time)
- If keygen takes longer, DKG will timeout
- Use `notify-dkg-timeout` to reset if needed

## Expected Timeline Summary

| Phase | Duration | State |
|-------|----------|-------|
| Trigger DKG | Immediate | 0 → 1 |
| Pool locks | ~5 seconds | 1 |
| Seed arrives | Depends on Random Beacon | 1 → 2 |
| **Key generation** | **30-60 min** | **2** |
| Result submission | Immediate | 2 → 3 |
| Challenge period | ~48 hours (prod) | 3 → 0 |
| Wallet created | After challenge | 0 |

## Quick Reference

**Current phase duration:**
- **Key generation:** 30-60 minutes (with 3 nodes)
- **Maximum timeout:** 89 minutes (536 blocks × 10s)

**Monitor:**
```bash
./scripts/monitor-dkg.sh
```

**Check if stuck:**
```bash
./scripts/check-dkg-state.sh
```

## Summary

**"Operators are generating keys" stage:**
- **Expected:** 30-60 minutes with 3 nodes
- **Maximum:** 89 minutes (before timeout)
- **Monitor:** Use `./scripts/monitor-dkg.sh`
- **Status:** DKG state = `2` (AWAITING_RESULT)

The key generation is the longest phase of DKG. With only 3 operators (vs 100 expected), it may take longer due to each operator handling multiple member slots.

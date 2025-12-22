# How to Use the Process DKG with 3 Nodes Guide

## Quick Reference

The `process-dkg-with-3-nodes.md` guide walks you through processing DKG with 3 running nodes. Here's how to use it:

## Step-by-Step Workflow

### 1. **Verify Prerequisites** (Step 1-2)

Use the automated test script:
```bash
./scripts/test-nodes-in-pool.sh
```

This checks:
- ✅ All 3 nodes are running
- ✅ All 3 operators are in sortition pool
- ✅ Pool state (must be IDLE/unlocked)

**Expected result:** All operators should show `true` (in pool)

### 2. **If Operators Not in Pool** (Step 2)

If any operator shows `false`:

**Option A: Wait for auto-join**
- Nodes check every 6 hours automatically
- They join if pool is unlocked and policy allows

**Option B: Manual join**
```bash
# If chaosnet is active, add as beta operators first:
./scripts/add-beta-operators.sh

# Then join to pool:
./scripts/fix-operators-not-in-pool.sh
```

### 3. **Trigger DKG** (Step 3)

Once all operators are in pool:
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

This will:
- Lock the sortition pool
- Request relay entry from Random Beacon
- Start DKG process

### 4. **Monitor Progress** (Step 4-7)

**Quick check:**
```bash
./scripts/monitor-dkg.sh
```

**Or check manually:**
```bash
# Check DKG state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Watch logs
tail -f logs/node*.log | grep -iE "dkg|keygen|member"
```

**Expected states:**
- `0` → `1` → `2` → `0` (IDLE → AWAITING_SEED → AWAITING_RESULT → IDLE)

### 5. **Verify Completion** (Step 8)

```bash
# Should return 0 (IDLE) when complete
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

## Complete Automated Workflow

Use the script that implements the guide:

```bash
./scripts/process-dkg-3-nodes.sh
```

This script:
1. Checks prerequisites
2. Verifies operators are in pool
3. Triggers DKG
4. Shows group selection
5. Provides monitoring instructions

## Common Issues & Solutions

### Issue: Operators Not in Pool

**Check:**
```bash
./scripts/test-nodes-in-pool.sh
```

**Fix:**
- If chaosnet active: `./scripts/add-beta-operators.sh`
- If pool locked: Wait for DKG or `./scripts/stop-dkg.sh`
- If not registered: `./scripts/register-operators.sh`

### Issue: DKG Stuck

**Check:**
```bash
./scripts/check-dkg-state.sh
```

**Fix:**
```bash
# If timed out
./scripts/stop-dkg.sh
```

### Issue: Nodes Not Connected

**Fix:**
```bash
./configs/stop-all-nodes.sh
./scripts/update-peer-ids.sh
./configs/start-all-nodes.sh
```

## Quick Commands Reference

| Task | Command |
|------|---------|
| Test nodes in pool | `./scripts/test-nodes-in-pool.sh` |
| Add beta operators | `./scripts/add-beta-operators.sh` |
| Fix operators not in pool | `./scripts/fix-operators-not-in-pool.sh` |
| Trigger DKG | `KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config configs/config.toml --developer` |
| Monitor DKG | `./scripts/monitor-dkg.sh` |
| Check DKG state | `./scripts/check-dkg-state.sh` |
| Stop DKG | `./scripts/stop-dkg.sh` |

## Expected Timeline

- **Trigger DKG**: Immediate
- **Pool locks**: ~5 seconds
- **Seed arrives**: Depends on Random Beacon
- **DKG execution**: 10-30 minutes (with 3 nodes, may take longer)
- **Result submission**: After DKG completes
- **Challenge period**: ~48 hours (in production), shorter in dev
- **Wallet created**: After challenge period

## Important Notes

1. **DKG requires 100 operators** - With 3 nodes, each operator will be selected ~33 times
2. **Pool must be unlocked** - DKG state must be `0` (IDLE) before operators can join
3. **Chaosnet active?** - Operators must be beta operators if chaosnet is active
4. **Nodes check every 6 hours** - They may join pool automatically

## Next Steps After Reading Guide

1. **Run prerequisite check:**
   ```bash
   ./scripts/test-nodes-in-pool.sh
   ```

2. **If all good, trigger DKG:**
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
     --submit --config configs/config.toml --developer
   ```

3. **Monitor progress:**
   ```bash
   ./scripts/monitor-dkg.sh
   ```

## Summary

The guide provides:
- ✅ Step-by-step instructions
- ✅ Troubleshooting tips
- ✅ Expected timelines
- ✅ Common issues and solutions

**Start here:** `./scripts/test-nodes-in-pool.sh` to verify you're ready!

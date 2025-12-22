# DKG Stuck in AWAITING_RESULT - Solutions

## Problem

DKG state is stuck at `AWAITING_RESULT` (state 2) - "Operators are generating keys..."

## Why This Happens

In local development with a single operator, DKG often gets stuck because:

1. **DKG requires 100 operators** - You only have 1 operator
2. **Operator not selected** - Your operator wasn't selected for this round
3. **Pre-parameters insufficient** - Node doesn't have enough pre-generated parameters
4. **Network issues** - LibP2P connectivity problems

## Solutions

### Solution 1: Wait for Timeout and Reset (Recommended)

DKG timeout is **536 blocks** (~8-9 minutes locally at 1s/block).

**Steps:**

1. **Wait for timeout to pass** (check how long DKG has been running)

2. **Reset DKG:**
   ```bash
   ./scripts/check-and-reset-dkg.sh configs/config.toml
   ```

   Or manually:
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
     --submit --config configs/config.toml --developer
   ```

3. **After reset, request new wallet again:**
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
     --submit --config configs/config.toml --developer
   ```

### Solution 2: Check Node Logs

**Check why operator isn't participating:**

```bash
tail -f <your-log-file> | grep -i "dkg\|eligibility\|pre-parameters"
```

**Look for:**
- `"not eligible for DKG"` → Operator not selected
- `"pre-parameters pool size is too small"` → Need more pre-params
- `"selecting group not possible"` → Sortition pool issue
- `"joining DKG"` → Operator is participating (good!)

### Solution 3: Restart Node (If Pre-Params Issue)

If logs show pre-parameters issue:

```bash
# Stop node
# Then restart
./scripts/start.sh
```

This will regenerate pre-parameters.

### Solution 4: Accept Limitation (Local Dev)

**Important:** In local development with a single operator, **DKG cannot complete** because:

- DKG requires **100 operators** to be selected
- You only have **1 operator**
- Even if selected, DKG protocol needs multiple operators to communicate

**This is expected behavior** for local testing with a single node.

## Understanding the Timeout

The DKG timeout check is:
```
block.number > (startBlock + 536 blocks)
```

- **startBlock**: Block when DKG started (when seed was received)
- **536 blocks**: Result submission timeout
- **Local dev**: ~8-9 minutes at 1s/block
- **Mainnet**: ~2.2 hours at 15s/block

## Quick Diagnostic Commands

```bash
# Check current state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Check timeout status (shows if timeout passed)
./scripts/check-dkg-timeout-status.sh configs/config.toml

# Check if timeout passed (call without submit)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --config configs/config.toml --developer

# Run full diagnostic
./scripts/diagnose-dkg-stuck.sh configs/config.toml

# Check and reset if ready
./scripts/check-and-reset-dkg.sh configs/config.toml
```

## Understanding the Error

If you see this error in logs:
```
execution reverted: DKG has not timed out
```

This means:
- ✅ DKG is in `AWAITING_RESULT` state (correct state for timeout)
- ❌ Not enough blocks have passed since DKG started
- ⏳ You need to wait for **536 blocks** from when DKG started

**Solution:** Wait ~10 minutes (local dev) and try again.

## Expected Behavior

**For Local Development:**
- DKG will likely get stuck because you don't have 100 operators
- This is **normal** and **expected**
- You can reset DKG after timeout and try again
- For full DKG testing, you need multiple nodes running

**For Production:**
- DKG should complete if enough operators participate
- If stuck, check operator logs and network connectivity
- Reset only after timeout has passed

## Related Scripts

- `scripts/diagnose-dkg-stuck.sh` - Diagnose why DKG is stuck
- `scripts/reset-dkg.sh` - Reset DKG (checks timeout first)
- `scripts/check-and-reset-dkg.sh` - Check timeout and reset if ready
- `scripts/monitor-dkg.sh` - Monitor DKG state and progress

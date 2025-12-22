# Fixing "Current state is not IDLE" Error

## Problem

When trying to trigger a new DKG with `request-new-wallet`, you get:
```
Error: got error [execution reverted: Current state is not IDLE]
```

This happens when:
- A DKG is already in progress (state is not IDLE)
- The previous DKG timed out but wasn't notified
- The sortition pool is locked

## Solution

### Step 1: Check Current DKG State

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**State Values:**
- `0` = IDLE (ready for new DKG)
- `1` = AWAITING_SEED (waiting for Random Beacon seed)
- `2` = AWAITING_RESULT (waiting for DKG result submission)
- `3` = CHALLENGE (DKG result is being challenged)

### Step 2: Check if DKG Timed Out

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
```

If this returns `true`, proceed to Step 3.

### Step 3: Notify DKG Timeout

This unlocks the sortition pool and resets the state to IDLE:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit \
  --config configs/config.toml \
  --developer
```

Wait for the transaction to be mined (a few seconds).

### Step 4: Verify State Reset

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

Should now return `0` (IDLE).

### Step 5: Trigger New DKG

Once state is IDLE, you can trigger a new DKG:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml \
  --developer
```

## Alternative: Seed Timeout

If the DKG is stuck in `AWAITING_SEED` state (state `1`), check for seed timeout:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config configs/config.toml --developer
```

If `true`, notify seed timeout:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
  --submit \
  --config configs/config.toml \
  --developer
```

## Quick Reference

```bash
# Check state
./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state --config configs/config.toml --developer

# Check timeout
./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out --config configs/config.toml --developer

# Unlock pool (if timed out)
./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout --submit --config configs/config.toml --developer

# Trigger new DKG (after unlock)
./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config configs/config.toml --developer
```

## Why This Happens

- DKG takes time to complete (off-chain key generation)
- If operators don't submit results in time, DKG times out
- The sortition pool remains locked until timeout is notified
- You cannot start a new DKG while the pool is locked

## Prevention

- Ensure all operators are running and connected
- Monitor DKG progress via logs: `tail -f logs/node*.log | grep -i dkg`
- Check operator connectivity: `curl -s http://localhost:9601/diagnostics | jq '.connected_peers'`
- Ensure sufficient operators are registered and authorized


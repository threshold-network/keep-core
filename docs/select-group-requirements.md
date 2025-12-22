# `select-group` Requirements and Usage

## Error: "Sortition pool unlocked"

This error occurs when trying to call `select-group` while the sortition pool is **not locked**.

## When `select-group` Can Be Called

`select-group` can **only** be called when:
1. **Sortition pool is locked** (during DKG)
2. **DKG state is NOT IDLE** (state 1, 2, or 3)

### DKG States When Pool is Locked

| State | Name | Pool Locked? | Can Call `select-group`? |
|-------|------|--------------|--------------------------|
| `0` | IDLE | ❌ No | ❌ No |
| `1` | AWAITING_SEED | ✅ Yes | ✅ Yes |
| `2` | AWAITING_RESULT | ✅ Yes | ✅ Yes |
| `3` | CHALLENGE | ✅ Yes | ✅ Yes |

## How to Use `select-group`

### Step 1: Trigger DKG (Locks Pool)

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

This locks the sortition pool and starts DKG.

### Step 2: Wait for Pool to Lock

```bash
# Check DKG state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Should return: 1 (AWAITING_SEED) or 2 (AWAITING_RESULT)
```

### Step 3: Call `select-group`

```bash
# Now this will work (pool is locked)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer
```

## Complete Example

```bash
# Step 1: Request new wallet (locks pool)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer

# Step 2: Wait a few seconds for pool to lock
sleep 5

# Step 3: Check state (should be 1 or 2)
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer 2>&1 | tail -1)

echo "DKG State: $STATE"

# Step 4: If state is not 0, select-group will work
if [ "$STATE" != "0" ]; then
  echo "Pool is locked, calling select-group..."
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
    --config configs/config.toml --developer
else
  echo "Pool is unlocked (state IDLE). Cannot call select-group."
fi
```

## Alternative: Check Pool Lock Status

You can check if the pool is locked by checking the DKG state:

```bash
# If state is 0 (IDLE), pool is unlocked
# If state is 1, 2, or 3, pool is locked

STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer 2>&1 | tail -1)

if [ "$STATE" = "0" ]; then
  echo "Pool is unlocked - cannot call select-group"
  echo "Trigger DKG first: request-new-wallet"
else
  echo "Pool is locked - can call select-group"
fi
```

## Why This Restriction?

The sortition pool is locked during DKG to:
- Prevent operators from joining/leaving during selection
- Ensure consistent group selection
- Maintain pool state integrity during DKG

## Workaround

If you want to test `select-group` without triggering a full DKG:

1. **Trigger DKG** (locks pool):
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
     --submit --config configs/config.toml --developer
   ```

2. **Call `select-group`** (now works):
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
     --config configs/config.toml --developer
   ```

3. **Stop DKG** (if you don't want to complete it):
   ```bash
   # Wait for timeout or notify timeout
   ./scripts/stop-dkg.sh
   ```

## Summary

- **Error**: "Sortition pool unlocked" means DKG is not in progress
- **Solution**: Trigger DKG first with `request-new-wallet`
- **Then**: `select-group` will work while DKG is active
- **Check**: Use `get-wallet-creation-state` to verify pool is locked

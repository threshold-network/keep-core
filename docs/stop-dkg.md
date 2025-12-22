# How to Stop DKG / New Wallet Creation

This guide explains how to stop or cancel an ongoing DKG process and prevent new wallet creation.

## Important: DKG Cannot Be Directly Cancelled

**There is no direct "cancel" or "abort" function for DKG.** The DKG process can only be stopped through:
1. **Timeout notification** (if DKG has timed out)
2. **Stopping nodes** (prevents participation but doesn't cancel the on-chain state)
3. **Waiting for completion** (DKG completes successfully or times out)

## Methods to Stop DKG

### Method 1: Notify DKG Timeout (If Timed Out)

If the DKG has timed out, you can notify the timeout to unlock the sortition pool:

```bash
# First, check if DKG has timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# If it returns "true", notify the timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer

# Verify state is reset to IDLE
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
# Should return: 0 (IDLE)
```

**Note:** This only works if the DKG has already timed out (~9 minutes after start).

### Method 2: Notify Seed Timeout (If Stuck in AWAITING_SEED)

If DKG is stuck waiting for seed (state `1`):

```bash
# Check if seed timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config configs/config.toml --developer

# If true, notify seed timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
  --submit --config configs/config.toml --developer
```

### Method 3: Stop Nodes (Prevent Participation)

Stopping nodes prevents them from participating in DKG, but **doesn't cancel the on-chain DKG state**:

```bash
# Stop all nodes
./configs/stop-all-nodes.sh

# Or stop specific nodes
pkill -f "keep-client.*node1"
pkill -f "keep-client.*node2"
```

**Important:** The DKG state on-chain will remain active. Other operators may still complete it, or it will timeout.

### Method 4: Wait for Natural Timeout

Simply wait for the DKG to timeout naturally (~9 minutes), then notify the timeout:

```bash
# Monitor until timeout
./scripts/wait-for-dkg-completion.sh 600  # Wait up to 10 minutes

# Once timed out, notify timeout (see Method 1)
```

## Preventing New Wallet Creation

### Option 1: Don't Call `request-new-wallet`

Simply **don't trigger new wallet requests**:

```bash
# Don't run this command:
# ./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit ...
```

### Option 2: Stop Nodes

If nodes are stopped, they won't participate in new DKG rounds:

```bash
./configs/stop-all-nodes.sh
```

### Option 3: Check State Before Requesting

Always check if DKG is IDLE before requesting a new wallet:

```bash
# Check state first
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer 2>&1 | tail -1)

if [ "$STATE" = "0" ]; then
    echo "State is IDLE, safe to request new wallet"
    # KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
    #   --submit --config configs/config.toml --developer
else
    echo "DKG is in progress (state: $STATE), cannot request new wallet"
fi
```

## Current DKG State

Check the current state:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**State Values:**
- `0` = **IDLE** - No DKG in progress, safe to request new wallet
- `1` = **AWAITING_SEED** - Waiting for Random Beacon seed
- `2` = **AWAITING_RESULT** - DKG is running, waiting for result
- `3` = **CHALLENGE** - DKG result submitted, in challenge period

## Complete Stop Procedure

If you want to completely stop DKG and reset to IDLE:

```bash
# Step 1: Check current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer 2>&1 | tail -1)

echo "Current state: $STATE"

# Step 2: If state is 2 (AWAITING_RESULT), check timeout
if [ "$STATE" = "2" ]; then
    echo "Checking if DKG has timed out..."
    HAS_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
      --config configs/config.toml --developer 2>&1 | tail -1)
    
    if [ "$HAS_TIMED_OUT" = "true" ]; then
        echo "DKG has timed out, notifying timeout..."
        KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
          --submit --config configs/config.toml --developer
        
        sleep 5
        
        # Verify state reset
        NEW_STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
          --config configs/config.toml --developer 2>&1 | tail -1)
        
        if [ "$NEW_STATE" = "0" ]; then
            echo "✓ DKG stopped, state is now IDLE"
        else
            echo "⚠ State is still: $NEW_STATE"
        fi
    else
        echo "DKG has not timed out yet. Wait ~9 minutes or stop nodes to prevent participation."
    fi
fi

# Step 3: If state is 1 (AWAITING_SEED), check seed timeout
if [ "$STATE" = "1" ]; then
    echo "Checking if seed has timed out..."
    HAS_SEED_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
      --config configs/config.toml --developer 2>&1 | tail -1)
    
    if [ "$HAS_SEED_TIMED_OUT" = "true" ]; then
        echo "Seed has timed out, notifying seed timeout..."
        KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
          --submit --config configs/config.toml --developer
    else
        echo "Seed has not timed out yet."
    fi
fi

# Step 4: Stop nodes to prevent participation in future DKG rounds
echo ""
echo "Stopping nodes to prevent participation..."
./configs/stop-all-nodes.sh
```

## Quick Reference

```bash
# Check DKG state
./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Check timeout
./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# Stop DKG (if timed out)
./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer

# Stop nodes
./configs/stop-all-nodes.sh
```

## Limitations

1. **Cannot cancel active DKG** - Once DKG is in progress (state 2), it can only:
   - Complete successfully
   - Timeout (after ~9 minutes)
   - Be challenged (if result is invalid)

2. **Stopping nodes doesn't cancel on-chain state** - Other operators may still complete the DKG

3. **Timeout is the only way to force-stop** - Must wait for timeout period (~9 minutes)

## Summary

- **To stop ongoing DKG**: Wait for timeout, then notify timeout
- **To prevent new DKG**: Don't call `request-new-wallet`, or stop nodes
- **To check status**: Use `get-wallet-creation-state`
- **To force reset**: Wait for timeout, then `notify-dkg-timeout`

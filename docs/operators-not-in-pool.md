# What to Do If Operators Are Not in Sortition Pool

## Quick Diagnosis

First, check if operators are actually in the pool:

```bash
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i ($OPERATOR):"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

If all return `false`, operators are not in the pool.

## Why Operators Might Not Be in Pool

### 1. **Pool is Locked (DKG in Progress)**

If DKG is active, the pool is locked and operators cannot join.

**Check:**
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**If state is NOT `0` (IDLE):**
- Pool is locked
- Wait for DKG to complete or timeout (~89 minutes)
- Or notify timeout if stuck:
  ```bash
  ./scripts/stop-dkg.sh
  ```

### 2. **Chaosnet Active + Not Beta Operator**

If chaosnet is active, only beta operators can join the pool.

**Check chaosnet status:**
```bash
# Note: Requires EcdsaSortitionPool address in config
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool is-chaosnet-active \
  --config configs/config.toml --developer
```

**If chaosnet is active:**
- Operators must be marked as beta operators
- Use Hardhat tasks to add beta operators (see below)

### 3. **Insufficient Authorization**

Operators need sufficient authorization (minimum 40k T tokens).

**Check authorization:**
```bash
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i ($OPERATOR):"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry authorized-stake \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

**If authorization is too low:**
- Top up stake/authorization
- Re-register operators

### 4. **Operator Not Registered**

Operators must be registered in WalletRegistry.

**Check registration:**
```bash
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i ($OPERATOR):"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-registered \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

**If not registered:**
- Register operators using `scripts/register-operators.sh`

### 5. **Nodes Haven't Tried to Join Yet**

Nodes check pool status every **6 hours** by default. They may not have attempted to join yet.

**Check logs for join attempts:**
```bash
for i in {1..3}; do
  echo "Node $i:"
  tail -200 "logs/node$i.log" | grep -iE "sortition.*pool|join.*pool|checking.*pool" | tail -5
done
```

**Look for messages like:**
- `checking sortition pool operator status`
- `operator is not in the sortition pool`
- `joining the sortition pool`
- `holding off with joining the sortition pool due to joining policy`

## Solutions

### Solution 1: Wait for Auto-Join

Nodes automatically join the pool when:
- Pool is unlocked (DKG state = IDLE)
- Chaosnet is not active, OR operator is beta operator
- Operator is registered and authorized

**Nodes check every 6 hours**, so wait or manually trigger (see Solution 2).

### Solution 2: Manually Join Pool

If pool is unlocked and policy allows, manually join:

```bash
# Join each operator using their node config
for i in {1..3}; do
  echo "Joining Node $i operator to pool..."
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry join-sortition-pool \
    --submit --config "configs/node$i.toml" --developer
  sleep 2
done

# Verify they joined
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1)
  echo "Node $i ($OPERATOR): $IN_POOL"
done
```

**Common errors:**
- `execution reverted: Sortition pool locked` → Pool is locked, wait for DKG to complete
- `execution reverted: Not beta operator for chaosnet` → Add operators as beta operators (see Solution 3)
- `execution reverted: Authorization below the minimum` → Top up authorization
- `execution reverted: Unknown operator` → Register operator first

### Solution 3: Add Beta Operators (If Chaosnet Active) ⚠️ CURRENT ISSUE

**Your operators are failing with: "Not beta operator for chaosnet"**

This means chaosnet is active and operators must be beta operators to join the pool.

**Option A: Add Beta Operators via Hardhat Console**

```bash
cd solidity/ecdsa

# Start Hardhat console
npx hardhat console --network development

# In console, get operator addresses and add them
const operators = [
  "0xEf38534ea190856217CBAF454a582BeB74b9e7BF",  // Node 1
  "0x5B4ad7861c4da60c033a30d199E30c47435Fe35A",  // Node 2
  "0x4e2A0254244d5298cfF5ea30c5d4bd21077b372d"   // Node 3
];

// Get sortition pool contract
const EcdsaSortitionPool = await ethers.getContractFactory("EcdsaSortitionPool");
const sortitionPool = await helpers.contracts.getContract("EcdsaSortitionPool");

// Get chaosnet owner
const chaosnetOwner = await sortitionPool.chaosnetOwner();
console.log("Chaosnet owner:", chaosnetOwner);

// Add beta operators (must use chaosnet owner account)
const signer = await ethers.getSigner(chaosnetOwner);
await sortitionPool.connect(signer).addBetaOperators(operators);
```

**Option B: Create Hardhat Task**

Create `solidity/ecdsa/tasks/add_beta_operators.ts`:

```typescript
import { task } from "hardhat/config";
import type { HardhatRuntimeEnvironment } from "hardhat/types";

task("add-beta-operators", "Add operators as beta operators")
  .addParam("operators", "Comma-separated operator addresses")
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    const { ethers, helpers } = hre;
    const sortitionPool = await helpers.contracts.getContract("EcdsaSortitionPool");
    const chaosnetOwner = await sortitionPool.chaosnetOwner();
    
    const operators = taskArgs.operators.split(",").map((addr: string) => addr.trim());
    console.log(`Adding ${operators.length} operators as beta operators...`);
    
    await (
      await sortitionPool
        .connect(await ethers.getSigner(chaosnetOwner))
        .addBetaOperators(operators)
    ).wait();
    
    console.log("Beta operators added successfully!");
  });
```

Then run:
```bash
cd solidity/ecdsa
npx hardhat add_beta_operator:ecdsa \
  --operator "0xEf38534ea190856217CBAF454a582BeB74b9e7BF" \
  --network development
# Repeat for other operators...
```

**Option C: Disable Chaosnet (If Possible)**

If you control the contract deployment, you may be able to deactivate chaosnet:
```bash
# In Hardhat console
const sortitionPool = await helpers.contracts.getContract("EcdsaSortitionPool");
const chaosnetOwner = await sortitionPool.chaosnetOwner();
await sortitionPool.connect(await ethers.getSigner(chaosnetOwner)).deactivateChaosnet();
```

**Note:** Requires `chaosnetOwner` account. Check contract deployment for owner address.

### Solution 4: Unlock Pool (If Locked)

If pool is locked due to stuck DKG:

```bash
# Check if DKG timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# If timed out, notify timeout to unlock pool
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer

# Or use script
./scripts/stop-dkg.sh
```

### Solution 5: Re-register Operators

If operators are not properly registered:

```bash
# Re-register all operators
./scripts/register-operators.sh

# Or register single operator
./scripts/register-single-operator.sh configs/node1.toml
```

## Complete Troubleshooting Script

```bash
#!/bin/bash
# Diagnose and fix operators not in pool

set -eou pipefail

CONFIG="configs/config.toml"

echo "=========================================="
echo "Diagnosing Operators Not in Pool"
echo "=========================================="
echo ""

# Step 1: Check pool status
echo "Step 1: Checking DKG state (pool must be unlocked)..."
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG" --developer 2>&1 | tail -1)

if [ "$STATE" != "0" ]; then
  echo "⚠ Pool is LOCKED (DKG state: $STATE)"
  echo ""
  echo "Options:"
  echo "  1. Wait for DKG to complete (~89 minutes)"
  echo "  2. Notify timeout if stuck: ./scripts/stop-dkg.sh"
  exit 1
else
  echo "✓ Pool is UNLOCKED (DKG state: IDLE)"
fi

echo ""
echo "Step 2: Checking operator pool status..."
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null)
  if [ -n "$OPERATOR" ] && [ "$OPERATOR" != "null" ]; then
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
      "$OPERATOR" --config "$CONFIG" --developer 2>&1 | tail -1)
    echo "  Node $i ($OPERATOR): $IN_POOL"
    
    if [ "$IN_POOL" = "false" ]; then
      echo "    → Not in pool, attempting to join..."
      KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry join-sortition-pool \
        --submit --config "configs/node$i.toml" --developer 2>&1 | tail -3 || echo "    ⚠ Join failed (check error above)"
      sleep 2
    fi
  fi
done

echo ""
echo "Step 3: Verifying final status..."
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null)
  if [ -n "$OPERATOR" ] && [ "$OPERATOR" != "null" ]; then
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
      "$OPERATOR" --config "$CONFIG" --developer 2>&1 | tail -1)
    if [ "$IN_POOL" = "true" ]; then
      echo "  ✓ Node $i: IN POOL"
    else
      echo "  ✗ Node $i: NOT IN POOL"
      echo "    Check logs: tail -50 logs/node$i.log | grep -i pool"
    fi
  fi
done

echo ""
echo "=========================================="
echo "Diagnosis Complete"
echo "=========================================="
```

## Quick Reference

| Issue | Solution |
|-------|----------|
| Pool locked | Wait for DKG or notify timeout |
| Chaosnet active + not beta | Add as beta operators via Hardhat |
| Insufficient authorization | Top up stake/authorization |
| Not registered | Register operators |
| Haven't tried yet | Wait 6 hours or manually join |
| Manual join fails | Check error message and fix root cause |

## Next Steps

Once operators are in the pool:

1. **Verify all are in pool:**
   ```bash
   for i in {1..3}; do
     OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
     KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
       "$OPERATOR" --config configs/config.toml --developer
   done
   ```

2. **Trigger DKG:**
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
     --submit --config configs/config.toml --developer
   ```

3. **Monitor progress:**
   ```bash
   ./scripts/monitor-dkg.sh
   ```

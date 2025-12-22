# How to Rerun DKG Process - Complete Guide

This guide walks you through rerunning the DKG (Distributed Key Generation) process with the new separate staking provider setup.

## Prerequisites

1. **All operators registered and in sortition pools**
   - Each operator must be registered by its staking provider
   - Each operator must be in both RandomBeacon and WalletRegistry sortition pools
   - Need at least 3 operators in pool for DKG to work

2. **Wallet owner set**
   - WalletRegistry must have a wallet owner configured
   - Wallet owner needs ETH for gas fees

3. **Nodes running**
   - All operator nodes should be running and connected
   - Nodes should be monitoring the blockchain for DKG events

## Quick Start (Automated)

Run the complete automated script:

```bash
./scripts/rerun-dkg-complete.sh
```

This script will:
1. Check operator registration status
2. Verify wallet owner is set
3. Check current DKG state
4. Reset DKG if stuck/timed out
5. Request new wallet (triggers DKG)
6. Provide monitoring commands

## Manual Steps

### Step 1: Verify Operators Are Registered

Check if operators are in sortition pools:

```bash
# Check a specific operator
OPERATOR="0xef38534ea190856217cbaf454a582beb74b9e7bf"
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
  --config configs/config.toml --developer

# Check all operators
for i in {1..10}; do
  NODE_CONFIG="configs/node${i}.toml"
  if [ -f "$NODE_CONFIG" ]; then
    KEYFILE=$(grep "^KeyFile" "$NODE_CONFIG" | cut -d'=' -f2 | tr -d ' "')
    OPERATOR=$(basename "$KEYFILE" | sed -E 's/.*--([0-9a-fA-F]{40})$/\1/' | sed 's/^/0x/')
    echo "Node $i ($OPERATOR):"
    KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
      --config configs/config.toml --developer | tail -1
  fi
done
```

If operators are not registered, register them:

```bash
./scripts/register-single-operator.sh <node-number>
```

### Step 2: Check Wallet Owner

```bash
cd solidity/ecdsa
npx hardhat console --network development << 'EOF'
const { helpers } = require("hardhat");
(async () => {
  const wr = await helpers.contracts.getContract("WalletRegistry");
  const owner = await wr.walletOwner();
  console.log("Wallet Owner:", owner);
  process.exit(0);
})();
EOF
cd ../..
```

If wallet owner is not set, initialize it:

```bash
./scripts/initialize-wallet-owner.sh
```

### Step 3: Check Current DKG State

```bash
./scripts/check-dkg-state.sh
```

DKG States:
- **0 (IDLE)**: No DKG in progress - can request new wallet
- **1 (AWAITING_SEED)**: Waiting for Random Beacon relay entry
- **2 (AWAITING_RESULT)**: Operators generating keys (~9 minutes)
- **3 (CHALLENGE)**: Result submitted, in challenge period

### Step 4: Reset DKG (if needed)

If DKG is stuck or timed out:

```bash
# Check if timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config configs/config.toml --developer

# Reset if timed out
./scripts/reset-dkg.sh
```

Or manually reset:

```bash
# For DKG timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer

# For seed timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
  --submit --config configs/config.toml --developer
```

### Step 5: Request New Wallet (Trigger DKG)

**Important**: This must be called by the wallet owner account.

```bash
# Using the wallet owner's config (usually node1)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/node1.toml --developer
```

Or if wallet owner is a different account:

```bash
# First, find wallet owner address
cd solidity/ecdsa
WALLET_OWNER=$(npx hardhat console --network development << 'EOF' | grep -oE "0x[0-9a-fA-F]{40}" | head -1
const { helpers } = require("hardhat");
(async () => {
  const wr = await helpers.contracts.getContract("WalletRegistry");
  const owner = await wr.walletOwner();
  console.log(owner);
  process.exit(0);
})();
EOF
cd ../..

# Then use the config file for that account
# (You may need to create a config file for the wallet owner)
```

### Step 6: Monitor DKG Progress

```bash
# Check DKG state periodically
watch -n 5 './scripts/check-dkg-state.sh'

# Monitor node logs
tail -f logs/node*.log | grep -i dkg

# Check DKG metric
curl http://localhost:9601/metrics | grep performance_dkg_requested_total

# Use monitoring script
./scripts/monitor-dkg.sh
```

## Expected DKG Flow

1. **Request New Wallet** → DKG state becomes `AWAITING_SEED`
2. **Random Beacon generates relay entry** → DKG state becomes `AWAITING_RESULT`
3. **Operators generate keys** (~9 minutes) → DKG state becomes `CHALLENGE`
4. **Result approved** → DKG state becomes `IDLE`, wallet created

## Troubleshooting

### DKG Stuck in AWAITING_SEED

- Check if Random Beacon is working
- Check if operators are in RandomBeacon sortition pool
- Wait for seed timeout, then reset

### DKG Stuck in AWAITING_RESULT

- Check node logs for errors
- Verify operators are running and connected
- Wait for DKG timeout (~9 minutes), then reset

### Operators Not Joining DKG

**Quick Fix**: Run the automated fix script:
```bash
# Fix all nodes
./scripts/fix-operators-not-joining-dkg.sh

# Fix a specific node
./scripts/fix-operators-not-joining-dkg.sh <node-number>
```

This script automatically checks and fixes all common issues. For manual troubleshooting, see below:

#### 1. Verify Operators Are in Sortition Pools

Check if operators are in both RandomBeacon and WalletRegistry sortition pools:

```bash
# Check a specific operator
OPERATOR="0xef38534ea190856217cbaf454a582beb74b9e7bf"
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon is-operator-in-pool "$OPERATOR" \
  --config configs/config.toml --developer

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
  --config configs/config.toml --developer

# Check all operators at once
./scripts/test-nodes-in-pool.sh
```

**Fix**: If operators are not in pools, register them:
```bash
./scripts/register-single-operator.sh <node-number>
```

#### 2. Check Operator Authorization Amounts

Operators need sufficient authorization for both RandomBeacon and WalletRegistry:

```bash
OPERATOR="0xef38534ea190856217cbaf454a582beb74b9e7bf"
STAKING_PROVIDER="0x60C414306e6924a2F2BA51F8bA114a28B3f573E2"  # Get from mapping file

# Check RandomBeacon authorization
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking eligible-stake \
  "$STAKING_PROVIDER" --config configs/config.toml --developer

# Check WalletRegistry authorization  
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry eligible-stake \
  "$STAKING_PROVIDER" --config configs/config.toml --developer
```

**Fix**: If authorization is insufficient, increase it:
```bash
# Authorize RandomBeacon (minimum: 40k T)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
  "$STAKING_PROVIDER" "0x18266866EbBab6cA7f5F2724e22CEF54a98Cda92" "0x878678326eac9000000" \
  --submit --config configs/config.toml --developer

# Authorize WalletRegistry (minimum: 40k T)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
  "$STAKING_PROVIDER" "0xbd49D2e3E501918CD08Eb4cCa34984F428c83464" "0x878678326eac9000000" \
  --submit --config configs/config.toml --developer
```

#### 3. Ensure Nodes Are Running and Connected

Check node status and connectivity:

```bash
# Check if nodes are running
ps aux | grep keep-client

# Check node logs for errors
tail -f logs/node*.log | grep -iE "(error|dkg|pool)"

# Check node metrics
curl http://localhost:9601/metrics | grep -E "(dkg|pool)"
```

**Fix**: If nodes are not running, start them:
```bash
./configs/start-all-nodes.sh
```

#### 4. Check for Beta Operator Requirement (Chaosnet)

If chaosnet is active, operators must be added as beta operators:

```bash
# Check if chaosnet is active
cd solidity/random-beacon
npx hardhat console --network development << 'EOF'
const { helpers } = require("hardhat");
(async () => {
  const pool = await helpers.contracts.getContract("BeaconSortitionPool");
  const isActive = await pool.isChaosnetActive();
  console.log("Chaosnet active:", isActive);
  process.exit(0);
})();
EOF
cd ../..
```

**Fix**: If chaosnet is active, add operators as beta operators:

```bash
# Add a single operator
OPERATOR="0xef38534ea190856217cbaf454a582beb74b9e7bf"
cd solidity/random-beacon
npx hardhat add_beta_operator:beacon --operator "$OPERATOR" --network development
cd ../ecdsa
npx hardhat add_beta_operator:ecdsa --operator "$OPERATOR" --network development
cd ../..

# Or add all operators at once
./scripts/add-beta-operators.sh
```

#### 5. Check Sortition Pool Lock Status

If the sortition pool is locked (DKG in progress), operators cannot join:

```bash
# Check DKG state
./scripts/check-dkg-state.sh

# If DKG is stuck, reset it
./scripts/reset-dkg.sh
```

#### 6. Verify Operator Registration

Ensure operators are properly registered by their staking providers:

```bash
OPERATOR="0xef38534ea190856217cbaf454a582beb74b9e7bf"

# Check RandomBeacon registration
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon operator-to-staking-provider "$OPERATOR" \
  --config configs/config.toml --developer

# Check WalletRegistry registration
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry operator-to-staking-provider "$OPERATOR" \
  --config configs/config.toml --developer
```

**Fix**: If not registered, register the operator:
```bash
./scripts/register-single-operator.sh <node-number>
```

### Transaction Fails: "Not wallet owner"

- Verify wallet owner address
- Use the correct config file (wallet owner's account)
- Check if wallet owner has ETH for gas

## Quick Reference Commands

```bash
# Check DKG state
./scripts/check-dkg-state.sh

# Request new wallet (triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/node1.toml --developer

# Monitor DKG
./scripts/monitor-dkg.sh

# Reset stuck DKG
./scripts/reset-dkg.sh

# Check operators in pool
./scripts/test-nodes-in-pool.sh

# Fix operators not joining DKG (automated)
./scripts/fix-operators-not-joining-dkg.sh

# Complete automated rerun
./scripts/rerun-dkg-complete.sh
```

## With Separate Staking Providers

Remember: With the new setup, each operator has a separate staking provider:

- **Staking Provider**: Owns stake, authorizes applications, registers operator
- **Operator**: Runs node, joins sortition pools, participates in DKG

The wallet owner can be any account (usually operator1), but must have ETH for gas fees.

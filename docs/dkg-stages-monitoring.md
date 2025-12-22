# DKG Stages Monitoring Guide

This guide explains the DKG (Distributed Key Generation) stages and how to monitor them in your local setup, similar to what you see on [tBTC Scan](https://tbtcscan.com/).

## DKG Stages Overview

The DKG process goes through several stages when creating a new wallet:

### Stage 1: `IDLE` (0)
- **Status**: No DKG in progress
- **Meaning**: System is waiting for a wallet creation request
- **What happens**: Nothing active, operators are ready

### Stage 2: `AWAITING_SEED` (1)
- **Status**: Waiting for seed submission
- **Meaning**: A wallet request has been made, waiting for RandomBeacon to generate a relay entry (seed)
- **What happens**: 
  - `requestNewWallet()` was called
  - RandomBeacon is generating a relay entry
  - Once seed is generated, moves to `AWAITING_RESULT`

### Stage 3: `AWAITING_RESULT` (2)
- **Status**: Waiting for DKG result submission
- **Meaning**: Seed is available, operators are executing DKG protocol off-chain
- **What happens**:
  - Selected operators participate in DKG
  - They generate a shared public key
  - One operator submits the result on-chain
  - **This is when `dkg_requested_total` metric increments!**

### Stage 4: `CHALLENGE` (3)
- **Status**: DKG result challenged
- **Meaning**: Someone challenged the submitted DKG result as invalid
- **What happens**:
  - Challenge period begins
  - Other operators validate the result
  - If valid, they approve it; if invalid, it's rejected

## Monitoring DKG Stages Locally

### 1. Check Current DKG State

```bash
# Check DKG state via CLI
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml \
  --developer
```

**Output interpretation:**
- `0` = IDLE
- `1` = AWAITING_SEED  
- `2` = AWAITING_RESULT
- `3` = CHALLENGE

### 2. Monitor DKG Events in Logs

```bash
# Watch for DKG started events (triggers when seed is ready)
tail -f logs/node1.log | grep -i "DKG started\|observed DKG started event"

# Watch for DKG result submissions
tail -f logs/node1.log | grep -i "DKG result\|submitted\|validation"

# Watch for DKG challenges/approvals
tail -f logs/node1.log | grep -i "challenge\|approve\|DKG result"
```

### 3. Check Metrics

```bash
# Check DKG requested metric (new!)
curl -s http://localhost:9601/metrics | grep dkg_requested

# Check all DKG metrics
curl -s http://localhost:9601/metrics | grep performance_dkg
```

**Available DKG metrics:**
- `performance_dkg_requested_total` - Number of DKG requests (increments when stage becomes AWAITING_RESULT)
- `performance_dkg_joined_total` - Number of times node joined DKG
- `performance_dkg_failed_total` - Number of failed DKG attempts
- `performance_dkg_validation_total` - Number of DKG validations performed
- `performance_dkg_challenges_submitted_total` - Number of challenges submitted
- `performance_dkg_approvals_submitted_total` - Number of approvals submitted
- `performance_dkg_duration_seconds` - Duration of DKG operations

### 4. Trigger DKG and Watch Stages Progress

```bash
# Step 1: Request a new wallet (triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml \
  --developer

# Step 2: Check state immediately (should be AWAITING_SEED or AWAITING_RESULT)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml \
  --developer

# Step 3: Watch logs for stage progression
tail -f logs/node*.log | grep -E "DKG started|DKG result|stage|AWAITING"
```

## Understanding "Stages Becoming Valid"

When you see "stages becoming valid" on tBTC Scan or in logs, it means:

1. **Stage transitions are happening**: The DKG process is progressing through stages
2. **Operators are participating**: Selected operators are actively working on DKG
3. **On-chain state is updating**: Each stage transition is confirmed on-chain

### Typical Flow:

```
IDLE → AWAITING_SEED → AWAITING_RESULT → (CHALLENGE if needed) → Wallet Created
```

### What Makes a Stage "Valid":

- **AWAITING_SEED**: Valid when RandomBeacon generates a relay entry
- **AWAITING_RESULT**: Valid when:
  - Seed is confirmed (20 blocks)
  - DKG state is confirmed as `AwaitingResult`
  - **This is when `performance_dkg_requested_total` increments!**
- **CHALLENGE**: Valid when challenge period is active and operators are validating

## Monitoring Your Local Nodes

### Check Operator Status

```bash
# Check if operators are in sortition pool
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i operator: $OPERATOR"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

### Watch Real-Time Stage Progression

```bash
# Monitor all nodes for DKG activity
watch -n 2 'for i in {1..3}; do
  echo "=== Node $i ==="
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r ".client_info.chain_address // \"N/A\"")
  echo "Operator: $OPERATOR"
  curl -s http://localhost:960$i/metrics 2>/dev/null | grep -E "dkg_requested|dkg_joined|dkg_failed" | head -5
  echo ""
done'
```

## Troubleshooting Stage Progression

### Stage Stuck at AWAITING_SEED

**Possible causes:**
- RandomBeacon not generating relay entries
- No operators authorized for RandomBeacon

**Check:**
```bash
# Check RandomBeacon authorization
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking authorized-stake \
  --staking-provider <OPERATOR_ADDRESS> \
  --application RandomBeacon \
  --config configs/config.toml \
  --developer
```

### Stage Stuck at AWAITING_RESULT

**Possible causes:**
- Operators not participating in DKG
- DKG timeout expired
- Network connectivity issues

**Check:**
```bash
# Check if DKG timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml \
  --developer

# Check operator connectivity
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'
```

### Stage in CHALLENGE

**This is normal** - it means someone challenged the result. Operators will validate:
- If valid: They approve → Wallet created
- If invalid: Result rejected → New DKG starts

## Comparing with tBTC Scan

The [tBTC Scan operator page](https://tbtcscan.com/?operator=0xe6c074228932f53c9e50928ad69db760649a8c4d) shows:

1. **Operator Status**: Whether operator is active, staked, authorized
2. **Wallet Participation**: Which wallets the operator is part of
3. **DKG History**: Past DKG participations and results
4. **Performance Metrics**: Success rates, response times

**In your local setup**, you can monitor similar information:

```bash
# Operator status
curl -s http://localhost:9601/diagnostics | jq '.client_info'

# DKG metrics (similar to scan)
curl -s http://localhost:9601/metrics | grep dkg

# Connected peers (network health)
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'
```

## Quick Reference

```bash
# Complete monitoring workflow
# 1. Check current state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# 2. Trigger DKG
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer

# 3. Watch metrics (including new performance_dkg_requested_total)
watch -n 5 'curl -s http://localhost:9601/metrics | grep -E "performance_dkg_requested|performance_dkg_joined|performance_dkg_failed"'

# 4. Monitor logs
tail -f logs/node1.log | grep -i "DKG\|stage\|AWAITING"
```

## Next Steps

Once DKG completes successfully:
- ✅ Wallet is created and registered
- ✅ Operators can participate in signing operations
- ✅ System is ready for deposits/redemptions

Your `performance_dkg_requested_total` metric will increment each time a DKG request reaches the `AWAITING_RESULT` stage, giving you visibility into wallet creation activity!

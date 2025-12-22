# Testing DKG (Distributed Key Generation) Locally

This guide explains how to trigger and test DKG rounds on a local keep-client node.

## Overview

DKG (Distributed Key Generation) is the process where multiple operators collaborate to generate a shared cryptographic key for a new wallet. The process is triggered when a new wallet is requested.

## Prerequisites

1. **Local Ethereum Node**: Geth running on developer network
2. **Contracts Deployed**: RandomBeacon, WalletRegistry, TokenStaking contracts
3. **Keep-Client Running**: At least one node running with proper configuration
4. **Operator Registered**: Your operator must be registered and authorized in WalletRegistry

## Quick Start

### Step 1: Ensure Your Node is Running

```bash
# Start your keep-client node
./scripts/start.sh

# In another terminal, verify it's running
curl http://localhost:9601/metrics
```

### Step 2: Request a New Wallet (Triggers DKG)

```bash
# Request a new wallet - this triggers the DKG process
./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml
```

This command will:
1. Lock the sortition pool
2. Request a relay entry from Random Beacon
3. Once relay entry is generated, start DKG process
4. Your node will automatically participate if eligible

### Step 3: Monitor DKG Progress

**Watch node logs:**
```bash
# Look for DKG-related log messages
tail -f <log-file> | grep -i "dkg\|wallet"
```

**Check metrics:**
```bash
# Monitor DKG performance metrics
watch -n 2 'curl -s http://localhost:9601/metrics | grep performance_dkg'
```

**Check diagnostics:**
```bash
curl -s http://localhost:9601/diagnostics | jq
```

## Using the Test Script

A convenience script is provided to automate the testing process:

```bash
./scripts/test-dkg.sh [config-file]
```

Example:
```bash
./scripts/test-dkg.sh configs/config.toml
```

The script will:
1. Check if your node is running
2. Get wallet owner address
3. Request a new wallet (with confirmation prompt)
4. Provide monitoring instructions

## Manual DKG Testing Commands

### Check Wallet Owner
```bash
./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml
```

### Check Wallet Creation State
```bash
./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml
```

### Check if Operator is in Pool
```bash
# Get your operator address first from diagnostics
OPERATOR_ADDR=$(curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address')

./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
  --operator $OPERATOR_ADDR \
  --config configs/config.toml
```

### Check DKG Parameters
```bash
./keep-client ethereum ecdsa wallet-registry dkg-parameters \
  --config configs/config.toml
```

### Query Existing Wallets
```bash
# Get wallet by public key hash
./keep-client ethereum ecdsa wallet-registry get-wallet \
  --wallet-public-key-hash <hash> \
  --config configs/config.toml
```

## Understanding the DKG Flow

1. **Request New Wallet**: `requestNewWallet()` is called
   - Locks the sortition pool
   - Requests relay entry from Random Beacon

2. **Relay Entry Generated**: Random Beacon generates entry
   - Triggers `__beaconCallback()` in WalletRegistry
   - Starts DKG process with relay entry as seed

3. **Group Selection**: Operators are selected
   - Based on relay entry seed
   - Selected operators form the DKG group

4. **DKG Execution**: Off-chain protocol
   - Selected operators perform DKG
   - Generate shared public key
   - Create threshold signature shares

5. **Result Submission**: DKG result submitted
   - One operator submits result to chain
   - Challenge period begins

6. **Challenge Period**: Others can challenge
   - If result is invalid, challenger gets reward
   - If valid, wallet is created after challenge period

## Monitoring DKG Metrics

The client exposes DKG-related metrics:

```bash
# Get all DKG metrics
curl -s http://localhost:9601/metrics | grep performance_dkg

# Available metrics:
# - performance_dkg_joined_total
# - performance_dkg_failed_total
# - performance_dkg_validation_total
# - performance_dkg_challenges_submitted_total
# - performance_dkg_approvals_submitted_total
# - performance_dkg_duration_seconds
```

## Troubleshooting

### Node Not Participating in DKG

1. **Check operator registration:**
   ```bash
   ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
     --operator <your-operator-address> \
     --config configs/config.toml
   ```

2. **Check authorization:**
   ```bash
   ./keep-client ethereum threshold token-staking authorized-stake \
     --staking-provider <your-staking-provider> \
     --application <wallet-registry-address> \
     --config configs/config.toml
   ```

3. **Check node logs** for eligibility messages

### DKG Failing

- Check that enough operators are registered and authorized
- Verify network connectivity between operators
- Check that Random Beacon is generating relay entries
- Review node logs for specific error messages

### Transaction Fails

- Ensure wallet owner address has sufficient ETH
- Check that DKG is not already in progress
- Verify contracts are properly deployed

## Testing with Multiple Nodes

For a complete DKG test, you need multiple nodes:

1. **Start multiple nodes** with different configs
2. **Register each operator** in TokenStaking
3. **Authorize each operator** for WalletRegistry
4. **Request new wallet** - all eligible operators will participate
5. **Monitor all nodes** to see DKG progress

## Example Complete Test Flow

```bash
# Terminal 1: Start node 1
./scripts/start.sh  # Select config1.toml

# Terminal 2: Start node 2  
./scripts/start.sh  # Select config2.toml

# Terminal 3: Request new wallet (triggers DKG)
./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml

# Terminal 4: Monitor metrics from all nodes
watch -n 2 'curl -s http://localhost:9601/metrics | grep dkg'
watch -n 2 'curl -s http://localhost:9602/metrics | grep dkg'
```

## Additional Resources

- [Local T Network Setup Guide](./development/local-t-network.adoc)
- [Running Keep Node](./run-keep-node.adoc)
- [WalletRegistry Contract Documentation](../solidity/ecdsa/README.adoc)

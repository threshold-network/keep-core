# Quick Start: Running T Network Locally

This guide will help you get the T network running locally for development.

## Prerequisites

- Geth Ethereum client installed
- Node.js (check Hardhat compatibility)
- Yarn package manager
- At least 11 Ethereum accounts created
- `keep-client` binary built (or use `./scripts/build.sh`)

## Step-by-Step Guide

### 1. Start Geth Ethereum Node

**In a separate terminal**, start Geth with mining enabled:

```bash
export GETH_DATA_DIR=~/ethereum/data
export GETH_ETHEREUM_ACCOUNT=$(geth account list --keystore ~/ethereum/data/keystore | head -1 | grep -o '{[^}]*}' | sed 's/{//;s/}//')

geth --port 3000 --networkid 1101 --identity 'local-dev' \
     --ws --ws.addr '127.0.0.1' --ws.port '8546' --ws.origins '*' \
     --ws.api 'admin, debug, web3, eth, txpool, personal, ethash, miner, net' \
     --http --http.port '8545' --http.addr '127.0.0.1' --http.corsdomain '' \
     --http.api 'admin, debug, web3, eth, txpool, personal, ethash, miner, net' \
     --datadir=$GETH_DATA_DIR --allow-insecure-unlock \
     --miner.etherbase=$GETH_ETHEREUM_ACCOUNT --mine --miner.threads=1
```

**Keep this terminal running!** Geth needs to be running for the network to work.

**Verify Geth is running:**
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
```

### 2. Deploy Contracts (if not already deployed)

Check deployment status:
```bash
cd /Users/levakhnazarov/threshold/fork2/keep-core
./scripts/check-deployments.sh
```

If contracts are missing, deploy them:
```bash
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/install.sh --network development
```

Or deploy just ECDSA and TBTC (if Threshold and Random Beacon are already deployed):
```bash
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/deploy-ecdsa-tbtc.sh
```

### 3. Update Configuration

Update `configs/config.toml` with deployed contract addresses. Your config already has:
- `TokenStakingAddress = "0x915887f9d484799CB10bEEd222E93dF25B989632"`
- `RandomBeaconAddress = "0xbeC27f14fc01983895617500e52Adb539b2E6feD"`
- `WalletRegistryAddress = "0x902C75DD90081c74121ad34d83E885E61E95cFe1"`

If TBTC contracts are deployed, uncomment and update:
```toml
BridgeAddress = "0x..."  # From tmp/tbtc-v2/solidity/deployments/development/Bridge.json
```

**Get contract addresses manually:**
```bash
# TokenStaking
cat tmp/solidity-contracts/deployments/development/TokenStaking.json | grep -o '"address": "[^"]*"' | cut -d'"' -f4

# RandomBeacon
cat solidity/random-beacon/deployments/development/RandomBeacon.json | grep -o '"address": "[^"]*"' | cut -d'"' -f4

# WalletRegistry (proxy address)
cat solidity/ecdsa/deployments/development/WalletRegistry.json | grep -o '"address": "[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "Check .openzeppelin/development.json for proxy address"

# Bridge (TBTC)
cat tmp/tbtc-v2/solidity/deployments/development/Bridge.json | grep -o '"address": "[^"]*"' | cut -d'"' -f4
```

### 4. Build Client (if not already built)

Build the `keep-client` binary:
```bash
./scripts/build.sh
```

Or build manually:
```bash
go build -o keep-client ./cmd
```

### 5. Initialize Client

Initialize your client (mint tokens, stake, register operator):

```bash
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/initialize.sh --network development
```

This will:
- Mint and approve T tokens
- Stake T tokens
- Increase authorization for RandomBeacon and WalletRegistry
- Register operator for RandomBeacon and WalletRegistry

**Note:** Select your config file (`config.toml`) when prompted.

### 6. Start Client

Start the keep-core client:

```bash
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/start.sh
```

Or run directly:
```bash
export KEEP_ETHEREUM_PASSWORD=password
./keep-client --config configs/config.toml start --developer
```

**Note:** Select your config file when prompted by the script.

## Complete Workflow Summary

```bash
# Terminal 1: Start Geth (keep running)
export GETH_DATA_DIR=~/ethereum/data
export GETH_ETHEREUM_ACCOUNT=$(geth account list --keystore ~/ethereum/data/keystore | head -1 | grep -o '{[^}]*}' | sed 's/{//;s/}//')
geth --port 3000 --networkid 1101 --identity 'local-dev' --ws --ws.addr '127.0.0.1' --ws.port '8546' --ws.origins '*' --ws.api 'admin, debug, web3, eth, txpool, personal, ethash, miner, net' --http --http.port '8545' --http.addr '127.0.0.1' --http.corsdomain '' --http.api 'admin, debug, web3, eth, txpool, personal, ethash, miner, net' --datadir=$GETH_DATA_DIR --allow-insecure-unlock --miner.etherbase=$GETH_ETHEREUM_ACCOUNT --mine --miner.threads=1

# Terminal 2: Deploy contracts (one-time setup)
cd /Users/levakhnazarov/threshold/fork2/keep-core
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/install.sh --network development

# Terminal 2: Initialize client (one-time per account)
./scripts/initialize.sh --network development

# Terminal 2: Start client
./scripts/start.sh
```

## Quick Reference

**Check deployment status:**
```bash
./scripts/check-deployments.sh
```

**Verify Geth is running:**
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
```

**Check account balance:**
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0xYOUR_ADDRESS","latest"],"id":1}' \
  http://localhost:8545
```

## Troubleshooting

**Error: "no contract code at given address"**
- Contracts not deployed yet - run deployment scripts
- Wrong addresses in config - update with correct addresses
- Geth not mining - ensure Geth is running with `--mine` flag

**Error: "insufficient funds"**
- Geth not mining - check Geth is running with `--mine` flag
- Accounts not funded - reinitialize chain with genesis.json
- Wait for blocks to be mined after deployment

**Error: "could not create TBTC chain handle"**
- TBTC contracts not deployed - deploy TBTC contracts
- Bridge address missing in config - add BridgeAddress to config
- Can be ignored if you're not using TBTC functionality

**Error: "missing value for bitcoin.electrum.url"**
- Already configured in your config file - should not occur

**Error: "could not decrypt Ethereum key"**
- Wrong password - ensure `KEEP_ETHEREUM_PASSWORD=password`
- Wrong key file path - check `ethereum.KeyFile` in config

**Client won't start:**
- Ensure Geth is running and mining
- Check all contract addresses in config are correct
- Verify `storage.Dir` path exists and is writable
- Check logs for specific error messages

For more details, see `docs/development/local-t-network.adoc`


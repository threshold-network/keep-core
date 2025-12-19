# Local T Network Development Guide

This guide explains how to use your local T network setup for development and testing.

## Quick Start

### 1. Start Geth (Terminal 1)
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

### 2. Start Keep Client (Terminal 2)
```bash
cd /Users/levakhnazarov/threshold/fork2/keep-core
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/start.sh
# Select: 1 (config.toml), 1 (info log level)
```

## Development Workflows

### Testing Contract Interactions

#### Check Contract Addresses
```bash
./scripts/check-deployments.sh
```

#### Query Contract State
```bash
# Check TokenStaking
cd tmp/solidity-contracts
export GETH_DATA_DIR=~/ethereum/data
npx hardhat --network development run - <<'EOF'
const { ethers } = require("hardhat");
const deployments = require("hardhat-deploy");

async function main() {
  const TokenStaking = await deployments.get("TokenStaking");
  const staking = await ethers.getContractAt("TokenStaking", TokenStaking.address);
  
  const provider = "0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53";
  const stake = await staking.stake(provider);
  console.log("Stake:", ethers.utils.formatEther(stake), "T");
}

main().catch(console.error);
EOF
```

#### Check RandomBeacon Status
```bash
cd solidity/random-beacon
export GETH_DATA_DIR=~/ethereum/data
npx hardhat --network development run - <<'EOF'
const { ethers } = require("hardhat");
const deployments = require("hardhat-deploy");

async function main() {
  const RandomBeacon = await deployments.get("RandomBeacon");
  const beacon = await ethers.getContractAt("RandomBeacon", RandomBeacon.address);
  
  const minAuth = await beacon.minimumAuthorization();
  console.log("Minimum Authorization:", ethers.utils.formatEther(minAuth), "T");
}

main().catch(console.error);
EOF
```

### Testing Staking Operations

#### Stake More Tokens
```bash
cd solidity/random-beacon
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
npx hardhat stake \
  --network development \
  --owner 0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53 \
  --provider 0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53 \
  --amount 50000
```

#### Authorize Stake
```bash
cd solidity/random-beacon
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
npx hardhat authorize \
  --network development \
  --owner 0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53 \
  --provider 0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53 \
  --application RandomBeacon \
  --amount 50000
```

### Monitoring and Debugging

#### Check Client Logs
The client logs to stdout. Key log levels:
- `INFO` - Normal operations
- `WARN` - Non-critical issues (e.g., bootstrap peer connection failures)
- `ERROR` - Errors that don't stop the client
- `FATAL` - Critical errors that stop the client

#### Monitor Ethereum Node
```bash
# Check block height
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545

# Check account balance
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53","latest"],"id":1}' \
  http://localhost:8545

# Get latest block
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}' \
  http://localhost:8545
```

#### Check Client Info Endpoint
The client exposes metrics on port 9601:
```bash
curl http://localhost:9601/metrics
curl http://localhost:9601/health
```

### Common Development Tasks

#### Reset Everything (Fresh Start)
```bash
# Stop Geth and client
# Then:

# 1. Reset Geth
rm -rf ~/ethereum/data/geth
export GETH_DATA_DIR=~/ethereum/data
./scripts/generate-genesis.sh
geth --datadir=$GETH_DATA_DIR init ~/ethereum/data/genesis.json

# 2. Redeploy Contracts
cd /Users/levakhnazarov/threshold/fork2/keep-core
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/install.sh --network development

# 3. Reinitialize Client
./scripts/initialize.sh --network development
```

#### Add More Test Accounts
```bash
# Generate new account
geth account new --keystore ~/ethereum/data/keystore

# Fund it in genesis.json (before init) or transfer from existing account
```

#### Test Contract Upgrades
```bash
# Example: Upgrade RandomBeacon
cd solidity/random-beacon
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
npx hardhat deploy --network development --tags RandomBeacon --reset
```

### Testing Specific Features

#### Test RandomBeacon
```bash
# Request a relay entry (if you have a requester authorized)
cd solidity/random-beacon
export GETH_DATA_DIR=~/ethereum/data
npx hardhat request-relay-entry \
  --network development \
  --requester 0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53
```

#### Test ECDSA Wallet Creation
```bash
# The client will automatically participate in DKG when conditions are met
# Monitor logs for DKG participation
```

### Troubleshooting

#### Client Won't Start
1. **Check Geth is running**: `curl http://localhost:8545`
2. **Check contract addresses**: `./scripts/check-deployments.sh`
3. **Verify account balance**: Ensure account has ETH for gas
4. **Check logs**: Look for FATAL errors

#### Contracts Not Found
```bash
# Redeploy specific contract
cd solidity/random-beacon  # or solidity/ecdsa
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
npx hardhat deploy --network development --tags <ContractName>
```

#### Out of Gas Errors
```bash
# Increase gas limit in hardhat.config.ts or use --gas-limit flag
# Or send more ETH to the account
```

#### Connection Issues
- **Geth not mining**: Check `--mine` flag is set
- **WebSocket errors**: Verify `--ws.port 8546` matches config
- **RPC errors**: Check `--http.port 8545` matches config

### Useful Commands Reference

```bash
# Check all deployed contracts
./scripts/check-deployments.sh

# View contract ABI
cat solidity/random-beacon/deployments/development/RandomBeacon.json | jq .abi

# Get contract bytecode
cat solidity/random-beacon/deployments/development/RandomBeacon.json | jq .bytecode

# Check transaction status
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt","params":["<tx_hash>"],"id":1}' \
  http://localhost:8545

# Unlock account in Geth console
geth attach ~/ethereum/data/geth.ipc
> personal.unlockAccount(eth.accounts[0], "password", 0)
```

### Development Tips

1. **Use Debug Log Level**: Set log level to `debug` for more verbose output
2. **Monitor Network**: Watch for peer connections and network activity
3. **Test Incrementally**: Test one feature at a time
4. **Keep Geth Mining**: Ensure blocks are being mined for transactions to confirm
5. **Save Transaction Hashes**: Keep track of important transactions for debugging

### Next Steps

- **Add More Operators**: Deploy additional operator nodes
- **Test Group Formation**: Trigger DKG and group selection
- **Test Signing**: Test threshold signing operations
- **Monitor Metrics**: Use the client info endpoint for monitoring
- **Customize Config**: Modify `configs/config.toml` for your needs

## Configuration Files

- **Client Config**: `configs/config.toml`
- **Geth Genesis**: `~/ethereum/data/genesis.json`
- **Contract Deployments**: `solidity/*/deployments/development/`

## Support

For issues:
1. Check logs for error messages
2. Verify all contracts are deployed: `./scripts/check-deployments.sh`
3. Ensure Geth is running and mining
4. Check account balances and gas availability

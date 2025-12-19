# T Network Local Setup - Complete! ✅

## Summary

Your local T network setup is now complete and ready to run! Here's what was accomplished:

### ✅ Completed Steps

1. **TokenStaking Redeployed** - Deployed as `ExtendedTokenStaking` (with `stake()` function) at `0x6d19C0b4bd2B49eCa000C2Fd910c2Db9607f34ee`
2. **RandomBeacon Redeployed** - Deployed at `0x18266866EbBab6cA7f5F2724e22CEF54a98Cda92` (with new TokenStaking address)
3. **RandomBeaconChaosnet Deployed** - Deployed at `0x72472aa2135E5F622Da549062698fF9c80d72282`
4. **RandomBeaconGovernance Deployed** - Deployed at `0x1711AD3b4e4315F67B4ec3c12cfaCEAF5777c47c`
5. **ECDSA Contracts Deployed** - WalletRegistry at `0xbB1Da788F9771318B2C3A72A557ba4cA0356208c`
6. **Applications Approved** - RandomBeacon and WalletRegistry approved in TokenStaking
7. **Client Initialized** - Tokens staked, operators registered, authorizations set

### 📋 Current Configuration

Your `configs/config.toml` is configured with:
- `TokenStakingAddress = "0x6d19C0b4bd2B49eCa000C2Fd910c2Db9607f34ee"`
- `RandomBeaconAddress = "0x18266866EbBab6cA7f5F2724e22CEF54a98Cda92"`
- `WalletRegistryAddress = "0xbB1Da788F9771318B2C3A72A557ba4cA0356208c"`

### 🚀 How to Run

**1. Start Geth (in a separate terminal):**
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

**2. Start the Keep Client:**
```bash
cd /Users/levakhnazarov/threshold/fork2/keep-core
export GETH_DATA_DIR=~/ethereum/data
export KEEP_ETHEREUM_PASSWORD=password
./scripts/start.sh
```

Select `1` for config file and `1` for log level when prompted.

### 📝 Important Notes

- **Geth must be running** before starting the client
- The client connects to Geth via WebSocket at `ws://127.0.0.1:8546`
- Your account `0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53` is:
  - Staked with 1,000,000 T
  - Authorized for RandomBeacon (40,000 T)
  - Authorized for WalletRegistry (40,000 T)
  - Registered as operator for both applications

### 🔧 Troubleshooting

If you encounter issues:

1. **Verify Geth is running and mining:**
   ```bash
   curl -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
     http://localhost:8545
   ```

2. **Check contract deployments:**
   ```bash
   ./scripts/check-deployments.sh
   ```

3. **Verify account balance:**
   ```bash
   curl -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53","latest"],"id":1}' \
     http://localhost:8545
   ```

### 🎉 You're Ready!

Your T network is fully set up and ready to use for local development!

# Quick Fix: Testing DKG Locally

## Common Issues and Solutions

### Issue 1: "Caller is not the Wallet Owner"

**Problem:** You're getting the error: **"Caller is not the Wallet Owner"** when trying to request a new wallet.

**Cause:**
- The WalletRegistry has a wallet owner set (not zero address)
- Your operator address doesn't match the wallet owner
- `requestNewWallet()` can only be called by the wallet owner

**Solution:** Update `configs/config.toml` to use the correct deployed contract address:
```toml
[developer]
WalletRegistryAddress = "0xbd49D2e3E501918CD08Eb4cCa34984F428c83464"  # Use deployed address
```

### Issue 2: "Current state is not IDLE"

**Problem:** You're getting the error: **"Current state is not IDLE"** when trying to request a new wallet.

**Cause:**
- A DKG round is already in progress
- The previous DKG hasn't completed yet

**Solution:** Wait for the current DKG to complete, or check its status:
```bash
# Check current DKG state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml \
  --developer

# Monitor DKG progress
./scripts/monitor-dkg.sh configs/config.toml
```

**DKG States:**
- `0` = IDLE (ready for new wallet request)
- `1` = AWAITING_SEED (waiting for Random Beacon seed)
- `2` = AWAITING_RESULT (operators generating keys)
- `3` = CHALLENGE (result submitted, in challenge period)

**How Long Does DKG Take?**

DKG completion depends on several factors:

1. **Result Submission Window**: 536 blocks (~9 minutes locally, ~2.2 hours mainnet)
   - Operators must submit the DKG result within this window

2. **Challenge Period**: 11,520 blocks (~192 minutes locally, ~48 hours mainnet)
   - After result submission, there's a challenge period
   - Anyone can challenge the result if it's invalid
   - After challenge period, result can be approved

3. **Total Time (Happy Path)**:
   - **Local Development**: ~3-4 hours (mostly challenge period)
   - **Mainnet**: ~48 hours (mostly challenge period)

**Check DKG Timing:**
```bash
./scripts/check-dkg-timing.sh configs/config.toml
```

**Monitor Progress:**
```bash
# Watch state changes
watch -n 5 './scripts/monitor-dkg.sh configs/config.toml'

# Check metrics
curl -s http://localhost:9601/metrics | grep performance_dkg
```

## Solutions

### Solution 1: Use Wallet Owner's Keyfile (Quickest)

If you know the wallet owner address, update your config to use its keyfile:

1. **Find the wallet owner address:**
   ```bash
   # Check what address is currently the wallet owner
   # (You may need to check deployment logs or use Hardhat console)
   ```

2. **Update config.toml:**
   ```toml
   [ethereum]
   KeyFile = "/path/to/wallet-owner-keyfile"
   ```

3. **Then request new wallet:**
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
     --submit \
     --config configs/config.toml \
     --developer
   ```

### Solution 2: Update Wallet Owner to Your Operator Address

**Note:** This requires governance access and may have a delay period.

1. **Begin wallet owner update:**
   ```bash
   cd solidity/ecdsa
   npx hardhat --network development begin-wallet-owner-update \
     --new-wallet-owner 0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53
   ```

2. **Wait for governance delay** (if configured)

3. **Finalize the update:**
   ```bash
   npx hardhat --network development finalize-wallet-owner-update
   ```

### Solution 3: For Local Development - Direct Update (If You Have Owner Access)

If you have direct owner access to WalletRegistryGovernance:

```bash
# This requires the transaction to be sent from the governance owner
cd solidity/ecdsa
npx hardhat --network development update-wallet-owner \
  --wallet-owner 0x7966C178f466B060aAeb2B91e9149A5FB2Ec9c53
```

**Note:** The `update-wallet-owner` command may not exist - you might need to use WalletRegistryGovernance contract directly.

### Solution 4: Check Current Setup

To understand your current setup:

```bash
# Check wallet owner via keep-client
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml \
  --developer

# Check your operator address
curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address'
```

## Recommended Approach for Local Testing

For the simplest local testing setup:

1. **Use your operator address as wallet owner** during initial deployment
2. **Or** update the wallet owner to match your operator address before testing
3. **Then** you can request new wallets using your operator's keyfile

## Complete DKG Test Command (Once Wallet Owner is Set)

```bash
# 1. Request new wallet (triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml \
  --developer

# 2. Monitor DKG progress
watch -n 2 'curl -s http://localhost:9601/metrics | grep performance_dkg'

# 3. Check wallet creation state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml \
  --developer
```

## Troubleshooting

- **"Wallet Owner already initialized"**: Wallet owner was set during deployment. Use update path or use that address's keyfile.
- **"Caller is not the Wallet Owner"**: Your keyfile doesn't match the wallet owner address. Update config or wallet owner.
- **Transaction fails**: Check ETH balance, gas settings, and that contracts are properly deployed.

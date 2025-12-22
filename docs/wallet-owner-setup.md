# Wallet Owner Setup

## Error: "Caller is not the Wallet Owner"

If you see this error when trying to request a new wallet:

```
Error: got error [execution reverted: Caller is not the Wallet Owner]
```

This means the **Wallet Owner** has not been initialized in the WalletRegistry contract.

## What is Wallet Owner?

The **Wallet Owner** is the address authorized to:
- Request new wallet creation (trigger DKG)
- Close existing wallets
- Slash misbehaving operators

In production, this is typically the Bridge contract. For local development, you can use any account (e.g., one of your operator accounts).

## Solution: Initialize Wallet Owner

### Option 1: Use Automated Script (Recommended)

```bash
./scripts/initialize-wallet-owner.sh
```

This script:
1. Extracts operator1 address from `configs/node1.toml`
2. Unlocks Ethereum accounts
3. Initializes Wallet Owner using the governance account

### Option 2: Manual Initialization

```bash
cd solidity/ecdsa

# Get operator1 address
OPERATOR1_ADDRESS="0xEf38534ea190856217CBAF454a582BeB74b9e7BF"

# Unlock accounts
KEEP_ETHEREUM_PASSWORD=password npx hardhat unlock-accounts --network development

# Initialize wallet owner
npx hardhat initialize-wallet-owner \
  --wallet-owner-address "$OPERATOR1_ADDRESS" \
  --network development
```

**Note:** The `initialize-wallet-owner` task uses the `governance` account (account index 2) to initialize the wallet owner.

## Verify Wallet Owner

After initialization, check the wallet owner:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml --developer
```

**Expected:** Should return the address you set (e.g., operator1 address)

## Use Wallet Owner Account

After initialization, use the **same account** that was set as Wallet Owner to request new wallets:

```bash
# Make sure config.toml uses the Wallet Owner account's keyfile
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

## Important Notes

1. **Wallet Owner can only be initialized once** - If already initialized, you'll get an error. Use `beginWalletOwnerUpdate`/`finalizeWalletOwnerUpdate` to change it.

2. **Governance account required** - Only the governance account (account index 2 in development) can initialize the wallet owner.

3. **Use same account** - The account used to request new wallets must be the Wallet Owner address.

## Troubleshooting

### Error: "Wallet Owner already initialized"

If Wallet Owner is already set but you want to change it:

```bash
cd solidity/ecdsa

# Start Hardhat console
npx hardhat console --network development

# In console:
const { helpers, ethers } = require("hardhat");
const governance = await helpers.contracts.getContract("WalletRegistryGovernance");
const signer = await ethers.getSigner(2); // governance account

// Begin update
await governance.connect(signer).beginWalletOwnerUpdate("0xNEW_ADDRESS");

// Wait for governance delay (check current delay)
const delay = await governance.governanceDelay();
console.log("Governance delay:", delay.toString());

// After delay, finalize
await governance.connect(signer).finalizeWalletOwnerUpdate();
```

### Error: "Governance account not unlocked"

Make sure to unlock accounts first:

```bash
cd solidity/ecdsa
KEEP_ETHEREUM_PASSWORD=password npx hardhat unlock-accounts --network development
```

### Check Current Wallet Owner

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml --developer
```

If it returns `0x0000000000000000000000000000000000000000`, Wallet Owner is not initialized.

## If Wallet Owner Already Initialized (Different Account)

If Wallet Owner is already set to a different account than the one in your config:

**Option 1: Update config to use Wallet Owner's account (Recommended)**

This is the **simplest solution** - just update `config.toml` to use the Wallet Owner's keyfile:

```bash
# Check current Wallet Owner
WALLET_OWNER=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml --developer 2>&1 | tail -1)

echo "Wallet Owner: $WALLET_OWNER"

# Find the keyfile (check original config path or search)
# Then update config.toml KeyFile to point to that keyfile
```

**Option 2: Update Wallet Owner to match your config**

This requires waiting for governance delay (7 days by default):

```bash
./scripts/update-wallet-owner-to-operator1.sh
```

This will:
1. Begin wallet owner update to operator1's address
2. Try to finalize immediately (if governance delay allows)
3. If delay required (7 days), provide instructions to finalize later

**Note:** Reducing governance delay also requires waiting for the delay, so Option 1 is usually faster.

## Summary

**If Wallet Owner not initialized:**
```bash
./scripts/initialize-wallet-owner.sh
```

**If Wallet Owner is different account:**
```bash
./scripts/update-wallet-owner-to-operator1.sh
```

**Then request new wallet:**
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

The Wallet Owner must match the account in your config **before** you can request new wallets!

# Setting Up Wallet Owner for Local Development

The WalletRegistry contract requires a wallet owner to be set before you can request new wallets (which triggers DKG). If the wallet owner is `0x0000000000000000000000000000000000000000`, you need to initialize it first.

## Check Current Wallet Owner

```bash
./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml
```

If this returns `0x0000000000000000000000000000000000000000`, the wallet owner needs to be initialized.

## Setting Wallet Owner

### Option 1: Using WalletRegistryGovernance (Recommended for Local Dev)

If you're using Hardhat for local development:

```bash
cd solidity/ecdsa
npx hardhat initialize-wallet-owner --wallet-owner <address> --network development
```

Replace `<address>` with the Ethereum address you want to use as the wallet owner (e.g., your operator address or a test account).

### Option 2: Direct Update (If You Have Owner Access)

If you have direct owner access to the WalletRegistry contract:

```bash
./keep-client ethereum ecdsa wallet-registry update-wallet-owner \
  <wallet-owner-address> \
  --submit \
  --config configs/config.toml
```

**Note:** This requires the transaction to be sent from the contract owner address.

### Option 3: During Contract Deployment

The wallet owner should ideally be set during contract deployment. Check your deployment scripts in `solidity/ecdsa/deploy/` directory.

## For Local Development Setup

If you're setting up a local T network:

1. **Use your operator address as wallet owner** - This is the simplest for testing
2. **Or use a dedicated test account** - Create a separate account for wallet operations

### Example: Set Operator Address as Wallet Owner

```bash
# Get your operator address from diagnostics
OPERATOR_ADDR=$(curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address')

# Set it as wallet owner (if you have governance access)
./keep-client ethereum ecdsa wallet-registry update-wallet-owner \
  $OPERATOR_ADDR \
  --submit \
  --config configs/config.toml
```

## Verify Wallet Owner is Set

After setting the wallet owner, verify it:

```bash
./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml
```

It should return a non-zero address.

## Requesting New Wallets

Once the wallet owner is set, you can request new wallets:

```bash
# Make sure the transaction is sent from the wallet owner address
./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml
```

**Important:** The transaction must be sent from the wallet owner address. Check your config file to ensure the `ethereum.keyFile` points to the wallet owner's keyfile.

## Troubleshooting

### "Wallet Owner address cannot be zero"
- The wallet owner hasn't been initialized
- Follow the steps above to set it

### "Caller is not the wallet owner"
- Your `ethereum.keyFile` in config doesn't match the wallet owner address
- Update your config to use the wallet owner's keyfile, or
- Change the wallet owner to match your current operator address

### Transaction Fails Silently
- Check that you have sufficient ETH for gas
- Verify the contract is properly deployed
- Check node logs for detailed error messages

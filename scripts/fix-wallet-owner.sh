#!/bin/bash
set -eou pipefail

# Script to fix wallet owner for local DKG testing
# 
# Usage:
#   ./scripts/fix-wallet-owner.sh [config-file] [wallet-owner-address]
#
# If wallet-owner-address is not provided, uses operator address from running node

CONFIG_FILE=${1:-"configs/config.toml"}
WALLET_OWNER_ADDR=${2:-""}
NETWORK="development"

echo "=========================================="
echo "Fix Wallet Owner for DKG Testing"
echo "=========================================="
echo ""

# Get operator address if not provided
if [ -z "$WALLET_OWNER_ADDR" ]; then
    echo "Step 1: Getting operator address from running node..."
    OPERATOR_ADDR=$(curl -s http://localhost:9601/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null || echo "")
    if [ -z "$OPERATOR_ADDR" ] || [ "$OPERATOR_ADDR" == "null" ]; then
        echo "⚠ Could not get operator address. Please provide it manually:"
        echo "  ./scripts/fix-wallet-owner.sh $CONFIG_FILE <address>"
        exit 1
    fi
    WALLET_OWNER_ADDR=$OPERATOR_ADDR
    echo "✓ Using operator address as wallet owner: $WALLET_OWNER_ADDR"
else
    echo "✓ Using provided wallet owner address: $WALLET_OWNER_ADDR"
fi
echo ""

# Check deployed contract address
echo "Step 2: Checking deployed contract addresses..."
echo "-----------------------------------"
DEPLOYED_WR=$(cat solidity/ecdsa/deployments/development/WalletRegistry.json 2>/dev/null | jq -r '.address' || echo "")
CONFIG_WR=$(grep -A 1 "\[developer\]" "$CONFIG_FILE" | grep "WalletRegistryAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")

if [ -n "$DEPLOYED_WR" ] && [ "$DEPLOYED_WR" != "null" ]; then
    echo "Deployed WalletRegistry: $DEPLOYED_WR"
    if [ -n "$CONFIG_WR" ] && [ "$CONFIG_WR" != "$DEPLOYED_WR" ]; then
        echo "⚠ WARNING: Config has different address: $CONFIG_WR"
        echo "  Consider updating config.toml to match deployed address"
    fi
else
    echo "⚠ Could not find deployed WalletRegistry address"
fi
echo ""

# Try to initialize wallet owner via Hardhat
echo "Step 3: Initializing wallet owner via WalletRegistryGovernance..."
echo "-----------------------------------"
cd solidity/ecdsa

# Check if wallet owner is already initialized
CURRENT_OWNER=$(npx hardhat run - <<EOF 2>&1 | grep -oE "0x[a-fA-F0-9]{40}" | head -1 || echo ""
const { deployments, ethers } = require("hardhat");
(async () => {
  const WalletRegistry = await ethers.getContractAt("WalletRegistry", "$DEPLOYED_WR");
  const owner = await WalletRegistry.walletOwner();
  console.log(owner);
})().catch(() => {});
EOF
)

if [ "$CURRENT_OWNER" == "0x0000000000000000000000000000000000000000" ] || [ -z "$CURRENT_OWNER" ]; then
    echo "Wallet owner is not initialized. Initializing..."
    npx hardhat initialize-wallet-owner \
      --wallet-owner-address "$WALLET_OWNER_ADDR" \
      --network "$NETWORK" 2>&1 | grep -E "(Initialized|Error|transaction)" || true
else
    echo "Wallet owner is already set to: $CURRENT_OWNER"
    if [ "$CURRENT_OWNER" != "$WALLET_OWNER_ADDR" ]; then
        echo ""
        echo "⚠ Wallet owner ($CURRENT_OWNER) doesn't match desired address ($WALLET_OWNER_ADDR)"
        echo ""
        echo "To update it, you need to use governance:"
        echo "  npx hardhat begin-wallet-owner-update --new-wallet-owner $WALLET_OWNER_ADDR --network $NETWORK"
        echo "  # Wait for governance delay..."
        echo "  npx hardhat finalize-wallet-owner-update --network $NETWORK"
        echo ""
        echo "OR update your config to use the wallet owner's keyfile:"
        echo "  [ethereum]"
        echo "  KeyFile = \"/path/to/keyfile-for-$CURRENT_OWNER\""
    else
        echo "✓ Wallet owner matches your operator address!"
    fi
fi

cd - > /dev/null

echo ""
echo "=========================================="
echo "Next Steps:"
echo "=========================================="
echo ""
echo "1. Verify wallet owner is set:"
echo "   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallet-owner --config $CONFIG_FILE --developer"
echo ""
echo "2. Request new wallet (triggers DKG):"
echo "   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config $CONFIG_FILE --developer"
echo ""
echo "3. Monitor DKG progress:"
echo "   watch -n 2 'curl -s http://localhost:9601/metrics | grep performance_dkg'"
echo ""

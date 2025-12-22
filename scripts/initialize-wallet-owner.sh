#!/bin/bash
# Initialize Wallet Owner for WalletRegistry

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/solidity/ecdsa"

# Get operator1 address (from node1 config)
OPERATOR1_KEYFILE=$(grep "^KeyFile" "$PROJECT_ROOT/configs/node1.toml" | head -1 | sed 's/.*KeyFile.*=.*"\(.*\)"/\1/')
OPERATOR1_ADDRESS=$(echo "$OPERATOR1_KEYFILE" | sed -E 's/.*--([a-fA-F0-9]{40})$/\1/' | tr '[:upper:]' '[:lower:]' | sed 's/^/0x/')

if [ -z "$OPERATOR1_ADDRESS" ] || [ "$OPERATOR1_ADDRESS" = "0x" ]; then
  echo "Error: Could not extract operator1 address from config"
  exit 1
fi

echo "=========================================="
echo "Initializing Wallet Owner"
echo "=========================================="
echo ""
echo "Wallet Owner Address: $OPERATOR1_ADDRESS"
echo ""
echo "This will set the Wallet Owner in WalletRegistryGovernance."
echo "Only the governance account can initialize the wallet owner."
echo ""

# Unlock accounts first
echo "Step 1: Unlocking accounts..."
KEEP_ETHEREUM_PASSWORD=${KEEP_ETHEREUM_PASSWORD:-password} \
  npx hardhat unlock-accounts --network development || {
  echo "⚠ Warning: Account unlock failed. Continuing anyway..."
}
echo ""

# Initialize wallet owner
echo "Step 2: Initializing wallet owner..."
npx hardhat initialize-wallet-owner \
  --wallet-owner-address "$OPERATOR1_ADDRESS" \
  --network development

echo ""
echo "=========================================="
echo "Wallet Owner Initialized!"
echo "=========================================="
echo ""
echo "You can now request new wallets using operator1 account:"
echo "  cd $PROJECT_ROOT"
echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
echo "    --submit --config configs/config.toml --developer"
echo ""

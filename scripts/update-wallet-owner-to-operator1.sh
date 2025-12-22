#!/bin/bash
# Update Wallet Owner to operator1 address

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/solidity/ecdsa"

# Get operator1 address
OPERATOR1_KEYFILE=$(grep "^KeyFile" "$PROJECT_ROOT/configs/node1.toml" | head -1 | sed 's/.*KeyFile.*=.*"\(.*\)"/\1/')
OPERATOR1_ADDRESS=$(echo "$OPERATOR1_KEYFILE" | sed -E 's/.*--([a-fA-F0-9]{40})$/\1/' | tr '[:upper:]' '[:lower:]' | sed 's/^/0x/')

echo "=========================================="
echo "Updating Wallet Owner to Operator1"
echo "=========================================="
echo ""
echo "New Wallet Owner: $OPERATOR1_ADDRESS"
echo ""

# Unlock accounts
echo "Step 1: Unlocking accounts..."
KEEP_ETHEREUM_PASSWORD=${KEEP_ETHEREUM_PASSWORD:-password} \
  npx hardhat unlock-accounts --network development || {
  echo "⚠ Warning: Account unlock failed. Continuing anyway..."
}
echo ""

# Check current wallet owner
echo "Step 2: Checking current wallet owner..."
CURRENT_OWNER=$(cd "$PROJECT_ROOT" && KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config configs/config.toml --developer 2>&1 | tail -1)
echo "Current Wallet Owner: $CURRENT_OWNER"
echo ""

if [ "$(echo "$CURRENT_OWNER" | tr '[:upper:]' '[:lower:]')" = "$(echo "$OPERATOR1_ADDRESS" | tr '[:upper:]' '[:lower:]')" ]; then
  echo "✅ Wallet Owner is already set to operator1!"
  echo ""
  echo "You can now request new wallets:"
  echo "  cd $PROJECT_ROOT"
  echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
  echo "    --submit --config configs/config.toml --developer"
  exit 0
fi

echo "Step 3: Beginning wallet owner update..."
echo "This requires governance account (account index 2)"
echo ""

# Use Hardhat task to update wallet owner
npx hardhat update-wallet-owner \
  --new-owner "$OPERATOR1_ADDRESS" \
  --network development

echo ""
echo "=========================================="
echo "Update Process Complete"
echo "=========================================="

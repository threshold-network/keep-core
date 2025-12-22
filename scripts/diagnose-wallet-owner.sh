#!/bin/bash
set -eou pipefail

# Script to diagnose wallet owner issue and provide solutions
# 
# Usage:
#   ./scripts/diagnose-wallet-owner.sh [config-file]

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

echo "=========================================="
echo "Wallet Owner Diagnostic Tool"
echo "=========================================="
echo ""

# Check if keep-client exists
if [ ! -f "$KEEP_CLIENT" ]; then
    echo "Error: keep-client binary not found at $KEEP_CLIENT"
    exit 1
fi

# Get operator address from running node
echo "Step 1: Getting your operator address..."
echo "-----------------------------------"
OPERATOR_ADDR=$(curl -s http://localhost:9601/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null || echo "")
if [ -z "$OPERATOR_ADDR" ] || [ "$OPERATOR_ADDR" == "null" ]; then
    echo "⚠ Could not get operator address from diagnostics"
    echo "  Make sure your node is running"
    exit 1
fi
echo "✓ Your operator address: $OPERATOR_ADDR"
echo ""

# Get wallet owner from contract
echo "Step 2: Getting wallet owner from contract..."
echo "-----------------------------------"
WALLET_OWNER=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry wallet-owner --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^0x[a-fA-F0-9]{40}$" | head -1 || echo "")
if [ -z "$WALLET_OWNER" ]; then
    echo "⚠ Could not get wallet owner address"
    exit 1
fi
echo "✓ Wallet owner address: $WALLET_OWNER"
echo ""

# Compare addresses
echo "Step 3: Comparing addresses..."
echo "-----------------------------------"
if [ "$OPERATOR_ADDR" == "$WALLET_OWNER" ]; then
    echo "✓ SUCCESS: Your operator address matches the wallet owner!"
    echo ""
    echo "You should be able to request new wallets now:"
    echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config $CONFIG_FILE --developer"
else
    echo "✗ MISMATCH: Your operator address does NOT match the wallet owner"
    echo ""
    echo "  Operator:  $OPERATOR_ADDR"
    echo "  Owner:     $WALLET_OWNER"
    echo ""
    echo "Solutions:"
    echo ""
    echo "Option 1: Update config to use wallet owner's keyfile"
    echo "  Find the keyfile for address: $WALLET_OWNER"
    echo "  Update config.toml:"
    echo "    [ethereum]"
    echo "    KeyFile = \"/path/to/keyfile-for-$WALLET_OWNER\""
    echo ""
    echo "Option 2: Update wallet owner to match your operator"
    echo "  This requires governance access. For local development:"
    echo ""
    echo "  cd solidity/ecdsa"
    echo "  # Check if you can update via governance"
    echo "  npx hardhat --network development begin-wallet-owner-update \\"
    echo "    --new-wallet-owner $OPERATOR_ADDR"
    echo ""
    echo "Option 3: Use Bridge address as wallet owner (production setup)"
    echo "  If you have Bridge contract deployed, use its address"
    echo ""
fi

echo ""
echo "=========================================="

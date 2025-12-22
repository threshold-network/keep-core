#!/bin/bash
set -eou pipefail

# Script to set wallet owner for local development
# 
# Usage:
#   ./scripts/set-wallet-owner.sh [wallet-owner-address] [network]
#
# If wallet-owner-address is not provided, it will use your operator address from diagnostics
# If network is not provided, it defaults to "development"

WALLET_OWNER_ADDR=${1:-""}
NETWORK=${2:-"development"}
CLIENT_INFO_PORT=${3:-"9601"}

echo "=========================================="
echo "Set Wallet Owner for WalletRegistry"
echo "=========================================="
echo ""

# If wallet owner address not provided, try to get it from diagnostics
if [ -z "$WALLET_OWNER_ADDR" ]; then
    echo "Wallet owner address not provided. Attempting to get operator address from running node..."
    
    if curl -s "http://localhost:$CLIENT_INFO_PORT/diagnostics" > /dev/null 2>&1; then
        WALLET_OWNER_ADDR=$(curl -s "http://localhost:$CLIENT_INFO_PORT/diagnostics" | jq -r '.client_info.chain_address' 2>/dev/null || echo "")
        
        if [ -z "$WALLET_OWNER_ADDR" ] || [ "$WALLET_OWNER_ADDR" == "null" ]; then
            echo "⚠ Could not get operator address from diagnostics"
            echo ""
            echo "Please provide wallet owner address manually:"
            echo "  ./scripts/set-wallet-owner.sh <wallet-owner-address>"
            exit 1
        fi
        
        echo "✓ Found operator address: $WALLET_OWNER_ADDR"
        echo "  Using this as wallet owner address"
    else
        echo "⚠ Could not reach diagnostics endpoint at http://localhost:$CLIENT_INFO_PORT/diagnostics"
        echo ""
        echo "Please provide wallet owner address manually:"
        echo "  ./scripts/set-wallet-owner.sh <wallet-owner-address>"
        exit 1
    fi
fi

echo ""
echo "Network: $NETWORK"
echo "Wallet Owner Address: $WALLET_OWNER_ADDR"
echo ""

# Check if Hardhat is available
if ! command -v npx &> /dev/null; then
    echo "✗ Error: npx not found. Please install Node.js and npm"
    exit 1
fi

# Navigate to ecdsa directory
ECDSA_DIR="solidity/ecdsa"
if [ ! -d "$ECDSA_DIR" ]; then
    echo "✗ Error: $ECDSA_DIR directory not found"
    echo "  Make sure you're running this from the keep-core root directory"
    exit 1
fi

echo "Initializing wallet owner..."
echo "-----------------------------------"
cd "$ECDSA_DIR"

# Run the Hardhat task
npx hardhat initialize-wallet-owner \
  --wallet-owner-address "$WALLET_OWNER_ADDR" \
  --network "$NETWORK"

EXIT_CODE=$?

cd - > /dev/null

if [ $EXIT_CODE -eq 0 ]; then
    echo ""
    echo "=========================================="
    echo "✓ Wallet owner initialized successfully!"
    echo "=========================================="
    echo ""
    echo "You can now request new wallets to trigger DKG:"
    echo "  ./scripts/test-dkg.sh configs/config.toml"
else
    echo ""
    echo "✗ Error initializing wallet owner"
    echo ""
    echo "Common issues:"
    echo "  - Wallet owner already initialized (can only be set once)"
    echo "  - Network not configured correctly"
    echo "  - Governance account doesn't have permissions"
    echo "  - Contracts not deployed"
    exit 1
fi

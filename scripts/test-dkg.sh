#!/bin/bash
set -eou pipefail

# Script to test DKG (Distributed Key Generation) on local keep-client
# 
# Prerequisites:
# 1. Local Ethereum node running (Geth on developer network)
# 2. Contracts deployed (use --developer flag or local network)
# 3. At least one keep-client node running and properly configured
# 4. Your operator must be registered and authorized in the WalletRegistry
#
# Usage:
#   ./scripts/test-dkg.sh [config-file]
#
# Example:
#   ./scripts/test-dkg.sh configs/config.toml

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

echo "=========================================="
echo "DKG Testing Script for Keep-Client"
echo "=========================================="
echo ""

# Check if keep-client exists
if [ ! -f "$KEEP_CLIENT" ]; then
    echo "Error: keep-client binary not found at $KEEP_CLIENT"
    echo "Please build it first: go build -o keep-client ."
    exit 1
fi

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Config file not found: $CONFIG_FILE"
    exit 1
fi

echo "Using config file: $CONFIG_FILE"
echo ""

# Step 1: Check node status
echo "Step 1: Checking node status..."
echo "-----------------------------------"
METRICS_URL=$(grep -A 5 "\[clientInfo\]" "$CONFIG_FILE" | grep "Port" | cut -d'=' -f2 | tr -d ' ' || echo "9601")
CLIENT_INFO_PORT=${METRICS_URL:-9601}

if curl -s "http://localhost:$CLIENT_INFO_PORT/metrics" > /dev/null 2>&1; then
    echo "✓ Node is running and metrics endpoint is accessible"
    CONNECTED_PEERS=$(curl -s "http://localhost:$CLIENT_INFO_PORT/metrics" | grep "connected_peers_count" | awk '{print $2}' || echo "0")
    echo "  Connected peers: $CONNECTED_PEERS"
else
    echo "⚠ Warning: Could not reach metrics endpoint at http://localhost:$CLIENT_INFO_PORT/metrics"
    echo "  Make sure your keep-client is running with this config file"
fi
echo ""

# Step 2: Get wallet owner address (needed to request new wallet)
echo "Step 2: Getting wallet owner address..."
echo "-----------------------------------"
WALLET_OWNER=$(./keep-client ethereum ecdsa wallet-registry wallet-owner --config "$CONFIG_FILE" 2>/dev/null | head -1 | tr -d ' ' || echo "")
if [ -z "$WALLET_OWNER" ] || [[ "$WALLET_OWNER" == *"Usage:"* ]] || [[ "$WALLET_OWNER" == *"Available Commands:"* ]]; then
    echo "⚠ Warning: Could not get wallet owner address"
    echo "  You may need to check your config and ensure contracts are deployed"
    echo "  Try running: ./keep-client ethereum ecdsa wallet-registry wallet-owner --config $CONFIG_FILE"
    WALLET_OWNER=""
elif [[ "$WALLET_OWNER" == "0x0000000000000000000000000000000000000000" ]] || [[ "$WALLET_OWNER" == "0x0" ]]; then
    echo "⚠ Warning: Wallet owner is not initialized (zero address)"
    echo "  Wallet owner address: $WALLET_OWNER"
    echo ""
    echo "  For local development, you need to initialize the wallet owner first."
    echo "  This is typically done during contract deployment or via governance."
    echo ""
    echo "  To set wallet owner (requires governance/owner access):"
    echo "    ./keep-client ethereum ecdsa wallet-registry update-wallet-owner <address> --submit --config $CONFIG_FILE"
    echo ""
    echo "  Or if using WalletRegistryGovernance:"
    echo "    Use Hardhat task: initialize-wallet-owner"
    echo ""
    WALLET_OWNER="0x0000000000000000000000000000000000000000"
else
    echo "✓ Wallet owner address: $WALLET_OWNER"
fi
echo ""

# Step 3: Check current wallet state
echo "Step 3: Checking current wallet state..."
echo "-----------------------------------"
echo "Checking if there are existing wallets..."
# This is a placeholder - actual command depends on available wallet-registry subcommands
echo ""

# Step 4: Request new wallet (triggers DKG)
echo "Step 4: Requesting new wallet (this triggers DKG)..."
echo "-----------------------------------"
echo "Command: ./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config $CONFIG_FILE"
echo ""
read -p "Do you want to proceed with requesting a new wallet? (y/N): " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    # Check if wallet owner is zero address
    if [[ "$WALLET_OWNER" == "0x0000000000000000000000000000000000000000" ]] || [[ -z "$WALLET_OWNER" ]]; then
        echo "✗ Cannot proceed: Wallet owner is not initialized"
        echo ""
        echo "The requestNewWallet() function requires the caller to be the wallet owner."
        echo "Since wallet owner is zero address, the transaction will fail."
        echo ""
        echo "Please initialize the wallet owner first, then run this script again."
        exit 1
    fi
    
    echo "Submitting request-new-wallet transaction..."
    echo "Note: This transaction must be sent from the wallet owner address: $WALLET_OWNER"
    echo ""
    
    TX_OUTPUT=$(./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config "$CONFIG_FILE" 2>&1)
    EXIT_CODE=$?
    
    # Extract transaction hash (usually the last line or line containing "0x")
    TX_HASH=$(echo "$TX_OUTPUT" | grep -oE "0x[a-fA-F0-9]{64}" | tail -1 || echo "")
    
    if [ $EXIT_CODE -eq 0 ] && [ -n "$TX_HASH" ]; then
        echo "✓ Transaction submitted successfully!"
        echo "  Transaction hash: $TX_HASH"
        echo ""
        echo "The DKG process will now start automatically:"
        echo "  1. Random Beacon will generate a relay entry"
        echo "  2. WalletRegistry will select a group of operators"
        echo "  3. Selected operators will perform DKG off-chain"
        echo "  4. DKG result will be submitted to the chain"
        echo ""
        echo "Monitor your node logs to see DKG participation."
    else
        echo "✗ Error submitting transaction:"
        echo "$TX_OUTPUT"
        echo ""
        echo "Common issues:"
        echo "  - Wallet owner not initialized (must not be zero address)"
        echo "  - Transaction sender is not the wallet owner"
        echo "  - Insufficient ETH balance for gas"
        echo "  - DKG already in progress"
        exit 1
    fi
else
    echo "Skipped. You can run the command manually:"
    echo "  ./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config $CONFIG_FILE"
fi
echo ""

# Step 5: Monitor DKG progress
echo "Step 5: Monitoring DKG progress..."
echo "-----------------------------------"
echo "You can monitor DKG progress by:"
echo ""
echo "1. Watching node logs for DKG events:"
echo "   tail -f <your-log-file> | grep -i dkg"
echo ""
echo "2. Checking metrics:"
echo "   watch -n 2 'curl -s http://localhost:$CLIENT_INFO_PORT/metrics | grep performance_dkg'"
echo ""
echo "3. Checking diagnostics:"
echo "   curl -s http://localhost:$CLIENT_INFO_PORT/diagnostics | jq"
echo ""
echo "4. Querying wallet registry for DKG state:"
echo "   ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state --config $CONFIG_FILE"
echo ""

echo "=========================================="
echo "DKG Test Script Complete"
echo "=========================================="

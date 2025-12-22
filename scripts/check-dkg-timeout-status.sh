#!/bin/bash
set -eou pipefail

# Script to check DKG timeout status and estimate when timeout will occur
# 
# Usage:
#   ./scripts/check-dkg-timeout-status.sh [config-file]

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

echo "=========================================="
echo "DKG Timeout Status Check"
echo "=========================================="
echo ""

# Check current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry get-wallet-creation-state --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1)

if [ -z "$STATE" ]; then
    echo "⚠ Could not get DKG state"
    exit 1
fi

get_state_name() {
    case "$1" in
        0) echo "IDLE" ;;
        1) echo "AWAITING_SEED" ;;
        2) echo "AWAITING_RESULT" ;;
        3) echo "CHALLENGE" ;;
        *) echo "UNKNOWN ($1)" ;;
    esac
}

STATE_NAME=$(get_state_name "$STATE")
echo "Current State: $STATE_NAME"
echo ""

if [ "$STATE" != "2" ]; then
    echo "ℹ DKG is not in AWAITING_RESULT state"
    if [ "$STATE" == "0" ]; then
        echo "✓ DKG is IDLE - no timeout check needed"
    fi
    exit 0
fi

echo "DKG is in AWAITING_RESULT state"
echo ""

# Try to call notify-dkg-timeout to see the error
echo "Checking timeout status..."
CALL_RESULT=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry notify-dkg-timeout --config "$CONFIG_FILE" --developer 2>&1)

if echo "$CALL_RESULT" | grep -q "DKG has not timed out"; then
    echo "⚠ DKG timeout has NOT passed yet"
    echo ""
    echo "Timeout Requirements:"
    echo "  - DKG timeout: 536 blocks"
    echo "  - Local dev (~1s/block): ~8-9 minutes"
    echo "  - Mainnet (~15s/block): ~2.2 hours"
    echo ""
    echo "The timeout is calculated from when DKG started (when seed was received)."
    echo ""
    echo "What to do:"
    echo "  1. Wait for more blocks to be mined"
    echo "  2. Check your node logs to see when DKG started"
    echo "  3. In local dev, wait ~10 minutes total from DKG start"
    echo ""
    echo "You can keep checking with:"
    echo "  ./scripts/check-and-reset-dkg.sh $CONFIG_FILE"
    echo ""
    echo "Or manually check:"
    echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \\"
    echo "    --config $CONFIG_FILE --developer"
    echo ""
    echo "Note: In local development with a single operator, DKG will likely"
    echo "      never complete (needs 100 operators). You can reset after timeout."
elif echo "$CALL_RESULT" | grep -q "success"; then
    echo "✓ DKG timeout HAS passed - ready to reset!"
    echo ""
    echo "You can now reset DKG:"
    echo "  ./scripts/check-and-reset-dkg.sh $CONFIG_FILE"
else
    echo "Unexpected response:"
    echo "$CALL_RESULT"
fi

echo ""

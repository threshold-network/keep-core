#!/bin/bash
set -eou pipefail

# Script to check DKG timeout status and reset if ready
# 
# Usage:
#   ./scripts/check-and-reset-dkg.sh [config-file]

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

echo "=========================================="
echo "Check and Reset DKG"
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
        echo "✓ DKG is already IDLE - no action needed"
    fi
    exit 0
fi

echo "DKG is in AWAITING_RESULT state"
echo ""

# Try to call notify-dkg-timeout (without submit) to check if timeout passed
echo "Checking if DKG timeout has passed..."
CALL_RESULT=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry notify-dkg-timeout --config "$CONFIG_FILE" --developer 2>&1)

if echo "$CALL_RESULT" | grep -q "DKG has not timed out"; then
    echo "⚠ DKG timeout has NOT passed yet"
    echo ""
    echo "The timeout is 536 blocks (~8-9 minutes locally)"
    echo ""
    echo "Options:"
    echo "  1. Wait for timeout to pass, then run this script again"
    echo "  2. Check node logs to see why operators aren't submitting:"
    echo "     tail -f <log-file> | grep -i dkg"
    echo "  3. If in local dev with single operator, DKG may never complete"
    echo "     (needs 100 operators for full DKG)"
    exit 1
elif echo "$CALL_RESULT" | grep -q "success"; then
    echo "✓ DKG timeout has passed - ready to reset"
    echo ""
    echo "Submitting reset transaction..."
    echo ""
    
    RESULT=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry notify-dkg-timeout --submit --config "$CONFIG_FILE" --developer 2>&1)
    
    if echo "$RESULT" | grep -q "0x"; then
        TX_HASH=$(echo "$RESULT" | grep "0x" | head -1)
        echo "✓ Reset transaction submitted: $TX_HASH"
        echo ""
        echo "Waiting for confirmation..."
        sleep 5
        
        NEW_STATE=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry get-wallet-creation-state --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1)
        NEW_STATE_NAME=$(get_state_name "$NEW_STATE")
        echo "New State: $NEW_STATE_NAME"
        
        if [ "$NEW_STATE" == "0" ]; then
            echo ""
            echo "✓✓✓ DKG successfully reset to IDLE!"
            echo ""
            echo "You can now request a new wallet:"
            echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
            echo "    --submit --config $CONFIG_FILE --developer"
        else
            echo "⚠ State changed but not to IDLE. Current: $NEW_STATE_NAME"
        fi
    else
        echo "Error submitting transaction:"
        echo "$RESULT"
        exit 1
    fi
else
    echo "Unexpected response:"
    echo "$CALL_RESULT"
    exit 1
fi

echo ""

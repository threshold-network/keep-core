#!/bin/bash
set -eou pipefail

# Script to reset stuck DKG by calling notifyDkgTimeout
# 
# Usage:
#   ./scripts/reset-dkg.sh [config-file]

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

echo "=========================================="
echo "Reset Stuck DKG"
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

if [ "$STATE" == "0" ]; then
    echo "✓ DKG is already IDLE - no reset needed"
    exit 0
fi

if [ "$STATE" == "2" ]; then
    echo "⚠ DKG is in AWAITING_RESULT state"
    echo "   Attempting to reset via notifyDkgTimeout..."
    echo ""
    
    # Try to reset
    RESULT=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry notify-dkg-timeout --submit --config "$CONFIG_FILE" --developer 2>&1)
    
    if echo "$RESULT" | grep -q "DKG has not timed out"; then
        echo "⚠ DKG timeout has not been reached yet"
        echo "   You need to wait for 536 blocks (~8-9 minutes locally)"
        echo ""
        echo "   Current state will remain until:"
        echo "   - Operators submit result, OR"
        echo "   - Timeout is reached and reset"
        exit 1
    elif echo "$RESULT" | grep -q "0x"; then
        echo "✓ DKG reset transaction submitted:"
        echo "$RESULT" | grep "0x"
        echo ""
        echo "Waiting a few seconds for confirmation..."
        sleep 3
        
        NEW_STATE=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry get-wallet-creation-state --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1)
        NEW_STATE_NAME=$(get_state_name "$NEW_STATE")
        echo "New State: $NEW_STATE_NAME"
        
        if [ "$NEW_STATE" == "0" ]; then
            echo "✓ DKG successfully reset to IDLE"
        fi
    else
        echo "Error resetting DKG:"
        echo "$RESULT"
        exit 1
    fi
elif [ "$STATE" == "1" ]; then
    echo "⚠ DKG is in AWAITING_SEED state"
    echo "   Use notify-seed-timeout instead:"
    echo ""
    echo "   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \\"
    echo "     --submit --config $CONFIG_FILE --developer"
elif [ "$STATE" == "3" ]; then
    echo "⚠ DKG is in CHALLENGE state"
    echo "   Cannot reset during challenge period"
    echo "   Wait for challenge period to complete or result to be approved"
else
    echo "⚠ Unknown state: $STATE"
fi

echo ""

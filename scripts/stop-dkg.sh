#!/bin/bash
# Script to stop DKG if timed out
# Usage: ./scripts/stop-dkg.sh

set -eou pipefail

CONFIG_FILE=${1:-"configs/config.toml"}

echo "=========================================="
echo "Stop DKG / New Wallet Creation"
echo "=========================================="
echo ""

# Check current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")

if [ -z "$STATE" ]; then
    echo "⚠ Could not get DKG state"
    exit 1
fi

echo "Current DKG State: $STATE"
echo ""

case "$STATE" in
    0)
        echo "✓ DKG is already IDLE. No DKG in progress."
        echo ""
        echo "To prevent new wallet creation:"
        echo "  - Don't call 'request-new-wallet'"
        echo "  - Or stop nodes: ./configs/stop-all-nodes.sh"
        ;;
    1)
        echo "⏳ DKG is AWAITING_SEED"
        echo ""
        echo "Checking if seed has timed out..."
        HAS_SEED_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
          --config "$CONFIG_FILE" --developer 2>&1 | tail -1 || echo "false")
        
        if [ "$HAS_SEED_TIMED_OUT" = "true" ]; then
            echo "✓ Seed has timed out. Notifying seed timeout..."
            KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
              --submit --config "$CONFIG_FILE" --developer
            echo ""
            echo "✓ Seed timeout notified. State should reset to IDLE."
        else
            echo "⚠ Seed has not timed out yet."
            echo ""
            echo "Options:"
            echo "  1. Wait for seed timeout"
            echo "  2. Stop nodes: ./configs/stop-all-nodes.sh"
        fi
        ;;
    2)
        echo "⏳ DKG is AWAITING_RESULT"
        echo ""
        echo "Checking if DKG has timed out..."
        HAS_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
          --config "$CONFIG_FILE" --developer 2>&1 | tail -1 || echo "false")
        
        if [ "$HAS_TIMED_OUT" = "true" ]; then
            echo "✓ DKG has timed out. Notifying timeout..."
            KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
              --submit --config "$CONFIG_FILE" --developer
            echo ""
            sleep 3
            echo "Verifying state reset..."
            NEW_STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
              --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")
            
            if [ "$NEW_STATE" = "0" ]; then
                echo "✓ DKG stopped successfully. State is now IDLE."
            else
                echo "⚠ State is still: $NEW_STATE"
            fi
        else
            echo "⚠ DKG has not timed out yet (~9 minutes total from start)."
            echo ""
            echo "Options:"
            echo "  1. Wait for timeout (~9 minutes total)"
            echo "  2. Stop nodes to prevent participation:"
            echo "     ./configs/stop-all-nodes.sh"
            echo ""
            echo "Note: Stopping nodes prevents participation but doesn't cancel on-chain DKG state."
        fi
        ;;
    3)
        echo "⏳ DKG is in CHALLENGE period"
        echo ""
        echo "DKG result has been submitted and is in challenge period."
        echo "Cannot stop at this stage. Must wait for approval or challenge."
        ;;
    *)
        echo "⚠ Unknown state: $STATE"
        ;;
esac

echo ""
echo "=========================================="
echo "To prevent new wallet creation:"
echo "=========================================="
echo ""
echo "1. Don't call 'request-new-wallet'"
echo "2. Stop nodes: ./configs/stop-all-nodes.sh"
echo "3. Check state before requesting:"
echo "   ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \\"
echo "     --config $CONFIG_FILE --developer"
echo ""

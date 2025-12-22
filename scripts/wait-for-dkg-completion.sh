#!/bin/bash
# Script to wait for DKG completion or timeout
# Usage: ./scripts/wait-for-dkg-completion.sh [timeout-seconds]

set -eou pipefail

TIMEOUT=${1:-300}  # Default 5 minutes
CONFIG_FILE="configs/config.toml"
CHECK_INTERVAL=10

echo "=========================================="
echo "Waiting for DKG Completion"
echo "=========================================="
echo "Timeout: ${TIMEOUT}s"
echo "Check interval: ${CHECK_INTERVAL}s"
echo ""

START_TIME=$(date +%s)
ELAPSED=0

while [ $ELAPSED -lt $TIMEOUT ]; do
    STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
        --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")
    
    if [ -z "$STATE" ]; then
        echo "⚠ Could not get DKG state"
        sleep $CHECK_INTERVAL
        ELAPSED=$(($(date +%s) - START_TIME))
        continue
    fi
    
    case "$STATE" in
        0)
            echo "✅ DKG completed! State is IDLE"
            echo ""
            echo "You can now trigger a new DKG:"
            echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
            echo "    --submit --config $CONFIG_FILE --developer"
            exit 0
            ;;
        1)
            echo "⏳ State: AWAITING_SEED ($ELAPSED/${TIMEOUT}s)"
            ;;
        2)
            echo "⏳ State: AWAITING_RESULT - DKG in progress ($ELAPSED/${TIMEOUT}s)"
            # Check for keygen activity
            RECENT_KEYGEN=$(tail -20 logs/node*.log 2>/dev/null | grep -c "keygen/prepare.go" || echo "0")
            if [ "$RECENT_KEYGEN" -gt 0 ]; then
                echo "   ✓ Keygen activity detected"
            fi
            ;;
        3)
            echo "⏳ State: CHALLENGE - Result submitted ($ELAPSED/${TIMEOUT}s)"
            ;;
        *)
            echo "⚠ Unknown state: $STATE ($ELAPSED/${TIMEOUT}s)"
            ;;
    esac
    
    # Check if timed out
    HAS_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
        --config "$CONFIG_FILE" --developer 2>&1 | tail -1 || echo "false")
    
    if [ "$HAS_TIMED_OUT" = "true" ]; then
        echo ""
        echo "⚠ DKG has timed out"
        echo ""
        echo "To unlock the pool, run:"
        echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \\"
        echo "    --submit --config $CONFIG_FILE --developer"
        exit 1
    fi
    
    sleep $CHECK_INTERVAL
    ELAPSED=$(($(date +%s) - START_TIME))
done

echo ""
echo "⏱️  Timeout reached (${TIMEOUT}s)"
echo ""
echo "Current state: $STATE"
echo ""
echo "To check timeout status:"
echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \\"
echo "    --config $CONFIG_FILE --developer"
echo ""
echo "To unlock if timed out:"
echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \\"
echo "    --submit --config $CONFIG_FILE --developer"


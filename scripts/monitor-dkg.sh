#!/bin/bash
set -eou pipefail

# Script to monitor DKG state and progress
# 
# Usage:
#   ./scripts/monitor-dkg.sh [config-file]

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

# DKG State mapping function
get_state_name() {
    case "$1" in
        0) echo "IDLE" ;;
        1) echo "AWAITING_SEED" ;;
        2) echo "AWAITING_RESULT" ;;
        3) echo "CHALLENGE" ;;
        *) echo "UNKNOWN ($1)" ;;
    esac
}

echo "=========================================="
echo "DKG State Monitor"
echo "=========================================="
echo ""

# Get current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry get-wallet-creation-state --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1)

if [ -z "$STATE" ]; then
    echo "⚠ Could not get DKG state"
    exit 1
fi

STATE_NAME=$(get_state_name "$STATE")

echo "Current DKG State: $STATE_NAME"
echo ""

case "$STATE" in
    0)
        echo "✓ DKG is IDLE - ready to request new wallet"
        echo ""
        echo "To start a new DKG round:"
        echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet --submit --config $CONFIG_FILE --developer"
        ;;
    1)
        echo "⏳ DKG is AWAITING_SEED"
        echo "   Waiting for Random Beacon to provide seed..."
        echo ""
        echo "Monitor Random Beacon state or wait for seed generation."
        ;;
    2)
        echo "⏳ DKG is AWAITING_RESULT"
        echo "   Operators are generating keys..."
        echo ""
        echo "This can take several minutes. Monitor your node logs for:"
        echo "  - DKG protocol messages"
        echo "  - Key generation progress"
        echo ""
        echo "Check node metrics:"
        echo "  curl -s http://localhost:9601/metrics | grep performance_dkg"
        ;;
    3)
        echo "⏳ DKG is in CHALLENGE period"
        echo "   Result submitted, waiting for approval/challenge..."
        echo ""
        echo "The DKG result has been submitted and is in challenge period."
        ;;
    *)
        echo "⚠ Unknown state: $STATE"
        ;;
esac

echo ""
echo "=========================================="
echo "Node Status:"
echo "=========================================="

# Check if node is running
if curl -s http://localhost:9601/diagnostics > /dev/null 2>&1; then
    OPERATOR=$(curl -s http://localhost:9601/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null || echo "unknown")
    echo "✓ Node is running"
    echo "  Operator address: $OPERATOR"
    
    # Check DKG metrics if available
    METRICS=$(curl -s http://localhost:9601/metrics 2>/dev/null | grep -E "performance_dkg|dkg_" | head -5 || echo "")
    if [ -n "$METRICS" ]; then
        echo ""
        echo "DKG Metrics:"
        echo "$METRICS" | sed 's/^/  /'
    fi
else
    echo "⚠ Node is not running or diagnostics endpoint unavailable"
fi

echo ""

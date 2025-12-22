#!/bin/bash
# Script to check on-chain DKG state
# Usage: ./scripts/check-dkg-state.sh [config-file]

set -eou pipefail

CONFIG_FILE=${1:-"configs/config.toml"}

echo "=========================================="
echo "On-Chain DKG State Check"
echo "=========================================="
echo ""

# Get current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")

if [ -z "$STATE" ]; then
    echo "⚠ Could not get DKG state"
    exit 1
fi

# Map state to name
get_state_name() {
    case "$1" in
        0) echo "IDLE" ;;
        1) echo "AWAITING_SEED" ;;
        2) echo "AWAITING_RESULT" ;;
        3) echo "CHALLENGE" ;;
        *) echo "UNKNOWN" ;;
    esac
}

STATE_NAME=$(get_state_name "$STATE")

echo "Current DKG State: $STATE ($STATE_NAME)"
echo ""

# Check timeout status
echo "Timeout Status:"
echo "---------------"

HAS_DKG_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config "$CONFIG_FILE" --developer 2>&1 | tail -1 || echo "false")

HAS_SEED_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config "$CONFIG_FILE" --developer 2>&1 | tail -1 || echo "false")

echo "DKG Timed Out: $HAS_DKG_TIMED_OUT"
echo "Seed Timed Out: $HAS_SEED_TIMED_OUT"
echo ""

# State-specific information
case "$STATE" in
    0)
        echo "✓ DKG is IDLE - No DKG in progress"
        echo ""
        echo "You can request a new wallet:"
        echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
        echo "    --submit --config $CONFIG_FILE --developer"
        ;;
    1)
        echo "⏳ DKG is AWAITING_SEED"
        echo "   Waiting for Random Beacon to provide seed..."
        echo ""
        if [ "$HAS_SEED_TIMED_OUT" = "true" ]; then
            echo "⚠ Seed has timed out!"
            echo ""
            echo "To unlock the pool:"
            echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \\"
            echo "    --submit --config $CONFIG_FILE --developer"
        else
            echo "Seed timeout has not occurred yet."
        fi
        ;;
    2)
        echo "⏳ DKG is AWAITING_RESULT"
        echo "   Operators are generating keys off-chain..."
        echo ""
        if [ "$HAS_DKG_TIMED_OUT" = "true" ]; then
            echo "⚠ DKG has timed out!"
            echo ""
            echo "To unlock the pool:"
            echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \\"
            echo "    --submit --config $CONFIG_FILE --developer"
        else
            echo "DKG timeout has not occurred yet (~9 minutes total from start)."
            echo ""
            echo "Monitor progress:"
            echo "  ./scripts/monitor-dkg.sh"
            echo "  tail -f logs/node*.log | grep -i dkg"
        fi
        ;;
    3)
        echo "⏳ DKG is in CHALLENGE period"
        echo "   Result has been submitted and is in challenge period."
        echo ""
        echo "Waiting for approval or challenge..."
        ;;
    *)
        echo "⚠ Unknown state: $STATE"
        ;;
esac

echo ""
echo "=========================================="
echo "Quick Commands:"
echo "=========================================="
echo ""
echo "Check state:"
echo "  ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \\"
echo "    --config $CONFIG_FILE --developer"
echo ""
echo "Check timeout:"
echo "  ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \\"
echo "    --config $CONFIG_FILE --developer"
echo ""
echo "Monitor DKG:"
echo "  ./scripts/monitor-dkg.sh"
echo ""

#!/bin/bash
set -eou pipefail

# Script to diagnose why DKG is stuck in AWAITING_RESULT
# 
# Usage:
#   ./scripts/diagnose-dkg-stuck.sh [config-file]

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

echo "=========================================="
echo "DKG Stuck Diagnostic Tool"
echo "=========================================="
echo ""

# Check current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry get-wallet-creation-state --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1)

if [ -z "$STATE" ]; then
    echo "⚠ Could not get DKG state"
    exit 1
fi

if [ "$STATE" != "2" ]; then
    echo "ℹ DKG is not in AWAITING_RESULT state (current: $STATE)"
    echo "   This script is for diagnosing stuck DKG in AWAITING_RESULT state"
    exit 0
fi

echo "Current State: AWAITING_RESULT (stuck)"
echo ""

# Check if node is running
OPERATOR_ADDR=$(curl -s http://localhost:9601/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null || echo "")
if [ -z "$OPERATOR_ADDR" ] || [ "$OPERATOR_ADDR" == "null" ]; then
    echo "⚠ Node is not running or diagnostics unavailable"
    echo "   Start your node first: ./scripts/start.sh"
    exit 1
fi

echo "✓ Node is running"
echo "  Operator address: $OPERATOR_ADDR"
echo ""

# Check node logs for DKG errors (if log file exists)
echo "=========================================="
echo "Common Causes of Stuck DKG:"
echo "=========================================="
echo ""
echo "1. ❌ Operator Not Selected"
echo "   - Your operator may not have been selected for this DKG round"
echo "   - Check node logs for 'not eligible for DKG' or 'selecting group not possible'"
echo ""
echo "2. ❌ Insufficient Pre-Parameters"
echo "   - DKG requires pre-generated cryptographic parameters"
echo "   - Check node logs for 'pre-parameters pool size is too small'"
echo ""
echo "3. ❌ Network Connectivity Issues"
echo "   - Operators need LibP2P connectivity to communicate"
echo "   - Check node logs for connection errors"
echo ""
echo "4. ❌ Not Enough Operators in Pool"
echo "   - DKG needs 100 operators selected"
echo "   - In local dev, you may only have 1 operator"
echo ""
echo "5. ⏳ Still Processing (Normal)"
echo "   - DKG protocol takes ~8-9 minutes locally"
echo "   - Check if timeout has passed (536 blocks)"
echo ""

echo "=========================================="
echo "Diagnostic Steps:"
echo "=========================================="
echo ""

# Check node logs
echo "1. Check Node Logs for DKG Messages:"
echo "   Look for these patterns in your node logs:"
echo "   - 'checking eligibility for DKG'"
echo "   - 'joining DKG' or 'not eligible for DKG'"
echo "   - 'pre-parameters pool size is too small'"
echo "   - 'selecting group not possible'"
echo "   - 'DKG protocol' or 'GJKR' messages"
echo ""

# Check metrics
echo "2. Check DKG Metrics:"
METRICS=$(curl -s http://localhost:9601/metrics 2>/dev/null | grep -E "performance_dkg|dkg_" | head -10 || echo "")
if [ -n "$METRICS" ]; then
    echo "$METRICS" | sed 's/^/   /'
else
    echo "   No DKG metrics found"
fi
echo ""

# Check if we can reset
echo "3. Check DKG Timeout Status:"
echo "   DKG timeout is 536 blocks (~8-9 minutes locally)"
echo "   If timeout has passed, you can reset DKG:"
echo ""
echo "   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \\"
echo "     --submit --config $CONFIG_FILE --developer"
echo ""

echo "=========================================="
echo "Recovery Options:"
echo "=========================================="
echo ""
echo "Option 1: Reset DKG (if timeout passed)"
echo "  ./scripts/reset-dkg.sh $CONFIG_FILE"
echo ""
echo "Option 2: Check Node Logs"
echo "  tail -f <your-log-file> | grep -i dkg"
echo ""
echo "Option 3: Restart Node (if pre-params issue)"
echo "  # Stop node, then restart"
echo "  ./scripts/start.sh"
echo ""

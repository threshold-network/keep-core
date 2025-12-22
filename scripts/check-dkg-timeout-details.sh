#!/bin/bash
# Script to check detailed DKG timeout information
# Usage: ./scripts/check-dkg-timeout-details.sh

set -eou pipefail

CONFIG_FILE=${1:-"configs/config.toml"}

echo "=========================================="
echo "DKG Timeout Details"
echo "=========================================="
echo ""

# Get current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")

if [ -z "$STATE" ]; then
    echo "⚠ Could not get DKG state"
    exit 1
fi

echo "Current DKG State: $STATE"
echo ""

if [ "$STATE" != "2" ]; then
    echo "DKG is not in AWAITING_RESULT state. Timeout check only applies to state 2."
    exit 0
fi

# Get current block
CURRENT_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545 | jq -r '.result' | xargs -I {} printf "%d\n" {} 2>/dev/null || echo "0")

echo "Current Block: $CURRENT_BLOCK"
echo ""

# Check timeout
HAS_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config "$CONFIG_FILE" --developer 2>&1 | tail -1 || echo "false")

echo "Has Timed Out: $HAS_TIMED_OUT"
echo ""

if [ "$HAS_TIMED_OUT" = "false" ]; then
    echo "⚠ DKG has not timed out yet."
    echo ""
    echo "Possible reasons:"
    echo "  1. DKG started more recently than expected"
    echo "  2. Block time is slower than 1 second"
    echo "  3. resultSubmissionStartBlockOffset is non-zero (if result was challenged)"
    echo ""
    echo "The timeout calculation is:"
    echo "  block.number > (startBlock + resultSubmissionStartBlockOffset + 536)"
    echo ""
    echo "If a DKG result was challenged, the offset increases, extending the timeout."
    echo ""
    echo "To check for challenges, look for 'challenge' or 'DkgResultChallenged' in logs:"
    echo "  grep -i challenge logs/node*.log"
    echo ""
    echo "Continue monitoring. The timeout will eventually trigger."
else
    echo "✓ DKG has timed out!"
    echo ""
    echo "To unlock the pool:"
    echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \\"
    echo "    --submit --config $CONFIG_FILE --developer"
fi

echo ""

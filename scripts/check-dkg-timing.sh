#!/bin/bash
set -eou pipefail

# Script to check DKG timing and estimate completion
# 
# Usage:
#   ./scripts/check-dkg-timing.sh [config-file]

CONFIG_FILE=${1:-"configs/config.toml"}
KEEP_CLIENT="./keep-client"

echo "=========================================="
echo "DKG Timing Information"
echo "=========================================="
echo ""

# Get current state
STATE=$(KEEP_ETHEREUM_PASSWORD=password $KEEP_CLIENT ethereum ecdsa wallet-registry get-wallet-creation-state --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1)

if [ -z "$STATE" ]; then
    echo "⚠ Could not get DKG state"
    exit 1
fi

# Get state name
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

# DKG Timeout Parameters (from WalletRegistry.sol)
# These are the default values set during initialization
RESULT_SUBMISSION_TIMEOUT=536      # blocks
RESULT_CHALLENGE_PERIOD=11520      # blocks (~48h at 15s/block)
SEED_TIMEOUT=11520                 # blocks (~48h at 15s/block)

echo "DKG Timeout Parameters:"
echo "  - Result Submission Timeout: $RESULT_SUBMISSION_TIMEOUT blocks"
echo "  - Result Challenge Period: $RESULT_CHALLENGE_PERIOD blocks"
echo "  - Seed Timeout: $SEED_TIMEOUT blocks"
echo ""

# Estimate time (assuming 15s per block for mainnet, but local dev is faster)
BLOCK_TIME_MAINNET=15  # seconds
BLOCK_TIME_LOCAL=1     # seconds (approximate for local dev)

echo "Time Estimates (approximate):"
echo "  Mainnet (15s/block):"
echo "    - Result submission window: ~$((RESULT_SUBMISSION_TIMEOUT * BLOCK_TIME_MAINNET / 60)) minutes"
echo "    - Challenge period: ~$((RESULT_CHALLENGE_PERIOD * BLOCK_TIME_MAINNET / 3600)) hours"
echo ""
echo "  Local Development (1s/block):"
echo "    - Result submission window: ~$((RESULT_SUBMISSION_TIMEOUT * BLOCK_TIME_LOCAL / 60)) minutes"
echo "    - Challenge period: ~$((RESULT_CHALLENGE_PERIOD * BLOCK_TIME_LOCAL / 3600)) hours"
echo ""

# State-specific timing
case "$STATE" in
    0)
        echo "✓ DKG is IDLE - no active DKG round"
        ;;
    1)
        echo "⏳ AWAITING_SEED:"
        echo "   - Waiting for Random Beacon to provide seed"
        echo "   - Timeout: $SEED_TIMEOUT blocks (~$((SEED_TIMEOUT * BLOCK_TIME_LOCAL / 60)) minutes locally)"
        echo "   - If timeout exceeded, call notifySeedTimeout()"
        ;;
    2)
        echo "⏳ AWAITING_RESULT:"
        echo "   - Operators are generating keys off-chain"
        echo "   - Result must be submitted within $RESULT_SUBMISSION_TIMEOUT blocks"
        echo "   - Estimated time: ~$((RESULT_SUBMISSION_TIMEOUT * BLOCK_TIME_LOCAL / 60)) minutes locally"
        echo "   - After submission, enters CHALLENGE period"
        ;;
    3)
        echo "⏳ CHALLENGE:"
        echo "   - DKG result has been submitted"
        echo "   - Challenge period: $RESULT_CHALLENGE_PERIOD blocks"
        echo "   - Estimated time: ~$((RESULT_CHALLENGE_PERIOD * BLOCK_TIME_LOCAL / 60)) minutes locally"
        echo "   - After challenge period, result can be approved"
        echo "   - Once approved, DKG completes and state returns to IDLE"
        ;;
esac

echo ""
echo "=========================================="
echo "How DKG Completes:"
echo "=========================================="
echo ""
echo "1. Request New Wallet → State: AWAITING_SEED"
echo "2. Random Beacon provides seed → State: AWAITING_RESULT"
echo "3. Operators generate keys (off-chain protocol)"
echo "4. Result submitted → State: CHALLENGE"
echo "5. Challenge period passes ($RESULT_CHALLENGE_PERIOD blocks)"
echo "6. Result approved → State: IDLE (DKG Complete!)"
echo ""
echo "Total Time (happy path):"
echo "  - Mainnet: ~48 hours (mostly challenge period)"
echo "  - Local Dev: ~$((RESULT_CHALLENGE_PERIOD * BLOCK_TIME_LOCAL / 60)) minutes"
echo ""
echo "Note: In local development, block times are much faster,"
echo "      so DKG completes much quicker than on mainnet."
echo ""

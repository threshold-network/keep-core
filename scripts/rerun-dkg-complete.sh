#!/bin/bash
# Script to rerun DKG process with complete setup
# Usage: ./scripts/rerun-dkg-complete.sh [config-file]
#
# This script:
# 1. Checks if all operators are registered and in sortition pools
# 2. Ensures wallet owner is set
# 3. Checks current DKG state
# 4. Resets DKG if needed (if stuck)
# 5. Requests a new wallet to trigger DKG
# 6. Monitors DKG progress

set -eou pipefail

CONFIG_FILE=${1:-"configs/config.toml"}
MAIN_CONFIG="$CONFIG_FILE"

# Function to clean and validate addresses
clean_address() {
    local addr="$1"
    # Remove all whitespace, newlines, carriage returns, and any non-printable chars
    addr=$(printf '%s' "$addr" | tr -d '[:space:]\n\r' | tr -cd '0-9a-fA-Fx' | sed 's/^x/0x/' | sed 's/^\([^0]\)/0x\1/')
    # Ensure it starts with 0x
    if [[ "$addr" != 0x* ]]; then
        addr="0x$addr"
    fi
    # Convert to lowercase
    addr=$(echo "$addr" | tr '[:upper:]' '[:lower:]')
    # Take only first 42 characters (0x + 40 hex)
    addr=$(printf '%.42s' "$addr")
    echo "$addr"
}

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Complete DKG Rerun Setup"
echo "=========================================="
echo ""

# Step 1: Check if operators are registered
echo "Step 1: Checking operator registration..."
echo "----------------------------------------"

OPERATORS_IN_POOL=0
TOTAL_OPERATORS=0

for i in {1..10}; do
    NODE_CONFIG="configs/node${i}.toml"
    if [ ! -f "$NODE_CONFIG" ]; then
        continue
    fi
    
    KEYFILE=$(grep "^KeyFile" "$NODE_CONFIG" 2>/dev/null | cut -d'=' -f2 | tr -d ' "')
    if [ -z "$KEYFILE" ]; then
        continue
    fi
    
    OPERATOR=$(cat "$KEYFILE" 2>/dev/null | jq -r '.address' 2>/dev/null || echo "")
    if [ -z "$OPERATOR" ] || [[ "$OPERATOR" != 0x* ]]; then
        FILENAME=$(basename "$KEYFILE")
        OPERATOR=$(echo "$FILENAME" | sed -E 's/.*--([0-9a-fA-F]{40})$/\1/' | sed 's/^/0x/')
    fi
    
    # Clean and validate the address
    OPERATOR=$(clean_address "$OPERATOR")
    
    if [ -z "$OPERATOR" ] || [ ${#OPERATOR} -ne 42 ] || ! printf '%s' "$OPERATOR" | grep -qE '^0x[0-9a-f]{40}$'; then
        continue
    fi
    
    TOTAL_OPERATORS=$((TOTAL_OPERATORS + 1))
    
    # Check if in WalletRegistry pool
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
      --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -iE "true" || echo "false")
    
    if [ "$IN_POOL" = "true" ]; then
        OPERATORS_IN_POOL=$((OPERATORS_IN_POOL + 1))
        echo "  ✓ Node $i ($OPERATOR) - in pool"
    else
        echo "  ✗ Node $i ($OPERATOR) - NOT in pool"
    fi
done

echo ""
echo "Operators in pool: $OPERATORS_IN_POOL / $TOTAL_OPERATORS"
echo ""

if [ "$OPERATORS_IN_POOL" -lt 3 ]; then
    echo -e "${RED}⚠ Error: Need at least 3 operators in pool for DKG${NC}"
    echo ""
    echo "Register operators with:"
    echo "  ./scripts/register-single-operator.sh <node-number>"
    exit 1
fi

# Step 2: Check wallet owner
echo "Step 2: Checking wallet owner..."
echo "---------------------------------"

WALLET_REGISTRY="0x18266866EbBab6cA7f5F2724e22CEF54a98Cda92"
WALLET_OWNER=$(cd solidity/ecdsa && npx hardhat console --network development 2>&1 << 'EOF' | grep -oE "0x[0-9a-fA-F]{40}" | head -1 || echo "")
const { helpers } = require("hardhat");
(async () => {
  const wr = await helpers.contracts.getContract("WalletRegistry");
  const owner = await wr.walletOwner();
  console.log(owner);
  process.exit(0);
})();
EOF
cd ../..

if [ -z "$WALLET_OWNER" ]; then
    echo -e "${RED}⚠ Error: Could not get wallet owner${NC}"
    exit 1
fi

echo "Wallet Owner: $WALLET_OWNER"
echo ""

# Check if wallet owner has ETH
OWNER_BALANCE=$(cd solidity/ecdsa && npx hardhat console --network development 2>&1 << EOF | grep -oE "[0-9]+\.[0-9]+" | head -1 || echo "0")
const { ethers, helpers } = require("hardhat");
(async () => {
  const provider = ethers.provider;
  const balance = await provider.getBalance("$WALLET_OWNER");
  console.log(ethers.utils.formatEther(balance));
  process.exit(0);
})();
EOF
cd ../..

echo "Wallet Owner ETH Balance: $OWNER_BALANCE ETH"
echo ""

if (( $(echo "$OWNER_BALANCE < 0.1" | bc -l) )); then
    echo -e "${YELLOW}⚠ Warning: Wallet owner has low ETH balance${NC}"
    echo "Funding wallet owner..."
    
    MAIN_ACCOUNT="0x7966c178f466b060aaeb2b91e9149a5fb2ec9c53"
    cd solidity/ecdsa
    npx hardhat console --network development 2>&1 << EOF | grep -E "(Funded|Error)" || true
const { ethers } = require("ethers");
(async () => {
  const provider = new ethers.providers.JsonRpcProvider("http://localhost:8545");
  const mainSigner = await provider.getSigner("$MAIN_ACCOUNT");
  const tx = await mainSigner.sendTransaction({
    to: "$WALLET_OWNER",
    value: ethers.utils.parseEther("10")
  });
  await tx.wait();
  console.log("Funded wallet owner");
  process.exit(0);
})();
EOF
    cd ../..
fi

# Step 3: Check current DKG state
echo "Step 3: Checking current DKG state..."
echo "-------------------------------------"

STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$MAIN_CONFIG" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")

if [ -z "$STATE" ]; then
    echo -e "${RED}⚠ Error: Could not get DKG state${NC}"
    exit 1
fi

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

# Step 4: Reset DKG if needed
if [ "$STATE" != "0" ]; then
    echo "Step 4: DKG is not IDLE. Checking if reset is needed..."
    echo "------------------------------------------------------"
    
    HAS_DKG_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
      --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -iE "true" || echo "false")
    
    HAS_SEED_TIMED_OUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
      --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -iE "true" || echo "false")
    
    echo "DKG Timed Out: $HAS_DKG_TIMED_OUT"
    echo "Seed Timed Out: $HAS_SEED_TIMED_OUT"
    echo ""
    
    if [ "$HAS_DKG_TIMED_OUT" = "true" ]; then
        echo -e "${YELLOW}⚠ DKG has timed out. Resetting...${NC}"
        echo ""
        echo "Calling notify-dkg-timeout..."
        KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
          --submit --config "$MAIN_CONFIG" --developer 2>&1 | grep -E "(transaction|hash|0x[0-9a-f]{64})" || echo "  (may already be reset)"
        echo ""
        echo "Waiting for transaction to be mined..."
        sleep 5
    elif [ "$HAS_SEED_TIMED_OUT" = "true" ]; then
        echo -e "${YELLOW}⚠ Seed has timed out. Resetting...${NC}"
        echo ""
        echo "Calling notify-seed-timeout..."
        KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
          --submit --config "$MAIN_CONFIG" --developer 2>&1 | grep -E "(transaction|hash|0x[0-9a-f]{64})" || echo "  (may already be reset)"
        echo ""
        echo "Waiting for transaction to be mined..."
        sleep 5
    else
        echo -e "${YELLOW}⚠ DKG is active but not timed out yet${NC}"
        echo "Current state: $STATE_NAME"
        echo ""
        echo "You can:"
        echo "  1. Wait for DKG to complete or timeout"
        echo "  2. Force reset (if you're sure):"
        echo "     ./scripts/reset-dkg.sh"
        echo ""
        read -p "Do you want to wait or reset? (wait/reset): " choice
        if [ "$choice" = "reset" ]; then
            echo "Resetting DKG..."
            ./scripts/reset-dkg.sh "$MAIN_CONFIG" || echo "Reset script not found, trying manual reset..."
        else
            echo "Monitoring DKG..."
            ./scripts/monitor-dkg.sh || tail -f logs/node*.log | grep -i dkg
            exit 0
        fi
    fi
    
    # Re-check state after reset
    sleep 3
    STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
      --config "$MAIN_CONFIG" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "0")
    STATE_NAME=$(get_state_name "$STATE")
    echo "New DKG State: $STATE ($STATE_NAME)"
    echo ""
fi

# Step 5: Request new wallet (trigger DKG)
if [ "$STATE" = "0" ]; then
    echo "Step 5: Requesting new wallet to trigger DKG..."
    echo "------------------------------------------------"
    echo ""
    echo "This will:"
    echo "  1. Lock the DKG state"
    echo "  2. Request a relay entry from Random Beacon"
    echo "  3. Start DKG automatically when relay entry is generated"
    echo ""
    
    read -p "Continue? (y/n): " confirm
    if [ "$confirm" != "y" ]; then
        echo "Aborted."
        exit 0
    fi
    
    echo ""
    echo "Calling request-new-wallet..."
    TX_HASH=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
      --submit --config "$MAIN_CONFIG" --developer 2>&1 | grep -oE "0x[0-9a-f]{64}" | head -1 || echo "")
    
    if [ -n "$TX_HASH" ]; then
        echo -e "${GREEN}✓ Transaction submitted: $TX_HASH${NC}"
        echo ""
        echo "Waiting for transaction to be mined..."
        sleep 5
        
        # Check new state
        NEW_STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
          --config "$MAIN_CONFIG" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")
        NEW_STATE_NAME=$(get_state_name "$NEW_STATE")
        echo ""
        echo "New DKG State: $NEW_STATE ($NEW_STATE_NAME)"
        echo ""
    else
        echo -e "${RED}⚠ Error: Failed to submit transaction${NC}"
        exit 1
    fi
else
    echo "Step 5: Skipping wallet request (DKG not IDLE)"
    echo "----------------------------------------------"
fi

# Step 6: Monitor DKG
echo ""
echo "=========================================="
echo "DKG Monitoring"
echo "=========================================="
echo ""
echo "DKG has been triggered!"
echo ""
echo "Monitor progress with:"
echo "  ./scripts/monitor-dkg.sh"
echo "  ./scripts/check-dkg-state.sh"
echo "  tail -f logs/node*.log | grep -i dkg"
echo ""
echo "Check metric:"
echo "  curl http://localhost:9601/metrics | grep performance_dkg_requested_total"
echo ""
echo "Expected flow:"
echo "  1. AWAITING_SEED - Waiting for Random Beacon relay entry"
echo "  2. AWAITING_RESULT - Operators generating keys (~9 minutes)"
echo "  3. CHALLENGE - Result submitted, in challenge period"
echo "  4. IDLE - DKG complete, wallet created"
echo ""

#!/bin/bash
# Script to request a new wallet and trigger DKG
# Usage: ./scripts/request-new-wallet.sh [config-file]
#        If config-file not provided, uses config.toml

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

CONFIG_FILE=${1:-"config.toml"}

echo "=========================================="
echo "Request New Wallet & Trigger DKG"
echo "=========================================="
echo ""

# Step 1: Check wallet owner
echo -e "${BLUE}Step 1: Checking wallet owner...${NC}"
WALLET_OWNER=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallet-owner \
  --config "$CONFIG_FILE" --developer 2>&1 | grep -oE "0x[0-9a-fA-F]{40}" | tail -1 || echo "")

if [ -z "$WALLET_OWNER" ] || [ "$WALLET_OWNER" = "0x0000000000000000000000000000000000000000" ]; then
    echo -e "${RED}✗ Wallet owner is not set!${NC}"
    echo ""
    echo "You need to set a wallet owner first. Options:"
    echo ""
    echo "Option 1: Use Hardhat task (recommended for local dev):"
    echo "  cd solidity/ecdsa"
    echo "  npx hardhat initialize-wallet-owner --wallet-owner <address> --network development"
    echo ""
    echo "Option 2: Use existing script:"
    echo "  ./scripts/initialize-wallet-owner.sh <wallet-owner-address>"
    echo ""
    echo "Option 3: Use operator address as wallet owner:"
    echo "  OPERATOR=\$(curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address')"
    echo "  cd solidity/ecdsa"
    echo "  npx hardhat initialize-wallet-owner --wallet-owner \$OPERATOR --network development"
    echo ""
    exit 1
fi

echo -e "${GREEN}✓ Wallet owner: $WALLET_OWNER${NC}"
echo ""

# Step 2: Check if config uses wallet owner's keyfile
echo -e "${BLUE}Step 2: Verifying config uses wallet owner's keyfile...${NC}"
CONFIG_KEYFILE=$(grep "^KeyFile" "$CONFIG_FILE" 2>/dev/null | head -1 | cut -d'=' -f2 | tr -d ' "' || echo "")

if [ -z "$CONFIG_KEYFILE" ]; then
    echo -e "${RED}✗ KeyFile not found in config${NC}"
    exit 1
fi

# Extract address from keyfile
CONFIG_ADDRESS=$(cat "$CONFIG_KEYFILE" 2>/dev/null | jq -r '.address' 2>/dev/null || echo "")
if [ -z "$CONFIG_ADDRESS" ] || [[ "$CONFIG_ADDRESS" != 0x* ]]; then
    FILENAME=$(basename "$CONFIG_KEYFILE")
    CONFIG_ADDRESS=$(echo "$FILENAME" | sed -E 's/.*--([0-9a-fA-F]{40})$/\1/' | sed 's/^/0x/' | tr '[:upper:]' '[:lower:]')
fi

CONFIG_ADDRESS=$(echo "$CONFIG_ADDRESS" | tr '[:upper:]' '[:lower:]')
WALLET_OWNER_LOWER=$(echo "$WALLET_OWNER" | tr '[:upper:]' '[:lower:]')

if [ "$CONFIG_ADDRESS" != "$WALLET_OWNER_LOWER" ]; then
    echo -e "${YELLOW}⚠ Warning: Config KeyFile address ($CONFIG_ADDRESS) doesn't match wallet owner ($WALLET_OWNER)${NC}"
    echo ""
    echo "The transaction must be sent from the wallet owner address."
    echo "Update your config file to use the wallet owner's keyfile, or"
    echo "set the wallet owner to match your current operator address."
    echo ""
    read -p "Continue anyway? (y/n): " confirm
    if [ "$confirm" != "y" ]; then
        exit 1
    fi
else
    echo -e "${GREEN}✓ Config uses wallet owner's keyfile${NC}"
fi
echo ""

# Step 3: Check current DKG state
echo -e "${BLUE}Step 3: Checking current DKG state...${NC}"
CURRENT_STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")

get_state_name() {
    case "$1" in
        0) echo "IDLE" ;;
        1) echo "AWAITING_SEED" ;;
        2) echo "AWAITING_RESULT" ;;
        3) echo "CHALLENGE" ;;
        *) echo "UNKNOWN" ;;
    esac
}

STATE_NAME=$(get_state_name "$CURRENT_STATE")
echo "Current DKG State: $CURRENT_STATE ($STATE_NAME)"
echo ""

if [ "$CURRENT_STATE" != "0" ] && [ "$CURRENT_STATE" != "" ]; then
    echo -e "${YELLOW}⚠ DKG is not in IDLE state${NC}"
    echo ""
    echo "DKG must be in IDLE state to request a new wallet."
    echo ""
    echo "Options:"
    echo "  1. Wait for current DKG to complete"
    echo "  2. Reset DKG if stuck (use scripts/check-and-reset-dkg.sh)"
    echo "  3. Use scripts/rerun-dkg-complete.sh which handles this automatically"
    echo ""
    read -p "Continue anyway? (y/n): " confirm
    if [ "$confirm" != "y" ]; then
        exit 1
    fi
fi

# Step 4: Check operators in pool
echo -e "${BLUE}Step 4: Checking operators in sortition pool...${NC}"
OPERATORS_IN_POOL=0

for i in {1..10}; do
    NODE_CONFIG="configs/node${i}.toml"
    if [ ! -f "$NODE_CONFIG" ]; then
        continue
    fi
    
    KEYFILE=$(grep "^KeyFile" "$NODE_CONFIG" 2>/dev/null | head -1 | cut -d'=' -f2 | tr -d ' "' || echo "")
    if [ -z "$KEYFILE" ]; then
        continue
    fi
    
    OPERATOR=$(cat "$KEYFILE" 2>/dev/null | jq -r '.address' 2>/dev/null || echo "")
    if [ -z "$OPERATOR" ] || [[ "$OPERATOR" != 0x* ]]; then
        FILENAME=$(basename "$KEYFILE")
        OPERATOR=$(echo "$FILENAME" | sed -E 's/.*--([0-9a-fA-F]{40})$/\1/' | sed 's/^/0x/' | tr '[:upper:]' '[:lower:]')
    fi
    
    OPERATOR=$(echo "$OPERATOR" | tr '[:upper:]' '[:lower:]')
    
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
      --config "$CONFIG_FILE" --developer 2>&1 | tail -1 | grep -iE "true" || echo "false")
    
    if [ "$IN_POOL" = "true" ]; then
        OPERATORS_IN_POOL=$((OPERATORS_IN_POOL + 1))
    fi
done

echo "Operators in pool: $OPERATORS_IN_POOL"
echo ""

if [ "$OPERATORS_IN_POOL" -lt 3 ]; then
    echo -e "${RED}✗ Error: Need at least 3 operators in pool for DKG${NC}"
    echo ""
    echo "Register operators with:"
    echo "  ./scripts/register-single-operator.sh <node-number>"
    exit 1
fi

echo -e "${GREEN}✓ Sufficient operators in pool ($OPERATORS_IN_POOL)${NC}"
echo ""

# Step 5: Request new wallet
echo -e "${BLUE}Step 5: Requesting new wallet...${NC}"
echo ""
echo "This will:"
echo "  1. Lock the DKG state"
echo "  2. Request a relay entry from RandomBeacon"
echo "  3. Start DKG automatically when relay entry is generated"
echo ""

read -p "Continue? (y/n): " confirm
if [ "$confirm" != "y" ]; then
    echo "Aborted."
    exit 0
fi

echo ""
echo "Submitting request-new-wallet transaction..."
TX_HASH=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config "$CONFIG_FILE" --developer 2>&1 | grep -oE "0x[0-9a-f]{64}" | head -1 || echo "")

if [ -z "$TX_HASH" ]; then
    echo -e "${RED}✗ Error: Failed to submit transaction${NC}"
    echo ""
    echo "Check:"
    echo "  - Wallet owner is set correctly"
    echo "  - Config uses wallet owner's keyfile"
    echo "  - Account has sufficient ETH for gas"
    echo "  - DKG is in IDLE state"
    exit 1
fi

echo -e "${GREEN}✓ Transaction submitted: $TX_HASH${NC}"
echo ""
echo "Waiting for transaction to be mined..."
sleep 5

# Step 6: Verify DKG started
echo ""
echo -e "${BLUE}Step 6: Verifying DKG state...${NC}"
NEW_STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG_FILE" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1 || echo "")

NEW_STATE_NAME=$(get_state_name "$NEW_STATE")
echo "New DKG State: $NEW_STATE ($NEW_STATE_NAME)"
echo ""

if [ "$NEW_STATE" = "1" ]; then
    echo -e "${GREEN}✓ DKG started successfully!${NC}"
    echo ""
    echo "DKG is now in AWAITING_SEED state."
    echo "RandomBeacon will generate a relay entry, which will trigger DKG."
    echo ""
    echo "Monitor DKG progress:"
    echo "  ./scripts/check-node-dkg-joined.sh"
    echo "  ./scripts/check-dkg-metrics.sh"
    echo "  tail -f logs/node*.log | grep -i dkg"
elif [ "$NEW_STATE" = "0" ]; then
    echo -e "${YELLOW}⚠ DKG state is still IDLE${NC}"
    echo "Transaction may still be processing, or relay entry generation may be pending."
    echo "Check transaction receipt:"
    echo "  ./scripts/check-transaction-receipt.sh $TX_HASH"
else
    echo -e "${YELLOW}⚠ Unexpected DKG state: $NEW_STATE_NAME${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Request complete${NC}"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Wait for RandomBeacon to generate relay entry"
echo "  2. Nodes will automatically join DKG when eligible"
echo "  3. Monitor progress with the scripts above"

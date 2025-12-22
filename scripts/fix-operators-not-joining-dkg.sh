#!/bin/bash
# Script to fix operators not joining DKG
# This script automates all the troubleshooting steps from the guide
# Usage: ./scripts/fix-operators-not-joining-dkg.sh [node-number]
#        If no node-number provided, fixes all nodes

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

MAIN_CONFIG="configs/config.toml"
RANDOM_BEACON="0x18266866EbBab6cA7f5F2724e22CEF54a98Cda92"
WALLET_REGISTRY="0xbd49D2e3E501918CD08Eb4cCa34984F428c83464"
MIN_AUTHORIZATION="0x878678326eac9000000"  # 40k T tokens

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Function to clean and validate addresses
clean_address() {
    local addr="$1"
    addr=$(printf '%s' "$addr" | tr -d '[:space:]\n\r' | tr -cd '0-9a-fA-Fx' | sed 's/^x/0x/' | sed 's/^\([^0]\)/0x\1/')
    if [[ "$addr" != 0x* ]]; then
        addr="0x$addr"
    fi
    addr=$(echo "$addr" | tr '[:upper:]' '[:lower:]')
    addr=$(printf '%.42s' "$addr")
    echo "$addr"
}

# Function to get operator address from config
get_operator_from_config() {
    local node_num=$1
    local node_config="configs/node${node_num}.toml"
    
    if [ ! -f "$node_config" ]; then
        return 1
    fi
    
    local keyfile=$(grep "^KeyFile" "$node_config" 2>/dev/null | head -1 | cut -d'=' -f2 | tr -d ' "' | head -1)
    if [ -z "$keyfile" ]; then
        return 1
    fi
    
    # Resolve relative path
    if [[ "$keyfile" == ./* ]] || [[ "$keyfile" != /* ]]; then
        keyfile="$(cd "$(dirname "$node_config")/.." && pwd)/${keyfile#./}"
    fi
    
    if [ ! -f "$keyfile" ]; then
        return 1
    fi
    
    local operator=$(cat "$keyfile" 2>/dev/null | jq -r '.address' 2>/dev/null | head -1 | tr -d '\n\r' || echo "")
    if [ -z "$operator" ] || [[ "$operator" != 0x* ]] || [ ${#operator} -ne 42 ]; then
        local filename=$(basename "$keyfile")
        operator=$(echo "$filename" | sed -E 's/.*--([0-9a-fA-F]{40})$/\1/' | tr '[:upper:]' '[:lower:]' | sed 's/^/0x/' || echo "")
    fi
    
    operator=$(clean_address "$operator")
    echo "$operator"
}

# Function to get staking provider from mapping
get_staking_provider() {
    local operator="$1"
    local mapping_file="keystore/staking-provider-mapping.txt"
    
    if [ ! -f "$mapping_file" ]; then
        return 1
    fi
    
    local staking_provider=$(grep "^${operator}=" "$mapping_file" 2>/dev/null | cut -d'=' -f2 | tr -d '[:space:]\n\r' || echo "")
    if [ -z "$staking_provider" ]; then
        return 1
    fi
    
    staking_provider=$(clean_address "$staking_provider")
    echo "$staking_provider"
}

# Function to check if operator is in pool
check_in_pool() {
    local operator="$1"
    local pool_type="$2"  # "beacon" or "ecdsa"
    
    if [ "$pool_type" = "beacon" ]; then
        local result=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon is-operator-in-pool "$operator" \
          --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -iE "true" || echo "false")
    else
        local result=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$operator" \
          --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -iE "true" || echo "false")
    fi
    
    [ "$result" = "true" ]
}

# Function to check authorization for an application
check_authorization() {
    local staking_provider="$1"
    local application="$2"  # RandomBeacon or WalletRegistry address
    
    # Check authorization using the application-specific command
    local output=""
    if [ "$application" = "$RANDOM_BEACON" ]; then
        output=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon eligible-stake \
          "$staking_provider" --config "$MAIN_CONFIG" --developer 2>&1 | tail -1)
    else
        output=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry eligible-stake \
          "$staking_provider" --config "$MAIN_CONFIG" --developer 2>&1 | tail -1)
    fi
    
    # Parse the output - it might be "+0", "0x0", or a hex number
    local auth=$(echo "$output" | grep -oE "(0x[0-9a-fA-F]+|\+[0-9]+|[0-9]+)" | head -1 || echo "0")
    
    # Remove + prefix if present
    auth=$(echo "$auth" | sed 's/^+//')
    
    # Convert to decimal for comparison (if it's hex, convert it)
    if [[ "$auth" == 0x* ]]; then
        # It's hex, check if it's non-zero
        [ "$auth" != "0x0" ] && [ "$auth" != "0x0000" ] && [ "$auth" != "0x0000000000000000000000000000000000000000" ]
    else
        # It's decimal, check if greater than 0
        [ "$auth" != "0" ] && [ -n "$auth" ]
    fi
}

# Function to check if operator is beta operator
check_beta_operator() {
    local operator="$1"
    local pool_type="$2"  # "beacon" or "ecdsa"
    
    cd solidity/$pool_type 2>/dev/null || return 1
    
    # Capitalize first letter for contract name (bash-compatible)
    local pool_name=$(echo "$pool_type" | awk '{print toupper(substr($0,1,1)) substr($0,2)}')
    
    local result=$(npx hardhat console --network development 2>&1 <<EOF | grep -iE "(true|false)" | head -1 || echo "false"
const { helpers } = require("hardhat");
(async () => {
  try {
    const pool = await helpers.contracts.getContract("${pool_name}SortitionPool");
    const isBeta = await pool.isBetaOperator("$operator");
    console.log(isBeta);
    process.exit(0);
  } catch (error) {
    console.error("false");
    process.exit(1);
  }
})();
EOF
)
    
    cd "$PROJECT_ROOT"
    [ "$result" = "true" ]
}

# Function to add operator as beta operator
add_beta_operator() {
    local operator="$1"
    local pool_type="$2"  # "beacon" or "ecdsa"
    
    # Capitalize first letter (bash-compatible)
    local pool_name=$(echo "$pool_type" | awk '{print toupper(substr($0,1,1)) substr($0,2)}')
    
    echo -e "${YELLOW}  Adding as beta operator for ${pool_name}...${NC}"
    cd solidity/$pool_type 2>/dev/null || return 1
    
    if [ "$pool_type" = "beacon" ]; then
        npx hardhat add_beta_operator:beacon --operator "$operator" --network development 2>&1 | grep -E "(Adding|Transaction|hash)" || true
    else
        npx hardhat add_beta_operator:ecdsa --operator "$operator" --network development 2>&1 | grep -E "(Adding|Transaction|hash)" || true
    fi
    
    cd "$PROJECT_ROOT"
    sleep 2
}

# Function to check if chaosnet is active
is_chaosnet_active() {
    cd solidity/ecdsa 2>/dev/null || return 1
    
    local result=$(npx hardhat console --network development 2>&1 <<EOF | grep -iE "(true|false)" | head -1 || echo "false"
const { helpers } = require("hardhat");
(async () => {
  try {
    const pool = await helpers.contracts.getContract("EcdsaSortitionPool");
    const isActive = await pool.isChaosnetActive();
    console.log(isActive);
    process.exit(0);
  } catch (error) {
    console.error("false");
    process.exit(1);
  }
})();
EOF
)
    
    cd "$PROJECT_ROOT"
    [ "$result" = "true" ]
}

# Function to fix a single node
fix_node() {
    local node_num=$1
    
    echo ""
    echo "=========================================="
    echo "Fixing Node $node_num"
    echo "=========================================="
    
    # Get operator address
    local operator=$(get_operator_from_config "$node_num")
    if [ -z "$operator" ] || [ ${#operator} -ne 42 ]; then
        echo -e "${RED}✗ Could not get operator address for Node $node_num${NC}"
        return 1
    fi
    
    echo "Operator: $operator"
    
    # Get staking provider
    local staking_provider=$(get_staking_provider "$operator")
    if [ -z "$staking_provider" ]; then
        echo -e "${RED}✗ Could not find staking provider for operator $operator${NC}"
        echo "  Add mapping to keystore/staking-provider-mapping.txt"
        return 1
    fi
    
    echo "Staking Provider: $staking_provider"
    echo ""
    
    local fixes_applied=0
    
    # 1. Check if in sortition pools
    echo "1. Checking sortition pool status..."
    local rb_in_pool=false
    local wr_in_pool=false
    local needs_registration=false
    
    if check_in_pool "$operator" "beacon"; then
        echo -e "  ${GREEN}✓${NC} In RandomBeacon pool"
        rb_in_pool=true
    else
        echo -e "  ${YELLOW}✗${NC} NOT in RandomBeacon pool"
        needs_registration=true
    fi
    
    if check_in_pool "$operator" "ecdsa"; then
        echo -e "  ${GREEN}✓${NC} In WalletRegistry pool"
        wr_in_pool=true
    else
        echo -e "  ${YELLOW}✗${NC} NOT in WalletRegistry pool"
        needs_registration=true
    fi
    
    if [ "$needs_registration" = true ]; then
        echo "  → Registering operator and joining pools..."
        ./scripts/register-single-operator.sh "$node_num" 2>&1 | grep -E "(✓|transaction|hash|already|registered|pool)" || true
        fixes_applied=$((fixes_applied + 1))
        sleep 3  # Wait for transactions to be mined
    fi
    
    # 2. Check authorization
    echo ""
    echo "2. Checking authorization amounts..."
    
    # Get staking provider keyfile for authorization commands
    local staking_provider_lower=$(echo "$staking_provider" | tr '[:upper:]' '[:lower:]')
    local staking_provider_hex=${staking_provider_lower#0x}
    local staking_provider_keyfile=$(ls keystore/staking-providers/*${staking_provider_hex}* 2>/dev/null | head -1)
    
    if [ -z "$staking_provider_keyfile" ]; then
        echo -e "  ${YELLOW}⚠${NC} Could not find staking provider keyfile - skipping authorization check"
        echo "  (Authorization may need to be done manually)"
    else
        # Resolve absolute path
        if [[ "$staking_provider_keyfile" == ./* ]] || [[ "$staking_provider_keyfile" != /* ]]; then
            staking_provider_keyfile="$(cd "$(dirname "$staking_provider_keyfile")" && pwd)/$(basename "$staking_provider_keyfile")"
        fi
        
        # Create temp config with staking provider's keyfile (macOS-compatible)
        local temp_config=$(mktemp "${TMPDIR:-/tmp}/keep-config-XXXXXX.toml")
        cp "$MAIN_CONFIG" "$temp_config"
        # macOS sed requires backup extension, but we'll remove it after
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s|KeyFile = .*|KeyFile = \"$staking_provider_keyfile\"|" "$temp_config"
        else
            sed -i.bak "s|KeyFile = .*|KeyFile = \"$staking_provider_keyfile\"|" "$temp_config"
            rm -f "${temp_config}.bak"
        fi
        
        if check_authorization "$staking_provider" "$RANDOM_BEACON"; then
            echo -e "  ${GREEN}✓${NC} RandomBeacon authorization sufficient"
        else
            echo -e "  ${YELLOW}✗${NC} RandomBeacon authorization insufficient"
            echo "  → Increasing authorization..."
            KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
              "$staking_provider" "$RANDOM_BEACON" "$MIN_AUTHORIZATION" \
              --submit --config "$temp_config" --developer 2>&1 | grep -E "(transaction|hash|0x[0-9a-f]{64})" || echo "  (May already be authorized)"
            fixes_applied=$((fixes_applied + 1))
            sleep 2
        fi
        
        if check_authorization "$staking_provider" "$WALLET_REGISTRY"; then
            echo -e "  ${GREEN}✓${NC} WalletRegistry authorization sufficient"
        else
            echo -e "  ${YELLOW}✗${NC} WalletRegistry authorization insufficient"
            echo "  → Increasing authorization..."
            KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
              "$staking_provider" "$WALLET_REGISTRY" "$MIN_AUTHORIZATION" \
              --submit --config "$temp_config" --developer 2>&1 | grep -E "(transaction|hash|0x[0-9a-f]{64})" || echo "  (May already be authorized)"
            fixes_applied=$((fixes_applied + 1))
            sleep 2
        fi
        
        rm -f "$temp_config" "${temp_config}.bak" 2>/dev/null || true
    fi
    
    # 3. Check beta operator status (if chaosnet is active)
    echo ""
    echo "3. Checking beta operator status..."
    
    if is_chaosnet_active; then
        echo "  Chaosnet is active - checking beta operator status..."
        
        if check_beta_operator "$operator" "beacon"; then
            echo -e "  ${GREEN}✓${NC} Beta operator for RandomBeacon"
        else
            echo -e "  ${YELLOW}✗${NC} NOT beta operator for RandomBeacon"
            add_beta_operator "$operator" "beacon"
            fixes_applied=$((fixes_applied + 1))
        fi
        
        if check_beta_operator "$operator" "ecdsa"; then
            echo -e "  ${GREEN}✓${NC} Beta operator for WalletRegistry"
        else
            echo -e "  ${YELLOW}✗${NC} NOT beta operator for WalletRegistry"
            add_beta_operator "$operator" "ecdsa"
            fixes_applied=$((fixes_applied + 1))
        fi
    else
        echo "  Chaosnet is not active - beta operator not required"
    fi
    
    # 4. Check registration
    echo ""
    echo "4. Checking operator registration..."
    
    local rb_registered=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon operator-to-staking-provider "$operator" \
      --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -oE "0x[0-9a-fA-F]{40}" || echo "0x0000")
    
    local wr_registered=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry operator-to-staking-provider "$operator" \
      --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -oE "0x[0-9a-fA-F]{40}" || echo "0x0000")
    
    if [ "$rb_registered" != "0x0000" ] && [ "$rb_registered" != "0x0000000000000000000000000000000000000000" ]; then
        echo -e "  ${GREEN}✓${NC} Registered in RandomBeacon"
    else
        echo -e "  ${YELLOW}✗${NC} NOT registered in RandomBeacon"
        echo "  → Registering operator..."
        ./scripts/register-single-operator.sh "$node_num" 2>&1 | grep -E "(✓|transaction|hash|already)" || true
        fixes_applied=$((fixes_applied + 1))
    fi
    
    if [ "$wr_registered" != "0x0000" ] && [ "$wr_registered" != "0x0000000000000000000000000000000000000000" ]; then
        echo -e "  ${GREEN}✓${NC} Registered in WalletRegistry"
    else
        echo -e "  ${YELLOW}✗${NC} NOT registered in WalletRegistry"
        if [ "$rb_registered" = "0x0000" ]; then
            echo "  → Registering operator..."
            ./scripts/register-single-operator.sh "$node_num" 2>&1 | grep -E "(✓|transaction|hash|already)" || true
            fixes_applied=$((fixes_applied + 1))
        fi
    fi
    
    # Summary
    echo ""
    if [ $fixes_applied -eq 0 ]; then
        echo -e "${GREEN}✓ Node $node_num is properly configured!${NC}"
    else
        echo -e "${YELLOW}⚠ Applied $fixes_applied fixes for Node $node_num${NC}"
        echo "  Wait a few seconds for transactions to be mined, then verify:"
        echo "    ./scripts/test-nodes-in-pool.sh"
    fi
}

# Main execution
echo "=========================================="
echo "Fix Operators Not Joining DKG"
echo "=========================================="
echo ""

# Check if specific node number provided
if [ $# -ge 1 ] && [ "$1" != "" ]; then
    NODE_NUM=$1
    if [ ! -f "configs/node${NODE_NUM}.toml" ]; then
        echo -e "${RED}Error: Config file not found for Node $NODE_NUM${NC}"
        exit 1
    fi
    fix_node "$NODE_NUM"
else
    # Fix all nodes
    echo "Fixing all nodes..."
    echo ""
    
    for i in {1..10}; do
        if [ -f "configs/node${i}.toml" ]; then
            fix_node "$i"
        fi
    done
    
    echo ""
    echo "=========================================="
    echo "Summary"
    echo "=========================================="
    echo ""
    echo "All nodes have been checked and fixed."
    echo ""
    echo "Next steps:"
    echo "  1. Wait for transactions to be mined (~30 seconds)"
    echo "  2. Verify operators are in pools:"
    echo "     ./scripts/test-nodes-in-pool.sh"
    echo "  3. Check DKG state:"
    echo "     ./scripts/check-dkg-state.sh"
    echo "  4. If DKG is IDLE, request new wallet:"
    echo "     ./scripts/rerun-dkg-complete.sh"
    echo ""
fi

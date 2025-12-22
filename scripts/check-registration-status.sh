#!/bin/bash
# Script to check operator registration status
# Usage: ./scripts/check-registration-status.sh [num-nodes] [config-dir]

set -u

NUM_NODES=${1:-5}
CONFIG_DIR=${2:-./configs}
MAIN_CONFIG=${3:-configs/config.toml}

echo "=========================================="
echo "Checking Operator Registration Status"
echo "=========================================="
echo ""

# Get contract addresses
WALLET_REGISTRY=$(grep -A 10 "\[developer\]" "$MAIN_CONFIG" | grep "WalletRegistryAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
TOKEN_STAKING=$(grep -A 10 "\[developer\]" "$MAIN_CONFIG" | grep "TokenStakingAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")

if [ -z "$WALLET_REGISTRY" ] || [ -z "$TOKEN_STAKING" ]; then
    echo "⚠ Error: Could not find contract addresses in $MAIN_CONFIG"
    exit 1
fi

echo "WalletRegistry: $WALLET_REGISTRY"
echo "TokenStaking: $TOKEN_STAKING"
echo ""

# Extract operator addresses from config files
declare -a OPERATOR_ADDRESSES
for i in $(seq 1 $NUM_NODES); do
    NODE_CONFIG="$CONFIG_DIR/node${i}.toml"
    if [ -f "$NODE_CONFIG" ]; then
        KEYFILE=$(grep -i "^KeyFile" "$NODE_CONFIG" | head -1 | awk -F'=' '{print $2}' | tr -d ' "')
        if [ -n "$KEYFILE" ]; then
            # Resolve keyfile path
            if [[ "$KEYFILE" == ./* ]]; then
                KEYFILE="${KEYFILE#./}"
                KEYFILE="$(cd "$(dirname "$NODE_CONFIG")/.." && pwd)/$KEYFILE"
            fi
            
            if [ -f "$KEYFILE" ]; then
                OPERATOR=$(cat "$KEYFILE" | jq -r '.address' 2>/dev/null || echo "")
                if [ -n "$OPERATOR" ] && [[ "$OPERATOR" == 0x* ]]; then
                    OPERATOR_ADDRESSES[$i]="$OPERATOR"
                fi
            fi
        fi
    fi
done

echo "Checking registration status for $NUM_NODES operators..."
echo ""

for i in $(seq 1 $NUM_NODES); do
    OPERATOR="${OPERATOR_ADDRESSES[$i]}"
    if [ -z "$OPERATOR" ]; then
        echo "⚠ Node $i: Could not extract operator address"
        continue
    fi
    
    echo "Node $i: $OPERATOR"
    
    # Check if operator is in pool
    echo -n "  - In pool: "
    IN_POOL=$(./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
      --operator "$OPERATOR" \
      --config "$MAIN_CONFIG" \
      --developer 2>&1 | grep -iE "(true|false)" | head -1 || echo "unknown")
    echo "$IN_POOL"
    
    # Check staking provider
    echo -n "  - Staking provider: "
    STAKING_PROVIDER=$(./keep-client ethereum threshold token-staking staking-provider \
      --operator "$OPERATOR" \
      --config "$MAIN_CONFIG" \
      --developer 2>&1 | grep -oE "0x[a-fA-F0-9]{40}" | head -1 || echo "unknown")
    echo "$STAKING_PROVIDER"
    
    # Check authorized stake
    if [ "$STAKING_PROVIDER" != "unknown" ] && [[ "$STAKING_PROVIDER" == 0x* ]]; then
        echo -n "  - Authorized stake: "
        AUTHORIZED=$(./keep-client ethereum threshold token-staking authorized-stake \
          --staking-provider "$STAKING_PROVIDER" \
          --application "$WALLET_REGISTRY" \
          --config "$MAIN_CONFIG" \
          --developer 2>&1 | grep -oE "[0-9]+" | head -1 || echo "0")
        echo "$AUTHORIZED"
    fi
    
    echo ""
done

echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo "If operators show 'false' for 'In pool', they need to be registered."
echo "Run: ./scripts/register-operators.sh"
echo ""


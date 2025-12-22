#!/bin/bash
# Script to fix common node startup issues
# Usage: ./scripts/fix-node-startup.sh

set -u

CONFIG_DIR=${1:-./configs}

echo "=========================================="
echo "Fixing Node Startup Issues"
echo "=========================================="
echo ""

# Fix 1: Remove invalid peer entries
echo "1. Fixing invalid peer configurations..."
FIXED=0
for config in "$CONFIG_DIR"/node*.toml; do
    if grep -q 'Peers = \["/ip4/127.0.0.1/tcp/3919/ipfs"\]' "$config"; then
        echo "  Fixing: $(basename $config)"
        sed -i '' 's|Peers = \["/ip4/127.0.0.1/tcp/3919/ipfs"\]|Peers = []|g' "$config"
        FIXED=$((FIXED + 1))
    fi
done
echo "  Fixed $FIXED config files"
echo ""

# Fix 2: Check operator registration
echo "2. Checking operator registration status..."
MAIN_CONFIG="$CONFIG_DIR/config.toml"
if [ ! -f "$MAIN_CONFIG" ]; then
    echo "  ⚠ Warning: Main config not found: $MAIN_CONFIG"
else
    WALLET_REGISTRY=$(grep -A 10 "\[developer\]" "$MAIN_CONFIG" | grep "WalletRegistryAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
    
    if [ -n "$WALLET_REGISTRY" ]; then
        UNREGISTERED=0
        for i in {1..10}; do
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
                            IS_REGISTERED=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
                              --config "$MAIN_CONFIG" --developer 2>&1 | grep -iE "(true|false)" | head -1 || echo "unknown")
                            
                            if [ "$IS_REGISTERED" != "true" ]; then
                                echo "  ⚠ Node $i ($OPERATOR): NOT REGISTERED"
                                UNREGISTERED=$((UNREGISTERED + 1))
                            fi
                        fi
                    fi
                fi
            fi
        done
        
        if [ $UNREGISTERED -gt 0 ]; then
            echo ""
            echo "  ⚠ Found $UNREGISTERED unregistered operators"
            echo "  Run: ./scripts/register-operators.sh"
        else
            echo "  ✓ All operators are registered"
        fi
    fi
fi
echo ""

# Fix 3: Clean up old PID files
echo "3. Cleaning up old PID files..."
rm -f logs/*.pid
echo "  ✓ Cleaned up PID files"
echo ""

echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo "Fixed issues:"
echo "  ✓ Invalid peer configurations"
echo "  ✓ Old PID files"
echo ""
echo "Next steps:"
echo "  1. Register operators (if needed):"
echo "     ./scripts/register-operators.sh"
echo ""
echo "  2. Start nodes:"
echo "     ./configs/start-all-nodes.sh"
echo ""
echo "  3. Check status:"
echo "     ./configs/check-nodes.sh"
echo ""


#!/bin/bash
set -eou pipefail

# Script to update peer IDs in node configs after nodes start
# 
# Usage:
#   ./scripts/update-peer-ids.sh [config-dir] [base-diagnostics-port]

CONFIG_DIR=${1:-./configs}
BASE_DIAG_PORT=${2:-9601}

echo "=========================================="
echo "Update Peer IDs in Node Configs"
echo "=========================================="
echo ""

NUM_NODES=$(ls -1 "$CONFIG_DIR"/node*.toml 2>/dev/null | wc -l | tr -d ' ')

if [ "$NUM_NODES" -eq 0 ]; then
    echo "⚠ No node configs found in $CONFIG_DIR"
    exit 1
fi

echo "Found $NUM_NODES node configs"
echo ""

# Extract peer IDs from running nodes
declare -a PEER_IDS
declare -a NODE_PORTS

for i in $(seq 1 $NUM_NODES); do
    CONFIG_FILE="$CONFIG_DIR/node${i}.toml"
    
    # Extract diagnostics port from config
    DIAG_PORT=$(awk '/\[clientinfo\]/{flag=1; next} /^\[/{flag=0} flag && /^Port/{print $3; exit}' "$CONFIG_FILE" 2>/dev/null | tr -d ' "' || echo "")
    
    # Fallback to calculated port
    if [ -z "$DIAG_PORT" ]; then
        DIAG_PORT=$((BASE_DIAG_PORT + i - 1))
    fi
    
    NODE_PORTS[$i]=$DIAG_PORT
    
    # Try to get peer ID from diagnostics (network_id in client_info)
    PEER_ID=$(curl -s "http://localhost:${DIAG_PORT}/diagnostics" 2>/dev/null | \
        jq -r '.client_info.network_id // empty' 2>/dev/null || echo "")
    
    if [ -z "$PEER_ID" ]; then
        # Fallback: try grep method
        PEER_ID=$(curl -s "http://localhost:${DIAG_PORT}/diagnostics" 2>/dev/null | \
            grep -o '"network_id":"[^"]*"' | cut -d'"' -f4 || echo "")
    fi
    
    if [ -z "$PEER_ID" ]; then
        # Try to get from logs
        if [ -f "logs/node${i}.log" ]; then
            PEER_ID=$(grep -oE "ipfs/[a-zA-Z0-9]{52}" "logs/node${i}.log" | head -1 | sed 's/ipfs\///' || echo "")
        fi
    fi
    
    if [ -n "$PEER_ID" ]; then
        PEER_IDS[$i]="$PEER_ID"
        echo "✓ Node $i: $PEER_ID (Port: $DIAG_PORT)"
    else
        echo "⚠ Node $i: Could not get peer ID (Port: $DIAG_PORT, node may not be running)"
        PEER_IDS[$i]=""
    fi
done

echo ""
echo "Updating config files..."
echo ""

# Update peer IDs in configs
for i in $(seq 2 $NUM_NODES); do
    CONFIG_FILE="$CONFIG_DIR/node${i}.toml"
    if [ ! -f "$CONFIG_FILE" ]; then
        continue
    fi
    
    # Extract network port from config
    NETWORK_PORT=$(awk '/\[network\]/{flag=1; next} /^\[/{flag=0} flag && /^Port/{print $3; exit}' "$CONFIG_FILE" 2>/dev/null | tr -d ' "' || echo "$((3918 + i))")
    
    # Build new peers list
    NEW_PEERS=""
    for j in $(seq 1 $((i-1))); do
        PEER_ID="${PEER_IDS[$j]}"
        if [ -z "$PEER_ID" ]; then
            continue
        fi
        
        # Get network port for previous node
        PREV_CONFIG="$CONFIG_DIR/node${j}.toml"
        PREV_NETWORK_PORT=$(awk '/\[network\]/{flag=1; next} /^\[/{flag=0} flag && /^Port/{print $3; exit}' "$PREV_CONFIG" 2>/dev/null | tr -d ' "' || echo "$((3918 + j))")
        
        PEER_ENTRY="/ip4/127.0.0.1/tcp/${PREV_NETWORK_PORT}/ipfs/${PEER_ID}"
        
        if [ -z "$NEW_PEERS" ]; then
            NEW_PEERS="\"${PEER_ENTRY}\""
        else
            NEW_PEERS="${NEW_PEERS}, \"${PEER_ENTRY}\""
        fi
    done
    
    if [ -n "$NEW_PEERS" ]; then
        # Update config file
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # macOS sed
            sed -i '' "s|Peers = \[.*\]|Peers = [${NEW_PEERS}]|" "$CONFIG_FILE"
        else
            # Linux sed
            sed -i "s|Peers = \[.*\]|Peers = [${NEW_PEERS}]|" "$CONFIG_FILE"
        fi
        echo "✓ Updated $CONFIG_FILE"
    else
        echo "⚠ Could not update $CONFIG_FILE (no peer IDs available)"
    fi
done

echo ""
echo "✓ Peer IDs updated!"
echo ""
echo "Note: You may need to restart nodes for peer connections to work."
echo ""

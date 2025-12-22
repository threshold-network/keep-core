#!/bin/bash
set -eou pipefail

# Script to automate multi-node DKG setup for local testing
# 
# Usage:
#   ./scripts/setup-multi-node-dkg.sh [num-nodes] [base-port] [base-diagnostics-port]
#
# Example:
#   ./scripts/setup-multi-node-dkg.sh 10 3919 9601

NUM_NODES=${1:-5}
BASE_PORT=${2:-3919}
BASE_DIAG_PORT=${3:-9601}
BASE_STORAGE_DIR="${4:-./storage}"
CONFIG_DIR="${5:-./configs}"
KEYSTORE_DIR="${6:-./keystore}"

echo "=========================================="
echo "Multi-Node DKG Setup Automation"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  Number of nodes: $NUM_NODES"
echo "  Base LibP2P port: $BASE_PORT"
echo "  Base diagnostics port: $BASE_DIAG_PORT"
echo "  Storage directory: $BASE_STORAGE_DIR"
echo "  Config directory: $CONFIG_DIR"
echo "  Keystore directory: $KEYSTORE_DIR"
echo ""

# Check prerequisites
if ! command -v geth &> /dev/null; then
    echo "⚠ Warning: geth not found. You'll need to create keyfiles manually."
    echo "  Install geth or create keyfiles at: $KEYSTORE_DIR"
fi

# Create directories
mkdir -p "$CONFIG_DIR"
mkdir -p "$KEYSTORE_DIR"
mkdir -p "$BASE_STORAGE_DIR"

echo "Step 1: Creating operator keyfiles..."
echo "-----------------------------------"

# Generate keyfiles if geth is available
if command -v geth &> /dev/null; then
    for i in $(seq 1 $NUM_NODES); do
        KEYFILE_PATH="$KEYSTORE_DIR/operator${i}"
        if [ ! -d "$KEYFILE_PATH" ]; then
            echo "Creating keyfile for operator $i..."
            mkdir -p "$KEYFILE_PATH"
            # Use expect or geth's --password flag if available
            echo "password" | geth account new --keystore "$KEYFILE_PATH" --password <(echo "password") 2>/dev/null || \
            geth account new --keystore "$KEYFILE_PATH" <<< $'password\npassword' 2>/dev/null || \
            echo "⚠ Could not auto-generate keyfile for operator $i"
            echo "  Please create manually: geth account new --keystore $KEYFILE_PATH"
        else
            echo "✓ Keyfile already exists for operator $i"
        fi
    done
else
    echo "⚠ geth not available. Please create keyfiles manually:"
    for i in $(seq 1 $NUM_NODES); do
        echo "  geth account new --keystore $KEYSTORE_DIR/operator${i}"
    done
fi

echo ""
echo "Step 2: Finding keyfiles..."
echo "-----------------------------------"

# Find keyfiles
declare -a KEYFILES
for i in $(seq 1 $NUM_NODES); do
    KEYFILE=$(find "$KEYSTORE_DIR/operator${i}" -name "UTC--*" 2>/dev/null | head -1 || echo "")
    if [ -z "$KEYFILE" ]; then
        KEYFILE=$(find "$KEYSTORE_DIR" -name "*operator${i}*" -name "UTC--*" 2>/dev/null | head -1 || echo "")
    fi
    if [ -z "$KEYFILE" ]; then
        echo "⚠ Warning: No keyfile found for operator $i"
        echo "  Please create: geth account new --keystore $KEYSTORE_DIR/operator${i}"
        KEYFILES[$i]=""
    else
        KEYFILES[$i]="$KEYFILE"
        echo "✓ Operator $i: $(basename $KEYFILE)"
    fi
done

echo ""
echo "Step 3: Creating config files..."
echo "-----------------------------------"

# Read base config to get contract addresses and Bitcoin config
if [ -f "configs/config.toml" ]; then
    WALLET_REGISTRY=$(grep -A 10 "\[developer\]" configs/config.toml | grep "WalletRegistryAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
    TOKEN_STAKING=$(grep -A 10 "\[developer\]" configs/config.toml | grep "TokenStakingAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
    RANDOM_BEACON=$(grep -A 10 "\[developer\]" configs/config.toml | grep "RandomBeaconAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
    BRIDGE=$(grep -A 10 "\[developer\]" configs/config.toml | grep "BridgeAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
    MAINTAINER_PROXY=$(grep -A 10 "\[developer\]" configs/config.toml | grep "MaintainerProxyAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
    WALLET_PROPOSAL_VALIDATOR=$(grep -A 10 "\[developer\]" configs/config.toml | grep "WalletProposalValidatorAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
    ETH_URL=$(grep "^URL" configs/config.toml | grep -A 5 "\[ethereum\]" | grep "^URL" | head -1 | cut -d'=' -f2 | tr -d ' "' || echo "http://localhost:8545")
    BITCOIN_ELECTRUM_URL=$(grep "^URL" configs/config.toml | grep -A 5 "\[bitcoin.electrum\]" | grep "^URL" | head -1 | cut -d'=' -f2 | tr -d ' "' || echo "tcp://148.251.237.196:50001")
else
    WALLET_REGISTRY=""
    TOKEN_STAKING=""
    RANDOM_BEACON=""
    BRIDGE=""
    MAINTAINER_PROXY=""
    WALLET_PROPOSAL_VALIDATOR=""
    ETH_URL="http://localhost:8545"
    BITCOIN_ELECTRUM_URL="tcp://148.251.237.196:50001"
fi

PEERS=()
for i in $(seq 1 $NUM_NODES); do
    PORT=$((BASE_PORT + i - 1))
    DIAG_PORT=$((BASE_DIAG_PORT + i - 1))
    STORAGE_DIR="$BASE_STORAGE_DIR/node${i}"
    CONFIG_FILE="$CONFIG_DIR/node${i}.toml"
    
    KEYFILE="${KEYFILES[$i]}"
    if [ -z "$KEYFILE" ]; then
        KEYFILE="/path/to/operator${i}-keyfile"
    fi
    
    mkdir -p "$STORAGE_DIR"
    
    # Build peers list (nodes connect to previous nodes)
    PEERS_STR=""
    if [ $i -gt 1 ]; then
        for j in $(seq 1 $((i-1))); do
            PREV_PORT=$((BASE_PORT + j - 1))
            # Peer ID will be filled in after nodes start
            if [ -z "$PEERS_STR" ]; then
                PEERS_STR="\"/ip4/127.0.0.1/tcp/${PREV_PORT}/ipfs/<node${j}-peer-id>\""
            else
                PEERS_STR="$PEERS_STR, \"/ip4/127.0.0.1/tcp/${PREV_PORT}/ipfs/<node${j}-peer-id>\""
            fi
        done
    fi
    
    cat > "$CONFIG_FILE" <<EOF
# Configuration for Node $i
# Generated by setup-multi-node-dkg.sh

[ethereum]
URL = "$ETH_URL"
KeyFile = "$KEYFILE"
KeyFilePassword = "password"
MiningCheckInterval = "1s"
RequestsPerSecondLimit = 100
ConcurrencyLimit = 100

[network]
Port = $PORT
Peers = [$PEERS_STR]

[storage]
Dir = "$STORAGE_DIR"

[clientinfo]
Port = $DIAG_PORT
NetworkMetricsTick = "10s"
EthereumMetricsTick = "10s"

[bitcoin.electrum]
URL = "$BITCOIN_ELECTRUM_URL"

[developer]
EOF

    if [ -n "$TOKEN_STAKING" ]; then
        echo "TokenStakingAddress = \"$TOKEN_STAKING\"" >> "$CONFIG_FILE"
    fi
    if [ -n "$RANDOM_BEACON" ]; then
        echo "RandomBeaconAddress = \"$RANDOM_BEACON\"" >> "$CONFIG_FILE"
    fi
    if [ -n "$WALLET_REGISTRY" ]; then
        echo "WalletRegistryAddress = \"$WALLET_REGISTRY\"" >> "$CONFIG_FILE"
    fi
    if [ -n "$BRIDGE" ]; then
        echo "BridgeAddress = \"$BRIDGE\"" >> "$CONFIG_FILE"
    fi
    if [ -n "$MAINTAINER_PROXY" ]; then
        echo "MaintainerProxyAddress = \"$MAINTAINER_PROXY\"" >> "$CONFIG_FILE"
    fi
    if [ -n "$WALLET_PROPOSAL_VALIDATOR" ]; then
        echo "WalletProposalValidatorAddress = \"$WALLET_PROPOSAL_VALIDATOR\"" >> "$CONFIG_FILE"
    fi
    
    echo "✓ Created config: $CONFIG_FILE"
done

echo ""
echo "Step 4: Creating startup script..."
echo "-----------------------------------"

START_SCRIPT="$CONFIG_DIR/start-all-nodes.sh"
cat > "$START_SCRIPT" <<'SCRIPT_EOF'
#!/bin/bash
# Auto-generated script to start all nodes
# Usage: ./configs/start-all-nodes.sh

set -eou pipefail

CONFIG_DIR="$(cd "$(dirname "$0")" && pwd)"
NUM_NODES=$(ls -1 "$CONFIG_DIR"/node*.toml 2>/dev/null | wc -l | tr -d ' ')

if [ "$NUM_NODES" -eq 0 ]; then
    echo "No node configs found in $CONFIG_DIR"
    exit 1
fi

echo "Starting $NUM_NODES nodes..."
echo ""

# Start each node in background
for i in $(seq 1 $NUM_NODES); do
    CONFIG_FILE="$CONFIG_DIR/node${i}.toml"
    if [ -f "$CONFIG_FILE" ]; then
        echo "Starting node $i..."
        KEEP_ETHEREUM_PASSWORD=password ./keep-client --config "$CONFIG_FILE" start --developer > "logs/node${i}.log" 2>&1 &
        echo $! > "logs/node${i}.pid"
        sleep 2
    fi
done

echo ""
echo "All nodes started!"
echo ""
echo "Check status:"
echo "  ./configs/check-nodes.sh"
echo ""
echo "Stop all nodes:"
echo "  ./configs/stop-all-nodes.sh"
SCRIPT_EOF

chmod +x "$START_SCRIPT"

STOP_SCRIPT="$CONFIG_DIR/stop-all-nodes.sh"
cat > "$STOP_SCRIPT" <<'SCRIPT_EOF'
#!/bin/bash
# Auto-generated script to stop all nodes

CONFIG_DIR="$(cd "$(dirname "$0")" && pwd)"
LOGS_DIR="logs"

if [ -d "$LOGS_DIR" ]; then
    for pidfile in "$LOGS_DIR"/node*.pid; do
        if [ -f "$pidfile" ]; then
            PID=$(cat "$pidfile")
            if kill -0 "$PID" 2>/dev/null; then
                echo "Stopping node (PID: $PID)..."
                kill "$PID"
            fi
            rm "$pidfile"
        fi
    done
fi

# Also kill any keep-client processes
pkill -f "keep-client.*start" || true

echo "All nodes stopped"
SCRIPT_EOF

chmod +x "$STOP_SCRIPT"

CHECK_SCRIPT="$CONFIG_DIR/check-nodes.sh"
cat > "$CHECK_SCRIPT" <<SCRIPT_EOF
#!/bin/bash
# Auto-generated script to check node status

CONFIG_DIR="\$(cd "\$(dirname "\$0")" && pwd)"
NUM_NODES=\$(ls -1 "\$CONFIG_DIR"/node*.toml 2>/dev/null | wc -l | tr -d ' ')
BASE_DIAG_PORT=${BASE_DIAG_PORT}

echo "Node Status:"
echo "============"
echo ""

for i in \$(seq 1 \$NUM_NODES); do
    CONFIG_FILE="\$CONFIG_DIR/node\${i}.toml"
    
    if [ -f "\$CONFIG_FILE" ]; then
        # Extract diagnostics port from config file
        DIAG_PORT=\$(awk '/\[clientinfo\]/{flag=1; next} /^\[/{flag=0} flag && /^Port/{print \$3; exit}' "\$CONFIG_FILE" | tr -d ' "' || echo "")
        
        # Fallback to calculated port if extraction failed
        if [ -z "\$DIAG_PORT" ]; then
            DIAG_PORT=\$((BASE_DIAG_PORT + i - 1))
        fi
        
        if curl -s "http://localhost:\${DIAG_PORT}/diagnostics" > /dev/null 2>&1; then
            OPERATOR=\$(curl -s "http://localhost:\${DIAG_PORT}/diagnostics" 2>/dev/null | grep -o '"chain_address":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
            PEER_ID=\$(curl -s "http://localhost:\${DIAG_PORT}/diagnostics" 2>/dev/null | grep -o '"peer_id":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
            echo "✓ Node \$i: Running (Operator: \$OPERATOR, Port: \$DIAG_PORT, Peer: \${PEER_ID:0:20}...)"
        else
            echo "✗ Node \$i: Not running (Port: \$DIAG_PORT)"
        fi
    fi
done
SCRIPT_EOF

chmod +x "$CHECK_SCRIPT"

mkdir -p logs

echo "✓ Created startup scripts:"
echo "  - $START_SCRIPT"
echo "  - $STOP_SCRIPT"
echo "  - $CHECK_SCRIPT"

echo ""
echo "=========================================="
echo "Setup Complete!"
echo "=========================================="
echo ""
echo "Next Steps:"
echo ""
echo "1. Register and authorize operators:"
echo "   ./scripts/register-operators.sh $NUM_NODES"
echo ""
echo "2. Start all nodes:"
echo "   $START_SCRIPT"
echo ""
echo "3. Update peer IDs in configs (after nodes start):"
echo "   ./scripts/update-peer-ids.sh"
echo ""
echo "4. Check node status:"
echo "   $CHECK_SCRIPT"
echo ""
echo "5. Request new wallet (triggers DKG):"
echo "   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
echo "     --submit --config configs/config.toml --developer"
echo ""

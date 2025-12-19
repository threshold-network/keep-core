#!/bin/bash
set -e

# Setup script for Geth with proper genesis initialization

export GETH_DATA_DIR=~/ethereum/data
export GETH_ETHEREUM_ACCOUNT=0x7966c178f466b060aaeb2b91e9149a5fb2ec9c53
export KEEP_ETHEREUM_PASSWORD=password

echo "=== Geth Setup Script ==="
echo "GETH_DATA_DIR: $GETH_DATA_DIR"
echo "GETH_ETHEREUM_ACCOUNT: $GETH_ETHEREUM_ACCOUNT"
echo ""

# Check if genesis.json exists
if [ ! -f "genesis.json" ]; then
    echo "ERROR: genesis.json not found in current directory!"
    echo "Please make sure you're running this from the keep-core root directory."
    exit 1
fi

# Check if Geth is running
if curl -s http://localhost:8545 > /dev/null 2>&1; then
    echo "WARNING: Geth appears to be running on port 8545."
    echo "You need to stop Geth first before re-initializing."
    echo ""
    echo "To stop Geth, find the process and kill it:"
    echo "  pkill -f 'geth.*--port 3000'"
    echo "  or"
    echo "  lsof -ti:8545 | xargs kill"
    echo ""
    read -p "Do you want to continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Expand GETH_DATA_DIR
EXPANDED_GETH_DATA_DIR=$(eval echo "$GETH_DATA_DIR")

# Remove existing chaindata
if [ -d "$EXPANDED_GETH_DATA_DIR/geth" ]; then
    echo "Removing existing chaindata..."
    rm -rf "$EXPANDED_GETH_DATA_DIR/geth"
fi

# Initialize chain
echo "Initializing chain with genesis.json..."
geth --datadir="$EXPANDED_GETH_DATA_DIR" init genesis.json

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Now start Geth with:"
echo ""
echo "export GETH_DATA_DIR=~/ethereum/data"
echo "export GETH_ETHEREUM_ACCOUNT=0x7966c178f466b060aaeb2b91e9149a5fb2ec9c53"
echo ""
echo "geth --port 3000 --networkid 1101 --identity 'somerandomidentity' --ws --ws.addr '127.0.0.1' --ws.port '8546' --ws.origins '*' --ws.api 'admin, debug, web3, eth, txpool, personal, ethash, miner, net' --http --http.port '8545' --http.addr '127.0.0.1' --http.corsdomain '' --http.api 'admin, debug, web3, eth, txpool, personal, ethash, miner, net' --datadir=\$GETH_DATA_DIR --allow-insecure-unlock --miner.etherbase=\$GETH_ETHEREUM_ACCOUNT --mine --miner.threads=1"
echo ""
echo "Then run the install script with:"
echo "export GETH_DATA_DIR=~/ethereum/data"
echo "export KEEP_ETHEREUM_PASSWORD=password"
echo "./scripts/install.sh --network development"

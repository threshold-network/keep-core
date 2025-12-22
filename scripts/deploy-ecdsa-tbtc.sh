#!/bin/bash
# Script to deploy ECDSA and TBTC contracts

set -e

export GETH_DATA_DIR="${GETH_DATA_DIR:-$HOME/ethereum/data}"
export KEEP_ETHEREUM_PASSWORD="${KEEP_ETHEREUM_PASSWORD:-password}"
export NETWORK="${NETWORK:-development}"

KEEP_CORE_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ECDSA_SOL_PATH="$KEEP_CORE_PATH/solidity/ecdsa"
TMP="$KEEP_CORE_PATH/tmp"
THRESHOLD_SOL_PATH="$TMP/solidity-contracts"
BEACON_SOL_PATH="$KEEP_CORE_PATH/solidity/random-beacon"

echo "=== Deploying ECDSA and TBTC Contracts ==="
echo "Network: $NETWORK"
echo "GETH_DATA_DIR: $GETH_DATA_DIR"
echo ""

# Check if threshold-network is deployed
if [ ! -d "$THRESHOLD_SOL_PATH/deployments/development" ]; then
  echo "ERROR: Threshold Network contracts must be deployed first!"
  echo "Run: ./scripts/install.sh --network development"
  exit 1
fi

# Check if random-beacon is deployed
if [ ! -d "$BEACON_SOL_PATH/deployments/development" ]; then
  echo "ERROR: Random Beacon contracts must be deployed first!"
  echo "Run: ./scripts/install.sh --network development"
  exit 1
fi

# Deploy ECDSA contracts
echo "📦 Deploying ECDSA contracts..."
cd "$ECDSA_SOL_PATH"

# Update resolutions
if [ -f "package.json" ] && [ -n "$THRESHOLD_SOL_PATH" ]; then
  THRESHOLD_PORTAL_PATH="portal:$THRESHOLD_SOL_PATH"
  node -e "
    const fs = require('fs');
    const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
    if (!pkg.resolutions) pkg.resolutions = {};
    pkg.resolutions['@threshold-network/solidity-contracts'] = '$THRESHOLD_PORTAL_PATH';
    pkg.resolutions['@openzeppelin/contracts'] = '4.7.3';
    fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
  " 2>/dev/null || true
  
  echo "Installing dependencies..."
  yarn install --mode=update-lockfile && yarn install
fi

# Link random-beacon
echo "Linking random-beacon..."
yarn unlink @keep-network/random-beacon 2>/dev/null || true
yarn link @keep-network/random-beacon || {
  echo "Warning: Could not link random-beacon, but continuing..."
}

# Build
echo "Building ECDSA contracts..."
yarn clean && yarn build

# Deploy
echo "Deploying ECDSA contracts..."
yarn deploy --reset --network $NETWORK

# Create link
echo "Creating ECDSA link..."
yarn unlink || true && yarn link
yarn prepack

echo ""
echo "✓ ECDSA contracts deployed!"
echo ""

# Deploy TBTC contracts
echo "📦 Deploying TBTC contracts..."

if [ ! -d "$TMP/tbtc-v2" ]; then
  echo "Cloning tbtc-v2..."
  cd "$TMP"
  git clone https://github.com/keep-network/tbtc-v2.git
fi

TBTC_SOL_PATH="$TMP/tbtc-v2/solidity"
cd "$TBTC_SOL_PATH"

echo "Installing TBTC dependencies..."
yarn install --mode=update-lockfile && yarn install

# Update resolutions
if [ -f "package.json" ] && [ -n "$THRESHOLD_SOL_PATH" ]; then
  THRESHOLD_PORTAL_PATH="portal:$THRESHOLD_SOL_PATH"
  node -e "
    const fs = require('fs');
    const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
    if (!pkg.resolutions) pkg.resolutions = {};
    pkg.resolutions['@threshold-network/solidity-contracts'] = '$THRESHOLD_PORTAL_PATH';
    pkg.resolutions['@openzeppelin/contracts'] = '4.7.3';
    fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
  " 2>/dev/null || true
  
  yarn install --mode=update-lockfile && yarn install 2>/dev/null || true
fi

# Link dependencies
echo "Linking dependencies..."
yarn unlink @threshold-network/solidity-contracts 2>/dev/null || true
yarn link "@threshold-network/solidity-contracts" 2>/dev/null || {
  echo "Warning: Could not link threshold-network/solidity-contracts, but continuing..."
}

yarn unlink @keep-network/random-beacon 2>/dev/null || true
yarn link @keep-network/random-beacon || {
  echo "Warning: Could not link random-beacon, but continuing..."
}

yarn unlink @keep-network/ecdsa 2>/dev/null || true
yarn link @keep-network/ecdsa || {
  echo "Warning: Could not link ecdsa, but continuing..."
}

# Build
echo "Building TBTC contracts..."
yarn build

# Deploy
echo "Deploying TBTC contracts..."
yarn deploy --reset --network $NETWORK

# Create export
echo "Creating TBTC export..."
yarn prepack

echo ""
echo "✓ TBTC contracts deployed!"
echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Run ./scripts/check-deployments.sh to verify all contracts are deployed"


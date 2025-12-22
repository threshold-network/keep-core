#!/bin/bash
set -eou pipefail

# Quick setup script for multi-node DKG testing
# This is a convenience wrapper that runs the full setup process
#
# Usage:
#   ./scripts/quick-dkg-setup.sh [num-nodes]

NUM_NODES=${1:-5}

echo "=========================================="
echo "Quick Multi-Node DKG Setup"
echo "=========================================="
echo ""
echo "This will set up $NUM_NODES nodes for DKG testing"
echo ""

# Step 1: Setup nodes
echo "Step 1: Setting up nodes..."
./scripts/setup-multi-node-dkg.sh "$NUM_NODES" || {
    echo "⚠ Setup failed. Please check errors above."
    exit 1
}

echo ""
echo "Step 2: Register operators..."
echo "  (This step requires manual interaction or can be done later)"
echo ""
read -p "Register operators now? (y/n) " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    ./scripts/register-operators.sh "$NUM_NODES" || {
        echo "⚠ Registration had issues. You can run it manually later."
    }
fi

echo ""
echo "=========================================="
echo "Setup Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo ""
echo "1. Start all nodes:"
echo "   ./configs/start-all-nodes.sh"
echo ""
echo "2. Wait for nodes to start, then update peer IDs:"
echo "   ./scripts/update-peer-ids.sh"
echo ""
echo "3. Restart nodes (to apply peer IDs):"
echo "   ./configs/stop-all-nodes.sh"
echo "   ./configs/start-all-nodes.sh"
echo ""
echo "4. Check node status:"
echo "   ./configs/check-nodes.sh"
echo ""
echo "5. Request new wallet (triggers DKG):"
echo "   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
echo "     --submit --config configs/config.toml --developer"
echo ""
echo "6. Monitor DKG:"
echo "   watch -n 2 './configs/check-nodes.sh'"
echo ""

#!/bin/bash
# Complete DKG workflow with 3 nodes

set -eou pipefail

CONFIG="configs/config.toml"

echo "=========================================="
echo "DKG Process with 3 Nodes"
echo "=========================================="
echo ""

# Step 1: Verify prerequisites
echo "Step 1: Checking prerequisites..."
echo ""

# Check nodes are running
echo "Node status:"
./configs/check-nodes.sh 2>&1 | head -5

# Check connectivity
echo ""
echo "Connectivity:"
for i in {1..3}; do
  PEERS=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq '.connected_peers | length' 2>/dev/null || echo "0")
  echo "  Node $i: $PEERS peers"
done

# Check operators are in pool
echo ""
echo "Operator pool status:"
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null)
  if [ -n "$OPERATOR" ] && [ "$OPERATOR" != "null" ]; then
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
      "$OPERATOR" --config "$CONFIG" --developer 2>&1 | tail -1)
    echo "  Node $i ($OPERATOR): $IN_POOL"
  fi
done

echo ""
echo "Step 2: Triggering DKG..."
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config "$CONFIG" --developer

echo ""
echo "Step 3: Waiting for pool to lock..."
sleep 5

STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG" --developer 2>&1 | tail -1)

echo "DKG State: $STATE"

if [ "$STATE" != "0" ]; then
  echo ""
  echo "Step 4: Pool is locked. Selecting group..."
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
    --config "$CONFIG" --developer
  
  echo ""
  echo "Step 5: Monitoring DKG progress..."
  echo "Watch logs: tail -f logs/node*.log | grep -i dkg"
  echo ""
  echo "Monitor script: ./scripts/monitor-dkg.sh"
else
  echo "⚠ Pool is still unlocked. Wait a few seconds and check again."
fi

echo ""
echo "=========================================="
echo "DKG Process Started"
echo "=========================================="

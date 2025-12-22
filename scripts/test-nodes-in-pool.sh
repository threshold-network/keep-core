#!/bin/bash
# Test if nodes/operators are in sortition pool

set -eou pipefail

CONFIG="configs/config.toml"

echo "=========================================="
echo "Testing Nodes in Sortition Pool"
echo "=========================================="
echo ""

# Step 1: Check if nodes are running
echo "Step 1: Checking if nodes are running..."
RUNNING_NODES=0
for i in {1..3}; do
  if curl -s http://localhost:960$i/diagnostics > /dev/null 2>&1; then
    RUNNING_NODES=$((RUNNING_NODES + 1))
    echo "  ✓ Node $i: Running"
  else
    echo "  ✗ Node $i: Not running"
  fi
done

if [ "$RUNNING_NODES" -eq 0 ]; then
  echo ""
  echo "⚠ No nodes are running!"
  echo "Start nodes with: ./configs/start-all-nodes.sh"
  exit 1
fi

echo ""
echo "Step 2: Checking operator pool status..."
echo ""

ALL_IN_POOL=true
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null)
  
  if [ -z "$OPERATOR" ] || [ "$OPERATOR" = "null" ]; then
    echo "  ✗ Node $i: Could not get operator address"
    ALL_IN_POOL=false
    continue
  fi
  
  echo "Node $i:"
  echo "  Operator: $OPERATOR"
  
  # Check if in pool
  IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config "$CONFIG" --developer 2>&1 | tail -1)
  
  if [ "$IN_POOL" = "true" ]; then
    echo "  ✓ In Pool: YES"
    
    # Get operator weight (if available)
    echo "  Checking operator weight..."
    WEIGHT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry authorized-stake \
      "$OPERATOR" --config "$CONFIG" --developer 2>&1 | grep -E "^[0-9]+$" | tail -1)
    if [ -n "$WEIGHT" ] && [ "$WEIGHT" != "Error" ]; then
      echo "  Authorized Stake: $WEIGHT"
    else
      echo "  Authorized Stake: (check manually)"
    fi
  else
    echo "  ✗ In Pool: NO"
    ALL_IN_POOL=false
  fi
  
  echo ""
done

echo "Step 3: Checking pool state..."
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG" --developer 2>&1 | tail -1)

case "$STATE" in
  0)
    echo "  Pool State: IDLE (unlocked)"
    ;;
  1)
    echo "  Pool State: AWAITING_SEED (locked)"
    ;;
  2)
    echo "  Pool State: AWAITING_RESULT (locked - DKG in progress)"
    ;;
  3)
    echo "  Pool State: CHALLENGE (locked)"
    ;;
  *)
    echo "  Pool State: Unknown ($STATE)"
    ;;
esac

echo ""
echo "Step 4: Summary"
echo "=========================================="
if [ "$ALL_IN_POOL" = true ]; then
  echo "✅ All operators are in the sortition pool!"
  echo ""
  echo "You can now trigger DKG:"
  echo "  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \\"
  echo "    --submit --config $CONFIG --developer"
else
  echo "⚠ Some operators are NOT in the pool"
  echo ""
  echo "To add operators to pool:"
  echo "  1. If chaosnet is active, add as beta operators:"
  echo "     ./scripts/add-beta-operators.sh"
  echo ""
  echo "  2. Then join to pool:"
  echo "     ./scripts/fix-operators-not-in-pool.sh"
fi

echo ""
echo "=========================================="

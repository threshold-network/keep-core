#!/bin/bash
# Diagnose and fix operators not in pool

set -eou pipefail

CONFIG="configs/config.toml"

echo "=========================================="
echo "Diagnosing Operators Not in Pool"
echo "=========================================="
echo ""

# Step 1: Check pool status
echo "Step 1: Checking DKG state (pool must be unlocked)..."
STATE=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config "$CONFIG" --developer 2>&1 | tail -1)

if [ "$STATE" != "0" ]; then
  echo "⚠ Pool is LOCKED (DKG state: $STATE)"
  echo ""
  echo "Options:"
  echo "  1. Wait for DKG to complete (~89 minutes)"
  echo "  2. Notify timeout if stuck: ./scripts/stop-dkg.sh"
  exit 1
else
  echo "✓ Pool is UNLOCKED (DKG state: IDLE)"
fi

echo ""
echo "Step 2: Checking operator pool status..."
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null)
  if [ -n "$OPERATOR" ] && [ "$OPERATOR" != "null" ]; then
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
      "$OPERATOR" --config "$CONFIG" --developer 2>&1 | tail -1)
    echo "  Node $i ($OPERATOR): $IN_POOL"
    
    if [ "$IN_POOL" = "false" ]; then
      echo "    → Not in pool, attempting to join..."
      KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry join-sortition-pool \
        --submit --config "configs/node$i.toml" --developer 2>&1 | tail -3 || echo "    ⚠ Join failed (check error above)"
      sleep 2
    fi
  fi
done

echo ""
echo "Step 3: Verifying final status..."
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics 2>/dev/null | jq -r '.client_info.chain_address' 2>/dev/null)
  if [ -n "$OPERATOR" ] && [ "$OPERATOR" != "null" ]; then
    IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
      "$OPERATOR" --config "$CONFIG" --developer 2>&1 | tail -1)
    if [ "$IN_POOL" = "true" ]; then
      echo "  ✓ Node $i: IN POOL"
    else
      echo "  ✗ Node $i: NOT IN POOL"
      echo "    Check logs: tail -50 logs/node$i.log | grep -i pool"
    fi
  fi
done

echo ""
echo "=========================================="
echo "Diagnosis Complete"
echo "=========================================="
echo ""
echo "If operators failed to join due to 'Not beta operator for chaosnet':"
echo "  1. Add them as beta operators:"
echo "     cd solidity/ecdsa"
echo "     npx hardhat add_beta_operator:ecdsa --operator <ADDRESS> --network developer"
echo ""
echo "  2. Then run this script again to join them to the pool"

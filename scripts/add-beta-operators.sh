#!/bin/bash
# Add all 3 operators as beta operators

set -eou pipefail

cd solidity/ecdsa

OPERATORS=(
  "0xEf38534ea190856217CBAF454a582BeB74b9e7BF"  # Node 1
  "0x5B4ad7861c4da60c033a30d199E30c47435Fe35A"  # Node 2
  "0x4e2A0254244d5298cfF5ea30c5d4bd21077b372d"  # Node 3
)

echo "=========================================="
echo "Adding Operators as Beta Operators"
echo "=========================================="
echo ""

# Step 1: Unlock accounts (required for transactions)
echo "Step 1: Unlocking Ethereum accounts..."
KEEP_ETHEREUM_PASSWORD=${KEEP_ETHEREUM_PASSWORD:-password} \
  npx hardhat unlock-accounts --network development || {
  echo "⚠ Warning: Account unlock failed. Continuing anyway..."
  echo "  If transactions fail, unlock accounts manually or check Geth is running."
}
echo ""

for i in "${!OPERATORS[@]}"; do
  OP="${OPERATORS[$i]}"
  NODE=$((i + 1))
  echo "Adding Node $NODE operator ($OP) as beta operator..."
  npx hardhat add_beta_operator:ecdsa --operator "$OP" --network development
  echo ""
done

echo "=========================================="
echo "All operators added as beta operators!"
echo "=========================================="
echo ""
echo "Next step: Join operators to sortition pool:"
echo "  cd ../.."
echo "  ./scripts/fix-operators-not-in-pool.sh"

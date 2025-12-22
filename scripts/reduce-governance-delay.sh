#!/bin/bash
# Reduce governance delay to 0 for local development

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/solidity/ecdsa"

echo "=========================================="
echo "Reducing Governance Delay to 0"
echo "=========================================="
echo ""
echo "This will allow immediate wallet owner updates."
echo ""

# Unlock accounts
echo "Step 1: Unlocking accounts..."
KEEP_ETHEREUM_PASSWORD=${KEEP_ETHEREUM_PASSWORD:-password} \
  npx hardhat unlock-accounts --network development || {
  echo "⚠ Warning: Account unlock failed. Continuing anyway..."
}
echo ""

# Update governance delay to 0
echo "Step 2: Updating governance delay to 0..."
npx hardhat console --network development <<'EOF'
const { helpers, ethers } = require("hardhat");
(async () => {
  const governance = await helpers.contracts.getContract("WalletRegistryGovernance");
  const signer = await ethers.getSigner(2); // governance account
  
  console.log("Current delay:", (await governance.governanceDelay()).toString());
  
  // Begin delay update
  const beginTx = await governance.connect(signer).beginGovernanceDelayUpdate(0);
  await beginTx.wait();
  console.log("✅ Delay update initiated:", beginTx.hash);
  
  // Try to finalize immediately
  try {
    const finalizeTx = await governance.connect(signer).finalizeGovernanceDelayUpdate();
    await finalizeTx.wait();
    console.log("✅ Delay updated to 0:", finalizeTx.hash);
  } catch (e) {
    console.log("⏳ Need to wait for governance delay to finalize");
    console.log("Then run: governance.connect(signer).finalizeGovernanceDelayUpdate()");
  }
})();
EOF

echo ""
echo "=========================================="

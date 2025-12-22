#!/bin/bash
# Script to fund all staking providers with ETH and T tokens
# Usage: ./scripts/fund-staking-providers.sh

set -eou pipefail

# Get absolute path to mapping file (before changing directories)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MAPPING_FILE="$PROJECT_ROOT/keystore/staking-provider-mapping.txt"
MAIN_ACCOUNT="0x7966c178f466b060aaeb2b91e9149a5fb2ec9c53"
ETH_AMOUNT="100"  # 100 ETH per staking provider
T_AMOUNT="50000"  # 50k T tokens per staking provider

if [ ! -f "$MAPPING_FILE" ]; then
    echo "⚠ Error: Mapping file not found: $MAPPING_FILE"
    exit 1
fi

echo "=========================================="
echo "Funding Staking Providers"
echo "=========================================="
echo ""

cd "$PROJECT_ROOT/solidity/ecdsa"

# Extract all staking provider addresses
STAKING_PROVIDERS=$(grep "^0x" "$MAPPING_FILE" | cut -d'=' -f2 | sort -u)

echo "Found $(echo "$STAKING_PROVIDERS" | wc -l | tr -d ' ') unique staking providers"
echo ""

# Create a temporary script file for funding (in the ecdsa directory)
TEMP_SCRIPT="$PROJECT_ROOT/solidity/ecdsa/temp-fund-script.js"
cat > "$TEMP_SCRIPT" << 'SCRIPT_EOF'
const { ethers, helpers } = require("hardhat");

(async () => {
  try {
    const mainAccount = process.env.MAIN_ACCOUNT;
    const stakingProvider = process.env.STAKING_PROVIDER;
    const ethAmount = process.env.ETH_AMOUNT;
    const tAmount = process.env.T_AMOUNT;
    
    const t = await helpers.contracts.getContract("T");
    const mainSigner = await ethers.getSigner(mainAccount);
    
    // Fund with ETH
    const ethTx = await mainSigner.sendTransaction({
      to: stakingProvider,
      value: ethers.utils.parseEther(ethAmount)
    });
    await ethTx.wait();
    console.log(`  ✓ Funded with ${ethAmount} ETH`);
    
    // Mint T tokens
    const tokenOwner = await t.owner();
    const ownerSigner = await ethers.getSigner(tokenOwner);
    const mintTx = await t.connect(ownerSigner).mint(stakingProvider, ethers.utils.parseEther(tAmount));
    await mintTx.wait();
    console.log(`  ✓ Minted ${tAmount} T tokens`);
    
    process.exit(0);
  } catch (error) {
    console.error("  Error:", error.message);
    process.exit(1);
  }
})();
SCRIPT_EOF

for STAKING_PROVIDER in $STAKING_PROVIDERS; do
    echo "Funding $STAKING_PROVIDER..."
    
    MAIN_ACCOUNT="$MAIN_ACCOUNT" \
    STAKING_PROVIDER="$STAKING_PROVIDER" \
    ETH_AMOUNT="$ETH_AMOUNT" \
    T_AMOUNT="$T_AMOUNT" \
    npx hardhat run temp-fund-script.js --network development 2>&1 | grep -E "(Funded|Minted|Error|✓)" || echo "  Processing..."
    
    sleep 1
done

rm -f "$TEMP_SCRIPT"

cd "$PROJECT_ROOT"

echo ""
echo "=========================================="
echo "✓ All staking providers funded!"
echo "=========================================="
echo ""
echo "You can now register operators:"
echo "  ./scripts/register-single-operator.sh <node-number>"

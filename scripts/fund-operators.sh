#!/bin/bash
set -eou pipefail

# Script to fund operator accounts with ETH for gas
# Usage: ./scripts/fund-operators.sh [num-nodes] [amount-eth]

NUM_NODES=${1:-5}
AMOUNT_ETH=${2:-1.0}
CONFIG_DIR=${3:-./configs}
KEYSTORE_DIR=${4:-./keystore}

echo "=========================================="
echo "Funding Operator Accounts with ETH"
echo "=========================================="
echo ""

# Extract operator addresses from keyfiles
declare -a OPERATOR_ADDRESSES
for i in $(seq 1 $NUM_NODES); do
    KEYFILE=$(find "$KEYSTORE_DIR/operator${i}" -name "UTC--*" 2>/dev/null | head -1 || echo "")
    if [ -z "$KEYFILE" ]; then
        KEYFILE=$(find "$KEYSTORE_DIR" -name "*operator${i}*" -name "UTC--*" 2>/dev/null | head -1 || echo "")
    fi
    
    if [ -z "$KEYFILE" ]; then
        echo "⚠ Warning: No keyfile found for operator $i"
        OPERATOR_ADDRESSES[$i]=""
        continue
    fi
    
    # Extract address from keyfile name
    RAW_ADDRESS=$(basename "$KEYFILE" | sed 's/UTC--[0-9TZ.-]*--//' | tr '[:upper:]' '[:lower:]')
    
    # Add 0x prefix if not present
    if [[ $RAW_ADDRESS != 0x* ]]; then
        ADDRESS="0x${RAW_ADDRESS}"
    else
        ADDRESS="$RAW_ADDRESS"
    fi
    
    # Validate address format
    if [ ${#ADDRESS} -eq 42 ] && [[ $ADDRESS == 0x* ]]; then
        OPERATOR_ADDRESSES[$i]="$ADDRESS"
        echo "✓ Operator $i: $ADDRESS"
    else
        OPERATOR_ADDRESSES[$i]=""
    fi
done

echo ""
echo "Funding each operator with $AMOUNT_ETH ETH..."
echo ""

cd solidity/ecdsa 2>/dev/null || cd ../solidity/ecdsa 2>/dev/null || {
    echo "⚠ Error: Could not find solidity/ecdsa directory"
    exit 1
}

for i in $(seq 1 $NUM_NODES); do
    OPERATOR="${OPERATOR_ADDRESSES[$i]}"
    if [ -z "$OPERATOR" ]; then
        echo "⚠ Skipping operator $i (no address)"
        continue
    fi
    
    echo "Funding operator $i ($OPERATOR)..."
    
    FUND_OUTPUT=$(npx hardhat console --network development 2>&1 <<EOF
const { ethers } = require("hardhat");
(async () => {
  const [signer] = await ethers.getSigners();
  const targetAddress = "$OPERATOR";
  const amount = ethers.utils.parseEther("$AMOUNT_ETH");
  
  const balance = await ethers.provider.getBalance(targetAddress);
  const balanceEth = parseFloat(ethers.utils.formatEther(balance));
  
  if (balanceEth >= $AMOUNT_ETH) {
    console.log(\`Already has \${balanceEth} ETH (sufficient)\`);
    process.exit(0);
  }
  
  console.log(\`Current balance: \${balanceEth} ETH\`);
  console.log(\`Sending $AMOUNT_ETH ETH from \${await signer.getAddress()}...\`);
  const tx = await signer.sendTransaction({
    to: targetAddress,
    value: amount
  });
  console.log(\`Transaction: \${tx.hash}\`);
  await tx.wait();
  
  const newBalance = await ethers.provider.getBalance(targetAddress);
  console.log(\`New balance: \${ethers.utils.formatEther(newBalance)} ETH\`);
  process.exit(0);
})();
EOF
)
    
    if echo "$FUND_OUTPUT" | grep -qE "(Transaction|New balance|Already has)"; then
        echo "$FUND_OUTPUT" | grep -E "(Transaction|New balance|Already has|Current balance)"
    elif echo "$FUND_OUTPUT" | grep -qE "(Error|error)"; then
        echo "$FUND_OUTPUT" | grep -E "(Error|error)" | head -3
    fi
    
    echo ""
    sleep 1
done

cd - > /dev/null 2>&1

echo "=========================================="
echo "✓ Funding complete!"
echo "=========================================="


#!/bin/bash
# Script to register a single operator
# Usage: ./scripts/register-single-operator.sh <node-number>
# Example: ./scripts/register-single-operator.sh 1

set -u

NODE_NUM=${1:-1}
CONFIG_DIR=${2:-./configs}
MAIN_CONFIG=${3:-configs/config.toml}

if [ -z "$1" ]; then
    echo "Usage: ./scripts/register-single-operator.sh <node-number>"
    echo "Example: ./scripts/register-single-operator.sh 1"
    exit 1
fi

NODE_CONFIG="$CONFIG_DIR/node${NODE_NUM}.toml"

if [ ! -f "$NODE_CONFIG" ]; then
    echo "⚠ Error: Config file not found: $NODE_CONFIG"
    exit 1
fi

echo "=========================================="
echo "Registering Operator for Node $NODE_NUM"
echo "=========================================="
echo ""

# Get contract addresses
WALLET_REGISTRY=$(grep -A 10 "\[developer\]" "$MAIN_CONFIG" | grep "WalletRegistryAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
TOKEN_STAKING=$(grep -A 10 "\[developer\]" "$MAIN_CONFIG" | grep "TokenStakingAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")

if [ -z "$WALLET_REGISTRY" ] || [ -z "$TOKEN_STAKING" ]; then
    echo "⚠ Error: Could not find contract addresses in $MAIN_CONFIG"
    exit 1
fi

# Extract operator address
KEYFILE=$(grep -i "^KeyFile[[:space:]]*=" "$NODE_CONFIG" | head -1 | awk -F'=' '{print $2}' | tr -d ' "')
if [ -z "$KEYFILE" ]; then
    echo "⚠ Error: Could not find KeyFile in $NODE_CONFIG"
    exit 1
fi

# Resolve keyfile path
if [[ "$KEYFILE" == ./* ]]; then
    KEYFILE="${KEYFILE#./}"
    KEYFILE="$(cd "$(dirname "$NODE_CONFIG")/.." && pwd)/$KEYFILE"
fi

if [ ! -f "$KEYFILE" ]; then
    echo "⚠ Error: Keyfile not found: $KEYFILE"
    echo "   Resolved path: $KEYFILE"
    echo "   Config dir: $(dirname "$NODE_CONFIG")"
    exit 1
fi

OPERATOR=$(cat "$KEYFILE" | jq -r '.address' 2>/dev/null || echo "")
# If jq fails, try extracting from filename (format: UTC--...--<address>)
if [ -z "$OPERATOR" ] || [[ "$OPERATOR" != 0x* ]]; then
    FILENAME=$(basename "$KEYFILE")
    # Extract address from filename: UTC--...--<40-char-hex>
    OPERATOR=$(echo "$FILENAME" | sed -E 's/.*--([0-9a-fA-F]{40})$/\1/' | sed 's/^/0x/' || echo "")
fi

if [ -z "$OPERATOR" ] || [[ "$OPERATOR" != 0x* ]]; then
    echo "⚠ Error: Could not extract operator address from keyfile"
    echo "   Keyfile: $KEYFILE"
    echo "   Attempted jq extraction: $(cat "$KEYFILE" | jq -r '.address' 2>&1 | head -1)"
    echo "   Filename: $(basename "$KEYFILE")"
    exit 1
fi

echo "Operator: $OPERATOR"
echo "Config: $NODE_CONFIG"
echo "WalletRegistry: $WALLET_REGISTRY"
echo ""

# Check current registration status
echo "Checking registration status..."
IS_REGISTERED=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
  --config "$MAIN_CONFIG" --developer 2>&1 | grep -iE "(true|false)" | head -1 || echo "unknown")

if [ "$IS_REGISTERED" == "true" ]; then
    echo "✓ Operator is already registered!"
    exit 0
fi

echo "Operator is not registered. Starting registration..."
echo ""

# Amounts (hex format)
STAKE_AMOUNT="0xa968163f0a57b400000"  # 50k T tokens
AUTHORIZATION_AMOUNT="0x878678326eac9000000"  # 40k T (minimum authorization)

# Step 0: Approve tokens
echo "Step 0: Approving T tokens..."
cd solidity/ecdsa 2>/dev/null || cd ../solidity/ecdsa 2>/dev/null || {
    echo "⚠ Error: Could not find solidity/ecdsa directory"
    exit 1
}

ABS_KEYFILE=$(cd "$(dirname "$KEYFILE")" && pwd)/$(basename "$KEYFILE")
APPROVE_OUTPUT=$(npx hardhat console --network development 2>&1 <<EOF
const { ethers, helpers } = require("hardhat");
const fs = require("fs");

(async () => {
  try {
    const t = await helpers.contracts.getContract("T");
    const staking = await helpers.contracts.getContract("TokenStaking");
    const operator = "$OPERATOR";
    const stakeAmountHex = "$STAKE_AMOUNT";
    const stakeAmount = ethers.BigNumber.from(stakeAmountHex);
    const keyfilePath = "$ABS_KEYFILE";
    const password = "${KEEP_ETHEREUM_PASSWORD:-password}";
    
    const keyfile = JSON.parse(fs.readFileSync(keyfilePath, "utf8"));
    const wallet = await ethers.Wallet.fromEncryptedJson(JSON.stringify(keyfile), password);
    const provider = new ethers.providers.JsonRpcProvider("http://localhost:8545");
    const operatorSigner = wallet.connect(provider);
    
    const tWithSigner = t.connect(operatorSigner);
    const currentAllowance = await tWithSigner.allowance(operatorSigner.address, staking.address);
    const { from1e18 } = helpers.number;
    
    console.log(\`Current allowance: \${from1e18(currentAllowance)} T\`);
    
    if (currentAllowance.gte(stakeAmount)) {
      console.log("✓ Already approved");
      process.exit(0);
    }
    
    console.log(\`Approving \${from1e18(stakeAmount)} T...\`);
    const tx = await tWithSigner.approve(staking.address, stakeAmount);
    console.log(\`Transaction hash: \${tx.hash}\`);
    await tx.wait();
    console.log("✓ Approval successful!");
    process.exit(0);
  } catch (error) {
    console.error("Error:", error.message);
    process.exit(1);
  }
})();
EOF
) || true

cd - > /dev/null 2>&1

if echo "$APPROVE_OUTPUT" | grep -qE "(Approval successful|Already approved)"; then
    echo "$APPROVE_OUTPUT" | grep -E "(Approval successful|Already approved|Transaction hash)"
else
    echo "⚠ Approval may have failed, but continuing..."
fi

echo ""

# Step 1: Stake tokens
echo "Step 1: Staking tokens..."
STAKE_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking stake \
  "$OPERATOR" "$OPERATOR" "$OPERATOR" "$STAKE_AMOUNT" \
  --submit --config "$NODE_CONFIG" --developer 2>&1) || true

if echo "$STAKE_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
    echo "✓ Staking transaction submitted:"
    echo "$STAKE_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
elif echo "$STAKE_OUTPUT" | grep -qiE "(already in use|already set|Provider is already)"; then
    echo "✓ Staking provider already in use (already staked)"
    echo "   Note: If authorization fails with 'Not enough stake', you may need to top-up stake"
    echo "   Continuing with authorization and registration..."
else
    echo "⚠ Staking failed:"
    echo "$STAKE_OUTPUT" | tail -5
    echo ""
    echo "   If you see 'Provider is already in use', the operator is already staked."
    echo "   Continuing with authorization and registration..."
fi

sleep 2

# Step 2: Authorize WalletRegistry
echo ""
echo "Step 2: Authorizing WalletRegistry..."
AUTH_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
  "$OPERATOR" "$WALLET_REGISTRY" "$AUTHORIZATION_AMOUNT" \
  --submit --config "$NODE_CONFIG" --developer 2>&1) || true

if echo "$AUTH_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
    echo "✓ Authorization transaction submitted:"
    echo "$AUTH_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
elif echo "$AUTH_OUTPUT" | grep -qiE "(Not enough stake|insufficient)"; then
    echo "⚠ Authorization failed: Not enough stake"
    echo "   The operator needs more staked tokens to authorize WalletRegistry"
    echo "   Current stake may be insufficient. Try topping up stake first."
    echo ""
    echo "   You can top-up stake with:"
    echo "   ./keep-client ethereum threshold token-staking top-up \\"
    echo "     $OPERATOR $STAKE_AMOUNT \\"
    echo "     --submit --config $NODE_CONFIG --developer"
    echo ""
    echo "   Or continue anyway - registration may still work if already authorized..."
elif echo "$AUTH_OUTPUT" | grep -qiE "(already|sufficient)"; then
    echo "✓ Authorization already sufficient"
else
    echo "⚠ Authorization failed:"
    echo "$AUTH_OUTPUT" | tail -5
    echo "   Continuing anyway - operator may already be authorized..."
fi

sleep 2

# Step 3a: Register operator in RandomBeacon (required for node startup)
echo ""
echo "Step 3a: Registering operator in RandomBeacon..."
RB_REG_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon register-operator \
  "$OPERATOR" \
  --submit --config "$NODE_CONFIG" --developer 2>&1) || true

if echo "$RB_REG_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
    echo "✓ RandomBeacon registration transaction submitted:"
    echo "$RB_REG_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
elif echo "$RB_REG_OUTPUT" | grep -qiE "(already in use|already set)"; then
    echo "✓ Operator already registered in RandomBeacon"
else
    echo "⚠ RandomBeacon registration failed:"
    echo "$RB_REG_OUTPUT" | tail -5
fi

sleep 2

# Step 3b: Register operator in WalletRegistry (required for ECDSA/DKG)
echo ""
echo "Step 3b: Registering operator in WalletRegistry..."
REG_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry register-operator \
  "$OPERATOR" \
  --submit --config "$NODE_CONFIG" --developer 2>&1) || true

if echo "$REG_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
    echo "✓ WalletRegistry registration transaction submitted:"
    echo "$REG_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
elif echo "$REG_OUTPUT" | grep -qiE "(already in use|already set|Provider is already)"; then
    echo "✓ Operator already registered in WalletRegistry"
else
    echo "⚠ WalletRegistry registration failed:"
    echo "$REG_OUTPUT" | tail -5
fi

sleep 2

# Verify registration
echo ""
echo "Verifying registration..."
RB_REGISTERED=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon operator-to-staking-provider "$OPERATOR" \
  --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -oE "0x[0-9a-fA-F]{40}" || echo "0x0000")
WR_REGISTERED=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry operator-to-staking-provider "$OPERATOR" \
  --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -oE "0x[0-9a-fA-F]{40}" || echo "0x0000")

echo "RandomBeacon registration: $RB_REGISTERED"
echo "WalletRegistry registration: $WR_REGISTERED"

if [ "$RB_REGISTERED" != "0x0000" ] && [ "$WR_REGISTERED" != "0x0000" ]; then
    echo ""
    echo "=========================================="
    echo "✓ Operator registered successfully in both contracts!"
    echo "=========================================="
    echo ""
    echo "You can now start the node:"
    echo "  ./configs/start-all-nodes.sh"
    echo ""
elif [ "$RB_REGISTERED" != "0x0000" ]; then
    echo ""
    echo "✓ Operator registered in RandomBeacon (required for node startup)"
    echo "⚠ Not registered in WalletRegistry (needed for DKG)"
    echo ""
elif [ "$WR_REGISTERED" != "0x0000" ]; then
    echo ""
    echo "⚠ Operator registered in WalletRegistry but NOT in RandomBeacon"
    echo "   Node startup will fail. Re-run registration to register in RandomBeacon."
    echo ""
else
    echo ""
    echo "⚠ Warning: Operator not fully registered"
    echo "   Wait a few seconds and check again, or check transaction status"
    echo ""
fi


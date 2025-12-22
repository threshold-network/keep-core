#!/bin/bash
# Script to register a single operator with a separate staking provider
# Usage: ./scripts/register-single-operator.sh <node-number>
# Example: ./scripts/register-single-operator.sh 1
#
# This script uses separate staking providers for each operator:
# - Each operator has a unique staking provider address
# - Staking provider owns/controls the stake
# - Staking provider registers the operator
# - Operator runs the node
#
# Mapping is defined in: keystore/staking-provider-mapping.txt

set -u

NODE_NUM=${1:-1}
CONFIG_DIR=${2:-./configs}
MAIN_CONFIG=${3:-configs/config.toml}

if [ -z "$1" ]; then
    echo "Usage: ./scripts/register-single-operator.sh <node-number>"
    echo "Example: ./scripts/register-single-operator.sh 1"
    echo ""
    echo "This script uses self-staking (operator = staking provider)"
    echo "Each node gets its own staking provider address."
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

OPERATOR=$(cat "$KEYFILE" | jq -r '.address' 2>/dev/null | head -1 | tr -d '\n\r' || echo "")
# If jq fails, try extracting from filename (format: UTC--...--<address>)
if [ -z "$OPERATOR" ] || [[ "$OPERATOR" != 0x* ]] || [ ${#OPERATOR} -ne 42 ]; then
    FILENAME=$(basename "$KEYFILE")
    # Extract address from filename: UTC--...--<40-char-hex>
    OPERATOR=$(echo "$FILENAME" | sed -E 's/.*--([0-9a-fA-F]{40})$/\1/' | tr '[:upper:]' '[:lower:]' | sed 's/^/0x/' || echo "")
fi

# Function to clean and validate addresses
clean_address() {
    local addr="$1"
    # Remove all whitespace, newlines, carriage returns, and any non-printable chars
    addr=$(printf '%s' "$addr" | tr -d '[:space:]\n\r' | tr -cd '0-9a-fA-Fx' | sed 's/^x/0x/' | sed 's/^\([^0]\)/0x\1/')
    # Ensure it starts with 0x
    if [[ "$addr" != 0x* ]]; then
        addr="0x$addr"
    fi
    # Convert to lowercase
    addr=$(echo "$addr" | tr '[:upper:]' '[:lower:]')
    # Take only first 42 characters (0x + 40 hex)
    addr=$(printf '%.42s' "$addr")
    echo "$addr"
}

# Normalize address: ensure lowercase, trim whitespace, and exactly 42 characters
OPERATOR=$(clean_address "$OPERATOR")

# Final validation: must be exactly 42 characters, start with 0x, followed by 40 hex chars
if [ -z "$OPERATOR" ] || [ ${#OPERATOR} -ne 42 ] || ! printf '%s' "$OPERATOR" | grep -qE '^0x[0-9a-f]{40}$'; then
    echo "⚠ Error: Could not extract valid operator address from keyfile"
    echo "   Keyfile: $KEYFILE"
    echo "   Attempted jq extraction: $(cat "$KEYFILE" | jq -r '.address' 2>&1 | head -1)"
    echo "   Filename: $(basename "$KEYFILE")"
    echo "   Extracted value: '$OPERATOR' (length: ${#OPERATOR})"
    echo "   Hex dump: $(printf '%s' "$OPERATOR" | od -An -tx1 | head -1)"
    exit 1
fi

# Get staking provider address from mapping file
MAPPING_FILE="keystore/staking-provider-mapping.txt"
STAKING_PROVIDER=$(grep "^${OPERATOR}=" "$MAPPING_FILE" 2>/dev/null | cut -d'=' -f2 | tr -d '[:space:]\n\r' || echo "")

if [ -z "$STAKING_PROVIDER" ]; then
    echo "⚠ Error: Could not find staking provider mapping for operator $OPERATOR"
    echo "   Please add mapping to $MAPPING_FILE"
    echo "   Format: $OPERATOR=<staking_provider_address>"
    exit 1
fi

# Normalize staking provider address using the same cleaning function
STAKING_PROVIDER=$(clean_address "$STAKING_PROVIDER")
if [ ${#STAKING_PROVIDER} -ne 42 ] || ! printf '%s' "$STAKING_PROVIDER" | grep -qE '^0x[0-9a-f]{40}$'; then
    echo "⚠ Error: Invalid staking provider address format: '$STAKING_PROVIDER'"
    exit 1
fi

STAKING_PROVIDER_LOWER="$STAKING_PROVIDER"
STAKING_PROVIDER_HEX=${STAKING_PROVIDER_LOWER#0x}

# Find staking provider keyfile (case-insensitive)
STAKING_PROVIDER_KEYFILE=$(ls keystore/staking-providers/*${STAKING_PROVIDER_HEX}* 2>/dev/null | head -1)
if [ -z "$STAKING_PROVIDER_KEYFILE" ]; then
    echo "⚠ Error: Could not find staking provider keyfile for $STAKING_PROVIDER"
    echo "   Looking for: keystore/staking-providers/*${STAKING_PROVIDER#0x}*"
    echo "   Available files:"
    ls -1 keystore/staking-providers/ 2>/dev/null | head -5
    exit 1
fi

echo "Operator: $OPERATOR"
echo "Staking Provider: $STAKING_PROVIDER"
echo "Staking Provider Keyfile: $STAKING_PROVIDER_KEYFILE"
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

# Step 0: Approve tokens (staking provider approves)
echo "Step 0: Approving T tokens for staking provider..."

cd solidity/ecdsa 2>/dev/null || cd ../solidity/ecdsa 2>/dev/null || {
    echo "⚠ Error: Could not find solidity/ecdsa directory"
    exit 1
}

# Resolve absolute path to staking provider keyfile
if [[ "$STAKING_PROVIDER_KEYFILE" == ./* ]] || [[ "$STAKING_PROVIDER_KEYFILE" != /* ]]; then
    ABS_STAKING_KEYFILE="$(cd "$(dirname "$STAKING_PROVIDER_KEYFILE")" && pwd)/$(basename "$STAKING_PROVIDER_KEYFILE")"
else
    ABS_STAKING_KEYFILE="$STAKING_PROVIDER_KEYFILE"
fi
APPROVE_OUTPUT=$(npx hardhat console --network development 2>&1 <<EOF
const { ethers, helpers } = require("hardhat");
const fs = require("fs");

(async () => {
  try {
    const t = await helpers.contracts.getContract("T");
    const staking = await helpers.contracts.getContract("TokenStaking");
    const stakingProvider = "$STAKING_PROVIDER";
    const stakeAmountHex = "$STAKE_AMOUNT";
    const stakeAmount = ethers.BigNumber.from(stakeAmountHex);
    const keyfilePath = "$ABS_STAKING_KEYFILE";
    const password = "${KEEP_ETHEREUM_PASSWORD:-password}";
    
    const keyfile = JSON.parse(fs.readFileSync(keyfilePath, "utf8"));
    const wallet = await ethers.Wallet.fromEncryptedJson(JSON.stringify(keyfile), password);
    const provider = new ethers.providers.JsonRpcProvider("http://localhost:8545");
    const stakingProviderSigner = wallet.connect(provider);
    
    const tWithSigner = t.connect(stakingProviderSigner);
    const currentAllowance = await tWithSigner.allowance(stakingProviderSigner.address, staking.address);
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

# Step 1: Stake tokens (staking provider stakes for operator)
echo "Step 1: Staking tokens..."
echo "  Staking Provider: $STAKING_PROVIDER"
echo "  Operator: $OPERATOR"
# Create temporary config with staking provider's keyfile for staking (macOS-compatible)
TEMP_CONFIG=$(mktemp "${TMPDIR:-/tmp}/keep-config-XXXXXX.toml")
cp "$NODE_CONFIG" "$TEMP_CONFIG"
# Use absolute path for keyfile
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
else
    sed -i.bak "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
    rm -f "${TEMP_CONFIG}.bak"
fi
STAKE_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking stake \
  "$STAKING_PROVIDER" "$OPERATOR" "$STAKING_PROVIDER" "$STAKE_AMOUNT" \
  --submit --config "$TEMP_CONFIG" --developer 2>&1) || true
rm -f "$TEMP_CONFIG" "${TEMP_CONFIG}.bak" 2>/dev/null || true

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

# Step 2a: Authorize RandomBeacon (staking provider authorizes)
echo ""
echo "Step 2a: Authorizing RandomBeacon..."
RANDOM_BEACON="0x18266866EbBab6cA7f5F2724e22CEF54a98Cda92"
# Create temporary config with staking provider's keyfile for authorization
TEMP_CONFIG=$(mktemp "${TMPDIR:-/tmp}/keep-config-XXXXXX.toml")
cp "$NODE_CONFIG" "$TEMP_CONFIG"
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
else
    sed -i.bak "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
    rm -f "${TEMP_CONFIG}.bak"
fi
RB_AUTH_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
  "$STAKING_PROVIDER" "$RANDOM_BEACON" "$AUTHORIZATION_AMOUNT" \
  --submit --config "$TEMP_CONFIG" --developer 2>&1) || true
rm -f "$TEMP_CONFIG" "${TEMP_CONFIG}.bak" 2>/dev/null || true

if echo "$RB_AUTH_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
    echo "✓ RandomBeacon authorization transaction submitted"
elif echo "$RB_AUTH_OUTPUT" | grep -qiE "(already|sufficient)"; then
    echo "✓ RandomBeacon authorization already sufficient"
else
    echo "⚠ RandomBeacon authorization failed (may already be authorized)"
fi

sleep 2

# Step 2b: Authorize WalletRegistry (staking provider authorizes)
echo ""
echo "Step 2b: Authorizing WalletRegistry..."
# Create temporary config with staking provider's keyfile for authorization
TEMP_CONFIG=$(mktemp "${TMPDIR:-/tmp}/keep-config-XXXXXX.toml")
cp "$NODE_CONFIG" "$TEMP_CONFIG"
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
else
    sed -i.bak "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
    rm -f "${TEMP_CONFIG}.bak"
fi
AUTH_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
  "$STAKING_PROVIDER" "$WALLET_REGISTRY" "$AUTHORIZATION_AMOUNT" \
  --submit --config "$TEMP_CONFIG" --developer 2>&1) || true
rm -f "$TEMP_CONFIG" "${TEMP_CONFIG}.bak" 2>/dev/null || true

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

# Step 3a: Register operator in RandomBeacon (staking provider registers operator)
echo ""
echo "Step 3a: Registering operator in RandomBeacon..."
echo "  Staking Provider ($STAKING_PROVIDER) registers Operator ($OPERATOR)"
# Create temporary config with staking provider's keyfile (msg.sender must be staking provider)
TEMP_CONFIG=$(mktemp "${TMPDIR:-/tmp}/keep-config-XXXXXX.toml")
cp "$NODE_CONFIG" "$TEMP_CONFIG"
# Replace KeyFile in temp config with staking provider's keyfile (use absolute path)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
else
    sed -i.bak "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
    rm -f "${TEMP_CONFIG}.bak"
fi
RB_REG_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon register-operator \
  "$OPERATOR" \
  --submit --config "$TEMP_CONFIG" --developer 2>&1) || true
rm -f "$TEMP_CONFIG" "${TEMP_CONFIG}.bak" 2>/dev/null || true

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

# Step 3b: Register operator in WalletRegistry (staking provider registers operator)
echo ""
echo "Step 3b: Registering operator in WalletRegistry..."
echo "  Staking Provider ($STAKING_PROVIDER) registers Operator ($OPERATOR)"
# Create temporary config with staking provider's keyfile
TEMP_CONFIG=$(mktemp "${TMPDIR:-/tmp}/keep-config-XXXXXX.toml")
cp "$NODE_CONFIG" "$TEMP_CONFIG"
# Replace KeyFile in temp config with staking provider's keyfile (use absolute path)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
else
    sed -i.bak "s|KeyFile = .*|KeyFile = \"$ABS_STAKING_KEYFILE\"|" "$TEMP_CONFIG"
    rm -f "${TEMP_CONFIG}.bak"
fi
REG_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry register-operator \
  "$OPERATOR" \
  --submit --config "$TEMP_CONFIG" --developer 2>&1) || true
rm -f "$TEMP_CONFIG" "${TEMP_CONFIG}.bak" 2>/dev/null || true

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

# Step 4a: Join RandomBeacon sortition pool
echo ""
echo "Step 4a: Joining RandomBeacon sortition pool..."
RB_JOIN_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon join-sortition-pool \
  --submit --config "$NODE_CONFIG" --developer 2>&1) || true

if echo "$RB_JOIN_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
    echo "✓ RandomBeacon join transaction submitted:"
    echo "$RB_JOIN_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
elif echo "$RB_JOIN_OUTPUT" | grep -qiE "(already|operator.*in.*pool)"; then
    echo "✓ Operator already in RandomBeacon sortition pool"
else
    echo "⚠ RandomBeacon join failed:"
    echo "$RB_JOIN_OUTPUT" | tail -5
    echo "   Note: Pool may be locked (DKG in progress). Try again later."
fi

sleep 2

# Step 4b: Join WalletRegistry sortition pool
echo ""
echo "Step 4b: Joining WalletRegistry sortition pool..."
WR_JOIN_OUTPUT=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry join-sortition-pool \
  --submit --config "$NODE_CONFIG" --developer 2>&1) || true

if echo "$WR_JOIN_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
    echo "✓ WalletRegistry join transaction submitted:"
    echo "$WR_JOIN_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
elif echo "$WR_JOIN_OUTPUT" | grep -qiE "(already|operator.*in.*pool)"; then
    echo "✓ Operator already in WalletRegistry sortition pool"
else
    echo "⚠ WalletRegistry join failed:"
    echo "$WR_JOIN_OUTPUT" | tail -5
    echo "   Note: Pool may be locked (DKG in progress). Try again later."
fi

sleep 2

# Verify registration and pool status
echo ""
echo "Verifying registration and pool status..."
RB_REGISTERED=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon operator-to-staking-provider "$OPERATOR" \
  --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -oE "0x[0-9a-fA-F]{40}" || echo "0x0000")
WR_REGISTERED=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry operator-to-staking-provider "$OPERATOR" \
  --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -oE "0x[0-9a-fA-F]{40}" || echo "0x0000")

RB_IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum beacon random-beacon is-operator-in-pool "$OPERATOR" \
  --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -iE "(true|false)" || echo "unknown")
WR_IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool "$OPERATOR" \
  --config "$MAIN_CONFIG" --developer 2>&1 | tail -1 | grep -iE "(true|false)" || echo "unknown")

echo "RandomBeacon registration: $RB_REGISTERED"
echo "RandomBeacon in pool: $RB_IN_POOL"
echo "WalletRegistry registration: $WR_REGISTERED"
echo "WalletRegistry in pool: $WR_IN_POOL"

if [ "$RB_REGISTERED" != "0x0000" ] && [ "$WR_REGISTERED" != "0x0000" ]; then
    echo ""
    if [ "$RB_IN_POOL" = "true" ] && [ "$WR_IN_POOL" = "true" ]; then
        echo "=========================================="
        echo "✓ Operator fully registered and in both sortition pools!"
        echo "=========================================="
    elif [ "$RB_IN_POOL" = "true" ] || [ "$WR_IN_POOL" = "true" ]; then
        echo "=========================================="
        echo "✓ Operator registered, but not fully in pools"
        echo "=========================================="
        echo ""
        echo "If pools are locked (DKG in progress), operators will join automatically"
        echo "when pools unlock. Otherwise, manually join with:"
        if [ "$RB_IN_POOL" != "true" ]; then
            echo "  ./keep-client ethereum beacon random-beacon join-sortition-pool \\"
            echo "    --submit --config $NODE_CONFIG --developer"
        fi
        if [ "$WR_IN_POOL" != "true" ]; then
            echo "  ./keep-client ethereum ecdsa wallet-registry join-sortition-pool \\"
            echo "    --submit --config $NODE_CONFIG --developer"
        fi
    else
        echo "=========================================="
        echo "✓ Operator registered successfully!"
        echo "=========================================="
        echo ""
        echo "Operators will join sortition pools automatically when pools are unlocked."
    fi
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


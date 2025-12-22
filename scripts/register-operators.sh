#!/bin/bash
# Use set -u to catch undefined variables, but don't exit on command failures
# since keep-client commands may return non-zero on errors
set -u

# Script to register and authorize operators for multi-node DKG
# 
# Usage:
#   ./scripts/register-operators.sh [num-nodes] [config-dir] [keystore-dir]

NUM_NODES=${1:-5}
CONFIG_DIR=${2:-./configs}
KEYSTORE_DIR=${3:-./keystore}
MAIN_CONFIG=${4:-configs/config.toml}

echo "=========================================="
echo "Register and Authorize Operators"
echo "=========================================="
echo ""

# Get contract addresses from main config
if [ ! -f "$MAIN_CONFIG" ]; then
    echo "⚠ Error: Main config not found: $MAIN_CONFIG"
    exit 1
fi

WALLET_REGISTRY=$(grep -A 10 "\[developer\]" "$MAIN_CONFIG" | grep "WalletRegistryAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")
TOKEN_STAKING=$(grep -A 10 "\[developer\]" "$MAIN_CONFIG" | grep "TokenStakingAddress" | cut -d'=' -f2 | tr -d ' "' || echo "")

if [ -z "$WALLET_REGISTRY" ] || [ -z "$TOKEN_STAKING" ]; then
    echo "⚠ Error: Could not find contract addresses in $MAIN_CONFIG"
    echo "  WalletRegistryAddress: $WALLET_REGISTRY"
    echo "  TokenStakingAddress: $TOKEN_STAKING"
    exit 1
fi

echo "Contracts:"
echo "  WalletRegistry: $WALLET_REGISTRY"
echo "  TokenStaking: $TOKEN_STAKING"
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
    
    # Extract address from keyfile name (format: UTC--timestamp--address)
    # Keyfile names don't include "0x" prefix, so we add it
    RAW_ADDRESS=$(basename "$KEYFILE" | sed 's/UTC--[0-9TZ.-]*--//' | tr '[:upper:]' '[:lower:]')
    
    # Add 0x prefix if not present
    if [[ $RAW_ADDRESS != 0x* ]]; then
        ADDRESS="0x${RAW_ADDRESS}"
    else
        ADDRESS="$RAW_ADDRESS"
    fi
    
    # Validate address format (42 chars including 0x)
    if [ ${#ADDRESS} -eq 42 ] && [[ $ADDRESS == 0x* ]]; then
        OPERATOR_ADDRESSES[$i]="$ADDRESS"
        echo "✓ Operator $i: $ADDRESS"
    else
        echo "⚠ Warning: Could not extract valid address from keyfile: $KEYFILE"
        echo "  Extracted: $ADDRESS (length: ${#ADDRESS})"
        OPERATOR_ADDRESSES[$i]=""
    fi
done

echo ""
echo "=========================================="
echo "Registration Commands"
echo "=========================================="
echo ""
echo "For each operator, run these commands:"
echo ""

for i in $(seq 1 $NUM_NODES); do
    OPERATOR="${OPERATOR_ADDRESSES[$i]}"
    if [ -z "$OPERATOR" ]; then
        continue
    fi
    
    echo "# Operator $i ($OPERATOR)"
    echo "# For local development, use Hardhat to initialize:"
    echo "cd solidity/ecdsa"
    echo "npx hardhat initialize \\"
    echo "  --network development \\"
    echo "  --owner $OPERATOR \\"
    echo "  --provider $OPERATOR \\"
    echo "  --operator $OPERATOR \\"
    echo "  --beneficiary $OPERATOR \\"
    echo "  --authorizer $OPERATOR"
    echo "# Note: Using default stake amount (1M tokens), which is sufficient for local testing"
    echo ""
    echo "# OR use CLI commands:"
    echo "# 1. Stake tokens (100k T)"
    echo "./keep-client ethereum threshold token-staking stake \\"
    echo "  $OPERATOR $OPERATOR $OPERATOR 100000000000000000000000 \\"
    echo "  --submit --config $CONFIG_DIR/node${i}.toml --developer"
    echo ""
    echo "# 2. Authorize WalletRegistry (40k T minimum)"
    echo "./keep-client ethereum threshold token-staking increase-authorization \\"
    echo "  $OPERATOR $WALLET_REGISTRY 40000000000000000000000 \\"
    echo "  --submit --config $CONFIG_DIR/node${i}.toml --developer"
    echo ""
    echo "# 3. Register operator in WalletRegistry"
    echo "./keep-client ethereum ecdsa wallet-registry register-operator \\"
    echo "  $OPERATOR --submit --config $CONFIG_DIR/node${i}.toml --developer"
    echo ""
done

echo ""
echo "=========================================="
echo "Automated Registration (Interactive)"
echo "=========================================="
echo ""
read -p "Do you want to register all operators now? (y/n) " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo "Using keep-client CLI to register operators..."
    echo ""
    
    # Check if keep-client exists
    if [ ! -f "./keep-client" ]; then
        echo "⚠ Error: keep-client binary not found. Please build it first:"
        echo "  go build -o keep-client ."
        exit 1
    fi
    
    # Use amounts that fit in uint96 (max: 79228162514264337593543950335)
    # CLI expects hex format (0x prefix) for amounts
    # 50k T = 50000 * 10^18 = 0xa968163f0a57b400000 (in hex)
    # 40k T = 40000 * 10^18 = 0x878678326eac9000000 (in hex, minimum authorization)
    STAKE_AMOUNT="0xa968163f0a57b400000"  # 50k T tokens (hex format)
    AUTHORIZATION_AMOUNT="0x878678326eac9000000"  # 40k T (hex format, minimum authorization)
    
    echo "⚠ Note: Operator accounts need ETH for gas transactions."
    echo "   For local development, accounts should be funded via genesis block or manually."
    echo "   If accounts are not funded, you can fund them using geth console:"
    echo "     geth attach http://localhost:8545"
    echo "     > eth.sendTransaction({from: eth.accounts[0], to: \"$OPERATOR\", value: web3.toWei(1, \"ether\")})"
    echo ""
    
    for i in $(seq 1 $NUM_NODES); do
        OPERATOR="${OPERATOR_ADDRESSES[$i]}"
        if [ -z "$OPERATOR" ]; then
            echo "⚠ Skipping operator $i (no address)"
            continue
        fi
        
        # Find the config file for this operator
        NODE_CONFIG="$CONFIG_DIR/node${i}.toml"
        if [ ! -f "$NODE_CONFIG" ]; then
            echo "⚠ Warning: Config file not found for operator $i: $NODE_CONFIG"
            echo "  Using main config: $MAIN_CONFIG"
            NODE_CONFIG="$MAIN_CONFIG"
        fi
        
        echo ""
        echo "=========================================="
        echo "Registering operator $i ($OPERATOR)"
        echo "=========================================="
        
        # Step 0: Approve T tokens for TokenStaking (required before staking)
        echo ""
        echo "Step 0: Approving T tokens for TokenStaking..."
        
        # Extract keyfile path from config file
        KEYFILE=$(grep -i "^KeyFile" "$NODE_CONFIG" | head -1 | awk -F'=' '{print $2}' | tr -d ' "')
        if [ -z "$KEYFILE" ]; then
            echo "⚠ Warning: Could not find KeyFile in config: $NODE_CONFIG"
            echo "   Skipping approval. Staking may fail."
        else
            # Resolve relative path (config uses ./keystore/... which is relative to project root)
            if [[ "$KEYFILE" == ./* ]]; then
                # Remove leading ./ and resolve from project root
                KEYFILE="${KEYFILE#./}"
                KEYFILE="$(cd "$(dirname "$NODE_CONFIG")/.." && pwd)/$KEYFILE"
            elif [ ! -f "$KEYFILE" ]; then
                # Try relative to config directory
                CONFIG_DIR_ABS=$(cd "$(dirname "$NODE_CONFIG")" && pwd)
                KEYFILE_REL="$CONFIG_DIR_ABS/$KEYFILE"
                if [ -f "$KEYFILE_REL" ]; then
                    KEYFILE="$KEYFILE_REL"
                fi
            fi
            
            if [ ! -f "$KEYFILE" ]; then
                echo "⚠ Warning: Keyfile not found: $KEYFILE"
                echo "   Skipping approval. Staking may fail."
            fi
        fi
        
        if [ -f "$KEYFILE" ]; then
            # Resolve absolute path before changing directories
            ABS_KEYFILE=$(cd "$(dirname "$KEYFILE")" && pwd)/$(basename "$KEYFILE")
            
            cd solidity/ecdsa 2>/dev/null || cd ../solidity/ecdsa 2>/dev/null || {
                echo "⚠ Error: Could not find solidity/ecdsa directory"
                echo "Skipping approval. Staking may fail."
                cd - > /dev/null 2>&1 || true
            }
            
            if [ -d "." ]; then
                # Use Hardhat console to approve tokens (like fund-operators.sh)
                # This ensures Hardhat dependencies are available
                echo "   Checking current allowance and approving if needed..."
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
    
    // Read and decrypt keyfile
    const keyfile = JSON.parse(fs.readFileSync(keyfilePath, "utf8"));
    let wallet;
    try {
      wallet = await ethers.Wallet.fromEncryptedJson(JSON.stringify(keyfile), password);
    } catch (err) {
      console.error("Failed to decrypt keyfile:", err.message);
      process.exit(1);
    }
    
    // Connect wallet to provider
    const provider = new ethers.providers.JsonRpcProvider("http://localhost:8545");
    const operatorSigner = wallet.connect(provider);
    
    // Check current allowance
    const currentAllowance = await t.allowance(operatorSigner.address, staking.address);
    const { from1e18 } = helpers.number;
    
    console.log(\`Operator: \${operatorSigner.address}\`);
    console.log(\`Current allowance: \${from1e18(currentAllowance)} T\`);
    console.log(\`Requested amount: \${from1e18(stakeAmount)} T\`);
    
    if (currentAllowance.gte(stakeAmount)) {
      console.log("✓ Already approved");
      process.exit(0);
    }
    
    // Approve tokens
    console.log(\`Approving \${from1e18(stakeAmount)} T for TokenStaking (\${staking.address})...\`);
    const tWithSigner = t.connect(operatorSigner);
    const tx = await tWithSigner.approve(staking.address, stakeAmount);
    console.log(\`Transaction hash: \${tx.hash}\`);
    console.log("Waiting for confirmation...");
    await tx.wait();
    console.log("✓ Approval successful!");
    
    // Verify new allowance
    const newAllowance = await tWithSigner.allowance(operatorSigner.address, staking.address);
    console.log(\`New allowance: \${from1e18(newAllowance)} T\`);
    
    process.exit(0);
  } catch (error) {
    console.error("Error:", error.message);
    if (error.stack) {
      console.error(error.stack);
    }
    process.exit(1);
  }
})();
EOF
) || true
                
                if echo "$APPROVE_OUTPUT" | grep -qE "(Approval successful|Already approved)"; then
                    echo "$APPROVE_OUTPUT" | grep -E "(Approval successful|Already approved|Current allowance|New allowance|Transaction hash)"
                elif echo "$APPROVE_OUTPUT" | grep -qiE "(Error|error|invalid password|Could not|Failed|Failed to decrypt)"; then
                    echo "⚠ Approval failed:"
                    echo "$APPROVE_OUTPUT" | grep -iE "(Error|error|invalid password|Could not|Failed|Failed to decrypt)" | head -5
                    echo ""
                    echo "Full approval output:"
                    echo "$APPROVE_OUTPUT" | tail -15
                    echo ""
                    echo "   Will try staking anyway (may fail if approval needed)"
                else
                    echo "Approval output:"
                    echo "$APPROVE_OUTPUT"
                fi
            fi
            
            cd - > /dev/null 2>&1
            sleep 1
        else
            echo "⚠ Skipping approval (keyfile not found)"
        fi
        
        # Step 1: Stake tokens
        echo ""
        echo "Step 1: Staking tokens..."
        echo "   Amount: 100k T tokens"
        echo "   (This may take a few seconds - waiting for transaction confirmation...)"
        
        set +e  # Don't exit on command failure
        STAKE_OUTPUT=$(./keep-client ethereum threshold token-staking stake \
          "$OPERATOR" "$OPERATOR" "$OPERATOR" "$STAKE_AMOUNT" \
          --submit --config "$NODE_CONFIG" --developer 2>&1)
        STAKE_EXIT=$?
        set -e
        
        if [ $STAKE_EXIT -eq 0 ] && echo "$STAKE_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
            echo "✓ Staking transaction submitted:"
            echo "$STAKE_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
        elif echo "$STAKE_OUTPUT" | grep -qiE "(Error|error|failed|revert|FATAL)"; then
            echo "⚠ Staking failed:"
            echo "$STAKE_OUTPUT" | grep -iE "(Error|error|failed|revert|FATAL)" | head -5
            echo ""
            echo "Full output:"
            echo "$STAKE_OUTPUT" | tail -10
            echo ""
            if echo "$STAKE_OUTPUT" | grep -qiE "exceeds allowance|allowance"; then
                echo ""
                echo "⚠ Token approval needed. Using Hardhat initialize task instead..."
                echo "   (This handles minting, approval, staking, authorization, and registration)"
                cd solidity/ecdsa 2>/dev/null || cd ../solidity/ecdsa 2>/dev/null || {
                    echo "⚠ Error: Could not find solidity/ecdsa directory"
                    echo "Skipping remaining steps for this operator..."
                    continue
                }
                
                # Use Hardhat initialize (handles everything, but may fail on "unknown account")
                INIT_OUTPUT=$(npx hardhat initialize \
                  --network development \
                  --owner "$OPERATOR" \
                  --provider "$OPERATOR" \
                  --operator "$OPERATOR" \
                  --beneficiary "$OPERATOR" \
                  --authorizer "$OPERATOR" \
                  --amount 50000 2>&1) || true
                
                cd - > /dev/null 2>&1
                
                if echo "$INIT_OUTPUT" | grep -qiE "(unknown account|Error)"; then
                    echo "⚠ Hardhat initialize failed (unknown account issue)"
                    echo "   You may need to:"
                    echo "   1. Configure Hardhat development network with operator accounts"
                    echo "   2. Or approve tokens manually using a script"
                    echo "   3. Or use a different registration method"
                    echo ""
                    echo "Skipping remaining steps for this operator..."
                    continue
                elif echo "$INIT_OUTPUT" | grep -qE "(transaction|hash|Initialized|Staked|Authorized|Registered)"; then
                    echo "✓ Hardhat initialize completed:"
                    echo "$INIT_OUTPUT" | grep -E "(transaction|hash|Initialized|Staked|Authorized|Registered)" | head -5
                    echo ""
                    echo "✓ Operator $i registration complete (via Hardhat)!"
                    continue
                fi
            else
                echo "Common issues:"
                echo "  - Account needs ETH for gas (run: ./scripts/fund-operators.sh)"
                echo "  - Account needs T tokens (should already have 1M T)"
                echo ""
                echo "Skipping remaining steps for this operator..."
                continue
            fi
        else
            echo "Output:"
            echo "$STAKE_OUTPUT"
            if [ $STAKE_EXIT -ne 0 ]; then
                echo "⚠ Command exited with code $STAKE_EXIT"
                echo "Skipping remaining steps for this operator..."
                continue
            fi
        fi
        
        sleep 2
        
        # Step 2: Authorize WalletRegistry application
        echo ""
        echo "Step 2: Authorizing WalletRegistry application..."
        
        set +e  # Don't exit on command failure
        AUTH_OUTPUT=$(./keep-client ethereum threshold token-staking increase-authorization \
          "$OPERATOR" "$WALLET_REGISTRY" "$AUTHORIZATION_AMOUNT" \
          --submit --config "$NODE_CONFIG" --developer 2>&1)
        AUTH_EXIT=$?
        set -e
        
        if [ $AUTH_EXIT -eq 0 ] && echo "$AUTH_OUTPUT" | grep -qE "(transaction|hash|already)"; then
            echo "$AUTH_OUTPUT" | grep -E "(transaction|hash|already)"
        elif echo "$AUTH_OUTPUT" | grep -qiE "(Error|error|failed|revert|FATAL)"; then
            echo "⚠ Authorization failed:"
            echo "$AUTH_OUTPUT" | grep -iE "(Error|error|failed|revert|FATAL)" | head -5
            echo "Full output:"
            echo "$AUTH_OUTPUT" | tail -10
            echo "⚠ Skipping registration step..."
            continue
        else
            echo "$AUTH_OUTPUT"
            if [ $AUTH_EXIT -ne 0 ]; then
                echo "⚠ Command exited with code $AUTH_EXIT"
                echo "⚠ Skipping registration step..."
                continue
            fi
        fi
        
        sleep 2
        
        # Step 3: Register operator in RandomBeacon (required for node startup)
        echo ""
        echo "Step 3a: Registering operator in RandomBeacon..."
        
        set +e  # Don't exit on command failure
        RB_REG_OUTPUT=$(./keep-client ethereum beacon random-beacon register-operator \
          "$OPERATOR" \
          --submit --config "$NODE_CONFIG" --developer 2>&1)
        RB_REG_EXIT=$?
        set -e
        
        if [ $RB_REG_EXIT -eq 0 ] && echo "$RB_REG_OUTPUT" | grep -qE "(transaction|hash|0x[0-9a-f]{64})"; then
            echo "✓ RandomBeacon registration transaction submitted:"
            echo "$RB_REG_OUTPUT" | grep -E "(transaction|hash|0x[0-9a-f]{64})" | head -1
        elif echo "$RB_REG_OUTPUT" | grep -qiE "(already in use|already set)"; then
            echo "✓ Operator already registered in RandomBeacon"
        elif echo "$RB_REG_OUTPUT" | grep -qiE "(Error|error|failed|revert|FATAL)"; then
            echo "⚠ RandomBeacon registration failed:"
            echo "$RB_REG_OUTPUT" | grep -iE "(Error|error|failed|revert|FATAL)" | head -3
            echo "   (This may be OK if already registered)"
        fi
        
        sleep 2
        
        # Step 3b: Register operator in WalletRegistry (required for ECDSA/DKG)
        echo ""
        echo "Step 3b: Registering operator in WalletRegistry..."
        
        set +e  # Don't exit on command failure
        REG_OUTPUT=$(./keep-client ethereum ecdsa wallet-registry register-operator \
          "$OPERATOR" \
          --submit --config "$NODE_CONFIG" --developer 2>&1)
        REG_EXIT=$?
        set -e
        
        if [ $REG_EXIT -eq 0 ] && echo "$REG_OUTPUT" | grep -qE "(transaction|hash|already)"; then
            echo "$REG_OUTPUT" | grep -E "(transaction|hash|already)"
        elif echo "$REG_OUTPUT" | grep -qiE "(Error|error|failed|revert|FATAL)"; then
            echo "⚠ Registration failed:"
            echo "$REG_OUTPUT" | grep -iE "(Error|error|failed|revert|FATAL)" | head -5
            echo "Full output:"
            echo "$REG_OUTPUT" | tail -10
        else
            echo "$REG_OUTPUT"
            if [ $REG_EXIT -ne 0 ]; then
                echo "⚠ Command exited with code $REG_EXIT"
            fi
        fi
        
        sleep 2
        
        echo ""
        echo "✓ Operator $i registration complete!"
    done
    
    echo ""
    echo "=========================================="
    echo "✓ All operators registered!"
    echo "=========================================="
else
    echo "Skipping automated registration."
    echo ""
    echo "You can register operators manually using keep-client CLI:"
    echo ""
    for i in $(seq 1 $NUM_NODES); do
        OPERATOR="${OPERATOR_ADDRESSES[$i]}"
        if [ -z "$OPERATOR" ]; then
            continue
        fi
        echo "  # For operator $i ($OPERATOR):"
        echo "  # 1. Stake tokens (100k T)"
        echo "  ./keep-client ethereum threshold token-staking stake \\"
        echo "    $OPERATOR $OPERATOR $OPERATOR 100000000000000000000000 \\"
        echo "    --submit --config $CONFIG_DIR/node${i}.toml --developer"
        echo ""
        echo "  # 2. Authorize WalletRegistry (40k T minimum)"
        echo "  ./keep-client ethereum threshold token-staking increase-authorization \\"
        echo "    $OPERATOR $WALLET_REGISTRY 40000000000000000000000 \\"
        echo "    --submit --config $CONFIG_DIR/node${i}.toml --developer"
        echo ""
        echo "  # 3. Register operator"
        echo "  ./keep-client ethereum ecdsa wallet-registry register-operator \\"
        echo "    $OPERATOR \\"
        echo "    --submit --config $CONFIG_DIR/node${i}.toml --developer"
        echo ""
    done
fi

echo ""

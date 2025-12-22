#!/bin/bash
set -e

# Script to generate genesis.json file with all accounts from keystore

GENESIS_FILE="genesis.json"
KEYSTORE_DIR="${GETH_DATA_DIR:-$HOME/ethereum/data}/keystore"

echo "=== Genesis File Generator ==="
echo "Keystore directory: $KEYSTORE_DIR"
echo ""

# Check if keystore directory exists
if [ ! -d "$KEYSTORE_DIR" ]; then
    echo "ERROR: Keystore directory not found: $KEYSTORE_DIR"
    echo "Please create accounts first or set GETH_DATA_DIR environment variable"
    exit 1
fi

# Extract account addresses
echo "Extracting account addresses..."
ACCOUNTS=$(geth account list --keystore "$KEYSTORE_DIR" 2>/dev/null | grep -o '{[^}]*}' | sed 's/{//;s/}//')

if [ -z "$ACCOUNTS" ]; then
    echo "ERROR: No accounts found in keystore directory"
    exit 1
fi

ACCOUNT_COUNT=$(echo "$ACCOUNTS" | wc -l | tr -d ' ')
echo "Found $ACCOUNT_COUNT accounts"

# Generate genesis.json
echo "Generating $GENESIS_FILE..."

cat > "$GENESIS_FILE" << 'GENESIS_HEAD'
{
    "config": {
        "chainId": 1101,
        "eip150Block": 0,
        "eip155Block": 0,
        "eip158Block": 0,
        "byzantiumBlock": 0,
        "homesteadBlock": 0,
        "constantinopleBlock": 0,
        "petersburgBlock": 0,
        "daoForkBlock": 0,
        "istanbulBlock": 0,
        "daoForkSupport": true,
        "terminalTotalDifficulty": null
    },
    "difficulty": "0x20",
    "gasLimit": "0x7A1200",
    "alloc": {
GENESIS_HEAD

# Add accounts to alloc section
FIRST=true
for addr in $ACCOUNTS; do
    if [ "$FIRST" = true ]; then
        FIRST=false
    else
        echo ',' >> "$GENESIS_FILE"
    fi
    echo "        \"0x$addr\": { \"balance\": \"1000000000000000000000000000000000000000000000000000000\" }" | tr -d '\n' >> "$GENESIS_FILE"
done

cat >> "$GENESIS_FILE" << 'GENESIS_TAIL'

    }
}
GENESIS_TAIL

echo ""
echo "✓ Successfully generated $GENESIS_FILE with $ACCOUNT_COUNT accounts"
echo ""
echo "Next steps:"
echo "1. Set environment variables:"
echo "   export GETH_DATA_DIR=~/ethereum/data"
echo "   export GETH_ETHEREUM_ACCOUNT=0x$(echo \"$ACCOUNTS\" | head -1)"
echo ""
echo "2. Initialize the chain:"
echo "   geth --datadir=\$GETH_DATA_DIR init $GENESIS_FILE"
echo ""
echo "3. Start Geth with mining enabled"


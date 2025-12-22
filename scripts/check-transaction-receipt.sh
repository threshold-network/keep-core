#!/bin/bash
# Script to check transaction receipt by hash
# Usage: ./scripts/check-transaction-receipt.sh <tx-hash> [tx-hash2] ...
# Example: ./scripts/check-transaction-receipt.sh 0x1234... 0x5678...

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Function to check a single transaction
check_transaction() {
    local tx_hash="$1"
    
    # Validate hash format
    if ! echo "$tx_hash" | grep -qE '^0x[0-9a-fA-F]{64}$'; then
        echo -e "${RED}✗ Invalid transaction hash format: $tx_hash${NC}"
        echo "  Expected format: 0x followed by 64 hex characters"
        return 1
    fi
    
    echo ""
    echo "=========================================="
    echo "Transaction: $tx_hash"
    echo "=========================================="
    
    # Use direct JSON-RPC calls for faster and more reliable checks
    GETH_URL="${GETH_URL:-http://localhost:8545}"
    
    # Get transaction receipt via JSON-RPC
    RECEIPT_JSON=$(curl -s -X POST "$GETH_URL" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$tx_hash\"],\"id\":1}" 2>&1)
    
    # Check if receipt exists (null means pending or not found)
    if echo "$RECEIPT_JSON" | grep -q '"result":null'; then
        echo ""
        echo -e "  ${YELLOW}⏳ Status: PENDING${NC}"
        echo "  Transaction not yet mined or hash not found"
        
        # Try to get transaction to see if it's pending
        TX_JSON=$(curl -s -X POST "$GETH_URL" \
            -H "Content-Type: application/json" \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionByHash\",\"params\":[\"$tx_hash\"],\"id\":1}" 2>&1)
        
        if echo "$TX_JSON" | grep -q '"result":null'; then
            echo "  Transaction not found in mempool or blockchain"
        else
            echo "  Transaction found in mempool (pending)"
        fi
        return 0
    fi
    
    # Check for JSON-RPC error
    if echo "$RECEIPT_JSON" | grep -q '"error"'; then
        ERROR_MSG=$(echo "$RECEIPT_JSON" | grep -o '"message":"[^"]*"' | cut -d'"' -f4 || echo "Unknown error")
        echo -e "  ${RED}✗ Error: $ERROR_MSG${NC}"
        return 1
    fi
    
    # Parse receipt JSON (requires jq or manual parsing)
    if command -v jq >/dev/null 2>&1; then
        STATUS_HEX=$(echo "$RECEIPT_JSON" | jq -r '.result.status // empty')
        BLOCK_NUMBER_HEX=$(echo "$RECEIPT_JSON" | jq -r '.result.blockNumber // empty')
        BLOCK_HASH=$(echo "$RECEIPT_JSON" | jq -r '.result.blockHash // empty')
        GAS_USED_HEX=$(echo "$RECEIPT_JSON" | jq -r '.result.gasUsed // empty')
        CUMULATIVE_GAS_HEX=$(echo "$RECEIPT_JSON" | jq -r '.result.cumulativeGasUsed // empty')
        EFFECTIVE_GAS_PRICE_HEX=$(echo "$RECEIPT_JSON" | jq -r '.result.effectiveGasPrice // empty')
        FROM=$(echo "$RECEIPT_JSON" | jq -r '.result.from // empty')
        TO=$(echo "$RECEIPT_JSON" | jq -r '.result.to // empty')
        TX_HASH=$(echo "$RECEIPT_JSON" | jq -r '.result.transactionHash // empty')
        TX_INDEX_HEX=$(echo "$RECEIPT_JSON" | jq -r '.result.transactionIndex // empty')
        LOGS_COUNT=$(echo "$RECEIPT_JSON" | jq -r '.result.logs | length')
        
        # Convert hex to decimal
        STATUS=$((16#${STATUS_HEX#0x}))
        BLOCK_NUMBER=$((16#${BLOCK_NUMBER_HEX#0x}))
        GAS_USED=$((16#${GAS_USED_HEX#0x}))
        CUMULATIVE_GAS=$((16#${CUMULATIVE_GAS_HEX#0x}))
        EFFECTIVE_GAS_PRICE=$((16#${EFFECTIVE_GAS_PRICE_HEX#0x}))
        TX_INDEX=$((16#${TX_INDEX_HEX#0x}))
        
        # Get transaction details
        TX_JSON=$(curl -s -X POST "$GETH_URL" \
            -H "Content-Type: application/json" \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionByHash\",\"params\":[\"$tx_hash\"],\"id\":1}" 2>&1)
        
        GAS_LIMIT_HEX=$(echo "$TX_JSON" | jq -r '.result.gas // empty')
        GAS_PRICE_HEX=$(echo "$TX_JSON" | jq -r '.result.gasPrice // empty')
        VALUE_HEX=$(echo "$TX_JSON" | jq -r '.result.value // empty')
        NONCE_HEX=$(echo "$TX_JSON" | jq -r '.result.nonce // empty')
        
        GAS_LIMIT=$((16#${GAS_LIMIT_HEX#0x}))
        GAS_PRICE=$((16#${GAS_PRICE_HEX#0x}))
        VALUE=$((16#${VALUE_HEX#0x}))
        NONCE=$((16#${NONCE_HEX#0x}))
        
    else
        # Fallback: parse JSON manually (basic parsing)
        echo -e "  ${YELLOW}⚠ Warning: jq not found. Install jq for better output.${NC}"
        echo "  Using basic parsing..."
        
        STATUS_HEX=$(echo "$RECEIPT_JSON" | grep -o '"status":"0x[0-9a-f]*"' | cut -d'"' -f4 || echo "")
        STATUS=$((16#${STATUS_HEX#0x}))
        BLOCK_NUMBER_HEX=$(echo "$RECEIPT_JSON" | grep -o '"blockNumber":"0x[0-9a-f]*"' | cut -d'"' -f4 || echo "")
        BLOCK_NUMBER=$((16#${BLOCK_NUMBER_HEX#0x}))
        GAS_USED_HEX=$(echo "$RECEIPT_JSON" | grep -o '"gasUsed":"0x[0-9a-f]*"' | cut -d'"' -f4 || echo "")
        GAS_USED=$((16#${GAS_USED_HEX#0x}))
        FROM=$(echo "$RECEIPT_JSON" | grep -o '"from":"0x[0-9a-f]*"' | cut -d'"' -f4 || echo "N/A")
        TO=$(echo "$RECEIPT_JSON" | grep -o '"to":"0x[0-9a-f]*"' | cut -d'"' -f4 || echo "Contract Creation")
        LOGS_COUNT=$(echo "$RECEIPT_JSON" | grep -o '"logs":\[' | wc -l | tr -d ' ')
    fi
    
    # Display results
    echo ""
    if [ "$STATUS" = "1" ]; then
        echo -e "  ${GREEN}✓ Status: SUCCESS${NC}"
    elif [ "$STATUS" = "0" ]; then
        echo -e "  ${RED}✗ Status: FAILED${NC}"
    else
        echo -e "  ${YELLOW}⏳ Status: UNKNOWN${NC}"
    fi
    
    echo "  Block Number: $BLOCK_NUMBER"
    echo "  Block Hash: $BLOCK_HASH"
    echo "  Gas Used: $GAS_USED"
    if [ -n "${CUMULATIVE_GAS:-}" ]; then
        echo "  Cumulative Gas Used: $CUMULATIVE_GAS"
    fi
    if [ -n "${EFFECTIVE_GAS_PRICE:-}" ] && [ "$EFFECTIVE_GAS_PRICE" != "0" ]; then
        echo "  Effective Gas Price: $EFFECTIVE_GAS_PRICE wei"
    fi
    echo "  From: $FROM"
    if [ "$TO" != "null" ] && [ -n "$TO" ]; then
        echo "  To: $TO"
    else
        echo "  To: Contract Creation"
    fi
    echo "  Transaction Hash: $TX_HASH"
    if [ -n "${TX_INDEX:-}" ]; then
        echo "  Transaction Index: $TX_INDEX"
    fi
    echo "  Events: $LOGS_COUNT"
    
    # Show full details if requested
    if [ "${VERBOSE:-}" = "1" ]; then
        echo ""
        echo "  Full Details:"
        if command -v jq >/dev/null 2>&1; then
            echo "$RECEIPT_JSON" | jq '.result' | sed 's/^/    /'
        else
            echo "$RECEIPT_JSON" | sed 's/^/    /'
        fi
        
        if [ -n "${GAS_LIMIT:-}" ] && [ "$GAS_LIMIT" != "0" ]; then
            echo "    Gas Limit: $GAS_LIMIT"
        fi
        if [ -n "${GAS_PRICE:-}" ] && [ "$GAS_PRICE" != "0" ]; then
            echo "    Gas Price: $GAS_PRICE wei"
        fi
        if [ -n "${VALUE:-}" ]; then
            echo "    Value: $VALUE wei ($(echo "scale=6; $VALUE / 1000000000000000000" | bc) ETH)"
        fi
        if [ -n "${NONCE:-}" ]; then
            echo "    Nonce: $NONCE"
        fi
    fi
    
    # Show error if transaction failed
    if [ "$STATUS" = "0" ]; then
        echo ""
        echo -e "  ${RED}⚠ Transaction reverted!${NC}"
        echo "  Check the transaction on-chain for revert reason."
    fi
}

# Main execution
if [ $# -eq 0 ]; then
    echo "Usage: $0 <tx-hash> [tx-hash2] ..."
    echo ""
    echo "Check transaction receipt(s) by hash"
    echo ""
    echo "Examples:"
    echo "  $0 0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
    echo "  $0 0x1234... 0x5678...  # Check multiple transactions"
    echo ""
    echo "Environment variables:"
    echo "  VERBOSE=1  Show full transaction details"
    echo ""
    exit 1
fi

echo "=========================================="
echo "Transaction Receipt Checker"
echo "=========================================="

# Check if Geth is running
if ! curl -s http://localhost:8545 > /dev/null 2>&1; then
    echo -e "${RED}✗ Error: Cannot connect to Geth at http://localhost:8545${NC}"
    echo "  Make sure Geth is running"
    exit 1
fi

# Process each transaction hash
ALL_SUCCESS=true
for tx_hash in "$@"; do
    if ! check_transaction "$tx_hash"; then
        ALL_SUCCESS=false
    fi
done

echo ""
echo "=========================================="
if [ "$ALL_SUCCESS" = true ]; then
    echo -e "${GREEN}✓ All transactions checked${NC}"
else
    echo -e "${YELLOW}⚠ Some transactions had errors${NC}"
fi
echo "=========================================="
echo ""
echo "Tip: Use VERBOSE=1 to see full details:"
echo "  VERBOSE=1 $0 <tx-hash>"

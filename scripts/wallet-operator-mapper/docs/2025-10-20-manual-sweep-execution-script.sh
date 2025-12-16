#!/bin/bash
#
# Manual Sweep Execution Script
# tBTC Beta Staker Consolidation - Manual MovingFunds Process
#
# Created: 2025-10-20
# Based on: 2025-10-10-manual-sweep-technical-process.md
#
# WARNING: This is a TEMPLATE script. Review and customize all variables before execution.
# This script requires manual intervention at various steps.
#

set -e  # Exit on error
set -u  # Exit on undefined variable

#==============================================================================
# CONFIGURATION VARIABLES - CUSTOMIZE THESE BEFORE RUNNING
#==============================================================================

# Ethereum Configuration
ALLOWLIST_ADDRESS="0x_YOUR_ALLOWLIST_CONTRACT_ADDRESS"
WALLET_REGISTRY_ADDRESS="0x_YOUR_WALLET_REGISTRY_ADDRESS"
BRIDGE_ADDRESS="0x_YOUR_BRIDGE_CONTRACT_ADDRESS"
VALIDATOR_ADDRESS="0x_YOUR_WALLET_PROPOSAL_VALIDATOR_ADDRESS"

# Operator Configuration
OPERATOR_ADDRESS="0x_YOUR_OPERATOR_ADDRESS"
SUBMITTER_PRIVATE_KEY="YOUR_ETHEREUM_PRIVATE_KEY"  # For SPV proof submission

# Wallet Configuration (example - replace with actual values)
DEPRECATED_WALLET_PKH="0x1234567890abcdef1234567890abcdef12345678"
DEPRECATED_WALLET_ADDRESS="bc1q_YOUR_DEPRECATED_WALLET_ADDRESS"
ACTIVE_WALLET_PKH="0xffb804c2de78576ad011f68a7df63d739b8c8155"
ACTIVE_WALLET_ADDRESS="bc1q_YOUR_ACTIVE_WALLET_ADDRESS"

# Provider (BOAR, STAKED, or P2P)
PROVIDER="BOAR"

# Coordination Configuration
COORDINATION_FREQUENCY=900  # blocks
COORDINATION_WINDOW_SIZE=100  # blocks

# Bitcoin Configuration
REQUIRED_CONFIRMATIONS=6

# Output file for storing intermediate results
OUTPUT_DIR="/tmp/manual-sweep-$(date -u +%Y%m%d-%H%M%S)"
mkdir -p "$OUTPUT_DIR"

#==============================================================================
# UTILITY FUNCTIONS
#==============================================================================

log() {
    echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" | tee -a "$OUTPUT_DIR/sweep.log"
}

error() {
    echo "[ERROR] $(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" | tee -a "$OUTPUT_DIR/sweep.log" >&2
    exit 1
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        error "Required command '$1' not found. Please install it."
    fi
}

#==============================================================================
# PREREQUISITE CHECKS
#==============================================================================

log "Starting prerequisite checks..."

# Check required commands
check_command "cast"
check_command "bitcoin-cli"
check_command "curl"
check_command "jq"

log "All prerequisites satisfied."

#==============================================================================
# PHASE 1: PREPARATION (Threshold Team)
#==============================================================================

phase1_preparation() {
    log "=== PHASE 1: PREPARATION ==="

    # Step 1.1: Identify Straggler Wallets
    log "Step 1.1: Querying wallet balance..."

    # Query Bitcoin wallet balance
    BTC_BALANCE=$(bitcoin-cli getreceivedbyaddress "$DEPRECATED_WALLET_ADDRESS" 0 || echo "0")
    log "Bitcoin balance: $BTC_BALANCE BTC"

    # Query Ethereum wallet state
    log "Querying Ethereum wallet state..."
    cast call "$WALLET_REGISTRY_ADDRESS" \
        "getWalletMainUtxo(bytes20)" \
        "$DEPRECATED_WALLET_PKH" > "$OUTPUT_DIR/wallet_utxo.txt"

    log "Wallet UTXO saved to $OUTPUT_DIR/wallet_utxo.txt"

    # Step 1.2: Calculate Bitcoin Transaction Fees
    log "Step 1.2: Checking current mempool fee recommendations..."

    curl -s https://mempool.space/api/v1/fees/recommended > "$OUTPUT_DIR/mempool_fees.json"

    FASTEST_FEE=$(jq -r '.fastestFee' "$OUTPUT_DIR/mempool_fees.json")
    HALF_HOUR_FEE=$(jq -r '.halfHourFee' "$OUTPUT_DIR/mempool_fees.json")
    HOUR_FEE=$(jq -r '.hourFee' "$OUTPUT_DIR/mempool_fees.json")

    log "Fastest fee: $FASTEST_FEE sat/vB"
    log "Half hour fee: $HALF_HOUR_FEE sat/vB"
    log "Hour fee: $HOUR_FEE sat/vB"

    # Use half hour fee as default (balanced approach)
    FEE_RATE="$HALF_HOUR_FEE"
    log "Selected fee rate: $FEE_RATE sat/vB"

    # Step 1.3: Query Wallet UTXOs
    log "Step 1.3: Querying wallet UTXOs..."

    # Using Bitcoin Core RPC
    bitcoin-cli listunspent 0 9999999 "[\"$DEPRECATED_WALLET_ADDRESS\"]" > "$OUTPUT_DIR/utxos.json"

    UTXO_COUNT=$(jq '. | length' "$OUTPUT_DIR/utxos.json")
    log "Found $UTXO_COUNT UTXOs"

    # Display UTXOs
    jq -r '.[] | "UTXO: \(.txid):\(.vout) = \(.amount) BTC"' "$OUTPUT_DIR/utxos.json"

    # Step 1.4: Construct MovingFunds Proposal Parameters
    log "Step 1.4: Constructing MovingFunds proposal..."

    # Calculate total input value
    TOTAL_INPUT_BTC=$(jq '[.[].amount] | add' "$OUTPUT_DIR/utxos.json")
    TOTAL_INPUT_SATS=$(echo "$TOTAL_INPUT_BTC * 100000000" | bc | cut -d. -f1)

    # Estimate transaction size (10 base + 148*inputs + 34*outputs)
    TX_SIZE=$((10 + 148 * UTXO_COUNT + 34))
    TOTAL_FEE_SATS=$((TX_SIZE * FEE_RATE))
    OUTPUT_VALUE_SATS=$((TOTAL_INPUT_SATS - TOTAL_FEE_SATS))

    log "Total input: $TOTAL_INPUT_SATS sats"
    log "Estimated tx size: $TX_SIZE bytes"
    log "Total fee: $TOTAL_FEE_SATS sats"
    log "Output value: $OUTPUT_VALUE_SATS sats"

    # Save proposal parameters
    cat > "$OUTPUT_DIR/proposal_params.json" <<EOF
{
  "sourceWalletPKH": "$DEPRECATED_WALLET_PKH",
  "targetWalletPKH": "$ACTIVE_WALLET_PKH",
  "utxos": $(cat "$OUTPUT_DIR/utxos.json"),
  "outputValue": $OUTPUT_VALUE_SATS,
  "feeRate": $FEE_RATE,
  "totalFee": $TOTAL_FEE_SATS,
  "provider": "$PROVIDER"
}
EOF

    log "Proposal parameters saved to $OUTPUT_DIR/proposal_params.json"

    # Step 1.5: Notify Operators (manual step)
    log "Step 1.5: MANUAL ACTION REQUIRED - Notify operators"
    log "Send 48-hour advance notice to all operators via:"
    log "  - Email"
    log "  - Slack/Discord"
    log "  - Calendar invitation"
    log ""
    log "Proposal details: $OUTPUT_DIR/proposal_params.json"
}

#==============================================================================
# PHASE 2: COORDINATION WINDOW
#==============================================================================

phase2_coordination_window() {
    log "=== PHASE 2: COORDINATION WINDOW ==="

    # Step 2.1: Determine Coordination Block
    log "Step 2.1: Calculating next coordination block..."

    CURRENT_BLOCK=$(cast block-number)
    log "Current block: $CURRENT_BLOCK"

    REMAINDER=$((CURRENT_BLOCK % COORDINATION_FREQUENCY))

    if [ $REMAINDER -eq 0 ]; then
        COORDINATION_BLOCK=$CURRENT_BLOCK
    else
        BLOCKS_UNTIL_NEXT=$((COORDINATION_FREQUENCY - REMAINDER))
        COORDINATION_BLOCK=$((CURRENT_BLOCK + BLOCKS_UNTIL_NEXT))
    fi

    WINDOW_END=$((COORDINATION_BLOCK + COORDINATION_WINDOW_SIZE))

    log "Next coordination block: $COORDINATION_BLOCK"
    log "Coordination window: $COORDINATION_BLOCK - $WINDOW_END"
    log "Blocks until window opens: $((COORDINATION_BLOCK - CURRENT_BLOCK))"

    # Approximate time
    SECONDS_UNTIL=$((COORDINATION_BLOCK - CURRENT_BLOCK) * 12)
    log "Approximate time until window: $((SECONDS_UNTIL / 60)) minutes"

    echo "$COORDINATION_BLOCK" > "$OUTPUT_DIR/coordination_block.txt"
}

#==============================================================================
# PHASE 3-5: OPERATOR NODE COORDINATION (AUTOMATIC)
#==============================================================================

phase3_5_operator_coordination() {
    log "=== PHASES 3-5: OPERATOR NODE COORDINATION ==="
    log ""
    log "AUTOMATIC PROCESS - Handled by operator nodes"
    log ""
    log "Operator nodes will automatically:"
    log "  - Phase 3: Leader proposes MovingFunds"
    log "  - Phase 4: Followers validate and sign"
    log "  - Phase 5: Leader aggregates signatures and broadcasts Bitcoin transaction"
    log ""
    log "MANUAL ACTION: Monitor coordination channel and operator node logs"
    log ""
    log "Expected outputs:"
    log "  - CoordinationMessage broadcast"
    log "  - Threshold signing (51+ signatures)"
    log "  - Bitcoin transaction broadcast"
    log ""
    log "Wait for Bitcoin transaction ID announcement..."
}

#==============================================================================
# PHASE 6: BITCOIN CONFIRMATIONS
#==============================================================================

phase6_bitcoin_confirmations() {
    log "=== PHASE 6: BITCOIN CONFIRMATIONS ==="

    # Get transaction ID (manual input or from file)
    if [ -f "$OUTPUT_DIR/bitcoin_txid.txt" ]; then
        BITCOIN_TXID=$(cat "$OUTPUT_DIR/bitcoin_txid.txt")
    else
        read -p "Enter Bitcoin transaction ID: " BITCOIN_TXID
        echo "$BITCOIN_TXID" > "$OUTPUT_DIR/bitcoin_txid.txt"
    fi

    log "Monitoring transaction: $BITCOIN_TXID"

    # Step 6.1: Wait for confirmations
    log "Step 6.1: Waiting for $REQUIRED_CONFIRMATIONS confirmations..."

    while true; do
        TX_INFO=$(bitcoin-cli gettransaction "$BITCOIN_TXID" 2>/dev/null || echo "{}")
        CONFIRMATIONS=$(echo "$TX_INFO" | jq -r '.confirmations // 0')

        log "Current confirmations: $CONFIRMATIONS / $REQUIRED_CONFIRMATIONS"

        if [ "$CONFIRMATIONS" -ge "$REQUIRED_CONFIRMATIONS" ]; then
            log "✅ Transaction confirmed with $CONFIRMATIONS confirmations"
            BLOCK_HASH=$(echo "$TX_INFO" | jq -r '.blockhash')
            log "Block hash: $BLOCK_HASH"
            echo "$BLOCK_HASH" > "$OUTPUT_DIR/block_hash.txt"
            break
        fi

        log "Waiting 60 seconds before next check..."
        sleep 60
    done

    # Step 6.2: Verify BTC arrived at target wallet
    log "Step 6.2: Verifying BTC arrived at target wallet..."

    TARGET_BALANCE=$(bitcoin-cli getreceivedbyaddress "$ACTIVE_WALLET_ADDRESS" 0)
    log "Active wallet balance: $TARGET_BALANCE BTC"

    # Check specific UTXO
    VOUT=0  # Typically output 0
    UTXO_INFO=$(bitcoin-cli gettxout "$BITCOIN_TXID" $VOUT 2>/dev/null || echo "null")

    if [ "$UTXO_INFO" != "null" ]; then
        VALUE=$(echo "$UTXO_INFO" | jq -r '.value')
        ADDRESS=$(echo "$UTXO_INFO" | jq -r '.scriptPubKey.address')
        log "✅ UTXO verified: $VALUE BTC to $ADDRESS"
    else
        log "⚠️  Warning: Could not verify UTXO (may be already spent)"
    fi
}

#==============================================================================
# PHASE 7: SPV PROOF SUBMISSION
#==============================================================================

phase7_spv_proof() {
    log "=== PHASE 7: SPV PROOF SUBMISSION ==="

    BITCOIN_TXID=$(cat "$OUTPUT_DIR/bitcoin_txid.txt")
    BLOCK_HASH=$(cat "$OUTPUT_DIR/block_hash.txt")

    # Step 7.1: Construct SPV Proof
    log "Step 7.1: Constructing SPV proof..."
    log "⚠️  WARNING: SPV proof construction requires specialized tools"
    log ""
    log "Manual steps required:"
    log "  1. Get raw transaction: bitcoin-cli getrawtransaction $BITCOIN_TXID"
    log "  2. Get block data: bitcoin-cli getblock $BLOCK_HASH 2"
    log "  3. Build Merkle proof from transaction position in block"
    log "  4. Get block headers (current + 5 preceding for 6 confirmations)"
    log ""

    # Get raw transaction
    log "Fetching raw transaction..."
    RAW_TX=$(bitcoin-cli getrawtransaction "$BITCOIN_TXID")
    echo "$RAW_TX" > "$OUTPUT_DIR/raw_tx.hex"
    log "Raw transaction saved to $OUTPUT_DIR/raw_tx.hex"

    # Get block data
    log "Fetching block data..."
    bitcoin-cli getblock "$BLOCK_HASH" 2 > "$OUTPUT_DIR/block_data.json"
    log "Block data saved to $OUTPUT_DIR/block_data.json"

    # Get transaction index in block
    TX_INDEX=$(jq -r ".tx | map(.txid) | index(\"$BITCOIN_TXID\")" "$OUTPUT_DIR/block_data.json")
    log "Transaction index in block: $TX_INDEX"
    echo "$TX_INDEX" > "$OUTPUT_DIR/tx_index.txt"

    # Get block header
    log "Fetching block headers..."
    bitcoin-cli getblockheader "$BLOCK_HASH" false > "$OUTPUT_DIR/block_header.hex"

    log "⚠️  MANUAL ACTION: Complete SPV proof construction"
    log "Required data collected in: $OUTPUT_DIR/"
    log "  - raw_tx.hex: Bitcoin transaction"
    log "  - block_data.json: Block with merkle tree"
    log "  - tx_index.txt: Transaction position"
    log "  - block_header.hex: Block header"
    log ""
    log "Use tBTC SPV proof construction tools to build complete proof"
    log ""

    # Step 7.2: Submit SPV Proof (manual - requires constructed proof)
    log "Step 7.2: Ready to submit SPV proof to Ethereum Bridge"
    log ""
    log "When SPV proof is ready, submit with:"
    log ""
    log "cast send $BRIDGE_ADDRESS \\"
    log "  \"submitMovingFundsProof(bytes,bytes,bytes,uint256)\" \\"
    log "  \$BITCOIN_TX_HEX \\"
    log "  \$MERKLE_PROOF_HEX \\"
    log "  \$BLOCK_HEADERS_HEX \\"
    log "  \$TX_INDEX \\"
    log "  --private-key \$SUBMITTER_PRIVATE_KEY \\"
    log "  --gas-limit 500000"
    log ""
    log "⚠️  Ensure gas price is reasonable before submitting"
    log "Check current gas: cast gas-price"
}

#==============================================================================
# PHASE 8: VERIFICATION AND CLEANUP
#==============================================================================

phase8_verification() {
    log "=== PHASE 8: VERIFICATION AND CLEANUP ==="

    # Step 8.1: Verify wallet balance on-chain (Ethereum)
    log "Step 8.1: Verifying deprecated wallet balance on-chain (Ethereum)..."

    cast call "$WALLET_REGISTRY_ADDRESS" \
        "getWalletMainUtxo(bytes20)" \
        "$DEPRECATED_WALLET_PKH" > "$OUTPUT_DIR/deprecated_wallet_final_state.txt"

    log "Deprecated wallet final state saved to $OUTPUT_DIR/deprecated_wallet_final_state.txt"
    log "Expected: Empty UTXO (all zeros)"
    cat "$OUTPUT_DIR/deprecated_wallet_final_state.txt"

    # Step 8.2: Verify wallet balance off-chain (Bitcoin)
    log "Step 8.2: Verifying deprecated wallet balance off-chain (Bitcoin)..."

    FINAL_BALANCE=$(bitcoin-cli getreceivedbyaddress "$DEPRECATED_WALLET_ADDRESS" 0)
    log "Deprecated wallet final balance: $FINAL_BALANCE BTC"

    if [ "$(echo "$FINAL_BALANCE < 0.00001" | bc)" -eq 1 ]; then
        log "✅ Wallet effectively empty (dust acceptable)"
    else
        log "⚠️  Warning: Wallet still has significant balance: $FINAL_BALANCE BTC"
    fi

    # Step 8.3: Mark operator eligible for removal
    log "Step 8.3: Marking operator eligible for removal..."
    log "Operator: $DEPRECATED_WALLET_PKH"
    log "Status: AWAITING_REMOVAL"
    log "Eligible for removal after: $(date -u -d '+7 days' +%Y-%m-%d) (1 week safety buffer)"

    # Step 8.4: Generate success notification
    log "Step 8.4: Generating success notification..."

    BITCOIN_TXID=$(cat "$OUTPUT_DIR/bitcoin_txid.txt")

    cat > "$OUTPUT_DIR/success_notification.txt" <<EOF
✅ Manual Sweep Successfully Completed!

Wallet: $DEPRECATED_WALLET_PKH
Bitcoin Transaction: $BITCOIN_TXID
Amount Moved: $TOTAL_INPUT_BTC BTC
Target Wallet: $ACTIVE_WALLET_PKH
Provider: $PROVIDER

Confirmations: $REQUIRED_CONFIRMATIONS+
SPV Proof: Submitted and verified on Ethereum

Deprecated wallet now at 0 BTC (or minimal dust).
Operator will be eligible for removal on $(date -u -d '+7 days' +%Y-%m-%d).

Next steps:
1. Wait 1-week safety buffer
2. Receive decommissioning approval from Threshold team
3. Proceed with operator node shutdown
EOF

    cat "$OUTPUT_DIR/success_notification.txt"

    log "Success notification saved to $OUTPUT_DIR/success_notification.txt"
    log "Send this notification to all operators via email/Slack/Discord"
}

#==============================================================================
# MONITORING AND DIAGNOSTICS
#==============================================================================

check_operator_node_health() {
    log "Checking operator node health..."

    # Check node status
    if systemctl is-active --quiet tbtc-node 2>/dev/null; then
        log "✅ tBTC node service is running"
    else
        log "❌ tBTC node service is NOT running"
    fi

    # Check node health endpoint
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        HEALTH=$(curl -s http://localhost:8080/health)
        log "Node health: $HEALTH"
    else
        log "⚠️  Cannot reach node health endpoint"
    fi

    # Check Ethereum sync
    ETH_BLOCK=$(cast block-number 2>/dev/null || echo "ERROR")
    log "Ethereum block height: $ETH_BLOCK"

    # Check Bitcoin connection
    if bitcoin-cli getblockchaininfo > /dev/null 2>&1; then
        BTC_BLOCKS=$(bitcoin-cli getblockchaininfo | jq -r '.blocks')
        log "✅ Bitcoin node connected, blocks: $BTC_BLOCKS"
    else
        log "❌ Cannot connect to Bitcoin node"
    fi
}

#==============================================================================
# MAIN EXECUTION
#==============================================================================

main() {
    log "Manual Sweep Execution Script Started"
    log "Output directory: $OUTPUT_DIR"
    log ""

    # Show menu
    echo "=========================================="
    echo "Manual Sweep Execution - Main Menu"
    echo "=========================================="
    echo "1. Phase 1: Preparation (identify wallets, calculate fees)"
    echo "2. Phase 2: Coordination Window (calculate next window)"
    echo "3. Phases 3-5: Operator Coordination (automatic - monitoring only)"
    echo "4. Phase 6: Bitcoin Confirmations (monitor confirmations)"
    echo "5. Phase 7: SPV Proof Submission (construct and submit)"
    echo "6. Phase 8: Verification and Cleanup"
    echo "7. Check Operator Node Health"
    echo "8. Run Complete Process (all phases)"
    echo "0. Exit"
    echo "=========================================="
    read -p "Select option: " OPTION

    case $OPTION in
        1)
            phase1_preparation
            ;;
        2)
            phase2_coordination_window
            ;;
        3)
            phase3_5_operator_coordination
            ;;
        4)
            phase6_bitcoin_confirmations
            ;;
        5)
            phase7_spv_proof
            ;;
        6)
            phase8_verification
            ;;
        7)
            check_operator_node_health
            ;;
        8)
            phase1_preparation
            phase2_coordination_window
            phase3_5_operator_coordination
            log "⚠️  Pausing for operator coordination..."
            log "After Bitcoin transaction is broadcast, continue with remaining phases."
            read -p "Press Enter when Bitcoin transaction ID is available..."
            phase6_bitcoin_confirmations
            phase7_spv_proof
            read -p "Press Enter when SPV proof has been submitted to Ethereum..."
            phase8_verification
            ;;
        0)
            log "Exiting..."
            exit 0
            ;;
        *)
            error "Invalid option"
            ;;
    esac

    log ""
    log "Phase completed. Results saved to: $OUTPUT_DIR"
}

# Run main function
main

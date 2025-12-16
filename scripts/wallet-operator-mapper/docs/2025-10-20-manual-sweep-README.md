# Manual Sweep Execution Script

**Created**: 2025-10-20
**Source**: 2025-10-10-manual-sweep-technical-process.md
**Script**: 2025-10-20-manual-sweep-execution-script.sh

## Overview

This shell script template extracts and organizes all executable commands from the Manual Sweep Technical Specification into a runnable format. It guides operators and the Threshold team through the 8-phase manual sweep process.

## What This Script Does

### ✅ Automated Phases
- **Phase 1**: Query wallet balances, UTXOs, and mempool fees
- **Phase 2**: Calculate next coordination window
- **Phase 6**: Monitor Bitcoin confirmations
- **Phase 8**: Verify final state and generate notifications

### ⚠️ Manual Intervention Required
- **Phases 3-5**: Operator node coordination (automatic via node software)
- **Phase 7**: SPV proof construction (requires specialized tools)
- **Notifications**: Sending alerts to operators

## Prerequisites

### Required Commands
- `cast` (Foundry) - Ethereum contract interactions
- `bitcoin-cli` - Bitcoin node RPC
- `curl` - HTTP requests
- `jq` - JSON parsing
- `bc` - Arithmetic calculations

### Configuration Required

Before running, you MUST customize these variables in the script:

```bash
# Ethereum Contracts
ALLOWLIST_ADDRESS="0x..."
WALLET_REGISTRY_ADDRESS="0x..."
BRIDGE_ADDRESS="0x..."
VALIDATOR_ADDRESS="0x..."

# Wallets
DEPRECATED_WALLET_PKH="0x..."
DEPRECATED_WALLET_ADDRESS="bc1q..."
ACTIVE_WALLET_PKH="0x..."
ACTIVE_WALLET_ADDRESS="bc1q..."

# Provider
PROVIDER="BOAR"  # or "STAKED" or "P2P"
```

## Usage

### Interactive Menu Mode

```bash
./2025-10-20-manual-sweep-execution-script.sh
```

This presents a menu to run individual phases:

```
1. Phase 1: Preparation
2. Phase 2: Coordination Window
3. Phases 3-5: Operator Coordination (monitoring)
4. Phase 6: Bitcoin Confirmations
5. Phase 7: SPV Proof Submission
6. Phase 8: Verification and Cleanup
7. Check Operator Node Health
8. Run Complete Process
```

### Running Specific Phases

You can modify the script to run specific functions directly:

```bash
# Run only preparation
source ./2025-10-20-manual-sweep-execution-script.sh
phase1_preparation

# Check operator health
check_operator_node_health

# Monitor Bitcoin confirmations
phase6_bitcoin_confirmations
```

## Phase-by-Phase Guide

### Phase 1: Preparation

**Purpose**: Identify straggler wallets and construct proposal parameters

**Commands executed**:
```bash
# Query Bitcoin balance
bitcoin-cli getreceivedbyaddress <address> 0

# Query Ethereum wallet state
cast call <registry> "getWalletMainUtxo(bytes20)" <pkh>

# Get mempool fees
curl https://mempool.space/api/v1/fees/recommended

# List UTXOs
bitcoin-cli listunspent 0 9999999 '["<address>"]'
```

**Output**: `proposal_params.json` with all MovingFunds parameters

### Phase 2: Coordination Window

**Purpose**: Calculate when the next RFC-12 coordination window opens

**Commands executed**:
```bash
# Get current block
cast block-number

# Calculate next coordination block (every 900 blocks)
```

**Output**: Coordination block number and timing

### Phases 3-5: Operator Coordination

**Purpose**: Monitor automatic coordination process

**Note**: These phases are handled automatically by operator node software. This script provides monitoring guidance only.

**Expected flow**:
1. Leader is deterministically selected
2. Leader broadcasts CoordinationMessage
3. Followers validate and sign
4. Leader aggregates signatures
5. Bitcoin transaction is broadcast

**Manual action**: Monitor coordination channel for Bitcoin transaction ID

### Phase 6: Bitcoin Confirmations

**Purpose**: Wait for 6 Bitcoin confirmations and verify transfer

**Commands executed**:
```bash
# Monitor confirmations (loops)
bitcoin-cli gettransaction <txid>

# Verify UTXO arrived
bitcoin-cli gettxout <txid> <vout>
bitcoin-cli getreceivedbyaddress <active_wallet> 0
```

**Output**: Confirmation when 6+ blocks reached

### Phase 7: SPV Proof Submission

**Purpose**: Construct and submit SPV proof to Ethereum Bridge

**Commands executed**:
```bash
# Get raw transaction
bitcoin-cli getrawtransaction <txid>

# Get block data with merkle tree
bitcoin-cli getblock <blockhash> 2

# Get block header
bitcoin-cli getblockheader <blockhash> false
```

**Manual action required**:
- Build Merkle proof using specialized tools
- Submit to Bridge contract with `cast send`

**Note**: SPV proof construction requires tBTC-specific tooling not included in this script.

### Phase 8: Verification

**Purpose**: Verify final state and generate notifications

**Commands executed**:
```bash
# Check Ethereum state
cast call <registry> "getWalletMainUtxo(bytes20)" <deprecated_pkh>

# Check Bitcoin balance
bitcoin-cli getreceivedbyaddress <deprecated_address> 0
```

**Output**: Success notification text file

## Output Files

All intermediate results are saved to a timestamped directory:
```
/tmp/manual-sweep-YYYYMMDD-HHMMSS/
├── sweep.log                           # Complete execution log
├── wallet_utxo.txt                     # Ethereum wallet state
├── mempool_fees.json                   # Current Bitcoin fees
├── utxos.json                          # All wallet UTXOs
├── proposal_params.json                # MovingFunds proposal
├── coordination_block.txt              # Coordination window
├── bitcoin_txid.txt                    # Bitcoin transaction ID
├── block_hash.txt                      # Bitcoin block hash
├── raw_tx.hex                          # Raw transaction
├── block_data.json                     # Block with merkle tree
├── tx_index.txt                        # Transaction index
├── block_header.hex                    # Block header
├── deprecated_wallet_final_state.txt   # Final Ethereum state
└── success_notification.txt            # Operator notification
```

## Example Workflow

### Complete Manual Sweep Process

```bash
# 1. Configure script variables (edit file)
vim 2025-10-20-manual-sweep-execution-script.sh

# 2. Run preparation phase
./2025-10-20-manual-sweep-execution-script.sh
# Select option 1: Phase 1 Preparation

# 3. Send notifications to operators (manual)
# Email/Slack the proposal_params.json

# 4. Calculate coordination window
# Select option 2: Phase 2 Coordination Window

# 5. Wait for coordination window
# Monitor operator nodes during window

# 6. After Bitcoin TX is broadcast, monitor confirmations
# Select option 4: Phase 6 Bitcoin Confirmations
# Enter Bitcoin transaction ID when prompted

# 7. Construct and submit SPV proof (manual + script)
# Select option 5: Phase 7 SPV Proof Submission
# Follow instructions to complete SPV proof

# 8. Verify and notify
# Select option 6: Phase 8 Verification
# Send success_notification.txt to operators
```

## Safety Features

- **Set -e**: Exits on any command error
- **Set -u**: Exits on undefined variables
- **Logging**: All actions logged with timestamps
- **Output preservation**: All intermediate data saved
- **Prerequisite checks**: Validates required commands exist

## Limitations

### What This Script CANNOT Do

1. **Operator node coordination**: Phases 3-5 require running operator nodes with tBTC software
2. **SPV proof construction**: Requires specialized tBTC tools (not just Bitcoin RPC)
3. **Automatic notifications**: Sending emails/Slack messages to operators
4. **Signature aggregation**: This is done by operator node software
5. **Leader selection**: Deterministic but requires node coordination protocol

### What This Script CAN Do

1. ✅ Query all necessary data (wallets, UTXOs, fees)
2. ✅ Calculate proposal parameters
3. ✅ Monitor Bitcoin confirmations
4. ✅ Verify final state
5. ✅ Generate notification templates

## Security Considerations

### ⚠️ WARNING: Private Keys

The script requires `SUBMITTER_PRIVATE_KEY` for SPV proof submission.

**Best practices**:
- Use environment variable instead of hardcoding
- Use hardware wallet or secure key management
- Only grant this key permission to submit proofs (no other powers)

### Recommended Approach

```bash
# Don't hardcode in script
export SUBMITTER_PRIVATE_KEY="0x..."

# Reference in script
SUBMITTER_PRIVATE_KEY="${SUBMITTER_PRIVATE_KEY:-}"
```

## Troubleshooting

### Bitcoin CLI Connection Issues
```bash
# Check Bitcoin node is running
bitcoin-cli getblockchaininfo

# If failing, check bitcoin.conf settings
```

### Ethereum RPC Issues
```bash
# Test connection
cast block-number

# Check RPC URL in environment
echo $ETH_RPC_URL
```

### Missing Commands
```bash
# Install Foundry (for cast)
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Bitcoin Core required for bitcoin-cli
```

## Cost Estimation

When running Phase 1, the script calculates expected costs:

- **Bitcoin fee**: `TX_SIZE × FEE_RATE` satoshis
- **Ethereum gas**: Manual SPV proof submission (~400K gas)

Check current prices before execution:
```bash
# Bitcoin mempool
curl https://mempool.space/api/v1/fees/recommended

# Ethereum gas
cast gas-price
```

## Next Steps After Running

1. **Week 4 Assessment**: Use Phase 1 to identify which wallets need sweeps
2. **Schedule Coordination**: Based on Phase 2 calculation, schedule operator availability
3. **Execute Sweep**: Run through all phases with operator participation
4. **Safety Buffer**: Wait 1 week after wallet reaches 0 BTC
5. **Operator Removal**: Proceed with allowlist updates and node decommissioning

## Support

- Review full technical specification: `2025-10-10-manual-sweep-technical-process.md`
- Check operator node logs during coordination
- Contact Threshold engineering team via `#operator-consolidation`

## License

This script is part of the tBTC Beta Staker Consolidation project.

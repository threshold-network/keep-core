# Wallet-Operator Mapper

**Purpose**: Maps tBTC v2 wallets to their controlling operators for beta staker consolidation.

## Quick Start

```bash
# 1. Install dependencies
npm install

# 2. Configure RPC endpoint
cp .env.example .env
# Edit .env with your Alchemy/Infura archive node URL

# 3. Set up data files (see Data Files Setup below)
mkdir -p data
# Copy required data files to ./data/ directory

# 4. Run analysis
node analyze-per-operator.js
```

## Data Files Setup

The scripts require two data files from the Memory Bank project:

### Required Files

1. **tBTC Proof of Funds** (`tbtc-proof-of-funds.json`)
   - Contains wallet balances and metadata
   - Source: Memory Bank `/knowledge/20251006-tbtc-proof-of-funds.json`

2. **Threshold Stakers CSV** (`threshold_stakers_may_2025.csv`)
   - Contains operator list and stake information
   - Source: Memory Bank `/knowledge/threshold_stakers_may_2025.csv`

### Setup Options

**Option 1: Use default data directory** (recommended)
```bash
mkdir -p data
cp /path/to/memory-bank/knowledge/20251006-tbtc-proof-of-funds.json data/tbtc-proof-of-funds.json
cp /path/to/memory-bank/knowledge/threshold_stakers_may_2025.csv data/threshold_stakers_may_2025.csv
```

**Option 2: Use environment variables**
```bash
# Add to .env file:
PROOF_OF_FUNDS_PATH=/custom/path/to/tbtc-proof-of-funds.json
THRESHOLD_STAKERS_CSV_PATH=/custom/path/to/threshold_stakers_may_2025.csv
```

## What This Does

Analyzes tBTC wallets to identify which contain deprecated operators being removed during consolidation.

**Core Function**: Queries on-chain DKG (Distributed Key Generation) events to extract the 100 operators controlling each wallet, then classifies them as KEEP (active) or DISABLE (deprecated) based on `operators.json`.

## Main Scripts

### query-dkg-events.js
Queries on-chain DKG events to extract wallet operator membership.
- **Requirements**: Archive node RPC (Alchemy recommended)
- **Runtime**: ~20 seconds per wallet
- **Output**: `wallet-operator-mapping.json`
- **Usage**: Run when wallet data needs updating

### analyze-per-operator.js
Calculates BTC distribution by provider from mapping data.
- **Runtime**: <1 second
- **Output**: Console report with per-provider BTC analysis
- **Usage**: Run after query-dkg-events.js to analyze results

### validate-operator-list.js
Verifies operator list completeness against CSV data.
- **Purpose**: Data quality checks
- **Usage**: Optional validation step

## Utility Scripts

### query-proof-of-funds.js
Queries ALL tBTC wallets from Bridge contract and their Bitcoin balances.
- **Requirements**: Archive node RPC + internet (uses mempool.space API)
- **Runtime**: Several minutes (rate-limited Bitcoin API calls)
- **Output**: JSON with wallet PKHs, Bitcoin addresses, and balances
- **Usage**: Generate proof-of-funds reports for audits

### query-sortition-pool.js
Checks operators from `operators.json` against sortition pool status.
- **Output**: `sortition-pool-operators.json`
- **Usage**: Verify which operators are active in the pool

### query-all-pool-operators.js
Discovers ALL operators in sortition pool by iterating member IDs.
- **Output**: `pool-operators-complete.json`
- **Usage**: Find operators not in our known list (completeness check)

### query-all-stakers.js
Queries all stakers from TokenStaking contract events.
- **Runtime**: ~1 minute (queries historical events)
- **Usage**: Verify completeness of staker documentation

## Configuration Files

### operators.json ⭐ CRITICAL
Defines which operators to keep vs disable during consolidation.

**Structure**:
- `operators.keep[]`: 4 active operators (1 per provider: STAKED, P2P, BOAR, NUCO)
- `operators.disable[]`: 16 deprecated operators being removed

**Purpose**: Used by scripts to tag discovered operators as KEEP or DISABLE. Without this file, scripts cannot classify operators or calculate BTC in deprecated wallets.

**Source**: Memory Bank `/knowledge/8-final-operator-consolidation-list.md`

### .env
RPC endpoint configuration. Archive node required for historical DKG event queries.

## Output Data

**wallet-operator-mapping.json** contains:
- Wallet metadata (PKH, BTC balance, state)
- Complete operator membership (100 operators per wallet)
- Operator addresses matched to providers
- KEEP/DISABLE status per operator
- Summary statistics by provider

## Integration

**Part of**: Beta Staker Consolidation (Memory Bank: `/memory-bank/20250809-beta-staker-consolidation/`)

**Used for**:
- Monitoring dashboard data source (load mapping into Prometheus)
- Draining progress assessment (identify wallets requiring manual sweeps)
- Operator removal validation (verify wallets empty before removal)

## Important Notes

- **Equal-split calculation** is for analysis only—operators hold cryptographic key shares, not BTC shares
- All wallets require 51/100 threshold signatures for transactions
- Manual sweeps need coordination from all 4 providers simultaneously
- Deprecated operators cannot be removed until their wallets reach 0 BTC

## Documentation

- `docs/` - Manual sweep procedures and technical processes
- See Memory Bank for complete consolidation planning and correlation analysis

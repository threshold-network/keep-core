# Manual Sweep Process - Complete Technical Specification
## tBTC Beta Staker Consolidation (18 → 3 Operators)

**Document Date**: 2025-10-10
**Version**: 1.0
**Audience**: Technical operators, engineering team
**Purpose**: Detailed technical documentation of the manual BTC wallet sweep process

---

## Table of Contents

1. [Overview](#overview)
2. [When Manual Sweeps Are Triggered](#when-manual-sweeps-are-triggered)
3. [Prerequisites](#prerequisites)
4. [Actors and Roles](#actors-and-roles)
5. [Technical Architecture](#technical-architecture)
6. [Step-by-Step Process](#step-by-step-process)
7. [Data Structures and Parameters](#data-structures-and-parameters)
8. [Bitcoin Transaction Details](#bitcoin-transaction-details)
9. [Ethereum Smart Contract Interactions](#ethereum-smart-contract-interactions)
10. [RFC-12 Coordination Protocol](#rfc-12-coordination-protocol)
11. [Failure Scenarios and Recovery](#failure-scenarios-and-recovery)
12. [Cost Analysis](#cost-analysis)
13. [Timing and Scheduling](#timing-and-scheduling)
14. [Security Considerations](#security-considerations)
15. [Monitoring and Verification](#monitoring-and-verification)
16. [Operator Instructions](#operator-instructions)

---

## Overview

### What is a Manual Sweep?

A **manual sweep** (formally called **MovingFunds** in the tBTC protocol) is a coordinated operation that moves Bitcoin (BTC) from a deprecated wallet to an active wallet within the same provider organization.

**Key Characteristics**:
- Uses existing tBTC MovingFunds mechanism (no new code)
- Requires threshold signing coordination (51 of 100 operators)
- Leverages RFC-12 decentralized coordination
- Moves BTC on Bitcoin blockchain, then proves it to Ethereum via SPV
- Provider-specific: Each provider sweeps their own wallets

**Not a Manual Sweep**:
- ❌ User-initiated redemptions (those are automatic)
- ❌ Emergency wallet draining (different mechanism)
- ❌ Cross-provider BTC movement (doesn't happen)
- ❌ Forced migration to new wallet system (this is same system)

---

### Why Manual Sweeps May Be Required

**Primary Reason**: Natural draining (user redemptions) is insufficient to empty deprecated wallets within the October 2025 deadline.

**Trigger Conditions**:
1. **Straggler Wallets**: Wallet retains >20% of original balance after 4 weeks of natural draining
2. **Timeline Pressure**: October deadline is <3 weeks away and wallets not empty
3. **Low Redemption Volume**: Redemption rate drops significantly below historical average
4. **Edge Cases**: Specific wallets not being selected by redemption algorithm

**Probability**: 30-50% chance that some wallets will require manual sweeps (hybrid approach assumption).

---

### What Moves During a Manual Sweep?

**Assets Moving**:
- ✅ **Bitcoin (BTC)** - Physical BTC on Bitcoin blockchain
- ❌ **NOT T tokens** - No token staking involved
- ❌ **NOT tBTC** - tBTC remains with users, unaffected
- ❌ **NOT NFTs** - No wallet ownership tokens move

**Path of BTC**:
```
Deprecated Wallet (Bitcoin Address)
    ↓
Bitcoin Transaction (on-chain)
    ↓
Active Wallet (Bitcoin Address, same provider)
    ↓
SPV Proof to Ethereum (proves movement occurred)
    ↓
Ethereum Bridge Updates State (wallet balances)
```

---

## When Manual Sweeps Are Triggered

### Timeline Context

Manual sweeps occur during **Phase 3: Assessment & Potential Manual Sweeps** (Weeks 4-5).

**Week 4 Assessment Checkpoint** (~2025-12-01):
- Evaluate draining progress across all 15 deprecated wallets
- Calculate % BTC drained via natural redemptions
- Analyze redemption volume trends (stable, declining, increasing)
- Project completion timeline based on current velocity

**Decision Matrix**:

| Draining Progress | Redemption Volume | Action |
|-------------------|-------------------|--------|
| >50% drained | Stable | Continue natural draining |
| 30-50% drained | Stable | Monitor closely, prepare manual sweep |
| <30% drained | Stable | **Trigger manual sweep** for stragglers |
| Any progress | Declining >30% | **Trigger manual sweep immediately** |

---

### Specific Trigger Criteria

**Threshold Team will initiate manual sweeps if ANY of the following are true**:

1. **Individual Wallet Threshold**:
   - Wallet has >20% of original balance after 4 weeks
   - Example: Wallet started with 10 BTC, still has >2 BTC at Week 4

2. **Timeline Threshold**:
   - Current date is >2025-12-10 (less than 3 weeks to year-end target)
   - Any wallet still has >5% balance

3. **Redemption Volume Drop**:
   - Weekly redemption volume drops >30% below historical average
   - Projection shows completion >Week 9 (beyond October target)

4. **Wallet Stagnation**:
   - Wallet balance hasn't decreased in 2+ weeks
   - Indicates wallet not being selected by redemption algorithm

---

### Notification to Operators

When manual sweeps are triggered, operators will receive:

**48 Hours Before Coordination**:
1. **Email notification** with:
   - Which wallets require sweeps (wallet public key hashes)
   - Target coordination window (specific Ethereum block range)
   - Expected BTC amounts to be moved
   - Required operator participation (which providers involved)

2. **Slack/Discord message** in operator coordination channel:
   - Summary of assessment results
   - Decision to proceed with manual sweeps
   - Link to coordination details

3. **Calendar invitation**:
   - Coordination window time (in operator local timezones)
   - Duration: 100 blocks (~20 minutes)
   - Link to runbook and instructions

4. **Dashboard alert**:
   - Visual indicator on monitoring dashboard
   - Affected wallets highlighted
   - Coordination countdown timer

---

## Prerequisites

### Technical Prerequisites

Before manual sweeps can be executed, the following must be in place:

#### 1. Ethereum Mainnet Infrastructure

- **Allowlist Contract**: Deployed and configured
- **WalletRegistry Contract**: Optimized and operational
- **Bridge Contract**: Active and accepting SPV proofs
- **WalletProposalValidator Contract**: Deployed for validation

**Verification**:
```bash
# Check contracts on Ethereum mainnet
cast call $ALLOWLIST_ADDRESS "stakingProviders(address)" $OPERATOR_ADDRESS
cast call $WALLET_REGISTRY_ADDRESS "wallets(bytes20)" $WALLET_PKH
```

#### 2. Bitcoin Network Access

- **Electrum Servers**: Multiple servers configured for redundancy
- **Bitcoin Mempool Monitoring**: Real-time fee estimation via mempool.space
- **Bitcoin Block Explorer**: For transaction verification (e.g., blockchain.info)

**Verification**:
```bash
# Test Electrum server connectivity
electrum-client get_balance $BITCOIN_ADDRESS
```

#### 3. Operator Nodes

- **Minimum 10 of 18 operators online** (51% threshold)
- **Node health**: All participating operators' nodes operational
- **Coordination sync**: Operators synchronized to same Ethereum block height

**Verification**:
```bash
# Check operator node status
curl http://operator-node:8080/health
# Should return: {"status": "healthy", "blockHeight": 12345678}
```

#### 4. Wallet State

- **Wallet Active**: Deprecated wallet is still in LIVE state (not closed/terminated)
- **Wallet Has BTC**: Confirmed UTXOs exist in deprecated wallet
- **No Pending Actions**: No other wallet actions pending (redemptions, heartbeats)

**Verification**:
```bash
# Query wallet state on-chain
cast call $WALLET_REGISTRY_ADDRESS "getWalletState(bytes20)" $WALLET_PKH
# Should return: 0 (LIVE)
```

---

### Organizational Prerequisites

#### 1. Provider Coordination

**Provider Communication**:
- Provider management notified of upcoming sweep
- Operators instructed to be available during coordination window
- Emergency contact information confirmed

**Provider Acknowledgment Required**:
- Confirmation of operator availability (via email/Slack)
- Agreement on coordination timing
- Backup operators identified (if primary unavailable)

#### 2. Cost Approval

**DAO Treasury**:
- Governance approval for manual sweep costs
- Budget allocated for Bitcoin miner fees ($50-200 per wallet)
- Budget allocated for Ethereum gas fees ($100-300 per SPV proof)

**Total Estimated Cost** (if all 15 wallets require sweeps):
- Minimum: 15 × $150 = $2,250
- Maximum: 15 × $500 = $7,500

#### 3. Documentation and Runbook

**Required Documentation**:
- ✅ This manual sweep technical specification
- ✅ Operator runbook (step-by-step instructions)
- ✅ Emergency rollback procedures
- ✅ Coordination window calendar

---

## Actors and Roles

### 1. Threshold Team (Coordination Initiator)

**Responsibilities**:
- Monitor draining progress via dashboard
- Execute Week 4 assessment and make go/no-go decision
- Identify straggler wallets requiring sweeps
- Notify operators 48 hours in advance
- Calculate Bitcoin transaction fees (mempool analysis)
- Construct MovingFunds proposal parameters
- Submit SPV proofs to Ethereum after Bitcoin confirmations

**Tools Used**:
- Grafana monitoring dashboard
- Bitcoin mempool monitoring (mempool.space)
- Ethereum contract interaction tools (cast, hardhat)

**Key Personnel**:
- Engineering lead
- DevOps coordinator
- Provider liaisons (3 people, one per provider)

---

### 2. Leader Operator (Proposes MovingFunds)

**Selection**: Automatically determined by RFC-12 coordination algorithm at start of coordination window.

**Calculation**:
```python
coordination_seed = sha256(wallet_PKH + safe_block_hash)
rng = RNG(coordination_seed)
shuffled_operators = rng.shuffle(all_active_operators)
leader = shuffled_operators[0]
```

Where:
- `safe_block_hash` = block hash at `coordination_block - 32`
- `all_active_operators` = list of 18 operators (before consolidation)

**Responsibilities**:
1. Receive MovingFunds proposal parameters from Threshold team
2. Validate proposal locally (off-chain checks)
3. Construct CoordinationMessage with MovingFunds proposal
4. Broadcast CoordinationMessage to all follower operators
5. Initiate threshold signing ceremony
6. Broadcast completed Bitcoin transaction to Electrum servers

**Leader Probability**:
- Before consolidation: 1/18 = 5.6% per coordination window
- After consolidation: 1/3 = 33.3% per coordination window

---

### 3. Follower Operators (Validate and Sign)

**Who**: All operators NOT selected as leader (17 operators before consolidation).

**Responsibilities**:
1. Receive CoordinationMessage from leader
2. Validate leader identity (check signature matches expected leader)
3. Validate timing (message received within 80% of coordination window)
4. Validate MovingFunds proposal:
   - **On-chain validation**: Call `WalletProposalValidator.validateMovingFundsProposal()`
   - **Off-chain validation**: Check Bitcoin transaction structure, UTXOs exist, fee reasonable
5. If valid, participate in threshold signing ceremony
6. Sign partial signature share
7. Submit signature share to leader for aggregation

**Minimum Required**: 51 out of 100 operators must sign (with 18 total operators, need ~10).

---

### 4. Active Operator (Receives BTC)

**Who**: The single operator from the provider who will remain active after consolidation.

**For This Consolidation**:
- **BOAR**: `0xffb804c2de78576ad011f68a7df63d739b8c8155`
- **STAKED**: `0xf401aae8c639eb1638fd99b90eae8a4c54f9894d`
- **P2P**: `0xb074a3b960f29a1448a2dd4de95210ca492c18d4`

**Responsibilities**:
- Maintain wallet that will receive swept BTC
- Monitor BTC arrival after transaction confirms
- Verify balance increase matches expected amount
- Report any discrepancies to Threshold team

**Note**: Active operator's node ALSO participates in threshold signing (if selected as leader or follower).

---

### 5. Deprecated Operator (Wallet Being Drained)

**Who**: Operator whose wallet is being swept (one of the 15 deprecated operators).

**Responsibilities**:
- Keep node online and operational during coordination window
- Participate in threshold signing if node is still active
- Monitor wallet balance decrease after transaction
- Confirm wallet reached 0 BTC (or minimal dust)
- Await approval for node decommissioning

**After Sweep**:
- Wallet marked as empty
- Operator flagged for removal from allowlist
- Wait 1 week at 0 BTC (safety buffer)
- Receive decommissioning approval from Threshold team

---

## Technical Architecture

### System Components

```
┌───────────────────────────────────────────────────────────────┐
│                    ETHEREUM MAINNET                           │
│                                                               │
│  ┌──────────────┐  ┌───────────────┐  ┌───────────────┐       │
│  │  Allowlist   │  │WalletRegistry │  │    Bridge     │       │
│  │  Contract    │  │   Contract    │  │   Contract    │       │
│  └──────┬───────┘  └───────┬───────┘  └───────┬───────┘       │
│         │                  │                  │               │
│         │ Weight=0         │ MovingFunds      │ SPV Proof     │
│         │                  │ State Update     │ Verification  │
│         └──────────────────┴──────────────────┘               │
│                             ▲                                 │
│                             │                                 │
└─────────────────────────────┼─────────────────────────────────┘
                              │
                    ┌─────────┴──────────┐
                    │  OPERATOR NODES    │
                    │  (18 total, need   │
                    │   ~10 for signing) │
                    │                    │
                    │  RFC-12 Protocol:  │
                    │  - Leader Election │
                    │  - Coordination    │
                    │  - Threshold ECDSA │
                    └─────────┬──────────┘
                              │
                              │ Bitcoin Tx
                              │ Broadcast
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    BITCOIN NETWORK                          │
│                                                             │
│  ┌──────────────┐          ┌──────────────┐                 │
│  │ Deprecated   │  ──────> │Active Wallet │                 │
│  │   Wallet     │   BTC    │ (Same Prov.) │                 │
│  └──────────────┘  Move    └──────────────┘                 │
│                                                             │
│  Via Electrum Servers → Bitcoin Miners → 6 Confirmations    │
└─────────────────────────────────────────────────────────────┘
```

---

### Data Flow Overview

```
[Threshold Team]
    │
    │ 1. Calculate MovingFunds Parameters
    │    (Target wallet, UTXOs, fee rate)
    │
    ▼
[Coordination Window Opens]
    │
    │ 2. Leader Selected (RFC-12 algorithm)
    │
    ▼
[Leader Operator]
    │
    │ 3. Receive parameters, construct proposal
    │
    │ 4. Broadcast CoordinationMessage
    │
    ▼
[Follower Operators]
    │
    │ 5. Validate proposal (on-chain + off-chain)
    │
    │ 6. Participate in threshold signing
    │    (Each produces signature share)
    │
    ▼
[Leader Operator]
    │
    │ 7. Aggregate signatures (51+ shares)
    │
    │ 8. Construct complete Bitcoin transaction
    │
    │ 9. Broadcast to Bitcoin network
    │
    ▼
[Bitcoin Network]
    │
    │ 10. Transaction propagates, miners include
    │
    │ 11. Wait 6 confirmations (~1 hour)
    │
    ▼
[Threshold Team or Any Operator]
    │
    │ 12. Construct SPV proof (tx + merkle proof)
    │
    │ 13. Submit SPV proof to Ethereum Bridge
    │
    ▼
[Ethereum Bridge Contract]
    │
    │ 14. Verify SPV proof
    │
    │ 15. Update wallet balances on-chain
    │
    │ 16. Mark deprecated wallet as empty
    │
    ▼
[Verification Complete]
```

---

## Step-by-Step Process

### Phase 1: Preparation (Threshold Team)

**Duration**: 1-2 hours before coordination window

#### Step 1.1: Identify Straggler Wallets

**Action**: Query all 15 deprecated wallets and identify which require manual sweeps.

**Technical Details**:
```bash
# Query wallet balance on Bitcoin blockchain
bitcoin-cli getreceivedbyaddress $BITCOIN_ADDRESS 0

# Query wallet state on Ethereum
cast call $WALLET_REGISTRY_ADDRESS \
  "getWalletMainUtxo(bytes20)" \
  $WALLET_PKH
```

**Criteria**: Wallet has >20% of original balance after 4 weeks.

**Output**: List of wallet public key hashes (PKH) requiring sweeps.

Example:
```
Straggler wallets identified:
- 0x1234...abcd (BOAR, 2.5 BTC remaining)
- 0x5678...efgh (STAKED, 3.1 BTC remaining)
- 0x9abc...ijkl (P2P, 1.8 BTC remaining)
```

---

#### Step 1.2: Calculate Bitcoin Transaction Fees

**Action**: Check current Bitcoin mempool congestion and estimate appropriate fee rate.

**Technical Details**:
```bash
# Check current mempool fee recommendations
curl https://mempool.space/api/v1/fees/recommended
```

**Output**:
```json
{
  "fastestFee": 15,    // sat/vB for next block
  "halfHourFee": 12,   // sat/vB for ~30 min
  "hourFee": 10,       // sat/vB for ~1 hour
  "economyFee": 8      // sat/vB for >1 hour
}
```

**Decision**: Choose fee rate based on urgency.
- **Urgent** (deadline <1 week): Use `fastestFee` (higher cost)
- **Normal** (deadline 1-3 weeks): Use `halfHourFee` (balanced)
- **Not urgent** (deadline >3 weeks): Use `hourFee` (economical)

**Fee Calculation**:
```
Transaction Size Estimate:
- Input: 148 bytes per UTXO (P2PKH) or 68 bytes (P2WPKH)
- Output: 34 bytes per output
- Overhead: 10 bytes

Typical MovingFunds transaction:
- 2 inputs × 148 bytes = 296 bytes
- 1 output × 34 bytes = 34 bytes
- Overhead = 10 bytes
- Total = 340 bytes

Fee = 340 bytes × 12 sat/vB = 4,080 sats ≈ 0.0000408 BTC

At $50,000/BTC: 0.0000408 × $50,000 = $2.04
```

**Note**: Actual fees vary based on:
- Number of UTXOs in deprecated wallet (more UTXOs = larger transaction)
- Wallet address type (P2PKH vs P2WPKH)
- Mempool congestion at time of broadcast

---

#### Step 1.3: Query Wallet UTXOs

**Action**: Identify all unspent transaction outputs (UTXOs) in deprecated wallet.

**Technical Details**:
```bash
# Using Electrum client
electrum-client listunspent $BITCOIN_ADDRESS

# Or via Bitcoin Core RPC
bitcoin-cli listunspent 0 9999999 '["$BITCOIN_ADDRESS"]'
```

**Output**:
```json
[
  {
    "txid": "abc123...",
    "vout": 0,
    "address": "bc1q...",
    "scriptPubKey": "0014...",
    "amount": 1.5,
    "confirmations": 100,
    "spendable": true
  },
  {
    "txid": "def456...",
    "vout": 1,
    "address": "bc1q...",
    "scriptPubKey": "0014...",
    "amount": 1.0,
    "confirmations": 50,
    "spendable": true
  }
]
```

**UTXO Selection Strategy**:
- Include all UTXOs (minimize remaining dust)
- Prioritize larger UTXOs first (if transaction size matters)
- Ensure UTXOs have sufficient confirmations (>6 confirmations)

---

#### Step 1.4: Construct MovingFunds Proposal Parameters

**Action**: Package all information needed for MovingFunds proposal.

**Data Structure**:
```typescript
interface MovingFundsParameters {
  // Wallet being drained
  sourceWalletPKH: bytes20;           // 20-byte wallet public key hash

  // Target wallet (same provider)
  targetWalletPKH: bytes20;           // 20-byte active wallet PKH

  // Bitcoin transaction inputs
  utxos: Array<{
    txHash: bytes32;                  // Previous transaction hash
    outputIndex: uint32;              // Output index in that transaction
    value: uint64;                    // Value in satoshis
  }>;

  // Bitcoin transaction output
  outputValue: uint64;                // Output amount (total - fee)

  // Fee
  feeRate: uint32;                    // Fee rate in sat/vB
  totalFee: uint64;                   // Total fee in satoshis

  // Metadata
  coordinationBlock: uint256;         // Target Ethereum coordination block
  provider: string;                   // "BOAR" | "STAKED" | "P2P"
}
```

**Example**:
```json
{
  "sourceWalletPKH": "0x1234567890abcdef1234567890abcdef12345678",
  "targetWalletPKH": "0xffb804c2de78576ad011f68a7df63d739b8c8155",
  "utxos": [
    {
      "txHash": "0xabc123...",
      "outputIndex": 0,
      "value": 150000000
    },
    {
      "txHash": "0xdef456...",
      "outputIndex": 1,
      "value": 100000000
    }
  ],
  "outputValue": 249995920,
  "feeRate": 12,
  "totalFee": 4080,
  "coordinationBlock": 12345600,
  "provider": "BOAR"
}
```

---

#### Step 1.5: Notify Operators

**Action**: Send 48-hour advance notice to all operators.

**Communication Channels**:

1. **Email** (to all 18 operators):
   ```
   Subject: Manual Sweep Required - Coordination Window 2025-11-29 14:00 UTC

   Body:
   Dear Operators,

   Week 4 assessment shows the following wallets require manual sweeps:
   - Wallet 0x1234...5678 (BOAR): 2.5 BTC
   - Wallet 0x5678...abcd (STAKED): 3.1 BTC
   - Wallet 0x9abc...efgh (P2P): 1.8 BTC

   Coordination Window:
   - Ethereum Block: 12345600 (~2025-11-29 14:00 UTC)
   - Duration: 100 blocks (~20 minutes)
   - Required: 10 of 18 operators online

   Please confirm availability by replying to this email.

   Runbook: [link to detailed instructions]
   Dashboard: [link to monitoring dashboard]

   Thank you,
   Threshold Team
   ```

2. **Slack/Discord** (in #operator-consolidation channel):
   ```
   🚨 Manual Sweep Coordination Required

   📅 Date: 2025-11-29 14:00 UTC
   ⛓️ Block: 12345600
   ⏱️ Duration: ~20 minutes

   Wallets to sweep:
   • BOAR: 2.5 BTC
   • STAKED: 3.1 BTC
   • P2P: 1.8 BTC

   Please ensure your node is online and responsive.
   Reply with ✅ to confirm availability.
   ```

3. **Calendar Invitation** (sent to all operators):
   - Event: "Manual Sweep Coordination Window"
   - Time: 2025-11-29 14:00 UTC (converted to operator timezones)
   - Duration: 30 minutes (buffer beyond 20 min coordination window)
   - Description: Link to runbook, dashboard, coordination details

---

### Phase 2: Coordination Window Opens

**Duration**: Every 900 blocks (~3 hours), window lasts 100 blocks (~20 minutes)

#### Step 2.1: Determine Coordination Block

**Action**: Calculate which Ethereum block opens the coordination window.

**Technical Details**:
```python
coordination_frequency = 900  # blocks
current_block = eth_get_block_number()

# Find next coordination block
if current_block % coordination_frequency == 0:
    coordination_block = current_block
else:
    coordination_block = current_block + (coordination_frequency - (current_block % coordination_frequency))
```

**Example**:
```
Current block: 12345678
12345678 % 900 = 78
Next coordination block: 12345678 + (900 - 78) = 12346500
```

**Coordination Window**:
- **Opens**: Block 12346500
- **Closes**: Block 12346600 (100 blocks later)
- **Duration**: 100 blocks × 12 seconds = 1200 seconds = 20 minutes

---

#### Step 2.2: Select Leader Operator

**Action**: Deterministically select leader using RFC-12 algorithm.

**Technical Details**:
```python
def select_leader(wallet_pkh: bytes, coordination_block: int, operators: List[Address]) -> Address:
    # Get safe block hash (32 blocks before coordination block)
    safe_block = coordination_block - 32
    safe_block_hash = eth_get_block_hash(safe_block)

    # Generate coordination seed
    coordination_seed = sha256(wallet_pkh + safe_block_hash)

    # Initialize RNG with seed
    rng = RNG(coordination_seed)

    # Shuffle operators and select first
    shuffled_operators = rng.shuffle(operators)
    leader = shuffled_operators[0]

    return leader
```

**Example**:
```
Inputs:
- wallet_pkh: 0x1234567890abcdef1234567890abcdef12345678
- coordination_block: 12346500
- safe_block: 12346468
- safe_block_hash: 0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890
- operators: [0xOperator1, 0xOperator2, ..., 0xOperator18]

Calculation:
- coordination_seed = sha256(0x1234...5678 + 0xabcd...7890)
                    = 0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba

- rng = RNG(0x9876...dcba)
- shuffled = rng.shuffle([0xOp1, 0xOp2, ..., 0xOp18])
           = [0xOp7, 0xOp3, 0xOp12, ...]  // deterministic shuffle

Leader: 0xOp7 (first in shuffled list)
```

**Important**: All operators independently calculate this and arrive at the same leader. No centralized leader selection.

---

### Phase 3: Leader Proposes MovingFunds

**Duration**: First few minutes of coordination window

#### Step 3.1: Leader Receives Proposal Parameters

**Action**: Threshold team provides MovingFunds parameters to identified leader (via email or direct message).

**Alternative**: Parameters could be posted publicly (encrypted or plaintext) for leader to retrieve.

**Data Received**: Full `MovingFundsParameters` structure from Step 1.4.

---

#### Step 3.2: Leader Validates Proposal Locally

**Action**: Before broadcasting, leader performs local validation to ensure proposal is sound.

**On-Chain Validation**:
```bash
# Call WalletProposalValidator contract
cast call $VALIDATOR_ADDRESS \
  "validateMovingFundsProposal(bytes20,bytes)" \
  $WALLET_PKH \
  $PROPOSAL_BYTES
```

**Expected Output**: `true` (proposal valid)

**Off-Chain Validation**:
1. **Wallet State**: Confirm wallet is LIVE
2. **UTXOs Exist**: Verify UTXOs are unspent on Bitcoin blockchain
3. **Fee Reasonable**: Check fee rate is within acceptable range (not too high/low)
4. **Target Valid**: Confirm target wallet belongs to same provider
5. **Amount Correct**: Verify output amount = (sum of UTXOs) - fee

**Example Checks**:
```python
def validate_proposal_offchain(params: MovingFundsParameters) -> bool:
    # 1. Wallet is LIVE
    wallet_state = wallet_registry.getWalletState(params.sourceWalletPKH)
    if wallet_state != WalletState.LIVE:
        return False

    # 2. UTXOs exist and unspent
    for utxo in params.utxos:
        if not bitcoin_is_unspent(utxo.txHash, utxo.outputIndex):
            return False

    # 3. Fee reasonable (between 5 and 100 sat/vB)
    if params.feeRate < 5 or params.feeRate > 100:
        return False

    # 4. Target wallet is same provider
    if not same_provider(params.sourceWalletPKH, params.targetWalletPKH):
        return False

    # 5. Amount calculation correct
    total_input = sum(utxo.value for utxo in params.utxos)
    if params.outputValue != total_input - params.totalFee:
        return False

    return True
```

---

#### Step 3.3: Leader Constructs CoordinationMessage

**Action**: Package proposal into RFC-12 CoordinationMessage structure.

**Data Structure**:
```typescript
interface CoordinationMessage {
  memberId: number;                   // First signer controlled by leader
  coordinationBlock: uint256;         // Coordination block number
  walletPublicKeyHash: bytes20;       // Wallet being coordinated (source)
  proposal: bytes;                    // Serialized MovingFunds proposal
}
```

**Proposal Serialization**:
```
Proposal Bytes (MovingFunds):
- Target wallet PKH (20 bytes)
- Number of UTXOs (1 byte)
- For each UTXO:
  - Transaction hash (32 bytes)
  - Output index (4 bytes)
  - Value (8 bytes)
- Output value (8 bytes)
- Fee rate (4 bytes)
```

**Example**:
```json
{
  "memberId": 42,
  "coordinationBlock": 12346500,
  "walletPublicKeyHash": "0x1234567890abcdef1234567890abcdef12345678",
  "proposal": "0xffb804c2de78576ad011f68a7df63d739b8c8155020000000000......"
}
```

---

#### Step 3.4: Leader Signs and Broadcasts CoordinationMessage

**Action**: Sign message with leader's private key and broadcast to all operators.

**Signature**:
```python
message_hash = keccak256(encode(coordination_message))
signature = ecdsa_sign(leader_private_key, message_hash)
```

**Broadcast Mechanism**:
- **Peer-to-peer network**: Direct connections between operator nodes
- **Gossip protocol**: Message propagates to all 18 operators within seconds
- **Backup channels**: Discord/Slack notification (optional)

**Message Format**:
```
CoordinationMessage + Signature:
{
  "message": { ... },
  "signature": {
    "r": "0x1234...",
    "s": "0x5678...",
    "v": 27
  },
  "timestamp": 1701193200,
  "leader": "0xOperator7"
}
```

---

### Phase 4: Followers Validate and Sign

**Duration**: Remaining coordination window (~15 minutes after leader broadcasts)

#### Step 4.1: Followers Receive CoordinationMessage

**Action**: All 17 follower operators receive message via peer-to-peer network.

**Reception Check**:
```python
def on_coordination_message_received(msg: CoordinationMessage, sig: Signature):
    logger.info(f"Received coordination message from {msg.sender}")
    logger.info(f"Block: {msg.coordinationBlock}, Wallet: {msg.walletPublicKeyHash}")

    # Proceed to validation
    validate_and_sign(msg, sig)
```

---

#### Step 4.2: Followers Validate Leader Identity

**Action**: Verify message was signed by expected leader.

**Technical Details**:
```python
def validate_leader_identity(msg: CoordinationMessage, sig: Signature) -> bool:
    # 1. Recover signer from signature
    message_hash = keccak256(encode(msg))
    recovered_address = ecrecover(message_hash, sig)

    # 2. Calculate expected leader
    expected_leader = select_leader(msg.walletPublicKeyHash, msg.coordinationBlock, all_operators)

    # 3. Verify match
    if recovered_address != expected_leader:
        logger.error(f"Invalid leader: got {recovered_address}, expected {expected_leader}")
        return False

    return True
```

**If Validation Fails**: Follower ignores message (does not participate in signing).

---

#### Step 4.3: Followers Validate Timing

**Action**: Confirm message received within acceptable window.

**Technical Details**:
```python
def validate_timing(msg: CoordinationMessage) -> bool:
    current_block = eth_get_block_number()

    # Coordination window: [coordination_block, coordination_block + 100]
    window_start = msg.coordinationBlock
    window_end = msg.coordinationBlock + 100

    # Message must arrive within 80% of window
    window_80_percent = msg.coordinationBlock + 80

    # Check 1: Current block is within window
    if current_block < window_start or current_block > window_end:
        logger.error(f"Outside coordination window: current={current_block}, window=[{window_start}, {window_end}]")
        return False

    # Check 2: Not too late (within 80% of window)
    if current_block > window_80_percent:
        logger.warning(f"Message arrived late: current={current_block}, deadline={window_80_percent}")
        return False

    return True
```

**If Validation Fails**: Follower ignores message (too late or outside window).

---

#### Step 4.4: Followers Validate MovingFunds Proposal (On-Chain)

**Action**: Call Ethereum smart contract to validate proposal parameters.

**Technical Details**:
```solidity
// WalletProposalValidator.sol
function validateMovingFundsProposal(
    bytes20 walletPubKeyHash,
    bytes calldata movingFundsProposal
) external view returns (bool);
```

**Validation Logic** (inside contract):
1. Wallet is LIVE (not closed/terminated/locked)
2. Wallet has no pending MovingFunds action
3. Target wallet exists and is LIVE
4. Target wallet is controlled by same operators (or authorized subset)
5. UTXOs referenced in proposal match wallet's main UTXO
6. Fee calculation is correct

**Operator Call**:
```bash
cast call $VALIDATOR_ADDRESS \
  "validateMovingFundsProposal(bytes20,bytes)" \
  $WALLET_PKH \
  $PROPOSAL_BYTES
```

**Expected Output**: `0x0000...0001` (true)

**If Validation Fails**: Follower rejects proposal and does NOT sign.

---

#### Step 4.5: Followers Validate MovingFunds Proposal (Off-Chain)

**Action**: Perform Bitcoin-side validations that can't be done on Ethereum.

**Checks**:
1. **UTXOs Exist and Unspent**:
   ```bash
   bitcoin-cli gettxout $TXID $VOUT
   # Should return UTXO data (not null = unspent)
   ```

2. **UTXO Values Match**:
   - Query Bitcoin blockchain for each UTXO value
   - Verify values match proposal parameters

3. **Target Wallet is Bitcoin-Valid**:
   - Derive Bitcoin address from target wallet PKH
   - Verify it's a valid Bitcoin address format

4. **Fee is Reasonable**:
   - Check fee rate is within 5-100 sat/vB range
   - Compare to current mempool recommendations
   - Verify fee isn't excessive (no value extraction)

5. **No Conflicting Transactions**:
   - Check Bitcoin mempool for any pending transactions spending same UTXOs
   - If found, reject proposal (double-spend attempt)

**Example**:
```python
def validate_proposal_offchain_follower(proposal: MovingFundsProposal) -> bool:
    for utxo in proposal.utxos:
        # Check UTXO exists and unspent
        utxo_data = bitcoin_get_txout(utxo.txHash, utxo.outputIndex)
        if utxo_data is None:
            logger.error(f"UTXO {utxo.txHash}:{utxo.outputIndex} is spent or doesn't exist")
            return False

        # Verify value matches
        if utxo_data.value != utxo.value:
            logger.error(f"UTXO value mismatch: expected {utxo.value}, got {utxo_data.value}")
            return False

    # Check fee reasonableness
    if proposal.feeRate < 5 or proposal.feeRate > 100:
        logger.error(f"Fee rate unreasonable: {proposal.feeRate} sat/vB")
        return False

    return True
```

**If Validation Fails**: Follower rejects proposal and does NOT sign.

---

#### Step 4.6: Followers Participate in Threshold Signing

**Action**: If all validations pass, follower generates a partial signature share.

**Threshold ECDSA Protocol**:

The tBTC protocol uses **GG20** threshold ECDSA scheme (or similar):

1. **Key Share**: Each operator holds a share of the wallet's private key (generated during DKG)
2. **Signing Ceremony**: Operators collaborate to produce a valid ECDSA signature WITHOUT reconstructing the full private key
3. **Threshold**: Need 51 out of 100 shares to produce valid signature

**Signing Process** (simplified):

```python
def participate_in_threshold_signing(proposal: MovingFundsProposal, wallet_pkh: bytes20):
    # 1. Construct Bitcoin transaction to be signed
    bitcoin_tx = construct_moving_funds_transaction(proposal)

    # 2. Generate signature hash (what we're signing)
    sighash = bitcoin_tx.signature_hash(input_index=0, sighash_type=SIGHASH_ALL)

    # 3. Generate partial signature using key share
    my_key_share = get_wallet_key_share(wallet_pkh)
    partial_signature = gg20_sign_share(my_key_share, sighash)

    # 4. Broadcast partial signature to leader
    send_to_leader(partial_signature)
```

**Partial Signature Structure**:
```typescript
interface PartialSignature {
  operatorId: number;           // Which operator produced this share
  signatureShare: bytes;        // The actual signature share
  commitment: bytes;            // Cryptographic commitment for verification
}
```

**Transmission**: Follower sends partial signature directly to leader (or broadcasts to all operators).

---

### Phase 5: Leader Aggregates Signatures

**Duration**: 2-5 minutes after receiving 51+ signature shares

#### Step 5.1: Leader Collects Partial Signatures

**Action**: Wait until at least 51 signature shares received.

**Technical Details**:
```python
def collect_signatures(timeout_blocks: int = 80) -> List[PartialSignature]:
    signatures = []
    start_block = eth_get_block_number()

    while len(signatures) < 51:
        # Check timeout
        current_block = eth_get_block_number()
        if current_block - start_block > timeout_blocks:
            logger.error(f"Timeout: only received {len(signatures)} signatures")
            return None

        # Receive signature shares from followers
        new_sig = receive_signature_share()
        if new_sig:
            # Verify share is valid (cryptographic check)
            if verify_signature_share(new_sig):
                signatures.append(new_sig)
                logger.info(f"Received signature {len(signatures)}/51")

    return signatures
```

**Minimum Required**: 51 shares (threshold)
**Typical Expected**: 14-16 shares (with 18 operators, ~80-90% participation)

**If Insufficient Signatures**: Coordination fails, retry in next coordination window (3 hours later).

---

#### Step 5.2: Leader Aggregates into Complete Signature

**Action**: Combine 51+ signature shares into single valid ECDSA signature.

**Technical Details**:
```python
def aggregate_signatures(shares: List[PartialSignature]) -> ECDSASignature:
    # GG20 signature aggregation
    # This is cryptographic magic - multiple shares combine into one signature
    aggregated_signature = gg20_aggregate(shares)

    # Verify aggregated signature is valid
    wallet_public_key = get_wallet_public_key(wallet_pkh)
    if not ecdsa_verify(wallet_public_key, sighash, aggregated_signature):
        raise Exception("Aggregated signature invalid!")

    return aggregated_signature
```

**Output**: A single, standard ECDSA signature `(r, s)` that can be used in Bitcoin transaction.

**Verification**: Leader verifies signature is valid before proceeding (safety check).

---

#### Step 5.3: Leader Constructs Complete Bitcoin Transaction

**Action**: Build final Bitcoin transaction with aggregated signature.

**Bitcoin Transaction Structure**:
```
Transaction {
  version: 2

  inputs: [
    {
      previous_output: {
        txid: "abc123...",
        vout: 0
      },
      script_sig: <signature> <public_key>,  // The aggregated signature goes here
      sequence: 0xFFFFFFFF
    },
    {
      previous_output: {
        txid: "def456...",
        vout: 1
      },
      script_sig: <signature> <public_key>,
      sequence: 0xFFFFFFFF
    }
  ]

  outputs: [
    {
      value: 249995920,  // 2.4999592 BTC (after fee deduction)
      script_pubkey: OP_DUP OP_HASH160 <active_wallet_address_hash> OP_EQUALVERIFY OP_CHECKSIG
    }
  ]

  locktime: 0
}
```

**Script Signature**:
```
<signature> = <r> <s> <sighash_type>
<public_key> = <wallet_public_key> (derived from wallet PKH)
```

**Serialization**:
```python
def construct_bitcoin_transaction(proposal: MovingFundsProposal, signature: ECDSASignature) -> bytes:
    tx = BitcoinTransaction()
    tx.version = 2

    # Add inputs
    for utxo in proposal.utxos:
        input = TxInput(
            previous_output=OutPoint(utxo.txHash, utxo.outputIndex),
            script_sig=build_script_sig(signature, wallet_public_key),
            sequence=0xFFFFFFFF
        )
        tx.inputs.append(input)

    # Add output
    output = TxOutput(
        value=proposal.outputValue,
        script_pubkey=build_p2pkh_script(proposal.targetWalletPKH)
    )
    tx.outputs.append(output)

    tx.locktime = 0

    return tx.serialize()
```

---

#### Step 5.4: Leader Broadcasts Transaction to Bitcoin Network

**Action**: Send signed transaction to Bitcoin network via Electrum servers.

**Technical Details**:
```bash
# Broadcast via Electrum
electrum-client broadcast $SIGNED_TX_HEX

# Or via Bitcoin Core
bitcoin-cli sendrawtransaction $SIGNED_TX_HEX
```

**Response**:
```json
{
  "result": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
  "error": null
}
```

Where `result` is the **transaction ID (txid)** on Bitcoin blockchain.

**Propagation**: Transaction propagates to Bitcoin nodes, enters mempool, waits for miners.

**Leader Actions**:
1. Broadcast to multiple Electrum servers (redundancy)
2. Verify transaction appears in mempool: `bitcoin-cli getmempoolentry $TXID`
3. Announce txid to all operators (via coordination channel)
4. Monitor transaction for confirmations

---

### Phase 6: Bitcoin Confirmations

**Duration**: ~1 hour (6 confirmations × 10 minutes average)

#### Step 6.1: Wait for Transaction to be Mined

**Action**: Monitor Bitcoin blockchain for transaction inclusion.

**Technical Details**:
```bash
# Check if transaction is mined
bitcoin-cli gettransaction $TXID
```

**Output** (before mining):
```json
{
  "confirmations": 0,
  "txid": "1234...",
  "time": 1701193200,
  "details": [...]
}
```

**Output** (after 1 confirmation):
```json
{
  "confirmations": 1,
  "blockhash": "0000000000000000000abc123...",
  "blockheight": 825000,
  "time": 1701193800,
  "details": [...]
}
```

**Monitoring**:
```python
def wait_for_confirmations(txid: str, required_confirmations: int = 6):
    while True:
        tx_data = bitcoin_get_transaction(txid)
        confirmations = tx_data.get('confirmations', 0)

        logger.info(f"Transaction {txid}: {confirmations}/{required_confirmations} confirmations")

        if confirmations >= required_confirmations:
            logger.info(f"Transaction confirmed! Block: {tx_data['blockhash']}")
            return tx_data

        # Wait for next block (~10 minutes)
        time.sleep(600)
```

**Progress Updates**: Leader posts updates to operator coordination channel:
- "Transaction broadcast: txid abc123..."
- "1/6 confirmations received"
- "6/6 confirmations received - ready for SPV proof"

---

#### Step 6.2: Verify BTC Arrived at Target Wallet

**Action**: Check active wallet received expected BTC amount.

**Technical Details**:
```bash
# Query active wallet balance
bitcoin-cli getreceivedbyaddress $ACTIVE_WALLET_ADDRESS 0

# Or check specific UTXO
bitcoin-cli gettxout $TXID $VOUT
```

**Expected**:
```json
{
  "bestblock": "00000000000000000000abc123...",
  "confirmations": 6,
  "value": 2.49995920,  // Matches proposal.outputValue
  "scriptPubKey": {
    "address": "bc1q...",  // Active wallet address
    "type": "witness_v0_keyhash"
  }
}
```

**Verification**:
```python
def verify_btc_received(txid: str, active_wallet_address: str, expected_value: int):
    # Get transaction output
    tx_data = bitcoin_get_transaction(txid)

    # Find output to active wallet
    for output in tx_data['vout']:
        if output['scriptPubKey']['address'] == active_wallet_address:
            actual_value = int(output['value'] * 100000000)  # Convert BTC to sats

            if actual_value == expected_value:
                logger.info(f"✅ Verified: {actual_value} sats received at {active_wallet_address}")
                return True
            else:
                logger.error(f"❌ Amount mismatch: expected {expected_value}, got {actual_value}")
                return False

    logger.error(f"❌ Output to {active_wallet_address} not found in transaction")
    return False
```

---

### Phase 7: SPV Proof Submission to Ethereum

**Duration**: 10-30 minutes

#### Step 7.1: Construct SPV Proof

**Action**: Build proof that Bitcoin transaction occurred and was confirmed.

**SPV (Simplified Payment Verification) Proof Components**:

1. **Bitcoin Transaction**: The complete, confirmed transaction
2. **Merkle Proof**: Proof that transaction is in a Bitcoin block
3. **Bitcoin Block Headers**: Headers proving the block exists in Bitcoin blockchain

**Technical Details**:

```python
def construct_spv_proof(txid: str) -> SPVProof:
    # 1. Get confirmed transaction
    tx_data = bitcoin_get_transaction(txid)
    raw_tx = tx_data['hex']
    block_hash = tx_data['blockhash']

    # 2. Get block with merkle tree
    block_data = bitcoin_get_block(block_hash, verbosity=2)

    # 3. Build merkle proof
    merkle_proof = build_merkle_proof(block_data['tx'], txid)

    # 4. Get block header
    block_header = bitcoin_get_block_header(block_hash)

    # 5. Get preceding block headers (for chain proof)
    # Ethereum Bridge needs N headers to verify chain validity
    preceding_headers = []
    current_hash = block_hash
    for i in range(6):  # 6 confirmations = 6 headers
        header = bitcoin_get_block_header(current_hash)
        preceding_headers.append(header)
        current_hash = header['previousblockhash']

    return SPVProof(
        transaction=raw_tx,
        merkle_proof=merkle_proof,
        block_header=block_header,
        preceding_headers=preceding_headers
    )
```

**Merkle Proof**:
A Merkle proof is a list of hashes that, when combined with the transaction hash, reconstructs the Merkle root in the Bitcoin block header.

```
Block Merkle Tree:
                    Root
                   /    \
                  H1     H2
                 /  \   /  \
               H3   H4 H5  H6
               /\   /\  /\  /\
              T1 T2 T3 T4 T5 T6

To prove T3 is in block:
Merkle Proof: [T4, H1, H2]

Verification:
hash(hash(T3, T4), H1) = H2_calc
hash(H2_calc, H2) = Root_calc
Root_calc == Block.merkleRoot? ✅ Proven
```

---

#### Step 7.2: Submit SPV Proof to Ethereum Bridge

**Action**: Call Bridge contract function to submit proof.

**Ethereum Contract Function**:
```solidity
// Bridge.sol
function submitMovingFundsProof(
    bytes calldata bitcoinTx,
    bytes calldata merkleProof,
    bytes calldata blockHeaders,
    uint256 txIndexInBlock
) external;
```

**Call Example**:
```bash
cast send $BRIDGE_ADDRESS \
  "submitMovingFundsProof(bytes,bytes,bytes,uint256)" \
  $BITCOIN_TX_HEX \
  $MERKLE_PROOF_HEX \
  $BLOCK_HEADERS_HEX \
  $TX_INDEX \
  --private-key $SUBMITTER_PRIVATE_KEY \
  --gas-limit 500000
```

**Who Submits**: Any operator or Threshold team member can submit (whoever submits first).

**Gas Cost**: ~300,000-500,000 gas (depends on proof size)
- At 30 gwei: 500,000 × 30 = 15,000,000 gwei = 0.015 ETH ≈ $30
- At 100 gwei: 500,000 × 100 = 50,000,000 gwei = 0.05 ETH ≈ $100

---

#### Step 7.3: Bridge Verifies SPV Proof

**Action**: Bridge contract executes verification logic (automatic, on-chain).

**Verification Steps** (inside smart contract):

1. **Parse Bitcoin Transaction**:
   - Extract inputs, outputs, amounts
   - Verify transaction structure is valid

2. **Verify Merkle Proof**:
   - Recompute Merkle root using tx and proof
   - Compare to Merkle root in block header

3. **Verify Block Headers**:
   - Check proof-of-work on each header (hash < target)
   - Verify headers form valid chain (each references previous)
   - Confirm headers are in Bitcoin blockchain

4. **Check Confirmations**:
   - Count confirmations (N headers after transaction block)
   - Require >= 6 confirmations

5. **Validate MovingFunds Action**:
   - Extract source wallet PKH from transaction
   - Extract target wallet PKH from transaction output
   - Confirm matches expected MovingFunds action

**If Verification Passes**:
- Event emitted: `MovingFundsProofSubmitted(walletPKH, txid, amount)`
- Wallet state updated (main UTXO changed)
- Deprecated wallet marked as having completed MovingFunds

**If Verification Fails**:
- Transaction reverts
- No state change
- Submitter lost gas fees
- Can resubmit correct proof

---

#### Step 7.4: Ethereum State Update

**Action**: Bridge contract updates wallet state on Ethereum.

**State Changes**:

1. **Deprecated Wallet**:
   ```solidity
   wallets[deprecatedWalletPKH].mainUtxo = EmptyUtxo;  // Wallet now empty
   wallets[deprecatedWalletPKH].state = WalletState.MovedFunds;
   ```

2. **Active Wallet**:
   ```solidity
   wallets[activeWalletPKH].mainUtxo = NewUtxo(
       txHash: movingFundsTxid,
       outputIndex: 0,
       value: outputValue
   );
   // Wallet's main UTXO now points to new BTC
   ```

3. **Events Emitted**:
   ```solidity
   emit MovingFundsCompleted(
       deprecatedWalletPKH,
       activeWalletPKH,
       movingFundsTxid,
       outputValue
   );
   ```

---

### Phase 8: Verification and Cleanup

**Duration**: 1-2 days

#### Step 8.1: Verify Wallet Balance On-Chain

**Action**: Confirm deprecated wallet shows 0 BTC on Ethereum.

**Technical Details**:
```bash
# Query wallet state
cast call $WALLET_REGISTRY_ADDRESS \
  "getWalletMainUtxo(bytes20)" \
  $DEPRECATED_WALLET_PKH

# Should return empty UTXO (0 value)
```

**Expected Output**:
```
(bytes32,uint32,uint64) = (
  0x0000000000000000000000000000000000000000000000000000000000000000,  // No txHash
  0,                                                                      // No vout
  0                                                                       // 0 value
)
```

---

#### Step 8.2: Verify Wallet Balance Off-Chain (Bitcoin)

**Action**: Confirm deprecated wallet shows 0 BTC on Bitcoin blockchain.

**Technical Details**:
```bash
# Check wallet balance on Bitcoin
bitcoin-cli getreceivedbyaddress $DEPRECATED_WALLET_ADDRESS 0

# Should return 0 (or minimal dust like 0.00000546 BTC = 546 sats)
```

**Dust Handling**: If minimal dust remains (<1000 sats), consider it acceptable (not economical to sweep).

---

#### Step 8.3: Mark Operator Eligible for Removal

**Action**: Update operator status in monitoring system.

**Technical Details**:
```python
# Update operator status
operator_status[deprecated_operator_address] = {
    'wallet_balance': 0,
    'wallet_empty_since': current_timestamp(),
    'eligible_for_removal_at': current_timestamp() + 7_days,
    'status': 'AWAITING_REMOVAL'
}
```

**Safety Buffer**: Wait 1 week at 0 BTC before actually removing operator.

**Rationale**: Ensures no pending transactions or edge cases.

---

#### Step 8.4: Notify Operators of Success

**Action**: Send confirmation to all operators.

**Communication**:

1. **Email** (to all operators):
   ```
   Subject: Manual Sweep Completed - Wallet 0x1234...5678

   ✅ Manual sweep successfully completed!

   Wallet: 0x1234567890abcdef1234567890abcdef12345678
   Bitcoin Transaction: 1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef
   Amount Moved: 2.5 BTC
   Target Wallet: 0xffb804c2de78576ad011f68a7df63d739b8c8155

   Confirmations: 6+
   SPV Proof: Submitted and verified on Ethereum

   Deprecated wallet now at 0 BTC.
   Operator 0x1234...5678 will be eligible for removal on 2025-12-06.
   ```

2. **Slack/Discord** (in #operator-consolidation channel):
   ```
   ✅ Manual Sweep Complete

   Wallet: 0x1234...5678
   Amount: 2.5 BTC → 0xffb8...8155
   Bitcoin TX: 1234...cdef

   Next: 1-week safety buffer, then operator removal.
   ```

---

## Data Structures and Parameters

### MovingFundsProposal Structure

```typescript
interface MovingFundsProposal {
  // Source wallet (being drained)
  walletPublicKeyHash: bytes20;

  // Target wallet (receiving BTC, same provider)
  targetWalletPublicKeyHash: bytes20;

  // Bitcoin transaction inputs (UTXOs to spend)
  utxos: Array<{
    txHash: bytes32;          // Previous transaction hash
    outputIndex: uint32;      // Output index (vout)
    value: uint64;            // Value in satoshis
  }>;

  // Bitcoin transaction output
  outputValue: uint64;        // Amount sent to target (sats)

  // Fee
  feeRate: uint32;            // Fee rate in sat/vB
  totalFee: uint64;           // Total fee in satoshis

  // Metadata
  coordinationBlock: uint256; // Target Ethereum block for coordination
  provider: string;           // Provider organization name
  estimatedGasCost: uint256;  // Estimated Ethereum gas for SPV proof
}
```

---

### CoordinationMessage Structure

```typescript
interface CoordinationMessage {
  // Identity
  memberId: number;                  // First signer controlled by leader

  // Timing
  coordinationBlock: uint256;        // Block number when window opens

  // Wallet being coordinated
  walletPublicKeyHash: bytes20;      // 20-byte wallet PKH

  // Proposal (serialized)
  proposal: bytes;                   // MovingFunds proposal bytes

  // Signature (added by leader)
  signature: {
    r: bytes32;
    s: bytes32;
    v: uint8;
  };
}
```

---

### SPVProof Structure

```typescript
interface SPVProof {
  // Bitcoin transaction
  bitcoinTransaction: bytes;         // Raw Bitcoin transaction (hex)

  // Merkle proof
  merkleProof: bytes;                // Hashes proving tx in block
  txIndexInBlock: uint256;           // Position of tx in block

  // Block headers
  confirmingBlockHeader: bytes;      // Header of block containing tx (80 bytes)
  precedingHeaders: bytes;           // Previous block headers for chain proof

  // Metadata
  blockHeight: uint256;              // Bitcoin block height
  confirmations: uint256;            // Number of confirmations
}
```

---

### Operator Status Tracking

```typescript
interface OperatorStatus {
  // Identity
  operatorAddress: address;          // Ethereum address
  provider: string;                  // "BOAR" | "STAKED" | "P2P"

  // Wallet info
  walletPublicKeyHash: bytes20;      // Associated wallet PKH
  walletBitcoinAddress: string;      // Bitcoin address (derived)

  // Balance tracking
  initialBalance: uint64;            // Balance at consolidation start (sats)
  currentBalance: uint64;            // Current balance (sats)
  lastUpdated: uint256;              // Timestamp of last balance update

  // Draining progress
  naturalDrainedAmount: uint64;      // BTC drained via redemptions (sats)
  manualSweptAmount: uint64;         // BTC moved via manual sweep (sats)
  percentDrained: uint8;             // 0-100 percent

  // Status
  status: "ACTIVE" | "DEPRECATED" | "AWAITING_REMOVAL" | "REMOVED";
  walletEmptySince: uint256;         // Timestamp when wallet reached 0 BTC
  eligibleForRemovalAt: uint256;     // Timestamp + 1 week safety buffer

  // Manual sweep tracking
  manualSweepRequired: boolean;      // True if >20% after Week 4
  manualSweepCompleted: boolean;     // True if sweep executed
  manualSweepTxid: bytes32;          // Bitcoin txid of sweep (if executed)
}
```

---

## Bitcoin Transaction Details

### Transaction Structure

A typical MovingFunds Bitcoin transaction:

```
Transaction ID: 1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef

Version: 2
Inputs: 2
Outputs: 1
Locktime: 0

Size: 340 bytes (typical)
Weight: 880 weight units (if SegWit)
Fee: 4,080 sats (12 sat/vB)

Input 1:
  Previous Transaction: abc1234567890...
  Output Index: 0
  ScriptSig: <signature> <pubkey>
  Sequence: 0xFFFFFFFF
  Value: 150,000,000 sats (1.5 BTC)

Input 2:
  Previous Transaction: def9876543210...
  Output Index: 1
  ScriptSig: <signature> <pubkey>
  Sequence: 0xFFFFFFFF
  Value: 100,000,000 sats (1.0 BTC)

Output 1:
  Value: 249,995,920 sats (2.4999592 BTC)
  ScriptPubKey: OP_DUP OP_HASH160 <active_wallet_hash> OP_EQUALVERIFY OP_CHECKSIG
  Address: bc1q... (active wallet)

Total Input: 250,000,000 sats
Total Output: 249,995,920 sats
Fee: 4,080 sats (250,000,000 - 249,995,920)
```

---

### Script Types

**Input Script (ScriptSig)**:
```
<signature> <public_key>

Where:
- signature: 71-73 bytes (DER-encoded ECDSA signature)
- public_key: 33 bytes (compressed) or 65 bytes (uncompressed)
```

**Output Script (ScriptPubKey) - P2PKH**:
```
OP_DUP OP_HASH160 <pubkey_hash> OP_EQUALVERIFY OP_CHECKSIG

Where:
- OP_DUP: 0x76
- OP_HASH160: 0xa9
- <pubkey_hash>: 20 bytes (RIPEMD160(SHA256(pubkey)))
- OP_EQUALVERIFY: 0x88
- OP_CHECKSIG: 0xac
```

**Output Script (ScriptPubKey) - P2WPKH (SegWit)**:
```
0 <pubkey_hash>

Where:
- 0: OP_0 (witness version 0)
- <pubkey_hash>: 20 bytes
```

---

### Fee Calculation

**Formula**:
```
Fee (sats) = Transaction Size (bytes) × Fee Rate (sat/vB)
```

**Transaction Size Estimation**:
```
Base Size:
- Version: 4 bytes
- Input Count: 1 byte
- Output Count: 1 byte
- Locktime: 4 bytes
- Total Base: 10 bytes

Per Input (P2PKH):
- Previous TX Hash: 32 bytes
- Output Index: 4 bytes
- Script Length: 1 byte
- ScriptSig: ~107 bytes (signature ~72 + pubkey ~33 + opcodes)
- Sequence: 4 bytes
- Total Per Input: 148 bytes

Per Output (P2PKH):
- Value: 8 bytes
- Script Length: 1 byte
- ScriptPubKey: 25 bytes
- Total Per Output: 34 bytes

Example Transaction (2 inputs, 1 output):
10 + (2 × 148) + (1 × 34) = 10 + 296 + 34 = 340 bytes

Fee at 12 sat/vB:
340 × 12 = 4,080 sats = 0.0000408 BTC ≈ $2 at $50k/BTC
```

---

### Transaction Verification

**Verification by Bitcoin Nodes**:

1. **Input Validation**:
   - Previous outputs (UTXOs) exist and are unspent
   - Signatures are valid (verify using public keys)
   - Sum of input values ≥ sum of output values + fee

2. **Script Execution**:
   - Execute ScriptSig + ScriptPubKey
   - Result must be TRUE (1 on stack)

3. **Consensus Rules**:
   - Transaction size < 100 KB
   - No double-spends (inputs not already spent)
   - Output values > 0 (no negative outputs)

---

## Ethereum Smart Contract Interactions

### Contracts Involved

1. **Allowlist.sol**: Manages operator weights (already deployed)
2. **WalletRegistry.sol**: Tracks wallet state and UTXOs
3. **Bridge.sol**: Accepts SPV proofs and updates state
4. **WalletProposalValidator.sol**: Validates MovingFunds proposals

---

### Key Contract Functions

#### WalletProposalValidator.validateMovingFundsProposal()

```solidity
function validateMovingFundsProposal(
    bytes20 walletPubKeyHash,
    bytes calldata movingFundsProposal
) external view returns (bool) {
    // Decode proposal
    (bytes20 targetWalletPKH, bytes memory utxos, uint64 outputValue) =
        decodeMovingFundsProposal(movingFundsProposal);

    // Get source wallet
    Wallet memory sourceWallet = walletRegistry.wallets(walletPubKeyHash);

    // Check 1: Wallet is LIVE
    require(sourceWallet.state == WalletState.LIVE, "Wallet not live");

    // Check 2: No pending MovingFunds
    require(!sourceWallet.movingFundsPending, "MovingFunds pending");

    // Check 3: Target wallet exists and is LIVE
    Wallet memory targetWallet = walletRegistry.wallets(targetWalletPKH);
    require(targetWallet.state == WalletState.LIVE, "Target not live");

    // Check 4: UTXOs match wallet's main UTXO
    require(utxosMatchMainUtxo(utxos, sourceWallet.mainUtxo), "UTXO mismatch");

    // Check 5: Fee calculation correct
    uint64 totalInput = calculateTotalInput(utxos);
    uint64 expectedOutput = totalInput - calculateFee(utxos.length);
    require(outputValue == expectedOutput, "Fee incorrect");

    return true;
}
```

---

#### Bridge.submitMovingFundsProof()

```solidity
function submitMovingFundsProof(
    bytes calldata bitcoinTx,
    bytes calldata merkleProof,
    bytes calldata blockHeaders,
    uint256 txIndexInBlock
) external {
    // 1. Verify SPV proof
    require(verifySPVProof(bitcoinTx, merkleProof, blockHeaders, txIndexInBlock), "Invalid SPV proof");

    // 2. Parse Bitcoin transaction
    (bytes20 sourceWalletPKH, bytes20 targetWalletPKH, uint64 outputValue) =
        parseBitcoinTransaction(bitcoinTx);

    // 3. Verify MovingFunds action was authorized
    require(walletRegistry.isMovingFundsAuthorized(sourceWalletPKH), "Not authorized");

    // 4. Update source wallet state
    walletRegistry.updateWallet(sourceWalletPKH, EmptyUtxo, WalletState.MovedFunds);

    // 5. Update target wallet state
    bytes32 txid = sha256(sha256(bitcoinTx));  // Bitcoin double SHA256
    Utxo memory newUtxo = Utxo(txid, 0, outputValue);
    walletRegistry.updateWallet(targetWalletPKH, newUtxo, WalletState.LIVE);

    // 6. Emit event
    emit MovingFundsCompleted(sourceWalletPKH, targetWalletPKH, txid, outputValue);
}
```

---

#### SPV Proof Verification Logic

```solidity
function verifySPVProof(
    bytes calldata bitcoinTx,
    bytes calldata merkleProof,
    bytes calldata blockHeaders,
    uint256 txIndexInBlock
) internal view returns (bool) {
    // 1. Calculate transaction hash (Bitcoin uses double SHA256)
    bytes32 txHash = sha256(sha256(bitcoinTx));

    // 2. Verify Merkle proof
    bytes32 computedRoot = computeMerkleRoot(txHash, merkleProof, txIndexInBlock);

    // 3. Extract Merkle root from block header
    bytes32 blockMerkleRoot = extractMerkleRoot(blockHeaders[0:80]);
    require(computedRoot == blockMerkleRoot, "Merkle root mismatch");

    // 4. Verify block headers form valid chain
    require(verifyBlockChain(blockHeaders), "Invalid block chain");

    // 5. Verify proof-of-work on each block
    for (uint i = 0; i < blockHeaders.length / 80; i++) {
        bytes memory header = blockHeaders[i*80:(i+1)*80];
        require(verifyProofOfWork(header), "Invalid PoW");
    }

    // 6. Check confirmations (number of headers provided)
    uint confirmations = blockHeaders.length / 80;
    require(confirmations >= 6, "Insufficient confirmations");

    return true;
}
```

---

## RFC-12 Coordination Protocol

### Coordination Windows

**Frequency**: Every 900 blocks (~3 hours)

**Calculation**:
```python
if current_block % 900 == 0:
    # Coordination window is open
    coordination_block = current_block
else:
    # Calculate next window
    blocks_until_next = 900 - (current_block % 900)
    coordination_block = current_block + blocks_until_next
```

**Example**:
```
Current block: 12,345,678
12,345,678 % 900 = 78

Next coordination window:
12,345,678 + (900 - 78) = 12,346,500

Window duration: Blocks 12,346,500 to 12,346,600 (100 blocks)
Time: ~20 minutes
```

---

### Leader Selection Algorithm

**Deterministic Selection**:
All operators independently calculate the same leader using:

```python
def select_leader(wallet_pkh, coordination_block, operators):
    # 1. Get safe block (32 blocks before coordination window)
    safe_block = coordination_block - 32
    safe_block_hash = eth_get_block_hash(safe_block)

    # 2. Generate seed
    seed = sha256(wallet_pkh + safe_block_hash)

    # 3. Shuffle operators deterministically
    rng = RNG(seed)
    shuffled = rng.shuffle(operators)

    # 4. First operator is leader
    return shuffled[0]
```

**Properties**:
- **Deterministic**: Same inputs always produce same leader
- **Unpredictable**: Cannot predict leader until safe block is mined
- **Fair**: Each operator has equal probability over time

---

### Leader Responsibilities

1. **Propose Action**: Construct and broadcast CoordinationMessage
2. **Collect Signatures**: Receive partial signatures from followers
3. **Aggregate**: Combine signatures into complete ECDSA signature
4. **Execute**: Broadcast Bitcoin transaction
5. **Report**: Announce transaction ID to all operators

---

### Follower Responsibilities

1. **Validate Leader**: Verify message sender is expected leader
2. **Validate Timing**: Confirm message within coordination window
3. **Validate Proposal**: Run on-chain and off-chain checks
4. **Sign**: Generate partial signature if valid
5. **Submit**: Send signature share to leader

---

### Coordination Window Timeline

```
Block 12,346,500: Coordination window opens
    ↓
Block 12,346,502: Leader calculates (knows they are leader)
    ↓
Block 12,346,505: Leader broadcasts CoordinationMessage
    ↓
Block 12,346,510: Followers validate and start signing
    ↓
Block 12,346,520: Leader receives 51+ signature shares
    ↓
Block 12,346,525: Leader aggregates signatures
    ↓
Block 12,346,530: Leader constructs Bitcoin transaction
    ↓
Block 12,346,535: Leader broadcasts to Bitcoin network
    ↓
Block 12,346,580: Coordination window 80% complete (deadline)
    ↓
Block 12,346,600: Coordination window closes

Success: Transaction broadcast by block 12,346,535
```

---

### Fallback: If Leader Doesn't Act

If leader fails to propose (offline, malfunction, censoring):

**Retry in Next Window**:
- Wait for next coordination window (3 hours later)
- New leader selected deterministically
- Repeat process

**Different Leader Each Time**:
- Each window has different safe block hash
- Different seed → different shuffle → different leader
- Probability that 3 consecutive leaders all fail: (1/18)³ = 0.017%

---

## Failure Scenarios and Recovery

### Scenario 1: Insufficient Operators Online (<51)

**Problem**: Only 8 of 18 operators available during coordination window.

**Detection**:
```python
if len(signature_shares) < 51:
    logger.error(f"Threshold not met: only {len(signature_shares)} signatures")
```

**Recovery**:
1. Leader announces failure in coordination channel
2. Threshold team posts reminder to all operators
3. Schedule coordination attempt for next window (3 hours later)
4. Ensure more operators available (direct communication with providers)

**Prevention**:
- Send 48-hour advance notice
- Confirm operator availability before window
- Schedule during business hours (not weekends/holidays)

---

### Scenario 2: Leader Offline or Unresponsive

**Problem**: Selected leader's node is offline or malfunctioning.

**Detection**:
```python
# Followers detect no CoordinationMessage after 50 blocks
if current_block > coordination_block + 50 and not received_coordination_message():
    logger.warning("Leader appears offline or unresponsive")
```

**Recovery**:
1. Wait for coordination window to close (no action taken)
2. Next window opens in 3 hours → new leader selected
3. New leader attempts coordination

**Probability**:
- With 18 operators, 10+ must be online (56% threshold)
- If 80% of operators online (14/18), probability leader is offline: 22%
- Multiple windows ensure success: 99%+ probability within 3 attempts

---

### Scenario 3: Invalid Proposal Parameters

**Problem**: MovingFunds proposal contains errors (wrong UTXOs, incorrect fee, etc.).

**Detection**:
```python
def validate_proposal_offchain(proposal):
    if not verify_utxos_unspent(proposal.utxos):
        logger.error("UTXOs are spent or don't exist")
        return False

    if not verify_fee_reasonable(proposal.feeRate):
        logger.error(f"Fee unreasonable: {proposal.feeRate} sat/vB")
        return False

    return True
```

**Recovery**:
1. Followers reject proposal (do not sign)
2. Leader receives <51 signatures → coordination fails
3. Threshold team corrects proposal parameters
4. Retry in next coordination window with fixed parameters

**Prevention**:
- Double-check UTXO queries before constructing proposal
- Validate proposal locally before giving to leader
- Use automated tools to generate correct parameters

---

### Scenario 4: Bitcoin Transaction Doesn't Confirm (Low Fee)

**Problem**: Bitcoin transaction broadcast but stuck in mempool due to low fee.

**Detection**:
```bash
# Transaction in mempool for >2 hours without confirmation
bitcoin-cli getmempoolentry $TXID
# Shows confirmations: 0, time: 2+ hours ago
```

**Recovery**:
1. **Option A - Wait**: If fee is reasonable, wait longer (could take days)
2. **Option B - Replace-By-Fee (RBF)**: Broadcast replacement transaction with higher fee
   ```bash
   # Construct new transaction with same inputs, higher fee
   # Mark original transaction as RBF-enabled (sequence < 0xFFFFFFFE)
   bitcoin-cli createrawtransaction '[...]' '{...}' 0 true
   ```
3. **Option C - Child-Pays-For-Parent (CPFP)**: Spend output with higher fee transaction

**Prevention**:
- Check current mempool congestion before coordination
- Use recommended fee rates (not lowest)
- Enable RBF flag on transaction (allows replacement)

---

### Scenario 5: SPV Proof Submission Fails

**Problem**: SPV proof transaction reverts on Ethereum (invalid proof or gas limit).

**Detection**:
```bash
# Ethereum transaction reverts
cast send ...
# Error: transaction reverted
```

**Recovery**:
1. **Debug**: Check revert reason
   ```bash
   cast call $BRIDGE_ADDRESS "getRevertReason(bytes32)" $TX_HASH
   ```
2. **Fix Issue**:
   - If invalid Merkle proof: Reconstruct proof correctly
   - If insufficient gas: Increase gas limit
   - If insufficient confirmations: Wait for more blocks
3. **Resubmit**: Call `submitMovingFundsProof` again

**Note**: Bitcoin transaction is already confirmed, so BTC has moved. SPV proof just tells Ethereum about it.

---

### Scenario 6: Wallet Has Dust Remaining

**Problem**: After sweep, wallet has minimal dust (e.g., 546 sats).

**Reason**: Bitcoin has "dust limit" - outputs below ~546 sats are non-standard and won't propagate.

**Decision**:
- **Accept dust**: Not economical to sweep (fee > dust value)
- **Mark as complete**: Wallet effectively empty

**On-Chain State**:
```solidity
// Wallet marked as MovedFunds even with dust
wallets[walletPKH].state = WalletState.MovedFunds;
```

**Operator Removal**: Proceed with operator removal (dust is negligible).

---

### Scenario 7: Wrong Target Wallet in Proposal

**Problem**: Proposal accidentally specifies wrong target wallet (e.g., different provider).

**Detection**:
```python
def validate_same_provider(source_pkh, target_pkh):
    source_provider = get_wallet_provider(source_pkh)
    target_provider = get_wallet_provider(target_pkh)

    if source_provider != target_provider:
        logger.error(f"Provider mismatch: {source_provider} != {target_provider}")
        return False

    return True
```

**Recovery**:
- **Before Signing**: Followers detect error, reject proposal, coordination fails
- **After Signing**: If BTC already moved to wrong wallet, requires manual coordination between providers to return BTC

**Prevention**:
- Strict validation in proposal construction
- Followers MUST check provider match before signing
- Use provider-specific proposal templates

---

## Cost Analysis

### Bitcoin Transaction Fees

**Variable Factors**:
- Transaction size (depends on # of UTXOs)
- Mempool congestion (fee rate in sat/vB)
- Urgency (faster confirmation = higher fee)

**Typical Costs**:

| Scenario | Size | Fee Rate | Total Fee | USD Cost |
|----------|------|----------|-----------|----------|
| Small (1 UTXO) | 192 bytes | 10 sat/vB | 1,920 sats | $0.96 |
| Medium (2 UTXOs) | 340 bytes | 12 sat/vB | 4,080 sats | $2.04 |
| Large (5 UTXOs) | 784 bytes | 15 sat/vB | 11,760 sats | $5.88 |
| Urgent (high fee) | 340 bytes | 50 sat/vB | 17,000 sats | $8.50 |

*Assuming BTC = $50,000*

**Worst Case** (extreme mempool congestion):
- Fee rate: 100 sat/vB
- Transaction size: 784 bytes (5 UTXOs)
- Total fee: 78,400 sats = $39.20

---

### Ethereum Gas Fees

**SPV Proof Submission**:

| Gas Price | Gas Used | Total Cost (ETH) | USD Cost |
|-----------|----------|------------------|----------|
| 20 gwei | 400,000 | 0.008 ETH | $16 |
| 50 gwei | 400,000 | 0.020 ETH | $40 |
| 100 gwei | 400,000 | 0.040 ETH | $80 |
| 200 gwei | 500,000 | 0.100 ETH | $200 |

*Assuming ETH = $2,000*

**Gas Breakdown**:
- Merkle proof verification: ~100,000 gas
- Block header verification: ~150,000 gas
- State updates (storage writes): ~100,000 gas
- Event emissions: ~50,000 gas

---

### Total Cost Per Manual Sweep

**Typical Sweep**:
- Bitcoin fee: $2-5
- Ethereum gas: $20-50
- **Total: $22-55 per wallet**

**Worst Case** (high congestion):
- Bitcoin fee: $10-40
- Ethereum gas: $80-200
- **Total: $90-240 per wallet**

---

### Total Project Cost

**If All 15 Wallets Require Manual Sweeps**:

| Scenario | Cost per Wallet | Total (15 wallets) |
|----------|-----------------|-------------------|
| Optimistic | $25 | $375 |
| Typical | $50 | $750 |
| Worst Case | $150 | $2,250 |

**Expected Reality** (hybrid approach):
- 50-70% drain naturally (free)
- 30-50% require manual sweeps (7-8 wallets)
- **Estimated Cost: $350-600**

---

### Who Pays?

**DAO Treasury**: All costs paid from Threshold DAO treasury.
- Governance approval obtained in advance
- Budget allocated from cost savings

**Individual Operators**: No out-of-pocket costs.

---

## Timing and Scheduling

### Coordination Window Schedule

**Frequency**: Every 900 blocks = ~3 hours

**Daily Windows**:
```
00:00 UTC - Block 12,345,000
03:00 UTC - Block 12,345,900
06:00 UTC - Block 12,346,800
09:00 UTC - Block 12,347,700
12:00 UTC - Block 12,348,600
15:00 UTC - Block 12,349,500
18:00 UTC - Block 12,350,400
21:00 UTC - Block 12,351,300
```

**Preferred Timing** (for operator availability):
- **09:00-18:00 UTC**: Business hours in Europe and Asia
- **15:00-21:00 UTC**: Business hours in Americas
- **Avoid**: Weekends, holidays, midnight hours

---

### Bitcoin Confirmation Time

**Expected**: 6 confirmations × 10 minutes = ~60 minutes

**Actual Range**:
- **Fast**: 30-45 minutes (lucky with fast blocks)
- **Typical**: 60-90 minutes
- **Slow**: 2-3 hours (if unlucky with block times)

**Factors**:
- Bitcoin's variable block time (target 10 min, actual 8-12 min)
- Mempool congestion (lower fee = longer wait)
- Network hashrate fluctuations

---

### Complete Sweep Timeline

**End-to-End Duration**:
```
T+0 hours:   Coordination window opens
T+0.5 hours: Leader broadcasts transaction to Bitcoin
T+1.5 hours: Bitcoin transaction gets 6 confirmations
T+2 hours:   SPV proof constructed
T+2.5 hours: SPV proof submitted to Ethereum
T+3 hours:   Verification complete

Total: ~3 hours from coordination start to completion
```

**Worst Case** (failures and retries):
```
T+0 hours:   First coordination attempt (failed - <51 operators)
T+3 hours:   Second coordination attempt (failed - invalid proposal)
T+6 hours:   Third coordination attempt (success)
T+7.5 hours: Bitcoin transaction confirmed
T+8 hours:   SPV proof submitted
T+9 hours:   Complete

Total: ~9 hours (3 coordination attempts)
```

---

### Project Timeline

**Week 4 Assessment** (2025-12-01):
- Identify 7-8 wallets requiring manual sweeps

**Week 5 Execution** (2025-12-02 to 2025-12-06):
- Day 1: Sweep wallets 1-3 (BOAR)
- Day 2: Sweep wallets 4-6 (STAKED)
- Day 3: Sweep wallets 7-8 (P2P)
- Day 4-5: Verify all sweeps, handle retries if needed

**Week 6+ Operator Removal** (2025-12-12 onwards):
- 1 week safety buffer after 0 BTC
- Progressive removal in batches

---

## Security Considerations

### Threshold Cryptography Security

**Private Key Never Reconstructed**:
- Each operator holds a secret share
- 51 shares can produce valid signature
- Full private key NEVER exists in memory
- Even leader cannot extract private key from signature shares

**Properties**:
- **Threshold Security**: Attacker needs 51+ shares to compromise wallet
- **Robustness**: System works even if 49 operators offline
- **Non-Interactivity**: Signing requires coordination but is non-interactive cryptographically

---

### SPV Proof Security

**What SPV Proves**:
- Transaction exists in a Bitcoin block
- Block has valid proof-of-work
- Block is part of Bitcoin's longest chain
- Transaction has N confirmations

**What SPV Doesn't Prove**:
- Transaction is economically final (6 confirmations is convention)
- No future reorganization will occur (low probability after 6 confirms)

**Bridge Contract Validation**:
- Requires 6 confirmations minimum
- Verifies proof-of-work on each block
- Checks Merkle proof correctness
- Validates block chain integrity

**Attack Resistance**:
- **51% Attack**: Attacker would need >50% of Bitcoin hashrate to create fake blocks
- **Merkle Tree Attack**: Cryptographically impossible to fake without collision
- **Reorganization Attack**: Probability of 6-block reorg: ~0.0001%

---

### Proposal Validation Security

**Multiple Layers**:
1. **Leader Local Validation**: Before broadcasting
2. **On-Chain Validation**: WalletProposalValidator contract
3. **Follower Off-Chain Validation**: Each operator independently checks
4. **Ethereum Bridge Validation**: Final SPV proof verification

**Defense in Depth**: Malicious proposal must pass 4 independent checks to succeed.

---

### Operator Security

**Private Key Protection**:
- Operators store key shares in secure hardware (HSM, encrypted storage)
- Key shares never transmitted over network
- Signatures generated locally, only shares transmitted

**Node Security**:
- Operators run nodes on secure infrastructure
- Regular security updates and monitoring
- Access controls and firewalls

**Communication Security**:
- Peer-to-peer network uses TLS encryption
- CoordinationMessages are signed (prevents spoofing)
- Leader identity verified by signature

---

### Transaction Irreversibility

**Bitcoin Transaction**: Once confirmed (6+ blocks), effectively irreversible.
- Reorganizations >6 blocks are extremely rare
- If sweep goes to wrong wallet, requires manual return (no automatic rollback)

**Prevention**:
- Multiple layers of validation before signing
- Operators must verify target wallet is correct
- Use provider-specific proposal templates (reduces human error)

---

## Monitoring and Verification

### Real-Time Monitoring

**Grafana Dashboard Metrics**:
1. **Coordination Status**:
   - Current coordination block
   - Time until next window
   - Active operators count

2. **Manual Sweep Progress**:
   - Wallets requiring sweeps (list)
   - Coordination attempts (success/failure)
   - Bitcoin transactions in progress (txids)
   - SPV proofs submitted (pending/confirmed)

3. **Wallet Balances**:
   - Deprecated wallet balances (BTC)
   - Active wallet balances (BTC)
   - Total BTC moved today/week

4. **Operator Participation**:
   - Operators online/offline
   - Signature participation rate
   - Leader selection history

---

### Alerts

**Critical Alerts** (immediate action required):

1. **Coordination Failure**:
   - Trigger: <51 signatures after 80 blocks
   - Action: Contact providers, ensure operators online for next window

2. **Bitcoin Transaction Stuck**:
   - Trigger: Transaction in mempool >2 hours without confirmation
   - Action: Consider RBF (Replace-By-Fee) or higher fee

3. **SPV Proof Revert**:
   - Trigger: SPV proof submission fails on Ethereum
   - Action: Debug revert reason, resubmit corrected proof

**Warning Alerts** (monitor closely):

1. **Low Operator Count**:
   - Trigger: <12 operators online (below comfortable margin)
   - Action: Notify providers to bring more operators online

2. **High Bitcoin Fees**:
   - Trigger: Mempool fee >50 sat/vB
   - Action: Consider delaying sweep to lower-fee period (if not urgent)

3. **Ethereum Gas Spike**:
   - Trigger: Gas price >100 gwei
   - Action: Delay SPV proof submission until gas drops (if not urgent)

---

### Post-Sweep Verification Checklist

After each manual sweep, verify:

- [ ] Bitcoin transaction has 6+ confirmations
- [ ] BTC arrived at correct active wallet (address matches provider)
- [ ] Amount matches expected (total input - fee = output)
- [ ] SPV proof submitted and verified on Ethereum
- [ ] Deprecated wallet on-chain state updated (mainUtxo = empty)
- [ ] Active wallet on-chain state updated (mainUtxo = new tx)
- [ ] Operator status updated (AWAITING_REMOVAL)
- [ ] Dashboard reflects 0 BTC balance
- [ ] Success notification sent to all operators
- [ ] Cost recorded (Bitcoin fee + Ethereum gas)

---

## Operator Instructions

### For All Operators

**Before Coordination Window**:

1. **Verify Node Health**:
   ```bash
   # Check node is running
   systemctl status tbtc-node

   # Check sync status
   curl http://localhost:8080/health
   # Should return: {"status": "healthy", "synced": true}
   ```

2. **Confirm Ethereum Sync**:
   ```bash
   # Check block height matches current
   curl http://localhost:8545 -X POST \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
   ```

3. **Verify Bitcoin Access**:
   ```bash
   # Test Electrum connection
   electrum-client getinfo
   ```

4. **Acknowledge Coordination Notice**:
   - Reply to email/Slack with ✅ confirming availability
   - Ensure you'll be monitoring during coordination window

---

**During Coordination Window** (20 minutes):

1. **Monitor Coordination Channel**:
   - Watch for leader announcement
   - Check for CoordinationMessage broadcast

2. **Validate Proposal** (if you're a follower):
   - Wait for CoordinationMessage from leader
   - Run local validation (on-chain + off-chain)
   - If valid, participate in signing (automatic by node)
   - If invalid, log reason and DO NOT sign

3. **Execute Proposal** (if you're the leader):
   - Receive proposal parameters from Threshold team
   - Validate locally
   - Construct and broadcast CoordinationMessage
   - Collect signature shares (wait for 51+)
   - Aggregate signatures
   - Construct and broadcast Bitcoin transaction
   - Announce txid to all operators

4. **Monitor Progress**:
   - Watch for Bitcoin transaction ID announcement
   - Verify transaction appears in Bitcoin mempool
   - Monitor confirmations

---

**After Coordination Window**:

1. **Verify Bitcoin Transaction**:
   ```bash
   # Check transaction status
   bitcoin-cli gettransaction $TXID

   # Verify it reached target wallet
   bitcoin-cli getreceivedbyaddress $TARGET_WALLET_ADDRESS
   ```

2. **Wait for SPV Proof**:
   - Monitor Ethereum for SPV proof submission
   - Verify on-chain state updated correctly

3. **Update Records**:
   - Log sweep details (txid, amount, timestamp)
   - Update local operator status tracking

4. **Report Any Issues**:
   - If transaction didn't confirm: Report to coordination channel
   - If SPV proof failed: Share error details with team
   - If discrepancies in amounts: Escalate immediately

---

### For Deprecated Operators (Wallets Being Drained)

**Additional Responsibilities**:

1. **Monitor Your Wallet Balance**:
   ```bash
   # Check current balance
   bitcoin-cli getreceivedbyaddress $YOUR_DEPRECATED_WALLET_ADDRESS 0
   ```

2. **Participate in Coordination**:
   - Your node MUST be online during sweep
   - You will participate in threshold signing (part of 51/100)

3. **Verify Wallet Emptied**:
   ```bash
   # After sweep, confirm 0 BTC (or minimal dust)
   bitcoin-cli getreceivedbyaddress $YOUR_DEPRECATED_WALLET_ADDRESS 0
   # Should return: 0.00000000 or ~0.00000546 (dust)
   ```

4. **Wait for Decommissioning Approval**:
   - DO NOT shut down node immediately after 0 BTC
   - Wait for 1-week safety buffer
   - Wait for explicit email confirmation from Threshold team
   - Only then proceed to decommission node

---

### For Active Operators (Receiving BTC)

**Additional Responsibilities**:

1. **Monitor Your Wallet Balance**:
   ```bash
   # Check balance increase after sweep
   bitcoin-cli getreceivedbyaddress $YOUR_ACTIVE_WALLET_ADDRESS 0
   ```

2. **Verify Amount Received**:
   - Compare received amount to expected (should match proposal parameters)
   - Report any discrepancies immediately

3. **Continue Normal Operations**:
   - Your wallet remains active after consolidation
   - Continue participating in DKG, redemptions, etc.
   - Monitor for increased leader probability (~33% after consolidation)

---

## Emergency Contacts

### Threshold Team

- **Engineering Lead**: [email - TBD]
- **DevOps Coordinator**: [email - TBD]
- **Emergency Hotline**: [phone - TBD]

### Provider Liaisons

- **BOAR**: [contact - TBD]
- **STAKED**: [contact - TBD]
- **P2P**: [contact - TBD]

### Communication Channels

- **Slack**: #operator-consolidation
- **Discord**: #tbtc-operators
- **Email**: operators@threshold.network

---

## Conclusion

Manual sweeps are a **fallback mechanism** designed to ensure the consolidation completes on time if natural draining is insufficient. The process leverages:

✅ **Existing tBTC infrastructure** (MovingFunds mechanism)
✅ **RFC-12 decentralized coordination** (no central authority)
✅ **Threshold cryptography** (secure, robust signing)
✅ **SPV proofs** (trustless Ethereum ↔ Bitcoin bridge)

**Expected Reality**:
- 50-70% of wallets drain naturally (no manual intervention)
- 30-50% require manual sweeps (7-8 wallets)
- Total project cost: $350-600

**Timeline**:
- Week 4 assessment determines which wallets need sweeps
- Week 5 execution (3-5 days to complete all sweeps)
- Week 6+ operator removal (after 1-week safety buffer)

**Success Criteria**:
- All 15 deprecated wallets at 0 BTC (or minimal dust)
- All 15 deprecated operators removed from allowlist
- 83% cost reduction achieved
- Zero tBTC service interruptions throughout process

---

**Document Version**: 1.0
**Last Updated**: 2025-10-10
**Next Review**: After first manual sweep execution (TBD)

**Questions?** Contact Threshold engineering team via #operator-consolidation channel.

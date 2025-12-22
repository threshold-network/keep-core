# Wallet Registry Operations Guide

This guide covers all operations that can be performed against the WalletRegistry and sortition pool, similar to generating a new wallet.

## Main Operations Categories

### 1. Wallet Lifecycle Operations

#### Request New Wallet (Triggers DKG)
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```
**What it does:**
- Locks the sortition pool
- Requests a relay entry from Random Beacon
- Triggers DKG process
- Selects operators from the pool

#### Close Wallet
```bash
# First, get wallet ID (from DKG result or events)
WALLET_ID="0x..."  # 32-byte wallet ID

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry close-wallet \
  "$WALLET_ID" --submit --config configs/config.toml --developer
```
**What it does:**
- Closes an existing wallet
- Removes wallet from registry
- Only wallet owner can call this

#### Get Wallet Information
```bash
# Check if wallet is registered
WALLET_ID="0x..."
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-wallet-registered \
  "$WALLET_ID" --config configs/config.toml --developer

# Get wallet details
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet \
  "$WALLET_ID" --config configs/config.toml --developer

# Get wallet public key
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-public-key \
  "$WALLET_ID" --config configs/config.toml --developer
```

### 2. DKG Operations

#### Select Group (View Function - Free)
```bash
# Note: Requires sortition pool to be locked (during DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer
```
**What it does:**
- Selects operators from sortition pool for DKG
- Uses current seed/relay entry
- Returns array of operator IDs
- **Free** - view function, no gas cost
- **Requires**: Sortition pool must be locked (during DKG)

#### Submit DKG Result
```bash
# This is typically done automatically by nodes, but can be called manually
# Requires DKG result data structure
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry submit-dkg-result \
  --config configs/config.toml --developer
```
**What it does:**
- Submits DKG result to chain
- Starts challenge period
- Requires valid DKG result with signatures

#### Approve DKG Result
```bash
# After challenge period passes
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry approve-dkg-result \
  --config configs/config.toml --developer
```
**What it does:**
- Approves submitted DKG result
- Creates wallet
- Unlocks sortition pool
- Submitter receives ETH reimbursement

#### Challenge DKG Result
```bash
# If malicious result detected
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry challenge-dkg-result \
  --config configs/config.toml --developer
```
**What it does:**
- Challenges a malicious DKG result
- Resets DKG timeout
- Submitter gets slashed
- Challenger receives reward

#### Notify Timeouts
```bash
# Notify DKG timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer

# Notify seed timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
  --submit --config configs/config.toml --developer
```

### 3. Sortition Pool Operations

#### Join Sortition Pool
```bash
OPERATOR="0x..."  # Operator address

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool join-sortition-pool \
  "$OPERATOR" --submit --config configs/config.toml --developer
```
**What it does:**
- Adds operator to sortition pool
- Operator becomes eligible for selection
- Requires sufficient authorization

#### Update Operator Status
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry update-operator-status \
  "$OPERATOR" --submit --config configs/config.toml --developer
```
**What it does:**
- Updates operator's weight in pool
- Syncs authorization changes
- Can be called when pool is not locked

#### Check Operator Status
```bash
OPERATOR="0x..."

# Check if operator is in pool
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
  "$OPERATOR" --config configs/config.toml --developer

# Check if operator is up to date
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-up-to-date \
  "$OPERATOR" --config configs/config.toml --developer
```

### 4. Authorization Operations

#### Increase Authorization
```bash
STAKING_PROVIDER="0x..."
APPLICATION="0xbd49D2e3E501918CD08Eb4cCa34984F428c83464"  # WalletRegistry
AMOUNT="0x2386f26fc10000"  # Hex amount

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum threshold token-staking increase-authorization \
  "$STAKING_PROVIDER" "$APPLICATION" "$AMOUNT" \
  --submit --config configs/config.toml --developer
```
**What it does:**
- Increases authorization for an application
- Updates operator weight in sortition pool
- Requires sufficient stake

#### Request Authorization Decrease
```bash
STAKING_PROVIDER="0x..."
APPLICATION="0xbd49D2e3E501918CD08Eb4cCa34984F428c83464"
AMOUNT="0x2386f26fc10000"

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry authorization-decrease-requested \
  "$STAKING_PROVIDER" "$APPLICATION" "$AMOUNT" \
  --submit --config configs/config.toml --developer
```
**What it does:**
- Requests authorization decrease
- Starts delay period
- Must wait for delay before decrease takes effect

#### Approve Authorization Decrease
```bash
STAKING_PROVIDER="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry approve-authorization-decrease \
  "$STAKING_PROVIDER" --submit --config configs/config.toml --developer
```
**What it does:**
- Approves pending authorization decrease
- Decreases authorization after delay period
- Updates operator weight

### 5. Rewards Operations

#### Check Available Rewards
```bash
STAKING_PROVIDER="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry available-rewards \
  "$STAKING_PROVIDER" --config configs/config.toml --developer
```
**What it does:**
- Returns amount of rewards available for withdrawal
- Rewards earned from sortition pool participation

#### Withdraw Rewards
```bash
STAKING_PROVIDER="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry withdraw-rewards \
  "$STAKING_PROVIDER" --submit --config configs/config.toml --developer
```
**What it does:**
- Withdraws available rewards
- Sends to staking provider's beneficiary address

### 6. Query Operations (View Functions - Free)

#### Check DKG State
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

#### Check Timeouts
```bash
# Check DKG timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# Check seed timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config configs/config.toml --developer
```

#### Get DKG Parameters
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry dkg-parameters \
  --config configs/config.toml --developer
```

#### Get Authorization Parameters
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry authorization-parameters \
  --config configs/config.toml --developer
```

#### Check Eligible Stake
```bash
STAKING_PROVIDER="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry eligible-stake \
  "$STAKING_PROVIDER" --config configs/config.toml --developer
```

## Operations That Trigger Pool Activity

### Similar to `request-new-wallet`:

1. **`select-group`** - Selects operators from pool (view function)
2. **`join-sortition-pool`** - Adds operator to pool
3. **`update-operator-status`** - Updates operator weight
4. **`submit-dkg-result`** - Submits DKG result (requires DKG completion)
5. **`challenge-dkg-result`** - Challenges malicious result

## Complete Workflow Example

### Full Wallet Creation Workflow

```bash
# Step 1: Request new wallet (triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer

# Step 2: Wait for seed (state becomes AWAITING_SEED then AWAITING_RESULT)
./scripts/monitor-dkg.sh

# Step 3: Select group (view function - see which operators selected)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer

# Step 4: Wait for DKG completion (nodes do this automatically)
# Or manually submit result if you have it

# Step 5: Approve result (after challenge period)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry approve-dkg-result \
  --config configs/config.toml --developer

# Step 6: Get wallet ID from events or result
# Step 7: Use wallet for signing operations
```

## Testing Operations

### Test Sortition Pool Selection

```bash
# Select group multiple times to see different selections
for i in {1..5}; do
  echo "Selection $i:"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
    --config configs/config.toml --developer
  sleep 2
done
```

### Test Operator Status Updates

```bash
# Check operator status
OPERATOR="0x..."
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
  "$OPERATOR" --config configs/config.toml --developer

# Update status
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry update-operator-status \
  "$OPERATOR" --submit --config configs/config.toml --developer
```

## Quick Reference

| Operation | Type | Gas Cost | Description |
|-----------|------|----------|-------------|
| `request-new-wallet` | Write | Yes | Triggers DKG |
| `select-group` | View | Free | Selects operators |
| `join-sortition-pool` | Write | Yes | Adds operator to pool |
| `update-operator-status` | Write | Yes | Updates operator weight |
| `submit-dkg-result` | Write | Yes | Submits DKG result |
| `approve-dkg-result` | Write | Yes | Approves result, creates wallet |
| `challenge-dkg-result` | Write | Yes | Challenges malicious result |
| `close-wallet` | Write | Yes | Closes existing wallet |
| `is-operator-in-pool` | View | Free | Checks operator status |
| `get-wallet-creation-state` | View | Free | Gets DKG state |

## Most Useful Operations for Testing

1. **`select-group`** - See which operators would be selected (free, no gas)
2. **`is-operator-in-pool`** - Check operator status
3. **`update-operator-status`** - Sync operator weight
4. **`request-new-wallet`** - Trigger DKG (what you're already doing)
5. **`close-wallet`** - Clean up after testing

## See All Available Commands

```bash
# Wallet Registry commands
./keep-client ethereum ecdsa wallet-registry --help

# Sortition Pool commands
./keep-client ethereum ecdsa sortition-pool --help

# Token Staking commands
./keep-client ethereum threshold token-staking --help
```

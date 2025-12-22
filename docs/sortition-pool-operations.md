# Sortition Pool Operations Guide

Operations that can be performed on the sortition pool, similar to generating a new wallet.

## Sortition Pool Operations

### Check Pool Status

#### Check if Pool is Locked
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool is-locked \
  --config configs/config.toml --developer
```
**What it does:**
- Returns `true` if pool is locked (DKG in progress)
- Returns `false` if pool is unlocked (operators can join/leave)

#### Get Total Pool Weight
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool total-weight \
  --config configs/config.toml --developer
```
**What it does:**
- Returns total weight of all operators in pool
- Weight is based on authorized stake

#### Get Operator Count
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool operators-in-pool \
  --config configs/config.toml --developer
```
**What it does:**
- Returns number of operators in the pool

### Operator Operations

#### Check if Operator is in Pool
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool is-operator-in-pool \
  "$OPERATOR" --config configs/config.toml --developer
```

#### Get Operator ID
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool get-operator-id \
  "$OPERATOR" --config configs/config.toml --developer
```
**What it does:**
- Returns operator's ID in the pool
- Returns 0 if operator is not in pool

#### Get Operator by ID
```bash
OPERATOR_ID=1

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool get-id-operator \
  "$OPERATOR_ID" --config configs/config.toml --developer
```
**What it does:**
- Returns operator address for given ID

#### Get Operator Pool Weight
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool get-pool-weight \
  "$OPERATOR" --config configs/config.toml --developer
```
**What it does:**
- Returns operator's weight in the pool
- Weight determines selection probability

#### Check if Operator is Up to Date
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool is-operator-up-to-date \
  "$OPERATOR" --config configs/config.toml --developer
```
**What it does:**
- Checks if operator's weight matches their authorization
- Returns `false` if update is needed

#### Update Operator Status
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool update-operator-status \
  "$OPERATOR" --submit --config configs/config.toml --developer
```
**What it does:**
- Updates operator's weight in pool
- Syncs with current authorization
- Requires pool to be unlocked

### Selection Operations

#### Select Group (During DKG)
```bash
# Only works when pool is locked (DKG in progress)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer
```
**What it does:**
- Selects group of operators for DKG
- Uses current DKG seed
- Returns array of operator IDs
- **Free** - view function

**Note:** This only works when sortition pool is locked (during DKG).

### Rewards Operations

#### Check Available Rewards
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool get-available-rewards \
  "$OPERATOR" --config configs/config.toml --developer
```

#### Check Rewards Eligibility
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool is-eligible-for-rewards \
  "$OPERATOR" --config configs/config.toml --developer
```

#### Withdraw Rewards
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool withdraw-rewards \
  "$OPERATOR" --submit --config configs/config.toml --developer
```

## Operations Similar to `request-new-wallet`

### 1. **`select-group`** (View Function)
- **When**: During DKG (pool locked)
- **Cost**: Free (view function)
- **Purpose**: See which operators would be selected
- **Use case**: Test selection algorithm

### 2. **`join-sortition-pool`** (Write Function)
- **When**: Pool unlocked
- **Cost**: Gas required
- **Purpose**: Add operator to pool
- **Use case**: Test operator joining

### 3. **`update-operator-status`** (Write Function)
- **When**: Pool unlocked
- **Cost**: Gas required
- **Purpose**: Sync operator weight
- **Use case**: Test weight updates

### 4. **`is-operator-in-pool`** (View Function)
- **When**: Anytime
- **Cost**: Free
- **Purpose**: Check operator status
- **Use case**: Verify operator registration

## Testing Scenarios

### Scenario 1: Test Group Selection (During DKG)

```bash
# Step 1: Trigger DKG
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer

# Step 2: Wait for pool to lock (state becomes AWAITING_RESULT)
sleep 5

# Step 3: Select group multiple times to see selection
for i in {1..5}; do
  echo "Selection $i:"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
    --config configs/config.toml --developer
  sleep 2
done
```

### Scenario 2: Test Operator Pool Operations

```bash
# Get operator address
OPERATOR=$(curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address')

# Check if in pool
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool is-operator-in-pool \
  "$OPERATOR" --config configs/config.toml --developer

# Get operator ID
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool get-operator-id \
  "$OPERATOR" --config configs/config.toml --developer

# Get pool weight
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool get-pool-weight \
  "$OPERATOR" --config configs/config.toml --developer

# Update status (if needed)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool update-operator-status \
  "$OPERATOR" --submit --config configs/config.toml --developer
```

### Scenario 3: Test Pool Statistics

```bash
# Get total weight
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool total-weight \
  --config configs/config.toml --developer

# Get operator count
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool operators-in-pool \
  --config configs/config.toml --developer

# Check if locked
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa sortition-pool is-locked \
  --config configs/config.toml --developer
```

## Quick Reference

| Operation | Type | Cost | When Available |
|-----------|------|------|----------------|
| `select-group` | View | Free | During DKG (pool locked) |
| `join-sortition-pool` | Write | Gas | Pool unlocked |
| `update-operator-status` | Write | Gas | Pool unlocked |
| `is-operator-in-pool` | View | Free | Anytime |
| `get-pool-weight` | View | Free | Anytime |
| `total-weight` | View | Free | Anytime |
| `operators-in-pool` | View | Free | Anytime |
| `is-locked` | View | Free | Anytime |

## Most Useful for Testing

1. **`select-group`** - See operator selection (requires DKG)
2. **`is-operator-in-pool`** - Check operator status
3. **`get-pool-weight`** - Check operator weight
4. **`total-weight`** - Check total pool weight
5. **`update-operator-status`** - Sync operator weight

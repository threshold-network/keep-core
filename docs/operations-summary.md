# Operations Similar to `request-new-wallet`

This guide lists operations that can be performed against the node pool/sortition pool, similar to generating a new wallet.

## Main Operations

### 1. **`select-group`** - Select Operators for DKG
```bash
# Requires: Sortition pool must be locked (during DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer
```
**What it does:**
- Selects group of operators from sortition pool
- Uses current DKG seed
- Returns array of operator IDs
- **Free** - view function, no gas cost
- **Requires**: Pool must be locked (DKG in progress)

**Use case:** See which operators would be selected for DKG

### 2. **`join-sortition-pool`** - Add Operator to Pool
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry join-sortition-pool \
  "$OPERATOR" --submit --config configs/config.toml --developer
```
**What it does:**
- Adds operator to sortition pool
- Makes operator eligible for selection
- Requires sufficient authorization
- **Requires**: Pool must be unlocked

**Use case:** Test operator joining process

### 3. **`update-operator-status`** - Sync Operator Weight
```bash
OPERATOR="0x..."

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry update-operator-status \
  "$OPERATOR" --submit --config configs/config.toml --developer
```
**What it does:**
- Updates operator's weight in pool
- Syncs with current authorization
- **Requires**: Pool must be unlocked

**Use case:** Test weight synchronization

### 4. **`close-wallet`** - Close Existing Wallet
```bash
WALLET_ID="0x..."  # 32-byte wallet ID

KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry close-wallet \
  "$WALLET_ID" --submit --config configs/config.toml --developer
```
**What it does:**
- Closes an existing wallet
- Removes wallet from registry
- Only wallet owner can call

**Use case:** Clean up after testing

### 5. **`challenge-dkg-result`** - Challenge Malicious Result
```bash
# Requires DKG result data
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry challenge-dkg-result \
  --config configs/config.toml --developer
```
**What it does:**
- Challenges a malicious DKG result
- Resets DKG timeout
- Submitter gets slashed
- Challenger receives reward

**Use case:** Test challenge mechanism

### 6. **`approve-dkg-result`** - Approve DKG Result
```bash
# After challenge period passes
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry approve-dkg-result \
  --config configs/config.toml --developer
```
**What it does:**
- Approves submitted DKG result
- Creates wallet
- Unlocks sortition pool

**Use case:** Complete DKG process

## Sortition Pool Query Operations (Free)

### Check Pool Status
```bash
# Check if pool is locked
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool is-locked \
  --config configs/config.toml --developer

# Get total pool weight
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool total-weight \
  --config configs/config.toml --developer

# Get operator count
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool operators-in-pool \
  --config configs/config.toml --developer
```

### Check Operator Status
```bash
OPERATOR="0x..."

# Check if operator is in pool
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool is-operator-in-pool \
  "$OPERATOR" --config configs/config.toml --developer

# Get operator ID
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool get-operator-i-d \
  "$OPERATOR" --config configs/config.toml --developer

# Get operator weight
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool get-pool-weight \
  "$OPERATOR" --config configs/config.toml --developer

# Check if operator is up to date
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool is-operator-up-to-date \
  "$OPERATOR" --config configs/config.toml --developer
```

## Complete Testing Workflow

### Test Group Selection
```bash
# Step 1: Trigger DKG
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer

# Step 2: Wait for pool to lock
sleep 5

# Step 3: Select group (can call multiple times)
for i in {1..3}; do
  echo "Selection $i:"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
    --config configs/config.toml --developer
done
```

### Test Operator Pool Operations
```bash
# Get operator address
OPERATOR=$(curl -s http://localhost:9601/diagnostics | jq -r '.client_info.chain_address')

# Check pool status
echo "Pool locked:"
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool is-locked \
  --config configs/config.toml --developer

echo "Total weight:"
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool total-weight \
  --config configs/config.toml --developer

echo "Operator count:"
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool operators-in-pool \
  --config configs/config.toml --developer

# Check operator
echo "Operator in pool:"
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa ecdsa-sortition-pool is-operator-in-pool \
  "$OPERATOR" --config configs/config.toml --developer
```

## Quick Reference Table

| Operation | Command | Type | Cost | When Available |
|-----------|---------|------|------|----------------|
| Select Group | `wallet-registry select-group` | View | Free | During DKG (pool locked) |
| Join Pool | `wallet-registry join-sortition-pool` | Write | Gas | Pool unlocked |
| Update Status | `wallet-registry update-operator-status` | Write | Gas | Pool unlocked |
| Close Wallet | `wallet-registry close-wallet` | Write | Gas | Anytime |
| Challenge Result | `wallet-registry challenge-dkg-result` | Write | Gas | During challenge period |
| Approve Result | `wallet-registry approve-dkg-result` | Write | Gas | After challenge period |
| Check Pool Lock | `ecdsa-sortition-pool is-locked` | View | Free | Anytime |
| Get Total Weight | `ecdsa-sortition-pool total-weight` | View | Free | Anytime |
| Get Operator Count | `ecdsa-sortition-pool operators-in-pool` | View | Free | Anytime |
| Check Operator | `ecdsa-sortition-pool is-operator-in-pool` | View | Free | Anytime |

## Most Useful for Testing

1. **`select-group`** - See operator selection (requires DKG)
2. **`is-operator-in-pool`** - Check operator status
3. **`total-weight`** - Check pool statistics
4. **`operators-in-pool`** - Get operator count
5. **`update-operator-status`** - Sync operator weight

## See All Commands

```bash
# Wallet Registry commands
./keep-client ethereum ecdsa wallet-registry --help

# Sortition Pool commands
./keep-client ethereum ecdsa ecdsa-sortition-pool --help
```

# DKG `request-new-wallet` Processing Flow and Statuses

## Overview

The `request-new-wallet` command initiates a Distributed Key Generation (DKG) process to create a new ECDSA wallet signing group. This document explains the complete flow from initiation to completion.

## Command

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml \
  --developer
```

## Complete Processing Flow

### Phase 1: Request Initiation

**What Happens:**
1. Wallet Owner calls `requestNewWallet()` on `WalletRegistry` contract
2. Contract locks the DKG state (prevents concurrent DKG rounds)
3. Contract requests a relay entry from Random Beacon
4. Sortition pool is locked

**On-Chain State:** `IDLE` → `AWAITING_SEED`

**Events Emitted:**
- `DkgStateLocked()` - DKG state locked, sortition pool locked

**Time:** Immediate (transaction confirmation)

---

### Phase 2: Seed Generation (AWAITING_SEED)

**What Happens:**
1. Random Beacon generates a relay entry (random seed)
2. Random Beacon calls `__beaconCallback()` on WalletRegistry
3. DKG starts with the provided seed
4. Sortition pool selects group members based on seed

**On-Chain State:** `AWAITING_SEED` → `AWAITING_RESULT`

**Events Emitted:**
- `DkgStarted(uint256 seed)` - DKG started with seed value

**Time:** 
- Mainnet: Variable (depends on Random Beacon group)
- Local Dev: Usually within minutes

**Timeout:** 11,520 blocks (~48h mainnet, ~3h local)
- If timeout exceeded: Anyone can call `notifySeedTimeout()` to reset

---

### Phase 3: Off-Chain DKG Protocol (AWAITING_RESULT)

**What Happens:**

This is the most complex phase where operators execute the GJKR DKG protocol off-chain:

#### 3.1 Group Selection
- Sortition pool selects 100 operators based on seed
- Selected operators check eligibility
- Operators join DKG if selected

#### 3.2 DKG Protocol Phases (Off-Chain)

**Phase 1: Ephemeral Key Generation**
- Each operator generates ephemeral ECDH keypairs
- Keys are broadcast to other group members

**Phase 2: Ephemeral ECDH**
- Operators perform ECDH key exchange
- Creates symmetric keys for encrypted communication

**Phase 3: Polynomial Generation**
- Each operator generates secret polynomials
- Calculates shares for other members
- Creates Pedersen commitments
- Encrypts shares with symmetric keys

**Phase 4: Share Verification**
- Operators decrypt and validate received shares
- Broadcast complaints if shares are invalid

**Phase 5: Share Complaint Resolution**
- Misbehaving operators are disqualified
- Valid shares are confirmed

**Phase 6: Share Calculation**
- Each operator calculates their final share
- Shares sum to the group secret

**Phase 7: Public Key Share Points**
- Operators broadcast public key components

**Phase 8: Public Key Share Validation**
- Operators validate public key components

**Phase 9: Second Complaint Resolution**
- Final validation and disqualification

**Phase 10-14: Result Preparation**
- Operators calculate group public key
- Prepare DKG result with signatures
- Coordinate result submission

**On-Chain State:** `AWAITING_RESULT` (entire duration)

**Events Emitted:** None (all off-chain)

**Time:**
- Mainnet: ~2.2 hours (536 blocks at 15s/block)
- Local Dev: ~8-9 minutes (536 blocks at 1s/block)

**Timeout:** 536 blocks
- If timeout exceeded: Anyone can call `notifyDkgTimeout()` to reset

**Monitoring:**
- Check node logs for DKG protocol messages
- Monitor metrics: `curl -s http://localhost:9601/metrics | grep performance_dkg`

---

### Phase 4: Result Submission (AWAITING_RESULT → CHALLENGE)

**What Happens:**
1. One operator submits DKG result to chain
2. Result includes:
   - Group public key
   - Misbehaved members list
   - Signatures from supporting members
   - Member indices
3. Result is registered optimistically
4. State transitions to challenge period

**On-Chain State:** `AWAITING_RESULT` → `CHALLENGE`

**Events Emitted:**
- `DkgResultSubmitted(bytes32 resultHash, uint256 seed, Result result)`

**Time:** Immediate (transaction confirmation)

**Who Can Submit:**
- Any operator in the selected group
- First valid submission wins

---

### Phase 5: Challenge Period (CHALLENGE)

**What Happens:**
1. DKG result is publicly available
2. Anyone can challenge the result if invalid
3. Challenge period allows verification
4. If challenged and proven invalid:
   - Result submitter is slashed
   - State returns to `AWAITING_RESULT`
   - New result can be submitted

**On-Chain State:** `CHALLENGE` (entire duration)

**Events Emitted:**
- `DkgResultChallenged(bytes32 resultHash, address challenger, string reason)` (if challenged)
- `DkgMaliciousResultSlashed(...)` (if challenge succeeds)

**Time:**
- Mainnet: ~48 hours (11,520 blocks at 15s/block)
- Local Dev: ~3 hours (11,520 blocks at 1s/block)

**Who Can Challenge:**
- Anyone (public knowledge transaction)
- Must prove result is invalid

---

### Phase 6: Result Approval (CHALLENGE → IDLE)

**What Happens:**
1. Challenge period ends
2. Result submitter has precedence period (20 blocks) to approve
3. After precedence period, anyone can approve
4. Approval:
   - Validates result
   - Bans misbehaved members from rewards
   - Creates wallet in registry
   - Calls wallet owner callback
   - Completes DKG (unlocks sortition pool)

**On-Chain State:** `CHALLENGE` → `IDLE`

**Events Emitted:**
- `DkgResultApproved(bytes32 resultHash, address approver)`
- `WalletCreated(bytes32 walletID, bytes32 dkgResultHash)`
- `EcdsaWalletCreated(bytes32 walletID, bytes32 publicKeyX, bytes32 publicKeyY)` (to wallet owner)

**Time:** Immediate (transaction confirmation)

**Who Can Approve:**
- Result submitter (first 20 blocks after challenge period)
- Anyone (after precedence period)

---

## DKG State Summary

| State | Value | Description | Duration (Local) | Duration (Mainnet) |
|-------|-------|-------------|------------------|-------------------|
| **IDLE** | 0 | Ready for new wallet request | - | - |
| **AWAITING_SEED** | 1 | Waiting for Random Beacon seed | Minutes | Variable |
| **AWAITING_RESULT** | 2 | Operators generating keys off-chain | ~8-9 minutes | ~2.2 hours |
| **CHALLENGE** | 3 | Result submitted, in challenge period | ~3 hours | ~48 hours |

## Complete Timeline (Happy Path)

**Local Development:**
```
Request → Seed → DKG Protocol → Submit → Challenge → Approve → Complete
  ~1s      ~1m      ~8 min        ~1s      ~3h        ~1s      ~1s
Total: ~3-4 hours
```

**Mainnet:**
```
Request → Seed → DKG Protocol → Submit → Challenge → Approve → Complete
  ~15s    Variable   ~2.2h        ~15s     ~48h        ~15s     ~15s
Total: ~48 hours
```

## Monitoring Commands

### Check Current State
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml \
  --developer
```

### Monitor State Changes
```bash
watch -n 5 './scripts/monitor-dkg.sh configs/config.toml'
```

### Check Timing
```bash
./scripts/check-dkg-timing.sh configs/config.toml
```

### View Node Metrics
```bash
curl -s http://localhost:9601/metrics | grep performance_dkg
```

### Check Node Logs
```bash
tail -f <log-file> | grep -i "dkg\|wallet"
```

## Error Conditions and Recovery

### Seed Timeout
- **Condition:** Random Beacon doesn't provide seed within timeout
- **Recovery:** Anyone can call `notifySeedTimeout()` to reset to IDLE
- **Command:**
  ```bash
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
    --submit --config configs/config.toml --developer
  ```

### DKG Stuck in AWAITING_RESULT

**Symptoms:** DKG state remains `AWAITING_RESULT` (state 2) for extended period

**Common Causes:**
1. **Operator Not Selected** - Your operator wasn't selected for this DKG round
2. **Insufficient Pre-Parameters** - Node doesn't have enough pre-generated parameters
3. **Network Connectivity** - Operators can't communicate via LibP2P
4. **Not Enough Operators** - Local dev may only have 1 operator (needs 100)
5. **Still Processing** - Normal if within timeout window (~8-9 min locally)

**Diagnosis:**
```bash
# Run diagnostic script
./scripts/diagnose-dkg-stuck.sh configs/config.toml

# Check node logs for:
# - 'not eligible for DKG'
# - 'pre-parameters pool size is too small'
# - 'selecting group not possible'
# - DKG protocol messages
```

**Recovery Options:**

1. **Reset DKG (if timeout passed):**
   ```bash
   ./scripts/reset-dkg.sh configs/config.toml
   # OR manually:
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
     --submit --config configs/config.toml --developer
   ```

2. **Check Node Logs:**
   ```bash
   tail -f <log-file> | grep -i "dkg\|eligibility\|pre-parameters"
   ```

3. **Restart Node (if pre-params issue):**
   ```bash
   # Stop node, then restart to regenerate pre-parameters
   ./scripts/start.sh
   ```

**Note:** DKG timeout is 536 blocks (~8-9 minutes locally). You can only reset after timeout.

### DKG Timeout
- **Condition:** Operators don't submit result within timeout (536 blocks)
- **Recovery:** Anyone can call `notifyDkgTimeout()` to reset to IDLE
- **Command:**
  ```bash
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
    --submit --config configs/config.toml --developer
  ```

### Invalid Result Challenge
- **Condition:** Submitted DKG result is invalid
- **Recovery:** Anyone can challenge, result submitter gets slashed
- **State:** Returns to `AWAITING_RESULT` for new submission

## Prerequisites

For DKG to complete successfully:

1. **Wallet Owner Set:** Must match your operator address
2. **Operator Registered:** Your operator must be in sortition pool
3. **Sufficient Authorization:** Operator must have enough stake
4. **Network Connectivity:** Operators must communicate via LibP2P
5. **Random Beacon Active:** Must provide relay entries

## Key Functions

- `requestNewWallet()` - Initiates DKG (wallet owner only)
- `__beaconCallback()` - Starts DKG with seed (Random Beacon only)
- `submitDkgResult()` - Submits DKG result (any group member)
- `challengeDkgResult()` - Challenges invalid result (anyone)
- `approveDkgResult()` - Approves result after challenge period
- `notifySeedTimeout()` - Resets if seed timeout
- `notifyDkgTimeout()` - Resets if DKG timeout

## Related Documentation

- `docs/dkg-testing-quick-fix.md` - Quick troubleshooting guide
- `docs/test-dkg-locally.md` - Local DKG testing guide
- `scripts/monitor-dkg.sh` - Monitoring script
- `scripts/check-dkg-timing.sh` - Timing information script
- `scripts/diagnose-dkg-stuck.sh` - Diagnose stuck DKG issues
- `scripts/reset-dkg.sh` - Reset stuck DKG (after timeout)

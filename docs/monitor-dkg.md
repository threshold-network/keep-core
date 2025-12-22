# How to Monitor DKG Processing

This guide shows multiple ways to check the status and progress of a DKG (Distributed Key Generation) process.

## Quick Status Check

### 1. Check DKG State (Contract Level)

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**State Values:**
- `0` = **IDLE** - No DKG in progress, ready for new wallet request
- `1` = **AWAITING_SEED** - Waiting for Random Beacon to provide seed
- `2` = **AWAITING_RESULT** - DKG is running, waiting for result submission
- `3` = **CHALLENGE** - DKG result submitted, in challenge period

### 2. Check Timeout Status

```bash
# Check if DKG timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# Check if seed timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config configs/config.toml --developer
```

## Monitor via Logs

### 3. Watch DKG Activity in Logs

```bash
# Monitor all nodes for DKG activity
tail -f logs/node*.log | grep -iE "dkg|keygen|wallet|member"

# Monitor specific node
tail -f logs/node1.log | grep -iE "dkg|keygen"

# Check for errors
tail -f logs/node*.log | grep -iE "error|fatal|dkg.*fail"
```

### 4. Key Log Messages to Look For

**DKG Started:**
```
INFO ... DkgStarted ...
```

**Key Generation (TSS-lib):**
```
INFO tss-lib keygen/prepare.go:63 generating the Paillier modulus, please wait...
INFO tss-lib keygen/prepare.go:78 generating the safe primes for the signing proofs, please wait...
INFO tss-lib keygen/prepare.go:71 paillier modulus generated. took [X]s
INFO tss-lib keygen/prepare.go:85 safe primes generated. took [X]s
```

**Member Participation:**
```
INFO ... member [operator_address] is starting signer generation for keep [wallet_id]...
```

**DKG Result Submission:**
```
INFO ... DkgResultSubmitted ...
```

**Wallet Created:**
```
INFO ... WalletCreated ...
```

## Monitor via Node Diagnostics

### 5. Check Node Diagnostics

```bash
# Check node 1 diagnostics
curl -s http://localhost:9601/diagnostics | jq '.'

# Check connected peers (important for DKG)
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'

# Check all nodes
for i in {1..3}; do
  echo "=== Node $i ==="
  curl -s http://localhost:960$i/diagnostics | jq '.client_info'
done
```

### 6. Check Metrics

```bash
# Check LibP2P metrics (peer connectivity)
curl -s http://localhost:9601/metrics | grep libp2p

# Check DKG-related metrics
curl -s http://localhost:9601/metrics | grep -i dkg

# Check connected peers count
curl -s http://localhost:9601/metrics | grep connected_peers
```

## Monitor via Contract Events

### 7. Check Recent DKG Events

Using Hardhat console:

```bash
npx hardhat console --network development
```

Then:

```javascript
const { ethers, helpers } = require("hardhat");
const walletRegistry = await helpers.contracts.getContract("WalletRegistry");

// Get recent DkgStarted events
const filter = walletRegistry.filters.DkgStarted();
const events = await walletRegistry.queryFilter(filter, -100); // Last 100 blocks
console.log("DKG Started events:", events.length);

// Get recent DkgResultSubmitted events
const resultFilter = walletRegistry.filters.DkgResultSubmitted();
const results = await walletRegistry.queryFilter(resultFilter, -100);
console.log("DKG Results submitted:", results.length);

// Get recent WalletCreated events
const walletFilter = walletRegistry.filters.WalletCreated();
const wallets = await walletRegistry.queryFilter(walletFilter, -100);
console.log("Wallets created:", wallets.length);
```

## Comprehensive Monitoring Script

### 8. Use the Monitor Script

```bash
# Run the monitoring script
./scripts/monitor-dkg.sh

# Or monitor continuously
watch -n 5 ./scripts/monitor-dkg.sh
```

## Check Operator Participation

### 9. Verify Operators Are Participating

```bash
# Check if operators are in the sortition pool
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i operator: $OPERATOR"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

## DKG Progress Indicators

### State 0 (IDLE)
- ✅ No DKG in progress
- Ready to request new wallet

### State 1 (AWAITING_SEED)
- ⏳ Waiting for Random Beacon relay entry
- Check Random Beacon status
- If stuck, check `has-seed-timed-out`

### State 2 (AWAITING_RESULT)
- 🔄 **DKG is actively running**
- Operators are generating keys off-chain
- Look for `keygen/prepare.go` messages in logs
- Check operator connectivity
- Monitor for `DkgResultSubmitted` events

### State 3 (CHALLENGE)
- ✅ DKG result submitted
- Waiting for challenge period
- Look for `approveDkgResult` or `challengeDkgResult` transactions

## Troubleshooting

### DKG Stuck in AWAITING_RESULT

**Check:**
1. Are all selected operators running?
   ```bash
   ./configs/check-nodes.sh
   ```

2. Are operators connected?
   ```bash
   curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'
   ```

3. Check for errors in logs:
   ```bash
   tail -100 logs/node*.log | grep -i error
   ```

4. Check if DKG timed out:
   ```bash
   ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
     --config configs/config.toml --developer
   ```

### DKG Taking Too Long

**Normal DKG Duration:**
- Key generation: 10-30 seconds per operator
- Result submission: Depends on network conditions
- Challenge period: Governable timeout (check contract)

**If stuck:**
- Check operator logs for keygen progress
- Verify LibP2P connectivity
- Check if operators have sufficient authorization

## Quick Reference Commands

```bash
# Quick status check
./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Monitor logs
tail -f logs/node*.log | grep -i dkg

# Check connectivity
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'

# Check for timeouts
./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# Full monitoring
./scripts/monitor-dkg.sh
```

## Expected DKG Flow

1. **Request New Wallet** → State changes to `1` (AWAITING_SEED)
2. **Random Beacon Provides Seed** → State changes to `2` (AWAITING_RESULT)
3. **Operators Generate Keys** → Look for `keygen` logs
4. **Result Submitted** → State changes to `3` (CHALLENGE)
5. **Result Approved** → Wallet created, State returns to `0` (IDLE)

Monitor each stage using the methods above!


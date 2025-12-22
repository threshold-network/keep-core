# Complete Guide to DKG (Distributed Key Generation)

## What is DKG?

**Distributed Key Generation (DKG)** is a cryptographic protocol that allows multiple parties (operators) to collaboratively generate a shared cryptographic key **without any single party ever knowing the complete private key**. This is essential for threshold cryptography, where signatures can be created only when a sufficient number of parties (the threshold) cooperate.

### Key Concepts

- **No Single Point of Failure**: The private key is never assembled in one place
- **Threshold Cryptography**: Requires a minimum number of parties (e.g., 51 out of 100) to sign
- **Distributed Trust**: No single operator can compromise the system
- **Verifiable**: All participants can verify the correctness of the process

## What DKG Involves

DKG in `keep-core` involves:

1. **On-Chain Components**:
   - `WalletRegistry` contract: Manages DKG lifecycle and wallet creation
   - `RandomBeacon` contract: Provides randomness seed for DKG
   - `EcdsaSortitionPool` contract: Selects operators for DKG
   - `TokenStaking` contract: Ensures operators have sufficient stake

2. **Off-Chain Components**:
   - Multiple `keep-client` nodes running the DKG protocol
   - LibP2P networking: Enables peer-to-peer communication
   - Cryptographic operations: Key generation, secret sharing, verification

3. **Protocol Phases**:
   - Ephemeral key exchange
   - Secret sharing (Pedersen-VSS)
   - Key generation (TSS-lib)
   - Result submission and verification

## DKG Stages

### On-Chain States (Contract Level)

The `WalletRegistry` contract tracks DKG progress through these states:

#### State 0: IDLE
- **Meaning**: No DKG in progress
- **What happens**: Ready to accept new wallet creation requests
- **Duration**: Indefinite (until triggered)

#### State 1: AWAITING_SEED
- **Meaning**: Sortition pool is locked, waiting for Random Beacon seed
- **What happens**:
  - Pool is locked (no operators can join/leave)
  - Group of 100 operators is selected
  - Random Beacon is requested to provide randomness
- **Duration**: Until seed arrives or seed timeout (~10 minutes)
- **Check**: `has-seed-timed-out`

#### State 2: AWAITING_RESULT
- **Meaning**: DKG protocol is executing off-chain
- **What happens**:
  - Operators generate keys collaboratively
  - Cryptographic operations run (see Off-Chain Phases below)
  - Result is calculated and prepared for submission
- **Duration**: ~30-60 minutes (for 3 nodes), up to 89 minutes max
- **Check**: `has-dkg-timed-out`

#### State 3: CHALLENGE
- **Meaning**: DKG result submitted, in challenge period
- **What happens**:
  - Result is on-chain
  - Anyone can challenge if result is invalid
  - Result submitter can approve
- **Duration**: Challenge period (configurable, ~48 hours in production)
- **Outcome**: 
  - If approved → Wallet created, state returns to IDLE
  - If challenged → State returns to AWAITING_RESULT

### Off-Chain Protocol Phases

The actual DKG protocol runs in multiple phases:

#### Phase 1: Ephemeral Key Generation
- **Purpose**: Establish secure communication channels
- **What happens**:
  - Each operator generates ephemeral ECDH keypairs for every other operator
  - Public keys are broadcast to all participants
- **Duration**: Seconds (depends on network latency)
- **Logs**: Look for `ephemeralPublicKeyMessage`

#### Phase 2: Symmetric Key Generation
- **Purpose**: Create shared symmetric keys for encrypted communication
- **What happens**:
  - Each operator performs ECDH with every other operator
  - Symmetric keys are derived for pairwise encryption
- **Duration**: Seconds
- **Logs**: Internal state transition

#### Phase 3: TSS Round One (Commitments & Paillier Keys)
- **Purpose**: Prepare for secret sharing
- **What happens**:
  - Each operator generates Paillier public key (for homomorphic encryption)
  - Commitments to secret shares are created
  - Messages are encrypted with symmetric keys from Phase 2
- **Duration**: Seconds to minutes (Paillier key generation is CPU-intensive)
- **Logs**: 
  ```
  INFO tss-lib keygen/prepare.go:63 generating the Paillier modulus, please wait...
  INFO tss-lib keygen/prepare.go:78 generating the safe primes for the signing proofs, please wait...
  ```

#### Phase 4: TSS Round Two (Share Distribution)
- **Purpose**: Distribute secret shares
- **What happens**:
  - Each operator creates secret shares for all other operators
  - Shares are encrypted and broadcast
  - De-commitments are provided
- **Duration**: Seconds to minutes
- **Logs**: Look for share distribution messages

#### Phase 5: TSS Round Three (Proofs)
- **Purpose**: Verify correctness of shares
- **What happens**:
  - Operators verify received shares
  - Paillier proofs are generated and broadcast
  - Invalid shares are detected
- **Duration**: Seconds
- **Logs**: Look for proof messages

#### Phase 6: Finalization
- **Purpose**: Calculate final key shares
- **What happens**:
  - Each operator calculates their final share of the private key
  - Group public key is computed
  - Result is prepared
- **Duration**: Seconds
- **Logs**: Look for finalization messages

#### Phase 7: Result Signing
- **Purpose**: Get consensus on the result
- **What happens**:
  - Each operator signs the DKG result hash
  - Signatures are collected
  - Result is prepared for submission
- **Duration**: Seconds
- **Logs**: Look for signature messages

#### Phase 8: Result Submission
- **Purpose**: Submit result to blockchain
- **What happens**:
  - One operator submits the result to `WalletRegistry`
  - Result includes: group public key, misbehaved members, signatures
  - Contract validates the result
- **Duration**: Seconds (blockchain transaction)
- **Logs**: 
  ```
  INFO ... DkgResultSubmitted ...
  ```

## How to Test DKG

### Prerequisites

Before testing DKG, ensure:

1. **Ethereum Network**: Local Geth node running
   ```bash
   # Check Geth is running
   curl -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
     http://localhost:8545
   ```

2. **Operators Registered**: All operators registered in contracts
   ```bash
   # Check operator registration
   for i in {1..3}; do
     OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
     echo "Node $i ($OPERATOR):"
     KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
       "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
   done
   ```

3. **Nodes Running**: All `keep-client` instances are running
   ```bash
   ./configs/check-nodes.sh
   ```

4. **Operators in Pool**: Operators must be in sortition pool
   ```bash
   # Check pool status
   for i in {1..3}; do
     OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
     IN_POOL=$(KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
       "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1)
     echo "Node $i: $IN_POOL"
   done
   ```

5. **Nodes Connected**: LibP2P connectivity established
   ```bash
   # Check peer connectivity
   for i in {1..3}; do
     PEERS=$(curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length')
     echo "Node $i: $PEERS peers"
   done
   ```

### Testing Methods

#### Method 1: Automated Script (Recommended)

Use the automated script for a complete DKG workflow:

```bash
# Run complete DKG process
./scripts/process-dkg-3-nodes.sh
```

This script:
- Verifies prerequisites
- Checks operator pool status
- Triggers DKG
- Monitors progress
- Reports results

#### Method 2: Manual Step-by-Step

**Step 1: Check Initial State**
```bash
# Should return: 0 (IDLE)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

**Step 2: Trigger DKG**
```bash
# Request new wallet (triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

**Step 3: Monitor State Transitions**
```bash
# Watch state change: 0 → 1 → 2 → 3 → 0
watch -n 5 'KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer'
```

**Step 4: Check Group Selection**
```bash
# See which operators were selected (only works when pool is locked)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry select-group \
  --config configs/config.toml --developer
```

**Step 5: Monitor Logs**
```bash
# Watch for DKG activity
tail -f logs/node*.log | grep -iE "dkg|keygen|member|protocol|result"
```

**Step 6: Check for Completion**
```bash
# State should return to 0 when complete
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

#### Method 3: Comprehensive Monitoring

Use the monitoring script for detailed progress tracking:

```bash
# Continuous monitoring
./scripts/monitor-dkg.sh

# Or with watch
watch -n 5 ./scripts/monitor-dkg.sh
```

This script reports:
- Current DKG state
- Elapsed time
- Timeout status
- Key generation activity
- Node connectivity

#### Method 4: Unit/Integration Tests

Run Go tests for DKG protocol:

```bash
# Test DKG protocol phases
cd pkg/tecdsa/dkg
go test -v -run TestGenerateEphemeralKeyPair

# Test full DKG execution
cd pkg/tbtc
go test -v -run TestDKG

# Test Random Beacon DKG (GJKR protocol)
cd pkg/beacon/gjkr
go test -v -run TestExecute_HappyPath
```

### Testing Specific Scenarios

#### Test 1: Normal DKG Flow
```bash
# 1. Ensure IDLE state
# 2. Trigger DKG
# 3. Monitor through all states
# 4. Verify wallet creation
```

#### Test 2: DKG Timeout
```bash
# 1. Trigger DKG
# 2. Wait for timeout (or simulate)
# 3. Check timeout status
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# 4. Notify timeout
./scripts/stop-dkg.sh
```

#### Test 3: Seed Timeout
```bash
# 1. Trigger DKG (state → 1)
# 2. Wait for seed timeout
# 3. Check seed timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config configs/config.toml --developer

# 4. Notify seed timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
  --submit --config configs/config.toml --developer
```

#### Test 4: Result Challenge
```bash
# 1. Wait for DKG to reach CHALLENGE state (state 3)
# 2. Use Hardhat to challenge result
cd solidity/ecdsa
npx hardhat console --network development

# In console:
const { helpers } = require("hardhat");
const walletRegistry = await helpers.contracts.getContract("WalletRegistry");
const dkgResult = await walletRegistry.getDkgResult();
// Challenge if invalid
```

#### Test 5: Operator Connectivity
```bash
# Test with nodes disconnected
# 1. Stop one node
./configs/stop-all-nodes.sh
# Start only 2 nodes
# 2. Trigger DKG
# 3. Observe behavior (should timeout or fail)
```

### What to Monitor During Testing

#### On-Chain Metrics
- **DKG State**: Should progress 0 → 1 → 2 → 3 → 0
- **Timeout Status**: Check `has-dkg-timed-out` and `has-seed-timed-out`
- **Pool Status**: Check if pool is locked (`select-group` only works when locked)
- **Events**: Monitor `DkgStarted`, `DkgResultSubmitted`, `WalletCreated`

#### Off-Chain Metrics
- **Log Messages**: 
  - `generating the Paillier modulus` (Phase 3)
  - `member [address] is starting signer generation` (Phase 4-6)
  - `DkgResultSubmitted` (Phase 8)
- **Node Connectivity**: Check `connected_peers` count
- **CPU Usage**: DKG is CPU-intensive, especially Paillier key generation
- **Network Traffic**: LibP2P messages between nodes

#### Expected Timelines

**With 3 Nodes:**
- **State 0 → 1**: ~5 seconds (pool locks)
- **State 1 → 2**: Depends on Random Beacon (seconds to minutes)
- **State 2 Duration**: 30-60 minutes (key generation)
- **State 2 → 3**: Seconds (result submission)
- **State 3 Duration**: Challenge period (configurable, ~48h in production)
- **State 3 → 0**: Seconds (after approval)

**With 100 Nodes:**
- **State 2 Duration**: ~36 minutes (216 blocks × ~10s block time)
- More operators = more coordination overhead

### Troubleshooting Tests

#### DKG Stuck in AWAITING_RESULT
```bash
# Check timeout
./scripts/check-dkg-state.sh

# Check node connectivity
for i in {1..3}; do
  curl -s http://localhost:960$i/diagnostics | jq '.connected_peers | length'
done

# Check logs for errors
tail -100 logs/node*.log | grep -i error
```

#### Operators Not Selected
```bash
# Verify operators are in pool
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer
done
```

#### Seed Never Arrives
```bash
# Check Random Beacon status
# Check seed timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-seed-timed-out \
  --config configs/config.toml --developer

# If timed out, notify
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-seed-timeout \
  --submit --config configs/config.toml --developer
```

## Quick Reference

### State Check
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
```

### Trigger DKG
```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

### Monitor Progress
```bash
./scripts/monitor-dkg.sh
```

### Check Timeout
```bash
./scripts/check-dkg-state.sh
```

### Stop DKG (if timed out)
```bash
./scripts/stop-dkg.sh
```

## Summary

DKG is a complex multi-phase protocol that:
1. **Locks the sortition pool** and selects operators
2. **Waits for randomness** from Random Beacon
3. **Generates keys collaboratively** through multiple cryptographic phases
4. **Submits result** to the blockchain
5. **Enters challenge period** for verification
6. **Creates wallet** after approval

Testing DKG requires:
- ✅ All operators registered and in pool
- ✅ Nodes running and connected
- ✅ Proper monitoring of states and logs
- ✅ Understanding of timeout mechanisms
- ✅ Patience (DKG takes 30-60 minutes with 3 nodes)

For detailed workflow instructions, see:
- [`docs/process-dkg-with-3-nodes.md`](./process-dkg-with-3-nodes.md)
- [`docs/monitor-dkg.md`](./monitor-dkg.md)
- [`docs/dkg-key-generation-duration.md`](./dkg-key-generation-duration.md)
